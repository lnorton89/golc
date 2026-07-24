---
phase: quick-toolchain-schema-v2
plan: 260723-svv
type: execute
wave: 1
depends_on:
  - 260723-s7n
files_modified:
  - golc.project.toml
  - config/application-defaults.toml
  - config/commands.toml
  - config/generation.toml
  - config/integrations/linear.toml
  - config/runtime.toml
  - config/toolchain.toml
  - internal/projectconfig/load.go
  - internal/projectconfig/local.go
  - internal/projectconfig/model.go
  - internal/projectconfig/load_test.go
  - internal/projectconfig/local_test.go
  - internal/projectconfig/resolve_test.go
  - internal/projectconfig/strict_test.go
  - internal/bootstrap/downloader.go
  - internal/bootstrap/bootstrap_test.go
  - internal/bootstrap/engine.go
  - internal/bootstrap/engine_test.go
  - internal/bootstrap/engine_linear.go
  - internal/bootstrap/engine_linear_test.go
  - internal/command/tools.go
  - internal/command/tools_test.go
  - internal/security/redact_test.go
  - golc.ps1
  - tests/acceptance/bootstrap.ps1
  - tests/acceptance/walking-skeleton.ps1
  - tests/fixtures/config/walking-skeleton/golc.project.toml
  - tests/fixtures/config/walking-skeleton/config/runtime.toml
  - tests/fixtures/config/walking-skeleton/config/toolchain.toml
autonomous: true
requirements: [CONF-01, CONF-02, CONF-03]
must_haves:
  truths:
    - "The root index and all six committed concern files declare schema_version = 2 together, and this build rejects schema version 1 rather than accepting a mixed repository."
    - "Go and Node keep version and official-source policy at [toolchain.<tool>], while exact archive_url/archive_sha256 pairs live only under [toolchain.<tool>.platforms.\"windows-amd64\"]."
    - "The currently configured archive set is exactly windows-amd64 for both Go and Node; Linux and Darwin layout knowledge does not become a configured pin or a support claim."
    - "DefaultSpec owns every resulting flattened dotted key explicitly, including the two windows-amd64 archive pairs, and an unregistered platform key fails strict validation without wildcard matching."
    - "The Step 1 Go bootstrap selects only PlatformKey()'s explicitly configured archive, fails closed before acquisition when that platform is absent, and preserves all existing URL and checksum bytes."
    - "archive_url is the single archive locator field for both generic tools and platform toolchain archives; archive_uri is no longer accepted by the Go or compatibility readers and fixtures."
    - "The existing PowerShell bootstrap remains only a compatibility shim for the schema-v2 shape until a later removal step; this task does not redirect commands to Go or remove PowerShell."
  artifacts:
    - path: config/toolchain.toml
      provides: "Schema-v2 tool-level metadata plus exact windows-amd64 Go and Node archive subtables"
    - path: internal/projectconfig/model.go
      provides: "Explicit strict ownership for every schema-v2 flattened key"
    - path: internal/projectconfig/load.go
      provides: "One global supportedSchemaVersion value of 2 for root, concern, user, and local readers"
    - path: internal/bootstrap/engine.go
      provides: "Platform-keyed schema-v2 manifest decoding and fail-closed archive selection"
    - path: internal/bootstrap/engine_linear.go
      provides: "Node provisioning through the selected current-platform pin"
    - path: internal/command/tools.go
      provides: "Surgical tools-update reads and writes for parent versions and windows-amd64 archive subtables"
    - path: golc.ps1
      provides: "Temporary schema-v2 compatibility parsing without Step 3 handoff or removal"
  key_links:
    - from: config/toolchain.toml
      to: internal/projectconfig/model.go
      via: "TOML flattening maps each quoted platform table to one exact DefaultSpec key"
      pattern: "toolchain\\.(go|node)\\.platforms\\.windows-amd64\\.(archive_url|archive_sha256)"
    - from: internal/projectconfig/load.go
      to: golc.project.toml
      via: "supportedSchemaVersion gates the root index and every indexed concern at the same version"
      pattern: "supportedSchemaVersion"
    - from: internal/bootstrap/engine.go
      to: config/toolchain.toml
      via: "PlatformKey selects one archive pair from toolchain.<tool>.platforms before validation or acquisition"
      pattern: "PlatformKey|Platforms"
    - from: internal/bootstrap/engine_linear.go
      to: internal/bootstrap/engine.go
      via: "Linear bootstrap receives the selected Node platform pin instead of reading archive fields from the parent tool table"
      pattern: "node|PlatformKey"
    - from: internal/command/tools.go
      to: config/toolchain.toml
      via: "tools update changes only the two versions and the four windows-amd64 archive values"
      pattern: "windows-amd64|archive_url|archive_sha256"
