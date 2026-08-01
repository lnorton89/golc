// app_test.go proves 06-04-PLAN.md Task 1's two acceptance criteria:
// OnStartup attempts a supervised daemon spawn when the pipe is
// unreachable (TestAppStartupAttemptsDaemonSpawnWhenPipeUnreachable), and
// a hotkey-registration failure is surfaced -- never silently swallowed
// (TestHotkeyRegisterSurfaced).
package wails

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.design/x/hotkey"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/bootstrap"
)

// testWailsPipeName returns a per-test, per-process, per-nanosecond-unique
// pipe path, mirroring internal/artnet/ipc/ipc_test.go's testPipeName
// convention, so this package's tests never collide with a real running
// daemon or with each other.
func testWailsPipeName(t *testing.T) string {
	t.Helper()
	return platformTestEndpoint(t, "wails")
}

func platformTestEndpoint(t *testing.T, prefix string) string {
	t.Helper()
	nameHash := sha256.Sum256([]byte(t.Name()))
	suffix := fmt.Sprintf("%s-%d-%d-%x", prefix, os.Getpid(), time.Now().UnixNano(), nameHash[:4])
	if runtime.GOOS == "windows" {
		return `\\.\pipe\golc-` + suffix
	}
	dir := filepath.Join("/tmp", "golc-"+suffix)
	endpoint := filepath.Join(dir, "artnet.sock")
	t.Cleanup(func() {
		_ = os.Remove(endpoint)
		_ = os.Remove(dir)
	})
	return endpoint
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
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
		atomic.AddInt32(&spawnCalls, 1)
		assert.Equal(t, pipeName, cfg.PipeName, "spawn called with unexpected PipeName")
		// Simulate a spawn that starts (no error) but never actually
		// brings a daemon up on pipeName -- OnStartup's retry loop must
		// observe the pipe stays unreachable and give up rather than
		// hang.
		return nil, nil, nil
	}

	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one daemon spawn attempt")
	require.True(t, app.DaemonUnreachable(), "expected DaemonUnreachable() to be true after a spawn stub that never brings up a real daemon")
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
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
		atomic.AddInt32(&spawnCalls, 1)
		return nil, nil, nil
	}
	app.dial = func(name string) (net.Conn, error) {
		return fakeConn{}, nil
	}

	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	require.Equal(t, int32(0), atomic.LoadInt32(&spawnCalls), "expected zero daemon spawn attempts when already reachable")
	require.False(t, app.DaemonUnreachable(), "expected DaemonUnreachable() to stay false when Dial already succeeds")
}

// TestResolveDaemonExecutableDefaultIncludesPlatformKey proves the unset-
// DaemonExecutable default path resolves through bootstrap.PlatformExecutablePath
// (i.e. includes the runtime.GOOS-runtime.GOARCH platform key between the
// install root and bin/), matching how the real bootstrap step
// (internal/bootstrap/engine.go) and internal/delivery/graph.go both
// resolve commands.cli_binary. A prior hardcoded-relative-path bug omitted
// the platform key, so a real bootstrap's golc-project.exe could never be
// found and golc-desktop always fell through to the degraded
// DaemonUnreachable path.
func TestResolveDaemonExecutableDefaultIncludesPlatformKey(t *testing.T) {
	root := t.TempDir()
	got, err := resolveDaemonExecutable(Config{ProjectRoot: root})
	require.NoError(t, err, "resolveDaemonExecutable")
	want := bootstrap.PlatformExecutablePath(filepath.Join(root, filepath.FromSlash(defaultCliBinaryInstallRoot)), "golc-project")
	require.Equal(t, want, got)
	require.Contains(t, got, bootstrap.PlatformKey())
}

// TestResolveDaemonExecutableOverrideWins proves an explicit
// Config.DaemonExecutable always wins over the default resolution, and that
// no ProjectRoot is required in that case.
func TestResolveDaemonExecutableOverrideWins(t *testing.T) {
	override := filepath.Join("some", "explicit", "path", "golc-project.exe")
	got, err := resolveDaemonExecutable(Config{DaemonExecutable: override})
	require.NoError(t, err, "resolveDaemonExecutable")
	require.Equal(t, override, got)
}

