// tools_test.go is the exact D-04 fake-source/read-only/write-only/
// no-install proof for "tools update --check|--write" (01-29-PLAN.md).
// The tools-update quick-test scope is declared through the exact
// production entrypoint (01-VALIDATION: every owning Go test task
// registers its scope through MustDeclareScope beside its TestScope
// marker, pattern set by config-local/bootstrap-cache). This file is
// package command (not an external _test package), matching build_test.go
// and router_test.go's own precedent of exercising tools.go's pure
// functions directly rather than only through the full Request/Result
// registry loop.
package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "tools-update",
	Summary: "D-04 deterministic check/write/allowlist/no-install proof for tools update.",
})

// fakeMetadataSource is the only MetadataSource this plan wires an actual
// change proposal through: a static, in-memory, injected pin set (CONTEXT
// D-04's own scope is the reviewable check/write mutation contract, not
// live registry polling).
type fakeMetadataSource struct {
	proposal ToolsUpdateProposal
	calls    int
}

func (s *fakeMetadataSource) Propose() (ToolsUpdateProposal, error) {
	s.calls++
	return s.proposal, nil
}

// fixtureCurrentFiles is a self-authored, entirely synthetic starting
// state for the five declared authorities -- shaped like the real
// repository files (same tables/keys/sections) but never read from or
// written to the real repository, so these tests can never corrupt
// config/toolchain.toml, go.mod, go.sum, or the real
// tools/linear-sync manifest/lock.
func fixtureCurrentFiles() ToolsUpdateCurrentFiles {
	toolchainTOML := `# GOLC toolchain concern: exact immutable bootstrap pins.
#
# tools_test.go fixture: comments and unrelated sections must survive a
# proposal/write untouched.

schema_version = 2

[toolchain.go]
version = "1.26.5"
official_host = "go.dev"
official_path_prefix = "/dl/"

[toolchain.go.platforms."windows-amd64"]
archive_url = "https://go.dev/dl/go1.26.5.windows-amd64.zip"
archive_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[toolchain.go.platforms."linux-amd64"]
archive_url = "https://go.dev/dl/go1.26.5.linux-amd64.tar.gz"
archive_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[toolchain.go.platforms."linux-arm64"]
archive_url = "https://go.dev/dl/go1.26.5.linux-arm64.tar.gz"
archive_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[toolchain.go.platforms."darwin-amd64"]
archive_url = "https://go.dev/dl/go1.26.5.darwin-amd64.tar.gz"
archive_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[toolchain.go.platforms."darwin-arm64"]
archive_url = "https://go.dev/dl/go1.26.5.darwin-arm64.tar.gz"
archive_sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[toolchain.node]
version = "24.18.0"
official_host = "nodejs.org"
official_path_prefix = "/dist/"

[toolchain.node.platforms."windows-amd64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-win-x64.zip"
archive_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

[toolchain.node.platforms."linux-amd64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-x64.tar.gz"
archive_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

[toolchain.node.platforms."linux-arm64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-arm64.tar.gz"
archive_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

[toolchain.node.platforms."darwin-amd64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-x64.tar.gz"
archive_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

[toolchain.node.platforms."darwin-arm64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-arm64.tar.gz"
archive_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

[cache]
downloads = ".tools/cache/downloads"
gomodcache = ".tools/cache/go-mod"
gocache = ".tools/cache/go-build"
`

	goMod := `module example.com/golcfixture

go 1.26.5

require (
	github.com/BurntSushi/toml v1.6.0
)
`

	goSum := `github.com/BurntSushi/toml v1.6.0 h1:dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk=
github.com/BurntSushi/toml v1.6.0/go.mod h1:ukJfTF/6rtPPRCnwkur4qwRxa8vTRFBF0uk2lLoLwho=
`

	packageJSON := `{
  "name": "linear-sync",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "engines": {
    "node": ">=24.18.0"
  },
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "node --test dist/test/"
  },
  "dependencies": {
    "@linear/sdk": "88.1.0"
  },
  "devDependencies": {
    "typescript": "7.0.2"
  }
}
`

	packageLock := `{
  "name": "linear-sync",
  "version": "0.1.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "linear-sync",
      "version": "0.1.0",
      "dependencies": {
        "@linear/sdk": "88.1.0"
      },
      "devDependencies": {
        "typescript": "7.0.2"
      }
    },
    "node_modules/@linear/sdk": {
      "version": "88.1.0",
      "resolved": "https://registry.npmjs.org/@linear/sdk/-/sdk-88.1.0.tgz",
      "integrity": "sha512-fixture-linear-sdk-88.1.0=="
    },
    "node_modules/typescript": {
      "version": "7.0.2",
      "resolved": "https://registry.npmjs.org/typescript/-/typescript-7.0.2.tgz",
      "integrity": "sha512-fixture-typescript-7.0.2=="
    }
  }
}
`

	return ToolsUpdateCurrentFiles{
		ToolchainTOML: []byte(toolchainTOML),
		GoMod:         []byte(goMod),
		GoSum:         []byte(goSum),
		PackageJSON:   []byte(packageJSON),
		PackageLock:   []byte(packageLock),
	}
}

