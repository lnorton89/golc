// scrub_test.go proves ScrubDanglingSelections cleans every layer across
// every scene, is a no-op when nothing is dangling, and handles multiple
// scenes independently. It also proves ScrubLayerRef resets exactly the
// targeted (Kind, ID) layer to its default, un-refed state and leaves
// every other layer -- same scene or a different one -- untouched.
package scene_test

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
)

func TestScrubDanglingSelectionsCleansEveryLayer(t *testing.T) {
	danglingPoolID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Scene 1", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	for i := range s.Layers {
		s.Layers[i].Selection = programming.Selection{PoolIDs: []uuid.UUID{danglingPoolID}}
	}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{s}, nil, nil, nil)
	if len(scrubbed) != 1 {
		t.Fatalf("expected exactly one scene, got %d", len(scrubbed))
	}
	for i, layer := range scrubbed[0].Layers {
		if len(layer.Selection.PoolIDs) != 0 {
			t.Fatalf("expected layer %d's dangling PoolIDs to be scrubbed, got %v", i, layer.Selection.PoolIDs)
		}
		if layer.Kind != s.Layers[i].Kind || layer.Enabled != s.Layers[i].Enabled || layer.Ref != s.Layers[i].Ref {
			t.Fatalf("expected Kind/Enabled/Ref to pass through unchanged for layer %d, got %+v want %+v", i, layer, s.Layers[i])
		}
	}
}

func TestScrubDanglingSelectionsNoOpWhenNothingDangling(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	s, err := scene.NewScene("Scene 1", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	s.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{p.ID}}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{s}, []pool.Pool{p}, nil, nil)
	if !reflect.DeepEqual(scrubbed[0].Layers, s.Layers) {
		t.Fatalf("expected layers to pass through unchanged when nothing is dangling, got %+v want %+v", scrubbed[0].Layers, s.Layers)
	}
}

func TestScrubDanglingSelectionsHandlesMultipleScenesIndependently(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	danglingPoolID := uuid.Must(uuid.NewV7())

	valid, err := scene.NewScene("Valid Scene", 4)
	if err != nil {
		t.Fatalf("NewScene (valid): %v", err)
	}
	valid.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{p.ID}}

	dangling, err := scene.NewScene("Dangling Scene", 4)
	if err != nil {
		t.Fatalf("NewScene (dangling): %v", err)
	}
	dangling.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{danglingPoolID}}

	scrubbed := scene.ScrubDanglingSelections([]scene.Scene{valid, dangling}, []pool.Pool{p}, nil, nil)
	if len(scrubbed[0].Layers[0].Selection.PoolIDs) != 1 {
		t.Fatalf("expected the valid scene's selection to survive untouched, got %v", scrubbed[0].Layers[0].Selection.PoolIDs)
	}
	if len(scrubbed[1].Layers[0].Selection.PoolIDs) != 0 {
		t.Fatalf("expected the dangling scene's selection to be scrubbed, got %v", scrubbed[1].Layers[0].Selection.PoolIDs)
	}
}

func TestScrubLayerRefResetsMatchingLayer(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}

	scrubbed := scene.ScrubLayerRef([]scene.Scene{s}, scene.ColorTheme, themeID)
	layer, ok := scrubbed[0].LayerByKind(scene.ColorTheme)
	if !ok {
		t.Fatalf("expected a color-theme layer, got %+v", scrubbed[0].Layers)
	}
	if layer.Enabled {
		t.Fatal("expected the matching layer to be disabled after scrub")
	}
	var zero uuid.UUID
	if layer.Ref != zero {
		t.Fatalf("expected the matching layer's Ref cleared to zero, got %s", layer.Ref)
	}
}

func TestScrubLayerRefNoOpWhenKindOrRefDontMatch(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())

	s, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}

	// A different Kind carrying the same ID is untouched.
	byWrongKind := scene.ScrubLayerRef([]scene.Scene{s}, scene.Chase, themeID)
	if !reflect.DeepEqual(byWrongKind[0].Layers, s.Layers) {
		t.Fatalf("expected layers untouched for a non-matching Kind, got %+v want %+v", byWrongKind[0].Layers, s.Layers)
	}

	// The right Kind but a different ID is untouched.
	byWrongID := scene.ScrubLayerRef([]scene.Scene{s}, scene.ColorTheme, otherID)
	if !reflect.DeepEqual(byWrongID[0].Layers, s.Layers) {
		t.Fatalf("expected layers untouched for a non-matching Ref, got %+v want %+v", byWrongID[0].Layers, s.Layers)
	}
}

func TestScrubLayerRefHandlesMultipleScenesIndependently(t *testing.T) {
	themeID := uuid.Must(uuid.NewV7())

	referencing, err := scene.NewScene("Referencing", 4)
	if err != nil {
		t.Fatalf("NewScene (referencing): %v", err)
	}
	referencing, err = scene.SetLayer(referencing, scene.Layer{Kind: scene.ColorTheme, Enabled: true, Ref: themeID})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}

	untouched, err := scene.NewScene("Untouched", 4)
	if err != nil {
		t.Fatalf("NewScene (untouched): %v", err)
	}

	scrubbed := scene.ScrubLayerRef([]scene.Scene{referencing, untouched}, scene.ColorTheme, themeID)
	layer, ok := scrubbed[0].LayerByKind(scene.ColorTheme)
	if !ok || layer.Enabled {
		t.Fatalf("expected the referencing scene's layer reset, got %+v", layer)
	}
	if !reflect.DeepEqual(scrubbed[1].Layers, untouched.Layers) {
		t.Fatalf("expected the untouched scene's layers to pass through unchanged, got %+v want %+v", scrubbed[1].Layers, untouched.Layers)
	}
}
