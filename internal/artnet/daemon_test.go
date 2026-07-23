// daemon_test.go proves 04-04-PLAN.md Task 2's contract: Run starts
// engine+worker+interface manager+IPC listener and serves a status/health
// Request end-to-end in-process (a); an unrecognized route is rejected
// rather than silently succeeding (b); "artnet configure" and "artnet
// target enable|disable" mutate the daemon's in-memory target set through
// the stop/rebuild/start reconfigure path without error, and an unknown
// target selector fails with GOLC_ARTNET_TARGET_NOT_FOUND (c); and ctx
// cancel triggers Run to return with the worker stopped, no goroutine leak
// (d).
//
// 06-02-PLAN.md Task 2 adds: "artnet safety blackout"/"stop-all"/
// "revoke-automation" round-trip and reject a malformed --on value (e);
// while Revoke Automation is active, a "--source automation" Request is
// rejected with GOLC_ARTNET_SAFETY_REVOKED regardless of route, while a
// manual (or --source-omitting) Request still succeeds (f, PLAY-08); and
// "artnet master set" accepts --grand/--group+--level, rejecting an
// out-of-range level and a malformed invocation (g, PLAY-06).
package artnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// testDaemonPipeName returns a per-test, per-process, per-nanosecond-unique
// pipe path so this package's daemon tests never collide with each other,
// with a real running daemon, or with internal/artnet/ipc's own tests
// running concurrently in a sibling package.
func testDaemonPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(`\\.\pipe\golc-artnet-daemon-test-%s-%d-%d`, sanitized, os.Getpid(), time.Now().UnixNano())
}

// minimalPlayableState builds the smallest show.State Compile accepts: one
// active scene, all four layers left disabled with a zero Selection/Ref,
// at a valid BPM -- enough for NewEngine to succeed and publish frames,
// with no fixture/pool/deployment content this test doesn't need.
func minimalPlayableState(t *testing.T) show.State {
	t.Helper()
	sc, err := scene.NewScene("Test Scene", 1)
	if err != nil {
		t.Fatalf("scene.NewScene: %v", err)
	}
	sc.Active = true
	return show.State{Scenes: []scene.Scene{sc}, Tempo: show.Tempo{BPM: 120}}
}

// loopbackInterfaceIndex finds the IPv4 loopback interface's index on this
// host (mirrors interfacemgr_test.go's own approach) so Run has a real,
// always-present interface to pin without depending on external hardware.
func loopbackInterfaceIndex(t *testing.T) int {
	t.Helper()
	ifaces, err := ListCandidateInterfaces()
	if err != nil {
		t.Fatalf("ListCandidateInterfaces: %v", err)
	}
	for _, iface := range ifaces {
		for _, addr := range iface.Addrs {
			if ip := addrIP(addr); ip != nil && ip.IsLoopback() && ip.To4() != nil {
				return iface.Index
			}
		}
	}
	t.Skip("no IPv4 loopback interface found on this host")
	return 0
}

// startTestDaemon starts Run in a goroutine against a fresh cancellable
// context and a per-test pipe name, dials it (retrying until the listener
// is up), and registers cleanup that cancels ctx and waits for Run to
// return.
func startTestDaemon(t *testing.T) (pipeName string, runDone chan error, cancel context.CancelFunc) {
	t.Helper()
	pipeName = testDaemonPipeName(t)
	interfaceIndex := loopbackInterfaceIndex(t)

	ctx, cancelFn := context.WithCancel(context.Background())
	runDone = make(chan error, 1)
	go func() {
		runDone <- Run(ctx, Config{
			State:          minimalPlayableState(t),
			InterfaceIndex: interfaceIndex,
			InterfaceName:  "loopback",
			PipeName:       pipeName,
		})
	}()

	t.Cleanup(func() {
		cancelFn()
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return within 5s of ctx cancel")
		}
	})

	return pipeName, runDone, cancelFn
}

