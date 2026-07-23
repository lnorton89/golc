// svc_playback.go fills 06-04-PLAN.md Task 1's PlaybackService stub
// (06-06-PLAN.md Task 1, PLAY-01/02): every bound method issues the exact
// route/args a matching CLI route in internal/command/playback.go or
// internal/command/scene.go already expects, executed in-process through a
// freshly built default command registry -- the identical show.Load-
// mutate-show.Save path a CLI invocation of the same route takes
// (06-RESEARCH.md Architectural Responsibility Map: "no new playback
// authority introduced"). None of these routes are daemon-resident
// (internal/artnet/daemon.go's handle switch has no playback/scene case --
// only artnet safety/configure/master, which svc_safety.go binds
// separately over IPC), so every method here executes directly against the
// on-disk show document, never dialing the daemon.
//
// GetState is the one read-only addition beyond a literal CLI mirror: no
// existing CLI route lists scenes/layers, and an on-screen scene
// selector/layer-toggle surface is unusable without knowing what exists to
// switch to. GetState calls show.Load directly (there is no registered
// route to reuse for a read) and returns a JSON-safe projection -- purely
// read-only, so it introduces no new mutation authority.
//
// SetLayerEnabled also reads via show.Load immediately before its mutating
// registry call, solely to preserve the target layer's existing Ref:
// internal/command/scene.go's own WR-03 doc comment documents that "scene
// layer set" only merges Selection when a selector flag is omitted on a
// given invocation, never Ref -- an enable/disable toggle that omitted
// this pre-read would silently null out a previously assigned base-look/
// color-theme/chase/motion reference on every flip ([Rule 2] auto-added:
// PLAY-01's own toggle contract must not destroy other authored state).
package wails

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// PlaybackService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. showPath/root are the exact "--show <path>"
// and command.Request.Root values every bound method's registry call
// uses -- the same show document and project root the supervised daemon
// (internal/wails/app.go's ensureDaemon) is configured with.
type PlaybackService struct {
	pipeName string
	showPath string
	root     string

	mu            sync.Mutex
	activeSurface string
}

// NewPlaybackService constructs a PlaybackService targeting pipeName (kept
// for parity with the other feature services even though no method in
// this file dials the daemon -- see the package doc comment above),
// showPath, and root.
func NewPlaybackService(pipeName, showPath, root string) *PlaybackService {
	return &PlaybackService{pipeName: pipeName, showPath: showPath, root: root}
}

// SetActiveSurface selects surfaceName as the operator surface
// PlaybackService's mutating methods (SwitchScene/SetLayerEnabled)
// authorize against (CR-01 fix): while an active surface is set, a call
// against a scene/layer control not assigned to it is rejected
// server-side by authorizeControl, mirroring MidiService's own
// activeSurface pattern and SafetyService's identical CR-01 fix. Passing
// "" clears the active surface, returning to unrestricted/author-mode
// dispatch. SetBPM/TapTempo/Evaluate are never gated: BPM/tempo has no
// corresponding operatorsurface.ControlKind (only scene/layer/master/
// safety are individually-assignable controls, internal/operatorsurface/
// model.go), so there is nothing for those methods to authorize against.
func (s *PlaybackService) SetActiveSurface(surfaceName string) Result {
	if surfaceName == "" {
		s.mu.Lock()
		s.activeSurface = ""
		s.mu.Unlock()
		return Result{Stdout: "GOLC_PLAYBACK_ACTIVE_SURFACE_CLEARED\n"}
	}

	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	if _, found := surfaceByName(state.OperatorSurfaces, surfaceName); !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName)}
	}

	s.mu.Lock()
	s.activeSurface = surfaceName
	s.mu.Unlock()
	return Result{Stdout: fmt.Sprintf("GOLC_PLAYBACK_ACTIVE_SURFACE_SET: %s\n", surfaceName)}
}

// authorizeControl is CR-01's server-side visible-but-locked enforcement
// point for scene/layer playback actions: when an active operator surface
// has been set (SetActiveSurface), ref must be a member of that surface's
// assignment set (command.Authorize, internal/command/operatorsurface.go's
// D-04 enforcement) before the mutating call may dispatch. No active
// surface (the default) means unrestricted/author-mode dispatch --
// matching this service's pre-CR-01 behavior exactly, so a caller that
// never opts into operator-surface scoping never regresses.
func (s *PlaybackService) authorizeControl(state show.State, ref operatorsurface.ControlRef) error {
	s.mu.Lock()
	surfaceName := s.activeSurface
	s.mu.Unlock()
	if surfaceName == "" {
		return nil
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return fmt.Errorf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists", surfaceName)
	}
	return command.Authorize(surface, ref)
}

