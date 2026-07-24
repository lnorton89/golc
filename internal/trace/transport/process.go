// process.go implements the real, transport-neutral process-boundary
// RemoteClient carrier (CONTEXT D-01/D-03; Plan 01-13/01-15 key_links):
// ProcessClient launches only the project-local compiled Node adapter
// (tools/linear-sync/dist/src/cli.js by default) over its already
// established strict, one-line, newline-delimited JSON contract
// (tools/linear-sync/src/protocol.ts's decodeOperation/OperationResult),
// enforces a per-call deadline, kills the full process tree on timeout or
// cancellation, and never lets raw child stderr bytes escape without first
// being redaction-scanned through internal/security. It owns no
// reconciliation, identity, merge, or apply policy whatsoever (CONTEXT
// D-11/D-13/D-17): Call accepts and returns opaque JSON bytes, so this
// file has zero dependency on internal/trace/reconcile or
// internal/trace/apply — avoiding the transport->apply->transport import
// cycle those two packages would otherwise form (apply/engine.go and
// apply/guard.go both already import this package). internal/command/linear.go
// is the one caller that encodes/decodes those bytes into the
// tools/linear-sync/src/protocol.ts Operation/OperationResult shapes and
// adapts them to apply.RemoteClient.
package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/security"
)

// RPCError is the exact, stable, redaction-safe error shape every
// ProcessClient failure returns: a machine-readable code and a safe
// message that has already been passed through internal/security.Redact,
// so a caller can never accidentally propagate a leaked secret by
// formatting the underlying error further.
type RPCError struct {
	Code    string
	Message string
}

// Error renders e as "<code>: <message>".
func (e *RPCError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// defaultStderrBudget bounds how many trailing stderr bytes ProcessClient
// ever retains for a failure diagnostic (T-01-44: information disclosure).
// Node/adapter failures are always short, allowlisted, or already-redacted
// text (tools/linear-sync/src/redact.ts's safeError never emits free-text
// content); this budget exists only to bound memory against a
// misbehaving or hostile child process, never to truncate a legitimate
// diagnostic.
const defaultStderrBudget = 8192

// defaultCloseGrace is how long Close waits for a clean process exit
// (after closing stdin, which ends tools/linear-sync/src/cli.ts's runCLI
// loop) before falling back to a process-tree kill.
const defaultCloseGrace = 3 * time.Second

// ProcessConfig configures one ProcessClient launch. NodeExecutable and
// ScriptPath must both already exist on disk (CONTEXT D-01/D-02): this
// package never falls back to a host PATH lookup and never downloads or
// installs anything itself — that is bootstrap's exclusive responsibility
// (internal/bootstrap, golc.ps1 "bootstrap --include linear-sync").
type ProcessConfig struct {
	// NodeExecutable is the absolute path to the pinned project-local
	// node.exe.
	NodeExecutable string
	// ScriptPath is the absolute path to the compiled adapter entrypoint
	// (tools/linear-sync/dist/src/cli.js in production).
	ScriptPath string
	// WorkDir is the child process's working directory — normally
	// tools/linear-sync, so its own node_modules resolve; a test harness
	// may point this at an isolated fake-SDK workspace instead.
	WorkDir string
	// ProjectRoot is the absolute normalized repository root propagated to
	// the otherwise explicit child environment.
	ProjectRoot string
	// Env is the complete child process environment (never inherited
	// implicitly): callers must pass exactly the variables the adapter
	// needs (for example LINEAR_API_KEY), never more.
	Env []string
	// Timeout bounds every individual Call when the caller's context
	// carries no deadline of its own. Zero means "no default timeout";
	// every call must then supply its own context deadline.
	Timeout time.Duration
}

// boundedBuffer retains at most limit trailing bytes written to it,
// dropping the oldest bytes first — used only for a bounded stderr
// diagnostic tail, never for protocol data.
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

// ProcessClient is one launched project-local Node adapter process bound
// to the strict newline-delimited JSON protocol
// tools/linear-sync/src/cli.ts's runCLI already implements. Exactly one
// Call may be in flight at a time (Call serializes internally): the
// underlying protocol is a strict one-request/one-response-line exchange,
// never pipelined.
type ProcessClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	stderr  *boundedBuffer
	timeout time.Duration

	mu     sync.Mutex
	closed bool
}

