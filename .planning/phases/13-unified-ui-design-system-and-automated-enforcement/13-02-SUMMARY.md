---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: 02
subsystem: ui-design-system
tags: [policy, postcss, typescript-ast, fixtures, vitest]
requires: [13-21]
provides:
  - Deterministic CSS and TSX policy boundaries for DS001 through DS010
  - Scoped and whole-source checker orchestration with exact exception validation
  - Allowed and forbidden held-out fixtures for every policy rule
affects: [frontend-validation, design-system-migration]
tech-stack:
  added: []
  patterns: [PostCSS parsing, TypeScript 6 compatibility AST traversal, normalized sorted diagnostics, exact exception matching]
key-files:
  created:
    - frontend/scripts/design-system/check.mjs
    - frontend/scripts/design-system/css-policy.mjs
    - frontend/scripts/design-system/tsx-policy.mjs
    - frontend/scripts/design-system/check.test.ts
    - frontend/scripts/design-system/fixtures
  modified: []
decisions:
  - Policy source is parsed rather than regex-scanned; malformed CSS and TSX fail closed.
  - Scoped feedback and full-source enumeration use the same policy engine and stable diagnostic ordering.
  - Exceptions may suppress exactly one diagnostic only and can never authorize off-grid spacing.
metrics:
  tasks_completed: 2
  tests: 17
status: complete
---

# Phase 13 Plan 02: Fail-Closed Policy Engine Summary

**PostCSS and TypeScript-AST enforcement now provides deterministic DS001–DS010 boundaries, polarity fixtures, and exact-waiver protections.**

## Accomplishments

- Added parser-first CSS checks for raw visual literals, custom-property ownership/reference validity, theme selectors, native-control styling, duplicate shared concepts, and focus/motion/safety signals.
- Added TypeScript 6 compatibility AST checks for feature-owned styled native controls and theme branches while permitting native semantics within public primitives.
- Added deterministic scoped and full-source orchestration that normalizes paths and LF, rejects unresolved sources, and sorts diagnostics.
- Added held-out allowed/forbidden fixtures for every DS001–DS010 rule plus 17 Vitest checks for policy boundaries and exact exception behavior.

## Task Commits

1. `30a438ad` — `test(13-02): add failing design policy fixtures`
2. `dcfa0a30` — `feat(13-02): implement CSS and TSX policy boundaries`
3. `c4b2feee` — `feat(13-02): orchestrate fail-closed design policy checks`

## Verification

- Passed: `cd frontend && npx vitest run scripts/design-system/check.test.ts` (17 tests).
- Passed: `cd frontend && node scripts/design-system/check.mjs scripts/design-system/fixtures/DS001/allowed.module.css`.

## Decisions Made

- Use PostCSS for CSS structure and the installed `@typescript/typescript6` compatibility API for TSX AST traversal.
- Require a one-diagnostic exact exception match; stale, broad, invalid, and spacing-bypass records report DS008.

## Integration Dependency

`frontend/package.json` remains untouched as assigned. Plan 13-18 must add the `check:design-system` script and make the normal frontend build invoke it; this checker is ready for that integration.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Ran the checker from the frontend root**
- **Found during:** Task 2 verification
- **Issue:** The first CLI invocation resolved `scripts/design-system/check.mjs` from the repository root and could not locate it.
- **Fix:** Re-ran the documented checker command from `frontend`, matching the plan's verification convention.
- **Files modified:** None
- **Commit:** N/A

## Known Stubs

None. The only apparent native button/class string is an intentional forbidden DS005 fixture.

## Self-Check: PASSED

- Confirmed all checker source, tests, and fixture files exist.
- Confirmed commits `30a438ad`, `dcfa0a30`, and `c4b2feee` exist in repository history.
