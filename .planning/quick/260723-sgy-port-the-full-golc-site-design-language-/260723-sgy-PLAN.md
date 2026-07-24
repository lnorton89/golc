---
phase: quick-full-golc-site-design-language
plan: 260723-sgy
type: execute
wave: 1
depends_on: []
files_modified:
  - .planning/sketches/themes/default.css
  - .planning/sketches/themes/fonts/Archivo-700.ttf
  - .planning/sketches/themes/fonts/Archivo-800.ttf
  - .planning/sketches/themes/fonts/JetBrainsMono-500.ttf
  - .planning/sketches/001-workspace-shell/index.html
  - .planning/sketches/002-programming-workspace/index.html
  - .planning/sketches/003-performance-workspace/index.html
  - .planning/sketches/004-patch-to-play-flow/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/themes/default.css
  - .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/Archivo-700.ttf
  - .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/Archivo-800.ttf
  - .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/JetBrainsMono-500.ttf
  - .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html
  - .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html
autonomous: true
requirements: [PLAY-01, PLAY-03, PLAY-07, PLAY-10, PLAY-11, PLAY-12]
must_haves:
  truths:
    - "All four canonical sketches render the full GOLC site design language at console density: real bundled fonts, canonical beam mark and outline icons, site-derived typography hierarchy, casing, radii, surfaces, controls, state chips, focus treatment, and selective motion."
    - "The approved information architecture, workflow content, interaction scripts, and variant order remain intact, with exactly these initial winners: 001 D, 002 A, 003 A, and 004 B."
    - "At desktop width each winner retains its fixed operational frame, and at compact desktop width every inspector or side-panel function remains reachable through a visible drawer, accordion, or explicit toggle while the safety cluster remains available."
    - "Canonical and packaged theme, font, and HTML assets are byte-for-byte mirrors, and no unrelated dirty-worktree content is changed."
  artifacts:
    - path: .planning/sketches/themes/default.css
      provides: "Offline font faces and shared Paper/Ink console design primitives"
      contains: "@font-face"
    - path: .planning/sketches/themes/fonts/Archivo-700.ttf
      provides: "Bundled Archivo bold face from the sibling site"
    - path: .planning/sketches/themes/fonts/Archivo-800.ttf
      provides: "Bundled Archivo extra-bold face from the sibling site"
    - path: .planning/sketches/themes/fonts/JetBrainsMono-500.ttf
      provides: "Bundled JetBrains Mono metadata face from the sibling site"
    - path: .planning/sketches/001-workspace-shell/index.html
      provides: "Focused Command Rail shell, initially variant D"
    - path: .planning/sketches/002-programming-workspace/index.html
      provides: "Scene Stack + Inspector programming workspace, initially variant A"
    - path: .planning/sketches/003-performance-workspace/index.html
      provides: "Launcher + Masters performance workspace, initially variant A"
    - path: .planning/sketches/004-patch-to-play-flow/index.html
      provides: "Guided First Show workflow, initially variant B"
    - path: .kimi-code/skills/sketch-findings-golc/sources/themes/default.css
      provides: "Packaged byte mirror of the canonical shared theme"
  key_links:
    - from: C:/Users/Lawrence/Documents/Dev/golc-site/src/app/fonts
      to: .planning/sketches/themes/fonts
      via: "Exact local font binaries referenced by relative @font-face URLs"
      pattern: "Archivo-(700|800)\\.ttf|JetBrainsMono-500\\.ttf"
    - from: C:/Users/Lawrence/Documents/Dev/golc-site/src/components/GolcMark.tsx
      to: .planning/sketches/001-workspace-shell/index.html
      via: "Equivalent inline SVG beam geometry in persistent top-bar marks"
      pattern: "50,25|73\\.5,84|aria-label=\"GOLC mark\""
    - from: C:/Users/Lawrence/Documents/Dev/golc-site/src/components/icons.tsx
      to: .planning/sketches/001-workspace-shell/index.html
      via: "Inline 24x24 square-ended 1.75px outline icon vocabulary"
      pattern: "viewBox=\"0 0 24 24\"|stroke-width=\"1\\.75\""
    - from: .planning/sketches/themes/default.css
      to: .planning/sketches/002-programming-workspace/index.html
      via: "Shared panel/card/control/chip/focus/motion primitives consumed by sketch-specific CSS"
      pattern: "radius-panel|radius-card|radius-control|focus-visible"
    - from: .planning/sketches
      to: .kimi-code/skills/sketch-findings-golc/sources
      via: "Exact same-relative-path byte mirroring after canonical work is complete"
      pattern: "themes/default\\.css|index\\.html|themes/fonts"
