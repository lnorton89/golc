// config.go serves three read-only views over GOLC's layered configuration
// system, all backed directly by internal/projectconfig's exported API
// (no subprocess, no invocation of the pinned CLI binary): the static
// concern/key registry (DefaultSpec), one concern's resolved JSON
// (InspectConcern — identical output to "golc.ps1 config inspect"), and
// one key's provenance (Explain — identical output to
// "golc.ps1 config explain").
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lnorton89/golc/internal/projectconfig"
)

type listConfigConcernsInput struct{}

type configConcernSummary struct {
	ID   string   `json:"id"`
	Path string   `json:"path"`
	Keys []string `json:"keys"`
}

type listConfigConcernsOutput struct {
	Concerns []configConcernSummary `json:"concerns"`
}

func handleListConfigConcerns(_ context.Context, _ *mcp.CallToolRequest, _ listConfigConcernsInput) (*mcp.CallToolResult, listConfigConcernsOutput, error) {
	spec := projectconfig.DefaultSpec()

	var out listConfigConcernsOutput
	for _, concern := range spec.Concerns {
		keys := make([]string, 0, len(concern.Keys))
		for key := range concern.Keys {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.Concerns = append(out.Concerns, configConcernSummary{ID: concern.ID, Path: concern.Path, Keys: keys})
	}
	sort.Slice(out.Concerns, func(i, j int) bool { return out.Concerns[i].ID < out.Concerns[j].ID })

	return nil, out, nil
}

type configInspectInput struct {
	Concern string `json:"concern" jsonschema:"concern id to inspect, e.g. \"runtime\" or \"toolchain\" (see golc_list_config_concerns for the full set)"`
}

type configInspectOutput struct {
	Concern string `json:"concern"`
	Values  any    `json:"values"`
}

func handleConfigInspect(_ context.Context, _ *mcp.CallToolRequest, input configInspectInput) (*mcp.CallToolResult, configInspectOutput, error) {
	if input.Concern == "" {
		return toolError[configInspectOutput](fmt.Errorf("concern is required, e.g. \"runtime\"; call golc_list_config_concerns for the full set"))
	}
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[configInspectOutput](err)
	}
	payload, err := projectconfig.InspectConcern(root, input.Concern)
	if err != nil {
		return toolError[configInspectOutput](fmt.Errorf("inspect concern %q: %w", input.Concern, err))
	}
	var values any
	if err := json.Unmarshal(payload, &values); err != nil {
		return toolError[configInspectOutput](fmt.Errorf("inspect concern %q: parse JSON: %w", input.Concern, err))
	}
	return nil, configInspectOutput{Concern: input.Concern, Values: values}, nil
}

type configExplainInput struct {
	Key string `json:"key" jsonschema:"canonical dotted config key to explain, e.g. \"runtime.log_level\""`
}

type configExplainOutput struct {
	Key        string `json:"key"`
	Provenance any    `json:"provenance"`
}

func handleConfigExplain(_ context.Context, _ *mcp.CallToolRequest, input configExplainInput) (*mcp.CallToolResult, configExplainOutput, error) {
	if input.Key == "" {
		return toolError[configExplainOutput](fmt.Errorf("key is required, e.g. \"runtime.log_level\""))
	}
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[configExplainOutput](err)
	}
	payload, err := projectconfig.Explain(root, input.Key)
	if err != nil {
		return toolError[configExplainOutput](fmt.Errorf("explain key %q: %w", input.Key, err))
	}
	var provenance any
	if err := json.Unmarshal(payload, &provenance); err != nil {
		return toolError[configExplainOutput](fmt.Errorf("explain key %q: parse JSON: %w", input.Key, err))
	}
	return nil, configExplainOutput{Key: input.Key, Provenance: provenance}, nil
}
