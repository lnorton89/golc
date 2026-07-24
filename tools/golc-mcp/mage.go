// mage.go serves golc_list_mage_targets from the same declarative target
// registry the executable Mage wrappers use. The PR target is enriched by
// validating and projecting config/commands.toml through LoadPRGraph; no
// target or graph step is ever executed here.
package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lnorton89/golc/internal/delivery"
)

const magePRAuthorityFile = "config/commands.toml"

var magePRAuthorityKeys = []string{
	"commands.pr.steps",
	"commands.pr.network_steps",
	"commands.pr.mutation_steps",
}

type listMageTargetsInput struct{}

type mageTargetOutput struct {
	Name      string                 `json:"name"`
	Kind      string                 `json:"kind"`
	Route     string                 `json:"route"`
	Args      []string               `json:"args"`
	Authority string                 `json:"authority"`
	PR        *magePRAuthorityOutput `json:"pr,omitempty"`
}

type magePRAuthorityOutput struct {
	AuthorityFile        string             `json:"authority_file"`
	AuthorityKeys        []string           `json:"authority_keys"`
	ConfiguredEntrypoint string             `json:"configured_entrypoint"`
	MutationPolicy       string             `json:"mutation_policy"`
	Steps                []magePRStepOutput `json:"steps"`
}

type magePRStepOutput struct {
	Name    string   `json:"name"`
	Route   string   `json:"route"`
	Args    []string `json:"args"`
	Network string   `json:"network"`
}

type listMageTargetsOutput struct {
	Targets []mageTargetOutput `json:"targets"`
}

func handleListMageTargets(_ context.Context, _ *mcp.CallToolRequest, _ listMageTargetsInput) (*mcp.CallToolResult, listMageTargetsOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[listMageTargetsOutput](err)
	}

	descriptors := delivery.MageTargets()
	out := listMageTargetsOutput{Targets: make([]mageTargetOutput, 0, len(descriptors))}
	for _, descriptor := range descriptors {
		target := mageTargetOutput{
			Name:      descriptor.Name,
			Kind:      string(descriptor.Kind),
			Route:     descriptor.Route,
			Args:      append([]string{}, descriptor.Args...),
			Authority: descriptor.Authority,
		}
		if descriptor.Kind == delivery.MageTargetKindPR {
			graph, err := delivery.LoadPRGraph(root)
			if err != nil {
				return toolError[listMageTargetsOutput](err)
			}
			pr := &magePRAuthorityOutput{
				AuthorityFile:        magePRAuthorityFile,
				AuthorityKeys:        append([]string(nil), magePRAuthorityKeys...),
				ConfiguredEntrypoint: graph.Inventory.Entrypoint,
				MutationPolicy:       "none",
				Steps:                make([]magePRStepOutput, 0, len(graph.Steps)),
			}
			for _, step := range graph.Steps {
				pr.Steps = append(pr.Steps, magePRStepOutput{
					Name:    step.Name,
					Route:   step.Route,
					Args:    append([]string{}, step.Args...),
					Network: step.Network.String(),
				})
			}
			target.PR = pr
		}
		out.Targets = append(out.Targets, target)
	}
	return nil, out, nil
}
