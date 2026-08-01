// compile_test.go proves Compile's all-or-nothing contract (03-07-PLAN.md
// Task 1, CONTEXT D-05/D-06): a single active scene with resolvable layer
// selections and references compiles into a fully-populated CompiledPlan;
// a State with no active scene fails with GOLC_PLAYBACK_NO_ACTIVE_SCENE; a
// dangling layer reference or an out-of-range attribute value fails the
// WHOLE compile with GOLC_PLAYBACK_PLAN_INVALID, never a partial plan; an
// invalid global BPM also fails compile; and Compile never mutates its
// input show.State.
package playback_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// testFixture is the shared show.State fixture every compile_test.go and
// evaluate_test.go case builds on: one pool ("Rig") with one member, one
// deployment with one instance patched from that member, a base-look
// preset, a color theme, a two-step chase, and a two-keyframe motion
// preset -- each referenced by the matching layer of a single 4-bar active
// scene ("Verse") at BPM 120.
type testFixture struct {
	state       show.State
	instanceID  uuid.UUID
	preset      programming.Preset
	theme       programming.Theme
	chase       programming.Chase
	motion      programming.MotionPreset
	activeScene scene.Scene
}

func newTestFixture(t *testing.T) testFixture {
	t.Helper()

	member := pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m1", FixtureContentHash: "hash1"}
	rig := pool.Pool{ID: uuid.New(), Name: "Rig", Members: []pool.PoolMember{member}}

	instance := deployment.Instance{ID: uuid.New(), PoolID: rig.ID, PoolMemberID: member.ID, Universe: 1, Address: 1}
	dep := deployment.Deployment{ID: uuid.New(), Name: "Dep", Active: true, Instances: []deployment.Instance{instance}}

	sel := programming.Selection{PoolIDs: []uuid.UUID{rig.ID}}

	preset, err := programming.NewPreset("Rest", programming.PresetIntensity)
	require.NoError(t, err, "NewPreset")
	preset.Attributes = []programming.PresetAttribute{
		{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.5},
	}

	theme, err := programming.NewTheme("Warm")
	require.NoError(t, err, "NewTheme")
	theme.Colors = []programming.ColorAssignment{{InstanceID: instance.ID, Value: 0.8}}

	chase, err := programming.NewChase("Sweep", []programming.ChaseStep{
		{Attributes: []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.1}}},
		{Attributes: []programming.PresetAttribute{{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.9}}},
	}, programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase")

	motion, err := programming.NewMotionPreset("Sway", []programming.MotionKeyframe{
		{Phase: 0.0, Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: 0.0}}},
		{Phase: 0.5, Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityPan, Value: 1.0}}},
	})
	require.NoError(t, err, "NewMotionPreset")

	verse, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	verse.Active = true
	verse, err = scene.SetLayer(verse, scene.Layer{Kind: scene.BaseLook, Enabled: true, Selection: sel, Ref: preset.ID})
	require.NoError(t, err, "SetLayer(BaseLook)")
	verse, err = scene.SetLayer(verse, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Selection: sel, Ref: theme.ID})
	require.NoError(t, err, "SetLayer(ColorTheme)")
	verse, err = scene.SetLayer(verse, scene.Layer{Kind: scene.Chase, Enabled: true, Selection: sel, Ref: chase.ID})
	require.NoError(t, err, "SetLayer(Chase)")
	verse, err = scene.SetLayer(verse, scene.Layer{Kind: scene.Motion, Enabled: true, Selection: sel, Ref: motion.ID})
	require.NoError(t, err, "SetLayer(Motion)")

	state := show.State{
		Pools:         []pool.Pool{rig},
		Deployments:   []deployment.Deployment{dep},
		Presets:       []programming.Preset{preset},
		Themes:        []programming.Theme{theme},
		Chases:        []programming.Chase{chase},
		MotionPresets: []programming.MotionPreset{motion},
		Scenes:        []scene.Scene{verse},
		Tempo:         show.Tempo{BPM: 120},
	}

	return testFixture{
		state:       state,
		instanceID:  instance.ID,
		preset:      preset,
		theme:       theme,
		chase:       chase,
		motion:      motion,
		activeScene: verse,
	}
}