// fixtureProposal is the static change fakeMetadataSource proposes over
// fixtureCurrentFiles: a bumped Go/Node toolchain pin, a bumped
// github.com/BurntSushi/toml module pin, and bumped @linear/sdk/
// typescript npm pins.
func fixtureProposal() ToolsUpdateProposal {
	return ToolsUpdateProposal{
		GoToolchain: ToolchainPin{
			Version: "1.26.6",
			Platforms: map[string]ToolchainArchivePin{
				"windows-amd64": {"https://go.dev/dl/go1.26.6.windows-amd64.zip", strings.Repeat("c", 64)},
				"linux-amd64":   {"https://go.dev/dl/go1.26.6.linux-amd64.tar.gz", strings.Repeat("c", 64)},
				"linux-arm64":   {"https://go.dev/dl/go1.26.6.linux-arm64.tar.gz", strings.Repeat("c", 64)},
				"darwin-amd64":  {"https://go.dev/dl/go1.26.6.darwin-amd64.tar.gz", strings.Repeat("c", 64)},
				"darwin-arm64":  {"https://go.dev/dl/go1.26.6.darwin-arm64.tar.gz", strings.Repeat("c", 64)},
			},
		},
		NodeToolchain: ToolchainPin{
			Version: "24.18.1",
			Platforms: map[string]ToolchainArchivePin{
				"windows-amd64": {"https://nodejs.org/dist/v24.18.1/node-v24.18.1-win-x64.zip", strings.Repeat("d", 64)},
				"linux-amd64":   {"https://nodejs.org/dist/v24.18.1/node-v24.18.1-linux-x64.tar.gz", strings.Repeat("d", 64)},
				"linux-arm64":   {"https://nodejs.org/dist/v24.18.1/node-v24.18.1-linux-arm64.tar.gz", strings.Repeat("d", 64)},
				"darwin-amd64":  {"https://nodejs.org/dist/v24.18.1/node-v24.18.1-darwin-x64.tar.gz", strings.Repeat("d", 64)},
				"darwin-arm64":  {"https://nodejs.org/dist/v24.18.1/node-v24.18.1-darwin-arm64.tar.gz", strings.Repeat("d", 64)},
			},
		},
		GoModule: GoModulePin{
			Path:    "github.com/BurntSushi/toml",
			Version: "v1.6.1",
			SumHash: "FAKESUMHASHFAKESUMHASHFAKESUMHASHFAKESUMHASHFAKESUMHASH1234=",
			ModHash: "FAKEMODHASHFAKEMODHASHFAKEMODHASHFAKEMODHASHFAKEMODHASH5678=",
		},
		LinearSDK: NpmPackagePin{
			Name:      "@linear/sdk",
			Version:   "88.2.0",
			Integrity: "sha512-fake-linear-sdk-88.2.0==",
			Resolved:  "https://registry.npmjs.org/@linear/sdk/-/sdk-88.2.0.tgz",
		},
		TypeScript: NpmPackagePin{
			Name:      "typescript",
			Version:   "7.0.3",
			Integrity: "sha512-fake-typescript-7.0.3==",
			Resolved:  "https://registry.npmjs.org/typescript/-/typescript-7.0.3.tgz",
		},
	}
}

