// impact_test.go proves BuildImpactPlan's determinism, empty-dependent,
// and auto-address-proposal contract (02-05-PLAN.md, Task 1: POOL-03/D-11):
// identical requests against an identical show model always produce a
// byte-identical plan_id and a stable operation order, adding then
// removing the same fixture nets the pool/deployment back to their
// original state, a pool with no dependents yields a well-formed
// zero-operation plan (never an error), and every proposed instance
// receives a distinct, in-bounds universe/address via
// deployment.NextFreeAddress.
//
// This file compiles against the already-implemented internal/pool
// model.go, internal/deployment, and internal/show packages, but fails at
// RUN time until impact.go implements BuildImpactPlan/PoolMemberSpec/
// ImpactRequest/ImpactPlan/ImpactOp/Apply (Task 2) -- that is the RED
// state this task proves.
package pool_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
)

// fixtureShow is the minimal show model content newFixtureState builds:
// one pool with one existing member, one active deployment with one
// instance already patched to that member (so the pool "has a
// dependent"), and one group referencing the same member.
type fixtureShow struct {
	pools       []pool.Pool
	deployments []deployment.Deployment
	groups      []pool.Group
	revision    int
}

// newFixtureState builds a deterministic show model fixture reused across
// impact_test.go and plan_test.go: a pool with one existing member, an
// active deployment with one instance already patched to that member (so
// BuildImpactPlan's dependent walk has content to discover), and a group
// referencing the same member.
func newFixtureState(t *testing.T) (fx fixtureShow, target pool.Pool, dep deployment.Deployment, existingMember pool.PoolMember, grp pool.Group) {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("acme/par64", "sha256:aaaaaaaa")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	d.Active = true
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	groupID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	g := pool.Group{
		ID:         groupID,
		Name:       "Front Wash",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
	}

	fx = fixtureShow{
		pools:       []pool.Pool{p},
		deployments: []deployment.Deployment{d},
		groups:      []pool.Group{g},
		revision:    1,
	}
	return fx, p, d, member, g
}

func TestBuildImpactPlanDeterministic(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	req := pool.ImpactRequest{
		PoolID: target.ID,
		Add: []pool.PoolMemberSpec{
			{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:bbbbbbbb", Mode: "Standard"},
		},
		Propagate: "preview",
	}

	first, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")
	second, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan (second)")
	require.NotEmpty(t, first.PlanID, "expected a non-empty plan_id")
	require.Equal(t, second.PlanID, first.PlanID, "expected byte-identical plan_id for identical inputs")
	require.NotEmpty(t, first.Operations, "expected at least one dependent operation for a pool with an existing deployment instance")
	require.Len(t, second.Operations, len(first.Operations), "expected stable operation count")
	for i := range first.Operations {
		require.Equal(t, second.Operations[i], first.Operations[i], "expected stable operation order at index %d", i)
	}

	// Adding then removing the same fixture nets the pool/deployment back
	// to their original state; the impact plan reflects the net effect.
	// The new member's UUID is only known after the add plan applies, so
	// the remove plan is built against the post-add state.
	newPools, newDeployments, newGroups, err := pool.Apply(fx.pools, fx.deployments, fx.groups, first)
	require.NoError(t, err, "Apply (add)")
	postAddRevision := fx.revision + 1 // simulate show.Save's revision bump

	var mintedMemberID uuid.UUID
	for _, m := range newPools[0].Members {
		if m.FixtureContentHash == "sha256:bbbbbbbb" {
			mintedMemberID = m.ID
		}
	}
	require.NotEqual(t, uuid.Nil, mintedMemberID, "expected the newly added pool member to be present after Apply")

	removeReq := pool.ImpactRequest{PoolID: target.ID, Remove: []uuid.UUID{mintedMemberID}, Propagate: "preview"}
	removePlan, err := pool.BuildImpactPlan(newPools, newDeployments, newGroups, postAddRevision, removeReq)
	require.NoError(t, err, "BuildImpactPlan (remove)")
	finalPools, finalDeployments, _, err := pool.Apply(newPools, newDeployments, newGroups, removePlan)
	require.NoError(t, err, "Apply (remove)")

	require.Len(t, finalPools[0].Members, len(fx.pools[0].Members), "expected pool membership to net back to the original count")
	require.Equal(t, fx.pools[0].Members[0].ID, finalPools[0].Members[0].ID, "expected the original member to survive add-then-remove unchanged")
	require.Len(t, finalDeployments[0].Instances, len(fx.deployments[0].Instances), "expected deployment instance count to net back to the original")
}

func TestBuildImpactPlanEmpty(t *testing.T) {
	p, err := pool.NewPool("Empty Pool", nil)
	require.NoError(t, err, "NewPool")

	plan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, pool.ImpactRequest{PoolID: p.ID, Propagate: "preview"})
	require.NoError(t, err, "BuildImpactPlan")
	require.Empty(t, plan.Operations, "expected zero dependent operations for an empty pool")
	require.NotEmpty(t, plan.PlanID, "expected a well-formed plan with a non-empty plan_id")

	// Adding a fixture to a pool with no dependents also yields zero
	// dependent operations (not an error): no deployment currently
	// references this pool, so nothing gets an auto-proposed instance.
	addReq := pool.ImpactRequest{
		PoolID:    p.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:cccccccc", Mode: "Standard"}},
		Propagate: "preview",
	}
	addPlan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, addReq)
	require.NoError(t, err, "BuildImpactPlan (add, no dependents)")
	require.Empty(t, addPlan.Operations, "expected zero dependent operations when no deployment references the pool yet")
}

