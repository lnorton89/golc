---
name: go-sum-verification
description: Correct procedure for verifying go.mod/go.sum changes in GOLC — a fresh-cache download/verify/list check, not `go mod tidy` or a plain build. Load before touching Go dependencies.
---

<context>
## Why this exists

GOLC's Go toolchain and module cache are repository-local and pinned
(`config/toolchain.toml`; see [docs/development.md](../../development.md)).
Bootstrap never rewrites `go.mod`, `go.sum`, or the toolchain pins — pins are
immutable inputs (`internal/bootstrap/bootstrap.go`).

`go mod tidy` and a plain `mage Build`/`go build` are the wrong tools for
verifying a `go.sum` change: `tidy` silently rewrites `go.mod`/`go.sum` to
whatever resolves locally, which can mask the exact problem you're trying to
catch, and a plain build reads from the already-warmed repository-local
module cache (`GOMODCACHE`, currently under `.tools/cache/go-mod`), so a
checksum mismatch or an unavailable module version won't surface — the
warm cache already has a (possibly stale-but-locally-valid) copy. A correct
`go.sum` change has to be proven against a cold cache, exactly as CI and a
fresh contributor clone would see it.

This distinction previously caused a correct PR to be wrongly rejected —
`go mod tidy`/`go build` reported no problem locally while the actual
`go.sum` entries were unverifiable from a clean state.
</context>

<procedure>
## Verification procedure

Use the pinned Go binary directly
(`.tools/toolchains/go/<version>/<platform>/go/bin/go[.exe]` — on this repo's
current pin, Windows: `.tools\toolchains\go\1.26.5\windows-amd64\go\bin\go.exe`),
pointed at a **fresh, empty GOPATH/GOMODCACHE** — never the repo-local warm
cache at `.tools/cache/go-mod`.

PowerShell:

```powershell
$fresh = Join-Path $env:TEMP "golc-gosum-check"
Remove-Item $fresh -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $fresh | Out-Null

$go = ".tools\toolchains\go\1.26.5\windows-amd64\go\bin\go.exe"
$env:GOPATH = $fresh
$env:GOMODCACHE = "$fresh\pkg\mod"
$env:GOTOOLCHAIN = "local"

& $go mod download all
& $go mod verify
& $go list -m all
```

bash/zsh (Linux/macOS):

```bash
fresh="$(mktemp -d)"
go="$(pwd)/.tools/toolchains/go/<version>/<platform>/go/bin/go"
GOPATH="$fresh" GOMODCACHE="$fresh/pkg/mod" GOTOOLCHAIN=local "$go" mod download all
GOPATH="$fresh" GOMODCACHE="$fresh/pkg/mod" GOTOOLCHAIN=local "$go" mod verify
GOPATH="$fresh" GOMODCACHE="$fresh/pkg/mod" GOTOOLCHAIN=local "$go" list -m all
```

All three must succeed cleanly:
- `mod download all` — proves every module version in `go.sum` is actually
  fetchable from a cold cache.
- `mod verify` — proves the downloaded content matches the checksums
  recorded in `go.sum`.
- `list -m all` — proves the full dependency graph resolves without
  ambiguity.

Clean up the temp dir afterwards; it's disposable.

### `go mod download all` REWRITES `go.sum` — check `git status` afterwards

`go mod download all` is a write command. Running it re-adds roughly 234
`go.sum` entries for the full module graph — including test-only and tool
dependencies of dependencies — that commit `e0ab3b5b` ("remove ~234 unused
go.sum entries") deliberately pruned. `-mod=readonly` does **not** prevent
this; it constrains `go.mod`, not `go.sum`.

So the procedure above leaves the working tree dirty even when nothing is
wrong, and that residue is easy to mistake for someone else's edit or to
commit by accident. Always `git diff --stat go.sum` when it finishes, and
`git checkout -- go.sum` if the only change is that re-expansion.

### Verifying that a *minimal* `go.sum` is sufficient

Because of the above, `go mod download all` cannot answer "does this pruned
`go.sum` contain everything a build needs?" — it answers by adding whatever
was missing. Use a cold-cache build instead, which makes a missing hash a
hard error and never writes:

```powershell
$fresh = Join-Path $env:TEMP "golc-gosum-check"
Remove-Item $fresh -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $fresh | Out-Null

$go = ".tools\toolchains\go\1.26.5\windows-amd64\go\bin\go.exe"
$env:GOPATH = $fresh
$env:GOMODCACHE = "$fresh\pkg\mod"
$env:GOTOOLCHAIN = "local"
$env:GOFLAGS = "-mod=readonly"

foreach ($os in @("windows","linux","darwin")) {
  $env:GOOS = $os
  & $go build ./...
  Write-Output "$os -> exit $LASTEXITCODE"
}
& $go mod verify
```

Two gotchas when reading that output:

- **Don't pipe the Go binary through `2>&1` in PowerShell.** It wraps each
  `go: downloading …` stderr line as a `NativeCommandError` and sets `$?`
  to `$false` even on a clean exit. Check `$LASTEXITCODE`, not `$?`.
- **`./...` cannot cross-compile on this repo today.** `internal/wails`
  links `golang.design/x/hotkey`, whose Linux/macOS backends need cgo, so a
  `GOOS=linux`/`darwin` build from Windows fails on `undefined:
  hotkey.ModCtrl` and friends. That is a pre-existing platform limitation,
  not a `go.sum` problem — scope the cross-platform check to the packages
  you changed (`go build ./internal/<pkg>/`) and use `go vet` per GOOS.

### Adding a new dependency

`go get <module>@<version>` is the right tool, but note it leaves the new
module marked `// indirect` in `go.mod` until something imports it, and
re-running `go get` afterwards does **not** clear that marker. Move the
require line into the direct block and drop the comment by hand rather than
reaching for `go mod tidy`. If the dependency has platform-specific
transitive deps (gopsutil is the live example: `yusufpapurcu/wmi` on
Windows, `tklauser/*` on Linux, `ebitengine/purego` on macOS), run the
`go get` once per `GOOS` or the other platforms' builds will fail on a
missing `go.sum` entry.

## What not to do

- Don't run `go mod tidy` to "fix" a `go.sum` question — it rewrites the
  files instead of verifying them, and can hide the actual discrepancy.
- Don't trust `mage Build`, `mage Test`, or a plain `go build ./...` as
  proof — they use the warmed repo-local `GOMODCACHE` and won't catch a
  cache/checksum problem a fresh clone or CI runner would hit.
</procedure>
