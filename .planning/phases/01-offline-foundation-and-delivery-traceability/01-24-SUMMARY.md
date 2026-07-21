---
phase: 01-offline-foundation-and-delivery-traceability
plan: 24
subsystem: linear-traceability
tags: [go, jsonschema, draft-2020-12, linear, apply, offline, credential-free]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 04
    provides: internal/contracts's exclusive SchemaDescriptor registry with MustRegisterSchema/RegisteredSchemas/GenerateAll/CheckDrift
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 11
    provides: internal/trace/apply package (RemoteClient, Apply/RunApply, ValidatePlanIntegrity/ValidatePlanFreshness/GuardAgainstPullRequestMutation, Report/OperationResult)
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 22
    provides: internal/command/linear.go's "linear" scope and the linear-map contract/route precedent this plan extends
provides:
  - Registered "linear-plan" and "linear-report" SchemaDescriptors (schemas/linear-plan.schema.json, schemas/linear-report.schema.json) through the unchanged Plan 04 registry
  - Self-registered exact route "linear apply {plan-file} --plan-id <id>" with strict decode-before-typed-use validation (duplicate/unknown JSON, tampered plan_id, out-of-canonical-order operations, malformed D-13 conflicts, illegal planned+conflicted local id)
  - Injected RemoteClientFactory boundary (applyRemoteClientFactory, nil by default) that fails GOLC_LINEAR_TRANSPORT_UNAVAILABLE before any credential/subprocess/mutation access
affects: [linear-traceability, contract-generation]

tech-stack:
  added: []
  patterns:
    - "linear_plan.go continues linear.go's leaf-package precedent: LinearPlanSchema/LinearReportSchema are fresh, purpose-built Go type projections of reconcile.Plan/apply.Report's JSON shape (not a direct reflection), so internal/contracts gains no new dependency on internal/trace/reconcile or internal/trace/apply -- only internal/command/linear.go imports those packages to perform the actual strict validation."
    - "The strict decode-before-typed-use gate (duplicate/unknown/unsorted/bad-digest/invalid-conflict/illegal-transition) composes existing proven entrypoints rather than reimplementing them: strictjson.DecodeStrict (duplicate/unknown), apply.ValidatePlanIntegrity (bad-digest/schema), reconcile.SortOperations re-derivation (unsorted -- an independent check beyond ValidatePlanIntegrity's self-hash-consistency alone, since a tampered plan that reorders operations and then recomputes its own hash over the new order would otherwise pass integrity unchallenged), and two new local checks (validateConflictsWellFormed, validateNoConflictedOperations) for invalid-conflict/illegal-transition."
    - "RemoteClientFactory is a plain package-level function-typed var (applyRemoteClientFactory), nil by default: 'linear apply' checks it for nil immediately after the --plan-id match and before any GuardAgainstPullRequestMutation/catalog/apply.Apply call, so a missing factory fails GOLC_LINEAR_TRANSPORT_UNAVAILABLE before any credential, subprocess, or mutation access is ever attempted. No ProcessClient (or any other concrete apply.RemoteClient implementation) is declared anywhere in this codebase yet."

key-files:
  created:
    - internal/contracts/linear_plan.go
    - internal/contracts/linear_plan_test.go
    - schemas/linear-plan.schema.json
    - schemas/linear-report.schema.json
  modified:
    - internal/command/linear.go

