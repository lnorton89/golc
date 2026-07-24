# golc-mcp

A local, read-only [MCP](https://modelcontextprotocol.io) server over the GOLC repository. It gives MCP-aware clients fast, accurate answers to “where is this project right now?” and “what does this config, schema, command, or delivery target actually say?” without grepping source or re-deriving project state.

## Scope

Every tool is read-only and closed-world: `readOnlyHint: true`, `idempotentHint: true`, `openWorldHint: false`, no mutation, and no subprocess execution. Tools read fixed repository files or in-process registries. The Mage inventory projects `internal/delivery.MageTargets` and validates PR metadata with `delivery.LoadPRGraph`; it never runs a target or graph step.

The contributor entrypoint is configured in `config/commands.toml`. `golc.ps1` is the currently retained compatibility entrypoint during the PowerShell-to-Mage migration; it is not the MCP server’s identity or a permanent owner of the route API. This server remains a standalone contributor tool outside the configured bootstrap/build/test/package graph. Building it uses the ambient Go toolchain directly.

## Tools

| Tool | What it returns |
|---|---|
| `golc_project_status` | Current GSD milestone/phase/status/progress plus authoritative Current Position activity, source, and frontmatter drift |
| `golc_list_phases` | Every roadmap phase’s number/title/goal/status/plan-progress |
| `golc_get_phase_detail` | One phase’s full detail section (goal, mode, deps, requirements, waves) |
| `golc_list_command_routes` | Every route reachable right now, live from the command API’s own registry |
| `golc_list_mage_targets` | All ten shared Mage descriptors; Bootstrap reports `GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC=1` as the pinned Node/Linear tooling prerequisite, and the PR record includes configured entrypoint, authority keys, ordered steps, arguments, network policy, and mutation policy |
| `golc_list_test_scopes` | Every valid `test --quick --scope <name>` value (best-effort source scan) |
| `golc_list_config_concerns` | The concern/key registry: which file owns which canonical config keys |
| `golc_config_inspect` | Resolved JSON for one config concern |
| `golc_config_explain` | Provenance for one config key |
| `golc_list_schemas` | Every generated schema under `schemas/*.schema.json` |
| `golc_get_schema` | Full JSON Schema for one named schema |
| `golc_list_reference_docs` | Every generated package doc under `docs/reference/*.md` |
| `golc_get_reference_doc` | Full Markdown for one package’s reference doc |

## Build

```powershell
go build -o tools/golc-mcp/golc-mcp.exe ./tools/golc-mcp
```

The binary is `.exe`-suffixed and gitignored (`*.exe` is already in `.gitignore`): build it locally and do not commit it. The committed `.mcp.json` currently launches this exact `tools/golc-mcp/golc-mcp.exe` path.

## Register with an MCP client

The repository’s existing `.mcp.json` already registers the binary:

```json
{
  "mcpServers": {
    "golc": {
      "command": "${CLAUDE_PROJECT_DIR}/tools/golc-mcp/golc-mcp.exe",
      "args": []
    }
  }
}
```

For another checkout or client, use the equivalent absolute binary path. The server resolves the repository root by checking `GOLC_PROJECT_ROOT` first, then walking up from its working directory to the nearest `golc.project.toml`.

## Design notes

- **Same Go module, not a separate one.** `tools/golc-mcp` lives inside `github.com/lnorton89/golc` so it can import repository-owned `internal/` packages directly.
- **Mage metadata has one authority.** `internal/delivery.MageTargets` is the registry used by the exported Mage wrappers and by `golc_list_mage_targets`; the MCP does not own a duplicate target or environment-option list. The Bootstrap option requests checksum-pinned project-local Node for the isolated Linear tooling workspace; it never discovers or falls back to host Node. PR enrichment comes from the validated `config/commands.toml` graph and reports repository-relative authority labels only.
- **`golc_list_test_scopes` is a best-effort source scan.** Go quick-test scopes are derived from marker names and the Node registry is unexported, so callers should confirm precision through the configured contributor entrypoint.
- **Status activity is freshness-aware.** Scalar state still comes from `.planning/STATE.md` frontmatter, but the exact `Last activity: YYYY-MM-DD — description` record in `## Current Position` is authoritative. The response labels that source and returns both records plus a drift flag; missing, malformed, or ambiguous body authority fails instead of falling back.
- **Protocol coverage is in-process.** Tests connect a real MCP client and server with `mcp.NewInMemoryTransports`, enumerate annotations, and call both status and Mage tools. AST guardrails separately reject execution/process paths in production MCP sources.
