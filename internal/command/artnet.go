// artnet.go is the artnet command file (04-05-PLAN.md): it owns the
// "artnet" routing scope and self-registers this phase's operator-facing
// routes (D-01, no standalone GUI/Wails window). Task 1 declared the
// scope plus "artnet serve" (D-03/D-04, the one route that IS the
// long-lived daemon, not a client), "artnet interface list" (ARTN-01),
// and "artnet configure" (CONTEXT D-08); Task 2 (below) adds "artnet
// status" (ARTN-05, D-02: one-shot snapshot plus a continuously-
// refreshing watch view, plain by default with --json for scripting) and
// "artnet target enable"/"artnet target disable" (D-12: take one target
// online/offline without stopping the rest of the rig).
//
// Every route except "artnet serve" is a thin client: it dials the
// running daemon's local named-pipe IPC listener (internal/artnet/ipc,
// Plan 04) and forwards a command.Request, converting to/from the ipc
// package's own local wire types at this call boundary (see
// internal/artnet/ipc/types.go's doc comment for why ipc/artnet never
// import internal/command themselves -- doing so would create a
// command -> artnet(/ipc) -> command import cycle, since this file needs
// to import both internal/artnet (Run, ListCandidateInterfaces) and
// internal/artnet/ipc (Dial, Forward) directly). A dial failure on any
// client route always surfaces as GOLC_ARTNET_DAEMON_UNREACHABLE,
// ExitCode 1, never a hang (Pattern 5).
//
// Arg parsing follows this repo's two-tier convention (internal/command/
// playback.go): malformed args (missing/unknown flags, non-numeric
// values) are GOLC_ARTNET_USAGE with ExitCode 2; validated-but-rejected
// domain values (e.g. an invalid target) are GOLC_ARTNET_* domain errors
// with ExitCode 1.
//
// 04-06-PLAN.md Task 2 adds "artnet discover" (ARTN-02, CONTEXT D-06):
// like "artnet interface list", it is a direct OS/network operation, not
// a daemon client -- it never dials the running daemon and never calls
// any configure/enable/disable path. Its results are suggestions only;
// there is no "add all discovered nodes" bulk-apply anywhere in this
// file, and promoting a suggestion to a live unicast target always
// requires a separate, explicit "artnet configure" invocation.
package command

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet"
	artnetipc "github.com/lnorton89/golc/internal/artnet/ipc"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "artnet",
	Summary: "Configure and inspect Art-Net live output (D-01): the CLI-only operator surface for the long-lived Art-Net daemon.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet serve",
	Summary: "Start the long-lived headless Art-Net daemon in the foreground (D-03/D-04): " +
		"artnet serve --show <path> --interface <index> [--interface-name <name>] [--fixtures <dir>] [--pipe <name>].",
	Handler: runArtnetServe,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet interface list",
	Summary: "List candidate Windows network interfaces for Art-Net output, annotating the daemon's pinned interface and its live status when reachable (ARTN-01/D-05): " +
		"artnet interface list [--json] [--pipe <name>].",
	Handler: runArtnetInterfaceList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet configure",
	Summary: "Add or update one unicast Art-Net output target for a universe (CONTEXT D-08): " +
		"artnet configure --universe <n> --ip <address> [--port <port>] [--enabled true|false] [--pipe <name>].",
	Handler: runArtnetConfigure,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "artnet status",
	Summary: "Inspect per-universe/target Art-Net health as a snapshot or watch view (ARTN-05, D-02): artnet status [--watch] [--json] [--pipe <name>].",
	Handler: runArtnetStatus,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet discover",
	Summary: "Scan a pinned interface for compatible Art-Net nodes and list them as suggestions only (ARTN-02, CONTEXT D-06): " +
		"artnet discover --interface <index> [--window <duration>] [--json]. Never adds/removes/modifies a live target -- " +
		"promoting a suggestion to a target is a separate 'artnet configure' action.",
	Handler: runArtnetDiscover,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet target enable",
	Summary: "Re-enable output to one configured unicast target without stopping the rig (D-12): " +
		"artnet target enable --universe <n> --ip <address> [--port <port>] [--pipe <name>].",
	Handler: runArtnetTargetEnable,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet target disable",
	Summary: "Take one configured unicast target offline without stopping the rig (D-12): " +
		"artnet target disable --universe <n> --ip <address> [--port <port>] [--pipe <name>].",
	Handler: runArtnetTargetDisable,
})

