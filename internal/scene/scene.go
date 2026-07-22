// scene.go declares the Scene/Layer domain model (CONTEXT SCEN-01/SCEN-04/
// SCEN-05, D-01..D-03): a Scene loops for a configured number of musical
// bars against the global BPM and combines four independently enabled,
// independently selectable layers -- base-look, color-theme, chase, motion
// -- resolved by a fixed priority order (see layerPriority and ReduceLayers
// in layer.go). Scene copies internal/pool/model.go's identity/
// construction/rename/unique-name shape verbatim (03-PATTERNS.md):
// identity is a durable UUIDv7 minted once at creation, never derived from
// Name, and never re-minted by RenameScene. ValidateSingleActiveScene/
// ActivateScene mirror internal/deployment/model.go's ValidateSingleActive/
// Activate exactly (SCEN-04): a State with more than one active scene is
// rejected, and Activate returns a fresh copy so two scenes are never
// simultaneously active even transiently.
package scene

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/programming"
)

// LayerKind names one of a Scene's four fixed layer slots (CONTEXT
// D-01..D-03).
type LayerKind string

// The four fixed layer kinds every Scene carries exactly one of, in the
// fixed resolution-priority order D-01..D-04 establish.
const (
	BaseLook   LayerKind = "base_look"
	ColorTheme LayerKind = "color_theme"
	Chase      LayerKind = "chase"
	Motion     LayerKind = "motion"
)

// layerPriority is the fixed layer-resolution order (CONTEXT D-02): a later
// layer in this order always overrides an earlier one for any attribute it
// touches. This is loop order, NOT highest-value-wins (HTP) arbitration and
// NOT per-layer blend-weight mixing -- see ReduceLayers in layer.go for the
// guard comment at the actual reduce site (RESEARCH.md anti-pattern: a
// contributor with prior lighting-console experience may reach for HTP by
// habit).
var layerPriority = []LayerKind{BaseLook, ColorTheme, Chase, Motion}

// maxBarsPerLoop bounds Scene.BarsPerLoop (DoS ceiling, CONTEXT threat
// T-03-02, mirrors internal/deployment's maxUniverseSearch precedent): a
// pathologically large bar-loop is rejected with GOLC_SCENE_BARS_INVALID
// rather than allowed to grow unbounded.
const maxBarsPerLoop = 1024

// Layer is one of a Scene's four fixed layer slots (CONTEXT D-01..D-03):
// Kind names which fixed slot this is; Enabled independently toggles
// whether this layer contributes at all; Selection independently scopes
// which fixtures this layer targets, narrower than the scene's overall
// reach (D-03); Ref points at the reusable programming object this layer
// plays (a color-theme/chase/motion layer's Ref should resolve to a Theme/
// Chase/MotionPreset respectively -- see ValidateLayerReferences). A
// zero-value Ref is only meaningful for a BaseLook layer, which may instead
// carry inline rest-state values rather than referencing a Preset (CONTEXT
// D-01: base-look is the scene's foundational rest state, not necessarily a
// reference to a reusable object).
type Layer struct {
	Kind      LayerKind             `json:"kind"`
	Enabled   bool                  `json:"enabled"`
	Selection programming.Selection `json:"selection"`
	Ref       uuid.UUID             `json:"ref,omitempty"`
}

// Scene is a tempo-aware looping performance container (SCEN-01/SCEN-05):
// it loops for BarsPerLoop musical bars against the global BPM and combines
// four independently enabled, independently selectable layers resolved by
// the fixed layerPriority order. PreserveMusicalPositionOnBPMChange is the
// SCEN-08 per-scene config flag the clock plan (03-06) reads when the
// global BPM changes. Identity is a durable UUIDv7 minted once at creation
// -- never derived from Name, and never re-minted by RenameScene.
type Scene struct {
	ID                                 uuid.UUID `json:"id"`
	Name                               string    `json:"name"`
	Active                             bool      `json:"active"`
	BarsPerLoop                        int       `json:"bars_per_loop"`
	PreserveMusicalPositionOnBPMChange bool      `json:"preserve_musical_position_on_bpm_change"`
	Layers                             [4]Layer  `json:"layers"`
}

// newLayers returns the four fixed layer slots in layerPriority order, each
// carrying its own Kind and starting disabled with a zero Selection/Ref.
func newLayers() [4]Layer {
	var layers [4]Layer
	for i, kind := range layerPriority {
		layers[i] = Layer{Kind: kind}
	}
	return layers
}

