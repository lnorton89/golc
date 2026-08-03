---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "34"
subsystem: design-system-visual-verification
tags: [playwright, screenshot-baseline, design-system, e2e, visual-regression, live-editors, midi, monaco, tiptap]
requires:
  - phase: 13-17
    provides: screenshot-tolerance.json (calibrated maxDiffPixelRatio 0), screenshot.css, waitForFonts, and the calibration seed/fixture pattern this plan's captures follow
  - phase: 13-32
    provides: design-system.visual-shell.spec.ts's withTheme/settleForCapture/mask-intersection conventions, mirrored verbatim by this plan's own spec file
  - phase: 13-33
    provides: design-system.visual-authoring.spec.ts's precedent of capturing a combined UI-SPEC matrix row as one bounded state, extended here into two small e2e-only composite fixtures
  - phase: 13-13
    provides: Desk.tsx's fully migrated projected-fader grid and its calibration seed (e2e/fixtures/designSystem.ts), reused verbatim for the Desk/Operator Surface capture
provides:
  - "frontend/e2e/design-system.visual-live-editors.spec.ts: the third canonical Playwright visual-regression spec, covering three more of UI-SPEC's Required reference matrix surfaces (Desk / Operator Surface, MIDI Mapping, Scripts / Notes)"
  - "12 accepted baseline PNGs (3 surfaces x 2 themes x 2 widths) under frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/"
  - "frontend/src/design-system/fixtures/DeskOperatorFixture.tsx and ScriptsNotesFixture.tsx: two new e2e-only ?e2e=... fixture routes composing real production components for UI-SPEC matrix rows that name two features living on separate, unrelated WorkspaceRouter destinations"
affects: [13-41]
tech-stack:
  added: []
  patterns:
    - "Reused design-system.visual-shell.spec.ts's/design-system.visual-authoring.spec.ts's exact withTheme/settleForCapture/mask-intersection-guard conventions rather than inventing a second pattern."
    - "A UI-SPEC 'Required reference matrix' row naming two features on two SEPARATE, unrelated WorkspaceRouter destinations (not the same workspace demonstrating both halves, as Plan 13-33's Fixture Library/Patch & Pools row was) is captured via a small, e2e-only composite fixture (mirroring the established DialogFeasibility.tsx/DesignSystemGallery.tsx/EmergencyFallbackFixture.tsx ?e2e=... seam in App.tsx) that mounts BOTH real production components side by side -- never a re-implementation of either."
    - "Stack combined-row composite fixtures vertically (full page width each), not side by side: a two-column split squeezes each real component below its own real minimum content width at the 900px compact-width regression viewport, overflowing the document horizontally or visually clipping narrow fields -- a symptom unique to this plan's own synthetic composite, not a real product bug."
    - "window.runtime.EventsOn (a no-op stub in installHealthyBindings) can be overridden with a real pub/sub plus a window.__emit escape hatch so a Playwright test can push a live 'midi:feedback'-shaped event the exact same way the Go host would, rather than approximating live feedback through static seed data alone."
    - "A button briefly disabled while its own click handler's async call is in flight is forcibly blurred by the browser -- asserting focus returns to <body> (a safe, neutral outcome) afterward, not that the button itself retains focus, mirrors Plan 13-33's identical FixturePatch.tsx reasoning."
key-files:
  created:
    - frontend/e2e/design-system.visual-live-editors.spec.ts
    - frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/desk-operator-{light,dark}-{900,1280}-win32.png
    - frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/midi-mapping-{light,dark}-{900,1280}-win32.png
    - frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/scripts-notes-{light,dark}-{900,1280}-win32.png
    - frontend/src/design-system/fixtures/DeskOperatorFixture.tsx
    - frontend/src/design-system/fixtures/ScriptsNotesFixture.tsx
  modified:
    - frontend/src/App.tsx
