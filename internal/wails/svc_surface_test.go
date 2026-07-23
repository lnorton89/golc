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
	"strings"
	"testing"

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

	if result := svc.CreateSurface("Front of House"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	summaries, err := svc.ListSurfaces()
	if err != nil {
		t.Fatalf("ListSurfaces: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "Front of House" {
		t.Fatalf("expected exactly one surface named Front of House, got %+v", summaries)
	}
	if summaries[0].AssignedCount != 0 || summaries[0].MidiMappingCount != 0 {
		t.Fatalf("expected a freshly created surface to have zero assignments, got %+v", summaries[0])
	}

	detail, err := svc.ShowSurface("Front of House")
	if err != nil {
		t.Fatalf("ShowSurface: %v", err)
	}
	// A fresh show has no scenes/groups yet -- only the fixed grand master
	// and the three safety controls are assignable.
	if len(detail.Controls) != 4 {
		t.Fatalf("expected 4 assignable controls (grand master + 3 safety) on an empty show, got %d: %+v", len(detail.Controls), detail.Controls)
	}
	for _, c := range detail.Controls {
		if c.Assigned {
			t.Fatalf("expected every control to start unassigned, got %+v", c)
		}
	}

	blackout := ControlRefInput{Kind: "safety", Safety: "blackout"}
	if result := svc.AssignItem("Front of House", blackout); result.ExitCode != 0 {
		t.Fatalf("AssignItem failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	// A repeated assign is idempotent (PLAY-03 idempotency edge).
	if result := svc.AssignItem("Front of House", blackout); result.ExitCode != 0 {
		t.Fatalf("AssignItem (idempotent repeat) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	detail, err = svc.ShowSurface("Front of House")
	if err != nil {
		t.Fatalf("ShowSurface after assign: %v", err)
	}
	assignedCount := 0
	for _, c := range detail.Controls {
		if c.Assigned {
			assignedCount++
			if c.Kind != "safety" || c.Safety != "blackout" {
				t.Fatalf("expected only the blackout safety control to be assigned, got %+v", c)
			}
		}
	}
	if assignedCount != 1 {
		t.Fatalf("expected exactly one assigned control after an idempotent re-assign, got %d", assignedCount)
	}

	summaries, err = svc.ListSurfaces()
	if err != nil {
		t.Fatalf("ListSurfaces after assign: %v", err)
	}
	if summaries[0].SafetyCount != 1 {
		t.Fatalf("expected SafetyCount=1 after assigning blackout, got %+v", summaries[0])
	}

	if result := svc.UnassignItem("Front of House", blackout); result.ExitCode != 0 {
		t.Fatalf("UnassignItem failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	detail, err = svc.ShowSurface("Front of House")
	if err != nil {
		t.Fatalf("ShowSurface after unassign: %v", err)
	}
	for _, c := range detail.Controls {
		if c.Assigned {
			t.Fatalf("expected no assigned controls after unassign, got %+v", c)
		}
	}

	if result := svc.RemoveSurface("Front of House"); result.ExitCode != 0 {
		t.Fatalf("RemoveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	summaries, err = svc.ListSurfaces()
	if err != nil {
		t.Fatalf("ListSurfaces after remove: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected zero surfaces after remove, got %+v", summaries)
	}
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

	if result := svc.CreateSurface("Front of House"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	unassigned := ControlRefInput{Kind: "safety", Safety: "blackout"}
	result := svc.AuthorizeControl("Front of House", unassigned)
	if result.ExitCode == 0 || !strings.Contains(result.Stderr, "GOLC_OPERATORSURFACE_LOCKED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_LOCKED for an unassigned control, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	if assignResult := svc.AssignItem("Front of House", unassigned); assignResult.ExitCode != 0 {
		t.Fatalf("AssignItem failed: exit=%d stderr=%s", assignResult.ExitCode, assignResult.Stderr)
	}
	result = svc.AuthorizeControl("Front of House", unassigned)
	if result.ExitCode != 0 {
		t.Fatalf("expected Authorize to accept an assigned control, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestSurfaceServiceAssignSceneAndLayer proves scenes/layers created
// elsewhere in the show (via the ordinary "scene create" CLI route) are
// resolvable AssignItem/ShowSurface selectors, not just the fixed
// grand-master/safety set every show carries.
func TestSurfaceServiceAssignSceneAndLayer(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")

	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	if result := registry.Execute(command.Request{Root: root, Args: []string{
		"scene", "create", "Opener", "--bars", "4", "--show", showPath,
	}}); result.ExitCode != 0 {
		t.Fatalf("scene create failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	svc := NewSurfaceService("", root, showPath)
	if result := svc.CreateSurface("Front of House"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	if result := svc.AssignItem("Front of House", ControlRefInput{Kind: "scene", Scene: "Opener"}); result.ExitCode != 0 {
		t.Fatalf("AssignItem scene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.AssignItem("Front of House", ControlRefInput{Kind: "layer", Scene: "Opener", LayerKind: "color_theme"}); result.ExitCode != 0 {
		t.Fatalf("AssignItem layer failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	detail, err := svc.ShowSurface("Front of House")
	if err != nil {
		t.Fatalf("ShowSurface: %v", err)
	}
	sceneAssigned, layerAssigned := false, false
	for _, c := range detail.Controls {
		if c.Kind == "scene" && c.Scene == "Opener" && c.Assigned {
			sceneAssigned = true
		}
		if c.Kind == "layer" && c.Scene == "Opener" && c.LayerKind == "color_theme" && c.Assigned {
			layerAssigned = true
		}
	}
	if !sceneAssigned || !layerAssigned {
		t.Fatalf("expected both the scene and its color_theme layer to be assigned, got %+v", detail.Controls)
	}
}
