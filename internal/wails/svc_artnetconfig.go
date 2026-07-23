// svc_artnetconfig.go fills ArtnetConfigService, the Wails binding closing
// VERIFICATION.md Gap B[0] for PLAY-11 (06-11-PLAN.md): a show author
// lists available network interfaces, configures Art-Net universe ->
// unicast target mappings, enables/disables individual targets, and reads
// live per-target status -- all on screen, driving the exact same
// "artnet interface list"/"artnet configure"/"artnet target enable"/
// "artnet target disable"/"artnet status" CLI routes internal/command/
// artnet.go already implements and tests, executed through the in-process
// command registry (svc_playback.go's execute pattern, mirrored below) so
// the route's own artnet.ValidateTarget-before-forward discipline
// (T-04-07) runs unmodified -- there is only one Art-Net configuration
// mutation implementation in this codebase, never a second one duplicated
// for the GUI.
//
// This file deliberately never imports internal/artnet (T-06-33): the
// "artnet ..." CLI routes it dispatches through already do, but
// ArtnetConfigService itself only imports internal/command (to build the
// registry) and mirrors the daemon's JSON wire shapes locally --
// artnetInterfaceWire/artnetStatusWire/artnetTargetWire below -- exactly
// as svc_safety.go's daemonPlaybackEnvelope mirrors the daemon's playback
// JSON rather than importing internal/artnet's own (untagged, wire-
// incompatible-by-name) TargetHealth/Target types directly. This keeps
// ArtnetConfigService a thin two-hop client (frontend -> this Go host ->
// artnet CLI route -> supervised daemon IPC), never a second Art-Net
// output path.
//
// FetchArtnetStatus returns the explicit offlineArtnetStatus() projection
// (Reachable=false, a non-nil empty Targets slice) on any failure --
// unreachable daemon, non-zero route result, or an undecodable response --
// mirroring svc_safety.go's offlineStatusSnapshot discipline (D-13/
// 06-UI-SPEC.md: never a blank/partial success the frontend has to guess
// the meaning of). ListInterfaces, by contrast, is OS-level enumeration
// (internal/command's own runArtnetInterfaceList never dials the daemon to
// succeed -- it only best-effort annotates the pinned interface when a
// daemon happens to be reachable), so it still returns a full interface
// list with every entry's Pinned=false when the daemon is offline, rather
// than an error.
package wails

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lnorton89/golc/internal/command"
)

// ArtnetConfigService is bound to the frontend via cmd/golc-desktop/
// main.go's options.App{Bind: [...]}. pipeName is forwarded as "--pipe
// <name>" on every dispatched route so a test (or a future multi-daemon
// setup) can target an isolated pipe exactly like every other artnet CLI
// route already supports; root is the command.Request.Root every
// dispatched call carries (unused by the artnet routes themselves today,
// but kept for parity with every other bound service in this package).
type ArtnetConfigService struct {
	pipeName string
	root     string
}

// NewArtnetConfigService constructs an ArtnetConfigService targeting
// pipeName and root.
func NewArtnetConfigService(pipeName, root string) *ArtnetConfigService {
	return &ArtnetConfigService{pipeName: pipeName, root: root}
}

// execute runs a full "artnet ..." route-plus-args word sequence, with
// "--pipe <s.pipeName>" always appended, through a freshly built default
// command registry -- the identical in-process path
// cmd/golc-project/main.go's run() takes, so a Wails-bound call and a CLI
// invocation of the exact same route behave identically (mirrors
// svc_playback.go's own execute helper). A registry-build failure (only
// ever a duplicate-registration programming error, never live input)
// surfaces as a Result rather than a panic.
func (s *ArtnetConfigService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_REGISTRY_BUILD_FAILED: " + err.Error()}
	}
	fullArgs := append(append([]string(nil), args...), "--pipe", s.pipeName)
	result := registry.Execute(command.Request{Root: s.root, Args: fullArgs})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// targetArgs builds the shared "--universe <n> --ip <ip> [--port <n>]"
