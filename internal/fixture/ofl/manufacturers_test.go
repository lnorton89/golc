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
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("FetchManufacturers: %v", err)
	}
	if len(manufacturers) != 2 {
		t.Fatalf("expected 2 real manufacturer entries (excluding $schema), got %d: %+v", len(manufacturers), manufacturers)
	}
	if manufacturers[0].Name != "Acme Lighting" || manufacturers[1].Name != "Chauvet DJ" {
		t.Fatalf("expected manufacturers sorted ascending by name, got %q then %q", manufacturers[0].Name, manufacturers[1].Name)
	}
	if manufacturers[0].Key != "acme" {
		t.Fatalf("expected first manufacturer key %q, got %q", "acme", manufacturers[0].Key)
	}
}

// TestFetchManufacturersRejectsForeignHostWithoutOptIn proves a non-default
// host is rejected with the existing GOLC_FIXTURE_OFL_MIRROR_HOST
// diagnostic when the caller has not opted into the mirror -- the exact
// SSRF guard fetch.go's Fetch already enforces, reused here rather than
// re-implemented.
func TestFetchManufacturersRejectsForeignHostWithoutOptIn(t *testing.T) {
	ref := ofl.ManufacturerIndexRef{Mirror: "https://example.com", AllowMirror: false}
	_, err := ofl.FetchManufacturers(context.Background(), ref)
	if err == nil {
		t.Fatal("expected FetchManufacturers to reject a non-default host without --allow-mirror")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_HOST") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_MIRROR_HOST, got %v", err)
	}
}

// TestFetchManufacturersBoundsResponseSize proves the response-size cap
// rejects an oversized manufacturer index rather than reading it
// unboundedly -- the exact shared bound fetch.go's Fetch already enforces.
func TestFetchManufacturersBoundsResponseSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oversized := make([]byte, 3*1024*1024) // exceeds the 2 MiB cap
		_, _ = w.Write(oversized)
	}))
	defer server.Close()

	_, err := ofl.FetchManufacturers(context.Background(), ofl.ManufacturerIndexRef{Mirror: server.URL, AllowMirror: true})
	if err == nil {
		t.Fatal("expected FetchManufacturers to reject an oversized response")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_TOO_LARGE") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_TOO_LARGE, got %v", err)
	}
}

// TestFetchManufacturersFailureCarriesDiagnostic proves a fetch failure
// (e.g. an unreachable server) carries GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED.
func TestFetchManufacturersFailureCarriesDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ofl.FetchManufacturers(context.Background(), ofl.ManufacturerIndexRef{Mirror: server.URL, AllowMirror: true})
	if err == nil {
		t.Fatal("expected FetchManufacturers to fail against a 500 response")
	}
	if !strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED") {
		t.Fatalf("expected GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED, got %v", err)
	}
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
	if len(byName) != 1 || byName[0].Key != "chauvet-dj" {
		t.Fatalf("expected a case-insensitive name match for %q, got %+v", "chauvet", byName)
	}

	byKey := ofl.FilterManufacturers(all, "ADJ")
	if len(byKey) != 1 || byKey[0].Key != "adj" {
		t.Fatalf("expected a case-insensitive key match for %q, got %+v", "ADJ", byKey)
	}

	empty := ofl.FilterManufacturers(all, "")
	if empty == nil {
		t.Fatal("expected FilterManufacturers to return a non-nil slice for an empty query")
	}
	if len(empty) != 0 {
		t.Fatalf("expected an empty query to return zero results, got %d: %+v", len(empty), empty)
	}

	whitespaceOnly := ofl.FilterManufacturers(all, "   ")
	if whitespaceOnly == nil {
		t.Fatal("expected FilterManufacturers to return a non-nil slice for a whitespace-only query")
	}
	if len(whitespaceOnly) != 0 {
		t.Fatalf("expected a whitespace-only query to return zero results, got %d: %+v", len(whitespaceOnly), whitespaceOnly)
	}

	noMatch := ofl.FilterManufacturers(all, "nonexistent")
	if noMatch == nil {
		t.Fatal("expected FilterManufacturers to return a non-nil slice for a query with no matches")
	}
	if len(noMatch) != 0 {
		t.Fatalf("expected zero matches for a nonexistent query, got %d: %+v", len(noMatch), noMatch)
	}
}
