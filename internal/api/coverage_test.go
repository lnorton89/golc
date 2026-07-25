// coverage_test.go is the API-01 capability-coverage gate: every route
// the daemon-side command registry declares must be either mapped to
// exactly one REST operation in this package (api.RegisteredRoutes()) or
// listed here, by name, with a documented reason, in excludedRoutes --
// never silently unmapped. This file lives in the external api_test
// package (not api) so it can reach a real, live registry through
// internal/routecatalog's test-only bridge without internal/api's own
// production code ever importing the package that bridge wraps (see
// internal/api/doc.go and internal/routecatalog's own doc comment for
// why: an import here, from this package's production files, would close
// the daemon<->api<->execution import cycle 07-02-PLAN.md's Executor
// interface exists to avoid).
package api_test

import (
	"sort"
	"testing"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
)

// excludedRoutes is the committed, documented exclusion set: every
// registered route not yet mapped to a REST operation, grouped by the
// reason it is excluded right now. A route only ever needs to appear
// here or in api.RegisteredRoutes() -- never both, and never neither.
var excludedRoutes = buildExcludedRoutes()

func buildExcludedRoutes() map[string]string {
	const reasonDevTooling = "repository development tooling (build/test/docs/packaging/Linear sync); not part of the external show-control API surface"
	const reasonDaemonLifecycle = "daemon lifecycle entrypoint -- IS the process that hosts /v1, not a route exposed on it"
	const reasonArtnetDeferred = "Art-Net daemon runtime route; deferred to a later 07-0x plan alongside the API's auth/rate-limit/mutation work"
	const reasonMutationDeferred = "show-domain mutation route; deferred to a later 07-0x plan (arrives with If-Match/dry-run/batch semantics) -- this plan proves the read-path translation + coverage mechanism only"
	const reasonReadDeferred = "show-domain read/inspect route not yet wired to a REST GET; this plan's minimum scope is GET /v1/config + GET /v1/show, following the same RegisterOperation seam in a later 07-0x plan"

	excluded := map[string]string{}
	addAll := func(reason string, routes ...string) {
		for _, route := range routes {
			excluded[route] = reason
		}
	}

	addAll(reasonDevTooling,
		"build", "check", "docs", "generate",
		"linear apply", "linear archive", "linear catalog", "linear drift",
		"linear map migrate", "linear preview", "linear status", "linear unlink",
		"linear validate", "package", "test", "tools update",
	)
	addAll(reasonDaemonLifecycle, "artnet serve")
	addAll(reasonArtnetDeferred,
		"artnet configure", "artnet discover", "artnet interface list",
		"artnet master set", "artnet safety blackout", "artnet safety revoke-automation",
		"artnet safety stop-all", "artnet status", "artnet target disable", "artnet target enable",
	)
	addAll(reasonMutationDeferred,
		"blend create", "chase create", "chase delete", "chase duplicate", "chase reorder", "chase update",
		"config set", "deployment activate", "deployment create", "fixture import",
		"motion create", "motion delete", "motion duplicate", "motion rename",
		"operatorsurface assign", "operatorsurface create", "operatorsurface remove", "operatorsurface unassign",
		"playback bpm set", "playback bpm tap", "playback evaluate", "playback switch",
		"pool apply", "pool substitute", "pool update",
		"preset delete", "preset record", "preset rename",
		"programmer clear", "programmer set",
		"scene activate", "scene create", "scene delete", "scene duplicate", "scene layer set", "scene rename",
		"show open", "show save", "show save-as",
		"theme create", "theme delete", "theme rename",
	)
	addAll(reasonReadDeferred,
		"config explain", "fixture inspect", "fixture validate",
		"operatorsurface list", "operatorsurface show",
		"programmer inspect", "show diagnose", "show export",
	)

	return excluded
}

// TestCapabilityCoverage proves API-01's mechanical claim: every route
// the real command registry declares is either mapped to exactly one REST
// operation, or explicitly, individually excluded with a reason -- never
// silently unmapped, and never claimed by both sets at once.
func TestCapabilityCoverage(t *testing.T) {
	catalog, err := routecatalog.New()
	if err != nil {
		t.Fatalf("routecatalog.New: %v", err)
	}
	allRoutes := catalog.Names()
	if len(allRoutes) == 0 {
		t.Fatal("expected the real command registry to declare at least one route")
	}

	covered := map[string]bool{}
	for _, route := range api.RegisteredRoutes() {
		if covered[route] {
			t.Fatalf("route %q is registered more than once in api.RegisteredRoutes()", route)
		}
		covered[route] = true
	}

	var uncovered []string
	var doubleMapped []string
	for _, route := range allRoutes {
		_, excluded := excludedRoutes[route]
		_, isCovered := covered[route]
		switch {
		case isCovered && excluded:
			doubleMapped = append(doubleMapped, route)
		case !isCovered && !excluded:
			uncovered = append(uncovered, route)
		}
	}

	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		t.Fatalf("routes present in neither api.RegisteredRoutes() nor the exclusion set: %v", uncovered)
	}
	if len(doubleMapped) > 0 {
		sort.Strings(doubleMapped)
		t.Fatalf("routes present in both api.RegisteredRoutes() and the exclusion set: %v", doubleMapped)
	}

	// Every excluded entry must name a route the real registry actually
	// declares -- an exclusion for a route that no longer exists (renamed
	// or removed) is exactly the kind of silent drift this gate exists to
	// catch.
	allRoutesSet := map[string]bool{}
	for _, route := range allRoutes {
		allRoutesSet[route] = true
	}
	var staleExclusions []string
	for route := range excludedRoutes {
		if !allRoutesSet[route] {
			staleExclusions = append(staleExclusions, route)
		}
	}
	if len(staleExclusions) > 0 {
		sort.Strings(staleExclusions)
		t.Fatalf("excludedRoutes names routes the live registry no longer declares: %v", staleExclusions)
	}
}
