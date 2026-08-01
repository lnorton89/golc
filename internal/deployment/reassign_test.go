// reassign_test.go proves Reassign's in-place mode/universe/address
// update contract: the target instance's fields change without a new ID,
// an out-of-range address is rejected, a collision with another instance
// is rejected, an unknown instance ID is rejected, and reassigning an
// instance to its own current address never spuriously self-collides.
package deployment_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestReassignUpdatesModeUniverseAddress(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{{ID: id, Mode: "Standard", Universe: 1, Address: 1}}

	updated, err := deployment.Reassign(existing, id, "Extended", 2, 50)
	require.NoError(t, err, "Reassign")
	require.Len(t, updated, 1)
	require.Equal(t, id, updated[0].ID, "expected the instance ID to survive reassignment")
	require.Equal(t, "Extended", updated[0].Mode, "expected mode/universe/address to update, got %+v", updated[0])
	require.Equal(t, 2, updated[0].Universe, "expected mode/universe/address to update, got %+v", updated[0])
	require.Equal(t, 50, updated[0].Address, "expected mode/universe/address to update, got %+v", updated[0])
}

func TestReassignUnknownInstanceRejected(t *testing.T) {
	existing := []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Mode: "Standard", Universe: 1, Address: 1}}
	unknownID := uuid.Must(uuid.NewV7())

	_, err := deployment.Reassign(existing, unknownID, "Standard", 1, 2)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND")
}

func TestReassignOutOfRangeAddressRejected(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{{ID: id, Mode: "Standard", Universe: 1, Address: 1}}

	_, err := deployment.Reassign(existing, id, "Standard", 1, 513)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE")
	_, err = deployment.Reassign(existing, id, "Standard", 0, 1)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE", "expected error for universe 0")
}

func TestReassignCollisionWithAnotherInstanceRejected(t *testing.T) {
	movingID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{
		{ID: movingID, Mode: "Standard", Universe: 1, Address: 1},
		{ID: otherID, Mode: "Standard", Universe: 1, Address: 5},
	}

	_, err := deployment.Reassign(existing, movingID, "Standard", 1, 5)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_ADDRESS_COLLISION")
}

func TestReassignToOwnCurrentAddressNeverSelfCollides(t *testing.T) {
	movingID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{
		{ID: movingID, Mode: "Standard", Universe: 1, Address: 1},
		{ID: otherID, Mode: "Standard", Universe: 1, Address: 5},
	}

	// Reassigning only the Mode, leaving universe/address unchanged, must
	// succeed even with another instance present elsewhere -- the moving
	// instance's own prior footprint must never collide with itself.
	updated, err := deployment.Reassign(existing, movingID, "Extended", 1, 1)
	require.NoError(t, err, "Reassign to own current address")
	require.Equal(t, "Extended", updated[0].Mode, "expected mode updated and address unchanged, got %+v", updated[0])
	require.Equal(t, 1, updated[0].Universe, "expected mode updated and address unchanged, got %+v", updated[0])
	require.Equal(t, 1, updated[0].Address, "expected mode updated and address unchanged, got %+v", updated[0])
}
