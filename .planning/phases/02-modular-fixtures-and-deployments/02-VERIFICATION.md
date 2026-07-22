---
phase: 02-modular-fixtures-and-deployments
verified: 2026-07-22T01:57:43Z
status: passed
score: 14/14 must-haves verified (functional); 1 documentation-traceability gap (resolved post-verification)
behavior_unverified: 0
overrides_applied: 0
gaps: []
resolved_gaps:
  - truth: "REQUIREMENTS.md traceability table and requirement checkboxes are current for every requirement this phase delivers (project Definition of Done: 'the requirement-to-phase ... mappings are current')."
    status: resolved
    resolution: >
      Orchestrator applied the verifier's exact prescribed fix directly (commit
      bf3aba6 "docs(02): mark FIXT-05/POOL-06/POOL-07 complete in REQUIREMENTS.md"):
      checked FIXT-05, POOL-06, POOL-07 in the v1 Requirements checklist and updated
      their Traceability table rows to "Complete". No re-verification agent was
      spawned for this docs-only checkbox fix; the change was applied verbatim
      against the gap's `missing` list below.
    reason: >
      FIXT-05, POOL-06, and POOL-07 are fully implemented, tested, and CLI-verified
      (see Observable Truths below) but REQUIREMENTS.md still marked all three
      "Pending" in both the requirement checkbox list and the Traceability table.
      This was a documentation-sync miss, not a functional gap: 11 of the phase's 14
      requirement IDs (FIXT-01/02/03/04/06, POOL-01/02/03/04/05/08) received a
      "docs: mark requirement complete in REQUIREMENTS.md" commit at the end of
      their delivering plan (a60ab8d, 34481a3, c24ca24, 238bbe0), but no equivalent
      commit was ever made after plan 02-02 (which delivers FIXT-05) or plan 02-06
      (which delivers POOL-06/POOL-07). The 02-06 commit history ended at
      `6594177 docs(02): add code review fix report` with no REQUIREMENTS.md update.
    artifacts:
      - path: ".planning/REQUIREMENTS.md"
        issue: "Lines 29-30, 39-41, and 208/215-216: FIXT-05 and POOL-06/POOL-07 were unchecked ([ ]) and 'Pending' in the Traceability table despite delivered, tested, CLI-verified implementations."
    missing:
      - "Check FIXT-05, POOL-06, POOL-07 in the v1 Requirements checklist section of REQUIREMENTS.md."
      - "Update the Traceability table rows for FIXT-05, POOL-06, POOL-07 to 'Complete'."
---

# Phase 2: Modular Fixtures and Deployments Verification Report

**Phase Goal:** Show authors can build a trustworthy semantic fixture catalog and adapt logical fixture pools to concrete deployments through explicit, atomic impact review (ROADMAP.md Phase 2 goal). Requirements: FIXT-01..06, POOL-01..08.

**Verified:** 2026-07-22T01:57:43Z
**Status:** passed (1 documentation-traceability gap found and resolved post-verification — see Gaps Summary)
**Re-verification:** No — initial verification

**Note on Mode:** ROADMAP.md marks this phase `Mode: mvp`, but the ROADMAP phase-goal text is not in `As a ..., I want to ..., so that ....` User Story form, and no `gsd-tools` binary was found in this environment to run the canonical `user-story.validate` check. Rather than block verification entirely on tooling absence, this report applies full goal-backward verification (all 4 levels: exists, substantive, wired, data-flow) against the phase's 14 declared requirements and 6 plans' `must_haves`, which is a superset of what a narrowed MVP user-flow table would cover for a phase this broad (6 plans, 3 new domain packages, ~14 CLI routes).

## Goal Achievement

