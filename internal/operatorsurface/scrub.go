// scrub.go implements the scene-delete-cascade safety net for operator
// surfaces: internal/command/programming.go's runSceneDelete calls
// ScrubSceneReferences before show.Save so deleting a scene an operator
// surface is currently wired to never fails with
// GOLC_OPERATORSURFACE_DANGLING_REFERENCE -- it just unassigns that scene
// from the surface instead, mirroring internal/scene/scrub.go's
// ScrubLayerRef precedent (deleting a look a scene actively plays resets
// that scene's layer rather than blocking the delete).
package operatorsurface

import "github.com/google/uuid"

// ScrubSceneReferences returns a copy of surfaces with sceneID removed from
// every SceneRefs membership set (via UnassignScene) and every LayerRef
// naming sceneID removed from LayerRefs -- the two places a Surface can
// reference a scene by ID. MasterRefs/SafetyRefs/MidiMappings are never
// touched: neither references a scene directly. A surface referencing a
// different scene, or no scene at all, passes through unchanged.
func ScrubSceneReferences(surfaces []Surface, sceneID uuid.UUID) []Surface {
	scrubbed := make([]Surface, len(surfaces))
	for i, s := range surfaces {
		s = UnassignScene(s, sceneID)
		var filteredLayerRefs []LayerRef
		for _, ref := range s.LayerRefs {
			if ref.SceneID != sceneID {
				filteredLayerRefs = append(filteredLayerRefs, ref)
			}
		}
		s.LayerRefs = filteredLayerRefs
		scrubbed[i] = s
	}
	return scrubbed
}
