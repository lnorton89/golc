---
phase: 1
slug: offline-foundation-and-delivery-traceability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-17
revised: 2026-07-18
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

## Test Infrastructure

| Property | Value |
|---|---|
| **Framework** | Go 1.26.5 `testing` and Node 24.18.0 `node:test` |
| **Config file** | none — Wave 0 creates Go/Node manifests and harnesses |
| **Quick run command** | `powershell -NoProfile -File .\golc.ps1 test --quick` |
| **Full suite command** | `powershell -NoProfile -File .\golc.ps1 test` |
| **Estimated runtime** | quick ≤ 30 seconds; full ≤ 120 seconds |

Route and scope ordering contract:

- Plan 01-17 creates `CommandHandler`, `CommandRegistry`, `CommandRegistration`, `ScopeRegistration`, duplicate route/scope rejection, deterministic lookup/listing, and the `MustDeclareRoute`/`MustDeclareScope` self-registration entrypoints.
- Plan 01-02 self-registers the generic `test --quick --scope` route. Every owning Go test task registers its exact scope through `MustDeclareScope` before verification and defines the matching `TestScope{PascalName}` marker.
- Node scopes are registered in the authoritative `config/commands.toml` graph before invocation; the dispatcher requires an exact matching `TestScope{PascalName}` marker and fails on missing/duplicate scope or zero markers.
- Plan 01-21 creates `internal/command/linear_validate.go` and owns the single `linear validate --offline` registration before its catalog-first verification; Plan 01-22 extends that handler with map/schema checks without redeclaring the route.
- Plan 01-04 creates the only central schema registry/generator owner: `SchemaDescriptor`, `RegisterSchema`, `RegisteredSchemas`, `GenerateAll`, and `CheckDrift` reject invalid/duplicate descriptors and traverse a defensive stable sorted snapshot exactly once. Its seven configuration projections register through that API; Plan 01-22 registers `linear-map` only from `internal/contracts/linear.go`, and Plan 01-24 registers `linear-plan`/`linear-report` only from `internal/contracts/linear_plan.go`. Plans 01-22 and 01-24 never modify `internal/contracts/generate.go`.
- Every route-owning task self-registers its exact route in its own command file. No plan after 01-17 modifies `internal/command/router.go`.

Additional phase gates:

- Offline acceptance: `powershell -NoProfile -File .\golc.ps1 check --offline`
- Generated-artifact drift: `powershell -NoProfile -File .\golc.ps1 generate --check`
- D-04 updates: `powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update`

## Sampling Rate

- **After every task commit:** Run the task-specific command below, then root quick tests once the route exists.
- **After every plan wave:** Run generation drift, offline check, and full tests once those gates exist.
- **Before `$gsd-verify-work`:** Generation clean, offline check, full tests, foundation build/package, and offline map validation must pass.
- **Max feedback latency:** 30 seconds per task sample and 120 seconds per wave gate.
- **External smoke:** One reviewed real Linear preview/apply/replay is recorded when credentials/taxonomy are available; unavailability remains pending and cannot fail offline acceptance.

## Per-Task Verification Map