---

<objective>
Migrate GOLC's strict configuration and toolchain readers to schema version 2 with explicit per-platform archive pins.

Purpose: Separate cross-platform tool identity/source policy from platform-specific executable bytes without weakening strict key ownership or fabricating archive pins for platforms the repository does not currently configure.
Output: An atomic schema-v2 configuration set, explicit windows-amd64 key registry, adapted Go bootstrap and update readers, temporary PowerShell compatibility, and offline TDD coverage for strictness and platform selection.
</objective>

<execution_context>
@C:/Users/Lawrence/.codex/gsd-core/workflows/execute-plan.md
@C:/Users/Lawrence/.codex/gsd-core/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/REQUIREMENTS.md
@.planning/quick/260723-s7n-implement-the-complete-go-bootstrap-engi/260723-s7n-PLAN.md
@.planning/quick/260723-s7n-implement-the-complete-go-bootstrap-engi/260723-s7n-SUMMARY.md
@golc.project.toml
@config/toolchain.toml
@config/commands.toml
@config/generation.toml
@config/application-defaults.toml
@config/runtime.toml
@config/integrations/linear.toml
@internal/projectconfig/model.go
@internal/projectconfig/load.go
@internal/projectconfig/decode.go
@internal/projectconfig/local.go
@internal/projectconfig/resolve.go
@internal/projectconfig/registry.go
@internal/bootstrap/engine.go
@internal/bootstrap/engine_linear.go
@internal/bootstrap/downloader.go
@internal/command/tools.go
@internal/security/redact_test.go
@golc.ps1
@tests/fixtures/config/walking-skeleton/golc.project.toml
@tests/fixtures/config/walking-skeleton/config/toolchain.toml
@tests/fixtures/config/walking-skeleton/config/runtime.toml

**Dirty-worktree constraint:** Preserve the unrelated untracked `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/` directory and any later user changes. Before each task, inspect `git status --short` plus `git diff --` for that task's files; layer changes onto existing user hunks and never revert, replace, or reformat unrelated work.

**Locked migration shape:** Use `[toolchain.go.platforms."windows-amd64"]` and `[toolchain.node.platforms."windows-amd64"]`. Keep `version`, `official_host`, and `official_path_prefix` in their existing parent tool tables. Use `archive_url` and `archive_sha256` in each platform table. Preserve these exact committed values byte-for-byte:
- Go URL `https://go.dev/dl/go1.26.5.windows-amd64.zip`
- Go SHA-256 `97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38`
- Node URL `https://nodejs.org/dist/v24.18.0/node-v24.18.0-win-x64.zip`
- Node SHA-256 `0ae68406b42d7725661da979b1403ec9926da205c6770827f33aac9d8f26e821`

**Platform audit:** The committed manifest currently pins only Windows AMD64 archives for Go and Node. `platformArchiveLayout` knows filename/executable layouts for additional Go OS/architecture pairs, but those pure mappings are not archive availability, qualification, or support. Do not add Linux, Darwin, ARM64, or any other platform table, URL, checksum, fallback, derived URL, empty placeholder, or support statement. A future platform remains unavailable until both exact archive_url and archive_sha256 values are explicitly committed and registered.

