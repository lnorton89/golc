---
phase: 2
slug: modular-fixtures-and-deployments
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-21
validated: 2026-07-22
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go 1.26.5 `testing` package + this repo's own `internal/command` "test" route wrapper |
| **Config file** | none — driven by `golc.project.toml`/`config/commands.toml` (`go_version = "ref:toolchain.go.version"`) |
| **Quick run command** | `golc test --quick` |
| **Full suite command** | `golc test` (bare) |
| **Estimated runtime** | quick ≤ 30 seconds; full ≤ 120 seconds (consistent with Phase 1's budget) |

---

## Sampling Rate

- **After every task commit:** Run `golc test --quick`
- **After every plan wave:** Run `golc test` (full suite)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds per task sample, 120 seconds per wave gate

---

## Per-Task Verification Map

Draft, requirement-level seed (no PLAN.md exists yet — Task ID/Plan/Wave columns are filled by the planner/validate-phase once tasks exist). Every phase requirement below must map to at least one automated test.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01 | 02-01 | 1 | FIXT-01 | — | Loads a valid YAML fixture definition into the canonical `FixtureDefinition` model | unit | `go test ./internal/fixture/... -run TestLoad` | ✅ `internal/fixture/decode_test.go` | ✅ green |
| 02-01 | 02-01 | 1 | FIXT-02 | T-FIXT-YAML | Rejects duplicate keys / unknown fields / invalid ranges with actionable `GOLC_FIXTURE_*` diagnostics; duplicate-key rejection prevents key-override tampering | unit (table-driven) | `go test ./internal/fixture/... -run TestDecodeRejects` | ✅ `internal/fixture/decode_test.go` | ✅ green |
| 02-03 | 02-03 | 3 | FIXT-03 | T-FIXT-OFL | Imports an OFL fixture through the same canonical normalization + validation pipeline as hand-authored YAML | unit + fixture-corpus | `go test ./internal/fixture/ofl/... -run TestNormalizeCanonicalPipeline\|TestNormalizeCorpusFixturesAllImport` | ✅ `internal/fixture/ofl/normalize_test.go` | ✅ green |
| 02-01 | 02-01 | 1 | FIXT-04 | — | Validates a hand-authored custom fixture via CLI (`golc fixture validate <file>`) | integration (CLI route) | `go test ./internal/command/... -run TestFixtureValidateRoute` | ✅ `internal/command/fixture_test.go` | ✅ green |
| 02-02 | 02-02 | 2 | FIXT-05 | — | Stable identity/hash pinning (sha256 over `strictjson.CanonicalEncode`) survives re-read | unit (round-trip) | `go test ./internal/fixture/... -run TestIdentityHashStable\|TestIdentityHashKeyOrderStable` | ✅ `internal/fixture/identity_test.go` | ✅ green |
| 02-02, 02-03 | 02-02, 02-03 | 2, 3 | FIXT-06 | T-FIXT-OFL | Provenance/warning inspection surfaces lossy/unsupported import details before use, never silently | unit | `go test ./internal/fixture/... -run TestProvenance` + `go test ./internal/fixture/ofl/... -run TestNormalizeLossyWarning\|TestNormalizeNoSilentDrop` | ✅ `internal/fixture/provenance_test.go`, `internal/fixture/ofl/normalize_test.go` | ✅ green |
| 02-04 | 02-04 | 2 | POOL-01, POOL-02 | — | Defines a logical pool independent of count/address, maps it to concrete instances/modes/universes/addresses in a deployment | unit | `go test ./internal/pool/... -run TestPoolIdentityStable\|TestPoolCountIndependent` + `go test ./internal/deployment/... -run TestDeploymentActivateSingle\|TestNextFreeAddressBoundary` | ✅ `internal/pool/model_test.go`, `internal/deployment/model_test.go` | ✅ green |
| 02-05 | 02-05 | 3 | POOL-03, POOL-04, POOL-05 | T-POOL-PLAN | Impact review computation + configurable propagation (review-required default) + atomic apply guarded by integrity/freshness checks | unit (table-driven; delivered as plain unit tests, not property-based — see audit trail) | `go test ./internal/pool/... -run TestBuildImpactPlanDeterministic\|TestBuildImpactPlanAutoAddress\|TestApplyAtomic\|TestPlanIntegrityRejectsTamper\|TestPlanFreshnessRejectsStale` + `go test ./internal/command/... -run TestPropagationDefaultReview` | ✅ `internal/pool/impact_test.go`, `internal/pool/plan_test.go`, `internal/command/poolimpact_test.go` | ✅ green |
| 02-06 | 02-06 | 4 | POOL-06, POOL-07 | — | Capability-based substitution diff; missing/incompatible/unsupported severity taxonomy surfaced, never silently approximated | unit | `go test ./internal/substitution/... -run TestCapabilityDiffMissing\|TestCapabilityDiffIncompatible\|TestCapabilityDiffUnsupported\|TestSubstitutionNeverApproximates\|TestSubstitutionStructuralError` | ✅ `internal/substitution/plan_test.go` | ✅ green |
| 02-05, 02-06 | 02-05, 02-06 | 3, 4 | POOL-08 | T-POOL-PLAN | Accept/revise/cancel an impact plan atomically before it changes the show; stale plan rejected with clear message | integration (CLI dry-run/apply route pair) | `go test ./internal/command/... -run TestPoolUpdateApplyRoutes\|TestPoolSubstituteRoute\|TestSubstitutionAtomicAcceptCancel` | ✅ `internal/command/poolimpact_test.go`, `internal/command/substitution_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/fixture/` package + tests — delivered in plan 02-01/02-02
- [x] `internal/pool/`, `internal/deployment/`, `internal/substitution/` packages + tests — delivered in plans 02-04/02-05/02-06
- [x] A small local OFL fixture-corpus under `tests/fixtures/ofl/` — delivered in plan 02-03; exercised by `TestNormalizeCorpusFixturesAllImport`
- [x] `go.yaml.in/yaml/v4` version pinned before FIXT-02 decode tests were written — resolved in plan 02-01
- [x] `schemas/fixture.schema.json` generated via `invopop/jsonschema` — `go run ./cmd/golc-project generate --check` reports no drift (confirmed in 02-VERIFICATION.md)

---

## Manual-Only Verifications

*None — all phase behaviors identified in research have automated verification paths. If planning surfaces a behavior that genuinely cannot be automated (e.g. a physical DMX validation step), add it here.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s (task) / 120s (wave) — full targeted suite ran in ~0.3s live
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-07-22

---

## Validation Audit 2026-07-22

This VALIDATION.md was seeded as a draft during plan-phase (before any plan existed) and was never updated after execution completed. Retroactive audit against the 6 delivered plans, their SUMMARY.md files, and 02-VERIFICATION.md (passed, 14/14 requirements, full suite green) found the underlying test coverage was already complete — only this document was stale.

| Metric | Count |
|--------|-------|
| Requirements audited | 14 |
| Gaps found (missing/failing tests) | 0 |
| Resolved | 0 |
| Escalated to manual-only | 0 |
| Document-only corrections (Task ID/Plan/Wave/File Exists/Status columns, frontmatter, Wave 0 checklist) | 14 |

**Notes:**
- All 14 requirements (FIXT-01..06, POOL-01..08) map to real, existing, passing test functions. Live re-run of `go test -count=1 ./internal/fixture/... ./internal/fixture/ofl/... ./internal/pool/... ./internal/deployment/... ./internal/substitution/... ./internal/command/...` on 2026-07-22 confirms all packages `ok`.
- Several draft-suggested test names (e.g. `TestImport`, `TestImpactPlan`, `TestPoolApplyRoute`) don't match the actual delivered test function names 1:1; the map above has been corrected to the real names.
- POOL-03/04/05 were originally scoped for `unit + property (pgregory.net/rapid)` testing (see draft row and RESEARCH.md); `pgregory.net/rapid` was never added to `go.mod`, and the delivered tests are plain table-driven unit tests (`TestBuildImpactPlanDeterministic`, `TestBuildImpactPlanAutoAddress`, etc.). Determinism and boundary behavior are still directly and adequately tested — this is a downgrade from the originally suggested test *style*, not a coverage gap, and is not treated as blocking.
