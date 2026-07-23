// svc_playback_test.go proves 06-06-PLAN.md Task 1's acceptance criteria:
// every enumerated playback action (scene switch, layer enable/disable,
// numeric BPM entry, tap tempo, evaluate/transport) has a corresponding
// PlaybackService binding that produces the exact route/args its matching
// CLI route expects (TestPlaybackServiceEnumeratesEveryPlaybackAction),
// and a binding called with a bad argument surfaces the underlying
// registry's own diagnostic rather than panicking.
// TestPlaybackServiceAuthorize* prove CR-01's fix: once an active operator
// surface is set (SetActiveSurface), SwitchScene/SetLayerEnabled against a
// scene/layer not assigned to it are rejected before dispatching, and the
// same calls against an assigned control still dispatch exactly as before.
// TestPlaybackServiceSetLayerEnabledPropagatesPreReadFailure proves WR-01's
// fix: a genuine currentLayerRef pre-read failure is surfaced as
// SetLayerEnabled's own Result rather than silently treated as "no ref
// assigned."
package wails

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// newTestPlaybackService constructs a PlaybackService against a fresh
// per-test root/show path, mirroring internal/command's own
// seed-then-exercise-CLI-routes test convention (e.g. playback_bpm_test.go).
func newTestPlaybackService(t *testing.T) (*PlaybackService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	return NewPlaybackService("", showPath, root), root, showPath
}

// execRegistry runs args directly through a fresh default command
// registry -- used to seed fixtures (scenes/themes) independent of the
// PlaybackService methods under test.
func execRegistry(t *testing.T, root string, args ...string) command.Result {
	t.Helper()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	result := registry.Execute(command.Request{Root: root, Args: args})
	if result.ExitCode != 0 {
		t.Fatalf("seed command %v failed: exit=%d stderr=%s", args, result.ExitCode, result.Stderr)
	}
	return result
}

// TestPlaybackServiceEnumeratesEveryPlaybackAction proves PlaybackService
// binds one method per action in PLAY-01/02's enumerated playback action
// set (scene switch, layer enable/disable, numeric BPM entry, tap tempo,
// evaluate/transport) -- catching a silently dropped action.
func TestPlaybackServiceEnumeratesEveryPlaybackAction(t *testing.T) {
	svc := NewPlaybackService("", "", "")
	want := []string{"SwitchScene", "SetLayerEnabled", "SetBPM", "TapTempo", "Evaluate", "GetState"}

	typ := reflect.TypeOf(svc)
	got := map[string]bool{}
	for i := 0; i < typ.NumMethod(); i++ {
		got[typ.Method(i).Name] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Fatalf("expected PlaybackService to bind method %q for the enumerated playback action set (PLAY-01/02); it is missing", name)
		}
	}
}

// TestPlaybackServiceSwitchScene proves SwitchScene issues "playback
// switch <scene> --show <path>" and the target scene becomes active.
func TestPlaybackServiceSwitchScene(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Chorus", "--bars", "4", "--show", showPath)

	result := svc.SwitchScene("Chorus")
	if result.ExitCode != 0 {
		t.Fatalf("SwitchScene failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" && !sc.Active {
			t.Fatalf("expected Chorus to be active after SwitchScene, got Active=%v", sc.Active)
		}
		if sc.Name == "Verse" && sc.Active {
			t.Fatalf("expected Verse to be inactive after switching to Chorus, got Active=%v", sc.Active)
		}
	}
}

// TestPlaybackServiceSwitchSceneUnknownSceneReturnsDiagnosticNotPanic
// proves a bad argument (an unknown scene name) surfaces the registry's
// own diagnostic instead of panicking.
func TestPlaybackServiceSwitchSceneUnknownSceneReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.SwitchScene("DoesNotExist")
	if result.ExitCode == 0 {
		t.Fatal("expected a non-zero exit for an unknown scene name")
	}
	if !strings.Contains(result.Stderr, "GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE") {
		t.Fatalf("expected GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE in stderr, got %q", result.Stderr)
	}
}

