// pool.go is the pool command file: it owns the "pool" routing scope and
// self-registers "pool create" (CONTEXT D-04/POOL-01), plus the D-15
// Terraform-style "pool update"/"pool apply" plan/apply split (CONTEXT
// POOL-03/POOL-04/POOL-05/POOL-08): "pool update" computes and
// writes/prints a deterministic impact-review plan and mutates nothing;
// "pool apply" validates (integrity then freshness) and applies it
// atomically. Propagation (review vs immediate) is configurable per
// update, resolved from application_defaults.pool_update_review through
// internal/projectconfig, with review-before-apply as the default
// (POOL-04).
package command

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/projectconfig"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/substitution"
)

// poolUpdateReviewKey is the canonical five-layer configuration key
// resolvePoolUpdateReview reads (CONTEXT POOL-04): committed default is
// "preview" (config/application-defaults.toml); the key is locked
// (internal/projectconfig.DefaultRegistry), so only this command's own
// --propagate flag -- never a higher configuration layer -- may override
// it for one invocation.
const poolUpdateReviewKey = "application_defaults.pool_update_review"

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "pool",
	Summary: "Logical fixture pool definitions, independent of concrete count/address/hardware.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "pool create",
	Summary: "Create a named logical pool against a ShowState document: pool create <name> [--requires <cap1,cap2,...>] --show <path>.",
	Handler: runPoolCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool update",
	Summary: "Compute and write/print a deterministic pool impact-review plan without mutating the ShowState document: " +
		"pool update <pool> [--add <fixture_stable_key>|<fixture_content_hash>|<mode>[|<channel_count>]]... " +
		"[--remove <pool_member_id>]... " +
		"[--attach-deployment <deployment_id>]... [--start-universe <n>] [--start-address <n>] " +
		"[--propagate immediate|preview] [--out <path>] [--json] --show <path>. " +
		"WARNING: --add's <fixture_stable_key>|<fixture_content_hash> pair is a low-level, unverified reference -- " +
		"unlike \"pool substitute\", nothing here decodes, pins, or otherwise checks it against a real fixture " +
		"definition; deriving a trustworthy pair (for example via \"fixture import\"/\"fixture inspect\") is the " +
		"caller's own responsibility. The optional trailing channel_count field is the mode's real channel width " +
		"(for example 5 for a 5-channel RGBW+strobe mode); when omitted, every proposed instance falls back to a " +
		"1-channel width, which can pack multiple wide fixtures too closely together in one batch add. " +
		"--attach-deployment force-includes a deployment that has never referenced " +
		"this pool before in the add dependent walk (closes the \"adopt a never-before-used pool\" gap); " +
		"--start-universe/--start-address optionally anchor the auto-address scan for every newly proposed " +
		"instance in this request instead of the default next-free slot.",
	Handler: runPoolUpdate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool apply",
	Summary: "Validate (integrity then freshness) and atomically apply an already-reviewed pool impact plan: " +
		"pool apply {plan-file} --plan-id <id> --show <path>.",
	Handler: runPoolApply,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool substitute",
	Summary: "Compute and write/print a deterministic fixture-substitution capability-diff review without mutating the ShowState document: " +
		"pool substitute <pool> --from <fixture-file> --to <fixture-file> [--out <path>] [--json] --show <path>.",
	Handler: runPoolSubstitute,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "pool rename",
	Summary: "Rename a pool, identity unchanged: pool rename <old-name> <new-name> --show <path>.",
	Handler: runPoolRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool delete",
	Summary: "Delete a pool, cascading to every deployment instance and group member ref that references it: " +
		"pool delete <name> --show <path>.",
	Handler: runPoolDelete,
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
	return Result{Stdout: fmt.Appendf(nil, "GOLC_POOL_CREATED: %s (%s)\n", newPool.Name, newPool.ID)}
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

// parsePositiveInt parses raw as a base-10 integer no smaller than 1,
// rejecting non-numeric input and zero/negative values alike -- shared by
// --start-universe/--start-address, where an explicit 0 or negative value
// is a usage error (only omitting the flag entirely means "use the
// system-suggested default").
func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("value %q must be a positive integer", raw)
	}
	return value, nil
}