// dialTestDaemon dials pipeName, retrying briefly while the daemon's IPC
// listener is still starting up.
func dialTestDaemon(t *testing.T, pipeName string) net.Conn {
	t.Helper()
	var conn net.Conn
	var dialErr error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr = ipc.Dial(pipeName)
		if dialErr == nil {
			return conn
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Dial: %v", dialErr)
	return nil
}

// TestDaemonRunServesStatusAndShutsDownCleanly proves (a) and (d): Run
// serves a status Request end-to-end in-process and returns cleanly once
// ctx is cancelled (asserted via startTestDaemon's own cleanup deadline).
func TestDaemonRunServesStatusAndShutsDownCleanly(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	conn := dialTestDaemon(t, pipeName)
	defer conn.Close()

	result := ipc.Forward(conn, ipc.Request{Route: "artnet status"})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if len(result.Stdout) == 0 {
		t.Fatal("expected a non-empty health snapshot in Stdout")
	}
	if !strings.Contains(string(result.Stdout), "OnCadence") {
		t.Fatalf("expected the health snapshot JSON to mention OnCadence, got: %s", result.Stdout)
	}
}

// TestDaemonUnknownRouteReturnsRouteUnknown proves (b): the daemon's
// handler rejects a route it does not recognize rather than silently
// succeeding.
func TestDaemonUnknownRouteReturnsRouteUnknown(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	conn := dialTestDaemon(t, pipeName)
	defer conn.Close()

	result := ipc.Forward(conn, ipc.Request{Route: "artnet bogus"})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for an unknown route, got %d", result.ExitCode)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_ROUTE_UNKNOWN") {
		t.Fatalf("expected GOLC_ARTNET_ROUTE_UNKNOWN, got: %s", result.Stderr)
	}
}

// TestDaemonConfigureThenTargetDisableEnable proves (c): "artnet configure"
// adds a fan-out target and "artnet target disable" toggles it without
// error (exercising the daemon's stop/rebuild/start reconfigure path
// end-to-end), while an unknown target selector fails with
// GOLC_ARTNET_TARGET_NOT_FOUND.
func TestDaemonConfigureThenTargetDisableEnable(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	configureConn := dialTestDaemon(t, pipeName)
	defer configureConn.Close()
	configureResult := ipc.Forward(configureConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454",
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	disableConn := dialTestDaemon(t, pipeName)
	defer disableConn.Close()
	disableResult := ipc.Forward(disableConn, ipc.Request{Route: "artnet target disable", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454",
	}})
	if disableResult.ExitCode != 0 {
		t.Fatalf("expected target disable to succeed, got ExitCode %d stderr %s", disableResult.ExitCode, disableResult.Stderr)
	}

	notFoundConn := dialTestDaemon(t, pipeName)
	defer notFoundConn.Close()
	notFoundResult := ipc.Forward(notFoundConn, ipc.Request{Route: "artnet target enable", Args: []string{
		"--universe", "99", "--ip", "10.0.0.9", "--port", "6454",
	}})
	if notFoundResult.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 for an unknown target, got %d", notFoundResult.ExitCode)
	}
	if !strings.Contains(string(notFoundResult.Stderr), "GOLC_ARTNET_TARGET_NOT_FOUND") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_NOT_FOUND, got: %s", notFoundResult.Stderr)
	}
}

