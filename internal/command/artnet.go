// artnet.go is the artnet command file (04-05-PLAN.md): it owns the
// "artnet" routing scope and self-registers this phase's operator-facing
// routes (D-01, no standalone GUI/Wails window). This file (Task 1)
// declares the scope plus the "artnet serve" (D-03/D-04, the one route
// that IS the long-lived daemon, not a client), "artnet interface list"
// (ARTN-01), and "artnet configure" (CONTEXT D-08) routes; "artnet
// status" and "artnet target enable|disable" follow in Task 2.
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
	Route:   "artnet interface list",
	Summary: "List candidate Windows network interfaces for Art-Net output (ARTN-01): artnet interface list [--json].",
	Handler: runArtnetInterfaceList,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "artnet configure",
	Summary: "Add or update one unicast Art-Net output target for a universe (CONTEXT D-08): " +
		"artnet configure --universe <n> --ip <address> [--port <port>] [--enabled true|false] [--pipe <name>].",
	Handler: runArtnetConfigure,
})

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

// runArtnetInterfaceList serves the self-registered "artnet interface
// list" route (ARTN-01): it calls artnet.ListCandidateInterfaces()
// directly -- no daemon round trip is needed for OS-level enumeration --
// and renders either a plain table or (with --json) canonical JSON.
func runArtnetInterfaceList(request Request) Result {
	usage := "artnet interface list [--json]"
	values, err := parseArtnetArgs(usage, request.Args, map[string]bool{"json": true})
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	interfaces, err := artnet.ListCandidateInterfaces()
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if values["json"] == "true" {
		payload, encodeErr := strictjson.CanonicalEncode(interfaces)
		if encodeErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_ARTNET_INTERFACE_ENCODE_FAILED: %v\n", encodeErr))}
		}
		return Result{Stdout: payload}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%-6s %-24s %-6s %s\n", "INDEX", "NAME", "UP", "ADDRS")
	for _, iface := range interfaces {
		addrs := make([]string, 0, len(iface.Addrs))
		for _, a := range iface.Addrs {
			addrs = append(addrs, a.String())
		}
		fmt.Fprintf(&b, "%-6d %-24s %-6v %s\n", iface.Index, iface.Name, iface.Up, strings.Join(addrs, ","))
	}
	return Result{Stdout: []byte(b.String())}
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
