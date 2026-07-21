---
phase: 01-offline-foundation-and-delivery-traceability
plan: 11
subsystem: linear-traceability
tags: [go, linear, apply, reconcile, transport, journal, offline]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 10
    provides: internal/trace/reconcile canonical model (Intent/RemoteObservation/Operation/Conflict/Plan), SHA-256 plan digests, PlanID, and RenderMarker/ParseMarker/ValidateMarkerIdentity
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 23
    provides: internal/trace/reconcile/diff.go (BuildCompletePreview, ArchivePreview/BuildArchivePreview/BuildUnlinkPreview) and internal/trace/transport (Transport interface, Snapshot/RemoteRecord, Mutation, credential-free Fake)
provides:
  - internal/trace/apply package (RemoteClient per-operation contract, Apply/RunApply state machine, ValidatePlanIntegrity/ValidatePlanFreshness/GuardAgainstPullRequestMutation/ApplyRemoval guards, Journal/ResumePrefix/CommitResultAtomically)
  - Three self-contained apply scenario fixtures (remote-stale, remote-timeout-after-create, remote-partial-apply) plus the canonical linear-apply-report.json golden
  - The linear-apply-core and linear-apply-resume quick-test scopes (TestScopeLinearApplyCore, TestScopeLinearApplyResume)
affects: [01-24, 01-26, 01-27, linear-traceability]

tech-stack:
  added: []
  patterns:
    - "guard.go independently recomputes plan_id from a local mirror of reconcile's private planBody shape (same JSON field names/order) rather than exporting reconcile internals, so ValidatePlanIntegrity proves the plan's own hash binding without reconcile.Plan trusting itself"
    - "ValidatePlanFreshness (D-18) is 'recompute and compare' rather than a bespoke diff: it re-runs reconcile.BuildCompletePreview against the caller's current intent/mapping/snapshot/baselines and requires an exact plan_id match, so any relevant drift -- changed fields, a newly discovered conflict, a newly ambiguous snapshot -- is caught by the exact same hash the preview itself is bound by"
    - "RemoteClient (model.go) is a narrower, per-operation contract than transport.Transport: ReadByUUID/ReadByMarker/Create/Update, with no method that can express archive/unlink at all -- removal only ever flows through ApplyRemoval (guard.go), which is the sole caller of transport.Transport.Apply with a MutationArchive/MutationUnlink"
    - "applyUnlinkedOperation always calls ReadByMarker before Create (timeout-after-create discovery, CONTEXT D-17/D-21): a prior interrupted create that actually succeeded remotely is found and treated as achieved/updated instead of retried into a duplicate"
    - "applyOperations stops attempting further operations at the first non-completed/non-noop result (retryable error, stale before-state, readback mismatch, or a blocked dependency), so the achieved outcome is always an exact contiguous prefix -- the exact invariant journal.go's ResumePrefix depends on for safe, replay-free resume"
    - "Task 1 (model.go/engine.go/guard.go) and Task 2 (journal.go, engine.go's RunApply) are separable compile units: engine.go's lower-level Apply(client, plan, mappings) has no journal dependency, and RunApply -- appended to engine.go in Task 2 -- is the only place Journal/ResumePrefix are referenced, matching the frontmatter's engine.go-in-both-tasks file ownership"

key-files:
  created:
    - internal/trace/apply/model.go
    - internal/trace/apply/engine.go
    - internal/trace/apply/guard.go
    - internal/trace/apply/journal.go
    - internal/trace/apply/apply_test.go
    - tests/fixtures/linear/remote-stale.json
    - tests/fixtures/linear/remote-timeout-after-create.json
    - tests/fixtures/linear/remote-partial-apply.json
    - tests/golden/linear-apply-report.json
  modified: []

