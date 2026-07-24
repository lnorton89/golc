package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type engineFakeSource struct {
	payload map[string][]byte
	calls   []string
}

func (source *engineFakeSource) Fetch(rawURL string) (io.ReadCloser, error) {
	source.calls = append(source.calls, rawURL)
	payload, ok := source.payload[rawURL]
	if !ok {
		return nil, fmt.Errorf("unexpected source URL %q", rawURL)
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

type processCall struct {
	executable string
	dir        string
	args       []string
	env        map[string]string
}

type engineFakeRunner struct {
	calls       []processCall
	moduleGraph string
	mutateLock  bool
}

func (runner *engineFakeRunner) Run(_ context.Context, request processRequest) ([]byte, error) {
	call := processCall{
		executable: request.Executable,
		dir:        request.Dir,
		args:       append([]string(nil), request.Args...),
		env:        cloneEngineTestMap(request.Env),
	}
	runner.calls = append(runner.calls, call)
	if runner.mutateLock && len(runner.calls) == 1 {
		if err := os.WriteFile(filepath.Join(request.Dir, "go.mod"), []byte("mutated\n"), 0o644); err != nil {
			return nil, err
		}
	}
	if len(request.Args) >= 4 && request.Args[0] == "build" && request.Args[1] == "-trimpath" && request.Args[2] == "-o" {
		if err := os.MkdirAll(filepath.Dir(request.Args[3]), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(request.Args[3], []byte("built\n"), 0o755); err != nil {
			return nil, err
		}
	}
	if strings.Join(request.Args, " ") == "list -m all" {
		return []byte(runner.moduleGraph), nil
	}
	return nil, nil
}

func cloneEngineTestMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func platformToolArchive(t *testing.T, root, tool, version string) (path string, digest string, archiveRoot string) {
	t.Helper()
	layout, err := platformArchiveLayout(tool, version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("platformArchiveLayout: %v", err)
	}
	archiveRoot = layout.Root
	switch layout.Format {
	case ".zip":
		returnPath, returnDigest := buildZipEntries(t, root, layout.FileName, []testArchiveEntry{
			{Name: filepath.ToSlash(filepath.Join(layout.Root, layout.Executable)), Body: tool + " executable\n", Mode: 0o755},
		})
		return returnPath, returnDigest, archiveRoot
	case ".tar.gz":
		returnPath, returnDigest := buildTarGzEntries(t, root, layout.FileName, []testArchiveEntry{
			{Name: filepath.ToSlash(filepath.Join(layout.Root, layout.Executable)), Body: tool + " executable\n", Mode: 0o755},
		})
		return returnPath, returnDigest, archiveRoot
	default:
		t.Fatalf("unsupported test archive format %q", layout.Format)
		return "", "", ""
	}
}

func writeEngineRepository(t *testing.T) (root string, source *engineFakeSource, goURL string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "golc-project"), 0o755); err != nil {
		t.Fatalf("mkdir command: %v", err)
	}
	goArchive, goDigest, _ := platformToolArchive(t, root, "go", "1.26.5")
	mageArchive, mageDigest, _ := platformToolArchive(t, root, "mage", "1.17.2")
	fixtureArchive, fixtureDigest := buildZipEntries(t, root, "fixture.zip", []testArchiveEntry{
		{Name: "bin/fixture", Body: "fixture\n", Mode: 0o755},
	})
	goURL = "https://go.dev/dl/" + filepath.Base(goArchive)
	mageURL := "https://github.com/magefile/mage/releases/download/v1.17.2/" + filepath.Base(mageArchive)
	fixtureURL := "https://fixtures.example.invalid/tool/" + filepath.Base(fixtureArchive)
	manifest := fmt.Sprintf(`schema_version = 2

[cache]
downloads = ".tools/cache/downloads"
gomodcache = ".tools/cache/go-mod"
gocache = ".tools/cache/go-build"

[tools.fixture]
archive_url = %q
archive_sha256 = %q
official_host = "fixtures.example.invalid"
official_path_prefix = "/tool/"

[toolchain.go]
version = "1.26.5"
official_host = "go.dev"
official_path_prefix = "/dl/"

[toolchain.go.platforms.%q]
archive_url = %q
archive_sha256 = %q

[toolchain.mage]
version = "1.17.2"
official_host = "github.com"
official_path_prefix = "/magefile/mage/releases/download/"

[toolchain.mage.platforms.%q]
archive_url = %q
archive_sha256 = %q
`, fixtureURL, fixtureDigest, PlatformKey(), goURL, goDigest, PlatformKey(), mageURL, mageDigest)
	if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/test\n\ngo 1.26.5\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("sum\n"), 0o644); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}
	goBytes, _ := os.ReadFile(goArchive)
	mageBytes, _ := os.ReadFile(mageArchive)
	fixtureBytes, _ := os.ReadFile(fixtureArchive)
	source = &engineFakeSource{payload: map[string][]byte{
		goURL:      goBytes,
		mageURL:    mageBytes,
		fixtureURL: fixtureBytes,
	}}
	return root, source, goURL
}

