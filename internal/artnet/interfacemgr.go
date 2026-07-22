// interfacemgr.go implements ARTN-01's Windows network-interface
// enumeration and CONTEXT D-05's pinned-by-index loss detection (04-02-
// PLAN.md Task 1, 04-RESEARCH.md Pattern 2, 04-PATTERNS.md Pitfall 4):
// ListCandidateInterfaces surfaces every OS network interface as an
// InterfaceInfo for operator selection (ARTN-01). InterfaceManager pins
// the operator's chosen interface by its stable net.Interface.Index --
// never by Name, since Windows can report GUID-shaped/unstable interface
// names (04-PATTERNS.md Pitfall 4) -- and re-resolves by that same index
// on its own independent 1Hz poll loop (mirroring internal/playback/
// engine.go's context.WithCancel/ticker/goroutine lifecycle). When the
// pinned interface disappears or goes down, pollInterfaceLoss calls
// markLost, which flips a concurrency-safe status flag to
// InterfaceStatusLost; no code path here ever selects a different
// interface index automatically (CONTEXT D-05: loss is terminal-until-
// reconfigured, not auto-recovered onto another NIC). LocalIP returns the
// pinned interface's own local unicast IPv4 address so Plan 03's worker
// can bind net.ListenUDP/net.DialUDP to that specific local address --
// Windows has no SO_BINDTODEVICE equivalent (04-RESEARCH.md Pitfall 5).
package artnet

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// interfacePollInterval is the interface-loss poll cadence: 1Hz is
// adequate for surfacing a "clear degraded state" (CONTEXT D-05) and
// runs independently of the Art-Net worker's own 40Hz send ticker
// (04-RESEARCH.md Pattern 2).
const interfacePollInterval = time.Second

// InterfaceInfo describes one OS network interface as a candidate for
// Art-Net output selection (ARTN-01): Index and Name identify it (Index
// is the stable identity -- see 04-PATTERNS.md Pitfall 4 -- Name is
// display-only), Up reports its current link state, and Addrs carries
// its currently assigned addresses.
type InterfaceInfo struct {
	Index int
	Name  string
	Up    bool
	Addrs []net.Addr
}

// ListCandidateInterfaces enumerates every OS network interface as a
// candidate for Art-Net output selection (ARTN-01, 04-RESEARCH.md
// Pattern 2). Enumeration failure is wrapped as
// GOLC_ARTNET_INTERFACE_ENUM_FAILED.
func ListCandidateInterfaces() ([]InterfaceInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_ENUM_FAILED: %v", err)
	}
	out := make([]InterfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		out = append(out, InterfaceInfo{
			Index: iface.Index,
			Name:  iface.Name,
			Up:    iface.Flags&net.FlagUp != 0,
			Addrs: addrs,
		})
	}
	return out, nil
}

// InterfaceStatus is InterfaceManager's readable health state.
type InterfaceStatus int32

const (
	// InterfaceStatusOK reports the pinned interface was last seen
	// present and up.
	InterfaceStatusOK InterfaceStatus = iota
	// InterfaceStatusLost reports the pinned interface disappeared or
	// went down (CONTEXT D-05): output must stop, and no code here ever
	// selects a different interface index to recover automatically.
	InterfaceStatusLost
)

// String renders status for logging/CLI display.
func (s InterfaceStatus) String() string {
	switch s {
	case InterfaceStatusOK:
		return "ok"
	case InterfaceStatusLost:
		return "lost"
	default:
		return "unknown"
	}
}

// InterfaceManager pins one operator-selected network interface by its
// stable net.Interface.Index (CONTEXT D-05, 04-PATTERNS.md Pitfall 4) and
// detects when it disappears or goes down via its own independent poll
// loop, mirroring internal/playback/engine.go's context.WithCancel/
// ticker/goroutine lifecycle. status is stored via atomic.Int32 so any
// concurrent goroutine (CLI/IPC status handler) can read it without
// locking.
type InterfaceManager struct {
	pinnedIndex int
	pinnedName  string

	status atomic.Int32
	cancel context.CancelFunc
}

// NewInterfaceManager pins index (the durable identity, CONTEXT D-05)
// and name (display-only) and returns a manager whose initial status is
// InterfaceStatusOK. It does not itself validate that index currently
// resolves to a live interface -- call Start (or a direct poll iteration
// via the exported Check method) to establish the first health reading.
func NewInterfaceManager(index int, name string) *InterfaceManager {
	m := &InterfaceManager{pinnedIndex: index, pinnedName: name}
	m.status.Store(int32(InterfaceStatusOK))
	return m
}

