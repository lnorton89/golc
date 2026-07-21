---
phase: 01-offline-foundation-and-delivery-traceability
plan: 10
subsystem: linear-traceability
tags: [go, json, sha256, reconcile, linear, quick-tests]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 08
    provides: BuildCatalog dynamic discovery, durable local ID grammar (catalog.ParseID), and the typed D-11 authority split
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 09
    provides: internal/strictjson.CanonicalEncode (deterministic LF-terminated JSON) and catalog.RemoteMapping/schema-2 linear-map.json
provides:
  - internal/trace/reconcile package (Intent/RemoteObservation/RemoteScope/SyncBaseline/Operation/Conflict/Plan types)
  - BuildPlan: deterministic SHA-256-digested D-17 preview with D-13 three-way conflict blocking
  - DigestIntent/DigestMapping/DigestRemoteScope/PlanID canonical hashing functions
  - SortOperations: fixed D-17 hierarchy ordering (Project Milestone -> Phase -> parent/requirement Issue -> task sub-issue) with local-ID tie-break
  - RenderMarker/ParseMarker/ValidateMarkerIdentity: the visible D-14 "GOLC local ID / GOLC mapping schema" footer contract
  - tests/golden/linear-preview.json and tests/golden/linear-conflict-preview.json canonical goldens
affects: [01-11, 01-23, 01-24, linear-traceability]

tech-stack:
  added: []
  patterns:
    - Canonical plan body excludes plan_id itself (plan_id = sha256(canonical_body)); an unexported planBody struct in canonical.go is what actually gets hashed, then PlanID is stamped onto the returned Plan
    - Every digested input (intents, mappings, remote observations) is copied and sorted by local ID before encoding, so BuildPlan/digest functions never depend on caller-supplied or map-derived traversal order
    - D-13 three-way conflict check only fires when a field has an observed remote value AND a recorded baseline value; a field with no baseline/observation is treated as a plain create/update, never a phantom conflict
    - A conflicted entity is excluded from Operations entirely (not included as an unapplyable operation record) — only its Conflict record appears, matching the plan's "conflict operations are not applyable" requirement
    - ValidateMarkerIdentity derives expected parent local ID from plan/task ID grammar itself (phase/plan numbers embedded in the ID) rather than needing a separate lookup, since requirement/phase/milestone IDs carry no parent structure in their own grammar

key-files:
  created:
    - internal/trace/reconcile/model.go
    - internal/trace/reconcile/canonical.go
    - internal/trace/reconcile/marker.go
    - internal/trace/reconcile/reconcile_test.go
    - tests/golden/linear-preview.json
    - tests/golden/linear-conflict-preview.json
  modified: []

key-decisions:
  - "reconcile imports catalog directly (catalog.Kind, catalog.RemoteMapping, catalog.ParseID) rather than duplicating the kind/grammar contract, keeping the D-17 hierarchy rank and D-14 marker parent-validation logic bound to the single source of truth for local-ID grammar (internal/trace/catalog/id.go)."
  - "DependsOn only names a parent when that parent's own kind is remote-managed (i.e. not KindProject): the repository-root project entity is never remote-mapped, so a milestone's operation has no dependency even though its catalog Parent is project:golc."
  - "Operation.Before/After are canonically re-marshaled field maps (sorted keys via an explicit ordered map, not raw pass-through), so byte-stability holds even if a caller supplies fields in different Go map iteration order across calls."
  - "The three-way conflict rule requires disagreement on all three legs (base != repo, base != linear, repo != linear) before blocking; if only the repository changed (base==linear) or only Linear changed (base==repo), or both sides converged to the same value (repo==linear), the field proceeds as a normal update, matching CONTEXT D-13's 'if both sides changed the same field' wording exactly."
  - "The two golden fixtures are hand-authored small scenarios (five-kind clean hierarchy; two clean creates plus one blocked conflict), not derived from the real repository catalog, generated once via a temporary in-module generator program (internal/trace/reconcile/_gengolden, removed before commit — the leading underscore keeps it outside any go build/test/vet '...' wildcard) and committed as the byte-stable golden targets reconcile_test.go compares against."

requirements-completed: []

duration: ~35min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 10: Define Canonical Reconciliation Operations, Plan Hashing, Ordering, and Visible Identity Markers Summary

**A new internal/trace/reconcile package: SHA-256-digested, byte-stable D-17 preview plans (Intent/Operation/Conflict/Plan types, BuildPlan, PlanID), fixed hierarchy/tie-break operation ordering, D-13 three-way conflict blocking, and the visible parser-stable D-14 "GOLC local ID" identity footer (RenderMarker/ParseMarker/ValidateMarkerIdentity), proven against two committed canonical goldens**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-07-21T00:20:00Z
- **Completed:** 2026-07-21T00:55:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 6 (all created)

## Accomplishments

