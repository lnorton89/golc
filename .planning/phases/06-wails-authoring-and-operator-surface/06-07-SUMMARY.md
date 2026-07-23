---
phase: 06-wails-authoring-and-operator-surface
plan: 07
subsystem: ui
tags: [wails, go, react, operator-surface, midi-adjacent, authorize]

# Dependency graph
requires:
  - phase: 06-01
    provides: internal/operatorsurface model + operatorsurface CLI routes (create/list/assign/unassign/show) + command.Authorize server-side enforcement
  - phase: 06-04
    provides: SurfaceService binding stub, Wails scaffold, React shell with OperatorSurface region stub
provides:
  - internal/wails/svc_surface.go filled with real SurfaceService methods (CreateSurface/ListSurfaces/AssignItem/UnassignItem/ShowSurface/RemoveSurface/AuthorizeControl)
  - "operatorsurface remove" CLI route (internal/command/operatorsurface.go) backing the destructive Remove-Operator-Surface UI action
  - frontend/src/components/OperatorSurface/{OperatorSurface,SurfaceList,AssignmentToggle}.tsx -- in-place per-item assignment authoring UI + visible-but-locked operator preview
affects: [06-08-midi-learn, 06-VALIDATION]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Wails-bound Go methods execute mutations via command.NewDefaultCommandRegistry().Execute (Load -> mutate -> Save), the identical path a CLI invocation takes -- never a second mutation implementation for the GUI"
    - "Wails-bound read methods (ListSurfaces/ShowSurface) Load the ShowState directly and project it into a JSON-safe camelCase view shape, since the CLI's own text Result isn't structured data"
    - "SurfaceService.AuthorizeControl is the server-side visible-but-locked enforcement point (D-04/ASVS V4): resolves a control reference against the loaded ShowState and calls command.Authorize before any operator-mode action may proceed -- frontend disabled/locked rendering is a UI affordance only, never the enforcement"
    - "Frontend accesses Wails-bound Go methods via window.go.wails.<Service>.<Method> directly (no wailsjs codegen run in this environment); all binding calls for a feature live in that feature's single root component, with child components staying purely presentational"

key-files:
  created:
    - internal/wails/svc_surface_test.go
    - frontend/src/components/OperatorSurface/SurfaceList.tsx
    - frontend/src/components/OperatorSurface/AssignmentToggle.tsx
    - frontend/src/components/OperatorSurface/OperatorSurface.module.css
  modified:
    - internal/wails/svc_surface.go
    - internal/command/operatorsurface.go
    - internal/command/operatorsurface_test.go
    - cmd/golc-desktop/main.go
    - frontend/src/components/OperatorSurface/OperatorSurface.tsx

key-decisions:
  - "Added an 'operatorsurface remove' CLI route (not present from 06-01) since the plan's RemoveSurface binding needs a matching registry route to execute against, and the destructive Remove-Operator-Surface UI copy (06-UI-SPEC.md) needs real backing functionality (Rule 2 deviation)"
  - "Added a 7th SurfaceService method, AuthorizeControl, beyond the plan's literal 6-method list, to satisfy the plan's own acceptance criteria ('a test proves an operator-mode action against an unassigned control returns GOLC_OPERATORSURFACE_LOCKED') and the artifact/key_links description of svc_surface.go calling command.Authorize server-side"
  - "ShowSurface returns every assignable control in the show (all scenes/layers/grand master/group masters/safety controls), each marked assigned or not, rather than only the surface's own assigned refs -- OperatorSurface.tsx's D-04 visible-but-locked renderer needs the complete set to render unassigned items as visible-but-locked, never only the assigned subset"
  - "No wailsjs codegen module exists in this environment (wails CLI was never run to generate it); OperatorSurface.tsx accesses window.go.wails.SurfaceService directly with a hand-written TypeScript type augmentation instead of importing a generated bindings module"
  - "All SurfaceService calls live in OperatorSurface.tsx alone; SurfaceList.tsx and AssignmentToggle.tsx are purely presentational (props/callbacks only), keeping the frontend change to exactly the 4 files the plan declared"
  - "OperatorSurface.tsx renders one combined component with an author/operate mode toggle ('Preview as Operator') rather than two separate screens, since no separate scene/layer authoring canvas exists yet elsewhere in this codebase for AssignmentToggle to be embedded into"
  - "NewSurfaceService's signature was extended from (pipeName) to (pipeName, root, showPath) since AssignItem/ListSurfaces/etc. need to Load/Save the ShowState directly; cmd/golc-desktop/main.go's construction call site was updated to match (Rule 3 deviation -- blocking, required for the service to function)"

