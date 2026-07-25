---
phase: 07-versioned-external-control-api
reviewed: 2026-07-25T16:42:59Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - internal/api/batch.go
  - internal/api/batch_test.go
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: issues_found
---

# Phase 07: Code Review Report (WR-05 gap-closure, round 3)

**Reviewed:** 2026-07-25T16:42:59Z
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found (info-only; no blockers or warnings)

## Summary

07-15 modifies `runBatch`'s locked section (from `mutationMutex.Lock()` through the
aggregated `show.Save`) so that all nine failure-return paths fire an audit observer
before returning, closing WR-05. I traced every one of the nine `return nil, `
statements individually against the preceding fire call, cross-checked the fan-out
semantics (batch-level failure = one row per sub-request via
`fireBatchFailureObservers`; the one sub-request-attributable failure = exactly one
row for the culpable index via a direct `fireMutationObservers` call), verified
`expectedRevisionPtr`'s hoisting is read-at-call-time and carries the correct value
(nil before `parseIfMatch` runs or on parse failure, the parsed value for the seven
sites after), and re-derived the structural test's own line-range scan by hand
(confirmed via `grep` that the literal string `mutationMutex.Lock()` appears exactly
twice in the file — once in a `//` doc comment the scanner correctly skips, once as
the real statement — and that `resultingRevision := baseRevision + 1` appears exactly
once, so the scanned region is unambiguous). I ran `go build`, `go vet`, and the full
`Test.*Batch.*` suite (26 tests, including all 6 new tests and
`TestBatchLockedSectionFailureReturnsAreAllAudited`); all pass.

**Verification against the review's six checkpoints:**

1. **All 9 locked-section failure paths fire the observer** — confirmed line-by-line.
   Fire sites at 283/289/296/303/319/331/347/355/371 each immediately precede their
   paired return at 284/290/297/304/324/332/348/356/372, with no other return or fire
   statement interleaved between a pair.
2. **Fan-out semantic is correct** — `fireBatchFailureObservers` (batch.go:268-276)
   iterates `routes`/`args` (populated for every sub-request during the pre-flight
   loop, since only a fully-translated, in-scope batch ever reaches
   `mutationMutex.Lock()`), firing one event per sub-request for all 8 batch-level
   failure sites. The one sub-request-attributable failure (batch.go:313-325, the
   `translateResult` error inside the execution loop) fires exactly once, scoped to
   `routes[i]`/`args[i]` alone, matching the 07-12-established single-row semantic —
   confirmed both by static trace and by `TestBatchSubRequestExecutionFailureIsAudited`
   asserting the non-culpable sub-request's name is absent from the one failure row.
3. **No double-firing** — the pre-flight loop (lines 208-243) returns immediately on
   any of its three failure branches, before `mutationMutex.Lock()` is ever reached,
   so no request can hit both a pre-flight fire and a locked-section fire. Within the
   locked section, every fire+return pair is immediately followed by a `return`
   (function exit), so no code path can reach two fire calls before returning. The
   structural test's `fireCount != wantCount` check independently corroborates this:
   any accidental double-fire on one branch would inflate `fireCount` past 9 while
   `returnCount` stays 9, failing the test.
4. **`expectedRevisionPtr` hoisting is correct** — declared nil (batch.go:253), set
   once at batch.go:292-294 only if `ifMatchPresent`, never reassigned afterward. The
   two sites that run before this assignment (`revErr` at 283, `parseErr` at 289)
   correctly observe nil; the seven sites after it (296, 303, 319, 331, 347, 355, 371)
   correctly observe the parsed value. Verified behaviorally: `TestBatchMalformedIfMatchIsAudited`
   asserts a null `expected_revision` for the parse-failure row, while
   `TestBatchStaleIfMatchIsAudited` and `TestBatchAndSingleMutationStaleIfMatchAuditIdentically`
   assert the parsed value is present on all fan-out rows for the later sites. This
   also matches the pre-flight loop's own fires (which never set `ExpectedRevision`
   at all, since `parseIfMatch` hasn't run yet at that point) — this is not an
   inconsistency, since `mutate.go`'s single-mutation scope-failure branches
   (mutate.go:182-199) exhibit the identical omission for the identical reason
   (checked for parity).