// poolByName returns the pool in pools whose Name matches name.
func poolByName(pools []pool.Pool, name string) (pool.Pool, bool) {
	for _, p := range pools {
		if p.Name == name {
			return p, true
		}
	}
	return pool.Pool{}, false
}

// parsePoolRenameArgs accepts exactly two positionals (old name, new name)
// followed by a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_POOL_USAGE) --
// mirrors parsePoolCreateArgs' single-positional-then-flags shape, widened
// to two positionals.
func parsePoolRenameArgs(usage string, args []string) (oldName, newName, showPath string, err error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return "", "", "", fmt.Errorf("GOLC_POOL_USAGE: usage: %s", usage)
	}
	oldName, newName = args[0], args[1]

	rest := args[2:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", "", fmt.Errorf("GOLC_POOL_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", "", fmt.Errorf("GOLC_POOL_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", "", fmt.Errorf("GOLC_POOL_USAGE: --show is required; usage: %s", usage)
	}
	return oldName, newName, showPath, nil
}

// runPoolRename serves the self-registered "pool rename" route: load the
// ShowState, resolve the pool by its current name, rename it (ID never
// re-minted -- POOL-01), replace it in place, and save. A rename
// colliding with an existing pool's name is rejected by show.Save's
// whole-State validation (GOLC_POOL_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID), exactly like "pool create" already relies on.
func runPoolRename(request Request) Result {
	usage := "pool rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parsePoolRenameArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetPool, found := poolByName(state.Pools, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_NOT_FOUND: no pool named %q exists\n", oldName)}
	}
	renamed, err := pool.Rename(targetPool, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	for i, p := range state.Pools {
		if p.ID == renamed.ID {
			state.Pools[i] = renamed
			break
		}
	}

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_POOL_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID)}
}

// parsePoolNameShowArgs accepts exactly one positional pool name followed
// by a required "--show <path>" (both forms), rejecting anything else
// (GOLC_POOL_USAGE) -- mirrors internal/command/deployment.go's
// parseDeploymentNameShowArgs, pool-scoped.
func parsePoolNameShowArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_POOL_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_POOL_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_POOL_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_POOL_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// runPoolDelete serves the self-registered "pool delete" route: load the
// ShowState, resolve the pool by name, cascade-delete it via
// pool.DeletePool (removing every deployment instance and group member
// ref that references it), and save. No explicit dangling-Selection scrub
// is needed here -- show.Save itself scrubs every Scene's Layer.Selection
// before validating, so a scene referencing the deleted pool is cleaned up
// automatically.
func runPoolDelete(request Request) Result {
	usage := "pool delete <name> --show <path>"
	name, showPath, err := parsePoolNameShowArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetPool, found := poolByName(state.Pools, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_NOT_FOUND: no pool named %q exists\n", name)}
	}

	newPools, newDeployments, newGroups, err := pool.DeletePool(state.Pools, state.Deployments, state.Groups, targetPool.ID)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Pools = newPools
	state.Deployments = newDeployments
	state.Groups = newGroups

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_POOL_DELETED: %s (%s)\n", name, targetPool.ID)}
}

// poolUpdateArgs is the parsed shape of one "pool update" invocation.
type poolUpdateArgs struct {
	poolName          string
	add               []pool.PoolMemberSpec
	remove            []uuid.UUID
	attachDeployments []uuid.UUID
	startUniverse     int
	startAddress      int
	propagateOverride string
	outPath           string
	json              bool
	showPath          string
}