---

<objective>
Port the full sibling `golc-site` design language into all four approved GOLC sketch assets, correcting the earlier palette-only pass while preserving the sketches as behavioral design evidence.

Purpose: Make the canonical and packaged sketches express the validated Paper/Ink component grammar—not merely its colors—without changing their information architecture, flows, scripts, variant order, or operator-safety contract.
Output: A shared offline theme/font foundation, four fully restyled canonical sketch files with initial winners 001 D / 002 A / 003 A / 004 B, and exact packaged mirrors.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/260723-sgy-RESEARCH.md
@.kimi-code/skills/sketch-findings-golc/SKILL.md
@.planning/sketches/MANIFEST.md
@.planning/sketches/UI-RESEARCH.md
@.planning/sketches/WORKFLOW-MAP.md
@.planning/sketches/WRAP-UP-SUMMARY.md
@.planning/sketches/themes/default.css
@.planning/sketches/001-workspace-shell/index.html
@.planning/sketches/002-programming-workspace/index.html
@.planning/sketches/003-performance-workspace/index.html
@.planning/sketches/004-patch-to-play-flow/index.html
@C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css
@C:/Users/Lawrence/Documents/Dev/golc-site/src/app/layout.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/GolcMark.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/icons.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/SiteHeader.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/StatusChip.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/MobileMenu.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/docs/ViewExplorer.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/roadmap/PhaseTimeline.tsx
@C:/Users/Lawrence/Documents/Dev/golc-site/src/components/architecture/RepoTree.tsx
@site/package.json
@site/playwright.config.ts

**Locked scope:** Preserve every variant and its order, all workflow copy and controls, all data attributes, the fixed live frame, persistent safety controls, and the existing interaction scripts. Initial active tab/section pairs must be corrected to 001 D, 002 A, 003 A, and 004 B. Do not rewrite these console layouts into marketing-site sections, add the site header/footer, add phone-first IA, or touch application source.

**Dirty-worktree constraint:** This repository is shared and already contains unrelated modified/untracked work. Before each task inspect `git status --short` and `git diff --` for every owned path. Layer work onto any owned-file changes that appeared after planning; never revert, replace, stage, or reformat unrelated files. Stop only if a concurrent edit to an owned path cannot be preserved safely.

**Discovery:** Level 1 local verification is complete in `260723-sgy-RESEARCH.md`. No dependency install is authorized or required. The sibling site and its checked-in font binaries are the source authority.

