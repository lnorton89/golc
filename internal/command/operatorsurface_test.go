// operatorsurface_test.go proves the "operatorsurface" command scope's
// route contract (06-01-PLAN.md Task 3): create/list/assign/unassign/show
// round-trip through show.Save/Load, a second identical assign is
// idempotent, an unknown scene/group/surface selector is rejected, and
// Authorize rejects a control not currently assigned to the surface
// (GOLC_OPERATORSURFACE_LOCKED, D-04). Mirrors playback_bpm_test.go's
// seed-then-exercise-CLI-routes convention.
package command_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/show"
	"github.com/stretchr/testify/require"
)

func TestOperatorSurfaceCreateAndList(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "create", "Front of House", "--show", showPath,
	}})
	require.Equal(t, 0, createResult.ExitCode, "operatorsurface create failed: exit=%d stderr=%s", createResult.ExitCode, createResult.Stderr)
	require.Contains(t, string(createResult.Stdout), "GOLC_OPERATORSURFACE_CREATED", "expected GOLC_OPERATORSURFACE_CREATED in stdout, got %s", createResult.Stdout)

	listResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "list", "--show", showPath,
	}})
	require.Equal(t, 0, listResult.ExitCode, "operatorsurface list failed: exit=%d stderr=%s", listResult.ExitCode, listResult.Stderr)
	require.Contains(t, string(listResult.Stdout), "Front of House", "expected Front of House in list output, got %s", listResult.Stdout)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, state.OperatorSurfaces, 1)
	require.Equal(t, "Front of House", state.OperatorSurfaces[0].Name, "expected exactly one persisted surface, got %+v", state.OperatorSurfaces)
}

// seedOperatorSurfaceShow builds a ShowState carrying one scene (with a
// ColorTheme layer), one group, and one operator surface, saving it at
// showPath so assign/unassign/show tests have real items to reference.
func seedOperatorSurfaceShow(t *testing.T, root, showPath string) {
	t.Helper()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)

	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Opener", "--bars", "4", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "scene create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "create", "Wash Pool", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "create", "Front of House", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestOperatorSurfaceAssignSceneIdempotentAndShowReflectsIt(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	first := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	require.Equal(t, 0, first.ExitCode, "operatorsurface assign (first) failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)

	// A second identical assign is idempotent -- not rejected, and does not
	// duplicate the assignment.
	second := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	require.Equal(t, 0, second.ExitCode, "operatorsurface assign (idempotent repeat) failed: exit=%d stderr=%s", second.ExitCode, second.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, state.OperatorSurfaces, 1)
	require.Len(t, state.OperatorSurfaces[0].SceneRefs, 1, "expected exactly one scene ref after an idempotent re-assign, got %+v", state.OperatorSurfaces)

	showResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "show", "--surface", "Front of House", "--show", showPath,
	}})
	require.Equal(t, 0, showResult.ExitCode, "operatorsurface show failed: exit=%d stderr=%s", showResult.ExitCode, showResult.Stderr)
	require.Contains(t, string(showResult.Stdout), "scenes: 1", "expected operatorsurface show to reflect the assignment, got %s", showResult.Stdout)

	// Unassign removes it again.
	unassignResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "unassign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	require.Equal(t, 0, unassignResult.ExitCode, "operatorsurface unassign failed: exit=%d stderr=%s", unassignResult.ExitCode, unassignResult.Stderr)
	afterUnassign, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, afterUnassign.OperatorSurfaces[0].SceneRefs, 0, "expected scene ref removed after unassign, got %+v", afterUnassign.OperatorSurfaces[0])
}

// TestSceneDeleteScrubsReferencingOperatorSurface proves "scene delete"
// against a scene an operator surface is assigned to (by SceneRef and by
// LayerRef) succeeds and unassigns that surface rather than being rejected
// with GOLC_OPERATORSURFACE_DANGLING_REFERENCE.
func TestSceneDeleteScrubsReferencingOperatorSurface(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface assign --scene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--layer", "Opener:color_theme", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface assign --layer failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "delete", "Opener", "--show", showPath,
	}})
	require.Equal(t, 0, deleteResult.ExitCode, "scene delete (referenced by operator surface) failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, state.Scenes, 0, "expected the scene to be gone after delete, got %+v", state.Scenes)
	require.Len(t, state.OperatorSurfaces, 1, "expected the operator surface to survive, got %+v", state.OperatorSurfaces)
	surface := state.OperatorSurfaces[0]
	require.Len(t, surface.SceneRefs, 0, "expected SceneRefs scrubbed after the scene delete, got %+v", surface.SceneRefs)
	require.Len(t, surface.LayerRefs, 0, "expected LayerRefs scrubbed after the scene delete, got %+v", surface.LayerRefs)
}

