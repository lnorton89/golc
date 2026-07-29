---
phase: quick-desktop-views-master-detail
plan: 260729-hsx
type: execute
wave: 1
depends_on: []
files_modified:
  - site/src/app/docs/desktop-views/page.tsx
  - site/src/components/docs/DesktopViewExplorer.tsx
  - site/tests/visual.spec.ts
  - site/tests/visual.spec.ts-snapshots/desktop-views-light-chromium-linux.png
  - site/tests/visual.spec.ts-snapshots/desktop-views-dark-chromium-linux.png
autonomous: true
requirements: [FDUI-01, FDUI-02, FDUI-03]
must_haves:
  truths:
    - "A reader sees one cohesive desktop-views section with a compact grouped view selector and one large selected workspace as the focal content."
    - "Selecting any current view updates the screenshot, purpose, principal actions, concepts, and operating notes from the generated catalog without maintaining a second inventory."
    - "Keyboard users can move through the selector, understand which view is selected, open and close the screenshot lightbox, and return focus to the exact trigger that opened it."
    - "The lightbox closes by Escape, its close control, or the backdrop; it keeps inside clicks open, locks background scrolling while open, and restores the previous scroll behavior on close."
    - "The master-detail section stacks into a readable selector-above-detail layout on mobile without horizontal overflow."
  artifacts:
    - path: site/src/components/docs/DesktopViewExplorer.tsx
      provides: "Catalog-driven master-detail selector, selected-view detail panel, and accessible screenshot lightbox."
    - path: site/src/app/docs/desktop-views/page.tsx
      provides: "Responsive route framing sized for the master-detail documentation section."
    - path: site/tests/visual.spec.ts
      provides: "Playwright interaction, accessibility-state, lightbox, responsive, and visual regression coverage."
  key_links:
    - from: site/src/app/docs/desktop-views/page.tsx
      to: site/src/content/desktop-views.json
      via: "The server page passes every generated group and view to the explorer unchanged."
      pattern: "desktopViews\\.groups"
    - from: site/src/components/docs/DesktopViewExplorer.tsx
      to: "selected view detail"
      via: "A stable selected view ID resolves against the flattened generated groups and drives one detail panel."
      pattern: "selected"
    - from: site/tests/visual.spec.ts
      to: site/src/components/docs/DesktopViewExplorer.tsx
      via: "Role- and accessible-name-based Playwright assertions exercise selection, keyboard navigation, the modal, and mobile stacking."
      pattern: "desktop views"
---

<objective>
Redesign the desktop views documentation as a single master-detail experience: a compact grouped view list on the left and one screenshot-led selected view on the right, with an accessible full-size screenshot lightbox.

Purpose: The current twelve-card grid gives every destination equal visual weight but makes comparison and focused reading cumbersome. A catalog-driven master-detail layout makes the selected workspace the clear focal point while preserving the complete generated inventory and documentation.
Output: A responsive master-detail explorer, accessible screenshot lightbox, updated interaction/mobile tests, intentional light/dark visual baselines, and site-first commit history followed by the parent gitlink commit.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260729-gq6-add-a-maintainable-desktop-views-documen/260729-gq6-SUMMARY.md
@site/AGENTS.md
@site/src/app/docs/desktop-views/page.tsx
@site/src/components/docs/DesktopViewExplorer.tsx
@site/tests/visual.spec.ts
@site/package.json

This quick task changes only the independently versioned `site/` submodule implementation and visual artifacts, followed by the parent repository's `site` gitlink and GSD artifacts. Do not edit the canonical desktop catalog, generated `site/src/content/desktop-views.json`, or screenshot assets: they remain the source of truth and all twelve current views/content must remain reachable through the redesigned explorer.

Before changing Next.js code, read the installed Next 16 App Router and Image guides under `site/node_modules/next/dist/docs/` as required by `site/AGENTS.md`. This is pattern verification inside the installed toolchain, not a research phase. Add no packages.

Recheck `git -C site status --short` and parent `git status --short` before staging or committing. Commit all site implementation, tests, and intentional snapshots inside `site/` first. Only after that commit exists may the parent repository record the new `site` gitlink together with this quick task's GSD artifacts. Use explicit pathspecs and do not stage unrelated work.

