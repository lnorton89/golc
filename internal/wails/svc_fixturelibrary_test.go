// svc_fixturelibrary_test.go proves FixtureLibraryService's ListLocal/
// Inspect bindings (09-01-PLAN.md Task 1 RED / Task 2-3 GREEN):
// ListLocal projects internal/fixture.ListDirectory's scan into rows
// sorted ascending by StableKey, treats a not-yet-created fixtures
// directory as an empty library rather than an error, and never marshals
// Rows as JSON null -- mirrors
// TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues' exact
// real-JSON-marshal-boundary discipline (a hand-authored frontend mock
// can never catch this class of bug, since it never round-trips through
// real Go JSON encoding). Inspect proves the binding forwards to the
// real, already-tested "fixture inspect" route rather than
// re-implementing decode/pin/normalize (T-09-01-03).
package wails

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/fixture/ofl"
)

const validFixtureYAMLForLibraryTest = `schema_version: 1
manufacturer: Chauvet
model: SlimPAR Pro
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
    comment: Master dimmer
  - type: color
    range: [0, 1]
    comment: RGB color mix
`

const secondValidFixtureYAMLForLibraryTest = `schema_version: 1
manufacturer: American DJ
model: Mega Par
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
    comment: Master dimmer
  - type: color
    range: [0, 1]
    comment: RGB color mix
`

// newTestFixtureLibraryService mirrors newTestShowService's identical
// seed-then-exercise-bindings convention: root is a fresh per-test temp
// dir, fixturesDir a subdirectory of it.
func newTestFixtureLibraryService(t *testing.T) (*FixtureLibraryService, string, string) {
	t.Helper()
	root := t.TempDir()
	fixturesDir := filepath.Join(root, "my-fixtures")
	if err := os.Mkdir(fixturesDir, 0o755); err != nil {
		t.Fatalf("Mkdir(fixturesDir): %v", err)
	}
	return NewFixtureLibraryService("", root, fixturesDir), root, fixturesDir
}

func writeLibraryTestFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
}

// TestFixtureLibraryServiceListLocalProjectsRowsSortedByStableKey proves
// ListLocal projects two valid fixtures into rows sorted ascending by
// StableKey, each carrying source "local" and status "valid".
func TestFixtureLibraryServiceListLocalProjectsRowsSortedByStableKey(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	writeLibraryTestFixture(t, fixturesDir, "slimpar.yaml", validFixtureYAMLForLibraryTest)
	writeLibraryTestFixture(t, fixturesDir, "megapar.yaml", secondValidFixtureYAMLForLibraryTest)

	view, err := svc.ListLocal()
	if err != nil {
		t.Fatalf("ListLocal: %v", err)
	}
	if len(view.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(view.Rows), view.Rows)
	}
	if view.Rows[0].StableKey >= view.Rows[1].StableKey {
		t.Fatalf("expected rows sorted ascending by StableKey, got %q then %q", view.Rows[0].StableKey, view.Rows[1].StableKey)
	}
	for _, row := range view.Rows {
		if row.Source != "local" {
			t.Fatalf("expected row source %q, got %q", "local", row.Source)
		}
		if row.Status != "valid" {
			t.Fatalf("expected row status %q, got %q", "valid", row.Status)
		}
	}
}

// TestFixtureLibraryServiceListLocalMissingDirectoryIsEmptyNotError
// proves a service pointed at a directory that has never been created
// returns a view whose Rows is a non-nil empty slice and a nil error --
// an operator who hasn't imported anything yet sees an empty library, not
// a broken one.
func TestFixtureLibraryServiceListLocalMissingDirectoryIsEmptyNotError(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "does-not-exist")
	svc := NewFixtureLibraryService("", root, missing)

	view, err := svc.ListLocal()
	if err != nil {
		t.Fatalf("expected a missing fixtures directory to be an empty library, not an error, got %v", err)
	}
	if view.Rows == nil {
		t.Fatalf("expected a non-nil empty Rows slice, got nil")
	}
	if len(view.Rows) != 0 {
		t.Fatalf("expected zero rows, got %d: %+v", len(view.Rows), view.Rows)
	}
}

