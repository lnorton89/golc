// discovery.go implements CONTEXT D-06's optional node discovery scan
// (04-06-PLAN.md Task 2, 04-RESEARCH.md Pattern 4): Discover broadcasts
// one ArtPoll (packet.go's EncodeArtPoll) on the given interface's local
// subnet broadcast address, then collects ArtPollReply datagrams
// (packet.go's DecodeArtPollReply) for a bounded window, returning every
// distinct replying node as a DiscoveredNode -- a suggestion only.
// Nothing here ever adds, removes, or modifies a live unicast target
// (CONTEXT D-06); promoting a discovered node to an active target
// remains a separate, explicit internal/command/artnet.go operator
// action ("artnet configure"). This ArtPoll broadcast is the opt-in
// discovery scan; it does not conflict with CONTEXT D-07's no-broadcast
// rule for the live DMX *output* path (04-RESEARCH.md Pattern 4's own
// doc comment).
//
// A malformed or truncated ArtPollReply is skipped (DecodeArtPollReply's
// own GOLC_ARTNET_POLLREPLY_INVALID error), never failing the scan
// (Security Domain V5/T-04-01: untrusted network input from an
// arbitrary, possibly spoofed device). Zero replies within window is not
// an error either -- Discover always returns a well-formed, non-nil
// (possibly empty) slice (the backstop behavior this plan's must_haves
// require).
//
// discoverListenAndSend is a package-level seam (mirrors worker.go's
// dialFunc/artNetSender pattern): production wiring opens a real
// net.ListenUDP-backed conn and best-effort broadcasts the ArtPoll (a
// send failure here does not fail Discover -- the collect loop below
// still runs for the full window and returns a well-formed, possibly
// empty list either way); discovery_test.go overrides this var with a
// real loopback UDP conn wired directly to an in-test responder, so
// ArtPollReply collection is proven against a real UDP socket without
// depending on OS-level SO_BROADCAST permission semantics, which vary by
// platform and are not portably testable in a unit test.
package artnet

import (
	"context"
	"fmt"
	"net"
	"time"
)

// defaultDiscoveryWindow is Discover's default reply-collection window
// when the caller passes window<=0 -- the Art-Net spec's own guidance
// that a controller may assume a maximum 3s reply timeout
// (04-RESEARCH.md Pattern 4).
const defaultDiscoveryWindow = 3 * time.Second

// artPollReplyReadBufferSize bounds every ReadFromUDP call Discover
// makes: ample headroom over artPollReplyMinLen (239 bytes) for a real
// ArtPollReply, while still bounding memory regardless of what an
// untrusted replying device sends (Security Domain V5).
const artPollReplyReadBufferSize = 1024

// DiscoveredNode is one node Discover found replying to an ArtPoll scan
// (CONTEXT D-06): a suggestion only, never automatically promoted to a
// live unicast target.
type DiscoveredNode struct {
	IP            net.IP
	ShortName     string
	LongName      string
	PortAddresses []uint16
}

// discoverListenAndSend opens localIP's UDP conn and best-effort
// broadcasts pkt to broadcastAddr, returning the conn Discover then reads
// ArtPollReply datagrams from. See package doc comment for why this is a
// package-level seam.
var discoverListenAndSend = func(localIP net.IP, broadcastAddr *net.UDPAddr, pkt []byte) (*net.UDPConn, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_DISCOVERY_LISTEN_FAILED: %v", err)
	}
	// Best-effort: a broadcast-permission or network error here does not
	// fail Discover -- the caller still collects for the full window and
	// returns a well-formed (possibly empty) list either way.
	_, _ = conn.WriteToUDP(pkt, broadcastAddr)
	return conn, nil
}

// localIPv4FromInterfaceInfo returns iface's own local unicast IPv4
// address to bind Discover's UDP conn to, mirroring
// InterfaceManager.LocalIP's use of addrIP (interfacemgr.go, same
// package).
func localIPv4FromInterfaceInfo(iface InterfaceInfo) (net.IP, error) {
	for _, addr := range iface.Addrs {
		if ip := addrIP(addr); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				return ip4, nil
			}
		}
	}
	return nil, fmt.Errorf("GOLC_ARTNET_DISCOVERY_NO_LOCAL_IP: interface %q (index %d) has no usable IPv4 unicast address", iface.Name, iface.Index)
}

// discoveryBroadcastAddr computes iface's own local subnet broadcast
// address (its IPNet's IP with every host bit set) so the ArtPoll
// broadcast stays scoped to this interface's own subnet rather than the
// wider Art-Net well-known broadcast address; falls back to the limited
// broadcast address (255.255.255.255) if no usable IPv4 netmask is
// found.
func discoveryBroadcastAddr(iface InterfaceInfo) net.IP {
	for _, addr := range iface.Addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil || len(ipNet.Mask) != net.IPv4len {
			continue
		}
		bcast := make(net.IP, net.IPv4len)
		for i := range ip4 {
			bcast[i] = ip4[i] | ^ipNet.Mask[i]
		}
		return bcast
	}
	return net.IPv4bcast
}

// Discover broadcasts one ArtPoll on iface's local subnet and collects
// ArtPollReply responses for window (defaulting to
// defaultDiscoveryWindow when window<=0), returning every distinct
// replying node (keyed by IP) as a suggestion-only DiscoveredNode
// (CONTEXT D-06). A malformed reply is skipped, never failing the scan;
// zero replies returns a well-formed, non-nil empty slice, never an
// error -- both are the required backstop behavior for untrusted,
// possibly-absent network responses (Security Domain V5).
func Discover(ctx context.Context, iface InterfaceInfo, window time.Duration) ([]DiscoveredNode, error) {
	if window <= 0 {
		window = defaultDiscoveryWindow
	}

	localIP, err := localIPv4FromInterfaceInfo(iface)
	if err != nil {
		return nil, err
	}

	broadcastAddr := &net.UDPAddr{IP: discoveryBroadcastAddr(iface), Port: artNetPort}
	conn, err := discoverListenAndSend(localIP, broadcastAddr, EncodeArtPoll())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(window)); err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_DISCOVERY_DEADLINE_FAILED: %v", err)
	}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close() // unblocks a pending ReadFromUDP on ctx cancel
		case <-stop:
		}
	}()

	byIP := map[string]DiscoveredNode{}
	var order []string
	buf := make([]byte, artPollReplyReadBufferSize)
	for {
		n, _, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			break // window elapsed, ctx cancelled, or conn closed
		}
		reply, decodeErr := DecodeArtPollReply(buf[:n])
		if decodeErr != nil {
			continue // malformed reply is skipped, never fails the scan
		}
		key := reply.IP.String()
		if _, seen := byIP[key]; !seen {
			order = append(order, key)
		}
		byIP[key] = DiscoveredNode{
			IP:            reply.IP,
			ShortName:     reply.ShortName,
			LongName:      reply.LongName,
			PortAddresses: reply.PortAddresses,
		}
	}

	nodes := make([]DiscoveredNode, 0, len(order))
	for _, key := range order {
		nodes = append(nodes, byIP[key])
	}
	return nodes, nil
}
