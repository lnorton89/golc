---
phase: 01-offline-foundation-and-delivery-traceability
plan: 15
subsystem: linear-traceability
tags: [go, nodejs, process-transport, linear-sdk, github-actions, offline-isolation]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 07
    provides: internal/security.SafeDiagnostic/Redact/ScanCanary/ScanCanaryAll centralized redaction contract
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 24
    provides: internal/command/linear.go's self-registered "linear apply {plan-file} --plan-id <id>" route, RemoteClientFactory injection seam, decodeAndValidatePlanStrict
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 27
    provides: tools/linear-sync/src/{protocol,adapter,redact}.ts's strict NDJSON Operation contract, MutationOutcome, safeError

provides:
  - internal/trace/transport.ProcessClient/ProcessConfig/RPCError — the real, transport-neutral Go<->Node process boundary (NewProcessClient/Call/Close) over the compiled project-local adapter, with deadline enforcement, process-tree kill, and redacted stderr
  - internal/command/linear.go's processLinearClient — the real apply.RemoteClient (Create/Update/ReadByUUID/ReadByMarker) and targeted CaptureSnapshot over the process transport, wired as the production applyRemoteClientFactory
  - Self-registered routes "linear preview --remote --out <path>" and "linear drift --remote --read-only"
  - "linear apply" now commits achieved results into .planning/linear-map.json (apply.CommitResultAtomically), making replay a safe no-op
  - internal/delivery/foundation.go's optional inclusion of the compiled Linear adapter (tools/linear-sync/dist/src/*.js, package.json/package-lock.json) in the deterministic foundation ZIP
  - .github/workflows/linear-sync.yml — protected/manual (workflow_dispatch-only) drift/apply workflow with ephemeral .env try/finally cleanup
  - tests/acceptance/linear-transport.ps1 (-Mode hierarchy/offline/workflow) — end-to-end proof against a fake @linear/sdk injected into the compiled adapter
affects: [linear-traceability, delivery, ci]

tech-stack:
  added: []
  patterns:
    - "processLinearClient lives in package command (internal/command/linear.go), not package transport, because apply.RemoteState is a package apply type and package apply already imports package transport (guard.go/engine.go) -- a transport->apply dependency would close that import cycle. internal/trace/transport/process.go therefore stays a purely byte-oriented boundary (Call(ctx, []byte) ([]byte, error)): it has zero dependency on reconcile/apply/protocol semantics, and command is the one package that already imports both apply and transport to bridge them."
    - "stateFromWireRecord/CaptureSnapshot synthesize RemoteState.Fields/RemoteRecord.Fields as {\"title\": record.Title} rather than passing through the wire record's own \"fields\" map: tools/linear-sync/src/adapter.ts's normalize() always emits fields: {} (hardcoded), so this repository's one owned field (title, CONTEXT D-11) crosses the wire only through the unified top-level title/name value."
    - "apply/model.go's fieldsMatch now compares decoded field content (json.Unmarshal into map[string]string) instead of marshaled bytes: a Plan loaded from a file was written through strictjson.CanonicalEncode (json.MarshalIndent), which reformats every nested json.RawMessage field -- including Operation.After/Before -- to match its own nesting depth in the whole document. A plain json.Marshal byte comparison against that differently-indented, differently-nested RawMessage spuriously mismatched for every already-linked entity; this was latent until this plan's first real end-to-end apply against a working RemoteClient."
    - "ReadByMarker returns found=false unconditionally for the live process transport: tools/linear-sync/src/protocol.ts's Operation contract exposes only read-by-immutable-UUID/create/update, no list/search-by-description action, so there is no way to discover a not-yet-linked remote object by its D-14 marker footer over this wire protocol. Safe recovery after an uncertain create/update instead goes through a fresh \"linear preview --remote\" (which re-reads .planning/linear-map.json's now-updated mappings and correctly resolves already-linked entities to a safe no-op) followed by a human-reviewed re-apply -- never a same-run automatic retry."
    - "CaptureSnapshot is targeted, not exhaustive: it reads back only entities this repository already recorded a Linear UUID for (seeded from catalog.MigrateV1ToV2's remote_mappings), never a full paginated connection scan (the wire protocol has no list/search action). An unlinked entity is simply absent from the snapshot, which reconcile's discoverObservations already treats as \"plan a create\" -- exactly the empty-fake-SDK hierarchy scenario this plan's acceptance proves."
    - "golc.ps1's Command/CommandArguments are read from the $args automatic variable, not a declared param() block: PowerShell 5.1's advanced parameter binder still prefix-matches any \"-something\"/\"--something\" token against its own common parameters (-OutVariable/-OutBuffer, ...) before ValueFromRemainingArguments ever collects it, so a legitimate route flag like \"--out <path>\" failed at the shim itself with \"the parameter name 'out' is ambiguous\" -- a real, previously-undiscovered bug no earlier acceptance script had exercised."

