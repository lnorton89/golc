// artnet_test.go proves 04-05-PLAN.md's "artnet" scope/route contract.
// TestScopeArtnet is the exact quick-test marker for scope "artnet" (test
// --quick --scope artnet): it exercises only pure, offline arg-parsing/
// registration logic -- no daemon, no network -- so the registered scope
// exits 0 offline, mirroring build_test.go's TestScopeBuildArgs
// convention. Task 1's tests prove "artnet configure"'s two-tier usage/
// domain exit-code split and that a client route with no daemon running
// returns GOLC_ARTNET_DAEMON_UNREACHABLE, all without ever dialing a real
// daemon. Task 2's tests (below) start a real artnet.Run daemon on an
// isolated per-test named pipe (mirroring internal/artnet/daemon_test.go's
// own startTestDaemon helper, built here entirely against artnet's
// exported surface) to exercise "artnet status" and "artnet target
// enable|disable" end-to-end.
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/artnet"
	artnetipc "github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// testArtnetPipeName returns a per-test, per-process, per-nanosecond-
// unique pipe path so these tests never collide with each other, with a
// real running daemon, or with internal/artnet's own concurrently-running
// package tests. Shared by Task 1's no-daemon test and Task 2's
// daemon-backed tests.
func testArtnetPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(`\\.\pipe\golc-artnet-cli-test-%s-%d-%d`, sanitized, os.Getpid(), time.Now().UnixNano())
}

