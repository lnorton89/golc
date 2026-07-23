// hotkey.go registers 06-04-PLAN.md Task 1's three OS-level safety-cluster
// global hotkeys (D-16: fixed, unmodifiable defaults, not user-rebindable
// in this phase) via golang.design/x/hotkey -- the only viable path to a
// true OS-level global shortcut in Wails v2, which ships no native
// global-shortcut API (06-RESEARCH.md Pattern 2, Assumption A3). Each
// callback dials+forwards the matching "artnet safety ..." daemon route
// DIRECTLY from this Go host process via ipc.Dial/ipc.Forward, never
// through a JS-mediated keydown path (06-RESEARCH.md Pitfall 1) -- so the
// shortcut keeps firing even if the webview's JS thread is completely
// stalled.
//
// RegisterAll attempts all three bindings independently: a conflict on one
// shortcut (e.g. another running application already owns that key
// combination) must never silently prevent the other two from registering
// or swallow the failure (06-RESEARCH.md Security Domain DoS mitigation,
// T-06-10). Do NOT extend this file to register any non-safety key as an
// OS-level global hotkey (RESEARCH.md Pitfall 4) -- the general PLAY-02
// keyboard workflow is ordinary in-webview keydown handling, added by
// 06-06-PLAN.md's frontend code, never here.
package wails

import (
	"encoding/json"
	"strconv"
	"sync"

	"golang.design/x/hotkey"

	"github.com/lnorton89/golc/internal/artnet/ipc"
)

// safetyRoute names one of the three daemon safety routes 06-02-PLAN.md
// already built (internal/command/artnet.go's runArtnetSafetyToggle
// client routes, forwarded here without going through that CLI package --
// this file dials the daemon directly, exactly as internal/artnet/
// ipc/client.go's own doc comment describes for every safety-cluster
// trigger source).
type safetyRoute string

const (
	routeBlackout         safetyRoute = "artnet safety blackout"
	routeStopAll          safetyRoute = "artnet safety stop-all"
	routeRevokeAutomation safetyRoute = "artnet safety revoke-automation"
)

// Fixed, unmodifiable default hotkey bindings for the three safety-cluster
// controls (D-16). Ctrl+Alt+Shift is used as the modifier combination for
// all three so the bindings read as one cohesive "emergency" cluster and
// stay unlikely to collide with a single, more common modifier combination
// another application already owns.
var (
	blackoutMods         = []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModAlt, hotkey.ModShift}
	blackoutKey          = hotkey.KeyB
	stopAllMods          = []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModAlt, hotkey.ModShift}
	stopAllKey           = hotkey.KeyS
	revokeAutomationMods = []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModAlt, hotkey.ModShift}
	revokeAutomationKey  = hotkey.KeyR
)

// hotkeyBinding pairs one safety control's fixed shortcut with the daemon
// route its callback forwards to.
type hotkeyBinding struct {
	control string
	mods    []hotkey.Modifier
	key     hotkey.Key
	route   safetyRoute
}

// safetyBindings returns the exact three D-16 bindings, in a stable order.
func safetyBindings() []hotkeyBinding {
	return []hotkeyBinding{
		{control: "blackout", mods: blackoutMods, key: blackoutKey, route: routeBlackout},
		{control: "stop-all", mods: stopAllMods, key: stopAllKey, route: routeStopAll},
		{control: "revoke-automation", mods: revokeAutomationMods, key: revokeAutomationKey, route: routeRevokeAutomation},
	}
}

// HotkeyFailure is one safety-cluster hotkey's registration failure --
// surfaced to the frontend, never silent (Security Domain DoS mitigation).
type HotkeyFailure struct {
	Control string
	Error   string
}

// registerer abstracts *hotkey.Hotkey's Register/Keydown/Unregister
// surface so tests can inject a fake without ever touching a real
// OS-level global hotkey (which would be unsafe to register/conflict-test
// from an automated test run). *hotkey.Hotkey satisfies this interface
// exactly as-is.
type registerer interface {
	Register() error
	Keydown() <-chan hotkey.Event
	Unregister() error
}

// hotkeyFactory constructs one registerer for a binding's mods/key.
// Production uses defaultHotkeyFactory (golang.design/x/hotkey.New);
// tests inject a fake factory.
type hotkeyFactory func(mods []hotkey.Modifier, key hotkey.Key) registerer

func defaultHotkeyFactory(mods []hotkey.Modifier, key hotkey.Key) registerer {
	return hotkey.New(mods, key)
}

// dialForwardFunc is the exact ipc.Dial + ipc.Forward pair a hotkey
// callback invokes directly from this Go host -- never through JS
// (Pitfall 1). Tests inject a fake to assert the callback wiring without a
// real named pipe.
type dialForwardFunc func(pipeName string, request ipc.Request) ipc.Result

