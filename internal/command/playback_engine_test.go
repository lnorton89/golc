// playback_engine_test.go proves the "playback evaluate"/"playback switch"
// route contract (03-07-PLAN.md Task 3, CONTEXT SCEN-06/SCEN-09): "playback
// evaluate" compiles the active scene and prints a deterministic Frame at
// a given musical position -- two runs against the same show produce
// byte-identical output; a show with no active scene exits non-zero with
// GOLC_PLAYBACK_NO_ACTIVE_SCENE; a show whose compile is otherwise invalid
// (here, an unset/invalid BPM) exits non-zero with GOLC_PLAYBACK_PLAN_INVALID.
// "playback switch" marks exactly one scene active and saves, rejecting an
// unknown scene name with GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE. Mirrors
// scene_test.go/playback_bpm_test.go's seed-a-ShowState-directly-then-
// exercise-CLI-routes convention.
package command_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/show"
)

// seedPlaybackEvaluateShow builds and saves a show.State with one pool/
// deployment/instance, one base-look preset, and one 4-bar active scene at
// BPM 120 -- a fully compilable show "playback evaluate" can run against.
func seedPlaybackEvaluateShow(t *testing.T, root, showPath string) uuid.UUID {
	t.Helper()

	member := pool.PoolMember{ID: uuid.New(), FixtureStableKey: "m1", FixtureContentHash: "hash1"}
	rig := pool.Pool{ID: uuid.New(), Name: "Rig", Members: []pool.PoolMember{member}}
	instance := deployment.Instance{ID: uuid.New(), PoolID: rig.ID, PoolMemberID: member.ID, Universe: 1, Address: 1}
	dep := deployment.Deployment{ID: uuid.New(), Name: "Dep", Active: true, Instances: []deployment.Instance{instance}}

	preset, err := programming.NewPreset("Rest", programming.PresetIntensity)
	if err != nil {
		t.Fatalf("NewPreset: %v", err)
	}
	preset.Attributes = []programming.PresetAttribute{
		{InstanceID: instance.ID, Capability: fixture.CapabilityIntensity, Value: 0.5},
	}

	verse, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	verse.Active = true
	verse, err = scene.SetLayer(verse, scene.Layer{
		Kind:      scene.BaseLook,
		Enabled:   true,
		Selection: programming.Selection{PoolIDs: []uuid.UUID{rig.ID}},
		Ref:       preset.ID,
	})
	if err != nil {
		t.Fatalf("SetLayer: %v", err)
	}

	state := show.State{
		Pools:       []pool.Pool{rig},
		Deployments: []deployment.Deployment{dep},
		Presets:     []programming.Preset{preset},
		Scenes:      []scene.Scene{verse},
		Tempo:       show.Tempo{BPM: 120},
	}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}
	return instance.ID
}

func TestPlaybackEvaluate(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedPlaybackEvaluateShow(t, root, showPath)

	first := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--at", "2.0", "--json", "--show", showPath,
	}})
	if first.ExitCode != 0 {
		t.Fatalf("playback evaluate failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)
	}

	second := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--at", "2.0", "--json", "--show", showPath,
	}})
	if second.ExitCode != 0 {
		t.Fatalf("playback evaluate (second run) failed: exit=%d stderr=%s", second.ExitCode, second.Stderr)
	}

	if !bytes.Equal(first.Stdout, second.Stdout) {
		t.Fatalf("expected byte-identical output across two evaluate runs (SCEN-09):\nfirst:  %s\nsecond: %s", first.Stdout, second.Stdout)
	}
	if !strings.Contains(string(first.Stdout), "0.5") {
		t.Fatalf("expected the resolved base-look intensity 0.5 in the evaluated frame, got %s", first.Stdout)
	}
}

