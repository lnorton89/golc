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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
const (
	dialogProofGoOverlayEnvName  = "GOLC_DIALOG_PROOF_GO_OVERLAY"
	dialogProofGoModfileEnvName  = "GOLC_DIALOG_PROOF_GO_MODFILE"
	dialogProofGoOverlayBuildTag = "golc_dialog_proof_overlay"
)

func projectBuildGoArguments(arguments []string) []string {
	overlay := strings.TrimSpace(os.Getenv(dialogProofGoOverlayEnvName))
	modfile := strings.TrimSpace(os.Getenv(dialogProofGoModfileEnvName))
	if overlay == "" || modfile == "" || len(arguments) == 0 || arguments[0] != "build" {
		return append([]string(nil), arguments...)
	}
	result := make([]string, 0, len(arguments)+3)
	result = append(result, arguments[0], "-modfile="+modfile, "-overlay="+overlay)
	foundTags := false
	for index := 1; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "-tags" && index+1 < len(arguments):
			result = append(result, argument, arguments[index+1]+","+dialogProofGoOverlayBuildTag)
			index++
			foundTags = true
		case strings.HasPrefix(argument, "-tags="):
			result = append(result, argument+","+dialogProofGoOverlayBuildTag)
			foundTags = true
		default:
			result = append(result, argument)
		}
	}
	if !foundTags {
		result = append(result[:3], append([]string{"-tags=" + dialogProofGoOverlayBuildTag}, result[3:]...)...)
	}
	return result
}

func runProjectGoLive(goExecutable, root string, arguments []string, stdout, stderr io.Writer) error {
	execution := exec.Command(goExecutable, projectBuildGoArguments(arguments)...)
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
		return bootstrap.NodeInstallation{}, errors.New("GOLC_BUILD_NODE_TOOLCHAIN_MISSING: config/toolchain.toml does not pin toolchain.node.version")
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
	// Vite's own build output (every rebuilt module, chunk sizes, gzip
	// stats) is captured into npmOutput rather than teed live to
	// stdout/stderr -- a successful build only needs the concise
	// start/done lines below, and the full dump is only useful for
	// diagnosing a failure, where it is surfaced in full.
	var npmOutput bytes.Buffer
	execution := exec.Command(node.Executable, node.NPMCLI, "run", "build")
	execution.Dir = frontendDir
	execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
	execution.Stdout = &npmOutput
	execution.Stderr = &npmOutput
	if err := execution.Run(); err != nil {
		stderr.Write(npmOutput.Bytes())
		return fmt.Errorf("GOLC_BUILD_FRONTEND_FAILED: %w", err)
	}
	if info, statErr := os.Stat(distIndexPath); statErr != nil || !info.Mode().IsRegular() {
		stderr.Write(npmOutput.Bytes())
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
			return "", errors.New("GOLC_BUILD_USAGE: --scope requires a scope name; usage: build [--scope <scope-name>]")
		}
		return args[1], nil
	}
	if len(args) == 1 && strings.HasPrefix(args[0], "--scope=") {
		value := strings.TrimPrefix(args[0], "--scope=")
		if value == "" {
			return "", errors.New("GOLC_BUILD_USAGE: --scope requires a scope name; usage: build [--scope <scope-name>]")
		}
		return value, nil
	}
	return "", fmt.Errorf("GOLC_BUILD_USAGE: unsupported argument %q; usage: build [--scope <scope-name>]", args[0])
}