// validateLayerKind rejects any LayerKind outside the four declared values
// (GOLC_SCENE_LAYER_KIND_INVALID).
func validateLayerKind(kind LayerKind) error {
	for _, known := range layerPriority {
		if kind == known {
			return nil
		}
	}
	return fmt.Errorf("GOLC_SCENE_LAYER_KIND_INVALID: %q is not a supported layer kind", kind)
}

// LayerByKind returns the Layer in s.Layers whose Kind matches kind. Every
// Scene always carries exactly one Layer per fixed kind (constructed by
// NewScene/newLayers), so a Scene built through this package always finds a
// match for one of the four declared LayerKind values.
func (s Scene) LayerByKind(kind LayerKind) (Layer, bool) {
	for _, layer := range s.Layers {
		if layer.Kind == kind {
			return layer, true
		}
	}
	return Layer{}, false
}

// SetLayer returns a copy of s with the layer slot matching layer.Kind
// replaced by layer; every other layer slot is left untouched. Fails with
// GOLC_SCENE_LAYER_KIND_INVALID if layer.Kind is not one of the four
// declared LayerKind values.
func SetLayer(s Scene, layer Layer) (Scene, error) {
	if err := validateLayerKind(layer.Kind); err != nil {
		return Scene{}, err
	}
	updated := s
	for i, existing := range updated.Layers {
		if existing.Kind == layer.Kind {
			updated.Layers[i] = layer
			return updated, nil
		}
	}
	return Scene{}, fmt.Errorf("GOLC_SCENE_LAYER_KIND_INVALID: scene %q has no layer slot for kind %q", s.Name, layer.Kind)
}

// validateBarsPerLoop rejects any BarsPerLoop outside [1, maxBarsPerLoop]
// (GOLC_SCENE_BARS_INVALID): barsPerLoop = 1 is valid (loops every bar);
// 0 or negative, and anything above the declared ceiling, is rejected.
func validateBarsPerLoop(barsPerLoop int) error {
	if barsPerLoop < 1 {
		return fmt.Errorf("GOLC_SCENE_BARS_INVALID: bars_per_loop %d must be at least 1", barsPerLoop)
	}
	if barsPerLoop > maxBarsPerLoop {
		return fmt.Errorf("GOLC_SCENE_BARS_INVALID: bars_per_loop %d exceeds the maximum of %d", barsPerLoop, maxBarsPerLoop)
	}
	return nil
}

// NewScene mints a fresh UUIDv7-identified, inactive Scene with all four
// layers present but disabled. IDs are minted only at creation time --
// never derived from Name, and never re-minted by RenameScene.
func NewScene(name string, barsPerLoop int) (Scene, error) {
	if strings.TrimSpace(name) == "" {
		return Scene{}, fmt.Errorf("GOLC_SCENE_NAME_EMPTY: scene name must not be empty")
	}
	if err := validateBarsPerLoop(barsPerLoop); err != nil {
		return Scene{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Scene{}, fmt.Errorf("GOLC_SCENE_ID_MINT_FAILED: %v", err)
	}
	return Scene{ID: id, Name: name, BarsPerLoop: barsPerLoop, Layers: newLayers()}, nil
}

// RenameScene returns s with Name replaced by newName; ID is never
// re-minted (identity is rename-stable).
func RenameScene(s Scene, newName string) (Scene, error) {
	if strings.TrimSpace(newName) == "" {
		return Scene{}, fmt.Errorf("GOLC_SCENE_NAME_EMPTY: scene name must not be empty")
	}
	s.Name = newName
	return s, nil
}

// ValidateScene re-checks every invariant a hand-edited or otherwise
// untrusted Scene must satisfy before it is trusted: Name is non-empty,
// BarsPerLoop is within [1, maxBarsPerLoop], and every layer's Kind is one
// of the four declared values.
func ValidateScene(s Scene) error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("GOLC_SCENE_NAME_EMPTY: scene %s declares an empty name", s.ID)
	}
	if err := validateBarsPerLoop(s.BarsPerLoop); err != nil {
		return err
	}
	for _, layer := range s.Layers {
		if err := validateLayerKind(layer.Kind); err != nil {
			return err
		}
	}
	return nil
}

