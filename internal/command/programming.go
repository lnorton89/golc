// programming.go is the programmer command file: it owns the
// "programmer" routing scope and self-registers "programmer set" /
// "programmer inspect" / "programmer clear" (CONTEXT PROG-01/PROG-02/
// PROG-03): a show author resolves a fixture selection (pool/group/
// deployment-instance/direct-fixture, via programming.Resolve), edits
// semantic attribute values on the resolved instances, and inspects or
// clears the resulting programmer.ProgrammerState scratch buffer
// persisted on show.State. Handlers follow internal/command/pool.go's
// parse-args-then-Load-mutate-Save-Stdout shape.
//
// This file also completes PROG-07's record/update/rename/reorder/
// duplicate/delete CLI surface (03-05-PLAN.md Task 2): "theme rename",
// "theme delete", "preset rename", "preset delete", "chase update",
// "chase reorder", "chase duplicate", "chase delete", "motion rename",
// "motion duplicate", "motion delete", "scene rename", "scene duplicate",
// and "scene delete". Every one of these routes follows the same
// parse->Load->mutate->Save->Stdout shape and never inspects whether the
// target object is part of a currently-active scene before mutating
// (CONTEXT D-08: editing a live-active object is never gated at the
// object-library level -- the D-05/D-06 adoption boundary, not a workflow
// step, is what keeps live output safe). The in-memory
// programming.History from history.go is never persisted by these routes
// (D-14): a stateless CLI invocation has no cross-invocation session to
// carry an undo stack across, so History is exercised directly by
// history_test.go's library-level tests, not wired into these handlers.
package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "programmer",
	Summary: "Fixture selection, semantic attribute editing, and touched-attribute inspection on a ShowState document's Programmer buffer.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "programmer set",
	Summary: "Resolve a selection and set semantic attribute values on it: programmer set " +
		"[--instance <id>]... [--pool <id>]... [--group <id>]... [--fixture <pool_id>|<pool_member_id>]... " +
		"--attr <capability>=<value>... --show <path>.",
	Handler: runProgrammerSet,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "programmer inspect",
	Summary: "Print every touched attribute in the Programmer buffer with its value, source, and record scope: programmer inspect --show <path>.",
	Handler: runProgrammerInspect,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "programmer clear",
	Summary: "Empty the Programmer buffer's touched-attribute set: programmer clear --show <path>.",
	Handler: runProgrammerClear,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "theme",
	Summary: "Reusable named color themes captured from a show author's programming (PROG-04).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "theme create",
	Summary: "Create a named color theme against a ShowState document: theme create <name> --show <path>.",
	Handler: runThemeCreate,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "preset",
	Summary: "Reusable intensity/color/position/beam presets recorded from the Programmer buffer (PROG-04).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "preset record",
	Summary: "Record a kind-scoped preset from the persisted Programmer buffer: preset record <name> " +
		"--kind intensity|color|position|beam --show <path>.",
	Handler: runPresetRecord,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "chase",
	Summary: "Reusable chases with ordered, tempo-relative steps (PROG-05).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "chase create",
	Summary: "Create a named chase against a ShowState document: chase create <name> " +
		"--unit bar|beat --step-duration <value> --show <path>.",
	Handler: runChaseCreate,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "motion",
	Summary: "Reusable motion presets scoped strictly to position/beam semantic capabilities (PROG-06, D-04).",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "motion create",
	Summary: "Create a named motion preset against a ShowState document: motion create <name> --show <path>.",
	Handler: runMotionCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "theme rename",
	Summary: "Rename a color theme, preserving its identity: theme rename <old-name> <new-name> --show <path>.",
	Handler: runThemeRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "theme delete",
	Summary: "Delete a color theme by name: theme delete <name> --show <path>.",
	Handler: runThemeDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "preset rename",
	Summary: "Rename a preset, preserving its identity: preset rename <old-name> <new-name> --show <path>.",
	Handler: runPresetRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "preset delete",
	Summary: "Delete a preset by name: preset delete <name> --show <path>.",
	Handler: runPresetDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "chase update",
	Summary: "Update a chase's name/step-unit/step-duration: chase update <name> [--name <new-name>] " +
		"[--unit bar|beat] [--step-duration <value>] --show <path>.",
	Handler: runChaseUpdate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "chase reorder",
	Summary: "Permute a chase's steps deterministically: chase reorder <name> --order <i,j,k,...> --show <path>.",
	Handler: runChaseReorder,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "chase duplicate",
	Summary: "Duplicate a chase under a fresh identity: chase duplicate <name> <new-name> --show <path>.",
	Handler: runChaseDuplicate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "chase delete",
	Summary: "Delete a chase by name: chase delete <name> --show <path>.",
	Handler: runChaseDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "motion rename",
	Summary: "Rename a motion preset, preserving its identity: motion rename <old-name> <new-name> --show <path>.",
	Handler: runMotionRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "motion duplicate",
	Summary: "Duplicate a motion preset under a fresh identity: motion duplicate <name> <new-name> --show <path>.",
	Handler: runMotionDuplicate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "motion delete",
	Summary: "Delete a motion preset by name: motion delete <name> --show <path>.",
	Handler: runMotionDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "scene rename",
	Summary: "Rename a scene, preserving its identity: scene rename <old-name> <new-name> --show <path>.",
	Handler: runSceneRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "scene duplicate",
	Summary: "Duplicate a scene under a fresh, inactive identity: scene duplicate <name> <new-name> --show <path>.",
	Handler: runSceneDuplicate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "scene delete",
	Summary: "Delete a scene by name: scene delete <name> --show <path>.",
	Handler: runSceneDelete,
})

// attrAssignment is one parsed "--attr <capability>=<value>" pair.
type attrAssignment struct {
	Capability fixture.CapabilityType
	Value      float64
}

// programmerSetArgs is the parsed shape of one "programmer set"
// invocation.
type programmerSetArgs struct {
	instances   []uuid.UUID
	pools       []uuid.UUID
	groups      []uuid.UUID
	fixtureRefs []programming.FixtureRef
	attrs       []attrAssignment
	showPath    string
}

