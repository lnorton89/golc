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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/lnorton89/golc/internal/bootstrap"
)

// progressSink is a build-progress destination that behaves two different
// ways depending on whether a real invocation supplied a live writer.
//
// With no live writer (every existing test: Request.Stdout/Stderr are nil,
// asserting directly against the returned Result), it is a pure
// bytes.Buffer -- byte-for-byte the prior fully-buffered behavior.
//
// With a live writer (magefile.go's runRouteTarget wires the real process
// stdout/stderr through for an actual "mage Build"), every write is teed to
// it immediately as it happens, so a long silent step like `go build`
// shows progress instead of the whole route going quiet until it returns.
// buffered() then reports nothing, since the content already reached the
// terminal in real time -- returning it too would print everything a
// second time when magefile.go flushes the final Result.
type progressSink struct {
	live io.Writer
	buf  bytes.Buffer
}

func (s *progressSink) Write(p []byte) (int, error) {
	if s.live != nil {
		if _, err := s.live.Write(p); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return s.buf.Write(p)
}

func (s *progressSink) writeString(text string) {
	_, _ = s.Write([]byte(text))
}

func (s *progressSink) buffered() []byte {
	if s.live != nil {
		return nil
	}
	return s.buf.Bytes()
}

// runProjectGoLive executes one pinned-toolchain Go invocation inside root,
// writing its stdout/stderr directly to the given sinks as the subprocess
// produces them (exec.Cmd copies into an io.Writer concurrently, so a
// progressSink with a live writer streams in real time rather than only
// after the process exits) -- mirrors test.go's runProjectGo environment
// setup exactly, just without the fully-buffered-only contract.
func runProjectGoLive(goExecutable, root string, arguments []string, stdout, stderr io.Writer) error {
	execution := exec.Command(goExecutable, arguments...)
	execution.Dir = root
	execution.Env = projectGoEnvironment(root)
	execution.Stdout = stdout
	execution.Stderr = stderr
	return execution.Run()
}

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

// resolvePinnedNodeInstallation locates the bootstrap-provisioned Node
// toolchain from the committed pin in config/toolchain.toml, mirroring
// resolvePinnedGoExecutable exactly (never a host PATH lookup, CONTEXT
// D-01/D-02). It returns the full bootstrap.NodeInstallation (not just the
// node executable) because ensureFrontendDistFresh below needs NPMCLI too.
func resolvePinnedNodeInstallation(root string) (bootstrap.NodeInstallation, error) {
	manifestPath := filepath.Join(root, "config", "toolchain.toml")
	manifest := struct {
		Toolchain struct {
			Node struct {
				Version string `toml:"version"`
			} `toml:"node"`
		} `toml:"toolchain"`
	}{}
	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		return bootstrap.NodeInstallation{}, fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: config/toolchain.toml: %v", err)
	}
	version := manifest.Toolchain.Node.Version
	if version == "" {
		return bootstrap.NodeInstallation{}, fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: config/toolchain.toml does not pin toolchain.node.version")
	}
	if !toolchainVersionPattern.MatchString(version) {
		return bootstrap.NodeInstallation{}, fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: pinned toolchain.node.version %q is not a safe dotted version", version)
	}
	nodeInstall := filepath.Join(root, ".tools", "toolchains", "node", version, bootstrap.PlatformKey())
	node, err := bootstrap.ResolveNodeInstallation(nodeInstall)
	if err != nil {
		return bootstrap.NodeInstallation{}, fmt.Errorf("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: %s: set GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC=1 and run 'mage Bootstrap' first: %v", nodeInstall, err)
	}
	return node, nil
}

// resolvePinnedNodeExecutable locates the bootstrap-provisioned Node
// executable alone, for callers (runBuildNodeScope) that never need NPMCLI.
func resolvePinnedNodeExecutable(root string) (string, error) {
	node, err := resolvePinnedNodeInstallation(root)
	if err != nil {
		return "", err
	}
	return node.Executable, nil
}

