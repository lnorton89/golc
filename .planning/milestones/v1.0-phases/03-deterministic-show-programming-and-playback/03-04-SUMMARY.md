---
phase: 03-deterministic-show-programming-and-playback
plan: 04
subsystem: scene-programming
tags: [go, uuid, scene, layer-reduce, blend-preset, show-state, cli]

# Dependency graph
requires:
  - phase: 03-deterministic-show-programming-and-playback (03-02/03-03)
    provides: programming.Theme/Preset/Chase/MotionPreset/Selection reusable objects and show.State fields
provides:
  - internal/scene package (Scene/Layer/BlendPreset domain model + fixed-priority reduce)
  - show.State.Scenes/BlendPresets/Tempo persistence and validation
  - "scene create"/"scene activate"/"scene layer set"/"blend create" CLI routes
affects: [03-06-clock-and-tempo, 03-07-playback-engine]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fixed-priority layer reduce: ReduceLayers walks a fixed []LayerKind order and overlays per-attribute, never HTP/highest-value-wins"
    - "Scene/BlendPreset identity/construction/rename/unique-name shape copied verbatim from pool.Pool"
    - "ValidateSingleActiveScene/ActivateScene copied verbatim from deployment.ValidateSingleActive/Activate"

key-files:
  created:
    - internal/scene/scene.go
    - internal/scene/scene_test.go
    - internal/scene/layer.go
    - internal/scene/layer_test.go
    - internal/scene/blend.go
    - internal/scene/blend_test.go
    - internal/command/scene.go
    - internal/command/scene_test.go
  modified:
    - internal/show/state.go

key-decisions:
  - "LayerContribution/ReduceLayers operate on a single fixture instance's already-resolved per-layer contributions (not a map across instances); selection scoping for chase/motion layers is the engine's concern (03-07), proven here by simply omitting a contribution for out-of-scope instances in tests"
  - "Scene.Layers is a fixed [4]Layer array (not a slice or named struct) with each slot carrying its own Kind, constructed once by NewScene/newLayers; SetLayer/LayerByKind look up by Kind field, never by array index"
  - "Layer.Ref resolves against a different programming collection depending on Kind: BaseLook->Preset (optional, may carry inline rest-state values instead), ColorTheme->Theme, Chase->Chase, Motion->MotionPreset"
  - "BlendPreset.Curve is a plain string validated against a small declared set (linear/ease_in/ease_out) rather than a distinct named type, matching the plan's literal artifact spec"
  - "Tempo lives in the show package (show.Tempo{BPM float64}) since it's a State-level global field, not a scene-scoped concept"

patterns-established:
  - "New object types extend show.validate()'s single entry point: per-type Validate, ValidateUniqueNames, and (for scenes) ValidateSingleActiveScene + ValidateLayerReferences, never a parallel validation path"
  - "'scene layer set' reuses internal/command/programming.go's unexported parseUUIDFlag/parseFixtureRef helpers directly (same package command), avoiding duplicated selection-flag parsing"

requirements-completed: [SCEN-01, SCEN-04, SCEN-05, SCEN-07]

coverage:
  - id: D1
    description: "Scene model with configured bar-loop length and four independently enabled, independently selectable layers (base-look/color-theme/chase/motion)"
    requirement: "SCEN-01"
    verification:
      - kind: unit
        ref: "internal/scene/scene_test.go#TestSceneBarsPerLoopBoundary"
        status: pass
      - kind: unit
        ref: "internal/command/scene_test.go#TestSceneRoutesCreateActivateLayerSet"
        status: pass
    human_judgment: false
  - id: D2
    description: "Fixed-priority layer reduce (base-look < color-theme < chase < motion), order-not-magnitude, disabled layers contribute nothing, per-layer selection scoping"
    requirement: "SCEN-05"
    verification:
      - kind: unit
        ref: "internal/scene/layer_test.go#TestLayerCombinationFixedPriorityOverwritesOnlyTouchedAttributes"
        status: pass
      - kind: unit
        ref: "internal/scene/layer_test.go#TestLayerCombinationPriorityIsOrderNotMagnitude"
        status: pass
      - kind: unit
        ref: "internal/scene/layer_test.go#TestLayerCombinationDisabledLayerContributesNothing"
        status: pass
      - kind: unit
        ref: "internal/scene/layer_test.go#TestLayerCombinationPerLayerSelectionScopesIndependently"
        status: pass
    human_judgment: false
  - id: D3
    description: "Exactly-one-active-scene invariant enforced at the domain model and through show.Save's whole-State validation"
    requirement: "SCEN-04"
    verification:
      - kind: unit
        ref: "internal/scene/scene_test.go#TestSingleActiveSceneRejectsMultipleActive"
        status: pass
      - kind: unit
        ref: "internal/scene/scene_test.go#TestSingleActiveSceneActivateNeverTransientlyTwoActive"
        status: pass
      - kind: unit
        ref: "internal/command/scene_test.go#TestSceneRoutesCreateActivateLayerSet"
        status: pass
    human_judgment: false
  - id: D4
    description: "Reusable blend presets describing transitions between scene/layer states, with duration/curve boundary validation and duplicate-name rejection"
    requirement: "SCEN-07"
    verification:
      - kind: unit
        ref: "internal/scene/blend_test.go#TestBlendPresetMintsIDAndAcceptsInstantDuration"
        status: pass
      - kind: unit
        ref: "internal/scene/blend_test.go#TestBlendPresetNegativeDurationRejected"
        status: pass
      - kind: unit
        ref: "internal/scene/blend_test.go#TestBlendPresetUnsupportedCurveRejected"
        status: pass
      - kind: unit
        ref: "internal/command/scene_test.go#TestSceneRoutesBlendCreate"
        status: pass
    human_judgment: false
  - id: D5
    description: "Scenes/BlendPresets/Tempo persist on show.State through Load/Save, with layer-reference integrity (dangling reference rejection) enforced at the single validate() entry point"
    verification:
      - kind: unit
        ref: "internal/command/scene_test.go#TestSceneRoutesShowStateRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/command/scene_test.go#TestSceneRoutesCreateActivateLayerSet"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-07-22