// parseUUIDFlag parses one repeatable "--flag <uuid>" value, returning a
// GOLC_PROGRAMMER_USAGE diagnostic for a malformed UUID.
func parseUUIDFlag(flagName, usage, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: %s value %q is not a valid UUID; usage: %s", flagName, raw, usage)
	}
	return id, nil
}

// parseFixtureRef parses one "--fixture <pool_id>|<pool_member_id>" value
// into a programming.FixtureRef, returning GOLC_SELECTION_USAGE for a
// malformed shape -- this is a Selection-input parsing error, distinct
// from this command's own general GOLC_PROGRAMMER_USAGE diagnostics.
func parseFixtureRef(usage, raw string) (programming.FixtureRef, error) {
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return programming.FixtureRef{}, fmt.Errorf("GOLC_SELECTION_USAGE: --fixture value %q must be \"<pool_id>|<pool_member_id>\"; usage: %s", raw, usage)
	}
	poolID, err := uuid.Parse(parts[0])
	if err != nil {
		return programming.FixtureRef{}, fmt.Errorf("GOLC_SELECTION_USAGE: --fixture pool_id %q is not a valid UUID; usage: %s", parts[0], usage)
	}
	poolMemberID, err := uuid.Parse(parts[1])
	if err != nil {
		return programming.FixtureRef{}, fmt.Errorf("GOLC_SELECTION_USAGE: --fixture pool_member_id %q is not a valid UUID; usage: %s", parts[1], usage)
	}
	return programming.FixtureRef{PoolID: poolID, PoolMemberID: poolMemberID}, nil
}

// parseAttrAssignment parses one "--attr <capability>=<value>" value:
// value must parse as a float64 (GOLC_PROGRAMMER_USAGE otherwise);
// capability's own validity (a supported fixture.CapabilityType) and
// value's own normalized [0,1] bound are checked later by
// ProgrammerState.SetAttribute, never re-derived here.
func parseAttrAssignment(usage, raw string) (attrAssignment, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return attrAssignment{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: --attr value %q must be \"<capability>=<value>\"; usage: %s", raw, usage)
	}
	value, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return attrAssignment{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: --attr value %q has a non-numeric value; usage: %s", raw, usage)
	}
	return attrAssignment{Capability: fixture.CapabilityType(parts[0]), Value: value}, nil
}

// parseProgrammerSetArgs accepts any number of --instance/--pool/--group/
// --fixture selectors, at least one --attr, and a required --show (both
// --flag value and --flag=value forms), rejecting anything else
// (GOLC_PROGRAMMER_USAGE).
func parseProgrammerSetArgs(usage string, args []string) (programmerSetArgs, error) {
	var parsed programmerSetArgs

	takeValue := func(i int, flagName string) (string, int, error) {
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("GOLC_PROGRAMMER_USAGE: %s requires a value; usage: %s", flagName, usage)
		}
		return args[i+1], i + 2, nil
	}

	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--instance":
			raw, next, err := takeValue(i, "--instance")
			if err != nil {
				return programmerSetArgs{}, err
			}
			id, err := parseUUIDFlag("--instance", usage, raw)
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.instances = append(parsed.instances, id)
			i = next
		case strings.HasPrefix(argument, "--instance="):
			id, err := parseUUIDFlag("--instance", usage, strings.TrimPrefix(argument, "--instance="))
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.instances = append(parsed.instances, id)
			i++
		case argument == "--pool":
			raw, next, err := takeValue(i, "--pool")
			if err != nil {
				return programmerSetArgs{}, err
			}
			id, err := parseUUIDFlag("--pool", usage, raw)
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.pools = append(parsed.pools, id)
			i = next
		case strings.HasPrefix(argument, "--pool="):
			id, err := parseUUIDFlag("--pool", usage, strings.TrimPrefix(argument, "--pool="))
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.pools = append(parsed.pools, id)
			i++
		case argument == "--group":
			raw, next, err := takeValue(i, "--group")
			if err != nil {
				return programmerSetArgs{}, err
			}
			id, err := parseUUIDFlag("--group", usage, raw)
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.groups = append(parsed.groups, id)
			i = next
		case strings.HasPrefix(argument, "--group="):
			id, err := parseUUIDFlag("--group", usage, strings.TrimPrefix(argument, "--group="))
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.groups = append(parsed.groups, id)
			i++
		case argument == "--fixture":
			raw, next, err := takeValue(i, "--fixture")
			if err != nil {
				return programmerSetArgs{}, err
			}
			ref, err := parseFixtureRef(usage, raw)
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.fixtureRefs = append(parsed.fixtureRefs, ref)
			i = next
		case strings.HasPrefix(argument, "--fixture="):
			ref, err := parseFixtureRef(usage, strings.TrimPrefix(argument, "--fixture="))
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.fixtureRefs = append(parsed.fixtureRefs, ref)
			i++
		case argument == "--attr":
			raw, next, err := takeValue(i, "--attr")
			if err != nil {
				return programmerSetArgs{}, err
			}
			attr, err := parseAttrAssignment(usage, raw)
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.attrs = append(parsed.attrs, attr)
			i = next
		case strings.HasPrefix(argument, "--attr="):
			attr, err := parseAttrAssignment(usage, strings.TrimPrefix(argument, "--attr="))
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.attrs = append(parsed.attrs, attr)
			i++
		case argument == "--show":
			raw, next, err := takeValue(i, "--show")
			if err != nil {
				return programmerSetArgs{}, err
			}
			parsed.showPath = raw
			i = next
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return programmerSetArgs{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" {
		return programmerSetArgs{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: --show is required; usage: %s", usage)
	}
	if len(parsed.attrs) == 0 {
		return programmerSetArgs{}, fmt.Errorf("GOLC_PROGRAMMER_USAGE: at least one --attr is required; usage: %s", usage)
	}
	return parsed, nil
}

