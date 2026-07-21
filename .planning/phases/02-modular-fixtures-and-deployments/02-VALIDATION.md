---
phase: 2
slug: modular-fixtures-and-deployments
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-21
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
| TBD | TBD | TBD | FIXT-01 | — | Loads a valid YAML fixture definition into the canonical `FixtureDefinition` model | unit | `go test ./internal/fixture/... -run TestLoad` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | FIXT-02 | T-FIXT-YAML | Rejects duplicate keys / unknown fields / invalid ranges with actionable `GOLC_FIXTURE_*` diagnostics; duplicate-key rejection prevents key-override tampering | unit (table-driven) | `go test ./internal/fixture/... -run TestDecodeRejects` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | FIXT-03 | T-FIXT-OFL | Imports an OFL fixture through the same canonical normalization + validation pipeline as hand-authored YAML | unit + fixture-corpus | `go test ./internal/fixture/ofl/... -run TestImport` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | FIXT-04 | — | Validates a hand-authored custom fixture via CLI (`golc fixture validate <file>`) | integration (CLI route) | `go test ./internal/command/... -run TestFixtureValidateRoute` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | FIXT-05 | — | Stable identity/hash pinning (sha256 over `strictjson.CanonicalEncode`) survives re-read | unit (round-trip) | `go test ./internal/fixture/... -run TestIdentityHashStable` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | FIXT-06 | T-FIXT-OFL | Provenance/warning inspection surfaces lossy/unsupported import details before use, never silently | unit | `go test ./internal/fixture/... -run TestProvenance` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | POOL-01, POOL-02 | — | Defines a logical pool independent of count/address, maps it to concrete instances/modes/universes/addresses in a deployment | unit | `go test ./internal/pool/... ./internal/deployment/...` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | POOL-03, POOL-04, POOL-05 | T-POOL-PLAN | Impact review computation + configurable propagation (review-required default) + atomic apply guarded by integrity/freshness checks | unit + property (`pgregory.net/rapid`) | `go test ./internal/pool/... -run TestImpactPlan` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | POOL-06, POOL-07 | — | Capability-based substitution diff; missing/incompatible/unsupported severity taxonomy surfaced, never silently approximated | unit | `go test ./internal/substitution/... -run TestCapabilityDiff` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | POOL-08 | T-POOL-PLAN | Accept/revise/cancel an impact plan atomically before it changes the show; stale plan rejected with clear message | integration (CLI dry-run/apply route pair) | `go test ./internal/command/... -run TestPoolApplyRoute` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/fixture/` package + tests — no fixture domain code exists yet (greenfield)
- [ ] `internal/pool/`, `internal/deployment/`, `internal/substitution/` packages + tests — greenfield
- [ ] A small local OFL fixture-corpus under `tests/fixtures/ofl/` (a handful of real, pinned OFL JSON files) so FIXT-03 tests don't depend on live network access
- [ ] Decide and pin the specific `go.yaml.in/yaml/v4` version before writing FIXT-02 decode tests (RESEARCH.md Pitfall 3 — rc.2 is currently pinned indirectly, rc.6 available upstream)
- [ ] `schemas/fixture.schema.json` generated via `invopop/jsonschema` from the canonical `FixtureDefinition` struct

---

## Manual-Only Verifications

*None — all phase behaviors identified in research have automated verification paths. If planning surfaces a behavior that genuinely cannot be automated (e.g. a physical DMX validation step), add it here.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s (task) / 120s (wave)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
