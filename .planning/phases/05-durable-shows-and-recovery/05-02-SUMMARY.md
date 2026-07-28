---
phase: 05-durable-shows-and-recovery
plan: 02
subsystem: storage
tags: [sqlite, cli, recovery, show-state, database-sql]

# Dependency graph
requires:
  - phase: 05-durable-shows-and-recovery (Plan 01)
    provides: SQLite-backed show.Load/show.Save/openStore/schema.go (show_meta/show_state/recovery_points tables, D-01-D-06)
provides:
  - internal/show.DetectRecoveryPoints/DiscardRecoveryPoints/AcceptRecoveryPoint (read side of SHOW-04)
  - internal/command "show open"/"show save"/"show save-as" CLI routes on the existing "show" scope
  - Interrupted-session recovery offer wired into "show open" (--accept-recovery/--discard-recovery)
affects: [05-03-schema-migration, 05-04-integrity-diagnostics-and-export, 05-05-golc-file-format-hardening]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Recovery detection is a pure read: DetectRecoveryPoints/DiscardRecoveryPoints share one offeredRecoveryRevision(db) threshold (revision > last clean show_meta.revision) so 'what is offered' and 'what gets discarded' can never drift apart."
    - "AcceptRecoveryPoint decodes+validates the chosen blob before ever calling Save, so an invalid recovery point is refused deterministically and never partially applied."
    - "CLI recovery offer mirrors pool update/apply's preview-then-confirm UX: show open always reports the offer, and only an explicit --accept-recovery <id>/--discard-recovery flag mutates anything."

key-files:
  created:
    - internal/show/recovery.go
    - internal/show/recovery_test.go
    - internal/command/show.go
    - internal/command/show_test.go
  modified: []

key-decisions:
  - "AcceptRecoveryPoint promotes through the existing exported Save path (not a private write) so the accepted content re-validates and re-stamps SchemaVersion/Revision exactly like any other edit -- no second write path exists."
  - "show open's --accept-recovery <id> is restricted server-side to ids currently returned by DetectRecoveryPoints; a stale or made-up id is refused with GOLC_SHOW_RECOVERY_NOT_FOUND rather than silently falling through to whatever row happens to exist."
  - "show save-as resolves --to through the existing resolveWritablePath rule and passes the already-resolved absolute path to show.Save, avoiding a second path-resolution rule (05-RESEARCH.md T-05-04)."

patterns-established:
  - "Offer-then-explicit-action for destructive/state-changing recovery: detection is always read-only, and acceptance/discard require a caller-supplied flag naming exactly what to do."

requirements-completed: [SHOW-02, SHOW-04]

coverage:
  - id: D1
    description: "Recovery points are detected read-only (newest-first, revision > last clean save), capped at 5 by store.go's existing pruning, and only ever removed via an explicit DELETE (discard) or promoted via an explicit accept through the existing Save path."
    requirement: "SHOW-04"
    verification:
      - kind: unit
        ref: "internal/show/recovery_test.go#TestRecoveryPointPruning"
        status: pass
      - kind: unit
        ref: "internal/show/recovery_test.go#TestRecoveryOfferedNotApplied"
        status: pass
      - kind: unit
        ref: "internal/show/recovery_test.go#TestRecoveryDiscardDeletes"
        status: pass
      - kind: unit
        ref: "internal/show/recovery_test.go#TestRecoveryAcceptPersists"
        status: pass
      - kind: unit
        ref: "internal/show/recovery_test.go#TestRecoveryAcceptRejectsInvalidBlob"
        status: pass
    human_judgment: false
  - id: D2
    description: "show open/save/save-as CLI routes exist on the existing 'show' scope; save-as never mutates the source; open surfaces (never auto-applies) an interrupted-session recovery offer, with --accept-recovery/--discard-recovery as the only explicit mutating paths."
    requirement: "SHOW-02"
    verification:
      - kind: unit
        ref: "internal/command/show_test.go#TestShowSaveRoute"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowSaveAsRoute"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenCleanFileReportsNoRecovery"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenReportsRecoveryOfferWithoutMutating"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenDiscardRecoveryRemovesPoints"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenAcceptRecoveryPromotesChosenPoint"
        status: pass
      - kind: manual_procedural
        ref: "go run ./cmd/golc-project pool create \"Wash Pool\" --show show.golc && go run ./cmd/golc-project show open --show show.golc"
        status: pass
    human_judgment: false

# Metrics
duration: 45min
completed: 2026-07-23
status: complete
---

# Phase 5 Plan 2: Recovery Detection and Show CLI Routes Summary

