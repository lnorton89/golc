---
phase: 08-isolated-typescript-automation
plan: 02
subsystem: infra
tags: [deno, toolchain, bootstrap, go, projectconfig, checksum-pinning, script]

# Dependency graph
requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    provides: config/toolchain.toml's checksum-pinned per-tool bootstrap pattern (Go/Node/Mage), internal/bootstrap's verify-before-extract install pipeline, internal/projectconfig's strict Spec/KeySpec configuration model
provides:
  - "[toolchain.deno] pin in config/toolchain.toml: version 2.9.4, all five platform archive_url/archive_sha256 pairs, byte-verified against the official per-asset github.com/denoland/deno/releases .sha256sum files"
  - 13 toolchain.deno.* canonical strict-config keys in internal/projectconfig DefaultSpec, plus a denoArchiveURLPattern helper
  - internal/bootstrap provisioning: platformArchiveLayout "deno" arm, validateManifestForPlatform widened to select/validate the deno pin, engine.run() installs it into .tools/toolchains/deno/<version>/<platform>/
  - bootstrap.ResolveDenoExecutable (mirrors ResolveMageExecutable) and the new internal/script package with script.ResolveDenoExecutable (wraps failures as GOLC_SCRIPT_DENO_MISSING) -- the single resolver every later Phase 8 plan uses instead of a PATH lookup
affects: [08-05, 08-06, 08-07, 08-08, 08-09]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Application-runtime toolchain pin: Deno follows the exact go/mage/node checksum-pinned per-platform archive pattern in config/toolchain.toml, but is documented as an application runtime dependency (not contributor tooling) and is provisioned unconditionally in engine.run(), not gated behind an Options flag."
    - "internal/script is a pure library sibling to internal/projectconfig: no CLI routes, no internal/command or internal/api import, to avoid closing the command -> delivery -> bootstrap -> script import cycle."

key-files:
  created:
    - internal/projectconfig/model_test.go
    - internal/script/toolchain.go
    - internal/script/toolchain_test.go
  modified:
    - config/toolchain.toml
    - internal/projectconfig/model.go
    - internal/projectconfig/strict_test.go
    - internal/bootstrap/engine.go
    - internal/bootstrap/engine_test.go

key-decisions:
  - "Deno's five platform archives (windows-amd64/linux-amd64/linux-arm64/darwin-amd64/darwin-arm64) were each independently downloaded and SHA-256 byte-verified against the official per-asset .sha256sum files fetched from github.com/denoland/deno/releases/download/v2.9.4/ before any hash was committed."
  - "platformArchiveLayout's deno arm sets Root empty (no nested directory) because Deno's real release ZIPs place the executable at the archive root, confirmed by unzip -l against the actual downloaded archives, not assumed from the plan text."
  - "validateManifestForPlatform's signature was widened from 3 manifestPin returns to 4 (adding denoPin) rather than introducing a new return-struct type, keeping the existing go/mage/node error-shape convention exactly."

patterns-established:
  - "New toolchain pins (Deno) join the existing go/mage/node checksum-pinned archive discipline end-to-end: config/toolchain.toml -> DefaultSpec strict keys -> platformArchiveLayout -> validateManifestForPlatform -> engine.run() install -> a dedicated Resolve<Tool>Executable function -- future toolchain additions can follow this exact five-point chain."

requirements-completed: [SCRP-03]