// runProgrammerSet serves the self-registered "programmer set" route
// (PROG-01/PROG-02): it loads the ShowState at --show, resolves the
// requested selection via programming.Resolve (a dangling pool/group/
// instance/fixture reference fails with GOLC_SELECTION_DANGLING_
// REFERENCE, never a silently smaller set), sets every --attr on every
// resolved instance (an out-of-range value or unsupported capability
// fails with GOLC_PROGRAMMER_VALUE_OUT_OF_RANGE/GOLC_PROGRAMMER_
// CAPABILITY_UNSUPPORTED and records nothing further), and saves
// atomically.
func runProgrammerSet(request Request) Result {
	usage := "programmer set [--instance <id>]... [--pool <id>]... [--group <id>]... " +
		"[--fixture <pool_id>|<pool_member_id>]... --attr <capability>=<value>... --show <path>"
	parsed, err := parseProgrammerSetArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	sel := programming.Selection{
		PoolIDs:     parsed.pools,
		GroupIDs:    parsed.groups,
		InstanceIDs: parsed.instances,
		FixtureRefs: parsed.fixtureRefs,
	}
	resolved, err := programming.Resolve(state.Pools, state.Groups, state.Deployments, sel)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if state.Programmer == nil {
		state.Programmer = programming.NewProgrammerState()
	}
	for _, instance := range resolved.Instances {
		for _, attr := range parsed.attrs {
			if err := state.Programmer.SetAttribute(instance.InstanceID, attr.Capability, attr.Value, programming.SourceManual); err != nil {
				return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
			}
		}
	}

	if err := show.Save(request.Root, parsed.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PROGRAMMER_SET: instances=%d attributes=%d\n", len(resolved.Instances), len(parsed.attrs)))}
}

// parseProgrammerShowOnlyArgs accepts exactly a required "--show <path>"
// (both --flag value and --flag=value forms), rejecting anything else
// (GOLC_PROGRAMMER_USAGE). Shared by "programmer inspect" and "programmer
// clear", which take the identical --show-only shape.
func parseProgrammerShowOnlyArgs(usage string, args []string) (string, error) {
	var showPath string
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_PROGRAMMER_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", fmt.Errorf("GOLC_PROGRAMMER_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", fmt.Errorf("GOLC_PROGRAMMER_USAGE: --show is required; usage: %s", usage)
	}
	return showPath, nil
}

// runProgrammerInspect serves the self-registered "programmer inspect"
// route (PROG-03): it loads the ShowState at --show (read-only -- inspect
// never mutates) and prints one line per touched attribute, carrying its
// value, source, and record scope (the exact instance it will be recorded
// into). A nil Programmer buffer prints nothing -- an empty buffer is not
// an error.
func runProgrammerInspect(request Request) Result {
	showPath, err := parseProgrammerShowOnlyArgs("programmer inspect --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var touched []programming.TouchedAttribute
	if state.Programmer != nil {
		touched = state.Programmer.Touched()
	}

	var out strings.Builder
	for _, t := range touched {
		fmt.Fprintf(&out, "GOLC_PROGRAMMER_TOUCHED: instance=%s capability=%s value=%v source=%s\n",
			t.InstanceID, t.Capability, t.Value, t.Source)
	}
	return Result{Stdout: []byte(out.String())}
}

// runProgrammerClear serves the self-registered "programmer clear" route:
// it loads the ShowState at --show, empties the Programmer buffer's
// touched-attribute set (a nil buffer is left nil -- there is nothing to
// clear), and saves atomically.
func runProgrammerClear(request Request) Result {
	showPath, err := parseProgrammerShowOnlyArgs("programmer clear --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if state.Programmer == nil {
		state.Programmer = programming.NewProgrammerState()
	} else {
		state.Programmer.Clear()
	}

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte("GOLC_PROGRAMMER_CLEARED\n")}
}

// parseThemeCreateArgs accepts exactly a positional theme name followed by
// a required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_THEME_USAGE) -- mirrors
// parsePoolCreateArgs's positional-name-plus-required-show shape.
func parseThemeCreateArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_THEME_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_THEME_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_THEME_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_THEME_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// runThemeCreate serves the self-registered "theme create" route
// (PROG-04): load the ShowState at --show, append the new theme, and save
// atomically. A duplicate theme name is rejected by show.Save's
// whole-State validation (surfaced as GOLC_THEME_DUPLICATE_NAME inside the
// wrapping GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent
// duplicate.
func runThemeCreate(request Request) Result {
	usage := "theme create <name> --show <path>"
	name, showPath, err := parseThemeCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newTheme, err := programming.NewTheme(name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Themes = append(state.Themes, newTheme)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_THEME_CREATED: %s (%s)\n", newTheme.Name, newTheme.ID))}
}

// parsePresetRecordArgs accepts a positional preset name followed by a
// required "--kind <intensity|color|position|beam>" and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_PRESET_USAGE). --kind's own validity against the
// four declared PresetKind values is checked later by
// programming.RecordPresetFromProgrammer, never re-derived here.
func parsePresetRecordArgs(usage string, args []string) (name string, kind programming.PresetKind, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: usage: %s", usage)
	}
	name = args[0]

	var rawKind string
	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--kind":
			if i+1 >= len(rest) {
				return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: --kind requires a value; usage: %s", usage)
			}
			rawKind = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--kind="):
			rawKind = strings.TrimPrefix(argument, "--kind=")
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if rawKind == "" {
		return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: --kind is required; usage: %s", usage)
	}
	if showPath == "" {
		return "", "", "", fmt.Errorf("GOLC_PRESET_USAGE: --show is required; usage: %s", usage)
	}
	return name, programming.PresetKind(rawKind), showPath, nil
}

