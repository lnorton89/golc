# Phase 13: Unified UI Design System and Automated Enforcement - Research

**Researched:** 2026-08-02  
**Domain:** React/CSS design-system consolidation, source-policy enforcement, accessibility, and visual regression  
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

### Visual Direction and Scope
- **D-01:** Preserve the approved GOLC Paper/Ink console language and existing theme choices. This is a unification phase, not a rebrand.
- **D-02:** Migrate every currently reachable desktop workspace, shell surface, dialog, editor-adjacent control, and reusable component in this phase. Do not leave a permanent split between "new-system" and legacy UI.
- **D-03:** Keep the dense, instrument-like desktop character: compact operational controls, bounded scroll regions, restrained motion, visible focus, and persistent operator safety controls. Phase 13 standardizes spacing on the 4px grid; it supersedes Phase 9 D-11's inherited 7px Guided First Show grid gap with 8px. The 210px rail width remains a sizing value, not a spacing token.

### Token and Component Contract
- **D-04:** Components consume semantic design tokens for surfaces, text, borders, actions, statuses, typography, spacing, radii, sizing, focus, and motion. Raw palette values remain confined to the theme/token definitions.
- **D-05:** Shared visual behavior belongs in typed React primitives and patterns. Feature CSS Modules may own feature layout and exceptional domain visualization, but may not reinvent common buttons, fields, panels, dialogs, rows, badges, tabs, toolbars, empty/loading/error states, or focus behavior.
- **D-06:** Theme variants must implement the same semantic contract. Feature code cannot branch on a theme name or read theme-specific palette values directly.
- **D-07:** Any necessary exception is declared in a small audited exception manifest with file, rule, rationale, and review condition. Inline suppression without a recorded exception is not allowed, and exceptions cannot bypass the 4px spacing grid.

### Agent Guidance and Enforcement
- **D-08:** Ship a concise design-system guide with component selection rules, token vocabulary, examples, anti-patterns, and a new-component checklist so coding agents have an authoritative path instead of guessing.
- **D-09:** Enforcement runs in normal frontend validation and CI. It must detect raw visual literals outside approved token/theme files, undeclared CSS custom properties, forbidden native control reinvention where a primitive exists, and drift between the documented and exported design-system surface.
- **D-10:** Use layered verification: fast static policy checks, unit/contract tests for primitives and tokens, accessibility interaction checks, and stable Playwright visual coverage for representative shell/workspace states in light and dark modes.
- **D-11:** New enforcement begins green. Existing violations are migrated or explicitly registered; the project does not adopt a permanently ignored baseline that allows known drift to grow.

### Accessibility and Safety
- **D-12:** Every interactive primitive owns consistent hover, active, disabled, loading, and `:focus-visible` states and exposes native semantics. Color is never the only status signal.
- **D-13:** The design system must not weaken the persistent visibility, priority, or independent behavior of Blackout and Revoke Automation.
- **D-14:** Cosmetic UI work remains a projection of Go-owned state and cannot introduce playback, Art-Net timing, script, or safety authority into React.

### the agent's Discretion

Exact file decomposition, checker implementation, component API names, and migration order are left to research and planning, provided the decisions above remain mechanically testable.

### Deferred Ideas (OUT OF SCOPE)

None â€” discussion stayed within phase scope.
</user_constraints>

## Project Constraints (from AGENTS.md)

- Keep Go with Wails and qualify Windows only for v1; portability remains an architectural goal. [VERIFIED: AGENTS.md]
- React/TypeScript is a projection of Go-owned state. UI work may not own or block playback, Art-Net, scripts, automation leases, or safety authority. [VERIFIED: AGENTS.md]
- Keep Blackout and Revoke Automation persistent, distinct, observable, and independent; Revoke Automation is a local priority path separate from blackout. [VERIFIED: AGENTS.md]
- Keep all UI, script, API, and LLM actions converged on shared domain commands/state. [VERIFIED: AGENTS.md]
- Keep the existing React 19, TypeScript 7, Vite 8, Vitest 4, Playwright, CSS Modules, Lucide, Monaco, and Tiptap stack; do not introduce shadcn/Tailwind or a second theme framework. [VERIFIED: AGENTS.md; 13-UI-SPEC.md]
- Use the repository-owned Mage entrypoint and centralized project configuration. Do not create a second CI/policy implementation. [VERIFIED: AGENTS.md; config/commands.toml]
- Use exact package pins and the committed lockfile; do not use floating `latest`. [VERIFIED: AGENTS.md]
- Preserve unrelated working-tree changes, including `internal/deskmidi/`. This research owns only this artifact. [VERIFIED: orchestrator task]

<phase_requirements>
## Phase Requirements

There are no new `REQ-*` IDs. The approved decisions and UI-SPEC acceptance contract are the phase requirements. [VERIFIED: orchestrator task; .planning/REQUIREMENTS.md grep]

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01..D-07 | Preserve Paper/Ink; complete migration; semantic token/component/exception contract | Manifest/export architecture, current violation inventory, component-gap map, and migration slices below. [VERIFIED: 13-CONTEXT.md] |
| D-08..D-11 | Agent guide, DS001–DS010, layered gates, green start | Deterministic checker architecture, fixture strategy, package/Mage/CI integration, and no-baseline rule below. [VERIFIED: 13-CONTEXT.md] |
| D-12..D-14 | Interaction/accessibility states, safety invariants, projection-only UI | Primitive API recommendations, accessibility test map, Playwright safety matrix, and Security Domain below. [VERIFIED: 13-CONTEXT.md; 13-UI-SPEC.md] |
| UI-SPEC acceptance | Full reachable-UI migration, exact export/docs parity, complete theme faces, green static/unit/a11y/visual gates | File plan, validation architecture, and completion gates below. [VERIFIED: 13-UI-SPEC.md] |
</phase_requirements>

## Summary

Phase 13 should be planned as a controlled platform migration, not a visual cleanup. The repository already has a useful seed—12 primitive implementations, semantic-ish global variables, 12 named themes, 53 Vitest files, and a strong Playwright geometry harness—but the reachable UI still has 56 CSS Modules totaling 7,141 lines, 181 production `<button>` elements across 43 files, 60 production `<input>/<select>` elements, 46 parsed raw-color declarations in 10 feature modules, 39 `var()` fallbacks, 59 feature/shell theme-specific rules in five modules, and hundreds of raw typography/spacing/radius values. [VERIFIED: codebase inventory using PostCSS 8.5.22 and codebase grep, 2026-08-02]

