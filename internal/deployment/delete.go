// delete.go implements deployment deletion: a deployment's own instances
// go with it. Nothing else references a Deployment by ID except through
// its own Instances (a Scene's Layer.Selection.InstanceIDs is the one
// exception, handled separately by programming.ScrubDangling/
// scene.ScrubDanglingSelections, called from show.Save).
package deployment

import (
	"fmt"

	"github.com/google/uuid"
)

// DeleteDeployment removes the deployment identified by deploymentID from
// deployments; its own instances go with it. Leaving zero active
// deployments afterward is always valid -- ValidateSingleActive only
// rejects more than one active, never zero. deployments is never
// mutated; a deploymentID absent from deployments fails with
// GOLC_DEPLOYMENT_NOT_FOUND before anything is touched.
func DeleteDeployment(deployments []Deployment, deploymentID uuid.UUID) ([]Deployment, error) {
	kept := make([]Deployment, 0, len(deployments))
	found := false
	for _, d := range deployments {
		if d.ID == deploymentID {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		return nil, fmt.Errorf("GOLC_DEPLOYMENT_NOT_FOUND: deployment %s does not exist", deploymentID)
	}
	return kept, nil
}
