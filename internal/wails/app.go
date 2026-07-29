// app.go implements 06-04-PLAN.md Task 1's Go host lifecycle: App.OnStartup
// checks daemon reachability via ipc.Dial(ipc.PipeName) (internal/artnet/
// ipc/client.go, unmodified usage) and, on GOLC_ARTNET_DAEMON_UNREACHABLE,
// spawns "golc-project.exe artnet serve ..." as a supervised child process
// (os/exec, capturing its lifetime, terminated on OnShutdown) before
// retrying Dial -- mirroring the WIN-02 "supervises every required
// runtime component" pattern already used for the TypeScript/Deno sidecar
// (06-RESEARCH.md Open Question 1). Subsystems start in order (daemon
// reachability -> safety-cluster hotkeys -> throttled event pusher) and
// stop in the reverse order on OnShutdown, mirroring internal/artnet/
// daemon.go's own Run() ordered start/reverse-ordered stop discipline
// (06-RESEARCH.md Pattern 2/Analog 2).
//
// A daemon that never becomes reachable is a degraded-but-non-fatal
// condition: OnStartup still registers the three OS-level safety-cluster
// hotkeys regardless (PLAY-09 requires them independent of daemon-spawn
// success), and DaemonUnreachable()/HotkeyFailures() expose both
// conditions for the frontend to render (never a silent failure --
// 06-RESEARCH.md Security Domain DoS mitigation).
package wails

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/lnorton89/golc/internal/command"
)

// DesktopShowPathEnvName is the single "GOLC_DESKTOP_SHOW" literal both
// cmd/golc-desktop/main.go's startup read and RelaunchWithShow's spawn (below)
// use -- the literal exists in exactly this one place across the whole
// repository (09-02-PLAN.md).
const DesktopShowPathEnvName = "GOLC_DESKTOP_SHOW"

// defaultCliBinaryInstallRoot mirrors config/commands.toml's "cli_binary"
// pin (".tools/installs/golc_project"): the single authority for where the
// bootstrapped golc-project executable lives, resolved relative to
// Config.ProjectRoot via bootstrap.PlatformExecutablePath (which inserts
// the platform key and bin/ segment, e.g. ".tools/installs/golc_project/
// windows-amd64/bin/golc-project.exe") when Config.DaemonExecutable is left
// unset. This is a value copy, not a live config read, because
// internal/projectconfig's strict single-authority decoder is a much
// larger dependency than this scaffold needs; a later plan may thread a
// real projectconfig read through Config instead.
const defaultCliBinaryInstallRoot = ".tools/installs/golc_project"

// defaultDialRetries/defaultDialRetryDelay bound how long OnStartup waits
// for a just-spawned daemon to become reachable before giving up and
// marking the connection degraded (never hanging OnStartup indefinitely).
const (
	defaultDialRetries    = 10
	defaultDialRetryDelay = 200 * time.Millisecond
)

// Result is the Wails-bound response shape every feature service method in
// svc_safety.go/svc_playback.go/svc_surface.go/svc_midi.go returns.
// ExitCode/Stdout/Stderr mirror internal/command.Result's shape (0
// success, 1 command failure, 2 routing/usage/startup failure) so a
// Wails-bound call and a CLI invocation of the exact same underlying route
// render identically; Stdout/Stderr are plain strings (not []byte) so
// Wails' TypeScript binding generator produces a simple frontend type
// rather than a base64-encoded byte array.
type Result struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Config configures one App instance. PipeName overrides the daemon IPC
// pipe (empty uses ipc.PipeName in production; tests pass a distinct
// per-test path so they never collide with a real running daemon).
// ShowPath/InterfaceIndex/InterfaceName/FixturesDir are the exact "artnet
// serve" arguments (internal/command/artnet.go's runArtnetServe shape)
// OnStartup passes when it spawns a supervised daemon child.
// DaemonExecutable overrides the resolved golc-project(.exe) path (empty
// resolves defaultCliBinaryInstallRoot relative to ProjectRoot).
type Config struct {
	PipeName         string
	ShowPath         string
	InterfaceIndex   int
	InterfaceName    string
	FixturesDir      string
	DaemonExecutable string
	ProjectRoot      string
	DialRetries      int
	DialRetryDelay   time.Duration
}

