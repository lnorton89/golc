// midi_driver.go isolates the blank import of
// gitlab.com/gomidi/midi/v2/drivers/midicatdrv to this cmd/golc-desktop
// main package ONLY (06-08-PLAN.md Task 2/3 wiring) -- internal/midi and
// internal/wails must never import it (see internal/midi/driver.go's own
// doc comment): midicatdrv's package init() shells out to
// `midicat version` and calls panic() -- not a returnable error -- when
// the binary is missing from PATH or the wrong version, and Go runs
// every imported package's init() unconditionally before main() starts.
// Isolating the import to this file (a `main` package with no test
// files) means `go test ./...` never triggers it; only the compiled
// golc-desktop.exe binary is affected, and it now requires midicat.exe to
// be present on PATH merely to START -- a real, load-bearing limitation
// of midicatdrv's own upstream design that this project could not route
// around from within the same binary (see 06-08-SUMMARY.md Decisions
// Made for the full analysis, and
// .planning/phases/06-wails-authoring-and-operator-surface/
// deferred-items.md for the remaining launcher-wrapper gap this doc
// comment describes below).
//
// `mage Bootstrap` now provisions midicat automatically
// (internal/bootstrap/engine.go's installGoInstallTools, pinned in
// config/toolchain.toml's [go_install.midicat]) via
// `go install gitlab.com/gomidi/tools/midicat@v1.0.7` using the
// already-verified, pinned Go toolchain -- checksum-verified through
// Go's own module proxy/sumdb, never a downloaded/pinned binary --
// landing at .tools/cache/go-bin/midicat.exe (the same project-local
// GOBIN cache.go's WailsBinaryPath already reserves for exactly this
// "go install a pinned tool" pattern). Provisioning is best-effort, not
// bootstrap-fatal, since MIDI hardware support remains optional.
//
// That closes the "how do I get midicat at all" gap, but not the
// "how does golc-desktop.exe find it at process start" gap on its own:
// midicatdrv's own exec_windows.go/exec_unix.go locate the binary via a
// bare `midicat`/`midicat.exe` PATH lookup (not an absolute path or env
// var), and Go application code cannot modify its own process's PATH
// before a transitively-imported package's init() runs. `mage Run`
// (internal/command/run.go) closes that gap for the dev-loop case: it
// execs the already-built golc-desktop[.exe] as a child process with
// .tools/cache/go-bin prepended onto that child's own PATH. Running the
// compiled binary directly instead of through `mage Run` still inherits
// whatever PATH the invoking shell has, independent of anything
// Bootstrap did -- see deferred-items.md above for the remaining
// packaged-end-user-launcher gap that boundary leaves open.
package main

import (
	_ "gitlab.com/gomidi/midi/v2/drivers/midicatdrv"
)