Locked implementation decisions:
- Replace the card grid and group filter with one cohesive master-detail section.
- Keep a grouped, compact list of every generated view on the left at desktop widths.
- Make one selected view on the right the focal point, led by a large screenshot followed by all existing detail fields.
- Open the screenshot in an accessible lightbox.
- Stack the selector above the selected detail on narrow viewports.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Build the catalog-driven master-detail explorer and accessible lightbox</name>
  <files>site/src/app/docs/desktop-views/page.tsx, site/src/components/docs/DesktopViewExplorer.tsx, site/tests/visual.spec.ts</files>
  <behavior>
    - Test 1: the page exposes all twelve generated views in their existing Show, Build, Operate, and Output groups, exactly one is selected, and activating another view replaces the single detail panel with that view's complete generated content.
    - Test 2: the selector communicates tab/selection state and supports ArrowUp/ArrowDown plus Home/End navigation across the complete ordered view list, with focus and selection staying synchronized.
    - Test 3: activating the selected screenshot opens a named modal with descriptive image alt text and a keyboard-reachable close control; Escape closes it and restores focus to the opening screenshot control.
    - Test 4: while the modal is open, document scrolling is locked; clicking the modal image/content does not close it; clicking the backdrop or close control does close it and restores the prior scroll state.
    - Test 5: at 390x844 the selector appears before the detail content, every selector option and the selected content remain usable, and neither the page nor the master-detail section overflows horizontally.
  </behavior>
  <action>
Write the Playwright behaviors above first, then replace the existing filter-and-card-grid implementation in `DesktopViewExplorer` with a single master-detail section. Flatten the supplied groups only for ordered selection/navigation; render the selector from the original groups so all catalog group labels and all current view labels remain visible. Initialize selection to the first generated view and keep selection keyed by stable view ID. At large viewports use a restrained fixed-width selector rail and a `minmax(0, 1fr)` focal column; at narrow viewports stack the grouped selector above the focal content. Update the route's max width/padding only as needed to let the 1440x900 image lead the right panel without crowding. Remove the old group filter and repeated card articles.

Use semantic tab selection: a named tablist containing grouped tab buttons, one selected tab via `aria-selected`, roving `tabIndex`, and one labelled tabpanel for the selected view. Preserve the catalog order across groups for ArrowUp/ArrowDown and Home/End controls, and make click/focus/selection behavior predictable. The detail panel must contain exactly one large screenshot trigger followed by the selected view's ID, title, purpose, principal actions, concepts when present, and operating notes when present. Keep the existing useful screenshot alt-text pattern and give the trigger an explicit accessible name that says it enlarges that workspace screenshot.

Implement the lightbox without a dependency. It must expose a named modal dialog, show the selected screenshot at its intrinsic aspect ratio, focus its close button on open, close on Escape, close from an explicit close button or a true backdrop click, ignore clicks inside the dialog content, and restore focus to the exact screenshot trigger after every close path. Save and restore the document body's prior overflow value so background scrolling is locked only while open, including component cleanup. Keep focus within the modal while it is open, honor reduced-motion preferences, use existing site color/focus tokens in both themes, and ensure controls have adequate target size and visible focus. Preserve meaningful alt text in both the focal image and lightbox.
  </action>
  <verify>
    <automated>npm --prefix site run lint &amp;&amp; npm --prefix site run typecheck &amp;&amp; npm --prefix site run test:visual -- --grep "desktop views"</automated>
  </verify>
  <done>The route renders one responsive catalog-driven master-detail section, every view and content field remains selectable, keyboard selection is semantic and complete, and every lightbox open/close path meets the focus, scroll-lock, backdrop, Escape, and alt-text contract.</done>
</task>

<task type="auto">
  <name>Task 2: Approve visual baselines and commit across the submodule boundary</name>
  <files>site/tests/visual.spec.ts, site/tests/visual.spec.ts-snapshots/desktop-views-light-chromium-linux.png, site/tests/visual.spec.ts-snapshots/desktop-views-dark-chromium-linux.png</files>
  <action>
Run the targeted desktop-views Playwright coverage at desktop and 390x844 mobile sizes and fix any role, keyboard, focus-restoration, scroll-lock, or overflow failure rather than weakening assertions. Update the existing Linux Chromium light and dark full-page baselines for the new default master-detail state, then inspect both images to confirm the selector is compact, the selected screenshot is the dominant element, details remain readable, both themes preserve focus/boundary contrast, and no generated view label or content is missing. Keep role/name-based interaction assertions alongside the snapshots so screenshots are not the sole acceptance signal.

Run the complete site quality suite. In `site/`, stage only the explorer, route, Playwright test, and intentional baseline files; inspect the staged diff and commit them. Return to the parent repository, verify that `site` points to that commit, then stage only the `site` gitlink and this quick task's GSD artifacts for the parent commit. The site commit must exist before the parent gitlink commit; do not amend the histories into the reverse order and do not include unrelated paths.
  </action>
  <verify>
    <automated>npm --prefix site run lint &amp;&amp; npm --prefix site run typecheck &amp;&amp; npm --prefix site run build &amp;&amp; npm --prefix site run test:links &amp;&amp; npm --prefix site run test:visual</automated>
    <human-check>Inspect `/docs/desktop-views` in light and dark themes at a desktop viewport and at 390x844. Confirm the grouped list is compact, the selected screenshot/details are the focal point, selection and focus are obvious, the lightbox is comfortable to use with mouse and keyboard, and mobile ordering and sizing remain readable.</human-check>
  </verify>
  <done>All site gates and focused accessibility interactions pass, the intentional light/dark baselines are visually approved, the site implementation is committed inside the submodule first, and the parent commit records only the resulting gitlink and GSD artifacts.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Generated catalog → client selection state | Repository-owned structured content supplies IDs, labels, copy, and image paths to interactive UI state. |
