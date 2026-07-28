# Phase 1: Offline Foundation and Delivery Traceability - Research

**Researched:** 2026-07-17  
**Domain:** Windows-first repository tooling, strict configuration, offline bootstrap, and Linear reconciliation  
**Confidence:** MEDIUM

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Contributor Setup Flow
- **D-01:** A clean Windows checkout is prepared through one bootstrap command that installs or downloads pinned project-local tools where practical and then verifies the complete environment.
- **D-02:** After one successful bootstrap, core generation, validation, build, and test operations must work offline from pinned local caches. Network-only operations such as dependency refresh and Linear sync fail clearly without breaking offline work.
- **D-03:** Contributors use one repository command with clear subcommands for at least bootstrap, check, generate, build, test, package, and Linear operations rather than discovering ecosystem-specific commands.
- **D-04:** Tool and dependency updates are explicit. An update command produces reviewable manifest and lockfile changes; bootstrap never silently upgrades pinned versions.

#### Configuration Hierarchy
- **D-05:** A small machine-readable manifest at the repository root is the central configuration index. It points to logically separated configuration files organized by concern rather than becoming a monolithic settings file.
- **D-06:** Override precedence is committed defaults, then user-level configuration, untracked project-local configuration, environment variables, and finally command-line flags.
- **D-07:** The effective configuration is inspectable, including the source layer that supplied each resolved value.
- **D-08:** Stable generated schemas, public contracts, and generated types needed for review or downstream consumers are committed. Caches, temporary generation output, and machine-specific artifacts are ignored.
- **D-09:** Configuration validation is strict: unknown keys, duplicate authority, invalid values, and unresolved references fail immediately. Deprecated keys warn and provide migration guidance.
- **D-10:** Local development and CI invoke the same repository commands and validate each concern independently while retaining one authoritative value for shared settings.

#### Linear Authority and Conflicts
- **D-11:** Repository artifacts own scope, durable local IDs, requirement text, and roadmap phase structure. Linear owns operational execution fields: status, assignee, priority, estimate, and completion timestamps.
- **D-12:** Linear comments and discussion remain in Linear and are not mirrored into repository planning artifacts.
- **D-13:** If both sides changed the same mapped field since the last synchronization, that item is blocked. The tool shows a field-by-field conflict preview and requires explicit resolution; neither side wins automatically.
- **D-14:** Stable local and Linear UUID identities never change during renames. Renames update display text only.
- **D-15:** Removal is never mirrored as an automatic deletion. It requires an explicit reviewed archive or unlink action.

#### Linear Synchronization Lifecycle
- **D-16:** Mutating synchronization runs only through explicit repository commands at planning or execution milestones. Pull-request CI performs read-only drift checks and must not mutate Linear.
- **D-17:** Mutation is a two-step operation: preview creates a deterministic reconciliation plan, and a separate apply command executes that exact plan.
- **D-18:** Apply rejects a preview if relevant repository or Linear state changed after the preview was produced.
- **D-19:** The repository commits `.env.example` with all supported variables and safe placeholders. The real local `.env` remains untracked.
- **D-20:** CI creates an ephemeral `.env` from its protected secret store and removes it after the job. Secret values must never appear in previews, logs, errors, mapping files, or committed artifacts.
- **D-21:** Synchronization failures, ambiguity, pagination, partial GraphQL errors, and rate limits are reported without blocking local planning, builds, tests, or application runtime.

### the agent's Discretion
- Select the implementation technology and internal name for the single repository command, provided its user-facing subcommands and behavior match the locked decisions.
- Choose exact root-manifest and concern-file names, schemas, and directory layout while preserving one machine-readable index and independent validation.
- Choose cache locations, download verification mechanics, and retry/backoff details for transient network operations.
- Choose the serialization format and hashing strategy for deterministic Linear preview plans.
- Choose the CI provider configuration and how its protected secret is materialized as an ephemeral `.env`.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within Phase 1 scope.
</user_constraints>

## Project Constraints (from AGENTS.md)

- Build application and repository tooling around Go and Wails; v1 qualification is Windows-only while architecture should remain portable. [VERIFIED: AGENTS.md]
- TypeScript is the required user scripting language, and Node remains build/tooling infrastructure rather than an application runtime dependency. [VERIFIED: AGENTS.md]
- Use Linear from project inception with explicit repository-to-Linear traceability, but keep repository planning complete offline. [VERIFIED: AGENTS.md]
- Centralize project configuration behind one documented root entrypoint while separating concerns and avoiding scattered sources of truth. [VERIFIED: AGENTS.md]
- Do not let UI, network-bound LLM work, scripts, APIs, or Linear enter the deterministic playback/output path; Phase 1 therefore keeps all Linear code under developer tooling. [VERIFIED: AGENTS.md]
- Pin Go 1.26.5, Wails v2.13.0, Node 24.18.0 LTS, TypeScript 7.0.2, and `@linear/sdk` 88.1.0; do not float `latest` in CI. [VERIFIED: AGENTS.md; VERIFIED: official Go/Node/Wails sources]
- Keep the official Linear SDK isolated under `tools/linear-sync`, use exact lockfiles, paginate GraphQL connections, and obtain `LINEAR_API_KEY` from external secrets. [VERIFIED: AGENTS.md]
- Use GSD workflow entrypoints before implementation edits; this research artifact is produced by the active GSD planning workflow. [VERIFIED: AGENTS.md]
- No project-specific skills exist in the configured skill directories, so no additional project skill rules constrain this phase. [VERIFIED: project skill discovery]

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONF-01 | A contributor can discover toolchain versions, setup, generation, validation, build, test, packaging, application-default, and runtime-configuration entrypoints from one documented root project configuration. | Root `golc.project.toml` index plus `golc.ps1` command boundary and concern registry. [VERIFIED: .planning/REQUIREMENTS.md] |
| CONF-02 | Project configuration is separated into independently validatable concerns without duplicating authoritative values across files. | Concern ownership registry, reference validation, schema generation, and duplicate-authority checks. [VERIFIED: .planning/REQUIREMENTS.md] |
| CONF-03 | A contributor and CI can invoke the same documented project commands for generation, validation, build, test, and packaging. | One PowerShell shim delegates to a pinned project-local Go CLI; GitHub Actions invokes the same commands. [VERIFIED: .planning/REQUIREMENTS.md] |
| CONF-04 | Secrets and machine-local values remain outside committed project configuration and are represented by documented names and safe examples. | `.env.example`, ignored `.env`, protected CI environment, redaction/canary tests, and secret-free plan/map schemas. [VERIFIED: .planning/REQUIREMENTS.md] |
| LINR-01 | Every milestone, phase, requirement, plan, and executable task has a durable local identifier that remains usable offline. | Schema-2 entity catalog extending `.planning/linear-map.json`, stable ID grammar, parent links, and source anchors. [VERIFIED: .planning/REQUIREMENTS.md] |
| LINR-02 | The repository maintains a credential-free mapping from durable local identifiers to immutable Linear UUIDs without making Linear the only source of planning truth. | Nullable remote mapping records, UUID-only linkage after explicit adoption, and local artifacts as scope/text authority. [VERIFIED: .planning/REQUIREMENTS.md] |
| LINR-03 | A contributor can preview and run an idempotent reconciliation that creates or updates the intended Linear project, milestones, issues, and sub-issues without duplicating retried work. | Canonical plan hash, marker-based recovery, per-operation before/after states, resumable apply, and no title-based adoption. [VERIFIED: .planning/REQUIREMENTS.md] |
| LINR-04 | Linear synchronization reports ambiguity, partial GraphQL errors, pagination, and rate limiting without blocking local planning, builds, tests, or application runtime. | Exhaustive Relay pagination, GraphQL data-plus-errors handling, rate headers/codes, isolated exit status, and fake-server tests. [VERIFIED: .planning/REQUIREMENTS.md] |
</phase_requirements>

## Summary

