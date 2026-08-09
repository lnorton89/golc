# GOLC Design System

The GOLC UI is a dense Paper/Ink operator console. Import reusable UI only from `src/design-system`; feature code owns domain state, Wails calls, and layout unique to the feature.

## Philosophy

This is an **operator console**, not a marketing site or a typical SaaS dashboard. The visual language — "Paper/Ink" — is deliberately dense, high-contrast, and unadorned: flat surfaces, sharp text, a strict 4px spacing grid, and no decorative motion. Every design decision here optimizes for a person running a live show under time pressure, often on a second monitor, sometimes on a small controller laptop screen — not for first-impression polish.

Two consequences follow directly from that:

1. **Consistency is a safety property, not a cosmetic one.** An operator who has learned where the Blackout control sits, what a disabled button looks like, or how a confirm dialog behaves should never have that expectation violated by one workspace styling itself differently from the rest. This is why the rules below are mechanically enforced (`npm run check:design-system`) rather than left as guidelines.
2. **The UI is a projection, not the authority.** Nothing here is "real" in the sense that matters for a lighting console — the Go daemon owns playback timing, safety state, and Art-Net output. See the frontend [README's Architecture section](./README.md#architecture) for how that boundary is enforced in code.

## Selection rules

- Use a primitive for one native control or bounded surface. Use a pattern for repeated workspace composition.
- Use semantic `--ds-*` tokens only: surfaces, text, borders, actions, statuses, typography, spacing, radii, sizing, focus, motion, and stacking. Raw palette values belong solely to approved theme/token files.
- Use `DataList` for zero/one/many plus busy/error states, `ImpactReview` for preview-before-commit effects, `GuidedFlow` for ordered onboarding, and `SafetyAction` only as presentation around an independently-owned safety command.
- Use `Button` or `IconButton`; do not restyle native controls to duplicate a primitive. Every interactive primitive retains hover, active, disabled, loading, and `:focus-visible` semantics.
- Use `Field` for a labeled text/number/select control; use `NumberStepper` instead of `Field` only when a compact pointer +/-1 nudge affordance is needed alongside typing and native keyboard stepping.

## Token architecture

Everything visual is generated from `design-system/tokens.json`, a single source of truth with four layers:

```
semanticRoles   →  the named roles every token maps to (surface, text, border, action, status, ...)
foundation      →  non-color scales shared by every theme: typography, spacing, radii, sizing, focus, motion, stacking
palettes        →  the two base value sets: "paper-ink-light" and "paper-ink-dark"
themes          →  24 named theme-face entries: 12 names (default, gruvbox, tokyo, dracula, nord,
                    catppuccin, solarized, one-dark, rose-pine, everforest, rainbow, acid) × 2 modes
                    (light/dark), each entry pointing at one of the two base palettes above
```

`npm run generate:design-system` compiles this into `src/design-system/tokens.generated.css` (the real `--ds-*` custom properties) and `tokens.generated.ts` (typed theme-name unions consumed by `src/lib/theme.ts`). Component code never references a palette or theme name directly — it references a semantic token (`var(--ds-surface-panel)`, `var(--ds-text-muted)`, `var(--ds-spacing-space3)`), and the active `data-theme`/`data-theme-name` attributes on `<html>` resolve it to the right concrete value. Today every one of the 12 names resolves to the same `paper-ink-light`/`paper-ink-dark` palette pair (see the `palette` field on each entry in `themes`) — the names are selectable and wired end-to-end, but they don't yet carry distinct color values of their own. This is what makes wiring up a new theme "free" for every component that follows the rules: there is no per-theme component code to write or maintain, only a new palette entry.

`design-system/runtime-geometry.json` is the one deliberate escape hatch from pure static tokens: a small, explicitly-bounded set of CSS custom properties that must be set at *runtime* rather than compile time (e.g. a resizable rail's current width). Each entry declares its own minimum/maximum/fallback and the exact component responsible for setting it — nothing here is a free-form custom property.

## Product boundaries (D-01 through D-14)

D-01 preserves Paper/Ink; D-02 migrates reachable UI; D-03 retains dense 4px-grid operation; D-04 requires semantic tokens; D-05 shares typed primitives/patterns; D-06 forbids theme-name branches; D-07 records only exact audited exceptions; D-08 documents selection; D-09 enforces drift checks; D-10 layers static, unit, accessibility, and browser checks; D-11 starts green; D-12 requires accessible interaction states; D-13 preserves Blackout and Revoke Automation visibility and independence; D-14 keeps React a projection of Go-owned state.

## Canonical examples

```tsx
import { DataList, ImpactReview, Button } from "../design-system";

<DataList label="Fixtures" items={fixtures} busy={loading} error={error} />
<ImpactReview summary="Review before applying." impacts={impacts}>
  <Button variant="primary">Apply plan</Button>
</ImpactReview>
```

## Anti-patterns and exceptions

Do not use raw colors, local token namespaces, theme-name conditionals, custom `z-index`, ad-hoc button/input/dialog/list state styling, or a UI callback as the authority for Art-Net, playback, scripts, or safety operations. A necessary geometry or vendor exception must be an exact record in `design-system/exceptions.json` with its rule, file, rationale, and review condition; it can never bypass the 4px spacing grid.

### The exception schema

Every record in `design-system/exceptions.json` (and any per-domain `exception-proposals/*.json` file merged into it) is validated structurally before it can suppress anything:

| Field | Requirement |
|---|---|
| `path` | The exact source file the exception applies to |
| `rule` | One of `DS001`–`DS010` |
| `match` | The exact diagnostic value being excepted — no `*`, `?`, or newlines allowed (never a wildcard/pattern) |
| `rationale` | A non-empty explanation of *why* this is a legitimate, irreducible exception |
| `reviewCondition` | What future change should trigger re-reviewing this exception |
| `owner` / `source` | Attribution for who introduced it and where |

A few hard rules the checker enforces mechanically, not just by convention:

- **No spacing exceptions, ever.** A `DS001` exception whose matched value contains `padding`, `margin`, `gap`, or `space` is rejected outright — the 4px grid has zero escape hatches.
- **Exact-match-first.** When more than one diagnostic in a file shares the same rule, the checker tries exact `value` equality against each exception before falling back to substring containment, and the fallback is scoped per-diagnostic — one exception can never accidentally absorb an unrelated diagnostic elsewhere in the same file.
- **Fail loud on staleness.** An exception whose matched diagnostic no longer exists (the code was fixed, or refactored away) fails the checker as a "stale exception" — exceptions are actively re-verified on every run, not write-once-forget.

## Enforcement rule reference

`scripts/design-system/check.mjs` is the single static policy checker. Every rule below is enforced against real CSS/TSX source, not just documented as a convention:

| Rule | Checks |
|---|---|
| `DS001` | Raw visual literal — a hard-coded color/size/etc. in a visually-relevant CSS property instead of a `--ds-*` token |
| `DS002` | A new custom CSS property declared outside the design-system's own token files (no local, ad-hoc token namespaces) |
| `DS003` | An undeclared or unknown `--ds-*`-looking custom property reference, or a `var()` fallback used outside the design-system itself |
| `DS004` | A theme-name branch — CSS selectors like `[data-theme=...]`, or TSX code branching on a theme-name string, outside the design-system's own theme-resolution code |
| `DS005` | A native `<button>`/`<input>`/`<select>`/`<textarea>` styled directly instead of using the `Button`/`Field` primitive |
| `DS006` | Feature code reinventing a shared primitive's own visual class (button/field/dialog chrome duplicated ad hoc) |
| `DS007` | Inventory parity — `design-system/components.json`, the public barrel (`src/design-system/index.ts`), and this guide's own anchors/tables must all agree |
| `DS008` | Exception record validity — schema, staleness, and broad-match checks on `design-system/exceptions.json` |
| `DS009` | Theme contract validity — `design-system/tokens.json`'s own structural integrity |
| `DS010` | Accessibility/safety invariants: a safety-related selector (Blackout/Revoke Automation/etc.) can never be hidden via `display:none`/`visibility:hidden`/zero size; `outline` can never be removed without a token-driven replacement; `transition` must use a `--ds-motion-*` token, never a raw duration |

## Enforcement pipeline

```bash
npm run generate:design-system          # Regenerate tokens.generated.{css,ts} from tokens.json
npm run check:design-system              # Run every DS rule against the whole source tree
node scripts/design-system/check.mjs --rule DS007     # Run one rule in isolation
node scripts/design-system/check.mjs --paths <a,b>     # Scope a run to specific files (fast local loop)
```

`check.mjs` supports whole-source (`--all`), rule-scoped (`--rule`), and path-scoped (`--paths`) invocations — the last is what a focused PR's own verify step typically uses, while `--all` is what CI and full local acceptance run. The checker must always leave the tree unchanged (it never auto-fixes) and reports every diagnostic with an exact rule, file, line, and value.

Beyond the static checker, `e2e/design-system.*.spec.ts` covers what static analysis can't: real-browser visual regression (pixel-diff against calibrated `*-win32.png` baselines, tolerance from `design-system/screenshot-tolerance.json`), theme-face contrast (WCAG-verified across all 24 themes, `src/design-system/theme-contrast.test.ts`), startup/error-boundary/offline-safety backstops, and text-zoom/expanded-copy reflow. These run via `npm run test:e2e:design-system` and are part of the required Windows CI workflow (`.github/workflows/design-system.yml`) — visual baselines are calibrated against GitHub's own hosted `windows-latest` runner, so a small local pixel-diff against a different machine's Chromium build is expected environmental noise, not necessarily a regression; the CI run is the canonical proof.

## New-component checklist

1. Prefer an inventory component; otherwise establish a reusable semantic need.
2. Use native semantics and focus-visible, disabled, loading, selected, busy/error, and long-copy states as applicable.
3. Add tests and a deterministic gallery fixture state.
4. Add one `components.json` record and regenerate/check its public surface.
5. Run the exact commands below before handing off.

## Public inventory

<!-- DESIGN-SYSTEM-INVENTORY:START generated by npm run generate:design-system -->
| Name | Kind | Export path | Contract test |
| --- | --- | --- | --- |
| Button | primitive | `src/components/primitives/Button/Button.tsx` | `src/components/primitives/Button/Button.test.tsx` |
| Checkbox | primitive | `src/components/primitives/Checkbox/Checkbox.tsx` | `src/components/primitives/Checkbox/Checkbox.test.tsx` |
| Chip | primitive | `src/components/primitives/Chip/Chip.tsx` | `src/components/primitives/Chip/Chip.test.tsx` |
| ColorField | primitive | `src/components/primitives/ColorField/ColorField.tsx` | `src/components/primitives/ColorField/ColorField.test.tsx` |
| Combobox | primitive | `src/components/primitives/Combobox/Combobox.tsx` | `src/components/primitives/Combobox/Combobox.test.tsx` |
| ConfirmDialog | primitive | `src/components/primitives/ConfirmDialog/ConfirmDialog.tsx` | `src/components/primitives/ConfirmDialog/ConfirmDialog.test.tsx` |
| DataList | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| Dialog | primitive | `src/components/primitives/Dialog/Dialog.tsx` | `src/components/primitives/Dialog/Dialog.test.tsx` |
| EmptyState | primitive | `src/components/primitives/EmptyState/EmptyState.tsx` | `src/components/primitives/EmptyState/EmptyState.test.tsx` |
| ErrorState | primitive | `src/components/primitives/ErrorState/ErrorState.tsx` | `src/components/primitives/ErrorState/ErrorState.test.tsx` |
| Field | primitive | `src/components/primitives/Field/Field.tsx` | `src/components/primitives/Field/Field.test.tsx` |
| FormActions | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| GuidedFlow | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| IconButton | primitive | `src/components/primitives/IconButton/IconButton.tsx` | `src/components/primitives/IconButton/IconButton.test.tsx` |
| ImpactReview | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| InfoTooltip | primitive | `src/components/primitives/InfoTooltip/InfoTooltip.tsx` | `src/components/primitives/InfoTooltip/InfoTooltip.test.tsx` |
| LauncherMasters | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| ListRow | primitive | `src/components/primitives/ListRow/ListRow.tsx` | `src/components/primitives/ListRow/ListRow.test.tsx` |
| LoadingState | primitive | `src/components/primitives/LoadingState/LoadingState.tsx` | `src/components/primitives/LoadingState/LoadingState.test.tsx` |
| Menu | primitive | `src/components/primitives/Menu/Menu.tsx` | `src/components/primitives/Menu/Menu.test.tsx` |
| MidiPickup | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| NumberStepper | primitive | `src/components/primitives/NumberStepper/NumberStepper.tsx` | `src/components/primitives/NumberStepper/NumberStepper.test.tsx` |
| Panel | primitive | `src/components/primitives/Panel/Panel.tsx` | `src/components/primitives/Panel/Panel.test.tsx` |
| PanelHeader | primitive | `src/components/primitives/PanelHeader/PanelHeader.tsx` | `src/components/primitives/PanelHeader/PanelHeader.test.tsx` |
| Popover | primitive | `src/components/primitives/Popover/Popover.tsx` | `src/components/primitives/Popover/Popover.test.tsx` |
| RadioGroup | primitive | `src/components/primitives/RadioGroup/RadioGroup.tsx` | `src/components/primitives/RadioGroup/RadioGroup.test.tsx` |
| ResizeHandle | primitive | `src/components/primitives/ResizeHandle/ResizeHandle.tsx` | `src/components/primitives/ResizeHandle/ResizeHandle.test.tsx` |
| SafetyAction | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| SceneStack | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| ScrollRegion | primitive | `src/components/primitives/ScrollRegion/ScrollRegion.tsx` | `src/components/primitives/ScrollRegion/ScrollRegion.test.tsx` |
| Select | primitive | `src/components/primitives/Select/Select.tsx` | `src/components/primitives/Select/Select.test.tsx` |
| Slider | primitive | `src/components/primitives/Slider/Slider.tsx` | `src/components/primitives/Slider/Slider.test.tsx` |
| SplitPane | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
| Switch | primitive | `src/components/primitives/Switch/Switch.tsx` | `src/components/primitives/Switch/Switch.test.tsx` |
| Tabs | primitive | `src/components/primitives/Tabs/Tabs.tsx` | `src/components/primitives/Tabs/Tabs.test.tsx` |
| ToggleGroup | primitive | `src/components/primitives/ToggleGroup/ToggleGroup.tsx` | `src/components/primitives/ToggleGroup/ToggleGroup.test.tsx` |
| Toolbar | primitive | `src/components/primitives/Toolbar/Toolbar.tsx` | `src/components/primitives/Toolbar/Toolbar.test.tsx` |
| WorkspaceFrame | pattern | `src/design-system/patterns/index.tsx` | `src/design-system/fixtures/DesignSystemGallery.test.tsx` |
<!-- DESIGN-SYSTEM-INVENTORY:END -->

### Component anchors

The inventory anchors are stable selection markers: #button #checkbox #chip #colorfield #combobox #confirmdialog #datalist #dialog #emptystate #errorstate #field #formactions #guidedflow #iconbutton #impactreview #infotooltip #launchermasters #listrow #loadingstate #menu #midipickup #numberstepper #panel #panelheader #popover #radiogroup #resizehandle #safetyaction #scenestack #scrollregion #select #slider #splitpane #switch #tabs #togglegroup #toolbar #workspaceframe.

## Commands

```sh
npm run generate:design-system
npx vitest run src/design-system
node scripts/design-system/check.mjs --rule DS007
```

`npm run generate:design-system` repairs generated artifacts. The check command must leave the tree unchanged and reports inventory drift for repair.
