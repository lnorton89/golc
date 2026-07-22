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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

func TestShowStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := "show.json"

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("NewDeployment: %v", err)
	}

	state := show.State{
		Pools:       []pool.Pool{p},
		Deployments: []deployment.Deployment{d},
	}

	if err := show.Save(root, path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := show.Load(root, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Revision != state.Revision+1 {
		t.Fatalf("expected Revision to bump by 1, got %d (was %d)", loaded.Revision, state.Revision)
	}
	if len(loaded.Pools) != 1 || loaded.Pools[0].ID != p.ID || loaded.Pools[0].Name != p.Name {
		t.Fatalf("pool did not round-trip: %+v", loaded.Pools)
	}
	if len(loaded.Deployments) != 1 || loaded.Deployments[0].ID != d.ID || loaded.Deployments[0].Name != d.Name {
		t.Fatalf("deployment did not round-trip: %+v", loaded.Deployments)
	}

	// Save again against the loaded state; Revision must bump monotonically.
	if err := show.Save(root, path, loaded); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	reloaded, err := show.Load(root, path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Revision != loaded.Revision+1 {
		t.Fatalf("expected monotonic revision bump, got %d after %d", reloaded.Revision, loaded.Revision)
	}

	// A tampered document (duplicate top-level JSON key) fails Load.
	tamperedPath := filepath.Join(root, "tampered.json")
	tampered := []byte(`{"schema_version":1,"schema_version":1,"revision":0,"pools":[],"deployments":[],"groups":[]}`)
	if err := os.WriteFile(tamperedPath, tampered, 0o644); err != nil {
		t.Fatalf("write tampered fixture: %v", err)
	}
	if _, err := show.Load(root, "tampered.json"); err == nil || !strings.Contains(err.Error(), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected GOLC_SHOW_STATE_INVALID for a tampered state, got %v", err)
	}

	// A duplicate-name State fails Save (never a silent duplicate).
	p2, err := pool.NewPool(p.Name, nil)
	if err != nil {
		t.Fatalf("NewPool (duplicate name): %v", err)
	}
	dupState := show.State{Pools: []pool.Pool{p, p2}}
	if err := show.Save(root, "dup.json", dupState); err == nil || !strings.Contains(err.Error(), "GOLC_SHOW_STATE_INVALID") {
		t.Fatalf("expected GOLC_SHOW_STATE_INVALID for duplicate pool names, got %v", err)
	}
}

func TestShowStateLoadMissingFileReturnsFreshState(t *testing.T) {
	root := t.TempDir()
	state, err := show.Load(root, "does-not-exist.json")
	if err != nil {
		t.Fatalf("Load (missing file): %v", err)
	}
	if state.Revision != 0 || len(state.Pools) != 0 || len(state.Deployments) != 0 {
		t.Fatalf("expected a fresh empty State for a missing file, got %+v", state)
	}
}
