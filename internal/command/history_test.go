// history_test.go proves PROG-07's completed record/update/rename/reorder/
// duplicate/delete CLI route surface (03-05-PLAN.md Task 2): "theme
// rename"/"theme delete", "preset rename"/"preset delete", "chase update"/
// "chase reorder"/"chase duplicate"/"chase delete", "motion rename"/
// "motion duplicate"/"motion delete", and "scene rename"/"scene duplicate"/
// "scene delete" -- each following the existing parse->Load->mutate->Save->
// Stdout shape, preserving identity on rename/update/reorder, minting a
// fresh identity on duplicate, and persisting through show.Load/show.Save.
// Also proves CONTEXT D-08 (TestHistoryLiveActiveEdit): a rename/reorder/
// duplicate/delete succeeds and persists with no pause/detach/lock
// precondition even while a scene referencing the edited object is active
// -- no CRUD handler here ever reads scene.Scene.Active before mutating.
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

func findThemeByID(themes []programming.Theme, id uuid.UUID) (programming.Theme, bool) {
	for _, th := range themes {
		if th.ID == id {
			return th, true
		}
	}
	return programming.Theme{}, false
}

func findPresetByID(presets []programming.Preset, id uuid.UUID) (programming.Preset, bool) {
	for _, p := range presets {
		if p.ID == id {
			return p, true
		}
	}
	return programming.Preset{}, false
}

func findChaseByID(chases []programming.Chase, id uuid.UUID) (programming.Chase, bool) {
	for _, c := range chases {
		if c.ID == id {
			return c, true
		}
	}
	return programming.Chase{}, false
}

func findMotionByID(motionPresets []programming.MotionPreset, id uuid.UUID) (programming.MotionPreset, bool) {
	for _, m := range motionPresets {
		if m.ID == id {
			return m, true
		}
	}
	return programming.MotionPreset{}, false
}

// findSceneByName is defined in scene_test.go (same package) and reused
// here.

// seedHistoryShowState builds and saves a ShowState carrying two of each
// renameable object type (so a rename-to-an-existing-name collision has a
// real target to hit) plus two inactive scenes, and returns the reloaded
// state for the caller to read starting IDs from.
func seedHistoryShowState(t *testing.T, root, showPath string) show.State {
	t.Helper()

	sunset, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme(Sunset): %v", err)
	}
	ocean, err := programming.NewTheme("Ocean")
	if err != nil {
		t.Fatalf("NewTheme(Ocean): %v", err)
	}

	fullWash, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(Full Wash): %v", err)
	}
	house, err := programming.NewPreset("House", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(House): %v", err)
	}

	// Steps are tagged with a distinguishing Attributes[0].Value equal to
	// their original index (0, 1, 2) so a later "chase reorder" can be
	// verified by reading which tagged step now occupies each position.
	steps := []programming.ChaseStep{
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 0}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 1}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 2}}},
	}
	sweep, err := programming.NewChase("Sweep", steps, programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase(Sweep): %v", err)
	}

	arc, err := programming.NewMotionPreset("Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset(Arc): %v", err)
	}
	fade, err := programming.NewMotionPreset("Fade", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset(Fade): %v", err)
	}

	primary, err := scene.NewScene("Primary", 4)
	if err != nil {
		t.Fatalf("NewScene(Primary): %v", err)
	}
	secondary, err := scene.NewScene("Secondary", 8)
	if err != nil {
		t.Fatalf("NewScene(Secondary): %v", err)
	}

	state := show.State{
		Themes:        []programming.Theme{sunset, ocean},
		Presets:       []programming.Preset{fullWash, house},
		Chases:        []programming.Chase{sweep},
		MotionPresets: []programming.MotionPreset{arc, fade},
		Scenes:        []scene.Scene{primary, secondary},
	}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save (seed): %v", err)
	}
	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (seed reload): %v", err)
	}
	return reloaded
}

func TestHistoryRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seeded := seedHistoryShowState(t, root, showPath)
	themeSunsetID := seeded.Themes[0].ID
	presetFullWashID := seeded.Presets[0].ID
	chaseSweepID := seeded.Chases[0].ID
	motionArcID := seeded.MotionPresets[0].ID
	scenePrimaryID := seeded.Scenes[0].ID

	// --- theme rename: duplicate-name rejection, success (ID-stable), not-found ---
	dupTheme := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "Sunset", "Ocean", "--show", showPath,
	}})
	if dupTheme.ExitCode == 0 || !strings.Contains(string(dupTheme.Stderr), "GOLC_THEME_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_THEME_DUPLICATE_NAME renaming Sunset->Ocean (existing name), got exit=%d stderr=%s", dupTheme.ExitCode, dupTheme.Stderr)
	}

	themeRename := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "Sunset", "Sunrise", "--show", showPath,
	}})
	if themeRename.ExitCode != 0 {
		t.Fatalf("theme rename failed: exit=%d stderr=%s", themeRename.ExitCode, themeRename.Stderr)
	}
	afterThemeRename, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after theme rename: %v", err)
	}
	renamedTheme, found := findThemeByID(afterThemeRename.Themes, themeSunsetID)
	if !found || renamedTheme.Name != "Sunrise" {
		t.Fatalf("expected theme %s renamed to Sunrise with ID preserved, got %+v", themeSunsetID, afterThemeRename.Themes)
	}

	themeRenameNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "NoSuchTheme", "Whatever", "--show", showPath,
	}})
	if themeRenameNotFound.ExitCode == 0 || !strings.Contains(string(themeRenameNotFound.Stderr), "GOLC_THEME_NOT_FOUND") {
		t.Fatalf("expected GOLC_THEME_NOT_FOUND, got exit=%d stderr=%s", themeRenameNotFound.ExitCode, themeRenameNotFound.Stderr)
	}

	// --- theme delete: not-found, success ---
	themeDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "delete", "NoSuchTheme", "--show", showPath,
	}})
	if themeDeleteNotFound.ExitCode == 0 || !strings.Contains(string(themeDeleteNotFound.Stderr), "GOLC_THEME_NOT_FOUND") {
		t.Fatalf("expected GOLC_THEME_NOT_FOUND, got exit=%d stderr=%s", themeDeleteNotFound.ExitCode, themeDeleteNotFound.Stderr)
	}
	themeDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "delete", "Sunrise", "--show", showPath,
	}})
	if themeDelete.ExitCode != 0 {
		t.Fatalf("theme delete failed: exit=%d stderr=%s", themeDelete.ExitCode, themeDelete.Stderr)
	}
	afterThemeDelete, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after theme delete: %v", err)
	}
	if len(afterThemeDelete.Themes) != 1 || afterThemeDelete.Themes[0].Name != "Ocean" {
		t.Fatalf("expected exactly Ocean to remain after deleting Sunrise, got %+v", afterThemeDelete.Themes)
	}

	// --- preset rename: duplicate-name rejection, success (ID-stable) ---
	presetDup := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "rename", "Full Wash", "House", "--show", showPath,
	}})
	if presetDup.ExitCode == 0 || !strings.Contains(string(presetDup.Stderr), "GOLC_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_PRESET_DUPLICATE_NAME, got exit=%d stderr=%s", presetDup.ExitCode, presetDup.Stderr)
	}
	presetRename := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "rename", "Full Wash", "Warm Wash", "--show", showPath,
	}})
	if presetRename.ExitCode != 0 {
		t.Fatalf("preset rename failed: exit=%d stderr=%s", presetRename.ExitCode, presetRename.Stderr)
	}
	afterPresetRename, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after preset rename: %v", err)
	}
	renamedPreset, found := findPresetByID(afterPresetRename.Presets, presetFullWashID)
	if !found || renamedPreset.Name != "Warm Wash" {
		t.Fatalf("expected preset %s renamed to Warm Wash with ID preserved, got %+v", presetFullWashID, afterPresetRename.Presets)
	}

	// --- preset delete: not-found, success ---
	presetDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "NoSuchPreset", "--show", showPath,
	}})
	if presetDeleteNotFound.ExitCode == 0 || !strings.Contains(string(presetDeleteNotFound.Stderr), "GOLC_PRESET_NOT_FOUND") {
		t.Fatalf("expected GOLC_PRESET_NOT_FOUND, got exit=%d stderr=%s", presetDeleteNotFound.ExitCode, presetDeleteNotFound.Stderr)
	}
	presetDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "Warm Wash", "--show", showPath,
	}})
	if presetDelete.ExitCode != 0 {
		t.Fatalf("preset delete failed: exit=%d stderr=%s", presetDelete.ExitCode, presetDelete.Stderr)
	}

	// --- chase update: usage rejection (no fields), success (rename+step-duration, ID-stable) ---
	chaseUpdateMissingFields := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "update", "Sweep", "--show", showPath,
	}})
	if chaseUpdateMissingFields.ExitCode != 2 || !strings.Contains(string(chaseUpdateMissingFields.Stderr), "GOLC_CHASE_USAGE") {
		t.Fatalf("expected exit 2 GOLC_CHASE_USAGE for chase update with no fields, got exit=%d stderr=%s", chaseUpdateMissingFields.ExitCode, chaseUpdateMissingFields.Stderr)
	}
	chaseUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "update", "Sweep", "--name", "Sweep2", "--step-duration", "2", "--show", showPath,
	}})
	if chaseUpdate.ExitCode != 0 {
		t.Fatalf("chase update failed: exit=%d stderr=%s", chaseUpdate.ExitCode, chaseUpdate.Stderr)
	}
	afterChaseUpdate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after chase update: %v", err)
	}
	updatedChase, found := findChaseByID(afterChaseUpdate.Chases, chaseSweepID)
	if !found || updatedChase.Name != "Sweep2" || updatedChase.StepDuration != 2 {
		t.Fatalf("expected chase %s renamed to Sweep2 with step-duration 2, ID preserved, got %+v", chaseSweepID, afterChaseUpdate.Chases)
	}

	// --- chase reorder: non-permutation rejection, deterministic success ---
	chaseReorderNonPermutation := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Sweep2", "--order", "0,0,1", "--show", showPath,
	}})
	if chaseReorderNonPermutation.ExitCode != 2 || !strings.Contains(string(chaseReorderNonPermutation.Stderr), "GOLC_CHASE_USAGE") {
		t.Fatalf("expected exit 2 GOLC_CHASE_USAGE for a non-permutation --order, got exit=%d stderr=%s", chaseReorderNonPermutation.ExitCode, chaseReorderNonPermutation.Stderr)
	}
	chaseReorder := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Sweep2", "--order", "2,0,1", "--show", showPath,
	}})
	if chaseReorder.ExitCode != 0 {
		t.Fatalf("chase reorder failed: exit=%d stderr=%s", chaseReorder.ExitCode, chaseReorder.Stderr)
	}
	afterReorder, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after chase reorder: %v", err)
	}
	reorderedChase, found := findChaseByID(afterReorder.Chases, chaseSweepID)
	if !found {
		t.Fatalf("expected chase %s to still exist after reorder", chaseSweepID)
	}
	if len(reorderedChase.Steps) != 3 ||
		reorderedChase.Steps[0].Attributes[0].Value != 2 ||
		reorderedChase.Steps[1].Attributes[0].Value != 0 ||
		reorderedChase.Steps[2].Attributes[0].Value != 1 {
		t.Fatalf("expected steps permuted to original-index order [2,0,1], got %+v", reorderedChase.Steps)
	}

	// --- chase duplicate: fresh ID, copied steps ---
	chaseDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "duplicate", "Sweep2", "Sweep3", "--show", showPath,
	}})
	if chaseDuplicate.ExitCode != 0 {
		t.Fatalf("chase duplicate failed: exit=%d stderr=%s", chaseDuplicate.ExitCode, chaseDuplicate.Stderr)
	}
	afterChaseDuplicate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after chase duplicate: %v", err)
	}
	var duplicatedChase *programming.Chase
	for i := range afterChaseDuplicate.Chases {
		if afterChaseDuplicate.Chases[i].Name == "Sweep3" {
			duplicatedChase = &afterChaseDuplicate.Chases[i]
		}
	}
	if duplicatedChase == nil {
		t.Fatalf("expected a duplicated chase named Sweep3, got %+v", afterChaseDuplicate.Chases)
	}
	if duplicatedChase.ID == chaseSweepID {
		t.Fatalf("expected the duplicated chase to mint a fresh ID distinct from the source")
	}
	if len(duplicatedChase.Steps) != 3 {
		t.Fatalf("expected the duplicated chase to copy all 3 steps, got %+v", duplicatedChase.Steps)
	}

	// --- chase delete: not-found, success ---
	chaseDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "delete", "NoSuchChase", "--show", showPath,
	}})
	if chaseDeleteNotFound.ExitCode == 0 || !strings.Contains(string(chaseDeleteNotFound.Stderr), "GOLC_CHASE_NOT_FOUND") {
		t.Fatalf("expected GOLC_CHASE_NOT_FOUND, got exit=%d stderr=%s", chaseDeleteNotFound.ExitCode, chaseDeleteNotFound.Stderr)
	}
	chaseDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "delete", "Sweep3", "--show", showPath,
	}})
	if chaseDelete.ExitCode != 0 {
		t.Fatalf("chase delete failed: exit=%d stderr=%s", chaseDelete.ExitCode, chaseDelete.Stderr)
	}

	// --- motion rename: duplicate-name rejection, success (ID-stable) ---
	motionDup := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "rename", "Arc", "Fade", "--show", showPath,
	}})
	if motionDup.ExitCode == 0 || !strings.Contains(string(motionDup.Stderr), "GOLC_MOTION_PRESET_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_MOTION_PRESET_DUPLICATE_NAME, got exit=%d stderr=%s", motionDup.ExitCode, motionDup.Stderr)
	}
	motionRename := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "rename", "Arc", "Sweep Motion", "--show", showPath,
	}})
	if motionRename.ExitCode != 0 {
		t.Fatalf("motion rename failed: exit=%d stderr=%s", motionRename.ExitCode, motionRename.Stderr)
	}
	afterMotionRename, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after motion rename: %v", err)
	}
	renamedMotion, found := findMotionByID(afterMotionRename.MotionPresets, motionArcID)
	if !found || renamedMotion.Name != "Sweep Motion" {
		t.Fatalf("expected motion preset %s renamed to 'Sweep Motion' with ID preserved, got %+v", motionArcID, afterMotionRename.MotionPresets)
	}

	// --- motion duplicate: fresh ID ---
	motionDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "duplicate", "Sweep Motion", "Sweep Motion Copy", "--show", showPath,
	}})
	if motionDuplicate.ExitCode != 0 {
		t.Fatalf("motion duplicate failed: exit=%d stderr=%s", motionDuplicate.ExitCode, motionDuplicate.Stderr)
	}
	afterMotionDuplicate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after motion duplicate: %v", err)
	}
	var duplicatedMotion *programming.MotionPreset
	for i := range afterMotionDuplicate.MotionPresets {
		if afterMotionDuplicate.MotionPresets[i].Name == "Sweep Motion Copy" {
			duplicatedMotion = &afterMotionDuplicate.MotionPresets[i]
		}
	}
	if duplicatedMotion == nil {
		t.Fatalf("expected a duplicated motion preset named 'Sweep Motion Copy', got %+v", afterMotionDuplicate.MotionPresets)
	}
	if duplicatedMotion.ID == motionArcID {
		t.Fatalf("expected the duplicated motion preset to mint a fresh ID distinct from the source")
	}

	// --- motion delete: not-found, success ---
	motionDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "delete", "NoSuchMotion", "--show", showPath,
	}})
	if motionDeleteNotFound.ExitCode == 0 || !strings.Contains(string(motionDeleteNotFound.Stderr), "GOLC_MOTION_PRESET_NOT_FOUND") {
		t.Fatalf("expected GOLC_MOTION_PRESET_NOT_FOUND, got exit=%d stderr=%s", motionDeleteNotFound.ExitCode, motionDeleteNotFound.Stderr)
	}
	motionDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "delete", "Sweep Motion Copy", "--show", showPath,
	}})
	if motionDelete.ExitCode != 0 {
		t.Fatalf("motion delete failed: exit=%d stderr=%s", motionDelete.ExitCode, motionDelete.Stderr)
	}

	// --- scene rename: duplicate-name rejection, success (ID-stable) ---
	sceneDup := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "rename", "Primary", "Secondary", "--show", showPath,
	}})
	if sceneDup.ExitCode == 0 || !strings.Contains(string(sceneDup.Stderr), "GOLC_SCENE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_SCENE_DUPLICATE_NAME, got exit=%d stderr=%s", sceneDup.ExitCode, sceneDup.Stderr)
	}
	sceneRename := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "rename", "Primary", "Main Stage", "--show", showPath,
	}})
	if sceneRename.ExitCode != 0 {
		t.Fatalf("scene rename failed: exit=%d stderr=%s", sceneRename.ExitCode, sceneRename.Stderr)
	}
	afterSceneRename, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after scene rename: %v", err)
	}
	renamedScene, found := findSceneByName(afterSceneRename.Scenes, "Main Stage")
	if !found || renamedScene.ID != scenePrimaryID {
		t.Fatalf("expected scene %s renamed to 'Main Stage' with ID preserved, got %+v", scenePrimaryID, afterSceneRename.Scenes)
	}

	// --- scene duplicate: fresh ID, forced-inactive ---
	sceneDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "duplicate", "Main Stage", "Main Stage Copy", "--show", showPath,
	}})
	if sceneDuplicate.ExitCode != 0 {
		t.Fatalf("scene duplicate failed: exit=%d stderr=%s", sceneDuplicate.ExitCode, sceneDuplicate.Stderr)
	}
	afterSceneDuplicate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after scene duplicate: %v", err)
	}
	duplicatedScene, found := findSceneByName(afterSceneDuplicate.Scenes, "Main Stage Copy")
	if !found {
		t.Fatalf("expected a duplicated scene named 'Main Stage Copy', got %+v", afterSceneDuplicate.Scenes)
	}
	if duplicatedScene.ID == scenePrimaryID {
		t.Fatalf("expected the duplicated scene to mint a fresh ID distinct from the source")
	}
	if duplicatedScene.BarsPerLoop != 4 {
		t.Fatalf("expected the duplicated scene to copy BarsPerLoop=4, got %d", duplicatedScene.BarsPerLoop)
	}
	if duplicatedScene.Active {
		t.Fatalf("expected the duplicated scene to start inactive regardless of the source's Active state")
	}

	// --- scene delete: not-found, success ---
	sceneDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "delete", "NoSuchScene", "--show", showPath,
	}})
	if sceneDeleteNotFound.ExitCode == 0 || !strings.Contains(string(sceneDeleteNotFound.Stderr), "GOLC_SCENE_NOT_FOUND") {
		t.Fatalf("expected GOLC_SCENE_NOT_FOUND, got exit=%d stderr=%s", sceneDeleteNotFound.ExitCode, sceneDeleteNotFound.Stderr)
	}
	sceneDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "delete", "Main Stage Copy", "--show", showPath,
	}})
	if sceneDelete.ExitCode != 0 {
		t.Fatalf("scene delete failed: exit=%d stderr=%s", sceneDelete.ExitCode, sceneDelete.Stderr)
	}
}

