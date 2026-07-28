---
phase: 04-observable-art-net-live-output
plan: 01
subsystem: artnet
tags: [artnet, dmx, fixture-model, protocol-codec, go]

# Dependency graph
requires:
  - phase: 02-modular-fixtures-and-deployments
    provides: fixture.FixtureDefinition/Mode/Capability canonical model and decode.go's single Validate() entrypoint
  - phase: 03-deterministic-show-programming-and-playback
    provides: playback.Frame and scene.AttributeSet (semantic per-instance attribute values)
provides:
  - fixture.Mode.Channels ordered DMX channel-layout field (D-16) with fixture.ChannelSlot{Type, Occurrence}
  - Hard-reject validation of missing/invalid channel layouts (D-17): GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING, GOLC_FIXTURE_CHANNEL_TYPE_UNKNOWN, GOLC_FIXTURE_CHANNEL_OCCURRENCE_INVALID
  - OFL-import channel-key resolution into fixture.Mode.Channels through the same fixture.Validate pipeline
  - internal/artnet.EncodeArtDMX byte-exact Art-Net 4 ArtDMX packet encoder, PortAddress universe mapping, sequence-wrap helper
  - internal/artnet.Encode: playback.Frame -> per-universe DMX byte buffers, driven strictly by Mode.Channels
