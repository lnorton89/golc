---
phase: 09-front-door-ui-completion
plan: 05
subsystem: fixture-catalog
tags: [go, react, wails, ssrf-guard, open-fixture-library, fixture-import]

# Dependency graph
requires:
  - phase: 09-front-door-ui-completion (plan 01)
    provides: FixtureLibraryWorkspace.tsx's browse/search/inline-inspect skeleton and FixtureLibraryService's ListLocal/Inspect bindings, extended by this plan
  - phase: 09-front-door-ui-completion (plan 02)
    provides: no direct code dependency; shared wave/UI-SPEC context
provides:
  - fixture.ImportEnvelope/DecodeEnvelope as the single shared fixture-import artifact shape
  - ListDirectory .json import-envelope support (inherited by the Art-Net fixture resolver)
  - ofl.FetchManufacturers/FilterManufacturers against the existing single SSRF-guarded host
  - FixtureLibraryService.SearchOFL with a once-per-process cached manufacturer index
  - FixtureLibraryWorkspace.tsx's "My Library" / "Open Fixture Library" source toggle and catalog search UI
affects: [09-06-front-door-ui-completion, 09-07-front-door-ui-completion]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared bounded, host-validated GET helper (ofl.getBounded) backing both Fetch and FetchManufacturers -- one SSRF guard, one timeout, one size cap"
    - "Once-per-process cached external-index fetch behind a mutex (FixtureLibraryService.loadOFLManufacturers), retried on the next call after a failure rather than permanently sticky"
    - "Client-side debounced search input shared by two list sources (local filter vs. catalog fetch) via a single source-toggle state"

key-files:
  created:
    - internal/fixture/envelope.go
    - internal/fixture/envelope_test.go
    - internal/fixture/ofl/manufacturers.go
    - internal/fixture/ofl/manufacturers_test.go
  modified:
    - internal/fixture/directory.go
    - internal/fixture/directory_test.go
    - internal/command/fixture.go
    - internal/fixture/ofl/fetch.go
    - internal/wails/svc_fixturelibrary.go
    - internal/wails/svc_fixturelibrary_test.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css

key-decisions:
  - "OFL catalog search is manufacturer-name-only for v1 (RESEARCH Open Question 1, resolved) -- no full fixture-model index is reachable from the single allowed raw.githubusercontent.com host without a second SSRF-relevant host decision; the scope is stated permanently on screen next to the results, not just in planning docs"
  - "fixture.ImportEnvelope replaces internal/command/fixture.go's private fixtureImportOutput -- exactly one definition of the import-artifact shape now exists in the repository"
  - "DecodeEnvelope introduces no new validation rule: it runs the decoded definition through the same exported fixture.Validate the YAML path and ofl.Normalize already share"

patterns-established:
  - "Pattern: extract a bounded/guarded network primitive (getBounded) once a second caller (FetchManufacturers) needs the identical SSRF guard/timeout/size-cap contract, rather than duplicating it"

requirements-completed: [FDUI-01]

coverage:
  - id: D1
    description: "An imported fixture (fixture import --out) becomes visible in the Fixture Library list and resolvable by the Art-Net fixture resolver"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/fixture/directory_test.go#TestListDirectoryIncludesImportArtifacts"
        status: pass
      - kind: unit
        ref: "internal/fixture/envelope_test.go#TestDecodeEnvelopeAcceptsImportArtifact"
        status: pass
    human_judgment: false
  - id: D2
    description: "A malformed/invalid .json import artifact is rejected with GOLC_FIXTURE_ENVELOPE_INVALID and never silently produces a zero-valued definition or aborts the directory scan"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/fixture/envelope_test.go#TestDecodeEnvelopeRejectsBareDefinition"
        status: pass
      - kind: unit
        ref: "internal/fixture/envelope_test.go#TestDecodeEnvelopeRejectsInvalidDefinition"
        status: pass
    human_judgment: false
  - id: D3
    description: "The Fixture Library has a 'My Library' / 'Open Fixture Library' source toggle; the catalog side searches OFL manufacturer names via SearchOFL"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#the source toggle switches between the local list and the catalog"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders manufacturer rows for a matching query"
        status: pass
    human_judgment: false
  - id: D4
    description: "Catalog empty/no-results/unreachable states render the exact UI-SPEC copy, with unreachable using the offline status tone and never a thrown error"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the catalog empty prompt with no query"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the no-results copy with the query interpolated"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the unreachable copy with the offline tone"
        status: pass
    human_judgment: false
  - id: D5
    description: "The OFL host allowlist is unchanged: the manufacturer index is fetched from the same single explicitly compared host, never a second host, suffix, or wildcard"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "internal/fixture/ofl/manufacturers_test.go#TestFetchManufacturersRejectsForeignHostWithoutOptIn"
        status: pass
      - kind: other
        ref: "grep -rn api.github.com internal/ | wc -l == 0; grep -v '^//' internal/fixture/ofl/manufacturers.go | grep -c -E 'HasSuffix|Contains\\(.*Hostname' == 0"
        status: pass
    human_judgment: false
  - id: D6
    description: "Manual desktop-app verification: importing a fixture via the CLI makes it appear in the running app's Fixture Library, and the catalog search shows real OFL manufacturers / the offline copy when disconnected"
    verification: []
    human_judgment: true
    rationale: "Requires running the built golc-desktop app against a live/disconnected network and visually confirming the list/chip/copy render as intended -- outside this automated unit-test suite's reach."

