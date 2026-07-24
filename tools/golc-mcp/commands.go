// commands.go serves golc_list_command_routes: the exact set of
// "golc.ps1 <route>" invocations reachable right now, read straight from
// internal/command's self-registration registry (the same one
// cmd/golc-project builds at startup) rather than a hand-maintained copy
// that would drift from the real CLI surface.
package main

import (
	"context"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lnorton89/golc/internal/command"
)

type listCommandRoutesInput struct{}

type commandScope struct {
	Scope   string `json:"scope"`
	Summary string `json:"summary"`
}

type commandRoute struct {
	Route   string `json:"route"`
	Summary string `json:"summary"`
}

type listCommandRoutesOutput struct {
	Scopes []commandScope `json:"scopes"`
	Routes []commandRoute `json:"routes"`
}

func handleListCommandRoutes(_ context.Context, _ *mcp.CallToolRequest, _ listCommandRoutesInput) (*mcp.CallToolResult, listCommandRoutesOutput, error) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return toolError[listCommandRoutesOutput](err)
	}

	var out listCommandRoutesOutput
	for _, scope := range registry.Scopes() {
		out.Scopes = append(out.Scopes, commandScope{Scope: scope.Scope, Summary: scope.Summary})
	}
	for _, route := range registry.Routes() {
		out.Routes = append(out.Routes, commandRoute{Route: route.Route, Summary: route.Summary})
	}
	sort.Slice(out.Scopes, func(i, j int) bool { return out.Scopes[i].Scope < out.Scopes[j].Scope })
	sort.Slice(out.Routes, func(i, j int) bool { return out.Routes[i].Route < out.Routes[j].Route })

	return nil, out, nil
}
