// strict_test.go covers the complete strict independently owned concern
// set (CONTEXT D-05/D-09/D-10): the root index discovers exactly the six
// Phase 1 concerns, every canonical key has one owning concern, and
// unknown, duplicate, invalid, deprecated-only, old-plus-new, duplicate
// authority, unresolved, and cyclic inputs fail with distinct stable
// diagnostics while deprecated-only input receives migration guidance.
//
// It is an external test package (like local_test.go) so it can declare
// its quick-test scope through the command package's exact registration
// entrypoint without an import cycle.
package projectconfig_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/projectconfig"
)

// The config-strict quick-test scope is declared through the exact
// production entrypoint (Plan 17 contract: every owning Go test file
// registers its scope beside its TestScope marker; duplicate scope
// declarations fail when the default registry is built, before any test
// handler could run).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "config-strict",
	Summary: "Strict concern-set decoding, authority, reference, and deprecation tests.",
})

// repositoryRoot resolves the real checkout root from the package
// directory so production concern files are validated exactly as
// committed.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "golc.project.toml")); err != nil {
		t.Fatalf("repository root %q has no golc.project.toml: %v", root, err)
	}
	return root
}

// writeStrictRepository materializes a synthetic repository root: a root
// index derived from the spec plus the given concern file contents.
func writeStrictRepository(t *testing.T, spec projectconfig.Spec, files map[string]string) string {
	t.Helper()
	root := t.TempDir()

	var index strings.Builder
	index.WriteString("schema_version = 2\n")
	for _, concern := range spec.Concerns {
		index.WriteString("\n[[concerns]]\n")
		index.WriteString("id = \"" + concern.ID + "\"\n")
		index.WriteString("path = \"" + concern.Path + "\"\n")
	}
	if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(index.String()), 0o644); err != nil {
		t.Fatalf("write root index: %v", err)
	}
	for relative, content := range files {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", relative, err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	return root
}

// syntheticSpec is a small two-concern registry with one deprecation used
// by the failure-mode subtests.
func syntheticSpec() projectconfig.Spec {
	return projectconfig.Spec{
		Concerns: []projectconfig.ConcernSpec{
			{
				ID:   "runtime",
				Path: "config/runtime.toml",
				Keys: map[string]projectconfig.KeySpec{
					"runtime.log_level": {AllowedValues: []string{"debug", "error", "info", "warn"}},
				},
			},
			{
				ID:   "toolchain",
				Path: "config/toolchain.toml",
				Keys: map[string]projectconfig.KeySpec{
					"toolchain.go.version": {Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)},
				},
			},
		},
		Deprecations: []projectconfig.Deprecation{
			{
				OldKey:         "runtime.verbosity",
				ReplacementKey: "runtime.log_level",
				IntroducedIn:   "0.1.0",
				DeprecatedIn:   "0.2.0",
				RemovalPlanned: "1.0.0",
				Message:        "rename runtime.verbosity to runtime.log_level; the value set is unchanged",
			},
		},
	}
}

// strictRuntimeConcern is the well-formed runtime file for syntheticSpec.
const strictRuntimeConcern = "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\n"

// strictToolchainConcern is the well-formed toolchain file for syntheticSpec.
const strictToolchainConcern = "schema_version = 2\n\n[toolchain.go]\nversion = \"1.26.5\"\n"

var magePlatformPins = map[string][2]string{
	"windows-amd64": {
		"https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_Windows-64bit.zip",
		"970bc6efa76d6dc7285098a7033f4e6c83c18dc02f80548ae8de8dc5586e0445",
	},
	"linux-amd64": {
		"https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_Linux-64bit.tar.gz",
		"b1dd189f5a4d38484176dd5be3b651eb7cbc0b78eaf4bb9715738aa24edec644",
	},
	"linux-arm64": {
		"https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_Linux-ARM64.tar.gz",
		"5a88f89b52a0270a60c1fd57f964d24af78ac21c6848642f05db1300fe193980",
	},
	"darwin-amd64": {
		"https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_macOS-64bit.tar.gz",
		"bb43eec76388b1445c4ce019c5ac3bb305a56f77c5f580c5067871ff01ea7741",
	},
	"darwin-arm64": {
		"https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_macOS-ARM64.tar.gz",
		"5fd6f61170bb7584a4ca3ce4fd01137fe5a8edaf6c096d9f2ad30754d1d92797",
	},
}

