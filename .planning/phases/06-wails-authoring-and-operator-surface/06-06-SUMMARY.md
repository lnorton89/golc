---
phase: 06-wails-authoring-and-operator-surface
plan: 06
subsystem: ui
tags: [wails, go, react, playback, keyboard-workflow, midi-adjacent]

# Dependency graph
requires:
  - phase: 06-04
    provides: PlaybackService binding stub, PlaybackControls/KeyboardShortcuts stub components, Wails scaffold
provides:
  - internal/wails/svc_playback.go filled with real PlaybackService methods (SwitchScene/SetLayerEnabled/SetBPM/TapTempo/Evaluate/GetState)
  - frontend/src/components/PlaybackControls/PlaybackControls.tsx -- on-screen controls for the complete playback workflow (PLAY-01), plus a shared `dispatch` action-function object
  - frontend/src/hooks/useKeyboardWorkflow.ts -- window-scoped in-webview keyboard workflow for the same actions (PLAY-02), deliberately not global hotkeys
  - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx -- documented, grouped, scrollable shortcut reference panel
affects: [06-08-midi-learn, 06-VALIDATION]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "PlaybackService binds one Go method per playback action, each building the exact command.Request{Route, Args, Root} the matching CLI route in internal/command/playback.go or scene.go expects and executing it through a freshly built default command registry -- the identical show.Load-mutate-show.Save path a CLI invocation takes, no second mutation implementation for the GUI"
    - "SetLayerEnabled reads the target layer's current Ref via a read-only show.Load immediately before its mutating registry call, so an enable/disable toggle never discards a previously assigned base-look/color-theme/chase/motion reference"
    - "GetState is a read-only projection (no existing CLI route lists scenes/layers) that PlaybackControls polls to render scenes/layers/BPM for the on-screen selector"
    - "A single exported `dispatch` object of action functions in PlaybackControls.tsx is the one source of truth both on-screen controls and useKeyboardWorkflow call, so PLAY-01 and PLAY-02 can never drift out of sync; tap tempo shares one accumulating buffer across mouse and keyboard taps"
    - "useKeyboardWorkflow is an ordinary window-scoped capture-phase keydown listener (in-webview), deliberately NOT golang.design/x/hotkey -- that OS-level global-hotkey path is reserved for the three safety-cluster controls in hotkey.go per 06-RESEARCH.md Pitfall 4"
    - "PLAYBACK_SHORTCUTS is the single documented source of truth both useKeyboardWorkflow and KeyboardShortcuts.tsx read, keeping the reference panel and the actual key bindings from drifting apart"

key-files:
  created:
    - internal/wails/svc_playback_test.go
    - frontend/src/hooks/useKeyboardWorkflow.ts
  modified:
    - internal/wails/svc_playback.go
    - cmd/golc-desktop/main.go
    - frontend/src/components/PlaybackControls/PlaybackControls.tsx
    - frontend/src/components/PlaybackControls/PlaybackControls.module.css
    - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx
    - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.module.css

key-decisions:
  - "Added GetState (a read-only projection of every scene's layers/active flag plus show-wide BPM) beyond the plan's literal bpm/switch/layer/evaluate action list, since no existing CLI route lists scenes/layers and an on-screen scene/layer selector is unusable without one (Rule 2 deviation)"
  - "NewPlaybackService's constructor signature was extended to take showPath/root parameters since every real method needs to show.Load/show.Save the ShowState; cmd/golc-desktop/main.go's construction call site was updated to match (Rule 3 deviation -- blocking)"
  - "Exported a single `dispatch` object of action functions from PlaybackControls.tsx as the one source of truth both the on-screen controls and useKeyboardWorkflow call, rather than duplicating action logic in the hook, so the two surfaces can never drift out of sync"
  - "KeyboardShortcuts is mounted from PlaybackControls.tsx via a toggle button, since App.tsx's layout/mount points are never edited per 06-04's contract"

patterns-established:
  - "PLAYBACK_SHORTCUTS as a single declared source of truth consumed by both the keyboard-handling hook and the documentation panel that describes it"

requirements-completed: [PLAY-01, PLAY-02]

