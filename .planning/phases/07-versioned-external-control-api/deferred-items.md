# Deferred Items — Phase 07

Out-of-scope discoveries logged during plan execution (Scope Boundary rule:
only auto-fix issues directly caused by the current task's changes).

## 07-12: pinned toolchain not bootstrapped in this worktree

**Discovered during:** 07-12 Task 2 verification (`go test ./internal/command/...`)

**Issue:** `internal/command`'s `TestBuildRouteCompilesTheProductionRepository`,
`TestBuildablePackagesExcludesMagefiles`, `TestScopeCrossPlatformCI`,
`TestScopeGreenSubprocess`, and `TestScopeOfflineAcceptance` fail with
`GOLC_TEST_TOOLCHAIN_MISSING: ... run 'mage Bootstrap' first` and
`pinned golc-project binary not built`. This worktree
(`C:\Users\Lawrence\Documents\Dev\golc\.claude\worktrees\agent-ab52ad4e29219d28e`)
has never had `mage Bootstrap` run against it, so `.tools/toolchains/go` and
`.tools/installs/golc_project` do not exist.

**Why deferred:** Entirely unrelated to this plan's file scope
(`internal/api/batch.go`, `internal/api/mutate.go`,
`internal/api/batch_test.go`). Bootstrapping a fresh worktree's pinned
toolchain is an environment-provisioning step, not a code defect introduced
by this plan's changes. `go test ./internal/api/... ./internal/show/...
./internal/artnet/... ./internal/artnet/ipc/...` all pass; `mage testquick`
was not run in this worktree for the same reason (mage itself is on PATH,
but its targets depend on the same unbootstrapped toolchain/binary).

**Action:** None taken here. If a future plan or CI run needs this worktree
green on `internal/command`'s CI-shape tests, run `mage Bootstrap` first.
