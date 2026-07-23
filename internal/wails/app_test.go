// app_test.go proves 06-04-PLAN.md Task 1's two acceptance criteria:
// OnStartup attempts a supervised daemon spawn when the pipe is
// unreachable (TestAppStartupAttemptsDaemonSpawnWhenPipeUnreachable), and
// a hotkey-registration failure is surfaced -- never silently swallowed
// (TestHotkeyRegisterSurfaced).
package wails

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.design/x/hotkey"

	"github.com/lnorton89/golc/internal/artnet/ipc"
)

// testWailsPipeName returns a per-test, per-process, per-nanosecond-unique
// pipe path, mirroring internal/artnet/ipc/ipc_test.go's testPipeName
// convention, so this package's tests never collide with a real running
// daemon or with each other.
func testWailsPipeName(t *testing.T) string {
	t.Helper()
	sanitized := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf(`\\.\pipe\golc-wails-test-%s-%d-%d`, sanitized, os.Getpid(), time.Now().UnixNano())
}

// fakeConn is a minimal net.Conn double: only Close is ever called by
// ensureDaemon, but the full interface must be satisfied to type-check as
// dialFunc's return value.
type fakeConn struct{ net.Conn }

func (fakeConn) Close() error { return nil }

// TestAppStartupAttemptsDaemonSpawnWhenPipeUnreachable proves OnStartup
// attempts exactly one supervised daemon spawn when the configured pipe is
// unreachable, retries the real ipc.Dial against that same (never
// listened-on) pipe, and leaves DaemonUnreachable() true when the spawn
// stub never actually brings a daemon up -- all without ever launching a
// real golc-project.exe.
func TestAppStartupAttemptsDaemonSpawnWhenPipeUnreachable(t *testing.T) {
	pipeName := testWailsPipeName(t)
	app := NewApp(Config{
		PipeName:       pipeName,
		DialRetries:    2,
		DialRetryDelay: time.Millisecond,
	})
	// This test exercises the daemon-spawn path only -- inject a fake
	// hotkey factory so OnStartup never touches a real OS-level global
	// hotkey (TestHotkeyRegisterSurfaced/TestHotkeyKeydownForwardsDirectlyToDaemon
	// cover the hotkey path in isolation).
	app.hotkeys.factory = func(mods []hotkey.Modifier, key hotkey.Key) registerer {
		return &fakeRegisterer{}
	}

	var spawnCalls int32
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, error) {
		atomic.AddInt32(&spawnCalls, 1)
		if cfg.PipeName != pipeName {
			t.Errorf("spawn called with PipeName %q, want %q", cfg.PipeName, pipeName)
		}
		// Simulate a spawn that starts (no error) but never actually
		// brings a daemon up on pipeName -- OnStartup's retry loop must
		// observe the pipe stays unreachable and give up rather than
		// hang.
		return nil, nil
	}

	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	if got := atomic.LoadInt32(&spawnCalls); got != 1 {
		t.Fatalf("expected exactly one daemon spawn attempt, got %d", got)
	}
	if !app.DaemonUnreachable() {
		t.Fatal("expected DaemonUnreachable() to be true after a spawn stub that never brings up a real daemon")
	}
}

// TestAppStartupSkipsSpawnWhenDaemonAlreadyReachable proves the inverse:
// when Dial already succeeds, OnStartup never attempts a spawn at all.
func TestAppStartupSkipsSpawnWhenDaemonAlreadyReachable(t *testing.T) {
	pipeName := testWailsPipeName(t)
	app := NewApp(Config{PipeName: pipeName})
	// Same isolation rationale as
	// TestAppStartupAttemptsDaemonSpawnWhenPipeUnreachable above.
	app.hotkeys.factory = func(mods []hotkey.Modifier, key hotkey.Key) registerer {
		return &fakeRegisterer{}
	}

	var spawnCalls int32
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, error) {
		atomic.AddInt32(&spawnCalls, 1)
		return nil, nil
	}
	app.dial = func(name string) (net.Conn, error) {
		return fakeConn{}, nil
	}

	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	if got := atomic.LoadInt32(&spawnCalls); got != 0 {
		t.Fatalf("expected zero daemon spawn attempts when already reachable, got %d", got)
	}
	if app.DaemonUnreachable() {
		t.Fatal("expected DaemonUnreachable() to stay false when Dial already succeeds")
	}
}

