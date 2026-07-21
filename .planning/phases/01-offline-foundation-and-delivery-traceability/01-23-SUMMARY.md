---
phase: 01-offline-foundation-and-delivery-traceability
plan: 23
subsystem: linear-traceability
tags: [go, linear, reconcile, transport, fake, offline, command-routing]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 10
    provides: internal/trace/reconcile canonical model (Intent/RemoteObservation/SyncBaseline/Operation/Conflict/Plan), SHA-256 plan digests, SortOperations, BuildPlan's D-13 three-way conflict rule, and RenderMarker/ParseMarker/ValidateMarkerIdentity
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 21
    provides: internal/command/linear.go owning the "linear" routing scope and the self-registration route/scope contract pattern
provides:
  - internal/trace/transport package (Transport interface, Snapshot/RemoteRecord/SnapshotStatus, Mutation, and a credential-free Fake with LoadFakeSnapshot)
  - internal/trace/reconcile/diff.go (ValidateCompleteSnapshot, ThreeWayField, marker-based zero/one/multiple discovery, BuildCompletePreview, BuildArchivePreview/BuildUnlinkPreview)
  - Self-registered offline routes "linear preview", "linear archive", "linear unlink" under the existing "linear" scope
  - Four self-contained fixtures (remote-complete, remote-conflict, remote-ambiguous, explicit-archive) plus the linear-reconcile quick-test scope (TestScopeLinearReconcile)
affects: [01-24, linear-traceability]

tech-stack:
  added: []
  patterns:
    - "BuildCompletePreview never reimplements the D-13 three-way conflict/D-17 ordering/hashing contract: it validates the snapshot, discovers each intent's current RemoteObservation (via already-linked UUID or exact D-14 marker match), and delegates straight to the existing (unmodified) reconcile.BuildPlan"
    - "Self-contained fixture-as-scenario JSON: remote-*.json fixtures bundle intents + mappings + baselines + the transport-neutral snapshot in one file, so each fixture is a complete, independently reviewable reconciliation scenario that needs no live catalog or transport"
    - "ValidateCompleteSnapshot folds two distinct 'ambiguous' failure modes into one gate: a transport-reported SnapshotAmbiguous status, and a status-complete snapshot where two different remote records both carry the same D-14 identity footer (T-01-28 duplication/spoofing)"

key-files:
  created:
    - internal/trace/transport/contract.go
    - internal/trace/transport/fake.go
    - internal/trace/reconcile/diff.go
    - tests/fixtures/linear/remote-complete.json
    - tests/fixtures/linear/remote-conflict.json
    - tests/fixtures/linear/remote-ambiguous.json
    - tests/fixtures/linear/explicit-archive.json
  modified:
    - internal/trace/reconcile/reconcile_test.go
    - internal/command/linear.go

key-decisions:
  - "diff.go deliberately does not touch canonical.go/model.go (not in this plan's file inventory): BuildCompletePreview builds a RemoteScope from discovered observations and calls the existing, already-tested BuildPlan rather than duplicating its three-way conflict logic. ThreeWayField is still exported as a standalone, independently testable extraction of that same rule for direct unit coverage."
  - "Identity for an already-linked entity comes only from its immutable Linear UUID (matched via mapping.LinearUUID against transport.RemoteRecord.LinearUUID); identity for an unlinked entity comes only from an exact D-14 marker match in a record's Description. Titles are read into RemoteObservation.Fields for three-way comparison but never consulted to establish or disambiguate identity."
  - "ValidateCompleteSnapshot blocks on snapshot Status (incomplete/partial/cursor_anomaly/ambiguous/rate_limited) before any discovery runs, and additionally rejects a status-complete snapshot if two records resolve to the same local ID via their identity footers -- covering both the D-21 transport-level diagnostics and the D-14/T-01-28 marker-duplication threat under a single gate."
  - "linear archive / linear unlink read the target's remote mapping from catalog.MigrateV1ToV2's already-validated schema-2 output (not a hand-rolled map reader) and fail closed with GOLC_RECONCILE_ARCHIVE_UNMAPPED when the mapping has no linear_uuid -- matching D-15's 'explicit reviewed archive/unlink only, never inferred from local absence.'"
  - "linear preview's --snapshot argument loads a plain transport.Snapshot fixture via transport.LoadFakeSnapshot, while intents/mappings for that route always come from the live repository catalog (catalog.MigrateV1ToV2), never from the richer self-contained test fixtures -- keeping the CLI route always accurate against real repository state instead of synthetic test data."
  - "LINR-03/LINR-04 are NOT marked complete in this plan (same precedent as 01-08's and 01-21's summaries): LINR-03 requires 'preview and run' but this plan only builds preview -- apply/run is 01-24's explicit scope per its 'expose apply' plan title. LINR-04 requires real GraphQL partial-error/rate-limit normalization, which stays owned by 01-26/01-27 against a live transport; this plan only defines the generic SnapshotStatus diagnostic categories a fake or real transport reports through."

