// svc_surface.go fills SurfaceService (06-04-PLAN.md Task 1 stub) with the
// operator-surface builder's real bindings (06-07-PLAN.md Task 1, PLAY-03
// D-01..D-04): every mutation (CreateSurface/AssignItem/UnassignItem/
// RemoveSurface) executes the matching self-registered "operatorsurface"
// CLI route (internal/command/operatorsurface.go, 06-01) via
// command.NewDefaultCommandRegistry -- Load -> mutate -> Save -- exactly
// the same code path a "golc-project.exe operatorsurface ..." invocation
// would take, so there is only one operator-surface mutation
// implementation in this codebase, never a second one duplicated for the
// GUI. ListSurfaces/ShowSurface read the ShowState directly and project it
// into a JSON-safe view shape for the frontend (list rows, per-control
// assignment membership) since the CLI's own text Result isn't structured
// data. AuthorizeControl is the server-side visible-but-locked enforcement
// point (D-04/ASVS V4, threat T-06-18): it resolves a control reference
// against the loaded ShowState and calls command.Authorize before any
// operator-mode action against that control may proceed -- the frontend's
// own disabled/locked rendering (OperatorSurface.tsx) is never trusted as
// the sole enforcement.
package wails

import (
	"fmt"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// SurfaceService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. root/showPath are the exact ShowState location
// every method Loads/Saves against (mirrors internal/wails.Config's own
// ProjectRoot/ShowPath fields).
type SurfaceService struct {
	pipeName string
	root     string
	showPath string
}

// NewSurfaceService constructs a SurfaceService targeting pipeName (reserved
// for a future daemon-side operator-mode dispatch call; unused by this
// plan's ShowState-only CRUD) and the ShowState at showPath, resolved
// against root.
func NewSurfaceService(pipeName, root, showPath string) *SurfaceService {
	return &SurfaceService{pipeName: pipeName, root: root, showPath: showPath}
}

// Ping is a placeholder Wails-bound method proving the service registers
// and builds; kept for parity with the other 06-04 scaffold stubs
// (SafetyService/PlaybackService/MidiService) until their own plans land.
func (s *SurfaceService) Ping() Result {
	return Result{ExitCode: 2, Stderr: "GOLC_WAILS_NOT_IMPLEMENTED: Ping is a 06-04 scaffold placeholder; use the real SurfaceService methods instead"}
}

// ControlRefInput is the Wails-bound JSON shape identifying one
// individually-assignable control (D-03: no bulk/category variant exists
// here or anywhere else), mirroring the exact selector grammar
// internal/command/operatorsurface.go's CLI routes already accept
// (--scene <name> | --layer <scene>:<kind> | --master grand|group:<name> |
// --safety <control>) so AssignItem/UnassignItem/AuthorizeControl execute
// against the identical selector a CLI invocation would build, and this
// file never reimplements operatorsurface's own name-resolution logic
// twice.
type ControlRefInput struct {
	Kind       string `json:"kind"`                 // "scene" | "layer" | "master" | "safety"
	Scene      string `json:"scene,omitempty"`       // scene name -- for kind="scene" or kind="layer"
	LayerKind  string `json:"layerKind,omitempty"`   // "base_look" | "color_theme" | "chase" | "motion" -- for kind="layer"
	MasterKind string `json:"masterKind,omitempty"`  // "grand" | "group" -- for kind="master"
	Group      string `json:"group,omitempty"`       // group name -- for kind="master" with masterKind="group"
	Safety     string `json:"safety,omitempty"`      // "blackout" | "stop_release_all" | "revoke_automation" -- for kind="safety"
}

// ControlRefView is ControlRefInput's read side: the same selector fields
// (so the frontend can echo one straight back into AssignItem/UnassignItem/
// AuthorizeControl) plus a human-readable Label and the control's current
// Assigned membership on the surface ShowSurface was called for. ShowSurface
// always returns every assignable control in the show, assigned or not --
// D-04's visible-but-locked rendering needs the full set, never only the
// assigned subset.
type ControlRefView struct {
	ControlRefInput
	Label    string `json:"label"`
	Assigned bool   `json:"assigned"`
}

// SurfaceSummary is one ListSurfaces row (06-UI-SPEC.md "populated" state:
// name, assigned scene/layer/master count, MIDI-mapping count).
type SurfaceSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	SceneCount       int    `json:"sceneCount"`
	LayerCount       int    `json:"layerCount"`
	MasterCount      int    `json:"masterCount"`
	SafetyCount      int    `json:"safetyCount"`
	AssignedCount    int    `json:"assignedCount"` // SceneCount+LayerCount+MasterCount
	MidiMappingCount int    `json:"midiMappingCount"`
}

