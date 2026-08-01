// store_test.go pins the SHOW-01/SHOW-02/SHOW-03 SQLite-backed store
// contract (05-01-PLAN.md Task 1, RED state) before internal/show/store.go
// and internal/show/schema.go exist: a non-trivial State round-trips
// byte-identically through Save/Load, repeated Saves bump Revision exactly
// once each with no entity duplication, repeated Loads never mutate the
// on-disk file (never bump Revision, never write a recovery point), and
// internal/show never imports internal/playback (SHOW-02's "storage never
// enters the playback timing path" invariant). This file is `package show`
// (not show_test) so onDiskRevision can call the not-yet-implemented
// openStore directly -- until Task 2 lands schema.go/store.go, that
// reference makes this whole package fail to compile, which is the
// intended RED state this task proves.
package show

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/strictjson"
)

// buildNonTrivialState constructs a State carrying at least one pool (with
// one member), one deployment with one concretely-addressed instance, one
// group referencing that pool member, a scene, and a non-zero Tempo.BPM --
// the minimum shape the RED-state contract in 05-01-PLAN.md Task 1 asks
// for.
func buildNonTrivialState(t *testing.T) State {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7 (instance)")
	universe, address, err := deployment.NextFreeAddress(nil, 3)
	require.NoError(t, err, "NextFreeAddress")
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "3ch",
		Universe:     universe,
		Address:      address,
	})

	groupID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7 (group)")
	group := pool.Group{
		ID:         groupID,
		Name:       "Front Wash",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
	}

	sc, err := scene.NewScene("Opener", 4)
	require.NoError(t, err, "NewScene")

	return State{
		Pools:       []pool.Pool{p},
		Deployments: []deployment.Deployment{d},
		Groups:      []pool.Group{group},
		Scenes:      []scene.Scene{sc},
		Tempo:       Tempo{BPM: 120},
	}
}

// assertDomainEqual compares every domain field of want/got, ignoring
// SchemaVersion/Revision (which Save always stamps/bumps and are asserted
// separately by each test).
func assertDomainEqual(t *testing.T, want, got State) {
	t.Helper()
	want.SchemaVersion, want.Revision = 0, 0
	got.SchemaVersion, got.Revision = 0, 0
	require.Equal(t, want, got, "domain fields did not round-trip")
}

// onDiskRevision opens the store directly (bypassing Load's validation) to
// read show_meta.revision, so tests can prove a Load never wrote a new
// revision -- calling openStore directly is the strongest possible proof,
// stronger than re-Loading and comparing State.Revision alone.
func onDiskRevision(t *testing.T, root, path string) int {
	t.Helper()
	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	defer db.Close()
	var revision int
	require.NoError(t, db.QueryRow(`SELECT revision FROM show_meta WHERE id = 1`).Scan(&revision), "querying show_meta.revision")
	return revision
}

// TestShowStoreRoundTrip proves SHOW-01: a complete State saves to and
// loads from one SQLite .golc file with byte-identical domain fields, and
// Revision increments exactly once per Save.
func TestShowStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")
	loaded, err := Load(root, path)
	require.NoError(t, err, "Load")
	require.Equal(t, state.Revision+1, loaded.Revision, "expected Revision to bump by 1")
	assertDomainEqual(t, state, loaded)

	// Save again against the loaded state; Revision must bump monotonically.
	require.NoError(t, Save(root, path, loaded), "second Save")
	reloaded, err := Load(root, path)
	require.NoError(t, err, "reload")
	require.Equal(t, loaded.Revision+1, reloaded.Revision, "expected monotonic revision bump")
	assertDomainEqual(t, loaded, reloaded)
}

