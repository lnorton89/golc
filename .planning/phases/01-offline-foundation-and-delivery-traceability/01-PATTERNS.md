# Phase 1: Offline Foundation and Delivery Traceability - Pattern Map

**Mapped:** 2026-07-17  
**Files/groups classified:** 39  
**Implementation analogs found:** 0 / 39  
**Repository contract analogs found:** 4  

## Greenfield Finding

This repository has no Go, PowerShell, TypeScript, CI, schema, test, or package-manifest implementation to copy. Phase 1 establishes those conventions. The only legitimate analogs are repository-owned contracts:

1. `.planning/linear-map.json` supplies the legacy credential-free identity seed that must be migrated, not replaced.
2. `.planning/config.json` demonstrates an existing machine-readable configuration concern that the new project configuration must remain distinct from.
3. `.planning/REQUIREMENTS.md` and `.planning/ROADMAP.md` are authoritative source artifacts for durable trace IDs.
4. `.gitignore` establishes that `.env` is already excluded and must remain so.

Research examples are implementation contracts, not existing-code analogs. They are labeled as such below; planners must not claim that the repository already implements them.

## File Classification

| New/Modified File | Change | Role | Data Flow | Closest Repository Analog | Match Quality |
|---|---|---|---|---|---|
| `golc.ps1` | create | utility / command shim | request-response, batch, file-I/O | none | no analog |
| `golc.project.toml` | create | config / root index | file-I/O, transform | `.planning/config.json` | boundary-contract only |
| `config/toolchain.toml` | create | config | file-I/O | `.planning/config.json` | boundary-contract only |
| `config/commands.toml` | create | config | request-response, file-I/O | `.planning/config.json` | boundary-contract only |
| `config/generation.toml` | create | config | transform, file-I/O | `.planning/config.json` | boundary-contract only |
| `config/application-defaults.toml` | create | config | file-I/O | `.planning/config.json` | boundary-contract only |
| `config/runtime.toml` | create | config | file-I/O | `.planning/config.json` | boundary-contract only |
| `config/integrations/linear.toml` | create | config | request-response, file-I/O | `.planning/config.json` | boundary-contract only |
| `.env.example` | create | config / documentation | file-I/O | `.gitignore` | boundary-match |
| `.gitignore` | modify | config | file-I/O | `.gitignore` | exact existing file |
| `go.mod` | create | config / dependency manifest | batch, file-I/O | none | no analog |
| `go.sum` | create | config / integrity lock | batch, file-I/O | none | no analog |
| `cmd/golc-project/main.go` | create | controller / CLI entrypoint | request-response | none | no analog |
| `internal/bootstrap/*.go` | create | service | batch, file-I/O, request-response | none | no analog |
| `internal/projectconfig/*.go` | create | service / model | file-I/O, transform | `.planning/config.json` | contract-boundary only |
| `internal/contracts/*.go` | create | model / generator | transform, file-I/O | none | no analog |
| `internal/trace/catalog/*.go` | create | model / service | CRUD, transform, file-I/O | `.planning/linear-map.json` plus planning artifacts | legacy-contract match |
| `internal/trace/reconcile/*.go` | create | service | transform, request-response | none | no analog |
| `internal/trace/apply/*.go` | create | service | request-response, CRUD, file-I/O | none | no analog |
| `schemas/golc-project.schema.json` | create/generated | config / contract | transform, file-I/O | none | no analog |
| `schemas/config-*.schema.json` | create/generated | config / contract | transform, file-I/O | none | no analog |
| `schemas/linear-map.schema.json` | create/generated | model / contract | transform, file-I/O | `.planning/linear-map.json` | source-instance only |
| `schemas/linear-plan.schema.json` | create/generated | model / contract | transform, file-I/O | none | no analog |
| `.planning/linear-map.json` | modify/migrate | model / mapping catalog | CRUD, file-I/O | `.planning/linear-map.json` schema 1 | exact legacy contract |
| `tools/linear-sync/package.json` | create | config / dependency manifest | batch, file-I/O | none | no analog |
| `tools/linear-sync/package-lock.json` | create | config / integrity lock | batch, file-I/O | none | no analog |
| `tools/linear-sync/tsconfig.json` | create | config | transform, file-I/O | none | no analog |
| `tools/linear-sync/src/*.ts` | create | service / adapter | request-response, CRUD | none | no analog |
| `internal/bootstrap/*_test.go` | create | test | batch, file-I/O | none | no analog |
| `internal/projectconfig/*_test.go` | create | test | transform, file-I/O | none | no analog |
| `internal/contracts/generate_test.go` | create | test | transform, file-I/O | none | no analog |
| `internal/trace/catalog/*_test.go` | create | test | CRUD, transform, file-I/O | none | no analog |
| `internal/trace/reconcile/*_test.go` | create | test | transform, request-response | none | no analog |
| `internal/trace/apply/*_test.go` | create | test | request-response, CRUD | none | no analog |
| `tools/linear-sync/test/*.test.ts` | create | test | request-response, CRUD | none | no analog |
| `tests/fixtures/config/**` | create | test fixture | file-I/O, transform | none | no analog |
| `tests/fixtures/linear/**` | create | test fixture | request-response, pub-sub transcript | none | no analog |
| `tests/golden/**` | create | test oracle | transform, file-I/O | none | no analog |
| `.github/workflows/check.yml` | create | config / CI | batch, request-response | none | no analog |