**Scope boundary:** Complete PowerShell-removal Step 2 only. Keep `golc.ps1` as the current root entrypoint and change only what it needs to parse/select the schema-v2 windows-amd64 tables and the unified generic-tool archive_url field. Do not call the Step 1 Go `Bootstrap` API from PowerShell, add a new bootstrap route, replace command delegation, delete PowerShell, rename the root entrypoint, change install/cache layouts, change tool versions, download new checksums, add dependencies, edit documentation, or perform any Step 3+ cutover/removal work.

**Discovery:** Level 0. This is an internal schema migration using the existing BurntSushi TOML decoder, strict flattening, Step 1 bootstrap seams, surgical tools-update helpers, and registered Go quick scopes. No external package or API research is required and no package legitimacy gate is triggered.

<interfaces>
From `internal/projectconfig/load.go`:
- `const supportedSchemaVersion = 1` is the single gate used by root, concern, user, and project-local decoding and becomes 2.

From `internal/projectconfig/local.go`:
- `canonicalLocalKeyPattern` currently rejects hyphenated segments even though `windows-amd64` must become part of a flattened strict key.
- `flattenLocalDocument(prefix, document, into)` recursively produces exact dotted keys and provides no wildcard semantics.
- `renderLocalDocument(values)` currently emits a literal schema version and must follow the same global version.

From `internal/bootstrap/engine.go`:
- `func PlatformKey() string` returns `runtime.GOOS + "-" + runtime.GOARCH`.
- `type manifestPin` currently combines version, archive locator/hash, and official-source policy.
- `type bootstrapManifest` currently maps both `Tools` and `Toolchain` directly to `manifestPin`.
- `func readBootstrapManifest(root string) (bootstrapManifest, OfficialSourcePolicy, error)` strictly rejects undecoded TOML keys.
- `func validatePlatformPin(tool string, pin manifestPin) error` validates the selected archive filename against `platformArchiveLayout`.
- `func (engine *bootstrapEngine) installPin(pin manifestPin, installDir string) error` checks the manifest/cache/source/staged-install path.

From `internal/bootstrap/engine_linear.go`:
- `runLinearSync` currently obtains Node version and archive data from one flat `engine.document.Toolchain["node"]` value.

From `internal/command/tools.go`:
- `readTOMLTableValue` and `replaceTOMLTableValue` operate on exact named table headers while preserving every other byte.
- `readTOMLTableTriple` currently assumes version, archive_url, and archive_sha256 share one table.
- `applyToolchainTOMLProposal` currently changes six exact pin lines across the two flat parent tables.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Lock the strict schema-v2 configuration contract</name>
  <files>internal/projectconfig/load_test.go, internal/projectconfig/local_test.go, internal/projectconfig/resolve_test.go, internal/projectconfig/strict_test.go, internal/projectconfig/load.go, internal/projectconfig/local.go, internal/projectconfig/model.go, golc.project.toml, config/application-defaults.toml, config/commands.toml, config/generation.toml, config/integrations/linear.toml, config/runtime.toml, config/toolchain.toml, internal/security/redact_test.go</files>
  <read_first>
    - `internal/projectconfig/model.go` DefaultSpec key allocation and URL/hash patterns.
    - `internal/projectconfig/load.go` root/concern schema gate.
    - `internal/projectconfig/decode.go` exact flattened-key lookup and unknown-key behavior.
    - `internal/projectconfig/local.go` canonical key grammar, fixed local registry, renderer, and local schema reader.
    - All tests in `internal/projectconfig/`, especially every embedded `schema_version` fixture and the wrong-version case.
    - The seven committed configuration files and `internal/security/redact_test.go`'s embedded foundation fixture.
  </read_first>
  <behavior>
    - "A root index or concern declaring schema version 2 validates; version 1, a missing version, or a mixed root/concern set fails with GOLC_CONFIG_SCHEMA_VERSION."
    - "Every committed concern validates alone, and the complete committed repository validates with no warnings after one atomic migration."
    - "DefaultSpec owns exactly toolchain.go.version, toolchain.go.official_host, toolchain.go.official_path_prefix, toolchain.go.platforms.windows-amd64.archive_url, toolchain.go.platforms.windows-amd64.archive_sha256, the corresponding five Node keys, and the existing three cache keys for the toolchain concern."
    - "A quoted TOML platform table flattens to the exact windows-amd64 dotted keys; a syntactically valid but unregistered linux-amd64 or other platform key fails GOLC_CONFIG_UNKNOWN_KEY."
    - "The canonical grammar admits a hyphen inside an otherwise lowercase dotted segment but still rejects empty segments, leading dots, path separators, traversal text, malformed hyphen placement, and environment-style redirection targets."
    - "Local/user test documents and renderLocalDocument use schema version 2, and the old flat toolchain.go.archive_url/toolchain.go.archive_sha256 names are no longer production registry keys."
    - "The four committed Go/Node URL and SHA-256 literals are unchanged after moving under the two windows-amd64 tables."
  </behavior>
  <action>RED: Update the projectconfig fixtures and assertions first. Make valid synthetic root, concern, user, and local documents use schema version 2; make the legacy-version test explicitly present version 1 and require the stable schema diagnostic. Add a production DefaultSpec assertion for the exact toolchain key set above, a successful quoted windows-amd64 flattening case, an unregistered-platform rejection case, and canonical-key grammar cases proving the required hyphen is narrowly accepted without weakening redirect/path rejection. Update the redaction foundation fixture's embedded configuration versions so its packaged example represents the supported schema. Run the focused projectconfig tests and confirm the new expectations fail against the version-1 implementation/flat manifest for the intended reasons.

