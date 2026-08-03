---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "13"
subsystem: desk-perform-ui
tags: [react, design-system, desk, perform, fader, midi-learn, checker-bug]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
affects: [remaining Wave 7 workspace migrations, any future plan registering 2+ same-rule exceptions in one file]
tech-stack:
  added: []
  patterns:
    - "IconButton disabledBehavior=\"soft\" (aria-disabled, not the native attribute) for controls that must stay hoverable/focusable while inert, so a title tooltip explaining why an action is unavailable isn't suppressed."
    - "IconButton size=\"compact\" (28px/16px icon) for a small cluster of icon buttons inside an already-dense row, distinct from size=\"target\" (44px) for a control that is the primary way to reach an action."
    - "A dynamic per-instance custom property with a conditional operator-chosen override and a role-specific default (e.g. --ds-card-font-color-primary/-secondary) is always set in JS (both the override and the fallback), never expressed as `var(--x, fallback)` in CSS -- the latter fails DS001 (not a bare token) and DS003 (fallback usage), and multiple same-shaped CSS call sites collapse to one indistinguishable DS003 diagnostic value the exception mechanism can't isolate."
    - "A narrow, single-purpose global (non-CSS-Module) utility class belongs in src/design-system/utilities.css (design-system files are exempt from DS001-DS006), not duplicated locally per component."
key-files:
  modified:
    - frontend/src/components/Desk/Desk.tsx
    - frontend/src/components/Desk/Desk.module.css
    - frontend/src/components/Desk/Fader.tsx
    - frontend/src/components/Desk/FixtureStyleModal.tsx
    - frontend/src/components/Desk/FixtureStyleModal.module.css
    - frontend/src/components/primitives/IconButton/IconButton.tsx
    - frontend/src/components/primitives/IconButton/IconButton.module.css
    - frontend/src/components/primitives/IconButton/IconButton.test.tsx
    - frontend/src/index.css
    - frontend/design-system/runtime-geometry.json
    - frontend/scripts/design-system/check.mjs
  created:
    - frontend/src/components/Desk/FaderLearnHitArea.tsx
    - frontend/src/components/Desk/Desk.test.tsx
    - frontend/src/design-system/utilities.css
    - frontend/design-system/exception-proposals/desk.json
key-decisions:
  - "Fixed a real bug in the design-system checker's exception-matching mechanism (scripts/design-system/check.mjs) rather than routing around it: the substring fallback (`sourceByPath.get(path).includes(match)`) checked the WHOLE FILE's source text, independent of which diagnostic was under test, so once any exception's match string appeared anywhere in a file, it silently matched EVERY diagnostic of that rule in that file -- making it structurally impossible to except more than one violation of the same rule per file no matter how precisely the match text was written. Fixed to prefer exact diagnostic.value equality first (falling back to per-diagnostic, not file-wide, substring containment only when no exact match exists), verified against the existing 4-case test matrix (accepts-exact/rejects-stale/rejects-broad/never-accepts-spacing) plus the new Desk.module.css cases. This was a correctness fix (precise exceptions now resolve correctly), not a weakening (nothing that was a genuine violation now silently passes)."
  - "Superseded both slider-thumb pseudo-elements' border-radius from 2px to --ds-radii-small (4px) rather than except them: they carried byte-identical diagnostic values (::-webkit-slider-thumb and ::-moz-range-thumb), which the exception mechanism cannot distinguish even after the matching fix (two diagnostics with the same rule+path+value are inherently ambiguous, by design -- see the checker's own 'rejects a broad exception' test). User-approved trade of a modest visual change (a 10px-tall thumb reads as somewhat more rounded) for a real token and zero cross-browser inconsistency."
  - "Loosened .faderScaleTickNumber's line-height from 1 to 1.1 (keeping .faderValue's own line-height:1, which has a real fixed-height centering constraint) rather than except both: same byte-identical-value collision as the thumb radius. The tick number isn't centered inside a fixed-height box, so it has no constraint forcing byte-identical text with .faderValue; 1.1 is imperceptible at its 11px font size. User-approved."
  - "Replaced Fader's linear-gradient track-line paint and the badge-family font-size clamp() exceptions' match strings to avoid the '*' character (calc()'s multiplication operator, and the gradient's own multi-line formatting collapsed to one line): the checker mechanically rejects `*`/`?`/newlines in any exception.match, so a clamp()/calc() declaration's own full value can never be its own exact match -- used unique, `*`-free suffix fragments instead (e.g. \"0.28), 11px)\"), relying on the (now diagnostic-scoped) substring-fallback tier."
  - "Replaced the two --ds-card-font-color(fallback) call-site problems (DS001 not-a-bare-token, DS003 fallback-usage, and 3 identical DS003 diagnostic values the checker can't tell apart) by having Desk.tsx's fixtureCardInlineStyle always set two role-specific custom properties (--ds-card-font-color-primary/-secondary), each already resolved to the operator's override or that role's own semantic default in JS -- eliminating the CSS-side fallback pattern entirely rather than working around the checker."
  - "Extracted a shared src/design-system/utilities.css with one .ds-sr-only class rather than except FaderLearnHitArea's local sr-only span's `margin: -1px`: 'margin' is a mechanically DS001-exception-banned word (spacing exceptions are categorically forbidden), and this exact 8-property sr-only boilerplate existed nowhere else in the codebase to reuse -- centralizing it is what should have existed already, not new scope creep."
  - "Fixed a second, unrelated real bug found only by the mock-bridge browser check (not tsc, not vitest, not the checker's own bare postcss.parse()): a CSS comment's own prose read \"--ds-surface-*/--ds-status-*\", whose \"*/\" substring closed the comment early, turning the rest of the sentence and prior context into a malformed CSS fragment that only the stricter postcss-modules-local-by-default plugin (Vite's real CSS-Modules pipeline) rejected. Reworded to avoid the accidental terminator; grepped the whole Desk file family for the same pattern (mid-line `*/`) and found no other instances."
  - "Did not adopt the LauncherMasters pattern the plan's must_haves named: it renders a 'name + value button' master-fader row (Operator Surface's own vocabulary), which has no counterpart in Desk's per-channel DMX override grid -- applying it would misrepresent Desk's actual UI, not unify it."
  - "Removed the rainbow/acid theme-branching accent-cycling CSS (a D-06 violation with no equivalent in the generated token set) and added IconButton's disabledBehavior=\"soft\" mode, both per explicit user approval earlier in this plan's execution."
