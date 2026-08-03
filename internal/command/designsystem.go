// designsystem.go is the design-system enforcement command file (Plan
// 13-18, CONTEXT D-09/D-10): it owns the "designsystem" scope and
// self-registers the exact route through the package declaration
// entrypoints (CONTEXT D-03) -- the central router is never edited.
//
// frontend/design-system/ already ships a complete, battle-tested
// enforcement surface: scripts/design-system/check.mjs (the parser-backed
// DS001-DS010 policy checker), a design-system-scoped Vitest unit suite
// (scripts/design-system/*.test.ts), and a Playwright visual suite
// (e2e/design-system.*.spec.ts). This file's job is wiring that existing
// surface into the pinned, offline-aware Go command graph -- not
// reimplementing or modifying any of the checker's own logic.
//
// Three modes compose the same enforcement:
//   - "designsystem --static" runs the whole-source DS001-DS010 policy
//     checker (scripts/design-system/check.mjs --all).
//   - "designsystem --unit" runs only the design-system-scoped Vitest
//     tests (scripts/design-system/*.test.ts), not the full frontend
//     suite.
//   - "designsystem --browser" runs the serialized Playwright visual
//     suite (e2e/design-system.*.spec.ts), one worker at a time so
//     screenshot comparisons never race each other.
//
// All three resolve the pinned project-local Node toolchain exactly the
// way build.go's runBuildNodeScope already does (CONTEXT D-01/D-02):
// never an ambient `npx`/global tool, and never a second, independently
// resolved Node installation. Each invocation runs a checked-in
// entrypoint directly (check.mjs, node_modules/vitest/vitest.mjs,
// node_modules/@playwright/test/cli.js) rather than shelling out through
// `npm run`, mirroring runBuildNodeScope's direct-binary pinned
// subprocess seam rather than inventing a second invocation style.
package command

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "designsystem",
	Summary: "Frontend design-system enforcement: static policy, unit, and browser visual checks.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "designsystem",
	Summary: "Run frontend design-system enforcement: designsystem --static|--unit|--browser.",
	Handler: runDesignSystem,
})

// designSystemMode is the parsed shape of one "designsystem" invocation:
// exactly one of --static, --unit, or --browser.
type designSystemMode string

const (
	designSystemModeStatic  designSystemMode = "static"
	designSystemModeUnit    designSystemMode = "unit"
	designSystemModeBrowser designSystemMode = "browser"
)

// parseDesignSystemArgs accepts exactly one of the three supported forms.
func parseDesignSystemArgs(args []string) (designSystemMode, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("GOLC_DESIGNSYSTEM_USAGE: usage: designsystem --static|--unit|--browser")
	}
	switch args[0] {
	case "--static":
		return designSystemModeStatic, nil
	case "--unit":
		return designSystemModeUnit, nil
	case "--browser":
		return designSystemModeBrowser, nil
	default:
		return "", fmt.Errorf(
			"GOLC_DESIGNSYSTEM_USAGE: unsupported argument %q; usage: designsystem --static|--unit|--browser", args[0])
	}
}

// runDesignSystem serves the self-registered "designsystem" route.
func runDesignSystem(request Request) Result {
	mode, err := parseDesignSystemArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	switch mode {
	case designSystemModeStatic:
		return runDesignSystemNode(request.Root, designSystemStaticArgv(), request.Stdout, request.Stderr,
			"static whole-source policy check (DS001-DS010)", "GOLC_DESIGNSYSTEM_STATIC_FAILED")
	case designSystemModeUnit:
		return runDesignSystemNode(request.Root, designSystemUnitArgv(), request.Stdout, request.Stderr,
			"design-system-scoped unit tests", "GOLC_DESIGNSYSTEM_UNIT_FAILED")
	default:
		return runDesignSystemNode(request.Root, designSystemBrowserArgv(), request.Stdout, request.Stderr,
			"serialized browser visual suite", "GOLC_DESIGNSYSTEM_BROWSER_FAILED")
	}
}

// designSystemStaticArgv is the exact whole-source invocation of the
// committed DS001-DS010 checker: `node scripts/design-system/check.mjs
// --all`. Relative paths are forward-slash literals (not filepath.Join):
// they are arguments interpreted by Node/Playwright, not Go-side
// filesystem operations, and every existing pinned-subprocess invocation
// in this repository (build.go's ensureFrontendDistFresh, scripts/ci/
// run-packaged-dialog-proof.ps1's Playwright invocation) already uses
// this exact convention on Windows.
func designSystemStaticArgv() []string {
	return []string{"scripts/design-system/check.mjs", "--all"}
}

// designSystemUnitArgv runs only the design-system-scoped Vitest tests
// (scripts/design-system/check.test.ts, manifest.test.ts) directly
// through the checked-in vitest entrypoint -- never the full frontend
// suite, and never an ambient `npx vitest`.
func designSystemUnitArgv() []string {
	return []string{"node_modules/vitest/vitest.mjs", "run", "scripts/design-system"}
}

// designSystemBrowserArgv runs the serialized Playwright visual suite
// directly through the checked-in @playwright/test CLI entrypoint --
// never an ambient `npx playwright`. "e2e/design-system" is a filename
// filter (every design-system visual spec shares this exact prefix:
// e2e/design-system.*.spec.ts), and --workers=1 keeps every screenshot
// comparison serialized so two specs can never race the same viewport.
func designSystemBrowserArgv() []string {
	return []string{"node_modules/@playwright/test/cli.js", "test", "e2e/design-system", "--workers=1"}
}

// runDesignSystemNode resolves the pinned project-local Node executable
// and runs argv inside frontend/, mirroring runBuildNodeScope's exact
// pinned-subprocess seam (CONTEXT D-01/D-02): never an ambient
// `npx`/global tool, and never a second, independently resolved Node
// installation. label/failureCode name the invocation in progress output
// and in the failure diagnostic respectively.
func runDesignSystemNode(root string, argv []string, liveStdout, liveStderr io.Writer, label, failureCode string) Result {
	nodeExecutable, err := resolvePinnedNodeExecutable(root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	frontendDir := filepath.Join(root, "frontend")

	stdout := &progressSink{live: liveStdout}
	stderr := &progressSink{live: liveStderr}
	stdout.writeString(fmt.Sprintf("GOLC designsystem: %s -> node %s\n", label, strings.Join(argv, " ")))

	execution := exec.Command(nodeExecutable, argv...)
	execution.Dir = frontendDir
	execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
	execution.Stdout = stdout
	execution.Stderr = stderr
	err = execution.Run()

	if err != nil {
		stderr.writeString(fmt.Sprintf("%s: %s: %v\n", failureCode, label, err))
		return Result{ExitCode: 1, Stdout: stdout.buffered(), Stderr: stderr.buffered()}
	}
	stdout.writeString(fmt.Sprintf("GOLC designsystem: %s completed cleanly.\n", label))
	return Result{Stdout: stdout.buffered(), Stderr: stderr.buffered()}
}
