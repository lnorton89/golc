package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDesignSystemUsage exercises only pure argument parsing -- no
// pinned-toolchain resolution or subprocess ever runs here.
func TestDesignSystemUsage(t *testing.T) {
	t.Run("no arguments is rejected", func(t *testing.T) {
		_, err := parseDesignSystemArgs(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_DESIGNSYSTEM_USAGE")
	})

	t.Run("two arguments is rejected", func(t *testing.T) {
		_, err := parseDesignSystemArgs([]string{"--static", "--unit"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_DESIGNSYSTEM_USAGE")
	})

	t.Run("an unsupported argument is rejected", func(t *testing.T) {
		_, err := parseDesignSystemArgs([]string{"--bogus"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "GOLC_DESIGNSYSTEM_USAGE")
	})

	for _, testCase := range []struct {
		argument string
		want     designSystemMode
	}{
		{"--static", designSystemModeStatic},
		{"--unit", designSystemModeUnit},
		{"--browser", designSystemModeBrowser},
	} {
		t.Run(testCase.argument+" selects the matching mode", func(t *testing.T) {
			mode, err := parseDesignSystemArgs([]string{testCase.argument})
			require.NoError(t, err)
			require.Equal(t, testCase.want, mode)
		})
	}

	t.Run("runDesignSystem rejects a malformed invocation with exit code 2", func(t *testing.T) {
		result := runDesignSystem(Request{Args: []string{"--bogus"}})
		require.Equal(t, 2, result.ExitCode)
		require.Contains(t, string(result.Stderr), "GOLC_DESIGNSYSTEM_USAGE")
	})
}

// TestDesignSystemArgv proves the exact, committed-entrypoint argv each
// mode constructs, independent of any subprocess ever running: these are
// the same forward-slash-literal argv the pinned-subprocess seam
// (runDesignSystemNode) below actually executes.
func TestDesignSystemArgv(t *testing.T) {
	require.Equal(t, []string{"scripts/design-system/check.mjs", "--all"}, designSystemStaticArgv())
	require.Equal(t, []string{"node_modules/vitest/vitest.mjs", "run", "scripts/design-system"}, designSystemUnitArgv())
	require.Equal(t,
		[]string{"node_modules/@playwright/test/cli.js", "test", "e2e/design-system", "--workers=1"},
		designSystemBrowserArgv())
}

// TestDesignSystemMissingToolchain proves every mode fails closed with a
// named diagnostic -- never a panic or an ambient-tool fallback -- when
// the pinned Node toolchain is not provisioned, using a synthetic
// repository root that declares no config/toolchain.toml. No real
// subprocess ever runs here.
func TestDesignSystemMissingToolchain(t *testing.T) {
	root := t.TempDir()
	for _, argument := range []string{"--static", "--unit", "--browser"} {
		t.Run(argument, func(t *testing.T) {
			result := runDesignSystem(Request{Args: []string{argument}, Root: root})
			require.Equal(t, 1, result.ExitCode)
			require.Contains(t, string(result.Stderr), "GOLC_BUILD_NODE_TOOLCHAIN_MISSING")
		})
	}
}

// TestDesignSystemStaticRoute runs the real whole-source DS001-DS10
// checker against the actual checkout root with the pinned Node
// toolchain (mirroring build_test.go's
// TestBuildRouteCompilesTheProductionRepository real-invocation
// pattern). Whole-source design-system parity is a moving target across
// this phase's migration plans (13-CONTEXT.md, this plan's own critical
// context), so this deliberately does not assert a clean result -- only
// that the pinned subprocess seam genuinely ran the real checker end to
// end: either a clean pass (stdout says so) or a real, parseable DS0xx
// diagnostic (never a route-level usage/routing failure, exit code 2).
func TestDesignSystemStaticRoute(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedNodeExecutable(root); err != nil {
		t.Skipf("pinned Node toolchain not provisioned: %v", err)
	}

	result := runDesignSystem(Request{Args: []string{"--static"}, Root: root})
	require.Contains(t, []int{0, 1}, result.ExitCode,
		"static route exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	if result.ExitCode == 0 {
		require.Contains(t, string(result.Stdout), "completed cleanly")
		return
	}
	require.Regexp(t, `DS0\d\d`, string(result.Stderr), "expected a real DS0xx diagnostic, got: %s", result.Stderr)
}

// TestDesignSystemUnitRoute runs the real design-system-scoped Vitest
// unit tests (scripts/design-system/*.test.ts) against the actual
// checkout root. Unlike the whole-source static check above, these tests
// exercise the checker's own logic (manifest loading, generation) rather
// than migrated application source, so they are expected to stay green
// independent of the ongoing frontend migration.
func TestDesignSystemUnitRoute(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedNodeExecutable(root); err != nil {
		t.Skipf("pinned Node toolchain not provisioned: %v", err)
	}

	result := runDesignSystem(Request{Args: []string{"--unit"}, Root: root})
	require.Equal(t, 0, result.ExitCode, "unit route exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	require.Contains(t, string(result.Stdout), "completed cleanly")
}

// TestDesignSystemBrowserDiscovery proves the browser route resolves the
// pinned Node executable and the checked-in @playwright/test CLI
// entrypoint, and that "e2e/design-system" selects exactly the
// design-system visual spec files -- without ever launching a browser or
// the dev server: appending "--list" to the exact production argv is
// Playwright's own fast, real discovery mode (used identically by
// 13-17-SUMMARY.md's own declared verify commands), so this stays fast
// and deterministic while still exercising the real pinned subprocess
// seam end to end.
func TestDesignSystemBrowserDiscovery(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedNodeExecutable(root); err != nil {
		t.Skipf("pinned Node toolchain not provisioned: %v", err)
	}

	listArgv := append(append([]string(nil), designSystemBrowserArgv()...), "--list")
	result := runDesignSystemNode(root, listArgv, nil, nil, "browser visual suite discovery", "GOLC_DESIGNSYSTEM_BROWSER_FAILED")
	require.Equal(t, 0, result.ExitCode, "browser discovery exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	require.Contains(t, string(result.Stdout), "design-system.calibration.spec.ts")
	require.NotContains(t, string(result.Stdout), "dialog-feasibility.spec.ts",
		"the e2e/design-system filter must not also select unrelated e2e specs")
}

// TestDesignSystemNodeResolvesFrontendDirectory proves runDesignSystemNode
// runs its child process inside frontend/ (not the repository root): a
// tiny inline Node script prints process.cwd(), and one of the buffered
// output lines must equal frontendDir exactly.
func TestDesignSystemNodeResolvesFrontendDirectory(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedNodeExecutable(root); err != nil {
		t.Skipf("pinned Node toolchain not provisioned: %v", err)
	}

	frontendDir := filepath.Join(root, "frontend")
	info, statErr := os.Stat(frontendDir)
	require.NoError(t, statErr)
	require.True(t, info.IsDir())

	result := runDesignSystemNode(root, []string{"-e", "console.log(process.cwd())"}, nil, nil, "cwd probe", "GOLC_DESIGNSYSTEM_PROBE_FAILED")
	require.Equal(t, 0, result.ExitCode, "cwd probe exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	found := false
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		if strings.TrimSpace(line) == frontendDir {
			found = true
			break
		}
	}
	require.True(t, found, "expected a line equal to %q in stdout, got: %s", frontendDir, result.Stdout)
}