// runPresetRecord serves the self-registered "preset record" route
// (PROG-04): load the ShowState at --show, record a new kind-filtered
// preset from the persisted Programmer buffer (an absent buffer records
// from an empty, zero-value ProgrammerState -- never an error), append
// it, and save atomically. An unknown --kind is rejected with
// GOLC_PRESET_KIND_INVALID; a duplicate preset name is rejected by
// show.Save's whole-State validation (GOLC_PRESET_DUPLICATE_NAME inside
// the wrapping GOLC_SHOW_STATE_INVALID diagnostic).
func runPresetRecord(request Request) Result {
	usage := "preset record <name> --kind intensity|color|position|beam --show <path>"
	name, kind, showPath, err := parsePresetRecordArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var ps programming.ProgrammerState
	if state.Programmer != nil {
		ps = *state.Programmer
	}
	newPreset, err := programming.RecordPresetFromProgrammer(ps, kind, name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Presets = append(state.Presets, newPreset)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_PRESET_RECORDED: %s (%s) kind=%s attributes=%d\n", newPreset.Name, newPreset.ID, newPreset.Kind, len(newPreset.Attributes)))}
}

// parseChaseCreateArgs accepts a positional chase name followed by a
// required "--unit bar|beat", a required "--step-duration <value>", and a
// required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_CHASE_USAGE). --unit's own validity against
// the two declared StepUnit values and --step-duration's own positivity are
// checked later by programming.NewChase, never re-derived here -- this
// parser only requires the flags to be present and --step-duration to parse
// as a float64.
func parseChaseCreateArgs(usage string, args []string) (name string, unit programming.StepUnit, stepDuration float64, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: usage: %s", usage)
	}
	name = args[0]

	var rawUnit, rawStepDuration string
	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--unit":
			if i+1 >= len(rest) {
				return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --unit requires a value; usage: %s", usage)
			}
			rawUnit = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--unit="):
			rawUnit = strings.TrimPrefix(argument, "--unit=")
			i++
		case argument == "--step-duration":
			if i+1 >= len(rest) {
				return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --step-duration requires a value; usage: %s", usage)
			}
			rawStepDuration = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--step-duration="):
			rawStepDuration = strings.TrimPrefix(argument, "--step-duration=")
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if rawUnit == "" {
		return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --unit is required; usage: %s", usage)
	}
	if rawStepDuration == "" {
		return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --step-duration is required; usage: %s", usage)
	}
	if showPath == "" {
		return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --show is required; usage: %s", usage)
	}
	stepDuration, parseErr := strconv.ParseFloat(rawStepDuration, 64)
	if parseErr != nil {
		return "", "", 0, "", fmt.Errorf("GOLC_CHASE_USAGE: --step-duration value %q is not a valid number; usage: %s", rawStepDuration, usage)
	}
	return name, programming.StepUnit(rawUnit), stepDuration, showPath, nil
}

// runChaseCreate serves the self-registered "chase create" route
// (PROG-05): load the ShowState at --show, append the new chase (with zero
// steps -- populating Steps from the current programmer/selection state is
// a later scene-authoring concern, not this route's own record path), and
// save atomically. An invalid --unit/--step-duration is rejected by
// programming.NewChase with GOLC_CHASE_STEP_UNIT_INVALID/GOLC_CHASE_STEP_
// DURATION_INVALID; a duplicate chase name is rejected by show.Save's
// whole-State validation (GOLC_CHASE_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent duplicate.
func runChaseCreate(request Request) Result {
	usage := "chase create <name> --unit bar|beat --step-duration <value> --show <path>"
	name, unit, stepDuration, showPath, err := parseChaseCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newChase, err := programming.NewChase(name, nil, unit, stepDuration)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Chases = append(state.Chases, newChase)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_CHASE_CREATED: %s (%s)\n", newChase.Name, newChase.ID))}
}

// parseMotionCreateArgs accepts exactly a positional motion preset name
// followed by a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_MOTION_USAGE) --
// mirrors parseThemeCreateArgs's positional-name-plus-required-show shape.
func parseMotionCreateArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_MOTION_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_MOTION_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_MOTION_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_MOTION_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// runMotionCreate serves the self-registered "motion create" route
// (PROG-06): load the ShowState at --show, append the new motion preset
// (with zero keyframes -- populating Keyframes from the current
// programmer/selection state is a later scene-authoring concern, not this
// route's own record path), and save atomically. A duplicate motion
// preset name is rejected by show.Save's whole-State validation (surfaced
// as GOLC_MOTION_PRESET_DUPLICATE_NAME inside the wrapping GOLC_SHOW_
// STATE_INVALID diagnostic) -- never a silent duplicate.
func runMotionCreate(request Request) Result {
	usage := "motion create <name> --show <path>"
	name, showPath, err := parseMotionCreateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newMotionPreset, err := programming.NewMotionPreset(name, nil)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.MotionPresets = append(state.MotionPresets, newMotionPreset)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_CREATED: %s (%s)\n", newMotionPreset.Name, newMotionPreset.ID))}
}

// themeByName returns the theme in themes whose Name matches name, plus its
// index (so the caller can splice a mutated copy back into place).
func themeByName(themes []programming.Theme, name string) (programming.Theme, int, bool) {
	for i, th := range themes {
		if th.Name == name {
			return th, i, true
		}
	}
	return programming.Theme{}, -1, false
}

// presetByName returns the preset in presets whose Name matches name, plus
// its index (so the caller can splice a mutated copy back into place).
func presetByName(presets []programming.Preset, name string) (programming.Preset, int, bool) {
	for i, p := range presets {
		if p.Name == name {
			return p, i, true
		}
	}
	return programming.Preset{}, -1, false
}

// chaseByName returns the chase in chases whose Name matches name, plus its
// index (so the caller can splice a mutated copy back into place).
func chaseByName(chases []programming.Chase, name string) (programming.Chase, int, bool) {
	for i, c := range chases {
		if c.Name == name {
			return c, i, true
		}
	}
	return programming.Chase{}, -1, false
}

// motionByName returns the motion preset in motionPresets whose Name
// matches name, plus its index (so the caller can splice a mutated copy
// back into place).
func motionByName(motionPresets []programming.MotionPreset, name string) (programming.MotionPreset, int, bool) {
	for i, m := range motionPresets {
		if m.Name == name {
			return m, i, true
		}
	}
	return programming.MotionPreset{}, -1, false
}

