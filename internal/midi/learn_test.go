package midi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLearnProposeMappingAcceptsNonCollidingCandidate covers the base
// accept path: a candidate whose (channel, kind, number) tuple is not
// already in the existing set is accepted (nil error).
func TestLearnProposeMappingAcceptsNonCollidingCandidate(t *testing.T) {
	existing := []ControlKey{
		{Channel: 1, Kind: ControlChange, Number: 20},
	}
	candidate := ControlKey{Channel: 1, Kind: ControlChange, Number: 21}

	require.NoError(t, ProposeMapping(existing, candidate), "ProposeMapping(non-colliding candidate)")
}

// TestLearnProposeMappingAcceptsIntoEmptySet covers proposing against an
// empty surface's mapping set.
func TestLearnProposeMappingAcceptsIntoEmptySet(t *testing.T) {
	var existing []ControlKey
	candidate := ControlKey{Channel: 1, Kind: Note, Number: 60}

	require.NoError(t, ProposeMapping(existing, candidate), "ProposeMapping(empty set)")
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
	require.ErrorContains(t, err, "GOLC_MIDI_MAPPING_CONFLICT", "ProposeMapping(colliding candidate)")
	require.Equal(t, before, existing, "D-06: existing must be untouched")
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

	err := ProposeMapping(surfaceA, candidate)
	require.ErrorContains(t, err, "GOLC_MIDI_MAPPING_CONFLICT", "ProposeMapping against surfaceA (candidate already mapped on surfaceA)")

	var surfaceB []ControlKey
	err = ProposeMapping(surfaceB, candidate)
	require.NoError(t, err, "ProposeMapping against surfaceB (surfaceB's set is empty; D-07 per-surface scoping)")
}

// TestLearnProposeMappingKindIsPartOfIdentity proves a Note and a
// ControlChange sharing the same channel/number are distinct keys: one
// being mapped does not block the other.
func TestLearnProposeMappingKindIsPartOfIdentity(t *testing.T) {
	existing := []ControlKey{
		{Channel: 1, Kind: Note, Number: 60},
	}
	candidate := ControlKey{Channel: 1, Kind: ControlChange, Number: 60}

	require.NoError(t, ProposeMapping(existing, candidate), "ProposeMapping(same channel/number, different kind) (kind is part of identity)")
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
	require.NoError(t, err, "CaptureCandidate()")
	require.Equal(t, want, got)
}

// TestLearnCaptureCandidateTimesOut covers the D-05 bound: if the capture
// window's timeout fires before any ControlKey is received, capture ends
// with GOLC_MIDI_LEARN_TIMEOUT rather than hanging indefinitely.
func TestLearnCaptureCandidateTimesOut(t *testing.T) {
	next := make(chan ControlKey)
	timeout := make(chan struct{})
	close(timeout)

	_, err := CaptureCandidate(next, timeout)
	require.ErrorContains(t, err, "GOLC_MIDI_LEARN_TIMEOUT", "CaptureCandidate() error")
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
		require.Error(t, err, "CaptureCandidate() error = nil, want GOLC_MIDI_LEARN_TIMEOUT")
	case <-time.After(2 * time.Second):
		require.Fail(t, "CaptureCandidate did not return within 2s of timeout firing")
	}
}