// fakeRegisterer is a test double for the registerer interface
// (hotkey.go): registerErr, when set, makes Register fail exactly the way
// a real OS-level hotkey conflict would; registerPanic, when set, makes
// Register panic instead -- simulating golang.design/x/hotkey's CGo-free,
// non-Windows build (hotkey_nocgo.go), which panics rather than returning
// an error; keydown lets a test simulate a Keydown event without a real
// OS-level global hotkey.
type fakeRegisterer struct {
	registerErr   error
	registerPanic any
	keydown       chan hotkey.Event

	mu           sync.Mutex
	unregistered bool
}

func (f *fakeRegisterer) Register() error {
	if f.registerPanic != nil {
		panic(f.registerPanic)
	}
	return f.registerErr
}

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

	require.Equal(t, int32(3), atomic.LoadInt32(&registerCalls), "expected RegisterAll to attempt all three bindings")
	require.Len(t, failures, 1, "expected exactly one surfaced failure: %+v", failures)
	require.Equal(t, "blackout", failures[0].Control, "expected the surfaced failure to name control %q", "blackout")
	require.NotEmpty(t, failures[0].Error, "expected the surfaced failure to carry a non-empty error message")

	// Failures() must report the same outcome App.OnStartup would log and
	// expose to the frontend -- never a silent pass.
	gotFailures := manager.Failures()
	require.Len(t, gotFailures, 1, "Failures() = %+v, want exactly one failure", gotFailures)
}

// TestHotkeyRegisterPanicSurfacedNotCrashed proves a Register() call that
// panics (golang.design/x/hotkey's actual behavior on a CGo-free,
// non-Windows build -- hotkey_nocgo.go panics unconditionally rather than
// returning an error) is caught by safeRegister and surfaced as a normal
// HotkeyFailure, and that the other two bindings still register
// successfully -- the same Security Domain DoS mitigation
// TestHotkeyRegisterSurfaced proves for a returned error must also hold for
// a panic, or a single unsupported-platform hotkey call would crash the
// entire desktop process before PLAY-09's safety cluster ever came up.
func TestHotkeyRegisterPanicSurfacedNotCrashed(t *testing.T) {
	pipeName := testWailsPipeName(t)
	manager := NewHotkeyManager(pipeName)

	var registerCalls int32
	manager.factory = func(mods []hotkey.Modifier, key hotkey.Key) registerer {
		atomic.AddInt32(&registerCalls, 1)
		if key == blackoutKey {
			return &fakeRegisterer{registerPanic: "hotkey: cannot use when CGO_ENABLED=0"}
		}
		return &fakeRegisterer{}
	}

	failures := manager.RegisterAll()
	defer manager.UnregisterAll()

	require.Equal(t, int32(3), atomic.LoadInt32(&registerCalls), "expected RegisterAll to attempt all three bindings")
	require.Len(t, failures, 1, "expected exactly one surfaced failure: %+v", failures)
	require.Equal(t, "blackout", failures[0].Control, "expected the surfaced failure to name control %q", "blackout")
	require.NotEmpty(t, failures[0].Error, "expected the surfaced failure to carry a non-empty error message")
}

// hotkeyStatusJSON builds a minimal "artnet status" JSON response body
// mirroring daemonStatusJSON (svc_safety_test.go) -- used here to control
// nextToggleValue's (CR-03 fix) query response independent of the actual
// toggle request under test.
func hotkeyStatusJSON(t *testing.T, controllingSource, outputState string) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]interface{}{"playback": map[string]interface{}{
		"controllingSource": controllingSource,
		"outputState":       outputState,
	}})
	require.NoError(t, err, "json.Marshal")
	return encoded
}

// TestHotkeyKeydownForwardsDirectlyToDaemon proves a safety hotkey's
// Keydown event queries the daemon's current state (CR-03's
// nextToggleValue) and then dials+forwards the matching daemon route
// directly (RESEARCH.md Pitfall 1: never a JS-mediated path) -- the
// callback lives entirely in hotkey.go's listen goroutine, never in
// frontend JS. The daemon here reports no active override, so the toggle
// activates ("--on true"), mirroring this test's pre-CR-03 behavior.
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
		assert.Equal(t, pipeName, name, "dial called with unexpected pipe")
		if request.Route == "artnet status" {
			return ipc.Result{Stdout: hotkeyStatusJSON(t, "live", "frame-lock")}
		}
		forwardedCh <- request
		return ipc.Result{}
	}

	failures := manager.RegisterAll()
	defer manager.UnregisterAll()
	require.Empty(t, failures, "expected all three bindings to register successfully with a fake registerer")

	fakes[blackoutKey].keydown <- hotkey.Event{}

	select {
	case request := <-forwardedCh:
		require.Equal(t, string(routeBlackout), request.Route, "forwarded route")
		want := []string{"--on", "true", "--source", "manual"}
		require.Equal(t, want, request.Args, "forwarded args")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the hotkey Keydown callback to dial+forward the daemon route")
	}
}

