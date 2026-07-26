// protocol.go declares the newline-delimited-JSON session protocol
// internal/script/host.go and session.go carry every child<->host frame
// over (08-05-PLAN.md Task 1, CONTEXT SCRP-01/SCRP-02/SCRP-03).
//
// This generalizes internal/trace/transport/process.go's Call (lines
// 283-357): that is a single strict request/response line pair
// serialized under one sync.Mutex, sufficient for the Linear adapter's
// one-shot RPC. A sandboxed script session is fundamentally different --
// the child may have several cmd-call frames outstanding at once, and
// log frames interleave freely with cmd-result frames -- so this
// protocol is deliberately multiplexed and correlation-id based (every
// cmd-call/cmd-result pair carries a shared ID) rather than reusing
// process.go's one-at-a-time Call as-is. This is new code, not a
// drop-in reuse of that pattern.
//
// Every frame carries a fixed "kind" JSON field naming its type
// (frameEnvelope below): an unrecognized kind, a line longer than
// maxFrameBytes, or malformed JSON are all protocol violations that fail
// closed (GOLC_SCRIPT_FRAME_UNKNOWN / GOLC_SCRIPT_FRAME_TOO_LARGE /
// GOLC_SCRIPT_FRAME_MALFORMED) rather than being silently ignored --
// this package treats every byte the child writes as attacker-controlled
// (T-08-17).
package script

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxFrameBytes bounds every line this protocol reads or writes: a line
// longer than this fails deterministically (GOLC_SCRIPT_FRAME_TOO_LARGE)
// before the scanner ever buffers the whole oversized line, rather than
// growing without bound against a runaway or hostile child (T-08-20).
const maxFrameBytes = 1 << 20 // 1 MiB

// frameScannerInitialBufferBytes is newFrameReader's starting bufio.Scanner
// buffer size -- small enough to stay cheap for the common case; the
// scanner itself grows this up to maxFrameBytes as needed and never
// beyond it.
const frameScannerInitialBufferBytes = 64 << 10

// Frame kind discriminants: the fixed "kind" JSON field value every wire
// frame this protocol declares carries.
const (
	FrameKindCmdCall   = "cmd-call"
	FrameKindLog       = "log"
	FrameKindReady     = "ready"
	FrameKindDone      = "done"
	FrameKindCmdResult = "cmd-result"
	FrameKindCancel    = "cancel"
)

// Frame is the common discriminant every decoded frame satisfies, so a
// caller can type-switch on the concrete value DecodeFrame returns
// without re-parsing the line.
type Frame interface {
	FrameKind() string
}

// CmdCallFrame is a child -> host frame invoking one typed SDK method.
// Despite the field's name, Method carries the exact internal/command
// route string (e.g. "scene activate") scriptsdk.RegisteredSDKMethods()
// indexes by Route -- the generated golc-runtime.ts shim (internal/
// scriptsdk/generate.go, updated alongside this file so the two never
// disagree) sends this exact value on every call, never the TypeScript
// dot-path method name (e.g. "scene.activate") golc.d.ts renders for
// autocomplete.
type CmdCallFrame struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// FrameKind implements Frame.
func (CmdCallFrame) FrameKind() string { return FrameKindCmdCall }

// LogFrame is a child -> host structured log line (D-04's live
// log/diagnostics stream, D-05's per-call outcome panel context). Source
// carries a "file:line" location when the child's own console/error
// reporting supplies one.
type LogFrame struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// FrameKind implements Frame.
func (LogFrame) FrameKind() string { return FrameKindLog }

// ReadyFrame is the child -> host signal the runtime shim's stdio reader
// loop is live: the host may now expect cmd-call frames.
type ReadyFrame struct{}

// FrameKind implements Frame.
func (ReadyFrame) FrameKind() string { return FrameKindReady }

// DoneFrame is the child -> host signal the script finished running on
// its own (as opposed to being killed by the host): ExitReason names why
// in the child's own words, when it has one.
type DoneFrame struct {
	ExitReason string `json:"exitReason,omitempty"`
}

// FrameKind implements Frame.
func (DoneFrame) FrameKind() string { return FrameKindDone }