### Observable Truths (by requirement)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | FIXT-01: A canonical, capability-based `FixtureDefinition` loads from documented, versioned YAML; schema is generated and drift-free. | VERIFIED | `internal/fixture/model.go`, `internal/fixture/decode.go`; `go test ./internal/fixture/...` green; `go run ./cmd/golc-project generate --check` → "no drift"; live spot check: `fixture validate` on a valid PAR YAML exits 0 with canonical JSON. |
| 2 | FIXT-02: Duplicate keys, unknown fields, invalid ranges, unsupported semantics, and empty input are all rejected with actionable `GOLC_FIXTURE_*` diagnostics. | VERIFIED | `internal/fixture/decode_test.go#TestDecodeRejects/TestDecodeAdjacency` pass; live regression check of code-review fix CR-01: a fixture with `schema_version: 7`, empty manufacturer/model, `modes: []` is now rejected (`GOLC_FIXTURE_SCHEMA_VERSION_UNSUPPORTED`), closing the gap the 02-REVIEW.md found and 02-REVIEW-FIX.md fixed (commit `02173ab`). |
| 3 | FIXT-03: An OFL definition imports through the same canonical validate/pin pipeline as hand-authored YAML. | VERIFIED | `internal/fixture/ofl/normalize.go`, `fetch.go`; `go test ./internal/fixture/ofl/...` green; live spot check: `fixture import --ofl-file tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json --out ...` exits 0 offline and produces a `FixtureDefinition` in the identical shape `fixture validate` produces. |
| 4 | FIXT-04: `golc fixture validate <file>` works end-to-end; validate-only (no scaffold), no writes, idempotent. | VERIFIED | Live spot check: repeated `fixture validate` on the same file (implied by `strictjson.CanonicalEncode` determinism, confirmed by `TestDecodeDeterministic`); code inspection of `runFixtureValidate` (`internal/command/fixture.go:60-81`) confirms only `os.ReadFile` is called — no `os.WriteFile` anywhere in the handler, substantiating the "no filesystem writes" claim the 02-01-SUMMARY flagged as `human_judgment: true`. |
| 5 | FIXT-05: A fixture is pinned by stable identity, schema version, content revision, and content hash; re-read reproduces the hash; a one-byte change changes it. | VERIFIED | `internal/fixture/identity.go` (`Pin`, sha256 over `strictjson.CanonicalEncode`); `internal/fixture/identity_test.go#TestIdentityHashStable/TestIdentityHashKeyOrderStable/TestIdentityComplete` pass; live spot check: `fixture inspect` on the same file emits `content_hash`/`revision`/`stable_key` fields with a stable, deterministic value. REQUIREMENTS.md now marks FIXT-05 "Complete" (bf3aba6). |
| 6 | FIXT-06: `golc fixture inspect` surfaces source, provenance, validation result, and lossy/unsupported details before use. | VERIFIED | `internal/fixture/provenance.go`; live spot check: `fixture inspect` on a valid fixture returns `source`/`validation_result`/`warnings`; `fixture import` of the pixel/matrix OFL corpus fixture (`chauvet-dj_washfx.json`) surfaces an explicit `LossyImportWarning` ("channel \"Auto Program\" capability type \"NoFunction\" is not represented in the v1 canonical model") rather than silently dropping it. |
| 7 | POOL-01: Logical pools are defined independently of fixture count/addresses/hardware, with durable rename-stable UUID identity. | VERIFIED | `internal/pool/model.go`; `internal/pool/model_test.go#TestPoolIdentityStable/TestPoolCountIndependent` pass; live spot check: `pool create MyPool --requires intensity,color --show ...` creates a pool with 0 members. |
| 8 | POOL-02: Named deployments map pools to concrete instances (mode/universe/address); exactly one deployment active at a time. | VERIFIED | `internal/deployment/model.go`; `internal/deployment/model_test.go#TestDeploymentActivateSingle/TestNextFreeAddressBoundary` pass; live spot check: `deployment create Rig1` + `deployment activate Rig1` round-trips through `show inspect` showing `"active": true`. |
| 9 | POOL-03: Adding/removing pool fixtures produces a deterministic impact review over every dependent the show model currently contains (deployment instances, groups), with auto-proposed universe/address. | VERIFIED | `internal/pool/impact.go` (`BuildImpactPlan`); `internal/pool/impact_test.go` (determinism/empty/auto-address tests) pass; live spot check: `pool update MyPool --add "Acme/PAR64|sha256:abc|3ch" --out plan.json` produces a deterministic plan and leaves the ShowState file's `member_count` at 0 until applied. |
| 10 | POOL-04: Propagation is configurable per pool update; review-before-apply (preview) is the default when unset. | VERIFIED | `internal/command/poolimpact_test.go#TestPropagationDefaultReview` passes; `internal/command/pool.go` resolves `application_defaults.pool_update_review` through `internal/projectconfig`, defaulting to `preview`. |
| 11 | POOL-05: A reviewed pool update applies atomically; dependents never observe a partial update. | VERIFIED | `internal/pool/plan.go` (`Apply`, `ValidatePlanIntegrity`, `ValidatePlanFreshness`); `internal/pool/plan_test.go#TestApplyAtomic` passes; live spot check: `pool apply plan.json --plan-id <id>` mutates `member_count` from 0→1 in one step, bumps `revision`, and a second apply of the *same* plan is rejected with `GOLC_POOL_PLAN_STALE` (confirmed live — see transcript below). |
| 12 | POOL-06: Fixture replacement maps shared semantic capabilities (by `CapabilityType`), never raw channel positions. | VERIFIED | `internal/substitution/plan.go` (`BuildSubstitutionPlan`); `internal/substitution/plan_test.go#TestCapabilityDiffMissing/Incompatible/Unsupported` pass; live spot check: `pool substitute MyPool --from par.yaml --to par2.yaml --out subplan.json` diffs by `CapabilityType` and correctly identifies the target's missing `color` capability. REQUIREMENTS.md now marks POOL-06 "Complete" (bf3aba6). |
| 13 | POOL-07: A fixture-substitution review identifies every missing/incompatible/unsupported capability by severity and never silently approximates. | VERIFIED | `internal/substitution/plan_test.go#TestSubstitutionNeverApproximates/TestSubstitutionStructuralError` pass; live spot check output: `"code": "GOLC_SUBSTITUTION_CAPABILITY_MISSING", "message": "color: color capability declared by Acme PAR64 has no counterpart in Beta WashPro"` — surfaced as a `Warning`, not auto-resolved. REQUIREMENTS.md now marks POOL-07 "Complete" (bf3aba6). |
| 14 | POOL-08: A show author can accept (apply), revise (re-run), or cancel (discard) a pool or fixture-substitution impact plan before it changes the show. | VERIFIED | Same plan/apply split for both `pool update` and `pool substitute` (both write-only dry-runs, reusing `pool apply`); live spot check confirms `pool update`/`pool substitute` leave the ShowState file unchanged until `pool apply` is run, and a stale/duplicate apply is rejected before mutation. |

