---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "29"
subsystem: shell-hotkey-ui
tags: [react, design-system, hotkeys, keyboard-shortcuts, workspace-chrome, empty-state]
requires:
  - phase: 13-02
    provides: DS001-DS010 policy checker
  - phase: 13-06
    provides: Packaged-proven Dialog primitive
  - phase: 13-07
    provides: Typed primitives/patterns and public barrel
provides:
  - Fully token-based HotkeySettings (Settings > Hotkeys rebind UI) using Button/IconButton
  - Fully token-based KeyboardShortcuts reference panel using Panel/ScrollRegion, plus its first test file
  - In-place token migration of the shared workspace.module.css (.workspace/.canvas contract unchanged)
  - ComingSoon stub migrated onto WorkspaceFrame + ScrollRegion + EmptyState
affects: [13-16 (shell overlays/HelpOverlay), any future plan adding a new ComingSoon-style stub destination]
tech-stack:
  added: []
  patterns:
    - "A rebindable key/chord control (HotkeySettings' keyButton) is the shared Button primitive with variant swapped secondary/primary to represent an active 'recording' state, instead of a bespoke accent-tinted CSS class -- reuses Button's existing hover/focus/active semantics for free."
    - "A read-only kbd-styled hint chip (HotkeySettings' fixedKey, KeyboardShortcuts' keys) stays a small custom domain class (not a shared primitive) since it's inert reference text, not an interactive control -- longhand border-width/border-style/border-color split to satisfy DS001's single-var()-per-declaration rule."
    - "A ComingSoon-style stub composes WorkspaceFrame + ScrollRegion(canvas) + EmptyState(heading/body) exactly like every already-migrated *Workspace.tsx, rather than a bespoke Toolbar+Panel pairing -- a future destination graduating from the stub inherits the same shell shape its replacement will already use."
    - "workspace.module.css is migrated in place (token swap only, .workspace/.canvas selectors and shape unchanged) because ~20 other *Workspace.tsx wrapper files still import it directly; only components that have individually adopted WorkspaceFrame stop referencing it."
key-files:
  modified:
    - frontend/src/components/HotkeySettings/HotkeySettings.tsx
    - frontend/src/components/HotkeySettings/HotkeySettings.module.css
    - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx
    - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.module.css
    - frontend/src/workspaces/workspace.module.css
    - frontend/src/workspaces/ComingSoon.tsx
    - frontend/src/workspaces/ComingSoon.module.css
    - frontend/src/workspaces/ComingSoon.test.tsx
  created:
    - frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.test.tsx
    - frontend/design-system/exception-proposals/shell-overlays.json
key-decisions:
  - "HotkeySettings' rebind buttons (.keyButton) became the shared Button primitive with variant={isRecording ? \"primary\" : \"secondary\"} instead of a custom .recording CSS class -- eliminates the need for any custom hover/focus/active CSS on this control entirely, and the class name previously used (\"keyButton\") would have collided with DS006's CONTROLLED_CLASS regex (contains \"button\") the moment it carried any shared visual property, which the original pre-migration CSS already did (padding/border/color/etc.)."
  - "Reset-to-default control became IconButton with native (not soft) disabled behavior -- the original CSS fully hid the button (visibility:hidden) until a binding was customized; switched to the standard dimmed-but-present disabled convention every other IconButton in the app already uses, per the plan's own instruction to 'adopt packaged-proven ... shared states.' Functional behavior (unclickable until customized) is unchanged; only the disabled visual affordance is now consistent with the rest of the app instead of a bespoke hide."
  - "Dropped ComingSoon's `icon` prop entirely rather than extending WorkspaceFrame to accept one: grepped the whole frontend for every current `<ComingSoon .../>` call site (there are none left in production routes -- Fixture Library, Save & Recovery, Overview, and Diagnostics have all since graduated to real workspaces) and every existing WorkspaceFrame consumer (Settings, Notes, Overview, Save & Recovery, Shows, Scenes & Looks, Patch & Pools, Project Fixtures) already omits an icon slot, so extending the shared pattern for a currently-unused prop would be scope creep, not unification."
  - "KeyboardShortcuts' Panel padding intentionally shrinks from the original bespoke 24px (--space-lg) to Panel's own generated default of 16px (--ds-spacing-space4) rather than fighting CSS-Modules class-order to force an override -- a deliberate, minor visual simplification adopting the shared primitive's own default instead of re-deriving a one-off value; not a behavior change (must_haves only requires the panel remain bounded/scrolling, which it still does)."
  - "groupHeading's line-height (originally a bespoke 1.4, with no equivalent generated token) moved to the real --ds-typography-line-height-body token (1.5) rather than an exception -- DS001 forbids raw line-height literals categorically and the 0.1 difference is imperceptible at an 11px all-caps label."