**Quick-workflow scope:** This must remain one PLAN file with at most three tasks. The bounded ownership is: Task 1 owns the shared theme/fonts plus canonical 001/002; Task 2 owns canonical 003/004; Task 3 owns all packaged mirrors and temporary validation/browser artifacts. Each task commits only its owned repository paths. If final browser evidence requires a canonical correction, make a focused fix commit for that canonical owner before re-mirroring; never fold unrelated dirty files into a task commit.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Build the shared foundation and port canonical 001/002</name>
  <files>.planning/sketches/themes/default.css, .planning/sketches/themes/fonts/Archivo-700.ttf, .planning/sketches/themes/fonts/Archivo-800.ttf, .planning/sketches/themes/fonts/JetBrainsMono-500.ttf, .planning/sketches/001-workspace-shell/index.html, .planning/sketches/002-programming-workspace/index.html</files>
  <read_first>
    - `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/260723-sgy-RESEARCH.md` — implementation-ready transfer rules, mismatch audit, per-winner requirements, and validation architecture.
    - `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css` and `layout.tsx` — authoritative Paper/Ink tokens, font roles, motion, and surface behavior.
    - `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/fonts/` — exact three checked-in font binaries to package offline.
    - `C:/Users/Lawrence/Documents/Dev/golc-site/src/components/GolcMark.tsx` and `icons.tsx` — exact beam mark and 24x24 outline-icon geometry.
    - `C:/Users/Lawrence/Documents/Dev/golc-site/src/components/{SiteHeader,StatusChip,MobileMenu}.tsx` — lockup, semantic state, control, and responsive-overlay grammar.
    - The canonical theme and all four HTML files — current public token names and the complete baseline inventory; this task edits only theme/fonts/001/002.
  </read_first>
  <action>Create `%TEMP%\golc-260723-sgy\baseline` and `%TEMP%\golc-260723-sgy\screenshots`; remove only a prior `%TEMP%\golc-260723-sgy` after resolving and confirming it is under `%TEMP%`. Copy the four pristine canonical sources to these exact unique baseline names and record the direct mapping in `%TEMP%\golc-260723-sgy\baseline-map.json`: `001-workspace-shell.html` -> `.planning/sketches/001-workspace-shell/index.html`, `002-programming-workspace.html` -> `.planning/sketches/002-programming-workspace/index.html`, `003-performance-workspace.html` -> `.planning/sketches/003-performance-workspace/index.html`, and `004-patch-to-play-flow.html` -> `.planning/sketches/004-patch-to-play-flow/index.html`.

Create `%TEMP%\golc-260723-sgy\structure-validator.cjs` before editing. It must use Node standard-library APIs only and expose baseline/final signature and inventory functions. For each unique mapping, capture ordered variant IDs; normalized visible copy; ordered IDs; every ordered `data-*` attribute name/value; and every ordered `button`, `input`, `select`, and `option` with tag, type, name, value, disabled/readonly state, accessible name, and normalized text. Capture the complete script block and inventories of hex, rgb/rgba, gradients, box/text shadows, SVG paint, `.mark` text, icon-well text, and non-ASCII code points. Final comparison must normalize only these explicit differences: canonical mark/icon nodes tagged `data-sgy-visual`; compact disclosure markup tagged `data-sgy-disclosure`; a disclosure handler block bounded by `/* sgy:compact-detail:start */` and `/* sgy:compact-detail:end */`; and `active` class movement limited to tab/section pairs 001 D, 002 A, 003 A, 004 B. It must explicitly map allowed old icon literals by selector/accessible label rather than deleting glyphs globally. Everything else—copy, IDs, original `data-*`, controls/types/names/values/order, variant order, and original script—must compare exactly.

Copy the sibling site's exact three font binaries into the canonical theme font directory and hash-check them. Expand `default.css` into the complete offline shared grammar from the research: local font faces; Paper/Ink semantic surfaces; 12/8/6/pill radii; 4/8/12/16/24 spacing; title/body/metadata roles; 1px boundaries and 2px semantic rules; flat panels; limited transient/browse elevation; buttons, fields, chips, focus-visible, property-scoped 120/200ms motion, and reduced motion. Enforce the research artifact's exhaustive final literal allowlists.

Port every 001/002 variant to those shared primitives, replace all textual `G` marks and selector-mapped raw control glyphs/placeholders with canonical tagged SVGs, and restyle the utility controls as `GOLC Paper`. Give 001 D and 002 A winner-specific treatment from the research. Add selector-stable compact detail contracts without removing the desktop target: 001 uses `.compact-detail-toggle[data-detail-toggle="001-d"][aria-controls="inspector-001-d"]` -> `#inspector-001-d`; 002 uses `.compact-detail-toggle[data-detail-toggle="002-a"][aria-controls="inspector-002-a"]` -> `#inspector-002-a`. The trigger is hidden at desktop and visible at compact width; it updates `aria-expanded` and target visibility through only the bounded disclosure handler. Correct initial winners to 001 D and 002 A while preserving variant order.

