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
	"strings"
	"testing"

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
			if err := projectconfig.ValidateConcernPath(relative); err == nil {
				t.Fatalf("expected %q to be rejected", relative)
			} else if !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
				t.Fatalf("expected GOLC_CONFIG_PATH_ESCAPE for %q, got %q", relative, err.Error())
			}
		}
	})

	t.Run("ValidateConcernPath accepts safe repository-relative shapes", func(t *testing.T) {
		for _, relative := range []string{
			"config/toolchain.toml",
			".tools/cache/downloads",
			"a/b/c",
		} {
			if err := projectconfig.ValidateConcernPath(relative); err != nil {
				t.Fatalf("expected %q to be accepted, got %v", relative, err)
			}
		}
	})

	t.Run("ResolveContainedPath rejects lexical escapes before touching disk", func(t *testing.T) {
		root := t.TempDir()
		_, err := projectconfig.ResolveContainedPath(root, "../escape")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
			t.Fatalf("expected GOLC_CONFIG_PATH_ESCAPE, got %v", err)
		}
	})

	t.Run("ResolveContainedPath accepts an existing contained path", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "config", "integrations"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		resolved, err := projectconfig.ResolveContainedPath(root, "config/integrations")
		if err != nil {
			t.Fatalf("ResolveContainedPath failed: %v", err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(root) failed: %v", err)
		}
		want := filepath.Join(resolvedRoot, "config", "integrations")
		if resolved != want {
			t.Fatalf("expected %q, got %q", want, resolved)
		}
	})

	t.Run("ResolveContainedPath accepts a not-yet-created leaf under an existing contained ancestor", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".tools", "cache"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		resolved, err := projectconfig.ResolveContainedPath(root, ".tools/cache/downloads")
		if err != nil {
			t.Fatalf("ResolveContainedPath failed on a lazily created cache path: %v", err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(root) failed: %v", err)
		}
		want := filepath.Join(resolvedRoot, ".tools", "cache", "downloads")
		if resolved != want {
			t.Fatalf("expected %q, got %q", want, resolved)
		}
		if _, statErr := os.Stat(resolved); !os.IsNotExist(statErr) {
			t.Fatal("ResolveContainedPath must not create the leaf itself")
		}
	})

	t.Run("ResolveContainedPath accepts a fully not-yet-created relative path", func(t *testing.T) {
		root := t.TempDir()
		resolved, err := projectconfig.ResolveContainedPath(root, ".tools/cache/downloads")
		if err != nil {
			t.Fatalf("ResolveContainedPath failed when no ancestor exists yet: %v", err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(root) failed: %v", err)
		}
		want := filepath.Join(resolvedRoot, ".tools", "cache", "downloads")
		if resolved != want {
			t.Fatalf("expected %q, got %q", want, resolved)
		}
	})

	t.Run("ResolveContainedPath rejects a symlinked ancestor that escapes the repository", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "escape-link")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		_, err := projectconfig.ResolveContainedPath(root, "escape-link/not-yet-created/leaf")
		if err == nil {
			t.Fatal("expected a symlinked ancestor escaping the repository to be rejected")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
			t.Fatalf("expected GOLC_CONFIG_PATH_ESCAPE, got %q", err.Error())
		}
	})

	t.Run("ResolveContainedPath rejects an existing leaf that is itself a symlink escaping the repository", func(t *testing.T) {
		root := t.TempDir()
		outsideFile := filepath.Join(t.TempDir(), "outside.toml")
		if err := os.WriteFile(outsideFile, []byte("x"), 0o644); err != nil {
			t.Fatalf("write outside file: %v", err)
		}
		if err := os.Symlink(outsideFile, filepath.Join(root, "leaf.toml")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		_, err := projectconfig.ResolveContainedPath(root, "leaf.toml")
		if err == nil {
			t.Fatal("expected a leaf symlink escaping the repository to be rejected")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
			t.Fatalf("expected GOLC_CONFIG_PATH_ESCAPE, got %q", err.Error())
		}
	})

	t.Run("ResolveContainedPath rejects a missing repository root", func(t *testing.T) {
		_, err := projectconfig.ResolveContainedPath(filepath.Join(t.TempDir(), "does-not-exist"), "config")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_ROOT_MISSING") {
			t.Fatalf("expected GOLC_CONFIG_ROOT_MISSING, got %v", err)
		}
	})
}
