---
phase: 02-modular-fixtures-and-deployments
plan: 01
subsystem: fixture-catalog
tags: [go-yaml, jsonschema, fixture, cli, strict-decode]

# Dependency graph
requires: []
provides:
  - "internal/fixture.FixtureDefinition/Capability/Mode/CapabilityType: the canonical, capability-based fixture model (D-08, GDTF-friendly)"
  - "internal/fixture.Decode(data []byte) (FixtureDefinition, error): the single strict-decode entrypoint every future fixture source normalizes through"
  - "golc fixture validate <file> CLI route (D-04, self-registered through internal/command)"
  - "schemas/fixture.schema.json generated + drift-checked through internal/contracts"
  - "go.yaml.in/yaml/v4 promoted to a direct dependency at v4.0.0-rc.6"
affects: [02-02, 02-03, 02-04, 02-05, 02-06]

# Tech tracking
tech-stack:
  added: ["go.yaml.in/yaml/v4 v4.0.0-rc.6 (direct dependency)"]
  patterns:
    - "Capability-based canonical fixture model (not OFL channel/mode-shaped) so a future OFL/GDTF import path normalizes into the same type"
    - "Strict decode via go.yaml.in/yaml/v4 WithKnownFields()+WithUniqueKeys() before any typed value is trusted, mirroring internal/strictjson's reject-before-populate discipline"
    - "Post-decode 'value, key, origin' diagnostic message shape copied from internal/projectconfig/decode.go's validateLiteral"
    - "Command self-registration (MustDeclareScope/MustDeclareRoute) and schema registration (MustRegisterSchema) reused verbatim from Phase 1's established idioms"

key-files:
  created:
    - internal/fixture/model.go
    - internal/fixture/decode.go
    - internal/fixture/decode_test.go
    - internal/command/fixture.go
    - internal/command/fixture_test.go
    - internal/contracts/fixture.go
    - internal/contracts/fixture_test.go
    - schemas/fixture.schema.json
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Pinned go.yaml.in/yaml/v4 at v4.0.0-rc.6 (the latest available rc, confirmed via docs/options.md in that exact tag) rather than leaving the previously-indirect v4.0.0-rc.2 pin, per RESEARCH.md Pitfall 3's rc-drift warning."
  - "Same-type capability range overlap validation only compares ranges within one CapabilityType (adjacent boundaries allowed, strict overlap rejected), supporting fixtures that declare multiple sub-ranges of one capability (for example a shutter channel's separate 'closed' and 'strobe' sub-ranges)."
  - "Fixed a jsonschema struct-tag description containing an unescaped comma that silently truncated the generated Range field description mid-sentence -- invopop/jsonschema tag values are comma-delimited key=value pairs, the same class of gotcha internal/contracts/model.go's header documents for backslash-escaped patterns, but for description text."

requirements-completed: [FIXT-01, FIXT-02, FIXT-04]

coverage:
  - id: D1
    description: "A valid RGB PAR YAML fixture definition decodes into a canonical FixtureDefinition carrying its declared capabilities"
    requirement: "FIXT-01"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestLoad"
        status: pass
    human_judgment: false
  - id: D2
    description: "golc fixture validate <file> exits 0 with a deterministic canonical summary for a valid definition and exits 2 with a GOLC_FIXTURE_* diagnostic for an invalid one"
    requirement: "FIXT-04"
    verification:
      - kind: integration
        ref: "internal/command/fixture_test.go#TestFixtureValidateRoute"
        status: pass
    human_judgment: false
  - id: D3
    description: "Duplicate mapping keys, unknown fields, out-of-range capability values, and unsupported capability types are each rejected with an actionable GOLC_FIXTURE_* diagnostic"
    requirement: "FIXT-02"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestDecodeRejects"
        status: pass
    human_judgment: false
  - id: D4
    description: "An empty YAML file, a fixture with zero capabilities, and a null capability list each produce GOLC_FIXTURE_EMPTY, never a panic"
    requirement: "FIXT-02"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestDecodeRejects (empty_file, zero_capabilities, null_capability_list rows)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Two capabilities whose normalized ranges are exactly adjacent both load; an overlapping range is rejected as a diagnostic, never silently merged"
    requirement: "FIXT-02"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestDecodeAdjacency"
        status: pass
    human_judgment: false
  - id: D6
    description: "Capability output order in the canonical summary is the declared source order and is stable across repeated decodes"
    requirement: "FIXT-02"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestLoad and #TestDecodeDeterministic"
        status: pass
    human_judgment: false
  - id: D7
    description: "Running golc fixture validate twice on the same file yields byte-identical output; validation performs no writes"
    requirement: "FIXT-04"
    verification:
      - kind: integration
        ref: "internal/command/fixture_test.go#TestFixtureValidateRoute (repeated-invocation byte-identical assertion)"
        status: pass
    human_judgment: false
  - id: D8
    description: "golc fixture validate performs no filesystem writes, so concurrent validation of the same file cannot corrupt shared state"
    requirement: "FIXT-04"
    verification: []
    human_judgment: true
    rationale: "No automated concurrent-invocation test was written this plan. By design runFixtureValidate only calls os.ReadFile and fixture.Decode (both read-only/pure) and never os.WriteFile, so there is no shared mutable state to corrupt -- but that design property is not mechanically proven by a concurrency test here."
  - id: D9
    description: "schemas/fixture.schema.json is generated from FixtureDefinition through internal/contracts and matches golc generate --check with no drift"
    requirement: "FIXT-01"
    verification:
      - kind: unit
        ref: "internal/contracts/fixture_test.go#TestFixtureSchemaRegisteredAndDriftFree"
        status: pass
      - kind: manual_procedural
        ref: "go run ./cmd/golc-project generate --check"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-07-22
