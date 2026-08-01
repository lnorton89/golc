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
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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
	err := os.Mkdir(fixturesDir, 0o755)
	require.NoError(t, err, "Mkdir(fixturesDir)")
	return NewFixtureLibraryService("", root, fixturesDir), root, fixturesDir
}

func writeLibraryTestFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	require.NoError(t, err, "WriteFile(%s)", name)
}

// TestFixtureLibraryServiceListLocalProjectsRowsSortedByStableKey proves
// ListLocal projects two valid fixtures into rows sorted ascending by
// StableKey, each carrying source "local" and status "valid".
func TestFixtureLibraryServiceListLocalProjectsRowsSortedByStableKey(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	writeLibraryTestFixture(t, fixturesDir, "slimpar.yaml", validFixtureYAMLForLibraryTest)
	writeLibraryTestFixture(t, fixturesDir, "megapar.yaml", secondValidFixtureYAMLForLibraryTest)

	view, err := svc.ListLocal()
	require.NoError(t, err, "ListLocal")
	require.Len(t, view.Rows, 2, "expected 2 rows: %+v", view.Rows)
	require.Less(t, view.Rows[0].StableKey, view.Rows[1].StableKey, "expected rows sorted ascending by StableKey")
	for _, row := range view.Rows {
		require.Equal(t, "local", row.Source, "row source")
		require.Equal(t, "valid", row.Status, "row status")
	}
}

// TestFixtureLibraryServiceListLocalProjectsModeChannelCounts proves each
// valid row's ModeChannelCounts maps every Modes entry's Name to its real
// channel width (len(Mode.Channels)) -- the "Standard" mode in
// validFixtureYAMLForLibraryTest declares two channels (intensity, color),
// so ModeChannelCounts["Standard"] must be 2, not the 1-channel fallback
// internal/pool/impact.go's BuildImpactPlan otherwise defaults to when a
// caller never resolves a real channel width.
func TestFixtureLibraryServiceListLocalProjectsModeChannelCounts(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	writeLibraryTestFixture(t, fixturesDir, "slimpar.yaml", validFixtureYAMLForLibraryTest)

	view, err := svc.ListLocal()
	require.NoError(t, err, "ListLocal")
	require.Len(t, view.Rows, 1, "expected 1 row: %+v", view.Rows)
	row := view.Rows[0]
	require.True(t, len(row.Modes) == 1 && row.Modes[0] == "Standard", "expected exactly one mode named Standard, got %+v", row.Modes)
	got := row.ModeChannelCounts["Standard"]
	require.Equal(t, 2, got, "expected ModeChannelCounts[%q] (full map %+v)", "Standard", row.ModeChannelCounts)
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
	require.NoError(t, err, "expected a missing fixtures directory to be an empty library, not an error")
	require.NotNil(t, view.Rows, "expected a non-nil empty Rows slice")
	require.Len(t, view.Rows, 0, "expected zero rows: %+v", view.Rows)
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
	require.NoError(t, err, "ListLocal")

	encoded, err := json.Marshal(view)
	require.NoError(t, err, "json.Marshal(view)")
	require.Contains(t, string(encoded), `"rows":[]`)
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
	require.NoError(t, err, "filepath.Rel(bad)")
	badView, err := svc.Inspect(badRel)
	require.NoError(t, err, "Inspect(bad)")
	require.False(t, badView.Valid, "expected the empty fixture document to be invalid, got %+v", badView)
	require.NotEmpty(t, badView.Errors, "expected a non-empty Errors slice for an invalid fixture, got %+v", badView)

	goodRel, err := filepath.Rel(root, filepath.Join(fixturesDir, "good.yaml"))
	require.NoError(t, err, "filepath.Rel(good)")
	goodView, err := svc.Inspect(goodRel)
	require.NoError(t, err, "Inspect(good)")
	require.True(t, goodView.Valid, "expected a well-formed fixture to be valid, got %+v", goodView)
	require.True(t, goodView.StableKey != "" && goodView.ContentHash != "", "expected a valid Inspect to carry a pinned StableKey/ContentHash, got %+v", goodView)
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
	require.NoError(t, err, "expected SearchOFL to never return an error")
	require.True(t, view.Unreachable, "expected view.Unreachable to be true for a catalog this test cannot reach, got %+v", view)
	require.NotNil(t, view.Manufacturers, "expected a non-nil empty Manufacturers slice")
	require.Len(t, view.Manufacturers, 0, "expected zero manufacturers for an unreachable catalog: %+v", view.Manufacturers)
	require.Equal(t, "acme", view.Query, "expected view.Query to echo the caller's query")
}

