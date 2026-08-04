---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "37"
subsystem: design-system-primitives
tags: [design-system, confirmation, dialog, migration, cleanup]

requires:
  - phase: 13-19
    provides: ConfirmModal's own token/architecture fixes (--ds-surface-scrim, --ds-stacking-modal) and the whole-source zero-diagnostic baseline this plan builds on
provides:
  - "ConfirmModal fully removed: implementation, module CSS, barrel export, components.json inventory record, DESIGN_SYSTEM.md doc row, and its two exceptions.json animation exception records are all absent"
  - "AppShell's guide-navigation guard and NotesWorkspace's delete confirmation now use ConfirmDialog, the sole surviving public confirmation contract"
  - "design-system.contract.test.ts 'ConfirmModal removal' describe block: semantic directory/export/inventory/exception-manifest/doc/source-import absence assertions (import.meta.glob-based, not a single file-existence check)"
affects: []

tech-stack:
  added: []
  patterns:
    - "A vitest contract test living under src/ (browser-scoped tsconfig, no @types/node) that needs to prove filesystem/text-content absence must use Vite's import.meta.glob (with { query: \"?raw\", import: \"default\" } for text content), not node:fs/node:path/node:url -- those builtins are only typed for scripts/ tsconfigs, not src/'s tsconfig.json (include: [\"src\"], no types field)."
    - "ConfirmDialog is now the only public confirmation contract; any 'are you sure?' prompt should render ConfirmDialog unconditionally with an open prop (Dialog itself returns null when !open) rather than conditionally mounting/unmounting the component tree -- matches ConfirmDialog's own existing API and avoids remount-driven focus/animation churn."

key-files:
  created: []
  modified:
    - frontend/DESIGN_SYSTEM.md
    - frontend/design-system/components.json
    - frontend/design-system/exceptions.json
    - frontend/e2e/design-system.expanded-copy.spec.ts
    - frontend/e2e/fixtures/expandedCopy.ts
    - frontend/src/components/primitives/LoadingState/LoadingState.module.css
    - frontend/src/design-system/design-system.contract.test.ts
    - frontend/src/design-system/index.ts
    - frontend/src/shell/AppShell.tsx
    - frontend/src/workspaces/show/NotesWorkspace.tsx
  deleted:
    - frontend/src/components/primitives/ConfirmModal/ConfirmModal.tsx
    - frontend/src/components/primitives/ConfirmModal/ConfirmModal.module.css

key-decisions:
  - "Discovered ConfirmModal was NOT already fully migrated as the plan's <context> implied (13-19's summary only covered ConfirmModal's own token/architecture fixes, not caller migration) -- AppShell.tsx's guide-navigation-leave guard and NotesWorkspace.tsx's delete confirmation both still imported and rendered ConfirmModal directly. Migrated both callers to ConfirmDialog before deleting anything, per the plan's own instruction to verify caller migration rather than assume it."
  - "AppShell's GuardedCommandRail: converted from conditional JSX mounting ({pendingDestination && <ConfirmModal .../>}) to ConfirmDialog's own open-prop pattern (always rendered, open={pendingDestination !== null}); onConfirm now guards with an explicit `if (pendingDestination)` since TypeScript's closure narrowing (which worked when the JSX literal was lexically nested inside the `pendingDestination &&` branch) no longer applies once the component is unconditionally rendered."
  - "NotesWorkspace's delete confirmation: added destructive={true} (ConfirmModal had no destructive concept; ConfirmDialog's destructive prop is what selects alertdialog role and the destructive button variant) -- required to keep the existing test's `getByRole('alertdialog')` expectation passing."
  - "Fixed a real e2e regression this migration causes, not just a rename: design-system.expanded-copy.spec.ts's dialog-category selector (`div[role='alertdialog'] p[class*='message']`) targeted ConfirmModal's own <p className={styles.message}> paragraph. Dialog (which ConfirmDialog wraps) renders its description in a <div className={styles.description}>, not a <p class='message'>. Updated the selector to `div[role='alertdialog'] div[class*='description']` -- confirmed correct against Dialog.module.css's actual .description class."
  - "The new contract test's directory/text-content absence checks use Vite's import.meta.glob rather than node:fs/node:path/node:url: src/design-system.contract.test.ts sits under frontend/tsconfig.json's browser-scoped 'include': ['src'], which has no 'types' field and therefore no @types/node -- tsc --noEmit failed with TS2591 on the node: imports. Rewrote using import.meta.glob (directory-absence via empty-match check, text content via { query: '?raw', import: 'default' }), matching the codebase's existing vite/client.d.ts reference and avoiding any Node-specific typing dependency."
  - "Removed exceptions.json's two ConfirmModal.module.css animation exception records (DS001, backdropFadeIn/dialogPopIn) since their path no longer exists on disk -- an orphaned exception referencing a deleted file would itself be a DS008 (exception-integrity) violation."
  - "Swept a stray comment in LoadingState.module.css referencing 'ConfirmModal's own backdropFadeIn/dialogPopIn exceptions' (used only as a shape-comparison example, not a real dependency) to reference Desk's own established exception precedent instead, keeping the comment accurate post-deletion."

