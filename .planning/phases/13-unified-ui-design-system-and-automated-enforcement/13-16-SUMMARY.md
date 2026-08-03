---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "16"
subsystem: shell-secondary-surfaces
tags: [react, design-system, dialog, focus-management, accessibility, checker-exceptions]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
provides:
  - Unified ContextualInspector/InspectorSlot, HelpOverlay, QuickSwitcher on design-system tokens and the shared Dialog primitive
  - A documented, narrowly-scoped token-independence exception set for ErrorBoundary's own emergency fallback
affects: [remaining Wave 7 shell/workspace migrations, any future plan touching HelpOverlay/QuickSwitcher/ErrorBoundary]
tech-stack:
  added: []
  patterns:
    - "HelpOverlay and QuickSwitcher now consume the shared Dialog primitive instead of a hand-rolled backdrop+dialog pair, eliminating a DS006 duplicate-dialog-chrome violation and inheriting Dialog's already-proven (13-06) focus-trap/Escape/backdrop/return-focus contract."
    - "A dialog needing no visible title bar (QuickSwitcher) still passes Dialog's required `title` prop, wrapped in the shared `.ds-sr-only` utility class, so the dialog keeps a real accessible name without introducing a visible heading that wasn't there before."
    - "ErrorBoundary.module.css's color/background declarations are the one place in this codebase intentionally exempt from --ds-* tokens: bare hex/rgb literals only, registered as narrow DS001 exceptions, because this screen must remain readable even when tokens.generated.css itself failed to load. Everything else in that file (spacing/radius/typography) safely uses --ds-* tokens, since losing those in the same failure only changes geometry, never legibility."
    - "When two DS001/DS005 diagnostics in the same file would carry a byte-identical value (the checker's exception-matching mechanism can only resolve a match to exactly one diagnostic), write the second occurrence in an equivalent-but-textually-distinct form (hex vs rgb(), quoted vs unquoted font-family) rather than skip disambiguation -- zero visual change, but each diagnostic becomes independently exceptable."
key-files:
  modified:
    - frontend/src/shell/ContextualInspector.module.css
    - frontend/src/shell/HelpOverlay.tsx
    - frontend/src/shell/HelpOverlay.module.css
    - frontend/src/shell/HelpOverlay.test.tsx
    - frontend/src/shell/QuickSwitcher.tsx
    - frontend/src/shell/QuickSwitcher.module.css
    - frontend/src/shell/QuickSwitcher.test.tsx
    - frontend/src/shell/ErrorBoundary.tsx
    - frontend/src/shell/ErrorBoundary.module.css
  created:
    - frontend/design-system/exception-proposals/shell.json
key-decisions:
  - "HelpOverlay and QuickSwitcher adopt the Dialog primitive per the plan's own key_links (Dialog pattern); their .test.tsx files were updated to match Dialog's real interaction contract (mousedown-based backdrop dismissal via the shared dialog-backdrop testid, matching Dialog.test.tsx's own convention) instead of the old onClick-based custom backdrop -- a test-mechanism change only, every original assertion/behavior is preserved."
  - "QuickSwitcher's local Escape handling was removed entirely (Dialog's own closeOnEscape already owns it); keeping both would have called onClose twice for one keypress."
  - "QuickSwitcher's .resultSelected now uses --ds-surface-selected (a real, declared 'current selection' surface token) instead of reimplementing the removed --accent-5 var via color-mix() -- the more correct fix per UI-SPEC's Semantic Token Contract, not a token-independence workaround."
  - "Fixed a real, pre-existing DS010 accessibility gap in QuickSwitcher.module.css: `.input:focus { outline: none; }` removed the focus indicator with no compensating replacement, violating D-12/the Interaction State Contract's 'never removed without an equally visible replacement.' Replaced with a proper `:focus-visible` ring using longhand outline-width/-style/-color/-offset tokens (the compound `outline: var(...) solid var(...)` shorthand used by some already-shipped primitives, e.g. Button.module.css, fails DS001's strict single-var()-per-declaration check and is a known, out-of-scope whole-source gap this plan does not extend)."
  - "ErrorBoundary.tsx's Reload control and 7 ErrorBoundary.module.css color/background literals are registered design-system exceptions (DS005/DS001) rather than converted, because this file is the app's documented last line of defense against a blank window and must not depend on the Button primitive or the generated token stylesheet, either of which could plausibly be part of what just crashed. This matches DESIGN_SYSTEM.md's UI-SPEC 'Token-independent ErrorBoundary fallback colors' carve-out, which 13-30 (a later wave-9 plan) is dedicated to backstop-testing."
  - "Two byte-identical DS001 value collisions inside ErrorBoundary.module.css (revoked-red used by both .title and .reload; muted-gray used by both .body and .detail) were resolved by writing the second occurrence as an equivalent rgb() form rather than the identical hex -- zero rendered color change, but each diagnostic becomes independently exceptable under the checker's per-diagnostic (not file-wide) matching."
  - "QuickSwitcher's search input and its role=\"option\" result button are both registered DS005 exceptions: Field always renders a visible <label> that doesn't fit a borderless command-palette box, and the result row composes icon+label+group behind one click target in a way no current Button/IconButton variant expresses."
