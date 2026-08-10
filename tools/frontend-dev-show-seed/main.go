// Command frontend-dev-show-seed builds a real GOLC show (.golc) patched
// with a realistic, varied touring-rig layout across four Art-Net
// universes -- for exercising golc-desktop's frontend (Fixture Library,
// Patch & Pools, Desk, Diagnostics) against a show with real scale and
// real fixture variety, rather than the single-fixture-type, two-instance
// show tools/mock-golc-show-seed produces (that tool exists to match a
// companion mock Art-Net rig 1:1; this one exists purely to give the
// frontend something substantial to render).
//
// The seven fixtures it patches were pulled from the live Open Fixture
// Library via `golc-project fixture import --ofl <manufacturer>/<key>`
// (see fixtures/*.json's own provenance) rather than invented: a beam
// mover (Clay Paky Sharpy), a wash mover (Martin MAC Aura), a spot mover
// (Robe Robin 600E Spot), an LED beam mover (Robe Robin LEDBeam 150), two
// static LED pars (Chauvet DJ ColorBand T3BT, Chauvet DJ SlimPAR Q12 BT),
// and a strobe (Martin Atomic 3000). Several other real, well-known
// fixtures were tried and rejected here because they don't import
// cleanly under GOLC's current OFL normalizer -- see this repo's
// coldstart notebook (`coldstart kb search "OFL import gap"`) for what
// failed and why; that is a backend gap, not something this tool works
// around silently.
//
// GOLC's canonical channel model is NOT raw OFL's DMX channel count: a
// mode's addressed span is len(modeChannels), the fixture's post-
// normalize semantic capability count (internal/fixture/model.go's Mode
// doc comment: "channel offset i ... driven by Channels[i]"), which
// collapses OFL's raw fine/coarse and dummy channels. A 16-channel raw
// Sharpy mode normalizes to 6 real GOLC channels, for one example. This
// tool computes each universe's addressing from the DECODED modeChannels
// count (fixtureModeChannelCount below), never a hardcoded number, so it
// stays correct if a fixture file is re-imported and its channel count
// changes.
//
// Usage: go run ./tools/frontend-dev-show-seed [-fixtures fixtures] [-out frontend-dev-show.golc]
package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// Fixture library files this tool patches -- each pulled live from OFL,
// see this file's own doc comment.
const (
	sharpyFile     = "clay-paky_sharpy.json"
	macAuraFile    = "martin_mac-aura.json"
	robin600EFile  = "robe_robin-600e-spot.json"
	robinLEDBeam   = "robe_robin-ledbeam-150.json"
	colorbandT3BT  = "chauvet-dj_colorband-t3bt.yaml"
	slimparQ12BT   = "chauvet-dj_slimpar-q12-bt.json"
	atomic3000File = "martin_atomic-3000.json"
)

// placement is one (fixture file, mode, instance count) group patched
// sequentially, at consecutive addresses, into one universe.
type placement struct {
	file  string
	mode  string
	count int
}

// universeSpec is one universe's ordered list of placements. Addressing
// within a universe starts at 1 (GOLC's logical universe numbering is
// 1-indexed end to end -- see tools/mock-golc-show-seed's identical note)
// and advances by each placement's own decoded mode channel count.
type universeSpec struct {
	universe int
	label    string
	slots    []placement
}

// The rig: four universes laid out the way a real touring/festival rig is
// commonly patched by position -- downstage movers, upstage
// washes/backlight, FOH audience par wash, and blinders plus a side-wash
// overflow position. Every mode named here is verified (2026-08-10) to
// exist in its fixture's own imported definition.
var rig = []universeSpec{
	{
		universe: 1,
		label:    "Downstage Movers",
		slots: []placement{
			{file: sharpyFile, mode: "Standard", count: 8},
			{file: robin600EFile, mode: "23-channel", count: 6},
		},
	},
	{
		universe: 2,
		label:    "Upstage Washes",
		slots: []placement{
			{file: macAuraFile, mode: "Extended", count: 10},
			{file: robinLEDBeam, mode: "1 – Standard 16bit RGBW", count: 8},
		},
	},
	{
		universe: 3,
		label:    "FOH Par Wash",
		slots: []placement{
			{file: colorbandT3BT, mode: "5ch", count: 24},
			{file: slimparQ12BT, mode: "8-channel", count: 16},
		},
	},
	{
		universe: 4,
		label:    "Blinders and Side Wash",
		slots: []placement{
			{file: atomic3000File, mode: "4-channel", count: 16},
			{file: sharpyFile, mode: "Standard", count: 6},
			{file: macAuraFile, mode: "Extended", count: 6},
		},
	},
}