// ensureGoModCacheFresh fails fast, before any `go list`/`go build`
// invocation, when the project-local GOMODCACHE (bootstrap.ProjectCacheLayout
// .GoModCache, populated by `mage Bootstrap`'s runGoPhase) was never
// populated for the current go.mod/go.sum, or was populated for an older
// one. Without this check a stale/missing cache surfaces only deep inside
// the pinned-toolchain `go build` invocation below (GOFLAGS=-mod=readonly,
// GOPROXY=off, D-02 -- see projectGoEnvironment) as a wall of per-file
// "module lookup disabled by GOPROXY=off" errors with no indication that
// running 'mage Bootstrap' is the fix.
func ensureGoModCacheFresh(root string) error {
	layout, err := bootstrap.NewProjectCacheLayout(root)
	if err != nil {
		return fmt.Errorf("GOLC_BUILD_BOOTSTRAP_REQUIRED: resolve project cache layout: %w", err)
	}
	recorded, err := os.ReadFile(layout.GoModLockManifestPath())
	if err != nil {
		return fmt.Errorf("GOLC_BUILD_BOOTSTRAP_REQUIRED: %s: the project-local Go module cache has never been populated; run 'mage Bootstrap' first: %w", layout.GoModLockManifestPath(), err)
	}
	current, err := bootstrap.GoModLockSignature(root)
	if err != nil {
		return fmt.Errorf("GOLC_BUILD_BOOTSTRAP_REQUIRED: %w", err)
	}
	if strings.TrimSpace(string(recorded)) != current {
		return errors.New("GOLC_BUILD_BOOTSTRAP_REQUIRED: go.mod/go.sum changed since the last 'mage Bootstrap'; run 'mage Bootstrap' again before building")
	}
	return nil
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

	stdoutSink := &progressSink{live: request.Stdout}
	stderrSink := &progressSink{live: request.Stderr}

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

	stdoutSink.writeString("GOLC build: resolving pinned Go toolchain...\n")
	goExecutable, err := resolvePinnedGoExecutable(request.Root)
	if err != nil {
		stderrSink.writeString(err.Error() + "\n")
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}

	stdoutSink.writeString("GOLC build: checking project-local Go module cache against go.mod/go.sum...\n")
	if err := ensureGoModCacheFresh(request.Root); err != nil {
		stderrSink.writeString(err.Error() + "\n")
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}

	stdoutSink.writeString("GOLC build: checking frontend dist freshness (hashing frontend/ source tree)...\n")
	if err := ensureFrontendDistFresh(request.Root, stdoutSink, stderrSink); err != nil {
		stderrSink.writeString(err.Error() + "\n")
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}

	stdoutSink.writeString("GOLC build: listing project packages (go list ./...)...\n")
	packages, err := buildablePackages(goExecutable, request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: fmt.Appendf(nil, "GOLC_BUILD_FAILED: %v\n", err)}
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
	buildArgs := append([]string{"build", "-v", "-trimpath", "-tags", "desktop,production", "-o", request.Root + string(filepath.Separator)}, packages...)
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
	rootProjectBinary := filepath.Join(request.Root, bootstrap.ExecutableName("golc-project"))
	if err := refreshPinnedGolcProject(request.Root, rootProjectBinary, stdoutSink); err != nil {
		stderrSink.writeString(fmt.Sprintf("GOLC_BUILD_PINNED_INSTALL_FAILED: %v\n", err))
		return Result{ExitCode: 1, Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
	}
	return Result{Stdout: stdoutSink.buffered(), Stderr: stderrSink.buffered()}
}

// refreshPinnedGolcProject refreshes the pinned golc-project[.exe] copy the
// Wails host's daemon-spawn path resolves (internal/wails/app.go's
// resolveDaemonExecutable) by copying rootBinaryPath -- the same
// -trimpath'd binary the multi-package build just above this call already
// produced at the repository root -- rather than re-running `go build
// ./cmd/golc-project` a second time. That second invocation used to cost a
// full extra compile+link of the whole dependency graph on every single
// "mage build": bootstrap.installGoInstallTools's build step and this one
// used identical flags, but GOCACHE still treats each `go build` process
// invocation independently, so nothing was actually shared between the two
// back-to-back builds in the same route. A missing cmd/golc-project
// directory is not an error here (mirrors bootstrap's own defensive
// os.Stat guard) -- a synthetic test fixture repository legitimately has
// no such directory.
func refreshPinnedGolcProject(root, rootBinaryPath string, liveStdout io.Writer) error {
	projectDir := filepath.Join(root, "cmd", "golc-project")
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	installPath := bootstrap.PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project")
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return fmt.Errorf("create pinned install directory: %w", err)
	}
	source, err := os.Open(rootBinaryPath)
	if err != nil {
		return fmt.Errorf("open freshly built %s: %w", rootBinaryPath, err)
	}
	defer source.Close()
	destination, err := os.OpenFile(installPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create pinned install binary: %w", err)
	}
	defer destination.Close()
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy pinned install binary: %w", err)
	}
	if liveStdout != nil {
		fmt.Fprintf(liveStdout, "GOLC build: copied %s -> %s\n", rootBinaryPath, installPath)
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

// buildablePackagesCacheName is where buildablePackages persists the last
// package list it resolved via a real `go list ./...`, keyed by
// packageListSignature. A real invocation on this repository was measured
// live at ~25-30s of continuous single-core CPU time with essentially no
// disk-I/O-wait component (Get-Process sampling during the call showed
// CPU time tracking wall time 1:1) -- consistent with something hooking
// every one of the ~2,000 first-party .go files' opens synchronously in
// go.exe's own thread (a security product's inline scan-on-open is the
// prime suspect, though it could not be confirmed without admin-level
// filter-driver enumeration) rather than the file parsing itself being
// slow. Since that cost is paid in full on every single "mage build" even
// when the buildable package set is byte-identical to the last one,
// caching it against a signature that never opens a .go file's content
// turns the common no-package-added-or-removed case from ~30s into a
// handful of file stats.
const buildablePackagesCacheName = "go-buildable-packages.json"

type buildablePackagesCache struct {
	Signature string   `json:"signature"`
	Packages  []string `json:"packages"`
}

// packageListSignature fingerprints every .go file under root (path,
// size, and mtime -- never file content, since reading content is exactly
// the expensive part a cache hit needs to avoid) plus go.mod/go.sum's
// content, sufficient to detect anything that could change `go list
// ./...`'s output: a file added, removed, edited, or a dependency change.
// node_modules/.git/.tools/any dot-directory are skipped as never
// containing project Go source.
func packageListSignature(root string) (string, error) {
	hasher := sha256.New()
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if relative == "." {
			return nil
		}
		base := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(base, ".") || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(base, ".go") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		fmt.Fprintf(hasher, "%s %d %d\n", filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			return "", readErr
		}
		hasher.Write([]byte(name + "\n"))
		hasher.Write(content)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func buildablePackagesCachePath(root string) string {
	return filepath.Join(root, ".tools", "cache", buildablePackagesCacheName)
}

// cachedBuildablePackages returns a previous buildablePackages result if
// its recorded signature still matches signature -- any read/decode
// failure is treated as a cache miss, never an error, since the fallback
// (a real `go list ./...`) is always correct, just slower.
func cachedBuildablePackages(root, signature string) ([]string, bool) {
	raw, err := os.ReadFile(buildablePackagesCachePath(root))
	if err != nil {
		return nil, false
	}
	var cache buildablePackagesCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, false
	}
	if cache.Signature != signature || len(cache.Packages) == 0 {
		return nil, false
	}
	return cache.Packages, true
}

// writeBuildablePackagesCache persists packages under signature; any
// failure to do so is non-fatal (the next call just pays for a real `go
// list ./...` again).
func writeBuildablePackagesCache(root, signature string, packages []string) {
	encoded, err := json.Marshal(buildablePackagesCache{Signature: signature, Packages: packages})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(buildablePackagesCachePath(root)), 0o755)
	_ = os.WriteFile(buildablePackagesCachePath(root), encoded, 0o644)
}

// buildablePackages lists every project Go package via the pinned
// toolchain and excludes the magefiles package, so "go build ./..." style
// verification never tries to link a package that intentionally has no
// func main() outside of mage's own generated wrapper. It first tries a
// cached result keyed by packageListSignature, only falling back to a
// real `go list ./...` when the project's own source tree has actually
// changed (see buildablePackagesCacheName).
func buildablePackages(goExecutable, root string) ([]string, error) {
	signature, sigErr := packageListSignature(root)
	if sigErr == nil {
		if cached, ok := cachedBuildablePackages(root, signature); ok {
			return cached, nil
		}
	}
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
		return nil, errors.New("no buildable packages found")
	}
	if sigErr == nil {
		writeBuildablePackagesCache(root, signature, packages)
	}
	return packages, nil
}
