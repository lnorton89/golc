// delete_test.go proves DeletePool's cascade contract: deleting a pool
// strips only that pool's own deployment instances and group member refs,
// leaving every other pool/deployment/group untouched, and rejects an
// unknown pool ID before touching anything.
package pool_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
)

func TestDeletePoolCascadesInstancesAndGroupRefs(t *testing.T) {
	poolA, err := pool.NewPool("Pool A", nil)
	if err != nil {
		t.Fatalf("NewPool A: %v", err)
	}
	memberA, err := pool.NewPoolMember("acme/par64", "sha256:aaaaaaaa")
	if err != nil {
		t.Fatalf("NewPoolMember A: %v", err)
	}
	poolA.Members = append(poolA.Members, memberA)

	poolB, err := pool.NewPool("Pool B", nil)
	if err != nil {
		t.Fatalf("NewPool B: %v", err)
	}
	memberB, err := pool.NewPoolMember("acme/par64", "sha256:bbbbbbbb")
	if err != nil {
		t.Fatalf("NewPoolMember B: %v", err)
	}
	poolB.Members = append(poolB.Members, memberB)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
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
	if err != nil {
		t.Fatalf("DeletePool: %v", err)
	}

	if len(newPools) != 1 || newPools[0].ID != poolB.ID {
		t.Fatalf("expected only Pool B to survive, got %+v", newPools)
	}
	if len(newDeployments) != 1 || len(newDeployments[0].Instances) != 1 || newDeployments[0].Instances[0].ID != instanceBID {
		t.Fatalf("expected only Pool B's instance to survive, got %+v", newDeployments)
	}
	if len(newGroups) != 1 || len(newGroups[0].MemberRefs) != 1 || newGroups[0].MemberRefs[0].PoolID != poolB.ID {
		t.Fatalf("expected only the Pool B member ref to survive, got %+v", newGroups)
	}
}

func TestDeletePoolUnknownIDRejected(t *testing.T) {
	p, err := pool.NewPool("Pool A", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	unknownID := uuid.Must(uuid.NewV7())

	_, _, _, err = pool.DeletePool([]pool.Pool{p}, nil, nil, unknownID)
	if err == nil || !strings.Contains(err.Error(), "GOLC_POOL_NOT_FOUND") {
		t.Fatalf("expected GOLC_POOL_NOT_FOUND, got %v", err)
	}
}