coverage:
  - id: D1
    description: "Deno 2.9.4 is pinned in config/toolchain.toml with checksum-verified archive_url/archive_sha256 for all five platforms, sourced only from the github.com/denoland/deno official-source allowlist."
    requirement: "SCRP-03"
    verification:
      - kind: unit
        ref: "internal/projectconfig/model_test.go#TestDenoToolchainKeysValidateInProduction"
        status: pass
      - kind: unit
        ref: "internal/projectconfig/model_test.go#TestDenoToolchainRejectsInvalidAuthority"
        status: pass
      - kind: integration
        ref: "mage Bootstrap (real run) -- .tools/toolchains/deno/2.9.4/windows-amd64/deno.exe produced, --version reports 2.9.4"
        status: pass
    human_judgment: false
  - id: D2
    description: "A hand-edited config/toolchain.toml with a malformed deno archive_sha256, a non-allowlisted archive_url host, or a missing required deno key fails strict configuration resolution with a stable diagnostic before any download is attempted."
    requirement: "SCRP-03"
    verification:
      - kind: unit
        ref: "internal/projectconfig/model_test.go#TestDenoToolchainRejectsInvalidAuthority"
        status: pass
    human_judgment: false
  - id: D3
    description: "internal/bootstrap provisions Deno through the same verify-before-extract path as Go/Mage/Node: platformArchiveLayout's deno arm, validateManifestForPlatform naming deno on a missing parent/platform, and engine.run() installing it into .tools/toolchains/deno/<version>/<platform>/, rejecting a checksum-mismatched archive."
    requirement: "SCRP-03"
    verification:
      - kind: unit
        ref: "internal/bootstrap/engine_test.go#TestScopeBootstrapEngine/PlatformKey_and_pure_platform_layouts_are_exact"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/engine_test.go#TestScopeBootstrapEngine/missing_[toolchain.deno]_entirely_fails_naming_deno_before_source_or_install_work"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/engine_test.go#TestScopeBootstrapEngine/missing_current_Deno_platform_fails_before_source_or_install_work"
        status: pass
      - kind: unit
        ref: "internal/bootstrap/engine_test.go#TestScopeBootstrapEngine/bootstrap_rejects_a_Deno_archive_whose_bytes_do_not_match_the_pinned_SHA-256"
        status: pass
      - kind: integration
        ref: "mage Bootstrap (real run, both first and second invocation): first installs deno, second reports 'deno 2.9.4 already verified' (no archive source consulted)"
        status: pass
    human_judgment: false
  - id: D4
    description: "script.ResolveDenoExecutable(root) is the single resolver returning the pinned Deno path, never falling back to a PATH lookup, and fails with GOLC_SCRIPT_DENO_MISSING when the install is absent."
    requirement: "SCRP-03"
    verification:
      - kind: unit
        ref: "internal/script/toolchain_test.go#TestScopeScriptToolchain/resolves_the_pinned_Deno_executable_when_a_verified_install_exists"
        status: pass
      - kind: unit
        ref: "internal/script/toolchain_test.go#TestResolveDenoExecutableMissing"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-07-26
status: complete
---

# Phase 8 Plan 2: Deno Toolchain Provisioning Summary

**Deno 2.9.4 is now a checksum-pinned, allowlist-sourced toolchain entry provisioned by `mage Bootstrap` exactly like Go/Node/Mage, resolvable through exactly one function (`script.ResolveDenoExecutable`) that never falls back to PATH.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2/2 completed
- **Files modified:** 5 modified, 3 created

## Accomplishments

- `[toolchain.deno]` added to `config/toolchain.toml` with `version = "2.9.4"`, the `github.com/denoland/deno/releases/download/` official-source allowlist, and all five platform `archive_url`/`archive_sha256` pins -- each hash independently downloaded and byte-verified against the official per-asset `.sha256sum` file, not copied from a summary
- 13 new `toolchain.deno.*` canonical strict-config keys registered in `internal/projectconfig` `DefaultSpec`, with a `denoArchiveURLPattern` helper anchoring host/path/asset-triple, and three negative-path tests (malformed checksum, non-allowlisted host, missing required key)
- `internal/bootstrap` provisions Deno through the identical download -> cache -> SHA-256 verify -> atomic extract -> install-manifest path every other pinned tool uses; `platformArchiveLayout`'s new `"deno"` arm was built and cross-checked against the real downloaded ZIP archives (`unzip -l`), confirming the executable sits at the archive root with no nested directory, matching Mage's shape rather than Go/Node's
- `bootstrap.ResolveDenoExecutable` and the new `internal/script` package's `script.ResolveDenoExecutable` give every later Phase 8 plan (08-05 through 08-09) one call site for the sandboxed Deno path, wrapping failures as `GOLC_SCRIPT_DENO_MISSING`
- A real `mage Bootstrap` run (not mocked) installed Deno end-to-end and a second run consulted no archive source for it; `go build ./...` succeeds with the produced `frontend/dist`

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin Deno in config/toolchain.toml and the strict configuration spec** - `2eb85f8` (feat)
2. **Task 2: Provision Deno through internal/bootstrap and expose ResolveDenoExecutable** - `323050f` (feat, tdd)

## Files Created/Modified