func TestBuildImpactPlanAutoAddress(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	req := pool.ImpactRequest{
		PoolID: target.ID,
		Add: []pool.PoolMemberSpec{
			{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:cccccccc", Mode: "Standard"},
			{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:dddddddd", Mode: "Standard"},
		},
		Propagate: "preview",
	}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Len(t, addOps, 2, "expected one proposed instance per Add spec (2), got %+v", addOps)
	seen := map[[2]int]bool{}
	for _, op := range addOps {
		require.False(t, op.ProposedUniverse < 1 || op.ProposedAddress < 1, "expected a positive proposed universe/address, got %+v", op)
		require.LessOrEqual(t, op.ProposedAddress, 512, "expected the proposed address to stay within one 512-channel universe, got %+v", op)
		key := [2]int{op.ProposedUniverse, op.ProposedAddress}
		require.False(t, seen[key], "expected distinct proposed addresses for two adds in the same request, got a collision at %+v", op)
		seen[key] = true
	}
}

// TestBuildImpactPlanAutoAddressRespectsChannelCount proves each Add spec's
// real ChannelCount (not the 1-channel defaultInstanceChannelCount
// fallback) spaces its proposed address: three 5-channel specs starting at
// (1, 1) against a deployment with zero existing instances land at
// 1, 6, 11 -- not 1, 2, 3, the exact multi-add address-collision bug
// ChannelCount closes for a batch of same-width fixtures (a real DMX
// fixture's channel span, unlike the pre-ChannelCount 1-channel
// assumption, routinely exceeds one address).
func TestBuildImpactPlanAutoAddressRespectsChannelCount(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")

	req := pool.ImpactRequest{
		PoolID: p.ID,
		Add: []pool.PoolMemberSpec{
			{FixtureStableKey: "acme/colorband", FixtureContentHash: "sha256:cccccccc", Mode: "5ch", ChannelCount: 5},
			{FixtureStableKey: "acme/colorband", FixtureContentHash: "sha256:dddddddd", Mode: "5ch", ChannelCount: 5},
			{FixtureStableKey: "acme/colorband", FixtureContentHash: "sha256:eeeeeeee", Mode: "5ch", ChannelCount: 5},
		},
		AttachDeployments: []uuid.UUID{d.ID},
		StartUniverse:     1,
		StartAddress:      1,
		Propagate:         "preview",
	}
	plan, err := pool.BuildImpactPlan([]pool.Pool{p}, []deployment.Deployment{d}, nil, 0, req)
	require.NoError(t, err, "BuildImpactPlan")

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Len(t, addOps, 3, "expected one proposed instance per Add spec (3), got %+v", addOps)
	wantAddresses := []int{1, 6, 11}
	for i, op := range addOps {
		require.Equal(t, 1, op.ProposedUniverse, "op[%d]=%+v", i, op)
		require.Equal(t, wantAddresses[i], op.ProposedAddress, "expected 5-channel-spaced addresses %v, got op[%d]=%+v", wantAddresses, i, op)
	}
}

// TestBuildImpactPlanForceAttachFreshPool proves AttachDeployments closes
// the "adopt a never-before-used pool" gap: a brand-new pool with zero
// dependents still yields a proposed instance for a deployment named in
// AttachDeployments, even though that deployment has never referenced the
// pool before (deploymentUsesPool would otherwise skip it entirely, as
// TestBuildImpactPlanEmpty's "add, no dependents" sub-case proves).
func TestBuildImpactPlanForceAttachFreshPool(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{d.ID},
		Propagate:         "preview",
	}
	plan, err := pool.BuildImpactPlan([]pool.Pool{p}, []deployment.Deployment{d}, nil, 0, req)
	require.NoError(t, err, "BuildImpactPlan")

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Len(t, addOps, 1, "expected exactly one proposed instance for the force-attached deployment, got %+v", addOps)
	require.Equal(t, d.ID, addOps[0].DependentID, "expected the proposed instance to target the force-attached deployment")
	require.Equal(t, 1, addOps[0].ProposedUniverse)
	require.Equal(t, 1, addOps[0].ProposedAddress, "expected the first proposed instance in a fresh deployment to land at (1, 1)")
}

// TestBuildImpactPlanForceAttachRejectsUnknownDeployment proves an
// AttachDeployments entry that doesn't exist in the given deployments slice
// fails outright, before any operation is computed.
func TestBuildImpactPlanForceAttachRejectsUnknownDeployment(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	unknownID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{unknownID},
		Propagate:         "preview",
	}
	_, err = pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, req)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_UNKNOWN_DEPLOYMENT")
}

