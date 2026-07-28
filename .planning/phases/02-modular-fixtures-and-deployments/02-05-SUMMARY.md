---
phase: 02-modular-fixtures-and-deployments
plan: 05
subsystem: api
tags: [golang, pool, deployment, impact-plan, apply-guard, cli]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments (plan 04)
    provides: internal/pool.Pool/Group, internal/deployment.Deployment/Instance/NextFreeAddress, internal/show.State/Load/Save, "pool create"/"deployment create"/"deployment activate"/"show inspect" routes
provides:
  - internal/pool.BuildImpactPlan: deterministic pool add/remove impact review over deployment instances and cross-pool groups, with D-11 auto-proposed next-free universe/address
  - internal/pool.ValidatePlanIntegrity / ValidatePlanFreshness: two-gate plan contract mirroring internal/trace/apply/guard.go (GOLC_POOL_PLAN_SCHEMA/HASH/STALE)
  - internal/pool.Apply: atomic, all-or-nothing pool/deployment/group mutation minting new UUIDs only at apply time
  - "pool update" (dry-run plan/apply split) and "pool apply" (integrity+freshness+atomic apply) CLI routes with configurable propagation default (application_defaults.pool_update_review)
affects: [phase-03, pool, deployment, show]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Plan/apply split (D-15) reused a third time: linear preview/apply, then pool create dry-run precedent, now pool update (compute+write, never mutate) / pool apply (validate+atomic apply)."
    - "Two-gate plan contract (ValidatePlanIntegrity self-hash recompute, then ValidatePlanFreshness rebuild-and-compare) copied verbatim in shape from internal/trace/apply/guard.go into a second domain (pool), proving the pattern generalizes."
    - "Functions in a domain package that would otherwise take a cross-package aggregate type (show.State) instead take that aggregate's plain component fields, to avoid an import cycle when the aggregate's own package already imports the domain package."

key-files:
  created:
    - internal/pool/impact.go
    - internal/pool/impact_test.go
    - internal/pool/plan.go
    - internal/pool/plan_test.go
    - internal/command/poolimpact_test.go
  modified:
    - internal/command/pool.go

key-decisions:
  - "BuildImpactPlan/ValidatePlanFreshness/Apply take plain []pool.Pool/[]deployment.Deployment/[]pool.Group/revision-int parameters instead of a literal show.State parameter, because internal/show already imports internal/pool (State embeds []pool.Pool/[]pool.Group) -- a show.State parameter in package pool would be a direct import cycle."
  - "--add's CLI value uses '|' as the field separator (<fixture_stable_key>|<fixture_content_hash>|<mode>), not ':', because a realistic content hash already carries its own algorithm prefix (e.g. sha256:...) that would collide with a ':'-delimited grammar."
  - "A newly added fixture only proposes a deployment instance in a deployment that has already adopted the pool (has at least one existing Instance referencing that PoolID); a deployment with zero instances from the pool is not treated as a dependent, matching the plan's 'zero dependent operations for a pool with no dependents, not an error' requirement."
  - "New PoolMember and Instance UUIDs are minted only inside Apply, never inside BuildImpactPlan: the hashed plan body only ever contains fixture specs (Add) and existing member IDs (Remove), so two BuildImpactPlan calls against identical inputs always produce an identical plan_id."
  - "application_defaults.pool_update_review is resolved through the existing five-layer internal/projectconfig.ResolveKey/DefaultRegistry (locked key; committed default 'preview'); --propagate overrides the resolved value for one invocation only, without writing through the locked configuration layer."

patterns-established:
  - "Dependent-category walk is a fixed ordered traversal (deployment instances, then groups) over the caller's own already-deterministic slices; a future phase's dependent types (themes/scenes/chases/...) extend the same walk without a rewrite."

requirements-completed: [POOL-03, POOL-04, POOL-05, POOL-08]