key-files:
  created:
    - internal/trace/transport/process.go
    - internal/trace/transport/process_test.go
    - tests/acceptance/linear-transport.ps1
    - .github/workflows/linear-sync.yml
  modified:
    - internal/command/linear.go
    - internal/trace/apply/model.go
    - golc.ps1
    - config/commands.toml
    - internal/delivery/graph.go
    - internal/delivery/foundation.go
    - .env.example

key-decisions:
  - "processLinearClient's uuidByLocalID/kindByUUID caches are seeded once at construction from catalog.MigrateV1ToV2's already-recorded remote mappings and grow as this run's own Create/Update calls succeed, so a sibling operation later in the same dependency-ordered apply run (a task depending on the plan just created) resolves its parent's Linear UUID without a second round trip, and a fresh process (a later invocation) still resolves any already-linked parent from the persisted map."
  - "buildEntityFields resolves LINEAR_TEAM_ID/parentUUID per entity kind: milestone -> Linear Project ({name, teamIds:[LINEAR_TEAM_ID]}), phase -> Project Milestone ({name, projectId: milestone's UUID}), plan/requirement -> parent/requirement Issue ({title, teamId, projectMilestoneId: phase's UUID}), task -> sub-issue ({title, teamId, parentId: plan's UUID}); every create/update also sends description = the operation's D-14 marker footer."
  - "linear apply now calls apply.CommitResultAtomically (Plan 11's existing, previously-unwired primitive) after every apply.Apply run with a non-empty achieved prefix, folding completed/noop results back into a copy of the loaded remote-mapping map (Status: \"linked\") and journaling the achieved prefix beside the plan file (<plan-file>.journal.json/.report.json). This is the mechanism that makes a later preview/apply against the same repository observe prior progress -- without it, replay could never be a safe no-op."
  - "GOLC_LINEAR_SYNC_WORKDIR (undocumented, test-only) lets the acceptance script point the process transport at an isolated workspace with a fake node_modules/@linear/sdk instead of the real tools/linear-sync tree, without any change to production route resolution or to tools/linear-sync's own compiled cli.ts/adapter.ts/protocol.ts (out of this plan's file scope)."

requirements-completed: [CONF-03, CONF-04, LINR-03, LINR-04]

coverage:
  - id: D1
    description: "The real Go<->Node process transport (ProcessClient) launches only the project-local compiled adapter, enforces a per-call deadline, kills the full process tree on timeout/cancellation, rejects protocol noise, and redacts stderr through internal/security before any error message is built."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/trace/transport/process_test.go#TestScopeTraceTransportProcess (9 subtests: round-trip, newline rejection, protocol noise, deadline+process-tree kill, cancellation, canary-redacted failure, missing-node, missing-adapter, idempotent Close)"
        status: pass
      - kind: integration
        ref: "GOPROXY=off GOFLAGS=-mod=readonly go test -count=1 ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Against a fake official Linear SDK injected into the real compiled adapter, the full required hierarchy (Project Milestone -> Project, Phase -> Project Milestone, parent/requirement Issue, task sub-issue) creates each object exactly once via linear preview --remote + linear apply, an induced create failure stops apply cleanly with no duplicate and no canary leak, a recovery preview+apply completes it, and a full replay (fresh preview --remote + apply) performs zero new create/update calls."
    requirement: LINR-03
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\linear-transport.ps1 -Mode hierarchy"
        status: pass
    human_judgment: false
  - id: D3
    description: "A missing/unreachable compiled adapter fails only linear preview --remote/linear drift --remote --read-only with GOLC_LINEAR_TRANSPORT_UNAVAILABLE; check --offline, build, and the offline core graph stay green."
    requirement: LINR-04
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\linear-transport.ps1 -Mode offline; powershell -NoProfile -File .\\golc.ps1 check --offline"
        status: pass
    human_judgment: false
  - id: D4
    description: "package --foundation deterministically includes the compiled adapter's dist/src/*.js output plus package.json/package-lock.json when present, with no vendored node_modules and no Node product-runtime claim; a checkout without the adapter still packages successfully."
    requirement: CONF-03
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\offline.ps1 -Mode package (byte-identical repeat run)"
        status: pass
      - kind: unit
        ref: "internal/delivery/delivery_test.go (existing golden-manifest suite, unaffected by the optional new entries)"
        status: pass
    human_judgment: false
  - id: D5
    description: ".github/workflows/linear-sync.yml has no pull_request/push/schedule trigger, gates linear-apply behind an exact confirm_apply=CONFIRM plus a protected GitHub Environment, materializes .env from secrets only inside a try/finally, and the runtime PR guard (GuardAgainstPullRequestMutation) remains authoritative against a real, hash-valid plan file under GITHUB_EVENT_NAME=pull_request independent of the workflow YAML."
    requirement: LINR-04
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\linear-transport.ps1 -Mode workflow"
        status: pass
      - kind: integration
        ref: "powershell -NoProfile -File .\\golc.ps1 generate --check; check --offline; test; build; package --foundation; linear validate --offline (full chained gate)"
        status: pass
    human_judgment: false