patterns-established:
  - "ControlRefInput/ControlRefView: a JSON-safe, camelCase discriminated-union shape mirroring operatorsurface.ControlRef's Kind+one-populated-field design, used as the single control-identity representation crossing the Wails Go<->JS boundary"

requirements-completed: [PLAY-03, PLAY-07]

coverage:
  - id: D1
    description: "SurfaceService bindings (CreateSurface/ListSurfaces/AssignItem/UnassignItem/ShowSurface/RemoveSurface) execute the matching operatorsurface CLI route via the command registry; assign is idempotent; ShowSurface reflects membership over the full control set"
    requirement: "PLAY-03"
    verification:
      - kind: unit
        ref: "internal/wails/svc_surface_test.go#TestSurfaceServiceCreateListAssignShowUnassignRemoveRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_surface_test.go#TestSurfaceServiceAssignSceneAndLayer"
        status: pass
      - kind: unit
        ref: "internal/command/operatorsurface_test.go#TestOperatorSurfaceRemove"
        status: pass
    human_judgment: false
  - id: D2
    description: "AuthorizeControl is the server-side visible-but-locked enforcement point (D-04/ASVS V4): an operator-mode action against a control not assigned to the surface is rejected with GOLC_OPERATORSURFACE_LOCKED, and accepted once assigned"
    requirement: "PLAY-03"
    verification:
      - kind: unit
        ref: "internal/wails/svc_surface_test.go#TestSurfaceServiceAuthorizeControlRejectsUnassignedControl"
        status: pass
    human_judgment: false
  - id: D3
    description: "SurfaceList/AssignmentToggle/OperatorSurface render multiple named surfaces (D-02), in-place per-item assignment toggles with no bulk/category control (D-01/D-03), and a visible-but-locked operator preview (assigned full-opacity + Signal Blue, unassigned reduced-opacity + non-interactive, never hidden, D-04); frontend build succeeds"
    requirement: "PLAY-03"
    verification:
      - kind: automated_ui
        ref: "cd frontend && npm run build (tsc --noEmit && vite build) -- exit 0"
        status: pass
    human_judgment: true
    rationale: "Visual/interaction correctness (Signal Blue selection indicator rendering, opacity/disabled treatment, name truncation with tooltip, scroll-panel overflow behavior, and the actual on-screen author/operate toggle flow) requires a human running the real golc-desktop app -- exactly what this plan's Task 3 checkpoint (type=checkpoint:human-verify, gate=blocking) specifies. Per workflow.human_verify_mode=end-of-phase (.planning/config.json), this worktree-isolated executor defers that live verification to the phase's end-of-phase UAT pass rather than halting mid-flight; see 'Checkpoint Verification' below for the exact steps to run."

duration: ~20min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 7: Operator-Surface Wails Bindings and Authoring UI Summary

**SurfaceService (Go) executes operatorsurface CRUD via the command registry and enforces D-04's visible-but-locked lock server-side through a new AuthorizeControl method; SurfaceList/AssignmentToggle/OperatorSurface (React) let an author build multiple named surfaces with in-place per-item assignment toggles and preview the resulting visible-but-locked operator view.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-23
- **Tasks:** 2 automatable tasks executed (Task 3 is a `checkpoint:human-verify` gate deferred to end-of-phase UAT, see below)
- **Files modified:** 9 (5 created, 4 modified)

## Accomplishments

