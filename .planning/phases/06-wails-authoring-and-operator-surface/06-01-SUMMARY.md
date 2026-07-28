---
phase: 06-wails-authoring-and-operator-surface
plan: 01
subsystem: playback
tags: [go, sqlite, cli, operator-surface, midi, show-state]

# Dependency graph
requires:
  - phase: 05-durable-shows-and-recovery
    provides: SQLite-backed show.State store (Load/Save/Migrate) and the single validate() entry point every domain object extends
provides:
  - internal/operatorsurface package: Surface named-collection model with individual scene/layer/master/safety refs and per-surface MIDI mapping set
  - operatorsurface.Validate wired into show.State's single validate() entry point
  - show.State.OperatorSurfaces field, additive schema_version 1->2 migration
  - "operatorsurface create/list/assign/unassign/show" CLI routes
  - command.Authorize server-side visible-but-locked enforcement helper
affects: [06-05-wails-host, 06-07-surface-ui, 06-08-midi-learn]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Named-collection + individual-item-ref model (mirrors internal/pool.Group/MemberRef) applied to operator surfaces"
    - "Discriminated-union ControlRef (Kind + one populated value field) as the single control-identity representation shared by MIDI mapping targets and server-side Authorize"
    - "Additive schema migration: optional/omitempty field bump registers an identity-transform migrations[N] entry rather than a byte transform"

key-files:
  created:
    - internal/operatorsurface/model.go
    - internal/operatorsurface/model_test.go
    - internal/operatorsurface/validate.go
    - internal/operatorsurface/validate_test.go
    - internal/command/operatorsurface.go
    - internal/command/operatorsurface_test.go
  modified:
    - internal/show/state.go
    - internal/show/state_test.go
    - internal/show/migrate.go
    - internal/show/migrate_test.go

key-decisions:
  - "ControlRef is a discriminated-union struct (Kind + Scene/Layer/Master/Safety fields), not an interface{} or per-call-site type, so JSON strict-decoding and equality comparisons stay simple and there is exactly one control-identity representation for both MidiMapping.Target and command.Authorize"
  - "The v1->v2 schema migration is registered as a real production entry (migrations[1]) rather than test-only, since OperatorSurfaces is genuinely the first field added since SchemaVersion was pinned at 1"
  - "AddMidiMapping mints the MidiMapping's UUIDv7 ID itself (ignoring any ID on the passed candidate), mirroring NewPool/NewPoolMember's 'identity minted at construction, never caller-supplied' discipline"

patterns-established:
  - "operatorsurface's Assign*/Unassign* mutators are copy-returning and idempotent; AddMidiMapping is the sole exception (hard rejection on conflict, D-06)"

requirements-completed: [PLAY-03]

coverage:
  - id: D1
    description: "operatorsurface.Surface model: named collection with individual scene/layer/master/safety refs, no bulk/category ref type, per-surface MIDI mapping set with conflict rejection"
    requirement: "PLAY-03"
    verification:
      - kind: unit
        ref: "internal/operatorsurface/model_test.go#TestSurfaceModelIdentityStable"
        status: pass
      - kind: unit
        ref: "internal/operatorsurface/model_test.go#TestSurfaceModelAssignSceneIdempotent"
        status: pass
      - kind: unit
        ref: "internal/operatorsurface/model_test.go#TestSurfaceModelMidiMappingConflictRejected"
        status: pass
    human_judgment: false
  - id: D2
    description: "operatorsurface.Validate wired into show.State's single validate() entry point: unique surface names, dangling scene/layer/group-master reference rejection"
    requirement: "PLAY-03"
    verification:
      - kind: unit
        ref: "internal/operatorsurface/validate_test.go#TestSurfaceValidateDanglingSceneReferenceRejected"
        status: pass
      - kind: unit
        ref: "internal/show/state_test.go#TestShowStateOperatorSurfaceValidation"
        status: pass
      - kind: unit
        ref: "internal/show/migrate_test.go#TestMigrateAppliesOperatorSurfacesAdditiveMigration"
        status: pass
    human_judgment: false
  - id: D3
    description: "operatorsurface create/list/assign/unassign/show CLI routes, and server-side Authorize rejecting a control not assigned to the active surface (D-04/ASVS V4)"
    requirement: "PLAY-03"
    verification:
      - kind: unit
        ref: "internal/command/operatorsurface_test.go#TestOperatorSurfaceAssignSceneIdempotentAndShowReflectsIt"
        status: pass
      - kind: unit
        ref: "internal/command/operatorsurface_test.go#TestOperatorSurfaceAuthorizeRejectsUnassignedControl"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-23