// TestShowStoreSaveIsIdempotent proves the SHOW-01 idempotency probe:
// saving the same State twice to the same path each produces a valid,
// openable .golc, Revision advances by exactly one per Save, and no entity
// is duplicated.
func TestShowStoreSaveIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "first Save")
	first, err := Load(root, path)
	require.NoError(t, err, "Load after first Save")

	require.NoError(t, Save(root, path, first), "second Save")
	second, err := Load(root, path)
	require.NoError(t, err, "Load after second Save")

	require.Equal(t, first.Revision+1, second.Revision, "expected Revision to advance by exactly 1")
	require.Len(t, second.Pools, len(first.Pools), "entity counts changed across idempotent saves: first=%+v second=%+v", first, second)
	require.Len(t, second.Deployments, len(first.Deployments), "entity counts changed across idempotent saves: first=%+v second=%+v", first, second)
	require.Len(t, second.Groups, len(first.Groups), "entity counts changed across idempotent saves: first=%+v second=%+v", first, second)
	require.Len(t, second.Scenes, len(first.Scenes), "entity counts changed across idempotent saves: first=%+v second=%+v", first, second)
	assertDomainEqual(t, first, second)
}

// TestShowLoadRejectsChecksumMismatch proves WR-01's fix: a blob altered
// after Save wrote it -- without the matching show_meta.checksum also
// being updated, exactly what a post-write corruption or tamper would
// look like -- is rejected by Load/LoadForRead instead of being silently
// trusted just because it still happens to decode and validate.
func TestShowLoadRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")

	loaded, err := Load(root, path)
	require.NoError(t, err, "Load")
	loaded.Scenes[0].Name = "Tampered After Save"
	tamperedPayload, err := strictjson.CanonicalEncode(loaded)
	require.NoError(t, err, "CanonicalEncode")

	db, err := openStore(root, path)
	require.NoError(t, err, "openStore")
	// Rewrite the blob without touching checksum -- simulating corruption
	// or a hand-edit of the .golc file outside GOLC's own write path,
	// which never leaves checksum in sync with a tampered blob.
	_, err = db.Exec(`UPDATE show_state SET blob = ? WHERE id = 1`, tamperedPayload)
	if err != nil {
		db.Close()
		require.NoError(t, err, "tampering show_state")
	}
	require.NoError(t, db.Close(), "closing tampered store")

	_, err = Load(root, path)
	require.Error(t, err, "expected Load to reject a checksum mismatch")
	require.Contains(t, err.Error(), "GOLC_SHOW_STATE_INVALID")
	require.Contains(t, err.Error(), "checksum mismatch")

	_, err = LoadForRead(root, path)
	require.Error(t, err, "expected LoadForRead to reject a checksum mismatch")
	require.Contains(t, err.Error(), "GOLC_SHOW_STATE_INVALID")
	require.Contains(t, err.Error(), "checksum mismatch")
}

// TestShowLoadDoesNotMutate proves the SHOW-02 idempotency probe: Load is
// read-only -- repeated Loads return identical State and never mutate the
// file (never bump the on-disk Revision, never write a recovery point).
func TestShowLoadDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	require.NoError(t, Save(root, path, state), "Save")

	revisionBefore := onDiskRevision(t, root, path)

	first, err := Load(root, path)
	require.NoError(t, err, "first Load")
	second, err := Load(root, path)
	require.NoError(t, err, "second Load")
	require.Equal(t, first, second, "repeated Loads returned different State")

	revisionAfter := onDiskRevision(t, root, path)
	require.Equal(t, revisionBefore, revisionAfter, "Load mutated the on-disk revision")
}

// TestShowLoadRejectsOverScopeMotionCapability proves Load's own
// validate() step re-checks untrusted disk content (CONTEXT threat
// T-02-10), not just Save's write-time guard: a MotionPreset carrying an
// out-of-scope "color" capability keyframe is written directly into
// show_state's blob via openStore, bypassing Save's validate() entirely
// (simulating a hand-edited .golc file), and Load must still reject it.
// This replaces internal/command/chase_motion_test.go's pre-SQLite
// equivalent, which wrote raw JSON bytes straight to the show path -- a
// technique the SQLite-backed store's application_id door check now
// rejects before ever reaching validate(), so the direct-write simulation
// has to happen at the blob-column level instead (05-01-PLAN.md Task 2
// deviation).
func TestShowLoadRejectsOverScopeMotionCapability(t *testing.T) {
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

	_, err = Load(root, path)
	require.Error(t, err, "expected Load to reject an over-scope motion capability")
	require.Contains(t, err.Error(), "GOLC_SHOW_STATE_INVALID")
	require.Contains(t, err.Error(), "GOLC_MOTION_PRESET_CAPABILITY_OUT_OF_SCOPE")
}

