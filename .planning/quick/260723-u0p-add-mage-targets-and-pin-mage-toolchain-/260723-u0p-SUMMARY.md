---
phase: quick-mage-targets-and-toolchain
plan: 260723-u0p
subsystem: developer-tooling
tags: [mage, bootstrap, toolchain, delivery, tdd]

requires:
  - phase: quick-260723-tgd
    provides: Cross-platform platform keys and propagated project root
provides:
  - Exact five-platform Mage 1.17.2 toolchain authority
  - Verified Mage installation and project-local executable discovery
  - Strict configured PR graph loader
  - Ten Go-native Mage targets
affects: [developer-entrypoint, bootstrap, pr-delivery, powershell-removal]

tech-stack:
  added: [Mage 1.17.2 development executable]
  patterns: [required strict configuration keys, Go-native target composition, configured serial delivery graph]

key-files:
  created: [magefiles/magefile.go, magefiles/magefile_test.go]
  modified:
    - config/toolchain.toml
    - internal/projectconfig/model.go
    - internal/projectconfig/decode.go
    - internal/projectconfig/strict_test.go
    - internal/bootstrap/engine.go
    - internal/bootstrap/engine_test.go
    - golc.ps1
    - tests/acceptance/bootstrap.ps1
    - internal/delivery/graph.go
    - internal/delivery/delivery_test.go

key-decisions:
  - "Mage configuration keys are individually required through a generic opt-in KeySpec.Required contract; unrelated keys remain optional."
  - "Mage targets compose bootstrap, command registry, and delivery APIs directly with one normalized GOLC_PROJECT_ROOT."
  - "PR ordering remains solely in commands.pr.steps and is executed serially through delivery.Run."

patterns-established:
  - "Pinned executable discovery requires the committed platform pin, matching live install inventory, and a regular non-symlink executable."
  - "Mage target functions use fixed argument vectors and never create a process or shell boundary."

requirements-completed: [CONF-01, CONF-02, CONF-03]

coverage:
  - id: D1
    description: "Mage 1.17.2 is strictly pinned to five exact official release assets and digests."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "go test ./internal/projectconfig -run '^TestScopeConfigStrict$' -count=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "Go and retained PowerShell bootstrap paths install and verify the current-platform Mage executable."
    requirement: CONF-01
    verification:
      - kind: integration
        ref: "go test ./internal/bootstrap -run '^TestScopeBootstrapEngine$' -count=1"
        status: pass
      - kind: e2e
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\bootstrap.ps1 -Mode local"
        status: pass
    human_judgment: false
  - id: D3
    description: "Ten Go-native Mage targets use exact command mappings and authoritative serial PR ordering."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "go test ./internal/delivery ./magefiles -count=1"
        status: pass
      - kind: integration
        ref: ".tools/toolchains/mage/1.17.2/windows-amd64/mage.exe -l"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-07-23
status: complete
---

# Quick Plan 260723-u0p: Mage Targets and Toolchain Summary

**Checksum-pinned Mage 1.17.2 bootstrap and discovery with ten Go-native targets driven by the shared command registry and configured PR graph**

## Performance

- **Duration:** 13 min
- **Started:** 2026-07-24T04:46:29Z
- **Completed:** 2026-07-24T04:59:44Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments

- Added the exact official Mage 1.17.2 URL and SHA-256 authority for Windows AMD64, Linux AMD64/ARM64, and Darwin AMD64/ARM64 without changing the existing Go or Node pins.
- Added checksum-first Mage bootstrap installation, source-free repeat behavior, and manifest/file-integrity discovery with no PATH fallback.
- Added the ten required Mage targets with exact route mappings and serial `Pr` execution sourced only from strict `commands.pr.*` configuration.

## Task Commits

1. **Task 1 RED: strict Mage authority tests** - `cfef0c3`
2. **Task 1 GREEN: exact five-platform authority** - `086d054`
3. **Task 2 RED: Mage bootstrap/discovery tests** - `d29a121`
4. **Task 2 GREEN: verified install and discovery** - `38f2fcd`
5. **Task 3 RED: target and PR graph tests** - `81c869d`
6. **Task 3 GREEN: Go-native targets and graph loader** - `2bcab1f`
7. **Verification fix: quoted platform parsing** - `afc4623`

Plan metadata remains uncommitted for the parent GSD workflow to finalize.

## Files Created/Modified

