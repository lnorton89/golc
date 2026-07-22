---
phase: 02-modular-fixtures-and-deployments
plan: 04
subsystem: pool-deployment-domain
tags: [uuid, uuidv7, pool, deployment, showstate, cli, strictjson]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments
    provides: "internal/fixture.CapabilityType/SupportedCapabilityTypes (02-01) -- Pool.RequiredCapabilities validates against this exact enum"
provides:
  - "internal/show.State/Load/Save: the revisioned ShowState substrate (SchemaVersion, Revision, Pools, Deployments, Groups) 02-05's impact-plan freshness guard checks (D-16)"
  - "internal/pool.Pool/PoolMember/Group/MemberRef: logical, count-independent fixture pools with rename-stable UUIDv7 identity (POOL-01)"
  - "internal/deployment.Deployment/Instance: named pool-to-concrete-instance mappings with an exactly-one-active invariant (POOL-02/D-09) and NextFreeAddress's bounded universe/address allocator (D-11 primitive)"
  - "golc pool create / deployment create / deployment activate / show inspect CLI routes (D-04, self-registered through internal/command)"
  - "github.com/google/uuid promoted to a direct dependency at v1.6.0"
affects: [02-05, 02-06]

# Tech tracking
tech-stack:
  added: ["github.com/google/uuid v1.6.0 (direct dependency, UUIDv7 identity minting)"]
  patterns:
    - "Identity minted only at creation time (uuid.NewV7()), never derived from a display name, never re-minted by Rename/Activate"
    - "Whole-State validation runs once inside internal/show's validate() and is reused by both Load and Save, so every GOLC_POOL_*/GOLC_DEPLOYMENT_* domain diagnostic surfaces wrapped in a single GOLC_SHOW_STATE_INVALID envelope at either checkpoint"
    - "CLI routes never validate duplicate names themselves -- they append then call show.Save, letting Save's validation be the single source of truth (mirrors internal/command/linear.go's preview/apply split: one call, one clear failure point)"
    - "Atomic write-temp-then-rename + strictjson.CanonicalEncode/DecodeStrict reused verbatim from internal/strictjson and internal/command/linear.go's writePreviewPlan shape"
    - "NextFreeAddress's same-width overlap simplification: since Instance does not yet carry its own channel-count field, every existing instance is conservatively treated as occupying the new instance's own channelCount when checking for overlap -- correct for same-width fixtures, over-conservative (never colliding) otherwise"

key-files:
  created:
    - internal/show/state.go
    - internal/show/state_test.go
    - internal/pool/model.go
    - internal/pool/model_test.go
    - internal/deployment/model.go
    - internal/deployment/model_test.go
    - internal/command/pool.go
    - internal/command/deployment.go
    - internal/command/pooldeploy_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "NextFreeAddress's occupied-span check assumes every existing Instance occupies the same channelCount as the instance currently being placed (Instance has no stored width yet); documented in code as a known simplification that is always safe (never returns a colliding address) though it can be over-conservative for genuinely mixed-width deployments."
  - "show.Load treats a not-yet-existing ShowState file as a fresh, empty State (SchemaVersion=1, Revision=0) rather than an error, so the very first `pool create`/`deployment create` against a brand-new --show path starts cleanly with no separate 'show init' command needed."
  - "Domain-level duplicate-name/single-active/address-bounds validation lives in one place (internal/show's validate(), reused by both Load and Save) rather than being re-checked in each CLI handler; CLI handlers just load, mutate, and save, so the GOLC_SHOW_STATE_INVALID wrapper always carries the specific GOLC_POOL_*/GOLC_DEPLOYMENT_* diagnostic substring callers assert on."
  - "show inspect's usage-parsing diagnostic reuses GOLC_DEPLOYMENT_USAGE (declared alongside 'deployment create'/'deployment activate' in the same file) since the plan's closed diagnostic-code list has no distinct show-usage code."

requirements-completed: [POOL-01, POOL-02]

