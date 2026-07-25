---
phase: 07-versioned-external-control-api
verified: 2026-07-25T12:00:00Z
status: gaps_found
score: 3/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 2/4
  gaps_closed:
    - "An external program can query and invoke every supported public domain capability through /api/v1, with Wails/HTTP parity (Success Criterion #1 / API-01) — CLOSED via a deliberate, honest re-scope (07-10): ROADMAP.md Success Criterion #1 and REQUIREMENTS.md API-01 now describe only the breadth /v1 actually delivers (config/show inspect, pool create, api-key lifecycle, atomic batch, revisioned events), and the remaining domains + the Wails/HTTP parity check are named to a new, mechanically-enforced deferral owner (EXTN-05, v1.x). `TestFutureWorkExclusionsNameDeferralOwner` proves the deferral pointer cannot be silently dropped."
    - "A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state, with no silently-missing gap (Success Criterion #3 / API-03, CR-01) — CLOSED. `internal/api/events.go`'s `ringEvent.Seq` is a strictly monotonic, per-process sequence, independent of `show.State.Revision`, assigned once under `b.mu` inside `publish`. A multi-sub-request batch's events are now individually addressable for Last-Event-ID replay. Verified by reading the implementation directly and running the two dedicated regression tests: `TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents` and `TestSSEFutureLastEventIDResyncs`, both PASS."
  gaps_remaining:
    - "Every API mutation result is auditable (Success Criterion #4 / API-06 component) — NOT closed. The original finding (WR-02: batch pre-flight scope/translation rejections were unaudited) IS fixed (07-12) — confirmed by direct code read of `batch.go`'s pre-flight loop (3 `fireMutationObservers` calls) and a passing parity test (`TestBatchAndSingleMutationScopeRejectionsAuditIdentically`). However, `runBatch`'s LOCKED section (`mutationMutex.Lock()` at line 237 through the final `show.Save` at line 299) still has 8 distinct failure-return paths that never call `fireMutationObservers`, including the batch-level If-Match precondition failure (412) and the pre-commit external-write race (412) — both reachable via existing, passing tests (`TestBatchIfMatch`, `TestBatchIfMatchExternalRace`) that assert only on status code, never on the audit log. This is the same defect class as the closed WR-02/WR-03, in a code region the gap-closure round's fix did not reach (identified fresh by 07-REVIEW-gaps.md as WR-05, confirmed independently here by direct code trace)."
  regressions: []
