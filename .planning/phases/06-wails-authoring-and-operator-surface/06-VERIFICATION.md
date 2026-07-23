---
phase: 06-wails-authoring-and-operator-surface
verified: 2026-07-23T21:21:06Z
status: gaps_found
score: 9/11 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "A user can complete fixture, deployment, and programming workflows through on-screen controls (ROADMAP Phase 6 Success Criterion 1)"
    status: failed
    reason: "None of the 8 executed plans (06-01..06-08) built any on-screen UI for fixture patching, deployment/interface configuration, or scene/look programming (base-look/color-theme/chase/motion authoring). The frontend contains exactly six components -- SafetyCluster, LiveStatusBar, PlaybackControls, OperatorSurface, MidiPanel, KeyboardShortcuts -- covering only playback (scene switch/layer toggle/BPM/tap/evaluate), operator-surface assignment, safety, and MIDI. Fixture/deployment/programming remain CLI-only. 06-VALIDATION.md's own Per-Task Verification Map and 06-CONTEXT.md's Decisions section never scope or address this SC1 clause either -- it was never decomposed into a PLAY-XX requirement or a plan task, so no plan's absence of this UI is a plan-level defect; it is an unmet roadmap Success Criterion with no plan ever targeting it."
    missing:
      - "On-screen fixture-patch UI (or an explicit, documented ROADMAP correction narrowing SC1's wording to match the actual PLAY-01..09 requirement scope, which never mentions fixture/deployment/programming)"
      - "On-screen deployment/interface-configuration UI"
      - "On-screen scene/look programming UI (base-look, color-theme, chase, motion authoring)"
  - truth: "MIDI-mapped controls (Note/CC) actually invoke the corresponding playback or safety command when triggered -- not only visual feedback (implied by PLAY-04 'map ... input to supported playback commands' and the phase goal's 'operate ... through ... constrained generic MIDI controls')"
    status: failed
    reason: "internal/wails/svc_midi.go's dispatchToActiveSurface (the only consumer of live MIDI events once a session isn't in learn mode) resolves the matching MidiMapping, runs takeover/arming logic, and calls emitMidiFeedback -- it never calls into internal/command or any playback/safety dispatch path. Pressing a MIDI button mapped to a scene, or crossing a fader mapped to a master level, updates only the on-screen armed/ghost-marker feedback; it does not switch scenes, toggle layers, change master levels, or trigger safety actions. This is explicitly self-disclosed in 06-08-SUMMARY.md's key-decisions ('Actual PLAYBACK COMMAND DISPATCH for a mapped control ... is out of this plan's scope ... flagged as follow-up work, not silently assumed complete') and its Next Phase Readiness section, but it is not tracked in deferred-items.md and no follow-up plan/phase currently owns it. Even a successful end-of-phase UAT pass of 06-08 Task 4 would not catch this gap: its own how-to-verify script only asks the human to confirm learn/conflict/timeout copy and slider/ghost-marker visual behavior, never actual playback effect."
    artifacts:
      - path: internal/wails/svc_midi.go
        issue: "dispatchToActiveSurface (lines ~211-233) only computes takeover arming/feedback and calls emitMidiFeedback; no command.Request is ever built or executed for a matched mapping"
    missing:
      - "A dispatch step in dispatchToActiveSurface (or a new integration point) that, once a fader/button is armed/pressed, builds and executes the command.Request the mapping's ControlRef target implies (scene switch, layer enable/disable, master level set, safety trigger) -- mirroring how PlaybackService/SafetyService already dispatch the same actions"
      - "Test coverage proving a mapped MIDI event actually changes playback/safety state, not just feedback state"
