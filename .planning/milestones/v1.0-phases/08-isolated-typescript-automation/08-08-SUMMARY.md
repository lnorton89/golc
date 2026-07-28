---
phase: 08-isolated-typescript-automation
plan: 08
subsystem: scripting
tags: [go, sse, event-bus, wails, audit-trail, typescript-automation]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "internal/script's session protocol/host/Run and CallOutcome/RunOutcome shapes (08-05); host-side capability enforcement and TerminationReason (08-06); internal/api's Phase-7 eventBroadcaster/MutationEvent/observer pattern; internal/wails' EventPusher throttle scaffold and ScriptService CRUD (08-04)"
provides:
  - "internal/script/events.go: script.log/outcome/status/terminal event bus (ring buffer, Seq ordering, replay/resync, redaction-at-publish) + PublishScriptEvent/SubscribeScriptEvents/ResetScriptEventsForTesting"
  - "internal/script/session.go: a guaranteed script.terminal event on every Run exit path (computeTerminalEvent/terminalStatusReason, outermost defer), a script.status event at run start, script.log per captured line, script.outcome per CallOutcome (also firing the audit seam)"
  - "internal/api/observer.go: PublishMutationEvent, the exported non-HTTP seam into fireMutationObservers"
  - "internal/api/events.go: scriptEventPayload + PublishScriptLifecycleEvent, a \"script\" SSE type on the existing global /v1/events stream (ringEvent.Payload generalized to any)"
  - "internal/wails/events.go: EventPusher.scriptEvents ordered staging slot + QueueScriptEvent, bounded with a synthetic script.gap overflow signal"
  - "internal/wails/svc_script.go: ScriptService.StartScriptEventStream/StopScriptEventStream forwarding script.SubscribeScriptEvents into the webview under \"script:event\""
affects: [08-09, 08-10]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/script/events.go's eventBus mirrors internal/api/events.go's eventBroadcaster structurally (ring buffer, monotonic Seq, replay/resync subscriber semantics) but is a deliberate, documented parallel implementation, not a shared import -- internal/api must never import internal/script (pinned by go list -deps)"
    - "internal/api's ringEvent.Payload generalized from the single domainEventPayload type to any, so one broadcaster/ring now carries both domainEventPayload/resyncEventPayload (Phase 7) and the new scriptEventPayload (08-08) on the same global stream, distinguished by huma/v2/sse.Register's type-to-event-name reflection"
    - "internal/wails EventPusher's ordered-slice staging convention (first established by WR-02's midiFeedback per-key fix) applied a second time for scriptEvents -- log lines/outcomes are discrete facts, never a coalescible snapshot"

key-files:
  created:
    - internal/script/events.go
    - internal/script/events_test.go
    - internal/script/session_audit_test.go
    - internal/api/observer_test.go
  modified:
    - internal/script/session.go
    - internal/api/observer.go
    - internal/api/events.go
    - internal/api/events_test.go
    - internal/wails/events.go
    - internal/wails/events_test.go
    - internal/wails/svc_script.go
    - internal/wails/svc_script_test.go

key-decisions:
  - "The guaranteed script.terminal event is computed by a pure function (computeTerminalEvent/terminalStatusReason) taking Run's own named return values (result, runErr), registered as the OUTERMOST defer in Host.Run -- this makes the seven-termination-cause guarantee directly unit-testable without spawning a real Deno process for each cause, while still being the exact code path production runs."
  - "internal/api/observer.go's audit-seam wiring (api.RegisterAuditObserver) was deliberately NOT added to internal/command/scriptrun.go/scriptstop.go in this plan -- neither file is in this plan's files_modified scope, and an unconditional registration call there would double-register an observer (duplicate audit rows) across multiple script runs within one long-lived process (e.g. golc-desktop.exe). Logged as a known gap in deferred-items.md, mirroring 07-07's own seam-built-but-unwired handoff to 07-09."
  - "ScriptService kept its existing 3-argument constructor (NewScriptService(pipeName, root, showPath)) and self-constructs its own *EventPusher, mirroring SafetyService/MidiService's established pattern exactly, rather than threading a shared *EventPusher through the constructor as the plan's action text suggested -- this avoids a signature change (and touching cmd/golc-desktop/main.go and every existing NewScriptService call site) for no functional gain, since every sibling service already owns its own EventPusher instance."

requirements-completed: [SCRP-05]

