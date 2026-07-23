package artnet

import (
	"net"
	"testing"
)

func TestTargetValidateTargetAcceptsValidTarget(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	if err := ValidateTarget(target); err != nil {
		t.Fatalf("expected valid target to pass, got error: %v", err)
	}
}

func TestTargetValidateTargetDefaultsUnspecifiedPort(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5")}
	if err := ValidateTarget(target); err != nil {
		t.Fatalf("expected target with unspecified port to default to %d and pass, got error: %v", artNetPort, err)
	}
}

func TestTargetValidateTargetRejectsNonPositiveUniverse(t *testing.T) {
	target := Target{Universe: 0, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
	if err := ValidateTarget(target); err == nil {
		t.Fatal("expected error for non-positive universe, got nil")
	}
}

func TestTargetValidateTargetRejectsUniverseAboveMaxRepresentable(t *testing.T) {
	for _, universe := range []int{256, 257} {
		target := Target{Universe: universe, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
		if err := ValidateTarget(target); err == nil {
			t.Fatalf("expected error for universe %d (exceeds artNetMaxUniverse=%d, would alias onto a lower universe's Port-Address), got nil", universe, artNetMaxUniverse)
		}
	}
}

func TestTargetValidateTargetAcceptsMaxRepresentableUniverse(t *testing.T) {
	target := Target{Universe: artNetMaxUniverse, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}
	if err := ValidateTarget(target); err != nil {
		t.Fatalf("expected universe %d (the maximum representable) to pass, got error: %v", artNetMaxUniverse, err)
	}
}

func TestTargetValidateTargetRejectsNilIP(t *testing.T) {
	target := Target{Universe: 1, Port: artNetPort}
	if err := ValidateTarget(target); err == nil {
		t.Fatal("expected error for nil IP, got nil")
	}
}

func TestTargetValidateTargetRejectsUnspecifiedIP(t *testing.T) {
	target := Target{Universe: 1, IP: net.IPv4zero, Port: artNetPort}
	if err := ValidateTarget(target); err == nil {
		t.Fatal("expected error for unspecified (0.0.0.0) IP, got nil")
	}
}

func TestTargetValidateTargetRejectsBroadcastIP(t *testing.T) {
	target := Target{Universe: 1, IP: net.IPv4bcast, Port: artNetPort}
	if err := ValidateTarget(target); err == nil {
		t.Fatal("expected error for the IPv4 broadcast address (D-07 unicast-only), got nil")
	}
}

func TestTargetValidateTargetRejectsOutOfRangePort(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: 70000}
	if err := ValidateTarget(target); err == nil {
		t.Fatal("expected error for out-of-range port, got nil")
	}
}

func TestTargetValidateUniqueTargetsAcceptsFanOutSameUniverseDifferentIPs(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.6"), Port: artNetPort},
	}
	if err := ValidateUniqueTargets(targets); err != nil {
		t.Fatalf("expected fan-out (same universe, different IPs, D-08) to be accepted, got error: %v", err)
	}
}

func TestTargetValidateUniqueTargetsAcceptsSameIPPortDifferentUniverses(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 2, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
	}
	if err := ValidateUniqueTargets(targets); err != nil {
		t.Fatalf("expected same (IP, port) serving multiple distinct universes to be accepted, got error: %v", err)
	}
}

func TestTargetValidateUniqueTargetsRejectsDuplicateTriple(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
	}
	if err := ValidateUniqueTargets(targets); err == nil {
		t.Fatal("expected duplicate (universe, IP, port) triple to be rejected, got nil")
	}
}

func TestTargetValidateUniqueTargetsAppliesDefaultPortToDuplicateDetection(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort},
		{Universe: 1, IP: net.ParseIP("10.0.0.5")}, // Port unspecified, defaults to artNetPort
	}
	if err := ValidateUniqueTargets(targets); err == nil {
		t.Fatal("expected duplicate triple to be rejected even when one target's port is defaulted, got nil")
	}
}

func TestTargetSetEnabledReturnsFreshSliceLeavingInputUnchanged(t *testing.T) {
	original := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true},
		{Universe: 2, IP: net.ParseIP("10.0.0.6"), Port: artNetPort, Enabled: true},
	}
	match := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort}

	updated, err := SetEnabled(original, match, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !original[0].Enabled {
		t.Fatal("expected caller's original slice to remain unchanged, but it was mutated")
	}
	if updated[0].Enabled {
		t.Fatal("expected updated slice's matched target to be disabled")
	}
	if !updated[1].Enabled {
		t.Fatal("expected non-matched target in the updated slice to remain enabled")
	}
	if len(updated) != len(original) {
		t.Fatalf("expected updated slice length %d to match original length %d", len(updated), len(original))
	}
}

func TestTargetSetEnabledReturnsNotFoundForUnmatchedTarget(t *testing.T) {
	targets := []Target{
		{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true},
	}
	missing := Target{Universe: 99, IP: net.ParseIP("10.0.0.99"), Port: artNetPort}
	if _, err := SetEnabled(targets, missing, true); err == nil {
		t.Fatal("expected GOLC_ARTNET_TARGET_NOT_FOUND error for an unmatched target, got nil")
	}
}
