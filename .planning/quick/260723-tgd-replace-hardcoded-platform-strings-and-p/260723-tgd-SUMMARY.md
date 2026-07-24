---
phase: quick-platform-paths-and-project-root
plan: 260723-tgd
subsystem: bootstrap-command-delivery
tags: [go, powershell, platform-paths, node, process-environment]
requires:
  - plan: 260723-s7n
    provides: callable Go bootstrap engine and secure archive installation
  - plan: 260723-svv
    provides: strict schema-v2 platform archive pins
provides:
  - canonical runtime executable names and platform install paths
  - filesystem-shape discovery of verified Node installations
  - platform-neutral CLI inventory and platform-keyed foundation outputs
  - authoritative project-root propagation across every child process boundary
affects: [bootstrap-entrypoint-removal, contributor-tooling, foundation-packaging, linear-sync]
tech-stack:
  added: []
  patterns:
    - derive executable paths from bootstrap.PlatformKey and ExecutableName
    - replace stale environment keys case-insensitively at process boundaries
key-files:
  created:
    - cmd/golc-project/main_test.go
    - internal/bootstrap/scope_registration_test.go
  modified:
    - internal/bootstrap/engine.go
    - internal/command/test.go
    - internal/delivery/graph.go
    - internal/trace/transport/process.go
    - golc.ps1
key-decisions:
  - "commands.cli_binary remains a repository-relative install root; consumers expose the resolved current-platform executable."
  - "Verified Node payloads are discovered from one real top-level directory plus the install manifest, never reconstructed from archive naming."
  - "Node test registrations retain arguments only and resolve the pinned executable from Request.Root at execution time."
  - "Bootstrap quick-scope declarations live in an external test package to keep delivery -> bootstrap from forming a test-only import cycle."
patterns-established:
  - "Runtime tool paths: <install-root>/<PlatformKey>/bin/<ExecutableName>."
  - "Child environments: preserve explicit values, remove case-insensitive root duplicates, append one canonical GOLC_PROJECT_ROOT."
requirements-completed: [CONF-01, CONF-02, CONF-03]
coverage:
  - id: D1
    description: "Platform-native Go, Node, CLI, and foundation paths derive from one runtime platform contract."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "go test ./internal/bootstrap ./internal/command ./internal/delivery -count=1"
        status: pass
      - kind: integration
        ref: "CGO_ENABLED=0 cross-builds for linux/amd64 and darwin/arm64"
        status: pass
    human_judgment: false
  - id: D2
    description: "Entrypoints and all Go, Node, bootstrap, and Linear children receive one normalized project root."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "go test ./cmd/golc-project ./internal/bootstrap ./internal/command ./internal/trace/transport -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Windows behavior and immutable toolchain/dependency pins remain unchanged."
    requirement: CONF-03
    verification:
      - kind: integration
        ref: "go test ./... -count=1 and SHA-256 pin audit"
        status: pass
    human_judgment: false
duration: 14min
completed: 2026-07-23
status: complete
---

# Quick Plan 260723-tgd: Platform Paths and Project Root Summary

**Runtime-keyed executable and foundation paths with strict Node discovery and deterministic `GOLC_PROJECT_ROOT` propagation across bootstrap, command, and Linear process boundaries.**

## Performance

- **Duration:** 14 min
- **Started:** 2026-07-24T04:21:05Z
- **Completed:** 2026-07-24T04:34:20Z
- **Tasks:** 3 TDD tasks
- **Files modified:** 23

## Accomplishments

- Added safe executable naming, platform command paths, and strict verified Node payload discovery without changing the Windows-only archive pins.
- Changed `commands.cli_binary` to a platform-neutral install root and resolved the CLI consistently for commands, foundation inventory, artifacts, and checksums.
- Established one absolute project root in PowerShell and Go entrypoints, then replaced stale inherited values across Go, Node, bootstrap, and Linear child environments.
- Preserved current Windows paths while proving the headless CLI compiles for linux/amd64 and darwin/arm64.

## Task Commits

