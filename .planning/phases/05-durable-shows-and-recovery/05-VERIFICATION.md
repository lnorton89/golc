---
phase: 05-durable-shows-and-recovery
verified: 2026-07-23T00:00:00Z
status: passed
score: 10/10 functional truths verified; 1 documentation-only gap found and closed post-verification
behavior_unverified: 0
overrides_applied: 0
re_verification: null
gaps: []
resolved_gaps:
  - truth: "REQUIREMENTS.md traceability accurately reflects delivered requirements (project Definition of Done: 'the requirement-to-phase ... mappings are current')"
    status: resolved
    resolved_at: "2026-07-23T00:00:00Z"
    resolution_commit: "3a8d77c"
    reason: "SHOW-01, SHOW-03, and SHOW-06 were genuinely implemented and test-verified in the codebase (confirmed independently below), but their REQUIREMENTS.md checkboxes/traceability rows were still marked [ ] Pending. Git archaeology confirmed this was a stale-checkbox/doc-sync omission, not a functional gap: the 05-01 completion commit (dcfe08a, claims SHOW-01/02/03) and the 05-04 completion commit (10cdac5, claims SHOW-06) never touched REQUIREMENTS.md, while the 05-02 commit (11c99b6) correctly flipped SHOW-02/SHOW-04 and the 05-05 commit (a03d20e) correctly flipped SHOW-05. Closed directly (no code changes needed) by flipping all three checkboxes/table rows to Complete in commit 3a8d77c."
    artifacts:
      - path: ".planning/REQUIREMENTS.md"
        issue: "Lines 88/90/93 (checkbox list) and the Traceability table (SHOW-01/SHOW-03/SHOW-06 rows) said 'Pending' despite delivered, tested implementations. Fixed."
---

# Phase 5: Durable Shows and Recovery Verification Report