// NewProcessClient launches cfg.NodeExecutable cfg.ScriptPath inside
// cfg.WorkDir with exactly cfg.Env as its environment. It fails closed
// with a GOLC_TRANSPORT_ADAPTER_MISSING RPCError before ever attempting
// to start a process if either the node executable or the compiled
// adapter script does not already exist on disk (CONTEXT D-21: a missing
// adapter must fail only the explicit remote command that needed it, with
// a clear, actionable diagnostic).
func NewProcessClient(cfg ProcessConfig) (*ProcessClient, error) {
	projectRoot := filepath.Clean(cfg.ProjectRoot)
	if strings.TrimSpace(cfg.ProjectRoot) == "" || !filepath.IsAbs(cfg.ProjectRoot) || projectRoot != cfg.ProjectRoot {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_CONFIG_INVALID", Message: "project root must be an absolute normalized path"}
	}
	if strings.TrimSpace(cfg.NodeExecutable) == "" || strings.TrimSpace(cfg.ScriptPath) == "" {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_CONFIG_INVALID", Message: "node executable and script path are both required"}
	}
	if _, err := os.Stat(cfg.NodeExecutable); err != nil {
		return nil, &RPCError{
			Code:    "GOLC_TRANSPORT_ADAPTER_MISSING",
			Message: fmt.Sprintf("pinned node executable not found: %s; run 'golc.ps1 bootstrap --include linear-sync' first", cfg.NodeExecutable),
		}
	}
	if _, err := os.Stat(cfg.ScriptPath); err != nil {
		return nil, &RPCError{
			Code: "GOLC_TRANSPORT_ADAPTER_MISSING",
			Message: fmt.Sprintf(
				"compiled adapter entrypoint not found: %s; run 'golc.ps1 bootstrap --include linear-sync' and 'golc.ps1 build --scope linear-sdk' first",
				cfg.ScriptPath),
		}
	}

	cmd := exec.Command(cfg.NodeExecutable, cfg.ScriptPath)
	cmd.Dir = cfg.WorkDir
	cmd.Env = upsertProcessEnvironment(cfg.Env, "GOLC_PROJECT_ROOT", projectRoot)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_START", Message: fmt.Sprintf("stdin pipe: %v", err)}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_START", Message: fmt.Sprintf("stdout pipe: %v", err)}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_START", Message: fmt.Sprintf("stderr pipe: %v", err)}
	}

	if err := cmd.Start(); err != nil {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_START", Message: fmt.Sprintf("%v", err)}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	client := &ProcessClient{
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
		stderr:  newBoundedBuffer(defaultStderrBudget),
		timeout: cfg.Timeout,
	}
	go client.drainStderr(stderr)
	return client, nil
}

func upsertProcessEnvironment(environment []string, name, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		existing, _, found := strings.Cut(entry, "=")
		if found && strings.EqualFold(existing, name) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, name+"="+value)
}

// drainStderr copies every byte the child writes to stderr into a bounded,
// never-unbounded buffer (T-01-44). It never echoes stderr to this
// process's own stderr and never blocks a Call on stderr activity.
func (c *ProcessClient) drainStderr(stderr io.Reader) {
	_, _ = io.Copy(c.stderr, stderr)
}

// safeFailureSummary renders a redaction-safe, bounded diagnostic from the
// captured stderr tail plus the process exit state (T-01-44/T-01-45): the
// whole stderr snippet is dropped in favor of the fixed redacted marker
// the instant any forbidden token is found anywhere in it, so a coding
// mistake elsewhere in the adapter can never leak a fragment of a real
// secret through a "safe" diagnostic.
func (c *ProcessClient) safeFailureSummary() string {
	tail := strings.TrimSpace(c.stderr.String())
	exitDetail := "still running"
	if c.cmd.ProcessState != nil {
		exitDetail = c.cmd.ProcessState.String()
	}
	if tail == "" {
		return fmt.Sprintf("process state: %s", exitDetail)
	}
	return fmt.Sprintf("process state: %s; stderr: %s", exitDetail, security.Redact(tail))
}

