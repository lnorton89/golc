// debugbridge_test.go covers internal/script/debugbridge.go
// (08-09-PLAN.md Task 2): the pure shim-offset line-math and message-
// formatting helpers directly (no CDP connection needed), plus the
// real-Deno-gated end-to-end proof that a real breakpoint/step debugger
// works against a live Deno --inspect-brk process, using the same
// skipUnlessDenoProvisioned helper session_test.go already established.
package script

import (
	"strings"
	"testing"

	"github.com/mafredri/cdp/protocol/runtime"

	"github.com/lnorton89/golc/internal/show"
)

// TestMaterializedCDPLine covers materializedCDPLine's exact inverse
// relationship with authorLineFromCDP/correctLine.
func TestMaterializedCDPLine(t *testing.T) {
	tests := []struct {
		authorLine, shimLineCount, want int
	}{
		{authorLine: 1, shimLineCount: 10, want: 10},
		{authorLine: 5, shimLineCount: 10, want: 14},
	}
	for _, tt := range tests {
		got := materializedCDPLine(tt.authorLine, tt.shimLineCount)
		if got != tt.want {
			t.Fatalf("materializedCDPLine(%d, %d) = %d, want %d", tt.authorLine, tt.shimLineCount, got, tt.want)
		}
	}
}

// TestAuthorLineFromCDPRoundTripsWithMaterializedCDPLine proves the two
// direction helpers agree for every non-shim author line: converting an
// author line into its CDP line and back recovers the original value.
func TestAuthorLineFromCDPRoundTripsWithMaterializedCDPLine(t *testing.T) {
	shimLineCount := 12
	for authorLine := 1; authorLine <= 20; authorLine++ {
		cdpLine := materializedCDPLine(authorLine, shimLineCount)
		gotAuthorLine, inShim := authorLineFromCDP(cdpLine, shimLineCount)
		if inShim {
			t.Fatalf("authorLineFromCDP(%d) unexpectedly reported inShim for author line %d", cdpLine, authorLine)
		}
		if gotAuthorLine != authorLine {
			t.Fatalf("round-trip authorLine = %d, want %d", gotAuthorLine, authorLine)
		}
	}
}

// TestAuthorLineFromCDPDetectsShimFrame covers a CDP line landing inside
// the injected shim (a 0-based CDP line strictly before shimLineCount-1's
// worth of shim lines).
func TestAuthorLineFromCDPDetectsShimFrame(t *testing.T) {
	_, inShim := authorLineFromCDP(0, 10)
	if !inShim {
		t.Fatal("expected CDP line 0 with a 10-line shim to be reported as inShim")
	}
}

