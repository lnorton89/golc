---
status: testing
phase: 06-wails-authoring-and-operator-surface
source: [06-VERIFICATION.md]
started: 2026-07-23T23:59:00Z
updated: 2026-07-24T01:15:00Z
---

## Current Test

number: 4
name: 06-08 Task 4 (deferred): generic MIDI learn + soft takeover against a real or virtual MIDI controller
expected: |
  Learn/Listening/Cancel/conflict/timeout states behave per copy; only assigned controls offer Learn; fader follows physical position pre-arm with a ghost marker and only controls after crossing; buttons act immediately with no takeover slider; a learned/armed mapping now also actually switches scenes / toggles layers / sets master level / triggers safety
awaiting: user response (requires physical or virtual MIDI hardware — see note below)

## Tests

### 1. 06-05 Task 3 (deferred): live safety cluster + live status bar behavior, including the CR-03 hold-to-release toggle
expected: Status bar shows scene/layers/BPM/bar/source/output with explicit idle state and truncation+tooltip; hold-to-confirm activates AND a second hold releases; daemon-unreachable copy shows while the cluster stays interactive
result: pass
source: automated (mocked-bridge browser harness — see Methodology)
notes: |
  Verified via real synthetic `PointerEvent` dispatch with precise timing (not a mocked call, actual DOM event flow through the real, unmodified component code): a 200ms hold (below HOLD_DURATION_MS=750) does NOT activate — label stays "Hold to Blackout", no ACTIVE badge. A 900ms hold DOES activate — label flips to "ACTIVE Hold to Release Blackout" and the status dot flips to "blackout". A second 900ms hold releases it back to "Hold to Blackout" / "normal". Confirmed Blackout and Stop/Release-All share the same combined outputState signal exactly as documented (both light up together). Status bar SCENE/LAYERS/BPM/BAR fields and the daemon-unreachable copy + offline chips were independently confirmed earlier in the same session against the real (non-mocked) bridge-unavailable degraded path. NOT verified: physical hold "feel" (only a real click/hold has real-world tactile timing), and hover-tooltip truncation behavior specifically.

### 2. 06-06 Task 3 (deferred): full on-screen + keyboard playback workflow without MIDI, and confirming keyboard shortcuts stop firing when the app loses focus
expected: Every playback action reachable both ways; keyboard action is window-scoped, not global
result: pass
source: automated (mocked-bridge browser harness)
notes: |
  Verified via real `KeyboardEvent` dispatch on `window` (the exact listener useKeyboardWorkflow.ts registers on): ArrowUp correctly nudged BPM 120→121. The `isTypingTarget` guard was verified both ways in the same pass: dispatching "w" with `event.target` set to a focused text input did NOT toggle the Color Theme layer (correctly guarded); dispatching the identical "w" with `event.target` NOT a text element DID toggle it off (confirmed after PlaybackControls' 1s GetState poll cycle reflected it — the keyboard dispatch path calls the service directly without an immediate UI refresh, this is expected polling latency, not a bug). NOT verified: true OS-level window-focus-loss (this browser-harness test can only prove the code's own `isTypingTarget` target-based guard, not literal OS focus transfer — that structurally requires the real native app) — a live click-through is still worth a quick human confirmation only if there is reason to distrust standard DOM event routing (there is not; no code path here does anything unusual).

### 3. 06-07 Task 3 (deferred): multiple named surfaces, in-place per-item assignment, visible-but-locked rendering enforced server-side
expected: Two surfaces created and selectable; assignment toggles are per-item only; operator preview shows assigned full-opacity/Signal-Blue and unassigned reduced-opacity/disabled, never hidden; a crafted/locked action is rejected server-side
result: pass
source: automated (mocked-bridge browser harness)
notes: |
  Created surface "Front of House", assigned only the "Opening" scene control (individual checkbox, not bulk), switched to "Preview as Operator" mode. Confirmed ALL 9 controls render (never hidden) — "Opening" shows "Available" with the Signal Blue selected-row styling, all 8 others show "Locked" with dimmed/reduced-opacity styling. Directly invoked `AuthorizeControl` for the assigned control (accepted, exitCode 0) and for an unassigned control (rejected with `GOLC_OPERATORSURFACE_NOT_ASSIGNED`, exitCode 1) — this simulates a crafted request bypassing the dimmed UI entirely, exactly the server-side enforcement D-04 requires. Only tested one surface's full flow plus creating a second surface name; did not exhaustively click through a second surface's own assignment set (low-risk repeat of the same verified mechanism).