key-decisions:
  - "Added two new e2e-only fixtures/routes (DeskOperatorFixture.tsx at ?e2e=desk-operator, ScriptsNotesFixture.tsx at ?e2e=scripts-notes) beyond the plan's own declared files_modified list, because 'Desk / Operator Surface' and 'Scripts / Notes' are each a single UI-SPEC matrix row naming two features that live on two separate, unrelated WorkspaceRouter destinations -- OperatorSurfaceWorkspace/DeskWorkspace and ScriptsWorkspace/NotesWorkspace respectively -- with no single existing navigable screen demonstrating both halves at once. This mirrors the plan's own precedent set by Plan 13-33 (capturing a combined row as one bounded state) and the repo's own established ?e2e=... fixture-route mechanism (DialogFeasibility.tsx, DesignSystemGallery.tsx, EmergencyFallbackFixture.tsx) rather than inventing a second mechanism or fabricating a fake composite UI. Both fixtures mount only real, unmodified production components (Launcher/Desk/SafetyCluster; ScriptsWorkspace/NotesWorkspace) -- documented as a Rule 2 deviation below."
  - "Stacked both composite fixtures' two halves vertically (each at full page width) rather than side by side, after discovering the side-by-side layout squeezed each real component below its own realistic minimum content width at 900px -- Desk's 'Release All' button overflowed the viewport horizontally, and NotesWorkspace's Note title field visually clipped its own value at ~450px. Vertical stacking gives each real production component the identical full page width it would render at through its own real navigable destination."
  - "Placed SafetyCluster first (immediately below the heading) in DeskOperatorFixture.tsx, not after Launcher+Desk: the combined content is taller than the 720px regression viewport, and toHaveScreenshot only captures the current viewport (not the full page) -- a safety cluster placed after the tall content would scroll out of the captured baseline. Mirrors GlobalFrame's own real persistent-header placement (top of screen, always reachable)."
  - "Overrode window.runtime.EventsOn with a real pub/sub (window.__emit escape hatch) for the MIDI Mapping capture, rather than relying on static feedback seeded only at page load, so the test could push a genuine 'midi:feedback' event mid-test the same way onMidiFeedback (wailsBridge.ts) expects the Go host to -- this is what lets one accepted baseline demonstrate BOTH the pickup-feedback state (physical=62%, target=35%) and the conflict/error state (Learn clicked against a mocked conflict response) together."
  - "Adjusted the initially-planned 'Validate button retains focus' assertion (Scripts/Notes) after a real test failure revealed the button is briefly `disabled` while its own async call is in flight, which browsers forcibly blur -- replaced with the same 'focus lands somewhere safe and visible' check already used for the MIDI Mapping conflict scenario, matching Plan 13-33's identical FixturePatch.tsx precedent rather than asserting an incorrect assumption."
requirements-completed: [D-01, D-02, D-03, D-06, D-10, D-12, D-13, D-14, UI-SPEC-VISUAL-MATRIX]
coverage:
  - id: D1
    description: "Desk / Operator Surface baseline matrix: 4 canonical screenshots (light/dark x 900/1280) of DeskOperatorFixture.tsx showing an active LIVE scene pad, a locked scene pad, an assigned Grand Master, Desk's calibrated two-instance projected-fader grid, and the persistent SafetyCluster, preceded by semantic/safety/containment/focus assertions."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep \"Desk / Operator Surface\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "MIDI Mapping baseline matrix: 4 canonical screenshots of one control_change mapping's live pickup feedback (physical=62%, target=35%, not armed) alongside a second control's conflict/error state, with pickup arithmetic, direction copy, target geometry, conflict semantics, and no support claim asserted first."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep \"MIDI Mapping\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Scripts / Notes baseline matrix: 4 canonical screenshots of ScriptsNotesFixture.tsx showing a real Monaco instance (with a genuine 2-error Validate diagnostic blocking Run/Debug) directly adjacent to a real Tiptap instance (with a genuine completed autosave), each workspace's own public toolbar, model independence, and no theme branch asserted first."
    requirement: "UI-SPEC-VISUAL-MATRIX"
    verification:
      - kind: e2e
        ref: "cd frontend && npx playwright test e2e/design-system.visual-live-editors.spec.ts --grep \"Scripts / Notes\" --project=chromium --workers=1"
        status: pass
    human_judgment: false
duration: "~1 session"
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 34: Bounded Live/Editor Visual Matrix Summary

**Accepted the third canonical Playwright baseline set -- 12 PNGs (Desk / Operator Surface, MIDI Mapping, Scripts / Notes, each at 2 themes x 2 widths) -- adding two small, e2e-only composite fixtures for the two UI-SPEC matrix rows that name features living on separate, unrelated real destinations, and a real live-feedback pub/sub override for MIDI's pickup/conflict state.**

## Performance

