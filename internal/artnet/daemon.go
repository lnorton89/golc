// daemon.go implements CONTEXT D-03/D-04's long-lived, standalone-capable
// process (04-04-PLAN.md Task 2): Run constructs and starts the playback
// Engine, the InterfaceManager (pinned interface, D-05), and the Art-Net
// Worker against the Engine's CurrentFrame source (Plans 01-03), then
// serves the IPC listener (Task 1) until ctx is cancelled. The handler
// dispatches inbound command.Request routes to daemon-side operations:
// "artnet status" reads the Health snapshot; "artnet configure" adds or
// updates one fan-out target for a universe (CONTEXT D-08); "artnet target
// enable"/"artnet target disable" toggle one target's output without
// touching the rest of the rig (CONTEXT D-12). The daemon is the single
// owner of this worker/target/interface state (D-03): every golc
// artnet ... CLI invocation (Plan 05) is a short-lived client that dials
// this same running process and Forwards a Request over the pipe -- no
// CLI invocation ever constructs its own Worker or owns a separate output
// process. Config state (the target map) lives in memory only for this
// phase; persistence is Phase 5 scope.
//
// The daemon is standalone-capable (D-04): Run's own import graph reaches
// only playback/show/command/strictjson plus this package and its ipc
// subpackage -- nothing from Wails/UI -- so it runs entirely headless.
// Phase 6's Wails app will later attach as just one more IPC client.
//
// worker.go exposes no dynamic reconfigure API by design (Start dials
// every target once); a configure/enable/disable mutation is therefore
// applied by stopping the current Worker and starting a fresh one built
// from the updated target map, reusing the same *Health instance
// (WorkerConfig.Health) so historical send counts/reachability survive a
// reconfigure rather than resetting to zero.
package artnet

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

// Config configures Run's long-lived daemon process. Targets is the
// initial fan-out target set (CONTEXT D-08); it is mutated in-memory
// afterward via "artnet configure"/"artnet target enable|disable" IPC
// requests. PipeName overrides the IPC listener's pipe path; empty uses
// ipc.PipeName (the production default) -- tests set a distinct value so
// concurrent package test runs never collide on the same named pipe.
type Config struct {
	State          show.State
	InterfaceIndex int
	InterfaceName  string
	Instances      []deployment.Instance
	Resolve        ResolveFunc
	Targets        map[int][]Target
	SendTimeout    time.Duration
	PipeName       string
}

// pipeNameOrDefault returns cfg.PipeName, or ipc.PipeName when cfg.PipeName
// is unset.
func pipeNameOrDefault(cfg Config) string {
	if cfg.PipeName != "" {
		return cfg.PipeName
	}
	return ipc.PipeName
}

// daemon holds every piece of state a golc artnet ... IPC request can read
// or mutate (CONTEXT D-03). mu guards targets/worker so a configure/
// enable/disable request and a concurrent status read never race, even
// though Health's own Snapshot stays lock-free internally.
type daemon struct {
	baseCtx context.Context

	engine   *playback.Engine
	ifaceMgr *InterfaceManager

	resolve   ResolveFunc
	instances []deployment.Instance

	mu          sync.Mutex
	targets     map[int][]Target
	worker      *Worker
	health      *Health
	localIP     net.IP
	sendTimeout time.Duration
}

// copyTargets returns a fresh, independent copy of targets so daemon never
// aliases the caller's own map/slices (mirrors internal/deployment/
// model.go's copy-returning discipline).
func copyTargets(targets map[int][]Target) map[int][]Target {
	out := make(map[int][]Target, len(targets))
	for universe, ts := range targets {
		out[universe] = append([]Target(nil), ts...)
	}
	return out
}

// startWorkerLocked builds a fresh Worker from d's current target map and
// starts it. Callers must hold d.mu.
func (d *daemon) startWorkerLocked() {
	d.worker = NewWorker(WorkerConfig{
		Frames:      d.engine,
		Instances:   d.instances,
		Resolve:     d.resolve,
		Targets:     d.targets,
		LocalIP:     d.localIP,
		SendTimeout: d.sendTimeout,
		Health:      d.health,
	})
	d.worker.Start(d.baseCtx)
}

// stopWorkerLocked stops d's current Worker, if any. Callers must hold
// d.mu.
func (d *daemon) stopWorkerLocked() {
	if d.worker != nil {
		d.worker.Stop()
		d.worker = nil
	}
}

// reconfigureLocked stops the current Worker and starts a fresh one built
// from d's just-mutated target map -- worker.go has no dynamic reconfigure
// API by design, so a full stop/rebuild/start is how the daemon applies a
// configure/enable/disable mutation while remaining the single owner of
// this state (CONTEXT D-03). Callers must hold d.mu.
func (d *daemon) reconfigureLocked() {
	d.stopWorkerLocked()
	d.startWorkerLocked()
}

