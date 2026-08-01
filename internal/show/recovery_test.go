// recovery_test.go proves the read side of SHOW-04 (CONTEXT D-04-D-07):
// detection is a read-only query that never advances the on-disk revision
// or writes anything, discard performs a real DELETE (not merely hiding
// declined data), accept only ever promotes an explicit id through the
// existing Save path, and the 5-point cap holds after repeated saves. This
// file is `package show` (not show_test), matching store_test.go's own
// convention, so tests can call openStore directly to seed simulated
// interrupted-session recovery rows and read back raw table state that
// DetectRecoveryPoints's own allowlisted RecoveryPoint view intentionally
// does not expose (the blob).
package show

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/strictjson"
)

// insertRecoveryPoint seeds one recovery_points row directly, bypassing
// Save's transaction, so tests can simulate an interrupted session: a
// recovery point whose revision is newer than the last clean show_meta
// save -- the exact "process was killed mid-save, after the recovery
// point commit but with show_meta never reaching this revision through a
// later clean Save" shape DetectRecoveryPoints must surface.
func insertRecoveryPoint(t *testing.T, root, path, createdAt string, revision int, state State) {
	t.Helper()
	payload, err := strictjson.CanonicalEncode(state)
	require.NoError(t, err, "CanonicalEncode")
	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	defer db.Close()
	_, err = db.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`, createdAt, revision, payload)
	require.NoError(t, err, "inserting simulated recovery point")
}

// recoveryPointCount returns the number of rows currently in
// recovery_points, bypassing DetectRecoveryPoints's offered-revision
// filter, so tests can prove a discard performed a real DELETE rather than
// merely filtering a later offer.
func recoveryPointCount(t *testing.T, root, path string) int {
	t.Helper()
	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	defer db.Close()
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM recovery_points`).Scan(&count), "counting recovery_points")
	return count
}

// TestRecoveryPointPruning proves CONTEXT D-06: after 7 saves, exactly the
// newest 5 recovery rows remain, oldest pruned by id.
func TestRecoveryPointPruning(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "initial Save")
	loaded, err := Load(root, path)
	require.NoError(t, err, "Load")
	for i := 0; i < 6; i++ { // 1 initial Save + 6 more = 7 total saves, revisions 1..7
		require.NoError(t, Save(root, path, loaded), "Save %d", i)
		loaded, err = Load(root, path)
		require.NoError(t, err, "Load %d", i)
	}

	require.Equal(t, 5, recoveryPointCount(t, root, path), "expected exactly 5 recovery points after 7 saves")

	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	defer db.Close()
	rows, err := db.Query(`SELECT revision FROM recovery_points ORDER BY id ASC`)
	require.NoError(t, err, "querying recovery_points")
	defer rows.Close()
	var revisions []int
	for rows.Next() {
		var revision int
		require.NoError(t, rows.Scan(&revision), "scanning revision")
		revisions = append(revisions, revision)
	}
	require.NoError(t, rows.Err(), "iterating recovery_points")
	want := []int{3, 4, 5, 6, 7}
	require.Equal(t, want, revisions, "expected revisions (newest 5 of 7, oldest pruned first)")
}

// TestRecoveryReachableViaRealInterruptedSave proves CR-01's fix: an
// interrupted session is detectable through Save's own real code path
// (stageRecoveryPoint's commit, then a simulated crash before
// promoteState ever runs), not only through insertRecoveryPoint's raw-SQL
// bypass every other test in this file uses. This is the exact shape a
// genuine process kill between Save's two commits produces.
func TestRecoveryReachableViaRealInterruptedSave(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "initial Save")
	cleanRevision := onDiskRevision(t, root, path)

	interrupted := buildNonTrivialState(t)
	interrupted.Scenes[0].Name = "Interrupted Edit"
	interrupted.SchemaVersion = SchemaVersion
	interrupted.Revision = cleanRevision + 1
	payload, err := strictjson.CanonicalEncode(interrupted)
	require.NoError(t, err, "CanonicalEncode")

	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	// Simulate a process kill between Save's two commits: stage the
	// recovery point through the exact production code path (not raw SQL),
	// then close without ever calling promoteState.
	require.NoError(t, stageRecoveryPoint(db, "2026-07-23T00:00:01Z", cleanRevision+1, payload), "stageRecoveryPoint")
	require.NoError(t, db.Close(), "closing db after simulated interruption")

	require.Equal(t, cleanRevision, onDiskRevision(t, root, path), "show_meta.revision advanced despite promoteState never running")

	points, err := DetectRecoveryPoints(root, path)
	require.NoError(t, err, "DetectRecoveryPoints")
	require.Len(t, points, 1, "expected the interrupted save's recovery point to be offered, got %+v", points)
	require.Equal(t, cleanRevision+1, points[0].Revision, "expected offered revision")

	require.NoError(t, AcceptRecoveryPoint(root, path, points[0].ID), "AcceptRecoveryPoint")
	final, err := Load(root, path)
	require.NoError(t, err, "Load after accept")
	// AcceptRecoveryPoint persists through Save, which bumps Revision once
	// more beyond the recovery blob's own stamped Revision.
	require.Equal(t, cleanRevision+2, final.Revision, "expected Revision to advance via Save")
	require.NotEmpty(t, final.Scenes, "expected the recovered working State to equal the interrupted edit's scenes")
	require.Equal(t, "Interrupted Edit", final.Scenes[0].Name, "expected the recovered working State to equal the interrupted edit's scenes")
}

