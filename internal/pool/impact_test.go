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
	"strings"
	"testing"

	"github.com/google/uuid"

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
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("acme/par64", "sha256:aaaaaaaa")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	d.Active = true
	instanceID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	groupID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
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
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}
	second, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan (second): %v", err)
	}
	if first.PlanID == "" {
		t.Fatal("expected a non-empty plan_id")
	}
	if first.PlanID != second.PlanID {
		t.Fatalf("expected byte-identical plan_id for identical inputs, got %q vs %q", first.PlanID, second.PlanID)
	}
	if len(first.Operations) == 0 {
		t.Fatal("expected at least one dependent operation for a pool with an existing deployment instance")
	}
	if len(first.Operations) != len(second.Operations) {
		t.Fatalf("expected stable operation count, got %d vs %d", len(first.Operations), len(second.Operations))
	}
	for i := range first.Operations {
		if first.Operations[i] != second.Operations[i] {
			t.Fatalf("expected stable operation order at index %d: %+v vs %+v", i, first.Operations[i], second.Operations[i])
		}
	}

	// Adding then removing the same fixture nets the pool/deployment back
	// to their original state; the impact plan reflects the net effect.
	// The new member's UUID is only known after the add plan applies, so
	// the remove plan is built against the post-add state.
	newPools, newDeployments, newGroups, err := pool.Apply(fx.pools, fx.deployments, fx.groups, first)
	if err != nil {
		t.Fatalf("Apply (add): %v", err)
	}
	postAddRevision := fx.revision + 1 // simulate show.Save's revision bump

	var mintedMemberID uuid.UUID
	for _, m := range newPools[0].Members {
		if m.FixtureContentHash == "sha256:bbbbbbbb" {
			mintedMemberID = m.ID
		}
	}
	if mintedMemberID == uuid.Nil {
		t.Fatal("expected the newly added pool member to be present after Apply")
	}

	removeReq := pool.ImpactRequest{PoolID: target.ID, Remove: []uuid.UUID{mintedMemberID}, Propagate: "preview"}
	removePlan, err := pool.BuildImpactPlan(newPools, newDeployments, newGroups, postAddRevision, removeReq)
	if err != nil {
		t.Fatalf("BuildImpactPlan (remove): %v", err)
	}
	finalPools, finalDeployments, _, err := pool.Apply(newPools, newDeployments, newGroups, removePlan)
	if err != nil {
		t.Fatalf("Apply (remove): %v", err)
	}

	if len(finalPools[0].Members) != len(fx.pools[0].Members) {
		t.Fatalf("expected pool membership to net back to the original count, got %d want %d", len(finalPools[0].Members), len(fx.pools[0].Members))
	}
	if finalPools[0].Members[0].ID != fx.pools[0].Members[0].ID {
		t.Fatalf("expected the original member to survive add-then-remove unchanged, got %s want %s", finalPools[0].Members[0].ID, fx.pools[0].Members[0].ID)
	}
	if len(finalDeployments[0].Instances) != len(fx.deployments[0].Instances) {
		t.Fatalf("expected deployment instance count to net back to the original, got %d want %d", len(finalDeployments[0].Instances), len(fx.deployments[0].Instances))
	}
}

func TestBuildImpactPlanEmpty(t *testing.T) {
	p, err := pool.NewPool("Empty Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	plan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, pool.ImpactRequest{PoolID: p.ID, Propagate: "preview"})
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}
	if len(plan.Operations) != 0 {
		t.Fatalf("expected zero dependent operations for an empty pool, got %d", len(plan.Operations))
	}
	if plan.PlanID == "" {
		t.Fatal("expected a well-formed plan with a non-empty plan_id")
	}

	// Adding a fixture to a pool with no dependents also yields zero
	// dependent operations (not an error): no deployment currently
	// references this pool, so nothing gets an auto-proposed instance.
	addReq := pool.ImpactRequest{
		PoolID:    p.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:cccccccc", Mode: "Standard"}},
		Propagate: "preview",
	}
	addPlan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, addReq)
	if err != nil {
		t.Fatalf("BuildImpactPlan (add, no dependents): %v", err)
	}
	if len(addPlan.Operations) != 0 {
		t.Fatalf("expected zero dependent operations when no deployment references the pool yet, got %d", len(addPlan.Operations))
	}
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
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	if len(addOps) != 2 {
		t.Fatalf("expected one proposed instance per Add spec (2), got %d: %+v", len(addOps), addOps)
	}
	seen := map[[2]int]bool{}
	for _, op := range addOps {
		if op.ProposedUniverse < 1 || op.ProposedAddress < 1 {
			t.Fatalf("expected a positive proposed universe/address, got %+v", op)
		}
		if op.ProposedAddress > 512 {
			t.Fatalf("expected the proposed address to stay within one 512-channel universe, got %+v", op)
		}
		key := [2]int{op.ProposedUniverse, op.ProposedAddress}
		if seen[key] {
			t.Fatalf("expected distinct proposed addresses for two adds in the same request, got a collision at %+v", op)
		}
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
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}

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
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	if len(addOps) != 3 {
		t.Fatalf("expected one proposed instance per Add spec (3), got %d: %+v", len(addOps), addOps)
	}
	wantAddresses := []int{1, 6, 11}
	for i, op := range addOps {
		if op.ProposedUniverse != 1 || op.ProposedAddress != wantAddresses[i] {
			t.Fatalf("expected 5-channel-spaced addresses %v, got op[%d]=%+v", wantAddresses, i, op)
		}
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
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{d.ID},
		Propagate:         "preview",
	}
	plan, err := pool.BuildImpactPlan([]pool.Pool{p}, []deployment.Deployment{d}, nil, 0, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	if len(addOps) != 1 {
		t.Fatalf("expected exactly one proposed instance for the force-attached deployment, got %d: %+v", len(addOps), addOps)
	}
	if addOps[0].DependentID != d.ID {
		t.Fatalf("expected the proposed instance to target the force-attached deployment %s, got %s", d.ID, addOps[0].DependentID)
	}
	if addOps[0].ProposedUniverse != 1 || addOps[0].ProposedAddress != 1 {
		t.Fatalf("expected the first proposed instance in a fresh deployment to land at (1, 1), got (%d, %d)", addOps[0].ProposedUniverse, addOps[0].ProposedAddress)
	}
}