requirements-completed: [D-02, D-05, D-08, D-11, UI-SPEC-MIGRATION-ACCEPTANCE]

coverage:
  - id: D1
    description: ConfirmModal implementation, imports, inventory/export, documentation, aliases, and compatibility layer are absent
    verification:
      - kind: unit
        ref: "cd frontend && npx vitest run src/design-system/design-system.contract.test.ts --testNamePattern=\"ConfirmModal removal\""
        status: pass
      - kind: static
        ref: "node scripts/design-system/check.mjs --rule DS007"
        status: pass
    human_judgment: false
  - id: D2
    description: Dialog/ConfirmDialog are the only public confirmation contracts
    verification:
      - kind: unit
        ref: "design-system.contract.test.ts's inventory record assertion confirms ConfirmDialog and Dialog remain present while ConfirmModal is absent"
        status: pass
    human_judgment: false
  - id: D3
    description: No regression to existing behavior, whole-source design-system parity, or type safety
    verification:
      - kind: static
        ref: "cd frontend && npx tsc --noEmit"
        status: pass
      - kind: unit
        ref: "cd frontend && npx vitest run (full suite)"
        status: pass
      - kind: static
        ref: "cd frontend && node scripts/design-system/check.mjs --all"
        status: pass
    human_judgment: false

metrics:
  duration: "~1 session"
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 37: ConfirmModal Removal Summary

**Deleted ConfirmModal (implementation, styles, barrel export, inventory record, doc reference, and its two exception records) after migrating its two remaining callers -- AppShell's guide-navigation guard and NotesWorkspace's delete confirmation -- to ConfirmDialog, the sole surviving public confirmation contract, and added semantic import/export/inventory/doc/exception-manifest absence assertions so a future regression can't silently reintroduce it.**

## Performance

- **Tasks:** 1/1 complete
- **Focused verify:** `npx vitest run src/design-system/design-system.contract.test.ts --testNamePattern="ConfirmModal removal"` -- 6 passed, 1 skipped (unrelated describe block); `node scripts/design-system/check.mjs --rule DS007` -- exit 0
- **Full verification:** `npx tsc --noEmit` clean; `npx vitest run` (full suite) 625/625 pass across 76 files; `node scripts/design-system/check.mjs --all` -- exit 0, zero diagnostics

## Accomplishments

