// reassign_test.go proves Reassign's in-place mode/universe/address
// update contract: the target instance's fields change without a new ID,
// an out-of-range address is rejected, a collision with another instance
// is rejected, an unknown instance ID is rejected, and reassigning an
// instance to its own current address never spuriously self-collides.
package deployment_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestReassignUpdatesModeUniverseAddress(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{{ID: id, Mode: "Standard", Universe: 1, Address: 1}}

	updated, err := deployment.Reassign(existing, id, "Extended", 2, 50)
	if err != nil {
		t.Fatalf("Reassign: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected exactly one instance, got %d", len(updated))
	}
	if updated[0].ID != id {
		t.Fatalf("expected the instance ID to survive reassignment, got %s want %s", updated[0].ID, id)
	}
	if updated[0].Mode != "Extended" || updated[0].Universe != 2 || updated[0].Address != 50 {
		t.Fatalf("expected mode/universe/address to update, got %+v", updated[0])
	}
}

func TestReassignUnknownInstanceRejected(t *testing.T) {
	existing := []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Mode: "Standard", Universe: 1, Address: 1}}
	unknownID := uuid.Must(uuid.NewV7())

	if _, err := deployment.Reassign(existing, unknownID, "Standard", 1, 2); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND") {
		t.Fatalf("expected GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND, got %v", err)
	}
}

func TestReassignOutOfRangeAddressRejected(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{{ID: id, Mode: "Standard", Universe: 1, Address: 1}}

	if _, err := deployment.Reassign(existing, id, "Standard", 1, 513); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE, got %v", err)
	}
	if _, err := deployment.Reassign(existing, id, "Standard", 0, 1); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE") {
		t.Fatalf("expected GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE for universe 0, got %v", err)
	}
}

func TestReassignCollisionWithAnotherInstanceRejected(t *testing.T) {
	movingID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())
	existing := []deployment.Instance{
		{ID: movingID, Mode: "Standard", Universe: 1, Address: 1},
		{ID: otherID, Mode: "Standard", Universe: 1, Address: 5},
	}

	if _, err := deployment.Reassign(existing, movingID, "Standard", 1, 5); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_ADDRESS_COLLISION") {
		t.Fatalf("expected GOLC_DEPLOYMENT_ADDRESS_COLLISION, got %v", err)
	}
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
	if err != nil {
		t.Fatalf("Reassign to own current address: %v", err)
	}
	if updated[0].Mode != "Extended" || updated[0].Universe != 1 || updated[0].Address != 1 {
		t.Fatalf("expected mode updated and address unchanged, got %+v", updated[0])
	}
}
