---
phase: 07-versioned-external-control-api
verified: 2026-07-25T00:00:00Z
status: gaps_found
score: 2/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "An external program can query and invoke every supported public domain capability through /api/v1, and parity checks show the same commands have the same outcomes through Wails and HTTP (Success Criterion #1 / API-01)."
    status: failed
    reason: >
      Only 6 of the real command registry's ~86 public routes are actually
      reachable over /v1 (config inspect, show inspect, pool create,
      api-key create/list/revoke), plus 2 synthetic API-only endpoints
      (batch apply, events watch). internal/api/coverage_test.go's own
      exclusion set honestly documents 61 routes as reasonArtnetFutureWork
      (10), reasonMutationFutureWork (42), and reasonReadFutureWork (9) --
      all explicitly labeled NOT permanent, i.e. still-owed future work,
      not a deliberate permanent scope boundary. Every scene/chase/motion/
      theme/preset/blend/deployment/operatorsurface/playback/Art-Net-
      runtime/programmer/fixture-import/show-open-save capability is
      entirely absent from /v1. 07-09-SUMMARY.md's own "Known Gaps"
      section already states this plainly: "Capability-coverage closure
      is NOT fully achieved." No Wails<->HTTP parity check exists either,
      since only one mutating domain (pool) has any REST exposure to
      compare against.
    artifacts:
      - path: "internal/api/coverage_test.go"
        issue: "61 of the ~86 real command routes remain in future-work exclusion categories (reasonArtnetFutureWork/reasonMutationFutureWork/reasonReadFutureWork), not permanent exclusions"
      - path: "internal/api/mutate.go"
        issue: "Only \"pool create\" is wired as a mutating REST operation (of ~43 show-mutating command routes)"
    missing:
      - "REST operations (Huma input structs + RegisterOperation wiring, reusing mutate.go's proven pipeline) for the remaining show-domain mutation routes (chase/scene/motion/theme/preset/blend/deployment/operatorsurface/playback/programmer/fixture-import/show-open-save)"
      - "REST GET operations for the remaining show-domain read/inspect routes, several of which additionally need JSON-output or path-handling design work (fixture inspect, operatorsurface list/show, programmer inspect)"
      - "Art-Net daemon runtime routes wired to REST operations, or an explicit ROADMAP.md re-scope of API-01 to formally defer Art-Net runtime control to a named future phase"
      - "A genuine Wails<->HTTP outcome-parity test once more than one mutating domain exists"
  - truth: "A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state (Success Criterion #3 / API-03), with no silently-missing gap (D-10)."
    status: failed
    reason: >
      Confirmed live and reproducible (not merely a code-review claim):
      runBatch (internal/api/batch.go) fires one MutationEvent per
      sub-request but all of them share the SAME resultingRevision. events.go's
      ringEvent/eventBroadcaster key replay and "already caught up" purely
      off Revision, so a client that received only the first of a
      multi-sub-request batch's events and reconnects with that event's id
      is told "already caught up" (lastID >= latest) -- no replay, no
      resync -- even though it never received the batch's remaining
      same-revision events. This directly falsifies the phase's own
      documented "never a silently missing gap" (D-10) guarantee for a
      reachable, already-shippable code path (a multi-sub-request /v1/batch
      of "pool create" sub-requests is fully invokable today). Verified by
      writing and running a standalone reproduction test
      (TestCR01Repro_BatchMultiEventSameRevisionSilentlyDrops, executed
      against the real HTTP handler and real SSE stream, then removed --
      not committed) that opens a live /v1/events subscription, issues a
      3-sub-request batch, reads only the first live frame, disconnects,
      reconnects with that frame's id, and observes silence for 2s with no
      resync and no replay -- the predicted symptom, reproduced.
    artifacts:
      - path: "internal/api/events.go"
        issue: "ringEvent has only a Revision field, used as both the SSE id and the sole replay/dedupe ordering key; no strictly-monotonic sequence number exists to distinguish same-revision events (07-REVIEW.md CR-01, independently reproduced)"
      - path: "internal/api/batch.go"
        issue: "runBatch (lines ~268-279) fires N MutationEvents sharing one resultingRevision for an N-sub-request batch"
    missing:
      - "A strictly monotonic sequence number (independent of show.State.Revision) used for the SSE id: line and subscribe()'s replay/resync comparisons, per 07-REVIEW.md CR-01's suggested fix"
      - "A test that opens a live event stream, issues a 2+ sub-request batch, disconnects after the first frame, reconnects with that frame's id, and asserts the remaining sub-request events are still delivered (not silently dropped) -- neither batch_test.go nor events_test.go currently combines a multi-sub-request batch with a live/reconnecting SSE subscriber"
  - truth: "Every API mutation result is auditable (Success Criterion #4 / API-06 component: 'every result is auditable')."
    status: failed
    reason: >
      Confirmed by direct code trace (matches 07-REVIEW.md WR-02): the
      single-mutation pipeline (mutate.go) fires a "failure" MutationEvent
      -- and therefore writes an audit_log row -- when RequireScope
      rejects a request. runBatch's own pre-lock translate/scope-check
      loop (batch.go lines ~184-200) returns on all three of its failure
      paths (translation error, scope-lookup error, RequireScope failure)
      WITHOUT ever calling fireMutationObservers, so an identical
      scope-probing attempt produces an audit_log row via POST /v1/pools
      but zero rows via POST /v1/batch. TestBatchRequiresScope
      (batch_test.go:364-386) only asserts the 403 status and unchanged
      revision -- it never asserts an audit row exists, so this gap is
      real and untested, not merely theoretical.
    artifacts:
      - path: "internal/api/batch.go"
        issue: "translate/scope-lookup/RequireScope failures in runBatch's pre-flight loop (lines ~184-200) never call fireMutationObservers, so they leave no audit_log row"
    missing:
      - "A fireMutationObservers(MutationEvent{Outcome: \"failure\", ...}) call for each translate/scope failure inside runBatch's pre-flight loop, mirroring mutate.go's own scope-failure branch"
      - "An audit_test.go assertion that a scope-rejected batch sub-request still produces exactly one audit_log row"