// TestHotkeyKeydownReleasesWhenAlreadyActive proves CR-03's fix: when the
// daemon reports Blackout/Stop-All's combined "blackout" outputState
// already active, a second Keydown on the same hotkey forwards "--on
// false" (release) instead of re-forwarding "--on true" -- there is now an
// OS-level release path, mirroring SafetyCluster.tsx's identical toggle
// fix for the on-screen hold buttons.
func TestHotkeyKeydownReleasesWhenAlreadyActive(t *testing.T) {
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
		if request.Route == "artnet status" {
			return ipc.Result{Stdout: hotkeyStatusJSON(t, "blackout", "blackout")}
		}
		forwardedCh <- request
		return ipc.Result{}
	}

	failures := manager.RegisterAll()
	defer manager.UnregisterAll()
	require.Empty(t, failures, "expected all three bindings to register successfully with a fake registerer")

	fakes[blackoutKey].keydown <- hotkey.Event{}

	select {
	case request := <-forwardedCh:
		want := []string{"--on", "false", "--source", "manual"}
		require.Equal(t, want, request.Args, "forwarded args")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the hotkey Keydown callback to dial+forward the daemon route")
	}
}

// --- 09-02-PLAN.md Task 1: RelaunchWithShow / PickShowPath / PickNewShowPath ---
//
// These tests pin RelaunchWithShow's exact contract (D-05/D-06/D-07): the
// injected app.relaunchSpawn/app.quit fields are overwritten directly, this
// package's own test-double discipline (mirrors app.spawn/app.dial above --
// never an exported setter). newTestRelaunchApp seeds a fresh, always-
// openable per-test root/show path (mirrors svc_show_test.go's
// newTestShowService) so "show save" succeeds by default; individual tests
// override relaunchSpawn/quit/dial/spawn as needed.

// newTestRelaunchApp constructs an App whose Config.ProjectRoot/ShowPath
// point at a fresh per-test location where "show save" succeeds, with ctx
// set directly (mirroring OnStartup's own first-statement assignment)
// so RelaunchWithShow's ensureDaemon/quit calls never see a nil context.
func newTestRelaunchApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	showPath := filepath.Join(root, "show.golc")
	app := NewApp(Config{ProjectRoot: root, ShowPath: showPath})
	app.ctx = context.Background()
	return app
}

// lastEnvValue scans env for the LAST "key=" entry and returns its value --
// os/exec deduplicates environment entries keeping the last occurrence, so
// this is the value the child process would actually observe.
func lastEnvValue(env []string, key string) string {
	prefix := key + "="
	value := ""
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
		}
	}
	return value
}

// TestRelaunchWithShowRejectsEmptyPath proves an empty or whitespace-only
// path is refused before any save/spawn/quit, carrying
// GOLC_WAILS_RELAUNCH_PATH_EMPTY.
func TestRelaunchWithShowRejectsEmptyPath(t *testing.T) {
	for _, path := range []string{"", "   "} {
		t.Run(fmt.Sprintf("path=%q", path), func(t *testing.T) {
			app := newTestRelaunchApp(t)

			var spawnCalls, quitCalls int32
			app.relaunchSpawn = func(exePath string, env []string) error {
				atomic.AddInt32(&spawnCalls, 1)
				return nil
			}
			app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

			result := app.RelaunchWithShow(path)
			require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit code for path %q", path)
			require.Contains(t, result.Stderr, "GOLC_WAILS_RELAUNCH_PATH_EMPTY")
			require.Equal(t, int32(0), atomic.LoadInt32(&spawnCalls), "expected zero spawn calls")
			require.Equal(t, int32(0), atomic.LoadInt32(&quitCalls), "expected zero quit calls")
		})
	}
}

