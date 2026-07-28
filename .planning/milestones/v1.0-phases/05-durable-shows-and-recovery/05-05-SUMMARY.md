---
phase: 05-durable-shows-and-recovery
plan: 05
subsystem: database
tags: [sqlite, cli, migration, schema-versioning, database-sql, go]

# Dependency graph
requires:
  - phase: 05-durable-shows-and-recovery (05-02)
    provides: "show open/save/save-as CLI routes on the 'show' scope, recovery-offer wiring"
  - phase: 05-durable-shows-and-recovery (05-03)
    provides: "internal/show.Migrate/verifiedBackup: verified-backup + transactional migrate + atomic-replace engine, ErrSchemaTooNew/ErrSchemaMigrationRequired sentinels"
provides:
  - "show open --confirm-migration: the D-08 automatic-on-open-with-confirmation migration flow (detect -> report GOLC_SHOW_MIGRATION_REQUIRED -> confirm -> Migrate -> GOLC_SHOW_MIGRATED)"
  - "D-10 newer-format refusal proven across every mutating show route (open-for-edit/save/save-as) with a byte-unchanged guarantee; read-only diagnose/export unaffected"
  - "internal/show.RegisterTestMigration: exported cross-package test seam for exercising Migrate end-to-end"
  - "fix: internal/show.readMeta now distinguishes a genuinely-saved historical show at schema_version 0 from a never-saved seed row via blob length, making ErrSchemaMigrationRequired reachable from Load/LoadForRead for the first time"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Migration confirm flow mirrors pool update/apply's preview-then-confirm UX: show open always reports GOLC_SHOW_MIGRATION_REQUIRED read-only, and only an explicit --confirm-migration flag calls show.Migrate -- no code path migrates without the token."
    - "Cross-package test seams for otherwise-unreachable internal state (show.RegisterTestMigration) are exported deliberately, mirroring the codebase's existing raw-SQL test-seam precedent (show_test.go's seedRecoveryPoint) rather than reaching into unexported package internals."

key-files:
  created: []
  modified:
    - internal/command/show.go
    - internal/command/show_test.go
    - internal/show/store.go
    - internal/show/migrate.go

key-decisions:
  - "readMeta's schema_version==0 collapse-to-'never-saved' check now also requires an empty show_state.blob (the same blob-length signal migrate.go's migrationMeta already established), because with SchemaVersion pinned at 1, schema_version=0 was the ONLY reachable 'older than current' value and the old schema_version-only check made ErrSchemaMigrationRequired permanently unreachable from Load/LoadForRead for any real on-disk file -- a bug fix that was a hard prerequisite for wiring the D-08 flow at all, not merely a test-convenience tweak."
  - "Added internal/show.RegisterTestMigration as a deliberate, small, exported test-only seam (package-level migrations registry mutator + cleanup func) so internal/command/show_test.go can exercise show.Migrate's verifiedBackup -> migrate-temp -> atomic-replace sequence end-to-end through the real 'show open --confirm-migration' CLI route -- the production migrations registry ships empty since only SchemaVersion=1 has ever existed, mirroring migrate_test.go's own package-internal registerIdentityMigration helper, exported here because that helper lives in a _test.go file and is invisible to other packages."
  - "GOLC_SHOW_MIGRATION_REQUIRED (detection) and GOLC_SHOW_MIGRATED (success) are new, distinct top-level codes at the CLI layer -- not reused verbatim from show.ErrSchemaMigrationRequired.Error()'s embedded GOLC_SHOW_SCHEMA_MIGRATION_REQUIRED text -- matching the plan's own frontmatter status-code list."
  - "Cleaned up a latent double-prefix in the pre-existing (Plan 02) too-new refusal message (it wrapped err.Error(), which already carries the GOLC_SHOW_SCHEMA_TOO_NEW prefix, in another GOLC_SHOW_SCHEMA_TOO_NEW-prefixed format string) while touching that branch for the migration-required addition; cosmetic only, both forms already satisfied every acceptance check."

