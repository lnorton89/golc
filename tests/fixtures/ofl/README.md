# OFL offline test corpus (Phase 2, Plan 02-03)

This directory pins a small, real Open Fixture Library (OFL) fixture
corpus so `internal/fixture/ofl`'s normalize/fetch tests and
`internal/command`'s `fixture import` route tests never depend on live
network access. Every file below was fetched verbatim (byte-for-byte, no
reformatting) from the upstream OFL repository and is pinned by its
sha256 content hash so an accidental future edit is immediately visible
in code review.

## Confirmed fetch URL pattern (RESEARCH Open Question 2)

OFL's canonical per-fixture JSON is fetched by raw GitHub content, not
through the documented `/api/v1` REST API (which exposes search/
manufacturer-listing/fixture-editor endpoints, not a direct
GET-one-fixture-by-key endpoint):

```
https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/<manufacturer>/<key>.json
```

Confirmed directly at plan-execution time (`curl` against the URL above
for `chauvet-dj/pan-tilt` and each corpus key below; HTTP 200 for all).
`internal/fixture/ofl/fetch.go`'s `defaultOFLURLPattern` uses this exact
pattern.

## License note (RESEARCH Pitfall 1)

The whole `OpenLightingProject/open-fixture-library` GitHub repository,
including every file under `fixtures/*.json`, is registered as a single
MIT-licensed repository -- there is no separate data-only Creative
Commons split to account for. These pinned corpus files are imported and
redistributed here under that same MIT license.

## Corpus filename convention

Each file is named `<manufacturer-key>_<fixture-key>.json`, mirroring
OFL's own `fixtures/<manufacturer>/<key>.json` repository layout with an
underscore in place of the path separator.
`internal/command/fixture.go`'s `oflSourceFromFilename` relies on this
exact convention to derive a "fixture import --ofl-file" invocation's
`<manufacturer>/<key>` source label without requiring a separate
`--manufacturer` flag.

## Pinned fixtures

Selected to span D-05's v1 target set (simple/color-changing PARs and
wash fixtures, plus moving-head spot/wash), favoring stable
generic-manufacturer-style entries where a true "generic" manufacturer
entry did not have a rich enough capability set to exercise this
importer's mapping and lossy-warning logic.

| File | OFL manufacturer/key | Category | sha256 |
|------|----------------------|----------|--------|
| `chauvet-dj_led-par-64-tri-b.json` | `chauvet-dj/led-par-64-tri-b` | Color Changer (RGB PAR) | `c01e998fdf3f836ca990b2b63085a644f5d48a44b1814126e12adb7158d32e99` |
| `chauvet-dj_washfx.json` | `chauvet-dj/washfx` | Color Changer (LED wash, pixel/matrix) | `63fb427c9b4f6ed78da8a5bafca2e70999dbc0cefac87b4816e31646f69a9ca6` |
| `chauvet-dj_intimidator-spot-260.json` | `chauvet-dj/intimidator-spot-260` | Moving Head (spot) | `1599f9d61aa5ef21778f8309342c5f92a50317b8f9222eea251ed56f7e2828b2` |
| `american-dj_vizi-q-wash7.json` | `american-dj/vizi-q-wash7` | Moving Head (wash, pixel/matrix) | `7ec880042becd37c51344d3b204d982a3ec7c278f35f96ccc908426d5d8b1fac` |

Verify any file's pinned hash locally with:

```bash
sha256sum tests/fixtures/ofl/<file>.json
```

### Why `chauvet-dj_washfx.json` doubles as the "outside the v1 target
### set" exotic-construct fixture (D-06)

WashFX declares both plain (fixture-level) RGB/strobe/dimmer channels
*and* a pixel matrix (`matrix.pixelKeys` plus `templateChannels` for
per-pixel Red/Green/Blue). `internal/fixture/ofl/normalize.go` never
folds a template (per-pixel) channel into the canonical model's flat,
fixture-level `Capabilities` list -- doing so would silently misrepresent
"N independently addressable pixels" as "one fixture-wide capability".
Every template channel construct instead becomes an explicit
`LossyImportWarning`, and the fixture still imports successfully
end-to-end (D-06: unsupported/lossy OFL constructs are surfaced, never
silently dropped and never rejected outright). This is the real-world
fixture `TestNormalizeLossyWarning`/`TestNormalizeNoSilentDrop`
(`internal/fixture/ofl/normalize_test.go`) exercise.

## No OFL hazard/safety field

None of these pinned files (and no OFL fixture generally, per
`docs/capability-types.md`) declares a hazard/safety-severity field.
`internal/fixture/ofl/model.go`/`normalize.go` do not parse or expect one
(RESEARCH Pitfall 2) -- POOL-07's severity taxonomy, when it is built in
a later plan, is GOLC-original domain logic, not something extracted from
imported fixture data.