// authorizeScene loads the show, resolves sceneName to its ControlRef, and
// calls authorizeControl (CR-01). An unknown scene name is never treated
// as an authorization failure here -- it is left for the underlying
// registry route's own GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE diagnostic to
// surface, exactly as it did before this fix, so authorization only ever
// adds a new rejection reason, never changes an existing one.
func (s *PlaybackService) authorizeScene(sceneName string) error {
	s.mu.Lock()
	surfaceName := s.activeSurface
	s.mu.Unlock()
	if surfaceName == "" {
		return nil
	}
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return err
	}
	for _, sc := range state.Scenes {
		if sc.Name == sceneName {
			return s.authorizeControl(state, operatorsurface.SceneControlRef(sc.ID))
		}
	}
	return nil
}

// authorizeLayer loads the show, resolves sceneName/kind to its
// ControlRef, and calls authorizeControl (CR-01). An unknown scene/layer
// kind is left for the underlying registry route's own diagnostic,
// mirroring authorizeScene's identical discipline.
func (s *PlaybackService) authorizeLayer(sceneName, kind string) error {
	s.mu.Lock()
	surfaceName := s.activeSurface
	s.mu.Unlock()
	if surfaceName == "" {
		return nil
	}
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return err
	}
	for _, sc := range state.Scenes {
		if sc.Name != sceneName {
			continue
		}
		if _, ok := sc.LayerByKind(scene.LayerKind(kind)); ok {
			return s.authorizeControl(state, operatorsurface.LayerControlRef(operatorsurface.LayerRef{SceneID: sc.ID, Kind: scene.LayerKind(kind)}))
		}
	}
	return nil
}

// execute runs a full route-plus-args word sequence (e.g. "playback",
// "switch", "Verse", "--show", s.showPath) through a freshly built default
// command registry -- the identical in-process path
// cmd/golc-project/main.go's run() takes, so a Wails-bound call and a CLI
// invocation of the exact same route behave identically. A registry-build
// failure (which would only ever happen from a duplicate-registration
// programming error, never live input) surfaces as a Result rather than a
// panic, matching every other route's own never-panic contract.
func (s *PlaybackService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_REGISTRY_BUILD_FAILED: " + err.Error()}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// SwitchScene issues "playback switch <scene> --show <path>" (PLAY-01
// scene switch). An unknown scene name surfaces the route's own
// GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE diagnostic, never a panic.
// authorizeScene (CR-01) gates dispatch first.
func (s *PlaybackService) SwitchScene(sceneName string) Result {
	if err := s.authorizeScene(sceneName); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}
	return s.execute("playback", "switch", sceneName, "--show", s.showPath)
}

// currentLayerRef reads the target scene/kind's currently assigned Ref (the
// zero UUID if the scene/layer simply does not exist) via a read-only
// show.Load -- see the package doc comment for why SetLayerEnabled needs
// this before its mutating call. WR-01 fix: a genuine show.Load failure
// (e.g. a transient I/O error) is returned as an error rather than folded
// into the same "no ref assigned" zero-UUID result -- SetLayerEnabled must
// be able to tell "no ref assigned" (safe to proceed without --ref) apart
// from "couldn't read the show at all" (unsafe to proceed, since a
// concurrent second show.Load inside the mutating registry call succeeding
// where this one failed would otherwise silently discard whatever Ref was
// actually on disk).
func (s *PlaybackService) currentLayerRef(sceneName, kind string) (uuid.UUID, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return uuid.Nil, err
	}
	for _, sc := range state.Scenes {
		if sc.Name != sceneName {
			continue
		}
		if layer, ok := sc.LayerByKind(scene.LayerKind(kind)); ok {
			return layer.Ref, nil
		}
	}
	return uuid.Nil, nil
}

// SetLayerEnabled toggles one scene layer's Enabled flag via
// "scene layer set <scene> --kind <kind> [--ref <preserved>] [--disable]
// --show <path>" (PLAY-01 layer enable/disable), re-supplying the layer's
// current Ref so the toggle never discards it (see package doc comment).
// Selection is left unmentioned entirely -- the route's own WR-03 merge
// behavior carries the existing Selection forward automatically. An
// unknown scene name or layer kind surfaces the route's own diagnostic,
// never a panic. authorizeLayer (CR-01) gates dispatch first. WR-01 fix: a
// pre-read failure from currentLayerRef is returned as this call's own
// Result rather than silently proceeding as if no Ref were assigned.
func (s *PlaybackService) SetLayerEnabled(sceneName, kind string, enabled bool) Result {
	if err := s.authorizeLayer(sceneName, kind); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}
	ref, err := s.currentLayerRef(sceneName, kind)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	args := []string{"scene", "layer", "set", sceneName, "--kind", kind}
	if ref != uuid.Nil {
		args = append(args, "--ref", ref.String())
	}
	if !enabled {
		args = append(args, "--disable")
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args...)
}

