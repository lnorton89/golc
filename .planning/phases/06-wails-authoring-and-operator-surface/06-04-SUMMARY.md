---
phase: 06-wails-authoring-and-operator-surface
plan: 04
subsystem: ui
tags: [wails, go, react, zustand, hotkey, ipc, daemon-supervision]

# Dependency graph
requires:
  - phase: 06-01
    provides: Wails/frontend toolchain decisions and STACK.md version pins consumed by this plan's scaffold
  - phase: 06-02
    provides: daemon-side artnet safety IPC routes (blackout / stop-all / revoke-automation) that hotkey.go calls directly
provides:
  - Wails v2 desktop scaffold (wails.json, cmd/golc-desktop entrypoint, frontend/ React+Zustand shell)
  - Go host lifecycle (internal/wails/app.go): daemon reachability check + supervised spawn-on-unreachable, ordered start/reverse-stop
  - OS-level safety-cluster hotkeys (internal/wails/hotkey.go) via golang.design/x/hotkey, calling ipc.Forward directly (no JS-mediated path)
  - Throttled event-push scaffold (internal/wails/events.go) for Wave 3/4 feature plans to fill
  - Registered-but-stubbed binding services (SafetyService, PlaybackService, SurfaceService, MidiService)
  - Persistent global React layout (App.tsx) with safety-cluster bar, live status bar, and five region-component stubs
affects: [06-05, 06-06, 06-07, 06-08]

# Tech tracking
tech-stack:
  added: [github.com/wailsapp/wails/v2 v2.13.0, golang.design/x/hotkey v0.6.1, react 19, zustand 5, vite, "@vitejs/plugin-react 6.0.4"]
  patterns:
    - "OS-level global hotkeys registered from the Go host (never JS keydown) call ipc.Forward directly for safety-critical actions"
    - "Wails App.OnStartup performs ipc.Dial reachability check; on GOLC_ARTNET_DAEMON_UNREACHABLE, spawns a supervised golc-project artnet serve child (mirrors WIN-02 supervised-helper pattern)"
    - "Zustand store is a cache of Go-pushed snapshots only, never authoritative state"
    - "Wave 3/4 plans fill the CONTENTS of pre-stubbed component/service files only; they never modify App.tsx or the binding-aggregation structure"

key-files:
  created:
    - internal/wails/app.go
    - internal/wails/hotkey.go
    - internal/wails/events.go
    - internal/wails/svc_safety.go
    - internal/wails/svc_playback.go
    - internal/wails/svc_surface.go
    - internal/wails/svc_midi.go
    - internal/wails/app_test.go
    - wails.json
    - cmd/golc-desktop/main.go
    - frontend/package.json
    - frontend/vite.config.ts
    - frontend/index.html
    - frontend/src/main.tsx
    - frontend/src/App.tsx
    - frontend/src/store/store.ts
    - frontend/src/components/SafetyCluster/SafetyCluster.tsx
    - frontend/src/components/LiveStatusBar/LiveStatusBar.tsx
    - frontend/src/components/PlaybackControls/PlaybackControls.tsx
    - frontend/src/components/OperatorSurface/OperatorSurface.tsx
    - frontend/src/components/MidiPanel/MidiPanel.tsx
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "golang.design/x/hotkey v0.6.1 and wails/v2 v2.13.0 verified legitimate (pkg.go.dev + CGo-free Windows impl) at the Task 0 blocking-human checkpoint before install"
  - "@vitejs/plugin-react bumped to 6.0.4 (beyond STACK.md's original pin) to satisfy vite 8's peer range"
  - "vite.config.ts redirects build output to cmd/golc-desktop/frontend/dist since go:embed cannot reference a directory outside its own package tree"

patterns-established:
  - "Safety-cluster hotkey callback -> ipc.Forward: OS keypress reaches the daemon safety route with zero JS involvement, satisfying D-16 even with a hung/unresponsive webview"
  - "Daemon supervision: OnStartup treats GOLC_ARTNET_DAEMON_UNREACHABLE as a spawn trigger, not a fatal error, then retries Dial"

requirements-completed: []
# PLAY-01 and PLAY-09 are NOT marked complete by this plan. This plan lays the
# shell/hotkey/daemon-supervision foundation both requirements need, but:
# - PLAY-01 ("complete playback workflow through on-screen controls") is only
#   satisfied once 06-06 (Operator Surface) and 06-05 (Playback Controls) fill
#   their region stubs with real controls.
# - PLAY-09 ("Blackout remains a separate local priority control...") has its
#   OS-level mechanism proven working end-to-end in this plan's checkpoint
#   verification, but the requirement is tracked complete alongside the full
#   safety-cluster wiring once all three hotkeys are exercised in later
#   integration (kept Pending here to avoid a premature traceability claim).