- `internal/wails/svc_surface.go`: `SurfaceService` filled with `CreateSurface`, `ListSurfaces`, `AssignItem`, `UnassignItem`, `ShowSurface`, `RemoveSurface`, and `AuthorizeControl` -- every mutation runs through `command.NewDefaultCommandRegistry().Execute` (the identical Load -> mutate -> Save path a `golc-project.exe operatorsurface ...` CLI call takes), and reads (`ListSurfaces`/`ShowSurface`) project the loaded `show.State` into camelCase JSON view shapes for the frontend.
- `AuthorizeControl` is the server-side visible-but-locked enforcement point (D-04/ASVS V4, threat T-06-18): resolves a control reference against the loaded ShowState and calls `command.Authorize` before any operator-mode action may proceed; a control not currently assigned to the surface is rejected with `GOLC_OPERATORSURFACE_LOCKED`, never trusted from frontend-disabled rendering alone.
- New `"operatorsurface remove"` CLI route (`internal/command/operatorsurface.go`) -- no such route existed from 06-01, and the plan's `RemoveSurface` binding + the UI-SPEC's destructive "Remove Operator Surface" confirm copy both need real backing functionality (T-06-20).
- `SurfaceList.tsx`: multiple independently named operator surfaces (D-02) -- "Create Operator Surface" CTA, the exact "No operator surfaces yet" empty state, populated rows (name, assigned scene/layer/master count, MIDI-mapping count) with zero-one-many pluralization, Signal Blue selected row, ellipsis name truncation with hover tooltip, and a fixed-height scroll panel.
- `AssignmentToggle.tsx`: the in-place, per-item "add to this operator surface" checkbox (D-01/D-03) -- no bulk/category assign control exists anywhere in the component tree; membership is idempotent both client-side (checkbox reflects real state, refetched after every toggle) and server-side (Assign*/Unassign* mutators).
- `OperatorSurface.tsx`: composes both, owns every `SurfaceService` call (`window.go.wails.SurfaceService`), and renders a "Preview as Operator" mode implementing D-04's visible-but-locked view -- every control in the show always renders, assigned or not; assigned is full opacity with the Signal Blue selection indicator, unassigned is reduced opacity and non-interactive.

## Task Commits

Each task was committed atomically:

1. **Task 1: SurfaceService bindings with server-side visible-but-locked authorization** - `55a8a93` (feat)
2. **Task 2: SurfaceList + in-place AssignmentToggle + visible-but-locked OperatorSurface renderer** - `cdb4a69` (feat)

_Note: Task 3 is a `checkpoint:human-verify` gate (type=checkpoint:human-verify, gate=blocking) -- deferred to end-of-phase UAT per `workflow.human_verify_mode=end-of-phase`; no code change, see "Checkpoint Verification" below._

## Files Created/Modified

- `internal/wails/svc_surface.go` - `SurfaceService`: CreateSurface/ListSurfaces/AssignItem/UnassignItem/ShowSurface/RemoveSurface/AuthorizeControl, `ControlRefInput`/`ControlRefView`/`SurfaceSummary`/`SurfaceDetail` JSON view types, selector/resolver helpers
- `internal/wails/svc_surface_test.go` - CRUD round-trip, idempotent assign, scene/layer selector resolution, AuthorizeControl rejects-then-accepts tests
- `internal/command/operatorsurface.go` - new `"operatorsurface remove"` route + `runOperatorSurfaceRemove` handler
- `internal/command/operatorsurface_test.go` - `TestOperatorSurfaceRemove` (deletes surface, rejects unknown surface)
- `cmd/golc-desktop/main.go` - `NewSurfaceService` call site updated to pass `cfg.ProjectRoot`/`cfg.ShowPath`
- `frontend/src/components/OperatorSurface/OperatorSurface.tsx` - root component: Wails binding calls, author/operate mode, visible-but-locked rendering
- `frontend/src/components/OperatorSurface/SurfaceList.tsx` - surface list (empty/populated/zero-one-many/long-text states)
- `frontend/src/components/OperatorSurface/AssignmentToggle.tsx` - in-place per-item assignment checkbox
- `frontend/src/components/OperatorSurface/OperatorSurface.module.css` - shared stylesheet for all three components

