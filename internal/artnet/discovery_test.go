// discovery_test.go proves CONTEXT D-06's discovery-scan contract
// (04-06-PLAN.md Task 2): Discover returns a real replying node as a
// suggestion (a); a malformed reply is skipped and never fails the scan
// (b, Security Domain V5 backstop); zero replies returns a well-formed,
// non-nil empty list, never an error (c, the same backstop). Every test
// overrides discoverListenAndSend (discovery.go's dialFunc-style seam)
// to stand up a real loopback UDP responder -- a genuine *net.UDPConn
// sending real datagrams over the loopback interface -- rather than
// depending on OS-level SO_BROADCAST permission semantics, which vary by
// platform and are not portably testable in a unit test.
package artnet

import (
	"context"
	"net"
	"testing"
	"time"
)

// loopbackInterfaceInfo is a minimal InterfaceInfo naming the IPv4
// loopback address/netmask, enough for localIPv4FromInterfaceInfo and
// discoveryBroadcastAddr to resolve without any real OS interface
// lookup.
func loopbackInterfaceInfo() InterfaceInfo {
	return InterfaceInfo{
		Index: 1,
		Name:  "loopback",
		Up:    true,
		Addrs: []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}},
	}
}

// withDiscoverListenAndSend overrides discoverListenAndSend for the
// duration of one test, restoring the original on cleanup.
func withDiscoverListenAndSend(t *testing.T, fn func(localIP net.IP, broadcastAddr *net.UDPAddr, pkt []byte) (*net.UDPConn, error)) {
	t.Helper()
	original := discoverListenAndSend
	discoverListenAndSend = fn
	t.Cleanup(func() { discoverListenAndSend = original })
}

// respondWith starts a real loopback "responder": after a short delay it
// dials conn's own local address and unicasts each of replies to it --
// simulating one or more nodes replying to the ArtPoll scan, using real
// UDP sockets rather than an in-process fake.
func respondWith(t *testing.T, conn *net.UDPConn, replies ...[]byte) {
	t.Helper()
	target := conn.LocalAddr().(*net.UDPAddr)
	go func() {
		time.Sleep(20 * time.Millisecond)
		responder, err := net.DialUDP("udp4", nil, target)
		if err != nil {
			return
		}
		defer responder.Close()
		for _, reply := range replies {
			_, _ = responder.Write(reply)
		}
	}()
}

// TestDiscoverReturnsGoodNodeAsSuggestion proves (a): Discover against a
// real (loopback) UDP responder returns exactly that node as a
// suggestion.
func TestDiscoverReturnsGoodNodeAsSuggestion(t *testing.T) {
	good := buildGoodArtPollReply([4]byte{10, 0, 0, 5}, "GOLC-Node", "GOLC Test Node", 0x00, 0x01, 0x03)

	withDiscoverListenAndSend(t, func(localIP net.IP, broadcastAddr *net.UDPAddr, pkt []byte) (*net.UDPConn, error) {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if err != nil {
			return nil, err
		}
		respondWith(t, conn, good)
		return conn, nil
	})

	nodes, err := Discover(context.Background(), loopbackInterfaceInfo(), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected exactly 1 discovered node, got %d: %+v", len(nodes), nodes)
	}
	if !nodes[0].IP.Equal(net.IPv4(10, 0, 0, 5)) {
		t.Fatalf("expected discovered IP 10.0.0.5, got %v", nodes[0].IP)
	}
	if nodes[0].ShortName != "GOLC-Node" {
		t.Fatalf("expected short name %q, got %q", "GOLC-Node", nodes[0].ShortName)
	}
}

// TestDiscoverSkipsMalformedReply proves (b): a malformed reply datagram
// is skipped -- it never fails the scan, and the well-formed reply that
// follows is still returned.
func TestDiscoverSkipsMalformedReply(t *testing.T) {
	good := buildGoodArtPollReply([4]byte{10, 0, 0, 6}, "N", "L", 0x00, 0x01, 0x02)
	malformed := []byte{0x01, 0x02, 0x03} // far too short to be an ArtPollReply

	withDiscoverListenAndSend(t, func(localIP net.IP, broadcastAddr *net.UDPAddr, pkt []byte) (*net.UDPConn, error) {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if err != nil {
			return nil, err
		}
		respondWith(t, conn, malformed, good)
		return conn, nil
	})

	nodes, err := Discover(context.Background(), loopbackInterfaceInfo(), 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected the malformed reply to be skipped and only the good node returned, got %d: %+v", len(nodes), nodes)
	}
	if !nodes[0].IP.Equal(net.IPv4(10, 0, 0, 6)) {
		t.Fatalf("expected discovered IP 10.0.0.6, got %v", nodes[0].IP)
	}
}

// TestDiscoverZeroRepliesReturnsEmptyList proves (c): zero replies within
// window returns a well-formed, non-nil empty list, never an error
// (Security Domain V5 backstop).
func TestDiscoverZeroRepliesReturnsEmptyList(t *testing.T) {
	withDiscoverListenAndSend(t, func(localIP net.IP, broadcastAddr *net.UDPAddr, pkt []byte) (*net.UDPConn, error) {
		return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	})

	nodes, err := Discover(context.Background(), loopbackInterfaceInfo(), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error on zero replies, got: %v", err)
	}
	if nodes == nil {
		t.Fatal("expected a non-nil empty slice, got nil")
	}
	if len(nodes) != 0 {
		t.Fatalf("expected zero discovered nodes, got %d: %+v", len(nodes), nodes)
	}
}
