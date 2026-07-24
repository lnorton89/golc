// ipc_test.go proves 04-04-PLAN.md Task 1's contract: a Request
// round-trips to a Result unchanged over local IPC (a); a dial to a
// nonexistent endpoint returns GOLC_ARTNET_DAEMON_UNREACHABLE rather than
// a hang or a raw error (b); and oversized frames remain bounded (c).
package ipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
	"time"
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
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- Serve(ctx, listener, handler) }()
	t.Cleanup(func() {
		cancel()
		if err := <-serveDone; err != nil {
			t.Errorf("Serve returned error after cancel: %v", err)
		}
	})

	conn, err := Dial(pipeName)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	request := Request{Route: "artnet status", Args: []string{"--json"}, Root: `C:\show`}
	got := Forward(conn, request)

	if !reflect.DeepEqual(got, wantResult) {
		t.Fatalf("Forward result = %+v, want %+v", got, wantResult)
	}
}

// TestIPCDialNonexistentPipeReturnsDaemonUnreachable proves (b): dialing a
// pipe name nothing is listening on fails fast with
// GOLC_ARTNET_DAEMON_UNREACHABLE, never a hang or a raw dial error.
func TestIPCDialNonexistentPipeReturnsDaemonUnreachable(t *testing.T) {
	pipeName := testPipeName(t)

	start := time.Now()
	_, err := Dial(pipeName)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Dial to a nonexistent pipe to fail, got nil error")
	}
	if !strings.Contains(err.Error(), "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected GOLC_ARTNET_DAEMON_UNREACHABLE, got: %v", err)
	}
	if elapsed > dialTimeout {
		t.Fatalf("expected Dial to fail well within dialTimeout (%s), took %s", dialTimeout, elapsed)
	}
}

// TestReadFrameRejectsOversizedLength proves readFrame bounds a declared
// frame length to maxFrameSize before allocating a buffer, so a forged
// length header can never force an unbounded allocation.
func TestReadFrameRejectsOversizedLength(t *testing.T) {
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameSize+1)

	if _, err := readFrame(bytes.NewReader(header)); err == nil {
		t.Fatal("expected readFrame to reject a declared length above maxFrameSize")
	}
}
