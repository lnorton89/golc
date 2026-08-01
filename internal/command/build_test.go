package command

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/stretchr/testify/require"
)

// TestScopeBuildArgs is the exact quick-test marker for scope
// "build-args" (test --quick --scope build-args). It exercises only the
// pure argument-parsing/lookup logic build.go's "--scope" extension adds
// (Plan 01-13) — no archive download, module fetch, tool install, or Node
// toolchain resolution ever happens here, so the registered scope exits 0
// offline.
func TestScopeBuildArgs(t *testing.T) {
	t.Run("no arguments means the bare full build", func(t *testing.T) {
		scope, err := parseBuildArgs(nil)
		require.NoError(t, err)
		require.Equal(t, "", scope, "expected an empty scope, got %q", scope)
	})

	t.Run("--scope <name> selects a named scope", func(t *testing.T) {
		scope, err := parseBuildArgs([]string{"--scope", "linear-sdk"})
		require.NoError(t, err)
		require.Equal(t, "linear-sdk", scope, "expected scope %q, got %q", "linear-sdk", scope)
	})

	t.Run("--scope=<name> selects a named scope", func(t *testing.T) {
		scope, err := parseBuildArgs([]string{"--scope=linear-sdk"})
		require.NoError(t, err)
		require.Equal(t, "linear-sdk", scope, "expected scope %q, got %q", "linear-sdk", scope)
	})

	t.Run("--scope without a value is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--scope"}); err == nil {
			require.Error(t, err)
		}
	})

	t.Run("--scope with an empty value is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--scope", ""}); err == nil {
			require.Error(t, err)
		}
		if _, err := parseBuildArgs([]string{"--scope="}); err == nil {
			require.Error(t, err)
		}
	})

	t.Run("an unsupported argument is rejected", func(t *testing.T) {
		if _, err := parseBuildArgs([]string{"--bogus"}); err == nil {
			require.Error(t, err)
		}
	})

	t.Run("linear-sdk build scope self-registers with the documented directory", func(t *testing.T) {
		registration, found := lookupNodeBuildScope("linear-sdk")
		require.True(t, found, "expected the linear-sdk build scope to be registered")
		require.Equal(t, "tools/linear-sync", registration.Dir, "expected Dir %q, got %q", "tools/linear-sync", registration.Dir)
	})

	t.Run("an unknown build scope is not registered", func(t *testing.T) {
		if _, found := lookupNodeBuildScope("does-not-exist"); found {
			require.False(t, found, "expected an unregistered scope name to be absent")
		}
	})

	t.Run("linear-sdk-operations test scope self-registers with a non-empty command", func(t *testing.T) {
		registration, found := lookupNodeScope("linear-sdk-operations")
		require.True(t, found, "expected the linear-sdk-operations quick-test scope to be registered")
		require.Equal(t, "tools/linear-sync", registration.Dir, "expected Dir %q, got %q", "tools/linear-sync", registration.Dir)
		require.NotEmpty(t, registration.Arguments)
		require.Equal(t, "--test", registration.Arguments[0], "expected registered Node arguments without an executable, got %v", registration.Arguments)
	})

	t.Run("environment upsert replaces stale root entries case-insensitively", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "project")
		got := upsertEnvironment(
			[]string{"PATH=fixture", "golc_project_root=stale", "GOLC_PROJECT_ROOT=older", "CACHE=keep"},
			"GOLC_PROJECT_ROOT",
			root,
		)
		rootEntries := 0
		for _, entry := range got {
			name, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, "GOLC_PROJECT_ROOT") {
				rootEntries++
				require.Equal(t, "GOLC_PROJECT_ROOT", name)
				require.Equal(t, root, value, "root entry = %q, want canonical replacement", entry)
			}
		}
		require.Equal(t, 1, rootEntries, "root entry count = %d in %v", rootEntries, got)
		require.False(t, !slices.Contains(got, "PATH=fixture"))
		require.False(t, !slices.Contains(got, "CACHE=keep"), "unrelated environment entries were not preserved: %v", got)

		t.Setenv("GOLC_PROJECT_ROOT", "stale")
		projectEnvironment := projectGoEnvironment(root)
		found := false
		for _, entry := range projectEnvironment {
			if entry == "GOLC_PROJECT_ROOT="+root {
				found = true
			}
		}
		require.True(t, found, "project Go environment does not contain authoritative root: %v", projectEnvironment)
	})

	t.Run("pinned Go and Node resolvers use the runtime platform layout", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
			require.NoError(t, err)
		}
		manifest := `schema_version = 2

[toolchain.go]
version = "1.26.5"

[toolchain.node]
version = "24.18.0"
`
		if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(manifest), 0o644); err != nil {
			require.NoError(t, err)
		}

		goBase := filepath.Join(root, ".tools", "toolchains", "go", "1.26.5")
		goExecutable := filepath.Join(goBase, bootstrap.PlatformKey(), "go", "bin", bootstrap.ExecutableName("go"))
		if err := os.MkdirAll(filepath.Dir(goExecutable), 0o755); err != nil {
			require.NoError(t, err)
		}
		if err := os.WriteFile(goExecutable, []byte("go\n"), 0o755); err != nil {
			require.NoError(t, err)
		}
		gotGo, err := resolvePinnedGoExecutable(root)
		require.NoError(t, err)
		require.Equal(t, goExecutable, gotGo, "Go executable = %q, want %q", gotGo, goExecutable)

		nodeInstall := filepath.Join(root, ".tools", "toolchains", "node", "24.18.0", bootstrap.PlatformKey())
		extractedRoot := filepath.Join(nodeInstall, "verified-payload-not-derived-from-version")
		var nodeRelative, npmRelative string
		if bootstrap.ExecutableName("node") == "node.exe" {
			nodeRelative = "node.exe"
			npmRelative = filepath.Join("node_modules", "npm", "bin", "npm-cli.js")
		} else {
			nodeRelative = filepath.Join("bin", "node")
			npmRelative = filepath.Join("lib", "node_modules", "npm", "bin", "npm-cli.js")
		}
		nodeExecutable := filepath.Join(extractedRoot, nodeRelative)
		for _, path := range []string{nodeExecutable, filepath.Join(extractedRoot, npmRelative)} {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				require.NoError(t, err)
			}
			if err := os.WriteFile(path, []byte("node\n"), 0o755); err != nil {
				require.NoError(t, err)
			}
		}
		if err := os.WriteFile(filepath.Join(nodeInstall, bootstrap.ManifestName), []byte("{}\n"), 0o644); err != nil {
			require.NoError(t, err)
		}
		gotNode, err := resolvePinnedNodeExecutable(root)
		require.NoError(t, err)
		require.Equal(t, nodeExecutable, gotNode, "Node executable = %q, want %q", gotNode, nodeExecutable)
	})
}

