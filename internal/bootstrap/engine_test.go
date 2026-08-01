package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	calls         []processCall
	moduleGraph   string
	mutateLock    bool
	failGoInstall bool
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
	if len(request.Args) == 2 && request.Args[0] == "install" {
		if runner.failGoInstall {
			return nil, errors.New("simulated go_install network failure")
		}
		spec := request.Args[1]
		modulePath := spec
		if idx := strings.LastIndex(spec, "@"); idx >= 0 {
			modulePath = spec[:idx]
		}
		base := modulePath
		if idx := strings.LastIndex(modulePath, "/"); idx >= 0 {
			base = modulePath[idx+1:]
		}
		name := base
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if gobin := request.Env["GOBIN"]; gobin != "" {
			if err := os.MkdirAll(gobin, 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(gobin, name), []byte(base+" executable\n"), 0o755); err != nil {
				return nil, err
			}
		}
	}
	// runFrontendBuild's "npm run build" (distinguished from tools/
	// linear-sync's own npm ci/tsc calls, which linearFakeRunner
	// intercepts before ever reaching this runner, by directory: this
	// simulates only frontend/'s build). The real frontend/vite.config.ts
	// sets outDir to ../cmd/golc-desktop/frontend/dist (relative to
	// frontend/), matching cmd/golc-desktop/main.go's own embed
	// directive, so the fake output must land there too, not under
	// frontend/dist itself.
	if len(request.Args) >= 3 && request.Args[1] == "run" && request.Args[2] == "build" && filepath.Base(request.Dir) == "frontend" {
		root := filepath.Dir(request.Dir)
		distIndex := filepath.Join(root, "cmd", "golc-desktop", "frontend", "dist", "index.html")
		if err := os.MkdirAll(filepath.Dir(distIndex), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(distIndex, []byte("<html></html>\n"), 0o644); err != nil {
			return nil, err
		}
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
	require.NoError(t, err, "platformArchiveLayout")
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
		require.Fail(t, fmt.Sprintf("unsupported test archive format %q", layout.Format))
		return "", "", ""
	}
}

func writeEngineRepository(t *testing.T) (root string, source *engineFakeSource, goURL string) {
	t.Helper()
	root = t.TempDir()
	// runBootstrap resolves its root through filepath.EvalSymlinks before
	// ever recording it as a child process's working directory, so this
	// helper's root must be resolved the same way before any test
	// compares against it -- otherwise a CI runner whose temp directory
	// sits behind an OS-level indirection (macOS's /var -> /private/var,
	// or a Windows short/8.3 alias for a long account name like GitHub
	// Actions' "runneradmin") produces two textually different strings
	// for the exact same directory, and every such comparison fails
	// (observed live in cross-platform-mage.yml run 30075276470 on both
	// macos-latest and windows-latest).
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "config"), 0o755), "mkdir config")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd", "golc-project"), 0o755), "mkdir command")
	goArchive, goDigest, _ := platformToolArchive(t, root, "go", "1.26.5")
	mageArchive, mageDigest, _ := platformToolArchive(t, root, "mage", "1.17.2")
	fixtureArchive, fixtureDigest := buildZipEntries(t, root, "fixture.zip", []testArchiveEntry{
		{Name: "bin/fixture", Body: "fixture\n", Mode: 0o755},
	})
	// Node is pinned here unconditionally, not only by the Linear-sync-
	// specific addLinearSyncFixture helper: runFrontendBuild now installs
	// Node and builds frontend/ on every bootstrap (cmd/golc-desktop's
	// //go:embed all:frontend/dist needs a built frontend/dist to compile
	// at all), so every test using this helper needs a working Node pin
	// and archive, whether or not it cares about Linear-sync specifically.
	nodeLayout, err := platformArchiveLayout("node", "24.18.0", runtime.GOOS, runtime.GOARCH)
	require.NoError(t, err, "node layout")
	nodeEntries := []testArchiveEntry{
		{Name: filepath.ToSlash(filepath.Join("verified-node-payload", nodeLayout.Executable)), Body: "node\n", Mode: 0o755},
		{Name: filepath.ToSlash(filepath.Join("verified-node-payload", nodeLayout.NPMCLI)), Body: "npm\n", Mode: 0o644},
	}
	var nodeArchive, nodeDigest string
	if nodeLayout.Format == ".zip" {
		nodeArchive, nodeDigest = buildZipEntries(t, root, nodeLayout.FileName, nodeEntries)
	} else {
		nodeArchive, nodeDigest = buildTarGzEntries(t, root, nodeLayout.FileName, nodeEntries)
	}
	// Deno is pinned unconditionally too (SCRP-03, 08-RESEARCH.md): every
	// bootstrap now provisions it exactly like Go/Mage/Node, not only a
	// Deno-specific test.
	denoArchive, denoDigest, _ := platformToolArchive(t, root, "deno", "2.9.4")
	goURL = "https://go.dev/dl/" + filepath.Base(goArchive)
	mageURL := "https://github.com/magefile/mage/releases/download/v1.17.2/" + filepath.Base(mageArchive)
	fixtureURL := "https://fixtures.example.invalid/tool/" + filepath.Base(fixtureArchive)
	nodeURL := "https://nodejs.org/dist/v24.18.0/" + filepath.Base(nodeArchive)
	denoURL := "https://github.com/denoland/deno/releases/download/v2.9.4/" + filepath.Base(denoArchive)
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