| Page → modal layer | Focus, keyboard input, pointer input, and document scrolling cross from the background page into the lightbox. |
| Site submodule → parent repository | The independent site commit must exist before the parent records its exact gitlink. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-hsx-01 | Tampering | `DesktopViewExplorer.tsx` catalog projection | medium | mitigate | Render groups/views directly from generated props, key selection by stable catalog ID, and assert all twelve options remain present so the redesign cannot silently replace the inventory. |
| T-hsx-02 | Denial of Service | lightbox focus and scroll lifecycle | medium | mitigate | Restore opener focus and prior body overflow for Escape, close-button, backdrop, and cleanup paths; focused Playwright tests cover each transition. |
| T-hsx-03 | Elevation of Privilege | modal keyboard/pointer handling | low | mitigate | Scope keyboard handling to the open modal, trap focus inside it, and distinguish true backdrop activation from clicks within dialog content. |
| T-hsx-04 | Repudiation | submodule and parent commits | low | mitigate | Inspect explicit staged pathsets, commit site content first, and record that exact site SHA in a later parent gitlink commit. |
| T-hsx-SC | Tampering | npm package execution | low | accept | No install or dependency change is planned; execution uses the site's committed lockfile and existing Next/Playwright toolchain. |
</threat_model>

<verification>
- The generated desktop-view catalog and screenshots are unchanged; the explorer still exposes all twelve current destinations and every existing content field.
- Targeted Playwright coverage proves semantic selection, ordered keyboard controls, modal naming, Escape, focus restoration, scroll locking/restoration, close/backdrop behavior, inside-click behavior, descriptive alt text, and mobile overflow safety.
- Updated light/dark baselines show a compact grouped selector beside one dominant screenshot-led detail panel.
- Site lint, typecheck, static build, link crawl, and complete visual suite pass.
- The site implementation commit precedes the parent commit that records the new `site` gitlink; unrelated files remain unstaged.
</verification>

<success_criteria>
- The desktop views guide is one cohesive master-detail section rather than a card grid.
- Every current generated view and its full documentation remains accessible through the compact grouped selector.
- The selected workspace screenshot and details are the clear desktop focal point.
- The screenshot lightbox is accessible by keyboard and pointer and leaves page focus/scroll state intact after close.
- Desktop, dark theme, and mobile behavior are covered by automated interaction and visual regression tests.
</success_criteria>

<source_coverage_audit>

| Source | Item | Status | Plan coverage |
|--------|------|--------|---------------|
| GOAL | Redesign desktop views documentation as one master-detail section | COVERED | Task 1 replaces the grid with a selector plus one focal detail panel; Task 2 approves visual output. |
| REQ | FDUI-01 Fixture Library front door remains documented | COVERED | Task 1 preserves every generated Build view and all catalog content. |
| REQ | FDUI-02 Shows front door remains documented | COVERED | Task 1 preserves every generated Show view and all catalog content. |
| REQ | FDUI-03 Guided First Show entry remains documented | COVERED | Task 1 preserves the generated Overview content without rewriting the catalog. |
| RESEARCH | No research phase for this quick redesign | EXCLUDED | Existing site, installed Next guides, and Playwright patterns provide sufficient Level 0/quick verification. |
| CONTEXT | Compact grouped list on the left | COVERED | Task 1 creates the grouped semantic selector rail. |
| CONTEXT | Large selected screenshot and details on the right | COVERED | Task 1 creates one screenshot-led tabpanel; Task 2 visually verifies focal hierarchy. |
| CONTEXT | Accessible screenshot lightbox | COVERED | Task 1 specifies modal semantics, keyboard/focus lifecycle, scroll locking, pointer close paths, and alt text. |
| CONTEXT | Preserve generated catalog and all current views/content | COVERED | Task 1 consumes generated props unchanged and tests the complete inventory; generated data/assets are explicitly out of the edit set. |
| CONTEXT | Mobile stacks list above focal content | COVERED | Tasks 1-2 implement and test the 390x844 stacked layout and overflow safety. |
| CONTEXT | Responsive and visual test updates | COVERED | Tasks 1-2 add interaction/mobile assertions and refresh inspected light/dark baselines. |
| CONTEXT | Site commit precedes parent gitlink commit | COVERED | Task 2 mandates explicit site-first and parent-second commits. |
| CONTEXT | No deferred ideas supplied | COVERED | The plan adds no scope beyond the locked redesign and its acceptance coverage. |

</source_coverage_audit>

<output>
Create `.planning/quick/260729-hsx-redesign-the-desktop-views-documentation/260729-hsx-SUMMARY.md` when done.
</output>
