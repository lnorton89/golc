package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type engineFakeSource struct {
	payload map[string][]byte
	calls   []string
}

func (source *engineFakeSource) Fetch(rawURL string) (io.ReadCloser, error) {
	source.calls = append(source.calls, rawURL)
	payload, ok := source.payload[rawURL]
	if !ok {
		return nil, fmt.Errorf("unexpected source URL %q", rawURL)
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

type processCall struct {
	executable string
	dir        string
	args       []string
	env        map[string]string
}

type engineFakeRunner struct {
	calls       []processCall
	moduleGraph string
	mutateLock  bool
}

func (runner *engineFakeRunner) Run(_ context.Context, request processRequest) ([]byte, error) {
	call := processCall{
		executable: request.Executable,
		dir:        request.Dir,
		args:       append([]string(nil), request.Args...),
		env:        cloneEngineTestMap(request.Env),
	}
	runner.calls = append(runner.calls, call)
	if runner.mutateLock && len(runner.calls) == 1 {
		if err := os.WriteFile(filepath.Join(request.Dir, "go.mod"), []byte("mutated\n"), 0o644); err != nil {
			return nil, err
		}
	}
	if len(request.Args) >= 4 && request.Args[0] == "build" && request.Args[1] == "-trimpath" && request.Args[2] == "-o" {
		if err := os.MkdirAll(filepath.Dir(request.Args[3]), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(request.Args[3], []byte("built\n"), 0o755); err != nil {
			return nil, err
		}
	}
	if strings.Join(request.Args, " ") == "list -m all" {
		return []byte(runner.moduleGraph), nil
	}
	return nil, nil
}

func cloneEngineTestMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func platformToolArchive(t *testing.T, root, tool, version string) (path string, digest string, archiveRoot string) {
	t.Helper()
	layout, err := platformArchiveLayout(tool, version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("platformArchiveLayout: %v", err)
	}
	archiveRoot = layout.Root
	switch layout.Format {
	case ".zip":
		returnPath, returnDigest := buildZipEntries(t, root, layout.FileName, []testArchiveEntry{
			{Name: filepath.ToSlash(filepath.Join(layout.Root, layout.Executable)), Body: "executable\n", Mode: 0o755},
		})
		return returnPath, returnDigest, archiveRoot
	case ".tar.gz":
		returnPath, returnDigest := buildTarGzEntries(t, root, layout.FileName, []testArchiveEntry{
			{Name: filepath.ToSlash(filepath.Join(layout.Root, layout.Executable)), Body: "executable\n", Mode: 0o755},
		})
		return returnPath, returnDigest, archiveRoot
	default:
		t.Fatalf("unsupported test archive format %q", layout.Format)
		return "", "", ""
	}
}

func writeEngineRepository(t *testing.T) (root string, source *engineFakeSource, goURL string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "golc-project"), 0o755); err != nil {
		t.Fatalf("mkdir command: %v", err)
	}
	goArchive, goDigest, _ := platformToolArchive(t, root, "go", "1.26.5")
	fixtureArchive, fixtureDigest := buildZipEntries(t, root, "fixture.zip", []testArchiveEntry{
		{Name: "bin/fixture", Body: "fixture\n", Mode: 0o755},
	})
	goURL = "https://go.dev/dl/" + filepath.Base(goArchive)
	fixtureURL := "https://fixtures.example.invalid/tool/" + filepath.Base(fixtureArchive)
	manifest := fmt.Sprintf(`schema_version = 2

[cache]
downloads = ".tools/cache/downloads"
gomodcache = ".tools/cache/go-mod"
gocache = ".tools/cache/go-build"

[tools.fixture]
archive_url = %q
archive_sha256 = %q
official_host = "fixtures.example.invalid"
official_path_prefix = "/tool/"

[toolchain.go]
version = "1.26.5"
official_host = "go.dev"
official_path_prefix = "/dl/"

[toolchain.go.platforms.%q]
archive_url = %q
archive_sha256 = %q
`, fixtureURL, fixtureDigest, PlatformKey(), goURL, goDigest)
	if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/test\n\ngo 1.26.5\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("sum\n"), 0o644); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}
	goBytes, _ := os.ReadFile(goArchive)
	fixtureBytes, _ := os.ReadFile(fixtureArchive)
	source = &engineFakeSource{payload: map[string][]byte{
		goURL:      goBytes,
		fixtureURL: fixtureBytes,
	}}
	return root, source, goURL
}

