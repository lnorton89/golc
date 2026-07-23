---
phase: 06-wails-authoring-and-operator-surface
plan: 12
subsystem: ui
tags: [wails, react, go, scene-programming, tdd, requirements-traceability]

# Dependency graph
requires:
  - phase: 06-wails-authoring-and-operator-surface (06-01/06-06)
    provides: "internal/command scene/programming CLI routes (scene create/activate/layer set, theme/motion/chase create, programmer set, preset record, blend create) and PlaybackService's currentLayerRef Ref-preservation pattern this plan mirrors"
  - phase: 06-wails-authoring-and-operator-surface (06-10/06-11)
    provides: "wailsBridge.ts's shared window.go.wails declaration, App.tsx's feature-region mount list, cmd/golc-desktop/main.go's Bind list -- all extended, none replaced"
provides:
  - "internal/wails.ProgrammingService: Go binding over scene/theme/chase/motion/programmer/preset/blend command routes (CreateScene, ActivateScene, SetSceneLayer, CreateTheme, CreateMotion, CreateChase, ProgrammerSet, RecordPreset, CreateBlend, ListProgramming)"
  - "frontend SceneProgramming component: on-screen scene/look programming surface (create scene, create each look kind, enable+point all four layers, activate, create blend)"
  - "wailsBridge.ts ProgrammingService bridge interface + helper exports"
  - "REQUIREMENTS.md PLAY-01..09 status corrected to Complete (checklist + Traceability table) per 06-VERIFICATION.md"
affects: [06-wails-authoring-and-operator-surface end-of-phase UAT, any future phase touching scene/programming CLI routes]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "ProgrammingService mirrors PlaybackService/SurfaceService/FixturePatchService's execute()-over-command.NewDefaultCommandRegistry pattern -- no new mutation path introduced"
    - "SetSceneLayer's currentLayerRef pre-read + re-supply discipline (WR-01/WR-03) copied verbatim from PlaybackService.SetLayerEnabled/currentLayerRef"
    - "ListProgramming projects ShowState directly into a JSON-safe view (no registered read route exists), mirroring GetState/ListPatch/ShowSurface"

key-files:
  created:
    - internal/wails/svc_programming.go
    - internal/wails/svc_programming_test.go
    - frontend/src/components/SceneProgramming/SceneProgramming.tsx
    - frontend/src/components/SceneProgramming/SceneProgramming.module.css
  modified:
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/App.tsx
    - cmd/golc-desktop/main.go
    - .planning/REQUIREMENTS.md

key-decisions:
  - "Task-1 seeding fixtures reordered so seedProgrammingInstance's show.Save call runs BEFORE any CLI-route-backed create call in the same test -- seedProgrammingInstance saves a fresh ShowState directly and would otherwise silently overwrite prior scene/theme/motion/chase creations"
  - "ProgrammerSet binds only instance selectors + capability=value attribute pairs (no pool/group/fixture selectors) -- the documented simplified-subset boundary enough to stage a base-look/color RecordPreset call"
  - "REQUIREMENTS.md correction folds PLAY-01/02/06/07/08/09 from Pending to Complete and records that PLAY-04/05's completion is now genuinely satisfied (06-09 closed the MIDI-dispatch gap this plan's own verification report flagged); PLAY-10/11/12 left untouched"

patterns-established: []

requirements-completed: [PLAY-12]

