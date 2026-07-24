---
phase: quick-platform-paths-and-project-root
plan: 260723-tgd
type: execute
wave: 1
depends_on: []
files_modified:
  - config/commands.toml
  - golc.ps1
  - cmd/golc-project/main.go
  - cmd/golc-project/main_test.go
  - internal/bootstrap/engine.go
  - internal/bootstrap/engine_test.go
  - internal/bootstrap/engine_linear.go
  - internal/bootstrap/engine_linear_test.go
  - internal/command/build.go
  - internal/command/build_test.go
  - internal/command/linear.go
  - internal/command/linear_sync.go
  - internal/command/test.go
  - internal/command/tools_test.go
  - internal/delivery/delivery_test.go
  - internal/delivery/foundation.go
  - internal/delivery/graph.go
  - internal/projectconfig/strict_test.go
  - internal/trace/transport/process.go
  - internal/trace/transport/process_test.go
autonomous: true
requirements: [CONF-01, CONF-02, CONF-03]
must_haves:
  truths:
    - "Every installed Go, Node, and golc-project path derives its platform directory from bootstrap.PlatformKey instead of a hardcoded windows-amd64 segment."
    - "The committed Go and Node archive pins remain exactly the current windows-amd64 pairs; Linux and Darwin path support does not invent URLs, checksums, pins, qualification, or support claims."
    - "commands.cli_binary is a platform-neutral golc-project install root, and each executable consumer adds the runtime platform segment, bin directory, and Windows-only .exe suffix."
    - "Node consumers discover the verified archive's single extracted top-level directory by listing the install directory, then validate the OS-specific node and npm-cli.js paths beneath it."
    - "Foundation ZIP, manifest, and checksum filenames contain bootstrap.PlatformKey, with current Windows output remaining golc-foundation-windows-amd64.*."
    - "The current PowerShell entrypoint and the compiled Go CLI establish one absolute GOLC_PROJECT_ROOT, and every Go, Node, and Linear adapter child receives that exact root even when the inherited environment is absent or stale."
    - "The Windows build/test/package/Linear behavior remains unchanged while cmd/golc-project compiles for linux/amd64 and darwin/arm64 without adding platform pins."
    - "No Mage migration, command-parity rewrite, PowerShell deletion, CI edit, entrypoint replacement, documentation migration, or Step 4+ work is performed."
  artifacts:
    - path: internal/bootstrap/engine.go
      provides: "Canonical platform key, executable naming/path helpers, and platform-keyed golc-project build output"
      exports: ["PlatformKey"]
    - path: internal/bootstrap/engine_linear.go
      provides: "Verified Node install discovery independent of archive top-level directory spelling"
    - path: config/commands.toml
      provides: "Platform-neutral commands.cli_binary install-root authority"
    - path: internal/delivery/graph.go
      provides: "Runtime resolution of commands.cli_binary into the platform-specific executable inventory"
    - path: internal/delivery/foundation.go
      provides: "Platform-keyed foundation artifact paths"
    - path: internal/trace/transport/process.go
      provides: "Explicit project-root propagation into the isolated Node adapter process"
    - path: cmd/golc-project/main.go
      provides: "Resolved-root environment establishment for current and future non-PowerShell invocation"
  key_links:
    - from: config/commands.toml
      to: internal/delivery/graph.go
      via: "LoadGraph treats commands.cli_binary as an install root and resolves the runtime executable through bootstrap path helpers"
      pattern: "cli_binary|PlatformKey"
    - from: internal/bootstrap/engine.go
      to: internal/command/test.go
      via: "test and build consumers use bootstrap.PlatformKey and the shared executable suffix contract"
      pattern: "bootstrap\\.PlatformKey"
    - from: internal/bootstrap/engine_linear.go
      to: internal/command/build.go
      via: "bootstrap and command dispatch share one Node install discovery contract rather than reconstructing node-v<version>-<os>-<arch>"
      pattern: "ResolveNode|ReadDir"
    - from: internal/delivery/graph.go
      to: internal/delivery/foundation.go
      via: "the graph exposes the resolved current-platform CLI path included in the bounded foundation inventory"
      pattern: "CLIBinary"
    - from: cmd/golc-project/main.go
      to: internal/command/test.go
      via: "the resolved absolute root is set as GOLC_PROJECT_ROOT and root-aware child environments overwrite stale inherited values"
      pattern: "GOLC_PROJECT_ROOT"
    - from: internal/command/linear.go
      to: internal/trace/transport/process.go
      via: "ProcessConfig carries the request root and NewProcessClient inserts it into the exact child environment"
      pattern: "ProjectRoot|GOLC_PROJECT_ROOT"
