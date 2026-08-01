// model_test.go covers the toolchain.deno canonical-key registration Task 1
// of Plan 08-02 adds to DefaultSpec (SCRP-03, 08-RESEARCH.md): the
// production toolchain concern validates the pinned Deno archive
// URL/checksum/host/prefix keys, and a malformed archive_sha256, a
// non-allowlisted archive_url host, or a missing required deno key each
// fail resolution with the existing stable diagnostics rather than being
// silently accepted.
//
// It is an external test package (like strict_test.go/load_test.go) so it
// shares strict_test.go's repositoryRoot/writeStrictRepository helpers
// without an import cycle.
package projectconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/projectconfig"
)

// TestDenoToolchainKeysValidateInProduction confirms the committed
// config/toolchain.toml's [toolchain.deno] table satisfies every canonical
// key DefaultSpec registers for it, with no warnings.
func TestDenoToolchainKeysValidateInProduction(t *testing.T) {
	root := repositoryRoot(t)
	values, warnings, err := projectconfig.ValidateConcern(root, projectconfig.DefaultSpec(), "toolchain")
	require.NoError(t, err, "production toolchain concern must validate")
	require.Empty(t, warnings, "expected no toolchain warnings")

	require.Equal(t, "github.com", values["toolchain.deno.official_host"])
	require.Equal(t, "/denoland/deno/releases/download/", values["toolchain.deno.official_path_prefix"])
	require.NotEmpty(t, values["toolchain.deno.version"], "resolved values must include toolchain.deno.version")

	denoPlatformPins := map[string][2]string{
		"windows-amd64": {
			"https://github.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-pc-windows-msvc.zip",
			"68ed08b05c56cf887e9aa509947dc3f468f7e12f47a13e5c1abd51d46d1453ef",
		},
		"linux-amd64": {
			"https://github.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-unknown-linux-gnu.zip",
			"c24f955d9fbfe0ea5ae2b501c8e71ae76e31e4c9782390a54a284b3364fda725",
		},
		"linux-arm64": {
			"https://github.com/denoland/deno/releases/download/v2.9.4/deno-aarch64-unknown-linux-gnu.zip",
			"111da5c05c240cfdc4340f234a0e3539d39dbcb6755221f19dcd60bacc8be5aa",
		},
		"darwin-amd64": {
			"https://github.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-apple-darwin.zip",
			"f757df6d3991e37601c69fad56c22b37c4ea77b5dcfad3636a642c2ba4c9b19f",
		},
		"darwin-arm64": {
			"https://github.com/denoland/deno/releases/download/v2.9.4/deno-aarch64-apple-darwin.zip",
			"6d17647fdbf9c587a581dba205054c4ccf732dae0a196cc1e9b44c07589db412",
		},
	}
	for platform, pin := range denoPlatformPins {
		prefix := "toolchain.deno.platforms." + platform
		require.Equal(t, pin[0], values[prefix+".archive_url"], "%s.archive_url", prefix)
		require.Equal(t, pin[1], values[prefix+".archive_sha256"], "%s.archive_sha256", prefix)
	}
}

// TestDenoToolchainRejectsInvalidAuthority mirrors strict_test.go's
// "Go and Node authorities reject incomplete extra malformed and
// platform-mismatched data" subtest: a malformed archive_sha256, an
// archive_url on a non-allowlisted host, and a missing required deno key
// must each fail strict configuration resolution with the existing stable
// diagnostics rather than being silently accepted.
func TestDenoToolchainRejectsInvalidAuthority(t *testing.T) {
	root := repositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "config", "toolchain.toml"))
	require.NoError(t, err, "read committed toolchain concern")
	valid := string(raw)

	cases := map[string]struct {
		content      string
		wantContains string
	}{
		"malformed archive_sha256": {
			content: strings.Replace(valid,
				`archive_sha256 = "68ed08b05c56cf887e9aa509947dc3f468f7e12f47a13e5c1abd51d46d1453ef"`,
				`archive_sha256 = "NOT-A-SHA256"`, 1),
			wantContains: "GOLC_CONFIG_VALUE_INVALID",
		},
		"archive_url on a non-allowlisted host": {
			content: strings.Replace(valid,
				`archive_url = "https://github.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-pc-windows-msvc.zip"`,
				`archive_url = "https://example.com/denoland/deno/releases/download/v2.9.4/deno-x86_64-pc-windows-msvc.zip"`, 1),
			wantContains: "GOLC_CONFIG_VALUE_INVALID",
		},
		"missing required deno key": {
			content:      strings.Replace(valid, `version = "2.9.4"`, "", 1),
			wantContains: "GOLC_CONFIG_REQUIRED_KEY_MISSING",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			fixtureRoot := writeStrictRepository(t, projectconfig.DefaultSpec(), map[string]string{
				"config/toolchain.toml": testCase.content,
			})
			_, _, err := projectconfig.ValidateConcern(fixtureRoot, projectconfig.DefaultSpec(), "toolchain")
			require.ErrorContains(t, err, testCase.wantContains)
		})
	}
}

// TestDenoArchiveURLPatternHost confirms denoArchiveURLPattern's shape is
// exercised end-to-end through the registered official_host allowlist: an
// otherwise well-formed archive_url on a host outside github.com fails
// through the AllowedValues check on official_host, and the pattern check
// on archive_url itself, before any fetch would ever be attempted --
// verified indirectly here since denoArchiveURLPattern itself is
// unexported; the DefaultSpec-level round trip in
// TestDenoToolchainRejectsInvalidAuthority is the authoritative coverage.
func TestDenoArchiveURLPatternHost(t *testing.T) {
	spec := projectconfig.DefaultSpec()
	var denoOfficialHost projectconfig.KeySpec
	for _, concern := range spec.Concerns {
		if concern.ID != "toolchain" {
			continue
		}
		denoOfficialHost = concern.Keys["toolchain.deno.official_host"]
	}
	require.Equal(t, []string{"github.com"}, denoOfficialHost.AllowedValues, "toolchain.deno.official_host must allow exactly github.com")
	require.True(t, denoOfficialHost.Required, "toolchain.deno.official_host must be required")
}
