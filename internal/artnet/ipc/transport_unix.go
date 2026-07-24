//go:build !windows

package ipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const unixSocketName = "artnet.sock"

// PipeName is the deterministic short per-user Unix-domain socket shared
// by separately started daemon and client processes.
var PipeName = filepath.Join("/tmp", fmt.Sprintf("golc-%d", os.Getuid()), unixSocketName)

const staleProbeTimeout = 100 * time.Millisecond

func listenTransport(endpoint string) (net.Listener, error) {
	if err := secureEndpointDirectory(filepath.Dir(endpoint)); err != nil {
		return nil, err
	}
	if err := prepareEndpoint(endpoint); err != nil {
		return nil, err
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: endpoint, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(true)

	if err := hardenSocket(endpoint); err != nil {
		_ = listener.Close()
		removeCreatedSocket(endpoint)
		return nil, err
	}
	return listener, nil
}

func dialTransport(endpoint string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", endpoint, timeout)
}

func secureEndpointDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("create endpoint directory: %w", err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("inspect endpoint directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("endpoint parent %q is not a real directory", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("endpoint directory %q is not owned by uid %d", dir, os.Getuid())
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("restrict endpoint directory: %w", err)
	}
	info, err = os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("verify endpoint directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("endpoint directory %q is not owner-only", dir)
	}
	return nil
}

func prepareEndpoint(endpoint string) error {
	info, err := os.Lstat(endpoint)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect endpoint: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket endpoint %q", endpoint)
	}

	conn, probeErr := net.DialTimeout("unix", endpoint, staleProbeTimeout)
	if probeErr == nil {
		_ = conn.Close()
		return fmt.Errorf("endpoint %q already has an active listener", endpoint)
	}
	if !errors.Is(probeErr, syscall.ECONNREFUSED) && !errors.Is(probeErr, syscall.ENOENT) && !os.IsNotExist(probeErr) {
		return fmt.Errorf("endpoint liveness probe was inconclusive: %w", probeErr)
	}
	if err := os.Remove(endpoint); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

func hardenSocket(endpoint string) error {
	if err := os.Chmod(endpoint, 0o600); err != nil {
		return fmt.Errorf("restrict socket: %w", err)
	}
	info, err := os.Lstat(endpoint)
	if err != nil {
		return fmt.Errorf("verify socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("socket %q is not owner-only", endpoint)
	}
	return nil
}

func removeCreatedSocket(endpoint string) {
	info, err := os.Lstat(endpoint)
	if err == nil && info.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(endpoint)
	}
}
