# Deferred Items — Phase 8

Out-of-scope discoveries logged during plan execution (not fixed, per the
SCOPE BOUNDARY rule: only issues directly caused by the current task's
changes are auto-fixed).

## 08-01

- **Pinned Go toolchain not bootstrapped in this worktree.** `.tools/` does
  not exist here, so `TestBuildRouteCompilesTheProductionRepository`,
  `TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
  `TestScopeGreenSubprocess`, and `TestScopeOfflineAcceptance` in
  `internal/command` fail with `GOLC_TEST_TOOLCHAIN_MISSING` / "pinned
  golc-project binary not built (run mage Bootstrap first)". This is a
  pre-existing environment condition (this worktree never ran `mage
  Bootstrap`), unrelated to 08-01's `internal/show/scripts.go` /
  `internal/command/script.go` changes — every other test in
  `internal/show`, `internal/command`, and `internal/routecatalog` passes.
  Not fixed here; run `mage Bootstrap` in this worktree (or verify in a
  bootstrapped environment) before relying on those five tests.

- **`go build ./...` fails on `cmd/golc-desktop`, unrelated to this
  plan.** `cmd\golc-desktop\main.go:28:12: pattern all:frontend/dist: no
  matching files found` — the desktop binary's `//go:embed` directive
  requires a built `frontend/dist` that does not exist in this worktree
  (no frontend build has run here). `go build ./internal/...
  ./cmd/golc-project/...` (the packages 08-01 actually touches) builds
  cleanly. Not fixed here; run the frontend build (or verify in an
  environment where it already ran) before relying on a whole-repo
  `go build ./...`.