// TestRecoveryOfferedNotApplied proves CONTEXT D-07: DetectRecoveryPoints
// surfaces recovery rows newer than the last clean save, newest-first, and
// detection itself never writes -- the on-disk revision and the
// recovery_points row count are unchanged by calling it.
func TestRecoveryOfferedNotApplied(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	cleanRevision := onDiskRevision(t, root, path)

	interrupted1 := state
	interrupted1.SchemaVersion = SchemaVersion
	interrupted1.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, interrupted1)

	interrupted2 := state
	interrupted2.SchemaVersion = SchemaVersion
	interrupted2.Revision = cleanRevision + 2
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:02Z", cleanRevision+2, interrupted2)

	countBefore := recoveryPointCount(t, root, path)

	points, err := DetectRecoveryPoints(root, path)
	require.NoError(t, err, "DetectRecoveryPoints")
	require.Len(t, points, 2, "expected 2 offered recovery points, got %+v", points)
	require.Equal(t, cleanRevision+2, points[0].Revision, "expected newest-first order")
	require.Equal(t, cleanRevision+1, points[1].Revision, "expected newest-first order")

	require.Equal(t, cleanRevision, onDiskRevision(t, root, path), "DetectRecoveryPoints mutated the on-disk revision")
	require.Equal(t, countBefore, recoveryPointCount(t, root, path), "DetectRecoveryPoints changed the recovery_points row count")
}

// TestRecoveryDiscardDeletes proves CONTEXT D-07 / 05-RESEARCH.md Security
// row 5 (threat T-05-05): discarding removes the offered recovery rows
// with a real DELETE, not merely hiding them from a later offer.
func TestRecoveryDiscardDeletes(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	cleanRevision := onDiskRevision(t, root, path)

	interrupted := state
	interrupted.SchemaVersion = SchemaVersion
	interrupted.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, interrupted)

	require.NoError(t, DiscardRecoveryPoints(root, path), "DiscardRecoveryPoints")

	points, err := DetectRecoveryPoints(root, path)
	require.NoError(t, err, "DetectRecoveryPoints after discard")
	require.Empty(t, points, "expected no offered recovery points after discard")

	// Only the clean Save's own recovery_points row (written by Save's own
	// transaction, at cleanRevision) may remain: the discarded row must be
	// genuinely gone from the table, not merely excluded from the offer.
	require.Equal(t, 1, recoveryPointCount(t, root, path), "expected exactly 1 recovery point (the clean save's own) to remain after discard")
}

// TestRecoveryAcceptPersists proves AcceptRecoveryPoint promotes a chosen
// recovery blob into the working State through the existing Save path: the
// accepted content becomes the current working State and Revision advances
// via Save (never partially applied, never bypassing Save's own
// validate()).
func TestRecoveryAcceptPersists(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	cleanRevision := onDiskRevision(t, root, path)

	recovered := buildNonTrivialState(t)
	recovered.Scenes[0].Name = "Recovered Opener"
	recovered.SchemaVersion = SchemaVersion
	recovered.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, recovered)

	points, err := DetectRecoveryPoints(root, path)
	require.NoError(t, err, "DetectRecoveryPoints")
	require.Len(t, points, 1, "expected exactly 1 offered recovery point")

	require.NoError(t, AcceptRecoveryPoint(root, path, points[0].ID), "AcceptRecoveryPoint")

	loaded, err := Load(root, path)
	require.NoError(t, err, "Load after accept")
	// AcceptRecoveryPoint persists through Save, which bumps Revision once
	// more beyond the recovery blob's own stamped Revision.
	require.Equal(t, cleanRevision+2, loaded.Revision, "expected Revision to advance via Save")
	require.NotEmpty(t, loaded.Scenes, "expected the working State to equal the accepted recovery blob's scenes")
	require.Equal(t, "Recovered Opener", loaded.Scenes[0].Name, "expected the working State to equal the accepted recovery blob's scenes")
}

// TestRecoveryAcceptRejectsInvalidBlob proves an invalid recovery blob is
// refused with GOLC_SHOW_STATE_INVALID and never partially applied: the
// working State (and its on-disk revision) must stay exactly as the last
// clean Save left them.
func TestRecoveryAcceptRejectsInvalidBlob(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	cleanRevision := onDiskRevision(t, root, path)

	invalid := buildNonTrivialState(t)
	invalid.Deployments = append(invalid.Deployments, invalid.Deployments[0]) // duplicate deployment name -> validate() rejects
	invalid.SchemaVersion = SchemaVersion
	invalid.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, invalid)

	points, err := DetectRecoveryPoints(root, path)
	require.NoError(t, err, "DetectRecoveryPoints")
	require.Len(t, points, 1, "expected exactly 1 offered recovery point")

	err = AcceptRecoveryPoint(root, path, points[0].ID)
	require.Error(t, err, "expected AcceptRecoveryPoint to reject an invalid recovery blob")
	require.True(t, strings.HasPrefix(err.Error(), "GOLC_SHOW_STATE_INVALID"), "expected GOLC_SHOW_STATE_INVALID, got %v", err)

	require.Equal(t, cleanRevision, onDiskRevision(t, root, path), "expected on-disk revision to stay unchanged after a rejected accept")
}
