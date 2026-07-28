---
phase: 05-durable-shows-and-recovery
reviewed: 2026-07-22T00:00:00Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - internal/command/chase_motion_test.go
  - internal/command/show.go
  - internal/command/show_diagnose.go
  - internal/command/show_diagnose_test.go
  - internal/command/show_test.go
  - internal/show/backup.go
  - internal/show/backup_test.go
  - internal/show/diagnose.go
  - internal/show/diagnose_test.go
  - internal/show/migrate.go
  - internal/show/migrate_test.go
  - internal/show/recovery.go
  - internal/show/recovery_test.go
  - internal/show/schema.go
  - internal/show/state.go
  - internal/show/store.go
  - internal/show/store_test.go
findings:
  critical: 1
  warning: 6
  info: 1
  total: 8
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-07-22T00:00:00Z
**Depth:** standard
**Files Reviewed:** 16
**Status:** issues_found

## Summary

The SQLite-backed store, backup/verify, migration, and diagnostics layers are well-documented and internally consistent, and the migration/schema-version handling that was flagged as an area of concern (`readMeta` vs `migrationMeta`'s blob-length signal) is coherent and correctly reasoned — I did not find a bug there. Atomic file replacement on Windows (`atomicReplace`'s `os.Rename` + stray `-wal`/`-shm` cleanup) is also sound for the destination side.

The one **critical** finding is structural, not cosmetic: the SHOW-04 interrupted-session recovery mechanism (`recovery.go`) can never actually be triggered by a real crash. `Save` writes the recovery-point INSERT in the *same* transaction as the `show_meta` UPDATE (store.go:227-241), so the two values that `DetectRecoveryPoints` compares (`recovery_points.revision` vs. `show_meta.revision`) are always committed together, atomically, in lockstep. There is no code path in production that can leave a `recovery_points` row with a revision *greater than* the current `show_meta.revision` — that state is only reachable in this codebase's own tests by writing directly into `recovery_points` via raw SQL, bypassing `Save` entirely (see every `insertRecoveryPoint`/`seedRecoveryPoint` helper). The feature is entirely inert against a genuine interrupted session.

Several warnings cover smaller robustness/quality gaps: a checksum column that is written but never verified, checkpoint/close errors that are silently discarded at most call sites (contradicting the documented contract), `AcceptRecoveryPoint` not enforcing its own "must be offered" precondition, and an exported, unsynchronized test-seam (`RegisterTestMigration`) that lives in production code rather than being scoped to tests.

## Critical Issues

### CR-01: SHOW-04 recovery-point offer is unreachable for any genuine interrupted session

**File:** `internal/show/store.go:227-241`, `internal/show/recovery.go:36-92`

**Issue:**
`Save` writes the state, bumps `show_meta.revision`, and inserts the matching `recovery_points` row all inside **one transaction**:

