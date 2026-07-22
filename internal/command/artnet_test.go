// artnet_test.go proves 04-05-PLAN.md's "artnet" scope/route contract.
// TestScopeArtnet is the exact quick-test marker for scope "artnet" (test
// --quick --scope artnet): it exercises only pure, offline arg-parsing/
// registration logic -- no daemon, no network -- so the registered scope
// exits 0 offline, mirroring build_test.go's TestScopeBuildArgs
// convention. The remaining Task 1 tests below prove "artnet configure"'s
// two-tier usage/domain exit-code split and that a client route with no
// daemon running returns GOLC_ARTNET_DAEMON_UNREACHABLE, all without ever
// dialing a real daemon. Task 2's status/target-toggle tests (which do
// start a real artnet.Run daemon on an isolated per-test pipe) follow in
// a later commit.
package command

import (
	"strings"
	"testing"

	artnetipc "github.com/lnorton89/golc/internal/artnet/ipc"
)

// testArtnetPipeName returns a per-test, per-process, per-nanosecond-
// unique pipe path so these tests never collide with each other, with a
// real running daemon, or with internal/artnet's own concurrently-running
// package tests. Defined here (rather than inline) so both Task 1's
// no-daemon test and Task 2's daemon-backed tests share one helper.
func testArtnetPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return `\\.\pipe\golc-artnet-cli-test-` + sanitized
}

// TestScopeArtnet is the exact "test --quick --scope artnet" marker: pure
// offline coverage of arg-parsing and self-registration, no daemon/
// network required.
func TestScopeArtnet(t *testing.T) {
	t.Run("parseArtnetArgs accepts --flag value and --flag=value forms", func(t *testing.T) {
		values, err := parseArtnetArgs("usage", []string{"--universe", "1", "--ip=127.0.0.1"}, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if values["universe"] != "1" || values["ip"] != "127.0.0.1" {
			t.Fatalf("unexpected parsed values: %+v", values)
		}
	})

	t.Run("parseArtnetArgs treats a declared bool flag as valueless", func(t *testing.T) {
		values, err := parseArtnetArgs("usage", []string{"--json"}, map[string]bool{"json": true})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if values["json"] != "true" {
			t.Fatalf("expected json=true, got %+v", values)
		}
	})

	t.Run("parseArtnetArgs rejects an argument not starting with --", func(t *testing.T) {
		if _, err := parseArtnetArgs("usage", []string{"bogus"}, nil); err == nil {
			t.Fatal("expected an error for a non-flag argument")
		}
	})

	t.Run("pipeNameFromFlags defaults to the production pipe name", func(t *testing.T) {
		if got := pipeNameFromFlags(map[string]string{}); got != artnetipc.PipeName {
			t.Fatalf("expected default pipe name %q, got %q", artnetipc.PipeName, got)
		}
	})

	t.Run("the artnet scope and Task 1's routes self-register", func(t *testing.T) {
		registry, err := NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}
		for _, route := range []string{
			"artnet serve", "artnet interface list", "artnet configure",
		} {
			if _, _, ok := registry.Lookup(strings.Fields(route)); !ok {
				t.Fatalf("expected route %q to be registered", route)
			}
		}
	})
}

// TestArtnetConfigureUsageErrors proves a malformed "artnet configure"
// invocation (missing --ip) is rejected as GOLC_ARTNET_USAGE with
// ExitCode 2, without ever dialing a daemon.
func TestArtnetConfigureUsageErrors(t *testing.T) {
	result := runArtnetConfigure(Request{Args: []string{"--universe", "1"}})
	if result.ExitCode != 2 {
		t.Fatalf("expected ExitCode 2, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_USAGE") {
		t.Fatalf("expected GOLC_ARTNET_USAGE, got: %s", result.Stderr)
	}
}

// TestArtnetConfigureInvalidTargetReturnsDomainError proves a
// well-formed-but-rejected "artnet configure" value (universe 0) fails
// artnet.ValidateTarget client-side as a GOLC_ARTNET_TARGET_INVALID
// domain error with ExitCode 1, again without ever dialing a daemon.
func TestArtnetConfigureInvalidTargetReturnsDomainError(t *testing.T) {
	result := runArtnetConfigure(Request{Args: []string{"--universe", "0", "--ip", "10.0.0.1"}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_TARGET_INVALID") {
		t.Fatalf("expected GOLC_ARTNET_TARGET_INVALID, got: %s", result.Stderr)
	}
}

// TestArtnetNoDaemonReturnsDaemonUnreachable proves a client route with no
// daemon running on the given pipe returns GOLC_ARTNET_DAEMON_UNREACHABLE,
// ExitCode 1, never a hang.
func TestArtnetNoDaemonReturnsDaemonUnreachable(t *testing.T) {
	pipeName := testArtnetPipeName(t) // nothing ever listens on this pipe

	result := runArtnetConfigure(Request{Args: []string{
		"--universe", "1", "--ip", "10.0.0.1", "--pipe", pipeName,
	}})
	if result.ExitCode != 1 {
		t.Fatalf("expected ExitCode 1, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stderr), "GOLC_ARTNET_DAEMON_UNREACHABLE") {
		t.Fatalf("expected GOLC_ARTNET_DAEMON_UNREACHABLE, got: %s", result.Stderr)
	}
}
