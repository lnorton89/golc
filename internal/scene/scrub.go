// scrub.go implements two dangling-reference safety nets:
//
// ScrubDanglingSelections runs unconditionally inside internal/show/
// store.go's Save (before validate()) so a pool/deployment/group cascade
// delete can never leave a Scene's persisted Layer.Selection referencing
// something that no longer exists. scene.ValidateLayerReferences (scene.go)
// only checks Layer.Ref (a theme/preset/chase/motion reference) -- it never
// inspects Layer.Selection, which is why this scrub exists as a separate,
// additive step rather than a new validate() rejection (rejecting here
// would make an otherwise-valid cascade delete fail Save, which is not the
// desired cascade-delete behavior).
//
// ScrubLayerRef is deliberately NOT wired into Save unconditionally --
// unlike Selection, Layer.Ref DOES have an existing validate() check
// (ValidateLayerReferences), which must keep rejecting a directly-assigned
// bad Ref (e.g. "scene layer set --ref <garbage>"): that is a user typo,
// not a cascade side-effect, and should fail loudly. Instead, each of
// theme/preset/chase/motion delete (internal/command/programming.go) calls
// ScrubLayerRef itself, targeted at the exact (Kind, ID) pair being
// deleted, right before show.Save -- so removing a look a scene actively
// plays resets that scene's layer to its default, un-refed state instead
// of blocking the delete, while any other (still-valid) Ref is left alone.
package scene

import (
	"github.com/google/uuid"

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

// ScrubLayerRef returns a copy of scenes with every layer of the given kind
// whose Ref equals id reset to its default, un-refed state (Ref cleared to
// the zero UUID, Enabled cleared to false) -- the same zero-value a fresh
// scene's layer starts in (see newLayers in scene.go). Selection and every
// layer of a different Kind or pointing at a different Ref pass through
// unchanged. A scene with nothing matching is returned unchanged
// (element-for-element).
func ScrubLayerRef(scenes []Scene, kind LayerKind, id uuid.UUID) []Scene {
	scrubbed := make([]Scene, len(scenes))
	for i, s := range scenes {
		for j, layer := range s.Layers {
			if layer.Kind == kind && layer.Ref == id {
				s.Layers[j].Ref = uuid.UUID{}
				s.Layers[j].Enabled = false
			}
		}
		scrubbed[i] = s
	}
	return scrubbed
}