func TestScopeBootstrapEngine(t *testing.T) {
	t.Run("PlatformKey and pure platform layouts are exact", func(t *testing.T) {
		if got, want := PlatformKey(), runtime.GOOS+"-"+runtime.GOARCH; got != want {
			t.Fatalf("PlatformKey() = %q, want %q", got, want)
		}
		cases := []struct {
			tool, version, goos, goarch string
			file, root, executable      string
		}{
			{"go", "1.26.5", "windows", "amd64", "go1.26.5.windows-amd64.zip", "go", filepath.Join("bin", "go.exe")},
			{"go", "1.26.5", "linux", "amd64", "go1.26.5.linux-amd64.tar.gz", "go", filepath.Join("bin", "go")},
			{"go", "1.26.5", "darwin", "arm64", "go1.26.5.darwin-arm64.tar.gz", "go", filepath.Join("bin", "go")},
			{"node", "24.18.0", "windows", "amd64", "node-v24.18.0-win-x64.zip", "node-v24.18.0-win-x64", "node.exe"},
			{"node", "24.18.0", "linux", "amd64", "node-v24.18.0-linux-x64.tar.gz", "node-v24.18.0-linux-x64", filepath.Join("bin", "node")},
			{"node", "24.18.0", "darwin", "arm64", "node-v24.18.0-darwin-arm64.tar.gz", "node-v24.18.0-darwin-arm64", filepath.Join("bin", "node")},
		}
		for _, testCase := range cases {
			layout, err := platformArchiveLayout(testCase.tool, testCase.version, testCase.goos, testCase.goarch)
			if err != nil {
				t.Fatalf("%s/%s: %v", testCase.goos, testCase.tool, err)
			}
			if layout.FileName != testCase.file || layout.Root != testCase.root || layout.Executable != testCase.executable {
				t.Fatalf("%s/%s: got %+v", testCase.goos, testCase.tool, layout)
			}
		}
		if got, want := ExecutableName("golc-project"), "golc-project"+map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]; got != want {
			t.Fatalf("ExecutableName(golc-project) = %q, want %q", got, want)
		}
		for _, unsafe := range []string{"", ".", "..", "bin/golc-project", `bin\golc-project`} {
			if got := ExecutableName(unsafe); got != "" {
				t.Fatalf("ExecutableName(%q) = %q, want rejection", unsafe, got)
			}
		}
		installRoot := filepath.Join("repo", ".tools", "installs", "golc_project")
		if got, want := PlatformExecutablePath(installRoot, "golc-project"), filepath.Join(installRoot, PlatformKey(), "bin", ExecutableName("golc-project")); got != want {
			t.Fatalf("PlatformExecutablePath() = %q, want %q", got, want)
		}
		if got := PlatformExecutablePath(installRoot, "../golc-project"); got != "" {
			t.Fatalf("PlatformExecutablePath accepted unsafe base: %q", got)
		}
	})

	t.Run("Node installation is discovered by verified filesystem shape", func(t *testing.T) {
		writeNodePayload := func(t *testing.T, installDir, rootName string) NodeInstallation {
			t.Helper()
			layout, err := platformArchiveLayout("node", "24.18.0", runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatalf("node layout: %v", err)
			}
			root := filepath.Join(installDir, rootName)
			executable := filepath.Join(root, layout.Executable)
			npmCLI := filepath.Join(root, layout.NPMCLI)
			for _, path := range []string{executable, npmCLI} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", path, err)
				}
				if err := os.WriteFile(path, []byte("fixture\n"), 0o755); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
			}
			if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
				t.Fatalf("write install manifest: %v", err)
			}
			return NodeInstallation{Root: root, Executable: executable, NPMCLI: npmCLI}
		}

		t.Run("accepts one non-derived payload directory", func(t *testing.T) {
			installDir := t.TempDir()
			want := writeNodePayload(t, installDir, "verified-payload-with-arbitrary-name")
			got, err := ResolveNodeInstallation(installDir)
			if err != nil {
				t.Fatalf("ResolveNodeInstallation: %v", err)
			}
			if got != want {
				t.Fatalf("ResolveNodeInstallation = %+v, want %+v", got, want)
			}
		})

		tests := []struct {
			name  string
			setup func(*testing.T, string)
		}{
			{"zero directories", func(t *testing.T, installDir string) {
				if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
			{"multiple directories", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "one")
				if err := os.MkdirAll(filepath.Join(installDir, "two"), 0o755); err != nil {
					t.Fatal(err)
				}
			}},
			{"unexpected top-level file", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "payload")
				if err := os.WriteFile(filepath.Join(installDir, "unexpected.txt"), []byte("no\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
			{"missing node executable", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				if err := os.Remove(want.Executable); err != nil {
					t.Fatal(err)
				}
			}},
			{"missing npm cli", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				if err := os.Remove(want.NPMCLI); err != nil {
					t.Fatal(err)
				}
			}},
		}
		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				installDir := t.TempDir()
				testCase.setup(t, installDir)
				_, err := ResolveNodeInstallation(installDir)
				if err == nil || !strings.Contains(err.Error(), "GOLC_NODE_TOOLCHAIN_MISSING") {
					t.Fatalf("expected stable Node diagnostic, got %v", err)
				}
			})
		}

		t.Run("rejects top-level symlink", func(t *testing.T) {
			installDir := t.TempDir()
			target := filepath.Join(t.TempDir(), "payload")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(installDir, "payload-link")); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := ResolveNodeInstallation(installDir)
			if err == nil || !strings.Contains(err.Error(), "GOLC_NODE_TOOLCHAIN_MISSING") {
				t.Fatalf("expected stable Node diagnostic, got %v", err)
			}
		})
	})

	t.Run("production manifest configures only windows-amd64 archives", func(t *testing.T) {
		root := filepath.Join("..", "..")
		document, _, err := readBootstrapManifest(root)
		if err != nil {
			t.Fatalf("read production manifest: %v", err)
		}
		for _, tool := range []string{"go", "node"} {
			parent, ok := document.Toolchain[tool]
			if !ok {
				t.Fatalf("production manifest missing toolchain.%s", tool)
			}
			if len(parent.Platforms) != 1 {
				t.Fatalf("toolchain.%s platforms = %v, want only windows-amd64", tool, parent.Platforms)
			}
			if _, ok := parent.Platforms["windows-amd64"]; !ok {
				t.Fatalf("toolchain.%s does not explicitly configure windows-amd64", tool)
			}
		}
	})

	t.Run("complete Go bootstrap uses pinned tools environment and process order", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		runner := &engineFakeRunner{moduleGraph: strings.Join([]string{
			"example.invalid/test",
			"github.com/BurntSushi/toml v1.6.0",
			"github.com/invopop/jsonschema v0.14.0",
		}, "\n") + "\n"}
		dependencies := bootstrapDependencies{Source: source, Runner: runner}
		if err := runBootstrap(context.Background(), root, Options{}, dependencies); err != nil {
			t.Fatalf("runBootstrap: %v", err)
		}
		if len(source.calls) != 2 {
			t.Fatalf("source calls = %v, want generic tool plus Go", source.calls)
		}
		wantArgs := [][]string{
			{"mod", "download", "all"},
			{"mod", "verify"},
			{"list", "-m", "all"},
			{"test", "-count=1", "./internal/bootstrap/"},
		}
		if len(runner.calls) != 5 {
			t.Fatalf("process calls = %d, want 5: %+v", len(runner.calls), runner.calls)
		}
		for index, args := range wantArgs {
			if got := strings.Join(runner.calls[index].args, "\x00"); got != strings.Join(args, "\x00") {
				t.Fatalf("call %d args = %v, want %v", index, runner.calls[index].args, args)
			}
		}
		build := runner.calls[4]
		if len(build.args) != 5 || strings.Join(build.args[:3], " ") != "build -trimpath -o" || build.args[4] != "./cmd/golc-project" {
			t.Fatalf("unexpected build args: %v", build.args)
		}
		for _, call := range runner.calls {
			if call.dir != root {
				t.Fatalf("working directory = %q, want %q", call.dir, root)
			}
			for _, key := range []string{"GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOBIN", "GOFLAGS"} {
				if call.env[key] == "" {
					t.Fatalf("call missing environment %s: %v", key, call.env)
				}
			}
			if !filepath.IsAbs(call.executable) {
				t.Fatalf("executable is not absolute: %q", call.executable)
			}
		}
		moduleRecord, err := os.ReadFile(filepath.Join(root, ".tools", "manifest", "go-modules.txt"))
		if err != nil || string(moduleRecord) != runner.moduleGraph {
			t.Fatalf("module record: err=%v bytes=%q", err, moduleRecord)
		}
		if _, err := os.Stat(PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project")); err != nil {
			t.Fatalf("built project command missing: %v", err)
		}

		source.calls = nil
		runner.calls = nil
		if err := runBootstrap(context.Background(), root, Options{}, dependencies); err != nil {
			t.Fatalf("second runBootstrap: %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("matching manifests consulted source: %v", source.calls)
		}
	})

	t.Run("Go lock mutation is diagnosed and original bytes are restored", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		before, _ := os.ReadFile(filepath.Join(root, "go.mod"))
		runner := &engineFakeRunner{
			moduleGraph: "github.com/BurntSushi/toml v1.6.0\ngithub.com/invopop/jsonschema v0.14.0\n",
			mutateLock:  true,
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: runner})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_LOCK_MUTATION") {
			t.Fatalf("expected lock mutation diagnostic, got %v", err)
		}
		after, _ := os.ReadFile(filepath.Join(root, "go.mod"))
		if !bytes.Equal(before, after) {
			t.Fatalf("go.mod changed on return: before=%q after=%q", before, after)
		}
	})

	t.Run("mismatched configured platform fails before source or install work", func(t *testing.T) {
		root, source, goURL := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		wrongURL := strings.Replace(goURL, filepath.Base(goURL), "go1.26.5.not-this-platform.zip", 1)
		raw = bytes.Replace(raw, []byte(goURL), []byte(wrongURL), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_PLATFORM_MISMATCH") {
			t.Fatalf("expected platform mismatch, got %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("platform mismatch consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("platform mismatch created .tools: %v", err)
		}
	})

	t.Run("missing current Go platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.go.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.go.platforms."unconfigured-platform"]`), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.go.platforms.%q]`, PlatformKey())
		if err == nil || !strings.Contains(err.Error(), required) {
			t.Fatalf("expected missing platform diagnostic naming %s, got %v", required, err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("missing platform consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("missing platform created .tools: %v", err)
		}
	})

	t.Run("obsolete generic archive locator is rejected before effects", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		raw = bytes.Replace(raw, []byte("archive_url"), []byte("archive_uri"), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		if err == nil || !strings.Contains(err.Error(), "archive_uri") {
			t.Fatalf("expected unsupported archive_uri diagnostic, got %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("obsolete locator consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("obsolete locator created .tools: %v", err)
		}
	})
}