// SurfaceDetail is ShowSurface's return shape: the named surface plus every
// assignable control in the show (see ControlRefView doc comment).
type SurfaceDetail struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Controls         []ControlRefView `json:"controls"`
	MidiMappingCount int              `json:"midiMappingCount"`
}

// surfaceLayerKindOrder is the fixed, deterministic layer-kind enumeration
// order ShowSurface walks for every scene (mirrors scene.go's own
// layerPriority order; every Scene always carries exactly one Layer per
// kind, so this is safe to enumerate unconditionally).
var surfaceLayerKindOrder = []scene.LayerKind{scene.BaseLook, scene.ColorTheme, scene.Chase, scene.Motion}

// surfaceSafetyOrder is the fixed, deterministic safety-control enumeration
// order ShowSurface walks (the three-member closed SafetyControl enum).
var surfaceSafetyOrder = []operatorsurface.SafetyControl{
	operatorsurface.Blackout, operatorsurface.StopReleaseAll, operatorsurface.RevokeAutomation,
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors cmd/golc-project/main.go's run() -- the identical
// Load -> mutate -> Save path a CLI invocation would take).
func (s *SurfaceService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// CreateSurface creates a new named operator surface (D-02) via
// "operatorsurface create".
func (s *SurfaceService) CreateSurface(name string) Result {
	return s.execute([]string{"operatorsurface", "create", name, "--show", s.showPath})
}

// ListSurfaces returns every operator surface on the loaded ShowState as a
// row summary for SurfaceList.tsx (populated/empty/zero-one-many states).
func (s *SurfaceService) ListSurfaces() ([]SurfaceSummary, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return nil, err
	}
	summaries := make([]SurfaceSummary, 0, len(state.OperatorSurfaces))
	for _, surface := range state.OperatorSurfaces {
		summaries = append(summaries, SurfaceSummary{
			ID:               surface.ID.String(),
			Name:             surface.Name,
			SceneCount:       len(surface.SceneRefs),
			LayerCount:       len(surface.LayerRefs),
			MasterCount:      len(surface.MasterRefs),
			SafetyCount:      len(surface.SafetyRefs),
			AssignedCount:    len(surface.SceneRefs) + len(surface.LayerRefs) + len(surface.MasterRefs),
			MidiMappingCount: len(surface.MidiMappings),
		})
	}
	return summaries, nil
}

// AssignItem assigns one individual control (D-01/D-03) to surfaceName via
// "operatorsurface assign". Assigning an already-assigned control is an
// idempotent no-op (PLAY-03), reflected by operatorsurface's own
// Assign*/Unassign* mutators.
func (s *SurfaceService) AssignItem(surfaceName string, controlRef ControlRefInput) Result {
	flag, value, err := controlRef.cliSelector()
	if err != nil {
		return Result{ExitCode: 2, Stderr: err.Error()}
	}
	return s.execute([]string{"operatorsurface", "assign", "--surface", surfaceName, flag, value, "--show", s.showPath})
}

// UnassignItem unassigns one individual control from surfaceName via
// "operatorsurface unassign". Unassigning a control not present is a no-op.
func (s *SurfaceService) UnassignItem(surfaceName string, controlRef ControlRefInput) Result {
	flag, value, err := controlRef.cliSelector()
	if err != nil {
		return Result{ExitCode: 2, Stderr: err.Error()}
	}
	return s.execute([]string{"operatorsurface", "unassign", "--surface", surfaceName, flag, value, "--show", s.showPath})
}

// ShowSurface returns surfaceName's full detail: every assignable control in
// the show (scenes, their four fixed layers, the grand master, every group
// master, and the three fixed safety controls), each marked Assigned or not
// against this surface (D-04: OperatorSurface.tsx's visible-but-locked
// renderer needs the complete set, never only the assigned subset).
func (s *SurfaceService) ShowSurface(surfaceName string) (SurfaceDetail, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return SurfaceDetail{}, err
	}
	target, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return SurfaceDetail{}, fmt.Errorf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists", surfaceName)
	}

	controls := make([]ControlRefView, 0, len(state.Scenes)*(1+len(surfaceLayerKindOrder))+len(state.Groups)+1+len(surfaceSafetyOrder))
	for _, sc := range state.Scenes {
		sceneRef := operatorsurface.SceneControlRef(sc.ID)
		controls = append(controls, ControlRefView{
			ControlRefInput: ControlRefInput{Kind: "scene", Scene: sc.Name},
			Label:           sc.Name,
			Assigned:        target.IsAssigned(sceneRef),
		})
		for _, kind := range surfaceLayerKindOrder {
			layerRef := operatorsurface.LayerControlRef(operatorsurface.LayerRef{SceneID: sc.ID, Kind: kind})
			controls = append(controls, ControlRefView{
				ControlRefInput: ControlRefInput{Kind: "layer", Scene: sc.Name, LayerKind: string(kind)},
				Label:           fmt.Sprintf("%s / %s", sc.Name, layerKindLabel(kind)),
				Assigned:        target.IsAssigned(layerRef),
			})
		}
	}

	grandRef := operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GrandMaster})
	controls = append(controls, ControlRefView{
		ControlRefInput: ControlRefInput{Kind: "master", MasterKind: "grand"},
		Label:           "Grand Master",
		Assigned:        target.IsAssigned(grandRef),
	})
	for _, g := range state.Groups {
		groupRef := operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GroupMaster, GroupID: g.ID})
		controls = append(controls, ControlRefView{
			ControlRefInput: ControlRefInput{Kind: "master", MasterKind: "group", Group: g.Name},
			Label:           fmt.Sprintf("Group Master: %s", g.Name),
			Assigned:        target.IsAssigned(groupRef),
		})
	}

	for _, sc := range surfaceSafetyOrder {
		safetyRef := operatorsurface.SafetyControlRef(sc)
		controls = append(controls, ControlRefView{
			ControlRefInput: ControlRefInput{Kind: "safety", Safety: string(sc)},
			Label:           safetyLabel(sc),
			Assigned:        target.IsAssigned(safetyRef),
		})
	}

	return SurfaceDetail{
		ID:               target.ID.String(),
		Name:             target.Name,
		Controls:         controls,
		MidiMappingCount: len(target.MidiMappings),
	}, nil
}

