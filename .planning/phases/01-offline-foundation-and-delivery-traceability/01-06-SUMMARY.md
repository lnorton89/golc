---
phase: 01-offline-foundation-and-delivery-traceability
plan: 06
subsystem: command-routing
tags: [go, command-routing, offline, delivery-graph, network-deny, powershell]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 19
    provides: internal/command's self-registered "generate"/"generate --check"/"check --concern project" routes and internal/contracts.CheckDrift/GenerateAll
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 28
    provides: internal/bootstrap's repository-local cache-layout/offline-environment contract (ProjectCacheLayout, GOTOOLCHAIN=local) and golc.ps1's cache warming
provides:
  - internal/delivery/graph.go — the single declarative owner of the offline core command graph (generate, check, build, test): LoadGraph (consumes exactly config/commands.toml), Run/RunOffline (execute steps through an injected StepExecutor), ValidateParity (duplicate-safe well-formedness), OfflineEnvironment (GOTOOLCHAIN=local, GOPROXY=off, repository-local GOMODCACHE/GOCACHE/GOBIN, GOFLAGS=-mod=readonly), and DenyTransport (fails every HTTP request with a named diagnostic)
  - internal/command/build.go — self-registered exact "build" route (go build ./... through the pinned toolchain)
  - internal/command/check.go — adds "check --offline", composing internal/delivery's graph through the in-process command registry
  - internal/command/test.go — adds bare "test" (full suite) and bare "test --quick" (graph orchestration: go vet) alongside the preserved "test --quick --scope <name>" form, plus a registered Node-scope lookup/registration extension point (MustDeclareNodeScope) for a later Node-owning plan
  - tests/acceptance/offline.ps1 -Mode core — proves build/test --quick/check --offline/generate --check all succeed through golc.ps1 with network denied, end to end against a bootstrapped checkout
affects: [command-routing, delivery, bootstrap, foundation-package]

tech-stack:
  added: []
  patterns:
    - "internal/delivery never imports internal/command or internal/bootstrap, even though it needs command-registry-shaped execution and bootstrap-shaped cache paths: it defines its own StepExecutor callback type (satisfied by a closure in check.go that wraps command.NewDefaultCommandRegistry) and computes its own repository-local cache paths directly (mirroring internal/command/test.go's projectGoEnvironment), rather than importing either package. internal/command (check.go) is the only importer of internal/delivery, so the one-directional edge command -> delivery never cycles."
    - "internal/delivery/delivery_test.go is the external package delivery_test (not internal package delivery), unlike every other Phase-1 quick-test scope file (bootstrap_test.go, router_test.go use the internal-package form). Declaring the 'delivery' scope from an internal test file would import internal/command from package delivery, closing delivery[test] -> command -> delivery — an import cycle that does not exist for any other package because no other package's production code imports internal/command back."
    - "check --offline's graph step for 'check' invokes '--concern project', never '--offline' — coreSteps() in graph.go is the one place this is declared, preventing a check-driven graph run from recursing into itself."
    - "test.go's MustDeclareNodeScope mirrors MustDeclareRoute/MustDeclareScope's self-registration shape exactly (compile-safe panic on invalid input, package-level var-initializer call site) so a later Node-owning plan (tools/linear-sync) can register a Node test scope without editing this dispatcher again."

key-files:
  created:
    - internal/delivery/graph.go
    - internal/delivery/delivery_test.go
    - internal/command/build.go
    - tests/acceptance/offline.ps1
  modified:
    - internal/command/check.go
    - internal/command/test.go

