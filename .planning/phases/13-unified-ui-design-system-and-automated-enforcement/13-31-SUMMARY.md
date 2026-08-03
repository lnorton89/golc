---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "31"
subsystem: design-system-visual-verification
tags: [playwright, geometry, resize, long-text, monaco, tiptap, e2e, accessibility]
requires:
  - phase: 13-13
    provides: Desk's own domain-geometry exceptions (design-system/exception-proposals/desk.json), the exact carve-out this plan proves holds under real, expanded/edge-case content
  - phase: 13-30
    provides: waitForFonts/screenshot-tolerance/getBuildSha/evidence-JSON conventions this plan's two backstops build on
provides:
  - "frontend/e2e/design-system.geometry.spec.ts: a named 6-family specialized-geometry matrix (vertical faders, scene timelines, Art-Net/diagnostic meters, Monaco, Tiptap, resize extremes) at 900/1280px, with resize extremes driven to their exact registered runtime-geometry.json minimum/maximum"
  - "frontend/e2e/fixtures/expandedCopy.ts + design-system.expanded-copy.spec.ts: an explicit, grapheme-counted >=2.0x canonical/expanded copy fixture exercised across shell/dialog/shared-states/guided-first-show/status/field-help categories in light/dark at both required widths"
  - "evidence/specialized-geometry.json and evidence/expanded-copy.json: per-case measurements, expectations, and pass booleans"
affects: [13-32, 13-33, 13-34, 13-41]
tech-stack:
  added: []
  patterns:
    - "Reading a domain component's own set CSS custom property (--ds-fader-track-width, --ds-scenelist-width, etc.) via getComputedStyle as the expected value for a geometry assertion, rather than re-deriving the arithmetic by hand -- mirrors Desk.tsx's own computeFitFaderWidth precedent of trusting real rendered/set values over reimplemented formulas."
    - "A horizontal-only viewport-containment check (withinViewportWidth) for content inside a workspace's own vertically-scrolling canvas (workspace.module.css .canvas: overflow:auto) -- a resized-to-max panel legitimately extends below the initial window height and scrolls into view; only horizontal escape past the window edge is a real violation."
    - "A local, spec-scoped overflow checker (findDeskOverflowingControls) that skips the individual right-edge check for controls legitimately inside a horizontally-scrolling ancestor (fixtureScroll, scrollWidth > clientWidth) -- the shared helpers.ts findOverflowingControls has no concept of nested internal horizontal scroll and would otherwise false-positive on Desk's own intended per-universe scroll-to-reveal behavior at wide fader-width extremes."
    - "Range.getClientRects() (not getBoundingClientRect on the element) is the correct way to prove a block of text actually wrapped across multiple rendered lines -- one rect per line box, immune to a block element always reporting one box regardless of how many lines it wrapped."
    - "Unicode grapheme-cluster counting (Intl.Segmenter) for a copy-expansion-ratio fixture, not UTF-16 code-unit length -- measures '2x-length' the way a reader actually perceives it."
    - "When a UI-SPEC-required copy category has no existing data-driven, error-path-reachable surface (Guided First Show's stage evidence templates are all fixed count-based strings; wailsBridge.ts's list* wrappers never let a rejection propagate), pair two already-shipped REAL strings from the same live surface that happen to meet the required ratio, rather than fabricating an unreachable mocked error path."
key-files:
  created:
    - frontend/e2e/design-system.geometry.spec.ts
    - frontend/e2e/fixtures/expandedCopy.ts
    - frontend/e2e/design-system.expanded-copy.spec.ts
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/specialized-geometry.json
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/expanded-copy.json
  modified:
    - frontend/src/shell/TitleBar.tsx
