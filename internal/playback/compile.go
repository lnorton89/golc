// compile.go implements the all-or-nothing Compile step (CONTEXT D-05/D-06,
// 03-RESEARCH.md Pattern 2/Code Examples): Compile mirrors
// internal/pool/impact.go's BuildImpactPlan pure "validate + flatten State
// into an immutable reviewable document, mutate nothing" shape --
// resolving the single active scene's four layers (selection + object
// reference) into an immutable CompiledPlan. A single unresolved
// scene->layer->theme/preset/chase/motion-preset reference or
// out-of-range attribute value fails the WHOLE compile with
// GOLC_PLAYBACK_PLAN_INVALID; Compile never returns a partial plan and
// never mutates its input show.State (it only reads state's fields).
package playback

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// CompiledChaseStep is one Chase step fully resolved at compile time: the
// step's authored Attributes plus its effective instance-membership set
// (the step's own optional Selection when present, resolved once here --
// Evaluate has no I/O and cannot re-resolve a *programming.Selection
// itself -- otherwise the containing layer's own resolved Instances,
// CONTEXT D-03).
type CompiledChaseStep struct {
	Attributes []programming.PresetAttribute
	Instances  map[uuid.UUID]bool
}

// CompiledLayer is one Scene layer's fully resolved, immutable compile-time
// snapshot (CONTEXT D-01..D-04): Instances is the layer's own Selection
// already resolved (D-03: independently scoped, possibly narrower than the
// scene's overall reach) into a fixed instance-ID membership set Evaluate
// filters every per-instance attribute against. Exactly one of
// Preset/Theme/Chase/MotionPreset is populated, matching Kind -- a
// zero-value Ref layer (most commonly a BaseLook with no preset, or any
// disabled layer) leaves all four nil.
type CompiledLayer struct {
	Kind      scene.LayerKind
	Enabled   bool
	Instances map[uuid.UUID]bool

	Preset       *programming.Preset
	Theme        *programming.Theme
	Chase        *programming.Chase
	ChaseSteps   []CompiledChaseStep
	MotionPreset *programming.MotionPreset
}

// CompiledPlan is the immutable, all-or-nothing compiled output of one
// Compile call (CONTEXT D-05/D-06): the single active scene's four layers,
// each fully resolved, plus the show-wide BPM/BarsPerLoop/
// PreserveOnBPMChange values Evaluate and the engine's tick loop (03-07
// Task 2) both need. A CompiledPlan is never itself persisted
// (03-RESEARCH.md Pitfall 3) -- it is a pure, in-memory, per-compile
// snapshot.
type CompiledPlan struct {
	SceneID             uuid.UUID
	BPM                 float64
	BarsPerLoop         int
	PreserveOnBPMChange bool
	Layers              map[scene.LayerKind]CompiledLayer
}

// instanceSet converts a programming.ResolvedSet into a fixed
// instance-ID membership lookup set.
func instanceSet(resolved programming.ResolvedSet) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(resolved.Instances))
	for _, instance := range resolved.Instances {
		set[instance.InstanceID] = true
	}
	return set
}

func findPreset(presets []programming.Preset, id uuid.UUID) (programming.Preset, bool) {
	for _, p := range presets {
		if p.ID == id {
			return p, true
		}
	}
	return programming.Preset{}, false
}

func findTheme(themes []programming.Theme, id uuid.UUID) (programming.Theme, bool) {
	for _, t := range themes {
		if t.ID == id {
			return t, true
		}
	}
	return programming.Theme{}, false
}

func findChase(chases []programming.Chase, id uuid.UUID) (programming.Chase, bool) {
	for _, c := range chases {
		if c.ID == id {
			return c, true
		}
	}
	return programming.Chase{}, false
}

func findMotionPreset(motionPresets []programming.MotionPreset, id uuid.UUID) (programming.MotionPreset, bool) {
	for _, m := range motionPresets {
		if m.ID == id {
			return m, true
		}
	}
	return programming.MotionPreset{}, false
}

// compileChaseSteps resolves every step's effective instance-membership set
// (CONTEXT D-03): a step with a nil Selection reuses the containing
// layer's own already-resolved Instances; a step with a non-nil Selection
// is resolved independently against the same pools/groups/deployments.
func compileChaseSteps(state show.State, chase programming.Chase, layerInstances map[uuid.UUID]bool) ([]CompiledChaseStep, error) {
	steps := make([]CompiledChaseStep, len(chase.Steps))
	for i, step := range chase.Steps {
		instances := layerInstances
		if step.Selection != nil {
			resolved, err := programming.Resolve(state.Pools, state.Groups, state.Deployments, *step.Selection)
			if err != nil {
				return nil, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: chase %q step %d selection: %v", chase.Name, i, err)
			}
			instances = instanceSet(resolved)
		}
		steps[i] = CompiledChaseStep{Attributes: step.Attributes, Instances: instances}
	}
	return steps, nil
}

