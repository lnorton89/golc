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
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	return platformTestEndpoint(t, "daemon")
}

func platformTestEndpoint(t *testing.T, prefix string) string {
	t.Helper()
	nameHash := sha256.Sum256([]byte(t.Name()))
	suffix := fmt.Sprintf("%s-%d-%d-%x", prefix, os.Getpid(), time.Now().UnixNano(), nameHash[:4])
	if runtime.GOOS == "windows" {
		return `\\.\pipe\golc-` + suffix
	}
	dir := filepath.Join("/tmp", "golc-"+suffix)
	endpoint := filepath.Join(dir, "artnet.sock")
	t.Cleanup(func() {
		_ = os.Remove(endpoint)
		_ = os.Remove(dir)
	})
	return endpoint
}

// minimalPlayableState builds the smallest show.State Compile accepts: one
// active scene, all four layers left disabled with a zero Selection/Ref,
// at a valid BPM -- enough for NewEngine to succeed and publish frames,
// with no fixture/pool/deployment content this test doesn't need.
func minimalPlayableState(t *testing.T) show.State {
	t.Helper()
	sc, err := scene.NewScene("Test Scene", 1)
	require.NoError(t, err, "scene.NewScene")
	sc.Active = true
	return show.State{Scenes: []scene.Scene{sc}, Tempo: show.Tempo{BPM: 120}}
}