// testArtnetLoopbackInterfaceIndex finds the IPv4 loopback interface's
// index on this host so startTestArtnetDaemon has a real, always-present
// interface to pin without depending on external hardware.
func testArtnetLoopbackInterfaceIndex(t *testing.T) int {
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

// minimalArtnetShowState builds the smallest show.State artnet.Run's
// internal playback.NewEngine accepts: one active, 1-bar scene at a valid
// BPM, no fixture/pool/deployment content these tests don't need.
func minimalArtnetShowState(t *testing.T) show.State {
	t.Helper()
	sc, err := scene.NewScene("Test Scene", 1)
	if err != nil {
		t.Fatalf("scene.NewScene: %v", err)
	}
	sc.Active = true
	return show.State{Scenes: []scene.Scene{sc}, Tempo: show.Tempo{BPM: 120}}
}

// startTestArtnetDaemon starts artnet.Run in a goroutine against a fresh
// cancellable context and an isolated per-test pipe name, waits for the
// listener to come up, and registers cleanup that cancels the context and
// waits for Run to return.
func startTestArtnetDaemon(t *testing.T) string {
	t.Helper()
	pipeName := testArtnetPipeName(t)
	interfaceIndex := testArtnetLoopbackInterfaceIndex(t)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- artnet.Run(ctx, artnet.Config{
			State:          minimalArtnetShowState(t),
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
		conn, err := artnetipc.Dial(pipeName)
		if err == nil {
			conn.Close()
			return pipeName
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("daemon did not come up on pipe %s", pipeName)
	return ""
}

// findArtnetTargetHealth returns the TargetHealth entry matching
// (universe, ip, port) out of payload.Targets, if any.
func findArtnetTargetHealth(payload artnetStatusPayload, universe int, ip string, port int) (artnet.TargetHealth, bool) {
	for _, th := range payload.Targets {
		if th.Universe == universe && th.Target.IP.String() == ip && displayPort(th.Target) == port {
			return th, true
		}
	}
	return artnet.TargetHealth{}, false
}

// TestScopeArtnet is the exact "test --quick --scope artnet" marker: pure
// offline coverage of arg-parsing and self-registration, no daemon/
// network required.
func TestScopeArtnet(t *testing.T) {
	t.Run("parseArtnetArgs accepts --flag value and --flag=value forms", func(t *testing.T) {
		values, err := parseArtnetArgs("usage", []string{"--universe", "1", "--ip=127.0.0.1"}, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if values["universe"] != "1" || values["ip"] != "127.0.0.1" {
			t.Fatalf("unexpected parsed values: %+v", values)
		}
	})

	t.Run("parseArtnetArgs treats a declared bool flag as valueless", func(t *testing.T) {
		values, err := parseArtnetArgs("usage", []string{"--json"}, map[string]bool{"json": true})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if values["json"] != "true" {
			t.Fatalf("expected json=true, got %+v", values)
		}
	})

	t.Run("parseArtnetArgs rejects an argument not starting with --", func(t *testing.T) {
		if _, err := parseArtnetArgs("usage", []string{"bogus"}, nil); err == nil {
			t.Fatal("expected an error for a non-flag argument")
		}
	})

	t.Run("pipeNameFromFlags defaults to the production pipe name", func(t *testing.T) {
		if got := pipeNameFromFlags(map[string]string{}); got != artnetipc.PipeName {
			t.Fatalf("expected default pipe name %q, got %q", artnetipc.PipeName, got)
		}
	})

	t.Run("the artnet scope and every route self-register", func(t *testing.T) {
		registry, err := NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}
		for _, route := range []string{
			"artnet serve", "artnet interface list", "artnet configure",
			"artnet status", "artnet target enable", "artnet target disable",
			"artnet discover",
		} {
			if _, _, ok := registry.Lookup(strings.Fields(route)); !ok {
				t.Fatalf("expected route %q to be registered", route)
			}
		}
	})
}

// TestArtnetConfigureUsageErrors proves a malformed "artnet configure"
// invocation (missing --ip) is rejected as GOLC_ARTNET_USAGE with
// ExitCode 2, without ever dialing a daemon.
func TestArtnetConfigureUsageErrors(t *testing.T) {
	result := runArtnetConfigure(Request{Args: []string{"--universe", "1"}})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", result.Stderr)
	}
}

// TestArtnetConfigureInvalidTargetReturnsDomainError proves a
// well-formed-but-rejected "artnet configure" value (universe 0) fails
// artnet.ValidateTarget client-side as a GOLC_ARTNET_TARGET_INVALID
// domain error with ExitCode 1, again without ever dialing a daemon.
func TestArtnetConfigureInvalidTargetReturnsDomainError(t *testing.T) {
	result := runArtnetConfigure(Request{Args: []string{"--universe", "0", "--ip", "10.0.0.1"}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_TARGET_INVALID") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_INVALID, got: %s", result.Stderr)
	}
}

// TestArtnetNoDaemonReturnsDaemonUnreachable proves a client route with no
// daemon running on the given pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE,
// ExitCode 1, never a hang.
func TestArtnetNoDaemonReturnsDaemonUnreachable(t *testing.T) {
	pipeName := testArtnetPipeName(t) // nothing ever listens on this pipe

	result := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "10.0.0.1", "--pipe", pipeName,
	}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected GOLC_ARTNET_DAEMON_UNREACHABLE, got: %s", result.Stderr)
	}
}

// TestArtnetStatusJSONContainsHealthFields proves "artnet status --json"
// emits canonical JSON containing frame and target health (ARTN-05).
func TestArtnetStatusJSONContainsHealthFields(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	result := runArtnetStatus(Request{Args: []string{"--json", "--pipe", pipeName}})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	body := string(result.Stdout)
	for _, want := range []string{`"frame"`, `"targets"`, "OnCadence"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected JSON status to contain %q, got: %s", want, body)
		}
	}
}

// TestArtnetStatusPlainRendersPersistentTable proves plain "artnet status"
// renders a persistent per-target status table (D-11) including a
// freshly configured target.
func TestArtnetStatusPlainRendersPersistentTable(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	configureResult := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	statusResult := runArtnetStatus(Request{Args: []string{"--pipe", pipeName}})
	if statusResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", statusResult.ExitCode, statusResult.Stderr)
	}
	body := string(statusResult.Stdout)
	if !strings.Contains(body, "GOLC_ARTNET_STATUS") {
		t.Fatalf("expected the plain status header, got: %s", body)
	}
	if !strings.Contains(body, "127.0.0.1") {
		t.Fatalf("expected the configured target's IP in the persistent table, got: %s", body)
	}
}