Write `%TEMP%\golc-260723-sgy\validate-task-1.cjs` as a thin caller of `structure-validator.cjs`. It must validate the two completed files, theme/font contract, font hashes, selector-specific disclosure bindings, exhaustive literal/glyph inventories, forbidden stale constructs, and `git diff --check` through `spawnSync('git', ['diff','--check','--', ...ownedTextFiles])`. Run it successfully, then commit only Task 1 repository paths with message `feat(260723-sgy): port site language to shell and programming sketches`. Do not add dependencies, remote assets, dynamic HTML injection, marketing chrome, or temporary artifacts to the repository.</action>
  <verify>
    <automated>node "C:/Users/Lawrence/AppData/Local/Temp/golc-260723-sgy/validate-task-1.cjs"</automated>
  </verify>
  <done>The uniquely mapped four-file baseline exists; its structural/inventory validator passes for completed 001/002; the shared local-font/design primitive contract and exhaustive allowlists pass; compact trigger/target selectors are exact; winners are D/A; and the Task 1 commit contains only theme/fonts/001/002.</done>
</task>

<task type="auto">
  <name>Task 2: Port canonical 003/004 against the shared contract</name>
  <files>.planning/sketches/003-performance-workspace/index.html, .planning/sketches/004-patch-to-play-flow/index.html</files>
  <read_first>
    - `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/260723-sgy-RESEARCH.md` — sections “Current Sketch Mismatch Audit,” “What Transfers vs. What Does Not,” and “Validation Architecture.”
    - `.kimi-code/skills/sketch-findings-golc/references/application-shell-navigation.md` — locked 001 D shell organization and persistent-control contract.
    - `.kimi-code/skills/sketch-findings-golc/references/programming-scene-authoring.md` — locked 002 A scene-led workflow and exactly four layer rows.
    - `.kimi-code/skills/sketch-findings-golc/references/live-operation-safety-midi.md` — locked 003 A launcher, masters, locked control, pickup, and safety semantics.
    - `.kimi-code/skills/sketch-findings-golc/references/onboarding-readiness-impact.md` — locked 004 B guided setup, evidence, deterministic review, and optional-exit semantics.
    - `.planning/sketches/themes/default.css` — completed Task 1 shared design-language contract.
    - `%TEMP%\golc-260723-sgy\structure-validator.cjs`, baseline map, and unique 003/004 baselines — preservation/inventory contract to reuse without weakening.
  </read_first>
  <action>Apply the completed shared grammar across every 003/004 variant, then prioritize the approved winners. Replace textual marks and selector-mapped raw control glyphs/placeholders with `data-sgy-visual` canonical SVGs while preserving accessible labels and all original semantic copy.

For 003 A, preserve the random-access launcher, quick masters, four live layers, live-state detail, locked scope, MIDI pickup semantics, and safety footer. Use the 16% selected well plus boundary/text/icon evidence; Paper transient toast; readable live labels; and explicit icon/border/text treatment for `LOCKED` and pickup states. Add `.compact-detail-toggle[data-detail-toggle="003-a"][aria-controls="live-inspector-003-a"]` -> `#live-inspector-003-a`, using only the bounded disclosure handler and preserving desktop visibility. Keep A initially active and A/B/C order unchanged.

For 004 B, preserve the resumable five-step rail, focused guide, three choices, deterministic impact review, Back/Continue, live frame, and safety footer. Use Archivo heading hierarchy, 12px choices/8px controls, lift only on browse/select choices, and warning icon/dot plus exact `Review required` copy. Add `.compact-detail-toggle[data-detail-toggle="004-b"][aria-controls="side-panel-004-b"]` -> `#side-panel-004-b`. Make B—and only B—initially active without changing A/B/C order.

Reuse the Task 1 exhaustive color/gradient/shadow/SVG/glyph allowlists and strong normalized preservation comparison; do not introduce a broader exception. Create `%TEMP%\golc-260723-sgy\validate-task-2.cjs` as a thin caller that validates 003/004 signatures, scripts, inventories, exact selectors/targets, D/A/A/B global active state, and `git diff --check`. Run it successfully, then commit only canonical 003/004 with `feat(260723-sgy): port site language to performance and guided sketches`. Do not hide detail or safety at compact width, add page scroll/phone IA, or add hover lift to live/transport/safety controls.</action>
  <verify>
    <automated>node "C:/Users/Lawrence/AppData/Local/Temp/golc-260723-sgy/validate-task-2.cjs"</automated>
  </verify>
  <done>Strong normalized preservation and exhaustive inventory checks pass for 003/004, compact selectors bind exactly to live-inspector/side-panel targets, winners are A/B, all four files now report D/A/A/B, and the Task 2 commit contains only canonical 003/004.</done>