patterns-established:
  - "When a checker's own exception-matching mechanism can't distinguish two violations because their diagnostic text is byte-identical (not because the match string is imprecise), the fix is a real, minimal, user-approved value change that removes the collision -- not a broader exception, not a checker weakening, and not blind file-fragmentation for its own sake."
  - "Before writing an exception whose match string is the violation's full value, check it against the checker's own forbidden-character rule (`*`, `?`, newlines) -- clamp()/calc() expressions and multi-line formatted values routinely contain a literal `*` or embedded newline and need a shorter, character-safe fragment instead."
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-13, D-14]
coverage:
  - id: D1
    description: Desk uses public primitives while preserving fader inputs, commands, and stable identity
    verification:
      - kind: unit
        ref: "frontend/src/components/Desk/Desk.test.tsx (12 tests): empty state, per-channel setDeskAttribute/clearDeskAttribute dispatch, per-fixture/per-universe/all release, MIDI learn start/cancel/conflict/timeout, FixtureStyleModal save/cancel"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/perform/DeskWorkspace.tsx,src/components/Desk,src/design-system/utilities.css,src/index.css --proposal design-system/exception-proposals/desk.json"
        status: pass
    human_judgment: false
  - id: D2
    description: High-density fader geometry is typed runtime sizing or one exact evidenced domain proposal
    verification:
      - kind: static
        ref: "design-system/exception-proposals/desk.json (29 records, each verified to resolve to exactly one diagnostic); design-system/runtime-geometry.json (--ds-universe-height, --ds-fader-width added)"
        status: pass
    human_judgment: false
  - id: D3
    description: React remains projection-only and does not own playback or output
    verification:
      - kind: unit
        ref: "Desk.test.tsx asserts every mutation (SetAttribute/ClearAttribute/ClearInstance/ClearAll/StartDeskLearn/CancelLearn) dispatches to window.go.wails.* unchanged; no new local authority introduced"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (spans a paused/resumed session, including an unplanned mid-plan investigation of an unrelated rendering-bug report)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 13: Desk (Perform) Migration Summary

