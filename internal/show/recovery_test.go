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
	"testing"

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
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}
	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO recovery_points (created_at, revision, blob) VALUES (?, ?, ?)`, createdAt, revision, payload); err != nil {
		t.Fatalf("inserting simulated recovery point: %v", err)
	}
}

// recoveryPointCount returns the number of rows currently in
// recovery_points, bypassing DetectRecoveryPoints's offered-revision
// filter, so tests can prove a discard performed a real DELETE rather than
// merely filtering a later offer.
func recoveryPointCount(t *testing.T, root, path string) int {
	t.Helper()
	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM recovery_points`).Scan(&count); err != nil {
		t.Fatalf("counting recovery_points: %v", err)
	}
	return count
}

// TestRecoveryPointPruning proves CONTEXT D-06: after 7 saves, exactly the
// newest 5 recovery rows remain, oldest pruned by id.
func TestRecoveryPointPruning(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	loaded, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i := 0; i < 6; i++ { // 1 initial Save + 6 more = 7 total saves, revisions 1..7
		if err := Save(root, path, loaded); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
		loaded, err = Load(root, path)
		if err != nil {
			t.Fatalf("Load %d: %v", i, err)
		}
	}

	if count := recoveryPointCount(t, root, path); count != 5 {
		t.Fatalf("expected exactly 5 recovery points after 7 saves, got %d", count)
	}

	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT revision FROM recovery_points ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("querying recovery_points: %v", err)
	}
	defer rows.Close()
	var revisions []int
	for rows.Next() {
		var revision int
		if err := rows.Scan(&revision); err != nil {
			t.Fatalf("scanning revision: %v", err)
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating recovery_points: %v", err)
	}
	want := []int{3, 4, 5, 6, 7}
	if len(revisions) != len(want) {
		t.Fatalf("expected revisions %v (newest 5 of 7), got %v", want, revisions)
	}
	for i, revision := range revisions {
		if revision != want[i] {
			t.Fatalf("expected revisions %v (oldest pruned first), got %v", want, revisions)
		}
	}
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

	if err := Save(root, path, state); err != nil {
		t.Fatalf("initial Save: %v", err)
	}
	cleanRevision := onDiskRevision(t, root, path)

	interrupted := buildNonTrivialState(t)
	interrupted.Scenes[0].Name = "Interrupted Edit"
	interrupted.SchemaVersion = SchemaVersion
	interrupted.Revision = cleanRevision + 1
	payload, err := strictjson.CanonicalEncode(interrupted)
	if err != nil {
		t.Fatalf("CanonicalEncode: %v", err)
	}

	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	// Simulate a process kill between Save's two commits: stage the
	// recovery point through the exact production code path (not raw SQL),
	// then close without ever calling promoteState.
	if err := stageRecoveryPoint(db, "2026-07-23T00:00:01Z", cleanRevision+1, payload); err != nil {
		t.Fatalf("stageRecoveryPoint: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("closing db after simulated interruption: %v", err)
	}

	if revisionAfter := onDiskRevision(t, root, path); revisionAfter != cleanRevision {
		t.Fatalf("show_meta.revision advanced despite promoteState never running: before=%d after=%d", cleanRevision, revisionAfter)
	}

	points, err := DetectRecoveryPoints(root, path)
	if err != nil {
		t.Fatalf("DetectRecoveryPoints: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected the interrupted save's recovery point to be offered, got %d points (%+v)", len(points), points)
	}
	if points[0].Revision != cleanRevision+1 {
		t.Fatalf("expected offered revision %d, got %d", cleanRevision+1, points[0].Revision)
	}

	if err := AcceptRecoveryPoint(root, path, points[0].ID); err != nil {
		t.Fatalf("AcceptRecoveryPoint: %v", err)
	}
	final, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load after accept: %v", err)
	}
	// AcceptRecoveryPoint persists through Save, which bumps Revision once
	// more beyond the recovery blob's own stamped Revision.
	if final.Revision != cleanRevision+2 {
		t.Fatalf("expected Revision to advance via Save to %d, got %d", cleanRevision+2, final.Revision)
	}
	if len(final.Scenes) == 0 || final.Scenes[0].Name != "Interrupted Edit" {
		t.Fatalf("expected the recovered working State to equal the interrupted edit's scenes, got %+v", final.Scenes)
	}
}

