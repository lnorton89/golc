---
phase: 06-wails-authoring-and-operator-surface
plan: 09
subsystem: playback
tags: [midi, gap-closure, wails, command-dispatch, sqlite, go]

# Dependency graph
requires:
  - phase: 06-08
    provides: "internal/wails.MidiService's dispatchToActiveSurface/takeoverStateFor/emitMidiFeedback scaffold (feedback-only arbitration) and svc_midi_test.go's testdrv-backed fixture conventions this plan extends"
  - phase: 06-06
    provides: "PlaybackService.execute/SwitchScene/SetLayerEnabled's exact in-process command-registry dispatch shape and WR-01/WR-03 Ref-preservation discipline, mirrored verbatim by dispatchSceneSwitch/dispatchLayerToggle"
  - phase: 06-05
    provides: "SafetyService.toggle/dialFn's daemon dial+forward shape and hotkey.go's routeBlackout/routeStopAll/routeRevokeAutomation constants, mirrored verbatim by dispatchSafetyTrigger"
  - phase: 06-02
    provides: "internal/command/artnet.go's runArtnetMasterSet and the daemon's handleMasterSet route/arg grammar (--grand|--group+--level, --source manual) dispatchMasterSet dials directly"
provides:
  - "internal/wails.MidiService.dispatchToActiveSurface now executes the ControlRef-implied command (scene switch, layer toggle, master level, safety trigger) in addition to feedback -- closes VERIFICATION.md Gap B[1]"
  - "dispatchMapping/dispatchSceneSwitch/dispatchLayerToggle/dispatchSafetyTrigger/dispatchMasterSet/safetyRouteFor: the per-Target-kind dispatch implementation, plus dial/dialFn and execute/executeWithRetry helpers on MidiService"
  - "showLoadWithRetry/executeWithRetry/isTransientShowLockError: a bounded retry around the show store's transient SQLite 'database is locked' contention, so a background dispatch loop's own show.Load/mutation never silently drops a physical control press"
