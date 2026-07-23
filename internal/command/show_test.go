// show_test.go proves the "show open"/"show save"/"show save-as" route
// contract (05-02-PLAN.md Task 2): save writes and bumps Revision,
// save-as writes the destination and leaves the source's Revision
// untouched, open on a clean file reports no recovery offer, open on a
// file carrying an interrupted-session recovery point reports
// GOLC_SHOW_RECOVERY_AVAILABLE without mutating anything, --discard-
// recovery removes the offered point(s), and --accept-recovery promotes
// the chosen point into the working State.
//
// Simulating "a file with newer recovery points" requires writing directly
// into the .golc SQLite file's recovery_points table, bypassing
// show.Save's own transaction -- internal/show exposes no such seam on
// purpose (CONTEXT D-07: nothing auto-writes a recovery point outside
// Save's own commit). This mirrors chase_motion_test.go's own precedent
// (its docstring: the equivalent direct-write simulation "requires
// internal/show's unexported openStore" and so lives at the show-package
// level) -- since this file is `package command_test` and cannot reach
// that unexported seam, it opens the same .golc file directly through the
// "sqlite" database/sql driver (registered by internal/show's own blank
// import, reimported here so this file's dependency is explicit) and
// writes one recovery_points row using the exact schema internal/show/
// schema.go documents (show_meta/show_state/recovery_points).
package command_test

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// seedRecoveryPoint inserts one recovery_points row directly into the
// .golc SQLite file at root/showPath, simulating an interrupted session:
// a recovery point whose revision is newer than the file's current
// show_meta.revision.
func seedRecoveryPoint(t *testing.T, root, showPath string, revision int, state show.State) {
	t.Helper()
	payload, err := strictjson.CanonicalEncode(state)
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, showPath))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`,
		"2026-07-23T00:00:01Z", revision, payload); err != nil {
		t.Fatalf("seeding recovery point: %v", err)
	}
}

// seedOlderSchemaShow creates a valid .golc file at root/showPath (via the
// "pool create" route, so openStore's schema/tables/seed rows already
// exist), then directly overwrites show_meta.schema_version and
// show_state.blob via raw SQL to simulate a genuinely-saved historical show
// at schema_version 0 -- the only "older than current" value available
// while show.SchemaVersion stays pinned at 1 (mirrors
// internal/show/migrate_test.go's seedRawShow/fixturePayload helpers,
// reimplemented here at the raw-SQL level since this command_test-package
// file cannot reach those package-internal helpers, exactly like
// seedRecoveryPoint above). A full checkpoint(TRUNCATE) is forced so the
// main .golc file -- not a -wal sidecar -- reflects the seeded content
// before a caller captures "before" bytes for a byte-unchanged assertion.
func seedOlderSchemaShow(t *testing.T, registry *command.CommandRegistry, root, showPath string) show.State {
	t.Helper()
	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	clean, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (seed): %v", err)
	}
	older := clean
	older.SchemaVersion = 0
	payload, err := strictjson.CanonicalEncode(older)
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(root, showPath))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE show_meta SET schema_version = 0 WHERE id = 1`); err != nil {
		t.Fatalf("seeding schema_version: %v", err)
	}
	if _, err := db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, payload); err != nil {
		t.Fatalf("seeding blob: %v", err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	return older
}

// TestShowSaveRoute proves "show save" loads and re-saves a ShowState,
// bumping Revision (via show.Save's own recovery-point-write-in-the-same-
// transaction contract, CONTEXT D-04) and reporting the new revision.
func TestShowSaveRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	afterCreate, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after pool create: %v", err)
	}

	saveResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "save", "--show", showPath}})
	if saveResult.ExitCode != 0 {
		t.Fatalf("show save failed: exit=%d stderr=%s", saveResult.ExitCode, saveResult.Stderr)
	}
	if !strings.Contains(string(saveResult.Stdout), "GOLC_SHOW_SAVED") {
		t.Fatalf("expected GOLC_SHOW_SAVED in show save output, got %s", saveResult.Stdout)
	}

	afterSave, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after show save: %v", err)
	}
	if afterSave.Revision != afterCreate.Revision+1 {
		t.Fatalf("expected show save to bump Revision by exactly 1, got %d (was %d)", afterSave.Revision, afterCreate.Revision)
	}
	if len(afterSave.Pools) != 1 || afterSave.Pools[0].Name != "Wash Pool" {
		t.Fatalf("expected show save to preserve the existing pool, got %+v", afterSave.Pools)
	}
}