// ensureFrontendDistFresh keeps the bare "build" route in sync with
// frontend source changes: cmd/golc-desktop's `//go:embed all:frontend/dist`
// only ever reflects whatever was last written there, and bootstrap's own
// frontend build gate previously only reacted to package.json/
// package-lock.json changes (dependency drift), never source-only edits --
// so a source-only change (a .tsx/.module.css edit, vite.config.ts, ...)
// never reached a freshly built golc-desktop[.exe] without a manual
// `npm run build`. bootstrap.FrontendDistFresh's hash gate is now
// source-tree-aware, so this is a cheap no-op on every build where nothing
// changed, and a real `node <NPMCLI> run build` only when it did. It never
// runs npm ci (mirrors runBuildNodeScope's own "node_modules is bootstrap's
// job, not build's" contract) -- a stale/missing node_modules fails here
// with Node's own diagnostic instead.
func ensureFrontendDistFresh(root string, stdout, stderr *progressSink) error {
	frontendDir := filepath.Join(root, "frontend")
	distIndexPath := filepath.Join(root, "cmd", "golc-desktop", "frontend", "dist", "index.html")
	fresh, err := bootstrap.FrontendDistFresh(frontendDir, distIndexPath)
	if err != nil {
		return err
	}
	if fresh {
		return nil
	}
	node, err := resolvePinnedNodeInstallation(root)
	if err != nil {
		return err
	}
	stdout.writeString("GOLC build: frontend source changed -> npm run build...\n")
	execution := exec.Command(node.Executable, node.NPMCLI, "run", "build")
	execution.Dir = frontendDir
	execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
	execution.Stdout = stdout
	execution.Stderr = stderr
	if err := execution.Run(); err != nil {
		return fmt.Errorf("GOLC_BUILD_FRONTEND_FAILED: %w", err)
	}
	if info, statErr := os.Stat(distIndexPath); statErr != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("GOLC_BUILD_FRONTEND_FAILED: expected %s after npm run build", distIndexPath)
	}
	if err := bootstrap.WriteFrontendBuildManifest(frontendDir); err != nil {
		return err
	}
	stdout.writeString("GOLC build: frontend dist rebuilt.\n")
	return nil
}

// runBuildNodeScope compiles one registered Node build scope with the
// pinned project-local Node/TypeScript compiler: `node
// <dir>/node_modules/typescript/bin/tsc -p <dir>/tsconfig.json`. It never
// runs npm install/ci (bootstrap already exact-lock-installed
// node_modules) and never falls back to a host-PATH `tsc`.
func runBuildNodeScope(root string, registration NodeBuildScopeRegistration, liveStdout, liveStderr io.Writer) Result {
	nodeExecutable, err := resolvePinnedNodeExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	scopeDir := filepath.Join(root, filepath.FromSlash(registration.Dir))
	tscPath := filepath.Join(scopeDir, "node_modules", "typescript", "bin", "tsc")
	if _, statErr := os.Stat(tscPath); statErr != nil {
		diagnostic := fmt.Sprintf(
			"GOLC_BUILD_SCOPE_TSC_MISSING: %s: set GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC=1 and run 'mage Bootstrap' first\n", tscPath)
		return Result{ExitCode: 1, Stderr: []byte(diagnostic)}
	}
	tsconfigPath := filepath.Join(scopeDir, "tsconfig.json")

	stdout := &progressSink{live: liveStdout}
	stderr := &progressSink{live: liveStderr}
	stdout.writeString(fmt.Sprintf("GOLC build: scope %s -> tsc -p %s\n", registration.Scope, tsconfigPath))

	execution := exec.Command(nodeExecutable, tscPath, "-p", tsconfigPath)
	execution.Dir = scopeDir
	execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
	execution.Stdout = stdout
	execution.Stderr = stderr
	err = execution.Run()

	if err != nil {
		stderr.writeString(fmt.Sprintf("GOLC_BUILD_FAILED: scope %s: %v\n", registration.Scope, err))
		return Result{ExitCode: 1, Stdout: stdout.buffered(), Stderr: stderr.buffered()}
	}
	stdout.writeString("GOLC build: scope " + registration.Scope + " compiled cleanly.\n")
	return Result{Stdout: stdout.buffered(), Stderr: stderr.buffered()}
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
		return runBuildNodeScope(request.Root, registration, request.Stdout, request.Stderr)
	}

	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	stdoutSink := &progressSink{live: request.Stdout}
	stderrSink := &progressSink{live: request.Stderr}
	if err := ensureFrontendDistFresh(request.Root, stdoutSink, stderrSink); err != nil {
		stderrSink.writeString(err.Error() + "\n")
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}

	packages, err := buildablePackages(goExecutable, request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: []byte(fmt.Sprintf("GOLC_BUILD_FAILED: %v\n", err))}
	}

	stdoutSink.writeString("GOLC build: compiling every project package with the pinned toolchain.\n")
	// -o <root>/ (a directory target, trailing separator required) makes
	// `go build` write each main package's binary into the repository
	// root using its default name (golc-project[.exe],
	// golc-desktop[.exe], golc-mcp[.exe]); non-main packages in the same
	// invocation are unaffected (go build already discards their object
	// output regardless of -o). Without -o, compiling more than one
	// package at once makes `go build` discard every resulting binary
	// entirely (it only ever writes an output file for a single-package
	// invocation) — this route silently produced nothing runnable before
	// this fix, even though every package genuinely compiled (observed
	// live: `mage Build` on a clean checkout, then no golc-desktop.exe
	// anywhere to actually run).
	//
	// -tags desktop,production is required by cmd/golc-desktop's Wails
	// build (its own conditionally-compiled frontend-embed/webview files
	// are gated behind these tags; without them the linked binary panics
	// at runtime with "GOLC_WAILS_RUN_FAILED: Wails applications will not
	// build without the correct build tags" -- this was already known and
	// manually worked around in every Phase 6 verification step
	// (06-04-SUMMARY.md et al.: "the correct invocation requires -tags
	// desktop,production"), but never wired into this shared route until
	// now, so mage Build/mage PackageFoundation always silently produced a
	// broken golc-desktop[.exe]. Harmless for every other package in this
	// invocation (cmd/golc-project, tools/golc-mcp): neither defines any
	// //go:build constraint on "desktop" or "production", so the extra
	// tags are simply ignored for them.
	// -v streams each package's import path to stderr as `go build` gets to
	// it (Go's own progress output), which combined with runProjectGoLive
	// writing straight to stdoutSink/stderrSink as the subprocess produces
	// output (rather than buffering it all and copying it in after Run
	// returns) is what actually makes a real "mage Build" show progress
	// instead of going silent for the whole compile.
	buildArgs := append([]string{"build", "-v", "-tags", "desktop,production", "-o", request.Root + string(filepath.Separator)}, packages...)
	err = runProjectGoLive(goExecutable, request.Root, buildArgs, stdoutSink, stderrSink)
	if err != nil {
		stderrSink.writeString(fmt.Sprintf("GOLC_BUILD_FAILED: %v\n", err))
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}
	stdoutSink.writeString("GOLC build: every project package compiled cleanly.\n")

	// internal/wails/app.go's Wails host spawns the Art-Net daemon from the
	// SEPARATE pinned copy at .tools/installs/golc_project/<platform>/bin/
	// golc-project[.exe] (bootstrap.installGoInstallTools's own
	// GOLC_BOOTSTRAP_PROJECT_BUILD step first provisions it) -- never the
	// repo-root golc-project[.exe] this route just wrote above. Without
	// this, "mage Build" gives every appearance of being up to date while
	// "mage Run" silently launches whatever daemon build `mage Bootstrap`
	// last happened to produce, however stale (observed live: a
	// day-old pinned copy kept failing with a since-fixed
	// GOLC_PLAYBACK_NO_ACTIVE_SCENE startup error well after the fix
	// landed and "mage Build" reported success). Refreshing it here too
	// keeps both binaries a single "mage Build" away from current source.
	stdoutSink.writeString("GOLC build: refreshing the pinned golc-project daemon binary.\n")
	if err := refreshPinnedGolcProject(goExecutable, request.Root, stdoutSink, stderrSink); err != nil {
		stderrSink.writeString(fmt.Sprintf("GOLC_BUILD_PINNED_INSTALL_FAILED: %v\n", err))
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}
	return Result{Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
}