**Desk.tsx/Fader.tsx/FixtureStyleModal.tsx fully consume shared design-system primitives and tokens with 29 narrow, individually-verified domain exceptions; a real bug in the design-system checker's exception-matching mechanism and a real CSS-comment-termination bug were found and fixed along the way; fader behavior, MIDI Learn, and every dispatch path are unchanged and covered by a new focused test suite.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** passes with zero diagnostics (18 DS001, 3 DS005, 9 DS006, 1 DS010 findings resolved — 6 via real token/architecture fixes eliminating the violation entirely, 29 via individually-verified narrow exceptions)
- **Focused tests:** 12/12 pass (Desk.test.tsx); full frontend suite 476/476 pass; `tsc --noEmit` clean; `npm run build` (tsc + vitest + vite build) clean

## Accomplishments

- Converted Desk.tsx's cards, badges, buttons, empty/loading/error states, and universe/fixture chrome to `Panel`/`Button`/`IconButton`/`Chip`/`EmptyState`/`LoadingState`/`ErrorState`.
- Converted FixtureStyleModal.tsx to the `Dialog` primitive; converted its 3 reset buttons to `IconButton` (soft-disabled); renamed `.imagePreviewEmpty`→`.imagePreviewPlaceholder`/`.uploadError`→`.uploadIssue` for clarity.
- Extended `IconButton` with `size="compact"` (28px/16px icon, for dense in-row clusters like universe/fixture release+edit buttons) and `disabledBehavior="soft"` (aria-disabled, keeps hover/focus so a title tooltip explaining why an action is unavailable isn't suppressed by the native `disabled` attribute).
- Extracted `FaderLearnHitArea.tsx` from `Fader.tsx` so the MIDI-learn click target's raw `<button>` is the only one the checker finds in that file (Fader.tsx's own `faderClearButton` is a second, separately-registered domain exception).
- Rewrote `Desk.module.css` end to end: every ordinary spacing/color/typography declaration now uses a generated `--ds-*` token; every genuinely irreducible domain-geometry declaration (the vertical fader input/thumb paint, fixed 18x18px clear button, MIDI-learn overlay pulse/z-index/focus-ring, fader-width-responsive `clamp()` typography) is registered in `design-system/exception-proposals/desk.json` with an exact, single-diagnostic match and documented rationale.
- Replaced the per-fixture `--ds-card-font-color` custom-property-with-CSS-fallback pattern (which failed DS001/DS003 and could never be exceptioned across its 3 identical-shaped call sites) with two always-set, role-specific custom properties (`--ds-card-font-color-primary`/`-secondary`) resolved entirely in `Desk.tsx`'s `fixtureCardInlineStyle`.
- Added `src/design-system/utilities.css` (`.ds-sr-only`) and pointed `FaderLearnHitArea.tsx`'s live-region span at it instead of a local, un-exceptable copy of the sr-only pattern.
- Wrote `Desk.test.tsx` (12 tests): empty state, per-channel fader change/clear, per-fixture/per-universe/all release, MIDI Learn start/cancel/conflict/timeout, FixtureStyleModal open/save/cancel.
- Registered `--ds-universe-height` and `--ds-fader-width` in `design-system/runtime-geometry.json`.

## Task Commits

1. **Task 1 + Task 2 (combined):** `5d142d4d` — Desk/Fader/FixtureStyleModal/IconButton conversion, the checker exception-matching bug fix, the CSS-comment-termination bug fix, `Desk.test.tsx`, `desk.json`, `utilities.css`, `runtime-geometry.json` updates.

## Verification

