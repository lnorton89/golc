// debugbridge.go implements D-01/D-02's real interactive debugger
// (08-09-PLAN.md Task 2, CONTEXT SCRP-01/SCRP-05): the Go daemon's own,
// SOLE Chrome DevTools/V8 Inspector Protocol client against a Debug-mode
// script's loopback inspector, translating breakpoint/step control-plane
// calls into CDP Debugger.*/Runtime.* calls and CDP Debugger.paused/
// Runtime.exceptionThrown events into script.ScriptEvent values on the
// exact same bus 08-08 established (never a second streaming mechanism).
//
// No exported method on DebugBridge returns a port, a WebSocket URL, or
// a raw CDP frame -- the browser/webview never holds a CDP connection of
// its own (08-RESEARCH.md's "Letting the frontend hold a direct CDP/
// inspector WebSocket" anti-pattern, T-08-40's mitigation). Every
// reported CDP position is corrected through stacktrace.go's correctLine
// -- the exact same shim-offset math parseStackTrace uses for Deno's own
// textual traces -- so a paused line and a crash's stack trace always
// agree on the author's own source coordinates.
package script

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/debugger"
	"github.com/mafredri/cdp/protocol/runtime"
	"github.com/mafredri/cdp/rpcc"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// debugConnectTimeout bounds how long NewDebugBridge waits for the
// spawned Deno child's inspector HTTP endpoint to come up: the child has
// just been exec'd when this runs, so its inspector listener may not be
// accepting connections on the very first attempt -- waitForInspectorTarget
// polls until this deadline rather than treating the first failed attempt
// as fatal.
const debugConnectTimeout = 5 * time.Second

// debugConnectPollInterval is the pause between successive
// waitForInspectorTarget polling attempts.
const debugConnectPollInterval = 50 * time.Millisecond

// DebugBridge is the Go daemon's sole CDP client for one Debug-mode run.
// It owns the loopback inspector's WebSocket connection, the run's
// identity and shim offset (both needed to translate every CDP position
// into the author's own source coordinates via correctLine), and the
// run's current paused/resumed state. Nothing on this type is copyable
// safely across goroutines except through its own methods.
type DebugBridge struct {
	runID         uuid.UUID
	scriptName    string
	shimLineCount int

	conn   *rpcc.Conn
	client *cdp.Client

	pausedEvents debugger.PausedClient
	exceptions   runtime.ExceptionThrownClient

	mu             sync.Mutex
	closed         bool
	paused         bool
	disconnectOnce sync.Once

	pumpWG sync.WaitGroup
}

// waitForInspectorTarget polls the loopback inspector's devtools target
// list (Deno implements the same /json/list HTTP surface Chrome/Node's
// inspector does, reporting itself as a devtool.Node-typed target) until
// a target with a non-empty WebSocketDebuggerURL appears or ctx is done --
// absorbing the brief startup race between cmd.Start() returning and the
// child's own inspector listener actually accepting connections.
//
// This deliberately does NOT poll /json/version: that endpoint reports
// only {Browser, Protocol-Version, V8-Version} on Deno 2.9.4 and never
// carries a WebSocketDebuggerURL at all (confirmed directly against the
// pinned toolchain during 08-13's acceptance pass -- deferred-items.md's
// 08-13 section), so a version-based wait can never succeed; it always
// runs out the connect timeout even though the inspector is already
// live and accepting connections, which is exactly the deadlock this
// plan's Task 1/2 acceptance checkpoints hit before this fix.
func waitForInspectorTarget(ctx context.Context, port int) (*devtool.Target, error) {
	devt := devtool.New(fmt.Sprintf("http://127.0.0.1:%d", port))
	var lastErr error
	for {
		target, err := devt.Get(ctx, devtool.Node)
		if err == nil && target.WebSocketDebuggerURL != "" {
			return target, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("inspector reported no webSocketDebuggerUrl")
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%v (last attempt: %v)", ctx.Err(), lastErr)
		case <-time.After(debugConnectPollInterval):
		}
	}
}

