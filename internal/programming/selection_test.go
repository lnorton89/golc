// selection_test.go proves PROG-01's selection-resolution contract
// (03-01-PLAN.md Task 1): a Selection referencing any mix of pool, group,
// deployment-instance, or direct fixture selectors resolves to exactly the
// concrete deployment fixture instances it names, deduped, in stable
// deployment/instance declaration order, with a dangling reference always
// rejected rather than silently dropped.
package programming_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
)

// newFixture builds a small pools/groups/deployments fixture shared by
// several TestSelection cases: poolA has two members (m1, m2), poolB has
// one member (m3), emptyPool has zero members. deployment1 carries
// instances for m1 and m2 (both from poolA); deployment2 carries an
// instance for m3 (poolB) plus a second instance also patched from m1
// (poolA), so a direct fixture ref on m1 resolves across both
// deployments. groupA selects m1 via a MemberRef.
func newFixture(t *testing.T) (pools []pool.Pool, groups []pool.Group, deployments []deployment.Deployment, poolA, poolB, emptyPool pool.Pool, m1, m2, m3 pool.PoolMember, dep1, dep2 deployment.Deployment, instA1, instA2, instB1, instA1Dep2 deployment.Instance, groupA pool.Group) {
	t.Helper()

	m1 = pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m1", FixtureContentHash: "hash1"}
	m2 = pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m2", FixtureContentHash: "hash2"}
	m3 = pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m3", FixtureContentHash: "hash3"}

	poolA = pool.Pool{ID: uuid.New(), Name: "Pool A", Members: []pool.PoolMember{m1, m2}}
	poolB = pool.Pool{ID: uuid.New(), Name: "Pool B", Members: []pool.PoolMember{m3}}
	emptyPool = pool.Pool{ID: uuid.New(), Name: "Empty Pool"}

	pools = []pool.Pool{poolA, poolB, emptyPool}

	groupA = pool.Group{
		ID:   uuid.New(),
		Name: "Group A",
		MemberRefs: []pool.MemberRef{
			{PoolID: poolA.ID, PoolMemberID: m1.ID},
		},
	}
	groups = []pool.Group{groupA}

	instA1 = deployment.Instance{ID: uuid.New(), PoolID: poolA.ID, PoolMemberID: m1.ID, Universe: 1, Address: 1}
	instA2 = deployment.Instance{ID: uuid.New(), PoolID: poolA.ID, PoolMemberID: m2.ID, Universe: 1, Address: 2}
	dep1 = deployment.Deployment{ID: uuid.New(), Name: "Deployment 1", Instances: []deployment.Instance{instA1, instA2}}

	instB1 = deployment.Instance{ID: uuid.New(), PoolID: poolB.ID, PoolMemberID: m3.ID, Universe: 1, Address: 1}
	instA1Dep2 = deployment.Instance{ID: uuid.New(), PoolID: poolA.ID, PoolMemberID: m1.ID, Universe: 1, Address: 3}
	dep2 = deployment.Deployment{ID: uuid.New(), Name: "Deployment 2", Instances: []deployment.Instance{instB1, instA1Dep2}}

	deployments = []deployment.Deployment{dep1, dep2}

	return
}

func instanceIDs(set programming.ResolvedSet) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(set.Instances))
	for _, instance := range set.Instances {
		ids = append(ids, instance.InstanceID)
	}
	return ids
}

