// events_test.go covers events.go's script event bus (08-08-PLAN.md
// Task 1): strictly increasing Seq assignment, live/replay/resync
// subscriber behavior mirroring internal/api/events.go's own proven
// semantics, redaction at the single publication point, and the
// guaranteed-terminal-event table covering all seven termination causes.
package script

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

func resetScriptEventsForTest(t *testing.T) {
	t.Helper()
	ResetScriptEventsForTesting()
	t.Cleanup(ResetScriptEventsForTesting)
}

// TestScriptEventBusPublishAssignsStrictlyIncreasingSeq covers: "publish
// assigns strictly increasing Seq values across all event kinds within a
// process."
func TestScriptEventBusPublishAssignsStrictlyIncreasingSeq(t *testing.T) {
	resetScriptEventsForTest(t)

	_, _, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()

	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "one"})
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventOutcome, Method: "two"})
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventStatus, Status: show.ScriptRunStatusRunning})

	var seqs []int64
	for i := 0; i < 3; i++ {
		select {
		case ev := <-ch:
			seqs = append(seqs, ev.Seq)
		case <-time.After(time.Second):
			require.FailNow(t, fmt.Sprintf("timed out waiting for event %d", i))
		}
	}
	for i, seq := range seqs {
		require.Equal(t, int64(i+1), seq, "seqs = %v, want strictly increasing starting at 1", seqs)
	}
}

// TestScriptEventBusLiveSubscriberReceivesEventsInOrderNoGaps covers: "A
// subscriber attached before a run receives every event in Seq order with
// no gaps."
func TestScriptEventBusLiveSubscriberReceivesEventsInOrderNoGaps(t *testing.T) {
	resetScriptEventsForTest(t)

	_, resync, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()
	require.False(t, resync, "a fresh subscriber (lastSeq<=0) must never be told to resync")

	for i := 0; i < 5; i++ {
		PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "line"})
	}

	var last int64
	for i := 0; i < 5; i++ {
		select {
		case ev := <-ch:
			require.Greater(t, ev.Seq, last, "event %d Seq=%d did not increase from %d", i, ev.Seq, last)
			last = ev.Seq
		case <-time.After(time.Second):
			require.FailNow(t, fmt.Sprintf("timed out waiting for event %d", i))
		}
	}
}

// TestScriptEventBusReconnectReplaysWithinWindow covers: "A subscriber
// reconnecting with a last-seen Seq still inside the ring receives a
// replay of everything after it."
func TestScriptEventBusReconnectReplaysWithinWindow(t *testing.T) {
	resetScriptEventsForTest(t)

	for i := 0; i < 5; i++ {
		PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "line"})
	}

	replay, resync, _, unsubscribe := SubscribeScriptEvents(2)
	defer unsubscribe()
	require.False(t, resync, "expected an in-window replay, not a resync")
	require.Len(t, replay, 3, "len(replay) = %d, want 3 (Seq 3,4,5)", len(replay))
	for i, ev := range replay {
		wantSeq := int64(3 + i)
		require.Equal(t, wantSeq, ev.Seq, "replay[%d].Seq = %d, want %d", i, ev.Seq, wantSeq)
	}
}

// TestScriptEventBusReconnectScrolledOutResyncsNoPartialReplay covers: "A
// subscriber reconnecting with a last-seen Seq that has scrolled out
// receives a resync signal carrying a reason, and no partial replay."
func TestScriptEventBusReconnectScrolledOutResyncsNoPartialReplay(t *testing.T) {
	resetScriptEventsForTest(t)
	original := ScriptEventRingCapacity
	ScriptEventRingCapacity = 3
	t.Cleanup(func() { ScriptEventRingCapacity = original })

	for i := 0; i < 10; i++ {
		PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "line"})
	}

	// The ring now holds only Seq 8,9,10 (capacity 3). Seq 1 has scrolled
	// out entirely.
	replay, resync, _, unsubscribe := SubscribeScriptEvents(1)
	defer unsubscribe()
	require.True(t, resync, "expected a resync for a scrolled-out Seq")
	require.Empty(t, replay, "expected no partial replay alongside a resync, got %d events", len(replay))
}