// parsePoolMemberSpec parses one "--add" value in the
// "<fixture_stable_key>|<fixture_content_hash>|<mode>[|<channel_count>]"
// shape: the trailing channel_count field is optional. "|" (not ":") is
// the field separator because a content hash routinely carries its own
// algorithm prefix (for example "sha256:...").
//
// WR-03: this only checks the first three fields are present and
// non-empty -- it never decodes, pins, or cross-checks fixture_stable_key/
// fixture_content_hash against a real fixture.FixtureDefinition the way
// "pool substitute" (which does read and fixture.Decode/fixture.Pin its
// --from/--to files) does. A caller can therefore pass a stable
// key/content hash pair that was never validated, decoded, or even
// exists; FIXT-05's content-addressed-pinning guarantee is only as
// trustworthy here as whatever produced the string the caller supplies
// (for example "fixture import"/"fixture inspect" output). Verifying
// that pair against an actual fixture definition before it enters a
// show is the caller's own responsibility until this route is changed
// to accept a fixture file path directly.
//
// channel_count, when supplied, is the real channel width of the named
// mode (for example 5 for a 5-channel RGBW+strobe mode); it lets
// pool.BuildImpactPlan space each newly proposed instance by the
// fixture's actual footprint instead of the 1-channel fallback
// (pool.defaultInstanceChannelCount) it otherwise uses. A caller that
// omits it (the common raw-CLI case, since nothing here resolves it from
// a real fixture definition) gets that 1-channel fallback, same as
// before this field existed.
func parsePoolMemberSpec(raw string) (pool.PoolMemberSpec, error) {
	parts := strings.SplitN(raw, "|", 4)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return pool.PoolMemberSpec{}, fmt.Errorf(
			"GOLC_POOL_APPLY_USAGE: --add value %q must be \"<fixture_stable_key>|<fixture_content_hash>|<mode>[|<channel_count>]\"", raw)
	}
	spec := pool.PoolMemberSpec{FixtureStableKey: parts[0], FixtureContentHash: parts[1], Mode: parts[2]}
	if len(parts) == 4 && parts[3] != "" {
		channelCount, err := parsePositiveInt(parts[3])
		if err != nil {
			return pool.PoolMemberSpec{}, fmt.Errorf(
				"GOLC_POOL_APPLY_USAGE: --add value %q channel_count field %v", raw, err)
		}
		spec.ChannelCount = channelCount
	}
	return spec, nil
}

// parsePoolUpdateArgs accepts a positional pool name followed by any
// number of --add/--remove flags, an optional --propagate override, an
// optional --out path, an optional --json flag, and a required --show
// path (both --flag value and --flag=value forms), rejecting anything
// else (GOLC_POOL_APPLY_USAGE).
func parsePoolUpdateArgs(usage string, args []string) (poolUpdateArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: usage: %s", usage)
	}
	parsed := poolUpdateArgs{poolName: args[0]}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--add":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --add requires a value; usage: %s", usage)
			}
			spec, err := parsePoolMemberSpec(rest[i+1])
			if err != nil {
				return poolUpdateArgs{}, err
			}
			parsed.add = append(parsed.add, spec)
			i += 2
		case strings.HasPrefix(argument, "--add="):
			spec, err := parsePoolMemberSpec(strings.TrimPrefix(argument, "--add="))
			if err != nil {
				return poolUpdateArgs{}, err
			}
			parsed.add = append(parsed.add, spec)
			i++
		case argument == "--remove":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --remove requires a value; usage: %s", usage)
			}
			id, err := uuid.Parse(rest[i+1])
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --remove value %q is not a valid pool member id; usage: %s", rest[i+1], usage)
			}
			parsed.remove = append(parsed.remove, id)
			i += 2
		case strings.HasPrefix(argument, "--remove="):
			raw := strings.TrimPrefix(argument, "--remove=")
			id, err := uuid.Parse(raw)
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --remove value %q is not a valid pool member id; usage: %s", raw, usage)
			}
			parsed.remove = append(parsed.remove, id)
			i++
		case argument == "--attach-deployment":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --attach-deployment requires a value; usage: %s", usage)
			}
			id, err := uuid.Parse(rest[i+1])
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --attach-deployment value %q is not a valid deployment id; usage: %s", rest[i+1], usage)
			}
			parsed.attachDeployments = append(parsed.attachDeployments, id)
			i += 2
		case strings.HasPrefix(argument, "--attach-deployment="):
			raw := strings.TrimPrefix(argument, "--attach-deployment=")
			id, err := uuid.Parse(raw)
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --attach-deployment value %q is not a valid deployment id; usage: %s", raw, usage)
			}
			parsed.attachDeployments = append(parsed.attachDeployments, id)
			i++
		case argument == "--start-universe":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-universe requires a value; usage: %s", usage)
			}
			value, err := parsePositiveInt(rest[i+1])
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-universe value %q must be a positive integer; usage: %s", rest[i+1], usage)
			}
			parsed.startUniverse = value
			i += 2
		case strings.HasPrefix(argument, "--start-universe="):
			raw := strings.TrimPrefix(argument, "--start-universe=")
			value, err := parsePositiveInt(raw)
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-universe value %q must be a positive integer; usage: %s", raw, usage)
			}
			parsed.startUniverse = value
			i++
		case argument == "--start-address":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-address requires a value; usage: %s", usage)
			}
			value, err := parsePositiveInt(rest[i+1])
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-address value %q must be a positive integer; usage: %s", rest[i+1], usage)
			}
			parsed.startAddress = value
			i += 2
		case strings.HasPrefix(argument, "--start-address="):
			raw := strings.TrimPrefix(argument, "--start-address=")
			value, err := parsePositiveInt(raw)
			if err != nil {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --start-address value %q must be a positive integer; usage: %s", raw, usage)
			}
			parsed.startAddress = value
			i++
		case argument == "--propagate":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --propagate requires a value; usage: %s", usage)
			}
			parsed.propagateOverride = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--propagate="):
			parsed.propagateOverride = strings.TrimPrefix(argument, "--propagate=")
			i++
		case argument == "--out":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --out requires a path; usage: %s", usage)
			}
			parsed.outPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--out="):
			parsed.outPath = strings.TrimPrefix(argument, "--out=")
			i++
		case argument == "--json":
			parsed.json = true
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" {
		return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --show is required; usage: %s", usage)
	}
	if parsed.propagateOverride != "" && parsed.propagateOverride != "immediate" && parsed.propagateOverride != "preview" {
		return poolUpdateArgs{}, fmt.Errorf("GOLC_POOL_APPLY_USAGE: --propagate must be \"immediate\" or \"preview\", got %q; usage: %s", parsed.propagateOverride, usage)
	}
	return parsed, nil
}

