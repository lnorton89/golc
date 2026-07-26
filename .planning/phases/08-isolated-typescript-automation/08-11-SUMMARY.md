---
phase: 08-isolated-typescript-automation
plan: 11
subsystem: ui
tags: [monaco-editor, react, wails, typescript, vitest, vite, code-editor]

# Dependency graph
requires:
  - phase: 08-03
    provides: internal/scriptsdk's committed internal/scriptsdk/generated/golc.d.ts (ambient, zero-import GOLC SDK types)
  - phase: 08-07
    provides: internal/wails.ScriptService and ScriptsWorkspace.tsx's D-16 library/editor scaffold (the placeholder textarea this plan replaces)
  - phase: 08-10
    provides: ScriptsWorkspace.tsx's Run/Debug/Validate/Stop toolbar, ScriptRunDialog, ScriptDebugPanel, and the "script:event" live stream this plan's editor sits beside
provides:
  - "monaco-editor pinned at the human-approved exact version 0.55.1, no range prefix"
  - "monacoTheme.ts: buildGolcMonacoThemes()/resolveThemeName() mapping Monaco's editor/token colours onto the exact index.css Paper/Ink custom properties, for both colour schemes"
  - "ScriptService.GetSDKTypeDefinitions + wailsBridge.getSdkTypeDefinitions(): serves the committed golc.d.ts to the frontend"
  - "ScriptEditor.tsx: a real Monaco instance replacing 08-04's placeholder textarea, registering golc.d.ts as a TypeScript extra lib for live autocomplete/diagnostics, theme-synced to prefers-color-scheme, disposed cleanly on unmount"
affects: [08-12]

# Tech tracking
tech-stack:
  added: ["monaco-editor@0.55.1"]
  patterns:
    - "monaco-editor is imported dynamically inside a mount effect (never a static top-level import) and wrapped in try/catch, because the real package touches document.queryCommandSupported at its own module-evaluation time -- a browser-only API jsdom does not implement. A static import would crash every jsdom test that happens to render this component's tree, including app-wide smoke/navigation tests with no reason to know they need to mock it. This mirrors wailsBridge.ts's own graceful-degradation contract at the editor layer."
    - "Worker loading (self.MonacoEnvironment.getWorker) is similarly guarded behind `typeof Worker === \"undefined\"`, since jsdom does not implement the Worker global at all and Vite's `?worker` wrapper module extends it at its own module-evaluation time."
    - "monaco-editor@0.55.1 moved the TypeScript language-service surface to a new top-level `typescript` export; `monaco.languages.typescript` is deprecated in this pinned version (typed as `{ deprecated: true }`) -- verified directly against this version's own .d.ts rather than assumed from older docs/examples."
    - "value/readOnly/sdkTypeDefinitions props are mirrored into refs, read by the async mount effect at completion time (not the effect's own closured props) -- Monaco's dynamic import resolves on a later microtask than the render that scheduled it, so a prop can already be stale by the time mounting finishes; refs give correctness without a `ready`-dependent follow-up effect racing a still-stale prop closure."

key-files:
  created:
    - frontend/src/components/Scripts/monacoTheme.ts
    - frontend/src/components/Scripts/monacoTheme.test.ts
    - frontend/src/components/Scripts/ScriptEditor.tsx
    - frontend/src/components/Scripts/ScriptEditor.module.css
    - frontend/src/components/Scripts/ScriptEditor.test.tsx
  modified:
    - frontend/package.json
    - frontend/package-lock.json
    - frontend/vite.config.ts
    - frontend/src/lib/wailsBridge.ts
    - internal/wails/svc_script.go
    - internal/wails/svc_script_test.go
    - frontend/src/workspaces/build/ScriptsWorkspace.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.test.tsx

