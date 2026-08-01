package docgen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/docgen"
)

// repositoryRoot resolves the real checkout root from the package
// directory (pattern set by internal/bootstrap/bootstrap_test.go and
// internal/contracts/fixture_test.go), so discovery runs against the real
// internal/ tree, not a synthetic fixture.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "resolve repository root")
	_, err = os.Stat(filepath.Join(root, "golc.project.toml"))
	require.NoError(t, err, "repository root %q has no golc.project.toml", root)
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
		require.NoError(t, err, "expected no error")

		var bootstrapPage *docgen.Page
		for i := range pages {
			if pages[i].Slug == "bootstrap" {
				bootstrapPage = &pages[i]
				break
			}
		}
		require.NotNil(t, bootstrapPage, "expected a discovered page for internal/bootstrap")
		require.Equal(t, "github.com/lnorton89/golc/internal/bootstrap", bootstrapPage.ImportPath, "unexpected import path")
		require.Equal(t, "bootstrap", bootstrapPage.Name, "unexpected package name")
		require.Contains(t, string(bootstrapPage.Body), "checksum-controlled installation boundary", "expected the real package doc comment in the rendered body, got: %s", bootstrapPage.Body)
	})

	t.Run("discovery is sorted by import path and skips test-only/undocumented directories", func(t *testing.T) {
		pages, err := docgen.Discover(root)
		require.NoError(t, err, "expected no error")
		for i := 1; i < len(pages); i++ {
			require.Less(t, pages[i-1].ImportPath, pages[i].ImportPath, "expected sorted import paths, got %q before %q", pages[i-1].ImportPath, pages[i].ImportPath)
		}
		for _, page := range pages {
			require.False(t, strings.HasSuffix(page.Name, "_test"), "expected no external test package in results, got %q", page.Name)
		}
	})

	t.Run("generation is deterministic and prunes stale pages", func(t *testing.T) {
		tempRoot := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tempRoot, "internal", "widget"), 0o755), "prepare fixture package")
		widgetSource := "// Package widget is a fixture used only by docgen's own test.\npackage widget\n"
		require.NoError(t, os.WriteFile(filepath.Join(tempRoot, "internal", "widget", "widget.go"), []byte(widgetSource), 0o644), "write fixture package")
		catalogPath := filepath.Join(tempRoot, filepath.FromSlash(docgen.DesktopViewsSource))
		require.NoError(t, os.MkdirAll(filepath.Dir(catalogPath), 0o755), "prepare catalog directory")
		require.NoError(t, os.WriteFile(catalogPath, validDesktopCatalog(), 0o644), "write fixture catalog")

		firstRun, err := docgen.GenerateAll(tempRoot)
		require.NoError(t, err, "expected no error")
		require.Len(t, firstRun, 1, "expected exactly one widget page, got: %+v", firstRun)
		require.Equal(t, "widget", firstRun[0].Slug, "expected exactly one widget page, got: %+v", firstRun)

		docsPage := filepath.Join(tempRoot, docgen.ReferenceDocsDir, "widget.md")
		sitePage := filepath.Join(tempRoot, docgen.SiteReferenceDir, "widget.md")
		firstDocsBytes, err := os.ReadFile(docsPage)
		require.NoError(t, err, "expected docs page to exist")
		firstSiteBytes, err := os.ReadFile(sitePage)
		require.NoError(t, err, "expected site copy to exist")
		require.Equal(t, string(firstSiteBytes), string(firstDocsBytes), "expected the docs page and its site copy to be byte-identical")

		_, err = docgen.GenerateAll(tempRoot)
		require.NoError(t, err, "expected no error on second run")
		secondDocsBytes, err := os.ReadFile(docsPage)
		require.NoError(t, err, "expected docs page to still exist")
		require.Equal(t, string(firstDocsBytes), string(secondDocsBytes), "expected repeated generation to be byte-identical")

		require.NoError(t, os.RemoveAll(filepath.Join(tempRoot, "internal", "widget")), "remove fixture package")
		_, err = docgen.GenerateAll(tempRoot)
		require.NoError(t, err, "expected no error on third run")
		_, err = os.Stat(docsPage)
		require.True(t, os.IsNotExist(err), "expected the stale docs page to be removed once its package disappears")
		_, err = os.Stat(sitePage)
		require.True(t, os.IsNotExist(err), "expected the stale site copy to be removed once its package disappears")
	})
}

