---
phase: 01-offline-foundation-and-delivery-traceability
plan: 04
subsystem: contract-generation
tags: [go, jsonschema, draft-2020-12, deterministic-generation, offline]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 16
    provides: Checksum-controlled bootstrap with the warmed, GOPROXY=off-resolvable github.com/invopop/jsonschema v0.14.0 module cache
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 18
    provides: internal/projectconfig's strict Spec/DefaultSpec single-authority concern model these contract projections mirror
provides:
  - Compile-safe SchemaDescriptor/RegisterSchema/MustRegisterSchema/RegisteredSchemas registry with blank/duplicate-name and blank/duplicate-output-path rejection and a defensive stable name-sorted snapshot
  - GenerateAll/GenerateInto/CheckDrift generation and read-only drift-checking, each traversing every registered descriptor exactly once without a closed switch/list
  - NormalizeSchema/SortJSON/NormalizeLF canonical JSON formatting: recursively key-sorted, two-space indented, LF-only, byte-identical on Windows and CI
  - Seven self-registered Phase 1 configuration schema projections (golc-project, config-toolchain, config-commands, config-generation, config-application-defaults, config-runtime, config-linear)
  - Registered contracts quick-test scope (TestScopeContracts)
affects: [01-05, contract-generation]

tech-stack:
  added: []
  patterns:
    - "Contract self-registration mirrors internal/command/router.go's MustDeclareRoute/MustDeclareScope idiom (D-03): MustRegisterSchema panics at program startup on a rejected registration, so a later contract file extends GenerateAll/GenerateInto/CheckDrift without editing generate.go"
    - "Struct-tag regex patterns use bracket character classes (e.g. \"[.]\") instead of backslash escapes, because reflect.StructTag.Lookup applies strconv.Unquote to the tag text and a bare backslash is an invalid Go string-literal escape there (verified empirically; see model.go package doc)"
    - "Normalization is round-tripped through a custom sortedObject JSON encoder rather than relying on encoding/json's incidental map-key sort order, so canonical key ordering is guaranteed by this package's own code, not an unspecified stdlib behavior"

key-files:
  created:
    - internal/contracts/model.go
    - internal/contracts/generate.go
    - internal/contracts/normalize.go
    - internal/contracts/generate_test.go

key-decisions:
  - "commands.go_version's committed value is the typed cross-concern reference \"ref:toolchain.go.version\" (D-05: refer, never repeat), not a literal dotted version, so its schema pattern accepts either a dotted version or the canonical ref:<dotted.key> grammar rather than only the literal shape."
  - "GenerateAll(root) and GenerateInto(targetRoot) share one underlying write path; both names exist in the exported API per the acceptance criteria — GenerateAll documents the committed-location intent for a future `generate` CLI command, GenerateInto is the general primitive CheckDrift and tests use against a disposable directory."
  - "renderSchema centrally sets a generated-marker $comment and a defensive AdditionalProperties:false on every descriptor's schema (in generate.go, not per-type in model.go), so a later contract file gets both guarantees automatically without repeating boilerplate."
  - "MustRegisterSchema is an additional exported entrypoint beyond the six the acceptance criteria names (SchemaDescriptor/RegisterSchema/RegisteredSchemas/GenerateAll/GenerateInto/CheckDrift); it is the panic-on-error self-registration wrapper model.go's package-level var block calls, matching this repo's established D-03 self-registration convention."

patterns-established:
  - "GOLC_CONTRACTS_NAME_EMPTY / GOLC_CONTRACTS_OUTPUT_EMPTY / GOLC_CONTRACTS_FACTORY_NIL / GOLC_CONTRACTS_NAME_DUPLICATE / GOLC_CONTRACTS_OUTPUT_DUPLICATE: stable RegisterSchema diagnostics."
  - "GOLC_CONTRACTS_SCHEMA_NIL / GOLC_CONTRACTS_WRITE / GOLC_CONTRACTS_READ / GOLC_CONTRACTS_TEMP / GOLC_CONTRACTS_ENCODE / GOLC_CONTRACTS_DECODE / GOLC_CONTRACTS_INDENT: stable generation/normalization diagnostics."