func TestPlaybackEvaluateHumanReadableSummary(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")
	seedPlaybackEvaluateShow(t, root, showPath)

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--at", "2.0", "--show", showPath,
	}})
	if result.ExitCode != 0 {
		t.Fatalf("playback evaluate failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stdout), "GOLC_PLAYBACK_EVALUATE: bar=2") {
		t.Fatalf("expected GOLC_PLAYBACK_EVALUATE summary with bar=2, got %s", result.Stdout)
	}
}

func TestPlaybackEvaluateNoActiveScene(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	verse, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	// Never activated -- state has no active scene.
	state := show.State{Scenes: []scene.Scene{verse}, Tempo: show.Tempo{BPM: 120}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--at", "0.0", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_PLAYBACK_NO_ACTIVE_SCENE") {
		t.Fatalf("expected non-zero exit with GOLC_PLAYBACK_NO_ACTIVE_SCENE, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestPlaybackEvaluateInvalidPlan(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	verse, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	verse.Active = true
	// Tempo.BPM is left at its zero-value "not yet set" state -- a
	// perfectly valid persisted show.State (show.validate() does not
	// itself bound BPM), but Compile's own ValidateBPM re-check rejects
	// it with GOLC_PLAYBACK_PLAN_INVALID.
	state := show.State{Scenes: []scene.Scene{verse}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--at", "0.0", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_PLAYBACK_PLAN_INVALID") {
		t.Fatalf("expected non-zero exit with GOLC_PLAYBACK_PLAN_INVALID, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestPlaybackEvaluateMissingAtUsage(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "evaluate", "--show", showPath,
	}})
	if result.ExitCode != 2 || !strings.Contains(string(result.Stderr), "GOLC_PLAYBACK_USAGE") {
		t.Fatalf("expected exit 2 GOLC_PLAYBACK_USAGE for a missing --at, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
}

func TestPlaybackSwitchActivatesScene(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	verse, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene(Verse): %v", err)
	}
	verse.Active = true
	chorus, err := scene.NewScene("Chorus", 8)
	if err != nil {
		t.Fatalf("NewScene(Chorus): %v", err)
	}
	state := show.State{Scenes: []scene.Scene{verse, chorus}, Tempo: show.Tempo{BPM: 120}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "switch", "Chorus", "--show", showPath,
	}})
	if result.ExitCode != 0 {
		t.Fatalf("playback switch failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stdout), "GOLC_PLAYBACK_SWITCH") {
		t.Fatalf("expected GOLC_PLAYBACK_SWITCH in stdout, got %s", result.Stdout)
	}

	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	activeCount := 0
	for _, s := range reloaded.Scenes {
		if s.Active {
			activeCount++
			if s.Name != "Chorus" {
				t.Fatalf("expected Chorus to be the only active scene, got %q active", s.Name)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active scene after switch, got %d", activeCount)
	}
}

func TestPlaybackSwitchUnknownScene(t *testing.T) {
	root := t.TempDir()
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry: %v", err)
	}
	showPath := filepath.Join(t.TempDir(), "show.json")

	verse, err := scene.NewScene("Verse", 4)
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	verse.Active = true
	state := show.State{Scenes: []scene.Scene{verse}, Tempo: show.Tempo{BPM: 120}}
	if err := show.Save(root, showPath, state); err != nil {
		t.Fatalf("show.Save: %v", err)
	}

	result := registry.Execute(command.Request{Root: root, Args: []string{
		"playback", "switch", "Unknown", "--show", showPath,
	}})
	if result.ExitCode == 0 || !strings.Contains(string(result.Stderr), "GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE") {
		t.Fatalf("expected non-zero exit with GOLC_PLAYBACK_SWITCH_UNKNOWN_SCENE, got exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	// The show must remain unchanged: Verse is still the only active
	// scene.
	reloaded, err := show.Load(root, showPath)
	if err != nil {
		t.Fatalf("show.Load: %v", err)
	}
	if len(reloaded.Scenes) != 1 || !reloaded.Scenes[0].Active || reloaded.Scenes[0].Name != "Verse" {
		t.Fatalf("expected Verse to remain the only active scene after a rejected switch, got %+v", reloaded.Scenes)
	}
}
