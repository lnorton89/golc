// rename_test.go proves Deployment identity stability across Rename,
// mirroring internal/pool/model_test.go's TestPoolIdentityStable exactly.
package deployment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeploymentIdentityStable(t *testing.T) {
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	originalID := d.ID

	renamed, err := deployment.Rename(d, "Venue A Renamed")
	require.NoError(t, err, "Rename")
	require.Equal(t, originalID, renamed.ID, "expected ID to survive rename")
	require.Equal(t, "Venue A Renamed", renamed.Name, "expected renamed deployment to carry its new name")

	_, err = deployment.Rename(d, "  ")
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_NAME_EMPTY", "expected error for a blank new name")

	other, err := deployment.NewDeployment(d.Name)
	require.NoError(t, err, "NewDeployment (second, same name)")
	err = deployment.ValidateUniqueNames([]deployment.Deployment{d, other})
	require.ErrorContains(t, err, "GOLC_DEPLOYMENT_DUPLICATE_NAME", "expected error for duplicate deployment names")
}
