// state_test.go proves ShowState's round-trip/revision/validation
// contract (02-04-PLAN.md, Task 1 Wave-0 scaffold): Save then Load yields
// an equal State with Revision bumped monotonically, and a tampered or
// duplicate-name State fails Load/Save with GOLC_SHOW_STATE_INVALID.
//
// This file fails at build time until internal/show, internal/pool, and
// internal/deployment exist (Task 2) -- that is the RED state this task
// proves.
package show_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/operatorsurface"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// mustNewUUID mints a fresh UUIDv7 for use as a deliberately dangling
// reference in a test fixture, failing the test immediately on mint
// failure (mirrors the uuid.NewV7 error-handling shape used throughout
// this package's non-test code).
func mustNewUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	return id
}

func TestShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "NewDeployment")

	state := show.State{
		Pools:       []pool.Pool{p},
		Deployments: []deployment.Deployment{d},
	}

	require.NoError(t, show.Save(root, path, state), "Save")
	loaded, err := show.Load(root, path)
	require.NoError(t, err, "Load")
	require.Equal(t, state.Revision+1, loaded.Revision, "expected Revision to bump by 1")
	require.Len(t, loaded.Pools, 1, "pool did not round-trip: %+v", loaded.Pools)
	require.Equal(t, p.ID, loaded.Pools[0].ID, "pool did not round-trip: %+v", loaded.Pools)
	require.Equal(t, p.Name, loaded.Pools[0].Name, "pool did not round-trip: %+v", loaded.Pools)
	require.Len(t, loaded.Deployments, 1, "deployment did not round-trip: %+v", loaded.Deployments)
	require.Equal(t, d.ID, loaded.Deployments[0].ID, "deployment did not round-trip: %+v", loaded.Deployments)
	require.Equal(t, d.Name, loaded.Deployments[0].Name, "deployment did not round-trip: %+v", loaded.Deployments)

	// Save again against the loaded state; Revision must bump monotonically.
	require.NoError(t, show.Save(root, path, loaded), "second Save")
	reloaded, err := show.Load(root, path)
	require.NoError(t, err, "reload")
	require.Equal(t, loaded.Revision+1, reloaded.Revision, "expected monotonic revision bump")

	// A tampered document (duplicate top-level JSON key) fails Load.
	tamperedPath := filepath.Join(root, "tampered.json")
	tampered := []byte(`{"schema_version":1,"schema_version":1,"revision":0,"pools":[],"deployments":[],"groups":[]}`)
	require.NoError(t, os.WriteFile(tamperedPath, tampered, 0o644), "write tampered fixture")
	_, err = show.Load(root, "tampered.json")
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for a tampered state")

	// A duplicate-name State fails Save (never a silent duplicate).
	p2, err := pool.NewPool(p.Name, nil)
	require.NoError(t, err, "NewPool (duplicate name)")
	dupState := show.State{Pools: []pool.Pool{p, p2}}
	err = show.Save(root, "dup.json", dupState)
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for duplicate pool names")
}

func TestShowStateGroupValidation(t *testing.T) {
	root := t.TempDir()

	p, err := pool.NewPool("Wash Pool", nil)
	require.NoError(t, err, "NewPool")
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	require.NoError(t, err, "NewPoolMember")
	p.Members = append(p.Members, member)

	// A duplicate-name Groups slice fails Save (WR-02: never a silent
	// duplicate, mirroring the Pool/Deployment guarantee).
	dupGroups := show.State{
		Pools:  []pool.Pool{p},
		Groups: []pool.Group{{Name: "Front Wash"}, {Name: "Front Wash"}},
	}
	err = show.Save(root, "dup-groups.json", dupGroups)
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for duplicate group names")

	// A Group with a MemberRef pointing at a pool member that does not
	// exist fails Save (WR-02: a dangling reference is never silently
	// persisted).
	err = show.Save(root, "dangling-group.json", show.State{
		Pools: []pool.Pool{p},
		Groups: []pool.Group{{
			Name: "Front Wash",
			MemberRefs: []pool.MemberRef{{
				PoolID:       p.ID,
				PoolMemberID: mustNewUUID(t),
			}},
		}},
	})
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for a dangling group member reference")

	// A Group whose MemberRefs all resolve to a real pool/pool member
	// saves and loads successfully.
	validGroups := show.State{
		Pools: []pool.Pool{p},
		Groups: []pool.Group{{
			Name:       "Front Wash",
			MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
		}},
	}
	require.NoError(t, show.Save(root, "valid-groups.json", validGroups), "expected a valid group to save")
	loaded, err := show.Load(root, "valid-groups.json")
	require.NoError(t, err, "Load")
	require.Len(t, loaded.Groups, 1, "group did not round-trip: %+v", loaded.Groups)
	require.Equal(t, "Front Wash", loaded.Groups[0].Name, "group did not round-trip: %+v", loaded.Groups)
}