```go
// store.go
if _, err := tx.Exec(`UPDATE show_meta SET schema_version = ?, revision = ?, ...`, s.SchemaVersion, s.Revision, ...); err != nil { ... }
if _, err := tx.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, payload); err != nil { ... }
if _, err := tx.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`, now, s.Revision, payload); err != nil { ... }
...
if err := tx.Commit(); err != nil { ... }
```

`DetectRecoveryPoints`/`offeredRecoveryRevision` (recovery.go:36-92) offer a recovery point only when its `revision` is *strictly greater than* `show_meta.revision`:

```go
threshold, err := offeredRecoveryRevision(db) // == show_meta.revision
rows, err := db.Query(`SELECT id, created_at, revision FROM recovery_points WHERE revision > ?`, threshold)
```

Because `s.Revision` is written to `show_meta` and inserted into `recovery_points` in the *same commit*, every successful `Save` leaves `recovery_points`'s newest row's revision exactly equal to (never greater than) `show_meta.revision`. WAL + `synchronous=FULL` guarantees this transaction is all-or-nothing: a crash before commit leaves neither row written (nothing to offer, and `show_meta.revision` unchanged); a crash after commit leaves both written together (still nothing to offer, since equal is not `>`). There is no window in real execution where a `recovery_points` row can be strictly newer than `show_meta.revision`.

This is confirmed by the fact that *every single test* that exercises the "offer" behavior (`TestRecoveryOfferedNotApplied`, `TestRecoveryDiscardDeletes`, `TestRecoveryAcceptPersists`, `TestShowOpenReportsRecoveryOfferWithoutMutating`, `TestShowOpenDiscardRecoveryRemovesPoints`, `TestShowOpenAcceptRecoveryPromotesChosenPoint`, etc.) has to seed the "offered" precondition by writing directly into `recovery_points` via raw SQL (`insertRecoveryPoint` in recovery_test.go, `seedRecoveryPoint` in show_test.go), bypassing `Save` entirely — there is no application code path that produces this state.

In production, `show open` will never surface `GOLC_SHOW_RECOVERY_AVAILABLE` after a real crash, no matter how the process was killed, because `Save`'s atomicity guarantee (CONTEXT D-04, intentionally designed "so a crash mid-command commits both atomically, or neither") is directly incompatible with SHOW-04/D-07's requirement that a recovery point be detectable as newer than the last clean save. As implemented, D-04 and D-07 cannot both hold simultaneously — this is a design conflict, not just a coding slip.

**Fix:**
This needs a structural decision, not a one-line patch. Two directions that would actually make recovery detectable:
1. Decouple the recovery-point write from the state-save commit: write the recovery point in its own transaction *before* the state-save transaction begins (e.g., snapshot "the state as loaded, before this command's edit" at the start of a mutating command, or snapshot at a lower-frequency cadence than every Save). Then a crash between the two commits genuinely leaves `recovery_points.revision > show_meta.revision`.
2. Alternatively, track "last explicitly-acknowledged clean revision" as a separate value the CLI updates only on a deliberate action (e.g., process exit / explicit `show save`), and compare recovery points against *that* instead of `show_meta.revision`, so per-command `Save` calls can advance `show_meta.revision` without simultaneously advancing the "acknowledged" watermark.

Whichever direction is chosen, the existing test suite's `insertRecoveryPoint`/`seedRecoveryPoint` helpers should be replaced (or supplemented) with a test that kills/interrupts real command execution (e.g., via a fault-injection point) to prove the offer is reachable through actual application code, not only through direct SQL tampering.

## Warnings

### WR-01: `checksum` column is written on every save but never read back or verified

**File:** `internal/show/store.go:69-72, 213, 228-231`; `internal/show/migrate.go:194-196`

**Issue:** `sha256Hex(payload)` is computed and persisted into `show_meta.checksum` on every `Save` and every `migrateTemp`, and `readMeta` even reads the column back into `storeMeta.Checksum`. But no code anywhere (`Load`, `LoadForRead`, `Diagnose`, or elsewhere) ever recomputes the hash of the stored blob and compares it against `storeMeta.Checksum`. The field's own doc comment (store.go:57-60) describes it as "an integrity check (detects accidental corruption)", but as wired up it detects nothing — it is write-only. A blob whose bytes were altered post-write (but which still happens to decode as valid JSON matching the struct shape, and still passes `validate()`) would silently be trusted.

**Fix:** Recompute `sha256Hex(blob)` in `decodeAndValidate` (or `Load`/`LoadForRead`) and compare against `meta.Checksum`, returning `GOLC_SHOW_STATE_INVALID` (checksum mismatch) on failure — mirroring what `verifyBackupReadBack` already does structurally for backups, just without the actual hash comparison.

### WR-02: Checkpoint/close errors are silently discarded at most call sites, contradicting the documented contract

**File:** `internal/show/store.go:145,176,219`; `internal/show/recovery.go:67,106`; `internal/show/diagnose.go:51`; `internal/show/backup.go:32,59`; `internal/show/migrate.go:155`

**Issue:** `checkpointAndClose`'s doc comment (schema.go:141-147) states: "a checkpoint failure is reported to the caller but is never treated as data loss." In practice, only two call sites actually capture and check the returned error (`migrate.go:97` and `recovery.go:138`); every other call site uses a bare `defer checkpointAndClose(db)`, e.g.:

```go
db, err := openStore(root, path)
if err != nil { return State{}, err }
defer checkpointAndClose(db)  // return value discarded
```

This affects `Load`, `LoadForRead`, `Save`, `DetectRecoveryPoints`, `DiscardRecoveryPoints`, `Diagnose`, `verifiedBackup`, `verifyBackupReadBack`, and `migrateTemp`. A failed `PRAGMA wal_checkpoint(PASSIVE)` (which can happen under I/O pressure or lock contention) is never surfaced to any of these callers, so unbounded `-wal` growth or a failed `db.Close()` would go completely unnoticed.

**Fix:** Either capture and log/return the error at each of these sites (consistent with the documented contract), or narrow the doc comment to describe what actually happens (best-effort, errors intentionally ignored) so the contract and the code agree.

### WR-03: `AcceptRecoveryPoint` does not itself enforce the "must be currently offered" precondition

**File:** `internal/show/recovery.go:131-158`

**Issue:** `AcceptRecoveryPoint(root, path, id)` looks up `id` directly in `recovery_points` and, if found, decodes/validates/`Save`s it — with no check that `id` is among the rows `DetectRecoveryPoints` would currently offer (i.e., `revision > offeredRecoveryRevision(db)`). The only place that enforces "must be offered" is the CLI layer (`internal/command/show.go:322-333`), which separately fetches `points` and checks membership before calling `AcceptRecoveryPoint`. Any other caller of this exported function (a future CLI route, a library consumer, a test helper reused incorrectly) could pass the id of an *already-superseded* or *older* recovery_points row (up to 5 are retained per file) and silently revert the show to older content via a normal `Save`, directly at odds with the package's own stated "recovery prohibition: MUST NOT auto-apply, silently overwrite... the user's explicitly-saved .golc contents" (recovery.go:9-14).

**Fix:** Move the "is `id` currently offered" check into `AcceptRecoveryPoint` itself (reuse `offeredRecoveryRevision`/the same predicate `DetectRecoveryPoints` uses), returning `GOLC_SHOW_RECOVERY_NOT_FOUND` from the package itself rather than relying entirely on the CLI layer to enforce it.

### WR-04: `RegisterTestMigration` is an exported, unsynchronized production API mutating shared global state

**File:** `internal/show/migrate.go:33,49-52`

**Issue:** `migrations` is a package-level `map[int]func([]byte) ([]byte, error)`, and `RegisterTestMigration` — despite existing solely to let other packages' tests exercise the migration engine — is exported from a non-`_test.go` file, making it part of `internal/show`'s real, importable API surface for any caller, not just tests. There is no mutex guarding reads (`migrateTemp`'s `migrations[v]` lookup) against concurrent writes (`RegisterTestMigration`/its cleanup closure), so any accidental production use, or any future test that runs in parallel (`t.Parallel()`), would race. More importantly, nothing in the type system stops production code from calling `show.RegisterTestMigration` to silently override real migration behavior process-wide.

**Fix:** If Go's lack of test-only visibility is the real constraint, prefer an internal, unexported seam with a package-private test-injection hook set via a build-tagged file (e.g., `migrate_testhook.go` with `//go:build test`) or guard the map behind a `sync.Mutex` at minimum so concurrent registration/lookup is not a data race, and document clearly that calling it outside tests is unsupported/unsafe.

