// scene_test.go proves the "scene"/"blend" command scopes' route contract
// (03-04-PLAN.md Task 3): "scene create" appends a bar-loop Scene and
// saves, rejecting a duplicate name through the existing
// GOLC_SHOW_STATE_INVALID wrapping diagnostic; "scene activate" marks
// exactly one scene active, clearing every other scene, and a second
// activate against a different scene keeps exactly one active; "scene
// layer set" enables/points one of a scene's four fixed layers, and a Ref
// to a non-existent programming object is rejected with
// GOLC_SCENE_LAYER_DANGLING_REFERENCE (wrapped in GOLC_SHOW_STATE_INVALID)
// at Load/Save time; "blend create" appends a reusable BlendPreset;
// show.Load/Save round-trips Scenes/BlendPresets/Tempo without loss. It
// also proves WR-03: a "scene layer set" invocation that omits every
// --pool/--group/--instance/--fixture flag preserves the existing layer's
// Selection rather than silently wiping it, while an invocation that
// explicitly re-supplies a selector kind still replaces it. Mirrors
// theme_preset_test.go/chase_motion_test.go's seed-a-ShowState-directly-
// then-exercise-CLI-routes convention.
package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

func assertExactlyOneSceneActiveNamed(t *testing.T, scenes []scene.Scene, expectedName string) {
	t.Helper()
	activeCount := 0
	for _, s := range scenes {
		if s.Active {
			activeCount++
			require.Equal(t, expectedName, s.Name, "expected %q to be the only active scene, got %q active", expectedName, s.Name)
		}
	}
	require.Equal(t, 1, activeCount, "expected exactly one active scene, got %d", activeCount)
}

func findSceneByName(scenes []scene.Scene, name string) (scene.Scene, bool) {
	for _, s := range scenes {
		if s.Name == name {
			return s, true
		}
	}
	return scene.Scene{}, false
}

func TestSceneRoutesCreateActivateLayerSet(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Verse", "--bars", "4", "--show", showPath,
	}})
	require.Equal(t, 0, createResult.ExitCode, "scene create failed: exit=%d stderr=%s", createResult.ExitCode, createResult.Stderr)

	afterCreate, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after scene create: %v", err)
	require.True(t, len(afterCreate.Scenes) == 1 && afterCreate.Scenes[0].Name == "Verse" && afterCreate.Scenes[0].BarsPerLoop == 4, "expected exactly one persisted 4-bar scene named Verse, got %+v", afterCreate.Scenes)

	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Verse", "--bars", "8", "--show", showPath,
	}})
	require.True(t, duplicateResult.ExitCode != 0 && strings.Contains(string(duplicateResult.Stderr), "GOLC_SCENE_DUPLICATE_NAME"), "expected GOLC_SCENE_DUPLICATE_NAME for a duplicate scene name, got exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)
	require.Contains(t, string(duplicateResult.Stderr), "GOLC_SHOW_STATE_INVALID", "expected the duplicate-name diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", duplicateResult.Stderr)

	secondCreate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Chorus", "--bars", "8", "--show", showPath,
	}})
	require.Equal(t, 0, secondCreate.ExitCode, "scene create (Chorus) failed: exit=%d stderr=%s", secondCreate.ExitCode, secondCreate.Stderr)

	activateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "activate", "Verse", "--show", showPath,
	}})
	require.Equal(t, 0, activateResult.ExitCode, "scene activate failed: exit=%d stderr=%s", activateResult.ExitCode, activateResult.Stderr)

	afterActivate, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after scene activate: %v", err)
	assertExactlyOneSceneActiveNamed(t, afterActivate.Scenes, "Verse")

	// A second activate against a different scene keeps exactly one
	// active -- never transiently two (SCEN-04).
	secondActivate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "activate", "Chorus", "--show", showPath,
	}})
	require.Equal(t, 0, secondActivate.ExitCode, "scene activate (Chorus) failed: exit=%d stderr=%s", secondActivate.ExitCode, secondActivate.Stderr)
	afterSecondActivate, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after second scene activate: %v", err)
	assertExactlyOneSceneActiveNamed(t, afterSecondActivate.Scenes, "Chorus")

	// Seed a real chase directly (chase authoring routes are 03-03's
	// concern, not this plan's) so "scene layer set" has a resolvable
	// reference to point at.
	chase, err := programming.NewChase("Sweep", nil, programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase: %v", err)
	withChase := afterSecondActivate
	withChase.Chases = append(withChase.Chases, chase)
	require.NoError(t, show.Save(root, showPath, withChase), "show.Save (seed chase): %v", err)

	layerSetResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", chase.ID.String(),
		"--show", showPath,
	}})
	require.Equal(t, 0, layerSetResult.ExitCode, "scene layer set failed: exit=%d stderr=%s", layerSetResult.ExitCode, layerSetResult.Stderr)

	afterLayerSet, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after scene layer set: %v", err)
	chorusScene, found := findSceneByName(afterLayerSet.Scenes, "Chorus")
	require.True(t, found, "expected Chorus scene to still exist, got %+v", afterLayerSet.Scenes)
	chaseLayer, ok := chorusScene.LayerByKind(scene.Chase)
	require.True(t, ok && chaseLayer.Enabled && chaseLayer.Ref == chase.ID, "expected the chase layer to be enabled and pointed at %s, got %+v", chase.ID, chaseLayer)

	// A Ref to a non-existent chase is rejected at Load/Save time.
	danglingResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", uuid.Must(uuid.NewV7()).String(),
		"--show", showPath,
	}})
	require.True(t, danglingResult.ExitCode != 0 && strings.Contains(string(danglingResult.Stderr), "GOLC_SCENE_LAYER_DANGLING_REFERENCE"), "expected GOLC_SCENE_LAYER_DANGLING_REFERENCE for a dangling chase reference, got exit=%d stderr=%s", danglingResult.ExitCode, danglingResult.Stderr)
	require.Contains(t, string(danglingResult.Stderr), "GOLC_SHOW_STATE_INVALID", "expected the dangling-reference diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", danglingResult.Stderr)
}

