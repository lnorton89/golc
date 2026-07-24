// test.go is the generic project test command file: it self-registers the
// exact "test" route (CONTEXT D-03) and serves three forms:
//   - "test --quick --scope <scope-name>": the original scoped quick run.
//     Route keys are normalized words and may not begin with dashes, so a
//     flag can never itself be a route word; this dispatcher parses the
//     remaining arguments strictly.
//   - "test --quick" (bare): the offline core graph's quick step
//     (internal/delivery's "test" Step) — a fast go vet sanity gate over
//     every project package, meeting 01-VALIDATION.md's <=30s quick
//     budget without executing every test body.
//   - "test" (bare, no flags): the full suite — every project Go package's
//     tests run once through the pinned toolchain, followed by every
//     registered Node scope's exact test command.
//
// The scoped dispatcher accepts only safe scope names, first consults the
// registered Node-scope registry (MustDeclareNodeScope; empty in Phase 1's
// core graph, populated by a later Node-owning plan), and otherwise
// derives the exact `TestScope{PascalName}` marker, lists matching
// markers through the pinned project-local Go toolchain before executing
// anything, fails when zero markers exist, and then runs only those
// project-local Go tests (01-VALIDATION route/scope ordering contract).
package command

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/lnorton89/golc/internal/bootstrap"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "test",
	Summary: "Project-local Go test execution.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "test",
	Summary: "Run project tests: test [--quick [--scope <scope-name>]].",
	Handler: runTest,
})

// testScopeNamePattern is the only accepted scope-name shape: lowercase
// alphanumeric words joined by single hyphens. Anything else — flags,
// paths, regex metacharacters — is rejected before any translation.
var testScopeNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// toolchainVersionPattern keeps the committed Go pin path-safe: a version
// is dotted digits only, so joining it under .tools/toolchains can never
// escape the repository (T-01-SC: execution stays on the verified
// project-local toolchain).
var toolchainVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

// testInvocation is the parsed shape of one "test" invocation: exactly one
// of the three supported modes below.
type testInvocation struct {
	// mode is "full" (bare test), "quick" (bare test --quick), or
	// "quick-scope" (test --quick --scope <scope-name>).
	mode  string
	scope string
}

// parseTestArgs accepts exactly the three supported forms: no arguments
// (full suite), "--quick" alone (quick graph orchestration), or
// "--quick --scope <scope-name>" / "--quick --scope=<scope-name>" (the
// original scoped quick run). "--scope" without "--quick" is rejected: a
// scope always runs through the quick marker-discovery path.
func parseTestArgs(args []string) (testInvocation, error) {
	quick := false
	scope := ""
	setScope := func(value string) error {
		if scope != "" {
			return fmt.Errorf("GOLC_TEST_USAGE: --scope may be given only once")
		}
		if value == "" {
			return fmt.Errorf("GOLC_TEST_USAGE: --scope requires a scope name")
		}
		scope = value
		return nil
	}
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--quick":
			quick = true
			i++
		case argument == "--scope":
			if i+1 >= len(args) {
				return testInvocation{}, fmt.Errorf("GOLC_TEST_USAGE: --scope requires a scope name")
			}
			if err := setScope(args[i+1]); err != nil {
				return testInvocation{}, err
			}
			i += 2
		case strings.HasPrefix(argument, "--scope="):
			if err := setScope(strings.TrimPrefix(argument, "--scope=")); err != nil {
				return testInvocation{}, err
			}
			i++
		default:
			return testInvocation{}, fmt.Errorf(
				"GOLC_TEST_USAGE: unsupported argument %q; usage: test [--quick [--scope <scope-name>]]", argument)
		}
	}
	switch {
	case quick && scope != "":
		return testInvocation{mode: "quick-scope", scope: scope}, nil
	case quick:
		return testInvocation{mode: "quick"}, nil
	case scope != "":
		return testInvocation{}, fmt.Errorf("GOLC_TEST_USAGE: --scope requires --quick; usage: test [--quick [--scope <scope-name>]]")
	default:
		return testInvocation{mode: "full"}, nil
	}
}

