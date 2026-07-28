---
phase: 02-modular-fixtures-and-deployments
fixed_at: 2026-07-22T01:44:21Z
review_path: .planning/phases/02-modular-fixtures-and-deployments/02-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-07-22T01:44:21Z
**Source review:** .planning/phases/02-modular-fixtures-and-deployments/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (fix_scope: critical_warning -- CR-01, CR-02, WR-01, WR-02, WR-03; IN-01/IN-02 out of scope for this run)
- Fixed: 5
- Skipped: 0

## Fixed Issues

### CR-01: `fixture.Decode`/`Validate` never enforces `schema_version`, `manufacturer`, `model`, or `modes`

**Files modified:** `internal/fixture/decode.go`, `internal/fixture/decode_test.go`
**Commit:** `02173ab`
**Applied fix:** Added `SchemaVersion != 1`, empty-`Manufacturer`, empty-`Model`, and zero-`Modes` checks to `validate()` ahead of the existing capabilities check, each with its own `GOLC_FIXTURE_*` diagnostic exactly as REVIEW.md specified. Verified the OFL-import path (`ofl.Normalize`/`decodeDefinition`) already supplies all four fields, so no OFL behavior changed. Added 8 new `TestDecodeRejects` cases (unsupported schema_version, empty/missing manufacturer, empty/missing model, zero/missing modes) plus confirmed the full `internal/fixture/...` suite still passes.

### CR-02: `ofl.Fetch`'s SSRF guard validates only the initial URL — redirects are followed unchecked

**Files modified:** `internal/fixture/ofl/fetch.go`, `internal/fixture/ofl/fetch_test.go`
**Commit:** `b01ddc4`
**Applied fix:** Replaced the bare `http.DefaultClient.Do` call with a dedicated `http.Client` carrying a `CheckRedirect` hook that re-runs `validateTargetURL` (the same scheme/host guard the initial request uses) against every redirect hop, applied verbatim from REVIEW.md's suggested fix. Added `TestFetchRejectsRedirectToDisallowedScheme`, which proves a server that 302-redirects to a non-http(s) target is rejected before the client follows it (this test fails against the pre-fix code path, confirming the fix is load-bearing).

### WR-01: DMX auto-address assignment always assumes a 1-channel-wide fixture

**Files modified:** `internal/pool/impact.go`
**Commit:** `65e00c1`
**Applied fix:** Chose REVIEW.md's fix option (b) rather than (a): threading a real, capability-derived channel count through `PoolMemberSpec`/`ImpactRequest`/`NextFreeAddress` would require a new channel-width concept the fixture/capability model does not currently carry (`fixture.Capability` has no channel-count field) -- a design change out of scope for an atomic bug fix. Instead, rewrote `defaultInstanceChannelCount`'s doc comment to remove the inaccurate "this never collides ... never under-conservative" blanket claim and explicitly document the known collision gap for multi-channel fixtures as a tracked follow-up, per the reviewer's own fallback option. No behavior changed; `internal/pool/...` and the full suite still pass.

### WR-02: `Group` records get no duplicate-name / dangling-reference validation

**Files modified:** `internal/pool/model.go`, `internal/pool/model_test.go`, `internal/show/state.go`, `internal/show/state_test.go`
**Commit:** `ef6807f`
**Applied fix:** Added `pool.ValidateUniqueGroupNames` (mirroring `pool.ValidateUniqueNames`, `GOLC_GROUP_DUPLICATE_NAME`) and `pool.ValidateGroupReferences` (checks every `MemberRef.PoolID`/`PoolMemberID` resolves to a real pool/pool member, `GOLC_GROUP_DANGLING_REFERENCE`), then wired both into `show.state.validate` alongside the existing Pool/Deployment checks. Added `TestGroupUniqueNamesRejected`/`TestGroupReferencesValidated` in `internal/pool` and `TestShowStateGroupValidation` in `internal/show` proving duplicate group names and dangling pool/member references are now rejected at `Save`, and that a valid group round-trips through `Save`/`Load`. Full `go test ./...` passes.

### WR-03: `pool update --add` accepts an arbitrary, never-cross-checked stable-key/content-hash pair

**Files modified:** `internal/command/pool.go`
**Commit:** `fcd6278`
**Applied fix:** Chose REVIEW.md's fix option (documentation) rather than requiring `--add` to take a fixture file path: changing `--add`'s argument shape is a breaking CLI change with a much larger blast radius (existing `pool update`/`pool apply` tests, help text, downstream scripting) than is appropriate for an atomic fix commit, and the reviewer offered this as an explicit alternative. Added a `WARNING:` clause to the `pool update` route's registered `Summary` (surfaced via `--help`) and expanded `parsePoolMemberSpec`'s doc comment to explicitly state that `--add`'s stable-key/content-hash pair is never decoded, pinned, or cross-checked against a real fixture definition (unlike `pool substitute`), and that verification is the caller's responsibility. No behavior changed; `internal/command/...` and the full suite still pass.

## Skipped Issues

None — all 5 in-scope findings were fixed.

---

_Fixed: 2026-07-22T01:44:21Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