status: complete
---

# Phase 6 Plan 1: Operator-Surface Data Foundation Summary

**Named, individually-assigned constrained operator-surface model (`internal/operatorsurface`) with per-surface MIDI mapping conflict rejection, wired into `show.State`'s single validate() entry point via an additive schema_version 1->2 migration, plus a full CLI (create/list/assign/unassign/show) and server-side `Authorize` visible-but-locked enforcement.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-23
- **Tasks:** 3
- **Files modified:** 10 (6 created, 4 modified)

## Accomplishments
- `internal/operatorsurface.Surface`: UUIDv7 identity minted once, individual scene/layer/master/safety-control assignment lists (no bulk/category ref type anywhere, D-03), copy-returning idempotent Assign*/Unassign* mutators (PLAY-03 idempotency edge), and per-surface `MidiMappings` with `AddMidiMapping` hard-rejecting a colliding (channel, kind, number) tuple (D-06 — no silent overwrite, no last-writer-wins)
- `operatorsurface.Validate` (unique names, then referential integrity for scene/layer/group-master refs) wired into `show.State`'s single `validate()` entry point as one more step, not a parallel path
- `show.State.OperatorSurfaces` added as an optional/omitempty field; `show.SchemaVersion` bumped 1->2 with a real production `migrations[1]` additive-identity entry, so a genuinely pre-field v1 `.golc` still opens (via `Migrate`) with an empty `OperatorSurfaces` slice
- `operatorsurface create/list/assign/unassign/show` CLI routes following `playback.go`'s parse-args -> Load -> mutate -> Save -> Stdout shape, resolving `--scene`/`--layer <scene>:<kind>`/`--master grand|group:<name>`/`--safety <control>` selectors against the loaded show's real scenes/groups
- `command.Authorize(surface, control)`: the server-side visible-but-locked enforcement point (D-04/ASVS V4) — rejects `GOLC_OPERATORSURFACE_LOCKED` for any control not currently in the surface's assignment set, ready for the Wails host (06-05/06-07) and MIDI dispatch (06-08) to call directly

## Task Commits

Each task was committed atomically:

1. **Task 1: operatorsurface model — named surfaces, individual-item refs, per-surface MIDI mappings** - `556ef4e` (feat)
2. **Task 2: operatorsurface validation wired into show.State, with additive schema migration** - `7cad135` (feat)
3. **Task 3: operatorsurface CLI routes + server-side visible-but-locked authorization** - `e448274` (feat)

## Files Created/Modified
- `internal/operatorsurface/model.go` - Surface/LayerRef/MasterRef/ControlRef/MidiMapping types, NewSurface/Rename/Assign*/Unassign*/AddMidiMapping/IsAssigned/ValidateUniqueSurfaceNames
- `internal/operatorsurface/model_test.go` - identity, idempotent assignment, MIDI-conflict-rejection, copy-returning, IsAssigned tests
- `internal/operatorsurface/validate.go` - `Validate(surfaces, scenes, groups)`: unique names then referential integrity
- `internal/operatorsurface/validate_test.go` - duplicate-name and dangling-reference tests
- `internal/command/operatorsurface.go` - CLI routes + `Authorize` server-side membership check
- `internal/command/operatorsurface_test.go` - CLI round-trip, idempotent assign, unknown-selector rejection, Authorize tests
- `internal/show/state.go` - `OperatorSurfaces` field on `State`, `SchemaVersion` bumped to 2, `operatorsurface.Validate` call added to `validate()`
- `internal/show/state_test.go` - `TestShowStateOperatorSurfaceValidation` (dangling ref rejected, valid surface round-trips)
- `internal/show/migrate.go` - registered `migrations[1] = migrateOperatorSurfacesAdditive` (identity transform) as the first real production migration entry
- `internal/show/migrate_test.go` - `TestMigrateAppliesOperatorSurfacesAdditiveMigration` (pre-field v1 blob still opens via Migrate with empty OperatorSurfaces)

