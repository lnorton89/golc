---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "20"
subsystem: testing
tags: [validation, evidence, tdd, node, vitest, git, sign-off]

requires:
  - phase: 13-18
    provides: "the design-system Node/Vitest/Playwright command surface (check:design-system, test:design-system, test:e2e:design-system) this validator's npm script sits alongside"
provides:
  - "frontend/scripts/design-system/validate-phase13-evidence.mjs: plan-derived command/hash contract parser, closed result-row/artifact schema, and typed semantic validators for calibration, mask audit, packaged WebView2, Windows CI, CI implementation-tree ancestry, and all six named UI backstops"
  - "frontend/scripts/design-system/validate-phase13-evidence.test.ts: 90 tests proving the validator accepts real already-committed evidence and fails closed on every named mutation category"
  - "npm run validate:phase13-evidence: default mode validates the plan-derived contract plus whatever evidence/*.json files already exist; --evidence <path> mode validates a full sign-off bundle"
affects: [13-35, 13-38, 13-39]

tech-stack:
  added: []
  patterns:
    - "A hand-rolled duplicate-key-detecting JSON parser (parseStrictJson) sits at the CLI --evidence boundary instead of a bare JSON.parse, which silently drops all but the last of duplicate object keys."
    - "Git-backed CI implementation-tree identity (sorted non-planning path/mode/blob manifest, SHA-256, descendant ancestry, .planning/**-only allowlist for later commits) is implemented behind an injectable gitRunner interface (lsTree/diffNameOnly/isAncestor) so tests never shell out to real git."
    - "Every semantic evidence validator (calibration, six backstops, packaged WebView2, Windows CI) is exercised first against the real evidence/*.json files already committed by sibling Wave 8-9 plans, then against single-fault mutations of those same files -- not synthetic-only fixtures."

key-files:
  created:
    - frontend/scripts/design-system/validate-phase13-evidence.mjs
    - frontend/scripts/design-system/validate-phase13-evidence.test.ts
  modified:
    - frontend/package.json

key-decisions:
  - "Task identity/command derivation includes checkpoint tasks, not just type=\"auto\" tasks: 13-01-01 and 13-35-01 are checkpoint tasks that carry their own <verify><automated> preflight command, and 13-VALIDATION.md's own Plan-Derived Command Contract lists both under normal wave rows. The parser extracts a command from any task element that has a <verify><automated> block, matching the real 77-task/77-command corpus exactly (confirmed via grep before writing the parser)."
  - "The default (no --evidence) CLI mode validates the plan-derived contract plus whatever evidence/*.json files already exist in the phase evidence directory, rather than requiring a not-yet-produced sign-off bundle to run at all. This makes `npm run validate:phase13-evidence` genuinely runnable today (validated against the real dialog-feasibility.json, screenshot-calibration.json, and all six backstop evidence files already committed by Plans 13-06/13-17/13-30/13-31/13-41) while the full `--evidence <bundle>` sign-off-gate mode (used by 13-35/13-38/13-39) is exercised in tests against constructed fixtures, since no phase-acceptance.json/windows-ci-run.json bundle exists yet."
  - "text-zoom-200.json's real evidence records requestedZoom/computedZoom as the strings \"2\"/\"2\" rather than the number 2 -- discovered empirically running the validator against the real file. validateBackstopTextZoom coerces with Number() before comparing to 2, rather than requiring a specific JS type, since the semantic contract is \"exactly 200%\" not \"typeof number\"."
  - "Split the two TDD tasks along a natural seam in the same test file rather than duplicating fixtures across two files: Task 1's commit contains the validator module, the npm script, and one describe block per category proving both real-evidence acceptance and a representative rejection; Task 2's commit adds the exhaustive single-fault mutation matrix, the CI implementation-tree ancestry/forgery suite (with its injectable fake git runner), coverage-exactness tests, the strict-JSON duplicate-key suite, and the whole-bundle false-sign-off gate tests. Both commits independently pass `npx tsc --noEmit` and their respective test-file states."

requirements-completed: [D-01, D-02, D-03, D-04, D-05, D-06, D-07, D-08, D-09, D-10, D-11, D-12, D-13, D-14, UI-SPEC-MIGRATION-ACCEPTANCE]