// parseDomainNameShowArgs accepts exactly a positional name and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else with errCode (e.g. "GOLC_THEME_USAGE"/"GOLC_CHASE_USAGE").
// Shared by every {domain} delete route (PROG-07), which all take the
// identical <name> --show <path> shape -- mirrors parseSceneNameShowArgs's
// positional-name-plus-required-show shape, generalized across every
// domain's own error-code prefix.
func parseDomainNameShowArgs(errCode, usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("%s: usage: %s", errCode, usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("%s: --show requires a path; usage: %s", errCode, usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("%s: unsupported argument %q; usage: %s", errCode, argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("%s: --show is required; usage: %s", errCode, usage)
	}
	return name, showPath, nil
}

// parseDomainRenameArgs accepts exactly two positional names (old, new)
// followed by a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else with errCode. Shared by
// every {domain} rename/duplicate route (PROG-07) -- both take the
// identical <old-name> <new-name> --show <path> shape (rename replaces
// Name in place, preserving ID; duplicate mints a fresh ID/copy under the
// new name).
func parseDomainRenameArgs(errCode, usage string, args []string) (oldName, newName, showPath string, err error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return "", "", "", fmt.Errorf("%s: usage: %s", errCode, usage)
	}
	oldName = args[0]
	newName = args[1]

	rest := args[2:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", "", fmt.Errorf("%s: --show requires a path; usage: %s", errCode, usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", "", fmt.Errorf("%s: unsupported argument %q; usage: %s", errCode, argument, usage)
		}
	}
	if showPath == "" {
		return "", "", "", fmt.Errorf("%s: --show is required; usage: %s", errCode, usage)
	}
	return oldName, newName, showPath, nil
}

// runThemeRename serves the self-registered "theme rename" route (PROG-07):
// load the ShowState at --show, resolve the target theme by name, rename it
// (ID stable, via programming.RenameTheme), and save atomically. A
// not-found theme name fails with GOLC_THEME_NOT_FOUND; a rename target
// colliding with an existing theme name is rejected by show.Save's
// whole-State validation (GOLC_THEME_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic). This never inspects whether the
// theme is referenced by a currently-active scene layer before mutating
// (CONTEXT D-08) -- renaming never changes the theme's ID, so an active
// scene's Ref stays resolvable regardless.
func runThemeRename(request Request) Result {
	usage := "theme rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parseDomainRenameArgs("GOLC_THEME_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := themeByName(state.Themes, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_THEME_NOT_FOUND: no theme named %q exists\n", oldName))}
	}
	renamed, err := programming.RenameTheme(target, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Themes[index] = renamed

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_THEME_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID))}
}

// runThemeDelete serves the self-registered "theme delete" route (PROG-07):
// load the ShowState at --show, remove the named theme, and save
// atomically -- letting show.Save's whole-State validation reject a delete
// that would leave a scene layer's Ref dangling (GOLC_SCENE_LAYER_DANGLING_
// REFERENCE inside GOLC_SHOW_STATE_INVALID, CONTEXT threat T-03-01). A
// not-found theme name fails with GOLC_THEME_NOT_FOUND. This never checks
// scene-active status before mutating (CONTEXT D-08): the referential-
// integrity re-check is a separate, always-on safety mechanism, not a
// live-edit workflow gate.
func runThemeDelete(request Request) Result {
	usage := "theme delete <name> --show <path>"
	name, showPath, err := parseDomainNameShowArgs("GOLC_THEME_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	_, index, found := themeByName(state.Themes, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_THEME_NOT_FOUND: no theme named %q exists\n", name))}
	}
	state.Themes = append(state.Themes[:index], state.Themes[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_THEME_DELETED: %s\n", name))}
}

// runPresetRename serves the self-registered "preset rename" route
// (PROG-07): load the ShowState at --show, resolve the target preset by
// name, rename it (ID stable, via programming.RenamePreset), and save
// atomically. A not-found preset name fails with GOLC_PRESET_NOT_FOUND; a
// rename target colliding with an existing preset name is rejected by
// show.Save's whole-State validation (GOLC_PRESET_DUPLICATE_NAME inside the
// wrapping GOLC_SHOW_STATE_INVALID diagnostic).
func runPresetRename(request Request) Result {
	usage := "preset rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parseDomainRenameArgs("GOLC_PRESET_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := presetByName(state.Presets, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_PRESET_NOT_FOUND: no preset named %q exists\n", oldName))}
	}
	renamed, err := programming.RenamePreset(target, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Presets[index] = renamed

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PRESET_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID))}
}