// TestShowStateOperatorSurfaceValidation proves the operatorsurface.Validate
// wiring into show.validate() (06-01-PLAN.md Task 2): a surface with a
// dangling scene/layer/group reference fails Save, and a surface whose
// refs all resolve against real scenes/groups round-trips byte-stably
// through Save/Load.
func TestShowStateOperatorSurfaceValidation(t *testing.T) {
	root := t.TempDir()

	sc, err := scene.NewScene("Opener", 4)
	require.NoError(t, err, "NewScene")
	groupID := mustNewUUID(t)
	group := pool.Group{ID: groupID, Name: "Front Wash"}

	// A dangling scene reference fails Save (T-06-02: never a silent
	// persist of an unresolvable assignment).
	danglingSurface := operatorsurface.Surface{
		ID:        mustNewUUID(t),
		Name:      "Front of House",
		SceneRefs: []uuid.UUID{mustNewUUID(t)},
	}
	err = show.Save(root, "dangling-surface.golc", show.State{
		OperatorSurfaces: []operatorsurface.Surface{danglingSurface},
	})
	require.ErrorContains(t, err, "GOLC_SHOW_STATE_INVALID", "expected error for a dangling operator surface reference")

	// A surface whose refs all resolve against real scenes/groups saves
	// and loads successfully, round-tripping byte-stably.
	validSurface := operatorsurface.Surface{
		ID:         mustNewUUID(t),
		Name:       "Front of House",
		SceneRefs:  []uuid.UUID{sc.ID},
		LayerRefs:  []operatorsurface.LayerRef{{SceneID: sc.ID, Kind: scene.ColorTheme}},
		MasterRefs: []operatorsurface.MasterRef{{Kind: operatorsurface.GroupMaster, GroupID: groupID}},
		SafetyRefs: []operatorsurface.SafetyControl{operatorsurface.RevokeAutomation},
	}
	validState := show.State{
		Scenes:           []scene.Scene{sc},
		Groups:           []pool.Group{group},
		OperatorSurfaces: []operatorsurface.Surface{validSurface},
	}
	require.NoError(t, show.Save(root, "valid-surface.golc", validState), "expected a valid operator surface to save")
	loaded, err := show.Load(root, "valid-surface.golc")
	require.NoError(t, err, "Load")
	require.Len(t, loaded.OperatorSurfaces, 1, "operator surface did not round-trip: %+v", loaded.OperatorSurfaces)
	require.Equal(t, "Front of House", loaded.OperatorSurfaces[0].Name, "operator surface did not round-trip: %+v", loaded.OperatorSurfaces)
	require.Len(t, loaded.OperatorSurfaces[0].SceneRefs, 1, "operator surface scene ref did not round-trip: %+v", loaded.OperatorSurfaces[0].SceneRefs)
	require.Equal(t, sc.ID, loaded.OperatorSurfaces[0].SceneRefs[0], "operator surface scene ref did not round-trip: %+v", loaded.OperatorSurfaces[0].SceneRefs)
}

func TestShowStateLoadMissingFileReturnsFreshState(t *testing.T) {
	root := t.TempDir()
	state, err := show.Load(root, "does-not-exist.json")
	require.NoError(t, err, "Load (missing file)")
	require.Equal(t, 0, state.Revision, "expected a fresh empty State for a missing file, got %+v", state)
	require.Empty(t, state.Pools, "expected a fresh empty State for a missing file, got %+v", state)
	require.Empty(t, state.Deployments, "expected a fresh empty State for a missing file, got %+v", state)
	require.EqualValues(t, show.DefaultBPM, state.Tempo.BPM, "expected a fresh State to seed Tempo.BPM = show.DefaultBPM -- a show with BPM<=0 fails deep inside playback.NewEngine's plan compilation the moment a daemon tries to serve it")
}
