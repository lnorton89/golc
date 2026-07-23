---
phase: 06-wails-authoring-and-operator-surface
reviewed: 2026-07-23T20:44:45Z
depth: standard
files_reviewed: 45
files_reviewed_list:
  - cmd/golc-desktop/main.go
  - cmd/golc-desktop/midi_driver.go
  - frontend/index.html
  - frontend/package.json
  - frontend/src/App.tsx
  - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.module.css
  - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx
  - frontend/src/components/LiveStatusBar/LiveStatusBar.module.css
  - frontend/src/components/LiveStatusBar/LiveStatusBar.tsx
  - frontend/src/components/MidiPanel/MidiLearn.tsx
  - frontend/src/components/MidiPanel/MidiPanel.tsx
  - frontend/src/components/MidiPanel/SoftTakeoverSlider.tsx
  - frontend/src/components/OperatorSurface/AssignmentToggle.tsx
  - frontend/src/components/OperatorSurface/OperatorSurface.tsx
  - frontend/src/components/OperatorSurface/SurfaceList.tsx
  - frontend/src/components/PlaybackControls/PlaybackControls.tsx
  - frontend/src/components/SafetyCluster/SafetyCluster.module.css
  - frontend/src/components/SafetyCluster/SafetyCluster.tsx
  - frontend/src/hooks/useKeyboardWorkflow.ts
  - frontend/src/index.css
  - frontend/src/lib/wailsBridge.ts
  - frontend/src/main.tsx
  - frontend/src/store/store.ts
  - frontend/vite.config.ts
  - internal/artnet/daemon.go
  - internal/artnet/safety.go
  - internal/artnet/safety_test.go
  - internal/artnet/worker.go
  - internal/command/artnet.go
  - internal/command/operatorsurface.go
  - internal/midi/driver.go
  - internal/midi/learn.go
  - internal/midi/takeover.go
  - internal/operatorsurface/model.go
  - internal/operatorsurface/validate.go
  - internal/playback/engine.go
  - internal/show/migrate.go
  - internal/show/state.go
  - internal/wails/app.go
  - internal/wails/app_test.go
  - internal/wails/events.go
  - internal/wails/hotkey.go
  - internal/wails/svc_midi.go
  - internal/wails/svc_midi_test.go
  - internal/wails/svc_playback.go
  - internal/wails/svc_playback_test.go
  - internal/wails/svc_safety.go
  - internal/wails/svc_safety_test.go
  - internal/wails/svc_surface.go
  - internal/wails/svc_surface_test.go
findings:
  critical: 3
  warning: 3
  info: 1
  total: 7
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-07-23T20:44:45Z
**Depth:** standard
**Files Reviewed:** 49 (see `files_reviewed_list`)
**Status:** issues_found

## Summary

Phase 6 delivers a Wails v2 desktop shell, a generic MIDI Note/CC learn +
soft-takeover surface, and a local-priority daemon safety-override system,
built across four executor waves and merged. The Art-Net-daemon-resident
safety path itself (`internal/artnet/safety.go`, `worker.go`, `daemon.go`)
is solid: blackout/stop-all/revoke-automation are lock-free atomics read
every tick, `applyOverrides` is a pure copy-returning transform with good
edge-case tests (empty frame, nil safety state, concurrent Set/Load under
`-race`), and `requestSource`'s automation-revocation gate runs before
route dispatch independent of any UI/script/API path exactly as designed.
The MIDI cross-to-catch crossing logic (`internal/midi/takeover.go`) is
correctly direction-aware (`crossedUp`/`crossedDown`, NaN-seeded
`LastPhysical` to avoid a spurious first-message arm) — no
proximity/threshold check exists anywhere in that file, matching the
explicit design mandate.

However, two areas the review was specifically asked to scrutinize turned
up real defects, plus a third, independently discovered, safety-cluster
usability defect that a reviewer of a "must never depend on UI completion"
subsystem should not let pass:

1. **The server-side operator-surface authorization enforcement point
   (`AuthorizeControl`/`command.Authorize`) is built and unit-tested in
   isolation, but is never actually invoked by any of the real dispatch
   paths it exists to gate** (`SafetyService`, `PlaybackService`, and the
   frontend itself). A crafted/replayed Wails-bound call to
   `SwitchScene`, `SetLayerEnabled`, `SetBPM`, `TapTempo`, `Blackout`,
   `StopReleaseAll`, or `RevokeAutomation` succeeds regardless of what the
   active operator surface has assigned — the entire D-04
   visible-but-locked access-control story described in this codebase's
   own doc comments does not currently exist at the enforcement layer.
   This is exactly the bypass the review was asked to verify is
   impossible.