</task>

<task type="auto">
  <name>Task 3: Mirror exactly and run deterministic Playwright verification</name>
  <files>.kimi-code/skills/sketch-findings-golc/sources/themes/default.css, .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/Archivo-700.ttf, .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/Archivo-800.ttf, .kimi-code/skills/sketch-findings-golc/sources/themes/fonts/JetBrainsMono-500.ttf, .kimi-code/skills/sketch-findings-golc/sources/001-workspace-shell/index.html, .kimi-code/skills/sketch-findings-golc/sources/002-programming-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/003-performance-workspace/index.html, .kimi-code/skills/sketch-findings-golc/sources/004-patch-to-play-flow/index.html</files>
  <read_first>
    - `.planning/sketches/themes/default.css` and `themes/fonts/` — completed canonical shared sources.
    - `.planning/sketches/{001-workspace-shell,002-programming-workspace,003-performance-workspace,004-patch-to-play-flow}/index.html` — completed canonical sketch sources.
    - `%TEMP%\golc-260723-sgy\baseline-map.json`, `baseline\*.html`, and `structure-validator.cjs` — unique direct mappings and strong preservation proof.
    - `.kimi-code/skills/sketch-findings-golc/sources/` — packaged destination; preserve all skill references and files outside the eight owned destination paths.
    - `site/package.json` and `site/playwright.config.ts` — installed `@playwright/test`/`playwright` 1.61.1 and Chromium authority; do not install or update packages.
  </read_first>
  <action>Copy complete canonical bytes for the theme, three fonts, and four HTML files to the same relative packaged paths. Extend `%TEMP%\golc-260723-sgy\validate-final.cjs` from the shared validator to run strong normalized preservation for all four mappings, exact D/A/A/B winners, exhaustive literal/glyph inventories, selector-specific disclosure binding, sibling/canonical font hashes, all eight canonical/mirror SHA-256 pairs, and `git diff --check`.

Create `%TEMP%\golc-260723-sgy\playwright-verify.cjs` using `require(path.resolve('site/node_modules/playwright'))`; assert its package version is exactly `1.61.1`. The script itself must start and stop the static server safely with Node `spawn(process.execPath, [path.resolve('site/node_modules/serve/build/main.js'), path.resolve('.planning/sketches'), '-l', '4177', '--no-clipboard'], { windowsHide: true })`, wait for `http://127.0.0.1:4177`, and always terminate the child in `finally`. Test exactly these routes: `/001-workspace-shell/`, `/002-programming-workspace/`, `/003-performance-workspace/`, `/004-patch-to-play-flow/`.

For each route at 1440×900 and 1024×768, assert the expected winner tab and section are initially visible; `document.fonts.ready` resolves and `document.fonts.check('700 16px Archivo')`, `document.fonts.check('800 16px Archivo')`, and `document.fonts.check('500 12px \"JetBrains Mono\"')` are true; the mark SVG and representative 1.75px icon SVG are visible; every variant tab activates its exact target and the winner can be restored; one existing selection/toggle/launcher/choice interaction changes the expected state; a focusable winner control reports a nonzero Signal Blue focus outline; document width/height do not create page-level overflow; and the safety footer plus all three safety buttons have nonzero bounds within the viewport. Dispatch pointer-down on one hold control, wait at least 750ms, assert its active text, then release it.

At 1024×768 only, bind the exact trigger/target selectors defined in Tasks 1/2, assert the target is initially compact/closed as designed, click the trigger, then assert `aria-expanded=\"true\"`, target computed visibility, nonzero target bounds, target bounds within the viewport or its bounded scroll container, and safety visibility. This browser trigger->target proof replaces broad regex inference.

