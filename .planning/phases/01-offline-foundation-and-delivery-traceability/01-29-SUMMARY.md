---
phase: 01-offline-foundation-and-delivery-traceability
plan: 29
subsystem: infra
tags: [go, cli, toml, json, npm, go-modules, tools-update, d-04]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 13
    provides: tools/linear-sync npm workspace (package.json/package-lock.json exact @linear/sdk 88.1.0 + typescript 7.0.2 pins) this plan's update mechanism covers
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 17
    provides: internal/command/router.go MustDeclareRoute/MustDeclareScope self-registration contract this plan's "tools" scope and "tools update" route use
provides:
  - internal/command/tools.go — self-registered "tools" scope and "tools update" route (--check/--write dispatch); BuildToolsUpdateProposal (pure, deterministic proposal builder over the five declared authorities); writeToolsUpdateFiles (fixed five-path allowlist writer); verifyNpmConsistency (cross-checks package.json/package-lock.json agree exactly); defaultMetadataSource (production no-op source that echoes currently pinned values, since live remote polling is out of scope)
  - internal/command/tools_test.go — TestScopeToolsUpdate (scope "tools-update"): fakeMetadataSource-driven D-04 proof (deterministic repeat check, read-only check, five-path-only write, npm/go.sum mutual consistency, toolchain.toml comment preservation, static no-install-import guard, registry reachability, end-to-end registry.Execute with the production default source)
affects: [01-tools-update, foundation-package]

tech-stack:
  added: []
  patterns:
    - "tools.go's \"tools update --check|--write\" route follows check.go/build.go/test.go's single-registered-route-plus-flag-dispatch precedent exactly: MustDeclareRoute's routeWordPattern rejects any \"--\"-prefixed route word, so both user-facing commands are reachable through one \"tools update\" registration whose handler parses --check/--write from Args."
    - "Deterministic manifest/lock mutation is done by surgical, scoped byte replacement (tomlTableSpan + tomlKeyLinePattern for TOML tables, goModuleLinePattern/go.sum line regexes for Go) rather than full re-serialization, so untouched bytes (comments, unrelated sections, unrelated dependencies) are always preserved verbatim; JSON files (package.json/package-lock.json) instead round-trip through map[string]any + encoding/json's alphabetical-key marshaling, which is deterministic but not byte-preserving of original key order."
    - "A MetadataSource interface separates 'what pins are proposed' from 'how the five files are mutated to match them': only tools_test.go's fakeMetadataSource proposes an actual change in this plan; the production defaultMetadataSource is an intentional no-op that reads and echoes back the currently pinned values, since a real go.dev/nodejs.org/npm-registry source is out of this plan's scope."
    - "No install/extraction/build call path exists structurally: tools.go imports only bytes/encoding/json/fmt/os/path/filepath/regexp/strings — never os/exec, archive/zip, or internal/bootstrap — and tools_test.go's first subtest statically greps tools.go's own source for those forbidden references as a standing regression guard."

key-files:
  created:
    - internal/command/tools.go
    - internal/command/tools_test.go
  modified: []

