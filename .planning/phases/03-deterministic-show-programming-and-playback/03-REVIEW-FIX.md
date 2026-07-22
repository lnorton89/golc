---
phase: 03-deterministic-show-programming-and-playback
fixed_at: 2026-07-22T05:17:54Z
review_path: .planning/phases/03-deterministic-show-programming-and-playback/03-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-07-22T05:17:54Z
**Source review:** .planning/phases/03-deterministic-show-programming-and-playback/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (fix_scope: critical_warning -- CR-01, WR-01, WR-02, WR-03)
- Fixed: 4
- Skipped: 0

## Fixed Issues

### CR-01: SCEN-08/D-11 BPM-change preserve-or-restart is never applied by the engine

**Files modified:** `internal/playback/engine.go`, `internal/playback/engine_test.go`
**Commit:** `0de891c`
**Applied fix:** `Engine.tick` now detects when the plan being adopted at a bar-boundary
crossing carries a different `BPM` than the plan it replaces, and in that case
recomputes `e.loopStart` via `RecomputeEpoch(pending.PreserveOnBPMChange, plan.BPM,
pending.BPM, plan.BarsPerLoop, e.loopStart, now)` before recomputing the post-adoption
position. This wires the previously-orphaned `RecomputeEpoch` primitive into the real
adoption path exactly as the review's fix suggested. Updated the `Engine`/`tick` doc
comments to describe the new recompute behavior instead of claiming `loopStart` is
"established once at construction and never recomputed afterward." Added two
engine-level integration tests, `TestEngineBPMChangePreservesPosition` and
`TestEngineBPMChangeRestartsAtBarZero`, that stage a plan with a different `Tempo.BPM`
through a running `Engine` and assert the resulting bar/beat position against what the
old-BPM plan would have reported at the same instant -- closing the gap the review
noted ("no integration test anywhere ... asserts the resulting bar/beat position").
Verified: `go build ./...` clean; `go test ./internal/playback/... ./internal/command/...`
all pass, including the two new tests.

### WR-01: `playback evaluate --at` does not wrap BarIndex modulo the scene's BarsPerLoop

**Files modified:** `internal/command/playback.go`, `internal/command/playback_engine_test.go`
**Commit:** `d291bcb`
**Applied fix:** `positionFromAt` now takes a `barsPerLoop int` parameter and wraps the
derived `BarIndex` into `[0, barsPerLoop)` (normalizing a negative remainder back into
range), matching `playback.Position`'s documented invariant that every other
`MusicalPosition` producer in the codebase relies on. `runPlaybackEvaluate` now passes
`plan.BarsPerLoop` (already available from the compiled plan) at the call site. Added
`TestPlaybackEvaluateWrapsBarIndexModuloBarsPerLoop`, which exercises `--at 5.5` against
a 4-bar scene and asserts the CLI output reports the wrapped `bar=1` rather than the
unwrapped `bar=5`. Verified: `go build ./...` clean; existing `--at 2.0`/`--at 0.0` tests
against 4-bar scenes are unaffected (already within range); new test passes.

### WR-02: exported `playback.Position` panics on `barsPerLoop <= 0`

**Files modified:** `internal/playback/clock.go`, `internal/playback/clock_test.go`
**Commit:** `8d3e53c`
**Applied fix:** `Position` now defensively clamps a non-positive `barsPerLoop` to `1`
before computing `wholeBarsElapsed % barsPerLoop`, rather than panicking with an integer
divide-by-zero. This keeps `Position`'s existing signature (no breaking API change for
current callers, all of which already validate `barsPerLoop` via `scene.ValidateScene`
before compile time) while making the exported function's own boundary safe for a future
direct caller (Phase 4/6/7, as the package doc comments name) that passes unvalidated
input. Documented the clamp behavior explicitly in the function's doc comment. Added
`TestClockPositionNonPositiveBarsPerLoopDoesNotPanic`, covering `barsPerLoop` of `0`,
`-1`, and `-100`, asserting the clamped single-bar-loop result rather than a panic.
Verified: `go build ./...` clean; `go test ./internal/playback/...` passes, including
the new test and all pre-existing `TestClockPosition*` cases.

### WR-03: `scene layer set` fully replaces a layer's Selection on every call

**Files modified:** `internal/command/scene.go`, `internal/command/scene_test.go`
**Commit:** `b9b2848`
**Applied fix:** Implemented fix option (a) from the review: `sceneLayerSetArgs` now
tracks a `has*` flag per selector kind (`hasPools`/`hasGroups`/`hasInstances`/
`hasFixtures`), set when the caller supplies at least one `--pool`/`--group`/
`--instance`/`--fixture` flag on that invocation (mirroring the existing `hasUnit`/
`hasStepDuration` convention in `chase update`). `runSceneLayerSet` now looks up the
target layer's existing `Selection` via `targetScene.LayerByKind` and, for any selector
kind the caller did not mention at all on this invocation, carries the existing value
for that kind forward instead of replacing it with an empty slice. A selector kind
supplied on this invocation still fully replaces the corresponding existing value, as
before. Added `TestSceneLayerSetPreservesSelectionWhenOmitted`, which sets a chase layer
with `--pool <id>`, then repoints `--ref` in a second call without re-supplying
`--pool`, and asserts the pool selector survives; a third call that does re-supply
`--pool` with a different value confirms explicit replacement still works. Verified:
`go build ./...` clean; `go test ./internal/command/...` passes, including the new test
and the pre-existing `TestSceneRoutesCreateActivateLayerSet`.

## Skipped Issues

None -- all in-scope findings were fixed. (IN-01 and IN-02 are out of scope for this run:
`fix_scope` is `critical_warning`, which excludes Info-severity findings.)

---

_Fixed: 2026-07-22T05:17:54Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