behavior_unverified_items: []
human_verification:
  - test: "06-05 Task 3 (deferred): live safety cluster + live status bar behavior, INCLUDING the new CR-03 hold-to-release toggle (label flips to 'Hold to Release ...' when active) which postdates the plan's original checkpoint script"
    expected: "Status bar shows scene/layers/BPM/bar/source/output with explicit idle state and truncation+tooltip; hold-to-confirm activates AND a second hold releases; daemon-unreachable copy shows while the cluster stays interactive"
    why_human: "Visual rendering, timing feel of the hold gesture, and daemon-unreachable UX require a running desktop app; deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase (project config), consistent with 06-04's already-approved equivalent checkpoint"
  - test: "06-06 Task 3 (deferred): full on-screen + keyboard playback workflow without MIDI, and confirming keyboard shortcuts stop firing when the app loses focus (unlike the safety hotkeys)"
    expected: "Every playback action reachable both ways; keyboard action is window-scoped, not global"
    why_human: "Live focus-loss behavior and on-screen/keyboard parity require a running desktop app; deferred to end-of-phase UAT"
  - test: "06-07 Task 3 (deferred): multiple named surfaces, in-place per-item assignment (no bulk control), visible-but-locked rendering enforced server-side"
    expected: "Two surfaces created and selectable; assignment toggles are per-item only; operator preview shows assigned full-opacity/Signal-Blue and unassigned reduced-opacity/disabled, never hidden; a crafted/locked action is rejected server-side"
    why_human: "Visual treatment (opacity, Signal Blue, truncation/tooltip) and the live author/operate toggle flow require a running desktop app; deferred to end-of-phase UAT"
  - test: "06-08 Task 4 (deferred): generic MIDI learn (with conflict rejection + surface-scoped learnability) and cross-to-catch soft takeover against a real or virtual MIDI controller"
    expected: "Learn/Listening/Cancel/conflict/timeout states behave per copy; only assigned controls offer Learn; fader follows physical position pre-arm with a ghost marker and only controls after crossing; buttons act immediately with no takeover slider"
    why_human: "Requires a live or virtual MIDI port and the running desktop app. NOTE: even a fully 'approved' pass of this checkpoint does NOT verify that a learned/armed mapping actually changes playback/safety state -- see the FAILED 'MIDI dispatch' gap above, which this checkpoint's own script never asks the tester to check."
---

# Phase 6: Wails Authoring and Operator Surface Verification Report

