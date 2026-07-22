// server.go implements CONTEXT D-03/D-04's local IPC transport (04-04-
// PLAN.md Task 1, 04-RESEARCH.md Standard Stack/Pattern 5, 04-PATTERNS.md
// "No Analog Found": no daemon/IPC listener of any kind existed anywhere
// in this repo before this file): NewListener opens a Windows named pipe
// (github.com/Microsoft/go-winio) whose security descriptor restricts
// connections to the owning principal alone (Security Domain V4:
// local-only, default-deny other principals) and never binds a routable
// TCP address. Serve accepts connections on that listener until ctx is
// cancelled, decoding one command.Request per connection (reusing this
// repo's existing command.Request/command.Result shapes as the wire
// format, per RESEARCH.md Pattern 5 -- no second protocol is invented),
// invoking an injected handler, and writing back the command.Result --
// both encoded/decoded via internal/strictjson's canonical, duplicate-safe
// convention.
//
// Named-pipe byte-mode connections carry no message boundary of their
// own (04-PATTERNS.md's "No Analog Found" callout: there is no in-repo
// framing precedent to copy either), so writeFrame/readFrame prefix every
// request and response with a 4-byte big-endian length so one connection
// unambiguously carries exactly one request and one response.
package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	winio "github.com/Microsoft/go-winio"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
)

// PipeName is the production named-pipe path the daemon (daemon.go) listens
// on by default and every golc artnet ... CLI client (Plan 05) dials.
// NewListener/Dial both accept an explicit pipeName parameter rather than
// hardcoding this constant internally, so test code (this package's own
// ipc_test.go, and internal/artnet's daemon_test.go) can exercise the exact
// same Serve/Dial/Forward machinery against an isolated per-test pipe name
// without colliding with a real running daemon or with another package's
// concurrently-running tests on this same well-known path.
const PipeName = `\\.\pipe\golc-artnet`

// ownerOnlySDDL restricts a named pipe's security descriptor to the owning
// principal only (Security Domain V4, CONTEXT prohibition: never bind a
// routable/TCP address by default): "D:P(...)" declares a Protected DACL
// (no inherited ACEs), granting Generic All ("GA") to the Owner ("OW")
// alone -- no other principal, including other local users on the same
// machine, is granted access.
const ownerOnlySDDL = "D:P(A;;GA;;;OW)"

// maxFrameSize bounds one length-prefixed request/response frame so a
// malformed or hostile local peer can never force Serve or Forward to
// allocate an unbounded buffer from a forged length header.
const maxFrameSize = 4 << 20 // 4 MiB

// NewListener creates a named-pipe listener at pipeName with a security
// descriptor restricting connections to the owning principal (Security
// Domain V4) and never binds a routable/TCP address -- the pipe transport
// itself makes a non-loopback bind structurally impossible.
func NewListener(pipeName string) (net.Listener, error) {
	listener, err := winio.ListenPipe(pipeName, &winio.PipeConfig{SecurityDescriptor: ownerOnlySDDL})
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_IPC_LISTEN_FAILED: %v", err)
	}
	return listener, nil
}

// Serve accepts connections on listener until ctx is cancelled: each
// connection carries exactly one length-prefixed command.Request, decoded
// via strictjson.DecodeStrict, dispatched to handler, and answered with
// exactly one length-prefixed command.Result encoded via
// strictjson.CanonicalEncode. Serve blocks until ctx is cancelled (at which
// point it closes listener, unblocking Accept, and returns nil) or Accept
// fails for a reason other than the listener having just been closed by
// that same cancellation.
func Serve(ctx context.Context, listener net.Listener, handler command.CommandHandler) error {
	closeOnce := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-closeOnce:
		}
	}()
	defer close(closeOnce)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("GOLC_ARTNET_IPC_ACCEPT_FAILED: %v", err)
			}
		}
		go handleConn(conn, handler)
	}
}

// handleConn serves exactly one request/response exchange on conn, then
// closes it. A frame-read or strict-decode failure is reported back to the
// caller as an ExitCode:1 GOLC_ARTNET_IPC_DECODE_FAILED Result rather than
// silently dropping the connection, so a malformed client invocation gets
// a diagnostic instead of a hang.
func handleConn(conn net.Conn, handler command.CommandHandler) {
	defer conn.Close()

	payload, err := readFrame(conn)
	if err != nil {
		_ = writeResult(conn, command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))})
		return
	}

	var request command.Request
	if err := strictjson.DecodeStrict(payload, &request); err != nil {
		_ = writeResult(conn, command.Result{ExitCode: 1, Stderr: []byte(
			fmt.Sprintf("GOLC_ARTNET_IPC_DECODE_FAILED: %v\n", err))})
		return
	}

	result := handler(request)
	_ = writeResult(conn, result)
}

// writeResult canonically encodes result (internal/strictjson) and writes
// it to conn as one length-prefixed frame.
func writeResult(conn net.Conn, result command.Result) error {
	encoded, err := strictjson.CanonicalEncode(result)
	if err != nil {
		return fmt.Errorf("GOLC_ARTNET_IPC_ENCODE_FAILED: %v", err)
	}
	return writeFrame(conn, encoded)
}

// writeFrame writes payload as a 4-byte big-endian length prefix followed
// by payload itself.
func writeFrame(w io.Writer, payload []byte) error {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readFrame reads one length-prefixed frame written by writeFrame,
// rejecting a declared length above maxFrameSize before allocating the
// payload buffer, so a malformed/hostile local peer can never force an
// unbounded allocation from a forged length header.
func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length > maxFrameSize {
		return nil, fmt.Errorf("frame length %d exceeds maximum %d", length, maxFrameSize)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