// RemoveSurface deletes surfaceName and all its assignments/MIDI mappings
// (destructive; the frontend's own "Remove Operator Surface" confirm copy
// gates the call, T-06-20) via "operatorsurface remove".
func (s *SurfaceService) RemoveSurface(surfaceName string) Result {
	return s.execute([]string{"operatorsurface", "remove", "--surface", surfaceName, "--show", s.showPath})
}

// AuthorizeControl is the Wails-bound server-side visible-but-locked
// enforcement point (D-04/ASVS V4, threat T-06-18): loads the ShowState,
// resolves surfaceName and controlRef against it, and calls
// command.Authorize(surface, ref) before any operator-mode action against
// controlRef may proceed. Every operator-mode dispatch path (06-05
// SafetyService, 06-06 PlaybackService) is expected to call this same
// check -- OperatorSurface.tsx's own visible-but-locked rendering is never
// trusted as the sole enforcement; a crafted/replayed call against an
// unassigned control is rejected here, in Go, exactly like the CLI's own
// command.Authorize.
func (s *SurfaceService) AuthorizeControl(surfaceName string, controlRef ControlRefInput) Result {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	surface, found := surfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists", surfaceName)}
	}
	ref, err := resolveSurfaceControlRef(state, controlRef)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	if err := command.Authorize(surface, ref); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	return Result{ExitCode: 0, Stdout: "GOLC_OPERATORSURFACE_AUTHORIZED"}
}

