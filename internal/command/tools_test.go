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
	"reflect"
	"strings"
	"testing"
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

[toolchain.go.platforms."fixture-unconfigured"]
metadata = "preserve this unrelated synthetic platform data"

[toolchain.node]
version = "24.18.0"
official_host = "nodejs.org"
official_path_prefix = "/dist/"

[toolchain.node.platforms."windows-amd64"]
archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-win-x64.zip"
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
			Version:       "1.26.6",
			ArchiveURL:    "https://go.dev/dl/go1.26.6.windows-amd64.zip",
			ArchiveSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		NodeToolchain: ToolchainPin{
			Version:       "24.18.1",
			ArchiveURL:    "https://nodejs.org/dist/v24.18.1/node-v24.18.1-win-x64.zip",
			ArchiveSHA256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
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
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatalf("mkdir fixture parent %q: %v", relative, err)
		}
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			t.Fatalf("write fixture %q: %v", relative, err)
		}
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
	if err != nil {
		t.Fatalf("snapshotDir(%q): %v", root, err)
	}
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
		if err != nil {
			t.Fatalf("read tools.go: %v", err)
		}
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
			if strings.Contains(string(source), needle) {
				t.Fatalf("tools.go must never reference %q (T-01-14/D-04: check/write must never install or execute anything)", needle)
			}
		}
	})

	t.Run("tools update --check and tools update --write are reachable through the default registry", func(t *testing.T) {
		registry, err := NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}
		for _, args := range [][]string{
			{"tools", "update", "--check"},
			{"tools", "update", "--write"},
		} {
			registration, rest, ok := registry.Lookup(args)
			if !ok {
				t.Fatalf("expected %v to resolve to a registered route", args)
			}
			if registration.Route != "tools update" {
				t.Fatalf("expected route %q, got %q", "tools update", registration.Route)
			}
			wantRest := args[2:]
			if !reflect.DeepEqual(rest, wantRest) {
				t.Fatalf("expected remaining args %v, got %v", wantRest, rest)
			}
		}
	})

	t.Run("tools update requires exactly one of --check or --write", func(t *testing.T) {
		if _, err := parseToolsUpdateArgs(nil); err == nil {
			t.Fatal("expected an error for no arguments")
		}
		if _, err := parseToolsUpdateArgs([]string{"--check", "--write"}); err == nil {
			t.Fatal("expected an error for both flags together")
		}
		if _, err := parseToolsUpdateArgs([]string{"--bogus"}); err == nil {
			t.Fatal("expected an error for an unsupported argument")
		}
		if mode, err := parseToolsUpdateArgs([]string{"--check"}); err != nil || mode != "check" {
			t.Fatalf("expected mode %q, got %q err %v", "check", mode, err)
		}
		if mode, err := parseToolsUpdateArgs([]string{"--write"}); err != nil || mode != "write" {
			t.Fatalf("expected mode %q, got %q err %v", "write", mode, err)
		}
	})

	t.Run("check is deterministic and never writes to disk", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())

		before := snapshotDir(t, dir)

		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		source := &fakeMetadataSource{proposal: fixtureProposal()}

		result1, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal (first): %v", err)
		}
		result2, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal (second): %v", err)
		}

		if !reflect.DeepEqual(result1.Files, result2.Files) {
			t.Fatal("expected byte-identical proposed files across two check runs against identical fake metadata")
		}
		if !reflect.DeepEqual(result1.Diffs, result2.Diffs) {
			t.Fatal("expected byte-identical diff bytes across two check runs against identical fake metadata")
		}
		if source.calls != 2 {
			t.Fatalf("expected the fake metadata source to be consulted exactly twice, got %d", source.calls)
		}

		after := snapshotDir(t, dir)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("check must never write to disk: fixture directory changed after two proposal builds")
		}
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
			if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
				t.Fatalf("mkdir decoy %q: %v", relative, err)
			}
			if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
				t.Fatalf("write decoy %q: %v", relative, err)
			}
		}

		before := snapshotDir(t, dir)

		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		source := &fakeMetadataSource{proposal: fixtureProposal()}
		result, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal: %v", err)
		}

		if err := writeToolsUpdateFiles(dir, result.Files); err != nil {
			t.Fatalf("writeToolsUpdateFiles: %v", err)
		}

		after := snapshotDir(t, dir)

		if len(after) != len(before) {
			t.Fatalf("write must create no new and delete no existing paths: before had %d files, after has %d", len(before), len(after))
		}

		changed := map[string]bool{}
		for relative, beforeContent := range before {
			afterContent, ok := after[relative]
			if !ok {
				t.Fatalf("path %q disappeared after write", relative)
			}
			if !bytes.Equal(beforeContent, afterContent) {
				changed[relative] = true
			}
		}

		wantChanged := map[string]bool{}
		for _, relative := range toolsUpdateAllowlist {
			wantChanged[relative] = true
		}
		if !reflect.DeepEqual(changed, wantChanged) {
			t.Fatalf("expected exactly the five allowlisted paths to change, got %v", changed)
		}

		for relative, content := range decoys {
			got, ok := after[filepath.ToSlash(relative)]
			if !ok || string(got) != content {
				t.Fatalf("decoy %q was modified by write (cache/node_modules/dist must remain unchanged)", relative)
			}
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
			if !bytes.Equal(got, want) {
				t.Fatalf("path %q on disk does not equal the reviewed proposal bytes", relative)
			}
		}
	})

	t.Run("npm proposal pins exact versions and produces mutually consistent package.json/package-lock.json bytes", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal: %v", err)
		}

		if err := verifyNpmConsistency(result.Files.PackageJSON, result.Files.PackageLock); err != nil {
			t.Fatalf("expected mutually consistent npm proposal, got: %v", err)
		}

		var manifest struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(result.Files.PackageJSON, &manifest); err != nil {
			t.Fatalf("unmarshal proposed package.json: %v", err)
		}
		if manifest.Dependencies["@linear/sdk"] != proposal.LinearSDK.Version {
			t.Fatalf("expected @linear/sdk %q, got %q", proposal.LinearSDK.Version, manifest.Dependencies["@linear/sdk"])
		}
		if manifest.DevDependencies["typescript"] != proposal.TypeScript.Version {
			t.Fatalf("expected typescript %q, got %q", proposal.TypeScript.Version, manifest.DevDependencies["typescript"])
		}

		var lock struct {
			LockfileVersion int `json:"lockfileVersion"`
			Packages        map[string]struct {
				Version   string `json:"version"`
				Integrity string `json:"integrity"`
				Resolved  string `json:"resolved"`
			} `json:"packages"`
		}
		if err := json.Unmarshal(result.Files.PackageLock, &lock); err != nil {
			t.Fatalf("unmarshal proposed package-lock.json: %v", err)
		}
		if lock.LockfileVersion != 3 {
			t.Fatalf("expected lockfileVersion 3 preserved, got %d", lock.LockfileVersion)
		}
		sdkEntry := lock.Packages["node_modules/@linear/sdk"]
		if sdkEntry.Version != proposal.LinearSDK.Version || sdkEntry.Integrity != proposal.LinearSDK.Integrity || sdkEntry.Resolved != proposal.LinearSDK.Resolved {
			t.Fatalf("node_modules/@linear/sdk entry does not match the proposed pin exactly: %+v", sdkEntry)
		}
		tsEntry := lock.Packages["node_modules/typescript"]
		if tsEntry.Version != proposal.TypeScript.Version || tsEntry.Integrity != proposal.TypeScript.Integrity || tsEntry.Resolved != proposal.TypeScript.Resolved {
			t.Fatalf("node_modules/typescript entry does not match the proposed pin exactly: %+v", tsEntry)
		}
	})

	t.Run("Go module proposal keeps go.mod and go.sum mutually consistent", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal: %v", err)
		}

		if !bytes.Contains(result.Files.GoMod, []byte(proposal.GoModule.Path+" "+proposal.GoModule.Version)) {
			t.Fatalf("expected go.mod to contain %s %s, got:\n%s", proposal.GoModule.Path, proposal.GoModule.Version, result.Files.GoMod)
		}
		if bytes.Contains(result.Files.GoMod, []byte(" v1.6.0")) {
			t.Fatal("expected the old go.mod pin to be fully replaced, not left alongside the new one")
		}

		wantSumLine := proposal.GoModule.Path + " " + proposal.GoModule.Version + " h1:" + proposal.GoModule.SumHash
		wantModLine := proposal.GoModule.Path + " " + proposal.GoModule.Version + "/go.mod h1:" + proposal.GoModule.ModHash
		if !bytes.Contains(result.Files.GoSum, []byte(wantSumLine)) {
			t.Fatalf("expected go.sum to contain %q, got:\n%s", wantSumLine, result.Files.GoSum)
		}
		if !bytes.Contains(result.Files.GoSum, []byte(wantModLine)) {
			t.Fatalf("expected go.sum to contain %q, got:\n%s", wantModLine, result.Files.GoSum)
		}
		if bytes.Contains(result.Files.GoSum, []byte("v1.6.0 h1:")) || bytes.Contains(result.Files.GoSum, []byte("v1.6.0/go.mod h1:")) {
			t.Fatal("expected the old go.sum lines to be fully replaced, not left alongside the new ones")
		}
	})

	t.Run("toolchain.toml proposal changes only the six declared pin lines and preserves everything else", func(t *testing.T) {
		dir := t.TempDir()
		fixture := fixtureCurrentFiles()
		writeFixtureFiles(t, dir, fixture)
		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		proposal := fixtureProposal()
		source := &fakeMetadataSource{proposal: proposal}
		result, err := BuildToolsUpdateProposal(source, current)
		if err != nil {
			t.Fatalf("BuildToolsUpdateProposal: %v", err)
		}

		oldLines := strings.Split(string(fixture.ToolchainTOML), "\n")
		newLines := strings.Split(string(result.Files.ToolchainTOML), "\n")
		if len(oldLines) != len(newLines) {
			t.Fatalf("expected the same line count (surgical value replacement only), got %d vs %d", len(oldLines), len(newLines))
		}
		changedLines := 0
		for i := range oldLines {
			if oldLines[i] != newLines[i] {
				changedLines++
			}
		}
		if changedLines != 6 {
			t.Fatalf("expected exactly 6 changed lines (version/archive_url/archive_sha256 x2 tables), got %d", changedLines)
		}
		if !strings.Contains(string(result.Files.ToolchainTOML), "# GOLC toolchain concern") {
			t.Fatal("expected the header comment to survive untouched")
		}
		if !strings.Contains(string(result.Files.ToolchainTOML), `downloads = ".tools/cache/downloads"`) {
			t.Fatal("expected the [cache] section to survive untouched")
		}
		if !strings.Contains(string(result.Files.ToolchainTOML), `metadata = "preserve this unrelated synthetic platform data"`) {
			t.Fatal("expected unrelated platform data to survive untouched")
		}
	})

	t.Run("check builds a proposal in memory only; a simulated bootstrap read still sees only the reviewed on-disk bytes", func(t *testing.T) {
		dir := t.TempDir()
		fixture := fixtureCurrentFiles()
		writeFixtureFiles(t, dir, fixture)

		current, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles: %v", err)
		}
		source := &fakeMetadataSource{proposal: fixtureProposal()}
		if _, err := BuildToolsUpdateProposal(source, current); err != nil {
			t.Fatalf("BuildToolsUpdateProposal: %v", err)
		}

		// A "bootstrap read" is just reading the five files back from disk:
		// it must still see the original, reviewed bytes, never the
		// in-memory proposal computed above.
		bootstrapRead, err := readToolsUpdateCurrentFiles(dir)
		if err != nil {
			t.Fatalf("readToolsUpdateCurrentFiles (simulated bootstrap): %v", err)
		}
		if !reflect.DeepEqual(bootstrapRead, current) {
			t.Fatal("expected a simulated bootstrap read after check to see only the original reviewed bytes")
		}
	})

	t.Run("registry.Execute serves tools update --check/--write end-to-end with the production default source", func(t *testing.T) {
		dir := t.TempDir()
		writeFixtureFiles(t, dir, fixtureCurrentFiles())
		before := snapshotDir(t, dir)

		registry, err := NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}

		checkResult := registry.Execute(Request{Args: []string{"tools", "update", "--check"}, Root: dir})
		if checkResult.ExitCode != 0 {
			t.Fatalf("expected exit 0 from tools update --check, got %d (stderr: %s)", checkResult.ExitCode, checkResult.Stderr)
		}
		after := snapshotDir(t, dir)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("tools update --check must never write to disk")
		}

		writeResult := registry.Execute(Request{Args: []string{"tools", "update", "--write"}, Root: dir})
		if writeResult.ExitCode != 0 {
			t.Fatalf("expected exit 0 from tools update --write, got %d (stderr: %s)", writeResult.ExitCode, writeResult.Stderr)
		}
		afterWrite := snapshotDir(t, dir)
		if len(after) != len(afterWrite) {
			t.Fatal("tools update --write must not create or delete any path outside the allowlist")
		}

		// config/toolchain.toml, go.mod, and go.sum are rewritten through
		// surgical line replacement, so a value-for-value no-op is also a
		// byte-for-byte no-op.
		for _, relative := range toolsUpdateAllowlist[:3] {
			if !bytes.Equal(after[relative], afterWrite[relative]) {
				t.Fatalf("expected the production default source to reaffirm the existing pin for %q as a byte-identical no-op write", relative)
			}
		}

		// package.json and package-lock.json are rewritten through
		// canonical deterministic JSON re-serialization, so a no-op
		// proposal is value-for-value identical but may reorder keys.
		for _, relative := range toolsUpdateAllowlist[3:] {
			var before, afterValue any
			if err := json.Unmarshal(after[relative], &before); err != nil {
				t.Fatalf("unmarshal pre-write %q: %v", relative, err)
			}
			if err := json.Unmarshal(afterWrite[relative], &afterValue); err != nil {
				t.Fatalf("unmarshal post-write %q: %v", relative, err)
			}
			if !reflect.DeepEqual(before, afterValue) {
				t.Fatalf("expected the production default source to reaffirm the existing pin for %q as a value-for-value no-op write", relative)
			}
		}
	})
}
