package projectconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/projectconfig"
)

const validRootIndex = `schema_version = 2

[[concerns]]
id = "toolchain"
path = "config/toolchain.toml"

[[concerns]]
id = "runtime"
path = "config/runtime.toml"
`

const validRuntimeConcern = `schema_version = 2

[runtime]
log_level = "info"
`

const validToolchainConcern = `schema_version = 2

[cache]
downloads = ".tools/cache/downloads"
`

func writeRepositoryFile(t *testing.T, root, relative, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755), "MkdirAll(%q) failed", relative)
	require.NoError(t, os.WriteFile(target, []byte(content), 0o644), "WriteFile(%q) failed", relative)
}

func newValidRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRepositoryFile(t, root, "golc.project.toml", validRootIndex)
	writeRepositoryFile(t, root, "config/runtime.toml", validRuntimeConcern)
	writeRepositoryFile(t, root, "config/toolchain.toml", validToolchainConcern)
	return root
}

func TestInspectConcernEmitsDeterministicSortedJSON(t *testing.T) {
	root := newValidRepository(t)

	first, err := projectconfig.InspectConcern(root, "runtime")
	require.NoError(t, err, "first InspectConcern failed")
	second, err := projectconfig.InspectConcern(root, "runtime")
	require.NoError(t, err, "second InspectConcern failed")
	require.Equal(t, first, second, "repeated inspection was not byte-identical")

	want := "{\"runtime\":{\"log_level\":\"info\"}}\n"
	require.Equal(t, want, string(first), "expected deterministic JSON")
}

func TestInspectConcernRejectsUnknownConcern(t *testing.T) {
	root := newValidRepository(t)

	_, err := projectconfig.InspectConcern(root, "nonexistent")
	require.ErrorContains(t, err, "GOLC_CONFIG_CONCERN_UNKNOWN", "expected stable GOLC_CONFIG_CONCERN_UNKNOWN diagnostic")
}

func TestLoadRootIndexRejectsUnknownKeys(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "golc.project.toml", validRootIndex+"\n[surprise]\nvalue = \"x\"\n")

	_, err := projectconfig.LoadRootIndex(root)
	require.ErrorContains(t, err, "GOLC_CONFIG_UNKNOWN_KEY", "expected stable GOLC_CONFIG_UNKNOWN_KEY diagnostic")
}

func TestLoadRootIndexRejectsDuplicateConcernIDs(t *testing.T) {
	root := newValidRepository(t)
	duplicated := validRootIndex + "\n[[concerns]]\nid = \"runtime\"\npath = \"config/runtime.toml\"\n"
	writeRepositoryFile(t, root, "golc.project.toml", duplicated)

	_, err := projectconfig.LoadRootIndex(root)
	require.ErrorContains(t, err, "GOLC_CONFIG_CONCERN_DUPLICATE", "expected stable GOLC_CONFIG_CONCERN_DUPLICATE diagnostic")
}

func TestLoadRootIndexRejectsWrongSchemaVersion(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "golc.project.toml", strings.Replace(validRootIndex, "schema_version = 2", "schema_version = 1", 1))

	_, err := projectconfig.LoadRootIndex(root)
	require.ErrorContains(t, err, "GOLC_CONFIG_SCHEMA_VERSION", "expected stable GOLC_CONFIG_SCHEMA_VERSION diagnostic")
}

func TestInspectConcernRequiresConcernSchemaVersion(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "config/runtime.toml", "[runtime]\nlog_level = \"info\"\n")

	_, err := projectconfig.InspectConcern(root, "runtime")
	require.ErrorContains(t, err, "GOLC_CONFIG_SCHEMA_VERSION", "expected stable GOLC_CONFIG_SCHEMA_VERSION diagnostic")
}

func TestConcernPathsCannotEscapeRepository(t *testing.T) {
	escapes := []string{
		"../outside.toml",
		"..\\outside.toml",
		"config/../../outside.toml",
		"config\\..\\..\\outside.toml",
		"/outside.toml",
		"\\outside.toml",
		"C:/outside.toml",
		"C:\\outside.toml",
		"config/./runtime.toml",
	}
	for _, escape := range escapes {
		root := newValidRepository(t)
		index := "schema_version = 2\n\n[[concerns]]\nid = \"runtime\"\npath = '" + escape + "'\n"
		writeRepositoryFile(t, root, "golc.project.toml", index)

		_, err := projectconfig.InspectConcern(root, "runtime")
		require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE", "expected concern path %q to be rejected", escape)
	}
}

func TestConcernPathsCannotEscapeThroughSymlinks(t *testing.T) {
	root := newValidRepository(t)
	outside := t.TempDir()
	writeRepositoryFile(t, outside, "secret.toml", "schema_version = 2\n\n[runtime]\nlog_level = \"debug\"\n")

	linkPath := filepath.Join(root, "config", "runtime.toml")
	require.NoError(t, os.Remove(linkPath), "removing runtime concern for symlink test failed")
	if err := os.Symlink(filepath.Join(outside, "secret.toml"), linkPath); err != nil {
		t.Skipf("symlink creation unavailable on this host: %v", err)
	}

	_, err := projectconfig.InspectConcern(root, "runtime")
	require.ErrorContains(t, err, "GOLC_CONFIG_PATH_ESCAPE", "expected a symlinked concern escaping the repository to be rejected")
}
