---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "27"
subsystem: scripts-monaco-ui
tags: [react, design-system, monaco, scripts, dialog, checker-exception]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
affects: [remaining Wave 7 workspace migrations, any future plan touching monacoTheme.ts or the Scripts component family]
tech-stack:
  added: []
  patterns:
    - "A stack-trace/log-style clickable technical row with no design-system primitive fit (variable-length wrapped monospace text, full-width, no icon/label/meta shape) is a genuinely narrow DS005 exception -- but first convert every OTHER raw-button instance in the same file to a real primitive (Button/IconButton) so only one raw <button> DS005 diagnostic remains in that file, since DS005's diagnostic value is always the fixed string \"button\"/\"input\"/\"select\"/\"textarea\" with no way to make two instances in one file byte-distinguishable."
    - "A CSS z-index on a position:absolute overlay sibling to a position:static element is frequently redundant -- positioned elements already paint above non-positioned siblings per CSS2.1 Appendix E regardless of z-index/DOM order; removing an unnecessary z-index eliminates a DS001 violation architecturally instead of needing a --ds-stacking-* token or an exception."
    - "border-radius: 50% on a small fixed-size square box (e.g. an 8x8px status dot) is visually identical to border-radius: var(--ds-radii-pill) (999px, which simply clamps) -- a real token substitution, not an exception, for exact-circle geometry."
    - "A hand-tuned margin offset centering a fixed-size pseudo-element inside a variable-height parent (e.g. Monaco's own glyph-margin cell) can usually be replaced by display:flex + align-items:center + justify-content:center on the parent, eliminating the raw margin literal entirely -- required here since DS001's own forbidden-word list makes any margin-touching exception mechanically impossible, not just discouraged."
    - "Monaco's theme API needs literal hex color constants for BOTH light and dark themes registered simultaneously, and a `:root[data-theme=\"light\"|\"dark\"]`-scoped custom property can only ever resolve to whichever ONE mode is currently applied to the real <html> element -- so a 'semantic theme adapter' for a vendor surface like this can pin static per-mode hex constants (verified byte-identical to the generated stylesheet's own resolution) named and re-exported via the real token mapping (tokens.generated.ts's semanticTokenCSSVariables), rather than a runtime getComputedStyle read, without that being a blanket exemption from the token system."
key-files:
  modified:
    - frontend/src/workspaces/build/ScriptsWorkspace.tsx
    - frontend/src/workspaces/build/ScriptsWorkspace.module.css
    - frontend/src/components/Scripts/ScriptRunDialog.tsx
    - frontend/src/components/Scripts/ScriptRunDialog.module.css
    - frontend/src/components/Scripts/ScriptRunDialog.test.tsx
    - frontend/src/components/Scripts/monacoTheme.ts
    - frontend/src/components/Scripts/monacoTheme.test.ts
    - frontend/src/components/Scripts/ScriptDebugPanel.tsx
    - frontend/src/components/Scripts/ScriptDebugPanel.module.css
    - frontend/src/components/Scripts/ScriptEditor.module.css
  created:
    - frontend/design-system/exception-proposals/editors.json