coverage:
  - id: D1
    description: "Go host lifecycle (App.OnStartup/OnShutdown) with daemon reachability check via ipc.Dial and supervised spawn of golc-project artnet serve when unreachable"
    verification:
      - kind: unit
        ref: "internal/wails/app_test.go#TestAppStartup"
        status: pass
      - kind: manual_procedural
        ref: "Task 3 checkpoint: launched golc-desktop.exe with no daemon pre-running; confirmed artnet status unreachable before launch, reachable+ticking (frame=on-cadence) after launch"
        status: pass
    human_judgment: false
  - id: D2
    description: "Three OS-level global safety hotkeys (Blackout / Stop-Release-All / Revoke Automation) registered via golang.design/x/hotkey, each calling ipc.Forward directly with no JS-mediated path; registration failures surfaced, never silent"
    verification:
      - kind: unit
        ref: "internal/wails/app_test.go#TestHotkeyRegisterSurfaced"
        status: pass
      - kind: manual_procedural
        ref: "Task 3 checkpoint: focus moved to a desktop icon (window verifiably unfocused), Ctrl+Alt+Shift+B sent; a temporary diagnostic log line confirmed the Go callback fired and dispatched 'blackout -> artnet safety blackout' the instant the physical hotkey was pressed, then the diagnostic line was reverted (git diff confirmed byte-identical to committed hotkey.go)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Wails scaffold + cmd/golc-desktop entrypoint + persistent React global layout (safety-cluster bar, live status bar, five region stubs with loading backstop)"
    verification:
      - kind: unit
        ref: "cd frontend && npm install && npm run build"
        status: pass
      - kind: manual_procedural
        ref: "Task 3 checkpoint: screenshot confirmed window title 'GOLC', safety-cluster bar with all three Hold-to controls, and the three Wave-3/4 region-stub placeholders ('Loading playback controls...', 'Loading operator surfaces...', 'Loading MIDI mappings...')"
        status: pass
    human_judgment: false
  - id: D4
    description: "Daemon-side safety IPC route wiring (built in 06-02) confirmed to accept the exact route+payload shape hotkey.go's callbacks use"
    verification:
      - kind: manual_procedural
        ref: "Task 3 checkpoint: manual golc-project.exe artnet safety blackout --on true --source manual invocation confirmed accepted (GOLC_ARTNET_SAFETY_BLACKOUT: on=true)"
        status: pass
    human_judgment: false

duration: ~35min active execution (Tasks 1-2) + asynchronous checkpoint verification pause
completed: 2026-07-23
status: complete
---

# Phase 06 Plan 04: Wails Scaffold, Go Host, and OS-Level Safety Hotkeys Summary

**Wails v2 desktop shell with a Go-host-owned daemon-supervision lifecycle and three OS-level safety hotkeys (golang.design/x/hotkey) that call the Art-Net daemon's safety IPC routes directly, bypassing the webview entirely.**

## Performance

- **Duration:** ~35 min active execution across Tasks 1-2 (04:10:44 -> 04:13:48 local); Task 3 checkpoint verification was performed asynchronously by the orchestrator with explicit user permission to drive the desktop
- **Started:** 2026-07-23T11:10:44Z (Task 1 commit)
- **Completed:** 2026-07-23T19:12:03Z (this SUMMARY)
- **Tasks:** 3 (Task 0 package-legitimacy checkpoint pre-approved; Task 1 and Task 2 auto tasks; Task 3 human-verify checkpoint approved)
- **Files modified:** 27 (10 in Task 1, 17 in Task 2)

## Accomplishments

