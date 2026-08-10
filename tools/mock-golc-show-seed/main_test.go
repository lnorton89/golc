package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/show"
)

func TestFindEntry(t *testing.T) {
	entries := []fixture.DirectoryEntry{
		{FileName: "good.yaml", Definition: fixture.FixtureDefinition{Model: "Good"}},
		{FileName: "broken.yaml", Err: assert.AnError},
	}

	t.Run("known file resolves", func(t *testing.T) {
		entry, err := findEntry(entries, "good.yaml")
		require.NoError(t, err)
		assert.Equal(t, "Good", entry.Definition.Model)
	})

	t.Run("a decode error on the matched entry is surfaced, not silently skipped", func(t *testing.T) {
		_, err := findEntry(entries, "broken.yaml")
		require.Error(t, err)
		assert.ErrorContains(t, err, "broken.yaml")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("missing file names the scan miss explicitly", func(t *testing.T) {
		_, err := findEntry(entries, "absent.yaml")
		require.Error(t, err)
		assert.ErrorContains(t, err, "absent.yaml")
		assert.ErrorContains(t, err, "not found")
	})
}

// syntheticChauvetYAML/syntheticBriteqJSON are minimal, self-contained
// fixture.FixtureDefinition documents -- deliberately NOT the real
// fixtures/chauvet-dj_colorband-t3bt.yaml/briteq_bt-coloray-120r.json
// content (the real briteq file is not currently present in fixtures/ at
// all; a run() test against the real directory would fail for that
// unrelated, pre-existing reason). These exist solely to prove run()'s
// own orchestration -- pool/deployment construction and the show save/
// load round trip -- independent of the real fixture library's current
// contents.
const syntheticChauvetYAML = `
schema_version: 1
manufacturer: "Test"
model: "Chauvet Stand-in"
capabilities:
  - type: intensity
    range: [0, 1]
modes:
  - name: "3ch"
    channels:
      - type: intensity
        occurrence: 0
`

// .json library entries are import envelopes (fixture.DecodeEnvelope), not
// a bare FixtureDefinition -- unlike the .yaml path above, they require a
// wrapping "definition" key (provenance is optional and omitted here).
const syntheticBriteqJSON = `{
  "definition": {
    "schema_version": 1,
    "manufacturer": "Test",
    "model": "Briteq Stand-in",
    "capabilities": [{"type": "intensity", "range": [0, 1]}],
    "modes": [{"name": "RGB + Dim/Strb", "channels": [{"type": "intensity", "occurrence": 0}]}]
  }
}`

func TestRunSeedsShowWithBothPoolsAndFourInstancesAcrossTwoUniverses(t *testing.T) {
	fixturesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fixturesDir, chauvetFile), []byte(syntheticChauvetYAML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fixturesDir, briteqFile), []byte(syntheticBriteqJSON), 0o644))

	outPath := filepath.Join(t.TempDir(), "seeded.golc")
	require.NoError(t, run(fixturesDir, outPath))

	state, err := show.Load(filepath.Dir(outPath), outPath)
	require.NoError(t, err, "the show run() wrote must itself load cleanly")

	require.Len(t, state.Pools, 2, "expected one pool per library file")
	require.Len(t, state.Deployments, 1)
	deploy := state.Deployments[0]
	assert.True(t, deploy.Active, "the seeded deployment must be marked active so Desk has something to display")
	require.Len(t, deploy.Instances, len(plan), "expected exactly plan's instance count")

	universes := map[int]int{}
	for _, instance := range deploy.Instances {
		universes[instance.Universe]++
		assert.NotEmpty(t, instance.PoolID, "every instance must resolve to a real pool")
		assert.NotEmpty(t, instance.PoolMemberID, "every instance must resolve to a real pool member")
	}
	assert.Equal(t, map[int]int{1: 2, 2: 2}, universes, "plan patches 2 chauvet instances on universe 1 and 2 briteq instances on universe 2")
}

func TestRunErrorsWhenALibraryFileIsMissing(t *testing.T) {
	fixturesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fixturesDir, chauvetFile), []byte(syntheticChauvetYAML), 0o644))
	// briteqFile deliberately absent.

	err := run(fixturesDir, filepath.Join(t.TempDir(), "seeded.golc"))
	require.Error(t, err)
	assert.ErrorContains(t, err, briteqFile)
}