// writeFixtureFiles writes files' bytes to their toolsUpdateAllowlist
// paths under dir, creating parent directories as needed.
func writeFixtureFiles(t *testing.T, dir string, files ToolsUpdateCurrentFiles) {
	t.Helper()
	writes := map[string][]byte{
		toolsUpdateAllowlist[0]: files.ToolchainTOML,
		toolsUpdateAllowlist[1]: files.GoMod,
		toolsUpdateAllowlist[2]: files.GoSum,
		toolsUpdateAllowlist[3]: files.PackageJSON,
		toolsUpdateAllowlist[4]: files.PackageLock,
	}
	for relative, content := range writes {
		absolute := filepath.Join(dir, filepath.FromSlash(relative))
		err := os.MkdirAll(filepath.Dir(absolute), 0o755)
		require.NoError(t, err, "mkdir fixture parent %q: %v", relative, err)
		err = os.WriteFile(absolute, content, 0o644)
		require.NoError(t, err, "write fixture %q: %v", relative, err)
	}
}

// snapshotDir returns every regular file under root, keyed by
// slash-normalized path relative to root, with its exact bytes.
func snapshotDir(t *testing.T, root string) map[string][]byte {
	t.Helper()
	snapshot := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		snapshot[filepath.ToSlash(relative)] = content
		return nil
	})
	require.NoError(t, err, "snapshotDir(%q): %v", root, err)
	return snapshot
}

