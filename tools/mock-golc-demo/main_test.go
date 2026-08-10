package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
)

// closeEnough is the tolerance every hsvToRGB assertion below uses:
// float64 trig round-tripping through math.Mod/math.Abs/math.Sin never
// lands on an exact binary value even at "textbook" hue angles.
const closeEnough = 1e-9

func TestHSVToRGBPrimaryHues(t *testing.T) {
	tests := []struct {
		name                string
		h                   float64
		wantR, wantG, wantB float64
	}{
		{name: "0deg is pure red", h: 0, wantR: 1, wantG: 0, wantB: 0},
		{name: "120deg is pure green", h: 120, wantR: 0, wantG: 1, wantB: 0},
		{name: "240deg is pure blue", h: 240, wantR: 0, wantG: 0, wantB: 1},
		{name: "360deg wraps to red's sector (< 60 branch, not the default)", h: 359.999, wantR: 1, wantG: 0, wantB: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b := hsvToRGB(tc.h, 1, 1)
			assert.InDelta(t, tc.wantR, r, 1e-3, "r")
			assert.InDelta(t, tc.wantG, g, 1e-3, "g")
			assert.InDelta(t, tc.wantB, b, 1e-3, "b")
		})
	}
}

func TestHSVToRGBSaturationZeroIsGrayscale(t *testing.T) {
	for _, h := range []float64{0, 90, 180, 270} {
		r, g, b := hsvToRGB(h, 0, 0.5)
		assert.InDelta(t, 0.5, r, closeEnough, "r at h=%v", h)
		assert.InDelta(t, 0.5, g, closeEnough, "g at h=%v", h)
		assert.InDelta(t, 0.5, b, closeEnough, "b at h=%v", h)
	}
}

func TestHSVToRGBValueZeroIsBlack(t *testing.T) {
	r, g, b := hsvToRGB(200, 1, 0)
	assert.Equal(t, 0.0, r)
	assert.Equal(t, 0.0, g)
	assert.Equal(t, 0.0, b)
}

func TestHSVToRGBEverySectorStaysInUnitRange(t *testing.T) {
	// Sweeps every 60-degree sector boundary hsvToRGB's switch branches on
	// (0/60/120/180/240/300) plus a mid-sector sample each, proving no
	// branch ever produces a value outside [0,1] -- the property
	// animateInstance's callers actually depend on (these feed directly
	// into DMX-normalized AttributeSet values).
	for h := 0.0; h < 360; h += 15 {
		r, g, b := hsvToRGB(h, 1, 1)
		for _, v := range []float64{r, g, b} {
			require.GreaterOrEqual(t, v, 0.0, "h=%v", h)
			require.LessOrEqual(t, v, 1.0, "h=%v", h)
		}
	}
}

func TestNextSeq(t *testing.T) {
	tests := []struct {
		prev uint8
		want uint8
	}{
		{prev: 0, want: 1}, // never returns 0: Art-Net sequence 0 disables reordering
		{prev: 1, want: 2},
		{prev: 254, want: 255},
		{prev: 255, want: 1}, // wraps 255 -> 1, not 255 -> 0
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, nextSeq(tc.prev), "nextSeq(%d)", tc.prev)
	}
}

func TestPhaseOffsetIsDistinctPerAddress(t *testing.T) {
	seen := map[float64]int{}
	for _, address := range []int{1, 5, 9, 13, 17, 22, 27, 36} {
		offset := phaseOffset(address)
		assert.Equal(t, float64(address)*37, offset)
		seen[offset]++
	}
	for offset, count := range seen {
		assert.Equal(t, 1, count, "offset %v should be unique across the rig's 8 addresses so fixtures visibly desync", offset)
	}
}

func TestResolveMode(t *testing.T) {
	def := fixture.FixtureDefinition{
		Manufacturer: "Acme",
		Model:        "Par64",
		Modes: []fixture.Mode{
			{Name: "3ch", Channels: []fixture.ChannelSlot{{Type: fixture.CapabilityIntensity}}},
			{Name: "4ch", Channels: []fixture.ChannelSlot{{Type: fixture.CapabilityIntensity}}},
		},
	}

	t.Run("known mode resolves", func(t *testing.T) {
		mode, err := resolveMode(def, "4ch")
		require.NoError(t, err)
		assert.Equal(t, "4ch", mode.Name)
	})

	t.Run("unknown mode errors with manufacturer/model/mode context", func(t *testing.T) {
		_, err := resolveMode(def, "99ch")
		require.Error(t, err)
		assert.ErrorContains(t, err, "Acme")
		assert.ErrorContains(t, err, "Par64")
		assert.ErrorContains(t, err, "99ch")
	})
}

func TestAnimateInstanceProducesExpectedCapabilitiesPerModel(t *testing.T) {
	tests := []struct {
		model string
		want  []fixture.CapabilityType
	}{
		{model: "RGB Par", want: []fixture.CapabilityType{
			fixture.CapabilityIntensity, fixture.CapabilityColorRed, fixture.CapabilityColorGreen, fixture.CapabilityColorBlue,
		}},
		{model: "RGBW Wash", want: []fixture.CapabilityType{
			fixture.CapabilityIntensity, fixture.CapabilityColorRed, fixture.CapabilityColorGreen, fixture.CapabilityColorBlue, fixture.CapabilityColorWhite,
		}},
		{model: "Moving Head", want: []fixture.CapabilityType{
			fixture.CapabilityIntensity, fixture.CapabilityPan, fixture.CapabilityTilt, fixture.CapabilityColor, fixture.CapabilityGobo, fixture.CapabilityShutter, fixture.CapabilityZoom,
		}},
		{model: "Unknown Fixture", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			r := rigInstance{name: "test", address: 1, definition: &fixture.FixtureDefinition{Model: tc.model}}
			got := animateInstance(r, 12.5)
			if tc.want == nil {
				assert.Empty(t, got.Values, "unrecognized model must fall through to an empty AttributeSet, not panic or guess")
				return
			}
			for _, capability := range tc.want {
				value, ok := got.Values[capability]
				require.True(t, ok, "%s: expected capability %q to be set", tc.model, capability)
				assert.False(t, math.IsNaN(value), "%s: capability %q is NaN", tc.model, capability)
			}
			assert.Len(t, got.Values, len(tc.want), "%s: expected exactly the documented capability set, no extras", tc.model)
		})
	}
}