GREEN: Change the one global `supportedSchemaVersion` constant to 2 and make the local renderer emit that same supported value rather than a separate stale literal. Replace the local registry's old flat Go archive keys with their exact windows-amd64 names and adjust the canonical segment grammar only enough to admit `windows-amd64`. In DefaultSpec, remove the four flat Go/Node archive keys and register the four exact platform-qualified archive keys with the existing tool-specific URL patterns and SHA-256 pattern; do not add a wildcard matcher, prefix matcher, inferred key path, or any unconfigured platform. Atomically change `schema_version` to 2 in `golc.project.toml` and all six concern files. In `config/toolchain.toml`, move only the existing archive_url/archive_sha256 pairs into the two locked platform subtables, retain version/source-policy values at the parent level, preserve URL/checksum literals exactly, and update comments to state that only windows-amd64 is configured while other platform pins remain absent. Do not add deprecation aliases for the old flat keys: schema 1 is rejected as a unit rather than partially migrated.</action>
  <verify>
    <automated>powershell -NoProfile -File .\golc.ps1 test --quick --scope config-strict</automated>
  </verify>
  <done>The repository has one strict schema-v2 authority set, every platform archive key is explicit and enumerated, version-1/mixed/unregistered-platform inputs fail closed, and the exact Windows pin bytes are preserved.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Select explicit platform pins in the Step 1 Go bootstrap</name>
  <files>internal/bootstrap/engine_test.go, internal/bootstrap/engine_linear_test.go, internal/bootstrap/bootstrap_test.go, internal/bootstrap/engine.go, internal/bootstrap/engine_linear.go, internal/bootstrap/downloader.go</files>
  <read_first>
    - `internal/bootstrap/engine.go` manifest types, strict decoder, validation ordering, platform filename mapping, installPin, and Go orchestration.
    - `internal/bootstrap/engine_test.go` writeEngineRepository fixture, injected Source/Runner, current-platform archive builder, mismatch/no-source assertions, and repeat behavior.
    - `internal/bootstrap/engine_linear.go` Node pin selection and install path.
    - `internal/bootstrap/engine_linear_test.go` Node table injection, exact-lock build, and zero-call repeat.
    - `internal/bootstrap/downloader.go` independent official-source policy reader.
    - `internal/bootstrap/bootstrap_test.go` schema probe, source-policy manifest writer, and committed-source assertions.
    - `.planning/quick/260723-s7n-implement-the-complete-go-bootstrap-engi/260723-s7n-SUMMARY.md` for Step 1 invariants that must not regress.
  </read_first>
  <behavior>
    - "Schema-v2 manifest decoding keeps generic tools as archive_url/archive_sha256 pins and decodes each toolchain parent into version/source policy plus a platforms map."
    - "Core bootstrap selects document.Toolchain.go.Platforms[PlatformKey()] before any cache warm, source fetch, or install and uses only that pair with the parent Go version."
    - "Linear include selects document.Toolchain.node.Platforms[PlatformKey()] through the same helper; include-off does not require or inspect a Node platform pin."
    - "A missing current-platform Go or requested Node pin returns a stable diagnostic naming the exact required platform table and produces zero Source calls and no .tools directory."
    - "A selected URL whose filename disagrees with platformArchiveLayout still returns GOLC_BOOTSTRAP_PLATFORM_MISMATCH before effects."
    - "Pure layout cases for Linux/Darwin remain filename/executable unit tests only; a production-manifest audit proves the configured platform key set is exactly windows-amd64 for Go and Node."
    - "Generic tools accept archive_url and reject the obsolete archive locator spelling as an undecoded/unsupported key."
    - "Existing checksum verification, official-source allowlisting, content-addressed cache, install paths, lock identity, process order, and zero-source repeats remain unchanged."
  </behavior>
  <action>RED: Rewrite the synthetic Step 1 manifests to schema version 2 and platform subtables, using quoted `PlatformKey()` table segments and archive_url for the generic fixture. Add focused tests for current-platform Go selection, optional Node selection, missing Go platform, missing requested Node platform, obsolete generic archive field rejection, filename mismatch, exact production platform enumeration, and the distinction between pure future layout mappings and committed pins. Each missing/mismatched case must assert zero Source calls and absence of `.tools`. Update bootstrap source-policy/probe fixtures to schema 2, then run the bootstrap-engine and bootstrap-linear-sync scopes and confirm the new schema/selection tests fail against the flat reader.

