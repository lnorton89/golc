// Command mock-golc-demo drives bonzupii/mock-golc's default Art-Net rig
// (https://github.com/bonzupii/mock-golc) from GOLC's own fixture/deployment/
// playback/artnet packages, as an end-to-end proof that a patched show can
// actually produce the DMX bytes the mock rig's built-in fixture profiles
// expect: RGB Par (Dimmer,R,G,B), RGBW Wash (Dimmer,R,G,B,W), and Moving
// Head (Dimmer,Pan,PanFine,Tilt,TiltFine,ColorWheel,GoboWheel,Shutter,Zoom)
// on universe 0 at the mock's documented default addresses.
//
// It builds one playback.Frame per animation tick directly (no show file,
// no scene/preset/programmer authoring surface -- those are optional
// authoring conveniences on top of the same Frame/Encode data path this
// tool exercises directly) and sends it as a real Art-Net ArtDMX UDP
// packet via internal/artnet's own wire encoder, so the bytes on the wire
// are exactly what GOLC's real Art-Net output worker would send for an
// identical Frame.
//
// Usage: go run ./tools/mock-golc-demo [-target 127.0.0.1:6454] [-hz 30]
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/artnet"
	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/scene"
)

//go:embed fixtures/rgb-par.yaml
var rgbParYAML []byte

//go:embed fixtures/rgbw-wash.yaml
var rgbwWashYAML []byte

//go:embed fixtures/moving-head.yaml
var movingHeadYAML []byte

// rigInstance is one of mock-golc's eight default-rig fixtures (README.md
// "Default Rig (Universe 0)" table): name, DMX start address, and which
// embedded fixture definition + mode drives it.
type rigInstance struct {
	name       string
	address    int
	definition *fixture.FixtureDefinition
	mode       string
}

func main() {
	target := flag.String("target", "127.0.0.1:6454", "mock-golc Art-Net listener address (host:port)")
	hz := flag.Float64("hz", 30, "animation frame rate in Hz")
	universe := flag.Int("universe", 0, "Art-Net universe (mock-golc's default rig is universe 0)")
	flag.Parse()

	if err := run(*target, *universe, *hz); err != nil {
		log.Fatalf("mock-golc-demo: %v", err)
	}
}

func run(target string, universe int, hz float64) error {
	rgbPar, err := decodeEmbedded(rgbParYAML)
	if err != nil {
		return fmt.Errorf("decoding rgb-par.yaml: %w", err)
	}
	rgbwWash, err := decodeEmbedded(rgbwWashYAML)
	if err != nil {
		return fmt.Errorf("decoding rgbw-wash.yaml: %w", err)
	}
	movingHead, err := decodeEmbedded(movingHeadYAML)
	if err != nil {
		return fmt.Errorf("decoding moving-head.yaml: %w", err)
	}

	rig := []rigInstance{
		{name: "Par 1", address: 1, definition: &rgbPar, mode: "4ch"},
		{name: "Par 2", address: 5, definition: &rgbPar, mode: "4ch"},
		{name: "Par 3", address: 9, definition: &rgbPar, mode: "4ch"},
		{name: "Par 4", address: 13, definition: &rgbPar, mode: "4ch"},
		{name: "Wash 1", address: 17, definition: &rgbwWash, mode: "5ch"},
		{name: "Wash 2", address: 22, definition: &rgbwWash, mode: "5ch"},
		{name: "Mover 1", address: 27, definition: &movingHead, mode: "9ch"},
		{name: "Mover 2", address: 36, definition: &movingHead, mode: "9ch"},
	}

	instances := make([]deployment.Instance, len(rig))
	instanceFixtures := make(map[uuid.UUID]artnet.InstanceFixture, len(rig))
	for i, r := range rig {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("minting instance id for %s: %w", r.name, err)
		}
		mode, err := resolveMode(*r.definition, r.mode)
		if err != nil {
			return fmt.Errorf("resolving mode for %s: %w", r.name, err)
		}
		instances[i] = deployment.Instance{ID: id, Universe: universe, Address: r.address}
		instanceFixtures[id] = artnet.InstanceFixture{Definition: *r.definition, Mode: mode}
	}

	resolve := func(instance deployment.Instance) (artnet.InstanceFixture, error) {
		resolved, ok := instanceFixtures[instance.ID]
		if !ok {
			return artnet.InstanceFixture{}, fmt.Errorf("no fixture resolved for instance %s", instance.ID)
		}
		return resolved, nil
	}

	conn, err := net.Dial("udp4", target)
	if err != nil {
		return fmt.Errorf("dialing %s: %w", target, err)
	}
	defer conn.Close()

	log.Printf("mock-golc-demo: sending universe %d to %s at %.0f Hz (Ctrl+C to stop)", universe, target, hz)

	tick := time.NewTicker(time.Duration(float64(time.Second) / hz))
	defer tick.Stop()

	var seq uint8
	start := time.Now()
	for now := range tick.C {
		elapsed := now.Sub(start).Seconds()

		frame := playback.Frame{Values: make(map[uuid.UUID]scene.AttributeSet, len(instances))}
		for i, r := range rig {
			frame.Values[instances[i].ID] = animateInstance(r, elapsed)
		}

		buffers, err := artnet.Encode(frame, instances, resolve)
		if err != nil {
			return fmt.Errorf("encoding frame: %w", err)
		}

		seq = nextSeq(seq)
		data, ok := buffers[universe]
		if !ok {
			continue
		}
		packet, err := artnet.EncodeArtDMX(seq, 0, artnet.PortAddress(universe), data)
		if err != nil {
			return fmt.Errorf("encoding ArtDMX packet: %w", err)
		}
		if _, err := conn.Write(packet); err != nil {
			return fmt.Errorf("sending to %s: %w", target, err)
		}
	}
	return nil
}

