// build.go is the build command file: it owns the "build" scope and
// self-registers the exact build route through the package declaration
// entrypoints (CONTEXT D-03/D-10) — the central router is never edited.
// It reuses the pinned-toolchain resolution and repository-local
// environment internal/command/test.go already establishes
// (resolvePinnedGoExecutable/runProjectGo/projectGoEnvironment) rather
// than re-implementing toolchain discovery, so build and test can never
// silently disagree about which Go binary or caches a project-local
// invocation uses.
//
// "build --scope <name>" (Plan 01-13) extends this route with the same
// registered-Node-scope pattern test.go's "test --quick --scope <name>"
// already establishes for quick tests: a Node-owning command file
// self-registers its build scope through MustDeclareNodeBuildScope, and
// this dispatcher resolves the pinned project-local Node/TypeScript
// compiler at request time (never a host PATH lookup) rather than baking
// an executable path into the registration itself.
package command

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "build",
	Summary: "Project-local Go build verification.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "build",
	Summary: "Compile every project Go package with the pinned toolchain: build [--scope <scope-name>].",
	Handler: runBuild,
})

// NodeBuildScopeRegistration declares one project-local Node/TypeScript
// build scope (CONTEXT D-03/D-10): a project-relative directory containing
// its own package.json/tsconfig.json, built with the pinned project-local
// Node/TypeScript compiler resolved from config/toolchain.toml at request
// time. Mirrors test.go's NodeScopeRegistration shape for the parallel
// build-side concern.
type NodeBuildScopeRegistration struct {
	Scope string
	Dir   string
}

// declaredNodeBuildScopes collects every Node build scope a command file
// registers through MustDeclareNodeBuildScope from a package-level var
// initializer.
var declaredNodeBuildScopes []NodeBuildScopeRegistration

// MustDeclareNodeBuildScope is the compile-safe self-registration
// entrypoint a Node-owning command file calls from a package-level var
// initializer, mirroring test.go's MustDeclareNodeScope:
//
//	var _ = command.MustDeclareNodeBuildScope(command.NodeBuildScopeRegistration{...})
func MustDeclareNodeBuildScope(registration NodeBuildScopeRegistration) NodeBuildScopeRegistration {
	if !testScopeNamePattern.MatchString(registration.Scope) {
		panic(fmt.Sprintf("GOLC_BUILD_NODE_SCOPE_INVALID: %q is not a safe scope name", registration.Scope))
	}
	if strings.TrimSpace(registration.Dir) == "" {
		panic(fmt.Sprintf("GOLC_BUILD_NODE_SCOPE_INVALID: %q declares no directory", registration.Scope))
	}
	for _, existing := range declaredNodeBuildScopes {
		if existing.Scope == registration.Scope {
			panic(fmt.Sprintf("GOLC_BUILD_NODE_SCOPE_DUPLICATE: %q is already registered", registration.Scope))
		}
	}
	declaredNodeBuildScopes = append(declaredNodeBuildScopes, registration)
	return registration
}

// lookupNodeBuildScope resolves one registered Node build scope by exact
// name.
func lookupNodeBuildScope(scopeName string) (NodeBuildScopeRegistration, bool) {
	for _, registration := range declaredNodeBuildScopes {
		if registration.Scope == scopeName {
			return registration, true
		}
	}
	return NodeBuildScopeRegistration{}, false
}

// resolvePinnedNodeExecutable locates the bootstrap-provisioned Node
// toolchain from the committed pin in config/toolchain.toml, mirroring
// resolvePinnedGoExecutable exactly (never a host PATH lookup, CONTEXT
// D-01/D-02).
func resolvePinnedNodeExecutable(root string) (string, error) {
	manifestPath := filepath.Join(root, "config", "toolchain.toml")
	manifest := struct {
		Toolchain struct {
			Node struct {
				Version string `toml:"version"`
			} `toml:"node"`
		} `toml:"toolchain"`
	}{}
	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		return "", fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: config/toolchain.toml: %v", err)
	}
	version := manifest.Toolchain.Node.Version
	if version == "" {
		return "", fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: config/toolchain.toml does not pin toolchain.node.version")
	}
	if !toolchainVersionPattern.MatchString(version) {
		return "", fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: pinned toolchain.node.version %q is not a safe dotted version", version)
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: project-local Node provisioning is Windows-only in Phase 1")
	}
	nodeExecutable := filepath.Join(
		root, ".tools", "toolchains", "node", version, "windows-amd64",
		"node-v"+version+"-win-x64", "node.exe")
	if _, err := os.Stat(nodeExecutable); err != nil {
		return "", fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: %s: run 'golc.ps1 bootstrap --include linear-sync' first", nodeExecutable)
	}
	return nodeExecutable, nil
}

