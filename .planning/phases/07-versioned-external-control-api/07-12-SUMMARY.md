---
phase: 07-versioned-external-control-api
plan: 12
subsystem: api
tags: [api, audit, batch, scope, observability]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-05 mutate.go's serialized mutation pipeline and observer.go's fireMutationObservers seam; 07-06 batch.go's atomic runBatch engine; 07-07 audit.go's redacting audit observer and show.QueryAuditLog; 07-09 the daemon-startup wiring that registers the audit observer via NewServer"
provides:
  - "Every early return in runBatch's pre-flight translate/scope-lookup/scope-rejection loop fires a failure MutationEvent, so a rejected batch sub-request writes exactly one audit_log row"
  - "mutate's own requiredScopeForRoute error branch fires the observer before its 500, closing the WR-03 trap"
  - "Five new tests in internal/api/batch_test.go pinning scope-rejection audit parity between POST /v1/pools and POST /v1/batch, translation-failure auditing, sub-request-order-preserving audit rows, and the zero-rows-for-an-empty-batch semantic"
affects: [07-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fire the post-mutation observer immediately before every early return on a mutating pipeline, even pre-lock/pre-copy rejection paths, so audit coverage is provably total rather than confined to the paths that happen to reach mutationMutex"

key-files:
  created: []
  modified:
    - internal/api/batch.go
    - internal/api/mutate.go
    - internal/api/batch_test.go

key-decisions:
  - "A rejected batch aborts at its first failing sub-request and writes exactly one audit row (not one per sub-request), preserving the existing all-or-nothing pre-flight control flow (D-15)."
  - "A translation-failure audit row records the client's own claimed method+resource (e.g. \"POST /v1/widgets\") as Route, since translation never resolved a real command route; real routes never contain a slash, so the two are always distinguishable."
  - "Pre-flight observer calls fire before mutationMutex is acquired, deliberately: nothing durable was ever at risk on the translate/scope-lookup/scope-rejection paths, so these audit rows carry no ordering guarantee relative to concurrently-committing mutations' rows -- only the guarantee that the row exists."

patterns-established:
  - "requireAuditRowCount/newAuditedBatchServer test helpers in batch_test.go: build a fresh *api.Server against its own t.TempDir() root (each NewServer call registers that root's own audit observer) and assert an exact QueryAuditLog row count, never \"at least one\"."

requirements-completed: [API-06]

coverage:
  - id: D1
    description: "A batch sub-request rejected for a missing domain scope writes exactly one audit_log row (failure, 403, actor, correlation id, null resulting_revision), at parity with the equivalent single POST /v1/pools rejection."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchScopeRejectionIsAudited"
        status: pass
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchAndSingleMutationScopeRejectionsAuditIdentically"
        status: pass
    human_judgment: false
  - id: D2
    description: "A batch sub-request whose method+resource has no registered translator writes exactly one audit row recording the claimed target."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchTranslationFailureIsAudited"
        status: pass
    human_judgment: false
  - id: D3
    description: "mutate's requiredScopeForRoute error branch fires the observer before its 500 (WR-03), removing the un-audited-500 trap."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/mutate_test.go#TestMutateObserverFires"
        status: pass
      - kind: other
        ref: "grep -c fireMutationObservers internal/api/mutate.go == 5"
        status: pass
    human_judgment: false
  - id: D4
    description: "A multi-sub-request batch's audit rows appear in the client's sub-request order, distinguishable by redacted_details."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchSubRequestAuditRowsFollowClientOrder"
        status: pass
    human_judgment: false
  - id: D5
    description: "An empty batch is rejected 400 and writes ZERO audit rows -- the decided semantic pinned by a test."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchEmptyWritesNoAuditRow"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-07-25
status: complete
---

# Phase 07 Plan 12: Batch pre-flight audit parity Summary

**Every early return in `runBatch`'s pre-flight translate/scope-lookup/scope-rejection loop, and `mutate`'s own scope-lookup-error branch, now fires the post-mutation observer -- closing 07-REVIEW.md WR-02/WR-03 so POST /v1/batch can no longer be used as an unaudited scope-probing surface.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2
- **Files modified:** 3 (internal/api/batch.go, internal/api/mutate.go, internal/api/batch_test.go)

## Accomplishments

- `runBatch`'s three pre-flight early returns (translation failure, scope-lookup failure, scope rejection) each fire a failure `MutationEvent` immediately before returning, mirroring `mutate.go`'s own scope-failure branch field for field.
- `mutate`'s `requiredScopeForRoute` error branch fires the observer before its 500, matching every other early return in the same function (WR-03).
- Five new tests in `internal/api/batch_test.go` pin: scope-rejection audit coverage, batch-versus-single scope-rejection parity, translation-failure audit coverage (with the claimed target recorded), sub-request audit-row ordering, and the zero-rows-for-an-empty-batch semantic.
- Confirmed the new tests are load-bearing: temporarily removed the scope-rejection `fireMutationObservers` call, observed `TestBatchScopeRejectionIsAudited` go RED (`expected exactly 1 audit_log row(s), got 0`), then restored the code to its committed state (verified via `git diff --stat` showing no diff).

## Task Commits

Each task was committed atomically:

1. **Task 1: Fire the audit observer on every batch pre-flight rejection and on mutate's scope-lookup error** - `2156d21` (fix)
2. **Task 2: Pin batch audit coverage, ordering, the empty-batch semantic, and single-versus-batch parity** - `4fa93f4` (test)

_Note: this plan's `tdd="true"` tasks did not follow a strict test-first RED/GREEN split per task -- Task 1 (production code) and Task 2 (tests) were planned and executed as separate, sequential tasks each with its own commit, matching the plan's own task boundaries. Task 2's RED-verification requirement (temporarily removing the Task 1 code path and observing the new test fail) was performed and is documented above._

## Files Created/Modified

- `internal/api/batch.go` - `runBatch`'s pre-flight loop fires a failure `MutationEvent` on each of its three early returns (translation failure, scope-lookup failure, scope rejection)
- `internal/api/mutate.go` - the `requiredScopeForRoute` error branch fires the observer before its 500; `mutate`'s doc comment updated to reflect the now-true "observers always fire exactly once" claim
- `internal/api/batch_test.go` - `newAuditedBatchServer`/`requireAuditRowCount` helpers plus `TestBatchScopeRejectionIsAudited`, `TestBatchAndSingleMutationScopeRejectionsAuditIdentically`, `TestBatchTranslationFailureIsAudited`, `TestBatchSubRequestAuditRowsFollowClientOrder`, `TestBatchEmptyWritesNoAuditRow`

## Decisions Made

- A rejected batch aborts at its first failing sub-request, writing exactly one audit row rather than one per sub-request -- preserves the existing all-or-nothing pre-flight control flow (D-15), matches the plan's own `[ASSUMED]` note.
- A translation-failure row records the client's claimed `<METHOD> <RESOURCE>` as `Route` with no `Args` (translation never produced routed args); real command routes never contain a slash, so the two remain distinguishable to a reader.
- Pre-flight observer calls fire outside `mutationMutex` deliberately, since nothing durable is at risk on those three paths -- documented in both the code comment and this summary per the plan's concurrency-edge `must_haves` truth.

## Deviations from Plan

None - plan executed exactly as written. One out-of-scope, pre-existing environment issue was discovered and logged rather than fixed (see below).

### Out-of-Scope Discovery (logged, not fixed)

`go test ./internal/command/...` fails in this worktree with `GOLC_TEST_TOOLCHAIN_MISSING: ... run 'mage Bootstrap' first` (five tests: `TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`). This worktree has never had `mage Bootstrap` run against it (`.tools/toolchains/go` and `.tools/installs/golc_project` do not exist), which is unrelated to this plan's `internal/api` file scope. Logged to `.planning/phases/07-versioned-external-control-api/deferred-items.md` per the Scope Boundary rule rather than auto-fixed. `mage testquick` was not run in this worktree for the same reason. All in-scope verification (`go test ./internal/api/... ./internal/show/... ./internal/artnet/... ./internal/artnet/ipc/...`) passed.

## Issues Encountered

None beyond the out-of-scope toolchain-bootstrap discovery above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Verification gap 3 (07-REVIEW.md WR-02) and WR-03 are closed: `POST /v1/batch` can no longer be used as a quieter, unaudited alternative to `POST /v1/pools` for scope-probing or endpoint-probing.
- `grep -c 'fireMutationObservers' internal/api/batch.go` == 4; the same grep on `internal/api/mutate.go` == 5, matching the plan's acceptance criteria exactly.
- The 07-VERIFICATION.md gap-3 code trace performed by hand is now a committed, permanently-running parity test (`TestBatchAndSingleMutationScopeRejectionsAuditIdentically`).
- Deferred: this worktree's `internal/command` toolchain-bootstrap gap should be resolved (via `mage Bootstrap`) before relying on its CI-shape tests in future work here.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED

- FOUND: internal/api/batch.go
- FOUND: internal/api/mutate.go
- FOUND: internal/api/batch_test.go
- FOUND: .planning/phases/07-versioned-external-control-api/07-12-SUMMARY.md
- FOUND: commit 2156d21 (Task 1)
- FOUND: commit 4fa93f4 (Task 2)
