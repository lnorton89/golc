---
phase: 07-versioned-external-control-api
plan: 06
subsystem: api
tags: [api, batch, atomicity, transactions, if-match, huma, chi]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-05: internal/api/mutate.go's mutationMutex + domainScope/requiredScopeForRoute/RequireScope, revision.go's parseIfMatch, translate.go's translateResult/statusFromHumaErr/buildMutationArgs, observer.go's MutationEvent/fireMutationObservers, show.CurrentRevision/show.NewTempCopy/show.Load/show.Save"
provides:
  - "internal/api/batch.go: POST /v1/batch -- genuinely atomic (all-or-nothing) multi-sub-request mutation via a throwaway VACUUM INTO copy + exactly ONE aggregated show.Save to the real show (D-15), batch-level If-Match checked at start and re-verified immediately before the commit, per-sub-request D-08 domain-scope enforcement, and per-index failure diagnostics"
  - "internal/api.BatchPreCommitHookForTesting: test-only seam for simulating a concurrent external write racing between a batch's start and its commit"
affects: [07-07-versioned-external-control-api, 07-08-versioned-external-control-api, 07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Atomic multi-mutation batch via 'copy-and-discard, single aggregated Save' (07-RESEARCH.md Pitfall 1's chosen strategy): each sub-request Executes against a throwaway show.NewTempCopy in client order (real internal/command handlers unmodified); on the first failure the copy is discarded and the real show is untouched; on full success the copy's final State is Loaded, its Revision is reset to the real show's own base revision (discarding the copy's own internal per-sub-request revision bookkeeping), and ONE show.Save commits it -- producing exactly one real revision bump regardless of how many sub-requests the batch carried."
    - "Batch-level If-Match is checked twice against the same captured base revision: once at the batch's start (before the copy is even made) and once again immediately before the final aggregated Save (after mutationMutex is already held) -- the second check is what catches a real external writer (e.g. a CLI process, which never goes through this package's in-process mutationMutex) racing between the batch's start and its commit."
    - "A batch's sub-requests are translated via a small per-resource batchTranslators map (method+path -> route+args builder) mirroring translate.go's HTTP->routed-command translation, rather than a generic HTTP-shape dispatcher -- seeded with only 'POST /v1/pools' today (07-05's own scope decision left only 'pool create' wired as a mutating REST operation); a future plan wiring another mutating route adds its own translator entry alongside it."
    - "Observers fire only for a successfully committed batch, one MutationEvent per committed sub-mutation, all sharing the batch's single real ResultingRevision -- never for a failed/rolled-back batch (unlike a single mutation's mutate.go pipeline, which fires a 'failure' event even on rejection); this is a deliberate difference documented in the plan's own Task 1 behavior requirement."

key-files:
  created:
    - internal/api/batch.go
    - internal/api/batch_test.go
  modified: []

key-decisions:
  - "Both plan tasks (the atomic engine and the empty/single/failure-report edge hardening) were implemented and committed together in one commit: the plan's own frontmatter lists the identical two files (internal/api/batch.go, internal/api/batch_test.go) for both tasks, and Task 2 hardens the exact same runBatch/batchSubRequestError functions Task 1 builds -- splitting them into two commits would have meant reverse-engineering an artificial diff boundary through code that was never meaningfully separable. All of both tasks' acceptance criteria are proven by this one commit's own test suite."
  - "finalState.Revision is explicitly reset to the real show's captured base revision before the single final show.Save call, discarding whatever revision the throwaway copy's own per-sub-request Save calls left it at (baseRevision + N for N successful sub-requests against the copy). This is the specific mechanism that makes 'copy-and-discard + single aggregated Save' actually produce D-15's required single revision bump (R -> R+1) regardless of batch size -- show.Save always does 's.Revision++' unconditionally on whatever Revision value it's handed, so without this reset a 3-sub-request batch would jump the real revision by 4 (3 internal copy bumps + 1 final bump), not 1. Not spelled out verbatim in PLAN.md's action text but directly implied by its own explicit 'R -> R+1 (one bump)' requirement; no other implementation reading satisfies that requirement."
  - "Added per-sub-request D-08 domain-scope enforcement (requiredScopeForRoute + RequireScope, reused directly from mutate.go since batch.go lives in the same internal/api package) even though the plan's action text does not explicitly mention it. Without this, a batch would be a scope-bypass vector: an authenticated key lacking the 'authoring' scope could not call POST /v1/pools directly (403), but could smuggle an identical 'pool create' mutation through a batch sub-request with no scope check at all. Flagged as a Rule 2 deviation below."
  - "Sub-request failure/scope/translation errors are reported via huma.NewError with a message naming the failing index, its diagnostic, and how many earlier sub-requests succeeded against the throwaway copy but were rolled back with it -- satisfying Task 2's 'successful-so-far sub-requests are reported as not-applied' requirement as an explicit count in the error text, rather than a parallel per-sub-request status array (Huma's typed-error contract does not have a natural per-item results shape for a whole-request failure)."

patterns-established:
  - "Batch sub-request translation seam (batchTranslators map) for any future plan wiring another mutating REST operation onto the single-mutation pipeline: add one map entry (\"METHOD /path\" -> translator func) rather than growing a generic HTTP-shape dispatcher."

requirements-completed: [API-04]

coverage:
  - id: D1
    description: "A fully-valid, ordered multi-sub-request batch applies every sub-request in client order against a throwaway copy, then commits the aggregated result to the real show with exactly one real revision bump (R -> R+1)"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchAtomic"
        status: pass
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchOrder"
        status: pass
    human_judgment: false
  - id: D2
    description: "A mid-batch sub-request failure leaves the real show and its revision completely unchanged, including any earlier sub-request's effect that had already succeeded against the throwaway copy"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchRollback"
        status: pass
    human_judgment: false
  - id: D3
    description: "A stale batch-level If-Match returns 412 and changes nothing; a real revision that changes underneath the batch between its start and its commit (simulated via a real external show.Save bypassing the in-process mutationMutex) also returns 412 and changes nothing beyond the external write itself"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchIfMatch"
        status: pass
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchIfMatchExternalRace"
        status: pass
    human_judgment: false
  - id: D4
    description: "An empty batch (no sub-requests) is rejected 400 GOLC_API_BATCH_EMPTY; a one-element batch produces the identical outcome and revision bump as the equivalent single mutation"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchEmpty"
        status: pass
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchSingle"
        status: pass
    human_judgment: false
  - id: D5
    description: "A batch failure response names the failing sub-request's index, its diagnostic, and how many earlier successful-so-far sub-requests were rolled back with it; no throwaway temp copy is ever left behind, on success or failure"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchFailureReport"
        status: pass
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchNoTempCopyLeftBehind"
        status: pass
    human_judgment: false
  - id: D6
    description: "A batch sub-request targeting a route the authenticated key lacks the required D-08 domain scope for is rejected 403 and mutates nothing (Rule 2 addition, not explicit in PLAN.md)"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/batch_test.go#TestBatchRequiresScope"
        status: pass
    human_judgment: false

# Metrics
duration: ~50min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 6: Atomic /v1/batch Summary

**POST /v1/batch applies an ordered list of sub-requests against a throwaway VACUUM INTO copy of the real show, then commits the aggregated result to the real show in exactly one atomic show.Save (real revision R -> R+1) -- or rolls back completely, leaving the real show untouched, with no internal/command handler modified.**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-07-25
- **Tasks:** 2/2
- **Files modified:** 2 (2 created)

## Accomplishments
- Built `internal/api/batch.go`'s `runBatch` engine: holds 07-05's `mutationMutex` for the whole batch, reads the real show's `CurrentRevision` as a captured base revision, checks a batch-level `If-Match` against it, copies the real `.golc` to a throwaway verified backup (`show.NewTempCopy`), Executes every sub-request in client order against that copy (on the first failure the copy is discarded via `defer cleanup()` and the real show is never touched), and -- only if every sub-request succeeded -- `Load`s the copy's final `State`, re-verifies the real show's revision hasn't changed underneath it (catching a real external writer racing between the batch's start and commit), resets the loaded `State.Revision` back to the captured base revision, and calls `show.Save` exactly once, producing precisely one real revision bump no matter how many sub-requests the batch carried.
- Added a small `batchTranslators` map (`"POST /v1/pools" -> translateBatchCreatePool`) mirroring `translate.go`'s HTTP-to-routed-command translation for a batch sub-request's own JSON body, matching the plan's "same translate.go path as single mutations" requirement without needing a generic HTTP-shape dispatcher -- only "pool create" exists as a mutating REST operation today (07-05-SUMMARY.md's own documented scope decision), so only one entry exists; a future plan wiring another route adds its own translator.
- Hardened batch edge semantics (Task 2): an empty sub-request list is rejected `400 GOLC_API_BATCH_EMPTY` before any state is touched; a one-element batch produces the identical revision bump as the equivalent single mutation (`TestBatchSingle` runs both against independent fresh shows and compares); a failing sub-request's error names its index, diagnostic, and exactly how many earlier sub-requests succeeded against the copy but were rolled back with it.
- Added per-sub-request D-08 domain-scope enforcement (`requiredScopeForRoute`/`RequireScope`, reused directly from `mutate.go` since both files live in the `internal/api` package) as a Rule 2 addition -- without it, a batch would let any authenticated key bypass the single-mutation pipeline's own scope gate for whichever routes its sub-requests target.
- Wired `POST /v1/batch` onto the router via the existing `RegisterOperation` self-registration seam (`Route: "batch apply"`, a synthetic name -- `coverage_test.go`'s `TestCapabilityCoverage` only requires every REAL `internal/command` route to be covered or excluded, never the reverse, so this needs no new exclusion entry and does not perturb that gate).
- Added `internal/api.BatchPreCommitHookForTesting`, a test-only seam called immediately before the final external-write race check, letting a test simulate a real external writer (e.g. a CLI process, which bypasses `mutationMutex` entirely since that mutex only ever serializes HTTP requests within this one Go process) racing between a batch's start and its commit.

## Task Commits

Both tasks were implemented and committed together in one commit -- see `key-decisions` in the frontmatter for the rationale (the plan's own frontmatter lists the identical two files for both tasks, and Task 2 hardens the exact same functions Task 1 builds).

1. **Task 1 + Task 2: Atomic batch engine + edge semantics (empty/single/failure-report)** - `ca53493` (feat)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md/REQUIREMENTS.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/api/batch.go` - `BatchPreCommitHookForTesting`, `batchSubRequest`/`batchTranslator`/`batchTranslators`, `createPoolBatchBody`/`translateBatchCreatePool`, `translateBatchSubRequest`, `batchSubRequestError`, `batchResultItem`/`batchOutput`, `batchInput`, `runBatch` engine, `registerBatch` + `RegisterOperation` self-registration
- `internal/api/batch_test.go` - `TestBatchAtomic`, `TestBatchOrder`, `TestBatchRollback`, `TestBatchIfMatch`, `TestBatchIfMatchExternalRace`, `TestBatchRequiresScope`, `TestBatchEmpty`, `TestBatchSingle`, `TestBatchFailureReport`, `TestBatchNoTempCopyLeftBehind` + shared helpers (`poolCreateBatchSubRequest`, `doBatchRequest`, `decodeBatchBody`, `assertNoTempCopyLeftBehind`)

## Decisions Made
- Both plan tasks committed together (identical files, tightly coupled functions) -- see `key-decisions`.
- `finalState.Revision` is reset to the real show's captured base revision before the single final `show.Save`, since `show.Save` unconditionally increments whatever revision it's handed -- this is the specific mechanism that turns "copy + single aggregated Save" into a genuine single real revision bump regardless of batch size.
- Per-sub-request D-08 scope enforcement added as a Rule 2 security fix, not explicit in `PLAN.md`'s action text.
- Sub-request failure reporting is a structured error message (index + diagnostic + rolled-back count) rather than a parallel per-item results array, since Huma's typed-error contract has no natural per-item shape for a whole-request failure.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added per-sub-request D-08 domain-scope enforcement**
- **Found during:** Task 1, while designing `runBatch`'s pre-flight checks
- **Issue:** The plan's action text describes translating each sub-request into a routed command + args and Executing it, but never mentions checking the D-08 coarse domain scope `mutate.go`'s own single-mutation pipeline (`requiredScopeForRoute`/`RequireScope`) already enforces for every equivalent single-mutation REST operation. Without this, `/v1/batch` would be a scope-bypass vector: a key holding only the `playback` scope (rejected 403 by `POST /v1/pools` directly) could smuggle an identical `pool create` mutation through a batch sub-request with zero scope check.
- **Fix:** Every sub-request is translated and its resolved route's required scope checked (via the exact same `requiredScopeForRoute`/`RequireScope` functions `mutate.go` uses, reusable directly since both files share the `internal/api` package) before `mutationMutex` is ever acquired or the real show is ever copied -- a scope failure touches nothing durable, matching mutate.go's own "a scope failure is reported without ever acquiring mutationMutex" doctrine.
- **Files modified:** `internal/api/batch.go`
- **Verification:** `TestBatchRequiresScope` (new) proves a batch sub-request targeting an authoring-scoped route is rejected 403 for a playback-only key, with the real revision unchanged.
- **Committed in:** `ca53493`

---

**Total deviations:** 1 auto-fixed (1 missing critical security check).
**Impact on plan:** Necessary to prevent batch from silently reopening the scope-gate bypass 07-05's mutation pipeline was specifically built to close. No scope creep beyond what D-08's own existing enforcement already requires.

## Issues Encountered
- `go build ./...` (whole-repo) fails on `cmd/golc-desktop/main.go:28: pattern all:frontend/dist: no matching files found` -- pre-existing, unrelated to this plan (the Wails frontend has not been built in this worktree), identical to 07-05-SUMMARY.md's own documented finding; `go build ./internal/...` is unaffected and green.
- `mage testquick` fails with `GOLC_TEST_TOOLCHAIN_MISSING` (`run 'mage Bootstrap' first`) -- this worktree has not run `mage Bootstrap`, identical to 07-04-SUMMARY.md and 07-05-SUMMARY.md's own documented environment limitation, unrelated to any file this plan touches. This plan's own stated verification command (`go test ./internal/api/... ./internal/show/... -run 'TestBatchAtomic|TestBatchRollback|TestBatchIfMatch|TestBatchOrder'` and the Task 2 equivalent) ran directly and is fully green, including the full `./internal/api/...` and `./internal/show/...` suites under `-race`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `/v1/batch` is now a stable, proven all-or-nothing endpoint: `runBatch`, `batchTranslators`, and `BatchPreCommitHookForTesting` are ready for later plans to extend (e.g. adding another mutating route's translator entry) without touching the atomicity engine itself.
- `fireMutationObservers` fires one `MutationEvent` per committed sub-mutation, all sharing the batch's single real `ResultingRevision`, only after a successful atomic commit -- 07-07 (audit) and 07-08 (SSE) can attach to this exactly as they do for single mutations, with no batch-specific plumbing needed on their end.
- API-04 is now functionally complete: revision/dry-run/idempotency/serialization (07-05) plus atomic `/v1/batch` (this plan, D-15's other half) are both proven.
- **Not yet closed:** only "pool create" has a batch translator (matching 07-05's own single wired mutating route); 07-09's coverage-closure task will need to add a `batchTranslators` entry for each additional mutating route it wires onto the single-mutation pipeline, alongside its own `registerXxx`/`RegisterOperation` work.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED
- FOUND: internal/api/batch.go
- FOUND: internal/api/batch_test.go
- FOUND: commit ca53493