5. **Structural test's own correctness** — `TestBatchLockedSectionFailureReturnsAreAllAudited`
   locates its region markers by skipping `//`-prefixed lines first (so the function's
   own doc comment, which mentions `mutationMutex.Lock()` in prose, cannot be mistaken
   for the real statement), then requires a `fired` flag be `true` immediately before
   each `return nil, ` line and resets it after each return — this is a genuine
   "return preceded by its own fire" check, not two independently-counted totals that
   could coincidentally match (a `(fire, fire, return)` followed later by an unfired
   `return` would fail immediately via the per-return check, not just via the final
   tally). Confirmed via `grep` that the region markers are each unambiguous (single
   real occurrence) and that the fire/return line counts are exactly 9/9.
6. **Fresh-eyes pass** — no new race conditions, stale-closure captures, or
   composition bugs found between 07-12's pre-flight loop and 07-15's locked-section
   closure. `expectedRevisionPtr` is a local variable inside a single `runBatch`
   invocation (no cross-request state), Go closures capture variables by reference so
   `fireBatchFailureObservers` correctly reads `expectedRevisionPtr`/`routes`/`args`
   at call time (all already finalized by the time the closure is ever invoked), and
   `routes`/`args` are never mutated after the pre-flight loop populates them.

No blockers or warnings were found. Two minor Info-level observations follow.

## Info

### IN-01: Structural test cannot verify audit-row *content* for 5 of the 9 failure branches

**File:** `internal/api/batch_test.go:1191-1266`
**Issue:** `TestBatchLockedSectionFailureReturnsAreAllAudited`'s own doc comment
correctly discloses that 5 of the 9 locked-section branches (both
`show.CurrentRevision` calls, `show.NewTempCopy`, `show.Load`, `show.Save` failing)
have no behavioral test — they require fault injection the codebase deliberately
declines to add (the show file doubles as the auth key store, so corrupting it to
force a failure gets requests rejected at 401 before `runBatch` is ever entered).
This is a reasonable, well-justified trade-off, but it does mean the structural test
only proves *a* fire call textually precedes each of those 5 returns — it cannot
prove the fire call's `StatusCode`/`ExpectedRevision`/fan-out arguments are actually
correct for those 5 branches (unlike the other 4, which have behavioral tests: parse
failure, stale If-Match, external-write race, sub-request execution failure). This
review manually verified all 5 status codes (500/500/500/412/500) match their
returned HTTP errors by static trace, but a future edit to one of those 5 branches
(e.g. an accidental `http.StatusBadRequest` typo on a 500 branch) would pass both the
structural test and `go vet`/`go build` silently.
**Fix:** No action required now — this is an accepted, documented trade-off, not an
oversight. If a future change adds a fault-injection seam elsewhere in the daemon for
similar branches, consider reusing it here rather than leaving this permanently
structural-only.

### IN-02: Minor duplication between `fireBatchFailureObservers` and the trailing success fan-out

**File:** `internal/api/batch.go:268-276` and `internal/api/batch.go:380-386`
**Issue:** Both blocks are the same `for i, route := range routes { fireMutationObservers(MutationEvent{...}) }` shape, differing only in `Outcome`/`StatusCode`/`ResultingRevision`. Not a bug, but a small amount of structural duplication that a future refactor could collapse into one helper taking `(statusCode int, outcome string, resultingRevision *int64)`.
**Fix:** Optional; e.g.:
```go
fireBatchFanOut := func(statusCode int, outcome string, resultingRevision *int64) {
	for i, route := range routes {
		fireMutationObservers(MutationEvent{
			Route: route, Args: args[i], Actor: actor, Source: "http",
			CorrelationID: correlationID, ExpectedRevision: expectedRevisionPtr,
			ResultingRevision: resultingRevision, Outcome: outcome, StatusCode: statusCode,
		})
	}
}
```
Low priority — the current split is easy to reason about and the duplication is small.

---

_Reviewed: 2026-07-25T16:42:59Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
