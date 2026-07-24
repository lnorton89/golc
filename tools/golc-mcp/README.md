# golc-mcp

A local, read-only [MCP](https://modelcontextprotocol.io) server over the GOLC repository. It gives Claude Code (or any other MCP-aware client) fast, accurate answers to "where is this project right now" and "what does this config/schema/command actually say," without grepping source or re-deriving GSD planning state by hand.

## Scope

Every tool is **read-only**: `readOnlyHint: true`, no mutation, no subprocess execution, nothing beyond files already committed to the repository plus the in-process `internal/command` and `internal/projectconfig` registries. It never shells out to `golc.ps1`, never runs `build`/`test`/`check`, and never writes anything. If you want the server to also *run* things, that is a deliberate follow-up, not something this version does implicitly.

This is a standalone contributor tool, not part of the pinned bootstrap/build/test/package graph `golc.ps1` owns — same relationship `tools/linear-sync` has to the rest of the repo, just same-ecosystem (Go) instead of a separate one. Building it uses the ambient `go` toolchain directly rather than going through `golc.ps1`.

## Tools

| Tool | What it returns |
|---|---|
| `golc_project_status` | Current GSD milestone/phase/status/progress, parsed from `.planning/STATE.md` |
| `golc_list_phases` | Every roadmap phase's number/title/goal/status/plan-progress |
| `golc_get_phase_detail` | One phase's full detail section (goal, mode, deps, requirements, waves) |
| `golc_list_command_routes` | Every `golc.ps1 <route>` reachable right now, live from the CLI's own registry |
| `golc_list_test_scopes` | Every valid `test --quick --scope <name>` value (best-effort source scan) |
| `golc_list_config_concerns` | The concern/key registry: which file owns which canonical config keys |
| `golc_config_inspect` | Resolved JSON for one config concern (same output as `golc.ps1 config inspect`) |
| `golc_config_explain` | Provenance for one config key (same output as `golc.ps1 config explain`) |
| `golc_list_schemas` | Every generated schema under `schemas/*.schema.json` |
| `golc_get_schema` | Full JSON Schema for one named schema |
| `golc_list_reference_docs` | Every generated package doc under `docs/reference/*.md` |
| `golc_get_reference_doc` | Full Markdown for one package's reference doc |

## Build

```powershell
go build -o tools/golc-mcp/golc-mcp.exe ./tools/golc-mcp
```

The binary is `.exe`-suffixed and gitignored (`*.exe` is already in `.gitignore`) — build it locally, don't commit it.

## Register with Claude Code

Add to your MCP client config (e.g. `.mcp.json` at the repo root, or Claude Code's own MCP settings):

```json
{
  "mcpServers": {
    "golc": {
      "command": "C:\\path\\to\\golc\\tools\\golc-mcp\\golc-mcp.exe",
      "args": []
    }
  }
}
```

The server resolves the repository root by (1) the `GOLC_PROJECT_ROOT` environment variable if set, or (2) walking up from its working directory to the nearest `golc.project.toml`. If your MCP client launches the binary with the repo as its working directory (the common case), no extra configuration is needed. Otherwise set `GOLC_PROJECT_ROOT` explicitly in the config's `env` block.

## Design notes

- **Same Go module, not a separate one.** `tools/golc-mcp` lives inside `github.com/lnorton89/golc` rather than its own `go.mod` so it can import `internal/command` and `internal/projectconfig` directly (Go's `internal/` visibility rule allows any package rooted under the module root to import them). That means `golc_list_command_routes` and the config tools call the *actual* registries — they can't drift from the real CLI the way a hand-maintained copy would.
- **`golc_list_test_scopes` is a best-effort source scan**, not a registry call: Go quick-test scopes aren't centrally registered anywhere (`test.go` derives a marker name from a scope name at run time and checks whether it exists), and the Node scope registry (`declaredNodeScopes`) is unexported. The tool reverses `test.go`'s own `scopeTestMarker` naming convention against a scan of `*_test.go` and `MustDeclareNodeScope` call sites. Treat its output as a strong hint, and confirm with the real CLI if precision matters.
- **`.planning/STATE.md`'s YAML frontmatter is parsed with a small hand-rolled scanner**, not a general YAML library. A strict YAML 1.1 decoder coerces bare dates like `2026-07-23` into a timestamp type that then refuses to bind to a Go `string` field — exactly the kind of surprise this tool exists to avoid handing back to a caller.
