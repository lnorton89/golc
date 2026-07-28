---
phase: 7
slug: versioned-external-control-api
status: validated
nyquist_compliant: true
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

*Updated after planning: real Task ID/Plan/Wave values from the 9 committed PLAN.md files (07-01 through 07-09). File Exists stays ❌ W0 for every row — `internal/api/` does not exist until execution begins.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 07-01 | 1 | API-02 | T-07-SC | Human approves chi/huma/x-time supply-chain legitimacy before any install | manual | (blocking human checkpoint — see Manual-Only Verifications) | ❌ W0 | ⬜ pending |
| 07-01-02 | 07-01 | 1 | API-01, API-02, API-05 | — | Three modules pinned into go.mod/go.sum with no unexpected transitive deps | unit | `go build ./... && mage testquick` | ❌ W0 | ⬜ pending |
| 07-02-01 | 07-02 | 2 | API-01 | — | HTTP mutation/query produces the same outcome as the equivalent CLI/Wails call (parity); no path-injection via client-supplied show path | integration | `go test ./internal/api/... -run 'TestParity\|TestCapabilityCoverage\|TestTranslateResult\|TestEmptyCollection\|TestShowPathInjection' -v` | ❌ W0 | ⬜ pending |
| 07-02-02 | 07-02 | 2 | API-01 | — | internal/api never imports internal/command; internal/artnet never imports internal/api (Subsystem injection) | unit | `go test ./internal/artnet/... ./internal/command/... -run 'TestRun\|TestServe\|TestSubsystem' && go build ./...` | ❌ W0 | ⬜ pending |
| 07-03-01 | 07-03 | 3 | API-05 | — | New `api` config concern resolves correctly through internal/projectconfig | unit | `go test ./internal/projectconfig/... && go build ./...` | ❌ W0 | ⬜ pending |
| 07-03-02 | 07-03 | 3 | API-05 | T-remote-bind | Server binds loopback-only when api.remote_enabled is unset/false; remote bind requires the explicit config flag | unit | `go test ./internal/api/... -run 'TestBindAddress\|TestLoopbackDefault\|TestRemoteRequiresInterface' -v` | ❌ W0 | ⬜ pending |
| 07-04-01 | 07-04 | 4 | API-05 | — | Scoped, expiring API keys generated/stored hashed (never raw) and individually revocable | unit | `go test ./internal/show/... ./internal/command/... -run 'TestAPIKey\|TestGenerateAPIKey\|TestLookupAPIKey' -v` | ❌ W0 | ⬜ pending |
| 07-04-02 | 07-04 | 4 | API-05 | — | Auth + coarse domain scope enforcement + per-key rate limit on every mutating route | unit | `go test ./internal/api/... -run 'TestAuth\|TestScope\|TestRateLimit\|TestKeysREST' -v` | ❌ W0 | ⬜ pending |
| 07-05-01 | 07-05 | 5 | API-04, API-01 | T-batch-atomicity | Stale If-Match returns 412; mutation observer seam fires exactly once per mutation | unit + integration | `go test ./internal/api/... ./internal/show/... -run 'TestMutate\|TestRevision\|TestIfMatch\|TestScopeGate\|TestObserver' -v` | ❌ W0 | ⬜ pending |
| 07-05-02 | 07-05 | 5 | API-04 | — | Dry-run mutates nothing, returns would-be impact | unit | `go test ./internal/api/... -run 'TestDryRun' -v` | ❌ W0 | ⬜ pending |
| 07-05-03 | 07-05 | 5 | API-04 | — | Idempotency-Key replays the original result without re-applying the mutation | unit | `go test ./internal/api/... -run 'TestIdempotency' -v` | ❌ W0 | ⬜ pending |
| 07-06-01 | 07-06 | 6 | API-04 | T-batch-atomicity | `/v1/batch` either fully applies (single aggregated Save) or fully rolls back — no partial batch state | unit + integration | `go test ./internal/api/... ./internal/show/... -run 'TestBatchAtomic\|TestBatchRollback\|TestBatchIfMatch\|TestBatchOrder' -v` | ❌ W0 | ⬜ pending |
| 07-06-02 | 07-06 | 6 | API-04 | — | Empty/single-item batches and failure reporting behave correctly | unit | `go test ./internal/api/... -run 'TestBatchEmpty\|TestBatchSingle\|TestBatchFailureReport' -v` | ❌ W0 | ⬜ pending |
| 07-07-01 | 07-07 | 7 | API-06 | T-audit-redaction | New `audit_log` SQLite table writes/reads via the existing single-writer `.golc` store discipline | unit | `go test ./internal/show/... -run 'TestAuditLog\|TestWriteAuditRecord\|TestQueryAuditLog' -v` | ❌ W0 | ⬜ pending |
| 07-07-02 | 07-07 | 7 | API-06 | T-audit-redaction | Every mutation writes exactly one audit_log row with actor/source/correlation/outcome/redacted fields populated | integration | `go test ./internal/api/... -run 'TestAuditObserver\|TestAuditRedaction\|TestAuditOutcomes' -v` | ❌ W0 | ⬜ pending |
| 07-08-01 | 07-08 | 7 | API-03 | — | Global SSE stream emits revisioned events in order; Last-Event-ID replay buffer works | unit | `go test ./internal/api/... -run 'TestSSEOrder\|TestSSEReplay\|TestSSEGapRecovery\|TestSSEBroadcast' -v` | ❌ W0 | ⬜ pending |
| 07-08-02 | 07-08 | 7 | API-03 | — | Client reconnecting with a stale Last-Event-ID past the buffer window receives a resync signal, not silently-missing events; revoked key's open stream is closed on revocation tick | unit | `go test ./internal/api/... -run 'TestSSEAuth\|TestSSECrossScope\|TestSSERevocationTick' -v` | ❌ W0 | ⬜ pending |
| 07-09-01 | 07-09 | 8 | API-02 | — | Generated OpenAPI 3.1 doc has no drift from committed snapshot; generation is deterministic | unit | `go test ./internal/api/... -run 'TestOpenAPIDrift\|TestOpenAPIDeterministic' -v` | ❌ W0 | ⬜ pending |
| 07-09-02 | 07-09 | 8 | API-01, API-02 | — | Full-coverage capability gate closed — every public command route has a REST equivalent | unit | `go test ./internal/api/... -run 'TestCapabilityCoverage\|TestNoPendingRoutes' -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/api/` package and its test files do not exist yet — this entire phase is Wave 0 for test infrastructure.
- [ ] `internal/show/audit_test.go` — covers the new `audit_log` table's write/read path (API-06), following `internal/show/store_test.go`'s existing style.
- [ ] Framework install: none — plain `testing`, no new test framework needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Supply-chain legitimacy of chi, huma/v2, golang.org/x/time before install (T-07-SC) | API-01, API-02, API-05 | Go's module ecosystem has no automated legitimacy scan equivalent to the npm/PyPI/crates tooling this project uses elsewhere; three new direct dependencies in an otherwise offline-pinned repo need a human supply-chain judgment call | Open each module's pkg.go.dev page (chi v5.3.1, huma v2.39.0, golang.org/x/time v0.15.0), confirm none is [SLOP]/[SUS], confirm acceptance of Huma as the D-03 OpenAPI-generation implementation, then type "approved" per 07-01-PLAN.md Task 1's `<resume-signal>` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (07-01-01 is the one manual checkpoint, documented above)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (`internal/api/` and its tests, `internal/show/audit_test.go`)
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-24 (via gsd-plan-checker VERIFICATION PASSED)
