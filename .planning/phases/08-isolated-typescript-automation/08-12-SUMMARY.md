---
phase: 08-isolated-typescript-automation
plan: 12
subsystem: ui
tags: [monaco-editor, react, wails, debugger, typescript-automation, vitest]

# Dependency graph
requires:
  - phase: 08-isolated-typescript-automation
    provides: "08-09's internal/script/debugbridge.go CDP debug bridge and internal/command/scriptdebug.go's five debug/step routes; 08-11's real Monaco ScriptEditor with glyphMargin:true scaffolded and monacoTheme.ts's --status-armed/--status-revoked mapping"
provides:
  - "ScriptEditor.tsx: breakpointLines/onToggleBreakpoint/currentExecutionLine props -- a glyph-margin breakpoint gutter and a paused/selected current-execution-line highlight, both maintained via Monaco IEditorDecorationsCollection.set() full-replacement semantics"
  - "ScriptDebugPanel.tsx: pausedLine as an explicit prop, the four Continue/Step Over/Step Into/Step Out controls (rendered only while paused, never optimistic), and per-frame keyboard-focusable stack-trace buttons calling onSelectFrame(line)"
  - "ScriptsWorkspace.tsx: single pausedLine derivation point (reduceScriptEvent), breakpointLines state sent verbatim to DebugScript on launch, the four step-control bridge handlers, and handleSelectFrame reusing currentExecutionLine to reveal a clicked crash frame"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Monaco decoration sets are replaced wholesale via IEditorDecorationsCollection.set(fullList), never appended to -- both the breakpoint-glyph collection and the current-execution-line collection follow this, created once per editor mount and fed the freshest ref/prop value at mount completion (mirroring ScriptEditor.tsx's existing value/readOnly/sdkTypeDefinitions ref-mirroring pattern from 08-11)."
    - "ScriptsWorkspace.tsx is the single derivation point for pausedLine (parsed from GOLC_SCRIPT_DEBUG_PAUSED: line=<N> in a live script.status event's Reason, inside reduceScriptEvent) -- the same value feeds both ScriptEditor's currentExecutionLine and ScriptDebugPanel's paused chip/step-control gate, so the two surfaces can never independently drift. ScriptDebugPanel no longer derives pausedLine itself."
    - "A clicked crash-trace frame's line is revealed by reusing the same currentExecutionLine prop/decoration mechanism Task 1 built for the paused-run highlight (ScriptsWorkspace's currentExecutionLine={panelState.pausedLine ?? selectedFrameLine}) -- no new imperative ScriptEditor API (ref/cursor method) was added, since ScriptEditor.tsx was outside Task 2's declared file scope."

key-files:
  created: []
  modified:
    - frontend/src/components/Scripts/ScriptEditor.tsx
    - frontend/src/components/Scripts/ScriptEditor.module.css
    - frontend/src/components/Scripts/ScriptEditor.test.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.module.css
    - frontend/src/components/Scripts/ScriptDebugPanel.test.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.test.tsx

key-decisions:
  - "ScriptDebugPanel's pausedLine moved from an internal events-scan (mostRecentPausedLine) to an explicit required prop, so ScriptsWorkspace becomes the one place that ever parses GOLC_SCRIPT_DEBUG_PAUSED -- the same value now drives both ScriptEditor's highlight and ScriptDebugPanel's chip/controls instead of two independent derivations that could disagree."
  - "handleSelectFrame reuses the existing currentExecutionLine mechanism (via a new selectedFrameLine workspace state, deferring to panelState.pausedLine whenever a run is actually paused) rather than adding a ScriptEditor ref/imperative cursor API -- Task 2's declared files_modified excludes ScriptEditor.tsx, and this design satisfies the 'moves the editor cursor to that line' behavior bullet without touching it."
  - "Stopping a paused run (handleStop) clears pausedLine synchronously in the same setState call that sets liveStatus to 'stopping', rather than waiting for the guaranteed terminal event -- the plan's own must_haves calls out that Stop Script clears the execution-line highlight as part of its single-click contract."

requirements-completed: [SCRP-01, SCRP-05]

coverage:
  - id: D1
    description: "Glyph-margin mouse-down on line N calls onToggleBreakpoint(N) exactly once; the line-number column and text area never do. Zero/one/many breakpoints render 0/1/N identical glyph decorations, replaced (not accumulated) on every breakpointLines change, all cleared on unmount."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptEditor.test.tsx (6 new tests: glyph mouse-down filtering, empty breakpoints, identical glyph class for 1 vs 3, replace-not-accumulate across 3 changes, current-execution-line apply/clear, decoration cleanup on unmount)"
        status: pass
    human_judgment: false
  - id: D2
    description: "On a paused event the panel renders 'Paused at breakpoint — line {N}' plus exactly Continue/Step Over/Step Into/Step Out; each control calls its own bridge function exactly once; controls are absent with no active debug run and during a plain Run; nothing is cleared optimistically on click."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptDebugPanel.test.tsx (6 new tests: no-control idle/running, exactly-four-while-paused + each callback, non-optimistic click, onSelectFrame with/without a parseable frame line)"
        status: pass
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (5 new tests: breakpoint gutter -> DebugScript(name, [4, 9]), step-control visibility across idle/running/paused + editor highlight, each step control -> its own ContinueScript/StepOverScript/StepIntoScript/StepOutScript call, terminal event clears paused line while log + banner survive, stack-frame click reveals its line)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ScriptsWorkspace passes the gutter's breakpoint lines to debugScript on Debug launch, derives pausedLine from live script.status events as the single source of truth for both ScriptEditor's currentExecutionLine and ScriptDebugPanel, and a terminal event clears currentExecutionLine/step controls while log history and the stopped banner survive (D-12)."
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx#'toggles a breakpoint via the editor's gutter and sends the exact set to DebugScript on Debug launch', #'a terminal event clears the paused line and step controls while the log history and stopped banner survive'"
        status: pass
    human_judgment: false
  - id: D4
    description: "Manual verification on a live Wails webview (Windows): set a gutter breakpoint, Debug, confirm the pause lands on that line with the armed highlight, step over twice, continue to completion, then repeat with a deliberate crash and click a trace frame to jump to its line."
    verification: []
    human_judgment: true
    rationale: "This execution ran headless inside a git worktree with no Wails runtime/webview available (no computer-use/browser tooling attached to this session) -- the plan's own <verification> block calls this out as a required manual check on Windows. Automated coverage (D1-D3 above) proves every prop/decoration/callback wiring is correct against a faithful Monaco fake; it does not substitute for watching a real paused Deno --inspect-brk process actually highlight the right line in a live editor. This also inherits 08-09's still-open real-Deno/real-CDP verification gap (see 08-09-SUMMARY.md's Next Phase Readiness and deferred-items.md's '## 08-09' section) -- a human/CI run on a bootstrapped Windows machine must confirm this end-to-end before D-01's real debugger is treated as fully live-verified."

duration: ~40min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 12: Debugger UI — Breakpoint Gutter, Paused State, and Step Controls Summary

**Glyph-margin breakpoint gutter and paused/selected current-execution-line highlight in Monaco, plus ScriptDebugPanel's four step controls and clickable stack-trace frames, all driven by ScriptsWorkspace's single pausedLine derivation from live script.status events.**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-07-26
- **Tasks:** 2 completed
- **Files modified:** 8 (0 created, 8 modified)

## Accomplishments

- `ScriptEditor.tsx` gained `breakpointLines`/`onToggleBreakpoint`/`currentExecutionLine` props: a glyph-margin mouse-down (filtered to exactly `MouseTargetType.GUTTER_GLYPH_MARGIN`, never the line-number column or text area) toggles a breakpoint; two `IEditorDecorationsCollection` instances (breakpoints, current-execution-line) always `.set()` the full replacement list, never accumulate, and are cleared alongside the existing editor/model disposal on unmount.
- `ScriptEditor.module.css` adds `.breakpointGlyph` (a `--status-revoked` dot, identical regardless of count) and `.currentExecutionLine` (a `--status-armed` whole-line highlight).
- `ScriptDebugPanel.tsx` now takes `pausedLine` as an explicit prop (its own internal `mostRecentPausedLine` event-scan was removed) plus `onContinue`/`onStepOver`/`onStepInto`/`onStepOut`/`onSelectFrame`: the four step controls render only while `status === "paused"`, are absent during a plain Run or with no active debug run, and are never cleared optimistically on click — only a fresh prop (the backend's own next event) changes what renders. Each expanded stack-trace frame is now its own keyboard-focusable `<button>` that calls `onSelectFrame` with the frame's parsed author-coordinate line (`:LINE:COL)` trailing-pattern match), a no-op for an unparseable frame (e.g. a shim-marker frame).
- `ScriptsWorkspace.tsx` is the single derivation point for `pausedLine` (folded into `ScriptPanelState` by `reduceScriptEvent`, parsing the same `GOLC_SCRIPT_DEBUG_PAUSED: line=<N>` marker the removed `ScriptDebugPanel` helper used), holds the gutter's own `breakpointLines` state (reset on script selection change) and sends it verbatim as `debugScript(name, breakpointLines)`'s second argument on Debug launch, wires the four step-control bridge calls (`continueScript`/`stepOverScript`/`stepIntoScript`/`stepOutScript`), and reveals a clicked crash frame's line by feeding it through the same `currentExecutionLine` prop Task 1 built (`panelState.pausedLine ?? selectedFrameLine`) rather than adding a new imperative editor API.
- Stopping a paused run (`handleStop`) now also clears `pausedLine` synchronously in the same state update that sets `liveStatus: "stopping"`, matching the plan's must_haves ("Stopping a paused debug run... clears the execution-line highlight").

## Task Commits

Each task was committed atomically:

1. **Task 1: Glyph-margin breakpoint gutter and current-execution-line highlight** - `4d80441` (feat)
2. **Task 2: Step controls, paused state, and clickable stack-trace frames** - `0c866f9` (feat)

**Plan metadata:** committed by the wave orchestrator after merge (worktree mode: this agent does not update STATE.md/ROADMAP.md).

_Both tasks were TDD (`tdd="true"`): tests were written alongside each implementation and verified green (`npm --prefix frontend run test -- Script`, `npm --prefix frontend run build`) before each task's single commit; this is an `execute`-type plan with `tdd="true"` on individual tasks, not a plan-level RED/GREEN/REFACTOR gate — see "TDD Gate Compliance" below._

## Files Created/Modified

- `frontend/src/components/Scripts/ScriptEditor.tsx` — `breakpointLines`/`onToggleBreakpoint`/`currentExecutionLine` props, glyph-margin mouse-down handler, two decoration collections
- `frontend/src/components/Scripts/ScriptEditor.module.css` — `.breakpointGlyph`/`.currentExecutionLine` classes
- `frontend/src/components/Scripts/ScriptEditor.test.tsx` — extended Monaco fake (`onMouseDown`, `createDecorationsCollection`, `MouseTargetType`) plus 6 new tests
- `frontend/src/components/Scripts/ScriptDebugPanel.tsx` — `pausedLine` prop (replacing internal derivation), four step controls, clickable stack-trace frames, `stackFrameLine` parser
- `frontend/src/components/Scripts/ScriptDebugPanel.module.css` — `.stepControls`/`.traceList`/`.traceFrame` classes (replacing `.tracePre`)
- `frontend/src/components/Scripts/ScriptDebugPanel.test.tsx` — updated paused-chip test to pass `pausedLine` directly, plus 6 new tests
- `frontend/src/workspaces/build/ScriptsWorkspace.tsx` — `breakpointLines`/`selectedFrameLine` state, `pausedLine` derivation in `reduceScriptEvent`, step-control handlers, `handleSelectFrame`, updated `ScriptEditor`/`ScriptDebugPanel` render wiring
- `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx` — Monaco fake extended to match `ScriptEditor.test.tsx`'s (Rule 3, committed with Task 1), `ContinueScript`/`StepOverScript`/`StepIntoScript`/`StepOutScript` added to `stubScriptService`, plus 5 new end-to-end tests

## Decisions Made

See `key-decisions` in frontmatter above. In short: `pausedLine` moved from `ScriptDebugPanel`'s own internal derivation to an explicit prop so `ScriptsWorkspace` is the single source of truth feeding both `ScriptEditor` and `ScriptDebugPanel`; a clicked crash frame reuses the existing `currentExecutionLine` highlight mechanism rather than adding a new `ScriptEditor` API (out of Task 2's declared file scope); and `handleStop` clears `pausedLine` synchronously rather than waiting for the terminal event, matching the plan's explicit must_haves wording.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `ScriptsWorkspace.test.tsx`'s separate `monaco-editor` fake lacked `onMouseDown`/`createDecorationsCollection`**
- **Found during:** Task 1, `npm --prefix frontend run build`
- **Issue:** `ScriptsWorkspace.test.tsx` maintains its own independent `vi.mock("monaco-editor", ...)` fake (documented reason: Monaco cannot instantiate under jsdom). Task 1's `ScriptEditor.tsx` unconditionally calls `editor.onMouseDown(...)` and `editor.createDecorationsCollection(...)` on every mount; the older fake's `create()` return value had neither, so every test mounting `ScriptsWorkspace` (which always mounts `ScriptEditor`) threw `TypeError: editor.onMouseDown is not a function` as an unhandled rejection, failing the build's `vitest run` step even though `ScriptsWorkspace.test.tsx` isn't in Task 1's declared `files_modified`.
- **Fix:** Extended that fake's `create()` return value with the same minimal `onMouseDown`/`createDecorationsCollection`/`MouseTargetType` surfaces `ScriptEditor.test.tsx`'s own fake exposes (no-op-sufficient for Task 1; Task 2 later upgraded these to fully tracking fakes — see below — to write its own end-to-end breakpoint/pausedLine tests against this same file).
- **Files modified:** `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx`
- **Verification:** `npm --prefix frontend run build` passes cleanly (tsc + full `vitest run` + `vite build`).
- **Committed in:** `4d80441` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 3, blocking — a pre-existing duplicated Monaco test fake required updating to keep the whole-suite build green, discovered while completing Task 1's own acceptance criteria).
**Impact on plan:** The fix was necessary for Task 1's own stated `npm --prefix frontend run build` acceptance criterion to pass; Task 2 then extended the same fake further (adding full decoration/mouse-event tracking) as part of its own declared file scope, so no duplicate work resulted.

## TDD Gate Compliance

Both tasks landed as a single `feat` commit rather than a separate `test(...)` (RED) then `feat(...)` (GREEN) pair. Both tasks extend existing components whose test files already `vi.mock("monaco-editor", ...)` against the real component under test — writing the new assertions first against the pre-Task-1/pre-Task-2 component shape would fail to compile (new props like `breakpointLines`/`pausedLine` don't exist yet), not merely fail at runtime, making a genuine compile-and-fail RED step impractical without discarding and re-adding implementation files purely for commit-history ceremony (the same reasoning 08-09-SUMMARY.md recorded for its own single-commit tasks). Every test was written and run to green before each task's single commit; the actual RED/GREEN discipline (tests exist, verify real behavior, pass) was followed — only the two-commit *shape* was not.

## Issues Encountered

- `go run ./cmd/golc-project generate` was run after both tasks landed and produced no diff — this plan is frontend-only UI wiring on top of already-existing backend routes (`ContinueScript`/`StepOverScript`/`StepIntoScript`/`StepOutScript`/`DebugScript` were all already declared in `wailsBridge.ts` and bound Go-side from 08-09/08-10), so no generated artifact needed updating.
- This worktree has no live Wails webview/computer-use tooling attached (headless git worktree execution), so the plan's own `<verification>` block's manual Windows check (set a gutter breakpoint, Debug, confirm the pause lands on that line with the `armed` highlight, step over twice, continue, then repeat with a deliberate crash and click a trace frame) could not be performed — recorded as coverage item D4 (`human_judgment: true`), inheriting the same open real-Deno/real-CDP verification gap 08-09-SUMMARY.md already flagged for this worktree's unprovisioned Deno toolchain.
- `npm --prefix frontend run test -- Script` (70 tests across `ScriptEditor`, `ScriptDebugPanel`, `ScriptRunDialog`, `ScriptsWorkspace`) and the full `npm --prefix frontend run build` (`tsc --noEmit && vitest run && vite build`) both pass cleanly with zero unrelated failures.

## User Setup Required

None - no external service configuration required. (The unperformed manual Windows verification in D4 requires a bootstrapped Deno toolchain and a live Wails webview, which is standard project setup per `mage Bootstrap`, not a new external service.)

## Next Phase Readiness

- D-01's real breakpoint/step debugger is now fully reachable end-to-end from the UI: a user can click the gutter to set breakpoints, launch Debug, watch the pause land with the `armed` highlight, step through with conventional controls, and click a crash's stack-trace frames to navigate.
- The one open gap is D4's manual Windows verification against a real paused Deno `--inspect-brk` process — the same gap 08-09-SUMMARY.md already flagged; a human/CI run on a bootstrapped Windows machine should close both together before treating D-01 as fully live-verified.
- No further Phase 8 plans are known to depend on this plan's own new surface (`ScriptEditor`'s breakpoint/current-line props, `ScriptDebugPanel`'s step controls) beyond what 08-11-SUMMARY.md already flagged this plan itself would consume.

## Self-Check: PASSED

- FOUND: frontend/src/components/Scripts/ScriptEditor.tsx (modified)
- FOUND: frontend/src/components/Scripts/ScriptEditor.module.css (modified)
- FOUND: frontend/src/components/Scripts/ScriptEditor.test.tsx (modified)
- FOUND: frontend/src/components/Scripts/ScriptDebugPanel.tsx (modified)
- FOUND: frontend/src/components/Scripts/ScriptDebugPanel.module.css (modified)
- FOUND: frontend/src/components/Scripts/ScriptDebugPanel.test.tsx (modified)
- FOUND: frontend/src/workspaces/build/ScriptsWorkspace.tsx (modified)
- FOUND: frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (modified)
- FOUND commit: 4d80441 (feat: glyph-margin breakpoint gutter and current-execution-line highlight)
- FOUND commit: 0c866f9 (feat: step controls, paused-state wiring, and clickable stack-trace frames)
- `npm --prefix frontend run test -- Script`: PASS (70/70)
- `npm --prefix frontend run build`: PASS (tsc + vitest + vite build)
- `grep -c 'status-revoked' frontend/src/components/Scripts/ScriptEditor.module.css`: 2
- `grep -c 'status-armed' frontend/src/components/Scripts/ScriptEditor.module.css`: 2
- `grep -c 'Paused at breakpoint' frontend/src/components/Scripts/ScriptDebugPanel.tsx`: 1
- `grep -c 'Step Over' frontend/src/components/Scripts/ScriptDebugPanel.tsx`: 1 (same for "Step Into"/"Step Out")
- `go run ./cmd/golc-project generate`: no diff (no API-surface change)

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
