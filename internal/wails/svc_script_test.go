// svc_script_test.go proves 08-04-PLAN.md Task 1's acceptance criteria
// (SCRP-01, D-16/D-07/D-09/D-14): a Wails-bound ScriptService binds every
// "script *" CLI route (internal/command/script.go) -- create, list, show,
// edit (as SaveScriptSource), delete, and profile set -- so the desktop
// Scripts workspace and the terminal reach the exact same mutation
// implementation, never a second one (mirrors svc_programming_test.go's
// seed-drive-assert shape exactly). This file compiles against the
// already-implemented internal/command package but fails to build/pass at
// RUN time until svc_script.go declares ScriptService and its methods --
// that is the RED state Task 1 proves; svc_script.go is NOT created by
// this task.
package wails

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/script"
)

// newTestScriptService constructs a ScriptService against a fresh per-test
// root/show path, mirroring newTestProgrammingService's identical
// seed-then-exercise-bindings convention.
func newTestScriptService(t *testing.T) (*ScriptService, string, string) {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(t.TempDir(), "show.json")
	return NewScriptService("", root, showPath), root, showPath
}

// findScriptSummary returns a pointer to the ScriptSummaryView in views
// whose Name matches name, or nil if absent.
func findScriptSummary(views []ScriptSummaryView, name string) *ScriptSummaryView {
	for i := range views {
		if views[i].Name == name {
			return &views[i]
		}
	}
	return nil
}

// TestScriptServiceListScriptsEmptyAndPopulated proves ListScripts returns
// an explicit empty projection for a fresh show, and reflects a created
// script's name/status/scope/preset once one exists (D-16's library-row
// projection).
func TestScriptServiceListScriptsEmptyAndPopulated(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	empty, err := svc.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts (empty show): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected an empty projection for a fresh show, got %+v", empty)
	}

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	views, err := svc.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	view := findScriptSummary(views, "Chase Cycler")
	if view == nil {
		t.Fatalf("expected script %q in ListScripts, got %+v", "Chase Cycler", views)
	}
	if view.LastRunStatus != "never_run" {
		t.Fatalf("expected a freshly created script's LastRunStatus to be never_run, got %q", view.LastRunStatus)
	}
	if view.Scope != "playback" {
		t.Fatalf("expected a freshly created script's Scope to default to playback, got %q", view.Scope)
	}
	if view.Preset != "quick-action" {
		t.Fatalf("expected a freshly created script's Preset to default to quick-action, got %q", view.Preset)
	}
}

// TestScriptServiceListScriptsMissingShow proves ListScripts surfaces an
// error rather than a panic when the show cannot be read (an invalid root
// makes show.Load fail).
func TestScriptServiceListScriptsMissingShow(t *testing.T) {
	svc := NewScriptService("", string([]byte{0}), filepath.Join(string([]byte{0}), "show.json"))
	if _, err := svc.ListScripts(); err == nil {
		t.Fatal("expected ListScripts to return an error for an unreadable show")
	}
}

// TestScriptServiceGetScriptIncludesSource proves GetScript returns a
// ScriptDetailView including Source.
func TestScriptServiceGetScriptIncludesSource(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	detail, err := svc.GetScript("Chase Cycler")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if detail.Name != "Chase Cycler" {
		t.Fatalf("expected Name=%q, got %q", "Chase Cycler", detail.Name)
	}
	if detail.Source != "" {
		t.Fatalf("expected a freshly created script's Source to be empty, got %q", detail.Source)
	}
}

// TestScriptServiceCreateScriptRejectsDuplicateName proves CreateScript
// returns Result{ExitCode:0} on success and a Result carrying
// GOLC_SCRIPT_NAME_DUPLICATE in Stderr on a duplicate name.
func TestScriptServiceCreateScriptRejectsDuplicateName(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	first := svc.CreateScript("Chase Cycler")
	if first.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", first.ExitCode, first.Stderr)
	}

	duplicate := svc.CreateScript("Chase Cycler")
	if duplicate.ExitCode == 0 {
		t.Fatal("expected a duplicate script name to be rejected")
	}
	if !strings.Contains(duplicate.Stderr, "GOLC_SCRIPT_NAME_DUPLICATE") {
		t.Fatalf("expected GOLC_SCRIPT_NAME_DUPLICATE in stderr, got %q", duplicate.Stderr)
	}
}