GREEN: Split the manifest model into parent toolchain metadata and platform archive records while keeping generic tool records independently flat. Remove ArchiveURI and its fallback method completely; archive_url is the only locator passed to Acquire. Make `readBootstrapManifest` require schema version 2, preserve strict undecoded-key rejection, and collect source-policy patterns from each parent tool plus generic tool entries without multiplying patterns per platform. Add one exact current-platform selection helper that combines the parent version with `Platforms[PlatformKey()]`, validates completeness/checksum/archive suffix/expected filename, and returns a diagnostic naming `[toolchain.<tool>.platforms."<key>"]` when absent. Call it for Go during initial validation and for Node only when IncludeLinearSync is true; carry the selected archive into installPin and Node orchestration rather than re-reading flat parent fields. Adapt the independent policy reader only as needed for the new parent/platform document without broadening its allowlist. Keep `platformArchiveLayout`'s pure mappings and every Step 1 security/cache/process invariant intact; never synthesize a URL, checksum, platform map entry, or fallback from the running OS.</action>
  <verify>
    <automated>powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-engine; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-linear-sync</automated>
  </verify>
  <done>The Go bootstrap and optional Linear path consume exactly the current platform's committed schema-v2 archive pair, distinguish layout capability from configured availability, and fail without effects when no exact pin exists.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Align direct readers, update proposals, and compatibility fixtures</name>
  <files>internal/command/tools_test.go, internal/command/tools.go, golc.ps1, tests/acceptance/bootstrap.ps1, tests/acceptance/walking-skeleton.ps1, tests/fixtures/config/walking-skeleton/golc.project.toml, tests/fixtures/config/walking-skeleton/config/runtime.toml, tests/fixtures/config/walking-skeleton/config/toolchain.toml</files>
  <read_first>
    - `internal/command/tools.go` exact-table span/read/replace helpers and six-line proposal contract.
    - `internal/command/tools_test.go` toolchain fixture, proposed Windows URLs, no-install structural gate, changed-line count, and production no-op.
    - `golc.ps1` Read-GolcToml, generic tool loop, Go bootstrap block, and Invoke-GolcBootstrapLinearSync Node block.
    - `tests/acceptance/bootstrap.ps1` generic archive fixture writer and zero-source repeat assertions.
    - `tests/acceptance/walking-skeleton.ps1` fixture placeholder replacement.
    - All three files under `tests/fixtures/config/walking-skeleton/`.
  </read_first>
  <behavior>
    - "tools update reads each version from toolchain.<tool> and each URL/checksum from toolchain.<tool>.platforms.\"windows-amd64\", then changes the same six values while preserving every other byte."
    - "The production default tools-update proposal remains a byte-identical no-op for config/toolchain.toml after migration."
    - "The PowerShell TOML reader accepts the exact quoted windows-amd64 section headers, selects that explicit table for Go and Node, and never derives or searches another platform."
    - "PowerShell generic tools read archive_url, with archive_uri absent from production code and active bootstrap fixtures."
    - "The walking-skeleton root/runtime/toolchain fixtures declare schema version 2 and its generic tool fixture uses archive_url while retaining the runtime-computed local file URL and checksum placeholders."
    - "The compatibility shim preserves install/cache/module/npm behavior and does not call the Go Bootstrap API, remove functions, change delegation, or perform Step 3 cutover."
  </behavior>
  <action>RED: Migrate the tools-update synthetic toolchain fixture to schema 2 with two quoted windows-amd64 platform tables. Update its expectations so parent version lines and child archive lines are the only six proposal changes, other platform data would be preserved, and the production default proposal is still a value/byte no-op. Update the bootstrap acceptance fixture to use schema 2 and archive_url, and update the walking-skeleton fixture expectations/placeholders accordingly. Add a narrow PowerShell acceptance assertion that the quoted platform table is parsed and used, while a missing windows-amd64 table fails rather than falling back. Run tools-update and the local bootstrap acceptance to establish the expected failures before reader changes.

