---
phase: quick-improve-golc-mcp-status-and-mage-introspection
plan: 260723-ule
type: execute
wave: 1
depends_on: [260723-u0p]
files_modified:
  - tools/golc-mcp/status.go
  - tools/golc-mcp/status_test.go
  - internal/delivery/mage_targets.go
  - internal/delivery/delivery_test.go
  - magefiles/magefile.go
  - magefiles/magefile_test.go
  - tools/golc-mcp/mage.go
  - tools/golc-mcp/main.go
  - tools/golc-mcp/protocol_test.go
  - tools/golc-mcp/README.md
autonomous: true
requirements: [CONF-01, CONF-02, CONF-03]
must_haves:
  truths:
    - "golc_project_status reports the exact Last activity line from STATE.md's Current Position section, not a stale frontmatter value."
    - "The status response preserves its existing last_activity and last_activity_desc fields while naming their body source and reporting any body/frontmatter disagreement as structured drift."
    - "A missing, malformed, or ambiguous Current Position Last activity line fails loudly instead of silently falling back to stale metadata."
    - "golc_list_mage_targets returns all ten current Mage targets with deterministic kind, route, argument, and authority metadata."
    - "The Mage executor and MCP introspection tool consume one shared target descriptor registry; neither owns a second target-to-route mapping."
    - "The Pr target response exposes the current config/commands.toml authority, configured entrypoint, ordered route/argument steps, network policy, and validated mutation-none policy without executing the graph."
    - "Every MCP tool remains read-only, idempotent, closed-world, subprocess-free, and unable to invoke bootstrap, command handlers, Mage targets, delivery.Run, builds, tests, or checks."
    - "MCP-facing descriptions and documentation describe registered routes and the configured contributor entrypoint without treating golc.ps1 as permanent; current PowerShell compatibility remains accurately documented."
    - "No command-parity rewrite, PowerShell deletion/rename, workflow edit, launcher replacement, root README migration, or Step 6-8 implementation is introduced."
  artifacts:
    - path: tools/golc-mcp/status.go
      provides: "Deterministic Current Position activity parsing, compatibility fields, source label, and structured drift report"
    - path: internal/delivery/mage_targets.go
      provides: "Single deterministic Mage target descriptor registry shared by execution and introspection"
      exports: ["MageTargets", "LookupMageTarget"]
    - path: magefiles/magefile.go
      provides: "The same ten Mage exports, now dispatched through shared target descriptors"
    - path: tools/golc-mcp/mage.go
      provides: "Read-only golc_list_mage_targets handler enriched with the live configured PR graph"
    - path: tools/golc-mcp/protocol_test.go
      provides: "In-memory MCP list/call coverage and read-only/no-execution guardrails"
    - path: tools/golc-mcp/README.md
      provides: "Migration-safe MCP tool and build/registration documentation"
  key_links:
    - from: .planning/STATE.md
      to: tools/golc-mcp/status.go
      via: "the exact ## Current Position section supplies the reported Last activity date/description; frontmatter is retained only as the compared metadata source"
      pattern: "Current Position|Last activity|last_activity_drift"
    - from: internal/delivery/mage_targets.go
      to: magefiles/magefile.go
      via: "each exported Mage wrapper resolves its shared descriptor before route/bootstrap/PR dispatch"
      pattern: "LookupMageTarget|MageTargets"
    - from: internal/delivery/mage_targets.go
      to: tools/golc-mcp/mage.go
      via: "golc_list_mage_targets projects the same defensive, deterministic descriptor snapshot"
      pattern: "MageTargets"
    - from: config/commands.toml
      to: tools/golc-mcp/mage.go
      via: "delivery.LoadPRGraph parses commands.pr.steps/network_steps/mutation_steps and command inventory for read-only reporting"
      pattern: "LoadPRGraph|commands\\.pr"
    - from: tools/golc-mcp/main.go
      to: tools/golc-mcp/protocol_test.go
      via: "registerTools is exercised through mcp.NewInMemoryTransports, ListTools, and CallTool"
      pattern: "registerTools|NewInMemoryTransports|CallTool"
---

<objective>
Make the local read-only GOLC MCP server trustworthy during the PowerShell-to-Mage migration by reporting live GSD activity and exposing the actual Mage command surface without duplicating execution authority.

