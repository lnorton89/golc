// resolve_test.go covers five-layer configuration resolution (CONTEXT
// D-06/D-07): committed -> user -> project-local -> environment -> CLI
// precedence for every adjacent layer pair, locked-key rejection from
// every higher layer, and deterministic safe-provenance rendering
// including sensitive set/unset disclosure.
//
// It is an external test package (like local_test.go and strict_test.go)
// so it can declare its quick-test scope through the command package's
// exact registration entrypoint without an import cycle. This file owns
// the "config" scope declaration and the TestScopeConfig marker; path.go's
// containment tests (path_test.go, same package) are pulled in as a
// subtest so one quick-test invocation covers both.
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

// The config quick-test scope is declared through the exact production
// entrypoint (01-VALIDATION: every owning Go test task registers its scope
// through MustDeclareScope beside its TestScope marker, pattern set by
// config-local/config-strict). This scope name intentionally matches the
// production CLI scope internal/command/config.go declares for "config
// inspect/set/explain": both declarations coexist safely because
// NewDefaultCommandRegistry (the only place a duplicate scope would be
// rejected) is never invoked by the quick-test dispatcher, which discovers
// TestScope{PascalName} markers through `go test -list` instead.
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "config",
	Summary: "Five-layer configuration resolution, path containment, and safe provenance tests.",
})

// newResolveTestRepository materializes a minimal two-concern repository:
// "runtime" owns the one writable production key (runtime.log_level) and
// "toolchain" owns a locked key (toolchain.go.version) used to exercise
// locked-key rejection from every higher layer.
func newResolveTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	rootIndex := strings.Join([]string{
		"schema_version = 2",
		"",
		"[[concerns]]",
		`id = "runtime"`,
		`path = "config/runtime.toml"`,
		"",
		"[[concerns]]",
		`id = "toolchain"`,
		`path = "config/toolchain.toml"`,
		"",
	}, "\n")
	runtimeConcern := strings.Join([]string{
		"schema_version = 2",
		"",
		"[runtime]",
		`log_level = "info"`,
		"",
	}, "\n")
	toolchainConcern := strings.Join([]string{
		"schema_version = 2",
		"",
		"[toolchain.go]",
		`version = "1.26.5"`,
		"",
	}, "\n")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "config"), 0o755), "mkdir config")
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(rootIndex), 0o644), "write root index")
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "runtime.toml"), []byte(runtimeConcern), 0o644), "write runtime concern")
	require.NoError(t, os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(toolchainConcern), 0o644), "write toolchain concern")
	return root
}

// resolveTestRegistry declares runtime.log_level as writable (matching
// DefaultRegistry's only writable production key) and toolchain.go.version
// as locked, both with an allowlisted environment variable so every layer
// is independently exercisable in tests.
func resolveTestRegistry() projectconfig.Registry {
	return projectconfig.Registry{
		Fields: map[string]projectconfig.FieldSpec{
			"runtime.log_level": {
				AllowedValues: []string{"debug", "error", "info", "warn"},
				EnvVar:        "GOLC_TEST_RUNTIME_LOG_LEVEL",
			},
			"toolchain.go.version": {
				Locked: true,
				EnvVar: "GOLC_TEST_TOOLCHAIN_GO_VERSION",
			},
		},
	}
}

// writeUserConfig writes a strict user-layer document at path.
func writeUserConfig(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir user config dir")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644), "write user config")
}

// noEnv is a LookupEnv stand-in that never reports a set variable, keeping
// resolution independent of the real process environment.
func noEnv(string) (string, bool) { return "", false }

