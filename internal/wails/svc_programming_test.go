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
//
// TestProgrammingServiceEmptyAndCountStates and
// TestProgrammingServiceRejectsInvalidInputs (Task 3, 06-UI-SPEC.md-style
// backstop) prove ListProgramming's empty/count projection and that a
// duplicate name, an invalid bars value, a malformed layer ref, and a
// dangling layer ref each surface the underlying route's own diagnostic
// in Result.Stderr -- never a panic.
package wails

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "pool.NewPool")
	member, err := pool.NewPoolMember("acme/par64", "sha256:11111111")
	require.NoError(t, err, "pool.NewPoolMember")
	p.Members = append(p.Members, member)

	d, err := deployment.NewDeployment("Venue A")
	require.NoError(t, err, "deployment.NewDeployment")
	instanceID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	d.Instances = append(d.Instances, deployment.Instance{
		ID:           instanceID,
		PoolID:       p.ID,
		PoolMemberID: member.ID,
		Mode:         "Standard",
		Universe:     1,
		Address:      1,
	})

	state := show.State{Pools: []pool.Pool{p}, Deployments: []deployment.Deployment{d}}
	require.NoError(t, show.Save(root, showPath, state), "show.Save (seed)")
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
	require.NoError(t, err, "ListProgramming (empty show)")
	require.Empty(t, empty.Scenes, "expected an empty projection for a fresh show, got %+v", empty)
	require.Empty(t, empty.Themes, "expected an empty projection for a fresh show, got %+v", empty)
	require.Empty(t, empty.Chases, "expected an empty projection for a fresh show, got %+v", empty)
	require.Empty(t, empty.Motions, "expected an empty projection for a fresh show, got %+v", empty)
	require.Empty(t, empty.Presets, "expected an empty projection for a fresh show, got %+v", empty)

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)

	view, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming")
	sc := findProgSceneView(view.Scenes, "Verse")
	require.NotNil(t, sc, "expected scene %q in ListProgramming, got %+v", "Verse", view.Scenes)
	require.False(t, sc.Active, "expected a newly created scene to be inactive")
	require.EqualValues(t, 4, sc.Bars, "expected barsPerLoop=4")
	require.Len(t, sc.Layers, 4, "expected 4 fixed layer slots")
	for _, layer := range sc.Layers {
		require.False(t, layer.Enabled, "expected layer %q to be disabled on a freshly created scene", layer.Kind)
		require.Empty(t, layer.Ref, "expected layer %q to have no ref on a freshly created scene, got %q", layer.Kind, layer.Ref)
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

	result := svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)
	result = svc.CreateMotion("Sweep")
	require.Equal(t, 0, result.ExitCode, "CreateMotion failed: stderr=%s", result.Stderr)
	result = svc.CreateChase("Strobe", "bar", 1)
	require.Equal(t, 0, result.ExitCode, "CreateChase failed: stderr=%s", result.Stderr)

	result = svc.ProgrammerSet([]string{instanceID.String()}, []string{"intensity=0.8"})
	require.Equal(t, 0, result.ExitCode, "ProgrammerSet failed: stderr=%s", result.Stderr)
	result = svc.RecordPreset("Bright", "intensity")
	require.Equal(t, 0, result.ExitCode, "RecordPreset failed: stderr=%s", result.Stderr)

	view, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming")
	require.Len(t, view.Themes, 1, "expected exactly one theme named Warm, got %+v", view.Themes)
	require.Equal(t, "Warm", view.Themes[0].Name)
	require.Len(t, view.Motions, 1, "expected exactly one motion preset named Sweep, got %+v", view.Motions)
	require.Equal(t, "Sweep", view.Motions[0].Name)
	require.Len(t, view.Chases, 1, "expected exactly one chase named Strobe, got %+v", view.Chases)
	require.Equal(t, "Strobe", view.Chases[0].Name)
	require.Len(t, view.Presets, 1, "expected exactly one intensity preset named Bright, got %+v", view.Presets)
	require.Equal(t, "Bright", view.Presets[0].Name)
	require.Equal(t, "intensity", view.Presets[0].Kind)
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

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)
	result = svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)
	result = svc.CreateMotion("Sweep")
	require.Equal(t, 0, result.ExitCode, "CreateMotion failed: stderr=%s", result.Stderr)
	result = svc.CreateChase("Strobe", "bar", 1)
	require.Equal(t, 0, result.ExitCode, "CreateChase failed: stderr=%s", result.Stderr)
	result = svc.ProgrammerSet([]string{instanceID.String()}, []string{"intensity=0.8"})
	require.Equal(t, 0, result.ExitCode, "ProgrammerSet failed: stderr=%s", result.Stderr)
	result = svc.RecordPreset("Bright", "intensity")
	require.Equal(t, 0, result.ExitCode, "RecordPreset failed: stderr=%s", result.Stderr)

	seeded, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (seed)")
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
		require.Equal(t, 0, result.ExitCode, "SetSceneLayer(%s) failed: stderr=%s", tc.kind, result.Stderr)
	}

	view, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming")
	sc := findProgSceneView(view.Scenes, "Verse")
	require.NotNil(t, sc, "expected scene %q in ListProgramming", "Verse")
	for _, tc := range cases {
		layer := findProgLayerView(sc.Layers, tc.kind)
		require.NotNil(t, layer, "expected layer kind %q in scene Verse", tc.kind)
		require.True(t, layer.Enabled, "expected layer %q to be enabled", tc.kind)
		require.Equal(t, tc.ref, layer.Ref, "expected layer %q ref", tc.kind)
	}
}

