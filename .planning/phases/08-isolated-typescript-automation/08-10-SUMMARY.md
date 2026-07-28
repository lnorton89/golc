---
phase: 08-isolated-typescript-automation
plan: 10
subsystem: ui
tags: [react, wails, typescript, vitest, script-execution, sse]

# Dependency graph
requires:
  - phase: 08-04
    provides: ScriptService CRUD bindings (ListScripts/GetScript/CreateScript/SaveScriptSource/DeleteScript/SetScriptProfile) and ScriptsWorkspace.tsx's library/editor scaffold
  - phase: 08-06
    provides: internal/script capability/resource-limit enforcement (Enforce, TerminationReason) and the Windows Job Object kill path
  - phase: 08-08
    provides: the internal/script ScriptEvent bus and internal/wails EventPusher's "script:event" push (log/outcome/status/terminal/gap kinds)
  - phase: 08-09
    provides: "script debug"/"script continue"/step-control CLI routes, D-03 source-mapped stack-trace parsing, and the CDP DebugBridge
provides:
  - "ScriptService.RunScript/DebugScript/StopScript/ValidateScript + four step-control Wails methods, decoding \"script run\"/\"script debug\"/\"script stop\"/\"script validate\"'s Stdout JSON into ScriptRunOutcomeView/ScriptValidationView"
  - "wailsBridge.ts wrappers (runScript/debugScript/stopScript/validateScript/continueScript/stepOverScript/stepIntoScript/stepOutScript/onScriptEvent) plus offline fallbacks"
  - "ScriptRunDialog: the Run/Debug launch dialog with pre-filled capability scope, resource preset, and Advanced numeric fields (D-07/D-09)"
  - "ScriptDebugPanel: the live append-only log/outcome stream, status chips, D-08 termination sentences, D-03 crash+expandable trace, D-12 Stopped banner with Dismiss/Run Again, D-13 no-auto-restart copy"
  - "ScriptsWorkspace Run/Debug/Validate/Stop Script toolbar wiring, live onScriptEvent subscription reduced into per-script panel state"