requirements-completed: [CONF-01, CONF-02, CONF-03]

coverage:
  - id: D1
    description: "Unchanged Go types generate byte-identical Draft 2020-12 schemas across repeated runs."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/contracts/generate_test.go TestScopeContracts/generation_is_deterministic_and_byte-identical_across_repeated_runs"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every generated object denies additional properties and the root names its source package via a generated-marker $comment."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "internal/contracts/generate_test.go TestScopeContracts/every_generated_object_denies_additional_properties"
        status: pass
    human_judgment: false
  - id: D3
    description: "Check mode (CheckDrift) reports changed paths and never writes to the committed target."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "internal/contracts/generate_test.go TestScopeContracts/CheckDrift_reports_changed_paths_without_touching_a_committed_target"
        status: pass
    human_judgment: false
  - id: D4
    description: "RegisterSchema rejects blank/duplicate names and output paths; RegisteredSchemas returns a defensive stable name-sorted snapshot; GenerateInto/CheckDrift call each descriptor's factory exactly once."
    requirement: CONF-02
    verification:
      - kind: unit
        ref: "internal/contracts/generate_test.go TestScopeContracts/RegisterSchema_rejects_blank_and_nil-factory_descriptors, .../RegisterSchema_rejects_duplicate_names_and_output_paths, .../RegisteredSchemas_returns_a_defensive_stable_name-sorted_snapshot, .../GenerateInto_and_CheckDrift_traverse_the_registry_exactly_once"
        status: pass
    human_judgment: false
  - id: D5
    description: "All seven Phase 1 configuration descriptors are registered under their exact stable names; generated bytes carry no timestamp, machine path, or credential."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/contracts/generate_test.go TestScopeContracts/known_configuration_descriptors_are_registered, .../generated_schemas_carry_no_timestamp_machine_path_or_credential"
        status: pass
    human_judgment: false
  - id: D6
    description: "Contracts verification builds/tests/vets offline (GOPROXY=off, -mod=readonly) against the Plan 16-warmed module cache and leaves go.mod/go.sum byte-unchanged."
    requirement: CONF-03
    verification:
      - kind: integration
        ref: "GOPROXY=off GOFLAGS=-mod=readonly go build ./... && go vet ./... && go test ./... (host Go 1.26.5, exact pin match); go.mod/go.sum SHA-256 unchanged before/after"
        status: pass
    human_judgment: false

duration: 42min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 04: Deterministic Strict Contract Generation Summary

**Compile-safe SchemaDescriptor registry plus deterministic Draft 2020-12 JSON Schema generation, normalization, and read-only drift checking for the seven Phase 1 configuration schema projections, extensible for later contract files without editing the central generator**

## Performance

- **Duration:** ~42 min
- **Started:** 2026-07-21T01:02:00Z
- **Completed:** 2026-07-21T01:44:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 4

## Accomplishments

