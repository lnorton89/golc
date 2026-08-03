---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: 15
subsystem: safety-live-tempo-ui
tags: [react, design-system, safety, hold-to-confirm, chip, number-stepper]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
provides:
  - Design-system-migrated SafetyCluster with an extracted, independently
    testable useHoldToConfirm state machine
  - Design-system-migrated LiveStatusBar (shared Chip primitive) and
    TempoControls (shared Button/NumberStepper primitives)
  - Additive NumberStepper `step`/`onBlur`/`onKeyDown` props for callers
    needing fractional nudges and commit-on-Enter/blur semantics
affects: [remaining Wave 7 shell/chrome migrations (13-24 core shell/command
  rail, 13-29 hotkey/shared chrome) that touch the same GlobalFrame header
  legacy-token consumers this plan closed out]
tech-stack:
  added: []
  patterns:
    - "A hold-to-confirm control's dynamic per-instance color is always a JS-set --ds-<name>-* custom property (never a CSS-side var(--x, fallback)) -- the --ds- prefix alone bypasses DS003's 'unknown custom property' check regardless of whether the name is a registered token, matching 13-13's --ds-card-font-color-* precedent."
    - "An active/error state ring on a control that also needs its own :focus-visible outline is rendered via a ::after border overlay (border-style/border-width/border-color longhand, never the box-shadow/outline shorthand) so the two visual states never collide on the same CSS property."
    - "A hold-progress fill (or any decorative absolutely-positioned overlay inside a flex-item-bearing native control) needs no z-index token at all if it is rendered FIRST in DOM order: flex items paint as if position:relative per the flexbox spec, so a later-in-DOM sibling at the same (auto) stack level always paints on top."
    - "NumberStepper gained optional step (default 1) and onBlur/onKeyDown pass-through props, both opt-in and backward compatible, for a caller needing fractional nudges and its own commit-on-Enter/Escape/blur semantics without duplicating its compact input+spinner markup."
key-files:
  created:
    - frontend/src/components/SafetyCluster/SafetyCluster.test.tsx
    - frontend/e2e/safety-action-hold.spec.ts
    - frontend/design-system/exception-proposals/safety-live.json
  modified:
    - frontend/src/components/SafetyCluster/SafetyCluster.tsx
    - frontend/src/components/SafetyCluster/SafetyCluster.module.css
    - frontend/src/components/LiveStatusBar/LiveStatusBar.tsx
    - frontend/src/components/LiveStatusBar/LiveStatusBar.module.css
    - frontend/src/components/TempoControls/TempoControls.tsx
    - frontend/src/components/TempoControls/TempoControls.module.css
    - frontend/src/components/primitives/NumberStepper/NumberStepper.tsx
    - frontend/src/components/primitives/NumberStepper/NumberStepper.test.tsx
key-decisions:
  - "Rejected the plan's literal 'compose SafetyAction' instruction for the actual hold gesture: SafetyAction's own onInvoke fires immediately on click, which is structurally incompatible with D-14's mandatory hold-to-confirm contract (dispatch only at threshold, never on click). Reused SafetyAction's underlying visual language (Button, destructive-adjacent semantic tokens) directly via a custom useHoldToConfirm hook instead, documented here rather than silently diverging from the plan text."
  - "Bumped SafetyCluster's control height from the pre-existing 28px to --ds-sizing-control-minimum-target (44px) to satisfy the plan's own must_haves truth ('compact shell targets remain at least 44px') -- a real, deliberate sizing increase, not a token-only reskin. Verified this still fits GlobalFrame's fixed 52px header without layout changes to that file (out of this plan's scope)."
  - "Added a per-control aria-live=assertive failure announcement (role=\"alert\", visually-hidden via the shared .ds-sr-only utility) plus a visible destructive-toned ring on dispatch failure -- neither existed before (the original safetyBlackout/etc. calls were fire-and-forget with zero error surfacing). This is new, previously-missing behavior explicitly required by the plan's own <behavior> block, not scope creep."
  - "Did NOT adopt NumberStepper's always-visible label+input for TempoControls' BPM field with a UX change to match: kept the exact existing click-to-edit toggle (button <-> field swap), preserving every assertion in the pre-existing TempoControls.test.tsx unchanged. NumberStepper is used only as the rendering primitive inside the already-existing 'editing' branch."
  - "LiveStatusBar's local StatusChip/STATUS_COLOR_VAR was fully replaced by the shared Chip primitive rather than migrated in place: the daemon's controllingSource/outputState vocabulary is byte-identical to Chip's own ChipTone union, so this was a straightforward primitive swap, not a reimplementation."
  - "No design-system exception was registered for either task (design-system/exception-proposals/safety-live.json stays empty, mirroring 13-13's empty fixtures.json): every DS001/DS003/DS005/DS006 finding was resolved via a genuine token/primitive/architecture fix -- none were routed around."
