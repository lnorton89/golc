---
phase: 07-versioned-external-control-api
plan: 08
subsystem: api
tags: [api, sse, events, revision, gap-recovery, streaming, huma]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-05: internal/api/observer.go's MutationEvent/RegisterMutationObserver seam (the source of published events, keyed by the resulting revision); 07-02: RegisterOperation self-registration seam + humachi API handle"
provides:
  - "internal/api/events.go: GET /v1/events -- a bounded revision-keyed ring buffer, Last-Event-ID replay, resync-on-overflow, all-clients broadcaster, typed huma/v2/sse.Register wiring, and a per-connection revocation/expiry tick"
  - "api.EventRingBufferCapacity / api.EventRevocationTickInterval: exported package vars (mirroring huma/v2/sse's own WriteTimeout precedent) a caller can tune, primarily so tests can force deterministic overflow/resync and fast revocation checks"
  - "api.ResetEventStreamForTesting(): test-only helper clearing this package's singleton broadcaster state and re-arming the SSE observer registration, needed because sibling *_test.go files' ResetMutationObserversForTesting calls otherwise permanently drop the SSE observer partway through the test binary"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Package-level singleton broadcaster (eventStreamBroadcaster), not a *Server field -- mirrors mutate.go's own mutationMutex precedent (exactly one *Server per daemon process, D-07), and kept this plan's changes fully contained to internal/api/events.go/events_test.go without touching server.go."
    - "Single generic \"state\" SSE event name carrying a Type field in its JSON payload, rather than one distinct Go type per domain event name -- huma/v2/sse.Register's eventTypeMap maps an SSE event name to exactly one Go type via reflection, so multiple names sharing one struct type would collide (the last-registered name would win for all of them). Since only \"pool create\" is wired as a concrete mutating route so far (07-05's own documented scope decision), inventing ~16 near-identical per-domain structs now would be speculative; the domain lives in the payload's own Type field instead, satisfying D-09's \"tagged with a type field\" requirement. Adding a genuinely distinct SSE event NAME per domain later is an additive, compatible change."
    - "Observer registration deferred to *Server-construction time (ensureEventStreamObserverRegistered, called from the /v1/events operation's own Register callback) instead of a package-init var initializer -- several sibling *_test.go files (mutate_test.go, dryrun_test.go) already call observer.go's ResetMutationObserversForTesting directly in their own setup/cleanup, which wipes the ENTIRE global observer list with no restoration; a package-init registration would be silently and permanently dropped the first time any sibling test's cleanup ran. ResetEventStreamForTesting (this plan's own test-only export) gives this file's tests a guaranteed clean re-arm regardless of test execution order."
    - "A synthetic, non-CLI RegisterOperation Route key (\"events watch\") for GET /v1/events -- this operation is pure new infrastructure (D-09), never a translated internal/command route, so it has no entry in routecatalog's registry and coverage_test.go's capability-coverage gate never sees it; the synthetic key exists solely so this operation participates in router.go's buildRouter registration loop like every other operation in this package."
    - "Non-blocking, drop-on-full broadcast delivery to live subscribers (select/default in eventBroadcaster.publish) -- publish is called synchronously from within mutate.go's held mutationMutex, and observer.go's own doc comment requires observers never block indefinitely; a subscriber that falls behind recovers via the same Last-Event-ID replay/resync path a fresh reconnect uses, never by stalling the next mutation."

key-files:
  created:
    - internal/api/events.go
    - internal/api/events_test.go
  modified: []

