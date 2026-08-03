// channelmap_property_test.go property-tests Encode's DMX packing contract
// (CONTEXT ARTN-03/ARTN-04), generalizing channelmap_test.go's fixed
// hand-picked examples (one or two instances, one fixed 2-channel mode,
// one or two hand-picked values) across arbitrary channel counts,
// addresses, universes, and attribute values -- including out-of-[0,1]
// input to exercise scaleToByte's clamping, which every existing fixed
// example only ever feeds valid [0,1] input.
package artnet_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/lnorton89/golc/internal/artnet"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

// expectedByte independently restates scaleToByte's documented clamp+scale
// spec (out-of-range input clamps to the nearest valid byte, in-range
// input truncates toward zero) without reaching into the unexported
// function itself -- this package is external (artnet_test), matching
// channelmap_test.go's own black-box convention.
func expectedByte(value float64) byte {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return byte(value * 255)
}

// buildChannelMode returns a fixture.Mode declaring one channel per
// capability in channelTypes, in that exact order (D-16: channel order
// comes from Mode.Channels, never capability declaration order).
func buildChannelMode(channelTypes []fixture.CapabilityType) fixture.Mode {
	channels := make([]fixture.ChannelSlot, len(channelTypes))
	for i, capabilityType := range channelTypes {
		channels[i] = fixture.ChannelSlot{Type: capabilityType, Occurrence: 0}
	}
	return fixture.Mode{Name: "Property", Channels: channels}
}

// drawDistinctCapabilityTypes draws n distinct capability types (n must be
// <= len(fixture.SupportedCapabilityTypes)) via independent per-type
// presence flags, retrying until exactly n are chosen -- simpler and lower
// risk than relying on a permutation generator.
func drawDistinctCapabilityTypes(rt *rapid.T, n int) []fixture.CapabilityType {
	for {
		chosen := make([]fixture.CapabilityType, 0, n)
		for _, capabilityType := range fixture.SupportedCapabilityTypes {
			if rapid.Bool().Draw(rt, "want_"+string(capabilityType)) {
				chosen = append(chosen, capabilityType)
			}
		}
		if len(chosen) == n {
			return chosen
		}
	}
}

// TestEncodeSingleInstancePlacementAndClampingProperty generalizes
// TestEncodeOffsetAndScaling across arbitrary channel counts, addresses,
// and attribute values (including out-of-[0,1] values, which every fixed
// example only ever feeds valid input): every channel byte lands at
// address-1+channelIndex scaled/clamped per scaleToByte's spec, every
// other byte in the 512-byte universe buffer stays zero, and the buffer
// is always exactly 512 bytes regardless of channel count or placement.
func TestEncodeSingleInstancePlacementAndClampingProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		channelCount := rapid.IntRange(1, 6).Draw(rt, "channelCount")
		channelTypes := drawDistinctCapabilityTypes(rt, channelCount)
		mode := buildChannelMode(channelTypes)

		address := rapid.IntRange(1, 512-channelCount+1).Draw(rt, "address")
		universe := rapid.IntRange(1, 5).Draw(rt, "universe")

		instanceID, err := uuid.NewV7()
		require.NoError(t, err, "uuid.NewV7")
		instance := deployment.Instance{ID: instanceID, Mode: "Property", Universe: universe, Address: address}

		values := make(map[fixture.CapabilityType]float64, channelCount)
		expected := make([]byte, channelCount)
		for i, capabilityType := range channelTypes {
			raw := rapid.Float64Range(-1, 2).Draw(rt, fmt.Sprintf("value%d", i))
			values[capabilityType] = raw
			expected[i] = expectedByte(raw)
		}
		frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{instanceID: {Values: values}}}

		resolve := func(deployment.Instance) (artnet.InstanceFixture, error) {
			return artnet.InstanceFixture{Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}}, Mode: mode}, nil
		}

		buffers, err := artnet.Encode(frame, []deployment.Instance{instance}, resolve)
		require.NoError(t, err, "Encode")
		require.Len(t, buffers, 1, "expected exactly one universe buffer")
		buffer, ok := buffers[universe]
		require.True(t, ok, "expected a buffer for universe %d", universe)
		require.Len(t, buffer, 512, "expected a full-length 512-byte buffer")

		touched := make(map[int]bool, channelCount)
		for i := 0; i < channelCount; i++ {
			offset := address - 1 + i
			touched[offset] = true
			require.Equalf(t, expected[i], buffer[offset],
				"channel %d (%s) at offset %d: expected clamp/scale of %v", i, channelTypes[i], offset, values[channelTypes[i]])
		}
		for offset := 0; offset < 512; offset++ {
			if !touched[offset] {
				require.Equalf(t, byte(0), buffer[offset], "expected untouched offset %d to stay zero", offset)
			}
		}
	})
}