# Metrics
duration: 45min
completed: 2026-07-28
status: complete
---

# Phase 09 Plan 05: Fixture-Import Visibility and Open Fixture Library Catalog Search Summary

**Imported fixtures now decode through a shared `fixture.ImportEnvelope`/`DecodeEnvelope` type into the library and Art-Net resolver, and the Fixture Library workspace gained an OFL manufacturer-name catalog search behind the existing single-host SSRF guard.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-07-28T00:00:00-07:00 (approx.)
- **Completed:** 2026-07-28T00:14:25-07:00
- **Tasks:** 3
- **Files modified:** 14 (4 created, 10 modified)

## Accomplishments
- `fixture.ImportEnvelope`/`DecodeEnvelope` is the single shared `{definition, provenance}` shape "fixture import --out" writes, replacing `internal/command/fixture.go`'s private duplicate struct; `DecodeEnvelope` runs the decoded definition through the exact same `fixture.Validate` the YAML path and `ofl.Normalize` already share, introducing no new validation rule.
- `ListDirectory` now decodes `.json` import envelopes alongside `.yaml`/`.yml`, carrying the envelope's `Provenance` onto `DirectoryEntry` -- inherited automatically by `internal/command/artnet.go`'s Art-Net fixture resolver, which previously could not see imported fixtures at all.
- `FixtureLibraryService.ListLocal` now projects an imported row's OFL provenance into its `source` field ("ofl" vs. "local"), making the catalog origin of a library row visible rather than indistinguishable from a hand-authored one.
- `internal/fixture/ofl/fetch.go`'s bounded, host-validated GET was extracted into a shared `getBounded` helper so the SSRF guard, timeout, and size cap have exactly one implementation across both the per-fixture fetch and the new manufacturer-index fetch.
- `ofl.FetchManufacturers`/`FilterManufacturers` implement D-01/D-03's OFL catalog search against the manufacturer index at `fixtures/manufacturers.json` under the identical `raw.githubusercontent.com` host the existing SSRF guard already allows -- no second host constant was added.
- `FixtureLibraryService.SearchOFL` lazily fetches and caches the manufacturer index once per process (mutex-guarded), so a typing burst issues at most one network request; a fetch failure returns an explicit `unreachable:true` view rather than a thrown error.
- `FixtureLibraryWorkspace.tsx` gained the "My Library" / "Open Fixture Library" source toggle, a debounced (~250ms) catalog search over the same input, the UI-SPEC's exact empty/no-results/unreachable copy, and a permanently visible note stating the manufacturer-name-only search scope.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write the failing envelope, catalog-index, and catalog-search tests** - `32267256` (test)
2. **Task 2: Make imported fixtures visible in the library and to the Art-Net resolver** - `ae5ce952` (feat)
3. **Task 3: Add Open Fixture Library catalog search end to end** - `a72e59a2` (feat)

_Note: this plan's tasks are each `tdd="true"`; Task 1 is the RED commit shared by all three tasks' GREEN implementations (Tasks 2 and 3 both turn tests from that one RED commit green)._