// artnetWatchInterval is "artnet status --watch"'s own refresh cadence --
// independent of the daemon's 40Hz worker tick and 1Hz interface poll,
// slow enough to be readable, fast enough to feel live (D-02/D-11).
const artnetWatchInterval = 500 * time.Millisecond

// artnetDefaultDisplayPort mirrors internal/artnet's own unexported
// effectivePort default (the fixed Art-Net UDP port) for display purposes
// only, since a Target with Port left at its zero value means "use the
// default" -- this is never sent over the wire, only rendered.
const artnetDefaultDisplayPort = 6454

// parseArtnetArgs parses args as a sequence of "--flag value" or
// "--flag=value" pairs (mirroring internal/command/playback.go's two-tier
// convention). Names present in boolFlags never require a value ("--name"
// alone sets it to "true"); every other name requires one. Anything not
// starting with "--" is rejected as GOLC_ARTNET_USAGE.
func parseArtnetArgs(usage string, args []string, boolFlags map[string]bool) (map[string]string, error) {
	values := map[string]string{}
	for i := 0; i < len(args); {
		argument := args[i]
		if !strings.HasPrefix(argument, "--") {
			return nil, fmt.Errorf("GOLC_ARTNET_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		if eq := strings.Index(argument, "="); eq >= 0 {
			name := argument[2:eq]
			if boolFlags[name] {
				return nil, fmt.Errorf("GOLC_ARTNET_USAGE: --%s does not take a value; usage: %s", name, usage)
			}
			values[name] = argument[eq+1:]
			i++
			continue
		}
		name := strings.TrimPrefix(argument, "--")
		if boolFlags[name] {
			values[name] = "true"
			i++
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("GOLC_ARTNET_USAGE: --%s requires a value; usage: %s", name, usage)
		}
		values[name] = args[i+1]
		i += 2
	}
	return values, nil
}

// pipeNameFromFlags resolves the "--pipe <name>" override every artnet
// route accepts (tests dial an isolated per-test pipe so they never
// collide with a real running daemon), defaulting to the production
// artnetipc.PipeName.
func pipeNameFromFlags(values map[string]string) string {
	if v, ok := values["pipe"]; ok && v != "" {
		return v
	}
	return artnetipc.PipeName
}

// forwardToDaemon dials pipeName and forwards request, converting between
// this package's Request/Result and the ipc package's own local wire
// types (RESEARCH.md Pattern 5). A dial failure surfaces as
// GOLC_ARTNET_DAEMON_UNREACHABLE, ExitCode 1, never a hang -- artnetipc.Dial
// already produces that exact diagnostic.
func forwardToDaemon(pipeName string, request Request) Result {
	conn, err := artnetipc.Dial(pipeName)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	defer conn.Close()

	result := artnetipc.Forward(conn, artnetipc.Request{
		Route: request.Route,
		Args:  request.Args,
		Root:  request.Root,
	})
	return Result{ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr}
}

// --- artnet serve -----------------------------------------------------

// artnetServeArgs is the parsed shape of one "artnet serve" invocation.
type artnetServeArgs struct {
	showPath      string
	interfaceIdx  int
	interfaceName string
	fixturesDir   string
	pipeName      string
}

// parseArtnetServeArgs accepts a required --show path, a required
// --interface index, and optional --interface-name/--fixtures/--pipe
// values, rejecting anything else (GOLC_ARTNET_USAGE). --interface is the
// durable net.Interface.Index CONTEXT D-05 pins by, never a display name.
func parseArtnetServeArgs(usage string, args []string) (artnetServeArgs, error) {
	values, err := parseArtnetArgs(usage, args, nil)
	if err != nil {
		return artnetServeArgs{}, err
	}

	showPath, ok := values["show"]
	if !ok || showPath == "" {
		return artnetServeArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --show is required; usage: %s", usage)
	}

	rawIdx, ok := values["interface"]
	if !ok {
		return artnetServeArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --interface is required; usage: %s", usage)
	}
	idx, convErr := strconv.Atoi(rawIdx)
	if convErr != nil {
		return artnetServeArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --interface value %q is not a valid integer; usage: %s", rawIdx, usage)
	}

	return artnetServeArgs{
		showPath:      showPath,
		interfaceIdx:  idx,
		interfaceName: values["interface-name"],
		fixturesDir:   values["fixtures"],
		pipeName:      pipeNameFromFlags(values),
	}, nil
}

// activeDeploymentInstances returns a fresh copy of the single active
// deployment's instances (CONTEXT D-08/POOL-02's exactly-one-active
// invariant), or nil if no deployment is active yet.
func activeDeploymentInstances(state show.State) []deployment.Instance {
	for _, d := range state.Deployments {
		if d.Active {
			return append([]deployment.Instance(nil), d.Instances...)
		}
	}
	return nil
}

// poolMemberRef pairs a pool.PoolMember with the pool.Pool that owns it,
// keyed by PoolMember.ID for artnetFixtureResolver's lookup.
type poolMemberRef struct {
	poolID uuid.UUID
	member pool.PoolMember
}

// loadFixtureDirectory decodes every *.yaml/*.yml file directly inside dir
// (non-recursive) via fixture.Decode -- the same strict decode/validate
// pipeline "fixture validate"/"pool substitute" already use, no second
// decode path invented -- and indexes each by its fixture.Pin'd StableKey.
func loadFixtureDirectory(dir string) (map[string]fixture.FixtureDefinition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %v", err)
	}
	byStableKey := map[string]fixture.FixtureDefinition{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), readErr)
		}
		def, decodeErr := fixture.Decode(data)
		if decodeErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), decodeErr)
		}
		identity, pinErr := fixture.Pin(def)
		if pinErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), pinErr)
		}
		byStableKey[identity.StableKey] = def
	}
	return byStableKey, nil
}