coverage:
  - id: D1
    description: "A bounded, ordered, redaction-at-publish script event bus (script.log/outcome/status/terminal) with replay-within-window and resync-on-overflow subscriber semantics, mirroring Phase 7's proven SSE pattern."
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/script/events_test.go#TestScriptEventBusPublishAssignsStrictlyIncreasingSeq"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestScriptEventBusReconnectReplaysWithinWindow"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestScriptEventBusReconnectScrolledOutResyncsNoPartialReplay"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestScriptEventBusOverflowTriggersResyncAtMeasuredCapacity"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestScriptEventBusPublishRedactsMessageAndReason"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every run publishes exactly one script.terminal event on every exit path (success, failure, Stopped by user, deadline, rate, scope, Job-Object resource kill) -- the flagged SCRP-05 operative edge, mitigated by an outermost defer."
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/script/events_test.go#TestComputeTerminalEventEverySevenTerminationCauses"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestPublishScriptEventTerminalPublishesExactlyOnePerCause"
        status: pass
      - kind: unit
        ref: "internal/script/events_test.go#TestComputeTerminalEventEarlyFailureBeforeDispatchStillPublishes"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every SDK call produces both a live script.outcome event AND an audit_log row in the Phase 7 audit trail via the new api.PublishMutationEvent seam, with Source=\"script\"."
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/script/session_audit_test.go#TestSDKCallProducesBothScriptOutcomeEventAndAuditRow"
        status: pass
      - kind: unit
        ref: "internal/api/observer_test.go#TestPublishMutationEventCarriesNonHTTPSource"
        status: pass
    human_judgment: false
  - id: D4
    description: "Script lifecycle (start, terminal) appears as a new \"script\" SSE type on the SAME existing global /v1/events stream, not a second endpoint."
    requirement: "SCRP-05"
    verification:
      - kind: integration
        ref: "internal/api/events_test.go#TestSSEScriptLifecycleEvent"
        status: pass
    human_judgment: false
  - id: D5
    description: "Script log lines and command outcomes reach the desktop webview individually and in Seq order (never coalesced), with an explicit synthetic gap event on staging-buffer overflow."
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "internal/wails/events_test.go#TestQueueScriptEventStagesFiveDistinctEventsAndEmitsAllInSeqOrder"
        status: pass
      - kind: unit
        ref: "internal/wails/events_test.go#TestQueueScriptEventOverflowEmitsGapEventBeforeSurvivingEvents"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_script_test.go#TestScriptEventStreamForwardsPublishedEventsToEmit"
        status: pass
    human_judgment: false

# Metrics
duration: ~90min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 8: Script Event Stream and Audit-Pipeline Seam Summary

**A live, ordered, non-coalescing script event stream (log/outcome/status/terminal) built on Phase 7's ring-buffer SSE pattern, a guaranteed terminal event on every run-exit path, and `api.PublishMutationEvent` — the exported seam that puts every script-issued SDK call into the same Phase 7 audit trail an HTTP mutation writes to.**

## Performance

- **Duration:** ~90 min
- **Tasks:** 3 completed
- **Files modified:** 12 (4 created, 8 modified)

## Accomplishments
- `internal/script/events.go` implements a bounded, Seq-ordered, redaction-at-publish script event bus — structurally mirroring `internal/api/events.go`'s proven `eventBroadcaster` pattern (ring buffer, replay-within-window, resync-on-overflow) as a deliberate, documented parallel implementation rather than a shared import, since `internal/api` must never import `internal/script`.
- `internal/script/session.go` wires the bus into the run lifecycle: a `script.status` event at run start, a `script.log` event per captured stdout/stderr line, a `script.outcome` event per `CallOutcome`, and — the plan's flagged SCRP-05 operative edge — an outermost `defer` that guarantees exactly one `script.terminal` event on every exit path, computed by a pure, directly-unit-testable function (`computeTerminalEvent`/`terminalStatusReason`) rather than duplicated inline logic.
- `internal/api/observer.go` adds `PublishMutationEvent`, the exported seam a non-HTTP control surface (script/wails/cli) uses to reach the same audit/SSE pipeline the HTTP path uses, keeping `fireMutationObservers` unexported so `mutate.go`'s ordering guarantee is untouched.
- `internal/api/events.go` generalizes `ringEvent.Payload` to `any` and adds `scriptEventPayload`/`PublishScriptLifecycleEvent`, registering a new `"script"` SSE type on the existing global `/v1/events` stream (no second endpoint) for script lifecycle (start/terminal) transitions.
- `internal/script/session.go` fires `api.PublishMutationEvent(...)` with `Source:"script"` immediately after every `CallOutcome` — proven, in the same test, to produce both a live `script.outcome` event AND an `audit_log` row (`internal/script/session_audit_test.go`).
- `internal/wails/events.go`'s `EventPusher` gains an ordered `scriptEvents` staging slot (never a map/single-value `latest` slot — the exact WR-02 `midiFeedback` failure this deliberately avoids repeating) with a bounded `maxStagedScriptEvents` overflow path that emits one synthetic `script.gap` event ahead of the surviving events.
- `internal/wails/svc_script.go`'s `ScriptService.StartScriptEventStream`/`StopScriptEventStream` subscribe to `script.SubscribeScriptEvents` and forward every event through `QueueScriptEvent`, mirroring `SafetyService.StartStatusPush`/`StopStatusPush`'s own self-contained `EventPusher` lifecycle.

