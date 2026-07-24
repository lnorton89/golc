---
status: testing
phase: 06-wails-authoring-and-operator-surface
source: [06-VERIFICATION.md]
started: 2026-07-23T23:59:00Z
updated: 2026-07-23T23:59:00Z
---

## Current Test

number: 1
name: 06-05 Task 3 (deferred): live safety cluster + live status bar behavior, including the CR-03 hold-to-release toggle
expected: |
  Status bar shows scene/layers/BPM/bar/source/output with explicit idle state and truncation+tooltip; hold-to-confirm activates AND a second hold releases; daemon-unreachable copy shows while the cluster stays interactive
awaiting: user response

## Tests

### 1. 06-05 Task 3 (deferred): live safety cluster + live status bar behavior, including the CR-03 hold-to-release toggle
expected: Status bar shows scene/layers/BPM/bar/source/output with explicit idle state and truncation+tooltip; hold-to-confirm activates AND a second hold releases; daemon-unreachable copy shows while the cluster stays interactive
result: [pending]

### 2. 06-06 Task 3 (deferred): full on-screen + keyboard playback workflow without MIDI, and confirming keyboard shortcuts stop firing when the app loses focus
expected: Every playback action reachable both ways; keyboard action is window-scoped, not global
result: [pending]

### 3. 06-07 Task 3 (deferred): multiple named surfaces, in-place per-item assignment, visible-but-locked rendering enforced server-side
expected: Two surfaces created and selectable; assignment toggles are per-item only; operator preview shows assigned full-opacity/Signal-Blue and unassigned reduced-opacity/disabled, never hidden; a crafted/locked action is rejected server-side
result: [pending]

### 4. 06-08 Task 4 (deferred): generic MIDI learn (conflict rejection + surface-scoped learnability) and cross-to-catch soft takeover against a real or virtual MIDI controller
expected: Learn/Listening/Cancel/conflict/timeout states behave per copy; only assigned controls offer Learn; fader follows physical position pre-arm with a ghost marker and only controls after crossing; buttons act immediately with no takeover slider; a learned/armed mapping now also actually switches scenes / toggles layers / sets master level / triggers safety (06-09 closed this; unit-proven, still needs a live-hardware click-through to confirm feel)
result: [pending]

### 5. 06-10 Task 3 (deferred): FixturePatch click-through — create a pool, add a fixture at a mode against a deployment that already references the pool, confirm the impact preview shows each affected instance's system-computed universe/address before Apply, apply it, create+activate a deployment
expected: Pool list, deployment active-state, and each instance's mode/universe/address update on screen; empty/error states render per UI-SPEC copy
result: [pending]

### 6. 06-11 Task 3 (deferred): ArtnetConfig click-through — pick an interface, add a universe->IP target, toggle enabled/disabled, confirm status panel reflects it, then kill the daemon and confirm the explicit daemon-unreachable state renders
expected: Configured target list and status panel update live; daemon-unreachable state renders per UI-SPEC (`offline` color + copy) when the daemon is killed
result: [pending]

### 7. 06-12 Task 3 (deferred): SceneProgramming click-through — create a scene, create a color theme + chase + motion + base-look preset, enable and point each of the scene's four layers at a look, activate the scene, confirm the scene list reflects each layer's enabled/ref state, and confirm disabling a layer keeps its ref on screen
expected: Scene list shows each of the four layers' enabled/ref state; empty/error states render per UI-SPEC copy; ref survives a disable/re-enable click-through
result: [pending]

### 8. CR-01 (from 06-REVIEW-FIX.md, human-flagged, not auto-resolved): whether a MIDI-triggered Blackout/master-level dispatch that fails to reach the daemon needs an operator-visible banner (not just a server log line) before the next live-show use of MIDI-mapped safety controls
expected: A product/human decision on whether `dispatchSafetyTrigger`/`dispatchMasterSet`'s current server-log-only failure signal is sufficient, or whether a distinct operator-visible "dispatch failed" push is required
result: [pending]

### 9. WR-01 (from 06-REVIEW-FIX.md, human-flagged, not auto-resolved): whether FixturePatch.tsx's initial ListPatch load silently degrading to an empty view on a missing bridge (converging onto ArtnetConfig.tsx/SceneProgramming.tsx's convention) is the intended UX, vs. its prior FixturePatch-specific explicit error banner on initial load
expected: A human confirms the convergence (silent empty-view degradation) is the intended behavior before end-of-phase UAT
result: [pending]

## Summary

total: 9
passed: 0
issues: 0
pending: 9
skipped: 0
blocked: 0

## Gaps
