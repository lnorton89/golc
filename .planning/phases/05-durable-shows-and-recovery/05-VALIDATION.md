---
phase: 5
slug: durable-shows-and-recovery
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-23
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib), via the project's own `test` command route |
| **Config file** | none — `go test` driven by `_test.go` files; project-local scope markers follow the existing `TestScope{PascalName}` convention (`internal/command/test.go`); the `show` scope already exists (`internal/command/deployment.go`) for `show inspect` and extends naturally to this phase's `show open/save/save-as/diagnose/recover/migrate` commands |
| **Quick run command** | `./golc.ps1 test --quick --scope show` |
| **Full suite command** | `./golc.ps1 test` |
| **Estimated runtime** | ~30 seconds (consistent with Phase 1–4's project-wide suite) |

---

## Sampling Rate

- **After every task commit:** Run `./golc.ps1 test --quick --scope show`
- **After every plan wave:** Run `./golc.ps1 test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | SHOW-01 | T-05-01 / mitigate | SQLite-backed Save/Open/Save-As round trip preserves the full `State`; `PRAGMA application_id` + `strictjson.DecodeStrict` + `validate()` reject a malformed/foreign SQLite file before trusting any field | unit | `go test ./internal/show/... -run TestShowStoreRoundTrip` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-02 | — | Open/Save/Save-As never import or call into `internal/playback` — no path from storage into the engine's hot path | unit (architecture/import check) | `go test ./internal/show/... -run TestShowStoreNoPlaybackImport` (or a `go list -deps` assertion) | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-03 | — | Every command mutation writes a recovery point inside the same SQLite transaction as the save (D-04) | unit | `go test ./internal/show/... -run TestSaveWritesRecoveryPoint` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-04 | — | Recovery points capped at 5, oldest pruned first (D-06); recovery is detected and offered on next open, never silently auto-applied (D-07) | unit | `go test ./internal/show/... -run TestRecoveryPointPruning` / `TestRecoveryOfferedNotApplied` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-05 | T-05-02, T-05-03 / mitigate | Migration creates a `VACUUM INTO` backup, verifies it via fresh-connection read-back + `validate()` (D-09, not checksum-only), applies atomically, and refuses newer-than-supported `schema_version` without rewriting the file (D-10) | unit + integration | `go test ./internal/show/... -run TestMigration` (fixture-per-schema-version corpus) | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-05 (forced-kill) | T-05-03 / mitigate | A process killed mid-migration leaves the original working file untouched — atomic swap only happens after full transactional success | integration (forced-kill simulation) | `go test ./internal/show/... -run TestMigrationForceKillLeavesOriginalIntact` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | SHOW-06 | — | Integrity diagnostics combine `PRAGMA integrity_check` (file-level) with `validate()` (structural); JSON export is byte-identical to `strictjson.CanonicalEncode(State)`; diagnostics run on-demand only, never automatically on open | unit | `go test ./internal/show/... -run TestDiagnose` / `TestExportMatchesCanonicalEncode` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs and Threat Refs are TBD pending PLAN.md creation — to be backfilled by gsd-plan-checker/gsd-verifier once plans exist, following Phase 3/4's own backfill precedent.*

---

## Wave 0 Requirements

- [ ] `internal/show/store_test.go` — SQLite-backed Load/Save round trip, replacing/extending the existing JSON-based `state_test.go` coverage (SHOW-01/02)
- [ ] `internal/show/recovery_test.go` — recovery-point write/prune/detect/offer (SHOW-03/04)
- [ ] `internal/show/migrate_test.go` — migration corpus: one fixture file per historical `schema_version`, plus a forced-kill-mid-migration test proving the original file survives untouched (SHOW-05)
- [ ] `internal/show/backup_test.go` — `VACUUM INTO` + fresh-connection read-back-and-validate (D-09), including a deliberately-corrupted backup fixture proving verification actually rejects a bad backup
- [ ] `internal/show/diagnose_test.go` — `integrity_check` + `validate()` combined report, JSON export byte-identity (SHOW-06)
- [ ] Framework install: `go get modernc.org/sqlite@v1.54.0` — no test framework install needed (stdlib `testing`)

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification. The forced-kill-mid-migration and WAL-sidecar-cleanup scenarios are automatable (simulated process termination in a test harness), unlike Phase 4's hardware/packet-capture verifications which genuinely require external tools.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
