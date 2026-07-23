// operatorsurface.go is the operatorsurface command file: it owns the
// "operatorsurface" routing scope and self-registers create/list/assign/
// unassign/show/remove (CONTEXT PLAY-03, D-01/D-02/D-03; "remove" added by
// 06-07-PLAN.md Task 1 for the destructive Remove-Operator-Surface UI
// action, T-06-20), giving a show author a
// CLI-testable surface to build and inspect a named, individually-assigned
// constrained operator surface entirely from the command line, with no
// Wails frontend dependency (06-01-PLAN.md Objective). Handlers follow
// internal/command/playback.go's parse-args-then-Load-mutate-Save-Stdout
// shape.
//
// This file also exports Authorize (CONTEXT D-04/ASVS V4): the
// server-side visible-but-locked enforcement point every control action
// dispatched under an active operator surface must call before mutating
// -- a control not currently in the surface's assignment set is rejected
// here, in Go, never trusted from a frontend-only disabled/hidden
// control. The Wails host (06-05/06-07) and any command dispatched under
// an active surface are expected to call this directly rather than
// re-implementing the membership check.
package command

import (
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "operatorsurface",
	Summary: "Named, individually-assigned constrained operator playback surfaces (PLAY-03) with per-surface MIDI mappings (D-07).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "operatorsurface create",
	Summary: "Create a named operator surface against a ShowState document: operatorsurface create <name> --show <path>.",
	Handler: runOperatorSurfaceCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "operatorsurface list",
	Summary: "List every operator surface on a ShowState document: operatorsurface list --show <path>.",
	Handler: runOperatorSurfaceList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "operatorsurface assign",
	Summary: "Assign one individual control to a named operator surface: operatorsurface assign --surface <name> " +
		"[--scene <scene>|--layer <scene>:<kind>|--master grand|--master group:<group>|--safety <blackout|stop_release_all|revoke_automation>] --show <path>.",
	Handler: runOperatorSurfaceAssign,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "operatorsurface unassign",
	Summary: "Unassign one individual control from a named operator surface (same selectors as \"operatorsurface assign\"): " +
		"operatorsurface unassign --surface <name> " +
		"[--scene <scene>|--layer <scene>:<kind>|--master grand|--master group:<group>|--safety <blackout|stop_release_all|revoke_automation>] --show <path>.",
	Handler: runOperatorSurfaceUnassign,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "operatorsurface show",
	Summary: "Print a named operator surface's assigned items and MIDI mappings: operatorsurface show --surface <name> --show <path>.",
	Handler: runOperatorSurfaceShow,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "operatorsurface remove",
	Summary: "Delete a named operator surface and all its assignments and MIDI mappings: operatorsurface remove --surface <name> --show <path>.",
	Handler: runOperatorSurfaceRemove,
})

// Authorize is the server-side visible-but-locked enforcement point
// (CONTEXT D-04/ASVS V4): control is rejected with GOLC_OPERATORSURFACE_LOCKED
// unless it is currently a member of surface's own assignment set
// (operatorsurface.Surface.IsAssigned) -- every command dispatched under
// an active operator surface must call this before mutating, never
// trusting a frontend-disabled control as sufficient enforcement on its
// own.
func Authorize(surface operatorsurface.Surface, control operatorsurface.ControlRef) error {
	if !surface.IsAssigned(control) {
		return fmt.Errorf("GOLC_OPERATORSURFACE_LOCKED: control is not assigned to surface %q", surface.Name)
	}
	return nil
}

// operatorSurfaceByName returns the surface in surfaces whose Name
// matches name.
func operatorSurfaceByName(surfaces []operatorsurface.Surface, name string) (operatorsurface.Surface, bool) {
	for _, s := range surfaces {
		if s.Name == name {
			return s, true
		}
	}
	return operatorsurface.Surface{}, false
}

// replaceOperatorSurface returns a copy of surfaces with the entry whose
// ID matches updated.ID replaced by updated; every other entry is left
// untouched. Never aliases the caller's own slice.
func replaceOperatorSurface(surfaces []operatorsurface.Surface, updated operatorsurface.Surface) []operatorsurface.Surface {
	out := make([]operatorsurface.Surface, len(surfaces))
	for i, s := range surfaces {
		if s.ID == updated.ID {
			out[i] = updated
			continue
		}
		out[i] = s
	}
	return out
}

