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
