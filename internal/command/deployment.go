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
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_DEPLOYMENT_CREATED: %s (%s)\n", newDeployment.Name, newDeployment.ID))}
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
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_DEPLOYMENT_ACTIVATED: %s\n", name))}
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
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_DEPLOYMENT_INSPECT_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}
