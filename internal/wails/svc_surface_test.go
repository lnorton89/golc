// svc_surface_test.go proves 06-07-PLAN.md Task 1's acceptance criteria:
// create/list/assign/unassign/show/remove round-trip through the real
// "operatorsurface" CLI routes, a repeated assign is idempotent (PLAY-03),
// scenes/layers created elsewhere in the show are resolvable selectors, and
// AuthorizeControl -- the server-side visible-but-locked enforcement point
// (D-04/ASVS V4) -- rejects an operator-mode action against a control not
// currently assigned to the surface (GOLC_OPERATORSURFACE_LOCKED).
package wails

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

// TestSurfaceServiceCreateListAssignShowUnassignRemoveRoundTrip proves the
// full CRUD round-trip: CreateSurface -> ListSurfaces reflects it ->
// AssignItem (idempotent on repeat) -> ShowSurface reflects membership over
// the full control set -> UnassignItem clears it -> RemoveSurface deletes
// the surface entirely.
func TestSurfaceServiceCreateListAssignShowUnassignRemoveRoundTrip(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewSurfaceService("", root, showPath)

	result := svc.CreateSurface("Front of House")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)

	summaries, err := svc.ListSurfaces()
	require.NoError(t, err, "ListSurfaces")
	require.Len(t, summaries, 1, "expected exactly one surface named Front of House, got %+v", summaries)
	require.Equal(t, "Front of House", summaries[0].Name)
	require.Equal(t, 0, summaries[0].AssignedCount, "expected a freshly created surface to have zero assignments, got %+v", summaries[0])
	require.Equal(t, 0, summaries[0].MidiMappingCount, "expected a freshly created surface to have zero assignments, got %+v", summaries[0])

	detail, err := svc.ShowSurface("Front of House")
	require.NoError(t, err, "ShowSurface")
	// A fresh show has no scenes/groups yet -- only the fixed grand master
	// and the three safety controls are assignable.
	require.Len(t, detail.Controls, 4, "expected 4 assignable controls (grand master + 3 safety) on an empty show, got %+v", detail.Controls)
	for _, c := range detail.Controls {
		require.False(t, c.Assigned, "expected every control to start unassigned, got %+v", c)
	}

	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	result = svc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, result.ExitCode, "AssignItem failed: stderr=%s", result.Stderr)
	// A repeated assign is idempotent (PLAY-03 idempotency edge).
	result = svc.AssignItem("Front of House", blackout)
	require.Equal(t, 0, result.ExitCode, "AssignItem (idempotent repeat) failed: stderr=%s", result.Stderr)

	detail, err = svc.ShowSurface("Front of House")
	require.NoError(t, err, "ShowSurface after assign")
	assignedCount := 0
	for _, c := range detail.Controls {
		if c.Assigned {
			assignedCount++
			require.Equal(t, "safety", c.Kind, "expected only the blackout safety control to be assigned, got %+v", c)
			require.Equal(t, "blackout", c.Safety, "expected only the blackout safety control to be assigned, got %+v", c)
		}
	}
	require.Equal(t, 1, assignedCount, "expected exactly one assigned control after an idempotent re-assign")

	summaries, err = svc.ListSurfaces()
	require.NoError(t, err, "ListSurfaces after assign")
	require.Equal(t, 1, summaries[0].SafetyCount, "expected SafetyCount=1 after assigning blackout, got %+v", summaries[0])

	result = svc.UnassignItem("Front of House", blackout)
	require.Equal(t, 0, result.ExitCode, "UnassignItem failed: stderr=%s", result.Stderr)
	detail, err = svc.ShowSurface("Front of House")
	require.NoError(t, err, "ShowSurface after unassign")
	for _, c := range detail.Controls {
		require.False(t, c.Assigned, "expected no assigned controls after unassign, got %+v", c)
	}

	result = svc.RemoveSurface("Front of House")
	require.Equal(t, 0, result.ExitCode, "RemoveSurface failed: stderr=%s", result.Stderr)
	summaries, err = svc.ListSurfaces()
	require.NoError(t, err, "ListSurfaces after remove")
	require.Empty(t, summaries, "expected zero surfaces after remove")
}

// TestSurfaceServiceAuthorizeControlRejectsUnassignedControl proves the
// server-side visible-but-locked enforcement point (D-04/ASVS V4, threat
// T-06-18): an operator-mode action against a control not currently
// assigned to the surface is rejected with GOLC_OPERATORSURFACE_LOCKED,
// never trusted from a frontend-disabled control alone; the same control
// is accepted once assigned.
func TestSurfaceServiceAuthorizeControlRejectsUnassignedControl(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	svc := NewSurfaceService("", root, showPath)

	result := svc.CreateSurface("Front of House")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)

	unassigned := ControlRefInput{Kind: "safety", Safety: "blackout"}
	result = svc.AuthorizeControl("Front of House", unassigned)
	require.NotEqual(t, 0, result.ExitCode, "expected GOLC_OPERATORSURFACE_LOCKED for an unassigned control")
	require.Contains(t, result.Stderr, "GOLC_OPERATORSURFACE_LOCKED")

	assignResult := svc.AssignItem("Front of House", unassigned)
	require.Equal(t, 0, assignResult.ExitCode, "AssignItem failed: stderr=%s", assignResult.Stderr)
	result = svc.AuthorizeControl("Front of House", unassigned)
	require.Equal(t, 0, result.ExitCode, "expected Authorize to accept an assigned control, got stderr=%s", result.Stderr)
}

// TestSurfaceServiceAssignSceneAndLayer proves scenes/layers created
// elsewhere in the show (via the ordinary "scene create" CLI route) are
// resolvable AssignItem/ShowSurface selectors, not just the fixed
// grand-master/safety set every show carries.
func TestSurfaceServiceAssignSceneAndLayer(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")

	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry")
	cmdResult := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Opener", "--bars", "4", "--show", showPath,
	}})
	require.Equal(t, 0, cmdResult.ExitCode, "scene create failed: stderr=%s", cmdResult.Stderr)

	svc := NewSurfaceService("", root, showPath)
	result := svc.CreateSurface("Front of House")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)

	result = svc.AssignItem("Front of House", ControlRefInput{Kind: "scene", Scene: "Opener"})
	require.Equal(t, 0, result.ExitCode, "AssignItem scene failed: stderr=%s", result.Stderr)
	result = svc.AssignItem("Front of House", ControlRefInput{Kind: "layer", Scene: "Opener", LayerKind: "color_theme"})
	require.Equal(t, 0, result.ExitCode, "AssignItem layer failed: stderr=%s", result.Stderr)

	detail, err := svc.ShowSurface("Front of House")
	require.NoError(t, err, "ShowSurface")
	sceneAssigned, layerAssigned := false, false
	for _, c := range detail.Controls {
		if c.Kind == "scene" && c.Scene == "Opener" && c.Assigned {
			sceneAssigned = true
		}
		if c.Kind == "layer" && c.Scene == "Opener" && c.LayerKind == "color_theme" && c.Assigned {
			layerAssigned = true
		}
	}
	require.True(t, sceneAssigned, "expected both the scene and its color_theme layer to be assigned, got %+v", detail.Controls)
	require.True(t, layerAssigned, "expected both the scene and its color_theme layer to be assigned, got %+v", detail.Controls)
}