// TestScopeConfig is the exact quick-test marker for scope "config" (test
// --quick --scope config).
func TestScopeConfig(t *testing.T) {
	t.Run("five-layer precedence resolves every adjacent pair in order", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")

		sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}

		// Committed only.
		record, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey (committed only) failed")
		require.Equal(t, "info", record.Value, "unexpected committed-only record: %+v", record)
		require.Equal(t, "committed", record.Layer, "unexpected committed-only record: %+v", record)
		require.Equal(t, "config/runtime.toml", record.Source, "unexpected committed-only record: %+v", record)
		require.Empty(t, record.Shadowed, "expected no shadowed origins at committed layer")

		// + user layer.
		writeUserConfig(t, userPath, "schema_version = 2\n\n[runtime]\nlog_level = \"debug\"\n")
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey (user) failed")
		require.Equal(t, "debug", record.Value, "expected user layer to win, got %+v", record)
		require.Equal(t, "user", record.Layer, "expected user layer to win, got %+v", record)
		require.Equal(t, "config.toml", record.Source, "expected user layer to win, got %+v", record)
		require.Len(t, record.Shadowed, 1, "expected exactly the committed origin shadowed, got %v", record.Shadowed)
		require.Equal(t, "committed", record.Shadowed[0].Layer)
		require.Equal(t, "info", record.Shadowed[0].Value)

		// + project-local layer (golc.local.toml), written directly since
		// this synthetic registry's writable key matches production's.
		require.NoError(t, projectconfig.WriteLocal(root, "runtime.log_level", "warn"), "WriteLocal failed")
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey (project-local) failed")
		require.Equal(t, "warn", record.Value, "expected project-local layer to win, got %+v", record)
		require.Equal(t, "project-local", record.Layer, "expected project-local layer to win, got %+v", record)
		require.Equal(t, "golc.local.toml", record.Source, "expected project-local layer to win, got %+v", record)
		wantShadowed := []string{"user:debug", "committed:info"}
		require.Equal(t, wantShadowed, originSummary(record.Shadowed), "expected shadowed order")

		// + environment layer.
		sources.LookupEnv = func(name string) (string, bool) {
			if name == "GOLC_TEST_RUNTIME_LOG_LEVEL" {
				return "error", true
			}
			return "", false
		}
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey (environment) failed")
		require.Equal(t, "error", record.Value, "expected environment layer to win, got %+v", record)
		require.Equal(t, "environment", record.Layer, "expected environment layer to win, got %+v", record)
		require.Equal(t, "GOLC_TEST_RUNTIME_LOG_LEVEL", record.Source, "expected environment layer to win, got %+v", record)
		wantShadowed = []string{"project-local:warn", "user:debug", "committed:info"}
		require.Equal(t, wantShadowed, originSummary(record.Shadowed), "expected shadowed order")

		// + CLI layer (highest precedence).
		sources.CLIOverrides = map[string]string{"runtime.log_level": "debug"}
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey (cli) failed")
		require.Equal(t, "debug", record.Value, "expected cli layer to win, got %+v", record)
		require.Equal(t, "cli", record.Layer, "expected cli layer to win, got %+v", record)
		require.Equal(t, "cli", record.Source, "expected cli layer to win, got %+v", record)
		wantShadowed = []string{"environment:error", "project-local:warn", "user:debug", "committed:info"}
		require.Equal(t, wantShadowed, originSummary(record.Shadowed), "expected shadowed order")

		// ResolveAll must agree with ResolveKey for the same sources.
		all, err := projectconfig.ResolveAll(registry, sources)
		require.NoError(t, err, "ResolveAll failed")
		require.Equal(t, record.Value, all["runtime.log_level"].Value, "ResolveAll disagreed with ResolveKey: %+v vs %+v", all["runtime.log_level"], record)
		require.Equal(t, record.Layer, all["runtime.log_level"].Layer, "ResolveAll disagreed with ResolveKey: %+v vs %+v", all["runtime.log_level"], record)
	})

	t.Run("locked keys reject every higher-layer override attempt", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()

		t.Run("user layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			writeUserConfig(t, userPath, "schema_version = 2\n\n[toolchain.go]\nversion = \"9.9.9\"\n")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			require.ErrorContains(t, err, "GOLC_CONFIG_LOCKED_OVERRIDE")
		})

		t.Run("project-local layer", func(t *testing.T) {
			// golc.local.toml's own strict layer also refuses this key
			// (local.go's independent registry locks every toolchain pin),
			// so the rejection is defense-in-depth: either stable code
			// confirms the override never takes effect.
			localPath := filepath.Join(root, "golc.local.toml")
			require.NoError(t, os.WriteFile(localPath, []byte("schema_version = 2\n\n[toolchain.go]\nversion = \"9.9.9\"\n"), 0o644), "write golc.local.toml")
			defer os.Remove(localPath)
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			require.ErrorContains(t, err, "LOCKED", "expected a locked-key rejection")
		})

		t.Run("environment layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{
				Root: root, UserConfigPath: userPath,
				LookupEnv: func(name string) (string, bool) {
					if name == "GOLC_TEST_TOOLCHAIN_GO_VERSION" {
						return "9.9.9", true
					}
					return "", false
				},
			}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			require.ErrorContains(t, err, "GOLC_CONFIG_LOCKED_OVERRIDE")
		})

		t.Run("cli layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{
				Root: root, UserConfigPath: userPath, LookupEnv: noEnv,
				CLIOverrides: map[string]string{"toolchain.go.version": "9.9.9"},
			}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			require.ErrorContains(t, err, "GOLC_CONFIG_LOCKED_OVERRIDE")
		})

		t.Run("locked key with no override attempt still resolves to committed", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			record, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			require.NoError(t, err, "ResolveKey failed")
			require.Equal(t, "1.26.5", record.Value, "expected locked key to resolve to its committed value, got %+v", record)
			require.Equal(t, "committed", record.Layer, "expected locked key to resolve to its committed value, got %+v", record)
		})
	})

	t.Run("user and environment layers reject values outside the allowed set", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()

		t.Run("user layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			writeUserConfig(t, userPath, "schema_version = 2\n\n[runtime]\nlog_level = \"verbose\"\n")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
			require.ErrorContains(t, err, "GOLC_CONFIG_VALUE_INVALID")
		})

		t.Run("environment layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{
				Root: root, UserConfigPath: userPath,
				LookupEnv: func(name string) (string, bool) {
					if name == "GOLC_TEST_RUNTIME_LOG_LEVEL" {
						return "verbose", true
					}
					return "", false
				},
			}
			_, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
			require.ErrorContains(t, err, "GOLC_CONFIG_VALUE_INVALID")
		})
	})

	t.Run("unknown user-layer keys fail the whole layer read", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
		writeUserConfig(t, userPath, "schema_version = 2\n\n[runtime]\nmystery = \"x\"\n")
		sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
		_, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.ErrorContains(t, err, "GOLC_CONFIG_USER_KEY_UNKNOWN")
	})

	t.Run("a missing user layer file resolves as an empty optional layer", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		sources := projectconfig.Sources{
			Root: root, UserConfigPath: filepath.Join(t.TempDir(), "GOLC", "config.toml"), LookupEnv: noEnv,
		}
		record, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey failed with a missing user file")
		require.Equal(t, "committed", record.Layer, "expected a missing user file to fall through to committed, got %+v", record)
	})

	t.Run("resolving an unregistered key fails", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		sources := projectconfig.Sources{Root: root, LookupEnv: noEnv}
		_, err := projectconfig.ResolveKey(registry, sources, "runtime.unknown_key")
		require.ErrorContains(t, err, "GOLC_CONFIG_FIELD_UNKNOWN")
	})

	t.Run("ExplainRecord is deterministic and renders sensitive declarations as set/unset only", func(t *testing.T) {
		record := projectconfig.ResolvedRecord{
			Key: "linear.env.api_key", Layer: "cli", Source: "cli", Value: "lin_api_deadbeef",
			Shadowed: []projectconfig.Origin{
				{Layer: "committed", Source: "config/integrations/linear.toml", Value: ""},
			},
		}
		sensitiveSpec := projectconfig.FieldSpec{Sensitive: true}

		first, err := projectconfig.ExplainRecord(record, sensitiveSpec)
		require.NoError(t, err, "first ExplainRecord failed")
		second, err := projectconfig.ExplainRecord(record, sensitiveSpec)
		require.NoError(t, err, "second ExplainRecord failed")
		require.Equal(t, first, second, "ExplainRecord output is not byte-identical")
		require.True(t, strings.HasSuffix(string(first), "\n"), "ExplainRecord output must end with a single trailing newline")

		document := map[string]any{}
		require.NoError(t, json.Unmarshal(first, &document), "ExplainRecord output is not JSON")
		fields := make([]string, 0, len(document))
		for field := range document {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		want := []string{"key", "layer", "shadowed", "source", "value"}
		require.Equal(t, strings.Join(want, ","), strings.Join(fields, ","), "expected exactly allowlisted fields %v, got %v", want, fields)
		require.Equal(t, "<set>", document["value"], "expected sensitive winning value to render <set>")
		shadowed, ok := document["shadowed"].([]any)
		require.True(t, ok, "expected exactly one shadowed origin, got %v", document["shadowed"])
		require.Len(t, shadowed, 1, "expected exactly one shadowed origin, got %v", document["shadowed"])
		origin, ok := shadowed[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "<unset>", origin["value"], "expected sensitive shadowed empty value to render <unset>, got %v", shadowed[0])

		lowered := strings.ToLower(string(first))
		require.NotContains(t, lowered, "lin_api_deadbeef", "ExplainRecord must never leak a sensitive literal value")

		nonSensitive, err := projectconfig.ExplainRecord(record, projectconfig.FieldSpec{Sensitive: false})
		require.NoError(t, err, "non-sensitive ExplainRecord failed")
		require.Contains(t, string(nonSensitive), "lin_api_deadbeef", "expected a non-sensitive field to render its literal value")
	})

	t.Run("golden explain output is byte-stable and credential-free", func(t *testing.T) {
		root := repositoryRoot(t)
		registry := projectconfig.DefaultRegistry()
		sources := projectconfig.Sources{
			Root:           root,
			UserConfigPath: filepath.Join(t.TempDir(), "GOLC", "config.toml"),
			LookupEnv:      noEnv,
		}

		record, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		require.NoError(t, err, "ResolveKey failed")

		payload, err := projectconfig.ExplainRecord(record, registry.Fields["runtime.log_level"])
		require.NoError(t, err, "ExplainRecord failed")

		golden, err := os.ReadFile(filepath.Join(root, "tests", "golden", "config-explain.json"))
		require.NoError(t, err, "read golden fixture")
		require.Equal(t, golden, payload, "explain output does not match tests/golden/config-explain.json")

		lowered := strings.ToLower(string(golden))
		for _, forbidden := range []string{"lin_api_", "-----begin", "secret", "password"} {
			require.NotContains(t, lowered, forbidden, "golden fixture must be credential-free")
		}
	})

	t.Run("path containment", testPathContainment)
}

// originSummary renders shadowed origins as "layer:value" strings for
// concise ordered-sequence assertions.
func originSummary(origins []projectconfig.Origin) []string {
	out := make([]string, 0, len(origins))
	for _, origin := range origins {
		out = append(out, origin.Layer+":"+origin.Value)
	}
	return out
}