gaps:
  - truth: "Every API mutation result is auditable (Success Criterion #4 / API-06 component: 'every result is auditable')."
    status: partial
    reason: >
      Confirmed by direct code trace of internal/api/batch.go (lines 176-315):
      the pre-flight loop (lines 200-235, fixed by 07-12) fires
      fireMutationObservers on all three of its failure paths, and
      mutate.go's every failure branch (scope-lookup error, RequireScope
      rejection, checkRevision/412, translate error) also fires it (lines
      180-259) -- the single-mutation pipeline is fully audited, and the
      original WR-02/WR-03 findings are genuinely closed. However,
      runBatch's LOCKED section (mutationMutex.Lock() at line 237 through
      the final show.Save at line 299) contains 8 distinct failure returns
      that never call fireMutationObservers: show.CurrentRevision failure
      (240-243), parseIfMatch failure (245-248), the batch-level If-Match
      precondition mismatch/412 (249-252), show.NewTempCopy failure
      (254-257), a per-sub-request execution/translateResult failure mid-
      batch (264-267), show.Load failure on the copy (271-274), the
      pre-commit external-write race/412 (280-288), and show.Save failure
      (299-301). Two of these (the batch-level If-Match 412 and the
      external-write-race 412) are exercised by existing, passing,
      reachable-via-real-HTTP tests (TestBatchIfMatch, TestBatchIfMatchExternalRace
      in batch_test.go) that assert only status code and revision, never an
      audit row -- confirmed by reading both tests directly. A stale
      If-Match on /v1/batch is a routine, expected optimistic-concurrency
      client interaction (not merely an adversarial probe), so this is not
      a narrow edge case: it means an ordinary rejected batch mutation
      attempt today leaves zero audit evidence, while the textually
      identical rejection via POST /v1/pools (mutate.go's checkRevision
      branch) is fully audited. 07-12-PLAN.md's own prohibition text ("MUST
      NOT leave a rejected or failed mutation attempt unaudited on any
      reachable code path: a request refused for scope, translation, or
      precondition reasons is precisely the kind of attempt the audit trail
      exists to record") explicitly names "precondition reasons" as
      in-scope -- yet the implementation's own summary (07-12-SUMMARY.md)
      confirms the delivered fix only reaches the pre-flight loop (`grep -c
      'fireMutationObservers' internal/api/batch.go == 4`: 3 pre-flight +
      1 success-fan-out), not the locked section's precondition-failure
      branch. The audit writer itself (audit.go) is sound and unconditional
      for every event it receives -- the defect is purely that the event is
      never fired for these 8 branches.
    artifacts:
      - path: "internal/api/batch.go"
        issue: "Every failure return inside runBatch's locked section (lines 240-301), including the batch-level If-Match 412 and the pre-commit external-write-race 412, never calls fireMutationObservers, so no audit_log row is written for these reachable, already-tested failure paths"
    missing:
      - "A fireMutationObservers(MutationEvent{Outcome: \"failure\", ...}) call (one per affected sub-request, per 07-REVIEW-gaps.md WR-05's suggested shape) immediately before each of the 8 unaudited locked-section returns in runBatch, mirroring mutate.go's unconditional-fire discipline"
      - "Regression tests asserting an audit_log row exists for at least the batch-level If-Match mismatch and the external-write-race paths, analogous to TestBatchScopeRejectionIsAudited -- both TestBatchIfMatch and TestBatchIfMatchExternalRace currently assert status/revision only"
human_verification: []
---

# Phase 7: Versioned External Control API Verification Report

**Phase Goal:** External programs can inspect and control all public GOLC capabilities through a secure, documented, revision-aware API that behaves like the desktop application.
**Verified:** 2026-07-25
**Status:** gaps_found
**Re-verification:** Yes — after gap-closure round (07-10 through 07-14), following the initial 07-VERIFICATION.md (2/4, gaps_found)

## Goal Achievement

