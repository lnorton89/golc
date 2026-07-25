---
phase: quick-final-powershell-removal-verification
plan: 260724-y2t
subsystem: developer-tooling
tags: [ci, powershell-removal, verification, bugfix]

requires:
  - phase: quick-260724-x7n
    provides: golc.ps1 deleted and every reference retired (not yet proven with real CI evidence)
provides:
  - check.yml's first-ever real, fully green pull_request-triggered run (30138546263)
  - linear-sync.yml's linear-drift job real, credential-free green run (30137404939)
  - cross-platform-mage.yml reconfirmed green on all three platforms against the final commit (30138724444)
  - A new mage TestQuick target closing a structural gap in commands.pr.steps that predated this entire migration
  - PowerShell removal plan Steps 0-9 fully complete
affects: [ci, powershell-removal]

tech-stack:
  added: []
  patterns: [open a throwaway empty-commit PR to give a pull_request-only workflow a real trigger, isolate GITHUB_EVENT_NAME in tests that exercise PR-guarded code paths, dedicated fixed-argument Mage targets for remote-failure-isolated variants of a route]

key-files:
  modified:
    - internal/contracts/linear_plan_test.go
    - internal/command/green_subprocess_test.go
    - internal/delivery/mage_targets.go
    - internal/delivery/delivery_test.go
    - magefiles/magefile.go
    - magefiles/magefile_test.go
    - config/commands.toml
    - .github/workflows/check.yml
    - internal/command/check_test.go
    - tools/golc-mcp/protocol_test.go
    - tools/golc-mcp/README.md

key-decisions:
  - "A throwaway PR (ci/verify-check-workflow, an empty commit against master) was opened specifically to give check.yml a real pull_request trigger: it has no other trigger, so there was no way to get real evidence without one. Rebased and re-pushed after each fix (three real runs total), then closed without merging and its branch deleted once verification completed -- never merged, no lasting trace in the repository beyond the fixes themselves."
  - "commands.pr.steps' bare 'test' step was a structural bug present since the file was first written, not something introduced by this session's work: bare test unconditionally requires tools/linear-sync's compiled Node output, but the PR graph's bootstrap step can never build it (D-16 forbids Linear credential/workspace access from untrusted pull-request CI). internal/delivery/graph.go's own offline core graph had already solved this exact problem with 'test --quick' -- the fix applies the same principle to the PR graph via a new dedicated testquick Mage target, rather than special-casing check.yml alone."
  - "cross-platform-mage.yml's own bare 'mage Test' step was deliberately left unchanged: that workflow's bootstrap does set GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC=1 at job scope, so the full Node-scope test suite genuinely can and should run there. The fix is specific to the credential-free PR graph, not a blanket replacement of 'test' with 'test --quick' everywhere."

requirements-completed: [CONF-01, CONF-02, CONF-03]

coverage:
  - id: D1
    description: "check.yml has a real, current, fully green pull_request-triggered run."
    requirement: CONF-01
    verification:
      - kind: manual
        ref: "check.yml run 30138546263 (pull_request trigger via throwaway PR)"
        status: pass
    human_judgment: true
  - id: D2
    description: "linear-sync.yml's linear-drift job has a real, current, green, credential-free run."
    requirement: CONF-02
    verification:
      - kind: manual
        ref: "linear-sync.yml run 30137404939 (workflow_dispatch)"
        status: pass
    human_judgment: true
  - id: D3
    description: "cross-platform-mage.yml still passes on all three platforms after every Step 7/Step 9 change."
    requirement: CONF-03
    verification:
      - kind: manual
        ref: "cross-platform-mage.yml run 30138724444 (workflow_dispatch)"
        status: pass
    human_judgment: true

duration: two fix rounds plus final multi-workflow confirmation, single session
completed: 2026-07-24
status: complete
---

# Quick Plan 260724-y2t: Final Byte-for-Byte Verification Across Three CI Workflows (PowerShell Removal Step 9) Summary

**check.yml ran for real for the first time in its history, surfaced two genuinely new bugs (a test-isolation gap and a structural PR-graph flaw predating this whole migration), both fixed with real evidence; all three CI workflows now have real, current, green runs against the final commit.**

## Performance