requirements-completed: []

coverage:
  - id: D1
    description: "ValidateCompleteSnapshot blocks incomplete, partial, cursor-anomalous, transport-ambiguous, and rate-limited snapshot inputs, and additionally blocks a complete snapshot whose two records share one identity footer"
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/ValidateCompleteSnapshot_blocks_every_non-complete_status_with_a_stable_diagnostic"
        status: pass
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/ValidateCompleteSnapshot_and_BuildCompletePreview_block_a_complete_snapshot_with_a_duplicated_identity_footer"
        status: pass
    human_judgment: false
  - id: D2
    description: "ThreeWayField emits a blocking conflict with base/repository/Linear values only when all three legs are pairwise distinct, matching D-13 exactly"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/ThreeWayField_blocks_only_when_base,_repository,_and_Linear_are_pairwise_distinct"
        status: pass
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/BuildCompletePreview_blocks_a_three-way_disagreement_discovered_through_an_already-linked_UUID"
        status: pass
    human_judgment: false
  - id: D3
    description: "Marker discovery follows the zero/one/multiple rule: an unmatched intent creates, an exact single marker match adopts/updates, and titles never establish identity"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/BuildCompletePreview_adopts_a_marker-matched_record_and_creates_an_unmatched_intent"
        status: pass
    human_judgment: false
  - id: D4
    description: "linear preview/archive/unlink self-register exact routes under the existing linear scope and are reachable through a credential-free Fake transport without live Linear access"
    requirement: "LINR-03"
    verification:
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/the_complete_preview_is_reachable_end_to_end_through_a_credential-free_Fake_transport"
        status: pass
      - kind: unit
        ref: "internal/command (go test ./internal/command/...) -- registry build succeeds with linear preview/archive/unlink routes declared"
        status: pass
    human_judgment: false
  - id: D5
    description: "Local absence never creates a delete; only an explicit archive/unlink request against an already-linked entity produces a reviewable removal preview, and an unmapped entity is rejected"
    requirement: "LINR-04"
    verification:
      - kind: unit
        ref: "internal/trace/reconcile/reconcile_test.go#TestScopeLinearReconcile/BuildArchivePreview_and_BuildUnlinkPreview_build_an_explicit_D-15_removal_preview,_and_reject_an_unmapped_entity"
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 23: Preview Complete Fake Snapshots with Conflicts and Ambiguity Blocked Summary

**New internal/trace/transport package (Transport contract, Snapshot/RemoteRecord, credential-free Fake) plus internal/trace/reconcile/diff.go (ValidateCompleteSnapshot, ThreeWayField, D-14 marker discovery, BuildCompletePreview delegating to the existing BuildPlan, and explicit D-15 archive/unlink review builders), wired into three new self-registered "linear preview/archive/unlink" routes and proven against four self-contained fixtures**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-07-21T00:22:00Z (approx.)
- **Completed:** 2026-07-21T01:19:56Z
- **Tasks:** 1 (TDD)
- **Files modified:** 9 (7 created, 2 modified)