duration: ~59min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 15: Real Process Transport, Complete Hierarchy Replay, and Protected Remote Workflow Summary

**ProcessClient (Go<->Node NDJSON boundary) plus processLinearClient wired as the production RemoteClient, proving the full Project/Milestone/Issue/sub-issue hierarchy creates once and replays as a safe no-op against a fake official SDK injected into the real compiled adapter, with a protected workflow_dispatch-only GitHub Actions workflow for optional manual apply**

## Performance

- **Duration:** ~59 min (git commit span; extensive upfront design/tracing time not separately timed)
- **Started:** 2026-07-21T08:10:07Z
- **Completed:** 2026-07-21T09:09:13Z
- **Tasks:** 3
- **Files modified:** 12 (4 created, 8 modified)

## Accomplishments

- `internal/trace/transport/process.go` is the real, transport-neutral process boundary: `ProcessClient`/`ProcessConfig`/`RPCError`, `NewProcessClient`/`Call(ctx, []byte)`/`Close`. It launches only an already-verified node executable and compiled adapter script (fails closed with `GOLC_TRANSPORT_ADAPTER_MISSING` before ever starting a process otherwise), enforces one strict single-line JSON request/response exchange with a per-call deadline (context or a configured default), kills the full process tree via `taskkill /T /F` on timeout/cancellation/process failure, and reduces any captured stderr through `internal/security.Redact` before it can ever appear in an error message.
- `internal/command/linear.go`'s new `processLinearClient` implements `apply.RemoteClient` (`ReadByUUID`/`ReadByMarker`/`Create`/`Update`) and a targeted `CaptureSnapshot` entirely by encoding/decoding `tools/linear-sync/src/protocol.ts`'s exact wire shapes over `ProcessClient.Call` -- it lives in package `command` (not `transport`) specifically to avoid a `transport -> apply -> transport` import cycle, since `package apply` already imports `package transport`. `applyRemoteClientFactory` is now wired to `newProcessRemoteClient` in production (previously `nil`).
- Two new self-registered routes: `linear preview --remote --out <path>` (targeted remote snapshot + the same D-17 preview build the existing fixture-based form already uses) and `linear drift --remote --read-only` (the same computation, reporting a summary without writing a file -- safe for read-only CI use per D-16).
- `linear apply` now calls `apply.CommitResultAtomically` (Plan 11's primitive, previously never wired into this route) after every run with a non-empty achieved prefix, folding completed/noop results back into `.planning/linear-map.json` and journaling beside the plan file -- the mechanism that makes a later preview/apply observe prior progress and makes replay a genuine no-op.
- `internal/delivery/foundation.go`'s `FoundationInventory` optionally includes the compiled adapter's `tools/linear-sync/dist/src/*.js` output plus its `package.json`/`package-lock.json` (via a new `collectSortedFilesOptional` that never fails when the directory is absent) -- developer-tooling review/build material, never a claim that Node is part of the GOLC application runtime, and never a vendored `node_modules` tree.
- `.github/workflows/linear-sync.yml` is a new, protected/manual (`workflow_dispatch`-only) workflow: `linear-drift` runs read-only unconditionally; `linear-apply` requires the exact literal `CONFIRM` in `confirm_apply`, the `linear-production` protected GitHub Environment, and both `plan_file`/`plan_id` inputs. Each job materializes `.env` from secrets only inside its own step's `try`/`finally` (plus a separate `always()`-gated cleanup step as defense in depth against cancellation) and never emits a secret to argv/logs/artifacts. A missing credential reports "pending" and exits 0 rather than failing.
- `tests/acceptance/linear-transport.ps1` (new, three modes) drives the real compiled adapter against a small in-process fake `@linear/sdk` injected into an isolated workspace (`GOLC_LINEAR_SYNC_WORKDIR`) to prove, end to end: the complete 72-entity real repository hierarchy (project -> milestone -> phase's project milestone -> parent/requirement issue -> task sub-issue) creates each object exactly once; an induced create failure (a canary-token-laden thrown error) stops `linear apply` cleanly with the failed entity attempted exactly once and the canary text never reaching Go's stdout/stderr; a recovery `linear preview --remote` + `linear apply` completes the remaining entity with zero unexpected duplicates; a full replay performs zero new create/update calls; a missing adapter fails only the two remote routes while `check --offline`/`build` stay green; and the protected workflow's structural safety plus the runtime PR guard's independence from the workflow YAML both hold.

## Task Commits

Each task was committed atomically:

1. **Task 1: Bridge Go to the adapter and prove the complete hierarchy replay** - `cbe20b2` (feat)
2. **Task 2: Complete offline graph and foundation package integration** - `7efdea4` (feat)
3. **Task 3: Protect optional remote drift/apply and run final phase gates** - `b29b747` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/trace/transport/process.go` - `ProcessClient`/`ProcessConfig`/`RPCError`, `NewProcessClient`/`Call`/`Close`; bounded stderr capture, process-tree kill.
- `internal/trace/transport/process_test.go` - `TestScopeTraceTransportProcess`: generic transport-boundary proof against a small Node CommonJS test-double fixture (not the real adapter).
- `internal/command/linear.go` - `processLinearClient` (`ReadByUUID`/`ReadByMarker`/`Create`/`Update`/`CaptureSnapshot`), wire types (`wireOperation`/`wireRecord`/`wireReadResult`/`wireMutationOutcome`/`wireDiagnostic`), `linearEntityKindForOperationKind`, `buildEntityFields`, `newProcessRemoteClient`/`newProcessLinearClient`, `resolveLinearSyncWorkspace`, `linearTransportCallTimeout`; new routes `linear preview --remote`/`linear drift`; `achievedApplyPrefix`/`applyMapPath`/`mergeApplyResultsIntoMap`/`commitApplyResults` wired into `runLinearApply`.
- `internal/trace/apply/model.go` - `fieldsMatch` now compares decoded field content instead of marshaled bytes (bug fix; see Deviations).
- `golc.ps1` - `Command`/`CommandArguments` extraction moved from a declared `param()` block to `$args` (bug fix; see Deviations).
- `tests/acceptance/linear-transport.ps1` - New: `-Mode hierarchy`/`-Mode offline`/`-Mode workflow`, `New-FakeLinearSdkWorkspace`, `Get-JsonArrayCount`/`ConvertTo-JsonArray` (PowerShell 5.1 `ConvertFrom-Json` empty-array-is-`$null` helpers), `Backup-LinearMap`/`Restore-LinearMap`.
- `config/commands.toml` - Documents the three real remote routes and the foundation package's optional adapter inclusion.
- `internal/delivery/graph.go` - Documents why the offline core graph needs no functional change for remote-failure isolation (T-01-46).
- `internal/delivery/foundation.go` - `collectSortedFilesOptional`; `FoundationInventory` includes the compiled adapter when present.
- `.github/workflows/linear-sync.yml` - New protected/manual `linear-drift`/`linear-apply` workflow.
- `.env.example` - Notes that all three remote routes (not only `apply`) read `LINEAR_API_KEY`/`LINEAR_TEAM_ID`, and that `linear-sync.yml` is the only CI place either is materialized.

## Decisions Made

See `key-decisions` in the frontmatter for the full list. In short: `processLinearClient` lives in `package command` to avoid an import cycle; wire records synthesize a `{"title": ...}` fields map since the adapter's own normalized `fields` is always empty; `ReadByMarker` is honestly unsupported over the live wire protocol (no search/list action exists) and recovery instead goes through a fresh preview, never a same-run automatic retry; `CaptureSnapshot` is a targeted read of already-linked entities, not an exhaustive connection scan; and `golc.ps1`'s parameter handling moved off a declared `param()` block to fix a real, previously-undiscovered `--out` ambiguity bug.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `apply/model.go`'s `fieldsMatch` compared marshaled bytes across incompatible JSON formatting**
- **Found during:** Task 1, first real end-to-end `linear apply` run against the fake SDK (every operation reported `GOLC_APPLY_READBACK_MISMATCH` even for a freshly created object with matching content).
- **Issue:** `strictjson.CanonicalEncode` (used to write/read every plan file) uses `json.MarshalIndent`, which reformats nested `json.RawMessage` content (`Operation.After`/`Before`) to match its own nesting depth inside the whole document. `fieldsMatch`'s plain `json.Marshal` byte comparison against that differently-indented, differently-nested `op.After` therefore never matched, even for semantically identical content -- this was latent because no earlier plan's tests ever exercised `apply.Apply`'s engine against a real working `RemoteClient` with a plan loaded from a file.
- **Fix:** `fieldsMatch` now decodes both sides into `map[string]string` and compares by content.
- **Files modified:** `internal/trace/apply/model.go`
- **Verification:** Full `go test -count=1 ./...` green; `tests/acceptance/linear-transport.ps1 -Mode hierarchy`'s 72-entity hierarchy create + recovery + replay all pass.
- **Committed in:** `cbe20b2` (Task 1 commit)

**2. [Rule 1 - Bug] `golc.ps1`'s declared `param()` block made `--out <path>` fail with an "ambiguous parameter" error**
- **Found during:** Task 1, first `linear preview --remote --out <path>` invocation through the actual `golc.ps1` shim (no earlier plan's acceptance script had ever exercised `--out` through the shim itself -- only through `command.NewDefaultCommandRegistry().Execute` directly in Go tests).
- **Issue:** PowerShell 5.1's advanced parameter binder, given a script with a declared `[Parameter(ValueFromRemainingArguments = $true)]` collection, still prefix-matches every `-`/`--`-prefixed token against the full common-parameter set (`-OutVariable`, `-OutBuffer`, ...) before `CommandArguments` ever collects it, failing closed with "the parameter name 'out' is ambiguous."
- **Fix:** `Command`/`CommandArguments` are now extracted from the `$args` automatic variable instead of a declared `param()` block (verified this eliminates the collision without any other behavior change).
- **Files modified:** `golc.ps1`
- **Verification:** `linear status --offline`, bare `golc.ps1` (usage message), and `linear preview --remote --out <path>` all behave correctly; full `check --offline`/`test`/`build`/`package --foundation` regression pass.
- **Committed in:** `cbe20b2` (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 bugs). Both were necessary for this plan's own stated acceptance criteria and `<verify>` commands to be achievable at all -- neither is a scope expansion.

## Issues Encountered

- `tools/linear-sync/src/protocol.ts`'s `Operation` contract exposes only a read-by-immutable-UUID action (no list/search-by-description). This means true D-14 marker-based discovery of a not-yet-linked remote object -- the mechanism `applyUnlinkedOperation`'s `ReadByMarker` call was designed around for recovering from an uncertain create/update outcome within the same apply attempt -- cannot be implemented over the real wire protocol without extending `protocol.ts`/`adapter.ts`/`cli.ts`, which are outside this plan's `files_modified`. `ReadByMarker` therefore always returns not-found for the live transport; safe recovery goes through a fresh `linear preview --remote` (which correctly resolves already-linked entities from the persisted map to a no-op) followed by a human-reviewed re-apply. This is fully proven safe by the acceptance script's induced-failure-then-recovery sequence, but it means an uncertain outcome always requires a fresh preview before retrying, never an automatic same-invocation retry. A future plan extending the Node-side protocol with a real search/connection operation could implement true same-run discovery without any change to `apply/engine.go`'s calling contract.
- This worktree had no bootstrapped `.tools/` state at start; the main checkout's own `.tools/toolchains/go` cache was mirrored via `robocopy /MIR` (gitignored, content-addressed, read-only from `golc.ps1`'s perspective -- the same precedent prior plans in this phase already used), then `golc.ps1 bootstrap --include linear-sync` ran live to provision Node/npm/tsc (network was reachable).

## Known Stubs

- `processLinearClient.ReadByMarker` always returns `found=false` for the live process transport (see "Issues Encountered" above) -- an intentional, documented scope boundary given the fixed wire protocol, not an oversight. Recovery after an uncertain create/update is proven safe through a fresh-preview-then-reapply flow instead of same-run discovery.
- `processLinearClient.CaptureSnapshot` is a targeted read of already-linked entities only, never an exhaustive paginated connection scan -- `linear preview --remote`/`linear drift --remote --read-only` cannot discover a remote object that was never linked through this repository's own `.planning/linear-map.json`. This is sufficient for the documented hierarchy build/replay/recovery flow (proven end to end) but is a narrower "remote read" than `tools/linear-sync/src/adapter.ts`'s own `captureSnapshot`/`fetchAllPages` connection-pagination machinery (Plan 01-14), which this route does not use.

## Threat Flags

None beyond this plan's own declared threat model (T-01-42 through T-01-46, T-01-SC), which is fully mitigated as designed: `ProcessClient`'s strict discriminated JSON, project-local-only executable resolution, deadlines, and unknown-field/noise rejection mitigate T-01-42; exact plan/readback plus per-operation counters (proven by the acceptance script's call-log assertions) mitigate T-01-43; env-only secret handling, allowlisted `TransportDiagnostic` rendering, and `.env` `try`/`finally` cleanup mitigate T-01-44; no PR trigger, protected environment, exact `confirm_apply`, and the runtime `GuardAgainstPullRequestMutation` guard (proven reachable independent of the workflow YAML) mitigate T-01-45; the offline core graph's structural independence from every Linear remote route mitigates T-01-46; and the compiled adapter's deterministic, non-vendored inclusion in the foundation package mitigates T-01-SC.

## User Setup Required

None for offline development -- every command in this plan's scope (`build`, `test`, `check --offline`, `package --foundation`, `linear preview --remote`/`linear drift --remote --read-only`/`linear apply` against a missing adapter) works without any credential. Real Linear synchronization requires a contributor or CI operator to configure `LINEAR_API_KEY`/`LINEAR_TEAM_ID` (via `.env` locally, or the `linear-production` protected GitHub Environment's secrets in CI) and to explicitly run `.github/workflows/linear-sync.yml` with `confirm_apply=CONFIRM` for a reviewed apply -- this remains pending/unauthorized in this session, matching CONTEXT's "leave real-service smoke pending unless separately authorized."

## Next Phase Readiness

- This is the final plan (01-15) of Phase 1 (offline-foundation-and-delivery-traceability); all 29 plans are now complete.
- The real process transport, the complete hierarchy preview/apply/replay proof, the protected workflow, and the foundation package's optional adapter inclusion together close out every plan-15-scoped item in `01-VALIDATION.md`'s per-task verification map (rows `01-15-01`/`01-15-02`/`01-15-03`).
- A future plan wanting true same-run discovery-after-uncertain-outcome, or a full paginated remote snapshot for `linear preview --remote`/`linear drift`, has the exact seam to extend: `tools/linear-sync/src/protocol.ts`'s `Operation` union (a new list/search action) plus `processLinearClient`'s `ReadByMarker`/`CaptureSnapshot`, with no change required to `apply/engine.go`'s calling contract.

## Self-Check: PASSED

- All four created files verified present on disk: `internal/trace/transport/process.go`, `internal/trace/transport/process_test.go`, `tests/acceptance/linear-transport.ps1`, `.github/workflows/linear-sync.yml`.
- Commits `cbe20b2`, `7efdea4`, and `b29b747` verified present in `git log --oneline` on branch `worktree-agent-adfcc0f94d876344d`; `git diff --diff-filter=D --name-only` against each commit's parent reports zero deleted files for all three.
- `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, and `go test -count=1 ./...` all exit 0 (11 packages, all `ok`) with the pinned Go 1.26.5 toolchain.
- `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode hierarchy` exits 0 (full 72-entity hierarchy create/recovery/replay).
- `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode offline` exits 0, followed by `check --offline` exits 0.
- `powershell -NoProfile -File .\tests\acceptance\linear-transport.ps1 -Mode workflow` exits 0, followed by the full chained gate (`generate --check`, `check --offline`, `test`, `build`, `package --foundation`, `linear validate --offline`) all exit 0.
- `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode core` and `-Mode package` both remain green (repeated `package --foundation` byte-identical), confirming no regression to Plan 01-06/01-20's existing acceptance.
- `powershell -NoProfile -File .\tests\acceptance\command-parity.ps1` remains green, confirming no regression to Plan 01-07's PR-parity acceptance.
- `git status --short` is clean of any test-run artifact leakage; `.planning/linear-map.json` verified byte-identical to its committed state after every acceptance run (backed up/restored around the hierarchy and workflow tests).

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
