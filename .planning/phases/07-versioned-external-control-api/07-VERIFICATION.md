---
phase: 07-versioned-external-control-api
verified: 2026-07-25T16:46:17Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "Every API mutation result is auditable (Success Criterion #4 / API-06 component) — CLOSED by 07-15. runBatch's LOCKED section (mutationMutex.Lock() through the aggregated show.Save) now fires a failure MutationEvent before all nine of its failure returns (eight batch-level fan-outs via a new fireBatchFailureObservers closure, one single-index fire for the sub-request-attributable translateResult error), matching mutate.go's and the pre-flight loop's (07-12) unconditional-fire discipline. Independently confirmed in this pass by reading internal/api/batch.go directly (not just 07-REVIEW-wr05.md's claims) and tracing all nine fire/return line pairs (283/284, 289/290, 296/297, 303/304, 319/324, 331/332, 347/348, 355/356, 371/372) — every fire immediately precedes its return with no interleaving statement. Confirmed the exact grep counts the plan's acceptance criteria specify (fireMutationObservers==6, fireBatchFailureObservers==8, one closure definition, one hoisted expectedRevisionPtr declaration) and ran the six new tests plus the full internal/api suite myself; all pass, including the source-structure gate TestBatchLockedSectionFailureReturnsAreAllAudited which pairs each return with its own preceding fire (not just independent totals) and correctly skips the doc-comment line that also mentions \"mutationMutex.Lock()\" in prose."
  gaps_remaining: []
  regressions: []
human_verification: []
---

# Phase 7: Versioned External Control API Verification Report

**Phase Goal:** External programs can inspect and control all public GOLC capabilities through a secure, documented, revision-aware API that behaves like the desktop application.
**Verified:** 2026-07-25
**Status:** passed
**Re-verification:** Yes — third pass, after gap-closure round 2 (07-15), following the second verification (3/4, gaps_found, sole remaining gap WR-05)

## Goal Achievement

### Observable Truths (mapped to CURRENT ROADMAP Success Criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | An external program can, through `/api/v1`, inspect config/show state, create a fixture pool, mint/list/revoke scoped API keys, apply an atomic multi-command batch, and subscribe to revisioned events — each dispatched through the same command route registry the UI uses, with a committed coverage gate naming every remaining public route and its deferral owner (SC1, re-scoped by 07-10 / API-01) | ✓ VERIFIED | Unchanged since prior pass; 07-15 did not touch any file in this area (`git show --stat` on both 07-15 commits shows only `internal/api/batch.go` and `internal/api/batch_test.go`). Re-ran `TestCapabilityCoverage`, `TestNoPendingRoutes`, `TestFutureWorkExclusionsNameDeferralOwner` — all PASS, confirming no regression. |
| 2 | A client can generate against the published OpenAPI contract, follow working examples, handle typed errors, and understand the documented compatibility/deprecation policy (SC2 / API-02) | ✓ VERIFIED | Unchanged since prior pass; not touched by 07-15. `TestOpenAPIDrift`/`TestOpenAPIDeterministic` re-run and PASS — `docs/api/openapi.json` is byte-unchanged, confirming 07-15 was a pure audit-completeness change with no contract impact, as its own plan required. |
| 3 | A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state (SC3 / API-03) | ✓ VERIFIED | Unchanged since prior pass; `internal/api/events.go` not touched by 07-15. `events.go`'s `publishMutationEvent` returns early for any non-"success" `Outcome`, so the nine new failure fires 07-15 added publish zero SSE events and cannot perturb the monotonic `Seq` sequence CR-01 restored — confirmed by reading that early-return guard directly. |
| 4 | Mutations support expected revisions, idempotency, dry-run previews, and atomic batches; every result is auditable; loopback is default; remote access requires explicit enablement + scoped auth (SC4 / API-04/API-05/API-06) | ✓ VERIFIED | Revision/If-Match, dry-run, composite-key idempotency, atomic `/v1/batch`, loopback-by-default, and scoped API-key auth remain proven end-to-end (unchanged since prior pass). **The prior pass's sole gap — "every result is auditable" — is now closed.** Confirmed by direct read of `internal/api/batch.go`: all nine of `runBatch`'s locked-section failure returns (revErr, parseErr, If-Match mismatch, copyErr, translateErr, loadErr, raceErr, raceRevision mismatch, saveErr) are each immediately preceded by an audit-observer fire (`fireBatchFailureObservers` for the eight batch-level failures, a direct `fireMutationObservers` for the one sub-request-attributable failure). Spot-checked line pairs 283/284, 296/297, 319/324, 371/372 by hand against the source; all nine pairs have no interleaving statement between fire and return. Ran all six of 07-15's new tests plus the full `internal/api` package suite myself (not just reading 07-REVIEW-wr05.md's claims) — all PASS. |