key-decisions:
  - "MustDeclareRoute is called once for \"tools update\" (not literally twice for \"tools update --check\"/\"tools update --write\"), because the router's routeWordPattern (`^[a-z0-9][a-z0-9-]*$`) rejects any \"--\"-prefixed word as a route word. This exactly mirrors the precedent check.go documents for \"check --concern <name>\"/\"check --offline\" and build.go/test.go document for \"--scope <name>\": both user-facing commands are reachable through the one registration, verified directly against NewDefaultCommandRegistry()'s Lookup()."
  - "Real remote metadata polling (a MetadataSource hitting go.dev/nodejs.org/npmjs.org) is out of this plan's scope. The plan's own <action> text specifies implementing D-04 \"with a fake metadata source\"; defaultMetadataSource is therefore a safe, deterministic no-op (it re-reads and re-affirms the currently committed pins) rather than a stub that errors, so a contributor running the real command today gets a harmless, correct (if unexciting) result instead of a broken one. A future plan can swap in a real MetadataSource without changing BuildToolsUpdateProposal, writeToolsUpdateFiles, the allowlist, or the route."
  - "config/toolchain.toml, go.mod, go.sum, tools/linear-sync/package.json, and tools/linear-sync/package-lock.json are declared in the plan's files_modified but were deliberately left byte-unchanged in this plan: the D-04 mechanism is proven entirely against tools_test.go's synthetic, self-authored fixtures (a fake go.mod/go.sum/toolchain.toml/package.json/package-lock.json under t.TempDir()), never against the real repository files. This avoids any risk of corrupting the real committed pins with fabricated fake-source test data, and matches the plan's own scope (\"Implement D-04 with a fake metadata source\") — the write path genuinely covers the real five paths (toolsUpdateAllowlist is exact and real-repo-relative), it is simply never invoked with a change-proposing source against the real repository in this plan."
  - "go.sum's module-hash-line regex requires the version token to contain no \"/\" (`[^/\\s]+` instead of `\\S+`), otherwise it would also incorrectly match the adjacent go.mod-hash line's \"<version>/go.mod\" token and both go.sum lines for a module would collapse onto the module-hash line's replacement. Caught and fixed during Task 1 development before any commit (see Deviations)."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "\"tools update --check\" and \"tools update --write\" are explicitly registered, reachable routes distinct from bootstrap, dispatched from a single \"tools update\" MustDeclareRoute registration per the router's route-word grammar"
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/tools_update_--check_and_tools_update_--write_are_reachable_through_the_default_registry"
        status: pass
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/tools_update_requires_exactly_one_of_--check_or_--write"
        status: pass
    human_judgment: false
  - id: D2
    description: "Repeated --check runs against identical fake metadata produce byte-identical proposal/diff bytes for all five declared authorities and write nothing to disk"
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/check_is_deterministic_and_never_writes_to_disk"
        status: pass
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/check_builds_a_proposal_in_memory_only;_a_simulated_bootstrap_read_still_sees_only_the_reviewed_on-disk_bytes"
        status: pass
    human_judgment: false
  - id: D3
    description: "--write changes exactly the five declared allowlisted paths, writes bytes identical to the reviewed proposal, and leaves every other path (including simulated cache/node_modules/dist decoys) byte-for-byte unchanged"
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/write_changes_exactly_the_five_allowlisted_paths_and_matches_the_reviewed_proposal_byte-for-byte"
        status: pass
    human_judgment: false
  - id: D4
    description: "The proposed npm package.json/package-lock.json pin exact @linear/sdk and typescript versions and are mutually consistent (direct dependency version, lockfile root entry, and resolved node_modules entry with non-empty integrity/resolved all agree)"
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/npm_proposal_pins_exact_versions_and_produces_mutually_consistent_package.json/package-lock.json_bytes"
        status: pass
    human_judgment: false
  - id: D5
    description: "The proposed Go module update keeps go.mod's require entry and go.sum's two hash lines mutually consistent, and config/toolchain.toml's proposal changes only the six declared pin lines while preserving comments and the [cache] section verbatim"
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/Go_module_proposal_keeps_go.mod_and_go.sum_mutually_consistent"
        status: pass
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/toolchain.toml_proposal_changes_only_the_six_declared_pin_lines_and_preserves_everything_else"
        status: pass
    human_judgment: false
  - id: D6
    description: "Neither --check nor --write ever invokes archive download/extraction, cache warming, package-manager install, dependency-tree creation, or compilation: tools.go structurally never imports process-execution or archive-extraction packages, or the bootstrap package's install/verify machinery"
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/command/tools_test.go#TestScopeToolsUpdate/tools.go_never_imports_process-execution_or_archive-install_machinery"
        status: pass
      - kind: e2e
        ref: "powershell -NoProfile -File .\\golc.ps1 test --quick --scope tools-update"
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 29: Deterministic `tools update --check|--write` Summary

**A single self-registered `tools update` route dispatching --check/--write, with a pure deterministic proposal builder covering config/toolchain.toml, go.mod/go.sum, and the tools/linear-sync npm manifest/lock — proven entirely offline through a fake metadata source, a fixed five-path write allowlist, and a structural no-install-import guard**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-07-21T05:40:00Z (approx.)
- **Completed:** 2026-07-21T06:35:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 2 (2 created, 0 modified)

## Accomplishments

