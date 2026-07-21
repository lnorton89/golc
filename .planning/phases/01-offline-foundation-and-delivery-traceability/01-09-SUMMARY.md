---
phase: 01-offline-foundation-and-delivery-traceability
plan: 09
subsystem: linear-traceability
tags: [go, json, catalog, migration, offline, linear, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 08
    provides: BuildCatalog dynamic discovery, durable local ID grammar, and the typed D-11 authority split
provides:
  - Duplicate-safe strict JSON guard (internal/strictjson): rejects duplicate object member names at any nesting level, more than one top-level JSON value, and unknown fields, before any typed decode
  - Deterministic canonical JSON encoder (LF-terminated, sorted map keys, idempotent)
  - MigrateV1ToV2/CheckMigration/WriteMigration: lossless schema-1-to-2 migration of .planning/linear-map.json that preserves the seed exactly and dynamically extends to the complete local catalog
  - .planning/linear-map.json migrated to schema 2: every phase/requirement/plan/task carries a pending/null remote mapping, credential-free
affects: [01-21, 01-22, 01-23, 01-24, linear-traceability]

tech-stack:
  added: []
  patterns:
    - Strict JSON via manual recursive-descent token walking (encoding/json.Decoder.Token) rather than trusting encoding/json's silent last-value-wins duplicate handling
    - Schema-1/schema-2-compatible Go struct (Map) so BuildCatalog's existing generic linear-map.json loader keeps working unchanged after migration
    - Migration preserves any existing remote_mappings entry verbatim by repo_id and only synthesizes pending/null entries for entities not yet mapped, making Write byte-idempotent and safe to re-run after a real future sync
    - Atomic write via os.CreateTemp in the same directory plus os.Rename (mirrors internal/projectconfig/local.go's WriteLocal pattern)

key-files:
  created:
    - internal/strictjson/decode.go
    - internal/strictjson/decode_test.go
    - internal/trace/catalog/migrate.go
    - internal/trace/catalog/migrate_test.go
    - tests/fixtures/linear/map-schema1.json
    - tests/golden/linear-map-schema2.json
  modified:
    - .planning/linear-map.json

key-decisions:
  - "Schema-2 Map reuses the exact schema-1 repository/active_milestone/remote_mappings shape and adds only an entities array; this keeps BuildCatalog's existing generic seed loader (parse.go) working unchanged on the migrated file and lets one Go struct decode either schema version."
  - "The project entity itself is never remote-mapped (matches schema 1, where only milestone:v1 had a remote_mappings entry); linear_type per kind is milestone->project, phase->project_milestone, requirement/plan/task->issue, matching the research/STACK.md Linear hierarchy table."
  - "MigrateV1ToV2 preserves any existing remote_mappings entry verbatim by repo_id and only synthesizes new pending/null entries for entities not yet mapped, so re-running WriteMigration after a real future Linear sync never resets a synced entry back to pending."
  - "requirements-completed intentionally left empty: LINR-01/LINR-02 are also declared by plans 01-21 and 01-22 (offline catalog inspection and the map contract/pending-linkage report), matching the conservative precedent set in 01-08's summary — this plan advances the requirement without claiming it complete alone."

requirements-completed: []

duration: ~25min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 09: Migrate the Seed to a Complete Credential-Free Schema-2 Map Summary

**A recursive-descent strict JSON guard (duplicate-name and multiple-value rejection before typed decode) plus a MigrateV1ToV2/CheckMigration/WriteMigration pipeline that losslessly migrates .planning/linear-map.json from the two-ID schema-1 seed to a dynamically complete, atomically-written, byte-idempotent schema-2 map**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-07-21T00:14:00Z
- **Completed:** 2026-07-21T00:38:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 7 (6 created, 1 migrated)

## Accomplishments

- `internal/strictjson/decode.go` walks JSON as a token stream via a recursive-descent `validateValue` over `encoding/json.Decoder`, rejecting duplicate object member names at any nesting level (`STRICTJSON_DUPLICATE_NAME`) and more than one top-level JSON value (`STRICTJSON_MULTIPLE_VALUES`) before `DecodeStrict` layers `DisallowUnknownFields` typed decoding on top. `CanonicalEncode` renders deterministic, LF-terminated, idempotent JSON (map keys are already sorted by `encoding/json`; struct field order is source-fixed).
- `internal/trace/catalog/migrate.go` implements `MigrateV1ToV2`: it strictly decodes the current `.planning/linear-map.json` (accepting either schema 1 or 2, since both share the same `repository`/`active_milestone`/`remote_mappings` shape), calls `BuildCatalog` for the complete dynamic local catalog, and produces a schema-2 `Map` whose `entities` array mirrors every catalog entity and whose `remote_mappings` array preserves any already-recorded mapping verbatim by `repo_id` while synthesizing a fresh `pending`/null mapping (typed by kind: milestone→project, phase→project_milestone, requirement/plan/task→issue) for everything not yet mapped. `validateMap` enforces completeness (every non-project entity mapped exactly once, no orphan mappings, the project root never mapped) and the credential-free invariant (a `pending` mapping can never carry a UUID, identifier, or URL) before anything is written.
- `CheckMigration` is a pure read-only byte comparison against the committed file; `WriteMigration` stages the canonical bytes through `os.CreateTemp` in `.planning/` and replaces atomically via `os.Rename`, so running it twice without any repository change is byte-identical (verified directly in tests, including confirming no leaked `.tmp-*` files).
- `.planning/linear-map.json` is migrated to schema 2 by actually running `WriteMigration` against the real repository: `project:golc`/`milestone:v1` and the existing pending/null `milestone:v1` mapping are preserved exactly, and every dynamically discovered phase, requirement, plan, and executable task (currently 8 requirements and 29 plans) now carries an `entities` record plus a pending/null `remote_mappings` record — all credential-free, nothing invented.
- `tests/fixtures/linear/map-schema1.json` is a standalone copy of the legacy schema-1 seed; `tests/golden/linear-map-schema2.json` is the canonical byte-for-byte expected migration output for the fixture repository built from it, asserted in `migrate_test.go`.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: strict JSON guard and schema-1-to-2 migration contract** - `f9cf5d9` (test) — both packages failed with `[build failed]` (no implementation); confirmed by temporarily removing `decode.go`/`migrate.go` and re-running `go test`.
2. **GREEN - Task 1: implementation, fixture, golden, and real migration** - `4a6a73d` (feat) — full suite passes, including `go test -race ./...` and `go vet ./...`.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/strictjson/decode.go` - `ValidateSingleValueNoDuplicateNames`, `DecodeStrict`, `CanonicalEncode`.
- `internal/strictjson/decode_test.go` - External test package; scope `linear-map`; marker `TestScopeLinearMap`; duplicate/multiple-value/unknown-field/canonical-encode coverage.
- `internal/trace/catalog/migrate.go` - `Map`/`EntitySummary`/`RemoteMapping` types, `linearTypeForKind`, `MigrateV1ToV2`, `validateMap`, `CheckMigration`, `WriteMigration`.
- `internal/trace/catalog/migrate_test.go` - Joins the `catalog_test` package; same scope `linear-map` and marker `TestScopeLinearMap`; seed preservation, linear-type assignment, malformed-seed rejection, read-only Check/atomic-idempotent Write, preserved-synced-mapping re-run, fixture/golden byte match, credential canary, and real-repository end-to-end coverage.
- `tests/fixtures/linear/map-schema1.json` - Standalone legacy schema-1 seed fixture (identical shape to the pre-migration `.planning/linear-map.json`).
- `tests/golden/linear-map-schema2.json` - Canonical schema-2 migration output for the fixture repository.
- `.planning/linear-map.json` - Migrated in place to schema 2 via `WriteMigration` against the real repository.

## Decisions Made

- **One struct for both schema versions:** `Map` decodes schema 1 and schema 2 alike (schema 1 documents simply leave `entities` empty), so `MigrateV1ToV2` is naturally idempotent across repeated runs without a separate legacy-vs-current code path.
- **Project entity never remote-mapped:** matches the schema-1 precedent exactly (only `milestone:v1` had a `remote_mappings` entry) and the research hierarchy table (release/milestone → Linear Project; the repository root itself is not a Linear object).
- **Existing mappings preserved verbatim, never reset:** tested explicitly by simulating a "linked" mapping (fake UUID/identifier/URL) and re-running `WriteMigration`, confirming the synced fields survive re-migration unchanged.
- **No orphan-mapping preservation for removed entities:** if a previously-mapped local ID later disappears from the dynamic catalog (e.g., a plan file removed), this plan does not carry its old mapping forward into an archived bucket — full D-15 explicit-archive/unlink semantics are deliberately deferred to the reconcile/apply plans (01-23/01-24) per `01-PATTERNS.md`'s package boundary, not implemented here.
- **No `synced_at`/normalized-sync-baseline field yet:** `01-PATTERNS.md`'s "Extend by contract" note lists a normalized sync baseline as part of the eventual schema-2 shape, but D-13 three-way conflict detection belongs to the reconcile plans; adding an always-null placeholder field now would be speculative, so it is left as a straightforward, additive extension for whichever of 01-23/01-24 first needs it.

## Deviations from Plan

None - plan executed exactly as written. The schema-2 field design (entities/remote_mappings shape, linear_type-per-kind mapping) falls within the plan's "agent's discretion" and `01-PATTERNS.md`'s "Extend by contract" guidance; no Rule 1-3 auto-fixes were needed and no files outside the seven owned paths were touched.

## Issues Encountered

- This worktree has no bootstrapped `.tools/` pinned toolchain (bootstrap was not run in this isolated worktree), so `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-map` could not be invoked directly. Verified instead with the host Go toolchain, which is `go1.26.5 windows/amd64` — an exact match for the pinned `config/toolchain.toml` version — running `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the exact scope-dispatch equivalent (`go test -list ^TestScopeLinearMap$ ./...` followed by `go test -run ^TestScopeLinearMap$` against the two matched packages), all passing.

## Known Stubs

- None blocking this plan's goal. The migrated map has no CLI route consuming it yet (no `linear` command surface exists in this phase's file inventory); downstream plans 01-21 (offline catalog inspection) and 01-22 (map contract/pending-linkage report) wire it further. This is the planned growth path noted in 01-08's summary, not a missing wire for this plan.

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-24 (tampering via duplicate/unknown JSON) is mitigated by `ValidateSingleValueNoDuplicateNames` plus `DisallowUnknownFields`; T-01-25 (remote identity spoofing) by nullable-only remote fields and never synthesizing a UUID/identifier/URL; T-01-26 (information disclosure) by the schema/model carrying only repository-derived IDs/display/text plus null remote fields, verified by an explicit environment-canary test that confirms migration output never leaks an unrelated value.

## User Setup Required

None - everything is repository-local; no credentials, npm, network, or Linear access involved.

## Next Phase Readiness

- Plans 01-21 and 01-22 (`depends_on` chains through 01-08/01-09) can consume `catalog.MigrateV1ToV2`/`CheckMigration`/`WriteMigration` and the stable `GOLC_MIGRATE_*` diagnostics to expose offline catalog inspection and a pending-linkage report.
- Plans 01-23/01-24 (preview/apply reconciliation) can extend `RemoteMapping` with sync-baseline and D-13 conflict fields without touching the already-stable `repo_id`/`linear_type`/`status`/`linear_uuid`/`identifier`/`url` shape.
- The `linear-map` quick-test scope spans `internal/strictjson` and `internal/trace/catalog` and joins `config-local`/`linear-catalog` as a standing quick gate: `golc.ps1 test --quick --scope linear-map` (once `.tools/` is bootstrapped in a given environment).

## Self-Check: PASSED

- All six created files exist on disk (`internal/strictjson/decode.go`, `internal/strictjson/decode_test.go`, `internal/trace/catalog/migrate.go`, `internal/trace/catalog/migrate_test.go`, `tests/fixtures/linear/map-schema1.json`, `tests/golden/linear-map-schema2.json`), and `.planning/linear-map.json` is migrated to schema 2 on disk.
- Commits `f9cf5d9` (test) and `4a6a73d` (feat) exist in git history on branch `worktree-agent-acd92264ff6d24e0a`.
- `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` all pass from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly).

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*

## Self-Check: PASSED (verified post-write)

- All eight paths above (six created plan files, the migrated `.planning/linear-map.json`, and this SUMMARY.md) confirmed present on disk via direct file existence checks.
- Commits `f9cf5d9` (test) and `4a6a73d` (feat) confirmed present via `git log --oneline --all`.