// TestProgrammingServiceDisableLayerPreservesRef proves SetSceneLayer's
// Ref-preserving pre-read: disabling then re-enabling a layer must never
// discard its previously assigned ref (WR-01/WR-03), even though "scene
// layer set" itself replaces Ref wholesale when --ref is omitted.
func TestProgrammingServiceDisableLayerPreservesRef(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)
	result = svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)

	seeded, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (seed)")
	themeID := seeded.Themes[0].ID

	result = svc.SetSceneLayer("Verse", "color_theme", themeID, true)
	require.Equal(t, 0, result.ExitCode, "SetSceneLayer(enable) failed: stderr=%s", result.Stderr)

	// Disable without re-supplying the ref.
	result = svc.SetSceneLayer("Verse", "color_theme", "", false)
	require.Equal(t, 0, result.ExitCode, "SetSceneLayer(disable) failed: stderr=%s", result.Stderr)
	afterDisable, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (after disable)")
	sc := findProgSceneView(afterDisable.Scenes, "Verse")
	layer := findProgLayerView(sc.Layers, "color_theme")
	require.False(t, layer.Enabled, "expected the layer to be disabled")
	require.Equal(t, themeID, layer.Ref, "expected Ref to be preserved across disable")

	// Re-enable without re-supplying the ref.
	result = svc.SetSceneLayer("Verse", "color_theme", "", true)
	require.Equal(t, 0, result.ExitCode, "SetSceneLayer(enable) failed: stderr=%s", result.Stderr)
	afterEnable, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (after enable)")
	sc = findProgSceneView(afterEnable.Scenes, "Verse")
	layer = findProgLayerView(sc.Layers, "color_theme")
	require.True(t, layer.Enabled, "expected the layer to be enabled")
	require.Equal(t, themeID, layer.Ref, "expected Ref to be preserved across re-enable")
}