// TestScriptServiceSaveScriptSourceRoundTrips proves SaveScriptSource
// persists the source verbatim (including trailing newlines), and a round
// trip through GetScript returns identical bytes (D-14).
func TestScriptServiceSaveScriptSourceRoundTrips(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	source := "export function run() {\n  console.log(\"hi\");\n}\n\n"
	if result := svc.SaveScriptSource("Chase Cycler", source); result.ExitCode != 0 {
		t.Fatalf("SaveScriptSource failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	detail, err := svc.GetScript("Chase Cycler")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if detail.Source != source {
		t.Fatalf("expected source to round-trip verbatim, got %q want %q", detail.Source, source)
	}
}

// TestScriptServiceSaveScriptSourceRejectsOversized proves SaveScriptSource
// rejects a source exceeding the 1 MiB bound with
// GOLC_SCRIPT_SOURCE_TOO_LARGE before writing anything (T-08-03/T-08-12).
func TestScriptServiceSaveScriptSourceRejectsOversized(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	oversized := strings.Repeat("a", (1<<20)+1)
	result := svc.SaveScriptSource("Chase Cycler", oversized)
	if result.ExitCode == 0 {
		t.Fatal("expected an oversized source to be rejected")
	}
	if !strings.Contains(result.Stderr, "GOLC_SCRIPT_SOURCE_TOO_LARGE") {
		t.Fatalf("expected GOLC_SCRIPT_SOURCE_TOO_LARGE in stderr, got %q", result.Stderr)
	}

	detail, err := svc.GetScript("Chase Cycler")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if detail.Source != "" {
		t.Fatalf("expected the rejected oversized source to never persist, got source of length %d", len(detail.Source))
	}
}

// TestScriptServiceDeleteScriptRemovesFromList proves DeleteScript removes
// a script such that a subsequent ListScripts omits it.
func TestScriptServiceDeleteScriptRemovesFromList(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	if result := svc.CreateScript("Blackout Fade"); result.ExitCode != 0 {
		t.Fatalf("CreateScript(Blackout Fade) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	if result := svc.DeleteScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("DeleteScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	views, err := svc.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	if findScriptSummary(views, "Chase Cycler") != nil {
		t.Fatalf("expected Chase Cycler to be removed, got %+v", views)
	}
	if findScriptSummary(views, "Blackout Fade") == nil {
		t.Fatalf("expected Blackout Fade to remain, got %+v", views)
	}
}

// TestScriptServiceSetScriptProfileForwardsOnlySuppliedFields proves
// SetScriptProfile forwards only the non-empty/positive values as flags,
// leaving unspecified profile fields untouched (D-09's partial-edit
// discipline).
func TestScriptServiceSetScriptProfileForwardsOnlySuppliedFields(t *testing.T) {
	svc, _, _ := newTestScriptService(t)

	if result := svc.CreateScript("Chase Cycler"); result.ExitCode != 0 {
		t.Fatalf("CreateScript failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	// Set scope+preset first.
	if result := svc.SetScriptProfile("Chase Cycler", "authoring", "advanced", 45, 0, 0, 0); result.ExitCode != 0 {
		t.Fatalf("SetScriptProfile failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	afterFirst, err := svc.GetScript("Chase Cycler")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if afterFirst.Scope != "authoring" {
		t.Fatalf("expected Scope=authoring, got %q", afterFirst.Scope)
	}
	if afterFirst.Preset != "advanced" {
		t.Fatalf("expected Preset=advanced, got %q", afterFirst.Preset)
	}
	if afterFirst.DeadlineSeconds != 45 {
		t.Fatalf("expected DeadlineSeconds=45, got %d", afterFirst.DeadlineSeconds)
	}

	// A second call touching only RatePerSecond must leave Scope/Preset/
	// DeadlineSeconds untouched (zero/empty values here must NOT be
	// forwarded as flags).
	if result := svc.SetScriptProfile("Chase Cycler", "", "", 0, 10, 0, 0); result.ExitCode != 0 {
		t.Fatalf("SetScriptProfile (second call) failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}

	afterSecond, err := svc.GetScript("Chase Cycler")
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if afterSecond.Scope != "authoring" {
		t.Fatalf("expected Scope to remain authoring, got %q", afterSecond.Scope)
	}
	if afterSecond.Preset != "advanced" {
		t.Fatalf("expected Preset to remain advanced, got %q", afterSecond.Preset)
	}
	if afterSecond.DeadlineSeconds != 45 {
		t.Fatalf("expected DeadlineSeconds to remain 45, got %d", afterSecond.DeadlineSeconds)
	}
	if afterSecond.RatePerSecond != 10 {
		t.Fatalf("expected RatePerSecond=10, got %d", afterSecond.RatePerSecond)
	}
}

// TestScriptEventStreamForwardsPublishedEventsToEmit covers 08-08-PLAN.md
// Task 3's StartScriptEventStream/StopScriptEventStream: a live event
// published on script.PublishScriptEvent (internal/script/events.go)
// after the stream starts reaches EventPusher's own emit under
// "script:event", and StopScriptEventStream cleanly unblocks the
// forwarding goroutine without leaking it.
func TestScriptEventStreamForwardsPublishedEventsToEmit(t *testing.T) {
	script.ResetScriptEventsForTesting()
	t.Cleanup(script.ResetScriptEventsForTesting)

	svc, _, _ := newTestScriptService(t)

	var mu sync.Mutex
	var pushed []ScriptEventView
	received := make(chan struct{}, 1)
	svc.events.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "script:event" {
			return
		}
		if view, ok := data[0].(ScriptEventView); ok {
			mu.Lock()
			pushed = append(pushed, view)
			mu.Unlock()
			select {
			case received <- struct{}{}:
			default:
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartScriptEventStream(ctx)
	defer svc.StopScriptEventStream()

	runID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	script.PublishScriptEvent(script.ScriptEvent{
		Kind: script.ScriptEventLog, RunID: runID, ScriptName: "Chase", Message: "hello",
	})

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-received:
			// A flush tick must actually run before emit fires (the
			// EventPusher's own ~25ms cadence) -- keep waiting for at
			// least one to land in pushed.
		case <-deadline:
			t.Fatal("timed out waiting for the published event to reach emit")
		}
		mu.Lock()
		found := len(pushed) > 0
		mu.Unlock()
		if found {
			break
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 1 || pushed[0].Message != "hello" || pushed[0].ScriptName != "Chase" {
		t.Fatalf("expected exactly one forwarded event carrying the published payload, got %+v", pushed)
	}
}

// TestStopScriptEventStreamBeforeStartIsNoop proves StopScriptEventStream
// is safe to call before StartScriptEventStream, mirroring
// SafetyService.StopStatusPush's own documented no-op contract.
func TestStopScriptEventStreamBeforeStartIsNoop(t *testing.T) {
	svc, _, _ := newTestScriptService(t)
	svc.StopScriptEventStream()
}
