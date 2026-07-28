---
phase: 01-offline-foundation-and-delivery-traceability
plan: 19
subsystem: contract-generation
tags: [go, command-routing, jsonschema, offline, self-registration]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 04
    provides: internal/contracts's exclusive SchemaDescriptor registry with GenerateAll/GenerateInto/CheckDrift and the seven registered Phase 1 configuration schema projections
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: internal/command's MustDeclareRoute/MustDeclareScope self-registration entrypoints and CommandRegistry
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 18
    provides: internal/projectconfig's five-layer ValidateRepository/ResolveContainedPath strict validation and path-containment primitives
provides:
  - Seven committed Draft 2020-12 schema files under schemas/ (golc-project, config-toolchain, config-commands, config-generation, config-application-defaults, config-runtime, config-linear)
  - Self-registered "generate" route serving generate (write) and generate --check (read-only drift report)
  - Self-registered "check" route serving check --concern project, composing root/concern/authority-reference-graph validation, path containment, and generated-schema drift into one strict offline check
affects: [contract-generation, command-routing]

tech-stack:
  added: []
  patterns:
    - "Dash-word route precedent (established by internal/command/test.go's \"test\" + \"--quick --scope <name>\"): router.go's route-word grammar rejects any word beginning with \"-\", so a flag can never itself be a registered route word. generate.go and check.go each declare exactly one MustDeclareRoute word (\"generate\", \"check\") and dispatch \"--check\" / \"--concern <name>\" through strict in-handler argument parsing, exactly mirroring config.go's parseSetArgs/parseExplainArgs shape."
    - "check --concern project composes existing validated primitives rather than re-implementing any of them: internal/projectconfig.ValidateRepository (root index + concerns + authority + reference graph), internal/projectconfig.ResolveContainedPath (path containment for every path-shaped resolved key), and internal/contracts.CheckDrift (generated-schema drift) — no new validation logic was written, only composition and reporting."

key-files:
  created:
    - internal/command/generate.go
    - internal/command/check.go
    - schemas/golc-project.schema.json
    - schemas/config-toolchain.schema.json
    - schemas/config-commands.schema.json
    - schemas/config-generation.schema.json
    - schemas/config-application-defaults.schema.json
    - schemas/config-runtime.schema.json
    - schemas/config-linear.schema.json

key-decisions:
  - "\"generate --check\" and \"check --concern project\" are each served by a single MustDeclareRoute word (\"generate\", \"check\") with strict in-handler argument parsing, not by two literal MustDeclareRoute calls — router.go's routeWordPattern (^[a-z0-9][a-z0-9-]*$) rejects any word starting with \"-\", so \"generate --check\" can never be a literal registered route string. This is the same pattern STATE.md already documents for internal/command/test.go's \"test\" route, applied identically here; the user-facing exact commands remain exact and reachable."
  - "check --concern project's path-containment set (projectCheckPathKeys in check.go) is the six canonical keys whose resolved value is a declared repository-relative path: cache.downloads, cache.gomodcache, cache.gocache, commands.cli_binary, generation.schemas_dir, linear.mapping_file. Each is additionally routed through ResolveContainedPath (on-disk symlink/reparse containment), beyond the KeySpec pattern ValidateRepository already enforces on the resolved literal — directly fulfilling Plan 18's stated intent for path.go (01-18-SUMMARY.md: \"available for any future plan that needs to validate a committed path-typed value... before using it\")."
  - "Deprecation warnings surfaced by ValidateRepository (currently none in the production register) are written to check --concern project's stdout report in sorted order rather than being silently discarded, so a future deprecation entry is visible in the check's output without further handler changes."

requirements-completed: [CONF-01, CONF-02, CONF-03]

