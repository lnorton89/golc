// frame_test.go proves Frame's deterministic-encoding contract
// (03-07-PLAN.md Task 1, CONTEXT SCEN-09): encoding.CanonicalEncode
// (which sorts map keys) produces byte-identical output for a Frame with
// multiple instance entries across repeated encodes.
package playback_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/strictjson"
)

func TestFrameCanonicalEncodeIsByteIdentical(t *testing.T) {
	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		uuid.New(): {Values: map[fixture.CapabilityType]float64{fixture.CapabilityIntensity: 0.5}},
		uuid.New(): {Values: map[fixture.CapabilityType]float64{fixture.CapabilityColor: 0.8}},
	}}

	first, err := strictjson.CanonicalEncode(frame)
	if err != nil {
		t.Fatalf("CanonicalEncode (first): %v", err)
	}
	second, err := strictjson.CanonicalEncode(frame)
	if err != nil {
		t.Fatalf("CanonicalEncode (second): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("expected byte-identical encodes of the same Frame:\nfirst:  %s\nsecond: %s", first, second)
	}
}