### Scope Notes

- `schemas/config-*.schema.json`, `internal/*/*.go`, and test globs are file families because research fixes their responsibility but does not prescribe every filename. The planner should split them into concrete files without inventing a second authority.
- Generated foundation ZIPs, checksum manifests, preview plans, apply journals/reports, `.tools/`, caches, `golc.local.toml`, and real `.env` are runtime/generated artifacts rather than committed source unless a plan explicitly defines a safe committed golden fixture.
- No Wails application, React frontend, Art-Net code, fixture YAML, playback engine, product installer, script host, or AI integration belongs in this phase.

## Pattern Assignments

### `golc.ps1`, `cmd/golc-project/main.go`, and `internal/bootstrap/*.go`

**Analog:** none. No command shim, CLI, downloader, or bootstrap code exists.

**Design contract:** use the two-stage boundary in `01-RESEARCH.md` lines 264-275:

- PowerShell 5.1 is the clean-checkout shim and bootstrap authority only.
- Normal subcommands delegate to the pinned project-local Go executable.
- `bootstrap` and explicit update/remote Linear operations are the only network-capable paths.
- `check`, `generate`, `build`, `test`, and foundation `package` must operate from local pinned tools/caches after bootstrap.

**Entrypoint shape to implement:**

```text
powershell -NoProfile -File .\golc.ps1 <subcommand>
  -> bootstrap: verify/download exact archives, warm caches, build local CLI
  -> otherwise: .tools/bin/golc-project.exe <subcommand>
```

**Concrete tool-install contract** (`01-RESEARCH.md` lines 633-641; research example, not repository code):