key-decisions:
  - "Contracts stays a pure schema-projection file: linear_plan.go has zero import beyond github.com/invopop/jsonschema, matching linear.go's established 'fresh types, no new internal dependency' pattern. All real decode/validate logic that needs reconcile.Plan/apply typed values lives in internal/command/linear.go instead, matching the plan's own key_links (contracts->generate.go only; command/linear.go->apply/engine.go via the injected factory)."
  - "'unsorted' is proven as an independent check, not a side effect of ValidatePlanIntegrity: linear_plan_test.go's unsorted/invalid-conflict/illegal-transition fixtures deliberately tamper with a valid plan and then recompute a self-consistent plan_id (mirroring apply/guard.go's own planBodyMirror shape) before asserting rejection, so a hash-self-consistent-but-illegal plan is caught by the new Go-level checks, not merely by digest mismatch."
  - "apply.Apply (the lower-level per-operation engine from Plan 11's engine.go), not RunApply, is what 'linear apply' calls: the route's exact grammar (\"linear apply {plan-file} --plan-id <id>\") carries no --snapshot/--journal argument, so freshness recomputation (ValidatePlanFreshness) and journaled resume are out of this plan's scope; GuardAgainstPullRequestMutation is still applied explicitly before calling the factory, matching CONTEXT D-16's independent-of-workflow-YAML requirement."
  - "GOLC_LINEAR_TRANSPORT_UNAVAILABLE (not GOLC_LINEAR_APPLY_TRANSPORT_UNAVAILABLE) is the exact diagnostic prefix, chosen to contain the literal substring the phase's must_haves truth names verbatim (\"fails LINEAR_TRANSPORT_UNAVAILABLE\")."
  - "linear_plan_test.go exercises 'linear apply' end to end through command.NewDefaultCommandRegistry().Execute(...) rather than calling unexported command-package helpers directly: contracts_test (external test package) can safely import internal/command (which itself imports internal/contracts) without an import cycle, exactly like generate_test.go already does for its own scope declaration."

requirements-completed: [LINR-03, LINR-04]

coverage:
  - id: D1
    description: "internal/contracts/linear_plan.go registers exactly one \"linear-plan\" descriptor and exactly one \"linear-report\" descriptor; the unchanged global generator (generate.go, not edited by this plan) reaches both through RegisteredSchemas and writes/checks their two committed schema paths."
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "internal/contracts/linear_plan_test.go#TestScopeLinearPlanContract/linear-plan_and_linear-report_are_each_registered_exactly_once, .../global_generation_and_drift_check_reach_both_descriptors"
        status: pass
      - kind: integration
        ref: "Built cmd/golc-project from source (host toolchain) and ran golc-project generate --check (reported drift for the two not-yet-committed schemas), then generate (wrote schemas/linear-plan.schema.json and schemas/linear-report.schema.json), then generate --check again (exit 0, no drift)."
        status: pass
    human_judgment: false
  - id: D2
    description: "linear apply requires an explicit plan file and --plan-id, and rejects a --plan-id that does not match the loaded plan's own plan_id, before any transport access."
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "internal/contracts/linear_plan_test.go#TestScopeLinearPlanContract/linear_apply_requires_an_explicit_plan_file_and_a_matching_plan_id"
        status: pass
      - kind: integration
        ref: "golc-project linear apply <preview.json> --plan-id deadbeef against a real linear preview built from the repository's own catalog exited 1 with GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH naming both the given and actual plan_id; golc-project linear apply (no args) and linear apply --plan-id abc (no plan file) both exited 2 with GOLC_LINEAR_USAGE."
        status: pass
    human_judgment: false
  - id: D3
    description: "linear apply rejects duplicate JSON member names, unknown JSON fields, a tampered plan_id, an out-of-canonical-order operation list, a structurally malformed D-13 conflict, and a local id that is both planned and unresolved-conflicted -- all before any typed value reaches apply.Apply."
    requirement: LINR-03
    verification:
      - kind: unit
        ref: "internal/contracts/linear_plan_test.go#TestScopeLinearPlanContract/linear_apply_rejects_duplicate_JSON_member_names_before_typed_use, .../linear_apply_rejects_unknown_JSON_fields_before_typed_use, .../linear_apply_rejects_a_plan_whose_plan_id_no_longer_matches_its_recomputed_hash, .../linear_apply_rejects_an_out-of-canonical-order_operation_list, .../linear_apply_rejects_a_structurally_malformed_conflict, .../linear_apply_rejects_a_local_id_that_is_both_planned_and_unresolved-conflicted"
        status: pass
    human_judgment: false
  - id: D4
    description: "With no RemoteClientFactory wired, linear apply fails GOLC_LINEAR_TRANSPORT_UNAVAILABLE before any credential, subprocess, or mutation access; no ProcessClient (or other concrete transport) is referenced anywhere in the codebase."
    requirement: LINR-04
    verification:
      - kind: unit
        ref: "internal/contracts/linear_plan_test.go#TestScopeLinearPlanContract/linear_apply_fails_LINEAR_TRANSPORT_UNAVAILABLE_before_any_credential_subprocess_or_mutation_access"
        status: pass
      - kind: integration
        ref: "golc-project linear apply <real-preview.json> --plan-id <matching-plan_id> against the real repository (built with a real linear preview through catalog.MigrateV1ToV2 and a credential-free fake snapshot) exited 1 with GOLC_LINEAR_TRANSPORT_UNAVAILABLE. grep of internal/command and internal/trace confirms no \"ProcessClient\" identifier exists anywhere in the codebase."
        status: pass
    human_judgment: false
  - id: D5
    description: "generate/generate --check, go build/vet/test/-race all pass offline (GOPROXY=off, -mod=readonly) against the host Go 1.26.5 toolchain (exact pin match), and go.mod/go.sum are byte-unchanged."
    requirement: "LINR-03, LINR-04"
    verification:
      - kind: integration
        ref: "GOPROXY=off GOFLAGS=-mod=readonly go build ./..., go vet ./..., go test -count=1 ./..., and go test -count=1 -race ./internal/contracts/... ./internal/command/... all exit 0; go.mod/go.sum SHA-256 hashes identical before and after the full run."
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 24: Strict Linear Plan/Report Contracts and Exact Apply Route Summary