// TestProgrammingServiceActivateScene proves ActivateScene leaves exactly
// one scene active.
func TestProgrammingServiceActivateScene(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene(Verse) failed: stderr=%s", result.Stderr)
	result = svc.CreateScene("Chorus", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene(Chorus) failed: stderr=%s", result.Stderr)

	result = svc.ActivateScene("Chorus")
	require.Equal(t, 0, result.ExitCode, "ActivateScene failed: stderr=%s", result.Stderr)

	view, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming")
	activeCount := 0
	for _, sc := range view.Scenes {
		if sc.Active {
			activeCount++
			require.Equal(t, "Chorus", sc.Name, "expected Chorus to be the active scene")
		}
	}
	require.Equal(t, 1, activeCount, "expected exactly one active scene")
}

// TestProgrammingServiceEmptyAndCountStates proves ListProgramming on a
// fresh show returns an explicit empty projection (every slice present,
// zero-length -- the shape SceneProgramming.tsx renders as its empty
// state), and that the count grows correctly as scenes/looks are added --
// the basis for the frontend's own one-vs-many singular/plural rendering.
func TestProgrammingServiceEmptyAndCountStates(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	empty, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (empty show)")
	require.NotNil(t, empty.Scenes, "expected a present, empty Scenes slice")
	require.Empty(t, empty.Scenes, "expected a present, empty Scenes slice")
	require.NotNil(t, empty.Themes, "expected a present, empty Themes slice")
	require.Empty(t, empty.Themes, "expected a present, empty Themes slice")
	require.NotNil(t, empty.Instances, "expected a present, empty Instances slice")
	require.Empty(t, empty.Instances, "expected a present, empty Instances slice")

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)
	result = svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)

	one, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (one each)")
	require.Len(t, one.Scenes, 1, "expected exactly 1 scene")
	require.Len(t, one.Themes, 1, "expected exactly 1 theme")

	result = svc.CreateScene("Chorus", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene(Chorus) failed: stderr=%s", result.Stderr)
	result = svc.CreateTheme("Cool")
	require.Equal(t, 0, result.ExitCode, "CreateTheme(Cool) failed: stderr=%s", result.Stderr)

	many, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (many)")
	require.Len(t, many.Scenes, 2, "expected exactly 2 scenes")
	require.Len(t, many.Themes, 2, "expected exactly 2 themes")
}

// TestProgrammingServiceRejectsInvalidInputs proves a duplicate scene/
// theme name, an invalid bars value, a malformed (non-UUID) layer ref, and
// a layer ref pointing at a nonexistent look each surface the underlying
// route's own diagnostic in Result.Stderr -- never a panic, and never a
// silently accepted mutation.
func TestProgrammingServiceRejectsInvalidInputs(t *testing.T) {
	svc, _, _ := newTestProgrammingService(t)

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)

	// Duplicate scene name.
	dupScene := svc.CreateScene("Verse", 4)
	require.NotEqual(t, 0, dupScene.ExitCode, "expected a duplicate scene name to be rejected")
	require.Contains(t, dupScene.Stderr, "GOLC_SCENE_DUPLICATE_NAME")

	// Invalid bars value (0 is below scene.NewScene's own minimum of 1).
	invalidBars := svc.CreateScene("Chorus", 0)
	require.NotEqual(t, 0, invalidBars.ExitCode, "expected a non-positive bars value to be rejected")
	require.Contains(t, invalidBars.Stderr, "GOLC_SCENE_BARS_INVALID")

	// Duplicate theme name.
	result = svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)
	dupTheme := svc.CreateTheme("Warm")
	require.NotEqual(t, 0, dupTheme.ExitCode, "expected a duplicate theme name to be rejected")
	require.Contains(t, dupTheme.Stderr, "GOLC_THEME_DUPLICATE_NAME")

	// Malformed (non-UUID) layer ref.
	malformedRef := svc.SetSceneLayer("Verse", "color_theme", "not-a-uuid", true)
	require.NotEqual(t, 0, malformedRef.ExitCode, "expected a malformed layer ref to be rejected")
	require.Contains(t, malformedRef.Stderr, "GOLC_WAILS_PROGRAMMING_REF_INVALID")

	// A well-formed but dangling (nonexistent) layer ref.
	danglingID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7")
	danglingRef := svc.SetSceneLayer("Verse", "color_theme", danglingID.String(), true)
	require.NotEqual(t, 0, danglingRef.ExitCode, "expected a dangling layer ref to be rejected")
	require.Contains(t, danglingRef.Stderr, "GOLC_SCENE_LAYER_DANGLING_REFERENCE")

	// The rejected dangling ref must never have been persisted.
	view, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming")
	sc := findProgSceneView(view.Scenes, "Verse")
	require.NotNil(t, sc, "expected scene %q in ListProgramming", "Verse")
	layer := findProgLayerView(sc.Layers, "color_theme")
	require.Empty(t, layer.Ref, "expected the rejected dangling ref to never persist")
}

