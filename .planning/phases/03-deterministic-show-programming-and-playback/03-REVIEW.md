---
phase: 03-deterministic-show-programming-and-playback
reviewed: 2026-07-21T00:00:00Z
depth: standard
files_reviewed: 38
files_reviewed_list:
  - internal/command/chase_motion_test.go
  - internal/command/history_test.go
  - internal/command/playback.go
  - internal/command/playback_bpm_test.go
  - internal/command/playback_engine_test.go
  - internal/command/programming.go
  - internal/command/programming_test.go
  - internal/command/scene.go
  - internal/command/scene_test.go
  - internal/command/theme_preset_test.go
  - internal/playback/clock.go
  - internal/playback/clock_test.go
  - internal/playback/compile.go
  - internal/playback/compile_test.go
  - internal/playback/engine.go
  - internal/playback/engine_test.go
  - internal/playback/evaluate.go
  - internal/playback/evaluate_test.go
  - internal/playback/frame.go
  - internal/playback/frame_test.go
  - internal/programming/chase.go
  - internal/programming/chase_test.go
  - internal/programming/history.go
  - internal/programming/history_test.go
  - internal/programming/motion.go
  - internal/programming/motion_test.go
  - internal/programming/preset.go
  - internal/programming/preset_test.go
  - internal/programming/programmer.go
  - internal/programming/programmer_test.go
  - internal/programming/selection.go
  - internal/programming/selection_test.go
  - internal/programming/theme.go
  - internal/programming/theme_test.go
  - internal/scene/blend.go
  - internal/scene/blend_test.go
  - internal/scene/layer.go
  - internal/scene/layer_test.go
  - internal/scene/scene.go
  - internal/scene/scene_test.go
  - internal/show/state.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-07-21T00:00:00Z
**Depth:** standard
**Files Reviewed:** 38 (source + test)
**Status:** issues_found

## Summary

This phase builds the deterministic show-programming domain model
(themes/presets/chases/motion presets/scenes/blend presets), the fixture
selection resolver, the programmer scratch buffer, and the real-time
playback engine (compile → evaluate → tick loop). The pure-function
design is careful and mostly well-tested: `Position`, `Evaluate`,
`ReduceLayers`, and `Resolve` are all provably deterministic and the
all-or-nothing `Compile` contract is solid.

One BLOCKER was found: the SCEN-08/D-11 "preserve musical position across
a BPM change" contract is fully implemented at the primitive level
(`playback.RecomputeEpoch`, unit-tested in isolation) but is never wired
into the engine's actual adoption path (`Engine.tick`/`StageEdit`/
`SwitchScene`), so the behavior the phase's own domain model
(`Scene.PreserveMusicalPositionOnBPMChange`) and `CompiledPlan.PreserveOnBPMChange`
promise is not actually observable through the delivered `Engine` — a BPM
change is invisible to the engine's fixed `loopStart` entirely, and no
code path recomputes it as "preserve" or "restart" would require. Three
WARNINGs (a CLI evaluate-position wrap gap, an unguarded exported
divide/mod, and a full-replace footgun in `scene layer set`) and two INFO
items round out the review.

## Critical Issues

### CR-01: SCEN-08/D-11 BPM-change preserve-or-restart is never applied by the engine

**File:** `internal/playback/engine.go:122-143` (also `internal/playback/compile.go:65,240`, `internal/playback/clock.go:145-168`)
**Issue:**
`clock.go` implements `RecomputeEpoch(preserve, oldBPM, newBPM, barsPerLoop, loopStart, now)` specifically to satisfy SCEN-08/D-11: "the running look/chase/motion neither blanks nor jumps mid-bar" across a BPM change, either by preserving the current bar/beat position (`preserve=true`) or restarting at bar 0 (`preserve=false`). `compile.go` dutifully carries the per-scene flag forward as `CompiledPlan.PreserveOnBPMChange` (from `Scene.PreserveMusicalPositionOnBPMChange`).

However, `Engine` never calls `RecomputeEpoch` and never reads `PreserveOnBPMChange` anywhere:
- `Engine.loopStart` is set exactly once in `NewEngine` (`e := &Engine{loopStart: time.Now(), ...}`) and is never reassigned by `tick`, `StageEdit`, or `SwitchScene`.
- `tick` promotes a staged plan at a bar boundary and simply recomputes `Position(now, plan.BPM, plan.BarsPerLoop, e.loopStart)` — using the *same* `loopStart` regardless of whether the newly-adopted plan's BPM differs from the previous plan's BPM, and regardless of `plan.PreserveOnBPMChange`.

A grep across the whole repository confirms `RecomputeEpoch` is called nowhere except its own unit tests (`clock_test.go`). There is no integration test anywhere (`engine_test.go`, `playback_bpm_test.go`, `playback_engine_test.go`) that stages a plan with a changed `Tempo.BPM` through a running `Engine` and asserts the resulting bar/beat position — the gap is untested as well as unimplemented.

