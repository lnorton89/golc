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
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	ocean, err := programming.NewTheme("Ocean")
	require.NoError(t, err)

	fullWash, err := programming.NewPreset("Full Wash", programming.PresetIntensity)
	require.NoError(t, err)
	house, err := programming.NewPreset("House", programming.PresetIntensity)
	require.NoError(t, err)

	// Steps are tagged with a distinguishing Attributes[0].Value equal to
	// their original index (0, 1, 2) so a later "chase reorder" can be
	// verified by reading which tagged step now occupies each position.
	steps := []programming.ChaseStep{
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 0}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 1}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 2}}},
	}
	sweep, err := programming.NewChase("Sweep", steps, programming.StepUnitBar, 1)
	require.NoError(t, err)

	arc, err := programming.NewMotionPreset("Arc", nil)
	require.NoError(t, err)
	fade, err := programming.NewMotionPreset("Fade", nil)
	require.NoError(t, err)

	primary, err := scene.NewScene("Primary", 4)
	require.NoError(t, err)
	secondary, err := scene.NewScene("Secondary", 8)
	require.NoError(t, err)

	state := show.State{
		Themes:        []programming.Theme{sunset, ocean},
		Presets:       []programming.Preset{fullWash, house},
		Chases:        []programming.Chase{sweep},
		MotionPresets: []programming.MotionPreset{arc, fade},
		Scenes:        []scene.Scene{primary, secondary},
	}
	if err := show.Save(root, showPath, state); err != nil {
		require.NoError(t, err)
	}
	reloaded, err := show.Load(root, showPath)
	require.NoError(t, err)
	return reloaded
}

