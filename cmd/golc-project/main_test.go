package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

func TestRunEstablishesResolvedProjectRootBeforeRegistryConstruction(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	tests := []struct {
		name        string
		environment func(string) string
	}{
		{
			name: "absent environment falls back to working directory",
			environment: func(string) string {
				return ""
			},
		},
		{
			name: "valid non-normalized environment is canonicalized",
			environment: func(root string) string {
				return root + string(filepath.Separator) + "."
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			previousWorkingDirectory, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(root))
			defer func() {
				assert.NoError(t, os.Chdir(previousWorkingDirectory), "restore working directory")
			}()
			environment := testCase.environment(root)
			if environment == "" {
				require.NoError(t, os.Unsetenv(repoRootEnvName))
			} else {
				require.NoError(t, os.Setenv(repoRootEnvName, environment))
			}
			t.Cleanup(func() { _ = os.Unsetenv(repoRootEnvName) })

			var observed string
			exitCode := runWithRegistryFactory(nil, func() (*command.CommandRegistry, error) {
				observed = os.Getenv(repoRootEnvName)
				return nil, errors.New("stop after observing environment")
			})
			require.Equal(t, 2, exitCode, "exit code, want startup failure 2")
			absoluteRoot, err := filepath.Abs(root)
			require.NoError(t, err)
			// Only the "absent environment" case routes through
			// os.Getwd() in production (resolveProjectRoot). On macOS,
			// Getwd() itself returns the fully symlink-resolved path
			// (/private/var/folders/... rather than t.TempDir()'s own
			// /var/folders/... form, since /var is itself a symlink to
			// /private/var); on Windows, os.Getwd() can instead report the
			// 8.3 short-name alias of a path component even when Chdir was
			// given the long form (observed live in run 30112915317 on
			// windows-latest: GOLC_PROJECT_ROOT came back with RUNNER~1
			// where the expectation had runneradmin). filepath.EvalSymlinks
			// resolves both of these (it walks the real filesystem, which
			// also canonicalizes short names on Windows), so both sides of
			// this one comparison are resolved through it -- the identical
			// class of bug already fixed in
			// internal/bootstrap/engine_test.go's writeEngineRepository
			// and magefiles/magefile_test.go. The explicit-environment
			// case only goes through filepath.Abs in production, never
			// EvalSymlinks, so its expectation must stay unresolved
			// (observed live in run 30111784838: resolving it
			// unconditionally broke that subtest on macos-latest).
			if testCase.name == "absent environment falls back to working directory" {
				if resolved, err := filepath.EvalSymlinks(absoluteRoot); err == nil {
					absoluteRoot = resolved
				}
				if resolved, err := filepath.EvalSymlinks(observed); err == nil {
					observed = resolved
				}
			}
			require.Equal(t, absoluteRoot, observed, "%s before registry construction", repoRootEnvName)
		})
	}
}
