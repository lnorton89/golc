# Deferred Items

Pre-existing, out-of-scope issues discovered during phase execution but not fixed (per executor SCOPE BOUNDARY rule: only auto-fix issues directly caused by the current task's changes).

## From Plan 06-02 execution

Discovered while running `go test -race ./...` as a broader sanity check after Task 3 (06-02 touches only `internal/artnet/*` and `internal/command/artnet*.go` — confirmed via `git diff --stat <base>..HEAD -- internal/trace` returning empty, i.e. zero overlap with these failures).

- **`internal/trace/catalog` — `TestScopeLinearCatalog` / `TestScopeLinearMap` fail**: `BuildCatalog: GOLC_CATALOG_ID_INVALID: requirement key "TBD" does not match the KEY-NN grammar`. Three subtests of `TestScopeLinearCatalog` and one subtest of `TestScopeLinearMap` fail against the real repository catalog. Unrelated to Art-Net safety overrides; last touched in Phase 1 commits, not this phase.
- **`internal/trace/transport` — `TestScopeTraceTransportProcess` flaky under `-race`**: a genuine data race between `ProcessClient.terminate`'s `exec.Cmd.Wait()` and `ProcessClient.safeFailureSummary()`'s concurrent read of the same field (`internal/trace/transport/process.go` lines ~217/252). Reproducible via `go test -race ./internal/trace/transport/...`. Unrelated to Art-Net safety overrides; last touched in Phase 1 commits, not this phase.

Both should be triaged and fixed in a dedicated `internal/trace` maintenance plan, not folded into Phase 6 UI/operator-surface work.

## From Plan 06-08 execution

- **`golc-desktop.exe` required `midicat` on the OS-level PATH merely to start, not just to use MIDI hardware — reported live by a contributor whose fresh checkout hit the documented panic.** `06-08-SUMMARY.md`'s Decisions Made section fully diagnosed the root cause at the time: `gitlab.com/gomidi/midi/v2/drivers/midicatdrv`'s package `init()` calls Go's `panic()` (not a returnable error) when the `midicat` binary is missing, and Go runs every imported package's `init()` unconditionally before `main()` starts — unrecoverable from application code. The risk was correctly identified and written up at the time but never propagated into a tracked backlog, so it sat undiscoverable outside one phase-completion doc until it actually crashed a contributor.

  **Fixed:**
  1. `mage Bootstrap` now provisions `midicat` automatically (`internal/bootstrap/engine.go`'s `installGoInstallTools`, pinned in `config/toolchain.toml`'s `[go_install.midicat]`), landing it at `.tools/cache/go-bin/midicat.exe` via `go install gitlab.com/gomidi/tools/midicat@v1.0.7` using the pinned, verified Go toolchain. Best-effort: a network hiccup reaching `gitlab.com` warns and continues bootstrap rather than failing it, matching MIDI's existing "hardware support remains optional" posture.
  2. `mage Run` (`internal/command/run.go`) closes the PATH-at-process-start gap for the dev-loop case: it execs the already-built `golc-desktop[.exe]` as a child process with `.tools/cache/go-bin` prepended onto that child's own PATH, so the go-install-provisioned `midicat` resolves regardless of the invoking shell's own PATH.

  **Still open — the packaged end-user launcher case.** A contributor (or end user) who runs the compiled `golc-desktop[.exe]` directly — double-clicked, or invoked from a fresh shell, bypassing `mage Run` — still inherits whatever PATH that shell/launcher has, entirely independent of anything Bootstrap or `mage Run` did. Nothing outside the process that execs `golc-desktop` can rewrite the environment it starts with. A full fix for the packaged Windows release needs a genuine launcher/wrapper: a small separate binary (that does **not** itself import `midicatdrv`) sets PATH and then execs (or `CreateProcess`+waits for) the real `golc-desktop` binary as a child process — mirroring `internal/artnet`'s own supervised-child-process pattern. This is real design work suitable for its own discuss-phase, likely folded into Phase 10 (Windows Release Qualification), where packaging/installation is already in scope.