key-decisions:
  - "ValidatePlanIntegrity/ValidatePlanFreshness are guard.go's two independent tamper/staleness gates (CONTEXT D-17/D-18): integrity recomputes plan_id from the plan's own bytes (catches a hand-edited or forged plan even if intent/mapping/snapshot never changed); freshness recomputes the whole preview from current inputs and requires an exact plan_id match (catches real-world drift). Both run before ResumePrefix and before any RemoteClient call."
  - "GuardAgainstPullRequestMutation (CONTEXT D-16) is a Go-level check independent of workflow YAML: it inspects an injected environment lookup for GITHUB_EVENT_NAME=pull_request and blocks RunApply entirely, so a misconfigured or bypassed CI workflow file cannot let PR-triggered CI mutate Linear."
  - "RemoteClient (model.go) deliberately has no archive/unlink method. Only ApplyRemoval, acting on an already-reviewed reconcile.ArchivePreview through the existing transport.Transport contract, can ever produce a MutationArchive/MutationUnlink -- removal can never be a side effect of a normal create/update Apply/RunApply call (CONTEXT D-15)."
  - "A rate-limited/retryable RemoteClient error (RetryableError) and a timeout are treated as two distinct failure semantics on purpose: a RetryableError means the client knows the mutation was rejected/throttled before taking effect (safe to retry with a fresh Create/Update attempt, proven by the remote-partial-apply.json fixture's create-fails-then-succeeds-on-retry flow), while an uncertain outcome after a create is always resolved by ReadByMarker discovery first (remote-timeout-after-create.json), never by blindly retrying Create."
  - "classifyBlocked runs once, up front, before any operation is attempted: an operation whose DependsOn parent has no operation in this plan and no already-linked remote mapping (most commonly a D-13-conflicted parent) is marked StatusBlocked and stops the run at that position -- deterministic and independent of whichever RemoteClient call would have run next."
  - "CommitResultAtomically's three-file atomicity is best-effort on a local filesystem, matching catalog.WriteMigration's existing single-file pattern extended to three: every payload is canonically encoded and staged to a temp file before any destination is touched, and only after all three stage successfully are they renamed into place in order (map, journal, report), with cleanup of already-staged temps on any earlier failure. A crash between the map rename and the journal rename is the one residual non-atomic window; this is documented in code rather than solved with a lock file, since no other writer of these three files exists yet."
  - "LINR-03/LINR-04 are NOT marked complete in this plan, matching 01-23's and 01-08's precedent: LINR-03 requires 'preview and run', but this plan only builds the apply engine as a library -- there is still no CLI route (`linear apply`) to invoke it; that is 01-24's explicit scope. LINR-04's real GraphQL partial-error/rate-limit normalization against a live transport stays owned by 01-26/01-27; this plan only defines and proves the generic RetryableError/StatusPending contract a fake or real RemoteClient reports through."

requirements-completed: []

coverage:
  - id: D1
    description: "ValidatePlanIntegrity rejects a plan whose plan_id no longer matches its own recomputed canonical hash, or whose schema_version has drifted, before any mutation or freshness check runs"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/ValidatePlanIntegrity_accepts_an_untampered_plan_and_rejects_a_tampered_schema_or_hash"
        status: pass
    human_judgment: false
  - id: D2
    description: "ValidatePlanFreshness (D-18) accepts an unchanged plan and rejects one whose recomputed preview no longer matches current repository/remote state; RunApply rejects a stale plan before ever calling a RemoteClient"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/ValidatePlanFreshness_accepts_an_unchanged_plan_and_rejects_the_remote-stale_fixture_before_any_mutation"
        status: pass
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyResume/RunApply_rejects_the_remote-stale_fixture_before_ever_touching_a_RemoteClient"
        status: pass
    human_judgment: false
  - id: D3
    description: "GuardAgainstPullRequestMutation blocks mutating apply from a pull_request-triggered CI event independent of workflow YAML, and RunApply refuses to run at all under it"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/GuardAgainstPullRequestMutation_blocks_a_pull_request_CI_event_independently_and_allows_everything_else"
        status: pass
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyResume/RunApply_refuses_to_run_at_all_from_a_pull_request_CI_event"
        status: pass
    human_judgment: false
  - id: D4
    description: "Apply achieves a clean create plan exactly once, and a later re-preview against the now-linked mapping/remote state replays every operation as an exact no-op with no extra create or update call"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/Apply_completes_a_clean_create_plan_and_a_later_re-preview_replays_as_an_exact_no-op"
        status: pass
    human_judgment: false
  - id: D5
    description: "A create whose remote outcome is unknown (timeout after create) is discovered by its exact D-14 marker footer before any retry, so Create is never called for an already-achieved object and no duplicate is left remotely"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/Apply_discovers_an_achieved_timeout-after-create_object_by_its_exact_marker_footer_before_ever_creating_again"
        status: pass
    human_judgment: false
  - id: D6
    description: "ApplyRemoval is the only path in the package that can archive or unlink, and it enforces the same pull-request guard as the regular create/update Apply path"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyCore/ApplyRemoval_is_the_only_path_that_can_archive_or_unlink,_and_it_enforces_the_same_pull-request_guard"
        status: pass
    human_judgment: false
  - id: D7
    description: "A rate-limited/retryable mutation mid-plan stops all further writes and reports every operation's exact state (completed/pending with retry metadata/stopped-pending) matching the committed golden apply report, and a same-plan resume via ResumePrefix skips the achieved prefix and never replays it"
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyResume/RunApply_stops_all_writes_on_a_retryable_error,_reports_every_operation_state_plus_retry_metadata,_and_resumes_the_exact_achieved_prefix_without_replay"
        status: pass
    human_judgment: false
  - id: D8
    description: "ResumePrefix rejects a journal bound to a different plan_id, an out-of-order/too-long journal, and a journal whose already-achieved remote state has drifted since it was written"
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyResume/ResumePrefix_rejects_a_journal_bound_to_a_different_plan,_an_out-of-order_journal,_and_drifted_already-achieved_state"
        status: pass
    human_judgment: false
  - id: D9
    description: "CommitResultAtomically persists the updated map, journal, and report as one validated result and leaves every destination file untouched when a later staging step fails"
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "internal/trace/apply/apply_test.go#TestScopeLinearApplyResume/CommitResultAtomically_writes_map/journal/report_as_one_validated_result_and_leaves_prior_state_intact_on_failure"
        status: pass
    human_judgment: false

