---
phase: 7
slug: versioned-external-control-api
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-24
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (this repo does not use testify/rapid — verified by grep) |
| **Config file** | none — plain `go test` |
| **Quick run command** | `mage testquick` |
| **Full suite command** | `mage test` |
| **Estimated runtime** | ~45 seconds (full suite, per this session's own runs) |

---

## Sampling Rate

- **After every task commit:** Run `mage testquick`
- **After every plan wave:** Run `mage test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

*Seeded from 07-RESEARCH.md's Phase Requirements → Test Map. The planner fills in exact Task ID/Plan/Wave columns once PLAN.md files exist; this row set is the requirement-level floor every task-level row must trace back to.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | 0 | API-01 | — | HTTP mutation produces the same outcome as the equivalent CLI/Wails call (parity) | integration | `go test ./internal/api/... -run TestParity -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | API-02 | — | Generated OpenAPI doc has no drift from committed snapshot | unit | `go test ./internal/api/... -run TestOpenAPIDrift -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | API-03 | — | Client reconnecting with a stale Last-Event-ID past the buffer window receives a resync signal, not silently-missing events | unit | `go test ./internal/api/... -run TestSSEGapRecovery -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | API-04 | T-batch-atomicity | Stale If-Match returns 412; dry-run mutates nothing; batch either fully applies or fully rolls back | unit + integration | `go test ./internal/api/... -run TestRevision\|TestDryRun\|TestBatchAtomic -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | API-05 | T-remote-bind | Server binds loopback-only when api.remote_enabled is unset/false; remote bind requires the flag AND a valid scoped key | unit | `go test ./internal/api/... -run TestBindAddress\|TestAuth -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | 0 | API-06 | T-audit-redaction | Every mutation writes exactly one audit_log row with actor/source/correlation/outcome/redacted fields populated | unit + integration | `go test ./internal/show/... -run TestAuditLog -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/` package and its test files do not exist yet — this entire phase is Wave 0 for test infrastructure.
- [ ] `internal/show/audit_test.go` — covers the new `audit_log` table's write/read path (API-06), following `internal/show/store_test.go`'s existing style.
- [ ] Framework install: none — plain `testing`, no new test framework needed.

---

## Manual-Only Verifications

*If none: "All phase behaviors have automated verification."*

All phase behaviors have automated verification. (Remote-access threat-model review and rate-limit tuning are design decisions captured in RESEARCH.md's Open Questions, not manual test steps.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