// runPresetDelete serves the self-registered "preset delete" route
// (PROG-07): load the ShowState at --show, remove the named preset, and
// save atomically -- letting show.Save's whole-State validation reject a
// delete that would leave a scene base-look layer's Ref dangling
// (GOLC_SCENE_LAYER_DANGLING_REFERENCE inside GOLC_SHOW_STATE_INVALID,
// CONTEXT threat T-03-01). A not-found preset name fails with
// GOLC_PRESET_NOT_FOUND.
func runPresetDelete(request Request) Result {
	usage := "preset delete <name> --show <path>"
	name, showPath, err := parseDomainNameShowArgs("GOLC_PRESET_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	_, index, found := presetByName(state.Presets, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_PRESET_NOT_FOUND: no preset named %q exists\n", name))}
	}
	state.Presets = append(state.Presets[:index], state.Presets[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_PRESET_DELETED: %s\n", name))}
}

// chaseUpdateArgs is the parsed shape of one "chase update" invocation.
type chaseUpdateArgs struct {
	chaseName       string
	newName         string
	unit            programming.StepUnit
	hasUnit         bool
	stepDuration    float64
	hasStepDuration bool
	showPath        string
}

// parseChaseUpdateArgs accepts a positional chase name followed by any
// combination of an optional "--name <new-name>", an optional "--unit
// bar|beat", and an optional "--step-duration <value>", plus a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_CHASE_USAGE). At least one of --name/--unit/
// --step-duration is required -- a "chase update" with none of them would
// be a no-op invocation and is rejected rather than silently accepted.
// --unit/--step-duration's own validity is checked later by
// programming.ValidateChase, never re-derived here beyond --step-duration
// parsing as a float64.
func parseChaseUpdateArgs(usage string, args []string) (chaseUpdateArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: usage: %s", usage)
	}
	parsed := chaseUpdateArgs{chaseName: args[0]}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--name":
			if i+1 >= len(rest) {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --name requires a value; usage: %s", usage)
			}
			parsed.newName = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--name="):
			parsed.newName = strings.TrimPrefix(argument, "--name=")
			i++
		case argument == "--unit":
			if i+1 >= len(rest) {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --unit requires a value; usage: %s", usage)
			}
			parsed.unit = programming.StepUnit(rest[i+1])
			parsed.hasUnit = true
			i += 2
		case strings.HasPrefix(argument, "--unit="):
			parsed.unit = programming.StepUnit(strings.TrimPrefix(argument, "--unit="))
			parsed.hasUnit = true
			i++
		case argument == "--step-duration":
			if i+1 >= len(rest) {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --step-duration requires a value; usage: %s", usage)
			}
			value, parseErr := strconv.ParseFloat(rest[i+1], 64)
			if parseErr != nil {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --step-duration value %q is not a valid number; usage: %s", rest[i+1], usage)
			}
			parsed.stepDuration = value
			parsed.hasStepDuration = true
			i += 2
		case strings.HasPrefix(argument, "--step-duration="):
			raw := strings.TrimPrefix(argument, "--step-duration=")
			value, parseErr := strconv.ParseFloat(raw, 64)
			if parseErr != nil {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --step-duration value %q is not a valid number; usage: %s", raw, usage)
			}
			parsed.stepDuration = value
			parsed.hasStepDuration = true
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" {
		return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: --show is required; usage: %s", usage)
	}
	if parsed.newName == "" && !parsed.hasUnit && !parsed.hasStepDuration {
		return chaseUpdateArgs{}, fmt.Errorf("GOLC_CHASE_USAGE: at least one of --name/--unit/--step-duration is required; usage: %s", usage)
	}
	return parsed, nil
}

// runChaseUpdate serves the self-registered "chase update" route (PROG-07):
// load the ShowState at --show, resolve the target chase by name, apply
// whichever of --name/--unit/--step-duration were supplied (a --name change
// goes through programming.RenameChase -- ID stable; --unit/--step-duration
// mutate the field directly, re-validated by programming.ValidateChase
// before save), and save atomically. A not-found chase name fails with
// GOLC_CHASE_NOT_FOUND; a --name colliding with an existing chase is
// rejected by show.Save's whole-State validation (GOLC_CHASE_DUPLICATE_NAME
// inside the wrapping GOLC_SHOW_STATE_INVALID diagnostic); an invalid
// --unit/--step-duration is rejected by programming.ValidateChase
// (GOLC_CHASE_STEP_UNIT_INVALID/GOLC_CHASE_STEP_DURATION_INVALID). This
// never checks scene-active status before mutating (CONTEXT D-08).
func runChaseUpdate(request Request) Result {
	usage := "chase update <name> [--name <new-name>] [--unit bar|beat] [--step-duration <value>] --show <path>"
	parsed, err := parseChaseUpdateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := chaseByName(state.Chases, parsed.chaseName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_CHASE_NOT_FOUND: no chase named %q exists\n", parsed.chaseName))}
	}

	updated := target
	if parsed.newName != "" {
		renamed, err := programming.RenameChase(updated, parsed.newName)
		if err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		updated = renamed
	}
	if parsed.hasUnit {
		updated.StepUnit = parsed.unit
	}
	if parsed.hasStepDuration {
		updated.StepDuration = parsed.stepDuration
	}
	if err := programming.ValidateChase(updated); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Chases[index] = updated

	if err := show.Save(request.Root, parsed.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_CHASE_UPDATED: %s (%s)\n", updated.Name, updated.ID))}
}

// parseChaseReorderArgs accepts a positional chase name followed by a
// required "--order <comma-separated 0-based indices>" and a required
// "--show <path>" (both --flag value and --flag=value forms), rejecting
// anything else (GOLC_CHASE_USAGE). --order's own permutation validity
// (every index 0..len(steps)-1 exactly once) is checked later by
// reorderChaseSteps, never re-derived here -- this parser only requires the
// flag to be present and every comma-separated value to parse as an
// integer.
func parseChaseReorderArgs(usage string, args []string) (name string, order []int, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: usage: %s", usage)
	}
	name = args[0]

	var rawOrder string
	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--order":
			if i+1 >= len(rest) {
				return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: --order requires a value; usage: %s", usage)
			}
			rawOrder = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--order="):
			rawOrder = strings.TrimPrefix(argument, "--order=")
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if rawOrder == "" {
		return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: --order is required; usage: %s", usage)
	}
	if showPath == "" {
		return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: --show is required; usage: %s", usage)
	}
	for _, part := range strings.Split(rawOrder, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		index, parseErr := strconv.Atoi(trimmed)
		if parseErr != nil {
			return "", nil, "", fmt.Errorf("GOLC_CHASE_USAGE: --order value %q is not a valid integer; usage: %s", trimmed, usage)
		}
		order = append(order, index)
	}
	return name, order, showPath, nil
}