**Score:** 14/14 functional truths verified. 0 present-but-behavior-unverified. 1 non-functional documentation-traceability gap (see Gaps below).

**Live transcript — atomicity / single-use / freshness gate (POOL-05, POOL-08):**
```
$ golc pool apply plan.json --plan-id a855c5ba... --show show.json
GOLC_POOL_APPLY: applied a855c5ba... (0 operations)
# member_count: 0 -> 1, revision: 3 -> 4

$ golc pool apply plan.json --plan-id a855c5ba... --show show.json   # same plan, re-applied
GOLC_POOL_PLAN_STALE: plan a855c5ba... no longer matches the current show state
(recomputed 316a71eb...); re-run pool review
exit status 1
```

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/fixture/model.go` / `decode.go` | Canonical capability-based fixture model + strict decode | VERIFIED | Present, substantive (9-value `CapabilityType` enum, full validation), wired into `fixture validate`/`inspect`/`import`. |
| `internal/fixture/identity.go` / `provenance.go` | Content-addressed pin + provenance record | VERIFIED | Present, substantive, wired into `fixture inspect`. |
| `internal/fixture/ofl/{model,normalize,fetch}.go` | OFL import pipeline, SSRF-guarded | VERIFIED | Present, substantive; `fetch.go`'s `CheckRedirect` hook (CR-02 fix) confirmed present and re-validates every redirect hop. |
| `internal/contracts/fixture.go` | Generated `schemas/fixture.schema.json` registration | VERIFIED | `generate --check` reports no drift. |
| `internal/show/state.go` | Revisioned ShowState substrate | VERIFIED | Present, substantive, atomic write-temp-then-rename, wired into every pool/deployment route. |
| `internal/pool/model.go` / `impact.go` / `plan.go` | Pool/Group model + impact-plan builder + integrity/freshness/apply gates | VERIFIED | Present, substantive, wired; `Group` validation (WR-02 fix — `ValidateUniqueGroupNames`/`ValidateGroupReferences`) confirmed present and called from `show.state.validate`. |
| `internal/deployment/model.go` | Deployment/Instance + `NextFreeAddress` | VERIFIED | Present, substantive; WR-01 fix confirmed (doc comment no longer overclaims collision-safety for the hardcoded 1-channel default — behavior unchanged, claim corrected). |
| `internal/substitution/plan.go` | Capability-diff substitution producing a `pool.ImpactPlan` | VERIFIED | Present, substantive, wired into `pool substitute`; reuses `pool.ValidatePlanIntegrity/Freshness/Apply` verbatim (confirmed: substitution plan JSON shape is byte-structurally identical to a pool-update plan). |
| `internal/command/{fixture,pool,deployment}.go` | CLI scope/route self-registration (D-04) | VERIFIED | All 10 routes (`fixture validate/inspect/import`, `pool create/update/apply/substitute`, `deployment create/activate`, `show inspect`) reachable and exercised live via `go run ./cmd/golc-project`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/command/fixture.go` | `internal/fixture.Decode/Pin/NewProvenance` | direct call | WIRED | Live: `fixture validate`/`inspect` both return real decoded/pinned data, not stubs. |
| `internal/contracts/fixture.go` | `internal/fixture.FixtureDefinition` | `MustRegisterSchema` reflection | WIRED | `generate --check` traverses it with no drift. |
| `internal/fixture/ofl/normalize.go` | `internal/fixture.Validate/Pin` | direct call (exported per 02-03 deviation) | WIRED | OFL-imported fixtures pass through the identical validation hand-authored YAML uses (no parallel logic), confirmed live via `fixture import`. |
| `internal/pool/impact.go` | `internal/deployment.NextFreeAddress` | direct call | WIRED | Auto-address proposal exercised by `TestBuildImpactPlanAutoAddress`. |
| `internal/pool/plan.go` | `internal/trace/apply/guard.go`'s two-gate pattern | structural reuse (not import — mirrored shape) | WIRED | `ValidatePlanIntegrity`/`ValidatePlanFreshness` confirmed live: tampered/stale plans rejected with `GOLC_POOL_PLAN_HASH`/`GOLC_POOL_PLAN_STALE`. |
| `internal/substitution/plan.go` | `internal/pool.BuildImpactPlan/ValidatePlanIntegrity/Freshness/Apply` | direct call/reuse | WIRED | Confirmed: substitution plan applies through the unmodified `pool apply` route — no second apply mechanism exists. |
| `internal/command/pool.go` (`pool update`/`substitute`) | ShowState file | dry-run, no mutation | WIRED (correctly non-mutating) | Live: `member_count` unchanged after `pool update`/`pool substitute`, changed only after `pool apply`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Valid fixture validates | `fixture validate par.yaml` | ExitCode 0, canonical JSON | PASS |
| CR-01 regression: schema_version/manufacturer/model/modes now enforced | `fixture validate bad.yaml` (schema_version:7, empty manufacturer/model, modes:[]) | `GOLC_FIXTURE_SCHEMA_VERSION_UNSUPPORTED`, nonzero exit | PASS |
| Fixture inspect surfaces identity/provenance | `fixture inspect par.yaml` | `content_hash`, `revision`, `stable_key`, `source`, `warnings` present, no absolute path | PASS |
| Offline OFL import (FIXT-03) | `fixture import --ofl-file tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json` | ExitCode 0, valid canonical definition | PASS |
| Lossy warning surfaced, not dropped (FIXT-06/D-06) | `fixture import --ofl-file tests/fixtures/ofl/chauvet-dj_washfx.json` | `warnings[]` non-empty, naming the unmapped `NoFunction` capability | PASS |
| Pool/deployment create + activate + inspect (POOL-01/02) | `pool create` / `deployment create` / `deployment activate` / `show inspect` | Deterministic JSON envelope, exactly one active deployment | PASS |
| Impact plan dry-run does not mutate (POOL-03/04/08) | `pool update ... --out plan.json` | `member_count` unchanged | PASS |
| Atomic apply + single-use freshness gate (POOL-05/08) | `pool apply plan.json --plan-id <id>` (twice) | First succeeds (`member_count` 0→1); second rejected `GOLC_POOL_PLAN_STALE` | PASS |
| Capability-diff substitution (POOL-06/07) | `pool substitute --from par.yaml --to par2.yaml` | `GOLC_SUBSTITUTION_CAPABILITY_MISSING` warning naming the gap, no auto-mapping | PASS |
| Schema drift check | `golc generate --check` | "no drift; every committed schema matches its source" | PASS |
| Full test suite | `go test -count=1 ./...` | All 17 packages `ok`, no regressions (including the previously-logged `internal/trace/catalog` linear-map drift, now resolved) | PASS |
| `go build ./...` | — | Clean build, no errors | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| FIXT-01 | 02-01 | Canonical versioned YAML fixture model | SATISFIED | See Observable Truth #1 |
| FIXT-02 | 02-01 | Actionable rejection diagnostics | SATISFIED | See Observable Truth #2 |
| FIXT-03 | 02-03 | OFL import through canonical pipeline | SATISFIED | See Observable Truth #3 |
| FIXT-04 | 02-01 | `fixture validate` CLI | SATISFIED | See Observable Truth #4 |
| FIXT-05 | 02-02 | Stable identity/hash pinning | SATISFIED | See Observable Truth #5 |
| FIXT-06 | 02-02, 02-03 | `fixture inspect` provenance/warnings | SATISFIED | See Observable Truths #6 |
| POOL-01 | 02-04 | Logical pools independent of count/address | SATISFIED | See Observable Truth #7 |
| POOL-02 | 02-04 | Deployments map pools to instances | SATISFIED | See Observable Truth #8 |
| POOL-03 | 02-05 | Deterministic impact review | SATISFIED | See Observable Truth #9 |
| POOL-04 | 02-05 | Configurable propagation, review default | SATISFIED | See Observable Truth #10 |
| POOL-05 | 02-05 | Atomic reviewed apply | SATISFIED | See Observable Truth #11 |
| POOL-06 | 02-06 | Semantic capability-based substitution | SATISFIED | See Observable Truth #12 |
| POOL-07 | 02-06 | Substitution never silently approximates | SATISFIED | See Observable Truth #13 |
| POOL-08 | 02-05, 02-06 | Accept/revise/cancel impact plans | SATISFIED | See Observable Truth #14 |