key-decisions:
  - "ScriptRunDialog.tsx migrated from its own hand-rolled backdrop/dialog markup onto the shared Dialog primitive (same adoption FixtureStyleModal.tsx made in Plan 13-13): initial focus now lands on the Cancel button (Dialog's own 'least-destructive action' convention, matching ConfirmDialog.tsx) instead of the dialog surface itself. This is a deliberate, documented deviation from the plan's literal 'dialog focus remains unchanged' truth -- the underlying MODAL BEHAVIOR (focus trap, Escape-to-close, backdrop-click-to-close, focus return to the invoking control, ARIA wiring) is fully preserved and is what actually matters for the D-07/D-09 review-before-launch contract; only the specific focused ELEMENT changed, to the framework's own standard 'safe action' target. ScriptRunDialog.test.tsx updated accordingly (asserts Cancel has focus, dispatches Escape on the dialog element and backdrop-close via mousedown on the real dialog-backdrop testid, matching Dialog.tsx's actual event wiring)."
  - "monacoTheme.ts's LIGHT_COLORS/DARK_COLORS stay pinned literal hex constants rather than switching to a getComputedStyle-based runtime read: Monaco's theme API requires BOTH light and dark themes registered up front (defineTheme is called for both at mount, independent of which one is currently active), and design-system/tokens.generated.css's custom properties are scoped by `:root[data-theme=\"light\"|\"dark\"]` -- only the currently-applied mode's values are ever live-readable from the real <html> element, with no way to read the other mode's values without forcibly toggling the document's own theme attribute (a real, visible side effect this component has no business causing). Each literal was hand-verified byte-identical to its named token's resolution in the generated stylesheet's un-suffixed ('default' theme) block; MONACO_TOKEN_CSS_VARIABLES derives each role's CSS variable name from the real generated semanticTokenCSSVariables mapping (not a second hand-copied string), and a new monacoTheme.test.ts assertion locks the two together so a renamed/removed token is a real test failure, not a silently stale comment -- the exact drift class (a hardcoded 'copied from index.css' comment outliving the file it referenced) this migration exists to fix."
  - "ScriptDebugPanel.tsx's 'Show/Hide stack trace' disclosure toggle converted from a raw `<button>` to the Button primitive (variant=\"secondary\" size=\"compact\", leadingIcon chevron) specifically so the file's OTHER raw `<button>` (the per-frame stack-trace row) could be registered as a single, clean DS005 exception. DS005's diagnostic value is always the fixed string \"button\" regardless of location, so two raw-button violations in the same file are byte-identical and mechanically unexceptable (the same collision class Plan 13-13 documented for DS001/DS006); eliminating one via a real primitive substitution was the only path to a working single-record exception for the other, which has no primitive fit (ListRow's icon/label/meta/selected shape doesn't suit a variable-length wrapped monospace stack-frame line; Button's fixed height/icon+label layout doesn't either)."
  - "ScriptEditor.module.css's breakpoint-dot pseudo-element centering switched from a hand-tuned `margin: 6px auto 0` to `display:flex; align-items:center; justify-content:center` on the parent -- required, not optional: DS001's exception mechanism categorically forbids any exception whose match string contains the word \"margin\" (a mechanical, code-level rule, not just project policy), so this raw literal could never be exempted no matter how narrow. The dot's `border-radius: 50%` was likewise replaced with `var(--ds-radii-pill)` (999px, clamps identically on an 8x8px fixed box) -- a real token, not an exception, and visually indistinguishable."
  - "ScriptEditor.module.css's .placeholder overlay's `z-index: 1` was removed outright rather than mapped to a --ds-stacking-* token: it is position:absolute while its .container sibling is plain static-positioned, and CSS2.1 Appendix E already paints positioned elements above non-positioned siblings in the same stacking context regardless of z-index or DOM order -- the declaration was redundant, not a genuine layering requirement, so removing it is a real fix rather than a token substitution or exception."
  - "Several ScriptsWorkspace.module.css/ScriptRunDialog.module.css/ScriptDebugPanel.module.css classes were renamed to drop a DS006 controlled-class-name collision once real visual properties (color/padding/font/border) landed on them: errorText->feedback (mirrors NotesWorkspace.module.css's own identical rename), loadingRow->pendingRow, emptySelection->noSelectionMessage, validationError->validationFeedback, inspectorEmpty->inspectorPlaceholder, and outcomeError->outcomeFailed. None of these are behavior changes -- CSS Modules class names are opaque identifiers to the DOM/tests, and no test in scope asserted against the literal class-name strings."
  - "ScriptsWorkspace.tsx's resizable-panel custom property was renamed from --library-width to --ds-scriptslist-width and its CSS-side fallback default (`, 240px`) was dropped, matching ScenesLooksWorkspace.module.css's identical --ds-scenelist-width convention -- DS003 flags ANY var() fallback regardless of the --ds- prefix, and the value is set unconditionally by the component's own inline style whenever the grid renders, so no fallback was ever needed."
patterns-established:
  - "When a checker's own diagnostic value is a fixed, non-distinguishing string for a whole diagnostic category (DS005's \"button\"/\"input\"/\"select\"/\"textarea\"), two violations of that category in one file are inherently unexceptable together -- convert every instance but the genuinely-irreducible one to a real primitive first, then except the single remainder."