// NewDebugBridge dials the loopback inspector at 127.0.0.1:port, enables
// the Debugger and Runtime domains, and subscribes to Debugger.paused and
// Runtime.exceptionThrown -- the Go daemon's only CDP connection for this
// run. It returns a GOLC_SCRIPT_DEBUG_CONNECT_FAILED error (never a raw
// CDP/rpcc error type) on any failure in this sequence, and leaves no
// partially-open connection behind on any exit path.
func NewDebugBridge(ctx context.Context, runID uuid.UUID, port int, shimLineCount int, scriptName string) (*DebugBridge, error) {
	connectCtx, cancel := context.WithTimeout(ctx, debugConnectTimeout)
	defer cancel()

	target, err := waitForInspectorTarget(connectCtx, port)
	if err != nil {
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: resolve inspector target: %v", err)
	}

	conn, err := rpcc.DialContext(connectCtx, target.WebSocketDebuggerURL)
	if err != nil {
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: dial inspector: %v", err)
	}

	client := cdp.NewClient(conn)

	pausedEvents, err := client.Debugger.Paused(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: subscribe Debugger.paused: %v", err)
	}
	exceptions, err := client.Runtime.ExceptionThrown(ctx)
	if err != nil {
		_ = pausedEvents.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: subscribe Runtime.exceptionThrown: %v", err)
	}

	if _, err := client.Debugger.Enable(ctx, debugger.NewEnableArgs()); err != nil {
		_ = exceptions.Close()
		_ = pausedEvents.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: Debugger.enable: %v", err)
	}
	if err := client.Runtime.Enable(ctx); err != nil {
		_ = exceptions.Close()
		_ = pausedEvents.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("GOLC_SCRIPT_DEBUG_CONNECT_FAILED: Runtime.enable: %v", err)
	}

	bridge := &DebugBridge{
		runID: runID, scriptName: scriptName, shimLineCount: shimLineCount,
		conn: conn, client: client,
		pausedEvents: pausedEvents, exceptions: exceptions,
	}

	bridge.pumpWG.Add(2)
	go bridge.pumpPausedEvents()
	go bridge.pumpExceptionEvents()

	return bridge, nil
}

// materializedCDPLine converts an author-coordinate (1-based) source line
// into the 0-based CDP line number the materialized shim+source file
// actually occupies -- the exact inverse of correctLine (stacktrace.go).
func materializedCDPLine(authorLine, shimLineCount int) int {
	return authorLine + shimLineCount - 1
}

// authorLineFromCDP converts a 0-based CDP line number (as reported by a
// Debugger.paused call frame or a Runtime.exceptionThrown location) back
// into the author's own 1-based source coordinate, delegating to
// stacktrace.go's correctLine so this file's and parseStackTrace's
// shim-offset correction can never independently drift.
func authorLineFromCDP(cdpLine, shimLineCount int) (userLine int, inShim bool) {
	return correctLine(cdpLine+1, shimLineCount)
}

// publishDebugStatus publishes one script.status ScriptEvent carrying a
// debug-session sub-state marker in Reason (D-01/D-02's "paused,
// resumed, breakpoint hit with line number, stepped, exception" states,
// reusing the exact ScriptEventStatus kind 08-08 already established --
// never a second streaming mechanism or a new ScriptEventKind).
func (b *DebugBridge) publishDebugStatus(reason string) {
	PublishScriptEvent(ScriptEvent{
		Kind: ScriptEventStatus, RunID: b.runID, ScriptName: b.scriptName, At: time.Now(),
		Status: show.ScriptRunStatusRunning, Reason: security.Redact(reason),
	})
}

// SetBreakpoints translates every author-coordinate line in authorLines
// into its shim-offset-corrected 0-based CDP line number and issues one
// Debugger.setBreakpointByUrl call per line, then resumes execution from
// Debug mode's initial break-on-first-line pause via
// Runtime.runIfWaitingForDebugger -- so every UI-configured breakpoint is
// registered before the author's own first line ever executes (D-01,
// matching the break-on-first-line launch Task 1's buildDenoArgs uses).
// The urlRegex matches only this run's own runID, embedded verbatim in
// the materialized script's temp filename (session.go's
// runID.String()+".ts") -- never a bare ".*". A universal wildcard
// matches every script V8 parses in this process, including Deno's own
// internal bootstrap/ext:core modules, and setting a breakpoint against
// one of those can pause execution inside Deno's own startup code
// instead of the author's line (found during 08-13's acceptance pass --
// deferred-items.md's 08-13 section). Scoping the regex is regexp-safe
// via regexp.QuoteMeta and needs no anchors: CDP's urlRegex is a
// substring test, and a UUID string can only ever appear in the one
// script whose own filename embeds it (the exact materialized temp path
// itself is still never surfaced outside this package, consistent with
// T-08-42 -- only its already-public runID is used here).
func (b *DebugBridge) SetBreakpoints(authorLines []int) error {
	ctx := context.Background()
	urlRegex := regexp.QuoteMeta(b.runID.String())
	for _, authorLine := range authorLines {
		cdpLine := materializedCDPLine(authorLine, b.shimLineCount)
		args := debugger.NewSetBreakpointByURLArgs(cdpLine).SetURLRegex(urlRegex)
		if _, err := b.client.Debugger.SetBreakpointByURL(ctx, args); err != nil {
			return fmt.Errorf("GOLC_SCRIPT_DEBUG_BREAKPOINT_FAILED: line %d: %v", authorLine, err)
		}
	}
	if err := b.client.Runtime.RunIfWaitingForDebugger(ctx); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_DEBUG_BREAKPOINT_FAILED: resume from initial break: %v", err)
	}
	b.publishDebugStatus("GOLC_SCRIPT_DEBUG_RESUMED: initial break, breakpoints set")
	return nil
}

