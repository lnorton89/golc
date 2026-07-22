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
// show.Load/Save round-trips Scenes/BlendPresets/Tempo without loss.
// Mirrors theme_preset_test.go/chase_motion_test.go's seed-a-ShowState-
// directly-then-exercise-CLI-routes convention.
package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

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
			if s.Name != expectedName {
				t.Fatalf("expected %q to be the only active scene, got %q active", expectedName, s.Name)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active scene, got %d", activeCount)
	}
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
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Verse", "--bars", "4", "--show", showPath,
	}})
	if createResult.ExitCode != 0 {
		t.Fatalf("scene create failed: exit=%d stderr=%s", createResult.ExitCode, createResult.Stderr)
	}

	afterCreate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after scene create: %v", err)
	}
	if len(afterCreate.Scenes) != 1 || afterCreate.Scenes[0].Name != "Verse" || afterCreate.Scenes[0].BarsPerLoop != 4 {
		t.Fatalf("expected exactly one persisted 4-bar scene named Verse, got %+v", afterCreate.Scenes)
	}

	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Verse", "--bars", "8", "--show", showPath,
	}})
	if duplicateResult.ExitCode == 0 || !strings.Contains(string(duplicateResult.Stderr), "GOLC_SCENE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_SCENE_DUPLICATE_NAME for a duplicate scene name, got exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)
	}
	if !strings.Contains(string(duplicateResult.Stderr), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected the duplicate-name diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", duplicateResult.Stderr)
	}

	secondCreate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Chorus", "--bars", "8", "--show", showPath,
	}})
	if secondCreate.ExitCode != 0 {
		t.Fatalf("scene create (Chorus) failed: exit=%d stderr=%s", secondCreate.ExitCode, secondCreate.Stderr)
	}

	activateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "activate", "Verse", "--show", showPath,
	}})
	if activateResult.ExitCode != 0 {
		t.Fatalf("scene activate failed: exit=%d stderr=%s", activateResult.ExitCode, activateResult.Stderr)
	}

	afterActivate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after scene activate: %v", err)
	}
	assertExactlyOneSceneActiveNamed(t, afterActivate.Scenes, "Verse")

	// A second activate against a different scene keeps exactly one
	// active -- never transiently two (SCEN-04).
	secondActivate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "activate", "Chorus", "--show", showPath,
	}})
	if secondActivate.ExitCode != 0 {
		t.Fatalf("scene activate (Chorus) failed: exit=%d stderr=%s", secondActivate.ExitCode, secondActivate.Stderr)
	}
	afterSecondActivate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after second scene activate: %v", err)
	}
	assertExactlyOneSceneActiveNamed(t, afterSecondActivate.Scenes, "Chorus")

	// Seed a real chase directly (chase authoring routes are 03-03's
	// concern, not this plan's) so "scene layer set" has a resolvable
	// reference to point at.
	chase, err := programming.NewChase("Sweep", nil, programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	withChase := afterSecondActivate
	withChase.Chases = append(withChase.Chases, chase)
	if err := show.Save(root, showPath, withChase); err != nil {
		t.Fatalf("show.Save (seed chase): %v", err)
	}

	layerSetResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", chase.ID.String(),
		"--show", showPath,
	}})
	if layerSetResult.ExitCode != 0 {
		t.Fatalf("scene layer set failed: exit=%d stderr=%s", layerSetResult.ExitCode, layerSetResult.Stderr)
	}

	afterLayerSet, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after scene layer set: %v", err)
	}
	chorusScene, found := findSceneByName(afterLayerSet.Scenes, "Chorus")
	if !found {
		t.Fatalf("expected Chorus scene to still exist, got %+v", afterLayerSet.Scenes)
	}
	chaseLayer, ok := chorusScene.LayerByKind(scene.Chase)
	if !ok || !chaseLayer.Enabled || chaseLayer.Ref != chase.ID {
		t.Fatalf("expected the chase layer to be enabled and pointed at %s, got %+v", chase.ID, chaseLayer)
	}

	// A Ref to a non-existent chase is rejected at Load/Save time.
	danglingResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "layer", "set", "Chorus",
		"--kind", "chase",
		"--ref", uuid.Must(uuid.NewV7()).String(),
		"--show", showPath,
	}})
	if danglingResult.ExitCode == 0 || !strings.Contains(string(danglingResult.Stderr), "GOLC_SCENE_LAYER_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SCENE_LAYER_DANGLING_REFERENCE for a dangling chase reference, got exit=%d stderr=%s", danglingResult.ExitCode, danglingResult.Stderr)
	}
	if !strings.Contains(string(danglingResult.Stderr), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected the dangling-reference diagnostic to be wrapped in GOLC_SHOW_STATE_INVALID, got stderr=%s", danglingResult.Stderr)
	}
}

func TestSceneRoutesCreateMissingBarsUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "No Bars", "--show", showPath,
	}})
	if result.ExitCode != 2 || !strings.Contains(string(result.Stderr), "GOLC_SCENE_USAGE") {
		t.Fatalf("expected exit 2 GOLC_SCENE_USAGE for a missing --bars, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestSceneRoutesBlendCreate(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	blendResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "2", "--show", showPath,
	}})
	if blendResult.ExitCode != 0 {
		t.Fatalf("blend create failed: exit=%d stderr=%s", blendResult.ExitCode, blendResult.Stderr)
	}

	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after blend create: %v", err)
	}
	if len(reloaded.BlendPresets) != 1 || reloaded.BlendPresets[0].Name != "Fade" || reloaded.BlendPresets[0].DurationBars != 2 {
		t.Fatalf("expected exactly one persisted blend preset named Fade, got %+v", reloaded.BlendPresets)
	}

	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"blend", "create", "Fade", "--duration-bars", "1", "--show", showPath,
	}})
	if duplicateResult.ExitCode == 0 || !strings.Contains(string(duplicateResult.Stderr), "GOLC_BLEND_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_BLEND_PRESET_DUPLICATE_NAME for a duplicate blend preset name, got exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)
	}
}

func TestSceneRoutesShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	newScene, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	blend, err := scene.NewBlendPreset("Fade", 2, scene.BlendCurveLinear)
	if err != nil {
		t.Fatalf("NewBlendPreset: %v", err)
	}

	state := show.State{
		Scenes:       []scene.Scene{newScene},
		BlendPresets: []scene.BlendPreset{blend},
		Tempo:        show.Tempo{BPM: 120},
	}
	if err := show.Save(root, path, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	reloaded, err := show.Load(root, path)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(reloaded.Scenes) != 1 || reloaded.Scenes[0].ID != newScene.ID || reloaded.Scenes[0].Name != newScene.Name {
		t.Fatalf("scene did not round-trip: %+v", reloaded.Scenes)
	}
	if len(reloaded.BlendPresets) != 1 || reloaded.BlendPresets[0].ID != blend.ID {
		t.Fatalf("blend preset did not round-trip: %+v", reloaded.BlendPresets)
	}
	if reloaded.Tempo.BPM != 120 {
		t.Fatalf("tempo did not round-trip: %+v", reloaded.Tempo)
	}
}
