---
phase: 06-wails-authoring-and-operator-surface
plan: 05
subsystem: ui
tags: [wails, go, react, zustand, artnet, safety, ipc]

# Dependency graph
requires:
  - phase: 06-02
    provides: "daemon-resident safety/master IPC routes (artnet safety blackout|stop-all|revoke-automation) this plan's SafetyService and hotkey.go both forward to"
  - phase: 06-04
    provides: "Wails scaffold, SafetyService/events.go stubs, App lifecycle, and the persistent SafetyCluster/LiveStatusBar mount points in App.tsx this plan fills"
provides:
  - "SafetyService.Blackout/StopReleaseAll/RevokeAutomation Wails bindings forwarding daemon safety routes with --source manual"
  - "SafetyService.FetchStatus + throttled StartStatusPush/StopStatusPush wired around App's own lifecycle in cmd/golc-desktop/main.go"
  - "Extended daemon statusPayload.Playback projection (active scene, enabled layers, BPM/bar, controlling source, output state) sourced from playback.Engine's new CurrentPlan/CurrentPosition/ActiveSceneName accessors"
  - "SafetyCluster.tsx: three 64px hold-to-confirm controls (D-14) wired to the SafetyService bindings"
  - "LiveStatusBar.tsx: PLAY-07 live status projection with explicit idle state, truncation+tooltip, and daemon-unreachable copy"
