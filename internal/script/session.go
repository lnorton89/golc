// session.go implements (*Host).Run: spawn a fresh, zero-permission Deno
// subprocess for one script run, drive the multiplexed session protocol
// (protocol.go) over its stdio, dispatch every cmd-call frame to the
// injected Executor, and return the run's outcome (08-05-PLAN.md Task 2,
// CONTEXT SCRP-01/SCRP-02/SCRP-03).
//
// The frame-dispatch loop (runDispatchIO/dispatchCmdCall) is deliberately
// decoupled from process spawning: it operates on any io.Writer/io.Reader
// pair, so host_test.go/session_test.go can exercise every dispatch
// behavior (unknown method, successful call, param decode failure)
// against io.Pipe()-based fakes and a fake Executor, without ever
// spawning a real Deno process. Only the tests that need to observe an
// actual OS-process boundary (materialized script file, temp-dir
// cleanup, real stdio) spawn Deno for real, gated behind a helper that
// skips when .tools/toolchains/deno/ is not provisioned.
package script

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/api"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/show"
)

// maxCapturedLogLines bounds Run's captured log stream (D-04/T-08-20): the
// oldest entries drop first once this cap is reached, mirroring
// process.go's drop-oldest-first boundedBuffer discipline applied to
// whole LogLine entries rather than raw bytes.
const maxCapturedLogLines = 1000

// stderrTailBudget bounds how many trailing stderr bytes a run retains
// for its failure-diagnostic tail (T-08-20), mirroring
// internal/trace/transport/process.go's defaultStderrBudget exactly.
const stderrTailBudget = 8192

// runCompletionTrailerTS is appended after target.Source (never before
// it, so it never shifts shimLineOffsetFor's line-number math or any
// breakpoint/stack-trace mapping, all of which are anchored to the shim
// that precedes the user's own source). scriptsdk.RuntimeShimTS's own
// __golcStartReader keeps a `for await (const chunk of Deno.stdin.
// readable)` loop pinned open for the life of the process -- by design,
// so a script can make cmd-calls at any point -- but that same open
// stream means Deno's event loop never drains on its own once the
// user's top-level code finishes: nothing ever signals completion, and
// runDispatchIO (session.go) keeps reading until the run's own deadline
// kills it (found during 08-13's acceptance pass against the real
// pinned Deno toolchain -- deferred-items.md's 08-13 section -- every
// prior test of a real run happened to skip because Deno was never
// actually provisioned until then).
//
// This trailer is textually the last thing in the materialized file, so
// module evaluation only reaches it once every top-level statement in
// the user's own source -- including any top-level `await` on a golc.*
// call -- has settled. It writes the DoneFrame protocol.go's
// runDispatchIO already listens for (and has since 08-05, unused until
// now) and then force-exits, since nothing else will ever close the
// process on its own. A script that throws is unaffected: an uncaught
// top-level exception aborts module evaluation before reaching this
// trailer, and the process's own non-zero exit already closes stdout
// (runDispatchIO's existing !ok break) -- Run's cmd.Wait() error and the
// captured stderr tail are what report that case, exactly as before.
const runCompletionTrailerTS = `await (globalThis as any).Deno.stdout.write(__golcEncoder.encode(JSON.stringify({ kind: "done" }) + "\n"));
(globalThis as any).Deno.exit(0);
`

// Run is one script execution's identity: a fresh, distinct RunID every
// time (D-13: two sequential runs of the same script are always
// independent, never resuming state from a prior run).
type Run struct {
	RunID      uuid.UUID
	ScriptName string
	Profile    show.CapabilityProfile
	StartedAt  time.Time

	// terminationMu guards termination/cancel/job below: multiple
	// goroutines can race to observe or set them (the dispatch loop, a
	// deadline timer, and -- once 08-06 Task 3 exists -- a "script stop"
	// caller running on a completely different goroutine).
	terminationMu sync.Mutex
	// termination is nil until the run's first hard-termination cause
	// (scope violation, rate overrun, deadline, or an explicit Stop) is
	// recorded; once set it is never overwritten (D-11: the exact
	// TerminationReason a script sees for every call after the instant
	// termination begins is the one that began it).
	termination *TerminationReason
	// cancel is Run's own context.CancelFunc, wired the moment Run
	// derives its deadline-bound context: calling it is this run's
	// always-present kill-path fallback -- covers the window before a Job
	// Object has been assigned, and is the entire kill mechanism on a
	// non-Windows/no-op jobObject.
	cancel context.CancelFunc
	// job is run's assigned Windows Job Object (08-06-PLAN.md Task 2):
	// closing it kills the child unconditionally
	// (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, SCRP-06) -- the primary,
	// kernel-enforced kill path; terminate() closes it before falling back
	// to cancel(). nil until Run assigns it, immediately after cmd.Start().
	job *jobObject
	// bridge is run's CDP debug bridge (08-09-PLAN.md Task 2), non-nil
	// only for a LaunchModeDebug run whose bridge connected successfully.
	// terminate() closes it before job, so a debug run's termination
	// sequence is always "close the CDP connection, then the Job
	// Object" -- a script paused at a breakpoint is not exempt from
	// Stop, the deadline, or any resource limit; pausing changes nothing
	// about how or when this run can be killed.
	bridge *DebugBridge

	// done is closed exactly once, by Run, the moment run's final
	// RunOutcome (below) is ready -- Stop (registry.go) blocks on this so
	// a caller always observes the run's true final outcome, including
	// any in-flight command's recorded result (D-11), rather than a
	// status snapshot taken mid-shutdown.
	done chan struct{}
	// outcome is run's final RunOutcome, written by Run exactly once
	// before done is closed. Never read before <-done unblocks.
	outcome RunOutcome
}

