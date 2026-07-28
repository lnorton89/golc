---
phase: 09-front-door-ui-completion
plan: 01
subsystem: ui
tags: [go, react, wails, fixture, typescript, vitest]

# Dependency graph
requires:
  - phase: 02-fixture-domain
    provides: internal/fixture.Decode/Pin/Validate and the "fixture inspect" CLI route this plan wires against
  - phase: 06-wails-shell
    provides: the Wails service pattern (execute helper, Bind list, primitives) this plan's FixtureLibraryService/workspace follow
provides:
  - internal/fixture.ListDirectory, the single local-fixture-directory scan (extracted from internal/command/artnet.go)
  - internal/wails.FixtureLibraryService (ListLocal/Inspect) bound to the desktop shell
  - frontend/src/lib/wailsBridge.ts fixture-library bridge exports (listLocalFixtures, inspectFixtureFile)
  - a real Fixture Library workspace (browse, search, inline inspect) replacing the ComingSoon stub
affects: [09-02, 09-03, 09-04, 09-05, 09-06, 09-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Wails-only read projection (no CLI-parity list route) mirroring ShowService.Inspect/FixturePatchService.ListPatch"
    - "Preview/inspect mutation forwards to the existing registered CLI route via the execute() helper, never a second decode/validate implementation"
    - "Bridge accessor / offline-fallback constant / try-catch wrapper trio for every new bridge export (never throws to the caller)"

key-files:
  created:
    - internal/fixture/directory.go
    - internal/fixture/directory_test.go
    - internal/wails/svc_fixturelibrary.go
    - internal/wails/svc_fixturelibrary_test.go
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css
  modified:
    - internal/command/artnet.go
    - cmd/golc-desktop/main.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx
    - frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx

key-decisions:
  - "Extracted the directory scan into internal/fixture.ListDirectory (slice, not map) so both the artnet resolver and the new Wails service share exactly one implementation, per-entry failures never abort the whole scan"
  - "A not-yet-created fixtures directory renders as an empty library (errors.Is(err, fs.ErrNotExist)), never an error state"
  - "Inline inspect is a second Panel in the same workspace grid, never a dialog/modal (D-02)"
  - "Search is client-side substring match over already-fetched rows, no backend round-trip (D-03)"

patterns-established:
  - "FixtureLibraryService.Inspect decodes nothing itself -- it forwards to the registered 'fixture inspect' route and only projects its allowlisted JSON envelope, so a fixture the CLI rejects can never render as usable on screen"

requirements-completed: [FDUI-01]

coverage:
  - id: D1
    description: "An operator opening Build -> Fixture Library sees every valid local fixture as a row with model, manufacturer, and a validation-status chip"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders local library rows with manufacturer and validation chip"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestFixtureLibraryServiceListLocalProjectsRowsSortedByStableKey"
        status: pass
    human_judgment: false
  - id: D2
    description: "Empty/invalid-file states never blank the library: a not-yet-created directory renders 'No fixtures yet', and one malformed file never aborts the scan"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the no-fixtures empty state"
        status: pass
      - kind: unit
        ref: "internal/fixture/directory_test.go#TestListDirectoryRecordsPerEntryFailureWithoutAbortingScan"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestFixtureLibraryServiceListLocalMissingDirectoryIsEmptyNotError"
        status: pass
    human_judgment: false
  - id: D3
    description: "Text search filters rows case-insensitively by manufacturer or model, client-side, no faceted controls"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#filters rows by search text, case-insensitively, matching manufacturer or model"
        status: pass
    human_judgment: false
  - id: D4
    description: "Selecting a row shows its inspect detail inline (stable key, content hash, revision, validation, warnings) in the same workspace, never a dialog"
    requirement: "FDUI-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders inline inspect detail for the selected row, not a dialog"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the validation-failure copy when the selected row's inspect result is invalid"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx#renders the lossy-import warning copy distinctly from the invalid state"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_fixturelibrary_test.go#TestFixtureLibraryServiceInspectReportsInvalidWithDiagnostics"
        status: pass
    human_judgment: false
  - id: D5
    description: "Launch golc-desktop, open Build -> Fixture Library, and visually confirm the real list/inspect surface against a live fixtures directory"
    verification: []
    human_judgment: true
    rationale: "Requires the actual Wails desktop shell running against a live fixtures directory (human-check step in the plan's own <verify> block) -- not exercisable from an automated unit/integration test in this environment"

# Metrics
duration: ~40min
completed: 2026-07-27
status: complete
---

# Phase 9 Plan 1: Fixture Library Workspace Summary

**Real browse/search/inline-inspect Fixture Library workspace backed by a new `internal/fixture.ListDirectory` scan and `FixtureLibraryService` Wails binding, replacing the `ComingSoon` CLI-hint stub.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3 (RED test authoring, list GREEN, search+inspect GREEN)
- **Files modified:** 11 (6 created, 5 modified)

## Accomplishments

- Extracted the single local-fixture-directory scan (`internal/fixture.ListDirectory`) from `internal/command/artnet.go`'s private `loadFixtureDirectory`, with per-entry decode/pin failures recorded on that entry rather than aborting the scan, and `artnet.go` now delegates to it (exactly one directory-scan implementation in the repo)
- Added `internal/wails.FixtureLibraryService` (`ListLocal`/`Inspect`), bound in `cmd/golc-desktop/main.go`: `ListLocal` is a Wails-only read projection over `ListDirectory` (never decodes/pins itself); `Inspect` forwards to the existing, already-tested `fixture inspect` CLI route
- Extended `frontend/src/lib/wailsBridge.ts` with `FixtureLibraryRowView`/`FixtureLibraryView`/`FixtureWarningView`/`FixtureInspectView` types and `listLocalFixtures`/`inspectFixtureFile` bridge functions (accessor / offline-fallback / try-catch wrapper trio, never throws)
- Replaced `FixtureLibraryWorkspace.tsx`'s `ComingSoon` stub with a real workspace: a scrollable, searchable list of local fixtures with validation-status chips, and an inline inspect panel (stable key, content hash, revision, validation verdict, lossy-import warnings) — never a dialog/modal

## Task Commits

1. **Task 1: Write the failing tests for local fixture listing, projection, and workspace rendering** - `7e24bf8` (test)
2. **Task 2: Ship the end-to-end local library list** - `18e98be` (feat)
3. **Task 3: Add inline inspect and search to the Fixture Library workspace** - `c20b1a0` (feat)

**Follow-up:** `daa238d` (style: gofmt struct-tag alignment; deferred-items.md logging pre-existing unrelated test failures)

## Files Created/Modified

- `internal/fixture/directory.go` - `ListDirectory`/`DirectoryEntry`, the single extracted local-fixture-directory scan
- `internal/fixture/directory_test.go` - scan/skip/per-entry-failure/missing-directory test coverage
- `internal/wails/svc_fixturelibrary.go` - `FixtureLibraryService.ListLocal`/`.Inspect` Wails bindings
- `internal/wails/svc_fixturelibrary_test.go` - binding test coverage, including the real-JSON-marshal nil-slice pin
- `internal/command/artnet.go` - `loadFixtureDirectory` now delegates to `fixture.ListDirectory`
- `cmd/golc-desktop/main.go` - constructs and binds `FixtureLibraryService`
- `frontend/src/lib/wailsBridge.ts` - fixture-library types + bridge functions
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` - real browse/search/inspect workspace
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.module.css` - workspace styles (spacing scale per UI-SPEC)
- `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` - full rewrite, real-workspace assertions

## Decisions Made

- Reused `decode_test.go`'s package-level `validRGBParYAML` constant in `directory_test.go` (same `fixture_test` package) rather than inventing a second minimal fixture fixture shape.
- `ListLocal`'s directory label reuses `internal/command/fixture.go`'s `fixtureInspectSource` repo-relative/`external:<basename>` discipline (T-09-01-01), reimplemented locally since it's a private helper in a different package.
- Panel header fixture count reflects the total library count (unfiltered), not the search-filtered count — matches `OverviewWorkspace`'s existing pool/deployment count convention and keeps "N fixtures" stable while typing a search query.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed frontend dependencies in the worktree**
- **Found during:** Task 1 (running the frontend test suite to confirm RED)
- **Issue:** This git worktree had no `frontend/node_modules` (untracked/gitignored, not shared across worktrees)
- **Fix:** Ran `npm ci` in `frontend/` against the existing `package-lock.json` — no dependency versions changed
- **Files modified:** none (node_modules is gitignored)
- **Verification:** `npx vitest run` executed successfully afterward

**2. [Rule 1 - Bug] gofmt struct-tag alignment**
- **Found during:** post-Task-3 formatting check
- **Issue:** `internal/wails/svc_fixturelibrary.go`'s struct tags had stale column alignment after edits
- **Fix:** Ran `gofmt -w` on the file
- **Files modified:** `internal/wails/svc_fixturelibrary.go`
- **Verification:** `gofmt -l` reports clean; `go build ./...` and `go test ./internal/wails/... -count=1` still pass
- **Commit:** `daa238d`

### Plan-Authoring Assumption Mismatches (not code defects, documented not "fixed")

**3. `cmd/golc-desktop/main.go`'s "NewFixtureLibraryService count is 2" acceptance criterion**
- The plan's acceptance criteria expected `grep -c "NewFixtureLibraryService" cmd/golc-desktop/main.go` to be 2 ("construction plus the Bind entry"). The established codebase convention (every existing sibling service — `showService`, `scriptService`, etc.) uses a lowerCamelCase local variable name in the `Bind: [...]` list, never repeating the constructor name. Verified: `grep -c "NewShowService" cmd/golc-desktop/main.go` is also 1, not 2. Kept the established convention (`fixtureLibraryService` in the Bind list) rather than introducing an inconsistent naming pattern just to satisfy the literal grep count.

**4. `wailsBridge.ts`'s "declare global count is 1" acceptance criterion**
- The plan expected `grep -c "declare global" frontend/src/lib/wailsBridge.ts` to be 1. The file's own pre-existing doc comment (above the actual `declare global {` block, untouched by this plan) mentions the phrase "declare global" twice in prose explaining why there's only one real block. Verified via `git show <pre-Task-2-commit>:frontend/src/lib/wailsBridge.ts | grep -c "declare global"` = 3, confirming this is pre-existing, not introduced by this plan. The actual invariant (exactly one real `declare global { ... }` code block) is intact — only one exists, at its original location.

**5. `go test ./internal/command/...` and `go test ./...` pre-existing unrelated failures**
- Five `internal/command` tests (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`) fail with `GOLC_TEST_TOOLCHAIN_MISSING` because this fresh worktree has never run `mage Bootstrap` (no `.tools/toolchains/`, no `.tools/installs/`). One `internal/trace/catalog` test (`TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline`) fails on a pre-existing `.planning/linear-map.json` drift, an unrelated domain this plan never touches. All artnet-specific tests (`go test ./internal/command/... -run 'TestArtnet...'`) pass. Logged to `.planning/phases/09-front-door-ui-completion/deferred-items.md` per the scope-boundary rule; not fixed (out of scope, pre-existing, environmental).

---

**Total deviations:** 2 auto-fixed (1 blocking dependency install, 1 formatting), 3 documented plan-authoring/environment mismatches.
**Impact on plan:** No scope creep. The two grep-count mismatches are corrected in this SUMMARY's documentation, not in code, because the actual code already satisfies the underlying invariant each grep check was trying to verify (single Bind entry per service; single real `declare global` block).

## Issues Encountered

- `go build ./cmd/golc-desktop/...` fails until `frontend/dist` exists (the package embeds it via `//go:embed all:frontend/dist`). Resolved by running the frontend build (`npm run build` / `npx vite build`) before the final `go build ./...` verification — this is the pre-existing, expected build order for this binary, not a regression.
- `npm run build`'s `vitest run` step runs the entire frontend suite, not just `FixtureLibraryWorkspace` — so it could only turn green once Task 3 (search + inline inspect) was implemented, even though Task 2's own acceptance criteria listed `npm --prefix frontend run build succeeds` as a checkbox. Verified Task 2's actual scoped criteria (`npm --prefix frontend test -- FixtureLibraryWorkspace` passing list/empty-state cases, `tsc --noEmit`, `go build`) independently at that point, then confirmed the full `npm run build` (228/228 tests, tsc, vite build) after Task 3 landed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- FDUI-01's local-browse half is delivered: an operator can browse, search, and inspect their local fixture library entirely on screen.
- `internal/fixture.ListDirectory` is the single shared scan implementation `internal/wails/svc_fixturelibrary.go` and `internal/command/artnet.go` both depend on — a future plan adding OFL catalog search (D-01's remaining "Open Fixture Library" half) can extend `FixtureLibraryService` alongside this without touching the local half.
- Human verification still needed: launching `golc-desktop`, opening Build -> Fixture Library, and confirming the panel against a real fixtures directory (D5 above) — flagged for end-of-phase UAT per this plan's own `<verify><human-check>` step.

---
*Phase: 09-front-door-ui-completion*
*Completed: 2026-07-27*