// cliSelector returns the exact "--scene"/"--layer"/"--master"/"--safety"
// flag and value pair internal/command/operatorsurface.go's assign/
// unassign routes accept for in, so AssignItem/UnassignItem execute the
// identical selector grammar a CLI invocation would build.
func (in ControlRefInput) cliSelector() (flag, value string, err error) {
	switch in.Kind {
	case "scene":
		if in.Scene == "" {
			return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: kind \"scene\" requires scene")
		}
		return "--scene", in.Scene, nil
	case "layer":
		if in.Scene == "" || in.LayerKind == "" {
			return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: kind \"layer\" requires scene and layerKind")
		}
		return "--layer", in.Scene + ":" + in.LayerKind, nil
	case "master":
		switch in.MasterKind {
		case "grand":
			return "--master", "grand", nil
		case "group":
			if in.Group == "" {
				return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: kind \"master\" with masterKind \"group\" requires group")
			}
			return "--master", "group:" + in.Group, nil
		default:
			return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: masterKind must be \"grand\" or \"group\", got %q", in.MasterKind)
		}
	case "safety":
		if in.Safety == "" {
			return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: kind \"safety\" requires safety")
		}
		return "--safety", in.Safety, nil
	default:
		return "", "", fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: unsupported kind %q", in.Kind)
	}
}

// resolveSurfaceControlRef resolves in's raw names against state's own
// scenes/groups into an operatorsurface.ControlRef, mirroring
// internal/command/operatorsurface.go's resolveControlRef exactly (a
// distinct copy is required here since that function is unexported to the
// command package and this file's input is JSON, not CLI args).
func resolveSurfaceControlRef(state show.State, in ControlRefInput) (operatorsurface.ControlRef, error) {
	switch in.Kind {
	case "scene":
		sc, found := sceneByNameInState(state.Scenes, in.Scene)
		if !found {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SCENE_NOT_FOUND: no scene named %q exists", in.Scene)
		}
		return operatorsurface.SceneControlRef(sc.ID), nil
	case "layer":
		sc, found := sceneByNameInState(state.Scenes, in.Scene)
		if !found {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SCENE_NOT_FOUND: no scene named %q exists", in.Scene)
		}
		kind := scene.LayerKind(in.LayerKind)
		if _, ok := sc.LayerByKind(kind); !ok {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_LAYER_NOT_FOUND: scene %q has no layer of kind %q", in.Scene, in.LayerKind)
		}
		return operatorsurface.LayerControlRef(operatorsurface.LayerRef{SceneID: sc.ID, Kind: kind}), nil
	case "master":
		if in.MasterKind == "grand" {
			return operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GrandMaster}), nil
		}
		for _, g := range state.Groups {
			if g.Name == in.Group {
				return operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GroupMaster, GroupID: g.ID}), nil
			}
		}
		return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_GROUP_NOT_FOUND: no group named %q exists", in.Group)
	case "safety":
		sc := operatorsurface.SafetyControl(in.Safety)
		switch sc {
		case operatorsurface.Blackout, operatorsurface.StopReleaseAll, operatorsurface.RevokeAutomation:
			return operatorsurface.SafetyControlRef(sc), nil
		default:
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SAFETY_INVALID: %q is not a supported safety control", in.Safety)
		}
	default:
		return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_WAILS_CONTROL_REF_INVALID: unsupported kind %q", in.Kind)
	}
}

// sceneByNameInState returns the scene in scenes whose Name matches name.
func sceneByNameInState(scenes []scene.Scene, name string) (scene.Scene, bool) {
	for _, sc := range scenes {
		if sc.Name == name {
			return sc, true
		}
	}
	return scene.Scene{}, false
}

// surfaceByName returns the surface in surfaces whose Name matches name.
func surfaceByName(surfaces []operatorsurface.Surface, name string) (operatorsurface.Surface, bool) {
	for _, surface := range surfaces {
		if surface.Name == name {
			return surface, true
		}
	}
	return operatorsurface.Surface{}, false
}

// layerKindLabel returns a human-readable label for kind.
func layerKindLabel(kind scene.LayerKind) string {
	switch kind {
	case scene.BaseLook:
		return "Base Look"
	case scene.ColorTheme:
		return "Color Theme"
	case scene.Chase:
		return "Chase"
	case scene.Motion:
		return "Motion"
	default:
		return string(kind)
	}
}

// safetyLabel returns a human-readable label for sc.
func safetyLabel(sc operatorsurface.SafetyControl) string {
	switch sc {
	case operatorsurface.Blackout:
		return "Blackout"
	case operatorsurface.StopReleaseAll:
		return "Stop / Release All"
	case operatorsurface.RevokeAutomation:
		return "Revoke Automation"
	default:
		return string(sc)
	}
}
