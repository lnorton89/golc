---
phase: 01-offline-foundation-and-delivery-traceability
plan: 22
subsystem: contract-generation
tags: [go, jsonschema, command-routing, linear, offline, credential-free]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 04
    provides: internal/contracts's exclusive SchemaDescriptor registry with MustRegisterSchema/RegisteredSchemas/GenerateAll/CheckDrift
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 09
    provides: internal/trace/catalog's schema-1-to-2 migration (MigrateV1ToV2/CheckMigration/WriteMigration) and the strict, credential-free Map/EntitySummary/RemoteMapping model
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 19
    provides: the committed schemas/ directory and self-registered offline "generate"/"generate --check"/"check --concern project" routes
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 21
    provides: internal/command/linear.go's "linear" scope and internal/command/linear_validate.go's single "linear validate" route declaration
provides:
  - Registered "linear-map" SchemaDescriptor and committed schemas/linear-map.schema.json (strict Draft 2020-12, additionalProperties:false, nullable pending-identity fields)
  - Self-registered offline "linear map migrate --check|--write" and "linear status --offline" routes
  - Extended "linear validate --offline" handler with generated-schema/canonical-map/catalog-correspondence/authority/source-containment/credential-absence checks
affects: [01-24, linear-traceability, contract-generation]

tech-stack:
  added: []
  patterns:
    - "internal/contracts/linear.go is a fresh, purpose-built Go type projection of the schema-2 linear-map document (LinearMapSchema/LinearMapRepositoryBlock/LinearMapMilestoneBlock/LinearMapEntitySchema/LinearMapRemoteMapping) rather than a direct jsonschema reflection of catalog.Map, keeping internal/contracts a leaf package with no new internal dependency while still projecting the exact JSON shape catalog.MigrateV1ToV2 produces."
    - "Nullable pending-identity fields use the invopop/jsonschema \"nullable\" tag (LinearUUID/Identifier/URL *string with jsonschema:\"required,nullable\"), which the reflector renders as oneOf[{type:string},{type:null}] -- expressing CONTEXT D-11's 'pending/null linkage is valid, never local failure' directly in the generated contract."
    - "jsonschema struct tag description text must never contain an unescaped comma: splitOnUnescapedCommas (invopop/jsonschema) treats every comma as a directive separator, so a raw comma inside description=... silently truncates the description at that comma with no compile or generation error. Every description in linear.go uses semicolons instead of commas; this is now a standing constraint for any future internal/contracts type (alongside model.go's existing documented backslash-escaping constraint)."
    - "linear map migrate/linear status follow the established dash-word precedent (test.go/generate.go/check.go): router.go's route-word grammar rejects any word beginning with \"-\", so each is one exact multi-word MustDeclareRoute (\"linear map migrate\", \"linear status\") whose handler strictly parses the remaining --check/--write/--offline flag."
    - "The extended \"linear validate --offline\" handler composes existing already-tested entrypoints (contracts.CheckDrift scoped to schemas/linear-map.schema.json, catalog.CheckMigration, catalog.BuildCatalog) rather than reimplementing any strict-decode, migration, or validation logic -- CheckMigration alone already exercises strict JSON decoding, catalog-correspondence, authority, source-containment, and credential-absence."

key-files:
  created:
    - internal/contracts/linear.go
    - schemas/linear-map.schema.json
  modified:
    - internal/command/linear.go
    - internal/command/linear_validate.go

key-decisions:
  - "internal/contracts/linear.go defines its own LinearMapSchema/... Go types rather than reflecting internal/trace/catalog.Map directly, mirroring model.go's seven configuration schemas: this keeps internal/contracts a leaf package (no new import of internal/trace/catalog) and lets the nullable-field jsonschema tags express D-11's pending/null semantics precisely, which catalog.Map's plain *string fields don't carry on their own."
  - "local_id/parent_local_id/repo_id patterns use a general \"kind:value\" shape (^[a-z]+:[A-Za-z0-9._-]+$) rather than duplicating internal/trace/catalog/id.go's exact six-way per-kind ID grammar as a second regex authority (D-05: refer, never repeat); the full grammar remains exclusively enforced in Go by catalog.ParseID/Validate, which every route in this plan already calls."
  - "\"linear map migrate --check\"/\"--write\" and \"linear status --offline\" are declared as single multi-word MustDeclareRoute calls (\"linear map migrate\", \"linear status\") with strict in-handler flag parsing, because router.go's route-word grammar rejects any word beginning with \"-\" -- the same precedent generate.go/check.go/test.go already establish and .planning/STATE.md documents."
  - "\"linear validate --offline\"'s generated-schema check is scoped to only schemas/linear-map.schema.json (filtering contracts.CheckDrift's full changed-paths result) rather than failing on drift in an unrelated configuration schema this route has no ownership over."
  - "\"linear status --offline\" reports allowlisted per-entity fields (local_id, kind, source, and -- only once mapped -- linear_type/status/identifier/url) with Identifier/URL omitted (not rendered as null) while a mapping is pending, giving a safe at-a-glance view without ever inventing or exposing a credential (T-01-26)."

