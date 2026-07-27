//go:build windows

package ipc

import (
	"net"
	"strings"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// PipeName is the stable production named-pipe endpoint shared by the
// daemon and every local CLI/UI client.
const PipeName = `\\.\pipe\golc-artnet`

// ResolveCustomPipeName turns a bare "--pipe <name>" value (every artnet CLI
// route's own usage string) into a full named-pipe path in the same
// `\\.\pipe\golc-` namespace PipeName uses. A value that already looks like
// a full pipe path (starts with `\\`, e.g. a test's own literal endpoint) is
// returned unchanged.
func ResolveCustomPipeName(name string) string {
	if name == "" || strings.HasPrefix(name, `\\`) {
		return name
	}
	return `\\.\pipe\golc-` + name
}

// ownerOnlySDDL is a protected DACL granting Generic All to the owner alone.
const ownerOnlySDDL = "D:P(A;;GA;;;OW)"

func listenTransport(pipeName string) (net.Listener, error) {
	return winio.ListenPipe(pipeName, &winio.PipeConfig{SecurityDescriptor: ownerOnlySDDL})
}

func dialTransport(pipeName string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(pipeName, &timeout)
}
