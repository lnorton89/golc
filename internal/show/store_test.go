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
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
)

// buildNonTrivialState constructs a State carrying at least one pool (with
// one member), one deployment with one concretely-addressed instance, one
// group referencing that pool member, a scene, and a non-zero Tempo.BPM --
// the minimum shape the RED-state contract in 05-01-PLAN.md Task 1 asks
// for.
func buildNonTrivialState(t *testing.T) State {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}
	instanceID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7 (instance): %v", err)
	}
	universe, address, err := deployment.NextFreeAddress(nil, 3)
	if err != nil {
		t.Fatalf("NextFreeAddress: %v", err)
	}
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "3ch",
		Universe:     universe,
		Address:      address,
	})

	groupID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7 (group): %v", err)
	}
	group := pool.Group{
		ID:         groupID,
		Name:       "Front Wash",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
	}

	sc, err := scene.NewScene("Opener", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}

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
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("domain fields did not round-trip:\nwant %+v\ngot  %+v", want, got)
	}
}

// onDiskRevision opens the store directly (bypassing Load's validation) to
// read show_meta.revision, so tests can prove a Load never wrote a new
// revision -- calling openStore directly is the strongest possible proof,
// stronger than re-Loading and comparing State.Revision alone.
func onDiskRevision(t *testing.T, root, path string) int {
	t.Helper()
	db, err := openStore(root, path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer db.Close()
	var revision int
	if err := db.QueryRow(`SELECT revision FROM show_meta WHERE id = 1`).Scan(&revision); err != nil {
		t.Fatalf("querying show_meta.revision: %v", err)
	}
	return revision
}

// TestShowStoreRoundTrip proves SHOW-01: a complete State saves to and
// loads from one SQLite .golc file with byte-identical domain fields, and
// Revision increments exactly once per Save.
func TestShowStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"

	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Revision != state.Revision+1 {
		t.Fatalf("expected Revision to bump by 1, got %d (was %d)", loaded.Revision, state.Revision)
	}
	assertDomainEqual(t, state, loaded)

	// Save again against the loaded state; Revision must bump monotonically.
	if err := Save(root, path, loaded); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	reloaded, err := Load(root, path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Revision != loaded.Revision+1 {
		t.Fatalf("expected monotonic revision bump, got %d after %d", reloaded.Revision, loaded.Revision)
	}
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

	if err := Save(root, path, state); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	first, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load after first Save: %v", err)
	}

	if err := Save(root, path, first); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	second, err := Load(root, path)
	if err != nil {
		t.Fatalf("Load after second Save: %v", err)
	}

	if second.Revision != first.Revision+1 {
		t.Fatalf("expected Revision to advance by exactly 1, got %d after %d", second.Revision, first.Revision)
	}
	if len(second.Pools) != len(first.Pools) ||
		len(second.Deployments) != len(first.Deployments) ||
		len(second.Groups) != len(first.Groups) ||
		len(second.Scenes) != len(first.Scenes) {
		t.Fatalf("entity counts changed across idempotent saves: first=%+v second=%+v", first, second)
	}
	assertDomainEqual(t, first, second)
}

// TestShowLoadDoesNotMutate proves the SHOW-02 idempotency probe: Load is
// read-only -- repeated Loads return identical State and never mutate the
// file (never bump the on-disk Revision, never write a recovery point).
func TestShowLoadDoesNotMutate(t *testing.T) {
	root := t.TempDir()
	path := "show.golc"
	state := buildNonTrivialState(t)

	if err := Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	revisionBefore := onDiskRevision(t, root, path)

	first, err := Load(root, path)
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	second, err := Load(root, path)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated Loads returned different State:\nfirst  %+v\nsecond %+v", first, second)
	}

	revisionAfter := onDiskRevision(t, root, path)
	if revisionBefore != revisionAfter {
		t.Fatalf("Load mutated the on-disk revision: before=%d after=%d", revisionBefore, revisionAfter)
	}
}

// TestShowStoreNoPlaybackImport guards the governing "storage never enters
// the playback path" invariant (SHOW-02 prohibition): internal/show must
// never import internal/playback, verified mechanically via `go list
// -deps` rather than a hand-maintained string list.
func TestShowStoreNoPlaybackImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/lnorton89/golc/internal/show").Output()
	if err != nil {
		t.Fatalf("go list -deps github.com/lnorton89/golc/internal/show: %v", err)
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if dep == "github.com/lnorton89/golc/internal/playback" {
			t.Fatalf("internal/show imports internal/playback (forbidden by the SHOW-02 storage/playback separation invariant)")
		}
	}
}
