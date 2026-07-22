// selection.go implements PROG-01's fixture selection resolver: a
// Selection (a set of pool/group/deployment-instance/direct-fixture
// selectors) resolves against a show's plain pool/group/deployment
// slices into a concrete, deterministically ordered ResolvedSet of
// deployment fixture instances. Resolve takes plain slices rather than a
// show.State value -- mirroring internal/pool/impact.go's BuildImpactPlan
// -- because internal/show already imports internal/pool (State embeds
// []pool.Pool/[]pool.Group), so this package cannot import internal/show
// without an import cycle.
package programming

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
)

// FixtureRef selects one PoolMember of one Pool directly for inclusion in
// a Selection, mirroring pool.MemberRef's {PoolID, PoolMemberID} shape.
type FixtureRef struct {
	PoolID       uuid.UUID `json:"pool_id"`
	PoolMemberID uuid.UUID `json:"pool_member_id"`
}

// Selection is the full set of selectors a show author has chosen
// (PROG-01): any mix of pool, group, deployment-instance, and direct
// fixture selectors, resolved together into one deduped ResolvedSet. A
// Selection with every field empty is valid and resolves to an empty
// ResolvedSet with no error.
type Selection struct {
	PoolIDs     []uuid.UUID  `json:"pool_ids,omitempty"`
	GroupIDs    []uuid.UUID  `json:"group_ids,omitempty"`
	InstanceIDs []uuid.UUID  `json:"instance_ids,omitempty"`
	FixtureRefs []FixtureRef `json:"fixture_refs,omitempty"`
}

// ResolvedInstance is one concrete deployment fixture instance a
// Selection resolved to.
type ResolvedInstance struct {
	DeploymentID uuid.UUID `json:"deployment_id"`
	InstanceID   uuid.UUID `json:"instance_id"`
	PoolID       uuid.UUID `json:"pool_id"`
	PoolMemberID uuid.UUID `json:"pool_member_id"`
}

// ResolvedSet is the deterministic, deduped result of resolving a
// Selection: every matched fixture instance appears exactly once, in
// (deployment declaration order, then instance declaration order) --
// identical across repeated resolutions of the same show state.
type ResolvedSet struct {
	Instances []ResolvedInstance `json:"instances,omitempty"`
}

// Resolve resolves sel against pools/groups/deployments into a
// ResolvedSet (PROG-01). Every selector that references a pool, group,
// deployment instance, or direct fixture pool member that does not exist
// is rejected with GOLC_SELECTION_DANGLING_REFERENCE -- an unresolved
// reference is always a diagnostic, never a silently smaller resolved
// set. A fixture reachable through more than one selector (for example
// both its pool and a direct fixture ref) appears exactly once in the
// result (dedupe by InstanceID). Resolve is a pure function of its slice
// inputs: it never mutates pools/groups/deployments and never imports
// internal/show.
func Resolve(pools []pool.Pool, groups []pool.Group, deployments []deployment.Deployment, sel Selection) (ResolvedSet, error) {
	poolExists := make(map[uuid.UUID]bool, len(pools))
	membersByPool := make(map[uuid.UUID]map[uuid.UUID]bool, len(pools))
	for _, p := range pools {
		poolExists[p.ID] = true
		members := make(map[uuid.UUID]bool, len(p.Members))
		for _, m := range p.Members {
			members[m.ID] = true
		}
		membersByPool[p.ID] = members
	}

	groupByID := make(map[uuid.UUID]pool.Group, len(groups))
	for _, g := range groups {
		groupByID[g.ID] = g
	}

	instanceExists := make(map[uuid.UUID]bool)
	for _, d := range deployments {
		for _, instance := range d.Instances {
			instanceExists[instance.ID] = true
		}
	}

	// Validate every selector resolves before building the result --
	// resolution never silently drops an unresolved reference.
	for _, id := range sel.PoolIDs {
		if !poolExists[id] {
			return ResolvedSet{}, fmt.Errorf("GOLC_SELECTION_DANGLING_REFERENCE: selection references pool %s, which does not exist", id)
		}
	}
	for _, id := range sel.GroupIDs {
		if _, ok := groupByID[id]; !ok {
			return ResolvedSet{}, fmt.Errorf("GOLC_SELECTION_DANGLING_REFERENCE: selection references group %s, which does not exist", id)
		}
	}
	for _, id := range sel.InstanceIDs {
		if !instanceExists[id] {
			return ResolvedSet{}, fmt.Errorf("GOLC_SELECTION_DANGLING_REFERENCE: selection references deployment instance %s, which does not exist", id)
		}
	}
	for _, ref := range sel.FixtureRefs {
		members, poolFound := membersByPool[ref.PoolID]
		if !poolFound {
			return ResolvedSet{}, fmt.Errorf("GOLC_SELECTION_DANGLING_REFERENCE: selection references pool %s, which does not exist", ref.PoolID)
		}
		if !members[ref.PoolMemberID] {
			return ResolvedSet{}, fmt.Errorf("GOLC_SELECTION_DANGLING_REFERENCE: selection references pool member %s in pool %s, which does not exist", ref.PoolMemberID, ref.PoolID)
		}
	}

	// Build membership lookup sets for the (now-validated) selectors.
	// Groups resolve to the fixture refs their MemberRefs name.
	selectedPools := make(map[uuid.UUID]bool, len(sel.PoolIDs))
	for _, id := range sel.PoolIDs {
		selectedPools[id] = true
	}
	selectedInstances := make(map[uuid.UUID]bool, len(sel.InstanceIDs))
	for _, id := range sel.InstanceIDs {
		selectedInstances[id] = true
	}
	selectedFixtureRefs := make(map[FixtureRef]bool, len(sel.FixtureRefs))
	for _, ref := range sel.FixtureRefs {
		selectedFixtureRefs[ref] = true
	}
	for _, id := range sel.GroupIDs {
		for _, ref := range groupByID[id].MemberRefs {
			selectedFixtureRefs[FixtureRef{PoolID: ref.PoolID, PoolMemberID: ref.PoolMemberID}] = true
		}
	}

	// Walk deployments/instances in the caller's own (already
	// deterministic) declaration order -- never a map -- so the result is
	// byte-identical across repeated calls on identical input.
	seen := make(map[uuid.UUID]bool)
	var instances []ResolvedInstance
	for _, d := range deployments {
		for _, instance := range d.Instances {
			matched := selectedPools[instance.PoolID] ||
				selectedInstances[instance.ID] ||
				selectedFixtureRefs[FixtureRef{PoolID: instance.PoolID, PoolMemberID: instance.PoolMemberID}]
			if !matched || seen[instance.ID] {
				continue
			}
			seen[instance.ID] = true
			instances = append(instances, ResolvedInstance{
				DeploymentID: d.ID,
				InstanceID:   instance.ID,
				PoolID:       instance.PoolID,
				PoolMemberID: instance.PoolMemberID,
			})
		}
	}

	return ResolvedSet{Instances: instances}, nil
}