const searchOFLManufacturerIndexBody = `{
  "chauvet-dj": {"name": "Chauvet DJ", "website": "https://chauvetdj.example"},
  "acme": {"name": "Acme Lighting", "website": "https://acme.example"}
}`

const searchOFLFixtureTreeBody = `{
  "tree": [
    {"path": "fixtures/chauvet-dj/colorband-pix.json", "type": "blob"},
    {"path": "fixtures/chauvet-dj/led-par-64-tri-b.json", "type": "blob"},
    {"path": "fixtures/acme/spotlight-1000.json", "type": "blob"},
    {"path": "fixtures/manufacturers.json", "type": "blob"}
  ],
  "truncated": false
}`

// newTestFixtureLibraryServiceWithOFLCatalog points a fresh service's
// oflIndexRef/oflFixtureIndexRef at two local httptest servers serving a
// small, deterministic manufacturer index and fixture-tree listing --
// never the live network (mirrors fetch_test.go's httptest-only
// convention).
func newTestFixtureLibraryServiceWithOFLCatalog(t *testing.T) *FixtureLibraryService {
	t.Helper()
	svc, _, _ := newTestFixtureLibraryService(t)

	manufacturerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(searchOFLManufacturerIndexBody))
	}))
	t.Cleanup(manufacturerServer.Close)
	svc.oflIndexRef = ofl.ManufacturerIndexRef{Mirror: manufacturerServer.URL, AllowMirror: true}

	fixtureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(searchOFLFixtureTreeBody))
	}))
	t.Cleanup(fixtureServer.Close)
	svc.oflFixtureIndexRef = ofl.FixtureIndexRef{Mirror: fixtureServer.URL, AllowMirror: true}

	return svc
}

// TestFixtureLibraryServiceSearchOFLMatchesFixtureKeys proves SearchOFL's
// fixture-name half: a query matching a fixture's own key ("colorband"),
// not any manufacturer's name or key, still returns that fixture --
// exactly the gap manufacturer-only search left (the "colorbar"/"colorband"
// bug report this feature fixes). The returned OFLFixtureView carries both
// keys, ready for PreviewOFL, and its ManufacturerName is resolved from the
// manufacturer index rather than left as a raw key.
func TestFixtureLibraryServiceSearchOFLMatchesFixtureKeys(t *testing.T) {
	svc := newTestFixtureLibraryServiceWithOFLCatalog(t)

	view, err := svc.SearchOFL("colorband")
	require.NoError(t, err, "expected SearchOFL to never return an error")
	require.False(t, view.Unreachable, "expected a reachable catalog, got %+v", view)
	require.Len(t, view.Manufacturers, 0, "expected zero manufacturer matches for a fixture-key-only query: %+v", view.Manufacturers)
	require.Len(t, view.Fixtures, 1, "expected exactly one fixture match for %q: %+v", "colorband", view.Fixtures)
	got := view.Fixtures[0]
	require.True(t, got.ManufacturerKey == "chauvet-dj" && got.FixtureKey == "colorband-pix", "expected chauvet-dj/colorband-pix, got %+v", got)
	require.Equal(t, "Chauvet DJ", got.ManufacturerName, "expected the fixture match's manufacturer name resolved from the manufacturer index")
}

// TestFixtureLibraryServiceSearchOFLManufacturerQueryOmitsFixtures proves a
// manufacturer-name query ("chauvet") does not also flood Fixtures with
// every one of that manufacturer's fixtures -- FilterFixtureIndex matches
// FixtureKey only, so manufacturer-name and fixture-key search stay two
// distinct, uncluttered result lists.
func TestFixtureLibraryServiceSearchOFLManufacturerQueryOmitsFixtures(t *testing.T) {
	svc := newTestFixtureLibraryServiceWithOFLCatalog(t)

	view, err := svc.SearchOFL("chauvet")
	require.NoError(t, err, "expected SearchOFL to never return an error")
	require.True(t, len(view.Manufacturers) == 1 && view.Manufacturers[0].Key == "chauvet-dj", "expected exactly one manufacturer match (chauvet-dj), got %+v", view.Manufacturers)
	require.Len(t, view.Fixtures, 0, "expected zero fixture matches for a manufacturer-name query: %+v", view.Fixtures)
}

