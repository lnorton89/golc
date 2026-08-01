// show_diagnose_test.go pins the "show diagnose"/"show export" route
// contract (05-04-PLAN.md Task 2): export prints the FULL canonical
// document byte-identical to strictjson.CanonicalEncode(state) and
// round-trips back into a fresh .golc via the Save/Load path (D-13);
// diagnose exits 0 for a healthy file and 1 (with the issues printed) for
// a corrupted one; and both routes tolerate a newer-than-supported
// schema_version read-only, never rewriting the file (D-10). Follows
// pooldeploy_test.go's route-invocation convention: build the default
// registry, Execute a Request, assert Result.ExitCode/Stdout/Stderr.
package command_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// TestShowExportMatchesCanonicalEncode proves D-13: "show export"'s output
// is byte-identical to strictjson.CanonicalEncode(state) for a loaded
// show -- the full canonical document, not "show inspect"'s allowlisted
// projection -- and re-importing it (decode, then Save into a fresh
// .golc, then Load) reproduces the same domain fields.
func TestShowExportMatchesCanonicalEncode(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--requires", "intensity,color", "--show", showPath}})
	require.Equal(t, 0, createPool.ExitCode, "pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)

	exportResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "export", "--show", showPath}})
	require.Equal(t, 0, exportResult.ExitCode, "show export failed: exit=%d stderr=%s", exportResult.ExitCode, exportResult.Stderr)

	loaded, err := show.LoadForRead(root, showPath)
	require.NoError(t, err, "show.LoadForRead: %v", err)
	want, err := strictjson.CanonicalEncode(loaded)
	require.NoError(t, err, "strictjson.CanonicalEncode: %v", err)
	require.Equal(t, string(want), string(exportResult.Stdout), "show export output did not match CanonicalEncode(state):\ngot:  %s\nwant: %s", exportResult.Stdout, want)

	// Round-trip: decode the exported document, Save it into a fresh
	// .golc, and Load it back -- the domain fields must survive
	// byte-identically (D-13's "naturally round-trippable" requirement).
	var reimported show.State
	err = strictjson.DecodeStrict(exportResult.Stdout, &reimported)
	require.NoError(t, err, "decode exported document: %v", err)
	reimportedPath := "reimported.golc"
	err = show.Save(root, reimportedPath, reimported)
	require.NoError(t, err, "Save reimported state: %v", err)
	reloaded, err := show.Load(root, reimportedPath)
	require.NoError(t, err, "Load reimported state: %v", err)
	reimported.SchemaVersion, reimported.Revision = 0, 0
	reloaded.SchemaVersion, reloaded.Revision = 0, 0
	require.Equal(t, reimported, reloaded, "exported document did not round-trip through Save/Load:\nwant %+v\ngot  %+v", reimported, reloaded)
}

// TestShowDiagnoseHealthyExitZero proves "show diagnose" exits 0 for a
// healthy file and prints a report with structural_ok=true.
func TestShowDiagnoseHealthyExitZero(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	require.Equal(t, 0, createPool.ExitCode, "pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)

	diagnoseResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "diagnose", "--show", showPath}})
	require.Equal(t, 0, diagnoseResult.ExitCode, "expected show diagnose to exit 0 for a healthy file: exit=%d stdout=%s stderr=%s", diagnoseResult.ExitCode, diagnoseResult.Stdout, diagnoseResult.Stderr)
	var report struct {
		StructuralOK bool `json:"structural_ok"`
	}
	err = json.Unmarshal(diagnoseResult.Stdout, &report)
	require.NoError(t, err, "unmarshal show diagnose output: %v", err)
	require.True(t, report.StructuralOK, "expected structural_ok=true for a healthy file, got %s", diagnoseResult.Stdout)
}

