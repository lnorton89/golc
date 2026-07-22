// plan.go implements the two-gate ImpactPlan apply contract (CONTEXT
// D-16/POOL-04/POOL-05), mirroring internal/trace/apply/guard.go's
// ValidatePlanIntegrity/ValidatePlanFreshness shape exactly, renamed to
// the pool domain: ValidatePlanIntegrity recomputes plan_id from the
// plan's own canonical body before any mutation is attempted, and
// ValidatePlanFreshness rebuilds the plan from the current show model and
// compares plan_id, rejecting a stale plan (the show revision moved since
// review, including immediately after the plan's own successful apply --
// single-use) before apply. Apply performs the reviewed mutation in one
// all-or-nothing pass: every new PoolMember/Instance UUID is minted here,
// at apply time (never inside BuildImpactPlan), and the caller is
// responsible for persisting the result (show.Save), which bumps
// Revision -- the freshness guard's single-use enforcement for any later
// re-apply attempt of the exact same plan (CONTEXT POOL-05: no dependent
// is ever observed partially updated).
package pool

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
)

// ValidatePlanIntegrity rejects plan outright before ValidatePlanFreshness
// or any mutation is attempted: its schema_version must match
// ImpactSchemaVersion, and its own recorded plan_id must match the
// SHA-256 binding recomputed from its own bytes -- a plan edited after
// being hashed, or hand-forged with an arbitrary plan_id, fails here
// before anything else runs (mirrors
// internal/trace/apply/guard.go.ValidatePlanIntegrity).
func ValidatePlanIntegrity(plan ImpactPlan) error {
	if plan.SchemaVersion != ImpactSchemaVersion {
		return fmt.Errorf("GOLC_POOL_PLAN_SCHEMA: plan schema_version %d does not match expected %d", plan.SchemaVersion, ImpactSchemaVersion)
	}
	recomputed, err := computePlanID(bodyOf(plan))
	if err != nil {
		return err
	}
	if recomputed != plan.PlanID {
		return fmt.Errorf("GOLC_POOL_PLAN_HASH: plan_id %q does not match its own recomputed canonical hash %q; the plan bytes were altered after hashing", plan.PlanID, recomputed)
	}
	return nil
}

// ValidatePlanFreshness rejects plan if rebuilding the exact same
// impact-plan computation from the given current show model no longer
// produces a byte-identical plan_id (CONTEXT D-16/POOL-04/POOL-05): a
// show model that changed after the plan was reviewed -- including the
// plan's own prior successful apply, which bumps Revision -- is caught
// here, before any mutation is attempted. Re-running "pool update" is the
// only way to obtain a fresh, applyable plan.
//
// BuildImpactPlan itself never derives Warnings/Errors (it always leaves
// them empty); those fields exist on ImpactPlan so a caller-supplied
// review layer built on top of the same Add/Remove/Operations shape --
// for example internal/substitution's capability-diff gaps and
// GOLC_SUBSTITUTION_TARGET_INVALID hard-block (D-16: one mechanism, not a
// second) -- can attach its own review annotations without BuildImpactPlan
// needing to know anything about fixture capability semantics. Freshness
// is therefore scoped to the state-dependent computation only: fresh
// carries plan's own Warnings/Errors forward, unchanged, before the
// plan_id comparison, so a rebuild whose Add/Remove/Propagate/
// ExpectedRevision/Operations still match is never rejected merely because
// BuildImpactPlan cannot regenerate a caller's own review annotations. For
// a plain pool update those annotations are always empty on both sides, so
// this is a no-op; ValidatePlanIntegrity (the tamper-check gate, called
// before this one) still hashes plan's Warnings/Errors exactly as stored,
// so any tampering of those fields is still caught before this function
// ever runs.
func ValidatePlanFreshness(plan ImpactPlan, pools []Pool, deployments []deployment.Deployment, groups []Group, revision int) error {
	req := ImpactRequest{PoolID: plan.PoolID, Add: plan.Add, Remove: plan.Remove, Propagate: plan.Propagate}
	fresh, err := BuildImpactPlan(pools, deployments, groups, revision, req)
	if err != nil {
		return fmt.Errorf("GOLC_POOL_PLAN_STALE: recomputing the current impact plan failed: %v; re-run pool review", err)
	}
	fresh.Warnings = plan.Warnings
	fresh.Errors = plan.Errors
	freshID, err := computePlanID(bodyOf(fresh))
	if err != nil {
		return err
	}
	if freshID != plan.PlanID {
		return fmt.Errorf("GOLC_POOL_PLAN_STALE: plan %s no longer matches the current show state (recomputed %s); re-run pool review", plan.PlanID, freshID)
	}
	return nil
}