coverage:
  - id: D1
    description: "A show author can define a logical pool of compatible fixtures independently of concrete fixture count, addresses, and deployment hardware (POOL-01)"
    requirement: "POOL-01"
    verification:
      - kind: unit
        ref: "internal/pool/model_test.go#TestPoolCountIndependent"
        status: pass
      - kind: integration
        ref: "internal/command/pooldeploy_test.go#TestPoolDeployRoutes"
        status: pass
    human_judgment: false
  - id: D2
    description: "A show author can create a named deployment mapping logical pools to concrete fixture instances/modes/universes/addresses, and mark exactly one deployment active (POOL-02, D-09)"
    requirement: "POOL-02"
    verification:
      - kind: unit
        ref: "internal/deployment/model_test.go#TestDeploymentActivateSingle"
        status: pass
      - kind: integration
        ref: "internal/command/pooldeploy_test.go#TestPoolDeployRoutes"
        status: pass
    human_judgment: false
  - id: D3
    description: "A pool with 0, 1, and the D-12 ceiling (~50) members is valid; pool identity is independent of member count"
    requirement: "POOL-01"
    verification:
      - kind: unit
        ref: "internal/pool/model_test.go#TestPoolCountIndependent"
        status: pass
    human_judgment: false
  - id: D4
    description: "Auto-assignable universe/address values use bounded integer arithmetic; a fixture's channel span never crosses a 512-channel universe boundary, including after many allocations force a rollover into a second universe"
    requirement: "POOL-01"
    verification:
      - kind: unit
        ref: "internal/deployment/model_test.go#TestNextFreeAddressBoundary"
        status: pass
    human_judgment: false
  - id: D5
    description: "Creating a pool or deployment with a name that already exists is rejected with a GOLC_POOL_DUPLICATE_NAME/GOLC_DEPLOYMENT_DUPLICATE_NAME diagnostic, never a silent duplicate"
    requirement: "POOL-02"
    verification:
      - kind: unit
        ref: "internal/pool/model_test.go#TestPoolIdentityStable"
        status: pass
      - kind: unit
        ref: "internal/deployment/model_test.go#TestNextFreeAddressBoundary"
        status: pass
      - kind: integration
        ref: "internal/command/pooldeploy_test.go#TestPoolDeployRoutes"
        status: pass
    human_judgment: false
  - id: D6
    description: "Pool, deployment, group, and instance identities are durable UUIDv7s minted only at creation time and survive rename/reorder -- identity is never derived from a display name"
    requirement: "POOL-01"
    verification:
      - kind: unit
        ref: "internal/pool/model_test.go#TestPoolIdentityStable"
        status: pass
    human_judgment: false
  - id: D7
    description: "ShowState.Save/Load are strict, atomic, and revision-bumping; Load rejects a tampered or duplicate-name document as GOLC_SHOW_STATE_INVALID"
    requirement: "POOL-02"
    verification:
      - kind: unit
        ref: "internal/show/state_test.go#TestShowStateRoundTrip"
        status: pass
    human_judgment: false
  - id: D8
    description: "show inspect prints a deterministic, allowlisted JSON envelope of pools and deployments with no absolute filesystem path"
    requirement: "POOL-02"
    verification:
      - kind: integration
        ref: "internal/command/pooldeploy_test.go#TestPoolDeployRoutes"
        status: pass
      - kind: manual_procedural
        ref: "go run ./cmd/golc-project pool create / deployment create / deployment activate / show inspect end-to-end against a real ShowState file"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-21
status: complete
---

# Phase 2 Plan 04: Pool/Deployment Domain Model and ShowState Substrate Summary

**Revisioned ShowState document (JSON, strictjson-backed) plus UUIDv7-identified Pool/Deployment/Instance types and the `pool create`/`deployment create`/`deployment activate`/`show inspect` CLI routes**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-21
- **Tasks:** 3
- **Files modified:** 11 (9 created, 2 modified: go.mod/go.sum)

## Accomplishments

