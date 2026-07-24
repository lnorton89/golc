// Command golc-mcp is a local, read-only MCP server over the GOLC
// repository: it gives an MCP client (Claude Code or any other MCP-aware
// tool) fast, accurate answers to "where is this project right now" and
// "what does this config/schema/command actually say" without grepping
// source or re-deriving GSD planning state by hand.
//
// Every tool is read-only (readOnlyHint: true) and touches only files
// already committed to the repository or the in-process
// internal/command and internal/projectconfig registries — it never
// shells out, never mutates state, and never runs build/test/check. It
// speaks MCP over stdio, matching how Claude Code and most local MCP
// clients launch a server.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverVersion = "0.1.0"

func main() {
	if err := run(); err != nil {
		log.Fatalf("golc-mcp: %v", err)
	}
}

func run() error {
	server := mcp.NewServer(&mcp.Implementation{Name: "golc-mcp", Version: serverVersion}, nil)

	registerTools(server)

	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("run stdio transport: %w", err)
	}
	return nil
}

// readOnly is the shared annotation set every golc-mcp tool declares:
// none of them modify the repository, none write outside their own
// return value, and none reach beyond the local checkout.
func readOnly() *mcp.ToolAnnotations {
	openWorld := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint:   true,
		IdempotentHint: true,
		OpenWorldHint:  &openWorld,
	}
}

func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_project_status",
		Description: "Current GSD planning position for the GOLC repository: active milestone, current phase " +
			"number/name, execution status, and completed/total phase and plan counts. Parsed from " +
			".planning/STATE.md's frontmatter, the single source of truth GSD tooling maintains for " +
			"\"where are we right now.\" Call this first when picking up work in a fresh session.",
		Annotations: readOnly(),
	}, handleProjectStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_phases",
		Description: "Every roadmap phase from .planning/ROADMAP.md with its number, title, one-line goal, " +
			"complete/incomplete status, completion date when finished, and plan progress (N/M) when the phase " +
			"has moved past TBD planning.",
		Annotations: readOnly(),
	}, handleListPhases)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_get_phase_detail",
		Description: "Full detail section for one roadmap phase (goal, mode, dependencies, requirements, and " +
			"plan waves) as verbatim Markdown from .planning/ROADMAP.md. Use golc_list_phases first to find the " +
			"phase number.",
		Annotations: readOnly(),
	}, handleGetPhaseDetail)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_command_routes",
		Description: "Every \"golc.ps1 <route>\" command currently reachable, read live from " +
			"internal/command's self-registration registry (the same one the golc-project CLI builds at " +
			"startup) rather than a hand-maintained copy — always in sync with the real CLI surface.",
		Annotations: readOnly(),
	}, handleListCommandRoutes)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_test_scopes",
		Description: "Every valid value for \"golc.ps1 test --quick --scope <name>\": Go scopes (derived from " +
			"TestScope{PascalName} marker functions found in *_test.go) and Node scopes (from " +
			"MustDeclareNodeScope registrations). This is a source scan mirroring test.go's own resolution " +
			"logic, not a live registry call — treat it as best-effort and confirm with the actual CLI if " +
			"precision matters.",
		Annotations: readOnly(),
	}, handleListTestScopes)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_config_concerns",
		Description: "The static configuration concern/key registry (internal/projectconfig's DefaultSpec): " +
			"every concern id, its owning file under config/, and the canonical keys it alone owns. Call this " +
			"before golc_config_inspect or golc_config_explain to find valid concern ids and keys.",
		Annotations: readOnly(),
	}, handleListConfigConcerns)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_config_inspect",
		Description: "Resolved JSON for one configuration concern (e.g. \"runtime\", \"toolchain\"), identical " +
			"to \"golc.ps1 config inspect <concern> --format json\" but called directly through " +
			"internal/projectconfig — no subprocess. See golc_list_config_concerns for valid concern ids.",
		Annotations: readOnly(),
	}, handleConfigInspect)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_config_explain",
		Description: "Deterministic provenance for one canonical config key (which layer/file wins and why), " +
			"identical to \"golc.ps1 config explain <key> --format json\" but called directly through " +
			"internal/projectconfig — no subprocess. See golc_list_config_concerns for valid keys.",
		Annotations: readOnly(),
	}, handleConfigExplain)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_schemas",
		Description: "Every generated JSON Schema under schemas/*.schema.json (fixture format, config " +
			"concerns, Linear map/plan/report shapes) with its file path and generator comment/$id.",
		Annotations: readOnly(),
	}, handleListSchemas)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "golc_get_schema",
		Description: "The full JSON Schema document for one named schema (e.g. \"fixture\", \"config-runtime\"). See golc_list_schemas for valid names.",
		Annotations: readOnly(),
	}, handleGetSchema)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_list_reference_docs",
		Description: "Every generated internal-package reference page under docs/reference/*.md (produced by " +
			"\"golc.ps1 docs\" from Go doc comments), with package name and title. Reading these is cheaper " +
			"than reading a package's source when you just need its documented contract.",
		Annotations: readOnly(),
	}, handleListReferenceDocs)

	mcp.AddTool(server, &mcp.Tool{
		Name: "golc_get_reference_doc",
		Description: "The full generated Markdown reference page for one internal package (e.g. \"command\", " +
			"\"projectconfig\", \"artnet\"). See golc_list_reference_docs for valid package names.",
		Annotations: readOnly(),
	}, handleGetReferenceDoc)
}