// argument shape "artnet configure"/"artnet target enable"/"artnet target
// disable" all expect (internal/command/artnet.go's targetSelectorArgs).
// port<=0 omits --port entirely, meaning "use the daemon's default port"
// (mirrors artnet.Target.Port's own zero-value-means-default convention)
// rather than ever forwarding a literal 0.
func targetArgs(universe int, ip string, port int) []string {
	args := []string{"--universe", strconv.Itoa(universe), "--ip", ip}
	if port > 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	return args
}

// Configure issues "artnet configure --universe <n> --ip <ip> [--port <n>]
// --enabled <bool>" (PLAY-11 must_haves: configure a universe -> unicast
// target). A malformed/out-of-range target is rejected by the route's own
// artnet.ValidateTarget check with GOLC_ARTNET_USAGE/
// GOLC_ARTNET_TARGET_INVALID before any daemon round trip (T-04-07) --
// this method never re-validates or duplicates that check itself.
func (s *ArtnetConfigService) Configure(universe int, ip string, port int, enabled bool) Result {
	args := append([]string{"artnet", "configure"}, targetArgs(universe, ip, port)...)
	args = append(args, "--enabled", strconv.FormatBool(enabled))
	return s.execute(args...)
}

// EnableTarget issues "artnet target enable --universe <n> --ip <ip>
// [--port <n>]" (PLAY-11 must_haves: enable a configured target). An
// unmatched selector surfaces the route's own GOLC_ARTNET_TARGET_NOT_FOUND
// diagnostic.
func (s *ArtnetConfigService) EnableTarget(universe int, ip string, port int) Result {
	args := append([]string{"artnet", "target", "enable"}, targetArgs(universe, ip, port)...)
	return s.execute(args...)
}

// DisableTarget issues "artnet target disable --universe <n> --ip <ip>
// [--port <n>]" (PLAY-11 must_haves: disable a configured target).
func (s *ArtnetConfigService) DisableTarget(universe int, ip string, port int) Result {
	args := append([]string{"artnet", "target", "disable"}, targetArgs(universe, ip, port)...)
	return s.execute(args...)
}

// ArtnetInterfaceView mirrors internal/command/artnet.go's own
// interfaceListEntry JSON shape field-for-field (Index/Name/Up/Addrs are
// artnet.InterfaceInfo's own OS enumeration; Pinned/Status/Error annotate
// the daemon's pinned candidate and its live status when reachable, all
// zero-valued otherwise) -- declared separately here rather than imported
// since that type is unexported in a different package.
type ArtnetInterfaceView struct {
	Index  int      `json:"index"`
	Name   string   `json:"name"`
	Up     bool     `json:"up"`
	Addrs  []string `json:"addrs"`
	Pinned bool     `json:"pinned"`
	Status string   `json:"status"`
	Error  string   `json:"error"`
}

// ListInterfaces issues "artnet interface list --json" (PLAY-11
// must_haves: list available network interfaces). This is OS-level
// enumeration (internal/command's runArtnetInterfaceList calls
// artnet.ListCandidateInterfaces() directly and only best-effort
// annotates the pinned interface when a daemon happens to be reachable),
// so it succeeds and returns every interface with Pinned=false even when
// the daemon cannot be reached -- never an error standing in for "the
// daemon is offline."
func (s *ArtnetConfigService) ListInterfaces() ([]ArtnetInterfaceView, error) {
	result := s.execute("artnet", "interface", "list", "--json")
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("%s", result.Stderr)
	}
	entries := make([]ArtnetInterfaceView, 0)
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		return nil, fmt.Errorf("GOLC_WAILS_ARTNET_INTERFACE_DECODE_FAILED: %v", err)
	}
	return entries, nil
}