// scopeTestMarker translates a validated scope name to its exact Go test
// marker: config-local -> TestScopeConfigLocal.
func scopeTestMarker(scopeName string) string {
	var builder strings.Builder
	builder.WriteString("TestScope")
	for _, segment := range strings.Split(scopeName, "-") {
		builder.WriteString(strings.ToUpper(segment[:1]) + segment[1:])
	}
	return builder.String()
}

// resolvePinnedGoExecutable locates the bootstrap-provisioned Go toolchain
// from the committed pin in config/toolchain.toml. The host Go toolchain
// is never consulted (Plan 16 GOTOOLCHAIN=local contract).
func resolvePinnedGoExecutable(root string) (string, error) {
	manifestPath := filepath.Join(root, "config", "toolchain.toml")
	manifest := struct {
		Toolchain struct {
			Go struct {
				Version string `toml:"version"`
			} `toml:"go"`
		} `toml:"toolchain"`
	}{}
	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		return "", fmt.Errorf("GOLC_TEST_TOOLCHAIN_MISSING: config/toolchain.toml: %v", err)
	}
	version := manifest.Toolchain.Go.Version
	if version == "" {
		return "", fmt.Errorf("GOLC_TEST_TOOLCHAIN_MISSING: config/toolchain.toml does not pin toolchain.go.version")
	}
	if !toolchainVersionPattern.MatchString(version) {
		return "", fmt.Errorf("GOLC_TEST_TOOLCHAIN_MISSING: pinned toolchain.go.version %q is not a safe dotted version", version)
	}
	goExecutable := filepath.Join(
		root, ".tools", "toolchains", "go", version, bootstrap.PlatformKey(),
		"go", "bin", bootstrap.ExecutableName("go"))
	if _, err := os.Stat(goExecutable); err != nil {
		return "", fmt.Errorf("GOLC_TEST_TOOLCHAIN_MISSING: %s: run 'golc.ps1 bootstrap' first", goExecutable)
	}
	return goExecutable, nil
}

// projectGoEnvironment returns the child environment for project-local Go
// invocations: the pinned-toolchain and repository-local cache variables
// are enforced even when the shim did not export them. GOPROXY=off (D-02)
// means a missing module sum fails closed with Go's own diagnostic instead
// of a silent network fetch, even though GOFLAGS=-mod=readonly already
// forbids the module graph from changing.
func projectGoEnvironment(root string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		name := strings.SplitN(entry, "=", 2)[0]
		switch strings.ToUpper(name) {
		case "GOTOOLCHAIN", "GOPROXY", "GOMODCACHE", "GOCACHE", "GOFLAGS":
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment,
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOMODCACHE="+filepath.Join(root, ".tools", "cache", "go-mod"),
		"GOCACHE="+filepath.Join(root, ".tools", "cache", "go-build"),
		"GOFLAGS=-mod=readonly",
	)
	return upsertEnvironment(environment, "GOLC_PROJECT_ROOT", root)
}

func upsertEnvironment(environment []string, name, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		existing, _, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(existing, name) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, name+"="+value)
}

// runProjectGo executes one pinned-toolchain Go invocation inside root.
func runProjectGo(goExecutable, root string, arguments []string) (stdout, stderr []byte, err error) {
	execution := exec.Command(goExecutable, arguments...)
	execution.Dir = root
	execution.Env = projectGoEnvironment(root)
	var stdoutBuffer, stderrBuffer bytes.Buffer
	execution.Stdout = &stdoutBuffer
	execution.Stderr = &stderrBuffer
	err = execution.Run()
	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
}

// listScopeMarkers discovers which project packages define the exact
// marker before anything executes. `go test -list` compiles test binaries
// but runs no test.
func listScopeMarkers(goExecutable, root, marker string) ([]string, error) {
	stdout, stderr, err := runProjectGo(goExecutable, root, []string{
		"test", "-list", "^" + marker + "$", "./...",
	})
	if err != nil {
		return nil, fmt.Errorf("GOLC_TEST_LIST_FAILED: %v\n%s%s", err, stdout, stderr)
	}

	packages := []string{}
	pendingMatch := false
	for _, line := range strings.Split(string(stdout), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == marker:
			pendingMatch = true
		case strings.HasPrefix(trimmed, "ok "):
			fields := strings.Fields(trimmed)
			if pendingMatch && len(fields) >= 2 {
				packages = append(packages, fields[1])
			}
			pendingMatch = false
		case strings.HasPrefix(trimmed, "? "):
			pendingMatch = false
		}
	}
	sort.Strings(packages)
	return packages, nil
}