// TestFixtureLibraryServiceListLocalNeverMarshalsRowsAsNull pins the
// svc_show.go DiagnosticReportView contract at this new view too:
// json.Marshal of the returned view must carry "rows":[], never
// "rows":null -- a mock frontend test can never catch this class of bug
// since it never round-trips through real Go JSON encoding, and there is
// no error boundary anywhere in this app (main.tsx has none).
func TestFixtureLibraryServiceListLocalNeverMarshalsRowsAsNull(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "does-not-exist")
	svc := NewFixtureLibraryService("", root, missing)

	view, err := svc.ListLocal()
	if err != nil {
		t.Fatalf("ListLocal: %v", err)
	}

	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("json.Marshal(view): %v", err)
	}
	if !strings.Contains(string(encoded), `"rows":[]`) {
		t.Fatalf("expected the JSON payload to carry \"rows\":[], got %s", encoded)
	}
}

// TestFixtureLibraryServiceInspectReportsInvalidWithDiagnostics proves
// Inspect forwards to the real "fixture inspect" route: an empty
// (guaranteed-invalid) fixture document returns valid:false with a
// non-empty Errors slice, and a well-formed one returns valid:true with
// its pinned StableKey/ContentHash.
func TestFixtureLibraryServiceInspectReportsInvalidWithDiagnostics(t *testing.T) {
	svc, root, fixturesDir := newTestFixtureLibraryService(t)
	writeLibraryTestFixture(t, fixturesDir, "bad.yaml", "")
	writeLibraryTestFixture(t, fixturesDir, "good.yaml", validFixtureYAMLForLibraryTest)

	badRel, err := filepath.Rel(root, filepath.Join(fixturesDir, "bad.yaml"))
	if err != nil {
		t.Fatalf("filepath.Rel(bad): %v", err)
	}
	badView, err := svc.Inspect(badRel)
	if err != nil {
		t.Fatalf("Inspect(bad): %v", err)
	}
	if badView.Valid {
		t.Fatalf("expected the empty fixture document to be invalid, got %+v", badView)
	}
	if len(badView.Errors) == 0 {
		t.Fatalf("expected a non-empty Errors slice for an invalid fixture, got %+v", badView)
	}

	goodRel, err := filepath.Rel(root, filepath.Join(fixturesDir, "good.yaml"))
	if err != nil {
		t.Fatalf("filepath.Rel(good): %v", err)
	}
	goodView, err := svc.Inspect(goodRel)
	if err != nil {
		t.Fatalf("Inspect(good): %v", err)
	}
	if !goodView.Valid {
		t.Fatalf("expected a well-formed fixture to be valid, got %+v", goodView)
	}
	if goodView.StableKey == "" || goodView.ContentHash == "" {
		t.Fatalf("expected a valid Inspect to carry a pinned StableKey/ContentHash, got %+v", goodView)
	}
}

// TestFixtureLibraryServiceSearchOFLReportsUnreachableWithoutThrowing
// proves SearchOFL projects an index-fetch failure into a renderable view
// -- unreachable:true, a non-nil empty manufacturers slice, and a nil
// error -- an unreachable catalog is a state to render, never a thrown
// exception (09-05-PLAN.md Task 1 RED / Task 3 GREEN). The service's
// unexported oflIndexRef is pointed at an unroutable loopback address
// (never the real network, mirroring fetch_test.go's httptest-only
// convention) so the failure is deterministic rather than depending on
// this test environment's actual network reachability.
func TestFixtureLibraryServiceSearchOFLReportsUnreachableWithoutThrowing(t *testing.T) {
	svc, _, _ := newTestFixtureLibraryService(t)
	svc.oflIndexRef = ofl.ManufacturerIndexRef{Mirror: "http://127.0.0.1:1", AllowMirror: true}

	view, err := svc.SearchOFL("acme")
	if err != nil {
		t.Fatalf("expected SearchOFL to never return an error, got %v", err)
	}
	if !view.Unreachable {
		t.Fatalf("expected view.Unreachable to be true for a catalog this test cannot reach, got %+v", view)
	}
	if view.Manufacturers == nil {
		t.Fatalf("expected a non-nil empty Manufacturers slice, got nil")
	}
	if len(view.Manufacturers) != 0 {
		t.Fatalf("expected zero manufacturers for an unreachable catalog, got %d: %+v", len(view.Manufacturers), view.Manufacturers)
	}
	if view.Query != "acme" {
		t.Fatalf("expected view.Query to echo the caller's query, got %q", view.Query)
	}
}
