// svc_artnetconfig_test.go proves 06-11-PLAN.md Task 1's three acceptance
// criteria for ArtnetConfigService (VERIFICATION.md Gap B[0], PLAY-11):
//
//   - TestArtnetConfigServiceRejectsMalformedTargetBeforeForward proves a
//     malformed/out-of-range target is rejected by the "artnet configure"
//     route's own validation before any daemon round trip -- asserted by
//     the absence of GOLC_ARTNET_DAEMON_UNREACHABLE in the failing
//     Result's stderr, since ArtnetConfigService's Configure/EnableTarget/
//     DisableTarget/FetchArtnetStatus/ListInterfaces methods dispatch
//     through the in-process command registry (svc_playback.go's execute
//     pattern, per this task's own read_first) rather than dialing the
//     daemon directly, so the exact same validate-before-forward
//     discipline internal/command/artnet.go's runArtnetConfigure already
//     has is exercised for free -- no seam/mock needed to prove it.
//   - TestArtnetConfigServiceOfflineWhenDaemonUnreachable proves
//     Configure/FetchArtnetStatus/ListInterfaces against an unreachable
//     daemon never return a partial/blank success: Configure surfaces
//     GOLC_ARTNET_DAEMON_UNREACHABLE, FetchArtnetStatus returns the
//     explicit offline projection (Reachable=false), and ListInterfaces
//     still succeeds (OS-level enumeration never dials the daemon) but
//     reports no interface as Pinned.
//   - TestArtnetConfigServiceConfigureThenStatusRoundTrip proves the real
//     happy path end-to-end against a genuine artnet.Run daemon: harness
//     path taken -- internal/command/artnet_test.go's own
//     startTestArtnetDaemon/testArtnetPipeName helpers are unexported in
//     a different package (internal/command), so this file declares its
//     own equivalent copies (startTestArtnetConfigDaemon and friends,
//     below) built against the identical exported surface
//     (artnet.Run/artnet.Config, artnet.ListCandidateInterfaces,
//     artnetipc.Dial) rather than a fake/injected seam -- the same
//     approach internal/command/artnet_test.go itself uses, mirrored here
//     rather than reused verbatim (Go has no cross-package _test.go
//     import). Configure -> FetchArtnetStatus -> EnableTarget/
//     DisableTarget round-trips against this real daemon.
package wails

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/artnet"
	artnetipc "github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// testArtnetConfigPipeName returns a per-test, per-process, per-nanosecond-
// unique pipe path so these tests never collide with each other, with a
// real running daemon, or with any other package's own concurrently-
// running tests. Also doubles as "a pipe nothing is listening on" for the
// offline/unreachable tests.
func testArtnetConfigPipeName(t *testing.T) string {
	t.Helper()
	return platformTestEndpoint(t, "config")
}

// testArtnetConfigLoopbackInterfaceIndex finds the IPv4 loopback
// interface's index on this host so startTestArtnetConfigDaemon has a
// real, always-present interface to pin without depending on external
// hardware (mirrors internal/command/artnet_test.go's identical helper).
func testArtnetConfigLoopbackInterfaceIndex(t *testing.T) int {
	t.Helper()
	ifaces, err := artnet.ListCandidateInterfaces()
	if err != nil {
		t.Fatalf("ListCandidateInterfaces: %v", err)
	}
	for _, iface := range ifaces {
		for _, addr := range iface.Addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				return iface.Index
			}
		}
	}
	t.Skip("no IPv4 loopback interface found on this host")
	return 0
}

// startTestArtnetConfigDaemon starts a real artnet.Run daemon in a
// goroutine against a fresh cancellable context and an isolated per-test
// pipe name, waits for the listener to come up, and registers cleanup
// that cancels the context and waits for Run to return (mirrors
// internal/command/artnet_test.go's startTestArtnetDaemon).
func startTestArtnetConfigDaemon(t *testing.T) string {
	t.Helper()
	pipeName := testArtnetConfigPipeName(t)
	interfaceIndex := testArtnetConfigLoopbackInterfaceIndex(t)

	sc, err := scene.NewScene("Test Scene", 1)
	if err != nil {
		t.Fatalf("scene.NewScene: %v", err)
	}
	sc.Active = true
	state := show.State{Scenes: []scene.Scene{sc}, Tempo: show.Tempo{BPM: 120}}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- artnet.Run(ctx, artnet.Config{
			State:          state,
			InterfaceIndex: interfaceIndex,
			InterfaceName:  "loopback",
			PipeName:       pipeName,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Fatal("artnet.Run did not return within 5s of ctx cancel")
		}
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := artnetipc.Dial(pipeName)
		if dialErr == nil {
			conn.Close()
			return pipeName
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("daemon did not come up on pipe %s", pipeName)
	return ""
}

// TestArtnetConfigServiceRejectsMalformedTargetBeforeForward proves
// Configure with an out-of-range universe is rejected by the route's own
// artnet.ValidateTarget check before any daemon forward: the pipe named
// here has nothing listening on it, so if Configure ever reached
// forwardToDaemon it would surface GOLC_ARTNET_DAEMON_UNREACHABLE instead
// of the domain validation error -- this test asserts that never happens.
func TestArtnetConfigServiceRejectsMalformedTargetBeforeForward(t *testing.T) {
	svc := NewArtnetConfigService(testArtnetConfigPipeName(t), t.TempDir())

	got := svc.Configure(0, "10.0.0.1", 0, true)

	if got.ExitCode == 0 {
		t.Fatal("expected Configure to reject an out-of-range universe")
	}
	if strings.Contains(got.Stderr, "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected validation to reject before any daemon forward, got a daemon-unreachable result: %s", got.Stderr)
	}
	if !strings.Contains(got.Stderr, "GOLC_ARTNET_TARGET_INVALID") && !strings.Contains(got.Stderr, "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_INVALID or GOLC_ARTNET_USAGE, got: %s", got.Stderr)
	}
}