// ArtnetPinnedInterfaceView mirrors internal/artnet/daemon.go's own
// interfaceStatusPayload JSON shape field-for-field (same json tags), the
// "interface" member of "artnet status --json".
type ArtnetPinnedInterfaceView struct {
	PinnedIndex int    `json:"pinnedIndex"`
	PinnedName  string `json:"pinnedName"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}

// ArtnetTargetView is the JSON-safe, camelCase per-target status row this
// service projects for the frontend, built from artnetTargetWire's
// decoded (untagged, Go-field-name) daemon wire shape below.
type ArtnetTargetView struct {
	Universe  int    `json:"universe"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Enabled   bool   `json:"enabled"`
	SendOK    int    `json:"sendOk"`
	SendErr   int    `json:"sendErr"`
	Reachable bool   `json:"reachable"`
	LastError string `json:"lastError"`
}

// ArtnetStatusView is FetchArtnetStatus's full JSON-safe return shape.
type ArtnetStatusView struct {
	Reachable bool                      `json:"reachable"`
	Interface ArtnetPinnedInterfaceView `json:"interface"`
	Targets   []ArtnetTargetView        `json:"targets"`
}

// offlineArtnetStatus is the explicit, never-blank idle projection
// FetchArtnetStatus returns whenever the daemon cannot be reached or its
// response cannot be decoded (06-UI-SPEC.md error state, D-13-style
// discipline mirrored from svc_safety.go's offlineStatusSnapshot). Targets
// is a non-nil empty slice, never null.
func offlineArtnetStatus() ArtnetStatusView {
	return ArtnetStatusView{Targets: []ArtnetTargetView{}}
}

// artnetTargetWire decodes one entry of "artnet status --json"'s
// "targets" array. internal/artnet.TargetHealth and internal/artnet.Target
// declare no json tags at all, so the daemon's actual wire shape for this
// nested object uses their plain (capitalized) Go field names -- this type
// mirrors that exact untagged shape (never imported from internal/artnet
// directly, per this file's own package doc comment) so
// encoding/json.Unmarshal matches it byte-for-byte.
type artnetTargetWire struct {
	Universe int
	Target   struct {
		Universe int
		IP       string
		Port     int
		Enabled  bool
	}
	SendOK    int
	SendErr   int
	Reachable bool
	LastError string
}

// artnetStatusWire decodes the members of "artnet status --json" this
// service projects: Targets (untagged nested shape, see artnetTargetWire)
// and Interface (internal/artnet/daemon.go's interfaceStatusPayload, which
// IS tagged camelCase). Frame/Universes/Playback are intentionally
// undeclared here (plain encoding/json ignores unknown-to-this-struct
// members) -- this service has no on-screen use for them yet, mirroring
// svc_safety.go's own daemonPlaybackEnvelope discipline of decoding only
// the members a given service actually projects.
type artnetStatusWire struct {
	Targets   []artnetTargetWire        `json:"targets"`
	Interface ArtnetPinnedInterfaceView `json:"interface"`
}

// FetchArtnetStatus issues "artnet status --json" and projects the
// decoded targets/interface fields into an ArtnetStatusView (PLAY-11
// must_haves: current interface and per-target status visible on screen).
// Any failure along the way (route error, or an undecodable response)
// returns offlineArtnetStatus() rather than a zero-valued/partial view --
// the frontend's daemon-unreachable state must always be an explicit
// signal, never blank fields a caller has to infer meaning from.
func (s *ArtnetConfigService) FetchArtnetStatus() ArtnetStatusView {
	result := s.execute("artnet", "status", "--json")
	if result.ExitCode != 0 {
		return offlineArtnetStatus()
	}

	var wire artnetStatusWire
	if err := json.Unmarshal([]byte(result.Stdout), &wire); err != nil {
		return offlineArtnetStatus()
	}

	targets := make([]ArtnetTargetView, 0, len(wire.Targets))
	for _, t := range wire.Targets {
		targets = append(targets, ArtnetTargetView{
			Universe:  t.Universe,
			IP:        t.Target.IP,
			Port:      t.Target.Port,
			Enabled:   t.Target.Enabled,
			SendOK:    t.SendOK,
			SendErr:   t.SendErr,
			Reachable: t.Reachable,
			LastError: t.LastError,
		})
	}

	return ArtnetStatusView{
		Reachable: true,
		Interface: wire.Interface,
		Targets:   targets,
	}
}
