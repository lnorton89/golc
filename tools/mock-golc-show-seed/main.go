// Command mock-golc-show-seed builds a real GOLC show (.golc) with an
// active deployment patching fixtures/'s library across two Art-Net
// universes, matching bonzupii/mock-golc's rig.toml one-for-one (same
// universes, addresses, and modes). Unlike tools/mock-golc-demo -- which
// sends raw playback.Frame/artnet.Encode bytes with no show file at all,
// by design (see its own doc comment) -- Perform > Desk in golc-desktop
// reads its fixture/universe layout from a real patched show via
// listPatch()/listLocalFixtures(), not from Art-Net traffic. This tool
// exists to produce that show so Desk has something real to display when
// GOLC's own "artnet serve" daemon (not mock-golc-demo) is pointed at a
// running mock-golc instance.
//
// Usage: go run ./tools/mock-golc-show-seed [-fixtures fixtures] [-out mock-golc-test-show.golc]
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

// chauvetFile/briteqFile are the two fixtures/ library entries this rig
// patches, matched by exact file name against fixture.ListDirectory's
// scan -- the same scan svc_fixturelibrary.go's ListLocal (and so Desk's
// listLocalFixtures()) uses, so this tool's StableKey/ContentHash always
// agree with what the running app resolves for the same files.
const (
	chauvetFile = "chauvet-dj_colorband-t3bt.yaml"
	briteqFile  = "briteq_bt-coloray-120r.json"
)

// plannedInstance is one Instance this tool mints, keyed by which library
// file's pool member it belongs to. Universe/address/mode mirror
// rig.toml's [[universes.fixtures]] entries in bonzupii/mock-golc
// (chauvet_t3bt_3ch/briteq_120r_4ch/briteq_120r_6ch profiles) exactly.
type plannedInstance struct {
	file     string
	mode     string
	universe int
	address  int
}

// Universes start at 1, not 0: deployment.Instance's own validation
// (GOLC_DEPLOYMENT_ADDRESS_OUT_OF_RANGE) rejects universe 0 -- GOLC's
// logical universe numbering is 1-indexed end to end, including on the
// Art-Net wire (internal/artnet.Encode keys buffers by Instance.Universe
// directly, with no -1 adjustment before EncodeArtDMX's port-address).
var plan = []plannedInstance{
	{file: chauvetFile, mode: "3ch", universe: 1, address: 1},
	{file: chauvetFile, mode: "3ch", universe: 1, address: 4},
	{file: briteqFile, mode: "RGB + Dim/Strb", universe: 2, address: 1},
	{file: briteqFile, mode: "RGBW + Dim + Strb", universe: 2, address: 5},
}

func main() {
	fixturesDir := flag.String("fixtures", "fixtures", "path to the fixture library directory")
	out := flag.String("out", "mock-golc-test-show.golc", "path to write the show file (*.golc, gitignored)")
	flag.Parse()

	if err := run(*fixturesDir, *out); err != nil {
		log.Fatalf("mock-golc-show-seed: %v", err)
	}
}

func run(fixturesDir, outPath string) error {
	entries, err := fixture.ListDirectory(fixturesDir)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", fixturesDir, err)
	}

	pools := map[string]*pool.Pool{}
	members := map[string]pool.PoolMember{}
	for _, file := range []string{chauvetFile, briteqFile} {
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

	deploy, err := deployment.NewDeployment("Mock-golc Test Rig")
	if err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}
	deploy.Active = true

	for _, inst := range plan {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("minting instance id: %w", err)
		}
		deploy.Instances = append(deploy.Instances, deployment.Instance{
			ID:           id,
			PoolID:       pools[inst.file].ID,
			PoolMemberID: members[inst.file].ID,
			Mode:         inst.mode,
			Universe:     inst.universe,
			Address:      inst.address,
		})
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
	state.Pools = []pool.Pool{*pools[chauvetFile], *pools[briteqFile]}
	state.Deployments = []deployment.Deployment{deploy}

	if err := show.Save(root, absOut, state); err != nil {
		return fmt.Errorf("saving show: %w", err)
	}

	log.Printf("mock-golc-show-seed: wrote %s (%d pools, deployment %q active with %d instances across universes 1-2)",
		absOut, len(state.Pools), deploy.Name, len(deploy.Instances))
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