// TestRelaunchWithShowAcceptsNonExistentAndCurrentPathsWithNoSpecialCase
// proves the remaining two BOUNDARY clauses (09-02-PLAN.md must_haves): a
// path whose file does not exist yet is accepted (D-06 -- the identical
// relaunch path a brand-new show takes, never a distinct "create" branch),
// and a path equal to the currently-open show (cfg.ShowPath) is accepted
// and takes the identical relaunch path with no special-case short
// circuit -- both spawn and quit exactly once, just like any other path.
func TestRelaunchWithShowAcceptsNonExistentAndCurrentPathsWithNoSpecialCase(t *testing.T) {
	t.Run("path whose file does not exist yet", func(t *testing.T) {
		app := newTestRelaunchApp(t)
		nonExistent := filepath.Join(t.TempDir(), "never-created.golc")

		var spawnCalls, quitCalls int32
		app.relaunchSpawn = func(exePath string, env []string) error {
			atomic.AddInt32(&spawnCalls, 1)
			return nil
		}
		app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

		result := app.RelaunchWithShow(nonExistent)
		require.Equal(t, 0, result.ExitCode, "expected a not-yet-existing path to be accepted, got stderr=%s", result.Stderr)
		require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one spawn call")
		require.Equal(t, int32(1), atomic.LoadInt32(&quitCalls), "expected exactly one quit call")
	})

	t.Run("path equal to the currently-open show", func(t *testing.T) {
		app := newTestRelaunchApp(t)

		var spawnCalls, quitCalls int32
		var gotEnv []string
		app.relaunchSpawn = func(exePath string, env []string) error {
			atomic.AddInt32(&spawnCalls, 1)
			gotEnv = env
			return nil
		}
		app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

		result := app.RelaunchWithShow(app.cfg.ShowPath)
		require.Equal(t, 0, result.ExitCode, "expected the current show path to be accepted with no special case, got stderr=%s", result.Stderr)
		require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one spawn call (the identical relaunch path)")
		require.Equal(t, int32(1), atomic.LoadInt32(&quitCalls), "expected exactly one quit call")
		require.Equal(t, app.cfg.ShowPath, lastEnvValue(gotEnv, DesktopShowPathEnvName), "relaunchSpawn env's last %s", DesktopShowPathEnvName)
	})
}

// TestRelaunchWithShowPassesShowPathThroughEnvironmentVerbatim proves the
// injected relaunchSpawn double receives an env slice whose last
// GOLC_DESKTOP_SHOW= assignment is byte-identical to the requested path
// (including a space and a non-ASCII character), and that the executable
// argument is the running executable, not the daemon executable.
func TestRelaunchWithShowPassesShowPathThroughEnvironmentVerbatim(t *testing.T) {
	app := newTestRelaunchApp(t)

	wantExe, err := os.Executable()
	require.NoError(t, err, "os.Executable")

	newPath := filepath.Join(t.TempDir(), "spécial dir", "shöw with spaces.golc")

	var gotExe string
	var gotEnv []string
	app.relaunchSpawn = func(exePath string, env []string) error {
		gotExe = exePath
		gotEnv = env
		return nil
	}
	app.quit = func(ctx context.Context) {}

	result := app.RelaunchWithShow(newPath)
	require.Equal(t, 0, result.ExitCode, "expected success, got stderr=%s", result.Stderr)
	require.Equal(t, wantExe, gotExe, "relaunchSpawn exePath should be the running executable")
	require.Equal(t, newPath, lastEnvValue(gotEnv, DesktopShowPathEnvName))
}

// TestRelaunchWithShowQuitsOnlyAfterSuccessfulSpawn proves quit is called
// exactly once when the spawn double succeeds, and never called (with
// GOLC_WAILS_RELAUNCH_SPAWN_FAILED surfaced) when it returns an error.
func TestRelaunchWithShowQuitsOnlyAfterSuccessfulSpawn(t *testing.T) {
	t.Run("spawn succeeds", func(t *testing.T) {
		app := newTestRelaunchApp(t)

		var quitCalls int32
		app.relaunchSpawn = func(exePath string, env []string) error { return nil }
		app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

		result := app.RelaunchWithShow(filepath.Join(t.TempDir(), "new.golc"))
		require.Equal(t, 0, result.ExitCode, "expected success, got stderr=%s", result.Stderr)
		require.Equal(t, int32(1), atomic.LoadInt32(&quitCalls), "expected exactly one quit call")
	})

	t.Run("spawn fails", func(t *testing.T) {
		app := newTestRelaunchApp(t)
		app.cfg.PipeName = testWailsPipeName(t)
		app.cfg.DialRetries = 1
		app.cfg.DialRetryDelay = time.Millisecond
		// Safe doubles for the daemon-supervision fields ensureDaemon
		// touches on the spawn-failure recovery path -- this test must
		// never touch a real named pipe or spawn a real golc-project.exe.
		app.dial = func(name string) (net.Conn, error) { return nil, errors.New("unreachable") }
		app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
			return nil, nil, nil
		}

		var quitCalls int32
		app.relaunchSpawn = func(exePath string, env []string) error {
			return errors.New("boom")
		}
		app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

		result := app.RelaunchWithShow(filepath.Join(t.TempDir(), "new.golc"))
		require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit code when spawn fails")
		require.Contains(t, result.Stderr, "GOLC_WAILS_RELAUNCH_SPAWN_FAILED")
		require.Equal(t, int32(0), atomic.LoadInt32(&quitCalls), "expected zero quit calls when spawn fails")
	})
}