coverage:
  - id: D1
    description: "ProgrammingService binds CreateScene/ActivateScene/SetSceneLayer/CreateTheme/CreateMotion/CreateChase/ProgrammerSet/RecordPreset/CreateBlend/ListProgramming over the existing scene/programming command routes"
    requirement: "PLAY-12"
    verification:
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceCreateAndListScene"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceCreateEachLookKind"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceSetEachLayerKind"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceActivateScene"
        status: pass
    human_judgment: false
  - id: D2
    description: "Disabling a scene layer preserves its previously assigned ref (WR-01/WR-03); a malformed or dangling layer ref, a duplicate scene/theme name, and an invalid bars value are rejected with the underlying route's own diagnostic, never a panic or silent mutation"
    requirement: "PLAY-12"
    verification:
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceDisableLayerPreservesRef"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceRejectsInvalidInputs"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_programming_test.go#TestProgrammingServiceEmptyAndCountStates"
        status: pass
    human_judgment: false
  - id: D3
    description: "SceneProgramming.tsx renders the scene list (per-layer enable+look picker), look lists, a base-look/attribute preset recording form, and a blend-preset form, with empty/loading/error/overflow states, and is mounted in App.tsx with ProgrammingService bound in cmd/golc-desktop/main.go"
    requirement: "PLAY-12"
    verification:
      - kind: other
        ref: "go build ./... && go vet ./... && cd frontend && npm run build (all green)"
        status: pass
    human_judgment: true
    rationale: "Live click-through (create scene, create each look kind, enable+point all four layers, activate, confirm empty/error states render per UI-SPEC copy) requires a running golc-desktop build against a real show; deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase, consistent with 06-10/06-11's identical deferral."
  - id: D4
    description: "REQUIREMENTS.md PLAY-01..09 status corrected to Complete in both the checklist and the Traceability status table, matching 06-VERIFICATION.md's Requirements Coverage determination; PLAY-10/11/12 left untouched; a dated footer note records the correction and the still-deferred end-of-phase UAT items"
    verification:
      - kind: other
        ref: "grep -cE '^\\| PLAY-0[1-9] \\| Phase 6 \\| Complete \\|' .planning/REQUIREMENTS.md == 9"
        status: pass
    human_judgment: false

# Metrics
duration: 55min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 12: Scene & Look Programming (PLAY-12 gap closure) Summary

**On-screen ProgrammingService binding + SceneProgramming React surface driving the existing scene/theme/chase/motion/programmer/preset/blend CLI routes, plus a REQUIREMENTS.md accuracy correction for PLAY-01..09.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-07-23T23:21:29Z
- **Tasks:** 4 (RED test, GREEN implementation, validation/polish, docs correction)
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- `internal/wails.ProgrammingService` binds `CreateScene`, `ActivateScene`, `SetSceneLayer`, `CreateTheme`, `CreateMotion`, `CreateChase`, `ProgrammerSet`, `RecordPreset`, `CreateBlend`, and `ListProgramming` over the already-implemented, already-tested `scene`/`theme`/`chase`/`motion`/`programmer`/`preset`/`blend` command routes -- no second scene/programming mutation implementation introduced.
- `SetSceneLayer` mirrors `PlaybackService.SetLayerEnabled`'s exact Ref-preserving pre-read discipline: disabling then re-enabling a scene layer, or pointing one layer kind while leaving another untouched, never discards a previously assigned base-look/color-theme/chase/motion reference.
- `SceneProgramming.tsx` renders the scene list (per-layer enable toggle + look picker for all four fixed layer kinds), inline create controls for scenes and every look kind, a minimal base-look/attribute preset recording flow (instance picker + `capability=value` attributes), a blend-preset form, and empty/loading/error/overflow states -- mounted in `App.tsx` alongside the existing `FixturePatch`/`ArtnetConfig`/`OperatorSurface` regions.
- `ProgrammingService` is bound in `cmd/golc-desktop/main.go`'s `Bind` list alongside the existing five services.
- `.planning/REQUIREMENTS.md`'s PLAY-01..09 status corrected to `Complete` in both the checklist and the Traceability status table (documentation-only, per 06-VERIFICATION.md), with a dated footer note recording the correction's basis and the still-deferred end-of-phase UAT items.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing end-to-end test (RED)** - `624c8d6` (test)
2. **Task 2: Thinnest end-to-end slice (GREEN)** - `09bf8fb` (feat)
3. **Task 3: Validation, empty/error/overflow states, simplified-subset boundary** - `1e1a7ff` (test)
4. **Task 4: REQUIREMENTS.md accuracy correction** - `dd453a9` (docs)

_TDD tasks 1-3 form the RED -> GREEN -> polish cycle for the PLAY-12 vertical slice; Task 4 is a separate docs-only correction executed after the slice landed, per plan instructions._

## Files Created/Modified