// newArtnetFixtureResolver builds the artnet.ResolveFunc "artnet serve"
// passes to artnet.Run. This repository has no fixture-store/lookup
// service yet (a real fixture library is a larger, future architectural
// addition outside this plan's scope): when fixturesDir is empty, the
// returned resolver always fails with GOLC_ARTNET_FIXTURES_DIR_REQUIRED
// rather than ever silently guessing a fixture (mirrors D-17's "never
// silently guess" convention) -- harmless when no deployment is active
// yet, since Resolve is then never called. When fixturesDir is given,
// every fixture file under it is decoded once up front and instances
// resolve by walking state.Pools to find the pool member's declared
// FixtureStableKey, then that fixture's Mode matching Instance.Mode.
func newArtnetFixtureResolver(root, fixturesDir string, pools []pool.Pool) (artnet.ResolveFunc, error) {
	if fixturesDir == "" {
		return func(instance deployment.Instance) (artnet.InstanceFixture, error) {
			return artnet.InstanceFixture{}, fmt.Errorf(
				"GOLC_ARTNET_FIXTURES_DIR_REQUIRED: instance %s needs fixture resolution but no --fixtures directory was given", instance.ID)
		}, nil
	}

	memberByID := map[uuid.UUID]poolMemberRef{}
	for _, p := range pools {
		for _, m := range p.Members {
			memberByID[m.ID] = poolMemberRef{poolID: p.ID, member: m}
		}
	}

	byStableKey, err := loadFixtureDirectory(resolveWritablePath(root, fixturesDir))
	if err != nil {
		return nil, err
	}

	return func(instance deployment.Instance) (artnet.InstanceFixture, error) {
		ref, ok := memberByID[instance.PoolMemberID]
		if !ok {
			return artnet.InstanceFixture{}, fmt.Errorf(
				"GOLC_ARTNET_POOL_MEMBER_NOT_FOUND: instance %s references pool member %s, which is not in any configured pool", instance.ID, instance.PoolMemberID)
		}
		def, ok := byStableKey[ref.member.FixtureStableKey]
		if !ok {
			return artnet.InstanceFixture{}, fmt.Errorf(
				"GOLC_ARTNET_FIXTURE_NOT_FOUND: pool member %s references fixture stable key %q, which was not found under --fixtures", ref.member.ID, ref.member.FixtureStableKey)
		}
		for _, mode := range def.Modes {
			if mode.Name == instance.Mode {
				return artnet.InstanceFixture{Definition: def, Mode: mode}, nil
			}
		}
		return artnet.InstanceFixture{}, fmt.Errorf(
			"GOLC_ARTNET_MODE_NOT_FOUND: instance %s requests mode %q, which is not declared on fixture %s/%s",
			instance.ID, instance.Mode, def.Manufacturer, def.Model)
	}, nil
}

