// pool.go is the pool command file: it owns the "pool" routing scope and
// self-registers the "pool create" route (CONTEXT D-04/POOL-01): a show
// author defines a logical pool of compatible fixtures, independent of
// concrete count/address/hardware, against a working ShowState document.
package command

import (
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "pool",
	Summary: "Logical fixture pool definitions, independent of concrete count/address/hardware.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "pool create",
	Summary: "Create a named logical pool against a ShowState document: pool create <name> [--requires <cap1,cap2,...>] --show <path>.",
	Handler: runPoolCreate,
})

// runPoolCreate serves the self-registered "pool create" route: load the
// ShowState at --show, append the new pool, and save atomically. A
// duplicate pool name is rejected by show.Save's whole-State validation
// (surfaced as GOLC_POOL_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic) -- never a silent duplicate.
func runPoolCreate(request Request) Result {
	name, showPath, requires, err := parsePoolCreateArgs("pool create <name> [--requires <cap1,cap2,...>] --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newPool, err := pool.NewPool(name, requires)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Pools = append(state.Pools, newPool)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_POOL_CREATED: %s (%s)\n", newPool.Name, newPool.ID))}
}

// parsePoolCreateArgs accepts exactly: a positional pool name, an
// optional "--requires <comma-separated capability types>", and a
// required "--show <path>" (both --flag value and --flag=value forms),
// rejecting anything else (GOLC_POOL_USAGE).
func parsePoolCreateArgs(usage string, args []string) (name, showPath string, requires []fixture.CapabilityType, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", nil, fmt.Errorf("GOLC_POOL_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--requires":
			if i+1 >= len(rest) {
				return "", "", nil, fmt.Errorf("GOLC_POOL_USAGE: --requires requires a value; usage: %s", usage)
			}
			requires = parseCapabilityList(rest[i+1])
			i += 2
		case strings.HasPrefix(argument, "--requires="):
			requires = parseCapabilityList(strings.TrimPrefix(argument, "--requires="))
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", nil, fmt.Errorf("GOLC_POOL_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", nil, fmt.Errorf("GOLC_POOL_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", nil, fmt.Errorf("GOLC_POOL_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, requires, nil
}

// parseCapabilityList splits a comma-separated capability-type list,
// trimming whitespace and dropping empty entries so "--requires
// intensity, color" and "--requires intensity,color" behave identically.
func parseCapabilityList(raw string) []fixture.CapabilityType {
	var types []fixture.CapabilityType
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		types = append(types, fixture.CapabilityType(trimmed))
	}
	return types
}