// refreshPinnedGolcProject rebuilds the pinned golc-project[.exe] copy the
// Wails host's daemon-spawn path resolves (internal/wails/app.go's
// resolveDaemonExecutable), mirroring bootstrap.installGoInstallTools's own
// build invocation exactly so the two can never drift out of sync. A
// missing cmd/golc-project directory is not an error here (mirrors
// bootstrap's own defensive os.Stat guard) -- a synthetic test fixture
// repository legitimately has no such directory.
func refreshPinnedGolcProject(goExecutable, root string, liveStdout, liveStderr io.Writer) error {
	projectDir := filepath.Join(root, "cmd", "golc-project")
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	installPath := bootstrap.PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project")
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return fmt.Errorf("create pinned install directory: %w", err)
	}
	var stderrBuffer bytes.Buffer
	var stderr io.Writer = &stderrBuffer
	if liveStderr != nil {
		stderr = io.MultiWriter(&stderrBuffer, liveStderr)
	}
	err = runProjectGoLive(goExecutable, root, []string{"build", "-trimpath", "-o", installPath, "./cmd/golc-project"}, liveStdout, stderr)
	if err != nil {
		return fmt.Errorf("%w: %s", err, stderrBuffer.Bytes())
	}
	return nil
}

// magefilesImportSuffix is the import-path suffix of the magefiles package,
// excluded from "build every package" below: it is compiled only by the
// mage binary itself (which supplies its own generated main), so it has no
// func main() of its own and is not an independently buildable artifact.
// magefiles/*.go now also carry a "//go:build mage" tag, so an untagged
// "go list ./..." (as buildablePackages uses) already omits the package on
// its own; this explicit filter is deliberate defense-in-depth against that
// tag ever being dropped, not the only thing preventing the failure.
const magefilesImportSuffix = "/magefiles"

// buildablePackages lists every project Go package via the pinned
// toolchain and excludes the magefiles package, so "go build ./..." style
// verification never tries to link a package that intentionally has no
// func main() outside of mage's own generated wrapper.
func buildablePackages(goExecutable, root string) ([]string, error) {
	stdout, stderr, err := runProjectGo(goExecutable, root, []string{"list", "./..."})
	if err != nil {
		return nil, fmt.Errorf("list packages: %w: %s", err, stderr)
	}
	var packages []string
	for _, line := range strings.Split(strings.TrimSpace(string(stdout)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, magefilesImportSuffix) {
			continue
		}
		packages = append(packages, line)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("no buildable packages found")
	}
	return packages, nil
}
