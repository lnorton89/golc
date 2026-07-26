---
phase: 08-isolated-typescript-automation
plan: 01
subsystem: scripting
tags: [go, sqlite, cli, typescript-automation, show-state]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: show.APIKeyScope closed set (playback/authoring/admin) and the internal/api/ratelimit.go safe-default discipline this plan's CapabilityProfile.Scope and ResolveResourceLimits reuse directly
provides:
  - "show.Script/show.CapabilityProfile entity persisted inside show.State (show.Scripts, json omitempty)"
  - "show.NewScript/ValidateScript/ValidateScriptUniqueNames/CapabilityProfile.ResolveResourceLimits"
  - "internal/command/script.go: script create/list/show/edit/delete/profile set CLI routes"
affects: [08-02, 08-03, 08-04, 08-05, 08-06, 08-07, 08-08, 08-09, 08-10, 08-11]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Script entity lives in package show (not a new internal/script package) so CapabilityProfile.Scope can import show.APIKeyScope directly with zero import cycle risk -- mirrors internal/show/apikeys.go's in-package-entity precedent"
    - "Named resource presets (quick-action/long-running-automation) always resolve to fixed values regardless of custom fields; only the advanced preset reads custom fields, and any zero/negative/absent custom field falls back to the package safe default -- mirrors internal/api/ratelimit.go's limit()/burstOrDefault() discipline"
    - "Every script CLI route self-registers via MustDeclareRoute/MustDeclareScope from package-level var initializers (internal/command/router.go's established contract) -- no central switch edit needed for future script routes"

key-files:
  created:
    - internal/show/scripts.go
    - internal/show/scripts_test.go
    - internal/command/script.go
    - internal/command/script_test.go
  modified:
    - internal/show/state.go

key-decisions:
  - "CapabilityProfile.Scope is typed show.APIKeyScope, imported directly from internal/show/apikeys.go -- there is exactly one scope enum in the codebase (D-06), enforced structurally (internal/show/scripts.go never declares a parallel APIKeyScope type)."
  - "Added a defensive sanity ceiling (GOLC_SCRIPT_LIMIT_INVALID) on CapabilityProfile's advanced-preset numeric fields, mirroring internal/scene/scene.go's maxBarsPerLoop DoS-ceiling precedent -- a zero/negative value still safely falls back to the package default per D-09, only a pathologically large explicit value is rejected."
  - "Every script CLI route writes deterministic JSON to Stdout unconditionally (no default-text/--json-opt-in split like scene.go/apikey.go use) -- the D-16 library view and D-07 run dialog both consume this output directly, so plain-text rendering would add a second shape with no consumer."

requirements-completed: [SCRP-01, SCRP-04]

coverage:
  - id: D1
    description: "show.Script and show.CapabilityProfile persist inside show.State (show.Scripts, D-17), with NewScript minting a stable UUIDv7 identity at a least-privileged default profile, ValidateScript/ValidateScriptUniqueNames enforcing every invariant, and ResolveResourceLimits resolving D-09's named presets deterministically (a zero/negative/absent advanced-preset value is never treated as unlimited)."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/show/scripts_test.go#TestNewScript"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestNewScriptMintsDistinctIDs"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestNewScriptRejectsEmptyName"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestValidateScript"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestValidateScriptUniqueNames"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestResolveResourceLimitsQuickAction"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestResolveResourceLimitsLongRunning"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestResolveResourceLimitsAdvancedFallsBackToSafeDefaults"
        status: pass
      - kind: unit
        ref: "internal/show/scripts_test.go#TestResolveResourceLimitsAdvancedHonorsExplicitValues"
        status: pass
      - kind: integration
        ref: "internal/show/scripts_test.go#TestShowStateScriptValidation"
        status: pass
    human_judgment: false
  - id: D2
    description: "The script CLI command graph (script create/list/show/edit/delete/profile set) is reachable, self-registered, and produces the exact JSON projections downstream plans consume: script list omits source (D-16 library shape), script show/create/edit/profile-set return the full script, script edit persists --source-file bytes verbatim under a 1MiB cap, and script profile set carries forward every field the caller did not mention (D-07)."
    requirement: "SCRP-04"
    verification:
      - kind: integration
        ref: "internal/command/script_test.go#TestScriptRoutesFullLifecycle"
        status: pass
      - kind: integration
        ref: "internal/command/script_test.go#TestScriptRoutesUsageErrors"
        status: pass
      - kind: integration
        ref: "internal/command/script_test.go#TestScriptCommandBinary"
        status: pass
    human_judgment: false

# Metrics
duration: 8min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 1: Script Entity and CLI Command Graph Summary

**Script/CapabilityProfile entity added to show.State (D-06/D-07/D-09/D-14/D-17) plus a self-registered `script create/list/show/edit/delete/profile set` CLI route graph producing the exact D-16 library-view JSON projection.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-26T00:46:27Z
- **Completed:** 2026-07-26T00:56:57Z
- **Tasks:** 2 completed
- **Files modified:** 5 (4 created, 1 modified)

