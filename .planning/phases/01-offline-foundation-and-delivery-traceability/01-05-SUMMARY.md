---
phase: 01-offline-foundation-and-delivery-traceability
plan: 05
subsystem: infra
tags: [go, bootstrap, archive-zip, sha256, checksum, atomic-install, toolchain-toml]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 18
    provides: Five-layer configuration resolution and the projectconfig strict single-authority decoder that this plan's config/toolchain.toml change had to stay compatible with
provides:
  - internal/bootstrap/archive.go — VerifySHA256, InspectZipEntries (traversal + symlink rejection), ExtractVerified (staging-only extraction), and PromoteAtomically (single-rename promotion exposing the complete tree or nothing)
  - internal/bootstrap/downloader.go — OfficialSourcePolicy (committed host/path allowlist from config/toolchain.toml), Source/HTTPSource acquisition abstraction, AcquireStaged, and AcquireAndPromote (full acquire -> verify -> extract -> promote flow)
  - config/toolchain.toml toolchain.go.official_host/official_path_prefix pins consumed by OfficialSourcePolicy
  - Registered bootstrap-archive quick-test scope (TestScopeBootstrapArchive), exercised via `golc.ps1 test --quick --scope bootstrap-archive`
affects: [01-06, bootstrap, tools-update]

tech-stack:
  added: []
  patterns:
    - "OfficialSourcePolicy reads toolchain.<name>.official_host/official_path_prefix as flat scalar keys (not a TOML array-of-tables) so the existing projectconfig strict single-authority decoder — which only supports string-valued canonical keys — keeps validating every toolchain.toml key without adding array-valued key support"
    - "archive.go decomposes the D-01 boundary into four small functions (VerifySHA256, InspectZipEntries, ExtractVerified, PromoteAtomically) reusing bootstrap.go's private helpers (hashFile, checkEntryName, normalizeExpectedSHA256, extractEntry) from the same package instead of duplicating them"
    - "downloader.go's Source interface separates acquisition transport from policy/verification so every test injects a fake payload source; the only production Source (HTTPSource) is never constructed by a test"
    - "PromoteAtomically creates the install directory's parent before renaming, so a caller only needs a staging parent, not the eventual install-tree parent, to already exist"

key-files:
  created:
    - internal/bootstrap/archive.go
    - internal/bootstrap/downloader.go
  modified:
    - config/toolchain.toml
    - internal/bootstrap/bootstrap_test.go
    - internal/projectconfig/model.go

key-decisions:
  - "Represented the official-source allowlist as flat scalar keys (toolchain.go.official_host/official_path_prefix) instead of a [[official_sources]] table array — TOML array-of-tables flattens to a single non-string value under projectconfig's strict decoder, which only validates string-valued canonical keys, so an array would have broken every existing config-strict test against the real committed config/toolchain.toml."
  - "Registered the two new toolchain.go keys in internal/projectconfig/model.go's DefaultSpec even though model.go was not in this plan's files_modified — leaving them undeclared made TestScopeConfigStrict's production-repository validation fail with GOLC_CONFIG_UNKNOWN_KEY (Rule 3: blocking issue caused by this plan's own required config/toolchain.toml change)."
  - "InspectZipEntries detects unsafe symlink entries via archive/zip's stdlib Mode()/SetMode() round-trip (Unix external-attribute mode bits) rather than a custom bit-mask check, matching the already-proven, well-tested stdlib decode path."
  - "AcquireAndPromote composes AcquireStaged + ExtractVerified + PromoteAtomically as one convenience entrypoint satisfying the plan's <behavior> retry contract, while each building block also stays independently callable and independently tested."