- `cd frontend && npx vitest run src/components/Desk src/components/primitives/IconButton src/design-system` — 57/57 pass.
- `cd frontend && npx vitest run` (full suite) — 476/476 pass.
- `cd frontend && npx tsc --noEmit` — clean.
- `cd frontend && node scripts/design-system/check.mjs --paths src/workspaces/perform/DeskWorkspace.tsx,src/components/Desk,src/design-system/utilities.css,src/index.css --proposal design-system/exception-proposals/desk.json` — exit 0, zero diagnostics. (`src/workspaces/perform/DeskWorkspace.module.css`, named in the plan's own verify command, does not exist — `DeskWorkspace.tsx` is a thin wrapper reading `../workspace.module.css`, the same shared chrome stylesheet every other workspace wrapper already uses; it needed no changes and has no CSS module of its own.)
- `cd frontend && node scripts/design-system/manifest.mjs` and `npx vitest run scripts/design-system` — pass (runtime-geometry.json's new entries and check.mjs's fix both validate cleanly).
- `cd frontend && npm run build` (`tsc --noEmit && vitest run && vite build`) — clean.
- Manual verification in a mock-bridge browser preview (`golc-desktop-frontend-dev`, port 4788) with a 2-instance/4-channel-each patched fixture: universe/fixture cards, badges, fader tracks and values, the icon-key legend, the universe release button, and the FixtureStyleModal open/save/cancel flow all render and interact correctly with zero console errors and zero broken (zero-size) icons; this is also where the CSS-comment-termination bug was actually caught (a `TypeError: Failed to fetch dynamically imported module: .../AppShell.tsx` from a PostCSS parse failure Vite's real CSS-Modules pipeline raised, that tsc/vitest/the checker's own bare `postcss.parse()` never surfaced).

## Deviations from Plan

### Auto-fixed Issues

1. **[Rule 3 - Blocking] Fixed a real bug in the design-system checker's exception-matching mechanism**
   - **Found during:** Task 2, while registering Desk's domain exceptions
   - **Issue:** `exceptionMatches()`'s substring-fallback (`sourceByPath.get(path).includes(match)`) tested whether the match string appeared anywhere in the *whole file*, not the specific diagnostic under test — so the moment any exception's match string appeared anywhere in a file, it counted as a match for *every* diagnostic of that rule in that file. A file with 2+ diagnostics of the same rule (Desk.module.css has 18 DS001 and 9 DS006) could never have even one of them individually excepted, no matter how precisely the match text was written — confirmed empirically with an isolated single-record test before concluding this wasn't a mistake in my own records.
   - **Fix:** User-approved (explicit choice among "fix the checker," "fragment the file," "stop and let the user look") — changed the matching logic to try exact `diagnostic.value` equality first (across only same-rule/same-path diagnostics), falling back to substring containment *scoped to each diagnostic's own value* only when no exact match exists anywhere. Removed the now-dead `sources` Map plumbing this replaced.
   - **Verification:** The checker's own existing 4-case test matrix (`check.test.ts`, 20 tests total) passes unchanged; Desk's 29-record `desk.json` resolves to zero diagnostics.

2. **[Rule 1 - Bug] Fixed an accidental early CSS-comment termination**
   - **Found during:** Task 2's mock-bridge browser verification (not caught by tsc, vitest, or the checker)
   - **Issue:** A comment's own prose, "--ds-surface-*/--ds-status-* option...", contained the literal substring `*/`, closing that comment early. Everything from that point through the next legitimate `*/` was parsed as malformed CSS by Vite's real CSS-Modules PostCSS pipeline (`postcss-modules-local-by-default`), which the checker's own bare `postcss.parse()` call tolerated silently.
   - **Fix:** Reworded to "--ds-surface-* and --ds-status-* option" (no adjacent `*` and `/`). Grepped the whole Desk file family for the same mid-line `*/` pattern; found no other instances.
   - **Verification:** Mock-bridge browser reload shows Desk rendering correctly with zero console errors; `npm run build` (which runs `vite build`, invoking the same real CSS-Modules pipeline) is clean.

3. **[Rule 3 - Blocking] Two byte-identical-value diagnostic collisions resolved with real, user-approved value changes**
   - **Found during:** Task 2
   - **Issue:** `.faderInput::-webkit-slider-thumb` and `::-moz-range-thumb` both had `border-radius: 2px` (identical diagnostic value); `.faderValue` and `.faderScaleTickNumber` both had `line-height: 1` (identical diagnostic value). Even after fixing the file-wide matching bug above, two diagnostics with byte-identical rule+path+value are inherently indistinguishable to any value-based exception mechanism (by the checker's own design — see its "rejects a broad exception" test).
   - **Fix:** User chose (via explicit question) to supersede both thumb radii to `--ds-radii-small` (4px, a real token, accepting a modestly more rounded thumb) rather than split files for no architectural reason; and to loosen the tick number's line-height to 1.1 (imperceptible at 11px, since it has no fixed-height centering constraint the way `.faderValue`'s pill does) rather than duplicate `.faderValue`'s own exact 1.
   - **Verification:** `node scripts/design-system/check.mjs --paths src/components/Desk` (no proposal) shows both collisions gone from the raw diagnostic list.

4. **[Rule 1 - Bug] Replaced the `--ds-card-font-color` fallback pattern instead of exceptioning it 3×2 times**
   - **Found during:** Task 2
   - **Issue:** `color: var(--ds-card-font-color, var(--ds-text-primary))` (and its `-secondary` sibling) appeared at 3 call sites, all with the identical DS003 "custom property fallback" diagnostic value (`--ds-card-font-color`) — un-exceptable for the same byte-identical-collision reason as issue 3, and DS002 forbids declaring the custom property's own default inside a non-design-system CSS file (ruling out an ordinary CSS-cascade default).
   - **Fix:** `Desk.tsx`'s `fixtureCardInlineStyle` now always sets two custom properties (`--ds-card-font-color-primary`/`-secondary`), each already resolved to the operator's override or that role's own default in JS; the 3 CSS call sites became bare, fallback-free `var(--ds-*)` references.
   - **Verification:** `Desk.test.tsx`'s FixtureStyleModal save test confirms the style still persists and applies; the scoped checker shows zero DS001/DS003 findings for these lines.

5. **[Rule 1 - Bug] Centralized the sr-only pattern instead of exceptioning `margin: -1px`**
   - **Found during:** Task 2
   - **Issue:** `FaderLearnHitArea.tsx`'s live-region span used the standard visually-hidden accessibility pattern locally in `Desk.module.css`; its `margin: -1px` cannot be exceptioned under any circumstance (`margin` is a mechanically banned word in the checker's own DS001 spacing-exception guard), and no shared `.sr-only`-equivalent existed anywhere else in the codebase to reuse.
   - **Fix:** Added `src/design-system/utilities.css` (`.ds-sr-only`), imported once from `index.css`; `FaderLearnHitArea.tsx` now uses the plain global class instead of a local CSS Module class.
   - **Verification:** Scoped checker shows zero findings for the removed rule; `Desk.test.tsx`'s MIDI-learn conflict/timeout tests (which render this span) pass unchanged.

