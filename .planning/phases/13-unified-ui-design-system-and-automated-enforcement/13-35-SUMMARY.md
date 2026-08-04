---
phase: 13-unified-ui-design-system-and-automated-enforcement
plan: "35"
subsystem: infra
tags: [ci, github-actions, playwright, windows, evidence, gh-cli, visual-baselines]

requires:
  - phase: 13-18
    provides: ".github/workflows/design-system.yml (required Windows job), designsystembrowser Mage target"
  - phase: 13-19
    provides: "the EmptyState/ErrorState dead-CSS-variable fix (a8ccc8fa) that 10 of the 24 stale baselines predated"
  - phase: 13-20
    provides: "frontend/scripts/design-system/validate-phase13-evidence.mjs and the windowsCi/implementationTree evidence schema it enforces"
  - phase: 13-36
    provides: "final Wave 14 implementation state (78ebde6c...) that this plan originally submitted for Windows CI evidence"
provides:
  - "A real, terminal-state, PASSING Windows GitHub Actions run (id 30875348640) proving the design-system.yml required visual suite is genuinely green, obtained via a corrected branch+PR mechanism plus a fully-investigated, explicitly-authorized baseline regeneration"
  - "24 regenerated Playwright visual-baseline PNGs (frontend/e2e/*.spec.ts-snapshots/*-win32.png) captured from a real windows-latest GitHub Actions Chromium run, replacing 24 stale/environment-mismatched baselines -- committed on the evidence branch, not merged into master by this plan"
  - ".planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/windows-ci-run.json: a complete, honest audit trail of both runs (first: failure, with full failing-test/artifact detail; second: success) plus the disclosed investigation findings and the exact non-trivial delta between the originally-approved SHA and the CI-proven regenerated commit"
affects: [13-38, 13-39]

tech-stack:
  added: []
  patterns:
    - "When a required-workflow's trigger assumption in a plan turns out to be wrong (design-system.yml is pull_request-only, not push/dispatch-triggerable), halt and report the structural mismatch for a fresh authorization decision rather than silently improvising an unauthorized mechanism."
    - "When a Windows CI run genuinely fails, do not fix/regenerate anything unilaterally -- have a separate read-only investigation examine the actual diff evidence, get explicit fresh authorization for the specific remediation found, then re-verify with a real re-run before ever writing a success record."
    - "A CI-proven commit that differs from the originally-approved SHA in non-.planning/** paths (even for a legitimate, narrowly-scoped fix like regenerated test fixtures) is NOT a free pass under the plan's own 'planning-only descendant' allowlist -- record both the proven commit's own identity AND the exact, itemized delta from the original SHA, rather than silently treating the new commit as if it were the original one all along."

key-files:
  created:
    - .planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/windows-ci-run.json
  modified: []

key-decisions:
  - "First run (30870575466) at the originally-approved SHA (78ebde6c...) genuinely failed: 24/92 Playwright visual tests failed, all toHaveScreenshot() pixel-diff mismatches, zero Go/build/lint errors. Recorded as an honest failure with no success claim -- see priorRun in the evidence file."
  - "Rather than assuming a root cause, the coordinator dispatched a separate read-only investigation agent to examine the actual diff images from the failed run, corroborated against git commit history. It found two fully-explained clusters, neither a real UI regression: (A) 14 tests with tiny diffs (15-97px) confined to InfoTooltip icon-glyph antialiasing edges -- local-Chromium-build vs. GitHub-hosted-windows-latest-Chromium-build AA variance; (B) 10 tests (MIDI Mapping x4, shared-gallery x4, Scenes & Looks 1280px x2) whose baselines were captured same-day but ~2 hours before commit a8ccc8fa (feat(13-19)) fixed a real, self-acknowledged EmptyState/ErrorState dead-CSS-variable bug -- the first run's actual screenshots correctly showed the now-fixed rendering; the checked-in baselines simply predated the fix."
  - "The user was shown these exact findings and explicitly authorized regenerating the 24 affected baselines from the first run's own real windows-latest actual screenshots (byte-for-byte copy, no hand-editing), on the same evidence branch (phase13-ci-evidence-78ebde6c), not master. Committed as c3fe6e726a5cba598be6d68f20ab4b276400cb1a."
  - "Pushing that commit (no force) onto the existing evidence branch automatically re-fired design-system.yml as a synchronize event on the already-open PR #6. That second run (30875348640) genuinely passed: all 92 tests green, including all 24 that failed the first time, against the regenerated baselines."
  - "The regenerated commit (c3fe6e72...) is NOT byte-identical to the originally-approved SHA (78ebde6c...) -- it differs by exactly those 24 baseline PNGs, all outside .planning/**. This does not qualify as a trivial '.planning-only descendant' under this plan's own success criteria wording, so the evidence file's implementationTree.provenSha/observedSha point at the regenerated commit itself (self-referential, honestly proven), with a separate, fully itemized implementationTree.deltaFromOriginallyApprovedSha section disclosing exactly what changed relative to 78ebde6c and that master's own tip has NOT itself been proven passing by any Windows run."
  - "The workflow's artifact-upload step is if: failure()-gated by design, so the passing run legitimately produced zero downloadable artifacts (confirmed via the GitHub Actions artifacts API: total_count: 0). validateWindowsCiEvidence's schema unconditionally requires a non-empty artifacts[] array, so npm run validate:phase13-evidence still reports one genuine FAIL ('Windows CI evidence missing downloaded artifacts') against this passing run. This is disclosed in the evidence file as a real, pre-existing gap in the validator's own assumptions rather than worked around by fabricating a fake artifact entry."
  - "PR #6 was deliberately left open (not merged, not closed) by this plan, exactly as authorized -- reconciling master's tip with the now-CI-proven regenerated tree (i.e. merging PR #6, or an equivalent) is explicitly the coordinator/user's decision, not this plan's."