// TestArtnetStatusJSONContainsUniverseValues proves 04-08-PLAN.md's
// ARTN-05 gap closure: after configuring universe 1, polling "artnet
// status --json" eventually yields a "universes" entry for universe 1
// whose decoded Values field is exactly 512 bytes -- the corrected
// acceptance test that decodes and length-checks the actual bytes rather
// than asserting substring presence (directly replacing 04-05-SUMMARY.md's
// false-pass mechanism).
func TestArtnetStatusJSONContainsUniverseValues(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	configureResult := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	type universeEntry struct {
		Universe int    `json:"universe"`
		Values   []byte `json:"values"`
	}
	type jsonStatusPayload struct {
		Universes []universeEntry `json:"universes"`
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result := runArtnetStatus(Request{Args: []string{"--json", "--pipe", pipeName}})
		if result.ExitCode != 0 {
			t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
		}
		var payload jsonStatusPayload
		if err := json.Unmarshal(result.Stdout, &payload); err != nil {
			t.Fatalf("json.Unmarshal: %v", err)
		}
		for _, u := range payload.Universes {
			if u.Universe == 1 {
				if len(u.Values) != 512 {
					t.Fatalf("expected universe 1's values to be 512 bytes, got %d", len(u.Values))
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected a populated universe 1 values entry within the deadline")
}

// TestArtnetStatusPlainRendersUniverseValues proves plain "artnet status"
// eventually renders a GOLC_ARTNET_UNIVERSE line for a configured
// universe with the correct channel count.
func TestArtnetStatusPlainRendersUniverseValues(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	configureResult := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusResult := runArtnetStatus(Request{Args: []string{"--pipe", pipeName}})
		if statusResult.ExitCode != 0 {
			t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", statusResult.ExitCode, statusResult.Stderr)
		}
		body := string(statusResult.Stdout)
		if strings.Contains(body, "GOLC_ARTNET_UNIVERSE: universe=1") && strings.Contains(body, "channels=512") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected a GOLC_ARTNET_UNIVERSE line for universe=1 channels=512 within the deadline")
}

// TestArtnetStatusPlainRendersPinnedInterface proves 04-09-PLAN.md's
// ARTN-01/D-05 gap closure: plain "artnet status" renders a
// GOLC_ARTNET_INTERFACE_STATUS line naming the pinned loopback index with
// status=ok.
func TestArtnetStatusPlainRendersPinnedInterface(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)
	loopbackIdx := testArtnetLoopbackInterfaceIndex(t)

	result := runArtnetStatus(Request{Args: []string{"--pipe", pipeName}})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	body := string(result.Stdout)
	if !strings.Contains(body, fmt.Sprintf("GOLC_ARTNET_INTERFACE_STATUS: index=%d", loopbackIdx)) {
		t.Fatalf("expected GOLC_ARTNET_INTERFACE_STATUS: index=%d, got: %s", loopbackIdx, body)
	}
	if !strings.Contains(body, "status=ok") {
		t.Fatalf("expected status=ok, got: %s", body)
	}
}

// TestArtnetStatusJSONIncludesInterfaceStatus proves "artnet status --json"
// decodes an Interface field matching the pinned loopback index with
// Status "ok" (04-09-PLAN.md, ARTN-01/D-05).
func TestArtnetStatusJSONIncludesInterfaceStatus(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)
	loopbackIdx := testArtnetLoopbackInterfaceIndex(t)

	payload, errResult, ok := fetchArtnetStatus(pipeName, "")
	if !ok {
		t.Fatalf("fetchArtnetStatus failed: %+v", errResult)
	}
	if payload.Interface.PinnedIndex != loopbackIdx {
		t.Fatalf("expected Interface.PinnedIndex %d, got %d", loopbackIdx, payload.Interface.PinnedIndex)
	}
	if payload.Interface.Status != "ok" {
		t.Fatalf("expected Interface.Status \"ok\", got %q", payload.Interface.Status)
	}
}

// TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning proves
// 04-09-PLAN.md's ARTN-01/D-05 gap closure: with a daemon running,
// "artnet interface list" marks the loopback candidate as pinned with its
// live status, in both plain and --json rendering.
func TestArtnetInterfaceListAnnotatesPinnedWhenDaemonRunning(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)
	loopbackIdx := testArtnetLoopbackInterfaceIndex(t)

	plainResult := runArtnetInterfaceList(Request{Args: []string{"--pipe", pipeName}})
	if plainResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", plainResult.ExitCode, plainResult.Stderr)
	}
	plainBody := string(plainResult.Stdout)
	found := false
	for _, line := range strings.Split(plainBody, "\n") {
		if strings.HasPrefix(line, strconv.Itoa(loopbackIdx)+" ") || strings.Contains(line, fmt.Sprintf("%-6d", loopbackIdx)) {
			if strings.Contains(line, "yes") && strings.Contains(line, "ok") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected the loopback row to be marked pinned=yes status=ok, got: %s", plainBody)
	}

	jsonResult := runArtnetInterfaceList(Request{Args: []string{"--json", "--pipe", pipeName}})
	if jsonResult.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", jsonResult.ExitCode, jsonResult.Stderr)
	}
	var entries []interfaceListEntry
	if err := json.Unmarshal(jsonResult.Stdout, &entries); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	entryFound := false
	for _, e := range entries {
		if e.Index == loopbackIdx {
			if !e.Pinned || e.Status != "ok" {
				t.Fatalf("expected loopback entry Pinned=true Status=ok, got: %+v", e)
			}
			entryFound = true
		}
	}
	if !entryFound {
		t.Fatalf("expected an entry for loopback index %d, got: %+v", loopbackIdx, entries)
	}
}