// animateInstance computes r's current AttributeSet at elapsed seconds
// into the demo: Pars/Washes cycle smoothly through the color wheel at
// full dimmer; Movers sweep pan/tilt and step through color/gobo wheel
// positions on a slower cycle, with shutter open and a mid zoom.
func animateInstance(r rigInstance, elapsed float64) scene.AttributeSet {
	switch r.definition.Model {
	case "RGB Par":
		red, green, blue := hsvToRGB(math.Mod(elapsed*20+phaseOffset(r.address), 360), 1, 1)
		return scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity:  1,
			fixture.CapabilityColorRed:   red,
			fixture.CapabilityColorGreen: green,
			fixture.CapabilityColorBlue:  blue,
		}}
	case "RGBW Wash":
		red, green, blue := hsvToRGB(math.Mod(elapsed*20+phaseOffset(r.address), 360), 1, 1)
		white := 0.5 + 0.5*math.Sin(elapsed*0.7+phaseOffset(r.address))
		return scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity:  1,
			fixture.CapabilityColorRed:   red,
			fixture.CapabilityColorGreen: green,
			fixture.CapabilityColorBlue:  blue,
			fixture.CapabilityColorWhite: white,
		}}
	case "Moving Head":
		pan := 0.5 + 0.45*math.Sin(elapsed*0.5+phaseOffset(r.address))
		tilt := 0.5 + 0.3*math.Sin(elapsed*0.33+phaseOffset(r.address)*1.7)
		colorWheel := math.Mod(elapsed*0.15+phaseOffset(r.address)/360, 1)
		goboWheel := math.Mod(elapsed*0.1, 1)
		return scene.AttributeSet{Values: map[fixture.CapabilityType]float64{
			fixture.CapabilityIntensity: 1,
			fixture.CapabilityPan:       pan,
			fixture.CapabilityTilt:      tilt,
			fixture.CapabilityColor:     colorWheel,
			fixture.CapabilityGobo:      goboWheel,
			fixture.CapabilityShutter:   1,
			fixture.CapabilityZoom:      0.5,
		}}
	default:
		return scene.AttributeSet{}
	}
}

// phaseOffset gives each fixture a distinct animation phase (keyed off its
// own DMX address, so the eight rig fixtures visibly desync rather than
// moving in lockstep).
func phaseOffset(address int) float64 {
	return float64(address) * 37
}

// hsvToRGB converts an HSV color (h in degrees [0,360), s and v in [0,1])
// to normalized [0,1] RGB, the standard six-sector conversion.
func hsvToRGB(h, s, v float64) (r, g, b float64) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	return r1 + m, g1 + m, b1 + m
}

// nextSeq cycles 1->255->1, never returning 0 (Art-Net sequence 0 disables
// receiver reordering; mirrors internal/artnet/packet.go's own unexported
// nextSeq -- duplicated here since this tool lives outside the package and
// packet.go deliberately keeps it unexported).
func nextSeq(prev uint8) uint8 {
	if prev == 0 || prev == 255 {
		return 1
	}
	return prev + 1
}

func decodeEmbedded(data []byte) (fixture.FixtureDefinition, error) {
	return fixture.Decode(data)
}

func resolveMode(def fixture.FixtureDefinition, name string) (fixture.Mode, error) {
	for _, mode := range def.Modes {
		if mode.Name == name {
			return mode, nil
		}
	}
	return fixture.Mode{}, fmt.Errorf("%s %s declares no mode %q", def.Manufacturer, def.Model, name)
}