// runBuildNodeScope compiles one registered Node build scope with the
// pinned project-local Node/TypeScript compiler: `node
// <dir>/node_modules/typescript/bin/tsc -p <dir>/tsconfig.json`. It never
// runs npm install/ci (bootstrap already exact-lock-installed
// node_modules) and never falls back to a host-PATH `tsc`.
func runBuildNodeScope(root string, registration NodeBuildScopeRegistration) Result {
	nodeExecutable, err := resolvePinnedNodeExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	scopeDir := filepath.Join(root, filepath.FromSlash(registration.Dir))
	tscPath := filepath.Join(scopeDir, "node_modules", "typescript", "bin", "tsc")
	if _, statErr := os.Stat(tscPath); statErr != nil {
		diagnostic := fmt.Sprintf(
			"GOLC_BUILD_SCOPE_TSC_MISSING: %s: run 'golc.ps1 bootstrap --include linear-sync' first\n", tscPath)
		return Result{ExitCode: 1, Stderr: []byte(diagnostic)}
	}
	tsconfigPath := filepath.Join(scopeDir, "tsconfig.json")

	execution := exec.Command(nodeExecutable, tscPath, "-p", tsconfigPath)
	execution.Dir = scopeDir
	execution.Env = os.Environ()
	var stdoutBuffer, stderrBuffer bytes.Buffer
	execution.Stdout = &stdoutBuffer
	execution.Stderr = &stderrBuffer
	err = execution.Run()

	var output bytes.Buffer
	fmt.Fprintf(&output, "GOLC build: scope %s -> tsc -p %s\n", registration.Scope, tsconfigPath)
	output.Write(stdoutBuffer.Bytes())
	if err != nil {
		stderr := append(stderrBuffer.Bytes(), []byte(fmt.Sprintf("GOLC_BUILD_FAILED: scope %s: %v\n", registration.Scope, err))...)
		return Result{ExitCode: 1, Stdout: output.Bytes(), Stderr: stderr}
	}
	output.WriteString("GOLC build: scope " + registration.Scope + " compiled cleanly.\n")
	return Result{Stdout: output.Bytes(), Stderr: stderrBuffer.Bytes()}
}

// parseBuildArgs accepts exactly two supported forms: no arguments (build
// every Go package with the pinned toolchain), or "--scope <scope-name>" /
// "--scope=<scope-name>" (build one registered Node scope).
func parseBuildArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 2 && args[0] == "--scope" {
		if args[1] == "" {
			return "", fmt.Errorf("GOLC_BUILD_USAGE: --scope requires a scope name; usage: build [--scope <scope-name>]")
		}
		return args[1], nil
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "--scope=") {
		value := strings.TrimPrefix(args[0], "--scope=")
		if value == "" {
			return "", fmt.Errorf("GOLC_BUILD_USAGE: --scope requires a scope name; usage: build [--scope <scope-name>]")
		}
		return value, nil
	}
	return "", fmt.Errorf("GOLC_BUILD_USAGE: unsupported argument %q; usage: build [--scope <scope-name>]", args[0])
}

// runBuild serves the self-registered "build" route. Bare "build" compiles
// every project Go package with the pinned toolchain (unchanged); "build
// --scope <name>" dispatches to one registered Node build scope instead.
// It never opens a network connection: projectGoEnvironment sets
// GOFLAGS=-mod=readonly and GOPROXY=off, so a missing module sum fails
// closed with Go's own diagnostic instead of a silent download.
func runBuild(request Request) Result {
	scopeName, err := parseBuildArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	if scopeName != "" {
		if !testScopeNamePattern.MatchString(scopeName) {
			diagnostic := fmt.Sprintf("GOLC_BUILD_SCOPE_INVALID: %q is not a safe scope name\n", scopeName)
			return Result{ExitCode: 2, Stderr: []byte(diagnostic)}
		}
		registration, found := lookupNodeBuildScope(scopeName)
		if !found {
			diagnostic := fmt.Sprintf("GOLC_BUILD_SCOPE_UNKNOWN: no registered build scope named %q\n", scopeName)
			return Result{ExitCode: 1, Stderr: []byte(diagnostic)}
		}
		return runBuildNodeScope(request.Root, registration)
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
