// delete_test.go proves DeletePool's cascade contract: deleting a pool
// strips only that pool's own deployment instances and group member refs,
// leaving every other pool/deployment/group untouched, and rejects an
// unknown pool ID before touching anything.
package pool_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
)

func TestDeletePoolCascadesInstancesAndGroupRefs(t *testing.T) {
	poolA, err := pool.NewPool("Pool A", nil)
	require.NoError(t, err, "NewPool A")
	memberA, err := pool.NewPoolMember("acme/par64", "sha256:aaaaaaaa")
	require.NoError(t, err, "NewPoolMember A")
	poolA.Members = append(poolA.Members, memberA)

	poolB, err := pool.NewPool("Pool B", nil)
	require.NoError(t, err, "NewPool B")
	memberB, err := pool.NewPoolMember("acme/par64", "sha256:bbbbbbbb")
	require.NoError(t, err, "NewPoolMember B")
	poolB.Members = append(poolB.Members, memberB)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	instanceAID := uuid.Must(uuid.NewV7())
	instanceBID := uuid.Must(uuid.NewV7())
	d.Instances = []deployment.Instance{
		{ID: instanceAID, PoolID: poolA.ID, PoolMemberID: memberA.ID, Mode: "Standard", Universe: 1, Address: 1},
		{ID: instanceBID, PoolID: poolB.ID, PoolMemberID: memberB.ID, Mode: "Standard", Universe: 1, Address: 2},
	}

	group := pool.Group{
		Name: "Mixed Group",
		MemberRefs: []pool.MemberRef{
			{PoolID: poolA.ID, PoolMemberID: memberA.ID},
			{PoolID: poolB.ID, PoolMemberID: memberB.ID},
		},
	}

	newPools, newDeployments, newGroups, err := pool.DeletePool(
		[]pool.Pool{poolA, poolB},
		[]deployment.Deployment{d},
		[]pool.Group{group},
		poolA.ID,
	)
	require.NoError(t, err, "DeletePool")

	require.Len(t, newPools, 1, "expected only Pool B to survive, got %+v", newPools)
	require.Equal(t, poolB.ID, newPools[0].ID)
	require.Len(t, newDeployments, 1)
	require.Len(t, newDeployments[0].Instances, 1, "expected only Pool B's instance to survive, got %+v", newDeployments)
	require.Equal(t, instanceBID, newDeployments[0].Instances[0].ID)
	require.Len(t, newGroups, 1)
	require.Len(t, newGroups[0].MemberRefs, 1, "expected only the Pool B member ref to survive, got %+v", newGroups)
	require.Equal(t, poolB.ID, newGroups[0].MemberRefs[0].PoolID)
}

func TestDeletePoolUnknownIDRejected(t *testing.T) {
	p, err := pool.NewPool("Pool A", nil)
	require.NoError(t, err, "NewPool")
	unknownID := uuid.Must(uuid.NewV7())

	_, _, _, err = pool.DeletePool([]pool.Pool{p}, nil, nil, unknownID)
	require.ErrorContains(t, err, "GOLC_POOL_NOT_FOUND")
}