Build the enforcement and its positive/negative fixtures before broad migration, then migrate in slices that keep the checker green at every merge. Use one pure-data token manifest to generate CSS and typed token names; use a single component inventory to drive the barrel, guide inventory, and DS007 parity test. Parse CSS with PostCSS and TSX with the official TypeScript 6 compatibility API because the pinned TypeScript 7.0.2 package explicitly does not expose a stable compiler API. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] [CITED: https://github.com/postcss/postcss]

The browser gate must be a dedicated Windows Chromium job, not part of the offline cross-platform Mage observation matrix. Reuse the current healthy Wails mocks, catalog-driven navigation, geometry helpers, and safety assertions; add deterministic seeded states, light/dark screenshots at 900×720 and 1280×720, dialog focus/return-focus, ARIA snapshots, reduced-motion checks, and exact font readiness. [VERIFIED: frontend/e2e/helpers.ts; frontend/playwright.config.ts; .github/workflows/*.yml] [CITED: https://playwright.dev/docs/test-snapshots]

**Primary recommendation:** Establish manifest → generated tokens/types → primitives/patterns → feature layouts as a one-way dependency graph, enforce it with AST/CSS parsing and exact exception records, then migrate shell, front-door, authoring, live/MIDI, and editor/output slices while the existing safety and responsive gates stay green. [VERIFIED: 13-UI-SPEC.md; codebase inventory]

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Token manifest, generation, and theme completeness | Browser / Client build tooling | CDN / Static | Tokens compile into the static CSS/TS bundle and remain cosmetic. [VERIFIED: 13-UI-SPEC.md; frontend architecture] |
| Typed primitives and patterns | Browser / Client | Frontend Server (Wails bridge only) | React owns semantics/presentation; Go remains state and command authority. [VERIFIED: AGENTS.md] |
| DS001–DS010 policy checker | Developer tooling | CI | It reads repository source/manifests and fails validation without becoming runtime code. [VERIFIED: 13-UI-SPEC.md] |
| Unit/accessibility contracts | Developer tooling | Browser / Client | Vitest/jsdom checks component contracts; real-browser behavior remains Playwright-owned. [CITED: https://vitest.dev/guide/browser/why.html] |
| Visual/geometry regression | Browser / Client test runtime | CI | Chromium renders the same HTML/CSS class of engine embedded by WebView2; Windows CI owns baselines. [VERIFIED: frontend/playwright.config.ts; AGENTS.md] |
| Blackout/Revoke/Stop presentation | Browser / Client | API / Backend | UI keeps controls visible, but existing Wails/Go command paths remain authoritative and independent. [VERIFIED: frontend/src/components/SafetyCluster; AGENTS.md] |

## Current Inventory and Migration Slices

### Measured baseline

Counts below are parsed production-source candidates, not a waiver baseline. Phase completion requires zero unregistered findings from the finished checker. [VERIFIED: D-11; 13-UI-SPEC.md]

| Surface | Current count | Planning implication |
|---------|---------------|----------------------|
| CSS Modules | 56 files / 7,141 lines | Plan multiple migration waves; a single “replace tokens” task is too broad. [VERIFIED: filesystem inventory] |
| Production TSX | 80 files | Barrel/pattern adoption must be workspace- and component-scoped. [VERIFIED: filesystem inventory] |
| Existing primitive seeds | 12 implementations | Keep and harden; do not rewrite all primitives from scratch. [VERIFIED: frontend/src/components/primitives] |
| Primitive consumers | 34 production TSX files import the seed surface | Migration must preserve current import compatibility before tightening exports. [VERIFIED: codebase grep] |
| Native controls | 181 buttons / 44 inputs / 16 selects in production | DS005 needs AST context and migration to Button/IconButton/Field/Tabs rather than a text grep. [VERIFIED: TypeScript-source inventory] |
| Raw colors outside global theme file | 46 parsed declarations in 10 modules | Migrate or register only emergency/vendor/domain exceptions. [VERIFIED: PostCSS inventory] |
| Raw `px` dimensions in feature modules | 668 occurrences | Classify by property: shared spacing/type/radius/focus/motion becomes tokens; singular domain geometry needs exact exceptions. [VERIFIED: PostCSS inventory] |
| Feature custom-property declarations | 2 declarations | Guided First Show's local aliases violate the declaration boundary. [VERIFIED: PostCSS inventory] |
| `var()` fallbacks | 39 | Feature fallbacks must be removed; emergency fallback and runtime geometry require approved records. [VERIFIED: PostCSS inventory] |
| Used but stylesheet-undeclared custom properties | 12 | These are runtime geometry/style inputs (`--rail-width`, `--fader-width`, etc.); declare them in a runtime-geometry contract with setter, type/range, and fallback. [VERIFIED: PostCSS inventory; TSX grep] |
| Theme-specific feature rules | 59 parsed rules in five modules | Move semantic variation into theme tokens; feature selectors may not read theme names. [VERIFIED: PostCSS inventory] |
| Raw type declarations | 262 `font-size`, 113 `font-weight`, 206 `font-family`, 17 `line-height` | Typography migration is a major workstream, not polish. [VERIFIED: PostCSS inventory] |
| Other shared visual declarations | 166 radii, 32 motion declarations, 13 z-index declarations, 5 shadows | Centralize in radius/motion/stacking/overlay tokens and primitives. [VERIFIED: PostCSS inventory] |
| Tests | 53 Vitest files / 372 passing tests; 3 Playwright specs / 30 listed tests | Extend existing infrastructure; do not replace it. [VERIFIED: `npm test`; `npx playwright test --list`, 2026-08-02] |

### Hotspots

| Hotspot | Evidence | Natural slice |
|---------|----------|---------------|
| `Desk.module.css` | 1,014 lines; domain geometry, native controls, raw colors, runtime custom properties | Treat as its own live/desk slice after primitives and runtime-geometry schema exist. [VERIFIED: codebase inventory] |
| Project Fixtures / Fixture Patch / Art-Net config | 550/309/382 CSS lines and repeated local buttons, rows, fields, dialogs, empty/error states | Migrate together around FormActions, DataList, ImpactReview, Dialog, and Field family. [VERIFIED: codebase inventory] |
| Settings / Command Rail / Layer Row / Fixture Library | All five feature-theme branching files | Migrate immediately after theme manifest generation to eliminate DS004 early. [VERIFIED: PostCSS inventory] |
| LookBrowser / FixturePatch / ProjectFixtures | 30/22/14 native buttons respectively | Use as DS005 held-out migration tests and primitive API pressure tests. [VERIFIED: TSX grep] |
| Notes / Scripts / Monaco / Tiptap | Specialized editor chrome with repeated toolbar/buttons/dialogs | Migrate shared chrome while retaining editor-owned internals as narrow exceptions. [VERIFIED: 13-UI-SPEC.md; codebase inventory] |

### Recommended migration order

1. **Contract and checker foundation:** manifest schema, generator, generated CSS/TS, exception schema, inventory, DS001–DS010 fixtures, and package scripts. This creates a green ratchet before migration begins. [VERIFIED: D-09..D-11]
2. **Primitive completion:** harden the 12 seeds; add IconButton, field variants, Tabs, LoadingState, ErrorState, Dialog/ConfirmDialog, and typed inventory/barrel. Add a dev-only state gallery. [VERIFIED: 13-UI-SPEC.md component inventory]
3. **Theme and shell:** move all 12 theme names and light/dark faces into the manifest; migrate `index.css`, AppShell, TitleBar, GlobalFrame, CommandRail, inspector, overlays, and SafetyCluster. [VERIFIED: frontend/src/lib/theme.ts; 13-UI-SPEC.md]
4. **Show/front-door:** Overview, Shows, Save/Recovery, Settings, Notes chrome, and Guided First Show. This removes the known 7px→8px change and most front-door state duplication. [VERIFIED: codebase inventory; D-03]
5. **Build/authoring:** Fixture Library, Patch & Pools, Project Fixtures, Scenes & Looks, and their large domain components. Use DataList, FormActions, ImpactReview, SceneStack, and WorkspaceFrame. [VERIFIED: 13-UI-SPEC.md]
6. **Operate/perform:** Operator Surface, Desk, MIDI Mapping, faders, pickup, and safety paths. Keep this isolated because it carries the highest live-operation regression risk. [VERIFIED: AGENTS.md; codebase inventory]
7. **Output/editors:** Art-Net, Diagnostics/logs, Scripts/Monaco, Notes/Tiptap, and remaining specialized exceptions. [VERIFIED: 13-UI-SPEC.md]
8. **Final parity and browser gate:** zero unregistered checker findings, docs/barrel parity, all theme faces, visual matrix, ARIA/focus, 200% zoom, reduced motion, and degraded-daemon safety. [VERIFIED: 13-UI-SPEC.md acceptance]

## Standard Stack

### Core

| Library / tool | Version | Purpose | Why standard here |
|----------------|---------|---------|-------------------|
| React / React DOM | 19.2.7 | Typed primitives and patterns | Existing locked UI stack. [VERIFIED: frontend/package-lock.json] |
| TypeScript CLI | 7.0.2 | Application type-checking | Existing pinned compiler; do not downgrade it for checker tooling. [VERIFIED: frontend/package-lock.json] |
| `@typescript/typescript6` | 6.0.2 | Stable TSX compiler API for the checker | Microsoft states TS7 has no stable API and recommends this side-by-side compatibility package. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] |
| PostCSS | 8.5.22 exact | CSS parsing/source diagnostics | Already resolved under Vite; promote the same version to a direct dev dependency instead of importing an undeclared transitive dependency or hand-rolling CSS parsing. [VERIFIED: npm dependency tree; Context7 `/postcss/postcss`] |
| Vitest + jsdom + Testing Library | 4.1.10 / 29.1.1 / 16.3.2 | Primitive/token contract and interaction tests | Existing green infrastructure. [VERIFIED: frontend/package-lock.json; `npm test`] |
| Playwright | 1.62.0 | Windows Chromium geometry, a11y interaction, and screenshots | Existing real-browser harness and installed browser runtime. [VERIFIED: package lock; e2e config] |

### Supporting

| Library / tool | Version | Purpose | When to use |
|----------------|---------|---------|-------------|
| Lucide React | 1.27.0 | Approved icon set | Primitive-owned icons and non-color state signals. [VERIFIED: frontend/package-lock.json; 13-UI-SPEC.md] |
| CSS Modules | existing Vite behavior | Feature layout and specialized geometry | Keep for layout only; no shared visual behavior or theme branching. [VERIFIED: 13-UI-SPEC.md] |
| Native `<dialog>` behind `Dialog` | WebView2/Chromium platform primitive | Modal semantics/top layer | Wrap it once and explicitly test initial focus, Escape, focus containment, and return focus. [ASSUMED] |
| Mage | 1.17.2 | Root contributor/CI entrypoint | Static/unit gates through `mage Build`; explicit Windows visual target/job for Playwright. [VERIFIED: local environment; magefiles] |

### Alternatives Considered

| Instead of | Could use | Tradeoff |
|------------|-----------|----------|
| PostCSS direct dependency | Regex/handwritten CSS tokenizer | Fewer declared dependencies, but cannot reliably distinguish comments, strings, declarations, nested functions, and syntax errors; conflicts with fail-closed/no-false-positive requirements. [CITED: https://www.w3.org/TR/css-syntax-3/] |
| TypeScript 6 compatibility API | TypeScript 7 `unstable/ast` exports | Avoids one package, but the API is explicitly unstable and TS7.0 officially ships without a stable programmatic API. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] |
| AST-based TSX checks | Text regex | Simpler, but misreads comments/strings/components and cannot reliably inspect JSX tag/attribute context. [VERIFIED: TypeScript compiler API linter example] |
| Existing Vitest + Playwright split | New browser-test framework | No capability gap justifies another runner; jsdom handles contracts and Playwright handles layout/browser behavior. [CITED: https://vitest.dev/guide/browser/why.html] |

**Installation:**

```bash
cd frontend
npm install --save-dev --save-exact postcss@8.5.22 @typescript/typescript6@6.0.2
```

Both additions require a planner `checkpoint:human-verify` before installation because the automated legitimacy seam returned `SUS` solely for recent publish dates, despite official Microsoft/PostCSS sources, established repositories, high download signals, and no postinstall script. [VERIFIED: package-legitimacy seam; npm registry]

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `postcss@8.5.22` [WARNING: flagged as suspicious — verify before using.] | npm | Package created 2013; selected patch published 2026-07-22 | 273M/week signal | `github.com/postcss/postcss` | SUS (`too-new` latest release heuristic); no postinstall | Flagged — planner must add checkpoint. [VERIFIED: npm registry; package-legitimacy seam; Context7] |
| `@typescript/typescript6@6.0.2` [WARNING: flagged as suspicious — verify before using.] | npm | Package created 2026-04-16; selected patch published 2026-07-06 | 2.9M/week signal | `github.com/microsoft/TypeScript` | SUS (`too-new`); no postinstall | Flagged — planner must add checkpoint; package is explicitly recommended by Microsoft for TS7-side-by-side tooling. [VERIFIED: npm registry; package-legitimacy seam] [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] |

**Packages removed due to [SLOP] verdict:** none. [VERIFIED: package-legitimacy seam]  
**Packages flagged as suspicious [SUS]:** `postcss`, `@typescript/typescript6`; planner inserts `checkpoint:human-verify` before installation. [VERIFIED: package-legitimacy seam]

## Architecture Patterns

### System Architecture Diagram

```text
reviewed tokens.json + components.json + exceptions.json
                 |
                 v
          generate/check scripts
          /        |         \
         v         v          v
generated CSS  generated TS  public barrel/guide parity
     |             |                 |
     +-------------+-----------------+
                   v
          typed primitives/patterns
                   |
                   v
             feature CSS layout
                   |
                   v
        React projection -> Wails command adapters -> Go authority

source CSS/TSX/docs --------> DS001–DS010 checker --------> stable diagnostics / exit 1
seeded browser fixture -----> Playwright Windows Chromium -> screenshots + geometry + a11y
```

The dependency arrows are one-way: feature code consumes generated semantics and public components; it never feeds theme names or shared visual literals back into the design system. [VERIFIED: 13-UI-SPEC.md]

### Recommended Project Structure

```text
frontend/
├── design-system/
│   ├── tokens.json                 # reviewed pure-data authority
│   ├── components.json             # public component/pattern inventory
│   ├── runtime-geometry.json       # typed/setter/range/fallback contract
│   ├── exceptions.json             # exact audited DS exceptions
│   └── schema/                     # JSON-schema-like validation owned by checker
├── scripts/design-system/
│   ├── check.mjs                   # orchestrates DS001–DS010
│   ├── generate.mjs                # deterministic CSS/TS generation
│   ├── css-policy.mjs              # PostCSS-based rules
│   ├── tsx-policy.mjs              # TypeScript 6 AST-based rules
│   └── fixtures/                   # one allowed + forbidden fixture per rule
├── src/design-system/
│   ├── tokens.generated.css
│   ├── tokens.generated.ts
│   ├── index.ts                    # one public barrel
│   ├── primitives/
│   ├── patterns/
│   └── fixtures/DesignSystemGallery.tsx
├── e2e/
│   ├── design-system.visual.spec.ts
│   ├── design-system.a11y.spec.ts
│   └── screenshot.css
└── DESIGN_SYSTEM.md
```

Existing `frontend/src/components/primitives` can be moved once or retained as the implementation directory; the important contract is one public barrel and one inventory, not path churn for its own sake. [VERIFIED: D-05; 13-UI-SPEC.md]

### Pattern 1: Pure-data token authority with deterministic outputs

Use JSON as the reviewed authority so the checker never executes the manifest. Require `schemaVersion`, exhaustive semantic groups, complete `themes.<name>.<light|dark>` faces, foundation tokens, and explicit runtime geometry. Sort object keys and normalize LF on generation so output is byte-stable on Windows and CI. [ASSUMED]

Recommended manifest shape:

```json
{
  "schemaVersion": 1,
  "contract": {
    "surface": ["canvas", "panel", "panel-subdued", "overlay", "control", "selected", "scrim"],
    "text": ["primary", "secondary", "muted", "inverse", "link"]
  },
  "foundation": {
    "space": { "1": "4px", "2": "8px", "4": "16px" },
    "radius": { "sm": "4px", "md": "8px", "lg": "12px", "pill": "999px" }
  },
  "themes": {
    "default": {
      "light": { "surface.canvas": "#E4E0D8" },
      "dark": { "surface.canvas": "#131419" }
    }
  }
}
```

Generate `--ds-*` custom properties and a frozen TypeScript token-name list from this file; DS009 compares every face to `contract`, and DS007 compares generated names, barrel, inventory, and guide markers bidirectionally. [VERIFIED: 13-UI-SPEC.md]

### Pattern 2: Parser-first policy checks

Enumerate files with Node filesystem APIs, normalize diagnostics to repository-relative `/` paths, sort by `(rule, path, line, column, offendingValue)`, and never rely on shell globs. Reject unreadable files, CSS syntax errors, TSX parse diagnostics, symlinks escaping the frontend root, malformed manifests, and unsupported manifest versions. [ASSUMED]

PostCSS owns CSS structure. Regexes may classify a parsed declaration value, but must never be the stylesheet parser. TypeScript 6 owns TSX structure. [CITED: https://github.com/postcss/postcss] [VERIFIED: TypeScript compiler API docs]

### Pattern 3: Exact exception records, not baselines

Each exception should contain:

```json
{
  "id": "DS-EX-001",
  "rule": "DS001",
  "file": "src/components/Desk/Desk.module.css",
  "locator": { "selector": ".faderTrack", "property": "height", "value": "236px" },
  "rationale": "Domain visualization coordinate; not layout spacing.",
  "reviewCondition": { "kind": "value-or-selector-changes" }
}
```

Forbid directory entries, globs, arbitrary regex, line-number-only locators, and DS007/DS008 bypasses. Fingerprint the exact parsed node; if it disappears or changes, DS008 fails as stale rather than silently matching something else. Spacing properties never accept off-grid exceptions. [VERIFIED: D-07; 13-UI-SPEC.md]

### Pattern 4: Public inventory as the parity seam

`components.json` records public name, kind (`primitive`/`pattern`), export path, guide anchor, and contract-test path. DS007 resolves every record and also scans the barrel for unregistered exports; neither docs nor barrel is a hand-maintained second list. [ASSUMED]

### Pattern 5: Static/browser split for DS010

Static DS010 checks `outline: none`/`outline: 0`, raw transitions, status-token use outside StatusBadge/Safety primitives, theme-dependent hiding, and safety selectors with `display:none`, `visibility:hidden`, or zero sizing. Browser DS010 checks actual focus, non-color labels/icons, viewport visibility, target geometry, modal behavior, reduced motion, and degraded-daemon independence. [VERIFIED: 13-UI-SPEC.md] Static analysis alone cannot prove rendered visibility or contrast. [ASSUMED]

## DS001–DS010 Parsing Strategy

| Rule | Deterministic implementation |
|------|------------------------------|
| DS001 | PostCSS declaration walker classifies property families. Permit `0`, approved border/focus/rail primitives only in DS files, semantic `var(--ds-*)`, and exact exceptions. Reject raw colors/named colors, type, spacing, radius, motion, shadow, and z-index literals in feature modules. [VERIFIED: 13-UI-SPEC.md; W3C CSS Syntax] |
| DS002 | Reject `decl.prop.startsWith("--")` outside generated token CSS and registered runtime-geometry setter/definition sites. Current Guided First Show declarations are known findings. [VERIFIED: PostCSS inventory] |
| DS003 | Build a case-sensitive declared-token set from generated CSS and runtime geometry; inspect parsed `var()` calls; reject unknown names and every feature fallback. [CITED: https://www.w3.org/TR/css-variables-1/] |
| DS004 | PostCSS rule selectors reject `data-theme`/`data-theme-name` outside generated theme CSS; TS AST rejects string/property access branches on theme names outside `lib/theme.ts` and generated theme code. [VERIFIED: 13-UI-SPEC.md] |
| DS005 | TSX AST flags lowercase `button/input/select/textarea` with `className` outside public primitive/vendor-specialization files. CSS rules targeting native controls outside DS files are also flagged. Native range/fader, Monaco/Tiptap, and other domain controls require exact exceptions. [ASSUMED] |
| DS006 | Reject feature CSS class names matching controlled concepts (`button`, `field`, `dialog`, `tab`, `toolbar`, `chip|badge`, `empty`, `loading`, `error`, `focus`) when the rule contains shared visual properties. Require the corresponding public import; use held-out fixtures to prevent a “return no findings” implementation. [ASSUMED] |
| DS007 | Compare inventory ↔ barrel exports ↔ contract tests ↔ guide machine markers ↔ generated token-name exports; report both missing and extra records. [VERIFIED: 13-UI-SPEC.md] |
| DS008 | Validate schema, unique IDs, exact in-root file, supported rule, exact parsed locator, nonblank rationale, structured review condition, no broad match, no spacing bypass, no stale/orphaned match. [VERIFIED: 13-UI-SPEC.md] |
| DS009 | Compare every name from `ThemeName`/manifest against exactly two faces (light/dark) and the exhaustive semantic color contract; reject missing and extra keys. `system` is preference resolution, not a third face. [VERIFIED: frontend/src/lib/theme.ts; 13-UI-SPEC.md] |
| DS010 | Combine static focus/motion/status/safety checks with Playwright focus, target, visibility, label/icon, modal, reduced-motion, and degraded-runtime assertions. [VERIFIED: 13-UI-SPEC.md] |

### Pragmatic native-control policy

Do not ban native elements globally; primitives should render native elements. Ban feature-owned visual reinvention. An AST finding requires all of: lowercase control tag, feature file outside approved primitive/specialized boundary, and a styling signal (`className`, inline `style`, or CSS Module selector). [ASSUMED]

This avoids false positives for semantic test scaffolding and keeps native semantics inside public primitives. It still catches the current high-value cases because most of the 181 buttons are feature-styled. [VERIFIED: codebase grep]

## Primitive and Pattern Gap Map

| Required surface | Closest existing analog | Gap to plan |
|------------------|-------------------------|-------------|
| Button | `primitives/Button` | Add trailing icon, compact/default/target sizes, loading semantics, label stability, ref forwarding, and complete active/disabled/loading/focus contract. [VERIFIED: source vs UI-SPEC] |
| IconButton | Button + InfoTooltip | Missing dedicated API, required accessible label, target sizes, and tooltip composition. [VERIFIED: source vs UI-SPEC] |
| Field family | `primitives/Field` | Current wrapper is input-centric and delegates selects; add description/error/required/disabled wiring and text/number/select/textarea/checkbox/switch/range shells. [VERIFIED: source vs UI-SPEC] |
| StatusBadge | `primitives/Chip` | Preserve icon+text tones; add public status name, foreground contract, slots/class policy, and tests for all six statuses. [VERIFIED: source vs UI-SPEC] |
| Panel/PanelHeader/Toolbar/ListRow/ScrollRegion | Existing primitives | Add required variants, heading hierarchy, locked/disabled states, axes/labels, ref/slot APIs, and overflow contracts. [VERIFIED: source vs UI-SPEC] |
| Tabs | Settings mode/theme button groups; source/filter groups | No shared ARIA tab keyboard model. Add once, then migrate only true tab semantics; segmented buttons remain buttons. [VERIFIED: codebase grep; UI-SPEC] |
| Empty/Loading/Error | EmptyState plus many feature-local classes | EmptyState lacks heading/body/action; LoadingState and ErrorState are missing. Consolidate before workspace migration. [VERIFIED: codebase grep] |
| Dialog/ConfirmDialog | ConfirmModal, HelpOverlay, QuickSwitcher, ScriptRunDialog, FixtureStyleModal | Current dialogs are separate custom overlays; ConfirmModal lacks a focus trap and explicit return-focus contract. Create Dialog foundation, then ConfirmDialog; migrate all overlay callers. [VERIFIED: source review] |
| Tooltip | InfoTooltip | Keep portal positioning; add dismissal/escape, ref/ID semantics, and shared stacking token. [VERIFIED: source vs UI-SPEC] |
| ResizeHandle/SplitPane | ResizeHandle + `useResizablePanel` | Add keyboard resizing and make persistence/clamping a typed SplitPane pattern. [VERIFIED: source vs UI-SPEC] |
| WorkspaceFrame | `workspace.module.css` + Toolbar + ScrollRegion | Repeated informal layout exists; make it a typed composition and route every destination/guide equivalent through it. [VERIFIED: WorkspaceRouter; UI-SPEC] |
| CommandRail patterns | CommandRail | Extract typed group/item APIs without changing navigation model. [VERIFIED: source review] |
| DataList/FormActions/ImpactReview | Fixture Library, Project Fixtures, Fixture Patch | Repeated state/actions exist locally; extract compositions while preserving domain command wiring. [VERIFIED: codebase inventory] |
| SceneStack/LauncherMasters/MidiPickup/GuidedFlow/SafetyAction | Existing scene, operator, MIDI, guide, and safety components | Promote proven compositions into documented patterns; do not genericize domain logic or move authority into React. [VERIFIED: source review; AGENTS.md] |

## Don't Hand-Roll

| Problem | Don't build | Use instead | Why |
|---------|-------------|-------------|-----|
| CSS parsing | Comment-stripping + regex parser | Direct-pinned PostCSS | CSS tokenization includes strings, comments, functions, blocks, hashes, and dimensions; parser errors must fail closed. [CITED: https://www.w3.org/TR/css-syntax-3/] |
| TSX parsing | Regex for `<button>`/imports/theme strings | `@typescript/typescript6` compiler API | AST traversal provides actual JSX context and stable positions; TS7 has no stable API. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] |
| Visual diff engine | Custom PNG comparison | Playwright `toHaveScreenshot` | It waits for consecutive stable screenshots and supports animation/caret/style/mask/diff controls. [CITED: https://playwright.dev/docs/test-snapshots] |
| Accessibility tree snapshotting | Custom DOM serializer | Playwright `toMatchAriaSnapshot` plus role/name/focus assertions | The existing runner already exposes accessibility semantics. [CITED: https://playwright.dev/docs/aria-snapshots] |
| Parallel component registry | Separate guide list, barrel list, and checker list | One inventory with bidirectional drift checks | Separate lists are the drift DS007 is meant to prevent. [VERIFIED: 13-UI-SPEC.md] |
| Focus behavior per modal | Per-dialog key listeners and ad hoc autofocus | One Dialog/ConfirmDialog foundation, preferably native `<dialog>` wrapper with explicit tests | Current overlay duplication is already visible in source. [VERIFIED: codebase inventory] [ASSUMED] |
| Policy baseline | Ignore file containing all existing violations | Exact exception manifest with stale-match failure | D-11 forbids permanent ignored debt. [VERIFIED: 13-CONTEXT.md] |

**Key insight:** the checker should be small orchestration over real parsers and explicit data contracts; “dependency-light” means two narrowly justified tooling dependencies, not reimplementing language grammars. [VERIFIED: source research]

## Common Pitfalls

### Pitfall 1: Treating every pixel as spacing

**What goes wrong:** Fader tracks, shell bounds, 1px borders, 2px focus rings, and 3px state rails are either incorrectly rejected or broadly exempted. [VERIFIED: 13-UI-SPEC.md]  
**How to avoid:** Classify by CSS property and ownership. Spacing properties use only the 4px scale; domain sizing uses named sizing tokens or exact exceptions; border/focus/rail primitives live in DS files. [VERIFIED: 13-UI-SPEC.md]  
**Warning sign:** A file-level exception or a rule that allows any `px` value in `width/height`. [ASSUMED]

### Pitfall 2: Migrating tokens without migrating components

**What goes wrong:** Local `.primaryButton`, `.errorText`, `.dialog`, and focus rules survive under new variable names, so visual drift continues. [VERIFIED: current CSS inventory]  
**How to avoid:** DS006 and migration tasks must pair CSS deletion with primitive/pattern adoption. [VERIFIED: D-05; UI-SPEC]  
**Warning sign:** Feature CSS still owns hover/active/disabled/loading/focus declarations. [VERIFIED: UI-SPEC]

### Pitfall 3: Using TypeScript 7 as if it exposed the old compiler API

**What goes wrong:** `import "typescript"` exposes version metadata, not the stable compiler API expected by classic tooling. [VERIFIED: installed `typescript@7.0.2` package exports]  
**How to avoid:** Keep TS7 for compilation and pin `@typescript/typescript6@6.0.2` for the checker until a separately planned migration to the future TS7 API. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/]  
**Warning sign:** Checker imports `typescript/unstable/*`. [VERIFIED: installed package exports]

### Pitfall 4: A cross-platform screenshot baseline

**What goes wrong:** Font rasterization and OS rendering create noise, while the current cross-platform workflow is explicitly observational and nonblocking. [ASSUMED] [VERIFIED: cross-platform-mage.yml]  
**How to avoid:** Keep canonical snapshots in one Windows Chromium project matching v1 qualification; keep semantic/geometry tests portable where useful. [VERIFIED: AGENTS.md]  
**Warning sign:** One snapshot folder is updated independently from Windows/Linux/macOS jobs. [ASSUMED]

### Pitfall 5: Masking unstable UI instead of stabilizing it

**What goes wrong:** Masks hide real regressions in live truth, safety state, or editor chrome. [VERIFIED: UI-SPEC]  
**How to avoid:** Seed data, freeze time/telemetry, load fonts, disable caret/animations, and mask only documented genuinely nondeterministic pixels. [CITED: https://playwright.dev/docs/test-snapshots]  
**Warning sign:** Whole panels or safety controls are masked. [VERIFIED: UI-SPEC]

### Pitfall 6: Breaking safety controls during loading/dialog work

**What goes wrong:** A global disabled/busy overlay intercepts Blackout or Revoke Automation, or a modal visually hides them without preserving priority behavior. [VERIFIED: AGENTS.md; UI-SPEC]  
**How to avoid:** Loading belongs to the initiating action/region. Playwright must exercise normal, busy, error, modal, compact, and daemon-failure states while invoking the existing safety buttons. [VERIFIED: UI-SPEC]  
**Warning sign:** `pointer-events:none` on shell ancestors, shared pending state for all actions, or safety actions moved into a menu/dialog. [ASSUMED]

### Pitfall 7: Letting exceptions become a second baseline

**What goes wrong:** Broad path/regex exceptions stay valid after the original code disappears. [VERIFIED: DS008 contract]  
**How to avoid:** Exact AST/declaration locators, stale-match failure, no globs/directories, structured review conditions, and exception-count review in every plan wave. [ASSUMED]  
**Warning sign:** Exception count only rises. [ASSUMED]

### Pitfall 8: Running the browser matrix in normal `npm run build`

**What goes wrong:** Offline Mage builds become browser-download dependent and slow; the existing config deliberately keeps Playwright separate. [VERIFIED: frontend/playwright.config.ts]  
**How to avoid:** `build` runs static policy + TS + Vitest + Vite. A separate Windows CI/Mage target runs required visual/a11y suites after browser provisioning. [VERIFIED: UI-SPEC; current build architecture]  
**Warning sign:** `npm run build` invokes `playwright install` or `test:e2e`. [VERIFIED: offline build constraints]

## Code Examples

### CSS policy traversal

```javascript
// Source: Context7 /postcss/postcss
import postcss from "postcss";

const root = postcss.parse(cssText, { from: absoluteFile });
root.walkDecls((decl) => {
  const { line, column } = decl.source.start;
  inspectDeclaration({ property: decl.prop, value: decl.value, line, column });
});
root.walkRules((rule) => inspectSelector(rule.selector, rule.source.start));
```

PostCSS `CssSyntaxError` must become a nonzero DS diagnostic; never catch and skip it. [CITED: https://github.com/postcss/postcss/blob/main/docs/guidelines/runner.md]

### TSX native-control traversal

```typescript
// Source: Microsoft TypeScript compiler API linter pattern
import ts from "@typescript/typescript6";

const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
function visit(node: ts.Node): void {
  if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
    inspectJsxElement(node, source);
  }
  ts.forEachChild(node, visit);
}
visit(source);
```

Report positions with `source.getLineAndCharacterOfPosition(node.getStart())`, adding one for human-readable line/column. [CITED: https://github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API]

### Stable screenshot assertion

```typescript
// Source: Playwright official screenshot assertions
await document.fonts.ready;
await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
await expect(page).toHaveScreenshot("shell-dark-1280.png", {
  animations: "disabled",
  caret: "hide",
  scale: "css",
  stylePath: path.join(import.meta.dirname, "screenshot.css"),
});
```

Calibrate and commit one reviewed diff tolerance after repeated Windows CI captures; do not inherit an undocumented default or per-test thresholds. [ASSUMED]

### Accessibility/focus contract

```typescript
// Source: Playwright official locator/ARIA assertions
const dialog = page.getByRole("alertdialog", { name: "Remove mapping?" });
await expect(dialog).toMatchAriaSnapshot({ name: "remove-mapping.aria.yml" });
await expect(page.getByRole("button", { name: "Keep Mapping" })).toBeFocused();
await page.keyboard.press("Escape");
await expect(trigger).toBeFocused();
```

[CITED: https://playwright.dev/docs/aria-snapshots] [CITED: https://playwright.dev/docs/test-assertions]

## CI, Mage, and Package Integration

Recommended package scripts:

```json
{
  "check:design-system": "node scripts/design-system/check.mjs",
  "generate:design-system": "node scripts/design-system/generate.mjs",
  "test:design-system": "vitest run src/design-system scripts/design-system",
  "test:e2e:design-system": "playwright test e2e/design-system.*.spec.ts --workers=1",
  "build": "npm run check:design-system && tsc --noEmit && vitest run && vite build"
}
```

The exact script file globs may differ, but `build` must start with the policy gate and must not run the browser suite. [VERIFIED: UI-SPEC; current package architecture]

Repository integration:

- `mage Build` already invokes `npm run build` through pinned Node whenever frontend source is stale, so DS static/unit gates automatically join the normal build without a second implementation. [VERIFIED: internal/command/build.go]
- Add an explicit registry-backed/Mage Windows UI target that invokes `npm run test:e2e:design-system`; follow the current Mage pattern of delegating through `internal/command`, not launching ad hoc tools from the Mage file. [VERIFIED: magefiles/magefile.go; config/commands.toml]
- Add a required Windows CI job that bootstraps, installs the Playwright Chromium matching the lockfile, runs the design-system browser target, and uploads diff artifacts on failure. Do not add canonical snapshots to `cross-platform-mage.yml`. [VERIFIED: AGENTS.md; current workflows] [ASSUMED]
- Keep the static checker network-free and deterministic after `mage Bootstrap`; neither checking nor generation reads registries or remote URLs. [VERIFIED: offline project constraint]
- Check generated token files in `generate --check` or make `check:design-system` byte-compare regenerated output in memory and print the repair command. Do not silently rewrite during CI. [VERIFIED: UI-SPEC]

## Likely File Plan

| Change group | Likely files |
|--------------|-------------|
| Authority | `frontend/design-system/{tokens,components,runtime-geometry,exceptions}.json`; validation schemas. [ASSUMED] |
| Generator/checker | `frontend/scripts/design-system/*.mjs`, per-rule fixtures/tests. [ASSUMED] |
| Generated surface | `frontend/src/design-system/tokens.generated.{css,ts}`, `frontend/src/design-system/index.ts`. [ASSUMED] |
| Primitive completion | Existing `frontend/src/components/primitives/**` plus new IconButton, Tabs, LoadingState, ErrorState, Dialog/ConfirmDialog. [VERIFIED: UI-SPEC] |
| Patterns | WorkspaceFrame, SplitPane, DataList, FormActions, ImpactReview, and documented product compositions. [VERIFIED: UI-SPEC] |
| Theme migration | `frontend/src/index.css`, `frontend/src/lib/theme.ts`, generated token tests. [VERIFIED: codebase] |
| Shell/safety | `frontend/src/shell/**`, GlobalFrame, SafetyCluster, TempoControls, LiveStatusBar. [VERIFIED: codebase] |
| Workspace waves | `frontend/src/workspaces/**` and colocated domain components/CSS Modules. [VERIFIED: codebase] |
| Guide/docs | `frontend/DESIGN_SYSTEM.md`, frontend README/project instruction link, generated inventory markers. [VERIFIED: UI-SPEC] |
| Browser gates | `frontend/e2e/design-system.*.spec.ts`, deterministic fixture helper/data, screenshot stylesheet/baselines, `playwright.config.ts`. [VERIFIED: UI-SPEC] |
| Root integration | `frontend/package.json`, lockfile, `internal/command/*` UI target/registration, `magefiles/magefile.go`, `.github/workflows/check.yml` or dedicated UI workflow. [VERIFIED: codebase integration points] |

## State of the Art

| Old/current approach | Phase 13 approach | Impact |
|----------------------|-------------------|--------|
| Raw theme palettes and semantic-ish aliases in one 900+ line `index.css` | Pure manifest + generated exhaustive semantic faces | Theme completeness becomes machine-checkable. [VERIFIED: codebase; UI-SPEC] |
| Feature theme-name selectors | Semantic theme tokens only | Removes palette-specific component branching. [VERIFIED: DS004] |
| 12 seed primitives plus local duplicates | Complete typed foundation + product patterns | Shared interaction states and accessibility become reusable contracts. [VERIFIED: UI-SPEC] |
| Regex-friendly manual review | PostCSS + TS6 AST policy checker | Stable syntax-aware diagnostics and fail-closed parsing. [CITED: official parser/compiler docs] |
| Screenshot documentation and geometry tests | Reviewed visual baselines + geometry + ARIA/focus matrix | Adds regression evidence without replacing existing responsive coverage. [VERIFIED: e2e source; UI-SPEC] |
| TypeScript package as historical compiler API | TS7 CLI plus official TS6 compatibility API for tooling | Required because TS7.0 has no stable programmatic API. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/] |

**Deprecated/outdated:**

- Feature consumption of `--page`, `--panel`, `--ink`, `--accent-*`, or theme-specific palette values: confine legacy aliases to generated theme implementation during migration, then remove feature use. [VERIFIED: UI-SPEC]
- `ConfirmModal` as the public name/contract: replace with Dialog + ConfirmDialog contract while preserving call-site compatibility during migration. [VERIFIED: UI-SPEC]
- Ad hoc `outline:none`, local button/field/dialog/state CSS, and arbitrary z-index: migrate to primitives/tokens. [VERIFIED: current inventory; UI-SPEC]
- The assumption that `typescript@7` exposes the classic API: officially false for 7.0. [CITED: https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Native `<dialog>` is the preferred implementation behind the shared Dialog wrapper in the supported WebView2 baseline. | Standard Stack / Don't Hand-Roll | If WebView2 acceptance finds focus/return-focus incompatibility, use one well-tested custom foundation without changing public API. |
| A2 | Pure JSON plus deterministic generation is preferable to an executable TypeScript manifest. | Architecture Patterns | Planner may choose checked bidirectional TS/CSS equivalence, but must preserve fail-closed parsing and no manifest execution. |
| A3 | DS005 flags styled native controls, not every native element. | DS strategy | Too narrow may miss unstyled duplicates; too broad may reject valid primitive implementation/test scaffolding. Held-out fixtures resolve boundary. |
| A4 | Canonical screenshot baselines should run on Windows Chromium only. | Pitfalls / CI | If project policy requires multiple OS baselines, separate snapshot projects/folders are needed. |
| A5 | Screenshot tolerance should be calibrated by repeated Windows CI captures and stored once globally. | Code Examples | Poor calibration either creates noise or hides small regressions. |
| A6 | An explicit registry-backed Mage UI visual target is preferable to adding Playwright to the full cross-platform Node scope. | CI Integration | Requires a small command-registry extension but prevents cross-platform snapshot noise and offline build coupling. |
| A7 | The checker should use sorted Node filesystem enumeration, normalized paths/LF, strict schema/path containment, exact parsed exception locators, and fail-closed syntax handling. | Architecture / DS008 / Security | A weaker implementation can be nondeterministic, silently skip malformed input, or allow broad/stale exceptions. |
| A8 | `components.json` and the proposed `frontend/design-system` / `scripts/design-system` file layout should be the registry/parity seam. | Architecture / Likely File Plan | Different names/paths are safe only if one inventory still drives exports, guide markers, tests, and drift checks. |
| A9 | Static DS010 and DS005/DS006 should use the pragmatic checks described, with held-out fixtures defining false-positive boundaries. | DS001–DS010 strategy | Overbroad name checks can reject valid domain controls; underbroad checks can permit visual reinvention. |
| A10 | The warning signs, per-task/per-wave sampling commands, and “no missing dependency after approval” assessment are planning defaults rather than already-proven implementation facts. | Pitfalls / Environment / Validation | Planner must adjust timings and commands if Wave 0 measurements or package approval differ. |

## Open Questions

1. **Native Dialog acceptance**
   - What we know: the current code has several custom overlays and the UI-SPEC requires one focus-managed Dialog/ConfirmDialog contract. [VERIFIED: codebase; UI-SPEC]
   - What's unclear: exact WebView2 focus restoration/keyboard behavior for the project’s supported runtime build. [ASSUMED]
   - Recommendation: prototype the wrapper in Wave 0 and test open, initial safe focus, Tab containment, Escape, backdrop, nested portal content, and return focus before migrating all dialogs. [ASSUMED]

2. **Reviewed screenshot tolerance**
   - What we know: Playwright supports global threshold/max-diff controls and waits for stable consecutive screenshots. [CITED: https://playwright.dev/docs/test-snapshots]
   - What's unclear: the smallest noise-free threshold on the Windows CI image. [ASSUMED]
   - Recommendation: capture the full matrix three times on the target CI image, diff repeats, select the smallest global tolerance, and record the evidence in the plan/PR. [ASSUMED]

3. **Exception count**
   - What we know: specialized surfaces need narrow geometry/vendor exceptions, while D-11 forbids an ignored debt baseline. [VERIFIED: UI-SPEC]
   - What's unclear: the final count after repeated values become sizing tokens. [VERIFIED: not yet implemented]
   - Recommendation: require each migration plan to state starting/ending exception counts and reject broad file exceptions. [ASSUMED]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Ambient Node | local inspection | ✓, but below project pin | 22.19.0 | Use `mage Bootstrap` pinned Node 24.18.0 for authoritative execution. [VERIFIED: local probe; config/toolchain.toml] |
| npm | frontend scripts | ✓ | 10.9.3 ambient | Pinned Node distribution/npm through bootstrap. [VERIFIED: local probe] |
| Mage | root validation | ✓ | 1.17.2, built with Go 1.26.5 | — [VERIFIED: local probe] |
| Frontend dependencies | unit/build | ✓ | lockfile installed | `mage Bootstrap` / `npm ci` through project workflow. [VERIFIED: `npm list`] |
| Playwright runner | browser gates | ✓ | 1.62.0 | — [VERIFIED: local probe] |
| Playwright Chromium | browser gates | ✓ | cached matching installations present | `npx playwright install chromium` in the dedicated network-enabled CI provisioning step. [VERIFIED: local probe] |
| PostCSS parser | checker | ✓ transitively via Vite | 8.5.22 | Promote exact version to direct dev dependency; do not rely on transitive resolution. [VERIFIED: `npm list postcss`] |
| TypeScript 6 compiler API | TSX checker | ✗ direct package absent | — | Install official `@typescript/typescript6@6.0.2` after human legitimacy checkpoint. [VERIFIED: npm registry; TS7 announcement] |
| Knowledge graph | semantic discovery | ✗ | — | Codebase grep and direct source inspection used. [VERIFIED: `.planning/graphs/graph.json` absent] |

**Missing dependencies with no fallback:** none after the two flagged tooling packages are approved and installed. [ASSUMED]

**Missing dependencies with fallback:** ambient Node mismatch is handled by the repository's pinned bootstrap. [VERIFIED: config/toolchain.toml; internal/bootstrap]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 4.1.10 + jsdom 29.1.1 + Testing Library 16.3.2; Playwright 1.62.0 Chromium. [VERIFIED: package lock] |
| Config file | `frontend/vite.config.ts` for Vitest; `frontend/playwright.config.ts` for browser tests. [VERIFIED: codebase] |
| Quick run command | `cd frontend && npm run check:design-system && npm run test:design-system` (target <30s). [ASSUMED] |
| Full frontend command | `cd frontend && npm run build`. [VERIFIED: package.json, after planned script update] |
| Browser command | `cd frontend && npm run test:e2e:design-system`. [ASSUMED] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-04/D-06 | Every theme face exports exact semantic contract | unit/contract | `npm run test:design-system -- tokens` | ❌ Wave 0 |
| D-05/D-12 | Primitives own variants, semantics, focus/loading/disabled states | component/a11y | `npm run test:design-system -- primitives` | ❌ Wave 0; seed tests exist |
| D-07 | Exceptions exact, valid, non-stale, no spacing bypass | checker fixture | `npm run check:design-system` | ❌ Wave 0 |
| D-08/D-09 | Guide/barrel/tokens/inventory parity and DS001–DS010 | checker fixture | `npm run check:design-system` | ❌ Wave 0 |
| D-10 | Stable light/dark representative visuals | visual | `npm run test:e2e:design-system` | ❌ Wave 0 |
| D-11 | Zero unregistered current violations | static integration | `npm run check:design-system` | ❌ Wave 0 |
| D-13 | Safety controls visible/operable in normal/busy/error/modal/compact/offline states | browser integration | `npm run test:e2e:design-system -- --grep safety` | Partial: helper exists; matrix missing |
| D-14 | UI remains projection-only and command wiring unchanged | unit/integration/regression | `npm test && npm run test:e2e:design-system` | Partial: current component tests exist |
| UI-SPEC | 900×720/1280×720 geometry, dialog focus, light/dark, reduced motion, 200% zoom | browser | `npm run test:e2e:design-system` | Partial: responsive/resize exist; visual/a11y matrix missing |

### Sampling Rate

- **Per task commit:** `npm run check:design-system && npm run test:design-system` plus the touched workspace's existing Vitest file. [ASSUMED]
- **Per wave merge:** `npm run build` and targeted 900×720/1280×720 Playwright states for the migrated slice. [ASSUMED]
- **Phase gate:** full `npm test`, full current Playwright suite, design-system visual/a11y matrix, and root Mage/CI gates green. [VERIFIED: UI-SPEC]

### Wave 0 Gaps

- [ ] Token/component/runtime-geometry/exception manifests and schemas. [VERIFIED: absent]
- [ ] DS001–DS010 checker plus allowed/forbidden fixtures for every rule. [VERIFIED: absent]
- [ ] Generated token CSS/TS and freshness test. [VERIFIED: absent]
- [ ] Public inventory/barrel/guide drift test. [VERIFIED: absent]
- [ ] Primitive state gallery with deterministic fixtures. [VERIFIED: absent]
- [ ] Dialog focus-management browser proof. [VERIFIED: absent]
- [ ] `design-system.visual.spec.ts`, `design-system.a11y.spec.ts`, snapshot stylesheet, and Windows baselines. [VERIFIED: absent]
- [ ] Dedicated Windows Mage/CI browser target with diff artifact upload. [VERIFIED: absent]
- [ ] Exact package pins after required human verification. [VERIFIED: absent]

The existing Vitest suite was green (53 files, 372 tests). The full Playwright suite exceeded a 120-second local research timeout; `--list` found 30 tests, so planning should budget browser work as a separate slower gate rather than a per-task quick check. [VERIFIED: local commands, 2026-08-02]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No authentication behavior changes in this cosmetic/tooling phase. [VERIFIED: phase boundary] |
| V3 Session Management | no | No session behavior changes. [VERIFIED: phase boundary] |
| V4 Access Control | yes, safety authority preservation | Existing Go/Wails authorization and independent safety command paths remain unchanged; React only projects state. [VERIFIED: AGENTS.md] |
| V5 Input Validation | yes | Strict schema validation for tokens/components/exceptions/runtime geometry; PostCSS/TS parsers fail closed; repository-relative path containment. [ASSUMED] |
| V6 Cryptography | no | No cryptographic work. [VERIFIED: phase boundary] |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Executable manifest or dynamic checker config | Tampering / Elevation | Pure JSON data, strict schema, no `eval`, no dynamic imports from manifest paths. [ASSUMED] |
| Path traversal/symlink escape in exception or file enumeration | Tampering / Information disclosure | Resolve against frontend root, reject absolute/`..`/out-of-root paths and escaping symlinks. [ASSUMED] |
| Silent parser skip on malformed CSS/TSX | Tampering | Surface syntax/parse diagnostics and exit nonzero. [VERIFIED: UI-SPEC] |
| Supply-chain compromise in new parser tooling | Tampering | Exact pins/lockfile, package legitimacy checkpoint, official repositories, no postinstall scripts, offline checks after bootstrap. [VERIFIED: package audit; AGENTS.md] |
| UI overlay or loading state intercepts safety actions | Denial of service | Region-scoped busy states and browser tests that invoke Blackout/Revoke in modal/offline/busy states. [VERIFIED: UI-SPEC] |
| Theme lowers status/focus contrast | Spoofing | Per-face token completeness/contrast tests plus text/icon status signals. [VERIFIED: UI-SPEC] |
| Visual test masks safety regressions | Repudiation / Spoofing | Prohibit safety-control masks and upload actual/diff images on failure. [ASSUMED] |

## Sources

### Primary (HIGH confidence)

- `13-CONTEXT.md` and approved `13-UI-SPEC.md` — locked decisions, component/token/checker/visual acceptance. [VERIFIED: local source]
- `AGENTS.md`, brand guidelines/tokens, sketch skill/references — project, safety, visual, and workflow constraints. [VERIFIED: local source]
- Frontend source, CSS Modules, package lock, Vitest/Playwright configs, Mage commands, and GitHub workflows — current architecture and measured inventory. [VERIFIED: codebase inspection]
- [Microsoft TypeScript 7.0 announcement](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/) — no stable TS7 API and official TS6 compatibility package.
- [W3C CSS Syntax Level 3](https://www.w3.org/TR/css-syntax-3/) and [CSS Custom Properties](https://www.w3.org/TR/css-variables-1/) — parsing/token/custom-property behavior.

### Secondary (MEDIUM confidence)

- Context7 `/postcss/postcss` — parse/walk/source/CssSyntaxError APIs.
- Context7 `/microsoft/typescript` — classic compiler API traversal/source-position pattern, applied through the official TS6 compatibility package.
- Context7 `/microsoft/playwright/v1.61.0` — screenshot, ARIA, role/name, focus, and reduced-motion APIs; installed version is 1.62.0. [VERIFIED: package lock]
- Context7 `/vitest-dev/vitest/v4.1.6` — jsdom configuration and simulated-browser limitation; installed version is 4.1.10. [VERIFIED: package lock]
- npm registry and GSD package-legitimacy seam — selected parser package versions, repositories, download signals, postinstall status, and SUS verdicts.

### Tertiary (LOW confidence)

- Assumptions A1–A6 above; each has a Wave 0 validation or explicit planning checkpoint.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — existing stack and versions inspected; two additions verified through official sources, registry, and legitimacy gate.
- Architecture: HIGH — prescribed by approved UI-SPEC and grounded in current repository seams.
- Inventory: HIGH — parsed/grepped from the live workspace on 2026-08-02; counts may change with concurrent user edits before execution.
- Checker details: MEDIUM — parser APIs are verified; exact policy boundaries require held-out fixture calibration.
- Visual strategy: HIGH — Playwright APIs and existing harness are verified; numeric diff tolerance remains an explicit Wave 0 decision.
- Pitfalls: HIGH — derived from locked safety constraints, current duplication, and verified tool limitations.

**Research date:** 2026-08-02  
**Valid until:** 2026-08-09 (fast-moving TypeScript 7/PostCSS/Playwright tooling and concurrent frontend edits)