func TestHistoryRoutes(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
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
	require.NotEqual(t, 0, dupTheme.ExitCode)
	require.Contains(t, string(dupTheme.Stderr), "GOLC_THEME_DUPLICATE_NAME", "expected GOLC_THEME_DUPLICATE_NAME renaming Sunset->Ocean (existing name), got exit=%d stderr=%s", dupTheme.ExitCode, dupTheme.Stderr)

	themeRename := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "Sunset", "Sunrise", "--show", showPath,
	}})
	require.Equal(t, 0, themeRename.ExitCode, "theme rename failed: exit=%d stderr=%s", themeRename.ExitCode, themeRename.Stderr)
	afterThemeRename, err := show.Load(root, showPath)
	require.NoError(t, err)
	renamedTheme, found := findThemeByID(afterThemeRename.Themes, themeSunsetID)
	require.True(t, found)
	require.Equal(t, "Sunrise", renamedTheme.Name, "expected theme %s renamed to Sunrise with ID preserved, got %+v", themeSunsetID, afterThemeRename.Themes)

	themeRenameNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "NoSuchTheme", "Whatever", "--show", showPath,
	}})
	require.NotEqual(t, 0, themeRenameNotFound.ExitCode)
	require.Contains(t, string(themeRenameNotFound.Stderr), "GOLC_THEME_NOT_FOUND", "expected GOLC_THEME_NOT_FOUND, got exit=%d stderr=%s", themeRenameNotFound.ExitCode, themeRenameNotFound.Stderr)

	// --- theme delete: not-found, success ---
	themeDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "delete", "NoSuchTheme", "--show", showPath,
	}})
	require.NotEqual(t, 0, themeDeleteNotFound.ExitCode)
	require.Contains(t, string(themeDeleteNotFound.Stderr), "GOLC_THEME_NOT_FOUND", "expected GOLC_THEME_NOT_FOUND, got exit=%d stderr=%s", themeDeleteNotFound.ExitCode, themeDeleteNotFound.Stderr)
	themeDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "delete", "Sunrise", "--show", showPath,
	}})
	require.Equal(t, 0, themeDelete.ExitCode, "theme delete failed: exit=%d stderr=%s", themeDelete.ExitCode, themeDelete.Stderr)
	afterThemeDelete, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, afterThemeDelete.Themes, 1)
	require.Equal(t, "Ocean", afterThemeDelete.Themes[0].Name, "expected exactly Ocean to remain after deleting Sunrise, got %+v", afterThemeDelete.Themes)

	// --- preset rename: duplicate-name rejection, success (ID-stable) ---
	presetDup := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "rename", "Full Wash", "House", "--show", showPath,
	}})
	require.NotEqual(t, 0, presetDup.ExitCode)
	require.Contains(t, string(presetDup.Stderr), "GOLC_PRESET_DUPLICATE_NAME", "expected GOLC_PRESET_DUPLICATE_NAME, got exit=%d stderr=%s", presetDup.ExitCode, presetDup.Stderr)
	presetRename := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "rename", "Full Wash", "Warm Wash", "--show", showPath,
	}})
	require.Equal(t, 0, presetRename.ExitCode, "preset rename failed: exit=%d stderr=%s", presetRename.ExitCode, presetRename.Stderr)
	afterPresetRename, err := show.Load(root, showPath)
	require.NoError(t, err)
	renamedPreset, found := findPresetByID(afterPresetRename.Presets, presetFullWashID)
	require.True(t, found)
	require.Equal(t, "Warm Wash", renamedPreset.Name, "expected preset %s renamed to Warm Wash with ID preserved, got %+v", presetFullWashID, afterPresetRename.Presets)

	// --- preset delete: not-found, success ---
	presetDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "NoSuchPreset", "--show", showPath,
	}})
	require.NotEqual(t, 0, presetDeleteNotFound.ExitCode)
	require.Contains(t, string(presetDeleteNotFound.Stderr), "GOLC_PRESET_NOT_FOUND", "expected GOLC_PRESET_NOT_FOUND, got exit=%d stderr=%s", presetDeleteNotFound.ExitCode, presetDeleteNotFound.Stderr)
	presetDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "Warm Wash", "--show", showPath,
	}})
	require.Equal(t, 0, presetDelete.ExitCode, "preset delete failed: exit=%d stderr=%s", presetDelete.ExitCode, presetDelete.Stderr)

	// --- chase update: usage rejection (no fields), success (rename+step-duration, ID-stable) ---
	chaseUpdateMissingFields := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "update", "Sweep", "--show", showPath,
	}})
	require.Equal(t, 2, chaseUpdateMissingFields.ExitCode)
	require.Contains(t, string(chaseUpdateMissingFields.Stderr), "GOLC_CHASE_USAGE", "expected exit 2 GOLC_CHASE_USAGE for chase update with no fields, got exit=%d stderr=%s", chaseUpdateMissingFields.ExitCode, chaseUpdateMissingFields.Stderr)
	chaseUpdate := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "update", "Sweep", "--name", "Sweep2", "--step-duration", "2", "--show", showPath,
	}})
	require.Equal(t, 0, chaseUpdate.ExitCode, "chase update failed: exit=%d stderr=%s", chaseUpdate.ExitCode, chaseUpdate.Stderr)
	afterChaseUpdate, err := show.Load(root, showPath)
	require.NoError(t, err)
	updatedChase, found := findChaseByID(afterChaseUpdate.Chases, chaseSweepID)
	require.True(t, found)
	require.Equal(t, "Sweep2", updatedChase.Name)
	require.EqualValues(t, 2, updatedChase.StepDuration, "expected chase %s renamed to Sweep2 with step-duration 2, ID preserved, got %+v", chaseSweepID, afterChaseUpdate.Chases)

	// --- chase reorder: non-permutation rejection, deterministic success ---
	chaseReorderNonPermutation := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Sweep2", "--order", "0,0,1", "--show", showPath,
	}})
	require.Equal(t, 2, chaseReorderNonPermutation.ExitCode)
	require.Contains(t, string(chaseReorderNonPermutation.Stderr), "GOLC_CHASE_USAGE", "expected exit 2 GOLC_CHASE_USAGE for a non-permutation --order, got exit=%d stderr=%s", chaseReorderNonPermutation.ExitCode, chaseReorderNonPermutation.Stderr)
	chaseReorder := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Sweep2", "--order", "2,0,1", "--show", showPath,
	}})
	require.Equal(t, 0, chaseReorder.ExitCode, "chase reorder failed: exit=%d stderr=%s", chaseReorder.ExitCode, chaseReorder.Stderr)
	afterReorder, err := show.Load(root, showPath)
	require.NoError(t, err)
	reorderedChase, found := findChaseByID(afterReorder.Chases, chaseSweepID)
	require.True(t, found, "expected chase %s to still exist after reorder", chaseSweepID)
	if len(reorderedChase.Steps) != 3 ||
		reorderedChase.Steps[0].Attributes[0].Value != 2 ||
		reorderedChase.Steps[1].Attributes[0].Value != 0 ||
		reorderedChase.Steps[2].Attributes[0].Value != 1 {
		require.Len(t, reorderedChase.Steps, 3)
		require.Equal(t, 2, reorderedChase.Steps[0].Attributes[0].Value)
		require.Equal(t, 0, reorderedChase.Steps[1].Attributes[0].Value)
		require.Equal(t, 1, reorderedChase.Steps[2].Attributes[0].Value, "expected steps permuted to original-index order [2,0,1], got %+v", reorderedChase.Steps)
	}

	// --- chase duplicate: fresh ID, copied steps ---
	chaseDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "duplicate", "Sweep2", "Sweep3", "--show", showPath,
	}})
	require.Equal(t, 0, chaseDuplicate.ExitCode, "chase duplicate failed: exit=%d stderr=%s", chaseDuplicate.ExitCode, chaseDuplicate.Stderr)
	afterChaseDuplicate, err := show.Load(root, showPath)
	require.NoError(t, err)
	var duplicatedChase *programming.Chase
	for i := range afterChaseDuplicate.Chases {
		if afterChaseDuplicate.Chases[i].Name == "Sweep3" {
			duplicatedChase = &afterChaseDuplicate.Chases[i]
		}
	}
	require.NotNil(t, duplicatedChase, "expected a duplicated chase named Sweep3, got %+v", afterChaseDuplicate.Chases)
	require.NotEqual(t, chaseSweepID, duplicatedChase.ID, "expected the duplicated chase to mint a fresh ID distinct from the source")
	require.Len(t, duplicatedChase.Steps, 3, "expected the duplicated chase to copy all 3 steps, got %+v", duplicatedChase.Steps)

	// --- chase delete: not-found, success ---
	chaseDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "delete", "NoSuchChase", "--show", showPath,
	}})
	require.NotEqual(t, 0, chaseDeleteNotFound.ExitCode)
	require.Contains(t, string(chaseDeleteNotFound.Stderr), "GOLC_CHASE_NOT_FOUND", "expected GOLC_CHASE_NOT_FOUND, got exit=%d stderr=%s", chaseDeleteNotFound.ExitCode, chaseDeleteNotFound.Stderr)
	chaseDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "delete", "Sweep3", "--show", showPath,
	}})
	require.Equal(t, 0, chaseDelete.ExitCode, "chase delete failed: exit=%d stderr=%s", chaseDelete.ExitCode, chaseDelete.Stderr)

	// --- motion rename: duplicate-name rejection, success (ID-stable) ---
	motionDup := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "rename", "Arc", "Fade", "--show", showPath,
	}})
	require.NotEqual(t, 0, motionDup.ExitCode)
	require.Contains(t, string(motionDup.Stderr), "GOLC_MOTION_PRESET_DUPLICATE_NAME", "expected GOLC_MOTION_PRESET_DUPLICATE_NAME, got exit=%d stderr=%s", motionDup.ExitCode, motionDup.Stderr)
	motionRename := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "rename", "Arc", "Sweep Motion", "--show", showPath,
	}})
	require.Equal(t, 0, motionRename.ExitCode, "motion rename failed: exit=%d stderr=%s", motionRename.ExitCode, motionRename.Stderr)
	afterMotionRename, err := show.Load(root, showPath)
	require.NoError(t, err)
	renamedMotion, found := findMotionByID(afterMotionRename.MotionPresets, motionArcID)
	require.True(t, found)
	require.Equal(t, "Sweep Motion", renamedMotion.Name, "expected motion preset %s renamed to 'Sweep Motion' with ID preserved, got %+v", motionArcID, afterMotionRename.MotionPresets)

	// --- motion duplicate: fresh ID ---
	motionDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "duplicate", "Sweep Motion", "Sweep Motion Copy", "--show", showPath,
	}})
	require.Equal(t, 0, motionDuplicate.ExitCode, "motion duplicate failed: exit=%d stderr=%s", motionDuplicate.ExitCode, motionDuplicate.Stderr)
	afterMotionDuplicate, err := show.Load(root, showPath)
	require.NoError(t, err)
	var duplicatedMotion *programming.MotionPreset
	for i := range afterMotionDuplicate.MotionPresets {
		if afterMotionDuplicate.MotionPresets[i].Name == "Sweep Motion Copy" {
			duplicatedMotion = &afterMotionDuplicate.MotionPresets[i]
		}
	}
	require.NotNil(t, duplicatedMotion, "expected a duplicated motion preset named 'Sweep Motion Copy', got %+v", afterMotionDuplicate.MotionPresets)
	require.NotEqual(t, motionArcID, duplicatedMotion.ID, "expected the duplicated motion preset to mint a fresh ID distinct from the source")

	// --- motion delete: not-found, success ---
	motionDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "delete", "NoSuchMotion", "--show", showPath,
	}})
	require.NotEqual(t, 0, motionDeleteNotFound.ExitCode)
	require.Contains(t, string(motionDeleteNotFound.Stderr), "GOLC_MOTION_PRESET_NOT_FOUND", "expected GOLC_MOTION_PRESET_NOT_FOUND, got exit=%d stderr=%s", motionDeleteNotFound.ExitCode, motionDeleteNotFound.Stderr)
	motionDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "delete", "Sweep Motion Copy", "--show", showPath,
	}})
	require.Equal(t, 0, motionDelete.ExitCode, "motion delete failed: exit=%d stderr=%s", motionDelete.ExitCode, motionDelete.Stderr)

	// --- scene rename: duplicate-name rejection, success (ID-stable) ---
	sceneDup := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "rename", "Primary", "Secondary", "--show", showPath,
	}})
	require.NotEqual(t, 0, sceneDup.ExitCode)
	require.Contains(t, string(sceneDup.Stderr), "GOLC_SCENE_DUPLICATE_NAME", "expected GOLC_SCENE_DUPLICATE_NAME, got exit=%d stderr=%s", sceneDup.ExitCode, sceneDup.Stderr)
	sceneRename := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "rename", "Primary", "Main Stage", "--show", showPath,
	}})
	require.Equal(t, 0, sceneRename.ExitCode, "scene rename failed: exit=%d stderr=%s", sceneRename.ExitCode, sceneRename.Stderr)
	afterSceneRename, err := show.Load(root, showPath)
	require.NoError(t, err)
	renamedScene, found := findSceneByName(afterSceneRename.Scenes, "Main Stage")
	require.True(t, found)
	require.Equal(t, scenePrimaryID, renamedScene.ID, "expected scene %s renamed to 'Main Stage' with ID preserved, got %+v", scenePrimaryID, afterSceneRename.Scenes)

	// --- scene duplicate: fresh ID, forced-inactive ---
	sceneDuplicate := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "duplicate", "Main Stage", "Main Stage Copy", "--show", showPath,
	}})
	require.Equal(t, 0, sceneDuplicate.ExitCode, "scene duplicate failed: exit=%d stderr=%s", sceneDuplicate.ExitCode, sceneDuplicate.Stderr)
	afterSceneDuplicate, err := show.Load(root, showPath)
	require.NoError(t, err)
	duplicatedScene, found := findSceneByName(afterSceneDuplicate.Scenes, "Main Stage Copy")
	require.True(t, found, "expected a duplicated scene named 'Main Stage Copy', got %+v", afterSceneDuplicate.Scenes)
	require.NotEqual(t, scenePrimaryID, duplicatedScene.ID, "expected the duplicated scene to mint a fresh ID distinct from the source")
	require.Equal(t, 4, duplicatedScene.BarsPerLoop, "expected the duplicated scene to copy BarsPerLoop=4, got %d", duplicatedScene.BarsPerLoop)
	require.False(t, duplicatedScene.Active, "expected the duplicated scene to start inactive regardless of the source's Active state")

	// --- scene delete: not-found, success ---
	sceneDeleteNotFound := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "delete", "NoSuchScene", "--show", showPath,
	}})
	require.NotEqual(t, 0, sceneDeleteNotFound.ExitCode)
	require.Contains(t, string(sceneDeleteNotFound.Stderr), "GOLC_SCENE_NOT_FOUND", "expected GOLC_SCENE_NOT_FOUND, got exit=%d stderr=%s", sceneDeleteNotFound.ExitCode, sceneDeleteNotFound.Stderr)
	sceneDelete := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "delete", "Main Stage Copy", "--show", showPath,
	}})
	require.Equal(t, 0, sceneDelete.ExitCode, "scene delete failed: exit=%d stderr=%s", sceneDelete.ExitCode, sceneDelete.Stderr)
}

