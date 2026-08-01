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
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

	require.NoError(t, os.MkdirAll(filepath.Join(root, "config"), 0o755), "mkdir config")
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(rootIndex), 0o644), "write root index")
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "runtime.toml"), []byte(runtimeConcern), 0o644), "write runtime concern")
	return root
}

// listRootEntries returns the sorted file names directly under root.
func listRootEntries(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err, "read root")
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

		require.NoError(t, projectconfig.WriteLocal(root, "runtime.log_level", "debug"), "WriteLocal failed")

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
		require.Len(t, added, 1, "expected exactly golc.local.toml to be created, got new entries %v", added)
		require.Equal(t, "golc.local.toml", added[0])
		for _, name := range after {
			require.NotContains(t, name, ".tmp", "temporary file %q leaked after atomic replacement", name)
		}

		committed, err := os.ReadFile(filepath.Join(root, "config", "runtime.toml"))
		require.NoError(t, err, "read committed concern")
		require.Contains(t, string(committed), `log_level = "info"`, "committed runtime concern must not be modified by a local write")

		// A fresh resolution reads only durable disk state, standing in for
		// the acceptance harness's separate-process readback.
		resolved, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		require.NoError(t, err, "ResolveRuntime failed")
		require.Equal(t, "debug", resolved.Value, "expected local value debug to win")
		require.Equal(t, "project-local", resolved.Layer, "expected winning layer project-local")
		require.Equal(t, "golc.local.toml", resolved.Source, "expected safe source golc.local.toml")
		require.Len(t, resolved.Shadowed, 1, "expected exactly one shadowed origin")
		shadowed := resolved.Shadowed[0]
		require.Equal(t, "committed", shadowed.Layer, "expected shadowed committed origin config/runtime.toml=info, got %+v", shadowed)
		require.Equal(t, "config/runtime.toml", shadowed.Source, "expected shadowed committed origin config/runtime.toml=info, got %+v", shadowed)
		require.Equal(t, "info", shadowed.Value, "expected shadowed committed origin config/runtime.toml=info, got %+v", shadowed)
	})

	t.Run("atomic replacement overwrites an existing local value", func(t *testing.T) {
		root := newLocalTestRepository(t)
		require.NoError(t, projectconfig.WriteLocal(root, "runtime.log_level", "warn"), "first WriteLocal failed")
		require.NoError(t, projectconfig.WriteLocal(root, "runtime.log_level", "debug"), "second WriteLocal failed")
		resolved, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		require.NoError(t, err, "ResolveRuntime failed")
		require.Equal(t, "debug", resolved.Value, "expected replaced value debug")
	})

	t.Run("unknown keys are rejected without writing", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "runtime.unknown_key", "x")
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_UNKNOWN", "expected unknown key to be rejected")
		_, statErr := os.Stat(filepath.Join(root, "golc.local.toml"))
		require.True(t, os.IsNotExist(statErr), "rejected write must not create golc.local.toml")
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
			require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_LOCKED", "expected locked key %q to be rejected", key)
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
			require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_REDIRECT", "expected redirecting key %q to be rejected", key)
		}
		_, statErr := os.Stat(filepath.Join(root, ".env"))
		require.True(t, os.IsNotExist(statErr), "rejected redirect must never create .env")
	})

	t.Run("canonical grammar narrowly admits hyphenated platform segments", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "toolchain.go.platforms.windows-amd64.archive_url", "override")
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_LOCKED", "expected registered hyphenated key to pass grammar and fail as locked")
	})

	t.Run("invalid values are rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		err := projectconfig.WriteLocal(root, "runtime.log_level", "verbose")
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_VALUE_INVALID", "expected invalid value to be rejected")
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
		require.NoError(t, os.WriteFile(filepath.Join(root, "golc.local.toml"), []byte(edited), 0o644), "write edited local file")
		_, err := projectconfig.ResolveRuntime(root, "runtime.log_level")
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_UNKNOWN", "expected unknown local key to fail resolution")
	})

	t.Run("explain without a local layer reports the committed origin", func(t *testing.T) {
		root := newLocalTestRepository(t)
		payload, err := projectconfig.Explain(root, "runtime.log_level")
		require.NoError(t, err, "Explain failed")
		document := map[string]any{}
		require.NoError(t, json.Unmarshal(payload, &document), "explain output is not JSON")
		require.Equal(t, "committed", document["layer"], "expected committed provenance, got %v", document)
		require.Equal(t, "config/runtime.toml", document["source"], "expected committed provenance, got %v", document)
		require.Equal(t, "info", document["value"], "expected committed provenance, got %v", document)
		shadowed, ok := document["shadowed"].([]any)
		require.True(t, ok, "expected empty shadowed array, got %v", document["shadowed"])
		require.Empty(t, shadowed, "expected empty shadowed array, got %v", document["shadowed"])
	})

	t.Run("explain is deterministic and exposes only allowlisted safe fields", func(t *testing.T) {
		root := newLocalTestRepository(t)
		require.NoError(t, projectconfig.WriteLocal(root, "runtime.log_level", "debug"), "WriteLocal failed")

		first, err := projectconfig.Explain(root, "runtime.log_level")
		require.NoError(t, err, "first Explain failed")
		second, err := projectconfig.Explain(root, "runtime.log_level")
		require.NoError(t, err, "second Explain failed")
		require.Equal(t, first, second, "explain output is not byte-identical")
		require.True(t, strings.HasSuffix(string(first), "\n"), "explain output must end with a single trailing newline")

		document := map[string]any{}
		require.NoError(t, json.Unmarshal(first, &document), "explain output is not JSON")
		got := make([]string, 0, len(document))
		for field := range document {
			got = append(got, field)
		}
		sort.Strings(got)
		want := []string{"key", "layer", "shadowed", "source", "value"}
		require.Equal(t, strings.Join(want, ","), strings.Join(got, ","), "expected exactly allowlisted fields %v, got %v", want, got)
		require.Equal(t, "runtime.log_level", document["key"], "unexpected provenance payload: %v", document)
		require.Equal(t, "project-local", document["layer"], "unexpected provenance payload: %v", document)
		require.Equal(t, "golc.local.toml", document["source"], "unexpected provenance payload: %v", document)
		require.Equal(t, "debug", document["value"], "unexpected provenance payload: %v", document)

		shadowed, ok := document["shadowed"].([]any)
		require.True(t, ok, "expected exactly one ordered shadowed origin, got %v", document["shadowed"])
		require.Len(t, shadowed, 1, "expected exactly one ordered shadowed origin, got %v", document["shadowed"])
		origin, ok := shadowed[0].(map[string]any)
		require.True(t, ok, "shadowed origin is not an object: %v", shadowed[0])
		originFields := make([]string, 0, len(origin))
		for field := range origin {
			originFields = append(originFields, field)
		}
		sort.Strings(originFields)
		require.Equal(t, "layer,source,value", strings.Join(originFields, ","), "expected shadowed origin fields [layer source value], got %v", originFields)
		require.Equal(t, "committed", origin["layer"], "unexpected shadowed origin: %v", origin)
		require.Equal(t, "config/runtime.toml", origin["source"], "unexpected shadowed origin: %v", origin)
		require.Equal(t, "info", origin["value"], "unexpected shadowed origin: %v", origin)

		lowered := strings.ToLower(string(first))
		for _, forbidden := range []string{"environment", "credential", "secret", "token", "path\\", "path/"} {
			require.NotContains(t, lowered, forbidden, "explain output leaks forbidden content %q: %s", forbidden, first)
		}
	})

	t.Run("a symlinked local destination is rejected", func(t *testing.T) {
		root := newLocalTestRepository(t)
		outside := filepath.Join(t.TempDir(), "outside.toml")
		require.NoError(t, os.WriteFile(outside, []byte("schema_version = 2\n"), 0o644), "write outside target")
		if err := os.Symlink(outside, filepath.Join(root, "golc.local.toml")); err != nil {
			t.Skipf("symlink creation unavailable on this host: %v", err)
		}
		err := projectconfig.WriteLocal(root, "runtime.log_level", "debug")
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_PATH_ESCAPE", "expected symlinked golc.local.toml destination to be rejected")
	})
}
