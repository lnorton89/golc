// svc_safety.go fills 06-04-PLAN.md's SafetyService stub (06-05-PLAN.md
// Task 1, PLAY-06/07/08/09): Blackout/StopReleaseAll/RevokeAutomation
// dial+forward the exact daemon "artnet safety ..." routes hotkey.go's
// OS-level callbacks already call directly (06-02-PLAN.md), always
// appending "--source manual" so an on-screen operator action is never
// blocked by an active Revoke Automation -- the on-screen button is the
// second, independent trigger into the same daemon override state
// RESEARCH.md Pitfall 1 requires (hotkey.go is the first). FetchStatus
// dials "artnet status" and projects the daemon's extended statusPayload
// (internal/artnet/daemon.go's playbackStatusPayload, 06-05-PLAN.md Task
// 1) into the JSON-safe StatusSnapshot the frontend's LiveStatusBar reads
// -- both this method and events.go's throttled pushStatus loop treat the
// daemon's response as the sole source of truth, never caching or
// re-deriving playback state locally.
package wails

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/artnet/ipc"
)

// statusPollInterval is how often StartStatusPush re-fetches the daemon's
// status via FetchStatus and stages it for the next throttled EventsEmit
// (events.go's own eventsTickInterval-cadence flush loop): reusing the
// same constant keeps this poll no faster than the flush that actually
// coalesces it, so a burst of polls between flushes never emits more than
// one "status:update" per eventsTickInterval (06-RESEARCH.md Open
// Question 3, "independent cadence, never share one ticker" -- this is
// the status feature's own independent cadence, decoupled from both the
// 40Hz Art-Net Worker tick and any MIDI message rate).
const statusPollInterval = eventsTickInterval

// SafetyService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. dial defaults to defaultDialForward
// (hotkey.go) -- the identical dial+forward implementation the OS-level
// hotkey callbacks use, so a test can inject a fake exactly the way
// hotkey_test.go's TestHotkeyKeydownForwardsDirectlyToDaemon does, and
// production code never diverges between the two independent trigger
// paths into the same daemon override state. events is this service's own
// EventPusher (events.go's throttle scaffold, reused rather than
// reimplemented) -- SafetyService owns it directly rather than reaching
// into internal/wails.App's own unexported events field, since app.go is
// a 06-04 stub this plan must not modify (06-05-PLAN.md interfaces note);
// cmd/golc-desktop/main.go starts/stops it alongside App's own lifecycle
// hooks.
type SafetyService struct {
	pipeName string
	dial     dialForwardFunc
	events   *EventPusher

	mu         sync.Mutex
	pollCancel context.CancelFunc
	pollDone   chan struct{}
}

// NewSafetyService constructs a SafetyService targeting pipeName, wired to
// the production ipc.Dial/ipc.Forward pair and its own idle EventPusher.
func NewSafetyService(pipeName string) *SafetyService {
	return &SafetyService{pipeName: pipeName, dial: defaultDialForward, events: NewEventPusher()}
}

// StartStatusPush begins this service's own throttled "status:update"
// push (PLAY-07, this file's own doc comment): it starts the underlying
// EventPusher's fixed-cadence flush loop, then polls FetchStatus on its
// own statusPollInterval ticker, staging each fresh StatusSnapshot via
// QueueStatus so a burst of polls between flushes coalesces into one
// emit, never one EventsEmit call per poll. Calling StartStatusPush again
// without an intervening StopStatusPush is a no-op (mirrors EventPusher's
// own Start idempotency).
func (s *SafetyService) StartStatusPush(ctx context.Context) {
	s.mu.Lock()
	if s.pollCancel != nil {
		s.mu.Unlock()
		return
	}
	pollCtx, cancel := context.WithCancel(ctx)
	s.pollCancel = cancel
	s.pollDone = make(chan struct{})
	s.mu.Unlock()

	s.events.Start(ctx)
	go s.pollStatus(pollCtx)
}

func (s *SafetyService) pollStatus(ctx context.Context) {
	defer close(s.pollDone)
	ticker := time.NewTicker(statusPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.events.QueueStatus(s.FetchStatus())
		}
	}
}

// StopStatusPush cancels the poll loop and stops the underlying
// EventPusher, mirroring App.OnShutdown's own reverse-order subsystem
// stop discipline. Safe to call more than once or before StartStatusPush.
func (s *SafetyService) StopStatusPush() {
	s.mu.Lock()
	cancel := s.pollCancel
	done := s.pollDone
	s.pollCancel = nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
	s.events.Stop()
}