func main() {
	fixturesDir := flag.String("fixtures", "fixtures", "path to the fixture library directory")
	out := flag.String("out", "frontend-dev-show.golc", "path to write the show file (*.golc, gitignored)")
	flag.Parse()

	if err := run(*fixturesDir, *out); err != nil {
		log.Fatalf("frontend-dev-show-seed: %v", err)
	}
}

func run(fixturesDir, outPath string) error {
	entries, err := fixture.ListDirectory(fixturesDir)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", fixturesDir, err)
	}

	// One pool + one pool member per DISTINCT FIXTURE FILE, never per
	// (file, mode): deployment.Instance carries its own Mode independently
	// of the pool member it references (mirrors tools/mock-golc-show-seed's
	// identical structure), so the same pool member is reused across every
	// universe/mode variant of that fixture below.
	pools := map[string]*pool.Pool{}
	members := map[string]pool.PoolMember{}
	usedFiles := map[string]bool{}
	for _, spec := range rig {
		for _, slot := range spec.slots {
			usedFiles[slot.file] = true
		}
	}
	for file := range usedFiles {
		entry, err := findEntry(entries, file)
		if err != nil {
			return err
		}
		member, err := pool.NewPoolMember(entry.Identity.StableKey, entry.Identity.ContentHash)
		if err != nil {
			return fmt.Errorf("minting pool member for %s: %w", file, err)
		}
		p, err := pool.NewPool(entry.Definition.Manufacturer+" "+entry.Definition.Model, nil)
		if err != nil {
			return fmt.Errorf("creating pool for %s: %w", file, err)
		}
		p.Members = append(p.Members, member)
		pools[file] = &p
		members[file] = member
	}

	deploy, err := deployment.NewDeployment("Festival Main Stage")
	if err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}
	deploy.Active = true

	totalInstances := 0
	for _, spec := range rig {
		address := 1
		for _, slot := range spec.slots {
			channelCount, err := fixtureModeChannelCount(entries, slot.file, slot.mode)
			if err != nil {
				return fmt.Errorf("universe %d (%s): %w", spec.universe, spec.label, err)
			}
			for i := 0; i < slot.count; i++ {
				if address+channelCount-1 > 512 {
					return fmt.Errorf(
						"universe %d (%s): %s %q instance %d would occupy addresses %d-%d, past the 512-channel universe limit -- reduce this universe's fixture counts",
						spec.universe, spec.label, slot.file, slot.mode, i+1, address, address+channelCount-1,
					)
				}
				id, err := uuid.NewV7()
				if err != nil {
					return fmt.Errorf("minting instance id: %w", err)
				}
				deploy.Instances = append(deploy.Instances, deployment.Instance{
					ID:           id,
					PoolID:       pools[slot.file].ID,
					PoolMemberID: members[slot.file].ID,
					Mode:         slot.mode,
					Universe:     spec.universe,
					Address:      address,
				})
				address += channelCount
				totalInstances++
			}
		}
	}

	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolving output path: %w", err)
	}
	root := filepath.Dir(absOut)

	state, err := show.Load(root, absOut)
	if err != nil {
		return fmt.Errorf("loading fresh show state: %w", err)
	}
	state.Pools = make([]pool.Pool, 0, len(pools))
	for _, p := range pools {
		state.Pools = append(state.Pools, *p)
	}
	state.Deployments = []deployment.Deployment{deploy}

	if err := show.Save(root, absOut, state); err != nil {
		return fmt.Errorf("saving show: %w", err)
	}

	log.Printf("frontend-dev-show-seed: wrote %s (%d pools, deployment %q active with %d instances across %d universes)",
		absOut, len(state.Pools), deploy.Name, totalInstances, len(rig))
	return nil
}

func findEntry(entries []fixture.DirectoryEntry, fileName string) (fixture.DirectoryEntry, error) {
	for _, entry := range entries {
		if entry.FileName != fileName {
			continue
		}
		if entry.Err != nil {
			return fixture.DirectoryEntry{}, fmt.Errorf("decoding %s: %w", fileName, entry.Err)
		}
		return entry, nil
	}
	return fixture.DirectoryEntry{}, fmt.Errorf("%s not found in fixture library scan", fileName)
}

// fixtureModeChannelCount looks up fileName's decoded definition and
// returns modeName's real (post-normalize) channel count -- the address
// span run uses to place the NEXT instance, never a number hardcoded in
// this file (see this file's own doc comment on why raw OFL channel
// counts are the wrong number to use here).
func fixtureModeChannelCount(entries []fixture.DirectoryEntry, fileName, modeName string) (int, error) {
	entry, err := findEntry(entries, fileName)
	if err != nil {
		return 0, err
	}
	for _, mode := range entry.Definition.Modes {
		if mode.Name == modeName {
			return len(mode.Channels), nil
		}
	}
	return 0, fmt.Errorf("%s declares no mode named %q", fileName, modeName)
}
