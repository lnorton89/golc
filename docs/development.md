# GOLC Development

This is the canonical contributor sequence for the Phase 1 walking
skeleton. Every command runs from the repository root through the single
supported entrypoint, `golc.ps1` (Windows PowerShell 5.1). No ecosystem
tool — `go`, `npm`, or anything else — is ever invoked directly, and no
credentials, `.env` file, or network access is required after the first
bootstrap.

## 1. Bootstrap once

```powershell
powershell -NoProfile -File .\golc.ps1 bootstrap
```

Bootstrap provisions the pinned project-local toolchain from exact
checksum pins in `config/toolchain.toml`, warms the repository-local Go
module cache, and builds the `golc-project` command that every other
subcommand delegates to. Pins are immutable inputs: bootstrap never
upgrades a version and never rewrites `go.mod`, `go.sum`, or
`config/toolchain.toml`. A second bootstrap with matching install
manifests performs zero archive-source calls, and afterwards the commands
below work offline.

## 2. Inspect the committed configuration

```powershell
powershell -NoProfile -File .\golc.ps1 config inspect runtime --format json
```

`golc.project.toml` is the root configuration index: it owns only schema
and index metadata and points at logically separated concern files
(`config/runtime.toml`, `config/toolchain.toml`). Inspection prints one
concern as deterministic JSON — repeated runs are byte-identical.

## 3. Set a machine-local value

```powershell
powershell -NoProfile -File .\golc.ps1 config set --local runtime.log_level debug
```

The value is written only to `golc.local.toml` at the repository root
through atomic replacement. That file is machine-local state: it is
ignored by git and never committed. Writes are strict — unknown keys,
locked keys (pins, hashes, schema versions), path-like keys, and `.env`
targets are all rejected with stable diagnostics.

## 4. Explain the effective value

```powershell
powershell -NoProfile -File .\golc.ps1 config explain runtime.log_level --format json
```

Explain resolves the key across the layers and reports which layer won,
the safe source file name, and the ordered shadowed origins:

```json
{"key":"runtime.log_level","layer":"project-local","shadowed":[{"layer":"committed","source":"config/runtime.toml","value":"info"}],"source":"golc.local.toml","value":"debug"}
```

Repeated calls with unchanged inputs are byte-identical, and the output
contains only the allowlisted fields above — never environment variables
or credentials. Because the local value lives on disk, a new process (a
fresh terminal, a fresh build) resolves the same answer.

## 5. Run the quick tests for a scope

```powershell
powershell -NoProfile -File .\golc.ps1 test --quick --scope config-local
```

The generic quick-test route translates a registered scope name into the
exact Go test marker `TestScope{PascalName}` (here
`TestScopeConfigLocal`), lists the matching markers first, and fails when
no marker exists. Tests always run through the pinned project-local Go
toolchain, never a host installation.

## 6. Build the deterministic foundation package

```powershell
powershell -NoProfile -File .\golc.ps1 package --foundation
```

`package --foundation` builds a **developer-tool bundle, not a product
installer**: a Windows AMD64 ZIP containing the bootstrap-built
`golc-project.exe`, the `golc.ps1` shim, `golc.project.toml`, every
committed `config/**/*.toml` concern, every committed `schemas/*.json`
contract, and `docs/development.md`. Output is written to
`dist/foundation/`:

- `golc-foundation-windows-amd64.zip` — the archive itself.
- `golc-foundation-windows-amd64.manifest.json` — a canonical, sorted
  inventory of every archived file's path, SHA-256, and size (also
  embedded inside the ZIP as `foundation-manifest.json`).
- `golc-foundation-windows-amd64.zip.sha256` — the archive's own SHA-256
  checksum, in the standard `<hex>  <filename>` sidecar shape.

Every entry's ZIP metadata (path, mode, timestamp) is normalized, and the
file list is a fixed, sorted allowlist rather than an unbounded directory
walk: identical repository inputs always produce byte-identical ZIP,
manifest, and checksum bytes. `dist/foundation/` is regenerated on every
run and is git-ignored; the only committed foundation-package fixture is
the golden test oracle at `tests/golden/foundation-manifest.json`.
`tests/acceptance/offline.ps1 -Mode package` proves this by running the
command twice and comparing all three output files byte-for-byte.

This command makes **no Wails or NSIS product-packaging claim** — see the
boundary below.

## What this walking skeleton is (and is not)

The Phase 1 adaptation of GOLC's architecture is deliberately narrow:
**CLI -> Go command registry -> TOML files** stands in for the eventual
user-interaction, routing, and data layers. Commands self-register exact
routes into a deterministic registry (no central switch), and committed
TOML concerns plus one ignored local file form the entire data layer.

Explicitly out of scope for Phase 1:

- **Wails UI** — there is no desktop shell or frontend; the CLI is the
  only user interaction surface.
- **SQLite show storage** — no `.golc` database exists; TOML
  configuration is the only persisted state.
- **NSIS product packaging** — nothing is installed or distributed.
  `package --foundation` (step 6 above) produces a deterministic
  developer-tool ZIP of the CLI, config, schemas, and docs — it is not an
  application installer, and it stages no Wails frontend or NSIS output.

Lighting-domain behavior, playback, Art-Net, scripting, and AI features
are later phases and are not part of this skeleton.
