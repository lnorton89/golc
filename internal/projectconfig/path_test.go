// path_test.go covers indexed path containment (CONTEXT D-05/D-09):
// ValidateConcernPath's lexical rejections and ResolveContainedPath's
// final on-disk containment, including symlinked ancestors that escape
// the repository even when the declared leaf does not exist yet.
//
// testPathContainment is a plain helper (not a top-level Go test) invoked
// from TestScopeConfig in resolve_test.go, the file that owns the "config"
// quick-test scope declaration (01-VALIDATION: one scope, one marker,
// contributed to by every file in the owning package).
package projectconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/projectconfig"
)

func testPathContainment(t *testing.T) {
	t.Run("ValidateConcernPath rejects every lexical escape shape", func(t *testing.T) {
		for _, relative := range []string{
			"",
			".",
			"..",
			"../escape",
			"config/../../escape",
			"/absolute",
			`C:\absolute`,
			`config\..\escape`,
			"config/./x",
		} {
			err := projectconfig.ValidateConcernPath(relative)
			require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE", "expected %q to be rejected", relative)
		}
	})

	t.Run("ValidateConcernPath accepts safe repository-relative shapes", func(t *testing.T) {
		for _, relative := range []string{
			"config/toolchain.toml",
			".tools/cache/downloads",
			"a/b/c",
		} {
			require.NoError(t, projectconfig.ValidateConcernPath(relative), "expected %q to be accepted", relative)
		}
	})

	t.Run("ResolveContainedPath rejects lexical escapes before touching disk", func(t *testing.T) {
		root := t.TempDir()
		_, err := projectconfig.ResolveContainedPath(root, "../escape")
		require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE")
	})

	t.Run("ResolveContainedPath accepts an existing contained path", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "config", "integrations"), 0o755), "mkdir")
		resolved, err := projectconfig.ResolveContainedPath(root, "config/integrations")
		require.NoError(t, err, "ResolveContainedPath failed")
		resolvedRoot, err := filepath.EvalSymlinks(root)
		require.NoError(t, err, "EvalSymlinks(root) failed")
		want := filepath.Join(resolvedRoot, "config", "integrations")
		require.Equal(t, want, resolved)
	})

	t.Run("ResolveContainedPath accepts a not-yet-created leaf under an existing contained ancestor", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".tools", "cache"), 0o755), "mkdir")
		resolved, err := projectconfig.ResolveContainedPath(root, ".tools/cache/downloads")
		require.NoError(t, err, "ResolveContainedPath failed on a lazily created cache path")
		resolvedRoot, err := filepath.EvalSymlinks(root)
		require.NoError(t, err, "EvalSymlinks(root) failed")
		want := filepath.Join(resolvedRoot, ".tools", "cache", "downloads")
		require.Equal(t, want, resolved)
		_, statErr := os.Stat(resolved)
		require.True(t, os.IsNotExist(statErr), "ResolveContainedPath must not create the leaf itself")
	})

	t.Run("ResolveContainedPath accepts a fully not-yet-created relative path", func(t *testing.T) {
		root := t.TempDir()
		resolved, err := projectconfig.ResolveContainedPath(root, ".tools/cache/downloads")
		require.NoError(t, err, "ResolveContainedPath failed when no ancestor exists yet")
		resolvedRoot, err := filepath.EvalSymlinks(root)
		require.NoError(t, err, "EvalSymlinks(root) failed")
		want := filepath.Join(resolvedRoot, ".tools", "cache", "downloads")
		require.Equal(t, want, resolved)
	})

	t.Run("ResolveContainedPath rejects a symlinked ancestor that escapes the repository", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "escape-link")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		_, err := projectconfig.ResolveContainedPath(root, "escape-link/not-yet-created/leaf")
		require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE", "expected a symlinked ancestor escaping the repository to be rejected")
	})

	t.Run("ResolveContainedPath rejects an existing leaf that is itself a symlink escaping the repository", func(t *testing.T) {
		root := t.TempDir()
		outsideFile := filepath.Join(t.TempDir(), "outside.toml")
		require.NoError(t, os.WriteFile(outsideFile, []byte("x"), 0o644), "write outside file")
		if err := os.Symlink(outsideFile, filepath.Join(root, "leaf.toml")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		_, err := projectconfig.ResolveContainedPath(root, "leaf.toml")
		require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE", "expected a leaf symlink escaping the repository to be rejected")
	})

	t.Run("ResolveContainedPath rejects a missing repository root", func(t *testing.T) {
		_, err := projectconfig.ResolveContainedPath(filepath.Join(t.TempDir(), "does-not-exist"), "config")
		require.ErrorContains(t, err, "GOLC_CONFIG_ROOT_MISSING")
	})
}