// TestScriptEventBusOverflowTriggersResyncAtMeasuredCapacity is the
// flagged SCRP-05 backstop truth: measures, rather than assumes, the
// exact ring capacity at which a fast-logging script begins triggering
// resync -- publishing exactly ScriptEventRingCapacity+1 events always
// scrolls the very first one out.
func TestScriptEventBusOverflowTriggersResyncAtMeasuredCapacity(t *testing.T) {
	resetScriptEventsForTest(t)
	original := ScriptEventRingCapacity
	ScriptEventRingCapacity = 8
	t.Cleanup(func() { ScriptEventRingCapacity = original })

	for i := 0; i < ScriptEventRingCapacity; i++ {
		PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "line"})
	}
	// At exactly ScriptEventRingCapacity published events, Seq 1 is still
	// the oldest retained entry -- no resync yet: the client has seen
	// everything up to (and including) the buffer's own oldest-1 boundary.
	_, resync, _, unsubscribe := SubscribeScriptEvents(1)
	unsubscribe()
	require.False(t, resync, "expected no resync at exactly ScriptEventRingCapacity published events")

	// Two more events push Seq 1 AND Seq 2 out of the ring (capacity 8
	// now retains Seq 3..10): a client still holding Seq 1 has a genuine
	// gap (it never saw Seq 2), which is the measured point resync must
	// trigger at -- exactly one event scrolling past a client's own
	// last-seen id is safe (no gap), but two is not.
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "n+1"})
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog, Message: "n+2"})
	_, resync, _, unsubscribe = SubscribeScriptEvents(1)
	defer unsubscribe()
	require.True(t, resync, "expected a resync once a genuine gap (two scrolled-out events) exists past the client's last-seen Seq")
}

// TestScriptEventBusPublishRedactsMessageAndReason covers: "Every
// published Message/Reason string has passed through security.Redact,"
// proving redaction happens inside publish itself, not at some later
// sink.
func TestScriptEventBusPublishRedactsMessageAndReason(t *testing.T) {
	resetScriptEventsForTest(t)

	_, _, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()

	PublishScriptEvent(ScriptEvent{
		Kind: ScriptEventLog, Message: "leaked: " + security.CanaryToken,
	})
	PublishScriptEvent(ScriptEvent{
		Kind: ScriptEventTerminal, Reason: "leaked: " + security.CanaryToken,
	})

	for i := 0; i < 2; i++ {
		select {
		case ev := <-ch:
			if ev.Kind == ScriptEventLog && (ev.Message == "" || contains(ev.Message, security.CanaryToken)) {
				require.FailNow(t, fmt.Sprintf("expected Message to be redacted, got %q", ev.Message))
			}
			if ev.Kind == ScriptEventTerminal && (ev.Reason == "" || contains(ev.Reason, security.CanaryToken)) {
				require.FailNow(t, fmt.Sprintf("expected Reason to be redacted, got %q", ev.Reason))
			}
		case <-time.After(time.Second):
			require.FailNow(t, "timed out waiting for event")
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestScriptEventBusResetGivesFreshSequence covers ResetScriptEventsForTesting.
func TestScriptEventBusResetGivesFreshSequence(t *testing.T) {
	resetScriptEventsForTest(t)
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog})
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog})

	ResetScriptEventsForTesting()

	_, _, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()
	PublishScriptEvent(ScriptEvent{Kind: ScriptEventLog})

	select {
	case ev := <-ch:
		require.Equal(t, int64(1), ev.Seq, "Seq = %d after reset, want 1", ev.Seq)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for event")
	}
}

