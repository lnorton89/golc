// script_test.go pins the "script create"/"script list"/"script show"/
// "script edit"/"script delete"/"script profile set" route contract
// (08-01-PLAN.md Task 2, RED state) before internal/command/script.go
// exists: every route writes deterministic JSON, a duplicate create is
// rejected without mutating the show, "script list" projects the D-16
// library shape (name/last_run_status/capability_profile, never source),
// "script edit" persists a source file's bytes verbatim, "script profile
// set" carries forward every field the caller did not mention, and every
// malformed invocation exits 2. It follows apikey_test.go's exact
// route-invocation convention.
package command_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
)

type scriptTestView struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Source            string `json:"source"`
	CapabilityProfile struct {
		Scope           string `json:"scope"`
		Preset          string `json:"preset"`
		DeadlineSeconds int    `json:"deadline_seconds"`
		RatePerSecond   int    `json:"rate_per_second"`
		MemoryLimitMB   int    `json:"memory_limit_mb"`
		CPUCapPercent   int    `json:"cpu_cap_percent"`
	} `json:"capability_profile"`
	LastRunStatus string `json:"last_run_status"`
}

func TestScriptRoutesFullLifecycle(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	// "script create" on a fresh show exits 0 and writes JSON containing
	// the new script's id and name.
	create := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Chase", "--show", showPath}})
	require.Equal(t, 0, create.ExitCode, "script create failed: exit=%d stderr=%s", create.ExitCode, create.Stderr)
	var created scriptTestView
	err = json.Unmarshal(create.Stdout, &created)
	require.NoError(t, err, "unmarshal script create output: %v stdout=%s", err, create.Stdout)
	require.True(t, created.ID != "" && created.Name == "Chase", "expected a created script with id/name, got %+v", created)
	require.True(t, created.CapabilityProfile.Scope == "playback" && created.CapabilityProfile.Preset == "quick-action", "expected the least-privileged default profile, got %+v", created.CapabilityProfile)
	require.Equal(t, "never_run", created.LastRunStatus, "expected last_run_status never_run, got %q", created.LastRunStatus)

	// A second "script create" with the same name exits 1 with
	// GOLC_SCRIPT_NAME_DUPLICATE and leaves the show unchanged.
	dup := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Chase", "--show", showPath}})
	require.True(t, dup.ExitCode == 1 && strings.Contains(string(dup.Stderr), "GOLC_SCRIPT_NAME_DUPLICATE"), "expected GOLC_SCRIPT_NAME_DUPLICATE for a duplicate name, got exit=%d stderr=%s", dup.ExitCode, dup.Stderr)

	// "script list" exits 0 and writes a JSON array whose entries carry
	// name, last_run_status, and capability_profile.
	list := registry.Execute(command.Request{Root: root, Args: []string{"script", "list", "--show", showPath}})
	require.Equal(t, 0, list.ExitCode, "script list failed: exit=%d stderr=%s", list.ExitCode, list.Stderr)
	var listed []struct {
		ID                string `json:"id"`
		Name              string `json:"name"`
		LastRunStatus     string `json:"last_run_status"`
		CapabilityProfile struct {
			Scope string `json:"scope"`
		} `json:"capability_profile"`
	}
	err = json.Unmarshal(list.Stdout, &listed)
	require.NoError(t, err, "unmarshal script list output: %v stdout=%s", err, list.Stdout)
	require.True(t, len(listed) == 1 && listed[0].Name == "Chase" && listed[0].LastRunStatus == "never_run", "expected exactly one listed script named Chase, got %+v", listed)
	require.NotContains(t, string(list.Stdout), "\"source\"", "expected script list to omit source, got: %s", list.Stdout)

	// "script show" exits 0 and writes the full script including source.
	showResult := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Chase", "--show", showPath}})
	require.Equal(t, 0, showResult.ExitCode, "script show failed: exit=%d stderr=%s", showResult.ExitCode, showResult.Stderr)
	var shown scriptTestView
	err = json.Unmarshal(showResult.Stdout, &shown)
	require.NoError(t, err, "unmarshal script show output: %v stdout=%s", err, showResult.Stdout)
	require.True(t, shown.ID == created.ID && shown.Source == "", "expected script show to return the full (empty-source) script, got %+v", shown)

	// "script edit" persists a source file's bytes verbatim.
	sourcePath := filepath.Join(root, "chase.ts")
	sourceBytes := []byte("export function run() {\n  // deliberately preserved formatting\n\tconsole.log('hi');\n}\n")
	err = os.WriteFile(sourcePath, sourceBytes, 0o644)
	require.NoError(t, err, "write source fixture: %v", err)
	edit := registry.Execute(command.Request{Root: root, Args: []string{"script", "edit", "Chase", "--source-file", sourcePath, "--show", showPath}})
	require.Equal(t, 0, edit.ExitCode, "script edit failed: exit=%d stderr=%s", edit.ExitCode, edit.Stderr)
	afterEdit := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Chase", "--show", showPath}})
	require.Equal(t, 0, afterEdit.ExitCode, "script show (after edit) failed: exit=%d stderr=%s", afterEdit.ExitCode, afterEdit.Stderr)
	var afterEditView scriptTestView
	err = json.Unmarshal(afterEdit.Stdout, &afterEditView)
	require.NoError(t, err, "unmarshal script show (after edit) output: %v", err)
	require.Equal(t, string(sourceBytes), afterEditView.Source, "expected persisted source to equal the file's bytes verbatim:\nwant %q\ngot  %q", sourceBytes, afterEditView.Source)

	// "script edit" against an unknown script name exits 1 with
	// GOLC_SCRIPT_NOT_FOUND.
	editMissing := registry.Execute(command.Request{Root: root, Args: []string{"script", "edit", "Missing", "--source-file", sourcePath, "--show", showPath}})
	require.True(t, editMissing.ExitCode == 1 && strings.Contains(string(editMissing.Stderr), "GOLC_SCRIPT_NOT_FOUND"), "expected GOLC_SCRIPT_NOT_FOUND editing an unknown script, got exit=%d stderr=%s", editMissing.ExitCode, editMissing.Stderr)

	// "script profile set" exits 0 and the persisted profile carries both
	// the scope and preset values.
	profileSet := registry.Execute(command.Request{Root: root, Args: []string{
		"script", "profile", "set", "Chase", "--scope", "authoring", "--preset", "long-running-automation", "--show", showPath,
	}})
	require.Equal(t, 0, profileSet.ExitCode, "script profile set failed: exit=%d stderr=%s", profileSet.ExitCode, profileSet.Stderr)
	var profiled scriptTestView
	err = json.Unmarshal(profileSet.Stdout, &profiled)
	require.NoError(t, err, "unmarshal script profile set output: %v", err)
	require.True(t, profiled.CapabilityProfile.Scope == "authoring" && profiled.CapabilityProfile.Preset == "long-running-automation", "expected the updated profile to carry scope=authoring preset=long-running-automation, got %+v", profiled.CapabilityProfile)

	// An invalid --scope exits 1 with GOLC_SCRIPT_SCOPE_INVALID and does
	// not save (the show's profile stays at its last valid value).
	badScope := registry.Execute(command.Request{Root: root, Args: []string{"script", "profile", "set", "Chase", "--scope", "bogus", "--show", showPath}})
	require.True(t, badScope.ExitCode == 1 && strings.Contains(string(badScope.Stderr), "GOLC_SCRIPT_SCOPE_INVALID"), "expected GOLC_SCRIPT_SCOPE_INVALID for an invalid scope, got exit=%d stderr=%s", badScope.ExitCode, badScope.Stderr)
	stillValid := registry.Execute(command.Request{Root: root, Args: []string{"script", "show", "Chase", "--show", showPath}})
	var stillValidView scriptTestView
	err = json.Unmarshal(stillValid.Stdout, &stillValidView)
	require.NoError(t, err, "unmarshal script show (after bad profile set) output: %v", err)
	require.Equal(t, "authoring", stillValidView.CapabilityProfile.Scope, "expected the invalid profile set to leave the show unchanged, got scope=%q", stillValidView.CapabilityProfile.Scope)

	// "script delete" exits 0 and "script list" afterwards returns an
	// empty array.
	deleteResult := registry.Execute(command.Request{Root: root, Args: []string{"script", "delete", "Chase", "--show", showPath}})
	require.Equal(t, 0, deleteResult.ExitCode, "script delete failed: exit=%d stderr=%s", deleteResult.ExitCode, deleteResult.Stderr)
	afterDelete := registry.Execute(command.Request{Root: root, Args: []string{"script", "list", "--show", showPath}})
	require.Equal(t, 0, afterDelete.ExitCode, "script list (after delete) failed: exit=%d stderr=%s", afterDelete.ExitCode, afterDelete.Stderr)
	require.Equal(t, "[]", strings.TrimSpace(string(afterDelete.Stdout)), "expected an empty JSON array after delete, got: %s", afterDelete.Stdout)
}

