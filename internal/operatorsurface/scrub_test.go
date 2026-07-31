// scrub_test.go proves ScrubSceneReferences removes exactly the targeted
// scene ID from a surface's SceneRefs and any LayerRef naming it, leaves a
// surface referencing a different scene untouched, and handles multiple
// surfaces independently.
package operatorsurface_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
)

func TestScrubSceneReferencesRemovesMatchingSceneAndLayerRefs(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())

	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	s = operatorsurface.AssignScene(s, sceneID)
	s = operatorsurface.AssignLayer(s, operatorsurface.LayerRef{SceneID: sceneID, Kind: scene.ColorTheme})

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{s}, sceneID)
	if len(scrubbed[0].SceneRefs) != 0 {
		t.Fatalf("expected SceneRefs scrubbed, got %v", scrubbed[0].SceneRefs)
	}
	if len(scrubbed[0].LayerRefs) != 0 {
		t.Fatalf("expected LayerRefs naming the scene scrubbed, got %v", scrubbed[0].LayerRefs)
	}
}

func TestScrubSceneReferencesLeavesOtherReferencesUntouched(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())
	otherSceneID := uuid.Must(uuid.NewV7())

	s, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface: %v", err)
	}
	s = operatorsurface.AssignScene(s, otherSceneID)
	s = operatorsurface.AssignLayer(s, operatorsurface.LayerRef{SceneID: otherSceneID, Kind: scene.Chase})

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{s}, sceneID)
	if !reflect.DeepEqual(scrubbed[0].SceneRefs, s.SceneRefs) {
		t.Fatalf("expected SceneRefs untouched, got %v want %v", scrubbed[0].SceneRefs, s.SceneRefs)
	}
	if !reflect.DeepEqual(scrubbed[0].LayerRefs, s.LayerRefs) {
		t.Fatalf("expected LayerRefs untouched, got %v want %v", scrubbed[0].LayerRefs, s.LayerRefs)
	}
}

func TestScrubSceneReferencesHandlesMultipleSurfacesIndependently(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())

	referencing, err := operatorsurface.NewSurface("Front of House")
	if err != nil {
		t.Fatalf("NewSurface (referencing): %v", err)
	}
	referencing = operatorsurface.AssignScene(referencing, sceneID)

	untouched, err := operatorsurface.NewSurface("Backstage")
	if err != nil {
		t.Fatalf("NewSurface (untouched): %v", err)
	}

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{referencing, untouched}, sceneID)
	if len(scrubbed[0].SceneRefs) != 0 {
		t.Fatalf("expected the referencing surface's SceneRefs scrubbed, got %v", scrubbed[0].SceneRefs)
	}
	// UnassignScene (reused by ScrubSceneReferences) always rebuilds SceneRefs
	// via a fresh non-nil slice even when nothing is removed, so compare
	// semantic content (ID/Name/membership counts) rather than the whole
	// struct -- a nil-vs-empty-slice difference here is not a real change.
	if scrubbed[1].ID != untouched.ID || scrubbed[1].Name != untouched.Name {
		t.Fatalf("expected the untouched surface's identity unchanged, got %+v want %+v", scrubbed[1], untouched)
	}
	if len(scrubbed[1].SceneRefs) != 0 || len(scrubbed[1].LayerRefs) != 0 {
		t.Fatalf("expected the untouched surface to still have no scene/layer refs, got %+v", scrubbed[1])
	}
}