// TestArtnetConfigServiceOfflineWhenDaemonUnreachable proves Configure,
// FetchArtnetStatus, and ListInterfaces each degrade explicitly (never a
// partial/blank success) when no daemon is reachable at pipeName.
func TestArtnetConfigServiceOfflineWhenDaemonUnreachable(t *testing.T) {
	pipeName := testArtnetConfigPipeName(t) // nothing ever listens on this pipe
	svc := NewArtnetConfigService(pipeName, t.TempDir())

	configureResult := svc.Configure(1, "10.0.0.1", 0, true)
	if configureResult.ExitCode == 0 {
		t.Fatal("expected Configure against an unreachable daemon to fail")
	}
	if !strings.Contains(configureResult.Stderr, "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected GOLC_ARTNET_DAEMON_UNREACHABLE, got: %s", configureResult.Stderr)
	}

	status := svc.FetchArtnetStatus()
	if status.Reachable {
		t.Fatal("expected FetchArtnetStatus to report Reachable=false when the daemon cannot be reached")
	}
	if status.Targets == nil {
		t.Fatal("expected a non-nil (possibly empty) Targets slice in the offline projection")
	}

	interfaces, err := svc.ListInterfaces()
	if err != nil {
		t.Fatalf("expected ListInterfaces to succeed offline (OS-level enumeration never dials the daemon): %v", err)
	}
	if interfaces == nil {
		t.Fatal("expected a non-nil (possibly empty) interfaces slice")
	}
	for _, iface := range interfaces {
		if iface.Pinned {
			t.Fatalf("expected no interface to be marked pinned when the daemon is unreachable, got %+v", iface)
		}
	}
}

// TestArtnetConfigServiceConfigureThenStatusRoundTrip proves the real
// happy path against a genuine artnet.Run daemon: Configure a target,
// observe it in FetchArtnetStatus, then flip it via DisableTarget/
// EnableTarget and observe the flag change.
func TestArtnetConfigServiceConfigureThenStatusRoundTrip(t *testing.T) {
	pipeName := startTestArtnetConfigDaemon(t)
	svc := NewArtnetConfigService(pipeName, t.TempDir())

	configureResult := svc.Configure(1, "127.0.0.1", 6454, true)
	if configureResult.ExitCode != 0 {
		t.Fatalf("Configure failed: exit=%d stderr=%s", configureResult.ExitCode, configureResult.Stderr)
	}

	status := svc.FetchArtnetStatus()
	if !status.Reachable {
		t.Fatal("expected FetchArtnetStatus to report Reachable=true against a running daemon")
	}
	assertTargetEnabled(t, status, 1, "127.0.0.1", true)

	disableResult := svc.DisableTarget(1, "127.0.0.1", 6454)
	if disableResult.ExitCode != 0 {
		t.Fatalf("DisableTarget failed: exit=%d stderr=%s", disableResult.ExitCode, disableResult.Stderr)
	}
	assertTargetEnabled(t, svc.FetchArtnetStatus(), 1, "127.0.0.1", false)

	enableResult := svc.EnableTarget(1, "127.0.0.1", 6454)
	if enableResult.ExitCode != 0 {
		t.Fatalf("EnableTarget failed: exit=%d stderr=%s", enableResult.ExitCode, enableResult.Stderr)
	}
	assertTargetEnabled(t, svc.FetchArtnetStatus(), 1, "127.0.0.1", true)
}

func assertTargetEnabled(t *testing.T, status ArtnetStatusView, universe int, ip string, wantEnabled bool) {
	t.Helper()
	for _, target := range status.Targets {
		if target.Universe == universe && target.IP == ip {
			if target.Enabled != wantEnabled {
				t.Fatalf("target %d/%s Enabled = %v, want %v", universe, ip, target.Enabled, wantEnabled)
			}
			return
		}
	}
	t.Fatalf("expected status to contain target %d/%s, got: %+v", universe, ip, status.Targets)
}

// TestArtnetConfigServiceStatusOfflineProjection (Task 3) proves
// FetchArtnetStatus's offline projection is explicit and never nil-sliced
// even in isolation from the other offline assertions above.
func TestArtnetConfigServiceStatusOfflineProjection(t *testing.T) {
	svc := NewArtnetConfigService(testArtnetConfigPipeName(t), t.TempDir())

	status := svc.FetchArtnetStatus()

	if status.Reachable {
		t.Fatal("expected the offline projection to report Reachable=false")
	}
	if status.Targets == nil {
		t.Fatal("expected a non-nil (possibly empty) Targets slice in the offline projection")
	}
}

// TestArtnetConfigServiceRejectsOutOfRangePort (Task 3) proves an
// out-of-range port is rejected by artnet.ValidateTarget's own domain
// check before any daemon forward, exactly like the out-of-range
// universe case above.
func TestArtnetConfigServiceRejectsOutOfRangePort(t *testing.T) {
	svc := NewArtnetConfigService(testArtnetConfigPipeName(t), t.TempDir())

	got := svc.Configure(1, "10.0.0.1", 70000, true)

	if got.ExitCode == 0 {
		t.Fatal("expected Configure to reject an out-of-range port")
	}
	if strings.Contains(got.Stderr, "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected the port to be rejected before any daemon forward, got: %s", got.Stderr)
	}
	if !strings.Contains(got.Stderr, "GOLC_ARTNET_TARGET_INVALID") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_INVALID for an out-of-range port, got: %s", got.Stderr)
	}
}
