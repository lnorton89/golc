// delete.go implements cascade pool deletion: removing a pool also strips
// every deployment instance and group member ref that references it, so a
// deleted pool never leaves a dangling instance/member-ref behind --
// mirroring plan.go's existing Apply removal cascade shape, but for
// deleting the whole pool rather than one member.
package pool

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
)

// DeletePool removes the pool identified by poolID from pools, cascading
// the removal to every deployment.Instance whose PoolID matches (across
// every deployment) and every Group.MemberRef whose PoolID matches
// (across every group). Input slices are never mutated; poolID absent
// from pools fails with GOLC_POOL_NOT_FOUND before anything is touched.
func DeletePool(pools []Pool, deployments []deployment.Deployment, groups []Group, poolID uuid.UUID) ([]Pool, []deployment.Deployment, []Group, error) {
	newPools := make([]Pool, 0, len(pools))
	found := false
	for _, p := range pools {
		if p.ID == poolID {
			found = true
			continue
		}
		newPools = append(newPools, p)
	}
	if !found {
		return nil, nil, nil, fmt.Errorf("GOLC_POOL_NOT_FOUND: pool %s does not exist", poolID)
	}

	newDeployments := make([]deployment.Deployment, len(deployments))
	for i, d := range deployments {
		kept := make([]deployment.Instance, 0, len(d.Instances))
		for _, instance := range d.Instances {
			if instance.PoolID != poolID {
				kept = append(kept, instance)
			}
		}
		d.Instances = kept
		newDeployments[i] = d
	}

	newGroups := make([]Group, len(groups))
	for i, g := range groups {
		kept := make([]MemberRef, 0, len(g.MemberRefs))
		for _, ref := range g.MemberRefs {
			if ref.PoolID != poolID {
				kept = append(kept, ref)
			}
		}
		g.MemberRefs = kept
		newGroups[i] = g
	}

	return newPools, newDeployments, newGroups, nil
}
