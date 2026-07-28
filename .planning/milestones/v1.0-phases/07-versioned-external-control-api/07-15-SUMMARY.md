---
phase: 07-versioned-external-control-api
plan: 15
subsystem: api
tags: [api, audit, batch, observability, precondition, wr-05]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-05 mutate.go's serialized mutation pipeline and observer.go's fireMutationObservers seam; 07-06 batch.go's atomic runBatch engine; 07-07 audit.go's redacting audit observer and show.QueryAuditLog; 07-12 runBatch's pre-flight loop audit parity (the sibling half of this same defect)"
provides:
  - "Every one of the nine failure returns inside runBatch's locked section (mutationMutex.Lock() through the aggregated show.Save) fires a failure MutationEvent before returning, closing 07-REVIEW-gaps.md WR-05 / 07-VERIFICATION.md's sole remaining gap"
  - "A stale batch-level If-Match writes one failure audit row per sub-request, at parity with the equivalent single POST /v1/pools rejection"
  - "A malformed If-Match writes one failure row with a NULL expected_revision, distinguishing a rejected header from a mismatched one"
  - "The pre-commit external-write race and a mid-batch sub-request execution failure both leave full audit evidence, with the row-count rule (batch-level = one row per sub-request; sub-request-level = one row for the culpable index) decided and pinned by tests"
  - "A structural source-reading test (TestBatchLockedSectionFailureReturnsAreAllAudited) mechanically proves all nine locked-section failure returns fire an audit observer, including the five reachable only via fault injection, and pins the true branch count (nine, not the eight both source findings transcribed)"
