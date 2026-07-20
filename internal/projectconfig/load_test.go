package projectconfig_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/projectconfig"
)

const validRootIndex = `schema_version = 1

[[concerns]]
id = "toolchain"
path = "config/toolchain.toml"

[[concerns]]
id = "runtime"
path = "config/runtime.toml"
`

const validRuntimeConcern = `schema_version = 1

[runtime]
log_level = "info"
`

const validToolchainConcern = `schema_version = 1

[cache]
downloads = ".tools/cache/downloads"
`

func writeRepositoryFile(t *testing.T, root, relative, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", relative, err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", relative, err)
	}
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
	if err != nil {
		t.Fatalf("first InspectConcern failed: %v", err)
	}
	second, err := projectconfig.InspectConcern(root, "runtime")
	if err != nil {
		t.Fatalf("second InspectConcern failed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("repeated inspection was not byte-identical:\nfirst:  %q\nsecond: %q", first, second)
	}

	want := "{\"runtime\":{\"log_level\":\"info\"}}\n"
	if string(first) != want {
		t.Fatalf("expected deterministic JSON %q, got %q", want, first)
	}
}

func TestInspectConcernRejectsUnknownConcern(t *testing.T) {
	root := newValidRepository(t)

	_, err := projectconfig.InspectConcern(root, "nonexistent")
	if err == nil {
		t.Fatal("expected an unknown concern id to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_CONCERN_UNKNOWN") {
		t.Fatalf("expected stable GOLC_CONFIG_CONCERN_UNKNOWN diagnostic, got %q", err.Error())
	}
}

func TestLoadRootIndexRejectsUnknownKeys(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "golc.project.toml", validRootIndex+"\n[surprise]\nvalue = \"x\"\n")

	_, err := projectconfig.LoadRootIndex(root)
	if err == nil {
		t.Fatal("expected unknown root-index keys to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_UNKNOWN_KEY") {
		t.Fatalf("expected stable GOLC_CONFIG_UNKNOWN_KEY diagnostic, got %q", err.Error())
	}
}

func TestLoadRootIndexRejectsDuplicateConcernIDs(t *testing.T) {
	root := newValidRepository(t)
	duplicated := validRootIndex + "\n[[concerns]]\nid = \"runtime\"\npath = \"config/runtime.toml\"\n"
	writeRepositoryFile(t, root, "golc.project.toml", duplicated)

	_, err := projectconfig.LoadRootIndex(root)
	if err == nil {
		t.Fatal("expected duplicate concern ids to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_CONCERN_DUPLICATE") {
		t.Fatalf("expected stable GOLC_CONFIG_CONCERN_DUPLICATE diagnostic, got %q", err.Error())
	}
}

func TestLoadRootIndexRejectsWrongSchemaVersion(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "golc.project.toml", strings.Replace(validRootIndex, "schema_version = 1", "schema_version = 2", 1))

	_, err := projectconfig.LoadRootIndex(root)
	if err == nil {
		t.Fatal("expected an unsupported schema_version to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_SCHEMA_VERSION") {
		t.Fatalf("expected stable GOLC_CONFIG_SCHEMA_VERSION diagnostic, got %q", err.Error())
	}
}

func TestInspectConcernRequiresConcernSchemaVersion(t *testing.T) {
	root := newValidRepository(t)
	writeRepositoryFile(t, root, "config/runtime.toml", "[runtime]\nlog_level = \"info\"\n")

	_, err := projectconfig.InspectConcern(root, "runtime")
	if err == nil {
		t.Fatal("expected a concern without schema_version to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_SCHEMA_VERSION") {
		t.Fatalf("expected stable GOLC_CONFIG_SCHEMA_VERSION diagnostic, got %q", err.Error())
	}
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
		index := "schema_version = 1\n\n[[concerns]]\nid = \"runtime\"\npath = '" + escape + "'\n"
		writeRepositoryFile(t, root, "golc.project.toml", index)

		_, err := projectconfig.InspectConcern(root, "runtime")
		if err == nil {
			t.Fatalf("expected concern path %q to be rejected", escape)
		}
		if !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
			t.Fatalf("expected stable GOLC_CONFIG_PATH_ESCAPE diagnostic for %q, got %q", escape, err.Error())
		}
	}
}

func TestConcernPathsCannotEscapeThroughSymlinks(t *testing.T) {
	root := newValidRepository(t)
	outside := t.TempDir()
	writeRepositoryFile(t, outside, "secret.toml", "schema_version = 1\n\n[runtime]\nlog_level = \"debug\"\n")

	linkPath := filepath.Join(root, "config", "runtime.toml")
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("removing runtime concern for symlink test failed: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.toml"), linkPath); err != nil {
		t.Skipf("symlink creation unavailable on this host: %v", err)
	}

	_, err := projectconfig.InspectConcern(root, "runtime")
	if err == nil {
		t.Fatal("expected a symlinked concern escaping the repository to be rejected")
	}
	if !strings.Contains(err.Error(), "GOLC_CONFIG_PATH_ESCAPE") {
		t.Fatalf("expected stable GOLC_CONFIG_PATH_ESCAPE diagnostic, got %q", err.Error())
	}
}