// SetBPM issues "playback bpm set <bpm> --show <path>" (PLAY-01 numeric
// BPM entry). A non-positive/non-finite value surfaces the route's own
// GOLC_PLAYBACK_BPM_INVALID diagnostic, never a panic.
func (s *PlaybackService) SetBPM(bpm float64) Result {
	return s.execute("playback", "bpm", "set", strconv.FormatFloat(bpm, 'f', -1, 64), "--show", s.showPath)
}

// TapTempo issues "playback bpm tap --at <ts> --at <ts> ... --show <path>"
// (PLAY-01 tap tempo). timestamps are RFC3339(Nano) strings -- the exact
// format JavaScript's Date.prototype.toISOString() produces, which
// internal/command/playback.go's parseTapTimestamp already accepts. Fewer
// than two taps surfaces the route's own GOLC_PLAYBACK_TAP_INVALID
// diagnostic, never a panic.
func (s *PlaybackService) TapTempo(timestamps []string) Result {
	args := []string{"playback", "bpm", "tap"}
	for _, ts := range timestamps {
		args = append(args, "--at", ts)
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args...)
}

// Evaluate issues "playback evaluate --at <bar>.<beatfraction> --json
// --show <path>" (PLAY-01 transport/evaluate preview): a read-only,
// deterministic demonstration of the compiled active scene at a given
// musical position -- it never mutates show state and never drives the
// live Art-Net daemon, which free-runs its own wall-clock position
// independent of this call (see package doc comment). No active scene, or
// an otherwise-invalid compiled plan, surfaces the route's own
// GOLC_PLAYBACK_NO_ACTIVE_SCENE/GOLC_PLAYBACK_PLAN_INVALID diagnostic,
// never a panic.
func (s *PlaybackService) Evaluate(at float64) Result {
	return s.execute("playback", "evaluate", "--at", strconv.FormatFloat(at, 'f', -1, 64), "--json", "--show", s.showPath)
}

// layerSummary is the JSON-safe rendering of one scene.Layer for GetState.
type layerSummary struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
	Ref     string `json:"ref,omitempty"`
}

// sceneSummary is the JSON-safe rendering of one scene.Scene for GetState.
type sceneSummary struct {
	Name   string         `json:"name"`
	Active bool           `json:"active"`
	Bars   int            `json:"barsPerLoop"`
	Layers []layerSummary `json:"layers"`
}

// playbackStateSummary is GetState's full JSON-safe payload: every scene's
// name/active flag/layer set, plus the show-wide BPM -- everything an
// on-screen scene selector, layer-toggle grid, and BPM readout need to
// render without the operator already knowing scene/layer names by heart.
type playbackStateSummary struct {
	BPM    float64        `json:"bpm"`
	Scenes []sceneSummary `json:"scenes"`
}

// GetState reads the show document directly (read-only show.Load -- see
// package doc comment for why this bypasses the command registry rather
// than mirroring a CLI route: no such read route exists) and returns
// every scene's layer/active state plus the current BPM as canonical
// JSON. A show that fails to load surfaces its own diagnostic in
// Result.Stderr, never a panic.
func (s *PlaybackService) GetState() Result {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}

	summary := playbackStateSummary{BPM: state.Tempo.BPM}
	for _, sc := range state.Scenes {
		layers := make([]layerSummary, 0, len(sc.Layers))
		for _, layer := range sc.Layers {
			ls := layerSummary{Kind: string(layer.Kind), Enabled: layer.Enabled}
			if layer.Ref != uuid.Nil {
				ls.Ref = layer.Ref.String()
			}
			layers = append(layers, ls)
		}
		summary.Scenes = append(summary.Scenes, sceneSummary{
			Name:   sc.Name,
			Active: sc.Active,
			Bars:   sc.BarsPerLoop,
			Layers: layers,
		})
	}

	payload, err := strictjson.CanonicalEncode(summary)
	if err != nil {
		return Result{ExitCode: 1, Stderr: "GOLC_WAILS_PLAYBACK_STATE_ENCODE_FAILED: " + err.Error()}
	}
	return Result{Stdout: string(payload)}
}
