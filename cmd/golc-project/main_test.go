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
			if err := os.Chdir(root); err != nil {
				t.Fatal(err)
			}
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
			if observed != absoluteRoot {
				t.Fatalf("%s = %q before registry construction, want %q", repoRootEnvName, observed, absoluteRoot)
			}
		})
	}
}
