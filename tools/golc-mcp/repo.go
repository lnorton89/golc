// repo.go resolves the GOLC repository root this server operates over. It
// mirrors cmd/golc-project's GOLC_PROJECT_ROOT convention (so a caller
// that already sets that variable needs no extra configuration), then
// falls back to walking up from the working directory to the nearest
// golc.project.toml so the server behaves correctly however an MCP client
// happens to launch it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	repoRootEnvName    = "GOLC_PROJECT_ROOT"
	rootIndexFileName  = "golc.project.toml"
	maxRootSearchDepth = 32
)

// resolveRepoRoot returns the absolute GOLC repository root.
func resolveRepoRoot() (string, error) {
	if root := os.Getenv(repoRootEnvName); root != "" {
		return filepath.Abs(root)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("golc-mcp: resolve working directory: %w", err)
	}

	dir := cwd
	for i := 0; i < maxRootSearchDepth; i++ {
		if _, err := os.Stat(filepath.Join(dir, rootIndexFileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// No golc.project.toml found above cwd: fall back to cwd itself so
	// tools still start and report a clear error on first use instead of
	// failing at launch.
	return filepath.Abs(cwd)
}
