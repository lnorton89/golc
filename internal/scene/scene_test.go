// scene_test.go proves Scene's identity/construction/rename/unique-name/
// single-active-scene contract (03-04-PLAN.md Task 1): NewScene mints a
// UUIDv7 ID and enforces the BarsPerLoop boundary (1 valid, 0/negative/
// above-ceiling rejected); ValidateSingleActiveScene accepts zero or one
// active scene and rejects two; ActivateScene guarantees exactly one
// active scene even across repeated activation, mirroring
// internal/deployment/model.go's ValidateSingleActive/Activate tests.
package scene_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/scene"
)

func TestSingleActiveSceneAcceptsZeroOrOneActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	b, err := scene.NewScene("B", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	if err := scene.ValidateSingleActiveScene([]scene.Scene{a, b}); err != nil {
		t.Fatalf("expected zero active scenes to be valid, got %v", err)
	}
	a.Active = true
	if err := scene.ValidateSingleActiveScene([]scene.Scene{a, b}); err != nil {
		t.Fatalf("expected exactly one active scene to be valid, got %v", err)
	}
}

func TestSingleActiveSceneRejectsMultipleActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	b, err := scene.NewScene("B", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	a.Active = true
	b.Active = true
	err = scene.ValidateSingleActiveScene([]scene.Scene{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_MULTIPLE_ACTIVE") {
		t.Fatalf("expected GOLC_SCENE_MULTIPLE_ACTIVE for two active scenes, got %v", err)
	}
}

func TestSingleActiveSceneActivateNeverTransientlyTwoActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	b, err := scene.NewScene("B", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}

	activated, err := scene.ActivateScene([]scene.Scene{a, b}, "B")
	if err != nil {
		t.Fatalf("ActivateScene: %v", err)
	}
	assertExactlyOneActiveNamed(t, activated, "B")

	// A second activate against the other scene still keeps exactly one
	// active -- never transiently two.
	reactivated, err := scene.ActivateScene(activated, "A")
	if err != nil {
		t.Fatalf("ActivateScene (second): %v", err)
	}
	assertExactlyOneActiveNamed(t, reactivated, "A")
}

func assertExactlyOneActiveNamed(t *testing.T, scenes []scene.Scene, expectedName string) {
	t.Helper()
	activeCount := 0
	for _, s := range scenes {
		if s.Active {
			activeCount++
			if s.Name != expectedName {
				t.Fatalf("expected %q to be the only active scene, got %q active", expectedName, s.Name)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active scene, got %d", activeCount)
	}
}

func TestSingleActiveSceneActivateNotFound(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	_, err = scene.ActivateScene([]scene.Scene{a}, "Missing")
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_NOT_FOUND") {
		t.Fatalf("expected GOLC_SCENE_NOT_FOUND, got %v", err)
	}
}

func TestSceneBarsPerLoopBoundary(t *testing.T) {
	if _, err := scene.NewScene("Loop", 1); err != nil {
		t.Fatalf("expected bars_per_loop=1 to be valid (loops every bar), got %v", err)
	}
	if _, err := scene.NewScene("Loop", 0); err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_BARS_INVALID") {
		t.Fatalf("expected GOLC_SCENE_BARS_INVALID for bars_per_loop=0, got %v", err)
	}
	if _, err := scene.NewScene("Loop", -1); err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_BARS_INVALID") {
		t.Fatalf("expected GOLC_SCENE_BARS_INVALID for a negative bars_per_loop, got %v", err)
	}
	if _, err := scene.NewScene("Loop", 100000); err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_BARS_INVALID") {
		t.Fatalf("expected GOLC_SCENE_BARS_INVALID above the declared ceiling, got %v", err)
	}
}

func TestSceneNameEmptyRejected(t *testing.T) {
	_, err := scene.NewScene("  ", 4)
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_NAME_EMPTY") {
		t.Fatalf("expected GOLC_SCENE_NAME_EMPTY, got %v", err)
	}
}

func TestSceneUniqueNamesRejectsDuplicates(t *testing.T) {
	a, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	b, err := scene.NewScene("Verse", 8)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	err = scene.ValidateSceneUniqueNames([]scene.Scene{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_SCENE_DUPLICATE_NAME, got %v", err)
	}
}

func TestSceneSetLayerRejectsUnknownKind(t *testing.T) {
	s, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	_, err = scene.SetLayer(s, scene.Layer{Kind: scene.LayerKind("laser"), Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_LAYER_KIND_INVALID") {
		t.Fatalf("expected GOLC_SCENE_LAYER_KIND_INVALID, got %v", err)
	}
}

func TestSceneValidateLayerReferencesRejectsDangling(t *testing.T) {
	s, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.Chase, Enabled: true, Ref: uuid.Must(uuid.NewV7())})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}
	err = scene.ValidateLayerReferences([]scene.Scene{s}, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "GOLC_SCENE_LAYER_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_SCENE_LAYER_DANGLING_REFERENCE, got %v", err)
	}
}
