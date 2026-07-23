---
phase: 05-durable-shows-and-recovery
fixed_at: 2026-07-23T06:15:00Z
review_path: .planning/phases/05-durable-shows-and-recovery/05-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 7
skipped: 1
status: partial
---

# Phase 05: Code Review Fix Report

**Fixed at:** 2026-07-23T06:15:00Z
**Source review:** .planning/phases/05-durable-shows-and-recovery/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (1 critical, 6 warning, 1 info)
- Fixed: 7 (1 critical, 6 warning)
- Skipped: 1 (info, not in default fix scope)

## Fixed Issues

### CR-01: SHOW-04 recovery-point offer is unreachable for any genuine interrupted session

**Files modified:** `internal/show/store.go`, `internal/show/recovery_test.go`
**Commit:** `c2bde32`
**Applied fix:** Split `Save`'s single transaction into two sequential transactions (`stageRecoveryPoint` then `promoteState`). A crash between the two now leaves a `recovery_points` row strictly newer than `show_meta.revision` — the signal `DetectRecoveryPoints` requires — instead of the two values always landing in lockstep. Added `TestRecoveryReachableViaRealInterruptedSave`, which proves the fix through the real `stageRecoveryPoint` code path (simulated process kill), not raw-SQL seeding.

### WR-01: `checksum` column written but never verified

**Files modified:** `internal/show/store.go`, `internal/show/store_test.go`, `internal/show/diagnose_test.go`
**Commit:** `0336547`
**Applied fix:** `decodeAndValidate` now recomputes `sha256Hex(blob)` and compares against `show_meta.checksum` before decoding, returning `GOLC_SHOW_STATE_INVALID` on mismatch. Updated two existing "tampered content" test fixtures that previously seeded `checksum = ''` (which would now trip the new check for the wrong reason) to seed the correct checksum for their tampered payload. Added `TestShowLoadRejectsChecksumMismatch`, which tampers with a saved file's blob via raw SQL without touching checksum and proves both `Load` and `LoadForRead` now reject it.

### WR-02: Checkpoint/close errors silently discarded at most call sites

**Files modified:** `internal/show/schema.go`, `internal/show/store.go`, `internal/show/recovery.go`, `internal/show/diagnose.go`, `internal/show/backup.go`, `internal/show/migrate.go`
**Commit:** `4141eb2`
**Applied fix:** Added `closeStoreCheckingErr` helper; `Load`, `LoadForRead`, `Save`, `DetectRecoveryPoints`, `DiscardRecoveryPoints`, `Diagnose`, `verifiedBackup`, `verifyBackupReadBack`, and `migrateTemp` now surface a checkpoint/close failure via a named error return instead of a bare `defer checkpointAndClose(db)`, without masking a more specific earlier error.

### WR-03: `AcceptRecoveryPoint` did not enforce its own "must be offered" precondition

**Files modified:** `internal/show/recovery.go`
**Commit:** `4141eb2`
**Applied fix:** `AcceptRecoveryPoint` now checks the requested id's revision against `offeredRecoveryRevision` itself before decoding, refusing a stale/superseded/non-existent id with `GOLC_SHOW_RECOVERY_NOT_FOUND` (matching the CLI layer's existing error code) rather than relying entirely on the caller to check membership first.

### WR-04: `RegisterTestMigration` exported, unsynchronized production API

**Files modified:** `internal/show/migrate.go`
**Commit:** `4141eb2`
**Applied fix:** Added `migrationsMu sync.Mutex` guarding every read (`migrateTemp`) and write (`RegisterTestMigration`) of the package-level `migrations` registry, plus a doc comment clarifying the seam is unsupported outside tests.

### WR-05: `atomicReplace` did not clean up the temp file's own sidecars

**Files modified:** `internal/show/migrate.go`
**Commit:** `4141eb2`
**Applied fix:** `atomicReplace` now also best-effort removes `tempPath + "-wal"`/`"-shm"` after a successful rename, in addition to the destination's sidecars it already cleaned up.

### WR-06: `Diagnose` conflated "requires migration" with genuine corruption

**Files modified:** `internal/show/diagnose.go`
**Commit:** `4141eb2`
**Applied fix:** `DiagnosticReport` gains a `MigrationRequired bool` field, set via `errors.As` against `ErrSchemaMigrationRequired`, distinct from `StructuralOK`/`StructuralError`.

## Skipped Issues

### IN-01: Backup filename has only 1-second granularity and can collide

**File:** `internal/show/backup.go:34`
**Reason:** Info-level, not in default fix scope (critical + warning only); cosmetic edge case (two `Migrate` calls against the same file within the same wall-clock second).
**Original issue:** `backupPath` uses second-resolution timestamps; a same-second collision causes `VACUUM INTO` to fail with `GOLC_SHOW_BACKUP_FAILED` instead of producing a distinguishable name.

---

_Fixed: 2026-07-23T06:15:00Z_
_Fixer: Claude (orchestrator, applied directly rather than via gsd-code-fixer)_
_Iteration: 1_