// NodeScopeRegistration declares one project-local Node/npm test scope
// (CONTEXT D-03/D-10): a project-relative directory containing its own
// package.json, plus the exact arguments that run its
// TestScope{PascalName} marker. No Node scope exists yet in Phase 1's
// core graph; this is the registered extension point a later Node-owning
// plan (for example the tools/linear-sync package) uses so
// "test --quick --scope <name>" and the full-suite "test" can reach a
// Node scope exactly the way they already reach a Go scope, without
// editing this dispatcher again.
type NodeScopeRegistration struct {
	Scope     string
	Dir       string
	Marker    string
	Arguments []string
}

// declaredNodeScopes collects every Node scope a command file registers
// through MustDeclareNodeScope from a package-level var initializer.
var declaredNodeScopes []NodeScopeRegistration

// MustDeclareNodeScope is the compile-safe self-registration entrypoint a
// future Node-owning command file calls from a package-level var
// initializer, mirroring MustDeclareRoute/MustDeclareScope's shape:
//
//	var _ = command.MustDeclareNodeScope(command.NodeScopeRegistration{...})
func MustDeclareNodeScope(registration NodeScopeRegistration) NodeScopeRegistration {
	if !testScopeNamePattern.MatchString(registration.Scope) {
		panic(fmt.Sprintf("GOLC_TEST_NODE_SCOPE_INVALID: %q is not a safe scope name", registration.Scope))
	}
	if strings.TrimSpace(registration.Marker) == "" {
		panic(fmt.Sprintf("GOLC_TEST_NODE_SCOPE_INVALID: %q has a blank marker", registration.Scope))
	}
	if len(registration.Arguments) == 0 {
		panic(fmt.Sprintf("GOLC_TEST_NODE_SCOPE_INVALID: %q declares no arguments", registration.Scope))
	}
	for _, existing := range declaredNodeScopes {
		if existing.Scope == registration.Scope {
			panic(fmt.Sprintf("GOLC_TEST_NODE_SCOPE_DUPLICATE: %q is already registered", registration.Scope))
		}
	}
	declaredNodeScopes = append(declaredNodeScopes, registration)
	return registration
}

// lookupNodeScope resolves one registered Node scope by exact name.
func lookupNodeScope(scopeName string) (NodeScopeRegistration, bool) {
	for _, registration := range declaredNodeScopes {
		if registration.Scope == scopeName {
			return registration, true
		}
	}
	return NodeScopeRegistration{}, false
}

