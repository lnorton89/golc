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
