// scene.go is the scene/blend command file: it owns the "scene" and
// "blend" routing scopes and self-registers "scene create" / "scene
// activate" / "scene layer set" (CONTEXT SCEN-01/SCEN-04/SCEN-05) and
// "blend create" (CONTEXT SCEN-07): a show author creates bar-loop scenes,
// activates exactly one at a time, points/enables one of a scene's four
// fixed layers (base-look/color-theme/chase/motion) against a resolvable
// object reference plus an independently scoped fixture selection, and
// creates reusable blend presets describing transitions between scene/
// layer states. Handlers follow internal/command/pool.go's parse-args-
// then-Load-mutate-Save-Stdout shape; "scene layer set"'s selection flags
// reuse internal/command/programming.go's --instance/--pool/--group/
// --fixture parsing helpers (parseUUIDFlag/parseFixtureRef), unexported but
// reachable from this file since both live in package command.
package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "scene",
	Summary: "Bar-loop scenes combining independently enabled base-look/color-theme/chase/motion layers by fixed priority (SCEN-01/SCEN-04/SCEN-05).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "scene create",
	Summary: "Create a named bar-loop scene against a ShowState document: scene create <name> --bars <n> --show <path>.",
	Handler: runSceneCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "scene activate",
	Summary: "Mark exactly one scene active, deactivating every other scene: scene activate <name> --show <path>.",
	Handler: runSceneActivate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "scene layer set",
	Summary: "Enable/point one of a scene's four fixed layers: scene layer set <scene> --kind base_look|color_theme|chase|motion " +
		"[--ref <id>] [--instance <id>]... [--pool <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]... [--disable] --show <path>.",
	Handler: runSceneLayerSet,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "blend",
	Summary: "Reusable blend presets describing transitions between scene/layer states (SCEN-07).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "blend create",
	Summary: "Create a named blend preset against a ShowState document: blend create <name> --duration-bars <value> " +
		"[--curve linear|ease_in|ease_out] --show <path>.",
	Handler: runBlendCreate,
})

// sceneByName returns the scene in scenes whose Name matches name, plus its
// index (so the caller can splice a mutated copy back into place).
func sceneByName(scenes []scene.Scene, name string) (scene.Scene, int, bool) {
	for i, s := range scenes {
		if s.Name == name {
			return s, i, true
		}
	}
	return scene.Scene{}, -1, false
}

// parseSceneCreateArgs accepts a positional scene name followed by a
// required "--bars <n>" and a required "--show <path>" (both --flag value
// and --flag=value forms), rejecting anything else (GOLC_SCENE_USAGE).
// --bars' own boundary (>=1, <= the declared ceiling) is checked later by
// scene.NewScene, never re-derived here -- this parser only requires the
// flag to be present and parse as an integer.
func parseSceneCreateArgs(usage string, args []string) (name string, bars int, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: usage: %s", usage)
	}
	name = args[0]

	var rawBars string
	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--bars":
			if i+1 >= len(rest) {
				return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: --bars requires a value; usage: %s", usage)
			}
			rawBars = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--bars="):
			rawBars = strings.TrimPrefix(argument, "--bars=")
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if rawBars == "" {
		return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: --bars is required; usage: %s", usage)
	}
	if showPath == "" {
		return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: --show is required; usage: %s", usage)
	}
	parsedBars, parseErr := strconv.Atoi(rawBars)
	if parseErr != nil {
		return "", 0, "", fmt.Errorf("GOLC_SCENE_USAGE: --bars value %q is not a valid integer; usage: %s", rawBars, usage)
	}
	return name, parsedBars, showPath, nil
}

// runSceneCreate serves the self-registered "scene create" route
// (SCEN-01): load the ShowState at --show, append the new inactive scene
// (all four layers present but disabled), and save atomically. A duplicate
// scene name is rejected by show.Save's whole-State validation (surfaced
// as GOLC_SCENE_DUPLICATE_NAME inside the wrapping GOLC_SHOW_STATE_INVALID
// diagnostic) -- never a silent duplicate.
func runSceneCreate(request Request) Result {
	usage := "scene create <name> --bars <n> --show <path>"
	name, bars, showPath, err := parseSceneCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newScene, err := scene.NewScene(name, bars)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Scenes = append(state.Scenes, newScene)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SCENE_CREATED: %s (%s)\n", newScene.Name, newScene.ID))}
}

