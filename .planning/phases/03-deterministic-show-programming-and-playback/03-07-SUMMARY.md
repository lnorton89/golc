---
phase: 03-deterministic-show-programming-and-playback
plan: 07
subsystem: playback
tags: [go, real-time-engine, sync-atomic, deterministic-evaluation, cli]

# Dependency graph
requires:
  - phase: 03-04
    provides: "scene.Scene/Layer model, fixed-priority ReduceLayers/AttributeSet.Overlay, ValidateLayerReferences"
  - phase: 03-06
    provides: "playback.Position/CrossedBarBoundary/ValidateBPM pure musical clock"
provides:
  - "internal/playback.Compile(show.State) -> CompiledPlan: all-or-nothing compiler resolving the active scene's four layers (selection + theme/preset/chase/motion-preset reference), never mutating input State"
  - "internal/playback.Evaluate(plan, pos) -> Frame: pure fixed-priority evaluator, byte-identical across repeated/concurrent calls (SCEN-09 mechanical proof)"
  - "internal/playback.Engine: real-time tick loop with atomic Frame/plan publish, next-bar staged-edit adoption (D-05), reject-and-keep-last-valid (D-06)"
  - "playback evaluate / playback switch CLI routes: headless deterministic demonstration surface for SCEN-06/SCEN-09"