- **Duration:** ~1 session
- **Tasks:** 3/3 complete
- **Files modified:** 8 (1 spec file, 12 baseline PNGs, 2 new fixture files, 1 App.tsx route-wiring edit)

## Accomplishments

- Added `frontend/e2e/design-system.visual-live-editors.spec.ts`, mirroring Plan 13-32/13-33's exact `withTheme`/`settleForCapture`/mask-intersection-guard conventions.
- **Desk / Operator Surface** (Task 1): built `DeskOperatorFixture.tsx` (new `?e2e=desk-operator` route) mounting the real operate-mode `Launcher` (LIVE/locked pads, masters) directly above the real `Desk` projected-fader grid, with `SafetyCluster` visible -- no single existing destination shows both. Seeded one active, assigned scene ("Opening Look") for the LIVE pad, one unassigned scene ("Interlude") for the locked pad, one assigned Grand Master, and reused Desk's own calibrated two-instance/four-channel seed (Plan 13-17/13-13) for the projected faders. Asserted LIVE/locked semantics, masters, fader presence, safety, containment, and focus before each capture.
- **MIDI Mapping** (Task 2): captured via real navigation (no new fixture needed). Overrode `window.runtime.EventsOn` with a genuine pub/sub (`window.__emit` escape hatch) so the test could push a real `"midi:feedback"` event mid-test, producing a live pickup state (physical=62%, target=35%, not armed) for one mapping, alongside a mocked `StartLearn` conflict response for a second control -- both render in the same accepted baseline. Asserted pickup arithmetic (`MidiPickup`'s exact rounded text), direction copy (the ghost marker's own title), target geometry (inline style + bounding-box containment), conflict semantics, no support claim, safety, containment, and focus.
- **Scripts / Notes** (Task 3): built `ScriptsNotesFixture.tsx` (new `?e2e=scripts-notes` route) mounting the real `ScriptsWorkspace` (a real Monaco instance plus its own Save/Delete/Validate/Run/Debug/Stop toolbar) directly above the real `NotesWorkspace` (a real Tiptap instance plus its own autosave status) -- no single existing destination shows both. Drove a real `Validate` click against a mocked 2-diagnostic response (blocking Run/Debug) and a real Notes title edit through the actual `AUTOSAVE_DEBOUNCE_MS` cycle to a genuine "Saved" status. Asserted editor mount, model independence, diagnostics/save truth, public toolbars/actions, no theme branch, focus, and containment before each capture.
- Confirmed determinism: ran the full 12-test suite 3+ separate consecutive times with identical pass results, plus each task's own individually declared verify command against the final committed state.

## Task Commits

Each task was committed atomically:

1. **Task 1: Capture Desk and Operator Surface matrix** - `4b810580` (feat)
2. **Task 2: Capture MIDI Mapping matrix** - `a80f97f3` (feat)
3. **Task 3: Capture Scripts and Notes matrix** - `6064f365` (feat)

_Note: these tasks are `tdd="true"` calibration/acceptance work (the spec file itself is the deliverable, not a test proving separate production code), matching Plan 13-17/13-32/13-33's own precedent of one commit per task rather than a RED/GREEN split. The spec file grew incrementally across the three commits (Task 1's commit contains only its own describe block plus shared helpers; Task 2's commit appends its block; Task 3's commit appends the final block), mirroring 13-33's own git history shape exactly._

## Files Created/Modified

- `frontend/e2e/design-system.visual-live-editors.spec.ts` - the three-surface live/editor matrix spec
- `frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/desk-operator-{light,dark}-{900,1280}-win32.png` - 4 accepted Desk/Operator Surface baselines
- `frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/midi-mapping-{light,dark}-{900,1280}-win32.png` - 4 accepted MIDI Mapping baselines
- `frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/scripts-notes-{light,dark}-{900,1280}-win32.png` - 4 accepted Scripts/Notes baselines
- `frontend/src/design-system/fixtures/DeskOperatorFixture.tsx` - new e2e-only composite fixture (`?e2e=desk-operator`) mounting Launcher + Desk + SafetyCluster
- `frontend/src/design-system/fixtures/ScriptsNotesFixture.tsx` - new e2e-only composite fixture (`?e2e=scripts-notes`) mounting ScriptsWorkspace + NotesWorkspace
- `frontend/src/App.tsx` - wires the two new `?e2e=...` routes, mirroring the three existing fixture seams exactly