// beginTermination records reason as run's TerminationReason if none is
// set yet (first writer wins -- D-11: the reason a script observes is
// always the one that actually began termination, never a later,
// possibly-unrelated cause racing in afterward) and reports whether this
// call actually set it.
func (r *Run) beginTermination(reason TerminationReason) bool {
	r.terminationMu.Lock()
	defer r.terminationMu.Unlock()
	if r.termination != nil {
		return false
	}
	r.termination = &reason
	return true
}

// terminationReason returns run's recorded TerminationReason, if any.
func (r *Run) terminationReason() (TerminationReason, bool) {
	r.terminationMu.Lock()
	defer r.terminationMu.Unlock()
	if r.termination == nil {
		return TerminationReason{}, false
	}
	return *r.termination, true
}

// terminate triggers run's kill path: closing its CDP debug bridge (if
// any) first, then its assigned Job Object -- the primary,
// kernel-enforced, uninterceptable kill (SCRP-06), which on Windows is a
// strict superset of a process-tree kill -- and only then cancelling its
// context as an always-present fallback (the window before job
// assignment completes, and the entire mechanism on a non-Windows/no-op
// jobObject, whose own Close already does the real kill work). A script
// paused at a breakpoint is killed exactly like a running one: closing
// the bridge never pauses or delays the Job Object close that follows it
// (08-09-PLAN.md Task 2 -- pausing must never become a way to hold a run
// open past any limit).
func (r *Run) terminate() {
	r.terminationMu.Lock()
	bridge := r.bridge
	job := r.job
	cancel := r.cancel
	r.terminationMu.Unlock()
	if bridge != nil {
		_ = bridge.Close()
	}
	if job != nil {
		_ = job.Close()
	}
	if cancel != nil {
		cancel()
	}
}

// activeRuns is the process-global registry of currently active runs,
// keyed by script name -- shared by every *Host a caller happens to
// construct (each "script run"/"script stop" CLI invocation builds its
// own Host, per internal/command/scriptrun.go/scriptstop.go), so a
// wholly separate "script stop" invocation (08-06-PLAN.md Task 3) can
// locate and terminate a run started by a different Host instance within
// the same process, without requiring a persistent, singleton Host or
// daemon-side IPC. Run registers itself here at the start of Run and
// deregisters on every exit path, including panic (via defer).
var activeRuns = struct {
	mu     sync.Mutex
	byName map[string]*Run
}{byName: map[string]*Run{}}

// registerActiveRun records run as the currently active run for its
// ScriptName.
func registerActiveRun(run *Run) {
	activeRuns.mu.Lock()
	defer activeRuns.mu.Unlock()
	activeRuns.byName[run.ScriptName] = run
}

// deregisterActiveRun removes run from the registry, but only if it is
// still the exact run recorded under its name -- guards against a
// pathological double-deregister racing a fresh run of the same name.
func deregisterActiveRun(run *Run) {
	activeRuns.mu.Lock()
	defer activeRuns.mu.Unlock()
	if activeRuns.byName[run.ScriptName] == run {
		delete(activeRuns.byName, run.ScriptName)
	}
}

// ActiveRun returns the currently active run for scriptName, if any --
// the seam internal/command/scriptstop.go (08-06-PLAN.md Task 3) uses to
// resolve "script stop <name>"'s target. Stop only ever needs to
// terminate an already-running process; it never needs its own Host
// instance or Executor.
func ActiveRun(scriptName string) (*Run, bool) {
	activeRuns.mu.Lock()
	defer activeRuns.mu.Unlock()
	run, found := activeRuns.byName[scriptName]
	return run, found
}