key-decisions:
  - "D-11/D-12 access semantics required zero additional gating code: router.go's AuthMiddleware already runs ahead of every /v1 operation (valid, non-expired key required, D-05), and this file's handler simply never calls RequireScope -- any valid key opens the stream (D-12), and every open connection observes every domain's events regardless of that key's scope (D-11), by omission rather than by an explicit bypass."
  - "Gap-recovery boundary math: a Last-Event-ID exactly one less than the buffer's oldest retained revision (lastID == oldest-1) is treated as IN-window (full replay of the whole buffer), not a gap -- since committed events are contiguous +1 per successful mutation, that boundary id proves nothing was actually missed. Only lastID < oldest-1 (a genuine hole between what the client last saw and what the buffer still retains) triggers resync. This plan's own tests use unambiguous margins (a Last-Event-ID clearly several revisions older than the shrunk buffer's oldest entry) so the exact boundary choice is not itself under test, but the implementation documents and applies the stricter, provably-correct interpretation rather than the plan text's more literal (and slightly over-eager) \"older than the oldest buffered revision\" wording."
  - "Revocation re-check (apiKeyStillValid) reimplements show.IsAPIKeyValid's exact predicate (RevokedAt.IsZero() && now.Before(ExpiresAt)) locally against show.ListAPIKeys' hash-free rows, rather than adding a new lookup-by-KeyID accessor to internal/show/apikeys.go. This plan's files_modified scope is internal/api/events.go/events_test.go only (confirmed non-overlapping with 07-07's sibling worktree, which does not touch apikeys.go either); ListAPIKeys already returns every field the predicate needs (RevokedAt/ExpiresAt/KeyID) with no internal/show change required."
  - "Both PLAN.md tasks (ring-buffer/replay/resync, and access-semantics/revocation-tick) landed in one commit rather than two: they share an identical files_modified list, and Task 2's own read_first explicitly names Task 1's broadcaster as \"the lifecycle the tick hooks into\" -- the two were designed and implemented as one coherent file, not two independently committable increments."

requirements-completed: ["API-03"]

coverage:
  - id: D1
    description: "After three successful mutations, a fresh subscriber with no Last-Event-ID opens live (no replay) and receives subsequent events in monotonic revision order"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEOrder"
        status: pass
    human_judgment: false
  - id: D2
    description: "A subscriber reconnecting with an in-window Last-Event-ID receives exactly the missed events, replayed in order, no duplicates"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEReplay"
        status: pass
    human_judgment: false
  - id: D3
    description: "A subscriber reconnecting with a Last-Event-ID older than the buffer's retained window receives a single resync event, never silently-missing state"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEGapRecovery"
        status: pass
    human_judgment: false
  - id: D4
    description: "A subscriber reconnecting with Last-Event-ID equal to the latest emitted revision receives no replay and no spurious resync -- the first frame it sees is the next genuinely new live event"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEAdjacentNoReplayNoResync"
        status: pass
    human_judgment: false
  - id: D5
    description: "Subscribing against an empty buffer with no Last-Event-ID opens live and blocks for future events without a spurious resync"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEEmptyBufferNoLastEventID"
        status: pass
    human_judgment: false
  - id: D6
    description: "Two concurrent subscribers both receive every event; a dry-run or a failed mutation produces no state-change event on either"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEBroadcast"
        status: pass
    human_judgment: false
  - id: D7
    description: "Any valid, non-expired key (regardless of scope) can open /v1/events; an unknown or expired key is rejected 401"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEAuth"
        status: pass
    human_judgment: false
  - id: D8
    description: "A narrowly-scoped (playback-only) key's open stream still receives an authoring-domain event (D-11 documented cross-scope exposure)"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSECrossScope"
        status: pass
    human_judgment: false
  - id: D9
    description: "Revoking an open stream's key closes that connection within one revocation-tick interval (T-07-12b)"
    requirement: "API-03"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSERevocationTick"
        status: pass
    human_judgment: false

# Metrics
duration: 50min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 8: Revisioned Global SSE Event Stream Summary

**GET /v1/events -- a bounded ring-buffer broadcaster keyed by show.State.Revision, with Last-Event-ID replay, a resync-on-overflow signal (never a silent gap), D-11/D-12 any-valid-key/all-events access semantics, and a periodic revocation tick that closes a revoked/expired key's open stream within one tick.**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-07-25
- **Tasks:** 2/2
- **Files modified:** 2 (both created)

