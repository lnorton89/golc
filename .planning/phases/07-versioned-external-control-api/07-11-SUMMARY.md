---
phase: 07-versioned-external-control-api
plan: 11
subsystem: api
tags: [sse, events, replay, gap-recovery, correctness, cr-01, openapi]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-06 (batch), 07-08 (SSE event stream), 07-09 (OpenAPI generation/COMPATIBILITY.md)"
provides:
  - "ringEvent.Seq -- strictly monotonic per-process SSE event id, decoupled from show.State.Revision"
  - "eventBroadcaster.subscribe's new lastID > latest resync branch for daemon-restart clients"
  - "Regenerated docs/api/openapi.json describing the new id semantics, drift-clean"
  - "docs/api/COMPATIBILITY.md SSE section describing the per-process sequence and batch multi-event case"
affects: [api, sse, events, batch]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SSE transport-ordering key (Seq) kept fully independent of domain data (Revision) carried in the same event -- lets N events legitimately share one committed revision while staying individually addressable"

key-files:
  created: []
  modified:
    - internal/api/events.go
    - internal/api/events_test.go
    - docs/api/openapi.json
    - docs/api/COMPATIBILITY.md

key-decisions:
  - "ringEvent.Seq assigned exactly once, inside eventBroadcaster.publish, under the existing broadcaster mutex -- no new atomic/lock needed since publish already serializes buffer appends"
  - "subscribe's lastID >= latest branch split into two: lastID == latest keeps the existing caught-up-no-op behavior, lastID > latest now resyncs (closes the daemon-restart silent-gap case, since the sequence is per-process and resets to 0 on restart while show.State.Revision persists)"
  - "Revision stays in domainEventPayload's body only, redocumented as domain data re-fetchable via REST resources -- not touched as a transport key anywhere"

requirements-completed: [API-03]

coverage:
  - id: D1
    description: "A reconnecting client that saw only the first frame of a multi-sub-request batch and reconnects with that frame's id receives the batch's remaining events instead of being told it is already caught up (CR-01)."
    requirement: "API-03"
    verification:
      - kind: unit
        ref: "internal/api/events_test.go#TestSSEBatchMultiSubRequestReconnectDeliversRemainingEvents"
        status: pass
    human_judgment: false
  - id: D2
    description: "SSE event ids are strictly monotonic and independent of the domain revision across mixed batch/single-mutation traffic."
    requirement: "API-03"
    verification:
      - kind: unit
        ref: "internal/api/events_test.go#TestSSEEventIDsStrictlyMonotonicAcrossBatchAndSingleMutation"
        status: pass
    human_judgment: false
  - id: D3
    description: "A Last-Event-ID greater than any id this process has issued (daemon-restart case) yields an explicit resync, never silence."
    requirement: "API-03"
    verification:
      - kind: unit
        ref: "internal/api/events_test.go#TestSSEFutureLastEventIDResyncs"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every pre-existing SSE behavior (monotonic live order, in-window replay, out-of-window resync, adjacency edge, empty-buffer edge, broadcast, auth, cross-scope, revocation tick) still holds unmodified."
    requirement: "API-03"
    verification:
      - kind: unit
        ref: "internal/api/events_test.go#TestSSEOrder,TestSSEReplay,TestSSEGapRecovery,TestSSEAdjacentNoReplayNoResync,TestSSEEmptyBufferNoLastEventID,TestSSEBroadcast,TestSSEAuth,TestSSECrossScope,TestSSERevocationTick"
        status: pass
    human_judgment: false
  - id: D5
    description: "Published OpenAPI contract and COMPATIBILITY.md describe the new per-process event-id semantics, with no drift between the committed contract and the code that serves it."
    requirement: "API-03"
    verification:
      - kind: unit
        ref: "go test ./internal/api/... -run 'TestOpenAPIDrift|TestOpenAPIDeterministic|TestOpenAPIDocumentsEveryOperation'"
        status: pass
      - kind: other
        ref: "mage generatecheck"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 11: SSE Event Sequence Decoupled From Show Revision Summary

**Strictly monotonic per-process SSE event sequence (`ringEvent.Seq`) replaces `show.State.Revision` as the `id:` line and replay/resync key, closing CR-01's multi-sub-request batch reconnect data-loss gap.**

## Performance

