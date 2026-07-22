---
phase: 02-modular-fixtures-and-deployments
plan: 06
subsystem: api
tags: [golang, fixture-substitution, capability-diff, impact-plan, cli]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments (plan 05)
    provides: internal/pool.ImpactPlan/BuildImpactPlan/ValidatePlanIntegrity/ValidatePlanFreshness/Apply, "pool update"/"pool apply" CLI split
  - phase: 02-modular-fixtures-and-deployments (plan 01)
    provides: internal/fixture.FixtureDefinition/Capability/CapabilityType, fixture.Decode/Validate/Pin
provides:
  - internal/substitution.BuildSubstitutionPlan: capability-diff (by CapabilityType, never raw channel position) fixture substitution producing the same internal/pool.ImpactPlan shape a pool update produces
  - internal/substitution.CapabilityGapSeverity/CapabilityGap: the D-14 missing/incompatible/unsupported severity taxonomy, carried through pool.ImpactPlan.Warnings
  - "pool substitute" CLI route (dry-run) that writes a substitution ImpactPlan the existing "pool apply" route applies atomically
  - internal/pool.RecomputePlanID (exported): lets a caller-built ImpactPlan with review data BuildImpactPlan cannot itself derive bind its own plan_id through the same canonical-hash mechanism
affects: [phase-03, pool, fixture, deployment]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A second ImpactPlan producer (substitution) reuses pool.BuildImpactPlan's Add/Remove-driven dependent walk directly instead of re-deriving deployment/group effects itself: a substitution is modeled as remove-old-member(s)+add-new-member(s), so the exact same Operations logic 02-05 already proved applies unchanged."
    - "ValidatePlanFreshness's rebuild-and-compare gate now explicitly separates state-dependent freshness (Add/Remove/Propagate/ExpectedRevision/Operations, which BuildImpactPlan can regenerate) from caller-supplied review annotations (Warnings/Errors, which it cannot) -- the rebuilt 'fresh' plan carries the original plan's Warnings/Errors forward before the plan_id comparison, so a second ImpactPlan producer can attach its own review data without re-implementing the freshness gate."
    - "Capability-diff severity taxonomy (missing/incompatible/unsupported) computed by grouping capabilities by CapabilityType, then classifying via range-set coverage (a target's merged ranges must contain every source range) and sub-range-count comparison (finer source granularity than target expresses) -- a GOLC-original design, confirmed via 02-RESEARCH.md Common Pitfalls #2 to have no OFL/GDTF field to borrow."

key-files:
  created:
    - internal/substitution/plan.go
    - internal/substitution/plan_test.go
    - internal/command/substitution_test.go
  modified:
    - internal/command/pool.go
    - internal/pool/plan.go

key-decisions:
  - "BuildSubstitutionPlan derives the added PoolMemberSpec's FixtureStableKey/FixtureContentHash from fixture.Pin(to) directly, not from SubstitutionRequest.ToFixtureRef verbatim -- the request field documents the caller's intended target for audit/traceability, but the plan always reflects the target fixture's own actual computed identity, so a caller can never force a plan to carry a fixture reference inconsistent with the target's real content."
  - "A substitution is modeled as a one-for-one pool-membership replacement: every existing PoolMember whose FixtureStableKey matches SubstitutionRequest.FromFixtureRef is removed, and one new PoolMemberSpec pinned to the target fixture is added per removed member, driven through the exact pool.ImpactRequest{Add,Remove} shape 02-05 already proved -- no new dependent-walk logic was written."
  - "pool.ImpactPlan.Warnings/Errors are populated by BuildSubstitutionPlan after calling pool.BuildImpactPlan (which never derives them), then plan_id is recomputed via the newly-exported pool.RecomputePlanID over the full body including those fields -- so a substitution plan's capability-diff review data is itself tamper-protected by pool.ValidatePlanIntegrity's unmodified self-hash check."
  - "pool.ValidatePlanFreshness was changed (Rule 3 - blocking) to carry the original plan's Warnings/Errors forward onto the rebuilt 'fresh' plan before comparing plan_id, because BuildImpactPlan structurally cannot regenerate a second producer's caller-supplied review annotations; without this, a substitution plan's own capability-diff Warnings would make every freshness check fail as stale immediately after being built, since BuildImpactPlan-driven recompute always produces empty Warnings/Errors."
  - "A target fixture that fails fixture.Decode's own strict decode/validate at the CLI layer ('pool substitute') surfaces immediately as GOLC_SUBSTITUTION_TARGET_INVALID (ExitCode 1) before a plan is even built; BuildSubstitutionPlan itself additionally re-validates an already-decoded target defensively (fixture.Validate(to)) and attaches the same code as a plan.Error, so a caller invoking BuildSubstitutionPlan directly (bypassing the CLI/YAML layer, e.g. a future API/UI caller) still gets the hard-block."