func TestCompileResolvesAllFourLayers(t *testing.T) {
	fx := newTestFixture(t)

	plan, err := playback.Compile(fx.state)
	require.NoError(t, err, "Compile")
	require.Equal(t, fx.activeScene.ID, plan.SceneID)
	require.EqualValues(t, 120, plan.BPM, "expected BPM=120 BarsPerLoop=4")
	require.Equal(t, 4, plan.BarsPerLoop, "expected BPM=120 BarsPerLoop=4")
	require.Len(t, plan.Layers, 4, "expected 4 compiled layers")

	baseLook := plan.Layers[scene.BaseLook]
	require.NotNil(t, baseLook.Preset, "expected base-look layer to resolve preset %s, got %+v", fx.preset.ID, baseLook.Preset)
	require.Equal(t, fx.preset.ID, baseLook.Preset.ID, "expected base-look layer to resolve preset %s", fx.preset.ID)
	require.True(t, baseLook.Instances[fx.instanceID], "expected base-look layer's resolved Instances to include %s", fx.instanceID)

	colorTheme := plan.Layers[scene.ColorTheme]
	require.NotNil(t, colorTheme.Theme, "expected color-theme layer to resolve theme %s, got %+v", fx.theme.ID, colorTheme.Theme)
	require.Equal(t, fx.theme.ID, colorTheme.Theme.ID, "expected color-theme layer to resolve theme %s", fx.theme.ID)

	chaseLayer := plan.Layers[scene.Chase]
	require.NotNil(t, chaseLayer.Chase, "expected chase layer to resolve chase %s, got %+v", fx.chase.ID, chaseLayer.Chase)
	require.Equal(t, fx.chase.ID, chaseLayer.Chase.ID, "expected chase layer to resolve chase %s", fx.chase.ID)
	require.Len(t, chaseLayer.ChaseSteps, 2, "expected 2 compiled chase steps")

	motionLayer := plan.Layers[scene.Motion]
	require.NotNil(t, motionLayer.MotionPreset, "expected motion layer to resolve motion preset %s, got %+v", fx.motion.ID, motionLayer.MotionPreset)
	require.Equal(t, fx.motion.ID, motionLayer.MotionPreset.ID, "expected motion layer to resolve motion preset %s", fx.motion.ID)
}

func TestCompileNoActiveScene(t *testing.T) {
	fx := newTestFixture(t)
	state := fx.state
	scenes := make([]scene.Scene, len(state.Scenes))
	copy(scenes, state.Scenes)
	scenes[0].Active = false
	state.Scenes = scenes

	_, err := playback.Compile(state)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_NO_ACTIVE_SCENE")
}

func TestCompileInvalidBPM(t *testing.T) {
	fx := newTestFixture(t)
	state := fx.state
	state.Tempo = show.Tempo{BPM: 0}

	_, err := playback.Compile(state)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_PLAN_INVALID", "expected error for an invalid BPM")
}

func TestCompileAllOrNothingDanglingLayerReference(t *testing.T) {
	fx := newTestFixture(t)
	state := fx.state

	// Corrupt only the chase layer's reference; every other layer remains
	// perfectly valid -- the WHOLE compile must still fail, not a partial
	// plan missing only the chase layer.
	scenes := make([]scene.Scene, len(state.Scenes))
	copy(scenes, state.Scenes)
	corrupted, err := scene.SetLayer(scenes[0], scene.Layer{Kind: scene.Chase, Enabled: true, Ref: uuid.New()})
	require.NoError(t, err, "SetLayer")
	scenes[0] = corrupted
	state.Scenes = scenes

	_, err = playback.Compile(state)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_PLAN_INVALID", "expected error for a dangling chase reference")
}

func TestCompileAllOrNothingDanglingSelectionReference(t *testing.T) {
	fx := newTestFixture(t)
	state := fx.state

	scenes := make([]scene.Scene, len(state.Scenes))
	copy(scenes, state.Scenes)
	corrupted, err := scene.SetLayer(scenes[0], scene.Layer{
		Kind:      scene.ColorTheme,
		Enabled:   true,
		Selection: programming.Selection{PoolIDs: []uuid.UUID{uuid.New()}},
		Ref:       fx.theme.ID,
	})
	require.NoError(t, err, "SetLayer")
	scenes[0] = corrupted
	state.Scenes = scenes

	_, err = playback.Compile(state)
	require.ErrorContains(t, err, "GOLC_PLAYBACK_PLAN_INVALID", "expected error for a dangling selection reference")
}

func TestCompileNeverMutatesState(t *testing.T) {
	fx := newTestFixture(t)
	before := fx.state

	_, err := playback.Compile(fx.state)
	require.NoError(t, err, "Compile")

	require.Equal(t, before, fx.state, "Compile mutated its input State")
}