Write exactly these eight screenshots outside the repository under `%TEMP%\golc-260723-sgy\screenshots`: `001-d-desktop-1440x900.png`, `001-d-compact-1024x768.png`, `002-a-desktop-1440x900.png`, `002-a-compact-1024x768.png`, `003-a-desktop-1440x900.png`, `003-a-compact-1024x768.png`, `004-b-desktop-1440x900.png`, `004-b-compact-1024x768.png`. Parse each PNG IHDR and fail unless its pixel dimensions equal the suffix. Fail on missing/extra PNGs. The Playwright script must call `validate-final.cjs` first and exit nonzero on any assertion.

Run the final script, inspect the eight images, and fix any clipping, hierarchy, interaction, focus, detail-access, or safety defect. After a canonical fix, rerun its owner validator, create a focused canonical fix commit, recopy the exact mirror, and rerun final verification. Commit only the eight packaged paths with `chore(260723-sgy): mirror validated sketch design sources`. Keep temporary scripts/server/screenshots outside the repository and preserve unrelated dirty work.</action>
  <verify>
    <automated>node "C:/Users/Lawrence/AppData/Local/Temp/golc-260723-sgy/playwright-verify.cjs"</automated>
    <human-check>Review the eight temporary screenshots as four desktop/compact pairs. For 001 D, verify the grouped command rail, scene/layer/timeline canvas, inspector access, and safety footer. For 002 A, verify the scene stack, exactly four layer rows, reusable looks, timeline, inspector access, and focus states. For 003 A, verify launcher density, active and locked pads, pickup evidence, live detail access, and safety footer. For 004 B, verify the five-step guide, three choices, Review required panel, Back/Continue controls, compact preview access, and safety footer. In every pair confirm the real fonts/mark/icons, Paper surface hierarchy, restrained casing, readable compact layout, no clipped essential controls, and no hidden live/safety truth.</human-check>
  </verify>
  <done>The strong final validator and installed Playwright 1.61.1/Chromium suite pass; selector-bound compact targets open at 1024×768; eight and only eight correctly dimensioned screenshots exist outside the repository; mirrors hash-match; screenshots are approved; and the final commit owns only packaged mirror paths.</done>
</task>

</tasks>

<source_audit>

| Source | ID | Required outcome | Task | Status |
|---|---|---|---:|---|
| GOAL | quick-task | Port the full GOLC site design language into all approved sketch assets, correcting palette-only work | 1-3 | COVERED |
| REQ | PLAY-01 | Preserve complete on-screen authoring/playback workflow evidence | 2-3 | COVERED |
| REQ | PLAY-03 | Preserve constrained operator-surface and locked-control evidence | 2-3 | COVERED |
| REQ | PLAY-07 | Preserve persistent live truth and non-color state distinctions | 1-3 | COVERED |
| REQ | PLAY-10 | Preserve patch/deployment flow and deterministic impact review | 2-3 | COVERED |
| REQ | PLAY-11 | Preserve deployment/output readiness evidence | 2-3 | COVERED |
| REQ | PLAY-12 | Preserve scene/look programming and exactly four layer roles | 2-3 | COVERED |
| RESEARCH | typography | Bundle Archivo/JetBrains faces and restore title/body/metadata hierarchy with restrained casing | 1-3 | COVERED |
| RESEARCH | surfaces-controls | Implement 12/8/6/pill radii, page/panel nesting, buttons, chips, forms, focus, and selective motion | 1-3 | COVERED |
| RESEARCH | mark-icons | Replace placeholder marks/glyphs with canonical beam SVG and 1.75px outline icons | 1-3 | COVERED |
| RESEARCH | responsive | Transform compact inspectors/details into reachable disclosure UI rather than hiding them | 2-3 | COVERED |
| RESEARCH | per-sketch | Apply the complete 001 D, 002 A, 003 A, and 004 B mismatch corrections | 2-3 | COVERED |
| RESEARCH | validation | Structural/script, palette, mirror, browser, and desktop/compact screenshot checks | 2-3 | COVERED |
| CONTEXT | winner-001 | Initial winner is 001 D, Focused Command Rail | 2-3 | COVERED |
| CONTEXT | winner-002 | Initial winner is 002 A, Scene Stack + Inspector | 2-3 | COVERED |
| CONTEXT | winner-003 | Initial winner is 003 A, Launcher + Masters | 2-3 | COVERED |
| CONTEXT | winner-004 | Initial winner is 004 B, Guided First Show | 2-3 | COVERED |
| CONTEXT | preservation | Preserve IA, flows, scripts, content, and variant order | 1-3 | COVERED |
| CONTEXT | mirrors | Canonical and packaged sources remain exact mirrors | 3 | COVERED |

