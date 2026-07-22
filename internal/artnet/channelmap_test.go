// channelmap_test.go proves ARTN-03/ARTN-04's semantic-frame-to-DMX
// channel map contract (04-01-PLAN.md, Task 3): exact DMX bytes for a
// known frame+layout (offset placement and [0,1]->[0,255] scaling), two
// instances sharing one 512-byte universe buffer at their own address
// offsets, a full-length all-zero buffer for a blackout universe, and a
// loud GOLC_ARTNET_CHANNEL_VALUE_MISSING failure for a partially-populated
// instance missing a declared channel's value.
package artnet_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

// intensityColorMode is the shared single-instance test fixture mode:
// Mode.Channels = [intensity@0, color@0] (D-16 declared order).
var intensityColorMode = fixture.Mode{
	Name: "Standard",
	Channels: []fixture.ChannelSlot{
		{Type: fixture.CapabilityIntensity, Occurrence: 0},
		{Type: fixture.CapabilityColor, Occurrence: 0},
	},
}

func staticResolver(mode fixture.Mode) artnet.ResolveFunc {
	return func(instance deployment.Instance) (artnet.InstanceFixture, error) {
		return artnet.InstanceFixture{
			Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}},
			Mode:       mode,
		}, nil
	}
}

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	return id
}

// TestEncodeOffsetAndScaling proves exact DMX byte placement and
// deterministic [0,1]->[0,255] scaling for a single instance.
func TestEncodeOffsetAndScaling(t *testing.T) {
	instanceID := mustUUID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 1.0,
			fixture.CapabilityColor:     0.5,
		}},
	}}

	buffers, err := artnet.Encode(frame, []deployment.Instance{instance}, staticResolver(intensityColorMode))
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	buffer, ok := buffers[1]
	if !ok {
		t.Fatalf("expected a universe-1 buffer, got %+v", buffers)
	}
	if len(buffer) != 512 {
		t.Fatalf("expected a 512-byte universe buffer, got %d bytes", len(buffer))
	}
	if buffer[0] != 255 {
		t.Fatalf("expected byte[0] (intensity 1.0) == 255, got %d", buffer[0])
	}
	if buffer[1] != 127 {
		t.Fatalf("expected byte[1] (color 0.5) == 127, got %d", buffer[1])
	}
	for i := 2; i < len(buffer); i++ {
		if buffer[i] != 0 {
			t.Fatalf("expected every other byte to be zero, byte[%d]=%d", i, buffer[i])
		}
	}
}

// TestEncodeTwoInstancesSharedBuffer proves two instances in the same
// universe at non-overlapping addresses write into their own address
// offsets in one shared 512-byte buffer.
func TestEncodeTwoInstancesSharedBuffer(t *testing.T) {
	firstID := mustUUID(t)
	secondID := mustUUID(t)

	first := deployment.Instance{ID: firstID, Mode: "Standard", Universe: 1, Address: 1}
	second := deployment.Instance{ID: secondID, Mode: "Standard", Universe: 1, Address: 10}

	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		firstID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 1.0,
			fixture.CapabilityColor:     1.0,
		}},
		secondID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 0.0,
			fixture.CapabilityColor:     1.0,
		}},
	}}

	buffers, err := artnet.Encode(frame, []deployment.Instance{first, second}, staticResolver(intensityColorMode))
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(buffers) != 1 {
		t.Fatalf("expected exactly one universe buffer, got %d", len(buffers))
	}
	buffer := buffers[1]
	if buffer[0] != 255 || buffer[1] != 255 {
		t.Fatalf("expected first instance's channels at offset 0-1 to be 255,255; got %d,%d", buffer[0], buffer[1])
	}
	if buffer[9] != 0 || buffer[10] != 255 {
		t.Fatalf("expected second instance's channels at offset 9-10 to be 0,255; got %d,%d", buffer[9], buffer[10])
	}
}

// TestEncodeBlackoutUniverse proves a fully-unset instance (absent from
// frame.Values entirely -- never evaluated) still yields a full-length,
// correct all-zero DMX buffer for its universe, never an empty/short
// buffer or an error (the backstop truth).
func TestEncodeBlackoutUniverse(t *testing.T) {
	instanceID := mustUUID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{}}

	buffers, err := artnet.Encode(frame, []deployment.Instance{instance}, staticResolver(intensityColorMode))
	if err != nil {
		t.Fatalf("Encode failed for a blackout universe: %v", err)
	}
	buffer, ok := buffers[1]
	if !ok {
		t.Fatalf("expected a universe-1 buffer even with no set attribute values, got %+v", buffers)
	}
	if len(buffer) != 512 {
		t.Fatalf("expected a full-length 512-byte buffer, got %d bytes", len(buffer))
	}
	for i, b := range buffer {
		if b != 0 {
			t.Fatalf("expected an all-zero blackout buffer, byte[%d]=%d", i, b)
		}
	}
}

// TestEncodeExplicitZeroValues proves an instance present in the frame
// with explicit zero attribute values (not absent, just zero) scales to
// all-zero bytes through ordinary scaling -- the same result as a
// blackout, reached through the normal value path rather than the
// fully-unset shortcut.
func TestEncodeExplicitZeroValues(t *testing.T) {
	instanceID := mustUUID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 0.0,
			fixture.CapabilityColor:     0.0,
		}},
	}}

	buffers, err := artnet.Encode(frame, []deployment.Instance{instance}, staticResolver(intensityColorMode))
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	buffer := buffers[1]
	if buffer[0] != 0 || buffer[1] != 0 {
		t.Fatalf("expected explicit zero values to scale to zero bytes, got %d,%d", buffer[0], buffer[1])
	}
}

// TestEncodeMissingChannelValueFails proves a channel slot whose
// CapabilityType is absent from a present-but-partially-populated
// instance's AttributeSet fails loudly with
// GOLC_ARTNET_CHANNEL_VALUE_MISSING, never silently defaulting to 0
// (D-17).
func TestEncodeMissingChannelValueFails(t *testing.T) {
	instanceID := mustUUID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 1.0,
			// color is declared in the Mode's channel layout but absent here.
		}},
	}}

	_, err := artnet.Encode(frame, []deployment.Instance{instance}, staticResolver(intensityColorMode))
	if err == nil {
		t.Fatal("expected a partially-populated instance missing a declared channel value to fail")
	}
	if !strings.Contains(err.Error(), "GOLC_ARTNET_CHANNEL_VALUE_MISSING") {
		t.Fatalf("expected GOLC_ARTNET_CHANNEL_VALUE_MISSING, got %v", err)
	}
}
