// layer_test.go proves ReduceLayers' fixed-priority contract (03-04-PLAN.md
// Task 1): layers resolve in base-look < color-theme < chase < motion
// order, a later layer overwrites only the attributes it touches, priority
// is order-not-magnitude (proving this is NOT highest-value-wins/HTP
// arbitration, CONTEXT D-02), a disabled layer contributes nothing, and a
// layer scoped to a narrower Selection only overlays the instances it
// actually touches.
package scene_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/scene"
)

func TestLayerCombinationFixedPriorityOverwritesOnlyTouchedAttributes(t *testing.T) {
	contributions := []scene.LayerContribution{
		{
			Kind:    scene.BaseLook,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 1.0,
				fixture.CapabilityColor:     0.5,
			}},
		},
		{
			Kind:    scene.Motion,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.2,
			}},
		},
	}
	result := scene.ReduceLayers(contributions)
	require.Equal(t, 0.2, result.Values[fixture.CapabilityIntensity], "expected motion to overwrite the shared intensity attribute")
	require.Equal(t, 0.5, result.Values[fixture.CapabilityColor], "expected base-look's untouched color attribute to survive")
}

func TestLayerCombinationDisabledLayerContributesNothing(t *testing.T) {
	contributions := []scene.LayerContribution{
		{
			Kind:    scene.BaseLook,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 1.0,
			}},
		},
		{
			Kind:    scene.Chase,
			Enabled: false,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.1,
			}},
		},
	}
	result := scene.ReduceLayers(contributions)
	require.Equal(t, 1.0, result.Values[fixture.CapabilityIntensity], "expected a disabled chase layer to contribute nothing")
}

func TestLayerCombinationPriorityIsOrderNotMagnitude(t *testing.T) {
	// base-look's value (1.0) is numerically larger than motion's (0.2),
	// but motion still wins because it comes later in layerPriority --
	// proving this is fixed-order precedence, NOT highest-value-wins
	// (HTP) arbitration (CONTEXT D-02).
	contributions := []scene.LayerContribution{
		{
			Kind:    scene.BaseLook,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 1.0,
			}},
		},
		{
			Kind:    scene.Motion,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.2,
			}},
		},
	}
	result := scene.ReduceLayers(contributions)
	require.Equal(t, 0.2, result.Values[fixture.CapabilityIntensity], "expected the lower-magnitude motion value to win by priority order (NOT HTP)")
}

func TestLayerCombinationPerLayerSelectionScopesIndependently(t *testing.T) {
	// Instance A: only base-look touches it -- a chase scoped narrower
	// than the scene's base-look (CONTEXT D-03) simply has no
	// contribution for this instance.
	instanceA := scene.ReduceLayers([]scene.LayerContribution{
		{
			Kind:    scene.BaseLook,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.8,
			}},
		},
	})
	require.Equal(t, 0.8, instanceA.Values[fixture.CapabilityIntensity], "expected base-look alone to cover instance A")

	// Instance B: both base-look and chase touch it -- chase's narrower
	// selection includes B, so it overlays base-look here.
	instanceB := scene.ReduceLayers([]scene.LayerContribution{
		{
			Kind:    scene.BaseLook,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.8,
			}},
		},
		{
			Kind:    scene.Chase,
			Enabled: true,
			Attributes: scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
				fixture.CapabilityIntensity: 0.3,
			}},
		},
	})
	require.Equal(t, 0.3, instanceB.Values[fixture.CapabilityIntensity], "expected chase to overlay base-look on the narrower-scoped instance B")
}
