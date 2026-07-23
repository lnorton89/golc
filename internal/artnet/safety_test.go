// safety_test.go proves 06-02-PLAN.md Task 1's contract: applyOverrides is
// a pure transform whose blackout/stop-all, multiplicative master
// composition, empty-Frame no-op, and identity-preservation behaviors all
// hold exactly as specified, and the daemon-resident atomic state
// underneath it converges under concurrent Set/Load with no data race
// (PLAY-06/08/09).
package artnet

import (
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

func mustSafetyTestUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	return id
}

func intensityFrame(instanceID uuid.UUID, value float64) playback.Frame {
	return playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{fixture.CapabilityIntensity: value}},
	}}
}

// TestSafetyApplyOverridesBlackoutZeroesIntensity proves applyOverrides
// with blackout=true zeros a non-empty Frame's intensity regardless of
// input, and leaves non-intensity attributes untouched.
func TestSafetyApplyOverridesBlackoutZeroesIntensity(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 1.0,
			fixture.CapabilityColor:     0.7,
		}},
	}}

	s := newSafetyState()
	s.setBlackout(true)

	out := applyOverrides(frame, s, nil)

	got := out.Values[instanceID].Values[fixture.CapabilityIntensity]
	if got != 0 {
		t.Fatalf("expected blacked-out intensity to be 0, got %v", got)
	}
	if got := out.Values[instanceID].Values[fixture.CapabilityColor]; got != 0.7 {
		t.Fatalf("expected non-intensity attribute to survive blackout unchanged, got %v", got)
	}
}

// TestSafetyApplyOverridesStopAllZeroesIntensity proves stopAll=true
// drives output to the safe/zero state exactly like blackout.
func TestSafetyApplyOverridesStopAllZeroesIntensity(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	frame := intensityFrame(instanceID, 1.0)

	s := newSafetyState()
	s.setStopAll(true)

	out := applyOverrides(frame, s, nil)

	if got := out.Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0 {
		t.Fatalf("expected stop-all intensity to be 0, got %v", got)
	}
}

// TestSafetyApplyOverridesBlackoutEmptyFrameIsSafeNoOp proves blackout=true
// on an empty Frame (nothing playing) is a safe no-op: no instances to
// zero, and the returned Frame has no entries -- the "zero/safe state" is
// realized downstream by Encode's own zero-buffer default, not by
// synthesizing instance entries here (PLAY-06 empty edge).
func TestSafetyApplyOverridesBlackoutEmptyFrameIsSafeNoOp(t *testing.T) {
	s := newSafetyState()
	s.setBlackout(true)

	out := applyOverrides(playback.Frame{}, s, nil)

	if len(out.Values) != 0 {
		t.Fatalf("expected an empty Frame to remain empty under blackout, got %d entries", len(out.Values))
	}
}

// TestSafetyApplyOverridesMultiplicativeMasterComposition proves PLAY-06's
// adjacency edge: group=0.5, grand=0.5, programmed=full -> 0.25 -- masters
// compose multiplicatively, never additively.
func TestSafetyApplyOverridesMultiplicativeMasterComposition(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	groupID := mustSafetyTestUUID(t)
	frame := intensityFrame(instanceID, 1.0)
	membership := map[uuid.UUID][]uuid.UUID{instanceID: {groupID}}

	s := newSafetyState()
	if err := s.setGrandMaster(0.5); err != nil {
		t.Fatalf("setGrandMaster: %v", err)
	}
	if err := s.setGroupMaster(groupID, 0.5); err != nil {
		t.Fatalf("setGroupMaster: %v", err)
	}

	out := applyOverrides(frame, s, membership)

	got := out.Values[instanceID].Values[fixture.CapabilityIntensity]
	const want = 0.25
	if got != want {
		t.Fatalf("expected multiplicative composition 0.5*0.5=%.2f, got %v", want, got)
	}
}

// TestSafetyApplyOverridesIdentityLeavesFrameUnchanged proves that with
// every override at its identity value (blackout/stopAll false, grand
// master 1.0, no group overrides), applyOverrides returns a Frame
// equivalent to the input.
func TestSafetyApplyOverridesIdentityLeavesFrameUnchanged(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	frame := playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 0.8,
			fixture.CapabilityColor:     0.3,
		}},
	}}

	s := newSafetyState()

	out := applyOverrides(frame, s, nil)

	if !reflect.DeepEqual(frame, out) {
		t.Fatalf("expected identity overrides to leave the Frame unchanged: input=%+v output=%+v", frame, out)
	}
}

// TestSafetyApplyOverridesNilSafetyStateIsIdentity proves a nil
// *safetyState (an unconfigured Worker, e.g. an existing test that never
// sets WorkerConfig.Safety) behaves as identity -- no panic, no behavior
// change from before this transform existed.
func TestSafetyApplyOverridesNilSafetyStateIsIdentity(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	frame := intensityFrame(instanceID, 0.6)

	out := applyOverrides(frame, nil, nil)

	if got := out.Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0.6 {
		t.Fatalf("expected a nil safetyState to behave as identity, got %v", got)
	}
}

// TestSafetyMasterLevelValidationRejectsOutOfRange proves setGrandMaster/
// setGroupMaster reject a level outside [0,1] as
// GOLC_ARTNET_SAFETY_MASTER_INVALID, and leave the prior value unchanged.
func TestSafetyMasterLevelValidationRejectsOutOfRange(t *testing.T) {
	s := newSafetyState()
	groupID := mustSafetyTestUUID(t)

	if err := s.setGrandMaster(1.5); err == nil {
		t.Fatal("expected an error for grand master level 1.5")
	}
	if err := s.setGrandMaster(-0.1); err == nil {
		t.Fatal("expected an error for grand master level -0.1")
	}
	if err := s.setGroupMaster(groupID, 2.0); err == nil {
		t.Fatal("expected an error for group master level 2.0")
	}

	if got := s.currentMasters().grand; got != 1.0 {
		t.Fatalf("expected the prior grand master (1.0) to survive a rejected update, got %v", got)
	}
}

// TestSafetyConcurrentBlackoutConvergesUnderRace proves (PLAY-09):
// concurrent/rapid-fire Blackout Set calls, raced against concurrent Load
// reads via applyOverrides, converge with no data race (run with -race)
// and the flag ends in a defined final state.
func TestSafetyConcurrentBlackoutConvergesUnderRace(t *testing.T) {
	instanceID := mustSafetyTestUUID(t)
	frame := intensityFrame(instanceID, 1.0)
	s := newSafetyState()

	var wg sync.WaitGroup
	const goroutines = 8
	const iterations = 200

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s.setBlackout(n%2 == 0)
			}
		}(i)
	}
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = applyOverrides(frame, s, nil)
			}
		}()
	}
	wg.Wait()

	// Converge to a defined final state: setting true unconditionally
	// after every racing goroutine finishes, then asserting the read-back
	// is exactly that state.
	s.setBlackout(true)
	out := applyOverrides(frame, s, nil)
	if got := out.Values[instanceID].Values[fixture.CapabilityIntensity]; got != 0 {
		t.Fatalf("expected blackout to converge to zeroed intensity, got %v", got)
	}
}
