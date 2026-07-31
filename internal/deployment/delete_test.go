// delete_test.go proves DeleteDeployment removes the named deployment
// (and its own instances go with it, since they're embedded), rejects an
// unknown ID, and confirms deleting the active deployment leaves zero
// active deployments as a still-valid state.
package deployment_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeleteDeploymentRemovesItsInstances(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment A: %v", err)
	}
	a.Instances = []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Universe: 1, Address: 1}}

	b, err := deployment.NewDeployment("Venue B")
	if err != nil {
		t.Fatalf("NewDeployment B: %v", err)
	}
	b.Instances = []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Universe: 1, Address: 1}}

	remaining, err := deployment.DeleteDeployment([]deployment.Deployment{a, b}, a.ID)
	if err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != b.ID {
		t.Fatalf("expected only Venue B to survive, got %+v", remaining)
	}
}

func TestDeleteDeploymentUnknownIDRejected(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	unknownID := uuid.Must(uuid.NewV7())

	if _, err := deployment.DeleteDeployment([]deployment.Deployment{a}, unknownID); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_NOT_FOUND") {
		t.Fatalf("expected GOLC_DEPLOYMENT_NOT_FOUND, got %v", err)
	}
}

func TestDeleteActiveDeploymentLeavesZeroActiveValid(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	a.Active = true

	remaining, err := deployment.DeleteDeployment([]deployment.Deployment{a}, a.ID)
	if err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected zero deployments to remain, got %+v", remaining)
	}
	if err := deployment.ValidateSingleActive(remaining); err != nil {
		t.Fatalf("expected zero active deployments to be a valid state, got %v", err)
	}
}