// AnyActiveRun returns the process's sole active run, if any -- the seam
// internal/command/scriptdebug.go's continue/step-over/step-into/
// step-out routes (08-09-PLAN.md Task 3) use to resolve "the single
// active debug run" without a --name flag. This is sound only because of
// 08-05's v1 "at most one active run, globally" scope call: at most one
// entry can ever exist in activeRuns.byName at a time, so returning
// whichever single entry is present IS returning the one active run, not
// an arbitrary pick among several.
func AnyActiveRun() (*Run, bool) {
	activeRuns.mu.Lock()
	defer activeRuns.mu.Unlock()
	for _, run := range activeRuns.byName {
		return run, true
	}
	return nil, false
}

// Name returns run's owning script name -- read-only accessor for
// callers outside this package (internal/command/scriptstop.go) that
// hold a *Run from ActiveRun but must not reach into its unexported
// fields.
func (r *Run) Name() string {
	return r.ScriptName
}

// ID returns run's RunID.
func (r *Run) ID() uuid.UUID {
	return r.RunID
}

// Bridge returns run's CDP debug bridge, or nil if run is not a Debug
// launch or its bridge never connected -- the seam
// internal/command/scriptdebug.go's continue/step-over/step-into/
// step-out routes use to resolve their single active debug run's control
// surface (08-09-PLAN.md Task 3), without reaching into any other
// unexported Run field.
func (r *Run) Bridge() *DebugBridge {
	r.terminationMu.Lock()
	defer r.terminationMu.Unlock()
	return r.bridge
}

// Stop terminates run with reason and blocks until Run's own goroutine
// observes the termination and returns, so the caller always receives
// the run's true final outcome -- including any in-flight command's
// recorded result (D-11: "any command already accepted is allowed to
// finish") -- rather than a status snapshot taken mid-shutdown.
func (r *Run) Stop(reason TerminationReason) RunOutcome {
	r.beginTermination(reason)
	r.terminate()
	<-r.done
	return r.outcome
}

// LogLine is one captured, already-redacted stdout/stderr line (D-04's
// live log/diagnostics stream).
type LogLine struct {
	Level   string
	Message string
	Source  string
}

// CallOutcome is one SDK call's recorded outcome (D-05: every command
// outcome appears individually in the script's own debug panel in real
// time, in addition to Phase 7's API-06 audit trail).
type CallOutcome struct {
	Method     string
	Route      string
	DurationMS int64
	Ok         bool
	Code       string
	Message    string
}

// RunOutcome is (*Host).Run's return value: the run's final status, every
// captured log line, and every SDK call's outcome.
type RunOutcome struct {
	RunID    uuid.UUID
	Status   show.ScriptRunStatus
	Reason   string
	Logs     []LogLine
	Outcomes []CallOutcome
}

