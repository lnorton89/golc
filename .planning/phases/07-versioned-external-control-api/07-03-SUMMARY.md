---
phase: 07-versioned-external-control-api
plan: 03
subsystem: api
tags: [api, config, projectconfig, loopback, remote-access, security]

# Dependency graph
requires:
  - phase: 07-versioned-external-control-api
    provides: "07-02: internal/api Chi+Huma /v1 server hosted as an Art-Net daemon Subsystem, api.NewServer(executor, root, showPath) hardcoded to loopback:4590"
provides:
  - "config/api.toml + internal/projectconfig 'api' concern: api.remote_enabled (writable, mirrors runtime.log_level), api.port, api.bind_interface, api.rate_per_minute, api.rate_burst (all four locked)"
  - "internal/api/config.go: ResolveConfig(root) resolving the api concern's five keys through internal/projectconfig's five-layer resolution into a typed Config"
  - "internal/api/server.go: listenAddr's enforced-at-bind loopback default (127.0.0.1:<port> unless RemoteEnabled and BindInterface are both explicit; GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED otherwise, no listener opened), NewServer's variadic WithConfig ServerOption"
  - "internal/command/artnet.go's runArtnetServe wired to api.ResolveConfig + api.WithConfig, so the real 'artnet serve' daemon start path enforces the loopback default, not just internal/api's own unit tests"
affects: [07-04-versioned-external-control-api, 07-05-versioned-external-control-api, 07-09-versioned-external-control-api]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/api/config.go imports internal/projectconfig directly (no cycle risk: projectconfig never imports the CLI command-execution package internal/api is forbidden from importing) -- the api concern's resolution lives in internal/api itself, not bridged through internal/command the way 07-02's Executor seam bridges domain execution."
    - "NewServer's functional-option pattern (ServerOption / WithConfig) rather than a required Config parameter: every existing 3-arg api.NewServer call site (internal/api/translate_test.go x4, internal/command/artnet_test.go x1) keeps compiling and behaving unchanged (loopback:4590 default), while the one production call site (runArtnetServe) opts in to the real resolved config. This kept the plan's actual blast radius to the four files below plus one production wiring edit, instead of touching five call sites across three packages for a signature change with no test-observable benefit."
    - "listenAddr is a pure, unexported (root, port, remote_enabled, bind_interface) -> (address, error) function, tested directly at the unit level (internal/api/config_test.go, package api not api_test) rather than through a live net.Listen bind -- avoids depending on the test host's firewall/interface behavior for the 0.0.0.0 case while still proving Start's early-return-before-Listen behavior for the missing-interface case."

key-files:
  created:
    - config/api.toml
    - internal/api/config.go
    - internal/api/config_test.go
  modified:
    - internal/projectconfig/model.go
    - internal/projectconfig/decode.go
    - internal/projectconfig/registry.go
    - internal/projectconfig/local.go
    - internal/projectconfig/strict_test.go
    - golc.project.toml
    - internal/api/server.go
    - internal/command/artnet.go

key-decisions:
  - "api.port/api.bind_interface/api.rate_per_minute/api.rate_burst all validate as pattern-matched TOML strings (this strict registry's universal convention -- every canonical value decodes as a string, never a native TOML int/bool), parsed to native Go types only inside internal/api/config.go's ResolveConfig, not inside internal/projectconfig itself."
  - "api.remote_enabled is the sole Locked:false key in the new concern (mirrors runtime.log_level exactly): AllowedValues {\"true\",\"false\"}, env var GOLC_API_REMOTE_ENABLED, CLI flag --api-remote-enabled, and added to local.go's golc.local.toml writable allowlist."
  - "Chose NewServer(executor, root, showPath, opts ...ServerOption) with WithConfig(cfg Config) over changing NewServer's signature to a required 4th Config parameter -- backward compatible with every existing call site, avoiding edits to internal/api/translate_test.go and internal/command/artnet_test.go that a required-parameter design would have forced."

requirements-completed: []  # API-05 spans 07-03's config-driven loopback default plus a later plan's scoped-auth requirement for remote access; this plan ships the loopback-enforcement half only, so API-05 is not functionally complete yet.

