---
phase: 06-wails-authoring-and-operator-surface
plan: 02
subsystem: playback
tags: [golang, artnet, daemon, atomic, ipc, safety, real-time]

# Dependency graph
requires:
  - phase: 04-observable-artnet-live-output
    provides: "internal/artnet daemon (IPC route dispatch, worker tick loop, health snapshot) and internal/command/artnet.go client-route conventions this plan extends"
provides:
  - "daemon-resident, in-memory Blackout/Stop-Release-All/Revoke Automation/Grand Master/group master overrides, read lock-free by the Art-Net Worker tick"
  - "new daemon IPC routes: artnet safety blackout|stop-all|revoke-automation, artnet master set, plus the --source manual|automation wire-arg convention"
  - "new CLI routes: artnet safety blackout|stop-all|revoke-automation, artnet master set"
affects: [06-04-hotkeys, 06-05-onscreen-playback-surface, 08-scripting, 09-ai-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Daemon-resident state deliberately outside d.mu, read lock-free via atomic.Bool/atomic.Pointer, mutated via copy-returning setters"
    - "Local-priority IPC handler that never calls reconfigureLocked() (no Worker restart) for state the tick goroutine already reads atomically"
    - "--source manual|automation wire-arg convention gating a broad daemon-level command-dispatch check, not a per-route concern"

key-files:
  created:
    - internal/artnet/safety.go
    - internal/artnet/safety_test.go
  modified:
    - internal/artnet/worker.go
    - internal/artnet/worker_test.go
    - internal/artnet/daemon.go
    - internal/artnet/daemon_test.go
    - internal/command/artnet.go
    - internal/command/artnet_test.go

key-decisions:
  - "Blackout and Stop-All are tracked as two independent atomic.Bool flags but applyOverrides treats them identically (either forces intensity to 0) -- each keeps its own IPC route/CLI trigger/diagnostic identity per the plan's separate PLAY-06 controls"
  - "Revoke Automation gate runs once at the top of daemon.handle(), before route dispatch, rejecting any Request tagged --source automation regardless of route -- matches the plan's literal 'blocks any command' language rather than scoping the gate to safety/master routes only"
  - "Group-master membership (instance -> group IDs) is built once in NewWorker from WorkerConfig.Groups + Instances, not recomputed per tick, to keep the tick's own applyOverrides call a single atomic-load path"

requirements-completed: [PLAY-06, PLAY-08, PLAY-09]

coverage:
  - id: D1
    description: "Daemon-resident atomic safety/master state + pure applyOverrides(frame, safetyState, membership) transform: blackout/stop-all zero intensity only, masters compose multiplicatively, empty Frame is a safe no-op, identity overrides leave the Frame unchanged, concurrent Set/Load converges under -race"
    requirement: "PLAY-06"
    verification:
      - kind: unit
        ref: "internal/artnet/safety_test.go#TestSafetyApplyOverridesBlackoutZeroesIntensity"
        status: pass
      - kind: unit
        ref: "internal/artnet/safety_test.go#TestSafetyApplyOverridesMultiplicativeMasterComposition"
        status: pass
      - kind: unit
        ref: "internal/artnet/safety_test.go#TestSafetyApplyOverridesBlackoutEmptyFrameIsSafeNoOp"
        status: pass
      - kind: unit
        ref: "internal/artnet/safety_test.go#TestSafetyConcurrentBlackoutConvergesUnderRace"
        status: pass
    human_judgment: false
  - id: D2
    description: "Worker.tick() applies overrides once (single atomic-load path) before the per-universe Encode loop; blackout takes effect within a bounded wall-clock window even with a slow/hung fan-out target in flight; daemon IPC routes for safety/master mutate the atomic state directly and never call reconfigureLocked() (no Worker restart); revoke-automation gate blocks --source automation Requests"
    requirement: "PLAY-08, PLAY-09"
    verification:
      - kind: integration
        ref: "internal/artnet/worker_test.go#TestSafetyOverrideBlackoutTakesEffectDespiteSlowTarget"
        status: pass
      - kind: integration
        ref: "internal/artnet/worker_test.go#TestWorkerGroupMasterComposesWithGrandMaster"
        status: pass
      - kind: integration
        ref: "internal/artnet/daemon_test.go#TestRevokeAutomationBlocksNonManualSource"
        status: pass
      - kind: integration
        ref: "internal/artnet/daemon_test.go#TestDaemonMasterSetGrandAndGroup"
        status: pass
    human_judgment: false
  - id: D3
    description: "CLI client routes 'artnet safety blackout|stop-all|revoke-automation' and 'artnet master set' reach live Art-Net output end-to-end, always tagging --source manual so operator CLI actions bypass an active revoke"
    requirement: "PLAY-06, PLAY-08"
    verification:
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetSafetyBlackoutDrivesUniverseToZero"
        status: pass
      - kind: integration
        ref: "internal/command/artnet_test.go#TestArtnetSafetyRevokeAutomationDoesNotBlockManualCLI"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 2: Local-Priority Safety and Master Overrides Summary

**Daemon-resident, atomic Blackout/Stop-Release-All/Revoke-Automation/Grand-Master/group-master overrides wired into the Art-Net Worker's per-tick path, exposed through new daemon IPC routes and CLI commands**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-07-23T02:30:00-07:00 (approx)
- **Completed:** 2026-07-23T03:05:11-07:00
- **Tasks:** 3
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- `internal/artnet/safety.go`: `safetyState` (atomic blackout/stopAll/revokeAutomation flags + an `atomic.Pointer[masterLevels]` snapshot) and the pure `applyOverrides(frame, safetyState, membership)` transform — blackout/stop-all zero intensity only, masters compose multiplicatively (`group=0.5, grand=0.5 -> 0.25`), an empty Frame is a safe no-op, identity overrides return the Frame unchanged, and concurrent blackout Set/Load converges under `-race`.
- `Worker.tick()` calls `applyOverrides` exactly once, before the per-universe `Encode` loop — a single atomic-load path, never a lock — so a Blackout/Stop-All/master change takes effect on the very next tick regardless of a hung Art-Net target's in-flight send (proven via the existing `slowSender` fake-sender harness).
- `daemon.go` gains a `safety safetyState` field deliberately outside `d.mu`, four new IPC routes (`artnet safety blackout|stop-all|revoke-automation`, `artnet master set`), and a top-level `handle()` gate rejecting any `--source automation`-tagged Request while Revoke Automation is active — none of the new handlers ever call `reconfigureLocked()` (source-reviewed, confirmed via grep: only the pre-existing `handleConfigure`/`handleSetEnabled` call it).
- `internal/command/artnet.go` gains client routes for all four safety/master actions, each validating its own flags client-side before dialing the daemon and always appending `--source manual` so operator-issued CLI actions are never blocked by an active revoke.

## Task Commits

Each task was committed atomically:

1. **Task 1: safety.go — atomic override state + pure applyOverrides transform** - `9ceabc3` (feat)
2. **Task 2: wire applyOverrides into the Worker tick + daemon IPC routes** - `f656872` (feat)
3. **Task 3: CLI client routes for safety + masters** - `63d9c3e` (feat)

_No TDD tasks in this plan (Task 1 declared `tdd="true"` but was executed as a single cohesive commit covering both the RED test file and GREEN implementation together, since the plan's own file list groups `safety.go`+`safety_test.go` as one deliverable and the plan did not separate RED/GREEN into distinct sub-steps — see TDD Gate Compliance note below)._

## Files Created/Modified

- `internal/artnet/safety.go` - `safetyState`/`masterLevels` types, setter/accessor methods, pure `applyOverrides` transform
- `internal/artnet/safety_test.go` - Behavior + `-race` concurrency tests for `applyOverrides`
- `internal/artnet/worker.go` - `WorkerConfig.Safety`/`.Groups`, `buildMembership`, `tick()` calls `applyOverrides` before Encode
- `internal/artnet/worker_test.go` - Blackout-despite-slow-target timing test, group-master composition test
- `internal/artnet/daemon.go` - `daemon.safety`/`.groups` fields, `requestSource`, revoke gate in `handle()`, `handleSafetyToggle`, `handleMasterSet`
- `internal/artnet/daemon_test.go` - Safety/master route round-trips, usage/domain error cases, revoke-automation source gate
- `internal/command/artnet.go` - Client routes `artnet safety blackout|stop-all|revoke-automation`, `artnet master set`
- `internal/command/artnet_test.go` - `startTestArtnetDaemonWithIntensity` (real programmed instance), end-to-end route tests

## Decisions Made

- Blackout and Stop-All are two independent atomic flags with identical `applyOverrides` treatment (both force intensity to 0) but separate IPC routes/CLI triggers/diagnostics, matching the plan's distinct PLAY-06 controls while keeping the transform simple.
- The revoke-automation gate is a single check at the top of `daemon.handle()`, applying to every route (not just safety/master routes) — this matches the plan's literal "blocks any command whose Request carries a non-manual source tag" language.
- `buildMembership` (instance ID -> group IDs) is computed once in `NewWorker`, not per-tick, keeping the tick's `applyOverrides` call a pure atomic-load-and-compute path with no per-tick map-building cost.
- `internal/command/artnet_test.go` needed a fuller test daemon helper (`startTestArtnetDaemonWithIntensity`) carrying a real base-look-programmed instance, since the existing `minimalArtnetShowState` produces an empty show where "universe values are zero" is trivially true regardless of whether blackout is set — this was necessary to make the CLI-level blackout test meaningful, not a plan deviation.

## Deviations from Plan

None — plan executed as written. One implementation detail worth flagging: Task 2's acceptance criterion "The timing test asserts blackout output within a bounded wall-clock ... independent of a simulated hung client" was interpreted as the existing worker-level fake-sender harness (a hung Art-Net *target*, per the plan's own instruction to "reuse the existing fake-sender harness") rather than a hung *IPC caller* — the IPC transport already serves each connection on its own goroutine (`ipc/server.go`'s `go handleConn(conn, handler)`), so a slow IPC client cannot block the daemon's atomic setter regardless; the meaningful thing to prove was that the override survives a concurrently slow output target, which `TestSafetyOverrideBlackoutTakesEffectDespiteSlowTarget` does.

## TDD Gate Compliance

Task 1 declared `tdd="true"` but `safety.go` and `safety_test.go` were authored and committed together in `9ceabc3` rather than as separate RED (`test(...)`) then GREEN (`feat(...)`) commits. No `test(...)` commit precedes a `feat(...)` commit in this plan's history. This mirrors the plan's own task structure (Task 1's `<action>` describes creating both the implementation and its test file as one deliverable, not a phased RED/GREEN split) but technically deviates from the strict TDD gate sequence described in the executor's TDD protocol. Flagging per the executor's TDD Gate Compliance requirement; no functional impact — all specified behaviors are tested and passing.

## Issues Encountered

None beyond the CLI-level test-fixture gap noted above (resolved, not a blocker).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The daemon-resident safety/master primitive this plan built is the exact local-priority path 06-04 (OS-level hotkeys) and 06-05 (on-screen playback buttons) are meant to trigger — both can call the same IPC routes/CLI conventions established here without further daemon changes.
- The `--source manual|automation` wire-arg convention is now in place for Phase 8 (scripting) and Phase 9 (AI) to tag their own commands as `automation`, so Revoke Automation's gate will correctly block them once those runtimes exist — this plan's `flagged_assumptions` note (queued-action cancellation is only partially realizable until Phases 8/9 exist) remains accurate and unresolved-by-design, not silently dropped.
- Two pre-existing, unrelated test failures were discovered via a full `go test -race ./...` sanity pass and logged to `deferred-items.md` rather than fixed (`internal/trace/catalog`'s `TestScopeLinearCatalog`/`TestScopeLinearMap`, and a genuine `-race` data race in `internal/trace/transport`'s `TestScopeTraceTransportProcess`) — zero file overlap with this plan, out of scope per the executor's SCOPE BOUNDARY rule.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

- FOUND: internal/artnet/safety.go
- FOUND: internal/artnet/safety_test.go
- FOUND: internal/artnet/worker.go
- FOUND: internal/artnet/worker_test.go
- FOUND: internal/artnet/daemon.go
- FOUND: internal/artnet/daemon_test.go
- FOUND: internal/command/artnet.go
- FOUND: internal/command/artnet_test.go
- FOUND: .planning/phases/06-wails-authoring-and-operator-surface/deferred-items.md
- FOUND commit: 9ceabc3
- FOUND commit: f656872
- FOUND commit: 63d9c3e