// TestHistoryLiveActiveEdit proves CONTEXT D-08: any CRUD verb succeeds
// against an object referenced by (or, for the scene-duplicate case,
// literally being) the currently-active scene, with no pause/detach/lock
// precondition -- no handler under test here ever inspects
// scene.Scene.Active before mutating.
func TestHistoryLiveActiveEdit(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	theme, err := programming.NewTheme("Warm")
	if err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	chase, err := programming.NewChase("Live Sweep", []programming.ChaseStep{
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 0}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 1}}},
	}, programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	motion, err := programming.NewMotionPreset("Live Arc", nil)
	if err != nil {
		t.Fatalf("NewMotionPreset: %v", err)
	}
	// "Spare" is deliberately NOT referenced by any scene layer below --
	// see the delete sub-test's comment for why deleting an unreferenced
	// object (rather than a referenced one) is the correct probe for D-08.
	spare, err := programming.NewPreset("Spare", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset(Spare): %v", err)
	}

	main, err := scene.NewScene("Main", 4)
	if err != nil {
		t.Fatalf("NewScene(Main): %v", err)
	}
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: theme.ID})
	if err != nil {
		t.Fatalf("SetLayer(ColorTheme): %v", err)
	}
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.Chase, Enabled: true, Ref: chase.ID})
	if err != nil {
		t.Fatalf("SetLayer(Chase): %v", err)
	}
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.Motion, Enabled: true, Ref: motion.ID})
	if err != nil {
		t.Fatalf("SetLayer(Motion): %v", err)
	}
	main.Active = true

	state := show.State{
		Themes:        []programming.Theme{theme},
		Presets:       []programming.Preset{spare},
		Chases:        []programming.Chase{chase},
		MotionPresets: []programming.MotionPreset{motion},
		Scenes:        []scene.Scene{main},
	}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save (seed): %v", err)
	}

	seeded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (seed reload): %v", err)
	}
	if len(seeded.Scenes) != 1 || !seeded.Scenes[0].Active {
		t.Fatalf("expected exactly one active scene in the seeded state, got %+v", seeded.Scenes)
	}

	// rename: "Warm" is referenced by Main's active ColorTheme layer.
	// Renaming never changes the theme's ID, so the layer's Ref stays
	// resolvable -- this must succeed with no pause/detach/lock
	// precondition, and it does: runThemeRename never reads scene.Active.
	renameResult := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "Warm", "Warm2", "--show", showPath,
	}})
	if renameResult.ExitCode != 0 {
		t.Fatalf("theme rename against a live-active-referenced theme failed: exit=%d stderr=%s", renameResult.ExitCode, renameResult.Stderr)
	}

	// reorder: "Live Sweep" is referenced by Main's active Chase layer.
	// Reordering steps never changes the chase's ID.
	reorderResult := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Live Sweep", "--order", "1,0", "--show", showPath,
	}})
	if reorderResult.ExitCode != 0 {
		t.Fatalf("chase reorder against a live-active-referenced chase failed: exit=%d stderr=%s", reorderResult.ExitCode, reorderResult.Stderr)
	}

	// duplicate: "Live Arc" is referenced by Main's active Motion layer.
	// Duplicating never touches the source object, so the active scene's
	// Ref is untouched.
	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "duplicate", "Live Arc", "Live Arc Copy", "--show", showPath,
	}})
	if duplicateResult.ExitCode != 0 {
		t.Fatalf("motion duplicate against a live-active-referenced motion preset failed: exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)
	}

	// duplicate (scene itself): Main is the currently-active scene. This
	// both proves the no-gate rule (duplicating the active scene succeeds)
	// and the duplicate-never-inherits-Active safeguard (scene.NewScene's
	// own Active=false default keeps the copy from becoming a second
	// active scene, SCEN-04) in one call.
	sceneDuplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "duplicate", "Main", "Main Copy", "--show", showPath,
	}})
	if sceneDuplicateResult.ExitCode != 0 {
		t.Fatalf("scene duplicate against the currently-active scene failed: exit=%d stderr=%s", sceneDuplicateResult.ExitCode, sceneDuplicateResult.Stderr)
	}

	// delete: "Spare" is a preset that is NOT referenced by any scene
	// layer. Deleting the *actually-referenced* theme/chase/motion above
	// would legitimately fail via GOLC_SCENE_LAYER_DANGLING_REFERENCE --
	// that is show.Save's separate, always-on referential-integrity
	// safeguard (CONTEXT threat T-03-01), not a live-edit workflow gate,
	// and proving it is not what D-08 claims. This delete instead proves
	// the distinct D-08 claim under test: a delete verb is never blocked
	// merely because some scene happens to be active -- there is no
	// global "an active scene exists" precondition anywhere in the delete
	// handlers.
	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "Spare", "--show", showPath,
	}})
	if deleteResult.ExitCode != 0 {
		t.Fatalf("preset delete while a scene is active failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	}

	final, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (final): %v", err)
	}
	if len(final.Presets) != 0 {
		t.Fatalf("expected Spare to be deleted, got %+v", final.Presets)
	}
	mainScene, found := findSceneByName(final.Scenes, "Main")
	if !found || !mainScene.Active {
		t.Fatalf("expected the original Main scene to remain active, got %+v", final.Scenes)
	}
	mainCopyScene, found := findSceneByName(final.Scenes, "Main Copy")
	if !found || mainCopyScene.Active {
		t.Fatalf("expected the duplicated Main Copy scene to be inactive, got %+v", final.Scenes)
	}
}