---

<objective>
Replace Step 3's hardcoded Windows platform strings and make project-root propagation explicit without changing the committed toolchain pins or replacing PowerShell yet.

Purpose: Make the existing bootstrap, command, packaging, and Linear process seams consume one runtime platform identity and one resolved repository root, preserving identical Windows behavior while allowing the headless CLI to compile for Linux and Darwin.
Output: Shared platform executable/Node-discovery helpers, a platform-neutral CLI install-root configuration value, platform-keyed foundation artifacts, root-aware child-process environments, and focused cross-platform regression coverage.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/ROADMAP.md
@.planning/quick/260723-rq4-add-cross-platform-transport-for-interna/260723-rq4-SUMMARY.md
@.planning/quick/260723-s7n-implement-the-complete-go-bootstrap-engi/260723-s7n-SUMMARY.md
@.planning/quick/260723-svv-migrate-toolchain-configuration-and-stri/260723-svv-SUMMARY.md
@config/commands.toml
@config/toolchain.toml
@golc.ps1
@cmd/golc-project/main.go
@internal/bootstrap/engine.go
@internal/bootstrap/engine_test.go
@internal/bootstrap/engine_linear.go
@internal/bootstrap/engine_linear_test.go
@internal/command/test.go
@internal/command/build.go
@internal/command/linear_sync.go
@internal/command/linear.go
@internal/delivery/graph.go
@internal/delivery/foundation.go
@internal/delivery/delivery_test.go
@internal/projectconfig/model.go
@internal/projectconfig/strict_test.go
@internal/trace/transport/process.go
@internal/trace/transport/process_test.go

**Dirty-worktree constraint:** Preserve the modified `.planning/sketches/003-performance-workspace/index.html`, modified `.planning/sketches/004-patch-to-play-flow/index.html`, and untracked `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/` work. Before each task, inspect `git diff --` for the task's exact files and layer changes onto any newly appearing user hunk; never reset, replace, stage, or reformat unrelated work.

**Step boundary:** Complete PowerShell-removal Step 3 only. Do not add Mage, alter the command graph or parity policy, delete or rename `golc.ps1`, create its replacement launcher, edit CI/workflows, alter acceptance PowerShell scripts, change documentation, add platform qualification claims, or perform Step 4+. `golc.ps1` remains the supported Windows entrypoint and receives only the path/root compatibility edits required here.

**Pin boundary:** Do not edit `config/toolchain.toml`, `go.mod`, `go.sum`, `tools/linear-sync/package.json`, or `tools/linear-sync/package-lock.json`. The only configured Go/Node platform remains `windows-amd64`; do not add Linux/Darwin tables, derive archive URLs, invent checksums, insert blank pin records, or weaken the missing-platform failure. Cross-platform acceptance in this step is path/layout logic and successful source compilation only.

**Configuration decision:** Change `commands.cli_binary` from `.tools/installs/golc_project/bin/golc-project.exe` to the platform-neutral install root `.tools/installs/golc_project`. The key name and strict `toolsPathPattern` remain unchanged. Consumers resolve `<cli_binary>/<bootstrap.PlatformKey()>/bin/<platform executable name>` at the point of use. On Windows this yields `.tools/installs/golc_project/windows-amd64/bin/golc-project.exe`.