coverage:
  - id: D1
    description: "PlaybackService bindings (SwitchScene/SetLayerEnabled/SetBPM/TapTempo/Evaluate/GetState) build the exact route/args the matching CLI route expects and execute through the default command registry; SetLayerEnabled preserves Ref across a toggle; bad arguments return the registry's own diagnostic, never a panic"
    requirement: "PLAY-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceEnumeratesEveryPlaybackAction"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSwitchScene"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSetLayerEnabledPreservesRefAcrossToggle"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSetBPM"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceTapTempo"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceEvaluate"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceGetState"
        status: pass
    human_judgment: false
  - id: D2
    description: "Bad-argument bindings (unknown scene, invalid BPM, fewer than two taps, evaluate with no active scene) surface the command registry's own diagnostic, never a panic"
    requirement: "PLAY-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSwitchSceneUnknownSceneReturnsDiagnosticNotPanic"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSetLayerEnabledUnknownSceneReturnsDiagnosticNotPanic"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceSetBPMInvalidValueReturnsDiagnosticNotPanic"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceTapTempoFewerThanTwoTapsReturnsDiagnosticNotPanic"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_playback_test.go#TestPlaybackServiceEvaluateNoActiveSceneReturnsDiagnosticNotPanic"
        status: pass
    human_judgment: false
  - id: D3
    description: "PlaybackControls exposes an on-screen control for every enumerated playback action (PLAY-01); useKeyboardWorkflow maps a documented key to every action via window-scoped keydown (not global hotkeys) calling the same dispatch handlers as the on-screen controls (PLAY-02); KeyboardShortcuts lists all shortcuts grouped and scrolling once they exceed one screen; frontend build succeeds"
    requirement: "PLAY-02"
    verification:
      - kind: automated_ui
        ref: "cd frontend && npm run build (tsc --noEmit && vite build) -- exit 0"
        status: pass
    human_judgment: true
    rationale: "Whether every playback action is actually reachable and behaves identically on screen and by keyboard against a live running show, and whether playback keys are truly window-scoped rather than global (unlike the 06-04 safety hotkeys), requires a human running the real golc-desktop app -- exactly what this plan's Task 3 checkpoint (type=checkpoint:human-verify, gate=blocking) specifies. Per workflow.human_verify_mode=end-of-phase (.planning/config.json), this worktree-isolated executor defers that live verification to the phase's end-of-phase UAT pass rather than halting mid-flight; see 'Checkpoint Verification' below for the exact steps to run."

duration: ~15min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 6: Playback Controls, Keyboard Workflow, and Shortcut Reference Summary

