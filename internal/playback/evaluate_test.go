// evaluate_test.go proves Evaluate's SCEN-09 determinism contract
// (03-07-PLAN.md Task 1): calling Evaluate twice -- or concurrently from
// many goroutines -- with the same (plan, position) always returns a
// byte-identical Frame; the fixed base-look < color-theme < chase < motion
// layer-priority reduce overrides per-attribute (never HTP/highest-value-
// wins); a disabled layer contributes nothing; and chase step selection is
// a pure function of position.
package playback_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

func TestDeterministicEvaluateSameArgs(t *testing.T) {
	fx := newTestFixture(t)
	plan, err := playback.Compile(fx.state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	pos := playback.MusicalPosition{BarIndex: 1, BeatFraction: 0.25}

	first := playback.Evaluate(plan, pos)
	second := playback.Evaluate(plan, pos)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Evaluate called twice with identical args returned different Frames:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestDeterministicEvaluateAcrossGoroutines(t *testing.T) {
	fx := newTestFixture(t)
	plan, err := playback.Compile(fx.state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	pos := playback.MusicalPosition{BarIndex: 2, BeatFraction: 0.75}

	want := playback.Evaluate(plan, pos)

	const goroutines = 100
	results := make([]playback.Frame, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			results[i] = playback.Evaluate(plan, pos)
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("goroutine %d: Evaluate = %+v, want byte-identical %+v", i, got, want)
		}
	}
}

func TestEvaluateFixedPriorityOverridesPerAttribute(t *testing.T) {
	fx := newTestFixture(t)
	plan, err := playback.Compile(fx.state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Position within step 0 of the chase (StepDuration=1 bar): the
	// base-look preset's intensity=0.5 must be overridden by the chase
	// step's own intensity=0.1 (chase > base-look, CONTEXT D-02), while
	// color-theme's color=0.8 (untouched by chase/motion) must survive
	// unchanged.
	pos := playback.MusicalPosition{BarIndex: 0, BeatFraction: 0.1}
	frame := playback.Evaluate(plan, pos)

	attrs, ok := frame.Values[fx.instanceID]
	if !ok {
		t.Fatalf("expected a Frame entry for instance %s, got %+v", fx.instanceID, frame.Values)
	}
	if diff := attrs.Values[fixture.CapabilityIntensity] - 0.1; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("expected chase step 0's intensity=0.1 to override base-look's 0.5, got %v", attrs.Values[fixture.CapabilityIntensity])
	}
	if diff := attrs.Values[fixture.CapabilityColor] - 0.8; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("expected color-theme's color=0.8 to survive untouched, got %v", attrs.Values[fixture.CapabilityColor])
	}
}

func TestEvaluateDisabledLayerContributesNothing(t *testing.T) {
	fx := newTestFixture(t)
	state := fx.state

	scenes := make([]scene.Scene, len(state.Scenes))
	copy(scenes, state.Scenes)
	disabledChase, err := scene.SetLayer(scenes[0], scene.Layer{Kind: scene.Chase, Enabled: false, Ref: fx.chase.ID})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}
	scenes[0] = disabledChase
	state.Scenes = scenes

	plan, err := playback.Compile(state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	pos := playback.MusicalPosition{BarIndex: 0, BeatFraction: 0.1}
	frame := playback.Evaluate(plan, pos)

	attrs := frame.Values[fx.instanceID]
	// With the chase layer disabled, intensity must fall back to
	// base-look's own 0.5 -- the chase step's 0.1 must never appear.
	if diff := attrs.Values[fixture.CapabilityIntensity] - 0.5; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("expected base-look's intensity=0.5 with chase disabled, got %v", attrs.Values[fixture.CapabilityIntensity])
	}
}

func TestEvaluateChaseStepAdvancesWithPosition(t *testing.T) {
	fx := newTestFixture(t)
	plan, err := playback.Compile(fx.state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// StepDuration=1 bar, 2 steps: bar 0 is step 0 (intensity=0.1), bar 1
	// is step 1 (intensity=0.9), bar 2 wraps back to step 0 (intensity=0.1).
	cases := []struct {
		bar           int
		wantIntensity float64
	}{
		{0, 0.1},
		{1, 0.9},
		{2, 0.1},
		{3, 0.9},
	}
	for _, tc := range cases {
		pos := playback.MusicalPosition{BarIndex: tc.bar, BeatFraction: 0.0}
		frame := playback.Evaluate(plan, pos)
		got := frame.Values[fx.instanceID].Values[fixture.CapabilityIntensity]
		if diff := got - tc.wantIntensity; diff < -1e-9 || diff > 1e-9 {
			t.Errorf("bar %d: intensity = %v, want %v", tc.bar, got, tc.wantIntensity)
		}
	}
}

func TestEvaluateMotionKeyframeSelection(t *testing.T) {
	fx := newTestFixture(t)
	plan, err := playback.Compile(fx.state)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// barsPerLoop=4; keyframes at phase 0.0 (pan=0.0) and phase 0.5
	// (pan=1.0). bar 0 -> phase 0.0 -> keyframe 0; bar 2 -> phase 0.5 ->
	// keyframe 1 (step-function: the active keyframe is the last one whose
	// Phase <= current phase).
	pos0 := playback.MusicalPosition{BarIndex: 0, BeatFraction: 0.0}
	frame0 := playback.Evaluate(plan, pos0)
	if diff := frame0.Values[fx.instanceID].Values[fixture.CapabilityPan] - 0.0; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("bar 0: expected pan=0.0, got %v", frame0.Values[fx.instanceID].Values[fixture.CapabilityPan])
	}

	pos2 := playback.MusicalPosition{BarIndex: 2, BeatFraction: 0.0}
	frame2 := playback.Evaluate(plan, pos2)
	if diff := frame2.Values[fx.instanceID].Values[fixture.CapabilityPan] - 1.0; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("bar 2: expected pan=1.0, got %v", frame2.Values[fx.instanceID].Values[fixture.CapabilityPan])
	}
}