// TestFixtureLibraryServiceSearchOFLFixtureIndexFailureStaysReachable
// proves a fixture-index-only fetch failure never marks the whole search
// unreachable -- manufacturer-name search still works from the
// (independently, successfully fetched) manufacturer index alone, with
// Fixtures simply empty.
func TestFixtureLibraryServiceSearchOFLFixtureIndexFailureStaysReachable(t *testing.T) {
	svc := newTestFixtureLibraryServiceWithOFLCatalog(t)
	svc.oflFixtureIndexRef = ofl.FixtureIndexRef{Mirror: "http://127.0.0.1:1", AllowMirror: true}

	view, err := svc.SearchOFL("chauvet")
	require.NoError(t, err, "expected SearchOFL to never return an error")
	require.False(t, view.Unreachable, "expected a fixture-index-only failure to leave the catalog reachable, got %+v", view)
	require.True(t, len(view.Manufacturers) == 1 && view.Manufacturers[0].Key == "chauvet-dj", "expected manufacturer search to still work: %+v", view.Manufacturers)
	require.NotNil(t, view.Fixtures, "expected a non-nil empty Fixtures slice")
	require.Len(t, view.Fixtures, 0, "expected zero fixtures for an unreachable fixture index: %+v", view.Fixtures)
}

// --- 09-06-PLAN.md Task 1 RED / Task 2 GREEN: preview-then-commit import ---

// oflFixtureCorpusPath resolves the shared chauvet-dj_led-par-64-tri-b.json
// OFL corpus fixture (internal/fixture/ofl/normalize_test.go's own
// TestNormalizeCanonicalPipeline fixture) relative to internal/wails, so
// PreviewOFL's tests exercise a real, warning-bearing OFL normalize result
// without maintaining a second copy of the corpus.
func oflFixtureCorpusPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "fixtures", "ofl", "chauvet-dj_led-par-64-tri-b.json")
	_, err := os.Stat(path)
	require.NoError(t, err, "OFL corpus fixture missing at %s", path)
	return path
}

// newMirrorServingCorpusFixture starts an httptest server serving the
// chauvet-dj/led-par-64-tri-b corpus fixture at the exact
// "/fixtures/<manufacturer>/<key>.json" path resolveTargetURL's mirror
// convention expects (internal/fixture/ofl/fetch.go) -- never the live
// network (mirrors fetch_test.go's httptest-only convention).
func newMirrorServingCorpusFixture(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(oflFixtureCorpusPath(t))
	require.NoError(t, err, "reading OFL corpus fixture")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fixtures/chauvet-dj/led-par-64-tri-b.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

// newTestFixtureLibraryServiceWithMirror mirrors newTestFixtureLibraryService,
// additionally pointing the service's preview mirror seam at server so
// PreviewOFL's "fixture import --ofl ... --mirror ... --allow-mirror" call
// never reaches the live catalog.
func newTestFixtureLibraryServiceWithMirror(t *testing.T, server *httptest.Server) (*FixtureLibraryService, string, string) {
	t.Helper()
	svc, root, fixturesDir := newTestFixtureLibraryService(t)
	svc.previewMirror = server.URL
	svc.previewAllowMirror = true
	return svc, root, fixturesDir
}

// TestPreviewOFLWritesNothingIntoTheLibrary proves a successful preview
// writes nothing into the fixtures directory and its preview token points
// outside the library directory (T-09-06-05).
func TestPreviewOFLWritesNothingIntoTheLibrary(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, fixturesDir := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)
	require.NotEmpty(t, view.PreviewToken, "expected a non-empty preview token")

	entries, err := os.ReadDir(fixturesDir)
	require.NoError(t, err, "ReadDir(fixturesDir)")
	require.Len(t, entries, 0, "expected the fixtures directory to remain empty after a preview")

	tokenDir := filepath.Dir(view.PreviewToken)
	absFixturesDir, err := filepath.Abs(fixturesDir)
	require.NoError(t, err, "filepath.Abs(fixturesDir)")
	require.NotEqual(t, absFixturesDir, tokenDir, "expected the preview token to live outside the library directory, got %q", view.PreviewToken)
}

// TestPreviewOFLReturnsInspectViewWithWarnings proves a candidate whose
// normalization produces lossy warnings returns valid:true with a
// non-empty warnings slice, each carrying severity and detail.
func TestPreviewOFLReturnsInspectViewWithWarnings(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, _ := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)
	require.NotEmpty(t, view.Inspect.Warnings, "expected the chauvet-dj/led-par-64-tri-b corpus fixture to carry lossy-import warnings, got none")
	for _, warning := range view.Inspect.Warnings {
		require.NotEmpty(t, warning.Severity, "expected every warning to carry a severity, got %+v", warning)
		require.NotEmpty(t, warning.Detail, "expected every warning to carry a detail, got %+v", warning)
	}
}