human_verification: []
---

# Phase 7: Versioned External Control API Verification Report

**Phase Goal:** External programs can inspect and control all public GOLC capabilities through a secure, documented, revision-aware API that behaves like the desktop application.
**Verified:** 2026-07-25
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (mapped to ROADMAP Success Criteria)

| # | Truth (Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | An external program can query and invoke every supported public domain capability through `/api/v1`, with Wails/HTTP parity (SC1 / API-01) | FAILED | Only 6 of ~86 real command routes are wired to REST (config inspect, show inspect, pool create, api-key create/list/revoke); 61 routes are explicitly labeled future-work (not permanent) in `internal/api/coverage_test.go`'s exclusion set; 07-09-SUMMARY.md's own "Known Gaps" section states this outright. No Wails<->HTTP parity check exists beyond the one wired mutating route. |
| 2 | A client can generate against the published OpenAPI contract, follow working examples, handle typed errors, and understand the compatibility/deprecation policy (SC2 / API-02) | VERIFIED | `docs/api/openapi.json` (871 lines, generated, byte-stable drift-checked via `TestOpenAPIDrift`/`TestOpenAPIDeterministic`), `docs/api/COMPATIBILITY.md` (232 lines: versioning, breaking-change definition, 180-day deprecation window, header signals, typed-error table, curl examples for query/If-Match/dry-run/batch/key-mint/SSE), `internal/api/deprecation.go`'s `MarkOperationDeprecated`/`DeprecationMiddleware` unit-tested. All confirmed passing directly (`go test ./internal/api/... -run 'TestOpenAPI...'`). |
| 3 | A client can consume revisioned server-sent events, detect a replay gap, and recover by querying authoritative state, with no silently-missing gap (SC3 / API-03) | FAILED | Live-reproduced: a 3-sub-request `/v1/batch` publishes 3 SSE events sharing one revision; a client that saw only the first and reconnects with that id is told "already caught up" — no replay, no resync — silently losing the remaining events. Single-mutation SSE behavior (order/replay/gap-resync/adjacency/broadcast/auth/cross-scope/revocation) is genuinely solid and passes (`TestSSE*`, 9/9). The general "no silently missing gap" guarantee is false for the (already-reachable) multi-sub-request-batch path. |
| 4 | Mutations support expected revisions, idempotency, dry-run previews, and atomic batches; every result is auditable; loopback is default; remote access requires explicit enablement + scoped auth (SC4 / API-04/API-05/API-06) | FAILED (partial) | Revision/If-Match (412), dry-run (`?dry_run=true`), Idempotency-Key replay, and atomic `/v1/batch` (all-or-nothing, single real revision bump, rollback, ordering, If-Match race re-check) are all proven end-to-end for the one wired mutating route (`pool create`) — `TestMutateIfMatchRevisionLifecycle`, `TestDryRun*`, `TestIdempotency*`, `TestBatch*` all pass. Loopback-by-default enforced at bind time (`listenAddr`, `TestLoopbackDefault`/`TestRemoteRequiresInterface`) and scoped API-key auth (`AuthMiddleware`, `TestAuthRejectsMissingUnknownExpiredAndRevokedKeys`) are both solid. **However** "every result is auditable" is false: a batch sub-request rejected for translation/scope failure never fires the observer seam, so no audit_log row is written for that outcome (unlike the identical single-mutation 403 path, which is audited) — confirmed by direct code trace of `runBatch`'s pre-flight loop. |

