---
phase: 03-deterministic-show-programming-and-playback
plan: 01
subsystem: programming
tags: [go, cli, uuid, fixture-capability, selection-resolution]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments
    provides: pool.Pool/pool.Group/deployment.Deployment domain types and fixture.CapabilityType normalized [0,1] range model
provides:
  - programming.Selection/Resolve (PROG-01 fixture selection resolution against pool/group/deployment/direct-fixture selectors)
  - programming.ProgrammerState/SetAttribute/Touched/Clear/ValidateProgrammer (PROG-02/PROG-03 semantic attribute editing and inspection)
  - show.State.Programmer persistence and the "programmer set/inspect/clear" CLI scope
affects: [03-02, 03-03, 03-04, 03-05, 03-06, 03-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Resolve(pools, groups, deployments, sel) takes plain slices, never show.State, to avoid a show->programming import cycle (mirrors internal/pool/impact.go)"
    - "GOLC_{DOMAIN}_{CONDITION} diagnostic convention extended with GOLC_SELECTION_*/GOLC_PROGRAMMER_* codes"
    - "Value validation reuses fixture.SupportedCapabilityTypes/normalized [0,1] bound directly -- no parallel range constant"

key-files:
  created:
    - internal/programming/selection.go
    - internal/programming/selection_test.go
    - internal/programming/programmer.go
    - internal/programming/programmer_test.go
    - internal/command/programming.go
    - internal/command/programming_test.go
    - .planning/phases/03-deterministic-show-programming-and-playback/deferred-items.md
  modified:
    - internal/show/state.go

key-decisions:
  - "PROG-03 'record scope' resolved as the TouchedAttribute's own InstanceID (the exact instance it will be recorded into) rather than a separate stored ResolvedSet field -- simplest reading consistent with the plan's flagged_assumptions and its exact 6-function API list (no SetScope/scope-setter function exists)."
  - "programmer set's --instance/--pool/--group/--fixture selectors build a programming.Selection resolved via programming.Resolve before any SetAttribute call, so a dangling reference is rejected before any attribute mutation happens."
  - "Malformed --fixture value shape parsing uses GOLC_SELECTION_USAGE (a Selection-input parsing error) while every other CLI usage error uses GOLC_PROGRAMMER_USAGE -- both diagnostic codes were declared in the plan's artifacts_produced and needed a clear split."

patterns-established:
  - "Selection resolution: build lookup sets, validate every selector resolves first (dangling reference is fatal, never silently smaller), then walk deployments/instances in the caller's own declaration order for deterministic, deduped output."
  - "Programmer buffer: an ordered slice keyed by (InstanceID, Capability) with overwrite-in-place semantics, never a map, so Touched() output is stable across repeated calls."

requirements-completed: [PROG-01, PROG-02, PROG-03]

coverage:
  - id: D1
    description: "A show author can resolve a mixed pool/group/deployment-instance/direct-fixture selection into a deterministic, deduped set of concrete fixture instances, with dangling references rejected rather than silently dropped."
    requirement: "PROG-01"
    verification:
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionResolvesPool"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionResolvesGroup"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionResolvesDeploymentInstance"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionResolvesDirectFixtureRef"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionOverlapDedupes"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionEmptyZeroSelectors"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionEmptyPool"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionStableOrdering"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionDanglingPool"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionDanglingGroup"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionDanglingInstance"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionDanglingFixtureRefUnknownPool"
        status: pass
      - kind: unit
        ref: "internal/programming/selection_test.go#TestSelectionDanglingFixtureRefUnknownMember"
        status: pass
    human_judgment: false
  - id: D2
    description: "A show author can set semantic intensity/color/position/beam attributes on resolved instances as normalized [0,1] values (never raw DMX), with out-of-range values and unsupported capabilities rejected and nothing recorded; the programmer buffer reports every touched attribute with value/source/record scope, in stable order, with no phantom entries."
    requirement: "PROG-02"
    verification:
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerSetAttributeInRange"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerSetAttributeOutOfRangeRejected"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerSetAttributeUnsupportedCapabilityRejected"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerSetAttributeOverwrites"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerClearEmptiesBuffer"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerInspectStableOrderNoPhantoms"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerValidateProgrammerAcceptsValidState"
        status: pass
      - kind: unit
        ref: "internal/programming/programmer_test.go#TestProgrammerValidateProgrammerRejectsHandTamperedState"
        status: pass
    human_judgment: false
  - id: D3
    description: "`golc programmer set/inspect/clear` routes an author's CLI edit through selection resolution and attribute validation, persisting the buffer on show.State through the existing atomic Save/Load round trip; a hand-tampered invalid buffer fails Load/Save as GOLC_SHOW_STATE_INVALID."
    requirement: "PROG-03"
    verification:
      - kind: integration
        ref: "internal/command/programming_test.go#TestProgrammerRoutes"
        status: pass
      - kind: integration
        ref: "internal/command/programming_test.go#TestProgrammerSetUnsupportedCapability"
        status: pass
      - kind: integration
        ref: "internal/command/programming_test.go#TestProgrammerSetDanglingInstance"
        status: pass
      - kind: integration
        ref: "internal/command/programming_test.go#TestProgrammerShowStateRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/show/... (go test ./internal/show/...)"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-21
status: complete
---

# Phase 3 Plan 01: Fixture Selection and Programmer State Summary

**PROG-01 selection resolver over pool/group/deployment-instance/direct-fixture selectors, PROG-02/03 normalized-attribute programmer buffer, and a `golc programmer set/inspect/clear` CLI scope persisted on show.State.**

## Performance

- **Duration:** 8 min (git commit-timestamp span; actual session wall time was longer including context-gathering and analog-file reading)
- **Started:** 2026-07-21T20:23:01-07:00 (first RED commit)
- **Completed:** 2026-07-21T20:31:10-07:00 (final GREEN commit)
- **Tasks:** 3
- **Files modified:** 7 (6 created, 1 modified) + 1 deferred-items log

## Accomplishments
- `programming.Resolve` deterministically resolves any mix of pool/group/deployment-instance/direct-fixture selectors into a deduped, stably-ordered `ResolvedSet`, rejecting any dangling reference with `GOLC_SELECTION_DANGLING_REFERENCE` rather than silently shrinking the result
- `programming.ProgrammerState` records semantic intensity/color/position/beam (and the other six `fixture.CapabilityType` values) as normalized `[0,1]` attributes, never raw DMX, with overwrite-in-place semantics and a stable, phantom-free `Touched()` inspection surface
- `show.State` gains a `Programmer` scratch-buffer field, validated through the existing single `validate()` entry point; the `programmer` CLI scope (`set`/`inspect`/`clear`) resolves selections, edits attributes, and persists through the existing atomic `show.Save`/`show.Load` round trip

## Task Commits

Each task was committed atomically (TDD RED/GREEN pairs):

1. **Task 1: Selection resolution (PROG-01)** - `03609ef` (test, RED) → `6c62b92` (feat, GREEN)
2. **Task 2: Programmer state and semantic attribute editing (PROG-02, PROG-03)** - `314c454` (test, RED) → `9f8a0a0` (feat, GREEN)
3. **Task 3: Persist programmer buffer on show.State and expose the `programmer` command scope (PROG-02, PROG-03)** - `480c4f8` (test, RED) → `020931b` (feat, GREEN)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/programming/selection.go` - `Selection`/`FixtureRef`/`ResolvedInstance`/`ResolvedSet` types and `Resolve` (PROG-01)
- `internal/programming/selection_test.go` - pool/group/instance/direct-fixture resolution, overlap dedupe, empty-selection/empty-pool, stable ordering, and dangling-reference coverage
- `internal/programming/programmer.go` - `ProgrammerState`/`TouchedAttribute`/`AttributeSource`, `NewProgrammerState`/`SetAttribute`/`Clear`/`Touched`/`ValidateProgrammer` (PROG-02/PROG-03)
- `internal/programming/programmer_test.go` - in-range set, out-of-range rejection, unsupported-capability rejection, overwrite, Clear, stable-order Touched coverage
- `internal/show/state.go` - added `Programmer *programming.ProgrammerState` field (`json:"programmer,omitempty"`), extended `validate()` to call `programming.ValidateProgrammer` when non-nil
- `internal/command/programming.go` - `programmer` scope with `set`/`inspect`/`clear` routes: arg parsing, selection resolution, attribute mutation, save, and touched-attribute inspection output
- `internal/command/programming_test.go` - end-to-end route coverage (set/inspect/clear round trip, out-of-range/unsupported-capability/dangling-instance rejection, direct show.State round trip)
- `.planning/phases/03-deterministic-show-programming-and-playback/deferred-items.md` - logged an unrelated pre-existing `go test ./...` failure (see Issues Encountered)

## Decisions Made
- PROG-03's "record scope" is implemented as the `TouchedAttribute`'s own `InstanceID` (the one instance that attribute will be recorded into) rather than a separate stored `ResolvedSet` field on `ProgrammerState`. The plan's own `flagged_assumptions` marked PROG-03 as unclassified/ambiguous, and its exact function list (`NewProgrammerState`, `SetAttribute`, `Clear`, `Touched`, `ValidateProgrammer`) has no scope-setter, so deriving "record scope" from the attribute's own instance ID is the simplest reading consistent with the declared API surface. `programmer inspect` prints this instance ID alongside capability/value/source per touched line.
- `programmer set`'s `--instance`/`--pool`/`--group`/`--fixture` flags build a full `programming.Selection`, resolved via `programming.Resolve` before any `SetAttribute` call — so a dangling selector is rejected up front, never partway through a batch of attribute edits.
- Malformed `--fixture <pool_id>|<pool_member_id>` shape errors use `GOLC_SELECTION_USAGE` (a Selection-input parsing error); every other CLI usage error (missing `--show`, bad `--attr` shape, invalid UUID on `--instance`/`--pool`/`--group`) uses `GOLC_PROGRAMMER_USAGE`. Both codes were declared in the plan's `artifacts_produced` list and needed a defensible split — this mirrors how `pool.go` scopes its own usage diagnostics by parsing concern.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected selection_test.go's own expected instance sets during RED->GREEN**
- **Found during:** Task 1 (Selection resolution)
- **Issue:** The test fixture's second deployment intentionally patches a second instance from poolA's member `m1` (to exercise the direct-fixture-ref-across-deployments case), but several of my own first-draft test expectations (`TestSelectionResolvesPool`, `TestSelectionResolvesGroup`, `TestSelectionOverlapDedupes`, `TestSelectionStableOrdering`) had not accounted for that instance also matching a pool-ID selector — they expected a smaller instance set than the correct, spec-consistent behavior ("a Selection referencing one pool resolves to every deployment Instance whose PoolID matches, across the deployments given") actually produces.
- **Fix:** Corrected each test's expected instance-ID list to match the fixture's real, spec-correct resolution (verified by re-deriving the expected set by hand from the fixture's declared instances); the overlap test was also redesigned to exercise `InstanceIDs` + `FixtureRefs` overlap instead of the original `PoolIDs` + `GroupIDs` + `FixtureRefs` combination, since the original combination didn't actually create a meaningful overlap scenario once the pool-selector semantics were correctly understood.
- **Files modified:** `internal/programming/selection_test.go` (test-only; `selection.go`'s implementation was correct on first GREEN attempt and was not changed)
- **Verification:** `go test ./internal/programming/... -run TestSelection` — 14/14 pass
- **Committed in:** `6c62b92` (Task 1 GREEN commit, alongside the implementation)

---

**Total deviations:** 1 auto-fixed (test-expectation bug, discovered and fixed during the RED→GREEN transition, not a code-behavior deviation from the plan)
**Impact on plan:** No scope creep; the resolver's actual behavior matches the plan's PROG-01 acceptance criteria exactly. The correction was to my own hand-written test expectations, not to the implementation or to the plan's specified behavior.

## Issues Encountered
- `go test ./...` (full repo suite) reports one unrelated pre-existing failure: `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` in `internal/trace/catalog`, a `GOLC_MIGRATE_DRIFT` mismatch against `.planning/linear-map.json`. Confirmed out of this plan's scope: `git status` shows zero uncommitted `.planning/` changes from this plan's work, and `internal/trace/catalog` was last touched in Phase 1 (01-08/01-09/01-21), with no file overlap against anything `03-01` modifies. Logged to `deferred-items.md` per the scope-boundary rule rather than fixed. All three of this plan's own target test scopes (`internal/programming/...`, `internal/command/... -run TestProgrammer`, `internal/show/...`) pass cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `programming.Selection`/`Resolve` and `programming.ProgrammerState` are ready for 03-02 (themes/presets) to build reusable objects from programmer state, and for 03-05/03-06 (scenes/layers) to reuse the same selection-resolution mechanics per-layer (CONTEXT D-03).
- `show.State.Programmer` establishes the exact field/validate()-extension pattern later plans (Themes/Presets/Chases/MotionPresets/Scenes/BlendPresets) should copy verbatim.
- Blocker/concern: the pre-existing `internal/trace/catalog` linear-map drift failure (see Issues Encountered) should be triaged by a future plan/phase — it will continue to fail `go test ./...` (though not the phase's own scoped test commands) until addressed.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-21*

## Self-Check: PASSED

- Verified all 9 created/modified files exist on disk (internal/programming/{selection,programmer}.go + tests, internal/command/programming.go + test, internal/show/state.go, deferred-items.md, this SUMMARY.md).
- Verified all 7 commit hashes exist in `git log --oneline --all` (03609ef, 6c62b92, 314c454, 9f8a0a0, 480c4f8, 020931b, cb952ec).