**Discovery:** Level 0. Step 1 already established `bootstrap.PlatformKey`, OS-specific archive layouts, standard-library filesystem/process seams, and Linux/Darwin mapping tests. Step 2 established the strict platform-table schema and explicitly locked the production manifest to Windows AMD64. This work adds no dependency and requires no external documentation or package legitimacy checkpoint.

<interfaces>
From `internal/bootstrap/engine.go`:
- `func PlatformKey() string` returns `runtime.GOOS + "-" + runtime.GOARCH`.
- `func platformArchiveLayout(tool, version, goos, goarch string) (platformLayout, error)` already separates platform key from archive filename, executable suffix, and Node architecture labels.
- `func Bootstrap(ctx context.Context, root string, options Options) error`.
- `type processRequest struct { Executable string; Dir string; Args []string; Env map[string]string }`.

Stable helper contract to expose from `internal/bootstrap`:
- `func ExecutableName(base string) string` returns `base + ".exe"` on Windows and `base` elsewhere.
- `func PlatformExecutablePath(installRoot, base string) string` returns `<installRoot>/<PlatformKey()>/bin/<ExecutableName(base)>`.
- `type NodeInstallation struct { Root string; Executable string; NPMCLI string }`.
- `func ResolveNodeInstallation(installDir string) (NodeInstallation, error)` enumerates the install directory, requires exactly one extracted top-level directory in addition to the install manifest, and validates the OS-specific Node/npm regular files below it.

From `internal/delivery/graph.go`:
- `type CommandInventory struct { Entrypoint string; CLIBinary string; GoVersion string }`.
- `func LoadGraph(root string) (Graph, error)`.
- Preserve `CLIBinary` as the resolved runtime executable path exposed to foundation packaging; only the raw TOML field changes to an install root.

From `internal/trace/transport/process.go`:
- `type ProcessConfig struct { NodeExecutable string; ScriptPath string; WorkDir string; Env []string; Timeout time.Duration }`.
- Extend it with `ProjectRoot string`; `NewProcessClient` must validate/normalize it and upsert `GOLC_PROJECT_ROOT=<absolute root>` into the exact environment instead of trusting inheritance.

From `cmd/golc-project/main.go`:
- `const repoRootEnvName = "GOLC_PROJECT_ROOT"`.
- `func resolveProjectRoot() (string, error)` returns an absolute root from the environment or working directory.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Centralize platform executable paths and discover verified Node installs</name>
  <files>internal/bootstrap/engine.go, internal/bootstrap/engine_test.go, internal/bootstrap/engine_linear.go, internal/bootstrap/engine_linear_test.go</files>
  <read_first>
    - `internal/bootstrap/engine.go` - retain `PlatformKey`, `platformArchiveLayout`, missing-platform diagnostics, checksum-first installation, and current platform-table selection.
    - `internal/bootstrap/engine_test.go` - retain synthetic ZIP/tar fixtures, injected runner/source, pure archive-layout cases, and zero-source idempotence assertions.
    - `internal/bootstrap/engine_linear.go` - replace only the reconstructed `nodeLayout.Root` consumption; retain the exact-lock npm/tsc sequence and manifest semantics.
    - `internal/bootstrap/engine_linear_test.go` - archive fixtures may still use `platformArchiveLayout` to create realistic payloads, but consumption assertions must prove directory discovery rather than folder-name reconstruction.
    - `config/toolchain.toml` - read-only proof that only windows-amd64 is pinned; do not modify.
  </read_first>
  <behavior>
    - "PlatformKey remains exactly runtime.GOOS-runtime.GOARCH and is used as the install segment for Go, Node, and golc-project."
    - "ExecutableName adds .exe only for Windows; Go, Node, npm, and project command archive/layout names remain separate from the stable platform key."
    - "PlatformExecutablePath resolves <install-root>/<platform>/bin/<name[.exe]> and rejects blank/unsafe base names rather than accepting a path separator or traversal."
    - "Bootstrap builds golc-project to .tools/installs/golc_project/<PlatformKey>/bin/golc-project[.exe], retaining -trimpath and the same lock/environment checks."
    - "ResolveNodeInstallation lists the verified install directory and accepts exactly one real top-level extracted directory plus the regular install-manifest file; it does not derive node-v<version>-<os>-<arch> from a version."
    - "Node resolution uses node.exe plus node_modules/npm/bin/npm-cli.js on Windows and bin/node plus lib/node_modules/npm/bin/npm-cli.js on Linux/Darwin."
    - "Zero extracted directories, multiple directories, a top-level symlink, missing/non-regular Node, or missing/non-regular npm CLI fails with a stable GOLC_NODE_TOOLCHAIN_MISSING diagnostic."
    - "The linear-sync bootstrap consumes ResolveNodeInstallation while preserving exact npm arguments, tsc path, expected outputs, lock restoration, and repeat no-op behavior."
  </behavior>
  <action>RED: Add table-driven tests for safe/unsafe executable base names and the exact current-platform project path. Extend the pure OS layout cases to assert Windows suffixes and suffixless Linux/Darwin paths without consulting or adding manifest pins. Create Node install trees whose extracted top-level directory is deliberately valid but not reconstructible from the version, then require discovery success. Add zero/multiple/symlink/missing-file cases and update the linear fake-runner assertions so current Windows arguments remain byte-for-byte equivalent. Run the two focused bootstrap scopes and confirm failures are due to the absent helpers and reconstructed Node root.

