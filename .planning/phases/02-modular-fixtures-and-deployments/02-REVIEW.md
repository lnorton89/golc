---
phase: 02-modular-fixtures-and-deployments
reviewed: 2026-07-21T00:00:00Z
depth: standard
files_reviewed: 39
files_reviewed_list:
  - internal/command/deployment.go
  - internal/command/fixture.go
  - internal/command/fixture_test.go
  - internal/command/pool.go
  - internal/command/pooldeploy_test.go
  - internal/command/poolimpact_test.go
  - internal/command/substitution_test.go
  - internal/contracts/fixture.go
  - internal/contracts/fixture_test.go
  - internal/deployment/model.go
  - internal/deployment/model_test.go
  - internal/fixture/decode.go
  - internal/fixture/decode_test.go
  - internal/fixture/identity.go
  - internal/fixture/identity_test.go
  - internal/fixture/model.go
  - internal/fixture/ofl/fetch.go
  - internal/fixture/ofl/fetch_test.go
  - internal/fixture/ofl/model.go
  - internal/fixture/ofl/normalize.go
  - internal/fixture/ofl/normalize_test.go
  - internal/fixture/provenance.go
  - internal/fixture/provenance_test.go
  - internal/pool/impact.go
  - internal/pool/impact_test.go
  - internal/pool/model.go
  - internal/pool/model_test.go
  - internal/pool/plan.go
  - internal/pool/plan_test.go
  - internal/show/state.go
  - internal/show/state_test.go
  - internal/substitution/plan.go
  - internal/substitution/plan_test.go
  - schemas/fixture.schema.json
  - tests/fixtures/ofl/README.md
  - tests/fixtures/ofl/american-dj_vizi-q-wash7.json
  - tests/fixtures/ofl/chauvet-dj_intimidator-spot-260.json
  - tests/fixtures/ofl/chauvet-dj_led-par-64-tri-b.json
  - tests/fixtures/ofl/chauvet-dj_washfx.json
findings:
  critical: 2
  warning: 3
  info: 2
  total: 7
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-07-21T00:00:00Z
**Depth:** standard
**Files Reviewed:** 39
**Status:** issues_found

## Summary

This phase implements the fixture/pool/deployment/substitution domain model and its command-layer routes. The plan/apply split (integrity + freshness gates), the OFL normalization pipeline's warning-not-drop discipline, and the substitution capability-diff taxonomy are all implemented carefully and are well covered by tests. However, two provable defects undercut guarantees the code explicitly claims to provide:

1. `fixture.Decode`/`fixture.Validate` (the hand-authored-YAML path) never enforces the required-field constraints the fixture's own generated JSON Schema and `jsonschema` struct tags declare (`schema_version` must be `1`, `manufacturer`/`model` must be non-empty, `modes` must be non-empty) — only the `capabilities` constraints are actually checked at runtime.
2. `ofl.Fetch`'s documented "SSRF guard" validates only the initial request URL; the underlying `http.DefaultClient` still follows HTTP redirects without re-validating the redirect target's scheme/host, so a compromised or malicious `--allow-mirror` endpoint can redirect the fetch anywhere.

Three further correctness/robustness gaps (DMX address auto-assignment ignoring real fixture channel width, `Group` records never getting the same duplicate-name/dangling-reference validation `Pool`/`Deployment` get, and `pool update --add` trusting an arbitrary, never-cross-checked stable-key/content-hash pair) are documented below as warnings.

## Critical Issues

### CR-01: `fixture.Decode`/`Validate` never enforces `schema_version`, `manufacturer`, `model`, or `modes` — only `capabilities` is checked

**File:** `internal/fixture/decode.go:67-96`
**Issue:** `internal/fixture/model.go`'s `FixtureDefinition` declares `schema_version` (`jsonschema:"required,enum=1"`), `manufacturer`/`model` (`jsonschema:"required,minLength=1"`), and `modes` (`jsonschema:"required,minItems=1"`) as required, non-empty fields — and the generated `schemas/fixture.schema.json` reflects exactly that contract. But `internal/fixture/decode.go`'s `validate(def FixtureDefinition)` (the function both `fixture.Decode` and `ofl.Normalize` call) only checks `len(def.Capabilities) == 0` and the per-capability type/range rules. It never checks `def.SchemaVersion`, `def.Manufacturer`, `def.Model`, or `len(def.Modes)`.