// boundedBuffer retains at most limit trailing bytes written to it,
// dropping the oldest bytes first -- copied verbatim from
// internal/trace/transport/process.go's boundedBuffer (T-08-20): used
// only for a bounded stderr diagnostic tail, never for protocol data.
type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newBoundedBuffer(limit int) *boundedBuffer {
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = b.data[len(b.data)-b.limit:]
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

// appendBoundedLog appends line to lines, dropping the oldest entry once
// maxCapturedLogLines is exceeded (T-08-20: captured output beyond the
// bounded buffer's capacity drops oldest bytes rather than growing
// without bound).
func appendBoundedLog(lines []LogLine, line LogLine) []LogLine {
	lines = append(lines, line)
	if len(lines) > maxCapturedLogLines {
		lines = lines[len(lines)-maxCapturedLogLines:]
	}
	return lines
}

// enforce is the seam 08-05 left as an always-allow stub and 08-06 Task 1
// fills with D-06/D-08 host-side capability-scope and rate enforcement,
// inserted between the method lookup and the Executor call: it calls
// Enforce (capability.go) with run's own profile/RunID against
// descriptor's route, using h.limiter as the run's own rate bucket. A
// non-nil TerminationReason is recorded on run (first writer wins,
// beginTermination) and triggers run's kill path before being returned
// to the caller, which never lets the call reach the Executor.
func (h *Host) enforce(descriptor scriptsdk.SDKMethodDescriptor, run *Run) *TerminationReason {
	reason := Enforce(run.Profile, run.RunID, descriptor.Route, h.limiter)
	if reason != nil {
		run.beginTermination(*reason)
		run.terminate()
	}
	return reason
}

// methodDescriptorsByRoute builds a lookup from scriptsdk.
// RegisteredSDKMethods() keyed by the exact Route string every
// CmdCallFrame.Method carries (protocol.go's doc comment: despite the
// field's name, Method carries the Route string, not the TypeScript
// dot-path method name golc.d.ts renders).
func methodDescriptorsByRoute() map[string]scriptsdk.SDKMethodDescriptor {
	byRoute := map[string]scriptsdk.SDKMethodDescriptor{}
	for _, d := range scriptsdk.RegisteredSDKMethods() {
		byRoute[d.Route] = d
	}
	return byRoute
}

// wrapExecutorResult renders one successful Executor.Execute call's raw
// stdout bytes into the JSON shape descriptor.Result declares: a
// scriptsdk.AckResult wraps the trimmed text as {"message": text};
// scriptsdk.JSONResult round-trips the raw text as {"json": text}
// (generate.go's own documented convention -- the caller parses it, this
// registry never re-declares a second copy of each command's report
// shape); scriptsdk.APIKeyCreateResult forwards the command's own --json
// output verbatim, since its fields already match field-for-field.
func wrapExecutorResult(descriptor scriptsdk.SDKMethodDescriptor, stdout []byte) json.RawMessage {
	text := strings.TrimSpace(string(stdout))
	switch descriptor.Result.(type) {
	case scriptsdk.JSONResult:
		payload, _ := json.Marshal(scriptsdk.JSONResult{JSON: text})
		return payload
	case scriptsdk.APIKeyCreateResult:
		if json.Valid(stdout) {
			return json.RawMessage(stdout)
		}
		payload, _ := json.Marshal(scriptsdk.AckResult{Message: text})
		return payload
	default:
		payload, _ := json.Marshal(scriptsdk.AckResult{Message: text})
		return payload
	}
}

// dispatchCmdCall handles exactly one CmdCallFrame: an unknown method
// (not present in scriptsdk.RegisteredSDKMethods()) never reaches the
// Executor and returns GOLC_SCRIPT_METHOD_UNKNOWN; a known method builds
// its argv from call.Params (buildRouteArgs) and calls
// h.cfg.Executor.Execute, translating its exit code into Ok:true/false. A
// Params decode failure similarly never reaches the Executor.
func (h *Host) dispatchCmdCall(run *Run, call CmdCallFrame) (CmdResultFrame, CallOutcome) {
	started := time.Now()

	// D-11: once a TerminationReason is set for run, every subsequent
	// inbound cmd-call is answered with that same reason and never
	// reaches the Executor. A call already past this check when
	// termination begins on another goroutine is unaffected -- Go gives
	// this function no preemption point until it returns, so it always
	// runs to completion and its outcome is still recorded below.
	if reason, terminating := run.terminationReason(); terminating {
		redactedMessage := security.Redact(reason.Message)
		outcome := CallOutcome{
			Method: call.Method, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: reason.Code, Message: redactedMessage,
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: reason.Code, Message: redactedMessage}, outcome
	}

	descriptor, known := methodDescriptorsByRoute()[call.Method]
	if !known {
		message := fmt.Sprintf("no SDK method registered for %q", call.Method)
		outcome := CallOutcome{
			Method: call.Method, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: "GOLC_SCRIPT_METHOD_UNKNOWN", Message: message,
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: outcome.Code, Message: "GOLC_SCRIPT_METHOD_UNKNOWN: " + message}, outcome
	}

	if reason := h.enforce(descriptor, run); reason != nil {
		redactedMessage := security.Redact(reason.Message)
		outcome := CallOutcome{
			Method: call.Method, Route: descriptor.Route, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: reason.Code, Message: redactedMessage,
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: reason.Code, Message: redactedMessage}, outcome
	}

	args, buildErr := buildRouteArgs(descriptor.Route, h.cfg.ShowPath, call.Params)
	if buildErr != nil {
		redactedMessage := security.Redact(buildErr.Error())
		outcome := CallOutcome{
			Method: call.Method, Route: descriptor.Route, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: "GOLC_SCRIPT_PARAMS_INVALID", Message: redactedMessage,
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: outcome.Code, Message: redactedMessage}, outcome
	}

	exitCode, stdout, stderr := h.cfg.Executor.Execute(descriptor.Route, args, h.cfg.Root)
	duration := time.Since(started).Milliseconds()

	if exitCode == 0 {
		outcome := CallOutcome{Method: call.Method, Route: descriptor.Route, DurationMS: duration, Ok: true}
		return CmdResultFrame{ID: call.ID, Ok: true, Result: wrapExecutorResult(descriptor, stdout)}, outcome
	}

	message := security.Redact(strings.TrimSpace(string(stderr)))
	if message == "" {
		message = fmt.Sprintf("route %q failed with exit code %d", descriptor.Route, exitCode)
	}
	code := "GOLC_SCRIPT_EXECUTE_FAILED"
	outcome := CallOutcome{Method: call.Method, Route: descriptor.Route, DurationMS: duration, Ok: false, Code: code, Message: message}
	return CmdResultFrame{ID: call.ID, Ok: false, Code: code, Message: message}, outcome
}

// mutationOutcomeFor renders a CallOutcome's Ok into the "success"/
// "failure" value api.MutationEvent.Outcome expects.
func mutationOutcomeFor(ok bool) string {
	if ok {
		return "success"
	}
	return "failure"
}

// mutationStatusCodeFor renders a CallOutcome's Ok into an HTTP-shaped
// status code for api.MutationEvent.StatusCode -- there is no real HTTP
// request behind a script-issued call, so this is an informational
// best-effort mapping (200/500), consistent with the field's meaning for
// every other MutationEvent source.
func mutationStatusCodeFor(ok bool) int {
	if ok {
		return 200
	}
	return 500
}

// publishCallOutcome fires both halves of D-05 for one recorded
// CallOutcome: a script.outcome ScriptEvent (the live debug-panel half,
// Task 1) and an api.MutationEvent carrying Source:"script" (the audit
// half, Task 2's exported api.PublishMutationEvent seam) -- so the exact
// same outcome reaches the running script's own panel in real time AND
// the Phase 7 audit trail, never only one of them. route falls back to
// call.Method when descriptor resolution never happened (an unknown
// method or a termination-denied call), so an audited/observed outcome
// always carries the caller's attempted route even when it never reached
// scriptsdk's registry.
func publishCallOutcome(run *Run, method string, outcome CallOutcome) {
	route := outcome.Route
	if route == "" {
		route = method
	}
	PublishScriptEvent(ScriptEvent{
		Kind:       ScriptEventOutcome,
		RunID:      run.RunID,
		ScriptName: run.ScriptName,
		At:         time.Now(),
		Method:     outcome.Method,
		Route:      outcome.Route,
		DurationMS: outcome.DurationMS,
		Ok:         outcome.Ok,
		Code:       outcome.Code,
		Message:    outcome.Message,
	})
	api.PublishMutationEvent(api.MutationEvent{
		Route:         route,
		Actor:         "script:" + run.ScriptName,
		Source:        "script",
		CorrelationID: run.RunID.String(),
		Outcome:       mutationOutcomeFor(outcome.Ok),
		StatusCode:    mutationStatusCodeFor(outcome.Ok),
	})
}

// runDispatchIO drives the multiplexed session protocol on an already-
// open stdin/stdout/stderr triple: every log frame is redacted and
// appended to the bounded Logs stream (D-04); every cmd-call frame is
// dispatched via dispatchCmdCall and answered with exactly one cmd-result
// frame; every captured stderr line is redacted and appended to both the
// bounded Logs stream and a bounded raw tail used for a failure summary.
// It returns once stdout reaches a clean EOF (the child exited) or a
// protocol violation forces an early stop.
func (h *Host) runDispatchIO(run *Run, stdin io.Writer, stdout, stderr io.Reader) RunOutcome {
	outcome := RunOutcome{RunID: run.RunID, Status: show.ScriptRunStatusSucceeded}

	stderrTail := newBoundedBuffer(stderrTailBudget)
	stderrDone := make(chan struct{})
	var stderrLogsMu sync.Mutex
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 0, frameScannerInitialBufferBytes), maxFrameBytes)
		for scanner.Scan() {
			redacted := security.Redact(scanner.Text())
			stderrTail.Write([]byte(redacted + "\n"))
			stderrLogsMu.Lock()
			outcome.Logs = appendBoundedLog(outcome.Logs, LogLine{Level: "stderr", Message: redacted})
			stderrLogsMu.Unlock()
			PublishScriptEvent(ScriptEvent{
				Kind: ScriptEventLog, RunID: run.RunID, ScriptName: run.ScriptName, At: time.Now(),
				Level: "stderr", Message: redacted, Source: "stderr",
			})
		}
	}()

	scanner := newFrameReader(stdout)
	for {
		line, ok, scanErr := scanFrameLine(scanner)
		if scanErr != nil {
			outcome.Status = show.ScriptRunStatusFailed
			outcome.Reason = scanErr.Error()
			break
		}
		if !ok {
			break
		}

		frame, decodeErr := DecodeFrame(line)
		if decodeErr != nil {
			outcome.Status = show.ScriptRunStatusFailed
			outcome.Reason = decodeErr.Error()
			break
		}

		switch f := frame.(type) {
		case ReadyFrame:
			// Informational only.
		case LogFrame:
			redacted := security.Redact(f.Message)
			outcome.Logs = appendBoundedLog(outcome.Logs, LogLine{
				Level:   f.Level,
				Message: redacted,
				Source:  f.Source,
			})
			PublishScriptEvent(ScriptEvent{
				Kind: ScriptEventLog, RunID: run.RunID, ScriptName: run.ScriptName, At: time.Now(),
				Level: f.Level, Message: redacted, Source: f.Source,
			})
		case DoneFrame:
			if f.ExitReason != "" {
				outcome.Reason = f.ExitReason
			}
			// Debug mode (08-13's acceptance pass, deferred-items.md's
			// 08-13 section): V8's inspector agent holds the process
			// open ("Waiting for the debugger to disconnect...") even
			// after runCompletionTrailerTS's own Deno.exit(0), for as
			// long as a CDP client stays attached -- Deno mirrors
			// Node's identical --inspect behavior here. Host.Run's own
			// `defer bridge.Close()` cannot break that hold: it only
			// runs once this loop returns, and this loop cannot return
			// until stdout reaches EOF, which is exactly what the
			// inspector is withholding. Closing the bridge here, the
			// moment the child signals it is done with its own code,
			// is what actually lets the held-open process exit and
			// this loop see its EOF.
			if bridge := run.Bridge(); bridge != nil {
				bridge.Close()
			}
		case CmdCallFrame:
			result, callOutcome := h.dispatchCmdCall(run, f)
			outcome.Outcomes = append(outcome.Outcomes, callOutcome)
			publishCallOutcome(run, f.Method, callOutcome)
			if err := EncodeFrame(stdin, result); err != nil {
				outcome.Status = show.ScriptRunStatusFailed
				outcome.Reason = err.Error()
				return outcome
			}
		default:
			outcome.Status = show.ScriptRunStatusFailed
			outcome.Reason = fmt.Sprintf("GOLC_SCRIPT_PROTOCOL_VIOLATION: unexpected frame kind %q from child", frame.FrameKind())
			return outcome
		}
	}

	<-stderrDone
	// Fill Reason from the captured stderr tail whenever nothing else set
	// it, regardless of Status: a script that throws exits the Deno
	// process non-zero without ever sending an explicit done frame, so
	// Run (not this loop) is what later learns the run failed from
	// cmd.Wait()'s error -- the stderr tail (source-mapped stack trace,
	// D-03) must already be populated here so Run has it to attach.
	if outcome.Reason == "" {
		if tail := strings.TrimSpace(stderrTail.String()); tail != "" {
			outcome.Reason = tail
		}
	}
	return outcome
}

