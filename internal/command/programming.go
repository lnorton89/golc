// programming.go is the programmer command file: it owns the
// "programmer" routing scope and self-registers "programmer set" /
// "programmer inspect" / "programmer clear" (CONTEXT PROG-01/PROG-02/
// PROG-03): a show author resolves a fixture selection (pool/group/
// deployment-instance/direct-fixture, via programming.Resolve), edits
// semantic attribute values on the resolved instances, and inspects or
// clears the resulting programmer.ProgrammerState scratch buffer
// persisted on show.State. Handlers follow internal/command/pool.go's
// parse-args-then-Load-mutate-Save-Stdout shape.
package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
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