- **Duration:** 35 min
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- A client that receives only the first frame of a multi-sub-request `/v1/batch` (which commits one revision but emits N separately-addressable events) and reconnects with that frame's id now receives the remaining events, in order, instead of being told "already caught up" (07-VERIFICATION.md's live-reproduced CR-01 gap, now a committed regression test).
- Introduced `ringEvent.Seq` and `eventBroadcaster.nextSeq`, assigned exactly once inside `publish` under the broadcaster's existing mutex -- the SSE `id:` line and every replay/resync comparison are now keyed on this per-process sequence, never `Revision`.
- Added a new `lastID > latest` resync branch to `subscribe`: a client presenting an id this process never issued (the daemon-restart case, newly reachable because the sequence resets to 0 on restart while `show.State.Revision` persists with the show) gets an explicit resync instead of a silent false "caught up".
- Regenerated `docs/api/openapi.json` and rewrote `docs/api/COMPATIBILITY.md`'s SSE section to describe the id as a per-process sequence, the batch multi-event case, and the previous-daemon-run resync case; `mage generatecheck` reports no drift.

## Task Commits

Each task was committed atomically (TDD: test -> feat, plus one docs commit):

1. **Task 1 RED: batch-reconnect, monotonic-id, and future-id-resync regression tests** - `8c00a0f` (test)
2. **Task 1 GREEN: decouple SSE event id from show revision with a per-process sequence** - `fc849c3` (feat)
3. **Task 2: Regenerate the published contract and document the new event-id semantics** - `f1286c9` (docs)

**Plan metadata:** committed in this SUMMARY's own final metadata commit (see execute-phase orchestrator).

## Files Created/Modified
- `internal/api/events.go` - `ringEvent.Seq`/`eventBroadcaster.nextSeq` added; `publish` is the single Seq assigner; `subscribe` re-keyed onto Seq with a new daemon-restart resync branch; `reset()` zeroes `nextSeq`; `handleEventStream` emits `Seq` as the SSE id in both the replay and live loops; doc tags/operation Description rewritten to describe the new semantics.
- `internal/api/events_test.go` - Three new tests reproducing CR-01, pinning strict cross-traffic monotonicity, and pinning the future-id resync case; all nine pre-existing `TestSSE*` tests unmodified and green.
- `docs/api/openapi.json` - Regenerated via `mage generate`; only the two changed doc-tag descriptions differ (SSE revision field, Last-Event-ID header); `DO NOT EDIT` generated marker intact.
- `docs/api/COMPATIBILITY.md` - SSE section rewritten to describe the per-process event sequence, the batch multi-event case, and the previous-daemon-run resync case; all other sections (versioning policy, deprecation window, typed-error table, non-SSE examples) untouched.

## Decisions Made
- Kept the sequence counter under the broadcaster's existing `mu` rather than introducing a separate atomic -- `publish` already serializes buffer appends, so assignment and append stay one indivisible step with no new synchronization primitive.
- Split the prior single `lastID >= latest` branch into `lastID == latest` (unchanged caught-up behavior) and `lastID > latest` (new resync) rather than collapsing them, since the two cases now have genuinely different correctness implications once the sequence is per-process instead of persisted.
- Left `Revision` on `ringEvent` (unused as a comparison key) so future debugging/logging can still see which show revision an event belonged to without decoding `Payload`.

## Deviations from Plan

None - plan executed exactly as written, including the TDD RED/GREEN gate sequence prescribed by the plan's own acceptance criteria (confirmed the three new tests fail against the pre-change revision-keyed implementation before implementing the fix).

## Issues Encountered
- `mage testquick` and `go vet ./...` (repo-wide) fail in this isolated worktree because the pinned Go toolchain is not bootstrapped here (`GOLC_TEST_TOOLCHAIN_MISSING: ... run 'mage Bootstrap' first`) and `cmd/golc-desktop`'s `//go:embed all:frontend/dist` has no built frontend to embed. Both are pre-existing environment/bootstrap gaps unrelated to this plan's files (internal/api, docs/api) -- confirmed by running the scoped equivalents directly: `go test ./internal/api/... -race` (green), `go test ./internal/api/... ./internal/show/...` (green), and `go vet ./internal/...` (clean, no findings). `internal/command`'s pre-existing toolchain-dependent tests (`TestBuildRouteCompilesTheProductionRepository`, `TestScopeGreenSubprocess`, etc.) fail identically and are out of this plan's `files_modified` scope.

## Next Phase Readiness
- Verification gap 2 (API-03 / 07-REVIEW.md CR-01) is closed: the phase's documented D-10 "never a silently missing gap" guarantee now holds for the multi-sub-request batch path, proven by a committed, permanently-running regression test.
- No blockers for the remaining 07-versioned-external-control-api gap-closure plans (07-12..07-14).

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*
