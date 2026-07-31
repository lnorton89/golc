// scrub.go implements the Scene-Selection dangling-reference safety net:
// internal/show/store.go's Save calls ScrubDanglingSelections before
// validate() so a pool/deployment/group cascade delete can never leave a
// Scene's persisted Layer.Selection referencing something that no longer
// exists. scene.ValidateLayerReferences (scene.go) only checks Layer.Ref
// (a theme/chase/motion reference) -- it never inspects Layer.Selection,
// which is why this scrub exists as a separate, additive step rather than
// a new validate() rejection (rejecting here would make an otherwise-valid
// cascade delete fail Save, which is not the desired cascade-delete
// behavior).
package scene

import (
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
)

// ScrubDanglingSelections returns a copy of scenes with every layer's
// Selection passed through programming.ScrubDangling against pools/
// groups/deployments. Only Selection is ever touched -- Kind/Enabled/Ref
// pass through unchanged. A Scene with nothing dangling is returned
// unchanged (element-for-element).
func ScrubDanglingSelections(scenes []Scene, pools []pool.Pool, groups []pool.Group, deployments []deployment.Deployment) []Scene {
	scrubbed := make([]Scene, len(scenes))
	for i, s := range scenes {
		for j, layer := range s.Layers {
			s.Layers[j].Selection = programming.ScrubDangling(layer.Selection, pools, groups, deployments)
		}
		scrubbed[i] = s
	}
	return scrubbed
}