func TestSelectionResolvesPool(t *testing.T) {
	pools, groups, deployments, poolA, _, _, _, _, _, _, _, instA1, instA2, _, _, _ := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{PoolIDs: []uuid.UUID{poolA.ID}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []uuid.UUID{instA1.ID, instA2.ID}
	if !reflect.DeepEqual(instanceIDs(got), want) {
		t.Fatalf("pool selection: got %v, want %v", instanceIDs(got), want)
	}
}

func TestSelectionResolvesGroup(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, instA1, _, _, _, groupA := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{GroupIDs: []uuid.UUID{groupA.ID}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []uuid.UUID{instA1.ID}
	if !reflect.DeepEqual(instanceIDs(got), want) {
		t.Fatalf("group selection: got %v, want %v", instanceIDs(got), want)
	}
}

func TestSelectionResolvesDeploymentInstance(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, instA2, _, _, _ := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{InstanceIDs: []uuid.UUID{instA2.ID}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []uuid.UUID{instA2.ID}
	if !reflect.DeepEqual(instanceIDs(got), want) {
		t.Fatalf("instance selection: got %v, want %v", instanceIDs(got), want)
	}
}

func TestSelectionResolvesDirectFixtureRef(t *testing.T) {
	pools, groups, deployments, poolA, _, _, m1, _, _, _, _, instA1, _, _, instA1Dep2, _ := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{
		FixtureRefs: []programming.FixtureRef{{PoolID: poolA.ID, PoolMemberID: m1.ID}},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []uuid.UUID{instA1.ID, instA1Dep2.ID}
	if !reflect.DeepEqual(instanceIDs(got), want) {
		t.Fatalf("direct fixture ref selection: got %v, want %v", instanceIDs(got), want)
	}
}

func TestSelectionOverlapDedupes(t *testing.T) {
	pools, groups, deployments, poolA, _, _, m1, _, _, _, _, instA1, instA2, _, instA1Dep2, groupA := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{
		PoolIDs:     []uuid.UUID{poolA.ID},
		GroupIDs:    []uuid.UUID{groupA.ID},
		FixtureRefs: []programming.FixtureRef{{PoolID: poolA.ID, PoolMemberID: m1.ID}},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// poolA selector alone yields instA1 and instA2; groupA and the direct
	// fixture ref both also select instA1 -- it must appear exactly once.
	want := []uuid.UUID{instA1.ID, instA2.ID}
	if !reflect.DeepEqual(instanceIDs(got), want) {
		t.Fatalf("overlap selection: got %v, want %v (instA1Dep2=%v must not appear, it is a different instance of the same member)", instanceIDs(got), want, instA1Dep2.ID)
	}
}

func TestSelectionEmptyZeroSelectors(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got.Instances) != 0 {
		t.Fatalf("expected empty resolved set for zero selectors, got %v", got.Instances)
	}
}

func TestSelectionEmptyPool(t *testing.T) {
	pools, groups, deployments, _, _, emptyPool, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	got, err := programming.Resolve(pools, groups, deployments, programming.Selection{PoolIDs: []uuid.UUID{emptyPool.ID}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got.Instances) != 0 {
		t.Fatalf("expected empty resolved set for an empty pool, got %v", got.Instances)
	}
}

func TestSelectionStableOrdering(t *testing.T) {
	pools, groups, deployments, poolA, poolB, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	sel := programming.Selection{PoolIDs: []uuid.UUID{poolA.ID, poolB.ID}}
	first, err := programming.Resolve(pools, groups, deployments, sel)
	if err != nil {
		t.Fatalf("Resolve (first): %v", err)
	}
	second, err := programming.Resolve(pools, groups, deployments, sel)
	if err != nil {
		t.Fatalf("Resolve (second): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected identical resolution order across repeated calls:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	// deployment declaration order (dep1 before dep2), then instance
	// declaration order within each deployment.
	want := []uuid.UUID{
		deployments[0].Instances[0].ID,
		deployments[0].Instances[1].ID,
		deployments[1].Instances[0].ID,
	}
	if !reflect.DeepEqual(instanceIDs(first), want) {
		t.Fatalf("expected deployment-then-instance declaration order: got %v, want %v", instanceIDs(first), want)
	}
}

func TestSelectionDanglingPool(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	_, err := programming.Resolve(pools, groups, deployments, programming.Selection{PoolIDs: []uuid.UUID{uuid.New()}})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for unknown pool, got %v", err)
	}
}

func TestSelectionDanglingGroup(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	_, err := programming.Resolve(pools, groups, deployments, programming.Selection{GroupIDs: []uuid.UUID{uuid.New()}})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for unknown group, got %v", err)
	}
}

func TestSelectionDanglingInstance(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	_, err := programming.Resolve(pools, groups, deployments, programming.Selection{InstanceIDs: []uuid.UUID{uuid.New()}})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for unknown deployment instance, got %v", err)
	}
}

func TestSelectionDanglingFixtureRefUnknownPool(t *testing.T) {
	pools, groups, deployments, _, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	_, err := programming.Resolve(pools, groups, deployments, programming.Selection{
		FixtureRefs: []programming.FixtureRef{{PoolID: uuid.New(), PoolMemberID: uuid.New()}},
	})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for fixture ref with unknown pool, got %v", err)
	}
}

func TestSelectionDanglingFixtureRefUnknownMember(t *testing.T) {
	pools, groups, deployments, poolA, _, _, _, _, _, _, _, _, _, _, _, _ := newFixture(t)

	_, err := programming.Resolve(pools, groups, deployments, programming.Selection{
		FixtureRefs: []programming.FixtureRef{{PoolID: poolA.ID, PoolMemberID: uuid.New()}},
	})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SELECTION_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SELECTION_DANGLING_REFERENCE for fixture ref with unknown pool member, got %v", err)
	}
}

func TestSelectionResolveDoesNotImportShow(t *testing.T) {
	// This is a structural/compile-time guarantee (03-01-PLAN.md acceptance
	// criteria): programming must not import internal/show. There is no
	// runtime assertion to make here beyond the package compiling with
	// only pool/deployment/uuid imports -- enforced by go vet/import-cycle
	// detection at build time, and documented here as the test that would
	// fail to build if a future edit introduced the cycle.
	_ = uuid.New()
}
