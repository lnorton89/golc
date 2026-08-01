// delete_test.go proves DeleteDeployment removes the named deployment
// (and its own instances go with it, since they're embedded), rejects an
// unknown ID, and confirms deleting the active deployment leaves zero
// active deployments as a still-valid state.
package deployment_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeleteDeploymentRemovesItsInstances(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment A")
	a.Instances = []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Universe: 1, Address: 1}}

	b, err := deployment.NewDeployment("Venue B")
	require.NoError(t, err, "NewDeployment B")
	b.Instances = []deployment.Instance{{ID: uuid.Must(uuid.NewV7()), Universe: 1, Address: 1}}

	remaining, err := deployment.DeleteDeployment([]deployment.Deployment{a, b}, a.ID)
	require.NoError(t, err, "DeleteDeployment")
	require.Len(t, remaining, 1, "expected only Venue B to survive, got %+v", remaining)
	require.Equal(t, b.ID, remaining[0].ID)
}

func TestDeleteDeploymentUnknownIDRejected(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	unknownID := uuid.Must(uuid.NewV7())

	_, err = deployment.DeleteDeployment([]deployment.Deployment{a}, unknownID)
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_NOT_FOUND")
}

func TestDeleteActiveDeploymentLeavesZeroActiveValid(t *testing.T) {
	a, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	a.Active = true

	remaining, err := deployment.DeleteDeployment([]deployment.Deployment{a}, a.ID)
	require.NoError(t, err, "DeleteDeployment")
	require.Empty(t, remaining, "expected zero deployments to remain")
	require.NoError(t, deployment.ValidateSingleActive(remaining), "expected zero active deployments to be a valid state")
}