// resolvePoolUpdateReview resolves the propagation mode for one "pool
// update" invocation (CONTEXT POOL-04): an explicit, already-validated
// --propagate override always wins for this invocation only (it never
// changes the committed default); otherwise the committed
// application_defaults.pool_update_review default is read through
// internal/projectconfig (locked: only the committed layer can ever
// supply it), which resolves to "preview" (review-required) unless the
// committed concern file itself is edited to "immediate" -- an unset or
// otherwise-unresolvable default is never silently treated as
// "immediate".
func resolvePoolUpdateReview(root, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	record, err := projectconfig.ResolveKey(projectconfig.DefaultRegistry(), projectconfig.NewSources(root), poolUpdateReviewKey)
	if err != nil {
		return "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: resolving %s: %v", poolUpdateReviewKey, err)
	}
	if record.Value != "immediate" && record.Value != "preview" {
		return "preview", nil
	}
	return record.Value, nil
}

// writeImpactPlan canonically encodes plan and writes it to outPath.
func writeImpactPlan(root, outPath string, plan pool.ImpactPlan) Result {
	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_PLAN_ENCODE_FAILED: %v\n", err)}
	}
	destination := resolveWritablePath(root, outPath)
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_PLAN_WRITE_FAILED: %v\n", err)}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_POOL_PLAN: wrote %s\n", destination)}
}

// runPoolUpdate serves the self-registered "pool update" route (CONTEXT
// POOL-03/POOL-04/D-11/D-15): it loads the ShowState at --show, resolves
// the target pool by name, resolves the propagation mode, builds a
// deterministic pool.ImpactPlan (never mutating the ShowState), and
// either writes it to --out, prints its canonical JSON (--json), or
// prints a short human-readable summary. This is the dry-run half of the
// D-15 plan/apply split: no code path here can ever write the ShowState
// file (CONTEXT T-02-12).
func runPoolUpdate(request Request) Result {
	usage := "pool update <pool> [--add <fixture_stable_key>|<fixture_content_hash>|<mode>[|<channel_count>]]... " +
		"[--remove <pool_member_id>]... [--attach-deployment <deployment_id>]... " +
		"[--start-universe <n>] [--start-address <n>] " +
		"[--propagate immediate|preview] [--out <path>] [--json] --show <path>"
	parsed, err := parsePoolUpdateArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetPool, found := poolByName(state.Pools, parsed.poolName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_NOT_FOUND: no pool named %q exists\n", parsed.poolName)}
	}

	propagate, err := resolvePoolUpdateReview(request.Root, parsed.propagateOverride)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	req := pool.ImpactRequest{
		PoolID:            targetPool.ID,
		Add:               parsed.add,
		Remove:            parsed.remove,
		AttachDeployments: parsed.attachDeployments,
		StartUniverse:     parsed.startUniverse,
		StartAddress:      parsed.startAddress,
		Propagate:         propagate,
	}
	plan, err := pool.BuildImpactPlan(state.Pools, state.Deployments, state.Groups, state.Revision, req)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if parsed.outPath != "" {
		return writeImpactPlan(request.Root, parsed.outPath, plan)
	}
	if parsed.json {
		payload, err := strictjson.CanonicalEncode(plan)
		if err != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_PLAN_ENCODE_FAILED: %v\n", err)}
		}
		return Result{Stdout: payload}
	}
	return Result{Stdout: fmt.Appendf(nil,
		"GOLC_POOL_PLAN: pool=%s operations=%d plan_id=%s propagate=%s\n",
		targetPool.Name, len(plan.Operations), plan.PlanID, plan.Propagate)}
}