Purpose: Let an MCP client determine current project activity and the exact Mage/PR route contract needed for later migration steps while preserving the server's strict no-execution boundary.
Output: Freshness-aware status output, a shared Mage target registry, a new golc_list_mage_targets tool, in-process/protocol regression tests, and migration-safe MCP descriptions/documentation.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@README.md
@.mcp.json
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/quick/260723-u0p-add-mage-targets-and-pin-mage-toolchain-/260723-u0p-SUMMARY.md
@config/commands.toml
@internal/projectconfig/model.go
@internal/delivery/graph.go
@internal/delivery/delivery_test.go
@magefiles/magefile.go
@magefiles/magefile_test.go
@tools/golc-mcp/main.go
@tools/golc-mcp/status.go
@tools/golc-mcp/commands.go
@tools/golc-mcp/config.go
@tools/golc-mcp/docs.go
@tools/golc-mcp/repo.go
@tools/golc-mcp/README.md

<interfaces>
Existing status contract in tools/golc-mcp/status.go:
- projectStatusOutput retains Milestone, MilestoneName, CurrentPhase, CurrentPhaseName, Status, StoppedAt, LastUpdated, LastActivity, LastActivityDesc, and Progress.
- handleProjectStatus resolves the repository, reads .planning/STATE.md, parses YAML frontmatter, and returns typed structured content.

Existing delivery contracts in internal/delivery/graph.go:
- type Step struct { Name string; Route string; Args []string; Network NetworkPolicy }
- type Graph struct { Root string; Inventory CommandInventory; Steps []Step }
- func LoadPRGraph(root string) (Graph, error)
- NetworkPolicy.String() returns "allowed" or "denied".
- A successful LoadPRGraph proves commands.pr.mutation_steps is exactly none and validates graph parity without executing it.

Existing Mage contracts in magefiles/magefile.go:
- Bootstrap and Pr accept context.Context.
- Generate, GenerateCheck, Check, CheckOffline, Build, Test, Package, and PackageFoundation return error with no arguments.
- targetRuntime is the package-private injected execution seam; shared descriptors must not remove it.

Official MCP Go SDK v1.6.1 test contracts:
- mcp.NewInMemoryTransports connects mcp.Server and mcp.Client sessions without a subprocess.
- ClientSession.ListTools enumerates the registered tool schema/annotations.
- ClientSession.CallTool invokes a registered tool and returns structured content.
</interfaces>

**Observed defect:** `.planning/STATE.md` frontmatter currently says quick task `260723-rym`, while `## Current Position` says quick task `260723-u0p`. GSD quick completion updates the body line, so frontmatter-only parsing silently reports stale activity.

**Dirty-worktree constraint:** Preserve the untracked `.planning/quick/260723-uj9-fold-the-sketch-findings-skill-into-plan/` work and any unrelated changes that appear during execution. Before each task, inspect `git status --short` and the exact task-file diffs; never reset, replace, stage, format, or commit unrelated work.

<locked_decisions>
- D-01: Current Position body activity is the reported live value; frontmatter activity is comparison metadata, never silent precedence.
- D-02: Preserve existing status compatibility fields and add explicit source/drift data.
- D-03: Mage introspection comes from the actual mapping used by magefiles through a shared Go registry, not source scraping or an MCP-owned list.
- D-04: The Mage tool includes exact route arguments and read-only PR authority metadata useful to later Steps 6-8.
- D-05: MCP remains read-only and never runs commands, Mage targets, bootstrap, builds, tests, checks, delivery graphs, or subprocesses.
- D-06: This task improves introspection only; Step 6 parity/deletion, Step 7 launcher work, Step 8 CI work, root README migration, and .mcp.json changes remain out of scope.
</locked_decisions>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Report authoritative body activity and explicit frontmatter drift</name>
  <files>tools/golc-mcp/status.go, tools/golc-mcp/status_test.go</files>
  <read_first>
    - tools/golc-mcp/status.go
    - .planning/STATE.md
    - tools/golc-mcp/errors.go
  </read_first>
  <behavior>
    - Test 1: A STATE fixture whose frontmatter names task A and Current Position names task B returns B in existing last_activity fields, source current_position_body, and drift detected with both source values preserved.
    - Test 2: Matching frontmatter/body activity returns the body value with drift detected false.
    - Test 3: LF and CRLF fixtures parse to byte-equivalent structured activity.
    - Test 4: Missing Current Position, zero matching Last activity lines, multiple matching lines, malformed date/separator, or an empty description returns a named tool error and never falls back to frontmatter.
    - Test 5: Existing milestone/phase/status/progress/frontmatter behavior remains unchanged.
  </behavior>
  <action>RED per D-01/D-02: create table-driven pure parser/handler tests from synthetic STATE.md strings before changing production code. Specify one exact `## Current Position` section and one exact `Last activity: YYYY-MM-DD — description` record; cover the observed stale-frontmatter/current-body pair and malformed/ambiguous cases. Confirm the focused test fails because status.go reads activity only from frontmatter.