func TestDesktopViewsCatalog(t *testing.T) {
	valid := validDesktopCatalog()

	t.Run("normalizes valid input byte-identically", func(t *testing.T) {
		first, err := docgen.NormalizeDesktopViews(valid)
		require.NoError(t, err, "normalize valid catalog")
		second, err := docgen.NormalizeDesktopViews(first)
		require.NoError(t, err, "normalize generated catalog")
		require.True(t, bytes.Equal(first, second), "expected byte-identical normalization\nfirst:\n%s\nsecond:\n%s", first, second)
		require.True(t, bytes.Contains(first, []byte(`"generatedBy": "github.com/lnorton89/golc/internal/docgen"`)), "expected generated source marker, got:\n%s", first)
	})

	tests := []struct {
		name    string
		replace string
		with    string
		code    string
	}{
		{"unknown schema", `"schemaVersion": 1`, `"schemaVersion": 2`, "GOLC_DOCGEN_DESKTOP_SCHEMA"},
		{"unknown field", `"schemaVersion": 1`, `"schemaVersion": 1, "surprise": true`, "GOLC_DOCGEN_DESKTOP_DECODE"},
		{"duplicate id", `    }]
  }]`, `    }, {
      "id": "show-overview",
      "slug": "overview-copy",
      "navLabel": "Overview copy",
      "title": "Overview copy",
      "purpose": "Duplicate fixture.",
      "actions": ["Inspect"],
      "screenshot": "/desktop-views/show-overview-copy.png"
    }]
  }]`, "GOLC_DOCGEN_DESKTOP_DUPLICATE"},
		{"invalid screenshot", `"/desktop-views/show-overview.png"`, `"../escape.png"`, "GOLC_DOCGEN_DESKTOP_SCREENSHOT"},
		{"empty purpose", `"Review the open show and enter the guided workflow."`, `""`, "GOLC_DOCGEN_DESKTOP_REQUIRED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := bytes.Replace(valid, []byte(tc.replace), []byte(tc.with), 1)
			_, err := docgen.NormalizeDesktopViews(input)
			require.Error(t, err, "expected %s, got %v", tc.code, err)
			require.Contains(t, err.Error(), tc.code, "expected %s, got %v", tc.code, err)
		})
	}
}

func TestDesktopViewsCatalogOnboarding(t *testing.T) {
	valid := validDesktopCatalogWithOnboarding()

	t.Run("normalizes valid onboarding input byte-identically", func(t *testing.T) {
		first, err := docgen.NormalizeDesktopViews(valid)
		require.NoError(t, err, "normalize valid catalog")
		second, err := docgen.NormalizeDesktopViews(first)
		require.NoError(t, err, "normalize generated catalog")
		require.True(t, bytes.Equal(first, second), "expected byte-identical normalization\nfirst:\n%s\nsecond:\n%s", first, second)
		require.True(t, bytes.Contains(first, []byte(`"onboarding"`)), "expected onboarding section to round-trip, got:\n%s", first)
	})

	tests := []struct {
		name    string
		replace string
		with    string
		code    string
	}{
		{"onboarding id must use the guide- prefix, not its label", `"id": "guide-fixtures"`, `"id": "guided-setup-fixtures"`, "GOLC_DOCGEN_DESKTOP_REFERENCE"},
		{"onboarding screenshot must match its id", `"/desktop-views/guide-fixtures.png"`, `"/desktop-views/guided-setup-fixtures.png"`, "GOLC_DOCGEN_DESKTOP_SCREENSHOT"},
		{"onboarding view still requires content", `"Get at least one fixture ready."`, `""`, "GOLC_DOCGEN_DESKTOP_REQUIRED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := bytes.Replace(valid, []byte(tc.replace), []byte(tc.with), 1)
			_, err := docgen.NormalizeDesktopViews(input)
			require.Error(t, err, "expected %s, got %v", tc.code, err)
			require.Contains(t, err.Error(), tc.code, "expected %s, got %v", tc.code, err)
		})
	}
}

func validDesktopCatalogWithOnboarding() []byte {
	return []byte(`{
  "schemaVersion": 1,
  "groups": [{
    "label": "Show",
    "views": [{
      "id": "show-overview",
      "slug": "overview",
      "navLabel": "Overview",
      "title": "Show overview",
      "purpose": "Review the open show and enter the guided workflow.",
      "actions": ["Start Guided First Show"],
      "concepts": ["Show state"],
      "operatingNotes": ["Uses deterministic fallback data in browser previews."],
      "screenshot": "/desktop-views/show-overview.png"
    }]
  }],
  "onboarding": {
    "label": "Guided Setup",
    "views": [{
      "id": "guide-fixtures",
      "slug": "guide-fixtures",
      "navLabel": "Fixtures",
      "title": "Guided Setup: Fixtures",
      "purpose": "Get at least one fixture ready.",
      "actions": ["Continue to the Fixture Library"],
      "screenshot": "/desktop-views/guide-fixtures.png"
    }]
  }
}`)
}

func validDesktopCatalog() []byte {
	return []byte(`{
  "schemaVersion": 1,
  "groups": [{
    "label": "Show",
    "views": [{
      "id": "show-overview",
      "slug": "overview",
      "navLabel": "Overview",
      "title": "Show overview",
      "purpose": "Review the open show and enter the guided workflow.",
      "actions": ["Start Guided First Show"],
      "concepts": ["Show state"],
      "operatingNotes": ["Uses deterministic fallback data in browser previews."],
      "screenshot": "/desktop-views/show-overview.png"
    }]
  }]
}`)
}