## Decisions Made

See frontmatter `key-decisions`. In short: (1) added two small, e2e-only composite fixtures beyond the plan's declared files, because "Desk / Operator Surface" and "Scripts / Notes" each name two features on two separate real destinations with no existing combined screen -- mirrors Plan 13-33's own combined-row precedent and the repo's established `?e2e=...` seam; (2) stacked both composites' halves vertically (not side by side) after a real 900px-width overflow/clipping bug in the side-by-side layout; (3) placed SafetyCluster first in the Desk/Operator fixture so it stays within the captured viewport; (4) overrode `window.runtime.EventsOn` with a real pub/sub for genuine live MIDI feedback; (5) corrected an initially-wrong focus assertion after discovering the browser forcibly blurs a button that becomes briefly `disabled` mid-click.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Added two new e2e-only composite fixtures and their App.tsx routes**

- **Found during:** Task 1 and Task 3, while determining how to satisfy UI-SPEC's "Desk / Operator Surface" and "Scripts / Notes" Required reference matrix rows.
- **Issue:** Both rows each name two features (Launcher/Desk; Monaco-editing/Tiptap-editing) that live on two separate, unrelated `WorkspaceRouter` destinations (`OperatorSurfaceWorkspace`/`DeskWorkspace`; `ScriptsWorkspace`/`NotesWorkspace`). No single existing navigable screen renders both halves of either row at once, so the plan's own declared `<files>` (only the spec file plus 4 PNGs per task) could not be satisfied by real navigation alone.
- **Fix:** Added `frontend/src/design-system/fixtures/DeskOperatorFixture.tsx` and `ScriptsNotesFixture.tsx`, each a small e2e-only composite that mounts the REAL, unmodified production components side by side (later corrected to stacked -- see deviation 2), wired via two new `?e2e=...` routes in `App.tsx` mirroring the three already-established fixture seams (`DialogFeasibility.tsx`, `DesignSystemGallery.tsx`, `EmergencyFallbackFixture.tsx`) rather than inventing a second routing mechanism or fabricating fake composite markup.
- **Files modified:** `frontend/src/design-system/fixtures/DeskOperatorFixture.tsx`, `frontend/src/design-system/fixtures/ScriptsNotesFixture.tsx`, `frontend/src/App.tsx`.
- **Verification:** `node scripts/design-system/check.mjs --paths src/App.tsx,src/design-system/fixtures/DeskOperatorFixture.tsx,src/design-system/fixtures/ScriptsNotesFixture.tsx` exits 0 (zero diagnostics); `npx tsc --noEmit` clean; both fixtures render correctly in all 8 of their own captures.
- **Committed in:** `4b810580` (Task 1, DeskOperatorFixture.tsx + its App.tsx route), `6064f365` (Task 3, ScriptsNotesFixture.tsx + its App.tsx route).

**2. [Rule 1 - Bug] Fixed a real 900px-width overflow/clipping bug in both composite fixtures' initial side-by-side layout**

- **Found during:** Task 1, while reviewing the first `desk-operator-*-900-*` captures; recurred identically in Task 3's `scripts-notes-*-900-*` captures.
- **Issue:** Placing each fixture's two halves in a two-column flex row squeezed each real component to roughly half of 900px (~450px) -- narrower than any real navigable destination ever renders at. Desk's own "Release All" button overflowed the viewport horizontally (`findOverflowingControls` caught this mechanically); NotesWorkspace's Note title field visually clipped its own "Load-in Checklist" value at the cramped width (not caught by `findOverflowingControls`, since the underlying DOM value was still correct -- `toHaveValue` passed -- but the rendered pixels were unrealistic).
- **Fix:** Restructured both fixtures to stack their two halves vertically instead, giving each real production component its own full page width -- identical to the width it would render at through its own real destination -- while still satisfying UI-SPEC's "adjacent" requirement (vertically adjacent, not necessarily side by side).
- **Files modified:** `frontend/src/design-system/fixtures/DeskOperatorFixture.tsx`, `frontend/src/design-system/fixtures/ScriptsNotesFixture.tsx`.
- **Verification:** Re-ran all 8 affected captures (Desk/Operator + Scripts/Notes, both widths/themes) -- `findOverflowingControls` returns `[]`, and visual review of the regenerated PNGs shows clean, unclipped rendering at both 900px and 1280px.
- **Committed in:** `4b810580` (Desk/Operator fix), `6064f365` (Scripts/Notes fix) -- both fixed before their respective task's baselines were ever accepted, so no stale/incorrect PNG was ever committed.