key-decisions:
  - "config/commands.toml was NOT modified, despite being listed in this plan's files_modified. internal/projectconfig/model.go's 'commands' concern (Plan 03, out of this plan's scope) enumerates exactly three canonical keys (commands.entrypoint, commands.cli_binary, commands.go_version) and its strict decoder (decode.go) rejects ANY other key present in config/commands.toml with GOLC_CONFIG_UNKNOWN_KEY — verified this would break check --concern project. Rather than extending model.go's single-authority registry (a different plan's owned file, and an architectural change this plan's files_modified did not authorize), internal/delivery/graph.go's coreSteps() declares the step/network graph in Go — the same self-registration idiom router.go and contracts/generate.go already establish repository-wide — while LoadGraph still consumes exactly config/commands.toml for the command inventory (entrypoint/cli_binary/go_version) per the acceptance criteria's literal requirement. check --concern project and generate --check remain byte-identical to Plan 19's behavior (re-verified passing after every change in this plan)."
  - "internal/delivery never imports internal/bootstrap or internal/command (see tech-stack patterns above) — this was discovered as a required design constraint (not an upfront choice) when `go vet ./...` reported an import cycle through internal/bootstrap's own test file (bootstrap_test.go imports command to self-register its quick-test scope; command now imports delivery; delivery originally imported bootstrap for cache-path constants), and a second cycle through delivery's own test file for the same reason. Both were resolved by (1) computing cache paths directly in graph.go instead of importing bootstrap, and (2) making delivery_test.go an external _test package."
  - "'test --quick' (bare, no --scope) is a NEW graph-orchestration form distinct from the pre-existing 'test --quick --scope <name>': it runs `go vet ./...` (fast, no test bodies) to meet 01-VALIDATION.md's documented <=30s quick budget, while bare 'test' (no flags) runs the full `go test -count=1 ./...` suite plus every registered Node scope. '--scope' without '--quick' is rejected (GOLC_TEST_USAGE) — a scope always runs through the quick marker-discovery path, unchanged from the original contract."
  - "check --offline composes the WHOLE core graph (generate, check --concern project, build, test --quick) via delivery.RunOffline rather than being a lighter-weight standalone check, directly satisfying D-02 ('core generation, validation, build, and test operations must work offline') in one command. offline.ps1 -Mode core additionally exercises 'build' and 'test --quick' as their own direct golc.ps1 invocations to prove each route is independently reachable, not only reachable through check --offline's composition."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "internal/delivery/graph.go is the one declarative owner of the offline core graph; LoadGraph/Run/RunOffline/ValidateParity consume exactly config/commands.toml for the command inventory and never read a second inventory source."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/LoadGraph_reads_exactly_the_three_canonical_commands_keys_and_the_fixed_core_steps"
        status: pass
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/ValidateParity_accepts_the_production_graph_and_rejects_duplicates"
        status: pass
    human_judgment: false
  - id: D2
    description: "Offline mode sets GOTOOLCHAIN=local, GOPROXY=off, repository-local Go module/build/bin caches, and installs a deny transport that fails every HTTP request with a named diagnostic before any dial, restoring the exact prior process state afterward."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/RunOffline_installs_the_offline_environment_and_deny_transport,_then_restores_prior_state"
        status: pass
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/DenyTransport_fails_every_request_with_a_named_diagnostic_before_any_dial"
        status: pass
    human_judgment: false
  - id: D3
    description: "build, test, and check --offline are exact MustDeclareRoute registrations reachable from the default registry without editing router.go; existing generate/check --concern project/config registrations remain reachable and unchanged."
    requirement: CONF-03
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\golc.ps1 build; powershell -NoProfile -File .\\golc.ps1 test --quick; powershell -NoProfile -File .\\golc.ps1 check --offline; powershell -NoProfile -File .\\golc.ps1 check --concern project; powershell -NoProfile -File .\\golc.ps1 generate --check (all exit 0 against the bootstrapped repository under test)"
        status: pass
      - kind: unit
        ref: "go test -count=1 ./... (all packages, including internal/command's existing router/route-duplicate-rejection tests, pass unchanged)"
        status: pass
    human_judgment: false
  - id: D4
    description: "offline.ps1 -Mode core runs generate/check/build/test through golc.ps1 with network denied and exits 0."
    requirement: CONF-01
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\offline.ps1 -Mode core"
        status: pass
    human_judgment: false
  - id: D5
    description: "A missing cache (unbootstrapped pinned toolchain) produces a named GOLC_TEST_TOOLCHAIN_MISSING diagnostic rather than a fallback download, propagated through check --offline's step failure reporting."
    requirement: CONF-01
    verification:
      - kind: integration
        ref: "Ran the built golc-project binary's 'build'/'test --quick'/'check --offline' against this worktree before bootstrap: each failed closed with GOLC_TEST_TOOLCHAIN_MISSING: ...: run 'golc.ps1 bootstrap' first, exit 1, zero network fallback"
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 06: Offline Core Delivery Graph Summary