// TestRelaunchWithShowIsNotReentrant proves a second concurrent
// RelaunchWithShow call, issued while the first is blocked inside its spawn
// double, returns GOLC_WAILS_RELAUNCH_IN_PROGRESS and never invokes spawn a
// second time.
func TestRelaunchWithShowIsNotReentrant(t *testing.T) {
	app := newTestRelaunchApp(t)

	started := make(chan struct{})
	release := make(chan struct{})
	var spawnCalls int32
	app.relaunchSpawn = func(exePath string, env []string) error {
		atomic.AddInt32(&spawnCalls, 1)
		close(started)
		<-release
		return nil
	}
	var quitCalls int32
	app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- app.RelaunchWithShow(filepath.Join(t.TempDir(), "new.golc"))
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the first relaunch's spawn to start")
	}

	second := app.RelaunchWithShow(filepath.Join(t.TempDir(), "another.golc"))
	require.NotEqual(t, 0, second.ExitCode, "expected the concurrent call to fail with a non-zero exit code")
	require.Contains(t, second.Stderr, "GOLC_WAILS_RELAUNCH_IN_PROGRESS")
	require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one spawn call while the first was in flight")

	close(release)
	select {
	case first := <-resultCh:
		require.Equal(t, 0, first.ExitCode, "expected the first relaunch to succeed, got stderr=%s", first.Stderr)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the first relaunch to finish")
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one spawn call in total")
	require.Equal(t, int32(1), atomic.LoadInt32(&quitCalls), "expected exactly one quit call from the first (successful) relaunch")
}

// TestRelaunchWithShowAbortsWhenSaveFails proves a save failure (here: the
// configured show path is a pre-existing file that is not a valid GOLC/
// SQLite document, so "show save" cannot succeed) aborts the switch with
// GOLC_WAILS_RELAUNCH_SAVE_FAILED and neither spawns nor quits.
func TestRelaunchWithShowAbortsWhenSaveFails(t *testing.T) {
	root := t.TempDir()
	showPath := filepath.Join(root, "corrupt.golc")
	err := os.WriteFile(showPath, []byte("this is not a sqlite database"), 0o644)
	require.NoError(t, err, "seeding a corrupt show file")

	app := NewApp(Config{ProjectRoot: root, ShowPath: showPath})
	app.ctx = context.Background()

	var spawnCalls, quitCalls int32
	app.relaunchSpawn = func(exePath string, env []string) error {
		atomic.AddInt32(&spawnCalls, 1)
		return nil
	}
	app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

	result := app.RelaunchWithShow(filepath.Join(root, "new-show.golc"))
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit code when the working show cannot be saved")
	require.Contains(t, result.Stderr, "GOLC_WAILS_RELAUNCH_SAVE_FAILED")
	require.Equal(t, int32(0), atomic.LoadInt32(&spawnCalls), "expected zero spawn calls when save fails")
	require.Equal(t, int32(0), atomic.LoadInt32(&quitCalls), "expected zero quit calls when save fails")

	// The relaunching guard must have been cleared on this failure path --
	// a subsequent call must be able to proceed rather than incorrectly
	// reporting GOLC_WAILS_RELAUNCH_IN_PROGRESS forever.
	second := app.RelaunchWithShow(filepath.Join(root, "another-new-show.golc"))
	require.NotContains(t, second.Stderr, "GOLC_WAILS_RELAUNCH_IN_PROGRESS", "expected the relaunching guard to be cleared after a save failure")
}

