---
phase: 01-offline-foundation-and-delivery-traceability
plan: 21
subsystem: linear-traceability
tags: [go, catalog, durable-ids, offline, linear, command-routing]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 08
    provides: BuildCatalog/Validate and the stable GOLC_CATALOG_* diagnostic family over the repository-owned planning identity graph
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: Self-registering command registry (MustDeclareRoute/MustDeclareScope) and NewDefaultCommandRegistry contract
provides:
  - Self-registered offline route `linear catalog --offline --format json` that projects the complete BuildCatalog graph as deterministic, allowlisted JSON
  - Self-registered offline route `linear validate --offline`, owned solely by internal/command/linear_validate.go, that runs BuildCatalog's full identity/hierarchy/source/authority validation and reports per-kind counts plus the same allowlisted entity list
  - Six new tests/fixtures/linear/*.json fixtures (four adversarial, two rename-stability) plus table-driven catalog_test.go coverage that loads every one of them
affects: [01-22, linear-traceability]

tech-stack:
  added: []
  patterns:
    - Command routes stay compile-safe against only their declared Plan 08 catalog interfaces; a later plan (01-22) extends the same handler instead of redeclaring the route
    - Fixture-driven validator coverage: JSON files under tests/fixtures/linear/ decode into catalog.Entity slices and exercise catalog.Validate directly, independent of the on-disk BuildCatalog directory-scan path
    - Allowlisted JSON projection (id/kind/parent/display/source only) for all "linear ..." command output; internal filesystem paths never leave the handler

key-files:
  created:
    - internal/command/linear.go
    - internal/command/linear_validate.go
    - tests/fixtures/linear/map-duplicate-id.json
    - tests/fixtures/linear/map-invalid-parent.json
    - tests/fixtures/linear/map-cycle.json
    - tests/fixtures/linear/map-unsafe-source.json
    - tests/fixtures/linear/catalog-rename-before.json
    - tests/fixtures/linear/catalog-rename-after.json
  modified:
    - internal/trace/catalog/catalog_test.go

key-decisions:
  - "The four adversarial fixtures are synthetic catalog.Entity graphs (JSON-decoded, not on-disk BuildCatalog trees) so each one isolates exactly one GOLC_CATALOG_* diagnostic: map-duplicate-id.json -> GOLC_CATALOG_ID_DUPLICATE, map-invalid-parent.json -> GOLC_CATALOG_PARENT_UNKNOWN, map-cycle.json -> GOLC_CATALOG_CYCLE, map-unsafe-source.json -> GOLC_CATALOG_SOURCE_EXTERNAL."
  - "Rename fixtures add a fixture-only `mapped_uuid` field (nullable) alongside id/kind/parent/display/source. It is not part of catalog.Entity; it is a JSON-fixture stand-in for a Linear remote mapping's immutable UUID, and the test asserts it is byte-identical between catalog-rename-before.json and catalog-rename-after.json for every ID while at least one display string changes."
  - "internal/command/linear_validate.go declares no new routing scope of its own; it registers only the exact route \"linear validate\" under the \"linear\" scope that internal/command/linear.go declares, matching the plan's requirement that it be the sole owner of that one route."
  - "runLinearValidate does not re-implement identity/hierarchy/source/authority checks: catalog.BuildCatalog already calls catalog.Validate before returning, so a validation failure surfaces its exact GOLC_CATALOG_* diagnostic directly from BuildCatalog's error. This keeps the handler compile-safe against only the Plan 08 catalog package, as required for Plan 22 to extend it later with map/schema checks."
  - "LINR-01/LINR-02 are NOT marked complete in this plan (same reasoning as 01-08's summary): plan 01-22 still declares them and owns the actual map/schema reconciliation; marking them complete here would misstate delivery status."

patterns-established:
  - "GOLC_LINEAR_USAGE / GOLC_LINEAR_FORMAT_UNSUPPORTED / GOLC_LINEAR_CATALOG_ENCODE_FAILED / GOLC_LINEAR_VALIDATE_ENCODE_FAILED diagnostics for the linear command family, following the same exit-code contract as config.go/test.go (0 success, 1 command failure, 2 usage/routing failure)."

requirements-completed: []

duration: 12min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 21: Offline Linear Catalog Inspection and Validation Routes Summary

**Self-registered `linear catalog --offline --format json` and `linear validate --offline` routes project/validate the Plan 08 repository-owned catalog as deterministic allowlisted JSON, backed by four new adversarial fixtures and a rename-stability fixture pair**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-07-20T17:20:00Z (approx.)
- **Completed:** 2026-07-20T17:33:12Z
- **Tasks:** 1 (TDD)
- **Files modified:** 8 (2 command files created, 6 fixture files created, 1 test file extended)

## Accomplishments

- `internal/command/linear.go` declares the `linear` routing scope and self-registers the exact route `linear catalog --offline --format json`. The handler runs `catalog.BuildCatalog(root)` and emits a deterministic JSON envelope containing only allowlisted entity fields (`id`, `kind`, `parent`, `display`, `source`) in the catalog's build order — no filesystem-absolute paths, network, Node, SDK, or Linear credential access is reachable from this route.
- `internal/command/linear_validate.go` is the sole owner of the exact route `linear validate --offline`, registered under the same `linear` scope. Its handler is compile-safe against only the Plan 08 catalog package: `BuildCatalog` already runs the complete identity/hierarchy/source/authority `Validate` pass before returning, so a graph violation surfaces its exact `GOLC_CATALOG_*` diagnostic directly. On success the handler reports `status: "ok"`, per-kind entity counts, and the same allowlisted entity projection used by `linear catalog`.
- Four new adversarial fixtures under `tests/fixtures/linear/` — `map-duplicate-id.json`, `map-invalid-parent.json`, `map-cycle.json`, `map-unsafe-source.json` — each isolate exactly one catalog validator failure mode (`GOLC_CATALOG_ID_DUPLICATE`, `GOLC_CATALOG_PARENT_UNKNOWN`, `GOLC_CATALOG_CYCLE`, `GOLC_CATALOG_SOURCE_EXTERNAL`).
- Two new rename-stability fixtures, `catalog-rename-before.json`/`catalog-rename-after.json`, carry identical durable local IDs, parents, sources, and a fixture-only nullable `mapped_uuid` field, with only display text differing. Both individually pass `catalog.Validate`.
- `internal/trace/catalog/catalog_test.go` gained a fixture-loading/decoding helper set plus two new `t.Run` subtests inside the existing `TestScopeLinearCatalog` marker: one table-driven case that loads each adversarial fixture and asserts its specific diagnostic, and one that loads the rename pair and asserts every ID/kind/parent/source/`mapped_uuid` is identical while display text changes for at least one entity. The pre-existing `linear-catalog` quick-test scope declaration and marker are untouched.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: adversarial/rename catalog fixtures and offline route RED evidence** - `f493ce4` (test) — fixture-based catalog tests pass immediately (existing Plan 08 `Validate` already implements the checked invariants), but `go run ./cmd/golc-project linear validate --offline` and `linear catalog --offline --format json` both failed with `GOLC_ROUTE_UNKNOWN` before this plan's command files existed, proving the routes were unreachable.
2. **GREEN - Task 1: self-register offline linear catalog/validate routes** - `650ff19` (feat) — `internal/command/linear.go` and `internal/command/linear_validate.go` added; both routes now resolve and return exit 0, `go build ./...`, `go vet ./...`, and `go test ./...` all pass.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/command/linear.go` - Declares the `linear` scope and the `linear catalog --offline --format json` route; allowlisted `catalogEntityView`/`catalogView` JSON types and `catalogEntityViews` helper shared with `linear_validate.go`.
- `internal/command/linear_validate.go` - Sole owner of the `linear validate --offline` route; reports validation status, per-kind counts, and the allowlisted entity list from a single `BuildCatalog` call.
- `internal/trace/catalog/catalog_test.go` - Added `fixtureEntity`/`fixtureCatalogFile` JSON-decoding types, `loadLinearFixture`/`fixtureCatalogEntities`/`uuidPointersEqual` helpers, and two new adversarial/rename `t.Run` subtests under `TestScopeLinearCatalog`.
- `tests/fixtures/linear/map-duplicate-id.json` - Valid chain plus one duplicated task ID; expects `GOLC_CATALOG_ID_DUPLICATE`.
- `tests/fixtures/linear/map-invalid-parent.json` - Task parented to a nonexistent plan ID; expects `GOLC_CATALOG_PARENT_UNKNOWN`.
- `tests/fixtures/linear/map-cycle.json` - Two phases parenting each other; expects `GOLC_CATALOG_CYCLE`.
- `tests/fixtures/linear/map-unsafe-source.json` - Requirement entity with a `.planning/../.env` escaping source; expects `GOLC_CATALOG_SOURCE_EXTERNAL`.
- `tests/fixtures/linear/catalog-rename-before.json` / `catalog-rename-after.json` - Identical IDs/parents/sources/`mapped_uuid` values with renamed display text, proving rename stability (D-14).

## Decisions Made

- Kept the adversarial/rename fixtures at the `catalog.Entity`/`catalog.Validate` level (via a small fixture-only JSON schema) rather than building full synthetic `.planning/` directory trees, so each fixture isolates exactly one validator failure mode or the rename-stability invariant without depending on the dynamic `BuildCatalog` filesystem-discovery path already covered by Plan 08's own fixture tests.
- Added a fixture-only nullable `mapped_uuid` field to satisfy "rename fixtures preserve local IDs and mapped UUID fields" without introducing a new field on the production `catalog.Entity` type — that stays scoped to Plan 22's schema/map work, keeping this plan's file inventory exactly the nine paths declared in its frontmatter.
- `linear_validate.go` declares no scope of its own (the `linear` scope is declared once, in `linear.go`); it registers only its one exact route, matching the plan's "sole owner of the route" requirement without duplicating scope declarations.
- Did not mark `LINR-01`/`LINR-02` complete in `REQUIREMENTS.md`, mirroring 01-08's precedent: plan 01-22 still declares both requirements and owns the actual credential-free UUID mapping/reconciliation work this plan's routes will be extended to validate.

## Deviations from Plan

None - plan executed exactly as written. Both owned files (`linear.go`, `linear_validate.go`) and the six fixture files match the frontmatter's `files_modified` list exactly, and `internal/trace/catalog/catalog_test.go` was extended in place rather than replaced.

## Issues Encountered

- This worktree had no `.tools/` bootstrap output (bootstrap had not been run here), so the plan's literal `<verify>` command (`powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-catalog` then `... linear validate --offline`) could not execute through the shim without first running a multi-minute bootstrap. Verification was instead performed with the equivalent pinned-toolchain calls directly — `go test -count=1 -run '^TestScopeLinearCatalog$' ./internal/trace/catalog/...` (exit 0, matching what `golc.ps1 test --quick --scope linear-catalog` would run) followed by a built `golc-project` binary invocation of `linear validate --offline` with `GOLC_PROJECT_ROOT` set (exit 0) — since the host's `go` toolchain already matched the pinned `go 1.26.5` version exactly. No production behavior differs; only the invocation wrapper was substituted.

## Known Stubs

- None. Both routes are fully wired to the real `catalog.BuildCatalog`/`catalog.Validate` implementation from Plan 08; there is no mocked or placeholder data path.

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-21 (route/fixture spoofing/tampering) is mitigated by exact `MustDeclareRoute` registration plus the ID/rename/adversarial fixture coverage; T-01-23 (information disclosure) is mitigated by the allowlisted `catalogEntityView` projection (no absolute paths, no remote/authority internals); T-01-SC (dependency tampering) is mitigated by adding zero new dependencies (`go.mod`/`go.sum` untouched — both new command files import only the standard library and the existing `internal/trace/catalog` package).

## User Setup Required

None - everything is repository-local; no credentials, npm, network, or Linear access involved. Both routes fail closed (`GOLC_ROUTE_UNKNOWN`/`GOLC_CATALOG_*`) rather than attempting any network call.

## Next Phase Readiness

- Plan 01-22 (`depends_on` this plan's route ownership) can extend `runLinearValidate` in `internal/command/linear_validate.go` with map/schema reconciliation checks without redeclaring the `linear validate` route.
- `linear catalog --offline --format json` gives contributors and downstream tooling a stable, allowlisted, offline view of the complete planning identity graph before any Linear mapping work begins.
- The `linear-catalog` quick-test scope (`golc.ps1 test --quick --scope linear-catalog`) now also exercises the new fixtures on every run, so a future adversarial-fixture regression fails the same gate Plan 08 established.

## Self-Check: PASSED

- All eight created/modified files exist on disk: `internal/command/linear.go`, `internal/command/linear_validate.go`, `internal/trace/catalog/catalog_test.go`, and the six `tests/fixtures/linear/*.json` fixtures.
- Commits `f493ce4` (test) and `650ff19` (feat) exist in git history on branch `worktree-agent-ae21d83d41fe2d3ff`.
- `go build ./...`, `go vet ./...`, and `go test -count=1 ./...` all pass from the repository root; `linear validate --offline` and `linear catalog --offline --format json` both resolve and exit 0 through a built `golc-project` binary.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
