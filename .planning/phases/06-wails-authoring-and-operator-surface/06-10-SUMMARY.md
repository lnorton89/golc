---
phase: 06-wails-authoring-and-operator-surface
plan: 10
subsystem: ui
tags: [wails, react, go, pool, deployment, fixture-patch, css-modules]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments
    provides: "pool/deployment domain model, impact-plan engine (internal/pool), and the pool/deployment CLI routes (internal/command/pool.go, internal/command/deployment.go)"
  - phase: 06-wails-authoring-and-operator-surface (06-04, 06-07)
    provides: "Wails Go-host scaffold (internal/wails, cmd/golc-desktop) and the SurfaceService/OperatorSurface UI-binding pattern this plan mirrors"
provides:
  - "internal/wails.FixturePatchService: CreatePool, AddPoolMemberPreview, RemovePoolMemberPreview, ApplyPatch, CreateDeployment, ActivateDeployment, ListPatch"
  - "frontend FixturePatch component (pool/deployment on-screen surface) with a preview-then-apply impact-review flow"
  - "wailsBridge.ts FixturePatchService ambient binding declaration"
affects: ["06-wails-authoring-and-operator-surface end-of-phase UAT", "VERIFICATION.md Gap B[0] closure"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "UI-binding-only Wails service: FixturePatchService forwards every mutation to an already-registered CLI route via command.NewDefaultCommandRegistry, never re-implementing pool/deployment mutation logic (mirrors SurfaceService)."
    - "In-memory impact-plan cache keyed by plan_id inside a Wails service, so a preview-then-apply frontend flow never round-trips plan bytes through the UI layer; ApplyPatch materializes the cached plan to a throwaway temp file only at apply time."

key-files:
  created:
    - internal/wails/svc_fixturepatch.go
    - internal/wails/svc_fixturepatch_test.go
    - frontend/src/components/FixturePatch/FixturePatch.tsx
    - frontend/src/components/FixturePatch/FixturePatch.module.css
  modified:
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/App.tsx
    - cmd/golc-desktop/main.go

key-decisions:
  - "FixturePatchService caches each previewed pool.ImpactPlan in memory by its own plan_id (never persisted to the ShowState) and only materializes it to a throwaway temp file at ApplyPatch time -- avoids inventing a second on-disk plan-file convention for the frontend while still driving the exact same 'pool apply {plan-file} --plan-id <id>' route a CLI invocation would use."
  - "Add-fixture control accepts a raw <stable_key>/<content_hash>/<mode> triple (per 06-10-PLAN.md's flagged assumption) rather than a rich fixture picker, since internal/command/fixture.go exposes no structured fixture-list read; values are expected to come from 'fixture inspect' output."
  - "Pool members display fixture identity only (no per-member Mode) since Mode is not a PoolMember field -- it is recorded per deployment Instance at apply time; Mode/Universe/Address are shown exclusively in the deployment/instance list, matching the domain model exactly."

requirements-completed: [PLAY-10]

coverage:
  - id: D1
    description: "A show author can create a logical fixture pool through on-screen controls (bound to 'pool create') and it appears in the on-screen pool list (read from show.Load)."
    requirement: "PLAY-10"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceCreateAndListPool"
        status: pass
    human_judgment: false
  - id: D2
    description: "A show author can assign a fixture to a pool at a concrete mode through on-screen controls, first seeing the deterministic impact preview (each affected instance's system-computed proposed_universe/proposed_address) and then committing it via Apply; review-before-apply is preserved on screen."
    requirement: "PLAY-10"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceAddMemberPreviewThenApply"
        status: pass
    human_judgment: true
    rationale: "The Go-side preview/apply round trip is unit-proven, but the on-screen rendering of the impact-preview panel (FixturePatch.tsx) and the actual click-through against a running golc-desktop build is a visual/interaction check queued for end-of-phase UAT (workflow.human_verify_mode=end-of-phase), not something a unit test can confirm."
  - id: D3
    description: "Each deployment's fixture instances display their assigned mode/universe/address (read from show.Load's persisted deployment.Instance fields), so PLAY-10's addressing is visible on screen rather than CLI-only."
    requirement: "PLAY-10"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceAddMemberPreviewThenApply"
        status: pass
    human_judgment: true
    rationale: "ListPatch's projection is unit-proven, but the actual on-screen rendering of instance mode/universe/address in FixturePatch.tsx is a visual check queued for end-of-phase UAT."
  - id: D4
    description: "A show author can create a deployment and mark exactly one active through on-screen controls (bound to 'deployment create' / 'deployment activate')."
    requirement: "PLAY-10"
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceCreateAndActivateDeployment"
        status: pass
    human_judgment: false
  - id: D5
    description: "The pool list renders an explicit empty state when no pools exist, and singular vs plural counts read correctly (zero-one-many)."
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceEmptyAndCountStates"
        status: pass
    human_judgment: true
    rationale: "Backend projection counts are unit-proven; the actual rendered empty-state copy and singular/plural row text in FixturePatch.tsx is a visual check queued for end-of-phase UAT."
  - id: D6
    description: "A malformed member triple never panics, returning the pool route's own GOLC_POOL_APPLY_USAGE diagnostic instead."
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceRejectsMalformedMember"
        status: pass
    human_judgment: false
  - id: D7
    description: "A stale/unknown plan-id apply is rejected (POOL-08 freshness/integrity gate), never a silent success."
    verification:
      - kind: unit
        ref: "internal/wails/svc_fixturepatch_test.go#TestFixturePatchServiceApplyStalePlanRejected"
        status: pass
    human_judgment: true
    rationale: "The Go-side rejection is unit-proven; whether the rejection renders legibly in FixturePatch.tsx's error banner during an actual click-through is a visual check queued for end-of-phase UAT."

# Metrics
duration: 40min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 10: FixturePatchService and On-Screen Fixture Patch Summary

**Wails FixturePatchService binds the existing pool/deployment CLI routes to a new FixturePatch React component, closing VERIFICATION.md Gap B[0] with a preview-then-apply on-screen flow that surfaces system-computed universe/address before commit.**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-07-23T22:27:29Z
- **Tasks:** 3 (TDD: RED / GREEN / polish)
- **Files modified:** 7 (4 created, 3 modified)

## Accomplishments

- `internal/wails.FixturePatchService` (CreatePool, AddPoolMemberPreview, RemovePoolMemberPreview, ApplyPatch, CreateDeployment, ActivateDeployment, ListPatch) drives the exact `pool`/`deployment` CLI routes internal/command/pool.go and internal/command/deployment.go already implement -- no second mutation path.
- A show author can now create a fixture pool, add a fixture at a mode with an on-screen, non-committing impact preview (showing each affected deployment instance's system-computed `proposed_universe`/`proposed_address`), apply it, and create/activate a deployment -- all through `frontend/src/components/FixturePatch/FixturePatch.tsx`.
- `ListPatch` reads `show.Load` directly (never the `instance_count`-only `show inspect --json` view) so every deployment instance's persisted mode/universe/address is genuinely visible on screen, satisfying PLAY-10's "universes, addresses" clause via display of the backend's own computed values.
- Pool/deployment/member/preview lists scroll within fixed-height panels (overflow backstop); explicit empty states and singular/plural counts are covered by both unit tests and rendered copy.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing end-to-end FixturePatchService tests** - `00e560b` (test)
2. **Task 2: FixturePatchService + FixturePatch component + bridge + mount + bind** - `cd996bc` (feat)
3. **Task 3: Impact-plan warnings/errors rendering + state-coverage documentation** - `2f7568b` (test)

_Note: this is a `tdd="true"` plan; Task 1 is RED, Task 2 is GREEN, Task 3 is the polish/validation pass._

## Files Created/Modified

- `internal/wails/svc_fixturepatch.go` - FixturePatchService binding: pool/deployment CLI-route forwarding, in-memory impact-plan cache, ListPatch's show.Load-direct projection
- `internal/wails/svc_fixturepatch_test.go` - Six TestFixturePatchService* tests (create/list, preview/apply, deployment create/activate, malformed-input rejection, empty/count states, stale-plan rejection)
- `frontend/src/components/FixturePatch/FixturePatch.tsx` - On-screen pool/deployment surface with preview-then-apply flow
- `frontend/src/components/FixturePatch/FixturePatch.module.css` - CSS Module for the fixture-patch feature, reusing GOLC brand tokens
- `frontend/src/lib/wailsBridge.ts` - Added `FixturePatchServiceBinding` interface and `window.go.wails.FixturePatchService` ambient property
- `frontend/src/App.tsx` - Mounted `<FixturePatch />` in the feature region between OperatorSurface and MidiPanel
- `cmd/golc-desktop/main.go` - Constructed and bound `FixturePatchService` in `options.App{Bind: [...]}`

## Decisions Made

- FixturePatchService caches each previewed `pool.ImpactPlan` in memory by `plan_id` and only writes it to a throwaway temp file at `ApplyPatch` time -- avoids a second on-disk plan-file convention while still driving the unmodified `pool apply {plan-file} --plan-id <id>` route.
- The add-fixture control accepts a raw stable-key/content-hash/mode triple (06-10-PLAN.md's flagged assumption) rather than a rich fixture picker, since `internal/command/fixture.go` has no structured fixture-list read yet.
- Pool members display fixture identity only; Mode/Universe/Address render exclusively on deployment instances, matching the domain model (`PoolMember` carries no `Mode` field -- only `deployment.Instance` does).

## Deviations from Plan

**1. [Plan-structure consolidation, non-functional] All six `TestFixturePatchService*` tests were written in Task 1's RED commit rather than splitting four (Task 1) / two (Task 3) across commits**
- **Found during:** Task 1 (writing the failing test file)
- **Issue:** 06-10-PLAN.md's Task 1 acceptance criteria calls for four tests (Create/List, Preview/Apply, Create/Activate, Malformed) and Task 3 calls for two more (Empty/Count states, Stale-plan rejection). Both sets share the exact same `seedFixturePatchShowState` fixture and package, so writing all six together in one RED pass was more efficient and avoided a redundant second edit-and-rerun cycle against the same test file.
- **Fix:** All six tests were authored and committed in Task 1's `test(06-10): ...` commit (`00e560b`); they remained correctly RED (failing to build) at that commit since `svc_fixturepatch.go` did not exist yet, and all six turned GREEN together at Task 2's commit (`cd996bc`).
- **Files modified:** `internal/wails/svc_fixturepatch_test.go`
- **Verification:** `go test ./internal/wails/... -run TestFixturePatchService` failed to build at the Task 1 commit (RED) and passed with all six tests green at the Task 2 commit (GREEN) -- TDD gate sequence (test commit before feat commit) is intact; only the internal test/task boundary shifted.
- **Committed in:** `00e560b` (Task 1), confirmed green in `cd996bc` (Task 2)

---

**Total deviations:** 1 (plan-structure consolidation, no functional impact)
**Impact on plan:** No scope creep; RED/GREEN TDD gate order preserved exactly (test commit precedes feat commit). Task 3's own commit (`2f7568b`) instead focused on genuine additional polish (impact-plan warnings/errors rendering, Apply-disabled-on-error guard, and state-coverage documentation) since its assigned tests were already passing.

## Issues Encountered

- `go build ./...` initially failed with `pattern all:frontend/dist: no matching files found` because `cmd/golc-desktop`'s `//go:embed` target had never been populated in this worktree. Resolved by running `npm ci` (frontend/node_modules was absent) followed by `npm run build`, which Vite redirects to `cmd/golc-desktop/frontend/dist` per `frontend/vite.config.ts`'s documented convention -- not a plan deviation, just first-build environment setup.

## Human Verification Queued (end-of-phase UAT)

Per `workflow.human_verify_mode=end-of-phase`, Task 3's `<human-check>` is recorded here rather than treated as a mid-execution checkpoint:

> Launch golc-desktop against a show whose deployment already references a pool, create a pool, add a fixture at a mode, confirm the impact preview shows each affected instance's system-computed universe/address before Apply, apply it, and create+activate a deployment -- the pool list, the deployment active-state, and each instance's mode/universe/address should update on screen; empty/error states should render legibly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- FixturePatchService and FixturePatch.tsx are fully wired (bound in `cmd/golc-desktop/main.go`, mounted in `App.tsx`) and ready for the end-of-phase UAT pass alongside the rest of Phase 6's on-screen surfaces.
- No blockers for sibling plan 06-09 (MIDI dispatch + PLAY-10/11/12 on-screen UI) -- file sets do not overlap.

## Self-Check: PASSED

All created files verified present on disk (internal/wails/svc_fixturepatch.go, internal/wails/svc_fixturepatch_test.go, frontend/src/components/FixturePatch/FixturePatch.tsx, FixturePatch.module.css) and all three task commits (00e560b, cd996bc, 2f7568b) confirmed present via `git log --oneline`.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*
