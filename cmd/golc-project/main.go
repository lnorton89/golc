// Command golc-project is the pinned project-local CLI that golc.ps1
// delegates every normal subcommand to. It performs no route wiring of its
// own: command files self-register through the command package's
// declaration entrypoints (D-03), and this entrypoint only imports them,
// builds the default registry, and applies the stable result-to-exit
// mapping (0 success, 1 command failure, 2 routing/usage/startup failure).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/command"
	// Self-registering command files. Adding a command means adding an
	// import here at most — never editing a central route switch.
	_ "github.com/lnorton89/golc/internal/projectconfig"
)

// repoRootEnvName is set by golc.ps1 so command behavior is independent of
// the caller's working directory.
const repoRootEnvName = "GOLC_PROJECT_ROOT"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	return runWithRegistryFactory(arguments, command.NewDefaultCommandRegistry)
}

func runWithRegistryFactory(arguments []string, factory func() (*command.CommandRegistry, error)) int {
	root, err := resolveProjectRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := os.Setenv(repoRootEnvName, root); err != nil {
		fmt.Fprintf(os.Stderr, "GOLC_PROJECT_ROOT_INVALID: %v\n", err)
		return 2
	}
	registry, err := factory()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	result := registry.Execute(command.Request{Root: root, Args: arguments})
	return command.WriteResult(os.Stdout, os.Stderr, result)
}

// resolveProjectRoot prefers the shim-provided repository root and falls
// back to the current working directory.
func resolveProjectRoot() (string, error) {
	root := os.Getenv(repoRootEnvName)
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("GOLC_PROJECT_ROOT_INVALID: %v", err)
		}
		root = workingDirectory
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_PROJECT_ROOT_INVALID: %q: %v", root, err)
	}
	return absolute, nil
}