- Go host (`internal/wails/app.go`) implements `OnStartup`/`OnShutdown` with daemon reachability check via `ipc.Dial(ipc.PipeName)`, spawning a supervised `golc-project artnet serve` child on `GOLC_ARTNET_DAEMON_UNREACHABLE` and retrying — verified end-to-end in the Task 3 checkpoint (no daemon running beforehand, daemon reachable and ticking after launch)
- Three OS-level global safety hotkeys registered via `golang.design/x/hotkey` (Blackout / Stop-Release-All / Revoke Automation) with callbacks that call `ipc.Forward` directly from the Go host — never through a JS-mediated keydown path — resolving RESEARCH.md's open Assumption A3 (hotkey vs. Wails message-loop conflict) with a positive result: the callback fired correctly while the webview was unfocused
- Every `hotkey.Register()` failure is surfaced (log line + frontend-renderable field), never silent, satisfying the DoS mitigation in the threat register
- Wails scaffold (`wails.json`, `cmd/golc-desktop/main.go`) binds `App` plus all four feature service stubs (`SafetyService`, `PlaybackService`, `SurfaceService`, `MidiService`)
- React shell (`App.tsx`) composes a persistent safety-cluster region and live status bar as fixed chrome, with five region-component stubs (SafetyCluster, LiveStatusBar, PlaybackControls, OperatorSurface, MidiPanel) each rendering a labeled placeholder + loading skeleton for Wave 3/4 plans to fill
- Zustand store (`store.ts`) established as a pure cache of Go-pushed snapshots, never authoritative
- Package legitimacy of `golang.design/x/hotkey` v0.6.1 and `wails/v2` v2.13.0 verified at the Task 0 blocking-human checkpoint before install (pkg.go.dev existence, CGo-free Windows implementation, `go mod verify`)

## Task Commits

Each task was committed atomically:

1. **Task 0: Package legitimacy gate — Wails + golang.design/x/hotkey install** - checkpoint approved (no code commit; gated the `go get` performed at the start of Task 1)
2. **Task 1: Go host — Wails lifecycle, daemon supervision, OS-level safety hotkeys, binding stubs** - `e062259` (feat)
3. **Task 2: Wails scaffold + entrypoint + React shell with region-component stubs** - `2f5fb70` (feat)
4. **Task 3: Verify Wails shell launch, daemon supervision, and OS-level Blackout hotkey** - checkpoint approved (verification-only, no code commit)

**Plan metadata:** (this commit) - `docs(06-04): complete Wails scaffold plan`

_Note: Task 3 is a `checkpoint:human-verify` gate — verification only, no code changes._

## Files Created/Modified

- `internal/wails/app.go` - App struct, OnStartup/OnShutdown, daemon reachability + supervised spawn, ordered start/reverse-stop
- `internal/wails/hotkey.go` - three OS-level global hotkeys (D-16), direct ipc.Forward callbacks, surfaced registration failures
- `internal/wails/events.go` - throttled pushStatus scaffold (25ms cadence, independent of 40Hz worker tick)
- `internal/wails/svc_safety.go`, `svc_playback.go`, `svc_surface.go`, `svc_midi.go` - empty-but-registered Wails-bound service stubs for Wave 3/4
- `internal/wails/app_test.go` - daemon-supervision reachability test + hotkey-registration-failure-surfaced test
- `go.mod`, `go.sum` - add wailsapp/wails/v2 v2.13.0, golang.design/x/hotkey v0.6.1
- `wails.json` - Wails v2 project manifest
- `cmd/golc-desktop/main.go` - wails.Run entrypoint binding App + all four feature services, embeds frontend/dist
- `frontend/package.json`, `vite.config.ts`, `index.html`, `src/main.tsx` - React 19 + Zustand 5 scaffold; vite.config.ts redirects build output into cmd/golc-desktop/frontend/dist for go:embed
- `frontend/src/App.tsx` - persistent global layout: safety-cluster region + live status bar + feature regions
- `frontend/src/store/store.ts` - Zustand cache of Go-pushed snapshots
- `frontend/src/components/{SafetyCluster,LiveStatusBar,PlaybackControls,OperatorSurface,MidiPanel}/*.tsx` - region stubs with loading backstop skeletons

## Decisions Made

- `golang.design/x/hotkey` v0.6.1 and `wails/v2` v2.13.0 confirmed legitimate and CGo-free on Windows before install (Task 0 blocking-human checkpoint)
- `@vitejs/plugin-react` bumped to 6.0.4 beyond STACK.md's original pin to satisfy vite 8's peer dependency range
- `vite.config.ts` redirects the frontend build output to `cmd/golc-desktop/frontend/dist` because Go's `go:embed` cannot reference a directory outside its own package tree
- A plain `go build` on the desktop binary fails at runtime with an explicit Wails error dialog; the correct invocation requires `-tags desktop,production`. This is expected Wails behavior (documented by the framework), not a plan defect, and required no code fix

## Deviations from Plan

None - plan executed exactly as written. The `-tags desktop,production` build requirement encountered during Task 3 verification is standard Wails framework behavior (not something the plan needed to account for as a fix) and is noted above for future reference.

## Issues Encountered

