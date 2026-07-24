//go:build windows

package ipc

import (
	"net"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// PipeName is the stable production named-pipe endpoint shared by the
// daemon and every local CLI/UI client.
const PipeName = `\\.\pipe\golc-artnet`

// ownerOnlySDDL is a protected DACL granting Generic All to the owner alone.
const ownerOnlySDDL = "D:P(A;;GA;;;OW)"

func listenTransport(pipeName string) (net.Listener, error) {
	return winio.ListenPipe(pipeName, &winio.PipeConfig{SecurityDescriptor: ownerOnlySDDL})
}

func dialTransport(pipeName string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(pipeName, &timeout)
}
