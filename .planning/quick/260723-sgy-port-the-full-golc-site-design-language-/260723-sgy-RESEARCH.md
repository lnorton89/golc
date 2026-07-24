# Quick Task 260723-sgy: Full GOLC Site Design-Language Port — Research

**Researched:** 2026-07-23  
**Domain:** Visual-system transfer from `golc-site` into dense desktop-console sketches  
**Confidence:** HIGH — the sibling site's complete `src/app`, `src/components`, and `src/lib` trees and all four current sketch HTML/CSS files were inspected directly. [VERIFIED: codebase inspection]

<user_constraints>
## User Constraints

### Locked Decisions

- Port the **full** design language from `C:/Users/Lawrence/Documents/Dev/golc-site`, correcting the prior palette-only pass. [VERIFIED: quick-task scope]
- Preserve the approved sketch information architecture, flows, interactions, and winners: **001 D, 002 A, 003 A, 004 B**. [VERIFIED: quick-task scope; `.planning/sketches/MANIFEST.md`; `.planning/sketches/WRAP-UP-SUMMARY.md`]
- Do not collapse the dense desktop console into a marketing-page layout; distinguish transferable visual grammar from site-specific presentation. [VERIFIED: quick-task scope]
- Do not edit assets during research. [VERIFIED: quick-task scope]
</user_constraints>

## Summary

The previous pass copied the site's Paper/Ink palette, font-family names, three spacing tokens, radii tokens, motion durations, and hover shadow into `themes/default.css`, then recolored the four sketches. It did **not** port the sibling site's actual component grammar: its large-versus-small type contrast, restrained casing, deliberate tracking, 1px-border/12px-card hierarchy, nested page/panel alternation, consistent pills and buttons, real GOLC beam mark, 1.75px square-ended outline icons, selective motion, or master-detail responsive transformations. [VERIFIED: `.planning/quick/260723-rym-*/260723-rym-PLAN.md`; `golc-site/src/app/globals.css`; `golc-site/src/components/*`; current sketch CSS]

The implementation should retain each winning console's fixed global frame, command rail, workspace canvas, inspector, and safety footer, but rebuild its visual vocabulary from reusable site-derived primitives. The correct translation is **site grammar at console density**: same typography roles, border/radius families, selection treatments, icon construction, state chips, and motion behavior; tighter spacing and smaller display sizes than marketing pages; no marketing header/footer, hero whitespace, CTA composition, or broad breakpoint-driven disappearance of operational controls. [VERIFIED: synthesis of `golc-site` patterns and locked sketch IA]

**Primary recommendation:** create a shared console design layer first (fonts, type roles, surfaces, borders/radii, controls, chips, icons/mark, motion/focus), then restyle only the approved variants without changing their DOM order, dimensions, or workflows. [VERIFIED: repeated site component patterns; locked quick-task scope]

## Project Constraints (from AGENTS.md)

- The desktop UI remains a projection of Go-owned commands and state; playback/Art-Net timing cannot depend on rendering. [VERIFIED: `AGENTS.md`]
- Persistent live truth and operator authority must remain visible; Revoke Automation is a local priority path distinct from Blackout. [VERIFIED: `AGENTS.md`]
- Signal/state meaning must not be reduced to color alone. [VERIFIED: `AGENTS.md`; `golc-site/src/components/StatusChip.tsx`]
- Phase 6 uses Go + Wails with React/TypeScript; the design port must remain suitable for a Windows-first dense operator surface. [VERIFIED: `AGENTS.md`]
- Safe structural edits and deterministic impact review remain core behavior; styling may not obscure or bypass preview-before-commit flows. [VERIFIED: `AGENTS.md`]
- Work is under the active GSD quick-task workflow; unrelated dirty-worktree changes must be preserved. [VERIFIED: `AGENTS.md`; `git status --short`]

## Source Inventory and Representative Authority

| Source | What it establishes |
|---|---|
| `golc-site/src/app/globals.css` | Theme variables, Archivo/JetBrains roles, 120/200ms motion, card hover, 12px-card language, dot-grid motif. [VERIFIED: local source] |
| `golc-site/src/app/layout.tsx` | Actual Archivo weights 400–900, JetBrains Mono 400–600, anti-aliasing, page/header/main/footer frame. [VERIFIED: local source] |
| `golc-site/src/app/page.tsx` | Full hierarchy, spacing cadence, cards, buttons, status pills, product-preview density, alternating surfaces, CTA motif. [VERIFIED: local source] |
| `golc-site/src/app/{architecture,docs,roadmap,changelog}/page.tsx` | Repeated page headers, section rhythm, compact data views, empty/coming-soon composition. [VERIFIED: local source] |
| `golc-site/src/components/{SiteHeader,SiteFooter,SectionHeading,StatusChip}.tsx` | Brand lockup, navigation grammar, section labels, state-card treatment. [VERIFIED: local source] |
| `golc-site/src/components/{docs,roadmap,architecture}/*` | Master-detail, sticky inspector, compact list rows, accordions, tree controls, graph/data-panel patterns. [VERIFIED: local source] |
| `golc-site/src/components/GolcMark.tsx`, `icons.tsx` | Canonical beam mark and icon geometry. [VERIFIED: local source] |
| `golc-site/src/lib/og-template.tsx` | Explicit brand-lockup proportions and spectrum-strip usage. [VERIFIED: local source] |
| `.planning/sketches/themes/default.css` and four `index.html` files | Current token mapping, component geometry, winners, and palette-only mismatches. [VERIFIED: local source] |