## Files Created/Modified
- `internal/fixture/envelope.go` - `ImportEnvelope` type and `DecodeEnvelope` (GOLC_FIXTURE_ENVELOPE_INVALID diagnostic)
- `internal/fixture/envelope_test.go` - envelope decode/reject tests
- `internal/fixture/directory.go` - widened `ListDirectory` to decode `.json` import envelopes; added `Provenance` to `DirectoryEntry`
- `internal/fixture/directory_test.go` - added `TestListDirectoryIncludesImportArtifacts`
- `internal/command/fixture.go` - deleted private `fixtureImportOutput`; writes `fixture.ImportEnvelope` instead (byte-identical output)
- `internal/fixture/ofl/fetch.go` - extracted `getBounded` shared SSRF-guarded GET helper
- `internal/fixture/ofl/manufacturers.go` - `Manufacturer`, `ManufacturerIndexRef`, `FetchManufacturers`, `FilterManufacturers`
- `internal/fixture/ofl/manufacturers_test.go` - manufacturer-index fetch/filter tests (httptest only, no live network)
- `internal/wails/svc_fixturelibrary.go` - `OFLManufacturerView`, `OFLSearchView`, `SearchOFL`, cached manufacturer-index loader; `ListLocal` row source projection
- `internal/wails/svc_fixturelibrary_test.go` - `TestFixtureLibraryServiceSearchOFLReportsUnreachableWithoutThrowing`
- `frontend/src/lib/wailsBridge.ts` - `OflManufacturerView`, `OflSearchView` types; `searchOflManufacturers`, `offlineOflSearchView` exports
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` - source toggle, debounced catalog search, empty/no-results/unreachable states, scope note
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` - catalog search test suite (6 new tests)
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css` - source-toggle and catalog-state styles

## Decisions Made
- Kept `ManufacturerIndexRef`'s `Mirror`/`AllowMirror` shape identical to `OFLRef`'s existing opt-in fields, purely so tests can point at an `httptest` server exactly like `fetch_test.go` already does -- no new SSRF surface, no new opt-in semantics.
- `FixtureLibraryService.oflIndexRef` is an unexported, zero-valued-by-default field: production always resolves to the default upstream host; only this package's own test overrides it (to a deliberately unroutable loopback address, `http://127.0.0.1:1`) to make the unreachable-catalog test deterministic without depending on this environment's real network reachability.
- A cached `SearchOFL` fetch failure is retried on the next call rather than permanently sticky, so a transient network blip does not wedge the catalog for the rest of the session.

## Deviations from Plan

None - plan executed exactly as written. The two RESEARCH-flagged scope decisions (manufacturer-name-only search, no second SSRF host) were already resolved at planning time (COVERAGE.md, 09-RESEARCH.md Open Question 1) and were implemented as specified, not decided ad hoc during execution.

## Issues Encountered
- The worktree had no `frontend/node_modules` (git worktrees don't share npm-installed dependencies with the main checkout) and no `cmd/golc-desktop/frontend/dist` (required by `go:embed` in `cmd/golc-desktop/main.go`), so an initial `go build ./...` and `npm test` both failed for reasons unrelated to this plan's code. Resolved by running `npm ci` in `frontend/` and `npm run build` (which also runs `tsc --noEmit` and the full `vitest run` suite) before final verification -- both are one-time worktree setup steps, not code changes.
- `go test ./...` shows five pre-existing failures in `internal/command` (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) — all `GOLC_TEST_TOOLCHAIN_MISSING`/`pinned golc-project binary not built`, requiring `mage Bootstrap` to have run in this worktree. These are environment-bootstrap gaps unrelated to any file this plan touches (confirmed unaffected by fixture-scoped `-run` filters) and are out of this plan's scope per the deviation rules' scope boundary — logged here rather than fixed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `FDUI-01` spans plans 09-01, 09-05, 09-06, and 09-07; this plan does not mark it complete in `REQUIREMENTS.md` since `09-06` (wave 4) and `09-07` (wave 5) have not executed yet.
- `09-06-PLAN.md` can now build the fixture-key field and "Add to Library" import action on top of this plan's catalog search (manufacturer selection) and the shared `fixture.ImportEnvelope`/`DecodeEnvelope` path.
- `09-07-PLAN.md` (custom YAML add) is unaffected by this plan's changes and remains ready independently.
- The manual desktop-app verification steps noted in Task 2/Task 3 (`<human-check>` blocks: importing a fixture via the CLI and confirming it appears in the running app; disconnecting the network and confirming the offline copy renders) were not run as part of this automated execution and remain open for a human verification pass.

---
*Phase: 09-front-door-ui-completion*
*Completed: 2026-07-28*

## Self-Check: PASSED

All created files (`internal/fixture/envelope.go`, `internal/fixture/ofl/manufacturers.go`, `internal/wails/svc_fixturelibrary.go`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx`, this SUMMARY.md) and all three task commit hashes (`32267256`, `ae5ce952`, `a72e59a2`) were verified present on disk / in `git log --oneline --all` before this file was finalized.