// TestArtnetInterfaceListWorksWithNoDaemon proves the no-regression
// requirement (04-09-PLAN.md, ARTN-01/D-05): with no daemon listening,
// "artnet interface list" still returns ExitCode 0 with the full candidate
// list and never GOLC_ARTNET_DAEMON_UNREACHABLE.
func TestArtnetInterfaceListWorksWithNoDaemon(t *testing.T) {
	pipeName := testArtnetPipeName(t) // nothing ever listens on this pipe

	result := runArtnetInterfaceList(Request{Args: []string{"--pipe", pipeName}})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	body := string(result.Stdout)
	if strings.Contains(body, "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected no GOLC_ARTNET_DAEMON_UNREACHABLE regression, got: %s", body)
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least a header and one candidate interface row, got: %s", body)
	}
}

// TestArtnetTargetEnableDisableRoundTrip proves "artnet target disable"
// then "artnet target enable" toggle one target's visible Enabled state
// in a subsequent status without error (D-12).
func TestArtnetTargetEnableDisableRoundTrip(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	configureResult := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	disableResult := runArtnetTargetDisable(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if disableResult.ExitCode != 0 {
		t.Fatalf("expected target disable to succeed, got ExitCode %d stderr %s", disableResult.ExitCode, disableResult.Stderr)
	}

	afterDisable, errResult, ok := fetchArtnetStatus(pipeName, "")
	if !ok {
		t.Fatalf("fetchArtnetStatus after disable failed: %+v", errResult)
	}
	th, found := findArtnetTargetHealth(afterDisable, 1, "127.0.0.1", 6454)
	if !found {
		t.Fatalf("expected target 1/127.0.0.1:6454 in status, got: %+v", afterDisable)
	}
	if th.Target.Enabled {
		t.Fatal("expected the target to be disabled after 'artnet target disable'")
	}

	enableResult := runArtnetTargetEnable(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if enableResult.ExitCode != 0 {
		t.Fatalf("expected target enable to succeed, got ExitCode %d stderr %s", enableResult.ExitCode, enableResult.Stderr)
	}

	afterEnable, errResult2, ok2 := fetchArtnetStatus(pipeName, "")
	if !ok2 {
		t.Fatalf("fetchArtnetStatus after enable failed: %+v", errResult2)
	}
	th2, found2 := findArtnetTargetHealth(afterEnable, 1, "127.0.0.1", 6454)
	if !found2 {
		t.Fatalf("expected target 1/127.0.0.1:6454 in status, got: %+v", afterEnable)
	}
	if !th2.Target.Enabled {
		t.Fatal("expected the target to be enabled after 'artnet target enable'")
	}
}

// TestArtnetTargetUnknownReturnsNotFound proves an unknown target selector
// fails with GOLC_ARTNET_TARGET_NOT_FOUND, ExitCode 1.
func TestArtnetTargetUnknownReturnsNotFound(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)

	result := runArtnetTargetEnable(Request{Args: []string{
		"--universe", "99", "--ip", "10.0.0.9", "--port", "6454", "--pipe", pipeName,
	}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_TARGET_NOT_FOUND") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_NOT_FOUND, got: %s", result.Stderr)
	}
}

// TestArtnetDiscoverUsageErrors proves a missing --interface is rejected
// as GOLC_ARTNET_USAGE, ExitCode 2, without ever scanning the network.
func TestArtnetDiscoverUsageErrors(t *testing.T) {
	result := runArtnetDiscover(Request{Args: nil})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", result.Stderr)
	}
}

