// validate.go re-checks deskmidi's own invariants before internal/show's
// validate() trusts or persists a show.State: no two Mappings share a
// (Channel, Kind, Number) tuple (defense in depth -- AddMapping already
// rejects a colliding candidate at write time, but this re-checks the
// invariant against the whole persisted collection, mirroring
// internal/operatorsurface/validate.go's identical "re-check what the
// mutator already enforced" discipline), and every Mapping's InstanceID
// resolves to a patch instance actually present in the owning State's
// Deployments (any deployment, not just the currently active one -- a
// mapping survives switching which deployment is active).
package deskmidi

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
)

// Validate is the single entry point internal/show/state.go's validate()
// calls to check every Mapping in mappings against the owning State's
// deployments.
func Validate(mappings []Mapping, deployments []deployment.Deployment) error {
	seen := make(map[Mapping]bool, len(mappings))
	for _, m := range mappings {
		key := Mapping{Channel: m.Channel, Kind: m.Kind, Number: m.Number}
		if seen[key] {
			return fmt.Errorf(
				"GOLC_DESKMIDI_DUPLICATE_MAPPING: channel=%d kind=%s number=%d is mapped more than once",
				m.Channel, m.Kind, m.Number)
		}
		seen[key] = true
	}

	instanceExists := make(map[string]bool)
	for _, d := range deployments {
		for _, instance := range d.Instances {
			instanceExists[instance.ID.String()] = true
		}
	}

	for _, m := range mappings {
		if _, err := uuid.Parse(m.InstanceID); err != nil {
			return fmt.Errorf("GOLC_DESKMIDI_INSTANCE_ID_INVALID: mapping %s carries an invalid instance id %q", m.ID, m.InstanceID)
		}
		if !instanceExists[m.InstanceID] {
			return fmt.Errorf(
				"GOLC_DESKMIDI_DANGLING_REFERENCE: mapping %s references patch instance %s, which does not exist",
				m.ID, m.InstanceID)
		}
	}
	return nil
}