## Accomplishments
- `show.Script`/`show.CapabilityProfile` entities live inside `show.State` (`Scripts []Script`, `json:"scripts,omitempty"`), validated from the existing single `validate()` entry point every other entity extends -- no new SQLite table.
- `show.NewScript` mints a stable UUIDv7 identity at the least-privileged `playback` scope and `quick-action` preset; `ResolveResourceLimits` resolves both named presets to fixed values and the `advanced` escape hatch to safe-default-backed custom values, exactly matching D-09's "never unlimited" rule.
- Six self-registered CLI routes (`script create`/`list`/`show`/`edit`/`delete`/`profile set`) give SCRP-01's create/edit verbs and SCRP-04's assignment surface a working, tested CLI surface every later Phase 8 plan (Deno host, SDK, workspace, debugger) can build on without re-litigating where a script lives.

## Task Commits

Each task was committed atomically as a TDD RED/GREEN pair:

1. **Task 1: Script entity and capability profile in show.State**
   - `af2cd1b` (test) - RED: failing show_test coverage for NewScript/ValidateScript/ValidateScriptUniqueNames/ResolveResourceLimits
   - `d2fc5e2` (feat) - GREEN: internal/show/scripts.go + state.go wiring
2. **Task 2: script CLI command graph**
   - `07161d0` (test) - RED: failing command_test coverage for the full six-route lifecycle
   - `4439177` (feat) - GREEN: internal/command/script.go

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_TDD tasks: each task has a separate RED (test) commit before its GREEN (feat) commit, per the plan's `tdd="true"` gate._

## Files Created/Modified
- `internal/show/scripts.go` - `Script`/`CapabilityProfile`/`ScriptRunStatus`/`ResourcePreset` types, `NewScript`, `ValidateScript`, `ValidateScriptUniqueNames`, `CapabilityProfile.ResolveResourceLimits`
- `internal/show/scripts_test.go` - table-driven `show_test` coverage for every `<behavior>` bullet, including a Save/Load round-trip
- `internal/show/state.go` - added `State.Scripts []Script` and wired `ValidateScript`/`ValidateScriptUniqueNames` into `validate()`
- `internal/command/script.go` - self-registered `script create/list/show/edit/delete/profile set` routes, JSON view projections
- `internal/command/script_test.go` - full-lifecycle route test plus usage-error and fresh-show empty-array coverage

## Decisions Made
- `CapabilityProfile.Scope` imports `show.APIKeyScope` directly rather than declaring a parallel enum (D-06) -- verified structurally (`grep -c 'type APIKeyScope\|APIKeyScope string' internal/show/scripts.go` is 0).
- Added `GOLC_SCRIPT_LIMIT_INVALID` sanity-ceiling validation on the `advanced` preset's numeric fields (mirrors `scene.go`'s `maxBarsPerLoop` DoS-ceiling precedent) so the diagnostic the plan named actually has a real trigger, without contradicting D-09's "zero/negative/absent silently falls back to default" rule (only a pathologically *large* explicit value is rejected).
- Every script route writes JSON unconditionally to Stdout (no scene.go/api-key.go-style default-plain-text-plus---json toggle), since every consumer (D-16 library view, D-07 run dialog) needs structured data, not a human-readable table.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Reject unknown CLI flags instead of silently accepting them**
- **Found during:** Task 2 (script CLI command graph), while writing `TestScriptRoutesUsageErrors`
- **Issue:** The initial flag parser (`parseScriptPositionalArgs`/`parseScriptFlags`) accepted any `--flag value` pair into a generic map with no allowlist check, so `script create Chase --bogus value --show <path>` exited 0 instead of the plan-required "a malformed invocation (missing `--show`, unknown flag) exits 2."
- **Fix:** Added `rejectUnknownScriptFlags(usage, flags, allowed)` and called it in every handler (`create`/`list`/`show`/`edit`/`delete`/`profile set`) with that route's exact allowed-flag set.
- **Files modified:** internal/command/script.go
- **Verification:** `TestScriptRoutesUsageErrors` (unknown-flag case) passes.
- **Committed in:** `4439177` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Necessary for correctness against the plan's own stated behavior ("a malformed invocation... exits 2"). No scope creep.

## Issues Encountered
- This worktree has no `.tools/` bootstrap: `TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, and `TestScopeOfflineAcceptance` in `internal/command`, plus a whole-repo `go build ./...` (blocked on `cmd/golc-desktop`'s `frontend/dist` embed), fail for pre-existing environment reasons unrelated to this plan's changes. Logged to `.planning/phases/08-isolated-typescript-automation/deferred-items.md`; every test this plan actually touches (`internal/show`, the rest of `internal/command`, `internal/routecatalog`) is green, and `go build ./internal/... ./cmd/golc-project/...` succeeds cleanly.
- The plan's own acceptance-criteria grep `grep -n 'Scripts \[\]Script' internal/show/state.go` (single space) does not match gofmt's column-aligned struct output (`Scripts          []Script`, padded to align with `OperatorSurfaces`). Verified intent instead with `grep -nE 'Scripts +\[\]Script' internal/show/state.go`, which matches on line 58 -- the field exists exactly as specified, gofmt-compliant.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `show.Script`/`show.CapabilityProfile` and the six `script *` CLI routes are the stable, tested foundation every remaining Phase 8 plan (08-02 through 08-11: Deno host, capability enforcement, SDK generation, script workspace UI, debugger) reads and writes -- no later plan needs to re-derive where a script lives, which scope enum it uses, or what its list/show JSON projection looks like.
- No blockers. `internal/script`/`internal/scriptsdk` (this phase's later, still-greenfield packages) can now depend on `show.Script`/`show.CapabilityProfile` as a stable import.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