GREEN: extract the Current Position section without scanning later sections, parse exactly one activity line, validate its date and non-empty description, and make that body record populate the existing LastActivity/LastActivityDesc JSON fields. Add an always-present source identifier and a structured drift object containing `detected`, the frontmatter record, and the Current Position record. Keep every unrelated frontmatter field and progress parser compatible. Fail with a stable `.planning/STATE.md` diagnostic when the body contract is missing or ambiguous; do not silently choose the first match or fall back to frontmatter. Use only the standard library and keep the handler read-only.</action>
  <verify>
    <automated>go test ./tools/golc-mcp -run '^(TestProjectStatus|TestStatus)' -count=1</automated>
  </verify>
  <done>The status tool reports the live body activity through its compatibility fields, identifies that source, exposes exact frontmatter drift, and rejects ambiguous/missing body authority deterministically.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Make Mage target descriptors a shared execution/introspection authority</name>
  <files>internal/delivery/mage_targets.go, internal/delivery/delivery_test.go, magefiles/magefile.go, magefiles/magefile_test.go</files>
  <read_first>
    - internal/delivery/graph.go
    - internal/delivery/delivery_test.go
    - magefiles/magefile.go
    - magefiles/magefile_test.go
    - config/commands.toml
  </read_first>
  <behavior>
    - Test 1: MageTargets returns exactly bootstrap, build, check, checkoffline, generate, generatecheck, package, packagefoundation, pr, and test in deterministic name order.
    - Test 2: Route descriptors carry the exact current route/argument vectors; bootstrap and pr carry typed special-operation kinds and explicit authority labels.
    - Test 3: Returned target and argument slices are defensive copies and lookup is deterministic for the Mage CLI target name.
    - Test 4: Every exported Mage function dispatches through its shared descriptor while preserving existing root, output, failure, bootstrap, configured PR order, and stop-on-first-failure behavior.
    - Test 5: The Mage file retains exactly the ten public exports and contains no independent route/argument table.
  </behavior>
  <action>RED per D-03/D-04: extend external delivery tests with the exact target descriptor inventory and defensive-copy expectations. Refine magefile tests so the exported wrappers are proven to dispatch according to those descriptors, including the package/packagefoundation alias and special bootstrap/pr operations. Add an AST assertion that wrappers delegate to the shared lookup/dispatch helper rather than embedding route argument vectors. Confirm focused tests fail because the shared registry does not exist and magefile owns its mappings.

GREEN: add `internal/delivery/mage_targets.go` with a typed target kind, immutable-by-copy descriptor (`Name`, `Kind`, `Route`, `Args`, `Authority`), deterministic `MageTargets`, and exact `LookupMageTarget`. Declare all ten mappings once there; route-backed targets name their self-registered route and arguments, Bootstrap names the bootstrap Go API authority, and Pr names `config/commands.toml` plus the `commands.pr.*` authority. Refactor magefile's package-private dispatcher to resolve these descriptors before using its existing injected runtime. Preserve all exported signatures and execution semantics, and preserve delivery's import direction: the registry is declarative and must not import internal/command or execute anything.</action>
  <verify>
    <automated>go test ./internal/delivery ./magefiles -count=1</automated>
  </verify>
  <done>One shared, defensive, deterministic registry owns every Mage target mapping, and the real Mage exports execute through that registry with no behavior or authority drift.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Expose Mage metadata through the read-only MCP protocol and update migration wording</name>
  <files>tools/golc-mcp/mage.go, tools/golc-mcp/main.go, tools/golc-mcp/protocol_test.go, tools/golc-mcp/README.md</files>
  <read_first>
    - tools/golc-mcp/main.go
    - tools/golc-mcp/errors.go
    - tools/golc-mcp/repo.go
    - tools/golc-mcp/commands.go
    - tools/golc-mcp/status.go
    - internal/delivery/mage_targets.go
    - internal/delivery/graph.go
    - config/commands.toml
    - tools/golc-mcp/README.md
    - .mcp.json
  </read_first>
  <behavior>
    - Test 1: An in-memory MCP client lists golc_list_mage_targets with read-only/idempotent/closed-world annotations and the server's existing tools remain registered.
    - Test 2: Calling golc_list_mage_targets against a temporary strict commands fixture returns the exact ten descriptors and PR authority file/keys, current configured entrypoint, ordered steps, route arguments, allowed/denied network values, and mutation policy none.
    - Test 3: Calling golc_project_status over the same protocol returns the authoritative body activity plus drift payload from Task 1.
    - Test 4: MCP production sources register no mutating tool and call no process, Mage execution, command Execute, bootstrap, delivery.Run, build, test, or check path.
    - Test 5: Tool descriptions no longer present golc.ps1 as the permanent API identity; current configured-entrypoint compatibility remains clear.
  </behavior>
  <action>RED per D-04/D-05/D-06: build a temporary repository fixture containing STATE.md and the strict commands concern, connect a client/server pair with `mcp.NewInMemoryTransports`, and test ListTools plus CallTool for status and Mage metadata. Add AST-based production-source guardrails for forbidden execution/process calls so the test measures code structure rather than comments. Confirm failure because the Mage tool is absent and the existing descriptions hard-code the transitional entrypoint.

