// channelmap.go implements ARTN-03/ARTN-04's semantic-frame-to-DMX
// channel map (04-01-PLAN.md Task 3): a pure, in-memory transform from a
// playback.Frame's per-instance scene.AttributeSet values into per-universe
// 512-byte DMX buffers, mirroring internal/scene/layer.go's own
// "pure function of inputs, mutates nothing, no I/O, no time dependency"
// discipline (RESEARCH.md Pattern: AttributeSet.Overlay).
//
// CHANNEL ORDER HERE COMES STRICTLY FROM MODE.CHANNELS' DECLARED ORDER,
// NEVER FROM CAPABILITIES' DECLARATION ORDER (D-16/D-17, 04-RESEARCH.md's
// own Anti-Pattern): deriving DMX channel order from a fixture's
// Capabilities slice would silently produce wrong output for any
// multi-capability fixture whose author ordered capabilities differently
// than the fixture's real wiring order.
package artnet

import (
	"fmt"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
)

// channelsPerUniverse mirrors internal/deployment's own DMX/Art-Net
// universe channel-address bound (1-512 inclusive).
const channelsPerUniverse = 512

// InstanceFixture is the resolved (fixture.FixtureDefinition, fixture.Mode)
// pair Encode needs for one deployment.Instance: Mode is the specific
// entry of Definition.Modes whose Name matches Instance.Mode, carrying the
// D-16 channel layout that decides which DMX offset each capability value
// occupies.
type InstanceFixture struct {
	Definition fixture.FixtureDefinition
	Mode       fixture.Mode
}

// ResolveFunc resolves one deployment.Instance to its InstanceFixture.
// Callers (the Art-Net worker) build this from the show's loaded fixture
// definitions and Instance.Mode name; Encode itself never loads a fixture
// definition, keeping this a pure transform.
type ResolveFunc func(instance deployment.Instance) (InstanceFixture, error)

// Encode turns frame's semantic per-instance AttributeSet values into
// per-universe 512-byte DMX buffers by walking each instance's resolved
// Mode.Channels layout in declared order and scaling each channel's
// normalized [0,1] value to an 8-bit [0,255] byte deterministically
// (truncating toward zero). It is a pure function of its arguments: no I/O, no
// time dependency, and it never mutates frame or instances.
//
// An instance entirely absent from frame.Values (no AttributeSet recorded
// for it at all -- e.g. before playback starts) is treated as a
// zero/blackout instance: its channels are left at the buffer's default
// zero bytes and no error is raised (the backstop truth: a universe whose
// fixtures have no set attribute values still encodes to a valid,
// correct-length, all-zero DMX frame). An instance that IS present in
// frame.Values but whose AttributeSet is missing a value for one of its
// declared channel's CapabilityType fails loudly with
// GOLC_ARTNET_CHANNEL_VALUE_MISSING -- GOLC never silently guesses 0 for
// a partially-populated instance (D-17). A resolved Mode with no declared
// Channels (which fixture.Validate already hard-rejects at decode time,
// D-17) surfaces defensively here too as GOLC_ARTNET_CHANNEL_LAYOUT_MISSING
// rather than silently producing an all-zero buffer for a fixture that
// should have real channel data.
func Encode(frame playback.Frame, instances []deployment.Instance, resolve ResolveFunc) (map[int][]byte, error) {
	buffers := map[int][]byte{}

	for _, instance := range instances {
		resolved, err := resolve(instance)
		if err != nil {
			return nil, err
		}
		if len(resolved.Mode.Channels) == 0 {
			return nil, fmt.Errorf(
				"GOLC_ARTNET_CHANNEL_LAYOUT_MISSING: instance %s (mode %q) has no declared DMX channel layout",
				instance.ID, instance.Mode)
		}

		buffer, ok := buffers[instance.Universe]
		if !ok {
			buffer = make([]byte, channelsPerUniverse)
			buffers[instance.Universe] = buffer
		}

		attrs, present := frame.Values[instance.ID]
		if !present {
			// Fully-unset instance: backstop blackout behavior -- leave
			// this instance's channels at the buffer's default zero bytes,
			// never an error.
			continue
		}

		for channelIndex, slot := range resolved.Mode.Channels {
			value, ok := attrs.Values[slot.Type]
			if !ok {
				return nil, fmt.Errorf(
					"GOLC_ARTNET_CHANNEL_VALUE_MISSING: instance %s channel %d (%s, occurrence %d) has no value in the current frame",
					instance.ID, channelIndex, slot.Type, slot.Occurrence)
			}

			offset := instance.Address - 1 + channelIndex
			if offset < 0 || offset >= channelsPerUniverse {
				return nil, fmt.Errorf(
					"GOLC_ARTNET_CHANNEL_LAYOUT_MISSING: instance %s channel %d at address offset %d exceeds the %d-channel universe bound",
					instance.ID, channelIndex, offset, channelsPerUniverse)
			}
			buffer[offset] = scaleToByte(value)
		}
	}

	return buffers, nil
}

// scaleToByte scales a normalized [0,1] value to an 8-bit [0,255] byte,
// deterministically truncating toward zero (0.5 -> 127, never 128) and
// clamping out-of-range input to the nearest valid byte rather than
// overflowing/wrapping.
func scaleToByte(value float64) byte {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return byte(value * 255)
}