**Score:** 2/4 truths verified (0 present-behavior-unverified)

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|---|---|---|---|---|
| API-01 | 07-02, 07-05, 07-06, 07-09 | Query/invoke every public capability through a versioned API, Wails-command-model parity | BLOCKED | Coverage mechanism (translation, capability-coverage gate, single-mutation and batch pipelines) is proven and sound, but only 6/~86 routes are actually wired; 61 explicitly deferred as future work. Not satisfiable as stated by this phase's delivered scope. |
| API-02 | 07-09 | OpenAPI contract, generated examples, typed errors, compatibility/deprecation guidance | SATISFIED | `docs/api/openapi.json` + `docs/api/COMPATIBILITY.md` + `deprecation.go`, all tested and passing. |
| API-03 | 07-08 (+ interaction with 07-06) | Revisioned SSE + gap recovery via authoritative re-query | BLOCKED | Single-mutation path fully proven; multi-sub-request-batch path silently drops events with no resync (CR-01, reproduced live). The "never a silently missing gap" guarantee is not universally true. |
| API-04 | 07-05, 07-06 | Expected revisions, idempotency, dry-run, atomic batches | SATISFIED (mechanically, for the wired route) | If-Match/412, dry-run, idempotency-replay, and atomic all-or-nothing batch all proven end-to-end for `pool create`. Note (not blocking): WR-01 — the idempotency store keys purely on the raw client-supplied `Idempotency-Key` string, not `(actor, route, key)`, a latent cross-actor/cross-route replay risk once more mutating routes exist. |
| API-05 | 07-03, 07-04 | Loopback default, explicit enablement + scoped auth for remote access | SATISFIED | `listenAddr` enforces loopback unless `remote_enabled=true` AND `bind_interface` is explicit (fails loudly, never silently 0.0.0.0); `AuthMiddleware` requires a valid, non-expired, non-revoked key on every request; per-key rate limiting; scopes (playback/authoring/admin) gate mutations. All test-proven. |
| API-06 | 07-07, 07-09 | Actor/source/correlation/outcome/redacted audit details on every mutation | BLOCKED (partial) | Every single-mutation outcome (success/failure/dry_run/idempotent_replay) and every successfully-committed batch's sub-events are audited and redacted correctly (proven, including a dedicated redaction test and a source-grep single-writer-discipline test). But a batch's pre-flight translate/scope failures are NOT audited (WR-02, confirmed by code trace) — "every API mutation records ... audit details" does not hold for that path. |

**Orphaned requirements check:** REQUIREMENTS.md maps exactly API-01..API-06 to Phase 7; all six appear in at least one plan's `requirements:` frontmatter (07-01 through 07-09). No orphans.