// reorderChaseSteps returns c with Steps permuted according to order
// (order[i] is the original index of the step that should occupy position
// i), rejecting any order that is not an exact permutation of
// [0, len(c.Steps)) -- GOLC_CHASE_USAGE for a wrong length, an
// out-of-range index, or a repeated index (CONTEXT threat T-03-05: reorder
// must never silently drop or duplicate a step). c.Steps itself is never
// mutated in place -- a fresh slice is built and assigned to the returned
// copy.
func reorderChaseSteps(c programming.Chase, order []int) (programming.Chase, error) {
	if len(order) != len(c.Steps) {
		return programming.Chase{}, fmt.Errorf(
			"GOLC_CHASE_USAGE: --order must list exactly %d indices (one per step), got %d", len(c.Steps), len(order))
	}
	seen := make(map[int]bool, len(order))
	reordered := make([]programming.ChaseStep, len(order))
	for i, index := range order {
		if index < 0 || index >= len(c.Steps) {
			return programming.Chase{}, fmt.Errorf("GOLC_CHASE_USAGE: --order index %d is out of range for %d steps", index, len(c.Steps))
		}
		if seen[index] {
			return programming.Chase{}, fmt.Errorf(
				"GOLC_CHASE_USAGE: --order index %d is repeated; it must be a permutation of 0..%d", index, len(c.Steps)-1)
		}
		seen[index] = true
		reordered[i] = c.Steps[index]
	}
	c.Steps = reordered
	return c, nil
}

// runChaseReorder serves the self-registered "chase reorder" route
// (PROG-07, CONTEXT threat T-03-05): load the ShowState at --show, resolve
// the target chase by name, permute its Steps according to --order (every
// original index exactly once -- a non-permutation is rejected with
// GOLC_CHASE_USAGE before any mutation), and save atomically. A not-found
// chase name fails with GOLC_CHASE_NOT_FOUND. This never checks
// scene-active status before mutating (CONTEXT D-08) -- reordering never
// changes the chase's ID, so an active scene's Ref stays resolvable
// regardless.
func runChaseReorder(request Request) Result {
	usage := "chase reorder <name> --order <i,j,k,...> --show <path>"
	name, order, showPath, err := parseChaseReorderArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := chaseByName(state.Chases, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_CHASE_NOT_FOUND: no chase named %q exists\n", name))}
	}

	reordered, err := reorderChaseSteps(target, order)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	state.Chases[index] = reordered

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_CHASE_REORDERED: %s (%s) steps=%d\n", reordered.Name, reordered.ID, len(reordered.Steps)))}
}

// runChaseDuplicate serves the self-registered "chase duplicate" route
// (PROG-07): load the ShowState at --show, resolve the source chase by
// name, mint a fresh UUIDv7-identified copy under the caller-supplied new
// name carrying the exact same Steps/StepUnit/StepDuration (via
// programming.NewChase, which re-validates and mints the new ID -- never
// re-using the source's ID), append it, and save atomically. A not-found
// source chase fails with GOLC_CHASE_NOT_FOUND; a new name colliding with
// an existing chase is rejected by show.Save's whole-State validation
// (GOLC_CHASE_DUPLICATE_NAME inside the wrapping GOLC_SHOW_STATE_INVALID
// diagnostic). This never checks scene-active status before mutating
// (CONTEXT D-08) -- duplicating never touches the source object, so an
// active scene's Ref to the source stays untouched and resolvable.
func runChaseDuplicate(request Request) Result {
	usage := "chase duplicate <name> <new-name> --show <path>"
	sourceName, newName, showPath, err := parseDomainRenameArgs("GOLC_CHASE_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	source, _, found := chaseByName(state.Chases, sourceName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_CHASE_NOT_FOUND: no chase named %q exists\n", sourceName))}
	}

	duplicate, err := programming.NewChase(newName, append([]programming.ChaseStep(nil), source.Steps...), source.StepUnit, source.StepDuration)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Chases = append(state.Chases, duplicate)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_CHASE_DUPLICATED: %s -> %s (%s)\n", sourceName, duplicate.Name, duplicate.ID))}
}

// runChaseDelete serves the self-registered "chase delete" route
// (PROG-07): load the ShowState at --show, remove the named chase, and
// save atomically -- letting show.Save's whole-State validation reject a
// delete that would leave a scene layer's Ref dangling
// (GOLC_SCENE_LAYER_DANGLING_REFERENCE inside GOLC_SHOW_STATE_INVALID,
// CONTEXT threat T-03-01). A not-found chase name fails with
// GOLC_CHASE_NOT_FOUND. This never checks scene-active status before
// mutating (CONTEXT D-08): the referential-integrity re-check is a
// separate, always-on safety mechanism, not a live-edit workflow gate.
func runChaseDelete(request Request) Result {
	usage := "chase delete <name> --show <path>"
	name, showPath, err := parseDomainNameShowArgs("GOLC_CHASE_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	_, index, found := chaseByName(state.Chases, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_CHASE_NOT_FOUND: no chase named %q exists\n", name))}
	}
	state.Chases = append(state.Chases[:index], state.Chases[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_CHASE_DELETED: %s\n", name))}
}

// runMotionRename serves the self-registered "motion rename" route
// (PROG-07): load the ShowState at --show, resolve the target motion
// preset by name, rename it (ID stable, via programming.RenameMotionPreset),
// and save atomically. A not-found motion preset name fails with
// GOLC_MOTION_PRESET_NOT_FOUND; a rename target colliding with an existing
// motion preset name is rejected by show.Save's whole-State validation
// (GOLC_MOTION_PRESET_DUPLICATE_NAME inside the wrapping GOLC_SHOW_STATE_
// INVALID diagnostic). This never checks scene-active status before
// mutating (CONTEXT D-08) -- renaming never changes the preset's ID, so an
// active scene's Ref stays resolvable regardless.
func runMotionRename(request Request) Result {
	usage := "motion rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parseDomainRenameArgs("GOLC_MOTION_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := motionByName(state.MotionPresets, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_NOT_FOUND: no motion preset named %q exists\n", oldName))}
	}
	renamed, err := programming.RenameMotionPreset(target, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.MotionPresets[index] = renamed

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID))}
}

