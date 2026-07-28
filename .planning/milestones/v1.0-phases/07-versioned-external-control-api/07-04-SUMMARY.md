---
phase: 07-versioned-external-control-api
plan: 04
subsystem: auth
tags: [api, auth, api-keys, scopes, rate-limit, security, sqlite, huma, chi]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-02: internal/api Chi+Huma /v1 server, RegisterOperation self-registration seam, Executor interface, translate.go's HTTP->routed-command translation; 07-03: api.Config/ResolveConfig (RatePerMinute/RateBurst carried through unused until now)"
provides:
  - "internal/show api_keys table + apikeys.go: GenerateAPIKey/InsertAPIKey/LookupAPIKeyByPrefix/ListAPIKeys/RevokeAPIKey/IsAPIKeyValid/CompareAPIKeyHash/APIKeyPrefixFromToken/ValidateAPIKeyScopes -- the single storage authority for scoped, expiring, revocable API keys"
  - "internal/command/apikey.go: 'api-key create'/'api-key list'/'api-key revoke' CLI routes -- the bootstrap path for the very first key, and the routes internal/api/keys.go forwards to via the Executor seam"
  - "internal/api/auth.go: AuthMiddleware (every /v1 request authenticated, identical 401 for every failure mode), RequireScope/HasScope/ScopesFromContext/KeyIDFromContext"
  - "internal/api/ratelimit.go: RateLimitMiddleware, per-key golang.org/x/time/rate token bucket"
  - "internal/api/keys.go: POST/GET/DELETE /v1/keys (mint/list/revoke, admin scope required), translating into the same CLI routes"
  - "router.go: humaAPI.UseMiddleware(AuthMiddleware, RateLimitMiddleware) wired ahead of the operation-registration loop, deterministically -- not dependent on cross-file init order"
