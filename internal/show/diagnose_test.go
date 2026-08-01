// diagnose_test.go pins the SHOW-06 Diagnose contract (05-04-PLAN.md Task
// 1): a healthy .golc reports empty FileLevelIssues and StructuralOK=true
// with the right SchemaVersion/Revision; a structurally-invalid-but-
// readable file reports StructuralOK=false with the validate error and no
// file-level issues; a file-level corrupted .golc reports non-"ok"
// integrity lines and is never reported healthy; and Diagnose completes
// well under a second at this app's small-show scale (05-RESEARCH.md
// Pitfall 4, confirmed empirically rather than assumed). package show (not
// show_test) so TestDiagnoseStructurallyInvalid can call openStore
// directly to write a tampered blob straight into show_state, bypassing
// Save's own validate() -- the same technique store_test.go's
// TestShowLoadRejectsOverScopeMotionCapability already established.
package show

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/strictjson"
)

// TestDiagnoseHealthyFile proves a freshly saved valid show returns empty
// FileLevelIssues and StructuralOK=true, with the right
// SchemaVersion/Revision.
func TestDiagnoseHealthyFile(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	loaded, err := Load(root, path)
	require.NoError(t, err, "Load")

	report, err := Diagnose(root, path)
	require.NoError(t, err, "Diagnose")
	require.Empty(t, report.FileLevelIssues, "expected no file-level issues for a healthy file")
	require.True(t, report.StructuralOK, "expected StructuralOK=true for a healthy file, got StructuralError=%q", report.StructuralError)
	require.Empty(t, report.StructuralError, "expected empty StructuralError for a healthy file")
	require.Equal(t, loaded.SchemaVersion, report.SchemaVersion)
	require.Equal(t, loaded.Revision, report.Revision)
}

// TestDiagnoseStructurallyInvalid proves a .golc whose blob decodes but
// fails validate() returns StructuralOK=false with the validate error,
// without crashing and without reporting a file-level issue for what is
// purely a structural problem.
func TestDiagnoseStructurallyInvalid(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	tampered := State{
		SchemaVersion: SchemaVersion,
		MotionPresets: []programming.MotionPreset{
			{
				ID:   uuid.Must(uuid.NewV7()),
				Name: "Tampered",
				Keyframes: []programming.MotionKeyframe{
					{Values: []programming.MotionKeyframeValue{{Capability: fixture.CapabilityColor, Value: 0.5}}},
				},
			},
		},
	}
	payload, err := strictjson.CanonicalEncode(tampered)
	require.NoError(t, err, "CanonicalEncode")

	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	_, err = db.Exec(`UPDATE show_meta SET schema_version = ?, revision = 1, checksum = ?, updated_at = '2026-01-01T00:00:00Z' WHERE id = 1`,
		SchemaVersion, sha256Hex(payload))
	if err != nil {
		db.Close()
		require.NoError(t, err, "seeding tampered show_meta")
	}
	_, err = db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, payload)
	if err != nil {
		db.Close()
		require.NoError(t, err, "seeding tampered show_state")
	}
	require.NoError(t, db.Close(), "closing seeded store")

	report, err := Diagnose(root, path)
	require.NoError(t, err, "Diagnose")
	require.False(t, report.StructuralOK, "expected StructuralOK=false for a structurally-invalid document, got a healthy report %+v", report)
	require.Contains(t, report.StructuralError, "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE", "expected StructuralError to surface the validate() failure")
	require.Empty(t, report.FileLevelIssues, "expected no file-level issues for a structurally-invalid-but-otherwise-healthy file")
	require.Equal(t, SchemaVersion, report.SchemaVersion, "expected SchemaVersion/Revision read from show_meta regardless of structural failure, got %+v", report)
	require.EqualValues(t, 1, report.Revision, "expected SchemaVersion/Revision read from show_meta regardless of structural failure, got %+v", report)
}

// TestDiagnoseFileCorruption proves the SHOW-06 corrupted-file probe: a
// .golc with injected page-level file corruption returns non-"ok"
// FileLevelIssues (or, in the rare case corruption is severe enough that
// the file cannot even be reopened, a non-nil error) -- Diagnose never
// silently reports a corrupted file as healthy. The state is padded with
// many pool members so the saved file spans several SQLite pages; only
// the file's tail quarter is overwritten with garbage, well past page 1's
// header and sqlite_master schema, so openStore's own PRAGMA/application_id
// door checks still succeed and PRAGMA integrity_check gets a chance to
// walk into (and report on) the corrupted region.
func TestDiagnoseFileCorruption(t *testing.T) {
	root := t.TempDir()
	path := "corrupt.golc"

	state := buildNonTrivialState(t)
	for i := 0; i < 500; i++ {
		member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
		require.NoError(t, err, "NewPoolMember")
		state.Pools[0].Members = append(state.Pools[0].Members, member)
	}

	require.NoError(t, Save(root, path, state), "Save")

	resolved := resolvePath(root, path)
	info, err := os.Stat(resolved)
	require.NoError(t, err, "stat")
	require.GreaterOrEqual(t, info.Size(), int64(8192), "expected a multi-page file to safely corrupt past the schema page")

	f, err := os.OpenFile(resolved, os.O_WRONLY, 0o644)
	require.NoError(t, err, "open for corruption")
	corruptFrom := info.Size() - info.Size()/4
	garbage := make([]byte, info.Size()-corruptFrom)
	for i := range garbage {
		garbage[i] = 0xFF
	}
	_, err = f.WriteAt(garbage, corruptFrom)
	if err != nil {
		f.Close()
		require.NoError(t, err, "WriteAt")
	}
	require.NoError(t, f.Close(), "close corrupted file")

	report, err := Diagnose(root, path)
	if err != nil {
		// A hard Diagnose failure (the file could not be reopened at all)
		// still proves the corruption was never silently reported healthy.
		return
	}
	require.False(t, len(report.FileLevelIssues) == 0 && report.StructuralOK,
		"expected injected corruption to surface as a file-level issue or a structural failure, got a healthy report %+v", report)
}

// TestDiagnoseCompletesUnderOneSecond confirms 05-RESEARCH.md Pitfall 4's
// scale assumption empirically for this application: a representative
// saved show's combined integrity_check + validate() completes well under
// a second, so show diagnose needs no async/streaming/cancellation
// machinery.
func TestDiagnoseCompletesUnderOneSecond(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)
	require.NoError(t, Save(root, path, state), "Save")

	start := time.Now()
	_, err := Diagnose(root, path)
	require.NoError(t, err, "Diagnose")
	require.LessOrEqual(t, time.Since(start), time.Second, "expected Diagnose to complete in under 1s at this app's scale")
}
