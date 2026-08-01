// ipc_test.go proves 04-04-PLAN.md Task 1's contract: a Request
// round-trips to a Result unchanged over local IPC (a); a dial to a
// nonexistent endpoint returns GOLC_ARTNET_DAEMON_UNREACHABLE rather than
// a hang or a raw error (b); and oversized frames remain bounded (c).
package ipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIPCRequestRoundTripsToResult proves (a): Forward's Request marshals
// over the pipe, the stub handler's Result comes back decoded unchanged.
func TestIPCRequestRoundTripsToResult(t *testing.T) {
	pipeName := testPipeName(t)

	wantResult := Result{ExitCode: 0, Stdout: []byte("hello from daemon\n")}
	handler := func(request Request) Result {
		return wantResult
	}

	listener, err := NewListener(pipeName)
	require.NoError(t, err, "NewListener")

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- Serve(ctx, listener, handler) }()
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-serveDone, "Serve returned error after cancel")
	})

	conn, err := Dial(pipeName)
	require.NoError(t, err, "Dial")
	defer conn.Close()

	request := Request{Route: "artnet status", Args: []string{"--json"}, Root: `C:\show`}
	got := Forward(conn, request)

	require.Equal(t, wantResult, got, "Forward result")
}

// TestIPCDialNonexistentPipeReturnsDaemonUnreachable proves (b): dialing a
// pipe name nothing is listening on fails fast with
// GOLC_ARTNET_DAEMON_UNREACHABLE, never a hang or a raw dial error.
func TestIPCDialNonexistentPipeReturnsDaemonUnreachable(t *testing.T) {
	pipeName := testPipeName(t)

	start := time.Now()
	_, err := Dial(pipeName)
	elapsed := time.Since(start)

	require.ErrorContains(t, err, "GOLC_ARTNET_DAEMON_UNREACHABLE")
	require.LessOrEqual(t, elapsed, dialTimeout, "expected Dial to fail well within dialTimeout (%s), took %s", dialTimeout, elapsed)
}

// TestReadFrameRejectsOversizedLength proves readFrame bounds a declared
// frame length to maxFrameSize before allocating a buffer, so a forged
// length header can never force an unbounded allocation.
func TestReadFrameRejectsOversizedLength(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameSize+1)

	_, err := readFrame(bytes.NewReader(header))
	require.Error(t, err, "expected readFrame to reject a declared length above maxFrameSize")
}
