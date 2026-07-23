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
)

func TestOperatorSurfaceCreateAndList(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")

	createResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "create", "Front of House", "--show", showPath,
	}})
	if createResult.ExitCode != 0 {
		t.Fatalf("operatorsurface create failed: exit=%d stderr=%s", createResult.ExitCode, createResult.Stderr)
	}
	if !strings.Contains(string(createResult.Stdout), "GOLC_OPERATORSURFACE_CREATED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_CREATED in stdout, got %s", createResult.Stdout)
	}

	listResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "list", "--show", showPath,
	}})
	if listResult.ExitCode != 0 {
		t.Fatalf("operatorsurface list failed: exit=%d stderr=%s", listResult.ExitCode, listResult.Stderr)
	}
	if !strings.Contains(string(listResult.Stdout), "Front of House") {
		t.Fatalf("expected Front of House in list output, got %s", listResult.Stdout)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.OperatorSurfaces) != 1 || state.OperatorSurfaces[0].Name != "Front of House" {
		t.Fatalf("expected exactly one persisted surface, got %+v", state.OperatorSurfaces)
	}
}

// seedOperatorSurfaceShow builds a ShowState carrying one scene (with a
// ColorTheme layer), one group, and one operator surface, saving it at
// showPath so assign/unassign/show tests have real items to reference.
func seedOperatorSurfaceShow(t *testing.T, root, showPath string) {
	t.Helper()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}

	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Opener", "--bars", "4", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("scene create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"pool", "create", "Wash Pool", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "create", "Front of House", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("operatorsurface create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestOperatorSurfaceAssignSceneIdempotentAndShowReflectsIt(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	first := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	if first.ExitCode != 0 {
		t.Fatalf("operatorsurface assign (first) failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)
	}

	// A second identical assign is idempotent -- not rejected, and does not
	// duplicate the assignment.
	second := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	if second.ExitCode != 0 {
		t.Fatalf("operatorsurface assign (idempotent repeat) failed: exit=%d stderr=%s", second.ExitCode, second.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.OperatorSurfaces) != 1 || len(state.OperatorSurfaces[0].SceneRefs) != 1 {
		t.Fatalf("expected exactly one scene ref after an idempotent re-assign, got %+v", state.OperatorSurfaces)
	}

	showResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "show", "--surface", "Front of House", "--show", showPath,
	}})
	if showResult.ExitCode != 0 {
		t.Fatalf("operatorsurface show failed: exit=%d stderr=%s", showResult.ExitCode, showResult.Stderr)
	}
	if !strings.Contains(string(showResult.Stdout), "scenes: 1") {
		t.Fatalf("expected operatorsurface show to reflect the assignment, got %s", showResult.Stdout)
	}

	// Unassign removes it again.
	unassignResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "unassign", "--surface", "Front of House", "--scene", "Opener", "--show", showPath,
	}})
	if unassignResult.ExitCode != 0 {
		t.Fatalf("operatorsurface unassign failed: exit=%d stderr=%s", unassignResult.ExitCode, unassignResult.Stderr)
	}
	afterUnassign, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after unassign: %v", err)
	}
	if len(afterUnassign.OperatorSurfaces[0].SceneRefs) != 0 {
		t.Fatalf("expected scene ref removed after unassign, got %+v", afterUnassign.OperatorSurfaces[0])
	}
}

func TestOperatorSurfaceAssignUnknownSceneRejected(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--scene", "Nonexistent Scene", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_OPERATORSURFACE_SCENE_NOT_FOUND") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_SCENE_NOT_FOUND for an unknown scene, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestOperatorSurfaceAssignUnknownSurfaceRejected(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Nonexistent Surface", "--scene", "Opener", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_OPERATORSURFACE_NOT_FOUND") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_NOT_FOUND for an unknown surface, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestOperatorSurfaceAssignLayerAndMasterAndSafety(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--layer", "Opener:color_theme", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("operatorsurface assign --layer failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--master", "grand", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("operatorsurface assign --master grand failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "assign", "--surface", "Front of House", "--safety", "revoke_automation", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("operatorsurface assign --safety failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	surface := state.OperatorSurfaces[0]
	if len(surface.LayerRefs) != 1 {
		t.Fatalf("expected one layer ref, got %+v", surface.LayerRefs)
	}
	if len(surface.MasterRefs) != 1 || surface.MasterRefs[0].Kind != operatorsurface.GrandMaster {
		t.Fatalf("expected one grand master ref, got %+v", surface.MasterRefs)
	}
	if len(surface.SafetyRefs) != 1 || surface.SafetyRefs[0] != operatorsurface.RevokeAutomation {
		t.Fatalf("expected one revoke_automation safety ref, got %+v", surface.SafetyRefs)
	}
}

// TestOperatorSurfaceRemove proves the "operatorsurface remove" route
// (06-07-PLAN.md Task 1, T-06-20) deletes a named surface and every
// assignment/MIDI mapping it owned, and rejects removing an unknown
// surface rather than silently no-op-ing.
func TestOperatorSurfaceRemove(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.golc")
	seedOperatorSurfaceShow(t, root, showPath)

	removeResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "remove", "--surface", "Front of House", "--show", showPath,
	}})
	if removeResult.ExitCode != 0 {
		t.Fatalf("operatorsurface remove failed: exit=%d stderr=%s", removeResult.ExitCode, removeResult.Stderr)
	}
	if !strings.Contains(string(removeResult.Stdout), "GOLC_OPERATORSURFACE_REMOVED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_REMOVED in stdout, got %s", removeResult.Stdout)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(state.OperatorSurfaces) != 0 {
		t.Fatalf("expected zero surfaces after remove, got %+v", state.OperatorSurfaces)
	}

	unknownResult := registry.Execute(command.Request{Root: root, Args: []string{
		"operatorsurface", "remove", "--surface", "Nonexistent Surface", "--show", showPath,
	}})
	if unknownResult.ExitCode == 0 || !strings.Contains(string(unknownResult.Stderr), "GOLC_OPERATORSURFACE_NOT_FOUND") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_NOT_FOUND for an unknown surface, got exit=%d stderr=%s", unknownResult.ExitCode, unknownResult.Stderr)
	}
}

func TestOperatorSurfaceAuthorizeRejectsUnassignedControl(t *testing.T) {
	surface, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}

	unassignedControl := operatorsurface.SafetyControlRef(operatorsurface.Blackout)
	if err := command.Authorize(surface, unassignedControl); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_LOCKED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_LOCKED for a control not assigned to the surface, got %v", err)
	}

	assignedSurface := operatorsurface.AssignSafety(surface, operatorsurface.Blackout)
	if err := command.Authorize(assignedSurface, unassignedControl); err != nil {
		t.Fatalf("expected Authorize to accept an assigned control, got %v", err)
	}
}
