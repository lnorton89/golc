// svc_midi.go fills MidiService (06-04-PLAN.md Task 1 stub) with the
// generic MIDI learn and soft-takeover bindings (06-08-PLAN.md Task 2,
// PLAY-04/05 D-05..D-12): StartLearn opens a bounded per-control capture
// window over an attached internal/midi.Driver's live event channel via
// midi.CaptureCandidate (06-03), checks the candidate against the target
// surface's existing MidiMappings via midi.ProposeMapping (D-06 reject
// outright, D-07 per-surface scope) and again via
// operatorsurface.AddMidiMapping's own belt-and-suspenders check, then
// persists Load -> mutate -> Save exactly like svc_surface.go's read
// methods (06-07) touch the same show.State directly -- there is no
// dedicated "operatorsurface midi add/remove" CLI route (unlike
// svc_surface.go's assign/unassign, which forward to self-registered
// internal/command routes), matching this plan's own <key_links> pointing
// straight at operatorsurface.AddMidiMapping rather than at a command
// package route. CancelLearn/RemoveMapping are the session-cancel and
// mapping-delete counterparts. Learnable controls are exactly the
// controls assigned to the surface (D-08): StartLearn calls
// command.Authorize (internal/command/operatorsurface.go, already the
// project's one D-04 enforcement point) before opening a capture window,
// never maintaining a second "MIDI-mappable" list.
//
// dispatchLoop consumes an attached Driver's live event channel and
// routes each decoded event either into an in-progress learn session's
// capture channel, or into cross-to-catch soft-takeover arbitration
// (midi.TakeoverState.Update, 06-03) against whichever surface
// SetActiveSurface last selected -- the crossing/arming decision itself
// runs on the UNTHROTTLED physical value (RESEARCH.md Open Question 3);
// only the resulting MidiFeedback push to the frontend is throttled, via
// events.go's existing EventPusher scaffold (emitMidiFeedback/
// QueueMidiFeedback). Note/button mappings bypass TakeoverState entirely
// and report Armed=true immediately (D-12).
package wails

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/midi"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// learnCaptureTimeout bounds StartLearn's capture window (D-05: no
// unbounded wait path) -- long enough for an operator to reach for and
// move/press the physical control after clicking Learn, short enough that
// an abandoned session does not linger indefinitely.
const learnCaptureTimeout = 15 * time.Second

// defaultTakeoverAppValue seeds a freshly-encountered mapping's ghost/
// target marker (D-10) when no prior AppValue is known. Querying the
// mapped control's actual live playback value (e.g. a group master's
// current level) requires a command-dispatch integration point this plan
// does not build (see 06-08-SUMMARY.md Next Phase Readiness) -- documented
// here as a known placeholder, not a silent approximation.
const defaultTakeoverAppValue = 0.5

// learnSession is StartLearn's in-progress capture state: next is the
// channel dispatchLoop forwards matching driver events onto (feeding
// midi.CaptureCandidate directly, 06-03); cancel lets CancelLearn abort
// the capture window before learnCaptureTimeout elapses.
type learnSession struct {
	next   chan midi.ControlKey
	cancel chan struct{}
}

// MidiFeedback is the throttled D-09/D-10/D-11 push emitMidiFeedback sends
// under the "midi:feedback" event name (events.go's QueueMidiFeedback):
// Physical is the live physical fader/button position (D-09, drives the
// on-screen slider even while not armed), AppValue is the fixed ghost/
// target marker while unarmed or the tracked controlling value once armed
// (D-10), and Armed reports whether the crossing has occurred (D-11) --
// always true for a Note/button mapping (D-12: no arming delay).
type MidiFeedback struct {
	SurfaceName string  `json:"surfaceName"`
	MappingID   string  `json:"mappingId"`
	Kind        string  `json:"kind"`
	Armed       bool    `json:"armed"`
	AppValue    float64 `json:"appValue"`
	Physical    float64 `json:"physical"`
}

// MidiService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. root/showPath mirror SurfaceService's own
// fields (06-07) -- the exact ShowState location every persistence method
// Loads/Saves against. events is this service's own EventPusher (reused
// scaffold, not App's own field, mirroring SafetyService's identical
// 06-05 rationale).
type MidiService struct {
	pipeName string
	root     string
	showPath string

	events *EventPusher
	dial   dialForwardFunc

	mu             sync.Mutex
	driver         *midi.Driver
	learning       *learnSession
	activeSurface  string
	activeMappings []operatorsurface.MidiMapping
	takeovers      map[string]*midi.TakeoverState
}