patterns-established: []
requirements-completed: [D-02, D-03, D-04, D-05, D-07, D-11, D-12, D-14, UI-SPEC-SHELL, UI-CONSIDERATIONS]
coverage:
  - id: D1
    description: "HotkeySettings and KeyboardShortcuts fully consume shared design-system primitives/tokens; hotkey editing/help, conflict messaging, and long-copy handling are unchanged"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "frontend/src/components/HotkeySettings/HotkeySettings.test.tsx (14 tests, unchanged from pre-migration): rebind/reset/reject-conflict/escape-cancel for both playback and navigation bindings"
        status: pass
      - kind: unit
        ref: "frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.test.tsx (4 new tests): heading/region, category grouping incl. fixed scene-switch entry, live playback-binding reflection, live nav-chord reflection"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/components/HotkeySettings/HotkeySettings.tsx,src/components/HotkeySettings/HotkeySettings.module.css,src/components/KeyboardShortcuts/KeyboardShortcuts.tsx,src/components/KeyboardShortcuts/KeyboardShortcuts.module.css"
        status: pass
    human_judgment: false
  - id: D2
    description: "Shared workspace.module.css keeps its exact .workspace/.canvas contract while migrating to generated tokens; ComingSoon adopts WorkspaceFrame/ScrollRegion/EmptyState with bounded canvas scrolling and unchanged empty-destination text"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "frontend/src/workspaces/ComingSoon.test.tsx (2 tests): renders title/description/cliHint verbatim; labels the workspace as a named region"
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --paths src/workspaces/workspace.module.css,src/workspaces/ComingSoon.tsx,src/workspaces/ComingSoon.module.css --proposal design-system/exception-proposals/shell-overlays.json"
        status: pass
    human_judgment: false
  - id: D3
    description: "Zero exceptions needed across all eleven owned files; full frontend suite and build gate remain green"
    requirement: "D-07"
    verification:
      - kind: unit
        ref: "cd frontend && npx vitest run (full suite): 481/481 pass"
        status: pass
      - kind: other
        ref: "cd frontend && npm run build (tsc --noEmit && vitest run && vite build): clean"
        status: pass
    human_judgment: false
metrics:
  duration: unavailable (start time not captured at session start)
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 29: Hotkey Settings, Keyboard Shortcuts, and Shared Workspace Chrome Summary

**HotkeySettings.tsx and KeyboardShortcuts.tsx now fully consume Button/IconButton/Panel/ScrollRegion and generated `--ds-*` tokens (closing the last two live consumers of the legacy `--space-*`/`--ink`/`--muted`/`--line`/`--page` tokens Plan 13-08 removed from `index.css`); workspace.module.css is migrated in place preserving its shared `.workspace`/`.canvas` contract, and the ComingSoon stub now composes `WorkspaceFrame`/`ScrollRegion`/`EmptyState` like every other migrated workspace — zero design-system exceptions needed across all eleven owned files.**

## Performance

- **Tasks:** 2/2 complete
- **Scoped design-system check:** passes with zero diagnostics across all eleven owned files combined
- **Focused tests:** 19/19 pass (HotkeySettings 14, KeyboardShortcuts 4 new, plus the pre-existing suite counts unchanged) + ComingSoon 2/2; full frontend suite 481/481 pass; `tsc --noEmit` clean; `npm run build` (tsc + vitest + vite build) clean

## Accomplishments

- Converted HotkeySettings.tsx's rebind key/chord controls to the shared `Button` primitive (variant swaps secondary/primary to represent the "recording" state) and its reset-to-default controls to `IconButton`, eliminating every raw `<button className=...>` in the file.
- Rewrote HotkeySettings.module.css end to end onto generated `--ds-*` tokens: spacing (`--ds-spacing-space1/2/4`), typography (`--ds-typography-font-*`), surfaces/borders (`--ds-surface-canvas`, `--ds-border-default`), and the conflict message's `--ds-status-revoked`. No exceptions registered.
- Converted KeyboardShortcuts.tsx's outer container and scrollable body to the shared `Panel` and `ScrollRegion` primitives, replacing a hand-rolled bordered div with custom `max-height`/`overflow-y`. Wrote its first test file (`KeyboardShortcuts.test.tsx`, 4 tests) — this component had no test coverage before this plan.
- Migrated `workspace.module.css` in place: its single legacy `--space-md` reference became `--ds-spacing-space4`, with the `.workspace`/`.canvas` class names and structure completely unchanged (the ~20 other `*Workspace.tsx` wrapper files that still import this shared file directly needed no changes).
- Migrated `ComingSoon.tsx`/`.module.css`/`.test.tsx` onto the same `WorkspaceFrame` + `ScrollRegion` + `EmptyState` composition every already-migrated `*Workspace.tsx` uses, dropping the unused `icon` prop (verified zero live call sites pass one).
- Registered an empty `design-system/exception-proposals/shell-overlays.json` manifest — no exception was needed for either task's slice.