## Decisions Made

- Added the `"operatorsurface remove"` CLI route since none existed from 06-01, needed for `RemoveSurface` to execute via the registry and for the UI-SPEC's destructive-confirm action to have real backing behavior.
- Added `AuthorizeControl` as a 7th `SurfaceService` method (beyond the plan's literal 6-name list) to satisfy the plan's own acceptance criteria requiring a test that proves an operator-mode action against an unassigned control is rejected server-side.
- `ShowSurface` returns the full assignable-control universe (every scene, its four fixed layers, the grand master, every group master, and the three fixed safety controls) marked assigned or not, rather than only the surface's own assigned refs, so `OperatorSurface.tsx` can render D-04's visible-but-locked view without depending on a separate scenes/groups-listing service that doesn't exist yet in this wave.
- No `wails` CLI codegen was run in this environment, so `OperatorSurface.tsx` accesses `window.go.wails.SurfaceService` directly via a hand-written TypeScript `declare global` augmentation instead of importing a generated `wailsjs/go/...` module.
- Kept every `SurfaceService` call inside `OperatorSurface.tsx` alone (SurfaceList/AssignmentToggle stay presentational) and put all CSS in the single declared `OperatorSurface.module.css`, so the frontend change stays within exactly the 4 files `06-07-PLAN.md` names.
- `OperatorSurface.tsx` is one component with an author/operate mode toggle rather than two separate screens, since no separate scene/layer authoring canvas exists elsewhere in this codebase yet for `AssignmentToggle` to be embedded into independently.
- `NewSurfaceService`'s constructor signature was extended from `(pipeName)` to `(pipeName, root, showPath)` since every method needs to `Load`/`Save` the `ShowState`; `cmd/golc-desktop/main.go`'s construction call site was updated to match.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added the `"operatorsurface remove"` CLI route**
- **Found during:** Task 1 (`internal/wails/svc_surface.go`)
- **Issue:** The plan requires a `RemoveSurface(surfaceName)` binding executing "the matching operatorsurface command via the registry," but 06-01 only implemented create/list/assign/unassign/show -- no remove/delete route existed to execute against. The UI-SPEC's "Remove Operator Surface" destructive-confirm copy (T-06-20) also has no backing functionality without it.
- **Fix:** Added `"operatorsurface remove"` to `internal/command/operatorsurface.go` (parse `--surface`/`--show`, reject an unknown surface with `GOLC_OPERATORSURFACE_NOT_FOUND`, filter the surface out, save) plus `TestOperatorSurfaceRemove`.
- **Files modified:** internal/command/operatorsurface.go, internal/command/operatorsurface_test.go
- **Verification:** `go test ./internal/command/... ./internal/wails/...` passes; `go build ./...` succeeds.
- **Committed in:** 55a8a93 (Task 1 commit)

**2. [Rule 3 - Blocking] Extended `NewSurfaceService`'s constructor and updated `cmd/golc-desktop/main.go`**
- **Found during:** Task 1 (`internal/wails/svc_surface.go`)
- **Issue:** The 06-04 stub's `NewSurfaceService(pipeName string)` had no `root`/`showPath`, but every real method needs to `show.Load`/`show.Save` the ShowState -- without them the service cannot function at all.
- **Fix:** Extended the constructor to `NewSurfaceService(pipeName, root, showPath string)` and updated `cmd/golc-desktop/main.go`'s single construction call site to pass `cfg.ProjectRoot`/`cfg.ShowPath` (fields `Config` already carried for `App`).
- **Files modified:** internal/wails/svc_surface.go, cmd/golc-desktop/main.go
- **Verification:** `go build ./...` succeeds (including `cmd/golc-desktop`, which embeds the built frontend).
- **Committed in:** 55a8a93 (Task 1 commit)

**3. [Rule 2 - Missing Critical] Added `AuthorizeControl`, a 7th `SurfaceService` method beyond the plan's literal list**
- **Found during:** Task 1 (`internal/wails/svc_surface.go`)
- **Issue:** The plan's acceptance criteria require "a test proves an operator-mode action against an unassigned control returns GOLC_OPERATORSURFACE_LOCKED (server-side)," and the artifact/key_links sections describe `svc_surface.go` calling `command.Authorize` server-side -- but none of the plan's 6 named methods (Create/List/Assign/Unassign/Show/Remove) actually models an operator-mode dispatch action that would call `Authorize`.
- **Fix:** Added `AuthorizeControl(surfaceName, controlRef)`, which resolves the control against the loaded ShowState and calls `command.Authorize`, directly satisfying the acceptance criterion and giving 06-05/06-06's future operator-mode dispatch a concrete call to reuse.
- **Files modified:** internal/wails/svc_surface.go, internal/wails/svc_surface_test.go
- **Verification:** `TestSurfaceServiceAuthorizeControlRejectsUnassignedControl` passes.
- **Committed in:** 55a8a93 (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (2 missing-critical, 1 blocking)
**Impact on plan:** All three were necessary for the plan's own stated methods/acceptance criteria to be implementable and functional. No unrelated scope creep.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None - no external service configuration required.

## Checkpoint Verification

**Task 3 (`checkpoint:human-verify`, `gate="blocking"`) — PENDING, deferred to end-of-phase.**

`.planning/config.json` sets `workflow.human_verify_mode: "end-of-phase"` for this project. Per that mode (documented in `references/checkpoints.md`), a `checkpoint:human-verify` task's live verification is deferred rather than halting this worktree-isolated executor mid-flight — especially since this plan runs in a disposable parallel worktree with no continuation agent to resume into after a pause. The checkpoint's content is preserved here (and in the `coverage` D3 entry's `human_judgment: true` + `rationale`) for the phase's end-of-phase UAT pass:

**What was built:** The operator-surface authoring UI -- surface list, in-place assignment toggles, and the visible-but-locked renderer.

**How to verify:**
1. Run the desktop app with a show that has scenes/layers/masters.
2. Create two named operator surfaces; confirm both appear in the list with counts and that selecting one highlights it in Signal Blue.
3. In the authoring view, toggle "add to this operator surface" in place on a few individual scenes/layers/masters; confirm no bulk/"assign all" control exists (D-03).
4. Switch to the operator view of one surface ("Preview as Operator"); confirm assigned items are interactive and unassigned items are visible-but-locked (grayed, disabled), NOT hidden.
5. Attempt to act on a locked item (e.g. via a crafted call if possible) and confirm the server rejects it (Authorize) -- the lock is not cosmetic. (`SurfaceService.AuthorizeControl` and the underlying `command.Authorize` are already unit-tested to reject an unassigned control server-side; this step confirms the same behavior end-to-end through the running app.)
6. Give a surface a long name; confirm truncation + tooltip.

**Resume signal:** Type "approved" if multiple surfaces, in-place per-item assignment, and visible-but-locked (server-enforced) all behave as specified; otherwise describe issues.

## Next Phase Readiness

- `SurfaceService` is a stable, fully Go-unit-tested contract: `AuthorizeControl` is ready for 06-05 (SafetyService) and 06-06 (PlaybackService) to call directly before any operator-mode dispatch action, without reimplementing the membership check.
- `ControlRefInput`/`ControlRefView`'s JSON shape is the established pattern for any future Wails-bound control-identity payload (e.g. 06-08's MIDI mapping target selection can reuse the identical selector grammar).
- The Task 3 checkpoint (live desktop verification of multiple surfaces, in-place assignment, and server-enforced visible-but-locked) remains open and should be exercised during the phase's end-of-phase UAT pass, alongside 06-04's already-approved Task 3 checkpoint and any other phase-6 plans carrying deferred human-verify items.
- `internal/command/operatorsurface.go` now has 6 self-registered routes (create/list/assign/unassign/show/remove); `06-VALIDATION.md`'s `TBD` cells for this plan's coverage should be backfilled with this SUMMARY's actual task/commit references during the phase's verifier pass (per 06-01-SUMMARY.md's own note, still outstanding).

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*