patterns-established:
  - "useHoldToConfirm (SafetyCluster.tsx): one hook implementing the full press-and-hold contract (determinate progress, single threshold timer, per-hold exactly-once latch, cancellation on early release/pointercancel/blur/window-blur/Escape) -- reusable verbatim by any future hold-to-confirm control in this codebase."
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-13, D-14, UI-SPEC-SAFETY]
coverage:
  - id: D1
    description: "SafetyCluster's three hold-to-confirm safety controls (Blackout, Automation, Stop/Release-All) migrated to design-system tokens/primitives with an independent, exactly-once, cancellable hold state machine"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "frontend/src/components/SafetyCluster/SafetyCluster.test.tsx (12 tests): threshold-minus-one/exact-threshold, continued hold, duplicate terminal events, every cancellation path (pointerup/pointerleave/pointercancel/blur/window-blur/Escape/keyup), successful retry, keyboard Space/Enter with repeat-guard, per-control busy/error independence"
        status: pass
      - kind: e2e
        ref: "frontend/e2e/safety-action-hold.spec.ts (6 tests, Chromium): real held-down mouse press to completion, early release cancels, real pointerleave cancels, keyboard Tab-away blur cancels, real Escape cancels then a fresh hold completes, all three controls stay visible/enabled"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/components/SafetyCluster --proposal design-system/exception-proposals/safety-live.json"
        status: pass
    human_judgment: false
  - id: D2
    description: "LiveStatusBar and TempoControls migrated to design-system primitives/tokens (Chip, Button, NumberStepper), preserving every existing bridge handler, accessible name, and interaction contract"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "frontend/src/components/LiveStatusBar/LiveStatusBar.test.tsx (2 tests) and frontend/src/components/TempoControls/TempoControls.test.tsx (6 tests) -- both pre-existing suites, unmodified, still pass"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/components/LiveStatusBar,src/components/TempoControls"
        status: pass
    human_judgment: false
  - id: D3
    description: "React remains projection-only; every safety/live/tempo command dispatch path to window.go.wails.* is unchanged"
    verification:
      - kind: unit
        ref: "SafetyCluster.test.tsx asserts Blackout/RevokeAutomation/StopReleaseAll called with the correct toggle argument exactly once per completed hold; TempoControls.test.tsx asserts dispatch.setBPM/recordTap called unchanged"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (spans a single continuous execution session)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 15: Safety Cluster, Live Truth, and Tempo Controls Migration Summary

**SafetyCluster/LiveStatusBar/TempoControls fully consume shared design-system primitives and tokens with zero exceptions; the hold-to-confirm gesture was extracted into an independently-tested, exactly-once state machine, and each safety control now surfaces a real dispatch-failure announcement that never existed before.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** both tasks pass with zero diagnostics and zero exceptions (`design-system/exception-proposals/safety-live.json` stays empty)
- **Focused tests:** SafetyCluster 12/12 (unit) + 6/6 (Playwright/Chromium); LiveStatusBar 2/2; TempoControls 6/6; NumberStepper 6/6 (2 new). Full frontend suite: 490/490 pass. `tsc --noEmit` clean. `npm run build` (tsc + vitest + vite build) clean.

## Accomplishments