// runArtnetServe serves the self-registered "artnet serve" route (D-03/
// D-04): the one route that IS the long-lived server, not a client. It
// loads the ShowState at --show, resolves the active deployment's
// instances and a fixture ResolveFunc (see newArtnetFixtureResolver), and
// calls artnet.Run in the foreground until interrupted (Ctrl+C) or the IPC
// listener fails, blocking for the daemon's entire lifetime.
func runArtnetServe(request Request) Result {
	usage := "artnet serve --show <path> --interface <index> [--interface-name <name>] [--fixtures <dir>] [--pipe <name>]"
	parsed, err := parseArtnetServeArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, parsed.showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	resolve, err := newArtnetFixtureResolver(request.Root, parsed.fixturesDir, state.Pools)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	cfg := artnet.Config{
		State:          state,
		InterfaceIndex: parsed.interfaceIdx,
		InterfaceName:  parsed.interfaceName,
		Instances:      activeDeploymentInstances(state),
		Resolve:        resolve,
		Targets:        map[int][]artnet.Target{},
		PipeName:       parsed.pipeName,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := artnet.Run(ctx, cfg); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_ARTNET_SERVE_FAILED: %v\n", err))}
	}
	return Result{Stdout: []byte("GOLC_ARTNET_SERVE: daemon stopped\n")}
}

// --- artnet interface list ---------------------------------------------

// interfaceListEntry is the self-describing per-candidate JSON rendering
// for "artnet interface list --json" (04-09-PLAN.md, ARTN-01/D-05):
// Index/Name/Up/Addrs mirror artnet.InterfaceInfo (Addrs stringified the
// same way the plain view already renders them), and Pinned/Status/Error
// annotate the daemon's pinned candidate, its live status, and (when
// lost) its error diagnostic when a daemon is reachable -- all
// zero-valued (false/""/"") otherwise. Error mirrors the plain-text
// rendering below, which already appends the same diagnostic to the
// status column (GC-WR-02: a scripting consumer of --json previously had
// no way to learn why a pinned interface was lost without a separate
// "artnet status --json" call).
type interfaceListEntry struct {
	Index  int      `json:"index"`
	Name   string   `json:"name"`
	Up     bool     `json:"up"`
	Addrs  []string `json:"addrs"`
	Pinned bool     `json:"pinned"`
	Status string   `json:"status"`
	Error  string   `json:"error"`
}

// runArtnetInterfaceList serves the self-registered "artnet interface
// list" route (ARTN-01): it calls artnet.ListCandidateInterfaces()
// directly -- enumeration is still daemon-free -- and then makes a
// best-effort daemon round trip to annotate which candidate is the
// daemon's pinned interface and its live status (04-09-PLAN.md,
// ARTN-01/D-05). When no daemon is reachable, the round trip's error is
// ignored and the plain candidate list renders exactly as before (ExitCode
// 0, full enumeration, no GOLC_ARTNET_DAEMON_UNREACHABLE regression).
func runArtnetInterfaceList(request Request) Result {
	usage := "artnet interface list [--json] [--pipe <name>]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	interfaces, err := artnet.ListCandidateInterfaces()
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	var (
		daemonReachable bool
		pinnedIndex     int
		pinnedStatus    string
		pinnedError     string
	)
	if statusPayload, _, ok := fetchArtnetStatus(pipeNameFromFlags(values), request.Root); ok {
		daemonReachable = true
		pinnedIndex = statusPayload.Interface.PinnedIndex
		pinnedStatus = statusPayload.Interface.Status
		pinnedError = statusPayload.Interface.Error
	}

	if values["json"] == "true" {
		entries := make([]interfaceListEntry, 0, len(interfaces))
		for _, iface := range interfaces {
			addrs := make([]string, 0, len(iface.Addrs))
			for _, a := range iface.Addrs {
				addrs = append(addrs, a.String())
			}
			entry := interfaceListEntry{Index: iface.Index, Name: iface.Name, Up: iface.Up, Addrs: addrs}
			if daemonReachable && iface.Index == pinnedIndex {
				entry.Pinned = true
				entry.Status = pinnedStatus
				entry.Error = pinnedError
			}
			entries = append(entries, entry)
		}
		payload, encodeErr := strictjson.CanonicalEncode(entries)
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_ARTNET_INTERFACE_ENCODE_FAILED: %v\n", encodeErr))}
		}
		return Result{Stdout: payload}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-6s %-24s %-6s %-8s %-6s %s\n", "INDEX", "NAME", "UP", "PINNED", "STATUS", "ADDRS")
	for _, iface := range interfaces {
		addrs := make([]string, 0, len(iface.Addrs))
		for _, a := range iface.Addrs {
			addrs = append(addrs, a.String())
		}
		pinned := ""
		status := ""
		if daemonReachable && iface.Index == pinnedIndex {
			pinned = "yes"
			status = pinnedStatus
			if pinnedError != "" {
				status = status + " " + pinnedError
			}
		}
		fmt.Fprintf(&b, "%-6d %-24s %-6v %-8s %-6s %s\n", iface.Index, iface.Name, iface.Up, pinned, status, strings.Join(addrs, ","))
	}
	return Result{Stdout: []byte(b.String())}
}