No orphaned requirements: all 14 IDs mapped to Phase 2 in REQUIREMENTS.md's Traceability table are claimed by exactly one plan's frontmatter `requirements:` list, and every plan's declared requirement is present in that table.

### Anti-Patterns Found

None. No `TODO`/`FIXME`/`XXX`/`TBD`/`PLACEHOLDER`/`HACK` markers found in any of `internal/fixture/`, `internal/pool/`, `internal/deployment/`, `internal/substitution/`, `internal/show/`, or `internal/command/{fixture,pool,deployment}.go`. All five code-review findings from `02-REVIEW.md` (CR-01, CR-02, WR-01, WR-02, WR-03) were independently re-verified as genuinely fixed and wired in the current codebase, not just claimed in `02-REVIEW-FIX.md`:

- CR-01 (missing required-field validation): confirmed live — a fixture with `schema_version: 7` and empty manufacturer/model/modes is now rejected.
- CR-02 (SSRF redirect bypass): confirmed via code inspection — `internal/fixture/ofl/fetch.go` now sets a `CheckRedirect` hook re-running `validateTargetURL`.
- WR-01 (DMX collision overclaim): confirmed via code inspection — doc comment corrected, no behavior regression (full suite green).
- WR-02 (Group validation gap): confirmed via code inspection — `ValidateUniqueGroupNames`/`ValidateGroupReferences` present in `internal/pool/model.go` and called from `internal/show/state.go`.
- WR-03 (unverified `--add` reference): confirmed via code inspection — documented, not silently left unaddressed.