GREEN: Keep `PlatformKey` as the sole runtime platform-directory authority. Add the narrow exported helper contracts in `<interfaces>`; validate executable base names as names, not paths. Change the golc-project build output to use the configured install root plus `PlatformExecutablePath`. Implement Node discovery with `os.ReadDir` and `Lstat`/`Stat`: ignore only the known regular install-manifest file, reject unexpected top-level regular files and symlinks, sort candidates for deterministic diagnostics, and require exactly one real directory. Validate the platform-specific Node and npm CLI regular files below it. Do not inspect archive URLs or infer archive availability in this resolver. Replace `engine_linear.go`'s `platformArchiveLayout(...).Root` reconstruction with `ResolveNodeInstallation`; leave archive selection, checksum verification, npm, tsc, lock, and output behavior unchanged.</action>
  <verify>
    <automated>go test ./internal/bootstrap -run 'TestScopeBootstrap(Engine|LinearSync)$' -count=1</automated>
  </verify>
  <done>Bootstrap exposes one tested runtime path contract, emits the project CLI beneath a platform segment, and finds verified Node payloads by filesystem shape without changing any toolchain pin or Windows command behavior.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Resolve platform-neutral command configuration across command and foundation consumers</name>
  <files>config/commands.toml, internal/projectconfig/strict_test.go, internal/command/test.go, internal/command/build.go, internal/command/build_test.go, internal/command/tools_test.go, internal/delivery/graph.go, internal/delivery/foundation.go, internal/delivery/delivery_test.go, internal/trace/transport/process_test.go</files>
  <read_first>
    - `config/commands.toml` - `commands.cli_binary` is the only command-location authority; preserve every other value and comment block.
    - `internal/projectconfig/model.go` - `commands.cli_binary` already uses `toolsPathPattern`; do not loosen it or add platform wildcard keys.
    - `internal/command/test.go` - `resolvePinnedGoExecutable` currently derives the suffix but hardcodes the platform directory.
    - `internal/command/build.go` - `resolvePinnedNodeExecutable` currently rejects non-Windows and reconstructs Node's archive directory name.
    - `internal/delivery/graph.go` - LoadGraph consumes the raw TOML command inventory and exposes `CommandInventory.CLIBinary`.
    - `internal/delivery/foundation.go` - FoundationInventory includes the resolved CLI payload, and DefaultFoundationOutputPaths owns all three artifact filenames.
    - `internal/trace/transport/process_test.go` - `resolveTestNode` must prefer a project-local provisioned Node without a Windows-only glob or archive-directory spelling.
  </read_first>
  <behavior>
    - "commands.cli_binary is exactly .tools/installs/golc_project and remains a valid strict project-local tools path; an absolute, traversing, or executable-looking value outside the allowed pattern still fails."
    - "LoadGraph resolves the raw install root into .tools/installs/golc_project/<PlatformKey>/bin/golc-project[.exe] before exposing CommandInventory.CLIBinary."
    - "FoundationInventory includes that resolved current-platform CLI path and retains its bounded sorted allowlist."
    - "DefaultFoundationOutputPaths emits golc-foundation-<PlatformKey>.zip, golc-foundation-<PlatformKey>.manifest.json, and golc-foundation-<PlatformKey>.zip.sha256; the checksum line uses the resolved ZIP basename."
    - "resolvePinnedGoExecutable uses bootstrap.PlatformKey and ExecutableName while still reading only toolchain.go.version and failing clearly when the current platform was not provisioned."
    - "resolvePinnedNodeExecutable uses bootstrap.PlatformKey for the install directory and ResolveNodeInstallation for the extracted payload; it has no Windows-only guard or node-v*-win-x64 construction."
    - "The process-transport test helper prefers the same platform-keyed, discovered project Node and only then falls back to a host PATH lookup, so the package remains testable before bootstrap."
    - "All Windows expectations remain the same after inserting the windows-amd64 platform directory into golc-project's install path."
  </behavior>
  <action>RED: Update configuration and fixture expectations first: make `commands.cli_binary` an install root, assert strict validation accepts that root, and assert LoadGraph returns the fully resolved runtime executable rather than the raw root. Update delivery fixtures to create the resolved CLI payload. Replace every fixed foundation filename expectation with `"golc-foundation-" + bootstrap.PlatformKey()` and verify the checksum sidecar names the exact ZIP basename. Add command tests that construct Go and Node installs beneath `bootstrap.PlatformKey()`, including a Node top-level directory with a non-derived name, and require both resolvers to return the correct current-platform executable. Update the transport test helper expectation. Run the focused packages and confirm the old hardcoded paths fail these tests.

