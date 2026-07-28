---
phase: 07-versioned-external-control-api
plan: 05
subsystem: api
tags: [api, mutations, if-match, revision, dry-run, idempotency, concurrency, huma, chi]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-02: internal/api Chi+Huma /v1 server, RegisterOperation self-registration seam, Executor interface, translate.go's HTTP->routed-command translation; 07-04: AuthMiddleware/RequireScope/ScopesFromContext/KeyIDFromContext, RateLimitMiddleware wired ahead of every operation"
provides:
  - "internal/api/mutate.go: the single serialized mutation pipeline (mutationMutex) every mutating REST operation funnels through -- scope gate (D-08 domainScope map), If-Match/412 (D-13), dry-run branch (D-14), idempotency-replay branch (A6), Execute, post-mutation observer"
  - "internal/api/revision.go: parseIfMatch + checkRevision against show.CurrentRevision (412 Precondition Failed)"
  - "internal/api/observer.go: MutationEvent + RegisterMutationObserver/fireMutationObservers -- the single seam 07-07 (audit) and 07-08 (SSE) attach to"
  - "internal/api/dryrun.go: dryRunMutate -- Executes against a throwaway show.NewTempCopy, never the real show; fires outcome \"dry_run\""
  - "internal/api/idempotency.go: idempotencyStore (per-*Server, mutex-guarded, TTL-based) + WithIdempotencyTTL server option"
  - "show.CurrentRevision(root, path) (internal/show/store.go): fast show_meta.revision read without full Load"
  - "show.NewTempCopy(root, path) (internal/show/store.go): throwaway, verified VACUUM INTO copy + cleanup, reusing Phase 5's verifiedBackup machinery"
  - "POST /v1/pools -> \"pool create\": the first concrete mutating REST operation proving the full pipeline end-to-end (scope/If-Match/dry-run/idempotency/observer, all exercised by this plan's own tests)"