func defaultDialForward(pipeName string, request ipc.Request) ipc.Result {
	conn, err := ipc.Dial(pipeName)
	if err != nil {
		return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error())}
	}
	defer conn.Close()
	return ipc.Forward(conn, request)
}

// HotkeyManager owns the three OS-level safety-cluster hotkeys (D-16).
type HotkeyManager struct {
	pipeName string
	factory  hotkeyFactory
	dial     dialForwardFunc

	mu       sync.Mutex
	active   []registerer
	stopChs  []chan struct{}
	failures []HotkeyFailure
}

// NewHotkeyManager constructs a HotkeyManager targeting pipeName (the
// daemon's IPC pipe every callback dials+forwards to directly), wired to
// the production hotkey factory and dial/forward implementation.
func NewHotkeyManager(pipeName string) *HotkeyManager {
	return &HotkeyManager{
		pipeName: pipeName,
		factory:  defaultHotkeyFactory,
		dial:     defaultDialForward,
	}
}

// RegisterAll registers all three D-16 safety-cluster shortcuts, returning
// every failure (there may be zero, one, two, or three) -- never stopping
// at the first failure, since a conflict on one shortcut must not silently
// prevent the other two from registering (Security Domain DoS mitigation).
func (m *HotkeyManager) RegisterAll() []HotkeyFailure {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failures = nil
	for _, binding := range safetyBindings() {
		hk := m.factory(binding.mods, binding.key)
		if err := hk.Register(); err != nil {
			m.failures = append(m.failures, HotkeyFailure{Control: binding.control, Error: err.Error()})
			continue
		}
		stop := make(chan struct{})
		m.active = append(m.active, hk)
		m.stopChs = append(m.stopChs, stop)
		go m.listen(hk, binding, stop)
	}
	return append([]HotkeyFailure(nil), m.failures...)
}

// listen forwards binding.route directly on every Keydown event -- the
// exact daemon call the on-screen safety button also makes (Pitfall 1: no
// JS-mediated path for this callback). Keydown's channel is captured once,
// before the loop starts: *hotkey.Hotkey.Unregister() replaces its own
// internal event channel as part of unregistering, so calling hk.Keydown()
// again inside the loop after UnregisterAll has started would race against
// that field reset -- capturing it once here reads it exactly once, before
// any concurrent Unregister could run. nextToggleValue (CR-03 fix) queries
// the daemon's current state first so a second press of the same hotkey
// releases the control instead of re-activating it -- the OS-level
// counterpart to SafetyCluster.tsx's own hold-button toggle fix.
func (m *HotkeyManager) listen(hk registerer, binding hotkeyBinding, stop <-chan struct{}) {
	keydown := hk.Keydown()
	for {
		select {
		case <-keydown:
			on := m.nextToggleValue(binding.route)
			m.dial(m.pipeName, ipc.Request{
				Route: string(binding.route),
				Args:  []string{"--on", strconv.FormatBool(on), "--source", "manual"},
			})
		case <-stop:
			return
		}
	}
}

// nextToggleValue queries "artnet status" and returns the opposite of
// route's currently observed active state (CR-03 fix): Blackout and
// Stop-All share one daemon-side combined "blackout" outputState signal
// (safety.go's own doc comment: tracked as independent flags, but
// applyOverrides/newPlaybackStatusPayload treat them identically for
// output purposes, and there is no separate per-flag field on the wire --
// mirrors SafetyCluster.tsx's own blackoutOrStopActive derivation and its
// documented ambiguity), while Revoke Automation toggles off its own
// unambiguous "revoked" controllingSource. A status query failure (daemon
// unreachable, non-zero exit, or an undecodable response) defaults to true
// (activate) -- the pre-CR-03 always-activate behavior -- rather than
// guessing a release the daemon cannot currently confirm.
func (m *HotkeyManager) nextToggleValue(route safetyRoute) bool {
	result := m.dial(m.pipeName, ipc.Request{Route: "artnet status"})
	if result.ExitCode != 0 {
		return true
	}
	var envelope daemonPlaybackEnvelope
	if err := json.Unmarshal(result.Stdout, &envelope); err != nil {
		return true
	}
	if route == routeRevokeAutomation {
		return envelope.Playback.ControllingSource != "revoked"
	}
	return envelope.Playback.OutputState != "blackout"
}

// Failures returns the most recent RegisterAll outcome.
func (m *HotkeyManager) Failures() []HotkeyFailure {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]HotkeyFailure(nil), m.failures...)
}

// UnregisterAll stops every listener goroutine and unregisters every
// successfully-registered hotkey (OnShutdown's reverse-order stop step).
func (m *HotkeyManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, hk := range m.active {
		close(m.stopChs[i])
		_ = hk.Unregister()
	}
	m.active = nil
	m.stopChs = nil
}
