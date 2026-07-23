// svc_programming_test.go proves 06-12-PLAN.md Task 1's acceptance
// criteria (VERIFICATION.md Gap B[0], PLAY-12): a real on-screen scene/
// look programming surface must create a bar-loop scene, create each of
// the four look kinds (color theme, chase, motion preset, and a
// base-look preset via "programmer set" + "preset record"), enable and
// point each of a scene's four fixed layers at a reusable look
// (preserving the layer's ref across a disable/re-enable toggle,
// WR-01/WR-03), and activate exactly one scene -- all through the exact
// same "scene"/"theme"/"chase"/"motion"/"programmer"/"preset" CLI routes
// internal/command/scene.go and internal/command/programming.go already
// declare and test (mirrors svc_playback_test.go/svc_surface_test.go's
// seed-drive-assert shape exactly). This file compiles against the
// already-implemented internal/command package but fails to build/pass at
// RUN time until svc_programming.go declares ProgrammingService and its
// methods -- that is the RED state Task 1 proves; svc_programming.go is
// NOT created by this task.
package wails

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// newTestProgrammingService constructs a ProgrammingService against a
// fresh per-test root/show path, mirroring newTestPlaybackService's
// identical seed-then-exercise-bindings convention.
func newTestProgrammingService(t *testing.T) (*ProgrammingService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	return NewProgrammingService("", root, showPath), root, showPath
}

// seedProgrammingInstance builds and saves a minimal ShowState with one
// pool (one member) and one deployment with an Instance patched to that
// member, returning the Instance's ID -- the target "programmer set
// --instance <id>" resolves and edits before a "preset record" call,
// mirroring internal/command/programming_test.go's identical
// seedProgrammerShowState fixture (unexported to that package, so this
// file keeps its own copy).
func seedProgrammingInstance(t *testing.T, root, showPath string) uuid.UUID {
	t.Helper()

	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("pool.NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	if err != nil {
		t.Fatalf("pool.NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	if err != nil {
		t.Fatalf("deployment.NewDeployment: %v", err)
	}
	instanceID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save (seed): %v", err)
	}
	return instanceID
}

// findProgSceneView returns a pointer to the ProgSceneView in scenes whose
// Name matches name, or nil if absent.
func findProgSceneView(scenes []ProgSceneView, name string) *ProgSceneView {
	for i := range scenes {
		if scenes[i].Name == name {
			return &scenes[i]
		}
	}
	return nil
}

// findProgLayerView returns a pointer to the ProgLayerView in layers whose
// Kind matches kind, or nil if absent.
func findProgLayerView(layers []ProgLayerView, kind string) *ProgLayerView {
	for i := range layers {
		if layers[i].Kind == kind {
			return &layers[i]
		}
	}
	return nil
}

// TestProgrammingServiceCreateAndListScene proves CreateScene creates a
// named bar-loop scene with all four layers present, disabled, and
// ref-less, and that ListProgramming renders an explicit empty
// projection first (before any scene/look exists).
func TestProgrammingServiceCreateAndListScene(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	empty, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming (empty show): %v", err)
	}
	if len(empty.Scenes) != 0 || len(empty.Themes) != 0 || len(empty.Chases) != 0 ||
		len(empty.Motions) != 0 || len(empty.Presets) != 0 {
		t.Fatalf("expected an empty projection for a fresh show, got %+v", empty)
	}

	result := svc.CreateScene("Verse", 4)
	if result.ExitCode != 0 {
		t.Fatalf("CreateScene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	view, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming: %v", err)
	}
	sc := findProgSceneView(view.Scenes, "Verse")
	if sc == nil {
		t.Fatalf("expected scene %q in ListProgramming, got %+v", "Verse", view.Scenes)
	}
	if sc.Active {
		t.Fatal("expected a newly created scene to be inactive")
	}
	if sc.Bars != 4 {
		t.Fatalf("expected barsPerLoop=4, got %d", sc.Bars)
	}
	if len(sc.Layers) != 4 {
		t.Fatalf("expected 4 fixed layer slots, got %d", len(sc.Layers))
	}
	for _, layer := range sc.Layers {
		if layer.Enabled {
			t.Fatalf("expected layer %q to be disabled on a freshly created scene", layer.Kind)
		}
		if layer.Ref != "" {
			t.Fatalf("expected layer %q to have no ref on a freshly created scene, got %q", layer.Kind, layer.Ref)
		}
	}
}

