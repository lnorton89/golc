// manufacturers.go implements D-01/D-03's Open Fixture Library catalog
// search (09-05-PLAN.md Task 3): FetchManufacturers retrieves and parses
// the OFL manufacturer index -- the only catalog-search surface reachable
// from the single SSRF-guarded host fetch.go's Fetch already allows (this
// version deliberately searches manufacturer names, not fixture model
// names; see 09-RESEARCH.md Open Question 1 and 09-05-PLAN.md's objective
// for why a fixture-name index would require a second, discretely
// reviewed allowed host). FilterManufacturers is the pure, in-memory
// name/key substring filter over an already-fetched index.
package ofl

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// manufacturerIndexURLPattern is the OFL manufacturer index's fixed path
// under the same repository defaultOFLURLPattern already targets --
// deliberately NOT a second host constant (09-RESEARCH.md Pitfall 3): the
// index lives at "fixtures/manufacturers.json" under the identical
// raw-content host/repository fetch.go's Fetch already fetches individual
// fixture JSON from.
const manufacturerIndexURLPattern = "https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/manufacturers.json"

// Manufacturer is one Open Fixture Library manufacturer index entry.
type Manufacturer struct {
	// Key is the OFL manufacturer directory key (for example "chauvet-dj"),
	// the same identity OFLRef.Manufacturer expects.
	Key string
	// Name is the manufacturer's display name (for example "Chauvet DJ").
	Name string
	// Website is the manufacturer's homepage URL, when the index declares
	// one.
	Website string
}

// ManufacturerIndexRef mirrors OFLRef's own Mirror/AllowMirror opt-in
// shape (T-02-06) so a test can point FetchManufacturers at an httptest
// server exactly the way fetch_test.go already does for Fetch. Mirror
// empty resolves to the default upstream host, identically to OFLRef.
type ManufacturerIndexRef struct {
	// Mirror, when non-empty, overrides the default upstream base URL.
	Mirror string
	// AllowMirror is the caller's explicit opt-in for Mirror resolving to
	// a host other than the default upstream host (T-02-06: SSRF guard).
	AllowMirror bool
}

// manufacturerIndexEntry is one raw OFL manufacturers.json value --
// "name"/"website"/"rdmId" per the index's own documented shape. rdmId is
// decoded only to confirm the field exists in the real document; this
// importer does not use it.
type manufacturerIndexEntry struct {
	Name    string `json:"name"`
	Website string `json:"website"`
	RDMID   *int   `json:"rdmId,omitempty"`
}

// FetchManufacturers retrieves and parses the OFL manufacturer index: a
// top-level JSON object mapping manufacturer key to
// {name, website, rdmId}, plus a "$schema" key the index also carries.
// It calls the exact same SSRF-guarded, bounded GET fetch.go's Fetch uses
// (getBounded) -- one SSRF guard, one timeout, one size cap in this
// package, never a second implementation. Every "$"-prefixed key is
// skipped (the index's own "$schema" entry, not a manufacturer). Failures
// carry GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED. The returned slice is
// sorted ascending by name and is never nil.
func FetchManufacturers(ctx context.Context, ref ManufacturerIndexRef) ([]Manufacturer, error) {
	target := manufacturerIndexURLPattern
	if ref.Mirror != "" {
		target = strings.TrimRight(ref.Mirror, "/") + "/fixtures/manufacturers.json"
	}

	body, err := getBounded(ctx, target, defaultOFLHost, ref.AllowMirror)
	if err != nil {
		if strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_SCHEME") || strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_HOST") || strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_TOO_LARGE") {
			return nil, err
		}
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED: %v", err)
	}

	manufacturers := make([]Manufacturer, 0, len(raw))
	for key, value := range raw {
		if strings.HasPrefix(key, "$") {
			continue
		}
		var entry manufacturerIndexEntry
		if err := json.Unmarshal(value, &entry); err != nil {
			return nil, fmt.Errorf("GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED: manufacturer %q: %v", key, err)
		}
		manufacturers = append(manufacturers, Manufacturer{Key: key, Name: entry.Name, Website: entry.Website})
	}

	sort.Slice(manufacturers, func(i, j int) bool { return manufacturers[i].Name < manufacturers[j].Name })
	return manufacturers, nil
}

// FilterManufacturers performs a case-insensitive substring match against
// both Name and Key, returning a non-nil slice (empty for an empty or
// whitespace-only query, or when nothing matches).
func FilterManufacturers(all []Manufacturer, query string) []Manufacturer {
	needle := strings.ToLower(strings.TrimSpace(query))
	results := make([]Manufacturer, 0)
	if needle == "" {
		return results
	}
	for _, manufacturer := range all {
		if strings.Contains(strings.ToLower(manufacturer.Name), needle) || strings.Contains(strings.ToLower(manufacturer.Key), needle) {
			results = append(results, manufacturer)
		}
	}
	return results
}
