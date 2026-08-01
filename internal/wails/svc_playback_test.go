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

	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewDefaultCommandRegistry")
	result := registry.Execute(command.Request{Root: root, Args: args})
	require.Equal(t, 0, result.ExitCode, "seed command %v failed: stderr=%s", args, result.Stderr)
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
		require.True(t, got[name], "expected PlaybackService to bind method %q for the enumerated playback action set (PLAY-01/02); it is missing", name)
	}
}

// TestPlaybackServiceSwitchScene proves SwitchScene issues "playback
// switch <scene> --show <path>" and the target scene becomes active.
func TestPlaybackServiceSwitchScene(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "scene", "create", "Verse", "--bars", "4", "--show", showPath)
	execRegistry(t, root, "scene", "create", "Chorus", "--bars", "4", "--show", showPath)

	result := svc.SwitchScene("Chorus")
	require.Equal(t, 0, result.ExitCode, "SwitchScene failed: stderr=%s", result.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" {
			require.True(t, sc.Active, "expected Chorus to be active after SwitchScene")
		}
		if sc.Name == "Verse" {
			require.False(t, sc.Active, "expected Verse to be inactive after switching to Chorus")
		}
	}
}

// TestPlaybackServiceSwitchSceneUnknownSceneReturnsDiagnosticNotPanic
// proves a bad argument (an unknown scene name) surfaces the registry's
// own diagnostic instead of panicking.
func TestPlaybackServiceSwitchSceneUnknownSceneReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.SwitchScene("DoesNotExist")
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit for an unknown scene name")
	require.Contains(t, result.Stderr, "GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE")
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
	require.NoError(t, err, "show.Load (seed)")
	require.Len(t, seeded.Themes, 1, "expected exactly one seeded theme")
	themeID := seeded.Themes[0].ID

	execRegistry(t, root, "scene", "layer", "set", "Verse", "--kind", "color_theme", "--ref", themeID.String(), "--show", showPath)

	// Disable the layer through the binding under test.
	disableResult := svc.SetLayerEnabled("Verse", "color_theme", false)
	require.Equal(t, 0, disableResult.ExitCode, "SetLayerEnabled(disable) failed: stderr=%s", disableResult.Stderr)

	afterDisable, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (after disable)")
	layer := findLayer(t, afterDisable, "Verse", "color_theme")
	require.False(t, layer.Enabled, "expected the layer to be disabled")
	require.Equal(t, themeID, layer.Ref, "expected Ref to be preserved across disable")

	// Re-enable through the binding under test.
	enableResult := svc.SetLayerEnabled("Verse", "color_theme", true)
	require.Equal(t, 0, enableResult.ExitCode, "SetLayerEnabled(enable) failed: stderr=%s", enableResult.Stderr)

	afterEnable, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load (after enable)")
	layer = findLayer(t, afterEnable, "Verse", "color_theme")
	require.True(t, layer.Enabled, "expected the layer to be enabled")
	require.Equal(t, themeID, layer.Ref, "expected Ref to be preserved across re-enable")
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
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)
	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)

	result = svc.SwitchScene("Chorus")
	require.NotEqual(t, 0, result.ExitCode, "expected SwitchScene to be rejected when the active surface has no matching SceneRef assigned")
	require.Contains(t, result.Stderr, "GOLC_OPERATORSURFACE_LOCKED")

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" {
			require.False(t, sc.Active, "expected Chorus to remain inactive after a rejected SwitchScene")
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
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)
	result = surfaceSvc.AssignItem("Operator A", ControlRefInput{Kind: "scene", Scene: "Chorus"})
	require.Equal(t, 0, result.ExitCode, "AssignItem failed: stderr=%s", result.Stderr)
	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)

	result = svc.SwitchScene("Chorus")
	require.Equal(t, 0, result.ExitCode, "expected SwitchScene to dispatch once Chorus is assigned, got stderr=%s", result.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	for _, sc := range state.Scenes {
		if sc.Name == "Chorus" {
			require.True(t, sc.Active, "expected Chorus to be active after an authorized SwitchScene")
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
	result := surfaceSvc.CreateSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "CreateSurface failed: stderr=%s", result.Stderr)
	result = svc.SetActiveSurface("Operator A")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface failed: stderr=%s", result.Stderr)
	result = svc.SetActiveSurface("")
	require.Equal(t, 0, result.ExitCode, "SetActiveSurface(\"\") failed: stderr=%s", result.Stderr)

	result = svc.SwitchScene("Verse")
	require.Equal(t, 0, result.ExitCode, "expected SwitchScene to dispatch after the active surface was cleared, got stderr=%s", result.Stderr)
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
	require.NoError(t, os.Mkdir(showPath, 0o755), "failed to seed a directory at showPath")
	svc := NewPlaybackService("", showPath, root)

	result := svc.SetLayerEnabled("Verse", "color_theme", false)
	require.NotEqual(t, 0, result.ExitCode, "expected SetLayerEnabled to fail when the pre-read show.Load cannot open the store")
	require.NotEmpty(t, result.Stderr, "expected a non-empty diagnostic when the pre-read fails")
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
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit for an unknown scene name")
	require.Contains(t, result.Stderr, "GOLC_SCENE_NOT_FOUND")
}

// TestPlaybackServiceSetBPM proves SetBPM issues "playback bpm set <bpm>
// --show <path>" and persists the value.
func TestPlaybackServiceSetBPM(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)

	result := svc.SetBPM(128)
	require.Equal(t, 0, result.ExitCode, "SetBPM failed: stderr=%s", result.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.EqualValues(t, 128, state.Tempo.BPM, "expected Tempo.BPM=128, got %v", state.Tempo.BPM)
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
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit for a non-positive BPM")
	require.Contains(t, result.Stderr, "GOLC_PLAYBACK_BPM_INVALID")
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
	require.Equal(t, 0, result.ExitCode, "TapTempo failed: stderr=%s", result.Stderr)

	state, err := show.Load(root, showPath)
	require.NoError(t, err, "show.Load")
	require.InDelta(t, 120.0, state.Tempo.BPM, 1e-6, "expected Tempo.BPM=120 from tap tempo")
}

// TestPlaybackServiceTapTempoFewerThanTwoTapsReturnsDiagnosticNotPanic
// proves a bad argument (fewer than two taps) surfaces the registry's own
// diagnostic instead of panicking.
func TestPlaybackServiceTapTempoFewerThanTwoTapsReturnsDiagnosticNotPanic(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.TapTempo([]string{"2026-01-01T00:00:00Z"})
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit for fewer than two taps")
	require.Contains(t, result.Stderr, "GOLC_PLAYBACK_TAP_INVALID")
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
	require.Equal(t, 0, result.ExitCode, "Evaluate failed: stderr=%s", result.Stderr)
	require.NotEmpty(t, strings.TrimSpace(result.Stdout), "expected a non-empty JSON payload from Evaluate")
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &decoded), "expected valid JSON from Evaluate (stdout=%s)", result.Stdout)
}

