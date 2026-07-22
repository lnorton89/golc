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
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/projectconfig"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
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
		"pool update <pool> [--add <fixture_stable_key>|<fixture_content_hash>|<mode>]... [--remove <pool_member_id>]... " +
		"[--propagate immediate|preview] [--out <path>] [--json] --show <path>.",
	Handler: runPoolUpdate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "pool apply",
	Summary: "Validate (integrity then freshness) and atomically apply an already-reviewed pool impact plan: " +
		"pool apply {plan-file} --plan-id <id> --show <path>.",
	Handler: runPoolApply,
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

// poolByName returns the pool in pools whose Name matches name.
func poolByName(pools []pool.Pool, name string) (pool.Pool, bool) {
	for _, p := range pools {
		if p.Name == name {
			return p, true
		}
	}
	return pool.Pool{}, false
}

// poolUpdateArgs is the parsed shape of one "pool update" invocation.
type poolUpdateArgs struct {
	poolName          string
	add               []pool.PoolMemberSpec
	remove            []uuid.UUID
	propagateOverride string
	outPath           string
	json              bool
	showPath          string
}

// parsePoolMemberSpec parses one "--add" value in the exact
// "<fixture_stable_key>|<fixture_content_hash>|<mode>" shape. "|" (not
// ":") is the field separator because a content hash routinely carries
// its own algorithm prefix (for example "sha256:...").
func parsePoolMemberSpec(raw string) (pool.PoolMemberSpec, error) {
	parts := strings.SplitN(raw, "|", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return pool.PoolMemberSpec{}, fmt.Errorf(
			"GOLC_POOL_APPLY_USAGE: --add value %q must be \"<fixture_stable_key>|<fixture_content_hash>|<mode>\"", raw)
	}
	return pool.PoolMemberSpec{FixtureStableKey: parts[0], FixtureContentHash: parts[1], Mode: parts[2]}, nil
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
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_PLAN_ENCODE_FAILED: %v\n", err))}
	}
	destination := resolveWritablePath(root, outPath)
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_PLAN_WRITE_FAILED: %v\n", err))}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_POOL_PLAN: wrote %s\n", destination))}
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
	usage := "pool update <pool> [--add <fixture_stable_key>|<fixture_content_hash>|<mode>]... " +
		"[--remove <pool_member_id>]... [--propagate immediate|preview] [--out <path>] [--json] --show <path>"
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
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_NOT_FOUND: no pool named %q exists\n", parsed.poolName))}
	}

	propagate, err := resolvePoolUpdateReview(request.Root, parsed.propagateOverride)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	req := pool.ImpactRequest{
		PoolID:    targetPool.ID,
		Add:       parsed.add,
		Remove:    parsed.remove,
		Propagate: propagate,
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
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_PLAN_ENCODE_FAILED: %v\n", err))}
		}
		return Result{Stdout: payload}
	}
	return Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_POOL_PLAN: pool=%s operations=%d plan_id=%s propagate=%s\n",
		targetPool.Name, len(plan.Operations), plan.PlanID, plan.Propagate))}
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
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_APPLY_PLAN_READ: %s: %v\n", resolvedPlanFile, err))}
	}
	var plan pool.ImpactPlan
	if err := strictjson.DecodeStrict(data, &plan); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_POOL_APPLY_PLAN_DECODE: %s: %v\n", resolvedPlanFile, err))}
	}
	if err := pool.ValidatePlanIntegrity(plan); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if plan.PlanID != planIDValue {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_POOL_APPLY_PLAN_ID_MISMATCH: --plan-id %q does not match the loaded plan's own plan_id %q\n", planIDValue, plan.PlanID))}
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
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_POOL_APPLY: applied %s (%d operations)\n", plan.PlanID, len(plan.Operations)))}
}
