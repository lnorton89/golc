---
phase: 01-offline-foundation-and-delivery-traceability
plan: 01
subsystem: testing
tags: [powershell, toml, acceptance, sha256, offline]

requires: []
provides:
  - Red/bootstrap/green contributor acceptance harness with exact failure classification
  - Checksum-first local toolchain archive fixture contract
  - Strict data-only TOML fixture repository for runtime inspection
affects: [01-16, 01-17, bootstrap, project-configuration]

tech-stack:
  added: []
  patterns:
    - Data-only fixture checkout with repository-owned command injection
    - SHA-256 calculation before bootstrap source metadata is exposed
    - Raw-byte comparison for deterministic JSON acceptance

key-files:
  created:
    - tests/acceptance/walking-skeleton.ps1
    - tests/fixtures/config/walking-skeleton/golc.project.toml
    - tests/fixtures/config/walking-skeleton/config/toolchain.toml
    - tests/fixtures/config/walking-skeleton/config/runtime.toml
  modified: []

key-decisions:
  - "Acceptance fixtures are data-only and restricted to the three expected TOML files; only the repository-owned root command may be executed."
  - "Bootstrap fixture metadata is populated only after hashing a locally built archive, and green acceptance compares raw output bytes."

patterns-established:
  - "Failure classification: missing golc.ps1 is distinct from fixture, shell, syntax, and nonzero command failures."
  - "Temporary acceptance state: all copied configuration, archives, and captured output are removed in finally blocks."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "Red mode succeeds only for the specifically absent repository-owned golc.ps1 command."
    requirement: CONF-01
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\walking-skeleton.ps1 -Mode red"
        status: pass
    human_judgment: false
  - id: D2
    description: "Bootstrap mode creates and hashes a local archive before invoking the root bootstrap command."
    requirement: CONF-03
    verification:
      - kind: other
        ref: "bootstrap-mode expected-failure probe reached ROOT_COMMAND_MISSING only after checksum fixture construction"
        status: pass
    human_judgment: false
  - id: D3
    description: "Green mode requires two runtime.log_level JSON reads to be byte-identical and equal to the committed value."
    requirement: CONF-03
    verification:
      - kind: other
        ref: "tests/acceptance/walking-skeleton.ps1#green raw-byte and runtime.log_level assertions"
        status: pass
    human_judgment: false
  - id: D4
    description: "The copied fixture repository accepts only the root index, toolchain concern, and runtime concern with no links or credential inputs."
    requirement: CONF-01
    verification:
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\walking-skeleton.ps1 -Mode red"
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 1: Walking Skeleton Acceptance Contract Summary

**PowerShell clean-checkout contract with exact missing-command classification, checksum-first bootstrap input, and byte-stable TOML runtime inspection**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-20T04:03:25Z
- **Completed:** 2026-07-20T04:10:31Z
- **Tasks:** 1
- **Files modified:** 4

## Accomplishments

- Added a self-contained `red|bootstrap|green` acceptance harness that copies a strict data-only fixture repository into a temporary checkout.
- Distinguished the intentionally absent root command from fixture, copy, PowerShell syntax, and present-but-failing command errors.
- Defined checksum-before-use bootstrap metadata and byte-identical repeated inspection of committed `runtime.log_level=info` JSON.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write the failing clean-checkout contributor test** - `e5cba74` (test)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `tests/acceptance/walking-skeleton.ps1` - Red/bootstrap/green clean-checkout acceptance contract and cleanup boundary.
- `tests/fixtures/config/walking-skeleton/golc.project.toml` - Root schema and fixed concern index.
- `tests/fixtures/config/walking-skeleton/config/toolchain.toml` - Exact local archive URI and SHA-256 injection points.
- `tests/fixtures/config/walking-skeleton/config/runtime.toml` - Committed `runtime.log_level=info` fixture value.

## Decisions Made

- Kept fixtures data-only and rejected unexpected files or reparse points so fixture input cannot supply executable code.
- Copied only the repository-owned `golc.ps1` into the temporary checkout, making a present but broken root command a real failure rather than an expected RED state.
- Compared Base64 encodings of captured raw output bytes so repeated JSON equality is byte-level, not object-level.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Moved the default fixture path out of parameter binding**
- **Found during:** Task 1 verification
- **Issue:** Windows PowerShell 5.1 evaluated the parameter default before `$PSScriptRoot` was available, producing an unrelated `Join-Path` failure.
- **Fix:** Made `-FixtureRoot` optional at binding time and resolved its repository-relative default immediately after script initialization.
- **Files modified:** `tests/acceptance/walking-skeleton.ps1`
- **Verification:** The exact RED command exits 0 under `powershell.exe`, and the parser reports no syntax errors.
- **Committed in:** `e5cba74`

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug)
**Impact on plan:** The compatibility correction preserves the planned public parameter and exact failure classification without expanding scope.

## Issues Encountered

None beyond the auto-fixed Windows PowerShell parameter-binding issue above.

## Known Stubs

- `golc.ps1` is intentionally absent in this RED plan; Plan 01-16 owns its checksum-verifying bootstrap implementation.
- `__GOLC_FIXTURE_ARCHIVE_URI__` and `__GOLC_FIXTURE_ARCHIVE_SHA256__` in `config/toolchain.toml` are intentional test injection markers replaced only in the temporary checkout before bootstrap.
- Bootstrap and green modes are executable acceptance contracts that remain red until Plans 01-16 and 01-17 provide their respective implementations.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 01-16 can implement the root bootstrap shim against the checksum-first archive contract.
- Plan 01-17 can implement `config inspect runtime --format json` against the deterministic green-mode contract.
- No credentials, registry access, Linear state, UI, SQLite, or product packaging were introduced.

## Self-Check: PASSED

- All four task files and this summary exist on disk.
- Task commit `e5cba74` exists in git history.
- The exact RED verification command passes from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