// pipeName returns cfg.PipeName, or ipc.PipeName when unset.
func (cfg Config) pipeName() string {
	if cfg.PipeName != "" {
		return cfg.PipeName
	}
	return ipc.PipeName
}

// dialFunc mirrors ipc.Dial's exact signature so App.dial can be swapped
// for a test double without ever touching a real named pipe.
type dialFunc func(pipeName string) (net.Conn, error)

// spawnFunc launches the supervised daemon child process; App.spawn is
// swapped for a test double so OnStartup's spawn-on-unreachable path never
// actually launches a real golc-project.exe during tests. The returned
// *daemonStderrBuffer (nil in every test double) captures the child's
// stderr as it runs, so ensureDaemon can surface *why* a spawned daemon
// never became reachable instead of only the generic dial-timeout
// message -- a daemon that exits immediately (a bad --show, a
// config-resolution failure, an engine construction failure) previously
// failed completely silently, since nothing read the child's stderr at
// all.
type spawnFunc func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error)

// relaunchSpawnFunc launches the replacement golc-desktop process bound to a
// new show path (RelaunchWithShow, below); App.relaunchSpawn is swapped for a
// test double so relaunch tests never actually launch a second real
// golc-desktop.exe.
type relaunchSpawnFunc func(exePath string, env []string) error

// quitFunc mirrors wailsruntime.Quit's signature so App.quit can be swapped
// for a test double that never touches the real Wails runtime.
type quitFunc func(ctx context.Context)

// maxDaemonStderrBytes bounds daemonStderrBuffer's retained size: the
// supervised daemon child is long-lived (it runs for the app's whole
// session, not just the spawn-retry window), so capturing its stderr must
// never grow unbounded -- only the most recent maxDaemonStderrBytes are
// kept, oldest bytes dropped first.
const maxDaemonStderrBytes = 4096

// daemonStderrBuffer is a concurrency-safe io.Writer that retains at most
// maxDaemonStderrBytes of the most recently written data. Go's os/exec
// package writes to this Writer from its own internal copy goroutine for
// as long as the child runs, while ensureDaemon reads String() from a
// separate goroutine after the dial-retry loop ends -- a plain
// bytes.Buffer is not safe for that concurrent access, so every access
// here is mutex-guarded.
type daemonStderrBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *daemonStderrBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if excess := b.buf.Len() - maxDaemonStderrBytes; excess > 0 {
		b.buf.Next(excess)
	}
	return len(p), nil
}