duration: ~65min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 11: Exact-Plan Apply, Stale Rejection, Safe Replay, and Journaled Resume Summary

**New internal/trace/apply package (RemoteClient per-operation contract, Apply/RunApply state machine, ValidatePlanIntegrity/ValidatePlanFreshness/GuardAgainstPullRequestMutation/ApplyRemoval guards, and an atomic achieved-prefix Journal), proven against a credential-free in-memory fake and three self-contained scenario fixtures for stale rejection, timeout-after-create discovery, and rate-limited partial apply with resume**

## Performance

- **Duration:** ~65 min
- **Started:** 2026-07-21T02:05:00Z (approx.)
- **Completed:** 2026-07-21T03:10:00Z (approx.)
- **Tasks:** 2 (both TDD)
- **Files modified:** 9 (9 created, 0 modified)

## Accomplishments

- `internal/trace/apply/model.go` defines the per-operation `RemoteClient` interface (`ReadByUUID`, `ReadByMarker`, `Create`, `Update` -- deliberately no archive/unlink method), `RemoteState`, `RetryableError`, `OperationStatus`/`OperationResult` (completed/noop/pending/blocked), and `Report`.
- `internal/trace/apply/guard.go` implements `ValidatePlanIntegrity` (recomputes `plan_id` from the plan's own canonical body via a local mirror of reconcile's private `planBody` shape), `ValidatePlanFreshness` (CONTEXT D-18: rejects a plan whose recomputed preview from current intent/mapping/snapshot/baselines no longer produces the same `plan_id`), `GuardAgainstPullRequestMutation` (CONTEXT D-16: blocks on `GITHUB_EVENT_NAME=pull_request` independent of workflow YAML), and `ApplyRemoval` (the sole function that can call `transport.Transport.Apply` with an archive/unlink `Mutation`, CONTEXT D-15).
- `internal/trace/apply/engine.go` implements the exact-plan apply state machine: `classifyBlocked` (pre-run dependency classification), `applyLinkedOperation`/`applyUnlinkedOperation` (read-before-write, exactly one mutation, immediate readback; discover by D-14 marker footer before any create), `Apply` (per-operation engine, no journal dependency -- Task 1's exported entrypoint), and `RunApply` (Task 2's full orchestration: integrity -> freshness -> PR guard -> `ResumePrefix` -> `Apply`, returning a merged `Report` and updated `Journal`).
- `internal/trace/apply/journal.go` implements `Journal`, `LoadJournal`, `ResumePrefix` (CONTEXT D-21: binds to an exact `plan_id`, requires an exact ordered achieved prefix, and re-reads every journaled object's current remote state before trusting it), and `CommitResultAtomically` (stages the updated map/journal/report to temp files and only renames them into place after every payload validates and stages).
- `internal/trace/apply/apply_test.go` registers the `linear-apply-core`/`linear-apply-resume` quick-test scopes with `TestScopeLinearApplyCore` (6 subtests: plan tamper/hash rejection, D-18 staleness, the PR guard, clean-apply-then-replay-no-op, timeout-after-create discovery, and `ApplyRemoval` exclusivity) and `TestScopeLinearApplyResume` (5 subtests: `RunApply`-level staleness/PR-guard coverage, rate-limited stop-and-resume against the committed golden report, `ResumePrefix` rejection cases, and `CommitResultAtomically` atomicity).
- Three new self-contained fixtures under `tests/fixtures/linear/`: `remote-stale.json` (a clean create whose relevant remote state changed after the preview was produced), `remote-timeout-after-create.json` (a create whose remote outcome is discoverable via marker before any retry), and `remote-partial-apply.json` (a four-operation milestone->phase->plan->task plan with one rate-limited operation). Plus `tests/golden/linear-apply-report.json`, the canonical fully-resumed apply report.

## Task Commits

TDD gates committed atomically, one RED/GREEN pair per task:

1. **RED - Task 1: exact-plan apply integrity/freshness/discovery contract** - `13eb9b9` (test) -- `apply_test.go` plus the two Task 1 fixtures added; confirmed the package had no non-test Go files and failed to build without `model.go`/`engine.go`/`guard.go`.
2. **GREEN - Task 1: exact-plan apply engine with integrity/freshness/PR guards** - `fc9b1d8` (feat) -- `model.go`, `engine.go` (per-operation `Apply`, no journal dependency), and `guard.go` added; `go build`/`go vet`/`go test` all pass.
3. **RED - Task 2: partial-apply stop/report and journal resume contract** - `bc7108b` (test) -- `apply_test.go` extended with `TestScopeLinearApplyResume`, plus `remote-partial-apply.json` and the golden report; confirmed `engine.go`'s `Journal`/`ResumePrefix` references failed to build without `journal.go`.
4. **GREEN - Task 2: journal partial apply and resume the exact achieved prefix** - `61a7aac` (feat) -- `journal.go` added and `engine.go` extended with `RunApply`; `go build`/`go vet`/`go test -race` all pass, including the resumed report matching the golden byte-for-byte.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/trace/apply/model.go` - `RemoteState`, `RemoteClient`, `RetryableError`, `OperationStatus`/`OperationResult`, `Report`, `canonicalFields`/`fieldsMatch`.
- `internal/trace/apply/engine.go` - `classifyBlocked`, `applyLinkedOperation`/`applyUnlinkedOperation`/`applyOperation`/`applyOperations`, `achievedPrefix`, `Apply`, `RunApply`.
- `internal/trace/apply/guard.go` - `ValidatePlanIntegrity`, `ValidatePlanFreshness`, `GuardAgainstPullRequestMutation`, `ApplyRemoval`, `planBodyMirror`/`recomputePlanID`.
- `internal/trace/apply/journal.go` - `Journal`, `LoadJournal`, `ResumePrefix`, `stageTemp`, `CommitResultAtomically`.
- `internal/trace/apply/apply_test.go` - `TestScopeLinearApplyCore`, `TestScopeLinearApplyResume`, `fakeRemoteClient`, fixture loaders.
- `tests/fixtures/linear/remote-stale.json` - Stale-plan rejection scenario (CONTEXT D-18).
- `tests/fixtures/linear/remote-timeout-after-create.json` - Timeout-after-create discovery scenario (CONTEXT D-17/D-21).
- `tests/fixtures/linear/remote-partial-apply.json` - Four-operation rate-limited partial-apply/resume scenario (CONTEXT D-21).
- `tests/golden/linear-apply-report.json` - Canonical fully-resumed apply report golden.

## Decisions Made

See `key-decisions` in frontmatter for the full list. In short: two independent plan-level guards (integrity = self-hash, freshness = recompute-and-compare) run before any RemoteClient call; `RemoteClient` structurally cannot express removal (only `ApplyRemoval` can); a `RetryableError` (known-safe-to-retry) and marker-discovery-before-create (unknown outcome) are handled as distinct failure semantics; and `classifyBlocked` resolves dependency-on-a-conflicted-parent deterministically before any mutation attempt.

## Deviations from Plan

None — all nine frontmatter-declared files were created exactly as listed, and Task 1/Task 2 file ownership (including `engine.go` and `apply_test.go` appearing in both tasks' `<files>`) was honored by splitting `engine.go`'s lower-level `Apply` (Task 1) from its `RunApply` orchestration (Task 2, layered on top once `journal.go` exists), and by splitting `apply_test.go`'s `TestScopeLinearApplyCore` (Task 1) from `TestScopeLinearApplyResume` (Task 2, added once `RunApply`/`Journal` exist).

## Issues Encountered

- This worktree has no bootstrapped `.tools/` pinned toolchain, so the plan's literal `<verify>` commands (`powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-apply-core` / `linear-apply-resume`) could not be invoked through the shim without a multi-minute bootstrap first. Verified instead with the host Go toolchain, which is `go1.26.5 windows/amd64` — an exact match for the pinned `config/toolchain.toml` version — running `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, `go test -count=1 -race ./...`, and the exact scope-dispatch equivalent (`go test -list '^TestScopeLinearApplyCore$' ./...` / `'^TestScopeLinearApplyResume$'` confirming each marker resolves in `internal/trace/apply` the same way `test --quick --scope <name>` would find it), all passing. This mirrors the same substitution documented in the 01-10/01-21/01-23 summaries.
- The initial design coupled `engine.go`'s top-level `Apply`/`RunApply` entrypoint directly to `journal.go` (a single combined orchestration function), which would have made Task 1 fail to compile on its own without `journal.go` (a Task 2 file). Restructured before committing: Task 1 exports the lower-level `Apply(client, plan, mappings) []OperationResult` with no journal dependency; Task 2 appends `RunApply` (the full guard+resume+apply orchestration) to the same file. Both RED states were then verified by physically staging out the not-yet-committed implementation files and confirming the package failed to build, per the TDD execution flow.

## Known Stubs

None. `internal/trace/apply` is fully wired to real logic end to end (against a credential-free fake `RemoteClient`/`transport.Transport` -- no live GraphQL client exists yet, which is this plan's explicit scope boundary, not a stub). `RunApply` has no CLI route yet (`linear apply` is 01-24's explicit scope, matching 01-23's identical precedent for `linear preview`/`archive`/`unlink`); a future adapter implementing `RemoteClient` can be substituted without changing any call site in this package.

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-31 (plan binding tampering/repudiation) is mitigated by `ValidatePlanIntegrity`'s self-hash recomputation and `ValidatePlanFreshness`'s recompute-and-compare staleness check. T-01-32 (unknown/partial-outcome duplication or corruption) is mitigated by `applyUnlinkedOperation`'s discover-by-marker-before-create ordering, `RetryableError`'s stop-all-further-writes behavior, and `journal.go`'s exact-prefix `ResumePrefix`/atomic `CommitResultAtomically`. T-01-33 (PR-triggered elevation of privilege) is mitigated by `GuardAgainstPullRequestMutation`, applied identically inside `RunApply` and `ApplyRemoval`.

## User Setup Required

None - everything is repository-local and exercised only against a credential-free in-memory fake `RemoteClient`/`transport.Fake`; no credentials, npm, network, or Linear access involved.

## Next Phase Readiness

- Plan 01-24 (strict plan/report contracts and the `linear apply` CLI route, per `affects`) can now inject a `RemoteClientFactory` returning either a test `FakeClient` or (in a later plan) a real GraphQL-backed `RemoteClient`, and wire `apply.RunApply`/`apply.CommitResultAtomically` behind the self-registered route, exactly matching its own `<read_first>`'s reference to `internal/trace/apply/model.go`, `engine.go`, and `guard.go`.
- Plans 01-26/01-27 (real GraphQL partial-error/rate-limit normalization) can report through the already-proven `apply.RetryableError` contract without any change to `engine.go`'s stop/report/resume behavior.
- The `linear-apply-core`/`linear-apply-resume` quick-test scopes (`golc.ps1 test --quick --scope linear-apply-core` / `linear-apply-resume`, once `.tools/` is bootstrapped) now join `linear-preview-contract`/`linear-reconcile`/`linear-catalog`/`linear-map`/`config-local` as standing quick gates.

## Self-Check: PASSED

- All nine created files exist on disk: `internal/trace/apply/model.go`, `engine.go`, `guard.go`, `journal.go`, `apply_test.go`, and the four `tests/fixtures/linear/*.json`/`tests/golden/*.json` files.
- Commits `13eb9b9` (test), `fc9b1d8` (feat), `bc7108b` (test), and `61a7aac` (feat) exist in git history on branch `worktree-agent-a81d69ac48de9d670`.
- `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `go test -count=1 -race ./...` all pass from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly); `go test -list '^TestScopeLinearApplyCore$' ./...` and `'^TestScopeLinearApplyResume$'` both confirm their markers resolve in `internal/trace/apply`.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
