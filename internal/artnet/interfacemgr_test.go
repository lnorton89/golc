package artnet

import (
	"testing"
)

// TestInterfaceListCandidateInterfacesFindsLoopback asserts
// ListCandidateInterfaces returns at least the loopback interface on the
// test host, with Index/Name/Up/Addrs populated (04-02-PLAN.md Task 1
// acceptance criteria).
func TestInterfaceListCandidateInterfacesFindsLoopback(t *testing.T) {
	ifaces, err := ListCandidateInterfaces()
	if err != nil {
		t.Fatalf("ListCandidateInterfaces returned error: %v", err)
	}
	if len(ifaces) == 0 {
		t.Fatal("expected at least one interface, got none")
	}

	foundLoopback := false
	for _, iface := range ifaces {
		if iface.Index <= 0 {
			t.Errorf("interface %q has non-positive Index %d", iface.Name, iface.Index)
		}
		for _, addr := range iface.Addrs {
			if ip := addrIP(addr); ip != nil && ip.IsLoopback() {
				foundLoopback = true
			}
		}
	}
	if !foundLoopback {
		t.Fatalf("expected at least one interface with a loopback address among %d interfaces", len(ifaces))
	}
}

// TestInterfaceManagerMarkLostTransitionsStatus asserts markLost
// transitions status to lost.
func TestInterfaceManagerMarkLostTransitionsStatus(t *testing.T) {
	m := NewInterfaceManager(1, "test")
	if got := m.Status(); got != InterfaceStatusOK {
		t.Fatalf("expected initial status %v, got %v", InterfaceStatusOK, got)
	}
	m.markLost()
	if got := m.Status(); got != InterfaceStatusLost {
		t.Fatalf("expected status %v after markLost, got %v", InterfaceStatusLost, got)
	}
	if m.Err() == nil {
		t.Fatal("expected Err() to return a GOLC_ARTNET_INTERFACE_LOST diagnostic once lost")
	}
}

// TestInterfaceManagerBogusIndexLostAfterOnePollIteration asserts an
// InterfaceManager pinned to a bogus index is reported lost by a single
// poll iteration (calling the poll body directly rather than sleeping on
// the ticker), and never re-pins itself to a different index (CONTEXT
// D-05).
func TestInterfaceManagerBogusIndexLostAfterOnePollIteration(t *testing.T) {
	const bogusIndex = 999999
	m := NewInterfaceManager(bogusIndex, "bogus-adapter")
	if got := m.Status(); got != InterfaceStatusOK {
		t.Fatalf("expected initial status %v, got %v", InterfaceStatusOK, got)
	}

	m.Check()

	if got := m.Status(); got != InterfaceStatusLost {
		t.Fatalf("expected status %v after one poll iteration against a bogus index, got %v", InterfaceStatusLost, got)
	}
	if got := m.PinnedIndex(); got != bogusIndex {
		t.Fatalf("expected PinnedIndex to remain the originally pinned bogus index %d (no auto-switch, CONTEXT D-05), got %d", bogusIndex, got)
	}
}

// TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP asserts a LocalIP/
// bind-address accessor exists and returns the pinned interface's own
// local IP.
func TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP(t *testing.T) {
	ifaces, err := ListCandidateInterfaces()
	if err != nil {
		t.Fatalf("ListCandidateInterfaces returned error: %v", err)
	}

	var loopbackIndex int
	for _, iface := range ifaces {
		for _, addr := range iface.Addrs {
			if ip := addrIP(addr); ip != nil && ip.IsLoopback() && ip.To4() != nil {
				loopbackIndex = iface.Index
			}
		}
	}
	if loopbackIndex == 0 {
		t.Skip("no IPv4 loopback interface found on this host")
	}

	m := NewInterfaceManager(loopbackIndex, "loopback")
	ip, err := m.LocalIP()
	if err != nil {
		t.Fatalf("LocalIP returned error: %v", err)
	}
	if !ip.IsLoopback() {
		t.Fatalf("expected LocalIP to return a loopback IP, got %v", ip)
	}
}

// TestInterfaceManagerLocalIPFailsForBogusIndex asserts LocalIP surfaces
// GOLC_ARTNET_INTERFACE_LOST when the pinned interface cannot be
// resolved.
func TestInterfaceManagerLocalIPFailsForBogusIndex(t *testing.T) {
	const bogusIndex = 999999
	m := NewInterfaceManager(bogusIndex, "bogus-adapter")
	if _, err := m.LocalIP(); err == nil {
		t.Fatal("expected LocalIP to fail for a bogus pinned index, got nil error")
	}
}