GREEN: Refactor the tools-update read/apply helpers so the version table and archive table are explicit inputs; keep the current proposal model limited to the one actually configured windows-amd64 build target and retain surgical byte preservation. Extend `Read-GolcToml` only enough to normalize the exact quoted platform table header, then make the Go and Node compatibility blocks require `toolchain.<tool>.platforms.windows-amd64` for archive_url/archive_sha256 while reading version from the parent. Change the generic tool loop to require archive_url. Update the walking-skeleton substitution names from URI terminology to URL terminology without changing the injected local file URL or computed checksum bytes. Keep all PowerShell bootstrap execution, delegation, caches, locks, output, and install directories otherwise unchanged. Do not introduce generic table matching, platform discovery, Go handoff, route changes, function deletion, or any removal work.</action>
  <verify>
    <automated>powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1</automated>
  </verify>
  <done>Every active configuration consumer and fixture understands the exact schema-v2 table shape and unified archive_url field, while the existing PowerShell entrypoint remains behaviorally intact pending a later removal step.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| TOML files to strict configuration registry | Quoted platform table names flatten into canonical keys and must not gain wildcard, traversal, or duplicate-authority semantics. |
| committed platform map to bootstrap selection | The runtime may execute only an archive pair explicitly committed for its exact GOOS-GOARCH key; layout knowledge is not authorization. |
| archive metadata to acquisition/install | URL, source policy, checksum, filename, cache, and staged-install checks remain the executable-byte trust boundary. |
| schema migration to repository consumers | Root, concerns, local/user layers, Go readers, update tools, compatibility PowerShell, and fixtures must not observe a mixed version/shape. |
| update proposal to toolchain authority | Surgical update logic may change only the intended version and windows-amd64 archive fields while preserving all unrelated bytes. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-svv-01 | Tampering | DefaultSpec / flattened platform keys | high | mitigate | Register each windows-amd64 URL/hash key exactly, narrow the canonical grammar to valid hyphenated segments, and test unregistered platform rejection with no wildcard path. |
| T-svv-02 | Spoofing / Elevation of Privilege | bootstrap platform selection | high | mitigate | Index only by PlatformKey, require the exact table before effects, validate archive filename against the pure layout, and never derive/fallback to another platform. |
| T-svv-03 | Tampering | committed archive pins | high | mitigate | Move the four existing URL/checksum literals without changing bytes and retain SHA-256, official-source, cache, and staged-install verification. |
| T-svv-04 | Tampering / Repudiation | schema-version cutover | medium | mitigate | Change the global gate, root index, six concerns, local renderer, and embedded fixtures in one task; explicitly reject legacy/mixed schema tests. |
| T-svv-05 | Tampering | tools update surgical rewrite | medium | mitigate | Name parent and windows-amd64 child tables explicitly, assert exactly six changed lines, preserve all other bytes, and keep the fixed write allowlist. |
| T-svv-06 | Information Disclosure | redaction foundation fixtures | low | mitigate | Update only schema metadata in credential-free embedded fixtures and rerun the secrets scope/full suite. |
| T-svv-07 | Denial of Service | temporary PowerShell compatibility | medium | mitigate | Teach the current shim only the exact v2 table syntax/selection and retain existing cache/install/no-op behavior until the later cutover is separately executed. |
| T-svv-SC | Tampering | package supply chain | high | mitigate | Add, remove, or update no package; keep go.mod, go.sum, package.json, and package-lock.json out of the task and use only existing decoders/test infrastructure. |
</threat_model>