1. **Task 1 RED:** `b704b68` — platform path and Node discovery contract tests.
2. **Task 1 GREEN:** `d00abd0` — canonical executable helpers and Node installation discovery.
3. **Task 2 RED:** `112a2e3` — platform-neutral command/delivery expectations.
4. **Task 2 GREEN:** `271dc47` — resolved command inventory, foundation paths, and Node consumers.
5. **Task 3 RED:** `e82d8eb` — project-root propagation tests.
6. **Task 3 GREEN:** `1abc22c` — entrypoint and child-process root propagation.
7. **Rule 1 fix:** `e024305` — aligned the security foundation fixture.

## TDD Gate Compliance

- Task 1 RED failed on the absent exported path/discovery contracts; GREEN passed both bootstrap scopes.
- Task 2 RED failed on the raw CLI root and reconstructed Node path; GREEN passed projectconfig, command, delivery, and transport.
- Task 3 RED failed on absent entrypoint/environment/process contracts; GREEN passed the CLI, bootstrap, command, and process suites.
- Every RED commit precedes its corresponding GREEN commit.

## Verification

- `go test ./internal/bootstrap ./internal/projectconfig ./internal/command ./internal/delivery ./internal/trace/transport ./cmd/golc-project -count=1` — PASS.
- `go test ./... -count=1` — PASS on Windows.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/golc-project` — PASS.
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./cmd/golc-project` — PASS.
- PowerShell syntax parse for `golc.ps1` — PASS.
- Platform-literal audit — PASS; remaining values are explicit Windows pin/update fixtures or Windows expected values.
- SHA-256 hashes for `config/toolchain.toml`, `go.mod`, `go.sum`, and both Linear npm manifests are byte-identical to the pre-execution baseline.
- Protected sketch/`260723-sgy` diff audit — no executor-authored changes.

## Decisions Made

- Unsafe executable base names return an empty helper result, preserving the plan’s fixed string-returning interface while rejecting traversal and path components.
- `ResolveNodeInstallation` requires the regular install manifest and exactly one real top-level directory, rejecting symlinks and unexpected files before executable use.
- `ProcessConfig.ProjectRoot` must already be absolute and normalized; it is inserted into a copied explicit environment without implicit inheritance.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Relocated bootstrap quick-scope declarations**

- **Found during:** Task 2 GREEN
- **Issue:** The required `delivery -> bootstrap` helper dependency closed a test-only cycle because internal bootstrap tests imported `command`.
- **Fix:** Moved the same four declarations into `scope_registration_test.go` under external package `bootstrap_test`; scope names and marker tests are unchanged.
- **Files modified:** `internal/bootstrap/bootstrap_test.go`, `internal/bootstrap/engine_test.go`, `internal/bootstrap/engine_linear_test.go`, `internal/bootstrap/scope_registration_test.go`
- **Verification:** Focused bootstrap, command, and delivery suites pass.
- **Committed in:** `271dc47`

**2. [Rule 3 - Blocking] Updated the foundation golden manifest**

- **Found during:** Task 2 GREEN
- **Issue:** The canonical CLI archive path and fixture configuration bytes changed by design, making the committed golden stale.
- **Fix:** Updated only the resolved Windows CLI path and the corresponding commands fixture hash/size.
- **Files modified:** `tests/golden/foundation-manifest.json`
- **Verification:** Delivery golden and deterministic bundle tests pass.
- **Committed in:** `271dc47`

**3. [Rule 1 - Bug] Aligned the security foundation fixture**

- **Found during:** Full-suite verification
- **Issue:** The security canary fixture still supplied an executable as `commands.cli_binary`, causing the new resolver to append a second platform path.
- **Fix:** Stored the platform-neutral install root and created the fixture payload at the resolved runtime path.
- **Files modified:** `internal/security/redact_test.go`
- **Verification:** `go test ./internal/security -count=1` and `go test ./... -count=1` pass.
- **Committed in:** `e024305`

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking issues). All were direct correctness requirements caused by the planned contract change.

## Known Stubs

None.

## Threat Flags

None — executable consumption, Node filesystem discovery, project-root inheritance, and the explicit Linear adapter environment were all declared in the plan threat model and retain their specified mitigations.

## Issues Encountered

Concurrent `260723-sgy` commits landed between this task’s atomic commits. They were preserved and never staged by this executor.

## User Setup Required

None.

## Self-Check: PASSED

All created files and the summary exist, all seven scoped commits resolve, the full verification suite passes, and the worktree contains only the intentionally uncommitted plan/summary directory.
