// plan_test.go proves the two-gate ImpactPlan apply contract (02-05-PLAN.md,
// Task 1: POOL-04/POOL-05/D-16), mirroring internal/trace/apply/guard.go's
// ValidatePlanIntegrity/ValidatePlanFreshness shape: a plan whose bytes
// were altered after hashing fails ValidatePlanIntegrity with
// GOLC_POOL_PLAN_HASH, a wrong schema_version fails with
// GOLC_POOL_PLAN_SCHEMA, a plan built against one show revision fails
// ValidatePlanFreshness with GOLC_POOL_PLAN_STALE once the revision moves
// (including immediately after the plan's own successful apply -- the
// single-use property), and a successful Apply mutates the pool/
// deployment model in one all-or-nothing step.
//
// This file fails at RUN time until plan.go implements
// ValidatePlanIntegrity/ValidatePlanFreshness/Apply (Task 2) -- that is
// the RED state this task proves.
package pool_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/pool"
)

func TestPlanIntegrityRejectsTamper(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)
	req := pool.ImpactRequest{PoolID: target.ID, Propagate: "preview"}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")
	require.NoError(t, pool.ValidatePlanIntegrity(plan), "expected a freshly built plan to pass integrity")

	tampered := plan
	tampered.Propagate = "immediate"
	err = pool.ValidatePlanIntegrity(tampered)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_HASH", "expected GOLC_POOL_PLAN_HASH for a plan altered after hashing")

	wrongSchema := plan
	wrongSchema.SchemaVersion = plan.SchemaVersion + 1
	err = pool.ValidatePlanIntegrity(wrongSchema)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_SCHEMA", "expected GOLC_POOL_PLAN_SCHEMA for a wrong schema version")
}

func TestPlanFreshnessRejectsStale(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)
	req := pool.ImpactRequest{PoolID: target.ID, Propagate: "preview"}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")
	require.NoError(t, pool.ValidatePlanFreshness(plan, fx.pools, fx.deployments, fx.groups, fx.revision), "expected a freshly built plan to pass freshness against the same state")

	err = pool.ValidatePlanFreshness(plan, fx.pools, fx.deployments, fx.groups, fx.revision+1)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_STALE", "expected GOLC_POOL_PLAN_STALE once the show revision moved")
}

func TestApplyAtomic(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	req := pool.ImpactRequest{
		PoolID:    target.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		Propagate: "preview",
	}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")

	beforeMemberCount := len(fx.pools[0].Members)
	beforeInstanceCount := len(fx.deployments[0].Instances)

	newPools, newDeployments, _, err := pool.Apply(fx.pools, fx.deployments, fx.groups, plan)
	require.NoError(t, err, "Apply")
	require.Len(t, newPools[0].Members, beforeMemberCount+1, "expected the pool to gain exactly one member")
	require.Len(t, newDeployments[0].Instances, beforeInstanceCount+1, "expected the deployment to gain exactly one proposed instance")
	// The original slices must be left completely unchanged (all-or-
	// nothing at the model boundary: Apply never mutates its inputs).
	require.Len(t, fx.pools[0].Members, beforeMemberCount, "expected Apply to leave the input pool slice unmutated")

	postApplyRevision := fx.revision + 1 // simulate show.Save's revision bump

	// A second apply attempt of the exact same plan is rejected as stale
	// by the freshness gate every "pool apply" invocation runs before
	// Apply (CONTEXT D-16): the plan's ExpectedRevision no longer matches
	// the post-apply revision (single-use).
	err = pool.ValidatePlanFreshness(plan, newPools, newDeployments, fx.groups, postApplyRevision)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_STALE", "expected a re-apply of the same plan to be rejected as stale")
}
