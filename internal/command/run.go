// run.go is the run command file: it owns the "run" scope and
// self-registers the "run" route (CONTEXT D-03) — the one dev-loop
// entrypoint that actually closes the "midicat provisioned on disk but
// not on PATH at golc-desktop's own process-exec time" gap described in
// internal/midi/driver.go and cmd/golc-desktop/midi_driver.go's doc
// comments.
//
// The gap, precisely: internal/bootstrap.installGoInstallTools (engine.go)
// provisions midicat via `go install` into the project-local
// .tools/cache/go-bin directory (bootstrap.ProjectCacheLayout.GoBin) —
// but that only puts the binary on disk. midicatdrv's own exec_unix.go/
// exec_windows.go resolve it with a bare `exec.Command("midicat", ...)`
// PATH lookup, evaluated against whatever PATH the golc-desktop *process*
// happens to have at the exact moment it starts. A contributor who runs
// `mage Bootstrap && mage Build` and then launches the compiled binary
// directly (`./golc-desktop` or `.\golc-desktop.exe`) gets none of that:
// the shell invoking the binary has its own PATH, entirely independent of
// where bootstrap put anything, and midicatdrv's package init() panics
// before main() ever runs — confirmed live in a nix-shell where
// `$(go env GOPATH)/bin` (wherever a manual `go install midicat` landed
// it) was not on PATH either. No code inside golc-desktop's own main()
// can fix this after the fact: Go runs every imported package's init()
// unconditionally before main() starts, so by the time main() could
// touch os.Setenv("PATH", ...), midicatdrv's init() has already run (or
// already panicked).
//
// "run" closes this gap for exactly the case it CAN control: a
// mage-dispatched dev-loop launch. It execs the already-built
// golc-desktop[.exe] as a child process with .tools/cache/go-bin
// prepended onto that child's own PATH before Start() — the same
// GoBin directory installGoInstallTools provisions midicat into — so the
// child process's PATH lookup succeeds regardless of what the invoking
// shell's PATH looks like (nix-shell, a fresh terminal, CI, or otherwise).
//
// Boundary this does NOT close, on purpose: a contributor who bypasses
// `mage Run` and executes the compiled binary directly still inherits
// whatever PATH their own shell has, with no mage-controlled process in
// between to fix it up. That is unavoidable — nothing outside the
// process that execs golc-desktop can rewrite the environment it starts
// with. README.md's "MIDI requires midicat" section documents the manual
// fallback (add .tools/cache/go-bin, or wherever `go install` placed
// midicat, to PATH yourself) for that direct-launch case explicitly,
// rather than leaving it as a silent gap.
package command

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lnorton89/golc/internal/bootstrap"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "run",
	Summary: "Launch the already-built golc-desktop binary with the project-local go-install bin directory on PATH.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "run",
	Summary: "Run the built golc-desktop[.exe] with .tools/cache/go-bin prepended onto its PATH, so go-install-" +
		"provisioned tools like midicat resolve at process start regardless of the invoking shell's own PATH: run.",
	Handler: runRun,
})

// runRun serves the self-registered "run" route. It takes no arguments:
// there is exactly one thing to run (the desktop app), matching build.go's
// bare-invocation default and package.go's single-purpose route shape.
func runRun(request Request) Result {
	if len(request.Args) != 0 {
		return Result{ExitCode: 2, Stderr: []byte("GOLC_RUN_USAGE: usage: run\n")}
	}

	desktopExecutable := filepath.Join(request.Root, bootstrap.ExecutableName("golc-desktop"))
	if info, err := os.Stat(desktopExecutable); err != nil || !info.Mode().IsRegular() {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_RUN_BINARY_MISSING: %s: run 'mage Build' first\n", desktopExecutable))}
	}

	layout, err := bootstrap.NewProjectCacheLayout(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_RUN_CACHE_LAYOUT: %v\n", err))}
	}

	execution := exec.Command(desktopExecutable)
	execution.Dir = request.Root
	execution.Env = prependPathDirectory(os.Environ(), layout.GoBin)
	// Streamed directly rather than buffered into Result (unlike build.go/
	// test.go's short-lived invocations): golc-desktop is a long-running
	// GUI process, and buffering everything until it exits would silence
	// exactly the startup diagnostic (a PATH-lookup panic from
	// midicatdrv's init()) this route exists to surface promptly.
	execution.Stdin = os.Stdin
	execution.Stdout = os.Stdout
	execution.Stderr = os.Stderr

	runErr := execution.Run()
	if runErr == nil {
		return Result{}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode()}
	}
	return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_RUN_FAILED: %v\n", runErr))}
}

// prependPathDirectory returns a copy of environment with dir prepended
// onto the PATH entry (case-insensitive match, so Windows' "Path" is
// recognized too), or a new PATH entry appended if none exists. This is
// the one line of this file that actually closes the PATH-at-exec-time
// gap described in this file's doc comment: it changes what the CHILD
// process (golc-desktop) sees, not merely what gets documented.
func prependPathDirectory(environment []string, dir string) []string {
	result := make([]string, 0, len(environment)+1)
	found := false
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, "PATH") {
			found = true
			result = append(result, name+"="+dir+string(os.PathListSeparator)+value)
			continue
		}
		result = append(result, entry)
	}
	if !found {
		result = append(result, "PATH="+dir)
	}
	return result
}