// TestShowDiagnoseCorruptExitOne proves "show diagnose" exits 1 and
// prints the issues found for a corrupted .golc file. The state is padded
// with many pool members so the saved file spans several SQLite pages;
// only the file's tail quarter is overwritten with garbage, well past
// page 1's header and sqlite_master schema, so the file remains openable
// while PRAGMA integrity_check still detects the corrupted region (the
// same technique internal/show/diagnose_test.go's
// TestDiagnoseFileCorruption uses).
func TestShowDiagnoseCorruptExitOne(t *testing.T) {
	root := t.TempDir()
	showPath := "corrupt.golc"

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool: %v", err)
	for i := 0; i < 500; i++ {
		member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
		require.NoError(t, err, "NewPoolMember: %v", err)
		p.Members = append(p.Members, member)
	}
	err = show.Save(root, showPath, show.State{Pools: []pool.Pool{p}})
	require.NoError(t, err, "Save: %v", err)

	resolved := filepath.Join(root, showPath)
	info, err := os.Stat(resolved)
	require.NoError(t, err, "stat: %v", err)
	require.GreaterOrEqual(t, info.Size(), int64(8192), "expected a multi-page file to safely corrupt past the schema page, got %d bytes", info.Size())
	f, err := os.OpenFile(resolved, os.O_WRONLY, 0o644)
	require.NoError(t, err, "open for corruption: %v", err)
	corruptFrom := info.Size() - info.Size()/4
	garbage := make([]byte, info.Size()-corruptFrom)
	for i := range garbage {
		garbage[i] = 0xFF
	}
	if _, err := f.WriteAt(garbage, corruptFrom); err != nil {
		f.Close()
		require.NoError(t, err, "WriteAt: %v", err)
	}
	err = f.Close()
	require.NoError(t, err, "close corrupted file: %v", err)

	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)
	diagnoseResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "diagnose", "--show", showPath}})
	require.Equal(t, 1, diagnoseResult.ExitCode, "expected show diagnose to exit 1 for a corrupted file, got exit=%d stdout=%s stderr=%s", diagnoseResult.ExitCode, diagnoseResult.Stdout, diagnoseResult.Stderr)
	require.NotEmpty(t, diagnoseResult.Stdout, "expected show diagnose to print the issues found for a corrupted file")
}

// TestShowExportTooNewReadOnly proves D-10: "show export" and "show
// diagnose" both succeed read-only against a .golc whose schema_version is
// newer than this build supports, and neither rewrites the file.
func TestShowExportTooNewReadOnly(t *testing.T) {
	root := t.TempDir()
	showPath := "future.golc"

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool: %v", err)
	err = show.Save(root, showPath, show.State{Pools: []pool.Pool{p}})
	require.NoError(t, err, "Save: %v", err)

	resolved := filepath.Join(root, showPath)
	// The "sqlite" driver is already registered process-wide by
	// internal/show's blank import of modernc.org/sqlite (transitively
	// linked into this test binary via internal/command -> internal/show),
	// so this test can open the same file directly without its own driver
	// import.
	db, err := sql.Open("sqlite", resolved)
	require.NoError(t, err, "sql.Open: %v", err)
	if _, err := db.Exec(`UPDATE show_meta SET schema_version = ? WHERE id = 1`, show.SchemaVersion+1); err != nil {
		db.Close()
		require.NoError(t, err, "bump schema_version: %v", err)
	}
	// Force a full checkpoint so the main .golc file (not a -wal sidecar)
	// reflects the bumped schema_version before "before" bytes are
	// captured -- otherwise a later read-only passive checkpoint from
	// openStore could change the main file's bytes for reasons unrelated
	// to show export/diagnose actually rewriting anything.
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		db.Close()
		require.NoError(t, err, "checkpoint: %v", err)
	}
	err = db.Close()
	require.NoError(t, err, "close: %v", err)

	before, err := os.ReadFile(resolved)
	require.NoError(t, err, "read before: %v", err)

	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err, "NewDefaultCommandRegistry: %v", err)

	exportResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "export", "--show", showPath}})
	require.Equal(t, 0, exportResult.ExitCode, "expected show export to succeed read-only for a newer-than-supported file: exit=%d stderr=%s", exportResult.ExitCode, exportResult.Stderr)
	require.NotEmpty(t, exportResult.Stdout, "expected show export to print the newer document's content")

	diagnoseResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "diagnose", "--show", showPath}})
	require.Equal(t, 0, diagnoseResult.ExitCode, "expected show diagnose to succeed read-only for a newer-than-supported file: exit=%d stdout=%s stderr=%s", diagnoseResult.ExitCode, diagnoseResult.Stdout, diagnoseResult.Stderr)

	after, err := os.ReadFile(resolved)
	require.NoError(t, err, "read after: %v", err)
	require.Equal(t, before, after, "expected read-only show export/diagnose to never rewrite a newer-than-supported .golc file")
}