// TestSceneLayerSetPreservesSelectionWhenOmitted proves WR-03: a second
// "scene layer set" invocation against the same layer that repoints --ref
// (or toggles --disable) without re-supplying any --pool/--group/
// --instance/--fixture flags must NOT silently discard the Selection
// configured by a prior invocation.
func TestSceneLayerSetPreservesSelectionWhenOmitted(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Chorus", "--bars", "4", "--show", showPath,
	}})
	require.Equal(t, 0, createResult.ExitCode, "scene create failed: exit=%d stderr=%s", createResult.ExitCode, createResult.Stderr)

	chaseA, err := programming.NewChase("SweepA", nil, programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase (A): %v", err)
	chaseB, err := programming.NewChase("SweepB", nil, programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase (B): %v", err)
	seeded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (seed chases): %v", err)
	seeded.Chases = append(seeded.Chases, chaseA, chaseB)
	require.NoError(t, show.Save(root, showPath, seeded), "show.Save (seed chases): %v", err)

	// A real pool is required here, not just a freshly-minted UUID: since
	// internal/show/store.go's Save now scrubs any Layer.Selection
	// selector referencing a pool that doesn't actually exist (the "adopt
	// a never-before-used pool" safety net, see programming.ScrubDangling/
	// scene.ScrubDanglingSelections), a selector pointed at a pool that
	// was never created would be cleaned up as dangling rather than
	// preserved -- which would defeat the very thing this test verifies.
	poolCreateResult := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	require.Equal(t, 0, poolCreateResult.ExitCode, "pool create failed: exit=%d stderr=%s", poolCreateResult.ExitCode, poolCreateResult.Stderr)
	seededPools, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (read pool id): %v", err)
	poolID := seededPools.Pools[0].ID

	firstSet := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", chaseA.ID.String(),
		"--pool", poolID.String(),
		"--show", showPath,
	}})
	require.Equal(t, 0, firstSet.ExitCode, "first scene layer set failed: exit=%d stderr=%s", firstSet.ExitCode, firstSet.Stderr)

	// Repoint --ref to chaseB WITHOUT re-supplying --pool: the previously
	// configured pool selector must be preserved, not wiped to empty.
	secondSet := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", chaseB.ID.String(),
		"--show", showPath,
	}})
	require.Equal(t, 0, secondSet.ExitCode, "second scene layer set failed: exit=%d stderr=%s", secondSet.ExitCode, secondSet.Stderr)

	after, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after second scene layer set: %v", err)
	chorusScene, found := findSceneByName(after.Scenes, "Chorus")
	require.True(t, found, "expected Chorus scene to still exist, got %+v", after.Scenes)
	chaseLayer, ok := chorusScene.LayerByKind(scene.Chase)
	require.True(t, ok, "expected a chase layer slot")
	require.Equal(t, chaseB.ID, chaseLayer.Ref, "expected the chase layer's Ref to be repointed to %s, got %s", chaseB.ID, chaseLayer.Ref)
	require.True(t, len(chaseLayer.Selection.PoolIDs) == 1 && chaseLayer.Selection.PoolIDs[0] == poolID, "expected the previously configured pool selector %s to be preserved, got %+v", poolID, chaseLayer.Selection.PoolIDs)

	// Explicitly re-supplying --pool with a different value still replaces
	// the pool selector as before -- the merge only applies when a
	// selector kind is omitted entirely. Another real pool, for the same
	// reason as poolID above.
	otherPoolCreateResult := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Other Pool", "--show", showPath}})
	require.Equal(t, 0, otherPoolCreateResult.ExitCode, "pool create (other) failed: exit=%d stderr=%s", otherPoolCreateResult.ExitCode, otherPoolCreateResult.Stderr)
	seededOtherPool, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (read other pool id): %v", err)
	var otherPoolID uuid.UUID
	for _, p := range seededOtherPool.Pools {
		if p.Name == "Other Pool" {
			otherPoolID = p.ID
		}
	}
	thirdSet := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", chaseB.ID.String(),
		"--pool", otherPoolID.String(),
		"--show", showPath,
	}})
	require.Equal(t, 0, thirdSet.ExitCode, "third scene layer set failed: exit=%d stderr=%s", thirdSet.ExitCode, thirdSet.Stderr)
	afterThird, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after third scene layer set: %v", err)
	chorusAfterThird, _ := findSceneByName(afterThird.Scenes, "Chorus")
	chaseLayerAfterThird, _ := chorusAfterThird.LayerByKind(scene.Chase)
	require.True(t, len(chaseLayerAfterThird.Selection.PoolIDs) == 1 && chaseLayerAfterThird.Selection.PoolIDs[0] == otherPoolID, "expected an explicitly re-supplied --pool to replace the selector, got %+v", chaseLayerAfterThird.Selection.PoolIDs)
}

func TestSceneRoutesCreateMissingBarsUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "No Bars", "--show", showPath,
	}})
	require.True(t, result.ExitCode == 2 && strings.Contains(string(result.Stderr), "GOLC_SCENE_USAGE"), "expected exit 2 GOLC_SCENE_USAGE for a missing --bars, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestSceneRoutesBlendCreate(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	blendResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "2", "--show", showPath,
	}})
	require.Equal(t, 0, blendResult.ExitCode, "blend create failed: exit=%d stderr=%s", blendResult.ExitCode, blendResult.Stderr)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load after blend create: %v", err)
	require.True(t, len(reloaded.BlendPresets) == 1 && reloaded.BlendPresets[0].Name == "Fade" && reloaded.BlendPresets[0].DurationBars == 2, "expected exactly one persisted blend preset named Fade, got %+v", reloaded.BlendPresets)

	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "1", "--show", showPath,
	}})
	require.True(t, duplicateResult.ExitCode != 0 && strings.Contains(string(duplicateResult.Stderr), "GOLC_BLEND_PRESET_DUPLICATE_NAME"), "expected GOLC_BLEND_PRESET_DUPLICATE_NAME for a duplicate blend preset name, got exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)
}

func TestBlendRenameRoute(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	fadeCreateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "2", "--show", showPath,
	}})
	require.Equal(t, 0, fadeCreateResult.ExitCode, "blend create failed: exit=%d stderr=%s", fadeCreateResult.ExitCode, fadeCreateResult.Stderr)

	rename := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "rename", "Fade", "Fade Renamed", "--show", showPath,
	}})
	require.Equal(t, 0, rename.ExitCode, "blend rename failed: exit=%d stderr=%s", rename.ExitCode, rename.Stderr)
	require.Contains(t, string(rename.Stdout), "GOLC_BLEND_PRESET_RENAMED", "expected GOLC_BLEND_PRESET_RENAMED in stdout, got %s", rename.Stdout)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load: %v", err)
	require.True(t, len(reloaded.BlendPresets) == 1 && reloaded.BlendPresets[0].Name == "Fade Renamed", "expected exactly one blend preset named %q, got %+v", "Fade Renamed", reloaded.BlendPresets)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "rename", "Nonexistent", "New Name", "--show", showPath,
	}})
	require.True(t, unknown.ExitCode != 0 && strings.Contains(string(unknown.Stderr), "GOLC_BLEND_PRESET_NOT_FOUND"), "expected GOLC_BLEND_PRESET_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)

	secondBlendCreateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Second Blend", "--duration-bars", "1", "--show", showPath,
	}})
	require.Equal(t, 0, secondBlendCreateResult.ExitCode, "blend create (second) failed: exit=%d stderr=%s", secondBlendCreateResult.ExitCode, secondBlendCreateResult.Stderr)
	collide := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "rename", "Second Blend", "Fade Renamed", "--show", showPath,
	}})
	require.True(t, collide.ExitCode != 0 && strings.Contains(string(collide.Stderr), "GOLC_BLEND_PRESET_DUPLICATE_NAME"), "expected GOLC_BLEND_PRESET_DUPLICATE_NAME for a colliding rename, got exit=%d stderr=%s", collide.ExitCode, collide.Stderr)
}