// parsePoolApplyArgs accepts exactly the supported apply form: a plan
// file path (the first argument, never a flag) followed by --plan-id
// <id> and --show <path> (both --flag value and --flag=value forms),
// mirroring internal/command/linear.go's parseApplyArgs positional-plus-
// flag shape.
func parsePoolApplyArgs(usage string, args []string) (planFile, planID, showPath string, err error) {
	if len(args) == 0 {
		return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: usage: %s", usage)
	}
	planFile = args[0]
	if strings.HasPrefix(planFile, "--") {
		return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: usage: %s", usage)
	}
	for i := 1; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--plan-id":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: --plan-id requires a value; usage: %s", usage)
			}
			planID = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--plan-id="):
			planID = strings.TrimPrefix(argument, "--plan-id=")
			i++
		case argument == "--show":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if planFile == "" || planID == "" || showPath == "" {
		return "", "", "", fmt.Errorf("GOLC_POOL_APPLY_USAGE: usage: %s", usage)
	}
	return planFile, planID, showPath, nil
}

// runPoolApply serves the self-registered "pool apply" route (CONTEXT
// POOL-04/POOL-05/D-16): it strictly decodes the plan file, requires
// --plan-id to exactly match the loaded plan's own plan_id, runs
// ValidatePlanIntegrity then ValidatePlanFreshness against the current
// ShowState, applies atomically, and saves -- bumping Revision, the
// single-use guard for any later re-apply of the exact same plan.
func runPoolApply(request Request) Result {
	usage := "pool apply {plan-file} --plan-id <id> --show <path>"
	planFile, planIDValue, showPath, err := parsePoolApplyArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	resolvedPlanFile := resolveWritablePath(request.Root, planFile)
	data, err := os.ReadFile(resolvedPlanFile)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_APPLY_PLAN_READ: %s: %v\n", resolvedPlanFile, err)}
	}
	var plan pool.ImpactPlan
	if err := strictjson.DecodeStrict(data, &plan); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_APPLY_PLAN_DECODE: %s: %v\n", resolvedPlanFile, err)}
	}
	if err := pool.ValidatePlanIntegrity(plan); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if plan.PlanID != planIDValue {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil,
			"GOLC_POOL_APPLY_PLAN_ID_MISMATCH: --plan-id %q does not match the loaded plan's own plan_id %q\n", planIDValue, plan.PlanID)}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if err := pool.ValidatePlanFreshness(plan, state.Pools, state.Deployments, state.Groups, state.Revision); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newPools, newDeployments, newGroups, err := pool.Apply(state.Pools, state.Deployments, state.Groups, plan)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Pools = newPools
	state.Deployments = newDeployments
	state.Groups = newGroups

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_POOL_APPLY: applied %s (%d operations)\n", plan.PlanID, len(plan.Operations))}
}

// poolSubstituteArgs is the parsed shape of one "pool substitute"
// invocation.
type poolSubstituteArgs struct {
	poolName string
	fromPath string
	toPath   string
	outPath  string
	json     bool
	showPath string
}

