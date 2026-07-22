---
phase: 03-deterministic-show-programming-and-playback
plan: 06
subsystem: playback
tags: [go, musical-clock, bpm, tap-tempo, monotonic-time, cli]

# Dependency graph
requires:
  - phase: 03-04
    provides: "show.State.Tempo field and scene.Scene.PreserveMusicalPositionOnBPMChange (SCEN-08 flag)"
provides:
  - "internal/playback.Position: pure musical-position clock, a deterministic function of (bpm, barsPerLoop, loopStart, now)"
  - "internal/playback.TapTempo: ordered tap timestamps -> BPM, rejecting <2 taps and zero/negative intervals"
  - "internal/playback.RecomputeEpoch: preserve-position-or-restart epoch recompute for a BPM change (SCEN-08/D-11)"
  - "internal/playback.ValidateBPM / CrossedBarBoundary: reusable validation and boundary-detection helpers for the engine (03-07)"
  - "playback bpm set / playback bpm tap CLI routes persisting show.State.Tempo.BPM (SCEN-02/SCEN-03)"
affects: [03-07, deterministic-playback-engine]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure function of elapsed monotonic time (now.Sub(epoch)), never an accumulated tick counter -- the mechanism behind SCEN-09's determinism guarantee"
    - "Integer BarIndex comparison for bar-boundary detection, never a BeatFraction==0 equality check (03-RESEARCH.md Pitfall 1)"

key-files:
  created:
    - internal/playback/clock.go
    - internal/playback/clock_test.go
    - internal/command/playback.go
    - internal/command/playback_bpm_test.go
  modified: []

key-decisions:
  - "maxBPM ceiling set to 999 as a named constant (DoS/tampering guard, mirrors internal/scene's maxBarsPerLoop precedent) -- no requirement specified an exact number, so a generous but bounded sane ceiling was chosen"
  - "playback bpm tap accepts ordered --at <RFC3339Nano timestamp> flags (repeatable, arrival order preserved) rather than a raw interval-list form, keeping the CLI surface directly aligned with playback.TapTempo's []time.Time signature"
  - "RecomputeEpoch's barsPerLoop parameter is accepted per the plan's declared signature but not used in the epoch math itself: preserving the raw (unwrapped) bars-elapsed fraction already preserves both BarIndex and BeatFraction identically under Position's own wrap, so no separate barsPerLoop-aware computation was needed"

patterns-established:
  - "Pattern 1 (03-RESEARCH.md): Position derives BarIndex/BeatFraction from now.Sub(loopStart) monotonic subtraction -- copied verbatim in shape, this is the reference implementation 03-07's engine imports and calls every tick"
  - "Boundary detection: CrossedBarBoundary(lastBarIndex, currentBarIndex int) bool, exported specifically so 03-07's tick loop reuses the exact same integer-comparison helper rather than re-deriving the equality-check pitfall"

requirements-completed: [SCEN-01, SCEN-02, SCEN-03, SCEN-08]

coverage:
  - id: D1
    description: "Pure musical-position clock: Position(now, bpm, barsPerLoop, loopStart) advances BarIndex/BeatFraction from monotonic elapsed time, wraps via modulo barsPerLoop, is deterministic across repeated calls and concurrent goroutines, and attributes an exact-boundary sample to the new bar (floor semantics)"
    requirement: "SCEN-01"
    verification:
      - kind: unit
        ref: "internal/playback/clock_test.go#TestClockPositionAdvancesAndWraps"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestClockPositionDeterministicSameArgs"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestClockPositionDeterministicAcrossGoroutines"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestClockPositionFloorSemanticsAtBarBoundary"
        status: pass
    human_judgment: false
  - id: D2
    description: "Operator can set global BPM by entering a numeric value via 'playback bpm set <bpm> --show <path>', persisted on show.State.Tempo.BPM; re-setting the current value is an idempotent no-op; a non-numeric or <=0 value is rejected with GOLC_PLAYBACK_BPM_INVALID; a missing positional argument is rejected with GOLC_PLAYBACK_USAGE (exit 2)"
    requirement: "SCEN-02"
    verification:
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestBPMSetValidValue"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestBPMSetCurrentValueIsIdempotentNoOp"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestBPMSetRejectsNonNumericValue"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestBPMSetRejectsNonPositiveValue"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestBPMSetMissingArgumentUsageExitTwo"
        status: pass
    human_judgment: false
  - id: D3
    description: "Operator can set global BPM through tap tempo via 'playback bpm tap --at <ts>... --show <path>', converting ordered tap timestamps to BPM and persisting it; fewer than two taps is rejected with GOLC_PLAYBACK_TAP_INVALID and does not persist a change"
    requirement: "SCEN-03"
    verification:
      - kind: unit
        ref: "internal/playback/clock_test.go#TestTapTempoComputesPositiveBPM"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestTapTempoRejectsFewerThanTwoTaps"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestTapTempoRejectsZeroInterval"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestTapTempoRejectsOutOfOrderTaps"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestTapTempoRoutePersistsBPM"
        status: pass
      - kind: unit
        ref: "internal/command/playback_bpm_test.go#TestTapTempoRouteRejectsFewerThanTwoTaps"
        status: pass
    human_judgment: false
  - id: D4
    description: "A global BPM change either preserves the active loop's musical position (RecomputeEpoch preserve=true keeps BarIndex/BeatFraction identical across the BPM change) or restarts the loop at bar 0 (preserve=false), per the active scene's SCEN-08 setting"
    requirement: "SCEN-08"
    verification:
      - kind: unit
        ref: "internal/playback/clock_test.go#TestBPMChangeEpochPreservesPosition"
        status: pass
      - kind: unit
        ref: "internal/playback/clock_test.go#TestBPMChangeEpochRestartsAtBarZero"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-07-22