// TestShowSaveUsageMissingShowFlag proves "show save" rejects a missing
// --show with exit 2 and GOLC_SHOW_USAGE.
func TestShowSaveUsageMissingShowFlag(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	result := registry.Execute(command.Request{Root: t.TempDir(), Args: []string{"show", "save"}})
	if result.ExitCode != 2 || !strings.Contains(string(result.Stderr), "GOLC_SHOW_USAGE") {
		t.Fatalf("expected exit 2 GOLC_SHOW_USAGE for a missing --show, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestShowSaveAsRoute proves "show save-as" writes the destination file
// and leaves the source file's Revision untouched (source is only ever
// Loaded, never re-Saved).
func TestShowSaveAsRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	srcPath := "src.golc"
	destPath := "dest.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", srcPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	srcBefore, err := show.Load(root, srcPath)
	if err != nil {
		t.Fatalf("show.Load(src) before save-as: %v", err)
	}

	saveAsResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "save-as", "--show", srcPath, "--to", destPath}})
	if saveAsResult.ExitCode != 0 {
		t.Fatalf("show save-as failed: exit=%d stderr=%s", saveAsResult.ExitCode, saveAsResult.Stderr)
	}
	if !strings.Contains(string(saveAsResult.Stdout), "GOLC_SHOW_SAVED_AS") {
		t.Fatalf("expected GOLC_SHOW_SAVED_AS in show save-as output, got %s", saveAsResult.Stdout)
	}

	srcAfter, err := show.Load(root, srcPath)
	if err != nil {
		t.Fatalf("show.Load(src) after save-as: %v", err)
	}
	if srcAfter.Revision != srcBefore.Revision {
		t.Fatalf("expected show save-as to leave the source Revision untouched: before=%d after=%d", srcBefore.Revision, srcAfter.Revision)
	}

	dest, err := show.Load(root, destPath)
	if err != nil {
		t.Fatalf("show.Load(dest) after save-as: %v", err)
	}
	if len(dest.Pools) != 1 || dest.Pools[0].Name != "Wash Pool" {
		t.Fatalf("expected the destination to carry the source's pool, got %+v", dest.Pools)
	}
}

// TestShowOpenCleanFileReportsNoRecovery proves "show open" on a cleanly
// saved file never emits the recovery offer.
func TestShowOpenCleanFileReportsNoRecovery(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}

	openResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "open", "--show", showPath}})
	if openResult.ExitCode != 0 {
		t.Fatalf("show open failed: exit=%d stderr=%s", openResult.ExitCode, openResult.Stderr)
	}
	if strings.Contains(string(openResult.Stdout), "GOLC_SHOW_RECOVERY_AVAILABLE") {
		t.Fatalf("expected no recovery offer on a cleanly saved file, got %s", openResult.Stdout)
	}
	if !strings.Contains(string(openResult.Stdout), "GOLC_SHOW_OPENED") {
		t.Fatalf("expected GOLC_SHOW_OPENED in show open output, got %s", openResult.Stdout)
	}
}

// TestShowOpenReportsRecoveryOfferWithoutMutating proves "show open" on a
// file carrying a simulated interrupted-session recovery point reports
// GOLC_SHOW_RECOVERY_AVAILABLE and never mutates the file (CONTEXT D-07:
// offered, not applied) when neither --accept-recovery nor
// --discard-recovery is given.
func TestShowOpenReportsRecoveryOfferWithoutMutating(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	clean, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}

	interrupted := clean
	interrupted.Revision = clean.Revision + 1
	seedRecoveryPoint(t, root, showPath, clean.Revision+1, interrupted)

	openResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "open", "--show", showPath}})
	if openResult.ExitCode != 0 {
		t.Fatalf("show open failed: exit=%d stderr=%s", openResult.ExitCode, openResult.Stderr)
	}
	if !strings.Contains(string(openResult.Stdout), "GOLC_SHOW_RECOVERY_AVAILABLE") {
		t.Fatalf("expected a recovery offer, got %s", openResult.Stdout)
	}

	after, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after open: %v", err)
	}
	if after.Revision != clean.Revision {
		t.Fatalf("expected show open to leave Revision unchanged (offered, not applied): before=%d after=%d", clean.Revision, after.Revision)
	}

	points, err := show.DetectRecoveryPoints(root, showPath)
	if err != nil {
		t.Fatalf("show.DetectRecoveryPoints after open: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected show open to leave the offered recovery point untouched, got %d points", len(points))
	}
}