func (b *daemonStderrBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// App is the Wails-bound host struct (cmd/golc-desktop/main.go's
// options.App{OnStartup: app.OnStartup, OnShutdown: app.OnShutdown}).
//
// Accepted limitation (09-02-PLAN.md T-09-02-04): when the supervised
// artnet daemon was started outside this process (this App never spawned
// it), stopSupervisedDaemon is a no-op and a RelaunchWithShow-spawned
// replacement may attach to that externally managed daemon still bound to
// the previous show -- an operator-managed daemon is outside this
// process's supervision.
type App struct {
	cfg   Config
	dial  dialFunc
	spawn spawnFunc

	relaunchSpawn relaunchSpawnFunc
	quit          quitFunc

	hotkeys *HotkeyManager
	events  *EventPusher

	mu                sync.Mutex
	ctx               context.Context
	daemonCmd         *exec.Cmd
	daemonStderr      *daemonStderrBuffer
	daemonSpawned     bool
	daemonUnreachable bool
	hotkeyFailures    []HotkeyFailure
	relaunching       bool
}

// NewApp constructs an App from cfg, filling DialRetries/DialRetryDelay
// defaults when unset and wiring the production ipc.Dial/defaultSpawn/
// defaultRelaunchSpawn/defaultQuit implementations (tests override
// App.dial/App.spawn/App.relaunchSpawn/App.quit directly -- this package's
// own test files, never an exported setter).
func NewApp(cfg Config) *App {
	if cfg.DialRetries <= 0 {
		cfg.DialRetries = defaultDialRetries
	}
	if cfg.DialRetryDelay <= 0 {
		cfg.DialRetryDelay = defaultDialRetryDelay
	}
	return &App{
		cfg:           cfg,
		dial:          ipc.Dial,
		spawn:         defaultSpawn,
		relaunchSpawn: defaultRelaunchSpawn,
		quit:          defaultQuit,
		hotkeys:       NewHotkeyManager(cfg.pipeName()),
		events:        NewEventPusher(),
	}
}

// OnStartup is Wails' lifecycle hook: the runtime ctx is stored first (under
// the mutex) so PickShowPath/PickNewShowPath/RelaunchWithShow can use it
// after OnStartup returns; ensureDaemon (reachability check + supervised
// spawn-if-absent) runs next, then subsystems start in order -- safety-
// cluster hotkeys, then the throttled event pusher -- mirroring
// internal/artnet/daemon.go Run()'s own ordered-start discipline. Every
// hotkey-registration failure is logged (never silently swallowed --
// Security Domain DoS mitigation) and recorded on the App for the
// frontend to render via HotkeyFailures().
func (a *App) OnStartup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	a.ensureDaemon(ctx)

	failures := a.hotkeys.RegisterAll()
	a.mu.Lock()
	a.hotkeyFailures = failures
	a.mu.Unlock()
	for _, f := range failures {
		log.Printf("GOLC_WAILS_HOTKEY_REGISTER_FAILED: control=%s error=%s", f.Control, f.Error)
	}

	a.events.Start(ctx)
}

// OnShutdown is Wails' lifecycle hook: subsystems stop in the reverse
// order OnStartup started them -- event pusher, then hotkeys, then the
// supervised daemon child (stopSupervisedDaemon) -- mirroring
// internal/artnet/daemon.go Run()'s own reverse-ordered stop discipline.
func (a *App) OnShutdown(ctx context.Context) {
	a.events.Stop()
	a.hotkeys.UnregisterAll()
	a.stopSupervisedDaemon()
}

