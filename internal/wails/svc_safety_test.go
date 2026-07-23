// svc_safety_test.go proves 06-05-PLAN.md Task 1's two acceptance
// criteria: each SafetyService safety binding (Blackout/StopReleaseAll/
// RevokeAutomation) forwards its exact daemon route with "--source
// manual" (TestSafetyService*), and FetchStatus's StatusSnapshot
// projection is an explicit idle/offline value -- never a blank/zero one
// a caller has to guess the meaning of -- both when the daemon reports no
// active plan and when the daemon cannot be reached at all
// (TestStatusPayload*). TestSafetyServiceAuthorize* prove CR-01's fix:
// once an active operator surface is set (SetActiveSurface), a safety
// toggle against a control not assigned to it is rejected before ever
// dialing the daemon, and a toggle against an assigned control still
// dispatches exactly as before.
package wails

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lnorton89/golc/internal/artnet/ipc"
)

// TestSafetyServiceBlackoutForwardsManualSource proves Blackout(true)
// dials+forwards "artnet safety blackout --on true --source manual" --
// the identical route+args shape hotkey.go's OS-level callback uses
// (RESEARCH.md Pitfall 1: two independent triggers into one daemon
// override state).
func TestSafetyServiceBlackoutForwardsManualSource(t *testing.T) {
	var captured ipc.Request
	var capturedPipe string
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		capturedPipe = pipeName
		captured = request
		return ipc.Result{}
	}

	svc.Blackout(true)

	if capturedPipe != "test-pipe" {
		t.Fatalf("dialed pipe = %q, want %q", capturedPipe, "test-pipe")
	}
	assertSafetyForward(t, captured, "artnet safety blackout", []string{"--on", "true", "--source", "manual"})
}

// TestSafetyServiceStopReleaseAllForwardsManualSource mirrors the
// Blackout test for StopReleaseAll(false).
func TestSafetyServiceStopReleaseAllForwardsManualSource(t *testing.T) {
	var captured ipc.Request
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		captured = request
		return ipc.Result{}
	}

	svc.StopReleaseAll(false)

	assertSafetyForward(t, captured, "artnet safety stop-all", []string{"--on", "false", "--source", "manual"})
}

// TestSafetyServiceRevokeAutomationForwardsManualSource mirrors the
// Blackout test for RevokeAutomation(true) -- crucially still tagged
// "--source manual" so an on-screen Revoke Automation press is never
// itself blocked by the revoke it is about to activate.
func TestSafetyServiceRevokeAutomationForwardsManualSource(t *testing.T) {
	var captured ipc.Request
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		captured = request
		return ipc.Result{}
	}

	svc.RevokeAutomation(true)

	assertSafetyForward(t, captured, "artnet safety revoke-automation", []string{"--on", "true", "--source", "manual"})
}

// TestSafetyServiceBlackoutRejectsWhenActiveSurfaceDoesNotAssignControl
// proves CR-01's fix: once SetActiveSurface has scoped SafetyService to a
// surface that does not have Blackout in its SafetyRefs, Blackout is
// rejected with GOLC_OPERATORSURFACE_LOCKED and never reaches dial.
func TestSafetyServiceBlackoutRejectsWhenActiveSurfaceDoesNotAssignControl(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	svc := NewSafetyService("test-pipe", root, showPath)
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		t.Fatal("dial must never be reached when authorization rejects the call")
		return ipc.Result{}
	}

	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	got := svc.Blackout(true)
	if got.ExitCode == 0 {
		t.Fatal("expected Blackout to be rejected when the active surface has no Blackout SafetyRef assigned")
	}
	if !strings.Contains(got.Stderr, "GOLC_OPERATORSURFACE_LOCKED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_LOCKED in stderr, got %q", got.Stderr)
	}
}