// sceneByName is declared in internal/command/scene.go (shared across
// this package's command files) and returns (scene, index, found);
// reused here rather than redeclared.

// groupByName returns the group in groups whose Name matches name.
func groupByName(groups []pool.Group, name string) (pool.Group, bool) {
	for _, g := range groups {
		if g.Name == name {
			return g, true
		}
	}
	return pool.Group{}, false
}

// operatorSurfaceSelector is the parsed shape of one --scene/--layer/
// --master/--safety selector, still carrying raw names (not yet resolved
// against a loaded ShowState).
type operatorSurfaceSelector struct {
	kind       string // "scene" | "layer" | "master" | "safety"
	sceneName  string // for "scene" and "layer"
	layerKind  string // for "layer"
	masterKind string // "grand" | "group", for "master"
	groupName  string // for "master" with masterKind=="group"
	safetyName string // for "safety"
}

// splitLayerSelector parses a "--layer" value in the exact
// "<scene>:<kind>" shape.
func splitLayerSelector(usage, raw string) (sceneName, kind string, err error) {
	sceneName, kind, found := strings.Cut(raw, ":")
	if !found || sceneName == "" || kind == "" {
		return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --layer value %q must be \"<scene>:<kind>\"; usage: %s", raw, usage)
	}
	return sceneName, kind, nil
}

// splitMasterSelector parses a "--master" value: exactly "grand", or
// "group:<name>" with a non-empty group name.
func splitMasterSelector(usage, raw string) (masterKind, groupName string, err error) {
	if raw == "grand" {
		return "grand", "", nil
	}
	if name, ok := strings.CutPrefix(raw, "group:"); ok && name != "" {
		return "group", name, nil
	}
	return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --master value %q must be \"grand\" or \"group:<name>\"; usage: %s", raw, usage)
}