// NewMidiService constructs a MidiService targeting pipeName (reserved for
// a future daemon-side dispatch call, unused by this plan's ShowState-only
// persistence -- mirrors SurfaceService's identical pipeName field) and
// the ShowState at showPath, resolved against root.
func NewMidiService(pipeName, root, showPath string) *MidiService {
	return &MidiService{
		pipeName:  pipeName,
		root:      root,
		showPath:  showPath,
		events:    NewEventPusher(),
		takeovers: map[string]*midi.TakeoverState{},
	}
}

// StartFeedback begins this service's own throttled "midi:feedback" push
// loop (events.go's EventPusher scaffold). Mirrors SafetyService's
// StartStatusPush lifecycle -- cmd/golc-desktop/main.go composes it
// alongside App's own OnStartup, without modifying app.go.
func (s *MidiService) StartFeedback(ctx context.Context) {
	s.events.Start(ctx)
}

// StopFeedback stops the throttled push loop. Safe to call more than once
// or before StartFeedback.
func (s *MidiService) StopFeedback() {
	s.events.Stop()
}

// AttachDriver wires d as this service's live MIDI event source: every
// decoded Note/CC message d.Listen() delivers is routed by dispatchLoop
// into either an in-progress learn capture or active-surface takeover
// arbitration. AttachDriver is safe to call at most once per Driver (mirrors
// Driver.Listen's own "at most once" contract) -- production
// (cmd/golc-desktop/main.go) calls it once after a successful
// midi.OpenFirstAvailable; a nil/failed open is never fatal (MIDI hardware
// remains optional, PROJECT.md), so main.go simply skips calling
// AttachDriver in that case rather than this method needing to tolerate a
// nil Driver itself.
func (s *MidiService) AttachDriver(d *midi.Driver) error {
	events, err := d.Listen()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.driver = d
	s.mu.Unlock()
	go s.dispatchLoop(events)
	return nil
}

// DetachDriver closes the attached Driver (if any), stopping dispatchLoop
// (Listen's channel closes, ending the loop's range). Safe to call more
// than once or before AttachDriver.
func (s *MidiService) DetachDriver() {
	s.mu.Lock()
	d := s.driver
	s.driver = nil
	s.mu.Unlock()
	if d != nil {
		_ = d.Close()
	}
}

// dispatchLoop consumes events until the channel closes (Driver.Close),
// routing each one via route.
func (s *MidiService) dispatchLoop(events <-chan midi.Event) {
	for evt := range events {
		s.route(evt)
	}
}

// route forwards evt to an in-progress learn session's capture channel
// (non-blocking -- a session only ever consumes its first match, per
// midi.CaptureCandidate's own single-receive contract) if one is active,
// or otherwise arbitrates it against the active surface's mappings.
// Routing to a learn session takes priority: while capturing, incoming
// messages are never simultaneously treated as live playback input.
func (s *MidiService) route(evt midi.Event) {
	s.mu.Lock()
	session := s.learning
	s.mu.Unlock()

	if session != nil {
		select {
		case session.next <- evt.Key:
		default:
		}
		return
	}

	s.dispatchToActiveSurface(evt)
}

// dispatchToActiveSurface matches evt against the active surface's cached
// mapping set (refreshed by SetActiveSurface/StartLearn/RemoveMapping),
// arbitrates it, and now dispatches the ControlRef-implied command in
// addition to feedback (06-09-PLAN.md, Gap B[1] closure -- previously this
// method only computed arming state and pushed feedback, never actually
// operating the show): Note-kind mappings report Armed=true immediately
// (D-12, no pickup/arming state) and their activation edge is Value>0 (a
// Note-off never (re-)dispatches); ControlChange-kind mappings run
// midi.TakeoverState.Update with the unthrottled physical value (D-11),
// report the resulting armed state + controlling/ghost value (D-09/D-10),
// and their activation edge is the not-armed-to-armed transition on this
// message (never re-firing on a later armed message). A message matching no
// mapping on the active surface is silently dropped -- there is nothing to
// arbitrate, feed back, or dispatch for it.
func (s *MidiService) dispatchToActiveSurface(evt midi.Event) {
	s.mu.Lock()
	surfaceName := s.activeSurface
	mappings := s.activeMappings
	s.mu.Unlock()

	if surfaceName == "" {
		return
	}
	mapping, found := findMapping(mappings, evt.Key)
	if !found {
		return
	}

	var armed, edge bool
	var controlValue float64

	if mapping.Kind == operatorsurface.Note {
		armed = true
		controlValue = evt.Value
		edge = evt.Value > 0
	} else {
		state := s.takeoverStateFor(mapping.ID)
		wasArmed := state.Armed
		armed, controlValue = state.Update(evt.Value)
		edge = !wasArmed && armed
	}

	s.dispatchMapping(mapping, armed, edge, controlValue, evt.Value)
	s.emitMidiFeedback(surfaceName, mapping, armed, controlValue, evt.Value)
}

