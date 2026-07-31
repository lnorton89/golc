// deployment.go is the deployment command file: it owns the
// "deployment" routing scope and self-registers "deployment create" /
// "deployment activate" (CONTEXT D-04/D-09/POOL-02), plus the "show"
// scope and "show inspect" route (CONTEXT D-04): a show author creates
// named deployments mapping pools to concrete instances, activates
// exactly one at a time, and inspects the resulting show document.
package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/show"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "deployment",
	Summary: "Named mappings of logical pools to concrete fixture instances, modes, universes, and addresses.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "deployment create",
	Summary: "Create a named deployment against a ShowState document: deployment create <name> --show <path>.",
	Handler: runDeploymentCreate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "deployment activate",
	Summary: "Mark exactly one deployment active, deactivating every other deployment: deployment activate <name> --show <path>.",
	Handler: runDeploymentActivate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "deployment rename",
	Summary: "Rename a deployment, identity unchanged: deployment rename <old-name> <new-name> --show <path>.",
	Handler: runDeploymentRename,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "deployment delete",
	Summary: "Delete a deployment; its own instances go with it: deployment delete <name> --show <path>.",
	Handler: runDeploymentDelete,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "deployment instance reassign",
	Summary: "In-place reassign one deployment instance's mode/universe/address (identity unchanged): " +
		"deployment instance reassign <deployment-name> <instance-id> [--mode <mode>] [--universe <n>] [--address <n>] --show <path>. " +
		"An omitted --mode/--universe/--address keeps the instance's current value.",
	Handler: runDeploymentInstanceReassign,
})

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "show",
	Summary: "Inspection of a working ShowState document's logical pools and deployments.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show inspect",
	Summary: "Print a deterministic JSON summary of a ShowState document's pools and deployments: show inspect --show <path>.",
	Handler: runShowInspect,
})

// runDeploymentCreate serves the self-registered "deployment create"
// route: load the ShowState at --show, append the new inactive
// deployment, and save atomically. A duplicate deployment name is
// rejected by show.Save's whole-State validation (surfaced as
// GOLC_DEPLOYMENT_DUPLICATE_NAME inside the wrapping
// GOLC_SHOW_STATE_INVALID diagnostic).
func runDeploymentCreate(request Request) Result {
	name, showPath, err := parseDeploymentNameShowArgs("deployment create <name> --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	newDeployment, err := deployment.NewDeployment(name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Deployments = append(state.Deployments, newDeployment)

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_DEPLOYMENT_CREATED: %s (%s)\n", newDeployment.Name, newDeployment.ID)}
}

// runDeploymentActivate serves the self-registered "deployment activate"
// route: load the ShowState, mark exactly the named deployment active
// (deployment.Activate guarantees every other deployment becomes
// inactive in the same call, so two deployments are never simultaneously
// active), and save atomically.
func runDeploymentActivate(request Request) Result {
	name, showPath, err := parseDeploymentNameShowArgs("deployment activate <name> --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	activated, err := deployment.Activate(state.Deployments, name)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Deployments = activated

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_DEPLOYMENT_ACTIVATED: %s\n", name)}
}

// parseDeploymentNameShowArgs accepts exactly: a positional deployment
// name and a required "--show <path>" (both --flag value and
// --flag=value forms), rejecting anything else (GOLC_DEPLOYMENT_USAGE).
// Shared by "deployment create" and "deployment activate", which take the
// identical <name> --show <path> shape.
func parseDeploymentNameShowArgs(usage string, args []string) (name, showPath string, err error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: usage: %s", usage)
	}
	name = args[0]

	rest := args[1:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show is required; usage: %s", usage)
	}
	return name, showPath, nil
}

// deploymentByName returns the deployment in deployments whose Name
// matches name, mirroring internal/command/pool.go's poolByName.
func deploymentByName(deployments []deployment.Deployment, name string) (deployment.Deployment, bool) {
	for _, d := range deployments {
		if d.Name == name {
			return d, true
		}
	}
	return deployment.Deployment{}, false
}

// parseDeploymentRenameArgs accepts exactly two positionals (old name, new
// name) followed by a required "--show <path>" (both forms), rejecting
// anything else (GOLC_DEPLOYMENT_USAGE) -- mirrors
// internal/command/pool.go's parsePoolRenameArgs.
func parseDeploymentRenameArgs(usage string, args []string) (oldName, newName, showPath string, err error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return "", "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: usage: %s", usage)
	}
	oldName, newName = args[0], args[1]

	rest := args[2:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--show":
			if i+1 >= len(rest) {
				return "", "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", "", "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show is required; usage: %s", usage)
	}
	return oldName, newName, showPath, nil
}

