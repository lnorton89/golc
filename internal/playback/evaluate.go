// evaluate.go implements the pure SCEN-09 evaluator (CONTEXT D-01..D-04,
// 03-RESEARCH.md Pattern 2): Evaluate(plan, pos) walks the fixed layer
// resolution order -- base-look < color-theme < chase < motion -- and
// reduces each enabled layer's already-compiled contribution onto the
// running per-instance result via scene.AttributeSet.Overlay, so a later
// layer overrides an earlier one only for the attributes it actually
// touches.
//
// PRECEDENCE HERE IS FIXED LAYER ORDER, NOT HIGHEST-VALUE-WINS (HTP)
// ARBITRATION AND NOT PER-LAYER BLEND-WEIGHT MIXING (CONTEXT D-02): this
// file never compares the values themselves to decide a winner -- only
// layerPriority's loop order matters.
//
// Every function in this file is a pure function of its arguments: no I/O,
// no goroutines, no package-level mutable state. Calling Evaluate twice --
// or concurrently from many goroutines -- with the same (plan, pos) always
// returns a byte-identical Frame (SCEN-09's mechanical proof).
package playback

import (
	"sort"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
)

// layerPriority is the fixed layer-resolution order (CONTEXT D-02),
// duplicated verbatim from internal/scene's own unexported layerPriority
// (scene.go) since that package deliberately keeps it unexported: a later
// layer in this order always overrides an earlier one for any attribute it
// touches.
var layerPriority = []scene.LayerKind{scene.BaseLook, scene.ColorTheme, scene.Chase, scene.Motion}

// positionInBars returns pos's continuous progress through the current
// scene loop, in bars: the wrapped BarIndex plus the fractional
// BeatFraction, ranging over [0, barsPerLoop) -- a pure function of pos
// alone (CONTEXT SCEN-09), used as the shared "how far into this loop
// iteration are we" clock both chase step selection and motion-preset
// phase derive from (CONTEXT D-10: one authoritative musical clock drives
// both scene looping and chase/motion step advancement).
func positionInBars(pos MusicalPosition) float64 {
	return float64(pos.BarIndex) + pos.BeatFraction
}

// overlayAttribute overlays a single (instance, capability, value) triple
// onto contributions, merging with any attribute already recorded for that
// instance in this same layer's contribution map.
func overlayAttribute(contributions map[uuid.UUID]scene.AttributeSet, instance uuid.UUID, capability fixture.CapabilityType, value float64) {
	existing := contributions[instance]
	contributions[instance] = existing.Overlay(scene.AttributeSet{
		Values: map[fixture.CapabilityType]float64{capability: value},
	})
}

// resolveBaseLook returns the base-look layer's per-instance contribution:
// every PresetAttribute in the resolved preset whose InstanceID is a
// member of the layer's own resolved Selection (CONTEXT D-03 -- a layer's
// Selection scopes which fixtures it actually touches, narrower than
// whatever else the scene reaches). An instance in the preset's authored
// data but outside this layer's Selection contributes nothing.
func resolveBaseLook(layer CompiledLayer) map[uuid.UUID]scene.AttributeSet {
	contributions := map[uuid.UUID]scene.AttributeSet{}
	if layer.Preset == nil {
		return contributions
	}
	for _, attr := range layer.Preset.Attributes {
		if !layer.Instances[attr.InstanceID] {
			continue
		}
		overlayAttribute(contributions, attr.InstanceID, attr.Capability, attr.Value)
	}
	return contributions
}

// resolveColorTheme returns the color-theme layer's per-instance
// contribution: every ColorAssignment in the resolved theme whose
// InstanceID is a member of the layer's own resolved Selection (CONTEXT
// D-03), applied to fixture.CapabilityColor.
func resolveColorTheme(layer CompiledLayer) map[uuid.UUID]scene.AttributeSet {
	contributions := map[uuid.UUID]scene.AttributeSet{}
	if layer.Theme == nil {
		return contributions
	}
	for _, assignment := range layer.Theme.Colors {
		if !layer.Instances[assignment.InstanceID] {
			continue
		}
		overlayAttribute(contributions, assignment.InstanceID, fixture.CapabilityColor, assignment.Value)
	}
	return contributions
}

// chaseStepIndex computes the currently-active step index of a chase with
// stepCount steps advancing every stepDuration StepUnit-units, at the
// given loop-relative bars position (CONTEXT D-10, 03-RESEARCH.md Pattern
// 2): a pure function of its four arguments -- never an accumulated
// "current step" counter -- so a stalled/coalesced tick changes only when
// the engine computes the answer, never what step index a given position
// resolves to (SCEN-09).
func chaseStepIndex(bars float64, stepUnit programming.StepUnit, stepDuration float64, stepCount int) int {
	if stepCount == 0 || stepDuration <= 0 {
		return 0
	}
	unit := bars
	if stepUnit == programming.StepUnitBeat {
		unit = bars * beatsPerBar
	}
	step := int(unit/stepDuration) % stepCount
	if step < 0 {
		step += stepCount
	}
	return step
}