// Run is the long-lived, standalone-capable daemon entrypoint (CONTEXT
// D-03/D-04): it constructs and starts the playback Engine, the pinned
// InterfaceManager, and the Art-Net Worker, then serves the IPC listener
// until ctx is cancelled. On cancellation, ipc.Serve's own ctx.Done
// handling closes the IPC listener first (unblocking Accept), after which
// Run stops the worker, the interface-loss poll loop, and finally the
// engine, in that order -- a clean shutdown with no goroutine leak. Run
// blocks until ctx is cancelled or the IPC listener fails to start/serve.
func Run(ctx context.Context, cfg Config) error {
	engine, err := playback.NewEngine(cfg.State)
	if err != nil {
		return fmt.Errorf("GOLC_ARTNET_DAEMON_ENGINE_FAILED: %v", err)
	}
	engine.Start(ctx)

	ifaceMgr := NewInterfaceManager(cfg.InterfaceIndex, cfg.InterfaceName)
	ifaceMgr.Start(ctx)

	// A not-yet-resolvable pinned interface at startup is a degraded
	// health state (D-05), not a fatal daemon error: the worker binds to
	// no specific local address until the interface is reachable, and
	// ifaceMgr.Status()/Err() already surface the loss to any status
	// caller.
	localIP, _ := ifaceMgr.LocalIP()

	d := &daemon{
		baseCtx:     ctx,
		engine:      engine,
		ifaceMgr:    ifaceMgr,
		resolve:     cfg.Resolve,
		instances:   cfg.Instances,
		targets:     copyTargets(cfg.Targets),
		health:      NewHealth(),
		localIP:     localIP,
		sendTimeout: cfg.SendTimeout,
	}

	d.mu.Lock()
	d.startWorkerLocked()
	d.mu.Unlock()

	listener, err := ipc.NewListener(pipeNameOrDefault(cfg))
	if err != nil {
		d.mu.Lock()
		d.stopWorkerLocked()
		d.mu.Unlock()
		ifaceMgr.Stop()
		engine.Stop()
		return fmt.Errorf("GOLC_ARTNET_DAEMON_IPC_LISTEN_FAILED: %v", err)
	}

	serveErr := ipc.Serve(ctx, listener, d.handle)

	d.mu.Lock()
	d.stopWorkerLocked()
	d.mu.Unlock()
	ifaceMgr.Stop()
	engine.Stop()

	return serveErr
}

// handle dispatches one IPC-forwarded command.Request to a daemon-side
// operation (CONTEXT D-03). Every route mutates only this daemon's own
// in-memory state -- no CLI invocation ever owns a separate output
// process.
func (d *daemon) handle(request command.Request) command.Result {
	switch request.Route {
	case "artnet status":
		return d.handleStatus()
	case "artnet configure":
		return d.handleConfigure(request.Args)
	case "artnet target enable":
		return d.handleSetEnabled(request.Args, true)
	case "artnet target disable":
		return d.handleSetEnabled(request.Args, false)
	default:
		return command.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_ROUTE_UNKNOWN: the daemon has no operation for route %q\n", request.Route))}
	}
}

// statusPayload is the JSON-safe wire rendering of a HealthSnapshot for
// "artnet status" (ARTN-05). HealthSnapshot.Targets is a
// map[targetKey]TargetHealth, and targetKey (health.go/target.go, out of
// this plan's file scope) is an unexported struct with no
// encoding.TextMarshaler -- encoding/json can only marshal a map key that
// is a string, an integer kind, or implements TextMarshaler, so encoding a
// HealthSnapshot directly fails with "json: unsupported type" ([Rule 1]
// bug found while wiring Task 2's own required status-encode path; see
// SUMMARY Deviations). Every TargetHealth value already carries its own
// Universe and Target fields (the exact information targetKey packs), so
// flattening to a slice loses nothing; the slice is sorted by (Universe,
// IP, Port) for deterministic, byte-stable output (mirrors this repo's
// own CanonicalEncode determinism convention).
type statusPayload struct {
	Frame   FrameHealth    `json:"frame"`
	Targets []TargetHealth `json:"targets"`
}

// newStatusPayload flattens snapshot's target map into a sorted slice (see
// statusPayload doc comment for why the map itself cannot be JSON-encoded
// directly).
func newStatusPayload(snapshot HealthSnapshot) statusPayload {
	targets := make([]TargetHealth, 0, len(snapshot.Targets))
	for _, th := range snapshot.Targets {
		targets = append(targets, th)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Universe != targets[j].Universe {
			return targets[i].Universe < targets[j].Universe
		}
		iIP, jIP := targets[i].Target.IP.String(), targets[j].Target.IP.String()
		if iIP != jIP {
			return iIP < jIP
		}
		return effectivePort(targets[i].Target) < effectivePort(targets[j].Target)
	})
	return statusPayload{Frame: snapshot.Frame, Targets: targets}
}

// handleStatus answers "artnet status" with the current Health snapshot
// (ARTN-05), canonically encoded (internal/strictjson) into Result.Stdout
// via the JSON-safe statusPayload rendering.
func (d *daemon) handleStatus() command.Result {
	payload, err := strictjson.CanonicalEncode(newStatusPayload(d.health.Snapshot()))
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_STATUS_ENCODE_FAILED: %v\n", err))}
	}
	return command.Result{Stdout: payload}
}