// TestScopeToolsUpdate is the exact quick-test marker for scope
// "tools-update" (test --quick --scope tools-update). It proves D-04
// across the toolchain, Go, and npm authorities entirely offline against
// synthetic fixtures: repeated deterministic proposals, check's read-only
// behavior, write's exact five-path allowlist, mutually consistent npm
// bytes, and the structural absence of any install/extraction/build call
// path in tools.go.
func TestScopeToolsUpdate(t *testing.T) {
	t.Run("tools.go never imports process-execution or archive-install machinery", func(t *testing.T) {
		source, err := os.ReadFile("tools.go")
		require.NoError(t, err, "read tools.go: %v", err)
		forbidden := []string{
			"os/exec",
			"exec.Command",
			"archive/zip",
			"npm install",
			"npm ci",
			"InstallStaged",
			"VerifyArchive",
			"internal/bootstrap",
		}
		for _, needle := range forbidden {
			require.NotContains(t, string(source), needle, "tools.go must never reference %q (T-01-14/D-04: check/write must never install or execute anything)", needle)
		}
	})

	t.Run("tools update --check and tools update --write are reachable through the default registry", func(t *testing.T) {
		registry, err := NewDefaultCommandRegistry()
		require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
		for _, args := range [][]string{
			{"tools", "update", "--check"},
			{"tools", "update", "--write"},
		} {
			registration, rest, ok := registry.Lookup(args)
			require.True(t, ok, "expected %v to resolve to a registered route", args)
			require.Equal(t, "tools update", registration.Route, "expected route %q, got %q", "tools update", registration.Route)
			wantRest := args[2:]
			require.Equal(t, wantRest, rest, "expected remaining args %v, got %v", wantRest, rest)
		}
	})

	t.Run("tools update requires exactly one of --check or --write", func(t *testing.T) {
		_, err := parseToolsUpdateArgs(nil)
		require.Error(t, err, "expected an error for no arguments")

		_, err = parseToolsUpdateArgs([]string{"--check", "--write"})
		require.Error(t, err, "expected an error for both flags together")

		_, err = parseToolsUpdateArgs([]string{"--bogus"})
		require.Error(t, err, "expected an error for an unsupported argument")

		mode, err := parseToolsUpdateArgs([]string{"--check"})
		require.NoError(t, err, "expected mode %q, got %q err %v", "check", mode, err)
		require.Equal(t, "check", mode, "expected mode %q, got %q err %v", "check", mode, err)

		mode, err = parseToolsUpdateArgs([]string{"--write"})
		require.NoError(t, err, "expected mode %q, got %q err %v", "write", mode, err)
		require.Equal(t, "write", mode, "expected mode %q, got %q err %v", "write", mode, err)
	})

	t.Run("check is deterministic and never writes to disk", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())

		before := snapshotDir(t, dir)

		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		source := &fakeMetadataSource{proposal: fixtureProposal()}

		result1, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal (first): %v", err)
		result2, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal (second): %v", err)

		require.Equal(t, result1.Files, result2.Files, "expected byte-identical proposed files across two check runs against identical fake metadata")
		require.Equal(t, result1.Diffs, result2.Diffs, "expected byte-identical diff bytes across two check runs against identical fake metadata")
		require.Equal(t, 2, source.calls, "expected the fake metadata source to be consulted exactly twice, got %d", source.calls)

		after := snapshotDir(t, dir)
		require.Equal(t, before, after, "check must never write to disk: fixture directory changed after two proposal builds")
	})

	t.Run("write changes exactly the five allowlisted paths and matches the reviewed proposal byte-for-byte", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())

		// Decoy files outside the allowlist -- caches, node_modules, dist,
		// and an unrelated root file -- must survive write byte-for-byte
		// unchanged.
		decoys := map[string]string{
			".tools/cache/downloads/canary.txt":        "cache-canary",
			"tools/linear-sync/node_modules/canary.js": "node-modules-canary",
			"tools/linear-sync/dist/canary.js":         "dist-canary",
			"README.md":                                "unrelated-root-file",
		}
		for relative, content := range decoys {
			absolute := filepath.Join(dir, filepath.FromSlash(relative))
			err := os.MkdirAll(filepath.Dir(absolute), 0o755)
			require.NoError(t, err, "mkdir decoy %q: %v", relative, err)
			err = os.WriteFile(absolute, []byte(content), 0o644)
			require.NoError(t, err, "write decoy %q: %v", relative, err)
		}

		before := snapshotDir(t, dir)

		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		source := &fakeMetadataSource{proposal: fixtureProposal()}
		result, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal: %v", err)

		err = writeToolsUpdateFiles(dir, result.Files)
		require.NoError(t, err, "writeToolsUpdateFiles: %v", err)

		after := snapshotDir(t, dir)

		require.Equal(t, len(before), len(after), "write must create no new and delete no existing paths: before had %d files, after has %d", len(before), len(after))

		changed := map[string]bool{}
		for relative, beforeContent := range before {
			afterContent, ok := after[relative]
			require.True(t, ok, "path %q disappeared after write", relative)
			if !bytes.Equal(beforeContent, afterContent) {
				changed[relative] = true
			}
		}

		wantChanged := map[string]bool{}
		for _, relative := range toolsUpdateAllowlist {
			wantChanged[relative] = true
		}
		require.Equal(t, wantChanged, changed, "expected exactly the five allowlisted paths to change, got %v", changed)

		for relative, content := range decoys {
			got, ok := after[filepath.ToSlash(relative)]
			require.True(t, ok, "decoy %q was modified by write (cache/node_modules/dist must remain unchanged)", relative)
			require.Equal(t, content, string(got), "decoy %q was modified by write (cache/node_modules/dist must remain unchanged)", relative)
		}

		wantFiles := map[string][]byte{
			toolsUpdateAllowlist[0]: result.Files.ToolchainTOML,
			toolsUpdateAllowlist[1]: result.Files.GoMod,
			toolsUpdateAllowlist[2]: result.Files.GoSum,
			toolsUpdateAllowlist[3]: result.Files.PackageJSON,
			toolsUpdateAllowlist[4]: result.Files.PackageLock,
		}
		for relative, want := range wantFiles {
			got := after[relative]
			require.Equal(t, want, got, "path %q on disk does not equal the reviewed proposal bytes", relative)
		}
	})

	t.Run("npm proposal pins exact versions and produces mutually consistent package.json/package-lock.json bytes", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal: %v", err)

		err = verifyNpmConsistency(result.Files.PackageJSON, result.Files.PackageLock)
		require.NoError(t, err, "expected mutually consistent npm proposal, got: %v", err)

		var manifest struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		err = json.Unmarshal(result.Files.PackageJSON, &manifest)
		require.NoError(t, err, "unmarshal proposed package.json: %v", err)
		require.Equal(t, proposal.LinearSDK.Version, manifest.Dependencies["@linear/sdk"], "expected @linear/sdk %q, got %q", proposal.LinearSDK.Version, manifest.Dependencies["@linear/sdk"])
		require.Equal(t, proposal.TypeScript.Version, manifest.DevDependencies["typescript"], "expected typescript %q, got %q", proposal.TypeScript.Version, manifest.DevDependencies["typescript"])

		var lock struct {
			LockfileVersion int `json:"lockfileVersion"`
			Packages        map[string]struct {
				Version   string `json:"version"`
				Integrity string `json:"integrity"`
				Resolved  string `json:"resolved"`
			} `json:"packages"`
		}
		err = json.Unmarshal(result.Files.PackageLock, &lock)
		require.NoError(t, err, "unmarshal proposed package-lock.json: %v", err)
		require.Equal(t, 3, lock.LockfileVersion, "expected lockfileVersion 3 preserved, got %d", lock.LockfileVersion)
		sdkEntry := lock.Packages["node_modules/@linear/sdk"]
		require.Equal(t, proposal.LinearSDK.Version, sdkEntry.Version, "node_modules/@linear/sdk entry does not match the proposed pin exactly: %+v", sdkEntry)
		require.Equal(t, proposal.LinearSDK.Integrity, sdkEntry.Integrity, "node_modules/@linear/sdk entry does not match the proposed pin exactly: %+v", sdkEntry)
		require.Equal(t, proposal.LinearSDK.Resolved, sdkEntry.Resolved, "node_modules/@linear/sdk entry does not match the proposed pin exactly: %+v", sdkEntry)
		tsEntry := lock.Packages["node_modules/typescript"]
		require.Equal(t, proposal.TypeScript.Version, tsEntry.Version, "node_modules/typescript entry does not match the proposed pin exactly: %+v", tsEntry)
		require.Equal(t, proposal.TypeScript.Integrity, tsEntry.Integrity, "node_modules/typescript entry does not match the proposed pin exactly: %+v", tsEntry)
		require.Equal(t, proposal.TypeScript.Resolved, tsEntry.Resolved, "node_modules/typescript entry does not match the proposed pin exactly: %+v", tsEntry)
	})

	t.Run("Go module proposal keeps go.mod and go.sum mutually consistent", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal: %v", err)

		require.Contains(t, string(result.Files.GoMod), proposal.GoModule.Path+" "+proposal.GoModule.Version, "expected go.mod to contain %s %s, got:\n%s", proposal.GoModule.Path, proposal.GoModule.Version, result.Files.GoMod)
		require.NotContains(t, string(result.Files.GoMod), " v1.6.0", "expected the old go.mod pin to be fully replaced, not left alongside the new one")

		wantSumLine := proposal.GoModule.Path + " " + proposal.GoModule.Version + " h1:" + proposal.GoModule.SumHash
		wantModLine := proposal.GoModule.Path + " " + proposal.GoModule.Version + "/go.mod h1:" + proposal.GoModule.ModHash
		require.Contains(t, string(result.Files.GoSum), wantSumLine, "expected go.sum to contain %q, got:\n%s", wantSumLine, result.Files.GoSum)
		require.Contains(t, string(result.Files.GoSum), wantModLine, "expected go.sum to contain %q, got:\n%s", wantModLine, result.Files.GoSum)
		require.NotContains(t, string(result.Files.GoSum), "v1.6.0 h1:", "expected the old go.sum lines to be fully replaced, not left alongside the new ones")
		require.NotContains(t, string(result.Files.GoSum), "v1.6.0/go.mod h1:", "expected the old go.sum lines to be fully replaced, not left alongside the new ones")
	})

	t.Run("toolchain.toml proposal changes all twenty-two declared pin lines and preserves everything else", func(t *testing.T) {
		dir := t.TempDir()
		fixture := fixtureCurrentFiles()
		writeFixtureFiles(t, dir, fixture)
		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal: %v", err)

		oldLines := strings.Split(string(fixture.ToolchainTOML), "\n")
		newLines := strings.Split(string(result.Files.ToolchainTOML), "\n")
		require.Equal(t, len(oldLines), len(newLines), "expected the same line count (surgical value replacement only), got %d vs %d", len(oldLines), len(newLines))
		changedLines := 0
		for i := range oldLines {
			if oldLines[i] != newLines[i] {
				changedLines++
			}
		}
		require.Equal(t, 22, changedLines, "expected exactly 22 changed lines (two versions plus twenty platform fields), got %d", changedLines)
		require.Contains(t, string(result.Files.ToolchainTOML), "# GOLC toolchain concern", "expected the header comment to survive untouched")
		require.Contains(t, string(result.Files.ToolchainTOML), `downloads = ".tools/cache/downloads"`, "expected the [cache] section to survive untouched")
		require.Contains(t, string(result.Files.ToolchainTOML), `downloads = ".tools/cache/downloads"`, "expected unrelated cache data to survive untouched")
	})

	t.Run("toolchain proposal rejects incomplete or extra platform maps before rewrite", func(t *testing.T) {
		current := fixtureCurrentFiles()
		for name, mutate := range map[string]func(*ToolsUpdateProposal){
			"missing": func(proposal *ToolsUpdateProposal) { delete(proposal.GoToolchain.Platforms, "linux-arm64") },
			"extra": func(proposal *ToolsUpdateProposal) {
				proposal.NodeToolchain.Platforms["windows-arm64"] = ToolchainArchivePin{"https://nodejs.org/dist/v24.18.1/node-v24.18.1-win-arm64.zip", strings.Repeat("d", 64)}
			},
		} {
			t.Run(name, func(t *testing.T) {
				proposal := fixtureProposal()
				mutate(&proposal)
				_, err := BuildToolsUpdateProposal(&fakeMetadataSource{proposal: proposal}, current)
				require.Error(t, err, "invalid platform map unexpectedly produced a proposal")
			})
		}
	})

	t.Run("check builds a proposal in memory only; a simulated bootstrap read still sees only the reviewed on-disk bytes", func(t *testing.T) {
		dir := t.TempDir()
		fixture := fixtureCurrentFiles()
		writeFixtureFiles(t, dir, fixture)

		current, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles: %v", err)
		source := &fakeMetadataSource{proposal: fixtureProposal()}
		_, err = BuildToolsUpdateProposal(source, current)
		require.NoError(t, err, "BuildToolsUpdateProposal: %v", err)

		// A "bootstrap read" is just reading the five files back from disk:
		// it must still see the original, reviewed bytes, never the
		// in-memory proposal computed above.
		bootstrapRead, err := readToolsUpdateCurrentFiles(dir)
		require.NoError(t, err, "readToolsUpdateCurrentFiles (simulated bootstrap): %v", err)
		require.Equal(t, current, bootstrapRead, "expected a simulated bootstrap read after check to see only the original reviewed bytes")
	})

	t.Run("registry.Execute serves tools update --check/--write end-to-end with the production default source", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		before := snapshotDir(t, dir)

		registry, err := NewDefaultCommandRegistry()
		require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

		checkResult := registry.Execute(Request{Args: []string{"tools", "update", "--check"}, Root: dir})
		require.Equal(t, 0, checkResult.ExitCode, "expected exit 0 from tools update --check, got %d (stderr: %s)", checkResult.ExitCode, checkResult.Stderr)
		after := snapshotDir(t, dir)
		require.Equal(t, before, after, "tools update --check must never write to disk")

		writeResult := registry.Execute(Request{Args: []string{"tools", "update", "--write"}, Root: dir})
		require.Equal(t, 0, writeResult.ExitCode, "expected exit 0 from tools update --write, got %d (stderr: %s)", writeResult.ExitCode, writeResult.Stderr)
		afterWrite := snapshotDir(t, dir)
		require.Equal(t, len(after), len(afterWrite), "tools update --write must not create or delete any path outside the allowlist")

		// config/toolchain.toml, go.mod, and go.sum are rewritten through
		// surgical line replacement, so a value-for-value no-op is also a
		// byte-for-byte no-op.
		for _, relative := range toolsUpdateAllowlist[:3] {
			require.Equal(t, after[relative], afterWrite[relative], "expected the production default source to reaffirm the existing pin for %q as a byte-identical no-op write", relative)
		}

		// package.json and package-lock.json are rewritten through
		// canonical deterministic JSON re-serialization, so a no-op
		// proposal is value-for-value identical but may reorder keys.
		for _, relative := range toolsUpdateAllowlist[3:] {
			var before, afterValue any
			err := json.Unmarshal(after[relative], &before)
			require.NoError(t, err, "unmarshal pre-write %q: %v", relative, err)
			err = json.Unmarshal(afterWrite[relative], &afterValue)
			require.NoError(t, err, "unmarshal post-write %q: %v", relative, err)
			require.Equal(t, before, afterValue, "expected the production default source to reaffirm the existing pin for %q as a value-for-value no-op write", relative)
		}
	})
}