patterns-established:
  - "Capability-diff gaps are carried through pool.Warning's existing Code/Message shape (Code = GOLC_SUBSTITUTION_CAPABILITY_{SEVERITY}, Message = '<capability_type>: <detail>') rather than adding a new structured field to pool.ImpactPlan -- keeps substitution plans byte-shape-identical to pool-update plans."

requirements-completed: [POOL-06, POOL-07, POOL-08]

coverage:
  - id: D1
    description: "BuildSubstitutionPlan diffs source/target fixture capability sets by CapabilityType (never raw DMX channel index) and classifies every gap missing/incompatible/unsupported, never silently approximating any of them."
    requirement: POOL-06
    verification:
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestCapabilityDiffMissing"
        status: pass
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestCapabilityDiffIncompatible"
        status: pass
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestCapabilityDiffUnsupported"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every capability gap surfaces as an explicit, severity-tagged warning (never dropped, merged, or silently resolved); a structural target-fixture problem hard-blocks separately from the accept-past warning list."
    requirement: POOL-07
    verification:
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestSubstitutionNeverApproximates"
        status: pass
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestSubstitutionStructuralError"
        status: pass
      - kind: unit
        ref: "internal/command/substitution_test.go#TestPoolSubstituteTargetInvalid"
        status: pass
    human_judgment: false
  - id: D3
    description: "A substitution impact plan is reviewed and accepted (apply)/revised (re-run with a different target)/cancelled (discard) as a single all-or-nothing unit, reusing pool.ValidatePlanIntegrity/ValidatePlanFreshness/Apply unmodified in call shape (D-16), through the existing pool apply CLI route (no second apply mechanism)."
    requirement: POOL-08
    verification:
      - kind: unit
        ref: "internal/substitution/plan_test.go#TestSubstitutionAtomicAcceptCancel"
        status: pass
      - kind: unit
        ref: "internal/command/substitution_test.go#TestPoolSubstituteRoute"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-07-21
status: complete
---

# Phase 2 Plan 6: Semantic Fixture Substitution Summary

**Capability-diff fixture substitution (missing/incompatible/unsupported severity taxonomy, keyed by `fixture.CapabilityType` not raw channel position) that reuses `internal/pool`'s `ImpactPlan`/`ValidatePlanIntegrity`/`ValidatePlanFreshness`/`Apply` contract verbatim, exposed through a `pool substitute` (dry-run) route that the existing `pool apply` route applies atomically.**

## Performance

- **Duration:** 45 min
- **Tasks:** 3
- **Files modified:** 5 (3 created, 2 modified)