// TestProgrammingServiceCreateEachLookKind proves CreateTheme/CreateMotion/
// CreateChase, and ProgrammerSet+RecordPreset (for a base-look preset),
// each create a named look that appears in ListProgramming's look lists.
func TestProgrammingServiceCreateEachLookKind(t *testing.T) {
	svc, root, showPath := newTestProgrammingService(t)

	// Seed the pool/deployment instance FIRST: seedProgrammingInstance
	// saves a fresh ShowState directly (show.Save), which would otherwise
	// overwrite any scene/theme/motion/chase already appended through the
	// CLI-route-backed CreateTheme/CreateMotion/CreateChase calls below.
	instanceID := seedProgrammingInstance(t, root, showPath)

	if result := svc.CreateTheme("Warm"); result.ExitCode != 0 {
		t.Fatalf("CreateTheme failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateMotion("Sweep"); result.ExitCode != 0 {
		t.Fatalf("CreateMotion failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateChase("Strobe", "bar", 1); result.ExitCode != 0 {
		t.Fatalf("CreateChase failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	if result := svc.ProgrammerSet([]string{instanceID.String()}, []string{"intensity=0.8"}); result.ExitCode != 0 {
		t.Fatalf("ProgrammerSet failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.RecordPreset("Bright", "intensity"); result.ExitCode != 0 {
		t.Fatalf("RecordPreset failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	view, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming: %v", err)
	}
	if len(view.Themes) != 1 || view.Themes[0].Name != "Warm" {
		t.Fatalf("expected exactly one theme named Warm, got %+v", view.Themes)
	}
	if len(view.Motions) != 1 || view.Motions[0].Name != "Sweep" {
		t.Fatalf("expected exactly one motion preset named Sweep, got %+v", view.Motions)
	}
	if len(view.Chases) != 1 || view.Chases[0].Name != "Strobe" {
		t.Fatalf("expected exactly one chase named Strobe, got %+v", view.Chases)
	}
	if len(view.Presets) != 1 || view.Presets[0].Name != "Bright" || view.Presets[0].Kind != "intensity" {
		t.Fatalf("expected exactly one intensity preset named Bright, got %+v", view.Presets)
	}
}

// TestProgrammingServiceSetEachLayerKind proves SetSceneLayer points+
// enables each of the four layer kinds (base_look/color_theme/chase/
// motion) and that ListProgramming reflects each layer's enabled flag
// and ref.
func TestProgrammingServiceSetEachLayerKind(t *testing.T) {
	svc, root, showPath := newTestProgrammingService(t)

	// Seed the pool/deployment instance FIRST (see identical note in
	// TestProgrammingServiceCreateEachLookKind).
	instanceID := seedProgrammingInstance(t, root, showPath)

	if result := svc.CreateScene("Verse", 4); result.ExitCode != 0 {
		t.Fatalf("CreateScene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateTheme("Warm"); result.ExitCode != 0 {
		t.Fatalf("CreateTheme failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateMotion("Sweep"); result.ExitCode != 0 {
		t.Fatalf("CreateMotion failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateChase("Strobe", "bar", 1); result.ExitCode != 0 {
		t.Fatalf("CreateChase failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.ProgrammerSet([]string{instanceID.String()}, []string{"intensity=0.8"}); result.ExitCode != 0 {
		t.Fatalf("ProgrammerSet failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.RecordPreset("Bright", "intensity"); result.ExitCode != 0 {
		t.Fatalf("RecordPreset failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	seeded, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming (seed): %v", err)
	}
	themeID := seeded.Themes[0].ID
	motionID := seeded.Motions[0].ID
	chaseID := seeded.Chases[0].ID
	presetID := seeded.Presets[0].ID

	cases := []struct {
		kind string
		ref  string
	}{
		{"color_theme", themeID},
		{"chase", chaseID},
		{"motion", motionID},
		{"base_look", presetID},
	}
	for _, tc := range cases {
		result := svc.SetSceneLayer("Verse", tc.kind, tc.ref, true)
		if result.ExitCode != 0 {
			t.Fatalf("SetSceneLayer(%s) failed: exit=%d stderr=%s", tc.kind, result.ExitCode, result.Stderr)
		}
	}

	view, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming: %v", err)
	}
	sc := findProgSceneView(view.Scenes, "Verse")
	if sc == nil {
		t.Fatalf("expected scene %q in ListProgramming", "Verse")
	}
	for _, tc := range cases {
		layer := findProgLayerView(sc.Layers, tc.kind)
		if layer == nil {
			t.Fatalf("expected layer kind %q in scene Verse", tc.kind)
		}
		if !layer.Enabled {
			t.Fatalf("expected layer %q to be enabled", tc.kind)
		}
		if layer.Ref != tc.ref {
			t.Fatalf("expected layer %q ref=%q, got %q", tc.kind, tc.ref, layer.Ref)
		}
	}
}

// TestProgrammingServiceDisableLayerPreservesRef proves SetSceneLayer's
// Ref-preserving pre-read: disabling then re-enabling a layer must never
// discard its previously assigned ref (WR-01/WR-03), even though "scene
// layer set" itself replaces Ref wholesale when --ref is omitted.
func TestProgrammingServiceDisableLayerPreservesRef(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	if result := svc.CreateScene("Verse", 4); result.ExitCode != 0 {
		t.Fatalf("CreateScene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateTheme("Warm"); result.ExitCode != 0 {
		t.Fatalf("CreateTheme failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	seeded, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming (seed): %v", err)
	}
	themeID := seeded.Themes[0].ID

	if result := svc.SetSceneLayer("Verse", "color_theme", themeID, true); result.ExitCode != 0 {
		t.Fatalf("SetSceneLayer(enable) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	// Disable without re-supplying the ref.
	if result := svc.SetSceneLayer("Verse", "color_theme", "", false); result.ExitCode != 0 {
		t.Fatalf("SetSceneLayer(disable) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	afterDisable, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming (after disable): %v", err)
	}
	sc := findProgSceneView(afterDisable.Scenes, "Verse")
	layer := findProgLayerView(sc.Layers, "color_theme")
	if layer.Enabled {
		t.Fatal("expected the layer to be disabled")
	}
	if layer.Ref != themeID {
		t.Fatalf("expected Ref to be preserved across disable, got %q want %q", layer.Ref, themeID)
	}

	// Re-enable without re-supplying the ref.
	if result := svc.SetSceneLayer("Verse", "color_theme", "", true); result.ExitCode != 0 {
		t.Fatalf("SetSceneLayer(enable) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	afterEnable, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming (after enable): %v", err)
	}
	sc = findProgSceneView(afterEnable.Scenes, "Verse")
	layer = findProgLayerView(sc.Layers, "color_theme")
	if !layer.Enabled {
		t.Fatal("expected the layer to be enabled")
	}
	if layer.Ref != themeID {
		t.Fatalf("expected Ref to be preserved across re-enable, got %q want %q", layer.Ref, themeID)
	}
}

// TestProgrammingServiceActivateScene proves ActivateScene leaves exactly
// one scene active.
func TestProgrammingServiceActivateScene(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	if result := svc.CreateScene("Verse", 4); result.ExitCode != 0 {
		t.Fatalf("CreateScene(Verse) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateScene("Chorus", 4); result.ExitCode != 0 {
		t.Fatalf("CreateScene(Chorus) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	if result := svc.ActivateScene("Chorus"); result.ExitCode != 0 {
		t.Fatalf("ActivateScene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	view, err := svc.ListProgramming()
	if err != nil {
		t.Fatalf("ListProgramming: %v", err)
	}
	activeCount := 0
	for _, sc := range view.Scenes {
		if sc.Active {
			activeCount++
			if sc.Name != "Chorus" {
				t.Fatalf("expected Chorus to be the active scene, got %q active", sc.Name)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active scene, got %d", activeCount)
	}
}