**Read-side recovery API (`DetectRecoveryPoints`/`DiscardRecoveryPoints`/`AcceptRecoveryPoint`) plus `show open`/`save`/`save-as` CLI routes, wiring an offer-only-never-auto-applied interrupted-session recovery flow into `show open`.**

## Performance

- **Duration:** 45 min
- **Started:** 2026-07-23T05:15:00Z
- **Completed:** 2026-07-23T05:52:43Z
- **Tasks:** 2
- **Files modified:** 4 (all new)

## Accomplishments
- `internal/show/recovery.go`: `DetectRecoveryPoints` (read-only, newest-first, never writes), `DiscardRecoveryPoints` (explicit `DELETE FROM recovery_points` for offered rows, not merely hiding them), `AcceptRecoveryPoint` (decode + validate a chosen blob, then persist through the existing `Save` path) — the read side of SHOW-04, completing Plan 01's write-side (D-04-D-06).
- `internal/command/show.go`: registers `show open`, `show save`, `show save-as` on the already-declared `show` scope (no duplicate scope registration); `show open` calls `DetectRecoveryPoints` after `Load` and offers (never auto-applies) any interrupted-session recovery, routing `--accept-recovery <id>` and `--discard-recovery` to the corresponding `internal/show` functions.
- `show open` on a schema-too-new file surfaces `GOLC_SHOW_SCHEMA_TOO_NEW` at exit 1 (D-10 edit-path hard refusal), distinguished from other runtime failures via `errors.As`.
- Manual CLI sanity check (`go run ./cmd/golc-project`) confirms a clean file reports `GOLC_SHOW_OPENED` with no recovery offer.

## Task Commits

Each task was committed atomically:

1. **Task 1: Recovery detect / accept / discard (read side of SHOW-04)** - `5818321` (feat)
2. **Task 2: show open / save / save-as CLI routes with recovery offer** - `7a8b55c` (feat)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/show/recovery.go` - `RecoveryPoint`, `DetectRecoveryPoints`, `DiscardRecoveryPoints`, `AcceptRecoveryPoint`
- `internal/show/recovery_test.go` - pruning cap, offered-not-applied, discard-deletes, accept-persists, invalid-blob-refused tests
- `internal/command/show.go` - `show open`/`show save`/`show save-as` routes, handlers, arg parsers
- `internal/command/show_test.go` - end-to-end route tests through `command.NewDefaultCommandRegistry()`

## Decisions Made
- `show open --accept-recovery <id>` is restricted to ids currently returned by `DetectRecoveryPoints` (checked in the command handler before calling `AcceptRecoveryPoint`), refusing a stale/made-up id with `GOLC_SHOW_RECOVERY_NOT_FOUND` rather than deferring entirely to the storage layer's own existence check.
- `show save` reloads after `Save` to report the bumped revision, following `store.go`'s documented contract that `Save` never mutates its `State` argument in place.
- `show save-as` pre-resolves `--to` through `resolveWritablePath` and passes the already-absolute destination into `show.Save`, so the single root-relative-vs-absolute rule is applied exactly once.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Simulating "a file with newer recovery points" for `internal/command/show_test.go` (a `command_test`-package, black-box test) required writing directly into the `.golc` SQLite file's `recovery_points` table, since `internal/show` intentionally exposes no seam for creating that state (CONTEXT D-07: nothing auto-writes a recovery point outside `Save`'s own commit). Resolved by opening the same `.golc` file directly through the already-registered `"sqlite"` `database/sql` driver (blank-imported explicitly in the test file for clarity) and inserting one `recovery_points` row via raw SQL, mirroring `internal/show/store_test.go`'s own equivalent technique at the package-internal level and `chase_motion_test.go`'s documented precedent for this class of simulation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- SHOW-02 (open/save/save-as) and the read side of SHOW-04 (recovery detect/accept/discard) are complete and covered by unit tests; `go test ./...` is green with no regressions.
- Plan 03 (schema migration) and later plans can build on `show open`'s existing route without touching its recovery-offer wiring; `show.ErrSchemaMigrationRequired` is already surfaced by `show.Load`/`LoadForRead` from Plan 01 and is ready for Plan 03's migration flow to intercept.
- Read-only inspect/export/diagnose for a newer-than-supported schema file (D-10's "not fully blind" requirement) remains explicitly out of this plan's scope, deferred to Plan 05 per the plan's own `<behavior>` note.

---
*Phase: 05-durable-shows-and-recovery*
*Completed: 2026-07-23*

## Self-Check: PASSED

All created files verified present on disk (internal/show/recovery.go, internal/show/recovery_test.go, internal/command/show.go, internal/command/show_test.go, this SUMMARY.md); all task/metadata commits (5818321, 7a8b55c, 11c99b6) verified present in `git log`.