affects: [08-11]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Wails run/debug/stop bindings return the JSON-safe outcome view directly (RunScript/DebugScript), or the raw Result when the caller must distinguish a specific non-decode failure (StopScript's GOLC_SCRIPT_NO_ACTIVE_RUN)"
    - "A blocking backend launch call (RunScript/DebugScript only resolves once the whole run finishes) is fired detached from the launch dialog's onSubmit, with the live SSE-style event stream as the actual progress source of truth"
    - "Frontend-side termination-reason parsing translates a backend machine-readable TerminationReason string into the exact Copywriting Contract sentence, falling back generically for any cause the backend does not yet describe structurally"

key-files:
  created:
    - frontend/src/components/Scripts/ScriptRunDialog.tsx
    - frontend/src/components/Scripts/ScriptRunDialog.module.css
    - frontend/src/components/Scripts/ScriptRunDialog.test.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.module.css
    - frontend/src/components/Scripts/ScriptDebugPanel.test.tsx
  modified:
    - internal/wails/svc_script.go
    - internal/wails/svc_script_test.go
    - frontend/src/lib/wailsBridge.ts
    - frontend/src/workspaces/build/ScriptsWorkspace.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.module.css
    - frontend/src/workspaces/build/ScriptsWorkspace.test.tsx

key-decisions:
  - "ScriptRunDialog's onSubmit resolves (closing the dialog) once SetScriptProfile succeeds, then fires RunScript/DebugScript detached -- RunScript/DebugScript's own Wails call is a full blocking round trip that only resolves once the ENTIRE run finishes, so awaiting it in the dialog would keep it open/busy for the run's whole duration, blocking the live debug panel behind it."
  - "StackFrames (D-03) is derived from RunOutcome.Reason's own multi-line text (first line = summary, remaining lines = trace) both Go-side (svc_script.go's deriveStackFrames) and TS-side (ScriptsWorkspace.tsx's deriveStackFramesFromReason applied to the live terminal event) -- internal/script exposes no separately structured []StackFrame outside its debug-mode CDP path."
  - "Deadline/rate/scope termination sentences (D-08) are derived by regex-matching the backend's GOLC_SCRIPT_DEADLINE_EXCEEDED/GOLC_SCRIPT_RATE_EXCEEDED/GOLC_SCRIPT_SCOPE_DENIED Reason text into the exact Copywriting Contract sentence; a memory/CPU resource-limit kill (the Windows Job Object terminates the process directly, with no TerminationReason recorded) and an explicit user Stop fall back to a generic \"Terminated: {reason}\" rendering."
  - "DebugScript is called with an empty breakpointLines array -- 08-04's textarea editor has no gutter UI to set breakpoints from yet; 08-11's Monaco integration is the intended source for this argument."
  - "Panel event history is keyed by scriptName (a Record<string, ScriptPanelState>), not literally by run id, since v1 scope allows at most one active run globally -- this preserves a script's frozen terminal banner across a selection change without literal per-run-id bookkeeping."

patterns-established:
  - "A launch dialog backed by a fully-blocking backend RPC closes on the fast synchronous half (profile save) and treats a live event stream as the actual progress channel, catching the detached launch promise's rejection only to synthesize a terminal state for pre-flight failures."

requirements-completed: [SCRP-04, SCRP-05]

coverage:
  - id: D1
    description: "ScriptService Run/Debug/Stop/Validate/step-control Wails bindings decode the CLI routes' Stdout JSON into typed views; StopScript/step-controls surface their own specific diagnostics (GOLC_SCRIPT_NO_ACTIVE_RUN/GOLC_SCRIPT_NO_ACTIVE_DEBUG) rather than a generic decode error"
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "internal/wails/svc_script_test.go#TestScriptServiceRunScriptMissingReturnsError,TestScriptServiceDebugScriptInvalidBreakpointReturnsError,TestScriptServiceStopScriptNoActiveRunReturnsResult,TestScriptServiceValidateScriptDecodesForbiddenImportDiagnostic,TestScriptServiceValidateScriptMissingReturnsError,TestScriptServiceControlRoutesNoActiveDebugReturnsResult"
        status: pass
      - kind: unit
        ref: "internal/wails/svc_script_test.go#TestScriptServiceRunScriptSucceeds,TestScriptServiceRunScriptCrashDerivesStackFrames,TestScriptServiceDebugScriptSucceeds (Deno-gated, skip in this worktree -- no provisioned .tools/toolchains/deno/)"
        status: unknown
    human_judgment: true
    rationale: "The three Deno-gated tests exercising a real successful run, a real crash, and a real debug launch did not execute in this worktree (no provisioned Deno toolchain) -- they compile and are wired to skip gracefully, but their pass/fail against a real Deno process is unverified here."
  - id: D2
    description: "ScriptRunDialog: mode-specific title/CTA, pre-filled-never-blank capability scope and resource preset, Advanced-gated numeric fields, Escape/backdrop/Cancel close without launching, busy state while submitting, inline error on a rejected submit"
    requirement: "SCRP-04"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptRunDialog.test.tsx (11 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ScriptDebugPanel: pre-first-run placeholder, live log/outcome append-in-order rendering, status chips, D-08 termination sentences, D-03 crash+expandable trace, D-12 Stopped banner persistence, D-13 no-auto-restart copy, D-11 stopping transient, gap resync notice"
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptDebugPanel.test.tsx (15 tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "ScriptsWorkspace toolbar wiring: Run/Debug/Validate/Stop Script actions, launch-dialog round trip (save profile then detached launch), live onScriptEvent subscription reduced into panel state, Validate-gated Run/Debug disabling, Run Again re-opens the dialog, Dismiss clears the frozen terminal state"
    requirement: "SCRP-05"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (16 tests, 7 new for this plan)"
        status: pass
    human_judgment: false
  - id: D5
    description: "A live launch against a real provisioned Deno toolchain (deadline overrun, Run Again re-opening the dialog rather than relaunching directly) -- this plan's own <verification> manual-transcript bullet"
    verification: []
    human_judgment: true
    rationale: "No Deno toolchain is provisioned in this worktree (.tools/toolchains/deno/ absent); this manual end-to-end transcript requires a real Wails runtime and a bootstrapped Deno install neither available here."

duration: ~55min
completed: 2026-07-25
status: complete
---

# Phase 8 Plan 10: Run/Debug launch dialog, live debug panel, and workspace toolbar wiring Summary

**Run/Debug/Stop/Validate Wails bindings, a pre-filled capability-profile launch dialog, and a live append-only debug panel with D-08 termination sentences, D-03 crash traces, and a persistent D-12/D-13 Stopped banner, wired into ScriptsWorkspace's toolbar.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-07-25
- **Tasks:** 3
- **Files modified:** 12 (6 created, 6 modified)

## Accomplishments

- `ScriptService.RunScript/DebugScript/StopScript/ValidateScript` plus the four step-control methods (`ContinueScript`/`StepOverScript`/`StepIntoScript`/`StepOutScript`) decode the already-implemented `"script run"`/`"script debug"`/`"script stop"`/`"script validate"` CLI routes' Stdout JSON into JSON-safe `ScriptRunOutcomeView`/`ScriptValidationView`, extended `wailsBridge.ts` with matching wrappers, offline fallbacks, and `onScriptEvent` (subscribing to the `"script:event"` push).
- `ScriptRunDialog`: mode-specific ("Run {script}"/"Debug {script}") launch dialog following `HelpOverlay.tsx`'s backdrop+dialog pattern, always pre-filled from the saved capability profile, Advanced-gated numeric limit fields, busy/error states, Escape/backdrop/Cancel close without launching.
- `ScriptDebugPanel`: append-only live log/outcome stream, status chips (`Running`/`Paused at breakpoint — line {N}`/`Terminated`/`Crashed`/`Offline`), the exact D-08 deadline/rate/scope termination sentences, a D-03 crash summary with a collapsed-by-default expandable stack trace, the D-11 "Stopping" transient copy, a visible resync notice for a gap event, and the D-12 persistent `"Stopped: {reason}"` banner with Dismiss/Run Again plus the D-13 no-auto-restart disclaimer.
- `ScriptsWorkspace.tsx` now wires Run/Debug/Validate/Stop Script toolbar actions (standard 32px `Button` height, explicitly not Phase 6's 64px safety cluster), a live `onScriptEvent` subscription reduced into per-script panel state, and the launch-dialog round trip (save profile, then a detached launch call so the dialog closes promptly and the debug panel takes over live).

## Task Commits

Each task was committed atomically:

1. **Task 1: Run/Debug/Stop service methods and bridge wrappers** - `2b26e3e` (feat)
2. **Task 2: Run/Debug launch dialog with capability profile and resource presets** - `b17b7d4` (feat)
3. **Task 3: Live debug panel, terminal states, and workspace toolbar actions** - `15ea985` (feat)

_All three tasks were TDD (`tdd="true"`): tests were written alongside each implementation and verified green (`go test ./internal/wails/... -count=1`, `npm --prefix frontend run test`) before commit; no separate RED-only commit was created per task since the plan did not require a `type: tdd` plan-level RED/GREEN/REFACTOR gate sequence (this is an `execute`-type plan with `tdd="true"` on individual tasks)._

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `internal/wails/svc_script.go` - Adds `RunScript`/`DebugScript`/`StopScript`/`ValidateScript`/`ContinueScript`/`StepOverScript`/`StepIntoScript`/`StepOutScript`, `ScriptRunOutcomeView`/`ScriptValidationView`, and `deriveStackFrames`
- `internal/wails/svc_script_test.go` - 13 new tests covering every method, Deno-gated where a real run/crash/debug launch is needed
- `frontend/src/lib/wailsBridge.ts` - `ScriptRunOutcomeView`/`ScriptValidationView`/`ScriptEventView` types, `runScript`/`debugScript`/`stopScript`/`validateScript`/`continueScript`/`stepOverScript`/`stepIntoScript`/`stepOutScript`/`onScriptEvent` wrappers, offline fallbacks
- `frontend/src/components/Scripts/ScriptRunDialog.tsx` + `.module.css` + `.test.tsx` - the Run/Debug launch dialog
- `frontend/src/components/Scripts/ScriptDebugPanel.tsx` + `.module.css` + `.test.tsx` - the live debug/log panel
- `frontend/src/workspaces/build/ScriptsWorkspace.tsx` + `.module.css` + `.test.tsx` - toolbar actions, dialog/panel wiring, live event reducer

## Decisions Made

See `key-decisions` in frontmatter above. In short: the launch dialog closes on the fast profile-save half rather than awaiting the full (blocking) run; D-03's stack frames and D-08's termination sentences are both derived client-side (and Go-side, for the RunScript/DebugScript return value) from the backend's existing unstructured `Reason` text, since `internal/script` does not expose a separately structured limit-name/value or `[]StackFrame` outside its debug-mode CDP path; and `DebugScript` is called with no breakpoints pending 08-11's Monaco gutter.

## Deviations from Plan

None - plan executed exactly as written for every explicit `<behavior>`/`<action>` instruction. The items above under "Decisions Made" are implementation choices made within the plan's own flagged `verification: backstop` gray areas (the exact spawn-success signal, the exact resync-notice copy, and the exact banner-reason substitution were all explicitly left unlocked by 08-UI-SPEC.md), not corrections to anything the plan specified.

## Known Stubs

- `ScriptDebugPanel`'s Advanced/custom field validation and the "Run/Debug dialog submit fails" inline treatment both follow the plan's own flagged backstop truths — implemented with sensible client-side `min`/`max`/`step` constraints and a generic inline error, but neither is independently verified against a representative long-running scenario or a real subprocess-spawn failure in this session.
- `DebugScript(name, [])` always launches with zero breakpoints — this is accurate to the current editor (a plain `<textarea>` with no gutter), not a broken feature; 08-11's Monaco integration is the documented source for this argument going forward (see the doc comment at the top of `ScriptsWorkspace.tsx`).

## Issues Encountered

A test-only race: three new `ScriptsWorkspace.test.tsx` tests clicked a toolbar button immediately after the script list became visible, but the selection-validity-repair effect that auto-selects the first script (and thereby enables Run/Debug/Validate) can land in a later render tick than the list's own render — the click could land on a still-disabled button. Fixed by waiting for the target button's `not.toBeDisabled()` state before clicking, in every affected test; re-ran the full file three times with no further flakiness observed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SCRP-04's pre-execution assignment surface (capability scope, resource preset, Advanced limits) exists in the GUI, pre-filled and editable, and SCRP-05's logs/diagnostics/source-location/command-outcome/cancellation-status surfaces are all visible live.
- D-03, D-04, D-05, D-07, D-08, D-09, D-10, D-12, and D-13 are each visibly implemented in the UI per this plan's `<success_criteria>`.
- 08-11 (Monaco editor) has a clear handoff: the `<textarea>` element and `DebugScript`'s empty breakpoint list are both explicitly marked for in-place replacement, and the D-01 breakpoint-gutter UI this plan's DebugScript call is missing is 08-11's to add.
- The three Deno-gated Go tests and the plan's own manual end-to-end verification transcript (a real deadline-overrun launch) remain unverified against a real Deno toolchain in this specific worktree/session — recommend running `go test ./internal/wails/... -count=1` and the manual transcript once Deno is provisioned (`mage Bootstrap`) or in CI.

## Self-Check: PASSED

- All 12 created/modified source files plus this SUMMARY.md verified present on disk (`git ls-files`).
- All four commits (`2b26e3e`, `b17b7d4`, `15ea985`, `8c4a69b`) verified present in `git log --oneline --all`.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-25*
