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
		"schema_version = 1",
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
		"schema_version = 1",
		"",
		"[runtime]",
		`log_level = "info"`,
		"",
	}, "\n")
	toolchainConcern := strings.Join([]string{
		"schema_version = 1",
		"",
		"[toolchain.go]",
		`version = "1.26.5"`,
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
	if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(toolchainConcern), 0o644); err != nil {
		t.Fatalf("write toolchain concern: %v", err)
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}
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
		if err != nil {
			t.Fatalf("ResolveKey (committed only) failed: %v", err)
		}
		if record.Value != "info" || record.Layer != "committed" || record.Source != "config/runtime.toml" {
			t.Fatalf("unexpected committed-only record: %+v", record)
		}
		if len(record.Shadowed) != 0 {
			t.Fatalf("expected no shadowed origins at committed layer, got %v", record.Shadowed)
		}

		// + user layer.
		writeUserConfig(t, userPath, "schema_version = 1\n\n[runtime]\nlog_level = \"debug\"\n")
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveKey (user) failed: %v", err)
		}
		if record.Value != "debug" || record.Layer != "user" || record.Source != "config.toml" {
			t.Fatalf("expected user layer to win, got %+v", record)
		}
		if len(record.Shadowed) != 1 || record.Shadowed[0].Layer != "committed" || record.Shadowed[0].Value != "info" {
			t.Fatalf("expected exactly the committed origin shadowed, got %v", record.Shadowed)
		}

		// + project-local layer (golc.local.toml), written directly since
		// this synthetic registry's writable key matches production's.
		if err := projectconfig.WriteLocal(root, "runtime.log_level", "warn"); err != nil {
			t.Fatalf("WriteLocal failed: %v", err)
		}
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveKey (project-local) failed: %v", err)
		}
		if record.Value != "warn" || record.Layer != "project-local" || record.Source != "golc.local.toml" {
			t.Fatalf("expected project-local layer to win, got %+v", record)
		}
		wantShadowed := []string{"user:debug", "committed:info"}
		if got := originSummary(record.Shadowed); !equalStrings(got, wantShadowed) {
			t.Fatalf("expected shadowed order %v, got %v", wantShadowed, got)
		}

		// + environment layer.
		sources.LookupEnv = func(name string) (string, bool) {
			if name == "GOLC_TEST_RUNTIME_LOG_LEVEL" {
				return "error", true
			}
			return "", false
		}
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveKey (environment) failed: %v", err)
		}
		if record.Value != "error" || record.Layer != "environment" || record.Source != "GOLC_TEST_RUNTIME_LOG_LEVEL" {
			t.Fatalf("expected environment layer to win, got %+v", record)
		}
		wantShadowed = []string{"project-local:warn", "user:debug", "committed:info"}
		if got := originSummary(record.Shadowed); !equalStrings(got, wantShadowed) {
			t.Fatalf("expected shadowed order %v, got %v", wantShadowed, got)
		}

		// + CLI layer (highest precedence).
		sources.CLIOverrides = map[string]string{"runtime.log_level": "debug"}
		record, err = projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveKey (cli) failed: %v", err)
		}
		if record.Value != "debug" || record.Layer != "cli" || record.Source != "cli" {
			t.Fatalf("expected cli layer to win, got %+v", record)
		}
		wantShadowed = []string{"environment:error", "project-local:warn", "user:debug", "committed:info"}
		if got := originSummary(record.Shadowed); !equalStrings(got, wantShadowed) {
			t.Fatalf("expected shadowed order %v, got %v", wantShadowed, got)
		}

		// ResolveAll must agree with ResolveKey for the same sources.
		all, err := projectconfig.ResolveAll(registry, sources)
		if err != nil {
			t.Fatalf("ResolveAll failed: %v", err)
		}
		if all["runtime.log_level"].Value != record.Value || all["runtime.log_level"].Layer != record.Layer {
			t.Fatalf("ResolveAll disagreed with ResolveKey: %+v vs %+v", all["runtime.log_level"], record)
		}
	})

	t.Run("locked keys reject every higher-layer override attempt", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()

		t.Run("user layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			writeUserConfig(t, userPath, "schema_version = 1\n\n[toolchain.go]\nversion = \"9.9.9\"\n")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_LOCKED_OVERRIDE") {
				t.Fatalf("expected GOLC_CONFIG_LOCKED_OVERRIDE, got %v", err)
			}
		})

		t.Run("project-local layer", func(t *testing.T) {
			// golc.local.toml's own strict layer also refuses this key
			// (local.go's independent registry locks every toolchain pin),
			// so the rejection is defense-in-depth: either stable code
			// confirms the override never takes effect.
			localPath := filepath.Join(root, "golc.local.toml")
			if err := os.WriteFile(localPath, []byte("schema_version = 1\n\n[toolchain.go]\nversion = \"9.9.9\"\n"), 0o644); err != nil {
				t.Fatalf("write golc.local.toml: %v", err)
			}
			defer os.Remove(localPath)
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			if err == nil || !strings.Contains(err.Error(), "LOCKED") {
				t.Fatalf("expected a locked-key rejection, got %v", err)
			}
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
			if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_LOCKED_OVERRIDE") {
				t.Fatalf("expected GOLC_CONFIG_LOCKED_OVERRIDE, got %v", err)
			}
		})

		t.Run("cli layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{
				Root: root, UserConfigPath: userPath, LookupEnv: noEnv,
				CLIOverrides: map[string]string{"toolchain.go.version": "9.9.9"},
			}
			_, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_LOCKED_OVERRIDE") {
				t.Fatalf("expected GOLC_CONFIG_LOCKED_OVERRIDE, got %v", err)
			}
		})

		t.Run("locked key with no override attempt still resolves to committed", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			record, err := projectconfig.ResolveKey(registry, sources, "toolchain.go.version")
			if err != nil {
				t.Fatalf("ResolveKey failed: %v", err)
			}
			if record.Value != "1.26.5" || record.Layer != "committed" {
				t.Fatalf("expected locked key to resolve to its committed value, got %+v", record)
			}
		})
	})

	t.Run("user and environment layers reject values outside the allowed set", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()

		t.Run("user layer", func(t *testing.T) {
			userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
			writeUserConfig(t, userPath, "schema_version = 1\n\n[runtime]\nlog_level = \"verbose\"\n")
			sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
			_, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
			if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_VALUE_INVALID") {
				t.Fatalf("expected GOLC_CONFIG_VALUE_INVALID, got %v", err)
			}
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
			if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_VALUE_INVALID") {
				t.Fatalf("expected GOLC_CONFIG_VALUE_INVALID, got %v", err)
			}
		})
	})

	t.Run("unknown user-layer keys fail the whole layer read", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		userPath := filepath.Join(t.TempDir(), "GOLC", "config.toml")
		writeUserConfig(t, userPath, "schema_version = 1\n\n[runtime]\nmystery = \"x\"\n")
		sources := projectconfig.Sources{Root: root, UserConfigPath: userPath, LookupEnv: noEnv}
		_, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_USER_KEY_UNKNOWN") {
			t.Fatalf("expected GOLC_CONFIG_USER_KEY_UNKNOWN, got %v", err)
		}
	})

	t.Run("a missing user layer file resolves as an empty optional layer", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		sources := projectconfig.Sources{
			Root: root, UserConfigPath: filepath.Join(t.TempDir(), "GOLC", "config.toml"), LookupEnv: noEnv,
		}
		record, err := projectconfig.ResolveKey(registry, sources, "runtime.log_level")
		if err != nil {
			t.Fatalf("ResolveKey failed with a missing user file: %v", err)
		}
		if record.Layer != "committed" {
			t.Fatalf("expected a missing user file to fall through to committed, got %+v", record)
		}
	})

	t.Run("resolving an unregistered key fails", func(t *testing.T) {
		root := newResolveTestRepository(t)
		registry := resolveTestRegistry()
		sources := projectconfig.Sources{Root: root, LookupEnv: noEnv}
		_, err := projectconfig.ResolveKey(registry, sources, "runtime.unknown_key")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_FIELD_UNKNOWN") {
			t.Fatalf("expected GOLC_CONFIG_FIELD_UNKNOWN, got %v", err)
		}
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
		if err != nil {
			t.Fatalf("first ExplainRecord failed: %v", err)
		}
		second, err := projectconfig.ExplainRecord(record, sensitiveSpec)
		if err != nil {
			t.Fatalf("second ExplainRecord failed: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("ExplainRecord output is not byte-identical:\n%q\n%q", first, second)
		}
		if !bytes.HasSuffix(first, []byte("\n")) {
			t.Fatal("ExplainRecord output must end with a single trailing newline")
		}

		document := map[string]any{}
		if err := json.Unmarshal(first, &document); err != nil {
			t.Fatalf("ExplainRecord output is not JSON: %v", err)
		}
		fields := make([]string, 0, len(document))
		for field := range document {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		want := []string{"key", "layer", "shadowed", "source", "value"}
		if strings.Join(fields, ",") != strings.Join(want, ",") {
			t.Fatalf("expected exactly allowlisted fields %v, got %v", want, fields)
		}
		if document["value"] != "<set>" {
			t.Fatalf("expected sensitive winning value to render <set>, got %v", document["value"])
		}
		shadowed, ok := document["shadowed"].([]any)
		if !ok || len(shadowed) != 1 {
			t.Fatalf("expected exactly one shadowed origin, got %v", document["shadowed"])
		}
		origin, ok := shadowed[0].(map[string]any)
		if !ok || origin["value"] != "<unset>" {
			t.Fatalf("expected sensitive shadowed empty value to render <unset>, got %v", shadowed[0])
		}

		lowered := strings.ToLower(string(first))
		if strings.Contains(lowered, "lin_api_deadbeef") {
			t.Fatal("ExplainRecord must never leak a sensitive literal value")
		}

		nonSensitive, err := projectconfig.ExplainRecord(record, projectconfig.FieldSpec{Sensitive: false})
		if err != nil {
			t.Fatalf("non-sensitive ExplainRecord failed: %v", err)
		}
		if !strings.Contains(string(nonSensitive), "lin_api_deadbeef") {
			t.Fatal("expected a non-sensitive field to render its literal value")
		}
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
		if err != nil {
			t.Fatalf("ResolveKey failed: %v", err)
		}

		payload, err := projectconfig.ExplainRecord(record, registry.Fields["runtime.log_level"])
		if err != nil {
			t.Fatalf("ExplainRecord failed: %v", err)
		}

		golden, err := os.ReadFile(filepath.Join(root, "tests", "golden", "config-explain.json"))
		if err != nil {
			t.Fatalf("read golden fixture: %v", err)
		}
		if !bytes.Equal(payload, golden) {
			t.Fatalf("explain output does not match tests/golden/config-explain.json:\ngot:  %s\nwant: %s", payload, golden)
		}

		lowered := strings.ToLower(string(golden))
		for _, forbidden := range []string{"lin_api_", "-----begin", "secret", "password"} {
			if strings.Contains(lowered, forbidden) {
				t.Fatalf("golden fixture must be credential-free, found %q", forbidden)
			}
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

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