// --- SelectInterface (restarting the daemon on a new network interface,
// without relaunching the rest of the app) ---
//
// SelectInterface drives ensureDaemon's own dial-or-spawn-and-retry loop
// directly, in this same process -- never a second process -- so
// app.dial/app.spawn (not app.relaunchSpawn/app.quit) are the doubles that
// matter here.

// newTestInterfaceSwitchApp mirrors newTestRelaunchApp but also seeds fast
// dial-retry knobs, a fake pipe name, and a starting pinned interface, so
// tests can drive ensureDaemon's retry loop quickly and assert on the
// before/after interface config.
func newTestInterfaceSwitchApp(t *testing.T) *App {
	t.Helper()
	app := newTestRelaunchApp(t)
	app.cfg.PipeName = testWailsPipeName(t)
	app.cfg.DialRetries = 1
	app.cfg.DialRetryDelay = time.Millisecond
	app.cfg.InterfaceIndex = 1
	app.cfg.InterfaceName = "original-nic"
	return app
}

// TestSelectInterfaceRestartsDaemonWithoutRelaunchingTheApp proves a
// successful switch updates cfg.InterfaceIndex/InterfaceName, drives
// exactly one daemon-spawn attempt for the new interface, and never
// touches relaunchSpawn/quit -- this process itself is never relaunched,
// only its supervised daemon child.
func TestSelectInterfaceRestartsDaemonWithoutRelaunchingTheApp(t *testing.T) {
	app := newTestInterfaceSwitchApp(t)

	var dialCalls, spawnCalls, relaunchSpawnCalls, quitCalls int32
	app.dial = func(name string) (net.Conn, error) {
		if atomic.AddInt32(&dialCalls, 1) == 1 {
			return nil, errors.New("unreachable")
		}
		return fakeConn{}, nil
	}
	var gotCfg Config
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
		atomic.AddInt32(&spawnCalls, 1)
		gotCfg = cfg
		return nil, nil, nil
	}
	app.relaunchSpawn = func(exePath string, env []string) error {
		atomic.AddInt32(&relaunchSpawnCalls, 1)
		return nil
	}
	app.quit = func(ctx context.Context) { atomic.AddInt32(&quitCalls, 1) }

	result := app.SelectInterface(42, "new-nic")
	require.Equal(t, 0, result.ExitCode, "expected success, got stderr=%s", result.Stderr)
	require.Equal(t, int32(1), atomic.LoadInt32(&spawnCalls), "expected exactly one daemon spawn attempt")
	require.Equal(t, int32(0), atomic.LoadInt32(&relaunchSpawnCalls), "expected zero relaunchSpawn calls -- this process itself must not relaunch")
	require.Equal(t, int32(0), atomic.LoadInt32(&quitCalls), "expected zero quit calls -- this process itself must not exit")
	require.Equal(t, 42, gotCfg.InterfaceIndex, "spawn cfg InterfaceIndex")
	require.Equal(t, "new-nic", gotCfg.InterfaceName, "spawn cfg InterfaceName")
	app.mu.Lock()
	gotIndex, gotName := app.cfg.InterfaceIndex, app.cfg.InterfaceName
	app.mu.Unlock()
	require.Equal(t, 42, gotIndex, "app.cfg InterfaceIndex after switch")
	require.Equal(t, "new-nic", gotName, "app.cfg InterfaceName after switch")
	require.False(t, app.DaemonUnreachable(), "expected DaemonUnreachable() to be false after a successful switch")
}

// TestSelectInterfaceRevertsOnUnreachableDaemon proves that when the daemon
// never becomes reachable on the requested interface, SelectInterface rolls
// the in-memory config back to the previously pinned interface, attempts a
// second spawn against it, and reports GOLC_WAILS_INTERFACE_SWITCH_FAILED
// rather than leaving the operator stranded on a dead interface.
func TestSelectInterfaceRevertsOnUnreachableDaemon(t *testing.T) {
	app := newTestInterfaceSwitchApp(t)

	var spawnCalls int32
	var mu sync.Mutex
	var spawnedIndexes []int
	app.dial = func(name string) (net.Conn, error) { return nil, errors.New("unreachable") }
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
		atomic.AddInt32(&spawnCalls, 1)
		mu.Lock()
		spawnedIndexes = append(spawnedIndexes, cfg.InterfaceIndex)
		mu.Unlock()
		return nil, nil, nil
	}

	result := app.SelectInterface(99, "bad-nic")
	require.NotEqual(t, 0, result.ExitCode, "expected a non-zero exit code when the daemon never becomes reachable")
	require.Contains(t, result.Stderr, "GOLC_WAILS_INTERFACE_SWITCH_FAILED")
	require.Equal(t, int32(2), atomic.LoadInt32(&spawnCalls), "expected exactly two spawn attempts (new interface, then rollback)")
	mu.Lock()
	got := append([]int(nil), spawnedIndexes...)
	mu.Unlock()
	require.Equal(t, []int{99, 1}, got, "spawned interface indexes (attempted, then reverted to the original)")
	app.mu.Lock()
	gotIndex, gotName := app.cfg.InterfaceIndex, app.cfg.InterfaceName
	app.mu.Unlock()
	require.Equal(t, 1, gotIndex, "app.cfg InterfaceIndex after failed switch should be reverted")
	require.Equal(t, "original-nic", gotName, "app.cfg InterfaceName after failed switch should be reverted")
}