patterns-established:
  - "BOOTSTRAP_SOURCE_NOT_ALLOWLISTED / BOOTSTRAP_SOURCE_SCHEME / BOOTSTRAP_SOURCE_INVALID_URL: stable diagnostics for OfficialSourcePolicy rejections, checked before any Source.Fetch call."
  - "BOOTSTRAP_ARCHIVE_UNSAFE_LINK: stable diagnostic for a symlink zip entry, alongside the existing BOOTSTRAP_ARCHIVE_TRAVERSAL/BOOTSTRAP_CHECKSUM_MISMATCH/BOOTSTRAP_CHECKSUM_FORMAT codes from bootstrap.go."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "OfficialSourcePolicy accepts only the committed official host/path patterns from config/toolchain.toml and rejects other hosts, look-alike subdomains, other paths, non-https schemes, and malformed URLs"
    requirement: "CONF-01"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/the_committed_config/toolchain.toml_pins_exactly_the_official_go.dev_source"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/OfficialSourcePolicy_accepts_only_the_committed_official_host/path_patterns"
        status: pass
    human_judgment: false
  - id: D2
    description: "VerifySHA256 and InspectZipEntries reject wrong hashes, path-traversal entries, and symlink entries before any extraction happens"
    requirement: "CONF-01"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/VerifySHA256_rejects_wrong_or_malformed_hashes"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/InspectZipEntries_rejects_traversal_and_symlink_entries_before_extraction"
        status: pass
    human_judgment: false
  - id: D3
    description: "ExtractVerified writes only into a fresh staging directory and leaves no residue on checksum or structure failure; PromoteAtomically exposes the complete verified tree or nothing and fully replaces (not merges) a prior install on a corrected retry"
    requirement: "CONF-03"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/ExtractVerified_writes_only_staging_and_leaves_no_residue_on_failure"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/PromoteAtomically_exposes_the_complete_tree_or_nothing"
        status: pass
    human_judgment: false
  - id: D4
    description: "AcquireStaged/AcquireAndPromote reject an unallowlisted source before any fetch call, leave no promoted install on tampered bytes, and a corrected retry over the committed source promotes atomically — using injected fakeSource fixtures only, no live network"
    requirement: "CONF-01"
    verification:
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/AcquireStaged_validates_policy_before_ever_calling_the_source"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/bootstrap_test.go#TestScopeBootstrapArchive/AcquireAndPromote_rejects_unofficial_sources_and_corrupt_bytes,_then_a_corrected_retry_promotes_atomically"
        status: pass
    human_judgment: false
  - id: D5
    description: "The registered bootstrap-archive scope exits 0 through the pinned-toolchain shim without live network"
    verification:
      - kind: integration
        ref: "powershell -NoProfile -File golc.ps1 test --quick --scope bootstrap-archive"
        status: pass
    human_judgment: false

duration: ~34min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 05: Official-Source Archive Acquisition, Verification, and Atomic Promotion Summary

**VerifySHA256/InspectZipEntries/ExtractVerified/PromoteAtomically archive building blocks plus an OfficialSourcePolicy-gated downloader, all fed only from config/toolchain.toml's committed host/path pins and executable via `golc.ps1 test --quick --scope bootstrap-archive`**

## Performance

