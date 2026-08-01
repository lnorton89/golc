package artnet

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetValidateTargetAcceptsValidTarget(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	require.NoError(t, ValidateTarget(target), "expected valid target to pass")
}

func TestTargetValidateTargetDefaultsUnspecifiedPort(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5")}
	require.NoErrorf(t, ValidateTarget(target), "expected target with unspecified port to default to %d and pass", artNetPort)
}

func TestTargetValidateTargetRejectsNonPositiveUniverse(t *testing.T) {
	target := Target{Universe: 0, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
	require.Error(t, ValidateTarget(target), "expected error for non-positive universe")
}

func TestTargetValidateTargetRejectsUniverseAboveMaxRepresentable(t *testing.T) {
	for _, universe := range []int{256, 257} {
		target := Target{Universe: universe, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
		require.Errorf(t, ValidateTarget(target), "expected error for universe %d (exceeds artNetMaxUniverse=%d, would alias onto a lower universe's Port-Address)", universe, artNetMaxUniverse)
	}
}

func TestTargetValidateTargetAcceptsMaxRepresentableUniverse(t *testing.T) {
	target := Target{Universe: artNetMaxUniverse, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
	require.NoErrorf(t, ValidateTarget(target), "expected universe %d (the maximum representable) to pass", artNetMaxUniverse)
}

func TestTargetValidateTargetRejectsNilIP(t *testing.T) {
	target := Target{Universe: 1, Port: artNetPort}
	require.Error(t, ValidateTarget(target), "expected error for nil IP")
}

func TestTargetValidateTargetRejectsUnspecifiedIP(t *testing.T) {
	target := Target{Universe: 1, IP: net.IPv4zero, Port: artNetPort}
	require.Error(t, ValidateTarget(target), "expected error for unspecified (0.0.0.0) IP")
}

func TestTargetValidateTargetRejectsBroadcastIP(t *testing.T) {
	target := Target{Universe: 1, IP: net.IPv4bcast, Port: artNetPort}
	require.Error(t, ValidateTarget(target), "expected error for the IPv4 broadcast address (D-07 unicast-only)")
}

func TestTargetValidateTargetRejectsOutOfRangePort(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: 70000}
	require.Error(t, ValidateTarget(target), "expected error for out-of-range port")
}

func TestTargetValidateUniqueTargetsAcceptsFanOutSameUniverseDifferentIPs(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.6"), Port: artNetPort},
	}
	require.NoError(t, ValidateUniqueTargets(targets), "expected fan-out (same universe, different IPs, D-08) to be accepted")
}

func TestTargetValidateUniqueTargetsAcceptsSameIPPortDifferentUniverses(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 2, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
	}
	require.NoError(t, ValidateUniqueTargets(targets), "expected same (IP, port) serving multiple distinct universes to be accepted")
}

func TestTargetValidateUniqueTargetsRejectsDuplicateTriple(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
	}
	require.Error(t, ValidateUniqueTargets(targets), "expected duplicate (universe, IP, port) triple to be rejected")
}

func TestTargetValidateUniqueTargetsAppliesDefaultPortToDuplicateDetection(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.5")}, // Port unspecified, defaults to artNetPort
	}
	require.Error(t, ValidateUniqueTargets(targets), "expected duplicate triple to be rejected even when one target's port is defaulted")
}

func TestTargetSetEnabledReturnsFreshSliceLeavingInputUnchanged(t *testing.T) {
	original := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true},
		{Universe: 2, IP: net.ParseIP("10.0.0.6"), Port: artNetPort, Enabled: true},
	}
	match := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}

	updated, err := SetEnabled(original, match, false)
	require.NoError(t, err)
	require.True(t, original[0].Enabled, "expected caller's original slice to remain unchanged, but it was mutated")
	require.False(t, updated[0].Enabled, "expected updated slice's matched target to be disabled")
	require.True(t, updated[1].Enabled, "expected non-matched target in the updated slice to remain enabled")
	require.Len(t, updated, len(original), "expected updated slice length to match original length")
}

func TestTargetSetEnabledReturnsNotFoundForUnmatchedTarget(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true},
	}
	missing := Target{Universe: 99, IP: net.ParseIP("10.0.0.99"), Port: artNetPort}
	_, err := SetEnabled(targets, missing, true)
	require.Error(t, err, "expected GOLC_ARTNET_TARGET_NOT_FOUND error for an unmatched target")
}