**Registered the "linear-plan"/"linear-report" JSON Schema contracts through the unchanged Plan 04 registry and self-registered the exact "linear apply {plan-file} --plan-id <id>" route with a full decode-before-typed-use gate (duplicate/unknown JSON, tampered plan_id, unsorted operations, malformed D-13 conflicts, illegal planned+conflicted state) plus an injected, currently-unwired RemoteClientFactory boundary that fails LINEAR_TRANSPORT_UNAVAILABLE before any credential/subprocess/mutation access**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-07-21T04:22:00Z (approx.)
- **Completed:** 2026-07-21T05:17:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 5 (4 created, 1 modified)

## Accomplishments

- `internal/contracts/linear_plan.go` registers `LinearPlanSchema`/`LinearReportSchema` (plus their nested `LinearPlanOperationSchema`/`LinearPlanConflictSchema`/`LinearReportResultSchema` block types) as fresh, purpose-built Go type projections of `reconcile.Plan`/`apply.Report`'s real JSON shape -- mirroring `linear.go`'s established leaf-package pattern exactly -- and registers each exactly once through `MustRegisterSchema` under the stable names `linear-plan` (`schemas/linear-plan.schema.json`) and `linear-report` (`schemas/linear-report.schema.json`). `internal/contracts/generate.go` is untouched; the unchanged `GenerateAll`/`CheckDrift` reach both descriptors automatically through `RegisteredSchemas`.
- `internal/command/linear.go` self-registers the exact route `linear apply` (grammar: `linear apply {plan-file} --plan-id <id>`), whose handler `runLinearApply` composes a full decode-before-typed-use gate in `decodeAndValidatePlanStrict`: `strictjson.DecodeStrict` rejects duplicate JSON member names and unknown fields before any typed decode; `apply.ValidatePlanIntegrity` (Plan 11) rejects a tampered `plan_id`; a new `validateOperationsSorted` re-derives the canonical D-17 order via `reconcile.SortOperations` and rejects any out-of-order operation list (an independent check from hash self-consistency -- a plan that reorders operations and recomputes its own hash over the new order would otherwise still pass integrity alone); a new `validateConflictsWellFormed` rejects a structurally incomplete D-13 conflict; and a new `validateNoConflictedOperations` rejects a local id that is simultaneously planned and unresolved-conflicted (an illegal joint state `BuildPlan` itself never produces).
- After decode/validation, `runLinearApply` requires `--plan-id` to exactly match the loaded plan's own `plan_id` (`GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH` otherwise), then checks the new package-level `applyRemoteClientFactory` (`type RemoteClientFactory func(root string) (apply.RemoteClient, error)`) for `nil`. Left unwired in this plan (no `ProcessClient` or any other concrete `apply.RemoteClient` implementation exists in the codebase), a missing factory fails `GOLC_LINEAR_TRANSPORT_UNAVAILABLE` before `GuardAgainstPullRequestMutation`, the factory call, `catalog.MigrateV1ToV2`, or `apply.Apply` are ever reached.
- `internal/contracts/linear_plan_test.go` registers the `linear-plan-contract` quick-test scope and `TestScopeLinearPlanContract` (10 subtests): registry-exactly-once presence, generate/drift-check reachability, and a full black-box exercise of the self-registered `linear apply` route via `command.NewDefaultCommandRegistry().Execute(...)` -- covering the plan-id-match/usage-error path and all six malformed/illegal-state rejections (duplicate JSON, unknown field, bad digest, unsorted operations, invalid conflict, illegal planned+conflicted transition) plus the missing-factory `LINEAR_TRANSPORT_UNAVAILABLE` path. Test fixtures for the tamper cases are built by taking a real `reconcile.BuildPlan` output and deliberately corrupting it, then (for the checks independent of hash self-consistency) recomputing a self-consistent `plan_id` via a local mirror of `apply/guard.go`'s own `planBodyMirror` shape, proving each new check fires on its own merit rather than merely as a side effect of `ValidatePlanIntegrity`.
- Verified end to end against the real repository with a host-built `golc-project` binary: `generate --check` (drift for the two new schemas) -> `generate` (wrote both) -> `generate --check` (clean); a real `linear preview` against the repository's own catalog and a credential-free empty fake snapshot; `linear apply <preview> --plan-id <wrong>` (`GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH`); `linear apply <preview> --plan-id <matching>` (`GOLC_LINEAR_TRANSPORT_UNAVAILABLE`); and `linear apply` / `linear apply --plan-id abc` with no plan file (`GOLC_LINEAR_USAGE`, exit 2).

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: linear-plan/linear-report contract and apply route** - `b4c73ec` (test)
2. **GREEN - Task 1: registry, apply route, and strict decode/validate gate** - `c1a1cb7` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/contracts/linear_plan.go` - `LinearPlanSchema`/`LinearReportSchema` fresh type projections plus their `MustRegisterSchema` self-registrations under `linear-plan`/`linear-report`.
- `internal/contracts/linear_plan_test.go` - `TestScopeLinearPlanContract`, the `linear-plan-contract` quick-test scope declaration, and full black-box coverage of the `linear apply` route via the default command registry.
- `internal/command/linear.go` - Added the self-registered `linear apply` route, `RemoteClientFactory`/`applyRemoteClientFactory`, `parseApplyArgs`, `decodeAndValidatePlanStrict` and its three composed validators, and `runLinearApply`.
- `schemas/linear-plan.schema.json` - Committed Draft 2020-12 projection of the canonical `reconcile.Plan` shape, generated via `generate`.
- `schemas/linear-report.schema.json` - Committed Draft 2020-12 projection of the canonical `apply.Report` shape, generated via `generate`.

## Decisions Made

See `key-decisions` in frontmatter for the full list. In short: `internal/contracts/linear_plan.go` stays a pure schema-projection leaf file with zero new internal dependency (matching `linear.go`'s precedent); all real strict-decode/validate logic lives in `internal/command/linear.go`, matching the plan's own `key_links` (contracts talks only to `generate.go`; the command file talks to `apply/engine.go` via the injected factory); "unsorted"/"invalid-conflict"/"illegal-transition" are proven as independent checks (not merely `ValidatePlanIntegrity` side effects) via deliberately self-consistent-hash tampered test fixtures; `apply.Apply` (not `RunApply`) is what the route calls, since the exact route grammar carries no `--snapshot`/`--journal` argument and freshness/resume are out of this plan's stated scope; and the transport-unavailable diagnostic is spelled `GOLC_LINEAR_TRANSPORT_UNAVAILABLE` to contain the phase's must_haves truth's exact substring verbatim.

## Deviations from Plan

None -- plan executed exactly as written. All five frontmatter-declared files were created/modified exactly as listed; the plan's `<behavior>` bullet ("Duplicate/unknown/unsorted/bad-digest/invalid-conflict/illegal-transition inputs fail before apply") is realized as six distinct, individually-tested checks composed in `decodeAndValidatePlanStrict`, matching the acceptance criteria's "Malformed ordering/digest/conflict/report transitions fail before typed apply" without reimplementing any logic that Plan 04's registry, Plan 11's `apply` package, or `internal/trace/reconcile` already own.

## Issues Encountered

- This worktree has no bootstrapped `.tools/` pinned toolchain (consistent with every prior plan's documented condition in this phase). The plan's literal `<verify>` commands (`powershell -NoProfile -File .\golc.ps1 generate --check` / `test --quick --scope linear-plan-contract`) could not be invoked through the shim without a multi-minute bootstrap first. Verified instead with the host Go toolchain, which is `go1.26.5 windows/amd64` -- an exact match for the pinned `config/toolchain.toml` version -- running `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `go test -count=1 -race ./internal/contracts/... ./internal/command/...`, `go test -list '^TestScopeLinearPlanContract$' ./...` (confirming the marker resolves in `internal/contracts` exactly the way `test --quick --scope linear-plan-contract` would find it), and building `cmd/golc-project` from source to exercise `generate`/`generate --check`/`linear preview`/`linear apply` directly against the real repository. This mirrors the identical substitution documented in every prior plan's summary in this phase (04, 09, 10, 11, 18, 19, 22, 23).