// dispatchMapping executes the command implied by mapping's Target once
// armed/edge indicate the control should act: a discrete scene/layer/safety
// action fires only on the activation edge (a Note press, or a CC's first
// arming crossing) -- never on every held/armed message (Task 3's
// once-per-press/once-per-crossing contract) -- while a master level
// forwards on every armed message (continuous for a CC, D-11's "one
// deliberately-repeating dispatch"; a Note-kind master toggles level
// 1.0/0.0 on both press and release since armed is unconditionally true for
// Note, D-12). rawValue is evt.Value verbatim, needed for the Note-kind
// master's press/release level, distinct from controlValue (the
// takeover-tracked value a CC master forwards instead).
func (s *MidiService) dispatchMapping(mapping operatorsurface.MidiMapping, armed, edge bool, controlValue, rawValue float64) {
	switch mapping.Target.Kind {
	case operatorsurface.ControlScene:
		if !edge {
			return
		}
		s.dispatchSceneSwitch(mapping.Target.Scene)
	case operatorsurface.ControlLayer:
		if !edge {
			return
		}
		s.dispatchLayerToggle(mapping.Target.Layer)
	case operatorsurface.ControlSafety:
		if !edge {
			return
		}
		s.dispatchSafetyTrigger(mapping.Target.Safety)
	case operatorsurface.ControlMaster:
		if !armed {
			return
		}
		level := controlValue
		if mapping.Kind == operatorsurface.Note {
			if rawValue > 0 {
				level = 1
			} else {
				level = 0
			}
		}
		s.dispatchMasterSet(mapping.Target.Master, level)
	}
}

// dispatchSceneSwitch runs "playback switch <scene> --show <path>" through
// a freshly built default command registry (mirrors
// PlaybackService.SwitchScene/execute's exact in-process dispatch path, so
// a MIDI-driven switch and a CLI/PlaybackService-driven switch behave
// identically -- no second, divergent playback-authority path). A target
// scene since deleted from the show resolves to "" (sceneNameByID's
// existing read-only projection tolerance) and dispatches nothing rather
// than risk executing against an empty/garbled scene name.
func (s *MidiService) dispatchSceneSwitch(sceneID uuid.UUID) {
	state, err := showLoadWithRetry(s.root, s.showPath)
	if err != nil {
		return
	}
	name := sceneNameByID(state.Scenes, sceneID)
	if name == "" {
		return
	}
	s.executeWithRetry("playback", "switch", name, "--show", s.showPath)
}

// dispatchLayerToggle flips ref's Enabled flag via "scene layer set <scene>
// --kind <kind> [--ref <preserved>] [--disable if currently enabled] --show
// <path>", re-supplying the layer's existing Ref so the toggle never
// discards it -- mirrors PlaybackService.SetLayerEnabled's WR-01/WR-03
// discipline exactly (the same pre-read-then-preserve pattern, not a second
// divergent copy). A target scene/layer since deleted from the show
// resolves to no match and dispatches nothing.
func (s *MidiService) dispatchLayerToggle(ref operatorsurface.LayerRef) {
	state, err := showLoadWithRetry(s.root, s.showPath)
	if err != nil {
		return
	}
	sceneName := sceneNameByID(state.Scenes, ref.SceneID)
	if sceneName == "" {
		return
	}

	var (
		found      bool
		currentRef uuid.UUID
		enabled    bool
	)
	for _, sc := range state.Scenes {
		if sc.ID != ref.SceneID {
			continue
		}
		layer, ok := sc.LayerByKind(ref.Kind)
		if !ok {
			return
		}
		found = true
		currentRef = layer.Ref
		enabled = layer.Enabled
		break
	}
	if !found {
		return
	}

	args := []string{"scene", "layer", "set", sceneName, "--kind", string(ref.Kind)}
	if currentRef != uuid.Nil {
		args = append(args, "--ref", currentRef.String())
	}
	if enabled {
		args = append(args, "--disable")
	}
	args = append(args, "--show", s.showPath)
	s.executeWithRetry(args...)
}

