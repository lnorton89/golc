package midi

import (
	"strings"
	"testing"
	"time"
)

// TestLearnProposeMappingAcceptsNonCollidingCandidate covers the base
// accept path: a candidate whose (channel, kind, number) tuple is not
// already in the existing set is accepted (nil error).
func TestLearnProposeMappingAcceptsNonCollidingCandidate(t *testing.T) {
	existing := []ControlKey{
		{Channel: 1, Kind: ControlChange, Number: 20},
	}
	candidate := ControlKey{Channel: 1, Kind: ControlChange, Number: 21}

	if err := ProposeMapping(existing, candidate); err != nil {
		t.Fatalf("ProposeMapping(non-colliding candidate) = %v, want nil", err)
	}
}

// TestLearnProposeMappingAcceptsIntoEmptySet covers proposing against an
// empty surface's mapping set.
func TestLearnProposeMappingAcceptsIntoEmptySet(t *testing.T) {
	var existing []ControlKey
	candidate := ControlKey{Channel: 1, Kind: Note, Number: 60}

	if err := ProposeMapping(existing, candidate); err != nil {
		t.Fatalf("ProposeMapping(empty set) = %v, want nil", err)
	}
}

// TestLearnProposeMappingRejectsConflictAndLeavesExistingUntouched is the
// D-06 core guarantee: a candidate colliding with an already-mapped
// (channel, kind, number) tuple is rejected outright with
// GOLC_MIDI_MAPPING_CONFLICT, and the existing set is never mutated.
func TestLearnProposeMappingRejectsConflictAndLeavesExistingUntouched(t *testing.T) {
	existing := []ControlKey{
		{Channel: 2, Kind: ControlChange, Number: 74},
		{Channel: 2, Kind: Note, Number: 36},
	}
	before := append([]ControlKey(nil), existing...)
	candidate := ControlKey{Channel: 2, Kind: ControlChange, Number: 74}

	err := ProposeMapping(existing, candidate)
	if err == nil {
		t.Fatalf("ProposeMapping(colliding candidate) = nil, want GOLC_MIDI_MAPPING_CONFLICT")
	}
	if !strings.Contains(err.Error(), "GOLC_MIDI_MAPPING_CONFLICT") {
		t.Fatalf("ProposeMapping error = %q, want it to contain GOLC_MIDI_MAPPING_CONFLICT", err.Error())
	}

	if len(existing) != len(before) {
		t.Fatalf("existing set length changed: got %d, want %d (D-06: existing must be untouched)", len(existing), len(before))
	}
	for i := range existing {
		if existing[i] != before[i] {
			t.Fatalf("existing[%d] changed: got %+v, want %+v (D-06: existing must be untouched)", i, existing[i], before[i])
		}
	}
}

// TestLearnProposeMappingScopedPerSurface proves D-07: the same
// (channel, kind, number) tuple that conflicts against one surface's set
// is freely accepted against a different (here, empty) surface's set --
// the check is scoped to whatever `existing` slice the caller passes in.
func TestLearnProposeMappingScopedPerSurface(t *testing.T) {
	surfaceA := []ControlKey{
		{Channel: 3, Kind: ControlChange, Number: 7},
	}
	candidate := ControlKey{Channel: 3, Kind: ControlChange, Number: 7}

	if err := ProposeMapping(surfaceA, candidate); err == nil {
		t.Fatalf("ProposeMapping against surfaceA = nil, want GOLC_MIDI_MAPPING_CONFLICT (candidate already mapped on surfaceA)")
	}

	var surfaceB []ControlKey
	if err := ProposeMapping(surfaceB, candidate); err != nil {
		t.Fatalf("ProposeMapping against surfaceB = %v, want nil (surfaceB's set is empty; D-07 per-surface scoping)", err)
	}
}

// TestLearnProposeMappingKindIsPartOfIdentity proves a Note and a
// ControlChange sharing the same channel/number are distinct keys: one
// being mapped does not block the other.
func TestLearnProposeMappingKindIsPartOfIdentity(t *testing.T) {
	existing := []ControlKey{
		{Channel: 1, Kind: Note, Number: 60},
	}
	candidate := ControlKey{Channel: 1, Kind: ControlChange, Number: 60}

	if err := ProposeMapping(existing, candidate); err != nil {
		t.Fatalf("ProposeMapping(same channel/number, different kind) = %v, want nil (kind is part of identity)", err)
	}
}

// TestLearnCaptureCandidateReturnsFirstReceived covers the accept path of
// the bounded capture window (D-05): the first ControlKey sent on next is
// returned before the timeout fires.
func TestLearnCaptureCandidateReturnsFirstReceived(t *testing.T) {
	next := make(chan ControlKey, 1)
	timeout := make(chan struct{})

	want := ControlKey{Channel: 4, Kind: ControlChange, Number: 11}
	next <- want

	got, err := CaptureCandidate(next, timeout)
	if err != nil {
		t.Fatalf("CaptureCandidate() error = %v, want nil", err)
	}
	if got != want {
		t.Fatalf("CaptureCandidate() = %+v, want %+v", got, want)
	}
}

// TestLearnCaptureCandidateTimesOut covers the D-05 bound: if the capture
// window's timeout fires before any ControlKey is received, capture ends
// with GOLC_MIDI_LEARN_TIMEOUT rather than hanging indefinitely.
func TestLearnCaptureCandidateTimesOut(t *testing.T) {
	next := make(chan ControlKey)
	timeout := make(chan struct{})
	close(timeout)

	_, err := CaptureCandidate(next, timeout)
	if err == nil {
		t.Fatalf("CaptureCandidate() error = nil, want GOLC_MIDI_LEARN_TIMEOUT")
	}
	if !strings.Contains(err.Error(), "GOLC_MIDI_LEARN_TIMEOUT") {
		t.Fatalf("CaptureCandidate() error = %q, want it to contain GOLC_MIDI_LEARN_TIMEOUT", err.Error())
	}
}

// TestLearnCaptureCandidateDoesNotHangWithoutEitherChannel guards against
// a regression to an unbounded select (e.g. accidentally dropping the
// timeout case): capture must return well within a generous deadline
// once the timeout channel closes, even under test-runner load.
func TestLearnCaptureCandidateDoesNotHangWithoutEitherChannel(t *testing.T) {
	next := make(chan ControlKey)
	timeout := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		_, err := CaptureCandidate(next, timeout)
		done <- err
	}()

	close(timeout)

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("CaptureCandidate() error = nil, want GOLC_MIDI_LEARN_TIMEOUT")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("CaptureCandidate did not return within 2s of timeout firing")
	}
}
