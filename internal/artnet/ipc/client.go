// client.go implements the CLI-side half of CONTEXT D-03/D-04's local IPC
// bridge (04-04-PLAN.md Task 1, 04-RESEARCH.md Pattern 5): Dial connects to
// the daemon's build-selected local endpoint, surfacing an unreachable daemon as
// GOLC_ARTNET_DAEMON_UNREACHABLE rather than a raw dial error or a hang
// (CONTEXT: "A CLI client that cannot reach the daemon gets a clear
// GOLC_ARTNET_DAEMON_UNREACHABLE result"). Forward marshals a Request over
// an already-dialed connection (the same length-prefixed, strictjson-
// canonical framing server.go's Serve uses) and returns the Result the
// daemon wrote back -- 04-05's thin `golc artnet ...` CLI routes
// (internal/command/artnet.go) call Dial then Forward exactly as
// RESEARCH.md Pattern 5's runArtnetStatus example shows, converting
// between these local types and command.Request/command.Result at that
// call site (see types.go's doc comment for why Request/Result are
// declared locally in this package rather than imported from
// internal/command).
package ipc

import (
	"fmt"
	"net"
	"time"

	"github.com/lnorton89/golc/internal/strictjson"
)

// dialTimeout bounds how long Dial waits for the daemon's local endpoint to
// accept a connection -- short enough that an unreachable daemon fails
// fast rather than hanging a short-lived CLI invocation.
const dialTimeout = 2 * time.Second

// Dial connects to the daemon's local endpoint at pipeName (callers pass
// PipeName in production; tests pass a distinct per-test path). An
// unreachable daemon -- not running, or the pipe not yet created --
// surfaces as GOLC_ARTNET_DAEMON_UNREACHABLE rather than a raw dial error
// or a hang.
func Dial(pipeName string) (net.Conn, error) {
	conn, err := dialTransport(pipeName, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running? (%v)", err)
	}
	return conn, nil
}

// Forward marshals request over conn as one length-prefixed,
// strictjson-canonical frame, then reads and decodes the daemon's
// length-prefixed Result response. Any transport or decode failure on this
// side of the exchange is reported as an ExitCode:1 Result (never a panic
// or a zero-value success) since Forward's caller -- a CLI route -- has no
// other channel to report it through.
func Forward(conn net.Conn, request Request) Result {
	encoded, err := strictjson.CanonicalEncode(request)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_ENCODE_FAILED: %v\n", err))}
	}
	if err := writeFrame(conn, encoded); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_WRITE_FAILED: %v\n", err))}
	}

	payload, err := readFrame(conn)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))}
	}

	var result Result
	if err := strictjson.DecodeStrict(payload, &result); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))}
	}
	return result
}