requirements-completed: [D-02, D-04, D-05, D-06, D-07, D-11, D-12, D-14, UI-SPEC-EDITORS]
coverage:
  - id: D1
    description: Scripts/Monaco chrome uses public contracts while isolation, cancellation, diagnostics, and disposal remain intact
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/build/ScriptsWorkspace.test.tsx (21 tests), ScriptRunDialog.test.tsx (11 tests), ScriptDebugPanel.test.tsx (24 tests), ScriptEditor.test.tsx (12 tests), monacoTheme.test.ts (7 tests) -- all pass unchanged in behavior (Sidecar RunScript/DebugScript/StopScript/ContinueScript/StepOverScript/StepIntoScript/StepOutScript dispatch, breakpoint gutter -> DebugScript wiring, live event stream folding, model disposal/cleanup) except ScriptRunDialog.test.tsx's own focus-target and Escape/backdrop-event-dispatch assertions, updated to match the Dialog primitive's real, documented behavior"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/build/ScriptsWorkspace.tsx,src/workspaces/build/ScriptsWorkspace.module.css,src/components/Scripts/ScriptRunDialog.tsx,src/components/Scripts/ScriptRunDialog.module.css (Task 1); --paths src/components/Scripts/monacoTheme.ts,src/components/Scripts/ScriptDebugPanel.tsx,src/components/Scripts/ScriptDebugPanel.module.css,src/components/Scripts/ScriptEditor.tsx,src/components/Scripts/ScriptEditor.module.css --proposal design-system/exception-proposals/editors.json (Task 2)"
        status: pass
    human_judgment: false
  - id: D2
    description: The fourteen-file plan is divided into five- and nine-file tasks with no broad vendor exceptions
    verification:
      - kind: static
        ref: "design-system/exception-proposals/editors.json: exactly one narrow, single-diagnostic DS005 record (ScriptDebugPanel.tsx's per-frame stack-trace button, no design-system primitive fit); zero DS001 spacing exceptions anywhere"
        status: pass
    human_judgment: false
  - id: D3
    description: React remains projection-only and does not own playback, script execution, or the Deno sandbox boundary
    verification:
      - kind: unit
        ref: "ScriptsWorkspace.test.tsx's full suite continues to assert every mutation dispatches unchanged to window.go.wails.ScriptService.*; no new local authority, no change to configureMonacoEnvironment's dynamic-import/try-catch contract, no change to model/editor/decoration-collection disposal in ScriptEditor.tsx"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (single continuous parallel-executor session)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 27: Scripts/Monaco Migration Summary

