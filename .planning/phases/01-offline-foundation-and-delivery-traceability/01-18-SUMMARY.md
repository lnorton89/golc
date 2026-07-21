---
phase: 01-offline-foundation-and-delivery-traceability
plan: 18
subsystem: project-configuration
tags: [go, toml, five-layer-resolution, provenance, path-containment, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 03
    provides: Six-concern DefaultSpec single-authority registry, strict decode.go validation, cross-concern ref: resolution, registered config-strict quick-test scope
provides:
  - registry.go one-owner canonical key/reference graph (FieldSpec/Registry) declaring locked/sensitive disposition and allowlisted environment variable per key; DefaultRegistry keeps runtime.log_level as the sole writable key, locking every other DefaultSpec canonical key
  - resolve.go ResolveAll/ResolveKey implementing D-06 five-layer precedence (committed -> user %APPDATA%\GOLC\config.toml -> project-local golc.local.toml -> explicit environment allowlist -> typed CLI overrides) with GOLC_CONFIG_LOCKED_OVERRIDE rejecting every higher-layer attempt on a locked key
  - resolve.go ExplainRecord rendering deterministic winning/shadowed safe provenance with sensitive values reduced to <set>/<unset> across every layer
  - path.go ValidateConcernPath/ResolveContainedPath extending concern-file containment to indexed path values, tolerant of not-yet-created leaves (lazy cache directories)
  - Registered config quick-test scope (TestScopeConfig) covering five-layer precedence, locked-key rejection, provenance disclosure, and path containment
  - tests/golden/config-explain.json: byte-stable, credential-free provenance fixture for the production runtime.log_level committed value
affects: [01-19, project-configuration]

tech-stack:
  added: []
  patterns:
    - Five-layer resolution reuses existing single-package private helpers (local.go's readLocalValues/resolveCommittedOrigin, decode.go's validateLiteral) instead of duplicating parsing/validation logic
    - Locked-key rejection is fail-closed by design (GOLC_CONFIG_LOCKED_OVERRIDE) rather than silently ignoring a higher-layer override attempt
    - Path containment tolerates not-yet-created leaves by resolving symlinks only against the deepest existing ancestor, then re-validating the full candidate path against that resolution

key-files:
  created:
    - internal/projectconfig/registry.go
    - internal/projectconfig/resolve.go
    - internal/projectconfig/path.go
    - internal/projectconfig/resolve_test.go
    - internal/projectconfig/path_test.go
    - tests/golden/config-explain.json

key-decisions:
  - "DefaultRegistry() intentionally mirrors local.go's existing localKeyRegistry writable set (runtime.log_level only) rather than opening every DefaultSpec key to override, keeping the five-layer surface conservative and consistent with the already-proven project-local layer."
  - "The new quick-test scope is named exactly \"config\" per the plan, coexisting with the production CLI scope internal/command/config.go already declares under the same name — both declarations are safe because NewDefaultCommandRegistry (the only place a duplicate scope would be rejected) is never invoked by the quick-test dispatcher, which discovers TestScope{PascalName} markers via `go test -list` instead. Verified via the full `go test ./...` run (internal/command's own test binary builds and passes unaffected)."
  - "Project-local layer reuses local.go's readLocalValues() unmodified (files_modified excludes local.go); it enforces local.go's own independent lock list first, so a locked-key override attempt through golc.local.toml is rejected with GOLC_CONFIG_LOCAL_KEY_LOCKED/GOLC_CONFIG_LOCAL_KEY_UNKNOWN (defense-in-depth) rather than always surfacing GOLC_CONFIG_LOCKED_OVERRIDE specifically — the override is still rejected either way."
  - "User-layer and environment/CLI-layer locked-key rejection go through this plan's own registry.go/resolve.go code paths directly and consistently report GOLC_CONFIG_LOCKED_OVERRIDE."

patterns-established:
  - "GOLC_CONFIG_LOCKED_OVERRIDE: <layer> layer cannot override locked key <key> — stable diagnostic for any higher-layer attempt on a Locked FieldSpec, across user/environment/CLI."
  - "GOLC_CONFIG_USER_KEY_UNKNOWN / GOLC_CONFIG_USER_KEY_REDIRECT / GOLC_CONFIG_USER_VALUE_INVALID / GOLC_CONFIG_USER_READ: user-layer strictness diagnostics mirroring local.go's project-local strictness."
  - "GOLC_CONFIG_FIELD_UNKNOWN: resolving a key the registry does not declare at all."

requirements-completed: [CONF-01, CONF-02, CONF-04]

coverage:
  - id: D1
    description: "Five-layer precedence (committed -> user -> project-local -> environment -> CLI) resolves correctly for every adjacent layer pair via ResolveKey/ResolveAll"
    requirement: "CONF-01"
    verification:
      - kind: unit
        ref: "internal/projectconfig/resolve_test.go#TestScopeConfig/five-layer_precedence_resolves_every_adjacent_pair_in_order"
        status: pass
    human_judgment: false
  - id: D2
    description: "Locked keys reject every higher-layer override attempt (user, project-local, environment, CLI)"
    requirement: "CONF-02"
    verification:
      - kind: unit
        ref: "internal/projectconfig/resolve_test.go#TestScopeConfig/locked_keys_reject_every_higher-layer_override_attempt"
        status: pass
    human_judgment: false
  - id: D3
    description: "ExplainRecord renders deterministic winning/shadowed safe provenance and reduces sensitive values to <set>/<unset>"
    requirement: "CONF-04"
    verification:
      - kind: unit
        ref: "internal/projectconfig/resolve_test.go#TestScopeConfig/ExplainRecord_is_deterministic_and_renders_sensitive_declarations_as_set/unset_only"
        status: pass
      - kind: unit
        ref: "internal/projectconfig/resolve_test.go#TestScopeConfig/golden_explain_output_is_byte-stable_and_credential-free"
        status: pass
    human_judgment: false
  - id: D4
    description: "ResolveContainedPath/ValidateConcernPath reject absolute, parent, symlink, and reparse path escapes while tolerating not-yet-created leaves"
    requirement: "CONF-04"
    verification:
      - kind: unit
        ref: "internal/projectconfig/path_test.go#testPathContainment (run via TestScopeConfig/path_containment)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Registered config quick-test scope exits 0 through the pinned-toolchain shim: golc.ps1 test --quick --scope config"
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File golc.ps1 test --quick --scope config"
        status: pass
    human_judgment: false

duration: 32min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 18: Five-Layer Configuration Resolution and Safe Provenance Summary

**Five-layer configuration resolution (committed -> user -> project-local -> environment -> CLI) with fail-closed locked-key rejection, symlink/reparse-safe indexed path containment, and deterministic set/unset-safe provenance, all executable via `golc.ps1 test --quick --scope config`**

## Performance

- **Duration:** ~32 min (includes one-time pinned-toolchain bootstrap for this worktree)
- **Started:** 2026-07-21T00:07:00Z
- **Completed:** 2026-07-21T00:39:20Z
- **Tasks:** 1 (TDD)
- **Files modified:** 6

## Accomplishments

- `internal/projectconfig/registry.go` declares the one-owner canonical key/reference graph for five-layer overrides: `FieldSpec` (Locked/Sensitive/AllowedValues/EnvVar/CLIFlag) and `Registry`. `DefaultRegistry()` keeps `runtime.log_level` as the single writable production key — the same key local.go's project-local layer already allows — and locks every other `DefaultSpec` canonical key, so the five-layer surface never silently widens beyond what's already proven.
- `internal/projectconfig/resolve.go` implements `ResolveKey`/`ResolveAll`: committed values come from local.go's existing `resolveCommittedOrigin`; the project-local layer reuses local.go's existing `readLocalValues` (golc.local.toml) unmodified; a new user layer reads `%APPDATA%\GOLC\config.toml` with the same strict schema-version/unknown-key/locked-key/allowed-value discipline; the environment layer consults only the exact allowlisted `FieldSpec.EnvVar` name (never a broad `os.Environ()` scan); the CLI layer reads an already-typed `Sources.CLIOverrides` map (never raw argv). Any higher layer that declares a value for a `Locked` key fails resolution with `GOLC_CONFIG_LOCKED_OVERRIDE` instead of being silently ignored.
- `ExplainRecord` renders allowlisted-field-only JSON (`key`, `layer`, `source`, `value`, `shadowed`) with sorted keys and a single trailing newline; when a `FieldSpec` is `Sensitive`, both the winning value and every shadowed origin's value render `<set>`/`<unset>` instead of the literal, so a sensitive value can never leak through any layer's provenance.
- `internal/projectconfig/path.go` adds `ValidateConcernPath` (the exact lexical containment check load.go already applies to concern files, reused verbatim) and `ResolveContainedPath`, which walks up to the deepest existing ancestor, resolves symlinks there, and re-validates the full candidate path against that resolution — so a not-yet-created cache directory (bootstrap creates `.tools/cache/downloads` lazily) resolves cleanly while a symlinked ancestor that escapes the repository is still rejected.
- `internal/projectconfig/resolve_test.go` self-registers quick-test scope `config` beside `TestScopeConfig` (Plan 17 contract) and covers: every adjacent five-layer precedence pair with ordered shadowed-origin assertions; locked-key rejection from all four higher layers plus a defense-in-depth path through local.go's own independent lock list; allowed-value enforcement at the user and environment layers; unknown-user-key strictness; a missing (optional) user layer; an unregistered field; deterministic sensitive-safe `ExplainRecord` output; and the byte-stable golden comparison. `internal/projectconfig/path_test.go` contributes `testPathContainment`, pulled in as a subtest so one quick-test invocation exercises both files.
- `tests/golden/config-explain.json` is the credential-free, byte-stable `ExplainRecord` output for the production `runtime.log_level` committed value (`config/runtime.toml`), verified against a live `ResolveKey` call over the real repository root with environment/user layers forced empty for determinism.
- `powershell -NoProfile -File golc.ps1 test --quick --scope config` exits 0 from the repository root; `config-local` and `config-strict` (prior plans' scopes) and the full `go test ./...` / `go vet ./...` continue to pass unaffected.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: five-layer resolution, path containment, and provenance contract** - `229991a` (test)
2. **GREEN - Task 1: registry, resolver, and path containment implementation** - `8f31296` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/projectconfig/registry.go` - `FieldSpec`/`Registry` one-owner key/reference graph plus production `DefaultRegistry()`.
- `internal/projectconfig/resolve.go` - `Sources`, `ResolvedRecord`, `ResolveKey`, `ResolveAll`, `ExplainRecord`, and the new user-layer reader.
- `internal/projectconfig/path.go` - `ValidateConcernPath`, `ResolveContainedPath`.
- `internal/projectconfig/resolve_test.go` - Registers scope `config`/`TestScopeConfig`; five-layer precedence, locked-key rejection, provenance, and golden-comparison subtests.
- `internal/projectconfig/path_test.go` - `testPathContainment` helper covering lexical and symlink/reparse escape rejection plus not-yet-created-leaf tolerance.
- `tests/golden/config-explain.json` - Byte-stable credential-free provenance golden fixture.

## Decisions Made

- **Writable-key parity with the existing project-local layer:** `DefaultRegistry()` deliberately locks every `DefaultSpec` key except `runtime.log_level`, matching local.go's existing `localKeyRegistry` rather than opening new override surface the plan didn't explicitly request.
- **Scope name reuses "config":** the plan explicitly specifies quick-test scope `config`, which is the same normalized scope name `internal/command/config.go` already declares for the production CLI routes (`config inspect/set/explain`). This is safe because the two declarations are never combined inside a single `NewDefaultCommandRegistry()` build — the quick-test dispatcher discovers `TestScope{PascalName}` markers via `go test -list` and never builds the full command registry. Verified with a full `go test ./...` run across every package, including `internal/command`'s own registry-building tests.
- **Reuse over duplication:** resolve.go calls local.go's `readLocalValues`/`resolveCommittedOrigin` and decode.go's `validateLiteral` directly (same package) instead of re-implementing project-local parsing or value-shape validation, keeping this plan's `files_modified` scope exact (local.go/decode.go untouched) while staying behaviorally consistent with the layers those files already own.
- **Defense-in-depth on project-local locked-key rejection:** because local.go's `readLocalValues` validates every declared key against its own independent `localKeyRegistry` before resolve.go's registry-based check runs, a locked-key override attempt through `golc.local.toml` is rejected by whichever layer recognizes it first (`GOLC_CONFIG_LOCAL_KEY_LOCKED`/`GOLC_CONFIG_LOCAL_KEY_UNKNOWN` from local.go, or `GOLC_CONFIG_LOCKED_OVERRIDE` from resolve.go for a key local.go doesn't recognize at all). Both outcomes reject the override; the test asserts on the shared "LOCKED" substring for this specific case and asserts the stable `GOLC_CONFIG_LOCKED_OVERRIDE` code for the user/environment/CLI layers, which go through resolve.go's own check exclusively.

## Deviations from Plan

None - plan executed exactly as written. One environment note: this worktree had no bootstrapped pinned toolchain (`.tools/` did not exist yet), so `powershell -NoProfile -File golc.ps1 bootstrap` was run once before verification to provision it (network access to `go.dev` succeeded; a stale `.golc-staging-*` directory from a first `Access to the path ... is denied` attempt was cleared and bootstrap succeeded on retry). This is one-time worktree setup, not a plan deviation.

## Issues Encountered

- The first `golc.ps1 bootstrap` invocation failed extraction with `Access to the path ... is denied` on the staging directory (likely a transient file lock during zip extraction on this host). Retrying the idempotent bootstrap succeeded immediately; the pinned archive was already checksum-cached from the first attempt, so no second network call was needed for it.

## Known Stubs

None. `commands.cli_binary`/`check|generate|build|package|linear` route gaps predate this plan (carried from 01-17/01-03) and are out of this plan's scope.

## Threat Flags

None — no new network, process-execution, or credential surface. T-01-07 (resolver tampering) and T-01-08 (path escape) mitigations are implemented and test-locked in `resolve_test.go`/`path_test.go`; T-01-09 (provenance disclosure) is implemented and test-locked via `ExplainRecord`'s sensitive-value reduction and the credential-free golden fixture.

## User Setup Required

None - everything is repository-local or resolves to a fixed OS-standard path (`%APPDATA%\GOLC\config.toml`); no credentials, npm, or additional network access involved beyond the one-time toolchain bootstrap already required by prior plans.

## Next Phase Readiness

- `ResolveAll`/`ResolveKey`/`ExplainRecord` are ready for a future CLI-facing `config explain --all` or `--layer` surface without further resolver changes; only `internal/command/config.go` route wiring would be needed, and this plan intentionally left that file untouched.
- `ResolveContainedPath` is available for any future plan that needs to validate a committed path-typed value (cache directories, generated-output roots, mapping files) before using it, without re-deriving containment logic.
- `registry.go`'s `FieldSpec.CLIFlag`/`EnvVar` fields are already in place for a future CLI flag parser or documented environment-variable table to bind against.

## Self-Check: PASSED

- All six created files verified present on disk via direct filesystem check: `internal/projectconfig/registry.go`, `resolve.go`, `path.go`, `resolve_test.go`, `path_test.go`, `tests/golden/config-explain.json` (all reported `FOUND`).
- Commits `229991a` (test) and `8f31296` (feat) verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only 229991a~1 8f31296` reports zero deleted files across both commits; working tree clean before this summary.
- `powershell -NoProfile -File golc.ps1 test --quick --scope config`, `golc.ps1 test --quick --scope config-local`, `golc.ps1 test --quick --scope config-strict`, `golc.ps1 test --quick --scope linear-catalog`, the full pinned-toolchain `go test ./...`, and `go vet ./...` all exit 0 from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