patterns-established:
  - "A schema-versioned mutating route detects-then-confirms a destructive-adjacent operation via a single explicit CLI flag (--confirm-migration), never a stdin prompt loop -- the same shape pool apply's --plan-id and show open's --accept-recovery/--discard-recovery already established."

requirements-completed: [SHOW-05]

coverage:
  - id: D1
    description: "show open on an older-schema .golc detects the pending migration, reports GOLC_SHOW_MIGRATION_REQUIRED naming found/supported versions, and refuses to touch the file without --confirm-migration"
    requirement: "SHOW-05"
    verification:
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenMigrationRequiresConfirm"
        status: pass
    human_judgment: false
  - id: D2
    description: "show open --confirm-migration runs verifiedBackup -> migrate-in-temp -> re-validate -> atomic replace, then opens the migrated show and reports GOLC_SHOW_MIGRATED with the backup path; a verifiable backup file exists on disk"
    requirement: "SHOW-05"
    verification:
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenMigrationWithConfirm"
        status: pass
    human_judgment: false
  - id: D3
    description: "show save/save-as/open(edit) all refuse a newer-than-supported .golc with GOLC_SHOW_SCHEMA_TOO_NEW (exit 1) and leave the file byte-unchanged; show save-as also never writes the destination"
    requirement: "SHOW-05"
    verification:
      - kind: unit
        ref: "internal/command/show_test.go#TestShowSaveRefusesNewerFormat"
        status: pass
      - kind: unit
        ref: "internal/command/show_test.go#TestShowOpenRefusesNewerFormat"
        status: pass
    human_judgment: false
  - id: D4
    description: "A current-version file is unaffected by the new migration/refusal branches: it opens and saves normally with no false migration-required or too-new notice"
    requirement: "SHOW-05"
    verification:
      - kind: unit
        ref: "internal/command/show_test.go#TestShowCurrentVersionOpensNormally"
        status: pass
    human_judgment: false
  - id: D5
    description: "go build ./..., go vet ./..., and go test ./... (whole-repo phase gate) all pass with the migration confirm flow and refusal matrix in place"
    verification:
      - kind: unit
        ref: "go test ./... (full suite, 21 packages)"
        status: pass
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-07-23
status: complete
---

# Phase 5 Plan 5: Migration-On-Open Confirm Flow and Newer-Format Refusal (SHOW-05) Summary

**`show open --confirm-migration` wires Plan 03's verified-backup + transactional-migrate + atomic-replace engine into a detect-then-confirm CLI flow (GOLC_SHOW_MIGRATION_REQUIRED -> GOLC_SHOW_MIGRATED), and every mutating `show` route now provably refuses a newer-than-supported `.golc` byte-unchanged -- completing SHOW-05 end to end.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 2
- **Files modified:** 4 (internal/command/show.go, internal/command/show_test.go, internal/show/store.go, internal/show/migrate.go)