- `internal/contracts/generate.go` owns the compile-safe registry: `SchemaDescriptor`, `RegisterSchema` (rejects blank/whitespace names, blank output paths, nil factories, duplicate names, and duplicate output paths), `MustRegisterSchema` (the panic-on-error self-registration entrypoint mirroring `internal/command`'s `MustDeclareRoute`/`MustDeclareScope` idiom), `RegisteredSchemas` (defensive, stable name-sorted snapshot), and `GenerateAll`/`GenerateInto`/`CheckDrift`, all of which traverse the registered-descriptor snapshot exactly once rather than a closed switch or list.
- `internal/contracts/normalize.go` provides `NormalizeSchema`, `SortJSON`, and `NormalizeLF`: JSON is decoded, every object's keys are recursively sorted through a custom explicit `sortedObject` encoder (not relying on `encoding/json`'s incidental map-key ordering), re-indented with two spaces, and CRLF/trailing-newline noise is collapsed to exactly one trailing LF — the single ordering/formatting authority every generated contract passes through.
- `internal/contracts/model.go` defines the seven Phase 1 configuration schema projections (`RootIndexSchema`, `ToolchainSchema`, `CommandsSchema`, `GenerationSchema`, `ApplicationDefaultsSchema`, `RuntimeSchema`, `LinearSchema`, plus their nested block types) as new, purpose-built authoritative Go types mirroring each concern file's real committed TOML structure, and self-registers all seven through `MustRegisterSchema` under the exact stable names `golc-project`, `config-toolchain`, `config-commands`, `config-generation`, `config-application-defaults`, `config-runtime`, and `config-linear`.
- `invopop/jsonschema` reflection defaults to `additionalProperties: false` on every object (verified empirically before writing model.go), and `renderSchema` in generate.go adds a defensive `additionalProperties: false` plus a fixed generated-marker `$comment` naming the source package centrally, so every current and future descriptor gets both guarantees without per-type boilerplate.
- `internal/contracts/generate_test.go` self-registers quick-test scope `contracts` beside `TestScopeContracts` (Plan 17 contract) and proves: blank-name/blank-output-path/nil-factory rejection; duplicate-name and duplicate-output-path rejection; `RegisteredSchemas`' defensive-copy and stable-sorted behavior; exactly-once `GenerateInto`/`CheckDrift` traversal (via a call-counting descriptor); `GenerateAll` writing to a committed path; byte-identical deterministic generation across repeated runs; universal `additionalProperties: false` plus the Draft 2020-12 `$schema` URL on every one of the seven known descriptors; `CheckDrift` reporting drift and leaving a seeded "committed" target byte-for-byte untouched; the absence of leaked cwd paths, `C:\Users`, `linear.app`, credential-shaped tokens, or `Bearer `/`sk-` prefixes in generated output; and `NormalizeSchema`/`SortJSON`/`NormalizeLF` LF-stability and deterministic sorted-key output.
- `go build ./...`, `go vet ./...`, and `go test ./...` (including `-race` on `internal/contracts`) all pass offline with `GOPROXY=off GOFLAGS=-mod=readonly`, and `go.mod`/`go.sum` SHA-256 hashes are byte-identical before and after the full verification run.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: deterministic contract generator test** - `c6d8116` (test)
2. **GREEN - Task 1: registry, generator, normalizer, and seven projections** - `0c60552` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/contracts/generate.go` - `SchemaDescriptor`/`RegisterSchema`/`MustRegisterSchema`/`RegisteredSchemas` compile-safe registry plus `GenerateAll`/`GenerateInto`/`CheckDrift`.
- `internal/contracts/normalize.go` - `NormalizeSchema`/`SortJSON`/`NormalizeLF` canonical formatting.
- `internal/contracts/model.go` - Seven Phase 1 configuration schema projections plus their `MustRegisterSchema` self-registrations.
- `internal/contracts/generate_test.go` - `TestScopeContracts` and the `contracts` quick-test scope declaration; full registry/generation/normalization/drift coverage.

## Decisions Made

- **`go_version` reference-shaped pattern:** `config/commands.toml`'s real committed value for `go_version` is `"ref:toolchain.go.version"` (a typed D-05 cross-concern reference), not a literal dotted version, so `CommandsBlock.GoVersion`'s pattern accepts either a literal dotted version or the canonical `ref:<dotted.key>` grammar — faithful to what decode.go's `validateDeclaredValue` actually accepts for any key.
- **`GenerateAll`/`GenerateInto` share one write path:** both names are required by the acceptance criteria and both are exported; `GenerateAll(root)` documents the "write the real committed location" intent a future `generate` CLI command will use, while `GenerateInto(targetRoot)` is the general primitive `CheckDrift` and every test call against a disposable directory — they are intentionally not collapsed into a single name.
- **Centralized generated-marker and additionalProperties defense:** `renderSchema` in generate.go (not per-type code in model.go) sets the `$comment` generated marker and a defensive `additionalProperties: false`, so a later contract file automatically inherits both without repeating logic — directly serving the "extensible without editing generate.go" requirement.
- **`MustRegisterSchema` as an additional exported entrypoint:** beyond the six names the acceptance criteria lists explicitly, `MustRegisterSchema` is the panic-on-error self-registration wrapper `model.go`'s package-level `var _ = ...` block calls, mirroring `internal/command/router.go`'s `MustDeclareRoute`/`MustDeclareScope` idiom already established elsewhere in this repository (CONTEXT D-03).
- **Struct-tag pattern escaping:** verified empirically (scratch `go run`) that a literal backslash inside a `jsonschema:"pattern=..."` struct tag value causes `reflect.StructTag.Lookup` to silently fail (`strconv.Unquote` rejects the invalid Go string-literal escape). Every pattern in model.go uses a bracket character class (e.g. `[.]` instead of `\.`) to express the identical regex without a backslash.

## Deviations from Plan

None — plan executed exactly as written. The seven projected Go types and their exact field/pattern shapes were not literally specified by the plan (only "seven Phase 1 configuration schema projections" and the `RootIndexSchema|ToolchainSchema` naming pattern were called out in `key_links`); their concrete field layout was derived directly from the real committed `golc.project.toml` / `config/*.toml` structure and `internal/projectconfig/model.go`'s `KeySpec` patterns, which is squarely within the plan's stated action, not a deviation.

## Issues Encountered

- This worktree had no bootstrapped pinned toolchain (`.tools/` did not exist, consistent with a fresh worktree checkout), so `golc.ps1 test --quick --scope contracts` could not be invoked directly. The host Go toolchain is exactly `go1.26.5 windows/amd64`, matching `config/toolchain.toml`'s pinned `toolchain.go.version` byte-for-byte, and the host module cache (`C:\Users\Lawrence\go\pkg\mod`) already resolves `github.com/BurntSushi/toml@v1.6.0` and `github.com/invopop/jsonschema@v0.14.0` (plus their full transitive graph) under `GOPROXY=off`. Per this plan's parallel-execution guidance, verification instead ran the equivalent host-toolchain commands directly: `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, `go test ./...` (including `-race ./internal/contracts/...`), and `go test -list '^TestScopeContracts$' ./...` (confirming the marker is discoverable in exactly the package/output shape `internal/command/test.go`'s `listScopeMarkers` parses), plus a `go.mod`/`go.sum` SHA-256 hash comparison before and after the full run (unchanged). This is one-time worktree-environment substitution, not a plan deviation, and mirrors the same accepted pattern documented in Plan 18's summary.
- Empirically confirmed (via a throwaway scratch Go program, removed before committing) that `github.com/invopop/jsonschema@v0.14.0`'s `Reflector` defaults `AllowAdditionalProperties` to `false`, so every reflected object already denies additional properties without any extra configuration; `renderSchema`'s explicit `additionalProperties: false` fallback is intentional defense-in-depth, not compensation for a missing default.

## Known Stubs

None. `schemas/*.schema.json` committed output does not exist yet — this plan's `files_modified` scope is exactly the four `internal/contracts/*.go` files (the generator/registry/normalizer/tests), not the generated artifacts themselves. `GenerateAll`/`GenerateInto`/`CheckDrift` are fully implemented and tested against temporary directories; wiring a `generate`/`generate --check` CLI route and committing the real `schemas/*.schema.json` files is out of this plan's scope and belongs to a later plan.

## User Setup Required

None — generation and drift-checking are pure Go, use only the already-pinned/warmed `invopop/jsonschema` module, and never touch the network or a credential.

## Next Phase Readiness

- `contracts.RegisteredSchemas()`/`GenerateAll`/`GenerateInto`/`CheckDrift` are ready for a future `internal/command` route (for example `generate` / `generate --check`) to wire directly without any change to this plan's files.
- A later plan adding a Linear mapping or plan JSON Schema contract can self-register through `MustRegisterSchema` from its own file exactly as `model.go` does here, with zero edits to `generate.go`.
- `NormalizeSchema`/`SortJSON`/`NormalizeLF` are general-purpose and available to any future package that needs deterministic, LF-stable JSON output beyond schemas.

## Self-Check: PASSED

- All four owned files verified present on disk: `internal/contracts/model.go`, `internal/contracts/generate.go`, `internal/contracts/normalize.go`, `internal/contracts/generate_test.go`.
- Commits `c6d8116` (test) and `0c60552` (feat) verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only c6d8116~1 0c60552` reports zero deleted files across both commits; working tree clean before this summary.
- `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./internal/contracts/...` all exit 0 from the repository root; `go.mod`/`go.sum` SHA-256 hashes are identical before and after the full run.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
