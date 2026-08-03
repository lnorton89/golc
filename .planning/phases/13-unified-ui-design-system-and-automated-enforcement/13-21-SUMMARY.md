---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: 21
subsystem: ui-design-system
tags: [tokens, manifests, generator, vitest]
requires: [13-01]
provides:
  - Strict schema-v1 authority for semantic theme tokens and UI design-system records
  - Byte-stable generated CSS and TypeScript token surfaces
affects: [ui-enforcement, theme-migration]
tech-stack:
  added: []
  patterns: [closed JSON manifests, duplicate-key rejection, contained exception paths, deterministic generation]
key-files:
  created:
    - frontend/design-system/tokens.json
    - frontend/scripts/design-system/manifest.mjs
    - frontend/scripts/design-system/generate.mjs
    - frontend/src/design-system/tokens.generated.css
    - frontend/src/design-system/tokens.generated.ts
  modified:
    - frontend/scripts/design-system/manifest.test.ts
decisions:
  - Two shared Paper/Ink palettes provide every semantic role for each approved theme/mode face.
  - Generator output is sorted, LF-normalized, timestamp-free, and check mode is read-only.
metrics:
  tasks_completed: 2
status: complete
---

# Phase 13 Plan 21: Strict Design-System Authority Summary

**Closed, Paper/Ink-preserving token manifests generate deterministic CSS and TypeScript surfaces for all approved theme faces.**

## Accomplishments

- Added strict pure-JSON schema-v1 token, component, runtime-geometry, and exception authorities.
- Rejected malformed JSON, duplicate object keys, unknown record fields, absolute/traversing paths, and symlink escapes.
- Defined every semantic role for each of the twelve approved themes in light and dark mode, with the default faces preserving Paper/Ink values.
- Enforced the 4px spacing grid, Guided First Show 8px gap, and its 210px sizing rail.
- Added deterministic CSS/TypeScript generation and read-only drift checking.

## Task Commits

1. `e5aaadb3` — `test(13-21): add failing manifest authority tests`
2. `ff0f8fbc` — `feat(13-21): add strict design-system manifest authority`
3. `e8870af5` — `test(13-21): add failing token generation contract`
4. `299d4736` — `feat(13-21): generate deterministic design tokens`

## Verification

- Passed: `cd frontend && npx vitest run scripts/design-system/manifest.test.ts` (7 tests).
- Passed: `cd frontend && node scripts/design-system/generate.mjs && node scripts/design-system/generate.mjs --check`.
- Pending root integration: the exact planned `npm run generate:design-system` command cannot run until the package integration owner adds that script to `frontend/package.json`, which is outside this plan's declared ownership.

## Deviations from Plan

None - implementation files followed the plan exactly. The missing package script is an out-of-scope integration dependency, reported to the phase orchestrator.

## Known Stubs

None.

## Self-Check: PASSED

- Confirmed every generated file and manifest authority file exists.
- Confirmed all four task commits exist in repository history.