2. **`MidiService.CancelLearn` double-closes a channel and panics** if
   called twice while a learn session is active — a crash reachable from
   the frontend's own Cancel button.
3. **The safety cluster (on-screen hold buttons and the three OS-level
   global hotkeys) can only ever turn Blackout / Stop-Release-All /
   Revoke Automation *on* — there is no in-app way to turn any of them
   back off.** Once activated, recovery requires dropping to a separate
   CLI process, which is likely unavailable to an on-site operator during
   a live show.

## Critical Issues

### CR-01: Operator-surface authorization is never enforced on any real dispatch path

**File:** `internal/wails/svc_safety.go:130-166`, `internal/wails/svc_playback.go:81-158`, `internal/wails/svc_surface.go:256-283`, `frontend/src/components/OperatorSurface/OperatorSurface.tsx:14-18`

**Issue:** `SurfaceService.AuthorizeControl` (`svc_surface.go:266`) resolves
a control against the loaded `ShowState` and calls `command.Authorize`,
and its own doc comment states plainly: *"Every operator-mode dispatch
path (06-05 SafetyService, 06-06 PlaybackService) is expected to call
this same check... a crafted/replayed call against an unassigned control
is rejected here, in Go, exactly like the CLI's own `command.Authorize`."*
`06-07-SUMMARY.md` (the plan that built it) explicitly records: *"ready
for 06-05 (SafetyService) and 06-06 (PlaybackService) to call directly
before any operator-mode dispatch action."*

Neither promise was kept:

- `SafetyService.toggle` (`svc_safety.go:130`, called by `Blackout`,
  `StopReleaseAll`, `RevokeAutomation`) dials the daemon directly with no
  authorization check at all.
- `PlaybackService.SwitchScene`, `SetLayerEnabled`, `SetBPM`, `TapTempo`
  (`svc_playback.go:81-145`) each call `s.execute(...)` directly against
  the command registry — none call `command.Authorize` or
  `AuthorizeControl`.
- The frontend never calls `AuthorizeControl` either: it is declared in
  `wailsBridge.ts`'s `SurfaceServiceBinding` and in
  `OperatorSurface.tsx`'s local binding interface, but a repo-wide search
  (`grep -rn '\.AuthorizeControl\('`) finds **zero** call sites outside
  test files. `OperatorSurface.tsx`'s own "operate" mode is confirmed
  (by its own doc comment) to be "a UI affordance only, never the actual
  enforcement" — but nothing else enforces it either.
- `06-05-SUMMARY.md` and `06-06-SUMMARY.md` (the plans that built
  `SafetyService`/`PlaybackService`) never mention `Authorize` at all —
  this was not a documented, deliberately-deferred decision; it is a
  dropped cross-plan integration point.

The only place `command.Authorize` is actually invoked outside tests is
`MidiService.StartLearn` (`svc_midi.go:339`), which gates *learning* a
new MIDI mapping — it does not gate exercising an existing one, and it
has no bearing on the on-screen Playback/Safety controls at all.