// CmdResultFrame is a host -> child response to exactly one CmdCallFrame,
// correlated by ID. A CmdResultFrame carrying an error has Ok:false and a
// Code/Message pair; it never carries a partial Result alongside an
// error -- callers constructing one must set exactly Result (success) or
// Code+Message (failure), never both.
type CmdResultFrame struct {
	ID      string          `json:"id"`
	Ok      bool            `json:"ok"`
	Result  json.RawMessage `json:"result,omitempty"`
	Code    string          `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
}

// FrameKind implements Frame.
func (CmdResultFrame) FrameKind() string { return FrameKindCmdResult }

// CancelFrame is a host -> child signal that the run is being cancelled
// (Stop, deadline overrun, or limit breach): Reason names why.
type CancelFrame struct {
	Reason string `json:"reason,omitempty"`
}

// FrameKind implements Frame.
func (CancelFrame) FrameKind() string { return FrameKindCancel }

// frameEnvelope is the minimal shape DecodeFrame reads first to learn a
// line's discriminant before unmarshalling into the concrete frame type.
type frameEnvelope struct {
	Kind string `json:"kind"`
}

// marshalFrame renders frame with its "kind" field emitted first (every
// wrapper below declares Kind before the embedded frame's own fields, and
// encoding/json emits struct fields in declaration order) -- EncodeFrame
// is the only caller.
func marshalFrame(frame Frame) ([]byte, error) {
	switch f := frame.(type) {
	case CmdCallFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			CmdCallFrame
		}{Kind: FrameKindCmdCall, CmdCallFrame: f})
	case LogFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			LogFrame
		}{Kind: FrameKindLog, LogFrame: f})
	case ReadyFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{Kind: FrameKindReady})
	case DoneFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			DoneFrame
		}{Kind: FrameKindDone, DoneFrame: f})
	case CmdResultFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			CmdResultFrame
		}{Kind: FrameKindCmdResult, CmdResultFrame: f})
	case CancelFrame:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			CancelFrame
		}{Kind: FrameKindCancel, CancelFrame: f})
	default:
		return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: unrecognized frame type %T", frame)
	}
}

// EncodeFrame writes frame to w as exactly one line terminated by a
// single "\n". It fails closed (GOLC_SCRIPT_PROTOCOL_VIOLATION) rather
// than writing a corrupt multi-line frame if the encoded JSON somehow
// contains an embedded newline in a string field -- a defensive check
// against a future frame type accidentally carrying literal newline
// bytes in a string value.
func EncodeFrame(w io.Writer, frame Frame) error {
	payload, err := marshalFrame(frame)
	if err != nil {
		return err
	}
	if bytes.ContainsAny(payload, "\n\r") {
		return fmt.Errorf("GOLC_SCRIPT_PROTOCOL_VIOLATION: encoded frame contains an embedded newline")
	}
	payload = append(payload, '\n')
	_, err = w.Write(payload)
	return err
}

// DecodeFrame decodes one already-newline-stripped line into its
// concrete Frame type. Malformed JSON yields GOLC_SCRIPT_FRAME_MALFORMED;
// a well-formed JSON object whose "kind" field is missing or does not
// name one of this package's declared frame kinds yields
// GOLC_SCRIPT_FRAME_UNKNOWN naming the offending kind -- the caller (the
// session loop) must treat either as a protocol violation, not a frame to
// silently skip.
func DecodeFrame(line []byte) (Frame, error) {
	if !json.Valid(line) {
		return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: line is not valid JSON")
	}

	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
	}

	switch envelope.Kind {
	case FrameKindCmdCall:
		var f CmdCallFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
		}
		return f, nil
	case FrameKindLog:
		var f LogFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
		}
		return f, nil
	case FrameKindReady:
		return ReadyFrame{}, nil
	case FrameKindDone:
		var f DoneFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
		}
		return f, nil
	case FrameKindCmdResult:
		var f CmdResultFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
		}
		return f, nil
	case FrameKindCancel:
		var f CancelFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_MALFORMED: %v", err)
		}
		return f, nil
	default:
		return nil, fmt.Errorf("GOLC_SCRIPT_FRAME_UNKNOWN: unrecognized frame kind %q", envelope.Kind)
	}
}

// newFrameReader returns a *bufio.Scanner reading newline-delimited
// frames from r, bounded at maxFrameBytes: a line longer than that fails
// the next Scan() with bufio.ErrTooLong (surfaced by scanFrameLine as
// GOLC_SCRIPT_FRAME_TOO_LARGE) rather than growing the scanner's internal
// buffer without limit -- the oversized line is never fully buffered.
func newFrameReader(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, frameScannerInitialBufferBytes), maxFrameBytes)
	return scanner
}

// scanFrameLine reads one line from scanner. It returns ok=false with a
// nil error at clean EOF, translates bufio.ErrTooLong into
// GOLC_SCRIPT_FRAME_TOO_LARGE, and passes through any other scanner
// error unchanged.
func scanFrameLine(scanner *bufio.Scanner) (line []byte, ok bool, err error) {
	if !scanner.Scan() {
		scanErr := scanner.Err()
		if scanErr == nil {
			return nil, false, nil
		}
		if errors.Is(scanErr, bufio.ErrTooLong) {
			return nil, false, fmt.Errorf("GOLC_SCRIPT_FRAME_TOO_LARGE: line exceeds the %d byte maximum", maxFrameBytes)
		}
		return nil, false, scanErr
	}
	return scanner.Bytes(), true, nil
}
