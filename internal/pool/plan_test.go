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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/pool"
)

func TestPlanIntegrityRejectsTamper(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)
	req := pool.ImpactRequest{PoolID: target.ID, Propagate: "preview"}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}
	if err := pool.ValidatePlanIntegrity(plan); err != nil {
		t.Fatalf("expected a freshly built plan to pass integrity, got %v", err)
	}

	tampered := plan
	tampered.Propagate = "immediate"
	if err := pool.ValidatePlanIntegrity(tampered); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_HASH") {
		t.Fatalf("expected GOLC_POOL_PLAN_HASH for a plan altered after hashing, got %v", err)
	}

	wrongSchema := plan
	wrongSchema.SchemaVersion = plan.SchemaVersion + 1
	if err := pool.ValidatePlanIntegrity(wrongSchema); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_SCHEMA") {
		t.Fatalf("expected GOLC_POOL_PLAN_SCHEMA for a wrong schema version, got %v", err)
	}
}

func TestPlanFreshnessRejectsStale(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)
	req := pool.ImpactRequest{PoolID: target.ID, Propagate: "preview"}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}
	if err := pool.ValidatePlanFreshness(plan, fx.pools, fx.deployments, fx.groups, fx.revision); err != nil {
		t.Fatalf("expected a freshly built plan to pass freshness against the same state, got %v", err)
	}

	if err := pool.ValidatePlanFreshness(plan, fx.pools, fx.deployments, fx.groups, fx.revision+1); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected GOLC_POOL_PLAN_STALE once the show revision moved, got %v", err)
	}
}

func TestApplyAtomic(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	req := pool.ImpactRequest{
		PoolID:    target.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		Propagate: "preview",
	}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}

	beforeMemberCount := len(fx.pools[0].Members)
	beforeInstanceCount := len(fx.deployments[0].Instances)

	newPools, newDeployments, _, err := pool.Apply(fx.pools, fx.deployments, fx.groups, plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(newPools[0].Members) != beforeMemberCount+1 {
		t.Fatalf("expected the pool to gain exactly one member, got %d want %d", len(newPools[0].Members), beforeMemberCount+1)
	}
	if len(newDeployments[0].Instances) != beforeInstanceCount+1 {
		t.Fatalf("expected the deployment to gain exactly one proposed instance, got %d want %d", len(newDeployments[0].Instances), beforeInstanceCount+1)
	}
	// The original slices must be left completely unchanged (all-or-
	// nothing at the model boundary: Apply never mutates its inputs).
	if len(fx.pools[0].Members) != beforeMemberCount {
		t.Fatalf("expected Apply to leave the input pool slice unmutated, got %d members", len(fx.pools[0].Members))
	}

	postApplyRevision := fx.revision + 1 // simulate show.Save's revision bump

	// A second apply attempt of the exact same plan is rejected as stale
	// by the freshness gate every "pool apply" invocation runs before
	// Apply (CONTEXT D-16): the plan's ExpectedRevision no longer matches
	// the post-apply revision (single-use).
	if err := pool.ValidatePlanFreshness(plan, newPools, newDeployments, fx.groups, postApplyRevision); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected a re-apply of the same plan to be rejected as stale, got %v", err)
	}
}