// ValidateSceneUniqueNames rejects any two scenes in scenes sharing the
// same Name: a duplicate name is always rejected with a diagnostic, never
// silently permitted.
func ValidateSceneUniqueNames(scenes []Scene) error {
	seen := make(map[string]bool, len(scenes))
	for _, s := range scenes {
		if seen[s.Name] {
			return fmt.Errorf("GOLC_SCENE_DUPLICATE_NAME: a scene named %q already exists", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// ValidateSingleActiveScene rejects any scenes slice with more than one
// Active=true entry, mirroring internal/deployment/model.go's
// ValidateSingleActive exactly (SCEN-04): zero active scenes is valid
// (nothing playing); more than one is always rejected.
func ValidateSingleActiveScene(scenes []Scene) error {
	activeCount := 0
	for _, s := range scenes {
		if s.Active {
			activeCount++
		}
	}
	if activeCount > 1 {
		return fmt.Errorf("GOLC_SCENE_MULTIPLE_ACTIVE: %d scenes are marked active; exactly one is allowed", activeCount)
	}
	return nil
}

// ActivateScene returns a copy of scenes with exactly the named scene
// Active and every other scene inactive, so the caller can never observe
// two active scenes even transiently (SCEN-04), mirroring
// internal/deployment/model.go's Activate exactly. It fails with
// GOLC_SCENE_NOT_FOUND if no scene in scenes carries the given name;
// scenes itself is never mutated.
func ActivateScene(scenes []Scene, name string) ([]Scene, error) {
	found := false
	activated := make([]Scene, len(scenes))
	for i, s := range scenes {
		s.Active = s.Name == name
		if s.Active {
			found = true
		}
		activated[i] = s
	}
	if !found {
		return nil, fmt.Errorf("GOLC_SCENE_NOT_FOUND: no scene named %q exists", name)
	}
	return activated, nil
}

// ValidateLayerReferences rejects any scene layer whose Ref points at a
// programming object that does not exist (CONTEXT threat T-03-01, mirrors
// internal/pool/model.go's ValidateGroupReferences build-lookup-then-check
// shape): a ColorTheme layer's Ref must resolve against themes, a Chase
// layer's Ref must resolve against chases, and a Motion layer's Ref must
// resolve against motionPresets. A zero-value Ref (uuid.UUID{}) is never
// checked against any collection -- it means "no object reference" (only
// meaningful for a BaseLook layer, or for a disabled layer of any kind);
// when a BaseLook layer does carry a non-zero Ref, it must resolve against
// presets (a full intensity/color/position/beam rest-state bundle).
func ValidateLayerReferences(scenes []Scene, themes []programming.Theme, presets []programming.Preset, chases []programming.Chase, motionPresets []programming.MotionPreset) error {
	themeExists := make(map[uuid.UUID]bool, len(themes))
	for _, t := range themes {
		themeExists[t.ID] = true
	}
	presetExists := make(map[uuid.UUID]bool, len(presets))
	for _, p := range presets {
		presetExists[p.ID] = true
	}
	chaseExists := make(map[uuid.UUID]bool, len(chases))
	for _, c := range chases {
		chaseExists[c.ID] = true
	}
	motionExists := make(map[uuid.UUID]bool, len(motionPresets))
	for _, m := range motionPresets {
		motionExists[m.ID] = true
	}

	var zero uuid.UUID
	for _, s := range scenes {
		for _, layer := range s.Layers {
			if layer.Ref == zero {
				continue
			}
			switch layer.Kind {
			case BaseLook:
				if !presetExists[layer.Ref] {
					return fmt.Errorf("GOLC_SCENE_LAYER_DANGLING_REFERENCE: scene %q base-look layer references preset %s, which does not exist", s.Name, layer.Ref)
				}
			case ColorTheme:
				if !themeExists[layer.Ref] {
					return fmt.Errorf("GOLC_SCENE_LAYER_DANGLING_REFERENCE: scene %q color-theme layer references theme %s, which does not exist", s.Name, layer.Ref)
				}
			case Chase:
				if !chaseExists[layer.Ref] {
					return fmt.Errorf("GOLC_SCENE_LAYER_DANGLING_REFERENCE: scene %q chase layer references chase %s, which does not exist", s.Name, layer.Ref)
				}
			case Motion:
				if !motionExists[layer.Ref] {
					return fmt.Errorf("GOLC_SCENE_LAYER_DANGLING_REFERENCE: scene %q motion layer references motion preset %s, which does not exist", s.Name, layer.Ref)
				}
			}
		}
	}
	return nil
}
