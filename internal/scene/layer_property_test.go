// layer_property_test.go property-tests ReduceLayers' fixed-priority merge
// contract (CONTEXT D-01..D-04), generalizing layer_test.go's fixed
// hand-picked examples (one or two layer kinds, one or two attributes
// each) across arbitrary combinations of the four fixed layer kinds, each
// independently present/absent, enabled/disabled, and touching an
// arbitrary subset of capability attributes with arbitrary [0,1] values:
//
//   - Order independence: ReduceLayers builds its own kind->contribution
//     map before consulting layerPriority, so the input contributions
//     slice's order must never affect the result -- a permutation of the
//     same contributions always produces an identical result.
//   - Per-attribute correctness: every attribute in the result comes from
//     the highest-priority ENABLED contribution that declares it (fixed
//     order, never magnitude -- CONTEXT D-02's "NOT HTP arbitration"),
//     with no extra or missing attribute. The expected value is computed
//     independently here (a plain forward map overwrite in priority
//     order) rather than by mirroring ReduceLayers'
//     byKind-map-plus-layerPriority-loop implementation.
package scene_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/scene"
)

// layerKindsInPriorityOrder mirrors scene.go's unexported layerPriority
// (base-look < color-theme < chase < motion).
var layerKindsInPriorityOrder = []scene.LayerKind{scene.BaseLook, scene.ColorTheme, scene.Chase, scene.Motion}

// genContribution draws one LayerContribution for kind: an Enabled flag,
// plus an independent presence/value draw per declared capability type so
// the touched-attribute subset varies freely across checks.
func genContribution(rt *rapid.T, kind scene.LayerKind) scene.LayerContribution {
	enabled := rapid.Bool().Draw(rt, string(kind)+"Enabled")
	values := make(map[fixture.CapabilityType]float64)
	for _, capability := range fixture.SupportedCapabilityTypes {
		if rapid.Bool().Draw(rt, string(kind)+"Has"+string(capability)) {
			values[capability] = rapid.Float64Range(0, 1).Draw(rt, string(kind)+"Value"+string(capability))
		}
	}
	return scene.LayerContribution{Kind: kind, Enabled: enabled, Attributes: scene.AttributeSet{Values: values}}
}

// shuffledCopy returns a Fisher-Yates-shuffled copy of items, drawing swap
// indices from rt so rapid can shrink toward a minimal reordering.
func shuffledCopy(rt *rapid.T, items []scene.LayerContribution) []scene.LayerContribution {
	shuffled := append([]scene.LayerContribution(nil), items...)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rapid.IntRange(0, i).Draw(rt, fmt.Sprintf("shuffleSwap%d", i))
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled
}

func TestReduceLayersProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Each of the four fixed kinds independently may or may not appear
		// in the contributions slice at all -- a kind with no contribution
		// contributes nothing, same as a disabled one.
		var contributions []scene.LayerContribution
		for _, kind := range layerKindsInPriorityOrder {
			if rapid.Bool().Draw(rt, string(kind)+"Present") {
				contributions = append(contributions, genContribution(rt, kind))
			}
		}

		result := scene.ReduceLayers(contributions)

		shuffled := shuffledCopy(rt, contributions)
		reshuffledResult := scene.ReduceLayers(shuffled)
		require.Equal(t, result.Values, reshuffledResult.Values,
			"ReduceLayers must be independent of the input contributions' slice order")

		// contributions was built above in ascending layerPriority order
		// (layerKindsInPriorityOrder), so a plain forward map overwrite
		// here independently computes "the highest-priority enabled
		// layer's value wins per attribute" without mirroring
		// ReduceLayers' own byKind-map-plus-layerPriority-loop algorithm.
		expected := map[fixture.CapabilityType]float64{}
		for _, contribution := range contributions {
			if !contribution.Enabled {
				continue
			}
			for capability, value := range contribution.Attributes.Values {
				expected[capability] = value
			}
		}
		// Compared key-by-key (not via require.Equal on the two maps
		// directly): ReduceLayers legitimately returns a nil Values map
		// when nothing is enabled, while expected above is always a
		// non-nil (possibly empty) map -- a representation difference
		// unrelated to this property, not a real mismatch.
		require.Len(t, result.Values, len(expected), "unexpected number of attributes in the reduced result")
		for capability, value := range expected {
			require.Equal(t, value, result.Values[capability],
				"expected the highest-priority enabled layer's value to win for attribute %s", capability)
		}
	})
}