coverage:
  - id: D1
    description: "BuildImpactPlan computes a deterministic pool add/remove impact plan over deployment instances and cross-pool groups, auto-proposing next-free universe/address for new instances (D-11)."
    requirement: POOL-03
    verification:
      - kind: unit
        ref: "internal/pool/impact_test.go#TestBuildImpactPlanDeterministic"
        status: pass
      - kind: unit
        ref: "internal/pool/impact_test.go#TestBuildImpactPlanEmpty"
        status: pass
      - kind: unit
        ref: "internal/pool/impact_test.go#TestBuildImpactPlanAutoAddress"
        status: pass
    human_judgment: false
  - id: D2
    description: "Propagation mode is configurable per pool update (immediate|preview), resolved from application_defaults.pool_update_review, with review-required (preview) as the default when unset."
    requirement: POOL-04
    verification:
      - kind: unit
        ref: "internal/command/poolimpact_test.go#TestPropagationDefaultReview"
        status: pass
    human_judgment: false
  - id: D3
    description: "A reviewed pool update applies atomically (all-or-nothing) via ValidatePlanIntegrity -> ValidatePlanFreshness -> Apply -> Save; a stale or tampered re-apply is rejected."
    requirement: POOL-05
    verification:
      - kind: unit
        ref: "internal/pool/plan_test.go#TestApplyAtomic"
        status: pass
      - kind: unit
        ref: "internal/pool/plan_test.go#TestPlanFreshnessRejectsStale"
        status: pass
      - kind: unit
        ref: "internal/pool/plan_test.go#TestPlanIntegrityRejectsTamper"
        status: pass
      - kind: unit
        ref: "internal/command/poolimpact_test.go#TestPoolUpdateApplyRoutes"
        status: pass
    human_judgment: false
  - id: D4
    description: "pool update (dry-run) / pool apply CLI routes implement accept (apply)/revise (re-run pool update)/cancel (discard the plan file) via the D-15 plan/apply split."
    requirement: POOL-08
    verification:
      - kind: unit
        ref: "internal/command/poolimpact_test.go#TestPoolUpdateApplyRoutes"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-07-21
status: complete
---

# Phase 2 Plan 5: Pool Impact-Review Engine and Atomic Apply Summary

**Deterministic pool add/remove impact-plan builder (auto-proposed next-free universe/address) with a two-gate integrity/freshness apply contract copied from internal/trace/apply/guard.go, exposed through a `pool update` (dry-run) / `pool apply` (atomic) CLI split with a configurable, review-by-default propagation mode.**

## Performance

- **Duration:** 55 min
- **Tasks:** 3
- **Files modified:** 6 (5 created, 1 modified)

## Accomplishments
- `internal/pool.BuildImpactPlan` walks deployment instances and cross-pool groups for a pool add/remove request, auto-proposing next-free universe/address for new instances via `deployment.NextFreeAddress`, with a byte-identical `plan_id` for identical inputs and zero dependent operations (not an error) for a pool with no dependents.
- `internal/pool.ValidatePlanIntegrity` / `ValidatePlanFreshness` mirror `internal/trace/apply/guard.go`'s two-gate contract exactly, emitting `GOLC_POOL_PLAN_SCHEMA` / `GOLC_POOL_PLAN_HASH` / `GOLC_POOL_PLAN_STALE`.
- `internal/pool.Apply` mutates pool membership, deployment instances, and group member refs in one all-or-nothing pass, minting every new UUID only at apply time.
- `pool update` / `pool apply` CLI routes implement the Terraform-style plan/apply split (D-15): dry-run compute-and-write vs. validate-and-apply, with `application_defaults.pool_update_review` resolved through `internal/projectconfig` (review-required by default) and a per-invocation `--propagate` override.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing tests for impact-plan build, integrity/freshness gates, atomic apply, and dry-run/apply routes** - `32ab3e7` (test)
2. **Task 2: Deterministic impact-plan builder + integrity/freshness gates + atomic apply** - `b4a7cc8` (feat)
3. **Task 3: pool update (dry-run) / pool apply routes + configurable propagation default** - `a5dc8c6` (feat)

