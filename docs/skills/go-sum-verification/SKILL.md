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

## What not to do

- Don't run `go mod tidy` to "fix" a `go.sum` question — it rewrites the
  files instead of verifying them, and can hide the actual discrepancy.
- Don't trust `mage Build`, `mage Test`, or a plain `go build ./...` as
  proof — they use the warmed repo-local `GOMODCACHE` and won't catch a
  cache/checksum problem a fresh clone or CI runner would hit.
</procedure>