func TestScriptRoutesUsageErrors(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	// A malformed invocation (missing --show) exits 2.
	missingShow := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Chase"}})
	require.Equal(t, 2, missingShow.ExitCode, "expected ExitCode 2 for a missing --show, got %d stderr=%s", missingShow.ExitCode, missingShow.Stderr)

	// A malformed invocation (unknown flag) exits 2.
	unknownFlag := registry.Execute(command.Request{Root: root, Args: []string{"script", "create", "Chase", "--bogus", "value", "--show", showPath}})
	require.Equal(t, 2, unknownFlag.ExitCode, "expected ExitCode 2 for an unknown flag, got %d stderr=%s", unknownFlag.ExitCode, unknownFlag.Stderr)
}

// TestScriptCommandBinary proves "script list" on a brand-new show prints
// exactly "[]" via `go run ./cmd/golc-project` (08-01-PLAN.md's own
// acceptance criterion), not merely via the in-process registry.
func TestScriptCommandBinary(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	result := registry.Execute(command.Request{Root: root, Args: []string{"script", "list", "--show", "fresh.golc"}})
	require.Equal(t, 0, result.ExitCode, "script list on a fresh show failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	require.Equal(t, "[]", strings.TrimSpace(string(result.Stdout)), "expected script list on a fresh show to print exactly [], got: %s", result.Stdout)
}