key-decisions:
  - "Human decision (Task 1 checkpoint, recorded verbatim): \"pin 0.55.1\" -- confirmed official github.com/microsoft/monaco-editor repository, ~7.9M weekly downloads, and a publish date (2025-11-20, same-day patch of 0.55.0) well predating the SUS-flagged 0.56.0 (published 2026-07-20, 5 days before 08-RESEARCH.md's audit ran). Verified again immediately before install: `npm view monaco-editor@0.55.1 version dist.integrity repository` returned the matching version/integrity and the same official repository URL."
  - "monaco-editor is imported dynamically inside ScriptEditor's mount effect, not statically at module top level -- a static import crashed every jsdom test rendering this component's tree (App.smoke.test.tsx, AppShell.navigation.test.tsx) because the real package touches `document.queryCommandSupported` at module-evaluation time. A caught dynamic-import failure degrades to an inline \"failed to load\" placeholder instead."
  - "vite.config.ts sets `worker: { format: \"es\" }`; ScriptEditor.tsx wires self.MonacoEnvironment.getWorker via Vite's `?worker` import suffix against monaco-editor@0.55.1's own editor.worker.js/ts.worker.js entry points, guarded behind `typeof Worker !== \"undefined\"` so the import never runs under jsdom."
  - "monacoTheme.ts imports monaco-editor's TYPES only (`import type`), never its runtime module, so monacoTheme.test.ts needs no mock at all -- only ScriptEditor.test.tsx (and ScriptsWorkspace.test.tsx, which mounts it) need to vi.mock(\"monaco-editor\")."

patterns-established:
  - "A component wrapping a heavyweight, browser-only third-party editor library imports it dynamically and catches load failure, rather than a static import that would propagate a load failure into a full render crash anywhere the component appears in the tree."

requirements-completed: [SCRP-01]