Phase 1 should establish a two-stage repository command: a tiny Windows PowerShell 5.1 shim at `golc.ps1` that can bootstrap from a clean checkout, followed by a project-local Go executable that owns every normal subcommand. The shim should download official Go and Node archives into ignored `.tools/` directories, verify committed SHA-256 values before extraction, set project-local caches, compile the Go command, install Wails at the exact module version, and run one verification pass. Go publishes Windows ZIP archives with SHA-256 values, Node publishes official binaries, and Wails v2.13.0 documents an exact `go install ...@v2.13.0` command. [CITED: https://go.dev/dl/] [CITED: https://nodejs.org/en/about/previous-releases] [CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0]

Use TOML 1.0 for hand-edited root/concern manifests, JSON for generated schemas and machine-mutated Linear maps/plans, and reserve strict YAML 1.2 for Phase 2 fixture authoring. TOML forbids duplicate key/table definitions; BurntSushi TOML exposes undecoded keys for strict validation; JSON only recommends unique object names, so GOLC must reject duplicate JSON names before ordinary decoding; YAML 1.2 requires unique mapping keys but introduces authoring features that Phase 1 does not need. [CITED: https://toml.io/en/v1.0.0] [CITED: https://github.com/BurntSushi/toml] [CITED: https://www.rfc-editor.org/rfc/rfc8259] [CITED: https://yaml.org/spec/1.2.2/]

Linear reconciliation must be a repository-owned three-way merge, not an API mirroring script. The official API is Relay-paginated GraphQL, can return partial data with an `errors` array even on HTTP 200, and reports rate limiting through error code `RATELIMITED` plus rate/complexity headers. Core create mutations should not be treated as remotely idempotent: the official documentation found in this session documents attachment-URL idempotency, not a general idempotency key for Project, Project Milestone, or Issue creation. Therefore each remote description should carry an exact visible GOLC local-ID footer, every create should re-discover that marker before mutation or retry, and apply should compare per-operation preconditions/postconditions so a partially completed plan can resume without duplicating work. [CITED: https://linear.app/developers/pagination] [CITED: https://linear.app/developers/graphql] [CITED: https://linear.app/developers/rate-limiting] [CITED: https://linear.app/developers/attachments]

**Primary recommendation:** Build the thinnest vertical slice around `powershell -NoProfile -File .\golc.ps1 <subcommand>`, strict concern manifests, a schema-2 local identity map, and a fake-server-proven Linear preview/apply engine; do not create a Wails UI or product installer in this phase.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Clean-checkout bootstrap | Developer host / root shim | Project-local tool cache | PowerShell exists on supported Windows and can obtain the pinned Go command before Go is available. [VERIFIED: environment audit; RECOMMENDATION] |
| Root command routing | Developer tooling (Go CLI) | PowerShell shim | Normal behavior belongs in tested Go packages; the shim stays limited to bootstrap/delegation. [RECOMMENDATION] |
| Configuration parsing/resolution | Developer tooling (Go CLI) | Filesystem | It validates committed, user, local, environment, and CLI layers and reports provenance. [VERIFIED: CONTEXT.md D-05 through D-10] |
| Generated schemas | Developer tooling (Go generator) | Repository contracts | Go types own semantics; committed JSON Schema is a review/drift artifact. [CITED: https://github.com/invopop/jsonschema] |
| Durable planning identity | Repository planning artifacts | `.planning/linear-map.json` | Local IDs and source text remain usable without network or credentials; remote UUIDs are optional mappings. [VERIFIED: CONTEXT.md D-11 and D-14] |
| Linear read/reconcile | `tools/linear-sync` TypeScript adapter | Go plan engine | The official SDK performs typed transport while GOLC owns merge rules, identity, and exact plans. [CITED: https://linear.app/developers/sdk-fetching-and-modifying-data] |
| Linear mutation | Explicit developer command | Linear GraphQL service | It is a network-only governance action and never participates in build, test, planning, or application startup. [VERIFIED: CONTEXT.md D-16 and D-21] |
| CI validation | GitHub Actions Windows runner | Root command | CI calls the same repository command; PR jobs remain read-only and secret-optional. [CITED: https://docs.github.com/en/actions/security-for-github-actions/security-guides/using-secrets-in-github-actions] |
| Product runtime/playback | Future Go application tiers | — | Phase 1 deliberately adds no lighting behavior, Wails UI, Art-Net, scripting, or AI runtime. [VERIFIED: 01-CONTEXT.md Phase Boundary] |

## Standard Stack

### Core

| Technology / Library | Version | Purpose | Why Standard |
|----------------------|---------|---------|--------------|
| Windows PowerShell | 5.1 baseline | Clean-checkout shim and checksum-verified bootstrap | Present on the audited Windows host and does not require an already-installed project runtime. [VERIFIED: environment audit] |
| Go | 1.26.5 | Project command, configuration model, plan engine, generators, and tests | Required project language; official Windows archives and hashes are available. [CITED: https://go.dev/dl/] |
| Wails CLI | v2.13.0 | Pinned future desktop build tool, installed project-locally during bootstrap | Official release gives an exact `go install` command; no Wails application/UI is created in Phase 1. [CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0] |
| Node.js | 24.18.0 LTS | Compile/run the isolated Linear TypeScript adapter | The official release page identifies v24.18.0 as Latest LTS; Node is developer tooling only. [CITED: https://nodejs.org/en/about/previous-releases] |
| TypeScript | 7.0.2 exact | Compile the Linear adapter | Required project language and locked by AGENTS.md; the exact pin was checked against the official package listing. `[WARNING: flagged as suspicious — verify before using.]` [CITED: https://www.npmjs.com/package/typescript] [VERIFIED: AGENTS.md] |
| `@linear/sdk` | 88.1.0 exact | Official typed GraphQL transport | Locked by AGENTS.md and official Linear docs recommend the TypeScript SDK. Registry latest was 88.2.0 during this audit, so bootstrap must retain 88.1.0 until an explicit update changes the lockfile. `[WARNING: flagged as suspicious — verify before using.]` [CITED: https://linear.app/developers/graphql] [CITED: https://www.npmjs.com/package/@linear/sdk] [VERIFIED: AGENTS.md] |
| `github.com/BurntSushi/toml` | v1.6.0 | TOML 1.0 parsing and unknown-key detection | `MetaData.Undecoded()` supports strict concern validation; version and source origin were verified with `go list -m -json`. [CITED: https://github.com/BurntSushi/toml] [VERIFIED: Go module metadata] |
| `github.com/invopop/jsonschema` | v0.14.0 | Generate Draft 2020-12 JSON Schema from authoritative Go structs | It emits `additionalProperties: false`, required fields, and descriptions from Go types; version/source were verified with `go list -m -json`. [CITED: https://github.com/invopop/jsonschema] [VERIFIED: Go module metadata] |

### Supporting

| Library / Facility | Version | Purpose | When to Use |
|--------------------|---------|---------|-------------|
| Go `crypto/sha256` | Go 1.26.5 stdlib | Tool archive verification and plan/state digests | Hash committed tool archives, normalized repository intent, remote observations, and canonical plans. [CITED: https://pkg.go.dev/crypto/sha256] |
| Go `encoding/json` token API | Go 1.26.5 stdlib | JSON syntax/token validation and deterministic encoding | Reject duplicate names before typed decode; use sorted structures for exact plan bytes. `Decoder.Token` guarantees matched delimiters, while `DisallowUnknownFields` rejects unknown struct fields. [CITED: https://pkg.go.dev/encoding/json] |
| Node `node:test` | Node 24.18.0 built-in | Linear adapter unit/contract tests | Avoid adding a separate Phase-1 JavaScript test framework. [CITED: https://nodejs.org/api/test.html] |
| GitHub Actions | Hosted `windows-2025` or repository-selected pinned image | Contributor/CI command parity and protected secret jobs | Use the same `golc.ps1` commands; keep PR validation read-only. [CITED: https://docs.github.com/en/actions] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| PowerShell shim + Go CLI | Task, mise, aqua, or a global Wails/Go install | These can simplify tool management, but a clean checkout still needs their bootstrap and global state. The shim/CLI pair satisfies the locked one-command Windows flow with fewer authorities. [RECOMMENDATION] |
| TOML concern manifests | JSON | JSON has excellent tooling but no comments and duplicate names are only a `SHOULD`-level interoperability rule; TOML is better for small hand-edited concerns. [CITED: https://www.rfc-editor.org/rfc/rfc8259] [CITED: https://toml.io/en/v1.0.0] |
| TOML concern manifests | YAML | YAML supports richer graphs, tags, anchors, and aliases that are unnecessary for developer configuration; reserve the strict subset for fixture authoring where nesting is valuable. [CITED: https://yaml.org/spec/1.2.2/] [VERIFIED: AGENTS.md] |
| Project-local caches after bootstrap | Commit all vendor/tool binaries | Vendoring can make builds independent of a module cache, but committing large toolchains/binaries would bloat the repository. Go documents both module caching and `-mod=vendor`; Phase 1 should use verified project-local caches and keep vendoring as a later policy option. [CITED: https://go.dev/ref/mod] |
| Official Linear TypeScript SDK | Hand-written GraphQL client or community CLI | A custom client reduces the Node boundary but must reproduce schema types and error behavior; a community CLI adds another identity interpretation. Keep the official SDK behind a small adapter. [CITED: https://linear.app/developers/graphql] |

### Installation and Bootstrap Contract

```powershell
# Supported contributor entrypoint from a clean Windows checkout.
powershell -NoProfile -ExecutionPolicy Bypass -File .\golc.ps1 bootstrap

# All later operations use the same entrypoint and pinned project-local tools.
.\golc.ps1 check --offline
.\golc.ps1 generate --check
.\golc.ps1 build
.\golc.ps1 test
.\golc.ps1 package --foundation
```

Bootstrap should:

1. Read only the locked root index and toolchain manifest using a minimal bootstrap-safe parser; never consult `.env`. [RECOMMENDATION]
2. Download official Go 1.26.5 and Node 24.18.0 Windows archives to a staging directory, verify committed SHA-256 values with `Get-FileHash`, then extract to `.tools/toolchains/<name>/<version>/<os-arch>/`. Go publishes the Windows AMD64 ZIP hash `97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38`. [CITED: https://go.dev/dl/]
3. Set `GOTOOLCHAIN=local` so an invocation cannot silently download a newer Go toolchain; automatic toolchain mode may otherwise download toolchains as modules. [CITED: https://go.dev/doc/toolchain]
4. Set project-local `GOMODCACHE`, `GOCACHE`, and npm cache paths under ignored `.tools/cache/`; run `go mod download` and `npm ci` once online, then compile `.tools/bin/golc-project.exe` and `tools/linear-sync/dist/`. Go documents that module downloads populate the module cache and that `GOPROXY=off` forbids network access. [CITED: https://go.dev/ref/mod]
5. Install Wails with the local Go binary and exact command `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`, setting `GOBIN=.tools/bin`. [CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0]
6. Finish with `check --offline`, which sets `GOPROXY=off`, disables npm network access, and fails if any core command attempts network I/O. [RECOMMENDATION]

The explicit `tools update` subcommand may access official release/registry sources, but it must only propose exact version/hash/lockfile changes and print a reviewable diff. `bootstrap` must treat manifest pins as immutable inputs. [VERIFIED: CONTEXT.md D-04]

## Package Legitimacy Audit

The npm legitimacy seam was run on every npm package installed by the Phase-1 tooling workspace. The seam marked both packages `SUS` only because their newest releases are recent; both names are confirmed by authoritative project documentation/source repositories, have long-lived package histories, high download counts, and no `postinstall` script. Protocol still requires a human verification checkpoint before first install. [VERIFIED: package-legitimacy seam] [CITED: https://www.npmjs.com/package/@linear/sdk] [CITED: https://www.npmjs.com/package/typescript]

| Package | Registry | Age / Published Pin | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|---------------------|-----------|-------------|---------|-------------|
| `@linear/sdk@88.1.0` | npm | Package created 2019-01-26; pin published 2026-07-08 | 1,657,119/week signal | `github.com/linear/linear` | SUS — newest package release is recent | Approved only after `checkpoint:human-verify`; exact lock, integrity `sha512-l0U5...STFxw==`, no postinstall. `[WARNING: flagged as suspicious — verify before using.]` [VERIFIED: package-legitimacy seam] [CITED: https://www.npmjs.com/package/@linear/sdk] |
| `typescript@7.0.2` | npm | Pin published 2026-07-08 | 218,061,988/week signal | `github.com/microsoft/TypeScript` | SUS — release is recent | Approved only after `checkpoint:human-verify`; exact lock, integrity `sha512-8FYa...RVUNA==`, no postinstall. `[WARNING: flagged as suspicious — verify before using.]` [VERIFIED: package-legitimacy seam] [CITED: https://www.npmjs.com/package/typescript] |

The Go legitimacy seam does not emit verdicts for Go modules. `github.com/BurntSushi/toml@v1.6.0`, `github.com/invopop/jsonschema@v0.14.0`, and `github.com/wailsapp/wails/v2@v2.13.0` were instead resolved through authoritative Context7/GitHub sources and verified with `go list -m -json`, including tag/source origins. [VERIFIED: Go module metadata; CITED: official repositories]

**Packages removed due to SLOP verdict:** none.  
**Packages flagged as suspicious [SUS]:** `@linear/sdk`, `typescript`; planner must insert `checkpoint:human-verify` before the first `npm ci`.

## Architecture Patterns

### System Architecture Diagram

```text
Clean checkout / Contributor / GitHub Actions
                    |
                    v
           golc.ps1 (PowerShell 5.1)
              /                    \
     bootstrap (network allowed)    normal subcommands
              |                    |
              v                    v
 official Go/Node archives   .tools/bin/golc-project.exe
 + committed SHA-256                  |
 + exact module/package pins          +--> config index/concern loader
              |                       |       |
              v                       |       +--> validate -> effective value + provenance
 .tools/toolchains + .tools/cache     |       +--> generate -> committed schemas
                                      |
                                      +--> check/build/test/package (offline core)
                                      |
                                      +--> linear validate/check (offline)
                                      |
                                      +--> linear preview/apply (explicit network boundary)
                                                  |
                                                  v
                                      tools/linear-sync (Node + official SDK)
                                                  |
                                   Relay pages + data/errors + rate headers
                                                  |
                                                  v
                                           Linear GraphQL API

Decision branches:
  no credential/network -> fail only the requested Linear network command;
                           all local commands continue
  unmapped local ID     -> exact marker discovery -> 0=create, 1=adopt/check,
                           >1=ambiguity block
  mapped local ID       -> query immutable UUID; never title-match
  apply current state   -> equals desired: no-op; equals preview before-state:
                           mutate; otherwise: stale/conflict block
```

### Recommended Project Structure

```text
golc.ps1                         # only supported root command/shim
golc.project.toml                # small root index; paths and schema version
config/
├── toolchain.toml               # exact tools, archives, hashes, cache policy
├── commands.toml                # subcommand/build/test/package entrypoints
├── generation.toml              # source -> committed generated artifact rules
├── application-defaults.toml    # future product defaults; no machine values
├── runtime.toml                 # runtime key declarations; no secrets
└── integrations/
    └── linear.toml              # non-secret taxonomy/config key names
schemas/
├── golc-project.schema.json     # generated and committed
├── config-*.schema.json         # one generated schema per concern
├── linear-map.schema.json       # generated and committed
└── linear-plan.schema.json      # generated and committed
cmd/
└── golc-project/                # Go developer command entrypoint
internal/
├── bootstrap/                   # downloader/checksum/cache abstractions
├── projectconfig/               # strict TOML, layers, provenance, deprecation
├── contracts/                   # Go source types for emitted JSON Schema
└── trace/
    ├── catalog/                 # durable local-ID graph validation
    ├── reconcile/               # three-way diff and deterministic plans
    └── apply/                   # preconditions, postconditions, resume journal
tools/
└── linear-sync/
    ├── package.json             # exact pins only
    ├── package-lock.json        # committed
    └── src/                     # narrow SDK transport, no merge policy
tests/
├── fixtures/config/             # valid/unknown/duplicate/deprecated/ref cases
├── fixtures/linear/             # paginated/partial/rate/conflict transcripts
└── golden/                      # plans, schemas, provenance output
.planning/
└── linear-map.json              # schema-2 credential-free IDs/mappings/baseline
.github/workflows/check.yml      # calls golc.ps1; PR path never mutates Linear
.env.example                     # names and safe empty/placeholders only
```

### Pattern 1: Bootstrap Shim, Tested CLI

**What:** Keep `golc.ps1` small: locate the repository root, read the exact bootstrap pins, provision the Go/Node archives when `bootstrap` is requested, then delegate all other behavior to `.tools/bin/golc-project.exe`. [RECOMMENDATION]

**When to use:** Every contributor and CI invocation. Do not expose `go test`, `npm run`, or `wails build` as supported top-level workflows even though the root command may invoke them internally. [VERIFIED: CONTEXT.md D-03 and D-10]

**Boundaries:**

- `bootstrap`: network allowed; exact manifests only; staged downloads; SHA-256 before extraction; idempotent when installed hashes match. [RECOMMENDATION]
- `tools update`: network allowed; changes pins/checksums/lockfiles; never installs silently. [VERIFIED: CONTEXT.md D-04]
- `check|generate|build|test|package`: no network after bootstrap; fail with a named missing-cache/tool diagnostic rather than attempting fallback download. [VERIFIED: CONTEXT.md D-02]
- `linear preview|apply|drift --remote`: network allowed only when explicitly selected. [VERIFIED: CONTEXT.md D-16 and D-21]

### Pattern 2: Concern Registry with Field-Level Provenance

**What:** `golc.project.toml` is an index, not a settings dump. It declares a schema version and a fixed list of concern IDs/paths. Each concern owns a non-overlapping namespace and is validated independently before any layers are merged. [RECOMMENDATION]

**Authoritative format allocation:**

| Format | Phase-1 use | Strictness |
|--------|-------------|------------|
| TOML 1.0 | Root index and human-edited concern manifests | Parser rejects duplicate keys/tables; `MetaData.Undecoded()` rejects unknown keys; semantic validator rejects unresolved refs/invalid values. [CITED: https://toml.io/en/v1.0.0] [CITED: https://github.com/BurntSushi/toml] |
| JSON | Generated schemas, Linear mapping, exact preview plan, apply report | Reject duplicate object names before typed decode; generated writers emit stable indentation/order; schemas use Draft 2020-12. [CITED: https://www.rfc-editor.org/rfc/rfc8259] [CITED: https://json-schema.org/draft/2020-12] |
| YAML 1.2 | None in Phase 1; reserved for fixture sources in Phase 2 | Future loader must use the locked strict subset and duplicate-key rejection. [CITED: https://yaml.org/spec/1.2.2/] [VERIFIED: AGENTS.md] |

**Resolution order (low to high):**

1. Committed concern default. [VERIFIED: CONTEXT.md D-06]
2. User configuration at `%APPDATA%\GOLC\config.toml`. [RECOMMENDATION]
3. Ignored repository-local `golc.local.toml`. [RECOMMENDATION]
4. Allowlisted `GOLC_*` environment variable. [VERIFIED: CONTEXT.md D-06]
5. Explicit CLI flag. [VERIFIED: CONTEXT.md D-06]

Do not permit overrides for schema versions, concern paths, tool versions, checksums, generated-output paths, or local identity. These are reproducibility/authority metadata, not runtime preferences. If any higher layer attempts to set a locked key, fail with `CFG_LOCKED_OVERRIDE`. [RECOMMENDATION]

Represent each resolved value as:

```go
type Resolved[T any] struct {
    Value      T
    Key        string
    Layer      Layer       // committed, user, project-local, env, cli
    Source     string      // safe path or variable/flag name; never secret value
    SourceLine int         // when the parser exposes it
    Overridden []Origin    // lower-precedence origins for explain output
}
```

`golc.ps1 config explain runtime.log_level` should show the winning value and source plus shadowed origins. Secret declarations should print `<set>` or `<unset>` only. [VERIFIED: CONTEXT.md D-07 and D-20]

**Duplicate authority rule:** create a registry of canonical keys to one owning concern. A concern may refer to another value through a typed reference such as `toolchain.node.version`, but it may not repeat the value. Validation must detect duplicate key ownership, reference cycles, unresolved references, absolute/root-escaping paths, and final symlink/reparse-point escapes. [RECOMMENDATION]

**Deprecation rule:** keep a machine-readable table of old key, replacement key, introduced/deprecated version, and migration message. Using only the old key emits a stable warning code and provenance; using old and new together is an error; unknown keys are always errors. [VERIFIED: CONTEXT.md D-09]

### Pattern 3: Go Types as Contract Authority

**What:** Define config, mapping, plan, operation, and report types in Go. Generate Draft 2020-12 JSON Schemas from those types, commit the schemas, and make `generate --check` fail on any diff. `invopop/jsonschema` supports reflecting required fields, descriptions, and `additionalProperties: false`. [CITED: https://github.com/invopop/jsonschema]

**When to use:** Phase-1 repository contracts. Do not generate future public API/fixture/script types early; those belong to their owning phases. [VERIFIED: 01-CONTEXT.md Phase Boundary]

The source tree, not generated JSON, is authoritative. Generated files should begin with an explicit generated marker and source package/version. Sort schema definitions and normalize line endings to LF before comparing, so Windows/CI generation is byte-stable. [RECOMMENDATION]

### Pattern 4: Durable Local Identity and Credential-Free Mapping

Extend `.planning/linear-map.json` from schema 1 to schema 2 with an explicit migration that preserves `project:golc` and `milestone:v1` exactly and keeps all remote UUIDs nullable. Never invent a Linear UUID. [VERIFIED: codebase grep; VERIFIED: CONTEXT.md D-14]

Recommended ID grammar:

| Kind | Grammar / Example | Rename behavior |
|------|-------------------|-----------------|
| Repository project | `project:golc` | Never changes. [VERIFIED: existing seed] |
| Release milestone | `milestone:v1` | Never changes when display name changes. [VERIFIED: existing seed] |
| Roadmap phase | `phase:01` | Phase title rename does not change ID. [RECOMMENDATION] |
| Requirement | `requirement:CONF-01` | Requirement text changes do not change ID. [RECOMMENDATION] |
| Plan | `plan:01-01` | Filename/title rename does not change ID. [RECOMMENDATION] |
| Executable task | `task:01-01-01` | Task wording/status changes do not change ID. [RECOMMENDATION] |

Recommended mapping shape:

```json
{
  "schema_version": 2,
  "repository": {
    "local_id": "project:golc",
    "name": "GOLC"
  },
  "entities": {
    "phase:01": {
      "kind": "phase",
      "parent_local_id": "milestone:v1",
      "source": {
        "path": ".planning/ROADMAP.md",
        "anchor": "phase-1-offline-foundation-and-delivery-traceability"
      },
      "linear": {
        "type": "projectMilestone",
        "uuid": null,
        "identifier": null,
        "url": null,
        "status": "pending"
      },
      "sync_base": null
    }
  }
}
```

`source.path` and `source.anchor` are navigation aids, not identity. `linear.uuid` is the only authoritative remote link after explicit create/link/adopt; `identifier` and `url` are mutable display conveniences. Comments are never stored. [VERIFIED: CONTEXT.md D-12 and D-14]

`sync_base` should store normalized last-successful values for fields in the reconciliation contract, separated into `repository_owned` and `linear_owned`. This enables a field-by-field three-way comparison `base -> repository` and `base -> Linear` without copying Linear discussion into planning documents. [RECOMMENDATION]

### Pattern 5: Exact Preview / Resumable Apply

Preview must emit canonical JSON with:

- `schema_version`, `intent_digest`, `mapping_digest`, and `remote_scope_digest`; digests use SHA-256. [RECOMMENDATION]
- A fully sorted, topologically ordered list of operations: Project, Project Milestone, parent/requirement Issue, then task sub-issue; tie-break by local ID. [RECOMMENDATION]
- For every operation: local ID, remote type/UUID or discovery marker, `before`, `after`, owned fields, expected `updatedAt`, and parent preconditions. [RECOMMENDATION]
- Conflict records containing base/repository/Linear values and an explicit resolution command; conflict operations are not applyable. [VERIFIED: CONTEXT.md D-13]
- Archive/unlink operations only when explicitly requested; never a delete operation inferred from absence. [VERIFIED: CONTEXT.md D-15]
- No timestamp/random ID in the hashed plan body, so identical observed states produce byte-identical plan bytes and `plan_id = sha256(canonical_body)`. [RECOMMENDATION]

Apply algorithm:

1. Verify the plan schema/hash and re-hash repository intent. Reject any source change. [VERIFIED: CONTEXT.md D-18]
2. For each operation, re-read its exact current remote fields immediately before mutation and exhaust every discovery page. [CITED: https://linear.app/developers/pagination]
3. If current state equals `after` and the UUID/local marker match, record a no-op success. This makes replay/partial resume safe. [RECOMMENDATION]
4. Otherwise current state must equal `before` on every relevant field and `updatedAt`; any other value is `STALE_PREVIEW` or a three-way conflict. [VERIFIED: CONTEXT.md D-13 and D-18]
5. Execute one mutation, then read back and verify the exact `after` state before atomically updating the credential-free mapping and apply journal. [RECOMMENDATION]
6. If transport fails after a mutation may have succeeded, do not blindly retry. Re-query immutable UUID or exact local-ID marker and compare the postcondition first. [RECOMMENDATION based on GraphQL non-transactionality]
7. On a partial apply, emit completed/no-op/pending/blocked operations and a retry time if rate-limited. Re-running the same plan may accept only an exact prefix of achieved postconditions; unrelated changes invalidate it. [RECOMMENDATION]

This is the minimum behavior needed to make retrying a create/update plan idempotent even though multiple GraphQL mutations are not one remote transaction. [RECOMMENDATION]

### Pattern 6: Remote Marker and Ambiguity Gate

Add a visible, parser-stable footer to every managed Linear description, for example:

```text
---
GOLC local ID: project:golc
GOLC mapping schema: 2
```

When no UUID is mapped, paginate the intended workspace/team/container and parse exact footers:

- zero matches: plan a create; immediately re-run marker discovery before executing it. [RECOMMENDATION]
- one match with the expected kind/parent: plan explicit adopt/link or an exact update. [RECOMMENDATION]
- more than one match: block as `AMBIGUOUS_REMOTE_IDENTITY`; never choose by title. [VERIFIED: CONTEXT.md D-14; RECOMMENDATION]
- title-only candidates: report them for review, but require `linear link --local-id ... --linear-uuid ...`; never auto-adopt. [RECOMMENDATION]

After an issue exists, an attachment with a deterministic repository URL can add a second idempotent link because Linear documents URL idempotency for attachments on an issue. Do not treat attachments as the primary identity because Projects and Project Milestones do not share that issue attachment model. [CITED: https://linear.app/developers/attachments]

### Pattern 7: Exhaustive Linear Transport

The TypeScript adapter should expose narrow commands such as `snapshot`, `createProject`, `createProjectMilestone`, `createIssue`, `update*`, and `archive*`. It should return normalized data plus transport diagnostics; it must not decide ownership/conflicts. [RECOMMENDATION]

Every connection must follow `pageInfo.hasNextPage` and `endCursor` or SDK `fetchNext()` until complete. Linear returns 50 records by default when pagination arguments are absent, so first-page-only code is incorrect. [CITED: https://linear.app/developers/pagination] [CITED: https://linear.app/developers/sdk-fetching-and-modifying-data]

Every response must:

- inspect both `data` and `errors`; HTTP 200 can contain partial success. [CITED: https://linear.app/developers/graphql]
- include `errors[].path`, `extensions.code`, operation name, page count, object count, and a redacted correlation ID in diagnostics. [CITED: https://linear.app/developers/graphql]
- treat partial mutation data as unknown outcome, then reconcile the intended postcondition before retry. [RECOMMENDATION]
- handle `RATELIMITED` and report request/endpoint/complexity remaining/reset headers. The current docs use HTTP 400 for GraphQL rate-limit errors. [CITED: https://linear.app/developers/rate-limiting]
- use bounded retry for read queries and 5xx responses; respect reset time plus jitter. Never blindly retry a mutation. [RECOMMENDATION]
- detect a repeated/null cursor while `hasNextPage=true` and fail as `PAGINATION_INCOMPLETE` rather than returning a partial snapshot. [RECOMMENDATION]

### Pattern 8: Secret and Offline Failure Isolation

`.env.example` should list only supported names and safe placeholders:

```dotenv
# Optional: needed only by explicit Linear remote commands.
LINEAR_API_KEY=
LINEAR_TEAM_ID=
```

`LINEAR_TEAM_ID` is configuration rather than a secret and may ultimately move to the non-secret Linear concern after the workspace taxonomy is selected; no value should be invented. [ASSUMED]

Use Node 24's environment-file support only for the isolated Linear adapter, or pass an already-set process environment. Do not parse/execute `.env` as PowerShell code and do not make the core Go CLI load secrets for unrelated commands. [RECOMMENDATION]

PR CI should always run local trace/config/generation drift checks with no secret. A separate manually dispatched or protected-main job may create an ephemeral `.env` from `${{ secrets.LINEAR_API_KEY }}`, invoke read-only remote drift or explicit apply, and remove the file in a `finally` step. GitHub documents that secrets are only available when explicitly referenced, are absent from fork-triggered workflows, and log redaction is not guaranteed for transformed values. [CITED: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets]

Core commands must not initialize the Linear SDK, read `.env`, resolve DNS, or make HTTP requests. The Linear adapter is launched only for a `linear ... --remote` action. Missing credentials/network return a Linear-specific nonzero result and a remediation message, while local validation/build/test state remains valid. [VERIFIED: CONTEXT.md D-02 and D-21]

### Anti-Patterns to Avoid

- **Global-tool success:** accepting a system Go/Node/Wails version because it happens to be on `PATH`; always run the project-local pin after bootstrap. [RECOMMENDATION]
- **Bootstrap as updater:** consulting `latest` during ordinary bootstrap; only `tools update` may propose version movement. [VERIFIED: CONTEXT.md D-04]
- **Monolithic root manifest:** putting every setting in `golc.project.toml`; keep it as index/authority metadata. [VERIFIED: CONTEXT.md D-05]
- **Environment wildcarding:** mapping arbitrary `GOLC_*` names into keys; maintain an explicit env-name registry so unknown or secret-shaped names cannot become configuration accidentally. [RECOMMENDATION]
- **Title identity:** linking a phase/issue by title or human issue key; immutable local IDs and GraphQL UUIDs own linkage. [VERIFIED: CONTEXT.md D-14]
- **One-shot sync:** fetching, diffing, and mutating in one command; preview/apply must be separate and exact. [VERIFIED: CONTEXT.md D-17 and D-18]
- **HTTP-status-only success:** ignoring GraphQL `errors` when `data` exists. [CITED: https://linear.app/developers/graphql]
- **Delete mirroring:** deleting remote objects because a local artifact disappeared. [VERIFIED: CONTEXT.md D-15]
- **CI mutation on pull requests:** no PR job may execute apply. [VERIFIED: CONTEXT.md D-16]
- **Secret in argv/output:** do not pass the API key as a command argument or serialize it into a plan/report. GitHub recommends environment/stdin over command-line secret arguments. [CITED: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TOML syntax/parser | Ad-hoc line splitting | `github.com/BurntSushi/toml@v1.6.0` | TOML tables, dotted keys, strings, dates, and duplicate definitions have edge cases; the library exposes strict undecoded-key metadata. [CITED: https://github.com/BurntSushi/toml] |
| JSON syntax | A custom JSON parser | Go `encoding/json.Decoder.Token` plus a narrow duplicate-name guard | The standard decoder handles syntax/delimiters; GOLC only adds the stricter unique-name policy required for deterministic maps/plans. [CITED: https://pkg.go.dev/encoding/json] [CITED: https://www.rfc-editor.org/rfc/rfc8259] |
| JSON Schema emitter | String templates for schema files | `github.com/invopop/jsonschema@v0.14.0` | Reflection keeps schema required fields/descriptions aligned with Go contract types. [CITED: https://github.com/invopop/jsonschema] |
| Toolchain verification | Trusting filenames/TLS alone | Committed SHA-256 plus `Get-FileHash`; Go/npm checksum mechanisms | Official Go releases publish SHA-256; Go modules and npm locks carry integrity data. [CITED: https://go.dev/dl/] [CITED: https://go.dev/ref/mod] |
| Linear GraphQL type layer | Raw string queries across the codebase | `@linear/sdk@88.1.0` behind one adapter | Official SDK tracks typed schema models and pagination helpers. [CITED: https://linear.app/developers/graphql] [CITED: https://linear.app/developers/sdk-fetching-and-modifying-data] |
| Remote idempotency assumption | Blind mutation retries | GOLC local-ID markers, pre/postcondition reads, mapping journal | Official documentation found only attachment-URL idempotency, not a general create idempotency key. [CITED: https://linear.app/developers/attachments] |
| Config precedence | Scattered `if env != ""` logic | One resolver returning value plus provenance | Locked order and inspectability must be uniform for every concern. [VERIFIED: CONTEXT.md D-06 and D-07] |
| Secret redaction | Regex-only log scrubbing after output | Structured safe diagnostics, allowlisted fields, secret canary tests, GitHub masking as defense in depth | GitHub states automatic redaction is not guaranteed for transformed values. [CITED: https://docs.github.com/en/actions/concepts/security/secrets] |
| Multi-object remote transaction | Pretending GraphQL mutations are atomic as a batch | Ordered operations, per-op verification, resumable apply report | The reconciliation spans multiple objects and must report partial completion explicitly. [VERIFIED: CONTEXT.md D-21; RECOMMENDATION] |

**Key insight:** parsing, transport, and cryptographic primitives are standard-library/official-package work; GOLC-specific authority, three-way merge, deterministic planning, and retry semantics are the custom domain logic Phase 1 must own.

## Common Pitfalls

### Pitfall 1: Bootstrap Uses the Host Toolchain

**What goes wrong:** A contributor with Go 1.22 or Node 22 passes locally while CI uses the required versions, or Go automatically downloads a newer toolchain. [VERIFIED: environment audit; CITED: https://go.dev/doc/toolchain]

**Why it happens:** Bootstrap finds a command on `PATH` and treats presence as qualification.

**How to avoid:** Use system PowerShell only to fetch verified archives, then call absolute project-local executable paths and set `GOTOOLCHAIN=local`. [RECOMMENDATION]

**Warning signs:** `where go`/logs point outside `.tools`, or offline checks contact `golang.org/toolchain`. [CITED: https://go.dev/doc/toolchain]

### Pitfall 2: Offline Is Claimed but Not Tested

**What goes wrong:** Build/test silently reaches npm, Go proxy/checksum services, or Linear and fails at a venue or on CI cache miss. [VERIFIED: CONTEXT.md D-02]

**Why it happens:** A warm developer machine hides network-dependent resolution.

**How to avoid:** Run the root acceptance suite with `GOPROXY=off`, npm offline settings, a transport that fails on any HTTP attempt, and no `.env`; separately test a missing-cache diagnostic. [CITED: https://go.dev/ref/mod] [RECOMMENDATION]

**Warning signs:** DNS/proxy errors occur under `check|generate|build|test`, or deleting `.tools/cache` triggers a download rather than a named bootstrap error.

### Pitfall 3: Two Concerns Own the Same Setting

**What goes wrong:** Tool version, output path, or runtime default is copied into multiple files and diverges. [VERIFIED: CONTEXT.md D-05 and D-10]

**Why it happens:** Separation by file is mistaken for separation of authority.

**How to avoid:** Register every canonical key to exactly one concern and use typed symbolic references. Validate reference uniqueness and cycles before resolving values. [RECOMMENDATION]

**Warning signs:** The same literal/version appears in multiple concern manifests or CI overrides a shared value differently from local commands.

### Pitfall 4: Unknown and Deprecated Keys Collapse Together

**What goes wrong:** A typo is treated as a deprecated option, or a deprecated key silently wins over its replacement.

**Why it happens:** Parsers only decode into structs and ignore undecoded metadata.

**How to avoid:** Unknown is fatal; deprecated is recognized and warns with code/replacement/origin; old+new together is fatal. BurntSushi TOML exposes undecoded keys for strict checking. [CITED: https://github.com/BurntSushi/toml] [VERIFIED: CONTEXT.md D-09]

**Warning signs:** Changing a key's spelling leaves command behavior unchanged without any diagnostic.

### Pitfall 5: JSON Duplicate Names Change a Plan

**What goes wrong:** A mapping/plan containing two `uuid` or `after` members is accepted with last-value-wins behavior.

**Why it happens:** RFC 8259 says names should be unique, but implementations vary; stable Go `encoding/json` does not provide strict duplicate rejection through `DisallowUnknownFields`. [CITED: https://www.rfc-editor.org/rfc/rfc8259] [CITED: https://pkg.go.dev/encoding/json]

**How to avoid:** Token-scan and reject duplicate names at every object depth before typed decode; fuzz the guard. [RECOMMENDATION]

**Warning signs:** Reformatting the same JSON changes the effective value or hash without an error.

### Pitfall 6: Title-Based Linear Adoption

**What goes wrong:** A rename or duplicate title links a local entity to the wrong remote object.

**Why it happens:** Human titles are easy to query while UUID mappings are initially null.

**How to avoid:** Parse exact GOLC local-ID footers, scope discovery to the expected parent, paginate fully, and require explicit UUID linkage for title-only candidates. [VERIFIED: CONTEXT.md D-14; RECOMMENDATION]

**Warning signs:** A preview proposes `link` because one title matched, or changes `local_id` during rename.

### Pitfall 7: Create Succeeds but Mapping Write Fails

**What goes wrong:** A network timeout/process crash occurs after Linear created an object but before `.planning/linear-map.json` records its UUID; a blind retry creates a duplicate.

**Why it happens:** Local file update and remote mutation cannot share one transaction.

**How to avoid:** Put the local-ID marker in the create payload, discover it before create, verify the postcondition after uncertain outcomes, and atomically journal each completed operation. [RECOMMENDATION]

**Warning signs:** Create retry logic calls `create*` before any read, or a test cannot simulate “remote committed, client timed out.”

### Pitfall 8: First Page Is Treated as the Workspace

**What goes wrong:** The desired object is on a later page, so sync creates a duplicate or reports false absence.

**Why it happens:** Linear defaults to the first 50 results and nested connections multiply complexity. [CITED: https://linear.app/developers/pagination] [CITED: https://linear.app/developers/rate-limiting]

**How to avoid:** Exhaust cursor pages, narrow filters, detect cursor loops, and include `pages_read`, `objects_examined`, and `complete=true` in preview diagnostics. [RECOMMENDATION]

**Warning signs:** SDK calls never invoke `fetchNext()`, or a snapshot has no explicit completeness flag.

### Pitfall 9: Partial GraphQL Data Is Success

**What goes wrong:** Code uses returned `data` while ignoring `errors`, producing a plan from an incomplete snapshot or assuming a mutation completed.

**Why it happens:** HTTP 200 is used as the only success condition.

**How to avoid:** Normalize `data` and `errors` together; any snapshot error blocks planning, and any mutation error triggers a postcondition read before retry. [CITED: https://linear.app/developers/graphql]

**Warning signs:** Error handling begins only in `catch`, or tests omit `{data, errors}` responses.

### Pitfall 10: Rate-Limit Retry Creates a Mutation Storm

**What goes wrong:** Generic exponential retry repeats writes or ignores Linear's reset window/complexity budget.

**Why it happens:** Rate handling treats all requests as safe reads.

**How to avoid:** Parse `RATELIMITED`, request/endpoint/complexity headers, stop writes, emit retry-at, and postcondition-check before any mutation retry. [CITED: https://linear.app/developers/rate-limiting]

**Warning signs:** A mutation retry loop has no remote read or plan-operation identity.

### Pitfall 11: Secret Appears in “Helpful” Diagnostics

**What goes wrong:** The API key is printed in config provenance, GraphQL headers, exception serialization, preview JSON, or ephemeral-file commands.

**Why it happens:** Logging generic request/config objects and relying on platform masking.

**How to avoid:** Diagnostics use an allowlist; secret fields are represented only as `set/unset`; never pass values in argv; use a unique canary secret in every test and scan all outputs/artifacts. GitHub warns redaction is not guaranteed. [CITED: https://docs.github.com/en/actions/concepts/security/secrets]

**Warning signs:** Any code path formats process environment, request headers, or full config objects.

### Pitfall 12: PR CI Mutates Linear

**What goes wrong:** A contributor branch changes remote project/issue state or exposes a privileged token.

**Why it happens:** Local `linear apply` and CI drift checks share one script/credential path.

**How to avoid:** Require explicit `apply <plan-file> --plan-id <hash>`, refuse apply when `CI_EVENT=pull_request`, and keep protected apply in a separate environment/manual workflow. [VERIFIED: CONTEXT.md D-16; RECOMMENDATION]

**Warning signs:** PR YAML references `LINEAR_API_KEY` or has a path that can reach a mutation command.

### Pitfall 13: Root Index Escapes the Repository

**What goes wrong:** A concern reference points to an absolute path, `..`, symlink, or Windows reparse point outside the repository.

**Why it happens:** The root index treats file paths as trusted strings.

**How to avoid:** Permit only normalized relative paths from a fixed concern allowlist, resolve final paths, and require committed concern files to remain within the repository. [RECOMMENDATION]

**Warning signs:** `config validate` reads from a user profile/temp directory through a root-manifest path.

## Code Examples

Verified library patterns and phase-specific pseudocode follow. Recommendations are illustrative contracts for the planner, not code already present in the repository.

### Strict TOML Decode

```go
// Source: https://github.com/BurntSushi/toml
func decodeConcern(data []byte, out any) error {
    md, err := toml.Decode(string(data), out)
    if err != nil {
        return fmt.Errorf("CFG_PARSE: %w", err)
    }
    if keys := md.Undecoded(); len(keys) != 0 {
        return fmt.Errorf("CFG_UNKNOWN_KEY: %v", keys)
    }
    return nil
}
```

`MetaData.Undecoded()` returns keys not consumed by the target Go value and is the documented basis for strict configuration validation. [CITED: https://github.com/BurntSushi/toml]

### Project-Local Wails Install

```powershell
# Source: https://github.com/wailsapp/wails/releases/tag/v2.13.0
$env:GOTOOLCHAIN = "local"
$env:GOBIN = (Join-Path $RepoRoot ".tools\bin")
& $ProjectGo install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

The official v2.13.0 release gives the exact module-version install command. [CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0]

### Exhaust a Linear SDK Connection

```typescript
// Source: https://linear.app/developers/sdk-fetching-and-modifying-data
let page = await linearClient.issues({ first: 50 });
const nodes = [...page.nodes];

while (page.pageInfo.hasNextPage) {
  page = await page.fetchNext();
  nodes.push(...page.nodes);
}
```

The SDK documents both `pageInfo.endCursor`/manual pagination and `fetchNext()`. The implementation must additionally detect cursor repetition and expose completeness diagnostics. [CITED: https://linear.app/developers/sdk-fetching-and-modifying-data]

### Deterministic Preview Operation

```go
// Recommendation: model current and desired state explicitly.
type Operation struct {
    Kind              string          `json:"kind"`
    LocalID           string          `json:"local_id"`
    LinearType        string          `json:"linear_type"`
    LinearUUID        *string         `json:"linear_uuid"`
    DiscoveryMarker   string          `json:"discovery_marker"`
    ExpectedUpdatedAt *time.Time      `json:"expected_updated_at"`
    Before            json.RawMessage `json:"before"`
    After             json.RawMessage `json:"after"`
    DependsOn         []string        `json:"depends_on"`
}
```

Sort `DependsOn`, operations, and any map-derived slices before `json.Marshal`; hash the canonical plan body with Go `crypto/sha256`. Go's JSON encoder sorts map keys, but explicit sorted slices keep semantic ordering independent of input traversal. [CITED: https://pkg.go.dev/encoding/json] [CITED: https://pkg.go.dev/crypto/sha256]

### Apply Precondition/Postcondition Decision

```go
// Recommendation: safe replay after partial apply.
switch {
case equalRelevantFields(current, op.After) && identityMatches(current, op):
    return markNoopComplete(op)
case equalRelevantFields(current, op.Before) && updatedAtMatches(current, op):
    result, err := mutateOnce(ctx, op)
    if err != nil {
        return reconcileUnknownOutcome(ctx, op)
    }
    return readBackAndCommitMapping(ctx, op, result)
default:
    return stalePreview(op, current)
}
```

This decision implements locked stale-preview rejection while accepting an already-achieved exact postcondition during replay. [VERIFIED: CONTEXT.md D-17 and D-18; RECOMMENDATION]

### GitHub Actions Ephemeral Secret File

```yaml
# Source basis: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets
- name: Read-only Linear drift
  if: github.event_name != 'pull_request'
  shell: powershell
  env:
    LINEAR_API_KEY_FROM_STORE: ${{ secrets.LINEAR_API_KEY }}
  run: |
    try {
      "LINEAR_API_KEY=$env:LINEAR_API_KEY_FROM_STORE" |
        Set-Content -LiteralPath .env -Encoding utf8
      .\golc.ps1 linear drift --remote --read-only
    } finally {
      Remove-Item -LiteralPath .env -Force -ErrorAction SilentlyContinue
    }
```

The real workflow must add masking before any derived sensitive value, minimize permissions, and ensure pull-request jobs do not reference the protected secret. GitHub notes that secrets are not passed to fork-triggered workflows and recommends environment variables over command-line secret arguments. [CITED: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets]

## State of the Art

| Old Approach | Current Approach for GOLC | When / Why | Impact |
|--------------|---------------------------|------------|--------|
| Global SDK/tool installs | Verified project-local toolchains and caches | Exact official archives plus Go/npm integrity mechanisms are available. [CITED: https://go.dev/dl/] [CITED: https://go.dev/ref/mod] | Contributor and CI commands resolve the same executables. |
| `go install ...@latest` | `go install ...@v2.13.0` | Wails v2.13.0 publishes an exact install command. [CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0] | Bootstrap cannot silently change CLI behavior. |
| Automatic Go toolchain switching | Project archive plus `GOTOOLCHAIN=local` | Go automatic selection can download toolchain modules. [CITED: https://go.dev/doc/toolchain] | Offline checks fail early instead of contacting the network. |
| One monolithic settings file | Root index plus independently validated concerns | Locked Phase-1 configuration model. [VERIFIED: CONTEXT.md D-05 and D-10] | One discovery point without duplicated authority. |
| Final-value-only config output | Value plus layer/source/shadowed origins | Locked provenance requirement. [VERIFIED: CONTEXT.md D-07] | Contributors can explain why CI/local differ. |
| Linear title/issue-key matching | Durable local ID plus immutable GraphQL UUID | Locked rename behavior; Linear exposes UUID `id` separately from issue `identifier`. [VERIFIED: CONTEXT.md D-14] [CITED: https://linear.app/developers/pagination] | Renames cannot re-identify an object. |
| First-page GraphQL reads | Exhaustive Relay cursor traversal | Linear list responses are paginated, with default first 50. [CITED: https://linear.app/developers/pagination] | Absence/ambiguity decisions use complete scope. |
| HTTP-status-only GraphQL handling | Inspect `data` and `errors`, then postcondition-read unknown writes | Linear documents partial data/errors on HTTP 200. [CITED: https://linear.app/developers/graphql] | Partial failures cannot silently corrupt the plan. |
| One-step sync | Deterministic preview artifact plus exact resumable apply | Locked decisions D-17/D-18. [VERIFIED: CONTEXT.md] | Human review and stale-state safety become testable. |
| Automatic deletion mirroring | Explicit archive/unlink operations | Locked decision D-15. [VERIFIED: CONTEXT.md] | Removing a local artifact cannot destroy remote work. |

**Deprecated/outdated:**

- Floating `latest` in bootstrap/CI is prohibited by D-04 and AGENTS.md. [VERIFIED: project constraints]
- Treating `.planning/linear-map.json` schema 1's pending remote UUID as completed synchronization is prohibited; migrate it explicitly while preserving local IDs. [VERIFIED: existing seed; VERIFIED: STATE.md]
- Wails NSIS application packaging is not Phase-1 acceptance. Wails documents `wails build -nsis`, but product UI/installer work belongs to later phases and NSIS is not installed on the audited host. [CITED: https://wails.io/docs/guides/windows-installer/] [VERIFIED: environment audit]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `LINEAR_TEAM_ID` may begin in `.env.example` but should move to committed non-secret Linear concern configuration after taxonomy selection. [ASSUMED] | Secret and Offline Failure Isolation | Low: placement changes, not identity or secret handling. |
| A2 | Linear's live schema may expose newer object-specific upsert/idempotency capabilities not surfaced by the official sources used here; the selected CI image and final Linear taxonomy also require confirmation. [ASSUMED] | Metadata / Open Questions | Medium: an implementation could miss a safer API primitive or encode the wrong remote taxonomy/image assumption. |

All other implementation choices are either locked decisions, verified codebase facts, cited official behavior, or explicit recommendations for the planner.

## Open Questions (RESOLVED)

All four planning seams are resolved below; external Linear values remain deliberately unset rather than unresolved implementation choices.

1. **RESOLVED - Linear taxonomy remains external, reviewed configuration.**
   - What we know: the release maps to a Linear Project, the phase maps to a Project Milestone, requirements/plans/tasks map to issues/sub-issues. [VERIFIED: AGENTS.md]
   - Resolution: workspace/team UUIDs, workflow states, labels, and credential choice remain external reviewed configuration; no value is inspected, defaulted, or invented. [VERIFIED: STATE.md; environment boundary]
   - Implementation contract: test the complete hierarchy against a fake SDK/service and require explicit `linear configure/link` review before the first real preview/apply.

2. **RESOLVED - use a visible, validated local-ID footer on every managed Linear object.**
   - What we know: Issue descriptions are available through the API, and Projects/Project Milestones expose description-like fields in the current SDK schema. [CITED: https://linear.app/developers/graphql] [CITED: https://github.com/linear/linear]
   - Resolution: use a visible parser-stable footer for all managed types and validate preservation before accepting the remote snapshot.
   - Implementation contract: retain exhaustive scoped pagination plus explicit ambiguity handling even after footer preservation validates.

3. **RESOLVED - foundation packaging produces a deterministic Windows AMD64 ZIP, canonical manifest, and SHA-256 file.**
   - What we know: CONF-01/03 require a discoverable, shared packaging entrypoint; the phase boundary excludes application UI and Wails product behavior. [VERIFIED: REQUIREMENTS.md; VERIFIED: CONTEXT.md]
   - Resolution: `package --foundation` creates a deterministic Windows AMD64 developer-tool ZIP, canonical manifest, and SHA-256 file.
   - Implementation contract: explicitly report that app/NSIS packaging is unavailable until its owning phase.

4. **RESOLVED - use warmed project-local Go/npm caches without committing vendored dependency trees.**
   - What we know: `-mod=vendor` avoids both network and module cache, while project-local module caches satisfy the locked “after bootstrap” offline condition. [CITED: https://go.dev/ref/mod]
   - Resolution: bootstrap warms verified project-local Go/npm caches, and dependency trees are not committed as vendored source.
   - Implementation contract: clean-clone-without-network is not claimed; the supported offline boundary begins after one verified bootstrap.

## Thinnest Phase-1 Vertical Walking Skeleton

Plan the phase as one end-to-end slice, then deepen tests—not as separate “configuration” and “Linear” projects:

1. **Root command boots itself:** `golc.ps1 bootstrap` installs verified project-local Go/Node, builds the Go command/TS adapter, warms caches, and ends with `check --offline`. [RECOMMENDATION]
2. **One real concern resolves end to end:** `golc.project.toml` indexes `config/toolchain.toml`; `config validate` rejects unknown/duplicate/unresolved values; `config explain` prints provenance through all five layers. Add the remaining concern files using the same registry. [RECOMMENDATION]
3. **Generation is a real drift gate:** Go contract types generate and check the root, concern, Linear map, and Linear plan schemas. [RECOMMENDATION]
4. **Build/test/package are real but phase-bounded:** build the developer command and Linear adapter, run their tests, and package a deterministic foundation-tool ZIP/checksum. Do not scaffold a Wails UI, lighting domain, or NSIS product installer. [VERIFIED: phase boundary; RECOMMENDATION]
5. **Local traceability works with no Linear:** migrate the existing seed to schema 2, add all Phase-1 entity IDs, validate uniqueness/parents/source references, and show pending remote mappings without error. [VERIFIED: existing seed; RECOMMENDATION]
6. **Linear behavior is proven against a fake service:** one fixture graph covers project -> milestone -> requirement/plan issue -> task sub-issue; preview is byte-stable; apply is safe across pagination, partial errors, rate limits, ambiguity, stale state, and “remote committed/client timed out.” [RECOMMENDATION]
7. **Real Linear remains an explicit checkpoint:** after taxonomy and credentials are supplied outside Git, run preview, review exact operations, then separately apply; automated tests never mutate a real workspace. [VERIFIED: CONTEXT.md D-16 through D-20]
8. **CI calls only root commands:** PR workflow runs bootstrap/check/generate/build/test/package and local Linear drift; protected manual/trusted workflow owns optional remote read/apply. [VERIFIED: CONTEXT.md D-10 and D-16]

This walking skeleton satisfies all eight requirements without pulling forward Wails UI, product packaging, fixtures, playback, Art-Net, storage, scripts, or AI. [VERIFIED: ROADMAP.md Phase 1 boundary]

## Environment Availability

No secret, `.env`, API key, credential, or Linear workspace was inspected. Remote Linear availability is intentionally unknown. [VERIFIED: research boundary]

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Windows PowerShell | Clean-checkout root shim | ✓ | 5.1.19041.6456 | None needed for Windows v1. [VERIFIED: environment audit] |
| Go | Build/test/bootstrap compiler | ✓, wrong version | 1.22.2 vs required 1.26.5 | Bootstrap official 1.26.5 ZIP into `.tools`; do not use host Go afterward. [VERIFIED: environment audit; CITED: https://go.dev/dl/] |
| Node.js | Linear adapter build/test | ✓, wrong version | 22.19.0 vs required 24.18.0 | Bootstrap official Node 24.18.0 archive into `.tools`. [VERIFIED: environment audit; CITED: https://nodejs.org/en/about/previous-releases] |
| npm | Initial exact dependency install/cache warm | ✓, host-only | 10.9.3 | Use npm bundled with project Node and committed lockfile after bootstrap. [VERIFIED: environment audit] |
| Git | Checkout/diff/update review | ✓ | 2.51.1.windows.1 | Root commands should not require Git for ordinary config resolution, but update/drift reporting may degrade explicitly. [VERIFIED: environment audit; RECOMMENDATION] |
| Wails CLI | Future desktop build pin/discovery | ✗ | — | Bootstrap exact v2.13.0 into `.tools/bin`; no Wails app is built in Phase 1. [VERIFIED: environment audit; CITED: https://github.com/wailsapp/wails/releases/tag/v2.13.0] |
| NSIS `makensis` | Future Windows product installer | ✗ | — | Not required for Phase-1 foundation package; defer product installer acceptance. [VERIFIED: environment audit; CITED: https://wails.io/docs/guides/windows-installer/] |
| `pwsh` (PowerShell 7) | Optional modern shell | ✗ | — | Use Windows PowerShell 5.1-compatible syntax in the root shim. [VERIFIED: environment audit] |
| Linear credentials/service | Explicit remote preview/apply only | Not probed | — | Local trace validation and fake-server tests; remote command reports unavailable without affecting core work. [VERIFIED: research boundary; VERIFIED: CONTEXT.md D-21] |

**Missing dependencies with no fallback:** none for local Phase-1 implementation after bootstrap; the first successful bootstrap necessarily requires network access to official tool/package sources. [VERIFIED: CONTEXT.md D-01 and D-02]

**Missing dependencies with fallback:** pinned Go, Node, Wails are provisioned project-locally; NSIS is deferred; real Linear is replaced by local validation/fakes until explicitly configured. [RECOMMENDATION]

## Validation Architecture

Nyquist validation is enabled and security enforcement is enabled at ASVS Level 1 in `.planning/config.json`. The repository currently contains no Go/Node manifests, source, test config, or test files, so all test infrastructure below is Wave 0. [VERIFIED: config.json; VERIFIED: codebase scan]

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` from project-local Go 1.26.5 plus Node 24.18.0 built-in `node:test`. [CITED: https://pkg.go.dev/testing] [CITED: https://nodejs.org/api/test.html] |
| Config file | None — see Wave 0. [VERIFIED: codebase scan] |
| Quick run command | `powershell -NoProfile -File .\golc.ps1 test --quick` |
| Full suite command | `powershell -NoProfile -File .\golc.ps1 test` |
| Offline acceptance command | `powershell -NoProfile -File .\golc.ps1 check --offline` |
| Generation drift command | `powershell -NoProfile -File .\golc.ps1 generate --check` |

The supported commands above are requirements contracts; the Go/Node commands beneath them remain implementation details. [VERIFIED: CONTEXT.md D-03 and D-10]

### Test Layers

| Layer | Scope | Required cases |
|-------|-------|----------------|
| Go unit/table tests | Config models/resolver/deprecation/path safety, ID grammar/graph, plan diff/apply decisions | Every precedence pair; locked override; duplicate authority; unknown/deprecated old+new; cycle/unresolved/root escape; rename stability; conflict matrix; exact before/after decision. [RECOMMENDATION] |
| Go fuzz tests | Strict JSON duplicate guard and map/plan decoding | Nested objects/arrays, repeated Unicode-equivalent bytes, malformed delimiters, multiple top-level values, huge numbers, unknown fields. [CITED: https://pkg.go.dev/testing] [RECOMMENDATION] |
| Node unit tests | SDK adapter normalization | Connection page exhaustion, cursor-loop failure, `data+errors`, `RATELIMITED`, header capture/redaction, timeout/5xx, no mutation retry. [CITED: https://linear.app/developers/graphql] [CITED: https://linear.app/developers/rate-limiting] |
| Fake GraphQL integration | Go plan engine + Node adapter | Project/milestone/issues/sub-issues; 51+ entities; ambiguity; stale update; timeout-after-commit; partial apply/resume; explicit archive/unlink; no delete. [RECOMMENDATION] |
| Golden tests | Schemas, effective config provenance, preview plan, apply report | Byte-identical regeneration and repeat preview; LF normalization; no timestamps/randomness in plan body. [RECOMMENDATION] |
| Root-command acceptance | `golc.ps1` and project-local tool paths | Clean bootstrap fixture, idempotent second bootstrap, checksum mismatch, missing cache offline, identical local/CI entrypoints. [RECOMMENDATION] |
| Secret canary | All stdout/stderr/files/artifacts | Unique fake token must never appear in plan, report, map, logs, error objects, command line, or generated schemas. [CITED: https://docs.github.com/en/actions/concepts/security/secrets] [RECOMMENDATION] |
| Offline acceptance | Core command graph | Inject a fail-on-network transport, set `GOPROXY=off`/npm offline, remove credentials, then run generate/check/build/test/package/local trace. [CITED: https://go.dev/ref/mod] [RECOMMENDATION] |
| Manual real-service smoke | Explicitly configured Linear workspace | Read-only schema/taxonomy check, reviewed preview, one approved apply/replay, then map/remote verification. Never part of automated PR tests. [VERIFIED: CONTEXT.md D-16 and D-17] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONF-01 | Root index discovers pins and all command/config entrypoints | acceptance + schema | `.\golc.ps1 check --concern project` | ❌ Wave 0 |
| CONF-02 | Concerns validate independently with one authority/reference graph | unit + golden | `.\golc.ps1 test --quick --scope config` | ❌ Wave 0 |
| CONF-03 | Contributor and CI invoke identical generate/check/build/test/package commands | acceptance | `.\golc.ps1 check --command-parity` | ❌ Wave 0 |
| CONF-04 | No committed/machine secret value; names/examples are safe | canary + repository scan | `.\golc.ps1 test --quick --scope secrets` | ❌ Wave 0 |
| LINR-01 | Every entity kind has unique stable local ID and valid parent/source | unit + fixture | `.\golc.ps1 linear validate --offline` | ❌ Wave 0 |
| LINR-02 | Credential-free map validates nullable UUIDs and preserves local truth | schema + migration | `.\golc.ps1 test --quick --scope linear-map` | ❌ Wave 0 |
| LINR-03 | Preview/apply is exact, stale-safe, replayable, and duplicate-free | fake GraphQL integration + golden | `.\golc.ps1 test --quick --scope linear-reconcile` | ❌ Wave 0 |
| LINR-04 | Pages/errors/rate/ambiguity are complete and isolated | Node contract + Go integration | `.\golc.ps1 test --quick --scope linear-transport` | ❌ Wave 0 |

### Required Acceptance Scenarios

1. Bootstrap from a fixture with no project toolchain; corrupt download hash must leave no promoted install, and rerun with correct bytes succeeds. [RECOMMENDATION]
2. Second bootstrap with identical manifests makes no network call and changes no file. [VERIFIED: CONTEXT.md D-04; RECOMMENDATION]
3. After bootstrap, core commands pass with the network transport set to fail immediately and no `.env`. [VERIFIED: CONTEXT.md D-02]
4. Every configuration layer can win over lower layers; locked fields reject all override attempts; `config explain` shows exact safe provenance. [VERIFIED: CONTEXT.md D-06 and D-07]
5. Unknown key, TOML duplicate, duplicate authority, invalid value, unresolved/cyclic reference, root escape, and deprecated old+new all produce stable actionable codes. [VERIFIED: CONTEXT.md D-09]
6. Renaming every display title/text retains local IDs and mapped UUIDs. [VERIFIED: CONTEXT.md D-14]
7. A 51-item remote collection locates the marker on page two and produces no create. [CITED: https://linear.app/developers/pagination]
8. A response with valid `data` plus `errors` blocks preview as incomplete. [CITED: https://linear.app/developers/graphql]
9. A create commits remotely, client receives timeout, mapping remains null; replay discovers the marker and records/adopts the one UUID without a second create. [RECOMMENDATION]
10. Apply operation 1 succeeds, operation 2 rate-limits; report contains completed/pending/retry-at, and the exact same plan resumes operation 2 without replaying operation 1. [CITED: https://linear.app/developers/rate-limiting] [RECOMMENDATION]
11. Relevant repository or remote field changes after preview; apply rejects with field-level stale/conflict output. [VERIFIED: CONTEXT.md D-13 and D-18]
12. Local removal proposes no delete; only an explicitly requested archive/unlink appears in a reviewed plan. [VERIFIED: CONTEXT.md D-15]
13. A fake secret canary never appears in any emitted byte, including errors from partial GraphQL responses. [VERIFIED: CONTEXT.md D-20]

### Sampling Rate

- **Per task commit:** `powershell -NoProfile -File .\golc.ps1 test --quick`
- **Per wave merge:** `powershell -NoProfile -File .\golc.ps1 generate --check; powershell -NoProfile -File .\golc.ps1 check --offline; powershell -NoProfile -File .\golc.ps1 test`
- **Phase gate:** generation clean, offline check green, full tests green, foundation build/package reproducible, and one human-reviewed real Linear preview/apply/replay if credentials/taxonomy are available. If Linear is unavailable, record the remote smoke as pending without failing offline requirement acceptance. [VERIFIED: CONTEXT.md D-21]

### Wave 0 Gaps

- [ ] `go.mod` / `go.sum` — Go command, TOML parser, schema generator, tests.
- [ ] `cmd/golc-project/main.go` — normal command entrypoint.
- [ ] `internal/projectconfig/*_test.go` — strict concerns, precedence, provenance, deprecation, path safety.
- [ ] `internal/trace/catalog/*_test.go` — local identity graph and schema-1-to-2 migration.
- [ ] `internal/trace/reconcile/*_test.go` — three-way merge and canonical plan goldens.
- [ ] `internal/trace/apply/*_test.go` — stale/replay/partial apply state machine.
- [ ] `internal/contracts/generate_test.go` — schema generation/drift.
- [ ] `tools/linear-sync/package.json` / exact lockfile / `tsconfig.json`.
- [ ] `tools/linear-sync/test/*.test.ts` — pagination, partial errors, rate limits, mutation uncertainty, redaction.
- [ ] `tests/fixtures/config/` and `tests/fixtures/linear/` — adversarial inputs/transcripts.
- [ ] `tests/golden/` — schemas, provenance, plans, reports.
- [ ] `.github/workflows/check.yml` — Windows same-command CI with no PR mutation.

## Security Domain

Security enforcement is enabled, so Phase 1 must treat repository/config files, environment variables, downloaded archives, GraphQL responses, and preview artifacts as untrusted inputs. [VERIFIED: .planning/config.json]

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Limited | Linear credential is external and used only by explicit adapter commands; no product authentication is introduced. [VERIFIED: phase boundary] |
| V3 Session Management | No | Phase 1 creates no user session or product server. [VERIFIED: phase boundary] |
| V4 Access Control | Yes | PR CI is read-only; apply requires explicit command/plan ID and a protected credential context; archive/unlink requires explicit review. [VERIFIED: CONTEXT.md D-15 through D-17] |
| V5 Validation, Sanitization and Encoding | Yes | Strict TOML/JSON/schema validation, allowlisted environment/flags, path containment, typed GraphQL normalization. [CITED: https://owasp.org/www-project-application-security-verification-standard/] |
| V6 Stored Cryptography | Yes | Use standard SHA-256/integrity mechanisms for archive and plan verification; never invent cryptography. [CITED: https://pkg.go.dev/crypto/sha256] |
| V7 Error Handling and Logging | Yes | Structured allowlisted diagnostics; never serialize secrets/headers/environment; distinguish partial errors/rate limits safely. [CITED: https://linear.app/developers/graphql] [CITED: https://docs.github.com/en/actions/concepts/security/secrets] |
| V8 Data Protection | Yes | Commit only safe examples and credential-free mappings; ephemeral CI `.env` is removed in `finally`. [VERIFIED: CONTEXT.md D-19 and D-20] |
| V10 Malicious Code | Yes | Exact package/tool versions, lockfiles, registry integrity, Go sums, archive checksums, postinstall audit. [CITED: https://go.dev/ref/mod] [VERIFIED: package audit] |
| V12 Files and Resources | Yes | Root-contained concern paths, staged extraction, atomic mapping/report writes, no automatic remote delete. [VERIFIED: CONTEXT.md D-15; RECOMMENDATION] |
| V13 API and Web Service | Yes | Exhaustive pagination, partial GraphQL error handling, rate limits, timeouts, bounded retries, postcondition reads. [CITED: https://linear.app/developers/graphql] [CITED: https://linear.app/developers/rate-limiting] |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tool archive/package substitution | Tampering | Official source allowlist, committed SHA-256, npm integrity, Go sums, no floating versions. [CITED: https://go.dev/ref/mod] |
| Root-manifest path/reparse escape | Tampering / Information Disclosure | Relative allowlist, canonical/final path containment, reject absolute/`..`/external link. [RECOMMENDATION] |
| Environment/CLI override of locked pins or identity | Tampering | Typed allowlist and non-overridable key class; provenance output. [VERIFIED: CONTEXT.md D-06 through D-09] |
| API key in logs, argv, plan, mapping, artifact | Information Disclosure | Secret-field allowlist, set/unset display, environment rather than argv, unique canary scan, ephemeral file cleanup. [CITED: https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets] |
| Forged/stale preview applied to changed state | Tampering / Repudiation | Plan SHA-256, source/remote preconditions, expected `updatedAt`, field-level conflicts, exact postcondition replay. [VERIFIED: CONTEXT.md D-13 and D-18] |
| Duplicate remote object after ambiguous timeout | Tampering / Denial of Service | Exact local-ID footer discovery before create/retry, postcondition read, atomic journal/mapping update. [RECOMMENDATION] |
| Hidden later GraphQL pages | Tampering / Spoofing | Exhaust all Relay pages, cursor-loop detection, completeness flag. [CITED: https://linear.app/developers/pagination] |
| Partial GraphQL success accepted | Tampering | Inspect `data` and `errors`; block incomplete snapshots; reconcile unknown mutation outcomes. [CITED: https://linear.app/developers/graphql] |
| Rate-limit mutation storm | Denial of Service | Parse `RATELIMITED`/reset headers, stop writes, bounded read retry, no blind mutation retry. [CITED: https://linear.app/developers/rate-limiting] |
| PR-triggered remote mutation | Elevation of Privilege / Tampering | Separate no-secret PR workflow; protected/manual apply; runtime guard on CI event. [VERIFIED: CONTEXT.md D-16] |
| Generated artifact drift | Tampering | Commit schemas/contracts, deterministic generation, `generate --check` in CI. [VERIFIED: CONTEXT.md D-08] |

## Sources

### Official and Context7-Resolved (MEDIUM confidence per research seam)

- Context7 `/websites/linear_app_developers` and `/linear/linear` — SDK pagination, GraphQL mutations, generated input fields, Projects/Project Milestones/Issues/sub-issues.
- Context7 `/websites/wails_io` and `/wailsapp/wails` — Wails v2 installation/build/NSIS behavior.
- Context7 `/golang/go/go1.26.0` and `/websites/go_dev_doc` — module cache/vendor/toolchain and JSON token/decoder behavior.
- Context7 `/burntsushi/toml` — strict decode with `MetaData.Undecoded()`.
- Context7 `/invopop/jsonschema` — Draft 2020-12 schema reflection from Go types.
- [Go 1.26.5 downloads and checksums](https://go.dev/dl/) — exact Windows toolchain archive/hash.
- [Go Modules Reference](https://go.dev/ref/mod) — module cache, checksums, `GOPROXY=off`, `-mod=vendor`, readonly behavior.
- [Go Toolchains](https://go.dev/doc/toolchain) — automatic toolchain downloads and `GOTOOLCHAIN`.
- [Go encoding/json](https://pkg.go.dev/encoding/json) — `DisallowUnknownFields`, token API, sorted map-key encoding.
- [Wails v2.13.0 release](https://github.com/wailsapp/wails/releases/tag/v2.13.0) — exact CLI install command.
- [Wails Windows NSIS guide](https://wails.io/docs/guides/windows-installer/) — later product installer command/dependency.
- [Node release status](https://nodejs.org/en/about/previous-releases) — Node 24 LTS.
- [TOML 1.0 specification](https://toml.io/en/v1.0.0) — duplicate key/table definitions are invalid.
- [BurntSushi TOML](https://github.com/BurntSushi/toml) — strict undecoded-key pattern.
- [Invopop JSON Schema](https://github.com/invopop/jsonschema) — Go reflection to Draft 2020-12 schemas.
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12) — schema contract version.
- [RFC 8259 JSON](https://www.rfc-editor.org/rfc/rfc8259) — object-name uniqueness/interoperability behavior.
- [YAML 1.2.2](https://yaml.org/spec/1.2.2/) — unique mapping keys and feature surface.
- [Linear GraphQL getting started/error handling](https://linear.app/developers/graphql) — endpoint, auth forms, partial errors.
- [Linear pagination](https://linear.app/developers/pagination) — Relay cursors, default 50, `updatedAt` ordering.
- [Linear SDK fetching/modifying data](https://linear.app/developers/sdk-fetching-and-modifying-data) — SDK `fetchNext()`, create/update patterns.
- [Linear rate limiting](https://linear.app/developers/rate-limiting) — `RATELIMITED`, HTTP 400, request/endpoint/complexity headers.
- [Linear attachments](https://linear.app/developers/attachments) — the specifically documented attachment URL idempotency behavior.
- [GitHub Actions secrets](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets) — secret contexts, fork behavior, masking, environment-vs-argv guidance.
- [GitHub secret concepts](https://docs.github.com/en/actions/concepts/security/secrets) — redaction limits and least privilege.
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/) — applicable verification categories.

### Repository Sources (HIGH confidence)

- `.planning/phases/01-offline-foundation-and-delivery-traceability/01-CONTEXT.md` — locked decisions/boundary.
- `.planning/REQUIREMENTS.md` — CONF-01..04 and LINR-01..04.
- `.planning/ROADMAP.md` — Phase-1 goal/success criteria and deferred product scope.
- `.planning/STATE.md` — pending remote mappings and offline authority.
- `.planning/PROJECT.md` and `AGENTS.md` — stack/project constraints.
- `.planning/linear-map.json` — schema-1 seed with `project:golc`, `milestone:v1`, null remote mapping.
- `.planning/config.json` — Nyquist/security workflow settings.
- `.gitignore` — existing `.env` exclusion.

### Registry / Environment Verification

- `npm view @linear/sdk@88.1.0 ...` and `npm view typescript@7.0.2 ...` — exact version/integrity/publish metadata and no postinstall.
- GSD `package-legitimacy check --ecosystem npm @linear/sdk typescript` — both SUS solely on release recency.
- `go list -m -json` — Wails v2.13.0, BurntSushi TOML v1.6.0, Invopop JSONSchema v0.14.0 source/tag metadata.
- Local command probes — PowerShell, Go, Node, npm, Git, Wails, NSIS availability; no credential/service probe.

## Metadata

**Confidence breakdown:**

- Standard stack: **MEDIUM-HIGH** — project pins and exact versions were verified through official sources/registries; npm legitimacy seam requires human review because current releases are recent.
- Architecture: **MEDIUM** — configuration and offline patterns are grounded in official tooling, but exact repository layout/command internals are Phase-1 recommendations.
- Linear transport facts: **MEDIUM-HIGH** — official docs and Context7 agree on pagination, partial errors, mutations, and rate-limit reporting.
- Linear idempotency/reconciliation: **MEDIUM** — retry/stale/conflict design is an implementation recommendation; it must be proven with fake-service fault tests and one reviewed real workspace exercise.
- Pitfalls/security: **MEDIUM-HIGH** — official platform behavior plus direct application of locked safety decisions.

**What might have been missed:** Linear's live schema may offer newer object-specific upsert/idempotency features not surfaced in the official pages/Context7 results used here; verify schema introspection read-only before implementation, but do not weaken marker/precondition safeguards without authoritative evidence. The selected CI image name and Linear taxonomy also require project-owner confirmation. [ASSUMED]

**Research date:** 2026-07-17  
**Valid until:** 2026-07-24 — Linear SDK/API and toolchain releases are fast-moving; re-check exact versions/rate-limit docs before installation.