// terminalStatusReason derives the guaranteed terminal event's Status/
// Reason from Run's own named return values: result.Status/Reason when
// the dispatch loop actually ran (runDispatchIO always leaves Status
// populated), falling back to show.ScriptRunStatusFailed and runErr's own
// message for an early failure return (e.g.
// GOLC_SCRIPT_RUN_TEMP_DIR_FAILED) that never reaches runDispatchIO at
// all -- so a terminal event is guaranteed even for a run that fails
// before a single frame is ever read (T-08-38).
func terminalStatusReason(result RunOutcome, runErr error) (show.ScriptRunStatus, string) {
	status := result.Status
	reason := result.Reason
	if status == "" {
		status = show.ScriptRunStatusFailed
	}
	if reason == "" && runErr != nil {
		reason = runErr.Error()
	}
	return status, reason
}

// computeTerminalEvent builds the guaranteed script.terminal ScriptEvent
// for one run's exit -- a pure function of run and Run's own named return
// values, directly unit-testable (events_test.go's seven-termination-cause
// table) without needing to spawn a real Deno process for each cause.
func computeTerminalEvent(run *Run, result RunOutcome, runErr error) ScriptEvent {
	status, reason := terminalStatusReason(result, runErr)
	return ScriptEvent{
		Kind: ScriptEventTerminal, RunID: run.RunID, ScriptName: run.ScriptName, At: time.Now(),
		Status: status, Reason: reason,
	}
}