GREEN: Change only `commands.cli_binary`'s value and nearby semantics comment; preserve all PR graph and Linear prose. Import the existing bootstrap helpers where consumption occurs. `LoadGraph` must join the repository root only for existence/use checks while retaining a repository-relative forward-slash `CommandInventory.CLIBinary` for foundation archive/source paths. Reject a blank or unsafe install root through the existing inventory diagnostic path. Make Go and Node command resolvers use `bootstrap.PlatformKey`; Node delegates top-level discovery to `bootstrap.ResolveNodeInstallation`. Keep missing-tool diagnostics actionable without claiming that Linux/Darwin pins exist. Parameterize foundation artifact names from `bootstrap.PlatformKey`, and derive the checksum sidecar's embedded filename from `filepath.Base(paths.ZIPPath)` so it cannot drift. Remove Windows archive-folder literals from the transport test helper. Do not edit `config/toolchain.toml`, `internal/projectconfig/model.go`'s exact registered toolchain platform keys, `internal/command/tools.go`'s Windows-pin update behavior, or any generated schema.</action>
  <verify>
    <automated>go test ./internal/projectconfig ./internal/command ./internal/delivery ./internal/trace/transport -count=1</automated>
  </verify>
  <done>The configuration stores one platform-neutral install root, every consumer resolves the current-platform executable/foundation identity consistently, Node no longer depends on an archive folder name, and the committed pin set is unchanged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Propagate the resolved project root through every child-process boundary</name>
  <files>golc.ps1, cmd/golc-project/main.go, cmd/golc-project/main_test.go, internal/bootstrap/engine.go, internal/bootstrap/engine_test.go, internal/command/test.go, internal/command/build.go, internal/command/linear_sync.go, internal/command/linear.go, internal/trace/transport/process.go, internal/trace/transport/process_test.go</files>
  <read_first>
    - `golc.ps1` - root resolution occurs before bootstrap/delegation; retain its complete compatibility implementation and change only root/path variables and environment propagation.
    - `cmd/golc-project/main.go` - `resolveProjectRoot` already normalizes the environment-or-cwd choice; `run` currently does not establish that resolved value for descendants.
    - `internal/command/test.go` - `projectGoEnvironment`, Node scope registration, and `runNodeScopeTest` are the Go/Node test child boundaries.
    - `internal/command/build.go` - `runBuildNodeScope` currently inherits `os.Environ` without overriding project root.
    - `internal/command/linear_sync.go` - Node test commands currently resolve their executable at package initialization through environment/cwd state.
    - `internal/command/linear.go` - `newProcessLinearClient(root, ...)` owns the production ProcessConfig call.
    - `internal/trace/transport/process.go` - ProcessConfig's environment is intentionally explicit and must not silently inherit anything not supplied.
  </read_first>
  <behavior>
    - "golc.ps1 sets GOLC_PROJECT_ROOT to its absolute RepoRoot immediately after root discovery, so bootstrap and delegated children observe the same root; current Windows command arguments and exit mapping remain unchanged."
    - "cmd/golc-project resolves the absolute root, sets GOLC_PROJECT_ROOT to that value before constructing/executing the command registry, and restores no stale caller value into children."
    - "A direct golc-project invocation from the repository root works when GOLC_PROJECT_ROOT was initially absent; a supplied valid root remains authoritative."
    - "Node test scope registration stores arguments/intent, not an executable path resolved during package initialization; runNodeScopeTest resolves pinned Node from Request.Root immediately before execution."
    - "projectGoEnvironment, runNodeScopeTest, and runBuildNodeScope upsert exactly one GOLC_PROJECT_ROOT=<absolute request root>, replacing a stale inherited value case-insensitively on Windows."
    - "bootstrap's process environment includes its symlink-resolved repository root as GOLC_PROJECT_ROOT for module, probe, build, npm, and tsc children."
    - "ProcessConfig requires ProjectRoot; NewProcessClient normalizes it and upserts it into cfg.Env while preserving the explicit-environment/no-implicit-inheritance contract."
    - "newProcessLinearClient passes its root into ProcessConfig, and the fixture Node process proves the received value equals the caller's resolved root."
    - "No credential value is logged, copied into configuration, or added beyond the environment already explicitly supplied to the Linear adapter."
  </behavior>
  <action>RED: Add `cmd/golc-project/main_test.go` around an isolated temporary repository/root seam, asserting environment-absent and stale-environment cases establish the resolved absolute value before command execution; use a narrow injectable registry/execution seam if needed instead of launching the real command graph. Add environment-list tests that require case-insensitive replacement and one final root entry for Go and Node children. Refactor the Node scope test expectations to require runtime root resolution rather than package-init resolution. Extend bootstrap fake-runner assertions to require GOLC_PROJECT_ROOT on every request. Extend the process fixture with a mode that returns `process.env.GOLC_PROJECT_ROOT`, then add missing/stale Env cases around `ProcessConfig.ProjectRoot`. Run focused tests and confirm failures against the current inherited/package-init behavior.