**Phase Goal:** Authors and playback operators can complete the conventional show workflow through a responsive Wails application, keyboard, and constrained generic MIDI controls without the frontend becoming runtime authority.
**Verified:** 2026-07-23T21:21:06Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | On-screen controls expose the complete playback workflow (PLAY-01) | VERIFIED | `internal/wails/svc_playback.go` binds SwitchScene/SetLayerEnabled/SetBPM/TapTempo/Evaluate/GetState over the existing command registry (`TestPlaybackServiceEnumeratesEveryPlaybackAction` passes); `frontend/src/components/PlaybackControls/PlaybackControls.tsx` renders on-screen controls for every action; `go build ./...`, `go test ./internal/wails/...`, `npm run build` all green |
| 2 | Documented keyboard workflow reaches every playback action without MIDI hardware, window-scoped not global (PLAY-02) | VERIFIED | `frontend/src/hooks/useKeyboardWorkflow.ts` is an ordinary `addEventListener('keydown', ...)` window listener (source-reviewed, no `golang.design/x/hotkey` import) calling the same `dispatch` functions as on-screen controls; `KeyboardShortcuts.tsx` documents every binding from the same `PLAYBACK_SHORTCUTS` source of truth |
| 3 | A user can complete fixture, deployment, and programming workflows through on-screen controls (ROADMAP SC1) | **FAILED** | No fixture-patch, deployment/interface, or scene/look-programming UI exists anywhere in `frontend/src/components/` (6 components total: SafetyCluster, LiveStatusBar, PlaybackControls, OperatorSurface, MidiPanel, KeyboardShortcuts). See Gaps. |
| 4 | A show author can create multiple named operator surfaces with in-place, individual-item assignment (PLAY-03/D-01/D-02/D-03) | VERIFIED | `internal/operatorsurface/model.go` (idempotent copy-returning Assign*/Unassign*, no bulk ref type); `internal/wails/svc_surface.go` CRUD bindings; `frontend/src/components/OperatorSurface/{SurfaceList,AssignmentToggle,OperatorSurface}.tsx`; `go test ./internal/operatorsurface/... ./internal/wails/...` pass |
| 5 | Unassigned controls render visible-but-locked and the lock is enforced server-side, not only by hiding (PLAY-03/D-04, CR-01 fix) | VERIFIED | `command.Authorize` (06-01) + `SurfaceService.AuthorizeControl` (06-07) exist; **CR-01 fix (commit `1887035`) closes the previously-found bypass** by adding `authorizeSafety`/`authorizeControl` gates + `SetActiveSurface` to `SafetyService`/`PlaybackService`, wired from `OperatorSurface.tsx`'s "Preview as Operator" toggle; `TestSafetyServiceBlackoutRejectsWhenActiveSurfaceDoesNotAssignControl`, `TestPlaybackServiceSwitchSceneRejectsWhenActiveSurfaceDoesNotAssignScene` pass under `-race` |
| 6 | The live status bar always shows scene/layers/BPM/bar/controlling source/output state with an explicit idle state (PLAY-07) | VERIFIED | `internal/artnet/daemon.go`'s extended `statusPayload.Playback`; `internal/playback/engine.go`'s lock-free accessors; `frontend/src/components/LiveStatusBar/LiveStatusBar.tsx`; `go test ./internal/wails/... ./internal/artnet/... -run 'TestSafetyService\|TestStatusPayload'` pass |
| 7 | MIDI Note/CC learn is per-control, per-surface-scoped, and rejects conflicts outright without mutating the existing mapping (PLAY-04/D-05/D-06/D-07/D-08) | VERIFIED | `internal/midi/learn.go` (`ProposeMapping`), `internal/wails/svc_midi.go` (`StartLearn` authorizes via `command.Authorize` then `ProposeMapping` then `operatorsurface.AddMidiMapping`); `TestMidiServiceStartLearnPersistsMapping`, `TestMidiServiceStartLearnRejectsConflictOnSameSurfaceButNotOther`, `TestMidiServiceStartLearnRejectsUnassignedControl` all pass |
| 8 | Soft takeover is cross-to-catch (directional crossing only, never proximity) and continuous-only (PLAY-05/D-09..D-12) | VERIFIED | `internal/midi/takeover.go`'s `Update` (NaN-seeded bootstrap, `crossedUp`/`crossedDown` only, no threshold constant anywhere in the file — source-reviewed); `TestMidiServiceFaderTakeoverCrossToCatchAndButtonActsImmediately` passes; frontend `SoftTakeoverSlider.tsx` renders live position + ghost marker |
| 9 | MIDI-mapped controls actually invoke the corresponding playback/safety command when triggered (implied by PLAY-04's "map ... to supported playback commands" and the phase goal) | **FAILED** | `dispatchToActiveSurface` (`internal/wails/svc_midi.go`) only computes arming state and calls `emitMidiFeedback`; no `internal/command` dispatch exists for a matched mapping. Self-disclosed as explicitly out-of-scope follow-up work in `06-08-SUMMARY.md`, not tracked in `deferred-items.md`. See Gaps. |
| 10 | Blackout, Stop/Release-All, Grand Master, and group masters act through a daemon-resident local-priority path within one Art-Net tick, independent of a hung caller (PLAY-06/09) | VERIFIED | `internal/artnet/safety.go`'s atomic `safetyState` + pure `applyOverrides`; `Worker.tick()` applies it once before Encode; `go test -race ./internal/artnet/...` passes incl. `TestSafetyOverrideBlackoutTakesEffectDespiteSlowTarget`, concurrent-blackout race test; orchestrator's own hands-on test additionally confirmed the OS-level Blackout hotkey fires the daemon route while the window is unfocused (06-04 Task 3, already approved) |
| 11 | Revoke Automation blocks non-manual-source commands and freezes the look without requiring the automation runtime to respond (PLAY-08) | VERIFIED | `daemon.handle()`'s top-level `requestSource` gate rejects `--source automation` while `revokeActive()`; `TestRevokeAutomationBlocksNonManualSource` passes. "Cancels queued actions" is explicitly and correctly deferred (`06-02-PLAN.md`'s `flagged_assumptions`: no automation-lease/queued-action model exists until Phases 8/9 build a runtime) — legitimate, documented deferral, not a gap |
| — | Safety cluster (on-screen + hotkeys) has a symmetric release path, not activate-only (CR-03 fix) | VERIFIED | `internal/wails/hotkey.go`'s `nextToggleValue` queries daemon status and toggles; `SafetyCluster.tsx`'s `blackoutOrStopActive`-driven `onActivate={() => safetyBlackout(!blackoutOrStopActive)}`; `TestHotkeyKeydownReleasesWhenAlreadyActive` passes |
| — | `MidiService.CancelLearn` does not panic on a double call (CR-02 fix) | VERIFIED | Mutex now guards `s.learning` nil-out before `close(session.cancel)`; `TestMidiServiceCancelLearnDoubleCallDoesNotPanic` and `...ConcurrentDoubleCallDoesNotPanic` pass under `-race` |