// Run spawns a fresh, zero-permission Deno subprocess for target,
// materializes the generated SDK runtime shim ahead of target.Source into
// a single .ts file under a per-run os.MkdirTemp directory (removed on
// every exit path, including panic, via defer), drives the session
// protocol over its stdio, and returns the run's outcome. At most one Run
// may be active on a Host at a time (v1 scope call, 08-05-PLAN.md's
// "Planner scope call"): a second Run request while one is active returns
// GOLC_SCRIPT_RUN_ACTIVE and never spawns a process. Two sequential Run
// calls always mint a distinct RunID (D-13: no state carries over from a
// prior run). breakpoints is only ever read when mode == LaunchModeDebug
// (08-09-PLAN.md Task 2): it names the author-coordinate lines the debug
// bridge sets before resuming from the initial break; a LaunchModeRun
// call ignores it entirely, so passing a non-nil breakpoints slice
// alongside LaunchModeRun can never open a debug channel -- the branch on
// mode is what gates every debug-only behavior, exactly like
// buildDenoArgs' own mode-gated inspector argument.
func (h *Host) Run(ctx context.Context, target show.Script, mode LaunchMode, breakpoints []int) (result RunOutcome, runErr error) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return RunOutcome{}, errors.New("GOLC_SCRIPT_RUN_ACTIVE: a script run is already active on this host")
	}
	h.running = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
	}()

	runID, err := uuid.NewV7()
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_ID_MINT_FAILED: %v", err)
	}
	run := &Run{
		RunID: runID, ScriptName: target.Name, Profile: target.CapabilityProfile, StartedAt: time.Now(),
		done: make(chan struct{}),
	}

	// 08-08-PLAN.md Task 1's flagged SCRP-05 edge mitigation: registered
	// as the OUTERMOST defer, immediately after run exists and before any
	// fallible step below, so a guaranteed script.terminal event -- the
	// run's final Status/Reason, derived from Run's own named return
	// values -- is published on every exit path from this point on,
	// including an early failure return, a panic, or a kill mid-dispatch
	// (T-08-38: a subscriber never has to infer that a run ended). Also
	// mirrors the same terminal transition onto the existing SSE stream
	// as a "script" event (D-04's live-streaming reuse of Phase 7's
	// pattern, via internal/api's PublishScriptLifecycleEvent seam).
	defer func() {
		ev := computeTerminalEvent(run, result, runErr)
		PublishScriptEvent(ev)
		api.PublishScriptLifecycleEvent(ev.RunID.String(), ev.ScriptName, string(ev.Status), ev.Reason)
	}()

	// Register run before any fallible step below so a concurrent
	// "script stop <name>" (08-06-PLAN.md Task 3) can always find and
	// terminate it, and finalize -- close done, deregister -- on every
	// exit path from here on, including an early failure return: a
	// Stop() call already blocked on <-run.done must never hang forever
	// because Run itself failed before ever spawning a process.
	registerActiveRun(run)
	defer func() {
		run.outcome = result
		close(run.done)
		deregisterActiveRun(run)
	}()

	// D-04: a script.status event marks the run's start, and mirrors onto
	// the existing SSE stream as a "script" lifecycle event, so a
	// subscriber attached before this run's first log line still sees it
	// begin.
	PublishScriptEvent(ScriptEvent{
		Kind: ScriptEventStatus, RunID: run.RunID, ScriptName: run.ScriptName, At: time.Now(),
		Status: show.ScriptRunStatusRunning,
	})
	api.PublishScriptLifecycleEvent(run.RunID.String(), run.ScriptName, string(show.ScriptRunStatusRunning), "")

	// D-08: every run carries its own wall-clock deadline, resolved from
	// the profile's capability preset (deadlineFor delegates entirely to
	// show.CapabilityProfile.ResolveResourceLimits -- the single place
	// the safe-default discipline lives). runCtx.Err() == context.
	// DeadlineExceeded is checked below once the run ends; the deadline
	// itself never grants a grace period beyond it.
	deadline := deadlineFor(target.CapabilityProfile)
	runCtx, cancel := context.WithDeadline(ctx, run.StartedAt.Add(deadline))
	defer cancel()
	run.terminationMu.Lock()
	run.cancel = cancel
	run.terminationMu.Unlock()

	runDir, err := os.MkdirTemp("", "golc-script-run-*")
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_TEMP_DIR_FAILED: %v", err)
	}
	defer os.RemoveAll(runDir)

	scriptPath := filepath.Join(runDir, runID.String()+".ts")
	materialized := scriptsdk.RuntimeShimTS + "\n" + target.Source + "\n" + runCompletionTrailerTS
	if err := os.WriteFile(scriptPath, []byte(materialized), 0o600); err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_SOURCE_WRITE_FAILED: %v", err)
	}

	// 08-09-PLAN.md Task 1/2: a debug port is only ever picked for
	// LaunchModeDebug -- picking it here (before cmd is built) lets the
	// exact same port value drive both buildDenoArgs' --inspect-brk
	// argument and, a few lines below, NewDebugBridge's CDP dial, without
	// buildDenoArgs itself ever choosing a port.
	debugPort := 0
	if mode == LaunchModeDebug {
		port, portErr := pickEphemeralLoopbackPort()
		if portErr != nil {
			return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_DEBUG_PORT_FAILED: %v", portErr)
		}
		debugPort = port
	}

	cmd := exec.CommandContext(runCtx, h.denoPath, buildDenoArgs(scriptPath, mode, debugPort)...)
	cmd.Dir = runDir
	// Explicit, never-inherited environment (T-08-16): the daemon's own
	// environment variables are never passed through to a script process.
	cmd.Env = []string{}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_START_FAILED: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_START_FAILED: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_START_FAILED: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_START_FAILED: %v", err)
	}

	// 08-06-PLAN.md Task 2: assign the child to a fresh Job Object
	// immediately after Start() and before any frame is read from it, so
	// the child cannot outrun assignment. A Job Object creation/assignment
	// failure never blocks the run itself -- run.cancel remains the kill
	// fallback -- but is never silently ignored either: it is appended to
	// the run's own captured logs so a "why didn't the cap apply" question
	// is answerable from the run's own outcome.
	job, jobErr := newJobObject(resourceLimitsFor(target.CapabilityProfile))
	var jobLog *LogLine
	if jobErr != nil {
		jobLog = &LogLine{Level: "error", Message: security.Redact(fmt.Sprintf("GOLC_SCRIPT_JOBOBJECT_CREATE_FAILED: %v", jobErr))}
	} else if assignErr := job.assign(uint32(cmd.Process.Pid)); assignErr != nil {
		_ = job.Close()
		jobLog = &LogLine{Level: "error", Message: security.Redact(assignErr.Error())}
	} else {
		run.terminationMu.Lock()
		run.job = job
		run.terminationMu.Unlock()
		defer job.Close()
	}

	// 08-09-PLAN.md Task 2: Debug mode's CDP bridge is constructed after
	// Job Object assignment (so a debug run is never less supervised than
	// a plain Run for even a moment) and before runDispatchIO starts
	// reading frames -- SetBreakpoints' own Runtime.runIfWaitingForDebugger
	// call is what actually lets the child's first authored line run, so
	// the dispatch loop below would otherwise never see a single frame. A
	// bridge construction or breakpoint-set failure never blocks the run
	// itself (it degrades to a plain, undebugged execution) but is never
	// silently ignored either -- logged exactly like a Job Object failure
	// above.
	var debugLog *LogLine
	if mode == LaunchModeDebug {
		shimLineCount := shimLineOffsetFor(scriptsdk.RuntimeShimTS)
		bridge, bridgeErr := NewDebugBridge(runCtx, run.RunID, debugPort, shimLineCount, target.Name)
		if bridgeErr != nil {
			debugLog = &LogLine{Level: "error", Message: security.Redact(bridgeErr.Error())}
		} else if bpErr := bridge.SetBreakpoints(breakpoints); bpErr != nil {
			_ = bridge.Close()
			debugLog = &LogLine{Level: "error", Message: security.Redact(bpErr.Error())}
		} else {
			run.terminationMu.Lock()
			run.bridge = bridge
			run.terminationMu.Unlock()
			// Registered after the Job Object's own defer above, so
			// (LIFO) this bridge closes BEFORE the Job Object closes on
			// every normal exit path too -- the same "CDP connection,
			// then Job Object" ordering terminate() enforces on an
			// explicit Stop/deadline/limit kill.
			defer bridge.Close()
		}
	}

	outcome := h.runDispatchIO(run, stdin, stdout, stderr)
	if jobLog != nil {
		outcome.Logs = appendBoundedLog(outcome.Logs, *jobLog)
	}
	if debugLog != nil {
		outcome.Logs = appendBoundedLog(outcome.Logs, *debugLog)
	}
	waitErr := cmd.Wait()

	// run.terminationReason() (a scope violation, rate overrun, or an
	// explicit Stop, 08-06-PLAN.md Task 3) takes priority over a generic
	// runCtx.Err() interpretation: D-11's "the reason a script observes
	// is the one that actually began termination".
	if reason, terminated := run.terminationReason(); terminated {
		outcome.Status = show.ScriptRunStatusTerminated
		outcome.Reason = reason.String()
	} else if runCtx.Err() == context.DeadlineExceeded {
		outcome.Status = show.ScriptRunStatusTerminated
		reason := checkDeadline(time.Since(run.StartedAt), deadline)
		if reason == nil {
			reason = &TerminationReason{
				Code:    "GOLC_SCRIPT_DEADLINE_EXCEEDED",
				Message: fmt.Sprintf("run exceeded its %s deadline", deadline),
				At:      time.Now(),
			}
		}
		outcome.Reason = reason.String()
	} else if runCtx.Err() != nil {
		outcome.Status = show.ScriptRunStatusTerminated
		if outcome.Reason == "" {
			outcome.Reason = runCtx.Err().Error()
		}
	} else if waitErr != nil && outcome.Status == show.ScriptRunStatusSucceeded {
		outcome.Status = show.ScriptRunStatusFailed
		if outcome.Reason == "" {
			outcome.Reason = waitErr.Error()
		}
	}

	return outcome, nil
}
