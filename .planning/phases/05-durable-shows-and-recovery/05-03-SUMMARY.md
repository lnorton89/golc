---
phase: 05-durable-shows-and-recovery
plan: 03
subsystem: database
tags: [sqlite, modernc-sqlite, migration, backup, vacuum-into, atomic-rename, go]

# Dependency graph
requires:
  - phase: 05-durable-shows-and-recovery
    provides: "05-01: internal/show/schema.go's openStore/checkpointAndClose connection lifecycle, store.go's Load/Save/readMeta/decodeAndValidate, ErrSchemaTooNew/ErrSchemaMigrationRequired, sha256Hex"
provides:
  - internal/show/backup.go: verifiedBackup (VACUUM INTO + fresh-connection read-back-and-validate, D-09) and verifyBackupReadBack (the reusable verification half)
  - internal/show/migrate.go: the migrations function-map registry, Migrate (detect-old -> verifiedBackup -> migrate-temp-copy-in-transaction -> re-validate -> atomicReplace), migrateTemp, atomicReplace, copyFile, migrationMeta
  - Error codes GOLC_SHOW_BACKUP_FAILED, GOLC_SHOW_BACKUP_UNVERIFIABLE, GOLC_SHOW_MIGRATE_SWAP_FAILED
affects: [05-04, 05-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Verified backup: VACUUM INTO a timestamped snapshot, then re-open that snapshot in a fresh connection and DecodeStrict+validate() before trusting it -- VACUUM INTO returning no error is never treated as proof on its own (D-09)"
    - "Migration registry as a Go function map (map[int]func([]byte)([]byte,error)) keyed by schema_version, not a SQL migration framework -- the store's schema is one blob's shape, not evolving DDL"
    - "Backup-then-migrate-temp-then-atomic-swap: the original working file is never written to until a fully re-validated migrated copy replaces it via os.Rename"
    - "migrationMeta reads schema_version/blob directly (not via store.go's readMeta) so blob length -- not the schema_version integer alone -- is the 'has this file ever actually been saved' signal, letting migrations legitimately originate at schema_version=0"

key-files:
  created:
    - internal/show/backup.go
    - internal/show/backup_test.go
    - internal/show/migrate.go
    - internal/show/migrate_test.go
  modified: []

key-decisions:
  - "migrationMeta bypasses store.go's readMeta (Plan 01) because readMeta deliberately collapses schema_version==0 into 'never saved' for the Load/Save fresh-show short circuit -- that collapse would make it impossible to test or ever perform a migration originating at schema_version=0, the only 'older than current' slot available while SchemaVersion is still pinned at 1. migrationMeta instead uses show_state.blob length (openStore's seed row always leaves it empty, X'') as the precise 'ever saved' signal, independent of the schema_version integer."
  - "Bounds-check floor set at schema_version >= 0 (GOLC_SHOW_STATE_INVALID below that), not >= 1 as 05-RESEARCH.md's threat-register prose loosely states -- schema_version==0 is this plan's own valid synthetic-fixture convention for a real historical version (distinguished from 'never saved' by blob length, not the integer), so the strict floor is 0, matching the concrete TestMigrateAppliesRegisteredTransforms/TestMigrateBoundsChecksVersion test specs in the plan's own <action> text over the looser prose in the threat register."
  - "verifiedBackup's read-back-and-validate step is split into a separate verifyBackupReadBack function so tests can exercise it in isolation against a deliberately-corrupted backup file, proving GOLC_SHOW_BACKUP_UNVERIFIABLE is actually returned rather than merely asserted by a round-trip test alone."
  - "The temp migration copy is created via a raw byte copy (copyFile) of the already-VACUUM-INTO'd, checkpointed, closed verified backup -- not a second VACUUM INTO of the live original -- since the backup is a static, closed snapshot at that point, making a raw copy safe (the 'never copy a live WAL-mode file' rule applies to the original working file, not to an already-checkpointed-and-closed backup)."

patterns-established:
  - "atomicReplace(root, destPath, tempPath) is the standard migration-swap entry point: os.Rename plus stray -wal/-shm sidecar cleanup at the destination, reused by any future migration-shaped feature in this package."
  - "migrationMeta(db) is the standard 'read raw on-disk version + blob for a migration-adjacent decision' entry point, distinct from readMeta's Load/Save-oriented semantics."

requirements-completed: [SHOW-05]

coverage:
  - id: D1
    description: "verifiedBackup produces a VACUUM INTO snapshot and only accepts it after a fresh connection re-decodes and validates its show_state blob; a backup whose blob is corrupted after the fact is rejected with GOLC_SHOW_BACKUP_UNVERIFIABLE"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/backup_test.go#TestVerifiedBackupRoundTrips"
        status: pass
      - kind: unit
        ref: "internal/show/backup_test.go#TestVerifiedBackupRejectsCorruptBackup"
        status: pass
    human_judgment: false
  - id: D2
    description: "Migrate applies ordered registered migration functions from the on-disk schema_version up to SchemaVersion inside one transaction on a temp copy, producing a schema_version-current, validate()-passing result"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrateAppliesRegisteredTransforms"
        status: pass
    human_judgment: false
  - id: D3
    description: "Migrate's backup is itself a genuinely verified, openable, valid show before the swap -- not merely a path string"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrateProducesVerifiedBackup"
        status: pass
    human_judgment: false
  - id: D4
    description: "A schema_version newer than this build supports returns ErrSchemaTooNew and leaves the file byte-for-byte unchanged (D-10) -- never migrated or rewritten"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrateRefusesNewerFormat"
        status: pass
    human_judgment: false
  - id: D5
    description: "An out-of-range on-disk schema_version (negative) is rejected as GOLC_SHOW_STATE_INVALID before it is ever used to index the migrations registry -- the registered migration function is never invoked"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrateBoundsChecksVersion"
        status: pass
    human_judgment: false
  - id: D6
    description: "A migration interrupted mid-flight (registered migration function itself fails) leaves the original working file fully intact -- byte-for-byte and at the raw meta/blob level -- and the backup taken before the failure remains independently verifiable recovery material"
    requirement: SHOW-05
    verification:
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrationForceKillLeavesOriginalIntact"
        status: pass
    human_judgment: false
  - id: D7
    description: "go build ./... and go test ./... pass with the new backup/migration code in place, and go vet ./internal/show/... is clean"
    verification:
      - kind: unit
        ref: "go test ./... (full suite, 21 packages)"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-07-22
status: complete
---

# Phase 5 Plan 3: Verified Backup and Schema Migration Engine Summary

**Storage-layer migration engine for SHOW-05: `VACUUM INTO` + fresh-connection read-back-and-validate backups (D-09), a Go function-map migration registry (not a SQL migration framework), and a Windows-safe atomic `os.Rename` swap that never touches the original `.golc` file until the migrated, re-validated temp copy is fully ready.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-22T22:40:00-07:00 (approx.)
- **Completed:** 2026-07-22T22:53:25-07:00
- **Tasks:** 2
- **Files modified:** 4 (all created)

## Accomplishments
- `internal/show/backup.go`: `verifiedBackup` produces a timestamped `VACUUM INTO` snapshot and only trusts it after a fresh connection re-decodes and validates the backup's blob (D-09) — `VACUUM INTO` succeeding is never treated as proof of a valid backup on its own
- `verifyBackupReadBack` is split out as its own function so the read-back-and-validate check can be exercised in isolation against a deliberately-corrupted backup, proving `GOLC_SHOW_BACKUP_UNVERIFIABLE` is actually returned, not just asserted by a happy-path test
- `internal/show/migrate.go`: `Migrate` detects an older on-disk `schema_version`, runs `verifiedBackup` first, migrates a raw-copied temp file through the ordered `migrations` registry inside one transaction, re-validates the result, and only then calls `atomicReplace` (`os.Rename`) — the original is never written to before that swap
- `migrations` ships as an empty `map[int]func([]byte)([]byte,error)` in production (only `schema_version=1` exists today); all five tests inject a synthetic entry at `migrations[0]` via `t.Cleanup` to exercise the engine end-to-end
- `Migrate` bounds-checks the on-disk `schema_version` before ever indexing the registry (`T-05-02`) and hard-refuses a newer-than-supported file with `ErrSchemaTooNew`, leaving it byte-for-byte unchanged (`D-10`)
- `TestMigrationForceKillLeavesOriginalIntact` simulates a failure inside the registered migration function itself (after the backup was already taken, before `atomicReplace`) and proves the original survives byte-for-byte and at the raw meta/blob level, and that the pre-failure backup remains independently verifiable

## Task Commits

Each task was committed atomically:

1. **Task 1: Verified backup (VACUUM INTO + read-back-and-validate)** - `a911346` (feat)
2. **Task 2: Migration registry + transactional migrate + atomic replace** - `e78fcaf` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `internal/show/backup.go` - `verifiedBackup`, `verifyBackupReadBack`
- `internal/show/backup_test.go` - `TestVerifiedBackupRoundTrips`, `TestVerifiedBackupRejectsCorruptBackup`
- `internal/show/migrate.go` - `migrations` registry, `migrationMeta`, `Migrate`, `migrateTemp`, `atomicReplace`, `copyFile`
- `internal/show/migrate_test.go` - `seedRawShow`, `fixturePayload`, `registerIdentityMigration` test helpers, `TestMigrateAppliesRegisteredTransforms`, `TestMigrateProducesVerifiedBackup`, `TestMigrateRefusesNewerFormat`, `TestMigrateBoundsChecksVersion`, `TestMigrationForceKillLeavesOriginalIntact`

## Decisions Made
- **`migrationMeta` bypasses `readMeta`.** Plan 01's `readMeta` deliberately collapses `schema_version==0` into "never saved" for the Load/Save fresh-show short circuit. Since `SchemaVersion` is a fixed const `== 1` today, `schema_version=0` is the *only* "older than current" value available to exercise the migration engine at all — so `Migrate` needed a way to distinguish a genuinely-saved historical show at version 0 from a truly-fresh, never-saved seed row. `migrationMeta` reads `show_meta.schema_version`/`show_state.blob` directly and uses blob length (empty only for the never-saved seed row) as that signal, leaving `readMeta`/`Load`/`Save` in `store.go` completely untouched.
- **Bounds-check floor is `>= 0`, not `>= 1`.** `05-RESEARCH.md`'s threat register describes the valid range loosely as "[1, SchemaVersion]", but the plan's own concrete test specifications (`TestMigrateAppliesRegisteredTransforms` migrating from a synthetic `schema_version=0` fixture, and `TestMigrateBoundsChecksVersion` testing negative values) only make sense with a floor of 0. Treated the concrete `<action>` test specs as authoritative over the looser prose description elsewhere in the same document.
- **Temp migration copy is a raw byte copy of the already-verified backup, not a second `VACUUM INTO` of the live original.** The backup produced by `verifiedBackup` is fully checkpointed and closed by the time it's copied, so a raw `copyFile` is safe (the "never raw-copy a live WAL-mode file" rule specifically concerns files that might still have an active writer/WAL, not an already-closed static snapshot).

## Deviations from Plan

None — plan executed as written, with the interpretive resolutions above (both already documented as Decisions Made, since they reconcile ambiguous/loosely-worded guidance rather than fix a bug or add unplanned functionality) applied to keep the plan's own concrete test specifications satisfiable given Plan 01's already-shipped `readMeta` semantics.

## Issues Encountered
None. `go build ./...`, `go vet ./internal/show/...`, and `go test ./...` (all 21 packages, including the `internal/trace/catalog` package flagged as having an unrelated pre-existing failure in 05-01-SUMMARY.md) are all green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/show.Migrate`/`verifiedBackup` are the stable storage-layer foundation the CLI confirm flow (the `runShowOpen` migration branch in `internal/command/show.go`, out of this plan's scope per its objective) will call in a later 05-xx plan.
- `GOLC_SHOW_BACKUP_FAILED`, `GOLC_SHOW_BACKUP_UNVERIFIABLE`, and `GOLC_SHOW_MIGRATE_SWAP_FAILED` are available for that CLI layer to surface to the operator.
- No blockers for 05-04/05-05.

---
*Phase: 05-durable-shows-and-recovery*
*Completed: 2026-07-22*

## Self-Check: PASSED

- FOUND: internal/show/backup.go
- FOUND: internal/show/backup_test.go
- FOUND: internal/show/migrate.go
- FOUND: internal/show/migrate_test.go
- FOUND: .planning/phases/05-durable-shows-and-recovery/05-03-SUMMARY.md
- FOUND commit: a911346 (feat(05-03): verified backup via VACUUM INTO + read-back-and-validate)
- FOUND commit: e78fcaf (feat(05-03): migration registry, transactional migrate, atomic replace)
