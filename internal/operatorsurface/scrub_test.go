// scrub_test.go proves ScrubSceneReferences removes exactly the targeted
// scene ID from a surface's SceneRefs and any LayerRef naming it, leaves a
// surface referencing a different scene untouched, and handles multiple
// surfaces independently.
package operatorsurface_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
)

func TestScrubSceneReferencesRemovesMatchingSceneAndLayerRefs(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())

	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	s = operatorsurface.AssignScene(s, sceneID)
	s = operatorsurface.AssignLayer(s, operatorsurface.LayerRef{SceneID: sceneID, Kind: scene.ColorTheme})

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{s}, sceneID)
	require.Empty(t, scrubbed[0].SceneRefs, "expected SceneRefs scrubbed")
	require.Empty(t, scrubbed[0].LayerRefs, "expected LayerRefs naming the scene scrubbed")
}

func TestScrubSceneReferencesLeavesOtherReferencesUntouched(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())
	otherSceneID := uuid.Must(uuid.NewV7())

	s, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface")
	s = operatorsurface.AssignScene(s, otherSceneID)
	s = operatorsurface.AssignLayer(s, operatorsurface.LayerRef{SceneID: otherSceneID, Kind: scene.Chase})

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{s}, sceneID)
	require.Equal(t, s.SceneRefs, scrubbed[0].SceneRefs, "expected SceneRefs untouched")
	require.Equal(t, s.LayerRefs, scrubbed[0].LayerRefs, "expected LayerRefs untouched")
}

func TestScrubSceneReferencesHandlesMultipleSurfacesIndependently(t *testing.T) {
	sceneID := uuid.Must(uuid.NewV7())

	referencing, err := operatorsurface.NewSurface("Front of House")
	require.NoError(t, err, "NewSurface (referencing)")
	referencing = operatorsurface.AssignScene(referencing, sceneID)

	untouched, err := operatorsurface.NewSurface("Backstage")
	require.NoError(t, err, "NewSurface (untouched)")

	scrubbed := operatorsurface.ScrubSceneReferences([]operatorsurface.Surface{referencing, untouched}, sceneID)
	require.Empty(t, scrubbed[0].SceneRefs, "expected the referencing surface's SceneRefs scrubbed")
	// UnassignScene (reused by ScrubSceneReferences) always rebuilds SceneRefs
	// via a fresh non-nil slice even when nothing is removed, so compare
	// semantic content (ID/Name/membership counts) rather than the whole
	// struct -- a nil-vs-empty-slice difference here is not a real change.
	require.Equal(t, untouched.ID, scrubbed[1].ID, "expected the untouched surface's identity unchanged")
	require.Equal(t, untouched.Name, scrubbed[1].Name, "expected the untouched surface's identity unchanged")
	require.Empty(t, scrubbed[1].SceneRefs, "expected the untouched surface to still have no scene refs")
	require.Empty(t, scrubbed[1].LayerRefs, "expected the untouched surface to still have no layer refs")
}