// TestSafetyServiceBlackoutDispatchesWhenActiveSurfaceAssignsControl
// proves the counterpart: once Blackout is assigned to the active surface,
// the call authorizes and dispatches exactly as before CR-01.
func TestSafetyServiceBlackoutDispatchesWhenActiveSurfaceAssignsControl(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := surfaceSvc.AssignItem("Operator A", ControlRefInput{Kind: "safety", Safety: "blackout"}); result.ExitCode != 0 {
		t.Fatalf("AssignItem failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	svc := NewSafetyService("test-pipe", root, showPath)
	var captured ipc.Request
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		captured = request
		return ipc.Result{}
	}

	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	got := svc.Blackout(true)
	if got.ExitCode != 0 {
		t.Fatalf("expected Blackout to dispatch once assigned, got exit=%d stderr=%s", got.ExitCode, got.Stderr)
	}
	assertSafetyForward(t, captured, "artnet safety blackout", []string{"--on", "true", "--source", "manual"})
}

// TestSafetyServiceSetActiveSurfaceEmptyClearsRestriction proves
// SetActiveSurface("") always returns to unrestricted/author-mode
// dispatch, even after a prior SetActiveSurface locked the service to a
// surface that did not assign the control under test.
func TestSafetyServiceSetActiveSurfaceEmptyClearsRestriction(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	svc := NewSafetyService("test-pipe", root, showPath)
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{}
	}

	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.SetActiveSurface(""); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface(\"\") failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	got := svc.Blackout(true)
	if got.ExitCode != 0 {
		t.Fatalf("expected Blackout to dispatch after the active surface was cleared, got exit=%d stderr=%s", got.ExitCode, got.Stderr)
	}
}

func assertSafetyForward(t *testing.T, got ipc.Request, wantRoute string, wantArgs []string) {
	t.Helper()
	if got.Route != wantRoute {
		t.Fatalf("forwarded route = %q, want %q", got.Route, wantRoute)
	}
	if len(got.Args) != len(wantArgs) {
		t.Fatalf("forwarded args = %v, want %v", got.Args, wantArgs)
	}
	for i := range wantArgs {
		if got.Args[i] != wantArgs[i] {
			t.Fatalf("forwarded args = %v, want %v", got.Args, wantArgs)
		}
	}
}

// TestSafetyServiceToggleSurfacesResultShape proves toggle's ipc.Result ->
// Result conversion never silently drops a non-zero ExitCode/Stderr --
// the frontend's hold-to-confirm control must be able to distinguish
// success from a rejected/failed daemon call.
func TestSafetyServiceToggleSurfacesResultShape(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{ExitCode: 1, Stderr: []byte("GOLC_ARTNET_SAFETY_REVOKED: rejected")}
	}

	got := svc.Blackout(true)
	if got.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", got.ExitCode)
	}
	if got.Stderr != "GOLC_ARTNET_SAFETY_REVOKED: rejected" {
		t.Fatalf("Stderr = %q, want the daemon's exact diagnostic", got.Stderr)
	}
}

// daemonStatusJSON builds a minimal "artnet status" JSON response body
// carrying only the "playback" member FetchStatus reads -- a real daemon
// response also carries frame/targets/universes/interface, but
// FetchStatus's own plain encoding/json.Unmarshal decode (svc_safety.go's
// own doc comment on why it is not strictjson.DecodeStrict) never
// requires them.
func daemonStatusJSON(t *testing.T, playback map[string]interface{}) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]interface{}{"playback": playback})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return encoded
}

// TestStatusPayloadReflectsActiveScene proves FetchStatus projects a
// daemon response reporting an active scene into a StatusSnapshot with
// Reachable=true and every PLAY-07 field populated from the decoded
// payload, never a zero/blank value standing in for real data.
func TestStatusPayloadReflectsActiveScene(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		if request.Route != "artnet status" {
			t.Fatalf("FetchStatus dialed route %q, want %q", request.Route, "artnet status")
		}
		return ipc.Result{Stdout: daemonStatusJSON(t, map[string]interface{}{
			"active":            true,
			"sceneId":           "11111111-1111-1111-1111-111111111111",
			"sceneName":         "Opening Look",
			"bpm":               120.0,
			"barIndex":          2,
			"beatFraction":      0.5,
			"enabledLayers":     []string{"base_look", "chase"},
			"controllingSource": "live",
			"outputState":       "frame-lock",
		})}
	}

	got := svc.FetchStatus()

	if !got.Reachable {
		t.Fatal("expected Reachable=true for a successful daemon response")
	}
	if !got.Active {
		t.Fatal("expected Active=true when the daemon reports an active scene")
	}
	if got.SceneName != "Opening Look" {
		t.Fatalf("SceneName = %q, want %q", got.SceneName, "Opening Look")
	}
	if got.BPM != 120 {
		t.Fatalf("BPM = %v, want 120", got.BPM)
	}
	if got.BarIndex != 2 {
		t.Fatalf("BarIndex = %d, want 2", got.BarIndex)
	}
	if len(got.EnabledLayers) != 2 || got.EnabledLayers[0] != "base_look" || got.EnabledLayers[1] != "chase" {
		t.Fatalf("EnabledLayers = %v, want [base_look chase]", got.EnabledLayers)
	}
	if got.ControllingSource != "live" || got.OutputState != "frame-lock" {
		t.Fatalf("ControllingSource/OutputState = %q/%q, want live/frame-lock", got.ControllingSource, got.OutputState)
	}
}