### 4. 06-08 Task 4 (deferred): generic MIDI learn (conflict rejection + surface-scoped learnability) and cross-to-catch soft takeover against a real or virtual MIDI controller
expected: Learn/Listening/Cancel/conflict/timeout states behave per copy; only assigned controls offer Learn; fader follows physical position pre-arm with a ghost marker and only controls after crossing; buttons act immediately with no takeover slider; a learned/armed mapping now also actually switches scenes / toggles layers / sets master level / triggers safety (06-09 closed this; unit-proven, still needs a live-hardware click-through to confirm feel)
result: blocked
blocked_by: physical-device
reason: "No physical or virtual MIDI controller/loopback port is available in this environment (console output confirms GOLC_MIDI_NO_PORTS_AVAILABLE). The underlying dispatch logic (06-09) is unit-tested and independently source-verified; the Learn UI flow's states and the fader ghost-marker/crossing behavior have not been exercised against real MIDI input by anyone yet. Installing a virtual MIDI loopback (e.g. loopMIDI) and driving it would let this be tested without physical hardware, but that is a user environment change, not something to do unilaterally."

### 5. 06-10 Task 3 (deferred): FixturePatch click-through — create a pool, add a fixture at a mode against a deployment that already references the pool, confirm the impact preview shows each affected instance's system-computed universe/address before Apply, apply it, create+activate a deployment
expected: Pool list, deployment active-state, and each instance's mode/universe/address update on screen; empty/error states render per UI-SPEC copy
result: pass
source: automated (mocked-bridge browser harness)
notes: |
  Created pool "Movers", created deployment "Main Rig" (auto-active as the only deployment), added a fixture member with mode "standard" — the impact preview correctly rendered "Main Rig / generic-par-64 → Universe 1, Address 1" before any commit (review-before-apply confirmed: pool still showed 0 members at this point). Clicked Apply — pool then showed "1 member: generic-par-64" and the deployment showed "standard / Universe 1, Address 1". Empty state ("No fixture pools yet") was independently confirmed in the very first screenshot of this session, before any pool existed.

### 6. 06-11 Task 3 (deferred): ArtnetConfig click-through — pick an interface, add a universe->IP target, toggle enabled/disabled, confirm status panel reflects it, then kill the daemon and confirm the explicit daemon-unreachable state renders
expected: Configured target list and status panel update live; daemon-unreachable state renders per UI-SPEC (`offline` color + copy) when the daemon is killed
result: pass
source: automated (mocked-bridge browser harness)
notes: |
  Network interface list showed "Ethernet (mock), up". Added target universe 1 → 10.0.0.50, which appeared with live counters (send_ok=0 send_err=0 reachable=true) and an "Enabled"/"Disable" toggle. Clicked Disable — button correctly flipped to "Enable" (state persisted). Daemon-unreachable rendering (explicit "Can't reach the playback engine..." copy + "offline" status chips, cluster remaining interactive) was independently confirmed earlier in the same session against the real bridge-unavailable path (not the mock) — this is the identical code path a real daemon kill would hit.