func TestScopeBootstrapEngine(t *testing.T) {
	t.Run("explicit platform selector validates all committed Go and Node pins", func(t *testing.T) {
		type pinCase struct {
			tool, version, goos, goarch, url, sha string
		}
		cases := []pinCase{
			{"go", "1.26.5", "windows", "amd64", "https://go.dev/dl/go1.26.5.windows-amd64.zip", "97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38"},
			{"go", "1.26.5", "linux", "amd64", "https://go.dev/dl/go1.26.5.linux-amd64.tar.gz", "5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"},
			{"go", "1.26.5", "linux", "arm64", "https://go.dev/dl/go1.26.5.linux-arm64.tar.gz", "fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49"},
			{"go", "1.26.5", "darwin", "amd64", "https://go.dev/dl/go1.26.5.darwin-amd64.tar.gz", "6231d8d3b8f5552ec6cbf6d685bdd5482e1e703214b120e89b3bf0d7bf1ef725"},
			{"go", "1.26.5", "darwin", "arm64", "https://go.dev/dl/go1.26.5.darwin-arm64.tar.gz", "efb87ff28af9a188d0536ef5d42e63dd52ba8263cd7344a993cc48dd11dedb6a"},
			{"node", "24.18.0", "windows", "amd64", "https://nodejs.org/dist/v24.18.0/node-v24.18.0-win-x64.zip", "0ae68406b42d7725661da979b1403ec9926da205c6770827f33aac9d8f26e821"},
			{"node", "24.18.0", "linux", "amd64", "https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-x64.tar.gz", "783130984963db7ba9cbd01089eaf2c2efb055c7c1693c943174b967b3050cb8"},
			{"node", "24.18.0", "linux", "arm64", "https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-arm64.tar.gz", "6b4484c2190274175df9aa8f28e2d758a819cb1c1fe6ab481e2f95b463ab8508"},
			{"node", "24.18.0", "darwin", "amd64", "https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-x64.tar.gz", "dfd0dbd3e721503434df7b7205e719f61b3a3a31b2bcf9729b8b91fea240f080"},
			{"node", "24.18.0", "darwin", "arm64", "https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-arm64.tar.gz", "e1a97e14c99c803e96c7339403282ea05a499c32f8d83defe9ef5ec66f979ed1"},
		}
		for _, testCase := range cases {
			parent := toolchainManifestPin{
				Version: testCase.version, OfficialHost: "example.invalid", OfficialPathPrefix: "/",
				Platforms: map[string]platformArchivePin{
					testCase.goos + "-" + testCase.goarch: {ArchiveURL: testCase.url, ArchiveSHA256: testCase.sha},
				},
			}
			pin, err := selectPlatformPinFor(testCase.tool, parent, testCase.goos, testCase.goarch)
			if err != nil {
				t.Fatalf("%s/%s-%s: %v", testCase.tool, testCase.goos, testCase.goarch, err)
			}
			if pin.ArchiveURL != testCase.url || pin.ArchiveSHA256 != testCase.sha {
				t.Fatalf("%s/%s-%s selected %+v", testCase.tool, testCase.goos, testCase.goarch, pin)
			}
		}
	})

	t.Run("explicit platform selector rejects absent and mismatched assets", func(t *testing.T) {
		parent := toolchainManifestPin{
			Version: "1.26.5",
			Platforms: map[string]platformArchivePin{
				"linux-arm64": {ArchiveURL: "https://go.dev/dl/go1.26.5.linux-amd64.tar.gz", ArchiveSHA256: strings.Repeat("a", 64)},
			},
		}
		if _, err := selectPlatformPinFor("go", parent, "darwin", "arm64"); err == nil {
			t.Fatal("missing explicit platform unexpectedly selected")
		}
		if _, err := selectPlatformPinFor("go", parent, "linux", "arm64"); err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_PLATFORM_MISMATCH") {
			t.Fatalf("expected platform mismatch, got %v", err)
		}
	})

	t.Run("PlatformKey and pure platform layouts are exact", func(t *testing.T) {
		if got, want := PlatformKey(), runtime.GOOS+"-"+runtime.GOARCH; got != want {
			t.Fatalf("PlatformKey() = %q, want %q", got, want)
		}
		cases := []struct {
			tool, version, goos, goarch string
			file, root, executable      string
		}{
			{"go", "1.26.5", "windows", "amd64", "go1.26.5.windows-amd64.zip", "go", filepath.Join("bin", "go.exe")},
			{"go", "1.26.5", "linux", "amd64", "go1.26.5.linux-amd64.tar.gz", "go", filepath.Join("bin", "go")},
			{"go", "1.26.5", "darwin", "arm64", "go1.26.5.darwin-arm64.tar.gz", "go", filepath.Join("bin", "go")},
			{"node", "24.18.0", "windows", "amd64", "node-v24.18.0-win-x64.zip", "node-v24.18.0-win-x64", "node.exe"},
			{"node", "24.18.0", "linux", "amd64", "node-v24.18.0-linux-x64.tar.gz", "node-v24.18.0-linux-x64", filepath.Join("bin", "node")},
			{"node", "24.18.0", "darwin", "arm64", "node-v24.18.0-darwin-arm64.tar.gz", "node-v24.18.0-darwin-arm64", filepath.Join("bin", "node")},
			{"mage", "1.17.2", "windows", "amd64", "mage_1.17.2_Windows-64bit.zip", "", "mage.exe"},
			{"mage", "1.17.2", "linux", "amd64", "mage_1.17.2_Linux-64bit.tar.gz", "", "mage"},
			{"mage", "1.17.2", "linux", "arm64", "mage_1.17.2_Linux-ARM64.tar.gz", "", "mage"},
			{"mage", "1.17.2", "darwin", "amd64", "mage_1.17.2_macOS-64bit.tar.gz", "", "mage"},
			{"mage", "1.17.2", "darwin", "arm64", "mage_1.17.2_macOS-ARM64.tar.gz", "", "mage"},
		}
		for _, testCase := range cases {
			layout, err := platformArchiveLayout(testCase.tool, testCase.version, testCase.goos, testCase.goarch)
			if err != nil {
				t.Fatalf("%s/%s: %v", testCase.goos, testCase.tool, err)
			}
			if layout.FileName != testCase.file || layout.Root != testCase.root || layout.Executable != testCase.executable {
				t.Fatalf("%s/%s: got %+v", testCase.goos, testCase.tool, layout)
			}
		}
		if got, want := ExecutableName("golc-project"), "golc-project"+map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]; got != want {
			t.Fatalf("ExecutableName(golc-project) = %q, want %q", got, want)
		}
		for _, unsafe := range []string{"", ".", "..", "bin/golc-project", `bin\golc-project`} {
			if got := ExecutableName(unsafe); got != "" {
				t.Fatalf("ExecutableName(%q) = %q, want rejection", unsafe, got)
			}
		}
		installRoot := filepath.Join("repo", ".tools", "installs", "golc_project")
		if got, want := PlatformExecutablePath(installRoot, "golc-project"), filepath.Join(installRoot, PlatformKey(), "bin", ExecutableName("golc-project")); got != want {
			t.Fatalf("PlatformExecutablePath() = %q, want %q", got, want)
		}
		if got := PlatformExecutablePath(installRoot, "../golc-project"); got != "" {
			t.Fatalf("PlatformExecutablePath accepted unsafe base: %q", got)
		}
	})

	t.Run("Node installation is discovered by verified filesystem shape", func(t *testing.T) {
		writeNodePayload := func(t *testing.T, installDir, rootName string) NodeInstallation {
			t.Helper()
			layout, err := platformArchiveLayout("node", "24.18.0", runtime.GOOS, runtime.GOARCH)
			if err != nil {
				t.Fatalf("node layout: %v", err)
			}
			root := filepath.Join(installDir, rootName)
			executable := filepath.Join(root, layout.Executable)
			npmCLI := filepath.Join(root, layout.NPMCLI)
			for _, path := range []string{executable, npmCLI} {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", path, err)
				}
				if err := os.WriteFile(path, []byte("fixture\n"), 0o755); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
			}
			if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
				t.Fatalf("write install manifest: %v", err)
			}
			return NodeInstallation{Root: root, Executable: executable, NPMCLI: npmCLI}
		}

		t.Run("accepts one non-derived payload directory", func(t *testing.T) {
			installDir := t.TempDir()
			want := writeNodePayload(t, installDir, "verified-payload-with-arbitrary-name")
			got, err := ResolveNodeInstallation(installDir)
			if err != nil {
				t.Fatalf("ResolveNodeInstallation: %v", err)
			}
			if got != want {
				t.Fatalf("ResolveNodeInstallation = %+v, want %+v", got, want)
			}
		})

		tests := []struct {
			name  string
			setup func(*testing.T, string)
		}{
			{"zero directories", func(t *testing.T, installDir string) {
				if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
			{"multiple directories", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "one")
				if err := os.MkdirAll(filepath.Join(installDir, "two"), 0o755); err != nil {
					t.Fatal(err)
				}
			}},
			{"unexpected top-level file", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "payload")
				if err := os.WriteFile(filepath.Join(installDir, "unexpected.txt"), []byte("no\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
			{"missing node executable", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				if err := os.Remove(want.Executable); err != nil {
					t.Fatal(err)
				}
			}},
			{"missing npm cli", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				if err := os.Remove(want.NPMCLI); err != nil {
					t.Fatal(err)
				}
			}},
		}
		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				installDir := t.TempDir()
				testCase.setup(t, installDir)
				_, err := ResolveNodeInstallation(installDir)
				if err == nil || !strings.Contains(err.Error(), "GOLC_NODE_TOOLCHAIN_MISSING") {
					t.Fatalf("expected stable Node diagnostic, got %v", err)
				}
			})
		}

		t.Run("rejects top-level symlink", func(t *testing.T) {
			installDir := t.TempDir()
			target := filepath.Join(t.TempDir(), "payload")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(installDir, "payload-link")); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if err := os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := ResolveNodeInstallation(installDir)
			if err == nil || !strings.Contains(err.Error(), "GOLC_NODE_TOOLCHAIN_MISSING") {
				t.Fatalf("expected stable Node diagnostic, got %v", err)
			}
		})
	})

	t.Run("production manifest configures exact platform authorities", func(t *testing.T) {
		root := filepath.Join("..", "..")
		document, _, err := readBootstrapManifest(root)
		if err != nil {
			t.Fatalf("read production manifest: %v", err)
		}
		wantPlatforms := []string{"windows-amd64", "linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
		for _, tool := range []string{"go", "node"} {
			parent, ok := document.Toolchain[tool]
			if !ok {
				t.Fatalf("production manifest missing toolchain.%s", tool)
			}
			if len(parent.Platforms) != len(wantPlatforms) {
				t.Fatalf("toolchain.%s platforms = %v, want %v", tool, parent.Platforms, wantPlatforms)
			}
			for _, platform := range wantPlatforms {
				if _, ok := parent.Platforms[platform]; !ok {
					t.Errorf("toolchain.%s missing %s", tool, platform)
				}
			}
		}
		mage := document.Toolchain["mage"]
		if len(mage.Platforms) != len(wantPlatforms) {
			t.Fatalf("toolchain.mage platforms = %v, want %v", mage.Platforms, wantPlatforms)
		}
		for _, platform := range wantPlatforms {
			if _, ok := mage.Platforms[platform]; !ok {
				t.Errorf("toolchain.mage missing %s", platform)
			}
		}
	})

	t.Run("complete Go bootstrap uses pinned tools environment and process order", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		runner := &engineFakeRunner{moduleGraph: strings.Join([]string{
			"example.invalid/test",
			"github.com/BurntSushi/toml v1.6.0",
			"github.com/invopop/jsonschema v0.14.0",
		}, "\n") + "\n"}
		dependencies := bootstrapDependencies{Source: source, Runner: runner}
		if err := runBootstrap(context.Background(), root, Options{}, dependencies); err != nil {
			t.Fatalf("runBootstrap: %v", err)
		}
		if len(source.calls) != 3 {
			t.Fatalf("source calls = %v, want generic tool plus Mage plus Go", source.calls)
		}
		wantArgs := [][]string{
			{"mod", "download", "all"},
			{"mod", "verify"},
			{"list", "-m", "all"},
			{"test", "-count=1", "./internal/bootstrap/"},
		}
		if len(runner.calls) != 5 {
			t.Fatalf("process calls = %d, want 5: %+v", len(runner.calls), runner.calls)
		}
		for index, args := range wantArgs {
			if got := strings.Join(runner.calls[index].args, "\x00"); got != strings.Join(args, "\x00") {
				t.Fatalf("call %d args = %v, want %v", index, runner.calls[index].args, args)
			}
		}
		build := runner.calls[4]
		if len(build.args) != 5 || strings.Join(build.args[:3], " ") != "build -trimpath -o" || build.args[4] != "./cmd/golc-project" {
			t.Fatalf("unexpected build args: %v", build.args)
		}
		for _, call := range runner.calls {
			if call.dir != root {
				t.Fatalf("working directory = %q, want %q", call.dir, root)
			}
			for _, key := range []string{"GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOBIN", "GOFLAGS"} {
				if call.env[key] == "" {
					t.Fatalf("call missing environment %s: %v", key, call.env)
				}
			}
			if call.env["GOLC_PROJECT_ROOT"] != root {
				t.Fatalf("call project root = %q, want %q", call.env["GOLC_PROJECT_ROOT"], root)
			}
			if !filepath.IsAbs(call.executable) {
				t.Fatalf("executable is not absolute: %q", call.executable)
			}
		}
		moduleRecord, err := os.ReadFile(filepath.Join(root, ".tools", "manifest", "go-modules.txt"))
		if err != nil || string(moduleRecord) != runner.moduleGraph {
			t.Fatalf("module record: err=%v bytes=%q", err, moduleRecord)
		}
		if _, err := os.Stat(PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project")); err != nil {
			t.Fatalf("built project command missing: %v", err)
		}
		mageExecutable, err := ResolveMageExecutable(root)
		if err != nil {
			t.Fatalf("ResolveMageExecutable: %v", err)
		}
		if want := filepath.Join(root, ".tools", "toolchains", "mage", "1.17.2", PlatformKey(), ExecutableName("mage")); mageExecutable != want {
			t.Fatalf("ResolveMageExecutable = %q, want %q", mageExecutable, want)
		}

		source.calls = nil
		runner.calls = nil
		if err := runBootstrap(context.Background(), root, Options{}, dependencies); err != nil {
			t.Fatalf("second runBootstrap: %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("matching manifests consulted source: %v", source.calls)
		}
	})

	t.Run("Mage discovery trusts only the current verified install", func(t *testing.T) {
		runFixture := func(t *testing.T) (string, string) {
			t.Helper()
			root, source, _ := writeEngineRepository(t)
			runner := &engineFakeRunner{moduleGraph: strings.Join([]string{
				"example.invalid/test",
				"github.com/BurntSushi/toml v1.6.0",
				"github.com/invopop/jsonschema v0.14.0",
			}, "\n") + "\n"}
			if err := runBootstrap(context.Background(), root, Options{},
				bootstrapDependencies{Source: source, Runner: runner}); err != nil {
				t.Fatalf("runBootstrap: %v", err)
			}
			executable, err := ResolveMageExecutable(root)
			if err != nil {
				t.Fatalf("ResolveMageExecutable: %v", err)
			}
			return root, executable
		}

		t.Run("missing executable", func(t *testing.T) {
			root, executable := runFixture(t)
			if err := os.Remove(executable); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveMageExecutable(root); err == nil {
				t.Fatal("missing Mage executable unexpectedly resolved")
			}
		})
		t.Run("tampered executable", func(t *testing.T) {
			root, executable := runFixture(t)
			if err := os.WriteFile(executable, []byte("tampered\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveMageExecutable(root); err == nil {
				t.Fatal("tampered Mage executable unexpectedly resolved")
			}
		})
		t.Run("mismatched manifest", func(t *testing.T) {
			root, _ := runFixture(t)
			manifestPath := filepath.Join(root, ".tools", "toolchains", "mage", "1.17.2", PlatformKey(), ManifestName)
			if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveMageExecutable(root); err == nil {
				t.Fatal("manifest-mismatched Mage executable unexpectedly resolved")
			}
		})
		t.Run("symlink executable", func(t *testing.T) {
			root, executable := runFixture(t)
			target := filepath.Join(t.TempDir(), ExecutableName("mage"))
			if err := os.WriteFile(target, []byte("mage executable\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(executable); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, executable); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			if _, err := ResolveMageExecutable(root); err == nil {
				t.Fatal("symlinked Mage executable unexpectedly resolved")
			}
		})
	})

	t.Run("Go lock mutation is diagnosed and original bytes are restored", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		before, _ := os.ReadFile(filepath.Join(root, "go.mod"))
		runner := &engineFakeRunner{
			moduleGraph: "github.com/BurntSushi/toml v1.6.0\ngithub.com/invopop/jsonschema v0.14.0\n",
			mutateLock:  true,
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: runner})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_LOCK_MUTATION") {
			t.Fatalf("expected lock mutation diagnostic, got %v", err)
		}
		after, _ := os.ReadFile(filepath.Join(root, "go.mod"))
		if !bytes.Equal(before, after) {
			t.Fatalf("go.mod changed on return: before=%q after=%q", before, after)
		}
	})

	t.Run("mismatched configured platform fails before source or install work", func(t *testing.T) {
		root, source, goURL := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		wrongURL := strings.Replace(goURL, filepath.Base(goURL), "go1.26.5.not-this-platform.zip", 1)
		raw = bytes.Replace(raw, []byte(goURL), []byte(wrongURL), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		if err == nil || !strings.Contains(err.Error(), "GOLC_BOOTSTRAP_PLATFORM_MISMATCH") {
			t.Fatalf("expected platform mismatch, got %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("platform mismatch consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("platform mismatch created .tools: %v", err)
		}
	})

	t.Run("missing current Go platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.go.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.go.platforms."unconfigured-platform"]`), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.go.platforms.%q]`, PlatformKey())
		if err == nil || !strings.Contains(err.Error(), required) {
			t.Fatalf("expected missing platform diagnostic naming %s, got %v", required, err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("missing platform consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("missing platform created .tools: %v", err)
		}
	})

	t.Run("missing current Mage platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.mage.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.mage.platforms."unconfigured-platform"]`), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{},
			bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.mage.platforms.%q]`, PlatformKey())
		if err == nil || !strings.Contains(err.Error(), required) {
			t.Fatalf("expected missing Mage platform diagnostic naming %s, got %v", required, err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("missing Mage platform consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("missing Mage platform created .tools: %v", err)
		}
	})

	t.Run("obsolete generic archive locator is rejected before effects", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		raw = bytes.Replace(raw, []byte("archive_url"), []byte("archive_uri"), 1)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatalf("rewrite manifest: %v", err)
		}
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		if err == nil || !strings.Contains(err.Error(), "archive_uri") {
			t.Fatalf("expected unsupported archive_uri diagnostic, got %v", err)
		}
		if len(source.calls) != 0 {
			t.Fatalf("obsolete locator consulted source: %v", source.calls)
		}
		if _, err := os.Stat(filepath.Join(root, ".tools")); !os.IsNotExist(err) {
			t.Fatalf("obsolete locator created .tools: %v", err)
		}
	})

	t.Run("runProcess includes captured output in its error, not just the bare exec error", func(t *testing.T) {
		// Regression: run 30074378227's GOLC_BOOTSTRAP_PROBE_FAILED
		// reported zero diagnostic detail beyond "exit status 1" for a
		// failing `go test ./internal/bootstrap/` invocation, because
		// runProcess discarded the captured output on failure instead
		// of including it in the returned error.
		engine := &bootstrapEngine{
			root: t.TempDir(),
			env:  map[string]string{},
			runner: outputCapturingFakeRunner{
				output: []byte("--- FAIL: TestSomething\n    some_test.go:12: assertion failed\n"),
				err:    fmt.Errorf("exit status 1"),
			},
		}
		_, err := engine.runProcess(context.Background(), "go", "GOLC_BOOTSTRAP_PROBE_FAILED", "test", "./...")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "some_test.go:12: assertion failed") {
			t.Fatalf("expected the captured process output in the error, got: %v", err)
		}

		emptyOutputEngine := &bootstrapEngine{
			root:   t.TempDir(),
			env:    map[string]string{},
			runner: outputCapturingFakeRunner{output: nil, err: fmt.Errorf("exit status 1")},
		}
		_, err = emptyOutputEngine.runProcess(context.Background(), "go", "GOLC_BOOTSTRAP_PROBE_FAILED", "test", "./...")
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), "\n") {
			t.Fatalf("expected no trailing detail when there is no captured output, got: %v", err)
		}
	})
}

// outputCapturingFakeRunner always returns the given output/err pair,
// mirroring a failing process that still writes diagnostic output.
type outputCapturingFakeRunner struct {
	output []byte
	err    error
}

func (runner outputCapturingFakeRunner) Run(context.Context, processRequest) ([]byte, error) {
	return runner.output, runner.err
}