// TestPreviewOFLReturnsErrorsForInvalidCandidate proves a fetch failure (a
// fixture key the mirror does not serve) returns valid:false with a
// non-empty errors slice and a nil error -- never a thrown exception.
func TestPreviewOFLReturnsErrorsForInvalidCandidate(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, _ := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "does-not-exist")
	require.NoError(t, err, "expected PreviewOFL to never return an error")
	require.False(t, view.Inspect.Valid, "expected an invalid candidate for a fixture the mirror does not serve, got %+v", view.Inspect)
	require.NotEmpty(t, view.Inspect.Errors, "expected a non-empty Errors slice for an invalid candidate, got %+v", view.Inspect)
}

// TestCommitPreviewWritesTheExactPreviewedBytes proves the committed
// library file is byte-identical to the previewed artifact -- CommitPreview
// moves the previewed bytes rather than re-encoding them.
func TestCommitPreviewWritesTheExactPreviewedBytes(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, fixturesDir := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")
	previewBytes, err := os.ReadFile(view.PreviewToken)
	require.NoError(t, err, "reading previewed artifact")

	result := svc.CommitPreview(view.PreviewToken, false)
	require.Equal(t, 0, result.ExitCode, "CommitPreview: stderr %q", result.Stderr)

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	committedBytes, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading committed library file")
	require.True(t, bytes.Equal(previewBytes, committedBytes), "expected the committed file to be byte-identical to the previewed artifact")

	_, err = os.Stat(view.PreviewToken)
	require.True(t, errors.Is(err, fs.ErrNotExist), "expected the previewed file to be moved (no longer present at the token path), got err=%v", err)
}

// TestCommitPreviewRefusesExistingDestination proves a destination that
// already exists is refused with GOLC_WAILS_FIXTURE_IMPORT_EXISTS, leaving
// the existing file unchanged (T-09-06-02).
func TestCommitPreviewRefusesExistingDestination(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, fixturesDir := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	sentinel := []byte("hand-edited sentinel content")
	err = os.WriteFile(destPath, sentinel, 0o644)
	require.NoError(t, err, "seeding existing destination")

	result := svc.CommitPreview(view.PreviewToken, false)
	require.NotEqual(t, 0, result.ExitCode, "expected CommitPreview to refuse an existing destination")
	require.Contains(t, result.Stderr, "GOLC_WAILS_FIXTURE_IMPORT_EXISTS")

	after, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading destination after refused commit")
	require.True(t, bytes.Equal(after, sentinel), "expected the existing destination to be unchanged, got %q", after)
}

// TestCommitPreviewReplacesOnlyWithExplicitOverwrite proves the same call
// with overwrite:true replaces the existing destination and succeeds.
func TestCommitPreviewReplacesOnlyWithExplicitOverwrite(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, fixturesDir := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	sentinel := []byte("hand-edited sentinel content")
	err = os.WriteFile(destPath, sentinel, 0o644)
	require.NoError(t, err, "seeding existing destination")

	result := svc.CommitPreview(view.PreviewToken, true)
	require.Equal(t, 0, result.ExitCode, "expected an explicit overwrite to succeed, got stderr %q", result.Stderr)

	after, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading destination after overwrite")
	require.False(t, bytes.Equal(after, sentinel), "expected the destination to be replaced, still carries the sentinel content")
}

// TestCommitPreviewRejectsATokenOutsideThePreviewDirectory proves a token
// pointing anywhere other than this service's own preview directory is
// refused with GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN and writes nothing
// (T-09-06-01).
func TestCommitPreviewRejectsATokenOutsideThePreviewDirectory(t *testing.T) {
	svc, root, fixturesDir := newTestFixtureLibraryService(t)
	outsidePath := filepath.Join(root, "outside-preview-dir.json")
	err := os.WriteFile(outsidePath, []byte(`{"definition":{},"provenance":{}}`), 0o644)
	require.NoError(t, err, "seeding outside-preview file")

	result := svc.CommitPreview(outsidePath, false)
	require.NotEqual(t, 0, result.ExitCode, "expected CommitPreview to reject a token outside the preview directory")
	require.Contains(t, result.Stderr, "GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN")

	entries, err := os.ReadDir(fixturesDir)
	require.NoError(t, err, "ReadDir(fixturesDir)")
	require.Len(t, entries, 0, "expected nothing written into the library")
}