key-decisions:
  - "index.css declares a global `*, *::before, *::after { box-sizing: border-box; }` reset (confirmed empirically, contradicting a stale in-repo comment on .faderValue claiming 'no global box-sizing reset') -- geometry assertions against Desk's own set --ds-universe-height/--ds-fader-width read this correctly (the CSS `height`/`width` property IS the full border-box, no separate padding/border addition needed)."
  - "Desk's per-universe fixtureScroll (overflow-x:auto) legitimately scrolls a later fixture's fader/clear-button past its own currently-visible right edge at wide fader-width extremes -- this is the intended scroll-owner behavior (D-13), not an escape, so the geometry spec's own containment checks assert vertical containment plus 'never escapes left' rather than a strict full-viewport-width bound for elements inside that specific scroll container."
  - "Monaco keyboard-typing verification uses page.keyboard.insertText (one atomic input event) instead of per-character keyboard.type -- the latter occasionally raced Monaco's own async TypeScript-worker-backed model updates and landed characters out of order, a harness timing artifact unrelated to the product."
  - "The 'guided-first-show' expanded-copy pair uses two already-shipped REAL evidence strings (Assign stage's short 'No operator surface yet' blocker detail and its always-appended, notably longer 'MIDI hardware (optional)' evidence detail) rather than a mocked error message: every readiness.ts derivation renders only fixed, count-based templates (no user-authored name ever appears in guide evidence text), and wailsBridge.ts's list* wrappers structurally never let a rejection reach a stage's own ErrorState (every one catches and falls back to an explicit empty/offline projection) -- making a mocked-error injection route both inauthentic to the app's real behavior and mechanically unreachable."
  - "GuidedFirstShow.module.css's own documented 'safety valve' (evidenceAside collapses toward 0 width before contentArea's overflow-x:auto engages, below stageSection's 460px floor) is a pre-existing, reviewed tradeoff -- confirmed empirically to be width-driven, not content-length-driven (the short canonical string collapses identically to the long expanded one at 900px). The expanded-copy spec asserts the actually-guaranteed invariant in that state (contentArea becomes the compensating scroll owner, text is never lost) rather than assuming a comfortable wrap width the aside's own box cannot offer at 900px."
patterns-established:
  - "A copy-expansion-ratio fixture (canonical/expanded pair + Intl.Segmenter-based grapheme counting + a hard assertPairsMeetMinimumExpansion() guard called at module scope, before any browser opens) is the reusable shape for any future 'prove reflow at Nx copy length' backstop."
  - "measureTextWrap/measureTextWrapByContent (Range.getClientRects() line-count + scrollHeight/clientHeight clipping check) is the reusable primitive for 'did this text genuinely wrap without clipping,' independent of the specific component under test."
requirements-completed: [D-02, D-03, D-10, D-12, D-13, D-14, UI-CONSIDERATIONS-BACKSTOP-GEOMETRY, UI-CONSIDERATIONS-BACKSTOP-LONG-TEXT]
coverage:
  - id: D1
    description: "Specialized geometry (vertical faders, scene timelines, Art-Net/diagnostic meters, Monaco, Tiptap) and both minimum/maximum resize extremes pass exact box, containment, scroll-owner, minimum-target, and visibility assertions at 900px and 1280px"
    requirement: "UI-CONSIDERATIONS-BACKSTOP-GEOMETRY"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.geometry.spec.ts (20/20 tests across 6 families)"
        status: pass
    human_judgment: false
  - id: D2
    description: "An explicit >=2.0x-expanded copy fixture proves full-text visibility, multi-line growth, non-overlap, scroll ownership, focus reachability, and persistent safety across shell/dialog/shared-states/guided-first-show/status/field-help categories in light/dark at both required widths"
    requirement: "UI-CONSIDERATIONS-BACKSTOP-LONG-TEXT"
    verification:
      - kind: e2e
        ref: "frontend/e2e/design-system.expanded-copy.spec.ts (24/24 tests across 6 categories x 2 widths x 2 themes)"
        status: pass
    human_judgment: false
duration: ~1 session
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 31: Specialized-Geometry and Expanded-Copy Backstops Summary

**Built two independent Playwright backstops: a named 6-family specialized-geometry matrix (vertical faders, scene timelines, Art-Net/diagnostic meters, Monaco, Tiptap, resize extremes) proving Desk's and its sibling surfaces' domain-geometry carve-out holds under real edge-case sizing, and an explicit grapheme-counted >=2.0x expanded-copy fixture proving six representative UI copy categories wrap/reflow correctly in light/dark at 900x720 and 1280x720 -- along with a real accessibility gap found and fixed in TitleBar's truncated show-identity label.**

## Performance

- **Tasks:** 2/2 complete
- **Files created:** 5
- **Files modified:** 1

## Accomplishments

