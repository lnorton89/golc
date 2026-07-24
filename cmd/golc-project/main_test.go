package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lnorton89/golc/internal/command"
)

func TestRunEstablishesResolvedProjectRootBeforeRegistryConstruction(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
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
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(root); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Chdir(previousWorkingDirectory); err != nil {
					t.Errorf("restore working directory: %v", err)
				}
			}()
			environment := testCase.environment(root)
			if environment == "" {
				if err := os.Unsetenv(repoRootEnvName); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Setenv(repoRootEnvName, environment); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Unsetenv(repoRootEnvName) })

			var observed string
			exitCode := runWithRegistryFactory(nil, func() (*command.CommandRegistry, error) {
				observed = os.Getenv(repoRootEnvName)
				return nil, errors.New("stop after observing environment")
			})
			if exitCode != 2 {
				t.Fatalf("exit code = %d, want startup failure 2", exitCode)
			}
			absoluteRoot, err := filepath.Abs(root)
			if err != nil {
				t.Fatal(err)
			}
			// Only the "absent environment" case routes through
			// os.Getwd() in production (resolveProjectRoot), and on macOS
			// Getwd() itself returns the fully symlink-resolved path
			// (/private/var/folders/... rather than t.TempDir()'s own
			// /var/folders/... form, since /var is itself a symlink to
			// /private/var) -- so only that case's expectation needs
			// resolving to match (observed live in
			// cross-platform-mage.yml run 30110425773 on macos-latest;
			// the identical class of bug already fixed in
			// internal/bootstrap/engine_test.go's writeEngineRepository
			// and magefiles/magefile_test.go). The explicit-environment
			// case only goes through filepath.Abs in production, never
			// EvalSymlinks, so its expectation must stay unresolved
			// (observed live in run 30111784838: resolving it
			// unconditionally broke this subtest on macos-latest).
			if testCase.name == "absent environment falls back to working directory" {
				if resolved, err := filepath.EvalSymlinks(absoluteRoot); err == nil {
					absoluteRoot = resolved
				}
			}
			if observed != absoluteRoot {
				t.Fatalf("%s = %q before registry construction, want %q", repoRootEnvName, observed, absoluteRoot)
			}
		})
	}
}
