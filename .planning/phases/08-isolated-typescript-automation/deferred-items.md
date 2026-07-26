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