// --- artnet discover -----------------------------------------------------

// artnetDiscoverArgs is the parsed shape of one "artnet discover"
// invocation.
type artnetDiscoverArgs struct {
	interfaceIdx int
	window       time.Duration
	json         bool
}

// parseArtnetDiscoverArgs accepts a required --interface index (the same
// durable net.Interface.Index "artnet serve" pins by, never a display
// name), an optional --window duration (Go duration syntax, e.g. "3s"),
// and an optional --json flag, rejecting anything else as
// GOLC_ARTNET_USAGE.
func parseArtnetDiscoverArgs(usage string, args []string) (artnetDiscoverArgs, error) {
	values, err := parseArtnetArgs(usage, args, map[string]bool{"json": true})
	if err != nil {
		return artnetDiscoverArgs{}, err
	}

	rawIdx, ok := values["interface"]
	if !ok {
		return artnetDiscoverArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --interface is required; usage: %s", usage)
	}
	idx, convErr := strconv.Atoi(rawIdx)
	if convErr != nil {
		return artnetDiscoverArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --interface value %q is not a valid integer; usage: %s", rawIdx, usage)
	}

	var window time.Duration
	if raw, ok := values["window"]; ok {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil {
			return artnetDiscoverArgs{}, fmt.Errorf("GOLC_ARTNET_USAGE: --window value %q is not a valid duration; usage: %s", raw, usage)
		}
		window = parsed
	}

	return artnetDiscoverArgs{interfaceIdx: idx, window: window, json: values["json"] == "true"}, nil
}

// findInterfaceByIndex resolves index against artnet.ListCandidateInterfaces
// (no daemon round trip -- discovery, like "artnet interface list", is a
// direct OS-level operation, not a daemon-state read). An index matching
// no candidate interface is GOLC_ARTNET_INTERFACE_NOT_FOUND.
func findInterfaceByIndex(index int) (artnet.InterfaceInfo, error) {
	interfaces, err := artnet.ListCandidateInterfaces()
	if err != nil {
		return artnet.InterfaceInfo{}, err
	}
	for _, iface := range interfaces {
		if iface.Index == index {
			return iface, nil
		}
	}
	return artnet.InterfaceInfo{}, fmt.Errorf("GOLC_ARTNET_INTERFACE_NOT_FOUND: no network interface with index %d", index)
}

// renderArtnetDiscoverPlain renders nodes as the plain, human-readable
// suggestions table: an explicit header naming these as suggestions only
// (CONTEXT D-06 -- promoting one to a live target is a separate
// "artnet configure" action), then one row per discovered node.
func renderArtnetDiscoverPlain(nodes []artnet.DiscoveredNode) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "GOLC_ARTNET_DISCOVER: %d suggested node(s) -- adding one as a live target is a separate 'artnet configure' action (D-06)\n", len(nodes))
	fmt.Fprintf(&b, "%-16s %-20s %-24s %s\n", "IP", "SHORT_NAME", "LONG_NAME", "PORT_ADDRESSES")
	for _, n := range nodes {
		addrs := make([]string, 0, len(n.PortAddresses))
		for _, pa := range n.PortAddresses {
			addrs = append(addrs, fmt.Sprintf("0x%04x", pa))
		}
		fmt.Fprintf(&b, "%-16s %-20s %-24s %s\n", n.IP.String(), n.ShortName, n.LongName, strings.Join(addrs, ","))
	}
	return []byte(b.String())
}

