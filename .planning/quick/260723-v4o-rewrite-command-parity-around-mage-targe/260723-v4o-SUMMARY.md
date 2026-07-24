---
phase: quick-mage-command-parity
plan: 260723-v4o
subsystem: developer-tooling
tags: [mage, ci, parity, security, tdd]

requires:
  - phase: quick-260723-u0p
    provides: Shared Mage target registry and strict configured PR graph loader
provides:
  - Descriptor-backed closed-world PR workflow parity validation
  - Cross-platform direct Go parity acceptance coverage
  - Six-target Mage pull-request workflow
affects: [developer-entrypoint, pr-delivery, powershell-removal]

tech-stack:
  added: []
  patterns: [configured graph versus shared descriptor comparison, closed-world workflow run scanning, in-process acceptance testing]

key-files:
  created: [internal/command/check_test.go]
  modified:
    - internal/command/check.go
    - config/commands.toml
    - .github/workflows/check.yml
  deleted:
    - tests/acceptance/command-parity.ps1

key-decisions:
  - "Expected PR behavior comes only from delivery.LoadPRGraph; workflow target behavior comes only from delivery.LookupMageTarget."
  - "Every workflow run directive is closed-world and must be exactly one argument-free Mage target invocation."
  - "Command-parity acceptance runs directly in Go and is independent of PowerShell, Mage execution, golc.ps1, and process credentials."

requirements-completed: [CONF-03, CONF-04, LINR-04]

coverage:
  - id: D1
    description: "PR workflow parity resolves shared Mage descriptors to the exact configured graph while preserving least-privilege scans."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "go test ./internal/command -run '^TestScopeCommandParity$' -count=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "The committed pull-request workflow invokes the six locked Mage targets with no extra executable or credential-bearing surface."
    requirement: CONF-04
    verification:
      - kind: integration
        ref: "go test ./internal/command ./internal/delivery ./magefiles ./tools/golc-mcp -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Command-parity acceptance is cross-platform and does not depend on Linear credentials or a child process."
    requirement: LINR-04
    verification:
      - kind: unit
        ref: "internal/command/check_test.go#TestScopeCommandParity"
        status: pass
      - kind: regression
        ref: "go test ./... -count=1"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-07-23
status: complete
---

# Quick Plan 260723-v4o: Mage Command Parity Summary

**Closed-world pull-request parity now compares the strict configured command graph with shared Mage descriptors entirely in-process**

## Performance

- **Duration:** 10 min
- **Started:** 2026-07-24T05:29:00Z
- **Completed:** 2026-07-24T05:39:08Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Replaced PowerShell-specific workflow extraction with strict parsing of every `run:` directive, case-insensitive shared Mage target lookup, generic route/argument projection, and exact comparison to `delivery.LoadPRGraph`.
- Preserved forbidden-trigger, required-trigger, credential, secret, and Linear mutation scans while rejecting unknown, recursive, argument-bearing, shell-composed, and otherwise undeclared executable lines.
- Migrated the committed pull-request workflow to the six locked Mage targets and replaced the Windows-only acceptance wrapper with direct cross-platform Go coverage.

## Task Commits

1. **Task 1 RED: Mage registry and parity failure coverage** - `155340e`
2. **Task 1 GREEN: descriptor-backed closed-world parity** - `7f7bdce`
3. **Task 2 RED: production workflow acceptance gate** - `cdf4af6`
4. **Task 2 GREEN: six-target Mage workflow migration** - `fbab0bd`

Plan metadata remains uncommitted for the parent GSD workflow to finalize.

## Files Created/Modified

- `internal/command/check.go` - Strict Mage run parsing, shared descriptor projection, configured graph comparison, and least-privilege scans.
- `internal/command/check_test.go` - Synthetic and production-root direct Go parity coverage.
- `.github/workflows/check.yml` - Exact ordered Bootstrap, GenerateCheck, CheckOffline, Build, Test, and PackageFoundation Mage invocations.
- `config/commands.toml` - Comments name `LoadPRGraph`, the shared registry, and direct Go parity as authorities; all values are unchanged.
- `tests/acceptance/command-parity.ps1` - Removed after its assertions moved into cross-platform Go tests.

## Decisions Made

- Workflow target spelling is normalized once to match Mage's case-insensitive behavior; diagnostics retain original spelling and one-based position.
- Route-kind descriptors contribute their declared route and arguments, bootstrap contributes its registered target name, and PR-kind descriptors are rejected to prevent graph recursion.
- Run directives are validated before token scans and graph counts, ensuring an undeclared executable line receives a direct closed-world diagnostic.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Verification

- `go test ./internal/command -run '^TestScopeCommandParity$' -count=1` - PASS
- `go test ./internal/delivery ./magefiles ./tools/golc-mcp -count=1` - PASS
- `go test ./... -count=1` - PASS
- `git diff --check` - PASS
- Exact six ordered `run: mage <Target>` directives, pull-request-only trigger, `contents: read`, and `windows-latest` runner - PASS
- No matrix, credential, secret, Linear mutation, setup, installer, download, environment, or extra executable workflow surface - PASS
- All non-comment `config/commands.toml` lines are byte-for-byte unchanged - PASS
- `tests/acceptance/command-parity.ps1` is absent; every other acceptance script is unchanged - PASS
- `golc.ps1`, MCP, Mage, delivery, documentation, toolchain, and other excluded paths are unchanged - PASS
- The Go parity test contains no subprocess API - PASS

## Known Stubs

None.

## User Setup Required

None.

## Next Phase Readiness

- PowerShell-removal Step 6 is complete; Step 7 can remove or replace the retained launcher separately.
- No CI matrix, launcher removal, other acceptance migration, package-manager setup action, or broader platform claim was introduced.

## Self-Check: PASSED

- All created and modified key files exist, and the removed wrapper is absent.
- All four RED/GREEN task commits exist.
- Every automated verification and structural preservation check listed above passed.

---
*Plan: 260723-v4o*
*Completed: 2026-07-23*
