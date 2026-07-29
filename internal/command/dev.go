// dev.go is the dev command file: it owns the "dev" scope and
// self-registers the "dev" route (CONTEXT D-03) -- the hot-reload sibling
// to run.go's "run" (mage Dev, matching mage Run's own naming).
//
// `wails dev` hard-requires wails.json to sit in the SAME directory as the
// Go package with the wails.Run() call (internal/project.Project has no
// field for "main package elsewhere" -- confirmed against the Wails v2
// source: cmd/wails/flags/dev.go's loadAndMergeProjectConfig calls
// project.Load(cwd), and pkg/commands/bindings/bindings.go's
// GenerateBindings runs `go build .` in that identical cwd with no
// override). cmd/golc-desktop/wails.json (not a repo-root wails.json)
// reflects this: its own "frontend:dir": "../../frontend" and
// "wailsjsdir": "../../frontend/wailsjs" point back at the repo-root
// frontend/ directory this project actually uses, since frontend/ is not
// wails.json's sibling here the way a plain `wails init` scaffold expects.
// execution.Dir below is cmd/golc-desktop for exactly this reason -- not
// request.Root, which has no Go files of its own and previously failed
// bindings generation with "no Go files in <root>" when this route ran
// from the wrong directory.
//
// `wails dev` itself shells out to bare `go` and `npm` on PATH internally;
// unlike every other route in this file, it is not parameterized with
// explicit toolchain paths. Closing the "never a host PATH lookup" gap
// (CONTEXT D-01/D-02) here means prepending three project-local
// directories onto the child's PATH, not just one the way run.go does for
// midicat: layout.GoBin (the pinned Wails CLI itself, plus midicat), the
// pinned Go toolchain's own bin directory, and the pinned Node
// installation's directory (the official Node distribution ships
// npm/npm.cmd alongside the node binary itself, so prepending its
// directory resolves a bare `npm run dev` to the pinned Node, not a host
// one).
package command

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/lnorton89/golc/internal/bootstrap"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "dev",
	Summary: "Launch `wails dev` (hot-reload desktop dev loop) with the pinned Go/Node/Wails toolchains on PATH.",
})

// windowsResourceSyso is the checked-in Windows resource file mage Build's
// plain `go build` needs to embed the app/titlebar icon (see build.go's own
// -tags desktop,production doc comment for why that route never invokes
// Wails' packaging step at all: it links cmd/golc-desktop with a bare `go
// build`). `wails dev` is a different build path entirely: Wails v2.13.0
// hardcodes Pack:true for dev builds (cmd/wails/flags/dev.go's
// GenerateBuildOptions -- no CLI flag disables it), so on EVERY dev
// rebuild it regenerates its own resource file at
// cmd/golc-desktop/golc-desktop-res.syso from the same
// build/windows/icon.ico + wails.exe.manifest + info.json inputs
// (pkg/commands/build/packager.go's compileResources), already at the
// correct titlebar icon resource ID (tc-hib/winres's RT_ICON constant is
// 3, the same ID winc.AppIconID expects -- the ID 0ed088a8 fixed for this
// checked-in file), and deletes it again once that build finishes
// (build.go's execBuildApplication, deferred). Go auto-links every *.syso
// file present in a package directory, and the linker accepts only one
// .rsrc section per binary, so the brief window where BOTH files coexist
// fails every single `mage dev` with "too many .rsrc sections"
// (root-caused in .planning/debug/resolved/too-many-rsrc-sections.md).
// The checked-in file is redundant for this path -- Wails' own generated
// one already carries the right icon at the right ID -- so this route
// disables it for the duration of the `wails dev` child process instead
// of trying to reconcile the two resource files.
const windowsResourceSyso = "rsrc_windows_amd64.syso"

// disabledSysoSuffix intentionally does not end in ".syso": Go's *.syso
// auto-link rule matches on filename suffix alone, so appending this is
// enough to drop the file out of the package's link inputs without
// deleting it.
const disabledSysoSuffix = ".wails-dev-disabled"

// disableWindowsResourceSyso renames windowsResourceSyso out of Go's
// *.syso auto-link path for the duration of the `wails dev` child process
// (see windowsResourceSyso's doc comment for why), returning a restore
// func the caller must invoke exactly once when that process exits. A
// missing file is not an error (mirrors this file's other defensive
// os.Stat guards): a synthetic test fixture, or a future checkout where
// the checked-in resource is dropped entirely, still runs cleanly.
func disableWindowsResourceSyso(desktopDir string) (func() error, error) {
	original := filepath.Join(desktopDir, windowsResourceSyso)
	disabled := original + disabledSysoSuffix
	if _, err := os.Stat(original); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return func() error { return nil }, nil
		}
		return nil, fmt.Errorf("stat %s: %w", original, err)
	}
	if err := os.Rename(original, disabled); err != nil {
		return nil, fmt.Errorf("disable %s for wails dev: %w", original, err)
	}
	return func() error {
		if err := os.Rename(disabled, original); err != nil {
			return fmt.Errorf("restore %s after wails dev: %w", original, err)
		}
		return nil
	}, nil
}