- Established `internal/show.State` as the revisioned ShowState container (`SchemaVersion`, `Revision`, `Pools`, `Deployments`, `Groups`) that pool/deployment CLI routes load and atomically save; `Revision` is the monotonic counter 02-05's impact-plan freshness guard (D-16) will check.
- `internal/show.Load`/`Save` strictly decode via `internal/strictjson.DecodeStrict` and canonically encode via `strictjson.CanonicalEncode`, writing atomically (write-temp-then-rename); every malformed document, duplicate pool/deployment name, multiple-active-deployment, or out-of-range instance address is rejected as `GOLC_SHOW_STATE_INVALID` before anything is trusted or persisted (CONTEXT threat T-02-10).
- `internal/pool.Pool`/`PoolMember`/`Group`/`MemberRef` implement POOL-01: a Pool's UUIDv7 identity is minted once at creation and survives rename (`pool.Rename`); a Pool is equally valid with 0, 1, or the D-12 ceiling (~50) members; `RequiredCapabilities` validates against the exact nine-value `fixture.CapabilityType` enum 02-01 established.
- `internal/deployment.Deployment`/`Instance` implement POOL-02/D-09: `deployment.Activate` always leaves exactly one deployment active (guarded independently by `deployment.ValidateSingleActive`, `GOLC_DEPLOYMENT_MULTIPLE_ACTIVE`); `NextFreeAddress` allocates the next integer universe/address slot with bounded arithmetic that never crosses the 512-channel universe boundary, verified across 150 sequential allocations that force a rollover into a second universe.
- `golc pool create`, `golc deployment create`, `golc deployment activate`, and `golc show inspect` self-register through `internal/command` (D-04): verified end-to-end via `go run ./cmd/golc-project` against a real ShowState file, including duplicate-name rejection and the active-deployment swap.
- `github.com/google/uuid` added and promoted to a direct dependency at the plan-pinned `v1.6.0` (`go get` + `go mod tidy`); every Pool/PoolMember/Group/Deployment/Instance ID is a UUIDv7 minted only at creation.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing tests for ShowState + pool/deployment create/activate/inspect** - `82cdd11` (test)
2. **Task 2: Pool, Deployment, Group, Instance types + revisioned ShowState load/save** - `0513c76` (feat)
3. **Task 3: pool create / deployment create / deployment activate / show inspect routes** - `cb961ff` (feat)

**Plan metadata:** (pending -- final commit created after this Summary)

## Files Created/Modified

- `internal/show/state.go` - `State{SchemaVersion, Revision, Pools, Deployments, Groups}`, `Load`/`Save` (strict decode, whole-State validation, atomic write, revision bump)
- `internal/show/state_test.go` - `TestShowStateRoundTrip`, `TestShowStateLoadMissingFileReturnsFreshState`
- `internal/pool/model.go` - `Pool`, `PoolMember`, `Group`, `MemberRef`, `NewPool`/`Rename`/`NewPoolMember`, `Validate`, `ValidateUniqueNames`
- `internal/pool/model_test.go` - `TestPoolIdentityStable`, `TestPoolCountIndependent`
- `internal/deployment/model.go` - `Deployment`, `Instance`, `NewDeployment`, `ValidateUniqueNames`, `ValidateSingleActive`, `Activate`, `ValidateInstanceAddress`, `NextFreeAddress`
- `internal/deployment/model_test.go` - `TestDeploymentActivateSingle`, `TestNextFreeAddressBoundary`
- `internal/command/pool.go` - `pool` scope + `pool create` route
- `internal/command/deployment.go` - `deployment` scope + `deployment create`/`deployment activate` routes; `show` scope + `show inspect` route
- `internal/command/pooldeploy_test.go` - `TestPoolDeployRoutes`
- `go.mod` / `go.sum` - `github.com/google/uuid` promoted to direct at `v1.6.0`

## Decisions Made

