---
phase: 01-offline-foundation-and-delivery-traceability
plan: 08
subsystem: linear-traceability
tags: [go, catalog, durable-ids, offline, linear, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 02
    provides: Generic quick-test dispatcher (test --quick --scope) and the external-test-package MustDeclareScope pattern
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: Self-registering command registry contract and pinned-toolchain execution conventions
provides:
  - Durable offline local ID grammar (project:slug, milestone:vN, phase:NN, req:KEY-NN, plan:NN-MM, task:NN-MM.p) derived only from structural metadata
  - BuildCatalog dynamic discovery of phase directories, NN-MM-PLAN.md files (numeric sort, no fixed count), and executable XML task positions
  - Validators for ID grammar/uniqueness, parent kinds, acyclicity, structural containment, .planning source containment
  - Fixed typed D-11 authority split (repository vs Linear operational fields) with D-12 comment exclusion
  - Quick-test scope linear-catalog with marker TestScopeLinearCatalog
affects: [01-09, 01-21, 01-22, linear-traceability]

tech-stack:
  added: []
  patterns:
    - Durable IDs from structural metadata only (seed IDs, two-digit numbers, XML task ordinals); display text is rename-safe and never identity (D-14)
    - Fixed typed authority registry instead of negotiated field ownership; comment/discussion fields unrepresentable and rejected (D-11/D-12)
    - Stdlib-only parsing (encoding/json, regexp, strings) over planning Markdown/JSON; no new dependencies (T-01-SC)
    - Catalog validation runs inside BuildCatalog so no unvalidated graph can be returned

key-files:
  created:
    - internal/trace/catalog/model.go
    - internal/trace/catalog/id.go
    - internal/trace/catalog/parse.go
    - internal/trace/catalog/validate.go
    - internal/trace/catalog/catalog_test.go
  modified: []

key-decisions:
  - "Executable-task identity is the 1-based position among ALL task elements in a plan's <tasks> block; checkpoint tasks keep their position but receive no catalog entity, so inserting or removing a checkpoint never renumbers neighbours silently while only type=\"auto\" tasks are executable."
  - "Plan filename grammar is enforced loudly: anything ending in -PLAN.md that is not NN-MM-PLAN.md with the owning directory's phase prefix fails GOLC_CATALOG_PLAN_FILENAME instead of being skipped."
  - "Frontmatter is cross-checked against filename structure (plan number and phase directory slug) with GOLC_CATALOG_PLAN_FRONTMATTER, so a copied or drifted plan file cannot mint a wrong identity."
  - "The D-11 authority split is a fixed typed registry — SetAuthority and ValidateAuthorities both reject reassigning repository fields (scope, local_id, requirement_text, structure) to Linear, claiming Linear operational fields (status, assignee, priority, estimate, completed_at) for the repository, storing comment/discussion fields, and unknown or missing fields."
  - "LINR-01/LINR-02 are NOT marked complete: plans 01-09, 01-21, and 01-22 still declare them, so this plan records progress without claiming requirement completion."

patterns-established:
  - "Stable catalog diagnostics: GOLC_CATALOG_ID_INVALID, GOLC_CATALOG_ID_DUPLICATE, GOLC_CATALOG_KIND_MISMATCH, GOLC_CATALOG_PARENT_UNKNOWN, GOLC_CATALOG_PARENT_KIND, GOLC_CATALOG_PARENT_MISMATCH, GOLC_CATALOG_CYCLE, GOLC_CATALOG_SOURCE_MISSING, GOLC_CATALOG_SOURCE_EXTERNAL, GOLC_CATALOG_PLAN_FILENAME, GOLC_CATALOG_PLAN_FRONTMATTER, GOLC_CATALOG_PLAN_TASKS_MISSING, GOLC_CATALOG_TASK_TYPE, GOLC_CATALOG_AUTHORITY_REPOSITORY_FIELD, GOLC_CATALOG_AUTHORITY_LINEAR_FIELD, GOLC_CATALOG_COMMENT_EXCLUDED, GOLC_CATALOG_FIELD_UNKNOWN, GOLC_CATALOG_AUTHORITY_INCOMPLETE, GOLC_CATALOG_SEED_INVALID, GOLC_CATALOG_REQUIREMENT_UNDEFINED, GOLC_CATALOG_ROADMAP_PHASE_MISSING, GOLC_CATALOG_ROADMAP_REQUIREMENTS_MISSING, GOLC_CATALOG_PHASE_DIRNAME, GOLC_CATALOG_SOURCE_UNREADABLE."
  - "Real-repository tests cross-check the catalog against an independent filesystem scan (plan-file glob plus task-tag scan) so no count is ever hard-coded."

requirements-completed: []

duration: 16min
completed: 2026-07-20
status: complete
---

# Phase 1 Plan 08: Repository-Owned Planning Identity Catalog Summary

**BuildCatalog turns linear-map.json, ROADMAP.md, REQUIREMENTS.md, and every dynamically discovered NN-MM-PLAN.md into a validated offline identity graph — project:golc through task:01-MM.p — where renames never change IDs, sources must live in .planning/, repository fields can never be reassigned to Linear, and comments cannot be stored**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-07-20T19:12:00Z
- **Completed:** 2026-07-20T19:28:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 5 (all created)

## Accomplishments

- `internal/trace/catalog/id.go` defines the durable local ID grammar (D-14): `project:slug`, `milestone:vN`, `phase:NN`, `req:KEY-NN`, `plan:NN-MM`, `task:NN-MM.p`. Every constructor derives IDs from structural metadata only — the seed IDs pinned in `linear-map.json`, two-digit phase/plan numbers, requirement keys, and 1-based XML task positions. Display titles and issue-key shapes (`GOLC-123`) cannot become identity.
- `internal/trace/catalog/parse.go` builds the complete catalog offline: schema-1 seed (`project:golc`, `milestone:v1`), roadmap phase structure and per-phase requirement keys (verified against `REQUIREMENTS.md` text), dynamic `NN-MM-PLAN.md` discovery sorted by parsed numeric plan ID with **no fixed plan-count assertion**, frontmatter/filename cross-checks, and one task entity per `type="auto"` task at its document position. Checkpoint tasks are positioned but not cataloged.
- `internal/trace/catalog/validate.go` enforces the graph invariants with stable `GOLC_CATALOG_*` codes: grammar/uniqueness, parent existence, acyclicity, parent-kind rules (task→plan→phase→milestone→project), structural containment (a plan can only parent to its own phase, a task to its own plan), and `.planning/` source containment rejecting absolute paths, `..` escapes, URLs, and anything outside the planning tree.
- `internal/trace/catalog/model.go` types the D-11 authority split as a fixed registry — repository fields (`scope`, `local_id`, `requirement_text`, `structure`) and Linear operational fields (`status`, `assignee`, `priority`, `estimate`, `completed_at`) — where reassignment in either direction fails, and D-12 comment exclusion makes `comment`/`comments`/`discussion`/`discussions` unstorable through both `SetAuthority` and post-hoc map tampering caught by `ValidateAuthorities`.
- `internal/trace/catalog/catalog_test.go` declares scope `linear-catalog` via `command.MustDeclareScope` beside marker `TestScopeLinearCatalog` (the 01-02 pattern). Real-repository subtests cross-check the catalog against an independent filesystem scan (currently 29 plans / 33 executable tasks, but asserted dynamically); fixture subtests cover numeric sort (01, 02, 10), checkpoint exclusion, rename immutability, grammar rejection, and every validator failure mode. `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-catalog` exits 0 with no `.env`, Node, network, or Linear access.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: catalog identity/authority contract** - `fb81138` (test) — scope failed with `[build failed]` (no implementation)
2. **GREEN - Task 1: model/id/parse/validate implementation** - `c36bee4` (feat) — scope and full suite pass

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/trace/catalog/model.go` - Entity/Catalog types, six kinds, rename-safe Display, fixed typed authority registry, comment exclusion, duplicate-rejecting Add.
- `internal/trace/catalog/id.go` - ID grammar patterns, ParseID decomposition, PhaseID/PlanID/TaskID/RequirementID structural constructors.
- `internal/trace/catalog/parse.go` - BuildCatalog: seed load, roadmap/requirements structure, dynamic plan discovery, XML task positions, validation before return.
- `internal/trace/catalog/validate.go` - ValidateIDs/ValidateHierarchy/ValidateSources/ValidateAuthorities plus aggregate Validate.
- `internal/trace/catalog/catalog_test.go` - External test package; scope `linear-catalog`; real-repo dynamic cross-checks plus synthetic fixture and validator rejection coverage.

## Decisions Made

- **Task position semantics:** the executable-task ordinal counts ALL `<task>` elements in the `<tasks>` block, matching how plans name tasks ("Task 2" is the second element regardless of type). Checkpoint tasks are excluded from the catalog (they are human gates, not executable repository work) but keep their position, so `task:01-01.3` stays stable when a checkpoint sits at position 2.
- **Loud grammar enforcement over silent skipping:** near-miss plan filenames (`01-3-PLAN.md`, `02-05-PLAN.md` in phase 01) and drifted frontmatter fail the whole build with stable codes rather than being ignored — a misnamed plan must never silently drop out of the identity graph.
- **Fixed authority registry:** rather than storing per-entity remote fields, ownership is a catalog-level typed map validated against the D-11 split; this makes "repository fields cannot be reassigned to Linear" a mechanical property (`SetAuthority` + `ValidateAuthorities`) instead of a convention.
- **Requirement completion withheld:** plan frontmatter lists LINR-01/LINR-02, but plans 01-09, 01-21, and 01-22 also declare them; marking them complete now would misstate delivery status under the Definition of Done, so REQUIREMENTS.md is unchanged.

## Deviations from Plan

None - plan executed exactly as written. (The choices above fall within the plan's discretion; no Rule 1-3 auto-fixes were needed and no files outside the owned five were touched.)

## Issues Encountered

- An untracked `README.md` (project overview, 8.5 KB) exists at the repository root. It was not created by this plan's execution and the session's starting status was clean, so it likely arrived concurrently. Logged in `deferred-items.md` for an owner decision; deliberately not committed here to avoid misattributing authorship.

## Known Stubs

- None in the catalog itself. The package is a pure library by design: no CLI route consumes it yet — downstream plans (01-09 strict JSON decoding, 01-21/01-22 mapping and reconciliation) wire it to commands. This is the planned growth path, not a missing wire for this plan's goal.

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-21 (identity spoofing/tampering) is mitigated by the filename/ID grammar, uniqueness, parent/cycle/source validators; T-01-22 (authority split) by the typed registry and comment exclusion; T-01-SC by stdlib-only parsing with no new dependencies (go.mod untouched).

## User Setup Required

None - everything is repository-local; no credentials, npm, network, or Linear access involved.

## Next Phase Readiness

- Plan 01-09 (`depends_on: [01-08]`) can consume `BuildCatalog`/`Validate` and the stable `GOLC_CATALOG_*` diagnostics.
- Plans 01-21/01-22 can map catalog IDs to remote UUIDs through `.planning/linear-map.json` without touching local identity — the seed boundary is already grammar-checked.
- The `linear-catalog` scope joins `config-local` as a standing quick gate: `golc.ps1 test --quick --scope linear-catalog`.

## Self-Check: PASSED

- All five created files exist on disk (`internal/trace/catalog/model.go`, `id.go`, `parse.go`, `validate.go`, `catalog_test.go`).
- Commits `fb81138` (test) and `c36bee4` (feat) exist in git history.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-catalog` exits 0 from the repository root; the full pinned-toolchain suite (`go test ./...`) passes.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-20*