// runArtnetDiscover serves the self-registered "artnet discover" route
// (ARTN-02, CONTEXT D-06): it resolves --interface to an InterfaceInfo,
// runs a bounded artnet.Discover scan, and renders the resulting nodes as
// suggestions only (plain table or --json). This route never dials the
// daemon and never calls artnet.ValidateTarget/ValidateUniqueTargets or
// any configure/enable/disable path -- there is no "add all discovered
// nodes" bulk-apply here or anywhere else in this file; promoting a
// suggestion to a live unicast target always requires a separate,
// explicit "artnet configure" invocation naming that exact node.
func runArtnetDiscover(request Request) Result {
	usage := "artnet discover --interface <index> [--window <duration>] [--json]"
	parsed, err := parseArtnetDiscoverArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	iface, err := findInterfaceByIndex(parsed.interfaceIdx)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	nodes, err := artnet.Discover(ctx, iface, parsed.window)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_ARTNET_DISCOVER_FAILED: %v\n", err))}
	}

	if parsed.json {
		payload, encodeErr := strictjson.CanonicalEncode(nodes)
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_ARTNET_DISCOVER_ENCODE_FAILED: %v\n", encodeErr))}
		}
		return Result{Stdout: payload}
	}

	return Result{Stdout: renderArtnetDiscoverPlain(nodes)}
}

// --- artnet configure ---------------------------------------------------

// parseArtnetTargetSelector reads the required --universe/--ip flags and
// the optional --port flag out of values, rejecting a missing or
// malformed value as GOLC_ARTNET_USAGE (a shape failure, ExitCode 2) --
// mirrors internal/artnet/daemon.go's own parseTargetSelector so the
// forwarded wire args land in the exact shape the daemon already expects.
func parseArtnetTargetSelector(usage string, values map[string]string) (universe int, ip net.IP, port int, err error) {
	rawUniverse, ok := values["universe"]
	if !ok {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --universe is required; usage: %s", usage)
	}
	universe, convErr := strconv.Atoi(rawUniverse)
	if convErr != nil {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --universe value %q is not a valid integer; usage: %s", rawUniverse, usage)
	}

	rawIP, ok := values["ip"]
	if !ok {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --ip is required; usage: %s", usage)
	}
	parsedIP := net.ParseIP(rawIP)
	if parsedIP == nil {
		return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --ip value %q is not a valid IP address; usage: %s", rawIP, usage)
	}

	if rawPort, ok := values["port"]; ok {
		parsedPort, portErr := strconv.Atoi(rawPort)
		if portErr != nil {
			return 0, nil, 0, fmt.Errorf("GOLC_ARTNET_USAGE: --port value %q is not a valid integer; usage: %s", rawPort, usage)
		}
		port = parsedPort
	}

	return universe, parsedIP, port, nil
}

// targetSelectorArgs builds the exact "--universe <n> --ip <address>
// [--port <port>]" wire-args shape internal/artnet/daemon.go's
// parseTargetSelector expects (04-04-PLAN.md's locked daemon-side wire
// contract) -- forwarded verbatim rather than passed through the CLI's own
// raw args, so an unrelated flag (e.g. --pipe) this file's own routes
// accept never reaches the daemon's parser.
func targetSelectorArgs(universe int, ip net.IP, port int, portGiven bool) []string {
	args := []string{"--universe", strconv.Itoa(universe), "--ip", ip.String()}
	if portGiven {
		args = append(args, "--port", strconv.Itoa(port))
	}
	return args
}

// runArtnetConfigure serves the self-registered "artnet configure" route
// (CONTEXT D-08): it parses and validates the target with
// artnet.ValidateTarget before ever forwarding to the daemon (T-04-07),
// so a shape failure is GOLC_ARTNET_USAGE (ExitCode 2) and a
// validated-but-rejected value is a GOLC_ARTNET_TARGET_INVALID domain
// error (ExitCode 1) without a daemon round trip at all.
func runArtnetConfigure(request Request) Result {
	usage := "artnet configure --universe <n> --ip <address> [--port <port>] [--enabled true|false] [--pipe <name>]"
	values, err := parseArtnetArgs(usage, request.Args, nil)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	universe, ip, port, err := parseArtnetTargetSelector(usage, values)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	_, portGiven := values["port"]

	enabled := true
	if raw, ok := values["enabled"]; ok {
		parsedEnabled, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
				"GOLC_ARTNET_USAGE: --enabled value %q is not a valid bool; usage: %s\n", raw, usage))}
		}
		enabled = parsedEnabled
	}

	target := artnet.Target{Universe: universe, IP: ip, Port: port, Enabled: enabled}
	if err := artnet.ValidateTarget(target); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	args := targetSelectorArgs(universe, ip, port, portGiven)
	args = append(args, "--enabled", strconv.FormatBool(enabled))

	return forwardToDaemon(pipeNameFromFlags(values), Request{
		Route: "artnet configure",
		Args:  args,
		Root:  request.Root,
	})
}

