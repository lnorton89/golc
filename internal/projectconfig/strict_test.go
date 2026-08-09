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

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "resolve repository root")
	_, err = os.Stat(filepath.Join(root, "golc.project.toml"))
	require.NoError(t, err, "repository root %q has no golc.project.toml", root)
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(index.String()), 0o644), "write root index")
	for relative, content := range files {
		target := filepath.Join(root, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755), "mkdir for %s", relative)
		require.NoError(t, os.WriteFile(target, []byte(content), 0o644), "write %s", relative)
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

var goPlatformPins = map[string][2]string{
	"windows-amd64": {"https://go.dev/dl/go1.26.5.windows-amd64.zip", "97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38"},
	"linux-amd64":   {"https://go.dev/dl/go1.26.5.linux-amd64.tar.gz", "5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"},
	"linux-arm64":   {"https://go.dev/dl/go1.26.5.linux-arm64.tar.gz", "fe4789e92b1f33358680864bbe8704289e7bb5fc207d80623c308935bd696d49"},
	"darwin-amd64":  {"https://go.dev/dl/go1.26.5.darwin-amd64.tar.gz", "6231d8d3b8f5552ec6cbf6d685bdd5482e1e703214b120e89b3bf0d7bf1ef725"},
	"darwin-arm64":  {"https://go.dev/dl/go1.26.5.darwin-arm64.tar.gz", "efb87ff28af9a188d0536ef5d42e63dd52ba8263cd7344a993cc48dd11dedb6a"},
}

var nodePlatformPins = map[string][2]string{
	"windows-amd64": {"https://nodejs.org/dist/v24.18.0/node-v24.18.0-win-x64.zip", "0ae68406b42d7725661da979b1403ec9926da205c6770827f33aac9d8f26e821"},
	"linux-amd64":   {"https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-x64.tar.gz", "783130984963db7ba9cbd01089eaf2c2efb055c7c1693c943174b967b3050cb8"},
	"linux-arm64":   {"https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-arm64.tar.gz", "6b4484c2190274175df9aa8f28e2d758a819cb1c1fe6ab481e2f95b463ab8508"},
	"darwin-amd64":  {"https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-x64.tar.gz", "dfd0dbd3e721503434df7b7205e719f61b3a3a31b2bcf9729b8b91fea240f080"},
	"darwin-arm64":  {"https://nodejs.org/dist/v24.18.0/node-v24.18.0-darwin-arm64.tar.gz", "e1a97e14c99c803e96c7339403282ea05a499c32f8d83defe9ef5ec66f979ed1"},
}

