// build.go is the build command file: it owns the "build" scope and
// self-registers the exact build route through the package declaration
// entrypoints (CONTEXT D-03/D-10) — the central router is never edited.
// It reuses the pinned-toolchain resolution and repository-local
// environment internal/command/test.go already establishes
// (resolvePinnedGoExecutable/runProjectGo/projectGoEnvironment) rather
// than re-implementing toolchain discovery, so build and test can never
// silently disagree about which Go binary or caches a project-local
// invocation uses.
package command

import (
	"bytes"
	"fmt"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "build",
	Summary: "Project-local Go build verification.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "build",
	Summary: "Compile every project Go package with the pinned toolchain: build.",
	Handler: runBuild,
})

// runBuild serves the self-registered "build" route. It accepts no
// arguments today; a future scoped build can extend parseBuildArgs the
// same way test.go's --scope form does, without changing this route's
// exact name. It never opens a network connection: projectGoEnvironment
// sets GOFLAGS=-mod=readonly and GOPROXY=off, so a missing module sum
// fails closed with Go's own diagnostic instead of a silent download.
func runBuild(request Request) Result {
	if len(request.Args) != 0 {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_BUILD_USAGE: unsupported argument %q; usage: build\n", request.Args[0]))}
	}

	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var output bytes.Buffer
	output.WriteString("GOLC build: compiling every project package with the pinned toolchain.\n")
	stdout, stderr, err := runProjectGo(goExecutable, request.Root, []string{"build", "./..."})
	output.Write(stdout)
	if err != nil {
		stderr = append(stderr, []byte(fmt.Sprintf("GOLC_BUILD_FAILED: %v\n", err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	output.WriteString("GOLC build: every project package compiled cleanly.\n")
	return Result{Stdout: output.Bytes(), Stderr: stderr}
}
