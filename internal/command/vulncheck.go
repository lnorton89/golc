// vulncheck.go is the vulncheck command file: it owns the "vulncheck"
// scope and self-registers the "vulncheck" route (CONTEXT D-03) --
// contributor/CI-facing dependency vulnerability scanning, never part of
// the offline core graph or the committed Windows PR workflow
// (internal/api/coverage_test.go's reasonDevTooling exclusion covers this
// route the same way it already covers build/check/docs/generate/lint/
// package/test: local development tooling, not a REST-exposed
// show-control operation). Unlike the offline core graph's steps,
// govulncheck reaches the network to fetch the public vulnerability
// database (https://vuln.go.dev) unless GOVULNDB is overridden, so this
// route deliberately stays outside RunOffline's deny-transport contract
// exactly like "bootstrap" does.
//
// govulncheck is provisioned the same way golangci-lint is
// (config/toolchain.toml's [go_install.govulncheck], installed by
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
	Scope:   "vulncheck",
	Summary: "Known-vulnerability scanning over every project Go package via the project-local pinned govulncheck.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "vulncheck",
	Summary: "Run govulncheck over every project Go package with the pinned toolchain: vulncheck.",
	Handler: runVulncheck,
})

// runVulncheck serves the self-registered "vulncheck" route. govulncheck
// shells out to the Go toolchain internally for call-graph analysis (the
// same reason lint's golangci-lint needs the pinned Go on PATH -- see
// lint.go's doc comment), so this route prepends the pinned Go
// executable's directory plus layout.GoBin onto the child's PATH before
// running it, exactly like runLint does.
func runVulncheck(request Request) Result {
	if len(request.Args) != 0 {
		return Result{ExitCode: 2, Stderr: fmt.Appendf(nil,
			"GOLC_VULNCHECK_USAGE: unsupported argument(s) %v; usage: vulncheck\n", request.Args)}
	}

	layout, err := bootstrap.NewProjectCacheLayout(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil, "GOLC_VULNCHECK_CACHE_LAYOUT: %v\n", err)}
	}
	executableSuffix := ""
	if runtime.GOOS == "windows" {
		executableSuffix = ".exe"
	}
	vulncheckExecutable := layout.VulncheckBinaryPath(executableSuffix)
	if info, statErr := os.Stat(vulncheckExecutable); statErr != nil || !info.Mode().IsRegular() {
		return Result{ExitCode: 1, Stderr: fmt.Appendf(nil,
			"GOLC_VULNCHECK_BINARY_MISSING: %s: run 'mage Bootstrap' first\n", vulncheckExecutable)}
	}
	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	stdout := &progressSink{live: request.Stdout}
	stderr := &progressSink{live: request.Stderr}

	execution := exec.Command(vulncheckExecutable, "./...")
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
			// govulncheck's own nonzero exit means "vulnerabilities found",
			// not necessarily that the tool itself crashed -- surfaced
			// as-is, same as lint's own exit-code passthrough.
			return Result{ExitCode: exitErr.ExitCode(), Stdout: stdout.buffered(), Stderr: stderr.buffered()}
		}
		stderr.writeString(fmt.Sprintf("GOLC_VULNCHECK_FAILED: %v\n", runErr))
		return Result{ExitCode: 1, Stdout: stdout.buffered(), Stderr: stderr.buffered()}
	}
	return Result{Stdout: stdout.buffered(), Stderr: stderr.buffered()}
}