## Accomplishments

- `internal/trace/transport/contract.go` defines the transport-neutral `Transport` interface (`CaptureSnapshot`/`Apply`), the exhaustive `SnapshotStatus` enum (`complete`/`incomplete`/`partial`/`cursor_anomaly`/`ambiguous`/`rate_limited`, CONTEXT D-21), `RemoteRecord` (a raw remote observation whose `Description` is the only place a D-14 identity footer may appear — `Title` is diagnostic only), `Snapshot`, and `Mutation`/`MutationKind` (`archive`/`unlink`, CONTEXT D-15).
- `internal/trace/transport/fake.go` implements a credential-free, in-memory `Fake` (`NewFake`, `LoadFakeSnapshot` via `strictjson.DecodeStrict`, `CaptureSnapshot`, `Apply`, `Applied`) satisfying the `Transport` interface with zero network/SDK/credential access.
- `internal/trace/reconcile/diff.go` adds `ValidateCompleteSnapshot` (blocks every non-complete status plus duplicate-marker ambiguity across records), `ThreeWayField` (the exact D-13 three-way rule as a standalone testable function), `discoverObservations` (UUID-linked or exact D-14 marker zero/one/multiple discovery), `BuildCompletePreview` (validates, discovers, then delegates to the unmodified `BuildPlan` for conflict/ordering/hashing), and `ArchivePreview`/`BuildArchivePreview`/`BuildUnlinkPreview` for explicit D-15 removal review.
- `internal/command/linear.go` self-registers three new exact routes under the existing `linear` scope: `linear preview --snapshot <path> --out <path>` (loads a fake transport snapshot, builds intent/mappings from the live repository catalog via `catalog.MigrateV1ToV2`, writes the canonical preview JSON), `linear archive --local-id <id> --preview-out <path>`, and `linear unlink --local-id <id> --preview-out <path>` (both resolve the target's already-recorded mapping and fail closed for anything unmapped).
- `internal/trace/reconcile/reconcile_test.go` gains the `linear-reconcile` quick-test scope and `TestScopeLinearReconcile` with nine subtests: every non-complete `SnapshotStatus` blocked, a clean complete snapshot accepted, a duplicated-marker complete snapshot blocked by both `ValidateCompleteSnapshot` and `BuildCompletePreview`, marker adoption vs. create, an already-linked three-way conflict block, direct `ThreeWayField` pairwise-distinct coverage, archive/unlink preview build plus unmapped rejection, and an end-to-end round trip through a `transport.Fake`.
- Four new fixtures under `tests/fixtures/linear/`: `remote-complete.json` (one marker-adopted update, one zero-match create), `remote-conflict.json` (already-UUID-linked entity whose title changed on both sides away from a recorded baseline), `remote-ambiguous.json` (two records sharing one identity footer — T-01-28), and `explicit-archive.json` (an already-linked mapping for the archive/unlink builders).

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: complete-snapshot reconciliation preview contract** - `118f34b` (test) — `internal/trace/reconcile/reconcile_test.go` and the four fixtures added; `internal/trace/transport/contract.go`, `fake.go`, `internal/trace/reconcile/diff.go` were staged out of the working tree and `internal/command/linear.go` was temporarily reverted to its prior committed content for this run. `go test ./internal/trace/reconcile/...` failed with `no required module provides package github.com/lnorton89/golc/internal/trace/transport`, confirming the test could not build without the new implementation. All staged-out/reverted files were restored immediately after.
2. **GREEN - Task 1: transport package, diff.go, and self-registered preview/archive/unlink routes** - `b9aacd8` (feat) — full suite passes: `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` all succeed, including `TestScopeLinearReconcile` and the pre-existing `TestScopeLinearPreviewContract` coexisting in the same file.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/trace/transport/contract.go` - `Transport` interface, `SnapshotStatus`, `RemoteRecord`, `Snapshot`, `MutationKind`/`Mutation`.
- `internal/trace/transport/fake.go` - `Fake` (in-memory `Transport`), `NewFake`, `LoadFakeSnapshot`, `Applied`.
- `internal/trace/reconcile/diff.go` - `ValidateCompleteSnapshot`, `ThreeWayField`, `discoverObservations`, `BuildCompletePreview`, `ArchivePreview`, `BuildArchivePreview`, `BuildUnlinkPreview`.
- `internal/trace/reconcile/reconcile_test.go` - Added the `linear-reconcile` scope, `TestScopeLinearReconcile`, `snapshotFixture`/`archiveFixture` loaders.
- `internal/command/linear.go` - Added `linear preview`/`linear archive`/`linear unlink` routes, arg parsers, `resolveWritablePath`, `intentsFromMigratedMap`, `writeArchivePreview`.
- `tests/fixtures/linear/remote-complete.json` - Clean complete snapshot: one marker-adopted update, one zero-match create.
- `tests/fixtures/linear/remote-conflict.json` - Already-linked entity with a D-13 three-way title conflict.
- `tests/fixtures/linear/remote-ambiguous.json` - Two records sharing one D-14 identity footer.
- `tests/fixtures/linear/explicit-archive.json` - Already-linked mapping fixture for the archive/unlink builders.

## Decisions Made

- **`BuildCompletePreview` delegates to the unmodified `BuildPlan` instead of reimplementing D-13/D-17:** it only validates the snapshot and discovers each intent's `RemoteObservation`, then hands off to the already-tested `canonical.go` logic — keeping `canonical.go`/`model.go` outside this plan's file inventory untouched while still reusing their exact, byte-stable conflict/ordering/hashing behavior.
- **Two identity sources, never titles:** an already-linked entity's identity comes only from its immutable Linear UUID matched against `RemoteRecord.LinearUUID`; an unlinked entity's identity comes only from an exact D-14 marker in `RemoteRecord.Description`, validated via the existing `ValidateMarkerIdentity` (kind/parent cross-check). Zero matches creates, exactly one adopts, more than one blocks.
- **`ValidateCompleteSnapshot` unifies two "ambiguous" failure modes:** a transport-reported `SnapshotAmbiguous` status and a status-complete snapshot where two distinct records resolve to the same local ID via their identity footers (duplication/spoofing, T-01-28) both fail the same gate before any discovery or planning runs.
- **Fixture-as-scenario shape:** `remote-*.json` fixtures bundle `intents`/`mappings`/`baselines`/`snapshot` together so each is a fully self-contained, independently reviewable reconciliation scenario, matching the existing `previewFixture()`/`conflictFixture()` Go-helper pattern from Plan 01-10 but as committed JSON.
- **`linear archive`/`linear unlink` read mappings from `catalog.MigrateV1ToV2`'s validated schema-2 output**, never a hand-rolled reader, and fail closed with `GOLC_RECONCILE_ARCHIVE_UNMAPPED` for any mapping without a `linear_uuid` — D-15 compliance without inventing a second map-reading code path.
- **`linear preview`'s `--snapshot` flag always loads a plain `transport.Snapshot` fixture, and intents/mappings for that route always come from the live repository catalog**, not the richer self-contained test fixtures — keeping the CLI route accurate against real repository state while the four required fixtures remain fully self-contained Go-test-only scenarios.

## Deviations from Plan

None — plan executed exactly as written. All nine frontmatter-declared files were created/modified exactly as listed; no other files were touched, and `internal/trace/reconcile/canonical.go`/`model.go`/`marker.go` (owned by Plan 01-10, not in this plan's inventory) were read but never edited.

## Issues Encountered

- This worktree has no bootstrapped `.tools/` pinned toolchain (bootstrap was not run here), so the plan's literal `<verify>` command (`powershell -NoProfile -File .\golc.ps1 test --quick --scope linear-reconcile`) could not be invoked through the shim without a multi-minute bootstrap first. Verified instead with the host Go toolchain, which is `go1.26.5 windows/amd64` — an exact match for the pinned `config/toolchain.toml` version — running `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the exact scope-dispatch equivalent (`go test -list ^TestScopeLinearReconcile$ ./...` confirming the marker resolves in `internal/trace/reconcile` the same way `test --quick --scope linear-reconcile` would find it), all passing. This mirrors the same substitution documented in the 01-10 and 01-21 summaries.

## Known Stubs

- None blocking this plan's goal. `linear preview`/`linear archive`/`linear unlink` are fully wired to real logic (`catalog.MigrateV1ToV2`, `reconcile.BuildCompletePreview`/`BuildArchivePreview`/`BuildUnlinkPreview`, `transport.LoadFakeSnapshot`) — there is no mocked or placeholder data path. The `--snapshot` argument to `linear preview` intentionally still requires a fake-transport fixture file (no live Linear transport exists yet); that is this plan's explicit scope boundary, not a stub, and a real adapter can later implement the same `transport.Transport` interface without changing `reconcile`'s call sites.

## Threat Flags

None — no new surface beyond the plan's threat model. T-01-28 (marker discovery spoofing) is mitigated by `discoverObservations`' zero/one/multiple rule plus `ValidateCompleteSnapshot`'s duplicate-identity-footer rejection, both driven by the existing `ParseMarker`/`ValidateMarkerIdentity` exact-match logic. T-01-29 (field conflict tampering) is mitigated by `ThreeWayField`/`BuildCompletePreview` delegating to the unmodified, already-proven three-way `BuildPlan` conflict check. T-01-30 (removal tampering) is mitigated by `BuildArchivePreview`/`BuildUnlinkPreview` requiring an already-linked `linear_uuid` and only ever running from an explicit `linear archive`/`linear unlink` invocation — `linear preview` never emits a delete for an intent with no discovered remote record.

## User Setup Required

None - everything is repository-local; no credentials, npm, network, or Linear access involved. `linear preview` reads only a local fake-snapshot fixture file and the committed repository catalog/map.

## Next Phase Readiness

- Plan 01-24 (strict plan/report contracts and apply CLI, per `affects`) can consume `reconcile.BuildCompletePreview`'s `Plan` output, `transport.Transport`/`transport.Mutation` for a future real adapter's `Apply` path, and `reconcile.ArchivePreview`/`BuildArchivePreview`/`BuildUnlinkPreview` for turning a reviewed archive/unlink preview into an actual apply step.
- The `linear-reconcile` quick-test scope (`golc.ps1 test --quick --scope linear-reconcile`, once `.tools/` is bootstrapped) now joins `linear-preview-contract`/`linear-catalog`/`linear-map`/`config-local` as a standing quick gate.
- `linear preview`/`linear archive`/`linear unlink` give contributors a fully offline way to exercise the complete D-13/D-14/D-15/D-17/D-21 reconciliation policy against fake snapshots before any live Linear transport exists.

## Self-Check: PASSED

- All seven created files exist on disk: `internal/trace/transport/contract.go`, `internal/trace/transport/fake.go`, `internal/trace/reconcile/diff.go`, and the four `tests/fixtures/linear/*.json` fixtures. Both modified files (`internal/trace/reconcile/reconcile_test.go`, `internal/command/linear.go`) carry the new scope/routes.
- Commits `118f34b` (test) and `b9aacd8` (feat) exist in git history on branch `worktree-agent-a258ed8f229bcdfd1`.
- `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `go test -count=1 -race ./...` all pass from the repository root with the host Go 1.26.5 toolchain (matches the pinned version exactly); `go test -list '^TestScopeLinearReconcile$' ./...` confirms the marker resolves in `internal/trace/reconcile`.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
