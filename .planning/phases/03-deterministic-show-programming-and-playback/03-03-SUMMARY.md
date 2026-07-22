---
phase: 03-deterministic-show-programming-and-playback
plan: 03
subsystem: programming
tags: [go, uuid, strictjson, domain-model, cli]

# Dependency graph
requires:
  - phase: 03-deterministic-show-programming-and-playback (03-01)
    provides: internal/programming package scaffold (Selection/ProgrammerState), show.State persistence pattern
provides:
  - "programming.Chase: reusable, ordered, tempo-relative chase domain type (PROG-05)"
  - "programming.MotionPreset: position/beam-scoped motion preset domain type (PROG-06)"
  - "show.State.Chases / show.State.MotionPresets persisted fields with extended validate()"
  - "chase create / motion create CLI routes"
affects: [03-04 (scene chase/motion layers), 03-07 (playback engine evaluating chases/motion against musical position)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Chase/MotionPreset copy pool.Pool's identity/construction/rename/unique-name shape verbatim (UUIDv7 minted once, never derived from Name)"
    - "NewChase/NewMotionPreset validate via the same ValidateChase/ValidateMotionPreset function show.validate() re-runs at Load/Save time -- one validation path, not a parallel one"
    - "MotionScopedCapabilities() is a hardcoded position/beam lookup set (pan/tilt/zoom/focus) independent of PresetBeam's broader beam set (which also allows gobo/shutter/strobe) -- D-04 deliberately narrows motion preset scope below the general 'beam' preset kind"

key-files:
  created:
    - internal/programming/chase.go
    - internal/programming/chase_test.go
    - internal/programming/motion.go
    - internal/programming/motion_test.go
    - internal/command/chase_motion_test.go
  modified:
    - internal/show/state.go
    - internal/command/programming.go

key-decisions:
  - "maxChaseSteps DoS ceiling set to 256 (mirrors internal/deployment's maxUniverseSearch precedent, threat T-03-02) -- no upstream guidance specified an exact number, chosen as generous headroom over v1's small-rig scale."
  - "MotionScopedCapabilities() = {pan, tilt, zoom, focus} exactly. D-04 names 'pan/tilt plus zoom/focus/iris/prism beam-shaping', but iris/prism are not part of fixture.SupportedCapabilityTypes' nine-value v1 enum -- the effective scoped set is the four capabilities that actually exist today. Adding iris/prism later is a fixture-model addition, not a motion.go change."
  - "chase create / motion create mint an empty chase/motion preset (zero Steps/Keyframes) rather than accepting authored steps/keyframes inline -- populating them from the programmer/selection state is a later scene-authoring concern (03-04), matching how theme create/preset record are split across this phase's plans."

patterns-established:
  - "Domain validation (ValidateX) is reused by both the constructor (NewX) at author-time and show.validate() at Load/Save time -- a hand-edited show file cannot smuggle a step-unit/duration/capability-scope violation past disk trust."

requirements-completed: [PROG-05, PROG-06]

coverage:
  - id: D1
    description: "A show author can create a reusable chase with ordered steps and tempo-relative (bar- or beat-relative) step timing; step order is never reordered/deduped/randomized."
    requirement: "PROG-05"
    verification:
      - kind: unit
        ref: "internal/programming/chase_test.go#TestChaseNewChaseMintsIDAndPreservesStepOrder"
        status: pass
      - kind: unit
        ref: "internal/programming/chase_test.go#TestChaseNewChaseDeterministicConstruction"
        status: pass
      - kind: unit
        ref: "internal/programming/chase_test.go#TestChaseNewChaseTooManyStepsRejected"
        status: pass
      - kind: integration
        ref: "internal/command/chase_motion_test.go#TestChaseMotionRoutes"
        status: pass
    human_judgment: false
  - id: D2
    description: "A show author can create a reusable motion preset built only from position/beam semantic capabilities; a color or gobo/color-wheel capability is always rejected."
    requirement: "PROG-06"
    verification:
      - kind: unit
        ref: "internal/programming/motion_test.go#TestMotionPresetNewMotionPresetMintsIDAndAcceptsScopedKeyframes"
        status: pass
      - kind: unit
        ref: "internal/programming/motion_test.go#TestMotionPresetNewMotionPresetRejectsColorCapability"
        status: pass
      - kind: unit
        ref: "internal/programming/motion_test.go#TestMotionPresetNewMotionPresetRejectsGoboCapability"
        status: pass
      - kind: integration
        ref: "internal/command/chase_motion_test.go#TestChaseMotionLoadRejectsOverScopeMotionCapability"
        status: pass
    human_judgment: false
  - id: D3
    description: "Chases/MotionPresets persist through show.Load/Save with unique-name enforcement, surfaced as chase create / motion create CLI routes."
    verification:
      - kind: integration
        ref: "internal/command/chase_motion_test.go#TestChaseMotionShowStateRoundTrip"
        status: pass
      - kind: integration
        ref: "internal/command/chase_motion_test.go#TestChaseMotionRoutes"
        status: pass
    human_judgment: false

# Metrics
duration: 35min
completed: 2026-07-22
status: complete
---

# Phase 3 Plan 3: Reusable Chases and Motion Presets Summary

**Reusable, ordered tempo-relative chases (PROG-05) and position/beam-scoped motion presets (PROG-06), persisted on show.State and driven by new chase create / motion create CLI routes.**

## Performance

- **Duration:** 35 min
- **Completed:** 2026-07-22
- **Tasks:** 3 completed
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments
- `programming.Chase` mints a UUIDv7 identity and never reorders/dedupes/randomizes its authored `Steps` (D-09); `StepUnit` is bar- or beat-relative (D-10), `StepDuration` must be positive, and `maxChaseSteps` (256) bounds step count as a DoS ceiling (T-03-02).
- `programming.MotionPreset` is strictly scoped to `pan`/`tilt`/`zoom`/`focus` via `MotionScopedCapabilities()` (D-04) -- any keyframe referencing `color` or `gobo` is rejected with `GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE` before an ID is ever minted.
- `show.State` gained non-omitempty `Chases`/`MotionPresets` fields; `show.validate()` extends with per-type `ValidateChase`/`ValidateMotionPreset` plus `ValidateChaseUniqueNames`/`ValidateMotionPresetUniqueNames`, in the single existing entry point.
- `chase create <name> --unit bar|beat --step-duration <value> --show <path>` and `motion create <name> --show <path>` CLI routes follow `runPoolCreate`'s parse-Load-mutate-Save-Stdout shape, with `GOLC_CHASE_CREATED`/`GOLC_MOTION_PRESET_CREATED` success lines and duplicate-name rejection wrapped in `GOLC_SHOW_STATE_INVALID`.

## Task Commits

Each task followed the RED/GREEN TDD gate sequence:

1. **Task 1: Chase domain type** - `9458fbf` (test, RED) -> `0727472` (feat, GREEN)
2. **Task 2: MotionPreset domain type** - `cf919a8` (test, RED) -> `153d029` (feat, GREEN)
3. **Task 3: show.State persistence + chase/motion routes** - `aad7f9c` (test, RED) -> `1911b53` (feat, GREEN)

_Note: this plan's TDD tasks are two-commit RED/GREEN pairs -- no REFACTOR commit was needed for any task._

## Files Created/Modified
- `internal/programming/chase.go` - `Chase`/`ChaseStep`/`StepUnit` domain types, `NewChase`/`RenameChase`/`ValidateChase`/`ValidateChaseUniqueNames`
- `internal/programming/chase_test.go` - Order-preservation, validation, ceiling, and unique-name tests
- `internal/programming/motion.go` - `MotionPreset`/`MotionKeyframe`/`MotionKeyframeValue` domain types, `MotionScopedCapabilities`, `NewMotionPreset`/`RenameMotionPreset`/`ValidateMotionPreset`/`ValidateMotionPresetUniqueNames`
- `internal/programming/motion_test.go` - Scope-rejection, out-of-range, validation, and unique-name tests
- `internal/show/state.go` - Added `Chases`/`MotionPresets` fields; extended `validate()`
- `internal/command/programming.go` - Added `chase`/`motion` scopes and `chase create`/`motion create` routes + handlers
- `internal/command/chase_motion_test.go` - Route round-trip, duplicate-name, usage-error, and hand-edited-over-scope-capability tests

## Decisions Made
- `maxChaseSteps` set to 256 (no upstream number specified; chosen as generous headroom mirroring `deployment.maxUniverseSearch`'s precedent-setting rationale).
- `MotionScopedCapabilities()` resolves to exactly `{pan, tilt, zoom, focus}` since `iris`/`prism` (named in D-04's prose) are not yet part of `fixture.SupportedCapabilityTypes`' nine-value v1 enum. This is documented in code comments so a future fixture-model addition of iris/prism has a clear, single place to extend the scoped set.
- `chase create`/`motion create` mint an empty (zero-step/zero-keyframe) object, deferring population to a later scene-authoring plan (03-04) -- matching this phase's existing `theme create`/`preset record` split.

## Deviations from Plan

None - plan executed exactly as written. All three tasks' `<behavior>`/`<action>`/`<acceptance_criteria>` were implemented as specified; no Rule 1-4 auto-fixes were needed.

## Issues Encountered

- `./golc.ps1 test` (the plan's wave-gate command) fails in this worktree with `GOLC_TOOL_MISSING: run 'powershell -NoProfile -File .\golc.ps1 bootstrap' first` -- the worktree's local toolchain has not been bootstrapped. Substituted `go build ./...`, `go vet ./...`, and `go test ./...` directly, which is a superset of the plan's own `go test` verification commands. Every package this plan touches (`internal/programming`, `internal/command`, `internal/show`) is green.
- `go test ./...` surfaces one unrelated pre-existing failure: `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` (`internal/trace/catalog`), a `GOLC_MIGRATE_DRIFT` on `.planning/linear-map.json` vs. the canonical schema-2 migration output. Confirmed pre-existing and out of this plan's scope (already logged once under 03-01 in `.planning/phases/03-deterministic-show-programming-and-playback/deferred-items.md`; reconfirmed and re-logged under a new 03-03 entry in the same file, no fix attempted per the scope-boundary rule).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `programming.Chase`/`programming.MotionPreset` and their persisted `show.State` fields are ready for 03-04 (scene chase/motion layers) to reference by ID.
- `03-07`'s playback engine can evaluate a `Chase`'s `Steps`/`StepUnit`/`StepDuration` and a `MotionPreset`'s `Keyframes`/`Phase` against the shared musical clock (D-10) once that plan builds the pure `Position`/`Evaluate` functions described in 03-RESEARCH.md.
- No blockers. The pre-existing `TestScopeLinearMap` drift (unrelated to this plan) remains open for a future triage pass.

## Self-Check: PASSED

All created files verified present on disk (`internal/programming/chase.go`, `internal/programming/chase_test.go`, `internal/programming/motion.go`, `internal/programming/motion_test.go`, `internal/command/chase_motion_test.go`), all modified files verified present (`internal/show/state.go`, `internal/command/programming.go`), and all six task commits plus the docs commit verified present in `git log` (`9458fbf`, `0727472`, `cf919a8`, `153d029`, `aad7f9c`, `1911b53`, `7428359`).

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-22*