// TestPlaybackServiceSetLayerEnabledPreservesRefAcrossToggle proves
// SetLayerEnabled's Ref-preserving pre-read: disabling then re-enabling a
// layer must never discard its previously assigned Ref, even though
// "scene layer set" itself replaces Ref wholesale when --ref is omitted
// (internal/command/scene.go's WR-03 doc comment).
func TestPlaybackServiceSetLayerEnabledPreservesRefAcrossToggle(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "theme", "create", "Warm", "--show", showPath)

	seeded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (seed): %v", err)
	}
	if len(seeded.Themes) != 1 {
		t.Fatalf("expected exactly one seeded theme, got %d", len(seeded.Themes))
	}
	themeID := seeded.Themes[0].ID

	execRegistry(t, root, "scene", "layer", "set", "Verse", "--kind", "color_theme", "--ref", themeID.String(), "--show", showPath)

	// Disable the layer through the binding under test.
	disableResult := svc.SetLayerEnabled("Verse", "color_theme", false)
	if disableResult.ExitCode != 0 {
		t.Fatalf("SetLayerEnabled(disable) failed: exit=%d stderr=%s", disableResult.ExitCode, disableResult.Stderr)
	}

	afterDisable, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (after disable): %v", err)
	}
	layer := findLayer(t, afterDisable, "Verse", "color_theme")
	if layer.Enabled {
		t.Fatal("expected the layer to be disabled")
	}
	if layer.Ref != themeID {
		t.Fatalf("expected Ref to be preserved across disable, got %v want %v", layer.Ref, themeID)
	}

	// Re-enable through the binding under test.
	enableResult := svc.SetLayerEnabled("Verse", "color_theme", true)
	if enableResult.ExitCode != 0 {
		t.Fatalf("SetLayerEnabled(enable) failed: exit=%d stderr=%s", enableResult.ExitCode, enableResult.Stderr)
	}

	afterEnable, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load (after enable): %v", err)
	}
	layer = findLayer(t, afterEnable, "Verse", "color_theme")
	if !layer.Enabled {
		t.Fatal("expected the layer to be enabled")
	}
	if layer.Ref != themeID {
		t.Fatalf("expected Ref to be preserved across re-enable, got %v want %v", layer.Ref, themeID)
	}
}

// TestPlaybackServiceSwitchSceneRejectsWhenActiveSurfaceDoesNotAssignScene
// proves CR-01's fix: once SetActiveSurface scopes PlaybackService to a
// surface that does not have the target scene in SceneRefs, SwitchScene is
// rejected with GOLC_OPERATORSURFACE_LOCKED and the show is left
// unmodified.
func TestPlaybackServiceSwitchSceneRejectsWhenActiveSurfaceDoesNotAssignScene(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Chorus", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	result := svc.SwitchScene("Chorus")
	if result.ExitCode == 0 {
		t.Fatal("expected SwitchScene to be rejected when the active surface has no matching SceneRef assigned")
	}
	if !strings.Contains(result.Stderr, "GOLC_OPERATORSURFACE_LOCKED") {
		t.Fatalf("expected GOLC_OPERATORSURFACE_LOCKED in stderr, got %q", result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" && sc.Active {
			t.Fatal("expected Chorus to remain inactive after a rejected SwitchScene")
		}
	}
}