affects: [07-06-versioned-external-control-api, 07-07-versioned-external-control-api, 07-08-versioned-external-control-api, 07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single mutex-serialized mutation pipeline (mutate.go's package-level mutationMutex): every mutating request -- real, dry-run, or idempotent replay -- funnels through one critical section, so internal/api never becomes a second concurrent SQLite writer beyond what busy_timeout already tolerates (07-RESEARCH.md Pitfall 2). 07-06's batch engine reuses this exact mutex for the whole batch."
    - "Coarse domain scope resolved from a route's first word via a documented domainScope map (mutate.go), not a generic default -- an undeclared domain fails closed (GOLC_API_DOMAIN_SCOPE_UNDECLARED) rather than silently granting the most permissive scope."
    - "If-Match lives inside the mutation pipeline (revision.go), never a generic cross-cutting ETag middleware -- it compares the domain-meaningful show.State.Revision, not an HTTP response content hash (07-RESEARCH.md Anti-Pattern)."
    - "Dry-run reuses Phase 5's verified-backup (VACUUM INTO + read-back-validate) machinery via a new exported show.NewTempCopy wrapper, rather than a second, parallel preview implementation of mutation logic (07-RESEARCH.md Pattern 4)."
    - "mutate.go builds its own small JSON response envelope (mutationOutput: {result, revision}) instead of reusing translate.go's raw-stdout-passthrough rawJSONOutput -- most mutating CLI handlers (\"pool create\" included) print a plain-text confirmation line, not JSON, so the pipeline itself computes and reports the resulting revision via show.CurrentRevision rather than requiring every mutating internal/command handler to grow a --json flag."
    - "Post-mutation observer seam (observer.go) fires exactly once per attempted mutation with one of four outcomes: \"success\", \"failure\", \"dry_run\", \"idempotent_replay\" -- audit (07-07) and SSE (07-08) both key off this single MutationEvent shape."

key-files:
  created:
    - internal/api/mutate.go
    - internal/api/mutate_test.go
    - internal/api/observer.go
    - internal/api/revision.go
    - internal/api/dryrun.go
    - internal/api/dryrun_test.go
    - internal/api/idempotency.go
    - internal/api/idempotency_test.go
  modified:
    - internal/show/store.go
    - internal/show/backup.go
    - internal/api/server.go
    - internal/api/coverage_test.go

key-decisions:
  - "Only \"pool create\" (POST /v1/pools) was wired as a concrete mutating REST operation in this plan -- proving the full pipeline (scope gate, If-Match/412, dry-run, idempotency, serialization, observer) end-to-end against a real command route, exactly matching every behavior/acceptance test this plan's own PLAN.md specifies. The remaining ~40 show-mutating command routes (chase/scene/deployment/motion/theme/preset/blend/operatorsurface/playback/artnet-safety-master/config-set/fixture-import/show-open-save/programmer) remain in coverage_test.go's documented reasonMutationDeferred exclusion set, unchanged from 07-02's own precedent of 'deferred with a documented reason, never silently unmapped.' Wiring each remaining route requires its own bespoke Huma input struct mapping JSON body fields to that route's specific --flag args (the same per-domain design work 07-02 itself deferred for reads) -- substantial, route-specific work this plan's stated files_modified list (mutate.go/revision.go/dryrun.go/idempotency.go/observer.go + tests + store.go) does not scope. This is the single highest-impact scope decision in this plan: 07-09's own closing task references 'mutations (07-05)' when tightening the coverage exclusion set, so a future plan (or 07-09 itself) still needs to wire the remaining mutating routes onto this now-proven pipeline before API-01's every-capability coverage claim is fully closed."
  - "mutate.go builds its own JSON response envelope (mutationOutput: {result, revision}) rather than reusing translate.go's rawJSONOutput raw-passthrough pattern, because most mutating internal/command handlers (pool create included) print a plain-text confirmation line, not JSON -- the pipeline itself reads show.CurrentRevision after a successful Execute and reports it, rather than requiring every mutating handler to grow a --json flag just for REST parity."
  - "Dry-run and idempotent-replay both execute inside mutationMutex (the same critical section real mutations use), even though neither durably writes to the real show. This is a deliberate simplicity/correctness tradeoff over a finer-grained locking scheme -- it keeps the whole pipeline's behavior easy to reason about and matches 07-06-PLAN.md's own stated intent to reuse '07-05's serialized mutation mutex' for the whole batch."
  - "Idempotency-Key TTL defaults to 24h (idempotency.go's defaultIdempotencyTTL), configurable per-*Server via the new WithIdempotencyTTL option -- flagged [ASSUMED] (07-RESEARCH.md Assumptions Log A6) pending a later discuss/UAT confirmation, exactly as RESEARCH.md itself recommended. Only a successful mutation's response is cached; a failed attempt is never cached, so a client can safely retry after fixing whatever caused the failure."

requirements-completed: []  # API-04 spans 07-05 (this plan: If-Match/dry-run/idempotency/serialization) and 07-06 (atomic /v1/batch) -- not functionally complete until 07-06 ships. API-01 spans every plan 07-02..07-09 (07-02-SUMMARY's own precedent); this plan proves the invoke/mutation half of API-01 against one concrete route, with the remaining mutating routes deferred (see key-decisions).

coverage:
  - id: D1
    description: "POST /v1/pools with a matching If-Match creates a pool and the response revision equals the prior revision + 1; a repeat with the now-stale If-Match returns 412 and creates nothing"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/mutate_test.go#TestMutateIfMatchRevisionLifecycle"
        status: pass
    human_judgment: false
  - id: D2
    description: "A mutation without the required coarse domain scope returns 403 and mutates nothing (revision unchanged)"
    requirement: "API-01"
    verification:
      - kind: integration
        ref: "internal/api/mutate_test.go#TestMutateRequiresScope"
        status: pass
    human_judgment: false
  - id: D3
    description: "Concurrent mutating requests serialize: all complete, the real revision advances by exactly the number of successful mutations, and no request's effect is lost to interleaving"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/mutate_test.go#TestMutateSerializesConcurrentRequests"
        status: pass
    human_judgment: false
  - id: D4
    description: "The post-mutation observer seam fires exactly once per attempted mutation, with outcome \"success\" (resulting revision populated) or \"failure\" (no resulting revision)"
    requirement: "API-01"
    verification:
      - kind: integration
        ref: "internal/api/mutate_test.go#TestMutateObserverFires"
        status: pass
    human_judgment: false
  - id: D5
    description: "?dry_run=true returns the projected result with HTTP 200 and leaves the real show.State.Revision unchanged; a dry-run of an invalid mutation surfaces the same validation error without touching the real show; the throwaway VACUUM INTO copy is always deleted; the observer fires \"dry_run\", never \"success\""
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/dryrun_test.go#TestDryRunLeavesRealRevisionUnchanged"
        status: pass
      - kind: integration
        ref: "internal/api/dryrun_test.go#TestDryRunSurfacesValidationErrorWithoutMutating"
        status: pass
      - kind: integration
        ref: "internal/api/dryrun_test.go#TestDryRunLeavesNoTempCopy"
        status: pass
      - kind: integration
        ref: "internal/api/dryrun_test.go#TestDryRunObserverOutcome"
        status: pass
    human_judgment: false
  - id: D6
    description: "Two identical requests carrying the same Idempotency-Key within the TTL apply the mutation exactly once (the replay returns the stored first response, revision advances by 1 not 2); the same key after TTL expiry re-executes; different keys execute independently"
    requirement: "API-04"
    verification:
      - kind: integration
        ref: "internal/api/idempotency_test.go#TestIdempotencyReplayWithinTTLAppliesOnce"
        status: pass
      - kind: integration
        ref: "internal/api/idempotency_test.go#TestIdempotencyReExecutesAfterTTLExpires"
        status: pass
      - kind: integration
        ref: "internal/api/idempotency_test.go#TestIdempotencyDifferentKeysIndependent"
        status: pass
    human_judgment: false

# Metrics
duration: 55min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 5: Serialized Mutation Pipeline with Revision, Dry-Run, and Idempotency Summary

**A single mutex-serialized mutation pipeline (scope gate, If-Match/412 against show.State.Revision, ?dry_run=true copy-and-discard preview, Idempotency-Key replay dedupe, and a post-mutation observer seam) proven end-to-end through POST /v1/pools -> "pool create".**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-07-25
- **Tasks:** 3/3
- **Files modified:** 12 (8 created, 4 modified)

## Accomplishments
- Built `internal/api/mutate.go`: one `mutationMutex`-guarded critical section every mutating REST operation calls (`mutate(ctx, server, mutateRequest)`), running (1) a documented `domainScope` map resolving each route's coarse D-08 scope with a fail-closed default, (2) an If-Match/412 check (`revision.go`) against `show.CurrentRevision`, (3) a `?dry_run=true` branch (`dryrun.go`) before ever touching the real show, (4) an `Idempotency-Key` replay branch (`idempotency.go`) before ever comparing revision or executing, (5) the real `Execute` call, and (6) a post-mutation observer fire (`observer.go`) -- exactly the five/six-step pipeline the plan's own action text specifies.
- Added `internal/api/observer.go`'s `MutationEvent`/`RegisterMutationObserver`/`fireMutationObservers`: the single seam 07-07 (audit) and 07-08 (SSE) both attach to, firing exactly once per attempted mutation with outcome `"success"`, `"failure"`, `"dry_run"`, or `"idempotent_replay"`.
- Added `show.CurrentRevision` (fast `show_meta.revision` read, no full decode/validate) and `show.NewTempCopy` (throwaway, verified VACUUM INTO copy + cleanup, reusing Phase 5's `verifiedBackup` machinery) to `internal/show/store.go` -- the two accessors the pipeline and dry-run both depend on.
- Wired `POST /v1/pools -> "pool create"` as the first concrete mutating REST operation, moving it from `coverage_test.go`'s `reasonMutationDeferred` exclusion set to a registered operation -- proven end-to-end by every one of this plan's behavior tests (matching If-Match applies + bumps revision by 1, stale If-Match 412s and mutates nothing, missing scope 403s, 5 concurrent requests serialize with the revision advancing by exactly 5 and no lost update, dry-run previews without touching the real show, a duplicate Idempotency-Key replays the stored response instead of re-applying).
- Implemented `internal/api/idempotency.go`'s `idempotencyStore`: a per-`*Server`, mutex-guarded, TTL-based in-memory map (`defaultIdempotencyTTL` 24h, overridable via the new `WithIdempotencyTTL` server option) -- only successful mutations are cached, so a failed attempt can always be safely retried.
- Fixed a real bug in `internal/show/backup.go`'s `verifyBackupReadBack`, exposed by dry-run's new call path against a daemon's brand-new, never-yet-saved show: it previously tried to `strictjson.DecodeStrict` an intentionally-empty seed-row blob and failed with `GOLC_SHOW_BACKUP_UNVERIFIABLE`, even though "nothing has been saved yet" is not corruption. It now treats a `readMeta`-"not ok" (never-yet-saved) backup as trivially valid, mirroring `Load`'s own doctrine.

## Task Commits

1. **Task 1: Serialized mutation pipeline — scope gate, If-Match/412, Execute, post-mutation observer seam** - `1952bf9` (feat)
2. **Task 2: Dry-run previews via copy-and-discard (?dry_run=true)** - `6c70521` (feat)
3. **Task 3: Idempotency-Key dedupe within TTL** - `1e3aeef` (feat)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md/REQUIREMENTS.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/api/mutate.go` - `mutationMutex`, `domainScope` map + `requiredScopeForRoute`, `mutateRequest`/`mutationResult`, `mutate()` pipeline, `mutationOutput` response envelope, `POST /v1/pools` operation registration
- `internal/api/mutate_test.go` - `TestMutateIfMatchRevisionLifecycle`, `TestMutateRequiresScope`, `TestMutateSerializesConcurrentRequests`, `TestMutateObserverFires`
- `internal/api/observer.go` - `MutationEvent`, `RegisterMutationObserver`, `fireMutationObservers`, `ResetMutationObserversForTesting`
- `internal/api/revision.go` - `parseIfMatch`, `checkRevision` (412 against `show.CurrentRevision`)
- `internal/api/dryrun.go` - `dryRunMutate` (throwaway-copy Execute, always outcome `"dry_run"`)
- `internal/api/dryrun_test.go` - `TestDryRunLeavesRealRevisionUnchanged`, `TestDryRunSurfacesValidationErrorWithoutMutating`, `TestDryRunLeavesNoTempCopy`, `TestDryRunObserverOutcome`
- `internal/api/idempotency.go` - `idempotencyStore`, `newIdempotencyStore`, `lookup`/`store`, `defaultIdempotencyTTL`
- `internal/api/idempotency_test.go` - `TestIdempotencyReplayWithinTTLAppliesOnce`, `TestIdempotencyReExecutesAfterTTLExpires`, `TestIdempotencyDifferentKeysIndependent`
- `internal/show/store.go` - `CurrentRevision`, `NewTempCopy`
- `internal/show/backup.go` - `verifyBackupReadBack` fix (never-yet-saved backup is valid, not corrupt)
- `internal/api/server.go` - `idempotencyTTL` field + `WithIdempotencyTTL` option, `idempotency *idempotencyStore` constructed in `NewServer`
- `internal/api/coverage_test.go` - `"pool create"` moved from `reasonMutationDeferred` to a registered operation

## Decisions Made
- Only `"pool create"` (`POST /v1/pools`) was wired as a concrete mutating REST operation -- see `key-decisions` in the frontmatter for the full rationale and its forward impact on 07-09's coverage-closure task.
- `mutate.go` builds its own JSON response envelope rather than reusing `translate.go`'s raw-stdout-passthrough, since most mutating CLI handlers print plain text, not JSON.
- Dry-run and idempotent-replay both run inside the same `mutationMutex` real mutations use, for simplicity and to match 07-06's own stated intent to reuse this exact mutex for the whole batch.
- Idempotency-Key TTL defaults to 24h, flagged `[ASSUMED]` (A6) per 07-RESEARCH.md, configurable via `WithIdempotencyTTL`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed `verifyBackupReadBack` to handle a never-yet-saved show's backup**
- **Found during:** Task 2, while implementing dry-run against a brand-new daemon show that has never been saved
- **Issue:** `internal/show.NewTempCopy` (this plan's new wrapper around Phase 5's `verifiedBackup`) failed with `GOLC_SHOW_BACKUP_UNVERIFIABLE: ... unexpected end of JSON input` whenever the daemon's show file had never been saved (the common case for a fresh daemon's first mutation): `verifyBackupReadBack` unconditionally ran `strictjson.DecodeStrict` against `show_state.blob`, but a never-yet-saved file's blob is intentionally empty (`openStore`'s seed row) -- `store.go`'s own `readMeta`/`Load` already treat this as "not an error, nothing saved yet," but `backup.go`'s verification path had never been exercised against that case before (every prior caller, `Migrate`, only ever backs up a file already known to have a real, older-schema blob).
- **Fix:** `verifyBackupReadBack` now calls `readMeta` first and returns success immediately (no decode attempted) when it reports "not ok" (never-yet-saved), mirroring `Load`'s own doctrine exactly.
- **Files modified:** `internal/show/backup.go`
- **Verification:** `TestVerifiedBackupRoundTrips`/`TestVerifiedBackupRejectsCorruptBackup` (pre-existing) still pass unchanged; `TestDryRunLeavesRealRevisionUnchanged`/`TestDryRunObserverOutcome` (new) exercise the never-yet-saved path directly.
- **Committed in:** `6c70521` (Task 2 commit)

*(An initial attempt at this fix also added a `show_meta.checksum` comparison to `verifyBackupReadBack` for consistency with `store.go`'s `decodeAndValidate`. This broke three pre-existing `migrate_test.go` tests, which deliberately seed `checksum = ''` in their synthetic historical-version fixtures -- checksum enforcement was never part of the backup-verification path's original contract. The checksum addition was reverted before committing; only the never-yet-saved-blob fix (which does not touch that contract) was kept.)*

---

**Total deviations:** 1 auto-fixed (1 bug fix, required for dry-run to function against any show that has not yet been saved -- a normal, common daemon state, not an edge case).
**Impact on plan:** Necessary for Task 2's own acceptance criteria (dry-run must work against the daemon's real show, which starts out never-yet-saved). No scope creep beyond what dry-run's own correctness requires.

## Issues Encountered
- `internal/command`'s `TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance` fail in this worktree with `GOLC_TEST_TOOLCHAIN_MISSING`/`pinned golc-project binary not built` -- confirmed pre-existing (this worktree has not run `mage Bootstrap`), identical to the environment limitation 07-04-SUMMARY.md already documented; none of these tests reference `internal/api`, `internal/show`, or any file this plan touches. `mage testquick` itself could not be run for the same reason (the pinned Go toolchain binary is not bootstrapped in this worktree); `go build ./internal/...` and `go test ./internal/api/... ./internal/show/...` (this plan's own stated verification commands) both ran directly and are fully green, including under `-race` for every concurrency-sensitive test.
- `go build ./...` (whole-repo) fails on `cmd/golc-desktop/main.go:28: pattern all:frontend/dist: no matching files found` -- pre-existing, unrelated to this plan (the Wails frontend has not been built in this worktree); `go build ./internal/...` is unaffected and green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The mutation pipeline (`mutate.go`) is a stable, proven seam: `mutationMutex`, `mutateRequest`/`mutationResult`, `checkRevision`, `dryRunMutate`, and the `idempotencyStore` are all ready for 07-06's batch engine to reuse directly (07-06-PLAN.md's own text: "batch reuses 07-05's serialized mutation mutex... + the copy-and-discard mechanism").
- `observer.go`'s `MutationEvent`/`RegisterMutationObserver` seam is ready for 07-07 (audit) and 07-08 (SSE) to attach to with zero further plumbing -- every outcome (`success`/`failure`/`dry_run`/`idempotent_replay`) is already populated with `Route`/`Args`/`Actor`/`Source`/`CorrelationID`/`ExpectedRevision`/`ResultingRevision`/`StatusCode`.
- `show.CurrentRevision`/`show.NewTempCopy` are now general-purpose exported accessors any future `internal/api` file can use without re-deriving Phase 5's verified-backup discipline.
- **Not yet closed:** only `"pool create"` is wired as a mutating REST operation; the remaining ~40 show-mutating command routes stay in `coverage_test.go`'s exclusion set. 07-09's own "close capability coverage" task will need either its own wave of route-by-route wiring or an explicit acknowledgment that this remains a documented, deliberate scope boundary from this plan (see `key-decisions`).
- API-04 is not yet functionally complete: revision/dry-run/idempotency/serialization are proven here; atomic `/v1/batch` (the other half of D-15) is 07-06's job.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED
- FOUND: internal/api/mutate.go
- FOUND: internal/api/mutate_test.go
- FOUND: internal/api/observer.go
- FOUND: internal/api/revision.go
- FOUND: internal/api/dryrun.go
- FOUND: internal/api/dryrun_test.go
- FOUND: internal/api/idempotency.go
- FOUND: internal/api/idempotency_test.go
- FOUND: internal/show/store.go
- FOUND: internal/show/backup.go
- FOUND: internal/api/server.go
- FOUND: internal/api/coverage_test.go
- FOUND: commit 1952bf9
- FOUND: commit 6c70521
- FOUND: commit 1e3aeef