- `config/toolchain.toml` - Added `[toolchain.deno]` with version, official-source allowlist, and five platform archive pins
- `internal/projectconfig/model.go` - Added `denoArchiveURLPattern` and 13 `toolchain.deno.*` DefaultSpec keys
- `internal/projectconfig/model_test.go` (new) - Production-pin validation plus three negative-authority cases for the deno keys
- `internal/projectconfig/strict_test.go` - Updated the exact-toolchain-key-set assertion for the added deno keys (pre-existing test broken by Task 1's change)
- `internal/bootstrap/engine.go` - `platformArchiveLayout` "deno" arm, `validateManifestForPlatform` widened to select/validate the deno pin, `engine.run()` installs it, new `ResolveDenoExecutable`
- `internal/bootstrap/engine_test.go` - Fixture manifest/source now pins deno too; new layout, missing-parent, missing-platform, and checksum-mismatch coverage naming deno; deno resolution assertion added to the full-bootstrap test
- `internal/script/toolchain.go` (new) - Pure-library `internal/script` package; `ResolveDenoExecutable` delegates to `bootstrap.ResolveDenoExecutable`, wrapping failures as `GOLC_SCRIPT_DENO_MISSING`
- `internal/script/toolchain_test.go` (new) - Covers a successful resolution against a real staged install and the plan's named `TestResolveDenoExecutableMissing` marker

## Decisions Made

- Verified the live Deno release before writing any pin: fetched `https://api.github.com/repos/denoland/deno/releases/latest`, confirmed `v2.9.4` (published 2026-07-23) is still current, matching the research's recorded version and validity window.
- Downloaded each of the five platform archives and their per-asset `.sha256sum` files directly from `github.com/denoland/deno/releases/download/v2.9.4/`, computed SHA-256 locally, and confirmed byte-for-byte matches before committing any hash (transcript below).
- Confirmed via `unzip -l` against the real downloaded Windows and Linux archives that Deno's ZIP has no nested top-level directory (`deno.exe`/`deno` sit at the archive root), so `platformArchiveLayout`'s deno arm sets `Root: ""`, matching Mage's existing shape rather than Go/Node's `Root: "go"` / `Root: "node-v..."` pattern.
- Widened `validateManifestForPlatform`'s return signature from three `manifestPin` values to four (adding `denoPin`) rather than introducing a new struct type, to keep the exact existing go/mage/node error-shape convention the plan asked to follow.
- Ran a real (non-mocked) `mage Bootstrap` end-to-end, not just the unit-test fakes, to satisfy Task 2's acceptance criterion that a real bootstrap produces a working `deno.exe` reporting the pinned version.

### Verification transcript: Deno archive SHA-256 byte-verification

| Platform | Asset | Computed SHA-256 | Matches official `.sha256sum` |
|---|---|---|---|
| windows-amd64 | deno-x86_64-pc-windows-msvc.zip | `68ed08b05c56cf887e9aa509947dc3f468f7e12f47a13e5c1abd51d46d1453ef` | yes |
| linux-amd64 | deno-x86_64-unknown-linux-gnu.zip | `c24f955d9fbfe0ea5ae2b501c8e71ae76e31e4c9782390a54a284b3364fda725` | yes |
| linux-arm64 | deno-aarch64-unknown-linux-gnu.zip | `111da5c05c240cfdc4340f234a0e3539d39dbcb6755221f19dcd60bacc8be5aa` | yes |
| darwin-amd64 | deno-x86_64-apple-darwin.zip | `f757df6d3991e37601c69fad56c22b37c4ea77b5dcfad3636a642c2ba4c9b19f` | yes |
| darwin-arm64 | deno-aarch64-apple-darwin.zip | `6d17647fdbf9c587a581dba205054c4ccf732dae0a196cc1e9b44c07589db412` | yes |

Each `.sha256sum` file was fetched from `https://github.com/denoland/deno/releases/download/v2.9.4/<asset>.sha256sum` and compared byte-for-byte against a local `sha256sum`/`openssl dgst -sha256` computation of the freshly downloaded archive.

### Verification transcript: real `mage Bootstrap`

First run (fresh install):
```
bootstrap: warming project-local cache layout...
bootstrap: installing mage 1.17.2...
bootstrap: mage 1.17.2 installed
bootstrap: installing go 1.26.5...
bootstrap: go 1.26.5 installed
bootstrap: installing deno 2.9.4...
bootstrap: deno 2.9.4 installed
bootstrap: warming go module cache and building golc-project...
bootstrap: installing midicat v1.0.7 (go install)...
bootstrap: midicat v1.0.7 installed at ...\.tools\cache\go-bin\midicat.exe
bootstrap: building frontend...
bootstrap: installing node 24.18.0...
bootstrap: node 24.18.0 installed
bootstrap: frontend: npm ci...
bootstrap: frontend: npm run build...
bootstrap: complete
```

Second run (matching install manifest, no archive source consulted for deno):
```
bootstrap: warming project-local cache layout...
bootstrap: mage 1.17.2 already verified
bootstrap: go 1.26.5 already verified
bootstrap: deno 2.9.4 already verified
bootstrap: warming go module cache and building golc-project...
bootstrap: midicat v1.0.7 already installed
bootstrap: building frontend...
bootstrap: complete
```

Executable check:
```
$ .tools/toolchains/deno/2.9.4/windows-amd64/deno.exe --version
deno 2.9.4 (stable, release, x86_64-pc-windows-msvc)
v8 15.0.245.2-rusty
typescript 6.0.3
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated `strict_test.go`'s exact-toolchain-key-set assertion**
- **Found during:** Task 1
- **Issue:** `TestScopeConfigStrict`'s "production toolchain owns only the exact schema-v2 keys" subtest asserts a hardcoded, exhaustive sorted key list; adding the 13 `toolchain.deno.*` keys to `DefaultSpec` (Task 1's explicit deliverable) made this pre-existing test fail immediately.
- **Fix:** Inserted the 13 new deno keys into the expected `want` list in their correct sorted position (`toolchain.deno.*` sorts before `toolchain.go.*` since `d` < `g`).
- **Files modified:** `internal/projectconfig/strict_test.go`
- **Verification:** `go test ./internal/projectconfig/... -count=1` passes.
- **Committed in:** `2eb85f8` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix, direct and unavoidable consequence of Task 1's own deliverable)
**Impact on plan:** No scope creep -- this is the exact kind of pre-existing-test breakage the plan's own `<read_first>` note anticipated ("adding required keys means updating that fixture"), just landing in `strict_test.go` (the file that actually asserts the exhaustive key set) rather than `load_test.go`/`model_test.go` (whose fixtures are schema-agnostic and were unaffected).

## Known Stubs

None. Every deliverable (config pin, strict-spec keys, bootstrap provisioning, `ResolveDenoExecutable`) is fully wired end-to-end and verified against a real `mage Bootstrap` run, not stubbed.

## Threat Flags

None. Both threats this plan's `<threat_model>` names (T-08-SC archive acquisition, T-08-06 executable-resolution spoofing) are mitigated exactly as described: the archive is SHA-256-verified against a hash independently byte-verified in this task, the URL is constrained to the committed `github.com`/`/denoland/deno/releases/download/` allowlist, and `script.ResolveDenoExecutable` is the only source of the Deno path with no `exec.LookPath`, env-var override, or caller-supplied path anywhere in `internal/script`. No new surface outside the plan's threat register was introduced.

## Issues Encountered

- `go build ./...` initially failed with `pattern all:frontend/dist: no matching files found` -- a pre-existing environment gap (no `mage Bootstrap` had ever been run in this worktree, so `frontend/dist` didn't exist), unrelated to this plan's code changes. Resolved as part of the real-bootstrap verification step: running `mage Bootstrap` produced `frontend/dist`, after which `go build ./...` succeeds cleanly.
- Go's `regexp` package (RE2) does not support lookahead assertions; an initial test helper using `(?=...)` to strip the `[toolchain.deno]` block panicked at `regexp.MustCompile`. Replaced with a plain `bytes.Index`-based slice-and-concatenate approach.

## User Setup Required

None - no external service configuration required. Deno is provisioned automatically by `mage Bootstrap`, identically to Go/Node/Mage.

## Next Phase Readiness

- `script.ResolveDenoExecutable(root)` is ready for 08-05 (and every later Phase 8 plan) to obtain the pinned Deno path for spawning zero-permission `deno run` subprocesses.
- `internal/script` exists as an empty-but-real pure-library package other Phase 8 plans can add files to without introducing a command/api import.
- No blockers identified for the next wave.

---
*Phase: 08-isolated-typescript-automation*
*Completed: 2026-07-26*