## Accomplishments
- `internal/substitution.BuildSubstitutionPlan` diffs a source/target `fixture.FixtureDefinition` pair by `CapabilityType`, classifying every gap the target cannot fully represent as `missing` (type absent), `incompatible` (target's range(s) don't cover the source's), or `unsupported` (target's range(s) cover the source's, but the source declares finer-grained sub-ranges the target flattens) -- a GOLC-original taxonomy design since neither OFL nor GDTF define a comparable field.
- Every capability gap is carried as a `pool.Warning` on the returned `pool.ImpactPlan`; a structurally invalid target fixture is instead a hard-blocking `pool.Error` (`GOLC_SUBSTITUTION_TARGET_INVALID`), distinct from the accept-past warnings -- `pool.Apply` already refuses any plan carrying an Error.
- A substitution is modeled as a one-for-one pool-membership replacement (remove every member matching the source fixture ref, add one new member pinned to the target's own `fixture.Pin` identity per removed member), driven through `pool.BuildImpactPlan`'s existing `Add`/`Remove` dependent walk -- no new deployment-instance/group-membership logic was written.
- `pool substitute <pool> --from <file> --to <file> [--out <path>] [--json] --show <path>` self-registers under the existing `pool` scope (D-04), decodes both fixtures through `fixture.Decode` (the same FIXT-01/02 pipeline `fixture validate` uses), builds the review, and writes/prints it without ever mutating the ShowState file; the existing, unmodified `pool apply` route applies the resulting plan atomically.
- `internal/pool.ValidatePlanFreshness` was extended (see Deviations) so a second `ImpactPlan` producer's caller-supplied review data (Warnings/Errors) can ride along on top of the state-dependent freshness check without a rebuild-blind mismatch, and a new exported `pool.RecomputePlanID` lets that producer bind its own plan_id through the exact same canonical-hash mechanism `ValidatePlanIntegrity` uses internally.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing tests for capability-diff severity taxonomy + substitute route** - `12fa6b9` (test)
2. **Task 2: Capability-diff substitution plan reusing the pool ImpactPlan contract** - `a0474bc` (feat)
3. **Task 3: pool substitute route (dry-run) reusing pool apply** - `ce897b8` (feat)

**Plan metadata:** committed separately by the orchestrator after the wave completes (worktree mode).

## Files Created/Modified
- `internal/substitution/plan.go` - `CapabilityGapSeverity`/`CapabilityGap`/`SubstitutionRequest`/`BuildSubstitutionPlan`: capability-diff engine producing a `pool.ImpactPlan`
- `internal/substitution/plan_test.go` - severity-taxonomy, never-approximates, structural-error, and integrity/freshness/apply-gate-reuse tests
- `internal/command/substitution_test.go` - `pool substitute` dry-run + `pool apply` atomic-apply route contract, plus a target-invalid CLI error case
- `internal/command/pool.go` - `pool substitute` route registration, arg parsing, and handler
- `internal/pool/plan.go` - exported `RecomputePlanID`; `ValidatePlanFreshness` now carries the original plan's Warnings/Errors forward onto the rebuilt "fresh" plan before comparing plan_id

## Decisions Made
See `key-decisions` in frontmatter: identity-derived-not-caller-trusted Add specs, one-for-one membership replacement reusing pool's existing Add/Remove walk, post-hoc Warnings/Errors attachment with a recomputed plan_id, the `ValidatePlanFreshness` generalization, and the two-layer (CLI + library) structural target-invalid hard-block.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Generalized `pool.ValidatePlanFreshness` and added `pool.RecomputePlanID` so a substitution plan's own review data doesn't break freshness/integrity**
- **Found during:** Task 2 (writing `internal/substitution/plan.go`)
- **Issue:** `pool.ImpactPlan`'s `plan_id` is a SHA-256 over the plan's full body, which already includes `Warnings`/`Errors` (`pool/impact.go`'s `planBody`). A substitution plan needs non-empty `Warnings` (capability gaps) and sometimes `Errors` (structural target-invalid) to satisfy POOL-07's "never silently approximated" and D-14's hard-block requirements. But `pool.BuildImpactPlan` -- the function both `ValidatePlanFreshness`'s rebuild step and `plan_id` computation for a plain pool update rely on -- never derives `Warnings`/`Errors` itself (always empty). Literally reusing `pool.ValidatePlanFreshness` unmodified against a substitution plan with non-empty `Warnings`/`Errors` would make its rebuild-and-compare `plan_id` check fail as stale immediately after the plan is built, since the rebuilt "fresh" plan can never reproduce the capability-diff data -- there is no code path for `ValidatePlanFreshness` to receive fixture data at all (its signature only takes pool/deployment/group slices + revision). This is a structural incompatibility inherent to reusing one shared `ImpactPlan` type for two different producers (pool update vs. substitution), not something `internal/substitution/plan.go` alone could work around.
- **Fix:** `internal/pool/plan.go`'s `ValidatePlanFreshness` now carries the original plan's `Warnings`/`Errors` forward onto the freshly-rebuilt plan before computing and comparing `plan_id`, scoping the freshness check to the state-dependent computation (`Add`/`Remove`/`Propagate`/`ExpectedRevision`/`Operations`) that `BuildImpactPlan` can actually regenerate. `ValidatePlanIntegrity` (the tamper-check gate, always run first) is unchanged and still hashes a plan's `Warnings`/`Errors` exactly as stored, so tampering with those fields is still caught before `ValidatePlanFreshness` ever runs. A new exported `pool.RecomputePlanID` (a one-line wrapper around the existing private `computePlanID(bodyOf(plan))`) lets `internal/substitution` bind its own `plan_id` -- after attaching capability-diff `Warnings`/structural `Errors` -- through the exact same canonical-hash mechanism `ValidatePlanIntegrity` uses internally, rather than hand-rolling a second hash implementation (D-16).
- **Files modified:** internal/pool/plan.go
- **Verification:** `go test ./internal/pool/...` (all pre-existing 02-05 tests, including `TestPlanFreshnessRejectsStale`/`TestPlanIntegrityRejectsTamper`/`TestApplyAtomic`) still pass unchanged -- the freshness change is a no-op for a plain pool update, where `Warnings`/`Errors` are always empty on both sides. `internal/substitution/plan_test.go#TestSubstitutionAtomicAcceptCancel` proves a substitution plan with empty `Warnings`/`Errors` passes both gates and a tampered/stale one is still rejected with the same `GOLC_POOL_PLAN_HASH`/`GOLC_POOL_PLAN_STALE` codes.
- **Committed in:** a0474bc (Task 2 commit)

**2. [Rule 2 - Missing Critical] Added a "source fixture ref not found in pool" guard**
- **Found during:** Task 2 (writing `internal/substitution/plan.go`)
- **Issue:** The plan's declared diagnostic-code list (`GOLC_SUBSTITUTION_TARGET_INVALID`, `GOLC_SUBSTITUTION_USAGE`) has no code for a `SubstitutionRequest.FromFixtureRef` that matches zero existing pool members -- without a guard, this would silently build a plan with zero `Remove`/`Add` entries and no operations, giving no indication the requested substitution never matched anything.
- **Fix:** `BuildSubstitutionPlan` returns a Go error (`GOLC_SUBSTITUTION_SOURCE_NOT_FOUND`) when no pool member's `FixtureStableKey` matches `FromFixtureRef`, following the same `fmt.Errorf`-return convention `pool.BuildImpactPlan` already uses for its own unknown-pool/unknown-member guards.
- **Files modified:** internal/substitution/plan.go
- **Verification:** Exercised implicitly by every passing test in `internal/substitution/plan_test.go` (all use a `FromFixtureRef` that does match the fixture's seeded member); no test currently exercises the not-found path directly since it wasn't a named behavior in the plan, but the guard mirrors an established repo convention rather than introducing a new one.
- **Committed in:** a0474bc (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing-critical)
**Impact on plan:** The `ValidatePlanFreshness` generalization was necessary for the plan's own explicit requirement ("the substitution plan reuses pool integrity/freshness gates... a tampered/stale substitution plan is rejected") to be satisfiable at all, given `pool.ImpactPlan`'s existing full-body hash already includes `Warnings`/`Errors`; it is fully backward-compatible (verified against the complete pre-existing `internal/pool` test suite). The source-not-found guard is a small, convention-consistent safety addition. No scope creep beyond what D-16's literal "reuse... do not build a second one" instruction required to actually hold together for a second `ImpactPlan` producer.

## Issues Encountered
None beyond the two deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/substitution.BuildSubstitutionPlan` and the `pool substitute`/`pool apply` route pair complete the phase's three impact-review surfaces (pool update, pool apply, pool substitute) all sharing the exact same `pool.ImpactPlan` integrity/freshness/atomic-apply contract.
- `pool.RecomputePlanID` and the generalized `ValidatePlanFreshness` are now available for any future `ImpactPlan` producer (e.g. a later phase's dependent-category extension) that needs to attach its own review data the same way.
- No blockers for phase completion.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-21*

## Self-Check: PASSED

All created files (internal/substitution/plan.go, internal/substitution/plan_test.go, internal/command/substitution_test.go, this SUMMARY.md) and all commit hashes (12fa6b9, a0474bc, ce897b8) were verified present. `internal/pool/plan.go` and `internal/command/pool.go` modifications were verified via `go build ./...` and the full `go test ./internal/...` suite passing.
