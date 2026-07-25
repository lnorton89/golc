---
phase: 07-versioned-external-control-api
reviewed: 2026-07-25T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/api/coverage_test.go
  - internal/api/events.go
  - internal/api/events_test.go
  - internal/api/batch.go
  - internal/api/mutate.go
  - internal/api/batch_test.go
  - internal/api/idempotency.go
  - internal/api/idempotency_test.go
  - internal/api/router.go
  - internal/api/deprecation_test.go
  - internal/api/keys.go
  - internal/api/keys_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 07: Code Review Report (Gap-Closure Round)

**Reviewed:** 2026-07-25T00:00:00Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

This is a gap-closure re-review of 07-10..07-14. All five stated gap-closure items were verified directly against source and are genuinely fixed, with solid, targeted test coverage for each:

- **CR-01 (SSE dedupe/replay keyed off Revision, not a monotonic id)** — FIXED. `events.go`'s `ringEvent.Seq` is now a strictly monotonic, per-process counter assigned exactly once inside `publish` under `b.mu`, fully independent of `show.State.Revision`. `subscribe`'s branch correctly distinguishes `lastID == latest` ("caught up", no resync) from `lastID > latest` (an id this process never issued — e.g. post-restart — explicit resync). Both the original multi-sub-request-batch reconnect scenario and the future-id/daemon-restart scenario have dedicated, well-constructed tests (`TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents`, `TestSSEFutureLastEventIDResyncs`) that I traced by hand against the implementation and confirmed pass logically. Locking discipline between `publish` and `subscribe` (both take `b.mu` for their entire critical section) rules out the obvious race where a client could subscribe mid-publish and miss or duplicate an event.
- **WR-02/WR-03 (audit gaps in batch/mutate pre-flight)** — FIXED for the specific branches named in the original findings: `batch.go`'s pre-flight loop (translation error, scope-lookup error, scope rejection) and `mutate.go`'s scope-lookup-error branch all now call `fireMutationObservers` before returning. However, see WR-05 below — the fix is narrower than the surrounding claim that "batch.go's runBatch" is now audited; several failure paths *inside* `runBatch`'s locked section remain silent.
- **WR-01 (idempotency scoping)** — FIXED. The store keys on `idempotencyKey{actor, route, key}`, not the raw client-supplied string, and `TestIdempotencyKeyScopedByActor` proves two actors sharing a key each get their own result.
- **WR-04 (deprecation middleware not installed)** — FIXED. `buildRouter` installs `DeprecationMiddleware(humaAPI)` alongside `AuthMiddleware`/`RateLimitMiddleware` in the single `UseMiddleware` call, and `TestBuildRouterInstallsDeprecationMiddleware` pins this by reading `router.go`'s own source so a future edit can't silently drop it.
- **IN-01/IN-02 (unbounded key expiry, unescaped comma-joined list values)** — FIXED. `keys.go` caps `expires_in` at `maxAPIKeyLifetime` (365 days) without disturbing the existing malformed-duration diagnostic, and `validateListValues` rejects any comma-bearing element in both `scopes` (keys.go) and `requires` (mutate.go's single-mutation path and batch.go's translator), at the HTTP boundary, with matching regression tests proving comma-free values still work.

One real, provable gap remains, found during the fresh-eyes pass: `runBatch`'s locked section (everything from `mutationMutex.Lock()` through the final `show.Save`) still has multiple failure returns that fire no observer at all — including the batch-level If-Match precondition failure and the pre-commit external-write race, both of which have dedicated behavioral tests (`TestBatchIfMatch`, `TestBatchIfMatchExternalRace`) that never assert on the audit log, which is presumably why this was missed. This is the same class of defect WR-02/WR-03 were opened to fix, just in a part of the file the gap-closure plans didn't reach.

## Warnings

### WR-05: `runBatch`'s locked-section failure paths still write no audit row

**File:** `internal/api/batch.go:240-301`
**Issue:** The WR-02/WR-03 fix only reaches the *pre-flight* loop (translation error, scope-lookup error, scope rejection — lines 202-235), which runs before `mutationMutex` is acquired. Every failure return from `mutationMutex.Lock()` (line 237) through the final `show.Save` (line 299) fires no `fireMutationObservers` call at all:

- `show.CurrentRevision` failure (line 240-243)
- `parseIfMatch` failure (line 245-248)
- the batch-level If-Match precondition mismatch, 412 (line 249-252) — this is the most consequential omission: it's a routine, expected client-facing failure mode (optimistic-concurrency conflict), has two dedicated behavioral tests (`TestBatchIfMatch`, `TestBatchIfMatchExternalRace` in `batch_test.go`), and neither test asserts an audit row exists, unlike the sibling `TestBatchScopeRejectionIsAudited`/`TestBatchTranslationFailureIsAudited` tests that do assert audit rows for the pre-flight-loop paths.
- `show.NewTempCopy` failure (line 254-257)
- a per-sub-request execution failure against the throwaway copy, i.e. `translateResult` erroring mid-batch (line 264-267) — note this *does* hold `mutationMutex` and *has* already touched a real (if throwaway) copy of the show, making its lack of an audit trail more surprising than the pre-flight loop's, not less
- `show.Load` failure on the copy (line 271-274)
- the pre-commit external-write race, 412 (line 280-288) — `TestBatchIfMatchExternalRace` exercises exactly this branch and also does not check the audit log
- `show.Save` failure on the real show (line 299-301)

Contrast with `mutate.go`: every one of its failure branches (scope lookup, scope rejection, `checkRevision` failure covering parse/lookup/mismatch, and execution/translate failure) unconditionally calls `fireMutationObservers` before returning (lines 188-259). `runBatch` is not at parity with the single-mutation pipeline it's supposed to mirror — a client hammering `/v1/batch` with a stale `If-Match` (or racing a concurrent external writer) leaves zero audit evidence, while the identical failure through `POST /v1/pools` is fully audited.

**Fix:** Add a `fireMutationObservers` call (outcome `"failure"`, correct `StatusCode` via `statusFromHumaErr`) immediately before each of the returns listed above, mirroring `mutate.go`'s unconditional-fire discipline. For the batch-level If-Match branches specifically, consider firing one observer event per sub-request in `requests` (matching how a rejected batch's audit rows are otherwise reported one-per-sub-request elsewhere in this file), e.g.:

```go
if ifMatchPresent && expectedRevision != baseRevision {
    err := huma.Error412PreconditionFailed(fmt.Sprintf(
        "GOLC_API_REVISION_MISMATCH: If-Match %d does not match the current revision %d", expectedRevision, baseRevision))
    for i, route := range routes {
        fireMutationObservers(MutationEvent{
            Route: route, Args: args[i], Actor: actor, Source: "http",
            CorrelationID: correlationID, Outcome: "failure", StatusCode: http.StatusPreconditionFailed,
        })
    }
    return nil, err
}
```

Add regression tests analogous to `TestBatchScopeRejectionIsAudited` for the If-Match-mismatch and external-race paths at minimum, since those are the two branches with existing dedicated behavioral tests that could trivially gain an audit-row assertion.

## Info

### IN-03: `expires_in` has no lower bound — a negative or zero duration is accepted and forwarded downstream

**File:** `internal/api/keys.go:73-77`
**Issue:** The new boundary check only rejects `requested > maxAPIKeyLifetime`. `time.ParseDuration("-1h")` succeeds and produces a negative duration, which is not greater than the max, so it passes through unrejected to `"api-key create"`. Depending on how `internal/command/apikey.go`'s `runAPIKeyCreate` computes the resulting `ExpiresAt` from a negative or zero duration, this could mint a key that is already expired at creation (mostly harmless — `IsAPIKeyValid`/`apiKeyStillValid` would just reject it immediately) or, worse, one whose expiry computation wraps in some unexpected way. This wasn't in scope for IN-01 (which is about an *unbounded* ceiling) but is an adjacent gap in the same validation the gap-closure plan touched.
**Fix:** Add a companion lower-bound check, e.g. `requested <= 0`, alongside the existing `> maxAPIKeyLifetime` check, with its own diagnostic code (e.g. `GOLC_API_KEY_LIFETIME_TOO_SHORT` or fold it into the existing message).

### IN-04: `int64` `Seq` truncated to `int` for the SSE `id:` line

**File:** `internal/api/events.go:432,448`
**Issue:** `sse.Message{ID: int(ev.Seq)}` narrows `ringEvent.Seq` (`int64`) to Go's platform-dependent `int`. On a 64-bit build target (this project's current Windows/amd64 target) this is a no-op, but it is not portable — a hypothetical 32-bit build would silently wrap the id after ~2.1 billion committed mutations in a single daemon's lifetime, at which point `subscribe`'s id comparisons would start comparing wrapped (possibly negative) values against the buffer's `int64` `Seq`s incorrectly. Low practical severity given current build targets, but worth tightening since huma's `sse.Message.ID` type is the actual constraint being cast against, not a hard architectural requirement.
**Fix:** Confirm `sse.Message.ID`'s declared type; if the huma API accepts a wider integer type or a string, prefer that over the `int()` narrowing conversion. If `int` is unavoidable, consider documenting the platform assumption inline where the cast occurs.

---

_Reviewed: 2026-07-25T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