// TestComputeTerminalEventEverySevenTerminationCauses is the plan's
// required table: enumerates all seven termination causes (success,
// failure, Stopped by user, deadline exceeded, rate exceeded, scope
// denied, and a Job-Object resource kill) and asserts computeTerminalEvent
// -- the exact function Run's guaranteed defer calls on every exit path --
// produces exactly one script.terminal event carrying the expected final
// status and reason for each.
func TestComputeTerminalEventEverySevenTerminationCauses(t *testing.T) {
	runID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7: %v", err)
	run := &Run{RunID: runID, ScriptName: "Chase"}

	cases := []struct {
		name       string
		result     RunOutcome
		runErr     error
		wantStatus show.ScriptRunStatus
		wantReason string
	}{
		{
			name:       "success",
			result:     RunOutcome{Status: show.ScriptRunStatusSucceeded},
			wantStatus: show.ScriptRunStatusSucceeded,
		},
		{
			name:       "failure",
			result:     RunOutcome{Status: show.ScriptRunStatusFailed, Reason: "script threw an error"},
			wantStatus: show.ScriptRunStatusFailed,
			wantReason: "script threw an error",
		},
		{
			name:       "stopped_by_user",
			result:     RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_STOPPED_BY_USER: stop requested"},
			wantStatus: show.ScriptRunStatusTerminated,
			wantReason: "GOLC_SCRIPT_STOPPED_BY_USER: stop requested",
		},
		{
			name:       "deadline_exceeded",
			result:     RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its deadline"},
			wantStatus: show.ScriptRunStatusTerminated,
			wantReason: "GOLC_SCRIPT_DEADLINE_EXCEEDED: run exceeded its deadline",
		},
		{
			name:       "rate_exceeded",
			result:     RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_RATE_EXCEEDED: run exceeded its rate limit"},
			wantStatus: show.ScriptRunStatusTerminated,
			wantReason: "GOLC_SCRIPT_RATE_EXCEEDED: run exceeded its rate limit",
		},
		{
			name:       "scope_denied",
			result:     RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_SCOPE_DENIED: method requires a wider scope"},
			wantStatus: show.ScriptRunStatusTerminated,
			wantReason: "GOLC_SCRIPT_SCOPE_DENIED: method requires a wider scope",
		},
		{
			// A Job-Object memory-limit kill is now a termination, not a
			// failure (08-14-PLAN.md Task 2): the proactive monitor records
			// a GOLC_SCRIPT_MEMORY_EXCEEDED TerminationReason via
			// beginTermination (branch 1 of the post-Wait chain), and the
			// post-exit classifyMemoryExhaustion backstop reclassifies the
			// same code in branch 4 when V8's own allocation failure wins
			// the race -- either way this bus's guarantee holds regardless
			// of which of Run's code paths actually set the final
			// Status/Reason.
			name:       "jobobject_resource_kill",
			result:     RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit"},
			wantStatus: show.ScriptRunStatusTerminated,
			wantReason: "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := computeTerminalEvent(run, tc.result, tc.runErr)
			require.Equal(t, ScriptEventTerminal, ev.Kind, "Kind = %q, want %q", ev.Kind, ScriptEventTerminal)
			if ev.RunID != run.RunID || ev.ScriptName != run.ScriptName {
				require.FailNow(t, fmt.Sprintf("expected RunID/ScriptName to be carried, got %+v", ev))
			}
			require.Equal(t, tc.wantStatus, ev.Status, "Status = %q, want %q", ev.Status, tc.wantStatus)
			require.Equal(t, tc.wantReason, ev.Reason, "Reason = %q, want %q", ev.Reason, tc.wantReason)
		})
	}
}