## Accomplishments
- Built `internal/api/events.go`'s `eventBroadcaster`: a bounded, revision-keyed ring buffer (`EventRingBufferCapacity`, default 256, exported var for test tuning) plus an all-subscribers fan-out (D-11), registered as `GET /v1/events` via `huma/v2/sse.Register` (D-09) with a synthetic, non-CLI `RegisterOperation` route key (`"events watch"`) so it participates in `router.go`'s registration loop without colliding with `coverage_test.go`'s real-route coverage gate.
- Implemented D-10's full gap-recovery contract: an in-window `Last-Event-ID` replays exactly the missed events in order (no duplicates); an out-of-window id yields a single `resync` event (`{"reason":"buffer_overflow"}`) instructing a REST re-fetch, never a silent gap; `Last-Event-ID` equal to the latest known revision produces neither replay nor resync; an empty buffer with no header opens live with no spurious resync.
- Registered a post-mutation observer (`publishMutationEvent`, observer.go's 07-05 seam) that only publishes `Outcome == "success"` mutations (never `dry_run`/`failure`/`idempotent_replay`), keyed by `ResultingRevision` -- proven directly by `TestSSEBroadcast`'s dry-run + duplicate-name-failure interleaving.
- Implemented D-11/D-12 by omission: the handler never calls `RequireScope`, so any request that already passed `router.go`'s global `AuthMiddleware` (valid, non-expired key, any scope) may open the stream and observes every domain's events regardless of its own scope -- documented directly in the operation's OpenAPI `Description` (T-07-12's mitigation) and proven by `TestSSEAuth`/`TestSSECrossScope`.
- Added a per-connection revocation/expiry tick (`EventRevocationTickInterval`, default 5s, exported var): `apiKeyStillValid` re-checks the connecting key against `show.ListAPIKeys` every tick and closes the connection the first tick after revocation or expiry (T-07-12b), closing the standard SSE "revoke doesn't close an already-open stream" gap.
- Solved a real cross-file test-isolation hazard: `mutate_test.go`/`dryrun_test.go` both call `observer.go`'s `ResetMutationObserversForTesting()` in their own setup/cleanup, which wipes the *entire* global observer list with no restoration. A package-init-registered SSE observer would have been silently and permanently dropped partway through the shared `api_test` binary. Fixed by deferring registration to `api.NewServer`'s own operation-registration pass (`ensureEventStreamObserverRegistered`, idempotency-guarded) and adding `api.ResetEventStreamForTesting()` (mirroring `ResetMutationObserversForTesting`'s own naming/shape) so this file's tests always get a clean, correctly-armed broadcaster regardless of what ran before them in the same test binary.

## Task Commits

1. **Task 1 (ring-buffer/replay/resync) + Task 2 (access semantics/revocation tick)** - `82d1983` (feat) -- combined into one commit; see Deviations for why.

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md/REQUIREMENTS.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/api/events.go` - `domainEventPayload`/`resyncEventPayload`/`ringEvent`, `eventBroadcaster` (ring buffer + subscriber fan-out + reset), `eventStreamBroadcaster` singleton, `ensureEventStreamObserverRegistered`/`ResetEventStreamForTesting`, `domainFromRoute`, `publishMutationEvent`, `apiKeyStillValid`, `eventsInput`, `handleEventStream`, `registerEventsOperation`, `EventRingBufferCapacity`/`EventRevocationTickInterval` exported vars, `GET /v1/events` operation registration
- `internal/api/events_test.go` - `newEventsTestServer`, `sseFrame`/`sseClient` (real streaming-HTTP SSE test harness: `openEventStream`/`next`/`nextWithTimeout`), `decodeDomainPayload`, `TestSSEOrder`, `TestSSEReplay`, `TestSSEGapRecovery`, `TestSSEAdjacentNoReplayNoResync`, `TestSSEEmptyBufferNoLastEventID`, `TestSSEBroadcast`, `TestSSEAuth`, `TestSSECrossScope`, `TestSSERevocationTick`

## Decisions Made
- Single generic `"state"` SSE event name with a `Type` field in its JSON payload, instead of one Go type per domain event name -- see `tech-stack.patterns` for the full huma reflection-collision rationale.
- Package-level singleton broadcaster and observer-registration guard, not `*Server` fields -- kept this plan's changes fully contained to `events.go`/`events_test.go` without touching `server.go`, mirroring `mutate.go`'s own `mutationMutex` precedent.
- Gap-recovery boundary uses the provably-correct `lastID < oldest-1` condition (not the plan text's more literal `lastID < oldest`) -- see `key-decisions` for the full reasoning; this plan's own tests use unambiguous margins so the exact boundary choice isn't itself under test, but the implementation is documented to apply the stricter interpretation.
- Revocation re-check reuses `show.ListAPIKeys` (already-exported, hash-free) rather than adding a new by-KeyID lookup to `internal/show/apikeys.go`, keeping this plan's `files_modified` scope exactly as declared.

## Deviations from Plan

### Auto-fixed Issues

None -- no bugs found in dependency code, no blocking issues requiring a fix.

### Scope/Process Deviations

**1. [Process] Both tasks committed together, not as two separate commits**
- **Found during:** Planning the commit boundary before starting Task 1
- **Reason:** Both tasks declare an identical `files_modified` list (`internal/api/events.go`, `internal/api/events_test.go`), and Task 2's own `<read_first>` explicitly names Task 1's broadcaster/subscription lifecycle as what the revocation tick hooks into -- they are one coherent file's design, not two independently useful increments. Splitting them into a fake two-commit sequence (e.g. committing Task 1 without the revocation-tick's `apiKeyStillValid`/ticker wiring already present in the same `handleEventStream` function) would have required either a contrived partial-file diff or reverting/re-adding code across commits, adding process overhead with no verification or review benefit.
- **Impact:** None on correctness or test coverage -- every acceptance criterion from both tasks is proven by a passing test in the single commit. Documented here per this plan's own commit-protocol expectations.

**2. [Rule 2 - documented interpretation] Gap-recovery boundary computed as `lastID < oldest-1`, not the plan text's literal `lastID < oldest`**
- **Found during:** Task 1, implementing `eventBroadcaster.subscribe`
- **Issue:** PLAN.md's behavior text says a client is out-of-window when its `Last-Event-ID` is "older than the oldest buffered revision" (literally `lastID < oldest`). Since committed events are contiguous (+1 per successful mutation, because only `Outcome == "success"` mutations publish), a `lastID` exactly one less than the buffer's oldest retained revision (`oldest - 1`) has, by construction, missed nothing -- the very next event it needs (`oldest`) is still in the buffer. Applying the literal `lastID < oldest` condition would resync a client that in fact suffered no gap at all.
- **Fix:** Implemented the stricter, provably-correct `lastID < oldest-1` condition instead, with a doc comment explaining the contiguous-revision reasoning.
- **Files modified:** `internal/api/events.go` (`eventBroadcaster.subscribe`)
- **Verification:** `TestSSEGapRecovery` and `TestSSEReplay` both use ids with an unambiguous margin (not the exact `oldest-1` edge) so they pass identically under either interpretation; the stricter interpretation is documented in `key-decisions` and the function's own doc comment rather than left as an implicit, unexplained choice.
- **Committed in:** `82d1983`

---

**Total deviations:** 1 process note (commit boundary), 1 documented implementation-interpretation decision (gap-recovery boundary). No bugs auto-fixed in dependency code; no architectural changes required.
**Impact on plan:** None on scope or correctness -- every behavior/acceptance criterion in both tasks is met and proven by a passing test.

## Issues Encountered
- `mage testQuick` fails in this worktree with `GOLC_TEST_TOOLCHAIN_MISSING: ... run 'mage Bootstrap' first` -- confirmed pre-existing, identical to the environment limitation 07-04-SUMMARY.md/07-05-SUMMARY.md already documented (this worktree has not run `mage Bootstrap`); unrelated to any file this plan touches. `go build ./internal/...` and `go test ./internal/api/... -run 'TestSSE...' -race -v` (this plan's own stated verification commands, plus the full `internal/api` package suite) all ran directly and are fully green.
- `go build ./...` (whole-repo) fails on `cmd/golc-desktop/main.go:28: pattern all:frontend/dist: no matching files found` -- pre-existing, unrelated to this plan (the Wails frontend has not been built in this worktree); `go build ./internal/...` is unaffected and green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `GET /v1/events` is a stable, working seam: any future plan wiring additional mutating routes onto 07-05's pipeline (07-09's coverage-closure task) automatically gains SSE publication for free -- `publishMutationEvent` keys off `MutationEvent.Route`/`ResultingRevision`, with no per-route SSE wiring required.
- If a future plan wants genuinely distinct SSE event NAMES per domain (rather than this plan's single generic `"state"` name + payload `Type` field), that is an additive, compatible change to `registerEventsOperation`'s `eventTypeMap` -- documented directly in `events.go`'s own package doc comment.
- `EventRingBufferCapacity`/`EventRevocationTickInterval` are process-wide package vars (not per-`*Server` config); a later plan wanting these operator-configurable via the `api` config concern (D-06's pattern) would need to thread them through `Config`/`WithConfig` in `server.go` -- out of this plan's stated `files_modified` scope.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED
- FOUND: internal/api/events.go
- FOUND: internal/api/events_test.go
- FOUND: commit 82d1983