**Score:** 4/4 truths verified (0 present-behavior-unverified)

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| API-01 | 07-02, 07-05, 07-06, 07-09, 07-10 | Query/invoke a documented, coverage-gated subset of public capability, with the remainder named and deferred to EXTN-05 | SATISFIED | Unchanged; mechanically enforced by `TestFutureWorkExclusionsNameDeferralOwner`, `TestCapabilityCoverage`, `TestNoPendingRoutes`, all PASS (re-run this pass). REQUIREMENTS.md line 100 still shows `[ ]`/"Pending" in its checkbox and traceability table — this is administrative bookkeeping the plan deliberately leaves to the phase-completion workflow, not a functional gap; the underlying behavior and the requirement's own (narrowed) text are both satisfied. |
| API-02 | 07-09, 07-14 | OpenAPI contract, generated examples, typed errors, compatibility/deprecation guidance | SATISFIED | Unchanged; all OpenAPI drift/determinism/coverage tests PASS (re-run). |
| API-03 | 07-08, 07-11 | Revisioned SSE + gap recovery via authoritative re-query | SATISFIED | Unchanged; monotonic `Seq` decouples SSE ordering from `Revision`, both regression tests re-run and PASS. |
| API-04 | 07-05, 07-06, 07-13 | Expected revisions, idempotency, dry-run, atomic batches | SATISFIED | Unchanged; `TestIdempotencyKeyScopedByActor` re-run and PASS. |
| API-05 | 07-03, 07-04, 07-14 | Loopback default, explicit enablement + scoped auth for remote access | SATISFIED | Unchanged from prior pass. |
| API-06 | 07-07, 07-09, 07-12, 07-15 | Actor/source/correlation/outcome/redacted audit details on every mutation | **SATISFIED** | WR-05 closed: `runBatch`'s locked-section failure returns now all fire the audit observer. REQUIREMENTS.md line 105 flipped to `[x]`/"Complete" in commit `5f0b753` (07-15's docs commit), a clean, isolated checkbox flip (`git show 5f0b753 -- .planning/REQUIREMENTS.md` shows only the two `[ ]`→`[x]` line edits, no text rewording). Confirmed independently in this pass rather than trusting that commit alone. |