coverage:
  - id: D1
    description: "monaco-editor pinned at the human-approved exact version (0.55.1, no caret/tilde), following the repo's exact-version convention"
    requirement: "SCRP-01"
    verification:
      - kind: other
        ref: "frontend/package.json (`\"monaco-editor\": \"0.55.1\"`); `npm view monaco-editor@0.55.1 version dist.integrity repository` re-confirmed immediately before install"
        status: pass
    human_judgment: true
    rationale: "This is the blocking-human package-legitimacy gate itself (T-08-SC-M) -- the human's exact reply (\"pin 0.55.1\") is the record of that judgment, not something automation can retroactively self-certify."
  - id: D2
    description: "buildGolcMonacoThemes()/resolveThemeName() map Monaco's editor background/foreground/selection/gutter colours onto index.css's exact Paper/Ink hex values, per colour scheme, with no runtime monaco-editor import required to test them"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/monacoTheme.test.ts (5 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "ScriptService.GetSDKTypeDefinitions reads the committed golc.d.ts, surfacing GOLC_SCRIPTSDK_TYPES_MISSING when absent; wailsBridge.getSdkTypeDefinitions() degrades to an empty string rather than throwing when the bridge is unavailable"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "internal/wails/svc_script_test.go#TestScriptServiceGetSDKTypeDefinitionsReadsCommittedFile,TestScriptServiceGetSDKTypeDefinitionsMissingReturnsError"
        status: pass
    human_judgment: false
  - id: D4
    description: "ScriptEditor mounts Monaco with the reconciled Paper/Ink editor options (JetBrains Mono 14px/21px line height, minimap off, glyph margin + line numbers on), registers the SDK extra lib exactly once per distinct value (not per keystroke), calls onChange on model edits, switches theme on prefers-color-scheme without remounting, and disposes editor+model cleanly on unmount with no leaked second instance"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/components/Scripts/ScriptEditor.test.tsx (6 tests)"
        status: pass
    human_judgment: false
  - id: D5
    description: "ScriptsWorkspace renders ScriptEditor in place of the 08-04 placeholder textarea, fetching golc.d.ts once via getSdkTypeDefinitions; every pre-existing workspace test still passes against the new (mocked-in-test) editor"
    requirement: "SCRP-01"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (16 tests, all pre-existing, none removed)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Manual verification: typing a bare `golc.` in a live Wails webview offers real GOLC capabilities via autocomplete, and a wrong argument type is underlined live; both Paper/Ink themes render correctly on their respective colour scheme"
    verification: []
    human_judgment: true
    rationale: "This execution ran headless inside a git worktree with no Wails runtime/webview available (no computer-use/browser tooling attached to this session) -- the plan's own <verify> block calls this out as a required manual check; it is unperformed here and must be completed by a human (or a later session with a live Wails instance) before this plan's D-15 delivery is fully signed off. Automated coverage (D2/D3/D4 above) proves the wiring is correct; it does not substitute for watching the real language service actually offer completions."

duration: ~70min
completed: 2026-07-25
status: complete
---

# Phase 8 Plan 11: Monaco editor with live TypeScript type-checking and autocomplete Summary

**Real Monaco editor (pinned exact 0.55.1) replacing the placeholder textarea, running the TypeScript language service against the generated golc.d.ts SDK, themed to Paper/Ink in both colour schemes.**

## Performance

- **Duration:** ~70 min
- **Completed:** 2026-07-25
- **Tasks:** 3 (Task 1 checkpoint resolved by a prior executor run; Task 2 and Task 3 executed in this continuation)
- **Files modified:** 13 (5 created, 8 modified)

## Accomplishments

- **Task 1 (checkpoint, resolved before this continuation):** the package-legitimacy gate on `monaco-editor` was honoured, not argued away. Evidence gathered: `npm view monaco-editor versions --json` (0.55.0/0.55.1/0.56.0 recent history), `npm view monaco-editor repository` (`github.com/microsoft/monaco-editor`, official), and weekly-download volume (~7.9M/week). **Human decision, recorded verbatim: "pin 0.55.1."** This continuation re-confirmed the same version/integrity/repository via `npm view monaco-editor@0.55.1 version dist.integrity repository` immediately before running the install, without re-presenting the checkpoint or re-gathering evidence, per the continuation's own instructions.
- **Task 2:** `monaco-editor@0.55.1` installed with `--save-exact` (`"monaco-editor": "0.55.1"` in `frontend/package.json`, no caret/tilde). `monacoTheme.ts` maps Monaco's editor background/foreground/comment/selection/gutter/error/debug-line colours onto `index.css`'s exact Paper/Ink hex values for both colour schemes, importing `monaco-editor` as types only so its own test needs no mock. `vite.config.ts` gained `worker: { format: "es" }` for Monaco's ESM worker chunks. `ScriptService.GetSDKTypeDefinitions` (Go) reads the committed `internal/scriptsdk/generated/golc.d.ts`, and `wailsBridge.getSdkTypeDefinitions()` serves it to the frontend with the same graceful-degradation contract every other bridge call follows.
- **Task 3:** `ScriptEditor.tsx` is a real, controlled Monaco instance: reconciled Paper/Ink editor options (JetBrains Mono 14px/21px line height, minimap disabled, glyph margin + line numbers on), the `golc.d.ts` SDK registered as a TypeScript extra lib exactly once per distinct value (never once per keystroke), theme switching on `prefers-color-scheme` without remounting, and full disposal (editor + model + listeners + extra lib) on unmount. `ScriptsWorkspace.tsx` renders it in place of the 08-04 placeholder textarea, fetching `golc.d.ts` once on mount.

## Task Commits

Each task was committed atomically:

1. **Task 1: Confirm the exact monaco-editor version before first install** — checkpoint, resolved by a prior executor run with no file changes (recorded above; nothing to commit for this task itself).
2. **Task 2: Install monaco-editor, register the Paper/Ink themes, and serve the SDK types** — `e172478` (feat)
3. **Task 3: ScriptEditor component with live type-checking and autocomplete** — `a5e066e` (feat)

**Plan metadata:** (this commit, docs: complete plan)

_Both Task 2 and Task 3 were TDD (`tdd="true"`): tests were written alongside each implementation and verified green (`go test ./internal/wails/... -count=1`, `npm --prefix frontend run build`) before commit; this is an `execute`-type plan with `tdd="true"` on individual tasks, not a plan-level RED/GREEN/REFACTOR gate._

## Files Created/Modified

- `frontend/package.json` / `package-lock.json` — pins `monaco-editor@0.55.1` exactly
- `frontend/vite.config.ts` — `worker: { format: "es" }` for Monaco's ESM worker chunks
- `frontend/src/components/Scripts/monacoTheme.ts` + `.test.ts` — Paper/Ink Monaco theme definitions, type-only monaco-editor import
- `frontend/src/lib/wailsBridge.ts` — `getSdkTypeDefinitions()` wrapper
- `internal/wails/svc_script.go` + `_test.go` — `GetSDKTypeDefinitions()` reading the committed `golc.d.ts`
- `frontend/src/components/Scripts/ScriptEditor.tsx` + `.module.css` + `.test.tsx` — the real Monaco editor component
- `frontend/src/workspaces/build/ScriptsWorkspace.tsx` + `.test.tsx` — renders `ScriptEditor` in place of the placeholder textarea

## Decisions Made

See `key-decisions` in frontmatter above. In short: the human-approved pin is 0.55.1 (verbatim reply "pin 0.55.1"); `monaco-editor` is imported dynamically rather than statically to avoid crashing every jsdom test that merely renders this component's tree (not just the tests that know to mock it); worker loading is guarded behind `typeof Worker` for the same jsdom-compatibility reason; and monaco-editor@0.55.1's real TypeScript language-service surface is the new top-level `typescript` export, not the deprecated `languages.typescript` an older reference might assume.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] monaco-editor@0.55.1 deprecated `monaco.languages.typescript`**
- **Found during:** Task 3, first `npm run build` after implementing `ScriptEditor.tsx`
- **Issue:** `tsc --noEmit` failed: `monaco.languages.typescript` is typed as `{ deprecated: true }` in this pinned version — the TypeScript language-service API moved to a new top-level `typescript` namespace exported directly off `"monaco-editor"`.
- **Fix:** Verified the real shape directly against `node_modules/monaco-editor/esm/vs/editor/editor.main.d.ts` (`export { ..., monaco_contribution as typescript }`) and switched every call site (`typescriptDefaults`, `ScriptTarget`, `ModuleKind`, `ModuleResolutionKind`) to `monaco.typescript.*`. `ScriptTarget` in this version also has no `ES2022` member (max named target is `ES2020`, plus `ESNext`/`Latest`); used `ESNext`.
- **Files modified:** `frontend/src/components/Scripts/ScriptEditor.tsx`, `frontend/src/components/Scripts/ScriptEditor.test.tsx`, `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx`
- **Verification:** `npm --prefix frontend run build` succeeds (tsc clean, all tests pass, vite build succeeds).
- **Committed in:** `a5e066e` (Task 3 commit)

