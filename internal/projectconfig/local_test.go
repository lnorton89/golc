// Package projectconfig_test covers the machine-local configuration layer
// (CONTEXT D-06/D-07): contained atomic writes to golc.local.toml, strict
// unknown/locked-key rejection, safe two-layer provenance, and
// deterministic explain output.
//
// It is an external test package so it can declare its quick-test scope
// through the command package's exact registration entrypoint without an
// import cycle (internal/command imports internal/projectconfig).
package projectconfig_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/projectconfig"
)

// The config-local quick-test scope is declared through the exact
// production entrypoint (01-VALIDATION: every owning Go test task registers
// its scope through MustDeclareScope beside its TestScope marker).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "config-local",
	Summary: "Machine-local configuration write, resolution, and provenance tests.",
})

// newLocalTestRepository creates a minimal repository root with a strict
// root index and a committed runtime concern owning runtime.log_level.
func newLocalTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	rootIndex := strings.Join([]string{
		"schema_version = 2",
		"",
		"[[concerns]]",
		`id = "runtime"`,
		`path = "config/runtime.toml"`,
		"",
	}, "\n")
	runtimeConcern := strings.Join([]string{
		"schema_version = 2",
		"",
		"[runtime]",
		`log_level = "info"`,
		"",
	}, "\n")

	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(rootIndex), 0o644); err != nil {
		t.Fatalf("write root index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "runtime.toml"), []byte(runtimeConcern), 0o644); err != nil {
		t.Fatalf("write runtime concern: %v", err)
	}
	return root
}