requirements-completed: [LINR-01, LINR-02]

coverage:
  - id: D1
    description: "internal/contracts/linear.go registers exactly one \"linear-map\" SchemaDescriptor; the unchanged global generator (internal/contracts/generate.go, not edited by this plan) reaches it through RegisteredSchemas and writes/checks schemas/linear-map.schema.json."
    requirement: LINR-01
    verification:
      - kind: integration
        ref: "Built cmd/golc-project from source (host toolchain) and ran GOLC_PROJECT_ROOT=<repo> golc-project generate --check (reported drift for the not-yet-committed schema), then generate (wrote schemas/linear-map.schema.json), then generate --check again (reported \"no drift\", exit 0)."
        status: pass
      - kind: unit
        ref: "go test ./... (all packages, including internal/contracts's existing registry/drift tests, which iterate contracts.RegisteredSchemas() unchanged) passes with the new descriptor present."
        status: pass
    human_judgment: false
  - id: D2
    description: "\"linear map migrate --check\"/\"--write\" and \"linear status --offline\" self-register through MustDeclareRoute under the existing \"linear\" scope, reachable offline with no router.go/.env/network/Node dependency."
    requirement: LINR-02
    verification:
      - kind: integration
        ref: "GOLC_PROJECT_ROOT=<repo> golc-project linear map migrate --check (exit 0, no drift), linear map migrate --write (exit 0, byte-idempotent -- git status showed zero change to the already-canonical committed .planning/linear-map.json), linear map migrate --bogus (exit 2, GOLC_LINEAR_USAGE), and linear status --offline (exit 0, 72 pending mappings reported, zero linear_uuid/identifier/url values present)."
        status: pass
      - kind: unit
        ref: "go test ./internal/command/... -- registry build (NewDefaultCommandRegistry) succeeds with the two new routes declared alongside the five pre-existing linear routes."
        status: pass
    human_judgment: false
  - id: D3
    description: "The existing single \"linear validate\" route declaration is preserved (no duplicate registration); its handler is extended with strict JSON, generated-schema, canonical-map, catalog-correspondence, authority, source-containment, and credential-absence checks, and passes clean against the real committed configuration."
    requirement: LINR-01
    verification:
      - kind: integration
        ref: "GOLC_PROJECT_ROOT=<repo> golc-project linear validate --offline exits 0 against the real repository (72 entities, full deterministic entity/count report)."
        status: pass
      - kind: integration
        ref: "Negative-path proof: corrupting schemas/linear-map.schema.json produced GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT (exit 1); injecting a non-null linear_uuid into a \"pending\" .planning/linear-map.json mapping produced GOLC_MIGRATE_TARGET_INVALID: pending mapping ... must not carry a remote identity (exit 1) from both linear map migrate --check and linear validate --offline. Both fixtures were restored from backup immediately after."
        status: pass
      - kind: unit
        ref: "go build ./..., go vet ./..., go test -count=1 ./..., and go test -race -count=1 ./... all pass; internal/command/router.go has zero diff in this plan."
        status: pass
    human_judgment: false

duration: ~50min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 22: Linear Map Contract and Offline Migrate/Validate/Status Routes Summary