var _ = MustDeclareRoute(CommandRegistration{
	Route: "dev",
	Summary: "Run `wails dev` with the project-local pinned Go/Node/Wails toolchains prepended onto its PATH, so its " +
		"own internal go/npm invocations resolve to the pinned toolchains regardless of the invoking shell's own PATH: dev.",
	Handler: runDev,
})

// runDev serves the self-registered "dev" route. It takes no arguments:
// there is exactly one thing to run (the dev-mode desktop app), matching
// run.go's "run".
func runDev(request Request) Result {
	if len(request.Args) != 0 {
		return Result{ExitCode: 2, Stderr: []byte("GOLC_DEV_USAGE: usage: dev\n")}
	}

	layout, err := bootstrap.NewProjectCacheLayout(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_DEV_CACHE_LAYOUT: %v\n", err))}
	}

	executableSuffix := ""
	if runtime.GOOS == "windows" {
		executableSuffix = ".exe"
	}
	wailsExecutable := layout.WailsBinaryPath(executableSuffix)
	if info, err := os.Stat(wailsExecutable); err != nil || !info.Mode().IsRegular() {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_DEV_WAILS_MISSING: %s: run 'mage Bootstrap' first\n", wailsExecutable))}
	}

	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	node, err := resolvePinnedNodeInstallation(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	// `wails dev` shells out to bare `go` (to rebuild/run the Go backend on
	// change) and bare `npm` (wails.json's own frontend:dev:watcher = "npm
	// run dev") -- neither is parameterized with an explicit toolchain path
	// the way this file's other routes invoke Go/Node directly, so the only
	// way to keep it on the pinned toolchains is PATH. projectGoEnvironment
	// already sets GOTOOLCHAIN=local/GOPROXY=off/GOMODCACHE/GOCACHE/
	// GOFLAGS=-mod=readonly the same way every other pinned-toolchain
	// invocation in this file does.
	environment := projectGoEnvironment(request.Root)
	environment = prependPathDirectory(environment, filepath.Dir(node.Executable))
	environment = prependPathDirectory(environment, filepath.Dir(goExecutable))
	environment = prependPathDirectory(environment, layout.GoBin)

	// -m (SkipModTidy) is required, not optional: without it, `wails dev`
	// unconditionally runs `go mod tidy` before every bindings-generation
	// pass (pkg/commands/bindings/bindings.go's GenerateBindings), which
	// rewrites go.sum -- against a build-tag-filtered module graph (tidy
	// runs without the "desktop"/"production" tags cmd/golc-desktop's own
	// files are gated behind), so it can strip checksums a full
	// -tags desktop,production build still needs. Observed live: a single
	// `mage Dev` run deleted 240 lines from go.sum. This project's own D-04
	// invariant is explicit elsewhere (cache.go's WailsModule/WailsVersion
	// doc comment: "Only an explicit `tools update` command may ever change
	// the Wails pin"; engine_frontend.go's runFrontendBuild defends
	// package.json/package-lock.json the same way) -- nothing outside that
	// command may rewrite go.mod/go.sum, and this route is no exception.
	execution := exec.Command(wailsExecutable, "dev", "-m")
	execution.Dir = filepath.Join(request.Root, "cmd", "golc-desktop")
	execution.Env = environment
	// Streamed directly rather than buffered (mirrors run.go's own
	// golc-desktop launch, not build.go's short-lived progressSink
	// invocations): `wails dev` is a long-running interactive dev server,
	// and buffering everything until it exits would silence exactly the
	// live rebuild/reload output this route exists to show.
	execution.Stdin = os.Stdin
	execution.Stdout = os.Stdout
	execution.Stderr = os.Stderr

	if runtime.GOOS == "windows" {
		restoreSyso, err := disableWindowsResourceSyso(execution.Dir)
		if err != nil {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_DEV_SYSO_DISABLE: %v\n", err))}
		}
		// Absorb Ctrl+C/SIGTERM here so this process survives long enough to
		// run restoreSyso below: on Windows, CTRL_C_EVENT is delivered to
		// every process sharing this console (this process AND the `wails
		// dev` child together, since exec.Command does not request a new
		// process group), and Go's default disposition for an unhandled
		// os.Interrupt is immediate termination -- which would skip the
		// deferred restore entirely and leave the checked-in resource file
		// renamed on disk for every subsequent `mage Build`. Registering a
		// handler (mirrors artnet.go's existing
		// signal.NotifyContext/signal.Notify convention in this package)
		// overrides that default; the child process, with no handler of its
		// own, still exits on the same signal, which is all that's needed to
		// unblock execution.Run() below.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		defer func() {
			if restoreErr := restoreSyso(); restoreErr != nil {
				fmt.Fprintf(os.Stderr, "GOLC_DEV_SYSO_RESTORE_FAILED: %v\n", restoreErr)
			}
		}()
	}

	runErr := execution.Run()
	if runErr == nil {
		return Result{}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode()}
	}
	return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_DEV_FAILED: %v\n", runErr))}
}
