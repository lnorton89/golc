package docgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/docgen"
)

// repositoryRoot resolves the real checkout root from the package
// directory (pattern set by internal/bootstrap/bootstrap_test.go and
// internal/contracts/fixture_test.go), so discovery runs against the real
// internal/ tree, not a synthetic fixture.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "golc.project.toml")); err != nil {
		t.Fatalf("repository root %q has no golc.project.toml: %v", root, err)
	}
	return root
}

// TestScopeDocs is the exact quick-test marker for scope "docs" (test
// --quick --scope docs). Discovery and generation only ever read the
// checked-out internal/ tree and write under docs/reference and
// site/src/content/reference, so the registered scope exits 0 offline.
func TestScopeDocs(t *testing.T) {
	root := repositoryRoot(t)

	t.Run("discovery finds a known documented package", func(t *testing.T) {
		pages, err := docgen.Discover(root)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var bootstrapPage *docgen.Page
		for i := range pages {
			if pages[i].Slug == "bootstrap" {
				bootstrapPage = &pages[i]
				break
			}
		}
		if bootstrapPage == nil {
			t.Fatal("expected a discovered page for internal/bootstrap")
		}
		if bootstrapPage.ImportPath != "github.com/lnorton89/golc/internal/bootstrap" {
			t.Fatalf("unexpected import path %q", bootstrapPage.ImportPath)
		}
		if bootstrapPage.Name != "bootstrap" {
			t.Fatalf("unexpected package name %q", bootstrapPage.Name)
		}
		if !strings.Contains(string(bootstrapPage.Body), "checksum-controlled installation boundary") {
			t.Fatalf("expected the real package doc comment in the rendered body, got: %s", bootstrapPage.Body)
		}
	})

	t.Run("discovery is sorted by import path and skips test-only/undocumented directories", func(t *testing.T) {
		pages, err := docgen.Discover(root)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		for i := 1; i < len(pages); i++ {
			if pages[i-1].ImportPath >= pages[i].ImportPath {
				t.Fatalf("expected sorted import paths, got %q before %q", pages[i-1].ImportPath, pages[i].ImportPath)
			}
		}
		for _, page := range pages {
			if strings.HasSuffix(page.Name, "_test") {
				t.Fatalf("expected no external test package in results, got %q", page.Name)
			}
		}
	})

	t.Run("generation is deterministic and prunes stale pages", func(t *testing.T) {
		tempRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tempRoot, "internal", "widget"), 0o755); err != nil {
			t.Fatalf("prepare fixture package: %v", err)
		}
		widgetSource := "// Package widget is a fixture used only by docgen's own test.\npackage widget\n"
		if err := os.WriteFile(filepath.Join(tempRoot, "internal", "widget", "widget.go"), []byte(widgetSource), 0o644); err != nil {
			t.Fatalf("write fixture package: %v", err)
		}

		firstRun, err := docgen.GenerateAll(tempRoot)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(firstRun) != 1 || firstRun[0].Slug != "widget" {
			t.Fatalf("expected exactly one widget page, got: %+v", firstRun)
		}

		docsPage := filepath.Join(tempRoot, docgen.ReferenceDocsDir, "widget.md")
		sitePage := filepath.Join(tempRoot, docgen.SiteReferenceDir, "widget.md")
		firstDocsBytes, err := os.ReadFile(docsPage)
		if err != nil {
			t.Fatalf("expected docs page to exist: %v", err)
		}
		firstSiteBytes, err := os.ReadFile(sitePage)
		if err != nil {
			t.Fatalf("expected site copy to exist: %v", err)
		}
		if string(firstDocsBytes) != string(firstSiteBytes) {
			t.Fatal("expected the docs page and its site copy to be byte-identical")
		}

		if _, err := docgen.GenerateAll(tempRoot); err != nil {
			t.Fatalf("expected no error on second run, got: %v", err)
		}
		secondDocsBytes, err := os.ReadFile(docsPage)
		if err != nil {
			t.Fatalf("expected docs page to still exist: %v", err)
		}
		if string(firstDocsBytes) != string(secondDocsBytes) {
			t.Fatal("expected repeated generation to be byte-identical")
		}

		if err := os.RemoveAll(filepath.Join(tempRoot, "internal", "widget")); err != nil {
			t.Fatalf("remove fixture package: %v", err)
		}
		if _, err := docgen.GenerateAll(tempRoot); err != nil {
			t.Fatalf("expected no error on third run, got: %v", err)
		}
		if _, err := os.Stat(docsPage); !os.IsNotExist(err) {
			t.Fatal("expected the stale docs page to be removed once its package disappears")
		}
		if _, err := os.Stat(sitePage); !os.IsNotExist(err) {
			t.Fatal("expected the stale site copy to be removed once its package disappears")
		}
	})
}