status: complete
---

# Phase 3 Plan 06: Musical Clock and BPM Controls Summary

**Pure musical-position clock (`playback.Position`) driven by monotonic elapsed time plus numeric/tap-tempo BPM entry persisted on `show.State.Tempo.BPM`**

## Performance

- **Duration:** ~35 min
- **Tasks:** 2 completed
- **Files modified:** 4 (all new)

## Accomplishments
- `internal/playback.Position(now, bpm, barsPerLoop, loopStart) MusicalPosition` is a pure function of its four arguments, deriving `BarIndex`/`BeatFraction` from `now.Sub(loopStart)` monotonic subtraction (never an accumulated tick counter) — proven deterministic across repeated calls and 100 concurrent goroutines, and floor-correct on an exact bar boundary.
- `playback.TapTempo(taps []time.Time) (float64, error)` converts ordered tap timestamps to BPM, rejecting fewer than two taps and any non-positive inter-tap interval with `GOLC_PLAYBACK_TAP_INVALID`.
- `playback.RecomputeEpoch(preserve, oldBPM, newBPM, barsPerLoop, loopStart, now) time.Time` preserves the exact current bar/beat position across a BPM change when `preserve=true`, or restarts at bar 0 (`now`) when `preserve=false` (SCEN-08/D-11).
- `playback.ValidateBPM` and `playback.CrossedBarBoundary` are exported reusable helpers: the former enforces a positive/finite/sane-ceiling BPM (`maxBPM = 999`), the latter detects bar transitions via integer `BarIndex` comparison (never a `BeatFraction == 0` equality) for the engine (03-07) to reuse verbatim.
- `playback bpm set <bpm> --show <path>` and `playback bpm tap --at <ts>... --show <path>` CLI routes load/mutate/save `show.State.Tempo.BPM` through the existing atomic Load/Save round trip, following `internal/command/pool.go`'s exact parse-args-then-Load-mutate-Save-Stdout shape.

## Task Commits

1. **Task 1: Pure musical-position clock, tap tempo, and preserve/restart epoch (SCEN-01, SCEN-03, SCEN-08)** - `3214380` (feat)
2. **Task 2: BPM entry and tap-tempo command routes (SCEN-02, SCEN-03)** - `4fc682f` (feat)

**Plan metadata:** commit pending final SUMMARY/state commit (see below)

## Files Created/Modified
- `internal/playback/clock.go` - `MusicalPosition`, `Position`, `TapTempo`, `RecomputeEpoch`, `ValidateBPM`, `CrossedBarBoundary`, `beatsPerBar`/`maxBPM` named constants
- `internal/playback/clock_test.go` - Table-driven + concurrency + boundary-precision tests for all of the above
- `internal/command/playback.go` - `playback` scope, `playback bpm set` / `playback bpm tap` routes
- `internal/command/playback_bpm_test.go` - CLI-route-level tests (valid set, idempotent no-op, invalid rejection, usage exit-2, tap persistence, tap rejection, Tempo round-trip)

## Decisions Made
- `maxBPM = 999` chosen as a named, documented sane ceiling (CONTEXT threat T-03-05 DoS/tampering guard) — no requirement specifies an exact number; mirrors `internal/scene`'s `maxBarsPerLoop` precedent of a generous-but-bounded constant.
- `playback bpm tap` accepts repeatable `--at <RFC3339Nano timestamp>` flags in given order (not re-sorted), matching `playback.TapTempo`'s `[]time.Time` arrival-order contract directly rather than inventing a separate interval-list CLI form.
- `RecomputeEpoch` keeps the plan-specified `barsPerLoop` parameter in its signature for API-shape consistency, though the epoch math itself does not need it: preserving the raw (unwrapped) bars-elapsed fraction already preserves `Position`'s wrapped `BarIndex` and `BeatFraction` identically regardless of `barsPerLoop`.

## Deviations from Plan

None - plan executed exactly as written. No architectural changes, no missing-functionality additions beyond what the plan's `<action>`/`<behavior>` blocks specified were needed.

## Issues Encountered

`go test ./...` (full repository) surfaces a pre-existing, unrelated failure: `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` (`internal/trace/catalog`) reports `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`. This is outside this plan's `files_modified` (`internal/playback/*`, `internal/command/playback*.go`) and was already logged as a recurring, unrelated pre-existing condition under 03-01/03-03/03-04 in `deferred-items.md` — not fixed here (scope-boundary rule), only re-logged with this plan's confirmation that its own touched packages are fully green (`go test ./internal/playback/... ./internal/command/... ./internal/show/...`, `go build ./...`, `go vet ./...`).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

`internal/playback.Position`/`TapTempo`/`RecomputeEpoch`/`ValidateBPM`/`CrossedBarBoundary` are the exact primitives 03-07's tick-loop engine needs to compute per-tick position, adopt staged edits at a bar boundary (D-05), and drive chase/motion step selection off the same clock (D-10). No blockers for 03-07: this plan's `internal/playback` package has zero imports from `internal/command`/any adapter package, keeping the real-time isolation boundary (SCEN-09) structurally intact.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-22*
