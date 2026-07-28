---
phase: 02-modular-fixtures-and-deployments
plan: 02
subsystem: fixture-catalog
tags: [content-addressing, sha256, provenance, cli, fixture]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments (02-01)
    provides: "internal/fixture.FixtureDefinition/Decode and the fixture command scope + fixture validate route"
provides:
  - "internal/fixture.Identity/Pin(def): content-addressed fixture pin (SchemaVersion, ContentHash, Revision, StableKey) — sha256 hex over strictjson.CanonicalEncode(def)"
  - "internal/fixture.Provenance/NewProvenance/LossyImportWarning: reviewable trust record (Source, SchemaVersion, ContentHash, Revision, ValidationResult, Warnings)"
  - "golc fixture inspect <file> CLI route: allowlisted, path-free JSON envelope of identity + provenance"
affects: [02-03, 02-04, 02-05, 02-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Content-addressed identity reuses internal/trace/apply/guard.go's recomputePlanID binding shape verbatim (strictjson.CanonicalEncode -> crypto/sha256, lowercase hex) — no second canonical encoder or hash scheme introduced"
    - "Allowlisted JSON projection for CLI output (fixtureInspectView/fixtureWarningView), mirroring internal/command/linear.go's catalogView discipline: only stable identity/provenance/validation/warning fields are emitted, never a raw absolute path"
    - "Provenance.Source derivation: repository-relative + slash-normalized when the resolved file lives under Request.Root, else a basename-only 'external:<name>' label — never the resolved absolute path"

key-files:
  created:
    - internal/fixture/identity.go
    - internal/fixture/identity_test.go
    - internal/fixture/provenance.go
    - internal/fixture/provenance_test.go
  modified:
    - internal/command/fixture.go
    - internal/command/fixture_test.go

key-decisions:
  - "Revision derives from the first 12 hex characters of ContentHash (revisionPrefixLength) rather than a separate counter — always non-empty for any valid fixture, changes exactly when ContentHash changes, and needs no additional state to track across pins."
  - "StableKey is Manufacturer+\"/\"+Model — the human-stable identity independent of any single revision's content hash, per the plan's Identity struct shape."
  - "parseFixtureValidateArgs was renamed parseFixtureFileArg since both fixture validate and fixture inspect share the identical single-positional-file-path grammar; no behavior change, just naming that now matches its shared use."

requirements-completed: [FIXT-05, FIXT-06]

coverage:
  - id: D1
    description: "A fixture is pinned by stable identity, schema version, content revision, and content hash so a later library update cannot silently change an existing show"
    requirement: "FIXT-05"
    verification:
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityHashStable"
        status: pass
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityComplete"
        status: pass
    human_judgment: false
  - id: D2
    description: "golc fixture inspect <file> surfaces source, provenance, schema version, content hash, revision, validation result, and any unsupported/lossy warnings before the fixture is used"
    requirement: "FIXT-06"
    verification:
      - kind: integration
        ref: "internal/command/fixture_test.go#TestFixtureInspectRoute"
        status: pass
      - kind: manual_procedural
        ref: "go run ./cmd/golc-project fixture inspect <file>"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two fixtures whose content differs by one byte produce different content hashes; identical content produces the identical hash"
    requirement: "FIXT-05"
    verification:
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityHashStable"
        status: pass
    human_judgment: false
  - id: D4
    description: "Pinning a fixture with no optional metadata still yields a complete identity (schema_version, revision, non-empty hash) — never an empty or nil hash"
    requirement: "FIXT-05"
    verification:
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityComplete"
        status: pass
    human_judgment: false
  - id: D5
    description: "Canonical encoding sorts keys so two semantically-equal fixtures whose YAML key order differs pin to the same content hash"
    requirement: "FIXT-05"
    verification:
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityHashKeyOrderStable"
        status: pass
    human_judgment: false
  - id: D6
    description: "Provenance/warning inspection surfaces lossy import details as a distinct list before use, independent of whether the source was hand-authored or imported"
    requirement: "FIXT-06"
    verification:
      - kind: unit
        ref: "internal/fixture/provenance_test.go#TestProvenance"
        status: pass
    human_judgment: false
  - id: D7
    description: "Re-reading and re-pinning the same fixture recomputes the identical hash (content-addressed identity is idempotent under re-read)"
    requirement: "FIXT-05"
    verification:
      - kind: unit
        ref: "internal/fixture/identity_test.go#TestIdentityHashStable"
        status: pass
      - kind: manual_procedural
        ref: "go run ./cmd/golc-project fixture inspect <file> (run twice, byte-identical output)"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-22
status: complete
---

# Phase 2 Plan 02: Content-Addressed Fixture Identity, Provenance, and Fixture Inspect Summary

**Content-addressed `fixture.Identity`/`Pin` (sha256 over `strictjson.CanonicalEncode`), `fixture.Provenance`, and the `golc fixture inspect <file>` CLI route surfacing both through an allowlisted, path-free JSON envelope**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-22
- **Tasks:** 3
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `internal/fixture.Identity`/`Pin(def)` gives every canonical `FixtureDefinition` a content-addressed pin (FIXT-05): `ContentHash` is the lowercase-hex sha256 digest of `strictjson.CanonicalEncode(def)`, reusing `internal/trace/apply/guard.go`'s `recomputePlanID` binding shape exactly (no bespoke hash scheme, per RESEARCH Don't-Hand-Roll). `Revision` derives from the first 12 hex characters of `ContentHash` (always non-empty), and `StableKey` is `Manufacturer/Model`.
- Verified: re-reading and re-pinning identical bytes reproduces the identical hash; a one-byte content change changes it; two fixtures decoded from differently-ordered YAML pin to the identical hash (canonical struct-field-order encoding is key-order-invariant by construction); a minimal fixture with no optional metadata still yields a complete, non-empty identity.
- `internal/fixture.Provenance`/`NewProvenance`/`LossyImportWarning` gives every fixture a reviewable trust record (FIXT-06): `Source`, `SchemaVersion`, `ContentHash`, `Revision`, `ValidationResult` ("valid" for a successfully-decoded fixture), and a `Warnings` list that starts empty for a hand-authored fixture and will be populated by 02-03's OFL import.
- `golc fixture inspect <file>` self-registers under the existing `fixture` scope beside `fixture validate` (D-04): it decodes, pins, and builds provenance for the given file, then emits a deterministic, allowlisted JSON envelope (`fixtureInspectView`/`fixtureWarningView`, mirroring `linear.go`'s `catalogView` allowlisted-projection discipline) containing only stable identity/provenance/validation/warning fields — never a raw absolute path or host detail (T-01-23). `fixtureInspectSource` derives `Source` as a repository-relative, slash-normalized path when the file lives under `Request.Root`, or a basename-only `external:<name>` label otherwise; end-to-end verified via `go run ./cmd/golc-project fixture inspect` with a file outside the repo root, confirming no absolute path leaks and repeated invocation is byte-identical.
- Decode/pin failure returns `ExitCode 2` with the underlying `GOLC_FIXTURE_*` diagnostic on Stderr (mirrors `fixture validate`'s error contract); envelope-encode failure is wrapped as `GOLC_FIXTURE_INSPECT_ENCODE_FAILED`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing tests for content-hash pinning + provenance inspection** - `03d5e22` (test)
2. **Task 2: Content-addressed identity + provenance record** - `510a6cd` (feat)
3. **Task 3: fixture inspect route surfacing identity + provenance** - `62f75eb` (feat)

**Plan metadata:** (pending — final commit created after this Summary)

## Files Created/Modified

- `internal/fixture/identity.go` - `Identity{SchemaVersion, ContentHash, Revision, StableKey}` + `Pin(def)`: sha256 hex over `strictjson.CanonicalEncode(def)`
- `internal/fixture/identity_test.go` - `TestIdentityHashStable`, `TestIdentityHashKeyOrderStable`, `TestIdentityComplete`
- `internal/fixture/provenance.go` - `Provenance`, `LossyImportWarning`, `NewProvenance(def, identity, source)`
- `internal/fixture/provenance_test.go` - `TestProvenance`
- `internal/command/fixture.go` - `fixture inspect` route: `runFixtureInspect`, `fixtureInspectView`/`fixtureWarningView` allowlisted projection, `fixtureInspectSource`; `parseFixtureValidateArgs` renamed `parseFixtureFileArg` (shared by both routes)
- `internal/command/fixture_test.go` - `TestFixtureInspectRoute`

## Decisions Made

- Revision = `ContentHash[:12]` rather than a separate schema-version-scoped counter: always non-empty, deterministic, and requires no additional state.
- `StableKey` = `Manufacturer + "/" + Model`, matching the plan's Identity struct shape verbatim.
- `fixtureInspectSource` falls back to `external:<basename>` (never the resolved absolute path, never the full relative traversal) for any file outside `Request.Root`, keeping the information-disclosure discipline intact even for files reached via `..` traversal or an absolute argument outside the repo.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. Full repository test suite (`go test ./...`) passes with no regressions and no pre-existing failures observed at this plan's completion.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/fixture.Identity`/`Pin` and `Provenance`/`LossyImportWarning` are the exact substrate 02-03's OFL import will populate `Warnings` through, and later pool/deployment plans (02-04+) can reference for pinned fixture identity.
- `golc fixture inspect` is reachable and end-to-end verified; no blockers for later plans in this wave.
- No stubs: `fixture inspect` is fully wired to `fixture.Decode`/`Pin`/`NewProvenance` with no placeholder or mock data path.

## Self-Check: PASSED

All created files verified present on disk; all three task commit hashes (`03d5e22`, `510a6cd`, `62f75eb`) verified present in `git log --oneline --all`.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-22*
