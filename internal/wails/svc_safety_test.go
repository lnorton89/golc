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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

	require.Equal(t, "test-pipe", capturedPipe, "dialed pipe")
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
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)

	svc := NewSafetyService("test-pipe", root, showPath)
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		require.Fail(t, "dial must never be reached when authorization rejects the call")
		return ipc.Result{}
	}

	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)

	got := svc.Blackout(true)
	require.NotEqual(t, 0, got.ExitCode, "expected Blackout to be rejected when the active surface has no Blackout SafetyRef assigned")
	require.Contains(t, got.Stderr, "GOLC_OPERATORSURFACE_LOCKED")
}

// TestSafetyServiceBlackoutDispatchesWhenActiveSurfaceAssignsControl
// proves the counterpart: once Blackout is assigned to the active surface,
// the call authorizes and dispatches exactly as before CR-01.
func TestSafetyServiceBlackoutDispatchesWhenActiveSurfaceAssignsControl(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.golc")
	surfaceSvc := NewSurfaceService("", root, showPath)
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)
	result = surfaceSvc.AssignItem("Operator A", ControlRefInput{Kind: "safety", Safety: "blackout"})
	require.Equal(t, 0, result.ExitCode, "AssignItem failed: stderr=%s", result.Stderr)

	svc := NewSafetyService("test-pipe", root, showPath)
	var captured ipc.Request
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		captured = request
		return ipc.Result{}
	}

	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)

	got := svc.Blackout(true)
	require.Equal(t, 0, got.ExitCode, "expected Blackout to dispatch once assigned, got stderr=%s", got.Stderr)
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
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)

	svc := NewSafetyService("test-pipe", root, showPath)
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		return ipc.Result{}
	}

	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)
	result = svc.SetActiveSurface("")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface(\"\") failed: stderr=%s", result.Stderr)

	got := svc.Blackout(true)
	require.Equal(t, 0, got.ExitCode, "expected Blackout to dispatch after the active surface was cleared, got stderr=%s", got.Stderr)
}

func assertSafetyForward(t *testing.T, got ipc.Request, wantRoute string, wantArgs []string) {
	t.Helper()
	require.Equal(t, wantRoute, got.Route, "forwarded route")
	require.Equal(t, wantArgs, got.Args, "forwarded args")
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
	require.Equal(t, 1, got.ExitCode)
	require.Equal(t, "GOLC_ARTNET_SAFETY_REVOKED: rejected", got.Stderr, "expected the daemon's exact diagnostic")
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
	require.NoError(t, err, "json.Marshal")
	return encoded
}

// TestStatusPayloadReflectsActiveScene proves FetchStatus projects a
// daemon response reporting an active scene into a StatusSnapshot with
// Reachable=true and every PLAY-07 field populated from the decoded
// payload, never a zero/blank value standing in for real data.
func TestStatusPayloadReflectsActiveScene(t *testing.T) {
	svc := NewSafetyService("test-pipe", "", "")
	svc.dial = func(pipeName string, request ipc.Request) ipc.Result {
		require.Equal(t, "artnet status", request.Route, "FetchStatus dialed route")
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

	require.True(t, got.Reachable, "expected Reachable=true for a successful daemon response")
	require.True(t, got.Active, "expected Active=true when the daemon reports an active scene")
	require.Equal(t, "Opening Look", got.SceneName)
	require.EqualValues(t, 120, got.BPM)
	require.Equal(t, 2, got.BarIndex)
	require.Equal(t, []string{"base_look", "chase"}, got.EnabledLayers)
	require.Equal(t, "live", got.ControllingSource)
	require.Equal(t, "frame-lock", got.OutputState)
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

	require.True(t, got.Reachable, "expected Reachable=true -- the daemon answered, it simply has no active plan")
	require.False(t, got.Active, "expected Active=false when the daemon reports no active plan")
	require.NotNil(t, got.EnabledLayers, "expected a non-nil EnabledLayers slice for the idle projection")
	require.NotEmpty(t, got.ControllingSource, "expected explicit non-empty ControllingSource/OutputState")
	require.NotEmpty(t, got.OutputState, "expected explicit non-empty ControllingSource/OutputState")
	require.Empty(t, got.SceneID, "expected empty SceneID/SceneName for the idle projection")
	require.Empty(t, got.SceneName, "expected empty SceneID/SceneName for the idle projection")
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

	require.False(t, got.Reachable, "expected Reachable=false when the daemon cannot be reached")
	require.Equal(t, "offline", got.ControllingSource)
	require.Equal(t, "offline", got.OutputState)
	require.NotNil(t, got.EnabledLayers, "expected a non-nil EnabledLayers slice for the offline projection")
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

	require.False(t, got.Reachable, "expected Reachable=false for an undecodable daemon response")
	require.Equal(t, "offline", got.ControllingSource)
	require.Equal(t, "offline", got.OutputState)
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
		require.Equal(t, "Push Test Scene", snapshot.SceneName)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for StartStatusPush to emit a status:update event")
	}
}