// toggle forwards route with "--on <on>" and "--source manual" (the
// operator-issued default -- never blocked by an active Revoke
// Automation, even one this same call might itself be toggling off,
// mirroring internal/command/artnet.go's runArtnetSafetyToggle
// convention), converting ipc.Result's []byte Stdout/Stderr into Result's
// plain-string fields (app.go's own doc comment on why: a simpler
// generated TypeScript type, not a base64-encoded byte array).
func (s *SafetyService) toggle(route string, on bool) Result {
	result := s.dialFn()(s.pipeName, ipc.Request{
		Route: route,
		Args:  []string{"--on", strconv.FormatBool(on), "--source", "manual"},
	})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// dialFn returns s.dial, defaulting to defaultDialForward for a
// SafetyService constructed via a bare struct literal (e.g. Wails' own
// binding-reflection scan) rather than NewSafetyService.
func (s *SafetyService) dialFn() dialForwardFunc {
	if s.dial != nil {
		return s.dial
	}
	return defaultDialForward
}

// Blackout dials+forwards "artnet safety blackout --on <on> --source
// manual" -- the same daemon route hotkey.go's OS-level Blackout callback
// forwards directly (RESEARCH.md Pitfall 1: two independent triggers into
// one override state).
func (s *SafetyService) Blackout(on bool) Result {
	return s.toggle(string(routeBlackout), on)
}

// StopReleaseAll dials+forwards "artnet safety stop-all --on <on>
// --source manual".
func (s *SafetyService) StopReleaseAll(on bool) Result {
	return s.toggle(string(routeStopAll), on)
}

// RevokeAutomation dials+forwards "artnet safety revoke-automation --on
// <on> --source manual".
func (s *SafetyService) RevokeAutomation(on bool) Result {
	return s.toggle(string(routeRevokeAutomation), on)
}

// StatusSnapshot is the JSON-safe PLAY-07 status projection FetchStatus
// returns and events.go's throttled pushStatus loop pushes under the
// "status:update" event name. Reachable distinguishes "daemon confirmed
// unreachable" from any other idle state (06-UI-SPEC.md's daemon-
// unreachable copy reads this directly rather than inferring offline-ness
// from a zeroed/blank status) -- SceneName/BPM/BarIndex/BeatFraction/
// EnabledLayers/ControllingSource/OutputState otherwise mirror
// internal/artnet/daemon.go's playbackStatusPayload field-for-field
// (06-05-PLAN.md Task 1). This is a throttled hint the frontend's Zustand
// store caches, never the playback/status source of truth
// (06-RESEARCH.md anti-pattern) -- FetchStatus is what the frontend calls
// to re-query authoritative state on a detected gap.
type StatusSnapshot struct {
	Reachable         bool     `json:"reachable"`
	Active            bool     `json:"active"`
	SceneID           string   `json:"sceneId,omitempty"`
	SceneName         string   `json:"sceneName,omitempty"`
	BPM               float64  `json:"bpm"`
	BarIndex          int      `json:"barIndex"`
	BeatFraction      float64  `json:"beatFraction"`
	EnabledLayers     []string `json:"enabledLayers"`
	ControllingSource string   `json:"controllingSource"`
	OutputState       string   `json:"outputState"`
}

// offlineStatusSnapshot is the explicit, never-blank idle projection
// FetchStatus returns whenever the daemon cannot be reached or its
// response cannot be decoded (06-UI-SPEC.md error state, D-13: the safety
// cluster itself stays interactive regardless -- only the status bar's
// projection degrades). EnabledLayers is a non-nil empty slice, matching
// the daemon's own playbackStatusPayload "never null" contract.
func offlineStatusSnapshot() StatusSnapshot {
	return StatusSnapshot{
		Reachable:         false,
		Active:            false,
		EnabledLayers:     []string{},
		ControllingSource: "offline",
		OutputState:       "offline",
	}
}

// daemonPlaybackEnvelope decodes just the "playback" member of the
// daemon's "artnet status" JSON response (internal/artnet/daemon.go's
// statusPayload/playbackStatusPayload) -- mirrored here field-for-field/
// tag-for-tag (internal/command/artnet.go's artnetPlaybackStatus follows
// the identical mirroring convention for the CLI) rather than imported,
// since internal/wails is a thin IPC client of the daemon process, never
// an in-process importer of internal/artnet (RESEARCH.md's "Wails app
// attaches as just one more IPC client" boundary). Deliberately decoded
// with plain encoding/json (not internal/strictjson.DecodeStrict): this
// envelope intentionally declares only the "playback" member it needs,
// and strictjson's DisallowUnknownFields would reject the daemon's
// sibling frame/targets/universes/interface members this Go-host-internal
// read has no use for -- unlike the CLI's artnetStatusPayload (which
// mirrors the full wire shape to render every field), FetchStatus only
// ever needs the playback projection.
type daemonPlaybackEnvelope struct {
	Playback struct {
		Active            bool     `json:"active"`
		SceneID           string   `json:"sceneId"`
		SceneName         string   `json:"sceneName"`
		BPM               float64  `json:"bpm"`
		BarIndex          int      `json:"barIndex"`
		BeatFraction      float64  `json:"beatFraction"`
		EnabledLayers     []string `json:"enabledLayers"`
		ControllingSource string   `json:"controllingSource"`
		OutputState       string   `json:"outputState"`
	} `json:"playback"`
}

// FetchStatus dials the daemon, forwards "artnet status", and projects the
// decoded playback fields into a StatusSnapshot. Any failure along the way
// (dial, non-zero daemon result, or decode) returns offlineStatusSnapshot
// rather than a zero-valued/partial StatusSnapshot -- the frontend's
// daemon-unreachable state must always be an explicit signal (D-13/
// 06-UI-SPEC.md), never blank fields a caller has to infer meaning from.
func (s *SafetyService) FetchStatus() StatusSnapshot {
	result := s.dialFn()(s.pipeName, ipc.Request{Route: "artnet status"})
	if result.ExitCode != 0 {
		return offlineStatusSnapshot()
	}

	var envelope daemonPlaybackEnvelope
	if err := json.Unmarshal(result.Stdout, &envelope); err != nil {
		return offlineStatusSnapshot()
	}

	pb := envelope.Playback
	layers := pb.EnabledLayers
	if layers == nil {
		layers = []string{}
	}
	return StatusSnapshot{
		Reachable:         true,
		Active:            pb.Active,
		SceneID:           pb.SceneID,
		SceneName:         pb.SceneName,
		BPM:               pb.BPM,
		BarIndex:          pb.BarIndex,
		BeatFraction:      pb.BeatFraction,
		EnabledLayers:     layers,
		ControllingSource: pb.ControllingSource,
		OutputState:       pb.OutputState,
	}
}