// TestScopeConfigStrict is the exact quick-test marker for scope
// "config-strict" (test --quick --scope config-strict).
func TestScopeConfigStrict(t *testing.T) {
	t.Run("root index discovers exactly the seven concerns", func(t *testing.T) {
		root := repositoryRoot(t)
		index, err := projectconfig.LoadRootIndex(root)
		require.NoError(t, err, "LoadRootIndex failed")
		expected := map[string]string{
			"toolchain":            "config/toolchain.toml",
			"commands":             "config/commands.toml",
			"generation":           "config/generation.toml",
			"application_defaults": "config/application-defaults.toml",
			"runtime":              "config/runtime.toml",
			"linear":               "config/integrations/linear.toml",
			"api":                  "config/api.toml",
		}
		require.Len(t, index.Concerns, len(expected), "expected exactly %d indexed concerns", len(expected))
		for _, concern := range index.Concerns {
			path, known := expected[concern.ID]
			require.True(t, known, "unexpected indexed concern %q", concern.ID)
			require.Equal(t, path, concern.Path, "concern %q must index %q", concern.ID, path)
		}
	})

	t.Run("production repository validates with one authority per key and no warnings", func(t *testing.T) {
		root := repositoryRoot(t)
		spec := projectconfig.DefaultSpec()
		require.NoError(t, projectconfig.ValidateAuthority(spec), "ValidateAuthority failed")
		values, warnings, err := projectconfig.ValidateRepository(root, spec)
		require.NoError(t, err, "ValidateRepository failed")
		require.Empty(t, warnings, "expected no production warnings")
		require.NotEmpty(t, values["runtime.log_level"], "resolved values must include runtime.log_level")
		goVersion := values["toolchain.go.version"]
		require.NotEmpty(t, goVersion, "resolved values must include toolchain.go.version")
		require.Equal(t, goVersion, values["commands.go_version"], "commands.go_version must resolve through its reference to toolchain.go.version")
		require.Equal(t, ".tools/installs/golc_project", values["commands.cli_binary"], "commands.cli_binary must be a platform-neutral install root")

		// The commands concern must refer to the toolchain authority, never
		// repeat the pinned literal (D-05 single authority).
		commandsBytes, err := os.ReadFile(filepath.Join(root, "config", "commands.toml"))
		require.NoError(t, err, "read commands concern")
		require.Contains(t, string(commandsBytes), "ref:toolchain.go.version", "config/commands.toml must declare a typed reference to toolchain.go.version")
		require.NotContains(t, string(commandsBytes), goVersion, "config/commands.toml must not duplicate the pinned Go version literal %q", goVersion)
	})

	t.Run("every production concern validates alone", func(t *testing.T) {
		root := repositoryRoot(t)
		spec := projectconfig.DefaultSpec()
		require.Len(t, spec.Concerns, 7, "DefaultSpec must declare seven concerns")
		for _, concern := range spec.Concerns {
			_, _, err := projectconfig.ValidateConcern(root, spec, concern.ID)
			require.NoError(t, err, "concern %q must validate alone", concern.ID)
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
			"go_install.golangci-lint.module",
			"go_install.golangci-lint.version",
			"go_install.govulncheck.module",
			"go_install.govulncheck.version",
			"go_install.midicat.module",
			"go_install.midicat.version",
			"go_install.wails.module",
			"go_install.wails.version",
			"toolchain.deno.official_host",
			"toolchain.deno.official_path_prefix",
			"toolchain.deno.platforms.darwin-amd64.archive_sha256",
			"toolchain.deno.platforms.darwin-amd64.archive_url",
			"toolchain.deno.platforms.darwin-arm64.archive_sha256",
			"toolchain.deno.platforms.darwin-arm64.archive_url",
			"toolchain.deno.platforms.linux-amd64.archive_sha256",
			"toolchain.deno.platforms.linux-amd64.archive_url",
			"toolchain.deno.platforms.linux-arm64.archive_sha256",
			"toolchain.deno.platforms.linux-arm64.archive_url",
			"toolchain.deno.platforms.windows-amd64.archive_sha256",
			"toolchain.deno.platforms.windows-amd64.archive_url",
			"toolchain.deno.version",
			"toolchain.go.official_host",
			"toolchain.go.official_path_prefix",
			"toolchain.go.platforms.darwin-amd64.archive_sha256",
			"toolchain.go.platforms.darwin-amd64.archive_url",
			"toolchain.go.platforms.darwin-arm64.archive_sha256",
			"toolchain.go.platforms.darwin-arm64.archive_url",
			"toolchain.go.platforms.linux-amd64.archive_sha256",
			"toolchain.go.platforms.linux-amd64.archive_url",
			"toolchain.go.platforms.linux-arm64.archive_sha256",
			"toolchain.go.platforms.linux-arm64.archive_url",
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
			"toolchain.node.platforms.darwin-amd64.archive_sha256",
			"toolchain.node.platforms.darwin-amd64.archive_url",
			"toolchain.node.platforms.darwin-arm64.archive_sha256",
			"toolchain.node.platforms.darwin-arm64.archive_url",
			"toolchain.node.platforms.linux-amd64.archive_sha256",
			"toolchain.node.platforms.linux-amd64.archive_url",
			"toolchain.node.platforms.linux-arm64.archive_sha256",
			"toolchain.node.platforms.linux-arm64.archive_url",
			"toolchain.node.platforms.windows-amd64.archive_sha256",
			"toolchain.node.platforms.windows-amd64.archive_url",
			"toolchain.node.version",
		}
		require.Equal(t, strings.Join(want, "\n"), strings.Join(got, "\n"), "toolchain keys mismatch")
	})

	t.Run("production Go and Node authorities pin exact closed five-platform sets", func(t *testing.T) {
		root := repositoryRoot(t)
		values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "toolchain")
		require.NoError(t, err, "production toolchain concern must validate")
		require.Empty(t, warnings, "expected no toolchain warnings")
		for tool, pins := range map[string]map[string][2]string{"go": goPlatformPins, "node": nodePlatformPins} {
			for platform, pin := range pins {
				prefix := "toolchain." + tool + ".platforms." + platform
				require.Equal(t, pin[0], values[prefix+".archive_url"], "%s.archive_url", prefix)
				require.Equal(t, pin[1], values[prefix+".archive_sha256"], "%s.archive_sha256", prefix)
			}
		}
	})

	t.Run("Go and Node authorities reject incomplete extra malformed and platform-mismatched data", func(t *testing.T) {
		root := repositoryRoot(t)
		raw, err := os.ReadFile(filepath.Join(root, "config", "toolchain.toml"))
		require.NoError(t, err, "read committed toolchain concern")
		valid := string(raw)
		cases := map[string]string{
			"missing Go digest": strings.Replace(valid,
				`archive_sha256 = "efb87ff28af9a188d0536ef5d42e63dd52ba8263cd7344a993cc48dd11dedb6a"`, "", 1),
			"missing Node URL": strings.Replace(valid,
				`archive_url = "https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-arm64.tar.gz"`, "", 1),
			"extra Go platform": valid + `
[toolchain.go.platforms."windows-arm64"]
archive_url = "https://go.dev/dl/go1.26.5.windows-arm64.zip"
archive_sha256 = "97e6b2a833b6d89f9ff17d25419ac0a7e3b482a044e9ab18cdef834bd834fd38"
`,
			"Go wrong platform asset":   strings.Replace(valid, "go1.26.5.linux-arm64.tar.gz", "go1.26.5.linux-amd64.tar.gz", 1),
			"Node wrong platform asset": strings.Replace(valid, "node-v24.18.0-darwin-arm64.tar.gz", "node-v24.18.0-darwin-x64.tar.gz", 1),
			"malformed Node checksum": strings.Replace(valid,
				"e1a97e14c99c803e96c7339403282ea05a499c32f8d83defe9ef5ec66f979ed1", "NOT-A-SHA256", 1),
		}
		for name, content := range cases {
			t.Run(name, func(t *testing.T) {
				fixtureRoot := writeStrictRepository(t, projectconfig.DefaultSpec(), map[string]string{
					"config/toolchain.toml": content,
				})
				_, _, err := projectconfig.ValidateConcern(fixtureRoot, projectconfig.DefaultSpec(), "toolchain")
				require.Error(t, err, "invalid Go/Node authority unexpectedly validated")
			})
		}
	})

	t.Run("production mage authority pins exactly five official release archives", func(t *testing.T) {
		root := repositoryRoot(t)
		values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "toolchain")
		require.NoError(t, err, "production toolchain concern must validate")
		require.Empty(t, warnings, "expected no toolchain warnings")
		require.Equal(t, "1.17.2", values["toolchain.mage.version"])
		require.Equal(t, "github.com", values["toolchain.mage.official_host"])
		require.Equal(t, "/magefile/mage/releases/download/", values["toolchain.mage.official_path_prefix"])
		for platform, pin := range magePlatformPins {
			prefix := "toolchain.mage.platforms." + platform
			require.Equal(t, pin[0], values[prefix+".archive_url"], "%s.archive_url", prefix)
			require.Equal(t, pin[1], values[prefix+".archive_sha256"], "%s.archive_sha256", prefix)
		}
	})

	t.Run("mage authority rejects missing extra or malformed platform data", func(t *testing.T) {
		root := repositoryRoot(t)
		raw, err := os.ReadFile(filepath.Join(root, "config", "toolchain.toml"))
		require.NoError(t, err, "read committed toolchain concern")
		valid := string(raw)
		cases := map[string]string{
			"missing platform digest": strings.Replace(valid,
				`archive_sha256 = "5fd6f61170bb7584a4ca3ce4fd01137fe5a8edaf6c096d9f2ad30754d1d92797"`, "", 1),
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
				_, _, err := projectconfig.ValidateConcern(
					fixtureRoot, projectconfig.DefaultSpec(), "toolchain")
				require.Error(t, err, "invalid Mage authority unexpectedly validated")
			})
		}
	})

	t.Run("required keys are opt-in and preserve unrelated partial specs", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[1].Keys["toolchain.go.optional_mirror"] = projectconfig.KeySpec{
			Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`),
		}
		root := writeStrictRepository(t, spec, map[string]string{
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "toolchain")
		require.NoError(t, err, "an absent non-required key must remain valid")

		required := spec
		required.Concerns[1].Keys["toolchain.go.required_mirror"] = projectconfig.KeySpec{
			Pattern:  regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`),
			Required: true,
		}
		_, _, err = projectconfig.ValidateConcern(root, required, "toolchain")
		require.ErrorContains(t, err, "GOLC_CONFIG_REQUIRED_KEY_MISSING", "expected required key failure")
	})

	t.Run("quoted windows platform tables flatten exactly and unregistered platforms fail", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[1].Keys = map[string]projectconfig.KeySpec{
			"toolchain.go.version": {Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)},
			"toolchain.go.platforms.windows-amd64.archive_url": {
				Pattern: regexp.MustCompile(`^https://go\.dev/dl/[A-Za-z0-9.\-]+\.zip$`),
			},
			"toolchain.go.platforms.windows-amd64.archive_sha256": {
				Pattern: regexp.MustCompile(`^[0-9a-f]{64}$`),
			},
			"toolchain.go.official_host": {
				Pattern: regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9]+)+$`),
			},
			"toolchain.go.official_path_prefix": {
				Pattern: regexp.MustCompile(`^/[A-Za-z0-9/_-]*/$`),
			},
		}
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
		require.NoError(t, err, "quoted windows-amd64 table must validate")
		require.NotEmpty(t, values["toolchain.go.platforms.windows-amd64.archive_url"], "quoted platform table did not flatten to its exact registered key")

		unregistered := strings.Replace(valid, `"windows-amd64"`, `"linux-amd64"`, 1)
		root = writeStrictRepository(t, spec, map[string]string{"config/toolchain.toml": unregistered})
		_, _, err = projectconfig.ValidateConcern(root, spec, "toolchain")
		require.ErrorContains(t, err, "GOLC_CONFIG_UNKNOWN_KEY", "expected unregistered platform to fail closed")
	})

	t.Run("unknown keys fail", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nmystery = \"x\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_UNKNOWN_KEY")
	})

	t.Run("duplicate toml keys fail distinctly", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nlog_level = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_DUPLICATE_KEY")
	})

	t.Run("invalid values fail", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"verbose\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_VALUE_INVALID", "expected error for closed-set violation")

		root = writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   strictRuntimeConcern,
			"config/toolchain.toml": "schema_version = 2\n\n[toolchain.go]\nversion = \"..\\\\escape\"\n",
		})
		_, _, err = projectconfig.ValidateConcern(root, spec, "toolchain")
		require.ErrorContains(t, err, "GOLC_CONFIG_VALUE_INVALID", "expected error for pattern violation")

		root = writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = 3\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err = projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_VALUE_INVALID", "expected error for non-string value")
	})

	t.Run("deprecated-only input warns with migration guidance", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nverbosity = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		values, warnings, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.NoError(t, err, "deprecated-only input must not be fatal")
		require.Equal(t, "debug", values["runtime.log_level"], "deprecated value must apply to the replacement key")
		require.Len(t, warnings, 1, "expected exactly one deprecation warning, got %v", warnings)
		warning := warnings[0]
		require.Equal(t, "CFG_DEPRECATED_KEY", warning.Code, "expected stable code CFG_DEPRECATED_KEY")
		require.Equal(t, "runtime.verbosity", warning.Key, "warning must name the deprecated key")
		require.Equal(t, "config/runtime.toml", warning.Origin, "warning origin must be the safe concern path")
		for _, needle := range []string{"runtime.log_level", "0.1.0", "0.2.0", "1.0.0", "rename runtime.verbosity"} {
			require.Contains(t, warning.Message, needle, "warning message must contain %q", needle)
		}
	})

	t.Run("deprecated plus replacement input collides", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\nverbosity = \"debug\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "CFG_DEPRECATED_COLLISION")
	})

	t.Run("duplicate authority in the registry fails", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[1].Keys["runtime.log_level"] = projectconfig.KeySpec{AllowedValues: []string{"info"}}
		err := projectconfig.ValidateAuthority(spec)
		require.ErrorContains(t, err, "GOLC_CONFIG_DUPLICATE_AUTHORITY")
	})

	t.Run("a concern declaring another concern's key fails as duplicate authority", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\n\n[toolchain.go]\nversion = \"9.9.9\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_DUPLICATE_AUTHORITY")
	})

	t.Run("unresolved references fail", func(t *testing.T) {
		spec := syntheticSpec()
		spec.Concerns[0].Keys["runtime.go_version"] = projectconfig.KeySpec{Pattern: regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)}
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   "schema_version = 2\n\n[runtime]\nlog_level = \"info\"\ngo_version = \"ref:toolchain.go.missing\"\n",
			"config/toolchain.toml": strictToolchainConcern,
		})
		_, _, err := projectconfig.ValidateConcern(root, spec, "runtime")
		require.NoError(t, err, "a pending cross-concern reference must validate alone")
		_, _, err = projectconfig.ValidateRepository(root, spec)
		require.ErrorContains(t, err, "GOLC_CONFIG_REF_UNRESOLVED")
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
		require.ErrorContains(t, err, "GOLC_CONFIG_REF_CYCLE")
	})

	t.Run("a root index that hides or invents concerns fails", func(t *testing.T) {
		spec := syntheticSpec()
		root := writeStrictRepository(t, spec, map[string]string{
			"config/runtime.toml":   strictRuntimeConcern,
			"config/toolchain.toml": strictToolchainConcern,
		})
		hidden := "schema_version = 2\n\n[[concerns]]\nid = \"runtime\"\npath = \"config/runtime.toml\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(hidden), 0o644), "rewrite root index")
		_, _, err := projectconfig.ValidateRepository(root, spec)
		require.ErrorContains(t, err, "GOLC_CONFIG_INDEX_MISMATCH", "expected error for a hidden concern")

		invented := hidden +
			"\n[[concerns]]\nid = \"toolchain\"\npath = \"config/toolchain.toml\"\n" +
			"\n[[concerns]]\nid = \"shadow\"\npath = \"config/runtime.toml\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte(invented), 0o644), "rewrite root index")
		_, _, err = projectconfig.ValidateRepository(root, spec)
		require.ErrorContains(t, err, "GOLC_CONFIG_INDEX_MISMATCH", "expected error for an invented concern")
	})

	t.Run("malformed deprecation register entries fail", func(t *testing.T) {
		missingMessage := syntheticSpec()
		missingMessage.Deprecations[0].Message = ""
		err := projectconfig.ValidateAuthority(missingMessage)
		require.ErrorContains(t, err, "GOLC_CONFIG_DEPRECATION_INVALID", "expected error for empty message")

		unknownReplacement := syntheticSpec()
		unknownReplacement.Deprecations[0].ReplacementKey = "runtime.nonexistent"
		err = projectconfig.ValidateAuthority(unknownReplacement)
		require.ErrorContains(t, err, "GOLC_CONFIG_DEPRECATION_INVALID", "expected error for unowned replacement")

		ownedOldKey := syntheticSpec()
		ownedOldKey.Deprecations[0].OldKey = "runtime.log_level"
		err = projectconfig.ValidateAuthority(ownedOldKey)
		require.ErrorContains(t, err, "GOLC_CONFIG_DEPRECATION_INVALID", "expected error for owned old key")
	})

	t.Run("linear concern declares names only and never credentials or remote ids", func(t *testing.T) {
		root := repositoryRoot(t)
		raw, err := os.ReadFile(filepath.Join(root, "config", "integrations", "linear.toml"))
		require.NoError(t, err, "read linear concern")
		content := string(raw)

		uuidPattern := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
		require.False(t, uuidPattern.MatchString(content), "config/integrations/linear.toml must never contain an invented remote UUID")
		require.NotContains(t, content, "lin_api_", "config/integrations/linear.toml must never contain a Linear API key")

		values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "linear")
		require.NoError(t, err, "linear concern must validate alone")
		require.Empty(t, warnings, "expected no linear warnings")
		envNamePattern := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
		for _, key := range []string{"linear.env.api_key", "linear.env.team_id"} {
			name, declared := values[key]
			require.True(t, declared, "linear concern must declare %s", key)
			require.True(t, envNamePattern.MatchString(name), "%s must be an environment variable name, got %q", key, name)
		}
	})
}