## Accomplishments
- `internal/command/show.go`'s `runShowOpen` branches on `show.ErrSchemaMigrationRequired`: without `--confirm-migration` it reports `GOLC_SHOW_MIGRATION_REQUIRED` (naming found -> supported schema_version and that a verified backup will be made first) and touches nothing; with the flag it calls `show.Migrate`, re-`Load`s the migrated show, and reports `GOLC_SHOW_MIGRATED` with the backup path -- mirroring `runPoolApply`'s detect-separately-from-mutate shape (D-08).
- Every mutating `show` route (`show open` edit path, `show save`, `show save-as`) is now proven, with a dedicated test matrix, to refuse a newer-than-supported `.golc` with `GOLC_SHOW_SCHEMA_TOO_NEW` (exit 1) and leave it byte-unchanged; read-only `show diagnose`/`show export` (Plan 04) are untouched and still read such a file (D-10).
- **Bug fix (prerequisite for D-08):** `internal/show/store.go`'s `readMeta` collapsed `schema_version == 0` into "never saved" purely from the integer, regardless of the saved blob -- with `SchemaVersion` pinned at 1, that made `ErrSchemaMigrationRequired` permanently unreachable from `Load`/`LoadForRead` for any real on-disk file (a genuinely-saved historical show at schema_version 0 was silently treated as a fresh, empty show). `readMeta` now reuses `migrate.go`'s already-established blob-length signal (empty blob = never saved) to distinguish the two cases, exactly matching `migrationMeta`'s existing convention.
- Added `internal/show.RegisterTestMigration`, a small exported test-only seam (package-level `migrations` registry mutator + required cleanup func) so `internal/command/show_test.go` can exercise `show.Migrate` end-to-end through the real CLI route -- necessary because the production `migrations` registry ships empty (only `SchemaVersion=1` has ever existed) and the package-internal `registerIdentityMigration` test helper lives in a `_test.go` file invisible to other packages.
- `go build ./...`, `go vet ./...`, and `go test ./...` (all 22 packages) are green.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration-on-open confirm flow in show open** - `48d9cfe` (feat)
2. **Task 2: Newer-format refusal across mutating show routes (D-10)** - `331c2fd` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `internal/command/show.go` - `showOpenArgs.confirmMigration`, `--confirm-migration` parsing, `runShowOpen`'s migration-required/too-new branches
- `internal/command/show_test.go` - `seedOlderSchemaShow`/`seedTooNewSchemaShow` fixture helpers, `TestShowOpenMigrationRequiresConfirm`, `TestShowOpenMigrationWithConfirm`, `TestShowSaveRefusesNewerFormat`, `TestShowOpenRefusesNewerFormat`, `TestShowCurrentVersionOpensNormally`
- `internal/show/store.go` - `readMeta`'s blob-length-aware never-saved check
- `internal/show/migrate.go` - exported `RegisterTestMigration` test seam

## Decisions Made
- **`readMeta`'s blob-length fix** is a Rule 1 bug fix, not a scope expansion: without it, `show.Load` could never produce `ErrSchemaMigrationRequired` for any real file while `SchemaVersion` stays at 1 (the only "older" integer, 0, was unconditionally treated as "never saved"), which would have made this plan's own Task 1 acceptance criteria (a real `show open` -> `ErrSchemaMigrationRequired` -> confirm -> `Migrate` round trip) impossible to satisfy honestly.
- **`RegisterTestMigration` as an exported test seam** (Rule 2/3: needed to fulfill Task 1's explicit acceptance test) rather than duplicating `migrate_test.go`'s package-internal `registerIdentityMigration` — the alternative (skipping a true end-to-end migrated-and-reopened assertion) would have left `TestShowOpenMigrationWithConfirm`'s core claim ("the file is migrated, a backup exists, and the migrated show loads") unverified.
- **`GOLC_SHOW_MIGRATION_REQUIRED`/`GOLC_SHOW_MIGRATED` are new CLI-layer codes**, not the internal error's own `GOLC_SHOW_SCHEMA_MIGRATION_REQUIRED` text, per the plan's frontmatter status-code list.
- **Cosmetic cleanup**: the pre-existing (Plan 02) too-new refusal message wrapped `err.Error()` (already `"GOLC_SHOW_SCHEMA_TOO_NEW: ..."`) inside another `"GOLC_SHOW_SCHEMA_TOO_NEW: %v"` format string, double-prefixing the code. Simplified to `err.Error()` directly while touching this branch for the migration-required addition; both forms already satisfied every `strings.Contains` acceptance check, so this is a pure readability fix, not a behavior change.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `readMeta` made `ErrSchemaMigrationRequired` unreachable from `Load`/`LoadForRead`**
- **Found during:** Task 1 (Migration-on-open confirm flow)
- **Issue:** `readMeta` treated any `show_meta.schema_version == 0` row as "never saved" purely from the integer value, discarding the row. Since `show.SchemaVersion` is pinned at `1`, `0` is the only reachable "older than current" value in this codebase -- meaning `show.Load` could never actually surface `ErrSchemaMigrationRequired` for a real saved file, contradicting this plan's own `<behavior>` spec ("Load returns ErrSchemaMigrationRequired") and `05-03-SUMMARY.md`'s own documented intent for that error.
- **Fix:** `readMeta` now also reads `show_state`'s blob length in the same query and only treats the row as "never saved" when `schema_version == 0 AND blob is empty` -- reusing `migrate.go`'s `migrationMeta` blob-length signal that already solved the identical ambiguity for `Migrate`.
- **Files modified:** `internal/show/store.go`
- **Verification:** `TestShowOpenMigrationRequiresConfirm`/`TestShowOpenMigrationWithConfirm` (new) exercise the real `Load` -> `ErrSchemaMigrationRequired` path end-to-end; full existing `internal/show` and `internal/command` suites remain green (the fix only changes behavior for the synthetic `schema_version==0`-with-non-empty-blob case no production `Save` call can produce).
- **Committed in:** `48d9cfe` (Task 1 commit)

**2. [Rule 2/3 - Missing test seam] Added `show.RegisterTestMigration`**
- **Found during:** Task 1 (Migration-on-open confirm flow)
- **Issue:** The production `migrations` registry (`internal/show/migrate.go`) ships empty, so `show.Migrate` had no registered transform to run for the CLI-level end-to-end test this task's acceptance criteria required (`TestShowOpenMigrationWithConfirm`: "the file is migrated, a backup exists, and the migrated show loads"). The existing package-internal test helper for this (`registerIdentityMigration`) lives in `internal/show/migrate_test.go` and is invisible outside that package.
- **Fix:** Added a small exported `show.RegisterTestMigration(fromVersion, fn) (cleanup func())` seam that mutates the package-level `migrations` map with a required cleanup, mirroring the package-internal helper's shape and contract.
- **Files modified:** `internal/show/migrate.go`
- **Verification:** `TestShowOpenMigrationWithConfirm` registers an identity transform via this seam, runs the full CLI migration flow, and cleans up via `t.Cleanup`; `go test ./...` confirms no state leaks between tests.
- **Committed in:** `48d9cfe` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug fix, 1 missing test seam)
**Impact on plan:** Both were hard prerequisites for honestly satisfying this plan's own stated Task 1 acceptance criteria, not scope creep -- without them, `show open`'s migration branch would be structurally present but permanently untestable/untriggerable for any real file.