coverage:
  - id: D1
    description: "Parser discovers every 13-NN-PLAN.md task (including checkpoint tasks with their own automated preflight command), derives the exact normalized command and SHA-256, and rejects missing/extra/stale rows or any non-equal command/hash"
    requirement: "D-09"
    verification:
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#plan-derived command contract (real PLAN.md corpus) > derives exactly 77 tasks across the phase's 41 PLAN.md files"
        status: pass
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#mutation: command/hash and exit-status boundaries"
        status: pass
    human_judgment: false
  - id: D2
    description: "Closed result-row schema (task ID, plan/wave, exact command/hash, exit status zero, timestamps, repository commit SHA, environment/build identity, typed artifacts) rejects Markdown/existence-only claims"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#closed result-row schema"
        status: pass
    human_judgment: false
  - id: D3
    description: "Calibration arithmetic, mask/protected-locator audit, packaged WebView2, Windows CI evidence, and all six named UI backstops are semantically validated against real already-committed evidence and fail closed on single-fault mutation"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#semantic calibration arithmetic, mask audit, packaged WebView2 evidence, semantic backstop:* (six describes), Windows CI run evidence"
        status: pass
    human_judgment: false
  - id: D4
    description: "CI-proven implementation-tree identity: sorted non-planning manifest/hash, descendant ancestry, .planning/**-only allowlist for later commits, and rejection of any non-planning change after the proven SHA"
    requirement: "D-11"
    verification:
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#implementation tree ancestry and non-planning manifest identity"
        status: pass
    human_judgment: false
  - id: D5
    description: "Whole-bundle sign-off gate: wave_0_complete/nyquist_compliant/approved can only be true after every row, evidence category, and coverage check passes; any single regression rejects a simultaneous premature sign-off claim"
    requirement: "UI-SPEC-MIGRATION-ACCEPTANCE"
    verification:
      - kind: unit
        ref: "scripts/design-system/validate-phase13-evidence.test.ts#whole evidence bundle: false sign-off is rejected"
        status: pass
    human_judgment: false

duration: ~1 session
completed: 2026-08-03
status: complete
---

# Phase 13 Plan 20: Plan-Derived Phase-13 Evidence Validator Summary

**A 937-line Node validator (`validate-phase13-evidence.mjs`) that derives the exact task/command/SHA-256 contract from all 41 `13-NN-PLAN.md` files (77 tasks, including checkpoint-task preflight commands), enforces a closed result-row/artifact schema, and semantically validates calibration arithmetic, mask/protected-locator audits, packaged WebView2 evidence, Windows CI runs, CI-proven implementation-tree ancestry, and all six named UI backstops -- proven against 90 tests that pass both on the real evidence already committed by sibling plans and on single-fault mutations of every trust field.**

## Performance

- **Tasks:** 2/2 complete
- **Files created:** 2
- **Files modified:** 1
- **Test count:** 90 (49 in Task 1's commit, 41 more in Task 2's commit)

## Accomplishments

- `derivePlanCommandContract` parses every `13-NN-PLAN.md` in the phase directory and derives 77 task/command/hash entries (verified equal to `13-VALIDATION.md`'s own declared `task_count: 77`), including the two checkpoint tasks (`13-01-01`, `13-35-01`) that carry their own automated preflight command alongside their human gate.
- `normalizeCommand`/`sha256` decode XML entities and normalize CRLF/LF and surrounding whitespace exactly once, then require byte-exact string equality plus SHA-256 -- no shell-equivalent substitution is accepted.
- `validateResultRow`/`validateArtifact` enforce the closed schema: exact command/hash match, exit status zero, ISO timestamps, 40-hex repository commit SHA, dirty-tree declaration, environment/build identity, and typed artifacts with contained repository-relative or immutable `https://` paths and 64-hex SHA-256.
- Typed semantic validators recompute rather than trust: `validateCalibrationEvidence` recomputes pairwise-diff maxima and the selected threshold; `validateMaskAudit` computes real rectangle intersection against protected locators (Blackout, Revoke Automation, etc.); `validateImplementationTreeIdentity`/`computeImplementationManifest` recompute a sorted non-planning `git ls-tree` manifest/SHA-256 at both the proven and observed commits and require ancestry plus a `.planning/**`-only changed-path allowlist.
- All six separately named backstops (`startup-theme-font-before-settle`, `error-boundary-before-theme-css`, `specialized-geometry-900-1280`, `expanded-copy-2x-reflow`, `text-zoom-200-900x720`, `provider-daemon-offline-safety`) have dedicated validators that were run against the real `evidence/*.json` files already committed by Plans 13-06/13-17/13-30/13-31/13-41 -- all pass unmodified.
- `parseStrictJson` is a hand-rolled JSON parser used at the `--evidence` CLI boundary that fails closed on duplicate object keys (including nested inside arrays) and malformed/truncated JSON, instead of a bare `JSON.parse` that silently keeps only the last of two duplicate keys.
- `validateEvidenceBundle` is the whole-bundle sign-off gate: it only reaches `ok: true` after every plan-derived row, every semantic evidence category, and exact D-01..D-14/UI-SPEC and six-backstop coverage all pass -- and explicitly rejects a bundle that claims `wave_0_complete`/`nyquist_compliant`/`approved` while any error remains.
- `npm run validate:phase13-evidence` (default mode, no arguments) is genuinely runnable today: it derives the 77-task contract and validates every already-produced `evidence/*.json` file in the phase directory, currently passing 0 errors against the real committed evidence.

## Task Commits

1. **Task 1: Implement exact plan-derived command and semantic evidence validation** - `bdab94e7` (feat)
2. **Task 2: Prove every false-sign-off mutation is rejected** - `a8fb271f` (test)

_TDD note: both tasks are `tdd="true"` evidence-validation-tooling tasks. Following Plan 13-30's established precedent for this class of task ("implemented and verified together per task, matching that plan's precedent rather than a literal unit-test RED/GREEN cycle"), Task 1's commit contains the validator module plus one describe block per category proving both real-evidence acceptance and a representative rejection (49 tests); Task 2's commit adds the exhaustive single-fault mutation matrix, CI implementation-tree ancestry/forgery suite, coverage-exactness tests, strict-JSON duplicate-key suite, and whole-bundle false-sign-off gate tests (41 more tests, 90 total). Both commits independently pass `npx tsc --noEmit` and their respective test-file states in isolation (verified before each commit)._

## Files Created/Modified

- `frontend/scripts/design-system/validate-phase13-evidence.mjs` - plan-derived command/hash contract parser, closed schema, and all typed semantic validators (937 lines)
- `frontend/scripts/design-system/validate-phase13-evidence.test.ts` - 90 tests across parsing, schema, semantic-evidence, mutation, coverage, and whole-bundle sign-off suites (808 lines)
- `frontend/package.json` - added `validate:phase13-evidence` script

## Decisions Made

See `key-decisions` in frontmatter. In short: (1) checkpoint tasks with their own automated preflight command are included in the derived contract, not just `type="auto"` tasks -- confirmed against the real PLAN.md corpus before writing the parser; (2) the default CLI mode validates whatever evidence already exists rather than requiring a not-yet-produced sign-off bundle; (3) `text-zoom-200.json`'s real `requestedZoom`/`computedZoom` fields are strings, so the validator coerces with `Number()` rather than requiring a specific JS type; (4) the two TDD tasks split along a natural seam in one test file rather than duplicating fixtures.

## Deviations from Plan

None - plan executed exactly as written. The action item's grounding note ("Follow `internal/projectconfig/strict_test.go` for stable fail-closed diagnostics") was followed in spirit: every validator returns a flat array of stable, descriptive string diagnostics rather than throwing on the first failure, so a single evidence-bundle run surfaces every problem at once (matching that Go test suite's fail-closed, exhaustive-diagnostic style).