// TestPlaybackServiceSwitchSceneDispatchesWhenActiveSurfaceAssignsScene
// proves the counterpart: once the target scene is assigned to the active
// surface, SwitchScene authorizes and dispatches exactly as before CR-01.
func TestPlaybackServiceSwitchSceneDispatchesWhenActiveSurfaceAssignsScene(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Chorus", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := surfaceSvc.AssignItem("Operator A", ControlRefInput{Kind: "scene", Scene: "Chorus"}); result.ExitCode != 0 {
		t.Fatalf("AssignItem failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	result := svc.SwitchScene("Chorus")
	if result.ExitCode != 0 {
		t.Fatalf("expected SwitchScene to dispatch once Chorus is assigned, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" && !sc.Active {
			t.Fatal("expected Chorus to be active after an authorized SwitchScene")
		}
	}
}

// TestPlaybackServiceSetActiveSurfaceEmptyClearsRestriction proves
// SetActiveSurface("") always returns to unrestricted/author-mode
// dispatch, even after a prior SetActiveSurface locked the service to a
// surface that did not assign the scene under test.
func TestPlaybackServiceSetActiveSurfaceEmptyClearsRestriction(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)

	surfaceSvc := NewSurfaceService("", root, showPath)
	if result := surfaceSvc.CreateSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("CreateSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.SetActiveSurface("Operator A"); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.SetActiveSurface(""); result.ExitCode != 0 {
		t.Fatalf("SetActiveSurface(\"\") failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	result := svc.SwitchScene("Verse")
	if result.ExitCode != 0 {
		t.Fatalf("expected SwitchScene to dispatch after the active surface was cleared, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// TestPlaybackServiceSetLayerEnabledPropagatesPreReadFailure proves WR-01's
// fix: when currentLayerRef's pre-read show.Load fails (here, showPath
// points at a directory rather than a valid .golc file, so opening the
// store errors), SetLayerEnabled returns that error as its own Result
// rather than silently proceeding as if no Ref were assigned -- proceeding
// would risk omitting --ref and discarding whatever Ref actually exists on
// disk if a subsequent show.Load inside the mutating registry call happens
// to succeed where this pre-read failed.
func TestPlaybackServiceSetLayerEnabledPropagatesPreReadFailure(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "not-a-file.golc")
	if err := os.Mkdir(showPath, 0o755); err != nil {
		t.Fatalf("failed to seed a directory at showPath: %v", err)
	}
	svc := NewPlaybackService("", showPath, root)

	result := svc.SetLayerEnabled("Verse", "color_theme", false)
	if result.ExitCode == 0 {
		t.Fatal("expected SetLayerEnabled to fail when the pre-read show.Load cannot open the store")
	}
	if result.Stderr == "" {
		t.Fatal("expected a non-empty diagnostic when the pre-read fails")
	}
}

func findLayer(t *testing.T, state show.State, sceneName, kind string) scene.Layer {
	t.Helper()
	for _, sc := range state.Scenes {
		if sc.Name != sceneName {
			continue
		}
		for _, l := range sc.Layers {
			if string(l.Kind) == kind {
				return l
			}
		}
	}
	t.Fatalf("scene %q layer %q not found", sceneName, kind)
	return scene.Layer{}
}

// TestPlaybackServiceSetLayerEnabledUnknownSceneReturnsDiagnosticNotPanic
// proves a bad argument (an unknown scene) surfaces the registry's own
// diagnostic instead of panicking.
func TestPlaybackServiceSetLayerEnabledUnknownSceneReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.SetLayerEnabled("DoesNotExist", "color_theme", false)
	if result.ExitCode == 0 {
		t.Fatal("expected a non-zero exit for an unknown scene name")
	}
	if !strings.Contains(result.Stderr, "GOLC_SCENE_NOT_FOUND") {
		t.Fatalf("expected GOLC_SCENE_NOT_FOUND in stderr, got %q", result.Stderr)
	}
}

// TestPlaybackServiceSetBPM proves SetBPM issues "playback bpm set <bpm>
// --show <path>" and persists the value.
func TestPlaybackServiceSetBPM(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)

	result := svc.SetBPM(128)
	if result.ExitCode != 0 {
		t.Fatalf("SetBPM failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if state.Tempo.BPM != 128 {
		t.Fatalf("expected Tempo.BPM=128, got %v", state.Tempo.BPM)
	}
}

// TestPlaybackServiceSetBPMInvalidValueReturnsDiagnosticNotPanic proves a
// bad argument (a non-positive BPM) surfaces the registry's own diagnostic
// instead of panicking. 0 (rather than a negative value) is used
// deliberately: internal/command/playback.go's own positional-argument
// parser treats a leading "-" as a flag prefix (GOLC_PLAYBACK_USAGE), so a
// negative BPM never reaches ValidateBPM's own domain check at all -- 0 is
// the smallest value that reaches GOLC_PLAYBACK_BPM_INVALID (mirrors
// internal/command/playback_bpm_test.go's identical TestBPMSetRejectsNonPositiveValue
// fixture).
func TestPlaybackServiceSetBPMInvalidValueReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.SetBPM(0)
	if result.ExitCode == 0 {
		t.Fatal("expected a non-zero exit for a non-positive BPM")
	}
	if !strings.Contains(result.Stderr, "GOLC_PLAYBACK_BPM_INVALID") {
		t.Fatalf("expected GOLC_PLAYBACK_BPM_INVALID in stderr, got %q", result.Stderr)
	}
}

// TestPlaybackServiceTapTempo proves TapTempo issues "playback bpm tap
// --at <ts> --at <ts> ... --show <path>" and persists the derived BPM.
func TestPlaybackServiceTapTempo(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)

	// Three taps 0.5s apart -> 120 BPM (mirrors internal/command's own
	// playback_bpm_test.go fixture).
	result := svc.TapTempo([]string{
		"2026-01-01T00:00:00Z",
		"2026-01-01T00:00:00.5Z",
		"2026-01-01T00:00:01Z",
	})
	if result.ExitCode != 0 {
		t.Fatalf("TapTempo failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	state, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if diff := state.Tempo.BPM - 120.0; diff < -1e-6 || diff > 1e-6 {
		t.Fatalf("expected Tempo.BPM=120 from tap tempo, got %v", state.Tempo.BPM)
	}
}

// TestPlaybackServiceTapTempoFewerThanTwoTapsReturnsDiagnosticNotPanic
// proves a bad argument (fewer than two taps) surfaces the registry's own
// diagnostic instead of panicking.
func TestPlaybackServiceTapTempoFewerThanTwoTapsReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.TapTempo([]string{"2026-01-01T00:00:00Z"})
	if result.ExitCode == 0 {
		t.Fatal("expected a non-zero exit for fewer than two taps")
	}
	if !strings.Contains(result.Stderr, "GOLC_PLAYBACK_TAP_INVALID") {
		t.Fatalf("expected GOLC_PLAYBACK_TAP_INVALID in stderr, got %q", result.Stderr)
	}
}

// TestPlaybackServiceEvaluate proves Evaluate issues "playback evaluate
// --at <pos> --json --show <path>" against a compiled active scene and
// returns a JSON payload.
func TestPlaybackServiceEvaluate(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "playback", "bpm", "set", "120", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "activate", "Verse", "--show", showPath)

	result := svc.Evaluate(0)
	if result.ExitCode != 0 {
		t.Fatalf("Evaluate failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if strings.TrimSpace(result.Stdout) == "" {
		t.Fatal("expected a non-empty JSON payload from Evaluate")
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatalf("expected valid JSON from Evaluate, got error: %v (stdout=%s)", err, result.Stdout)
	}
}

// TestPlaybackServiceEvaluateNoActiveSceneReturnsDiagnosticNotPanic proves
// a bad argument (evaluating with no active scene) surfaces the
// registry's own diagnostic instead of panicking.
func TestPlaybackServiceEvaluateNoActiveSceneReturnsDiagnosticNotPanic(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "playback", "bpm", "set", "120", "--show", showPath)

	result := svc.Evaluate(0)
	if result.ExitCode == 0 {
		t.Fatal("expected a non-zero exit with no active scene")
	}
	if !strings.Contains(result.Stderr, "GOLC_PLAYBACK_NO_ACTIVE_SCENE") {
		t.Fatalf("expected GOLC_PLAYBACK_NO_ACTIVE_SCENE in stderr, got %q", result.Stderr)
	}
}

// TestPlaybackServiceGetState proves GetState's JSON-safe projection
// includes every scene's name/active flag/layer set plus the show-wide
// BPM.
func TestPlaybackServiceGetState(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "playback", "bpm", "set", "110", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "activate", "Verse", "--show", showPath)

	result := svc.GetState()
	if result.ExitCode != 0 {
		t.Fatalf("GetState failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	var decoded playbackStateSummary
	if err := json.Unmarshal([]byte(result.Stdout), &decoded); err != nil {
		t.Fatalf("failed to decode GetState payload: %v (stdout=%s)", err, result.Stdout)
	}
	if decoded.BPM != 110 {
		t.Fatalf("expected BPM=110, got %v", decoded.BPM)
	}
	if len(decoded.Scenes) != 1 || decoded.Scenes[0].Name != "Verse" || !decoded.Scenes[0].Active {
		t.Fatalf("expected exactly one active scene named Verse, got %+v", decoded.Scenes)
	}
	if len(decoded.Scenes[0].Layers) != 4 {
		t.Fatalf("expected 4 fixed layer slots, got %d", len(decoded.Scenes[0].Layers))
	}
}
