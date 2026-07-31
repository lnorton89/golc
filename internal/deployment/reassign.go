// reassign.go implements in-place instance reassignment: changing an
// already-patched Instance's Mode/Universe/Address without removing and
// re-creating it, so its identity (and any Selection.InstanceIDs entry
// referencing it) survives the edit.
package deployment

import (
	"fmt"

	"github.com/google/uuid"
)

// reassignChannelCount mirrors internal/pool/impact.go's
// defaultInstanceChannelCount 1-channel-width simplification (Instance
// does not yet carry its own channel width) -- reused here so Reassign's
// collision check is consistent with NextFreeAddressFrom's own.
const reassignChannelCount = 1

// Reassign returns a copy of existing with the instance identified by
// instanceID having its Mode/Universe/Address replaced. It fails with
// GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND if instanceID is absent from
// existing, with ValidateInstanceAddress's own out-of-range error if the
// new universe/address is invalid, and with
// GOLC_DEPLOYMENT_ADDRESS_COLLISION if the new footprint overlaps any
// OTHER instance's own footprint in existing (the moving instance's own
// prior footprint is excluded from the check against itself, so
// reassigning an instance to its own current address never spuriously
// collides). existing is never mutated.
func Reassign(existing []Instance, instanceID uuid.UUID, mode string, universe, address int) ([]Instance, error) {
	updated := append([]Instance(nil), existing...)
	index := -1
	for i, instance := range updated {
		if instance.ID == instanceID {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, fmt.Errorf("GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND: instance %s does not exist", instanceID)
	}

	candidate := updated[index]
	candidate.Mode, candidate.Universe, candidate.Address = mode, universe, address
	if err := ValidateInstanceAddress(candidate); err != nil {
		return nil, err
	}

	others := make([]Instance, 0, len(updated)-1)
	for i, instance := range updated {
		if i != index {
			others = append(others, instance)
		}
	}
	if overlapsExisting(others, universe, address, reassignChannelCount) {
		return nil, fmt.Errorf("GOLC_DEPLOYMENT_ADDRESS_COLLISION: universe %d address %d collides with another instance in this deployment", universe, address)
	}

	updated[index] = candidate
	return updated, nil
}