- Migrated `SafetyCluster.tsx`/`.module.css` off every legacy `--space-*`/`--ink`/`--line`/`--page`/`--accent-5`/`--radius-sm`/`--motion-settle` token onto `--ds-*` equivalents, closing one of the three files (`13-08`'s legacy-token removal) directly responsible for the user-reported garbled top-bar rendering.
- Extracted `useHoldToConfirm`, one shared hold-to-confirm state machine for pointer and keyboard activation: determinate 0..1 progress, a single threshold timer, a per-hold exactly-once dispatch latch, and cancellation on early release, `pointercancel`, `pointerleave`, element blur, window blur, or Escape.
- Bumped each safety control's height from 28px to `--ds-sizing-control-minimum-target` (44px), the plan's own required minimum for this embedded-in-shell-header context.
- Added a previously-missing per-control failure surface: an `aria-live="assertive"` `role="alert"` announcement plus a visible destructive-toned ring, without ever disabling the control (D-13's "stays available ... in busy/error states").
- Solved the active/error-ring-vs-focus-outline collision with a `::after` border overlay (not `box-shadow`/`outline` shorthand), and the hold-progress-fill-vs-icon/label stacking order with DOM ordering alone (no `z-index` token needed).
- Migrated `LiveStatusBar.tsx` off its local `StatusChip`/`STATUS_COLOR_VAR` onto the shared `Chip` primitive (its `ChipTone` vocabulary is byte-identical to the daemon's `controllingSource`/`outputState` strings) and every remaining legacy token.
- Migrated `TempoControls.tsx` off three raw `<button>`s and one raw `<input>` onto `Button`/`NumberStepper`, preserving the exact click-to-edit toggle, 0.1 BPM nudge, and Enter/Escape/blur commit handlers.
- Extended `NumberStepper` with an optional `step` (default 1, backward compatible) and `onBlur`/`onKeyDown` pass-through props so `TempoControls` could reuse its compact input+spinner markup instead of duplicating a hand-rolled, unstyleable native input+button pair.

## Task Commits

1. **Task 1: Migrate persistent independent safety controls** — `35e3d64c`
2. **Task 2: Migrate live truth and tempo controls** — `c8faa16f`

## Files Created/Modified

- `frontend/src/components/SafetyCluster/SafetyCluster.tsx` - hold-to-confirm state machine + Button composition
- `frontend/src/components/SafetyCluster/SafetyCluster.module.css` - token migration, `::after` ring overlays
- `frontend/src/components/SafetyCluster/SafetyCluster.test.tsx` - new fake-timer regression suite (12 tests)
- `frontend/e2e/safety-action-hold.spec.ts` - new real-browser Playwright suite (6 tests)
- `frontend/design-system/exception-proposals/safety-live.json` - created, empty (no exceptions needed)
- `frontend/src/components/LiveStatusBar/LiveStatusBar.tsx` - Chip primitive adoption, token migration
- `frontend/src/components/LiveStatusBar/LiveStatusBar.module.css` - token migration, DS006-safe class renames
- `frontend/src/components/TempoControls/TempoControls.tsx` - Button/NumberStepper adoption
- `frontend/src/components/TempoControls/TempoControls.module.css` - drastically simplified (primitives own their own paint now)
- `frontend/src/components/primitives/NumberStepper/NumberStepper.tsx` - additive `step`/`onBlur`/`onKeyDown` props
- `frontend/src/components/primitives/NumberStepper/NumberStepper.test.tsx` - 2 new tests for the additive props

## Decisions Made

See `key-decisions` in frontmatter for full rationale. Summary: rejected the plan's literal "compose SafetyAction" instruction for the hold gesture itself (incompatible immediate-onClick contract), bumped control sizing to the required 44px minimum, added a previously-missing failure-announcement surface, and resolved every checker finding through genuine primitive/token/architecture fixes rather than exceptions.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added per-control failure announcement and visible error ring**
- **Found during:** Task 1
- **Issue:** The pre-existing `safetyBlackout`/`safetyRevokeAutomation`/`safetyStopReleaseAll` calls were fire-and-forget (`void safetyBlackout(...)`) with zero error handling -- if a safety command actually failed (e.g. daemon unreachable), the operator received no feedback at all that the action didn't take effect. The plan's own `<behavior>` block explicitly requires "Failure announcements are assertive."
- **Fix:** Added per-control `busy`/`error` state; on a non-zero exit code or thrown rejection, set an `aria-live="assertive"` `role="alert"` message (visually hidden via the shared `.ds-sr-only` utility, since GlobalFrame's fixed 52px header has no spare layout room for visible inline copy) plus a visible destructive-toned `::after` ring, without ever disabling the control.
- **Files modified:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx`, `SafetyCluster.module.css`
- **Verification:** `SafetyCluster.test.tsx`'s "keeps each control's busy/error state independent" test; scoped checker shows zero findings for the new markup.
- **Committed in:** `35e3d64c`

**2. [Rule 1 - Bug] Fixed the DS001 z-index/stacking pattern via DOM order instead of a token**
- **Found during:** Task 1
- **Issue:** The original hold-progress fill relied on `z-index: 1` (a raw literal, no `--ds-stacking-*` token maps to "1") on the icon/label to paint above the fill overlay.
- **Fix:** Rendered the fill span first in DOM order (no `leadingIcon` prop on `Button`; icon rendered manually as a later child) -- flexbox's own "flex items paint as if position:relative" rule plus DOM-order tiebreaking within the same stack level achieves the identical visual result with zero `z-index` declarations anywhere.
- **Files modified:** `frontend/src/components/SafetyCluster/SafetyCluster.tsx`, `SafetyCluster.module.css`
- **Verification:** Scoped checker shows zero DS001 findings; `SafetyCluster.test.tsx` and the Playwright suite both pass, confirming visible/interactive behavior is unchanged.
- **Committed in:** `35e3d64c`

**3. [Rule 3 - Blocking] Extended NumberStepper additively instead of hand-rolling TempoControls' input**
- **Found during:** Task 2
- **Issue:** TempoControls' raw `<input type="number">` with a custom 0.1-BPM-step spinner triggered DS005 (styled native control); no existing primitive supported both a compact micro-spinner (IconButton's 28px floor is too large for this exact geometry) and a fractional nudge amount.
- **Fix:** Added optional `step` (default 1, backward compatible) and `onBlur`/`onKeyDown` pass-through props to `NumberStepper` (a primitives-directory file, exempt from DS005), then adopted it in `TempoControls.tsx`'s existing editing-mode branch -- preserving the exact click-to-edit toggle and Enter/Escape/blur commit semantics.
- **Files modified:** `frontend/src/components/primitives/NumberStepper/NumberStepper.tsx`, `NumberStepper.test.tsx`, `frontend/src/components/TempoControls/TempoControls.tsx`, `TempoControls.module.css`
- **Verification:** `NumberStepper.test.tsx`'s 2 new tests (fractional step, onBlur/onKeyDown forwarding); `TempoControls.test.tsx`'s pre-existing 6 tests pass unmodified; scoped checker shows zero DS005 findings.
- **Committed in:** `c8faa16f`

---

**Total deviations:** 3 auto-fixed (1 missing-critical, 2 blocking)
**Impact on plan:** All three were necessary for correctness (failure feedback), token-compliance (stacking order), or completing the migration without an exception (NumberStepper extension). No scope creep beyond what each task's own `<behavior>`/`<action>` already required.

## Issues Encountered

- `13-PATTERNS.md`, referenced by the plan's `<action>` text ("Follow the exact migration-in-place analog row `13-PATTERNS.md:18`"), does not exist in this worktree (confirmed via repo-wide glob) -- it is apparently an uncommitted artifact from a sibling in-progress plan's session, not yet merged to the commit this worktree was based on. Proceeded using the plan's own `<behavior>`/`<action>` text, the referenced `13-06-SUMMARY.md`/`13-07-SUMMARY.md`, and the most recently completed sibling (`13-13-SUMMARY.md`) as the migration precedent instead.
- No live mock-bridge browser preview was performed (per `docs/skills/frontend-verify/SKILL.md`'s guidance) -- this execution environment did not expose a browser-preview tool. Compensating evidence: the full `npm run build` (which runs the real `vite build`/CSS-Modules pipeline that previously caught a mid-line-`*/`-comment bug undetectable by `tsc`/`vitest`/the checker's own bare `postcss.parse()`) passed cleanly, and both edited `*.module.css` files were manually verified to have balanced `/*`/`*/` comment-delimiter counts.

## Next Phase Readiness

Three of the files the user-reported top-bar/left-nav rendering bug traced to (`13-13-SUMMARY.md`'s "Issues Encountered") are now migrated: `SafetyCluster.tsx`, `LiveStatusBar.tsx`, `TempoControls.tsx`. The remaining two named in that trace -- `13-24` (core shell/command rail) and `13-29` (hotkey/shared chrome) -- still need to land before the legacy-token-removal gap (`13-08`) is fully closed; `GlobalFrame.module.css` itself (the actual header chrome, out of this plan's `files_modified` scope) still references legacy tokens and is presumably one of those two plans' targets.

## Self-Check: PASSED

- Commits `35e3d64c` and `c8faa16f` exist and contain all 11 declared files (verified via `git log --oneline` and `git diff --diff-filter=D` showing zero accidental deletions in either commit).
- `frontend/src/components/SafetyCluster/SafetyCluster.test.tsx`, `frontend/e2e/safety-action-hold.spec.ts`, and `frontend/design-system/exception-proposals/safety-live.json` all exist on disk.
- Both tasks' exact plan-declared verify commands pass with exit 0.
- Full frontend suite (490/490), `tsc --noEmit`, and `npm run build` all pass cleanly.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