// PinnedIndex returns the durable net.Interface.Index this manager is
// pinned to (CONTEXT D-05: identity is never re-derived from Name).
func (m *InterfaceManager) PinnedIndex() int {
	return m.pinnedIndex
}

// PinnedName returns the display-only name captured at pin time -- never
// used to re-resolve the interface (04-PATTERNS.md Pitfall 4).
func (m *InterfaceManager) PinnedName() string {
	return m.pinnedName
}

// Status returns the manager's current health reading. Safe to call from
// any goroutine concurrently with Start's poll loop.
func (m *InterfaceManager) Status() InterfaceStatus {
	return InterfaceStatus(m.status.Load())
}

// Err returns nil when Status is InterfaceStatusOK, or a
// GOLC_ARTNET_INTERFACE_LOST diagnostic identifying the pinned index
// otherwise (CONTEXT D-11 diagnostic convention).
func (m *InterfaceManager) Err() error {
	if m.Status() == InterfaceStatusLost {
		return fmt.Errorf("GOLC_ARTNET_INTERFACE_LOST: pinned interface index %d (%q) is no longer present or is down", m.pinnedIndex, m.pinnedName)
	}
	return nil
}

// markLost flips status to InterfaceStatusLost. It never selects or
// switches to a different interface index (CONTEXT D-05) -- loss is
// terminal-until-reconfigured.
func (m *InterfaceManager) markLost() {
	m.status.Store(int32(InterfaceStatusLost))
}

// Check re-resolves the pinned interface by index and marks it lost if
// it can no longer be resolved or is no longer up. This is the poll
// loop's body, exposed as its own method so callers (and tests) can
// trigger a single check iteration directly rather than waiting on the
// ticker (04-02-PLAN.md Task 1 acceptance criteria).
func (m *InterfaceManager) Check() {
	iface, err := net.InterfaceByIndex(m.pinnedIndex)
	if err != nil {
		m.markLost()
		return
	}
	if iface.Flags&net.FlagUp == 0 {
		m.markLost()
	}
}

// pollInterfaceLoss re-checks the pinned interface on its own 1Hz
// ticker, independent of the Art-Net worker's 40Hz send ticker
// (04-RESEARCH.md Pattern 2), until ctx is cancelled.
func (m *InterfaceManager) pollInterfaceLoss(ctx context.Context) {
	ticker := time.NewTicker(interfacePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Check()
		}
	}
}

// Start begins the interface-loss poll loop in its own goroutine,
// mirroring internal/playback/engine.go's Start(ctx)/Stop() lifecycle.
// The caller owns the InterfaceManager's lifecycle: a single owner
// starts and later stops one manager.
func (m *InterfaceManager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	go m.pollInterfaceLoss(ctx)
}

// Stop cancels the context Start derived, terminating the poll goroutine
// cleanly. Calling Stop before Start (or more than once) is a safe
// no-op.
func (m *InterfaceManager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// LocalIP returns the pinned interface's own local unicast IPv4 address
// so the worker (Plan 03) can bind net.ListenUDP/net.DialUDP to that
// specific local address rather than 0.0.0.0 or a Linux-only device-bind
// socket option -- Windows has no SO_BINDTODEVICE equivalent
// (04-RESEARCH.md Pitfall 5). It re-resolves the interface by index at
// call time and fails with GOLC_ARTNET_INTERFACE_LOST if the interface
// is gone or carries no usable IPv4 address.
func (m *InterfaceManager) LocalIP() (net.IP, error) {
	iface, err := net.InterfaceByIndex(m.pinnedIndex)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_LOST: pinned interface index %d: %v", m.pinnedIndex, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_LOST: pinned interface index %d: %v", m.pinnedIndex, err)
	}
	for _, addr := range addrs {
		if ip := addrIP(addr); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				return ip4, nil
			}
		}
	}
	return nil, fmt.Errorf("GOLC_ARTNET_INTERFACE_LOST: pinned interface index %d has no usable IPv4 unicast address", m.pinnedIndex)
}

// addrIP extracts the net.IP from a net.Addr returned by
// net.Interface.Addrs(), which is always a *net.IPNet in practice (a
// *net.IPAddr fallback is handled defensively).
func addrIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	}
	return nil
}