// runDeploymentRename serves the self-registered "deployment rename"
// route: load the ShowState, resolve the deployment by its current name,
// rename it (ID never re-minted), replace it in place, and save. A rename
// colliding with an existing deployment's name is rejected by show.Save's
// whole-State validation (GOLC_DEPLOYMENT_DUPLICATE_NAME inside the
// wrapping GOLC_SHOW_STATE_INVALID).
func runDeploymentRename(request Request) Result {
	usage := "deployment rename <old-name> <new-name> --show <path>"
	oldName, newName, showPath, err := parseDeploymentRenameArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetDeployment, found := deploymentByName(state.Deployments, oldName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_DEPLOYMENT_NOT_FOUND: no deployment named %q exists\n", oldName)}
	}
	renamed, err := deployment.Rename(targetDeployment, newName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	for i, d := range state.Deployments {
		if d.ID == renamed.ID {
			state.Deployments[i] = renamed
			break
		}
	}

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_DEPLOYMENT_RENAMED: %s -> %s (%s)\n", oldName, renamed.Name, renamed.ID)}
}

// runDeploymentDelete serves the self-registered "deployment delete"
// route: load the ShowState, resolve the deployment by name, delete it via
// deployment.DeleteDeployment (its own instances go with it), and save. No
// explicit dangling-Selection scrub is needed here -- show.Save itself
// scrubs every Scene's Layer.Selection before validating.
func runDeploymentDelete(request Request) Result {
	usage := "deployment delete <name> --show <path>"
	name, showPath, err := parseDeploymentNameShowArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetDeployment, found := deploymentByName(state.Deployments, name)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_DEPLOYMENT_NOT_FOUND: no deployment named %q exists\n", name)}
	}

	newDeployments, err := deployment.DeleteDeployment(state.Deployments, targetDeployment.ID)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	state.Deployments = newDeployments

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_DEPLOYMENT_DELETED: %s (%s)\n", name, targetDeployment.ID)}
}

// deploymentInstanceReassignArgs is the parsed shape of one "deployment
// instance reassign" invocation. Mode/Universe/Address are all optional --
// an unset field ("" for Mode, 0 for Universe/Address) means "keep the
// instance's current value" (mirrors internal/command/pool.go's
// --start-universe/--start-address 0-means-unset convention).
type deploymentInstanceReassignArgs struct {
	deploymentName string
	instanceID     uuid.UUID
	mode           string
	universe       int
	address        int
	showPath       string
}

// parseDeploymentInstanceReassignArgs accepts two positionals (deployment
// name, instance id) followed by optional --mode/--universe/--address and
// a required --show (both forms), rejecting anything else
// (GOLC_DEPLOYMENT_USAGE).
func parseDeploymentInstanceReassignArgs(usage string, args []string) (deploymentInstanceReassignArgs, error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: usage: %s", usage)
	}
	instanceID, err := uuid.Parse(args[1])
	if err != nil {
		return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: instance id %q is not a valid UUID; usage: %s", args[1], usage)
	}
	parsed := deploymentInstanceReassignArgs{deploymentName: args[0], instanceID: instanceID}

	rest := args[2:]
	for i := 0; i < len(rest); {
		argument := rest[i]
		switch {
		case argument == "--mode":
			if i+1 >= len(rest) {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --mode requires a value; usage: %s", usage)
			}
			parsed.mode = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--mode="):
			parsed.mode = strings.TrimPrefix(argument, "--mode=")
			i++
		case argument == "--universe":
			if i+1 >= len(rest) {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --universe requires a value; usage: %s", usage)
			}
			value, err := parsePositiveInt(rest[i+1])
			if err != nil {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --universe value %q must be a positive integer; usage: %s", rest[i+1], usage)
			}
			parsed.universe = value
			i += 2
		case strings.HasPrefix(argument, "--universe="):
			raw := strings.TrimPrefix(argument, "--universe=")
			value, err := parsePositiveInt(raw)
			if err != nil {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --universe value %q must be a positive integer; usage: %s", raw, usage)
			}
			parsed.universe = value
			i++
		case argument == "--address":
			if i+1 >= len(rest) {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --address requires a value; usage: %s", usage)
			}
			value, err := parsePositiveInt(rest[i+1])
			if err != nil {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --address value %q must be a positive integer; usage: %s", rest[i+1], usage)
			}
			parsed.address = value
			i += 2
		case strings.HasPrefix(argument, "--address="):
			raw := strings.TrimPrefix(argument, "--address=")
			value, err := parsePositiveInt(raw)
			if err != nil {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --address value %q must be a positive integer; usage: %s", raw, usage)
			}
			parsed.address = value
			i++
		case argument == "--show":
			if i+1 >= len(rest) {
				return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show requires a path; usage: %s", usage)
			}
			parsed.showPath = rest[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			parsed.showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.showPath == "" {
		return deploymentInstanceReassignArgs{}, fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show is required; usage: %s", usage)
	}
	return parsed, nil
}

// runDeploymentInstanceReassign serves the self-registered "deployment
// instance reassign" route: load the ShowState, resolve the deployment and
// instance, fill any omitted --mode/--universe/--address from the
// instance's current value, call deployment.Reassign (validates the new
// address and checks for a collision against every other instance in the
// same deployment), replace the deployment in place, and save
// immediately -- no impact-plan preview, since this is one pure,
// synchronous, all-or-nothing operation on a single instance.
func runDeploymentInstanceReassign(request Request) Result {
	usage := "deployment instance reassign <deployment-name> <instance-id> [--mode <mode>] [--universe <n>] [--address <n>] --show <path>"
	parsed, err := parseDeploymentInstanceReassignArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	targetDeployment, found := deploymentByName(state.Deployments, parsed.deploymentName)
	if !found {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_DEPLOYMENT_NOT_FOUND: no deployment named %q exists\n", parsed.deploymentName)}
	}

	var current deployment.Instance
	instanceFound := false
	for _, instance := range targetDeployment.Instances {
		if instance.ID == parsed.instanceID {
			current = instance
			instanceFound = true
			break
		}
	}
	if !instanceFound {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_DEPLOYMENT_INSTANCE_NOT_FOUND: instance %s does not exist\n", parsed.instanceID)}
	}

	mode := current.Mode
	if parsed.mode != "" {
		mode = parsed.mode
	}
	universe := current.Universe
	if parsed.universe > 0 {
		universe = parsed.universe
	}
	address := current.Address
	if parsed.address > 0 {
		address = parsed.address
	}

	newInstances, err := deployment.Reassign(targetDeployment.Instances, parsed.instanceID, mode, universe, address)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	targetDeployment.Instances = newInstances
	for i, d := range state.Deployments {
		if d.ID == targetDeployment.ID {
			state.Deployments[i] = targetDeployment
			break
		}
	}

	if err := show.Save(request.Root, parsed.showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: fmt.Appendf(nil, "GOLC_DEPLOYMENT_INSTANCE_REASSIGNED: %s mode=%s universe=%d address=%d\n", parsed.instanceID, mode, universe, address)}
}

// parseShowInspectArgs accepts exactly a required "--show <path>" (both
// --flag value and --flag=value forms), rejecting anything else. It
// reuses GOLC_DEPLOYMENT_USAGE since "show inspect" is declared alongside
// "deployment create"/"deployment activate" in this file and the plan's
// diagnostic set has no distinct show-usage code.
func parseShowInspectArgs(usage string, args []string) (showPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--show":
			if i+1 >= len(args) {
				return "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show requires a path; usage: %s", usage)
			}
			showPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--show="):
			showPath = strings.TrimPrefix(argument, "--show=")
			i++
		default:
			return "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if showPath == "" {
		return "", fmt.Errorf("GOLC_DEPLOYMENT_USAGE: --show is required; usage: %s", usage)
	}
	return showPath, nil
}