- `frontend/e2e/design-system.geometry.spec.ts`: a 6-family matrix -- Desk's vertical faders (universe-row height, per-channel fader-track width, fixed-domain clear-button geometry), Scenes & Looks' scene-list column and BarTimelinePanel's fixed 130px timeline row, ArtnetConfig's/Diagnostics' bounded 320px-max-height scroll panels, Scripts' real Monaco instance, Notes' real Tiptap instance, and Desk fader-width/universe-height plus Scenes & Looks scene-list-width driven to their exact `design-system/runtime-geometry.json` registered minimum/maximum. Every case reads the domain component's own set CSS custom property as the expected value (never a fabricated number) and records measurements/expectations/pass booleans into `evidence/specialized-geometry.json`.
- `frontend/e2e/fixtures/expandedCopy.ts` + `frontend/e2e/design-system.expanded-copy.spec.ts`: an explicit canonical/expanded copy-pair fixture (Unicode grapheme-counted, mechanically rejected below 2.0x via `assertPairsMeetMinimumExpansion()`) exercised across shell identity (TitleBar), dialog impact (Notes' destructive confirm), shared error states (Diagnostics' structural error), Guided First Show evidence (Assign stage), live status/log copy (Diagnostics' Application Log), and field-help validation copy (ArtnetConfig's real, user-typed-length-driven port validation message) -- in light/dark at both required widths, 24 cases total, recorded into `evidence/expanded-copy.json`.
- Found and fixed a real UI-SPEC Accessibility Contract gap in `TitleBar.tsx`: the truncated (ellipsis) show-identity label had no `title` attribute, so its complete un-truncated value had no accessible affordance at all once visually clipped.

## Task Commits

1. **Task 1: Backstop exact specialized geometry at 900 and 1280** - `1b9cd517` (test)
2. **Task 2: Backstop explicit 2x expanded-copy reflow** - `50bca50b` (test)

## Files Created/Modified

- `frontend/e2e/design-system.geometry.spec.ts` - 6-family specialized-geometry matrix, both widths, resize extremes
- `frontend/e2e/fixtures/expandedCopy.ts` - Canonical/expanded copy-pair fixture, grapheme-count ratio arithmetic
- `frontend/e2e/design-system.expanded-copy.spec.ts` - 6-category expanded-copy reflow backstop, both widths/themes
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/specialized-geometry.json` - Task 1 evidence
- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/expanded-copy.json` - Task 2 evidence
- `frontend/src/shell/TitleBar.tsx` - Added `title={projectName}` to the truncated show-identity label

## Decisions Made

- **index.css's global box-sizing:border-box reset is real** (confirmed empirically), contradicting a stale in-repo doc comment on `.faderValue` claiming no such reset exists -- geometry assertions against Desk's own set `--ds-universe-height`/`--ds-fader-width` account for this correctly.
- **Desk's fixtureScroll horizontal scroll-to-reveal is intended behavior, not an escape** -- the geometry spec's containment checks for elements inside it assert vertical containment plus "never escapes left," not a strict full-viewport-width bound, mirroring the same reasoning already applied to Desk's own vertically-scrolling canvas.
- **Monaco typing verification uses `page.keyboard.insertText`, not per-character `keyboard.type`** -- the latter occasionally raced Monaco's own async model updates and scrambled character order, a harness artifact unrelated to the product.
- **The guided-first-show expanded-copy pair uses two already-shipped real evidence strings** (not a mocked error) because every Guided First Show stage's own evidence template is a fixed, count-based string, and `wailsBridge.ts`'s `list*` wrappers structurally never let a rejection reach a stage's `ErrorState` -- a mocked-error injection route would have been both inauthentic and mechanically unreachable.
- **GuidedFirstShow's documented "safety valve" (evidenceAside collapsing toward 0 width below stageSection's 460px floor) is pre-existing and width-driven**, confirmed empirically against the identically-collapsing short canonical string -- the spec asserts the actually-guaranteed invariant (contentArea becomes the compensating scroll owner) in that state rather than assuming a wrap width the collapsed aside cannot offer.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added a `title` attribute to TitleBar's truncated show-identity label**
- **Found during:** Task 2, while proving the shell-identity long-text backstop
- **Issue:** `TitleBar.tsx`'s `.projectName` span truncates a long show name with CSS ellipsis (`overflow:hidden; text-overflow:ellipsis`) and is `pointer-events:none` (so window-drag/double-click-to-maximize still work underneath it), but rendered with no `title` attribute at all -- UI-SPEC's Accessibility Contract explicitly requires "Long names truncate only in bounded identity/readout locations and expose the full value through accessible text/title/tooltip," and this element had no such affordance once its content was visually clipped.
- **Fix:** Added `title={projectName}` to the span. The DOM text content itself was already the untruncated full string (assistive tech reading the accessible name was never affected), so this specifically restores the sighted-hover-tooltip path UI-SPEC requires.
- **Files modified:** `frontend/src/shell/TitleBar.tsx`
- **Verification:** `design-system.expanded-copy.spec.ts`'s shell category asserts the `title` attribute equals the full expanded name at both widths/themes; `node scripts/design-system/check.mjs --paths src/shell/TitleBar.tsx` remains clean; full `npx vitest run` (528/528) unaffected.
- **Committed in:** `50bca50b` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** A small, narrowly-scoped accessibility fix directly required to make this plan's own shell-identity backstop assertion true. No scope creep -- no other file in the shell chrome was touched.

