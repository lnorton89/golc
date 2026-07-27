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
	"path/filepath"
	"runtime"

	"github.com/lnorton89/golc/internal/bootstrap"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "dev",
	Summary: "Launch `wails dev` (hot-reload desktop dev loop) with the pinned Go/Node/Wails toolchains on PATH.",
})

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
