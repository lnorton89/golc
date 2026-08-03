// membership_property_test.go property-tests the pool review-before-apply
// pipeline (BuildImpactPlan -> ValidatePlanIntegrity -> ValidatePlanFreshness
// -> Apply, CONTEXT POOL-03/POOL-04/POOL-05/D-16) against arbitrary
// sequences of member add/remove requests, using pgregory.net/rapid's
// stateful T.Repeat model: after every single step, the real pool's member
// set must exactly match what the sequence of reviewed-and-applied plans
// should have produced -- no member silently dropped, duplicated, or
// left over from a partially applied step. It also generalizes
// plan_test.go's fixed single-use staleness example (one 2-request case)
// across arbitrary starting pool sizes and revisions: a plan reviewed at
// one revision is always rejected once any other request has been
// applied, not just the plan's own repeat-apply.
//
// Both properties scope to a single pool with no deployments/groups,
// isolating membership reconciliation from the dependent-walk/addressing
// concerns impact_test.go already covers.
package pool_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/lnorton89/golc/internal/pool"
)

// membershipModel is the reference model TestPoolMembershipReconciliationProperty
// drives via T.Repeat: the real pool plus the set of member IDs -> fixture
// stable keys the model expects the pipeline to currently hold. Every
// assertion routes through the outer *testing.T (not the *rapid.T action
// parameters, which are only ever used for Draw) -- this mirrors
// reference_property_test.go's proven pattern in internal/projectconfig.
type membershipModel struct {
	t        *testing.T
	pool     pool.Pool
	revision int
	byID     map[uuid.UUID]string
}

func newMembershipModel(t *testing.T) *membershipModel {
	t.Helper()
	p, err := pool.NewPool("Property Pool", nil)
	require.NoError(t, err, "NewPool")
	return &membershipModel{t: t, pool: p, byID: map[uuid.UUID]string{}}
}

// reviewAndApply runs req through the exact production pipeline against
// m's current pool/revision, failing the outer test immediately on any
// unexpected error -- every action below only ever issues a request that
// must succeed.
func (m *membershipModel) reviewAndApply(req pool.ImpactRequest) pool.ImpactPlan {
	m.t.Helper()
	plan, err := pool.BuildImpactPlan([]pool.Pool{m.pool}, nil, nil, m.revision, req)
	require.NoError(m.t, err, "BuildImpactPlan")
	require.NoError(m.t, pool.ValidatePlanIntegrity(plan), "ValidatePlanIntegrity")
	require.NoError(m.t, pool.ValidatePlanFreshness(plan, []pool.Pool{m.pool}, nil, nil, m.revision), "ValidatePlanFreshness")
	newPools, _, _, err := pool.Apply([]pool.Pool{m.pool}, nil, nil, plan)
	require.NoError(m.t, err, "Apply")
	m.pool = newPools[0]
	m.revision++
	return plan
}

// add reviews and applies a single new member with a freshly drawn
// fixture stable key, then records the newly minted PoolMember.ID (always
// the last element after Apply, since this request adds exactly one
// member) in the model.
func (m *membershipModel) add(rt *rapid.T) {
	key := rapid.StringMatching(`[a-z][a-z0-9]{0,7}/[a-z][a-z0-9]{0,7}`).Draw(rt, "fixtureStableKey")
	before := len(m.pool.Members)
	m.reviewAndApply(pool.ImpactRequest{
		PoolID:    m.pool.ID,
		Add:       []pool.PoolMemberSpec{{FixtureStableKey: key, FixtureContentHash: "sha256:" + key}},
		Propagate: "preview",
	})
	require.Len(m.t, m.pool.Members, before+1, "expected exactly one new member after add")
	added := m.pool.Members[len(m.pool.Members)-1]
	m.byID[added.ID] = key
}

// remove reviews and applies removal of one existing member, chosen
// deterministically from a sorted ID list so rapid can shrink the choice.
// A no-op when the model currently tracks no members is a valid step.
func (m *membershipModel) remove(rt *rapid.T) {
	if len(m.byID) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(m.byID))
	for id := range m.byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	target := ids[rapid.IntRange(0, len(ids)-1).Draw(rt, "removeIndex")]

	m.reviewAndApply(pool.ImpactRequest{
		PoolID:    m.pool.ID,
		Remove:    []uuid.UUID{target},
		Propagate: "preview",
	})
	delete(m.byID, target)
}