## Known Stubs

None.

## Issues Encountered

- **Nested horizontal scroll containers are outside `helpers.ts`'s `findOverflowingControls`'s model.** The shared helper checks every button/input/select's own right edge against the window's viewport width; it has no concept of a control legitimately positioned past its own horizontally-scrolling ancestor's visible edge (Desk's per-universe `fixtureScroll`). At the registered `FADER_WIDTH_MAX` (96px) extreme, later fixtures' controls genuinely scroll past the window width, which the shared helper would report as false-positive overflow. Resolved with a spec-scoped `findDeskOverflowingControls` (this file only) that skips the individual right-edge check specifically for controls inside a genuinely-scrolled `fixtureScroll` ancestor, while keeping the document/appShell-level checks unchanged and unrelaxed.
- **`.faderInput`'s expected track width must be read from `--ds-fader-track-width` directly, not re-derived.** An initial attempt to compute the expected fader-input width from `--ds-fader-width` minus a fixed constant broke once a wide fader-width extreme (96px) crossed `DETAILED_MIN_FADER_WIDTH` and Desk.tsx's own `detailed` branch additionally reserved 28px for the value-scale ruler -- reading the row's own already-computed `--ds-fader-track-width` custom property directly (which Desk.tsx sets accounting for exactly this) is both simpler and correct regardless of which width preset/extreme is active.
- **`GuidedFirstShow`'s evidence-aside narrow-width "safety valve" reproduces identically for both the short and long copy strings at 900px**, confirming it is a pre-existing, width-driven, already-reviewed tradeoff (documented in `GuidedFirstShow.module.css`'s own comments) rather than something this plan's expanded-copy content newly triggered -- the spec accounts for this rather than treating it as a regression to fix.
- **`ErrorState.module.css` and `ConfirmModal.module.css` still reference pre-Phase-13 legacy tokens** (`--space-*`, `--ink`, `--panel`, `--text`, `--text2`, `--line`, `--radius-*`, hardcoded `"Archivo", system-ui` font stacks) that were removed from `index.css` by Plan 13-08 -- confirmed as the same expected mid-migration state `13-13-SUMMARY.md`/`.continue-here.md` already document for other shell-adjacent files. This does not affect this plan's own functional assertions (the load-bearing CSS -- `overflow-wrap`, block-level wrapping, bounded `max-width`/`overflow` -- is unaffected by an invalid custom property falling back to its inherited/initial value), so it was not touched; flagging here for whichever still-pending Wave 7 migration plan owns these two primitives.

## Next Phase Readiness

Both backstops are independently executable and evidence-producing, matching Plan 13-30's established conventions (deterministic fixtures, per-case JSON evidence with build SHA and environment metadata). `findDeskOverflowingControls`, `measureTextWrap`/`measureTextWrapByContent`, and the `expandedCopy.ts` canonical/expanded-pair-plus-ratio-guard pattern are reusable by any later Wave 9-13 visual spec needing the same primitives. No blockers for Plans 13-32 through 13-34/13-41.

## Self-Check: PASSED

- Commits `1b9cd517` and `50bca50b` exist in `git log` and together contain all declared/created files.
- All files listed above exist on disk at their declared paths.
- Both of the plan's own declared verify commands pass against the final committed state: `cd frontend && npx playwright test e2e/design-system.geometry.spec.ts --project=chromium --workers=1` (20/20 pass) and `cd frontend && npx playwright test e2e/design-system.expanded-copy.spec.ts --project=chromium --workers=1` (24/24 pass).
- `npx tsc --noEmit` clean.
- `npx vitest run` -- 528/528 pass (no regression from the TitleBar title-attribute change).
- `npm run build` (`tsc --noEmit && vitest run && vite build`) -- clean.
- Evidence JSON files reverted to their as-committed content after final confirmation re-runs (they regenerate a fresh timestamp/build SHA on every execution, matching Plan 13-17/13-30's precedent).

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