```powershell
$env:GOTOOLCHAIN = "local"
$env:GOBIN = (Join-Path $RepoRoot ".tools\bin")
& $ProjectGo install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

**Error handling to establish:** stable named diagnostics for checksum failure, missing offline cache/tool, path escape, and prohibited network access. Stage downloads/extraction and promote only after SHA-256 verification; an idempotent second bootstrap must make no network call.

**Tests:** corrupt checksum, correct retry, idempotent second bootstrap, project-local executable path, `GOTOOLCHAIN=local`, and fail-on-network core command graph.

---

### `golc.project.toml` and `config/**/*.toml`

**Closest contract analog:** `.planning/config.json` is an existing configuration concern, not a format or structure to clone.

**Existing nested concern pattern** (`.planning/config.json` lines 1-17):

```json
{
  "mode": "interactive",
  "granularity": "standard",
  "model_profile": "inherit",
  "commit_docs": true,
  "parallelization": true,
  "search_gitignored": false,
  "git": {
    "branching_strategy": "none",
    "create_tag": true,
    "phase_branch_template": "gsd/phase-{phase}-{slug}"
  }
}
```

**Boundary to preserve:** `.planning/config.json` lines 18-46 contains GSD workflow/security settings (`nyquist_validation`, `pattern_mapper`, `security_enforcement`, ASVS level). The new `golc.project.toml` must index GOLC developer/application concerns without absorbing, shadowing, or rewriting this GSD configuration.

**Required allocation:**

| File | Single authority |
|---|---|
| `golc.project.toml` | schema version and fixed concern IDs/relative paths only |
| `config/toolchain.toml` | exact Go/Node/Wails versions, official archive URLs/hashes, cache policy |
| `config/commands.toml` | supported root subcommands and internal build/test/package command graph |
| `config/generation.toml` | source-to-generated-output rules and drift policy |
| `config/application-defaults.toml` | future committed product defaults only |
| `config/runtime.toml` | runtime key declarations; no secrets or machine values |
| `config/integrations/linear.toml` | non-secret Linear taxonomy/key names; no API key or invented UUID |

**Strict TOML pattern** (`01-RESEARCH.md` lines 615-627; research example):

```go
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

**Resolution pattern** (`01-RESEARCH.md` lines 277-316): committed default -> user config -> ignored project-local config -> allowlisted environment -> CLI. Return the winning value with layer, safe source name/path/line, and shadowed origins. Locked pins, hashes, concern paths, generated outputs, schema versions, and identity must reject overrides.

**Validation:** reject unknown keys, duplicate authority, duplicate TOML definitions, invalid values, unresolved/cyclic references, absolute or root-escaping paths, final symlink/reparse escapes, and old+new deprecated keys together.

---

### `internal/projectconfig/*.go` and `internal/projectconfig/*_test.go`

**Closest contract analog:** `.planning/config.json`; no loader/resolver implementation exists.

**Copy contract, not JSON shape:** preserve concern separation and nested validation, but implement one canonical-key ownership registry and typed symbolic references. Every resolved value should carry provenance; secret declarations render only `<set>` or `<unset>`.

**Testing pattern to establish:** Go table tests for every precedence pair, locked-key override attempts, unknown/deprecated handling, duplicate authority, cycles, unresolved refs, path containment, and deterministic explain output. Golden files under `tests/golden/` should hold safe provenance output.

---

### `internal/contracts/*.go` and `schemas/*.schema.json`

**Analog:** none. No generated contract or schema exists.

**Design contract:** Go types are authoritative; generated Draft 2020-12 JSON Schema is committed review output. Cover the root index, each concern, Linear mapping, and deterministic Linear plan. Emit `additionalProperties: false`, stable definitions/order, a generated marker naming the source package/version, LF line endings, and byte-for-byte `generate --check` drift.

**Generation flow:**

```text
Go contract structs
  -> invopop/jsonschema reflection
  -> deterministic normalization/sort/LF
  -> schemas/*.schema.json
  -> generate --check compares bytes without rewriting
```

**Tests:** `internal/contracts/generate_test.go` regenerates into a temporary location, compares committed bytes, and verifies unknown-property rejection. Schema goldens belong under `tests/golden/`.

---

### `.planning/linear-map.json`, `internal/trace/catalog/*.go`, and catalog tests

**Analog:** `.planning/linear-map.json` is the exact legacy source contract.

**Existing schema-1 identity seed** (`.planning/linear-map.json` lines 1-20):

```json
{
  "schema": 1,
  "repository": {
    "project_id": "project:golc",
    "name": "GOLC"
  },
  "active_milestone": {
    "milestone_id": "milestone:v1",
    "name": "GOLC v1"
  },
  "remote_mappings": [
    {
      "repo_id": "milestone:v1",
      "linear_type": "project",
      "status": "pending",
      "linear_uuid": null,
      "identifier": null,
      "url": null
    }
  ]
}
```