// TestShowOpenDiscardRecoveryRemovesPoints proves --discard-recovery
// removes the offered recovery point(s), reported via
// GOLC_SHOW_RECOVERY_DISCARDED, and a subsequent open no longer offers
// anything.
func TestShowOpenDiscardRecoveryRemovesPoints(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	clean, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	interrupted := clean
	interrupted.Revision = clean.Revision + 1
	seedRecoveryPoint(t, root, showPath, clean.Revision+1, interrupted)

	discardResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "open", "--show", showPath, "--discard-recovery"}})
	if discardResult.ExitCode != 0 {
		t.Fatalf("show open --discard-recovery failed: exit=%d stderr=%s", discardResult.ExitCode, discardResult.Stderr)
	}
	if !strings.Contains(string(discardResult.Stdout), "GOLC_SHOW_RECOVERY_DISCARDED") {
		t.Fatalf("expected GOLC_SHOW_RECOVERY_DISCARDED, got %s", discardResult.Stdout)
	}

	points, err := show.DetectRecoveryPoints(root, showPath)
	if err != nil {
		t.Fatalf("show.DetectRecoveryPoints after discard: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected no offered recovery points after discard, got %d", len(points))
	}

	reopenResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "open", "--show", showPath}})
	if reopenResult.ExitCode != 0 {
		t.Fatalf("show open (after discard) failed: exit=%d stderr=%s", reopenResult.ExitCode, reopenResult.Stderr)
	}
	if strings.Contains(string(reopenResult.Stdout), "GOLC_SHOW_RECOVERY_AVAILABLE") {
		t.Fatalf("expected no recovery offer after discard, got %s", reopenResult.Stdout)
	}
}

// TestShowOpenAcceptRecoveryPromotesChosenPoint proves --accept-recovery
// promotes the identified recovery point into the working State through
// show.AcceptRecoveryPoint's own Save path (Revision advances beyond the
// accepted blob's own stamped Revision).
func TestShowOpenAcceptRecoveryPromotesChosenPoint(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}
	clean, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}

	recovered := clean
	recovered.Pools = append([]pool.Pool(nil), clean.Pools...)
	recovered.Pools[0].Name = "Recovered Pool"
	recovered.Revision = clean.Revision + 1
	seedRecoveryPoint(t, root, showPath, clean.Revision+1, recovered)

	points, err := show.DetectRecoveryPoints(root, showPath)
	if err != nil {
		t.Fatalf("show.DetectRecoveryPoints: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected exactly 1 offered recovery point, got %d", len(points))
	}

	acceptResult := registry.Execute(command.Request{Root: root, Args: []string{
		"show", "open", "--show", showPath, "--accept-recovery", strconv.Itoa(points[0].ID),
	}})
	if acceptResult.ExitCode != 0 {
		t.Fatalf("show open --accept-recovery failed: exit=%d stderr=%s", acceptResult.ExitCode, acceptResult.Stderr)
	}
	if !strings.Contains(string(acceptResult.Stdout), "GOLC_SHOW_RECOVERY_ACCEPTED") {
		t.Fatalf("expected GOLC_SHOW_RECOVERY_ACCEPTED, got %s", acceptResult.Stdout)
	}

	after, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after accept: %v", err)
	}
	if len(after.Pools) != 1 || after.Pools[0].Name != "Recovered Pool" {
		t.Fatalf("expected the accepted recovery point's pool to become current, got %+v", after.Pools)
	}
	if after.Revision != clean.Revision+2 {
		t.Fatalf("expected Revision to advance via Save to %d, got %d", clean.Revision+2, after.Revision)
	}
}

