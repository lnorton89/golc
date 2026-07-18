---
phase: 1
slug: offline-foundation-and-delivery-traceability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-17
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go 1.26.5 `testing` and Node 24.18.0 `node:test` |
| **Config file** | none — Wave 0 creates the Go and Node manifests plus test harnesses |
| **Quick run command** | `powershell -NoProfile -File .\golc.ps1 test --quick` |
| **Full suite command** | `powershell -NoProfile -File .\golc.ps1 test` |
| **Estimated runtime** | quick ≤ 30 seconds; full ≤ 120 seconds |

Additional phase gates:

- Offline acceptance: `powershell -NoProfile -File .\golc.ps1 check --offline`
- Generated-artifact drift: `powershell -NoProfile -File .\golc.ps1 generate --check`

---

## Sampling Rate

- **After every task commit:** Run `powershell -NoProfile -File .\golc.ps1 test --quick`.
- **After every plan wave:** Run `powershell -NoProfile -File .\golc.ps1 generate --check`, `powershell -NoProfile -File .\golc.ps1 check --offline`, and `powershell -NoProfile -File .\golc.ps1 test`.
- **Before `$gsd-verify-work`:** Generation must be clean; offline check, full tests, foundation build, and package checks must pass.
- **Max feedback latency:** 30 seconds for task sampling and 120 seconds for a wave gate.
- **External smoke:** One reviewed real Linear preview/apply/replay is recorded when credentials and taxonomy are available; unavailability remains pending and cannot fail offline acceptance.

---

## Per-Task Verification Map

Task IDs and plan/wave assignments are finalized by the Phase 1 planner. These requirement-level rows are the minimum validation obligations each resulting task map must preserve.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-CONF-01 | TBD | TBD | CONF-01 | T-01 / T-02 | Pinned downloads are verified and configuration paths remain repository-contained | acceptance + schema | `.\golc.ps1 check --concern project` | ❌ W0 | ⬜ pending |
| TBD-CONF-02 | TBD | TBD | CONF-02 | T-02 / T-03 | Unknown, duplicate-authority, invalid, cyclic, and unsafe-path input fails closed | unit + golden | `.\golc.ps1 test --quick --scope config` | ❌ W0 | ⬜ pending |
| TBD-CONF-03 | TBD | TBD | CONF-03 | T-01 / T-09 | Contributors and CI use the same command graph; PR jobs cannot mutate Linear | acceptance | `.\golc.ps1 check --command-parity` | ❌ W0 | ⬜ pending |
| TBD-CONF-04 | TBD | TBD | CONF-04 | T-04 | Secret values never enter logs, plans, maps, artifacts, or generated contracts | canary + repository scan | `.\golc.ps1 test --quick --scope secrets` | ❌ W0 | ⬜ pending |
| TBD-LINR-01 | TBD | TBD | LINR-01 | T-03 / T-05 | Stable IDs and parent/source links reject unauthorized override or malformed graphs | unit + fixture | `.\golc.ps1 linear validate --offline` | ❌ W0 | ⬜ pending |
| TBD-LINR-02 | TBD | TBD | LINR-02 | Credential-free maps contain nullable UUID links and preserve repository authority | schema + migration | `.\golc.ps1 test --quick --scope linear-map` | ❌ W0 | ⬜ pending |
| TBD-LINR-03 | TBD | TBD | LINR-03 | Exact previews are hash-bound, stale-safe, resumable, and duplicate-resistant | fake GraphQL integration + golden | `.\golc.ps1 test --quick --scope linear-reconcile` | ❌ W0 | ⬜ pending |
| TBD-LINR-04 | TBD | TBD | LINR-04 | Pagination, partial errors, ambiguity, and rate limits fail safely without exposing secrets or blocking offline work | Node contract + Go integration | `.\golc.ps1 test --quick --scope linear-transport` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` and `go.sum` — Go command, TOML parser, schema generator, and test dependencies.
- [ ] `cmd/golc-project/main.go` — normal command entrypoint exercised through `golc.ps1`.
- [ ] `internal/projectconfig/*_test.go` — strict concerns, precedence, provenance, deprecation, and path safety.
- [ ] `internal/trace/catalog/*_test.go` — local identity graph and schema-1-to-2 migration.
- [ ] `internal/trace/reconcile/*_test.go` — three-way merge and canonical plan goldens.
- [ ] `internal/trace/apply/*_test.go` — stale, replay, and partial-apply state machine.
- [ ] `internal/contracts/generate_test.go` — deterministic schema generation and drift checks.
- [ ] `tools/linear-sync/package.json`, exact lockfile, and `tsconfig.json` — isolated Linear SDK adapter.
- [ ] `tools/linear-sync/test/*.test.ts` — pagination, partial errors, rate limits, uncertain mutations, and redaction.
- [ ] `tests/fixtures/config/` and `tests/fixtures/linear/` — adversarial configuration and GraphQL transcripts.
- [ ] `tests/golden/` — generated schemas, provenance output, preview plans, and reports.
- [ ] `.github/workflows/check.yml` — Windows CI invoking the same root commands without PR mutation.

Required cases include a corrupt bootstrap checksum, idempotent second bootstrap, fail-on-network offline execution, every configuration precedence pair, stable IDs across display renames, a 51-item paginated Linear collection, GraphQL `data` plus `errors`, timeout-after-create recovery, rate-limited partial apply/resume, stale preview rejection, explicit archive/unlink review, and a fake-secret canary scan over every emitted byte.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Reviewed real Linear preview/apply/replay | LINR-03, LINR-04 | Requires an explicitly configured workspace, protected credential, and approved taxonomy | Perform a read-only schema/taxonomy check; review the exact preview; approve one apply; replay the same plan; verify one remote object per local ID and a credential-free map. Never run from automated PR CI. |

---

## Validation Sign-Off

- [ ] Every planned task has an `<automated>` verification or an explicit Wave 0 dependency.
- [ ] Sampling continuity has no three consecutive tasks without automated verification.
- [ ] Wave 0 covers every missing test reference above.
- [ ] No command uses watch mode.
- [ ] Quick feedback latency is under 30 seconds and full feedback is under 120 seconds.
- [ ] The fake-secret canary is absent from stdout, stderr, files, reports, maps, errors, and generated artifacts.
- [ ] Offline acceptance passes without `.env`, credentials, or network access after bootstrap.
- [ ] `nyquist_compliant: true` and `wave_0_complete: true` are set only after the planner assigns concrete task IDs and execution creates the infrastructure.

**Approval:** pending