## Task Commits

1. **Task 1: Script event bus following the Phase 7 broadcaster pattern**
   - `914bc24` (feat) - internal/script/events.go, events_test.go, session.go wiring (log/outcome/status/terminal events)
2. **Task 2: Audit-pipeline seam and SSE type tag**
   - `abd1d82` (feat) - internal/api/observer.go (PublishMutationEvent), events.go (scriptEventPayload/PublishScriptLifecycleEvent), observer_test.go, events_test.go, internal/script/session_audit_test.go
3. **Task 3: Ordered, non-coalescing script event delivery to the desktop webview**
   - `2d3677c` (feat) - internal/wails/events.go (QueueScriptEvent/scriptEvents), events_test.go, svc_script.go (StartScriptEventStream/StopScriptEventStream), svc_script_test.go

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_Note: this plan's task-level `<action>` text for Task 2 also required editing `internal/script/session.go` (the audit-seam call site), which landed in the Task 1 commit alongside the `script.outcome` event publish since both concerns share the same `publishCallOutcome` call site in `runDispatchIO` — the diff could not be cleanly split without duplicating that call site; the Task 2 commit covers every `internal/api` file plus the standalone audit-integration test._

## Files Created/Modified
- `internal/script/events.go` - `ScriptEventKind`/`ScriptEvent`, `ScriptEventRingCapacity`, `eventBus` (publish/Subscribe/reset), `PublishScriptEvent`/`SubscribeScriptEvents`/`ResetScriptEventsForTesting`
- `internal/script/events_test.go` - bus ordering/replay/resync/redaction/reset coverage, the seven-termination-cause table (`TestComputeTerminalEventEverySevenTerminationCauses`), and a `runDispatchIO`-level log/outcome-event test
- `internal/script/session_audit_test.go` - proves one dispatched SDK call produces both a `script.outcome` event and an `audit_log` row in the same run
- `internal/script/session.go` - `mutationOutcomeFor`/`mutationStatusCodeFor`/`publishCallOutcome`, `terminalStatusReason`/`computeTerminalEvent`, `Run`'s outermost terminal-event defer and run-start status event, per-line log-event publishing in `runDispatchIO`
- `internal/api/observer.go` - `PublishMutationEvent`
- `internal/api/observer_test.go` - `PublishMutationEvent` registration-order/no-observers/non-HTTP-Source coverage
- `internal/api/events.go` - `ringEvent.Payload` generalized to `any`, `scriptEventPayload`, `PublishScriptLifecycleEvent`, `"script"` registered in the SSE type map
- `internal/api/events_test.go` - `TestSSEScriptLifecycleEvent` (real `httptest.Server` SSE connection)
- `internal/wails/events.go` - `maxStagedScriptEvents`, `ScriptEventView`, `EventPusher.scriptEvents`/`pendingScriptEventGap`, `QueueScriptEvent`, `flush`'s script-event/gap emission
- `internal/wails/events_test.go` - Seq-order/overflow-gap/no-gap coverage for `QueueScriptEvent`
- `internal/wails/svc_script.go` - `ScriptService.events` field, `toScriptEventView`, `StartScriptEventStream`/`StopScriptEventStream`/`forwardScriptEvents`
- `internal/wails/svc_script_test.go` - end-to-end publish-to-emit forwarding test, before-Start no-op test

