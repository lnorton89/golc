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
	"sync"
	"time"

	"github.com/google/uuid"

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
// mapping set (refreshed by SetActiveSurface/StartLearn/RemoveMapping) and
// arbitrates it: Note-kind mappings report Armed=true immediately (D-12,
// no pickup/arming state); ControlChange-kind mappings run
// midi.TakeoverState.Update with the unthrottled physical value (D-11) and
// report the resulting armed state + controlling/ghost value (D-09/D-10).
// A message matching no mapping on the active surface is silently
// dropped -- there is nothing to arbitrate or feed back for it.
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

	if mapping.Kind == operatorsurface.Note {
		s.emitMidiFeedback(surfaceName, mapping, true, evt.Value, evt.Value)
		return
	}

	state := s.takeoverStateFor(mapping.ID)
	armed, controlValue := state.Update(evt.Value)
	s.emitMidiFeedback(surfaceName, mapping, armed, controlValue, evt.Value)
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
	s.mu.Unlock()
	if session == nil {
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_LEARN_NOT_ACTIVE: no learn capture session is in progress\n"}
	}
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