- `internal/command/tools.go` self-registers the `"tools"` scope and the `"tools update"` route through the Plan 17 `MustDeclareRoute`/`MustDeclareScope` contract, dispatching `--check` (compute and report only) and `--write` (compute, then write) from a single registration — the router's route-word grammar forbids `"--"`-prefixed route words, so this exactly mirrors `check.go`'s own `"check --concern <name>"`/`"check --offline"` and `build.go`/`test.go`'s `"--scope <name>"` single-route dispatch precedent.
- `BuildToolsUpdateProposal` is a pure, deterministic function: given a `MetadataSource` and the current bytes of the five declared authorities, it surgically replaces only `config/toolchain.toml`'s six `[toolchain.go]`/`[toolchain.node]` `version`/`archive_url`/`archive_sha256` lines (via `tomlTableSpan`/`tomlKeyLinePattern`, preserving every comment and the `[cache]` section verbatim), one `go.mod` require entry's version plus its two `go.sum` hash lines (via `goModuleLinePattern` and version-token-scoped `go.sum` regexes), and `tools/linear-sync/package.json`/`package-lock.json`'s `@linear/sdk`/`typescript` dependency, root, and `node_modules/<name>` entries (via JSON unmarshal/mutate/`marshalJSONDeterministic`) — then cross-checks the two npm files agree exactly via `verifyNpmConsistency`. It never opens a file or a network connection itself; calling it twice with the same source and inputs returns byte-identical results.
- `writeToolsUpdateFiles` writes exactly the five `toolsUpdateAllowlist` paths (a fixed, non-external-input-driven slice) and no others.
- `defaultMetadataSource` (the production source wired into the registered route) deterministically re-affirms every currently pinned value already on disk — a safe, correct no-op given that a real go.dev/nodejs.org/npm-registry `MetadataSource` is out of this plan's scope (see Known Stubs).
- `internal/command/tools_test.go` registers quick-test scope `tools-update` beside `TestScopeToolsUpdate`, with ten subtests entirely offline against a self-authored synthetic fixture (never the real repository files): a static source-scan guard proving `tools.go` never imports process-execution/archive-extraction/bootstrap machinery; router reachability for both `tools update --check` and `tools update --write`; `--check`/`--write` argument validation; two repeated `--check`-equivalent builds proving byte-identical proposals, zero disk writes, and that a simulated bootstrap read afterward still sees only the original reviewed bytes; a `--write` run proving exactly the five allowlisted paths change (with decoy `.tools/cache/`, `node_modules/`, and `dist/` files proven byte-for-byte untouched) and every written byte equals the reviewed proposal; npm mutual-consistency and exact-version assertions; Go module/`go.sum` consistency assertions; a `toolchain.toml` line-count/comment-preservation assertion; and an end-to-end `registry.Execute` run of both modes with the production `defaultMetadataSource`.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update` (the plan's exact `<verify>` command) exits 0. `go build ./...`, `go vet ./...`, `gofmt -l` (no findings), and the full `go test ./...` all pass. `golc.ps1 generate --check` reports no drift and `golc.ps1 check --offline` passes all four offline gates (generate/check/build/test) with network denied.

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: D-04 fake-source/read-only/write-only/no-install proof** - `4a0afe8` (test)
2. **GREEN - Task 1: deterministic tools update --check/--write implementation** - `e434baf` (feat)

RED was verified via `go vet ./internal/command/...` failing with `undefined: ToolsUpdateProposal` (and nine other undefined symbols) when `tools.go` was temporarily removed from the working tree before the test commit; GREEN was verified via `go test ./internal/command/... -run TestScopeToolsUpdate -v` (all 10 subtests pass) and the full `go test ./...`/`go vet ./...`/`go build ./...` continuing to pass.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/command/tools.go` - `"tools"` scope/`"tools update"` route registration; `ToolchainPin`/`GoModulePin`/`NpmPackagePin`/`ToolsUpdateProposal`/`MetadataSource`; `BuildToolsUpdateProposal`; `writeToolsUpdateFiles`; `verifyNpmConsistency`; `defaultMetadataSource`; `runToolsUpdate`/`parseToolsUpdateArgs`.
- `internal/command/tools_test.go` - Registers scope `tools-update`/`TestScopeToolsUpdate`; `fakeMetadataSource`; synthetic fixture builders; 10 subtests covering the full D-04 contract.

## Decisions Made