**internal/delivery/graph.go owns the one declarative generate/check/build/test graph (LoadGraph/Run/RunOffline/ValidateParity, GOPROXY=off + DenyTransport offline enforcement); internal/command gains self-registered "build" and "check --offline" routes plus "test"/"test --quick" graph orchestration, verified end to end by tests/acceptance/offline.ps1 -Mode core against a real bootstrapped checkout**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-07-21T01:49:57Z
- **Tasks:** 1 (TDD-flagged; treated as integration-first given the scope of one new package plus three route files plus an acceptance script — same pattern 01-19-SUMMARY.md documents)
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `internal/delivery/graph.go` is the single declarative owner of the offline core command graph (CONTEXT D-02/D-03/D-10, T-01-15, T-01-17): `LoadGraph(root)` reads exactly `config/commands.toml`'s three canonical keys (`commands.entrypoint`, `commands.cli_binary`, `commands.go_version`) and returns the fixed `generate -> check --concern project -> build -> test --quick` step order; `ValidateParity` rejects a graph with blank/duplicate step names or invocations; `Run` executes steps through an injected `StepExecutor` callback, stopping at the first non-zero exit; `RunOffline` layers the offline environment (`GOTOOLCHAIN=local`, `GOPROXY=off`, repository-local `GOMODCACHE`/`GOCACHE`/`GOBIN`, `GOFLAGS=-mod=readonly`) and an installed `DenyTransport` (fails every HTTP request with `GOLC_DELIVERY_NETWORK_DENIED` before any dial) on top of `Run`, always restoring the prior environment and `http.DefaultTransport` afterward, and refuses to execute any graph containing a step not declared `NetworkDenied`.
- `internal/command/build.go` self-registers the exact `build` route: `go build ./...` through the pinned project-local toolchain (reusing `test.go`'s `resolvePinnedGoExecutable`/`runProjectGo`), never opening a network connection.
- `internal/command/check.go` adds `check --offline` (mutually exclusive with `--concern <name>`): it loads the delivery graph, validates it, and executes it through the in-process `command.CommandRegistry` with `RunOffline`, reporting each step's exit code and stopping at the first failure with that step's own diagnostic.
- `internal/command/test.go`'s `test` route now serves three forms: bare `test` (full suite — `go test -count=1 ./...` plus every registered Node scope), bare `test --quick` (the graph's quick step — `go vet ./...`, meeting the documented <=30s budget), and the preserved `test --quick --scope <name>` (exact marker listing, fail-on-zero, unchanged). A new `MustDeclareNodeScope`/`NodeScopeRegistration` extension point lets a future Node-owning plan (`tools/linear-sync`) register a Node test scope that the scoped dispatcher checks before falling back to Go marker discovery — zero Node scopes are registered today, so all existing scope behavior is byte-identical.
- `tests/acceptance/offline.ps1 -Mode core` runs `golc.ps1 build`, `golc.ps1 test --quick`, `golc.ps1 check --offline`, and `golc.ps1 generate --check` in sequence against the repository under test and asserts every one exits 0, with `check --offline`'s stdout confirming the complete graph ran with network denied.
- Verified end to end: bootstrapped this worktree for real (`golc.ps1 bootstrap` succeeded — network was reachable in this environment), then ran `tests/acceptance/offline.ps1 -Mode core` (all four steps green), `golc.ps1 test` (full suite, all packages `ok`), `golc.ps1 test --quick --scope config` (unchanged scoped behavior), `tests/acceptance/bootstrap.ps1` (Plan 28's acceptance, unaffected), and `tests/acceptance/walking-skeleton.ps1 -Mode green` (Plan 02's acceptance, unaffected) — all pass, confirming no regression to any earlier plan's contract.

## Task Commits