## Decisions Made
- The guaranteed terminal event is computed by a pure function (`computeTerminalEvent`) rather than inline logic in `Run`'s defer, so the seven-termination-cause guarantee is directly unit-testable without a real Deno process per cause — production and test call the identical function.
- `api.RegisterAuditObserver` production wiring for `internal/command/scriptrun.go`/`scriptstop.go` was deliberately left out of this plan's scope (see Deviations and `deferred-items.md`'s `## 08-08` section) — an unconditional registration call there would risk duplicate audit rows across multiple runs within one long-lived process without its own idempotent-registration design.
- `ScriptService` self-constructs its own `*EventPusher` (matching `SafetyService`/`MidiService`'s established pattern) instead of threading a shared instance through the constructor — no signature change, no `cmd/golc-desktop/main.go` wiring needed.
- `ringEvent.Payload` was generalized from `domainEventPayload` to `any` so one broadcaster/ring buffer carries multiple SSE payload shapes (`domainEventPayload`, `resyncEventPayload`, and the new `scriptEventPayload`) on the same global stream, resolved to distinct SSE `"event:"` names entirely by `huma/v2/sse.Register`'s existing type-to-name reflection — no second stream, no second broadcaster.

## Deviations from Plan

### Auto-fixed Issues

None — every change is additive/new-file within this plan's own scope; no pre-existing behavior needed correction.

---

**Total deviations:** 0 auto-fixed. One scoped design choice (documented above under Decisions Made): `api.RegisterAuditObserver` production wiring for the CLI/GUI script-run entry points was deliberately deferred rather than risked as a naive, potentially-duplicating fix outside this plan's `files_modified` list — logged in `deferred-items.md` for a future gap-closure plan, mirroring the 07-07 → 07-09 precedent.
**Impact on plan:** None of this plan's own acceptance criteria are affected — the audit seam itself (`api.PublishMutationEvent`) is fully implemented and tested; only its production *registration* for the script-run CLI/GUI processes remains open, the same class of gap 07-07 intentionally left for 07-09 to close in the identical wave-collision-avoidance spirit.

## Issues Encountered
- Same five pre-existing toolchain-bootstrap failures as 08-01/08-03/08-05/08-06/08-07 (`TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance`), unrelated to this plan's changes — logged in `deferred-items.md`'s `## 08-08` section. `go test ./internal/script/... ./internal/api/... ./internal/wails/... -count=1` is fully green; `go test ./internal/command/...` is green except these five pre-existing failures.
- `go build ./...` still fails only on `cmd/golc-desktop` (`pattern all:frontend/dist: no matching files found`), the same pre-existing condition 08-01 first logged (no frontend build has run in this worktree); `go build ./internal/... ./cmd/golc-project/...` is clean.
- Unlike every prior plan in this phase, **no new tests in this plan needed Deno-gating** — every `<behavior>` bullet is provable against `io.Pipe()`-based fakes, direct function calls, or a real `httptest.Server` SSE connection, so `go test ./internal/script/... ./internal/api/... ./internal/wails/... -count=1` has zero skips in this worktree despite the partial/unverified `.tools/toolchains/deno/` install 08-05/08-06/08-07 already logged.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- 08-09 (debugger CDP bridge) and 08-10 (desktop debug panel UI) both build directly on this plan's `script.ScriptEvent`/`ScriptEventKind` shapes, the `internal/wails` `"script:event"` delivery contract, and the `"script"` SSE type on `/v1/events` — all now stable and tested.
- Known gap for a future plan (or 08-10's own desktop-wiring work) to close: production registration of `api.RegisterAuditObserver` for the process that actually executes `script run`/`script stop` (CLI or the Wails GUI's in-process execution) — see `deferred-items.md`'s `## 08-08` section for the full rationale and the idempotent-registration design constraint any fix must satisfy.
- No blockers to 08-09/08-10 proceeding: every unit/integration-testable behavior in this plan is green on this machine, with zero Deno-gated skips.

## Self-Check: PASSED

- FOUND: internal/script/events.go
- FOUND: internal/script/events_test.go
- FOUND: internal/script/session_audit_test.go
- FOUND: internal/api/observer_test.go
- FOUND: internal/script/session.go (modified)
- FOUND: internal/api/observer.go (modified)
- FOUND: internal/api/events.go (modified)
- FOUND: internal/api/events_test.go (modified)
- FOUND: internal/wails/events.go (modified)
- FOUND: internal/wails/events_test.go (modified)
- FOUND: internal/wails/svc_script.go (modified)
- FOUND: internal/wails/svc_script_test.go (modified)
- FOUND: .planning/phases/08-isolated-typescript-automation/08-08-SUMMARY.md
- FOUND: .planning/phases/08-isolated-typescript-automation/deferred-items.md (08-08 section appended)
- FOUND commit: 914bc24 (feat: script event bus with guaranteed terminal event)
- FOUND commit: abd1d82 (feat: audit-pipeline seam and script SSE type tag)
- FOUND commit: 2d3677c (feat: ordered, non-coalescing script event delivery to the webview)
- `go build ./internal/... ./cmd/golc-project/...`: PASS
- `go test ./internal/script/... ./internal/api/... ./internal/wails/... -count=1`: PASS (zero skips)
- `go test ./internal/command/... -count=1`: PASS except 5 pre-existing unrelated failures (logged in deferred-items.md)
- `go test ./internal/scriptsdk/... ./internal/show/... -count=1`: PASS
- `go test ./internal/api/... -run TestCapabilityCoverage -count=1`: PASS (no new command routes added by this plan)
- `go test ./internal/command/... -run TestEveryDeclaredRouteIsClassified -count=1`: PASS

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