// dispatchSafetyTrigger dials+forwards the daemon safety route matching
// control with "--on true --source manual" -- the identical daemon path
// SafetyService.toggle already uses (hotkey.go's routeBlackout/routeStopAll/
// routeRevokeAutomation constants), so a MIDI-triggered safety action and an
// on-screen/hotkey-triggered one are indistinguishable to the daemon, never
// a slow show.Save-backed path (threat T-06-26).
func (s *MidiService) dispatchSafetyTrigger(control operatorsurface.SafetyControl) {
	route := safetyRouteFor(control)
	if route == "" {
		return
	}
	result := s.dialFn()(s.pipeName, ipc.Request{
		Route: string(route),
		Args:  []string{"--on", "true", "--source", "manual"},
	})
	if result.ExitCode != 0 {
		log.Printf("GOLC_WAILS_MIDI_SAFETY_DISPATCH_FAILED: route=%s: %s", route, result.Stderr)
	}
}

// safetyRouteFor maps an operatorsurface.SafetyControl to its daemon route
// constant, or "" for an unrecognized value (never reached in practice --
// operatorsurface.SafetyControl is a closed enum per model.go's own doc
// comment).
func safetyRouteFor(control operatorsurface.SafetyControl) safetyRoute {
	switch control {
	case operatorsurface.Blackout:
		return routeBlackout
	case operatorsurface.StopReleaseAll:
		return routeStopAll
	case operatorsurface.RevokeAutomation:
		return routeRevokeAutomation
	default:
		return ""
	}
}

// dispatchMasterSet dials+forwards "artnet master set --grand <level>
// --source manual" or "--group <id> --level <level> --source manual" -- the
// exact daemon route/arg grammar runArtnetMasterSet
// (internal/command/artnet.go) and the daemon's own handleMasterSet
// (internal/artnet/daemon.go) both expect. This dials the daemon directly
// (mirrors SafetyService.toggle), bypassing the CLI route's own
// text-argument validation layer, since level/GroupID here are already
// typed/validated values sourced from the show document and the takeover
// state machine, never raw user text -- master/safety dispatch always takes
// this daemon path, never show.Save (threat T-06-26).
func (s *MidiService) dispatchMasterSet(ref operatorsurface.MasterRef, level float64) {
	rawLevel := strconv.FormatFloat(level, 'f', -1, 64)
	var args []string
	if ref.Kind == operatorsurface.GrandMaster {
		args = []string{"--grand", rawLevel, "--source", "manual"}
	} else {
		args = []string{"--group", ref.GroupID.String(), "--level", rawLevel, "--source", "manual"}
	}
	result := s.dialFn()(s.pipeName, ipc.Request{Route: "artnet master set", Args: args})
	if result.ExitCode != 0 {
		log.Printf("GOLC_WAILS_MIDI_MASTER_DISPATCH_FAILED: ref=%s: %s", ref.Kind, result.Stderr)
	}
}