- `NextFreeAddress` treats every existing `Instance` as occupying the same `channelCount` as the instance currently being allocated, since `Instance` does not yet carry its own channel-width field. This is always safe (never returns a colliding address) though it can skip an address that would genuinely have been free for a mixed-width deployment -- documented inline as a simplification for a future plan to refine once `Instance` gains a stored width.
- `show.Load` returns a fresh, empty `State` (never an error) when the `--show` file does not yet exist, so the first `pool create`/`deployment create` against a brand-new show starts cleanly without a separate `show init` command.
- Whole-State validation (unique pool/deployment names, single-active, address bounds) lives in one place inside `internal/show` and is reused by both `Load` and `Save`; CLI route handlers never re-check uniqueness themselves, so a duplicate name always surfaces as the specific `GOLC_POOL_DUPLICATE_NAME`/`GOLC_DEPLOYMENT_DUPLICATE_NAME` diagnostic wrapped inside `GOLC_SHOW_STATE_INVALID`.
- `show inspect`'s usage-parsing diagnostic reuses `GOLC_DEPLOYMENT_USAGE` (it is declared in `internal/command/deployment.go` alongside `deployment create`/`deployment activate`) since the plan's closed diagnostic-code list defines no distinct show-usage code.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added `GOLC_DEPLOYMENT_NOT_FOUND` for activating a nonexistent deployment name**
- **Found during:** Task 2 (`deployment.Activate`)
- **Issue:** The plan's closed diagnostic-code list did not enumerate a code for `deployment activate <name>` where `<name>` matches no existing deployment; without one, `Activate` would either silently no-op (leaving zero deployments active, which is a valid single-active state but a confusing UX for a typo'd name) or panic.
- **Fix:** `deployment.Activate` returns `GOLC_DEPLOYMENT_NOT_FOUND: no deployment named %q exists` and leaves the input slice's active state untouched (returns `nil, err`) when no deployment matches.
- **Files modified:** `internal/deployment/model.go`
- **Verification:** `internal/deployment/model_test.go#TestDeploymentActivateSingle` asserts activating a nonexistent name fails.
- **Committed in:** `0513c76` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Necessary correctness guard for the `deployment activate` route's error path; no scope creep -- the diagnostic follows the existing `GOLC_DEPLOYMENT_*` naming convention.

## Issues Encountered

None beyond the one auto-fixed deviation above. Full-repository `go test ./...` and `go vet ./...` both pass with no regressions in any pre-existing package (including `internal/trace/catalog`, whose prior unrelated `linear-map.json` drift failure — logged in `deferred-items.md` under 02-01 — was already resolved by an earlier commit on this branch, `18a55fe`, before this plan started).

`golc test --quick` (the plan's per-task commit gate) could not be run in this worktree: the pinned toolchain at `.tools/toolchains/go/1.26.5/windows-amd64/go/bin/go.exe` has not been bootstrapped here (`golc.ps1 bootstrap` was not run in this ephemeral worktree). This is a pre-existing environment-setup gap unrelated to this plan's files, not a regression this plan introduced. `go vet ./...` and `go test ./...` (run directly against the system Go 1.26.5 toolchain, which matches the project's pinned version) were used as the equivalent gate instead, and both pass cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/show.State`/`Load`/`Save` and `internal/pool`/`internal/deployment`'s types are the exact substrate 02-05 (impact-review engine) and 02-06 (fixture substitution) mutate: `State.Revision` is ready for 02-05's freshness guard (D-16), and `deployment.NextFreeAddress` is the auto-assignment primitive D-11 calls for.
- `pool create`/`deployment create`/`deployment activate`/`show inspect` are reachable end-to-end and self-registered per D-04; no blockers for later plans in this wave.
- `internal/pool.Group`/`MemberRef` (D-10) are defined and round-trip through `ShowState`, but no CLI route exists for them yet (out of this plan's scope per the plan's task list) -- a future plan should add a `group create`/`group` CLI surface when group membership becomes actionable.
- `deployment.Instance` does not yet carry its own channel-width field; `NextFreeAddress`'s same-width overlap simplification (documented above) should be revisited once per-instance width is modeled, likely in 02-05 or 02-06.

## Self-Check: PASSED

All created files verified present on disk; all three task commit hashes (`82cdd11`, `0513c76`, `cb961ff`) verified present in `git log --oneline --all`.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-21*