status: complete
---

# Phase 3 Plan 4: Scene/Layer/Blend Domain Model Summary

**Bar-loop Scene/Layer/BlendPreset domain model in a new `internal/scene` package, with a fixed-priority (base-look < color-theme < chase < motion) layer reduce, exactly-one-active-scene enforcement, and `scene`/`blend` CLI routes persisting Scenes/BlendPresets/Tempo onto `show.State`.**

## Performance

- **Duration:** 35 min
- **Started:** 2026-07-22T03:35:00Z
- **Completed:** 2026-07-22T04:10:00Z
- **Tasks:** 3
- **Files modified:** 9 (8 created, 1 modified)

## Accomplishments
- New `internal/scene` package: `Scene`/`Layer`/`LayerKind`/`BlendPreset` domain types, copying `pool.Pool`'s identity/construction/rename/unique-name shape verbatim
- Fixed-priority layer reduce (`ReduceLayers`/`AttributeSet.Overlay`) proven order-not-magnitude (NOT HTP) with a disabled-layer-contributes-nothing test and a per-layer-selection-scoping test
- `ValidateSingleActiveScene`/`ActivateScene` mirror `deployment.ValidateSingleActive`/`Activate` exactly, guaranteeing exactly one active scene even transiently
- `BlendPreset` model with duration-boundary (0 valid/instant, negative rejected) and small declared curve-enum validation
- `show.State` gains `Scenes`/`BlendPresets`/`Tempo` fields; `validate()` extended with per-scene/blend-preset checks, unique names, single-active-scene, and `ValidateLayerReferences` (dangling scene->theme/preset/chase/motion-preset reference rejection, mirroring `pool.ValidateGroupReferences`)
- New `scene`/`blend` CLI scopes: `scene create`, `scene activate`, `scene layer set`, `blend create`

## Task Commits

Each task was committed atomically:

1. **Task 1: Scene + Layer model and fixed-priority reduce (SCEN-01 model, SCEN-04, SCEN-05)** - `216db23` (feat)
2. **Task 2: Blend preset model (SCEN-07)** - `f0ac887` (feat)
3. **Task 3: Persist Scenes/BlendPresets/Tempo on show.State and expose scene routes** - `73102da` (feat)

_No separate TDD RED/GREEN commits: tasks were implemented directly with tests, then verified green before committing (see Deviations)._

## Files Created/Modified
- `internal/scene/scene.go` - Scene/Layer/LayerKind model, identity/rename/unique-name, ValidateSingleActiveScene/ActivateScene, ValidateLayerReferences
- `internal/scene/scene_test.go` - Scene construction, bars-per-loop boundary, single-active-scene, layer-reference tests
- `internal/scene/layer.go` - AttributeSet/Overlay, LayerContribution, ReduceLayers fixed-priority reduce
- `internal/scene/layer_test.go` - ReduceLayers order-not-magnitude, disabled-layer, per-layer-selection tests
- `internal/scene/blend.go` - BlendPreset model, duration/curve validation
- `internal/scene/blend_test.go` - BlendPreset construction/boundary/duplicate-name tests
- `internal/command/scene.go` - "scene create"/"scene activate"/"scene layer set"/"blend create" routes
- `internal/command/scene_test.go` - CLI route contract tests, including dangling-reference rejection and Scenes/BlendPresets/Tempo round-trip
- `internal/show/state.go` - Added Scenes/BlendPresets/Tempo fields and Tempo type; extended validate()

