// evaluate_property_test.go property-tests Evaluate's chase-step and
// motion-keyframe cue-transition selection (CONTEXT SCEN-09, D-09/D-10),
// generalizing evaluate_test.go's fixed hand-picked positions
// (TestEvaluateChaseStepAdvancesWithPosition's 4 bars,
// TestEvaluateMotionKeyframeSelection's 2 keyframes/2 positions) across
// arbitrary step counts/durations/units/positions and arbitrary
// keyframe sets/positions. CompiledPlan/CompiledLayer are hand-built
// directly here rather than through Compile(), the same lightweight seam
// impact_test.go uses for pool.Pool/deployment.Deployment -- Evaluate is
// documented as a pure function of these plain structs alone.
package playback_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
)

// beatsPerBar mirrors clock.go's unexported fixed v1 time signature
// (4/4, "the v1 fixed time signature") so this property's independent
// oracle can restate chaseStepIndex's beat-unit conversion without
// reaching into the unexported constant itself.
const beatsPerBar = 4.0

// TestEvaluateChaseStepIndexProperty generalizes
// TestEvaluateChaseStepAdvancesWithPosition across arbitrary step counts,
// step durations, step units (bar/beat), and musical positions: the
// active chase step always matches chaseStepIndex's documented formula
// (int(unit/stepDuration) % stepCount, unit being bars or bars*beatsPerBar
// depending on StepUnit), computed here independently as an oracle rather
// than by reaching into the unexported function.
func TestEvaluateChaseStepIndexProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		instanceID, err := uuid.NewV7()
		require.NoError(t, err, "uuid.NewV7")

		stepCount := rapid.IntRange(1, 8).Draw(rt, "stepCount")
		stepDuration := rapid.Float64Range(0.1, 4).Draw(rt, "stepDuration")
		useBeats := rapid.Bool().Draw(rt, "useBeats")
		stepUnit := programming.StepUnitBar
		if useBeats {
			stepUnit = programming.StepUnitBeat
		}
		barIndex := rapid.IntRange(0, 20).Draw(rt, "barIndex")
		beatFraction := rapid.Float64Range(0, 0.999).Draw(rt, "beatFraction")

		steps := make([]playback.CompiledChaseStep, stepCount)
		for i := 0; i < stepCount; i++ {
			steps[i] = playback.CompiledChaseStep{
				Attributes: []programming.PresetAttribute{
					{InstanceID: instanceID, Capability: fixture.CapabilityIntensity, Value: float64(i) / float64(stepCount)},
				},
				Instances: map[uuid.UUID]bool{instanceID: true},
			}
		}

		plan := playback.CompiledPlan{
			Layers: map[scene.LayerKind]playback.CompiledLayer{
				scene.Chase: {
					Kind:       scene.Chase,
					Enabled:    true,
					Instances:  map[uuid.UUID]bool{instanceID: true},
					Chase:      &programming.Chase{StepUnit: stepUnit, StepDuration: stepDuration},
					ChaseSteps: steps,
				},
			},
		}
		pos := playback.MusicalPosition{BarIndex: barIndex, BeatFraction: beatFraction}

		bars := float64(barIndex) + beatFraction
		unit := bars
		if useBeats {
			unit = bars * beatsPerBar
		}
		expectedIndex := int(unit/stepDuration) % stepCount
		expectedValue := float64(expectedIndex) / float64(stepCount)

		frame := playback.Evaluate(plan, pos)
		got := frame.Values[instanceID].Values[fixture.CapabilityIntensity]
		require.InDelta(t, expectedValue, got, 1e-9,
			"bar=%d beatFraction=%v stepUnit=%v stepDuration=%v stepCount=%d: expected step index %d",
			barIndex, beatFraction, stepUnit, stepDuration, stepCount, expectedIndex)
	})
}

// TestEvaluateMotionKeyframeSelectionProperty generalizes
// TestEvaluateMotionKeyframeSelection across an arbitrary number of
// distinct keyframes and an arbitrary in-loop position: the active
// keyframe is always the last one (by ascending Phase) whose Phase is <=
// the current phase, or the greatest-Phase keyframe if the current phase
// precedes every keyframe -- a step-function selection, never
// interpolated (CONTEXT D-09) -- computed here independently as an
// oracle. Positions are kept within [0, barsPerLoop) (one loop iteration),
// matching the engine's own contract of always supplying an
// already-wrapped BarIndex (CONTEXT D-10); phase selection once bars
// exceeds barsPerLoop is the caller's wrapping responsibility, not this
// function's own spec.
func TestEvaluateMotionKeyframeSelectionProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		instanceID, err := uuid.NewV7()
		require.NoError(t, err, "uuid.NewV7")

		keyframeCount := rapid.IntRange(1, 6).Draw(rt, "keyframeCount")
		keyframes := make([]programming.MotionKeyframe, keyframeCount)
		phases := make([]float64, keyframeCount)
		for i := 0; i < keyframeCount; i++ {
			phase := rapid.Float64Range(0, 0.999).Draw(rt, fmt.Sprintf("phase%d", i))
			phases[i] = phase
			keyframes[i] = programming.MotionKeyframe{
				Phase:  phase,
				Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: float64(i)}},
			}
		}

		barsPerLoop := rapid.IntRange(1, 8).Draw(rt, "barsPerLoop")
		barIndex := rapid.IntRange(0, barsPerLoop-1).Draw(rt, "barIndex")
		beatFraction := rapid.Float64Range(0, 0.999).Draw(rt, "beatFraction")

		plan := playback.CompiledPlan{
			BarsPerLoop: barsPerLoop,
			Layers: map[scene.LayerKind]playback.CompiledLayer{
				scene.Motion: {
					Kind:         scene.Motion,
					Enabled:      true,
					Instances:    map[uuid.UUID]bool{instanceID: true},
					MotionPreset: &programming.MotionPreset{Keyframes: keyframes},
				},
			},
		}
		pos := playback.MusicalPosition{BarIndex: barIndex, BeatFraction: beatFraction}

		bars := float64(barIndex) + beatFraction
		phase := bars / float64(barsPerLoop)

		sortedIndices := make([]int, keyframeCount)
		for i := range sortedIndices {
			sortedIndices[i] = i
		}
		sort.Slice(sortedIndices, func(a, b int) bool { return phases[sortedIndices[a]] < phases[sortedIndices[b]] })
		activeIndex := sortedIndices[len(sortedIndices)-1]
		for _, i := range sortedIndices {
			if phases[i] <= phase {
				activeIndex = i
			}
		}
		expectedValue := float64(activeIndex)

		frame := playback.Evaluate(plan, pos)
		got := frame.Values[instanceID].Values[fixture.CapabilityPan]
		require.InDeltaf(t, expectedValue, got, 1e-9,
			"barIndex=%d beatFraction=%v barsPerLoop=%d phases=%v phase=%v: expected keyframe index %d",
			barIndex, beatFraction, barsPerLoop, phases, phase, activeIndex)
	})
}
