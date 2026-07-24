//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPipeName(t *testing.T) string {
	t.Helper()
	name := fmt.Sprintf("t-%d-%x.sock", os.Getpid(), time.Now().UnixNano())
	path := filepath.Join("/tmp", name)
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestUnixProductionEndpointIsShortStableAndPerUser(t *testing.T) {
	wantDir := filepath.Join("/tmp", fmt.Sprintf("golc-%d", os.Getuid()))
	if filepath.Dir(PipeName) != wantDir {
		t.Fatalf("PipeName directory = %q, want %q", filepath.Dir(PipeName), wantDir)
	}
	if filepath.Base(PipeName) != "artnet.sock" {
		t.Fatalf("PipeName base = %q, want artnet.sock", filepath.Base(PipeName))
	}
	if len(PipeName) >= 100 {
		t.Fatalf("PipeName is too long for portable Unix sockets: %d bytes", len(PipeName))
	}
}

func TestUnixListenerUsesOwnerOnlyModesAndUnlinksOnClose(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ipc")
	endpoint := filepath.Join(dir, "artnet.sock")

	listener, err := NewListener(endpoint)
	if err != nil {
		t.Fatalf("NewListener: %v", err)
	}

	dirInfo, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("Lstat directory: %v", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 || dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v, want real directory 0700", dirInfo.Mode())
	}
	socketInfo, err := os.Lstat(endpoint)
	if err != nil {
		t.Fatalf("Lstat socket: %v", err)
	}
	if socketInfo.Mode()&os.ModeSocket == 0 || socketInfo.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %v, want socket 0600", socketInfo.Mode())
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Lstat(endpoint); !os.IsNotExist(err) {
		t.Fatalf("socket remains after Close: %v", err)
	}
}

func TestUnixListenerPreservesActiveSocket(t *testing.T) {
	endpoint := filepath.Join(t.TempDir(), "active.sock")
	active, err := NewListener(endpoint)
	if err != nil {
		t.Fatalf("first NewListener: %v", err)
	}
	defer active.Close()

	if _, err := NewListener(endpoint); err == nil {
		t.Fatal("second NewListener unexpectedly replaced active socket")
	}
	conn, err := net.DialTimeout("unix", endpoint, time.Second)
	if err != nil {
		t.Fatalf("active listener was displaced: %v", err)
	}
	_ = conn.Close()
}

func TestUnixListenerRecoversVerifiedStaleSocket(t *testing.T) {
	endpoint := filepath.Join(t.TempDir(), "stale.sock")
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint, Net: "unix"})
	if err != nil {
		t.Fatalf("seed stale socket: %v", err)
	}
	stale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatalf("close stale socket: %v", err)
	}

	listener, err := NewListener(endpoint)
	if err != nil {
		t.Fatalf("NewListener did not recover stale socket: %v", err)
	}
	_ = listener.Close()
}

func TestUnixListenerPreservesUnsafeEndpointObjects(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(string) error
	}{
		{name: "regular file", seed: func(path string) error { return os.WriteFile(path, []byte("keep"), 0o600) }},
		{name: "directory", seed: func(path string) error { return os.Mkdir(path, 0o700) }},
		{name: "symlink", seed: func(path string) error {
			target := path + "-target"
			if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
				return err
			}
			return os.Symlink(target, path)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := filepath.Join(t.TempDir(), "endpoint")
			if err := tc.seed(endpoint); err != nil {
				t.Fatalf("seed: %v", err)
			}
			before, err := os.Lstat(endpoint)
			if err != nil {
				t.Fatalf("Lstat before: %v", err)
			}

			if _, err := NewListener(endpoint); err == nil {
				t.Fatal("NewListener unexpectedly accepted unsafe endpoint")
			}
			after, err := os.Lstat(endpoint)
			if err != nil {
				t.Fatalf("unsafe endpoint was removed: %v", err)
			}
			if before.Mode().Type() != after.Mode().Type() {
				t.Fatalf("endpoint type changed from %v to %v", before.Mode(), after.Mode())
			}
		})
	}
}

func TestUnixListenerRejectsSymlinkParent(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(root, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatal(err)
	}

	_, err := NewListener(filepath.Join(linkDir, "artnet.sock"))
	if err == nil || !strings.Contains(err.Error(), "GOLC_ARTNET_IPC_LISTEN_FAILED") {
		t.Fatalf("expected safe listen failure for symlink parent, got %v", err)
	}
}