// TestDiscardPreviewRemovesTheCandidate proves discarding a staged preview
// removes the preview file at its token path (T-09-06-05).
func TestDiscardPreviewRemovesTheCandidate(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, _ := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")
	_, err = os.Stat(view.PreviewToken)
	require.NoError(t, err, "expected the preview file to exist before discarding")

	result := svc.DiscardPreview(view.PreviewToken)
	require.Equal(t, 0, result.ExitCode, "DiscardPreview: stderr %q", result.Stderr)

	_, err = os.Stat(view.PreviewToken)
	require.True(t, errors.Is(err, fs.ErrNotExist), "expected the preview path to no longer exist after discarding, got err=%v", err)
}

// --- 09-07-PLAN.md Task 1 RED / Task 2 GREEN: hand-authored YAML add ---

// TestPreviewRegistryKeepsCatalogBehaviourUnchanged proves the in-memory
// preview registry introduced for PreviewFile (09-07-PLAN.md Task 2) does
// not change the OFL catalog import path's own observable behaviour:
// PreviewOFL then CommitPreview still writes byte-identical bytes to the
// destination this service itself derived, and the committed fixture then
// appears in ListLocal -- proving the registry refactor is non-breaking
// rather than merely assumed.
func TestPreviewRegistryKeepsCatalogBehaviourUnchanged(t *testing.T) {
	server := newMirrorServingCorpusFixture(t)
	svc, _, fixturesDir := newTestFixtureLibraryServiceWithMirror(t, server)

	view, err := svc.PreviewOFL("chauvet-dj", "led-par-64-tri-b")
	require.NoError(t, err, "PreviewOFL")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)
	previewBytes, err := os.ReadFile(view.PreviewToken)
	require.NoError(t, err, "reading previewed artifact")

	result := svc.CommitPreview(view.PreviewToken, false)
	require.Equal(t, 0, result.ExitCode, "CommitPreview: stderr %q", result.Stderr)

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	committedBytes, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading committed library file")
	require.True(t, bytes.Equal(previewBytes, committedBytes), "expected the committed file to be byte-identical to the previewed artifact")

	localView, err := svc.ListLocal()
	require.NoError(t, err, "ListLocal")
	found := false
	for _, row := range localView.Rows {
		if row.FileName == view.SuggestedFileName {
			found = true
			break
		}
	}
	require.True(t, found, "expected the committed OFL fixture to appear in ListLocal, got %+v", localView.Rows)
}

// TestPreviewFileStagesAValidDefinitionWithoutTouchingTheLibrary proves
// previewing a valid hand-authored YAML file returns valid:true with the
// pinned stable key, and the library directory gains no file (mirrors
// TestPreviewOFLWritesNothingIntoTheLibrary's identical write-nothing
// discipline for the catalog path).
func TestPreviewFileStagesAValidDefinitionWithoutTouchingTheLibrary(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "hand-authored.yaml")
	err := os.WriteFile(sourcePath, []byte(validFixtureYAMLForLibraryTest), 0o644)
	require.NoError(t, err, "seeding hand-authored fixture file")

	view, err := svc.PreviewFile(sourcePath)
	require.NoError(t, err, "PreviewFile")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)
	require.Equal(t, "Chauvet/SlimPAR Pro", view.Inspect.StableKey, "expected the pinned stable key")
	require.NotEmpty(t, view.PreviewToken, "expected a non-empty preview token")

	entries, err := os.ReadDir(fixturesDir)
	require.NoError(t, err, "ReadDir(fixturesDir)")
	require.Len(t, entries, 0, "expected the fixtures directory to remain empty after a preview")
}

// TestPreviewFileReportsAnUnreadablePath proves a path that cannot be read
// (the file does not exist) returns valid:false with a non-empty errors
// slice and a nil error, and stages nothing.
func TestPreviewFileReportsAnUnreadablePath(t *testing.T) {
	svc, root, _ := newTestFixtureLibraryService(t)
	missing := filepath.Join(root, "does-not-exist.yaml")

	view, err := svc.PreviewFile(missing)
	require.NoError(t, err, "expected PreviewFile to never return an error")
	require.False(t, view.Inspect.Valid, "expected an invalid preview for an unreadable path, got %+v", view.Inspect)
	require.NotEmpty(t, view.Inspect.Errors, "expected a non-empty Errors slice for an unreadable path, got %+v", view.Inspect)
	require.Empty(t, view.PreviewToken, "expected no preview token to be staged for an unreadable path")
}

