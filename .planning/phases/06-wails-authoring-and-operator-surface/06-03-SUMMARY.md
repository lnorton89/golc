---
phase: 06-wails-authoring-and-operator-surface
plan: 03
subsystem: playback
tags: [midi, state-machine, soft-takeover, learn-mode, go]

# Dependency graph
requires:
  - phase: none
    provides: "greenfield package; no prior phase precedent for MIDI logic"
provides:
  - "internal/midi.TakeoverState: cross-to-catch soft-takeover state machine (D-09..D-12)"
  - "internal/midi.ControlKey/MessageKind/ProposeMapping/CaptureCandidate: bounded learn capture + per-surface conflict rejection (D-05/D-06/D-07)"
affects: ["06-08 (live gomidi driver wiring)", "operatorsurface command/host layer mapping ControlKey<->MidiMapping"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure, dependency-free Go package (no gomidi import) isolating the phase's hardest correctness logic (crossing vs. proximity) into fast unit tests ahead of live device wiring"
    - "NaN-seeded LastPhysical sentinel to make the first physical reading in a crossing check never spuriously arm, exploiting IEEE 754 NaN comparison semantics instead of an explicit has-received-first-value flag"

key-files:
  created:
    - internal/midi/takeover.go
    - internal/midi/takeover_test.go
    - internal/midi/learn.go
    - internal/midi/learn_test.go
  modified: []

key-decisions:
  - "NewTakeoverState seeds LastPhysical to math.NaN() rather than AppValue or zero, because seeding it to AppValue makes the crossing check's <=/>= equality trivially true on the very first Update call regardless of physical value (spurious arm-on-message-one); NaN comparisons are always false, correctly deferring the crossing decision until a real second reading exists"
  - "SetAppValue only re-seeds AppValue while the control is not armed; an already-armed control's AppValue continues to be driven by Update tracking live physical position, so an external app-value change never fights the armed invariant"
  - "learn_test.go test function names were renamed to a TestLearn* prefix (not TestProposeMapping*/TestCaptureCandidate*) so the plan's specified \`go test ./internal/midi/... -run TestLearn\` verification command actually selects them"

patterns-established:
  - "TakeoverState.Update implements RESEARCH.md Pattern 4 verbatim (crossedUp OR crossedDown on <=/>=, never abs(physical-appValue) < threshold) -- future MIDI-adjacent state machines in this codebase should follow the same verbatim-pattern-plus-constructor-hardening shape rather than reinventing the crossing check"
  - "ProposeMapping/ControlKey equality (a plain comparable struct with Kind as a field) is the per-surface conflict-check shape: no separate keyOf() helper needed since ControlKey's fields are already the full identity tuple"

requirements-completed: [PLAY-04, PLAY-05]

coverage:
  - id: D1
    description: "Cross-to-catch soft takeover: a continuous CC/fader control arms only on a true directional crossing (or exact landing) of the app's current value, never on proximity"
    requirement: PLAY-05
    verification:
      - kind: unit
        ref: "internal/midi/takeover_test.go#TestTakeoverRisingCross"
        status: pass
      - kind: unit
        ref: "internal/midi/takeover_test.go#TestTakeoverFallingCross"
        status: pass
      - kind: unit
        ref: "internal/midi/takeover_test.go#TestTakeoverNeverCrosses"
        status: pass
      - kind: unit
        ref: "internal/midi/takeover_test.go#TestTakeoverExactLanding"
        status: pass
      - kind: unit
        ref: "internal/midi/takeover_test.go#TestTakeoverFirstMessageNeverArmsSpuriously"
        status: pass
    human_judgment: false
  - id: D2
    description: "Bounded per-control MIDI learn capture: the next Note-on/CC message is the mapping candidate, or the window closes with GOLC_MIDI_LEARN_TIMEOUT"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnCaptureCandidateReturnsFirstReceived"
        status: pass
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnCaptureCandidateTimesOut"
        status: pass
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnCaptureCandidateDoesNotHangWithoutEitherChannel"
        status: pass
    human_judgment: false
  - id: D3
    description: "Learn-conflict rejection: a candidate colliding with an existing per-surface mapping is rejected outright with GOLC_MIDI_MAPPING_CONFLICT and the existing mapping is never mutated"
    requirement: PLAY-04
    verification:
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnProposeMappingRejectsConflictAndLeavesExistingUntouched"
        status: pass
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnProposeMappingScopedPerSurface"
        status: pass
      - kind: unit
        ref: "internal/midi/learn_test.go#TestLearnProposeMappingKindIsPartOfIdentity"
        status: pass
    human_judgment: false

# Metrics
duration: 4min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 03: MIDI Learn and Soft-Takeover Core Logic Summary

**Pure, dependency-free `internal/midi` package: cross-to-catch soft-takeover state machine (NaN-bootstrapped crossing check) and bounded learn capture with outright per-surface conflict rejection.**

## Performance

- **Duration:** 4min
- **Started:** 2026-07-23T02:47:17-07:00
- **Completed:** 2026-07-23T02:50:49-07:00
- **Tasks:** 2 completed
- **Files modified:** 4 (all new)

## Accomplishments
- `TakeoverState.Update` implements the direction-aware cross-to-catch algorithm verbatim from RESEARCH.md Pattern 4: a continuous CC/fader control arms only when the physical value reaches or crosses `AppValue` in the direction of travel (or lands exactly on it) -- no proximity/threshold constant exists anywhere in the file (D-11, RESEARCH.md Pitfall 2 explicitly avoided)
- `NewTakeoverState` solves the un-stated bootstrap problem (the very first physical reading a fresh state receives must never spuriously "cross" from an unknown prior position) by seeding `LastPhysical` to `math.NaN()`, relying on IEEE 754's guarantee that every `NaN <= x` / `NaN >= x` comparison is false
- `SetAppValue` re-seeds the ghost/target marker (D-10) from an external app-value change while unarmed, without disturbing an already-armed control's live tracking
- `ProposeMapping`/`ControlKey`/`MessageKind` implement the D-05/D-06/D-07 bounded learn-conflict rule: a colliding `(Channel, Kind, Number)` candidate is rejected outright with `GOLC_MIDI_MAPPING_CONFLICT` and the existing slice is never mutated; the check is scoped to whichever surface's slice the caller passes in, and `Kind` (Note vs. ControlChange) is part of identity so the same channel/number never collides across kinds
- `CaptureCandidate` implements the D-05 bounded capture window as a two-case `select` over a caller-supplied message channel and timeout channel, returning `GOLC_MIDI_LEARN_TIMEOUT` if the timeout fires first -- no unbounded wait path exists
- Zero new go.mod/go.sum entries; `internal/midi` imports only `fmt` and `math` from the standard library

## Task Commits

Each task followed strict TDD (test -> feat), both tasks had `tdd="true"`:

1. **Task 1: takeover.go -- cross-to-catch soft-takeover state machine**
   - `20668b8` (test) - failing tests for rising-cross, falling-cross, never-cross, exact-landing, first-message bootstrap, SetAppValue re-seed
   - `e8765d7` (feat) - `TakeoverState`, `NewTakeoverState`, `SetAppValue`, `Update`
2. **Task 2: learn.go -- bounded capture candidate + conflict rejection**
   - `cb9dc30` (test) - failing tests for accept, conflict-reject, per-surface scoping, kind-as-identity, capture accept/timeout
   - `4abc849` (feat) - `ControlKey`, `MessageKind`, `ProposeMapping`, `CaptureCandidate`

No REFACTOR commits were needed -- both GREEN implementations passed `go vet` and `gofmt -l` cleanly on the first pass.

## Files Created/Modified
- `internal/midi/takeover.go` - `TakeoverState` struct + `NewTakeoverState`/`SetAppValue`/`Update`; the cross-to-catch soft-takeover state machine
- `internal/midi/takeover_test.go` - rising-cross, falling-cross, never-cross, exact-landing, first-message-bootstrap, and SetAppValue tests
- `internal/midi/learn.go` - `ControlKey`, `MessageKind` (`Note`/`ControlChange`), `ProposeMapping`, `CaptureCandidate`
- `internal/midi/learn_test.go` - accept, conflict-reject (existing untouched), per-surface scoping, kind-identity, and capture accept/timeout tests

## Decisions Made
- Seed `TakeoverState.LastPhysical` to `math.NaN()` in `NewTakeoverState` rather than `AppValue` or the Go zero value: seeding to `AppValue` makes the crossing check's `<=`/`>=` equality trivially satisfied on the very first `Update` call (both `crossedUp` and `crossedDown` conditions hold at `LastPhysical == AppValue`), spuriously arming on message one no matter what the first physical reading is; seeding to `0.0` only avoids this by accident for `AppValue` values on one particular side of zero. NaN's IEEE 754 comparison semantics (`NaN <= x` and `NaN >= x` are always false) cleanly defer the crossing decision until a real second reading exists, with no extra `hasReceived` field needed and the exported struct kept to exactly the three fields the plan specified.
- `SetAppValue` is a no-op while `Armed`, so an external app-value change never contends with an already-armed control's live-tracking invariant (`AppValue == LastPhysical` while armed); it only affects the not-armed ghost/target case the plan describes.
- Renamed `learn_test.go`'s test functions from `TestProposeMapping*`/`TestCaptureCandidate*` to a `TestLearn*` prefix after discovering the plan's specified `go test ./internal/midi/... -run TestLearn` verification command wouldn't otherwise select any of them (Rule 1 -- bug fix, applied before the GREEN commit; the RED commit still has the original names since renaming was itself part of getting to GREEN cleanly).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed learn_test.go test names to match the plan's specified verify command**
- **Found during:** Task 2 (GREEN step, running `go test ./internal/midi/... -run TestLearn`)
- **Issue:** The initially written test functions were named `TestProposeMapping*` and `TestCaptureCandidate*`, none of which contain the substring `TestLearn`, so the plan's specified verification command (`-run TestLearn`) matched zero tests ("no tests to run") even though the tests themselves were correct and would pass under a broader `-run` pattern.
- **Fix:** Renamed all eight test functions in `learn_test.go` to a `TestLearn*` prefix (e.g. `TestLearnProposeMappingAcceptsNonCollidingCandidate`), preserving all test bodies/assertions unchanged.
- **Files modified:** internal/midi/learn_test.go
- **Verification:** `go test ./internal/midi/... -run TestLearn -v` now selects and passes all eight tests.
- **Committed in:** `4abc849` (Task 2 GREEN commit, alongside learn.go)

---

**Total deviations:** 1 auto-fixed (1 bug fix, test-naming only, no behavior change)
**Impact on plan:** Cosmetic/naming fix required for the plan's own verification command to work as specified. No scope creep, no production code affected.

## Issues Encountered
None beyond the test-naming deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/midi.TakeoverState` and `internal/midi.ControlKey`/`ProposeMapping`/`CaptureCandidate` are ready for 06-08 to wire against a live `gomidi`/`midicatdrv` driver: 06-08 supplies the physical CC value stream to `Update`, the message channel and a wall-clock timer to `CaptureCandidate`, and maps `operatorsurface.MidiMapping` <-> `midi.ControlKey` at the command/host layer (per this plan's `<interfaces>` section, deliberately not imported here to keep `internal/midi` decoupled from persistence).
- No blockers. `go build ./...` and `go test -race ./internal/midi/...` are both green; no new go.mod/go.sum entries were introduced, so the CGo-free build constraint (RESEARCH.md Pitfall 3) is untouched by this plan.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*