**Orphaned requirements check:** REQUIREMENTS.md maps exactly API-01..API-06 to Phase 7; all six appear in at least one of the 15 plans' `requirements:` frontmatter (07-01 through 07-15). No orphans.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/api/batch.go` | Atomic `/v1/batch`, D-15, full audit parity with mutate.go across both the pre-flight loop and the locked section | ✓ VERIFIED | Read directly (413 lines). Doc comment (lines 170-183) accurately describes the nine-branch fire discipline. `fireBatchFailureObservers` closure (lines 268-276) fans out one failure row per sub-request for batch-level failures; `expectedRevisionPtr` hoisted once (line 253), read at call time by every fire site — nil for the 2 sites before If-Match parsing, the parsed value for the 7 after. All nine return sites individually confirmed preceded by a fire. |
| `internal/api/batch_test.go` | Behavioral tests for the 4 reachable locked-section paths + 1 batch-vs-single parity test + 1 structural gate for all 9 | ✓ VERIFIED | `TestBatchStaleIfMatchIsAudited`, `TestBatchAndSingleMutationStaleIfMatchAuditIdentically`, `TestBatchMalformedIfMatchIsAudited`, `TestBatchExternalWriteRaceIsAudited`, `TestBatchSubRequestExecutionFailureIsAudited`, `TestBatchLockedSectionFailureReturnsAreAllAudited` all present (confirmed via `grep -n "^func Test"`) and all PASS under `-race -v`, run directly in this verification pass. |
| `.planning/phases/07-versioned-external-control-api/07-REVIEW-wr05.md` | Independent code review confirming the fix | ✓ VERIFIED (spot-checked, not trusted blindly) | Review's line-by-line fire/return pairing claims cross-checked against the live source; matches. Its two Info-level findings (IN-01: 5 of 9 branches have no behavioral test, by design; IN-02: minor duplication between the two fan-out loops) are non-blocking and accurately described. |
| `.planning/REQUIREMENTS.md` (API-06) | Checkbox + traceability table flipped to Complete | ✓ VERIFIED | Commit `5f0b753` is a clean, isolated flip; no requirement text was altered. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/api/batch.go` locked section (all 9 failure returns) | `internal/api/observer.go` | `fireMutationObservers` / `fireBatchFailureObservers` | **WIRED** | All 9 fire/return pairs confirmed by direct source read this pass: lines 283→284, 289→290, 296→297, 303→304, 319→324, 331→332, 347→348, 355→356, 371→372. No return in the region lacks a preceding fire. |
| `internal/api/batch.go` locked-section failure fires | `internal/api/events.go` `publishMutationEvent`'s `Outcome != "success"` early return | Non-invocation confirmed by reading the guard | WIRED, NO SSE LEAKAGE | The nine new failure fires cannot publish an SSE event or perturb the monotonic `Seq` sequence. |
| `internal/api/batch.go` fire calls | `internal/api/audit.go` `auditObserver` → `internal/show.WriteAuditRecord` | Existing seam, no new write path | WIRED | Same seam mutate.go and the pre-flight loop already use; `redactArgs`/`internal/security.Redact` apply unchanged (D-16). No new production Go types or write paths introduced (confirmed: `git show --stat` shows only edits inside existing functions). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `go build ./internal/...` | `go build ./internal/...` | exit 0 | PASS |
| `go vet ./internal/api/...` | `go vet ./internal/api/...` | no output | PASS |
| Six named WR-05 closure tests | `go test ./internal/api/... -run 'TestBatchLockedSectionFailureReturnsAreAllAudited\|TestBatchStaleIfMatchIsAudited\|TestBatchAndSingleMutationStaleIfMatchAuditIdentically\|TestBatchMalformedIfMatchIsAudited\|TestBatchExternalWriteRaceIsAudited\|TestBatchSubRequestExecutionFailureIsAudited' -race -v` | all 6 PASS | PASS |
| Full `internal/api` package suite (single run) | `go test ./internal/api/... -race` | `ok  github.com/lnorton89/golc/internal/api  24.524s` | PASS |
| OpenAPI + coverage gates (regression check, unaffected files) | `go test ./internal/api/... -run 'TestOpenAPIDrift\|TestOpenAPIDeterministic\|TestCapabilityCoverage\|TestNoPendingRoutes\|TestFutureWorkExclusionsNameDeferralOwner' -v` | all 5 PASS | PASS |
| Fire/count grep gates from 07-15's own acceptance criteria | `grep -v '^[[:space:]]*//' internal/api/batch.go \| grep -c 'fireMutationObservers('` → 6; same for `fireBatchFailureObservers(` → 8; closure def count → 1; `expectedRevisionPtr \*int64` count → 1 | 6 / 8 / 1 / 1 | PASS (matches plan exactly) |
| Debt-marker scan | `grep -n -E "TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER" internal/api/batch.go internal/api/batch_test.go` | no matches | PASS |
| Formatting | `gofmt -l internal/api/batch.go internal/api/batch_test.go` | no output | PASS |

### Anti-Patterns Found

None blocking. Carried-forward, non-blocking Info items (unchanged from prior pass, not touched by 07-15):

| File | Line(s) | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/api/keys.go` | ~73-90 | `expires_in` has no lower bound (negative/zero durations pass through) | ℹ️ Info (IN-03, carried forward) | Low severity, admin-scope-gated |
| `internal/api/events.go` | 432, 448 | `int64` `Seq` narrowed to `int` for the SSE `id:` line | ℹ️ Info (IN-04, carried forward) | No-op on current 64-bit build target |
| `internal/api/observer.go` | (whole file) | `gofmt -l` reports a whitespace-alignment issue | ℹ️ Info (carried forward, cosmetic) | Cosmetic only |
| `internal/api/batch.go` / `batch.go:268-276` and `380-386` | Minor structural duplication between `fireBatchFailureObservers` and the trailing success fan-out | ℹ️ Info (IN-02, 07-REVIEW-wr05.md, new this round) | Low priority; both blocks share the same shape and could be collapsed into one parameterized helper, not required |

No `TBD`/`FIXME`/`XXX` debt markers found in any Phase 7 file, including the two files 07-15 modified.

### Human Verification Required

None. All four Success Criteria and all six requirement IDs (API-01 through API-06) are verified programmatically, by direct code read and passing tests run in this verification pass — not by trusting SUMMARY.md or 07-REVIEW-wr05.md claims alone.

### Gaps Summary

None remaining. This is the third and final verification pass for Phase 7.

**History across the three passes:**
- **Pass 1 (initial):** 2/4 — gaps in SC1 (API-01 breadth overclaim) and SC3 (SSE CR-01 data-loss bug), plus a partial SC4 (audit trail).
- **Pass 2 (after 07-10..07-14):** 3/4 — SC1 closed via honest re-scope to EXTN-05; SC3 closed via monotonic `Seq`; SC4 still failed on WR-05 (`runBatch`'s locked section had 8 — actually 9, as this round's own scope-correction found — unaudited failure returns).
- **Pass 3 (this pass, after 07-15):** 4/4 — WR-05 closed. `runBatch`'s locked section now fires the audit observer before all nine of its failure returns, confirmed independently in this pass by direct source read (not just by trusting 07-REVIEW-wr05.md's line-by-line trace) and by running the six new tests plus the full `internal/api` suite myself.