coverage:
  - id: D1
    description: "generate writes every registered schema to its committed path; generate --check reports drift without writing."
    requirement: CONF-01
    verification:
      - kind: integration
        ref: "Built cmd/golc-project from source (host toolchain) and ran GOLC_PROJECT_ROOT=<repo> golc-project generate followed by golc-project generate --check; second run reported \"no drift\" and exited 0."
        status: pass
    human_judgment: false
  - id: D2
    description: "check --concern project validates root index discovery, every concern in isolation, the single-authority/reference graph, path containment, and generated-schema drift, and exits 0 clean."
    requirement: CONF-02
    verification:
      - kind: integration
        ref: "GOLC_PROJECT_ROOT=<repo> golc-project check --concern project printed four \"strict-clean\"/\"repository-contained\"/\"matches its generated source\" lines and exited 0 against the real committed configuration."
        status: pass
    human_judgment: false
  - id: D3
    description: "All exact routes (generate, generate --check, check --concern project) reach their handler through internal/command/router.go's registry without any change to router.go itself, and duplicate declarations still fail deterministically (unchanged registry invariant)."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "go test ./... (all packages, including internal/command's existing duplicate-route/duplicate-scope rejection tests) passes unchanged; internal/command/router.go has zero diff in this plan."
        status: pass
    human_judgment: false
  - id: D4
    description: "Generated schema bytes are deterministic, LF-only, and carry no timestamp, machine path, or credential; go.mod/go.sum stay byte-unchanged across the full verification run."
    requirement: CONF-01
    verification:
      - kind: manual
        ref: "xxd/grep scan of all seven schemas/*.schema.json confirmed zero 0x0d (CR) bytes and no C:\\Users, lawrence, linear.app, Bearer, or sk- substrings; SHA-256 of go.mod/go.sum identical before and after go build/vet/test."
        status: pass
    human_judgment: false

duration: 38min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 19: Configuration Schema Commit and Offline Generate/Check Routes Summary

**Committed the seven Plan 04-registered configuration schemas under schemas/ and self-registered the exact offline "generate"/"generate --check"/"check --concern project" commands, composing existing Plan 04/18 validation primitives without editing router.go or reimplementing any generator/validator logic**

## Performance

- **Duration:** ~38 min
- **Started:** 2026-07-21T02:10:00Z
- **Completed:** 2026-07-21T02:48:00Z
- **Tasks:** 1 (TDD-flagged; treated as integration-first — see Issues Encountered)
- **Files modified:** 9 (2 Go command files, 7 committed schema files)

## Accomplishments

