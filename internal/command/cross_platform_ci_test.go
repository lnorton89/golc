package command

import (
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
		if err != nil {
			t.Fatalf("read observation workflow: %v", err)
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		lines := strings.Split(text, "\n")

		requireLine := func(want string) {
			t.Helper()
			for _, line := range lines {
				if line == want {
					return
				}
			}
			t.Errorf("workflow missing exact line %q", want)
		}
		for _, line := range []string{
			"  pull_request:",
			"  workflow_dispatch:",
			"  contents: read",
			"    continue-on-error: true",
			"      fail-fast: false",
			`      GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC: "1"`,
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
		if !reflect.DeepEqual(runners, wantRunners) {
			t.Fatalf("matrix runners = %v, want %v", runners, wantRunners)
		}
		wantExecutable := []string{
			"go install github.com/magefile/mage@v1.17.2",
			"mage Bootstrap",
			"mage GenerateCheck",
			"mage CheckOffline",
			"mage Build",
			"mage Test",
			"mage PackageFoundation",
		}
		if !reflect.DeepEqual(executable, wantExecutable) {
			t.Fatalf("executable order = %v, want %v", executable, wantExecutable)
		}
		if !reflect.DeepEqual(actions, []string{"actions/checkout@v4"}) {
			t.Fatalf("actions = %v, want only checkout", actions)
		}

		lower := strings.ToLower(text)
		for _, forbidden := range []string{
			"secrets.", "${{ secrets", "environment:", "setup-go", "setup-node",
			"powershell", "golc.ps1", "linear_api_key", "linear_team_id",
			"mage pr", "upload-artifact", "release:", "publish",
			"supports linux", "supports macos", "linux support", "macos support",
			"qualified platform",
		} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("workflow contains forbidden token %q", forbidden)
			}
		}
	})

	t.Run("Mage tests cross-compile for every configured contributor platform", func(t *testing.T) {
		goExecutable, err := resolvePinnedGoExecutable(root)
		if err != nil {
			t.Fatal(err)
		}
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
				cmd := exec.Command(goExecutable, "test", "-c", "-o", output, "./magefiles")
				cmd.Dir = root
				environment := projectGoEnvironment(root)
				environment = upsertEnvironment(environment, "GOOS", platform.goos)
				environment = upsertEnvironment(environment, "GOARCH", platform.goarch)
				environment = upsertEnvironment(environment, "CGO_ENABLED", "0")
				cmd.Env = environment
				if combined, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("cross-compile: %v\n%s", err, combined)
				}
				if info, err := os.Stat(output); err != nil || !info.Mode().IsRegular() {
					t.Fatalf("cross-compile output missing: %v", err)
				}
				if runtime.GOOS == "windows" && platform.goos == "windows" && platform.goarch == "amd64" {
					native := exec.Command(output, "-test.run", "^TestMagefileExportsAndImports$", "-test.count=1")
					native.Dir = filepath.Join(root, "magefiles")
					native.Env = environment
					if combined, err := native.CombinedOutput(); err != nil {
						t.Fatalf("native Mage test execution: %v\n%s", err, combined)
					}
				}
			})
		}
	})
}