// TestHistoryLiveActiveEdit proves CONTEXT D-08: any CRUD verb succeeds
// against an object referenced by (or, for the scene-duplicate case,
// literally being) the currently-active scene, with no pause/detach/lock
// precondition -- no handler under test here ever inspects
// scene.Scene.Active before mutating.
func TestHistoryLiveActiveEdit(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.json")

	theme, err := programming.NewTheme("Warm")
	require.NoError(t, err)
	chase, err := programming.NewChase("Live Sweep", []programming.ChaseStep{
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 0}}},
		{Attributes: []programming.PresetAttribute{{Capability: "intensity", Value: 1}}},
	}, programming.StepUnitBar, 1)
	require.NoError(t, err)
	motion, err := programming.NewMotionPreset("Live Arc", nil)
	require.NoError(t, err)
	// "Spare" is deliberately NOT referenced by any scene layer below --
	// see the delete sub-test's comment for why deleting an unreferenced
	// object (rather than a referenced one) is the correct probe for D-08.
	spare, err := programming.NewPreset("Spare", programming.PresetIntensity)
	require.NoError(t, err)

	main, err := scene.NewScene("Main", 4)
	require.NoError(t, err)
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: theme.ID})
	require.NoError(t, err)
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.Chase, Enabled: true, Ref: chase.ID})
	require.NoError(t, err)
	main, err = scene.SetLayer(main, scene.Layer{Kind: scene.Motion, Enabled: true, Ref: motion.ID})
	require.NoError(t, err)
	main.Active = true

	state := show.State{
		Themes:        []programming.Theme{theme},
		Presets:       []programming.Preset{spare},
		Chases:        []programming.Chase{chase},
		MotionPresets: []programming.MotionPreset{motion},
		Scenes:        []scene.Scene{main},
	}
	if err := show.Save(root, showPath, state); err != nil {
		require.NoError(t, err)
	}

	seeded, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, seeded.Scenes, 1)
	require.True(t, seeded.Scenes[0].Active, "expected exactly one active scene in the seeded state, got %+v", seeded.Scenes)

	// rename: "Warm" is referenced by Main's active ColorTheme layer.
	// Renaming never changes the theme's ID, so the layer's Ref stays
	// resolvable -- this must succeed with no pause/detach/lock
	// precondition, and it does: runThemeRename never reads scene.Active.
	renameResult := registry.Execute(command.Request{Root: root, Args: []string{
		"theme", "rename", "Warm", "Warm2", "--show", showPath,
	}})
	require.Equal(t, 0, renameResult.ExitCode, "theme rename against a live-active-referenced theme failed: exit=%d stderr=%s", renameResult.ExitCode, renameResult.Stderr)

	// reorder: "Live Sweep" is referenced by Main's active Chase layer.
	// Reordering steps never changes the chase's ID.
	reorderResult := registry.Execute(command.Request{Root: root, Args: []string{
		"chase", "reorder", "Live Sweep", "--order", "1,0", "--show", showPath,
	}})
	require.Equal(t, 0, reorderResult.ExitCode, "chase reorder against a live-active-referenced chase failed: exit=%d stderr=%s", reorderResult.ExitCode, reorderResult.Stderr)

	// duplicate: "Live Arc" is referenced by Main's active Motion layer.
	// Duplicating never touches the source object, so the active scene's
	// Ref is untouched.
	duplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"motion", "duplicate", "Live Arc", "Live Arc Copy", "--show", showPath,
	}})
	require.Equal(t, 0, duplicateResult.ExitCode, "motion duplicate against a live-active-referenced motion preset failed: exit=%d stderr=%s", duplicateResult.ExitCode, duplicateResult.Stderr)

	// duplicate (scene itself): Main is the currently-active scene. This
	// both proves the no-gate rule (duplicating the active scene succeeds)
	// and the duplicate-never-inherits-Active safeguard (scene.NewScene's
	// own Active=false default keeps the copy from becoming a second
	// active scene, SCEN-04) in one call.
	sceneDuplicateResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "duplicate", "Main", "Main Copy", "--show", showPath,
	}})
	require.Equal(t, 0, sceneDuplicateResult.ExitCode, "scene duplicate against the currently-active scene failed: exit=%d stderr=%s", sceneDuplicateResult.ExitCode, sceneDuplicateResult.Stderr)

	// delete: "Spare" is a preset that is NOT referenced by any scene
	// layer, so this exercises the plain not-referenced path. Deleting the
	// *actually-referenced* theme/chase/motion above would instead reset
	// the referencing scene's layer to its default, un-refed state
	// (scene.ScrubLayerRef) rather than failing -- deletion is never
	// blocked by a scene reference, so it is also never blocked merely
	// because some scene happens to be active. This delete proves the
	// distinct D-08 claim under test: there is no global "an active scene
	// exists" precondition anywhere in the delete handlers.
	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{
		"preset", "delete", "Spare", "--show", showPath,
	}})
	require.Equal(t, 0, deleteResult.ExitCode, "preset delete while a scene is active failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)

	final, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, final.Presets, 0, "expected Spare to be deleted, got %+v", final.Presets)
	mainScene, found := findSceneByName(final.Scenes, "Main")
	require.True(t, found)
	require.True(t, mainScene.Active, "expected the original Main scene to remain active, got %+v", final.Scenes)
	mainCopyScene, found := findSceneByName(final.Scenes, "Main Copy")
	require.True(t, found)
	require.False(t, mainCopyScene.Active, "expected the duplicated Main Copy scene to be inactive, got %+v", final.Scenes)
}