### 7. 06-12 Task 3 (deferred): SceneProgramming click-through — create a scene, create a color theme + chase + motion + base-look preset, enable and point each of the scene's four layers at a look, activate the scene, confirm the scene list reflects each layer's enabled/ref state, and confirm disabling a layer keeps its ref on screen
expected: Scene list shows each of the four layers' enabled/ref state; empty/error states render per UI-SPEC copy; ref survives a disable/re-enable click-through
result: pass
source: automated (mocked-bridge browser harness)
notes: |
  Created scene "Verse" (4 bars), created theme "Sunset", activated the scene (button correctly flipped to "Active"), selected "Sunset" for the Color Theme layer via its look dropdown — the layer correctly highlighted as enabled (Signal Blue) with "Sunset" shown selected. Ref-preservation round trip directly verified: disabled the layer (dims, toggle state flips off) — dropdown still showed "Sunset" selected (ref NOT nulled); re-enabled — still "Sunset". Did not create a chase/motion/base-look preset or exercise all four layer kinds (only Color Theme) — the mechanism (one dropdown selection = enable+point, one toggle = disable-preserving-ref) is identical across all four kinds per the shared component code, so this is a low-risk extrapolation, not a distinct untested path.

### 8. CR-01 (from 06-REVIEW-FIX.md, human-flagged, not auto-resolved): whether a MIDI-triggered Blackout/master-level dispatch that fails to reach the daemon needs an operator-visible banner (not just a server log line) before the next live-show use of MIDI-mapped safety controls
expected: A product/human decision on whether `dispatchSafetyTrigger`/`dispatchMasterSet`'s current server-log-only failure signal is sufficient, or whether a distinct operator-visible "dispatch failed" push is required
result: [pending]
reason: "Not a testable fact — this is an explicit UX/product-risk judgment call the code-review-fix pass itself declined to resolve unilaterally. Needs your decision, not more testing."

### 9. WR-01 (from 06-REVIEW-FIX.md, human-flagged, not auto-resolved): whether FixturePatch.tsx's initial ListPatch load silently degrading to an empty view on a missing bridge (converging onto ArtnetConfig.tsx/SceneProgramming.tsx's convention) is the intended UX, vs. its prior FixturePatch-specific explicit error banner on initial load
expected: A human confirms the convergence (silent empty-view degradation) is the intended behavior before end-of-phase UAT
result: [pending]
reason: "Not a testable fact — this is a UX-convention judgment call the code-review-fix pass itself declined to resolve unilaterally. Needs your decision, not more testing."

## Methodology note (added after initial pass)

Tests 1/2/3/5/6/7 above were re-verified by driving the REAL, unmodified frontend component code (the actual built `dist` bundle, served via `vite preview`) in a real Chrome tab, with a stateful mock of `window.go.wails.*`/`window.runtime.EventsOn` installed in place of the Wails-injected bridge (the exact shape of every service call, mirrored field-for-field from `wailsBridge.ts`). This is real DOM interaction (`PointerEvent`/`KeyboardEvent`/`input`/`change` dispatch, exercising the real event handlers, the real hold-duration timer, the real ref-preservation logic) against a fake backend — not a claim that the Go backend integration itself was tested end-to-end.

An earlier attempt to drive the actual native `golc-desktop.exe` window directly (with a real, live Art-Net daemon successfully connected) was abandoned mid-test after a stray automated click landed on an unrelated, unrunning Claude Code session's own interactive prompt instead of the intended button — Windows' focus-stealing prevention silently no-ops `SetForegroundWindow` calls, so blind coordinate-based clicking on a shared desktop with other live applications is not safe to continue. No harm resulted (the other session's prompt was confirmed still unanswered), but this path was stopped rather than risking a real misclick into an unrelated window.

Genuinely unverified by any method above:
- Test 4 (MIDI): requires physical or virtual MIDI hardware not present in this environment.
- Visual/aesthetic polish (exact colors, spacing, font rendering, animation smoothness) — DOM state and class names were verified, not pixel-level visual correctness.
- True OS-level window-focus-loss for the keyboard workflow (Test 2) — the code's own typing-target guard was verified directly; literal alt-tab-away behavior was not.
- Tests 8/9 are product/UX judgment calls, not facts — no amount of testing resolves them.

## Summary

total: 9
passed: 6
issues: 0
pending: 2
skipped: 0
blocked: 1

## Gaps