Net effect: the entire "operator surface" access-control feature this
phase's D-04 requirement centers on is decorative. Any process capable of
calling a bound Wails method (which includes anything running inside the
webview's JS context, or a replayed/scripted call) can drive playback and
safety state without regard to which controls are assigned to which
surface. This is precisely the threat T-06-18 was written to mitigate,
and precisely what the review brief asked to be verified as impossible.

**Fix:** Every mutating method on `SafetyService` and `PlaybackService`
that corresponds to an operator-assignable control must resolve the
active surface + control ref and call `command.Authorize` (or
`AuthorizeControl`'s equivalent logic) before dispatching, returning the
`GOLC_OPERATORSURFACE_LOCKED` rejection Result on failure — mirroring
`MidiService.StartLearn`'s own pattern:

```go
func (s *SafetyService) Blackout(on bool) Result {
    if err := s.authorizeSafety(operatorsurface.Blackout); err != nil {
        return Result{ExitCode: 1, Stderr: err.Error()}
    }
    return s.toggle(string(routeBlackout), on)
}
```

This requires `SafetyService`/`PlaybackService` to know which surface is
"active" the same way `MidiService.activeSurface` already does (or to
accept an explicit surface name/ID from the frontend on every call). At
minimum, this gap needs to be either closed or explicitly, visibly
descoped in a follow-up plan — it should not ship silently as "done."

---

### CR-02: `MidiService.CancelLearn` panics on a double call ("close of closed channel")

**File:** `internal/wails/svc_midi.go:427-436`

**Issue:**

```go
func (s *MidiService) CancelLearn() Result {
	s.mu.Lock()
	session := s.learning
	s.mu.Unlock()
	if session == nil {
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_LEARN_NOT_ACTIVE: ...\n"}
	}
	close(session.cancel)
	return Result{Stdout: "GOLC_MIDI_LEARN_CANCELLED\n"}
}
```

`s.learning` is only set to `nil` by `StartLearn`'s own deferred cleanup,
which runs only after `StartLearn` itself returns (i.e., after
`CaptureCandidate` unblocks). If `CancelLearn` is called twice while a
learn session is still open — e.g. a user double-clicking the frontend's
"Cancel" button in `MidiLearn.tsx` before the first click's async result
resolves and the button's status flips away from `"listening"` (nothing
in `MidiLearn.tsx.handleCancel` disables the button or de-dupes the
call) — both calls read the same non-nil `session` and both call
`close(session.cancel)`. The second `close` on an already-closed channel
panics. Nothing recovers from a panic inside a Wails-bound method call,
so this crashes the whole desktop process, not just the request.

This is confirmed unguarded by any test: `svc_midi_test.go` never
exercises `CancelLearn` at all, let alone a concurrent double call.

**Fix:** Guard the close with the same mutex that already protects
`s.learning`, and nil out `s.learning` (or a `cancelled` flag) at the
point of cancellation so a second call sees the already-cancelled state
instead of double-closing:

```go
func (s *MidiService) CancelLearn() Result {
	s.mu.Lock()
	session := s.learning
	if session == nil {
		s.mu.Unlock()
		return Result{ExitCode: 1, Stderr: "GOLC_MIDI_LEARN_NOT_ACTIVE: ...\n"}
	}
	s.learning = nil
	s.mu.Unlock()
	close(session.cancel)
	return Result{Stdout: "GOLC_MIDI_LEARN_CANCELLED\n"}
}
```
(`StartLearn`'s own deferred cleanup already tolerates `s.learning`
having been changed out from under it via its `if s.learning == session`
guard, so this does not conflict with it.)

---

### CR-03: The safety cluster (hotkeys + on-screen buttons) can activate Blackout/Stop-Release-All/Revoke Automation but has no in-app way to release them

**File:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx:171-189`, `internal/wails/hotkey.go:176-180`

**Issue:** All three on-screen hold-to-confirm controls call their
matching `SafetyService` binding with a hardcoded `true`:

```tsx
<HoldButton label="Hold to Blackout" ... onActivate={() => { void safetyBlackout(true); }} />
<HoldButton label="Hold to Revoke Automation" ... onActivate={() => { void safetyRevokeAutomation(true); }} />
<HoldButton label="Hold to Stop / Release All" ... onActivate={() => { void safetyStopReleaseAll(true); }} />
```

The three OS-level global hotkeys (`hotkey.go:176-180`) do the same:
`Args: []string{"--on", "true", "--source", "manual"}`, always `true`,
never conditioned on current state.

`SafetyService`/the daemon route both support `--on false` (used by the
CLI: `artnet safety blackout --on false`), but nothing in the desktop
shell — no button, no hotkey, no other affordance anywhere in
`App.tsx`'s component tree — ever calls any of the three toggles with
`false`. Once an operator engages Blackout, Stop/Release-All, or Revoke
Automation from inside the desktop app (via mouse or the documented
emergency hotkeys), there is no way back to a live state from within the
app itself; recovery requires shelling out to `golc-project.exe artnet
safety blackout --on false` from a separate terminal. `06-UI-SPEC.md` and
`06-CONTEXT.md` (D-13/D-14/D-15/D-16) describe activation in detail but
never mention a release/deactivate flow, and `deferred-items.md` does not
track this as a known, deliberately-scoped gap either — it appears to
have simply never been designed.

For a subsystem whose entire premise is "must never depend on UI/script/
API/LLM completion" and must be reachable "regardless of what screen an
author or operator is on" (D-13), shipping an activate-only control with
no symmetric release path is a serious operational risk: a live show can
be driven into a Blackout/Stop-All state by a stray hold, a misfired
hotkey, or a deliberate emergency stop, and the on-screen safety cluster
that got it there cannot get it back out.

**Fix:** Make the hold buttons toggle against the currently observed
state (already available via `useGolcStore`'s `status.outputState` /
`status.controllingSource`, exactly as `SafetyCluster.tsx`'s own `active`
computation already derives), and give `hotkey.go`'s callbacks the same
toggle semantics (e.g. query current state via a lightweight daemon round
trip before forwarding, or have the daemon's own toggle route flip rather
than always set):

```tsx
onActivate={() => { void safetyBlackout(!blackoutOrStopActive); }}
```

At minimum, if "activate-only, CLI-recovers" is an intentional Phase 6
scope cut, it needs to be documented as a tracked, visible limitation
(e.g. in `deferred-items.md` and in the UI copy itself), not left
silently undiscoverable.

## Warnings

### WR-01: `svc_playback.go`'s `SetLayerEnabled` pre-read failure is silently swallowed

**File:** `internal/wails/svc_playback.go:89-103`

**Issue:** `currentLayerRef` returns `uuid.Nil` both when the layer
genuinely has no `Ref` and when `show.Load` itself fails (e.g. a
transient I/O error). In the latter case, `SetLayerEnabled` proceeds to
call the mutating route *without* `--ref`, silently discarding
whichever `Ref` was actually on disk if the subsequent `s.execute(...)`
call's own `show.Load` (inside the registry route) happens to succeed
where the first, pre-read `show.Load` failed. This is a narrow race
(two back-to-back loads of the same file, one failing and one
succeeding), but it directly contradicts this same method's own stated
purpose ("an enable/disable toggle that omitted this pre-read would
silently null out a previously assigned ... reference on every flip").

**Fix:** Propagate the pre-read error instead of treating it identically
to "no ref assigned":

```go
func (s *PlaybackService) currentLayerRef(sceneName, kind string) (uuid.UUID, error) {
	state, err := show.Load(s.root, s.showPath)
	if err != nil {
		return uuid.Nil, err
	}
	...
}
```
and have `SetLayerEnabled` return the pre-read error as its own `Result`
rather than proceeding.

### WR-02: Desktop app loads Google Fonts over the network on every launch

**File:** `frontend/index.html:6-9`

**Issue:** The only external network dependency in the entire desktop
shell is a `<link rel="stylesheet" href="https://fonts.googleapis.com/...">`
tag. A lighting-control desktop app is a plausible candidate for running
on an isolated/offline show network (the rest of this codebase goes to
considerable lengths to keep the Art-Net daemon, MIDI, and safety paths
fully local and dependency-free). If the venue network has no outbound
internet access, or DNS/TLS to `fonts.googleapis.com` is blocked or slow,
this stylesheet request will fail or stall page load/paint on every
startup, and it silently sends the user's IP to Google on every launch
where the network is available. Nothing in this phase's docs discusses
this as an accepted trade-off.

**Fix:** Self-host the Archivo/JetBrains Mono font files under
`frontend/public/fonts/` (or `@font-face` them via a bundled asset) so
the desktop shell has zero runtime network dependency for its own chrome.

### WR-03: `PlaybackControls.tsx` polls `GetState` every second unconditionally, including while the daemon is unreachable

**File:** `frontend/src/components/PlaybackControls/PlaybackControls.tsx:169-175`

**Issue:** The 1s `setInterval` polling loop runs regardless of
`connectionStatus`; when the bridge/daemon is unavailable,
`dispatch.getState()` resolves to `undefined` every second forever, and
the loop keeps firing indefinitely with no backoff. This is not
incorrect, but every other slice in this codebase (`LiveStatusBar`'s
`STATUS_GAP_MS` re-query, `SafetyService`'s throttled push) treats a
disconnected daemon as a reason to change cadence or state; this poller
is the one outlier that just keeps hammering at a fixed 1s cadence with
no distinguishing behavior when `connectionStatus === "unreachable"`.
Low severity (out of the stated performance-issue exclusion, this is a
consistency/robustness note, not a perf complaint) but worth aligning
with the rest of the codebase's "explicit connection-state handling"
convention.

**Fix:** Skip the poll (or fall back to a slower cadence) while
`connectionStatus !== "connected"`.

## Info

### IN-01: `frontend/package.json` pins unusually high major versions with no way to verify they resolve

**File:** `frontend/package.json:11-22`

**Issue:** `typescript: "7.0.2"` and `vite: "8.1.4"` are pinned as exact
versions. These may be legitimate as of this repo's stated "current
date," but no lockfile was in scope for this review to cross-check
against, and a reviewer cannot independently confirm these resolve to
real, installable npm packages without network access. If either version
string is a typo or was pinned against a pre-release/beta tag that later
got yanked, `npm ci`/`npm install` would fail outright for every
contributor and CI run. Worth a one-time sanity check (`npm view
typescript@7.0.2 version`, `npm view vite@8.1.4 version`) if not already
verified in CI.

**Fix:** Confirm both resolve on the npm registry, or pin to a caret
range if exact-pin was not a deliberate choice.

---

_Reviewed: 2026-07-23T20:44:45Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
