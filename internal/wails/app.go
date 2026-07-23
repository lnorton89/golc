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
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/artnet/ipc"
)

// defaultCliBinaryRelPath mirrors config/commands.toml's "cli_binary" pin
// (".tools/installs/golc_project/bin/golc-project.exe"): the single
// authority for where the bootstrapped golc-project executable lives,
// resolved relative to Config.ProjectRoot when Config.DaemonExecutable is
// left unset. This is a value copy, not a live config read, because
// internal/projectconfig's strict single-authority decoder is a much
// larger dependency than this scaffold needs; a later plan may thread a
// real projectconfig read through Config instead.
const defaultCliBinaryRelPath = `.tools\installs\golc_project\bin\golc-project.exe`

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
// resolves defaultCliBinaryRelPath relative to ProjectRoot).
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
// actually launches a real golc-project.exe during tests.
type spawnFunc func(ctx context.Context, cfg Config) (*exec.Cmd, error)

// App is the Wails-bound host struct (cmd/golc-desktop/main.go's
// options.App{OnStartup: app.OnStartup, OnShutdown: app.OnShutdown}).
type App struct {
	cfg   Config
	dial  dialFunc
	spawn spawnFunc

	hotkeys *HotkeyManager
	events  *EventPusher

	mu                sync.Mutex
	daemonCmd         *exec.Cmd
	daemonSpawned     bool
	daemonUnreachable bool
	hotkeyFailures    []HotkeyFailure
}

// NewApp constructs an App from cfg, filling DialRetries/DialRetryDelay
// defaults when unset and wiring the production ipc.Dial/defaultSpawn
// implementations (tests override App.dial/App.spawn directly -- this
// package's own test files, never an exported setter).
func NewApp(cfg Config) *App {
	if cfg.DialRetries <= 0 {
		cfg.DialRetries = defaultDialRetries
	}
	if cfg.DialRetryDelay <= 0 {
		cfg.DialRetryDelay = defaultDialRetryDelay
	}
	return &App{
		cfg:     cfg,
		dial:    ipc.Dial,
		spawn:   defaultSpawn,
		hotkeys: NewHotkeyManager(cfg.pipeName()),
		events:  NewEventPusher(),
	}
}

// OnStartup is Wails' lifecycle hook: ensureDaemon (reachability check +
// supervised spawn-if-absent) runs first, then subsystems start in order
// -- safety-cluster hotkeys, then the throttled event pusher -- mirroring
// internal/artnet/daemon.go Run()'s own ordered-start discipline. Every
// hotkey-registration failure is logged (never silently swallowed --
// Security Domain DoS mitigation) and recorded on the App for the
// frontend to render via HotkeyFailures().
func (a *App) OnStartup(ctx context.Context) {
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
// order OnStartup started them -- event pusher, then hotkeys -- and
// finally the supervised daemon child (if this App spawned one) is
// terminated, mirroring internal/artnet/daemon.go Run()'s own
// reverse-ordered stop discipline.
func (a *App) OnShutdown(ctx context.Context) {
	a.events.Stop()
	a.hotkeys.UnregisterAll()

	a.mu.Lock()
	cmd := a.daemonCmd
	spawned := a.daemonSpawned
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

	cmd, err := a.spawn(ctx, a.cfg)
	if err != nil {
		log.Printf("GOLC_WAILS_DAEMON_SPAWN_FAILED: %v", err)
		a.mu.Lock()
		a.daemonUnreachable = true
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	a.daemonCmd = cmd
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
	log.Printf("GOLC_WAILS_DAEMON_UNREACHABLE: daemon spawned but never became reachable on %s after %d retries", pipeName, a.cfg.DialRetries)
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
// defaultCliBinaryRelPath resolved relative to cfg.ProjectRoot when unset.
func resolveDaemonExecutable(cfg Config) (string, error) {
	if cfg.DaemonExecutable != "" {
		return cfg.DaemonExecutable, nil
	}
	if cfg.ProjectRoot == "" {
		return "", fmt.Errorf("GOLC_WAILS_DAEMON_EXECUTABLE_UNRESOLVED: no DaemonExecutable and no ProjectRoot to resolve %s against", defaultCliBinaryRelPath)
	}
	return filepath.Join(cfg.ProjectRoot, defaultCliBinaryRelPath), nil
}

// defaultSpawn launches golc-project.exe artnet serve as a supervised
// child process (WIN-02 pattern), mirroring internal/command/artnet.go's
// runArtnetServe argument shape exactly so the spawned daemon accepts the
// identical flags a "golc artnet serve" CLI invocation would.
func defaultSpawn(ctx context.Context, cfg Config) (*exec.Cmd, error) {
	exePath, err := resolveDaemonExecutable(cfg)
	if err != nil {
		return nil, err
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
	if startErr := cmd.Start(); startErr != nil {
		return nil, fmt.Errorf("GOLC_WAILS_DAEMON_SPAWN_FAILED: %v", startErr)
	}
	return cmd, nil
}