// TestArtnetDiscoverUnknownInterfaceReturnsNotFound proves an interface
// index that does not exist on this host is rejected as
// GOLC_ARTNET_INTERFACE_NOT_FOUND, ExitCode 1.
func TestArtnetDiscoverUnknownInterfaceReturnsNotFound(t *testing.T) {
	result := runArtnetDiscover(Request{Args: []string{"--interface", "999999"}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_INTERFACE_NOT_FOUND") {
		t.Fatalf("expected GOLC_ARTNET_INTERFACE_NOT_FOUND, got: %s", result.Stderr)
	}
}

// TestArtnetDiscoverRendersSuggestions proves a bounded "artnet discover"
// scan on the loopback interface completes and renders the suggestions
// header without error, even though nothing replies on this host (a
// well-formed empty scan, Security Domain V5 backstop).
func TestArtnetDiscoverRendersSuggestions(t *testing.T) {
	interfaceIndex := testArtnetLoopbackInterfaceIndex(t)

	result := runArtnetDiscover(Request{Args: []string{
		"--interface", strconv.Itoa(interfaceIndex), "--window", "50ms",
	}})
	if result.ExitCode != 0 {
		t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stdout), "GOLC_ARTNET_DISCOVER") {
		t.Fatalf("expected the discover suggestions header, got: %s", result.Stdout)
	}
}

// TestArtnetDiscoverPerformsNoTargetMutation proves (D-06): running
// "artnet discover" never adds, removes, or modifies a configured
// target -- the daemon's configured target set (including its Enabled
// state) is unchanged before and after.
func TestArtnetDiscoverPerformsNoTargetMutation(t *testing.T) {
	pipeName := startTestArtnetDaemon(t)
	interfaceIndex := testArtnetLoopbackInterfaceIndex(t)

	configureResult := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "127.0.0.1", "--port", "6454", "--pipe", pipeName,
	}})
	if configureResult.ExitCode != 0 {
		t.Fatalf("expected configure to succeed, got ExitCode %d stderr %s", configureResult.ExitCode, configureResult.Stderr)
	}

	before, errResult, ok := fetchArtnetStatus(pipeName, "")
	if !ok {
		t.Fatalf("fetchArtnetStatus before discover failed: %+v", errResult)
	}

	discoverResult := runArtnetDiscover(Request{Args: []string{
		"--interface", strconv.Itoa(interfaceIndex), "--window", "50ms",
	}})
	if discoverResult.ExitCode != 0 {
		t.Fatalf("expected discover ExitCode 0, got %d (stderr: %s)", discoverResult.ExitCode, discoverResult.Stderr)
	}

	after, errResult2, ok2 := fetchArtnetStatus(pipeName, "")
	if !ok2 {
		t.Fatalf("fetchArtnetStatus after discover failed: %+v", errResult2)
	}

	if len(before.Targets) != len(after.Targets) {
		t.Fatalf("expected target count unchanged, before=%d after=%d", len(before.Targets), len(after.Targets))
	}
	beforeTarget, foundBefore := findArtnetTargetHealth(before, 1, "127.0.0.1", 6454)
	afterTarget, foundAfter := findArtnetTargetHealth(after, 1, "127.0.0.1", 6454)
	if !foundBefore || !foundAfter {
		t.Fatalf("expected the configured target present before and after discover: before=%v after=%v", foundBefore, foundAfter)
	}
	if beforeTarget.Target.Enabled != afterTarget.Target.Enabled {
		t.Fatalf("expected the target's Enabled state unchanged by discover: before=%v after=%v", beforeTarget.Target.Enabled, afterTarget.Target.Enabled)
	}
}
