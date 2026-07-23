// target.go implements ARTN-02's unicast Art-Net output target model
// (04-02-PLAN.md Task 2): a Target is (Universe, IP, Port, Enabled).
// Output is strictly per-target unicast -- there is no broadcast target
// construct anywhere in this package (CONTEXT D-07); ValidateTarget
// explicitly rejects the IPv4 broadcast address so that guarantee is
// enforced at the data layer, not just by omission. A single universe may
// fan out to multiple distinct unicast targets simultaneously (CONTEXT
// D-08, e.g. redundant/backup nodes); ValidateUniqueTargets only rejects
// an exact (Universe, IP, Port) triple collision, never a shared
// Universe alone or a shared (IP, Port) serving multiple distinct
// universes. SetEnabled mirrors internal/deployment/model.go's Activate
// copy-returning discipline (CONTEXT D-12): it never mutates the
// caller's own slice.
package artnet

import (
	"fmt"
	"net"
)

// Target is one unicast Art-Net output destination (CONTEXT D-07/D-08/
// D-12). Port defaults to the fixed Art-Net UDP port (artNetPort, see
// packet.go) when left at its zero value -- effectivePort resolves this
// default consistently everywhere a Target's port is compared.
type Target struct {
	Universe int
	IP       net.IP
	Port     int
	Enabled  bool
}

// effectivePort returns t.Port, defaulting to the fixed Art-Net UDP port
// (6454) when Port is unspecified (its zero value).
func effectivePort(t Target) int {
	if t.Port == 0 {
		return artNetPort
	}
	return t.Port
}

// targetKey is the (Universe, IP, Port) identity ValidateUniqueTargets
// and SetEnabled key duplicate/match detection on -- IP is compared by
// its String() form so equivalent net.IP representations (4-byte vs.
// 16-byte) collide correctly.
type targetKey struct {
	Universe int
	IP       string
	Port     int
}

func keyOf(t Target) targetKey {
	return targetKey{Universe: t.Universe, IP: t.IP.String(), Port: effectivePort(t)}
}

// artNetMaxUniverse is the highest Universe value PortAddress's locked
// Net=0-fixed mapping (packet.go) can represent without aliasing onto a
// lower universe's Port-Address: Sub-Net (4 bits) + Universe (4 bits) = 8
// usable bits, so only 1..255 map uniquely -- PortAddress(257) and
// PortAddress(1) both mask down to the identical 0x0001 Port-Address.
const artNetMaxUniverse = 255

// ValidateTarget rejects a non-positive or out-of-range Universe, a
// nil/unspecified/broadcast IP, and a Port outside 1..65535 (after
// defaulting), each as GOLC_ARTNET_TARGET_INVALID with a specific message
// -- mirrors internal/deployment/model.go's ValidateInstanceAddress
// bounds-check-then-diagnostic shape.
func ValidateTarget(t Target) error {
	if t.Universe < 1 {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: universe %d must be at least 1", t.Universe)
	}
	if t.Universe > artNetMaxUniverse {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: universe %d exceeds the maximum representable Port-Address universe %d (Net=0-fixed mapping would alias onto a lower universe)", t.Universe, artNetMaxUniverse)
	}
	if t.IP == nil || t.IP.IsUnspecified() {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: IP must be a specific unicast address, got %v", t.IP)
	}
	if t.IP.Equal(net.IPv4bcast) {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: %v is the IPv4 broadcast address; targets are unicast-only (D-07)", t.IP)
	}
	port := effectivePort(t)
	if port < 1 || port > 65535 {
		return fmt.Errorf("GOLC_ARTNET_TARGET_INVALID: port %d is outside the valid 1-65535 range", port)
	}
	return nil
}

// ValidateUniqueTargets rejects any two targets in targets sharing the
// same (Universe, IP, Port) triple as GOLC_ARTNET_TARGET_DUPLICATE
// (mirrors internal/deployment/model.go's ValidateUniqueNames dedupe
// shape). The same Universe on multiple distinct targets (CONTEXT D-08
// fan-out) and the same (IP, Port) serving multiple distinct universes
// are both explicitly allowed.
func ValidateUniqueTargets(targets []Target) error {
	seen := make(map[targetKey]bool, len(targets))
	for _, t := range targets {
		key := keyOf(t)
		if seen[key] {
			return fmt.Errorf("GOLC_ARTNET_TARGET_DUPLICATE: universe %d, IP %s, port %d is already configured", t.Universe, t.IP, key.Port)
		}
		seen[key] = true
	}
	return nil
}

// SetEnabled returns a copy of targets with the single target matching
// match's (Universe, IP, Port) triple set to enabled, never mutating the
// caller's own targets slice (CONTEXT D-12, mirrors
// internal/deployment/model.go's Activate copy-returning discipline). It
// fails with GOLC_ARTNET_TARGET_NOT_FOUND if no target in targets
// matches match.
func SetEnabled(targets []Target, match Target, enabled bool) ([]Target, error) {
	matchKey := keyOf(match)
	found := false
	updated := make([]Target, len(targets))
	for i, t := range targets {
		if keyOf(t) == matchKey {
			t.Enabled = enabled
			found = true
		}
		updated[i] = t
	}
	if !found {
		return nil, fmt.Errorf("GOLC_ARTNET_TARGET_NOT_FOUND: no target matches universe %d, IP %s, port %d", match.Universe, match.IP, matchKey.Port)
	}
	return updated, nil
}