// compileLayer resolves one Scene layer's Selection and object Ref against
// state, failing all-or-nothing on any unresolved reference or invalid
// value (CONTEXT D-06): a single bad layer fails the WHOLE Compile, never
// producing a partial CompiledPlan.
func compileLayer(state show.State, layer scene.Layer) (CompiledLayer, error) {
	resolved, err := programming.Resolve(state.Pools, state.Groups, state.Deployments, layer.Selection)
	if err != nil {
		return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: layer %q selection: %v", layer.Kind, err)
	}
	compiled := CompiledLayer{Kind: layer.Kind, Enabled: layer.Enabled, Instances: instanceSet(resolved)}

	var zero uuid.UUID
	if layer.Ref == zero {
		return compiled, nil
	}

	switch layer.Kind {
	case scene.BaseLook:
		preset, found := findPreset(state.Presets, layer.Ref)
		if !found {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: base-look layer references preset %s, which does not exist", layer.Ref)
		}
		if err := programming.ValidatePreset(preset); err != nil {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %v", err)
		}
		compiled.Preset = &preset
	case scene.ColorTheme:
		theme, found := findTheme(state.Themes, layer.Ref)
		if !found {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: color-theme layer references theme %s, which does not exist", layer.Ref)
		}
		compiled.Theme = &theme
	case scene.Chase:
		chase, found := findChase(state.Chases, layer.Ref)
		if !found {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: chase layer references chase %s, which does not exist", layer.Ref)
		}
		if err := programming.ValidateChase(chase); err != nil {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %v", err)
		}
		steps, err := compileChaseSteps(state, chase, compiled.Instances)
		if err != nil {
			return CompiledLayer{}, err
		}
		compiled.Chase = &chase
		compiled.ChaseSteps = steps
	case scene.Motion:
		motionPreset, found := findMotionPreset(state.MotionPresets, layer.Ref)
		if !found {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: motion layer references motion preset %s, which does not exist", layer.Ref)
		}
		if err := programming.ValidateMotionPreset(motionPreset); err != nil {
			return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %v", err)
		}
		compiled.MotionPreset = &motionPreset
	default:
		return CompiledLayer{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: layer kind %q is not supported", layer.Kind)
	}
	return compiled, nil
}

// findActiveScene returns the single scene in scenes with Active == true
// (SCEN-04: exactly one scene is active at a time).
func findActiveScene(scenes []scene.Scene) (scene.Scene, bool) {
	for _, s := range scenes {
		if s.Active {
			return s, true
		}
	}
	return scene.Scene{}, false
}

// Compile validates and flattens state's single active scene into an
// immutable CompiledPlan (CONTEXT D-05/D-06, 03-RESEARCH.md Pattern 2/Code
// Examples): every scene->layer->theme/preset/chase/motion-preset
// reference and every resolved attribute value is validated; a single
// unresolved reference or invalid value fails the WHOLE compile with
// GOLC_PLAYBACK_PLAN_INVALID -- Compile never returns a partial plan. A
// State with no active scene fails with GOLC_PLAYBACK_NO_ACTIVE_SCENE. A
// non-positive/non-finite/too-large global BPM fails with
// GOLC_PLAYBACK_PLAN_INVALID (wrapping ValidateBPM). Compile never mutates
// state: it only reads state's fields and returns a brand-new CompiledPlan
// value.
func Compile(state show.State) (CompiledPlan, error) {
	activeScene, found := findActiveScene(state.Scenes)
	if !found {
		return CompiledPlan{}, fmt.Errorf("GOLC_PLAYBACK_NO_ACTIVE_SCENE: state has no active scene")
	}
	if err := ValidateBPM(state.Tempo.BPM); err != nil {
		return CompiledPlan{}, fmt.Errorf("GOLC_PLAYBACK_PLAN_INVALID: %v", err)
	}

	layers := make(map[scene.LayerKind]CompiledLayer, len(activeScene.Layers))
	for _, layer := range activeScene.Layers {
		compiled, err := compileLayer(state, layer)
		if err != nil {
			return CompiledPlan{}, err
		}
		layers[layer.Kind] = compiled
	}

	return CompiledPlan{
		SceneID:             activeScene.ID,
		BPM:                 state.Tempo.BPM,
		BarsPerLoop:         activeScene.BarsPerLoop,
		PreserveOnBPMChange: activeScene.PreserveMusicalPositionOnBPMChange,
		Layers:              layers,
	}, nil
}