// TestShowStoreNoPlaybackImport guards the governing "storage never enters
// the playback path" invariant (SHOW-02 prohibition): internal/show must
// never import internal/playback, verified mechanically via `go list
// -deps` rather than a hand-maintained string list.
func TestShowStoreNoPlaybackImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lnorton89/golc/internal/show").Output()
	require.NoError(t, err, "go list -deps github.com/lnorton89/golc/internal/show")
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		require.NotEqual(t, "github.com/lnorton89/golc/internal/playback", dep,
			"internal/show imports internal/playback (forbidden by the SHOW-02 storage/playback separation invariant)")
	}
}

// TestSaveScrubsDanglingSceneSelectionBeforeValidate proves Save's new
// scrub step: a Scene's Layer.Selection referencing a pool that no longer
// exists (as a cascade delete would leave it) is cleaned up automatically
// rather than causing Save to fail or silently persisting the dangling
// reference for Resolve to choke on later.
func TestSaveScrubsDanglingSceneSelectionBeforeValidate(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	d.Instances = append(d.Instances, deployment.Instance{
		ID: instanceID, PoolID: p.ID, PoolMemberID: member.ID, Mode: "Standard", Universe: 1, Address: 1,
	})

	sc, err := scene.NewScene("Opener", 4)
	require.NoError(t, err, "NewScene")
	sc.Layers[0].Selection = programming.Selection{PoolIDs: []uuid.UUID{p.ID}}

	state := State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}, Scenes: []scene.Scene{sc}}
	require.NoError(t, Save(root, path, state), "initial Save")
	loaded, err := Load(root, path)
	require.NoError(t, err, "Load")
	require.Len(t, loaded.Scenes[0].Layers[0].Selection.PoolIDs, 1, "expected the valid selection to round-trip untouched, got %+v", loaded.Scenes[0].Layers[0].Selection)

	// Cascade-delete the pool (mirroring pool.DeletePool's own cascade) --
	// the scene's Layer.Selection is left dangling on purpose here, exactly
	// as a real "pool delete" CLI invocation would leave it before Save's
	// scrub runs.
	newPools, newDeployments, newGroups, err := pool.DeletePool(loaded.Pools, loaded.Deployments, loaded.Groups, p.ID)
	require.NoError(t, err, "pool.DeletePool")
	loaded.Pools, loaded.Deployments, loaded.Groups = newPools, newDeployments, newGroups

	require.NoError(t, Save(root, path, loaded), "expected Save to succeed despite the now-dangling scene selection (scrub should clean it, not reject it)")

	reloaded, err := Load(root, path)
	require.NoError(t, err, "Load after cascade delete")
	require.Empty(t, reloaded.Scenes[0].Layers[0].Selection.PoolIDs, "expected the scene's dangling PoolIDs entry to be scrubbed, got %+v", reloaded.Scenes[0].Layers[0].Selection.PoolIDs)
}

// TestSaveStillRejectsGenuinelyInvalidStateUnrelatedToSelections is a
// negative control: the new scrub step must never mask a real validate()
// failure unrelated to Selections (here, a duplicate pool name).
func TestSaveStillRejectsGenuinelyInvalidStateUnrelatedToSelections(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	p1, err := pool.NewPool("Duplicate Name", nil)
	require.NoError(t, err, "NewPool")
	p2, err := pool.NewPool("Duplicate Name", nil)
	require.NoError(t, err, "NewPool")

	state := State{Pools: []pool.Pool{p1, p2}}
	err = Save(root, path, state)
	require.Error(t, err, "expected an error for duplicate pool names")
	require.Contains(t, err.Error(), "GOLC_SHOW_STATE_INVALID")
	require.Contains(t, err.Error(), "GOLC_POOL_DUPLICATE_NAME")
}