### WR-05: `atomicReplace` does not clean up stray sidecars left at the temp-file name

**File:** `internal/show/migrate.go:213-223`

**Issue:** `atomicReplace` removes `-wal`/`-shm` sidecars only at `resolvedDest` (the final `.golc` path) after the rename. It does not account for the possibility that `tempPath`'s *own* `-wal`/`-shm` files (`path + ".migrate-tmp-wal"`, `"-shm"`) are still present when `os.Rename` runs — `os.Rename` on Windows does not move sidecar files, so if `migrateTemp`'s checkpoint-then-close (itself a bare `defer`, see WR-02) does not fully drain/remove them, they are orphaned under the `.migrate-tmp-*` name after the rename succeeds. `Migrate`'s own best-effort `defer os.Remove(resolvePath(root, tempPath))` (migrate.go:134) only targets the main temp db file, not its sidecars.

**Fix:** After a successful rename, also best-effort `os.Remove` the `tempPath + "-wal"`/`"-shm"` sidecars (mirroring what's already done for `resolvedDest`), so a partially-drained temp WAL never accumulates on disk across repeated migrations.

### WR-06: `Diagnose` reports "requires migration" the same as genuine structural corruption

**File:** `internal/show/diagnose.go:88-101`

**Issue:** `Diagnose` reuses `LoadForRead` for its structural check, but `LoadForRead` returns `ErrSchemaMigrationRequired` for an older-schema (unmigrated) file — an expected, recoverable state, not corruption. `Diagnose` folds this into the same `StructuralOK=false` / `StructuralError` bucket as an actual `validate()` failure:

```go
_, structuralErr := LoadForRead(root, path)
report := DiagnosticReport{ ..., StructuralOK: structuralErr == nil, ... }
if structuralErr != nil { report.StructuralError = structuralErr.Error() }
```

`show diagnose` on a perfectly healthy, merely-unmigrated file will exit 1 and print a `StructuralError` that reads like corruption ("... GOLC_SHOW_SCHEMA_MIGRATION_REQUIRED: ...") rather than distinctly signaling "this file just needs `show open --confirm-migration`", which could send an operator down the wrong remediation path.

**Fix:** Detect `errors.As(structuralErr, &ErrSchemaMigrationRequired{})` explicitly in `Diagnose` and surface it as a distinct field/state (e.g., a `MigrationRequired bool`) rather than conflating it with `StructuralOK=false`.

## Info

### IN-01: Backup filename has only 1-second granularity and can collide

**File:** `internal/show/backup.go:34`

**Issue:** `backupPath = path + ".backup-" + time.Now().UTC().Format("20060102T150405Z")` has second-level resolution. Two backups of the same `path` within the same wall-clock second (e.g., `Migrate` invoked twice in quick succession, or a scripted retry loop) will produce the same `backupPath`, and `VACUUM INTO` refuses to overwrite an existing file, so the second call fails with `GOLC_SHOW_BACKUP_FAILED` instead of succeeding or producing a distinguishable name.

**Fix:** Add sub-second precision or a short random/uuid suffix to the backup filename to avoid same-second collisions.

---

_Reviewed: 2026-07-22T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
