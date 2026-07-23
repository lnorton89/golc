// docs_test.go proves docs.go's "docs" route contract: it rejects any
// argument and, given a real repository root, regenerates the reference
// pages and reports how many it wrote. internal/docgen's own tests
// (TestScopeDocs) cover discovery/rendering/pruning in depth; these tests
// only prove the route wiring above it.
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsRouteRejectsArguments(t *testing.T) {
	result := runDocs(Request{Route: "docs", Args: []string{"--bogus"}, Root: t.TempDir()})
	if result.ExitCode != 2 {
		t.Fatalf("expected exit code 2, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_DOCS_USAGE") {
		t.Fatalf("expected a GOLC_DOCS_USAGE diagnostic, got: %s", result.Stderr)
	}
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
		t.Fatalf("prepare fixture package: %v", err)
	}
	source := "// Package widget is a fixture used only by docs_test.go.\npackage widget\n"
	if err := os.WriteFile(filepath.Join(packageDir, "widget.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture package: %v", err)
	}

	result := runDocs(Request{Route: "docs", Args: nil, Root: root})
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stdout), "1 package reference page(s) written") {
		t.Fatalf("expected a summary reporting one page, got: %s", result.Stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "reference", "widget.md")); err != nil {
		t.Fatalf("expected the widget page to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "site", "src", "content", "reference", "widget.md")); err != nil {
		t.Fatalf("expected the widget page's site copy to exist: %v", err)
	}
}