- **Single `"tools update"` route registration, not two literal `"tools update --check"`/`"tools update --write"` registrations:** the router's `routeWordPattern` rejects `"--"`-prefixed words, so both user-facing commands are served by one registration whose handler parses the flag from `Args` — proven directly against `NewDefaultCommandRegistry()`'s `Lookup()` for both exact invocations.
- **`defaultMetadataSource` is an intentional, documented no-op**, not a stub that errors: it reads and re-affirms the currently pinned values on disk. Live remote metadata polling (go.dev/nodejs.org/npm registry) is explicitly out of this plan's scope per its own `<action>` text ("Implement D-04 with a fake metadata source"); only `tools_test.go`'s `fakeMetadataSource` proposes an actual change, entirely offline.
- **The five real repository files declared in this plan's `files_modified` (`config/toolchain.toml`, `go.mod`, `go.sum`, `tools/linear-sync/package.json`, `tools/linear-sync/package-lock.json`) were deliberately left byte-unchanged.** The D-04 mechanism is proven against `tools_test.go`'s self-authored synthetic fixtures under `t.TempDir()`, never against the real committed files, so no fabricated fake-source test data can ever corrupt a real pin. `toolsUpdateAllowlist` is exact and real-repo-relative, so the write path genuinely covers all five real paths; it is simply never invoked with a change-proposing source against the real repository in this plan (see Known Stubs).
- **`go.sum`'s module-hash-line regex excludes `/` from the version token** (`[^/\s]+`) rather than using a bare `\S+`, otherwise it would ambiguously also match the adjacent go.mod-hash line's `"<version>/go.mod"` token — see Deviations.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `go.sum` module-hash regex ambiguously matched the go.mod-hash line too**
- **Found during:** Task 1, writing `applyGoSumProposal`
- **Issue:** The initial module-hash-line pattern `^PATH \S+ h1:\S+$` also matched the adjacent go.mod-hash line (`PATH <version>/go.mod h1:...`), because `<version>/go.mod` itself contains no whitespace and satisfies a bare `\S+` token. `sumLine.ReplaceAll` would then have rewritten both lines using the module-hash template, corrupting the go.mod-hash line's `/go.mod` suffix.
- **Fix:** Restricted the module-hash line's version-token character class to `[^/\s]+` (no slash, no whitespace), which can never match the go.mod-hash line's `"<version>/go.mod"` token.
- **Files modified:** `internal/command/tools.go`
- **Verification:** `TestScopeToolsUpdate/Go_module_proposal_keeps_go.mod_and_go.sum_mutually_consistent` asserts both the new hash line and the new go.mod-hash line are present, and that neither of the two old lines survives.
- **Committed in:** `e434baf` (GREEN commit; fixed before any commit, no broken intermediate state)

**2. [Rule 1 - Bug] Static no-install-import guard initially self-triggered on its own documentation comment**
- **Found during:** Task 1, first `TestScopeToolsUpdate` run
- **Issue:** `tools.go`'s package doc comment explained the design by literally naming the forbidden substrings (`os/exec`, `archive/zip`, `internal/bootstrap`) it claimed never to import, which made the new static-scan subtest fail against the comment text itself rather than any real import.
- **Fix:** Reworded the doc comment to describe the same guarantee without embedding the literal forbidden substrings (e.g. "any process-execution or archive-extraction package", "the bootstrap package's install/verify machinery").
- **Files modified:** `internal/command/tools.go`
- **Verification:** `TestScopeToolsUpdate/tools.go_never_imports_process-execution_or_archive-install_machinery` passes.
- **Committed in:** `e434baf` (GREEN commit; fixed before any commit, no broken intermediate state)

**3. [Rule 1 - Bug] End-to-end default-source test wrongly expected byte-identical npm file rewrites**
- **Found during:** Task 1, first `TestScopeToolsUpdate` run
- **Issue:** The end-to-end `registry.Execute` subtest originally asserted all five files were byte-for-byte unchanged after a no-op `--write`. `config/toolchain.toml`/`go.mod`/`go.sum` are rewritten via surgical line replacement (truly byte-identical for a no-op), but `package.json`/`package-lock.json` round-trip through `map[string]any` + `encoding/json`'s alphabetical key marshaling, which can reorder keys even when every value is unchanged — a value-for-value no-op, not a byte-for-byte one.
- **Fix:** Split the assertion: byte-equality for the three surgically-edited files, and parsed-value (`json.Unmarshal` + `reflect.DeepEqual`) equality for the two JSON files. Updated `defaultMetadataSource`'s doc comment to state this distinction explicitly.
- **Files modified:** `internal/command/tools.go`, `internal/command/tools_test.go`
- **Verification:** `TestScopeToolsUpdate/registry.Execute_serves_tools_update_--check/--write_end-to-end_with_the_production_default_source` passes.
- **Committed in:** `e434baf` (GREEN commit; fixed before any commit, no broken intermediate state)