- Grepped the whole `frontend` tree for `ConfirmModal` before deleting anything and found it was **not** fully migrated as the plan's context implied -- `AppShell.tsx` (guide-leave confirmation) and `NotesWorkspace.tsx` (delete-note confirmation) both still imported and rendered it directly.
- Migrated `AppShell.tsx`'s `GuardedCommandRail` from `{pendingDestination && <ConfirmModal .../>}` to `ConfirmDialog`'s own `open` prop pattern, with an explicit `if (pendingDestination)` guard inside `onConfirm` (TS closure narrowing no longer applies once the conditional JSX wrapper is gone).
- Migrated `NotesWorkspace.tsx`'s delete confirmation to `ConfirmDialog` with `destructive` set, preserving the existing `alertdialog` role and `Delete Note`/`Cancel` button labels the workspace's own Vitest test already asserts.
- Deleted `ConfirmModal.tsx` and `ConfirmModal.module.css`; removed its `design-system/index.ts` barrel export, its `design-system/components.json` inventory record, its `DESIGN_SYSTEM.md` doc table row, and its two `exceptions.json` animation exception records (both referenced the now-deleted `ConfirmModal.module.css` path, which would otherwise be an orphaned DS008 exception-integrity violation).
- Fixed a real e2e regression the migration itself causes: `design-system.expanded-copy.spec.ts`'s dialog-category selector targeted `ConfirmModal`'s own `<p className={styles.message}>`, but `Dialog` (which `ConfirmDialog` wraps) renders its description in a `<div className={styles.description}>`. Updated the selector from `p[class*="message"]` to `div[class*="description"]`.
- Added a `"ConfirmModal removal"` describe block to `design-system.contract.test.ts` with six semantic assertions: directory absence (via `import.meta.glob`, empty match set), barrel non-export, inventory non-record (plus confirming `ConfirmDialog`/`Dialog` remain present), exception-manifest non-entry, doc-guide non-reference, and a whole-`src`-tree scan (via `import.meta.glob` with `?raw` text imports) for any stray `ConfirmModal` string in source.
- Swept a stray shape-comparison comment in `LoadingState.module.css` that referenced `ConfirmModal`'s own animation exceptions, pointing it at Desk's own established precedent instead.

## Task Commits