// TestBuildRouteCompilesTheProductionRepository runs the real bare "build"
// route against the actual checkout root with the pinned Go toolchain. It
// exists because every prior build.go test exercised only argument
// parsing/lookup (TestScopeBuildArgs above) — none of them ever invoked
// runBuild end to end, so magefiles/magefile.go's lack of func main() (mage
// supplies its own generated main; the package is never independently
// buildable) silently broke "go build ./..." for the whole module without
// any test catching it. buildablePackages excludes the magefiles import
// path from the bare build sweep for exactly that reason.
func TestBuildRouteCompilesTheProductionRepository(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedGoExecutable(root); err != nil {
		require.NoError(t, err)
	}

	result := runBuild(Request{Route: "build", Root: root})
	require.Equal(t, 0, result.ExitCode, "bare build route exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
}

// TestBuildablePackagesExcludesMagefiles proves the exclusion directly
// against the production package graph, independent of how long the full
// compile above takes.
func TestBuildablePackagesExcludesMagefiles(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	goExecutable, err := resolvePinnedGoExecutable(root)
	require.NoError(t, err)

	packages, err := buildablePackages(goExecutable, root)
	require.NoError(t, err)
	require.NotEmpty(t, packages, "expected at least one buildable package")
	for _, pkg := range packages {
		require.False(t, strings.HasSuffix(pkg, magefilesImportSuffix), "expected the magefiles package to be excluded, found %q", pkg)
	}
}