**3. [Rule 1 - Bug] Corrected an incorrect "button retains focus" assertion after a real test failure**

- **Found during:** Task 3, first run of the Scripts/Notes describe block.
- **Issue:** The initial assertion expected the `Validate` button to remain focused after its own click resolved. In reality, `ScriptsWorkspace.tsx`'s `handleValidate` briefly sets the button's own `disabled` attribute true while the mocked `ValidateScript` call is in flight (`disabled={!selectedName || validating}`) -- browsers forcibly blur an element the instant it becomes `disabled`, so focus reliably lands on `<body>` once the button re-enables, not back on the button itself.
- **Fix:** Replaced the assertion with the same "focus lands somewhere safe and visible" check already used for the MIDI Mapping conflict scenario (and matching Plan 13-33's own documented `FixturePatch.tsx` precedent for an analogous re-render-driven focus loss).
- **Files modified:** `frontend/e2e/design-system.visual-live-editors.spec.ts` (test-only).
- **Verification:** All 4 Scripts/Notes tests pass with the corrected assertion; re-confirmed across 3 consecutive full-suite runs.
- **Committed in:** `6064f365` (Task 3 commit).

---

**Total deviations:** 3 auto-fixed (1 missing critical functionality, 2 bugs)
**Impact on plan:** Deviation 1 was necessary to make this plan's own two combined UI-SPEC matrix rows capturable at all, using the repo's own established, precedented mechanism rather than a novel one. Deviations 2 and 3 were both necessary corrections found via real, reproducing test/visual failures during this plan's own execution, not speculative changes. None changes any existing component's public API or behavior; the design-system checker reports zero violations against all touched files.

## Known Stubs

None.

## Issues Encountered

- `node_modules` was missing in this worktree at session start (known Windows-junction issue, documented in prior plans' summaries) -- resolved with a plain `npm install` inside this worktree's own `frontend/` directory.
- Extensive layout iteration was required to fit each composite fixture's full required content (safety controls, LIVE/locked pads, masters, and projected faders for Desk/Operator; both editors' full toolbars/content for Scripts/Notes) inside the 720px regression viewport height, since `toHaveScreenshot` only captures the current viewport, not the full page -- resolved via a compact heading, `density="compact"` on one Panel, and reordering `SafetyCluster` to the top of `DeskOperatorFixture.tsx`. See Deviations above for the two real bugs this process also surfaced.

## Next Phase Readiness

`frontend/e2e/design-system.visual-live-editors.spec.ts` and its 12 accepted baselines are ready as ground truth for any future re-run. `DeskOperatorFixture.tsx`/`ScriptsNotesFixture.tsx` and their `?e2e=...` routes are available as a reusable pattern for any future plan needing to demonstrate a UI-SPEC matrix row spanning two otherwise-unrelated real destinations. The `window.runtime.EventsOn` pub/sub override pattern (this spec's `installMidiMappingBindings`) is available to any future spec needing to push a genuine live Wails event mid-test rather than only static page-load seed data.

## Self-Check: PASSED

- All 6 declared new/modified files (`design-system.visual-live-editors.spec.ts`, 12 baseline PNGs, `DeskOperatorFixture.tsx`, `ScriptsNotesFixture.tsx`, `App.tsx`) confirmed tracked via `git status --short` (clean tree) and `git log`.
- All three task commit hashes (`4b810580`, `a80f97f3`, `6064f365`) confirmed present via `git log --oneline -5`.
- All three of the plan's own declared verify commands re-confirmed passing against the final committed state (`--grep "Desk / Operator Surface"`, `--grep "MIDI Mapping"`, `--grep "Scripts / Notes"`, each 4/4).
- `npx tsc --noEmit` clean; `npx vitest run` 528/528 pass; `node scripts/design-system/check.mjs --paths src/App.tsx,src/design-system/fixtures/DeskOperatorFixture.tsx,src/design-system/fixtures/ScriptsNotesFixture.tsx` exits 0 (no violations).
- Full 12-test suite re-confirmed passing 3+ consecutive times on the final committed state; exactly 12 PNG files present under `frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/`, matching the plan's declared filenames exactly.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
