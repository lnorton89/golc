# Phase 13: Unified UI Design System and Automated Enforcement - Context

**Gathered:** 2026-08-02
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase turns GOLC's existing Paper/Ink visual direction, token seed, and small primitive library into one documented, reusable, and mechanically enforced frontend design system. It migrates the current desktop UI to that system and adds automated checks that prevent future agents from introducing unapproved visual values, duplicate primitives, inconsistent interaction states, or undocumented exceptions.

</domain>

<decisions>
## Implementation Decisions

### Visual Direction and Scope
- **D-01:** Preserve the approved GOLC Paper/Ink console language and existing theme choices. This is a unification phase, not a rebrand.
- **D-02:** Migrate every currently reachable desktop workspace, shell surface, dialog, editor-adjacent control, and reusable component in this phase. Do not leave a permanent split between "new-system" and legacy UI.
- **D-03:** Keep the dense, instrument-like desktop character: compact operational controls, bounded scroll regions, restrained motion, visible focus, and persistent operator safety controls.

### Token and Component Contract
- **D-04:** Components consume semantic design tokens for surfaces, text, borders, actions, statuses, typography, spacing, radii, sizing, focus, and motion. Raw palette values remain confined to the theme/token definitions.
- **D-05:** Shared visual behavior belongs in typed React primitives and patterns. Feature CSS Modules may own feature layout and exceptional domain visualization, but may not reinvent common buttons, fields, panels, dialogs, rows, badges, tabs, toolbars, empty/loading/error states, or focus behavior.
- **D-06:** Theme variants must implement the same semantic contract. Feature code cannot branch on a theme name or read theme-specific palette values directly.
- **D-07:** Any necessary exception is declared in a small audited exception manifest with file, rule, rationale, and review condition. Inline suppression without a recorded exception is not allowed.

### Agent Guidance and Enforcement
- **D-08:** Ship a concise design-system guide with component selection rules, token vocabulary, examples, anti-patterns, and a new-component checklist so coding agents have an authoritative path instead of guessing.
- **D-09:** Enforcement runs in normal frontend validation and CI. It must detect raw visual literals outside approved token/theme files, undeclared CSS custom properties, forbidden native control reinvention where a primitive exists, and drift between the documented and exported design-system surface.
- **D-10:** Use layered verification: fast static policy checks, unit/contract tests for primitives and tokens, accessibility interaction checks, and stable Playwright visual coverage for representative shell/workspace states in light and dark modes.
- **D-11:** New enforcement begins green. Existing violations are migrated or explicitly registered; the project does not adopt a permanently ignored baseline that allows known drift to grow.

### Accessibility and Safety
- **D-12:** Every interactive primitive owns consistent hover, active, disabled, loading, and `:focus-visible` states and exposes native semantics. Color is never the only status signal.
- **D-13:** The design system must not weaken the persistent visibility, priority, or independent behavior of Blackout and Revoke Automation.
- **D-14:** Cosmetic UI work remains a projection of Go-owned state and cannot introduce playback, Art-Net timing, script, or safety authority into React.

### Claude's Discretion
Exact file decomposition, checker implementation, component API names, and migration order are left to research and planning, provided the decisions above remain mechanically testable.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Brand and validated interaction direction
- `.planning/brand/GOLC-Brand-Guidelines.md` — canonical color, typography, state, voice, iconography, motion, and operator-authority rules.
- `.planning/brand/GOLC-Brand-Tokens.md` — canonical base palette and timing values.
- `.planning/sketches/SKILL.md` — validated desktop design direction and implementation sequence.
- `.planning/sketches/references/application-shell-navigation.md` — focused command-rail shell and persistent frame contract.
- `.planning/sketches/references/programming-scene-authoring.md` — scene-authoring density and inspector contract.
- `.planning/sketches/references/live-operation-safety-midi.md` — live operation, safety, and MIDI interaction contract.
- `.planning/sketches/references/onboarding-readiness-impact.md` — guided flow and evidence-state contract.

### Existing phase contracts
- `.planning/phases/06-wails-authoring-and-operator-surface/06-UI-SPEC.md` — original desktop UI contract and token/primitives direction.
- `.planning/phases/09-front-door-ui-completion/09-UI-SPEC.md` — most recent UI additions and approved exception precedent.
- `.planning/phases/09-front-door-ui-completion/09-CONTEXT.md` — locked front-door workspace and guide decisions.
- `.planning/ROADMAP.md` — Phase 13 boundary and project phase dependencies.
- `.planning/PROJECT.md` — live reliability, control consistency, and autonomy-safety constraints.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/index.css` — current global theme variables and palette implementations; seed for a clearer token layer.
- `frontend/src/components/primitives/` — existing Button, Chip, ConfirmModal, EmptyState, Field, InfoTooltip, ListRow, Panel, PanelHeader, ResizeHandle, ScrollRegion, and Toolbar primitives.
- `frontend/src/lib/theme.ts` — existing theme selection and persistence.
- `frontend/e2e/` — current Playwright harness, responsive tests, resize tests, and screenshot tooling.

### Established Patterns
- React components use colocated CSS Modules; global CSS provides theme variables and base element behavior.
- Vitest and Testing Library cover components; Playwright covers desktop browser behavior and documentation screenshots.
- Archivo is the UI font and JetBrains Mono is reserved for technical values.
- Feature workspaces already share some primitives, but many styles and native controls still encode visual behavior independently.

### Integration Points
- `frontend/package.json` is the normal validation entrypoint and must expose a design-system check included by `build`.
- `.github/workflows/` and the repository's Mage validation graph are the CI integration points.
- `frontend/src/components/primitives/` is the public implementation surface; a single barrel/export contract and guide should make allowed building blocks discoverable.
- All `frontend/src/**/*.module.css` and TSX call sites are in migration scope, excluding deliberately domain-specific canvases only when covered by the exception mechanism.

</code_context>

<specifics>
## Specific Ideas

- Treat the current tokens and primitives as a useful seed, not evidence that the system is complete.
- Prefer a dependency-light policy checker that produces actionable file/line diagnostics and can be run by agents before they finish.
- Make the design system self-explaining through canonical examples and tests, so a new agent can copy a good pattern and receive an immediate failure for drift.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 13-Unified UI Design System and Automated Enforcement*
*Context gathered: 2026-08-02*