### Rejected Plan Elements

1. **LauncherMasters pattern (named in must_haves) was not adopted**
   - **Reason:** `LauncherMasters` renders a "name + value button" master-fader row — Operator Surface's own vocabulary for its master-fader launcher grid. Desk has no "masters" concept; it is a per-channel DMX override grid grouped by universe/fixture. Applying `LauncherMasters` would misrepresent Desk's actual UI rather than unify it with a genuinely-shared pattern.
   - **What was used instead:** `Panel`/`Button`/`IconButton`/`Chip`/`EmptyState`/`LoadingState`/`ErrorState` — the primitives that actually fit Desk's real content.

## Known Stubs

None.

## Issues Encountered

- An unrelated, unplanned investigation consumed part of this plan's execution: the user reported garbled top-bar/left-nav rendering via `mage dev`. Root-caused to Plan 13-08 (already executed, commit `45c95f82`) having removed ~944 lines of legacy token definitions from `index.css` before its sibling Wave 7 plans (13-15 safety/live-truth/tempo, 13-24 core shell/command rail, 13-29 hotkey/shared-chrome) migrate the remaining consumers to `--ds-*` tokens — an expected, if visually jarring, intermediate state of the phased migration, not a regression from this plan's own work. No code change was needed for that investigation itself; it's documented here because it explains a real gap in wave sequencing that the still-pending 13-15/13-24/13-29 plans need to close.
- `src/workspaces/perform/DeskWorkspace.module.css`, named in the plan's own frontmatter (`files_modified`) and Task 1's verify command, does not exist. `DeskWorkspace.tsx` reads `../workspace.module.css` (shared workspace chrome, the same file every other workspace wrapper imports) and needed no changes.

## Next Phase Readiness

Desk's three files (`Desk.tsx`, `Fader.tsx`/`FaderLearnHitArea.tsx`, `FixtureStyleModal.tsx`) are fully migrated with zero unregistered violations. The design-system checker's exception-matching fix and the new `--ds-card-font-color-*`/`utilities.css` patterns are available to every remaining Wave 7 plan, several of which (13-24 core shell, 13-26 Art-Net/diagnostics, 13-27 Scripts/Monaco, 13-28 MIDI mapping, 13-29 hotkey/shared chrome) are dense enough to plausibly hit the same same-rule-multiple-diagnostics or byte-identical-value situations this plan resolved.

## Self-Check: PASSED

- Commit `5d142d4d` exists and contains all 15 declared files.
- The plan's declared verify command (adjusted for the nonexistent `DeskWorkspace.module.css`) passes: `cd frontend && npx vitest run src/components/Desk && node scripts/design-system/check.mjs --paths src/workspaces/perform/DeskWorkspace.tsx,src/components/Desk --proposal design-system/exception-proposals/desk.json`.
- Full frontend build gate (`npm run build`) passes.
