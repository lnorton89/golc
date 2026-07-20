// test.go is the generic project test command file: it self-registers the
// route that serves `test --quick --scope {scope-name}` (CONTEXT D-03).
// Route keys are normalized words and may not begin with dashes, so the
// registered route is "test" and the handler accepts exactly the
// `--quick --scope <scope-name>` form.
//
// The dispatcher accepts only safe scope names, derives the exact
// `TestScope{PascalName}` marker, lists matching markers through the
// pinned project-local Go toolchain before executing anything, fails when
// zero markers exist, and then runs only those project-local Go tests
// (01-VALIDATION route/scope ordering contract).
package command

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "test",
	Summary: "Project-local Go test execution.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "test",
	Summary: "Run project-local Go tests for one registered scope: test --quick --scope <scope-name>.",
	Handler: runTestQuickScope,
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

// parseQuickScopeArgs accepts exactly the supported quick form:
// --quick --scope <scope-name> (or --scope=<scope-name>).
func parseQuickScopeArgs(args []string) (string, error) {
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
				return "", fmt.Errorf("GOLC_TEST_USAGE: --scope requires a scope name")
			}
			if err := setScope(args[i+1]); err != nil {
				return "", err
			}
			i += 2
		case strings.HasPrefix(argument, "--scope="):
			if err := setScope(strings.TrimPrefix(argument, "--scope=")); err != nil {
				return "", err
			}
			i++
		default:
			return "", fmt.Errorf("GOLC_TEST_USAGE: unsupported argument %q; usage: test --quick --scope <scope-name>", argument)
		}
	}
	if !quick || scope == "" {
		return "", fmt.Errorf("GOLC_TEST_USAGE: usage: test --quick --scope <scope-name>")
	}
	return scope, nil
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
	executableName := "go"
	if runtime.GOOS == "windows" {
		executableName = "go.exe"
	}
	goExecutable := filepath.Join(root, ".tools", "toolchains", "go", version, "windows-amd64", "go", "bin", executableName)
	if _, err := os.Stat(goExecutable); err != nil {
		return "", fmt.Errorf("GOLC_TEST_TOOLCHAIN_MISSING: %s: run 'golc.ps1 bootstrap' first", goExecutable)
	}
	return goExecutable, nil
}

// projectGoEnvironment returns the child environment for project-local Go
// invocations: the pinned-toolchain and repository-local cache variables
// are enforced even when the shim did not export them.
func projectGoEnvironment(root string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		name := strings.SplitN(entry, "=", 2)[0]
		switch strings.ToUpper(name) {
		case "GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOFLAGS":
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment,
		"GOTOOLCHAIN=local",
		"GOMODCACHE="+filepath.Join(root, ".tools", "cache", "go-mod"),
		"GOCACHE="+filepath.Join(root, ".tools", "cache", "go-build"),
		"GOFLAGS=-mod=readonly",
	)
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

// runTestQuickScope serves the registered generic quick/scope route.
func runTestQuickScope(request Request) Result {
	scopeName, err := parseQuickScopeArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if !testScopeNamePattern.MatchString(scopeName) {
		diagnostic := fmt.Sprintf("GOLC_TEST_SCOPE_INVALID: %q is not a safe scope name\n", scopeName)
		return Result{ExitCode: 2, Stderr: []byte(diagnostic)}
	}
	marker := scopeTestMarker(scopeName)

	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	packages, err := listScopeMarkers(goExecutable, request.Root, marker)
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
	stdout, stderr, err := runProjectGo(goExecutable, request.Root, arguments)
	output.Write(stdout)
	if err != nil {
		stderr = append(stderr, []byte(fmt.Sprintf("GOLC_TEST_FAILED: scope %s: %v\n", scopeName, err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	return Result{Stdout: output.Bytes(), Stderr: stderr}
}