affects: [06-06, 06-07, 06-08]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SafetyService owns its own EventPusher instance (reusing events.go's throttle scaffold) rather than reaching into App's unexported events field, since app.go/hotkey.go/App.tsx are 06-04 stubs this plan must not modify -- cmd/golc-desktop/main.go composes App.OnStartup/OnShutdown with SafetyService.StartStatusPush/StopStatusPush via closures instead"
    - "frontend/src/lib/wailsBridge.ts centralizes all window.go/window.runtime access behind typed helper functions that degrade to an explicit offline/unavailable result rather than throwing -- every future Wave 3/4 component follows this one pattern"
    - "playback.Engine exposes lock-free CurrentPlan/CurrentPosition/ActiveSceneName accessors (mirroring CurrentFrame's own atomic.Pointer discipline) so the daemon status payload can read live scene/BPM/bar data without touching the tick loop"

key-files:
  created:
    - internal/wails/svc_safety_test.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/components/SafetyCluster/SafetyCluster.module.css
    - frontend/src/components/LiveStatusBar/LiveStatusBar.module.css
  modified:
    - internal/wails/svc_safety.go
    - internal/wails/events.go
    - internal/artnet/daemon.go
    - internal/artnet/daemon_test.go
    - internal/command/artnet.go
    - internal/playback/engine.go
    - internal/playback/engine_test.go
    - cmd/golc-desktop/main.go
    - frontend/src/components/SafetyCluster/SafetyCluster.tsx
    - frontend/src/components/LiveStatusBar/LiveStatusBar.tsx
    - frontend/src/store/store.ts
    - frontend/src/index.css

key-decisions:
  - "On-screen safety controls always send --on true (never toggle), mirroring hotkey.go's own always-activate convention exactly -- deactivation is out of this plan's scope, matching the plan's own checkpoint verification steps which never test UI-driven deactivation"
  - "Per-control 'active' indicator on SafetyCluster is best-effort: the daemon's single combined controllingSource/outputState vocabulary cannot distinguish Blackout from Stop/Release-All (both drive outputState='blackout' identically) -- documented as a known limitation in SafetyCluster.tsx rather than silently claimed as precise"
  - "FetchStatus's daemon JSON decode uses plain encoding/json, not strictjson.DecodeStrict -- the envelope intentionally declares only the 'playback' member it needs, and DecodeStrict's DisallowUnknownFields would reject the daemon's sibling frame/targets/universes/interface members"
  - "internal/command/artnet.go's artnetStatusPayload CLI mirror was updated to include the new Playback field -- without this fix, strictjson.DecodeStrict would reject every 'artnet status' CLI call once the daemon started emitting the new field (Rule 1 bug caught before it shipped)"

requirements-completed: []
# PLAY-06/07/08/09 are intentionally left Pending in REQUIREMENTS.md by this
# SUMMARY: Task 3 (checkpoint:human-verify, gate="blocking") has not yet
# been performed. The orchestrator/next executor session should mark these
# complete only after the human-verify checkpoint below is approved.

duration: ~90min (Tasks 1-2; Task 3 checkpoint pending)
completed: 2026-07-23
status: checkpoint-pending-verification
---

# Phase 06 Plan 05: SafetyService Bindings, PLAY-07 Status Push, Safety Cluster & Live Status Bar Summary

**On-screen hold-to-confirm Blackout/Revoke Automation/Stop-Release-All wired to the same daemon safety IPC routes as the OS-level hotkeys, plus a throttled PLAY-07 live status bar sourced from a newly extended daemon status payload**

## Performance

- **Duration:** ~90 min active execution (Tasks 1-2)
- **Started:** 2026-07-23 (this session)
- **Tasks:** 2 of 3 complete (Task 3 is a blocking human-verify checkpoint, not yet performed)
- **Files modified:** 16 (4 created, 12 modified)

## Accomplishments

- `SafetyService.Blackout/StopReleaseAll/RevokeAutomation` (`internal/wails/svc_safety.go`) dial+forward the daemon's `artnet safety ...` routes with `--source manual` -- the identical route+args shape `hotkey.go`'s OS-level callback already uses, giving the on-screen buttons and the OS-level hotkeys two independent triggers into the same daemon override state (RESEARCH.md Pitfall 1).
- Extended the daemon's `statusPayload` (`internal/artnet/daemon.go`) with a new `Playback` projection (active scene, enabled layers, BPM/bar position, controlling source, output state), sourced from three new lock-free `playback.Engine` accessors (`CurrentPlan`/`CurrentPosition`/`ActiveSceneName`, `internal/playback/engine.go`) plus the daemon's existing `safetyState`. The idle/no-active-plan case is handled explicitly (never a blank/zero-valued payload) -- proven by both a pure-transform unit test and a real-daemon integration test.
- `SafetyService.FetchStatus` decodes the daemon's extended status JSON into a JSON-safe `StatusSnapshot`, falling back to an explicit offline projection on any dial/decode failure -- never a blank/undefined result.
- `SafetyService` now owns its own `EventPusher` (reusing `events.go`'s existing throttle scaffold) and polls `FetchStatus` on `statusPollInterval`, staging each snapshot for a coalesced `status:update` `EventsEmit` -- wired into `cmd/golc-desktop/main.go`'s `OnStartup`/`OnShutdown` closures around `App`'s own lifecycle hooks, without modifying `app.go`/`hotkey.go` (per this plan's explicit interfaces constraint).
- `SafetyCluster.tsx`: three 64px hold-to-confirm controls (D-13/D-14/D-15) with the exact Copywriting Contract labels and status resting colors, a visible progress fill during the ~750ms hold window, instant cancel on early release, and no gating on daemon reachability.
- `LiveStatusBar.tsx`: renders all five PLAY-07 fields (scene, layers, BPM/bar, controlling source, output state) with an explicit idle state, ellipsis truncation + hover tooltips at fixed column widths, and the UI-SPEC daemon-unreachable copy while the safety cluster stays interactive.
- A new `frontend/src/lib/wailsBridge.ts` centralizes all `window.go`/`window.runtime` access behind typed, non-throwing helpers.

## Task Commits

Each task was committed atomically:

1. **Task 1: SafetyService bindings + throttled status push with PLAY-07 fields** - `7b70ebe` (feat)
2. **Task 2: SafetyCluster (hold-to-confirm) + LiveStatusBar components** - `a6a816b` (feat)
3. **Task 3: Verify on-screen safety cluster + live status bar behavior** - NOT YET PERFORMED (`checkpoint:human-verify`, `gate="blocking"`)

## Files Created/Modified

- `internal/wails/svc_safety.go` - `SafetyService.Blackout/StopReleaseAll/RevokeAutomation/FetchStatus`, `StartStatusPush/StopStatusPush`, `StatusSnapshot`
- `internal/wails/svc_safety_test.go` - route/args forwarding tests, idle/offline `StatusSnapshot` tests, status-push emit test
- `internal/wails/events.go` - `QueueStatus` typed to `StatusSnapshot`
- `internal/artnet/daemon.go` - `playbackStatusPayload`, `newPlaybackStatusPayload`, `snapshotEngine`, `enabledLayerNames`
- `internal/artnet/daemon_test.go` - daemon-level playback-status wiring test + pure-transform idle test
- `internal/command/artnet.go` - `artnetStatusPayload`/`artnetPlaybackStatus` CLI mirror updated to match the daemon's extended wire shape (Rule 1 fix, see Deviations)
- `internal/playback/engine.go` - `CurrentPlan`/`CurrentPosition`/`ActiveSceneName` lock-free accessors
- `internal/playback/engine_test.go` - accessor coverage incl. the defensive zero-value Engine case
- `cmd/golc-desktop/main.go` - `OnStartup`/`OnShutdown` closures composing `App`'s lifecycle with `SafetyService.StartStatusPush/StopStatusPush`
- `frontend/src/components/SafetyCluster/SafetyCluster.tsx` + `.module.css` - hold-to-confirm safety cluster
- `frontend/src/components/LiveStatusBar/LiveStatusBar.tsx` + `.module.css` - PLAY-07 live status bar
- `frontend/src/lib/wailsBridge.ts` - typed `window.go`/`window.runtime` access
- `frontend/src/store/store.ts` - new `status` slice (Zustand cache of Go-pushed snapshots)
- `frontend/src/index.css` - added the missing `--motion-snap: 0ms` brand token

## Decisions Made

- On-screen safety controls always send `--on true` (never a toggle), mirroring `hotkey.go`'s own always-activate convention -- deactivation via the UI is out of this plan's scope.
- Per-control "active" indicator in `SafetyCluster.tsx` is best-effort and documented as such: the daemon's combined `controllingSource`/`outputState` vocabulary cannot distinguish Blackout from Stop/Release-All.
- `FetchStatus`'s daemon JSON decode uses plain `encoding/json`, not `strictjson.DecodeStrict`, since the envelope intentionally declares only the `playback` member it needs.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed CLI `artnet status` decode regression from the extended statusPayload**
- **Found during:** Task 1 (extending the daemon's `statusPayload` with the new `Playback` field)
- **Issue:** `internal/command/artnet.go`'s `artnetStatusPayload` mirrors the daemon's wire shape field-for-field for `strictjson.DecodeStrict` (which rejects unknown JSON members). Adding `Playback` to the daemon's payload without updating this mirror would break every `artnet status` CLI invocation.
- **Fix:** Added a matching `Playback artnetPlaybackStatus` field (and the `artnetPlaybackStatus` mirror type) to `internal/command/artnet.go`, plus a `GOLC_ARTNET_PLAYBACK_STATUS` line in `renderArtnetStatusPlain` so the CLI's human-readable view surfaces the new data instead of silently dropping it.
- **Files modified:** internal/command/artnet.go
- **Verification:** `go test ./internal/command/...` passes; full-repo `go build ./...` succeeds.
- **Committed in:** 7b70ebe (Task 1 commit)

**2. [Rule 2 - Missing Critical] Added `playback.Engine` accessors needed to source the status payload**
- **Found during:** Task 1
- **Issue:** The plan's file list did not include `internal/playback/engine.go`, but the daemon has no existing way to read the active scene/BPM/bar position from `*playback.Engine` -- `CurrentFrame()` only exposes the resolved attribute values, not scene identity or musical position.
- **Fix:** Added `CurrentPlan()`, `CurrentPosition()`, and `ActiveSceneName()` -- lock-free accessors mirroring `CurrentFrame`'s own `atomic.Pointer` discipline -- plus a new `position atomic.Pointer[MusicalPosition]` field updated alongside `activeFrame` in both `NewEngine` and `tick`.
- **Files modified:** internal/playback/engine.go, internal/playback/engine_test.go
- **Verification:** `go test ./internal/playback/...` passes, including a defensive zero-value-Engine test proving the accessors never panic and report an explicit idle state.
- **Committed in:** 7b70ebe (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 bug fix, 1 missing-critical-functionality addition)
**Impact on plan:** Both were necessary for the plan's own stated goal (a working PLAY-07 status payload) and for CLI correctness. No scope creep beyond what Task 1 explicitly required.

## Issues Encountered

None beyond the two auto-fixed items above.

## User Setup Required

None - no external service configuration required.

## Checkpoint Verification

**Task 3 (`checkpoint:human-verify`, `gate="blocking"`) — NOT YET PERFORMED.**

This plan paused at Task 3 per the standard (non-auto-mode) checkpoint protocol. `.planning/config.json`'s `workflow.auto_advance` is `false` and no `_auto_chain_active` flag was set for this execution, so the checkpoint was not auto-approved.

**How to verify** (from the plan's own `<how-to-verify>`):
1. Run the desktop app (`go build -tags desktop,production ./cmd/golc-desktop/...` then launch `golc-desktop.exe`, per 06-04-SUMMARY.md's own build-tag note) with a daemon + an active show scene.
2. Confirm the live status bar shows active scene, enabled layers, BPM/bar, controlling source, and output state; switch a scene and confirm it updates.
3. Stop playback / clear the active scene and confirm the bar shows an explicit idle/stopped state (not blank).
4. Press and hold "Hold to Blackout" ~1s; confirm output blacks out (verify via a separate `artnet status`). Release early on another control and confirm it does NOT trigger.
5. Kill the daemon; confirm the unreachable copy appears AND the safety cluster is still interactive.
6. Give a scene a very long name; confirm the status bar truncates with a tooltip and its height does not grow.

**Resume-signal:** Type "approved" if all of the above behave as specified; otherwise describe issues.

## Next Phase Readiness

- Tasks 1-2 are fully committed and verified via automated tests (`go test ./internal/wails/... ./internal/artnet/... -run 'TestSafetyService|TestStatusPayload'` passes; `cd frontend && npm run build` passes) plus a full-repo `go build ./...`/`go vet ./...` pass.
- PLAY-06/07/08/09 remain Pending in REQUIREMENTS.md until Task 3's human-verify checkpoint is approved by a follow-up execution session.
- The `SafetyService`/`StatusSnapshot`/`wailsBridge.ts` surface this plan built is available for 06-06 (Operator Surface)/06-07/06-08 to extend with their own bindings and store slices, following the same established patterns.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

- FOUND: internal/wails/svc_safety.go
- FOUND: internal/wails/svc_safety_test.go
- FOUND: internal/wails/events.go
- FOUND: internal/artnet/daemon.go
- FOUND: internal/artnet/daemon_test.go
- FOUND: internal/command/artnet.go
- FOUND: internal/playback/engine.go
- FOUND: internal/playback/engine_test.go
- FOUND: cmd/golc-desktop/main.go
- FOUND: frontend/src/components/SafetyCluster/SafetyCluster.tsx
- FOUND: frontend/src/components/SafetyCluster/SafetyCluster.module.css
- FOUND: frontend/src/components/LiveStatusBar/LiveStatusBar.tsx
- FOUND: frontend/src/components/LiveStatusBar/LiveStatusBar.module.css
- FOUND: frontend/src/lib/wailsBridge.ts
- FOUND: frontend/src/store/store.ts
- FOUND: frontend/src/index.css
- FOUND commit: 7b70ebe
- FOUND commit: a6a816b