GREEN: In `golc.ps1`, set `$env:GOLC_PROJECT_ROOT` once immediately after `$RepoRoot` is resolved; remove the later redundant assignment but preserve all command parsing, bootstrap compatibility, and exit behavior. In `cmd/golc-project`, resolve the root before registry use, set the process environment to the normalized value, and fail with the existing root diagnostic if setting it fails. Introduce one small command-package environment upsert helper used by `projectGoEnvironment`, Node tests, and Node builds; preserve existing cache/offline values and replace, rather than append beside, stale root entries.

Change `NodeScopeRegistration` so registrations in `linear_sync.go` retain only Node arguments/test files and resolve the pinned executable from the `root` passed to `runNodeScopeTest`; do not consult `GOLC_PROJECT_ROOT` or cwd during package initialization and do not weaken the non-empty/marker/duplicate validation. Set bootstrap's merged process environment root explicitly after root symlink resolution. Add `ProjectRoot` to `transport.ProcessConfig`; validate it as an absolute normalized path, upsert it into a copy of `cfg.Env`, and keep `cmd.Env` explicit. Pass `root` from `newProcessLinearClient`. Do not broaden the adapter environment allowlist, echo environment values, or alter RPC framing, timeouts, redaction, cancellation, and process-tree cleanup.</action>
  <verify>
    <automated>go test ./cmd/golc-project ./internal/bootstrap ./internal/command ./internal/trace/transport -count=1</automated>
  </verify>
  <done>Both entrypoint paths establish one authoritative absolute project root, every Go/Node/Linear child receives it deterministically, and Node registrations no longer depend on package-init environment timing.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| committed config to executable path | A repository-relative install root becomes a platform-specific executable/source path and must remain contained and deterministic. |