This map covers all 33 automated implementation tasks in the finalized 29-plan set. Plan 01-12 is the separate blocking package-legitimacy checkpoint and appears under Manual-Only Verifications. Every implementation row has a concrete automated command.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure behavior | Automated command | Status |
|---|---:|---:|---|---|---|---|---|
| 01-01-01 | 01-01 | 1 | CONF-01, CONF-03 | T-01-01 | Specific failing end-to-end walking-skeleton contract | `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode red` | pending |
| 01-16-01 | 01-16 | 2 | CONF-01, CONF-03 | T-01-01, T-01-SC | Online bootstrap pins/downloads `github.com/BurntSushi/toml@v1.6.0` plus `github.com/invopop/jsonschema@v0.14.0` and transitive sums into the local cache, then a network-denied readonly schema probe resolves with zero download/mutation | `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode bootstrap` | pending |
| 01-17-01 | 01-17 | 3 | CONF-01, CONF-03 | T-01-02, T-01-03 | Duplicate-safe deterministic self-registration and contained inspect route | `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode green` | pending |
| 01-02-01 | 01-02 | 4 | CONF-01, CONF-02, CONF-04 | T-01-04, T-01-05 | Atomic local persistence and exact registered config-local scope | `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-local` | pending |
| 01-02-02 | 01-02 | 4 | CONF-01, CONF-02, CONF-04 | T-01-05, T-01-06 | Cross-process set/explain stays deterministic and secret-safe | `powershell -NoProfile -File .\tests\acceptance\walking-skeleton.ps1 -Mode green` | pending |
| 01-03-01 | 01-03 | 5 | CONF-01, CONF-02, CONF-04 | T-01-07, T-01-09 | Strict concern/deprecation diagnostics through registered scope | `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-strict` | pending |
| 01-08-01 | 01-08 | 5 | LINR-01, LINR-02 | T-01-21, T-01-22 | Dynamic durable catalog and repository authority | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-catalog` | pending |
| 01-18-01 | 01-18 | 6 | CONF-01, CONF-02, CONF-04 | T-01-07, T-01-08, T-01-09 | Five-layer precedence, locked keys, containment, safe provenance | `powershell -NoProfile -File .\golc.ps1 test --quick --scope config` | pending |
| 01-09-01 | 01-09 | 6 | LINR-01, LINR-02 | T-01-24, T-01-25, T-01-26 | Strict schema-1 migration preserves dynamic identity | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-map` | pending |
| 01-21-01 | 01-21 | 6 | LINR-01, LINR-02 | T-01-21, T-01-23 | Every adversarial/rename fixture is loaded before invoking the Plan 21-owned offline validation route | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-catalog; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 linear validate --offline` | pending |
| 01-04-01 | 01-04 | 7 | CONF-01, CONF-02, CONF-03 | T-01-10, T-01-10R, T-01-SC | Source-owned deterministic registry generation with invalid/duplicate rejection, stable exactly-once traversal, and network-free readonly module resolution | `$beforeMod=(Get-FileHash .\go.mod -Algorithm SHA256).Hash; $beforeSum=(Get-FileHash .\go.sum -Algorithm SHA256).Hash; $env:GOPROXY='off'; $env:GOFLAGS='-mod=readonly'; powershell -NoProfile -File .\golc.ps1 test --quick --scope contracts; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; if ((Get-FileHash .\go.mod -Algorithm SHA256).Hash -ne $beforeMod -or (Get-FileHash .\go.sum -Algorithm SHA256).Hash -ne $beforeSum) { throw 'offline contracts verification mutated Go locks' }` | pending |
| 01-05-01 | 01-05 | 7 | CONF-01, CONF-03 | T-01-12 | Official-source archive/hash/traversal boundary | `powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-archive` | pending |
| 01-10-01 | 01-10 | 7 | LINR-03, LINR-04 | T-01-27, T-01-28 | Canonical plan/digests/order/visible marker | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-preview-contract` | pending |
| 01-19-01 | 01-19 | 8 | CONF-01, CONF-02, CONF-03 | T-01-10, T-01-11 | Exact registered generate/check routes and seven schemas | `powershell -NoProfile -File .\golc.ps1 generate --check; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 check --concern project` | pending |
| 01-23-01 | 01-23 | 8 | LINR-03, LINR-04 | T-01-28, T-01-29, T-01-30 | Conflict/ambiguity/incomplete/removal preview gates | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-reconcile` | pending |
| 01-28-01 | 01-28 | 8 | CONF-01, CONF-03 | T-01-13, T-01-SC | Cache warm plus zero-call/zero-diff second bootstrap | `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1` | pending |
| 01-06-01 | 01-06 | 9 | CONF-01, CONF-03 | T-01-15, T-01-17 | Exact registered core routes fail closed offline | `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode core` | pending |
| 01-11-01 | 01-11 | 9 | LINR-03, LINR-04 | T-01-31, T-01-32, T-01-33 | Exact apply, stale rejection, replay, uncertain outcome | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-apply-core` | pending |
| 01-11-02 | 01-11 | 9 | LINR-03, LINR-04 | T-01-32 | Atomic journal and exact-prefix resume | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-apply-resume` | pending |
| 01-22-01 | 01-22 | 9 | LINR-01, LINR-02 | T-01-24, T-01-26 | `linear-map` registers exactly once from `linear.go`; unchanged global generation/check reaches it before exact offline routes validate the canonical map | `powershell -NoProfile -File .\golc.ps1 generate --check; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 linear validate --offline` | pending |
| 01-29-01 | 01-29 | 13 | CONF-01, CONF-03 | T-01-14, T-01-SC | D-04 deterministic toolchain/Go/npm check-write, exact mutually consistent package/lock bytes, five-path allowlist, zero installs/builds | `powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update` | pending |
| 01-20-01 | 01-20 | 10 | CONF-01, CONF-03 | T-01-16 | Exact registered deterministic foundation package | `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode package` | pending |
| 01-24-01 | 01-24 | 10 | LINR-03, LINR-04 | T-01-31, T-01-34 | `linear-plan` and `linear-report` register exactly once from `linear_plan.go`; unchanged global generation/check reaches both before strict apply-route tests | `powershell -NoProfile -File .\golc.ps1 generate --check; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-plan-contract` | pending |
| 01-07-01 | 01-07 | 11 | CONF-03, CONF-04, LINR-04 | T-01-18 | Safe examples and cross-artifact canary | `powershell -NoProfile -File .\golc.ps1 test --quick --scope secrets` | pending |
| 01-07-02 | 01-07 | 11 | CONF-03, CONF-04, LINR-04 | T-01-19, T-01-20 | PR parity and remote-mutation unreachability | `powershell -NoProfile -File .\tests\acceptance\command-parity.ps1` | pending |
| 01-13-01 | 01-13 | 12 | CONF-01, CONF-03, LINR-03 | T-01-36, T-01-37, T-01-SC | Approved exact workspace and pre-registered scopes | `powershell -NoProfile -File .\golc.ps1 bootstrap --include linear-sync; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 build --scope linear-sdk` | pending |
| 01-25-01 | 01-25 | 14 | CONF-01, CONF-03, LINR-03 | T-01-36, T-01-37, T-01-38 | Offline repeat/cache reinstall and fake hierarchy operations after explicit npm update authority | `powershell -NoProfile -File .\tests\acceptance\bootstrap-node.ps1; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-sdk-operations` | pending |
| 01-14-01 | 01-14 | 15 | CONF-04, LINR-03, LINR-04 | T-01-39 | Exhaustive pagination and page-two marker | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-pagination` | pending |
| 01-26-01 | 01-26 | 16 | CONF-04, LINR-03, LINR-04 | T-01-40 | Data-plus-errors/rate limits and bounded reads | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-errors` | pending |
| 01-27-01 | 01-27 | 17 | CONF-04, LINR-03, LINR-04 | T-01-40, T-01-41 | Unknown writes never retry; all bytes redacted | `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-transport-node` | pending |
| 01-15-01 | 01-15 | 18 | CONF-03, CONF-04, LINR-03, LINR-04 | T-01-42, T-01-43 | Real process boundary complete hierarchy apply/replay | `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode hierarchy` | pending |
| 01-15-02 | 01-15 | 18 | CONF-03, CONF-04, LINR-03, LINR-04 | T-01-42, T-01-46 | Final adapter graph/package and remote-failure isolation | `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode offline; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 check --offline` | pending |
| 01-15-03 | 01-15 | 18 | CONF-03, CONF-04, LINR-03, LINR-04 | T-01-44, T-01-45, T-01-46 | Protected exact apply/cleanup plus final phase gate | `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode workflow; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 generate --check; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 check --offline; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 test; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 build; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 package --foundation; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 linear validate --offline` | pending |

## Wave 0 Requirements

- [ ] `go.mod`/`go.sum` pin and cache `github.com/BurntSushi/toml@v1.6.0` plus `github.com/invopop/jsonschema@v0.14.0` and required transitive sums before the network-denied Plan 04 probe; `cmd/golc-project/main.go` and `internal/command/router.go` provide deterministic command self-registration tests.
- [ ] `internal/contracts/generate.go` provides the exclusive deterministic schema registry/generator API; configuration, map, plan, and report descriptors register from their owning contract files and global generation/check reaches each exactly once.
- [ ] `internal/command/test.go` generic scope route with exact registered-scope/marker matching and fail-on-zero behavior.
- [ ] Go tests for strict configuration, contracts, catalog/map, reconcile/apply, delivery, bootstrap, updates, process transport, and redaction.
- [ ] Exact `tools/linear-sync` package/lock/tsconfig plus Node tests for operations, pagination, partial errors, rates, mutation uncertainty, and redaction.
- [ ] Configuration/Linear adversarial fixtures and deterministic schema/provenance/preview/report/package goldens.
- [ ] Windows PR parity workflow and protected/manual Linear workflow.

Required cases include corrupt checksum/traversal, idempotent second bootstrap, D-04 repeat check/read-only check/five-path write for toolchain+Go+npm/exact package-lock consistency/zero install-or-build, fail-on-network core execution, every precedence pair, deprecation guidance/collision, dynamic plan discovery, stable IDs across renames, 51-item pagination, `data+errors`, timeout-after-create, rate-limited resume, stale preview rejection, explicit archive/unlink, and fake-secret canary scans over every emitted byte.

## Manual-Only Verifications

| Behavior | Requirement | Why manual | Test instructions |
|---|---|---|---|
| Exact npm package legitimacy checkpoint (Plan 01-12) | CONF-03, LINR-03, LINR-04 | Recently published packages require explicit approval before install | Run its research-audit gate, open the exact registry pages, verify source/integrity/no lifecycle scripts, and approve both exact pins before Plans 13-15/25-27. |
| Reviewed real Linear preview/apply/replay | LINR-03, LINR-04 | Requires configured taxonomy, protected credential, and explicit authorization | Perform read-only taxonomy check, review exact preview, approve one apply, replay same plan, and verify one object per local ID; never run from PR CI. |

## Validation Sign-Off

- [ ] Every one of 33 implementation tasks has a concrete automated command above.
- [ ] All 29 PLAN task blocks parse as XML; auto-task count equals `read_first` count equals `acceptance_criteria` count.
- [ ] Every route/scope exists through explicit registration before its mapped command runs.
- [ ] No three consecutive tasks lack automated feedback; no watch mode is used.
- [ ] Fake-secret canary is absent from stdout, stderr, files, reports, maps, errors, generated artifacts, and packages.
- [ ] Offline acceptance passes without `.env`, credentials, or network after bootstrap.
- [ ] `nyquist_compliant: true` and `wave_0_complete: true` are set only after execution creates infrastructure and all mapped commands pass.

**Approval:** pending