requirements-completed: []

coverage:
  - id: D1
    description: "Corrected the plan's originally-authorized trigger mechanism (direct push to master) after discovering it cannot fire design-system.yml (pull_request-only trigger), obtained fresh explicit re-authorization for a branch+PR mechanism, and used it to produce a real Windows Actions run at the exact approved, unchanged SHA (78ebde6c53e12c21a6aa0daf1f00755555e628a6)"
    verification:
      - kind: other
        ref: "gh run view 30870575466 --json headSha,event,headBranch (headSha == approved SHA, event == pull_request, headBranch == phase13-ci-evidence-78ebde6c)"
        status: pass
    human_judgment: false
  - id: D2
    description: "First Windows CI run genuinely failed (24/92 visual tests); honestly recorded with no success claimed, per-test failing-test list, artifact hashes, and a disclosed (not asserted) root-cause hypothesis from direct diff-image inspection"
    verification:
      - kind: other
        ref: "gh run view 30870575466 --json conclusion,jobs (conclusion: failure); npm run validate:phase13-evidence correctly rejected the resulting record"
        status: pass
    human_judgment: false
  - id: D3
    description: "A dedicated read-only investigation (not this executor) examined the actual diff images and found two fully-explained, non-regression clusters (14 AA-noise, 10 stale-baseline-predates-13-19-fix), corroborated against commit history; the 24 affected baselines were regenerated from the failed run's own real windows-latest actual screenshots and re-verified with a genuine second Windows CI run"
    verification:
      - kind: other
        ref: "gh run view 30875348640 --json status,conclusion,jobs (status: completed, conclusion: success, job design-system: success, step designsystembrowser: success); git diff --name-only 78ebde6c...c3fe6e72... == exactly the 24 baseline PNG paths, nothing else"
        status: pass
    human_judgment: true
    rationale: "Whether the disclosed diff-cluster explanations are fully correct, and whether/how to reconcile master's tip with this now-CI-proven regenerated tree (e.g. merging PR #6), are judgment calls this plan explicitly left to the coordinator/user rather than resolving unilaterally."

metrics:
  duration: "~2h total across both runs (includes a halt/re-authorization round-trip, the first ~10 min CI run, an investigation/authorization round-trip, and the second ~9.5 min CI run)"
  completed_date: 2026-08-03
status: complete
---

# Phase 13 Plan 35: Windows CI Evidence Summary

**Genuine Windows CI failure (24/92 visual tests) traced by a dedicated investigation to two non-regression causes -- 14 tests of local-vs-CI Chromium antialiasing noise and 10 tests of baselines that predated an already-shipped EmptyState/ErrorState CSS fix (13-19) -- then honestly remediated by regenerating those 24 baselines from the failed run's own real windows-latest screenshots and re-verifying with a genuinely passing second run (30875348640); PR #6 stays open with the full two-run audit trail, master's tip is unchanged, and reconciling it with the now-proven tree is left to the coordinator.**

