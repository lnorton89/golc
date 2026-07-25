// config_test.go covers 07-03-PLAN.md Task 2's must_haves: loopback is the
// enforced-at-bind default regardless of api.bind_interface unless
// api.remote_enabled is explicitly true, and enabling remote access with
// no explicit interface fails Start loudly rather than ever silently
// binding 0.0.0.0 or silently falling back to loopback (07-RESEARCH.md
// Pitfall 4). listenAddr is exercised directly (no real socket bind) for
// the address-derivation cases so this suite never depends on the host's
// firewall/loopback-vs-non-loopback bind behavior; TestRemoteRequiresInterface
// additionally proves through Start itself that no listener is ever opened
// when the interface is missing.
//
// This file lives in the internal api package (not api_test) so it can
// call the unexported listenAddr method directly.
package api

import (
	"context"
	"strings"
	"testing"
)

// stubExecutorForConfigTest is a minimal Executor sufficient to construct a
// *Server for these tests -- none of them dispatch a real request.
type stubExecutorForConfigTest struct{}

func (stubExecutorForConfigTest) Execute(route string, args []string, root string) (int, []byte, []byte) {
	return 0, nil, nil
}

// TestLoopbackDefault proves remote_enabled=false (and the Config zero
// value, which is also false) always resolves to a loopback listen
// address, even when BindInterface names a non-empty non-loopback value.
func TestLoopbackDefault(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "remote disabled with a bind interface set",
			cfg:  Config{RemoteEnabled: false, BindInterface: "10.0.0.5", Port: 8080},
		},
		{
			name: "zero-value config (remote_enabled unset)",
			cfg:  Config{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(stubExecutorForConfigTest{}, "/repo/root", "/repo/root/show.golc", WithConfig(tc.cfg))
			addr, err := server.listenAddr()
			if err != nil {
				t.Fatalf("listenAddr failed: %v", err)
			}
			if !strings.HasPrefix(addr, "127.0.0.1:") {
				t.Fatalf("expected a loopback address, got %q", addr)
			}
		})
	}
}

// TestRemoteRequiresInterface proves remote_enabled=true with an empty
// bind_interface fails Start with GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED,
// and that no listener is opened as a result (s.httpServer stays nil,
// proving net.Listen was never reached).
func TestRemoteRequiresInterface(t *testing.T) {
	server := NewServer(stubExecutorForConfigTest{}, "/repo/root", "/repo/root/show.golc",
		WithConfig(Config{RemoteEnabled: true, BindInterface: "", Port: 4591}))

	err := server.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when remote_enabled is true with an empty bind_interface")
	}
	if !strings.Contains(err.Error(), "GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED") {
		t.Fatalf("expected GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED, got %v", err)
	}
	if server.httpServer != nil {
		t.Fatal("expected no listener to be opened when Start fails before net.Listen")
	}
}

// TestBindAddress proves remote_enabled=true with an explicit
// bind_interface derives that exact address, not loopback and not a
// silently-substituted 0.0.0.0.
func TestBindAddress(t *testing.T) {
	server := NewServer(stubExecutorForConfigTest{}, "/repo/root", "/repo/root/show.golc",
		WithConfig(Config{RemoteEnabled: true, BindInterface: "0.0.0.0", Port: 4592}))

	addr, err := server.listenAddr()
	if err != nil {
		t.Fatalf("listenAddr failed: %v", err)
	}
	if addr != "0.0.0.0:4592" {
		t.Fatalf("expected listen addr \"0.0.0.0:4592\", got %q", addr)
	}
}

// TestListenAddrDefaultsPortWhenUnset proves a zero Config.Port falls back
// to defaultPort rather than binding port 0.
func TestListenAddrDefaultsPortWhenUnset(t *testing.T) {
	server := NewServer(stubExecutorForConfigTest{}, "/repo/root", "/repo/root/show.golc")
	addr, err := server.listenAddr()
	if err != nil {
		t.Fatalf("listenAddr failed: %v", err)
	}
	want := "127.0.0.1:4590"
	if addr != want {
		t.Fatalf("expected default listen addr %q, got %q", want, addr)
	}
}