// execute runs a full route-plus-args word sequence through a freshly built
// default command registry -- mirrors PlaybackService.execute exactly, so
// scene/layer dispatch here and a CLI/PlaybackService invocation of the
// identical route behave identically (there is only one in-process
// playback-authority implementation, never a second one duplicated here).
func (s *MidiService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_REGISTRY_BUILD_FAILED: " + err.Error()}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// dispatchLockRetries/dispatchLockRetryDelay bound the retry this file
// applies around the show store's transient "database is locked"
// contention (internal/show/schema.go sets no busy_timeout and performs no
// retry of its own -- it documents a single-writer-per-process model, but
// MidiService's own dispatch loop is a persistent background goroutine that
// can race a concurrent, independently-triggered show.Load from another
// service, e.g. PlaybackService.GetState()/SurfaceService.ListMappings()
// polled by the frontend at the same moment as a physical MIDI press).
// [Rule 2]: without this retry, a transient lock would silently drop the
// operator's button press (dispatchSceneSwitch/dispatchLayerToggle already
// discard a show.Load error as "nothing to dispatch," and the underlying
// GOLC_SHOW_STATE_INVALID mutation failure carries no automatic retry of
// its own) rather than switch the scene/toggle the layer as pressed. Five
// attempts at a 5ms backoff bound the added worst-case latency to ~25ms,
// well inside a physical control's perceived response budget.
const (
	dispatchLockRetries   = 5
	dispatchLockRetryWait = 5 * time.Millisecond
)

// isTransientShowLockError reports whether err is the show store's own
// "database is locked" (SQLite SQLITE_BUSY) diagnostic -- the one show.Load/
// mutation failure mode this file retries; every other error (a genuinely
// corrupt/missing store, GOLC_SHOW_NOT_GOLC_FORMAT, etc.) is not transient
// and is returned to the caller immediately, unretried.
func isTransientShowLockError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

// showLoadWithRetry retries show.Load up to dispatchLockRetries times when
// the failure is isTransientShowLockError, so dispatchSceneSwitch/
// dispatchLayerToggle's pre-read does not silently treat a transient lock
// as "nothing to dispatch."
func showLoadWithRetry(root, showPath string) (show.State, error) {
	var lastErr error
	for attempt := 0; attempt < dispatchLockRetries; attempt++ {
		state, err := show.Load(root, showPath)
		if err == nil {
			return state, nil
		}
		lastErr = err
		if !isTransientShowLockError(err) {
			return show.State{}, err
		}
		time.Sleep(dispatchLockRetryWait)
	}
	return show.State{}, lastErr
}

// executeWithRetry runs execute, retrying up to dispatchLockRetries times
// when the registry route's own Result surfaces the show store's transient
// "database is locked" diagnostic in Stderr -- the mutating counterpart to
// showLoadWithRetry, so a scene switch/layer toggle is not silently dropped
// by the same transient contention on its own internal Load-mutate-Save
// call.
func (s *MidiService) executeWithRetry(args ...string) Result {
	var result Result
	for attempt := 0; attempt < dispatchLockRetries; attempt++ {
		result = s.execute(args...)
		if result.ExitCode == 0 || !isTransientShowLockError(fmt.Errorf("%s", result.Stderr)) {
			return result
		}
		time.Sleep(dispatchLockRetryWait)
	}
	return result
}

// dialFn returns s.dial, defaulting to defaultDialForward for a MidiService
// constructed via a bare struct literal (e.g. Wails' own binding-reflection
// scan) rather than NewMidiService -- mirrors SafetyService.dialFn exactly.
func (s *MidiService) dialFn() dialForwardFunc {
	if s.dial != nil {
		return s.dial
	}
	return defaultDialForward
}

// takeoverStateFor returns the TakeoverState for mappingID, constructing
// and caching a freshly-seeded one (defaultTakeoverAppValue) on first use.
func (s *MidiService) takeoverStateFor(mappingID uuid.UUID) *midi.TakeoverState {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mappingID.String()
	state, ok := s.takeovers[key]
	if !ok {
		seeded := midi.NewTakeoverState(defaultTakeoverAppValue)
		state = &seeded
		s.takeovers[key] = state
	}
	return state
}

// emitMidiFeedback stages a MidiFeedback snapshot for the next throttled
// "midi:feedback" push (events.go).
func (s *MidiService) emitMidiFeedback(surfaceName string, mapping operatorsurface.MidiMapping, armed bool, appValue, physical float64) {
	s.events.QueueMidiFeedback(MidiFeedback{
		SurfaceName: surfaceName,
		MappingID:   mapping.ID.String(),
		Kind:        string(mapping.Kind),
		Armed:       armed,
		AppValue:    appValue,
		Physical:    physical,
	})
}

// SetActiveSurface selects surfaceName as the surface dispatchLoop
// arbitrates live MIDI input against, caching its current MidiMappings
// (D-07: mappings are per-surface -- only one surface's set is ever live
// at a time). The MidiPanel frontend calls this whenever the operator
// switches which surface they are viewing/operating.
func (s *MidiService) SetActiveSurface(surfaceName string) Result {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName)}
	}

	s.mu.Lock()
	s.activeSurface = surfaceName
	s.activeMappings = append([]operatorsurface.MidiMapping(nil), surface.MidiMappings...)
	s.mu.Unlock()

	return Result{Stdout: fmt.Sprintf("GOLC_MIDI_ACTIVE_SURFACE_SET: %s\n", surfaceName)}
}

