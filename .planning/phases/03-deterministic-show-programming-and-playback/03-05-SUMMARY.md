---
phase: 03-deterministic-show-programming-and-playback
plan: 05
subsystem: programming
tags: [go, cli, undo-redo, crud, uuid]

# Dependency graph
requires:
  - phase: 03-deterministic-show-programming-and-playback (03-02/03-03/03-04)
    provides: Theme/Preset/Chase/MotionPreset/Scene object types and their record/create CLI routes
provides:
  - programming.History session-only linear undo/redo stack (EditOp/EditKind)
  - Full PROG-07 CRUD CLI surface: theme rename/delete, preset rename/delete, chase update/reorder/duplicate/delete, motion rename/duplicate/delete, scene rename/duplicate/delete
affects: [phase-06-ui-and-playback-control, phase-07-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Session-only in-memory undo/redo stack (slice + cursor), never a show.State field, never touched by show.Save"
    - "parseDomainNameShowArgs/parseDomainRenameArgs shared generic arg parsers taking a domain error-code prefix, reused across theme/preset/chase/motion/scene delete/rename/duplicate routes"
    - "Duplicate mints a fresh identity via each type's existing New{Type} constructor rather than a bespoke clone path"

key-files:
  created:
    - internal/programming/history.go
    - internal/programming/history_test.go
    - internal/command/history_test.go
  modified:
    - internal/command/programming.go

key-decisions:
  - "scene duplicate always forces the copy's Active field to false (scene.NewScene's own default) regardless of the source scene's Active state, to preserve SCEN-04's single-active-scene invariant"
  - "chase update (not 'chase rename') is the chase type's rename-capable verb, supporting optional --name/--unit/--step-duration so at least one field must change per invocation"

requirements-completed: [PROG-07]

coverage:
  - id: D1
    description: "Session-only whole-session linear undo/redo history (programming.History): Record/Undo/Redo with redo-tail truncation, round-trip idempotency, and empty-boundary no-crash errors"
    requirement: "PROG-07"
    verification:
      - kind: unit
        ref: "internal/programming/history_test.go#TestHistoryRecordUndoRedoRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/programming/history_test.go#TestHistoryUndoEmptyBoundaryNoCrash"
        status: pass
      - kind: unit
        ref: "internal/programming/history_test.go#TestHistoryRedoNoTailBoundaryNoCrash"
        status: pass
      - kind: unit
        ref: "internal/programming/history_test.go#TestHistoryRecordTruncatesRedoTail"
        status: pass
      - kind: unit
        ref: "internal/programming/history_test.go#TestHistoryMixedObjectTypeSingleGlobalStack"
        status: pass
    human_judgment: false
  - id: D2
    description: "Full record/update/rename/reorder/duplicate/delete CLI surface across theme/preset/chase/motion/scene, persisting through show.Save"
    requirement: "PROG-07"
    verification:
      - kind: integration
        ref: "internal/command/history_test.go#TestHistoryRoutes"
        status: pass
    human_judgment: false
  - id: D3
    description: "CRUD verbs succeed against an object referenced by (or, for scene duplicate, being) the currently-active scene with no pause/detach/lock precondition (D-08)"
    requirement: "PROG-07"
    verification:
      - kind: integration
        ref: "internal/command/history_test.go#TestHistoryLiveActiveEdit"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-21
status: complete
---

# Phase 3 Plan 5: Programming Object CRUD and Session Undo/Redo History Summary

**Completed PROG-07: session-only linear undo/redo history (`programming.History`) plus the full record/update/rename/reorder/duplicate/delete CLI surface across theme/preset/chase/motion/scene.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-21T21:15:55-07:00 (worktree base commit)
- **Completed:** 2026-07-21T21:32:52-07:00
- **Tasks:** 2
- **Files modified:** 4 (2 created programming/, 1 created command/, 1 modified command/)

## Accomplishments
- `programming.History`: a single global `EditOp` slice + cursor (D-12), never a `show.State` field and never touched by `show.Save` (D-14); `Undo`/`Redo` round-trip to the identical op and never inspect scene-active status (D-13); empty-history/no-redo-tail boundaries return `GOLC_HISTORY_NOTHING_TO_UNDO`/`GOLC_HISTORY_NOTHING_TO_REDO` rather than crashing.
- Completed the PROG-07 CLI CRUD surface: `theme rename`/`theme delete`, `preset rename`/`preset delete`, `chase update`/`chase reorder`/`chase duplicate`/`chase delete`, `motion rename`/`motion duplicate`/`motion delete`, `scene rename`/`scene duplicate`/`scene delete` — 14 new routes, all following the established parse→Load→mutate→Save→Stdout shape.
- `chase reorder` permutes `Steps` deterministically via an `--order` index list, rejecting a non-permutation (wrong length, out-of-range, or repeated index) with `GOLC_CHASE_USAGE` before any mutation (T-03-05).
- Every delete route relies on `show.Save`'s existing whole-State validation to reject a delete that would dangle a scene layer's `Ref` (`GOLC_SCENE_LAYER_DANGLING_REFERENCE` inside `GOLC_SHOW_STATE_INVALID`, T-03-01) — no bespoke reference-check duplicated here.
- Verified D-08 directly: `TestHistoryLiveActiveEdit` exercises rename/reorder/duplicate/delete against an active scene's referenced objects (and duplicates the active scene itself) and confirms none of the new handlers read `scene.Scene.Active` before mutating.

## Task Commits

Each task was committed atomically:

1. **Task 1: Session-only linear undo/redo history (PROG-07, D-12/D-13/D-14)** - `56dc116` (feat)
2. **Task 2: Complete the record/update/rename/reorder/duplicate/delete CLI surface (PROG-07)** - `a9bfc6a` (feat)

**Plan metadata:** committed in the same worktree branch by the orchestrator after wave merge (per worktree isolation policy, this executor does not create the final metadata commit — see `<parallel_execution>` in the executor prompt).

## Files Created/Modified
- `internal/programming/history.go` - `programming.History`/`EditOp`/`EditKind`: single global undo/redo stack, session-only, never persisted
- `internal/programming/history_test.go` - Library-level TestHistory* suite: round-trip, boundaries, redo-tail truncation, mixed-object-type ordering
- `internal/command/programming.go` - Added 14 CRUD routes (theme/preset/chase/motion/scene rename/delete/update/reorder/duplicate) plus their arg parsers and handlers
- `internal/command/history_test.go` - `TestHistoryRoutes` (route-level CRUD contract) and `TestHistoryLiveActiveEdit` (D-08 live-edit-no-gate proof)

## Decisions Made
- **`scene duplicate` forces `Active: false` on the copy.** The plan's "produces a new object with a fresh ID and copied contents" phrasing would, taken literally, let duplicating an active scene create a second `Active: true` scene — violating SCEN-04's single-active-scene invariant (`scene.ValidateSingleActiveScene` would then reject the whole `show.Save`). `scene.NewScene`'s own `Active: false` default is used instead, and `TestHistoryLiveActiveEdit` locks this in by duplicating the currently-active scene and asserting only the original stays active.
- **`chase update` (not `chase rename`) is chase's identity-preserving name-change verb.** The plan's artifact list names this verb "chase update" rather than "chase rename" (unlike theme/preset/motion/scene, which all get a plain two-positional `rename`); it accepts optional `--name`/`--unit`/`--step-duration` so a caller can rename and/or adjust step timing in one call, rejecting an invocation with none of the three (`GOLC_CHASE_USAGE`) as a no-op.
- **Shared generic arg parsers** (`parseDomainNameShowArgs`, `parseDomainRenameArgs`) take the domain's own error-code prefix as a parameter, avoiding 5x duplicated `<name> --show <path>` / `<old> <new> --show <path>` parsing bodies while still emitting each domain's exact `GOLC_{DOMAIN}_USAGE` diagnostic.

## Deviations from Plan

None - plan executed exactly as written. The two design choices above (forcing scene-duplicate inactivity, and using "chase update" as the rename-capable verb) were made within the plan's own stated task action ("chase reorder accepts an --order index permutation...", "Reuse each type's Rename{Type} helper") rather than departing from it — no Rule 1-4 auto-fix was needed for either.

## Issues Encountered
- **Pre-existing, unrelated test failure:** `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` (`internal/trace/catalog`) fails on a full `go test ./...` with `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`. This is unrelated to this plan's files (`internal/programming`, `internal/command`) and has been logged for 03-01/03-03/03-04 already; a 03-05 entry was appended to `deferred-items.md` following the same precedent. Every package this plan actually touches (`internal/programming/...`, `internal/command/...`, `internal/show/...`) passes independently, plus a full `go build ./...` and `go vet ./...` are clean.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- PROG-07 is now fully delivered: an author can record/update/rename/reorder/duplicate/delete every programming object type, and a session-only linear undo/redo history exists as a library ready for Phase 6's interactive UI session to wire up (this plan intentionally does not wire history into the stateless CLI — a fresh CLI invocation has no cross-invocation session to carry an undo stack across, per D-14).
- This is the last plan of Phase 3's programming/CRUD surface; remaining Phase 3 work (per the roadmap) moves to the deterministic playback engine/compiler.
- `deferred-items.md`'s `TestScopeLinearMap`/`linear-map.json` drift remains open for a future triage pass — not blocking for this plan or phase.

---
*Phase: 03-deterministic-show-programming-and-playback*
*Completed: 2026-07-21*

## Self-Check: PASSED

- FOUND: internal/programming/history.go
- FOUND: internal/programming/history_test.go
- FOUND: internal/command/history_test.go
- FOUND: .planning/phases/03-deterministic-show-programming-and-playback/03-05-SUMMARY.md
- FOUND commit: 56dc116 (Task 1)
- FOUND commit: a9bfc6a (Task 2)