| verified archive install to Node execution | Top-level extracted filesystem entries are consumed as executable locations only after type/count/path validation. |
| entrypoint to child processes | The resolved repository root crosses into Go, Node, npm, tsc, and Linear adapter environments and must overwrite stale ambient state. |
| host environment to Linear adapter | Credentials remain explicit environment inputs while project-root propagation must not cause implicit inheritance or logging. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-tgd-01 | Tampering | `commands.cli_binary` consumption | high | mitigate | Keep the strict project-local tools-path grammar, resolve only with canonical platform/name helpers, retain repository-relative inventory paths, and test traversal/absolute rejection. |
| T-tgd-02 | Spoofing / Elevation of Privilege | Node top-level discovery | high | mitigate | Enumerate deterministically, reject symlinks/unexpected entries/multiple roots, and require regular OS-specific Node/npm files beneath the sole verified install directory. |
| T-tgd-03 | Tampering | `GOLC_PROJECT_ROOT` inheritance | high | mitigate | Normalize the entrypoint root and case-insensitively replace stale environment entries at every child boundary; tests cover absent and conflicting inherited values. |
| T-tgd-04 | Information Disclosure | Linear adapter environment | high | mitigate | Preserve explicit `ProcessConfig.Env`, add only the normalized project root, never log environment values, and retain existing stderr redaction/canary tests. |
| T-tgd-05 | Denial of Service | missing future-platform archive pin | low | accept | Missing Linux/Darwin pins continue to fail closed before bootstrap effects; this step proves source layout/compilation only and makes no runtime availability claim. |
| T-tgd-SC | Tampering | dependency/toolchain supply chain | low | accept | No dependency or pin is added, upgraded, installed, or removed; exact committed Windows archive hashes and lockfiles remain unchanged. |
</threat_model>

<source_audit>