Because `go.yaml.in/yaml/v4`'s `WithKnownFields()`/`WithUniqueKeys()` reject unknown/duplicate keys but do not enforce "required" fields, a hand-authored fixture YAML that omits `manufacturer:`, `model:`, or `modes:` entirely (or sets `modes: []`, or declares `schema_version: 99`) decodes and validates successfully today. Concretely:
```yaml
schema_version: 7
manufacturer: ""
model: ""
modes: []
capabilities:
  - type: intensity
    range: [0, 1]
```
passes `fixture.Decode` with `ExitCode 0`.

This is a real correctness/data-integrity gap, not just a schema/doc mismatch:
- `fixture.Pin`'s `StableKey` is computed as `def.Manufacturer + "/" + def.Model`; an empty manufacturer/model pins to the degenerate key `"/"`, which collides with any other equally malformed fixture and silently corrupts pool-member identity (`internal/fixture/identity.go:67`).
- `internal/fixture/ofl/model.go`'s `decodeDefinition` (the *OFL* import path) does check for a non-empty name and at least one mode (`internal/fixture/ofl/model.go:123-131`), so the two import paths (hand-authored YAML vs. OFL import) enforce different — and inconsistent — structural guarantees despite the package doc comment's explicit claim that "hand-authored and OFL-imported fixtures run through the exact same validation logic" (`internal/fixture/decode.go:47-50`).
- None of `decode_test.go`'s `TestDecodeRejects` cases exercise a missing/empty `manufacturer`, `model`, `modes`, or wrong `schema_version` — confirming the gap is untested, not intentionally relaxed.

**Fix:**
```go
// internal/fixture/decode.go
func validate(def FixtureDefinition) error {
	if def.SchemaVersion != 1 {
		return fmt.Errorf(
			"GOLC_FIXTURE_SCHEMA_VERSION_UNSUPPORTED: schema_version %d is not supported (only 1 is supported)",
			def.SchemaVersion)
	}
	if strings.TrimSpace(def.Manufacturer) == "" {
		return fmt.Errorf("GOLC_FIXTURE_MANUFACTURER_EMPTY: fixture manufacturer must not be empty")
	}
	if strings.TrimSpace(def.Model) == "" {
		return fmt.Errorf("GOLC_FIXTURE_MODEL_EMPTY: fixture model must not be empty")
	}
	if len(def.Modes) == 0 {
		return fmt.Errorf(
			"GOLC_FIXTURE_MODES_EMPTY: %s %s declares zero modes; a fixture must declare at least one",
			def.Manufacturer, def.Model)
	}
	if len(def.Capabilities) == 0 {
		return fmt.Errorf(
			"GOLC_FIXTURE_EMPTY: %s %s declares zero capabilities; a fixture must declare at least one",
			def.Manufacturer, def.Model)
	}
	// ... existing capability-type/range/overlap checks unchanged
}
```
Add matching `TestDecodeRejects` cases for missing/empty `manufacturer`, `model`, `modes`, and an out-of-range `schema_version`.

### CR-02: `ofl.Fetch`'s SSRF guard validates only the initial URL — HTTP redirects are followed unchecked

**File:** `internal/fixture/ofl/fetch.go:82-121`
**Issue:** `Fetch` calls `validateTargetURL(target, ref.AllowMirror)` (line 84) to enforce that the request targets `http`/`https` and either the default upstream host or an explicitly `--allow-mirror`-opted-in host — this is the file's documented "SSRF guard" (T-02-06). The actual request, however, is issued with `http.DefaultClient.Do(request)` (line 97). `http.DefaultClient` has no custom `CheckRedirect`, so it uses Go's default policy: follow up to 10 redirects, forwarding to whatever the server's `Location` header specifies, with **no re-validation of scheme or host** on any hop after the first.