<source_audit>

| SOURCE | ID | Feature/Requirement | Plan | Status | Notes |
|--------|----|---------------------|------|--------|-------|
| GOAL | — | Complete PowerShell-removal Step 2 by migrating strict configuration to explicit platform pins | 260723-svv | COVERED | Tasks 1-3 migrate the authority, Go consumers, direct readers, and compatibility fixtures without Step 3 cutover. |
| REQ | CONF-01 | Root-owned discoverable toolchain configuration | 260723-svv | COVERED | The root still indexes toolchain; the manifest makes platform availability explicit. |
| REQ | CONF-02 | Independently validatable concerns with one authority per value | 260723-svv | COVERED | Task 1 preserves six concerns and exact DefaultSpec ownership for every v2 key. |
| REQ | CONF-03 | Shared documented project command behavior | 260723-svv | COVERED | Task 3 keeps the current root command compatible while Go/test consumers adopt the same shape. |
| RESEARCH | — | No quick-task RESEARCH.md or DISCOVERY.md exists | — | EXCLUDED | Level 0 migration over established repository code and pinned TOML library. |
| CONTEXT | — | Platform subtables use `<goos>-<goarch>` keys | 260723-svv | COVERED | Locked to quoted windows-amd64 tables, selected by PlatformKey. |
| CONTEXT | — | Use archive_url consistently | 260723-svv | COVERED | Removed archive_uri compatibility from Go, PowerShell generic fixtures, and walking-skeleton data. |
| CONTEXT | — | DefaultSpec enumerates every strict dotted key with no wildcards | 260723-svv | COVERED | Task 1 names all four platform keys and tests an unregistered platform failure. |
| CONTEXT | — | Bump root plus all six concerns and the global gate to schema 2 atomically | 260723-svv | COVERED | Task 1 owns the complete cutover and every config-schema fixture. |
| CONTEXT | — | Update embedded redaction fixtures | 260723-svv | COVERED | Task 1 migrates `internal/security/redact_test.go` fixture versions. |
| CONTEXT | — | Adapt the completed Step 1 bootstrap engine/readers/tests | 260723-svv | COVERED | Task 2 adapts core/Linear/policy readers and their registered offline scopes. |
| CONTEXT | — | Preserve exact checksums and URLs | 260723-svv | COVERED | Locked values are recorded in context and tested before/after relocation. |
| CONTEXT | — | Do not invent unavailable platform pins | 260723-svv | COVERED | Only windows-amd64 is configured; layout-only future cases stay unpinned. |
| CONTEXT | — | Distinguish configured build platforms from future missing pins | 260723-svv | COVERED | Config comments, production audit, and missing-current-platform tests establish the distinction. |
| CONTEXT | — | No Step 3+ work | 260723-svv | COVERED | PowerShell receives compatibility parsing only; no handoff, route replacement, deletion, or entrypoint change occurs. |
| CONTEXT | — | Preserve unrelated quick-task directory | 260723-svv | COVERED | Dirty-worktree guard explicitly protects the sgy directory and later user work. |