// parsePoolSubstituteArgs accepts a positional pool name followed by
// required --from/--to fixture file paths, an optional --out path, an
// optional --json flag, and a required --show path (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_SUBSTITUTION_USAGE) --
// mirroring parsePoolUpdateArgs's exact parsing shape.
func parsePoolSubstituteArgs(usage string, args []string) (poolSubstituteArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: usage: %s", usage)
	}
	parsed := poolSubstituteArgs{poolName: args[0]}

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--from":
			if i+1 >= len(rest) {
				return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: --from requires a path; usage: %s", usage)
			}
			parsed.fromPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--from="):
			parsed.fromPath = strings.TrimPrefix(argument, "--from=")
			i++
		case argument == "--to":
			if i+1 >= len(rest) {
				return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: --to requires a path; usage: %s", usage)
			}
			parsed.toPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--to="):
			parsed.toPath = strings.TrimPrefix(argument, "--to=")
			i++
		case argument == "--out":
			if i+1 >= len(rest) {
				return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: --out requires a path; usage: %s", usage)
			}
			parsed.outPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--out="):
			parsed.outPath = strings.TrimPrefix(argument, "--out=")
			i++
		case argument == "--json":
			parsed.json = true
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.fromPath == "" || parsed.toPath == "" || parsed.showPath == "" {
		return poolSubstituteArgs{}, fmt.Errorf("GOLC_SUBSTITUTION_USAGE: --from, --to, and --show are required; usage: %s", usage)
	}
	return parsed, nil
}

// runPoolSubstitute serves the self-registered "pool substitute" route
// (CONTEXT POOL-06/POOL-07/POOL-08/D-14/D-15): it loads the ShowState at
// --show, resolves the target pool by name, strictly decodes both the
// --from and --to fixture files (fixture.Decode, the same FIXT-01/02
// pipeline "fixture validate" uses), builds a deterministic capability-diff
// substitution.SubstitutionPlan (never mutating the ShowState), and either
// writes it to --out, prints its canonical JSON (--json), or prints a
// short human-readable summary. A --to file that fails fixture.Decode's
// own strict decode/validate is a hard-blocking, route-level
// GOLC_SUBSTITUTION_TARGET_INVALID failure (T-02-14) before any plan can
// even be built; this is the same diagnostic code
// BuildSubstitutionPlan itself emits inside a plan's Errors when called
// directly with an already-decoded, structurally invalid target. Like
// "pool update", no code path here can ever write the ShowState file
// (D-15): acceptance is the existing "pool apply" route (no second apply
// mechanism), cancel is discarding the written plan file, and revise is
// re-running "pool substitute" with a different --to (D-13).
func runPoolSubstitute(request Request) Result {
	usage := "pool substitute <pool> --from <fixture-file> --to <fixture-file> [--out <path>] [--json] --show <path>"
	parsed, err := parsePoolSubstituteArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetPool, found := poolByName(state.Pools, parsed.poolName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_NOT_FOUND: no pool named %q exists\n", parsed.poolName)}
	}

	fromData, err := os.ReadFile(resolveWritablePath(request.Root, parsed.fromPath))
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_FIXTURE_READ_FAILED: %v\n", err)}
	}
	from, err := fixture.Decode(fromData)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	toData, err := os.ReadFile(resolveWritablePath(request.Root, parsed.toPath))
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_FIXTURE_READ_FAILED: %v\n", err)}
	}
	to, err := fixture.Decode(toData)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_SUBSTITUTION_TARGET_INVALID: %v\n", err)}
	}

	fromIdentity, err := fixture.Pin(from)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	toIdentity, err := fixture.Pin(to)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	substitutionReq := substitution.SubstitutionRequest{
		PoolID:         targetPool.ID,
		FromFixtureRef: fromIdentity.StableKey,
		ToFixtureRef:   toIdentity.StableKey,
	}
	plan, err := substitution.BuildSubstitutionPlan(state, from, to, substitutionReq)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if parsed.outPath != "" {
		return writeImpactPlan(request.Root, parsed.outPath, plan)
	}
	if parsed.json {
		payload, err := strictjson.CanonicalEncode(plan)
		if err != nil {
			return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_POOL_PLAN_ENCODE_FAILED: %v\n", err)}
		}
		return Result{Stdout: payload}
	}
	return Result{Stdout: fmt.Appendf(nil,
		"GOLC_POOL_PLAN: pool=%s operations=%d warnings=%d errors=%d plan_id=%s\n",
		targetPool.Name, len(plan.Operations), len(plan.Warnings), len(plan.Errors), plan.PlanID)}
}