func TestBlendDeleteRoute(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "2", "--show", showPath,
	}})
	require.Equal(t, 0, result.ExitCode, "blend create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)

	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "delete", "Fade", "--show", showPath,
	}})
	require.Equal(t, 0, deleteResult.ExitCode, "blend delete failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	require.Contains(t, string(deleteResult.Stdout), "GOLC_BLEND_PRESET_DELETED", "expected GOLC_BLEND_PRESET_DELETED in stdout, got %s", deleteResult.Stdout)

	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load: %v", err)
	require.Empty(t, reloaded.BlendPresets, "expected zero blend presets after delete, got %+v", reloaded.BlendPresets)

	unknown := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "delete", "Nonexistent", "--show", showPath,
	}})
	require.True(t, unknown.ExitCode != 0 && strings.Contains(string(unknown.Stderr), "GOLC_BLEND_PRESET_NOT_FOUND"), "expected GOLC_BLEND_PRESET_NOT_FOUND, got exit=%d stderr=%s", unknown.ExitCode, unknown.Stderr)
}

func TestSceneRoutesShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	newScene, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene: %v", err)
	blend, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveLinear)
	require.NoError(t, err, "NewBlendPreset: %v", err)

	state := show.State{
		Scenes:       []scene.Scene{newScene},
		BlendPresets: []scene.BlendPreset{blend},
		Tempo:        show.Tempo{BPM: 120},
	}
	require.NoError(t, show.Save(root, path, state), "show.Save: %v", err)

	reloaded, err := show.Load(root, path)
	require.NoError(t, err, "show.Load: %v", err)
	require.True(t, len(reloaded.Scenes) == 1 && reloaded.Scenes[0].ID == newScene.ID && reloaded.Scenes[0].Name == newScene.Name, "scene did not round-trip: %+v", reloaded.Scenes)
	require.True(t, len(reloaded.BlendPresets) == 1 && reloaded.BlendPresets[0].ID == blend.ID, "blend preset did not round-trip: %+v", reloaded.BlendPresets)
	require.EqualValues(t, 120, reloaded.Tempo.BPM, "tempo did not round-trip: %+v", reloaded.Tempo)
}