</source_audit>

<verification>
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-strict` passes exact production key ownership, schema-v2 validation, legacy rejection, and no-wildcard platform tests.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope config-local` and `powershell -NoProfile -File .\golc.ps1 test --quick --scope config` pass schema-v2 local/user rendering, locked-key, precedence, and redirect tests.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope bootstrap-archive`, `bootstrap-engine`, and `bootstrap-linear-sync` pass source-policy, explicit platform selection, missing-pin no-effect, checksum/cache/install, and repeat behavior offline.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope tools-update` passes exact parent/child table updates, fixed allowlist, six-line surgical changes, and production no-op behavior.
- `powershell -NoProfile -File .\golc.ps1 test --quick --scope secrets` passes the migrated embedded foundation fixture without credential-shaped bytes.
- `powershell -NoProfile -File .\tests\acceptance\bootstrap.ps1` passes schema-v2 generic archive_url first-install, corrupt-input, atomic promotion, and zero-source repeat coverage.
- `powershell -NoProfile -File .\golc.ps1 test` passes the complete repository test graph, including unscoped projectconfig load/path cases.
- `powershell -NoProfile -File .\golc.ps1 build` succeeds through the unchanged root entrypoint.
- `rg -n "schema_version = 1|archive_uri" golc.project.toml config internal/projectconfig internal/bootstrap internal/security/redact_test.go internal/command/tools.go internal/command/tools_test.go tests/fixtures/config/walking-skeleton tests/acceptance/bootstrap.ps1 tests/acceptance/walking-skeleton.ps1 golc.ps1` returns no active schema/config/archive-field occurrence; intentional legacy rejection fixtures, if retained, are asserted by test context rather than mistaken for supported configuration.
- A parsed-key audit confirms `config/toolchain.toml` has only `toolchain.go.platforms.windows-amd64` and `toolchain.node.platforms.windows-amd64` platform entries, with no Linux/Darwin/ARM table.
- `git diff -- go.mod go.sum tools/linear-sync/package.json tools/linear-sync/package-lock.json docs README.md` contains no executor-created change.
- `git status --short` still shows the unrelated `.planning/quick/260723-sgy-port-the-full-golc-site-design-language-/` directory untouched.
</verification>

<success_criteria>
- The entire committed configuration authority is schema version 2, never a mixed v1/v2 set.
- Strict validation owns exact platform-qualified dotted keys and rejects every unregistered platform without wildcard support.
- Go and Node use their unchanged Windows AMD64 URLs and checksums only when `PlatformKey()` selects that explicit table.
- Missing future platform pins fail before source/cache/install effects and are not mistaken for configured or supported targets.
- archive_url is consistent across active toolchain and generic-tool readers/fixtures.
- Step 1 bootstrap security, idempotence, lock immutability, and optional Linear behavior remain green.
- The current PowerShell root command remains compatible but is not redirected, removed, or otherwise advanced into Step 3.
- No dependency, documentation, unrelated quick task, or user worktree change is included.
</success_criteria>

<output>
Create `.planning/quick/260723-svv-migrate-toolchain-configuration-and-stri/260723-svv-SUMMARY.md` when done.
</output>
