// svc_programming.go fills ProgrammingService, the Wails binding closing
// VERIFICATION.md Gap B[0] for PLAY-12 (06-12-PLAN.md): a show author
// creates bar-loop scenes, creates each of the four reusable look kinds
// (color theme, chase, motion preset, and a base-look preset via
// "programmer set" + "preset record"), enables and points each of a
// scene's four fixed layers at a reusable look, activates exactly one
// scene, and creates reusable blend presets -- every mutation executes the
// matching already-implemented, already-tested "scene"/"theme"/"chase"/
// "motion"/"programmer"/"preset"/"blend" CLI route (internal/command/
// scene.go, internal/command/programming.go) via
// command.NewDefaultCommandRegistry, exactly the SurfaceService/
// FixturePatchService pattern (svc_surface.go/svc_fixturepatch.go) this
// file mirrors -- so there is only one scene/programming mutation
// implementation in this codebase, never a second one duplicated for the
// GUI.
//
// SetSceneLayer mirrors PlaybackService.SetLayerEnabled's exact
// Ref-preserving pre-read discipline (svc_playback.go's currentLayerRef/
// WR-01/WR-03 doc comments): it reads the layer's currently assigned Ref
// before every mutating call and re-supplies it whenever the caller does
// not explicitly pass a new refID, so a disable/re-enable toggle -- or
// pointing one layer while leaving another untouched -- never silently
// nulls out a previously assigned base-look/color-theme/chase/motion
// reference ([Rule 2] auto-added: the same discipline PLAY-01's own
// toggle contract already requires).
//
// ListProgramming reads the ShowState directly (there is no registered
// read route to reuse, mirroring GetState/ListPatch/ShowSurface's
// identical rationale) and projects every scene's layer/active state plus
// every theme/preset/chase/motion-preset/blend-preset "look" into a
// JSON-safe view for SceneProgramming.tsx, including a flattened list of
// every deployment instance (id + human-readable label) so the frontend's
// minimal "programmer set" control has something concrete to select
// against without duplicating FixturePatch.tsx's own pool/deployment
// authoring surface.
//
// Simplified-subset boundary (Claude's Discretion, 06-12-PLAN.md flagged
// assumption -- no CONTEXT decision covers PLAY-12): this file binds the
// core authoring path required by PLAY-12 -- create scene, create/record
// each look kind, enable+point the four scene layers, activate a scene,
// create a blend -- plus enough of "programmer set"'s selection+attribute
// grammar to record a base-look/color preset (instance selectors and
// capability=value attribute pairs only, no pool/group/fixture
// selectors). internal/command/programming.go's full rename/reorder/
// duplicate/delete surface and the complete programmer attribute matrix
// remain the CLI's own full-fidelity path this round; they are a
// documented simplified subset, never silently dropped scope.
package wails

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// ProgrammingService is bound to the frontend via cmd/golc-desktop/
// main.go's options.App{Bind: [...]}. root/showPath are the exact
// ShowState location every method Loads/Saves against (mirrors
// SurfaceService/FixturePatchService's own fields).
type ProgrammingService struct {
	pipeName string
	root     string
	showPath string
}