1. **Task 1: Remove ConfirmModal and every compatibility seam** - `072bf373` (feat) - caller migration (AppShell, NotesWorkspace), file/export/inventory/doc/exception deletion, contract-test semantic assertions, e2e selector fix, stray-comment sweep.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Callers were not already migrated to Dialog/ConfirmDialog**
- **Found during:** Task 1, pre-deletion grep sweep (per the plan's own critical-context instruction to verify, not assume)
- **Issue:** `AppShell.tsx` and `NotesWorkspace.tsx` both still imported and rendered `ConfirmModal` directly. Deleting the primitive without migrating them first would have been a build-breaking regression.
- **Fix:** Migrated both callers to `ConfirmDialog`, matching its existing `open`/`destructive`/`initialFocusRef`-via-`Dialog` API.
- **Files modified:** `frontend/src/shell/AppShell.tsx`, `frontend/src/workspaces/show/NotesWorkspace.tsx`
- **Verification:** `npx tsc --noEmit` clean; `npx vitest run` full suite 625/625 pass (including `AppShell.test.tsx`, `AppShell.navigation.test.tsx`, `NotesWorkspace.test.tsx`).
- **Committed in:** `072bf373`

**2. [Rule 1 - Bug] e2e dialog-category selector would silently stop matching**
- **Found during:** Task 1, while reviewing every remaining `ConfirmModal` string reference before removal
- **Issue:** `design-system.expanded-copy.spec.ts`'s `assertDialogCategory` used `div[role="alertdialog"] p[class*="message"]` to measure the confirm dialog's message-wrap behavior -- a selector coupled to `ConfirmModal.module.css`'s own `.message` class on a `<p>`. `Dialog.module.css` (which `ConfirmDialog` wraps) renders its description in a `<div className={styles.description}>`, so the old selector would match zero elements post-migration, silently disabling the assertion rather than failing loudly.
- **Fix:** Updated the selector to `div[role="alertdialog"] div[class*="description"]`, confirmed against `Dialog.module.css`'s actual `.description` class.
- **Files modified:** `frontend/e2e/design-system.expanded-copy.spec.ts`
- **Verification:** Selector confirmed by reading `Dialog.module.css` directly; this worktree has no Playwright/browser tooling available to run the e2e suite itself (see Issues Encountered).
- **Committed in:** `072bf373`

**3. [Rule 3 - Blocking] node:fs/node:path/node:url are untyped under src/'s tsconfig**
- **Found during:** Task 1, first `npx tsc --noEmit` verification pass after writing the contract test with Node builtins
- **Issue:** `frontend/tsconfig.json`'s `include: ["src"]` scope has no `types` field and no `@types/node`, so `import { readFileSync } from "node:fs"` (and `node:path`, `node:url`) failed with `TS2591: Cannot find name 'node:fs'` inside `src/design-system/design-system.contract.test.ts`.
- **Fix:** Rewrote the new `"ConfirmModal removal"` assertions using Vite's `import.meta.glob` (directory-absence via an empty match-set check; text-content checks via `{ query: "?raw", import: "default" }`) instead of Node filesystem APIs, matching the browser/Vite typing already available via `src/vite-env.d.ts`'s `/// <reference types="vite/client" />`.
- **Files modified:** `frontend/src/design-system/design-system.contract.test.ts`
- **Verification:** `npx tsc --noEmit` clean; focused test re-run 6/6 pass.
- **Committed in:** `072bf373`

---

**Total deviations:** 3 (1 Rule 1 auto-fix, 2 Rule 3 auto-fixes, no user permission required)
**Impact on plan:** All three were necessary consequences of correctly executing the plan's own stated must_haves ("ConfirmModal implementation, imports, inventory/export, documentation, aliases, and compatibility layer are absent" and "Dialog/ConfirmDialog are the only public confirmation contracts") -- none changed the plan's scope or intent.

## Known Stubs

None. Every change is a real caller migration, file/reference deletion, or semantic test assertion -- nothing renders a placeholder.

## Threat Flags

None. No new network endpoint, auth path, file access pattern, or schema change at a trust boundary was introduced. The plan's own threat register entry (T-13-40, "Alias retention could preserve duplicate authority") is mitigated exactly as planned: the new contract test's semantic import/export/inventory/doc/exception-manifest absence assertions are the mechanical guard against a future reintroduction, verified passing above.

## Issues Encountered

- **This worktree has no Playwright/browser tooling available.** The e2e selector fix (`design-system.expanded-copy.spec.ts`) is verified by direct code inspection against `Dialog.module.css`'s actual `.description` class, not by running the Playwright suite itself -- matching the same limitation `13-19-SUMMARY.md` documented for this same worktree-isolated execution model. `npx vitest run` (the full Vitest suite, including `NotesWorkspace.test.tsx`'s `alertdialog`/button-label assertions and `AppShell.test.tsx`/`AppShell.navigation.test.tsx`) is green.
- **`frontend/node_modules` was absent in this worktree at start** (per this session's setup note about a prior agent's junction-symlink incident) -- ran `npm install` inside this worktree's own `frontend/` directory to get a real, independent copy before any verification command could run. No lockfile changes resulted (`npm install` against the existing `package-lock.json`).

## Next Phase Readiness

ConfirmModal is fully absent -- implementation, styles, barrel export, inventory record, doc reference, and exception-manifest entries -- and its two callers now route through `ConfirmDialog`, matching `13-UI-SPEC.md`'s Component Inventory (`ConfirmDialog` listed as the primitive owning "Safe initial focus and action-specific confirmation"; `ConfirmModal` listed only as a "migration seed, not automatic proof"). `node scripts/design-system/check.mjs --all` remains at zero diagnostics, and the new `design-system.contract.test.ts` assertions stand as a permanent regression guard for this specific removal.

## Self-Check: PASSED

- Commit `072bf373` exists in `git log` and contains all 12 declared file changes (10 modified, 2 deleted).
- `frontend/src/components/primitives/ConfirmModal/` no longer exists on disk.
- `cd frontend && npx vitest run src/design-system/design-system.contract.test.ts --testNamePattern="ConfirmModal removal"` -- 6/6 pass.
- `cd frontend && node scripts/design-system/check.mjs --rule DS007` -- exit 0.
- `cd frontend && npx tsc --noEmit` clean.
- `cd frontend && npx vitest run` (full suite) 625/625 pass across 76 files.
- `cd frontend && node scripts/design-system/check.mjs --all` -- exit 0, zero diagnostics.
- No protected paths (`go.mod`, `go.sum`, `.planning/STATE.md`, `.planning/ROADMAP.md`) were touched.