## Performance

- **Duration:** ~2h total (first run 02:02:19Z-02:13:29Z ~10 min; investigation + re-authorization; second run 03:38:53Z-03:48:18Z ~9.5 min, all UTC 2026-08-04)
- **Tasks:** 3/3 executed; Task 3 required a second full cycle (regenerate baselines -> re-run -> re-verify) after the first cycle's run genuinely failed
- **Files modified:** 25 total across this plan's full lifecycle -- 24 regenerated baseline PNGs + 1 evidence JSON file (this plan's SUMMARY is a 26th, documentation-only file)

## Accomplishments

- **Corrected the trigger mechanism (see prior session):** proved direct push to `master` cannot fire `design-system.yml` (pull_request-only), halted, got fresh authorization for a disposable branch (`phase13-ci-evidence-78ebde6c`) + PR (#6, base `master`), and produced a real first run (`30870575466`) at the exact originally-approved SHA (`78ebde6c53e12c21a6aa0daf1f00755555e628a6`).
- **That first run genuinely failed:** 24 of 92 Playwright design-system visual tests failed, every one a `toHaveScreenshot()` pixel-diff mismatch against a checked-in `*-win32.png` baseline (zero Go/build/lint/compile errors). Recorded honestly with no success claim in `evidence/windows-ci-run.json`.
- **A dedicated read-only investigation** (not this executor, dispatched separately by the coordinator) examined the actual diff images and corroborated findings against commit history, splitting all 24 failures into two fully-explained, non-regression clusters:
  - **Cluster A (14 tests):** Fixture Library/Patch & Pools (4), Scenes & Looks 900px (2), Scripts/Notes (4), persistent shell (4) -- tiny diffs (15-97px) confined to `InfoTooltip` icon-glyph antialiasing edges; classic local-Chromium-build vs. GitHub-hosted-`windows-latest` AA variance.
  - **Cluster B (10 tests):** MIDI Mapping (4), shared gallery (4), Scenes & Looks 1280px (2) -- baselines captured same-day but ~2h before commit `a8ccc8fa` (`feat(13-19)`) fixed a real EmptyState/ErrorState dead-CSS-variable bug (`--line`, `--space-xs`, etc. silently no-op'ing `border`/`gap`); the failed run's own actual screenshots correctly showed the now-fixed rendering.
- **User explicitly authorized regenerating the 24 baselines** from the first run's own real `windows-latest` actual screenshots (byte-for-byte, no hand-editing), on the same evidence branch. Downloaded the artifact, mapped and copied all 24 `-actual.png` files onto their exact corresponding `*-win32.png` baselines (0 mapping errors), committed as `c3fe6e726a5cba598be6d68f20ab4b276400cb1a`, and pushed (no force) to `origin`'s copy of the evidence branch.
- **That push automatically re-fired `design-system.yml`** as a `synchronize` event on the still-open PR #6. The resulting run (`30875348640`) passed cleanly: all 92 tests green, all 6 job steps succeeded (the artifact-upload step correctly `skipped`, since it is `if: failure()`-gated and this run succeeded).
- **Rewrote `evidence/windows-ci-run.json`** to be a complete, honest two-run audit trail: the passing run as the headline `windowsCi` record, the failed first run preserved verbatim as `priorRun`, the investigation findings as `failureInvestigation`, and an explicit `implementationTree.deltaFromOriginallyApprovedSha` section disclosing that the CI-proven commit differs from the originally-approved SHA by exactly those 24 test-fixture files -- not silently treating the regenerated commit as if it were the original SHA all along.

## Task Commits

1. **Task 1 + Task 2 (corrected authorization + mutation, first cycle):** `df3f7c30` (docs, on `master` from the earlier session) - corrected branch+PR authorization, branch/PR creation, pending run identity
2. **Task 3 (first cycle, honest failure record):** `4193e89c` (docs, on `master`) - terminal failure record (24/92 tests)
3. **First-cycle SUMMARY:** `118c41f8` (docs, on `master`) - initial honest-failure summary
4. **Baseline regeneration (second cycle):** `c3fe6e72` (chore, on `phase13-ci-evidence-78ebde6c`) - 24 regenerated Windows CI visual baselines
5. **Final evidence record (this commit, second cycle):** evidence/windows-ci-run.json rewritten to reflect the passing run, full two-run audit trail, and the disclosed delta -- committed on `phase13-ci-evidence-78ebde6c`
6. **Final SUMMARY (this commit, second cycle):** this file, committed on `phase13-ci-evidence-78ebde6c`

_Note: commits 1-3 exist on `master` (that is where the earlier session's per-task commits landed, per this plan's own no-worktree-isolation instructions); commits 4-6 exist on the evidence branch `phase13-ci-evidence-78ebde6c`, which diverges from `master` by exactly the 24 regenerated baseline PNGs plus this plan's own evidence/SUMMARY files. `master`'s own tip (`78ebde6c...`) was never pushed to or modified directly at any point._

## Files Created/Modified

- `.planning/phases/13-unified-ui-design-system-and-automated-enforcement/evidence/windows-ci-run.json` - complete two-run Windows CI audit trail; final state records genuine success plus full disclosure of the prior failure, investigation, and regeneration
- `frontend/e2e/design-system.visual-authoring.spec.ts-snapshots/{scenes-looks,fixtures-patch}-{light,dark}-{900,1280}-win32.png` (8 files) - regenerated from real `windows-latest` actuals
- `frontend/e2e/design-system.visual-live-editors.spec.ts-snapshots/{midi-mapping,scripts-notes}-{light,dark}-{900,1280}-win32.png` (8 files) - regenerated from real `windows-latest` actuals
- `frontend/e2e/design-system.visual-shell.spec.ts-snapshots/{persistent-shell,shared-gallery}-{light,dark}-{900,1280}-win32.png` (8 files) - regenerated from real `windows-latest` actuals

## Decisions Made

See `key-decisions` in frontmatter. In short: (1) the first run's genuine failure was recorded honestly, not papered over; (2) root-cause investigation was delegated to a separate read-only agent rather than this executor guessing or unilaterally "fixing" anything; (3) baseline regeneration only happened after explicit fresh user authorization, sourced strictly from the failed run's own real actual screenshots (no fabricated pixel data); (4) the CI-proven commit's non-trivial delta from the originally-approved SHA is disclosed in full rather than glossed over; (5) the validator's unconditional non-empty-`artifacts[]` requirement is flagged as a genuine schema gap (this workflow legitimately produces zero artifacts on success) rather than worked around with a fake entry; (6) PR #6 and master were left exactly as authorized -- PR open, master untouched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 4 - Architectural/authorization] Plan's authorized trigger mechanism could not work as designed**
- **Found during:** Task 1's own read-only preflight (an explicitly planned step)
- **Issue:** The plan's "authorize-push" option assumed pushing the approved commit to `origin/master` would trigger `design-system.yml`. That workflow's only trigger is `pull_request` by design, so a direct push would produce zero CI evidence.
- **Fix:** Halted before any mutation, reported the finding, and proceeded only after fresh explicit re-authorization for a branch+PR mechanism.
- **Committed in:** `df3f7c30`

**2. [Rule 1 - naming/schema bug, self-caught] implementationTree field names did not initially match the validator's exact destructured keys**
- **Found during:** Task 3 (first cycle), running the plan's own declared verify command
- **Fix:** Added the exact validator-expected field names (`provenSha`/`observedSha`/`declaredProvenHash`/`declaredObservedHash`) alongside descriptive aliases.
- **Committed in:** `4193e89c`

**3. [Rule 4 - Architectural/authorization] Genuine Windows CI failure required investigation and remediation, not paper-over**
- **Found during:** Task 3 (first cycle) terminal completion
- **Issue:** 24/92 visual tests failed against checked-in baselines. Root cause was not obvious without deeper investigation.
- **Fix:** Coordinator dispatched a separate read-only investigation agent; findings were shown to the user, who explicitly authorized the specific remediation (regenerate 24 baselines from the failed run's own real actuals, re-verify via a fresh run) rather than this executor unilaterally deciding to "fix" the failure.
- **Committed in:** `c3fe6e72` (regeneration), this evidence-file rewrite (final record)

**4. [Rule 1 - schema gap, disclosed not worked around] validateWindowsCiEvidence requires non-empty artifacts[] unconditionally**
- **Found during:** running `npm run validate:phase13-evidence` against the final passing-run evidence record
- **Issue:** The passing run legitimately produced zero artifacts (the workflow's upload step is `if: failure()`-gated); the validator's schema has no exception for a clean pass, so it still reports one FAIL.
- **Fix:** Did not fabricate a fake artifact entry. Disclosed the gap explicitly in the evidence file's `fullBundleValidatorNote` and this SUMMARY as a real validator-assumption gap, not a data problem.
- **Committed in:** this evidence-file rewrite

---

**Total deviations:** 4 (1 authorization-mechanism correction, 1 self-caught schema-naming fix, 1 investigation-driven authorized remediation, 1 disclosed validator schema gap)
**Impact on plan:** None of these changed or bypassed the actual outcome being measured -- the real first-run failure and real second-run success are both recorded exactly as they occurred, with every remediation step explicitly user-authorized after investigation, never assumed or unilaterally applied.

## Issues Encountered

- **Both background `gh run watch` processes lost their notification path** (once per CI run). Each time, the coordinator independently checked `gh run view`/`gh api .../artifacts` and reported the terminal result; each time, I independently re-verified every claimed fact myself before acting on it (never took the coordinator's report at face value) -- all details matched exactly both times.
- **The evidence branch (`phase13-ci-evidence-78ebde6c`) was cut from the originally-approved SHA before the first evidence file/SUMMARY were ever created** (those were created and committed on `master` in the earlier session). This meant `evidence/windows-ci-run.json` and `13-35-SUMMARY.md` did not exist on this branch and had to be authored fresh here, incorporating the full history from the earlier `master`-branch work rather than editing forward from a checked-out copy.
- **The regenerated commit is a genuine, disclosed delta from the originally-approved SHA**, not a no-op -- see `implementationTree.deltaFromOriginallyApprovedSha` in the evidence file for the full, itemized accounting (exactly 24 baseline PNGs, zero production/workflow/dependency/build changes).

## Next Phase Readiness

**Evidence now exists for a genuinely passing Windows CI run**, but two things remain for the coordinator/user, not this plan:
1. **Reconcile master with the CI-proven tree.** `master`'s tip (`78ebde6c53e1...`) itself was never proven passing -- only the regenerated descendant (`c3fe6e72...`, currently only on PR #6 / the evidence branch) was. Plans 13-38/13-39 will need this reconciled (e.g. merging PR #6) before they can treat `master` as carrying proven Windows CI evidence.
2. **Decide what to do with PR #6.** Left open, not merged or closed, exactly as authorized -- it now carries the full two-run audit trail (failing run, investigation, regenerated baselines, passing run) and is a legitimate artifact of this process, not just a disposable trigger mechanism.

No baseline was left un-investigated, no fix was applied without explicit authorization, and no success was ever claimed without independent re-verification.

## Self-Check: PASSED

- `evidence/windows-ci-run.json` exists on branch `phase13-ci-evidence-78ebde6c` and is valid JSON (`node -e "JSON.parse(...)"` succeeds).
- `gh run view 30875348640` independently confirms `conclusion: success`, `headSha: c3fe6e726a5cba598be6d68f20ab4b276400cb1a`, matching the evidence file exactly.
- `gh api repos/lnorton89/golc/actions/runs/30875348640/artifacts` independently confirms `total_count: 0`, matching the evidence file's `artifacts: []` and `artifactsNote`.
- `git diff --name-only 78ebde6c53e1... c3fe6e726a5c...` returns exactly the 24 baseline PNG paths recorded in `implementationTree.deltaFromOriginallyApprovedSha.changedPaths`, nothing else.
- `npm run validate:phase13-evidence -- --evidence evidence/windows-ci-run.json` reports zero `implementation tree:` errors and zero `Windows CI:` "must be completed/success" or "missing a successful job record" errors (both genuinely pass); remaining FAILs are exactly the disclosed, pre-existing scope-mismatch categories (rows/calibration/masks/packagedWebView2/backstops/coverage, plus the disclosed artifacts[] schema gap).
- `master` was never pushed to or otherwise mutated directly at any point across either cycle; the only remote mutations were commits on the disposable branch `phase13-ci-evidence-78ebde6c` and its already-open PR #6, exactly as authorized both times.
- No protected paths (`go.mod`, `go.sum`, `.planning/STATE.md`, `.planning/ROADMAP.md`) were touched.

---
*Phase: 13-unified-ui-design-system-and-automated-enforcement*
*Completed: 2026-08-03*