// refreshActiveSurfaceMappings re-caches the active surface's mapping set
// from a just-Saved state, if surfaceName is currently active (StartLearn/
// RemoveMapping call this after a successful mutation so live arbitration
// immediately reflects the change without waiting for a separate
// SetActiveSurface call).
func (s *MidiService) refreshActiveSurfaceMappings(state show.State, surfaceName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeSurface != surfaceName {
		return
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		s.activeMappings = nil
		return
	}
	s.activeMappings = append([]operatorsurface.MidiMapping(nil), surface.MidiMappings...)
}

// StartLearn opens a bounded per-control capture window (D-05) for
// controlRef on surfaceName: it authorizes controlRef against the surface's
// assignment set (D-08 -- command.Authorize, this project's one D-04
// enforcement point, so learnable controls are exactly the controls
// assigned to the surface, never a separate list), waits for the next
// matching MIDI message (midi.CaptureCandidate, fed by dispatchLoop's
// route), checks the candidate for a conflict against the surface's
// existing mappings (midi.ProposeMapping, D-06/D-07), and on success
// persists it (operatorsurface.AddMidiMapping -> show.Save). StartLearn
// blocks its caller for up to learnCaptureTimeout (or until CancelLearn).
func (s *MidiService) StartLearn(surfaceName string, controlRef ControlRefInput) Result {
	s.mu.Lock()
	if s.driver == nil {
		s.mu.Unlock()
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_DRIVER_UNAVAILABLE: no MIDI input device is attached\n"}
	}
	if s.learning != nil {
		s.mu.Unlock()
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_LEARN_ALREADY_ACTIVE: a learn capture session is already in progress\n"}
	}
	s.mu.Unlock()

	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName)}
	}
	ref, err := resolveSurfaceControlRef(state, controlRef)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	if err := command.Authorize(surface, ref); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	session := &learnSession{next: make(chan midi.ControlKey, 1), cancel: make(chan struct{})}
	s.mu.Lock()
	s.learning = session
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.learning == session {
			s.learning = nil
		}
		s.mu.Unlock()
	}()

	timer := time.NewTimer(learnCaptureTimeout)
	defer timer.Stop()
	combinedTimeout := make(chan struct{})
	stopCombine := make(chan struct{})
	defer close(stopCombine)
	go func() {
		select {
		case <-timer.C:
			close(combinedTimeout)
		case <-session.cancel:
			close(combinedTimeout)
		case <-stopCombine:
		}
	}()

	candidate, err := midi.CaptureCandidate(session.next, combinedTimeout)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	// Reload in case the surface changed while this call was blocked
	// waiting for a physical message.
	state, err = show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	surface, found = surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName)}
	}

	if err := midi.ProposeMapping(controlKeysOf(surface.MidiMappings), candidate); err != nil {
		// Look up which existing mapping collided so the diagnostic can
		// embed the exact 06-UI-SPEC.md mapping-conflict copy ("That
		// Note/CC is already mapped to {control name}. Remove the
		// existing mapping before assigning it here.") verbatim after the
		// GOLC_MIDI_MAPPING_CONFLICT prefix -- the frontend strips the
		// prefix and renders the remainder as-is, rather than needing to
		// re-derive a control name from the raw error alone.
		if conflicting, found := findConflictingMapping(surface.MidiMappings, candidate); found {
			return Result{ExitCode: 1, Stderr: fmt.Sprintf(
				"GOLC_MIDI_MAPPING_CONFLICT: That Note/CC is already mapped to %q. Remove the existing mapping before assigning it here.\n",
				controlRefLabel(state, conflicting.Target),
			)}
		}
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	updated, err := operatorsurface.AddMidiMapping(surface, operatorsurface.MidiMapping{
		Channel: candidate.Channel,
		Kind:    toOperatorSurfaceKind(candidate.Kind),
		Number:  candidate.Number,
		Target:  ref,
	})
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, updated)
	if err := show.Save(s.root, s.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	s.refreshActiveSurfaceMappings(state, surfaceName)

	return Result{Stdout: fmt.Sprintf("GOLC_MIDI_LEARNED: channel=%d kind=%s number=%d\n", candidate.Channel, candidate.Kind, candidate.Number)}
}