Practical effect: if a future caller feeds a `show.State` with a changed `Tempo.BPM` into `Engine.StageEdit`, the adopted plan's musical position at the moment of adoption is neither "preserved" (bar/beat frozen across the change) nor "restarted" (bar 0) — it is whatever `Position` computes for the new BPM against the *original, untouched* epoch, an arbitrary discontinuity that violates the "never blanks or jumps mid-bar" invariant the whole `RecomputeEpoch` primitive, and SCEN-08/D-11 themselves, exist to prevent. (Today `Engine` is not yet wired into any `internal/command` route, so this is not yet reachable from the CLI — but it is a real gap in the phase's own Task 2 deliverable, which explicitly claims D-05/D-07/D-08 coverage including "one consistent adoption rule for every layer type and for a scene switch alike," and SCEN-08 is one of the phase's declared scenarios.)

**Fix:**
```go
// In Engine, track the BPM the currently-adopted plan was compiled with,
// and recompute loopStart via RecomputeEpoch at the moment a pending plan
// with a different BPM is adopted:

func (e *Engine) tick(now time.Time) {
	plan := e.activePlan.Load()
	if plan == nil {
		return
	}
	pos := Position(now, plan.BPM, plan.BarsPerLoop, e.loopStart)

	if pending := e.pendingPlan.Load(); pending != nil && crossedBarBoundary(e.lastBar, pos.BarIndex, plan.BarsPerLoop) {
		if pending.BPM != plan.BPM {
			e.loopStart = RecomputeEpoch(pending.PreserveOnBPMChange, plan.BPM, pending.BPM, plan.BarsPerLoop, e.loopStart, now)
		}
		e.activePlan.Store(pending)
		e.pendingPlan.Store(nil)
		plan = pending
		pos = Position(now, plan.BPM, plan.BarsPerLoop, e.loopStart)
	}
	...
}
```
Add an engine-level integration test (`TestEngineBPMChangePreservesPosition` / `...RestartsAtBarZero`) that stages a plan with a different `Tempo.BPM` and asserts the post-adoption `MusicalPosition` matches the documented preserve/restart contract, mirroring `clock_test.go`'s `TestBPMChangeEpoch*` but exercised through the real tick loop.

## Warnings

### WR-01: `playback evaluate --at` does not wrap BarIndex modulo the scene's BarsPerLoop, diverging from the engine's own `Position` semantics

**File:** `internal/command/playback.go:293-300` (`positionFromAt`)
**Issue:** Every other producer of a `MusicalPosition` in this codebase (`playback.Position`) guarantees `BarIndex` is always wrapped into `[0, barsPerLoop)` — this invariant is relied on by `motionPhase` (`bars/barsPerLoop` assumed to stay in `[0,1)`) and is explicitly documented in `clock.go`'s `MusicalPosition` doc comment ("already wrapped modulo barsPerLoop"). `positionFromAt`, used by the "playback evaluate" CLI route, does not wrap: `--at 5.5` against a 4-bar scene produces `BarIndex=5`, and `Evaluate` will compute `motionPhase = 5.5/4 = 1.375`, well outside the documented `[0,1)` range. `activeMotionKeyframe` doesn't crash (it degrades to "select the last keyframe"), but the CLI's own doc comment claims this route is "the headless, deterministic demonstration surface for the compiler/evaluator/engine" — yet for any `--at` beyond the loop length it silently diverges from what the real engine would ever produce (the engine's `Position` always wraps). No test exercises `--at` beyond `BarsPerLoop-1`, so this divergence is unnoticed by the current test suite.
**Fix:** Either reject `--at` values whose derived `BarIndex` falls outside `[0, activeScene.BarsPerLoop)` with `GOLC_PLAYBACK_USAGE`, or wrap it explicitly:
```go
func positionFromAt(at float64, barsPerLoop int) playback.MusicalPosition {
	bar := math.Floor(at)
	beatFraction := at - bar
	wrapped := int(bar) % barsPerLoop
	if wrapped < 0 {
		wrapped += barsPerLoop
	}
	return playback.MusicalPosition{BarIndex: wrapped, BeatFraction: beatFraction}
}
```

### WR-02: exported `playback.Position` panics (integer divide-by-zero) on `barsPerLoop <= 0` instead of returning a validation error

