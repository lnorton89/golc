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
// of midicatdrv's own upstream design that this plan could not route
// around (see 06-08-SUMMARY.md Decisions Made for the full analysis and
// a documented follow-up option). The midicat binary itself is installed
// via `go install gitlab.com/gomidi/tools/midicat@v1.0.7` (Task 0's
// human-approved checkpoint refinement -- go's own module-proxy checksum
// verification, never a downloaded/pinned binary), landing on
// `$(go env GOPATH)/bin/midicat.exe`, which midicatdrv's own
// exec_windows.go locates via a bare `midicat.exe` PATH lookup (not an
// absolute path or env var) -- so GOPATH/bin must be on PATH for the
// desktop binary to find it.
package main

import (
	_ "gitlab.com/gomidi/midi/v2/drivers/midicatdrv"
)
