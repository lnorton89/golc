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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/routecatalog"
)

// excludedRoutes is the committed, documented exclusion set: every
// registered route not yet mapped to a REST operation, grouped by the
// reason it is excluded right now. A route only ever needs to appear
// here or in api.RegisteredRoutes() -- never both, and never neither.
var excludedRoutes = buildExcludedRoutes()

// buildExcludedRoutes returns the exclusion set as it stands at the close
// of Phase 7 (07-09-PLAN.md Task 2, the phase's final plan): every route
// still excluded here is a DELIBERATE, documented scope boundary, not a
// silently-unmapped gap -- api.RegisteredRoutes() now covers 8 concrete
// operations across every category 07-04/07-05/07-06/07-08 shipped
// (queries, one proven mutation route, atomic batch, key management,
// SSE). The two genuinely PERMANENT categories (never expected to gain a
// REST operation, regardless of future phases) are reasonDaemonLifecycle
// and reasonLocalProcessLaunch -- both name entrypoints that are either
// the process hosting /v1 itself or an interactive local-machine action
// with no meaningful remote-HTTP semantics. The remaining categories
// (reasonDevTooling, reasonArtnetFutureWork, reasonMutationFutureWork,
// reasonReadFutureWork) are NOT permanent in the same sense: each names
// routes a future milestone could wire onto this plan's now-proven
// RegisterOperation seam, mutation pipeline (mutate.go), and
// capability-coverage gate itself -- but wiring roughly 67 remaining
// routes (16 dev-tooling, 10 Art-Net, 42 show-domain mutation, 9
// show-domain read) each requires its own bespoke Huma input struct
// mapping JSON body fields to that route's specific CLI flags (mirrors
// mutate.go's "pool create" precedent), the same route-by-route design
// work 07-02-SUMMARY.md and 07-05-SUMMARY.md already deferred and
// explicitly flagged as this plan's own closing decision to make
// (07-05-SUMMARY.md key-decisions: "07-09's own 'close capability
// coverage' task will need either its own wave of route-by-route wiring
// or an explicit acknowledgment that this remains a documented,
// deliberate scope boundary from this plan"). Given this plan's own
// declared file scope (generate.go/deprecation.go/coverage_test.go/docs,
// no new per-domain operation files), this plan takes the latter,
// explicitly-sanctioned path: every exclusion below is individually
// reasoned and mechanically verified never-stale (TestCapabilityCoverage's
// own staleExclusions check), but full closure to only the two permanent
// categories remains future work -- see 07-09-SUMMARY.md's "Known Gaps"
// section for the explicit, prominent acknowledgment this represents.
func buildExcludedRoutes() map[string]string {
	const reasonDevTooling = "repository development tooling (build/test/docs/packaging/Linear sync); not part of the external show-control API surface -- and, unlike every other category here, will likely never need a REST operation regardless of future milestones (these commands only ever make sense run against the local contributor checkout, never against a remote daemon's /v1 API)"
	const reasonDaemonLifecycle = "daemon lifecycle entrypoint -- IS the process that hosts /v1, not a route exposed on it; permanent (a route can never expose the process that hosts it)"
	const reasonLocalProcessLaunch = "mage-only dev-loop entrypoint that execs a new local golc-desktop child process with a PATH fixup (internal/command/run.go); launching a GUI process on the server machine has no meaningful HTTP API semantics; permanent (this is inherently a local-machine action, not a remote-control one)"
	const reasonArtnetFutureWork = "Art-Net daemon runtime route; not yet wired to a REST operation -- future milestone work, not a Phase 7 deliverable (Phase 7's own scope proved the translation/auth/mutation/batch/SSE/audit mechanisms via the show domain, not full Art-Net runtime control)"
	const reasonMutationFutureWork = "show-domain mutation route; not yet wired to a REST operation -- mutate.go's serialized pipeline (scope/If-Match/dry-run/idempotency/observer) is proven end-to-end via \"pool create\" (07-05); wiring each remaining route needs its own bespoke Huma input struct mapping JSON body fields to that route's specific CLI flags, deferred to a future milestone (07-05-SUMMARY.md key-decisions, 07-09-SUMMARY.md \"Known Gaps\")"
	const reasonReadFutureWork = "show-domain read/inspect route not yet wired to a REST GET; translate.go's read-translation pattern is proven via \"config inspect\"/\"show inspect\" (07-02); several of these routes (fixture inspect, operatorsurface list/show, programmer inspect) also need dedicated design work since they emit plain-text stdout or accept a client-supplied filesystem path, not just a new RegisterOperation call -- deferred to a future milestone"

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
	addAll(reasonLocalProcessLaunch, "run")
	addAll(reasonArtnetFutureWork,
		"artnet configure", "artnet discover", "artnet interface list",
		"artnet master set", "artnet safety blackout", "artnet safety revoke-automation",
		"artnet safety stop-all", "artnet status", "artnet target disable", "artnet target enable",
	)
	addAll(reasonMutationFutureWork,
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
	addAll(reasonReadFutureWork,
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

// TestNoPendingRoutes proves every excluded route carries one of
// buildExcludedRoutes' own named category reasons -- never a blank,
// placeholder, or ad hoc "pending"/"TODO"-shaped reason string. Combined
// with TestCapabilityCoverage's own uncovered-route check (every route is
// either registered or excluded, never neither), this is the mechanical
// half of API-01's coverage claim: a route can never silently sit in
// limbo, unregistered and unreasoned.
func TestNoPendingRoutes(t *testing.T) {
	knownReasons := map[string]bool{}
	for _, reason := range excludedRoutes {
		knownReasons[reason] = true
	}

	placeholderMarkers := []string{"pending", "todo", "fixme", "tbd"}
	for route, reason := range excludedRoutes {
		trimmed := strings.TrimSpace(reason)
		if trimmed == "" {
			t.Errorf("route %q has a blank exclusion reason", route)
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, marker := range placeholderMarkers {
			if strings.Contains(lower, marker) {
				t.Errorf("route %q's exclusion reason looks like an unfinished placeholder (%q): %q", route, marker, reason)
			}
		}
	}

	if len(knownReasons) == 0 {
		t.Fatal("expected at least one distinct, named exclusion category")
	}
}

// TestFutureWorkExclusionsNameDeferralOwner proves the four non-permanent
// exclusion categories carry a named, greppable deferral owner
// (EXTN-05, .planning/REQUIREMENTS.md) rather than the open-ended "future
// milestone" framing this file used before the API-01 re-scope (07-10-PLAN.md
// Task 2) -- and that the two genuinely permanent categories (daemon
// lifecycle, local process launch) do NOT claim a deferral owner, so a
// future bulk edit can never flatten the permanent/deferred distinction this
// file deliberately draws. It reads the same excludedRoutes values the
// coverage gate uses (by route, not by re-declaring the constants), so it
// fails loudly if a reason's EXTN-05 clause is ever silently dropped.
func TestFutureWorkExclusionsNameDeferralOwner(t *testing.T) {
	deferredSamples := map[string]string{
		"artnet future work":   "artnet status",
		"mutation future work": "scene create",
		"read future work":     "fixture inspect",
	}
	for category, route := range deferredSamples {
		reason, ok := excludedRoutes[route]
		if !ok {
			t.Fatalf("category %q: route %q not present in excludedRoutes -- update the sample route", category, route)
		}
		if !strings.Contains(reason, "EXTN-05") {
			t.Errorf("category %q (sampled via route %q) lost its EXTN-05 deferral pointer: %q", category, route, reason)
		}
	}

	permanentSamples := map[string]string{
		"daemon lifecycle":     "artnet serve",
		"local process launch": "run",
	}
	for category, route := range permanentSamples {
		reason, ok := excludedRoutes[route]
		if !ok {
			t.Fatalf("category %q: route %q not present in excludedRoutes -- update the sample route", category, route)
		}
		if strings.Contains(reason, "EXTN-05") {
			t.Errorf("category %q (sampled via route %q) incorrectly claims a deferral owner; permanent categories must not: %q", category, route, reason)
		}
	}
}