**Copy exactly:** preserve `project:golc`, `milestone:v1`, pending status, and nullable remote fields during the schema-1-to-2 migration. Never synthesize a remote UUID.

**Extend by contract:** schema 2 must represent project, milestone, phase, requirement, plan, and executable task local IDs; parent local IDs; source path/anchor; optional immutable Linear UUID; mutable display identifier/URL; mapping status; and normalized sync baseline. Source path/anchor aids navigation but never defines identity.

**Planning source analogs:**

- `.planning/ROADMAP.md` lines 24-35 defines Phase 1, its eight requirement IDs, and success criteria.
- `.planning/REQUIREMENTS.md` lines 16-21 defines `CONF-01` through `CONF-04`.
- `.planning/REQUIREMENTS.md` lines 132-137 defines `LINR-01` through `LINR-04`.
- `.planning/STATE.md` lines 72-76 explicitly states that remote mappings are pending and no IDs may be invented.

**Hierarchy contract** (`.planning/research/STACK.md` lines 85-93): release -> Linear Project; phase -> Project Milestone; plan/feature -> parent Issue; requirement -> labeled Issue; task -> Issue/sub-issue. Stable repository ID and immutable GraphQL UUID own linkage, not title or human issue key.

**Tests:** migration byte safety, duplicate IDs, ID grammar, parent-kind validation, missing/cyclic parents, safe source paths/anchors, stable identity across display renames, nullable UUID validation, and credential-canary absence.

---

### `internal/trace/reconcile/*.go` and reconcile tests

**Analog:** none. No diff, merge, canonical JSON, or plan implementation exists.

**Operation shape contract** (`01-RESEARCH.md` lines 659-675; research example):