// TestBuildImpactPlanForceAttachChangesPlanID proves AttachDeployments is
// part of the hashed plan body: two otherwise-identical requests differing
// only in AttachDeployments must produce different plan_ids, since they are
// materially different requests even when a request's resulting Operations
// happen to be empty either way.
func TestBuildImpactPlanForceAttachChangesPlanID(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	deployments := []deployment.Deployment{d}

	baseReq := pool.ImpactRequest{
		PoolID:    p.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		Propagate: "preview",
	}
	without, err := pool.BuildImpactPlan([]pool.Pool{p}, deployments, nil, 0, baseReq)
	require.NoError(t, err, "BuildImpactPlan (without attach)")

	attachedReq := baseReq
	attachedReq.AttachDeployments = []uuid.UUID{d.ID}
	with, err := pool.BuildImpactPlan([]pool.Pool{p}, deployments, nil, 0, attachedReq)
	require.NoError(t, err, "BuildImpactPlan (with attach)")

	require.NotEqual(t, with.PlanID, without.PlanID, "expected AttachDeployments to change plan_id")
}

// TestBuildImpactPlanForceAttachFreshnessRoundTrip proves a force-attach
// plan round-trips through the same integrity/freshness/apply contract
// every other plan does: it validates, applies, and a re-derived plan
// against the post-apply state is correctly rejected as stale (single-use).
func TestBuildImpactPlanForceAttachFreshnessRoundTrip(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	pools := []pool.Pool{p}
	deployments := []deployment.Deployment{d}

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{d.ID},
		Propagate:         "preview",
	}
	plan, err := pool.BuildImpactPlan(pools, deployments, nil, 0, req)
	require.NoError(t, err, "BuildImpactPlan")
	require.NoError(t, pool.ValidatePlanIntegrity(plan), "ValidatePlanIntegrity")
	require.NoError(t, pool.ValidatePlanFreshness(plan, pools, deployments, nil, 0), "ValidatePlanFreshness (pre-apply)")

	newPools, newDeployments, _, err := pool.Apply(pools, deployments, nil, plan)
	require.NoError(t, err, "Apply")
	postApplyRevision := 1 // simulate show.Save's revision bump

	err = pool.ValidatePlanFreshness(plan, newPools, newDeployments, nil, postApplyRevision)
	require.ErrorContains(t, err, "GOLC_POOL_PLAN_STALE", "expected GOLC_POOL_PLAN_STALE re-validating the same plan against post-apply state")
}

// TestBuildImpactPlanStartAddressOverride proves StartUniverse/StartAddress
// anchor the scan for a batch of adds, with the second unit auto-
// incrementing past the first exactly like the default (1,1) scan already
// does.
func TestBuildImpactPlanStartAddressOverride(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	req := pool.ImpactRequest{
		PoolID: target.ID,
		Add: []pool.PoolMemberSpec{
			{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:cccccccc", Mode: "Standard"},
			{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:dddddddd", Mode: "Standard"},
		},
		StartUniverse: 3,
		StartAddress:  50,
		Propagate:     "preview",
	}
	plan, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	require.NoError(t, err, "BuildImpactPlan")

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	require.Len(t, addOps, 2, "expected 2 proposed instances, got %+v", addOps)
	require.Equal(t, 3, addOps[0].ProposedUniverse)
	require.Equal(t, 50, addOps[0].ProposedAddress, "expected the first proposed instance to anchor at (3, 50)")
	require.Equal(t, 3, addOps[1].ProposedUniverse)
	require.Equal(t, 51, addOps[1].ProposedAddress, "expected the second proposed instance to auto-increment to (3, 51)")
}

// TestBuildImpactPlanZeroStartAddressMatchesDefault proves an explicit
// (0, 0) StartUniverse/StartAddress produces a byte-identical plan_id to
// omitting the fields entirely -- the additive default-preservation
// contract every existing caller relies on.
func TestBuildImpactPlanZeroStartAddressMatchesDefault(t *testing.T) {
	fx, target, _, _, _ := newFixtureState(t)

	withoutFields := pool.ImpactRequest{
		PoolID:    target.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:cccccccc", Mode: "Standard"}},
		Propagate: "preview",
	}
	explicitZero := withoutFields
	explicitZero.StartUniverse = 0
	explicitZero.StartAddress = 0

	without, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, withoutFields)
	require.NoError(t, err, "BuildImpactPlan (without fields)")
	withZero, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, explicitZero)
	require.NoError(t, err, "BuildImpactPlan (explicit zero)")
	require.Equal(t, withZero.PlanID, without.PlanID, "expected explicit (0,0) StartUniverse/StartAddress to match omitting the fields")
}