// TestFramesFromCDPCallFrames covers framesFromCDPCallFrames' exact
// contract: File is always scriptName (never a URL from the CDP frame),
// an empty FunctionName renders as "<anonymous>", and an in-shim frame
// carries the shim marker with Line 0.
func TestFramesFromCDPCallFrames(t *testing.T) {
	frames := []runtime.CallFrame{
		{FunctionName: "doThing", LineNumber: 14, ColumnNumber: 3},
		{FunctionName: "", LineNumber: 15, ColumnNumber: 0},
		{FunctionName: "shimHelper", LineNumber: 2, ColumnNumber: 1},
	}

	got := framesFromCDPCallFrames(frames, 10, "MyScript")
	want := []StackFrame{
		{Function: "doThing", File: "MyScript", Line: 5, Column: 3},
		{Function: "<anonymous>", File: "MyScript", Line: 6, Column: 0},
		{Function: "shimHelper: " + shimErrorMarker, File: "MyScript", Line: 0, Column: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("framesFromCDPCallFrames() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("framesFromCDPCallFrames()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestFormatExceptionMessage covers the rendered multi-line message shape
// formatExceptionMessage produces.
func TestFormatExceptionMessage(t *testing.T) {
	frames := []StackFrame{
		{Function: "doThing", File: "MyScript", Line: 5, Column: 3},
	}
	got := formatExceptionMessage("Uncaught Error: boom", frames)
	if !strings.HasPrefix(got, "Uncaught Error: boom") {
		t.Fatalf("formatExceptionMessage() = %q, want it to start with the header text", got)
	}
	if !strings.Contains(got, "at doThing (MyScript:5:3)") {
		t.Fatalf("formatExceptionMessage() = %q, want it to contain the rendered frame", got)
	}
}

// --- real-Deno/CDP-gated tests -----------------------------------------

// TestDebugBridgeConnectsSetsBreakpointsAndReceivesPausedEvent is the
// live, real-process proof of D-01/D-02 end to end: a genuine Debug-mode
// Deno subprocess, a real CDP connection, a breakpoint set at an
// author-coordinate line, and a script.status ScriptEvent observed at
// that exact line once the run resumes and reaches it.
func TestDebugBridgeConnectsSetsBreakpointsAndReceivesPausedEvent(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)
	ResetScriptEventsForTesting()

	_, resync, ch, unsubscribe := SubscribeScriptEvents(0)
	if resync {
		t.Fatal("expected a fresh subscription, not an immediate resync")
	}
	defer unsubscribe()

	host, err := NewHost(HostConfig{Root: root, ShowPath: root + "/fixture.golc", Executor: &fakeExecutor{}})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}

	// Line 2 is `const x = 1;` -- a real, reachable statement the
	// breakpoint should actually hit before the script completes.
	source := "const x = 1;\nconst y = 2;\n"
	outcome, runErr := host.Run(t.Context(), show.Script{Name: "DebugMe", Source: source}, LaunchModeDebug, []int{2})
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	if outcome.Status != show.ScriptRunStatusSucceeded {
		t.Fatalf("Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusSucceeded, outcome.Reason)
	}

	var sawPausedAtLine2 bool
	drain := func() {
		for {
			select {
			case ev := <-ch:
				if ev.Kind == ScriptEventStatus && strings.Contains(ev.Reason, "GOLC_SCRIPT_DEBUG_PAUSED") && strings.Contains(ev.Reason, "line=2") {
					sawPausedAtLine2 = true
				}
			default:
				return
			}
		}
	}
	drain()
	if !sawPausedAtLine2 {
		t.Fatalf("expected a GOLC_SCRIPT_DEBUG_PAUSED event at author line 2")
	}
}

// TestPausedStillTerminates proves T-08-41/D-08's load-bearing guarantee:
// a script paused at a breakpoint past its deadline is still terminated
// -- pausing is never a way to hold a run open indefinitely.
func TestPausedStillTerminates(t *testing.T) {
	root := skipUnlessDenoProvisioned(t)

	host, err := NewHost(HostConfig{Root: root, ShowPath: root + "/fixture.golc", Executor: &fakeExecutor{}})
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}

	// Breakpoint at line 1 pauses before any of the script's own code
	// runs; the run is never resumed further, so it sits paused until its
	// 1-second deadline fires.
	source := "await new Promise((resolve) => setTimeout(resolve, 60000));\n"
	profile := show.CapabilityProfile{Scope: show.APIKeyScopeAdmin, Preset: show.ResourcePresetAdvanced, DeadlineSeconds: 1}
	outcome, runErr := host.Run(t.Context(), show.Script{Name: "PausedForever", Source: source, CapabilityProfile: profile}, LaunchModeDebug, []int{1})
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}
	if outcome.Status != show.ScriptRunStatusTerminated {
		t.Fatalf("Status = %q, want %q (reason: %s)", outcome.Status, show.ScriptRunStatusTerminated, outcome.Reason)
	}
	if !strings.Contains(outcome.Reason, "GOLC_SCRIPT_DEADLINE_EXCEEDED") {
		t.Fatalf("Reason = %q, want it to contain GOLC_SCRIPT_DEADLINE_EXCEEDED", outcome.Reason)
	}
}