- `config/toolchain.toml` - Exact Mage 1.17.2 parent and five platform pins.
- `internal/projectconfig/model.go` - Mage value constraints and opt-in required-key declarations.
- `internal/projectconfig/decode.go` - Generic required-key enforcement.
- `internal/projectconfig/strict_test.go` - Exact authority inventory and rejection cases.
- `internal/bootstrap/engine.go` - Mage layouts, installation, and `ResolveMageExecutable`.
- `internal/bootstrap/engine_test.go` - Archive, install, tamper, repeat, and discovery tests.
- `golc.ps1` - Narrow first-bootstrap Mage installation compatibility.
- `tests/acceptance/bootstrap.ps1` - Offline local Mage ZIP acceptance coverage.
- `internal/delivery/graph.go` - Strict `LoadPRGraph` implementation.
- `internal/delivery/delivery_test.go` - Config-order and PR policy tests.
- `magefiles/magefile.go` - Ten Go-native target functions.
- `magefiles/magefile_test.go` - Mapping, root, output, failure, ordering, and AST tests.

## Decisions Made

- Required-key validation is generic and opt-in at `KeySpec`, avoiding Mage-specific decoder logic and preserving all prior optional-key behavior.
- `Pr` builds one command registry, loads one strict graph, and invokes `delivery.Run` once; bootstrap is the sole specially dispatched Go API node.
- A stale `GOLC_PROJECT_ROOT` is ignored unless it resolves to a repository marker; the working directory is then searched upward and the normalized absolute root is re-established.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added generic required-key enforcement**

- **Found during:** Task 1
- **Issue:** Strict validation rejected unknown and malformed keys but allowed registered keys to be omitted, so an incomplete Mage platform pin group could pass.
- **Fix:** Added opt-in `KeySpec.Required` enforcement and marked all thirteen Mage authority keys required.
- **Files modified:** `internal/projectconfig/model.go`, `internal/projectconfig/decode.go`, `internal/projectconfig/strict_test.go`
- **Verification:** Focused strict-config tests cover an omitted Mage digest and prove unrelated optional specs remain valid.
- **Committed in:** `086d054`

**2. [Rule 1 - Bug] Parsed all committed quoted Mage platform tables**

- **Found during:** Final production bootstrap verification
- **Issue:** The retained PowerShell TOML reader recognized quoted platform tables only for its selected Windows record and rejected the committed Linux/Darwin Mage tables before selection.
- **Fix:** Generalized quoted platform parsing for the allowed tool names and safe platform-key grammar while retaining Windows-only installation selection.
- **Files modified:** `golc.ps1`, `tests/acceptance/bootstrap.ps1`
- **Verification:** Local acceptance includes an unselected Linux Mage table and passes both first-install and source-free repeat checks.
- **Committed in:** `afc4623`

---

**Total deviations:** 2 auto-fixed (1 missing critical functionality, 1 bug).
**Impact on plan:** Both changes are required to satisfy strict atomic pins and make the retained Windows bootstrap consume the five-platform authority. No Step 6+ work was introduced.

## Issues Encountered

- A diagnostic production `golc.ps1 bootstrap` run installed Mage successfully, then the pre-existing Go module phase detected an attempted `go.sum` expansion and failed closed with `GOLC_BOOTSTRAP_LOCK_MUTATION`. The tracked `go.sum` was restored byte-for-byte; the plan's offline local acceptance and all Go tests pass.
- Mage's `-l` display preserves CamelCase for multiword Go function names. Target matching is case-insensitive; normalizing the listing confirms all required lowercase CLI spellings are discoverable.

## Verification

- `go test ./internal/projectconfig ./internal/bootstrap ./internal/delivery ./magefiles -count=1` - PASS
- `go test ./internal/delivery -run '^TestScopeDelivery$' -count=1` - PASS
- `go test ./... -count=1` - PASS
- `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1 -Mode local` - PASS
- Resolved Mage 1.17.2 `-l`, normalized case, lists all ten required targets - PASS
- `go.mod`, `go.sum`, Linear SDK `package.json`, and `package-lock.json` match their recorded pre-task SHA-256 values - PASS
- `.github`, `internal/command`, and `config/commands.toml` have no executor-authored changes - PASS

## Known Stubs

None.

## User Setup Required

None.

## Next Phase Readiness

- Step 4 and Step 5 are complete; later PowerShell-removal steps can consume the verified Mage entrypoint.
- No workflow, command-parity, launcher-removal, CI fallback, or product packaging work from Step 6+ was implemented.

## Self-Check: PASSED

- All created key files exist.
- All seven task/deviation commits exist.
- Every automated verification listed above passed.

---
*Plan: 260723-u0p*
*Completed: 2026-07-23*