Once a user has passed `--allow-mirror` for a given host (the code's only gate), that host can 30x-redirect the client to an arbitrary internal address (e.g. a cloud metadata endpoint, `localhost`, or an RFC1918 address) and `Fetch` will follow it and return the response bytes as if they came from the validated host. This defeats the stated purpose of "resolved target URL's scheme and host are validated **before any request is issued**" (doc comment, lines 73-75) — the guarantee only holds for the first request, not for any redirect hop.

**Fix:** Re-validate every redirect target through the same `validateTargetURL` check, or refuse redirects entirely (the fixture JSON is served directly by raw.githubusercontent.com with no redirect in the happy path):
```go
func Fetch(ctx context.Context, ref OFLRef) ([]byte, error) {
	target := resolveTargetURL(ref)
	parsed, err := validateTargetURL(target, ref.AllowMirror)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: %v", err)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if _, err := validateTargetURL(req.URL.String(), ref.AllowMirror); err != nil {
				return fmt.Errorf("GOLC_FIXTURE_OFL_MIRROR_HOST: redirect to %q rejected: %v", req.URL, err)
			}
			return nil
		},
	}
	response, err := client.Do(request)
	// ... unchanged
```

## Warnings

### WR-01: DMX auto-address assignment always assumes a 1-channel-wide fixture, contradicting its own "never under-conservative" claim

**File:** `internal/pool/impact.go:42-52`, `internal/deployment/model.go:134-184`
**Issue:** Every call site in this repository proposes new `deployment.Instance`s via `deployment.NextFreeAddress(existing, defaultInstanceChannelCount)` with `defaultInstanceChannelCount = 1` (`internal/pool/impact.go:52`, used at line 250). `NextFreeAddress`'s own doc comment claims: "This never collides with a real, wider-than-modeled fixture... at worst, over-conservative... never under-conservative" (`internal/deployment/model.go:144-149`).

That claim only holds if the `channelCount` passed in matches the *real* channel width of every instance already in the search. Since every caller always passes `1`, and almost no real DMX fixture (RGB PAR, moving-head spot/wash — the very fixture classes this phase targets, per D-05) occupies only 1 channel, `NextFreeAddress` will happily pack two 3+-channel fixtures back-to-back one address apart (e.g. universe 1, address 1 and address 2), which is a genuine DMX address collision once the real fixture's channel span is considered — the opposite of "never under-conservative." `overlapsExisting` (`internal/deployment/model.go:172-184`) has no way to know this because `Instance` carries no channel-width field at all.

This is called out in the code as a known, deferred model gap ("a future plan's concern"), but the accompanying claim that the current behavior is safe is not accurate for this phase's own target fixture set, and there is no test exercising the multi-channel-collision scenario (`TestNextFreeAddressBoundary` only exercises the boundary with a caller-supplied `channelCount=4`, not the `defaultInstanceChannelCount=1` path actually used by `BuildImpactPlan`).
**Fix:** Either (a) thread the fixture's real capability-derived channel count through `PoolMemberSpec`/`ImpactRequest` into `NextFreeAddress` instead of hardcoding `1`, or (b) narrow the doc comment's claim to be explicit that collision-safety is not guaranteed until `Instance` carries real channel width, and track this as an explicit, tested follow-up rather than an implicit assumption baked into a `const`.

### WR-02: `Group` records get none of the duplicate-name / dangling-reference validation `Pool` and `Deployment` get

**File:** `internal/show/state.go:119-147`, `internal/pool/model.go`
**Issue:** `show.State.validate` calls `pool.Validate`+`pool.ValidateUniqueNames` for every `Pool`, and `deployment.ValidateUniqueNames`+`deployment.ValidateSingleActive` for `Deployment`s — but never validates `s.Groups` at all. `internal/pool/model.go` defines `Group`/`MemberRef` (POOL-01/D-10) but exposes no `ValidateUniqueNames`-equivalent for groups, and nothing checks that a `MemberRef.PoolID`/`PoolMemberID` actually refers to an existing pool/member. As a result:
- Two `Group`s with the same `Name` can be saved without error (unlike the explicit "a duplicate name is always rejected, never silently permitted" guarantee documented for `Pool`/`Deployment`).
- A `MemberRef` pointing at a pool or pool member that no longer exists is never flagged by `Load`/`Save`.

No `group create` command exists yet in this phase's files, so this gap is not directly reachable from the CLI today, but `Group`/`MemberRef` are already part of the persisted `ShowState` schema and are read/written by `pool.BuildImpactPlan`/`pool.Apply`'s dependent walk, so the gap will surface as soon as group authoring is added.
**Fix:** Add a `pool.ValidateUniqueGroupNames` (mirroring `ValidateUniqueNames`) and a `pool.ValidateGroupReferences(pools, groups)` check that every `MemberRef` resolves to an existing pool/member, and call both from `show.state.validate`.

### WR-03: `pool update --add` accepts an arbitrary stable-key/content-hash pair with no cross-check against a real, pinned fixture

**File:** `internal/command/pool.go:175-186`, `internal/pool/model.go:106-117`
**Issue:** `parsePoolMemberSpec` (`internal/command/pool.go:179-186`) parses `--add "<stable_key>|<content_hash>|<mode>"` as three opaque strings and passes them straight through to `pool.PoolMemberSpec`/`pool.NewPoolMember`, which only checks that `FixtureStableKey` is non-empty (`internal/pool/model.go:108-111`) — `FixtureContentHash` is never checked for emptiness, and neither is ever checked against any actual fixture definition. Nothing in `pool update`'s flow reads a fixture file or looks up a previously "fixture import"/"fixture inspect"-pinned identity, so a user can add a member pinned to a fixture stable key/content hash that was never validated, decoded, or even exists.

This is inconsistent with `pool substitute` (`internal/command/pool.go:580-605`), which does read and `fixture.Decode`/`fixture.Pin` the actual `--from`/`--to` files before building its plan, deriving the pinned identity from real file content rather than trusting a caller-supplied string. Given FIXT-05's whole purpose is content-addressed pinning specifically to prevent an unreviewed/unverified fixture reference from entering a show, `pool update --add` currently bypasses that guarantee entirely.
**Fix:** Either require `pool update --add` to take a fixture file path (like `pool substitute --to`) and derive `FixtureStableKey`/`FixtureContentHash` via `fixture.Decode`+`fixture.Pin`, or explicitly document (in the route summary and `GOLC_POOL_APPLY_USAGE` help text) that `--add` is a low-level, unverified reference and that pinning verification is the caller's responsibility.

## Info

### IN-01: `pool update`/`pool substitute` silently ignore `--json` when `--out` is also supplied

**File:** `internal/command/pool.go:360-369`, `internal/command/pool.go:617-626`
**Issue:** `runPoolUpdate`/`runPoolSubstitute` check `parsed.outPath != ""` first and return immediately via `writeImpactPlan`; the `--json` flag is only consulted if `--out` was not given. If a caller passes both `--out plan.json --json`, `--json` is silently a no-op with no diagnostic, which can be confusing during scripting/debugging.
**Fix:** Either reject the combination in the arg parser (`GOLC_POOL_APPLY_USAGE: --out and --json are mutually exclusive`) or print the plan to stdout in addition to writing `--out` when both are given.

### IN-02: `runFixtureImport`/`writeImpactPlan` write `--out` without creating parent directories, unlike `show.Save`

**File:** `internal/command/fixture.go:369-373`, `internal/command/pool.go:305-316`
**Issue:** `show.Save` calls `os.MkdirAll(filepath.Dir(resolved), 0o755)` before writing (`internal/show/state.go:105`), but `runFixtureImport`'s `os.WriteFile(destination, payload, 0o644)` and `writeImpactPlan`'s equivalent call do not — an `--out` path under a not-yet-existing directory fails with a raw `os` error instead of succeeding like a `show.Save` call to a similarly nested path would.
**Fix:** Add the same `os.MkdirAll(filepath.Dir(destination), 0o755)` step before `os.WriteFile` in both `runFixtureImport` and `writeImpactPlan` for consistent behavior.

---

_Reviewed: 2026-07-21T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
