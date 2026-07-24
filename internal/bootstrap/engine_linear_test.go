package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type linearFakeRunner struct {
	goRunner      *engineFakeRunner
	root          string
	npmCalls      int
	tscCalls      int
	missingOutput string
	mutateLock    bool
	linearCalls   []processCall
}

func (runner *linearFakeRunner) Run(ctx context.Context, request processRequest) ([]byte, error) {
	// runFrontendBuild's npm ci/npm run build also invoke npm-cli.js, so
	// this must be scoped to tools/linear-sync specifically -- otherwise
	// frontend/'s own npm calls would be misidentified as Linear-sync's
	// and never produce dist/index.html.
	if filepath.Base(request.Dir) == "frontend" {
		return runner.goRunner.Run(ctx, request)
	}
	if len(request.Args) > 0 && strings.HasSuffix(filepath.ToSlash(request.Args[0]), "/npm/bin/npm-cli.js") {
		runner.npmCalls++
		runner.linearCalls = append(runner.linearCalls, processCall{
			executable: request.Executable, dir: request.Dir,
			args: append([]string(nil), request.Args...), env: cloneEngineTestMap(request.Env),
		})
		tsc := filepath.Join(runner.root, "tools", "linear-sync", "node_modules", "typescript", "bin", "tsc")
		if err := os.MkdirAll(filepath.Dir(tsc), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(tsc, []byte("compiler\n"), 0o644); err != nil {
			return nil, err
		}
		if runner.mutateLock {
			if err := os.WriteFile(filepath.Join(runner.root, "tools", "linear-sync", "package-lock.json"), []byte("mutated\n"), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if len(request.Args) > 0 && strings.HasSuffix(filepath.ToSlash(request.Args[0]), "/typescript/bin/tsc") {
		runner.tscCalls++
		runner.linearCalls = append(runner.linearCalls, processCall{
			executable: request.Executable, dir: request.Dir,
			args: append([]string(nil), request.Args...), env: cloneEngineTestMap(request.Env),
		})
		for _, relative := range linearSyncExpectedOutputs {
			if relative == runner.missingOutput {
				continue
			}
			path := filepath.Join(runner.root, "tools", "linear-sync", filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, []byte("compiled\n"), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	return runner.goRunner.Run(ctx, request)
}

// addLinearSyncFixture sets up the tools/linear-sync workspace fixture
// only. It no longer declares [toolchain.node] or registers a Node
// archive payload itself: writeEngineRepository now does that
// unconditionally for every test (runFrontendBuild needs a working Node
// pin on every bootstrap, not just Linear-sync-enabled ones), so a
// second declaration here would collide as a duplicate TOML key.
func addLinearSyncFixture(t *testing.T, root string) {
	t.Helper()
	linearDir := filepath.Join(root, "tools", "linear-sync")
	if err := os.MkdirAll(linearDir, 0o755); err != nil {
		t.Fatalf("mkdir linear-sync: %v", err)
	}
	for name, body := range map[string]string{
		"package.json":      `{"name":"fixture","devDependencies":{"typescript":"7.0.2"}}` + "\n",
		"package-lock.json": `{"lockfileVersion":3,"packages":{}}` + "\n",
		"tsconfig.json":     `{"compilerOptions":{"outDir":"dist"}}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(linearDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func newLinearRunner(root string) *linearFakeRunner {
	return &linearFakeRunner{
		root: root,
		goRunner: &engineFakeRunner{moduleGraph: strings.Join([]string{
			"example.invalid/test",
			"github.com/BurntSushi/toml v1.6.0",
			"github.com/invopop/jsonschema v0.14.0",
		}, "\n") + "\n"},
	}
}

func TestScopeBootstrapLinearSync(t *testing.T) {
	t.Run("include false never inspects or provisions Linear tooling", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		linearDir := filepath.Join(root, "tools", "linear-sync")
		if err := os.MkdirAll(linearDir, 0o755); err != nil {
			t.Fatalf("mkdir canary: %v", err)
		}
		canary := filepath.Join(linearDir, "package.json")
		if err := os.WriteFile(canary, []byte("not json and intentionally ignored"), 0o644); err != nil {
			t.Fatalf("write canary: %v", err)
		}
		runner := newLinearRunner(root)
		if err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: runner}); err != nil {
			t.Fatalf("include-off bootstrap: %v", err)
		}
		if runner.npmCalls != 0 || runner.tscCalls != 0 {
			t.Fatalf("include-off invoked Linear processes: npm=%d tsc=%d", runner.npmCalls, runner.tscCalls)
		}
		if body, _ := os.ReadFile(canary); string(body) != "not json and intentionally ignored" {
			t.Fatalf("include-off changed package input: %q", body)
		}
	})

	t.Run("missing requested Node platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		addLinearSyncFixture(t, root)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.node.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.node.platforms."unconfigured-platform"]`), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, bootstrapDependencies{
			Source: source,
			Runner: newLinearRunner(root),
		})
		required := fmt.Sprintf(`[toolchain.node.platforms.%q]`, PlatformKey())
		if err == nil || !strings.Contains(err.Error(), required) {
			t.Fatalf("expected missing platform diagnostic naming %s, got %v", required, err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("missing Node platform consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("missing Node platform created .tools: %v", err)
		}
	})

	t.Run("first include runs exact-lock npm and tsc then repeat is a zero-call no-op", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		addLinearSyncFixture(t, root)
		runner := newLinearRunner(root)
		dependencies := bootstrapDependencies{Source: source, Runner: runner}
		if err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, dependencies); err != nil {
			t.Fatalf("first include bootstrap: %v", err)
		}
		if runner.npmCalls != 1 || runner.tscCalls != 1 {
			t.Fatalf("linear calls: npm=%d tsc=%d", runner.npmCalls, runner.tscCalls)
		}
		npm := runner.linearCalls[0]
		if got, want := strings.Join(npm.args[1:], " "), "ci --ignore-scripts --no-audit --no-fund"; got != want {
			t.Fatalf("npm args = %q, want %q", got, want)
		}
		if npm.env["NPM_CONFIG_CACHE"] != filepath.Join(root, ".tools", "cache", "npm") {
			t.Fatalf("npm cache = %q", npm.env["NPM_CONFIG_CACHE"])
		}
		tsc := runner.linearCalls[1]
		if len(tsc.args) != 3 || tsc.args[1] != "-p" || tsc.args[2] != filepath.Join(root, "tools", "linear-sync", "tsconfig.json") {
			t.Fatalf("tsc args = %v", tsc.args)
		}
		var manifest npmCIManifest
		manifestRaw, err := os.ReadFile(filepath.Join(root, "tools", "linear-sync", "node_modules", npmCIManifestName))
		if err != nil {
			t.Fatalf("read npm manifest: %v", err)
		}
		if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
			t.Fatalf("decode npm manifest: %v", err)
		}
		if manifest.SchemaVersion != npmCIManifestSchemaVersion || len(manifest.Outputs) != len(linearSyncExpectedOutputs) {
			t.Fatalf("unexpected npm manifest: %+v", manifest)
		}

		source.calls = nil
		runner.npmCalls, runner.tscCalls = 0, 0
		runner.linearCalls = nil
		if err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, dependencies); err != nil {
			t.Fatalf("repeat include bootstrap: %v", err)
		}
		if len(source.calls) != 0 || runner.npmCalls != 0 || runner.tscCalls != 0 {
			t.Fatalf("matching repeat was not zero-call: source=%v npm=%d tsc=%d", source.calls, runner.npmCalls, runner.tscCalls)
		}
	})

	t.Run("missing compiled output fails and writes no success manifest", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		addLinearSyncFixture(t, root)
		runner := newLinearRunner(root)
		runner.missingOutput = "dist/src/adapter.js"
		err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, bootstrapDependencies{Source: source, Runner: runner})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED") {
			t.Fatalf("expected missing output failure, got %v", err)
		}
		manifestPath := filepath.Join(root, "tools", "linear-sync", "node_modules", npmCIManifestName)
		if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
			t.Fatalf("failed build wrote success manifest: %v", err)
		}
	})

	t.Run("package lock mutation is restored and writes no success manifest", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		addLinearSyncFixture(t, root)
		lockPath := filepath.Join(root, "tools", "linear-sync", "package-lock.json")
		before, _ := os.ReadFile(lockPath)
		runner := newLinearRunner(root)
		runner.mutateLock = true
		err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, bootstrapDependencies{Source: source, Runner: runner})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_NODE_LOCK_MUTATION") {
			t.Fatalf("expected node lock mutation, got %v", err)
		}
		after, _ := os.ReadFile(lockPath)
		if !bytes.Equal(before, after) {
			t.Fatalf("package-lock changed on return: before=%q after=%q", before, after)
		}
		manifestPath := filepath.Join(root, "tools", "linear-sync", "node_modules", npmCIManifestName)
		if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
			t.Fatalf("mutation wrote success manifest: %v", err)
		}
	})

	t.Run("legacy two-hash manifest forces exact-lock revalidation", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		addLinearSyncFixture(t, root)
		nodeModules := filepath.Join(root, "tools", "linear-sync", "node_modules")
		if err := os.MkdirAll(nodeModules, 0o755); err != nil {
			t.Fatalf("mkdir node_modules: %v", err)
		}
		legacy := `{"package_json_sha256":"legacy","package_lock_sha256":"legacy"}` + "\n"
		if err := os.WriteFile(filepath.Join(nodeModules, npmCIManifestName), []byte(legacy), 0o644); err != nil {
			t.Fatalf("write legacy manifest: %v", err)
		}
		runner := newLinearRunner(root)
		if err := runBootstrap(context.Background(), root, Options{IncludeLinearSync: true}, bootstrapDependencies{Source: source, Runner: runner}); err != nil {
			t.Fatalf("legacy revalidation: %v", err)
		}
		if runner.npmCalls != 1 || runner.tscCalls != 1 {
			t.Fatalf("legacy manifest skipped revalidation: npm=%d tsc=%d", runner.npmCalls, runner.tscCalls)
		}
	})
}