// resolveChase returns the chase layer's per-instance contribution: the
// currently-active step's (already compile-time-resolved, CONTEXT D-03)
// Attributes, filtered to the step's own effective instance-membership set
// -- a pure function of the layer's compiled steps and pos alone.
func resolveChase(layer CompiledLayer, pos MusicalPosition) map[uuid.UUID]scene.AttributeSet {
	contributions := map[uuid.UUID]scene.AttributeSet{}
	if layer.Chase == nil || len(layer.ChaseSteps) == 0 {
		return contributions
	}
	bars := positionInBars(pos)
	index := chaseStepIndex(bars, layer.Chase.StepUnit, layer.Chase.StepDuration, len(layer.ChaseSteps))
	step := layer.ChaseSteps[index]
	for _, attr := range step.Attributes {
		if !step.Instances[attr.InstanceID] {
			continue
		}
		overlayAttribute(contributions, attr.InstanceID, attr.Capability, attr.Value)
	}
	return contributions
}

// motionPhase returns the motion preset's normalized [0,1) run phase at
// the given loop-relative bars position: a motion preset runs exactly once
// per scene loop iteration (CONTEXT D-10 -- the same bar-position clock
// that drives scene looping), so phase is simply the loop-relative bars
// position divided by barsPerLoop.
func motionPhase(bars float64, barsPerLoop int) float64 {
	if barsPerLoop <= 0 {
		return 0
	}
	return bars / float64(barsPerLoop)
}

// activeMotionKeyframe returns the last keyframe (by ascending Phase)
// whose Phase is <= phase; if phase precedes every keyframe's Phase, the
// keyframe with the greatest Phase applies -- a step-function selection
// over explicitly authored points, never interpolated or randomized
// (CONTEXT D-09).
func activeMotionKeyframe(keyframes []programming.MotionKeyframe, phase float64) programming.MotionKeyframe {
	sorted := append([]programming.MotionKeyframe(nil), keyframes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Phase < sorted[j].Phase })

	active := sorted[len(sorted)-1]
	for _, k := range sorted {
		if k.Phase <= phase {
			active = k
		}
	}
	return active
}

// resolveMotion returns the motion layer's per-instance contribution: the
// currently-active keyframe's position/beam values (CONTEXT D-04) applied
// uniformly to every instance in the layer's resolved Selection --
// MotionKeyframeValue carries no InstanceID (a motion preset's authored
// path is selection-wide by design, unlike a per-instance-recorded
// preset/theme).
func resolveMotion(layer CompiledLayer, pos MusicalPosition, barsPerLoop int) map[uuid.UUID]scene.AttributeSet {
	contributions := map[uuid.UUID]scene.AttributeSet{}
	if layer.MotionPreset == nil || len(layer.MotionPreset.Keyframes) == 0 {
		return contributions
	}
	phase := motionPhase(positionInBars(pos), barsPerLoop)
	keyframe := activeMotionKeyframe(layer.MotionPreset.Keyframes, phase)

	values := make(map[fixture.CapabilityType]float64, len(keyframe.Values))
	for _, v := range keyframe.Values {
		values[v.Capability] = v.Value
	}
	attrSet := scene.AttributeSet{Values: values}
	for instance := range layer.Instances {
		contributions[instance] = attrSet
	}
	return contributions
}

// resolveLayerContribution dispatches to the per-kind resolver for kind.
func resolveLayerContribution(kind scene.LayerKind, layer CompiledLayer, pos MusicalPosition, barsPerLoop int) map[uuid.UUID]scene.AttributeSet {
	switch kind {
	case scene.BaseLook:
		return resolveBaseLook(layer)
	case scene.ColorTheme:
		return resolveColorTheme(layer)
	case scene.Chase:
		return resolveChase(layer, pos)
	case scene.Motion:
		return resolveMotion(layer, pos, barsPerLoop)
	default:
		return nil
	}
}

// Evaluate is the pure SCEN-09 evaluator: it walks the fixed layerPriority
// order (CONTEXT D-01..D-04), resolves each enabled layer's contribution
// at pos, and reduces via scene.AttributeSet.Overlay so a later layer
// overrides an earlier one only for the attributes it actually touches.
// Evaluate performs no I/O, spawns no goroutines, and touches no
// package-level mutable state: calling it twice -- or concurrently from
// many goroutines -- with the same (plan, pos) always returns a
// byte-identical Frame (SCEN-09's mechanical proof).
func Evaluate(plan CompiledPlan, pos MusicalPosition) Frame {
	perInstance := map[uuid.UUID]scene.AttributeSet{}

	for _, kind := range layerPriority {
		layer, ok := plan.Layers[kind]
		if !ok || !layer.Enabled {
			continue
		}

		contribution := resolveLayerContribution(kind, layer, pos, plan.BarsPerLoop)
		for instance, attrs := range contribution {
			perInstance[instance] = perInstance[instance].Overlay(attrs)
		}
	}

	return Frame{Values: perInstance}
}
