// scene_test.go proves Scene's identity/construction/rename/unique-name/
// single-active-scene contract (03-04-PLAN.md Task 1): NewScene mints a
// UUIDv7 ID and enforces the BarsPerLoop boundary (1 valid, 0/negative/
// above-ceiling rejected); ValidateSingleActiveScene accepts zero or one
// active scene and rejects two; ActivateScene guarantees exactly one
// active scene even across repeated activation, mirroring
// internal/deployment/model.go's ValidateSingleActive/Activate tests.
package scene_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/scene"
)

func TestSingleActiveSceneAcceptsZeroOrOneActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	require.NoError(t, err, "NewScene")
	b, err := scene.NewScene("B", 4)
	require.NoError(t, err, "NewScene")
	require.NoError(t, scene.ValidateSingleActiveScene([]scene.Scene{a, b}), "expected zero active scenes to be valid")
	a.Active = true
	require.NoError(t, scene.ValidateSingleActiveScene([]scene.Scene{a, b}), "expected exactly one active scene to be valid")
}

func TestSingleActiveSceneRejectsMultipleActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	require.NoError(t, err, "NewScene")
	b, err := scene.NewScene("B", 4)
	require.NoError(t, err, "NewScene")
	a.Active = true
	b.Active = true
	err = scene.ValidateSingleActiveScene([]scene.Scene{a, b})
	require.ErrorContains(t, err, "GOLC_SCENE_MULTIPLE_ACTIVE", "expected error for two active scenes")
}

func TestSingleActiveSceneActivateNeverTransientlyTwoActive(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	require.NoError(t, err, "NewScene")
	b, err := scene.NewScene("B", 4)
	require.NoError(t, err, "NewScene")

	activated, err := scene.ActivateScene([]scene.Scene{a, b}, "B")
	require.NoError(t, err, "ActivateScene")
	assertExactlyOneActiveNamed(t, activated, "B")

	// A second activate against the other scene still keeps exactly one
	// active -- never transiently two.
	reactivated, err := scene.ActivateScene(activated, "A")
	require.NoError(t, err, "ActivateScene (second)")
	assertExactlyOneActiveNamed(t, reactivated, "A")
}

func assertExactlyOneActiveNamed(t *testing.T, scenes []scene.Scene, expectedName string) {
	t.Helper()
	activeCount := 0
	for _, s := range scenes {
		if s.Active {
			activeCount++
			require.Equal(t, expectedName, s.Name, "expected %q to be the only active scene", expectedName)
		}
	}
	require.Equal(t, 1, activeCount, "expected exactly one active scene")
}

func TestSingleActiveSceneActivateNotFound(t *testing.T) {
	a, err := scene.NewScene("A", 4)
	require.NoError(t, err, "NewScene")
	_, err = scene.ActivateScene([]scene.Scene{a}, "Missing")
	require.ErrorContains(t, err, "GOLC_SCENE_NOT_FOUND")
}

func TestSceneBarsPerLoopBoundary(t *testing.T) {
	_, err := scene.NewScene("Loop", 1)
	require.NoError(t, err, "expected bars_per_loop=1 to be valid (loops every bar)")
	_, err = scene.NewScene("Loop", 0)
	require.ErrorContains(t, err, "GOLC_SCENE_BARS_INVALID", "expected error for bars_per_loop=0")
	_, err = scene.NewScene("Loop", -1)
	require.ErrorContains(t, err, "GOLC_SCENE_BARS_INVALID", "expected error for a negative bars_per_loop")
	_, err = scene.NewScene("Loop", 100000)
	require.ErrorContains(t, err, "GOLC_SCENE_BARS_INVALID", "expected error above the declared ceiling")
}

func TestSceneNameEmptyRejected(t *testing.T) {
	_, err := scene.NewScene("  ", 4)
	require.ErrorContains(t, err, "GOLC_SCENE_NAME_EMPTY")
}

func TestSceneUniqueNamesRejectsDuplicates(t *testing.T) {
	a, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	b, err := scene.NewScene("Verse", 8)
	require.NoError(t, err, "NewScene")
	err = scene.ValidateSceneUniqueNames([]scene.Scene{a, b})
	require.ErrorContains(t, err, "GOLC_SCENE_DUPLICATE_NAME")
}

func TestSceneSetLayerRejectsUnknownKind(t *testing.T) {
	s, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	_, err = scene.SetLayer(s, scene.Layer{Kind: scene.LayerKind("laser"), Enabled: true})
	require.ErrorContains(t, err, "GOLC_SCENE_LAYER_KIND_INVALID")
}

func TestSceneValidateLayerReferencesRejectsDangling(t *testing.T) {
	s, err := scene.NewScene("Verse", 4)
	require.NoError(t, err, "NewScene")
	s, err = scene.SetLayer(s, scene.Layer{Kind: scene.Chase, Enabled: true, Ref: uuid.Must(uuid.NewV7())})
	require.NoError(t, err, "SetLayer")
	err = scene.ValidateLayerReferences([]scene.Scene{s}, nil, nil, nil, nil)
	require.ErrorContains(t, err, "GOLC_SCENE_LAYER_DANGLING_REFERENCE")
}