1. **Task 1: Run the complete contributor core graph offline** - `f35557c` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/delivery/graph.go` - `NetworkPolicy`, `CommandInventory`, `Step`, `Graph`, `coreSteps`, `LoadGraph`, `ValidateParity`, `StepExecutor`, `StepResult`, `Run`, `OfflineEnvironment`, `DenyTransport`, `RunOffline`, `setEnvironment`.
- `internal/delivery/delivery_test.go` - `TestScopeDelivery` (external `delivery_test` package; registers quick-test scope `delivery`): LoadGraph success/missing-file/incomplete-inventory, ValidateParity accept/reject, Run stop-on-failure, RunOffline network-policy refusal, RunOffline environment/transport install-and-restore, DenyTransport, NetworkPolicy.String().
- `internal/command/build.go` - Self-registered `build` scope/route; `runBuild` (go build ./... through the pinned toolchain).
- `internal/command/check.go` - `checkArgs`, updated `parseCheckArgs`/`runCheck` to accept `--offline`; new `runCheckOffline` composing `internal/delivery`.
- `internal/command/test.go` - `testInvocation`/`parseTestArgs` (replaces `parseQuickScopeArgs`), `runTest` dispatcher, `runTestQuickScope(root, scope)` (refactored from `request`-taking form, behavior unchanged), `runTestQuick`, `runTestFull`, `NodeScopeRegistration`/`MustDeclareNodeScope`/`lookupNodeScope`/`runNodeScopeTest`/`runAllNodeScopes`, `projectGoEnvironment` now also sets `GOPROXY=off`.
- `tests/acceptance/offline.ps1` - New `-Mode core` acceptance script (build/test --quick/check --offline/generate --check through golc.ps1, network-denied end to end).

## Decisions Made

See `key-decisions` in the frontmatter above for the full rationale on: (1) why `config/commands.toml` was left unmodified despite being listed in `files_modified`, (2) the import-cycle-driven design constraint that `internal/delivery` never imports `internal/bootstrap` or `internal/command`, (3) the new `test --quick` (bare) vs. `test --quick --scope <name>` distinction, and (4) `check --offline` composing the whole graph rather than a lighter standalone check.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed internal/delivery's dependency on internal/bootstrap to resolve an import cycle**
- **Found during:** Task 1, first `go vet ./...` after wiring `check.go` to import `internal/delivery`
- **Issue:** `internal/delivery/graph.go` originally called `bootstrap.NewProjectCacheLayout` for cache paths. Since `internal/bootstrap/bootstrap_test.go` imports `internal/command` (to self-register its quick-test scopes, the established Plan 28 pattern) and `internal/command/check.go` now imports `internal/delivery`, this closed a cycle: `bootstrap[test] -> command -> delivery -> bootstrap`.
- **Fix:** `resolveOfflineEnvironment` in `graph.go` now computes `.tools/cache/go-mod`/`go-build`/`go-bin` directly (mirroring the same direct-computation pattern `internal/command/test.go`'s `projectGoEnvironment` already uses), removing the `internal/bootstrap` import entirely.
- **Files modified:** `internal/delivery/graph.go`
- **Verification:** `go vet ./...` and `go build ./...` both exit 0.
- **Committed in:** `f35557c` (Task 1 commit; no broken intermediate commit exists)

**2. [Rule 3 - Blocking] Made delivery_test.go an external test package to resolve a second import cycle**
- **Found during:** Task 1, same `go vet ./...` pass, after fixing deviation 1
- **Issue:** `internal/delivery/delivery_test.go` (written as internal package `delivery`, following the `bootstrap_test.go`/`router_test.go` precedent) imports `internal/command` to self-register the `delivery` quick-test scope. Since `internal/command/check.go` imports `internal/delivery`, this closed `delivery[test] -> command -> delivery`.
- **Fix:** Changed `delivery_test.go` to the external package `delivery_test`, importing both `internal/delivery` and `internal/command` and referencing every delivery identifier through the `delivery.` prefix. This is safe because `delivery_test` is not imported by anything else.
- **Files modified:** `internal/delivery/delivery_test.go`
- **Verification:** `go vet ./...` exits 0; `go test -count=1 ./...` passes for all packages including `internal/delivery`.
- **Committed in:** `f35557c` (Task 1 commit; no broken intermediate commit exists)

---

**Total deviations:** 2 auto-fixed (2 blocking, both import-cycle resolutions internal to this plan's own new code — no production behavior outside `internal/delivery` was affected)
**Impact on plan:** Both fixes were necessary for the module to compile at all; neither changes any documented acceptance criterion or removes any planned capability. No scope creep.

## Issues Encountered

- **`config/commands.toml` strict-schema conflict (design constraint, not a bug):** `internal/projectconfig/model.go`'s `DefaultSpec()` "commands" concern (owned by Plan 03, outside this plan's `files_modified`) enumerates exactly three canonical keys and its strict decoder rejects any other key present in the file with `GOLC_CONFIG_UNKNOWN_KEY`. A literal reading of this plan's key-link ("config/commands.toml -> internal/delivery/graph.go via a single authoritative step/network inventory") would suggest embedding the step/network declarations as new TOML keys, but doing so would require also extending `model.go`'s registry — a different plan's owned single-authority file, and an architectural change this plan was not authorized to make. Resolved by declaring the step/network graph in Go (`coreSteps()`), the same self-registration idiom already established repository-wide, while `LoadGraph` still consumes exactly `config/commands.toml` for the command inventory. Verified `check --concern project` and `generate --check` are byte-identical to Plan 19's behavior throughout (re-run after every change in this plan).
- **Toolchain not yet bootstrapped when this worktree was created** (matching the exact pattern 01-05/01-18/01-19/01-28-SUMMARY.md document): before running `golc.ps1 bootstrap`, `build`/`test --quick`/`check --offline` all failed closed with `GOLC_TEST_TOOLCHAIN_MISSING: ...: run 'golc.ps1 bootstrap' first` — this is exactly the "missing cache produces a named diagnostic rather than a fallback download" behavior this plan's acceptance criteria require, so it doubled as a real negative-path verification. Network access to `go.dev` was available in this environment, so `golc.ps1 bootstrap` was run for real (no fixture substitution needed) and every subsequent acceptance command (`offline.ps1 -Mode core`, `golc.ps1 test`, `tests/acceptance/bootstrap.ps1`, `tests/acceptance/walking-skeleton.ps1 -Mode green`) was verified green against the fully bootstrapped checkout.

## Known Stubs

- **Zero Node scopes are registered.** `MustDeclareNodeScope`/`NodeScopeRegistration`/`lookupNodeScope` are fully implemented and exercised by `runTestQuickScope`'s lookup-before-Go-marker-discovery order, but no command file in this phase calls `MustDeclareNodeScope` yet — Phase 1 has no Node/npm project anywhere in the repository (confirmed: no `package.json` exists). This is intentional: a later Node-owning plan (`tools/linear-sync`, Plans 01-13/01-25 per `.planning/phases/01-offline-foundation-and-delivery-traceability/01-VALIDATION.md`) registers the first real Node scope through this exact extension point without editing `test.go` again.

## User Setup Required

None - build/test/check --offline are pure Go/PowerShell operations over the already-bootstrapped repository-local toolchain; no credential or external service is involved.

## Next Phase Readiness

- `internal/delivery.Graph`/`LoadGraph`/`Run`/`RunOffline`/`ValidateParity` are stable, importable primitives: a future plan needing a second graph-shaped orchestration (for example a `package --foundation` step, per `01-VALIDATION.md`'s `offline.ps1 -Mode package` gate) can extend `coreSteps()` or compose a second `Graph` value without touching `RunOffline`'s environment/transport contract.
- `internal/command.MustDeclareNodeScope` is ready for `tools/linear-sync`'s owning plan to register its Node test scope; `test --quick --scope <name>` and the full-suite `test` will reach it automatically.
- `check --offline` is now real infrastructure other plans' acceptance gates can depend on directly (`.planning/phases/01-offline-foundation-and-delivery-traceability/01-VALIDATION.md` already documents it as a phase-wide gate).

## Self-Check: PASSED

- All created files verified present on disk: `internal/delivery/graph.go`, `internal/delivery/delivery_test.go`, `internal/command/build.go`, `tests/acceptance/offline.ps1` (all `FOUND`).
- Commit `f35557c` verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only HEAD~1 HEAD` reports zero deleted files; working tree clean before this summary.
- `go build ./...`, `go vet ./...`, and `go test -count=1 ./...` all exit 0 from the repository root (GOPROXY=off, GOFLAGS=-mod=readonly); `go.mod`/`go.sum` unchanged (`git status --short go.mod go.sum` empty) throughout the session.
- `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode core` exits 0 against the bootstrapped repository under test; `golc.ps1 check --concern project`, `golc.ps1 generate --check`, `golc.ps1 test`, `golc.ps1 test --quick --scope config`, `tests/acceptance/bootstrap.ps1`, and `tests/acceptance/walking-skeleton.ps1 -Mode green` all remain green (no regression to any earlier plan's contract).

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