## Task Commits

1. **Task 1: Migrate hotkey settings and keyboard shortcuts** — `ba70415e`
2. **Task 2: Migrate shared workspace chrome and empty destinations** — `51c77e77`

## Files Created/Modified

- `frontend/src/components/HotkeySettings/HotkeySettings.tsx` — Button/IconButton conversion
- `frontend/src/components/HotkeySettings/HotkeySettings.module.css` — full token migration
- `frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.tsx` — Panel/ScrollRegion conversion
- `frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.module.css` — full token migration
- `frontend/src/components/KeyboardShortcuts/KeyboardShortcuts.test.tsx` — new, 4 tests
- `frontend/src/workspaces/workspace.module.css` — in-place token swap only
- `frontend/src/workspaces/ComingSoon.tsx` — WorkspaceFrame/ScrollRegion/EmptyState conversion
- `frontend/src/workspaces/ComingSoon.module.css` — reduced to one `.canvas` centering rule
- `frontend/src/workspaces/ComingSoon.test.tsx` — added a named-region assertion
- `frontend/design-system/exception-proposals/shell-overlays.json` — new, empty manifest

## Decisions Made

See `key-decisions` in frontmatter. In summary: rebind buttons became `Button` (variant-driven recording state), reset controls became standard-disabled `IconButton` (was previously fully hidden via `visibility:hidden` until customized — now dimmed like every other disabled control in the app), `ComingSoon`'s unused `icon` prop was dropped rather than extending `WorkspaceFrame`'s contract for zero live callers, and two token substitutions (Panel's default padding, `groupHeading`'s line-height) accepted the shared primitive's/token's own nearest value over re-deriving a bespoke one-off.

## Deviations from Plan

### Auto-fixed Issues

None — this plan's `<verify>` blocks required zero exceptions, and the migration achieved that without needing any Rule 1/2/3 fixes. All changes were direct token/primitive substitutions per the plan's own action text.

### Rejected Plan Elements

None.

---

**Total deviations:** 0
**Impact on plan:** Executed as written; the only judgment calls were selecting the nearest existing token/primitive default where the legacy CSS used a bespoke one-off value (documented above), none of which affect functional behavior.

## Known Stubs

None. `ComingSoon` itself is an intentional, documented stub component (for nav destinations with no frontend/Wails binding yet) but currently has zero live call sites in production routes — Fixture Library, Save & Recovery, Overview, and Diagnostics have all since graduated to real workspaces per prior plans. It remains available for whichever destination becomes a stub next.

## Issues Encountered

- This worktree had no `frontend/node_modules` (worktrees do not inherit the parent checkout's gitignored `node_modules`). Ran `npm ci` inside the worktree to install dependencies locally before any test/build command could run; this is local, gitignored tooling state, not a repository change.
- Confirmed via `git diff` that `frontend/package-lock.json` and `frontend/package.json` are byte-identical between the main repository and this worktree before running `npm ci`, so the installed dependency set matches what CI/the rest of the team would install.

## Next Phase Readiness

HotkeySettings.tsx and KeyboardShortcuts.tsx are fully migrated with zero unregistered violations, closing two of the three files (`13-15`/`13-24`/`13-29`) identified in `13-13-SUMMARY.md`'s "Issues Encountered" as live consumers of the legacy tokens Plan 13-08 removed from `index.css`. `workspace.module.css`'s in-place migration is available to every remaining `*Workspace.tsx` wrapper without further changes. `ComingSoon`'s new WorkspaceFrame/ScrollRegion/EmptyState composition is a template for whichever future nav destination needs a stub next.

## Self-Check: PASSED

- Commits `ba70415e` and `51c77e77` exist on `worktree-agent-a697fe1814433e95b` and together contain all eleven declared files plus the two new test/manifest files.
- All declared file paths exist on disk (`HotkeySettings.tsx/.module.css/.test.tsx`, `KeyboardShortcuts.tsx/.module.css/.test.tsx`, `workspace.module.css`, `ComingSoon.tsx/.module.css/.test.tsx`, `shell-overlays.json`).
- The plan's own `<verify>` commands for both tasks pass, as does the combined check across all eleven files, `tsc --noEmit`, the full 481/481 frontend suite, and the full `npm run build` gate.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