affects: [07-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A batch-level failure (not attributable to any single sub-request) fans out one failure audit row per sub-request via a local closure (fireBatchFailureObservers); a sub-request-level failure (translateResult erroring against the throwaway copy) fires a single direct fireMutationObservers call for the culpable index alone -- the same batch-level-vs-sub-request-level row-count split 07-12 established for the pre-flight loop, now extended to the locked section"
    - "A source-structure test (os.ReadFile + region markers + prefix scan) is the deliberate substitute for behavioral audit coverage of failure branches reachable only via fault injection, mirroring deprecation_test.go's TestBuildRouterInstallsDeprecationMiddleware precedent"

key-files:
  created: []
  modified:
    - internal/api/batch.go
    - internal/api/batch_test.go

key-decisions:
  - "expectedRevisionPtr is hoisted once, before mutationMutex.Lock(), and read at CALL time by every locked-section fire (failure and success): the two call sites preceding If-Match parsing record a NULL expected revision, the seven that follow it record the client's claimed revision -- deleting the success path's own duplicate declaration is a pure dedup with no behavior change."
  - "fireBatchFailureObservers is defined before mutationMutex.Lock() (so the locked region itself contains only call sites, per the plan's own structural constraint), but every one of its invocations happens with the mutex held -- unlike 07-12's pre-flight rows, these rows ARE strictly ordered against concurrently-committing mutations' rows."
  - "The pre-commit show.CurrentRevision re-read failure (raceErr, a 500) is a distinct ninth branch from the adjacent external-write-race 412 (raceRevision mismatch) -- both source findings (07-REVIEW-gaps.md and 07-VERIFICATION.md) had collapsed them into one cited line range; this plan fires and tests them as two separate branches, matching the true source-region count."

patterns-established:
  - "TestBatchLockedSectionFailureReturnsAreAllAudited's region-marker scan skips comment lines (trimmed strings.HasPrefix '//') both when locating the mutationMutex.Lock()/resultingRevision start/end markers themselves and when scanning inside the region -- a doc comment that happens to mention 'mutationMutex.Lock()' in prose (this plan's own updated runBatch doc comment does) must never be mistaken for the real statement."

requirements-completed: [API-06]

coverage:
  - id: D1
    description: "A stale batch-level If-Match writes one failure audit row per sub-request (status 412, expected_revision = the client's claimed value, NULL resulting_revision), and the same rejection through POST /v1/pools and POST /v1/batch leaves parity evidence."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchStaleIfMatchIsAudited"
        status: pass
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchAndSingleMutationStaleIfMatchAuditIdentically"
        status: pass
    human_judgment: false
  - id: D2
    description: "A malformed (unparseable) If-Match writes one failure row with status 400 and a NULL expected_revision."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchMalformedIfMatchIsAudited"
        status: pass
    human_judgment: false
  - id: D3
    description: "The pre-commit external-write race writes one failure row per sub-request, even though every sub-request had already succeeded against the throwaway copy."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchExternalWriteRaceIsAudited"
        status: pass
    human_judgment: false
  - id: D4
    description: "A mid-batch sub-request execution failure writes exactly one failure row, for the culpable index only, whose status equals the HTTP status the client received."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchSubRequestExecutionFailureIsAudited"
        status: pass
    human_judgment: false
  - id: D5
    description: "All nine locked-section failure returns (including the five reachable only via fault injection) fire an audit observer before returning; the region is pinned at exactly nine returns and nine fires."
    requirement: "API-06"
    verification:
      - kind: unit
        ref: "internal/api/batch_test.go#TestBatchLockedSectionFailureReturnsAreAllAudited"
        status: pass
      - kind: other
        ref: "grep -v '^[[:space:]]*//' internal/api/batch.go | grep -c 'fireMutationObservers(' == 6; same filtered grep for 'fireBatchFailureObservers(' == 8"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-07-25
status: complete
---

# Phase 07 Plan 15: Locked-section batch audit gap closure Summary

**All nine of `runBatch`'s locked-section failure returns (from `mutationMutex.Lock()` through the aggregated `show.Save`) now fire the audit observer before returning, closing 07-VERIFICATION.md's sole remaining gap (07-REVIEW-gaps.md WR-05) and correcting a miscount both source findings shared -- the region has nine unaudited branches, not eight.**

## Performance

- **Duration:** ~55 min
- **Tasks:** 2
- **Files modified:** 2 (internal/api/batch.go, internal/api/batch_test.go)

## Accomplishments

- Hoisted `expectedRevisionPtr` above `mutationMutex.Lock()` and added a local `fireBatchFailureObservers` closure, fanning out one failure row per sub-request for every BATCH-LEVEL failure (the initial `show.CurrentRevision` read, `parseIfMatch`, the If-Match mismatch, `show.NewTempCopy`, `show.Load`, the pre-commit `show.CurrentRevision` re-read, the pre-commit external-write race, and `show.Save`).
- Fired a single direct `fireMutationObservers` call, for the culpable index alone, on the one SUB-REQUEST-LEVEL failure (`translateResult` erroring against the throwaway copy) -- preserving 07-12's established "one row for the failing sub-request" semantic.
- Removed the success path's now-duplicated `expectedRevisionPtr` local declaration so both success and failure rows read the single hoisted variable.
- Confirmed by direct count (not the source findings' transcription) that the locked region has nine `return nil,` statements, not eight: the pre-commit `show.CurrentRevision` re-read failure (`raceErr`, a 500) is a distinct branch from the adjacent external-write-race 412, four lines apart in the source both findings cited as one range.
- Added five behavioral tests for the four reachable failure paths (stale If-Match, malformed If-Match, external-write race, mid-batch execution failure) plus one batch-versus-single parity test, all asserting exact audit-row counts and per-field content -- never "at least one".
- Added `TestBatchLockedSectionFailureReturnsAreAllAudited`, a source-structure test that reads `batch.go`'s own source, locates the locked region by its `mutationMutex.Lock()`/`resultingRevision := baseRevision + 1` markers, and mechanically proves every `return nil,` in that region is preceded by an audit fire -- the deliberate substitute for behavioral coverage of the five branches reachable only via fault injection (both `show.CurrentRevision` calls, `show.NewTempCopy`, `show.Load`, `show.Save`), and the gate that pins the true nine-branch count.
- Confirmed the new tests are load-bearing, not tautological: temporarily removed the If-Match-mismatch branch's `fireBatchFailureObservers` call, observed both `TestBatchStaleIfMatchIsAudited` (row-count assertion) and `TestBatchLockedSectionFailureReturnsAreAllAudited` (unaudited-return message, naming the exact source line) go RED, then restored `batch.go` and confirmed `git diff --stat internal/api/batch.go` was empty.
- `mage Bootstrap` was run successfully in this worktree (unlike 07-12's worktree, which had to defer it), so the full broader test set and `mage testquick` both ran and passed green in-plan rather than being logged as deferred.

## Task Commits

Each task was committed atomically:

1. **Task 1: Fire the audit observer on all nine of runBatch's locked-section failure returns** - `3706075` (feat)
2. **Task 2: Pin locked-section audit coverage behaviorally for the four reachable paths and structurally for all nine** - `865af7f` (test)

## Files Created/Modified

- `internal/api/batch.go` - hoisted `expectedRevisionPtr`, added the `fireBatchFailureObservers` closure, fired it before eight of the nine locked-section failure returns, fired a direct `fireMutationObservers` call before the ninth (the sub-request-attributable `translateErr` return), deleted the success path's duplicate pointer declaration, and updated `runBatch`'s doc comment to record the fire discipline and the row-count rule
- `internal/api/batch_test.go` - added `TestBatchStaleIfMatchIsAudited`, `TestBatchAndSingleMutationStaleIfMatchAuditIdentically`, `TestBatchMalformedIfMatchIsAudited`, `TestBatchExternalWriteRaceIsAudited`, `TestBatchSubRequestExecutionFailureIsAudited`, and `TestBatchLockedSectionFailureReturnsAreAllAudited`; added the `os` import for the structural test's `os.ReadFile("batch.go")` call

## Decisions Made

- A BATCH-LEVEL failure (any of the eight `fireBatchFailureObservers` call sites) writes one failure row per sub-request -- no single index is more culpable than another, and the whole batch was rejected by one cause. A SUB-REQUEST-LEVEL failure (the one `translateResult` error site) writes exactly one row, for the culpable index. This is the `[ASSUMED]` rule the plan carried forward from 07-REVIEW-gaps.md WR-05's suggested fan-out shape, compatible with 07-12's own exactly-one-row-per-rejected-batch decision for pre-flight failures.
- The pre-commit `show.CurrentRevision` re-read failure (`raceErr`) is fired and tested as its own distinct branch, separate from the adjacent external-write-race 412 -- correcting the undercount both 07-REVIEW-gaps.md and 07-VERIFICATION.md shared.
- No response status code, response body, error message, diagnostic code, revision outcome, rollback behavior, or temp-copy cleanup changed on any of the nine paths -- verified by every pre-existing batch test (including `TestBatchIfMatch` and `TestBatchIfMatchExternalRace`) passing unmodified, and by `TestOpenAPIDrift` staying green (no OpenAPI-visible change).

## Deviations from Plan

None - plan executed exactly as written.

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a false-positive region-marker match in TestBatchLockedSectionFailureReturnsAreAllAudited's own first draft**
- **Found during:** Task 2, first test run
- **Issue:** The structural test's region-start search (`strings.Contains(line, "mutationMutex.Lock()")`) matched `runBatch`'s own doc comment (this plan's Task 1 update to that comment references "mutationMutex.Lock() through the aggregated show.Save" in prose), so the scanned region started at the top of the function instead of the real locked section -- causing a false failure on the unrelated `GOLC_API_BATCH_EMPTY` early return.
- **Fix:** Added a comment-skip check (`strings.HasPrefix(strings.TrimSpace(line), "//")`) to the region-marker search itself, not just the in-region scan, so a doc comment can never be mistaken for the real statement that opens the locked region.
- **Files modified:** internal/api/batch_test.go
- **Commit:** 865af7f (folded into Task 2's single commit; the test never reached a committed broken state)

## Issues Encountered

None. `mage Bootstrap` (needed for the broader `internal/command` test set and `mage testquick`) had not yet been run in this worktree; running it succeeded without incident, so no toolchain gap needed to be logged to deferred-items.md for this plan.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 07-VERIFICATION.md's sole remaining gap is closed: Success Criterion #4's "every result is auditable" now holds for `runBatch`'s locked section, at parity with `mutate.go`'s single-mutation pipeline and with `runBatch`'s own pre-flight loop (07-12). API-06 is no longer BLOCKED.
- `grep -v '^[[:space:]]*//' internal/api/batch.go | grep -c 'fireMutationObservers('` == 6; the same filtered grep for `'fireBatchFailureObservers('` == 8; `grep -c 'fireBatchFailureObservers := func(statusCode int)'` == 1; `grep -c 'expectedRevisionPtr \*int64'` == 1 -- all matching the plan's acceptance criteria exactly.
- The WR-05 code trace 07-REVIEW-gaps.md and 07-VERIFICATION.md each performed by hand is now a committed, permanently-running structural test (`TestBatchLockedSectionFailureReturnsAreAllAudited`) that pins the true branch count (nine) and cannot be satisfied by a partial fix.
- `docs/api/openapi.json` is byte-unchanged (`TestOpenAPIDrift` green): this was confirmed as a pure audit-completeness change, not a contract change.
- No further known gaps remain open against 07-VERIFICATION.md as of this plan.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED

- FOUND: internal/api/batch.go
- FOUND: internal/api/batch_test.go
- FOUND: .planning/phases/07-versioned-external-control-api/07-15-SUMMARY.md
- FOUND: commit 3706075 (Task 1)
- FOUND: commit 865af7f (Task 2)