### Observable Truths (mapped to CURRENT ROADMAP Success Criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | An external program can, through `/api/v1`, inspect config/show state, create a fixture pool, mint/list/revoke scoped API keys, apply an atomic multi-command batch, and subscribe to revisioned events — each dispatched through the same command route registry the UI uses, with a committed coverage gate naming every remaining public route and its deferral owner (SC1, re-scoped by 07-10 / API-01) | ✓ VERIFIED | ROADMAP.md line 354 and REQUIREMENTS.md line 100 both now describe the delivered breadth only, and name EXTN-05 (v1.x, `.planning/REQUIREMENTS.md` line 163) as the deferral owner for the remaining show domains + Art-Net runtime + Wails/HTTP parity. `internal/api/coverage_test.go`'s three deferred-category reason constants (`reasonArtnetFutureWork`, `reasonMutationFutureWork`, `reasonReadFutureWork`) each name "EXTN-05" verbatim; `TestFutureWorkExclusionsNameDeferralOwner` mechanically asserts this and that the two permanent categories do NOT claim a deferral owner. `TestCapabilityCoverage`/`TestNoPendingRoutes` still pass (never-both/never-neither route-membership gate intact). |
| 2 | A client can generate against the published OpenAPI contract, follow working examples, handle typed errors, and understand the documented compatibility/deprecation policy (SC2 / API-02) | ✓ VERIFIED | `docs/api/openapi.json` regenerated and byte-stable (`TestOpenAPIDrift`/`TestOpenAPIDeterministic` PASS). `docs/api/COMPATIBILITY.md` now also documents the deprecation header mechanism as *real* (07-14 wired `DeprecationMiddleware` into `buildRouter`, confirmed by `TestBuildRouterInstallsDeprecationMiddleware` PASS and by reading `router.go`'s `UseMiddleware` call directly), the API-key lifetime bound (`GOLC_API_KEY_LIFETIME_TOO_LONG`, 8760h), and the comma-delimiter restriction (`GOLC_API_LIST_VALUE_INVALID`). |
| 3 | A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state (SC3 / API-03) | ✓ VERIFIED | CR-01 closed: `internal/api/events.go`'s `ringEvent.Seq` (int64, package/broadcaster-scoped, assigned once under `b.mu` inside `publish`, line 166-167) is now the sole ordering/dedupe key for the SSE `id:` line and `subscribe`'s replay/resync decision (line 258: `bufEv.Seq > lastID`), fully decoupled from `show.State.Revision`. Confirmed by direct code read and by running `TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents` and `TestSSEFutureLastEventIDResyncs` — both PASS. A multi-sub-request batch's events are now individually replayable even though they share one `Revision`. |
| 4 | Mutations support expected revisions, idempotency, dry-run previews, and atomic batches; every result is auditable; loopback is default; remote access requires explicit enablement + scoped auth (SC4 / API-04/API-05/API-06) | ✗ FAILED (partial) | Revision/If-Match, dry-run, composite-key idempotency (WR-01 fixed — `TestIdempotencyKeyScopedByActor` PASS), comma-delimiter input validation (IN-02 fixed), atomic `/v1/batch`, loopback-by-default, and scoped API-key auth are all proven end-to-end and pass. `mutate.go`'s single-mutation pipeline is fully audited on every failure branch (WR-02/WR-03 fixed for the pre-flight batch loop and the mutate.go scope-lookup branch — confirmed by direct code read and `TestBatchAndSingleMutationScopeRejectionsAuditIdentically` PASS). **However**, "every result is auditable" is still false: `runBatch`'s locked section (batch.go:240-301) has 8 failure-return paths — including the batch-level If-Match 412 and the pre-commit external-write-race 412 — that never call `fireMutationObservers`, confirmed by direct code trace and by reading `TestBatchIfMatch`/`TestBatchIfMatchExternalRace`, which exercise exactly these branches via real HTTP requests but assert only status code, never an audit row. |

**Score:** 3/4 truths verified (0 present-behavior-unverified)

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|
| API-01 | 07-02, 07-05, 07-06, 07-09, **07-10** | Query/invoke a documented, coverage-gated subset of public capability, with the remainder named and deferred to EXTN-05 | SATISFIED | Claim narrowed to match delivered scope (07-10); mechanically enforced by `TestFutureWorkExclusionsNameDeferralOwner`, `TestCapabilityCoverage`, `TestNoPendingRoutes`, all PASS. REQUIREMENTS.md still shows `[ ]`/"Pending" in its traceability table — expected, since the plan explicitly leaves checkbox-flipping to the phase-completion workflow, not this gap-closure round. |
| API-02 | 07-09, **07-14** | OpenAPI contract, generated examples, typed errors, compatibility/deprecation guidance | SATISFIED | Unchanged from initial verification plus 07-14's now-real deprecation header mechanism; all OpenAPI drift/determinism/coverage tests PASS. |
| API-03 | 07-08, **07-11** | Revisioned SSE + gap recovery via authoritative re-query | SATISFIED | CR-01 closed; monotonic `Seq` decouples SSE ordering from `Revision`; both regression tests PASS. |
| API-04 | 07-05, 07-06, **07-13** | Expected revisions, idempotency, dry-run, atomic batches | SATISFIED | WR-01 (idempotency cross-actor leak) closed via composite `(actor, route, key)` key; `TestIdempotencyKeyScopedByActor` PASS. All mechanically proven for the one wired mutating route. |
| API-05 | 07-03, 07-04, **07-14** | Loopback default, explicit enablement + scoped auth for remote access | SATISFIED | Unchanged from initial verification (already solid) plus 07-14's API-key lifetime bound (IN-01 closed) and scopes-comma rejection (IN-02 closed for keys.go). |
| API-06 | 07-07, 07-09, **07-12** | Actor/source/correlation/outcome/redacted audit details on every mutation | **BLOCKED (partial)** | Single-mutation pipeline and batch pre-flight rejections are now fully audited (WR-02/WR-03 closed, confirmed by code trace + `TestBatchAndSingleMutationScopeRejectionsAuditIdentically`). But `runBatch`'s locked-section failure paths (including the routine batch-level If-Match 412) remain unaudited (WR-05, newly surfaced by the gap-closure code review, independently confirmed here) — "every API mutation records audit details" does not hold for that path. REQUIREMENTS.md still correctly shows API-06 as `[ ]`/"Pending", consistent with this finding. |

**Orphaned requirements check:** REQUIREMENTS.md maps exactly API-01..API-06 to Phase 7; all six appear in at least one of the 14 plans' `requirements:` frontmatter (07-01 through 07-14, including the five gap-closure plans). No orphans.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `.planning/ROADMAP.md` (Phase 7 SC1) | Delivered-scope claim + named deferral owner | ✓ VERIFIED | Line 354 rewritten by 07-10; matches delivered breadth. |
| `.planning/REQUIREMENTS.md` (API-01, EXTN-05) | Narrowed API-01 text; new EXTN-05 v1.x requirement | ✓ VERIFIED | Line 100 (API-01) and line 163 (EXTN-05) present and consistent; EXTN-05 correctly excluded from the v1 traceability table per the existing EXTN-01..04 convention. |
| `internal/api/coverage_test.go` | Deferral pointer mechanically enforced | ✓ VERIFIED | `TestFutureWorkExclusionsNameDeferralOwner` PASS; deferred categories name EXTN-05, permanent categories do not. |
| `internal/api/events.go` | Monotonic SSE sequence decoupled from Revision | ✓ VERIFIED | `ringEvent.Seq`/`nextSeq` present, wired, tested. |
| `internal/api/batch.go` | Atomic `/v1/batch`, D-15, full audit parity with mutate.go | ⚠️ PARTIAL | Atomicity intact; pre-flight audit parity fixed; locked-section audit parity still missing (WR-05, see gap above). |
| `internal/api/idempotency.go` | Composite `(actor, route, key)` scoping | ✓ VERIFIED | `idempotencyKey` struct confirmed; `TestIdempotencyKeyScopedByActor` PASS. |
| `internal/api/router.go`, `internal/api/deprecation.go` | Deprecation middleware installed | ✓ VERIFIED | `buildRouter` installs `DeprecationMiddleware(humaAPI)`; `TestBuildRouterInstallsDeprecationMiddleware` PASS. |
| `internal/api/keys.go` | Lifetime bound + scopes comma rejection | ✓ VERIFIED | `maxAPIKeyLifetime` (8760h) enforced; `validateListValues` shared with mutate.go/batch.go. |
| `docs/api/openapi.json`, `docs/api/COMPATIBILITY.md` | Published contract + policy, updated for gap-closure changes | ✓ VERIFIED | Regenerated, drift-checked, documents new SSE id semantics, lifetime bound, and comma restriction. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/api/coverage_test.go` deferred reasons | `.planning/REQUIREMENTS.md` EXTN-05 | String match, `TestFutureWorkExclusionsNameDeferralOwner` | WIRED | Confirmed passing; test fails if the EXTN-05 clause is removed (per plan's own documented RED/GREEN cycle). |
| `internal/api/batch.go` pre-flight loop | `internal/api/observer.go` | `fireMutationObservers` | WIRED | 3 pre-flight calls confirmed present and tested. |
| `internal/api/batch.go` locked section | `internal/api/observer.go` | `fireMutationObservers` | **NOT WIRED** | Zero calls across 8 failure-return paths (lines 240-301) — see gap above. |
| `internal/api/batch.go` (multi-sub-request success) | `internal/api/events.go` (`ringEvent.Seq`/`eventBroadcaster`) | `fireMutationObservers` → `publishMutationEvent` | WIRED, INVARIANT RESTORED | Events now individually replayable; CR-01 closed. |
| `internal/api/router.go` | `internal/api/deprecation.go` | `UseMiddleware(..., DeprecationMiddleware(humaAPI))` | WIRED | Confirmed by direct source read and `TestBuildRouterInstallsDeprecationMiddleware`. |
| `internal/api/mutate.go`, `internal/api/batch.go`, `internal/api/keys.go` | `internal/api/mutate.go`'s `validateListValues` | Shared validator call | WIRED | Confirmed single validator used by all three comma-joined-field call sites. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `go build ./internal/...` | `go build ./internal/...` | exit 0 | PASS |
| Full Phase 7 package suites | `go test ./internal/api/... ./internal/show/... ./internal/command/... ./internal/artnet/... ./internal/routecatalog/...` | all `ok` | PASS |
| Gap-closure regression tests (named, single run) | `go test ./internal/api/... -run 'TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents\|TestSSEFutureLastEventIDResyncs\|TestIdempotencyKeyScopedByActor\|TestBuildRouterInstallsDeprecationMiddleware\|TestFutureWorkExclusionsNameDeferralOwner\|TestBatchScopeRejectionIsAudited\|TestBatchTranslationFailureIsAudited' -v` | all PASS (7/7) | PASS |
| OpenAPI drift/determinism/coverage gates | `go test ./internal/api/... -run 'TestOpenAPIDrift\|TestOpenAPIDeterministic\|TestOpenAPIDocumentsEveryOperation\|TestCapabilityCoverage\|TestNoPendingRoutes' -v` | all PASS | PASS |
| WR-05 residual-gap confirmation (direct code trace, not a written/executed repro this round — code inspection of batch.go lines 176-315 plus the two named existing tests) | Read `internal/api/batch.go`, `internal/api/mutate.go`; read `TestBatchIfMatch`/`TestBatchIfMatchExternalRace` in `batch_test.go` | Zero `fireMutationObservers` calls in the locked section (lines 240-301); neither test asserts an audit row | Confirms the gap (real, not resolved) |
| `gofmt -l internal/api/*.go` | `gofmt -l internal/api/*.go` | `internal/api/observer.go` | ℹ️ Info — pre-existing cosmetic whitespace issue, unchanged from initial verification, non-blocking |

### Anti-Patterns Found

| File | Line(s) | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/api/batch.go` | 240-301 | `runBatch`'s locked-section failure returns never fire the mutation observer (8 distinct branches) | 🛑 Blocker (narrower than original WR-02, same defect class) | Batch-level If-Match/412 and external-write-race rejections leave no audit trail, unlike the identical single-mutation path (Success Criterion #4 / API-06) |
| `internal/api/keys.go` | ~73-90 | `expires_in` has no lower bound (negative/zero durations pass through) | ℹ️ Info (IN-03, carried forward from 07-REVIEW-gaps.md, not blocking) | Low severity, admin-scope-gated |
| `internal/api/events.go` | 432, 448 | `int64` `Seq` narrowed to `int` for the SSE `id:` line | ℹ️ Info (IN-04, carried forward, not blocking) | No-op on current 64-bit build target |
| `internal/api/observer.go` | (whole file) | `gofmt -l` reports a whitespace-alignment issue | ℹ️ Info (carried forward, cosmetic) | Cosmetic only |

No `TBD`/`FIXME`/`XXX` debt markers found in any Phase 7 file (`internal/api/*.go`, `internal/show/apikeys.go`, `internal/show/audit.go`, `internal/command/apikey.go`, `docs/api/*`), including the five gap-closure plans' files.

### Human Verification Required

None. The residual finding (WR-05 / batch locked-section audit gap) was resolved programmatically: confirmed by direct code read of `internal/api/batch.go` (zero `fireMutationObservers` calls across the 8 locked-section failure returns) cross-referenced against `internal/api/mutate.go`'s fully-audited equivalent branches, and against the two named existing tests (`TestBatchIfMatch`, `TestBatchIfMatchExternalRace`) that exercise the exact reachable failure paths via real HTTP requests without asserting on the audit log.

### Gaps Summary

The gap-closure round (07-10 through 07-14) genuinely closed 2 of the original 3 verification gaps and 4 of the 4 non-blocking review findings it targeted:

- **Gap 1 (SC1 / API-01 breadth) — CLOSED.** A deliberate, honest re-scope: ROADMAP.md and REQUIREMENTS.md now claim only the delivered breadth, and the remaining domains are named to a mechanically-enforced deferral owner (EXTN-05, v1.x). This is a legitimate resolution, not a silent scope reduction — the deferred capability list is enumerated by name in both files and the pointer is test-guarded against deletion.
- **Gap 2 (SC3 / API-03, CR-01 SSE data loss) — CLOSED.** The monotonic `Seq` fix is correct, well-tested, and independently confirmed by direct code read plus passing regression tests for both the original multi-sub-request-batch scenario and the daemon-restart future-id scenario.
- **Gap 3 (SC4 / API-06, WR-02/WR-03 pre-flight audit gap) — CLOSED for the specific branches originally named.** `runBatch`'s pre-flight loop and `mutate.go`'s scope-lookup-error branch are now fully audited, confirmed by direct code trace and a passing single-vs-batch parity test.
- **WR-01 (idempotency cross-actor leak), WR-04 (deprecation middleware not installed), IN-01 (unbounded key lifetime), IN-02 (unescaped comma delimiter) — all CLOSED**, each confirmed by direct code read and a passing named regression test.

**However, one genuine gap remains and blocks Success Criterion #4:** the gap-closure code review's fresh-eyes pass (07-REVIEW-gaps.md) surfaced WR-05 — `runBatch`'s LOCKED section (everything from `mutationMutex.Lock()` through the final `show.Save`, batch.go lines 240-301) has 8 distinct failure-return paths that never fire the mutation observer, so no audit_log row is ever written for them. This verification independently confirmed WR-05 by direct code trace (not merely accepting the review's classification): reading `batch.go` line-by-line shows zero `fireMutationObservers` calls in that region, and reading `mutate.go`'s equivalent branches shows every one of them unconditionally fires the observer. Two of the eight branches — the batch-level If-Match precondition mismatch (412) and the pre-commit external-write race (412) — are exercised today by existing, passing, real-HTTP-request tests (`TestBatchIfMatch`, `TestBatchIfMatchExternalRace`), confirming these are reachable, routine failure modes (a stale If-Match on a batch request is an ordinary optimistic-concurrency conflict, not an edge case or an adversarial probe), not hypothetical ones.

This is judged to be the SAME class of defect that caused Success Criterion #4 to fail in the initial verification (an unaudited, reachable, already-tested rejection path on `/v1/batch` that its single-mutation sibling does audit), just in a code region the 07-12 gap-closure plan's scope did not reach. 07-12-PLAN.md's own prohibition text ("MUST NOT leave a rejected or failed mutation attempt unaudited on any reachable code path: a request refused for scope, translation, or precondition reasons...") explicitly names "precondition reasons" as in-scope, yet the delivered fix (confirmed via `07-12-SUMMARY.md`'s own `grep -c 'fireMutationObservers' internal/api/batch.go == 4` acceptance check) only reaches the pre-flight loop's 3 branches plus the success-path fan-out — not the locked section's precondition-failure branch. "Every result is auditable" therefore remains not fully true of the delivered system.

**Recommendation:** A narrow, well-scoped closure plan (mirroring 07-12's own approach, applied to the 8 locked-section branches instead of the pre-flight loop) should close this before Phase 7 is marked complete. The fix shape is already specified in 07-REVIEW-gaps.md's WR-05 finding.

---

_Verified: 2026-07-25_
_Verifier: Claude (gsd-verifier)_