// TestPreviewFileReportsAnInvalidDefinition proves a malformed YAML file
// returns valid:false carrying the canonical GOLC_FIXTURE_* diagnostic
// from the route, not a locally invented message.
func TestPreviewFileReportsAnInvalidDefinition(t *testing.T) {
	svc, _, _ := newTestFixtureLibraryService(t)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "bad.yaml")
	err := os.WriteFile(sourcePath, []byte(""), 0o644)
	require.NoError(t, err, "seeding malformed fixture file")

	view, err := svc.PreviewFile(sourcePath)
	require.NoError(t, err, "expected PreviewFile to never return an error")
	require.False(t, view.Inspect.Valid, "expected an invalid preview for a malformed definition, got %+v", view.Inspect)
	require.NotEmpty(t, view.Inspect.Errors, "expected a non-empty Errors slice: %+v", view.Inspect)
	for _, message := range view.Inspect.Errors {
		require.Contains(t, message, "GOLC_FIXTURE", "expected the canonical route's own GOLC_FIXTURE_* diagnostic")
	}
}

// TestCommitPreviewWritesTheCustomFixtureVerbatim proves committing a
// staged custom-YAML preview places a byte-identical copy of the
// operator's file into the library, and it then appears in ListLocal.
func TestCommitPreviewWritesTheCustomFixtureVerbatim(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "hand-authored.yaml")
	err := os.WriteFile(sourcePath, []byte(validFixtureYAMLForLibraryTest), 0o644)
	require.NoError(t, err, "seeding hand-authored fixture file")

	view, err := svc.PreviewFile(sourcePath)
	require.NoError(t, err, "PreviewFile")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)

	result := svc.CommitPreview(view.PreviewToken, false)
	require.Equal(t, 0, result.ExitCode, "CommitPreview: stderr %q", result.Stderr)

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	committedBytes, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading committed library file")
	require.Equal(t, validFixtureYAMLForLibraryTest, string(committedBytes), "expected the committed file to be byte-identical to the operator's file")

	localView, err := svc.ListLocal()
	require.NoError(t, err, "ListLocal")
	found := false
	for _, row := range localView.Rows {
		if row.FileName == view.SuggestedFileName {
			found = true
			require.Equal(t, "valid", row.Status, "expected the newly added custom fixture to be valid, got %+v", row)
		}
	}
	require.True(t, found, "expected the committed custom fixture to appear in ListLocal, got %+v", localView.Rows)
}

// TestCommitPreviewRefusesExistingDestinationForCustomFixtures proves the
// same refuse-and-require-overwrite behaviour the catalog path has (see
// TestCommitPreviewRefusesExistingDestination) also applies to a
// hand-authored custom fixture.
func TestCommitPreviewRefusesExistingDestinationForCustomFixtures(t *testing.T) {
	svc, _, fixturesDir := newTestFixtureLibraryService(t)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "hand-authored.yaml")
	err := os.WriteFile(sourcePath, []byte(validFixtureYAMLForLibraryTest), 0o644)
	require.NoError(t, err, "seeding hand-authored fixture file")

	view, err := svc.PreviewFile(sourcePath)
	require.NoError(t, err, "PreviewFile")
	require.True(t, view.Inspect.Valid, "expected a valid preview, got %+v", view.Inspect)

	destPath := filepath.Join(fixturesDir, view.SuggestedFileName)
	sentinel := []byte("hand-edited sentinel content")
	err = os.WriteFile(destPath, sentinel, 0o644)
	require.NoError(t, err, "seeding existing destination")

	result := svc.CommitPreview(view.PreviewToken, false)
	require.NotEqual(t, 0, result.ExitCode, "expected CommitPreview to refuse an existing destination")
	require.Contains(t, result.Stderr, "GOLC_WAILS_FIXTURE_IMPORT_EXISTS")

	after, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading destination after refused commit")
	require.True(t, bytes.Equal(after, sentinel), "expected the existing destination to be unchanged, got %q", after)

	overwriteResult := svc.CommitPreview(view.PreviewToken, true)
	require.Equal(t, 0, overwriteResult.ExitCode, "expected an explicit overwrite to succeed, got stderr %q", overwriteResult.Stderr)
	replaced, err := os.ReadFile(destPath)
	require.NoError(t, err, "reading destination after overwrite")
	require.False(t, bytes.Equal(replaced, sentinel), "expected the destination to be replaced, still carries the sentinel content")
}
