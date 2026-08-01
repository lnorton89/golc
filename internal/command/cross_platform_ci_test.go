package command

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "cross-platform-ci",
	Summary: "Nonblocking cross-platform Mage workflow and portable Mage test compilation.",
})

func TestScopeCrossPlatformCI(t *testing.T) {
	root := commandParityRepositoryRoot(t)

	t.Run("production observation workflow is a closed nonblocking contract", func(t *testing.T) {
		path := filepath.Join(root, ".github", "workflows", "cross-platform-mage.yml")
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		lines := strings.Split(text, "\n")

		requireLine := func(want string) {
			t.Helper()
			for _, line := range lines {
				if line == want {
					return
				}
			}
			assert.Fail(t, "", "workflow missing exact line %q", want)
		}
		for _, line := range []string{
			"  pull_request:",
			"  workflow_dispatch:",
			"  contents: read",
			"    continue-on-error: true",
			"      fail-fast: false",
			`      GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC: "1"`,
			"        if: runner.os == 'Linux'",
			"        if: runner.os != 'Linux'",
			"        if: runner.os != 'Windows'",
			"        if: runner.os == 'Windows'",
		} {
			requireLine(line)
		}

		var runners []string
		var executable []string
		var actions []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- windows-latest") ||
				strings.HasPrefix(trimmed, "- ubuntu-latest") ||
				strings.HasPrefix(trimmed, "- macos-latest") {
				runners = append(runners, strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			}
			if strings.HasPrefix(trimmed, "run: ") {
				executable = append(executable, strings.TrimPrefix(trimmed, "run: "))
			}
			if strings.HasPrefix(trimmed, "uses: ") {
				actions = append(actions, strings.TrimPrefix(trimmed, "uses: "))
			}
		}
		wantRunners := []string{"windows-latest", "ubuntu-latest", "macos-latest"}
		require.True(t, reflect.DeepEqual(runners, wantRunners), "matrix runners = %v, want %v", runners, wantRunners)
		wantExecutable := []string{
			"sudo apt-get update && sudo apt-get install -y libx11-dev xvfb",
			`Xvfb :99 -screen 0 1024x768x24 & echo "DISPLAY=:99" >> "$GITHUB_ENV"`,
			`sh <(curl -L https://nixos.org/nix/install) --no-daemon && echo "NIX_PATH=nixpkgs=https://github.com/NixOS/nixpkgs/archive/nixpkgs-unstable.tar.gz" >> "$GITHUB_ENV"`,
			"sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0",
			"bash scripts/ci/install-pinned-mage.sh",
			"pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/ci/install-pinned-mage.ps1",
			"mage Bootstrap",
			"mage GenerateCheck",
			`. "$HOME/.nix-profile/etc/profile.d/nix.sh" && nix-shell shell.nix --run "mage CheckOffline"`,
			"mage CheckOffline",
			`. "$HOME/.nix-profile/etc/profile.d/nix.sh" && nix-shell shell.nix --run "mage Build"`,
			"mage Build",
			`. "$HOME/.nix-profile/etc/profile.d/nix.sh" && nix-shell shell.nix --run "mage Test"`,
			"mage Test",
			"mage PackageFoundation",
		}
		require.True(t, reflect.DeepEqual(executable, wantExecutable), "executable order = %v, want %v", executable, wantExecutable)
		require.True(t, reflect.DeepEqual(actions, []string{"actions/checkout@v4"}), "actions = %v, want only checkout", actions)

		lower := strings.ToLower(text)
		for _, forbidden := range []string{
			"secrets.", "${{ secrets", "environment:", "setup-go", "setup-node",
			"powershell", "golc.ps1", "linear_api_key", "linear_team_id",
			"mage pr", "upload-artifact", "release:", "publish",
			"supports linux", "supports macos", "linux support", "macos support",
			"qualified platform",
		} {
			assert.NotContains(t, lower, forbidden, "workflow contains forbidden token %q", forbidden)
		}
	})

	t.Run("pinned Go/Mage install scripts exist and read the committed toolchain pins", func(t *testing.T) {
		// This is a structural, network-free check only (D-02: generate/
		// check/build/test never open a network connection; only the
		// "bootstrap" route may). Actually downloading and verifying the
		// pinned Go/Mage archives was validated manually against the
		// real config/toolchain.toml before these scripts were
		// committed. Both toolchains are provisioned here, not just
		// Mage: Mage itself JIT-compiles the magefile package at every
		// invocation, so it always needs a Go compiler on PATH
		// regardless of how the mage binary itself was obtained, and
		// this project never trusts an ambient one for anything else
		// either (resolvePinnedGoExecutable never does a host PATH
		// lookup).
		for _, relative := range []string{
			filepath.Join("scripts", "ci", "install-pinned-mage.sh"),
			filepath.Join("scripts", "ci", "install-pinned-mage.ps1"),
		} {
			path := filepath.Join(root, relative)
			info, err := os.Stat(path)
			require.NoError(t, err, "stat %s: %v", relative, err)
			require.NotEqual(t, 0, info.Size(), "%s is empty", relative)
			raw, err := os.ReadFile(path)
			require.NoError(t, err, "read %s: %v", relative, err)
			text := string(raw)
			// Both scripts parameterize the section lookup by tool name
			// rather than spelling "toolchain.go"/"toolchain.mage"
			// literally, so check the generic section-path shape plus
			// that both tool names are actually invoked.
			for _, want := range []string{"toolchain.", ".platforms.", "archive_url", "archive_sha256", `"go"`, `"mage"`} {
				assert.Contains(t, text, want, "%s does not reference %q; it must read the pin from config/toolchain.toml, not duplicate it", relative, want)
			}
		}

		toolchainPath := filepath.Join(root, "config", "toolchain.toml")
		toolchainText, err := os.ReadFile(toolchainPath)
		require.NoError(t, err)
		for _, section := range []string{
			`[toolchain.go.platforms."windows-amd64"]`,
			`[toolchain.go.platforms."linux-amd64"]`,
			`[toolchain.go.platforms."darwin-amd64"]`,
			`[toolchain.go.platforms."darwin-arm64"]`,
			`[toolchain.mage.platforms."windows-amd64"]`,
			`[toolchain.mage.platforms."linux-amd64"]`,
			`[toolchain.mage.platforms."darwin-amd64"]`,
			`[toolchain.mage.platforms."darwin-arm64"]`,
		} {
			assert.Contains(t, string(toolchainText), section, "config/toolchain.toml is missing expected section %q the install scripts depend on", section)
		}
	})

	t.Run("Mage tests cross-compile for every configured contributor platform", func(t *testing.T) {
		goExecutable, err := resolvePinnedGoExecutable(root)
		require.NoError(t, err)
		platforms := []struct{ goos, goarch string }{
			{"windows", "amd64"},
			{"linux", "amd64"},
			{"linux", "arm64"},
			{"darwin", "amd64"},
			{"darwin", "arm64"},
		}
		for _, platform := range platforms {
			platform := platform
			t.Run(platform.goos+"-"+platform.goarch, func(t *testing.T) {
				suffix := ""
				if platform.goos == "windows" {
					suffix = ".exe"
				}
				output := filepath.Join(t.TempDir(), "magefiles.test"+suffix)
				cmd := exec.Command(goExecutable, "test", "-c", "-tags", "mage", "-o", output, "./magefiles")
				cmd.Dir = root
				environment := projectGoEnvironment(root)
				environment = upsertEnvironment(environment, "GOOS", platform.goos)
				environment = upsertEnvironment(environment, "GOARCH", platform.goarch)
				environment = upsertEnvironment(environment, "CGO_ENABLED", "0")
				cmd.Env = environment
				if combined, err := cmd.CombinedOutput(); err != nil {
					require.NoError(t, err, "cross-compile: %v\n%s", err, combined)
				}
				if info, err := os.Stat(output); err != nil || !info.Mode().IsRegular() {
					require.NoError(t, err)
					require.True(t, info.Mode().IsRegular(), "cross-compile output missing: %v", err)
				}
				if runtime.GOOS == "windows" && platform.goos == "windows" && platform.goarch == "amd64" {
					native := exec.Command(output, "-test.run", "^TestMagefileExportsAndImports$", "-test.count=1")
					native.Dir = filepath.Join(root, "magefiles")
					native.Env = environment
					if combined, err := native.CombinedOutput(); err != nil {
						require.NoError(t, err, "native Mage test execution: %v\n%s", err, combined)
					}
				}
			})
		}
	})
}