status: complete
---

# Phase 2 Plan 01: Canonical Fixture Model, Strict Decode, and Validate Route Summary

**Capability-based `FixtureDefinition` model with strict go.yaml.in/yaml/v4 decode, GOLC_FIXTURE_* diagnostics, a generated `schemas/fixture.schema.json`, and the `golc fixture validate <file>` CLI route**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-22
- **Tasks:** 3
- **Files modified:** 10 (8 created, 2 modified: go.mod/go.sum)

## Accomplishments

- Established `internal/fixture.FixtureDefinition` as the canonical, capability-based fixture model (D-08: GDTF-friendly, not hard-wired to Open Fixture Library's channel/mode shape) with `SchemaVersion`, `Manufacturer`, `Model`, `Modes`, and order-preserving `Capabilities`.
- `internal/fixture.Decode` strictly decodes hand-authored YAML through `go.yaml.in/yaml/v4`'s `WithKnownFields()`+`WithUniqueKeys()` (rejecting duplicate mapping keys and unmodeled fields before any typed value is populated), then validates capability ranges (`[0,1]`), capability type semantics (the nine-value enum), same-type range overlap (adjacent ranges allowed, overlap rejected), and empty/zero/null capability documents -- every rejection a `GOLC_FIXTURE_*` diagnostic naming the offending value, key, and origin.
- `golc fixture validate <file>` self-registers through `internal/command` (D-04): exits 0 with a `strictjson.CanonicalEncode` summary for a valid definition, exits 2 with a `GOLC_FIXTURE_*` diagnostic for an invalid one. Verified end-to-end via `go run ./cmd/golc-project fixture validate`.
- `schemas/fixture.schema.json` is generated from `FixtureDefinition` through `internal/contracts.MustRegisterSchema`, joining the existing `GenerateAll`/`CheckDrift` traversal; `golc generate --check` reports no drift.
- `go.yaml.in/yaml/v4` promoted from an indirect dependency pinned at `v4.0.0-rc.2` to a direct dependency at the verified `v4.0.0-rc.6` (confirmed the `WithKnownFields`/`WithUniqueKeys` Options API is present in that exact tag's `docs/options.md` before writing decode code, per RESEARCH.md Pitfall 3).
- No fixture-editor or pool-management UI was built (D-01): this plan is entirely `internal/fixture` domain code plus one CLI route, consistent with Phase 2's headless scope; the fixture-editor/pool-management UI remains Phase 6 work.
- No capability flattening: `CapabilityType` is preserved verbatim on every decoded `Capability` (intensity/color/pan/tilt/zoom/focus/gobo/shutter/strobe are nine distinct enum values, never collapsed into a generic "intensity" bucket), satisfying the threat-model prohibition that a strobe/UV/laser capability's distinct type must stay visible to later output phases.

## Task Commits

Each task was committed atomically:

1. **Task 1: Failing end-to-end + unit tests for fixture validate (Wave 0 scaffold)** - `6f65e01` (test)
2. **Task 2: Canonical capability model + strict YAML decode + fixture validate route (thin happy path)** - `559f596` (feat)
3. **Task 3: Harden validation diagnostics + generate versioned fixture schema** - `0f51630` (feat)

**Plan metadata:** (pending -- final commit created after this Summary)

_Note: Task 1 is the RED scaffold; Task 2 is GREEN for the happy path and duplicate-key rejection; Task 3 is GREEN for every remaining FIXT-02 rejection row plus the generated schema. This plan's tasks were not declared `tdd="true"` in frontmatter but follow the same red/green shape by construction._

## Files Created/Modified

- `internal/fixture/model.go` - `CapabilityType` enum (9 values), `Capability`, `Mode`, `FixtureDefinition` structs with yaml/json/jsonschema tags
- `internal/fixture/decode.go` - `Decode(data []byte) (FixtureDefinition, error)`: strict YAML decode + full post-decode validation (range, type, overlap, empty)
- `internal/fixture/decode_test.go` - `TestLoad`, `TestDecodeRejects`, `TestDecodeAdjacency`, `TestDecodeDeterministic`
- `internal/command/fixture.go` - `fixture` scope + `fixture validate` route (D-04)
- `internal/command/fixture_test.go` - `TestFixtureValidateRoute`
- `internal/contracts/fixture.go` - registers the `fixture` schema descriptor
- `internal/contracts/fixture_test.go` - `TestFixtureSchemaRegisteredAndDriftFree`
- `schemas/fixture.schema.json` - generated Draft 2020-12 schema for `FixtureDefinition`
- `go.mod` / `go.sum` - `go.yaml.in/yaml/v4` promoted to direct at `v4.0.0-rc.6`

## Decisions Made

- Pinned `go.yaml.in/yaml/v4` at the latest available `v4.0.0-rc.6` (verified via `go list -m -versions` and a direct read of `docs/options.md` in that exact tag) rather than the previously-pinned `rc.2`, since the plan's own read-first step required confirming the Options API's presence before writing decode code.
- Range-overlap validation is scoped per `CapabilityType`: two capabilities of *different* types may share or overlap ranges freely (they are independent DMX-style parameters); only two capabilities of the *same* type are checked for overlap, with exactly-touching boundaries explicitly allowed.
- Diagnostic message shape ("value, key, origin") was copied verbatim from `internal/projectconfig/decode.go`'s `validateLiteral`, keeping the repo-wide `GOLC_{DOMAIN}_{CONDITION}` diagnostic convention consistent across concerns.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed a jsonschema struct-tag description that silently truncated mid-sentence**
- **Found during:** Task 3 (generating `schemas/fixture.schema.json` and inspecting the output)
- **Issue:** `Capability.Range`'s `jsonschema:"...description=Normalized [min,max] value range in [0,1] (not raw DMX)."` tag contained unescaped commas; `invopop/jsonschema`'s struct-tag parser splits tag values on commas as option separators, so the generated schema's description silently truncated to `"Normalized [min"`. This is the same class of struct-tag-escaping gotcha `internal/contracts/model.go`'s header already documents for backslash-escaped regex patterns, but for description text.
- **Fix:** Rewrote the description to avoid commas: `"Normalized [min max] value range within the 0 to 1 interval; never raw DMX."`
- **Files modified:** `internal/fixture/model.go`
- **Verification:** Regenerated `schemas/fixture.schema.json` and confirmed the full description text is present; `go test ./internal/contracts/...` passes (no drift).
- **Committed in:** `0f51630` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary for the generated schema's documentation quality (FIXT-01: "documented, versioned YAML schema"); no scope creep.

## Issues Encountered

- `go test ./...` (full repository suite, run once during Task 3 to confirm no regressions) surfaced one pre-existing, unrelated failure: `internal/trace/catalog` `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails with `GOLC_MIGRATE_DRIFT: .planning/linear-map.json does not match the canonical schema-2 migration output`. `git status --short .planning/` confirms this plan made no changes to `.planning/linear-map.json`, and the failure is in Phase 1's Linear-sync catalog code, not any file this plan touches. Logged (not fixed, per the executor's scope-boundary rule) to `.planning/phases/02-modular-fixtures-and-deployments/deferred-items.md`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/fixture.FixtureDefinition`/`Decode` are the exact substrate 02-03 (OFL import) and the pool/deployment plans (02-04+) normalize into and reference, per RESEARCH.md's architecture map.
- `golc fixture validate` is reachable and end-to-end verified; no blockers for later plans in this wave.
- One pre-existing, unrelated full-suite failure (`internal/trace/catalog` linear-map drift) is logged in `deferred-items.md` and should be picked up by a dedicated fix outside this plan's scope.

## Self-Check: PASSED

All created files verified present on disk; all four task/summary commit hashes (`6f65e01`, `559f596`, `0f51630`, `86a152b`) verified present in `git log --oneline --all`.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-22*
