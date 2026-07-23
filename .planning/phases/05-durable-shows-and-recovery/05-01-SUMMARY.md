---
phase: 05-durable-shows-and-recovery
plan: 01
subsystem: database
tags: [sqlite, modernc-sqlite, database-sql, wal, go, persistence]

# Dependency graph
requires:
  - phase: 03-deterministic-show-programming-and-playback
    provides: internal/show.State (domain model), validate(), the Load/Save call-site shape every internal/command/*.go handler already uses
provides:
  - SQLite-backed .golc single-file store replacing internal/show's JSON file I/O
  - internal/show/schema.go: openStore (PRAGMAs, application_id door check, table creation, singleton-row seeding), checkpointAndClose
  - internal/show/store.go: Load/Save/LoadForRead (identical signatures to the pre-Phase-5 JSON-backed Load/Save), sha256Hex, ErrSchemaTooNew, ErrSchemaMigrationRequired
  - Every command mutation now writes a pruned-to-5 recovery point inside the same transaction as its Save (SHOW-03's write half)
affects: [05-02, 05-03, 05-04, 05-05]

# Tech tracking
tech-stack:
  added: ["modernc.org/sqlite v1.54.0 (pure-Go database/sql driver, blank-imported)"]
  patterns:
    - "Single-blob-plus-metadata SQLite schema (show_meta/show_state/recovery_points) instead of a normalized relational schema"
    - "PRAGMA application_id door check before ever querying domain tables, so a foreign SQLite file fails cleanly with GOLC_SHOW_NOT_GOLC_FORMAT instead of a deep 'no such table' error"
    - "Seed singleton rows (schema_version=0 sentinel) at table-creation time so Save's UPDATE...WHERE id=1 always finds a row; schema_version==0 (or sql.ErrNoRows) both mean 'never saved yet'"
    - "Save's recovery-point INSERT + prune-to-5 DELETE share the same transaction as the state UPDATE, so a crash mid-save commits both or neither (D-04)"

key-files:
  created:
    - internal/show/schema.go
    - internal/show/store.go
    - internal/show/store_test.go
  modified:
    - go.mod
    - go.sum
    - internal/show/state.go
    - internal/command/chase_motion_test.go

key-decisions:
  - "Load/Save moved out of state.go into store.go (state.go keeps only State/Tempo/resolvePath/validate) since Go disallows two definitions of the same function name in one package -- state.go's own doc comment now points to store.go"
  - "A schema_version==0 singleton row (seeded by openStore at table-creation time) is the SQLite-era equivalent of the old 'file does not exist' fresh-State short circuit, kept alongside a defensive sql.ErrNoRows check"
  - "Corrupt-but-not-even-SQLite files (e.g. raw JSON text) fail during the PRAGMA setup step and are wrapped as GOLC_SHOW_STATE_INVALID; only a structurally valid SQLite file with a foreign application_id is rejected as GOLC_SHOW_NOT_GOLC_FORMAT -- this ordering kept the pre-existing state_test.go tampered-file assertion passing unmodified"

patterns-established:
  - "openStore is internal/show's single connection-lifecycle entry point every Load/Save/LoadForRead call goes through -- later plans (recovery, migration, diagnose) should reuse it rather than opening sql.DB directly"
  - "checkpointAndClose(db) is the standard Close path: passive WAL checkpoint before Close so -wal/-shm sidecars don't grow unbounded across the per-command-process lifecycle"

requirements-completed: [SHOW-01, SHOW-02, SHOW-03]

coverage:
  - id: D1
    description: "A complete show.State saves to and loads from one SQLite .golc file with byte-identical domain fields, and Revision increments exactly once per Save"
    requirement: SHOW-01
    verification:
      - kind: unit
        ref: "internal/show/store_test.go#TestShowStoreRoundTrip"
        status: pass
    human_judgment: false
  - id: D2
    description: "Saving the same State twice to the same path each produces a valid, openable .golc; Revision advances by exactly one per Save with no entity duplication"
    requirement: SHOW-01
    verification:
      - kind: unit
        ref: "internal/show/store_test.go#TestShowStoreSaveIsIdempotent"
        status: pass
    human_judgment: false
  - id: D3
    description: "Load is read-only and idempotent: repeated Loads return identical State and never mutate the on-disk revision"
    requirement: SHOW-02
    verification:
      - kind: unit
        ref: "internal/show/store_test.go#TestShowLoadDoesNotMutate"
        status: pass
    human_judgment: false
  - id: D4
    description: "internal/show has no dependency on internal/playback (storage never enters the playback timing path), verified mechanically via go list -deps rather than a hand-maintained string list"
    requirement: SHOW-02
    verification:
      - kind: unit
        ref: "internal/show/store_test.go#TestShowStoreNoPlaybackImport"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every Save commits a pruned-to-5 recovery point inside the same transaction as the state save (SHOW-03's write half; D-04)"
    requirement: SHOW-03
    verification:
      - kind: other
        ref: "internal/show/store.go Save() -- single db.Begin()/tx.Commit() transaction: UPDATE show_meta, UPDATE show_state, INSERT recovery_points, DELETE-beyond-5, COMMIT; code-review-verified structural guarantee, not a forced-kill test in this plan's scope"
        status: pass
    human_judgment: true
    rationale: "Structural transaction-atomicity is enforced by SQLite itself and confirmed by code review, but this plan's two tasks did not include an explicit forced-process-kill-mid-transaction test (that level of crash-simulation coverage is scoped to a later 05-xx plan per 05-RESEARCH.md's Wave-0 gaps); flagging for human sign-off rather than auto-passing on code review alone."
  - id: D6
    description: "Every existing mutating command (deployment create, pool update, scene, chase, motion, etc.) round-trips a complete show through SQLite with zero call-site changes (D-02)"
    verification:
      - kind: unit
        ref: "go test ./... (full suite, all internal/command/*_test.go call sites unchanged)"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-07-23
status: complete
---

# Phase 5 Plan 1: SQLite-Backed .golc Store Summary

**Replaced internal/show's JSON file I/O with a SQLite-backed `.golc` store (modernc.org/sqlite, WAL + synchronous=FULL) behind unchanged Load/Save signatures, so every existing mutating command now persists through one durable, transactional file with an in-transaction recovery point on every save.**

## Performance

- **Duration:** 18 min
- **Started:** 2026-07-22T22:17:00-07:00
- **Completed:** 2026-07-22T22:34:43-07:00
- **Tasks:** 2 (RED scaffold, GREEN implementation)
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments
- `internal/show/schema.go`: `openStore` opens/creates the `.golc` SQLite database, applies `PRAGMA journal_mode=WAL`/`synchronous=FULL`/`foreign_keys=ON`, stamps or verifies the GOLC `application_id` door check, creates the `show_meta`/`show_state`/`recovery_points` tables, and seeds their singleton rows on first create
- `internal/show/store.go`: `Load`, `Save`, and the new `LoadForRead` reimplement the domain's persistence contract over SQLite with identical signatures to the pre-Phase-5 JSON-backed functions (D-02) — every `internal/command/*.go` call site compiles and behaves unchanged
- `Save`'s single transaction now writes and prunes-to-5 a recovery point (SHOW-03's write half, D-04/D-05/D-06) atomically alongside the state update
- `Load`/`LoadForRead` preserve `state.go`'s original "nothing from disk is trusted before `validate()` passes" doctrine (CONTEXT T-02-10), now applied to a SQLite blob column
- Full existing test suite (`go test ./...`) still passes except one pre-existing, unrelated `internal/trace/catalog` failure (logged, not fixed — see Deviations)

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing SQLite round-trip test scaffold + driver install (RED)** - `63b9aff` (test)
2. **Task 2: SQLite-backed schema + Load/Save/LoadForRead (GREEN)** - `3ab2bfd` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `internal/show/schema.go` - `openStore`/`checkpointAndClose`, PRAGMA setup, `application_id` door check, `CREATE TABLE IF NOT EXISTS` for the three-table schema, singleton-row seeding
- `internal/show/store.go` - `Load`, `Save`, `LoadForRead`, `sha256Hex`, `ErrSchemaTooNew`, `ErrSchemaMigrationRequired`, `readMeta`/`decodeAndValidate` helpers
- `internal/show/store_test.go` - `TestShowStoreRoundTrip`, `TestShowStoreSaveIsIdempotent`, `TestShowLoadDoesNotMutate`, `TestShowStoreNoPlaybackImport` (Task 1 RED contract), plus `TestShowLoadRejectsOverScopeMotionCapability` (moved from `internal/command`, see Deviations)
- `internal/show/state.go` - Trimmed to `State`/`Tempo`/`resolvePath`/`validate`; `Load`/`Save` moved to `store.go`
- `internal/command/chase_motion_test.go` - Removed the now-format-incompatible `TestChaseMotionLoadRejectsOverScopeMotionCapability` (superseded by the `internal/show`-level test); trimmed now-unused imports
- `go.mod`/`go.sum` - Pinned `modernc.org/sqlite v1.54.0` (direct dependency after `go mod tidy`)
- `.planning/phases/05-durable-shows-and-recovery/deferred-items.md` - Logged the unrelated pre-existing `internal/trace/catalog` failure

## Decisions Made
- **Load/Save relocated from `state.go` to `store.go`.** RESEARCH.md's "Recommended Project Structure" table lists `state.go` as "unchanged," but Go does not allow two `Load`/`Save` definitions in one package — the SQLite-backed versions had to replace, not coexist with, the JSON-backed ones. `state.go` now carries a doc comment pointing to `store.go`.
- **`schema_version==0` singleton-row seeding.** `openStore` seeds a placeholder `show_meta`/`show_state` row (`schema_version=0`) at table-creation time so `Save`'s plain `UPDATE ... WHERE id=1` always finds a row (a bare `UPDATE` against zero rows is a silent no-op in SQLite, not an error). `Load`/`LoadForRead` treat `schema_version==0` identically to `sql.ErrNoRows` as "never saved yet," preserving the pre-SQLite "not-yet-existing file returns a fresh State" contract.
- **Error-code ordering for the `application_id` door check.** PRAGMA setup runs before the `application_id` read; a file that isn't valid SQLite at all fails during PRAGMA setup and is wrapped as `GOLC_SHOW_STATE_INVALID`, while only a structurally-valid-but-foreign SQLite file reaches the `application_id` mismatch branch (`GOLC_SHOW_NOT_GOLC_FORMAT`). This ordering was chosen deliberately so the pre-existing `state_test.go` tampered-JSON-file assertion (`strings.Contains(err.Error(), "GOLC_SHOW_STATE_INVALID")`) kept passing unmodified.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `internal/command/chase_motion_test.go`'s over-scope-capability test wrote raw JSON bytes directly to the show path**
- **Found during:** Task 2 (`go test ./...` regression run)
- **Issue:** `TestChaseMotionLoadRejectsOverScopeMotionCapability` simulated a hand-edited show document by `os.WriteFile`-ing canonical JSON straight to the show path, then calling `show.Load` and asserting `GOLC_SHOW_STATE_INVALID` wrapping `GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE`. With the SQLite-backed store, that raw-JSON file fails the `application_id`/PRAGMA door check before `validate()` is ever reached, breaking the test's premise.
- **Fix:** Moved the equivalent coverage into `internal/show/store_test.go`'s new `TestShowLoadRejectsOverScopeMotionCapability`, which uses `openStore` (available since the test is `package show`, not `show_test`) to write the tampered payload directly into `show_state`'s blob column, bypassing `Save`'s `validate()` call exactly as the original test intended — then asserts `Load` still rejects it. Removed the obsolete test and its now-unused imports (`os`, `uuid`, `strictjson`) from `chase_motion_test.go`.
- **Files modified:** `internal/show/store_test.go`, `internal/command/chase_motion_test.go`
- **Verification:** `go test ./internal/show/... ./internal/command/...` — both packages green
- **Committed in:** `3ab2bfd` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix necessitated by the storage-format refactor)
**Impact on plan:** Necessary to keep `go test ./...` green per Task 2's own acceptance criteria (D-02's "every existing command round-trips through SQLite unchanged" guarantee). No scope creep — the replaced test's exact intent is preserved, just retargeted at the new storage layer's public surface.

## Issues Encountered
- `internal/trace/catalog`'s `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails with a pre-existing `GOLC_MIGRATE_DRIFT` on `.planning/linear-map.json`. Confirmed unrelated: `internal/trace/catalog` has no dependency on `internal/show`, the file has no diff in this worktree, and git history shows this exact drift has recurred and been fixed across multiple earlier phases (`5d76a5f`, `4285645`, `9d44376`). Logged to `deferred-items.md`, not fixed (out of this plan's scope per the deviation rules' scope boundary).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/show.Load`/`Save`/`LoadForRead` are the stable foundation later Phase 5 plans (recovery detection/offer, migration, backup, diagnose, JSON export) build directly on top of, per `openStore`/`checkpointAndClose` being the single connection-lifecycle entry point.
- SHOW-01/SHOW-02 are fully delivered; SHOW-03's write half (recovery-point commit inside Save's transaction) is delivered — the read/offer half (detect + surface on `show open`) is explicitly out of this plan's scope, deferred to a later 05-xx plan per the phase's wave structure.
- No blockers for 05-02 onward.

---
*Phase: 05-durable-shows-and-recovery*
*Completed: 2026-07-23*

## Self-Check: PASSED

- FOUND: internal/show/schema.go
- FOUND: internal/show/store.go
- FOUND: internal/show/store_test.go
- FOUND: .planning/phases/05-durable-shows-and-recovery/05-01-SUMMARY.md
- FOUND: .planning/phases/05-durable-shows-and-recovery/deferred-items.md
- FOUND commit: 63b9aff (test(05-01): add failing SQLite round-trip test scaffold)
- FOUND commit: 3ab2bfd (feat(05-01): reimplement show Load/Save/LoadForRead over SQLite (GREEN))