// runNodeScopeTest resolves pinned Node from root, then runs one
// registered scope's exact arguments inside its repository-relative
// directory.
func runNodeScopeTest(root string, registration NodeScopeRegistration) Result {
	nodeExecutable, err := resolvePinnedNodeExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	execution := exec.Command(nodeExecutable, registration.Arguments...)
	execution.Dir = filepath.Join(root, filepath.FromSlash(registration.Dir))
	execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
	var stdoutBuffer, stderrBuffer bytes.Buffer
	execution.Stdout = &stdoutBuffer
	execution.Stderr = &stderrBuffer
	err = execution.Run()

	var output bytes.Buffer
	fmt.Fprintf(&output, "GOLC test: scope %s -> Node command %s %s\n", registration.Scope, nodeExecutable, strings.Join(registration.Arguments, " "))
	output.Write(stdoutBuffer.Bytes())
	if err != nil {
		stderr := append(stderrBuffer.Bytes(), []byte(fmt.Sprintf("GOLC_TEST_FAILED: scope %s: %v\n", registration.Scope, err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	return Result{Stdout: output.Bytes(), Stderr: stderrBuffer.Bytes()}
}

// runAllNodeScopes runs every registered Node scope in declaration order,
// stopping at the first failure.
func runAllNodeScopes(root string) ([]byte, error) {
	var output bytes.Buffer
	for _, registration := range declaredNodeScopes {
		result := runNodeScopeTest(root, registration)
		output.Write(result.Stdout)
		if result.ExitCode != 0 {
			return output.Bytes(), fmt.Errorf("GOLC_TEST_FAILED: node scope %s failed", registration.Scope)
		}
	}
	return output.Bytes(), nil
}

// runTest serves the self-registered "test" route, dispatching to one of
// the three supported forms.
func runTest(request Request) Result {
	invocation, err := parseTestArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	switch invocation.mode {
	case "quick-scope":
		return runTestQuickScope(request.Root, invocation.scope)
	case "quick":
		return runTestQuick(request.Root)
	default:
		return runTestFull(request.Root)
	}
}

// runTestQuickScope serves "test --quick --scope <scope-name>": a
// registered Node scope is preferred when present; otherwise this falls
// back to exact Go marker discovery, fail-on-zero, and the pinned
// toolchain, unchanged from the original scoped-quick behavior.
func runTestQuickScope(root, scopeName string) Result {
	if !testScopeNamePattern.MatchString(scopeName) {
		diagnostic := fmt.Sprintf("GOLC_TEST_SCOPE_INVALID: %q is not a safe scope name\n", scopeName)
		return Result{ExitCode: 2, Stderr: []byte(diagnostic)}
	}

	if nodeScope, found := lookupNodeScope(scopeName); found {
		return runNodeScopeTest(root, nodeScope)
	}

	marker := scopeTestMarker(scopeName)

	goExecutable, err := resolvePinnedGoExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	packages, err := listScopeMarkers(goExecutable, root, marker)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if len(packages) == 0 {
		diagnostic := fmt.Sprintf(
			"GOLC_TEST_SCOPE_NO_MARKERS: scope %q has no %s marker in any project package\n", scopeName, marker)
		return Result{ExitCode: 1, Stderr: []byte(diagnostic)}
	}

	var output bytes.Buffer
	fmt.Fprintf(&output, "GOLC test: scope %s -> marker %s\n", scopeName, marker)
	for _, packagePath := range packages {
		fmt.Fprintf(&output, "GOLC test: %s found in %s\n", marker, packagePath)
	}

	arguments := append([]string{"test", "-count=1", "-run", "^" + marker + "$"}, packages...)
	stdout, stderr, err := runProjectGo(goExecutable, root, arguments)
	output.Write(stdout)
	if err != nil {
		stderr = append(stderr, []byte(fmt.Sprintf("GOLC_TEST_FAILED: scope %s: %v\n", scopeName, err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	return Result{Stdout: output.Bytes(), Stderr: stderr}
}

// runTestQuick serves bare "test --quick": internal/delivery's offline
// core graph "test" Step. It is a fast go vet compile/lint sanity gate
// over every project package — no test body executes — meeting
// 01-VALIDATION.md's <=30s quick budget.
func runTestQuick(root string) Result {
	goExecutable, err := resolvePinnedGoExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	var output bytes.Buffer
	output.WriteString("GOLC test --quick: go vet ./...\n")
	stdout, stderr, err := runProjectGo(goExecutable, root, []string{"vet", "./..."})
	output.Write(stdout)
	if err != nil {
		stderr = append(stderr, []byte(fmt.Sprintf("GOLC_TEST_FAILED: quick vet: %v\n", err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	output.WriteString("GOLC test --quick: go vet passed.\n")
	return Result{Stdout: output.Bytes(), Stderr: stderr}
}

// runTestFull serves bare "test": every project Go package's tests run
// once through the pinned toolchain (fail on any failure), followed by
// every registered Node scope's exact test command. "test" never filters
// by marker: it is the one full-suite contributor/CI gate
// 01-VALIDATION.md documents.
func runTestFull(root string) Result {
	goExecutable, err := resolvePinnedGoExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	var output bytes.Buffer
	output.WriteString("GOLC test: full suite (go test ./...).\n")
	stdout, stderr, err := runProjectGo(goExecutable, root, []string{"test", "-count=1", "./..."})
	output.Write(stdout)
	if err != nil {
		stderr = append(stderr, []byte(fmt.Sprintf("GOLC_TEST_FAILED: full suite: %v\n", err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}

	nodeOutput, nodeErr := runAllNodeScopes(root)
	output.Write(nodeOutput)
	if nodeErr != nil {
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: append(stderr, []byte(nodeErr.Error()+"\n")...)}
	}
	output.WriteString("GOLC test: full suite passed.\n")
	return Result{Stdout: output.Bytes(), Stderr: stderr}
}
