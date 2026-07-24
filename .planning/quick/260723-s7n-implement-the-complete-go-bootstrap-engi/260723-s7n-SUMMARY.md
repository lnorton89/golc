---
phase: quick-go-bootstrap-engine
plan: 260723-s7n
subsystem: bootstrap
tags: [go, bootstrap, archives, toolchains, npm, security]
requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    provides: checksum pins, project-local caches, bootstrap command and Linear tooling contracts
provides:
  - Secure ZIP and tar.gz staged installation with versioned per-file manifests
  - Platform-aware callable Go bootstrap engine with injected source and process seams
  - Optional exact-lock Node/npm/TypeScript Linear workspace bootstrap
affects: [bootstrap-entrypoint-removal, contributor-tooling, linear-sync]
tech-stack:
  added: []
  patterns:
    - Fully inspect checksum-pinned archives before staging and accept only regular files/directories
    - Validate live installed bytes and POSIX modes before trusting a no-op manifest
    - Run pinned child executables through argument-vector and explicit-environment seams
key-files:
  created:
    - internal/bootstrap/engine.go
    - internal/bootstrap/engine_test.go
    - internal/bootstrap/engine_linear.go
    - internal/bootstrap/engine_linear_test.go
  modified:
    - internal/bootstrap/archive.go
    - internal/bootstrap/bootstrap.go
    - internal/bootstrap/bootstrap_test.go
    - internal/bootstrap/cache.go
    - internal/bootstrap/downloader.go
key-decisions:
  - "Install manifests use schema version 1 with sorted normalized path, lowercase SHA-256, and four-digit ordinary permission mode for every regular file."
  - "Go and Node archive filenames and extracted roots are derived from explicit pure OS/architecture mappings while PlatformKey remains exactly runtime.GOOS-runtime.GOARCH."
  - "Go and npm lock inputs are restored to their original raw bytes before returning a stable mutation diagnostic."
requirements-completed: [CONF-01, CONF-03]
duration: ~18min
completed: 2026-07-23
status: complete
---

# Quick Task 260723-s7n: Complete Go Bootstrap Engine Summary

**A callable, platform-aware bootstrap engine now securely installs ZIP/tar.gz pins, warms and verifies the Go toolchain/module graph, builds `golc-project`, and optionally provisions Linear tooling through exact-lock npm/TypeScript without host-tool fallback.**

## Performance

- **Duration:** ~18 min
- **Completed:** 2026-07-23
- **Tasks:** 3 TDD tasks
- **Files modified:** 9

## Accomplishments

- Added full ZIP and gzip-tar pre-inspection, shared path normalization, duplicate/link/special-entry rejection, staged extraction, ordinary POSIX permission preservation, rollback-safe promotion, and strict versioned install manifests whose live inventory must match before acquisition is skipped.
- Added `PlatformKey`, public `Bootstrap`, strict current-manifest parsing, contained cache overrides, sorted generic-tool provisioning, content-addressed verified cache reuse, local-file/allowlisted-HTTPS acquisition, redirect revalidation, pinned Go execution, exact module pin assertions, LF module recording, bootstrap probe, and platform-correct `-trimpath` project build.
- Added the opt-in Linear bootstrap unit using the pinned Node layout, exact `npm ci --ignore-scripts --no-audit --no-fund`, pinned TypeScript compiler, required output checks, lock restoration, and a strict npm-ci manifest that makes a matching repeat perform zero npm/tsc/source calls.
- Added fully offline registered test scopes using generated archives, synthetic repositories, a counting source, and injected process runners; no test invokes a real package manager or opens a live network connection.

## Task Commits

1. **Task 1 RED:** `4f1f80f` — archive and install-manifest contract tests.
2. **Task 1 GREEN:** `65b30d0` — secure ZIP/tar.gz installs and versioned manifest cutover.
3. **Task 2 RED:** `a119c20` — platform-aware Go engine contract tests.
4. **Task 2 GREEN:** `ab38fc9` — complete generic-tool and Go bootstrap engine.
5. **Task 3 RED:** `cbd795a` — optional Linear npm/TypeScript contract tests.
6. **Task 3 GREEN:** `33b1242` — pinned Node, exact-lock npm ci, tsc, and repeat manifest.

## Verification

- `powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-archive` — PASS
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-engine` — PASS
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-linear-sync` — PASS
- `powershell -NoProfile -File .\golc.ps1 build` — PASS
- Forbidden-scope diff audit covering `golc.ps1`, configuration, projectconfig, `cmd/golc-project`, module locks, Linear workspace sources, acceptance scripts, docs, and README — no executor-created changes.

## Deviations from Plan

None — the plan was executed within its declared implementation/test files and dependency constraints.

## Known Stubs

None.

## Threat Flags

None — the new archive, network, cache, process, platform, and npm trust-boundary surfaces are all declared in the plan threat model and covered by the specified mitigations and offline tests.

## Self-Check: PASSED

- All nine owned implementation/test files exist.
- Commits `4f1f80f`, `65b30d0`, `a119c20`, `ab38fc9`, `cbd795a`, and `33b1242` exist in repository history.
- No tracked file was deleted by any task commit.
- All three quick scopes and the supported root build pass.
- The unrelated quick-task directory and concurrent Phase 6/planning work were preserved.