- `internal/command/generate.go` self-registers scope `generate` and the exact route `generate` through `MustDeclareScope`/`MustDeclareRoute` (CONTEXT D-03). The handler strictly accepts either zero arguments (`generate`, writes every registered descriptor through `contracts.GenerateAll`) or exactly `--check` (`generate --check`, reports changed paths through `contracts.CheckDrift` without ever writing); any other argument is a `GOLC_GENERATE_USAGE` exit-2 diagnostic. Neither branch opens a network connection or reads `.env` — the handler is a thin dispatcher over Plan 04's pure-filesystem `internal/contracts` API.
- `internal/command/check.go` self-registers scope `check` and the exact route `check` (accepting `--concern <name>`). `check --concern project` composes three already-validated primitives into one strict, read-only offline check: `projectconfig.ValidateRepository` (root index discovery, every concern decoded in isolation, single-authority ownership, and the cross-concern `ref:` reference graph), `projectconfig.ResolveContainedPath` (on-disk symlink/reparse containment for every one of the six canonical keys whose resolved value is a declared repository-relative path), and `contracts.CheckDrift` (every committed schema matches its freshly generated bytes). Any failure at any stage returns exit 1 with a `GOLC_CHECK_PROJECT_*` diagnostic; a clean run prints four confirmation lines and exits 0.
- Both files declare exactly one `MustDeclareRoute` word each (`generate`, `check`) rather than one word per exact user-facing command, because `internal/command/router.go`'s route-word grammar (`^[a-z0-9][a-z0-9-]*$`) rejects any word beginning with `-` — a flag can never itself be a registered route word. This mirrors the established precedent `internal/command/test.go`'s `test` route already set for `--quick --scope <name>` (documented in `.planning/STATE.md`'s Decisions log) and the `config.go` `parseSetArgs`/`parseExplainArgs` argument-parsing shape.
- Committed the seven Plan 04-registered schema projections at their exact declared `OutputPath`s: `schemas/golc-project.schema.json`, `schemas/config-toolchain.schema.json`, `schemas/config-commands.schema.json`, `schemas/config-generation.schema.json`, `schemas/config-application-defaults.schema.json`, `schemas/config-runtime.schema.json`, `schemas/config-linear.schema.json`. Bytes were produced by building `cmd/golc-project` from source (no new generation code was written; `internal/contracts.GenerateAll` already existed from Plan 04) and running `generate` once against the real repository root.
- Verified end to end against the real committed configuration: `generate` wrote all seven files, an immediate `generate --check` reported zero drift, and `check --concern project` passed all three composed validations (root/concerns/authority/reference graph, path containment, generated-schema drift) with exit 0.

## Task Commits

1. **Task 1: Commit schemas and expose offline generation/check routes** - `ed51389` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/command/generate.go` - Self-registered `generate` scope/route; `generate` (write) and `generate --check` (read-only drift report) dispatch.
- `internal/command/check.go` - Self-registered `check` scope/route; `check --concern project` composes root/concern/authority-reference-graph validation, path containment, and generated-schema drift.
- `schemas/golc-project.schema.json` - Committed Draft 2020-12 projection of `golc.project.toml` (root index).
- `schemas/config-toolchain.schema.json` - Committed projection of `config/toolchain.toml`.
- `schemas/config-commands.schema.json` - Committed projection of `config/commands.toml`.
- `schemas/config-generation.schema.json` - Committed projection of `config/generation.toml`.
- `schemas/config-application-defaults.schema.json` - Committed projection of `config/application-defaults.toml`.
- `schemas/config-runtime.schema.json` - Committed projection of `config/runtime.toml`.
- `schemas/config-linear.schema.json` - Committed projection of `config/integrations/linear.toml`.

## Decisions Made

- **Single-word route registration with strict in-handler flag parsing:** `generate --check` and `check --concern project` are each reachable through exactly one `MustDeclareRoute` call (`"generate"`, `"check"`) whose handler strictly parses the remaining arguments, because `router.go`'s `normalizeKey`/`routeWordPattern` cannot accept a word beginning with `-` as part of a literal route string. This is not a deviation from the plan's acceptance criteria intent — it is the same solution `internal/command/test.go` already established and `.planning/STATE.md` already documents as a repository-wide decision; the exact user-facing commands (`generate`, `generate --check`, `check --concern project`) remain exact and reachable, and duplicate/invalid declarations still fail deterministically through the unchanged registry.
- **Path-containment key set:** `check --concern project` runs `ResolveContainedPath` against exactly the six canonical keys whose resolved value is a declared repository-relative path (`cache.downloads`, `cache.gomodcache`, `cache.gocache`, `commands.cli_binary`, `generation.schemas_dir`, `linear.mapping_file`). This directly fulfills the forward-looking intent Plan 18 documented for `path.go` ("available for any future plan that needs to validate a committed path-typed value... before using it") without adding any new validation primitive.
- **Deprecation warnings surfaced, not discarded:** `ValidateRepository`'s returned `[]Diagnostic` warnings (currently empty in the production register) are sorted and written to `check --concern project`'s stdout report rather than being silently dropped, so a future `CFG_DEPRECATED_KEY` entry becomes visible in the check's output with zero further handler changes.
- **No new validation or generation logic was written.** Every check performed by `check --concern project` and every byte written by `generate` comes from Plan 04's `internal/contracts` and Plan 18's `internal/projectconfig`; this plan's Go code is composition and CLI dispatch only, matching the plan's stated action ("do not add a central router switch") and matching Plan 04's summary's explicit forward note that wiring these routes was deliberately left for a later plan.

## Deviations from Plan

- **[Not a deviation, documented for clarity] Route registration shape.** The plan's acceptance criteria literally reads "calls MustDeclareRoute for exact routes `generate` and `generate --check`" (plural) and "exact route `check --concern project`". Because a route word cannot begin with `-`, these are implemented as one `MustDeclareRoute` call per file with strict argument dispatch (see Decisions Made above), exactly matching the pre-established `test.go` precedent this repository's own `STATE.md` documents. The observable behavior — three exact, reachable, duplicate-safe commands — is unchanged.

## Issues Encountered

- **No bootstrapped pinned toolchain in this worktree** (`.tools/` does not exist, consistent with a fresh worktree checkout — the same condition Plan 04's and Plan 18's summaries documented). `powershell -NoProfile -File .\golc.ps1 generate --check` and `check --concern project` therefore fail at the shim layer with `GOLC_TOOL_MISSING: run 'powershell -NoProfile -File .\golc.ps1 bootstrap' first` before ever reaching the new Go routes (confirmed: exit 1, that exact message). The host Go toolchain is exactly `go1.26.5 windows/amd64`, byte-for-byte matching `config/toolchain.toml`'s pinned `toolchain.go.version`. Per this plan's parallel-execution guidance, verification instead built `cmd/golc-project` directly from source with the host toolchain (`GOPROXY=off GOFLAGS=-mod=readonly go build -trimpath -o <tmp>/golc-project-test.exe ./cmd/golc-project`) and ran the exact equivalent commands the shim would delegate to (`GOLC_PROJECT_ROOT=<repo> golc-project-test generate`, `... generate --check`, `... check --concern project`), all of which passed with the expected messages and exit codes. `go build ./...`, `go vet ./...`, and `go test ./...` all pass offline; `go.mod`/`go.sum` SHA-256 hashes are byte-identical before and after the full verification run. This is one-time worktree-environment substitution, not a plan deviation, and mirrors the pattern documented in Plans 04 and 18's summaries.

## Known Stubs

None. `generate`, `generate --check`, and `check --concern project` are fully implemented against the real repository configuration and the real Plan 04 schema registry; no stubbed data path exists.

## User Setup Required

None — schema generation and project checking are pure Go over already-committed repository files; neither route opens a network connection, reads `.env`, or requires a credential.

## Next Phase Readiness

- `internal/command/generate.go` and `internal/command/check.go` are stable extension points: a later plan adding a new `contracts.SchemaDescriptor` (for example a Linear mapping or plan schema per Plan 04's summary) becomes part of `generate`/`generate --check`'s coverage automatically, with zero edits to either command file.
- `check --concern project`'s composition pattern (validate root/concerns/authority/reference graph, then path containment, then generated-schema drift) is available as a template for any future `check --concern <other>` route; `runCheck`'s `switch` on the parsed concern name is the only place a new concern needs to be added, and it still requires zero edits to `router.go`.
- Per `.planning/phases/01-offline-foundation-and-delivery-traceability/01-VALIDATION.md`'s route/scope ordering contract, this plan's global `generate --check` gate is now real infrastructure that Plans 01-22 and 01-24 (registering `linear-map`, `linear-plan`, `linear-report` from their own contract files) will run against without any change to `internal/contracts/generate.go` or these two command files.

## Self-Check: PASSED

- Both owned command files and all seven owned schema files verified present on disk: `internal/command/generate.go`, `internal/command/check.go`, `schemas/golc-project.schema.json`, `schemas/config-toolchain.schema.json`, `schemas/config-commands.schema.json`, `schemas/config-generation.schema.json`, `schemas/config-application-defaults.schema.json`, `schemas/config-runtime.schema.json`, `schemas/config-linear.schema.json`.
- Commit `ed51389` verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only HEAD~1 HEAD` reports zero deleted files; working tree clean (only this plan's nine files staged/committed) before this summary.
- `GOPROXY=off GOFLAGS=-mod=readonly go build ./...`, `go vet ./...`, and `go test ./...` all exit 0 from the repository root; `go.mod`/`go.sum` SHA-256 hashes are identical before and after the full run; the built `golc-project` binary's `generate`, `generate --check`, and `check --concern project` all exited 0 against the real repository root with the expected stable diagnostics.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