affects: [04-02, 04-03, 04-04, 04-05, 04-06, 04-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GOLC_{DOMAIN}_{CONDITION} diagnostic convention extended into internal/artnet (GOLC_ARTNET_*)"
    - "Pure-transform channel mapping mirroring internal/scene/layer.go's AttributeSet.Overlay style (no I/O, no time dependency, fixed declared order never derived from Capabilities)"
    - "Single shared fixture.Validate() entrypoint extended for D-16/D-17 channel-layout rules, reused unmodified by both hand-authored YAML decode and OFL import"

key-files:
  created:
    - internal/artnet/packet.go
    - internal/artnet/packet_test.go
    - internal/artnet/channelmap.go
    - internal/artnet/channelmap_test.go
  modified:
    - internal/fixture/model.go
    - internal/fixture/decode.go
    - internal/fixture/decode_test.go
    - internal/fixture/identity_test.go
    - internal/fixture/provenance_test.go
    - internal/fixture/ofl/model.go
    - internal/fixture/ofl/normalize.go
    - internal/fixture/ofl/normalize_test.go
    - internal/command/fixture_test.go
    - internal/command/substitution_test.go
    - internal/substitution/plan_test.go
    - schemas/fixture.schema.json

key-decisions:
  - "OFL-derived ChannelSlot entries always use Occurrence 0, since normalize.go's own capability mapping merges every same-Type channel into exactly one canonical Capability per Type -- Occurrence only disambiguates multiple same-Type Capabilities, which this import pipeline never produces (see Deviations)."
  - "DMX byte scaling truncates ([0,1]*255 cast to byte, e.g. 0.5 -> 127), matching the plan's own worked example, not round-half-up."
  - "An instance entirely absent from a Frame's Values map is the backstop blackout case (all-zero bytes, no error); an instance present but missing one declared channel's value is a hard GOLC_ARTNET_CHANNEL_VALUE_MISSING error."

patterns-established:
  - "internal/artnet package seeded: packet.go (protocol codec) + channelmap.go (pure semantic-to-DMX transform), both GOLC_ARTNET_* diagnostics, ready for Plan 02's worker/target/health layer."

requirements-completed: [ARTN-03]

coverage:
  - id: D1
    description: "fixture.Mode gains an ordered, validated D-16/D-17 DMX channel layout (ChannelSlot{Type, Occurrence}); missing/invalid layouts hard-reject at decode time for both hand-authored and OFL-imported fixtures"
    requirement: "ARTN-03"
    verification:
      - kind: unit
        ref: "internal/fixture/decode_test.go#TestChannelLayout"
        status: pass
      - kind: unit
        ref: "internal/fixture/ofl/normalize_test.go#TestNormalizeModeChannels"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/artnet.EncodeArtDMX produces a byte-exact Art-Net 4 ArtDMX packet (id, opcode, protocol version, seq, physical, Port-Address, length, data); PortAddress implements the locked universe-to-Port-Address mapping; sequence wraps 1->255->1 never emitting 0"
    requirement: "ARTN-03"
    verification:
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestEncodeArtDMXGoldenVector"
        status: pass
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestEncodeArtDMXLengthRejections"
        status: pass
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestPortAddressDistinct"
        status: pass
      - kind: unit
        ref: "internal/artnet/packet_test.go#TestSequenceNeverZero"
        status: pass
    human_judgment: false
  - id: D3
    description: "internal/artnet.Encode turns a playback.Frame's semantic per-instance attribute values into per-universe 512-byte DMX buffers, driven strictly by each instance's Mode.Channels declared order, with backstop blackout and loud missing-value diagnostics"
    requirement: "ARTN-03"
    verification:
      - kind: unit
        ref: "internal/artnet/channelmap_test.go#TestEncodeOffsetAndScaling"
        status: pass
      - kind: unit
        ref: "internal/artnet/channelmap_test.go#TestEncodeTwoInstancesSharedBuffer"
        status: pass
      - kind: unit
        ref: "internal/artnet/channelmap_test.go#TestEncodeBlackoutUniverse"
        status: pass
      - kind: unit
        ref: "internal/artnet/channelmap_test.go#TestEncodeMissingChannelValueFails"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-07-22
status: complete
---

# Phase 4 Plan 01: Art-Net Encoding Foundation Summary

**Byte-exact Art-Net 4 ArtDMX encoder plus an additive per-mode DMX channel-order field on the canonical fixture model, wired end-to-end from a playback.Frame to per-universe DMX bytes.**

## Performance

- **Duration:** 45 min
- **Started:** 2026-07-22T07:45:00Z
- **Completed:** 2026-07-22T08:03:56Z
- **Tasks:** 3
- **Files modified:** 16 (4 created, 12 modified)

## Accomplishments
- `fixture.Mode.Channels []ChannelSlot` (D-16) gives every fixture mode an ordered, validated DMX channel layout; a missing layout, an unknown capability type, or an occurrence index with no matching declared Capability is a hard rejection (D-17) through the single shared `fixture.Validate()` path.
- OFL import (`internal/fixture/ofl`) resolves each mode's channel-key list (including matrix/pixel "insert" expansion entries, skipped rather than guessed) into the same canonical `fixture.Mode.Channels` field, running through the identical validation pipeline as hand-authored YAML.
- `internal/artnet.EncodeArtDMX` produces byte-exact Art-Net 4 ArtDMX packets against a golden vector; `PortAddress` implements the locked universe-to-Port-Address mapping (Assumption A1); the sequence helper wraps 1-255 and never emits 0.
- `internal/artnet.Encode` maps a `playback.Frame`'s per-instance semantic values into per-universe 512-byte DMX buffers strictly via each instance's `Mode.Channels` declared order — never via `Capabilities` declaration order.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add additive per-mode DMX channel-order field to the canonical fixture model (D-16/D-17)** - `1914944` (feat)
2. **Task 2: Byte-exact ArtDMX encoder + universe-to-Port-Address mapping + sequence wrap** - `233e5a2` (feat)
3. **Task 3: Semantic-frame-to-DMX channel map using the new Mode.Channels layout** - `f0fad8c` (feat)

**Plan metadata:** (recorded in final commit)

## Files Created/Modified
- `internal/artnet/packet.go` - EncodeArtDMX, PortAddress, nextSeq, Art-Net 4 wire constants
- `internal/artnet/packet_test.go` - golden-byte-vector, length-rejection, Port-Address, sequence tests
- `internal/artnet/channelmap.go` - Encode: Frame + instances + resolver -> per-universe DMX buffers
- `internal/artnet/channelmap_test.go` - offset/scaling, shared-buffer, blackout, missing-value tests
- `internal/fixture/model.go` - `ChannelSlot` type, `Mode.Channels` field
- `internal/fixture/decode.go` - `validateChannelLayouts`: GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING/TYPE_UNKNOWN/OCCURRENCE_INVALID
- `internal/fixture/decode_test.go` - `TestChannelLayout` + updated fixtures to declare valid channel layouts
- `internal/fixture/identity_test.go`, `internal/fixture/provenance_test.go` - updated fixtures to declare channel layouts
- `internal/fixture/ofl/model.go` - re-added `Mode.Channels []json.RawMessage` (channel-key or matrix-insert entries)
- `internal/fixture/ofl/normalize.go` - `resolveModeChannels`/`firstMappedCapabilityType`
- `internal/fixture/ofl/normalize_test.go` - `TestNormalizeModeChannels` + fixed `equivalentHandAuthoredYAML`
- `internal/command/fixture_test.go`, `internal/command/substitution_test.go`, `internal/substitution/plan_test.go` - updated fixtures/helpers to declare channel layouts (D-17 fallout)
- `schemas/fixture.schema.json` - regenerated from the additive `Mode.Channels` field

## Decisions Made
- OFL-derived `ChannelSlot` entries always use `Occurrence: 0` (see Deviations #1) rather than incrementing per repeated same-type channel, because `normalize.go`'s existing capability mapping merges every same-Type channel into exactly one canonical `Capability` per Type — incrementing occurrence would have produced `GOLC_FIXTURE_CHANNEL_OCCURRENCE_INVALID` rejections for common multi-channel RGB/RGBW fixtures.
- DMX scaling truncates toward zero (`byte(value*255)`) rather than rounding half up, matching the plan's own worked example (`color=0.5 -> 127`).
- A channel value fully absent because the *instance itself* is unevaluated in the current Frame is the backstop blackout case (all-zero, no error); a channel value missing from an otherwise-present instance's AttributeSet is a hard error (`GOLC_ARTNET_CHANNEL_VALUE_MISSING`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] OFL channel-occurrence resolution clamped to 0 instead of resolution-order increment**
- **Found during:** Task 1 (OFL wiring)
- **Issue:** The plan's action text said to compute each OFL-resolved `ChannelSlot`'s `Occurrence` "from resolution order" (incrementing per repeated same-Type channel). `normalize.go`'s existing capability-merge logic (`mergeRange`/`capabilitiesFromRanges`) always produces at most one canonical `Capability` per Type, so a naive incrementing implementation produced `Occurrence` values (1, 2, ...) with no matching declared `Capability`, which the new `GOLC_FIXTURE_CHANNEL_OCCURRENCE_INVALID` check correctly rejected — breaking `ofl.Normalize` for every real multi-channel-per-type OFL fixture in the pinned corpus (e.g. the LED PAR 64 TRI-B's separate Red/Green/Blue channels, all mapping to the single `color` CapabilityType).
- **Fix:** OFL-derived `ChannelSlot`s always use `Occurrence: 0`, documented in `resolveModeChannels`'s doc comment as a direct consequence of the existing one-Capability-per-Type merge invariant.
- **Files modified:** internal/fixture/ofl/normalize.go
- **Verification:** `go test ./internal/fixture/ofl/...` green, including the full pinned corpus import test.
- **Committed in:** 1914944 (Task 1 commit)

**2. [Rule 1 - Bug] OFL mode `channels` array can contain matrix/pixel expansion objects, not just channel-key strings**
- **Found during:** Task 1 (OFL wiring)
- **Issue:** `chauvet-dj_washfx.json` (an existing pinned corpus fixture) declares a mode `channels` array containing a plain-string channel key mixed with an `{"insert": "matrixChannels", ...}` expansion object. Decoding `Mode.Channels` as `[]string` failed with a JSON unmarshal error for every fixture using this OFL construct.
- **Fix:** `ofl.Mode.Channels` decodes as `[]json.RawMessage`; `resolveModeChannels` decodes each entry as a string first and skips (rather than errors on) any entry that isn't a plain string, since the expansion object's own per-pixel template channels are already surfaced as `LossyImportWarning`s via the existing `matrixChannelWarning` walk.
- **Files modified:** internal/fixture/ofl/model.go, internal/fixture/ofl/normalize.go
- **Verification:** `go test ./internal/fixture/ofl/...` green, including `TestNormalizeLossyWarning`/`TestNormalizeNoSilentDrop` against the WashFX fixture.
- **Committed in:** 1914944 (Task 1 commit)

**3. [Rule 1 - Bug] D-17's hard rejection broke pre-existing tests repo-wide**
- **Found during:** Task 1 (after adding the channel-layout validation)
- **Issue:** Making a missing channel layout a hard rejection (as D-17 requires) broke every pre-existing test fixture across `internal/fixture`, `internal/fixture/ofl`, `internal/command`, `internal/substitution`, and the committed `schemas/fixture.schema.json` (drift), since none of them declared a channel layout before this plan.
- **Fix:** Added valid `channels:` declarations (or, for Go-struct-literal test fixtures in `internal/substitution/plan_test.go`, a small `channelSlotsFor` test helper deriving slots from the test's own capability list) to every affected fixture, and regenerated `schemas/fixture.schema.json` via `go run ./cmd/golc-project generate`.
- **Files modified:** internal/fixture/decode_test.go, internal/fixture/identity_test.go, internal/fixture/provenance_test.go, internal/fixture/ofl/normalize_test.go, internal/command/fixture_test.go, internal/command/substitution_test.go, internal/substitution/plan_test.go, schemas/fixture.schema.json
- **Verification:** `go test ./...` green except one pre-existing, unrelated failure (see Issues Encountered).
- **Committed in:** 1914944 (Task 1 commit)

**4. [Rule 1 - Bug] DMX byte scaling used round-half-up initially, producing 128 instead of the plan's documented 127 for color=0.5**
- **Found during:** Task 3 (channel map)
- **Issue:** `TestEncodeOffsetAndScaling` initially failed: round-half-up scaling gave `byte(0.5*255+0.5)=128`, but the plan's own worked example states `color=0.5 -> byte[1]=127`.
- **Fix:** Changed `scaleToByte` to truncate toward zero (`byte(value*255)`), matching the plan's documented example exactly.
- **Files modified:** internal/artnet/channelmap.go
- **Verification:** `TestEncodeOffsetAndScaling` passes.
- **Committed in:** f0fad8c (Task 3 commit)

---

**Total deviations:** 4 auto-fixed (all Rule 1 - bug fixes required to make the plan's own stated behavior/acceptance criteria correct and to keep the existing test suite green).
**Impact on plan:** All auto-fixes were necessary for correctness given the plan's own explicit examples and acceptance criteria (D-17's hard-rejection semantics, the plan's documented rounding example, real OFL corpus data shapes). No architectural changes, no scope creep beyond fixing test fixtures that this plan's own additive-but-hard-rejecting validation rule broke.

## Issues Encountered
- `internal/trace/catalog`'s `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails with `GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`. Confirmed via `git stash` that this failure exists on the base commit, independent of any change in this plan. Logged to `.planning/phases/04-observable-art-net-live-output/deferred-items.md`; not touched (out of scope).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/artnet` is seeded with `packet.go` (protocol codec) and `channelmap.go` (pure semantic-to-DMX transform), both following the `GOLC_ARTNET_*` diagnostic convention — ready for Plan 02's worker/target/interface-manager layer to consume directly.
- `fixture.Mode.Channels` is now the single source of truth for DMX channel order across both hand-authored and OFL-imported fixtures; any later phase authoring or importing a fixture must declare a channel layout or hit `GOLC_FIXTURE_CHANNEL_LAYOUT_MISSING` at decode time.
- No blockers for Plan 02 (worker/target/health) or Plan 03 (on-wire send path).

---
*Phase: 04-observable-art-net-live-output*
*Completed: 2026-07-22*

## Self-Check: PASSED

- FOUND: internal/artnet/packet.go
- FOUND: internal/artnet/channelmap.go
- FOUND: SUMMARY.md
- FOUND commits: 1914944, 233e5a2, f0fad8c (all present in `git log --oneline --all`)
