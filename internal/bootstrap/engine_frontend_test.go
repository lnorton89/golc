package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFrontendFixture writes a minimal but realistic frontend/ directory
// (package.json, package-lock.json, index.html, vite.config.ts,
// tsconfig.json, src/App.tsx) -- exactly the file set FrontendSourceDigest
// is meant to cover.
func writeFrontendFixture(t *testing.T, frontendDir string) {
	t.Helper()
	files := map[string]string{
		"package.json":                  `{"name":"frontend"}` + "\n",
		"package-lock.json":             `{"lockfileVersion":3}` + "\n",
		"index.html":                    "<html></html>\n",
		"vite.config.ts":                "export default {}\n",
		"tsconfig.json":                 "{}\n",
		filepath.Join("src", "App.tsx"): "export const App = () => null;\n",
	}
	for relative, content := range files {
		path := filepath.Join(frontendDir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrontendSourceDigestIsDeterministicAndExcludesNodeModules(t *testing.T) {
	root := t.TempDir()
	frontendDir := filepath.Join(root, "frontend")
	writeFrontendFixture(t, frontendDir)

	first, err := FrontendSourceDigest(frontendDir)
	if err != nil {
		t.Fatalf("FrontendSourceDigest: %v", err)
	}
	second, err := FrontendSourceDigest(frontendDir)
	if err != nil {
		t.Fatalf("FrontendSourceDigest: %v", err)
	}
	if first != second {
		t.Fatalf("digest not deterministic: %q vs %q", first, second)
	}

	nodeModulesFile := filepath.Join(frontendDir, "node_modules", "some-pkg", "index.js")
	if err := os.MkdirAll(filepath.Dir(nodeModulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nodeModulesFile, []byte("module.exports = {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterNodeModules, err := FrontendSourceDigest(frontendDir)
	if err != nil {
		t.Fatalf("FrontendSourceDigest: %v", err)
	}
	if afterNodeModules != first {
		t.Fatalf("node_modules changed the digest: %q vs %q", afterNodeModules, first)
	}

	appPath := filepath.Join(frontendDir, "src", "App.tsx")
	if err := os.WriteFile(appPath, []byte("export const App = () => 'changed';\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterSourceEdit, err := FrontendSourceDigest(frontendDir)
	if err != nil {
		t.Fatalf("FrontendSourceDigest: %v", err)
	}
	if afterSourceEdit == first {
		t.Fatal("source edit did not change the digest")
	}
}

func TestFrontendDistFreshTracksManifestAndSource(t *testing.T) {
	root := t.TempDir()
	frontendDir := filepath.Join(root, "frontend")
	writeFrontendFixture(t, frontendDir)
	distIndexPath := filepath.Join(root, "cmd", "golc-desktop", "frontend", "dist", "index.html")

	fresh, err := FrontendDistFresh(frontendDir, distIndexPath)
	if err != nil {
		t.Fatalf("FrontendDistFresh: %v", err)
	}
	if fresh {
		t.Fatal("expected stale: no manifest and no dist yet")
	}

	if err := os.MkdirAll(filepath.Dir(distIndexPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(distIndexPath, []byte("<html></html>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrontendBuildManifest(frontendDir); err != nil {
		t.Fatalf("WriteFrontendBuildManifest: %v", err)
	}

	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	if err != nil {
		t.Fatalf("FrontendDistFresh: %v", err)
	}
	if !fresh {
		t.Fatal("expected fresh immediately after WriteFrontendBuildManifest")
	}

	appPath := filepath.Join(frontendDir, "src", "App.tsx")
	if err := os.WriteFile(appPath, []byte("export const App = () => 'changed';\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	if err != nil {
		t.Fatalf("FrontendDistFresh: %v", err)
	}
	if fresh {
		t.Fatal("expected stale after a source-only edit (dependencies unchanged)")
	}

	// Revert, then confirm a schema-v1-shaped manifest (no source_sha256) is
	// treated as stale rather than accidentally decoded as "matches" -- this
	// is what forces one rebuild on every pre-existing checkout after the
	// schema bump, instead of silently trusting an old-shape manifest.
	if err := os.WriteFile(appPath, []byte("export const App = () => null;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrontendBuildManifest(frontendDir); err != nil {
		t.Fatalf("WriteFrontendBuildManifest: %v", err)
	}
	staleManifest := `{"schema_version":1,"package_json_sha256":"x","package_lock_sha256":"y"}` + "\n"
	manifestPath := filepath.Join(frontendDir, "node_modules", frontendBuildManifestName)
	if err := os.WriteFile(manifestPath, []byte(staleManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	if err != nil {
		t.Fatalf("FrontendDistFresh: %v", err)
	}
	if fresh {
		t.Fatal("expected stale for a schema-v1-shaped manifest")
	}
}
