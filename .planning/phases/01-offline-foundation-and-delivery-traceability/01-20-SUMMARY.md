---
phase: 01-offline-foundation-and-delivery-traceability
plan: 20
subsystem: delivery
tags: [go, packaging, zip, determinism, sha256, powershell, offline]

requires:
  - phase: 01-offline-foundation-and-delivery-traceability
    plan: 06
    provides: internal/delivery/graph.go's LoadGraph/CommandInventory (entrypoint, cli_binary, go_version) and the self-registration idiom check.go/build.go/test.go establish
provides:
  - internal/delivery/foundation.go — FoundationInventory (sorted, duplicate-free allowlist derived from the graph's CommandInventory plus config/**/*.toml and schemas/*.json), CanonicalManifest (sorted, hashed, size-recorded manifest), EncodeManifest (canonical LF JSON), BuildFoundationBundle (deterministic ZIP with fixed 1980-01-01 epoch/0644 mode/forward-slash entries), DefaultFoundationOutputPaths/WriteFoundationBundle (atomic dist/foundation/ output)
  - internal/command/package.go — self-registered "package" route strictly accepting "package --foundation"
  - tests/acceptance/offline.ps1 -Mode package — runs package --foundation twice and diffs ZIP/manifest/checksum bytes end to end
affects: [delivery, foundation-package, ci-packaging]

tech-stack:
  added: []
  patterns:
    - "internal/delivery/foundation.go never imports internal/command or internal/bootstrap (same one-directional import rule graph.go's package doc establishes): it calls delivery.LoadGraph directly for the CommandInventory, and internal/command/package.go is the only self-registered route that calls BuildFoundationBundle — mirroring check.go's LoadGraph/RunOffline usage."
    - "FoundationInventory is a fixed, bounded allowlist (graph Entrypoint/CLIBinary + golc.project.toml + docs/development.md + a sorted walk of config/**/*.toml and schemas/*.json) rather than an unbounded directory scan, so an unrelated future top-level file (a Wails frontend, an NSIS script) can never silently enter the developer-tool ZIP."
    - "Determinism is achieved by never trusting real filesystem/OS state for archive metadata: every ZIP entry uses a fixed 1980-01-01 UTC epoch (not the source file's mtime), a normalized 0644 mode, and forward-slash names; CanonicalManifest always re-sorts by archive path before hashing so payload/manifest/ZIP entry order stays in lockstep regardless of the caller's input order."
    - "package.go's registered Route is the single word \"package\" (not \"package --foundation\") — router.go's word pattern forbids a route word starting with a dash, so \"--foundation\" is strictly parsed inside the handler, following the same precedent check.go (--concern/--offline) and test.go (--quick/--scope) already establish."
    - "The golden test (tests/golden/foundation-manifest.json) uses a synthetic, repository-independent fixture tree (writeFoundationFixture) rather than the real repository's current files, so the golden file never drifts when the real repository gains or loses config/schema files; only CanonicalManifest/EncodeManifest's canonicalization logic is under test, not the real repo's current byte contents."

key-files:
  created:
    - internal/delivery/foundation.go
    - internal/command/package.go
    - tests/golden/foundation-manifest.json
  modified:
    - internal/delivery/delivery_test.go
    - tests/acceptance/offline.ps1
    - .gitignore
    - docs/development.md

key-decisions:
  - "Foundation ZIP contents are a developer-tool bundle (bootstrap-built golc-project.exe + golc.ps1 + golc.project.toml + every config/**/*.toml concern + every schemas/*.json contract + docs/development.md), not a source tarball and not a Wails/NSIS product installer — internal/, cmd/, and .planning/ are deliberately excluded, matching 01-CONTEXT.md's Phase 1 boundary and 01-RESEARCH.md's resolved Open Question 3 (\"package --foundation creates a deterministic Windows AMD64 developer-tool ZIP, canonical manifest, and SHA-256 file\")."
  - "Output is written to a single fixed location, dist/foundation/{zip,manifest.json,zip.sha256}, that is overwritten on every run rather than a timestamped path — this makes tests/acceptance/offline.ps1 -Mode package's repeat-and-compare verification a direct byte comparison of the same file identity, and keeps dist/foundation/ covered by the existing generic `dist/` gitignore rule (a dedicated /dist/foundation/ line was added anyway, for discoverability, per this plan's explicit .gitignore acceptance criterion)."
  - "The manifest is embedded inside the ZIP itself (as foundation-manifest.json, the final entry after every file) in addition to being written as a sibling .manifest.json file, so a recipient who only has the ZIP can still verify its own contents without a companion file."

requirements-completed: [CONF-01, CONF-03]

coverage:
  - id: D1
    description: "internal/command/package.go self-registers the exact route \"package --foundation\", reachable through the default command registry, before verification runs."
    requirement: CONF-03
    verification:
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/package_--foundation_route_is_self-registered_and_reachable"
        status: pass
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\offline.ps1 -Mode package"
        status: pass
    human_judgment: false
  - id: D2
    description: "BuildFoundationBundle/CanonicalManifest sort files, normalize ZIP metadata (fixed epoch, 0644 mode, forward slashes), and produce byte-identical ZIP/manifest/checksum output across repeated runs of unchanged inputs."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/BuildFoundationBundle_produces_byte-identical_ZIP,_manifest,_and_checksums_across_repeated_runs"
        status: pass
      - kind: integration
        ref: "powershell -NoProfile -File .\\tests\\acceptance\\offline.ps1 -Mode package (real bootstrapped checkout, two full package --foundation runs diffed byte-for-byte)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Bundle contents come from the authoritative graph inventory (Entrypoint/CLIBinary) plus a bounded, explicit allowlist (config/**/*.toml, schemas/*.json, golc.project.toml, docs/development.md) — no unrelated file can enter the developer-tool ZIP, preserving the Phase 1 boundary."
    requirement: CONF-01
    verification:
      - kind: unit
        ref: "internal/delivery/delivery_test.go#TestScopeDelivery/FoundationInventory_returns_a_sorted,_duplicate-free_allowlist_derived_from_the_graph_inventory"
        status: pass
    human_judgment: false
  - id: D4
    description: ".gitignore ignores the exact dist/foundation/ distribution output without hiding go.sum, schemas/*.json, tests/golden/*, or .planning/linear-map.json; docs/development.md names the exact package --foundation outputs and the no-Wails/NSIS-product boundary."
    requirement: CONF-01
    verification:
      - kind: manual_procedural
        ref: "git check-ignore -v against dist/foundation/*.zip (ignored), tests/golden/foundation-manifest.json, schemas/golc-project.schema.json, go.sum, go.mod, and .planning/linear-map.json (all NOT ignored)"
        status: pass
    human_judgment: false
duration: ~55min
completed: 2026-07-21
status: complete
---

# Phase 1 Plan 20: Deterministic Foundation Package Summary

**`package --foundation` self-registers through the graph-inventory-driven allowlist and produces a byte-reproducible Windows AMD64 developer-tool ZIP (CLI binary + config + schemas + docs), canonical manifest, and SHA-256 checksum under `dist/foundation/`, verified end to end by `offline.ps1 -Mode package` against a real bootstrapped checkout**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-07-21T04:22:00Z
- **Completed:** 2026-07-21T05:17:38Z
- **Tasks:** 1 (TDD-flagged; treated as integration-first given the scope of one new package plus one route file plus an acceptance-script mode — same pattern 01-06-SUMMARY.md and 01-19-SUMMARY.md document)
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments

- `internal/delivery/foundation.go` is the single declarative owner of the foundation bundle contract (01-RESEARCH.md's resolved Open Question 3, T-01-16): `FoundationInventory(root, inventory)` returns a sorted, duplicate-free file allowlist derived from `delivery.LoadGraph`'s own `CommandInventory` (`Entrypoint`, `CLIBinary`) plus a bounded, explicit set of committed contributor-facing paths — `golc.project.toml`, `docs/development.md`, every `config/**/*.toml` concern, and every committed `schemas/*.json` contract (`config/`/`schemas/` are walked and sorted, never an unbounded scan of the repository). `CanonicalManifest` re-sorts by archive path, rejects a blank or duplicate archive path, hashes each entry's exact bytes with SHA-256, and returns payloads in the same sorted order the ZIP writer consumes. `EncodeManifest` renders the manifest as canonical two-space-indented, LF-terminated JSON. `BuildFoundationBundle` composes all of the above into a deterministic Windows AMD64 ZIP: every entry uses a fixed `1980-01-01T00:00:00Z` epoch (never the real file mtime), a normalized `0644` mode, and forward-slash names, with the encoded manifest itself embedded as the final `foundation-manifest.json` entry — repeating the whole build against unchanged inputs is byte-identical for the ZIP, the manifest, and both SHA-256 checksums.
- `internal/command/package.go` self-registers the `package` scope and route; the handler strictly accepts only `["--foundation"]`, calls `BuildFoundationBundle`, and writes the ZIP, manifest, and checksum sidecar to the one fixed location `DefaultFoundationOutputPaths` returns (`dist/foundation/golc-foundation-windows-amd64.{zip,manifest.json,zip.sha256}`) via `WriteFoundationBundle`'s stage-then-rename atomic writes — a second invocation overwrites the same paths rather than accumulating a new artifact, so repeat-and-compare acceptance observes one file identity across two runs.
- `tests/acceptance/offline.ps1` gained `-Mode package`: it runs `golc.ps1 package --foundation` once, copies the three output files' bytes aside, runs it again, and asserts byte-for-byte equality (Base64-string comparison, avoiding a `System.Linq.Enumerable` dependency in Windows PowerShell 5.1) for the ZIP, manifest, and checksum sidecar. Verified green end to end: `golc.ps1 bootstrap` succeeded for real in this environment (network reachable), then `offline.ps1 -Mode package` (both runs byte-identical), `offline.ps1 -Mode core` (unaffected — build/test --quick/check --offline/generate --check all still green), `golc.ps1 generate --check` (zero drift after the `docs/development.md` edit), and `golc.ps1 test` (full suite, every package `ok`) all passed.
- `docs/development.md` gained step 6 ("Build the deterministic foundation package") documenting the exact three output files and the explicit "developer-tool bundle, not a product installer" boundary; the existing "NSIS product packaging" out-of-scope bullet now cross-references `package --foundation` directly. `.gitignore` gained a dedicated `/dist/foundation/` section (in addition to the pre-existing generic `dist/` rule that already covered it) documenting the transient/regenerated nature of the output and explicitly noting it must never match `tests/golden/foundation-manifest.json`.

## Task Commits

1. **Task 1: Produce the deterministic foundation ZIP** - `f13c22d` (feat)

**Plan metadata:** committed with this summary

## Files Created/Modified

- `internal/delivery/foundation.go` - `FoundationEntry`, `FoundationInventory`, `collectSortedFiles`, `ManifestFileEntry`, `Manifest`, `CanonicalManifest`, `EncodeManifest`, `FoundationBundle`, `BuildFoundationBundle`, `buildDeterministicZIP`, `FoundationOutputPaths`, `DefaultFoundationOutputPaths`, `WriteFoundationBundle`, `writeFileAtomic`.
- `internal/command/package.go` - Self-registered `package` scope/route; `runPackage` (strict `--foundation` argument check), `runPackageFoundation` (build + write + report).
- `internal/delivery/delivery_test.go` - Added to `TestScopeDelivery`: route reachability, `FoundationInventory` sorting/dedup/incomplete-inventory rejection, `CanonicalManifest` hashing/duplicate/blank/missing-file rejection, `EncodeManifest` determinism plus golden-file byte match, `BuildFoundationBundle` repeat-build determinism and normalized ZIP entry metadata (epoch/mode/forward-slash) plus embedded-manifest decode, `WriteFoundationBundle` fixed-path atomic write and overwrite-on-repeat. Added `writeFoundationFixture` (synthetic fixture tree helper) and `goldenFoundationManifestPath` (locates `tests/golden/foundation-manifest.json` from the package test working directory).
- `tests/golden/foundation-manifest.json` - New golden fixture: the canonical `EncodeManifest` output for `writeFoundationFixture`'s synthetic 9-file tree (generated via a temporary in-repo throwaway program calling the production `delivery` functions directly, then deleted before commit — never hand-computed).
- `tests/acceptance/offline.ps1` - New `-Mode package` (`Invoke-FoundationPackageAcceptance`); `-Mode` parameter's `ValidateSet` extended to `("core", "package")`; mode-specific closing confirmation messages.
- `docs/development.md` - New "6. Build the deterministic foundation package" section; updated NSIS out-of-scope bullet to cross-reference it.
- `.gitignore` - New "Foundation package output" section documenting `/dist/foundation/`.

## Decisions Made

See `key-decisions` in the frontmatter above for the full rationale on: (1) the exact developer-tool-bundle scope of the ZIP contents (excluding source/`.planning/`, no Wails/NSIS claim), (2) the single fixed `dist/foundation/` output location enabling direct repeat-and-compare verification, and (3) embedding the manifest inside the ZIP itself in addition to the sibling `.manifest.json` file.

## Deviations from Plan

None - plan executed as written. `internal/delivery/graph.go` (listed only in `read_first`, not `files_modified`) was read for its `CommandInventory`/`LoadGraph` contract and imported by `foundation.go`, but never edited — the same "don't touch a file outside this plan's `files_modified`" discipline 01-06-SUMMARY.md's `config/commands.toml` deviation note documents.

## Issues Encountered

- **Golden fixture generation:** rather than hand-computing SHA-256 hashes for the synthetic fixture tree, a temporary throwaway Go program (`cmd/gengolden-tmp/main.go`, deleted before commit — never part of any commit) called the real `FoundationInventory`/`CanonicalManifest`/`EncodeManifest` functions against the exact fixture `writeFoundationFixture` builds, and its stdout was copied byte-for-byte into `tests/golden/foundation-manifest.json`. This guarantees the golden file is a genuine oracle of the production code path rather than a hand-typed approximation that could silently diverge from actual encoding behavior (indentation, key order, trailing newline).
- **PowerShell byte-array equality:** the acceptance script originally used `[System.Linq.Enumerable]::SequenceEqual`, which risks not being loaded by default in Windows PowerShell 5.1; switched to `[System.Convert]::ToBase64String(...)` case-sensitive string comparison (`-cne`), which has no additional assembly dependency and was verified working in this environment's actual `powershell.exe`.

## Known Stubs

None - `package --foundation` is fully wired: real bootstrap, real build, real ZIP/manifest/checksum output, verified against the actual repository contents (18 real files packaged in the live run, including the built `golc-project.exe`), not a placeholder.

## User Setup Required

None - `package --foundation` is a pure offline Go/PowerShell operation over the already-bootstrapped repository-local toolchain; no credential or external service is involved.

## Next Phase Readiness

- `internal/delivery.BuildFoundationBundle`/`FoundationInventory`/`CanonicalManifest`/`EncodeManifest`/`WriteFoundationBundle` are stable, importable primitives: a later plan needing a second deterministic-archive contract (for example a future product/Wails packaging step, explicitly out of Phase 1's scope) can reuse the same normalized-ZIP/canonical-manifest pattern without touching this plan's code.
- `01-VALIDATION.md`'s phase-gate command sequence (`01-15-03`) already references `golc.ps1 package --foundation`; that command is now real, registered, and green.
- `tests/acceptance/offline.ps1 -Mode package` is ready for CI wiring alongside the existing `-Mode core`.

## Self-Check: PASSED

- All created files verified present on disk: `internal/delivery/foundation.go`, `internal/command/package.go`, `tests/golden/foundation-manifest.json` (all `FOUND`).
- Commit `f13c22d` verified present in `git log --oneline --all`; `git diff --diff-filter=D --name-only HEAD~1 HEAD` reports zero deleted files; working tree clean before this summary.
- `go build ./...`, `go vet ./...`, and `go test -count=1 ./...` all exit 0 from the repository root; `go.mod`/`go.sum` unchanged (`git status --short go.mod go.sum` empty) throughout the session.
- `powershell -NoProfile -File .\tests\acceptance\offline.ps1 -Mode package` and `-Mode core` both exit 0 against a freshly bootstrapped repository under test (real `golc.ps1 bootstrap`, network reachable in this environment); `golc.ps1 generate --check` and `golc.ps1 test` (full suite) both remain green after the `docs/development.md` edit.
- `git check-ignore -v` confirmed `dist/foundation/*` is ignored while `tests/golden/foundation-manifest.json`, `schemas/golc-project.schema.json`, `go.sum`, `go.mod`, and `.planning/linear-map.json` are NOT ignored.

---
*Phase: 01-offline-foundation-and-delivery-traceability*
*Completed: 2026-07-21*
