// fixtureindex.go implements the fixture-name half of D-01/D-03's Open
// Fixture Library catalog search (09-RESEARCH.md Open Question 1's flagged
// v1.x follow-up): manufacturers.go's FetchManufacturers can only search
// manufacturer names, because raw.githubusercontent.com serves file
// content, never directory listings. FetchFixtureIndex instead calls the
// GitHub REST "get a tree" endpoint (the second, discretely reviewed
// SSRF-allowed host fetch.go's githubAPIHost adds) to enumerate every
// fixtures/<manufacturer>/<key>.json path in one request, so a query like
// "colorband" can match a fixture's own key, not just its manufacturer's
// name. FilterFixtureIndex is the pure, in-memory key substring filter
// over an already-fetched index -- deliberately matching FixtureKey only
// (never ManufacturerKey), so a manufacturer-name query does not also
// flood the results with every one of that manufacturer's fixtures;
// manufacturer-name matches are FilterManufacturers' job.
package ofl

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// fixtureTreeURL is the OFL repository's recursive git-tree listing --
// every blob path in the repository in one response, from which
// fixturePathPattern extracts each fixtures/<manufacturer>/<key>.json
// entry.
const fixtureTreeURL = "https://api.github.com/repos/OpenLightingProject/open-fixture-library/git/trees/master?recursive=1"

// fixturePathPattern matches exactly the fixture JSON files the tree
// listing carries -- "fixtures/<manufacturer>/<key>.json" -- excluding
// fixtures/manufacturers.json (no second path segment) and anything
// outside the fixtures/ directory.
var fixturePathPattern = regexp.MustCompile(`^fixtures/([a-z0-9-]+)/([a-z0-9-]+)\.json$`)

// FixtureIndexEntry is one fixture identified by the repository tree
// listing: its manufacturer directory key and its own fixture file key,
// mirroring OFLRef's Manufacturer/Key identity.
type FixtureIndexEntry struct {
	// ManufacturerKey is the OFL manufacturer directory key (for example
	// "chauvet-dj").
	ManufacturerKey string
	// FixtureKey is the OFL fixture file key (for example
	// "colorband-pix").
	FixtureKey string
}

// FixtureIndexRef mirrors ManufacturerIndexRef's own Mirror/AllowMirror
// opt-in shape so a test can point FetchFixtureIndex at an httptest server
// exactly the way manufacturers_test.go already does for
// FetchManufacturers. Mirror empty resolves to the default upstream host
// (githubAPIHost), identically to ManufacturerIndexRef. Production code
// never sets Mirror -- this seam exists for tests only.
type FixtureIndexRef struct {
	// Mirror, when non-empty, overrides the default upstream base URL.
	Mirror string
	// AllowMirror is the caller's explicit opt-in for Mirror resolving to
	// a host other than githubAPIHost (T-02-06: SSRF guard).
	AllowMirror bool
}

// gitTreeResponse is the subset of GitHub's "get a tree" response shape
// this package decodes: each tree entry's path and type ("blob" for a
// file, "tree" for a subdirectory -- only "blob" entries are fixture
// files).
type gitTreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
	Truncated bool `json:"truncated"`
}

// FetchFixtureIndex retrieves and parses the OFL repository's recursive
// git-tree listing into one FixtureIndexEntry per fixtures/<manufacturer>/
// <key>.json blob. It calls the exact same SSRF-guarded, bounded GET
// fetch.go's Fetch/FetchManufacturers use (getBounded), validated against
// githubAPIHost rather than defaultOFLHost -- one SSRF guard, one timeout,
// one size cap in this package, never a second implementation. Failures
// carry GOLC_FIXTURE_OFL_INDEX_FETCH_FAILED. The returned slice is sorted
// ascending by ManufacturerKey then FixtureKey and is never nil.
func FetchFixtureIndex(ctx context.Context, ref FixtureIndexRef) ([]FixtureIndexEntry, error) {
	target := fixtureTreeURL
	if ref.Mirror != "" {
		target = strings.TrimRight(ref.Mirror, "/") + "/repos/OpenLightingProject/open-fixture-library/git/trees/master?recursive=1"
	}

	body, err := getBounded(ctx, target, githubAPIHost, ref.AllowMirror)
	if err != nil {
		if strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_SCHEME") || strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_MIRROR_HOST") || strings.Contains(err.Error(), "GOLC_FIXTURE_OFL_TOO_LARGE") {
			return nil, err
		}
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_INDEX_FETCH_FAILED: %v", err)
	}

	var parsed gitTreeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_INDEX_FETCH_FAILED: %v", err)
	}

	entries := make([]FixtureIndexEntry, 0, len(parsed.Tree))
	for _, node := range parsed.Tree {
		if node.Type != "blob" {
			continue
		}
		match := fixturePathPattern.FindStringSubmatch(node.Path)
		if match == nil {
			continue
		}
		entries = append(entries, FixtureIndexEntry{ManufacturerKey: match[1], FixtureKey: match[2]})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ManufacturerKey != entries[j].ManufacturerKey {
			return entries[i].ManufacturerKey < entries[j].ManufacturerKey
		}
		return entries[i].FixtureKey < entries[j].FixtureKey
	})
	return entries, nil
}

// FilterFixtureIndex performs a case-insensitive substring match against
// FixtureKey only (never ManufacturerKey -- see this file's header),
// returning a non-nil slice (empty for an empty or whitespace-only query,
// or when nothing matches).
func FilterFixtureIndex(all []FixtureIndexEntry, query string) []FixtureIndexEntry {
	needle := strings.ToLower(strings.TrimSpace(query))
	results := make([]FixtureIndexEntry, 0)
	if needle == "" {
		return results
	}
	for _, entry := range all {
		if strings.Contains(strings.ToLower(entry.FixtureKey), needle) {
			results = append(results, entry)
		}
	}
	return results
}