**2. [Rule 1 - Bug] Static `monaco-editor` import crashed every jsdom test rendering this component's tree**
- **Found during:** Task 3, first full `npm run build` after wiring `ScriptEditor` into `ScriptsWorkspace`
- **Issue:** A static top-level `import * as monaco from "monaco-editor"` evaluates the real package's module graph immediately on import. Under jsdom, this threw `TypeError: document.queryCommandSupported is not a function` from `monaco-editor/esm/vs/editor/contrib/clipboard/browser/clipboard.js`, breaking `App.smoke.test.tsx` and `AppShell.navigation.test.tsx` — two pre-existing, unrelated app-wide smoke/navigation tests that render the full app tree and have no reason to know they'd need to mock `monaco-editor`.
- **Fix:** Changed to a dynamic `await import("monaco-editor")` inside the mount effect, wrapped in try/catch; a failed/rejected import sets a `loadFailed` state rendering an inline "failed to load" placeholder instead of propagating the exception. Also guarded the Vite `?worker` worker-loader imports behind `typeof Worker === "undefined"` (jsdom does not implement the `Worker` global at all, and Vite's `?worker` wrapper class extends it at its own module-evaluation time).
- **Files modified:** `frontend/src/components/Scripts/ScriptEditor.tsx`, `frontend/src/components/Scripts/ScriptEditor.test.tsx` (awaits mount resolution before asserting)
- **Verification:** Full `npm --prefix frontend run build` (40 test files, 193 tests) passes, run twice with no flakiness.
- **Committed in:** `a5e066e` (Task 3 commit)

**3. [Rule 1 - Bug] A `[value, ready]`-dependent sync effect could stomp a freshly-typed value with a stale prop**
- **Found during:** Task 3, TDD implementation of the `ScriptsWorkspace.test.tsx` "edits the textarea and saves via SaveScriptSource" test
- **Issue:** An effect that re-synced the Monaco model's value whenever `value` OR `ready` changed could fire on the `ready` transition (a state update from within the async mount) with a still-stale `value` prop closure from before the user's own edit had propagated back up through `onChange`/`setSource`, immediately reverting the model (and the reported source) back to empty.
- **Fix:** Mirrored `value`/`readOnly`/`sdkTypeDefinitions` into refs (updated unconditionally every render) and had the async mount effect apply the freshest ref values at completion time, rather than its own stale closured props. Removed `ready` from every follow-up sync effect's dependency array — they now only react to a genuine subsequent prop change, eliminating the race entirely.
- **Files modified:** `frontend/src/components/Scripts/ScriptEditor.tsx`
- **Verification:** `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx`'s full 16-test suite passes; re-ran isolated and full-suite multiple times with no further flakiness.
- **Committed in:** `a5e066e` (Task 3 commit)

**4. [Rule 1 - Bug] A test-only React 18 automatic-batching timing gap**
- **Found during:** Task 3, same test as above
- **Issue:** Even after fix #3, a synchronous `fireEvent.change(...)` immediately followed by `fireEvent.click(Save)` could read stale `source` state: the change originates from a native (non-React-synthetic) `"change"` DOM listener in the test's Monaco mock, so React 18's automatic batching can defer the resulting `setSource` to a microtask a plain synchronous `fireEvent` sequence does not wait for.
- **Fix:** Wrapped the `fireEvent.change(...)` call in `await act(async () => { ... })` in the affected test, which flushes the pending microtask-scheduled update before the subsequent `fireEvent.click(Save)` runs.
- **Files modified:** `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx`
- **Verification:** Test passes consistently across repeated runs.
- **Committed in:** `a5e066e` (Task 3 commit)

**5. [Rule 1 - Bug] A synchronous assertion raced the selection-validity-repair effect**
- **Found during:** Task 3, `ScriptsWorkspace.test.tsx`'s pre-existing "streams live script:event log lines" test
- **Issue:** A direct (non-`waitFor`) `expect(screen.getByText("Run or Debug this script...")).toBeInTheDocument()` immediately after the script list rendered assumed the selection-validity-repair effect (which auto-selects the first script) had already committed. With one more async effect now in the render tree (the new `getSdkTypeDefinitions` fetch), this assumption became unreliable.
- **Fix:** Wrapped the assertion in `await waitFor(...)`, matching the pattern every other test in the same file that depends on auto-selection already uses.
- **Files modified:** `frontend/src/workspaces/build/ScriptsWorkspace.test.tsx`
- **Verification:** Test passes consistently across repeated runs.
- **Committed in:** `a5e066e` (Task 3 commit)

**6. [Rule 1 - Bug] A leftover doc-comment reference tripped the plan's own textarea-removal grep**
- **Found during:** Task 3, running the plan's own acceptance-criteria greps after Task 3's commit
- **Issue:** `grep -c 'textarea' frontend/src/workspaces/build/ScriptsWorkspace.tsx` returned 1 (expected 0) — a leftover doc comment ("replacing 08-04's plain `<textarea>` in place") referenced the literal word, even though the live `<textarea>` element itself was already fully removed.
- **Fix:** Reworded the comment to avoid the literal string while keeping the same meaning.
- **Files modified:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx`
- **Verification:** `grep -c 'textarea' frontend/src/workspaces/build/ScriptsWorkspace.tsx` now returns 0; full test suite re-confirmed green.
- **Committed in:** `a5e066e` (Task 3 commit)

---

**Total deviations:** 6 auto-fixed (2 blocking-API-mismatch fixes, 4 bugs — 3 of them test-only timing/assertion races surfaced by adding an async-mounting Monaco component into an existing jsdom test suite).
**Impact on plan:** All auto-fixes were necessary for `npm --prefix frontend run build` to pass cleanly and for the existing test suite to remain trustworthy (not flaky). No scope creep — every fix stayed within Task 2/Task 3's own files.

## Issues Encountered

Monaco-editor's real-world jsdom incompatibility (module-eval-time `document.queryCommandSupported` access, and no `Worker` global) was not something the plan could have specified in advance without implementation-time investigation against the exact pinned version; both are now documented in `ScriptEditor.tsx`'s own header comment so a later reader does not attempt to "fix" the dynamic import back into a static one.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-15's live type-checking/autocomplete wiring (SDK extra-lib registration, Paper/Ink theming, dispose-on-unmount) is fully implemented and unit-tested against a faithful Monaco fake.
- **Not yet independently verified in this session:** the actual in-browser behavior of typing `golc.` and seeing real completions, and a wrong-argument-type live squiggle — this execution had no live Wails webview available (headless worktree, no computer-use/browser tooling attached). Recorded as coverage item D6 (`human_judgment: true`); a human (or a future session with a real Wails runtime) should perform this check before treating D-15 as fully signed off end-to-end.
- 08-12 (the next plan, referenced in this plan's own `key_links`) can build on `monacoTheme.ts`'s `--status-armed` debug-current-line colour mapping and `ScriptEditor.tsx`'s existing `glyphMargin: true` scaffold for the D-01 breakpoint gutter's click wiring.

## Self-Check: PASSED

- All 13 created/modified source files verified present via `git ls-files` (5 created, 8 modified).
- Both task commits (`e172478`, `a5e066e`) verified present in `git log --oneline --all`.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-25*