// Continue issues Debugger.resume and publishes the resulting state
// transition.
func (b *DebugBridge) Continue() error {
	if err := b.client.Debugger.Resume(context.Background(), nil); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_DEBUG_CONTROL_FAILED: continue: %v", err)
	}
	b.setPaused(false)
	b.publishDebugStatus("GOLC_SCRIPT_DEBUG_RESUMED")
	return nil
}

// StepOver issues Debugger.stepOver and publishes the resulting state
// transition.
func (b *DebugBridge) StepOver() error {
	if err := b.client.Debugger.StepOver(context.Background(), nil); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_DEBUG_CONTROL_FAILED: step-over: %v", err)
	}
	b.setPaused(false)
	b.publishDebugStatus("GOLC_SCRIPT_DEBUG_STEPPED: step-over")
	return nil
}

// StepInto issues Debugger.stepInto and publishes the resulting state
// transition.
func (b *DebugBridge) StepInto() error {
	if err := b.client.Debugger.StepInto(context.Background(), nil); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_DEBUG_CONTROL_FAILED: step-into: %v", err)
	}
	b.setPaused(false)
	b.publishDebugStatus("GOLC_SCRIPT_DEBUG_STEPPED: step-into")
	return nil
}

// StepOut issues Debugger.stepOut and publishes the resulting state
// transition.
func (b *DebugBridge) StepOut() error {
	if err := b.client.Debugger.StepOut(context.Background()); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_DEBUG_CONTROL_FAILED: step-out: %v", err)
	}
	b.setPaused(false)
	b.publishDebugStatus("GOLC_SCRIPT_DEBUG_STEPPED: step-out")
	return nil
}

// setPaused records b's current paused sub-state under mu.
func (b *DebugBridge) setPaused(paused bool) {
	b.mu.Lock()
	b.paused = paused
	b.mu.Unlock()
}

// handlePaused translates one Debugger.paused event into a script.status
// ScriptEvent carrying the paused author-coordinate line number (D-01: a
// hit breakpoint publishes a paused state with that line number) --
// covers every pause reason CDP reports, including the initial
// break-on-first-line pause SetBreakpoints itself resumes from.
func (b *DebugBridge) handlePaused(reply *debugger.PausedReply) {
	b.setPaused(true)

	if len(reply.CallFrames) == 0 {
		b.publishDebugStatus(fmt.Sprintf("GOLC_SCRIPT_DEBUG_PAUSED: reason=%s", reply.Reason))
		return
	}
	userLine, inShim := authorLineFromCDP(reply.CallFrames[0].Location.LineNumber, b.shimLineCount)
	if inShim {
		b.publishDebugStatus(fmt.Sprintf("GOLC_SCRIPT_DEBUG_PAUSED: reason=%s, %s", reply.Reason, shimErrorMarker))
		return
	}
	b.publishDebugStatus(fmt.Sprintf("GOLC_SCRIPT_DEBUG_PAUSED: line=%d, reason=%s", userLine, reply.Reason))
}

// framesFromCDPCallFrames renders CDP's runtime.CallFrame slice (as
// carried on a Runtime.exceptionThrown event's StackTrace) into
// []StackFrame using the exact same author-coordinate correction and
// temp-path-never-leaks discipline parseStackTrace applies to Deno's own
// textual traces (stacktrace.go).
func framesFromCDPCallFrames(frames []runtime.CallFrame, shimLineCount int, scriptName string) []StackFrame {
	out := make([]StackFrame, 0, len(frames))
	for _, frame := range frames {
		function := frame.FunctionName
		if function == "" {
			function = "<anonymous>"
		}
		function = security.Redact(function)
		file := security.Redact(scriptName)

		userLine, inShim := authorLineFromCDP(frame.LineNumber, shimLineCount)
		if inShim {
			out = append(out, StackFrame{Function: function + ": " + shimErrorMarker, File: file, Line: 0, Column: frame.ColumnNumber})
			continue
		}
		out = append(out, StackFrame{Function: function, File: file, Line: userLine, Column: frame.ColumnNumber})
	}
	return out
}