**Registered the "linear-map" JSON Schema contract (nullable pending-identity fields) through the existing Plan 04 registry, self-registered offline "linear map migrate --check\|--write" and "linear status --offline" routes, and extended the existing "linear validate --offline" handler with schema/canonical-map/catalog/authority/credential checks -- all without touching internal/contracts/generate.go or internal/command/router.go**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-07-21T02:55:00Z (approx.)
- **Completed:** 2026-07-21T03:45:00Z (approx.)
- **Tasks:** 1
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- `internal/contracts/linear.go` declares `LinearMapSchema` (plus `LinearMapRepositoryBlock`/`LinearMapMilestoneBlock`/`LinearMapEntitySchema`/`LinearMapRemoteMapping`) as a fresh, purpose-built Go type projection of the schema-2 `.planning/linear-map.json` shape, and registers it exactly once through `MustRegisterSchema(SchemaDescriptor{Name: "linear-map", OutputPath: "schemas/linear-map.schema.json", ...})`. The unchanged `internal/contracts/generate.go`/`RegisteredSchemas`/`GenerateAll`/`CheckDrift` reach it automatically, proven by `generate --check` reporting drift before commit and "no drift" after.
- `LinearMapRemoteMapping`'s `LinearUUID`/`Identifier`/`URL` fields are `*string` with `jsonschema:"required,nullable"`, which the reflector renders as `oneOf[{type:string},{type:null}]` in the committed schema -- encoding CONTEXT D-11's "pending/null linkage is valid, never local failure" directly into the generated contract rather than only in Go-level validation.
- `internal/command/linear.go` self-registers `linear map migrate --check` (read-only, delegates to `catalog.CheckMigration`), `linear map migrate --write` (atomic replace via `catalog.WriteMigration`), and `linear status --offline` (delegates to `catalog.MigrateV1ToV2`, reports allowlisted per-entity `local_id`/`kind`/`source` plus mapped `linear_type`/`status`/`identifier`/`url`, omitting the latter three while pending). All three follow the established dash-word precedent (`test.go`/`generate.go`/`check.go`): one exact multi-word `MustDeclareRoute`, strict in-handler flag parsing.
- `internal/command/linear_validate.go` preserves Plan 21's single `linear validate` route declaration untouched and extends only `runLinearValidate`: a generated-schema drift check scoped to `schemas/linear-map.schema.json` (via `contracts.CheckDrift`), then `catalog.CheckMigration` (which itself composes strict JSON decoding, canonical-map comparison, catalog-correspondence, authority, source-containment, and credential-absence via `catalog.Validate`/`validateMap`), then the existing `catalog.BuildCatalog`-based entity report.
- Verified end to end against the real repository: `generate`/`generate --check`, `check --concern project`, `linear map migrate --check`/`--write` (byte-idempotent -- writing produced zero diff against the already-canonical committed map), `linear status --offline` (72 pending mappings, zero credential fields present), and `linear validate --offline` (72-entity clean report) all passed. Two negative-path probes confirmed the new checks actually fire: corrupting the committed schema produced `GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT`, and injecting a non-null `linear_uuid` into a `"pending"` mapping produced `GOLC_MIGRATE_TARGET_INVALID: pending mapping ... must not carry a remote identity` from both `linear map migrate --check` and `linear validate --offline`.

## Task Commits

1. **Task 1: Generate the map contract and report pending linkage offline** - `88d8d18` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/contracts/linear.go` - Fresh `LinearMapSchema` Go type projection; registers the `linear-map` SchemaDescriptor exactly once.
- `schemas/linear-map.schema.json` - Committed Draft 2020-12 projection of the schema-2 linear-map document, generated via `generate`.
- `internal/command/linear.go` - Added self-registered `linear map migrate --check|--write` and `linear status --offline` routes plus their handlers/view types.
- `internal/command/linear_validate.go` - Extended `runLinearValidate` with generated-schema, canonical-map, catalog-correspondence, authority, source-containment, and credential-absence checks; the route declaration itself is unchanged.

## Decisions Made

- **`internal/contracts/linear.go` defines fresh Go types instead of reflecting `catalog.Map` directly** -- keeps `internal/contracts` a leaf package with no new dependency on `internal/trace/catalog`, and lets `jsonschema:"nullable"` express D-11's pending/null semantics precisely on the three identity fields.
- **ID patterns stay a general `kind:value` shape** rather than duplicating `internal/trace/catalog/id.go`'s six-way per-kind grammar as a second regex authority (D-05: refer, never repeat) -- the exact grammar remains exclusively enforced in Go by `catalog.ParseID`/`Validate`, which every route in this plan already calls.
- **`linear map migrate`/`linear status` are single multi-word routes with strict in-handler flag parsing**, following the exact precedent `test.go`/`generate.go`/`check.go` already establish, because `router.go`'s route-word grammar rejects any word beginning with `-`.
- **The generated-schema drift check in `linear validate --offline` is scoped to only `schemas/linear-map.schema.json`**, filtering `contracts.CheckDrift`'s full result, so this route never fails on drift in an unrelated configuration schema it does not own.
- **`linear status --offline` omits (never nulls) `identifier`/`url` while a mapping is pending** -- an allowlisted, credential-free at-a-glance view (T-01-26) that treats pending/null linkage as valid status output, not an error.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unescaped commas from jsonschema struct-tag `description=` text**
- **Found during:** Task 1, first `generate` run against the real repository
- **Issue:** `invopop/jsonschema`'s `splitOnUnescapedCommas` treats every comma inside a struct tag as a directive separator with no comma-escaping in this codebase's established tag style (unlike backslashes, which model.go's own top comment already documents as unsafe). Five `description=` values in the initial draft of `internal/contracts/linear.go` contained a literal comma (e.g. "Supported linear map schema version; only schema 2 (the migrated, complete catalog map) is projected."), which silently truncated the rendered `$comment`/`description` at that comma with no compile error and no generation failure -- only visible by inspecting the actual generated `schemas/linear-map.schema.json` output.
- **Fix:** Reworded every affected description to use a semicolon instead of a comma (matching the existing codebase convention already used for every other description in `model.go`), then regenerated and confirmed every description renders in full.
- **Files modified:** `internal/contracts/linear.go`
- **Verification:** `generate` re-run; `schemas/linear-map.schema.json`'s five previously-truncated descriptions now render completely (verified via direct grep of the generated file).
- **Committed in:** `88d8d18` (Task 1 commit; the fix was applied before the file was ever staged, so there is no separate commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Pre-commit fix to the plan's own new file; no impact on any other plan's owned files or on the acceptance criteria, which this plan still meets exactly.

## Issues Encountered

- **No bootstrapped pinned toolchain in this worktree** (`.tools/` does not exist, matching every prior plan's documented condition in this phase). `powershell -NoProfile -File .\golc.ps1 generate --check` and `linear validate --offline` therefore cannot run through the shim without a multi-minute bootstrap first. Verification instead built `cmd/golc-project` directly from source with the host Go toolchain (`GOPROXY=off GOFLAGS=-mod=readonly go build -trimpath -o <tmp>/golc-project-test.exe ./cmd/golc-project`), which is exactly `go1.26.5 windows/amd64` -- a byte-for-byte match for `config/toolchain.toml`'s pinned version -- and ran the exact equivalent commands the shim would delegate to. This mirrors the identical substitution documented in Plans 04, 09, 18, 19, and 23's summaries. `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `go test -race -count=1 ./...` all pass offline; `go.mod`/`go.sum` are untouched (confirmed via `git status --short go.mod go.sum` showing no output before and after).
- **No test files added.** This plan's frontmatter file inventory (`internal/contracts/linear.go`, `internal/command/linear.go`, `internal/command/linear_validate.go`, `schemas/linear-map.schema.json`) is described as the plan's "complete owned file inventory" and does not include any `_test.go` file, matching Plan 01-19's identical precedent for a `tdd="true"`-flagged plan whose file inventory also excluded tests (treated as integration-first, verified via the shell `<verify>` block rather than a literal RED/GREEN unit-test cycle). Coverage instead comes from the extensive positive- and negative-path integration verification documented above (including two deliberate tamper probes proving the new checks actually fire, with fixtures restored from backup immediately after each probe) plus the full existing `go test ./...`/`go test -race ./...` suite, which already exercises `contracts.RegisteredSchemas()`/`CheckDrift`/`GenerateAll` generically and `catalog.CheckMigration`/`BuildCatalog`/`MigrateV1ToV2` directly against both fixture and real-repository data.

