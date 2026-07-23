---
phase: 06-wails-authoring-and-operator-surface
fixed_at: 2026-07-23T21:09:23Z
review_path: .planning/phases/06-wails-authoring-and-operator-surface/06-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 6: Code Review Fix Report

**Fixed at:** 2026-07-23T21:09:23Z
**Source review:** .planning/phases/06-wails-authoring-and-operator-surface/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (3 Critical, 3 Warning -- `fix_scope: critical_warning`; IN-01 excluded from scope)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: Operator-surface authorization is never enforced on any real dispatch path

**Files modified:** `internal/wails/svc_safety.go`, `internal/wails/svc_safety_test.go`, `internal/wails/svc_playback.go`, `internal/wails/svc_playback_test.go`, `cmd/golc-desktop/main.go`, `frontend/src/lib/wailsBridge.ts`, `frontend/src/components/OperatorSurface/OperatorSurface.tsx`
**Commit:** 1887035
**Applied fix:** Added an `activeSurface` concept to `SafetyService` and `PlaybackService`, mirroring `MidiService`'s existing pattern: a new `SetActiveSurface(surfaceName string)` bound method on each, and an `authorizeSafety`/`authorizeControl` helper that calls `command.Authorize` (the same server-side D-04 enforcement point `SurfaceService.AuthorizeControl` and `MidiService.StartLearn` already use) before dispatching. `Blackout`/`StopReleaseAll`/`RevokeAutomation` now authorize against `operatorsurface.SafetyControlRef`; `SwitchScene`/`SetLayerEnabled` now authorize against the resolved scene/layer `ControlRef`. When no active surface is set (the default), dispatch is unrestricted -- identical to pre-fix behavior, so every existing test passes unmodified. `SafetyService`'s constructor gained `root`/`showPath` parameters (previously IPC-only) so it can load `ShowState` to resolve the active surface; all call sites (`main.go`, all 9 test constructions) were updated. Wired the frontend: `OperatorSurface.tsx`'s existing "Preview as Operator" toggle (previously "a UI affordance only, never the actual enforcement" per its own doc comment) now calls the new `setSafetyActiveSurface`/`setPlaybackActiveSurface` bridge helpers on entering/leaving operate mode, so the one existing "operate mode" affordance in this codebase now actually scopes real dispatch, not just rendering.

**Scope note:** `SetBPM`/`TapTempo`/`Evaluate` are intentionally NOT gated -- `internal/operatorsurface/model.go`'s `ControlKind` enum has no "tempo" member (only scene/layer/master/safety are individually-assignable controls), so there is structurally nothing for those three methods to authorize against. This mirrors the CR-01 finding's own caveat and is a pre-existing domain-model constraint, not something this fix could close.

**Verification:** Added `TestSafetyServiceBlackoutRejectsWhenActiveSurfaceDoesNotAssignControl`, `TestSafetyServiceBlackoutDispatchesWhenActiveSurfaceAssignsControl`, `TestSafetyServiceSetActiveSurfaceEmptyClearsRestriction`, `TestPlaybackServiceSwitchSceneRejectsWhenActiveSurfaceDoesNotAssignScene`, `TestPlaybackServiceSwitchSceneDispatchesWhenActiveSurfaceAssignsScene`, `TestPlaybackServiceSetActiveSurfaceEmptyClearsRestriction`. `go build ./...`, `go vet ./...`, `go test ./...`, and `npm run build` all pass.

### CR-02: `MidiService.CancelLearn` panics on a double call ("close of closed channel")

**Files modified:** `internal/wails/svc_midi.go`, `internal/wails/svc_midi_test.go`
**Commit:** 3d69a45
**Applied fix:** Applied exactly the guard the review suggested: moved the `s.learning == nil` check and the `close(session.cancel)` call under the same mutex, nil-ing `s.learning` before releasing the lock and closing the channel -- a second concurrent/sequential `CancelLearn` now observes `s.learning == nil` and returns `GOLC_MIDI_LEARN_NOT_ACTIVE` instead of racing into a double-close. Matches `StartLearn`'s own `if s.learning == session` deferred-cleanup guard, so no conflict with that path.

**Verification:** Added `TestMidiServiceCancelLearnDoubleCallDoesNotPanic` (sequential double call) and `TestMidiServiceCancelLearnConcurrentDoubleCallDoesNotPanic` (two goroutines calling `CancelLearn` concurrently, exactly reproducing the reported double-click race) -- both pass under `go test -race`.

### CR-03: Safety cluster (hotkeys + on-screen buttons) has no in-app release path