// TestStatusPayloadExplicitIdleWhenNoActiveScene proves the PLAY-07 idle
// edge (this plan's own must_haves.truths): when the daemon reports
// active=false (no current plan), FetchStatus's StatusSnapshot carries
// Active=false, a non-nil-but-empty EnabledLayers slice, and explicit
// (never empty-string) ControllingSource/OutputState values -- never a
// blank/undefined-looking payload.
func TestStatusPayloadExplicitIdleWhenNoActiveScene(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{Stdout: daemonStatusJSON(t, map[string]interface{}{
			"active":            false,
			"bpm":               0.0,
			"barIndex":          0,
			"beatFraction":      0.0,
			"enabledLayers":     []string{},
			"controllingSource": "live",
			"outputState":       "frame-lock",
		})}
	}

	got := svc.FetchStatus()

	if !got.Reachable {
		t.Fatal("expected Reachable=true -- the daemon answered, it simply has no active plan")
	}
	if got.Active {
		t.Fatal("expected Active=false when the daemon reports no active plan")
	}
	if got.EnabledLayers == nil {
		t.Fatal("expected a non-nil EnabledLayers slice for the idle projection")
	}
	if got.ControllingSource == "" || got.OutputState == "" {
		t.Fatalf("expected explicit non-empty ControllingSource/OutputState, got %q/%q", got.ControllingSource, got.OutputState)
	}
	if got.SceneID != "" || got.SceneName != "" {
		t.Fatalf("expected empty SceneID/SceneName for the idle projection, got %q/%q", got.SceneID, got.SceneName)
	}
}

// TestStatusPayloadOfflineWhenDaemonUnreachable proves FetchStatus never
// returns a zero-valued StatusSnapshot when the daemon cannot be reached
// at all (dial failure) -- it always returns the explicit offline
// projection (D-13/06-UI-SPEC.md: the safety cluster itself must stay
// interactive regardless, but the status bar's own copy must clearly say
// "can't reach the playback engine," never render blank fields).
func TestStatusPayloadOfflineWhenDaemonUnreachable(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{ExitCode: 1, Stderr: []byte("GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running?")}
	}

	got := svc.FetchStatus()

	if got.Reachable {
		t.Fatal("expected Reachable=false when the daemon cannot be reached")
	}
	if got.ControllingSource != "offline" || got.OutputState != "offline" {
		t.Fatalf("ControllingSource/OutputState = %q/%q, want offline/offline", got.ControllingSource, got.OutputState)
	}
	if got.EnabledLayers == nil {
		t.Fatal("expected a non-nil EnabledLayers slice for the offline projection")
	}
}

// TestStatusPayloadOfflineWhenDecodeFails proves the same explicit
// offline fallback applies to a malformed/undecodable daemon response,
// not merely a dial failure.
func TestStatusPayloadOfflineWhenDecodeFails(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{Stdout: []byte("not json")}
	}

	got := svc.FetchStatus()

	if got.Reachable {
		t.Fatal("expected Reachable=false for an undecodable daemon response")
	}
	if got.ControllingSource != "offline" || got.OutputState != "offline" {
		t.Fatalf("ControllingSource/OutputState = %q/%q, want offline/offline", got.ControllingSource, got.OutputState)
	}
}

// TestSafetyServiceStartStatusPushEmitsStatusUpdate proves
// StartStatusPush actually reaches runtime.EventsEmit end-to-end: it
// polls FetchStatus and stages the result through the underlying
// EventPusher (events.go), which flushes it as one "status:update" emit
// -- the exact key_link 06-05-PLAN.md declares between this file and
// LiveStatusBar.tsx.
func TestSafetyServiceStartStatusPushEmitsStatusUpdate(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{Stdout: daemonStatusJSON(t, map[string]interface{}{
			"active":            true,
			"sceneName":         "Push Test Scene",
			"controllingSource": "live",
			"outputState":       "frame-lock",
			"enabledLayers":     []string{},
		})}
	}

	emitted := make(chan StatusSnapshot, 4)
	svc.events.emit = func(ctx context.Context, eventName string, data ...interface{}) {
		if eventName != "status:update" {
			return
		}
		if snapshot, ok := data[0].(StatusSnapshot); ok {
			emitted <- snapshot
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartStatusPush(ctx)
	defer svc.StopStatusPush()

	select {
	case snapshot := <-emitted:
		if snapshot.SceneName != "Push Test Scene" {
			t.Fatalf("emitted SceneName = %q, want %q", snapshot.SceneName, "Push Test Scene")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for StartStatusPush to emit a status:update event")
	}
}