### Required Artifacts (representative — full list in each plan's SUMMARY.md)

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/api/` package | Chi+Huma /v1 server, translation, auth, mutation pipeline, batch, audit, SSE, generate | VERIFIED | All files present, substantive, wired; `go build ./internal/...` clean. |
| `internal/api/coverage_test.go` | Every real command route mapped or excluded, no silent gaps | VERIFIED (mechanism) / gap in outcome | Gate itself is real and passing (`TestCapabilityCoverage`, `TestNoPendingRoutes`), but the *outcome* it certifies is "61 routes honestly deferred," not "fully covered." |
| `internal/show/apikeys.go`, `internal/show/audit.go` | API-key store + audit_log store, single-writer discipline | VERIFIED | crypto/rand keys, SHA-256 hash + prefix, `openStore` reuse confirmed via source-grep test. |
| `internal/api/events.go` | Revisioned SSE ring buffer, replay, resync-on-overflow | VERIFIED for single mutations / FAILED for multi-sub-request batches | See Success Criterion #3 above. |
| `internal/api/batch.go` | Atomic `/v1/batch`, D-15 | VERIFIED (atomicity) / FAILED (SSE + audit side-effects) | Atomicity itself (`TestBatchAtomic`/`TestBatchRollback`/`TestBatchIfMatchExternalRace`) is genuinely proven at the data layer; the interaction with events.go and the pre-flight-failure audit gap are the failures above. |
| `docs/api/openapi.json`, `docs/api/COMPATIBILITY.md` | Published contract + policy | VERIFIED | Present, generated, drift-checked, documented. |
| `config/api.toml` + `api` projectconfig concern | Loopback-default, remote-access config | VERIFIED | Registered concern, `api.remote_enabled` writable, resolved through `internal/api/config.go`. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/command/artnet.go` (`runArtnetServe`) | `internal/api.NewServer` | `apiCommandExecutor` adapter + `Subsystem` interface | WIRED | `TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP` proves real daemon hosting. |
| `internal/api/mutate.go` | `internal/api/observer.go` | `fireMutationObservers` | WIRED (single mutation) / PARTIALLY WIRED (batch pre-flight failures skip it) | See API-06 gap above. |
| `internal/api/observer.go` | `internal/api/audit.go`, `internal/api/events.go` | `RegisterMutationObserver`/`RegisterAuditObserver` | WIRED | Both registered; `RegisterAuditObserver` confirmed wired into `NewServer` (closing 07-07's flagged gap) in commit `3f5a843`. |
| `internal/api/batch.go` (multi-sub-request success) | `internal/api/events.go` (`ringEvent`/`eventBroadcaster`) | `fireMutationObservers` -> `publishMutationEvent` | WIRED but BROKEN INVARIANT | Events are delivered, but the shared-revision defect breaks Last-Event-ID replay/resync — see Success Criterion #3. |
| `internal/api` package | `internal/command` package | Executor interface, never a direct import | VERIFIED | `grep -rn "internal/command" internal/api/` returns 0 matches (structural, mechanically enforced). |
| `internal/api` config | `internal/projectconfig` `api` concern | `ResolveConfig` | WIRED | Loopback-default enforced at bind time in production (`runArtnetServe`), not just in unit tests. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| `go build ./internal/...` | `go build ./internal/...` | exit 0 | PASS |
| Phase 7 package test suites | `go test ./internal/api/... ./internal/show/... ./internal/command/... ./internal/artnet/... ./internal/routecatalog/...` | all `ok` (cached green) | PASS |
| OpenAPI drift/determinism/coverage gates | `go test ./internal/api/... -run 'TestOpenAPIDrift\|TestOpenAPIDeterministic\|TestOpenAPIDocumentsEveryOperation\|TestCapabilityCoverage\|TestNoPendingRoutes' -v` | all PASS | PASS |
| CR-01 reproduction (multi-sub-request batch SSE gap) | Standalone `internal/api/cr01_repro_test.go` (written for this verification, executed, then removed — not committed) opening a live SSE stream, issuing a 3-sub-request batch, disconnecting after the first frame, reconnecting with that frame's id | `--- FAIL`: reconnect received nothing within 2s (no replay, no resync) — predicted symptom reproduced | FAILED (confirms the gap, i.e. the spot-check surfaces a real defect) |
| Pre-existing unrelated failure check | `go test ./internal/trace/catalog/... -run TestScopeLinearMap` | fails with `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json` | Confirmed pre-existing/unrelated (linear-map tracking-file drift, not Phase 7 code); excluded from this phase's must-haves per known context. |
| Documented regression fix present | `git show --stat bb0a381` | present, committed, updates a stale offline-acceptance test string for the new drift-check output | PASS (confirms the known-context fix landed) |

### Anti-Patterns Found

| File | Line(s) | Pattern | Severity | Impact |
|---|---|---|---|---|
| `internal/api/events.go` | 106-111, 135-155, 191-219 | `ringEvent.Revision` doubles as both domain revision and SSE replay/dedupe key; no monotonic sequence number | 🛑 Blocker | Silent event loss for multi-sub-request batches (Success Criterion #3 / CR-01) |
| `internal/api/batch.go` | 184-200 | Pre-flight translate/scope-check failures never fire the mutation observer | 🛑 Blocker (narrower) | Batch scope-probing/translation-error attempts leave no audit trail, unlike the equivalent single-mutation path (Success Criterion #4 / WR-02) |
| `internal/api/idempotency.go` | 45-87 | Idempotency store keyed by raw client string only, not `(actor, route, key)` | ⚠️ Warning | Cross-actor/cross-route replay risk once more mutating routes exist; low-impact today (one route, no sensitive body) |
| `internal/api/mutate.go` | 153-157 | `requiredScopeForRoute` error path returns 500 without firing the observer | ⚠️ Warning (latent, unreachable today) | Every currently-registered route has a `domainScope` entry, so not exploitable yet, but is a trap for the next contributor wiring a new mutating route |
| `internal/api/router.go` vs `internal/api/deprecation.go` | 113-124 vs 71-93 | `DeprecationMiddleware` built and tested but never installed in `buildRouter` | ⚠️ Warning | No operation is deprecated yet, so currently a no-op gap; the `Deprecation`/`Sunset` headers `COMPATIBILITY.md` documents as load-bearing would silently never appear the day an operation is actually marked deprecated, unless this is fixed first |
| `internal/api/keys.go`, `internal/api/batch.go` | 27-32 / 94-98, 286-289 | `expires_in` has no documented upper bound; `,`-joined list fields (`requires`/`scopes`) have no escaping | ℹ️ Info | Low-severity, admin-scope-gated / controlled-vocabulary today |
| `internal/api/observer.go` | (whole file) | `gofmt -l` reports a whitespace-alignment issue (introduced in `1952bf9`, flagged by 07-09-SUMMARY.md itself, not yet fixed) | ℹ️ Info | Cosmetic only |

No `TBD`/`FIXME`/`XXX` debt markers found in any Phase 7 file (`internal/api/*.go`, `internal/show/apikeys.go`, `internal/show/audit.go`, `internal/command/apikey.go`, `docs/api/*`).

### Human Verification Required

None. Every finding above was resolved programmatically: the coverage gap is countable from `coverage_test.go`'s own exclusion-set source, and CR-01/WR-02 were each independently confirmed — CR-01 via a live, executed (then removed) reproduction test against the real HTTP handler and real SSE stream, WR-02 via direct code trace of `runBatch`'s pre-flight loop cross-referenced against `mutate.go`'s equivalent branch and `TestBatchRequiresScope`'s actual assertions.

### Gaps Summary

Phase 7 delivered a substantial, well-tested, and genuinely correct piece of *infrastructure*: the Chi+Huma `/v1` server, the command-translation seam, config-driven loopback enforcement, scoped API-key auth with per-key rate limiting, a serialized mutation pipeline (revision/dry-run/idempotency), atomic-batch commit semantics, a redacting audit writer, a revisioned SSE broadcaster, and a byte-stable generated OpenAPI contract with a compatibility/deprecation policy are all real, well-designed, and pass their own tests — including under `-race`.

However, the phase's stated goal — "External programs can inspect and control **all** public GOLC capabilities ... that behaves like the desktop application" — is not yet true of the delivered system, for three independently-confirmed reasons:

1. **Breadth (Success Criterion #1 / API-01):** only 6 of ~86 real command routes are reachable over `/v1`. 61 routes are explicitly, honestly labeled as deferred future-milestone work by the phase's own final plan (07-09-SUMMARY.md's "Known Gaps" section already says this outright) — this verification independently confirms the same count from the coverage gate's own source. Every domain except config/show(read)/pool(create)/api-key has zero REST exposure.
2. **Correctness (Success Criterion #3 / API-03, CR-01):** the SSE gap-recovery mechanism, which the phase explicitly documents as guaranteeing "never a silently missing gap" (D-10), is demonstrably broken for an already-reachable code path (a multi-sub-request `/v1/batch`). This was independently reproduced live in this verification, not merely inferred from the prior code review.
3. **Completeness (Success Criterion #4 / API-06):** "every result is auditable" does not hold for a batch's pre-flight scope/translation failures, which leave no audit trail, unlike the identical single-mutation rejection path.

None of these are hypothetical or forward-looking concerns — all three are demonstrated against code that ships today. The phase is not ready to be marked complete against its own stated Success Criteria without either (a) closing gaps 2 and 3 (both narrow, well-scoped fixes matching 07-REVIEW.md's own CR-01/WR-02 recommendations) and (b) making an explicit scoping decision on gap 1 (either commit to a follow-up wiring effort, or formally re-scope API-01's "every capability" claim in ROADMAP.md/REQUIREMENTS.md to the domains this phase actually covers).

---

_Verified: 2026-07-25_
_Verifier: Claude (gsd-verifier)_