## Known Stubs

None. `linear map migrate --check`/`--write`, `linear status --offline`, and the extended `linear validate --offline` are fully wired to real logic (`catalog.CheckMigration`/`WriteMigration`/`MigrateV1ToV2`/`BuildCatalog`, `contracts.CheckDrift`) -- there is no mocked or placeholder data path.

## User Setup Required

None -- schema generation, map migration, status reporting, and validation are pure Go over already-committed repository files; none of the four new/extended routes open a network connection, read `.env`, or require a credential.

## Next Phase Readiness

- `internal/contracts/generate.go`/`CheckDrift` now cover `linear-map` alongside the seven Plan 19 configuration schemas with zero further edits to that file, exactly as its own comments anticipated ("a later plan adding a new `contracts.SchemaDescriptor` (for example a Linear mapping or plan schema)").
- `internal/command/linear.go` now exposes the complete offline map lifecycle contributors need before any live Linear transport exists: inspect (`linear catalog`), migrate (`linear map migrate`), validate (`linear validate`), and status (`linear status`), alongside Plan 23's `linear preview`/`linear archive`/`linear unlink`.
- Plan 01-24 ("Generate strict plan/report contracts and expose apply") can follow this exact same registration/route-extension pattern for its own `linear-plan`/`linear-report` schemas without needing to consult anything beyond `internal/contracts/generate.go`'s and `internal/command/router.go`'s already-stable public entrypoints.

## Self-Check: PASSED

- All four owned files verified present on disk: `internal/contracts/linear.go`, `internal/command/linear.go`, `internal/command/linear_validate.go`, `schemas/linear-map.schema.json`.
- Commit `88d8d18` verified present in `git log --oneline --all` on branch `worktree-agent-aeeb75e54f307a552`; `git diff --diff-filter=D --name-only HEAD~1 HEAD` reports zero deleted files; working tree clean (only this plan's four files staged/committed) before this summary.
- `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `go test -race -count=1 ./...` all exit 0 from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly); the built `golc-project` binary's `generate`, `generate --check`, `check --concern project`, `linear map migrate --check`/`--write`, `linear status --offline`, and `linear validate --offline` all exited 0 against the real repository root with the expected stable diagnostics, and both deliberate tamper probes produced the expected `GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT`/`GOLC_MIGRATE_TARGET_INVALID` failures before being restored.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