const configureUsage = "artnet configure --universe <n> --ip <address> [--port <port>] [--enabled true|false]"

// handleConfigure answers "artnet configure": it adds a new fan-out target
// for the given universe, or replaces the existing target sharing the same
// (Universe, IP, Port) triple (CONTEXT D-08), then reconfigures the
// running Worker so the change takes effect on the next tick.
func (d *daemon) handleConfigure(args []string) command.Result {
	flags, err := parseFlags(configureUsage, args)
	if err != nil {
		return command.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	universe, ip, port, err := parseTargetSelector(configureUsage, flags)
	if err != nil {
		return command.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	enabled := true
	if raw, ok := flags["enabled"]; ok {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return command.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_ARTNET_USAGE: --enabled value %q is not a valid bool; usage: %s\n", raw, configureUsage))}
		}
		enabled = parsed
	}

	target := Target{Universe: universe, IP: ip, Port: port, Enabled: enabled}
	if err := ValidateTarget(target); err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	existing := d.targets[universe]
	updated := make([]Target, 0, len(existing)+1)
	replaced := false
	for _, t := range existing {
		if keyOf(t) == keyOf(target) {
			updated = append(updated, target)
			replaced = true
			continue
		}
		updated = append(updated, t)
	}
	if !replaced {
		updated = append(updated, target)
	}
	if err := ValidateUniqueTargets(updated); err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	d.targets[universe] = updated
	d.reconfigureLocked()

	return command.Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_ARTNET_CONFIGURE: universe %d target %s:%d configured (enabled=%v)\n", universe, ip, port, enabled))}
}

// handleSetEnabled answers "artnet target enable"/"artnet target disable"
// (CONTEXT D-12): it toggles the single target matching the given
// selector's Enabled flag via SetEnabled (copy-returning) and reconfigures
// the running Worker, leaving every other target in the rig untouched.
func (d *daemon) handleSetEnabled(args []string, enabled bool) command.Result {
	usage := "artnet target enable|disable --universe <n> --ip <address> [--port <port>]"
	flags, err := parseFlags(usage, args)
	if err != nil {
		return command.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	universe, ip, port, err := parseTargetSelector(usage, flags)
	if err != nil {
		return command.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	match := Target{Universe: universe, IP: ip, Port: port}

	d.mu.Lock()
	defer d.mu.Unlock()

	updated, err := SetEnabled(d.targets[universe], match, enabled)
	if err != nil {
		return command.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	d.targets[universe] = updated
	d.reconfigureLocked()

	return command.Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_ARTNET_TARGET_SET_ENABLED: universe %d target %s:%d enabled=%v\n", universe, ip, port, enabled))}
}

// parseFlags parses args as a sequence of "--flag value" or "--flag=value"
// pairs into a map, rejecting anything else as GOLC_ARTNET_USAGE (mirrors
// internal/command/playback.go's two-tier --flag value/--flag=value
// convention, reused here so a future golc artnet ... CLI client (Plan 05)
// can forward its own already-validated flags through unchanged).
func parseFlags(usage string, args []string) (map[string]string, error) {
	flags := map[string]string{}
	for i := 0; i < len(args); {
		argument := args[i]
		if !strings.HasPrefix(argument, "--") {
			return nil, fmt.Errorf("GOLC_ARTNET_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			flags[argument[2:eq]] = argument[eq+1:]
			i++
			continue
		}
		name := strings.TrimPrefix(argument, "--")
		if i+1 >= len(args) {
			return nil, fmt.Errorf("GOLC_ARTNET_USAGE: --%s requires a value; usage: %s", name, usage)
		}
		flags[name] = args[i+1]
		i += 2
	}
	return flags, nil
}

// parseTargetSelector reads the required --universe/--ip flags (and the
// optional --port flag) out of flags, rejecting a missing/malformed value
// as GOLC_ARTNET_USAGE.
func parseTargetSelector(usage string, flags map[string]string) (universe int, ip net.IP, port int, err error) {
	rawUniverse, ok := flags["universe"]
	if !ok {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --universe is required; usage: %s", usage)
	}
	universe, convErr := strconv.Atoi(rawUniverse)
	if convErr != nil {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --universe value %q is not a valid integer; usage: %s", rawUniverse, usage)
	}

	rawIP, ok := flags["ip"]
	if !ok {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --ip is required; usage: %s", usage)
	}
	parsedIP := net.ParseIP(rawIP)
	if parsedIP == nil {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --ip value %q is not a valid IP address; usage: %s", rawIP, usage)
	}

	if rawPort, ok := flags["port"]; ok {
		parsedPort, convErr := strconv.Atoi(rawPort)
		if convErr != nil {
			return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --port value %q is not a valid integer; usage: %s", rawPort, usage)
		}
		port = parsedPort
	}

	return universe, parsedIP, port, nil
}