## Known Stubs

None. `linear apply`'s decode/validate gate, plan-id matching, and transport-unavailable failure path are fully wired to real logic and proven both by unit tests and by a real host-built binary against the repository's own catalog. The absence of a concrete `apply.RemoteClient`/`ProcessClient` implementation is this plan's explicit, documented scope boundary (CONTEXT: the real GraphQL-backed adapter belongs to a later plan), not a stub -- `applyRemoteClientFactory` is the exact, already-tested injection point that later plan wires without any change to this plan's files.

## Threat Flags

None -- no new surface beyond the plan's threat model. T-01-31 (apply route/binding tampering/repudiation) is mitigated by the exact `--plan-id` match requirement plus `apply.ValidatePlanIntegrity`'s hash self-consistency check. T-01-34 (contracts/registration tampering) is mitigated by the exactly-once named descriptor registration and unchanged sorted global traversal, matching Plan 04's/Plan 22's precedent. T-01-SC (dependency tampering) is mitigated by the existing exact Go lock; no npm install and no new Go module dependency were introduced.

## User Setup Required

None -- schema generation and the `linear apply` decode/validate/transport-unavailable gate are pure Go, operate only on already-committed repository files and a locally supplied plan file, and never touch the network or a credential. The route cannot yet perform an actual remote mutation (`applyRemoteClientFactory` is nil), which is this plan's explicit scope boundary.