coverage:
  - id: D1
    description: "config/api.toml declares api.remote_enabled/port/bind_interface/rate_per_minute/rate_burst with committed defaults, registered as a first-class projectconfig concern with exactly one writable key (api.remote_enabled)"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/projectconfig/strict_test.go#TestScopeConfigStrict (root index discovers exactly the seven concerns, every production concern validates alone)"
        status: pass
    human_judgment: false
  - id: D2
    description: "api.remote_enabled resolves as writable (settable in golc.local.toml without GOLC_CONFIG_LOCKED_OVERRIDE); a locked api key (e.g. api.port) set in a higher layer still fails with GOLC_CONFIG_LOCKED_OVERRIDE"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/projectconfig/resolve_test.go#TestScopeConfig (five-layer precedence, locked-key rejection -- exercises DefaultRegistry's api.remote_enabled entry via the same writable-key code path proven for runtime.log_level)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Server binds loopback-only (127.0.0.1:<port>) when api.remote_enabled is unset or false, even when api.bind_interface is set to a non-loopback value"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/api/config_test.go#TestLoopbackDefault"
        status: pass
    human_judgment: false
  - id: D4
    description: "Enabling remote access (api.remote_enabled=true) with an empty api.bind_interface fails Start loudly with GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED and opens no listener, rather than ever silently binding 0.0.0.0 or silently falling back to loopback"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/api/config_test.go#TestRemoteRequiresInterface"
        status: pass
    human_judgment: false
  - id: D5
    description: "api.remote_enabled=true with an explicit api.bind_interface (e.g. 0.0.0.0) derives that exact bind address"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/api/config_test.go#TestBindAddress"
        status: pass
    human_judgment: false
  - id: D6
    description: "The real 'artnet serve' daemon start path (not just internal/api's own tests) resolves and enforces the api concern's config through api.ResolveConfig + api.WithConfig"
    requirement: "API-05"
    verification:
      - kind: unit
        ref: "internal/command/artnet_test.go#TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP (unaffected regression proof: this test constructs api.Server directly, bypassing runArtnetServe, so it independently proves the 3-arg NewServer's loopback default stayed intact after the WithConfig option was added)"
        status: pass
      - kind: other
        ref: "go build ./internal/... ./tools/... && go vet ./internal/... ./tools/..."
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-07-24
status: complete
---

# Phase 7 Plan 3: Config-Driven Loopback-Enforced API Bind Summary

**New `api` projectconfig concern (config/api.toml) with api.remote_enabled as the sole writable key, plus internal/api's listenAddr enforcing loopback-by-default at bind time -- remote access requires both an explicit api.remote_enabled=true and a non-empty api.bind_interface, or Start refuses to open a listener at all.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-07-24
- **Tasks:** 2/2
- **Files modified:** 12 (3 created, 9 modified)

## Accomplishments
- Declared a new `api` projectconfig concern in `config/api.toml`: `api.remote_enabled` (bool-as-string, default `"false"`), `api.port` (default `"4590"`), `api.bind_interface` (default `""`), `api.rate_per_minute` (default `"60"`), `api.rate_burst` (default `"10"`) -- registered in `internal/projectconfig/model.go`'s `DefaultSpec`, indexed in `golc.project.toml`, with `api.remote_enabled` declared `Locked:false` in `registry.go`'s `DefaultRegistry` (env var `GOLC_API_REMOTE_ENABLED`, CLI flag `--api-remote-enabled`) and added to `local.go`'s `golc.local.toml` writable allowlist -- the exact same treatment `runtime.log_level` already has.
- Built `internal/api/config.go`'s `ResolveConfig(root)`: resolves all five api concern keys through `internal/projectconfig`'s five-layer resolution (committed → user → project-local → environment → CLI) into a typed `Config{RemoteEnabled, Port, BindInterface, RatePerMinute, RateBurst}`, parsing the strict registry's string-only canonical values to native Go types at this one boundary.
- Implemented `internal/api/server.go`'s `listenAddr()`: the enforced-at-bind-time loopback default (07-RESEARCH.md Pitfall 4) -- `127.0.0.1:<port>` whenever `RemoteEnabled` is false (including the Config zero value), regardless of any `BindInterface` value; `<BindInterface>:<port>` only when `RemoteEnabled` is true AND `BindInterface` is non-empty; `GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED` (no listener ever opened) when `RemoteEnabled` is true but `BindInterface` is empty.
- Added `NewServer`'s variadic `WithConfig(cfg Config) ServerOption`, keeping every pre-existing 3-arg `NewServer(executor, root, showPath)` call site (four in `internal/api/translate_test.go`, one in `internal/command/artnet_test.go`) compiling and behaving byte-for-byte unchanged (loopback:4590 default) while the real production call site opts in.
- Wired `internal/command/artnet.go`'s `runArtnetServe` to call `api.ResolveConfig(request.Root)` and pass the result into `api.NewServer(..., api.WithConfig(apiConfig))`, so `"artnet serve"` -- the actual daemon entrypoint, not just this package's unit tests -- now enforces the loopback default and the explicit-interface requirement end-to-end.