**Phase 7 goal is achieved.** External programs can inspect and control the delivered `/api/v1` breadth (config/show inspect, pool create, API-key lifecycle, atomic batch, revisioned events) through the same command route registry the desktop UI uses; the OpenAPI contract, typed errors, and compatibility/deprecation policy are published and real; SSE events are strictly ordered and gap-recoverable; and every mutation attempt — success or failure, single or batched, in the pre-flight loop or the locked transactional core — now leaves a redacted, reconstructable audit trail. Loopback remains the default bind and remote access requires explicit enablement plus scoped authentication. The remaining show domains and the Wails/HTTP parity check are honestly named and deferred to EXTN-05 (v1.x), not silently dropped.

**Recommendation:** Proceed to mark Phase 7 complete. One administrative item is worth a follow-up note, not a blocker: REQUIREMENTS.md's API-01 checkbox/table entry is still `[ ]`/"Pending" even though ROADMAP.md's Success Criterion #1 and REQUIREMENTS.md's own API-01 description already reflect the delivered (narrowed) scope — this appears to be intentionally left for the phase-completion workflow to flip (as 07-10's own plan stated), not an oversight, but the orchestrator should confirm that flip happens as part of closing out Phase 7.

---

_Verified: 2026-07-25_
_Verifier: Claude (gsd-verifier)_