// TestSelectInterfaceIsNotReentrant proves a second concurrent
// SelectInterface call, issued while the first is blocked inside its spawn
// double, returns GOLC_WAILS_RELAUNCH_IN_PROGRESS and never overlaps a
// second spawn concurrently.
func TestSelectInterfaceIsNotReentrant(t *testing.T) {
	app := newTestInterfaceSwitchApp(t)

	started := make(chan struct{})
	release := make(chan struct{})
	var spawnCalls int32
	app.dial = func(name string) (net.Conn, error) { return nil, errors.New("unreachable") }
	app.spawn = func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
		if atomic.AddInt32(&spawnCalls, 1) == 1 {
			close(started)
			<-release
		}
		return nil, nil, nil
	}

	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- app.SelectInterface(5, "nic-a")
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the first switch's spawn to start")
	}

	second := app.SelectInterface(6, "nic-b")
	require.NotEqual(t, 0, second.ExitCode, "expected the concurrent call to fail with a non-zero exit code")
	require.Contains(t, second.Stderr, "GOLC_WAILS_RELAUNCH_IN_PROGRESS")

	close(release)
	select {
	case <-resultCh:
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for the first switch to finish")
	}
}

// TestPickShowPathWithoutRuntimeContextFails proves calling either picker on
// an App whose OnStartup has never run (ctx nil) returns
// GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE rather than panicking or reaching
// the real Wails runtime dialog call.
func TestPickShowPathWithoutRuntimeContextFails(t *testing.T) {
	app := NewApp(Config{})

	_, err := app.PickShowPath()
	require.ErrorContains(t, err, "GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE")

	_, err = app.PickNewShowPath()
	require.ErrorContains(t, err, "GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE")
}

// --- 09-07-PLAN.md Task 1: PickFixtureFile (hand-authored YAML add) ---

// TestPickFixtureFileWithoutRuntimeContextFails proves calling the
// custom-fixture file picker on an App whose OnStartup has never run (ctx
// nil) returns GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE rather than
// panicking or reaching the real Wails runtime dialog call -- mirrors
// TestPickShowPathWithoutRuntimeContextFails' identical guard.
func TestPickFixtureFileWithoutRuntimeContextFails(t *testing.T) {
	app := NewApp(Config{})

	_, err := app.PickFixtureFile()
	require.ErrorContains(t, err, "GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE")
}

// --- OpenExternalURL (Settings' About panel open-source credit links) ---

// TestOpenExternalURLRejectsNonHTTPScheme proves a non-http(s) URL is
// rejected before ever reaching runtimeContext -- the same call rejects it
// with no OnStartup/ctx required.
func TestOpenExternalURLRejectsNonHTTPScheme(t *testing.T) {
	app := NewApp(Config{})

	err := app.OpenExternalURL("file:///etc/passwd")
	require.ErrorContains(t, err, "GOLC_WAILS_OPEN_URL_REJECTED")
}

// TestOpenExternalURLWithoutRuntimeContextFails proves calling
// OpenExternalURL on an App whose OnStartup has never run (ctx nil) returns
// GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE rather than panicking or reaching
// the real Wails runtime call -- mirrors
// TestPickShowPathWithoutRuntimeContextFails' identical guard.
func TestOpenExternalURLWithoutRuntimeContextFails(t *testing.T) {
	app := NewApp(Config{})

	err := app.OpenExternalURL("https://pkg.go.dev/github.com/wailsapp/wails/v2")
	require.ErrorContains(t, err, "GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE")
}
