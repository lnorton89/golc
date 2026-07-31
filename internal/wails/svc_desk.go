// svc_desk.go fills DeskService, the Wails binding backing the Perform >
// Desk workspace (a QLC+-style "Simple Desk" fader view): every mutation
// (SetAttribute/ClearAttribute/ClearInstance/ClearAll) dispatches the
// matching already-implemented, already-tested "artnet desk ..." CLI route
// (internal/command/artnet.go) through the in-process command registry --
// exactly the ArtnetConfigService/ProgrammingService pattern this file
// mirrors -- so there is only one live-channel-override implementation in
// this codebase, never a second one duplicated for the GUI.
//
// FetchUniverseValues issues "artnet status --json" and projects only its
// "universes" member (internal/artnet/daemon.go's own statusPayload.
// Universes, ARTN-05) into a JSON-safe, byte-decoded view: this is the same
// per-tick final DMX buffer TargetHealth/RecordUniverseValues already
// tracks, already inclusive of every desk override applyDeskOverrides
// composited in before the safety/master transform, so the Desk workspace
// can visualize live channel values (including any active override) with
// no separate "read the desk state" round trip.
package wails

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lnorton89/golc/internal/command"
)

// DeskService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. pipeName is forwarded as "--pipe <s.pipeName>"
// on every dispatched route (mirrors ArtnetConfigService's identical
// field); root is the command.Request.Root every dispatched call carries
// (unused by the artnet routes themselves today, kept for parity with
// every other bound service in this package).
type DeskService struct {
	pipeName string
	root     string
}

// NewDeskService constructs a DeskService targeting pipeName and root.
func NewDeskService(pipeName, root string) *DeskService {
	return &DeskService{pipeName: pipeName, root: root}
}

// execute runs a full "artnet ..." route-plus-args word sequence, with
// "--pipe <s.pipeName>" always appended, through a freshly built default
// command registry (mirrors ArtnetConfigService.execute exactly).
func (s *DeskService) execute(args ...string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_REGISTRY_BUILD_FAILED: " + err.Error()}
	}
	fullArgs := append(append([]string(nil), args...), "--pipe", s.pipeName)
	result := registry.Execute(command.Request{Root: s.root, Args: fullArgs})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// formatDeskAttr builds one "capability=value" --attr value, formatting
// value with strconv.FormatFloat's shortest-round-trip representation
// (mirrors ProgrammingService.CreateChase's identical convention) rather
// than fmt's %v, which can emit scientific notation for very small
// normalized values.
func formatDeskAttr(capability string, value float64) string {
	return capability + "=" + strconv.FormatFloat(value, 'f', -1, 64)
}

// SetAttribute issues "artnet desk set --instance <instanceID> --attr
// <capability>=<value>" (the Desk workspace's own fader-drag write path):
// value is the normalized [0,1] capability value every other capability
// value in this codebase uses (fixture.Capability.Range's own doc
// comment). Takes effect on the Art-Net worker's very next tick, always
// subject to Blackout/Stop-All/master scaling, independent of whether any
// scene is active.
func (s *DeskService) SetAttribute(instanceID, capability string, value float64) Result {
	return s.execute("artnet", "desk", "set", "--instance", instanceID, "--attr", formatDeskAttr(capability, value))
}

// ClearAttribute issues "artnet desk clear --instance <instanceID> --attr
// <capability>", releasing one channel's override back to scene-
// passthrough (the Desk workspace's own per-fader "revert to programmed"
// control).
func (s *DeskService) ClearAttribute(instanceID, capability string) Result {
	return s.execute("artnet", "desk", "clear", "--instance", instanceID, "--attr", capability)
}

// ClearInstance issues "artnet desk clear --instance <instanceID>",
// releasing every override on that instance at once.
func (s *DeskService) ClearInstance(instanceID string) Result {
	return s.execute("artnet", "desk", "clear", "--instance", instanceID)
}

// ClearAll issues "artnet desk clear-all", releasing every manual desk
// override across every instance at once (the Desk workspace's own
// "release all faders" control).
func (s *DeskService) ClearAll() Result {
	return s.execute("artnet", "desk", "clear-all")
}

// DeskUniverseValuesView is one universe's final per-tick DMX buffer,
// decoded from base64 into plain 0-255 integers so the frontend never has
// to decode a base64 string itself just to index a channel value.
type DeskUniverseValuesView struct {
	Universe int   `json:"universe"`
	Values   []int `json:"values"`
}

// deskStatusWire decodes only the "universes" member of "artnet status
// --json" (internal/artnet/daemon.go's own statusPayload/universeValues,
// ARTN-05) -- mirrors svc_safety.go's daemonPlaybackEnvelope/
// svc_artnetconfig.go's artnetStatusWire discipline of decoding only the
// members a given service actually projects; plain encoding/json ignores
// every other member.
type deskStatusWire struct {
	Universes []struct {
		Universe int    `json:"universe"`
		Values   []byte `json:"values"`
	} `json:"universes"`
}

// FetchUniverseValues issues "artnet status --json" and projects its
// "universes" member into a byte-decoded, JSON-safe view (see package doc
// comment for why this already reflects every active desk override). A
// dial failure, non-zero daemon result, or undecodable response returns a
// non-nil empty slice (never null) alongside the error -- the Desk
// workspace's own daemon-unreachable state is driven by the returned
// error, mirroring FetchArtnetStatus/FetchStatus's "never a blank success"
// discipline applied to a Go (value, error) return instead of an explicit
// offline sentinel struct.
func (s *DeskService) FetchUniverseValues() ([]DeskUniverseValuesView, error) {
	result := s.execute("artnet", "status", "--json")
	if result.ExitCode != 0 {
		return []DeskUniverseValuesView{}, fmt.Errorf("%s", result.Stderr)
	}

	var wire deskStatusWire
	if err := json.Unmarshal([]byte(result.Stdout), &wire); err != nil {
		return []DeskUniverseValuesView{}, fmt.Errorf("GOLC_WAILS_DESK_STATUS_DECODE_FAILED: %v", err)
	}

	views := make([]DeskUniverseValuesView, 0, len(wire.Universes))
	for _, u := range wire.Universes {
		values := make([]int, len(u.Values))
		for i, b := range u.Values {
			values[i] = int(b)
		}
		views = append(views, DeskUniverseValuesView{Universe: u.Universe, Values: values})
	}
	return views, nil
}
