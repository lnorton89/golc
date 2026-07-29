// lint.go is the lint command file: it owns the "lint" scope and
// self-registers the "lint" route (CONTEXT D-03) — contributor-facing
// static analysis, never part of the offline core graph or the
// committed Windows PR workflow (internal/api/coverage_test.go's
// reasonDevTooling exclusion covers this route the same way it already
// covers build/check/docs/generate/package/test: local development
// tooling, not a REST-exposed show-control operation).
//
// golangci-lint is provisioned the same way midicat/wails are
// (config/toolchain.toml's [go_install.golangci-lint], installed by
// internal/bootstrap/engine.go's installGoInstallTools into
// .tools/cache/go-bin) rather than pinned as a separate checksummed
// archive like Go/Mage/Node: it is a plain `go install <module>@<version>`
// of a Go-ecosystem CLI, so Go's own module proxy/sumdb already verifies
// its integrity, matching this repository's existing precedent for
// exactly this kind of tool.
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
	Scope:   "lint",
	Summary: "Static analysis over every project Go package via the project-local pinned golangci-lint.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "lint",
	Summary: "Run golangci-lint over every project Go package with the pinned toolchain: lint [--fix].",
	Handler: runLint,
})

// parseLintArgs accepts exactly two supported forms: no arguments (report
// only) or "--fix" (apply every auto-fixable finding in place).
func parseLintArgs(args []string) (fix bool, err error) {
	switch {
	case len(args) == 0:
		return false, nil
	case len(args) == 1 && args[0] == "--fix":
		return true, nil
	default:
		return false, fmt.Errorf("GOLC_LINT_USAGE: unsupported argument(s) %v; usage: lint [--fix]", args)
	}
}

// runLint serves the self-registered "lint" route. golangci-lint shells
// out to the Go toolchain internally for type information (the same
// reason wails dev needs the pinned Go on PATH -- see dev.go's doc
// comment), so this route prepends the pinned Go executable's directory
// plus layout.GoBin (where golangci-lint itself, and any other
// go-install-provisioned tool it might shell out to, lives) onto the
// child's PATH before running it, exactly like dev.go does for `wails
// dev`.
func runLint(request Request) Result {
	fix, err := parseLintArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	layout, err := bootstrap.NewProjectCacheLayout(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_LINT_CACHE_LAYOUT: %v\n", err)}
	}
	executableSuffix := ""
	if runtime.GOOS == "windows" {
		executableSuffix = ".exe"
	}
	lintExecutable := layout.LintBinaryPath(executableSuffix)
	if info, statErr := os.Stat(lintExecutable); statErr != nil || !info.Mode().IsRegular() {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil,
			"GOLC_LINT_BINARY_MISSING: %s: run 'mage Bootstrap' first\n", lintExecutable)}
	}
	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	arguments := []string{"run"}
	if fix {
		arguments = append(arguments, "--fix")
	}

	stdout := &progressSink{live: request.Stdout}
	stderr := &progressSink{live: request.Stderr}

	execution := exec.Command(lintExecutable, arguments...)
	execution.Dir = request.Root
	environment := projectGoEnvironment(request.Root)
	environment = prependPathDirectory(environment, filepath.Dir(goExecutable))
	environment = prependPathDirectory(environment, layout.GoBin)
	execution.Env = environment
	execution.Stdout = stdout
	execution.Stderr = stderr

	runErr := execution.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			// golangci-lint's own nonzero exit means "findings reported"
			// (or --fix left unfixable findings behind), not necessarily
			// that the tool itself crashed -- surfaced as-is, same as
			// build/test's own exit-code passthrough for a failing
			// pinned-toolchain invocation.
			return Result{ExitCode: exitErr.ExitCode(), Stdout: stdout.buffered(), Stderr: stderr.buffered()}
		}
		stderr.writeString(fmt.Sprintf("GOLC_LINT_FAILED: %v\n", runErr))
		return Result{ExitCode: 1, Stdout: stdout.buffered(), Stderr: stderr.buffered()}
	}
	return Result{Stdout: stdout.buffered(), Stderr: stderr.buffered()}
}