**PlaybackService (Go) executes the complete playback workflow (scene switch, layer toggle, BPM set/tap, evaluate) via the existing command registry; PlaybackControls (React) exposes it on screen while useKeyboardWorkflow mirrors the identical actions through window-scoped keydown handling, with KeyboardShortcuts documenting every binding.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-23
- **Tasks:** 2 automatable tasks executed (Task 3 is a `checkpoint:human-verify` gate deferred to end-of-phase UAT, see below)
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- `internal/wails/svc_playback.go`: `PlaybackService` filled with `SwitchScene`, `SetLayerEnabled`, `SetBPM`, `TapTempo`, `Evaluate` -- each builds the exact route/args the matching CLI route in `internal/command/playback.go` or `scene.go` expects and executes it in-process through a freshly built default command registry, the same `show.Load`-mutate-`show.Save` path a CLI invocation of the same route takes. No new playback authority is introduced.
- `SetLayerEnabled` reads the target layer's current `Ref` via a read-only `show.Load` immediately before its mutating registry call, so an enable/disable toggle never discards a previously assigned base-look/color-theme/chase/motion reference (scene.go's WR-03 only merges `Selection`, never `Ref`, when a selector flag is omitted).
- `GetState` added as a read-only projection of every scene's layers/active flag plus show-wide BPM, since no existing CLI route lists scenes/layers and an on-screen selector needs one.
- `PlaybackControls.tsx` (+ module.css): on-screen controls for the complete workflow -- scene selector/switch, per-layer enable/disable toggles, numeric BPM entry + tap-tempo button, and a transport/evaluate preview. BPM/bar readouts use JetBrains Mono (UI-SPEC Typography). Polls `PlaybackService.GetState()` to render scenes/layers/BPM, and exports a `dispatch` object of action functions as the single source of truth for both surfaces.
- `frontend/src/hooks/useKeyboardWorkflow.ts`: a window-scoped (in-webview) capture-phase keydown listener for the complete playback workflow -- ordinary DOM handling, deliberately not `golang.design/x/hotkey` (that OS-level global-hotkey path is reserved for the three safety-cluster controls in `hotkey.go` per 06-RESEARCH.md Pitfall 4). Declares `PLAYBACK_SHORTCUTS` as the single documented source of truth this hook and `KeyboardShortcuts.tsx` both read.
- `KeyboardShortcuts.tsx` (+ module.css): the documented shortcut reference panel, grouped by category, scrolling within a fixed-height area once it exceeds one screen (UI-SPEC overflow backstop). Mounted from `PlaybackControls.tsx` via a toggle button (App.tsx's layout/mount points are never edited).

## Task Commits

Each task was committed atomically:

1. **Task 1: PlaybackService bindings over the existing command registry** - `0f15208` (feat)
2. **Task 2: On-screen playback controls + in-webview keyboard workflow + shortcut reference** - `94a3802` (feat)

_Note: Task 3 is a `checkpoint:human-verify` gate (type=checkpoint:human-verify, gate=blocking) -- deferred to end-of-phase UAT per `workflow.human_verify_mode=end-of-phase`; no code change, see "Checkpoint Verification" below._

## Files Created/Modified

- `internal/wails/svc_playback.go` - `PlaybackService`: SwitchScene/SetLayerEnabled/SetBPM/TapTempo/Evaluate/GetState
- `internal/wails/svc_playback_test.go` - route/args shape coverage for every enumerated playback action, Ref-preservation across a toggle, bad-argument diagnostics
- `cmd/golc-desktop/main.go` - `NewPlaybackService` call site updated to pass showPath/root
- `frontend/src/components/PlaybackControls/PlaybackControls.tsx` - on-screen controls, `GetState` polling, shared `dispatch` action functions
- `frontend/src/components/PlaybackControls/PlaybackControls.module.css` - styling for the on-screen playback panel
- `frontend/src/hooks/useKeyboardWorkflow.ts` - window-scoped keydown handling mapping documented keys to `dispatch` actions
- `frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx` - grouped, scrollable shortcut reference panel
- `frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.module.css` - styling for the reference panel

## Decisions Made

- Added `GetState` as a 6th `PlaybackService` method beyond the plan's bpm/switch/layer/evaluate action list, since no existing CLI route lists scenes/layers and an on-screen selector is unusable without one.
- Extended `NewPlaybackService`'s constructor to take showPath/root parameters (every method needs to `Load`/`Save` the ShowState) and updated `cmd/golc-desktop/main.go`'s single construction call site to match.
- Exported a single `dispatch` object of action functions from `PlaybackControls.tsx` as the one source of truth both surfaces call, so PLAY-01 and PLAY-02 can never drift out of sync. Tap tempo shares one accumulating buffer across mouse and keyboard taps.
- `KeyboardShortcuts` is mounted from `PlaybackControls.tsx` via a toggle button rather than a new App.tsx mount point, consistent with 06-04's contract that App.tsx layout/mount points are never edited.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added `GetState`, a 6th `PlaybackService` method beyond the plan's named action list**
- **Found during:** Task 1 (`internal/wails/svc_playback.go`)
- **Issue:** The plan requires on-screen scene/layer selection ("switch scenes" via on-screen controls), but no existing CLI route in `internal/command/playback.go`/`scene.go` lists the show's scenes/layers -- without a read projection, `PlaybackControls.tsx` has no data source to render a selector against.
- **Fix:** Added `GetState`, a read-only projection of every scene's layers/active flag plus show-wide BPM, built from a read-only `show.Load` (no mutation).
- **Files modified:** internal/wails/svc_playback.go, internal/wails/svc_playback_test.go
- **Verification:** `TestPlaybackServiceGetState` passes; `go build ./...` succeeds.
- **Committed in:** 0f15208 (Task 1 commit)

**2. [Rule 3 - Blocking] Extended `NewPlaybackService`'s constructor and updated `cmd/golc-desktop/main.go`**
- **Found during:** Task 1 (`internal/wails/svc_playback.go`)
- **Issue:** The 06-04 stub's `NewPlaybackService(pipeName string)` had no `root`/`showPath`, but every real method needs to `show.Load`/`show.Save` the ShowState -- without them the service cannot function at all.
- **Fix:** Extended the constructor to accept showPath/root and updated `cmd/golc-desktop/main.go`'s single construction call site to pass them.
- **Files modified:** internal/wails/svc_playback.go, cmd/golc-desktop/main.go
- **Verification:** `go build ./...` succeeds (including `cmd/golc-desktop`, which embeds the built frontend).
- **Committed in:** 0f15208 (Task 1 commit)

**3. [Rule 1 - Bug] SetLayerEnabled preserves the layer's existing Ref across an enable/disable toggle**
- **Found during:** Task 1 (`internal/wails/svc_playback.go`)
- **Issue:** `scene.go`'s underlying WR-03 selector-merge behavior only merges `Selection`, never `Ref`, when a selector flag is omitted -- a naive enable/disable binding that only passed the enabled-flag argument would silently discard a previously assigned base-look/color-theme/chase/motion reference on every toggle.
- **Fix:** `SetLayerEnabled` reads the target layer's current `Ref` via a read-only `show.Load` immediately before building its mutating registry call, so the Ref is explicitly re-supplied and never dropped.
- **Files modified:** internal/wails/svc_playback.go
- **Verification:** `TestPlaybackServiceSetLayerEnabledPreservesRefAcrossToggle` passes.
- **Committed in:** 0f15208 (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (2 missing-critical, 1 blocking; 1 bug fix folded into the same commit)
**Impact on plan:** All necessary for the plan's own stated methods/acceptance criteria to be implementable, functional, and correct. No unrelated scope creep.

## Issues Encountered

None beyond the deviations above.

## User Setup Required

None - no external service configuration required.

## Checkpoint Verification -- Deferred to End-of-Phase UAT

**Task 3 (`checkpoint:human-verify`, `gate="blocking"`) -- PENDING, deferred to end-of-phase.**

`.planning/config.json` sets `workflow.human_verify_mode: "end-of-phase"` for this project. Per that mode (documented in `references/checkpoints.md`: "New projects do NOT halt mid-flight at `checkpoint:human-verify`... the verifier harvests every `<verify><human-check>` at end-of-phase... and consolidates them into the existing `human_needed -> UAT.md` flow"), this task's live verification is deferred rather than halting this worktree-isolated executor mid-flight -- especially since this plan runs in a disposable parallel worktree with no continuation agent to resume into after a pause. This is also consistent with how plan 06-07 (same phase) already handled its equivalent checkpoint. The checkpoint's content is preserved here (and in the `coverage` D3 entry's `human_judgment: true` + `rationale`) for the phase's end-of-phase UAT pass.

**What was built:** On-screen playback controls and the documented keyboard workflow for the complete playback action set.

**How to verify:**

1. Launch the desktop app with these environment variables (from this worktree root):
   `GOLC_PROJECT_ROOT=C:\Users\Lawrence\AppData\Local\Temp\claude\C--Users-Lawrence-Documents-Dev-golc\74d9188f-18e3-4830-adee-dcafb0ebda56\scratchpad\06-06-checkpoint`
   `GOLC_DESKTOP_SHOW=demo.golc`
   `GOLC_DESKTOP_INTERFACE=1`
   then run the built `golc-desktop.exe` at `.tools\installs\golc_desktop\bin\golc-desktop.exe`.
2. Using ONLY on-screen controls: click "Chorus" to switch scenes, toggle each layer button, type a BPM and click "Set BPM", click "Tap Tempo" a few times, click "Evaluate" -- confirm each reflects immediately in the Playback panel (scene highlight, layer toggle state, BPM readout, JSON preview).
3. Click "Keyboard Shortcuts" to open the reference panel; using ONLY the keyboard (window focused), press 1/2 to switch scenes, Q/W/E/R to toggle layers, Space (twice, ~0.5s apart) to tap tempo, up/down to nudge BPM, Enter to evaluate -- confirm each performs the identical action the on-screen controls do.
4. Confirm the keyboard shortcuts stop firing when focus moves to another application (window-scoped, not global) -- unlike the three safety-cluster hotkeys from 06-04, which fire regardless of focus.

**Resume signal (per the original plan):** Type "approved" if every playback action is reachable both on screen and by documented keyboard without MIDI hardware; otherwise list any missing action.

**REQUIREMENTS.md note:** PLAY-01 and PLAY-02 are NOT marked complete in `.planning/REQUIREMENTS.md` by this executor, given the live-verification gap above. They remain open pending the end-of-phase UAT pass confirming this checkpoint.

## Next Phase Readiness

- The complete playback workflow (scene switch, layer toggle, BPM set/tap, evaluate) is bindable, on-screen, and keyboard-operable, with `dispatch` as the single shared action source keeping both surfaces in sync -- ready for the phase's end-of-phase UAT pass and for 06-08 (MIDI learn) to layer a third input surface onto the same `dispatch` actions.
- Blocker/concern: Task 3's live verification (on-screen + keyboard parity, window-scoping of playback keys) is unverified pending end-of-phase UAT; PLAY-01/PLAY-02 remain open in REQUIREMENTS.md until then.

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

All created files and commit hashes verified present on disk / in git log.