// TestScopeConfigStrict is the exact quick-test marker for scope
// "config-strict" (test --quick --scope config-strict).
func TestScopeConfigStrict(t *testing.T) {
	t.Run("root index discovers exactly the six phase 1 concerns", func(t *testing.T) {
		root := repositoryRoot(t)
		index, err := projectconfig.LoadRootIndex(root)
		if err != nil {
			t.Fatalf("LoadRootIndex failed: %v", err)
		}
		expected := map[string]string{
			"toolchain":            "config/toolchain.toml",
			"commands":             "config/commands.toml",
			"generation":           "config/generation.toml",
			"application_defaults": "config/application-defaults.toml",
			"runtime":              "config/runtime.toml",
			"linear":               "config/integrations/linear.toml",
		}
		if len(index.Concerns) != len(expected) {
			t.Fatalf("expected exactly %d indexed concerns, got %d", len(expected), len(index.Concerns))
		}
		for _, concern := range index.Concerns {
			path, known := expected[concern.ID]
			if !known {
				t.Fatalf("unexpected indexed concern %q", concern.ID)
			}
			if concern.Path != path {
				t.Fatalf("concern %q must index %q, got %q", concern.ID, path, concern.Path)
			}
		}
	})

	t.Run("production repository validates with one authority per key and no warnings", func(t *testing.T) {
		root := repositoryRoot(t)
		spec := projectconfig.DefaultSpec()
		if err := projectconfig.ValidateAuthority(spec); err != nil {
			t.Fatalf("ValidateAuthority failed: %v", err)
		}
		values, warnings, err := projectconfig.ValidateRepository(root, spec)
		if err != nil {
			t.Fatalf("ValidateRepository failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no production warnings, got %v", warnings)
		}
		if values["runtime.log_level"] == "" {
			t.Fatal("resolved values must include runtime.log_level")
		}
		goVersion := values["toolchain.go.version"]
		if goVersion == "" {
			t.Fatal("resolved values must include toolchain.go.version")
		}
		if values["commands.go_version"] != goVersion {
			t.Fatalf("commands.go_version must resolve through its reference to toolchain.go.version %q, got %q",
				goVersion, values["commands.go_version"])
		}
		if got, want := values["commands.cli_binary"], ".tools/installs/golc_project"; got != want {
			t.Fatalf("commands.cli_binary = %q, want platform-neutral install root %q", got, want)
		}

		// The commands concern must refer to the toolchain authority, never
		// repeat the pinned literal (D-05 single authority).
		commandsBytes, err := os.ReadFile(filepath.Join(root, "config", "commands.toml"))
		if err != nil {
			t.Fatalf("read commands concern: %v", err)
		}
		if !strings.Contains(string(commandsBytes), "ref:toolchain.go.version") {
			t.Fatal("config/commands.toml must declare a typed reference to toolchain.go.version")
		}
		if strings.Contains(string(commandsBytes), goVersion) {
			t.Fatalf("config/commands.toml must not duplicate the pinned Go version literal %q", goVersion)
		}
	})

	t.Run("every production concern validates alone", func(t *testing.T) {
		root := repositoryRoot(t)
		spec := projectconfig.DefaultSpec()
		if len(spec.Concerns) != 6 {
			t.Fatalf("DefaultSpec must declare six concerns, got %d", len(spec.Concerns))
		}
		for _, concern := range spec.Concerns {
			if _, _, err := projectconfig.ValidateConcern(root, spec, concern.ID); err != nil {
				t.Fatalf("concern %q must validate alone: %v", concern.ID, err)
			}
		}
	})

	t.Run("production toolchain owns only the exact schema-v2 keys", func(t *testing.T) {
		spec := projectconfig.DefaultSpec()
		var got []string
		for _, concern := range spec.Concerns {
			if concern.ID != "toolchain" {
				continue
			}
			for key := range concern.Keys {
				got = append(got, key)
			}
		}
		sort.Strings(got)
		want := []string{
			"cache.downloads",
			"cache.gocache",
			"cache.gomodcache",
			"toolchain.go.official_host",
			"toolchain.go.official_path_prefix",
			"toolchain.go.platforms.windows-amd64.archive_sha256",
			"toolchain.go.platforms.windows-amd64.archive_url",
			"toolchain.go.version",
			"toolchain.mage.official_host",
			"toolchain.mage.official_path_prefix",
			"toolchain.mage.platforms.darwin-amd64.archive_sha256",
			"toolchain.mage.platforms.darwin-amd64.archive_url",
			"toolchain.mage.platforms.darwin-arm64.archive_sha256",
			"toolchain.mage.platforms.darwin-arm64.archive_url",
			"toolchain.mage.platforms.linux-amd64.archive_sha256",
			"toolchain.mage.platforms.linux-amd64.archive_url",
			"toolchain.mage.platforms.linux-arm64.archive_sha256",
			"toolchain.mage.platforms.linux-arm64.archive_url",
			"toolchain.mage.platforms.windows-amd64.archive_sha256",
			"toolchain.mage.platforms.windows-amd64.archive_url",
			"toolchain.mage.version",
			"toolchain.node.official_host",
			"toolchain.node.official_path_prefix",
			"toolchain.node.platforms.windows-amd64.archive_sha256",
			"toolchain.node.platforms.windows-amd64.archive_url",
			"toolchain.node.version",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("toolchain keys mismatch:\ngot:  %v\nwant: %v", got, want)
		}
	})

	t.Run("production mage authority pins exactly five official release archives", func(t *testing.T) {
		root := repositoryRoot(t)
		values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "toolchain")
		if err != nil {
			t.Fatalf("production toolchain concern must validate: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no toolchain warnings, got %v", warnings)
		}
		if got, want := values["toolchain.mage.version"], "1.17.2"; got != want {
			t.Fatalf("toolchain.mage.version = %q, want %q", got, want)
		}
		if got, want := values["toolchain.mage.official_host"], "github.com"; got != want {
			t.Fatalf("toolchain.mage.official_host = %q, want %q", got, want)
		}
		if got, want := values["toolchain.mage.official_path_prefix"], "/magefile/mage/releases/download/"; got != want {
			t.Fatalf("toolchain.mage.official_path_prefix = %q, want %q", got, want)
		}
		for platform, pin := range magePlatformPins {
			prefix := "toolchain.mage.platforms." + platform
			if got := values[prefix+".archive_url"]; got != pin[0] {
				t.Errorf("%s.archive_url = %q, want %q", prefix, got, pin[0])
			}
			if got := values[prefix+".archive_sha256"]; got != pin[1] {
				t.Errorf("%s.archive_sha256 = %q, want %q", prefix, got, pin[1])
			}
		}
	})

	t.Run("mage authority rejects missing extra or malformed platform data", func(t *testing.T) {
		root := repositoryRoot(t)
		raw, err := os.ReadFile(filepath.Join(root, "config", "toolchain.toml"))
		if err != nil {
			t.Fatalf("read committed toolchain concern: %v", err)
		}
		valid := string(raw)
		cases := map[string]string{
			"missing platform digest": strings.Replace(valid,
				`archive_sha256 = "5fd6f61170bb7584a4ca3ce4fd01137fe5a8edaf6c096d9f2ad30754d1d92797"`+"\n", "", 1),
			"extra platform": valid + `
[toolchain.mage.platforms."windows-arm64"]
archive_url = "https://github.com/magefile/mage/releases/download/v1.17.2/mage_1.17.2_Windows-ARM64.zip"
archive_sha256 = "970bc6efa76d6dc7285098a7033f4e6c83c18dc02f80548ae8de8dc5586e0445"
`,
			"wrong asset casing": strings.Replace(valid, "mage_1.17.2_Windows-64bit.zip", "mage_1.17.2_windows-64bit.zip", 1),
			"wrong release host": strings.Replace(valid, "https://github.com/magefile/mage/", "https://example.com/magefile/mage/", 1),
			"wrong release path": strings.Replace(valid, "/magefile/mage/releases/download/v1.17.2/", "/magefile/mage/releases/latest/", 1),
			"malformed checksum": strings.Replace(valid,
				"970bc6efa76d6dc7285098a7033f4e6c83c18dc02f80548ae8de8dc5586e0445",
				"NOT-A-SHA256", 1),
		}
		for name, content := range cases {
			t.Run(name, func(t *testing.T) {
				fixtureRoot := writeStrictRepository(t, projectconfig.DefaultSpec(), map[string]string{
					"config/toolchain.toml": content,
				})
				if _, _, err := projectconfig.ValidateConcern(
					fixtureRoot, projectconfig.DefaultSpec(), "toolchain"); err == nil {
					t.Fatal("invalid Mage authority unexpectedly validated")
				}
			})
		}
	})

	t.Run("quoted windows platform tables flatten exactly and unregistered platforms fail", func(t *testing.T) {
		spec := projectconfig.DefaultSpec()
		valid := `schema_version = 2

[toolchain.go]
version = "1.26.5"
official_host = "go.dev"
official_path_prefix = "/dl/"

[toolchain.go.platforms."windows-amd64"]
archive_url = "https://go.dev/dl/go1.26.5.windows-amd64.zip"
archive_sha256 = "97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38"
`
		root := writeStrictRepository(t, spec, map[string]string{"config/toolchain.toml": valid})
		values, _, err := projectconfig.ValidateConcern(root, spec, "toolchain")
		if err != nil {
			t.Fatalf("quoted windows-amd64 table must validate: %v", err)
		}
		if values["toolchain.go.platforms.windows-amd64.archive_url"] == "" {
			t.Fatal("quoted platform table did not flatten to its exact registered key")
		}

		unregistered := strings.Replace(valid, `"windows-amd64"`, `"linux-amd64"`, 1)
		root = writeStrictRepository(t, spec, map[string]string{"config/toolchain.toml": unregistered})
		_, _, err = projectconfig.ValidateConcern(root, spec, "toolchain")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_UNKNOWN_KEY") {
			t.Fatalf("expected unregistered platform to fail closed, got %v", err)
		}
	})

	t.Run("unknown keys fail", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nmystery = \"x\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_UNKNOWN_KEY") {
			t.Fatalf("expected GOLC_CONFIG_UNKNOWN_KEY, got %v", err)
		}
	})

	t.Run("duplicate toml keys fail distinctly", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nlog_level = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_DUPLICATE_KEY") {
			t.Fatalf("expected GOLC_CONFIG_DUPLICATE_KEY, got %v", err)
		}
	})

	t.Run("invalid values fail", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"verbose\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_VALUE_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_VALUE_INVALID for closed-set violation, got %v", err)
		}

		root = writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   strictRuntimeConcern,
			"config/toolchain.toml": "schema_version = 2\n\n[toolchain.go]\nversion = \"..\\\\escape\"\n",
		})
		_, _, err = projectconfig.ValidateConcern(root, spec, "toolchain")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_VALUE_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_VALUE_INVALID for pattern violation, got %v", err)
		}

		root = writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = 3\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err = projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_VALUE_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_VALUE_INVALID for non-string value, got %v", err)
		}
	})

	t.Run("deprecated-only input warns with migration guidance", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nverbosity = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		values, warnings, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err != nil {
			t.Fatalf("deprecated-only input must not be fatal: %v", err)
		}
		if values["runtime.log_level"] != "debug" {
			t.Fatalf("deprecated value must apply to the replacement key, got %q", values["runtime.log_level"])
		}
		if len(warnings) != 1 {
			t.Fatalf("expected exactly one deprecation warning, got %v", warnings)
		}
		warning := warnings[0]
		if warning.Code != "CFG_DEPRECATED_KEY" {
			t.Fatalf("expected stable code CFG_DEPRECATED_KEY, got %q", warning.Code)
		}
		if warning.Key != "runtime.verbosity" {
			t.Fatalf("warning must name the deprecated key, got %q", warning.Key)
		}
		if warning.Origin != "config/runtime.toml" {
			t.Fatalf("warning origin must be the safe concern path, got %q", warning.Origin)
		}
		for _, needle := range []string{"runtime.log_level", "0.1.0", "0.2.0", "1.0.0", "rename runtime.verbosity"} {
			if !strings.Contains(warning.Message, needle) {
				t.Fatalf("warning message must contain %q, got %q", needle, warning.Message)
			}
		}
	})

	t.Run("deprecated plus replacement input collides", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nverbosity = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "CFG_DEPRECATED_COLLISION") {
			t.Fatalf("expected CFG_DEPRECATED_COLLISION, got %v", err)
		}
	})

	t.Run("duplicate authority in the registry fails", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[1].Keys["runtime.log_level"] = projectconfig.KeySpec{AllowedValues: []string{"info"}}
		err := projectconfig.ValidateAuthority(spec)
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_DUPLICATE_AUTHORITY") {
			t.Fatalf("expected GOLC_CONFIG_DUPLICATE_AUTHORITY, got %v", err)
		}
	})

	t.Run("a concern declaring another concern's key fails as duplicate authority", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\n\n[toolchain.go]\nversion = \"9.9.9\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_DUPLICATE_AUTHORITY") {
			t.Fatalf("expected GOLC_CONFIG_DUPLICATE_AUTHORITY, got %v", err)
		}
	})

	t.Run("unresolved references fail", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[0].Keys["runtime.go_version"] = projectconfig.KeySpec{Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)}
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\ngo_version = \"ref:toolchain.go.missing\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		if _, _, err := projectconfig.ValidateConcern(root, spec, "runtime"); err != nil {
			t.Fatalf("a pending cross-concern reference must validate alone: %v", err)
		}
		_, _, err := projectconfig.ValidateRepository(root, spec)
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_REF_UNRESOLVED") {
			t.Fatalf("expected GOLC_CONFIG_REF_UNRESOLVED, got %v", err)
		}
	})

	t.Run("cyclic references fail", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[0].Keys["runtime.go_version"] = projectconfig.KeySpec{Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)}
		spec.Concerns[1].Keys["toolchain.go.mirror"] = projectconfig.KeySpec{Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)}
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\ngo_version = \"ref:toolchain.go.mirror\"\n",
			"config/toolchain.toml": "schema_version = 2\n\n[toolchain.go]\nversion = \"1.26.5\"\nmirror = \"ref:runtime.go_version\"\n",
		})
		_, _, err := projectconfig.ValidateRepository(root, spec)
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_REF_CYCLE") {
			t.Fatalf("expected GOLC_CONFIG_REF_CYCLE, got %v", err)
		}
	})

	t.Run("a root index that hides or invents concerns fails", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   strictRuntimeConcern,
			"config/toolchain.toml": strictToolchainConcern,
		})
		hidden := "schema_version = 2\n\n[[concerns]]\nid = \"runtime\"\npath = \"config/runtime.toml\"\n"
		if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(hidden), 0o644); err != nil {
			t.Fatalf("rewrite root index: %v", err)
		}
		_, _, err := projectconfig.ValidateRepository(root, spec)
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_INDEX_MISMATCH") {
			t.Fatalf("expected GOLC_CONFIG_INDEX_MISMATCH for a hidden concern, got %v", err)
		}

		invented := hidden +
			"\n[[concerns]]\nid = \"toolchain\"\npath = \"config/toolchain.toml\"\n" +
			"\n[[concerns]]\nid = \"shadow\"\npath = \"config/runtime.toml\"\n"
		if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(invented), 0o644); err != nil {
			t.Fatalf("rewrite root index: %v", err)
		}
		_, _, err = projectconfig.ValidateRepository(root, spec)
		if err == nil || !strings.Contains(err.Error(), "GOLC_CONFIG_INDEX_MISMATCH") {
			t.Fatalf("expected GOLC_CONFIG_INDEX_MISMATCH for an invented concern, got %v", err)
		}
	})

	t.Run("malformed deprecation register entries fail", func(t *testing.T) {
		missingMessage := syntheticSpec()
		missingMessage.Deprecations[0].Message = ""
		if err := projectconfig.ValidateAuthority(missingMessage); err == nil ||
			!strings.Contains(err.Error(), "GOLC_CONFIG_DEPRECATION_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_DEPRECATION_INVALID for empty message, got %v", err)
		}

		unknownReplacement := syntheticSpec()
		unknownReplacement.Deprecations[0].ReplacementKey = "runtime.nonexistent"
		if err := projectconfig.ValidateAuthority(unknownReplacement); err == nil ||
			!strings.Contains(err.Error(), "GOLC_CONFIG_DEPRECATION_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_DEPRECATION_INVALID for unowned replacement, got %v", err)
		}

		ownedOldKey := syntheticSpec()
		ownedOldKey.Deprecations[0].OldKey = "runtime.log_level"
		if err := projectconfig.ValidateAuthority(ownedOldKey); err == nil ||
			!strings.Contains(err.Error(), "GOLC_CONFIG_DEPRECATION_INVALID") {
			t.Fatalf("expected GOLC_CONFIG_DEPRECATION_INVALID for owned old key, got %v", err)
		}
	})

	t.Run("linear concern declares names only and never credentials or remote ids", func(t *testing.T) {
		root := repositoryRoot(t)
		raw, err := os.ReadFile(filepath.Join(root, "config", "integrations", "linear.toml"))
		if err != nil {
			t.Fatalf("read linear concern: %v", err)
		}
		content := string(raw)

		uuidPattern := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
		if uuidPattern.MatchString(content) {
			t.Fatal("config/integrations/linear.toml must never contain an invented remote UUID")
		}
		if strings.Contains(content, "lin_api_") {
			t.Fatal("config/integrations/linear.toml must never contain a Linear API key")
		}

		values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "linear")
		if err != nil {
			t.Fatalf("linear concern must validate alone: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no linear warnings, got %v", warnings)
		}
		envNamePattern := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
		for _, key := range []string{"linear.env.api_key", "linear.env.team_id"} {
			name, declared := values[key]
			if !declared {
				t.Fatalf("linear concern must declare %s", key)
			}
			if !envNamePattern.MatchString(name) {
				t.Fatalf("%s must be an environment variable name, got %q", key, name)
			}
		}
	})
}