// TestDaemonStatusPayloadIncludesConfiguredUniverseValues proves
// 04-08-PLAN.md's ARTN-05 gap closure: after configuring universe 1, the
// daemon's status payload eventually carries a "universes" entry for
// universe 1 whose Values field decodes to exactly channelsPerUniverse
// (512) bytes -- an actual populated per-universe values field, not
// merely a JSON key (correcting 04-05-SUMMARY.md's false-pass substring
// check).
func TestDaemonStatusPayloadIncludesConfiguredUniverseValues(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	configureConn := dialTestDaemon(t, pipeName)
	defer configureConn.Close()
	configureResult := ipc.Forward(configureConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454",
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusConn := dialTestDaemon(t, pipeName)
		result := ipc.Forward(statusConn, ipc.Request{Route: "artnet status"})
		statusConn.Close()
		if result.ExitCode != 0 {
			t.Fatalf("expected status ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
		}

		var payload statusPayload
		if err := strictjson.DecodeStrict(result.Stdout, &payload); err != nil {
			t.Fatalf("DecodeStrict: %v", err)
		}
		for _, u := range payload.Universes {
			if u.Universe == 1 {
				if len(u.Values) != channelsPerUniverse {
					t.Fatalf("expected universe 1's values to be %d bytes, got %d", channelsPerUniverse, len(u.Values))
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected a populated universe 1 values entry within the deadline")
}

// TestDaemonStatusPayloadIncludesPinnedInterfaceStatus proves 04-09-PLAN.md's
// ARTN-01/D-05 gap closure: against a daemon pinned to the healthy loopback
// interface, "artnet status" reports the pinned index, status=ok, and an
// empty error.
func TestDaemonStatusPayloadIncludesPinnedInterfaceStatus(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)
	loopbackIdx := loopbackInterfaceIndex(t)

	conn := dialTestDaemon(t, pipeName)
	defer conn.Close()

	result := ipc.Forward(conn, ipc.Request{Route: "artnet status"})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}

	var payload statusPayload
	if err := strictjson.DecodeStrict(result.Stdout, &payload); err != nil {
		t.Fatalf("DecodeStrict: %v", err)
	}
	if payload.Interface.PinnedIndex != loopbackIdx {
		t.Fatalf("expected Interface.PinnedIndex %d, got %d", loopbackIdx, payload.Interface.PinnedIndex)
	}
	if payload.Interface.Status != "ok" {
		t.Fatalf("expected Interface.Status \"ok\", got %q", payload.Interface.Status)
	}
	if payload.Interface.Error != "" {
		t.Fatalf("expected empty Interface.Error, got %q", payload.Interface.Error)
	}
}

// TestDaemonStatusPayloadSurfacesLostInterface proves 04-09-PLAN.md's
// ARTN-01/D-05 gap closure for the degraded path: a daemon pinned to a
// deliberately-invalid interface index eventually reports Interface.Status
// "lost" and a GOLC_ARTNET_INTERFACE_LOST error through "artnet status" --
// the degraded state is genuinely surfaced, not just the healthy path, and
// Run tolerates the unresolvable pinned interface at startup rather than
// failing.
func TestDaemonStatusPayloadSurfacesLostInterface(t *testing.T) {
	const bogusInterfaceIndex = 999999

	pipeName := testDaemonPipeName(t)
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(ctx, Config{
			State:          minimalPlayableState(t),
			InterfaceIndex: bogusInterfaceIndex,
			InterfaceName:  "bogus",
			PipeName:       pipeName,
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return within 5s of ctx cancel")
		}
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn := dialTestDaemon(t, pipeName)
		result := ipc.Forward(conn, ipc.Request{Route: "artnet status"})
		conn.Close()
		if result.ExitCode != 0 {
			t.Fatalf("expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)
		}

		var payload statusPayload
		if err := strictjson.DecodeStrict(result.Stdout, &payload); err != nil {
			t.Fatalf("DecodeStrict: %v", err)
		}
		if payload.Interface.Status == "lost" {
			if !strings.Contains(payload.Interface.Error, "GOLC_ARTNET_INTERFACE_LOST") {
				t.Fatalf("expected Interface.Error to contain GOLC_ARTNET_INTERFACE_LOST, got %q", payload.Interface.Error)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("expected Interface.Status to become \"lost\" within the deadline")
}

// TestDaemonSafetyBlackoutRoundTrip proves 06-02-PLAN.md Task 2's "artnet
// safety blackout" route: it round-trips on and off with ExitCode 0, and a
// malformed --on value is rejected as GOLC_ARTNET_USAGE, ExitCode 2.
func TestDaemonSafetyBlackoutRoundTrip(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	onConn := dialTestDaemon(t, pipeName)
	defer onConn.Close()
	onResult := ipc.Forward(onConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "true"}})
	if onResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", onResult.ExitCode, onResult.Stderr)
	}

	offConn := dialTestDaemon(t, pipeName)
	defer offConn.Close()
	offResult := ipc.Forward(offConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "false"}})
	if offResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", offResult.ExitCode, offResult.Stderr)
	}

	malformedConn := dialTestDaemon(t, pipeName)
	defer malformedConn.Close()
	malformedResult := ipc.Forward(malformedConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "not-a-bool"}})
	if malformedResult.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d", malformedResult.ExitCode)
	}
	if !strings.Contains(string(malformedResult.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", malformedResult.Stderr)
	}
}

// TestDaemonSafetyStopAllAndRevokeAutomationRoundTrip proves "artnet
// safety stop-all" and "artnet safety revoke-automation" both round-trip
// with ExitCode 0, and that omitting --on defaults to on=true.
func TestDaemonSafetyStopAllAndRevokeAutomationRoundTrip(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	stopAllConn := dialTestDaemon(t, pipeName)
	defer stopAllConn.Close()
	stopAllResult := ipc.Forward(stopAllConn, ipc.Request{Route: "artnet safety stop-all"})
	if stopAllResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", stopAllResult.ExitCode, stopAllResult.Stderr)
	}
	if !strings.Contains(string(stopAllResult.Stdout), "on=true") {
		t.Fatalf("expected omitted --on to default to on=true, got: %s", stopAllResult.Stdout)
	}

	revokeConn := dialTestDaemon(t, pipeName)
	defer revokeConn.Close()
	revokeResult := ipc.Forward(revokeConn, ipc.Request{Route: "artnet safety revoke-automation", Args: []string{"--on", "true"}})
	if revokeResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", revokeResult.ExitCode, revokeResult.Stderr)
	}
}