// TestPlaybackServiceEvaluateNoActiveSceneReturnsDiagnosticNotPanic proves
// a bad argument (evaluating with no active scene) surfaces the
// registry's own diagnostic instead of panicking.
func TestPlaybackServiceEvaluateNoActiveSceneReturnsDiagnosticNotPanic(t *testing.T) {
	svc, root, showPath := newTestPlaybackService(t)
	execRegistry(t, root, "playback", "bpm", "set", "120", "--show", showPath)

	result := svc.Evaluate(0)
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit with no active scene")
	require.Contains(t, result.Stderr, "GOLC_PLAYBACK_NO_ACTIVE_SCENE")
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
	require.Equal(t, 0, result.ExitCode, "GetState failed: stderr=%s", result.Stderr)

	var decoded playbackStateSummary
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &decoded), "failed to decode GetState payload (stdout=%s)", result.Stdout)
	require.EqualValues(t, 110, decoded.BPM, "expected BPM=110")
	require.Len(t, decoded.Scenes, 1, "expected exactly one active scene named Verse, got %+v", decoded.Scenes)
	require.Equal(t, "Verse", decoded.Scenes[0].Name, "expected exactly one active scene named Verse, got %+v", decoded.Scenes)
	require.True(t, decoded.Scenes[0].Active, "expected exactly one active scene named Verse, got %+v", decoded.Scenes)
	require.Len(t, decoded.Scenes[0].Layers, 4, "expected 4 fixed layer slots")
}

// TestPlaybackServiceGetStateEmptyShowScenesIsArrayNotNull proves a
// fresh/scene-less show's GetState renders "scenes":[] on the wire, never
// "scenes":null. Decoding the payload into playbackStateSummary and
// checking len(Scenes) == 0 would NOT catch a regression here: both a nil
// slice and an empty slice decode identically. The frontend's
// PlaybackStateSummary.scenes is typed as a non-nullable array
// (frontend/src/lib/playbackDispatch.ts) and several consumers call
// state.scenes.find/map/some directly -- a raw `null` on the wire crashed
// the entire webview with "Cannot read properties of null" the moment a
// genuinely scene-less show (playback.NewEngine's own no-active-scene
// idle state) reached this route, so this test asserts the literal JSON
// bytes, not just the decoded Go value.
func TestPlaybackServiceGetStateEmptyShowScenesIsArrayNotNull(t *testing.T) {
	svc, _, _ := newTestPlaybackService(t)

	result := svc.GetState()
	require.Equal(t, 0, result.ExitCode, "GetState failed: stderr=%s", result.Stderr)

	var raw struct {
		Scenes json.RawMessage `json:"scenes"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout), &raw), "failed to decode GetState payload (stdout=%s)", result.Stdout)
	require.NotEqual(t, "null", strings.TrimSpace(string(raw.Scenes)), "GetState rendered scenes as the literal JSON null instead of an empty array: %s", result.Stdout)
	var scenes []sceneSummary
	require.NoError(t, json.Unmarshal(raw.Scenes, &scenes), "scenes field did not decode as a JSON array (raw=%s)", raw.Scenes)
	require.Empty(t, scenes, "expected zero scenes on a fresh show")
}