// NewProgrammingService constructs a ProgrammingService targeting
// pipeName (reserved, unused by this ShowState-only CRUD -- mirrors
// SurfaceService/FixturePatchService's own unused pipeName field) and the
// ShowState at showPath, resolved against root.
func NewProgrammingService(pipeName, root, showPath string) *ProgrammingService {
	return &ProgrammingService{pipeName: pipeName, root: root, showPath: showPath}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_surface.go/svc_fixturepatch.go's identical
// helper).
func (s *ProgrammingService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// CreateScene creates a new named bar-loop scene (SCEN-01) via
// "scene create <name> --bars <n> --show <path>".
func (s *ProgrammingService) CreateScene(name string, bars int) Result {
	return s.execute("scene", "create", name, "--bars", strconv.Itoa(bars), "--show", s.showPath)
}

// ActivateScene marks name the exactly-one active scene (SCEN-04) via
// "scene activate <name> --show <path>", deactivating every other scene.
func (s *ProgrammingService) ActivateScene(name string) Result {
	return s.execute("scene", "activate", name, "--show", s.showPath)
}

// currentLayerRef reads the target scene/kind's currently assigned Ref
// (the zero UUID if the scene/layer does not exist) via a read-only
// show.Load -- see the package doc comment for why SetSceneLayer needs
// this before its mutating call. A genuine show.Load failure is returned
// as an error rather than folded into "no ref assigned" (mirrors
// PlaybackService.currentLayerRef's identical WR-01 discipline).
func (s *ProgrammingService) currentLayerRef(sceneName, kind string) (uuid.UUID, error) {
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

// SetSceneLayer enables/points one of a scene's four fixed layers via
// "scene layer set <scene> --kind <kind> [--ref <resolved>] [--disable]
// --show <path>" (SCEN-01/SCEN-05): refID (when non-empty) must parse as a
// UUID and becomes the layer's new Ref; an empty refID re-supplies the
// layer's CURRENT Ref (the currentLayerRef pre-read) so a disable/
// re-enable toggle -- or pointing one layer kind while leaving another
// untouched -- never discards a previously assigned reference (WR-01/
// WR-03, see package doc comment). An unknown scene name or layer kind
// surfaces the route's own diagnostic, never a panic.
func (s *ProgrammingService) SetSceneLayer(sceneName, kind, refID string, enabled bool) Result {
	existingRef, err := s.currentLayerRef(sceneName, kind)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}

	ref := existingRef
	if refID != "" {
		parsed, parseErr := uuid.Parse(refID)
		if parseErr != nil {
			return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_PROGRAMMING_REF_INVALID: %q is not a valid UUID\n", refID)}
		}
		ref = parsed
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

// CreateTheme creates a new named reusable color theme (PROG-04) via
// "theme create <name> --show <path>".
func (s *ProgrammingService) CreateTheme(name string) Result {
	return s.execute("theme", "create", name, "--show", s.showPath)
}

// CreateMotion creates a new named reusable motion preset (PROG-06) via
// "motion create <name> --show <path>".
func (s *ProgrammingService) CreateMotion(name string) Result {
	return s.execute("motion", "create", name, "--show", s.showPath)
}

// CreateChase creates a new named reusable chase (PROG-05) via
// "chase create <name> --unit <unit> --step-duration <stepDuration>
// --show <path>". unit must be "bar" or "beat"; an invalid unit or
// non-positive stepDuration surfaces the route's own diagnostic.
func (s *ProgrammingService) CreateChase(name, unit string, stepDuration float64) Result {
	return s.execute("chase", "create", name,
		"--unit", unit,
		"--step-duration", strconv.FormatFloat(stepDuration, 'f', -1, 64),
		"--show", s.showPath)
}

// ProgrammerSet resolves instanceIDs (deployment instance UUIDs) and sets
// every "capability=value" pair in attrs on the resolved selection via
// "programmer set --instance <id>... --attr <capability>=<value>...
// --show <path>" (PROG-01/PROG-02) -- the minimal selection+attribute
// grammar this simplified subset binds (instance selectors only; see
// package doc comment), enough to stage a RecordPreset call. A dangling
// instance, an out-of-range value, or an unsupported capability surfaces
// the route's own diagnostic, never a panic.
func (s *ProgrammingService) ProgrammerSet(instanceIDs []string, attrs []string) Result {
	args := []string{"programmer", "set"}
	for _, id := range instanceIDs {
		args = append(args, "--instance", id)
	}
	for _, attr := range attrs {
		args = append(args, "--attr", attr)
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args...)
}

// RecordPreset records a new kind-scoped preset (PROG-04) from the
// persisted Programmer buffer via "preset record <name> --kind <kind>
// --show <path>". kind must be one of intensity/color/position/beam; an
// unknown kind surfaces the route's own GOLC_PRESET_KIND_INVALID
// diagnostic. A base-look scene layer's Ref may point at any recorded
// preset regardless of kind (internal/scene/scene.go's
// ValidateLayerReferences imposes no kind restriction on a BaseLook Ref).
func (s *ProgrammingService) RecordPreset(name, kind string) Result {
	return s.execute("preset", "record", name, "--kind", kind, "--show", s.showPath)
}

// CreateBlend creates a new named reusable blend preset (SCEN-07) via
// "blend create <name> --duration-bars <durationBars> [--curve <curve>]
// --show <path>". An empty curve omits the flag, letting the route default
// to scene.BlendCurveLinear.
func (s *ProgrammingService) CreateBlend(name string, durationBars float64, curve string) Result {
	args := []string{"blend", "create", name, "--duration-bars", strconv.FormatFloat(durationBars, 'f', -1, 64)}
	if curve != "" {
		args = append(args, "--curve", curve)
	}
	args = append(args, "--show", s.showPath)
	return s.execute(args...)
}

// ProgLayerView is the JSON-safe rendering of one scene.Layer for
// ListProgramming.
type ProgLayerView struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
	Ref     string `json:"ref,omitempty"`
}

// ProgSceneView is the JSON-safe rendering of one scene.Scene for
// ListProgramming.
type ProgSceneView struct {
	Name   string          `json:"name"`
	Active bool            `json:"active"`
	Bars   int             `json:"barsPerLoop"`
	Layers []ProgLayerView `json:"layers"`
}

// ProgLookView is one reusable look's JSON-safe row (theme/chase/motion
// preset/blend preset all share this id+name shape).
type ProgLookView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProgPresetView is one recorded preset's JSON-safe row -- a look plus
// its kind (intensity/color/position/beam), since a base-look layer's Ref
// must resolve against a preset specifically (see RecordPreset doc
// comment).
type ProgPresetView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// ProgInstanceView is one deployment instance's minimal JSON-safe row --
// just enough (id + a human-readable label) for the frontend's
// simplified-subset "programmer set" instance picker (see package doc
// comment); it never duplicates FixturePatch.tsx's own pool/deployment
// authoring surface.
type ProgInstanceView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ProgrammingView is ListProgramming's full JSON-safe payload: every
// scene's layer/active state, every reusable look collection, and every
// deployment instance -- everything SceneProgramming.tsx needs to render
// without the author already knowing scene/look names or instance IDs by
// heart.
type ProgrammingView struct {
	Scenes    []ProgSceneView    `json:"scenes"`
	Themes    []ProgLookView     `json:"themes"`
	Presets   []ProgPresetView   `json:"presets"`
	Chases    []ProgLookView     `json:"chases"`
	Motions   []ProgLookView     `json:"motions"`
	Blends    []ProgLookView     `json:"blends"`
	Instances []ProgInstanceView `json:"instances"`
}

// ListProgramming reads the show document directly (read-only show.Load
// -- see package doc comment for why this bypasses the command registry:
// no such read route exists) and returns every scene's layer/active
// state, every theme/preset/chase/motion-preset/blend-preset, and every
// deployment instance as canonical JSON. A show that fails to load
// surfaces its own diagnostic in the returned error, never a panic. An
// empty/fresh show returns an explicit empty projection (every slice
// present but zero-length) rather than nulls, so the frontend's own
// zero-one-many rendering never has to distinguish "not yet loaded" from
// "genuinely empty."
func (s *ProgrammingService) ListProgramming() (ProgrammingView, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return ProgrammingView{}, err
	}

	view := ProgrammingView{
		Scenes:    make([]ProgSceneView, 0, len(state.Scenes)),
		Themes:    make([]ProgLookView, 0, len(state.Themes)),
		Presets:   make([]ProgPresetView, 0, len(state.Presets)),
		Chases:    make([]ProgLookView, 0, len(state.Chases)),
		Motions:   make([]ProgLookView, 0, len(state.MotionPresets)),
		Blends:    make([]ProgLookView, 0, len(state.BlendPresets)),
		Instances: make([]ProgInstanceView, 0),
	}

	for _, sc := range state.Scenes {
		layers := make([]ProgLayerView, 0, len(sc.Layers))
		for _, layer := range sc.Layers {
			lv := ProgLayerView{Kind: string(layer.Kind), Enabled: layer.Enabled}
			if layer.Ref != uuid.Nil {
				lv.Ref = layer.Ref.String()
			}
			layers = append(layers, lv)
		}
		view.Scenes = append(view.Scenes, ProgSceneView{
			Name:   sc.Name,
			Active: sc.Active,
			Bars:   sc.BarsPerLoop,
			Layers: layers,
		})
	}

	for _, th := range state.Themes {
		view.Themes = append(view.Themes, ProgLookView{ID: th.ID.String(), Name: th.Name})
	}
	for _, p := range state.Presets {
		view.Presets = append(view.Presets, ProgPresetView{ID: p.ID.String(), Name: p.Name, Kind: string(p.Kind)})
	}
	for _, c := range state.Chases {
		view.Chases = append(view.Chases, ProgLookView{ID: c.ID.String(), Name: c.Name})
	}
	for _, m := range state.MotionPresets {
		view.Motions = append(view.Motions, ProgLookView{ID: m.ID.String(), Name: m.Name})
	}
	for _, b := range state.BlendPresets {
		view.Blends = append(view.Blends, ProgLookView{ID: b.ID.String(), Name: b.Name})
	}
	for _, d := range state.Deployments {
		for _, instance := range d.Instances {
			view.Instances = append(view.Instances, ProgInstanceView{
				ID:    instance.ID.String(),
				Label: fmt.Sprintf("%s / %s (U%d:A%d)", d.Name, instance.Mode, instance.Universe, instance.Address),
			})
		}
	}

	// payload is discarded here -- ListProgramming returns the Go struct
	// directly (Wails marshals it for the frontend); the canonical-encode
	// round trip only proves the view is JSON-safe (mirrors ListPatch/
	// ShowSurface never calling strictjson themselves, but GetState does
	// for its own Result.Stdout-carried payload -- this method's own
	// signature returns a real Go value, not a Result, so no such carry is
	// needed here beyond this safety check).
	if _, err := strictjson.CanonicalEncode(view); err != nil {
		return ProgrammingView{}, fmt.Errorf("GOLC_WAILS_PROGRAMMING_STATE_ENCODE_FAILED: %w", err)
	}
	return view, nil
}
