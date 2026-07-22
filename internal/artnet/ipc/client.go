// client.go implements the CLI-side half of CONTEXT D-03/D-04's local IPC
// bridge (04-04-PLAN.md Task 1, 04-RESEARCH.md Pattern 5): Dial connects to
// the daemon's named pipe, surfacing an unreachable daemon as
// GOLC_ARTNET_DAEMON_UNREACHABLE rather than a raw dial error or a hang
// (CONTEXT: "A CLI client that cannot reach the daemon gets a clear
// GOLC_ARTNET_DAEMON_UNREACHABLE result"). Forward marshals a
// command.Request over an already-dialed connection (the same
// length-prefixed, strictjson-canonical framing server.go's Serve uses)
// and returns the command.Result the daemon wrote back -- Plan 05's thin
// `golc artnet ...` CLI routes are expected to call Dial then Forward
// exactly as RESEARCH.md Pattern 5's runArtnetStatus example shows.
package ipc

import (
	"fmt"
	"net"
	"time"

	winio "github.com/Microsoft/go-winio"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
)

// dialTimeout bounds how long Dial waits for the daemon's named pipe to
// accept a connection -- short enough that an unreachable daemon fails
// fast rather than hanging a short-lived CLI invocation.
const dialTimeout = 2 * time.Second

// Dial connects to the daemon's named pipe at pipeName (callers pass
// PipeName in production; tests pass a distinct per-test path). An
// unreachable daemon -- not running, or the pipe not yet created --
// surfaces as GOLC_ARTNET_DAEMON_UNREACHABLE rather than a raw dial error
// or a hang.
func Dial(pipeName string) (net.Conn, error) {
	timeout := dialTimeout
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_DAEMON_UNREACHABLE: is the GOLC background process running? (%v)", err)
	}
	return conn, nil
}

// Forward marshals request over conn as one length-prefixed,
// strictjson-canonical frame, then reads and decodes the daemon's
// length-prefixed command.Result response. Any transport or decode failure
// on this side of the exchange is reported as an ExitCode:1 Result (never
// a panic or a zero-value success) since Forward's caller -- a CLI route
// -- has no other channel to report it through.
func Forward(conn net.Conn, request command.Request) command.Result {
	encoded, err := strictjson.CanonicalEncode(request)
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_ENCODE_FAILED: %v\n", err))}
	}
	if err := writeFrame(conn, encoded); err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_WRITE_FAILED: %v\n", err))}
	}

	payload, err := readFrame(conn)
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))}
	}

	var result command.Result
	if err := strictjson.DecodeStrict(payload, &result); err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))}
	}
	return result
}