// parseOperatorSurfaceSelectorArgs accepts a required "--surface <name>",
// exactly one of "--scene <scene>" / "--layer <scene>:<kind>" /
// "--master grand|group:<group>" / "--safety <control>", and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_OPERATORSURFACE_USAGE). Shared by "operatorsurface
// assign" and "operatorsurface unassign", which take the identical
// selector shape.
func parseOperatorSurfaceSelectorArgs(usage string, args []string) (surfaceName string, selector operatorSurfaceSelector, showPath string, err error) {
	usageErr := func(format string, a ...any) (string, operatorSurfaceSelector, string, error) {
		return "", operatorSurfaceSelector{}, "", fmt.Errorf(format, a...)
	}
	selectorSeen := false

	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--surface":
			if i+1 >= len(args) {
				return usageErr("GOLC_OPERATORSURFACE_USAGE: --surface requires a value; usage: %s", usage)
			}
			surfaceName = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--surface="):
			surfaceName = strings.TrimPrefix(argument, "--surface=")
			i++
		case argument == "--scene", argument == "--layer", argument == "--master", argument == "--safety":
			if selectorSeen {
				return usageErr("GOLC_OPERATORSURFACE_USAGE: only one of --scene/--layer/--master/--safety may be given; usage: %s", usage)
			}
			if i+1 >= len(args) {
				return usageErr("GOLC_OPERATORSURFACE_USAGE: %s requires a value; usage: %s", argument, usage)
			}
			parsed, parseErr := parseSelectorValue(usage, argument, args[i+1])
			if parseErr != nil {
				return "", operatorSurfaceSelector{}, "", parseErr
			}
			selector = parsed
			selectorSeen = true
			i += 2
		case strings.HasPrefix(argument, "--scene="), strings.HasPrefix(argument, "--layer="),
			strings.HasPrefix(argument, "--master="), strings.HasPrefix(argument, "--safety="):
			if selectorSeen {
				return usageErr("GOLC_OPERATORSURFACE_USAGE: only one of --scene/--layer/--master/--safety may be given; usage: %s", usage)
			}
			flag, value, _ := strings.Cut(argument, "=")
			parsed, parseErr := parseSelectorValue(usage, flag, value)
			if parseErr != nil {
				return "", operatorSurfaceSelector{}, "", parseErr
			}
			selector = parsed
			selectorSeen = true
			i++
		case argument == "--show":
			if i+1 >= len(args) {
				return usageErr("GOLC_OPERATORSURFACE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return usageErr("GOLC_OPERATORSURFACE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if surfaceName == "" {
		return usageErr("GOLC_OPERATORSURFACE_USAGE: --surface is required; usage: %s", usage)
	}
	if !selectorSeen {
		return usageErr("GOLC_OPERATORSURFACE_USAGE: exactly one of --scene/--layer/--master/--safety is required; usage: %s", usage)
	}
	if showPath == "" {
		return usageErr("GOLC_OPERATORSURFACE_USAGE: --show is required; usage: %s", usage)
	}
	return surfaceName, selector, showPath, nil
}

// parseSelectorValue builds an operatorSurfaceSelector for flag ("--scene",
// "--layer", "--master", or "--safety") given its raw value.
func parseSelectorValue(usage, flag, value string) (operatorSurfaceSelector, error) {
	switch flag {
	case "--scene":
		return operatorSurfaceSelector{kind: "scene", sceneName: value}, nil
	case "--layer":
		sceneName, kind, err := splitLayerSelector(usage, value)
		if err != nil {
			return operatorSurfaceSelector{}, err
		}
		return operatorSurfaceSelector{kind: "layer", sceneName: sceneName, layerKind: kind}, nil
	case "--master":
		masterKind, groupName, err := splitMasterSelector(usage, value)
		if err != nil {
			return operatorSurfaceSelector{}, err
		}
		return operatorSurfaceSelector{kind: "master", masterKind: masterKind, groupName: groupName}, nil
	case "--safety":
		return operatorSurfaceSelector{kind: "safety", safetyName: value}, nil
	default:
		return operatorSurfaceSelector{}, fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: unsupported selector flag %q; usage: %s", flag, usage)
	}
}

// resolveControlRef resolves selector's raw names against state's own
// scenes/groups into an operatorsurface.ControlRef, failing with a
// GOLC_OPERATORSURFACE_{SCENE,LAYER,GROUP,SAFETY}_* diagnostic if the
// referenced item does not exist.
func resolveControlRef(state show.State, selector operatorSurfaceSelector) (operatorsurface.ControlRef, error) {
	switch selector.kind {
	case "scene":
		sc, _, found := sceneByName(state.Scenes, selector.sceneName)
		if !found {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SCENE_NOT_FOUND: no scene named %q exists", selector.sceneName)
		}
		return operatorsurface.SceneControlRef(sc.ID), nil
	case "layer":
		sc, _, found := sceneByName(state.Scenes, selector.sceneName)
		if !found {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SCENE_NOT_FOUND: no scene named %q exists", selector.sceneName)
		}
		kind := scene.LayerKind(selector.layerKind)
		if _, ok := sc.LayerByKind(kind); !ok {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_LAYER_NOT_FOUND: scene %q has no layer of kind %q", selector.sceneName, selector.layerKind)
		}
		return operatorsurface.LayerControlRef(operatorsurface.LayerRef{SceneID: sc.ID, Kind: kind}), nil
	case "master":
		if selector.masterKind == "grand" {
			return operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GrandMaster}), nil
		}
		g, found := groupByName(state.Groups, selector.groupName)
		if !found {
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_GROUP_NOT_FOUND: no group named %q exists", selector.groupName)
		}
		return operatorsurface.MasterControlRef(operatorsurface.MasterRef{Kind: operatorsurface.GroupMaster, GroupID: g.ID}), nil
	case "safety":
		sc := operatorsurface.SafetyControl(selector.safetyName)
		switch sc {
		case operatorsurface.Blackout, operatorsurface.StopReleaseAll, operatorsurface.RevokeAutomation:
			return operatorsurface.SafetyControlRef(sc), nil
		default:
			return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_SAFETY_INVALID: %q is not a supported safety control", selector.safetyName)
		}
	default:
		return operatorsurface.ControlRef{}, fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: no selector given")
	}
}

