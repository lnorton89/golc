// daemon.go implements CONTEXT D-03/D-04's long-lived, standalone-capable
// process (04-04-PLAN.md Task 2): Run constructs and starts the playback
// Engine, the InterfaceManager (pinned interface, D-05), and the Art-Net
// Worker against the Engine's CurrentFrame source (Plans 01-03), then
// serves the IPC listener (Task 1) until ctx is cancelled. The handler
// dispatches inbound ipc.Request routes to daemon-side operations:
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
// only playback/show/strictjson plus this package and its ipc subpackage
// -- nothing from Wails/UI, and (as of 04-05-PLAN.md's import-cycle fix)
// not internal/command either -- so it runs entirely headless and can be
// imported by internal/command/artnet.go without a cycle. Phase 6's Wails
// app will later attach as just one more IPC client.
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

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/pool"
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
// though Health's own Snapshot stays lock-free internally. safety is the
// one piece of daemon-resident state deliberately NOT guarded by mu
// (06-02-PLAN.md Task 2, PLAY-06/08/09): its own atomic fields are read
// lock-free by the Worker's tick goroutine every ~25ms, so a Blackout/
// Stop-All/Revoke-Automation/master mutation must never contend with mu
// (which a slow configure/status path can hold) or wait for a Worker
// restart -- see handleSafetyToggle/handleMasterSet's own doc comments for
// why they never call reconfigureLocked().
type daemon struct {
	baseCtx context.Context

	engine   *playback.Engine
	ifaceMgr *InterfaceManager

	resolve   ResolveFunc
	instances []deployment.Instance
	groups    []pool.Group

	safety safetyState

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
		Groups:      d.groups,
		Resolve:     d.resolve,
		Targets:     d.targets,
		Safety:      &d.safety,
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
	// no specific local address until the interface is reachable. Its
	// live Status()/Err() is read by handleStatus into the status
	// payload's `interface` field (04-09-PLAN.md), so a caller of
	// "artnet status" can see the degraded state even when this initial
	// LocalIP resolution fails.
	localIP, _ := ifaceMgr.LocalIP()

	d := &daemon{
		baseCtx:     ctx,
		engine:      engine,
		ifaceMgr:    ifaceMgr,
		resolve:     cfg.Resolve,
		instances:   cfg.Instances,
		groups:      cfg.State.Groups,
		targets:     copyTargets(cfg.Targets),
		health:      NewHealth(),
		localIP:     localIP,
		sendTimeout: cfg.SendTimeout,
	}
	// d.safety starts at its identity values (no blackout/stop-all/revoke,
	// grand master 1.0, no group overrides) so a freshly started daemon's
	// very first tick behaves exactly as it did before Task 2 introduced
	// this field.
	d.safety.masters.Store(identityMasterLevels())

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

// requestSource reads the "--source manual|automation" wire-arg convention
// (06-02-PLAN.md Task 2) directly out of args, without going through
// parseFlags' full validation -- every existing route's own args (which
// never declared a --source flag before this task) must keep parsing
// exactly as before even when a caller now appends --source, and a
// request that omits --source entirely must default to "manual" (every
// pre-Task-2 caller, and every operator-issued CLI action, is manual by
// definition -- see internal/command/artnet.go's client routes). Returns
// "manual" for any args value other than exactly "automation".
func requestSource(args []string) string {
	for i, arg := range args {
		if arg == "--source" && i+1 < len(args) {
			if args[i+1] == "automation" {
				return "automation"
			}
			return "manual"
		}
		if strings.HasPrefix(arg, "--source=") {
			if strings.TrimPrefix(arg, "--source=") == "automation" {
				return "automation"
			}
			return "manual"
		}
	}
	return "manual"
}

// handle dispatches one IPC-forwarded ipc.Request to a daemon-side
// operation (CONTEXT D-03). Every route mutates only this daemon's own
// in-memory state -- no CLI invocation ever owns a separate output
// process.
//
// The revoke-automation gate (PLAY-08) runs first, before route dispatch:
// while d.safety.revokeActive() is true, any Request whose args tag it as
// "--source automation" is rejected with GOLC_ARTNET_SAFETY_REVOKED,
// regardless of route -- this blocks a script/AI-issued command (queued or
// new) from reaching any handler without depending on that automation
// runtime ever acknowledging or responding (CONTEXT: "Revoke Automation
// must not depend on an AI provider, script runtime, or queued application
// command completing"). A manual (operator/CLI-issued, the default when
// --source is absent) Request is never blocked by this gate.
func (d *daemon) handle(request ipc.Request) ipc.Result {
	if d.safety.revokeActive() && requestSource(request.Args) == "automation" {
		return ipc.Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_SAFETY_REVOKED: automation is revoked; route %q rejected for a non-manual source\n", request.Route))}
	}

	switch request.Route {
	case "artnet status":
		return d.handleStatus()
	case "artnet configure":
		return d.handleConfigure(request.Args)
	case "artnet target enable":
		return d.handleSetEnabled(request.Args, true)
	case "artnet target disable":
		return d.handleSetEnabled(request.Args, false)
	case "artnet safety blackout":
		return d.handleSafetyToggle(request.Args, d.safety.setBlackout, "GOLC_ARTNET_SAFETY_BLACKOUT")
	case "artnet safety stop-all":
		return d.handleSafetyToggle(request.Args, d.safety.setStopAll, "GOLC_ARTNET_SAFETY_STOP_ALL")
	case "artnet safety revoke-automation":
		return d.handleSafetyToggle(request.Args, d.safety.setRevokeAutomation, "GOLC_ARTNET_SAFETY_REVOKE_AUTOMATION")
	case "artnet master set":
		return d.handleMasterSet(request.Args)
	default:
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
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
// own CanonicalEncode determinism convention). Universes carries each
// configured universe's final 512-byte DMX buffer (04-08-PLAN.md,
// HealthSnapshot.UniverseValues flattened the same way, sorted ascending
// by Universe), closing the gap where the worker's per-tick computed
// buffer was previously discarded after building the outbound packet.
// Interface carries the daemon's own InterfaceManager's live pinned-
// interface status (04-09-PLAN.md, ARTN-01/D-05) so a lost/degraded
// pinned interface is visible to any status caller, never a silent
// switch. Playback carries the PLAY-07 live status projection (06-05-
// PLAN.md Task 1): active scene, enabled layers, BPM/bar position,
// controlling source, and final output state, read from the daemon's own
// playback.Engine and safetyState -- the exact data source the Wails Go
// host's SafetyService.FetchStatus (internal/wails/svc_safety.go) and the
// throttled status:update push (internal/wails/events.go) both project to
// the frontend's LiveStatusBar.
type statusPayload struct {
	Frame     FrameHealth            `json:"frame"`
	Targets   []TargetHealth         `json:"targets"`
	Universes []universeValues       `json:"universes"`
	Interface interfaceStatusPayload `json:"interface"`
	Playback  playbackStatusPayload  `json:"playback"`
}

// playbackStatusPayload is the JSON-safe wire rendering of PLAY-07's live
// status fields (06-05-PLAN.md Task 1). Active is false only when the
// daemon's playback.Engine has no current plan -- a defensive edge this
// package's own Compile requirement makes unreachable through normal
// operation (every successfully constructed Engine always has an active
// scene), but newPlaybackStatusPayload still handles a nil plan
// explicitly (never a blank/zero-valued Active:true payload) so a future
// caller building a daemon around a differently-constructed Engine, or a
// unit test exercising this transform directly, observes an explicit
// idle projection rather than undefined-looking zero values (PLAY-07 idle
// edge, D-04 "visible not hidden"). SceneID/SceneName use `omitempty` so
// the idle case omits them entirely rather than serializing an empty
// string that could be mistaken for "found, but nameless."
// ControllingSource is one of "live" (default: manual, nothing overriding
// it), "revoked" (Revoke Automation active), or "blackout" (Blackout or
// Stop/Release-All active -- blackout takes priority over revoked, since
// blacked-out output is the more severe/visible state).
// OutputState is one of "frame-lock" (on-cadence), "stalled" (frame
// health past frameStaleAfter), or "blackout" (Blackout or Stop/
// Release-All active -- takes priority over a stalled read, since a
// commanded blackout is never confused with an unintended stall).
// EnabledLayers is never nil (an idle/no-plan payload still serializes
// "enabledLayers":[] rather than "enabledLayers":null, matching the "not
// blank/undefined" contract this whole payload exists to satisfy).
type playbackStatusPayload struct {
	Active            bool     `json:"active"`
	SceneID           string   `json:"sceneId,omitempty"`
	SceneName         string   `json:"sceneName,omitempty"`
	BPM               float64  `json:"bpm"`
	BarIndex          int      `json:"barIndex"`
	BeatFraction      float64  `json:"beatFraction"`
	EnabledLayers     []string `json:"enabledLayers"`
	ControllingSource string   `json:"controllingSource"`
	OutputState       string   `json:"outputState"`
}

// layerKindOrder fixes the deterministic ordering enabledLayerNames sorts
// its output into -- the same four layer kinds scene.LayerKind declares
// (scene.BaseLook/ColorTheme/Chase/Motion), spelled out here as plain
// strings so this file never needs to import internal/scene solely for
// this ordering constant.
var layerKindOrder = []string{"base_look", "color_theme", "chase", "motion"}

// enabledLayerNames returns plan's enabled layer Kinds as plain strings,
// sorted by layerKindOrder for deterministic, byte-stable output (mirrors
// this repo's own CanonicalEncode determinism convention). A nil plan or a
// plan with no enabled layers returns an empty (never nil) slice, so the
// wire payload always serializes "enabledLayers":[] rather than null.
func enabledLayerNames(plan *playback.CompiledPlan) []string {
	names := make([]string, 0, len(layerKindOrder))
	if plan == nil {
		return names
	}
	enabled := make(map[string]bool, len(plan.Layers))
	for kind, layer := range plan.Layers {
		if layer.Enabled {
			enabled[string(kind)] = true
		}
	}
	for _, kind := range layerKindOrder {
		if enabled[kind] {
			names = append(names, kind)
		}
	}
	return names
}

// playbackEngineSnapshot isolates the exact playback.Engine values
// newPlaybackStatusPayload needs behind a small struct so the pure
// transform -- and its unit tests -- never need a real *playback.Engine
// (a zero-value playbackEngineSnapshot{} directly exercises the "no
// active plan" idle path).
type playbackEngineSnapshot struct {
	plan      *playback.CompiledPlan
	position  playback.MusicalPosition
	sceneName string
	sceneOK   bool
}

// snapshotEngine extracts engine's current plan/position/scene-name into
// a playbackEngineSnapshot via engine's own lock-free accessors
// (CurrentPlan/CurrentPosition/ActiveSceneName, 06-05-PLAN.md Task 1). A
// nil engine (defensive) returns the zero playbackEngineSnapshot, which
// newPlaybackStatusPayload treats identically to "no active plan."
func snapshotEngine(engine *playback.Engine) playbackEngineSnapshot {
	if engine == nil {
		return playbackEngineSnapshot{}
	}
	name, ok := engine.ActiveSceneName()
	return playbackEngineSnapshot{
		plan:      engine.CurrentPlan(),
		position:  engine.CurrentPosition(),
		sceneName: name,
		sceneOK:   ok,
	}
}

// newPlaybackStatusPayload is the pure transform from a
// playbackEngineSnapshot + the daemon's own safetyState + the current
// frame health into the JSON-safe playbackStatusPayload (see that type's
// own doc comment for the exact ControllingSource/OutputState vocabulary
// and priority rules). A nil safety is treated as "no override active"
// (identity behavior), matching safetyState's own nil-receiver defaults
// elsewhere in this file (e.g. revokeActive()).
func newPlaybackStatusPayload(snap playbackEngineSnapshot, safety *safetyState, frame FrameHealth) playbackStatusPayload {
	blackoutActive := false
	revokeActive := false
	if safety != nil {
		blackoutActive = safety.blackout.Load() || safety.stopAll.Load()
		revokeActive = safety.revokeActive()
	}

	controllingSource := "live"
	switch {
	case blackoutActive:
		controllingSource = "blackout"
	case revokeActive:
		controllingSource = "revoked"
	}

	outputState := "frame-lock"
	switch {
	case blackoutActive:
		outputState = "blackout"
	case !frame.OnCadence:
		outputState = "stalled"
	}

	if snap.plan == nil {
		return playbackStatusPayload{
			Active:            false,
			EnabledLayers:     enabledLayerNames(nil),
			ControllingSource: controllingSource,
			OutputState:       outputState,
		}
	}

	sceneName := ""
	if snap.sceneOK {
		sceneName = snap.sceneName
	}

	return playbackStatusPayload{
		Active:            true,
		SceneID:           snap.plan.SceneID.String(),
		SceneName:         sceneName,
		BPM:               snap.plan.BPM,
		BarIndex:          snap.position.BarIndex,
		BeatFraction:      snap.position.BeatFraction,
		EnabledLayers:     enabledLayerNames(snap.plan),
		ControllingSource: controllingSource,
		OutputState:       outputState,
	}
}

// interfaceStatusPayload is the JSON-safe wire rendering of the daemon's
// pinned InterfaceManager status (04-09-PLAN.md, ARTN-01/D-05):
// PinnedIndex/PinnedName identify the pinned interface (Index is the
// durable identity, Name is display-only -- 04-PATTERNS.md Pitfall 4),
// Status is InterfaceStatus.String() ("ok"/"lost"), and Error is the
// GOLC_ARTNET_INTERFACE_LOST diagnostic string when lost, else empty.
type interfaceStatusPayload struct {
	PinnedIndex int    `json:"pinnedIndex"`
	PinnedName  string `json:"pinnedName"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}

// universeValues is the JSON-safe wire rendering of one configured
// universe's final per-tick DMX buffer (04-08-PLAN.md, ARTN-05). []byte
// marshals as a base64 string under encoding/json (which strictjson
// wraps), which is the intended wire form.
type universeValues struct {
	Universe int    `json:"universe"`
	Values   []byte `json:"values"`
}

// newStatusPayload flattens snapshot's target map into a sorted slice (see
// statusPayload doc comment for why the map itself cannot be JSON-encoded
// directly), and flattens snapshot.UniverseValues into a universe-sorted
// slice the same way. iface is the daemon's own InterfaceManager status
// (04-09-PLAN.md), read separately from the HealthSnapshot since interface
// health is not tracked by Health. pb is the PLAY-07 live status
// projection (06-05-PLAN.md Task 1), already computed by the caller via
// newPlaybackStatusPayload.
func newStatusPayload(snapshot HealthSnapshot, iface interfaceStatusPayload, pb playbackStatusPayload) statusPayload {
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

	universes := make([]universeValues, 0, len(snapshot.UniverseValues))
	for universe, values := range snapshot.UniverseValues {
		universes = append(universes, universeValues{Universe: universe, Values: values})
	}
	sort.Slice(universes, func(i, j int) bool {
		return universes[i].Universe < universes[j].Universe
	})

	return statusPayload{Frame: snapshot.Frame, Targets: targets, Universes: universes, Interface: iface, Playback: pb}
}

// handleStatus answers "artnet status" with the current Health snapshot
// (ARTN-05) plus the daemon's own InterfaceManager's live pinned-interface
// status (04-09-PLAN.md, ARTN-01/D-05), canonically encoded
// (internal/strictjson) into Result.Stdout via the JSON-safe statusPayload
// rendering.
func (d *daemon) handleStatus() ipc.Result {
	// Read Status() exactly once and derive Error from that single
	// observed value (GC-WR-01): calling d.ifaceMgr.Err() and
	// d.ifaceMgr.Status() as two separate loads of the same underlying
	// atomic.Int32 could observe an OK->Lost transition in between,
	// producing Status: "lost" with Error: "" -- contradicting this
	// payload's own doc comment. Safe because the transition is
	// one-directional/terminal (interfacemgr.go's markLost never
	// reverts), so re-deriving Err() from an already-Lost status can
	// never race back to OK. Mirrors evaluateFrameHealth's single-read
	// discipline in health.go.
	status := d.ifaceMgr.Status()
	ifaceErr := ""
	if status == InterfaceStatusLost {
		if err := d.ifaceMgr.Err(); err != nil {
			ifaceErr = err.Error()
		}
	}
	iface := interfaceStatusPayload{
		PinnedIndex: d.ifaceMgr.PinnedIndex(),
		PinnedName:  d.ifaceMgr.PinnedName(),
		Status:      status.String(),
		Error:       ifaceErr,
	}

	snapshot := d.health.Snapshot()
	pb := newPlaybackStatusPayload(snapshotEngine(d.engine), &d.safety, snapshot.Frame)

	payload, err := strictjson.CanonicalEncode(newStatusPayload(snapshot, iface, pb))
	if err != nil {
		return ipc.Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_STATUS_ENCODE_FAILED: %v\n", err))}
	}
	return ipc.Result{Stdout: payload}
}

const configureUsage = "artnet configure --universe <n> --ip <address> [--port <port>] [--enabled true|false]"

// handleConfigure answers "artnet configure": it adds a new fan-out target
// for the given universe, or replaces the existing target sharing the same
// (Universe, IP, Port) triple (CONTEXT D-08), then reconfigures the
// running Worker so the change takes effect on the next tick.
func (d *daemon) handleConfigure(args []string) ipc.Result {
	flags, err := parseFlags(configureUsage, args)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	universe, ip, port, err := parseTargetSelector(configureUsage, flags)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	enabled := true
	if raw, ok := flags["enabled"]; ok {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_ARTNET_USAGE: --enabled value %q is not a valid bool; usage: %s\n", raw, configureUsage))}
		}
		enabled = parsed
	}

	target := Target{Universe: universe, IP: ip, Port: port, Enabled: enabled}
	if err := ValidateTarget(target); err != nil {
		return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
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
		return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	d.targets[universe] = updated
	d.reconfigureLocked()

	return ipc.Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_ARTNET_CONFIGURE: universe %d target %s:%d configured (enabled=%v)\n", universe, ip, port, enabled))}
}

// handleSetEnabled answers "artnet target enable"/"artnet target disable"
// (CONTEXT D-12): it toggles the single target matching the given
// selector's Enabled flag via SetEnabled (copy-returning) and reconfigures
// the running Worker, leaving every other target in the rig untouched.
func (d *daemon) handleSetEnabled(args []string, enabled bool) ipc.Result {
	usage := "artnet target enable|disable --universe <n> --ip <address> [--port <port>]"
	flags, err := parseFlags(usage, args)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	universe, ip, port, err := parseTargetSelector(usage, flags)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	match := Target{Universe: universe, IP: ip, Port: port}

	d.mu.Lock()
	defer d.mu.Unlock()

	updated, err := SetEnabled(d.targets[universe], match, enabled)
	if err != nil {
		return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	d.targets[universe] = updated
	d.reconfigureLocked()

	return ipc.Result{Stdout: []byte(fmt.Sprintf(
		"GOLC_ARTNET_TARGET_SET_ENABLED: universe %d target %s:%d enabled=%v\n", universe, ip, port, enabled))}
}

const safetyToggleUsage = "artnet safety blackout|stop-all|revoke-automation [--on true|false] [--source manual|automation]"

// handleSafetyToggle answers "artnet safety blackout"/"artnet safety
// stop-all"/"artnet safety revoke-automation" (06-02-PLAN.md Task 2,
// PLAY-06/08/09): it parses the optional "--on true|false" flag
// (defaulting to true -- "artnet safety blackout" alone means "turn it
// on"), then calls setter directly against d.safety's own atomic field and
// returns immediately.
//
// This handler deliberately never touches d.mu and never calls
// d.reconfigureLocked() -- unlike handleConfigure/handleSetEnabled above,
// which mutate the target map and therefore must stop/rebuild/start the
// Worker (worker.go has no dynamic reconfigure API by design), a safety
// flag is an atomic the Worker's tick goroutine already reads lock-free
// every tick (daemon.go's own doc comment on the daemon struct's safety
// field). Routing this through reconfigureLocked would reintroduce exactly
// the Worker-restart latency PLAY-06/09 require this path to avoid.
func (d *daemon) handleSafetyToggle(args []string, setter func(bool), label string) ipc.Result {
	flags, err := parseFlags(safetyToggleUsage, args)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	on := true
	if raw, ok := flags["on"]; ok {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_ARTNET_USAGE: --on value %q is not a valid bool; usage: %s\n", raw, safetyToggleUsage))}
		}
		on = parsed
	}

	setter(on)

	return ipc.Result{Stdout: []byte(fmt.Sprintf("%s: on=%v\n", label, on))}
}

const masterSetUsage = "artnet master set --grand <0..1> | --group <id> --level <0..1> [--source manual|automation]"

// handleMasterSet answers "artnet master set" (06-02-PLAN.md Task 2,
// PLAY-06): exactly one of --grand or --group+--level selects which
// master this call replaces. Like handleSafetyToggle, this never touches
// d.mu or reconfigureLocked -- d.safety.setGrandMaster/setGroupMaster
// atomically swap an immutable masterLevels snapshot (safety.go) the
// Worker's tick goroutine reads lock-free, taking effect on the very next
// tick with no Worker restart.
func (d *daemon) handleMasterSet(args []string) ipc.Result {
	flags, err := parseFlags(masterSetUsage, args)
	if err != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	if rawGrand, ok := flags["grand"]; ok {
		level, parseErr := strconv.ParseFloat(rawGrand, 64)
		if parseErr != nil {
			return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_ARTNET_USAGE: --grand value %q is not a valid number; usage: %s\n", rawGrand, masterSetUsage))}
		}
		if err := d.safety.setGrandMaster(level); err != nil {
			return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		return ipc.Result{Stdout: []byte(fmt.Sprintf("GOLC_ARTNET_MASTER_SET: grand=%v\n", level))}
	}

	rawGroup, ok := flags["group"]
	if !ok {
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_USAGE: --grand or --group is required; usage: %s\n", masterSetUsage))}
	}
	groupID, parseErr := uuid.Parse(rawGroup)
	if parseErr != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_USAGE: --group value %q is not a valid UUID; usage: %s\n", rawGroup, masterSetUsage))}
	}
	rawLevel, ok := flags["level"]
	if !ok {
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_USAGE: --level is required with --group; usage: %s\n", masterSetUsage))}
	}
	level, parseErr := strconv.ParseFloat(rawLevel, 64)
	if parseErr != nil {
		return ipc.Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_USAGE: --level value %q is not a valid number; usage: %s\n", rawLevel, masterSetUsage))}
	}
	if err := d.safety.setGroupMaster(groupID, level); err != nil {
		return ipc.Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return ipc.Result{Stdout: []byte(fmt.Sprintf("GOLC_ARTNET_MASTER_SET: group=%s level=%v\n", groupID, level))}
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