patterns-established:
  - "A dialog with no visible title bar still supplies a real (visually-hidden) accessible title to the shared Dialog primitive, rather than fighting Dialog's required title prop or duplicating dialog chrome locally to avoid it."
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-13, D-14, UI-SPEC-SHELL, UI-CONSIDERATIONS]
coverage:
  - id: D1
    description: ContextualInspector/InspectorSlot and HelpOverlay migrated to design-system tokens and the shared Dialog primitive with zero exceptions needed
    verification:
      - kind: unit
        ref: "frontend/src/shell/ContextualInspector.test.tsx, HelpOverlay.test.tsx, InspectorSlot.test.tsx (10 tests)"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/shell/ContextualInspector.tsx,src/shell/ContextualInspector.module.css,src/shell/InspectorSlot.tsx,src/shell/HelpOverlay.tsx,src/shell/HelpOverlay.module.css"
        status: pass
    human_judgment: false
  - id: D2
    description: QuickSwitcher and ErrorBoundary migrated to design-system tokens/Dialog with narrowly-scoped, documented exceptions for ErrorBoundary's token-independent fallback colors and QuickSwitcher's unlabeled search input/result row
    verification:
      - kind: unit
        ref: "frontend/src/shell/QuickSwitcher.test.tsx, ErrorBoundary.test.tsx, AppLogStream.test.tsx (17 tests)"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/shell/QuickSwitcher.tsx,src/shell/QuickSwitcher.module.css,src/shell/ErrorBoundary.tsx,src/shell/ErrorBoundary.module.css,src/shell/AppLogStream.tsx --proposal design-system/exception-proposals/shell.json"
        status: pass
    human_judgment: false
  - id: D3
    description: React remains projection-only; navigation, hotkey, error-recovery, and log-truth dispatch paths are unchanged
    verification:
      - kind: unit
        ref: "QuickSwitcher.test.tsx asserts onNavigate/onClose dispatch is unchanged; ErrorBoundary.test.tsx asserts window.location.reload is unchanged; AppLogStream.test.tsx asserts store-write/backlog-dedup behavior is unchanged"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (autonomous worktree execution; no wall-clock session boundary recorded)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 16: Shell Secondary Surfaces (Inspector/HelpOverlay/QuickSwitcher/ErrorBoundary/AppLogStream) Summary

**ContextualInspector, InspectorSlot, HelpOverlay, and QuickSwitcher now consume design-system tokens and the shared Dialog primitive; ErrorBoundary keeps a documented, exception-registered set of bare-literal colors for its own token-independent emergency fallback; a real pre-existing focus-indicator accessibility gap in QuickSwitcher was found and fixed along the way.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** Task 1 passes with zero diagnostics and no exceptions needed; Task 2 passes with 9 narrow, individually-verified exceptions (7 ErrorBoundary color/background literals, 1 ErrorBoundary Reload button, 1 QuickSwitcher search input, 1 QuickSwitcher result-row button)
- **Focused tests:** 27/27 pass across the two tasks' own test files; full frontend suite 477/477 pass; `tsc --noEmit` clean

## Accomplishments