// stopSupervisedDaemon terminates the supervised artnet-daemon child process
// this App spawned (if any): it takes the mutex, reads and clears
// daemonCmd/daemonSpawned under the lock, then outside the lock performs
// Kill() then Wait(). Safe to call more than once (OnShutdown calls it on
// every normal exit; RelaunchWithShow, below, also calls it before spawning
// the replacement process so the named pipe is free when the replacement's
// own ensureDaemon dials it -- T-09-02-04).
func (a *App) stopSupervisedDaemon() {
	a.mu.Lock()
	cmd := a.daemonCmd
	spawned := a.daemonSpawned
	a.daemonCmd = nil
	a.daemonSpawned = false
	a.mu.Unlock()
	if spawned && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// ensureDaemon dials the daemon's named pipe; on
// GOLC_ARTNET_DAEMON_UNREACHABLE it spawns a supervised
// "golc-project.exe artnet serve" child process and retries Dial up to
// cfg.DialRetries times (WIN-02 supervised-helper pattern). A daemon that
// never becomes reachable leaves DaemonUnreachable() true rather than
// blocking OnStartup indefinitely -- the safety-cluster hotkeys still
// register regardless (PLAY-09).
func (a *App) ensureDaemon(ctx context.Context) {
	pipeName := a.cfg.pipeName()

	if conn, err := a.dial(pipeName); err == nil {
		_ = conn.Close()
		return
	}

	cmd, stderr, err := a.spawn(ctx, a.cfg)
	if err != nil {
		log.Printf("GOLC_WAILS_DAEMON_SPAWN_FAILED: %v", err)
		a.mu.Lock()
		a.daemonUnreachable = true
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	a.daemonCmd = cmd
	a.daemonStderr = stderr
	a.daemonSpawned = true
	a.mu.Unlock()

	for i := 0; i < a.cfg.DialRetries; i++ {
		time.Sleep(a.cfg.DialRetryDelay)
		if conn, dialErr := a.dial(pipeName); dialErr == nil {
			_ = conn.Close()
			return
		}
	}

	a.mu.Lock()
	a.daemonUnreachable = true
	a.mu.Unlock()
	log.Printf("GOLC_WAILS_DAEMON_UNREACHABLE: daemon spawned but never became reachable on %s after %d retries%s",
		pipeName, a.cfg.DialRetries, daemonStderrDetail(stderr))
}

// daemonStderrDetail returns a ": <trimmed captured stderr>" suffix when
// stderr is non-nil and non-empty, or "" otherwise -- so the unreachable
// diagnostic surfaces exactly why a spawned daemon exited/never listened
// (e.g. GOLC_ARTNET_SERVE_FAILED: ...) instead of leaving the operator
// with only a generic timeout and no other data to act on.
func daemonStderrDetail(stderr *daemonStderrBuffer) string {
	if stderr == nil {
		return ""
	}
	trimmed := strings.TrimSpace(stderr.String())
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}

// DaemonUnreachable reports whether the most recent OnStartup ended with
// the daemon still unreachable -- the frontend's daemon-unreachable copy
// (06-UI-SPEC.md error state) reads this rather than inferring it from a
// failed status fetch alone.
func (a *App) DaemonUnreachable() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.daemonUnreachable
}

// HotkeyFailures returns the most recent hotkey-registration outcome (may
// be empty) -- the frontend can render a visible warning per failed
// control rather than the failure staying silent (Security Domain DoS
// mitigation).
func (a *App) HotkeyFailures() []HotkeyFailure {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]HotkeyFailure(nil), a.hotkeyFailures...)
}

// resolveDaemonExecutable returns cfg.DaemonExecutable, or
// defaultCliBinaryInstallRoot resolved relative to cfg.ProjectRoot (via
// bootstrap.PlatformExecutablePath) when unset.
func resolveDaemonExecutable(cfg Config) (string, error) {
	if cfg.DaemonExecutable != "" {
		return cfg.DaemonExecutable, nil
	}
	if cfg.ProjectRoot == "" {
		return "", fmt.Errorf("GOLC_WAILS_DAEMON_EXECUTABLE_UNRESOLVED: no DaemonExecutable and no ProjectRoot to resolve %s against", defaultCliBinaryInstallRoot)
	}
	installRoot := filepath.Join(cfg.ProjectRoot, filepath.FromSlash(defaultCliBinaryInstallRoot))
	resolved := bootstrap.PlatformExecutablePath(installRoot, "golc-project")
	if resolved == "" {
		return "", fmt.Errorf("GOLC_WAILS_DAEMON_EXECUTABLE_UNRESOLVED: could not resolve golc-project executable under %s", installRoot)
	}
	return resolved, nil
}

// defaultSpawn launches golc-project.exe artnet serve as a supervised
// child process (WIN-02 pattern), mirroring internal/command/artnet.go's
// runArtnetServe argument shape exactly so the spawned daemon accepts the
// identical flags a "golc artnet serve" CLI invocation would.
func defaultSpawn(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
	exePath, err := resolveDaemonExecutable(cfg)
	if err != nil {
		return nil, nil, err
	}

	args := []string{
		"artnet", "serve",
		"--show", cfg.ShowPath,
		"--interface", strconv.Itoa(cfg.InterfaceIndex),
	}
	if cfg.InterfaceName != "" {
		args = append(args, "--interface-name", cfg.InterfaceName)
	}
	if cfg.FixturesDir != "" {
		args = append(args, "--fixtures", cfg.FixturesDir)
	}
	if cfg.PipeName != "" {
		args = append(args, "--pipe", cfg.PipeName)
	}

	cmd := exec.CommandContext(ctx, exePath, args...)
	if cfg.ProjectRoot != "" {
		cmd.Env = append(os.Environ(), "GOLC_PROJECT_ROOT="+cfg.ProjectRoot)
	}
	stderr := &daemonStderrBuffer{}
	cmd.Stderr = stderr
	if startErr := cmd.Start(); startErr != nil {
		return nil, nil, fmt.Errorf("GOLC_WAILS_DAEMON_SPAWN_FAILED: %v", startErr)
	}
	return cmd, stderr, nil
}