// fakeRegisterer is a test double for the registerer interface
// (hotkey.go): registerErr, when set, makes Register fail exactly the way
// a real OS-level hotkey conflict would; keydown lets a test simulate a
// Keydown event without a real OS-level global hotkey.
type fakeRegisterer struct {
	registerErr error
	keydown     chan hotkey.Event

	mu           sync.Mutex
	unregistered bool
}

func (f *fakeRegisterer) Register() error { return f.registerErr }

func (f *fakeRegisterer) Keydown() <-chan hotkey.Event {
	if f.keydown == nil {
		f.keydown = make(chan hotkey.Event)
	}
	return f.keydown
}

func (f *fakeRegisterer) Unregister() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unregistered = true
	return nil
}

// TestHotkeyRegisterSurfaced proves a hotkey-registration failure is
// surfaced (HotkeyManager.Failures()), never silently swallowed, and that
// the other two bindings still register successfully (Security Domain DoS
// mitigation: one conflict must not block the rest).
func TestHotkeyRegisterSurfaced(t *testing.T) {
	pipeName := testWailsPipeName(t)
	manager := NewHotkeyManager(pipeName)

	var registerCalls int32
	manager.factory = func(mods []hotkey.Modifier, key hotkey.Key) registerer {
		atomic.AddInt32(&registerCalls, 1)
		if key == blackoutKey {
			return &fakeRegisterer{registerErr: errors.New("hotkey: failed to register, the combination might already be taken by another application")}
		}
		return &fakeRegisterer{}
	}

	failures := manager.RegisterAll()
	defer manager.UnregisterAll()

	if got := atomic.LoadInt32(&registerCalls); got != 3 {
		t.Fatalf("expected RegisterAll to attempt all three bindings, got %d calls", got)
	}
	if len(failures) != 1 {
		t.Fatalf("expected exactly one surfaced failure, got %d: %+v", len(failures), failures)
	}
	if failures[0].Control != "blackout" {
		t.Fatalf("expected the surfaced failure to name control %q, got %q", "blackout", failures[0].Control)
	}
	if failures[0].Error == "" {
		t.Fatal("expected the surfaced failure to carry a non-empty error message")
	}

	// Failures() must report the same outcome App.OnStartup would log and
	// expose to the frontend -- never a silent pass.
	if got := manager.Failures(); len(got) != 1 {
		t.Fatalf("Failures() = %+v, want exactly one failure", got)
	}
}

// TestHotkeyKeydownForwardsDirectlyToDaemon proves a safety hotkey's
// Keydown event dials+forwards the matching daemon route directly
// (RESEARCH.md Pitfall 1: never a JS-mediated path) -- the callback lives
// entirely in hotkey.go's listen goroutine, never in frontend JS.
func TestHotkeyKeydownForwardsDirectlyToDaemon(t *testing.T) {
	pipeName := testWailsPipeName(t)
	manager := NewHotkeyManager(pipeName)

	fakes := map[hotkey.Key]*fakeRegisterer{}
	manager.factory = func(mods []hotkey.Modifier, key hotkey.Key) registerer {
		f := &fakeRegisterer{keydown: make(chan hotkey.Event, 1)}
		fakes[key] = f
		return f
	}

	forwardedCh := make(chan ipc.Request, 1)
	manager.dial = func(name string, request ipc.Request) ipc.Result {
		if name != pipeName {
			t.Errorf("dial called with pipe %q, want %q", name, pipeName)
		}
		forwardedCh <- request
		return ipc.Result{}
	}

	failures := manager.RegisterAll()
	defer manager.UnregisterAll()
	if len(failures) != 0 {
		t.Fatalf("expected all three bindings to register successfully with a fake registerer, got failures: %+v", failures)
	}

	fakes[blackoutKey].keydown <- hotkey.Event{}

	select {
	case request := <-forwardedCh:
		if request.Route != string(routeBlackout) {
			t.Fatalf("forwarded route = %q, want %q", request.Route, routeBlackout)
		}
		want := []string{"--on", "true", "--source", "manual"}
		if len(request.Args) != len(want) {
			t.Fatalf("forwarded args = %v, want %v", request.Args, want)
		}
		for i := range want {
			if request.Args[i] != want[i] {
				t.Fatalf("forwarded args = %v, want %v", request.Args, want)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the hotkey Keydown callback to dial+forward the daemon route")
	}
}
