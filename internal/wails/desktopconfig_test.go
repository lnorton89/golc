package wails_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	golcwails "github.com/lnorton89/golc/internal/wails"
)

// TestResolveProjectRoot mirrors cmd/golc-project/main_test.go's own
// resolveProjectRoot coverage, adapted for ResolveProjectRoot's silent-""
// (not error) failure contract.
func TestResolveProjectRoot(t *testing.T) {
	previousWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(previousWorkingDirectory) })
	t.Cleanup(func() { _ = os.Unsetenv("GOLC_PROJECT_ROOT") })

	tests := []struct {
		name        string
		environment func(root string) string
	}{
		{
			name: "absent environment falls back to working directory",
			environment: func(string) string {
				return ""
			},
		},
		{
			name: "non-normalized environment is canonicalized",
			environment: func(root string) string {
				return root + string(filepath.Separator) + "."
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Chdir(root))
			// Restore the working directory before this subtest's own
			// t.TempDir() cleanup runs (t.Cleanup is LIFO, so registering
			// this after TempDir() makes it fire first): on Windows,
			// RemoveAll on a directory that is still the process's
			// current working directory fails with "The process cannot
			// access the file because it is being used by another
			// process."
			t.Cleanup(func() { _ = os.Chdir(previousWorkingDirectory) })

			environment := testCase.environment(root)
			if environment == "" {
				require.NoError(t, os.Unsetenv("GOLC_PROJECT_ROOT"))
			} else {
				require.NoError(t, os.Setenv("GOLC_PROJECT_ROOT", environment))
			}

			// filepath.EvalSymlinks on both sides: os.Getwd() itself can
			// return a fully symlink-resolved or short-name-aliased form
			// of root that differs byte-for-byte from t.TempDir()'s own
			// string even though both name the same directory (observed
			// on macOS/Windows CI -- see cmd/golc-project/main_test.go's
			// identical comment for the two concrete failure modes this
			// guards against).
			wantRoot, err := filepath.Abs(root)
			require.NoError(t, err)
			if resolved, evalErr := filepath.EvalSymlinks(wantRoot); evalErr == nil {
				wantRoot = resolved
			}

			got := golcwails.ResolveProjectRoot()
			if resolved, evalErr := filepath.EvalSymlinks(got); evalErr == nil {
				got = resolved
			}
			assert.Equal(t, wantRoot, got)
		})
	}
}

func TestEnvInt(t *testing.T) {
	const name = "GOLC_TEST_ENV_INT"
	t.Cleanup(func() { _ = os.Unsetenv(name) })

	tests := []struct {
		name     string
		value    string
		unset    bool
		fallback int
		want     int
	}{
		{name: "unset returns fallback", unset: true, fallback: 7, want: 7},
		{name: "empty returns fallback", value: "", fallback: 3, want: 3},
		{name: "valid integer is parsed", value: "42", fallback: 0, want: 42},
		{name: "negative integer is parsed", value: "-5", fallback: 0, want: -5},
		{name: "non-numeric falls back", value: "not-a-number", fallback: 9, want: 9},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.unset {
				require.NoError(t, os.Unsetenv(name))
			} else {
				require.NoError(t, os.Setenv(name, testCase.value))
			}
			assert.Equal(t, testCase.want, golcwails.EnvInt(name, testCase.fallback))
		})
	}
}

// TestEnvIntStrconvParity pins EnvInt's parser to strconv.Atoi exactly
// (base-10, no leading "+"/whitespace tolerance beyond what Atoi itself
// allows), so a future change to a more permissive parser is a visible,
// deliberate diff here rather than a silent behavior change.
func TestEnvIntStrconvParity(t *testing.T) {
	const name = "GOLC_TEST_ENV_INT_PARITY"
	t.Cleanup(func() { _ = os.Unsetenv(name) })

	for _, raw := range []string{"0", "123", "-123", "007", "1x", " 1", ""} {
		t.Run(raw, func(t *testing.T) {
			require.NoError(t, os.Setenv(name, raw))
			want, atoiErr := strconv.Atoi(raw)
			if raw == "" {
				want = -1 // unreachable: empty is handled before Atoi
			}
			got := golcwails.EnvInt(name, -1)
			if raw == "" || atoiErr != nil {
				assert.Equal(t, -1, got, "expected fallback for %q", raw)
			} else {
				assert.Equal(t, want, got, "expected strconv.Atoi(%q) parity", raw)
			}
		})
	}
}