// RecomputePlanID recomputes plan_id = sha256(canonical_body) from plan's
// own current fields, reusing the exact same canonical-hash computation
// ValidatePlanIntegrity uses internally. Exported so a caller building a
// pool.ImpactPlan-shaped plan with review data BuildImpactPlan itself has
// no way to derive (for example internal/substitution's capability-diff
// Warnings and GOLC_SUBSTITUTION_TARGET_INVALID Errors) can bind its own
// plan_id through this exact single mechanism (D-16), rather than
// hand-rolling a second hash implementation.
func RecomputePlanID(plan ImpactPlan) (string, error) {
	return computePlanID(bodyOf(plan))
}

// Apply performs plan's reviewed mutation against pools/deployments/
// groups in one all-or-nothing pass, returning new slices -- the input
// slices are never mutated, so a caller observing an error return knows
// the original model was left completely unchanged (CONTEXT POOL-05: no
// dependent is ever observed partially updated). Every new PoolMember/
// Instance UUID is minted here, at apply time. The caller is responsible
// for running ValidatePlanIntegrity/ValidatePlanFreshness before calling
// Apply, and for persisting the result afterward (show.Save, which bumps
// Revision -- the single-use freshness guard for any later re-apply
// attempt of the exact same plan).
func Apply(pools []Pool, deployments []deployment.Deployment, groups []Group, plan ImpactPlan) ([]Pool, []deployment.Deployment, []Group, error) {
	if len(plan.Errors) > 0 {
		return nil, nil, nil, fmt.Errorf("GOLC_POOL_APPLY_PLAN_ERRORS: plan carries %d unresolved error(s); revise and re-run pool update", len(plan.Errors))
	}

	newPools := append([]Pool(nil), pools...)
	poolIndex := -1
	for i, p := range newPools {
		if p.ID == plan.PoolID {
			poolIndex = i
			break
		}
	}
	if poolIndex == -1 {
		return nil, nil, nil, fmt.Errorf("GOLC_POOL_PLAN_STALE: pool %s no longer exists in the current show state; re-run pool review", plan.PoolID)
	}

	removeSet := make(map[uuid.UUID]bool, len(plan.Remove))
	for _, id := range plan.Remove {
		removeSet[id] = true
	}

	targetPool := newPools[poolIndex]
	keptMembers := make([]PoolMember, 0, len(targetPool.Members))
	for _, m := range targetPool.Members {
		if removeSet[m.ID] {
			continue
		}
		keptMembers = append(keptMembers, m)
	}
	newMembers := make([]PoolMember, 0, len(plan.Add))
	for _, spec := range plan.Add {
		member, err := NewPoolMember(spec.FixtureStableKey, spec.FixtureContentHash)
		if err != nil {
			return nil, nil, nil, err
		}
		newMembers = append(newMembers, member)
	}
	targetPool.Members = append(keptMembers, newMembers...)
	newPools[poolIndex] = targetPool

	newDeployments := append([]deployment.Deployment(nil), deployments...)
	for i, d := range newDeployments {
		kept := make([]deployment.Instance, 0, len(d.Instances))
		for _, instance := range d.Instances {
			if instance.PoolID == plan.PoolID && removeSet[instance.PoolMemberID] {
				continue
			}
			kept = append(kept, instance)
		}
		for _, op := range plan.Operations {
			if op.DependentKind != "deployment_instance" || op.Action != "add" || op.DependentID != d.ID {
				continue
			}
			if op.PoolMemberIndex < 0 || op.PoolMemberIndex >= len(newMembers) {
				return nil, nil, nil, fmt.Errorf("GOLC_POOL_PLAN_STALE: operation for deployment %q references an out-of-range add index; re-run pool review", d.Name)
			}
			member := newMembers[op.PoolMemberIndex]
			instanceID, err := uuid.NewV7()
			if err != nil {
				return nil, nil, nil, fmt.Errorf("GOLC_POOL_INSTANCE_ID_MINT_FAILED: %v", err)
			}
			kept = append(kept, deployment.Instance{
				ID:           instanceID,
				PoolID:       plan.PoolID,
				PoolMemberID: member.ID,
				Mode:         plan.Add[op.PoolMemberIndex].Mode,
				Universe:     op.ProposedUniverse,
				Address:      op.ProposedAddress,
			})
		}
		d.Instances = kept
		newDeployments[i] = d
	}

	newGroups := append([]Group(nil), groups...)
	for i, g := range newGroups {
		if len(g.MemberRefs) == 0 {
			continue
		}
		kept := make([]MemberRef, 0, len(g.MemberRefs))
		for _, ref := range g.MemberRefs {
			if ref.PoolID == plan.PoolID && removeSet[ref.PoolMemberID] {
				continue
			}
			kept = append(kept, ref)
		}
		g.MemberRefs = kept
		newGroups[i] = g
	}

	return newPools, newDeployments, newGroups, nil
}
