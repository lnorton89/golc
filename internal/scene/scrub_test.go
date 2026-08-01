// scrub_test.go proves ScrubDanglingSelections cleans every layer across
// every scene, is a no-op when nothing is dangling, and handles multiple
// scenes independently. It also proves ScrubLayerRef resets exactly the
// targeted (Kind, ID) layer to its default, un-refed state and leaves
// every other layer -- same scene or a different one -- untouched.
package scene_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
)

func TestScrubDanglingSelectionsCleansEveryLayer(t *testing.T) {
	danglingPoolID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Scene 1", 4)
	require.NoError(t, err, "NewScene")
	for i := range s.Layers {
		s.Layers[i].Selection = programming.Selection{PoolIDs: []uuid.UUID{danglingPoolID}}
	}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{s}, nil, nil, nil)
	require.Len(t, scrubbed, 1)
	for i, layer := range scrubbed[0].Layers {
		require.Empty(t, layer.Selection.PoolIDs, "expected layer %d's dangling PoolIDs to be scrubbed", i)
		require.Equal(t, s.Layers[i].Kind, layer.Kind, "expected Kind/Enabled/Ref to pass through unchanged for layer %d", i)
		require.Equal(t, s.Layers[i].Enabled, layer.Enabled, "expected Kind/Enabled/Ref to pass through unchanged for layer %d", i)
		require.Equal(t, s.Layers[i].Ref, layer.Ref, "expected Kind/Enabled/Ref to pass through unchanged for layer %d", i)
	}
}

func TestScrubDanglingSelectionsNoOpWhenNothingDangling(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")

	s, err := scene.NewScene("Scene 1", 4)
	require.NoError(t, err, "NewScene")
	s.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{p.ID}}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{s}, []pool.Pool{p}, nil, nil)
	require.Equal(t, s.Layers, scrubbed[0].Layers, "expected layers to pass through unchanged when nothing is dangling")
}

func TestScrubDanglingSelectionsHandlesMultipleScenesIndependently(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	danglingPoolID := uuid.Must(uuid.NewV7())

	valid, err := scene.NewScene("Valid Scene", 4)
	require.NoError(t, err, "NewScene (valid)")
	valid.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{p.ID}}

	dangling, err := scene.NewScene("Dangling Scene", 4)
	require.NoError(t, err, "NewScene (dangling)")
	dangling.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{danglingPoolID}}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{valid, dangling}, []pool.Pool{p}, nil, nil)
	require.Len(t, scrubbed[0].Layers[0].Selection.PoolIDs, 1, "expected the valid scene's selection to survive untouched")
	require.Empty(t, scrubbed[1].Layers[0].Selection.PoolIDs, "expected the dangling scene's selection to be scrubbed")
}

func TestScrubLayerRefResetsMatchingLayer(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	require.NoError(t, err, "SetLayer")

	scrubbed := scene.ScrubLayerRef([]scene.Scene{s}, scene.ColorTheme, themeID)
	layer, ok := scrubbed[0].LayerByKind(scene.ColorTheme)
	require.True(t, ok, "expected a color-theme layer, got %+v", scrubbed[0].Layers)
	require.False(t, layer.Enabled, "expected the matching layer to be disabled after scrub")
	var zero uuid.UUID
	require.Equal(t, zero, layer.Ref, "expected the matching layer's Ref cleared to zero")
}

func TestScrubLayerRefNoOpWhenKindOrRefDontMatch(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	require.NoError(t, err, "SetLayer")

	// A different Kind carrying the same ID is untouched.
	byWrongKind := scene.ScrubLayerRef([]scene.Scene{s}, scene.Chase, themeID)
	require.Equal(t, s.Layers, byWrongKind[0].Layers, "expected layers untouched for a non-matching Kind")

	// The right Kind but a different ID is untouched.
	byWrongID := scene.ScrubLayerRef([]scene.Scene{s}, scene.ColorTheme, otherID)
	require.Equal(t, s.Layers, byWrongID[0].Layers, "expected layers untouched for a non-matching Ref")
}

func TestScrubLayerRefHandlesMultipleScenesIndependently(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())

	referencing, err := scene.NewScene("Referencing", 4)
	require.NoError(t, err, "NewScene (referencing)")
	referencing, err = scene.SetLayer(referencing, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	require.NoError(t, err, "SetLayer")

	untouched, err := scene.NewScene("Untouched", 4)
	require.NoError(t, err, "NewScene (untouched)")

	scrubbed := scene.ScrubLayerRef([]scene.Scene{referencing, untouched}, scene.ColorTheme, themeID)
	layer, ok := scrubbed[0].LayerByKind(scene.ColorTheme)
	require.True(t, ok, "expected the referencing scene's layer reset")
	require.False(t, layer.Enabled, "expected the referencing scene's layer reset, got %+v", layer)
	require.Equal(t, untouched.Layers, scrubbed[1].Layers, "expected the untouched scene's layers to pass through unchanged")
}