// --- artnet target enable/disable ---------------------------------------

// runArtnetTargetToggle is shared by "artnet target enable" and "artnet
// target disable" (CONTEXT D-12): it parses the target selector only
// (GOLC_ARTNET_USAGE on a shape failure), then forwards to the daemon,
// which reports an unmatched selector as GOLC_ARTNET_TARGET_NOT_FOUND
// (ExitCode 1) -- this route never re-validates the target itself, since
// enable/disable only needs to match an already-configured target, not
// pass ValidateTarget's own construction-time checks.
func runArtnetTargetToggle(route, usage string, request Request) Result {
	values, err := parseArtnetArgs(usage, request.Args, nil)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	universe, ip, port, err := parseArtnetTargetSelector(usage, values)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	_, portGiven := values["port"]

	return forwardToDaemon(pipeNameFromFlags(values), Request{
		Route: route,
		Args:  targetSelectorArgs(universe, ip, port, portGiven),
		Root:  request.Root,
	})
}

func runArtnetTargetEnable(request Request) Result {
	usage := "artnet target enable --universe <n> --ip <address> [--port <port>] [--pipe <name>]"
	return runArtnetTargetToggle("artnet target enable", usage, request)
}

func runArtnetTargetDisable(request Request) Result {
	usage := "artnet target disable --universe <n> --ip <address> [--port <port>] [--pipe <name>]"
	return runArtnetTargetToggle("artnet target disable", usage, request)
}

// --- artnet status ------------------------------------------------------

// artnetStatusPayload mirrors internal/artnet/daemon.go's own
// statusPayload wire shape exactly (same json tags) so DecodeStrict can
// parse "artnet status"'s Result.Stdout back into typed
// artnet.FrameHealth/artnet.TargetHealth/artnetUniverseValues/
// artnetInterfaceStatus values for rendering.
type artnetStatusPayload struct {
	Frame     artnet.FrameHealth     `json:"frame"`
	Targets   []artnet.TargetHealth  `json:"targets"`
	Universes []artnetUniverseValues `json:"universes"`
	Interface artnetInterfaceStatus  `json:"interface"`
}

