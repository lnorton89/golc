// manufacturers_test.go proves ofl.FetchManufacturers/ofl.FilterManufacturers's
// contract (09-05-PLAN.md Task 1 RED / Task 3 GREEN, D-01's catalog half):
// FetchManufacturers parses the OFL manufacturer-index shape (a top-level
// object of key -> {name, website, rdmId} plus a "$schema" key) into a
// sorted, non-nil Manufacturer slice, reusing fetch_test.go's exact
// httptest + AllowMirror convention -- no test in this file reaches the
// real raw.githubusercontent.com host. FilterManufacturers proves the pure
// case-insensitive name/key substring match.
package ofl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture/ofl"
)

const manufacturerIndexBody = `{
  "$schema": "https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/schemas/manufacturers.json",
  "acme": {"name": "Acme Lighting", "website": "https://acme.example", "rdmId": 1},
  "chauvet-dj": {"name": "Chauvet DJ", "website": "https://chauvetdj.example"}
}`

// TestFetchManufacturersParsesIndex proves a document shaped as a
// top-level object of key -> {name, website, rdmId} plus a "$schema" key
// yields one Manufacturer per real entry, sorted ascending by name, and
// skips every "$"-prefixed key.
func TestFetchManufacturersParsesIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(manufacturerIndexBody))
	}))
	defer server.Close()

	manufacturers, err := ofl.FetchManufacturers(context.Background(), ofl.ManufacturerIndexRef{Mirror: server.URL, AllowMirror: true})
	require.NoError(t, err, "FetchManufacturers")
	require.Len(t, manufacturers, 2, "expected 2 real manufacturer entries (excluding $schema), got %+v", manufacturers)
	require.Equal(t, "Acme Lighting", manufacturers[0].Name, "expected manufacturers sorted ascending by name, got %q then %q", manufacturers[0].Name, manufacturers[1].Name)
	require.Equal(t, "Chauvet DJ", manufacturers[1].Name, "expected manufacturers sorted ascending by name, got %q then %q", manufacturers[0].Name, manufacturers[1].Name)
	require.Equal(t, "acme", manufacturers[0].Key, "expected first manufacturer key %q, got %q", "acme", manufacturers[0].Key)
}

// TestFetchManufacturersRejectsForeignHostWithoutOptIn proves a non-default
// host is rejected with the existing GOLC_FIXTURE_OFL_MIRROR_HOST
// diagnostic when the caller has not opted into the mirror -- the exact
// SSRF guard fetch.go's Fetch already enforces, reused here rather than
// re-implemented.
func TestFetchManufacturersRejectsForeignHostWithoutOptIn(t *testing.T) {
	ref := ofl.ManufacturerIndexRef{Mirror: "https://example.com", AllowMirror: false}
	_, err := ofl.FetchManufacturers(context.Background(), ref)
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_MIRROR_HOST", "expected FetchManufacturers to reject a non-default host without --allow-mirror")
}

// TestFetchManufacturersBoundsResponseSize proves the response-size cap
// rejects an oversized manufacturer index rather than reading it
// unboundedly -- the exact shared bound fetch.go's Fetch already enforces.
func TestFetchManufacturersBoundsResponseSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oversized := make([]byte, 5*1024*1024) // exceeds the 4 MiB cap
		_, _ = w.Write(oversized)
	}))
	defer server.Close()

	_, err := ofl.FetchManufacturers(context.Background(), ofl.ManufacturerIndexRef{Mirror: server.URL, AllowMirror: true})
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_TOO_LARGE", "expected FetchManufacturers to reject an oversized response")
}

// TestFetchManufacturersFailureCarriesDiagnostic proves a fetch failure
// (e.g. an unreachable server) carries GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED.
func TestFetchManufacturersFailureCarriesDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ofl.FetchManufacturers(context.Background(), ofl.ManufacturerIndexRef{Mirror: server.URL, AllowMirror: true})
	require.ErrorContains(t, err, "GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED", "expected FetchManufacturers to fail against a 500 response")
}

// TestFilterManufacturersMatchesNameAndKeyCaseInsensitively proves
// FilterManufacturers performs a case-insensitive substring match against
// both Name and Key, returns a non-nil slice, and returns an empty slice
// for an empty query.
func TestFilterManufacturersMatchesNameAndKeyCaseInsensitively(t *testing.T) {
	all := []ofl.Manufacturer{
		{Key: "acme", Name: "Acme Lighting"},
		{Key: "chauvet-dj", Name: "Chauvet DJ"},
		{Key: "adj", Name: "American DJ"},
	}

	byName := ofl.FilterManufacturers(all, "chauvet")
	require.Len(t, byName, 1, "expected a case-insensitive name match for %q, got %+v", "chauvet", byName)
	require.Equal(t, "chauvet-dj", byName[0].Key, "expected a case-insensitive name match for %q, got %+v", "chauvet", byName)

	byKey := ofl.FilterManufacturers(all, "ADJ")
	require.Len(t, byKey, 1, "expected a case-insensitive key match for %q, got %+v", "ADJ", byKey)
	require.Equal(t, "adj", byKey[0].Key, "expected a case-insensitive key match for %q, got %+v", "ADJ", byKey)

	empty := ofl.FilterManufacturers(all, "")
	require.NotNil(t, empty, "expected FilterManufacturers to return a non-nil slice for an empty query")
	require.Empty(t, empty, "expected an empty query to return zero results, got %+v", empty)

	whitespaceOnly := ofl.FilterManufacturers(all, "   ")
	require.NotNil(t, whitespaceOnly, "expected FilterManufacturers to return a non-nil slice for a whitespace-only query")
	require.Empty(t, whitespaceOnly, "expected a whitespace-only query to return zero results, got %+v", whitespaceOnly)

	noMatch := ofl.FilterManufacturers(all, "nonexistent")
	require.NotNil(t, noMatch, "expected FilterManufacturers to return a non-nil slice for a query with no matches")
	require.Empty(t, noMatch, "expected zero matches for a nonexistent query, got %+v", noMatch)
}