affects: [07-05-versioned-external-control-api, 07-08-versioned-external-control-api, 07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/api/keys.go's REST key-management operations translate into the CLI's own 'api-key create'/'api-key list'/'api-key revoke' routes through the existing Executor seam (translate.go's Pattern 1), exactly like 'config inspect'/'show inspect' -- REST and CLI share one execution/storage authority (D-01) over the api_keys table; internal/api never calls internal/show's Insert/List/Revoke functions directly for key CRUD."
    - "internal/api/auth.go IS the one place internal/api imports internal/show directly (not through the Executor): API-key authentication is infrastructure this package itself owns, not a translated domain command, and internal/show has no import-cycle risk (it never imports internal/command or internal/api)."
    - "Global Huma middleware (AuthMiddleware/RateLimitMiddleware) is installed via humaAPI.UseMiddleware(...) in buildRouter BEFORE the operationRegistrations loop, not through the RegisterOperation self-registration idiom every other operation file uses -- Huma's huma.Register captures api.Middlewares() by value at registration time (chain.go), so middleware order relative to operation registration is load-bearing and must not depend on cross-file package-init ordering."
    - "API keys are hashed at rest (SHA-256 of a crypto/rand 256-bit token) with a short unhashed lookup prefix for O(1) LookupAPIKeyByPrefix -- never a second sql.Open against the .golc file; apikeys.go reuses schema.go/store.go's existing openStore single-writer machinery."

key-files:
  created:
    - internal/show/apikeys.go
    - internal/show/apikeys_test.go
    - internal/command/apikey.go
    - internal/command/apikey_test.go
    - internal/api/auth.go
    - internal/api/ratelimit.go
    - internal/api/keys.go
    - internal/api/auth_test.go
  modified:
    - internal/show/schema.go
    - internal/api/router.go
    - internal/api/translate_test.go
    - internal/command/artnet_test.go

key-decisions:
  - "internal/api/keys.go forwards POST/GET/DELETE /v1/keys through server.executor.Execute('api-key create'/'api-key list'/'api-key revoke', ...) rather than calling internal/show's key functions directly -- this both satisfies D-01 (CommandRegistry.Execute as the sole execution authority) and makes the coverage-gate registration automatic: RegisterOperation's Route field exactly matches the real CLI route Task 1 added, so coverage_test.go needs no new exclusion entries."
  - "AuthMiddleware/RateLimitMiddleware are wired in router.go's buildRouter via humaAPI.UseMiddleware(...) BEFORE the operationRegistrations loop, not through the RegisterOperation self-registration idiom the plan's own RESEARCH/PATTERNS narrative suggested ('no router.go/server.go edit'). Huma's huma.Register captures api.Middlewares() by value at each operation's own registration time (github.com/danielgtaylor/huma/v2's chain.go), so any middleware added after an operation registers never applies to that operation. Relying on cross-file Go package-init order (auth.go alphabetically precedes translate.go, keys.go, etc.) to guarantee 'auth registers first' would have been an undocumented, fragile accident -- a future file sorting ahead of auth.go could silently bypass authentication on every route it adds. A 2-line router.go edit makes 'every /v1 request is authenticated and rate-limited' a structural guarantee instead. Documented here as a deliberate deviation from the plan's stated file list, justified by T-07-05's threat-model disposition (mitigate, not accept)."
  - "All three /v1/keys REST operations (mint/list/revoke) require the admin scope, not just mint (the plan's acceptance criteria only explicitly tests mint). Key metadata and the ability to revoke another key are themselves security-sensitive administrative surface -- a compromised playback-scoped key must not be able to enumerate or revoke every other key (including admin keys), which would be a privilege-escalation/DoS path the coarse D-08 admin scope exists precisely to gate."
  - "GenerateAPIKey's raw token is 256-bit crypto/rand, base64url encoded (43 chars); the lookup prefix is its leading 8 characters (~48 bits of prefix-collision entropy), stored unhashed alongside the SHA-256 hash. Prefix collisions are handled safely, not by uniqueness constraints: LookupAPIKeyByPrefix returns the oldest matching row and CompareAPIKeyHash's constant-time compare simply rejects a token that doesn't match that row's stored hash -- a false negative in the astronomically unlikely collision case, never a false positive/bypass."

requirements-completed: [API-05]

coverage:
  - id: D1
    description: "api_keys table + internal/show/apikeys.go: crypto/rand 256-bit tokens, SHA-256 hash + short prefix stored (never the raw token past mint-time), expiry/revocation checked server-side per lookup, constant-time hash compare"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/show/apikeys_test.go#TestGenerateAPIKey"
        status: pass
      - kind: unit
        ref: "internal/show/apikeys_test.go#TestAPIKeyInsertAndLookup"
        status: pass
      - kind: unit
        ref: "internal/show/apikeys_test.go#TestAPIKeyExpiryAndRevocation"
        status: pass
    human_judgment: false
  - id: D2
    description: "api-key create/list/revoke CLI routes: raw token printed exactly once at mint, list/revoke never expose the hash or raw token, revoke fails cleanly for an unknown/already-revoked id -- the CLI bootstrap path for the very first key"
    requirement: "API-05"
    verification:
      - kind: integration
        ref: "internal/command/apikey_test.go#TestAPIKeyCreateStoresOnlyHashAndPrefix"
        status: pass
      - kind: integration
        ref: "internal/command/apikey_test.go#TestAPIKeyRevokeMarksRevoked"
        status: pass
      - kind: unit
        ref: "internal/command/apikey_test.go#TestAPIKeyCreateUsageErrors"
        status: pass
    human_judgment: false
  - id: D3
    description: "AuthMiddleware: every /v1 request requires a valid, non-expired, non-revoked API key; missing/unknown/expired/revoked all produce an identical 401 (no prefix-existence leak); a valid key's scopes attach to the request context"
    requirement: "API-05"
    verification:
      - kind: integration
        ref: "internal/api/auth_test.go#TestAuthRejectsMissingUnknownExpiredAndRevokedKeys"
        status: pass
      - kind: integration
        ref: "internal/api/auth_test.go#TestAuthValidKeyProceeds"
        status: pass
    human_judgment: false
  - id: D4
    description: "RateLimitMiddleware: per-key golang.org/x/time/rate token bucket, a key over its own limit gets 429 without affecting an independent key's own bucket"
    requirement: "API-05"
    verification:
      - kind: integration
        ref: "internal/api/auth_test.go#TestRateLimitPerKeyIndependent"
        status: pass
    human_judgment: false
  - id: D5
    description: "POST/GET/DELETE /v1/keys mint/list/revoke through the same internal/show store the CLI uses (single execution authority via the Executor seam); mint requires admin scope (non-admin -> 403); list never exposes a raw token"
    requirement: "API-05"
    verification:
      - kind: integration
        ref: "internal/api/auth_test.go#TestKeysRESTMintRequiresAdminListsAndRevokes"
        status: pass
    human_judgment: false
  - id: D6
    description: "internal/api/{auth,ratelimit,keys}.go never import the CLI command-execution package and never log a raw token or hash; apikeys.go never opens a second SQLite connection (reuses openStore)"
    requirement: "API-05"
    verification:
      - kind: other
        ref: "grep -rn '\"github.com/lnorton89/golc/internal/command\"' internal/api/ (0 matches); grep -n 'sql.Open' internal/show/apikeys.go (0 matches, only a doc-comment mention)"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-07-25
status: complete
---

# Phase 7 Plan 4: Scoped, Expiring API-Key Authentication + Per-Key Rate Limiting Summary

**crypto/rand-generated, SHA-256-hashed API keys (playback/authoring/admin scopes, D-08) minted via CLI bootstrap and REST, enforced on every /v1 request through a global Huma auth middleware wired ahead of route registration, plus an independent per-key golang.org/x/time/rate token bucket.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-25
- **Tasks:** 2/2
- **Files modified:** 12 (8 created, 4 modified)

## Accomplishments
- Added the `api_keys` table (`internal/show/schema.go`) and `internal/show/apikeys.go`: `GenerateAPIKey` (crypto/rand 256-bit token, base64url, short lookup prefix + hex SHA-256 hash), `InsertAPIKey`/`LookupAPIKeyByPrefix`/`ListAPIKeys`/`RevokeAPIKey` through the existing single-writer `openStore` machinery, `IsAPIKeyValid` (revocation + expiry checked server-side on every call), `CompareAPIKeyHash` (constant-time), and `ValidateAPIKeyScopes` (closed `playback`/`authoring`/`admin` set, D-08).
- Added `internal/command/apikey.go`: `api-key create`/`api-key list`/`api-key revoke` CLI routes -- the bootstrap path for the very first key (no daemon or existing key required), printing the raw token exactly once and never exposing the hash.
- Added `internal/api/auth.go`: `AuthMiddleware`, a global Huma middleware validating `Authorization: Bearer <token>` against the api_keys store, returning an identical 401 for every failure mode (missing header, unknown prefix, hash mismatch, expired, revoked -- T-07-05), and attaching the authenticated key's id + scopes to the request context (`RequireScope`/`HasScope`/`ScopesFromContext`/`KeyIDFromContext`).
- Added `internal/api/ratelimit.go`: `RateLimitMiddleware`, a per-key `golang.org/x/time/rate` token bucket sized from the `api` config concern's `RatePerMinute`/`RateBurst` (carried through unused since 07-03), independent per key id.
- Added `internal/api/keys.go`: `POST /v1/keys` (mint, admin scope required), `GET /v1/keys` (list metadata, admin scope required), `DELETE /v1/keys/{id}` (revoke, admin scope required) -- each translating into the exact same `api-key create`/`api-key list`/`api-key revoke` CLI routes via the Executor seam (single execution/storage authority, D-01), which also makes the capability-coverage gate pass automatically with no new exclusion entries.
- Wired `AuthMiddleware`/`RateLimitMiddleware` onto `humaAPI` in `router.go`'s `buildRouter`, deliberately ahead of the `operationRegistrations` loop (a documented deviation from the plan's stated file list -- see Decisions Made) so every `/v1` request is authenticated and rate-limited by construction, not by an accident of cross-file Go init order.
- Fixed the three pre-existing 07-02/07-03 tests (`translate_test.go`'s `TestParity`/`TestEmptyCollection`/`TestShowPathInjection`/`TestTranslateResult`, and `internal/command/artnet_test.go`'s `TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP`) that broke once authentication became mandatory on every request -- each now seeds and presents a real API key.

## Task Commits

1. **Task 1a: RED test for API key store** - `594c8a6` (test)
2. **Task 1b: API key store implementation** - `0bb6a84` (feat)
3. **Task 1c: RED test for api-key CLI routes** - `36ac021` (test)
4. **Task 1d: api-key CLI routes implementation** - `29c5f5c` (feat)
5. **Task 2a: RED test for auth/scope/rate-limit/keys REST** - `26770de` (test)
6. **Task 2b: auth/rate-limit/keys REST implementation + router.go wiring** - `1041670` (feat)
7. **Fix: seed API key in pre-existing artnet HTTP integration test** - `81c3a47` (fix)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `internal/show/schema.go` - added the `api_keys` table (append-only, revoked in place via `revoked_at`) to `createTablesSQL`
- `internal/show/apikeys.go` - the API-key store: generation, hashing, insert/lookup/list/revoke, validity predicate, scope validation
- `internal/show/apikeys_test.go` - `TestGenerateAPIKey`, `TestAPIKeyInsertAndLookup`, `TestAPIKeyExpiryAndRevocation`
- `internal/command/apikey.go` - `api-key create`/`api-key list`/`api-key revoke` CLI routes, JSON and plain-text output
- `internal/command/apikey_test.go` - `TestAPIKeyCreateStoresOnlyHashAndPrefix`, `TestAPIKeyRevokeMarksRevoked`, `TestAPIKeyCreateUsageErrors`
- `internal/api/auth.go` - `AuthMiddleware`, `RequireScope`/`HasScope`/`ScopesFromContext`/`KeyIDFromContext`, `bearerToken`
- `internal/api/ratelimit.go` - `RateLimitMiddleware`, `keyRateLimiter` (per-key `rate.Limiter` map)
- `internal/api/keys.go` - `POST/GET/DELETE /v1/keys` operations, each forwarding through the Executor to the matching CLI route
- `internal/api/auth_test.go` - `TestAuthRejectsMissingUnknownExpiredAndRevokedKeys`, `TestAuthValidKeyProceeds`, `TestRateLimitPerKeyIndependent`, `TestKeysRESTMintRequiresAdminListsAndRevokes`
- `internal/api/router.go` - `buildRouter` now calls `humaAPI.UseMiddleware(AuthMiddleware(...), RateLimitMiddleware(...))` before the operation-registration loop
- `internal/api/translate_test.go` - updated every existing test's server construction to seed and present a valid API key now that auth is mandatory; switched two tests off placeholder non-real filesystem roots (`/repo/root`) to real `t.TempDir()` locations, since `AuthMiddleware`'s key lookup opens a real `.golc` store
- `internal/command/artnet_test.go` - `TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP` now seeds a real key and presents it as a bearer token

## Decisions Made
- `internal/api/keys.go`'s REST key-management operations translate into the CLI's own `api-key create`/`api-key list`/`api-key revoke` routes through the Executor seam (mirrors `config inspect`/`show inspect`'s established pattern) rather than calling `internal/show`'s Insert/List/Revoke functions directly -- keeps D-01's single execution authority intact and makes `RegisterOperation`'s `Route` field exactly match the real CLI route, so the capability-coverage gate is satisfied automatically.
- `AuthMiddleware`/`RateLimitMiddleware` are wired directly in `router.go`'s `buildRouter` via `humaAPI.UseMiddleware(...)`, ahead of the operation-registration loop -- see the Deviations section below for the full rationale (Huma captures middleware by value at each operation's own registration time, so relying on cross-file init order to sequence this correctly would be fragile and undocumented).
- All three `/v1/keys` REST operations (not just mint) require the admin scope: key metadata and revocation are themselves security-sensitive administrative surface.
- The lookup prefix is 8 base64url characters (~48 bits); a theoretical prefix collision fails safe (the constant-time hash compare simply rejects the wrong key), so no uniqueness constraint was added to the `prefix` column.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Wired AuthMiddleware/RateLimitMiddleware in router.go instead of relying on the RegisterOperation self-registration idiom**
- **Found during:** Task 2, while wiring the middleware
- **Issue:** The plan's own RESEARCH/PATTERNS narrative suggested registering the middleware "through 07-02's RegisterOperation seam... so neither router.go nor server.go is edited," implicitly relying on the middleware-registering file (`auth.go`) initializing before every operation-registering file (`translate.go`, `keys.go`) in Go's cross-file package-init order. Verified against `github.com/danielgtaylor/huma/v2`'s `chain.go`: `huma.Register` captures `api.Middlewares()` by value at the moment each operation registers, so any middleware added after an operation's own `Register` call never applies to that operation. This is a real, security-critical correctness gap (T-07-05: auth bypass), not a hypothetical -- a future file sorting alphabetically ahead of `auth.go` (e.g. one added in 07-08/07-09) could silently register a route with no authentication at all, and nothing would catch it mechanically.
- **Fix:** Added two lines to `router.go`'s `buildRouter`: `humaAPI.UseMiddleware(AuthMiddleware(humaAPI, server), RateLimitMiddleware(humaAPI, server))`, called once, deterministically, before the `operationRegistrations` loop.
- **Files modified:** `internal/api/router.go`
- **Verification:** `TestAuthRejectsMissingUnknownExpiredAndRevokedKeys`/`TestAuthValidKeyProceeds`/`TestRateLimitPerKeyIndependent` all pass against the real `buildRouter`-constructed handler; `TestParity`/`TestEmptyCollection`/`TestShowPathInjection`/`TestTranslateResult`/`TestCapabilityCoverage` (07-02's own tests) still pass unchanged in structure (only updated to present a valid key).
- **Committed in:** `1041670` (Task 2 commit)

**2. [Rule 3 - Blocking] Updated translate_test.go and artnet_test.go to present a valid API key**
- **Found during:** Task 2, after wiring AuthMiddleware
- **Issue:** Every pre-existing `internal/api` test that issues an HTTP request against `server.Handler()` (07-02's `translate_test.go`) and the one real end-to-end daemon integration test (`internal/command/artnet_test.go`'s `TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP`) issued requests with no `Authorization` header, since no prior plan required one. Once `AuthMiddleware` applies to every `/v1` request, these tests began failing with 401 -- not because their own logic (translation, exit-code mapping, parity, daemon lifecycle) regressed, but as a direct, expected consequence of this task's own change. The plan's own acceptance criteria require `go build ./...` and `mage testquick` green.
- **Fix:** `doGet` (translate_test.go) now takes and presents a bearer token; every server construction site seeds a real API key via `seedAPIKey` (defined in `auth_test.go`, same `api_test` package) against a real root/showPath. Two tests (`TestShowPathInjection`, `TestTranslateResult`) previously used placeholder non-filesystem paths (`/repo/root`) since they only needed a canned `stubExecutor` -- these were switched to real `t.TempDir()` locations, since `AuthMiddleware`'s key lookup now needs a real `.golc` SQLite store regardless of which `Executor` is otherwise stubbed.
- **Files modified:** `internal/api/translate_test.go`, `internal/command/artnet_test.go`
- **Verification:** `go test ./internal/api/... ./internal/command/... ./internal/show/...` all green (excluding pre-existing, unrelated `mage Bootstrap`-dependent failures already present before this plan -- see Issues Encountered).
- **Committed in:** `1041670` (translate_test.go, part of Task 2 commit), `81c3a47` (artnet_test.go, separate fix commit)

**3. [Rule 2 - Missing Critical Functionality] Added `show.APIKeyPrefixFromToken`, exported from `internal/show`**
- **Found during:** Task 2, while writing `auth.go`'s prefix derivation
- **Issue:** The plan's key link ("`internal/api/auth.go -> internal/show.LookupAPIKeyByPrefix(root, showPath, prefix)`") requires `auth.go` to derive the same lookup prefix `GenerateAPIKey` computed at mint time. Deriving it independently in `internal/api` would have required hardcoding the prefix length as a second, drift-prone copy of an `internal/show`-internal constant.
- **Fix:** Added `show.APIKeyPrefixFromToken(token string) string`, the single source of truth both `GenerateAPIKey` (refactored to call it) and `auth.go` now use.
- **Files modified:** `internal/show/apikeys.go`
- **Verification:** `TestGenerateAPIKey` continues to pass; `auth_test.go`'s `TestAuthValidKeyProceeds` proves a token generated by `GenerateAPIKey` is correctly looked up by `AuthMiddleware` via this same derivation.
- **Committed in:** `26770de` (added alongside the Task 2 RED test, since the test exercises the full round trip)

---

**Total deviations:** 3 auto-fixed (1 missing critical functionality -- middleware wiring, 1 blocking -- pre-existing test fixes, 1 missing critical functionality -- prefix helper)
**Impact on plan:** All three were necessary for the plan's own stated correctness requirements (T-07-05's "auth bypass" threat disposition is `mitigate`, not `accept`) and stated acceptance criteria (`go build ./...` green). No scope creep beyond what those requirements demand; the router.go edit is two lines, and the test fixes only add authentication credentials to already-correct assertions.

## Issues Encountered
- `internal/trace/catalog`'s `TestScopeLinearMap/real_repository_seed_migrates_end_to_end_offline` fails in this worktree both before and after this plan's changes (`GOLC_MIGRATE_DRIFT` against `.planning/linear-map.json`) -- confirmed pre-existing and unrelated (verified via `git stash`/re-run against the pre-plan commit) by running the test at the base commit; not touched by this plan.
- `internal/command`'s `TestBuildRouteCompilesTheProductionRepository`, `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`, `TestScopeGreenSubprocess`, `TestScopeOfflineAcceptance` all fail in this worktree with `GOLC_TEST_TOOLCHAIN_MISSING`/`pinned golc-project binary not built` -- this worktree has not run `mage Bootstrap`, a pre-existing environment limitation unrelated to this plan's code changes (confirmed unrelated to `api-key`/auth work by inspection: none reference `internal/api`, `internal/show/apikeys.go`, or `internal/command/apikey.go`).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Every `/v1` request now requires a valid, scoped API key; a compromised or misbehaving key can be revoked (`api-key revoke` / `DELETE /v1/keys/{id}`) or is independently rate-limited (`RateLimitMiddleware`) without affecting other keys.
- `RequireScope`/`HasScope`/`ScopesFromContext` are ready for 07-05's mutation-gating work to consume directly -- no further auth-layer plumbing needed.
- `internal/api/router.go`'s `buildRouter` now installs global middleware deterministically ahead of operation registration; any later plan adding a new `RegisterOperation` file (07-05 through 07-09) automatically inherits authentication and rate limiting with zero additional wiring.
- API-05 is now functionally complete: 07-03 shipped the loopback-default/explicit-enablement half, and this plan ships "+ scoped authentication for remote access" (D-05/D-08) -- both requirement halves are proven.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-25*

## Self-Check: PASSED
- FOUND: internal/show/apikeys.go
- FOUND: internal/show/apikeys_test.go
- FOUND: internal/command/apikey.go
- FOUND: internal/command/apikey_test.go
- FOUND: internal/api/auth.go
- FOUND: internal/api/ratelimit.go
- FOUND: internal/api/keys.go
- FOUND: internal/api/auth_test.go
- FOUND: internal/show/schema.go (modified)
- FOUND: internal/api/router.go (modified)
- FOUND: internal/api/translate_test.go (modified)
- FOUND: internal/command/artnet_test.go (modified)
- FOUND: commit 594c8a6
- FOUND: commit 0bb6a84
- FOUND: commit 36ac021
- FOUND: commit 29c5f5c
- FOUND: commit 26770de
- FOUND: commit 1041670
- FOUND: commit 81c3a47