// TestProgrammingServiceRenameAndDelete proves the new rename/delete
// Wails methods for scene/theme/preset/motion/blend, plus UpdateChase,
// each mutate in place (identity/other fields untouched) and are
// reflected through ListProgramming, and that deleting a look currently
// referenced by an enabled scene layer succeeds and resets that layer to
// its default, un-refed state (scene.ScrubLayerRef) rather than being
// rejected.
func TestProgrammingServiceRenameAndDelete(t *testing.T) {
	svc, root, showPath := newTestProgrammingService(t)
	instanceID := seedProgrammingInstance(t, root, showPath)

	result := svc.CreateScene("Verse", 4)
	require.Equal(t, 0, result.ExitCode, "CreateScene failed: stderr=%s", result.Stderr)
	result = svc.CreateTheme("Warm")
	require.Equal(t, 0, result.ExitCode, "CreateTheme failed: stderr=%s", result.Stderr)
	result = svc.CreateMotion("Sweep")
	require.Equal(t, 0, result.ExitCode, "CreateMotion failed: stderr=%s", result.Stderr)
	result = svc.CreateChase("Strobe", "bar", 1)
	require.Equal(t, 0, result.ExitCode, "CreateChase failed: stderr=%s", result.Stderr)
	result = svc.ProgrammerSet([]string{instanceID.String()}, []string{"intensity=0.8"})
	require.Equal(t, 0, result.ExitCode, "ProgrammerSet failed: stderr=%s", result.Stderr)
	result = svc.RecordPreset("Bright", "intensity")
	require.Equal(t, 0, result.ExitCode, "RecordPreset failed: stderr=%s", result.Stderr)
	result = svc.CreateBlend("Fade", 2, "linear")
	require.Equal(t, 0, result.ExitCode, "CreateBlend failed: stderr=%s", result.Stderr)

	// Rename each kind; verify via ListProgramming.
	result = svc.RenameScene("Verse", "Verse Renamed")
	require.Equal(t, 0, result.ExitCode, "RenameScene failed: stderr=%s", result.Stderr)
	result = svc.RenameTheme("Warm", "Warm Renamed")
	require.Equal(t, 0, result.ExitCode, "RenameTheme failed: stderr=%s", result.Stderr)
	result = svc.RenamePreset("Bright", "Bright Renamed")
	require.Equal(t, 0, result.ExitCode, "RenamePreset failed: stderr=%s", result.Stderr)
	result = svc.RenameMotion("Sweep", "Sweep Renamed")
	require.Equal(t, 0, result.ExitCode, "RenameMotion failed: stderr=%s", result.Stderr)
	result = svc.RenameBlend("Fade", "Fade Renamed")
	require.Equal(t, 0, result.ExitCode, "RenameBlend failed: stderr=%s", result.Stderr)
	result = svc.UpdateChase("Strobe", "Strobe Renamed", "beat", 2)
	require.Equal(t, 0, result.ExitCode, "UpdateChase failed: stderr=%s", result.Stderr)

	renamed, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (after rename)")
	require.NotNil(t, findProgSceneView(renamed.Scenes, "Verse Renamed"), "expected renamed scene, got %+v", renamed.Scenes)
	require.Len(t, renamed.Themes, 1, "expected renamed theme, got %+v", renamed.Themes)
	require.Equal(t, "Warm Renamed", renamed.Themes[0].Name)
	require.Len(t, renamed.Presets, 1, "expected renamed preset, got %+v", renamed.Presets)
	require.Equal(t, "Bright Renamed", renamed.Presets[0].Name)
	require.Len(t, renamed.Motions, 1, "expected renamed motion preset, got %+v", renamed.Motions)
	require.Equal(t, "Sweep Renamed", renamed.Motions[0].Name)
	require.Len(t, renamed.Blends, 1, "expected renamed blend preset, got %+v", renamed.Blends)
	require.Equal(t, "Fade Renamed", renamed.Blends[0].Name)
	require.Len(t, renamed.Chases, 1, "expected chase updated (name/unit/step-duration), got %+v", renamed.Chases)
	require.Equal(t, "Strobe Renamed", renamed.Chases[0].Name, "expected chase updated (name/unit/step-duration), got %+v", renamed.Chases)
	require.Equal(t, "beat", renamed.Chases[0].StepUnit, "expected chase updated (name/unit/step-duration), got %+v", renamed.Chases)
	require.EqualValues(t, 2, renamed.Chases[0].StepDuration, "expected chase updated (name/unit/step-duration), got %+v", renamed.Chases)

	// Point a scene layer at the (renamed) theme, then verify deleting it
	// now succeeds and resets that layer to its default, un-refed state
	// instead of being rejected.
	themeID := renamed.Themes[0].ID
	result = svc.SetSceneLayer("Verse Renamed", "color_theme", themeID, true)
	require.Equal(t, 0, result.ExitCode, "SetSceneLayer failed: stderr=%s", result.Stderr)
	result = svc.DeleteTheme("Warm Renamed")
	require.Equal(t, 0, result.ExitCode, "DeleteTheme (referenced) failed: stderr=%s", result.Stderr)
	afterReferencedDelete, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (after referenced delete)")
	require.Empty(t, afterReferencedDelete.Themes, "expected the theme to be gone after delete")
	sceneAfterThemeDelete := findProgSceneView(afterReferencedDelete.Scenes, "Verse Renamed")
	require.NotNil(t, sceneAfterThemeDelete, "expected scene %q to survive the theme delete", "Verse Renamed")
	colorThemeLayer := findProgLayerView(sceneAfterThemeDelete.Layers, "color_theme")
	require.NotNil(t, colorThemeLayer, "expected the color-theme layer reset to its default, un-refed state")
	require.False(t, colorThemeLayer.Enabled, "expected the color-theme layer reset to its default, un-refed state, got %+v", colorThemeLayer)
	require.Empty(t, colorThemeLayer.Ref, "expected the color-theme layer reset to its default, un-refed state, got %+v", colorThemeLayer)

	// Delete every remaining kind; verify via ListProgramming. Blend has no
	// reference to guard at all.
	result = svc.DeleteScene("Verse Renamed")
	require.Equal(t, 0, result.ExitCode, "DeleteScene failed: stderr=%s", result.Stderr)
	result = svc.DeletePreset("Bright Renamed")
	require.Equal(t, 0, result.ExitCode, "DeletePreset failed: stderr=%s", result.Stderr)
	result = svc.DeleteMotion("Sweep Renamed")
	require.Equal(t, 0, result.ExitCode, "DeleteMotion failed: stderr=%s", result.Stderr)
	result = svc.DeleteBlend("Fade Renamed")
	require.Equal(t, 0, result.ExitCode, "DeleteBlend failed: stderr=%s", result.Stderr)
	result = svc.DeleteChase("Strobe Renamed")
	require.Equal(t, 0, result.ExitCode, "DeleteChase failed: stderr=%s", result.Stderr)

	finalView, err := svc.ListProgramming()
	require.NoError(t, err, "ListProgramming (final)")
	require.Empty(t, finalView.Scenes, "expected zero scenes after delete")
	require.Empty(t, finalView.Presets, "expected zero presets after delete")
	require.Empty(t, finalView.Motions, "expected zero motion presets after delete")
	require.Empty(t, finalView.Blends, "expected zero blend presets after delete")
	require.Empty(t, finalView.Chases, "expected zero chases after delete")
	require.Empty(t, finalView.Themes, "expected zero themes after delete")
}