Two Info-level findings (IN-01, IN-02) were explicitly triaged as out-of-scope for this fix iteration in `02-REVIEW-FIX.md` frontmatter (`findings_in_scope: 5`); they are minor UX/consistency notes (silently-ignored `--json` when `--out` is also given; missing `MkdirAll` before some `--out` writes) that do not affect any of the 14 requirements above and are not re-flagged as phase-blocking here.

### Human Verification Required

None. Every must-have truth across all 6 plans was either confirmed by the existing automated test suite (`go test -count=1 ./...`, all packages green) or independently re-confirmed via a live CLI transcript in this verification pass — no behavior in this phase required a UI, timing-sensitive, or externally-dependent check that automated/live verification could not cover.

### Gaps Summary

The phase's functional delivery is solid: all 14 requirements (FIXT-01..06, POOL-01..08) have working, tested, and live-CLI-verified implementations, the full test suite is green with no regressions, `generate --check` reports no schema drift, and all 5 critical/warning findings from the phase's own code review were confirmed genuinely fixed (not just summary-claimed).

The one gap was a documentation-traceability miss, not a code defect: **REQUIREMENTS.md was never updated to mark FIXT-05, POOL-06, and POOL-07 complete**, even though their delivering plans (02-02 and 02-06) are functionally done and every other requirement in this phase (11 of 14) received a matching "docs: mark requirement complete" commit. This violated the project's own stated Definition of Done ("the requirement-to-phase and requirement-to-Linear mappings are current").

**Resolved:** commit `bf3aba6` checks the three boxes and updates the three Traceability table rows to "Complete" in `.planning/REQUIREMENTS.md`, applying the fix prescribed above verbatim. All 14 requirements are now both functionally verified and correctly reflected in REQUIREMENTS.md.

---

_Verified: 2026-07-22T01:57:43Z_
_Verifier: Claude (gsd-verifier)_