// applyControlRef dispatches ref to the matching Assign*/Unassign*
// mutator on s, returning a fresh copy (operatorsurface's own
// copy-returning discipline).
func applyControlRef(s operatorsurface.Surface, ref operatorsurface.ControlRef, assign bool) operatorsurface.Surface {
	switch ref.Kind {
	case operatorsurface.ControlScene:
		if assign {
			return operatorsurface.AssignScene(s, ref.Scene)
		}
		return operatorsurface.UnassignScene(s, ref.Scene)
	case operatorsurface.ControlLayer:
		if assign {
			return operatorsurface.AssignLayer(s, ref.Layer)
		}
		return operatorsurface.UnassignLayer(s, ref.Layer)
	case operatorsurface.ControlMaster:
		if assign {
			return operatorsurface.AssignMaster(s, ref.Master)
		}
		return operatorsurface.UnassignMaster(s, ref.Master)
	case operatorsurface.ControlSafety:
		if assign {
			return operatorsurface.AssignSafety(s, ref.Safety)
		}
		return operatorsurface.UnassignSafety(s, ref.Safety)
	default:
		return s
	}
}

// parseOperatorSurfaceCreateArgs accepts exactly: a positional surface
// name and a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_OPERATORSURFACE_USAGE).
func parseOperatorSurfaceCreateArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// parseOperatorSurfaceShowOnlyArgs accepts exactly a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else. Used by "operatorsurface list".
func parseOperatorSurfaceShowOnlyArgs(usage string, args []string) (showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --show is required; usage: %s", usage)
	}
	return showPath, nil
}

// parseOperatorSurfaceNameShowArgs accepts exactly a required
// "--surface <name>" and a required "--show <path>" (both --flag value
// and --flag=value forms), rejecting anything else. Used by
// "operatorsurface show".
func parseOperatorSurfaceNameShowArgs(usage string, args []string) (surfaceName, showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--surface":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --surface requires a value; usage: %s", usage)
			}
			surfaceName = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--surface="):
			surfaceName = strings.TrimPrefix(argument, "--surface=")
			i++
		case argument == "--show":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if surfaceName == "" || showPath == "" {
		return "", "", fmt.Errorf("GOLC_OPERATORSURFACE_USAGE: --surface and --show are required; usage: %s", usage)
	}
	return surfaceName, showPath, nil
}

// runOperatorSurfaceCreate serves the self-registered "operatorsurface
// create" route: load the ShowState at --show, append the new surface,
// and save atomically. A duplicate surface name is rejected by
// show.Save's whole-State validation (surfaced as
// GOLC_OPERATORSURFACE_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic).
func runOperatorSurfaceCreate(request Request) Result {
	usage := "operatorsurface create <name> --show <path>"
	name, showPath, err := parseOperatorSurfaceCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newSurface, err := operatorsurface.NewSurface(name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.OperatorSurfaces = append(state.OperatorSurfaces, newSurface)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_OPERATORSURFACE_CREATED: %s (%s)\n", newSurface.Name, newSurface.ID))}
}

// runOperatorSurfaceList serves the self-registered "operatorsurface
// list" route: load the ShowState at --show (read-only) and print every
// surface's name and ID.
func runOperatorSurfaceList(request Request) Result {
	usage := "operatorsurface list --show <path>"
	showPath, err := parseOperatorSurfaceShowOnlyArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "GOLC_OPERATORSURFACE_LIST: %d surface(s)\n", len(state.OperatorSurfaces))
	for _, s := range state.OperatorSurfaces {
		fmt.Fprintf(&b, "  %s (%s)\n", s.Name, s.ID)
	}
	return Result{Stdout: []byte(b.String())}
}

// runOperatorSurfaceAssign serves the self-registered "operatorsurface
// assign" route.
func runOperatorSurfaceAssign(request Request) Result {
	usage := "operatorsurface assign --surface <name> " +
		"[--scene <scene>|--layer <scene>:<kind>|--master grand|--master group:<group>|--safety <blackout|stop_release_all|revoke_automation>] --show <path>"
	return runOperatorSurfaceAssignment(request, true, usage)
}

// runOperatorSurfaceUnassign serves the self-registered "operatorsurface
// unassign" route (same selector shape as "operatorsurface assign").
func runOperatorSurfaceUnassign(request Request) Result {
	usage := "operatorsurface unassign --surface <name> " +
		"[--scene <scene>|--layer <scene>:<kind>|--master grand|--master group:<group>|--safety <blackout|stop_release_all|revoke_automation>] --show <path>"
	return runOperatorSurfaceAssignment(request, false, usage)
}

