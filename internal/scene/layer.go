// layer.go implements the fixed-priority layer reduce (CONTEXT D-01..D-04,
// 03-RESEARCH.md Pattern 2): AttributeSet is one fixture instance's plain
// semantic attribute map with an Overlay per-attribute merge; ReduceLayers
// walks layerPriority (declared in scene.go) in fixed order and overlays
// each enabled layer's already-resolved contribution onto the running
// result, so a later layer overrides an earlier one only for the
// attributes it actually touches. This mirrors internal/pool/impact.go's
// "deterministic pure function of inputs, mutates nothing" style: no I/O,
// no time dependency -- resolving a chase/motion layer's own time-varying
// value at a given musical position is the engine's concern (03-07); this
// package only reduces already-resolved per-instance contributions.
package scene

import (
	"github.com/lnorton89/golc/internal/fixture"
)

// AttributeSet is one fixture instance's semantic attribute values: a plain
// capability -> normalized [0,1] value map.
type AttributeSet struct {
	Values map[fixture.CapabilityType]float64 `json:"values,omitempty"`
}

// Overlay returns a new AttributeSet with every value in a, overwritten by
// any matching capability present in other -- other wins per-attribute,
// and a's own untouched attributes survive unchanged. This is a plain map
// merge: the caller's own layerPriority loop order is what determines
// precedence, never a comparison of the values themselves (CONTEXT D-02:
// this is NOT highest-value-wins/HTP arbitration).
func (a AttributeSet) Overlay(other AttributeSet) AttributeSet {
	merged := make(map[fixture.CapabilityType]float64, len(a.Values)+len(other.Values))
	for capability, value := range a.Values {
		merged[capability] = value
	}
	for capability, value := range other.Values {
		merged[capability] = value
	}
	return AttributeSet{Values: merged}
}

// LayerContribution is one layer's already-resolved contribution to a
// single fixture instance: Kind identifies which fixed layer slot produced
// it, Enabled mirrors the source Layer's own Enabled flag (a disabled
// layer's Attributes are never applied even if present, CONTEXT D-01..
// D-04), and Attributes is that layer's touched semantic attribute values
// for this instance. Callers assemble one []LayerContribution per fixture
// instance -- a chase/motion layer scoped to a Selection narrower than the
// scene's base-look (CONTEXT D-03) simply omits a contribution for an
// out-of-scope instance, rather than ReduceLayers itself doing any
// selection filtering.
type LayerContribution struct {
	Kind       LayerKind
	Enabled    bool
	Attributes AttributeSet
}

// ReduceLayers walks layerPriority in fixed order (CONTEXT D-01..D-04:
// base-look < color-theme < chase < motion) and overlays each enabled
// contribution's Attributes onto the running result, so a later layer in
// the fixed order overrides an earlier one for any attribute it actually
// touches.
//
// PRECEDENCE HERE IS FIXED LAYER ORDER, NOT HIGHEST-VALUE-WINS (HTP)
// ARBITRATION AND NOT PER-LAYER BLEND-WEIGHT MIXING (CONTEXT D-02): this
// function never compares the values themselves to decide a winner --
// only layerPriority's loop order matters. A disabled layer, or a layer
// kind with no contribution at all for this instance, contributes
// nothing.
func ReduceLayers(contributions []LayerContribution) AttributeSet {
	byKind := make(map[LayerKind]LayerContribution, len(contributions))
	for _, contribution := range contributions {
		byKind[contribution.Kind] = contribution
	}

	result := AttributeSet{}
	for _, kind := range layerPriority {
		contribution, ok := byKind[kind]
		if !ok || !contribution.Enabled {
			continue
		}
		result = result.Overlay(contribution.Attributes)
	}
	return result
}
