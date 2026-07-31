// scrub_test.go proves ScrubDanglingSelections cleans every layer across
// every scene, is a no-op when nothing is dangling, and handles multiple
// scenes independently.
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