// check is the T.Repeat invariant action (key ""): rapid runs it before
// the first step and after every subsequent step. The real pool's member
// set must equal the model's tracked set exactly -- same IDs, same
// FixtureStableKey per ID, no more and no fewer.
func (m *membershipModel) check(_ *rapid.T) {
	require.Len(m.t, m.pool.Members, len(m.byID), "pool member count must match the model's tracked count")
	for _, member := range m.pool.Members {
		expectedKey, tracked := m.byID[member.ID]
		require.True(m.t, tracked, "pool member %s is not tracked by the model", member.ID)
		require.Equal(m.t, expectedKey, member.FixtureStableKey, "pool member %s has an unexpected fixture stable key", member.ID)
	}
}

// TestPoolMembershipReconciliationProperty drives arbitrary-length,
// arbitrary-shaped add/remove sequences through the production
// review-before-apply pipeline, asserting the reconciliation invariant
// holds after every single step regardless of sequence shape --
// generalizing impact_test.go's fixed "add then remove the same fixture
// nets back to original state" example.
func TestPoolMembershipReconciliationProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		model := newMembershipModel(t)
		rt.Repeat(map[string]func(*rapid.T){
			"add":    model.add,
			"remove": model.remove,
			"":       model.check,
		})
	})
}

// TestPoolPlanStaleAfterUnrelatedApplyProperty generalizes plan_test.go's
// fixed single-use staleness example (one 2-request, one-member case): for
// an arbitrary starting pool of members and an arbitrary starting
// revision, a plan reviewed against the current revision always validates
// fresh at review time, and always fails ValidatePlanFreshness with
// GOLC_POOL_PLAN_STALE once ANY other request has been applied and bumped
// the revision -- even one that never touches the same members. A stale
// plan is never silently accepted.
func TestPoolPlanStaleAfterUnrelatedApplyProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		p, err := pool.NewPool("Property Pool", nil)
		require.NoError(t, err, "NewPool")

		initialMembers := rapid.IntRange(0, 5).Draw(rt, "initialMembers")
		for i := 0; i < initialMembers; i++ {
			member, err := pool.NewPoolMember(fmt.Sprintf("acme/fixture%d", i), "sha256:seed")
			require.NoError(t, err, "NewPoolMember")
			p.Members = append(p.Members, member)
		}
		revision := rapid.IntRange(0, 100).Draw(rt, "initialRevision")

		reviewedPlan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, revision, pool.ImpactRequest{
			PoolID:    p.ID,
			Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/reviewed", FixtureContentHash: "sha256:reviewed"}},
			Propagate: "preview",
		})
		require.NoError(t, err, "BuildImpactPlan (reviewed)")
		require.NoError(t, pool.ValidatePlanFreshness(reviewedPlan, []pool.Pool{p}, nil, nil, revision),
			"a freshly built plan must validate against its own review-time revision")

		otherPlan, err := pool.BuildImpactPlan([]pool.Pool{p}, nil, nil, revision, pool.ImpactRequest{
			PoolID:    p.ID,
			Add:       []pool.PoolMemberSpec{{FixtureStableKey: "acme/other", FixtureContentHash: "sha256:other"}},
			Propagate: "preview",
		})
		require.NoError(t, err, "BuildImpactPlan (other)")
		require.NoError(t, pool.ValidatePlanIntegrity(otherPlan), "ValidatePlanIntegrity (other)")
		newPools, _, _, err := pool.Apply([]pool.Pool{p}, nil, nil, otherPlan)
		require.NoError(t, err, "Apply (other)")
		p = newPools[0]
		revision++

		err = pool.ValidatePlanFreshness(reviewedPlan, []pool.Pool{p}, nil, nil, revision)
		require.ErrorContains(t, err, "GOLC_POOL_PLAN_STALE",
			"a plan reviewed at an earlier revision must be rejected once any other request has been applied")
	})
}
