// rename_test.go proves Deployment identity stability across Rename,
// mirroring internal/pool/model_test.go's TestPoolIdentityStable exactly.
package deployment_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/deployment"
)

func TestDeploymentIdentityStable(t *testing.T) {
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	originalID := d.ID

	renamed, err := deployment.Rename(d, "Venue A Renamed")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to survive rename, got %s want %s", renamed.ID, originalID)
	}
	if renamed.Name != "Venue A Renamed" {
		t.Fatalf("expected renamed deployment to carry its new name, got %q", renamed.Name)
	}

	if _, err := deployment.Rename(d, "  "); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_NAME_EMPTY") {
		t.Fatalf("expected GOLC_DEPLOYMENT_NAME_EMPTY for a blank new name, got %v", err)
	}

	other, err := deployment.NewDeployment(d.Name)
	if err != nil {
		t.Fatalf("NewDeployment (second, same name): %v", err)
	}
	if err := deployment.ValidateUniqueNames([]deployment.Deployment{d, other}); err == nil || !strings.Contains(err.Error(), "GOLC_DEPLOYMENT_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_DEPLOYMENT_DUPLICATE_NAME for duplicate deployment names, got %v", err)
	}
}
