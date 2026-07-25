# GOLC Development

This is the canonical contributor sequence for the Phase 1 walking
skeleton. Every command runs from the repository root through Mage
(`magefiles/magefile.go`), the sole supported build/bootstrap entrypoint,
or the pinned project-local `golc-project` CLI binary Mage's own
bootstrap step compiles. No ecosystem tool — `npm` or anything else — is
ever invoked directly, and no credentials, `.env` file, or network access
is required after the first bootstrap.

## 1. Bootstrap once

Install Go and Mage once, ambiently: Mage JIT-compiles the magefile
package at every invocation, so it always needs a Go compiler on PATH,
and there is no way around installing Mage itself before it can take
over. Mage is pinned to the exact version `config/toolchain.toml`
declares (currently 1.17.2) — install it with
`go install github.com/magefile/mage@v1.17.2` or a manual download from
https://github.com/magefile/mage/releases. Then:

```powershell
mage Bootstrap
```

Bootstrap provisions the rest of the pinned project-local toolchain from
exact checksum pins in `config/toolchain.toml`, warms the
repository-local Go module cache, and builds the `golc-project` command
every other subcommand below delegates to, at
`.tools/installs/golc_project/<platform>/bin/golc-project` (`.exe` on
Windows). Pins are immutable inputs: bootstrap never upgrades a version
and never rewrites `go.mod`, `go.sum`, or `config/toolchain.toml`. A
second bootstrap with matching install manifests performs zero
archive-source calls, and afterwards the commands below work offline.

## 2. Inspect the committed configuration

`config inspect`/`config set`/`config explain`, the quick-test route, and
`docs` below are not in Mage's fixed target set (`mage Bootstrap`,
`GenerateCheck`, `CheckOffline`, `Build`, `Test`, `PackageFoundation`,
`Pr`): they take variable arguments a fixed Mage target descriptor can't
model, so they're invoked directly against the CLI binary Bootstrap just
built.

```powershell
.\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe config inspect runtime --format json
```

`golc.project.toml` is the root configuration index: it owns only schema
and index metadata and points at logically separated concern files
(`config/runtime.toml`, `config/toolchain.toml`). Inspection prints one
concern as deterministic JSON — repeated runs are byte-identical.

## 3. Set a machine-local value

```powershell
.\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe config set --local runtime.log_level debug
```

The value is written only to `golc.local.toml` at the repository root
through atomic replacement. That file is machine-local state: it is
ignored by git and never committed. Writes are strict — unknown keys,
locked keys (pins, hashes, schema versions), path-like keys, and `.env`
targets are all rejected with stable diagnostics.

## 4. Explain the effective value

```powershell
.\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe config explain runtime.log_level --format json
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
.\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe test --quick --scope config-local
```

The generic quick-test route translates a registered scope name into the
exact Go test marker `TestScope{PascalName}` (here
`TestScopeConfigLocal`), lists the matching markers first, and fails when
no marker exists. Tests always run through the pinned project-local Go
toolchain, never a host installation.

## 6. Build the deterministic foundation package

```powershell
mage PackageFoundation
```

`package --foundation` (Mage's `PackageFoundation` target) builds a
**developer-tool bundle, not a product installer**: a Windows AMD64 ZIP
containing the bootstrap-built `golc-project.exe`, `golc.project.toml`,
every committed `config/**/*.toml` concern, every committed
`schemas/*.json` contract, and `docs/development.md`. Output is written
to `dist/foundation/`:

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
`internal/delivery`'s "BuildFoundationBundle produces byte-identical ZIP,
manifest, and checksums across repeated runs" test proves this by
building the bundle twice and comparing all three outputs byte-for-byte.

This command makes **no Wails or NSIS product-packaging claim** — see the
boundary below.

## 7. Generate the package reference docs

```powershell
.\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe docs
```

`docs` (internal/docgen) walks every `internal/**` package, extracts the
one file whose leading comment starts with `Package <name> ...`, and
writes a Markdown reference page per documented package to
`docs/reference/` plus an identical copy to `site/src/content/reference/`
so the marketing site's Docs page can render the same content
statically. A package with no such comment is silently skipped, and a
package that disappears (or loses its comment) has its stale page
removed on the next run — the command is safe to re-run at any time and
always reflects only the current source. Because `site/` is a separate
git repository checked out as a submodule, its copy needs its own commit
there before the parent repository's submodule pointer is updated.

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