// killProcessTree terminates pid and every descendant it spawned. On
// Windows (the only platform this repository's toolchain provisioning
// targets in Phase 1) this uses "taskkill /T /F" so a child the adapter
// itself spawned can never outlive a timeout or cancellation; on any other
// platform it falls back to killing only the direct child.
func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		_ = kill.Run()
		return
	}
	_ = cmd.Process.Kill()
}

// terminate kills the process tree, closes stdin, marks the client closed,
// and reaps the process in the background. Callers must already hold c.mu.
func (c *ProcessClient) terminate() {
	if c.closed {
		return
	}
	c.closed = true
	killProcessTree(c.cmd)
	_ = c.stdin.Close()
	go func() { _ = c.cmd.Wait() }()
}

// lineResult is the outcome of one background stdout-line read.
type lineResult struct {
	line []byte
	err  error
	eof  bool
}

// Call sends exactly one strict, single-line JSON request and returns
// exactly one strict, single-line JSON response, enforcing ctx's deadline
// (or cfg.Timeout when ctx carries none) end to end (CONTEXT D-21). A
// response that is not valid, single-object JSON on its own line — the
// exact "protocol noise" tools/linear-sync/src/cli.ts's runCLI never
// itself produces on its happy path, but any other bytes reaching this
// boundary must still fail closed rather than being silently accepted —
// is rejected as GOLC_TRANSPORT_PROTOCOL_NOISE. A timeout or a caller
// cancellation kills the full process tree and permanently closes this
// client (CONTEXT T-01-42: tampering/DoS containment) so a later Call can
// never be silently answered by a previous, abandoned request's response.
func (c *ProcessClient) Call(ctx context.Context, request []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_CLOSED", Message: "process client is already closed"}
	}
	if bytes.ContainsAny(request, "\n\r") {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_REQUEST_INVALID", Message: "request must not contain a newline"}
	}
	if !json.Valid(request) {
		return nil, &RPCError{Code: "GOLC_TRANSPORT_REQUEST_INVALID", Message: "request is not valid JSON"}
	}

	callCtx := ctx
	if callCtx == nil {
		callCtx = context.Background()
	}
	if _, hasDeadline := callCtx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, c.timeout)
		defer cancel()
	}

	line := append(append([]byte(nil), request...), '\n')
	if _, err := c.stdin.Write(line); err != nil {
		c.terminate()
		return nil, &RPCError{Code: "GOLC_TRANSPORT_WRITE_FAILED", Message: c.safeFailureSummary()}
	}

	resultCh := make(chan lineResult, 1)
	go func() {
		if c.scanner.Scan() {
			resultCh <- lineResult{line: append([]byte(nil), c.scanner.Bytes()...)}
			return
		}
		resultCh <- lineResult{err: c.scanner.Err(), eof: true}
	}()

	select {
	case <-callCtx.Done():
		c.terminate()
		reason := "GOLC_TRANSPORT_TIMEOUT"
		if ctx != nil && ctx.Err() == context.Canceled {
			reason = "GOLC_TRANSPORT_CANCELED"
		}
		return nil, &RPCError{Code: reason, Message: fmt.Sprintf("%v (%s)", callCtx.Err(), c.safeFailureSummary())}
	case result := <-resultCh:
		if result.err != nil {
			c.terminate()
			return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_FAILED", Message: c.safeFailureSummary()}
		}
		if result.eof {
			c.terminate()
			return nil, &RPCError{Code: "GOLC_TRANSPORT_PROCESS_EXITED", Message: c.safeFailureSummary()}
		}
		trimmed := bytes.TrimSpace(result.line)
		if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
			c.terminate()
			return nil, &RPCError{Code: "GOLC_TRANSPORT_PROTOCOL_NOISE", Message: "response line is not a single strict JSON object"}
		}
		return trimmed, nil
	}
}

// Close ends the adapter process cleanly: it closes stdin (which ends
// tools/linear-sync/src/cli.ts's runCLI input loop and lets the process
// exit with code 0 on its own) and waits up to defaultCloseGrace before
// falling back to a full process-tree kill. Close is idempotent and safe
// to call more than once or after a Call already terminated the client.
func (c *ProcessClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	_ = c.stdin.Close()
	cmd := c.cmd
	c.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		return nil
	case <-time.After(defaultCloseGrace):
		killProcessTree(cmd)
		<-done
		return nil
	}
}