- `internal/wails/svc_programming.go` - `ProgrammingService`: scene/theme/chase/motion/programmer/preset/blend bindings + `ListProgramming`'s JSON-safe view projection
- `internal/wails/svc_programming_test.go` - `TestProgrammingService*` suite (create/list, create-each-look-kind, set-each-layer-kind, disable-preserves-ref, activate, empty/count states, invalid-input rejection)
- `frontend/src/components/SceneProgramming/SceneProgramming.tsx` - on-screen scene/look programming surface
- `frontend/src/components/SceneProgramming/SceneProgramming.module.css` - CSS Module (brand tokens, fixed-height scroll panels)
- `frontend/src/lib/wailsBridge.ts` - `ProgrammingServiceBinding` + `ProgLayerView`/`ProgSceneView`/`ProgLookView`/`ProgPresetView`/`ProgInstanceView`/`ProgrammingView` types + helper exports
- `frontend/src/App.tsx` - mounts `<SceneProgramming/>` alongside the existing feature regions
- `cmd/golc-desktop/main.go` - binds `ProgrammingService`
- `.planning/REQUIREMENTS.md` - PLAY-01..09 status correction (checklist + Traceability table + dated footer note)

## Decisions Made

- **Test-fixture ordering fix (not a plan deviation, a test-authoring correction within Task 1):** `seedProgrammingInstance` saves a fresh `ShowState` directly via `show.Save`, which would silently overwrite any scene/theme/motion/chase already appended through the CLI-route-backed `Create*` calls if seeded afterward. `TestProgrammingServiceCreateEachLookKind` and `TestProgrammingServiceSetEachLayerKind` seed the pool/deployment instance first, then create looks/scenes on top of it (Load-mutate-Save preserves prior state). Caught immediately by the first Task-2 test run, fixed before commit -- no separate deviation entry needed since this was corrected within the same RED-authoring pass, not a post-hoc bug fix against already-committed code.
- **ProgrammerSet's minimal selection grammar:** binds only `--instance` selectors (no `--pool`/`--group`/`--fixture`), matching the plan's documented simplified-subset boundary -- enough to stage a base-look/color preset recording without duplicating FixturePatch.tsx's own pool/deployment authoring surface.
- **REQUIREMENTS.md correction scope:** PLAY-01/02/06/07/08/09 flipped Pending->Complete; PLAY-03/04/05 (already checked, but prematurely per 06-VERIFICATION.md's traceability note) left checked since 06-09's dispatch fix now genuinely satisfies them; PLAY-10/11/12 explicitly untouched (owned by their own gap-closure plans' SUMMARYs).

## Deviations from Plan

None - plan executed exactly as written. (See "Decisions Made" above for one in-flight test-fixture ordering correction made during Task 1/2 authoring, before any commit -- not a post-commit deviation.)

## Issues Encountered

- Initial Task-2 test run failed two of the five Task-1 tests (`TestProgrammingServiceCreateEachLookKind`, `TestProgrammingServiceSetEachLayerKind`) because `seedProgrammingInstance`'s direct `show.Save` call ran after prior `Create*` calls in the same test, silently discarding them. Fixed by reordering both tests to seed first (see Decisions Made); all five Task-1 tests then passed on the next run.
- `frontend/node_modules` was not yet installed in this worktree; ran `npm install` once before the first `npm run build` (standard first-build step, not a deviation).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- This is the final wave-3 plan of Phase 6 (06-09/06-10/06-11 already landed in earlier waves). All Wave 1-3 gap-closure work for VERIFICATION.md's two FAILED truths (MIDI dispatch via 06-09; on-screen fixture/deployment/programming UI via 06-10/06-11/06-12) is now complete.
- Live end-of-phase UAT remains outstanding for every Wave-3/4 checkpoint script this plan and its predecessors deferred (06-05/06-06/06-07/06-08/06-10/06-11/06-12), per `workflow.human_verify_mode=end-of-phase` -- this SUMMARY records the click-through as `human_judgment: true` (coverage D3) rather than claiming it satisfied.
- REQUIREMENTS.md now accurately reflects PLAY-01..09 as Complete; PLAY-10/11/12 remain Pending pending their own end-of-phase UAT sign-off, consistent with the phase's overall gap-closure state.
- No blockers for the orchestrator's phase-level STATE.md/ROADMAP.md updates, which this worktree agent does not itself perform (owned centrally after merge, per dispatch instructions).

## Known Stubs

None - every mutation flows through the existing scene/programming command routes; no hardcoded empty/placeholder values reach the UI (empty arrays render the explicit empty state, not a stub).

## Threat Flags

None beyond what the plan's own `<threat_model>` already registers (T-06-34/T-06-35/T-06-36/T-06-SC) -- no new network endpoint, auth path, file-access pattern, or schema change at a trust boundary was introduced beyond the ShowState mutation surface those entries already cover.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*