GREEN: implement `golc_list_mage_targets` as a zero-input typed handler. Project `delivery.MageTargets()` into deterministic JSON and call only `delivery.LoadPRGraph(root)` to enrich the Pr record with `config/commands.toml`, the three `commands.pr.*` keys, current configured entrypoint, validated mutation policy `none`, and ordered step name/route/args/network data. Never call `delivery.Run`, `command.Execute`, bootstrap, Mage functions, or a subprocess. Register the tool with the shared readOnly annotations and bump the MCP server version for the additive tool/output contract.

Update public descriptions in main.go and tools/golc-mcp/README.md to describe route-native operations and the configured contributor entrypoint, noting that golc.ps1 is the currently retained compatibility entrypoint during migration rather than the MCP's identity or permanent owner. Document the new Mage tool, status source/drift fields, in-memory test boundary, existing binary path in `.mcp.json`, and build command. Do not edit root README.md, `.mcp.json`, config/commands.toml, workflows, parity tests, or any launcher per D-06.</action>
  <verify>
    <automated>go test ./tools/golc-mcp ./internal/delivery ./magefiles -count=1</automated>
  </verify>
  <done>An MCP client can inspect fresh project activity and the real Mage/PR target contract over an additive read-only protocol, with descriptions ready for later entrypoint migration and structural proof that no command can run.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| `.planning/STATE.md` to MCP structured status | Repository Markdown controls the activity text an MCP client trusts for session pickup. |
| Shared target descriptors to Mage and MCP | One declarative mapping feeds both executable dispatch and read-only introspection; drift here would mislead later migration work. |
| `config/commands.toml` to MCP PR metadata | Strict committed PR strings become reported ordered steps and policy metadata, but must never become executed actions. |
| MCP client to local repository reader | Untrusted tool requests may select tools, but every handler is limited to fixed repository files/in-process registries and returns data only. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-ule-01 | Spoofing / Repudiation | `golc_project_status` activity source | medium | mitigate | Parse exactly one record inside the exact Current Position section, label the reported source, expose both compared records and drift, and fail on missing/ambiguous body state. |
| T-ule-02 | Tampering | Mage target mapping | high | mitigate | Make delivery.MageTargets the sole descriptor registry, dispatch real Mage wrappers through it, return defensive sorted copies, and AST-test against a second magefile mapping. |
| T-ule-03 | Elevation of Privilege | `golc_list_mage_targets` | high | mitigate | Permit only descriptor projection and LoadPRGraph parsing; AST/protocol tests reject process APIs, command execution, bootstrap, Mage invocation, delivery.Run, and non-read-only annotations. |
| T-ule-04 | Tampering / Repudiation | PR authority reporting | high | mitigate | Use strict projectconfig-backed LoadPRGraph, report the authority file/keys and current configured entrypoint, preserve config order, expose per-step network policy, and report mutation none only after validation succeeds. |
| T-ule-05 | Information Disclosure | MCP errors/metadata | low | mitigate | Return repository-relative authority/source labels and existing allowlisted route/config metadata only; do not expose environment values, credentials, absolute roots, or file content outside fixed inputs. |
| T-ule-06 | Denial of Service | STATE/config parsers | low | accept | Inputs are bounded local planning/config files and parsing is linear; malformed data fails the individual tool call without affecting playback or repository state. |
</threat_model>

<source_audit>

