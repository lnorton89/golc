---
phase: 01-offline-foundation-and-delivery-traceability
plan: 03
subsystem: project-configuration
tags: [go, toml, strict-validation, single-authority, deprecation, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 02
    provides: Pure-library projectconfig (load/local), config CLI routes in internal/command/config.go, generic quick-test dispatcher, external-test-package scope pattern
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: MustDeclareScope/MustDeclareRoute registry contract and strict root-index/concern loading
provides:
  - Six-concern root index (toolchain, commands, generation, application_defaults, runtime, linear) with GOLC_CONFIG_INDEX_MISMATCH discovery enforcement
  - Typed Spec/ConcernSpec/KeySpec/Deprecation/Diagnostic model with production DefaultSpec single-authority key registry
  - Strict decoder distinguishing unknown, duplicate, invalid, deprecated-only, old-plus-new, duplicate-authority, unresolved, and cyclic inputs with stable codes
  - Typed cross-concern "ref:" values resolved at repository level so no authority literal is duplicated
  - Credential-free Linear integration concern (names/declarations only)
  - Registered config-strict quick-test scope beside TestScopeConfigStrict
affects: [01-18, 01-19, project-configuration]

tech-stack:
  added: []
  patterns:
    - Spec-driven strict decoding - production allocation lives in DefaultSpec; tests inject synthetic Specs for failure modes
    - Typed reference values ("ref:<canonical.key>") defer constraint checks to repository-level resolution, keeping single-file validation independent
    - Deprecations are machine-readable register entries validated by ValidateAuthority before any file is read

key-files:
  created:
    - internal/projectconfig/model.go
    - internal/projectconfig/decode.go
    - internal/projectconfig/strict_test.go
    - config/commands.toml
    - config/generation.toml
    - config/application-defaults.toml
    - config/integrations/linear.toml
  modified:
    - golc.project.toml
    - config/runtime.toml

key-decisions:
  - "Deprecation outcome codes use the plan-specified CFG_DEPRECATED_KEY / CFG_DEPRECATED_COLLISION spellings; every other new diagnostic keeps the established GOLC_CONFIG_* prefix."
  - "Concern id application_defaults uses an underscore so its canonical keys satisfy the dotted-lowercase key grammar shared with the local layer; the file keeps the researched application-defaults.toml name."
  - "Deprecated-only input applies its value to the replacement key alongside an explicit warning; the committed file is never rewritten (D-09 non-silent migration)."
  - "The production deprecation register is empty (greenfield); the register/decoder mechanics are proven by injected synthetic Specs in the strict scope."

patterns-established:
  - "Stable diagnostics: GOLC_CONFIG_DUPLICATE_KEY, GOLC_CONFIG_VALUE_INVALID, GOLC_CONFIG_DUPLICATE_AUTHORITY, GOLC_CONFIG_REF_UNRESOLVED, GOLC_CONFIG_REF_CYCLE, GOLC_CONFIG_INDEX_MISMATCH, GOLC_CONFIG_DEPRECATION_INVALID, CFG_DEPRECATED_KEY (warning), CFG_DEPRECATED_COLLISION (fatal)."
  - "A concern declaring another concern's key fails as duplicate authority with a remediation hint to use ref:<key> instead of repeating the value."

requirements-completed: [CONF-01, CONF-02, CONF-04]

duration: 7min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 03: Strict Independently Owned Concern Set Summary

**The root index now discovers all six Phase 1 concerns, every canonical key has exactly one owning concern (commands refer to the Go pin via `ref:toolchain.go.version` instead of repeating it), and the strict decoder fails unknown/duplicate/invalid/collided/duplicate-authority/unresolved/cyclic inputs with distinct stable codes while deprecated-only input warns with actionable migration guidance — all executable via `golc.ps1 test --quick --scope config-strict`**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-07-20T19:00:48Z
- **Completed:** 2026-07-20T19:07:59Z
- **Tasks:** 1 (TDD)
- **Files modified:** 9

## Accomplishments

- `golc.project.toml` indexes exactly the six Phase 1 concerns — toolchain, commands, generation, application_defaults, runtime, linear — and `ValidateRepository` enforces that discovery contract: a root index that hides or invents a concern fails with `GOLC_CONFIG_INDEX_MISMATCH` (D-05/D-10).
- `internal/projectconfig/model.go` defines the typed model: `Spec` (concern set + deprecation register), `ConcernSpec` (id/path/owned canonical keys), `KeySpec` (closed value sets or shape patterns), `Deprecation` (old/replacement keys, introduced/deprecated/optional-removal versions, non-empty migration message), and safe `Diagnostic` findings. `DefaultSpec()` is the production single-authority allocation covering all sixteen canonical keys.
- `internal/projectconfig/decode.go` implements the strict decoder: each concern validates alone (`ValidateConcern`), the registry itself is validated before any file read (`ValidateAuthority`), and `ValidateRepository` combines index discovery, per-concern validation, and cross-concern `ref:` resolution. Distinct stable outcomes: `GOLC_CONFIG_UNKNOWN_KEY`, `GOLC_CONFIG_DUPLICATE_KEY`, `GOLC_CONFIG_VALUE_INVALID`, `CFG_DEPRECATED_KEY` (warning with origin, replacement, versions, and migration message), `CFG_DEPRECATED_COLLISION`, `GOLC_CONFIG_DUPLICATE_AUTHORITY`, `GOLC_CONFIG_REF_UNRESOLVED`, `GOLC_CONFIG_REF_CYCLE`, `GOLC_CONFIG_DEPRECATION_INVALID`.
- `config/commands.toml` demonstrates the refer-don't-repeat rule in production: `commands.go_version = "ref:toolchain.go.version"` resolves to the pinned literal at repository level, and the strict test asserts the literal never appears in the commands file.
- `config/integrations/linear.toml` declares only taxonomy and environment variable NAMES (`LINEAR_API_KEY`, `LINEAR_TEAM_ID`, `requirement` label, mapping-file path); the strict test asserts no UUID shape and no `lin_api_` credential shape can appear (T-01-09).
- `config/application-defaults.toml` restates two locked PROJECT.md product decisions (`pool_update_review = "preview"`, `scene_apply = "immediate"`) as discoverable committed defaults with closed value sets; no machine values.
- `internal/projectconfig/strict_test.go` declares scope `config-strict` through `command.MustDeclareScope` beside the exact `TestScopeConfigStrict` marker; `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-strict` exits 0 from the repository root.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: strict concern-set validation contract** - `ef77d38` (test)
2. **GREEN - Task 1: six concern files, DefaultSpec registry, strict decoder** - `d8008ac` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/projectconfig/model.go` - Typed concern/provenance/deprecation model plus the production `DefaultSpec` allocation and shared value-shape patterns.
- `internal/projectconfig/decode.go` - `ValidateAuthority`/`ValidateConcern`/`ValidateRepository` strict decoding, deprecation handling, reference resolution, index-discovery enforcement.
- `internal/projectconfig/strict_test.go` - External test package; scope `config-strict` declaration beside `TestScopeConfigStrict`; fifteen subtests covering the production repository and every distinct failure mode via synthetic Specs/roots.
- `golc.project.toml` - Now indexes all six concerns; header documents the discovery contract.
- `config/commands.toml` - Contributor entrypoint, delegated CLI path, and the typed Go-version reference.
- `config/generation.toml` - Generated-artifact rules (schemas dir, LF normalization) per D-08.
- `config/application-defaults.toml` - Future product defaults restating locked PROJECT.md decisions.
- `config/integrations/linear.toml` - Credential-free Linear taxonomy/env-name declarations.
- `config/runtime.toml` - Header now cites the strict single-authority registry; values unchanged.

## Decisions Made

- **Diagnostic code spellings:** the plan specifies `CFG_DEPRECATED_KEY`/`CFG_DEPRECATED_COLLISION` exactly, so those two codes are used verbatim for deprecation outcomes; all other new diagnostics keep the repository's established `GOLC_CONFIG_*` prefix for consistency.
- **Concern id grammar:** `application_defaults` (underscore) keeps canonical keys inside the dotted-lowercase grammar shared with the local-layer key pattern, while the file path retains the researched `config/application-defaults.toml` spelling.
- **Deprecated input semantics:** deprecated-only input is non-fatal — the value applies to the replacement key and a `CFG_DEPRECATED_KEY` warning carries origin, replacement, all three versions, and the migration message. Nothing is silently rewritten; old-plus-replacement is the hard `CFG_DEPRECATED_COLLISION` error.
- **Empty production deprecation register:** the greenfield repository has no renamed keys, so `DefaultSpec().Deprecations` is empty; register mechanics are fully exercised by synthetic Specs in the strict scope, and `ValidateAuthority` rejects malformed entries (empty message/versions, unowned replacement, still-owned old key, duplicates).
- **Ref values defer constraints:** a `ref:` value passes single-concern validation (only its target's key shape is checked) and is resolved plus constraint-checked at repository level, so "each concern validates alone" and "no duplicated authority" hold simultaneously.

## Deviations from Plan

None - plan executed exactly as written. (The only naming adaptation: test helper constants renamed to `strictRuntimeConcern`/`strictToolchainConcern` to avoid colliding with identifiers already defined in `load_test.go` in the same external test package.)

## Issues Encountered

None. The pre-built CLI needed no rebuild: the quick dispatcher discovers `TestScopeConfigStrict` at runtime through `go test -list`, and no `internal/command` source changed.

## Known Stubs

- `generation.schemas_dir` points at `schemas/`, which does not exist yet; the generation pipeline that populates it belongs to the schema-generation plan (research Pattern 3). The concern declares the rule set only.
- `commands.cli_binary` documents the delegated executable location already produced by bootstrap; `check|generate|build|package|linear` routes still fail with stable `GOLC_ROUTE_UNKNOWN` until their owning plans self-register them (carried from 01-17).

## Threat Flags

None — no new network, process-execution, or credential surface. T-01-07 (strict configuration tampering) and T-01-09 (Linear disclosure) mitigations from the plan's threat model are implemented and test-locked; T-01-SC is untouched (`go.mod`/`go.sum` unchanged).

## User Setup Required

None - everything is repository-local; no credentials, npm, or network access involved.

## Next Phase Readiness

- Plan 01-18 (five-layer precedence) can extend `ResolveRuntime`/`Explain` over the same canonical-key registry; `DefaultSpec` is the authoritative key-to-concern map to gate env/CLI overrides against.
- Schema generation (research Pattern 3) can reflect `Spec`/`ConcernSpec`/`Deprecation` directly into committed Draft 2020-12 schemas under `generation.schemas_dir`.
- Any future key rename is now a one-line `Deprecation` register entry with mechanical warning/collision behavior already proven.

## Self-Check: PASSED

- All nine created/modified files exist on disk (`internal/projectconfig/model.go`, `decode.go`, `strict_test.go`, `golc.project.toml`, `config/commands.toml`, `config/generation.toml`, `config/application-defaults.toml`, `config/integrations/linear.toml`, `config/runtime.toml`).
- Commits `ef77d38` (test) and `d8008ac` (feat) exist in git history; no file deletions in either commit; working tree clean before summary.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-strict`, the full pinned-toolchain `go test ./...`, `go vet ./...`, and the green walking skeleton all exit 0 from the repository root.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