## Issues Encountered

Discovered mid-Task-1 (not a blocker, resolved inline as the Rule 1 fix above): `internal/show`'s `readMeta`/`Load` and `migrate.go`'s `migrationMeta`/`Migrate` used two different "has this file ever been saved" signals (integer-only vs. blob-length-aware) for the identical `schema_version==0` edge case introduced by Plan 03's own synthetic-fixture convention. `05-03-SUMMARY.md` had already documented `migrationMeta`'s blob-length rationale; this plan's Task 1 needed the same signal applied to `readMeta` for `Load`/`LoadForRead` to actually surface the error `runShowOpen` needed to branch on.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- SHOW-05 is fully delivered: the D-08 migration confirm flow and D-10 newer-format refusal are both live and covered end-to-end, completing every Phase 5 requirement (SHOW-01 through SHOW-06) across Plans 01-05.
- No blockers for the phase-level wave merge or milestone completion.

---
*Phase: 05-durable-shows-and-recovery*
*Completed: 2026-07-23*

## Self-Check: PASSED

- FOUND: internal/command/show.go (migration branch present, `grep -c 'confirm-migration'` = 5, `grep -c 'Migrate'` = 2)
- FOUND: internal/command/show_test.go (TestShowOpenMigrationRequiresConfirm, TestShowOpenMigrationWithConfirm, TestShowSaveRefusesNewerFormat, TestShowOpenRefusesNewerFormat, TestShowCurrentVersionOpensNormally all present and passing)
- FOUND: internal/show/store.go (readMeta blob-length fix)
- FOUND: internal/show/migrate.go (RegisterTestMigration)
- FOUND commit: 48d9cfe (feat(05-05): migration-on-open confirm flow in show open)
- FOUND commit: 331c2fd (test(05-05): newer-format refusal matrix across mutating show routes (D-10))
- go build ./..., go vet ./..., go test ./... all pass (22 packages)
