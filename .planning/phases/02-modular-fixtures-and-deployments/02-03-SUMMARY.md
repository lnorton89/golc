---
phase: 02-modular-fixtures-and-deployments
plan: 03
subsystem: fixture-import
tags: [ofl, open-fixture-library, fixture-model, ssrf, net-http, yaml, json, cli]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments (02-01/02-02)
    provides: internal/fixture's canonical FixtureDefinition/Capability model, strict YAML Decode, content-addressed Pin/Identity, and Provenance/LossyImportWarning types the OFL importer normalizes into and reuses unchanged.
provides:
  - internal/fixture/ofl package (model.go, normalize.go, fetch.go): OFL JSON parsing, canonical normalization with lossy-warning surfacing, and an SSRF-guarded live-fetch-plus-cache client.
  - "fixture import" CLI route (--ofl <man>/<key> | --ofl-file <path>, --out <path>) under the existing fixture command scope.
  - A pinned, real, offline OFL test corpus (tests/fixtures/ofl/) spanning D-05's v1 target set plus one pixel/matrix exotic-construct fixture.
  - fixture.Validate exported from internal/fixture/decode.go (was unexported `validate`).
affects: [pool, deployment, substitution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "OFL import normalizes into the same canonical FixtureDefinition and runs the exact same fixture.Validate + fixture.Pin pipeline hand-authored YAML uses (no parallel validation logic)."
    - "Unmapped/unsupported source-format constructs become explicit LossyImportWarning entries on Provenance rather than being silently dropped or causing rejection."
    - "SSRF guard for a user-configurable fetch target: scheme allowlist (http/https) + default-host allowlist with explicit --allow-mirror opt-in, both checked before any request is issued."

key-files:
  created:
    - internal/fixture/ofl/model.go
    - internal/fixture/ofl/normalize.go
    - internal/fixture/ofl/fetch.go
    - internal/fixture/ofl/normalize_test.go
    - internal/fixture/ofl/fetch_test.go
    - tests/fixtures/ofl/README.md
    - tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json
    - tests/fixtures/ofl/chauvet-dj_washfx.json
    - tests/fixtures/ofl/chauvet-dj_intimidator-spot-260.json
    - tests/fixtures/ofl/american-dj_vizi-q-wash7.json
  modified:
    - internal/command/fixture.go
    - internal/command/fixture_test.go
    - internal/fixture/decode.go

key-decisions:
  - "Canonical capability mapping is fixture-level, not channel/DMX-level: every OFL channel mapping onto the same v1 CapabilityType merges into one Capability per type (range = union of contributing channels' normalized DMX occupancy), avoiding decode.go's same-type overlap rejection entirely and matching D-08's 'no OFL channel/mode shape leaks downstream'."
  - "OFL's single ShutterStrobe capability type is split into canonical shutter (static aperture: Open/Closed/Frost/Iris) vs strobe (dynamic/timed: Strobe/Pulse/Ramp*/Lightning/other) by its shutterEffect field."
  - "A wheel-based capability (WheelSlot/WheelRotation/WheelShake/WheelSlotRotation) maps to canonical gobo only when its channel name contains 'gobo' (case-insensitive); a color wheel's identically-typed capability stays unmapped (a discrete slot selection is not the same semantic as continuous ColorIntensity)."
  - "Template (pixel/matrix) channels are never folded into the flat canonical Capabilities list regardless of their own capability type -- every template channel construct is always a LossyImportWarning, since per-pixel addressing is a structurally different shape than one fixture-wide capability."
  - "Confirmed fetch URL pattern (RESEARCH Open Question 2): raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/<man>/<key>.json, verified live at execution time."
  - "OFL live fetch cache is content-addressed (sha256) under the OS user cache dir, best-effort (a cache-write failure never fails an otherwise-successful fetch)."

patterns-established:
  - "Source-format importer (OFL) parses into its own permissive intermediate model, then normalizes into the canonical domain type and runs the domain type's own existing validation -- never a parallel validation path."
  - "fixture import --ofl-file / --ofl mirrors linear preview's --snapshot / --remote dual-source-with-shared-downstream-pipeline shape."

requirements-completed: [FIXT-03, FIXT-06]

coverage:
  - id: D1
    description: "An OFL fixture JSON (generic RGB PAR) imports through the canonical validate+pin pipeline into the same FixtureDefinition type a hand-authored fixture uses, with identical capability-type set behavior."
    requirement: "FIXT-03"
    verification:
      - kind: unit
        ref: "internal/fixture/ofl/normalize_test.go#TestNormalizeCanonicalPipeline"
        status: pass
    human_judgment: false
  - id: D2
    description: "An OFL fixture outside the v1 target set (WashFX's pixel/matrix construct) still imports successfully, surfacing its unmodeled construct as an explicit LossyImportWarning rather than failing."
    requirement: "FIXT-06"
    verification:
      - kind: unit
        ref: "internal/fixture/ofl/normalize_test.go#TestNormalizeLossyWarning"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every OFL capability the v1 canonical model does not represent is accounted for by at least one warning (no silent drop) -- proven with an exact, hand-counted warning total against a real fixture."
    requirement: "FIXT-06"
    verification:
      - kind: unit
        ref: "internal/fixture/ofl/normalize_test.go#TestNormalizeNoSilentDrop"
        status: pass
    human_judgment: false
  - id: D4
    description: "A mirror URL with a non-http(s) scheme, or a non-default host without --allow-mirror, is refused before any network request is made (SSRF guard)."
    requirement: "FIXT-03"
    verification:
      - kind: unit
        ref: "internal/fixture/ofl/fetch_test.go#TestFetchRejectsBadScheme"
        status: pass
      - kind: unit
        ref: "internal/fixture/ofl/fetch_test.go#TestFetchRejectsUnapprovedHost"
        status: pass
    human_judgment: false
  - id: D5
    description: "golc fixture import --ofl-file <corpus file> --out <path> imports offline (no network call) with ExitCode 0 and writes a pinned canonical fixture + provenance envelope."
    requirement: "FIXT-03"
    verification:
      - kind: unit
        ref: "internal/command/fixture_test.go#TestFixtureImportRoute"
        status: pass
      - kind: manual_procedural
        ref: "golc fixture import --ofl-file tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json --out <tmp> (built cmd/golc-project binary, run directly during execution)"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-07-21
status: complete
---

# Phase 2 Plan 3: OFL Import Summary

**OFL fixture import (`golc fixture import --ofl-file|--ofl`) through the same canonical normalize/validate/pin pipeline as hand-authored YAML, with SSRF-guarded fetch and explicit lossy-import warnings for unmapped constructs, backed by a pinned real 4-fixture offline corpus.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-07-21
- **Tasks:** 3
- **Files modified:** 11 (8 created new for Task 1's corpus/tests, 3 created + 2 modified for Tasks 2/3)

## Accomplishments
- `internal/fixture/ofl` package: permissive OFL JSON model (`model.go`), a normalizer (`normalize.go`) that maps OFL's channel/capability vocabulary onto the canonical `fixture.FixtureDefinition`/`Capability` types and surfaces every unmapped construct as a `LossyImportWarning`, and an SSRF-guarded live-fetch-plus-cache client (`fetch.go`).
- `golc fixture import --ofl <man>/<key> [--mirror <url>] [--allow-mirror] --out <path>` and `golc fixture import --ofl-file <path> --out <path>` self-registered under the existing `fixture` command scope, writing a pinned canonical fixture + provenance envelope.
- A pinned, real, network-free OFL test corpus (`tests/fixtures/ofl/`, 4 fixtures spanning D-05's v1 target set: a generic RGB PAR, an LED wash with a pixel matrix, a moving-head spot, and a moving-head wash), with the confirmed raw-GitHub-content fetch URL pattern and sha256 hashes recorded in its README.
- `fixture.Validate` exported from `internal/fixture/decode.go` so the OFL normalizer runs the identical post-decode validation logic hand-authored YAML uses, rather than a second copy of it.

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin an offline OFL corpus + failing normalize/fetch/import tests** - `a0644cc` (test)
2. **Task 2: OFL model + normalizer onto the canonical FixtureDefinition** - `ea097b5` (feat)
3. **Task 3: OFL fetch/cache client (SSRF-guarded) + fixture import route** - `9709191` (feat)

_TDD flow followed per task: Task 1's tests were verified to fail to build/run (RED) before Tasks 2/3 implemented the package/route to green._

## Files Created/Modified
- `internal/fixture/ofl/model.go` - OFL JSON intermediate model (permissive decode; malformed JSON or missing name/availableChannels/modes -> `GOLC_FIXTURE_OFL_INVALID`)
- `internal/fixture/ofl/normalize.go` - OFL -> canonical mapping, lossy-warning generation, validate+pin
- `internal/fixture/ofl/fetch.go` - SSRF-guarded fetch (scheme/host validation, timeout, size cap) + content-addressed cache
- `internal/fixture/ofl/normalize_test.go` - `TestNormalizeCanonicalPipeline`, `TestNormalizeLossyWarning`, `TestNormalizeNoSilentDrop`, `TestNormalizeCorpusFixturesAllImport`
- `internal/fixture/ofl/fetch_test.go` - `TestFetchRejectsBadScheme`, `TestFetchRejectsUnapprovedHost`, plus bonus `TestFetchAllowsApprovedMirrorAndCaches`/`TestFetchRejectsOversizedResponse`
- `internal/command/fixture.go` - adds the `fixture import` route, arg parsing, and the canonical fixture+provenance output envelope
- `internal/command/fixture_test.go` - adds `TestFixtureImportRoute`
- `internal/fixture/decode.go` - exports `Validate` (was unexported `validate`)
- `tests/fixtures/ofl/README.md` - pinned corpus manifest: keys, sha256 hashes, confirmed fetch URL pattern, MIT license note
- `tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json`, `chauvet-dj_washfx.json`, `chauvet-dj_intimidator-spot-260.json`, `american-dj_vizi-q-wash7.json` - pinned real OFL fixtures

## Decisions Made
- **Capability mapping is fixture-level, not per-channel/DMX-indexed.** The canonical `FixtureDefinition` has no channel concept at all (D-08), so every OFL channel mapping onto the same `CapabilityType` merges (by DMX-range union) into exactly one `Capability` per type. This both matches the canonical model's abstraction level and sidesteps `decode.go`'s same-type-range-overlap rejection by construction (only ever one range per type is ever produced).
- **`ShutterStrobe` splits into `shutter` vs `strobe`** by OFL's own `shutterEffect` field (static aperture states `Open`/`Closed`/`Frost`/`Iris` -> `shutter`; everything else, including any future/unrecognized `shutterEffect` value, -> `strobe`). This is a defensible, documented GOLC design choice — OFL itself does not distinguish these as separate capability types.
- **Wheel-based capabilities (`WheelSlot`/`WheelRotation`/`WheelShake`/`WheelSlotRotation`) map to canonical `gobo` only when the channel name contains "gobo".** A color wheel's discrete slot selection is semantically different from `ColorIntensity`'s continuous RGB control, so it deliberately stays unmapped (a warning) rather than being silently treated as equivalent to true color-intensity capability.
- **Template (pixel/matrix) channels are always unmodeled**, regardless of their own capability type — per-pixel addressing cannot be represented by the canonical model's flat, fixture-wide `Capabilities` list without misrepresenting "N independently addressable pixels" as "one capability."
- **Confirmed OFL live-fetch URL pattern** (RESEARCH Open Question 2): `raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/<man>/<key>.json` — verified with a live request during execution, not assumed.
- **Corpus fixture selection**: chose real, well-known chauvet-dj/american-dj fixtures over OFL's `generic` manufacturer, because `generic`'s entries (simple faders/dimmers) were too minimal to exercise the mapping/lossy-warning logic meaningfully; `chauvet-dj/washfx` doubles as both the "LED wash" D-05 category example and the "pixel/matrix outside v1 target set" D-06 example, keeping the corpus at 4 files instead of needing a 5th purely-exotic fixture.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Exported `fixture.Validate` from `internal/fixture/decode.go`**
- **Found during:** Task 2 (OFL model + normalizer)
- **Issue:** The plan requires `ofl.Normalize` to "run the normalized definition through internal/fixture validation" using the *same* pipeline `Decode` uses — but `decode.go`'s post-decode validation function (`validate`) was unexported, so package `ofl` could not call it without either duplicating ~50 lines of validation logic (range/type/overlap checks) or reflecting the def back through a YAML round-trip (wasteful and fragile). Duplicating the logic would also create exactly the "two independently-evolving validation mechanisms" drift risk RESEARCH's own D-16/Pattern-3 guidance explicitly warns against for this repo.
- **Fix:** Renamed the internal call site to an exported `Validate(def FixtureDefinition) error` wrapper in `decode.go` (one-line body delegating to the existing `validate`); `Decode` now calls `Validate` instead of the unexported name. Behavior is unchanged — this is purely an export, not a logic change.
- **Files modified:** `internal/fixture/decode.go`
- **Verification:** Full `internal/fixture` package test suite (pre-existing `TestLoad`, `TestDecodeRejects`, `TestDecodeAdjacency`, `TestDecodeDeterministic`, `TestIdentityHashStable`, `TestIdentityHashKeyOrderStable`, `TestIdentityComplete`, `TestProvenance`) remains green after the change.
- **Committed in:** `ea097b5` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to satisfy the plan's own explicit "run the same validation pipeline" requirement without duplicating logic; no scope creep, no behavior change to existing hand-authored YAML validation.

## Issues Encountered
None — network access to fetch the real OFL corpus and to confirm the live fetch URL pattern was available during execution; all design decisions documented above were resolved directly against real OFL fixture data rather than guessed.

## User Setup Required
None - no external service configuration required. (Live `fixture import --ofl` fetch requires outbound network access at the time a user runs it, which is a runtime/environment concern, not a setup step.)

## Next Phase Readiness
- `internal/fixture` (hand-authored YAML) and `internal/fixture/ofl` (OFL import) both now feed the identical canonical `FixtureDefinition`/`Provenance` types with pinned identity — ready for `internal/pool`/`internal/deployment` (POOL-01/02) to consume fixtures from either source indistinguishably.
- No blockers. The `internal/fixture/ofl` package's capability-mapping table is intentionally scoped to what the pinned corpus exercises (9 of the ~30 OFL capability types are directly handled; everything else is a documented, tested lossy warning) — a future phase adding new v1 CapabilityType values (for example a dedicated wheel/color-macro type) would extend `mapCapabilityType` rather than requiring a redesign.

---
*Phase: 02-modular-fixtures-and-deployments*
*Completed: 2026-07-21*

## Self-Check: PASSED

All created files verified present on disk; all three task commit hashes (`a0644cc`, `ea097b5`, `9709191`) verified present in git history.
