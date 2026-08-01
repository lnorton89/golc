package artnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInterfaceListCandidateInterfacesFindsLoopback asserts
// ListCandidateInterfaces returns at least the loopback interface on the
// test host, with Index/Name/Up/Addrs populated (04-02-PLAN.md Task 1
// acceptance criteria).
func TestInterfaceListCandidateInterfacesFindsLoopback(t *testing.T) {
	ifaces, err := ListCandidateInterfaces()
	require.NoError(t, err, "ListCandidateInterfaces returned error")
	require.NotEmpty(t, ifaces, "expected at least one interface, got none")

	foundLoopback := false
	for _, iface := range ifaces {
		assert.Greaterf(t, iface.Index, 0, "interface %q has non-positive Index %d", iface.Name, iface.Index)
		for _, addr := range iface.Addrs {
			if ip := addrIP(addr); ip != nil && ip.IsLoopback() {
				foundLoopback = true
			}
		}
	}
	require.Truef(t, foundLoopback, "expected at least one interface with a loopback address among %d interfaces", len(ifaces))
}

// TestInterfaceManagerMarkLostTransitionsStatus asserts markLost
// transitions status to lost.
func TestInterfaceManagerMarkLostTransitionsStatus(t *testing.T) {
	m := NewInterfaceManager(1, "test")
	require.Equalf(t, InterfaceStatusOK, m.Status(), "expected initial status %v, got %v", InterfaceStatusOK, m.Status())
	m.markLost()
	require.Equalf(t, InterfaceStatusLost, m.Status(), "expected status %v after markLost, got %v", InterfaceStatusLost, m.Status())
	require.NotNil(t, m.Err(), "expected Err() to return a GOLC_ARTNET_INTERFACE_LOST diagnostic once lost")
}

// TestInterfaceManagerBogusIndexLostAfterOnePollIteration asserts an
// InterfaceManager pinned to a bogus index is reported lost by a single
// poll iteration (calling the poll body directly rather than sleeping on
// the ticker), and never re-pins itself to a different index (CONTEXT
// D-05).
func TestInterfaceManagerBogusIndexLostAfterOnePollIteration(t *testing.T) {
	const bogusIndex = 999999
	m := NewInterfaceManager(bogusIndex, "bogus-adapter")
	require.Equalf(t, InterfaceStatusOK, m.Status(), "expected initial status %v, got %v", InterfaceStatusOK, m.Status())

	m.Check()

	require.Equalf(t, InterfaceStatusLost, m.Status(), "expected status %v after one poll iteration against a bogus index, got %v", InterfaceStatusLost, m.Status())
	require.Equalf(t, bogusIndex, m.PinnedIndex(), "expected PinnedIndex to remain the originally pinned bogus index %d (no auto-switch, CONTEXT D-05), got %d", bogusIndex, m.PinnedIndex())
}

// TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP asserts a LocalIP/
// bind-address accessor exists and returns the pinned interface's own
// local IP.
func TestInterfaceManagerLocalIPReturnsPinnedInterfaceIP(t *testing.T) {
	ifaces, err := ListCandidateInterfaces()
	require.NoError(t, err, "ListCandidateInterfaces returned error")

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
	require.NoError(t, err, "LocalIP returned error")
	require.Truef(t, ip.IsLoopback(), "expected LocalIP to return a loopback IP, got %v", ip)
}

// TestInterfaceManagerLocalIPFailsForBogusIndex asserts LocalIP surfaces
// GOLC_ARTNET_INTERFACE_LOST when the pinned interface cannot be
// resolved.
func TestInterfaceManagerLocalIPFailsForBogusIndex(t *testing.T) {
	const bogusIndex = 999999
	m := NewInterfaceManager(bogusIndex, "bogus-adapter")
	_, err := m.LocalIP()
	require.Error(t, err, "expected LocalIP to fail for a bogus pinned index, got nil error")
}