// showInspectPoolView is the allowlisted JSON projection of one Pool:
// identity, name, required capabilities, and member count only -- never
// per-member fixture identity/hash detail and never a filesystem path
// (mirrors internal/command/linear.go's linearStatusEntry allowlisted
// projection discipline).
type showInspectPoolView struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	MemberCount          int      `json:"member_count"`
}

// showInspectDeploymentView is the allowlisted JSON projection of one
// Deployment: identity, name, active flag, and instance count only.
type showInspectDeploymentView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Active        bool   `json:"active"`
	InstanceCount int    `json:"instance_count"`
}

// showInspectView is the deterministic JSON envelope "show inspect"
// emits.
type showInspectView struct {
	SchemaVersion int                         `json:"schema_version"`
	Revision      int                         `json:"revision"`
	Pools         []showInspectPoolView       `json:"pools"`
	Deployments   []showInspectDeploymentView `json:"deployments"`
}

// buildShowInspectView projects state into its allowlisted view, in
// state's own deterministic (declared/append) order.
func buildShowInspectView(state show.State) showInspectView {
	view := showInspectView{
		SchemaVersion: state.SchemaVersion,
		Revision:      state.Revision,
		Pools:         make([]showInspectPoolView, 0, len(state.Pools)),
		Deployments:   make([]showInspectDeploymentView, 0, len(state.Deployments)),
	}
	for _, p := range state.Pools {
		capabilities := make([]string, 0, len(p.RequiredCapabilities))
		for _, capabilityType := range p.RequiredCapabilities {
			capabilities = append(capabilities, string(capabilityType))
		}
		view.Pools = append(view.Pools, showInspectPoolView{
			ID:                   p.ID.String(),
			Name:                 p.Name,
			RequiredCapabilities: capabilities,
			MemberCount:          len(p.Members),
		})
	}
	for _, d := range state.Deployments {
		view.Deployments = append(view.Deployments, showInspectDeploymentView{
			ID:            d.ID.String(),
			Name:          d.Name,
			Active:        d.Active,
			InstanceCount: len(d.Instances),
		})
	}
	return view
}

// runShowInspect serves the self-registered "show inspect" route: load
// the ShowState at --show (read-only -- inspect never mutates) and print
// its allowlisted, deterministic JSON projection.
func runShowInspect(request Request) Result {
	showPath, err := parseShowInspectArgs("show inspect --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, err := json.MarshalIndent(buildShowInspectView(state), "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_DEPLOYMENT_INSPECT_ENCODE_FAILED: %v\n", err)}
	}
	return Result{Stdout: append(payload, '\n')}
}