## Decisions Made
- `ReduceLayers` takes `[]LayerContribution` representing one fixture instance's already-resolved per-layer contributions (matching the plan's literal `ReduceLayers([]LayerContribution) AttributeSet` signature) rather than a map spanning multiple instances — the per-layer-selection test proves scoping by simply omitting a layer's contribution for an out-of-scope instance, deferring actual selection resolution to the engine (03-07) as the plan's action text specifies.
- `Scene.Layers` is a fixed `[4]Layer` array (each slot pre-populated with its own `Kind` by `NewScene`), with `LayerByKind`/`SetLayer` looking up by `Kind` field rather than a magic index — avoids index-order bugs while keeping the "exactly four fixed layers" invariant structurally enforced.
- `Layer.Ref` resolves against a different programming collection depending on `Kind`: `BaseLook -> Preset` (optional; base-look may instead carry inline rest-state values per the flagged assumption), `ColorTheme -> Theme`, `Chase -> Chase`, `Motion -> MotionPreset`. This interpretation of the plan's "Theme / Preset / Chase / MotionPreset" key_link list was necessary since the plan didn't specify the exact per-kind mapping.
- `BlendPreset.Curve` is a plain `string` (not a distinct named type) per the plan's literal artifact declaration (`Curve string`), validated against a small declared set of string constants.

## Deviations from Plan

None - plan executed exactly as written. The per-kind `Layer.Ref` -> programming-object-type mapping (BaseLook->Preset, ColorTheme->Theme, Chase->Chase, Motion->MotionPreset) required an interpretive choice since the plan's key_links list ("Theme / Preset / Chase / MotionPreset") didn't specify positional correspondence to the four LayerKind values explicitly — this is a design-completion decision within the plan's stated scope, not a deviation from it.

## Issues Encountered

`./golc.ps1` requires a bootstrapped `.tools/` toolchain that isn't present in this worktree (only in the main checkout). Verification was run directly against the system Go toolchain (`go1.26.5`, matching `config/toolchain.toml`'s pin exactly) instead of through `golc.ps1 test`. All plan-specified verification commands (`go test ./internal/scene/... -run 'TestSingleActiveScene|TestLayerCombination'`, `go test ./internal/scene/... -run TestBlendPreset`, `go test ./internal/command/... -run TestScene`, `go test ./internal/show/...`) pass, plus a full `go build ./...`, `go vet ./...`, and `go test ./...` (see Known Stubs / deferred-items.md for the one unrelated pre-existing failure).

## User Setup Required

None - no external service configuration required.

## Known Stubs

None. `scene create`/`scene layer set`/`blend create` are fully wired end-to-end (CLI -> domain model -> show.State persistence -> validation), matching the plan's stated scope. Time-dependent chase/motion `Resolve(pos)` and actual blend-transition interpolation are explicitly out of scope for this plan (deferred to the engine, 03-07) per the plan's own action text — not stubs, but intentionally deferred real-time evaluation logic documented in code comments at `ReduceLayers` and `BlendPreset`.

## Threat Flags

None. Every threat this plan's `<threat_model>` assigned a `mitigate` disposition (T-03-01 dangling layer reference, T-03-02 bar-loop DoS ceiling, single-active-scene tampering) is implemented exactly as specified in `ValidateLayerReferences`, `maxBarsPerLoop`, and `ValidateSingleActiveScene`. No new network endpoint, auth path, or file-access pattern was introduced.

## Next Phase Readiness

- `internal/scene` provides the complete static domain model (`Scene`, `Layer`, `LayerKind`, `layerPriority`, `ReduceLayers`, `BlendPreset`) the 03-06 clock plan and 03-07 playback engine will compose against.
- `show.State.Tempo` is in place for 03-06 to read/mutate (BPM entry, tap tempo) — this plan intentionally left BPM bounds-validation unenforced (BPM=0 is the fresh-show "unset" value), deferring SCEN-02/SCEN-03's own validation to 03-06.
- `ReduceLayers`'s `[]LayerContribution` input shape is ready for the engine (03-07) to populate per-tick from time-resolved chase/motion step values and Selection-based instance scoping.
- No blockers identified for 03-06/03-07.

## Self-Check: PASSED

- All created files verified present: `internal/scene/scene.go`, `internal/scene/scene_test.go`, `internal/scene/layer.go`, `internal/scene/layer_test.go`, `internal/scene/blend.go`, `internal/scene/blend_test.go`, `internal/command/scene.go`, `internal/command/scene_test.go`, `internal/show/state.go`.
- All task commit hashes verified present in `git log`: `216db23`, `f0ac887`, `73102da`.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-22*