- **Rounds:** 2 fix rounds + 1 final confirmation round (3 real check.yml runs total via a throwaway PR)
- **Files modified:** 11
- **Completed:** 2026-07-24

## Accomplishments

- Discovered `check.yml` had literally zero prior runs (`gh run list --workflow=check.yml` returned nothing) despite having been rewritten to Mage in an earlier session — its `pull_request`-only trigger meant no PR had ever exercised it.
- Opened a throwaway empty-commit PR specifically to give it a real trigger; used it for three real runs, then closed it without merging.
- Round 1 fixed two bugs: a test that only passed by accident outside a real `pull_request` CI environment (`testLinearApplyFailsWithoutFactory`, missing the same `GITHUB_EVENT_NAME` isolation a sibling test already documented and used), and the same 8.3 short-name-aliasing class of bug from `260724-w3f`, now in a brand-new test (`green_subprocess_test.go`) added just one task earlier.
- Round 2 found and fixed a real structural bug in `config/commands.toml`'s `commands.pr.steps` that predated this entire session's work: bare `test` unconditionally runs Node-based Linear test scopes that the credential-free PR bootstrap can never build. Added a proper `testquick` Mage target (mirroring `internal/delivery/graph.go`'s own established `test --quick` remote-failure-isolation principle for the offline core graph) rather than a one-off workaround.
- Final round: `check.yml` went fully green for the first time ever (30138546263); re-confirmed `cross-platform-mage.yml` still green on all three platforms against the final commit (30138724444); `linear-sync.yml`'s `linear-drift` job was already confirmed green in Round 1's window (30137404939).

## Task Commits

1. **Round 1: two real bugs found by check.yml's first-ever real run** - `c0b95f2`
2. **Round 2: add mage TestQuick target for the PR graph's structural test-step bug** - `8658b32`

## Files Created/Modified

- `internal/contracts/linear_plan_test.go` - `testLinearApplyFailsWithoutFactory` isolates `GITHUB_EVENT_NAME`.
- `internal/command/green_subprocess_test.go` - subprocess `GOLC_PROJECT_ROOT` set explicitly, avoiding Windows short-name ambiguity.
- `internal/delivery/mage_targets.go` / `magefiles/magefile.go` - new `testquick` target and `TestQuick()` wrapper.
- `config/commands.toml` - `commands.pr.steps` uses `test --quick`.
- `.github/workflows/check.yml` - test step runs `mage TestQuick`.
- `internal/delivery/delivery_test.go`, `internal/command/check_test.go`, `magefiles/magefile_test.go`, `tools/golc-mcp/protocol_test.go`, `tools/golc-mcp/README.md` - updated to reflect the new target/PR-graph shape.

## Decisions Made

See `key-decisions` in the frontmatter above.

## Deviations from Plan

- No upfront plan existed; reconstructed after the fact per this repository's `.planning/quick/` convention. The work was inherently reactive to real CI failures across three sequential real runs of the same throwaway PR.

## Issues Encountered

- Two genuinely new bugs, both invisible until `check.yml` executed for real for the first time (see Accomplishments). Neither was a regression from this session's own Step 7 work — the `GITHUB_EVENT_NAME` isolation gap existed in code untouched by Step 7, and the `commands.pr.steps` structural flaw predates the entire PowerShell-removal effort.

## Verification

- `go test ./... -count=1` (normal and `GITHUB_EVENT_NAME=pull_request`) - PASS
- `go run ./cmd/golc-project check --command-parity` - PASS
- `check.yml` run 30138546263 - PASS (first-ever fully green real run)
- `linear-sync.yml` run 30137404939 (`linear-drift`) - PASS
- `cross-platform-mage.yml` run 30138724444 - PASS (all 3 platforms)

## Known Stubs

None.

## User Setup Required

None.

## Next Phase Readiness

- PowerShell removal plan (Steps 0-9) is fully complete. `golc.ps1` is deleted; Mage is the sole contributor entrypoint; every CI workflow that depends on the new build system has real, current, green evidence.
- No further PowerShell-removal work remains.

## Self-Check: PASSED

- Both commits exist on `master` and are pushed to `origin/master`.
- All three CI workflow runs cited are real, verifiable, and current against the final commit.
- The throwaway verification PR was closed without merging and its branch deleted.

---
*Plan: 260724-y2t*
*Completed: 2026-07-24*