// runtimeContext returns the ctx stored by OnStartup, or nil (with ok
// false) when OnStartup has never run.
func (a *App) runtimeContext() (context.Context, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx, a.ctx != nil
}

// showPathFileFilter is the single *.golc filter both PickShowPath and
// PickNewShowPath apply -- the on-screen show open/new/switch surface only
// ever picks a GOLC show document (09-02-PLAN.md FDUI-02).
var showPathFileFilter = wailsruntime.FileFilter{DisplayName: "GOLC show (*.golc)", Pattern: "*.golc"}

// PickShowPath opens a native "Open Show" file-picker dialog filtered to
// *.golc, returning the chosen path -- or an empty string and a nil error
// when the operator cancels (a cancelled dialog is never an error). Returns
// GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE when OnStartup has never run (no
// stored ctx) rather than panicking inside the real Wails runtime dialog
// call.
func (a *App) PickShowPath() (string, error) {
	ctx, ok := a.runtimeContext()
	if !ok {
		return "", errors.New("GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE: OnStartup has not run yet")
	}
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:   "Open Show",
		Filters: []wailsruntime.FileFilter{showPathFileFilter},
	})
}

// PickNewShowPath opens a native "New Show" save-file-picker dialog with the
// same *.golc filter and a "show.golc" default filename -- the identical
// mechanism PickShowPath uses, pointed at a destination that does not exist
// yet (D-06: there is no separate new-show setup flow, no second backend
// concept). Cancellation is likewise an empty string and a nil error.
func (a *App) PickNewShowPath() (string, error) {
	ctx, ok := a.runtimeContext()
	if !ok {
		return "", errors.New("GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE: OnStartup has not run yet")
	}
	return wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "New Show",
		DefaultFilename: "show.golc",
		Filters:         []wailsruntime.FileFilter{showPathFileFilter},
	})
}