**Files modified:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx`, `internal/wails/hotkey.go`, `internal/wails/app_test.go`
**Commit:** a890296
**Applied fix:** Applied the review's suggested toggle pattern on the frontend: each `HoldButton`'s `onActivate` now calls its `SafetyService` binding with `!blackoutOrStopActive` / `!revokeActive` instead of a hardcoded `true`, and the button label flips to "Hold to Release ..." while active. On the Go side, `hotkey.go`'s `listen` loop gained `nextToggleValue`, which queries `"artnet status"` (reusing `svc_safety.go`'s existing `daemonPlaybackEnvelope` decode type, same package) and forwards the opposite of the currently observed combined state, instead of always forwarding `"--on true"`. A status-query failure defaults to `true` (activate) -- the pre-fix always-activate behavior -- rather than guessing a release the daemon cannot currently confirm. Blackout/Stop-Release-All share one combined `outputState=="blackout"` signal on the wire (no separate per-flag field exists, mirrored from `SafetyCluster.tsx`'s own pre-existing `blackoutOrStopActive` ambiguity note); Revoke Automation toggles off its own unambiguous `controllingSource=="revoked"`. The daemon-side `"artnet safety ... --on true|false"` route (already supporting both) is unchanged -- this fix only changes which value the two trigger paths send.

**Verification:** Updated `TestHotkeyKeydownForwardsDirectlyToDaemon` (now mocks the status query separately from the toggle forward) and added `TestHotkeyKeydownReleasesWhenAlreadyActive` (daemon reports blackout active -> hotkey forwards `--on false`). `go build ./...`, `go vet ./...`, `go test ./...`, and `npm run build` all pass.

### WR-01: `svc_playback.go`'s `SetLayerEnabled` pre-read failure is silently swallowed

**Files modified:** `internal/wails/svc_playback.go`, `internal/wails/svc_playback_test.go`
**Commit:** ffd8bc9
**Applied fix:** Applied exactly the fix the review suggested: `currentLayerRef` now returns `(uuid.UUID, error)` instead of folding a `show.Load` failure into the same zero-UUID "no ref assigned" result, and `SetLayerEnabled` returns the pre-read error as its own `Result` rather than proceeding to the mutating call without `--ref`.

**Verification:** Added `TestPlaybackServiceSetLayerEnabledPropagatesPreReadFailure` (points `showPath` at a directory so the pre-read `show.Load` fails to open the store) proving `SetLayerEnabled` now fails loudly instead of silently omitting `--ref`. All existing `SetLayerEnabled` tests still pass unmodified.

### WR-02: Desktop app loads Google Fonts over the network on every launch

**Files modified:** `frontend/index.html`, `frontend/src/main.tsx`, `frontend/package.json`, `frontend/package-lock.json`
**Commit:** 17016c9
**Applied fix:** Applied the review's suggested fix: removed the `<link rel="stylesheet" href="https://fonts.googleapis.com/...">` tag from `index.html` and self-hosted Archivo/JetBrains Mono via `@fontsource/archivo` and `@fontsource/jetbrains-mono` (pinned to exact version `5.3.0`, matching this repo's exact-pin convention), imported per-weight in `main.tsx` for the identical weight set the Google Fonts request specified (Archivo 400/500/600/700/800/900, JetBrains Mono 400/500/600). `vite build` now bundles every font file as a local hashed asset under `dist/assets/`; confirmed the built `index.html` contains zero external `href`/`src` references.

**Verification:** `npm run build` succeeds and produces a fully self-contained `dist/` with no `fonts.googleapis.com` reference in the built output.

### WR-03: `PlaybackControls.tsx` polls `GetState` every second unconditionally, including while the daemon is unreachable

**Files modified:** `frontend/src/components/PlaybackControls/PlaybackControls.tsx`
**Commit:** 9b2ced8
**Applied fix:** Applied the review's suggested fix: the polling effect now skips starting/continuing the `setInterval` loop while `connectionStatus !== "connected"`, and re-runs (via `connectionStatus` now in the effect's dependency array) the moment the daemon reconnects -- so polling resumes immediately rather than waiting up to `STATE_POLL_INTERVAL_MS` for the next already-scheduled tick. Matches `LiveStatusBar.tsx`'s `STATUS_GAP_MS` and `SafetyService`'s throttled-push convention of changing cadence/behavior on a disconnected daemon.

**Verification:** `npm run build` (`tsc --noEmit && vite build`) passes with no type errors.

## Skipped Issues

None -- all in-scope findings were fixed. IN-01 (`frontend/package.json` version pins) was excluded by `fix_scope: critical_warning` and left untouched.

## Full Verification (run after every fix in this report)

- `go build ./...` -- pass
- `go vet ./...` -- pass
- `go test ./...` -- pass (all packages, including `internal/wails` under `-race` for the CR-02/CR-01 concurrency-sensitive paths)
- `cd frontend && npm run build` (`tsc --noEmit && vite build`) -- pass

## Notes for Human Review

- **CR-01** introduces new bound methods (`SafetyService.SetActiveSurface`, `PlaybackService.SetActiveSurface`) and changes `NewSafetyService`'s constructor signature (added `root`, `showPath`). This is a genuine architectural addition (an "active operator surface" concept previously only `MidiService` had) rather than a narrow bug patch -- recommend a human confirm the "unrestricted when no active surface is set, scoped only while `OperatorSurface.tsx`'s 'Preview as Operator' toggle is engaged" design matches the intended D-04 threat model, since `PlaybackControls.tsx`/`SafetyCluster.tsx` themselves still never set an active surface on their own (they remain full-author-mode controls by design in this phase) -- only `OperatorSurface.tsx`'s own operator-preview toggle currently exercises the new lock.
- **CR-03**'s Blackout/Stop-Release-All toggle share one combined `outputState` signal on the wire (pre-existing constraint, not introduced by this fix) -- releasing "Hold to Blackout" while only Stop-Release-All is actually active will appear to have no visible effect (Blackout's own flag was already off). This ambiguity is inherited from `SafetyCluster.tsx`'s own pre-existing `blackoutOrStopActive` derivation and was out of scope for CR-03 to resolve (would require a new daemon-side per-flag status field).

---

_Fixed: 2026-07-23T21:09:23Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