**Scripts workspace, run-launch dialog, Monaco editor chrome, debug panel, and the theme adapter now consume shared design-system primitives and --ds-* tokens end-to-end, with exactly one narrow, single-diagnostic DS005 exception for a stack-trace row with no primitive fit; sidecar dispatch, breakpoint/step-debugging wiring, model disposal, and the Deno isolation boundary are all unchanged.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system checks:** both pass with zero diagnostics (Task 1: ScriptsWorkspace + ScriptRunDialog; Task 2: monacoTheme/ScriptDebugPanel/ScriptEditor with `editors.json`'s one exception resolving cleanly)
- **Focused tests:** 21 + 11 + 6 + 24 + 12 = 74 tests across the five touched test files, all pass. Full frontend suite: 477/477 pass. `tsc --noEmit`: clean. `npm run build` (tsc + vitest + vite build): clean.

## Accomplishments

- Converted `ScriptsWorkspace.tsx` to consume `Button`/`Chip`/`EmptyState`/`Field`/`FormActions`/`ListRow`/`ResizeHandle`/`ScrollRegion`/`Toolbar` from the design-system barrel; replaced the hand-rolled empty-state markup with `EmptyState`, the "New script name" bare-`aria-label` input with a labeled `Field`, and retokenized every declaration in `ScriptsWorkspace.module.css` to `--ds-*`.
- Migrated `ScriptRunDialog.tsx` onto the shared `Dialog` primitive (focus trap, Escape, backdrop click, focus return, ARIA wiring) and `Field`-wrapped `<select>`s for capability scope and resource preset, eliminating its own duplicated backdrop/dialog CSS entirely.
- Re-anchored `monacoTheme.ts`'s Paper/Ink color table onto `design-system/tokens.generated.ts`'s real semantic token names (`MONACO_TOKEN_SOURCES`/`MONACO_TOKEN_CSS_VARIABLES`), replacing a stale "copied from index.css" provenance comment (that file's legacy block no longer exists) with a machine-checkable link `monacoTheme.test.ts` now asserts directly.
- Converted `ScriptDebugPanel.tsx`'s stack-trace disclosure toggle to the `Button` primitive and retokenized `ScriptDebugPanel.module.css`, renaming its one DS006-colliding class (`outcomeError` -> `outcomeFailed`).
- Retokenized `ScriptEditor.module.css`, fixing two real bugs found along the way (an unnecessary `z-index`, and a hand-tuned breakpoint-dot margin offset replaced with flex centering) rather than exempting either.
- Registered exactly one exception in the new `frontend/design-system/exception-proposals/editors.json` (the per-frame stack-trace row's raw `<button>`, which has no design-system primitive fit).

## Task Commits

1. **Task 1:** `0e7b8e18` — ScriptsWorkspace/ScriptRunDialog design-system migration.
2. **Task 2:** `9c6c4bcd` — monacoTheme/ScriptDebugPanel/ScriptEditor design-system migration, `editors.json`.

## Verification

- `cd frontend && npx vitest run src/workspaces/build/ScriptsWorkspace.test.tsx src/components/Scripts/ScriptRunDialog.test.tsx` — 32/32 pass.
- `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/build/ScriptsWorkspace.tsx,src/workspaces/build/ScriptsWorkspace.module.css,src/components/Scripts/ScriptRunDialog.tsx,src/components/Scripts/ScriptRunDialog.module.css` — exit 0, zero diagnostics.
- `cd frontend && npx vitest run src/components/Scripts/monacoTheme.test.ts src/components/Scripts/ScriptDebugPanel.test.tsx src/components/Scripts/ScriptEditor.test.tsx` — 42/42 pass.
- `cd frontend && node scripts/design-system/check.mjs --paths src/components/Scripts/monacoTheme.ts,src/components/Scripts/ScriptDebugPanel.tsx,src/components/Scripts/ScriptDebugPanel.module.css,src/components/Scripts/ScriptEditor.tsx,src/components/Scripts/ScriptEditor.module.css --proposal design-system/exception-proposals/editors.json` — exit 0, zero diagnostics.
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && npx vitest run` (full suite) — 477/477 pass.
- `cd frontend && npm run build` (`tsc --noEmit && vitest run && vite build`) — clean.
- Grepped every touched `.module.css` file for a literal mid-line `*/` substring (per this phase's own known CSS-comment-termination bug) — every match found is a legitimate comment-closing terminator at the true end of its comment, not an early-closing substring embedded in prose.

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 1 - Bug] Removed a redundant `z-index: 1` from ScriptEditor.module.css's `.placeholder`**
   - **Found during:** Task 2, retokenizing `ScriptEditor.module.css`
   - **Issue:** `.placeholder` is `position: absolute` while its `.container` sibling is plain static-positioned; a raw `z-index: 1` literal fails DS001 with no `--ds-stacking-*` token that fits a purely-local overlay concern.
   - **Fix:** Removed the declaration entirely — CSS2.1 Appendix E already paints positioned elements above non-positioned siblings in the same stacking context regardless of z-index or DOM order, so it was redundant, not a genuine layering requirement.
   - **Verification:** `node scripts/design-system/check.mjs --paths src/components/Scripts/ScriptEditor.module.css` shows the DS001 finding gone; `ScriptEditor.test.tsx`'s loading/failed-placeholder rendering tests pass unchanged.

2. **[Rule 3 - Blocking] Replaced the breakpoint dot's hand-tuned margin offset with flex centering**
   - **Found during:** Task 2
   - **Issue:** `.breakpointGlyph::before`'s `margin: 6px auto 0` is a raw DS001 literal with no bare-token equivalent — and DS001's exception mechanism mechanically rejects ANY exception whose match string contains the word "margin" (a code-level rule, not just policy), so this could never be exempted, no matter how narrow.
   - **Fix:** Removed the margin entirely; added `display: flex; align-items: center; justify-content: center;` to the real `.breakpointGlyph` element, self-centering the fixed 8x8px dot within Monaco's own glyph-margin cell regardless of exact line height (an architectural improvement over a hardcoded guess, not just a workaround).
   - **Verification:** `ScriptEditor.test.tsx`'s breakpoint-decoration tests pass unchanged; scoped checker shows zero DS001 findings for this rule.

3. **[Rule 3 - Blocking] Two byte-identical DS005 `<button>` diagnostics resolved by converting one to a real primitive**
   - **Found during:** Task 2, registering `ScriptDebugPanel.tsx`'s domain exception
   - **Issue:** DS005's diagnostic value is always the fixed string `"button"` regardless of source location — the Show/Hide stack-trace toggle and the per-frame stack-trace row were both raw `<button>` elements in the same file, producing two byte-identical diagnostics the exception mechanism cannot distinguish (the same collision class Plan 13-13 documented for DS001/DS006, now confirmed for DS005 too).
   - **Fix:** Converted the disclosure toggle to the `Button` primitive (`variant="secondary" size="compact"`, leading chevron icon) — a clean, appropriate fit for that control — leaving only the per-frame row (which has no primitive fit: `ListRow`'s icon/label/meta/selected shape doesn't suit a variable-length wrapped monospace line, and `Button`'s fixed height/icon+label layout doesn't either) as the file's sole remaining raw button, now cleanly exceptable.
   - **Verification:** `node scripts/design-system/check.mjs --paths src/components/Scripts/ScriptDebugPanel.tsx,src/components/Scripts/ScriptDebugPanel.module.css --proposal design-system/exception-proposals/editors.json` — exit 0; `ScriptDebugPanel.test.tsx`'s "Show stack trace"/frame-click tests pass unchanged (Button still renders an accessible `role="button"` named "Show stack trace").

4. **[Rule 1 - Bug] `border-radius: 50%` replaced with `var(--ds-radii-pill)`**
   - **Found during:** Task 2
   - **Issue:** `.breakpointGlyph::before`'s `border-radius: 50%` is a raw DS001 literal.
   - **Fix:** Replaced with `var(--ds-radii-pill)` (999px) — clamps to an identical perfect circle on the fixed 8x8px box, a real token substitution with zero visual difference.
   - **Verification:** Scoped checker shows zero DS001 findings for this declaration.

### Documented Behavior Deviation (not a bug fix, a deliberate migration tradeoff)

1. **ScriptRunDialog's initial-focus target changed from the dialog surface itself to the Cancel button**
   - **Reason:** Migrating onto the shared `Dialog` primitive (this plan's own explicit instruction: "use the packaged-proven Dialog API") adopts its documented focus convention — the "least-destructive action" (same as `ConfirmDialog.tsx`'s `cancelRef`) — rather than focusing the non-interactive dialog container the original hand-rolled implementation did.
   - **Impact:** The modal's actual safety-relevant behavior (focus trap, Escape-to-close, backdrop-click-to-close, focus return to the invoking control on close, ARIA `role="dialog"`/`aria-modal`) is fully preserved and now backed by the shared, packaged-proven primitive instead of a second bespoke implementation. Only the specific focused element changed.
   - **Verification:** `ScriptRunDialog.test.tsx` updated to assert focus lands on Cancel, and to dispatch Escape/backdrop-close events the way `Dialog.tsx` actually wires them (`onKeyDown` on the dialog element itself, `onMouseDown` on the `data-testid="dialog-backdrop"` element) — all 11 tests pass.

## Known Stubs

None.

## Issues Encountered

- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-PATTERNS.md`, referenced by this plan's own `<context>` block and task actions, does not exist in this worktree (it is untracked in a sibling location, not committed to the branch this worktree was created from). Proceeded using the equivalent already-committed analogs directly: `FixtureLibraryWorkspace.tsx`/`workspace.module.css` (Task 1's named migration template) and `Desk.module.css`/`FixtureStyleModal.tsx` (Plan 13-13's established border/outline/padding-splitting and Dialog/Field-adoption conventions, read directly from `13-13-SUMMARY.md` and the committed source).
- `frontend/node_modules` did not exist in this worktree (each Claude Code worktree gets its own working directory, and `node_modules` is gitignored/not copied). A directory junction to the main repo's `frontend/node_modules` was created solely to run `vitest`/`tsc`/the design-system checker locally, and was removed again (via `cmd.exe /c rmdir`, which does not recurse into a junction's target) before this plan finished, to avoid any risk of the orchestrator's worktree-cleanup step recursively deleting through the junction into the main repo's real `node_modules`. The main repo's `frontend/node_modules/postcss` was confirmed still present and intact after the junction's removal.

## Next Phase Readiness

Scripts/Monaco's fourteen declared files are fully migrated with zero unregistered violations. The `MONACO_TOKEN_CSS_VARIABLES` pattern (deriving a vendor-surface's own color-role names from the real generated token mapping, rather than a second hand-copied constant table) and the "convert every raw-button instance but one to a real primitive so DS005's fixed diagnostic value stays exceptable" pattern are both available to any remaining Wave 7 plan with a similar vendor-chrome or multi-raw-control surface.

## Self-Check: PASSED

- Commits `0e7b8e18` and `9c6c4bcd` exist and together contain all 11 declared files plus the new `design-system/exception-proposals/editors.json`.
- `git diff --diff-filter=D --name-only` against both commits shows zero unintended file deletions.
- The plan's own exact Task 1 and Task 2 `<verify>` commands (adjusted only to add `editors.json`'s proposal path, per Task 2's own `--proposal` flag) both pass with exit 0 and zero diagnostics.
- Full frontend build gate (`npm run build`) and full test suite (477/477) pass.