// CancelLearn aborts the in-progress learn capture session (if any),
// unblocking its StartLearn call via combinedTimeout -- StartLearn's own
// caller sees midi.CaptureCandidate's GOLC_MIDI_LEARN_TIMEOUT error in
// that case; CancelLearn's own caller (the frontend's Cancel affordance)
// gets GOLC_MIDI_LEARN_CANCELLED directly.
func (s *MidiService) CancelLearn() Result {
	s.mu.Lock()
	session := s.learning
	if session == nil {
		s.mu.Unlock()
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_LEARN_NOT_ACTIVE: no learn capture session is in progress\n"}
	}
	s.learning = nil
	s.mu.Unlock()
	close(session.cancel)
	return Result{Stdout: "GOLC_MIDI_LEARN_CANCELLED\n"}
}

// RemoveMapping deletes the mapping identified by mappingID from
// surfaceName (the frontend's own "Remove Mapping" confirm copy gates the
// call). Removing an unknown ID is an idempotent no-op
// (operatorsurface.RemoveMidiMapping's own discipline).
func (s *MidiService) RemoveMapping(surfaceName, mappingID string) Result {
	id, err := uuid.Parse(mappingID)
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_MIDI_MAPPING_ID_INVALID: %q is not a valid mapping id\n", mappingID)}
	}

	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName)}
	}

	updated := operatorsurface.RemoveMidiMapping(surface, id)
	state.OperatorSurfaces = replaceSurfaceByID(state.OperatorSurfaces, updated)
	if err := show.Save(s.root, s.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	s.refreshActiveSurfaceMappings(state, surfaceName)

	s.mu.Lock()
	delete(s.takeovers, id.String())
	s.mu.Unlock()

	return Result{Stdout: fmt.Sprintf("GOLC_MIDI_MAPPING_REMOVED: %s\n", mappingID)}
}

// ListMappings returns surfaceName's current MIDI mapping rows (06-UI-SPEC.md
// populated/empty states: control name, Note/CC/channel, Remove
// affordance, armed chip for fader mappings) -- surfaceByName/show.Load
// mirror svc_surface.go's own read-path convention exactly. Label is
// computed server-side (layerKindLabel/safetyLabel, svc_surface.go's own
// helpers, reused verbatim) so the frontend never needs to re-derive a
// human-readable control name from raw kind/ID pieces.
type MidiMappingView struct {
	ID      string          `json:"id"`
	Channel int             `json:"channel"`
	Kind    string          `json:"kind"`
	Number  int             `json:"number"`
	Target  ControlRefInput `json:"target"`
	Label   string          `json:"label"`
}

// ListMappings projects surfaceName's MidiMappings for MidiPanel.tsx.
func (s *MidiService) ListMappings(surfaceName string) ([]MidiMappingView, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return nil, err
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return nil, fmt.Errorf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists", surfaceName)
	}
	views := make([]MidiMappingView, 0, len(surface.MidiMappings))
	for _, m := range surface.MidiMappings {
		views = append(views, MidiMappingView{
			ID:      m.ID.String(),
			Channel: m.Channel,
			Kind:    string(m.Kind),
			Number:  m.Number,
			Target:  controlRefInputOf(state, m.Target),
			Label:   controlRefLabel(state, m.Target),
		})
	}
	return views, nil
}

// controlRefInputOf projects an operatorsurface.ControlRef back into the
// wire ControlRefInput shape ListMappings returns. Scene/Group carry
// NAMES (resolved against state), matching ControlRefInput's established
// contract everywhere else in this package (svc_surface.go's own
// cliSelector/resolveSurfaceControlRef round-trip through names, never
// raw IDs) -- an earlier version of this function returned raw UUIDs
// here, which would have silently broken AssignItem/UnassignItem-style
// round-tripping of a mapping's Target back through the frontend (Rule 1
// fix, caught before any frontend code consumed this method).
func controlRefInputOf(state show.State, ref operatorsurface.ControlRef) ControlRefInput {
	switch ref.Kind {
	case operatorsurface.ControlScene:
		return ControlRefInput{Kind: "scene", Scene: sceneNameByID(state.Scenes, ref.Scene)}
	case operatorsurface.ControlLayer:
		return ControlRefInput{Kind: "layer", Scene: sceneNameByID(state.Scenes, ref.Layer.SceneID), LayerKind: string(ref.Layer.Kind)}
	case operatorsurface.ControlMaster:
		if ref.Master.Kind == operatorsurface.GrandMaster {
			return ControlRefInput{Kind: "master", MasterKind: "grand"}
		}
		return ControlRefInput{Kind: "master", MasterKind: "group", Group: groupNameByID(state.Groups, ref.Master.GroupID)}
	case operatorsurface.ControlSafety:
		return ControlRefInput{Kind: "safety", Safety: string(ref.Safety)}
	default:
		return ControlRefInput{}
	}
}

