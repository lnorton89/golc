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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

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

// Run is one script execution's identity: a fresh, distinct RunID every
// time (D-13: two sequential runs of the same script are always
// independent, never resuming state from a prior run).
type Run struct {
	RunID      uuid.UUID
	ScriptName string
	Profile    show.CapabilityProfile
	StartedAt  time.Time
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

// enforce is the single seam 08-06 fills with D-06/D-08 host-side
// capability/deadline/rate/resource-limit enforcement (08-05-PLAN.md
// Task 2 action step 7), inserted between the method lookup and the
// Executor call. It always allows for now: this plan does not implement
// capability enforcement, only the seam 08-06 fills with one function
// rather than restructuring the dispatch loop.
func (h *Host) enforce(descriptor scriptsdk.SDKMethodDescriptor, run *Run) error {
	return nil
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

	descriptor, known := methodDescriptorsByRoute()[call.Method]
	if !known {
		message := fmt.Sprintf("no SDK method registered for %q", call.Method)
		outcome := CallOutcome{
			Method: call.Method, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: "GOLC_SCRIPT_METHOD_UNKNOWN", Message: message,
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: outcome.Code, Message: "GOLC_SCRIPT_METHOD_UNKNOWN: " + message}, outcome
	}

	if err := h.enforce(descriptor, run); err != nil {
		outcome := CallOutcome{
			Method: call.Method, Route: descriptor.Route, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: "GOLC_SCRIPT_CAPABILITY_DENIED", Message: err.Error(),
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: outcome.Code, Message: security.Redact(err.Error())}, outcome
	}

	args, buildErr := buildRouteArgs(descriptor.Route, h.cfg.ShowPath, call.Params)
	if buildErr != nil {
		outcome := CallOutcome{
			Method: call.Method, Route: descriptor.Route, DurationMS: time.Since(started).Milliseconds(),
			Ok: false, Code: "GOLC_SCRIPT_PARAMS_INVALID", Message: buildErr.Error(),
		}
		return CmdResultFrame{ID: call.ID, Ok: false, Code: outcome.Code, Message: security.Redact(buildErr.Error())}, outcome
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
			outcome.Logs = appendBoundedLog(outcome.Logs, LogLine{
				Level:   f.Level,
				Message: security.Redact(f.Message),
				Source:  f.Source,
			})
		case DoneFrame:
			if f.ExitReason != "" {
				outcome.Reason = f.ExitReason
			}
		case CmdCallFrame:
			result, callOutcome := h.dispatchCmdCall(run, f)
			outcome.Outcomes = append(outcome.Outcomes, callOutcome)
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

// Run spawns a fresh, zero-permission Deno subprocess for target,
// materializes the generated SDK runtime shim ahead of target.Source into
// a single .ts file under a per-run os.MkdirTemp directory (removed on
// every exit path, including panic, via defer), drives the session
// protocol over its stdio, and returns the run's outcome. At most one Run
// may be active on a Host at a time (v1 scope call, 08-05-PLAN.md's
// "Planner scope call"): a second Run request while one is active returns
// GOLC_SCRIPT_RUN_ACTIVE and never spawns a process. Two sequential Run
// calls always mint a distinct RunID (D-13: no state carries over from a
// prior run).
func (h *Host) Run(ctx context.Context, target show.Script, mode LaunchMode) (RunOutcome, error) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_ACTIVE: a script run is already active on this host")
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
	run := &Run{RunID: runID, ScriptName: target.Name, Profile: target.CapabilityProfile, StartedAt: time.Now()}

	runDir, err := os.MkdirTemp("", "golc-script-run-*")
	if err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_TEMP_DIR_FAILED: %v", err)
	}
	defer os.RemoveAll(runDir)

	scriptPath := filepath.Join(runDir, runID.String()+".ts")
	materialized := scriptsdk.RuntimeShimTS + "\n" + target.Source
	if err := os.WriteFile(scriptPath, []byte(materialized), 0o600); err != nil {
		return RunOutcome{}, fmt.Errorf("GOLC_SCRIPT_RUN_SOURCE_WRITE_FAILED: %v", err)
	}

	cmd := exec.CommandContext(ctx, h.denoPath, buildDenoArgs(scriptPath, mode)...)
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

	outcome := h.runDispatchIO(run, stdin, stdout, stderr)
	waitErr := cmd.Wait()

	switch {
	case ctx.Err() != nil:
		outcome.Status = show.ScriptRunStatusTerminated
		if outcome.Reason == "" {
			outcome.Reason = ctx.Err().Error()
		}
	case waitErr != nil && outcome.Status == show.ScriptRunStatusSucceeded:
		outcome.Status = show.ScriptRunStatusFailed
		if outcome.Reason == "" {
			outcome.Reason = waitErr.Error()
		}
	}

	return outcome, nil
}