## Next Phase Readiness

- A later plan (the real GraphQL-backed process transport) can assign `applyRemoteClientFactory` from its own package-level var initializer in `internal/command` (or a new file in the same package) without any change to `linear.go`'s route declaration, parsing, or strict-validation gate -- exactly the "injected factory" seam this plan establishes.
- `schemas/linear-plan.schema.json`/`schemas/linear-report.schema.json` are now committed, reviewable Draft 2020-12 contracts a future documentation or client-generation step can consume directly.
- `decodeAndValidatePlanStrict`'s six composed checks (duplicate/unknown/bad-digest/unsorted/invalid-conflict/illegal-transition) are now the standing gate every future `linear apply` invocation runs through, regardless of which concrete `RemoteClient` is eventually wired in.

## Self-Check: PASSED

- All five owned files verified present on disk: `internal/contracts/linear_plan.go`, `internal/contracts/linear_plan_test.go`, `internal/command/linear.go`, `schemas/linear-plan.schema.json`, `schemas/linear-report.schema.json`.
- Commits `b4c73ec` (test) and `c1a1cb7` (feat) verified present in `git log --oneline` on branch `worktree-agent-ac7bb7fabc8ec7b6d`; `git diff --diff-filter=D --name-only b4c73ec~1 c1a1cb7` reports zero deleted files across both commits; working tree clean before this summary.
- `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `go test -count=1 -race ./internal/contracts/... ./internal/command/...` all exit 0 from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly); `go.mod`/`go.sum` SHA-256 hashes are identical before and after the full run; the host-built `golc-project` binary's `generate`, `generate --check`, `linear preview`, and `linear apply` all exited with the expected diagnostics against the real repository.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*

## Self-Check: PASSED (verified)

- Files re-verified present on disk: `internal/contracts/linear_plan.go`, `internal/contracts/linear_plan_test.go`, `internal/command/linear.go`, `schemas/linear-plan.schema.json`, `schemas/linear-report.schema.json`, and this SUMMARY.md itself.
- Commits `b4c73ec` (test), `c1a1cb7` (feat), and `63efe01` (docs, this summary) all verified present in `git log --oneline` on branch `worktree-agent-ac7bb7fabc8ec7b6d`; working tree clean.