**File:** `internal/playback/clock.go:72-80`
**Issue:** `Position` is an exported, documented "pure function of its four arguments," yet unlike its sibling `ValidateBPM` (which defensively rejects a non-positive/non-finite/out-of-range `bpm`), `Position` performs `wholeBarsElapsed % barsPerLoop` with no guard on `barsPerLoop`. Calling `Position(now, bpm, 0, loopStart)` panics with an integer divide-by-zero rather than returning an error. Today every caller (`engine.go`, via `Compile`) only ever supplies a `BarsPerLoop` that has already passed `scene.ValidateScene`'s `[1, maxBarsPerLoop]` check, so this is not reachable through the current CLI/engine paths — but `Position` is a public package API with no such guarantee enforced at its own boundary, and a future caller (Phase 4 Art-Net worker, Phase 6 UI, Phase 7 API — all explicitly named in this package's own doc comments as future consumers) that calls it directly against untrusted/hand-constructed input will crash the process instead of getting a diagnostic.
**Fix:**
```go
func Position(now time.Time, bpm float64, barsPerLoop int, loopStart time.Time) MusicalPosition {
	if barsPerLoop <= 0 {
		barsPerLoop = 1 // or: document/require callers to pre-validate and panic is acceptable — but that should be explicit, not implicit
	}
	...
}
```
Preferably, have `Position` return `(MusicalPosition, error)` for `barsPerLoop <= 0`, or clearly document that callers must have validated `barsPerLoop` first (mirroring `Compile`'s own re-validation of `state.Tempo.BPM` via `ValidateBPM` before ever computing a position).

### WR-03: `scene layer set` fully replaces a layer's Selection on every call, silently discarding any previously configured selector unless every flag is re-supplied

**File:** `internal/command/scene.go:369-419` (`runSceneLayerSet`)
**Issue:** Unlike `chase update` (`internal/command/programming.go:1094-1238`), which tracks `has*` flags per field so an update only touches the fields the caller actually supplied, `runSceneLayerSet` always constructs a brand-new `scene.Layer{Kind, Enabled, Selection, Ref}` from only the flags present in *this* invocation and replaces the whole layer slot via `scene.SetLayer`. If an operator originally set a layer with `--pool <id>` and later runs `scene layer set <scene> --kind chase --ref <newid>` to simply repoint the chase reference (or `--disable` to temporarily disable it), the previously configured `Selection` (pools/groups/instances/fixtures) is silently wiped to its zero value — the layer's fixture scoping vanishes with no warning, exit code 0, and no diagnostic. This is a plausible operator footgun: nothing in the CLI output (`GOLC_SCENE_LAYER_SET: scene=%s kind=%s enabled=%t`) surfaces the fact that Selection was reset.
**Fix:** Either (a) merge into the existing layer's `Selection` when no selector flags are supplied on this invocation (fetch `targetScene.LayerByKind(kind)` first and default `parsed.pools/groups/instances/fixtureRefs` to the existing layer's values when the corresponding flags were never passed), or (b) explicitly document/require a `--selection-unchanged` style flag and print a warning/require confirmation when an existing non-empty Selection is about to be replaced with an empty one.

## Info

### IN-01: `activeMotionKeyframe` uses an unstable sort for tie-breaking equal-Phase keyframes

**File:** `internal/playback/evaluate.go:151-167`
**Issue:** `validateMotionKeyframe` (in `internal/programming/motion.go`) only bounds `Phase` to `[0,1]` — it never enforces uniqueness across a preset's keyframes. `activeMotionKeyframe` sorts a copy via `sort.Slice`, which is not a stable sort. If two authored keyframes share the exact same `Phase`, which one "wins" as the active keyframe at that phase is determined by `sort.Slice`'s internal (unstable, implementation-defined) tie-breaking rather than by authored order — the result is still deterministic run-to-run for a fixed Go toolchain/input (so SCEN-09's byte-identical-output guarantee itself holds), but the specific keyframe selected among ties is an accident of the sort implementation rather than a documented, intentional rule. This is easy for a future contributor to be surprised by.
**Fix:** Use `sort.SliceStable` (near-zero cost given `maxChaseSteps`-scale inputs) so "last-authored wins" among equal-Phase keyframes becomes an explicit, stable, and documentable contract, or add an explicit validation rejecting duplicate Phase values within one `MotionPreset` if ties should simply never be authorable.

### IN-02: `activeMotionKeyframe` panics on an empty `keyframes` slice; the only guard lives in its sole caller

**File:** `internal/playback/evaluate.go:151-167`, guarded at `internal/playback/evaluate.go:177`
**Issue:** `activeMotionKeyframe(keyframes, phase)` unconditionally indexes `sorted[len(sorted)-1]` after sorting; if `keyframes` is empty this panics with an out-of-range index. The function is currently only reachable through `resolveMotion`, which checks `len(layer.MotionPreset.Keyframes) == 0` first — so today this is unreachable — but the panic risk lives entirely in caller discipline rather than in the function's own contract, and nothing about the function's name or signature signals "never call this with an empty slice."
**Fix:** Add a defensive check inside `activeMotionKeyframe` itself (return a zero-value `MotionKeyframe` or an `(ok bool)` result for an empty slice) so the invariant is enforced at the function boundary rather than relying on every future caller remembering the guard.

---

_Reviewed: 2026-07-21T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