[toolchain.node]
version = "24.18.0"
official_host = "nodejs.org"
official_path_prefix = "/dist/"

[toolchain.node.platforms.%q]
archive_url = %q
archive_sha256 = %q

[toolchain.deno]
version = "2.9.4"
official_host = "github.com"
official_path_prefix = "/denoland/deno/releases/download/"

[toolchain.deno.platforms.%q]
archive_url = %q
archive_sha256 = %q

[go_install.midicat]
version = "v1.0.7"
module = "gitlab.com/gomidi/tools/midicat"
`, fixtureURL, fixtureDigest, PlatformKey(), goURL, goDigest, PlatformKey(), mageURL, mageDigest, PlatformKey(), nodeURL, nodeDigest, PlatformKey(), denoURL, denoDigest)
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(manifest), 0o644), "write manifest")
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/test\n\ngo 1.26.5\n"), 0o644), "write go.mod")
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), []byte("sum\n"), 0o644), "write go.sum")
	frontendDir := filepath.Join(root, "frontend")
	require.NoError(t, os.MkdirAll(frontendDir, 0o755), "mkdir frontend")
	for name, body := range map[string]string{
		"package.json":      `{"name":"fixture-frontend","scripts":{"build":"true"}}` + "\n",
		"package-lock.json": `{"lockfileVersion":3,"packages":{}}` + "\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(frontendDir, name), []byte(body), 0o644), "write frontend/%s", name)
	}
	goBytes, _ := os.ReadFile(goArchive)
	mageBytes, _ := os.ReadFile(mageArchive)
	fixtureBytes, _ := os.ReadFile(fixtureArchive)
	nodeBytes, _ := os.ReadFile(nodeArchive)
	denoBytes, _ := os.ReadFile(denoArchive)
	source = &engineFakeSource{payload: map[string][]byte{
		goURL:      goBytes,
		mageURL:    mageBytes,
		fixtureURL: fixtureBytes,
		nodeURL:    nodeBytes,
		denoURL:    denoBytes,
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
			require.NoError(t, err, "%s/%s-%s", testCase.tool, testCase.goos, testCase.goarch)
			require.True(t, pin.ArchiveURL == testCase.url && pin.ArchiveSHA256 == testCase.sha, "%s/%s-%s selected %+v", testCase.tool, testCase.goos, testCase.goarch, pin)
		}
	})

	t.Run("explicit platform selector rejects absent and mismatched assets", func(t *testing.T) {
		parent := toolchainManifestPin{
			Version: "1.26.5",
			Platforms: map[string]platformArchivePin{
				"linux-arm64": {ArchiveURL: "https://go.dev/dl/go1.26.5.linux-amd64.tar.gz", ArchiveSHA256: strings.Repeat("a", 64)},
			},
		}
		_, err := selectPlatformPinFor("go", parent, "darwin", "arm64")
		require.Error(t, err, "missing explicit platform unexpectedly selected")
		_, err = selectPlatformPinFor("go", parent, "linux", "arm64")
		require.ErrorContains(t, err, "GOLC_BOOTSTRAP_PLATFORM_MISMATCH", "expected platform mismatch")
	})

	t.Run("PlatformKey and pure platform layouts are exact", func(t *testing.T) {
		got, want := PlatformKey(), runtime.GOOS+"-"+runtime.GOARCH
		require.Equal(t, want, got, "PlatformKey()")
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
			{"deno", "2.9.4", "windows", "amd64", "deno-x86_64-pc-windows-msvc.zip", "", "deno.exe"},
			{"deno", "2.9.4", "linux", "amd64", "deno-x86_64-unknown-linux-gnu.zip", "", "deno"},
			{"deno", "2.9.4", "linux", "arm64", "deno-aarch64-unknown-linux-gnu.zip", "", "deno"},
			{"deno", "2.9.4", "darwin", "amd64", "deno-x86_64-apple-darwin.zip", "", "deno"},
			{"deno", "2.9.4", "darwin", "arm64", "deno-aarch64-apple-darwin.zip", "", "deno"},
		}
		for _, testCase := range cases {
			layout, err := platformArchiveLayout(testCase.tool, testCase.version, testCase.goos, testCase.goarch)
			require.NoError(t, err, "%s/%s", testCase.goos, testCase.tool)
			require.True(t, layout.FileName == testCase.file && layout.Root == testCase.root && layout.Executable == testCase.executable, "%s/%s: got %+v", testCase.goos, testCase.tool, layout)
		}
		gotExecutable, wantExecutable := ExecutableName("golc-project"), "golc-project"+map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]
		require.Equal(t, wantExecutable, gotExecutable, "ExecutableName(golc-project)")
		for _, unsafe := range []string{"", ".", "..", "bin/golc-project", `bin\golc-project`} {
			got := ExecutableName(unsafe)
			require.Empty(t, got, "ExecutableName(%q) want rejection", unsafe)
		}
		installRoot := filepath.Join("repo", ".tools", "installs", "golc_project")
		gotPath, wantPath := PlatformExecutablePath(installRoot, "golc-project"), filepath.Join(installRoot, PlatformKey(), "bin", ExecutableName("golc-project"))
		require.Equal(t, wantPath, gotPath, "PlatformExecutablePath()")
		unsafePath := PlatformExecutablePath(installRoot, "../golc-project")
		require.Empty(t, unsafePath, "PlatformExecutablePath accepted unsafe base")
	})

	t.Run("Node installation is discovered by verified filesystem shape", func(t *testing.T) {
		writeNodePayload := func(t *testing.T, installDir, rootName string) NodeInstallation {
			t.Helper()
			layout, err := platformArchiveLayout("node", "24.18.0", runtime.GOOS, runtime.GOARCH)
			require.NoError(t, err, "node layout")
			root := filepath.Join(installDir, rootName)
			executable := filepath.Join(root, layout.Executable)
			npmCLI := filepath.Join(root, layout.NPMCLI)
			for _, path := range []string{executable, npmCLI} {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", path)
				require.NoError(t, os.WriteFile(path, []byte("fixture\n"), 0o755), "write %s", path)
			}
			require.NoError(t, os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644), "write install manifest")
			return NodeInstallation{Root: root, Executable: executable, NPMCLI: npmCLI}
		}

		t.Run("accepts one non-derived payload directory", func(t *testing.T) {
			installDir := t.TempDir()
			want := writeNodePayload(t, installDir, "verified-payload-with-arbitrary-name")
			got, err := ResolveNodeInstallation(installDir)
			require.NoError(t, err, "ResolveNodeInstallation")
			require.Equal(t, want, got, "ResolveNodeInstallation")
		})

		tests := []struct {
			name  string
			setup func(*testing.T, string)
		}{
			{"zero directories", func(t *testing.T, installDir string) {
				require.NoError(t, os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644))
			}},
			{"multiple directories", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "one")
				require.NoError(t, os.MkdirAll(filepath.Join(installDir, "two"), 0o755))
			}},
			{"unexpected top-level file", func(t *testing.T, installDir string) {
				writeNodePayload(t, installDir, "payload")
				require.NoError(t, os.WriteFile(filepath.Join(installDir, "unexpected.txt"), []byte("no\n"), 0o644))
			}},
			{"missing node executable", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				require.NoError(t, os.Remove(want.Executable))
			}},
			{"missing npm cli", func(t *testing.T, installDir string) {
				want := writeNodePayload(t, installDir, "payload")
				require.NoError(t, os.Remove(want.NPMCLI))
			}},
		}
		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				installDir := t.TempDir()
				testCase.setup(t, installDir)
				_, err := ResolveNodeInstallation(installDir)
				require.ErrorContains(t, err, "GOLC_NODE_TOOLCHAIN_MISSING", "expected stable Node diagnostic")
			})
		}

		t.Run("rejects top-level symlink", func(t *testing.T) {
			installDir := t.TempDir()
			target := filepath.Join(t.TempDir(), "payload")
			require.NoError(t, os.MkdirAll(target, 0o755))
			if err := os.Symlink(target, filepath.Join(installDir, "payload-link")); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			require.NoError(t, os.WriteFile(filepath.Join(installDir, ManifestName), []byte("{}\n"), 0o644))
			_, err := ResolveNodeInstallation(installDir)
			require.ErrorContains(t, err, "GOLC_NODE_TOOLCHAIN_MISSING", "expected stable Node diagnostic")
		})
	})

	t.Run("production manifest configures exact platform authorities", func(t *testing.T) {
		root := filepath.Join("..", "..")
		document, _, err := readBootstrapManifest(root)
		require.NoError(t, err, "read production manifest")
		wantPlatforms := []string{"windows-amd64", "linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
		for _, tool := range []string{"go", "node", "deno"} {
			parent, ok := document.Toolchain[tool]
			require.True(t, ok, "production manifest missing toolchain.%s", tool)
			require.Len(t, parent.Platforms, len(wantPlatforms), "toolchain.%s platforms = %v, want %v", tool, parent.Platforms, wantPlatforms)
			for _, platform := range wantPlatforms {
				_, ok := parent.Platforms[platform]
				assert.True(t, ok, "toolchain.%s missing %s", tool, platform)
			}
		}
		mage := document.Toolchain["mage"]
		require.Len(t, mage.Platforms, len(wantPlatforms), "toolchain.mage platforms = %v, want %v", mage.Platforms, wantPlatforms)
		for _, platform := range wantPlatforms {
			_, ok := mage.Platforms[platform]
			assert.True(t, ok, "toolchain.mage missing %s", platform)
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
		require.NoError(t, runBootstrap(context.Background(), root, Options{}, dependencies), "runBootstrap")
		require.Len(t, source.calls, 5, "source calls = %v, want generic tool plus Mage plus Go plus Deno plus Node", source.calls)
		wantArgs := [][]string{
			{"mod", "download", "all"},
			{"mod", "verify"},
			{"list", "-m", "all"},
			{"test", "-count=1", "./internal/bootstrap/"},
		}
		// 4 Go module/probe calls + build golc-project + go_install's
		// `go install` (midicat) + runFrontendBuild's npm ci and npm run
		// build (unconditional now: cmd/golc-desktop's //go:embed
		// all:frontend/dist needs frontend/dist to exist on every
		// bootstrap, not only Linear-sync-enabled ones).
		require.Len(t, runner.calls, 8, "process calls, want 8: %+v", runner.calls)
		for index, args := range wantArgs {
			got := strings.Join(runner.calls[index].args, "\x00")
			require.Equal(t, strings.Join(args, "\x00"), got, "call %d args = %v, want %v", index, runner.calls[index].args, args)
		}
		build := runner.calls[4]
		require.True(t, len(build.args) == 5 && strings.Join(build.args[:3], " ") == "build -trimpath -o" && build.args[4] == "./cmd/golc-project", "unexpected build args: %v", build.args)
		goInstall := runner.calls[5]
		require.True(t, len(goInstall.args) == 2 && goInstall.args[0] == "install" && goInstall.args[1] == "gitlab.com/gomidi/tools/midicat@v1.0.7" && goInstall.dir == root, "unexpected go_install call: %+v", goInstall)
		midicatExecutable := filepath.Join(root, ".tools", "cache", "go-bin", ExecutableName("midicat"))
		info, err := os.Stat(midicatExecutable)
		require.NoError(t, err, "expected midicat to be installed at %s", midicatExecutable)
		require.True(t, info.Mode().IsRegular(), "expected midicat to be a regular file at %s", midicatExecutable)
		frontendDir := filepath.Join(root, "frontend")
		npmCI := runner.calls[6]
		require.True(t, len(npmCI.args) == 4 && strings.Join(npmCI.args[1:], " ") == "ci --no-audit --no-fund" && npmCI.dir == frontendDir, "unexpected frontend npm ci call: %+v", npmCI)
		npmBuild := runner.calls[7]
		require.True(t, len(npmBuild.args) == 3 && strings.Join(npmBuild.args[1:], " ") == "run build" && npmBuild.dir == frontendDir, "unexpected frontend npm run build call: %+v", npmBuild)
		for index, call := range runner.calls {
			wantDir := root
			if index >= 6 {
				wantDir = frontendDir
			}
			require.Equal(t, wantDir, call.dir, "call %d working directory", index)
			for _, key := range []string{"GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOBIN", "GOFLAGS"} {
				require.NotEmpty(t, call.env[key], "call missing environment %s: %v", key, call.env)
			}
			require.Equal(t, root, call.env["GOLC_PROJECT_ROOT"], "call project root")
			require.True(t, filepath.IsAbs(call.executable), "executable is not absolute: %q", call.executable)
		}
		_, err = os.Stat(filepath.Join(root, "cmd", "golc-desktop", "frontend", "dist", "index.html"))
		require.NoError(t, err, "expected cmd/golc-desktop/frontend/dist/index.html to be produced")
		moduleRecord, err := os.ReadFile(filepath.Join(root, ".tools", "manifest", "go-modules.txt"))
		require.NoError(t, err, "module record")
		require.Equal(t, runner.moduleGraph, string(moduleRecord), "module record")
		_, err = os.Stat(PlatformExecutablePath(filepath.Join(root, ".tools", "installs", "golc_project"), "golc-project"))
		require.NoError(t, err, "built project command missing")
		mageExecutable, err := ResolveMageExecutable(root)
		require.NoError(t, err, "ResolveMageExecutable")
		require.Equal(t, filepath.Join(root, ".tools", "toolchains", "mage", "1.17.2", PlatformKey(), ExecutableName("mage")), mageExecutable, "ResolveMageExecutable")
		denoExecutable, err := ResolveDenoExecutable(root)
		require.NoError(t, err, "ResolveDenoExecutable")
		require.Equal(t, filepath.Join(root, ".tools", "toolchains", "deno", "2.9.4", PlatformKey(), ExecutableName("deno")), denoExecutable, "ResolveDenoExecutable")
		info, err = os.Stat(denoExecutable)
		require.NoError(t, err, "expected regular deno executable at %s", denoExecutable)
		require.True(t, info.Mode().IsRegular(), "expected regular deno executable at %s", denoExecutable)

		source.calls = nil
		runner.calls = nil
		require.NoError(t, runBootstrap(context.Background(), root, Options{}, dependencies), "second runBootstrap")
		require.Empty(t, source.calls, "matching manifests consulted source")
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
			require.NoError(t, runBootstrap(context.Background(), root, Options{},
				bootstrapDependencies{Source: source, Runner: runner}), "runBootstrap")
			executable, err := ResolveMageExecutable(root)
			require.NoError(t, err, "ResolveMageExecutable")
			return root, executable
		}

		t.Run("missing executable", func(t *testing.T) {
			root, executable := runFixture(t)
			require.NoError(t, os.Remove(executable))
			_, err := ResolveMageExecutable(root)
			require.Error(t, err, "missing Mage executable unexpectedly resolved")
		})
		t.Run("tampered executable", func(t *testing.T) {
			root, executable := runFixture(t)
			require.NoError(t, os.WriteFile(executable, []byte("tampered\n"), 0o755))
			_, err := ResolveMageExecutable(root)
			require.Error(t, err, "tampered Mage executable unexpectedly resolved")
		})
		t.Run("mismatched manifest", func(t *testing.T) {
			root, _ := runFixture(t)
			manifestPath := filepath.Join(root, ".tools", "toolchains", "mage", "1.17.2", PlatformKey(), ManifestName)
			require.NoError(t, os.WriteFile(manifestPath, []byte("{}\n"), 0o644))
			_, err := ResolveMageExecutable(root)
			require.Error(t, err, "manifest-mismatched Mage executable unexpectedly resolved")
		})
		t.Run("symlink executable", func(t *testing.T) {
			root, executable := runFixture(t)
			target := filepath.Join(t.TempDir(), ExecutableName("mage"))
			require.NoError(t, os.WriteFile(target, []byte("mage executable\n"), 0o755))
			require.NoError(t, os.Remove(executable))
			if err := os.Symlink(target, executable); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			_, err := ResolveMageExecutable(root)
			require.Error(t, err, "symlinked Mage executable unexpectedly resolved")
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
		require.ErrorContains(t, err, "GOLC_BOOTSTRAP_LOCK_MUTATION", "expected lock mutation diagnostic")
		after, _ := os.ReadFile(filepath.Join(root, "go.mod"))
		require.Equal(t, before, after, "go.mod changed on return")
	})

	t.Run("mismatched configured platform fails before source or install work", func(t *testing.T) {
		root, source, goURL := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		wrongURL := strings.Replace(goURL, filepath.Base(goURL), "go1.26.5.not-this-platform.zip", 1)
		raw = bytes.Replace(raw, []byte(goURL), []byte(wrongURL), 1)
		require.NoError(t, os.WriteFile(manifestPath, raw, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		require.ErrorContains(t, err, "GOLC_BOOTSTRAP_PLATFORM_MISMATCH", "expected platform mismatch")
		require.Empty(t, source.calls, "platform mismatch consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "platform mismatch created .tools: %v", statErr)
	})

	t.Run("missing current Go platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.go.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.go.platforms."unconfigured-platform"]`), 1)
		require.NoError(t, os.WriteFile(manifestPath, raw, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.go.platforms.%q]`, PlatformKey())
		require.ErrorContains(t, err, required, "expected missing platform diagnostic")
		require.Empty(t, source.calls, "missing platform consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "missing platform created .tools: %v", statErr)
	})

	t.Run("missing current Mage platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.mage.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.mage.platforms."unconfigured-platform"]`), 1)
		require.NoError(t, os.WriteFile(manifestPath, raw, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{},
			bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.mage.platforms.%q]`, PlatformKey())
		require.ErrorContains(t, err, required, "expected missing Mage platform diagnostic")
		require.Empty(t, source.calls, "missing Mage platform consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "missing Mage platform created .tools: %v", statErr)
	})

	t.Run("missing [toolchain.deno] entirely fails naming deno before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		// Remove the whole [toolchain.deno] block, including its nested
		// platforms table, so no "[toolchain.deno.*]" header remains to
		// implicitly recreate the parent table (a partial header-only
		// rename would leave TOML's implicit-table-creation rule
		// re-establishing [toolchain.deno] with a different failure mode).
		start := bytes.Index(raw, []byte("[toolchain.deno]"))
		end := bytes.Index(raw, []byte("[go_install."))
		require.True(t, start >= 0 && end >= 0 && end > start, "test setup did not locate the entire [toolchain.deno] block")
		stripped := append(append([]byte(nil), raw[:start]...), raw[end:]...)
		require.NoError(t, os.WriteFile(manifestPath, stripped, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{},
			bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		require.ErrorContains(t, err, "GOLC_DENO_TOOLCHAIN_MISSING", "expected GOLC_DENO_TOOLCHAIN_MISSING naming deno")
		require.Empty(t, source.calls, "missing deno parent consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "missing deno parent created .tools: %v", statErr)
	})

	t.Run("missing current Deno platform fails before source or install work", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		current := fmt.Sprintf("[toolchain.deno.platforms.%q]", PlatformKey())
		raw = bytes.Replace(raw, []byte(current), []byte(`[toolchain.deno.platforms."unconfigured-platform"]`), 1)
		require.NoError(t, os.WriteFile(manifestPath, raw, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{},
			bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		required := fmt.Sprintf(`[toolchain.deno.platforms.%q]`, PlatformKey())
		require.ErrorContains(t, err, required, "expected missing Deno platform diagnostic")
		require.Empty(t, source.calls, "missing Deno platform consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "missing Deno platform created .tools: %v", statErr)
	})

	t.Run("bootstrap rejects a Deno archive whose bytes do not match the pinned SHA-256", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		// Corrupt only the pinned checksum, leaving the archive URL (and
		// therefore the fake source's fetchable payload) untouched, so the
		// mismatch is detected only once the real bytes are hashed and
		// compared -- not from a missing/malformed URL.
		denoShaMarker := regexp.MustCompile(`(\[toolchain\.deno\.platforms\."` + regexp.QuoteMeta(PlatformKey()) + `"\]\narchive_url = "[^"]+"\narchive_sha256 = ")[0-9a-f]{64}(")`)
		corrupted := denoShaMarker.ReplaceAll(raw, []byte("${1}"+strings.Repeat("a", 64)+"${2}"))
		require.NotEqual(t, raw, corrupted, "test setup did not corrupt the deno archive_sha256")
		require.NoError(t, os.WriteFile(manifestPath, corrupted, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{},
			bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		require.ErrorContains(t, err, "BOOTSTRAP_CHECKSUM_MISMATCH", "expected checksum mismatch for tampered deno pin")
		_, err = ResolveDenoExecutable(root)
		require.Error(t, err, "checksum-mismatched Deno install unexpectedly resolved")
	})

	t.Run("obsolete generic archive locator is rejected before effects", func(t *testing.T) {
		root, source, _ := writeEngineRepository(t)
		manifestPath := filepath.Join(root, "config", "toolchain.toml")
		raw, _ := os.ReadFile(manifestPath)
		raw = bytes.Replace(raw, []byte("archive_url"), []byte("archive_uri"), 1)
		require.NoError(t, os.WriteFile(manifestPath, raw, 0o644), "rewrite manifest")
		err := runBootstrap(context.Background(), root, Options{}, bootstrapDependencies{Source: source, Runner: &engineFakeRunner{}})
		require.ErrorContains(t, err, "archive_uri", "expected unsupported archive_uri diagnostic")
		require.Empty(t, source.calls, "obsolete locator consulted source")
		_, statErr := os.Stat(filepath.Join(root, ".tools"))
		require.True(t, os.IsNotExist(statErr), "obsolete locator created .tools: %v", statErr)
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
				err:    errors.New("exit status 1"),
			},
		}
		_, err := engine.runProcess(context.Background(), "go", "GOLC_BOOTSTRAP_PROBE_FAILED", "test", "./...")
		require.Error(t, err, "expected an error")
		require.Contains(t, err.Error(), "some_test.go:12: assertion failed", "expected the captured process output in the error")

		emptyOutputEngine := &bootstrapEngine{
			root:   t.TempDir(),
			env:    map[string]string{},
			runner: outputCapturingFakeRunner{output: nil, err: errors.New("exit status 1")},
		}
		_, err = emptyOutputEngine.runProcess(context.Background(), "go", "GOLC_BOOTSTRAP_PROBE_FAILED", "test", "./...")
		require.Error(t, err, "expected an error")
		require.NotContains(t, err.Error(), "\n", "expected no trailing detail when there is no captured output")
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
