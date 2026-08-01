package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func TestFrontendSourceDigestIsDeterministicAndExcludesNodeModules(t *testing.T) {
	root := t.TempDir()
	frontendDir := filepath.Join(root, "frontend")
	writeFrontendFixture(t, frontendDir)

	first, err := FrontendSourceDigest(frontendDir)
	require.NoError(t, err, "FrontendSourceDigest")
	second, err := FrontendSourceDigest(frontendDir)
	require.NoError(t, err, "FrontendSourceDigest")
	require.Equal(t, second, first, "digest not deterministic")

	nodeModulesFile := filepath.Join(frontendDir, "node_modules", "some-pkg", "index.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(nodeModulesFile), 0o755))
	require.NoError(t, os.WriteFile(nodeModulesFile, []byte("module.exports = {};\n"), 0o644))
	afterNodeModules, err := FrontendSourceDigest(frontendDir)
	require.NoError(t, err, "FrontendSourceDigest")
	require.Equal(t, first, afterNodeModules, "node_modules changed the digest")

	appPath := filepath.Join(frontendDir, "src", "App.tsx")
	require.NoError(t, os.WriteFile(appPath, []byte("export const App = () => 'changed';\n"), 0o644))
	afterSourceEdit, err := FrontendSourceDigest(frontendDir)
	require.NoError(t, err, "FrontendSourceDigest")
	require.NotEqual(t, first, afterSourceEdit, "source edit did not change the digest")
}

func TestFrontendDistFreshTracksManifestAndSource(t *testing.T) {
	root := t.TempDir()
	frontendDir := filepath.Join(root, "frontend")
	writeFrontendFixture(t, frontendDir)
	distIndexPath := filepath.Join(root, "cmd", "golc-desktop", "frontend", "dist", "index.html")

	fresh, err := FrontendDistFresh(frontendDir, distIndexPath)
	require.NoError(t, err, "FrontendDistFresh")
	require.False(t, fresh, "expected stale: no manifest and no dist yet")

	require.NoError(t, os.MkdirAll(filepath.Dir(distIndexPath), 0o755))
	require.NoError(t, os.WriteFile(distIndexPath, []byte("<html></html>\n"), 0o644))
	require.NoError(t, WriteFrontendBuildManifest(frontendDir), "WriteFrontendBuildManifest")

	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	require.NoError(t, err, "FrontendDistFresh")
	require.True(t, fresh, "expected fresh immediately after WriteFrontendBuildManifest")

	appPath := filepath.Join(frontendDir, "src", "App.tsx")
	require.NoError(t, os.WriteFile(appPath, []byte("export const App = () => 'changed';\n"), 0o644))
	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	require.NoError(t, err, "FrontendDistFresh")
	require.False(t, fresh, "expected stale after a source-only edit (dependencies unchanged)")

	// Revert, then confirm a schema-v1-shaped manifest (no source_sha256) is
	// treated as stale rather than accidentally decoded as "matches" -- this
	// is what forces one rebuild on every pre-existing checkout after the
	// schema bump, instead of silently trusting an old-shape manifest.
	require.NoError(t, os.WriteFile(appPath, []byte("export const App = () => null;\n"), 0o644))
	require.NoError(t, WriteFrontendBuildManifest(frontendDir), "WriteFrontendBuildManifest")
	staleManifest := `{"schema_version":1,"package_json_sha256":"x","package_lock_sha256":"y"}` + "\n"
	manifestPath := filepath.Join(frontendDir, "node_modules", frontendBuildManifestName)
	require.NoError(t, os.WriteFile(manifestPath, []byte(staleManifest), 0o644))
	fresh, err = FrontendDistFresh(frontendDir, distIndexPath)
	require.NoError(t, err, "FrontendDistFresh")
	require.False(t, fresh, "expected stale for a schema-v1-shaped manifest")
}