// parseSceneNameShowArgs accepts exactly a positional scene name and a
// required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_SCENE_USAGE) -- mirrors
// parseDeploymentNameShowArgs's identical <name> --show <path> shape.
func parseSceneNameShowArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_SCENE_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_SCENE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_SCENE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_SCENE_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// runSceneActivate serves the self-registered "scene activate" route
// (SCEN-04): load the ShowState, mark exactly the named scene active
// (scene.ActivateScene guarantees every other scene becomes inactive in
// the same call, so two scenes are never simultaneously active), and save
// atomically.
func runSceneActivate(request Request) Result {
	usage := "scene activate <name> --show <path>"
	name, showPath, err := parseSceneNameShowArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	activated, err := scene.ActivateScene(state.Scenes, name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Scenes = activated

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SCENE_ACTIVATED: %s\n", name))}
}

// sceneLayerSetArgs is the parsed shape of one "scene layer set"
// invocation. The has* flags (WR-03) record whether the caller supplied at
// least one flag of that selector kind on THIS invocation -- distinct from
// the corresponding slice being non-empty -- so runSceneLayerSet can tell
// "the caller didn't mention pools this time" (merge in the existing
// layer's PoolIDs) apart from "the caller explicitly supplied zero pools"
// (which cannot be expressed by omission alone, since --pool is a
// repeatable flag with no "clear" form; clearing one selector kind while
// leaving others untouched still requires re-supplying every OTHER
// selector kind that should be kept).
type sceneLayerSetArgs struct {
	sceneName    string
	kind         string
	ref          uuid.UUID
	instances    []uuid.UUID
	hasInstances bool
	pools        []uuid.UUID
	hasPools     bool
	groups       []uuid.UUID
	hasGroups    bool
	fixtureRefs  []programming.FixtureRef
	hasFixtures  bool
	disable      bool
	showPath     string
}

// parseSceneLayerSetArgs accepts a positional scene name followed by a
// required "--kind <kind>", an optional "--ref <id>", any number of
// --instance/--pool/--group/--fixture selectors, an optional "--disable"
// flag, and a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_SCENE_USAGE). --kind's
// own validity against the four declared LayerKind values is checked later
// by scene.SetLayer, never re-derived here.
func parseSceneLayerSetArgs(usage string, args []string) (sceneLayerSetArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: usage: %s", usage)
	}
	parsed := sceneLayerSetArgs{sceneName: args[0]}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--kind":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --kind requires a value; usage: %s", usage)
			}
			parsed.kind = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--kind="):
			parsed.kind = strings.TrimPrefix(argument, "--kind=")
			i++
		case argument == "--ref":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --ref requires a value; usage: %s", usage)
			}
			id, err := parseUUIDFlag("--ref", usage, rest[i+1])
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.ref = id
			i += 2
		case strings.HasPrefix(argument, "--ref="):
			id, err := parseUUIDFlag("--ref", usage, strings.TrimPrefix(argument, "--ref="))
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.ref = id
			i++
		case argument == "--instance":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --instance requires a value; usage: %s", usage)
			}
			id, err := parseUUIDFlag("--instance", usage, rest[i+1])
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.instances = append(parsed.instances, id)
			parsed.hasInstances = true
			i += 2
		case strings.HasPrefix(argument, "--instance="):
			id, err := parseUUIDFlag("--instance", usage, strings.TrimPrefix(argument, "--instance="))
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.instances = append(parsed.instances, id)
			parsed.hasInstances = true
			i++
		case argument == "--pool":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --pool requires a value; usage: %s", usage)
			}
			id, err := parseUUIDFlag("--pool", usage, rest[i+1])
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.pools = append(parsed.pools, id)
			parsed.hasPools = true
			i += 2
		case strings.HasPrefix(argument, "--pool="):
			id, err := parseUUIDFlag("--pool", usage, strings.TrimPrefix(argument, "--pool="))
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.pools = append(parsed.pools, id)
			parsed.hasPools = true
			i++
		case argument == "--group":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --group requires a value; usage: %s", usage)
			}
			id, err := parseUUIDFlag("--group", usage, rest[i+1])
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.groups = append(parsed.groups, id)
			parsed.hasGroups = true
			i += 2
		case strings.HasPrefix(argument, "--group="):
			id, err := parseUUIDFlag("--group", usage, strings.TrimPrefix(argument, "--group="))
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.groups = append(parsed.groups, id)
			parsed.hasGroups = true
			i++
		case argument == "--fixture":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --fixture requires a value; usage: %s", usage)
			}
			ref, err := parseFixtureRef(usage, rest[i+1])
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.fixtureRefs = append(parsed.fixtureRefs, ref)
			parsed.hasFixtures = true
			i += 2
		case strings.HasPrefix(argument, "--fixture="):
			ref, err := parseFixtureRef(usage, strings.TrimPrefix(argument, "--fixture="))
			if err != nil {
				return sceneLayerSetArgs{}, err
			}
			parsed.fixtureRefs = append(parsed.fixtureRefs, ref)
			parsed.hasFixtures = true
			i++
		case argument == "--disable":
			parsed.disable = true
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.kind == "" {
		return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --kind is required; usage: %s", usage)
	}
	if parsed.showPath == "" {
		return sceneLayerSetArgs{}, fmt.Errorf("GOLC_SCENE_USAGE: --show is required; usage: %s", usage)
	}
	return parsed, nil
}

