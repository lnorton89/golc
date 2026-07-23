// validate_test.go proves operatorsurface.Validate's duplicate-name and
// referential-integrity contract (06-01-PLAN.md Task 2): two surfaces
// sharing a Name are rejected, a SceneRef/LayerRef/GroupMaster ref
// pointing at a scene/layer/group that does not exist is rejected, and a
// surface whose refs all resolve against the real owning collections
// passes.
package operatorsurface_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
)

func TestSurfaceValidateUniqueNamesRejected(t *testing.T) {
	surfaces := []operatorsurface.Surface{{Name: "Front of House"}, {Name: "Front of House"}}
	if err := operatorsurface.Validate(surfaces, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DUPLICATE_NAME for duplicate surface names, got %v", err)
	}
}

func TestSurfaceValidateDanglingSceneReferenceRejected(t *testing.T) {
	surfaces := []operatorsurface.Surface{{
		Name:      "Front of House",
		SceneRefs: []uuid.UUID{mustNewUUID(t)},
	}}
	if err := operatorsurface.Validate(surfaces, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DANGLING_REFERENCE for a dangling scene ref, got %v", err)
	}
}

func TestSurfaceValidateDanglingLayerReferenceRejected(t *testing.T) {
	sc, err := scene.NewScene("Opener", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}

	// A layer ref against a real scene but the wrong kind (using a bogus
	// LayerKind value that no real scene ever carries) must fail.
	surfaces := []operatorsurface.Surface{{
		Name: "Front of House",
		LayerRefs: []operatorsurface.LayerRef{
			{SceneID: sc.ID, Kind: scene.LayerKind("not_a_real_kind")},
		},
	}}
	if err := operatorsurface.Validate(surfaces, []scene.Scene{sc}, nil); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DANGLING_REFERENCE for an unknown layer kind, got %v", err)
	}

	// A layer ref against a scene ID that does not exist at all must fail.
	danglingSceneSurfaces := []operatorsurface.Surface{{
		Name: "Monitor Desk",
		LayerRefs: []operatorsurface.LayerRef{
			{SceneID: mustNewUUID(t), Kind: scene.ColorTheme},
		},
	}}
	if err := operatorsurface.Validate(danglingSceneSurfaces, []scene.Scene{sc}, nil); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DANGLING_REFERENCE for a layer ref against a nonexistent scene, got %v", err)
	}

	// A layer ref that resolves against a real scene and a real layer kind
	// passes.
	validSurfaces := []operatorsurface.Surface{{
		Name: "Valid Surface",
		LayerRefs: []operatorsurface.LayerRef{
			{SceneID: sc.ID, Kind: scene.ColorTheme},
		},
	}}
	if err := operatorsurface.Validate(validSurfaces, []scene.Scene{sc}, nil); err != nil {
		t.Fatalf("expected a layer ref resolving against a real scene/kind to be valid, got %v", err)
	}
}

func TestSurfaceValidateDanglingGroupMasterReferenceRejected(t *testing.T) {
	groupID := mustNewUUID(t)
	group := pool.Group{ID: groupID, Name: "Front Wash"}

	// A GroupMaster ref against a real group passes.
	validSurfaces := []operatorsurface.Surface{{
		Name: "Front of House",
		MasterRefs: []operatorsurface.MasterRef{
			{Kind: operatorsurface.GroupMaster, GroupID: groupID},
		},
	}}
	if err := operatorsurface.Validate(validSurfaces, nil, []pool.Group{group}); err != nil {
		t.Fatalf("expected a group master ref resolving against a real group to be valid, got %v", err)
	}

	// A GroupMaster ref against a group that does not exist fails.
	danglingSurfaces := []operatorsurface.Surface{{
		Name: "Monitor Desk",
		MasterRefs: []operatorsurface.MasterRef{
			{Kind: operatorsurface.GroupMaster, GroupID: mustNewUUID(t)},
		},
	}}
	if err := operatorsurface.Validate(danglingSurfaces, nil, []pool.Group{group}); err == nil ||
		!strings.Contains(err.Error(), "GOLC_OPERATORSURFACE_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_DANGLING_REFERENCE for a dangling group master ref, got %v", err)
	}

	// A GrandMaster ref never dangles -- it needs no group at all.
	grandMasterSurfaces := []operatorsurface.Surface{{
		Name:       "Grand Master Only",
		MasterRefs: []operatorsurface.MasterRef{{Kind: operatorsurface.GrandMaster}},
	}}
	if err := operatorsurface.Validate(grandMasterSurfaces, nil, nil); err != nil {
		t.Fatalf("expected a grand master ref to never dangle, got %v", err)
	}
}

func TestSurfaceValidateSafetyRefNeverDangles(t *testing.T) {
	surfaces := []operatorsurface.Surface{{
		Name:       "Front of House",
		SafetyRefs: []operatorsurface.SafetyControl{operatorsurface.Blackout, operatorsurface.RevokeAutomation},
	}}
	if err := operatorsurface.Validate(surfaces, nil, nil); err != nil {
		t.Fatalf("expected safety refs to never dangle, got %v", err)
	}
}