```go
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

**Core pattern:** three-way compare last successful base, current repository-owned fields, and complete Linear snapshot. Same-field changes on both sides become field-level conflicts; neither side wins. Sort operations topologically by remote hierarchy, then by local ID. Canonical plan bodies contain no time/random value and are bound to intent, mapping, and remote-scope SHA-256 digests.

**Removal pattern:** absence must never produce delete. Only an explicit reviewed archive or unlink request may produce that operation.

**Tests:** repeat-preview byte identity, conflict matrix, rename stability, 51-item second-page marker discovery, ambiguity, source/remote digest drift, no implicit delete, and canonical golden plans.

---

### `internal/trace/apply/*.go` and apply tests

**Analog:** none. No mutation state machine or journal exists.

**Apply decision contract** (`01-RESEARCH.md` lines 678-694; research example):

```go
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

**Core pattern:** verify schema/hash and repository intent; re-read complete remote state before each mutation; accept an exact achieved postcondition as replay no-op; otherwise require exact before-state and `updatedAt`; mutate once; read back; atomically persist mapping/journal. After an uncertain mutation outcome, discover by immutable UUID or exact local-ID footer before any retry.

**Tests:** stale source and remote state, timeout-after-create, exact replay, partial apply/resume, rate-limited second operation, unrelated remote change, atomic mapping update, explicit archive/unlink, and no blind mutation retry.

---

### `tools/linear-sync/*`

**Analog:** none. No Node workspace or transport adapter exists.

**Boundary:** `@linear/sdk@88.1.0` and TypeScript `7.0.2` are exact pins in an isolated tooling workspace. The adapter owns typed GraphQL transport and normalization only; Go owns identity, authority, merge, plan, and apply policy.

**Pagination contract** (`01-RESEARCH.md` lines 644-656; research example):

```typescript
let page = await linearClient.issues({ first: 50 });
const nodes = [...page.nodes];

while (page.pageInfo.hasNextPage) {
  page = await page.fetchNext();
  nodes.push(...page.nodes);
}
```

Add cursor repetition/null-cursor detection and explicit `complete`, page-count, and object-count diagnostics. Inspect both GraphQL `data` and `errors`, even on HTTP 200. Normalize `errors[].path`, `extensions.code`, operation name, and safe rate metadata. Bounded retries apply to reads/5xx only; mutations require postcondition reconciliation.

**Package patterns:** exact dependency versions, committed lock integrity, no postinstall assumption, Node built-in `node:test`, compiled `dist/` generated locally. First install of the recently published exact pins requires the research-mandated human verification checkpoint.

**Tests:** pagination over 51 items, cursor loop, partial `data+errors`, `RATELIMITED`, reset/complexity metadata, 5xx/read retry, mutation uncertainty, header/error redaction, and secret canary over stdout/stderr/results.

---

### `.env.example` and `.gitignore`

**Analog:** `.gitignore` line 1 is the exact existing secret boundary:

```gitignore
.env
```

Preserve it. Add ignores for project-local toolchains/caches, local overrides, generated transient plans/journals/reports, and package output as their exact paths are finalized. Do not broadly ignore committed schemas, mapping, lockfiles, test goldens, or `.env.example`.

**Safe example contract:**

```dotenv
# Optional: needed only by explicit Linear remote commands.
LINEAR_API_KEY=
LINEAR_TEAM_ID=
```

This example contains names and safe empty placeholders only. The Go core and offline commands must not load `.env`; only the isolated explicit Linear remote path may consume an already-set environment or safe env-file loader. Never echo values or serialize environment/request headers.

---

### `.github/workflows/check.yml`

**Analog:** none. No CI workflow exists.

**Command parity:** Windows CI invokes the same root commands as contributors: quick tests per task, then `generate --check`, `check --offline`, and full `test` per wave; foundation build/package joins the phase gate.

**Security boundary:** pull-request jobs have no Linear secret and cannot reach apply. Optional remote drift/apply belongs in a separate protected/manual context. Runtime code must also refuse apply under a pull-request event, so YAML separation is not the only guard.

**Ephemeral secret cleanup contract** (`01-RESEARCH.md` lines 698-716; research example): create `.env` only in protected non-PR work, invoke the explicit remote command, and remove `.env` in `finally`. Do not put secret values in argv or logs.

---

### Test fixtures and goldens

**Analog:** none. No test infrastructure exists.

**Fixture allocation:**

| Path | Contents |
|---|---|
| `tests/fixtures/config/` | valid, unknown, TOML duplicate, duplicate authority, deprecated, unresolved/cyclic ref, locked override, root/reparse escape inputs |
| `tests/fixtures/linear/` | complete/paginated, partial errors, rate limit, ambiguity, timeout-after-commit, stale state, partial apply/resume transcripts |
| `tests/golden/` | generated schemas, safe provenance output, canonical preview plans, apply reports |

Goldens must be deterministic, credential-free, LF-normalized, and scanned with a unique fake-secret canary. No fixture should contain a real workspace UUID, token, `.env`, or credential.

## Shared Patterns

### Repository-Owned Authority

**Source:** `.planning/linear-map.json` lines 1-20; `.planning/ROADMAP.md` lines 24-35; `.planning/REQUIREMENTS.md` lines 16-21 and 132-137.  
**Apply to:** catalog, map migration, reconcile, apply, generated schemas, fixtures.

Local IDs and repository text/structure remain authoritative and complete offline. Linear owns operational status, assignee, priority, estimate, and completion fields. Comments remain only in Linear.

### Generated Contracts, Source Types First

**Source:** `01-RESEARCH.md` lines 318-324.  
**Apply to:** `internal/contracts`, all committed schemas, map/plan decoding, CI.

Generate reviewable contracts from Go types, reject unknown/duplicate JSON members, normalize deterministically, and fail CI on drift. Generated files never become the editable source of truth.

### Safe Error Handling

**Source:** no repository implementation analog; required by locked decisions and validation.  
**Apply to:** all CLI, config, mapping, reconciliation, transport, and CI paths.

Use stable error codes and structured allowlisted fields. Wrap causes without dumping configs, environment, headers, GraphQL clients/responses, or secrets. Missing network/credential fails only the requested Linear remote operation and never invalidates local planning/build/test results.

### Exact Preview Before Mutation

**Source:** `01-RESEARCH.md` lines 375-396.  
**Apply to:** reconcile, apply, Linear adapter, plan schema, goldens, protected CI/manual flow.

Preview is canonical and hash-bound; apply consumes the exact reviewed artifact and rejects repository or remote drift. An already-achieved exact postcondition is a no-op success; ambiguous or unknown state blocks.

### Secret Isolation

**Source:** `.gitignore` line 1; `.planning/STATE.md` line 75.  
**Apply to:** `.env.example`, project config, CLI provenance, adapter diagnostics, plan/map/report schemas, tests, CI.

Commit only names/safe placeholders and credential-free mappings. Never inspect or load `.env` outside the explicit remote adapter path. Scan every emitted byte with a fake canary.

### Offline/Remote Split

**Source:** `.planning/ROADMAP.md` lines 24-35 and `.planning/research/STACK.md` lines 93-95.  
**Apply to:** root command, bootstrap, Go services, Node adapter, CI.

After bootstrap, core generation/check/build/test/package and local trace validation use project-local pins/caches with network disabled. Linear remote commands are explicit, isolated, and non-blocking to local work.

## No Analog Found

The planner must use `01-RESEARCH.md` and `01-VALIDATION.md` contracts for these greenfield areas:

| File/Group | Role | Data Flow | Reason |
|---|---|---|---|
| `golc.ps1`, `cmd/golc-project/main.go`, `internal/bootstrap/*` | utility/controller/service | request-response, batch, file-I/O | no scripts or source exist |
| `go.mod`, `go.sum` | dependency config | batch, file-I/O | no Go module exists |
| `internal/projectconfig/*` | config service/model | transform, file-I/O | only a GSD JSON concern exists; no TOML/resolver code |
| `internal/contracts/*`, `schemas/*` | generator/contract | transform, file-I/O | no generated artifacts exist |
| `internal/trace/reconcile/*` | service | transform, request-response | no merge/diff implementation exists |
| `internal/trace/apply/*` | service | request-response, CRUD | no mutation/journal implementation exists |
| `tools/linear-sync/*` | transport adapter/config/test | request-response, CRUD | no Node workspace exists |
| `tests/fixtures/**`, `tests/golden/**` | test fixtures/oracles | file-I/O, transform | no test corpus exists |
| `.github/workflows/check.yml` | CI config | batch | no workflow exists |

## Planner Guardrails

1. Do not create implementation tasks that copy `.planning/config.json` into GOLC config; it is a separate GSD concern and a collision boundary.
2. Migrate `.planning/linear-map.json` explicitly and preserve its two stable local IDs and null remote mapping.
3. Keep the Linear SDK narrow and outside application/runtime packages; merge and safety policy stay in Go.
4. Do not make remote Linear smoke a prerequisite for offline requirement acceptance. Record it pending when credentials/taxonomy are unavailable.
5. Do not add title-based remote adoption, automatic remote delete, blind mutation retry, first-page-only reads, or HTTP-status-only success handling.
6. Do not inspect `.env`, introduce secret values into fixtures, or make unrelated core commands load integration credentials.
7. Do not pull Wails UI or product NSIS packaging into the phase. `package --foundation` is phase-bounded developer-tool packaging/readiness.
8. Preserve the validation sampling contract: quick tests after tasks; generation drift, offline check, and full tests after waves.

## Metadata

**Analog search scope:** repository root; `.planning/`; hidden files included; `.git/`, `.env`, credentials, and research cache contents excluded.  
**Repository files inventoried:** 16 non-cache planning/project files plus the four Phase 1 artifacts.  
**Implementation source files present:** 0.  
**Existing analog files read:** `.planning/linear-map.json`, `.planning/config.json`, `.gitignore`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, `.planning/PROJECT.md`, relevant stack/summary sections, and `AGENTS.md`.  
**Pattern extraction date:** 2026-07-17.