**Score:** 9/11 core truths verified (2 FAILED — see Gaps), plus 2 additional review-fix truths verified (CR-01/CR-03), for 11/13 total including fix-verification rows. 0 behavior-unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/operatorsurface/{model,validate}.go` | Named-surface model + validation | VERIFIED | Present, wired into `show.State.validate()`, schema v1->v2 migration registered |
| `internal/command/operatorsurface.go` | CLI routes + `Authorize` | VERIFIED | create/list/assign/unassign/show/remove routes; `Authorize` used by CLI, `command.Authorize` reused by Wails services |
| `internal/artnet/safety.go` | Daemon-resident atomic overrides | VERIFIED | `applyOverrides`, atomic flags, `-race` clean |
| `internal/midi/{takeover,learn,driver}.go` | Pure logic + live gomidi driver | VERIFIED | `takeover.go`/`learn.go` pure (no gomidi import); `driver.go` wraps a caller-resolved `drivers.In`, testable via `testdrv` |
| `internal/wails/{app,hotkey,events,svc_safety,svc_playback,svc_surface,svc_midi}.go` | Go host + 4 binding services | VERIFIED | All present, filled, build clean, `go vet` clean |
| `frontend/src/components/{SafetyCluster,LiveStatusBar,PlaybackControls,OperatorSurface,MidiPanel,KeyboardShortcuts}` | Region components | VERIFIED | All present, filled, `npm run build` clean; **no fixture/deployment/programming component exists** (see Gaps) |
| `cmd/golc-desktop/main.go` | Wails entrypoint | VERIFIED | Binds `App` + all 4 services; `-tags desktop,production` build confirmed working by orchestrator's hands-on test |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `frontend/src/components/SafetyCluster/SafetyCluster.tsx` | `internal/wails/svc_safety.go` | hold-to-confirm -> `SafetyService` binding | VERIFIED | `onActivate` calls `safetyBlackout`/`safetyStopReleaseAll`/`safetyRevokeAutomation` |
| `internal/wails/hotkey.go` | `internal/artnet/ipc` | OS hotkey callback -> `ipc.Forward` directly | VERIFIED | No JS-mediated path (source-reviewed); confirmed live by orchestrator (06-04 Task 3) |
| `frontend/src/components/OperatorSurface/AssignmentToggle.tsx` | `internal/wails/svc_surface.go` | in-place toggle -> assign/unassign | VERIFIED | `AssignItem`/`UnassignItem` bindings, idempotent |
| `internal/wails/svc_midi.go` | `internal/midi/learn.go` + `internal/operatorsurface/model.go` | `ProposeMapping` -> `AddMidiMapping` | VERIFIED | Conflict-checked persistence path proven by tests |
| `internal/wails/svc_midi.go` | *(missing)* `internal/command` | armed/pressed mapping -> playback/safety dispatch | **NOT WIRED** | No such link exists; see FAILED truth #9 |
| `internal/artnet/worker.go` | `internal/artnet/safety.go` | `tick()` -> `applyOverrides` | VERIFIED | Single atomic-load path before Encode, no lock |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| PLAY-01 | 06-04, 06-06 | On-screen complete playback workflow | SATISFIED (functionally); REQUIREMENTS.md still shows Pending | Automated tests + build green; live UAT deferred (human_verification) |
| PLAY-02 | 06-06 | Documented keyboard workflow, no MIDI required | SATISFIED (functionally); REQUIREMENTS.md still shows Pending | Same as above |
| PLAY-03 | 06-01, 06-07 | Constrained operator surface | SATISFIED; REQUIREMENTS.md shows Complete (marked at 06-01, before 06-07's UI/CR-01 fix existed) | Automated tests green; CR-01 closes the authorization bypass found in review; live UAT (visual/interaction) deferred |
| PLAY-04 | 06-03, 06-08 | Map MIDI Note/CC to playback commands | **PARTIALLY SATISFIED** — mapping/learn/persistence complete; actual command dispatch on a triggered mapping is missing (see FAILED truth #9). REQUIREMENTS.md shows Complete (marked prematurely at 06-03, pure-logic-only wave, via commit `e7e6b0c`, before 06-08 built the live driver/UI and before this dispatch gap was even architecturally possible to discover) | See Gaps |
| PLAY-05 | 06-03, 06-08 | Fader soft takeover, no value jumps | SATISFIED (the crossing mechanism itself); REQUIREMENTS.md shows Complete (same premature-marking issue as PLAY-04) | Automated tests green; live UAT deferred |
| PLAY-06 | 06-02, 06-05 | Group/Grand masters, stop/release-all, blackout | SATISFIED | Automated + `-race` tests green; CR-03 adds release path |
| PLAY-07 | 06-05, 06-07 | Live status: scene/layers/BPM/bar/source/output | SATISFIED | Automated tests green; live UAT deferred |
| PLAY-08 | 06-02, 06-05 | Revoke Automation blocks/cancels/freezes/restores | SATISFIED (manual trigger + freeze); "cancels queued actions" legitimately deferred to Phases 8/9 (documented) | Automated tests green |
| PLAY-09 | 06-02, 06-04, 06-05 | Blackout independent local-priority path | SATISFIED | Automated + `-race` tests green; orchestrator confirmed live unfocused-hotkey firing |

**Orphaned requirements:** None — every PLAY-01..09 ID is claimed by at least one plan (cross-checked against REQUIREMENTS.md §Playback and Operator Surface).

**Traceability note (not a code gap, but a documentation-accuracy finding):** `.planning/REQUIREMENTS.md` marks PLAY-03/04/05 `Complete` via commits `bf497f2` (06-01) and `e7e6b0c` (06-03). Both of these commits landed *before* the corresponding Wave 3/4 UI and live-wiring plans (06-07, 06-08) even existed, and both of those later plans' own SUMMARY.md files explicitly state "this SUMMARY does not mark them complete" pending end-of-phase UAT. REQUIREMENTS.md was never corrected afterward and currently overstates completion relative to the phase's own final-wave SUMMARYs, and (for PLAY-04) relative to the dispatch gap found in this verification.

### Anti-Patterns Found

None. `grep` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented" across all phase-6-touched Go and TypeScript source (excluding tests and build output) found no debt markers. `deferred-items.md` only records two pre-existing, unrelated `internal/trace` issues explicitly out of this phase's scope.

### Code Review Fix Verification (per task instructions)

All 6 findings from `06-REVIEW.md` were confirmed fixed in the current code, with passing tests:

| Finding | Fix Commit | Verified |
|---------|-----------|----------|
| CR-01 (authorization never enforced on real dispatch paths) | `1887035` | YES — `authorizeSafety`/`authorizeControl` + `SetActiveSurface` on `SafetyService`/`PlaybackService`; tests pass under `-race` |
| CR-02 (`CancelLearn` double-close panic) | `3d69a45` | YES — mutex-guarded nil-out; double-call and concurrent-double-call tests pass under `-race` |
| CR-03 (safety cluster activate-only, no release) | `a890296` | YES — `nextToggleValue` (hotkey.go) + `blackoutOrStopActive`-driven toggle (SafetyCluster.tsx); test passes |
| WR-01 (`SetLayerEnabled` pre-read failure swallowed) | `ffd8bc9` | YES — `currentLayerRef` now returns `(uuid.UUID, error)`; propagation test passes |
| WR-02 (Google Fonts network dependency) | `17016c9` | YES — self-hosted via `@fontsource/*`; build output has zero `fonts.googleapis.com` reference |
| WR-03 (unconditional 1s polling while unreachable) | `9b2ced8` | YES — poll skips while `connectionStatus !== "connected"` |

`go build ./...`, `go vet ./...`, `go test -race ./internal/wails/... ./internal/artnet/... ./internal/midi/... ./internal/operatorsurface/...`, and `cd frontend && npm run build` were independently re-run in this verification pass and all pass.

### Human Verification Required

See frontmatter `human_verification` — four deferred `checkpoint:human-verify` items (06-05/06-06/06-07/06-08 Task 3/3/3/4), consistent with `workflow.human_verify_mode=end-of-phase`. Two equivalent items from 06-04 (desktop shell launch + daemon supervision; OS-level Blackout hotkey firing unfocused) were already confirmed live by the orchestrator and are not re-flagged.

### Gaps Summary

Two FAILED truths block a clean `passed` verdict:

1. **ROADMAP Phase 6 Success Criterion 1's fixture/deployment/programming clause is unmet.** The phase's own requirement decomposition (PLAY-01..09), CONTEXT.md decisions, and VALIDATION.md verification map never actually scope this — only "playback workflow" (PLAY-01/02) was ever assigned to a plan. This looks like an imprecision in the ROADMAP's SC1 prose (which echoes the broader "patch, program, play" framing from CONTEXT.md's Phase Boundary paragraph) rather than a plan-execution defect, since no plan was ever asked to build it. It still needs an explicit human decision: either (a) treat this as a documentation correction to SC1 (its literal fixture/deployment/programming clause was never meant to be delivered by this phase), or (b) treat it as real missing scope requiring a new plan.

2. **MIDI-mapped controls do not actually operate the show.** Learn, per-surface conflict rejection, and cross-to-catch soft-takeover arming/feedback are all real and well-tested — but no code path turns an armed/pressed mapping into an actual scene switch, layer toggle, master-level change, or safety trigger. This is honestly self-disclosed in `06-08-SUMMARY.md` as intentionally out of that plan's scope, but it is not tracked in `deferred-items.md`, is not covered by any deferred checkpoint's own verification script, and materially undercuts the phase goal's explicit promise that operators can "run a prepared show through ... constrained generic MIDI controls." This needs a follow-up plan wiring `MidiService`'s arbitrated output into `internal/command` dispatch (mirroring how `PlaybackService`/`SafetyService` already dispatch the same actions) before PLAY-04/PLAY-05 can be considered functionally complete.

Neither gap is deferred to an identifiable later phase (Phases 7-11 cover API/Scripting/AI/Windows-Release/Telemetry, none of which own fixture/deployment/programming UI or MIDI command dispatch), so neither was moved to the `deferred` section.

---

*Verified: 2026-07-23T21:21:06Z*
*Verifier: Claude (gsd-verifier)*