// runOperatorSurfaceAssignment is the shared assign/unassign handler:
// parse the selector, load the ShowState, resolve the named surface,
// resolve the selector into an operatorsurface.ControlRef against the
// loaded state's own scenes/groups, apply the matching idempotent
// Assign*/Unassign* mutator, and save atomically.
func runOperatorSurfaceAssignment(request Request, assign bool, usage string) Result {
	surfaceName, selector, showPath, err := parseOperatorSurfaceSelectorArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, found := operatorSurfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName))}
	}

	ref, err := resolveControlRef(state, selector)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	updated := applyControlRef(target, ref, assign)
	state.OperatorSurfaces = replaceOperatorSurface(state.OperatorSurfaces, updated)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	verb := "GOLC_OPERATORSURFACE_ASSIGNED"
	if !assign {
		verb = "GOLC_OPERATORSURFACE_UNASSIGNED"
	}
	return Result{Stdout: []byte(fmt.Sprintf("%s: %s\n", verb, surfaceName))}
}

// runOperatorSurfaceShow serves the self-registered "operatorsurface
// show" route: load the ShowState at --show (read-only), resolve the
// named surface, and print its assigned items and MIDI mappings.
func runOperatorSurfaceShow(request Request) Result {
	usage := "operatorsurface show --surface <name> --show <path>"
	surfaceName, showPath, err := parseOperatorSurfaceNameShowArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, found := operatorSurfaceByName(state.OperatorSurfaces, surfaceName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName))}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "GOLC_OPERATORSURFACE_SHOW: %s (%s)\n", target.Name, target.ID)
	fmt.Fprintf(&b, "  scenes: %d\n", len(target.SceneRefs))
	for _, id := range target.SceneRefs {
		fmt.Fprintf(&b, "    - %s\n", id)
	}
	fmt.Fprintf(&b, "  layers: %d\n", len(target.LayerRefs))
	for _, ref := range target.LayerRefs {
		fmt.Fprintf(&b, "    - %s:%s\n", ref.SceneID, ref.Kind)
	}
	fmt.Fprintf(&b, "  masters: %d\n", len(target.MasterRefs))
	for _, ref := range target.MasterRefs {
		if ref.Kind == operatorsurface.GrandMaster {
			b.WriteString("    - grand\n")
			continue
		}
		fmt.Fprintf(&b, "    - group:%s\n", ref.GroupID)
	}
	fmt.Fprintf(&b, "  safety: %d\n", len(target.SafetyRefs))
	for _, sc := range target.SafetyRefs {
		fmt.Fprintf(&b, "    - %s\n", sc)
	}
	fmt.Fprintf(&b, "  midi_mappings: %d\n", len(target.MidiMappings))
	for _, m := range target.MidiMappings {
		fmt.Fprintf(&b, "    - channel=%d kind=%s number=%d\n", m.Channel, m.Kind, m.Number)
	}
	return Result{Stdout: []byte(b.String())}
}

// runOperatorSurfaceRemove serves the self-registered "operatorsurface
// remove" route (06-07-PLAN.md Task 1: RemoveSurface, T-06-20): load the
// ShowState at --show, delete the named surface (and, with it, every
// assignment and MIDI mapping it owned), and save atomically. Removing an
// unknown surface is rejected -- never a silent no-op.
func runOperatorSurfaceRemove(request Request) Result {
	usage := "operatorsurface remove --surface <name> --show <path>"
	surfaceName, showPath, err := parseOperatorSurfaceNameShowArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if _, found := operatorSurfaceByName(state.OperatorSurfaces, surfaceName); !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_OPERATORSURFACE_NOT_FOUND: no operator surface named %q exists\n", surfaceName))}
	}

	filtered := make([]operatorsurface.Surface, 0, len(state.OperatorSurfaces))
	for _, s := range state.OperatorSurfaces {
		if s.Name != surfaceName {
			filtered = append(filtered, s)
		}
	}
	state.OperatorSurfaces = filtered

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_OPERATORSURFACE_REMOVED: %s\n", surfaceName))}
}