// loopbackInterfaceIndex finds the IPv4 loopback interface's index on this
// host (mirrors interfacemgr_test.go's own approach) so Run has a real,
// always-present interface to pin without depending on external hardware.
func loopbackInterfaceIndex(t *testing.T) int {
	t.Helper()
	ifaces, err := ListCandidateInterfaces()
	require.NoError(t, err, "ListCandidateInterfaces")
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
			require.Fail(t, "Run did not return within 5s of ctx cancel")
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
	require.NoError(t, dialErr, "Dial")
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
	require.Equalf(t, 0, result.ExitCode, "expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	require.NotEmpty(t, result.Stdout, "expected a non-empty health snapshot in Stdout")
	require.Contains(t, string(result.Stdout), "OnCadence", "expected the health snapshot JSON to mention OnCadence")
}

// TestDaemonUnknownRouteReturnsRouteUnknown proves (b): the daemon's
// handler rejects a route it does not recognize rather than silently
// succeeding.
func TestDaemonUnknownRouteReturnsRouteUnknown(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	conn := dialTestDaemon(t, pipeName)
	defer conn.Close()

	result := ipc.Forward(conn, ipc.Request{Route: "artnet bogus"})
	require.Equalf(t, 2, result.ExitCode, "expected ExitCode 2 for an unknown route, got %d", result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_ARTNET_ROUTE_UNKNOWN")
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
	require.Equalf(t, 0, configureResult.ExitCode, "expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)

	disableConn := dialTestDaemon(t, pipeName)
	defer disableConn.Close()
	disableResult := ipc.Forward(disableConn, ipc.Request{Route: "artnet target disable", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454",
	}})
	require.Equalf(t, 0, disableResult.ExitCode, "expected target disable to succeed, got ExitCode %d stderr %s", disableResult.ExitCode, disableResult.Stderr)

	notFoundConn := dialTestDaemon(t, pipeName)
	defer notFoundConn.Close()
	notFoundResult := ipc.Forward(notFoundConn, ipc.Request{Route: "artnet target enable", Args: []string{
		"--universe", "99", "--ip", "10.0.0.9", "--port", "6454",
	}})
	require.Equalf(t, 1, notFoundResult.ExitCode, "expected ExitCode 1 for an unknown target, got %d", notFoundResult.ExitCode)
	require.Contains(t, string(notFoundResult.Stderr), "GOLC_ARTNET_TARGET_NOT_FOUND")
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
	require.Equalf(t, 0, configureResult.ExitCode, "expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusConn := dialTestDaemon(t, pipeName)
		result := ipc.Forward(statusConn, ipc.Request{Route: "artnet status"})
		statusConn.Close()
		require.Equalf(t, 0, result.ExitCode, "expected status ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)

		var payload statusPayload
		require.NoError(t, strictjson.DecodeStrict(result.Stdout, &payload), "DecodeStrict")
		for _, u := range payload.Universes {
			if u.Universe == 1 {
				require.Lenf(t, u.Values, channelsPerUniverse, "expected universe 1's values to be %d bytes, got %d", channelsPerUniverse, len(u.Values))
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Fail(t, "expected a populated universe 1 values entry within the deadline")
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
	require.Equalf(t, 0, result.ExitCode, "expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)

	var payload statusPayload
	require.NoError(t, strictjson.DecodeStrict(result.Stdout, &payload), "DecodeStrict")
	require.Equalf(t, loopbackIdx, payload.Interface.PinnedIndex, "expected Interface.PinnedIndex %d, got %d", loopbackIdx, payload.Interface.PinnedIndex)
	require.Equalf(t, "ok", payload.Interface.Status, "expected Interface.Status \"ok\", got %q", payload.Interface.Status)
	require.Emptyf(t, payload.Interface.Error, "expected empty Interface.Error, got %q", payload.Interface.Error)
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
			require.Fail(t, "Run did not return within 5s of ctx cancel")
		}
	})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn := dialTestDaemon(t, pipeName)
		result := ipc.Forward(conn, ipc.Request{Route: "artnet status"})
		conn.Close()
		require.Equalf(t, 0, result.ExitCode, "expected ExitCode 0 from status, got %d (stderr: %s)", result.ExitCode, result.Stderr)

		var payload statusPayload
		require.NoError(t, strictjson.DecodeStrict(result.Stdout, &payload), "DecodeStrict")
		if payload.Interface.Status == "lost" {
			require.Containsf(t, payload.Interface.Error, "GOLC_ARTNET_INTERFACE_LOST", "expected Interface.Error to contain GOLC_ARTNET_INTERFACE_LOST, got %q", payload.Interface.Error)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Fail(t, "expected Interface.Status to become \"lost\" within the deadline")
}

// TestDaemonSafetyBlackoutRoundTrip proves 06-02-PLAN.md Task 2's "artnet
// safety blackout" route: it round-trips on and off with ExitCode 0, and a
// malformed --on value is rejected as GOLC_ARTNET_USAGE, ExitCode 2.
func TestDaemonSafetyBlackoutRoundTrip(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	onConn := dialTestDaemon(t, pipeName)
	defer onConn.Close()
	onResult := ipc.Forward(onConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "true"}})
	require.Equalf(t, 0, onResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", onResult.ExitCode, onResult.Stderr)

	offConn := dialTestDaemon(t, pipeName)
	defer offConn.Close()
	offResult := ipc.Forward(offConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "false"}})
	require.Equalf(t, 0, offResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", offResult.ExitCode, offResult.Stderr)

	malformedConn := dialTestDaemon(t, pipeName)
	defer malformedConn.Close()
	malformedResult := ipc.Forward(malformedConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "not-a-bool"}})
	require.Equalf(t, 2, malformedResult.ExitCode, "expected ExitCode 2, got %d", malformedResult.ExitCode)
	require.Contains(t, string(malformedResult.Stderr), "GOLC_ARTNET_USAGE")
}

// TestDaemonSafetyStopAllAndRevokeAutomationRoundTrip proves "artnet
// safety stop-all" and "artnet safety revoke-automation" both round-trip
// with ExitCode 0, and that omitting --on defaults to on=true.
func TestDaemonSafetyStopAllAndRevokeAutomationRoundTrip(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	stopAllConn := dialTestDaemon(t, pipeName)
	defer stopAllConn.Close()
	stopAllResult := ipc.Forward(stopAllConn, ipc.Request{Route: "artnet safety stop-all"})
	require.Equalf(t, 0, stopAllResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", stopAllResult.ExitCode, stopAllResult.Stderr)
	require.Contains(t, string(stopAllResult.Stdout), "on=true", "expected omitted --on to default to on=true")

	revokeConn := dialTestDaemon(t, pipeName)
	defer revokeConn.Close()
	revokeResult := ipc.Forward(revokeConn, ipc.Request{Route: "artnet safety revoke-automation", Args: []string{"--on", "true"}})
	require.Equalf(t, 0, revokeResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", revokeResult.ExitCode, revokeResult.Stderr)
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
	require.Equalf(t, 0, revokeResult.ExitCode, "expected revoke-automation to succeed, got ExitCode %d (stderr: %s)", revokeResult.ExitCode, revokeResult.Stderr)

	automationConn := dialTestDaemon(t, pipeName)
	defer automationConn.Close()
	automationResult := ipc.Forward(automationConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--source", "automation",
	}})
	require.Equalf(t, 1, automationResult.ExitCode, "expected ExitCode 1 for an automation-sourced request while revoked, got %d (stdout: %s)", automationResult.ExitCode, automationResult.Stdout)
	require.Contains(t, string(automationResult.Stderr), "GOLC_ARTNET_SAFETY_REVOKED")

	manualConn := dialTestDaemon(t, pipeName)
	defer manualConn.Close()
	manualResult := ipc.Forward(manualConn, ipc.Request{Route: "artnet configure", Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--source", "manual",
	}})
	require.Equalf(t, 0, manualResult.ExitCode, "expected a manual-sourced request to succeed while revoked, got ExitCode %d (stderr: %s)", manualResult.ExitCode, manualResult.Stderr)

	defaultSourceConn := dialTestDaemon(t, pipeName)
	defer defaultSourceConn.Close()
	defaultSourceResult := ipc.Forward(defaultSourceConn, ipc.Request{Route: "artnet status"})
	require.Equalf(t, 0, defaultSourceResult.ExitCode, "expected a Request with no --source (default manual) to succeed while revoked, got ExitCode %d (stderr: %s)", defaultSourceResult.ExitCode, defaultSourceResult.Stderr)
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
	require.Equalf(t, 0, grandResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", grandResult.ExitCode, grandResult.Stderr)

	groupID := uuid.New()
	groupConn := dialTestDaemon(t, pipeName)
	defer groupConn.Close()
	groupResult := ipc.Forward(groupConn, ipc.Request{Route: "artnet master set", Args: []string{
		"--group", groupID.String(), "--level", "0.5",
	}})
	require.Equalf(t, 0, groupResult.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", groupResult.ExitCode, groupResult.Stderr)

	invalidConn := dialTestDaemon(t, pipeName)
	defer invalidConn.Close()
	invalidResult := ipc.Forward(invalidConn, ipc.Request{Route: "artnet master set", Args: []string{"--grand", "1.5"}})
	require.Equalf(t, 1, invalidResult.ExitCode, "expected ExitCode 1, got %d", invalidResult.ExitCode)
	require.Contains(t, string(invalidResult.Stderr), "GOLC_ARTNET_SAFETY_MASTER_INVALID")

	malformedConn := dialTestDaemon(t, pipeName)
	defer malformedConn.Close()
	malformedResult := ipc.Forward(malformedConn, ipc.Request{Route: "artnet master set"})
	require.Equalf(t, 2, malformedResult.ExitCode, "expected ExitCode 2, got %d", malformedResult.ExitCode)
	require.Contains(t, string(malformedResult.Stderr), "GOLC_ARTNET_USAGE")
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
	require.Equalf(t, 2, result.ExitCode, "expected ExitCode 2 for a malformed configure request, got %d", result.ExitCode)
	require.Contains(t, string(result.Stderr), "GOLC_ARTNET_USAGE")
}

// TestDaemonStatusPayloadIncludesPlaybackFields proves 06-05-PLAN.md Task
// 1's daemon-side wiring: a real daemon running minimalPlayableState's
// scene reports it as the "artnet status" payload's Playback.SceneName,
// with the scene's BPM and an explicit (never nil) EnabledLayers slice,
// and that activating Blackout flips both ControllingSource and
// OutputState to "blackout" on the very next status read -- the same
// daemon-resident safety state (06-02-PLAN.md) the on-screen safety
// cluster and OS-level hotkeys both drive.
func TestDaemonStatusPayloadIncludesPlaybackFields(t *testing.T) {
	pipeName, _, _ := startTestDaemon(t)

	statusConn := dialTestDaemon(t, pipeName)
	result := ipc.Forward(statusConn, ipc.Request{Route: "artnet status"})
	statusConn.Close()
	require.Equalf(t, 0, result.ExitCode, "expected status ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)

	var payload statusPayload
	require.NoError(t, strictjson.DecodeStrict(result.Stdout, &payload), "DecodeStrict")
	require.True(t, payload.Playback.Active, "expected Playback.Active=true against a daemon running a valid active-scene state")
	require.Equalf(t, "Test Scene", payload.Playback.SceneName, "Playback.SceneName = %q, want %q", payload.Playback.SceneName, "Test Scene")
	require.EqualValuesf(t, 120, payload.Playback.BPM, "Playback.BPM = %v, want 120", payload.Playback.BPM)
	require.NotNil(t, payload.Playback.EnabledLayers, "expected a non-nil (never null) EnabledLayers slice")
	require.Equalf(t, "live", payload.Playback.ControllingSource, "Playback.ControllingSource = %q, want %q before any override is active", payload.Playback.ControllingSource, "live")
	require.NotEmpty(t, payload.Playback.OutputState, "expected a non-empty OutputState")

	blackoutConn := dialTestDaemon(t, pipeName)
	blackoutResult := ipc.Forward(blackoutConn, ipc.Request{Route: "artnet safety blackout", Args: []string{"--on", "true"}})
	blackoutConn.Close()
	require.Equalf(t, 0, blackoutResult.ExitCode, "expected blackout toggle to succeed, got ExitCode %d stderr %s", blackoutResult.ExitCode, blackoutResult.Stderr)

	statusConn2 := dialTestDaemon(t, pipeName)
	result2 := ipc.Forward(statusConn2, ipc.Request{Route: "artnet status"})
	statusConn2.Close()
	require.Equalf(t, 0, result2.ExitCode, "expected status ExitCode 0 after blackout, got %d (stderr: %s)", result2.ExitCode, result2.Stderr)
	var payload2 statusPayload
	require.NoError(t, strictjson.DecodeStrict(result2.Stdout, &payload2), "DecodeStrict (after blackout)")
	require.Equalf(t, "blackout", payload2.Playback.ControllingSource, "Playback.ControllingSource after blackout = %q, want %q", payload2.Playback.ControllingSource, "blackout")
	require.Equalf(t, "blackout", payload2.Playback.OutputState, "Playback.OutputState after blackout = %q, want %q", payload2.Playback.OutputState, "blackout")
}

// TestNewPlaybackStatusPayloadIdleWhenNoActivePlan proves the pure
// transform's PLAY-07 idle edge directly (no real Engine/daemon
// required): a zero-value playbackEngineSnapshot (nil plan) yields
// Active=false, a non-nil-but-empty EnabledLayers slice, and explicit
// (never blank) ControllingSource/OutputState values -- never a
// zero-valued payload indistinguishable from "no data."
func TestNewPlaybackStatusPayloadIdleWhenNoActivePlan(t *testing.T) {
	payload := newPlaybackStatusPayload(playbackEngineSnapshot{}, nil, FrameHealth{OnCadence: true})

	require.False(t, payload.Active, "expected Active=false for a nil plan")
	require.NotNilf(t, payload.EnabledLayers, "expected a non-nil, empty EnabledLayers slice, got %#v", payload.EnabledLayers)
	require.Lenf(t, payload.EnabledLayers, 0, "expected a non-nil, empty EnabledLayers slice, got %#v", payload.EnabledLayers)
	require.Equalf(t, "live", payload.ControllingSource, "ControllingSource = %q, want %q (no override active)", payload.ControllingSource, "live")
	require.Equalf(t, "frame-lock", payload.OutputState, "OutputState = %q, want %q (on-cadence frame health, no override)", payload.OutputState, "frame-lock")
	require.Truef(t, payload.SceneID == "" && payload.SceneName == "", "expected empty SceneID/SceneName for the idle payload, got %q/%q", payload.SceneID, payload.SceneName)
}

// fakeSubsystem is a minimal Subsystem whose Start/Shutdown calls append
// to a shared, test-owned log -- used to prove 07-02-PLAN.md Task 2's
// ordering contract without depending on any real hosted component (e.g.
// the versioned external control API's HTTP server, which this package
// must never import).
type fakeSubsystem struct {
	name     string
	startErr error
	log      *[]string
	mu       *sync.Mutex
}

func (f *fakeSubsystem) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.log = append(*f.log, "start:"+f.name)
	return f.startErr
}

func (f *fakeSubsystem) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.log = append(*f.log, "shutdown:"+f.name)
	return nil
}

// TestSubsystemsStartAfterListenerAndStopInReverseOrder proves 07-02-
// PLAN.md Task 2's D-07 ordering contract end-to-end: two Subsystems
// start (in registration order) only once the IPC listener is already
// answering requests, and stop (in reverse order) on ctx cancellation.
func TestSubsystemsStartAfterListenerAndStopInReverseOrder(t *testing.T) {
	var mu sync.Mutex
	var log []string
	one := &fakeSubsystem{name: "one", log: &log, mu: &mu}
	two := &fakeSubsystem{name: "two", log: &log, mu: &mu}

	pipeName := testDaemonPipeName(t)
	interfaceIndex := loopbackInterfaceIndex(t)
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(ctx, Config{
			State:          minimalPlayableState(t),
			InterfaceIndex: interfaceIndex,
			InterfaceName:  "loopback",
			PipeName:       pipeName,
			Subsystems:     []Subsystem{one, two},
		})
	}()

	// Proves subsystems start only after the IPC listener is already
	// answering requests: by the time this dial+status round-trip
	// succeeds, the listener must be up, and both Start calls must
	// already be recorded (Run calls startSubsystems synchronously,
	// before ipc.Serve begins accepting).
	conn := dialTestDaemon(t, pipeName)
	result := ipc.Forward(conn, ipc.Request{Route: "artnet status"})
	conn.Close()
	require.Equalf(t, 0, result.ExitCode, "expected status ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)

	mu.Lock()
	started := append([]string(nil), log...)
	mu.Unlock()
	require.Equalf(t, []string{"start:one", "start:two"}, started, "expected subsystems to start in order once the listener answers, got %v", started)

	cancel()
	select {
	case err := <-runDone:
		require.NoError(t, err, "Run returned an error on clean shutdown")
	case <-time.After(5 * time.Second):
		require.Fail(t, "Run did not return within 5s of ctx cancel")
	}

	mu.Lock()
	final := append([]string(nil), log...)
	mu.Unlock()
	want := []string{"start:one", "start:two", "shutdown:two", "shutdown:one"}
	require.Equalf(t, want, final, "expected subsystems to start in order and stop in reverse order, got %v, want %v", final, want)
}

// TestSubsystemStartFailureUnwindsAlreadyStartedSubsystems proves the
// partial-startup-failure path: when a later Subsystem's Start fails,
// Run shuts down exactly the subsystems that already started (in
// reverse), never calls Shutdown on the one that failed to start, and
// still returns a GOLC_ARTNET_DAEMON_SUBSYSTEM_START_FAILED error rather
// than leaving the daemon half-running.
func TestSubsystemStartFailureUnwindsAlreadyStartedSubsystems(t *testing.T) {
	var mu sync.Mutex
	var log []string
	one := &fakeSubsystem{name: "one", log: &log, mu: &mu}
	two := &fakeSubsystem{name: "two", log: &log, mu: &mu, startErr: errors.New("boom")}

	pipeName := testDaemonPipeName(t)
	interfaceIndex := loopbackInterfaceIndex(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := Run(ctx, Config{
		State:          minimalPlayableState(t),
		InterfaceIndex: interfaceIndex,
		InterfaceName:  "loopback",
		PipeName:       pipeName,
		Subsystems:     []Subsystem{one, two},
	})
	require.Error(t, err, "expected Run to return an error when a subsystem fails to start")
	require.ErrorContains(t, err, "GOLC_ARTNET_DAEMON_SUBSYSTEM_START_FAILED")

	want := []string{"start:one", "start:two", "shutdown:one"}
	require.Equalf(t, want, log, "expected exactly the already-started subsystem to be shut down, got %v, want %v", log, want)

	_, dialErr := ipc.Dial(pipeName)
	require.Error(t, dialErr, "expected the IPC listener to have been closed after the subsystem start failure")
}