// runSceneLayerSet serves the self-registered "scene layer set" route
// (SCEN-01/SCEN-05): load the ShowState at --show, resolve the target
// scene by name, replace the matching layer slot with the parsed Kind/Ref/
// Selection (Enabled defaults to true; --disable sets it false while still
// updating Ref/Selection), and save atomically. A Ref that does not
// resolve to an existing programming object of the expected type is
// rejected by show.Save's whole-State validation (surfaced as
// GOLC_SCENE_LAYER_DANGLING_REFERENCE inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic).
//
// WR-03: Selection is built per selector kind (pools/groups/instances/
// fixtures) rather than wholesale from this invocation's flags alone --
// for any selector kind the caller did not mention at all on this
// invocation, the existing layer's own value for that kind is carried
// forward unchanged, so e.g. repointing a chase's --ref (or toggling
// --disable) without re-supplying --pool no longer silently discards a
// previously configured pool/group/instance/fixture selector. A selector
// kind is only replaced when the caller supplies at least one flag of
// that kind on this invocation -- there is no dedicated "clear this
// selector kind" flag, so clearing one kind while preserving the others
// still requires explicitly re-supplying every OTHER kind that should be
// kept (matching this package's existing has*-flag convention in
// "chase update").
func runSceneLayerSet(request Request) Result {
	usage := "scene layer set <scene> --kind base_look|color_theme|chase|motion [--ref <id>] " +
		"[--instance <id>]... [--pool <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]... [--disable] --show <path>"
	parsed, err := parseSceneLayerSetArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetScene, index, found := sceneByName(state.Scenes, parsed.sceneName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCENE_NOT_FOUND: no scene named %q exists\n", parsed.sceneName))}
	}

	pools := parsed.pools
	groups := parsed.groups
	instances := parsed.instances
	fixtureRefs := parsed.fixtureRefs
	if existing, ok := targetScene.LayerByKind(scene.LayerKind(parsed.kind)); ok {
		if !parsed.hasPools {
			pools = existing.Selection.PoolIDs
		}
		if !parsed.hasGroups {
			groups = existing.Selection.GroupIDs
		}
		if !parsed.hasInstances {
			instances = existing.Selection.InstanceIDs
		}
		if !parsed.hasFixtures {
			fixtureRefs = existing.Selection.FixtureRefs
		}
	}

	layer := scene.Layer{
		Kind:    scene.LayerKind(parsed.kind),
		Enabled: !parsed.disable,
		Selection: programming.Selection{
			PoolIDs:     pools,
			GroupIDs:    groups,
			InstanceIDs: instances,
			FixtureRefs: fixtureRefs,
		},
		Ref: parsed.ref,
	}

	updatedScene, err := scene.SetLayer(targetScene, layer)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Scenes[index] = updatedScene

	if err := show.Save(request.Root, parsed.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_SCENE_LAYER_SET: scene=%s kind=%s enabled=%t\n", updatedScene.Name, layer.Kind, layer.Enabled))}
}

// parseBlendCreateArgs accepts a positional blend preset name followed by a
// required "--duration-bars <value>", an optional "--curve <curve>"
// (defaulting to scene.BlendCurveLinear when omitted), and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_BLEND_USAGE). --curve's own validity against the
// small declared set is checked later by scene.NewBlendPreset, never
// re-derived here.
func parseBlendCreateArgs(usage string, args []string) (name string, durationBars float64, curve, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: usage: %s", usage)
	}
	name = args[0]
	curve = scene.BlendCurveLinear

	var rawDuration string
	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--duration-bars":
			if i+1 >= len(rest) {
				return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --duration-bars requires a value; usage: %s", usage)
			}
			rawDuration = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--duration-bars="):
			rawDuration = strings.TrimPrefix(argument, "--duration-bars=")
			i++
		case argument == "--curve":
			if i+1 >= len(rest) {
				return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --curve requires a value; usage: %s", usage)
			}
			curve = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--curve="):
			curve = strings.TrimPrefix(argument, "--curve=")
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if rawDuration == "" {
		return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --duration-bars is required; usage: %s", usage)
	}
	if showPath == "" {
		return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --show is required; usage: %s", usage)
	}
	parsedDuration, parseErr := strconv.ParseFloat(rawDuration, 64)
	if parseErr != nil {
		return "", 0, "", "", fmt.Errorf("GOLC_BLEND_USAGE: --duration-bars value %q is not a valid number; usage: %s", rawDuration, usage)
	}
	return name, parsedDuration, curve, showPath, nil
}

// runBlendCreate serves the self-registered "blend create" route
// (SCEN-07): load the ShowState at --show, append the new blend preset,
// and save atomically. A duplicate blend preset name is rejected by
// show.Save's whole-State validation (surfaced as
// GOLC_BLEND_PRESET_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent duplicate.
func runBlendCreate(request Request) Result {
	usage := "blend create <name> --duration-bars <value> [--curve linear|ease_in|ease_out] --show <path>"
	name, durationBars, curve, showPath, err := parseBlendCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newBlend, err := scene.NewBlendPreset(name, durationBars, curve)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.BlendPresets = append(state.BlendPresets, newBlend)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_BLEND_PRESET_CREATED: %s (%s)\n", newBlend.Name, newBlend.ID))}
}