affects: ["06-secure-phase (threat T-06-24/T-06-25/T-06-26 verification against this plan's threat register)", "any future plan that adds a second concurrent show-store writer alongside MidiService's dispatch loop"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A background dispatch loop that reacts to live, external, sub-second-cadence input (MIDI messages) and touches the show store treats a transient SQLite lock as retryable (showLoadWithRetry/executeWithRetry), not as 'nothing to dispatch' -- distinguishing recoverable I/O contention from a genuine domain rejection (unknown scene, invalid layer kind) which still surfaces/no-ops immediately, unretried"
    - "Discrete vs. continuous MIDI dispatch is decided by a single edge boolean computed once in dispatchToActiveSurface (Note: evt.Value>0; CC: the not-armed-to-armed TakeoverState transition) and threaded into dispatchMapping, rather than re-deriving arming/edge state per Target kind"

key-files:
  created: []
  modified:
    - internal/wails/svc_midi.go
    - internal/wails/svc_midi_test.go

key-decisions:
  - "dispatchMapping computes one shared (armed, edge, controlValue) triple in dispatchToActiveSurface before branching on Target.Kind, so scene/layer/safety's 'fire once per activation edge' rule and master's 'continuous while armed' rule share one crossing/edge computation instead of two independent state machines that could disagree."
  - "flagged_assumption (per plan Task 2 instruction): a Note-kind master mapping's level is a hard 1.0 on Note-on / 0.0 on Note-off (not the raw velocity), dispatched on every message since a Note mapping is always 'armed' (D-12) -- a reasonable default for the routes runArtnetMasterSet already supports, not new command semantics. A CC-kind master mapping forwards the tracked takeover value continuously while armed, per the plan's own D-11 characterization as 'the one deliberately-repeating dispatch.'"
  - "[Rule 2 - missing critical] Added a bounded 5-attempt/5ms-backoff retry (showLoadWithRetry for the pre-read, executeWithRetry for the mutating registry call) around the show store's transient 'database is locked' (SQLITE_BUSY) diagnostic. internal/show/schema.go sets no busy_timeout and performs no retry of its own (it documents a single-writer-per-process CLI model), but MidiService's dispatch loop is a persistent background goroutine that can race another service's independently-triggered show.Load (e.g. PlaybackService.GetState()/SurfaceService.ListMappings() polled by the frontend) at the exact moment of a physical MIDI press. Without this retry a transient lock silently drops the operator's button press instead of switching the scene/toggling the layer -- confirmed via a reproducible test failure (a discrete 'database is locked' error inside dispatchSceneSwitch's own pre-read, observed on Windows) before the fix, and the retry cleared it. This is svc_midi.go-local (files_modified scope); internal/show/schema.go itself is untouched."
  - "Master/safety dispatch dials the daemon directly via dialFn() (mirrors SafetyService.toggle exactly), bypassing internal/command/artnet.go's own runArtnetMasterSet CLI-arg-parsing layer, since dispatchMasterSet's level/GroupID values are already typed/validated (sourced from the show document and the takeover state machine), never raw user text -- confirmed against the daemon's own handleMasterSet route/arg grammar in internal/artnet/daemon.go, not assumed from the CLI wrapper alone."
  - "Two pre-existing test-fixture bugs were found and fixed while stabilizing the new suite, both caught before any dispatch-code change was needed: (1) TestMidiServiceDispatchDeletedTargetIsSilentNoOp originally assigned the doomed scene to the surface's SceneRefs, which show.Save itself rejects as a dangling reference (GOLC_OPERATORSURFACE_DANGLING_REFERENCE) before the MIDI-dispatch scenario under test ever ran -- fixed by never assigning that scene (operatorsurface.AddMidiMapping doesn't require surface-assignment membership, only StartLearn's command.Authorize does). (2) TestMidiServiceDispatchMasterCcContinuesWhileArmed originally sent a crossing-value CC as a control's very first message, which midi.TakeoverState can never arm on (LastPhysical seeds to NaN) -- fixed by seeding a below-threshold value first for each independently-tracked mapping."

patterns-established:
  - "A service method that touches the show store from a background goroutine (not a one-shot CLI-style call) wraps show.Load and its mutating registry call with a small retry keyed on the store's own 'database is locked' diagnostic string, rather than propagating a busy_timeout/retry change into internal/show itself (out of this plan's file scope) or discarding the error silently."

requirements-completed: [PLAY-04, PLAY-05]
# This plan is the gap-closure amendment to 06-03/06-08's learn+takeover
# work: it closes VERIFICATION.md Gap B[1] (the FAILED "MIDI dispatch"
# truth) by making dispatchToActiveSurface actually execute the
# ControlRef-implied command, not only compute feedback. PLAY-04's
# idempotency/concurrency edges and PLAY-05's soft-takeover arbitration
# were already proven by 06-03/06-08 and are not re-resolved here.

coverage:
  - id: D1
    description: "A MIDI Note mapped to a scene control switches the show's active scene when pressed, not only the on-screen marker; a following Note-off does not re-switch or error"
    requirement: "PLAY-04"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchSceneNoteSwitchesActiveScene"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchSceneEdgeFiresPerPressNotPerMessage"
        status: pass
    human_judgment: false
  - id: D2
    description: "A MIDI Note mapped to a layer control flips the layer's Enabled flag when pressed, preserving the layer's existing Ref (WR-01/WR-03 discipline)"
    requirement: "PLAY-04"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchLayerNoteTogglesEnabledPreservingRef"
        status: pass
    human_judgment: false
  - id: D3
    description: "A MIDI CC mapped to a grand/group master forwards 'artnet master set' with the crossed level only once the fader crosses to armed (cross-to-catch), continuing to forward on every subsequent armed message"
    requirement: "PLAY-05"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchMasterCcForwardsOnlyAfterCrossing"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchMasterCcContinuesWhileArmed"
        status: pass
    human_judgment: false
  - id: D4
    description: "A MIDI Note mapped to a safety control forwards the matching 'artnet safety ...' route with --source manual when pressed, and does not re-forward on release"
    requirement: "PLAY-04"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchSafetyNoteForwardsDaemonRoute"
        status: pass
    human_judgment: false
  - id: D5
    description: "A message matching no mapping on the active surface dispatches nothing and changes no state; a mapping whose target scene was deleted dispatches nothing and does not panic, and the dispatch loop keeps working afterward"
    verification:
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchUnmappedEventDoesNothing"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_midi_test.go#TestMidiServiceDispatchDeletedTargetIsSilentNoOp"
        status: pass
    human_judgment: false

# Metrics
duration: 55min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 9: MIDI Dispatch Gap Closure Summary

**dispatchToActiveSurface now executes scene switches, layer toggles, master-level forwards, and safety triggers implied by a matched MIDI mapping -- not just feedback -- closing VERIFICATION.md Gap B[1], plus a bounded retry hardening the show store's own transient SQLite lock contention against MidiService's new background writer.**

## Performance

- **Duration:** 55 min
- **Tasks:** 3 (RED tests, GREEN implementation, edge coverage -- edge tests were authored alongside the RED commit for full-gap coverage; see Deviations)
- **Files modified:** 2

## Accomplishments
- `dispatchToActiveSurface` computes one shared `(armed, edge, controlValue)` triple per event and dispatches the ControlRef-implied command: scene switch and layer toggle run through the in-process command registry (mirrors `PlaybackService.execute`/`SetLayerEnabled`'s exact WR-01/WR-03 Ref-preserving pattern); master level and safety trigger dial the daemon directly (mirrors `SafetyService.toggle`'s exact route/arg convention)
- Discrete actions (scene switch, layer toggle, safety trigger) fire exactly once per activation edge (a Note press, or a CC's first arming crossing) and never re-fire on a held/repeated armed message; a master level forwards on every armed message (continuous for CC, immediate level-1.0/0.0 for Note press/release)
- A deleted mapping target (scene/layer since removed from the show) resolves to a silent no-op, never a panic, and the dispatch loop continues processing subsequent events normally
- Hardened the dispatch path against the show store's transient "database is locked" contention with a bounded retry (`showLoadWithRetry`/`executeWithRetry`), since `MidiService`'s dispatch loop is now a persistent background goroutine writing to the same show store other services read/write on demand

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing dispatch tests** - `e9ebfca` (test) -- all 8 dispatch tests (the 5 required by Task 1 plus Task 3's 3 edge tests, added together for full-gap RED coverage) confirmed failing against the untouched feedback-only `dispatchToActiveSurface`
2. **Task 2: Implement command dispatch** - `6e400a8` (feat) -- all 8 tests pass; includes the Rule 2 retry hardening and two test-fixture bug fixes found while stabilizing the suite

Task 3 ("Edge coverage") required no additional code: its three named tests
(`TestMidiServiceDispatchSceneEdgeFiresPerPressNotPerMessage`,
`TestMidiServiceDispatchMasterCcContinuesWhileArmed`,
`TestMidiServiceDispatchDeletedTargetIsSilentNoOp`) were already written in
Task 1's RED commit and already pass as of Task 2's GREEN commit -- see
Deviations.

## TDD Gate Compliance

- RED gate: `e9ebfca` (`test(06-09): ...`)
- GREEN gate: `6e400a8` (`feat(06-09): ...`)
- No REFACTOR-gate commit was needed; the fixture-bug fixes and the retry
  hardening found during stabilization were folded into the GREEN commit
  rather than requiring a separate cleanup pass.

## Files Created/Modified
- `internal/wails/svc_midi.go` - `dispatchToActiveSurface` now dispatches (not just feeds back); adds `dispatchMapping`/`dispatchSceneSwitch`/`dispatchLayerToggle`/`dispatchSafetyTrigger`/`dispatchMasterSet`/`safetyRouteFor`, the `dial`/`dialFn` daemon-dial plumbing, `execute`/`executeWithRetry`, and `showLoadWithRetry`/`isTransientShowLockError`
- `internal/wails/svc_midi_test.go` - Adds the 8 dispatch tests (5 required + 3 edge) and their fixture/assertion helpers (`dispatchCapture`, `waitForDispatchCount`, `waitForSceneActive`, `waitForLayerEnabled`, `assertMasterSetForward`, `loadShowWithRetry`)

## Decisions Made
- See `key-decisions` in frontmatter for the full list, including the flagged assumption on Note-kind master level semantics and the Rule 2 retry hardening rationale.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added a bounded retry around the show store's transient "database is locked" contention**
- **Found during:** Task 2 (stabilizing the new dispatch tests)
- **Issue:** `MidiService`'s dispatch loop is a persistent background goroutine; once it started calling `show.Load`/the mutating command registry on every relevant MIDI message, it could race another goroutine's concurrent show-store access (in tests, the test's own polling `show.Load`; in production, any other on-demand service call like `PlaybackService.GetState()`). `internal/show/schema.go` sets no `busy_timeout` and performs no retry (it documents a single-writer-per-process CLI model). Without a retry, a transient SQLite lock silently drops the physical button press -- reproduced directly (a discrete "database is locked" error observed inside `dispatchSceneSwitch`'s own pre-read on Windows).
- **Fix:** Added `showLoadWithRetry` (wraps the pre-read `show.Load`) and `executeWithRetry` (wraps the mutating registry call), both bounded to 5 attempts at a 5ms backoff, retrying only on the store's own "database is locked" diagnostic -- every other error (corrupt store, wrong format) still surfaces/no-ops immediately, unretried.
- **Files modified:** internal/wails/svc_midi.go
- **Verification:** `go test ./internal/wails/... -run TestMidiServiceDispatch -count=10` is green (previously flaky, ~1-in-5 failure rate without the retry)
- **Committed in:** 6e400a8 (Task 2 commit)

**2. [Rule 1 - Bug] Fixed two test-fixture bugs unrelated to the dispatch implementation, found while stabilizing the new suite**
- **Found during:** Task 2 (making all 8 new tests green)
- **Issue:** (a) `TestMidiServiceDispatchDeletedTargetIsSilentNoOp` originally assigned the doomed "Ghost" scene to the test surface's `SceneRefs` before deleting the scene, which `show.Save`'s own `GOLC_OPERATORSURFACE_DANGLING_REFERENCE` validation rejects outright -- the test never reached the MIDI-dispatch scenario it was meant to prove. (b) `TestMidiServiceDispatchMasterCcContinuesWhileArmed` sent a crossing-value CC as a mapping's very first message; `midi.TakeoverState` seeds `LastPhysical` to `NaN` specifically so a control's first message can never satisfy the crossing check (by design, per `internal/midi/takeover.go`'s own doc comment) -- the test asserted an arming transition that could never occur as written.
- **Fix:** (a) Stopped assigning "Ghost" to the surface's `SceneRefs` (`operatorsurface.AddMidiMapping` doesn't require surface-assignment membership; only `StartLearn`'s `command.Authorize` does, and this test bypasses `StartLearn` deliberately). (b) Send a below-threshold CC value first for each independently-tracked mapping before its crossing message.
- **Files modified:** internal/wails/svc_midi_test.go
- **Verification:** Both tests pass consistently across repeated runs (`-count=10`)
- **Committed in:** 6e400a8 (Task 2 commit)

**3. [Process deviation, not a Rule 1-3 fix] Task 3's edge tests were authored in Task 1's RED commit rather than a separate Task 3 commit**
- **Found during:** Task 1 (writing the RED test suite)
- **Rationale:** All 8 dispatch tests (the plan's Task 1 set of 5 plus Task 3's set of 3) exercise the same untouched `dispatchToActiveSurface`, so writing them together maximized RED-phase gap coverage before any implementation existed, and Task 2's implementation was designed against the full 8-test contract from the start rather than being extended mid-stream for Task 3.
- **Impact:** Task 3 required no additional code or test changes -- its three named tests already existed and already passed as of the Task 2 (GREEN) commit. No separate Task 3 commit was created since there was nothing new to commit; `go test ./internal/wails/... -run TestMidiServiceDispatch` and `go build ./internal/... ./cmd/golc-project/...` are both green, satisfying Task 3's stated verification.
- **Committed in:** e9ebfca (tests), 6e400a8 (implementation) -- no additional commit

---

**Total deviations:** 3 (1 Rule 2 missing-critical fix, 1 Rule 1 bug fix bundle across two test-fixture issues, 1 process deviation in commit sequencing)
**Impact on plan:** The Rule 2 retry is a genuine correctness hardening this plan's new background-writer pattern requires; it does not expand scope beyond `internal/wails/svc_midi.go`. The Rule 1 fixes are test-only. The process deviation changes commit sequencing, not deliverable scope -- all of Task 3's named tests and acceptance criteria are satisfied.

## Issues Encountered
- Reproducing the SQLite lock flakiness required isolating it from an initial false lead: adding `fmt.Println` debug statements around the dispatch calls made the failures disappear on some runs (classic scheduling-perturbation symptom of a genuine race), which pointed at concurrent show-store access rather than a logic bug in the new dispatch code itself. Confirmed by observing the literal `database is locked (261)` / `database is locked (5) (SQLITE_BUSY)` diagnostic surface directly inside `dispatchSceneSwitch`'s own pre-read and (separately) inside a test's own post-wait `show.Load` call.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- VERIFICATION.md Gap B[1] ("MIDI dispatch" truth) is closed: a mapped MIDI Note/CC now operates the show (scene switch, layer toggle, master level, safety trigger), not only the on-screen feedback marker.
- No regression in existing learn/takeover/feedback coverage (`go test ./internal/wails/...` full package suite is green).
- `NewMidiService`'s signature and `cmd/golc-desktop/main.go` remain unchanged, as required -- this plan is isolated to `internal/wails/svc_midi.go` and its test.
- Sibling gap-closure plans 06-10/06-11/06-12 (PLAY-10/11/12 on-screen UI) are independent of this plan's file scope (`internal/wails/svc_midi.go`, `internal/wails/svc_midi_test.go`) and were executed in a parallel worktree.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED
