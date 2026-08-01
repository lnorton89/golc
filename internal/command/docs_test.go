// docs_test.go proves docs.go's "docs" route contract: it rejects any
// argument and, given a real repository root, regenerates the reference
// pages and reports how many it wrote. internal/docgen's own tests
// (TestScopeDocs) cover discovery/rendering/pruning in depth; these tests
// only prove the route wiring above it.
package command

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestDocsRouteRejectsArguments(t *testing.T) {
	result := runDocs(Request{Route: "docs", Args: []string{"--bogus"}, Root: t.TempDir()})
	require.Equal(t, 2, result.ExitCode, "expected exit code 2, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stderr), "GOLC_DOCS_USAGE", "expected a GOLC_DOCS_USAGE diagnostic, got: %s", result.Stderr)
}

// TestDocsRouteRegeneratesIntoDisposableRoot proves the route wiring
// against a synthetic root (mirroring internal/docgen's own fixture
// package), not the real checkout: a route test must never mutate the
// committed docs/reference or site/src/content/reference trees as a side
// effect of `go test`.
func TestDocsRouteRegeneratesIntoDisposableRoot(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "internal", "widget")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		require.NoError(t, err)
	}
	source := "// Package widget is a fixture used only by docs_test.go.\npackage widget\n"
	if err := os.WriteFile(filepath.Join(packageDir, "widget.go"), []byte(source), 0o644); err != nil {
		require.NoError(t, err)
	}
	catalogDir := filepath.Join(root, "frontend", "src", "shell")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		require.NoError(t, err)
	}
	catalog := `{"schemaVersion":1,"groups":[{"label":"Show","views":[{"id":"show-overview","slug":"show-overview","navLabel":"Overview","title":"Show overview","purpose":"Review the current show.","actions":["Inspect"],"screenshot":"/desktop-views/show-overview.png"}]}]}`
	if err := os.WriteFile(filepath.Join(catalogDir, "desktopViews.json"), []byte(catalog), 0o644); err != nil {
		require.NoError(t, err)
	}

	result := runDocs(Request{Route: "docs", Args: nil, Root: root})
	require.Equal(t, 0, result.ExitCode, "expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	require.Contains(t, string(result.Stdout), "1 package reference page(s) written", "expected a summary reporting one page, got: %s", result.Stdout)
	if _, err := os.Stat(filepath.Join(root, "docs", "reference", "widget.md")); err != nil {
		require.NoError(t, err)
	}
	if _, err := os.Stat(filepath.Join(root, "site", "src", "content", "reference", "widget.md")); err != nil {
		require.NoError(t, err)
	}
}
