//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// shortTestDir returns a short, per-test directory under /tmp rather than
// t.TempDir(): a Unix domain socket path is limited to sizeof(sun_path)
// (~104 bytes on macOS, ~108 on Linux), and t.TempDir()'s own path
// already embeds the full test name plus the system temp root -- on
// macOS that root is itself /private/var/folders/<hash>/T/, so the
// combination routinely exceeds the limit long before any socket
// filename is even appended (observed live: cross-platform-mage.yml run
// 30110425773 failed three transport_unix_test.go tests on macos-latest
// with "bind: invalid argument", the classic oversized-sun_path
// symptom; linux-amd64's slightly larger limit did not trip it, but the
// same class of failure could on a longer runner path).
//
// Unlike t.TempDir(), this directory is not created automatically, so it
// is created here: NewListener's endpoint-directory creation is a single
// os.Mkdir (production only ever needs one level under the always-present
// /tmp), and callers that append their own subdirectory rely on this
// directory already existing (observed live in run 30111784838: without
// this, NewListener failed with "no such file or directory" on both
// macos-latest and ubuntu-latest).
func shortTestDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("/tmp", fmt.Sprintf("golc-t-%d-%x", os.Getpid(), time.Now().UnixNano()))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func testPipeName(t *testing.T) string {
	t.Helper()
	endpoint := filepath.Join(shortTestDir(t), "artnet.sock")
	return endpoint
}

func TestUnixProductionEndpointIsShortStableAndPerUser(t *testing.T) {
	wantDir := filepath.Join("/tmp", fmt.Sprintf("golc-%d", os.Getuid()))
	require.Equal(t, wantDir, filepath.Dir(PipeName), "PipeName directory")
	require.Equal(t, "artnet.sock", filepath.Base(PipeName), "PipeName base")
	require.Less(t, len(PipeName), 100, "PipeName is too long for portable Unix sockets")
}

func TestUnixListenerUsesOwnerOnlyModesAndUnlinksOnClose(t *testing.T) {
	dir := filepath.Join(shortTestDir(t), "ipc")
	endpoint := filepath.Join(dir, "artnet.sock")

	listener, err := NewListener(endpoint)
	require.NoError(t, err, "NewListener")

	dirInfo, err := os.Lstat(dir)
	require.NoError(t, err, "Lstat directory")
	require.True(t, dirInfo.IsDir(), "directory mode = %v, want real directory 0700", dirInfo.Mode())
	require.Equal(t, os.FileMode(0), dirInfo.Mode()&os.ModeSymlink, "directory mode = %v, want real directory 0700", dirInfo.Mode())
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "directory mode = %v, want real directory 0700", dirInfo.Mode())
	socketInfo, err := os.Lstat(endpoint)
	require.NoError(t, err, "Lstat socket")
	require.NotEqual(t, os.FileMode(0), socketInfo.Mode()&os.ModeSocket, "socket mode = %v, want socket 0600", socketInfo.Mode())
	require.Equal(t, os.FileMode(0o600), socketInfo.Mode().Perm(), "socket mode = %v, want socket 0600", socketInfo.Mode())

	require.NoError(t, listener.Close(), "Close")
	_, err = os.Lstat(endpoint)
	require.True(t, os.IsNotExist(err), "socket remains after Close: %v", err)
}

func TestUnixListenerPreservesActiveSocket(t *testing.T) {
	endpoint := filepath.Join(shortTestDir(t), "active.sock")
	active, err := NewListener(endpoint)
	require.NoError(t, err, "first NewListener")
	defer active.Close()

	_, err = NewListener(endpoint)
	require.Error(t, err, "second NewListener unexpectedly replaced active socket")
	conn, err := net.DialTimeout("unix", endpoint, time.Second)
	require.NoError(t, err, "active listener was displaced")
	_ = conn.Close()
}

func TestUnixListenerRecoversVerifiedStaleSocket(t *testing.T) {
	endpoint := filepath.Join(shortTestDir(t), "stale.sock")
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint, Net: "unix"})
	require.NoError(t, err, "seed stale socket")
	stale.SetUnlinkOnClose(false)
	require.NoError(t, stale.Close(), "close stale socket")

	listener, err := NewListener(endpoint)
	require.NoError(t, err, "NewListener did not recover stale socket")
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
			dir := shortTestDir(t)
			require.NoError(t, os.MkdirAll(dir, 0o700))
			endpoint := filepath.Join(dir, "endpoint")
			require.NoError(t, tc.seed(endpoint), "seed")
			before, err := os.Lstat(endpoint)
			require.NoError(t, err, "Lstat before")

			_, err = NewListener(endpoint)
			require.Error(t, err, "NewListener unexpectedly accepted unsafe endpoint")
			after, err := os.Lstat(endpoint)
			require.NoError(t, err, "unsafe endpoint was removed")
			require.Equal(t, before.Mode().Type(), after.Mode().Type(), "endpoint type changed")
		})
	}
}

func TestUnixListenerRejectsSymlinkParent(t *testing.T) {
	root := shortTestDir(t)
	require.NoError(t, os.MkdirAll(root, 0o700))
	realDir := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(realDir, 0o700))
	linkDir := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(realDir, linkDir))

	_, err := NewListener(filepath.Join(linkDir, "artnet.sock"))
	require.ErrorContains(t, err, "GOLC_ARTNET_IPC_LISTEN_FAILED", "expected safe listen failure for symlink parent")
}