**Plan metadata:** committed separately by the orchestrator after the wave completes (worktree mode).

## Files Created/Modified
- `internal/pool/impact.go` - `ImpactRequest`/`PoolMemberSpec`/`ImpactOp`/`ImpactPlan`/`BuildImpactPlan`: deterministic dependent walk + `plan_id` binding
- `internal/pool/impact_test.go` - determinism, empty-dependent, auto-address-proposal, add-then-remove-nets-to-original tests
- `internal/pool/plan.go` - `ValidatePlanIntegrity`/`ValidatePlanFreshness`/`Apply`: two-gate contract + atomic mutation
- `internal/pool/plan_test.go` - tamper/staleness/atomic-apply tests
- `internal/command/pool.go` - `pool update`/`pool apply` routes, arg parsing, propagation default resolution
- `internal/command/poolimpact_test.go` - end-to-end CLI route contract and propagation default tests

## Decisions Made
- See `key-decisions` in frontmatter: the show.State-avoidance signature change, the `|`-delimited `--add` grammar, the "existing dependent" precondition for auto-proposal, apply-time-only UUID minting, and the locked five-layer config resolution for the propagation default.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Changed BuildImpactPlan/ValidatePlanFreshness/Apply signatures to avoid an import cycle**
- **Found during:** Task 2 (writing internal/pool/impact.go)
- **Issue:** The plan's literal artifact listing (`BuildImpactPlan(state show.State, req ImpactRequest)`) would require `internal/pool` to import `internal/show`. `internal/show` already imports `internal/pool` (its `State` struct embeds `[]pool.Pool` and `[]pool.Group`), so this is a direct, unavoidable import cycle -- the code cannot compile as literally specified.
- **Fix:** `BuildImpactPlan`, `ValidatePlanFreshness`, and `Apply` take the show model's plain component fields (`[]Pool`, `[]deployment.Deployment`, `[]Group`, `revision int`) instead of a `show.State` value. `internal/command/pool.go` (which already imports both `internal/show` and `internal/pool`) passes `state.Pools`/`state.Deployments`/`state.Groups`/`state.Revision` through directly at each call site, and reassigns `Apply`'s returned slices back onto `state` before calling `show.Save`. Every other artifact in the plan (`ImpactRequest`, `ImpactPlan`, `ImpactOp`, diagnostic codes, the `pool update`/`pool apply` CLI shape) is unchanged.
- **Files modified:** internal/pool/impact.go, internal/pool/plan.go, internal/command/pool.go
- **Verification:** `go build ./...` succeeds with no cycle; `go test ./internal/pool/... ./internal/command/...` and the full `go test ./internal/...` suite pass.
- **Committed in:** b4a7cc8 (Task 2 commit), a5dc8c6 (Task 3 commit, call-site wiring)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary for the code to compile at all; every other artifact, diagnostic code, and CLI contract in the plan is implemented as written. No scope creep.

## Issues Encountered
None beyond the import-cycle deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/pool.BuildImpactPlan`/`ValidatePlanIntegrity`/`ValidatePlanFreshness`/`Apply` and the `pool update`/`pool apply` routes are ready for Phase 3+ dependent-category extensions (themes/scenes/chases/motion presets/controller mappings) to plug into the same ordered walk without a rewrite (per the plan's phase-boundary note).
- No blockers for the next wave.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-21*

## Self-Check: PASSED

All created files (internal/pool/impact.go, internal/pool/impact_test.go, internal/pool/plan.go, internal/pool/plan_test.go, internal/command/poolimpact_test.go, this SUMMARY.md) and all commit hashes (32ab3e7, b4a7cc8, a5dc8c6, 238bbe0) were verified present.