// TestBuildImpactPlanForceAttachRejectsUnknownDeployment proves an
// AttachDeployments entry that doesn't exist in the given deployments slice
// fails outright, before any operation is computed.
func TestBuildImpactPlanForceAttachRejectsUnknownDeployment(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	unknownID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{unknownID},
		Propagate:         "preview",
	}
	_, err = pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, 0, req)
	if err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_UNKNOWN_DEPLOYMENT") {
		t.Fatalf("expected GOLC_POOL_PLAN_UNKNOWN_DEPLOYMENT, got %v", err)
	}
}

// TestBuildImpactPlanForceAttachChangesPlanID proves AttachDeployments is
// part of the hashed plan body: two otherwise-identical requests differing
// only in AttachDeployments must produce different plan_ids, since they are
// materially different requests even when a request's resulting Operations
// happen to be empty either way.
func TestBuildImpactPlanForceAttachChangesPlanID(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	deployments := []deployment.Deployment{d}

	baseReq := pool.ImpactRequest{
		PoolID:    p.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		Propagate: "preview",
	}
	without, err := pool.BuildImpactPlan([]pool.Pool{p}, deployments, nil, 0, baseReq)
	if err != nil {
		t.Fatalf("BuildImpactPlan (without attach): %v", err)
	}

	attachedReq := baseReq
	attachedReq.AttachDeployments = []uuid.UUID{d.ID}
	with, err := pool.BuildImpactPlan([]pool.Pool{p}, deployments, nil, 0, attachedReq)
	if err != nil {
		t.Fatalf("BuildImpactPlan (with attach): %v", err)
	}

	if without.PlanID == with.PlanID {
		t.Fatalf("expected AttachDeployments to change plan_id, got the same %q for both", without.PlanID)
	}
}

// TestBuildImpactPlanForceAttachFreshnessRoundTrip proves a force-attach
// plan round-trips through the same integrity/freshness/apply contract
// every other plan does: it validates, applies, and a re-derived plan
// against the post-apply state is correctly rejected as stale (single-use).
func TestBuildImpactPlanForceAttachFreshnessRoundTrip(t *testing.T) {
	p, err := pool.NewPool("Fresh Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	pools := []pool.Pool{p}
	deployments := []deployment.Deployment{d}

	req := pool.ImpactRequest{
		PoolID:            p.ID,
		Add:               []pool.PoolMemberSpec{{FixtureStableKey: "acme/par64", FixtureContentHash: "sha256:eeeeeeee", Mode: "Standard"}},
		AttachDeployments: []uuid.UUID{d.ID},
		Propagate:         "preview",
	}
	plan, err := pool.BuildImpactPlan(pools, deployments, nil, 0, req)
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}
	if err := pool.ValidatePlanIntegrity(plan); err != nil {
		t.Fatalf("ValidatePlanIntegrity: %v", err)
	}
	if err := pool.ValidatePlanFreshness(plan, pools, deployments, nil, 0); err != nil {
		t.Fatalf("ValidatePlanFreshness (pre-apply): %v", err)
	}

	newPools, newDeployments, _, err := pool.Apply(pools, deployments, nil, plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	postApplyRevision := 1 // simulate show.Save's revision bump

	if err := pool.ValidatePlanFreshness(plan, newPools, newDeployments, nil, postApplyRevision); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_PLAN_STALE") {
		t.Fatalf("expected GOLC_POOL_PLAN_STALE re-validating the same plan against post-apply state, got %v", err)
	}
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
	if err != nil {
		t.Fatalf("BuildImpactPlan: %v", err)
	}

	var addOps []pool.ImpactOp
	for _, op := range plan.Operations {
		if op.DependentKind == "deployment_instance" && op.Action == "add" {
			addOps = append(addOps, op)
		}
	}
	if len(addOps) != 2 {
		t.Fatalf("expected 2 proposed instances, got %d: %+v", len(addOps), addOps)
	}
	if addOps[0].ProposedUniverse != 3 || addOps[0].ProposedAddress != 50 {
		t.Fatalf("expected the first proposed instance to anchor at (3, 50), got (%d, %d)", addOps[0].ProposedUniverse, addOps[0].ProposedAddress)
	}
	if addOps[1].ProposedUniverse != 3 || addOps[1].ProposedAddress != 51 {
		t.Fatalf("expected the second proposed instance to auto-increment to (3, 51), got (%d, %d)", addOps[1].ProposedUniverse, addOps[1].ProposedAddress)
	}
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
	if err != nil {
		t.Fatalf("BuildImpactPlan (without fields): %v", err)
	}
	withZero, err := pool.BuildImpactPlan(fx.pools, fx.deployments, fx.groups, fx.revision, explicitZero)
	if err != nil {
		t.Fatalf("BuildImpactPlan (explicit zero): %v", err)
	}
	if without.PlanID != withZero.PlanID {
		t.Fatalf("expected explicit (0,0) StartUniverse/StartAddress to match omitting the fields, got %q vs %q", without.PlanID, withZero.PlanID)
	}
}