**Phase Goal:** Users can preserve and recover complete shows in a portable versioned format while storage remains outside the deterministic playback path.
**Verified:** 2026-07-23
**Status:** passed (one documentation-only gap found and closed post-verification, commit 3a8d77c; all functional truths verified)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A complete `show.State` (pools, deployments, groups, programmer, themes, presets, chases, motion presets, scenes, blend presets, tempo) saves to and loads from one portable SQLite `.golc` file with byte-identical domain fields (SHOW-01) | VERIFIED | `internal/show/store.go` `Save`/`Load`; `TestShowStoreRoundTrip` PASS. No host/machine-local data embedded in the payload — `Save` writes only `strictjson.CanonicalEncode(s)`; confirmed by source inspection (`grep -rn "os.Hostname\|user.Current"` in `internal/show/*.go` = empty). |
| 2 | Saving twice produces a valid, re-openable file; Revision bumps by exactly one per save, no duplication (SHOW-01 idempotency) | VERIFIED | `TestShowStoreSaveIsIdempotent` PASS |
| 3 | Load is read-only and idempotent — never mutates the file or bumps Revision (SHOW-02) | VERIFIED | `TestShowLoadDoesNotMutate` PASS |
| 4 | `internal/show` never imports `internal/playback` — storage structurally cannot enter the deterministic timing path (governing constraint) | VERIFIED | `TestShowStoreNoPlaybackImport` PASS; independently reconfirmed with `go list -deps github.com/lnorton89/golc/internal/show \| grep -x .../internal/playback` → no match (exit 1) |
| 5 | `show open`/`save`/`save-as` operate without stopping deterministic output unexpectedly (SHOW-02) | VERIFIED | Each is a separate short-lived CLI process (`internal/command/show.go`); `internal/playback.Engine` never calls `show.Load`/`Save` (confirmed via source read, `internal/playback/engine.go` holds `State` in-memory via `atomic.Pointer`); `TestShowSaveRoute`/`TestShowSaveAsRoute`/`TestShowOpenCleanFileReportsNoRecovery` PASS |
| 6 | Authoring changes are autosaved to rotating recovery points without storage work in the playback timing path, and an interrupted session can genuinely be detected and restored (SHOW-03/SHOW-04) | VERIFIED | **Critical review finding CR-01 resolved.** `Save` now commits `stageRecoveryPoint` (INSERT+prune, own transaction) then `promoteState` (UPDATE show_meta/show_state, separate transaction) — see `internal/show/store.go:204-300`. `TestRecoveryReachableViaRealInterruptedSave` proves the offer is reachable via the real `stageRecoveryPoint` production code path (simulated kill between the two commits), not only via raw-SQL test seeding — PASS. Recovery capped at 5, oldest pruned (`TestRecoveryPointPruning` PASS). Detection is read-only (`TestRecoveryOfferedNotApplied` PASS); discard is a real `DELETE` (`TestRecoveryDiscardDeletes` PASS, `grep -c 'DELETE FROM recovery_points'` = 2 across recovery.go); acceptance requires an explicit id currently offered, enforced inside the package itself post-review (`TestShowOpenAcceptRecoveryRejectsUnofferedID` PASS, closing WR-03). |
| 7 | A schema migration creates and verifies a backup, commits atomically, and refuses unsupported newer formats without rewriting (SHOW-05) | VERIFIED | `internal/show/backup.go` `verifiedBackup` (`VACUUM INTO` + fresh-connection read-back + `validate()`, never trusts `VACUUM INTO` success alone) — `TestVerifiedBackupRoundTrips`/`TestVerifiedBackupRejectsCorruptBackup` PASS. `internal/show/migrate.go` `Migrate`: backup → temp-copy migrate-in-transaction → re-validate → `atomicReplace` (`os.Rename`) only on full success — `TestMigrateAppliesRegisteredTransforms`, `TestMigrateProducesVerifiedBackup`, `TestMigrationForceKillLeavesOriginalIntact` PASS. Newer-format refusal: `TestMigrateRefusesNewerFormat` (byte-unchanged) PASS; bounds-check before registry index: `TestMigrateBoundsChecksVersion` PASS. CLI confirm flow: `TestShowOpenMigrationRequiresConfirm`/`TestShowOpenMigrationWithConfirm` PASS; newer-format refusal across every mutating route: `TestShowSaveRefusesNewerFormat`/`TestShowOpenRefusesNewerFormat`/`TestShowCurrentVersionOpensNormally` PASS. |
| 8 | A user can run integrity diagnostics that combine file-level and structural checks, and never falsely report a bad file as healthy (SHOW-06) | VERIFIED | `internal/show/diagnose.go` `Diagnose` runs `PRAGMA integrity_check` + `LoadForRead`(`validate()`) — `TestDiagnoseHealthyFile`, `TestDiagnoseStructurallyInvalid`, `TestDiagnoseFileCorruption` (genuine injected page corruption, verified real `integrity_check` findings per 05-04-SUMMARY.md) all PASS. `show diagnose` exits 1 on any issue: `TestShowDiagnoseCorruptExitOne` PASS. |
| 9 | A user can export a versioned, human-readable, round-trippable JSON representation for troubleshooting/interchange, not a filtered projection (SHOW-06) | VERIFIED | `internal/command/show_diagnose.go` `runShowExport` calls `strictjson.CanonicalEncode(state)` directly (not `buildShowInspectView`) — `grep -c CanonicalEncode` ≥ 1; `TestShowExportMatchesCanonicalEncode` (byte-identical + round-trip) PASS. |
| 10 | Diagnose/export tolerate a newer-than-supported schema_version read-only without rewriting, and neither runs automatically on open (D-10/D-12) | VERIFIED | `TestShowExportTooNewReadOnly` PASS (uses `LoadForRead`, not `Load`); `show diagnose`/`show export` are separate explicit CLI routes, never invoked from `runShowOpen` (confirmed by source read of `internal/command/show.go` and `show_diagnose.go`). |

**Score:** 10/10 functional truths verified (0 behavior-unverified, 0 overrides). One documentation-traceability gap identified separately (see Gaps below) — it does not represent unimplemented functionality.

### Critical Review Fix Verification (CR-01)

The orchestrator's fix for the code-review-flagged structural defect (Save's single transaction made SHOW-04 recovery unreachable in production) was independently re-derived and confirmed sound:

- **Before fix:** `Save` wrote the `show_meta` UPDATE and the `recovery_points` INSERT in one transaction, so `recovery_points.revision` and `show_meta.revision` could never differ — `DetectRecoveryPoints`' `revision > show_meta.revision` predicate could never fire from a real crash, only from raw-SQL test seeding.
- **After fix:** `Save` now runs `stageRecoveryPoint` (own transaction: INSERT + prune-to-5, `store.go:219-239`) followed by `promoteState` (own transaction: UPDATE show_meta/show_state, `store.go:245-263`). A process kill between the two commits leaves `recovery_points` strictly ahead of `show_meta.revision` — exactly the signal the detector needs.
- **Proof, not assertion:** `TestRecoveryReachableViaRealInterruptedSave` (`internal/show/recovery_test.go:125`) calls `stageRecoveryPoint` directly (the real production function) and then closes the DB without calling `promoteState`, simulating a genuine interrupted process — then asserts `DetectRecoveryPoints` offers exactly that point and `AcceptRecoveryPoint` promotes it correctly. This is materially different from every other recovery test in the suite, which seed the "offered" precondition via raw SQL because, before the fix, no application code path could produce it.
- **Verdict:** SHOW-04 is now genuinely achievable via real interrupted-session recovery, not merely "present" as originally shipped by Plan 02. `go test ./internal/show/... ./internal/command/...` green, full suite confirmed.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/show/schema.go` | `openStore`, PRAGMAs, application_id door check, table creation | VERIFIED | Present, substantive, wired (used by every other file in the package) |
| `internal/show/store.go` | `Load`/`Save`/`LoadForRead`, sentinel errors, two-transaction Save | VERIFIED | Present, substantive, wired; two-transaction split confirmed by reading source |
| `internal/show/store_test.go` | Round-trip/idempotency/no-mutation/no-playback-import tests | VERIFIED | All 4 named tests present and pass |
| `internal/show/recovery.go` | `DetectRecoveryPoints`/`DiscardRecoveryPoints`/`AcceptRecoveryPoint` | VERIFIED | Present, substantive, wired; "must be offered" enforced inside the package (WR-03 fix) |
| `internal/show/recovery_test.go` | Pruning/offered-not-applied/discard/accept + real-interruption test | VERIFIED | 6 tests present incl. `TestRecoveryReachableViaRealInterruptedSave`, all pass |
| `internal/show/backup.go` | `verifiedBackup` (`VACUUM INTO` + read-back-validate) | VERIFIED | Present, substantive, wired |
| `internal/show/migrate.go` | `Migrate`, `atomicReplace`, `migrations` registry | VERIFIED | Present, substantive, wired; mutex-guarded registry (WR-04 fix) |
| `internal/show/diagnose.go` | `Diagnose`/`DiagnosticReport` | VERIFIED | Present, substantive, wired; `MigrationRequired` field distinguishes migration-pending from corruption (WR-06 fix) |
| `internal/command/show.go` | `show open`/`save`/`save-as` routes + recovery/migration flow | VERIFIED | Present, substantive, wired on existing `show` scope (no duplicate registration — confirmed by green tests) |
| `internal/command/show_diagnose.go` | `show diagnose`/`show export` routes | VERIFIED | Present, substantive, wired |
| All `*_test.go` companions | Named tests per plan | VERIFIED | Every named test in every plan's acceptance criteria exists and passes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/command/*.go` handlers | `show.Load`/`show.Save` | Unchanged call-site signatures (D-02) | WIRED | `go build ./...` + `go test ./...` green; no call-site rewiring needed |
| `store.go` Save | `stageRecoveryPoint` → `promoteState` | Two sequential transactions | WIRED | Confirmed sound by source read + `TestRecoveryReachableViaRealInterruptedSave` |
| `internal/command/show.go` `runShowOpen` | `show.DetectRecoveryPoints` | Offer-not-apply after Load | WIRED | `grep -c DetectRecoveryPoints internal/command/show.go` ≥ 1; test-confirmed non-mutating |
| `migrate.go` `Migrate` | `verifiedBackup` → `migrateTemp` → `atomicReplace` | Sequenced, only replaces on full success | WIRED | `TestMigrationForceKillLeavesOriginalIntact` proves original survives interruption |
| `internal/command/show.go` `runShowOpen` | `show.Migrate` | Gated behind `--confirm-migration` | WIRED | `TestShowOpenMigrationRequiresConfirm` proves untouched-without-flag; `TestShowOpenMigrationWithConfirm` proves migrate-with-flag |
| `internal/command/show_diagnose.go` `runShowExport` | `strictjson.CanonicalEncode` | Full document, not filtered projection | WIRED | `TestShowExportMatchesCanonicalEncode` byte-identity + round-trip proof |
| `internal/show` package | `internal/playback` | MUST NOT import | CONFIRMED ABSENT | `go list -deps` check, zero matches |

### Behavioral Spot-Checks / Full Test Evidence

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full `internal/show` + `internal/command` suites | `go test ./internal/show/... ./internal/command/...` | ok (all tests incl. named phase-5 tests) | PASS |
| No playback import | `go list -deps .../internal/show \| grep -x .../internal/playback` | no match | PASS |
| No debt markers in phase-5 files | `grep -rn -E "TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER" internal/show/*.go internal/command/show*.go` | no matches | PASS |
| No host/machine-local leakage | `grep -rn -E "os.Hostname\|os.Getenv\("USER\|user.Current\|filepath.Abs" internal/show/*.go` | no matches | PASS |
| Pure-Go SQLite driver pinned | `go.mod`/`go.sum` `modernc.org/sqlite v1.54.0` | present, matches research decision (no CGo) | PASS |
| Full repo test suite | `go test ./...` | 1 pre-existing, unrelated, documented failure (`internal/trace/catalog`, Phase-11 `TBD` placeholder) | PASS (regression-free for Phase 5) |

### Requirements Coverage