| SOURCE | ID | Feature/Requirement | Plan | Status | Notes |
|--------|----|---------------------|------|--------|-------|
| GOAL | — | Improve the local read-only MCP server during PowerShell-to-Mage migration | 260723-ule | COVERED | Tasks 1-3 improve status, shared target introspection, protocol tests, and docs. |
| REQ | CONF-01 | One discoverable root configuration/entrypoint contract | 260723-ule | COVERED | Mage/PR metadata reports the configured entrypoint and exact target authorities. |
| REQ | CONF-02 | No duplicated authoritative configuration values | 260723-ule | COVERED | Task 2 creates one target descriptor registry; Task 3 consumes LoadPRGraph rather than re-parsing PR policy. |
| REQ | CONF-03 | Contributor and CI commands remain consistent | 260723-ule | COVERED | The MCP exposes the same Mage route mappings and config-ordered PR graph used by real execution. |
| RESEARCH | MCP Go SDK | Use NewInMemoryTransports, ListTools, and CallTool for protocol-level tests | 260723-ule | COVERED | Task 3 uses the official v1.6.1 APIs already pinned in go.mod. |
| CONTEXT | D-01 | Report live Current Position activity | 260723-ule | COVERED | Task 1 parses and promotes the exact body record. |
| CONTEXT | D-02 | Expose activity source/drift without breaking existing fields | 260723-ule | COVERED | Task 1 keeps compatibility fields and adds source/comparison output. |
| CONTEXT | D-03 | Mage tool derives from actual mapping/shared registry | 260723-ule | COVERED | Task 2 moves execution mapping to delivery.MageTargets; Task 3 projects it. |
| CONTEXT | D-04 | Include route args and PR-authority metadata useful for Steps 6-8 | 260723-ule | COVERED | Task 3 reports exact target descriptors and the validated configured graph. |
| CONTEXT | D-05 | Do not add mutating tools or run commands from MCP | 260723-ule | COVERED | Task 3 structural/protocol guards enforce metadata-only behavior. |
| CONTEXT | D-06 | Do not implement Step 6 parity/deletion or later migration steps | 260723-ule | COVERED | Root docs, config, parity, workflows, launchers, and .mcp.json are explicit exclusions. |
| CONTEXT | — | Update MCP descriptions/docs without assuming permanent PowerShell | 260723-ule | COVERED | Task 3 updates main.go descriptions and tools/golc-mcp/README.md while documenting current compatibility. |
| CONTEXT | — | Add in-process/protocol tests and build verification | 260723-ule | COVERED | Task 3 adds in-memory MCP coverage; overall verification compiles the server binary. |
| CONTEXT | — | Preserve unrelated work | 260723-ule | COVERED | Dirty-worktree rule protects the current untracked quick task and future unrelated changes. |

</source_audit>

<verification>
- Run `go test ./tools/golc-mcp ./internal/delivery ./magefiles -count=1`.
- Run `go test ./... -count=1` to compile and exercise every existing package against the shared descriptor refactor.
- Build the MCP executable to a task-specific path outside the repository with `$out = Join-Path $env:TEMP 'golc-mcp-260723-ule.exe'; go build -o $out ./tools/golc-mcp`; remove only that exact temporary artifact after the build result is recorded.
- Through the in-memory protocol test, assert every listed GOLC MCP tool carries `readOnlyHint: true`, `idempotentHint: true`, and `openWorldHint: false`; call status and Mage tools and compare their structured content to synthetic fixture bytes.
- Inspect `git diff -- README.md .mcp.json config/commands.toml golc.ps1 .github tests/acceptance`; these Step 6+ and compatibility files must have no executor-authored changes.
- Inspect `git diff -- .planning/quick/260723-uj9-fold-the-sketch-findings-skill-into-plan/`; it must have no executor-authored changes.
</verification>

<success_criteria>
- Status activity always reflects the exact Current Position body record and exposes any frontmatter disagreement.
- All ten Mage targets and their exact route/API authorities come from one shared registry used by actual Mage execution.
- The MCP reports current PR order/policy/configured entrypoint without executing any step.
- MCP protocol and structural tests prove additive tool registration, structured responses, and the no-execution/read-only boundary.
- MCP descriptions/documentation are migration-safe while retained PowerShell compatibility remains intact.
- The full Go suite and a standalone golc-mcp build pass.
- Step 6-8 files/behavior and unrelated work remain untouched.
</success_criteria>

<output>
Create `.planning/quick/260723-ule-improve-golc-mcp-status-freshness-and-ex/260723-ule-SUMMARY.md` when done.
</output>