// TestRevokeAutomationBlocksNonManualSource proves PLAY-08's
// daemon-side gate: while Revoke Automation is active, a Request tagged
// "--source automation" is rejected with GOLC_ARTNET_SAFETY_REVOKED
// (ExitCode 1) regardless of route, while a "--source manual" Request (or
// one that omits --source entirely, the default) still succeeds.
func TestRevokeAutomationBlocksNonManualSource(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	revokeConn := dialTestDaemon(t, pipeName)
	defer revokeConn.Close()
	revokeResult := ipc.Forward(revokeConn, ipc.Request{Route: "artnet safety revoke-automation", Args: []string{"--on", "true"}})
	if revokeResult.ExitCode != 0 {
		t.Fatalf("expected revoke-automation to succeed, got ExitCode %d (stderr: %s)", revokeResult.ExitCode, revokeResult.Stderr)
	}

	automationConn := dialTestDaemon(t, pipeName)
	defer automationConn.Close()
	automationResult := ipc.Forward(automationConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--source", "automation",
	}})
	if automationResult.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1 for an automation-sourced request while revoked, got %d (stdout: %s)", automationResult.ExitCode, automationResult.Stdout)
	}
	if !strings.Contains(string(automationResult.Stderr), "GOLC_ARTNET_SAFETY_REVOKED") {
		t.Fatalf("expected GOLC_ARTNET_SAFETY_REVOKED, got: %s", automationResult.Stderr)
	}

	manualConn := dialTestDaemon(t, pipeName)
	defer manualConn.Close()
	manualResult := ipc.Forward(manualConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--source", "manual",
	}})
	if manualResult.ExitCode != 0 {
		t.Fatalf("expected a manual-sourced request to succeed while revoked, got ExitCode %d (stderr: %s)", manualResult.ExitCode, manualResult.Stderr)
	}

	defaultSourceConn := dialTestDaemon(t, pipeName)
	defer defaultSourceConn.Close()
	defaultSourceResult := ipc.Forward(defaultSourceConn, ipc.Request{Route: "artnet status"})
	if defaultSourceResult.ExitCode != 0 {
		t.Fatalf("expected a Request with no --source (default manual) to succeed while revoked, got ExitCode %d (stderr: %s)", defaultSourceResult.ExitCode, defaultSourceResult.Stderr)
	}
}

// TestDaemonMasterSetGrandAndGroup proves "artnet master set" accepts
// --grand and --group/--level, rejects an out-of-range level as
// GOLC_ARTNET_SAFETY_MASTER_INVALID (ExitCode 1), and rejects a malformed
// invocation (neither --grand nor --group) as GOLC_ARTNET_USAGE
// (ExitCode 2).
func TestDaemonMasterSetGrandAndGroup(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	grandConn := dialTestDaemon(t, pipeName)
	defer grandConn.Close()
	grandResult := ipc.Forward(grandConn, ipc.Request{Route: "artnet master set", Args: []string{"--grand", "0.5"}})
	if grandResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", grandResult.ExitCode, grandResult.Stderr)
	}

	groupID := uuid.New()
	groupConn := dialTestDaemon(t, pipeName)
	defer groupConn.Close()
	groupResult := ipc.Forward(groupConn, ipc.Request{Route: "artnet master set", Args: []string{
		"--group", groupID.String(), "--level", "0.5",
	}})
	if groupResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", groupResult.ExitCode, groupResult.Stderr)
	}

	invalidConn := dialTestDaemon(t, pipeName)
	defer invalidConn.Close()
	invalidResult := ipc.Forward(invalidConn, ipc.Request{Route: "artnet master set", Args: []string{"--grand", "1.5"}})
	if invalidResult.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d", invalidResult.ExitCode)
	}
	if !strings.Contains(string(invalidResult.Stderr), "GOLC_ARTNET_SAFETY_MASTER_INVALID") {
		t.Fatalf("expected GOLC_ARTNET_SAFETY_MASTER_INVALID, got: %s", invalidResult.Stderr)
	}

	malformedConn := dialTestDaemon(t, pipeName)
	defer malformedConn.Close()
	malformedResult := ipc.Forward(malformedConn, ipc.Request{Route: "artnet master set"})
	if malformedResult.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d", malformedResult.ExitCode)
	}
	if !strings.Contains(string(malformedResult.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", malformedResult.Stderr)
	}
}

// TestDaemonMalformedConfigureArgsReturnUsageError proves a malformed
// "artnet configure" invocation (missing --ip) is rejected as
// GOLC_ARTNET_USAGE with ExitCode 2, mirroring this repo's two-tier
// usage/domain exit-code convention.
func TestDaemonMalformedConfigureArgsReturnUsageError(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	conn := dialTestDaemon(t, pipeName)
	defer conn.Close()

	result := ipc.Forward(conn, ipc.Request{Route: "artnet configure", Args: []string{"--universe", "1"}})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2 for a malformed configure request, got %d", result.ExitCode)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", result.Stderr)
	}
}