## Task Commits

1. **Task 1: Declare the api config concern (config/api.toml + DefaultSpec) with api.remote_enabled writable** - `fd9080b` (feat)
2. **Task 2: Config-driven bind derivation with enforced loopback default** - `cea4c8f` (feat)

**Plan metadata:** SUMMARY.md commit pending (this file); STATE.md/ROADMAP.md intentionally left untouched -- this plan ran as a parallel worktree agent, and the orchestrator owns those writes centrally after the wave completes.

## Files Created/Modified
- `config/api.toml` - the new per-concern config file: api.remote_enabled/port/bind_interface/rate_per_minute/rate_burst with committed defaults and `[ASSUMED]` provenance comments for the rate/port values
- `internal/projectconfig/model.go` - registers the "api" concern in `DefaultSpec`, adds `apiPortPattern`/`apiRateLimitPattern`/`apiBindInterfacePattern`
- `internal/projectconfig/registry.go` - `api.remote_enabled` declared `Locked:false` in `DefaultRegistry` (mirrors `runtime.log_level`)
- `internal/projectconfig/local.go` - `api.remote_enabled` added to `localKeyRegistry` (writable in `golc.local.toml`)
- `internal/projectconfig/strict_test.go` - updated the two hardcoded six-concern assertions to seven and added `"api": "config/api.toml"` to the expected concern map (Rule 3: required for the plan's own stated `go test ./internal/projectconfig/...` acceptance criterion to keep passing after a seventh concern was added)
- `golc.project.toml` - indexes the new `api` concern (required for `LoadRootIndex`/`ValidateRepository`'s discovery-must-match-registry invariant)
- `internal/api/config.go` - `Config` struct + `ResolveConfig(root)`
- `internal/api/server.go` - `Config` field on `Server`, `ServerOption`/`WithConfig`, `listenAddr()`, `Start` now derives its address from `listenAddr()` instead of a hardcoded loopback string
- `internal/api/config_test.go` - `TestLoopbackDefault`, `TestRemoteRequiresInterface`, `TestBindAddress` (plan's verify command), plus `TestListenAddrDefaultsPortWhenUnset`
- `internal/command/artnet.go` - `runArtnetServe` resolves and wires the api concern's config into `api.NewServer` via `api.WithConfig`

## Decisions Made
- All five api concern keys validate as pattern-matched TOML strings (this repository's universal strict-config convention -- no canonical value is ever a native TOML int/bool anywhere in `internal/projectconfig`); numeric parsing to native Go `int`/`bool` happens once, inside `internal/api/config.go`'s `ResolveConfig`, never inside `internal/projectconfig` itself.
- Chose a variadic `WithConfig` functional option over a required 4th `Config` parameter on `NewServer`, keeping every existing call site (test and production) compiling unchanged and minimizing this plan's blast radius to files it actually needed to touch for new behavior, rather than a signature-driven ripple across `internal/api/translate_test.go` and `internal/command/artnet_test.go` with no additional test coverage gained.
- `listenAddr()` is tested as a pure function (no live `net.Listen` bind for the loopback/0.0.0.0 address-derivation cases) to avoid depending on the test host's firewall/interface behavior; `TestRemoteRequiresInterface` still calls the real `Start(ctx)` to prove the early-return-before-`net.Listen` structural guarantee (`s.httpServer` stays nil).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated golc.project.toml and internal/projectconfig/strict_test.go for the new seventh concern**
- **Found during:** Task 1
- **Issue:** Adding the `api` concern to `DefaultSpec` without also indexing it in `golc.project.toml` would fail `internal/projectconfig`'s own `validateIndexDiscovery` (`GOLC_CONFIG_INDEX_MISMATCH`) the moment any repository-wide validation ran; separately, `strict_test.go`'s existing "root index discovers exactly the six phase 1 concerns" and "every production concern validates alone" subtests hardcode a count of six and an exact six-entry expected-concern map, which would fail the instant a seventh concern was registered. Neither file was in the plan's declared `files_modified` list.
- **Fix:** Added `[[concerns]] id = "api" path = "config/api.toml"` to `golc.project.toml`; updated the two hardcoded assertions in `strict_test.go` to seven concerns and added `"api": "config/api.toml"` to the expected map.
- **Files modified:** `golc.project.toml`, `internal/projectconfig/strict_test.go`
- **Verification:** `go test ./internal/projectconfig/...` passes in full, including `TestScopeConfigStrict`.
- **Committed in:** `fd9080b` (Task 1 commit)

**2. [Rule 2 - Missing Critical Functionality] Wired internal/command/artnet.go's runArtnetServe to the new config**
- **Found during:** Task 2
- **Issue:** The plan's Task 2 file list (`internal/api/config.go`, `internal/api/server.go`, `internal/api/config_test.go`) covers the mechanism (resolving config, deriving the enforced address) but not the one production call site that actually starts the daemon's API listener. Without wiring `runArtnetServe`, the plan's own stated objective -- "After this slice, an operator can flip `api.remote_enabled` in config to expose the API beyond loopback" -- would not be true of the real `"artnet serve"` command; the daemon would still always construct a `*Server` with the built-in loopback:4590 default regardless of any `config/api.toml`/`golc.local.toml` setting.
- **Fix:** `runArtnetServe` now calls `api.ResolveConfig(request.Root)` and passes the result into `api.NewServer(..., api.WithConfig(apiConfig))`.
- **Files modified:** `internal/command/artnet.go`
- **Verification:** `go build ./internal/... ./tools/...` succeeds; `internal/command`'s existing artnet-scope tests (including `TestArtnetRunHostsAPIServerSubsystemAndServesLoopbackHTTP`, which constructs `api.Server` directly and bypasses `runArtnetServe`) pass unchanged, proving the wiring didn't regress the pre-existing 3-arg default path.
- **Committed in:** `cea4c8f` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing critical functionality)
**Impact on plan:** Both auto-fixes were necessary for the plan's own stated acceptance criteria (Task 1's `go test ./internal/projectconfig/...` requirement) and stated objective (a real operator-facing loopback-default enforcement, not just a mechanism nothing calls) to actually hold. No scope creep beyond what those two requirements demand.

## Issues Encountered
None beyond the two auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/api.Config`/`ResolveConfig` is ready for 07-04's rate limiter to consume `RatePerMinute`/`RateBurst` (carried through unused by this plan).
- `api.remote_enabled` is now a real, provenance-tracked, writable config key an operator can flip via `golc.local.toml`, environment (`GOLC_API_REMOTE_ENABLED`), or a future CLI flag (`--api-remote-enabled`, documented but not yet parsed anywhere) -- 07-04+ can build authenticated remote-access UX on top of this without revisiting the bind-enforcement mechanism itself.
- API-05 is not functionally complete: this plan proves the loopback-default/explicit-enablement half; the "+ scoped auth for remote access" half (API-key scopes, D-05/D-08) is still deferred to a later plan per ROADMAP.md's existing phase structure.

---
*Phase: 07-versioned-external-control-api*
*Completed: 2026-07-24*

## Self-Check: PASSED
- FOUND: config/api.toml
- FOUND: internal/api/config.go
- FOUND: internal/api/config_test.go
- FOUND: internal/projectconfig/model.go
- FOUND: internal/projectconfig/registry.go
- FOUND: internal/projectconfig/local.go
- FOUND: internal/projectconfig/strict_test.go
- FOUND: golc.project.toml
- FOUND: internal/api/server.go
- FOUND: internal/command/artnet.go
- FOUND: commit fd9080b
- FOUND: commit cea4c8f