affects: [phase-4-artnet, phase-6-ui, phase-7-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure compile-then-evaluate split mirroring internal/pool/impact.go's BuildImpactPlan shape: Compile validates+flattens (all-or-nothing), Evaluate is a pure function of (plan, position)"
    - "Lock-free single-writer/multi-reader publish via atomic.Pointer[T] for activeFrame/activePlan/pendingPlan -- no adapter can block or backpressure the tick loop"
    - "Integer BarIndex-transition detection (crossedBarBoundary) for bar-boundary adoption, never a BeatFraction equality check"

key-files:
  created:
    - internal/playback/frame.go
    - internal/playback/frame_test.go
    - internal/playback/compile.go
    - internal/playback/compile_test.go
    - internal/playback/evaluate.go
    - internal/playback/evaluate_test.go
    - internal/playback/engine.go
    - internal/playback/engine_test.go
    - internal/command/playback_engine_test.go
  modified:
    - internal/command/playback.go

key-decisions:
  - "CompiledPlan carries no LoopStart field; the engine owns a single fixed loopStart epoch established once at NewEngine construction (never recomputed on a scene switch or edit) -- Position's determinism guarantee then applies uniformly regardless of how many scene switches have occurred, and avoids needing a separate epoch-recompute rule for every possible edit type"
  - "A layer's per-instance object data (base-look Preset.Attributes, color-theme Theme.Colors) is filtered by intersecting PresetAttribute/ColorAssignment.InstanceID against the layer's own resolved Selection (D-03) -- an authored attribute for an instance outside the layer's current selection contributes nothing"
  - "Chase step selection is a step-function of loop-relative bars position (floor(bars-or-beats / stepDuration) % stepCount); motion keyframe selection is also a step-function (last keyframe whose Phase <= current loop phase), not interpolated -- both are pure, deterministic, and match the plan's 'pure function of pos' requirement without introducing an undocumented interpolation contract"
  - "Motion keyframe values apply uniformly to every instance in the layer's Selection (MotionKeyframeValue carries no InstanceID, unlike Preset/Theme's per-instance records) -- a motion preset's authored path is selection-wide by design (CONTEXT D-04)"
  - "SwitchScene(name string) takes only a scene name (per the plan's declared signature); Engine stores the most recently staged show.State via its own atomic.Pointer[show.State] (lastState) so SwitchScene can activate a scene without the caller resupplying the whole State"

patterns-established:
  - "playback.CompiledLayer/CompiledChaseStep: every reference (Selection, per-step Selection override) is resolved once at Compile time; Evaluate never re-resolves a Selection or does any I/O"
  - "crossedBarBoundary(lastBar, curBar, barsPerLoop int) bool: -1 sentinel for 'no tick yet', and an out-of-range lastBar (stale from a differently-sized prior loop) always reports crossed -- both self-heal cleanly rather than requiring special-cased engine startup/switch logic"

requirements-completed: [SCEN-06, SCEN-09]

coverage:
  - id: D1
    description: "Compile(show.State) validates every scene->layer->theme/preset/chase/motion-preset reference and every resolved attribute value, flattening the single active scene into an immutable CompiledPlan all-or-nothing -- any unresolved reference, invalid BPM, or missing active scene fails the whole compile (GOLC_PLAYBACK_PLAN_INVALID / GOLC_PLAYBACK_NO_ACTIVE_SCENE), never a partial plan, and never mutates its input State"
    requirement: "SCEN-09"
    verification:
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileResolvesAllFourLayers"
        status: pass
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileNoActiveScene"
        status: pass
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileInvalidBPM"
        status: pass
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileAllOrNothingDanglingLayerReference"
        status: pass
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileAllOrNothingDanglingSelectionReference"
        status: pass
      - kind: unit
        ref: "internal/playback/compile_test.go#TestCompileNeverMutatesState"
        status: pass
    human_judgment: false
  - id: D2
    description: "Evaluate(plan, pos) is a pure fixed-priority (base-look < color-theme < chase < motion) layer reduce producing byte-identical Frames across repeated and concurrent calls with the same (plan, pos) -- the mechanical proof of SCEN-09; a disabled layer contributes nothing; chase step index advances as a pure function of position"
    requirement: "SCEN-09"
    verification:
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestDeterministicEvaluateSameArgs"
        status: pass
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestDeterministicEvaluateAcrossGoroutines"
        status: pass
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestEvaluateFixedPriorityOverridesPerAttribute"
        status: pass
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestEvaluateDisabledLayerContributesNothing"
        status: pass
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestEvaluateChaseStepAdvancesWithPosition"
        status: pass
      - kind: unit
        ref: "internal/playback/evaluate_test.go#TestEvaluateMotionKeyframeSelection"
        status: pass
    human_judgment: false
  - id: D3
    description: "The engine adopts a staged edit or scene switch atomically at the next bar boundary (never mid-bar), keeps running the last valid plan on a rejected edit (activePlan/pendingPlan untouched), requires no lock/pause/detach precondition (D-08), and is delay-deterministic and race-clean under concurrent CurrentFrame reads"
    requirement: "SCEN-06"
    verification:
      - kind: unit
        ref: "internal/playback/engine_test.go#TestImmediateSwitch"
        status: pass
      - kind: unit
        ref: "internal/playback/engine_test.go#TestEngineStageEditRejectsInvalidLeavesPlansUntouched"
        status: pass
      - kind: unit
        ref: "internal/playback/engine_test.go#TestEngineStageEditLiveActiveObjectNoLockRequired"
        status: pass
      - kind: unit
        ref: "internal/playback/engine_test.go#TestEngineDelayedTickMatchesSequentialTicks"
        status: pass
      - kind: unit
        ref: "internal/playback/engine_test.go#TestEngineStartStopCleanShutdown"
        status: pass
      - kind: unit
        ref: "internal/playback/engine_test.go#TestEngineCurrentFrameNonBlockingUnderConcurrentTick"
        status: pass
    human_judgment: false
  - id: D4
    description: "'playback evaluate --at <bar>.<beatfraction> [--json] --show <path>' compiles the active scene and prints the deterministic Frame (byte-identical across repeated runs); a show with no active scene or an invalid compile exits non-zero with the matching diagnostic. 'playback switch <scene> --show <path>' activates a scene via scene.ActivateScene and saves; an unknown scene is rejected"
    requirement: "SCEN-06"
    verification:
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackEvaluate"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackEvaluateHumanReadableSummary"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackEvaluateNoActiveScene"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackEvaluateInvalidPlan"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackEvaluateMissingAtUsage"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackSwitchActivatesScene"
        status: pass
      - kind: unit
        ref: "internal/command/playback_engine_test.go#TestPlaybackSwitchUnknownScene"
        status: pass
    human_judgment: false

duration: 50min
completed: 2026-07-22
status: complete
---

# Phase 3 Plan 07: Deterministic Playback Compiler, Evaluator, and Real-Time Engine Summary

**All-or-nothing `playback.Compile` + pure `playback.Evaluate` + a lock-free `playback.Engine` tick loop that adopts staged edits/scene switches only at the next bar boundary, proving SCEN-06/SCEN-09 mechanically and via a headless `playback evaluate`/`playback switch` CLI**

## Performance

- **Duration:** ~50 min
- **Tasks:** 3 completed
- **Files modified:** 9 (8 new, 1 modified)

## Accomplishments
- `playback.Compile(state show.State) (CompiledPlan, error)` mirrors `pool.BuildImpactPlan`'s pure validate-and-flatten shape: resolves the single active scene's four layers (Selection + theme/preset/chase/motion-preset Ref), fails all-or-nothing with `GOLC_PLAYBACK_PLAN_INVALID` on any dangling reference or invalid BPM, fails with `GOLC_PLAYBACK_NO_ACTIVE_SCENE` when no scene is active, and never mutates its input `show.State`.
- `playback.Evaluate(plan CompiledPlan, pos MusicalPosition) Frame` is a pure function reducing the fixed `base-look < color-theme < chase < motion` layer-priority order via `scene.AttributeSet.Overlay` — proven byte-identical across repeated calls and 100 concurrent goroutines with the same `(plan, pos)` (the SCEN-09 mechanical proof). Chase step index and motion-preset keyframe selection are both pure step-functions of the loop-relative bars position.
- `playback.Engine` (`activeFrame`/`activePlan`/`pendingPlan` as `atomic.Pointer[T]`, `lastState atomic.Pointer[show.State]`, a fixed `loopStart` epoch, `lastBar int`) runs a 40Hz `time.Ticker` tick loop: `StageEdit`/`SwitchScene` compile-and-stage a `pendingPlan` non-blockingly, rejecting an invalid edit with `GOLC_PLAYBACK_PLAN_INVALID` while leaving `activePlan`/`pendingPlan` completely untouched (D-06); `tick` promotes `pendingPlan` to `activePlan` only when `crossedBarBoundary` (integer `BarIndex` comparison) reports a transition (D-05), never mid-bar. Neither `StageEdit` nor `SwitchScene` requires any preceding lock/pause/detach call (D-08) — Engine exposes no such API. Verified delay-deterministic (a single coalesced tick jumping to a final `now` matches sequential on-time ticks) and race-clean under concurrent `CurrentFrame` reads during ticking.
- `playback evaluate --at <bar>.<beatfraction> [--json] --show <path>` and `playback switch <scene> --show <path>` CLI routes give SCEN-06/SCEN-09 a headless, deterministic demonstration surface: two `evaluate` runs against the same show produce byte-identical output; `switch` reuses `scene.ActivateScene` directly (no reimplemented layer resolution).

## Task Commits

1. **Task 1: Frame, all-or-nothing Compile, and pure Evaluate (SCEN-09, D-06)** - `c1c052c` (feat)
2. **Task 2: Real-time engine with next-bar adoption and lock-free publish (SCEN-06, SCEN-09, D-05/D-06/D-07)** - `833dda8` (feat)
3. **Task 3: Headless playback evaluate/switch command surface (SCEN-06, SCEN-09)** - `44930c2` (feat)

**Plan metadata:** commit pending final SUMMARY/state commit (see below)

## Files Created/Modified
- `internal/playback/frame.go` - `Frame` type (instance-keyed `scene.AttributeSet` snapshot)
- `internal/playback/frame_test.go` - Deterministic canonical-encode test
- `internal/playback/compile.go` - `Compile`, `CompiledPlan`, `CompiledLayer`, `CompiledChaseStep`
- `internal/playback/compile_test.go` - All-or-nothing / no-active-scene / invalid-BPM / never-mutates-State tests
- `internal/playback/evaluate.go` - `Evaluate`, per-kind layer resolvers, chase step index, motion keyframe selection
- `internal/playback/evaluate_test.go` - SCEN-09 determinism property test, fixed-priority override, disabled-layer, chase/motion selection tests
- `internal/playback/engine.go` - `Engine`, `NewEngine`, `StageEdit`, `SwitchScene`, `CurrentFrame`, `Start`, `Stop`, `tick`, `crossedBarBoundary`, `tickHz`/`tickInterval`
- `internal/playback/engine_test.go` - Bar-boundary adoption, reject-keeps-last-valid, D-08 no-lock, delay-determinism, Start/Stop, concurrent-Load race tests
- `internal/command/playback.go` - Added `playback evaluate` / `playback switch` routes (existing `playback bpm set`/`tap` routes from 03-06 untouched)
- `internal/command/playback_engine_test.go` - CLI-route-level tests for both new routes

## Decisions Made
- `CompiledPlan` carries no `LoopStart` field; `Engine` owns one fixed `loopStart` epoch set once at construction, never recomputed on a switch or edit — simpler than a per-switch epoch-recompute rule, and `Position`'s own purity keeps this fully deterministic regardless of how many switches occur.
- Base-look/color-theme per-instance attribute data is filtered by intersecting the object's `InstanceID`s against the layer's own resolved Selection (D-03) — an authored attribute for an instance outside the layer's current selection contributes nothing.
- Chase step and motion-preset keyframe selection are both step-functions (never interpolated) of the loop-relative bars/phase position — deterministic, pure, and matching the plan's "pure function of pos" requirement without inventing an undocumented interpolation contract 03-RESEARCH.md's Open Question 3 left unresolved.
- `SwitchScene(name string) error` matches the plan's declared signature exactly; `Engine` tracks the most recently staged `show.State` via its own `atomic.Pointer[show.State]` so `SwitchScene` never needs the caller to resupply the whole State.

## Deviations from Plan

None - plan executed exactly as written. Task 1's `<behavior>` bullet "Compile of a State with no active scene returns GOLC_PLAYBACK_NO_ACTIVE_SCENE (or an empty-but-valid plan per the chosen contract -- pick one and test it)" explicitly left the choice open; GOLC_PLAYBACK_NO_ACTIVE_SCENE was chosen (matches the artifacts_produced error-code list) and is tested (`TestCompileNoActiveScene`). No architectural changes, no missing-functionality additions beyond what the plan's `<action>`/`<behavior>` blocks specified were needed.

## Issues Encountered

`go test ./...` (full repository) surfaces the same pre-existing, unrelated failure already logged under 03-01/03-03/03-04/03-05/03-06: `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` (`internal/trace/catalog`) reports `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`. Outside this plan's `files_modified` (`internal/playback/*`, `internal/command/playback.go`) — not fixed here (scope-boundary rule); re-logged in `deferred-items.md` with confirmation this plan's own touched packages are fully green (`go test ./internal/playback/... -race`, `go test ./internal/command/... -run TestPlayback`, `go test ./internal/show/...`, full `go build ./...` and `go vet ./...` clean).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

This closes out Phase 3's final plan (wave 6, depends_on 03-04/03-06): the compiler/evaluator/engine trio assembles the clock (03-06), scene model + fixed-priority reduce (03-04), and programming objects (03-01/02/03) into the real-time engine PROJECT.md's "Live reliability" constraint requires, with `internal/playback` importing nothing from `internal/command` or any future adapter package (verified via `go list -deps`). SCEN-06 and SCEN-09 are proven both mechanically (unit/property tests) and at the headless CLI surface — Phase 4's Art-Net worker, Phase 6's UI, and Phase 7's API can all consume `Engine.CurrentFrame()` as read-only `atomic.Pointer` consumers without any rework.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-22*

## Self-Check: PASSED

All 9 created/modified files verified present on disk (`internal/playback/frame.go`, `frame_test.go`, `compile.go`, `compile_test.go`, `evaluate.go`, `evaluate_test.go`, `engine.go`, `engine_test.go`, `internal/command/playback_engine_test.go`, plus modified `internal/command/playback.go`); all three task commits (`c1c052c`, `833dda8`, `44930c2`) verified present in `git log --oneline --all`.
