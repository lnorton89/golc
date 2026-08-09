# GOLC Development

This is the canonical contributor sequence, verified working on Windows,
Linux, and macOS (`.github/workflows/cross-platform-mage.yml` runs the
full graph on `windows-latest`/`ubuntu-latest`/`macos-latest`; see the
[repository README's platform note](../README.md#platform-note) for what
that does and doesn't claim about the GOLC *application*). Every command
runs from the repository root through Mage (`magefiles/magefile.go`),
the sole supported build/bootstrap entrypoint, or the pinned
project-local `golc-project` CLI binary Mage's own bootstrap step
compiles. No ecosystem tool — `npm` or anything else — is ever invoked
directly, and no credentials, `.env` file, or network access is required
after the first bootstrap.

Commands below are shown as POSIX shell (`bash`/`zsh`, for Linux/macOS).
On Windows (PowerShell), `mage <Target>` is identical; direct CLI-binary
invocations use `\`-separated paths, a `.exe` suffix, and the
`windows-amd64` platform directory, e.g.
`.tools/installs/golc_project/<platform>/bin/golc-project`.

**Tip:** `config inspect <concern>`, `config explain <key>`, `test --quick
--scope <name>`, and `docs` below can't become Mage targets — Mage
targets are fixed, no-argument Go functions, and these routes take
genuinely open-ended values (any concern name, any dotted key, any
registered scope). After bootstrap, alias the binary once per shell
session instead of retyping its full path:

```bash
# bash/zsh
alias golc="$(pwd)/.tools/installs/golc_project/<platform>/bin/golc-project"
golc config inspect runtime --format json
```

```powershell
# PowerShell
function golc { & "$PWD\.tools\installs\golc_project\windows-amd64\bin\golc-project.exe" @args }
golc config inspect runtime --format json
```

## 1. Bootstrap once

Install Go and Mage once, ambiently: Mage JIT-compiles the magefile
package at every invocation, so it always needs a Go compiler on PATH,
and there is no way around installing Mage itself before it can take
over. Mage is pinned to the exact version `config/toolchain.toml`
declares (currently 1.17.2) — install it with
`go install github.com/magefile/mage@v1.17.2` or a manual download from
https://github.com/magefile/mage/releases. Then:

```bash
mage Bootstrap
```

Bootstrap provisions the rest of the pinned project-local toolchain from
exact checksum pins in `config/toolchain.toml`, warms the
repository-local Go module cache, and builds the `golc-project` command
every other subcommand below delegates to, at
`.tools/installs/golc_project/<platform>/bin/golc-project` (`.exe` on
Windows; `<platform>` is `windows-amd64`, `linux-amd64`, `linux-arm64`,
`darwin-amd64`, or `darwin-arm64`). Pins are immutable inputs: bootstrap
never upgrades a version and never rewrites `go.mod`, `go.sum`, or
`config/toolchain.toml`. A second bootstrap with matching install
manifests performs zero archive-source calls, and afterwards the
commands below work offline.

## 2. Inspect the committed configuration

`config inspect`/`config set`/`config explain`, `test --quick --scope
<name>`, and `docs` below are not in Mage's fixed target set (`mage
Bootstrap`, `Generate`, `GenerateCheck`, `Check`, `CheckOffline`,
`Build`, `Lint`, `Dev`, `Run`, `Test`, `TestQuick`, `Package`/`PackageFoundation`, `Pr` — see
the [repository README](../README.md#every-mage-target) for what each
one does): they take variable arguments a fixed Mage target descriptor
can't model, so they're invoked directly against the CLI binary
Bootstrap just built.

```bash
golc config inspect runtime --format json
```

`golc.project.toml` is the root configuration index: it owns only schema
and index metadata and points at logically separated concern files
(`config/runtime.toml`, `config/toolchain.toml`). Inspection prints one
concern as deterministic JSON — repeated runs are byte-identical.

## 3. Set a machine-local value

```bash
golc config set --local runtime.log_level debug
```

The value is written only to `golc.local.toml` at the repository root
through atomic replacement. That file is machine-local state: it is
ignored by git and never committed. Writes are strict — unknown keys,
locked keys (pins, hashes, schema versions), path-like keys, and `.env`
targets are all rejected with stable diagnostics.

## 4. Explain the effective value

```bash
golc config explain runtime.log_level --format json
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

```bash
golc test --quick --scope config-local
```

The generic quick-test route translates a registered scope name into the
exact Go test marker `TestScope{PascalName}` (here
`TestScopeConfigLocal`), lists the matching markers first, and fails when
no marker exists. Tests always run through the pinned project-local Go
toolchain, never a host installation.

Property-based tests (`pgregory.net/rapid`, pinned in AGENTS.md's stack
table) generalize a package's hand-picked example tests across arbitrary
generated input instead of one fixed case. Six files are the reference
pattern for adding another:
`internal/projectconfig/reference_property_test.go` (typed reference
resolution), `internal/pool/membership_property_test.go` (the
review-before-apply state machine, via `rapid.T.Repeat`),
`internal/show/store_property_test.go` (SQLite save/load round trip),
`internal/scene/layer_property_test.go` (fixed-priority layer merge),
`internal/artnet/channelmap_property_test.go` (DMX channel packing), and
`internal/playback/evaluate_property_test.go` (chase/motion cue-transition
selection). Each stays in the package's existing external `_test` package,
generalizes an existing fixed example rather than duplicating it, and
checks against an independently-computed oracle rather than reaching into
unexported internals.

## 6. Build the deterministic foundation package

```bash
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

```bash
golc docs
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

## What this walking skeleton was (and how it's grown since)

This section describes the deliberately narrow Phase 1 boundary this
document was originally written for: **CLI -> Go command registry ->
TOML files** stood in for the eventual user-interaction, routing, and
data layers. Commands self-register exact routes into a deterministic
registry (no central switch) — that part is still true and still the
architecture every later phase builds on. The specific scope boundaries
below are historical, not current: see [Running GOLC](../README.md#running-golc)
in the repository README for what actually exists today (a working Wails
desktop shell, SQLite show storage, the full show-authoring/playback CLI
route surface) and [Roadmap](../README.md#roadmap) for what's still
ahead.

Originally out of scope for Phase 1 (now delivered — see
[Roadmap](../README.md#roadmap)):

- **Wails UI** — Phase 6 added the desktop shell, operator surface,
  safety cluster, and playback controls (`cmd/golc-desktop`); it was not
  part of Phase 1.
- **SQLite show storage** — Phase 5 added the single-file, versioned
  `.golc` store with rotating recovery points; Phase 1 only had TOML
  configuration as persisted state.
- **NSIS product packaging** — still genuinely out of scope today
  (Phase 11, "Windows Release Qualification," has not started). Nothing
  is installed or distributed yet. `package --foundation` (step 6 above)
  produces a deterministic developer-tool ZIP of the CLI, config,
  schemas, and docs — it is not an application installer, and it stages
  no Wails frontend or NSIS output.

At Phase 1 time, none of GOLC's lighting-domain behavior existed yet.
Fixture pools/deployments (Phase 2), deterministic show programming and
playback (Phase 3), observable Art-Net output (Phase 4), the versioned
external `/v1` control API (Phase 7), and isolated TypeScript scripting
(Phase 8) shipped as v1.0 (2026-07-27). Front-Door UI Completion
(Phase 9) is implemented but its human UAT pass is still open. The
unified UI design system and automated enforcement work (Phase 13) is
the current focus, 40/41 plans executed. AI autonomy (Phase 10),
Windows Release Qualification (Phase 11), and telemetry (Phase 12)
have not started.

## Optional: AI coding-agent tooling (coldstart, ship-it)

Two additions live outside GOLC's own build graph — neither is required
to build, test, or run the application, and neither is enforced by CI or
the pinned toolchain above.

- **[coldstart](https://coldstartmcp.dev/)** — a Tree-sitter-based
  codebase index and per-agent notebook for AI coding agents. Installed
  globally (`npm install -g @cstart/coldstart`) and wired into this repo
  with `coldstart init` (CLI invocation, Claude Code client). The
  index/notebook live in `.coldstart/` and the hook wiring
  (`PreToolUse`/`PostToolUse`/`UserPromptSubmit`/`Stop`/`SubagentStop`)
  lives in `.claude/settings.local.json` — both gitignored, personal,
  machine-local state, matching coldstart's own default (share the
  notebook later with `coldstart init --commit-notebook` if desired). A
  teammate, or this repo on a different machine, gets none of it
  automatically; run `coldstart init` there to activate it. `coldstart.md`
  (repo root) is the usage reference; it's imported into
  [`.claude/CLAUDE.md`](../.claude/CLAUDE.md) (`@../coldstart.md`) since
  that file — not the root-level `CLAUDE.md` coldstart also generates —
  is the one this repo's harness actually loads as project instructions.
- **[ship-it](https://github.com/LunkiBR/ship-it)** — a markdown-only
  Claude Code skill cataloging UX/product-completeness checklists (login,
  pricing, onboarding, and 56 more). Installed at the user level
  (`~/.claude/skills/ship-it/`), not inside this repo, since it's a
  generic checklist rather than anything GOLC-specific — it's available
  in any project for whoever has it installed on their own machine.
