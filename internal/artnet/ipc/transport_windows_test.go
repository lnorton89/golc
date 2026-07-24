//go:build windows

package ipc

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func testPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(`\\.\pipe\golc-artnet-test-%s-%d-%d`, sanitized, os.Getpid(), time.Now().UnixNano())
}

func TestWindowsTransportUsesStableProductionPipe(t *testing.T) {
	if PipeName != `\\.\pipe\golc-artnet` {
		t.Fatalf("PipeName = %q, want production named pipe", PipeName)
	}
}

func TestOwnerOnlySDDLRestrictsToOwner(t *testing.T) {
	if !strings.Contains(ownerOnlySDDL, "D:P") {
		t.Fatalf("expected a Protected DACL (D:P prefix), got %q", ownerOnlySDDL)
	}
	if !strings.Contains(ownerOnlySDDL, ";OW)") {
		t.Fatalf("expected the sole ACE to grant access to the Owner (OW), got %q", ownerOnlySDDL)
	}
}