None. The Task 3 checkpoint required a temporary diagnostic `log.Printf` line inside `HotkeyManager.listen()` to make the otherwise-silent-on-success hotkey callback observable during manual verification. This line was fully reverted after verification; `git diff internal/wails/hotkey.go` against the committed version was confirmed byte-identical, and this worktree's `git status --short` is clean.

## User Setup Required

None - no external service configuration required.

## Checkpoint Verification

**Task 3 (`checkpoint:human-verify`, `gate="blocking"`) — APPROVED.**

Performed directly by the orchestrator with explicit user permission to drive the desktop. Evidence recorded:

1. Built `golc-desktop.exe` from this worktree with `go build -tags desktop,production` (Wails-required build tags; a plain `go build` fails at runtime with an explicit Wails error dialog — expected framework behavior, not a defect).
2. Created a minimal synthetic test show via the CLI (`show save` bootstrap -> `scene create` + `scene activate` -> `playback bpm set 120`), since no `.golc` fixture existed anywhere in the repo yet (expected — first GUI-touching phase).
3. Launched `golc-desktop.exe` with `GOLC_DESKTOP_SHOW`/`GOLC_DESKTOP_INTERFACE` set (interface 1 = Loopback). Console confirmed `[WebView2] Environment created successfully`. Confirmed via a separate `golc-project.exe artnet status` invocation that the daemon was NOT running before launch, then WAS reachable and ticking (`frame=on-cadence`) after launch — the `ensureDaemon` supervised spawn-on-unreachable path working correctly end-to-end.
4. Confirmed visually (screenshot) the window renders the title "GOLC", the safety-cluster bar with "Hold to Blackout" / "Hold to Revoke Automation" / "Hold to Stop / Release All", and the three Wave-3/4 region stubs ("Loading playback controls...", "Loading operator surfaces...", "Loading MIDI mappings...") — expected, since those regions are filled by later plans (06-05/06-06/06-07/06-08).
5. Moved focus to a desktop icon (confirmed via a file-info tooltip appearing) so the GOLC window was verifiably NOT focused, then sent the physical key combo Ctrl+Alt+Shift+B.
6. To make the otherwise-silent-on-success hotkey callback observable, a temporary diagnostic `log.Printf` line was added inside `HotkeyManager.listen()` in `internal/wails/hotkey.go`, rebuilt, and steps 3-5 repeated. The exact line `GOLC_WAILS_HOTKEY_FIRED_TEMP_DEBUG: blackout -> artnet safety blackout` appeared in the console the instant the physical hotkey was pressed — proving the OS-level `golang.design/x/hotkey` registration fires its Go callback correctly while the webview is unfocused, with no conflict against Wails' own Win32 message loop. This directly resolves RESEARCH.md's Assumption A3 (previously an open question).
7. The diagnostic line was fully reverted (`git diff internal/wails/hotkey.go` confirmed byte-identical to the committed version) and `go build ./...` re-run clean.
8. Separately, `golc-project.exe artnet safety blackout --on true --source manual` was invoked manually via CLI and confirmed accepted by the daemon (`GOLC_ARTNET_SAFETY_BLACKOUT: on=true`) — the exact IPC route+payload shape `hotkey.go`'s callback uses, confirming the daemon-side handler (built in 06-02) is correctly wired.
9. No `GOLC_WAILS_HOTKEY_REGISTER_FAILED` line appeared at any point — all three hotkey registrations succeeded silently, as expected on success.
10. Test processes (`golc-desktop.exe`, `golc-project.exe`) were killed afterward; no leftover processes or show files were left in the repo (all test artifacts were in the OS temp scratch directory, outside the repo).

**Resume-signal received:** "approved"

## Next Phase Readiness

- The binding-aggregation and component-stub structure (SafetyCluster, LiveStatusBar, PlaybackControls, OperatorSurface, MidiPanel + Safety/Playback/Surface/MidiService) is in place for Wave 3/4 plans (06-05 through 06-08) to fill disjoint files in parallel.
- Daemon supervision and OS-level safety hotkeys are proven end-to-end; no known blockers for downstream plans.
- PLAY-01 and PLAY-09 remain Pending in REQUIREMENTS.md — this plan lays their foundation but does not complete either requirement (on-screen controls land in 06-05/06-06; full safety-cluster requirement completion is tracked once all three hotkeys and their UI feedback are wired end-to-end).

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

All 16 claimed files (10 from Task 1, 6 representative from Task 2) confirmed present on disk; both task commit hashes (`e062259`, `2f5fb70`) confirmed present in git log.