- **Duration:** ~34 min (includes one-time pinned-toolchain bootstrap for this worktree)
- **Started:** 2026-07-21T00:25:00Z
- **Completed:** 2026-07-21T00:59:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `internal/bootstrap/archive.go` decomposes the D-01 executable-byte trust boundary into four independently testable building blocks: `VerifySHA256` (exact SHA-256 check), `InspectZipEntries` (rejects absolute/traversal entries via the existing `checkEntryName` helper, plus symlink entries detected through `archive/zip`'s stdlib `Mode()` decode of Unix external attributes), `ExtractVerified` (runs both checks first, then extracts only into a fresh staging directory — never anywhere else — and returns its path), and `PromoteAtomically` (a single `os.Rename` that replaces, never merges, any prior install; creates the install parent directory first so callers only need a staging parent to pre-exist).
- `internal/bootstrap/downloader.go` adds the acquisition half: `OfficialSourcePolicy`/`LoadOfficialSourcePolicy` read every pinned tool's `official_host`/`official_path_prefix` from `config/toolchain.toml` and accept only an `https` URL matching a committed host and path prefix; `Source`/`HTTPSource` separate transport from policy (production wires `HTTPSource`, every test injects a `fakeSource`); `AcquireStaged` validates policy before ever calling `Source.Fetch` and writes bytes to a contained staging file; `AcquireAndPromote` composes `AcquireStaged` + `ExtractVerified` + `PromoteAtomically` into the full acquire → verify → extract → promote flow the plan's `<behavior>` describes, so a corrected retry with the same arguments promotes a complete verified tree.
- `config/toolchain.toml` adds `toolchain.go.official_host = "go.dev"` and `toolchain.go.official_path_prefix = "/dl/"` as flat scalar keys under the existing `[toolchain.go]` table (deliberately not a `[[official_sources]]` array — see Deviations).
- `internal/bootstrap/bootstrap_test.go` self-registers quick-test scope `bootstrap-archive` beside `TestScopeBootstrapArchive`, covering: the committed `config/toolchain.toml` pin validated against the real repository root; `OfficialSourcePolicy.Allows` accepting the committed host/path and rejecting a different host, a look-alike subdomain, a different path, an insecure scheme, and a malformed URL; `LoadOfficialSourcePolicy` failing closed when no source is pinned; `VerifySHA256` rejecting wrong/malformed hashes; `InspectZipEntries` rejecting a traversal entry and a symlink entry (built via `zip.FileHeader.SetMode(os.ModeSymlink|0o777)`) while accepting a clean archive; `ExtractVerified` writing only into staging and leaving no residue on checksum or structural failure; `PromoteAtomically` exposing a complete tree on first install and fully replacing (not merging) it on a corrected retry; `AcquireStaged` making zero fetch calls and creating no staging directory when policy rejects a URL; and `AcquireAndPromote`'s three-step contract (untrusted host rejected before any fetch, tampered bytes over an allowlisted host leave no install, a corrected retry over the allowlisted host with the pinned bytes promotes atomically) using only `fakeSource` fixtures — no live network call anywhere in the file.
- `internal/projectconfig/model.go` registers `toolchain.go.official_host`/`toolchain.go.official_path_prefix` in `DefaultSpec()` with new `officialHostPattern`/`officialPathPrefixPattern` shape validators, so the strict single-authority decoder keeps accepting the real committed `config/toolchain.toml` (see Deviations).
- `powershell -NoProfile -File golc.ps1 test --quick --scope bootstrap-archive` exits 0 from the repository root with no network access; the full pinned-toolchain `go test ./...` and `go vet ./...` continue to pass across every package (`internal/bootstrap`, `internal/command`, `internal/projectconfig`, `internal/strictjson`, `internal/trace/catalog`).

## Task Commits

TDD gates committed atomically:

1. **RED - Task 1: bootstrap-archive acquisition and promotion contract** - `0632152` (test)
2. **GREEN - Task 1: archive/downloader implementation, toolchain pin, and model.go registration** - `4426721` (feat)

RED was verified by temporarily removing `archive.go`/`downloader.go` and confirming `go test ./internal/bootstrap/...` failed to build (`undefined: SourcePattern`, `undefined: LoadOfficialSourcePolicy`, `undefined: VerifySHA256`, ...) before restoring the files and confirming GREEN.

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/bootstrap/archive.go` - `VerifySHA256`, `InspectZipEntries`, `ExtractVerified`, `PromoteAtomically`.
- `internal/bootstrap/downloader.go` - `SourcePattern`, `OfficialSourcePolicy`, `LoadOfficialSourcePolicy`, `Source`, `HTTPSource`, `AcquireStaged`, `AcquireAndPromote`.
- `internal/bootstrap/bootstrap_test.go` - Registers scope `bootstrap-archive`/`TestScopeBootstrapArchive`; fixture helpers (`writeTestToolchainManifest`, `buildZipWithSymlinkEntry`, `fakeSource`, `repositoryRoot`) plus the full subtest suite listed above.
- `config/toolchain.toml` - Adds `toolchain.go.official_host`/`toolchain.go.official_path_prefix` pins.
- `internal/projectconfig/model.go` - Registers the two new toolchain.go keys with hostname/path-prefix `KeySpec` patterns in `DefaultSpec()`.

## Decisions Made

- **Flat scalar keys instead of a TOML array-of-tables:** `[[official_sources]]` flattens to a single non-string value under `internal/projectconfig`'s strict decoder (which only validates string-valued canonical keys via `validateDeclaredValue`'s `raw.(string)` check), so it would have failed every concern with `GOLC_CONFIG_VALUE_INVALID`/`GOLC_CONFIG_UNKNOWN_KEY` regardless of `model.go` changes. Per-tool flat keys (`toolchain.<name>.official_host`/`official_path_prefix`) stay compatible with the existing single-authority model and generalize cleanly if a second tool (e.g. Node) is pinned later.
- **`LoadOfficialSourcePolicy` decodes `config/toolchain.toml` independently of `internal/projectconfig`:** `internal/bootstrap` has no dependency on `internal/projectconfig` (and adding one isn't needed — `internal/command` already imports `projectconfig` with no cycle risk either way). Reading the manifest directly with `toml.DecodeFile` into a small private struct keeps the acquisition boundary self-contained and avoids coupling bootstrap's fail-closed archive logic to the five-layer resolution/strict-validation machinery, which serves a different concern (contributor-facing config precedence, not archive trust).
- **`PromoteAtomically` creates the install directory's parent:** the original design assumed the caller had already created it (mirroring `bootstrap.go`'s `InstallStaged`), but decomposing extraction and promotion into separate calls means a test (or a future caller) may reasonably extract into one staging parent and promote to an install tree whose own parent doesn't exist yet. Creating it defensively inside `PromoteAtomically` keeps the function self-sufficient and was caught immediately by the `PromoteAtomically exposes the complete tree or nothing` test.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Registered the two new toolchain.go keys in internal/projectconfig/model.go**
- **Found during:** Task 1, after adding `toolchain.go.official_host`/`official_path_prefix` to `config/toolchain.toml`
- **Issue:** `internal/projectconfig`'s strict single-authority decoder (`config-strict` scope, `TestScopeConfigStrict`) validates the real committed `config/toolchain.toml` against `DefaultSpec()`. Any key present in the file but undeclared in `DefaultSpec` fails with `GOLC_CONFIG_UNKNOWN_KEY`. Adding the two new committed pins without declaring them broke `TestScopeConfigStrict/production_repository_validates_with_one_authority_per_key_and_no_warnings` and `TestScopeConfigStrict/every_production_concern_validates_alone` (confirmed by running the test before the fix).
- **Fix:** Added `officialHostPattern`/`officialPathPrefixPattern` regexes and two `KeySpec` entries (`toolchain.go.official_host`, `toolchain.go.official_path_prefix`) to the `toolchain` concern in `DefaultSpec()`. `internal/projectconfig/registry.go`'s `DefaultRegistry()` automatically locks both new keys (it locks every `DefaultSpec` key it doesn't explicitly leave writable), so no further change was needed there.
- **Files modified:** `internal/projectconfig/model.go`
- **Verification:** `go test ./internal/projectconfig/... -run TestScopeConfigStrict -v` passes all 14 subtests; full `go test ./...` and `go vet ./...` pass through the pinned toolchain.
- **Committed in:** `4426721` (part of the GREEN commit, since the config/toolchain.toml change and its model.go registration are one coupled unit — splitting them would have left an intermediate commit with a broken config-strict scope)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The fix was a required side effect of the plan's own `key_links` contract (toolchain.toml carrying the OfficialSourcePolicy pin) meeting an existing strict-decoder invariant from Plan 03/18. No scope creep beyond declaring the two new keys' shape.

## Issues Encountered

- This worktree had no bootstrapped pinned toolchain (`.tools/` did not exist), matching the pattern noted in 01-18-SUMMARY.md. The host Go toolchain (`go1.26.5 windows/amd64`) exactly matches the committed pin, so implementation was verified with the host toolchain first via `go build`/`go test`/`go vet`. `powershell -NoProfile -File golc.ps1 bootstrap` was then run to provision `.tools/` for a true pinned-toolchain verification pass; the first attempt hit the same transient `Access to the path ... is denied` staging error documented in 01-18-SUMMARY.md, and the same idempotent retry (clearing the stale `.golc-staging-*` directory) succeeded immediately using the already checksum-cached archive. This is one-time worktree setup, not a plan deviation.

## Known Stubs

None. `AcquireAndPromote`/`HTTPSource` are unused by any production entrypoint yet (no caller wires them into the actual `golc.ps1 bootstrap` flow, which still performs its own PowerShell-side download/verify for the Go toolchain) — this is intentional per the plan's scope ("Harden tool archive acquisition and atomic promotion" as a Go-owned boundary, not a rewrite of the existing PowerShell bootstrap path) and matches the plan's `files_modified` list, which does not include `golc.ps1`. A future plan can wire `AcquireAndPromote`/`HTTPSource` into `tools update` or a Go-native bootstrap path without further changes here.

## Threat Flags

None — this plan directly implements the two threat-register mitigations it owns (T-01-12: allowlist/hash/traversal/atomic-promotion; T-01-SC: exact committed sources) and introduces no new network/process/credential surface beyond the already-scoped tool-archive download path. `InspectZipEntries`'s symlink rejection and `OfficialSourcePolicy`'s host/path/scheme allowlist are both implemented and test-locked in `bootstrap_test.go`.

## User Setup Required

None - everything is repository-local; no credentials, npm, or additional network access involved beyond the one-time toolchain bootstrap already required by prior plans.

## Next Phase Readiness

- `AcquireAndPromote`/`OfficialSourcePolicy`/`HTTPSource` are ready for a future `tools update` command or Go-native bootstrap path to call directly without further archive/downloader changes.
- `LoadOfficialSourcePolicy`'s per-tool `toolchain.<name>.official_host`/`official_path_prefix` convention is ready to extend to a second pinned tool (e.g. Node) by adding matching flat keys to `config/toolchain.toml` and `internal/projectconfig/model.go`'s `DefaultSpec`, following the exact pattern this plan established.
- `PromoteAtomically`'s replace-not-merge promotion semantics and `ExtractVerified`'s staging-only writes are available for any future cache-warming or update flow that needs to swap an installed tool tree safely.

## Self-Check: PASSED

- Both created files verified present on disk: `internal/bootstrap/archive.go`, `internal/bootstrap/downloader.go` (both `FOUND`).
- Commits `0632152` (test) and `4426721` (feat) verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only 0632152~1 4426721` reports zero deleted files across both commits; working tree clean before this summary.
- `powershell -NoProfile -File golc.ps1 test --quick --scope bootstrap-archive`, the full pinned-toolchain `go test ./...`, and `go vet ./...` all exit 0 from the repository root. `golc.ps1 test --quick --scope config`, `config-local`, `config-strict`, and `linear-catalog` (prior plans' scopes) also continue to pass unaffected.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