- `ContextualInspector.module.css`: replaced undefined legacy `--space-md`/`--line`/`--page` vars (removed from `index.css` by Plan 13-08, leaving this file's styling silently broken) with `--ds-spacing-space4`/`--ds-border-default`/`--ds-surface-canvas`.
- `HelpOverlay.tsx`: replaced its hand-rolled backdrop+dialog pair with the shared, packaged-proven `Dialog` primitive (13-06); Close is now a design-system `Button`.
- `QuickSwitcher.tsx`: replaced its hand-rolled backdrop+dialog pair with `Dialog`, using a visually-hidden (`.ds-sr-only`) title to preserve the previous no-visible-heading layout while giving the dialog a real accessible name; removed local Escape handling (Dialog owns it) to avoid a double `onClose` call; fixed `.resultSelected` to use the real `--ds-surface-selected` token instead of a `color-mix()` against an undefined `--accent-5` var.
- Fixed a real DS010 accessibility violation: `QuickSwitcher.module.css`'s `.input:focus { outline: none; }` removed the focus indicator with no compensating replacement. Replaced with a `:focus-visible` ring via longhand `outline-width`/`-style`/`-color`/`-offset` tokens.
- `ErrorBoundary.module.css`: converted every non-color declaration (spacing, radius, typography) to `--ds-*` tokens; kept its 7 color/background declarations as intentionally bare literals (registered exceptions) since this is the app's last line of defense against a blank window and must remain readable even if the generated token stylesheet itself failed to load.
- Registered `frontend/design-system/exception-proposals/shell.json` (9 records) documenting every narrow domain/token-independence exception with rationale, source, owner, and review condition.
- `AppLogStream.tsx` needed no changes (renders `null`, no CSS or policed native controls).

## Task Commits

1. **Task 1: Migrate inspector and help overlay** — `4ae8dc64`
2. **Task 2: Migrate quick switcher, error boundary, and log projection** — `4b67ca03`

## Files Created/Modified

- `frontend/src/shell/ContextualInspector.module.css` — legacy vars replaced with `--ds-*` tokens.
- `frontend/src/shell/HelpOverlay.tsx` / `.module.css` / `.test.tsx` — migrated to the `Dialog` primitive.
- `frontend/src/shell/QuickSwitcher.tsx` / `.module.css` / `.test.tsx` — migrated to the `Dialog` primitive; fixed the focus-indicator gap.
- `frontend/src/shell/ErrorBoundary.tsx` / `.module.css` — non-color properties tokenized; color/background literals registered as exceptions.
- `frontend/design-system/exception-proposals/shell.json` — new, 9 exception records.

## Decisions Made

See `key-decisions` in frontmatter for full rationale on: Dialog adoption for HelpOverlay/QuickSwitcher, the visually-hidden Dialog title pattern, removing QuickSwitcher's redundant Escape handling, the `--ds-surface-selected` fix, the DS010 focus-indicator fix, ErrorBoundary's intentional non-token-independence-for-colors-only split, and the two byte-identical-value collision resolutions (hex vs. equivalent `rgb()`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a real, pre-existing focus-indicator accessibility gap in QuickSwitcher**
- **Found during:** Task 2, while resolving the scoped design-system check's DS010 diagnostic
- **Issue:** `.input:focus { outline: none; }` removed the browser's default focus ring with no compensating visible indicator at all, violating the Interaction State Contract's "never removed without an equally visible replacement."
- **Fix:** Replaced with `.input:focus-visible { outline-width: var(--ds-focus-ring); outline-style: solid; outline-color: var(--ds-focus-color); outline-offset: var(--ds-focus-offset); }` (longhand, since the shorthand `outline: var(...) solid var(...)` form fails DS001's strict single-`var()`-per-declaration check).
- **Files modified:** `frontend/src/shell/QuickSwitcher.module.css`
- **Verification:** Scoped design-system check shows zero DS010 findings; `QuickSwitcher.test.tsx`'s focus-on-open test still passes (focus itself is unaffected, only the visible ring changed).
- **Committed in:** `4b67ca03`

**2. [Rule 3 - Blocking] Registered ErrorBoundary/QuickSwitcher design-system exceptions to unblock the scoped check**
- **Found during:** Task 2
- **Issue:** ErrorBoundary's intentionally-literal color/background declarations (7) and its raw Reload `<button>` (DS005), plus QuickSwitcher's unlabeled search `<input>` and role="option" result `<button>` (DS005 each), are legitimate, narrow domain/architecture exceptions, not bugs to fix by converting them to tokens/primitives that would defeat their own purpose.
- **Fix:** Registered all 9 in `frontend/design-system/exception-proposals/shell.json`, each with an exact single-diagnostic match, rationale, and review condition; added `--proposal design-system/exception-proposals/shell.json` to Task 2's verify command (the plan's own text omitted it, matching 13-13's precedent of adjusting the declared verify command when a proposal file is genuinely required).
- **Verification:** `node scripts/design-system/check.mjs --paths ... --proposal design-system/exception-proposals/shell.json` — exit 0, zero diagnostics.
- **Committed in:** `4b67ca03`

**3. [Rule 3 - Blocking] Resolved two byte-identical DS001 value collisions in ErrorBoundary.module.css**
- **Found during:** Task 2
- **Issue:** `.title`'s and `.reload`'s `color: #e23a2e` were byte-identical, as were `.body`'s and `.detail`'s `color: #8a887f` — the checker's exception-matching mechanism (fixed in 13-13) can only resolve a match to exactly one diagnostic per rule+path+value, so neither pair could be exempted individually.
- **Fix:** Wrote the second occurrence of each as an equivalent `rgb()` form (`rgb(226, 58, 46)` and `rgb(138, 136, 127)` respectively) — textually distinct, zero rendered color change — following 13-13's established resolution pattern for this exact class of collision.
- **Verification:** `node scripts/design-system/check.mjs --paths src/shell/ErrorBoundary.module.css` (no proposal) shows both collisions gone from the raw diagnostic list; each now resolves to its own single exception.
- **Committed in:** `4b67ca03`

### Rejected Plan Elements

None. `13-PATTERNS.md`, referenced by this plan's `<context>`, does not exist in this worktree (an untracked file from a concurrent session that was never committed to the base this worktree branched from) — its guidance was substituted with the equivalent, already-committed precedent in `13-13-SUMMARY.md` (Dialog adoption, IconButton `disabledBehavior="soft"`, `--ds-card-font-color-*` pattern, `utilities.css`'s `.ds-sr-only`, and the checker's exception-matching/byte-identical-collision fixes), which this plan's own `<files_to_read>` instructions already pointed to as the primary reference.

---

**Total deviations:** 3 auto-fixed (1 missing-critical accessibility fix, 2 blocking/exception-registration issues)
**Impact on plan:** All auto-fixes were necessary for correctness (a real accessibility gap) or to unblock the declared verification gate with legitimate, narrowly-scoped, individually-verified exceptions. No scope creep.

## Known Stubs

None.

## Issues Encountered

- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/13-PATTERNS.md`, referenced by this plan's `<context>` block, does not exist in this worktree (it appears as an untracked file in the main repository's working tree from a concurrent session, but was never committed to the commit this worktree branched from, so it was not available here). Substituted `13-13-SUMMARY.md`'s already-committed, equivalent guidance per this plan's own `<files_to_read>` instructions, which named it as the primary reference for exactly this situation.
- `frontend/node_modules` did not exist in this worktree (each git worktree needs its own install); ran `npm ci --prefer-offline --no-audit --no-fund` once at the start of execution to restore it before running any tests or the checker.

## Next Phase Readiness

`ContextualInspector`, `InspectorSlot`, `HelpOverlay`, `QuickSwitcher`, `ErrorBoundary`, and `AppLogStream` are fully migrated (or, for `ErrorBoundary`'s colors, deliberately and narrowly excepted) with zero unregistered violations. The visually-hidden-Dialog-title pattern and the DS010 focus-visible longhand pattern are available to any remaining Wave 7 plan that needs a titleless dialog or a compensating focus ring on a bare `<input>`.

## Self-Check: PASSED

- Commits `4ae8dc64` and `4b67ca03` exist and contain all declared files.
- `frontend/design-system/exception-proposals/shell.json` exists with 9 records.
- Both tasks' declared verify commands pass (Task 2's adjusted to add `--proposal design-system/exception-proposals/shell.json`, matching 13-13's precedent).
- Full frontend suite (`npx vitest run`) — 477/477 pass. `npx tsc --noEmit` — clean. `node scripts/design-system/check.mjs --rule DS007` — clean.