// listRootEntries returns the sorted file names directly under root.
func listRootEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// TestScopeConfigLocal is the exact quick-test marker for scope
// "config-local" (test --quick --scope config-local).
func TestScopeConfigLocal(t *testing.T) {
	t.Run("write persists only golc.local.toml atomically and survives fresh reads", func(t *testing.T) {
		root := newLocalTestRepository(t)
		before := listRootEntries(t, root)

		if err := projectconfig.WriteLocal(root, "runtime.log_level", "debug"); err != nil {
			t.Fatalf("WriteLocal failed: %v", err)
		}

		after := listRootEntries(t, root)
		added := []string{}
		for _, name := range after {
			found := false
			for _, existing := range before {
				if name == existing {
					found = true
					break
				}
			}
			if !found {
				added = append(added, name)
			}
		}
		if len(added) != 1 || added[0] != "golc.local.toml" {
			t.Fatalf("expected exactly golc.local.toml to be created, got new entries %v", added)
		}
		for _, name := range after {
			if strings.Contains(name, ".tmp") {
				t.Fatalf("temporary file %q leaked after atomic replacement", name)
			}
		}

		committed, err := os.ReadFile(filepath.Join(root, "config", "runtime.toml"))
		if err != nil {
			t.Fatalf("read committed concern: %v", err)
		}
		if !strings.Contains(string(committed), `log_level = "info"`) {
			t.Fatal("committed runtime concern must not be modified by a local write")
		}

		// A fresh resolution reads only durable disk state, standing in for
		// the acceptance harness's separate-process readback.
		resolved, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveRuntime failed: %v", err)
		}
		if resolved.Value != "debug" {
			t.Fatalf("expected local value debug to win, got %q", resolved.Value)
		}
		if resolved.Layer != "project-local" {
			t.Fatalf("expected winning layer project-local, got %q", resolved.Layer)
		}
		if resolved.Source != "golc.local.toml" {
			t.Fatalf("expected safe source golc.local.toml, got %q", resolved.Source)
		}
		if len(resolved.Shadowed) != 1 {
			t.Fatalf("expected exactly one shadowed origin, got %d", len(resolved.Shadowed))
		}
		shadowed := resolved.Shadowed[0]
		if shadowed.Layer != "committed" || shadowed.Source != "config/runtime.toml" || shadowed.Value != "info" {
			t.Fatalf("expected shadowed committed origin config/runtime.toml=info, got %+v", shadowed)
		}
	})

	t.Run("atomic replacement overwrites an existing local value", func(t *testing.T) {
		root := newLocalTestRepository(t)
		if err := projectconfig.WriteLocal(root, "runtime.log_level", "warn"); err != nil {
			t.Fatalf("first WriteLocal failed: %v", err)
		}
		if err := projectconfig.WriteLocal(root, "runtime.log_level", "debug"); err != nil {
			t.Fatalf("second WriteLocal failed: %v", err)
		}
		resolved, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveRuntime failed: %v", err)
		}
		if resolved.Value != "debug" {
			t.Fatalf("expected replaced value debug, got %q", resolved.Value)
		}
	})

	t.Run("unknown keys are rejected without writing", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "runtime.unknown_key", "x")
		if err == nil {
			t.Fatal("expected unknown key to be rejected")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_KEY_UNKNOWN") {
			t.Fatalf("expected GOLC_CONFIG_LOCAL_KEY_UNKNOWN, got %q", err.Error())
		}
		if _, statErr := os.Stat(filepath.Join(root, "golc.local.toml")); !os.IsNotExist(statErr) {
			t.Fatal("rejected write must not create golc.local.toml")
		}
	})

	t.Run("locked keys are rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		for _, key := range []string{
			"schema_version",
			"toolchain.go.version",
			"toolchain.go.platforms.windows-amd64.archive_url",
			"toolchain.go.platforms.windows-amd64.archive_sha256",
		} {
			err := projectconfig.WriteLocal(root, key, "override")
			if err == nil {
				t.Fatalf("expected locked key %q to be rejected", key)
			}
			if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_KEY_LOCKED") {
				t.Fatalf("expected GOLC_CONFIG_LOCAL_KEY_LOCKED for %q, got %q", key, err.Error())
			}
		}
	})

	t.Run("path redirection and .env targets are rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		for _, key := range []string{
			".env",
			".env.log_level",
			"../escape",
			"config/.env",
			`config\.env`,
			"runtime..log_level",
			"runtime.-log_level",
			"runtime.log_level-",
			"runtime.log--level",
			"/runtime.log_level",
		} {
			err := projectconfig.WriteLocal(root, key, "debug")
			if err == nil {
				t.Fatalf("expected redirecting key %q to be rejected", key)
			}
			if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_KEY_REDIRECT") {
				t.Fatalf("expected GOLC_CONFIG_LOCAL_KEY_REDIRECT for %q, got %q", key, err.Error())
			}
		}
		if _, statErr := os.Stat(filepath.Join(root, ".env")); !os.IsNotExist(statErr) {
			t.Fatal("rejected redirect must never create .env")
		}
	})

	t.Run("canonical grammar narrowly admits hyphenated platform segments", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "toolchain.go.platforms.windows-amd64.archive_url", "override")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_KEY_LOCKED") {
			t.Fatalf("expected registered hyphenated key to pass grammar and fail as locked, got %v", err)
		}
	})

	t.Run("invalid values are rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "runtime.log_level", "verbose")
		if err == nil {
			t.Fatal("expected invalid value to be rejected")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_VALUE_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_LOCAL_VALUE_INVALID, got %q", err.Error())
		}
	})

	t.Run("hand-edited local files with unknown keys fail strictly", func(t *testing.T) {
		root := newLocalTestRepository(t)
		edited := strings.Join([]string{
			"schema_version = 2",
			"",
			"[runtime]",
			`log_level = "debug"`,
			`surprise = "value"`,
			"",
		}, "\n")
		if err := os.WriteFile(filepath.Join(root, "golc.local.toml"), []byte(edited), 0o644); err != nil {
			t.Fatalf("write edited local file: %v", err)
		}
		_, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		if err == nil {
			t.Fatal("expected unknown local key to fail resolution")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_KEY_UNKNOWN") {
			t.Fatalf("expected GOLC_CONFIG_LOCAL_KEY_UNKNOWN, got %q", err.Error())
		}
	})

	t.Run("explain without a local layer reports the committed origin", func(t *testing.T) {
		root := newLocalTestRepository(t)
		payload, err := projectconfig.Explain(root, "runtime.log_level")
		if err != nil {
			t.Fatalf("Explain failed: %v", err)
		}
		document := map[string]any{}
		if err := json.Unmarshal(payload, &document); err != nil {
			t.Fatalf("explain output is not JSON: %v", err)
		}
		if document["layer"] != "committed" || document["source"] != "config/runtime.toml" || document["value"] != "info" {
			t.Fatalf("expected committed provenance, got %v", document)
		}
		shadowed, ok := document["shadowed"].([]any)
		if !ok || len(shadowed) != 0 {
			t.Fatalf("expected empty shadowed array, got %v", document["shadowed"])
		}
	})

	t.Run("explain is deterministic and exposes only allowlisted safe fields", func(t *testing.T) {
		root := newLocalTestRepository(t)
		if err := projectconfig.WriteLocal(root, "runtime.log_level", "debug"); err != nil {
			t.Fatalf("WriteLocal failed: %v", err)
		}

		first, err := projectconfig.Explain(root, "runtime.log_level")
		if err != nil {
			t.Fatalf("first Explain failed: %v", err)
		}
		second, err := projectconfig.Explain(root, "runtime.log_level")
		if err != nil {
			t.Fatalf("second Explain failed: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("explain output is not byte-identical:\n%q\n%q", first, second)
		}
		if !bytes.HasSuffix(first, []byte("\n")) {
			t.Fatal("explain output must end with a single trailing newline")
		}

		document := map[string]any{}
		if err := json.Unmarshal(first, &document); err != nil {
			t.Fatalf("explain output is not JSON: %v", err)
		}
		got := make([]string, 0, len(document))
		for field := range document {
			got = append(got, field)
		}
		sort.Strings(got)
		want := []string{"key", "layer", "shadowed", "source", "value"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("expected exactly allowlisted fields %v, got %v", want, got)
		}
		if document["key"] != "runtime.log_level" || document["layer"] != "project-local" ||
			document["source"] != "golc.local.toml" || document["value"] != "debug" {
			t.Fatalf("unexpected provenance payload: %v", document)
		}

		shadowed, ok := document["shadowed"].([]any)
		if !ok || len(shadowed) != 1 {
			t.Fatalf("expected exactly one ordered shadowed origin, got %v", document["shadowed"])
		}
		origin, ok := shadowed[0].(map[string]any)
		if !ok {
			t.Fatalf("shadowed origin is not an object: %v", shadowed[0])
		}
		originFields := make([]string, 0, len(origin))
		for field := range origin {
			originFields = append(originFields, field)
		}
		sort.Strings(originFields)
		if strings.Join(originFields, ",") != "layer,source,value" {
			t.Fatalf("expected shadowed origin fields [layer source value], got %v", originFields)
		}
		if origin["layer"] != "committed" || origin["source"] != "config/runtime.toml" || origin["value"] != "info" {
			t.Fatalf("unexpected shadowed origin: %v", origin)
		}

		lowered := strings.ToLower(string(first))
		for _, forbidden := range []string{"environment", "credential", "secret", "token", "path\\", "path/"} {
			if strings.Contains(lowered, forbidden) {
				t.Fatalf("explain output leaks forbidden content %q: %s", forbidden, first)
			}
		}
	})

	t.Run("a symlinked local destination is rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		outside := filepath.Join(t.TempDir(), "outside.toml")
		if err := os.WriteFile(outside, []byte("schema_version = 2\n"), 0o644); err != nil {
			t.Fatalf("write outside target: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "golc.local.toml")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		err := projectconfig.WriteLocal(root, "runtime.log_level", "debug")
		if err == nil {
			t.Fatal("expected symlinked golc.local.toml destination to be rejected")
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_LOCAL_PATH_ESCAPE") {
			t.Fatalf("expected GOLC_CONFIG_LOCAL_PATH_ESCAPE, got %q", err.Error())
		}
	})
}