## Issues Encountered

- This worktree started with no `frontend/node_modules` at all (consistent with the worktree instructions' warning about a prior agent's junction). Ran `npm ci` inside this worktree's own `frontend/` directory to get a real, independent copy before any verification could run.
- One test assertion string had to be corrected to match the validator's actual (accurate) diagnostic wording ("does not equal recomputed max" rather than "does not equal the recomputed max") -- caught immediately by the first full test run, fixed in the test, not the validator.
- `text-zoom-200.json`'s real `requestedZoom`/`computedZoom` values are the strings `"2"`/`"2"`, not the number `2` -- discovered empirically running the validator's default CLI mode against the real committed evidence file before writing tests against it. Adjusted the comparison to `Number(value) === 2`.

## Next Phase Readiness

The plan-derived evidence-validation mechanism required by Plans 13-35, 13-38, and 13-39 (Windows CI evidence, complete local acceptance, and final sign-off) is implemented and independently proven against real evidence. `npm run validate:phase13-evidence -- --evidence <path>` is ready to validate a `phase-acceptance.json` or `windows-ci-run.json` bundle once those later plans produce one; the exact bundle shape it expects (`rows`, `calibration`, `masks`, `packagedWebView2`, `windowsCi`, `implementationTree`, `backstops`, `requirementsCovered`, `backstopsCovered`, `signOff`) is documented by `validateEvidenceBundle`'s own implementation and exercised end-to-end by the "whole evidence bundle" test suite. No blockers for the remaining Wave 12-17 plans.

## Self-Check: PASSED

- Commits `bdab94e7` and `a8fb271f` exist in `git log` and together contain `frontend/scripts/design-system/validate-phase13-evidence.mjs`, `frontend/scripts/design-system/validate-phase13-evidence.test.ts`, and `frontend/package.json`.
- All three files exist on disk at their declared paths.
- Both of this plan's own declared verify commands pass against the final committed state: `cd frontend && npx vitest run scripts/design-system/validate-phase13-evidence.test.ts` (90/90) and the same command with `--testNamePattern="mutation|false sign-off|semantic|implementation tree|zoom|offline safety"` (53/53 matched, 37 skipped by design).
- `cd frontend && npx tsc --noEmit` clean.
- `cd frontend && npx vitest run` (full suite) -- 618/618 pass across 76 files, no regression.
- `node scripts/design-system/validate-phase13-evidence.mjs` (default CLI mode) exits 0 against the real repository state.
- No protected paths (`internal/deskmidi/`, `site/`, `go.mod`, `go.sum`, `internal/projectconfig/reference_property_test.go`, `cmd/golc-desktop/rsrc_windows_amd64.syso`) were touched. No commit deleted any tracked file (`git diff --diff-filter=D` empty for both commits).

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