func TestOperatorSurfaceAssignUnknownSceneRejected(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Nonexistent Scene", "--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_OPERATORSURFACE_SCENE_NOT_FOUND", "expected GOLC_OPERATORSURFACE_SCENE_NOT_FOUND for an unknown scene, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestOperatorSurfaceAssignUnknownSurfaceRejected(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Nonexistent Surface", "--scene", "Opener", "--show", showPath,
	}})
	require.NotEqual(t, 0, result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_OPERATORSURFACE_NOT_FOUND", "expected GOLC_OPERATORSURFACE_NOT_FOUND for an unknown surface, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
}

func TestOperatorSurfaceAssignLayerAndMasterAndSafety(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--layer", "Opener:color_theme", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface assign --layer failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--master", "grand", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface assign --master grand failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--safety", "revoke_automation", "--show", showPath,
	}}); result.ExitCode != 0 {
		require.Equal(t, 0, result.ExitCode, "operatorsurface assign --safety failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	surface := state.OperatorSurfaces[0]
	require.Len(t, surface.LayerRefs, 1, "expected one layer ref, got %+v", surface.LayerRefs)
	require.Len(t, surface.MasterRefs, 1)
	require.Equal(t, operatorsurface.GrandMaster, surface.MasterRefs[0].Kind, "expected one grand master ref, got %+v", surface.MasterRefs)
	require.Len(t, surface.SafetyRefs, 1)
	require.Equal(t, operatorsurface.RevokeAutomation, surface.SafetyRefs[0], "expected one revoke_automation safety ref, got %+v", surface.SafetyRefs)
}

// TestOperatorSurfaceRemove proves the "operatorsurface remove" route
// (06-07-PLAN.md Task 1, T-06-20) deletes a named surface and every
// assignment/MIDI mapping it owned, and rejects removing an unknown
// surface rather than silently no-op-ing.
func TestOperatorSurfaceRemove(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	removeResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "remove", "--surface", "Front of House", "--show", showPath,
	}})
	require.Equal(t, 0, removeResult.ExitCode, "operatorsurface remove failed: exit=%d stderr=%s", removeResult.ExitCode, removeResult.Stderr)
	require.Contains(t, string(removeResult.Stdout), "GOLC_OPERATORSURFACE_REMOVED", "expected GOLC_OPERATORSURFACE_REMOVED in stdout, got %s", removeResult.Stdout)

	state, err := show.Load(root, showPath)
	require.NoError(t, err)
	require.Len(t, state.OperatorSurfaces, 0, "expected zero surfaces after remove, got %+v", state.OperatorSurfaces)

	unknownResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "remove", "--surface", "Nonexistent Surface", "--show", showPath,
	}})
	require.NotEqual(t, 0, unknownResult.ExitCode)
	require.Contains(t, string(unknownResult.Stderr), "GOLC_OPERATORSURFACE_NOT_FOUND", "expected GOLC_OPERATORSURFACE_NOT_FOUND for an unknown surface, got exit=%d stderr=%s", unknownResult.ExitCode, unknownResult.Stderr)
}

func TestOperatorSurfaceAuthorizeRejectsUnassignedControl(t *testing.T) {
	surface, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err)

	unassignedControl := operatorsurface.SafetyControlRef(operatorsurface.Blackout)
	if err := command.Authorize(surface, unassignedControl); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_LOCKED") {
		require.ErrorContains(t, err, "GOLC_OPERATORSURFACE_LOCKED", "expected GOLC_OPERATORSURFACE_LOCKED for a control not assigned to the surface, got %v", err)
	}

	assignedSurface := operatorsurface.AssignSafety(surface, operatorsurface.Blackout)
	if err := command.Authorize(assignedSurface, unassignedControl); err != nil {
		require.NoError(t, err)
	}
}
