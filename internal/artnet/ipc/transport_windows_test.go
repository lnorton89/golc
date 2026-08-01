//go:build windows

package ipc

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(`\\.\pipe\golc-artnet-test-%s-%d-%d`, sanitized, os.Getpid(), time.Now().UnixNano())
}

func TestWindowsTransportUsesStableProductionPipe(t *testing.T) {
	require.Equal(t, `\\.\pipe\golc-artnet`, PipeName, "want production named pipe")
}

func TestOwnerOnlySDDLRestrictsToOwner(t *testing.T) {
	require.Contains(t, ownerOnlySDDL, "D:P", "expected a Protected DACL (D:P prefix)")
	require.Contains(t, ownerOnlySDDL, ";OW)", "expected the sole ACE to grant access to the Owner (OW)")
}
