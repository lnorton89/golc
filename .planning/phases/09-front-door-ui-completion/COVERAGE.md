# API Coverage — Open Fixture Library (OFL)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.
>
> Produced at plan time for Phase 9 because the API-coverage detector fired on this phase's scope
> (`api-coverage.cjs --json` → `detected: true`, signals: `rest`, `sdk`, `api`). The integrated
> service is the Open Fixture Library, a GitHub-hosted fixture catalog reached over HTTP through the
> existing SSRF-guarded client in `internal/fixture/ofl`.
>
> Baseline note: this is the **second** integration round against OFL (Phase 2 built
> `fixture import --ofl`). Per the full-coverage rule this matrix re-decides every capability from a
> full-coverage baseline rather than inheriting Phase 2's opt-outs silently.

| capability | decision | reason |
|---|---|---|
| fetch-fixture-by-manufacturer-key | INTEGRATE | |
| normalize-fixture-to-golc-definition | INTEGRATE | |
| lossy-import-warnings | INTEGRATE | |
| content-addressed-fetch-cache | INTEGRATE | |
| manufacturer-index | INTEGRATE | |
| search-by-manufacturer-name | INTEGRATE | |
| import-from-local-ofl-json-file | INTEGRATE | |
| user-configured-mirror-base-url | OPT-OUT | not needed on screen — `fixture import --mirror/--allow-mirror` remains available from the CLI; exposing a mirror override in the desktop UI would surface an SSRF-relevant opt-in as a casual setting |
| search-by-fixture-model-name | OPT-OUT | not possible from the currently allowed host — no full fixture-name index is committed to the OFL repository (`build:register` is a website-build-time artifact, live-404 verified in 09-RESEARCH.md A2), and raw content serving has no directory listing. Would require adding `api.github.com` as a second SSRF-allowlisted host with a 60 req/hr unauthenticated rate limit; 09-RESEARCH.md Open Question 1 recommends deferring that as a discrete security decision. Tracked as a v1.x follow-up |
| per-manufacturer-fixture-listing | OPT-OUT | not possible from the currently allowed host — same directory-listing limitation as above; deferred with `search-by-fixture-model-name` |
| faceted-filtering-by-category-or-capability | OPT-OUT | explicitly out of scope — D-03 scopes the library to basic text search, "no faceted filtering", for a v1 single small rig |
| rdm-id-lookup | OPT-OUT | not needed — GOLC has no RDM support anywhere in v1; the field is present in the manufacturer index and is deliberately not projected |
| manufacturer-website-links | OPT-OUT | not needed yet — the value is fetched and available in the projection but no on-screen link is rendered this phase; adding one is a UI decision with no new API surface |
| fixture-images-and-gallery | OPT-OUT | not needed — GOLC's fixture model is channel/capability data only; imagery has no consumer |
| ofl-export-plugins (QLC+, e1.31, DMXControl, …) | OPT-OUT | explicitly out of scope — GOLC imports fixture definitions; it is not an OFL export target and has its own canonical fixture format |
| submit-or-contribute-fixture-upstream | OPT-OUT | explicitly out of scope — contributing back to OFL is a browser workflow against a third-party repository, not a GOLC capability |
| fixture-key-autocomplete-or-validation-before-fetch | OPT-OUT | not possible from the currently allowed host — validating a fixture key without fetching it requires the same missing index; an invalid key surfaces as the existing `fixture import` fetch failure with its canonical diagnostic |

## Where each INTEGRATE lands

| capability | plan |
|---|---|
| manufacturer-index, search-by-manufacturer-name | `09-05-PLAN.md` (`internal/fixture/ofl/manufacturers.go`, `FixtureLibraryService.SearchOFL`) |
| fetch-fixture-by-manufacturer-key, normalize-fixture-to-golc-definition, lossy-import-warnings, content-addressed-fetch-cache | `09-06-PLAN.md` — reused verbatim through the existing `fixture import --ofl` route; Phase 2's implementation is unchanged |
| import-from-local-ofl-json-file | already shipped in Phase 2 (`fixture import --ofl-file`); reachable unchanged from the CLI. The desktop equivalent for hand-authored YAML is `09-07-PLAN.md` |

## Developer decision requested

The two `not possible from the currently allowed host` opt-outs
(`search-by-fixture-model-name`, `per-manufacturer-fixture-listing`) are the only ones that reduce
what an operator can do relative to a naive reading of D-01/D-03's "OFL catalog search". They are
carried honestly into the UI: plan 09-05 requires a permanently visible on-screen note stating that
the search matches manufacturer names and that the operator supplies the fixture key. If model-name
search is required for v1 instead, the alternative is adding `api.github.com` as a second explicit
SSRF-allowlisted host constant — reviewable as its own security decision, with the unauthenticated
GitHub rate limit as the accepted cost.