// TestComputeTerminalEventEarlyFailureBeforeDispatchStillPublishes covers
// the case a real early return (e.g. GOLC_SCRIPT_RUN_TEMP_DIR_FAILED)
// exercises in production: result is the RunOutcome{} zero value and
// runErr is non-nil -- computeTerminalEvent must still produce a
// meaningful terminal event (Status defaults to Failed, Reason comes from
// runErr), never an empty/blank one.
func TestComputeTerminalEventEarlyFailureBeforeDispatchStillPublishes(t *testing.T) {
	runID, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7: %v", err)
	run := &Run{RunID: runID, ScriptName: "Chase"}

	ev := computeTerminalEvent(run, RunOutcome{}, errWithMessage("GOLC_SCRIPT_RUN_TEMP_DIR_FAILED: disk full"))
	require.Equal(t, show.ScriptRunStatusFailed, ev.Status, "Status = %q, want %q for an early failure", ev.Status, show.ScriptRunStatusFailed)
	require.Equal(t, "GOLC_SCRIPT_RUN_TEMP_DIR_FAILED: disk full", ev.Reason, "Reason = %q, want the early failure's own message", ev.Reason)
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func errWithMessage(msg string) error { return simpleErr(msg) }

// TestPublishScriptEventTerminalPublishesExactlyOnePerCause proves the
// same seven-cause table actually publishes exactly one script.terminal
// event onto the bus each time -- not just that computeTerminalEvent
// builds the right value, but that PublishScriptEvent(computeTerminalEvent(...))
// (the exact call Run's guaranteed defer makes) results in exactly one
// observable event per run.
func TestPublishScriptEventTerminalPublishesExactlyOnePerCause(t *testing.T) {
	causes := []struct {
		name   string
		result RunOutcome
		runErr error
	}{
		{"success", RunOutcome{Status: show.ScriptRunStatusSucceeded}, nil},
		{"failure", RunOutcome{Status: show.ScriptRunStatusFailed, Reason: "threw"}, nil},
		{"stopped_by_user", RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_STOPPED_BY_USER: x"}, nil},
		{"deadline_exceeded", RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_DEADLINE_EXCEEDED: x"}, nil},
		{"rate_exceeded", RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_RATE_EXCEEDED: x"}, nil},
		{"scope_denied", RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_SCOPE_DENIED: x"}, nil},
		{"jobobject_resource_kill", RunOutcome{Status: show.ScriptRunStatusTerminated, Reason: "GOLC_SCRIPT_MEMORY_EXCEEDED: x"}, nil},
	}

	for _, tc := range causes {
		t.Run(tc.name, func(t *testing.T) {
			resetScriptEventsForTest(t)
			runID, err := uuid.NewV7()
			require.NoError(t, err, "uuid.NewV7: %v", err)
			run := &Run{RunID: runID, ScriptName: "Chase"}

			_, _, ch, unsubscribe := SubscribeScriptEvents(0)
			defer unsubscribe()

			PublishScriptEvent(computeTerminalEvent(run, tc.result, tc.runErr))

			var terminals []ScriptEvent
			select {
			case ev := <-ch:
				terminals = append(terminals, ev)
			case <-time.After(time.Second):
				require.FailNow(t, "timed out waiting for the terminal event")
			}
			select {
			case ev := <-ch:
				terminals = append(terminals, ev)
			default:
			}
			require.Len(t, terminals, 1, "expected exactly one terminal event, got %d: %+v", len(terminals), terminals)
			require.Equal(t, ScriptEventTerminal, terminals[0].Kind, "Kind = %q, want %q", terminals[0].Kind, ScriptEventTerminal)
		})
	}
}

// TestRunDispatchIOPublishesLogAndOutcomeScriptEvents proves session.go's
// runDispatchIO publishes a script.log event for every captured log line
// (both stdout LogFrame and stderr) and a script.outcome event for every
// dispatched CmdCallFrame -- driven against io.Pipe()-based fakes exactly
// like session_test.go's own TestRunDispatchIOEndToEnd, so no real Deno
// process is needed.
func TestRunDispatchIOPublishesLogAndOutcomeScriptEvents(t *testing.T) {
	resetScriptEventsForTest(t)

	exec := &fakeExecutor{}
	h := &Host{cfg: HostConfig{Root: "/repo", ShowPath: "/repo/show.golc", Executor: exec}}
	run := mustNewRun(t)

	_, _, ch, unsubscribe := SubscribeScriptEvents(0)
	defer unsubscribe()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	go func() {
		defer stdinR.Close()
		_, _ = io.Copy(io.Discard, stdinR)
	}()
	go func() {
		defer stderrW.Close()
	}()
	go func() {
		defer stdoutW.Close()
		_ = EncodeFrame(stdoutW, ReadyFrame{})
		_ = EncodeFrame(stdoutW, LogFrame{Level: "info", Message: "hello"})
		_ = EncodeFrame(stdoutW, CmdCallFrame{ID: "c1", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`)})
		_ = EncodeFrame(stdoutW, DoneFrame{ExitReason: "completed"})
	}()

	h.runDispatchIO(run, stdinW, stdoutR, stderrR)

	var sawLog, sawOutcome bool
	timeout := time.After(2 * time.Second)
	for !sawLog || !sawOutcome {
		select {
		case ev := <-ch:
			if ev.Kind == ScriptEventLog && ev.Message == "hello" {
				sawLog = true
			}
			if ev.Kind == ScriptEventOutcome && ev.Route == "scene activate" {
				sawOutcome = true
			}
		case <-timeout:
			require.FailNow(t, fmt.Sprintf("timed out waiting for both a script.log and script.outcome event (sawLog=%v sawOutcome=%v)", sawLog, sawOutcome))
		}
	}
}
