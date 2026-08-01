// model_test.go proves Pool identity stability and count-independence
// (02-04-PLAN.md, Task 1 Wave-0 scaffold): a Pool's UUID survives a
// rename, two pools may share a name only if creation of the duplicate
// name is rejected, and a Pool with 0, 1, or ~50 members is equally
// valid.
//
// This file fails at build time until internal/pool exists (Task 2) --
// that is the RED state this task proves.
package pool_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/pool"
)

func TestPoolIdentityStable(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	originalID := p.ID

	renamed, err := pool.Rename(p, "Wash Pool Renamed")
	require.NoError(t, err, "Rename")
	require.Equal(t, originalID, renamed.ID, "expected ID to survive rename")
	require.Equal(t, "Wash Pool Renamed", renamed.Name, "expected renamed pool to carry its new name")

	// Two pools may share a name only if creation of the duplicate is
	// rejected (GOLC_POOL_DUPLICATE_NAME) -- never a silent duplicate.
	other, err := pool.NewPool(p.Name, nil)
	require.NoError(t, err, "NewPool (second, same name)")
	err = pool.ValidateUniqueNames([]pool.Pool{p, other})
	require.ErrorContains(t, err, "GOLC_POOL_DUPLICATE_NAME", "expected GOLC_POOL_DUPLICATE_NAME for duplicate pool names")
}

func TestGroupUniqueNamesRejected(t *testing.T) {
	first := pool.Group{Name: "Front Wash"}
	second := pool.Group{Name: "Front Wash"}
	err := pool.ValidateUniqueGroupNames([]pool.Group{first, second})
	require.ErrorContains(t, err, "GOLC_GROUP_DUPLICATE_NAME", "expected GOLC_GROUP_DUPLICATE_NAME for duplicate group names")
	err = pool.ValidateUniqueGroupNames([]pool.Group{first, {Name: "Back Wash"}})
	require.NoError(t, err, "expected distinctly named groups to be valid")
}

func TestGroupReferencesValidated(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	valid := pool.Group{
		Name:       "Front Wash",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
	}
	err = pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{valid})
	require.NoError(t, err, "expected a group referencing a real pool member to be valid")

	danglingPool := pool.Group{
		Name:       "Dangling Pool Ref",
		MemberRefs: []pool.MemberRef{{PoolID: uuid.Must(uuid.NewV7()), PoolMemberID: member.ID}},
	}
	err = pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{danglingPool})
	require.ErrorContains(t, err, "GOLC_GROUP_DANGLING_REFERENCE", "expected GOLC_GROUP_DANGLING_REFERENCE for a reference to a nonexistent pool")

	danglingMember := pool.Group{
		Name:       "Dangling Member Ref",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: uuid.Must(uuid.NewV7())}},
	}
	err = pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{danglingMember})
	require.ErrorContains(t, err, "GOLC_GROUP_DANGLING_REFERENCE", "expected GOLC_GROUP_DANGLING_REFERENCE for a reference to a nonexistent pool member")
}

func TestPoolCountIndependent(t *testing.T) {
	zero, err := pool.NewPool("Zero Members", nil)
	require.NoError(t, err, "NewPool")
	require.NoError(t, pool.Validate(zero), "expected a zero-member pool to be valid")

	one, err := pool.NewPool("One Member", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	require.NoError(t, err, "NewPoolMember")
	one.Members = append(one.Members, member)
	require.NoError(t, pool.Validate(one), "expected a one-member pool to be valid")

	many, err := pool.NewPool("Fifty Members", nil)
	require.NoError(t, err, "NewPool")
	for i := 0; i < 50; i++ {
		m, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
		require.NoError(t, err, "NewPoolMember (%d)", i)
		many.Members = append(many.Members, m)
	}
	require.NoError(t, pool.Validate(many), "expected a 50-member pool to be valid")
	require.Len(t, many.Members, 50)
}