// artnetInterfaceStatus mirrors internal/artnet/daemon.go's own
// interfaceStatusPayload wire shape exactly (same json tags, 04-09-PLAN.md,
// ARTN-01/D-05) so DecodeStrict can round-trip the daemon's pinned
// InterfaceManager's live status for rendering.
type artnetInterfaceStatus struct {
	PinnedIndex int    `json:"pinnedIndex"`
	PinnedName  string `json:"pinnedName"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}

// artnetUniverseValues mirrors internal/artnet/daemon.go's own
// universeValues wire shape exactly (same json tags) so DecodeStrict can
// round-trip the daemon's per-universe final DMX buffer (04-08-PLAN.md,
// ARTN-05).
type artnetUniverseValues struct {
	Universe int    `json:"universe"`
	Values   []byte `json:"values"`
}

// displayPort mirrors internal/artnet's own unexported default-port
// convention for display only: a Target.Port left at its zero value means
// "the fixed Art-Net UDP port" (never sent back over the wire as 0).
func displayPort(t artnet.Target) int {
	if t.Port == 0 {
		return artnetDefaultDisplayPort
	}
	return t.Port
}

// renderArtnetStatusPlain renders payload as the persistent, human-
// readable per-universe/target status table D-11 requires: frame
// cadence/staleness on its own summary line, then the pinned interface's
// live status line (04-09-PLAN.md, ARTN-01/D-05 -- a lost/degraded pinned
// interface is always visible here, never a silent switch), then one row
// per configured target with send success/error counts, reachability, and
// the last recorded error (if any), then one GOLC_ARTNET_UNIVERSE line per
// configured universe's final DMX values (04-08-PLAN.md, ARTN-05).
func renderArtnetStatusPlain(payload artnetStatusPayload) []byte {
	var b strings.Builder

	frameStatus := "on-cadence"
	if !payload.Frame.OnCadence {
		frameStatus = "STALLED"
	}
	lastFrameAt := "never"
	if !payload.Frame.LastFrameAt.IsZero() {
		lastFrameAt = payload.Frame.LastFrameAt.UTC().Format(time.RFC3339Nano)
	}
	fmt.Fprintf(&b, "GOLC_ARTNET_STATUS: frame=%s last_frame_at=%s\n", frameStatus, lastFrameAt)

	fmt.Fprintf(&b, "GOLC_ARTNET_INTERFACE_STATUS: index=%d name=%s status=%s",
		payload.Interface.PinnedIndex, payload.Interface.PinnedName, payload.Interface.Status)
	if payload.Interface.Error != "" {
		fmt.Fprintf(&b, " error=%s", payload.Interface.Error)
	}
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "%-6s %-20s %-6s %-8s %-8s %-9s %-6s %s\n",
		"UNIV", "TARGET", "PORT", "ENABLED", "SEND_OK", "SEND_ERR", "REACH", "LAST_ERROR")
	for _, t := range payload.Targets {
		fmt.Fprintf(&b, "%-6d %-20s %-6d %-8v %-8d %-9d %-6v %s\n",
			t.Universe, t.Target.IP.String(), displayPort(t.Target), t.Target.Enabled,
			t.SendOK, t.SendErr, t.Reachable, t.LastError)
	}

	for _, u := range payload.Universes {
		pairs := make([]string, 0)
		for i, v := range u.Values {
			if v != 0 {
				pairs = append(pairs, fmt.Sprintf("%d=%d", i+1, v))
			}
		}
		fmt.Fprintf(&b, "GOLC_ARTNET_UNIVERSE: universe=%d channels=%d nonzero=%d values=[%s]\n",
			u.Universe, len(u.Values), len(pairs), strings.Join(pairs, " "))
	}

	return []byte(b.String())
}

// fetchArtnetStatus dials the daemon, forwards "artnet status", and
// decodes the daemon's canonical JSON snapshot into artnetStatusPayload.
// On any failure (dial, non-zero daemon result, or decode) it returns
// ok=false and the exact Result the caller should return/print instead.
func fetchArtnetStatus(pipeName, root string) (artnetStatusPayload, Result, bool) {
	result := forwardToDaemon(pipeName, Request{Route: "artnet status", Root: root})
	if result.ExitCode != 0 {
		return artnetStatusPayload{}, result, false
	}

	var payload artnetStatusPayload
	if err := strictjson.DecodeStrict(result.Stdout, &payload); err != nil {
		return artnetStatusPayload{}, Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_STATUS_DECODE_FAILED: %v\n", err))}, false
	}
	return payload, Result{}, true
}

// runArtnetStatusWatch continuously re-fetches and re-renders the plain
// status table on artnetWatchInterval (D-02/D-11's watch view) until the
// operator interrupts (Ctrl+C) or a fetch fails.
func runArtnetStatusWatch(pipeName, root string) Result {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(artnetWatchInterval)
	defer ticker.Stop()

	for {
		payload, errResult, ok := fetchArtnetStatus(pipeName, root)
		if !ok {
			return errResult
		}
		os.Stdout.Write(renderArtnetStatusPlain(payload))
		fmt.Fprintln(os.Stdout, "--- (Ctrl+C to stop) ---")

		select {
		case <-sigCh:
			return Result{Stdout: []byte("GOLC_ARTNET_STATUS_WATCH: stopped\n")}
		case <-ticker.C:
		}
	}
}

// runArtnetStatus serves the self-registered "artnet status" route
// (ARTN-05, D-02): a one-shot snapshot (default plain human-readable
// output, --json for canonical JSON scripting) or, with --watch, a
// continuously-refreshing plain-text view. --watch and --json are
// mutually exclusive (the watch view is always the plain table).
func runArtnetStatus(request Request) Result {
	usage := "artnet status [--watch] [--json] [--pipe <name>]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"watch": true, "json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	watch := values["watch"] == "true"
	jsonOut := values["json"] == "true"
	if watch && jsonOut {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf(
			"GOLC_ARTNET_USAGE: --watch and --json are mutually exclusive; usage: %s\n", usage))}
	}

	pipeName := pipeNameFromFlags(values)

	if watch {
		return runArtnetStatusWatch(pipeName, request.Root)
	}

	if jsonOut {
		return forwardToDaemon(pipeName, Request{Route: "artnet status", Root: request.Root})
	}

	payload, errResult, ok := fetchArtnetStatus(pipeName, request.Root)
	if !ok {
		return errResult
	}
	return Result{Stdout: renderArtnetStatusPlain(payload)}
}