// TestShowOpenAcceptRecoveryRejectsUnofferedID proves --accept-recovery
// refuses an id that is not among the currently offered recovery points
// (for example a stale or made-up id) with GOLC_SHOW_RECOVERY_NOT_FOUND,
// never silently applying anything.
func TestShowOpenAcceptRecoveryRejectsUnofferedID(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"

	createPool := registry.Execute(command.Request{Root: root, Args: []string{"pool", "create", "Wash Pool", "--show", showPath}})
	if createPool.ExitCode != 0 {
		t.Fatalf("pool create failed: exit=%d stderr=%s", createPool.ExitCode, createPool.Stderr)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"show", "open", "--show", showPath, "--accept-recovery", "999999",
	}})
	if result.ExitCode != 1 || !strings.Contains(string(result.Stderr), "GOLC_SHOW_RECOVERY_NOT_FOUND") {
		t.Fatalf("expected exit 1 GOLC_SHOW_RECOVERY_NOT_FOUND for an unoffered id, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestShowOpenUsageRejectsAcceptAndDiscardTogether proves
// --accept-recovery and --discard-recovery are mutually exclusive.
func TestShowOpenUsageRejectsAcceptAndDiscardTogether(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	result := registry.Execute(command.Request{Root: t.TempDir(), Args: []string{
		"show", "open", "--show", "show.golc", "--accept-recovery", "1", "--discard-recovery",
	}})
	if result.ExitCode != 2 || !strings.Contains(string(result.Stderr), "GOLC_SHOW_USAGE") {
		t.Fatalf("expected exit 2 GOLC_SHOW_USAGE for --accept-recovery and --discard-recovery together, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestShowOpenMigrationRequiresConfirm proves CONTEXT D-08's detection
// half: opening an older-schema .golc WITHOUT --confirm-migration reports
// GOLC_SHOW_MIGRATION_REQUIRED, exits non-zero, and leaves the file
// byte-unchanged -- no backup is taken and nothing is migrated until the
// caller explicitly confirms.
func TestShowOpenMigrationRequiresConfirm(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"
	seedOlderSchemaShow(t, registry, root, showPath)

	resolved := filepath.Join(root, showPath)
	before, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("reading fixture bytes: %v", err)
	}

	openResult := registry.Execute(command.Request{Root: root, Args: []string{"show", "open", "--show", showPath}})
	if openResult.ExitCode == 0 {
		t.Fatalf("expected show open to refuse an older-schema file without --confirm-migration, got exit 0: stdout=%s", openResult.Stdout)
	}
	if !strings.Contains(string(openResult.Stderr), "GOLC_SHOW_MIGRATION_REQUIRED") {
		t.Fatalf("expected GOLC_SHOW_MIGRATION_REQUIRED, got exit=%d stderr=%s", openResult.ExitCode, openResult.Stderr)
	}

	after, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("re-reading fixture bytes: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("show open without --confirm-migration rewrote the file; expected byte-for-byte unchanged")
	}

	// No backup should exist either -- detection alone never triggers
	// verifiedBackup.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".backup-") {
			t.Fatalf("expected no backup file from detection alone, found %s", entry.Name())
		}
	}
}

// TestShowOpenMigrationWithConfirm proves CONTEXT D-08's confirmed-migrate
// half: "show open --confirm-migration" against an older-schema .golc runs
// show.Migrate's verifiedBackup -> migrate-temp -> atomic-replace sequence,
// leaves a verifiable backup, and opens the migrated show at the current
// SchemaVersion. A synthetic identity migration is registered via
// show.RegisterTestMigration since the production migrations registry
// ships empty (only SchemaVersion=1 has ever existed).
func TestShowOpenMigrationWithConfirm(t *testing.T) {
	cleanup := show.RegisterTestMigration(0, func(blob []byte) ([]byte, error) { return blob, nil })
	t.Cleanup(cleanup)

	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	root := t.TempDir()
	showPath := "show.golc"
	seedOlderSchemaShow(t, registry, root, showPath)

	openResult := registry.Execute(command.Request{Root: root, Args: []string{
		"show", "open", "--show", showPath, "--confirm-migration",
	}})
	if openResult.ExitCode != 0 {
		t.Fatalf("show open --confirm-migration failed: exit=%d stderr=%s", openResult.ExitCode, openResult.Stderr)
	}
	if !strings.Contains(string(openResult.Stdout), "GOLC_SHOW_MIGRATED") {
		t.Fatalf("expected GOLC_SHOW_MIGRATED in show open --confirm-migration output, got %s", openResult.Stdout)
	}
	if !strings.Contains(string(openResult.Stdout), "GOLC_SHOW_OPENED") {
		t.Fatalf("expected GOLC_SHOW_OPENED after a successful migration, got %s", openResult.Stdout)
	}

	migrated, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load after migration: %v", err)
	}
	if migrated.SchemaVersion != show.SchemaVersion {
		t.Fatalf("expected schema_version %d after migration, got %d", show.SchemaVersion, migrated.SchemaVersion)
	}
	if len(migrated.Pools) != 1 || migrated.Pools[0].Name != "Wash Pool" {
		t.Fatalf("expected the migrated show to preserve the original pool, got %+v", migrated.Pools)
	}

	// A verified backup must exist on disk in root.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), showPath+".backup-") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a %s.backup-* file after a confirmed migration, found none in %v", showPath, entries)
	}
}