// controlRefLabel computes ref's human-readable label, reusing
// svc_surface.go's own layerKindLabel/safetyLabel helpers verbatim so
// there is exactly one label-formatting implementation in this package.
func controlRefLabel(state show.State, ref operatorsurface.ControlRef) string {
	switch ref.Kind {
	case operatorsurface.ControlScene:
		return sceneNameByID(state.Scenes, ref.Scene)
	case operatorsurface.ControlLayer:
		return fmt.Sprintf("%s / %s", sceneNameByID(state.Scenes, ref.Layer.SceneID), layerKindLabel(ref.Layer.Kind))
	case operatorsurface.ControlMaster:
		if ref.Master.Kind == operatorsurface.GrandMaster {
			return "Grand Master"
		}
		return fmt.Sprintf("Group Master: %s", groupNameByID(state.Groups, ref.Master.GroupID))
	case operatorsurface.ControlSafety:
		return safetyLabel(ref.Safety)
	default:
		return ""
	}
}

// sceneNameByID returns the Name of the scene in scenes whose ID matches
// id, or "" if not found (a mapping whose target scene was since deleted
// -- rendered as an empty label rather than a lookup failure, matching
// this file's read-only projection contract).
func sceneNameByID(scenes []scene.Scene, id uuid.UUID) string {
	for _, sc := range scenes {
		if sc.ID == id {
			return sc.Name
		}
	}
	return ""
}

// groupNameByID returns the Name of the group in groups whose ID matches
// id, or "" if not found.
func groupNameByID(groups []pool.Group, id uuid.UUID) string {
	for _, g := range groups {
		if g.ID == id {
			return g.Name
		}
	}
	return ""
}

// findMapping returns the mapping in mappings whose (Channel, Kind,
// Number) tuple matches key.
func findMapping(mappings []operatorsurface.MidiMapping, key midi.ControlKey) (operatorsurface.MidiMapping, bool) {
	for _, m := range mappings {
		if m.Channel == key.Channel && toMidiKind(m.Kind) == key.Kind && m.Number == key.Number {
			return m, true
		}
	}
	return operatorsurface.MidiMapping{}, false
}

// findConflictingMapping returns the mapping in mappings whose (Channel,
// Kind, Number) tuple matches candidate -- the same equality
// midi.ProposeMapping checks, used here only to recover the conflicting
// mapping's Target for the UI-SPEC mapping-conflict copy (StartLearn).
func findConflictingMapping(mappings []operatorsurface.MidiMapping, candidate midi.ControlKey) (operatorsurface.MidiMapping, bool) {
	return findMapping(mappings, candidate)
}

// controlKeysOf converts surface mappings into midi.ControlKey values for
// midi.ProposeMapping's conflict check.
func controlKeysOf(mappings []operatorsurface.MidiMapping) []midi.ControlKey {
	keys := make([]midi.ControlKey, 0, len(mappings))
	for _, m := range mappings {
		keys = append(keys, midi.ControlKey{Channel: m.Channel, Kind: toMidiKind(m.Kind), Number: m.Number})
	}
	return keys
}

// toOperatorSurfaceKind converts a live midi.MessageKind into its
// persisted operatorsurface.MidiMessageKind counterpart.
func toOperatorSurfaceKind(k midi.MessageKind) operatorsurface.MidiMessageKind {
	if k == midi.ControlChange {
		return operatorsurface.ControlChange
	}
	return operatorsurface.Note
}

// toMidiKind converts a persisted operatorsurface.MidiMessageKind into its
// live midi.MessageKind counterpart.
func toMidiKind(k operatorsurface.MidiMessageKind) midi.MessageKind {
	if k == operatorsurface.ControlChange {
		return midi.ControlChange
	}
	return midi.Note
}

// replaceSurfaceByID returns a copy of surfaces with the entry whose ID
// matches updated.ID replaced by updated -- mirrors
// internal/command/operatorsurface.go's replaceOperatorSurface exactly
// (a distinct copy is required here since that function is unexported to
// the command package).
func replaceSurfaceByID(surfaces []operatorsurface.Surface, updated operatorsurface.Surface) []operatorsurface.Surface {
	out := make([]operatorsurface.Surface, len(surfaces))
	for i, existing := range surfaces {
		if existing.ID == updated.ID {
			out[i] = updated
			continue
		}
		out[i] = existing
	}
	return out
}