// RelaunchWithShow implements the supervised self-relaunch (09-02-PLAN.md
// D-05/D-06/D-07): trim and validate newShowPath, save the working show
// through the canonical "show save" route, resolve this same running
// executable, stop supervising the current daemon child so the named pipe
// is free, start a replacement golc-desktop process bound to newShowPath via
// DesktopShowPathEnvName, and -- only once that replacement has actually
// started -- quit this process. Every failure path leaves this process
// running with its original show untouched and the relaunching guard
// cleared (T-09-02-02/T-09-02-05); the success path deliberately leaves the
// guard set, since this process is exiting.
func (a *App) RelaunchWithShow(newShowPath string) Result {
	trimmed := strings.TrimSpace(newShowPath)
	if trimmed == "" {
		return Result{ExitCode: 1, Stderr: "GOLC_WAILS_RELAUNCH_PATH_EMPTY: a show path is required\n"}
	}

	a.mu.Lock()
	if a.relaunching {
		a.mu.Unlock()
		return Result{ExitCode: 1, Stderr: "GOLC_WAILS_RELAUNCH_IN_PROGRESS: a show switch is already in flight\n"}
	}
	a.relaunching = true
	a.mu.Unlock()

	fail := func(stderr string) Result {
		a.mu.Lock()
		a.relaunching = false
		a.mu.Unlock()
		return Result{ExitCode: 1, Stderr: stderr}
	}

	// Step 3: save the working show through the exact same registered
	// "show save" route ShowService.Save uses -- never a second save
	// implementation (svc_show.go's execute helper mirrored here).
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return fail(fmt.Sprintf("GOLC_WAILS_RELAUNCH_SAVE_FAILED: GOLC_WAILS_REGISTRY_BUILD_FAILED: %v\n", err))
	}
	saveResult := registry.Execute(command.Request{Root: a.cfg.ProjectRoot, Args: []string{"show", "save", "--show", a.cfg.ShowPath}})
	if saveResult.ExitCode != 0 {
		return fail("GOLC_WAILS_RELAUNCH_SAVE_FAILED: " + string(saveResult.Stderr))
	}

	// Step 4: resolve this same running executable -- nothing has spawned
	// or stopped anything yet.
	exePath, err := os.Executable()
	if err != nil {
		return fail(fmt.Sprintf("GOLC_WAILS_RELAUNCH_SPAWN_FAILED: resolving the running executable: %v\n", err))
	}

	// Step 5: free the named pipe before the replacement starts, otherwise
	// the replacement's own ensureDaemon dials successfully and silently
	// attaches to a daemon still bound to the OLD show path (T-09-02-04).
	a.stopSupervisedDaemon()

	// Step 6: spawn the replacement, bound to newShowPath via the single
	// DesktopShowPathEnvName literal.
	env := append(os.Environ(), DesktopShowPathEnvName+"="+trimmed)
	if spawnErr := a.relaunchSpawn(exePath, env); spawnErr != nil {
		if ctx, ok := a.runtimeContext(); ok {
			a.ensureDaemon(ctx)
		}
		return fail(fmt.Sprintf("GOLC_WAILS_RELAUNCH_SPAWN_FAILED: %v\n", spawnErr))
	}

	// Step 7: only on spawn success, quit -- the guard deliberately stays
	// set, since this process is exiting.
	if ctx, ok := a.runtimeContext(); ok {
		a.quit(ctx)
	}
	return Result{ExitCode: 0, Stdout: "GOLC_WAILS_RELAUNCH_STARTED: relaunching with a new show path\n"}
}

// fixtureFileFilter is the *.yaml/*.yml filter PickFixtureFile applies --
// the custom-fixture-add path only ever picks a hand-authored YAML fixture
// definition (09-07-PLAN.md D-04), never an arbitrary file.
var fixtureFileFilter = wailsruntime.FileFilter{DisplayName: "GOLC fixture (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"}

// PickFixtureFile opens a native "Add Custom Fixture" file-picker dialog
// filtered to *.yaml/*.yml, returning the chosen path -- or an empty string
// and a nil error when the operator cancels (mirrors PickShowPath's
// identical cancellation contract, 09-07-PLAN.md D-04). Returns
// GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE when OnStartup has never run (no
// stored ctx) rather than panicking inside the real Wails runtime dialog
// call.
func (a *App) PickFixtureFile() (string, error) {
	ctx, ok := a.runtimeContext()
	if !ok {
		return "", errors.New("GOLC_WAILS_RUNTIME_CONTEXT_UNAVAILABLE: OnStartup has not run yet")
	}
	return wailsruntime.OpenFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:   "Add Custom Fixture",
		Filters: []wailsruntime.FileFilter{fixtureFileFilter},
	})
}

// defaultRelaunchSpawn starts exePath (the running golc-desktop.exe itself)
// as a new, independent process with env as its complete environment --
// deliberately exec.Command, not exec.CommandContext, because the stored
// ctx is cancelled by this process's own exit and would kill the
// replacement before it can finish starting. It never invokes a shell, so
// the new show path travels as a plain environment assignment, never a
// shell-interpreted string (T-09-02-01).
func defaultRelaunchSpawn(exePath string, env []string) error {
	cmd := exec.Command(exePath)
	cmd.Env = env
	return cmd.Start()
}

// defaultQuit is the production quitFunc: the real Wails runtime quit.
func defaultQuit(ctx context.Context) {
	wailsruntime.Quit(ctx)
}