// runMotionDuplicate serves the self-registered "motion duplicate" route
// (PROG-07): load the ShowState at --show, resolve the source motion
// preset by name, mint a fresh UUIDv7-identified copy under the
// caller-supplied new name carrying the exact same Keyframes (via
// programming.NewMotionPreset, which re-validates and mints the new ID),
// append it, and save atomically. A not-found source motion preset fails
// with GOLC_MOTION_PRESET_NOT_FOUND; a new name colliding with an existing
// motion preset is rejected by show.Save's whole-State validation
// (GOLC_MOTION_PRESET_DUPLICATE_NAME inside the wrapping GOLC_SHOW_STATE_
// INVALID diagnostic). This never checks scene-active status before
// mutating (CONTEXT D-08) -- duplicating never touches the source object.
func runMotionDuplicate(request Request) Result {
	usage := "motion duplicate <name> <new-name> --show <path>"
	sourceName, newName, showPath, err := parseDomainRenameArgs("GOLC_MOTION_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	source, _, found := motionByName(state.MotionPresets, sourceName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_NOT_FOUND: no motion preset named %q exists\n", sourceName))}
	}

	duplicate, err := programming.NewMotionPreset(newName, append([]programming.MotionKeyframe(nil), source.Keyframes...))
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.MotionPresets = append(state.MotionPresets, duplicate)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_DUPLICATED: %s -> %s (%s)\n", sourceName, duplicate.Name, duplicate.ID))}
}

// runMotionDelete serves the self-registered "motion delete" route
// (PROG-07): load the ShowState at --show, remove the named motion preset,
// and save atomically -- letting show.Save's whole-State validation reject
// a delete that would leave a scene motion layer's Ref dangling
// (GOLC_SCENE_LAYER_DANGLING_REFERENCE inside GOLC_SHOW_STATE_INVALID,
// CONTEXT threat T-03-01). A not-found motion preset name fails with
// GOLC_MOTION_PRESET_NOT_FOUND. This never checks scene-active status
// before mutating (CONTEXT D-08): the referential-integrity re-check is a
// separate, always-on safety mechanism, not a live-edit workflow gate.
func runMotionDelete(request Request) Result {
	usage := "motion delete <name> --show <path>"
	name, showPath, err := parseDomainNameShowArgs("GOLC_MOTION_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	_, index, found := motionByName(state.MotionPresets, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_NOT_FOUND: no motion preset named %q exists\n", name))}
	}
	state.MotionPresets = append(state.MotionPresets[:index], state.MotionPresets[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_MOTION_PRESET_DELETED: %s\n", name))}
}

// runSceneRename serves the self-registered "scene rename" route
// (PROG-07): load the ShowState at --show, resolve the target scene by
// name, rename it (ID stable, via scene.RenameScene), and save atomically.
// A not-found scene name fails with GOLC_SCENE_NOT_FOUND; a rename target
// colliding with an existing scene name is rejected by show.Save's
// whole-State validation (GOLC_SCENE_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic). This never checks whether the scene
// is the currently-active one before mutating (CONTEXT D-08) -- renaming
// never changes Active or the scene's ID.
func runSceneRename(request Request) Result {
	usage := "scene rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parseDomainRenameArgs("GOLC_SCENE_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	target, index, found := sceneByName(state.Scenes, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCENE_NOT_FOUND: no scene named %q exists\n", oldName))}
	}
	renamed, err := scene.RenameScene(target, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Scenes[index] = renamed

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SCENE_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID))}
}

// runSceneDuplicate serves the self-registered "scene duplicate" route
// (PROG-07): load the ShowState at --show, resolve the source scene by
// name, mint a fresh UUIDv7-identified copy under the caller-supplied new
// name carrying the exact same BarsPerLoop/PreserveMusicalPositionOnBPM
// Change/Layers, append it, and save atomically. The duplicate always
// starts inactive (Active: false, scene.NewScene's own default) regardless
// of the source's own Active state -- SCEN-04 permits at most one active
// scene at a time, so duplicating a currently-active scene must never
// silently create a second active scene (a mechanical correctness
// safeguard, not a plan-specified CLI flag). A not-found source scene
// fails with GOLC_SCENE_NOT_FOUND; a new name colliding with an existing
// scene is rejected by show.Save's whole-State validation
// (GOLC_SCENE_DUPLICATE_NAME inside the wrapping GOLC_SHOW_STATE_INVALID
// diagnostic). This never checks scene-active status before mutating
// (CONTEXT D-08) -- duplicating never touches the source object.
func runSceneDuplicate(request Request) Result {
	usage := "scene duplicate <name> <new-name> --show <path>"
	sourceName, newName, showPath, err := parseDomainRenameArgs("GOLC_SCENE_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	source, _, found := sceneByName(state.Scenes, sourceName)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCENE_NOT_FOUND: no scene named %q exists\n", sourceName))}
	}

	duplicate, err := scene.NewScene(newName, source.BarsPerLoop)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	duplicate.PreserveMusicalPositionOnBPMChange = source.PreserveMusicalPositionOnBPMChange
	duplicate.Layers = source.Layers
	state.Scenes = append(state.Scenes, duplicate)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SCENE_DUPLICATED: %s -> %s (%s)\n", sourceName, duplicate.Name, duplicate.ID))}
}

// runSceneDelete serves the self-registered "scene delete" route
// (PROG-07): load the ShowState at --show, remove the named scene, and
// save atomically. A not-found scene name fails with GOLC_SCENE_NOT_FOUND.
// Deleting the currently-active scene is not specially rejected here --
// scene.ValidateSingleActiveScene only bounds the maximum active count at
// one, never a minimum, so a show with zero active scenes (nothing
// playing) remains valid. This never checks scene-active status before
// mutating (CONTEXT D-08).
func runSceneDelete(request Request) Result {
	usage := "scene delete <name> --show <path>"
	name, showPath, err := parseDomainNameShowArgs("GOLC_SCENE_USAGE", usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	_, index, found := sceneByName(state.Scenes, name)
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_SCENE_NOT_FOUND: no scene named %q exists\n", name))}
	}
	state.Scenes = append(state.Scenes[:index], state.Scenes[index+1:]...)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_SCENE_DELETED: %s\n", name))}
}
