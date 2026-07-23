// validate.go re-checks every operator-surface invariant Load and Save
// both enforce before trusting or persisting a show.State (CONTEXT threat
// T-06-02): unique surface names across the collection, then referential
// integrity -- every SceneRef, LayerRef, and GroupMaster MasterRef must
// resolve against a scene/layer-slot/group actually present in the owning
// State. This mirrors internal/pool/model.go's ValidateUniqueGroupNames +
// ValidateGroupReferences two-step shape exactly (06-PATTERNS.md).
// GrandMaster refs and SafetyRefs are enum-fixed and never dangle -- there
// is no external collection for either to resolve against.
package operatorsurface

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
)

// Validate is the single entry point internal/show/state.go's validate()
// calls to check every Surface in surfaces against the owning State's
// scenes and groups: first ValidateUniqueSurfaceNames, then referential
// integrity for every SceneRef/LayerRef/GroupMaster MasterRef (mirrors
// pool.ValidateGroupReferences' build-lookup-then-check shape).
func Validate(surfaces []Surface, scenes []scene.Scene, groups []pool.Group) error {
	if err := ValidateUniqueSurfaceNames(surfaces); err != nil {
		return err
	}

	sceneExists := make(map[uuid.UUID]bool, len(scenes))
	sceneLayerKinds := make(map[uuid.UUID]map[scene.LayerKind]bool, len(scenes))
	for _, sc := range scenes {
		sceneExists[sc.ID] = true
		kinds := make(map[scene.LayerKind]bool, len(sc.Layers))
		for _, layer := range sc.Layers {
			kinds[layer.Kind] = true
		}
		sceneLayerKinds[sc.ID] = kinds
	}

	groupExists := make(map[uuid.UUID]bool, len(groups))
	for _, g := range groups {
		groupExists[g.ID] = true
	}

	for _, s := range surfaces {
		for _, sceneRef := range s.SceneRefs {
			if !sceneExists[sceneRef] {
				return fmt.Errorf(
					"GOLC_OPERATORSURFACE_DANGLING_REFERENCE: surface %q references scene %s, which does not exist",
					s.Name, sceneRef)
			}
		}
		for _, layerRef := range s.LayerRefs {
			kinds, sceneOK := sceneLayerKinds[layerRef.SceneID]
			if !sceneOK {
				return fmt.Errorf(
					"GOLC_OPERATORSURFACE_DANGLING_REFERENCE: surface %q references scene %s for a layer, which does not exist",
					s.Name, layerRef.SceneID)
			}
			if !kinds[layerRef.Kind] {
				return fmt.Errorf(
					"GOLC_OPERATORSURFACE_DANGLING_REFERENCE: surface %q references layer %q of scene %s, which does not exist",
					s.Name, layerRef.Kind, layerRef.SceneID)
			}
		}
		for _, masterRef := range s.MasterRefs {
			if masterRef.Kind == GroupMaster && !groupExists[masterRef.GroupID] {
				return fmt.Errorf(
					"GOLC_OPERATORSURFACE_DANGLING_REFERENCE: surface %q references group %s for a group master, which does not exist",
					s.Name, masterRef.GroupID)
			}
		}
	}
	return nil
}