// formatExceptionMessage renders headerText plus every frame as a single
// multi-line, already-redacted message string -- the shape
// publishException hands to ScriptEvent.Message, since ScriptEvent has no
// dedicated stack-frame-array field (events.go, out of this task's own
// file scope; reusing the existing flat ScriptEvent shape rather than
// extending it keeps this task from touching 08-08's file).
func formatExceptionMessage(headerText string, frames []StackFrame) string {
	message := security.Redact(headerText)
	for _, frame := range frames {
		message += fmt.Sprintf("\n    at %s (%s:%d:%d)", frame.Function, frame.File, frame.Line, frame.Column)
	}
	return message
}

// handleExceptionThrown translates one Runtime.exceptionThrown event into
// a script.log ScriptEvent carrying the source-mapped stack frames (D-03,
// this task's exact <behavior> bullet: "a thrown exception publishes a
// script.terminal-adjacent event carrying the source-mapped stack frames
// from Task 1").
func (b *DebugBridge) handleExceptionThrown(reply *runtime.ExceptionThrownReply) {
	var frames []StackFrame
	if reply.ExceptionDetails.StackTrace != nil {
		frames = framesFromCDPCallFrames(reply.ExceptionDetails.StackTrace.CallFrames, b.shimLineCount, b.scriptName)
	}
	message := formatExceptionMessage(reply.ExceptionDetails.Text, frames)
	PublishScriptEvent(ScriptEvent{
		Kind: ScriptEventLog, RunID: b.runID, ScriptName: b.scriptName, At: time.Now(),
		Level: "exception", Message: message, Source: "debugger",
	})
}

// pumpPausedEvents blocks on Debugger.paused.Recv() until the stream
// closes (Close() unsubscribing it, or the connection dropping
// unexpectedly), translating every received event via handlePaused.
func (b *DebugBridge) pumpPausedEvents() {
	defer b.pumpWG.Done()
	for {
		reply, err := b.pausedEvents.Recv()
		if err != nil {
			b.reportUnexpectedDisconnect(err)
			return
		}
		b.handlePaused(reply)
	}
}

// pumpExceptionEvents blocks on Runtime.exceptionThrown.Recv() until the
// stream closes, translating every received event via
// handleExceptionThrown.
func (b *DebugBridge) pumpExceptionEvents() {
	defer b.pumpWG.Done()
	for {
		reply, err := b.exceptions.Recv()
		if err != nil {
			b.reportUnexpectedDisconnect(err)
			return
		}
		b.handleExceptionThrown(reply)
	}
}

// reportUnexpectedDisconnect records GOLC_SCRIPT_DEBUG_DISCONNECTED
// exactly once (across both pump goroutines) when a subscription stream
// ends for a reason other than this bridge's own Close() -- T-08-44's
// accepted-severity DoS disposition: the run continues to its normal
// termination under the Job Object and deadline regardless, so this is
// an observability signal, never a second kill path.
func (b *DebugBridge) reportUnexpectedDisconnect(err error) {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return
	}
	b.disconnectOnce.Do(func() {
		PublishScriptEvent(ScriptEvent{
			Kind: ScriptEventLog, RunID: b.runID, ScriptName: b.scriptName, At: time.Now(),
			Level: "error", Message: security.Redact(fmt.Sprintf("GOLC_SCRIPT_DEBUG_DISCONNECTED: %v", err)), Source: "debugger",
		})
	})
}

// Close unsubscribes from every CDP event stream and closes the
// underlying connection -- idempotent, so both an explicit termination
// path (session.go's Run.terminate) and Run's own deferred cleanup may
// call it safely. It never returns a port, URL, or raw CDP frame, and it
// never itself decides whether the child process is killed: closing the
// CDP connection and closing the Job Object are session.go's two
// separate, ordered steps (this bridge only ever performs the first).
func (b *DebugBridge) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	_ = b.pausedEvents.Close()
	_ = b.exceptions.Close()
	err := b.conn.Close()
	b.pumpWG.Wait()
	return err
}
