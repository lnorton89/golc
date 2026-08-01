// fixtureindex_test.go proves ofl.FetchFixtureIndex/ofl.FilterFixtureIndex's
// contract (09-RESEARCH.md Open Question 1's flagged follow-up,
// implemented): FetchFixtureIndex parses a GitHub recursive-tree response
// into one FixtureIndexEntry per fixtures/<manufacturer>/<key>.json blob,
// skipping directory entries, non-fixture files, and the manufacturer
// index itself. FilterFixtureIndex proves the pure case-insensitive
// FixtureKey-only substring match -- reusing fetch_test.go's exact
// httptest + AllowMirror convention, so no test in this file reaches the
// real api.github.com host.
package ofl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture/ofl"
)

const fixtureTreeBody = `{
  "sha": "abc123",
  "url": "https://api.github.com/repos/OpenLightingProject/open-fixture-library/git/trees/abc123",
  "tree": [
    {"path": "fixtures", "mode": "040000", "type": "tree", "sha": "t1"},
    {"path": "fixtures/chauvet-dj", "mode": "040000", "type": "tree", "sha": "t2"},
    {"path": "fixtures/chauvet-dj/colorband-pix.json", "mode": "100644", "type": "blob", "sha": "b1", "size": 100},
    {"path": "fixtures/chauvet-dj/led-par-64-tri-b.json", "mode": "100644", "type": "blob", "sha": "b2", "size": 100},
    {"path": "fixtures/manufacturers.json", "mode": "100644", "type": "blob", "sha": "b3", "size": 100},
    {"path": "fixtures/acme", "mode": "040000", "type": "tree", "sha": "t3"},
    {"path": "fixtures/acme/spotlight-1000.json", "mode": "100644", "type": "blob", "sha": "b4", "size": 100},
    {"path": "README.md", "mode": "100644", "type": "blob", "sha": "b5", "size": 100}
  ],
  "truncated": false
}`

// TestFetchFixtureIndexParsesTree proves FetchFixtureIndex extracts exactly
// the fixtures/<manufacturer>/<key>.json blob paths, sorted ascending by
// ManufacturerKey then FixtureKey, skipping directory ("tree") entries,
// fixtures/manufacturers.json (no manufacturer subdirectory), and anything
// outside fixtures/.
func TestFetchFixtureIndexParsesTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureTreeBody))
	}))
	defer server.Close()

	entries, err := ofl.FetchFixtureIndex(context.Background(), ofl.FixtureIndexRef{Mirror: server.URL, AllowMirror: true})
	require.NoError(t, err, "FetchFixtureIndex")
	require.Len(t, entries, 3, "expected 3 fixture entries, got %+v", entries)
	require.Equal(t, "acme", entries[0].ManufacturerKey, "expected the first entry sorted to acme/spotlight-1000, got %+v", entries[0])
	require.Equal(t, "spotlight-1000", entries[0].FixtureKey, "expected the first entry sorted to acme/spotlight-1000, got %+v", entries[0])
	require.Equal(t, "chauvet-dj", entries[1].ManufacturerKey, "expected the second entry to be chauvet-dj/colorband-pix, got %+v", entries[1])
	require.Equal(t, "colorband-pix", entries[1].FixtureKey, "expected the second entry to be chauvet-dj/colorband-pix, got %+v", entries[1])
	require.Equal(t, "chauvet-dj", entries[2].ManufacturerKey, "expected the third entry to be chauvet-dj/led-par-64-tri-b, got %+v", entries[2])
	require.Equal(t, "led-par-64-tri-b", entries[2].FixtureKey, "expected the third entry to be chauvet-dj/led-par-64-tri-b, got %+v", entries[2])
}

// TestFetchFixtureIndexRejectsForeignHostWithoutOptIn proves a non-default
// host is rejected with GOLC_FIXTURE_OFL_MIRROR_HOST when the caller has
// not opted into the mirror -- the exact SSRF guard fetch.go's Fetch
// already enforces, reused here rather than re-implemented.
func TestFetchFixtureIndexRejectsForeignHostWithoutOptIn(t *testing.T) {
	ref := ofl.FixtureIndexRef{Mirror: "https://example.com", AllowMirror: false}
	_, err := ofl.FetchFixtureIndex(context.Background(), ref)
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_MIRROR_HOST", "expected FetchFixtureIndex to reject a non-default host without --allow-mirror")
}

// TestFetchFixtureIndexBoundsResponseSize proves the shared response-size
// cap rejects an oversized tree listing rather than reading it unboundedly.
func TestFetchFixtureIndexBoundsResponseSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oversized := make([]byte, 5*1024*1024) // exceeds the 4 MiB cap
		_, _ = w.Write(oversized)
	}))
	defer server.Close()

	_, err := ofl.FetchFixtureIndex(context.Background(), ofl.FixtureIndexRef{Mirror: server.URL, AllowMirror: true})
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_TOO_LARGE", "expected FetchFixtureIndex to reject an oversized response")
}

// TestFetchFixtureIndexFailureCarriesDiagnostic proves a fetch failure
// (e.g. an unreachable server) carries GOLC_FIXTURE_OFL_INDEX_FETCH_FAILED.
func TestFetchFixtureIndexFailureCarriesDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ofl.FetchFixtureIndex(context.Background(), ofl.FixtureIndexRef{Mirror: server.URL, AllowMirror: true})
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_INDEX_FETCH_FAILED", "expected FetchFixtureIndex to fail against a 500 response")
}

// TestFilterFixtureIndexMatchesFixtureKeyOnlyCaseInsensitively proves
// FilterFixtureIndex performs a case-insensitive substring match against
// FixtureKey only (never ManufacturerKey -- that is FilterManufacturers'
// job), returns a non-nil slice, and returns an empty slice for an empty
// query. This is the exact match a query like "colorband" needs to find
// chauvet-dj's COLORband PiX fixture even though "colorband" is a fixture
// key, not a manufacturer name.
func TestFilterFixtureIndexMatchesFixtureKeyOnlyCaseInsensitively(t *testing.T) {
	all := []ofl.FixtureIndexEntry{
		{ManufacturerKey: "chauvet-dj", FixtureKey: "colorband-pix"},
		{ManufacturerKey: "chauvet-dj", FixtureKey: "led-par-64-tri-b"},
		{ManufacturerKey: "acme", FixtureKey: "spotlight-1000"},
	}

	byKey := ofl.FilterFixtureIndex(all, "COLORBAND")
	require.Len(t, byKey, 1, "expected a case-insensitive fixture-key match for %q, got %+v", "COLORBAND", byKey)
	require.Equal(t, "colorband-pix", byKey[0].FixtureKey, "expected a case-insensitive fixture-key match for %q, got %+v", "COLORBAND", byKey)

	byManufacturer := ofl.FilterFixtureIndex(all, "chauvet")
	require.Empty(t, byManufacturer, "expected a manufacturer-only query to match zero fixture keys, got %+v", byManufacturer)

	empty := ofl.FilterFixtureIndex(all, "")
	require.NotNil(t, empty, "expected FilterFixtureIndex to return a non-nil slice for an empty query")
	require.Empty(t, empty, "expected an empty query to return zero results, got %+v", empty)

	noMatch := ofl.FilterFixtureIndex(all, "nonexistent")
	require.NotNil(t, noMatch, "expected FilterFixtureIndex to return a non-nil slice for a query with no matches")
	require.Empty(t, noMatch, "expected zero matches for a nonexistent query, got %+v", noMatch)
}