</source_audit>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|---|---|
| Sibling `golc-site` -> canonical sketch assets | External repository files provide trusted visual geometry, palette, and font binaries that must be copied without drift or remote runtime dependency. |
| Canonical HTML -> browser demo behavior | Presentation edits share files with interaction scripts and can accidentally alter workflow or add unsafe DOM behavior. |
| Canonical sketches -> packaged skill sources | Duplicate assets can diverge and give downstream agents stale design evidence. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|---|---|---|---|---|---|
| T-Q-SGY-01 | Tampering | workflow structure and initial winners | high | mitigate | Capture pre-edit baselines; compare scripts; assert variant counts/order, workflow evidence, and exact D/A/A/B active pairs. |
| T-Q-SGY-02 | Tampering | font/mark/icon authority | medium | mitigate | Hash font binaries against sibling sources and assert canonical mark/icon geometry in every canonical HTML file. |
| T-Q-SGY-03 | Tampering | packaged mirrors | medium | mitigate | Require SHA-256 equality for theme, three font binaries, and four HTML pairs after all visual fixes. |
| T-Q-SGY-04 | Elevation of Privilege | static demo script surface | medium | mitigate | Preserve existing script blocks exactly, forbid dynamic HTML injection APIs, and add no network/package/runtime authority. |
| T-Q-SGY-05 | Denial of Service | compact operational access | medium | mitigate | Browser-test 1024x768 layouts and require reachable detail/safety controls with bounded internal scrolling. |
| T-Q-SGY-06 | Information Disclosure | static sketch/font assets | low | accept | Owned design examples and public font binaries contain no credentials, personal data, persistence, or production state. |
</threat_model>

<verification>
- Shared-token checks cover offline fonts, surface/radius/spacing primitives, focus-visible, selective motion, and reduced motion.
- Static inventory checks enumerate and allowlist every final hex, rgb/rgba, gradient, box/text shadow, SVG paint, placeholder selector, raw control glyph, and non-ASCII code point; they reject stale marks, two-letter wells, transport/chevron glyphs, dark utility chrome, broad transitions, unsafe DOM APIs, and pre-port radii.
- Strong normalized structural checks use four uniquely named direct baselines and preserve visible copy, IDs, ordered original `data-*`, ordered controls with types/names/values/states, variant order, and original script; only tagged marks/icons/disclosures and D/A/A/B active corrections are allowed.
- SHA-256 checks cover all eight canonical/package pairs and the three sibling/canonical font-source pairs.
- Installed Playwright 1.61.1/Chromium serves the four exact loopback routes on port 4177 and checks fonts, variant switching, representative selection/toggle behavior, focus, page overflow, safety bounds/hold behavior, and selector-bound compact trigger-to-target visibility.
- Screenshot review covers exactly eight named PNGs with parsed 1440×900 or 1024×768 dimensions, including fonts, mark/icons, hierarchy, states, focus/hover, internal scroll, overflow, compact detail access, and safety persistence.
- `git diff --check` passes for every owned text file, and `git status --short` confirms unrelated bootstrap and quick-task changes remain untouched.
</verification>

<success_criteria>
- The result visibly reads as the sibling site's complete Paper/Ink design system adapted to a dense desktop console, not as the old sketch with a new palette.
- All variants remain available in their original order and preserve their existing flows/scripts; the initial winners are exactly 001 D, 002 A, 003 A, and 004 B.
- Approved winners retain their locked IA and workflow evidence while gaining real fonts, mark/icons, component hierarchy, controls, semantic states, focus, and responsive detail access.
- Desktop and compact screenshot pairs are readable, unclipped, internally scrollable, and keep live truth plus independent safety actions available.
- Theme, fonts, and HTML are byte-identical between canonical and packaged locations, with no unrelated repository changes.
</success_criteria>

<output>
Create `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/260723-sgy-SUMMARY.md` when done.
</output>
