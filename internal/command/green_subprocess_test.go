package command

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/stretchr/testify/require"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "green-subprocess",
	Summary: "Real compiled golc-project.exe subprocess smoke test (golc.ps1 removal Step 7).",
})

// TestScopeGreenSubprocess is the Go-native replacement for
// tests/acceptance/walking-skeleton.ps1 -Mode green: every other test in
// this repository exercises internal/projectconfig's config
// inspect/set/explain logic in-process (internal/projectconfig/
// local_test.go's TestScopeConfigLocal, resolve_test.go), and
// cmd/golc-project/main_test.go only exercises resolveProjectRoot
// in-process too -- nothing spawns the real compiled binary as a real OS
// subprocess and checks its CLI-serialized output the way a contributor
// or CI actually invokes it. This test does exactly that, against an
// isolated temporary copy of the committed config surface so it never
// writes golc.local.toml into the real repository under test.
func TestScopeGreenSubprocess(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	binary := bootstrap.PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project")
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
		require.NoError(t, err)
		require.True(t, info.Mode().IsRegular(), "pinned golc-project binary not built (run mage Bootstrap first): %v", err)
	}

	workDir := t.TempDir()
	copyFile(t, filepath.Join(root, "golc.project.toml"), filepath.Join(workDir, "golc.project.toml"))
	copyTree(t, filepath.Join(root, "config"), filepath.Join(workDir, "config"))

	t.Run("config inspect runtime is deterministic and reports the committed default", func(t *testing.T) {
		first := runGolcProjectSubprocess(t, binary, workDir, "config", "inspect", "runtime", "--format", "json")
		second := runGolcProjectSubprocess(t, binary, workDir, "config", "inspect", "runtime", "--format", "json")
		require.True(t, bytes.Equal(first, second), "repeated config inspect output was not byte-identical:\nfirst:  %s\nsecond: %s", first, second)

		var document struct {
			Runtime struct {
				LogLevel string `json:"log_level"`
			} `json:"runtime"`
		}
		if err := json.Unmarshal(first, &document); err != nil {
			require.NoError(t, err, "config inspect output is not valid JSON: %v\n%s", err, first)
		}
		require.Equal(t, "info", document.Runtime.LogLevel, "runtime.log_level = %q, want %q", document.Runtime.LogLevel, "info")
	})

	t.Run("config set --local writes golc.local.toml and config explain reports safe deterministic provenance", func(t *testing.T) {
		runGolcProjectSubprocess(t, binary, workDir, "config", "set", "--local", "runtime.log_level", "debug")

		localFile := filepath.Join(workDir, "golc.local.toml")
		if info, err := os.Stat(localFile); err != nil || !info.Mode().IsRegular() {
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular(), "config set --local did not write %s: %v", localFile, err)
		}

		first := runGolcProjectSubprocess(t, binary, workDir, "config", "explain", "runtime.log_level", "--format", "json")
		second := runGolcProjectSubprocess(t, binary, workDir, "config", "explain", "runtime.log_level", "--format", "json")
		require.True(t, bytes.Equal(first, second), "repeated config explain output was not byte-identical:\nfirst:  %s\nsecond: %s", first, second)

		var document map[string]any
		if err := json.Unmarshal(first, &document); err != nil {
			require.NoError(t, err, "config explain output is not valid JSON: %v\n%s", err, first)
		}
		fieldNames := make([]string, 0, len(document))
		for name := range document {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)
		wantFields := []string{"key", "layer", "shadowed", "source", "value"}
		require.Len(t, fieldNames, len(wantFields), "config explain fields = %v, want exactly %v", fieldNames, wantFields)
		for i, name := range fieldNames {
			require.Equal(t, wantFields[i], name, "config explain fields = %v, want exactly %v", fieldNames, wantFields)
		}
		require.Equal(t, "runtime.log_level", document["key"], `config explain "key" = %v, want "runtime.log_level"`, document["key"])
		require.Equal(t, "project-local", document["layer"], `config explain "layer" = %v, want "project-local"`, document["layer"])
		require.Equal(t, "golc.local.toml", document["source"], `config explain "source" = %v, want "golc.local.toml"`, document["source"])
		require.Equal(t, "debug", document["value"], `config explain "value" = %v, want "debug"`, document["value"])
	})
}

// runGolcProjectSubprocess runs binary as a real OS process rooted at
// workDir and returns its captured, trimmed stdout, failing the test on a
// non-zero exit. GOLC_PROJECT_ROOT is set explicitly to the exact workDir
// string rather than relying on the subprocess's own os.Getwd() fallback:
// on Windows, a child process's cwd can resolve to the 8.3 short-name
// alias of a path component (RUNNER~1 for runneradmin) even when it was
// launched with cmd.Dir set to the long form, which would otherwise make
// the file the subprocess writes and the path this test later os.Stats
// disagree on identity (observed live in cross-platform-mage.yml run
// 30137395628 on windows-latest: "config set --local did not write
// ...RUNNER~1...\golc.local.toml" even though the subprocess had just
// written it, under the long-form path, one directory over).
func runGolcProjectSubprocess(t *testing.T, binary, workDir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOLC_PROJECT_ROOT="+workDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		require.NoError(t, err, "%v %v: %v\nstdout: %s\nstderr: %s", binary, args, err, stdout.String(), stderr.String())
	}
	return bytes.TrimSpace(stdout.Bytes())
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	require.NoError(t, err, "read %s: %v", source, err)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		require.NoError(t, err, "mkdir for %s: %v", destination, err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		require.NoError(t, err, "write %s: %v", destination, err)
	}
}

func copyTree(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	err := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(destinationRoot, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		copyFile(t, path, destination)
		return nil
	})
	require.NoError(t, err, "copy tree %s: %v", sourceRoot, err)
}