## Decisions Made
- `ControlRef` is a discriminated-union struct (`Kind` + `Scene`/`Layer`/`Master`/`Safety` value fields) rather than an `interface{}` or ad hoc per-call-site type — keeps strict JSON decoding simple and gives `MidiMapping.Target` and `command.Authorize` exactly one shared control-identity representation.
- The v1->v2 migration is a real production `migrations[1]` registry entry (not test-only via `RegisterTestMigration`), since `OperatorSurfaces` is genuinely the first field added since `SchemaVersion` was pinned at 1 in Phase 5.
- The "old-blob-opens" test lives in `internal/show/migrate_test.go` rather than `state_test.go` (as the plan's file list suggested): `state_test.go`'s package (`show_test`) has no access to the internal `openStore`/raw-seeding helpers needed to construct a genuine pre-field v1 blob, while `migrate_test.go` (package `show`) already has exactly that machinery (`seedRawShow`/`fixturePayload`), matching the existing `TestMigrateAppliesRegisteredTransforms` precedent. The round-trip-through-the-public-API half of the "old blob opens + surface round-trip" requirement was added to `state_test.go` as planned.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed a helper to avoid a package-level redeclaration collision**
- **Found during:** Task 3 (`internal/command/operatorsurface.go`)
- **Issue:** `internal/command/scene.go` already declares a package-level `sceneByName(scenes []scene.Scene, name string) (scene.Scene, int, bool)` helper. My first draft of `operatorsurface.go` declared a second `sceneByName` with a different signature, which would fail to compile (duplicate declaration in the same package).
- **Fix:** Removed the duplicate declaration and reused the existing `scene.go` helper (adapting call sites to its 3-return signature). `groupByName` had no existing collision and was kept as a new helper local to `operatorsurface.go`.
- **Files modified:** internal/command/operatorsurface.go
- **Verification:** `go build ./...` succeeds; `go test ./internal/command/...` passes.
- **Committed in:** e448274 (Task 3 commit — the collision was caught and fixed before the first commit of this file, so no separate fix-up commit was needed)

---

**Total deviations:** 1 auto-fixed (1 blocking/Rule 3)
**Impact on plan:** No scope creep — a pre-existing package-level helper was reused instead of duplicated.

## Issues Encountered

- **Pre-existing, out-of-scope test failure (not touched, not fixed):** `go test ./...` at the repo root shows `internal/trace/catalog` failing (`TestScopeLinearCatalog`/`TestScopeLinearMap`, `GOLC_CATALOG_ID_INVALID: requirement key "TBD" does not match the KEY-NN grammar`). This is caused by `.planning/phases/06-wails-authoring-and-operator-surface/06-VALIDATION.md` carrying literal `TBD` placeholder Task ID/Plan-Wave/Threat-Ref cells (its own doc comment says these are "pending PLAN.md creation ... to be backfilled by gsd-plan-checker/gsd-verifier"). Verified pre-existing by stashing all of this plan's changes and re-running the same test — it fails identically with only Task 1 committed and zero operatorsurface changes present. Out of scope for 06-01-PLAN.md's own task list (no task here touches 06-VALIDATION.md); left for the phase's own doc-backfill or verifier pass.

## Next Phase Readiness

- `internal/operatorsurface` is a stable, CLI-testable contract: later Wails frontend slices (06-05 host, 06-07 surface UI) and MIDI learn/dispatch (06-08) can persist and authorize against `Surface`/`ControlRef`/`command.Authorize` without any GUI dependency.
- The per-surface `MidiMappings` shape (D-07) and `AddMidiMapping`'s conflict-rejection (D-06) are ready for 06-08's MIDI learn session to drive directly — no model changes anticipated there.
- `06-VALIDATION.md`'s `TBD` cells for PLAY-03 (row 2) should be backfilled with this plan's actual task IDs / commit refs during the phase's verifier pass; this plan does not do so itself (out of scope per its own task list).

---
*Phase: 06-wails-authoring-and-operator-surface*
*Completed: 2026-07-23*

## Self-Check: PASSED

All 10 claimed files verified present via `git ls-files`; all 3 task commit hashes (`556ef4e`, `7cad135`, `e448274`) verified present via `git log --oneline --all`.