// TestEncodeMultipleUniversesIsolationProperty generalizes
// TestEncodeTwoInstancesSharedBuffer's universe-splitting guarantee: an
// arbitrary number of single-channel instances scattered across distinct
// universes each produce their own independent, correctly-scaled,
// full-length buffer -- no instance's value ever leaks into another
// universe's buffer.
func TestEncodeMultipleUniversesIsolationProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		mode := buildChannelMode([]fixture.CapabilityType{fixture.CapabilityIntensity})

		instanceCount := rapid.IntRange(1, 5).Draw(rt, "instanceCount")
		var instances []deployment.Instance
		values := map[uuid.UUID]scene.AttributeSet{}
		expectedByUniverse := map[int]byte{}

		for i := 0; i < instanceCount; i++ {
			instanceID, err := uuid.NewV7()
			require.NoError(t, err, "uuid.NewV7")
			// Each instance gets its own universe number (offset by index),
			// so distinct instances never share a universe -- isolating
			// this property to universe-splitting, not same-universe
			// address packing (already covered by the placement property).
			universe := i + 1
			instances = append(instances, deployment.Instance{ID: instanceID, Mode: "Property", Universe: universe, Address: 1})

			raw := rapid.Float64Range(0, 1).Draw(rt, fmt.Sprintf("value%d", i))
			values[instanceID] = scene.AttributeSet{Values: map[fixture.CapabilityType]float64{fixture.CapabilityIntensity: raw}}
			expectedByUniverse[universe] = expectedByte(raw)
		}

		resolve := func(deployment.Instance) (artnet.InstanceFixture, error) {
			return artnet.InstanceFixture{Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}}, Mode: mode}, nil
		}

		buffers, err := artnet.Encode(playback.Frame{Values: values}, instances, resolve)
		require.NoError(t, err, "Encode")
		require.Len(t, buffers, instanceCount, "expected exactly one buffer per distinct universe")
		for universe, expectedFirstByte := range expectedByUniverse {
			buffer, ok := buffers[universe]
			require.Truef(t, ok, "expected a buffer for universe %d", universe)
			require.Lenf(t, buffer, 512, "expected a full-length buffer for universe %d", universe)
			require.Equalf(t, expectedFirstByte, buffer[0], "universe %d: unexpected byte 0", universe)
			for offset := 1; offset < 512; offset++ {
				require.Equalf(t, byte(0), buffer[offset], "universe %d: expected untouched offset %d to stay zero", universe, offset)
			}
		}
	})
}

// TestEncodeMissingChannelValueProperty generalizes
// TestEncodeMissingChannelValueFails across arbitrary channel counts and
// an arbitrary missing channel position: a present-but-partially-populated
// instance missing any one of its declared channels' values always fails
// with GOLC_ARTNET_CHANNEL_VALUE_MISSING, never silently defaulting to 0
// (D-17), regardless of how many channels the mode declares or which one
// is missing.
func TestEncodeMissingChannelValueProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		channelCount := rapid.IntRange(1, 6).Draw(rt, "channelCount")
		channelTypes := drawDistinctCapabilityTypes(rt, channelCount)
		mode := buildChannelMode(channelTypes)
		missingIndex := rapid.IntRange(0, channelCount-1).Draw(rt, "missingIndex")

		instanceID, err := uuid.NewV7()
		require.NoError(t, err, "uuid.NewV7")
		instance := deployment.Instance{ID: instanceID, Mode: "Property", Universe: 1, Address: 1}

		values := make(map[fixture.CapabilityType]float64, channelCount-1)
		for i, capabilityType := range channelTypes {
			if i == missingIndex {
				continue
			}
			values[capabilityType] = rapid.Float64Range(0, 1).Draw(rt, fmt.Sprintf("value%d", i))
		}
		frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{instanceID: {Values: values}}}

		resolve := func(deployment.Instance) (artnet.InstanceFixture, error) {
			return artnet.InstanceFixture{Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}}, Mode: mode}, nil
		}

		_, err = artnet.Encode(frame, []deployment.Instance{instance}, resolve)
		require.ErrorContains(t, err, "GOLC_ARTNET_CHANNEL_VALUE_MISSING")
	})
}