| SOURCE | ID | Feature/Requirement | Plan | Status | Notes |
|--------|----|---------------------|------|--------|-------|
| GOAL | — | Replace hardcoded platform strings and propagate project root for PowerShell-removal Step 3 | 260723-tgd | COVERED | Three TDD tasks cover helpers, consumers, and process propagation. |
| REQ | CONF-01 | One discoverable root configuration and entrypoint contract | 260723-tgd | COVERED | `commands.cli_binary` remains the single platform-neutral command-location authority. |
| REQ | CONF-02 | No duplicated authoritative configuration values | 260723-tgd | COVERED | Platform and suffix are derived at consumption; no extra pin/config copy is added. |
| REQ | CONF-03 | Contributor and CI command behavior remains consistent | 260723-tgd | COVERED | Windows behavior is retained and the headless CLI is cross-compiled from the same source graph. |
| RESEARCH | Step 1 | `bootstrap.PlatformKey` and archive layout are distinct contracts | 260723-tgd | COVERED | Task 1 preserves the distinction and exports narrow consumption helpers. |
| RESEARCH | Step 2 | Production platform pins are exactly windows-amd64 | 260723-tgd | COVERED | Pin boundary forbids any new URL/checksum/table. |
| CONTEXT | — | Use bootstrap.PlatformKey instead of hardcoded windows-amd64 | 260723-tgd | COVERED | Tasks 1-2 cover bootstrap, command, delivery, and test consumers. |
| CONTEXT | — | Use platform-key foundation filenames | 260723-tgd | COVERED | Task 2 updates all three paths and checksum identity. |
| CONTEXT | — | Stop treating commands.cli_binary as one .exe literal | 260723-tgd | COVERED | Config becomes an install root; LoadGraph resolves the executable at consumption. |
| CONTEXT | — | Discover Node's extracted top-level directory by listing | 260723-tgd | COVERED | Task 1 centralizes strict discovery; Task 2 removes reconstructed consumers. |
| CONTEXT | — | Propagate GOLC_PROJECT_ROOT into trace and Linear child processes | 260723-tgd | COVERED | Task 3 covers PowerShell, Go CLI, command children, bootstrap, and ProcessConfig. |
| CONTEXT | — | Keep Windows behavior identical and prepare Linux/Darwin builds | 260723-tgd | COVERED | Current-path assertions plus target builds are in verification. |
| CONTEXT | — | Exclude Mage, parity, deletion, CI, and Step 4+ | 260723-tgd | COVERED | Explicit Step boundary and task actions prohibit these edits. |
| CONTEXT | — | Preserve unrelated sgy/sketch work | 260723-tgd | COVERED | Dirty-worktree constraint names every current unrelated path. |

</source_audit>

<verification>
- Run `go test ./internal/bootstrap ./internal/projectconfig ./internal/command ./internal/delivery ./internal/trace/transport ./cmd/golc-project -count=1`.
- Run `go test ./... -count=1` on Windows to catch all config/command/foundation consumers.
- Hash `config/toolchain.toml`, `go.mod`, `go.sum`, `tools/linear-sync/package.json`, and `tools/linear-sync/package-lock.json` before and after verification; all five must be byte-identical.
- Build `./cmd/golc-project` for `linux/amd64` and `darwin/arm64` with `CGO_ENABLED=0`, writing each output to an explicitly named file under `t.TempDir()` in an automated Go test or to an exact temporary path with guaranteed cleanup; successful compilation is not a support/qualification claim.
- On Windows, assert `bootstrap.PlatformKey() == "windows-amd64"`, the project command path ends in `windows-amd64/bin/golc-project.exe`, and foundation outputs retain `golc-foundation-windows-amd64.*`.
- `rg -n 'windows-amd64|node-v.*win-x64|golc-project\\.exe|foundation-windows-amd64' internal/bootstrap internal/command internal/delivery internal/trace/transport config/commands.toml` may retain platform literals only in Step 2 pin fixtures/update tests and explicit Windows expected-value tests; production path consumers contain none.
- `git diff -- .planning/sketches .planning/quick/260723-sgy-port-the-full-golc-site-design-language-` shows no executor-authored changes.
</verification>

<success_criteria>
- Runtime platform directories and executable suffixes come from one tested bootstrap contract.
- The platform-neutral CLI install root resolves to the correct current-platform binary everywhere it is consumed or packaged.
- Node execution no longer relies on reconstructing the official archive's top-level folder name.
- Every child process that can resolve project state receives the same normalized `GOLC_PROJECT_ROOT`.
- Windows behavior remains identical; linux/amd64 and darwin/arm64 headless CLI builds compile without new toolchain pins.
- Toolchain pins, dependency locks, command parity, CI, PowerShell lifetime, and unrelated sketch work remain untouched.
</success_criteria>

<output>
Create `.planning/quick/260723-tgd-replace-hardcoded-platform-strings-and-p/260723-tgd-SUMMARY.md` when done.
</output>