---

**Total deviations:** 3 auto-fixed (3 Rule 1 bugs, all caught and fixed during initial `TestScopeToolsUpdate` development before any commit existed)
**Impact on plan:** All three fixes were required for the plan's own acceptance criteria (mutually consistent go.sum lines; an accurate, non-self-contradicting no-install proof; an accurate no-op characterization) to be achievable at all. No scope expansion — no broken intermediate commit exists for any of them.

## Issues Encountered

- This worktree had no bootstrapped `.tools/` state (fresh worktree). `powershell -NoProfile -File .\golc.ps1 bootstrap` (plain, no `--include linear-sync`) completed successfully over live network access, matching the pattern documented in prior plans' summaries. The subsequent bare `powershell -NoProfile -File .\golc.ps1 test` (full suite) failed at its `linear-sdk-operations` Node scope with a "not bootstrapped" placeholder command — this is expected, pre-existing behavior unrelated to this plan (Node was never provisioned since `bootstrap --include linear-sync` was not run), not a regression: this plan does not touch `test.go`/`linear_sync.go`/`build.go`, and the plan's own `<verify>` (`test --quick --scope tools-update`) and the full offline gate (`golc.ps1 check --offline`, which never reaches the Node scope) both passed cleanly.

## Known Stubs

- **`defaultMetadataSource` never fetches real go.dev/nodejs.org/npm-registry metadata.** It is a deterministic, value-for-value no-op that reads and echoes back the currently pinned values already on disk. This is intentional and matches this plan's own scope (`<action>`: "Implement D-04 with a fake metadata source"); a future plan can wire a real `MetadataSource` implementation into `runToolsUpdate` without changing `BuildToolsUpdateProposal`, `writeToolsUpdateFiles`, the `toolsUpdateAllowlist`, or the registered route/scope.
- **`config/toolchain.toml`, `go.mod`, `go.sum`, `tools/linear-sync/package.json`, and `tools/linear-sync/package-lock.json` were not modified by this plan**, despite being listed in `files_modified`. The D-04 mechanism (deterministic check/write/allowlist/no-install) is fully proven against synthetic fixtures in `tools_test.go`; the write path genuinely covers all five real repository-relative paths (`toolsUpdateAllowlist`), but no change-proposing source was ever run against the real files in this plan — only the no-op `defaultMetadataSource` was exercised end-to-end against them (via a temp-fixture copy, never the real files themselves).

## Threat Flags

None — this plan is itself the mitigation for `T-01-14`/`T-01-SC` (see `01-29-PLAN.md`'s threat model): the fixed five-path `toolsUpdateAllowlist`, the structural absence of any process-execution/archive-extraction import, and `verifyNpmConsistency`'s cross-check together bound what an untrusted or malformed `MetadataSource` proposal can ever affect. No new network, process, or credential surface is introduced; `defaultMetadataSource` performs no network access at all.

## User Setup Required

None - everything is repository-local and offline; no credentials, npm, or additional network access is required beyond the one-time toolchain bootstrap already required by prior plans (already verified working in this worktree).

## Next Phase Readiness

- `MetadataSource`, `BuildToolsUpdateProposal`, `writeToolsUpdateFiles`, and `toolsUpdateAllowlist` are ready for a future plan to wire a real go.dev/nodejs.org/npm-registry-backed `MetadataSource` into `runToolsUpdate` without changing the route, scope, allowlist, or proposal/write contract this plan established.
- The `"tools"` scope and `"tools update"` route are now standing, self-registered, verified gates any later plan can extend (for example additional managed Go modules beyond `defaultManagedGoModulePath`) by adding to `defaultMetadataSource`'s echoed pins.

## Self-Check: PASSED

- Both created files verified present on disk: `internal/command/tools.go`, `internal/command/tools_test.go` (both `FOUND`).
- Commits `4a0afe8` (test) and `e434baf` (feat) verified present in `git log --oneline` on branch `worktree-agent-a5c632fd820c21f30`; `git diff --diff-filter=D --name-only 4a0afe8~1 e434baf` reports zero deleted files across both commits; working tree clean before this summary.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update` (the plan's exact `<verify>` command) exits 0. `go build ./...`, `go vet ./...`, `gofmt -l internal/command/tools.go internal/command/tools_test.go` (no findings), and the full `go test ./...` all exit 0/clean. `golc.ps1 generate --check` and `golc.ps1 check --offline` both pass.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