| Requirement | Source Plan | Description | Functional Status | REQUIREMENTS.md Status |
|-------------|------------|-------------|--------------------|--------------------------|
| SHOW-01 | 05-01 | Save complete show to one portable `.golc` | SATISFIED (verified above) | **STALE — marked Pending, should be Complete** |
| SHOW-02 | 05-01/05-02 | Open/save/save-as without stopping playback | SATISFIED | Complete (correctly marked) |
| SHOW-03 | 05-01 | Autosave recoverable changes outside playback path | SATISFIED (verified above; recovery-point write rides inside Save's now-two-transaction commit, no timer/background writer, no playback import) | **STALE — marked Pending, should be Complete** |
| SHOW-04 | 05-02 (+ CR-01 fix) | Recover from interrupted session via rotating recovery points | SATISFIED (post-fix; genuinely reachable, not just present) | Complete (correctly marked) |
| SHOW-05 | 05-03/05-05 | Verified backup, atomic migration, refuse newer formats | SATISFIED | Complete (correctly marked) |
| SHOW-06 | 05-04 | Integrity diagnostics + versioned JSON export | SATISFIED (verified above) | **STALE — marked Pending, should be Complete** |

**Finding on the orchestrator's specific question (SHOW-01/03/06 vs. SHOW-02/04/05):** Confirmed via git archaeology to be a **stale checkbox, not a genuine functional gap**. The plan-completion doc-commits for this phase were inconsistent about updating `REQUIREMENTS.md`:
- `dcfe08a` ("docs(05-01): complete SQLite-backed .golc store plan") — claims SHOW-01/02/03 in its own SUMMARY frontmatter, but its diff touches **zero** lines of `REQUIREMENTS.md`.
- `11c99b6` ("docs(05-02): complete recovery-detection-and-show-cli-routes plan") — correctly flips SHOW-02 and SHOW-04 to `[x]`/Complete.
- `10cdac5` ("docs(05-04): complete on-demand show diagnostics and export plan") — claims SHOW-06 in its own SUMMARY frontmatter, but its diff touches **zero** lines of `REQUIREMENTS.md`.
- `a03d20e` ("docs(05-05): complete migration-on-open confirm flow and D-10 refusal plan") — correctly flips SHOW-05 to `[x]`/Complete.

The underlying code for SHOW-01, SHOW-03, and SHOW-06 is genuinely implemented, tested, and passing (see Observable Truths #1, #6, #8/#9 above) — this is purely a bookkeeping omission in two of the five plan-completion commits, not evidence of missing work. It is listed as a `gaps` entry below because the project's own Definition of Done ("...the requirement-to-phase and requirement-to-Linear mappings are current") is not currently met for these three rows, and the fix is a trivial, code-free doc edit.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in any Phase 5 file. No hardcoded empty stubs. No silently-discarded errors (WR-02 explicitly fixed this). No unenforced preconditions (WR-03 fixed). No unsynchronized shared mutable state in production code paths (WR-04 fixed with a mutex).

One **info-level, deliberately-skipped** item remains from the code review: **IN-01** — `backup.go`'s timestamp-based backup filename has 1-second granularity and could theoretically collide if `Migrate` runs twice against the same file within the same wall-clock second. This was explicitly triaged as info-level/cosmetic and skipped per `05-REVIEW-FIX.md`; it does not block any SHOW-0X requirement (a same-second collision fails loudly with `GOLC_SHOW_BACKUP_FAILED` rather than corrupting anything) and is reasonable to leave as a low-priority follow-up.

### Human Verification Required

None. Every truth in this phase was verifiable programmatically — including the SHOW-04 crash-recovery invariant, which is exercised by a test that reproduces the real interrupted-commit sequence through production code (`TestRecoveryReachableViaRealInterruptedSave`) rather than relying on code-review judgment alone.

### Gaps Summary

**One gap, documentation-only, zero functional risk — found and closed post-verification.** `REQUIREMENTS.md`'s checkbox list and Traceability table showed SHOW-01, SHOW-03, and SHOW-06 as "Pending" even though all three are fully implemented and covered by passing automated tests (verified independently in this report, not taken on SUMMARY.md's word). This was a stale-checkbox/doc-sync gap caused by two of the phase's five plan-completion commits omitting the `REQUIREMENTS.md` update step that the other three performed correctly. Closed by the orchestrator immediately after this report (commit `3a8d77c`) with a two-line-per-requirement documentation edit — no code changes required or made. See `resolved_gaps` in the frontmatter for detail.

All other review-flagged issues (1 critical, 6 warnings) from `05-REVIEW.md` were fixed and independently re-verified in this report as sound — in particular the critical SHOW-04-reachability defect (CR-01), which was structurally re-derived and confirmed via the two-transaction `Save` split and its real-interruption test.

---

_Verified: 2026-07-23_
_Verifier: Claude (gsd-verifier)_