- `internal/trace/reconcile/model.go` defines the complete D-17/D-14 contract types: `Intent` (repository-owned desired state, `IntentFromEntity` helper deriving it from `catalog.Entity`), `RemoteObservation`/`RemoteScope` (already-normalized current remote state), `SyncBaseline` (last-synchronized three-way reference), `Operation` (local ID, remote type/UUID or discovery marker, `Before`/`After` canonical field snapshots, owned fields, `ExpectedUpdatedAt` precondition, `DependsOn`), `Conflict` (base/repository/Linear values plus an explicit resolution command), and `Plan` (schema version, three input digests, sorted operations/conflicts, `PlanID`). No comment/discussion or credential field exists anywhere in the contract (CONTEXT D-12).
- `internal/trace/reconcile/canonical.go` implements `DigestIntent`/`DigestMapping`/`DigestRemoteScope` (SHA-256 over sorted, canonically encoded copies of each input via `internal/strictjson.CanonicalEncode`), `hierarchyRank`/`SortOperations` (the fixed D-17 order: Project Milestone -> Phase -> parent/requirement Issue -> task sub-issue, tie-broken by local ID), and `BuildPlan`, which joins intents against the credential-free remote-mapping set, the captured remote scope, and the sync baseline to produce ordered operations, blocks any field where repository/Linear/base all disagree as a `Conflict` (excluding that entity's operation entirely), and stamps `PlanID = sha256(canonical_body)` from an unexported `planBody` that never includes `plan_id`, a timestamp, or a random value.
- `internal/trace/reconcile/marker.go` implements `RenderMarker` (renders the exact `---\nGOLC local ID: <id>\nGOLC mapping schema: 2\n` footer, validated against `catalog.ParseID` before rendering), `ParseMarker` (scans a full description for exactly one footer, erroring on zero-vs-ambiguous-vs-malformed rather than guessing), and `ValidateMarkerIdentity` (schema/local-ID/kind cross-check, plus a structural parent check for plan and task IDs whose grammar itself encodes the phase/plan numbers of their expected parent).
- `internal/trace/reconcile/reconcile_test.go` declares `MustDeclareScope(Scope: "linear-preview-contract")` and defines `TestScopeLinearPreviewContract` with eleven subtests: byte-stability across repeated `BuildPlan` calls, input-order independence of every digest, hierarchy/tie-break ordering, marker round-trip across all six entity-kind ID shapes, `ParseMarker` absent/ambiguous/malformed handling, `ValidateMarkerIdentity` accept-and-reject-per-failure-mode coverage, three-way conflict blocking with operation exclusion, missing-mapping rejection, both golden byte-for-byte comparisons, and an unrelated-environment-credential canary.
- `tests/golden/linear-preview.json` (clean five-kind hierarchy: milestone, phase, requirement, plan, task, all creates, zero conflicts) and `tests/golden/linear-conflict-preview.json` (phase and plan creates plus one requirement blocked by a three-way title disagreement) are the committed canonical byte targets, generated once via a temporary in-module generator program and removed before committing.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: canonical reconciliation preview contract** - `992a63a` (test) — `internal/trace/reconcile` had no non-test Go files; `go test ./internal/trace/reconcile/...` failed with `[build failed]`, confirmed by writing the test file against a package with only `reconcile_test.go` present (the three implementation files were staged out of the working tree for the RED run and restored immediately after).
2. **GREEN - Task 1: model/canonical/marker implementation and goldens** - `cc8de0a` (feat) — full suite passes, including `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...`.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/trace/reconcile/model.go` - `Intent`/`IntentFromEntity`, `RemoteObservation`/`RemoteScope`, `SyncBaseline`, `Operation`, `Conflict`, `Plan`, `planBody`, `Marker`.
- `internal/trace/reconcile/canonical.go` - `SchemaVersion`, `DigestBytes`/`DigestValue`, `DigestIntent`/`DigestMapping`/`DigestRemoteScope`, `hierarchyRank`/`SortOperations`, `BuildPlan`, `PlanID`.
- `internal/trace/reconcile/marker.go` - `MarkerSchema`, `RenderMarker`, `ParseMarker`, `ValidateMarkerIdentity`.
- `internal/trace/reconcile/reconcile_test.go` - External test package; scope `linear-preview-contract`; marker `TestScopeLinearPreviewContract`; eleven subtests covering byte-stability, ordering, marker round-trip/validation, conflict blocking, and both goldens.
- `tests/golden/linear-preview.json` - Canonical five-kind clean hierarchy preview.
- `tests/golden/linear-conflict-preview.json` - Canonical preview with two clean creates and one blocked D-13 conflict.

## Decisions Made

- **`reconcile` depends directly on `catalog`:** hierarchy ranking and marker parent-validation reuse `catalog.Kind`/`catalog.ParseID` rather than re-implementing the local-ID grammar, keeping identity rules single-sourced.
- **Project root never a dependency:** `DependsOn` omits a parent whose kind is `KindProject`, since the project entity is never remote-mapped (matches 01-09's precedent) — only milestone/phase/requirement/plan/task parents ever appear as dependencies.
- **Explicit sorted-key re-marshal for `Before`/`After`:** rather than trusting `encoding/json.Marshal` on a raw `map[string]string` (which does sort keys already, but implicitly), `canonicalFieldsJSON` builds an explicit ordered copy first so the byte-stability guarantee is visible in the code, not just an incidental property of the standard library.
- **Strict three-way conflict predicate:** a field only blocks when base, repository, and Linear values are pairwise distinct (all three legs disagree) — matches CONTEXT D-13's exact wording ("if both sides changed the same mapped field") rather than blocking on any repository/Linear difference regardless of baseline.
- **Golden generation via a disposable in-module program:** `internal/trace/reconcile/_gengolden/main.go` (underscore-prefixed so `go build/test/vet ./...` never touches it) built the two fixture `Plan` values with the exact same Go code path as the tests, wrote canonical bytes to `tests/golden/`, and was deleted before committing — the goldens are therefore real `BuildPlan` output, not hand-typed JSON.

## Deviations from Plan

None - plan executed exactly as written. All six frontmatter-declared files were created; no other files were touched. The three-way conflict merge logic, remote-scope/baseline input shapes, and golden-generation mechanics fall within the plan's "agent's discretion" (serialization format and hashing strategy for deterministic Linear preview plans) and the RESEARCH.md Pattern 4/5 recommendations; no Rule 1-3 auto-fixes were needed.

## Issues Encountered

- This worktree has no bootstrapped `.tools/` pinned toolchain (bootstrap was not run in this isolated worktree), so `powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-preview-contract` could not be invoked directly. Verified instead with the host Go toolchain, which is `go1.26.5 windows/amd64` — an exact match for the pinned `config/toolchain.toml` version — running `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the exact scope-dispatch equivalent (`go test -list ^TestScopeLinearPreviewContract$ ./...` followed by `go test -run ^TestScopeLinearPreviewContract$` against the one matched package), all passing.

## Known Stubs

- None blocking this plan's goal. `BuildPlan` is a complete, real implementation (not a placeholder) but this plan only defines the preview/canonicalization/marker contract per its file inventory — it does not yet wire a CLI route, real Linear transport, or apply/journal logic. Those are explicitly out of scope here and belong to later plans (apply/replay is 01-11 per `depends_on`; live preview/apply CLI surfaces are 01-23/01-24 per `affects`).

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-27 (tampering/repudiation via plan binding) is mitigated by `PlanID = sha256(canonical_body)` over sorted, canonically encoded inputs with no timestamp/randomness in the hashed body (verified by the byte-stability and input-order-independence tests). T-01-28 (identity-marker spoofing) is mitigated by `RenderMarker`/`ParseMarker` encoding only local ID and schema (never title), `ParseMarker` rejecting ambiguous/malformed footers instead of guessing, and `ValidateMarkerIdentity` cross-checking decoded kind and structural parent before any marker could be trusted.

## User Setup Required

None - everything is repository-local; no credentials, npm, network, or Linear access involved.

## Next Phase Readiness

- Plan 01-11 (`depends_on: [01-10]`) can consume `reconcile.BuildPlan`, `reconcile.Plan`/`Operation`/`Conflict`, and the `GOLC_RECONCILE_*` diagnostics to implement exact-plan apply, stale-preview rejection, and safe partial-apply resume (RESEARCH.md Pattern 5 apply algorithm), reusing `PlanID` to detect a changed source.
- Plans 01-23/01-24 (preview/apply CLI surfaces) can wire `reconcile.BuildPlan` to real repository intent (via `catalog.BuildCatalog`/`IntentFromEntity`) and a real Linear transport-observed `RemoteScope`/`SyncBaseline` without changing the already-stable `Plan`/`Operation`/`Conflict`/`Marker` JSON shapes.
- The `linear-preview-contract` quick-test scope joins `config-local`/`linear-catalog`/`linear-map` as a standing quick gate: `golc.ps1 test --quick --scope linear-preview-contract` (once `.tools/` is bootstrapped in a given environment).

## Self-Check: PASSED

- All six created files exist on disk (`internal/trace/reconcile/model.go`, `internal/trace/reconcile/canonical.go`, `internal/trace/reconcile/marker.go`, `internal/trace/reconcile/reconcile_test.go`, `tests/golden/linear-preview.json`, `tests/golden/linear-conflict-preview.json`).
- Commits `992a63a` (test) and `cc8de0a` (feat) exist in git history on branch `worktree-agent-ae0ae950f47d4ae9b`.
- `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` all pass from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly).

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