All sibling `src` files were reviewed. Data-only files (`docsData.ts`, `phaseData.ts`, `repoTreeData.ts`) inform content density but introduce no additional visual rules beyond their rendering components. [VERIFIED: complete `golc-site/src` file inventory and inspection]

## Implementation-Ready Design Rules

### 1. Typography hierarchy and casing

The sibling site uses Archivo for prose/display and JetBrains Mono for machine-readable metadata. The sketches currently only name those fonts in a fallback stack; they do not load them. Package the same local font files or add equivalent Wails-bundled `@font-face` declarations before visual verification. [VERIFIED: `golc-site/src/app/layout.tsx`; `golc-site/src/app/fonts/*`; `.planning/sketches/themes/default.css`]

Use this console-adapted role map:

| Role | Rule to implement | Site evidence |
|---|---|---|
| Brand/show title | Archivo 800, 16–18px, `letter-spacing:-0.02em`; title case except literal `GOLC`. | `SiteHeader.tsx:21-25` uses `text-lg font-extrabold tracking-[-0.02em]`; OG lockup uses Archivo 800. [VERIFIED: local source] |
| Workspace title | Archivo 700, 18–20px, `letter-spacing:-0.01em`; title case. Do not uppercase. | Site cards use 18px/600 and headings use negative tracking. [VERIFIED: `app/page.tsx:228-233`, `SectionHeading.tsx`] |
| Panel title | Archivo 600, 12–13px, title case; no tracking. | Detail/card titles use `text-sm font-semibold text-ink`. [VERIFIED: `PhaseDetail.tsx`, `ViewDetail.tsx`, `DependencyGraph.tsx`] |
| Eyebrow/section index | JetBrains Mono 500–600, 10–11px, uppercase only for short categorical metadata, tracking `0.10–0.14em`; Signal Blue for high-level section identity, muted for local labels. | `SectionHeading.tsx` uses mono 13px/1.3px; repeated local labels use `font-mono text-[10px] uppercase tracking-wider text-muted`. [VERIFIED: local source] |
| Body/help | Archivo 400, 12–14px, line-height 1.5–1.7; use `text` for primary explanation and `text2`/muted for secondary. | Site body cards use 14px/24px; compact graph details use 12px/20px. [VERIFIED: `app/page.tsx`, `DependencyGraph.tsx`] |
| Numeric/readout/key | JetBrains Mono 500–600, 10–13px, tabular numerals; preserve original case for values. | Product preview, dependency metadata, tags, and tree paths consistently use mono. [VERIFIED: `app/page.tsx:283-350`, architecture components] |

Do **not** uppercase every panel header, rail group, label, and safety caption. Reserve uppercase mono for compact metadata (`LIVE`, `FRAME LOCK`, `BPM`, `STEP 02`, field categories); keep actionable navigation, panel names, scene/look names, and button text in title/sentence case. [VERIFIED: site headers/nav/buttons are normal case while metadata labels are uppercase mono]

### 2. Spacing cadence and density

The site uses a strong 4px-derived cadence: micro gaps 6–8px; control/content gaps 12–16px; card padding 16/20/24px; section gaps 24–40px; page sections 64px mobile and 96px desktop. [VERIFIED: `SiteHeader.tsx`; `app/page.tsx`; architecture/docs/roadmap components]

Translate that into the console without importing marketing whitespace:

- Keep the locked 52px global bar, 42px workspace bar, 48px safety footer, 186px rail, and 258px inspector geometry. [VERIFIED: approved sketch winners]
- Normalize micro spacing to 4/8/12/16/24px; remove arbitrary 3/5/6/7/9/10px gaps except where a one-pixel optical adjustment is intentional. [VERIFIED: comparison between site cadence and current sketch CSS]
- Use 8px row padding, 12px compact card padding, 16px inspector section padding, and 24px only for focused onboarding content. [VERIFIED: console-density adaptation of repeated site `p-3/p-4/p-6` patterns]
- Maintain deliberate negative space around the **current task**, not between every control: one clear canvas gutter (8–12px), one inspector section rhythm (16–20px), and compact list rows. [VERIFIED: site's master-detail layouts and approved dense-console requirement]

### 3. Borders, radii, surfaces, and card construction

The site's base border is 1px `line`. Structural cards/panels overwhelmingly use `rounded-xl` (12px), compact interactive rows/icons use `rounded-lg` (8px), buttons/row highlights use `rounded-md` (6px), and state chips are full pills. [VERIFIED: `app/page.tsx`; `StatusChip.tsx`; `ViewExplorer.tsx`; `PhaseTimeline.tsx`; `RepoTree.tsx`]

Port this family directly:

- Shell separators: square, 1px `line`; use the stronger muted border only for high-risk or hardware-like controls. [VERIFIED: site header/footer and section borders use `border-line`]
- Primary panels/cards: 12px radius, 1px line, `panel` over `page`; nested cards alternate `page` within `panel` or `panel` within `page`. [VERIFIED: repeated `rounded-xl border border-line bg-panel/bg-page`]
- Interactive rows/control groups: 6–8px radius, never the current blanket 3px. [VERIFIED: site `rounded-md`/`rounded-lg` row patterns]
- Chips: pill radius; 1px line; page background when nested in panel; mono 10px; dot + label for state. [VERIFIED: `PhaseDetail.tsx:64-71`, `ViewDetail.tsx:63-73`]
- Semantic top rule: use a **2px** top border for categorized cards/layers, not the sketches' 3px rail/layer accents. [VERIFIED: `StatusChip.tsx:16-18`; `app/page.tsx:224-247`]
- Shadows are exceptional: no default panel elevation. Apply `0 8px 24px rgba(0,0,0,.08)` only to hoverable cards, transient menus, toasts, or floating overlays. [VERIFIED: `globals.css:121-134`; `MobileMenu.tsx`]

Section/card construction should follow `surface -> 1px boundary -> compact header/content`, with hierarchy carried by nested surface alternation and type—not extra outlines on every cell. [VERIFIED: site master-detail, repository tree, product preview]

### 4. Header, navigation, and footer grammar

Transfer the site's lockup grammar into the persistent desktop top bar: real 28–32px `GolcMark`, two-line brand block, bold Archivo primary line, 10px uppercase mono secondary line. In the console, the primary line may remain the current show name, but the real mark and hierarchy must replace the placeholder `<div class="mark">G</div>`. [VERIFIED: `SiteHeader.tsx:17-27`; current winner markup]

The desktop command rail is not a marketing nav, but its interaction grammar should match site master-detail lists: normal-case labels, 8px rounded row, muted inactive text, page/soft-blue selected background, blue icon/indicator, and 120ms color transition. Preserve Show/Build/Operate/Output grouping and all destinations. [VERIFIED: `ViewExplorer.tsx:16-57`; `PhaseTimeline.tsx:17-65`; approved 001 D]

The safety footer is also not a site footer. Preserve it as a persistent local-authority cluster, but make its construction site-consistent: 1px top line, clear mono eyebrow, 6px/8px controls, red only for Revoke, ink/blackout treatment for Blackout, armed amber for Stop/Release, and explicit text labels. Do not turn it into a decorative red strip. [VERIFIED: `AGENTS.md`; `StatusChip.tsx`; approved sketch safety contract]

### 5. Controls, buttons, chips, and forms

Use three button levels derived from the site:

- Primary: Signal Blue background/border, on-accent text, 6px radius, 12px/20px compact/regular horizontal padding, 600 weight, hover to deep blue. [VERIFIED: `SiteHeader.tsx:45`; `app/page.tsx:152`; `docs/page.tsx:96`]
- Secondary: transparent or page surface, 1px line, ink text, 6px radius; hover changes border and text to blue. [VERIFIED: `app/page.tsx:159`; `changelog/page.tsx:50`]
- Icon-only: 32–36px square/circle, 1px line, panel surface; hover changes border to blue. [VERIFIED: `ThemeToggle.tsx:20`; `MobileMenu.tsx:24`]

Do not use `transition: all`. Transition only the changed properties: color/background/border at 120ms; transform/shadow at 200ms. Pressed/selected state must not animate layout. [VERIFIED: `globals.css:121-129`; site button classes]

Forms should use Archivo value text, mono uppercase 10px labels, page-filled inputs nested in panels, 1px line, 6px radius, and a visible Signal Blue focus ring/border. The website has little form content, so the sketches' inspector structure remains authoritative; only the site's type/surface/control grammar transfers. [VERIFIED: site source inventory; current inspector markup]

For binary switches, replace the raw `background:white` knob with an on-accent/mark-bar token and add an accessible outline/focus state. [VERIFIED: `.planning/sketches/002-programming-workspace/index.html:66`; site theme token contract]

### 6. Mark, icons, and spectrum usage

Use the canonical beam mark exactly: rounded square tile with six spectral beams and top bar. Do not approximate it with a `G`. Reuse `GolcMark.tsx` geometry or an equivalent local SVG at 28px in chrome and 32px where space permits. [VERIFIED: `GolcMark.tsx`; `icon.tsx`; current sketch markup]

Use one outline icon family matching `icons.tsx`: 24×24 viewBox, `fill:none`, `stroke:currentColor`, `stroke-width:1.75`, square caps, round joins; filled geometry only where the source icon intentionally does so. Use 14–18px icons in rail/rows and 18–20px in feature tiles. [VERIFIED: `icons.tsx:1-17` and exported icons]

Spectrum colors are secondary categorical accents, not general decoration. Appropriate uses in the console are 2px card/layer top rules, small icon wells with a 16% tint, channel/bar visualization, and the canonical mark. Signal Blue remains selection/live/manual control; green frame lock; amber armed/pickup; red revoke/error; ink blackout; muted offline. [VERIFIED: `app/page.tsx:224-259`, `StatusChip.tsx`, `GolcMark.tsx`]

### 7. Motifs, shadows, and motion

The only reusable background motif is the subtle 20px dot grid: `radial-gradient(var(--line) 1px, transparent 1px)` on panel. Use it sparingly in empty states, onboarding hero/impact-review summaries, or a single central canvas well—not behind dense lists, faders, or timelines. [VERIFIED: `globals.css:136-141`; site use in CTA]

Card hover is `border -> accent`, `translateY(-2px)`, and the light shadow over 120/200ms. In the desktop console, restrict that lift to browse/select cards such as fixture choices or onboarding choices; list rows, faders, transport, safety, and launcher pads should use color/border changes without vertical movement. [VERIFIED: `globals.css`; dense-console interaction requirements]

No bounce, spin, or ambient animation. Chevron rotation, disclosure, hover, selection, toast entry, and subtle card lift are the motion vocabulary. Respect reduced-motion even though the current site does not yet define a media override. [VERIFIED: `globals.css` comment and component transitions; reduced-motion recommendation is `[ASSUMED]` accessibility best practice]

### 8. Responsive behavior

The site does not merely hide the right column: at `lg` it uses master-detail (`1fr 1.3fr`) with a sticky detail card; below `lg` it converts the same content to one-open-at-a-time accordions. Header nav becomes a mobile menu; grids step 3→2→1 columns; padding changes 48→24px. [VERIFIED: `ViewExplorer.tsx`; `PhaseTimeline.tsx`; `MobileMenu.tsx`; page grids]

For GOLC's Windows desktop console:

- Keep the fixed operational frame at supported desktop widths; use internal scroll, not page scroll. [VERIFIED: approved sketch IA]
- At compact width, convert inspector content to a drawer/accordion or explicit toggle; do not silently `display:none` operational detail. [VERIFIED: site responsive master-detail pattern; current sketch mismatch]
- Rail may narrow to icon + selected label, but workspace destinations and safety actions remain reachable. [VERIFIED: approved persistent-control contract]
- Launcher/matrix grids should horizontally scroll or reduce columns intentionally; do not crush cells below their minimum usable width. [VERIFIED: current grid min-width rules and dense-console requirements]
- This task should not add mobile/phone IA; Windows v1 is the qualification target. [VERIFIED: `AGENTS.md`]

## What Transfers vs. What Does Not

| Transfer into desktop console | Keep marketing-site-specific |
|---|---|
| Archivo/JetBrains roles, weight/tracking hierarchy, selective uppercase. [VERIFIED: site-wide] | 44–64px hero type and long-form 18–20px body copy. [VERIFIED: home hero] |
| 1px line boundaries; 12/8/6px radius family; pill chips. [VERIFIED: site-wide] | 1160px centered page container and 64–112px vertical section padding. [VERIFIED: page layouts] |
| Page/panel alternation and nested card construction. [VERIFIED: site-wide] | Marketing header links, GitHub CTA, site footer, and page-to-page navigation. [VERIFIED: `SiteHeader`, `SiteFooter`] |
| Real GOLC mark, square-ended outline icons, restrained spectrum accents. [VERIFIED: mark/icons/cards] | Large logo showcase, full-width spectrum strip, and positioning illustration. [VERIFIED: home/OG sources] |
| Primary/secondary/icon button grammar; mono state pills; state dot + text. [VERIFIED: components/pages] | CTA-centered composition and sales-copy card grids. [VERIFIED: home/docs CTA] |
| 120/200ms selective transitions; rare hover shadow/lift. [VERIFIED: globals] | Hover lift on live controls, faders, transport, or safety actions. [VERIFIED: console-specific safety constraint] |
| Master-detail to accordion/drawer responsive transformation. [VERIFIED: docs/roadmap] | Phone-first layout as a v1 requirement. [VERIFIED: Windows-first constraint] |
| Dot grid for a limited empty/onboarding surface. [VERIFIED: globals] | Dot grid as an application-wide canvas texture. [VERIFIED: site motif is limited to CTA] |

## Current Sketch Mismatch Audit

### Shared across all four sketches

1. **Fonts are declared but not loaded.** `default.css` names Archivo and JetBrains Mono, but the HTML files only link `default.css`; no `@font-face` or font asset is present, so most environments render Segoe UI/Cascadia fallbacks. [VERIFIED: sketch `<head>` and theme; site `layout.tsx`/font files]
2. **Brand mark is a placeholder.** Every winning top bar uses `<div class="mark">G</div>` rather than the canonical spectral-beam SVG. [VERIFIED: four winner markups; `GolcMark.tsx`]
3. **Radius system is still pre-port.** Most panels, rows, fields, buttons, cards, and toasts use 3–5px radii, while the site grammar is 12px structural, 8px compact card/icon, 6px button/row, pill chip. [VERIFIED: sketch CSS; site class audit]
4. **Typography is over-compressed and over-uppercase.** Panel headers, rail labels, field labels, safety labels, states, and many values are 8–10px uppercase; the site uses that treatment only for metadata and lets titles/actions remain normal case. [VERIFIED: sketch CSS; site type patterns]
5. **Motion is too broad.** Repeated `transition:all` can animate unintended geometry; the site specifies property-scoped transitions. [VERIFIED: sketch CSS; `globals.css`]
6. **Surface hierarchy is flat.** `--color-surface` and `--color-surface-raised` are identical, so hover/raised treatments often show no change; almost every nested item receives its own border instead of using site-like page/panel alternation. [VERIFIED: `themes/default.css`; sketch CSS]
7. **Floating sketch tools retain dark chrome.** `#sketch-tools` uses `rgba(23,24,28,.84)` in all files, visually contradicting the Paper system. In 001 its select still says “Midnight Console.” [VERIFIED: `001:index.html:186+`; `002:137`; `003:53`; `004:35`]
8. **No canonical icon vocabulary.** Transport uses text glyphs and most navigation has no icons, despite the site's consistent 1.75px SVG system. [VERIFIED: sketch markup; `icons.tsx`]
9. **Focus states are missing.** Buttons, fields, rows, switches, and launcher pads rely on hover/selected styling; site-derived blue focus-visible treatment is not represented. [VERIFIED: sketch CSS audit]
10. **Compact response hides context.** Inspector/side panels are set to `display:none` at 900/1050px instead of transforming to a drawer/accordion. [VERIFIED: sketch media queries; site master-detail responsive components]
11. **Two approved winners are not the initial active variants.** 001 labels D as selected in the planning artifacts but marks tab/section A active; 004 labels B “Selected” in its tab text but marks tab/section A active. 002 A and 003 A are correct. The implementation must correct 001 to D and 004 to B while preserving all variants and scripts. [VERIFIED: `001:index.html:208,216`; `004:index.html:40,43`; manifest/wrap-up]

### 001 D — Focused Command Rail

- Preserve the locked grouped Show/Build/Operate/Output rail, 52px top frame, central scene/layer/timeline canvas, right inspector, and independent safety footer. [VERIFIED: `001-workspace-shell/index.html` winner D; manifest]
- The rail uses 9px uppercase section labels, 11px text, 3px row radius, and an inset 3px active bar; restyle to site-like normal-case row labels, 8px radius, 16px icon well, soft-blue/page selected surface, and a 2px blue indicator without changing grouping. [VERIFIED: `001:index.html:153-160`; `ViewExplorer.tsx`; `PhaseTimeline.tsx`]
- Panels use 5px radius and layer cards 4px/3px top rules; move to 12px panels, 8px layer cards, 2px semantic top rules. [VERIFIED: `001:index.html:72,88-90`; site card patterns]
- Timeline grid uses a nearly invisible `rgba(244,241,235,.035)` light line and blocks use arbitrary translucent fills; derive the grid from `line` and use 16% categorical wells/clear selected borders. [VERIFIED: `001:index.html:120-123`; `globals.css`; site icon-well patterns]
- Winner D is present but is not initially active: the A tab and A section carry `active`. Correct the initial tab/section markers to D without changing variant order or switcher behavior. [VERIFIED: `001:index.html:208,216`; `MANIFEST.md`; `WRAP-UP-SUMMARY.md`]

### 002 A — Scene Stack + Inspector

- Preserve the scene list, explicit four layer rows, reusable-look browser, bar timeline, contextual inspector, and safety footer. [VERIFIED: `002-programming-workspace/index.html:158+`; manifest]
- Toggle knobs hardcode `background:white`, outside the semantic token contract. [VERIFIED: `002:index.html:66`]
- Timeline uses old dark-canvas `rgba(255,255,255,.04)` grid lines and an unapproved `rgba(96,126,241,.55)` border, proving the hex-only palette check missed RGB literals. [VERIFIED: `002:index.html:89-91`; prior plan verification]
- Selected rows/layers still look like compact developer tooling: 3px radii, 9–10px category labels, border on nearly every row. Apply the site hierarchy without changing row count or editing behavior. [VERIFIED: `002:index.html:75-90`]
- The inspector fields require the new focus-visible/form grammar; their structure and current values stay unchanged. [VERIFIED: current winner inspector; site transfer rules]

### 003 A — Launcher + Masters

- Preserve random-access scene launcher, quick masters, four live layers, live-state inspector, locked scope, MIDI pickup semantics, and safety footer. [VERIFIED: `003-performance-workspace/index.html` winner A; manifest/readme]
- Active pads use `rgba(...,.32)` plus a 3px inset blue bar; translate to the site's soft 16% selected well, 1px blue boundary, and text/icon emphasis. Keep pad geometry and keyboard labels. [VERIFIED: `003:index.html:40-43`; site selected-list patterns]
- The toast inverses to mark-tile/mark-bar while other sketches use Paper panels. Use a consistent transient panel with site shadow unless the message is specifically blackout/revoke severity. [VERIFIED: `003:index.html:36`; other sketches; site transient grammar]
- Numerous live labels are 8–9px, and locked state is only opacity/saturation plus generated text. Increase the label floor, retain explicit `LOCKED`, and add icon/border differentiation. [VERIFIED: `003:index.html:33-35`; color-not-only constraint]
- Compact mode hides the entire live inspector. Replace with reachable drawer/accordion detail while keeping the operator launcher visible. [VERIFIED: `003:index.html:54`; site responsive master-detail]

### 004 B — Guided First Show

- Preserve the resumable guided step rail, focused guide card, three explicit choices, deterministic impact-review warning, Back/Continue controls, persistent live frame, and safety footer. [VERIFIED: `004-patch-to-play-flow/index.html` winner B; manifest]
- Winner B is visibly labeled “Selected” but tab/section A carry `active`. Correct the initial active markers to B without changing the existing variant-switching script. [VERIFIED: `004:index.html:40,43`; manifest/wrap-up]
- The guide's 24px `h1` is the right place for stronger site hierarchy: Archivo 700/800, slight negative tracking, while step names and choice names remain normal case. [VERIFIED: `004:index.html:30`; site heading hierarchy]
- Choice cards use 5px radius and selected state is only blue border/soft fill. Move to 12px cards, 8px inner controls, optional site-style icon well, and 200ms lift only because these are browse/select cards. [VERIFIED: `004:index.html:30`; site capability/CTA cards]
- The impact-review block is inline-styled with amber border/background. Preserve the semantic warning and content, but construct it as a standard panel/card with dot/icon + `Review required` label so color is not the only signal. [VERIFIED: winner markup; status-chip grammar]
- Compact mode hides the whole side panel rather than making guidance/context reachable. Use drawer/accordion behavior. [VERIFIED: `004:index.html:36`; site responsive components]

## Recommended Shared CSS/Component Contract

Implement once, then consume from all four winners:

```css
/* Source patterns: golc-site globals.css, layout.tsx, repeated component classes */
@font-face { font-family: "Archivo"; /* bundled 400–900 weights */ }
@font-face { font-family: "JetBrains Mono"; /* bundled 400–600 weights */ }

:root {
  --radius-panel: 12px;
  --radius-card: 8px;
  --radius-control: 6px;
  --radius-pill: 999px;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-6: 24px;
}

.panel {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-panel);
  background: var(--color-surface);
}

.meta-label {
  font: 600 10px/1.2 var(--font-mono);
  letter-spacing: .1em;
  text-transform: uppercase;
  color: var(--color-text-muted);
}

.button-primary {
  border: 1px solid var(--color-primary);
  border-radius: var(--radius-control);
  background: var(--color-primary);
  color: var(--color-on-primary);
  font-weight: 600;
  transition: background-color var(--transition-tap), border-color var(--transition-tap);
}

.state-chip {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-pill);
  background: var(--color-bg);
  font: 500 10px/1.2 var(--font-mono);
  letter-spacing: .08em;
  text-transform: uppercase;
}

:where(button, input, select, [tabindex]):focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
```

[VERIFIED: implementation synthesis of authoritative local patterns]

## Anti-Patterns to Avoid

- Do not rewrite the winner layouts into centered website sections or add site header/footer chrome. [VERIFIED: task scope]
- Do not make every label uppercase mono; that removes the site's essential type contrast. [VERIFIED: site-wide hierarchy]
- Do not make every panel float with a shadow; site cards are flat until interaction. [VERIFIED: `globals.css` and page components]
- Do not add spectrum color to arbitrary scene tiles; color retains semantic/categorical meaning. [VERIFIED: site/status system; UI research]
- Do not hide safety, live truth, or inspector access at compact widths. [VERIFIED: `AGENTS.md`; workflow map]
- Do not preserve old RGB literals merely because the hex palette audit passes. Audit `rgb()`, `rgba()`, gradients, inline styles, shadows, and SVG fills/strokes. [VERIFIED: `002` timeline mismatch and shared sketch CSS]
- Do not use placeholder letters/Unicode glyphs where the canonical mark/icon set exists. [VERIFIED: current-versus-site comparison]
- Do not alter variant scripts, DOM order, content labels, initial winners, or workflow state while restyling. [VERIFIED: task scope]

## Validation Architecture

This quick task changes static sketch HTML/CSS, so validation should combine structural automation and browser judgment. [VERIFIED: repository artifact type]

| Check | Automated evidence |
|---|---|
| Winners/IA preserved | Snapshot to four uniquely named files (`001-workspace-shell.html`, `002-programming-workspace.html`, `003-performance-workspace.html`, `004-patch-to-play-flow.html`), then compare normalized visible copy, IDs, ordered `data-*` attributes, ordered controls with tag/type/name/value, variant order, and scripts. The only allowlisted differences are mark/icon wrappers, selector-stable compact-disclosure markup/handler blocks, and the 001 D / 002 A / 003 A / 004 B active-marker corrections. [RESOLVED: checker review] |
| Full design primitives present | Assert bundled font declarations, canonical mark SVG points/colors, shared radius tokens, property-scoped transitions, focus-visible rules, and icon base geometry. [VERIFIED: required port rules] |
| No stale palette/chrome | Inventory every hex, `rgb()`/`rgba()`, gradient, `box-shadow`, `text-shadow`, SVG fill/stroke, placeholder mark, raw icon glyph, and non-ASCII code point before and after. Final literals must belong to the explicit allowlists below; `white`, “Midnight Console,” text `G` marks, raw transport squares/triangles, text-only rail abbreviations, and icon-only chevrons are forbidden. [RESOLVED: checker review] |
| Responsive access | Static checks bind each winner's `.compact-detail-toggle[aria-controls]` to its exact unique target ID; Playwright at 1024×768 clicks the trigger and asserts `aria-expanded=true`, target visibility, nonzero target bounds, and visible safety controls. [RESOLVED: checker review] |
| Visual parity | Installed Playwright 1.61.1/Chromium opens loopback routes on port 4177, verifies loaded fonts and interactions, and writes exactly eight named 1440×900 / 1024×768 PNGs outside the repository for inspection. [VERIFIED: repository tooling; RESOLVED: checker review] |
| Mirror parity if packaged copies are updated | SHA-256 compare canonical and `.kimi-code/skills/sketch-findings-golc/sources` pairs. [VERIFIED: existing packaging contract] |

### Exhaustive final literal allowlists

- **Hex colors:** `#e4e0d8`, `#f4f1eb`, `#17181c`, `#4a4941`, `#57564e`, `#8a887f`, `#d2ccc0`, `#1b44d9`, `#1233a8`, `#5ac26a`, `#c8a24b`, `#e23a2e`, `#c0554a`, `#cc8a47`, `#b6a24c`, `#4e9e68`, `#6a50a8` (case-insensitive). [VERIFIED: authoritative site palette]
- **RGB/RGBA literals:** only `rgba(0, 0, 0, 0.08)` for the documented transient/browse-card shadow. Selection/status alpha must use semantic `color-mix()` expressions rather than additional numeric RGB literals. [RESOLVED: deterministic audit choice]
- **Gradients:** only the timeline grid `linear-gradient(to right, var(--color-border) 1px, transparent 1px)` and the optional limited empty/onboarding dot grid `radial-gradient(var(--color-border) 1px, transparent 1px)`. No other gradient is allowed. [RESOLVED: transfer rules]
- **Shadows:** only `0 8px 24px rgba(0, 0, 0, 0.08)` for transient/browse-card elevation and `inset 2px 0 var(--color-primary)` for the command-rail selected indicator. Focus uses `outline`, not an additional shadow. `text-shadow` is empty. [RESOLVED: transfer rules]
- **SVG paint:** canonical mark polygons may use the six spectrum hex values and mark tile/bar variables; outline icons use `fill="none"`, `stroke="currentColor"`, `stroke-width="1.75"`, square caps, and round joins, with `fill="currentColor"` only on the source icon paths/rectangles that are filled in `icons.tsx`. [VERIFIED: `GolcMark.tsx`, `icons.tsx`]
- **Allowed non-ASCII visible characters:** middle dot `·` (U+00B7), multiplication sign `×` (U+00D7), en/em dash `–`/`—` (U+2013/U+2014), typographic quotes (U+2018/U+2019/U+201C/U+201D), ellipsis `…` (U+2026), semantic arrows `↑`/`→`/`↓` (U+2191/U+2192/U+2193), check mark `✓` (U+2713), and selected-star `★` (U+2605). Raw play triangle `▶` (U+25B6), stop square `■` (U+25A0), and single guillemets `‹`/`›` (U+2039/U+203A) are forbidden because those controls receive SVG icons. [RESOLVED: placeholder/glyph audit]
- **Placeholder selectors:** `.mark` must contain the canonical SVG and no text node; transport icon buttons and `.icon-button` must contain SVG; former two-letter rail wells (`PX`, `PG`, `PF`, `ST`) are forbidden; textual `+ Scene` remains permitted while an icon-only `+` is forbidden. [RESOLVED: selector-specific audit]

## Security Domain

This research proposes presentation-only changes to static local sketch assets; it adds no authentication, session, access-control, cryptography, persistence, network, or command-execution surface. ASVS V2/V3/V4/V6 do not apply. V5 input handling remains relevant only insofar as existing demo inputs/buttons must not gain unsafe script interpolation; preserve current static event behavior and do not introduce dynamic HTML injection. [VERIFIED: sketch source and proposed scope]

## Assumptions Log

| # | Claim | Risk if wrong |
|---|---|---|
| A1 | Add a reduced-motion override while porting motion. [ASSUMED] | Low; may be deferred if the sketches intentionally mirror the site exactly, which currently lacks this rule. |
| A2 | Use a selector-stable explicit trigger/drawer for compact inspectors. [RESOLVED] | Browser validation binds and exercises each trigger/target pair. |
| A3 | Package the site's three checked-in font binaries into canonical and mirrored theme font paths. [RESOLVED] | Hash equality proves offline source fidelity. |

## Resolved Implementation Decisions

1. **All variants receive the shared grammar; approved winners receive workflow-specific priority. — RESOLVED**
   - Apply the common font, mark/icon, typography, surface, radius, control, chip, focus, and motion contract across every A/B/C/D comparison variant so no asset retains incompatible chrome.
   - Spend winner-specific layout and compact-response effort on 001 D, 002 A, 003 A, and 004 B. Rejected variants remain useful comparisons and keep their existing IA/content/scripts, but are not independently redesigned. [RESOLVED: quick-plan scope and checker review]
2. **Sketch-only variant/tool controls remain visible and become a Paper utility surface. — RESOLVED**
   - Retain the controls because they are required to evaluate variant order, interactions, and viewport behavior.
   - Restyle them with the shared Paper primitives, label the theme `GOLC Paper`, and exclude the utility chrome from product-component inference and winner screenshots where framing permits. [RESOLVED: quick-plan scope and checker review]
3. **Compact detail uses an explicit trigger-to-target contract. — RESOLVED**
   - Each approved winner exposes one selector-stable `.compact-detail-toggle` carrying `aria-controls` and `aria-expanded`; its corresponding inspector/side panel has a unique ID and becomes visible after the trigger is activated at 1024×768.
   - Browser validation must click each trigger and assert the exact controlled target is visible; a stylesheet-only negative grep is insufficient. [RESOLVED: checker review]
4. **Validation uses the repository's installed Playwright 1.61.1 and Chromium. — RESOLVED**
   - Serve `.planning/sketches` on loopback port 4177 with the checked-in `site/node_modules/.bin/serve.cmd`, then run a temporary CommonJS validator through `site/node_modules/playwright`.
   - Capture exactly eight PNGs outside the repository: one 1440×900 and one 1024×768 image for each approved winner. [VERIFIED: `site/package.json`, `site/playwright.config.ts`, installed Playwright/Chromium; RESOLVED: checker review]

## Sources

### Primary — HIGH confidence

- `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/globals.css` — tokens, motion, shadow, motif.
- `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/layout.tsx` — font families and weights.
- `C:/Users/Lawrence/Documents/Dev/golc-site/src/app/page.tsx` and all other app pages — repeated hierarchy, spacing, card, button, and responsive classes.
- `C:/Users/Lawrence/Documents/Dev/golc-site/src/components/**` — complete component grammar, mark, icons, master-detail, chips, navigation.
- `C:/Users/Lawrence/Documents/Dev/golc-site/src/lib/og-template.tsx` — explicit lockup and spectrum treatment.
- `.planning/sketches/themes/default.css` and `.planning/sketches/{001,002,003,004}-*/index.html` — current implementation and mismatches.
- `.planning/sketches/{MANIFEST,UI-RESEARCH,WORKFLOW-MAP,WRAP-UP-SUMMARY}.md` — locked IA, workflows, and winners.

No external web or package research was needed; this is a codebase-only design-language comparison. [VERIFIED: research method]

## Metadata

**Confidence breakdown:**
- Source design-language extraction: HIGH — complete sibling source inspected.
- Sketch mismatch audit: HIGH — all shared and inline CSS/markup inspected.
- Console transfer rules: HIGH — grounded in repeated source patterns and locked GOLC IA.
- Exact compact-width interaction: HIGH — selector-stable trigger/target behavior and Playwright assertions are now resolved in the execution contract.

**Research date:** 2026-07-23  
**Valid until:** Until `golc-site` materially changes its visual system or the approved sketch winners change.