// TestRecoveryOfferedNotApplied proves CONTEXT D-07: DetectRecoveryPoints
// surfaces recovery rows newer than the last clean save, newest-first, and
// detection itself never writes -- the on-disk revision and the
// recovery_points row count are unchanged by calling it.
func TestRecoveryOfferedNotApplied(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
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
	if err != nil {
		t.Fatalf("DetectRecoveryPoints: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 offered recovery points, got %d (%+v)", len(points), points)
	}
	if points[0].Revision != cleanRevision+2 || points[1].Revision != cleanRevision+1 {
		t.Fatalf("expected newest-first order [%d,%d], got %+v", cleanRevision+2, cleanRevision+1, points)
	}

	revisionAfter := onDiskRevision(t, root, path)
	if revisionAfter != cleanRevision {
		t.Fatalf("DetectRecoveryPoints mutated the on-disk revision: before=%d after=%d", cleanRevision, revisionAfter)
	}
	countAfter := recoveryPointCount(t, root, path)
	if countAfter != countBefore {
		t.Fatalf("DetectRecoveryPoints changed the recovery_points row count: before=%d after=%d", countBefore, countAfter)
	}
}

// TestRecoveryDiscardDeletes proves CONTEXT D-07 / 05-RESEARCH.md Security
// row 5 (threat T-05-05): discarding removes the offered recovery rows
// with a real DELETE, not merely hiding them from a later offer.
func TestRecoveryDiscardDeletes(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cleanRevision := onDiskRevision(t, root, path)

	interrupted := state
	interrupted.SchemaVersion = SchemaVersion
	interrupted.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, interrupted)

	if err := DiscardRecoveryPoints(root, path); err != nil {
		t.Fatalf("DiscardRecoveryPoints: %v", err)
	}

	points, err := DetectRecoveryPoints(root, path)
	if err != nil {
		t.Fatalf("DetectRecoveryPoints after discard: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected no offered recovery points after discard, got %d", len(points))
	}

	// Only the clean Save's own recovery_points row (written by Save's own
	// transaction, at cleanRevision) may remain: the discarded row must be
	// genuinely gone from the table, not merely excluded from the offer.
	if count := recoveryPointCount(t, root, path); count != 1 {
		t.Fatalf("expected exactly 1 recovery point (the clean save's own) to remain after discard, got %d", count)
	}
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

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cleanRevision := onDiskRevision(t, root, path)

	recovered := buildNonTrivialState(t)
	recovered.Scenes[0].Name = "Recovered Opener"
	recovered.SchemaVersion = SchemaVersion
	recovered.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, recovered)

	points, err := DetectRecoveryPoints(root, path)
	if err != nil {
		t.Fatalf("DetectRecoveryPoints: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected exactly 1 offered recovery point, got %d", len(points))
	}

	if err := AcceptRecoveryPoint(root, path, points[0].ID); err != nil {
		t.Fatalf("AcceptRecoveryPoint: %v", err)
	}

	loaded, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load after accept: %v", err)
	}
	// AcceptRecoveryPoint persists through Save, which bumps Revision once
	// more beyond the recovery blob's own stamped Revision.
	if loaded.Revision != cleanRevision+2 {
		t.Fatalf("expected Revision to advance via Save to %d, got %d", cleanRevision+2, loaded.Revision)
	}
	if len(loaded.Scenes) == 0 || loaded.Scenes[0].Name != "Recovered Opener" {
		t.Fatalf("expected the working State to equal the accepted recovery blob's scenes, got %+v", loaded.Scenes)
	}
}

// TestRecoveryAcceptRejectsInvalidBlob proves an invalid recovery blob is
// refused with GOLC_SHOW_STATE_INVALID and never partially applied: the
// working State (and its on-disk revision) must stay exactly as the last
// clean Save left them.
func TestRecoveryAcceptRejectsInvalidBlob(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cleanRevision := onDiskRevision(t, root, path)

	invalid := buildNonTrivialState(t)
	invalid.Deployments = append(invalid.Deployments, invalid.Deployments[0]) // duplicate deployment name -> validate() rejects
	invalid.SchemaVersion = SchemaVersion
	invalid.Revision = cleanRevision + 1
	insertRecoveryPoint(t, root, path, "2026-07-23T00:00:01Z", cleanRevision+1, invalid)

	points, err := DetectRecoveryPoints(root, path)
	if err != nil {
		t.Fatalf("DetectRecoveryPoints: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected exactly 1 offered recovery point, got %d", len(points))
	}

	err = AcceptRecoveryPoint(root, path, points[0].ID)
	if err == nil {
		t.Fatalf("expected AcceptRecoveryPoint to reject an invalid recovery blob, got no error")
	}
	if got := err.Error(); len(got) == 0 || got[:len("GOLC_SHOW_STATE_INVALID")] != "GOLC_SHOW_STATE_INVALID" {
		t.Fatalf("expected GOLC_SHOW_STATE_INVALID, got %v", err)
	}

	revisionAfter := onDiskRevision(t, root, path)
	if revisionAfter != cleanRevision {
		t.Fatalf("expected on-disk revision to stay at %d after a rejected accept, got %d", cleanRevision, revisionAfter)
	}
}
