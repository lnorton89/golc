// store_property_test.go property-tests the SQLite-backed Save/Load round
// trip (CONTEXT SHOW-01/D-01/D-02/D-03) across arbitrary show shapes,
// generalizing store_test.go's fixed single-shape examples
// (TestShowStoreRoundTrip, TestShowStoreSaveIsIdempotent): for any number
// of pools, any number of members per pool, and any finite positive Tempo,
// Save then Load always returns Pools/Tempo byte-identical to what was
// saved and bumps Revision by exactly 1 -- regardless of how many entities
// the store's canonical-encode/checksum/SQLite-blob round trip actually
// has to carry.
//
// This is package show_test (external, black-box) since it only needs the
// exported Save/Load/State/Tempo surface, unlike store_test.go's internal
// package show (which also calls openStore directly to inspect on-disk
// revision). Deployments/groups/scenes/programming content is
// deliberately out of scope here -- see internal/pool's
// membership_property_test.go for the pool-mutation pipeline itself; this
// property isolates the SQLite persistence layer's own round-trip
// guarantee from the rest of the domain's referential-integrity rules.
package show_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/show"
)

// buildPoolsWithMembers builds one uniquely-named pool per entry in
// memberCounts, each carrying that many members with distinct fixture
// stable keys.
func buildPoolsWithMembers(t *testing.T, memberCounts []int) []pool.Pool {
	t.Helper()
	pools := make([]pool.Pool, 0, len(memberCounts))
	for i, memberCount := range memberCounts {
		p, err := pool.NewPool(fmt.Sprintf("Property Pool %d", i), nil)
		require.NoError(t, err, "NewPool")
		for m := 0; m < memberCount; m++ {
			member, err := pool.NewPoolMember(fmt.Sprintf("acme/fixture-%d-%d", i, m), "sha256:seed")
			require.NoError(t, err, "NewPoolMember")
			p.Members = append(p.Members, member)
		}
		pools = append(pools, p)
	}
	return pools
}

// assertPoolsAndTempoEqual compares only the fields this property actually
// varies: Pools and Tempo. It deliberately does not compare the whole
// State (unlike store_test.go's assertDomainEqual, which can because its
// fixed fixture always populates every field with a concrete value):
// Save() unconditionally runs every generated State's Scenes through
// scene.ScrubDanglingSelections (and DeskMidiMappings through
// deskmidi.ScrubDangling), which normalizes a nil slice to a non-nil empty
// one -- a real, documented, but unrelated side effect of fields this
// property never sets. Asserting whole-struct equality here would fail on
// that unrelated normalization instead of testing what this property is
// actually about.
func assertPoolsAndTempoEqual(t *testing.T, want, got show.State) {
	t.Helper()
	require.Equal(t, want.Pools, got.Pools, "pools did not round-trip byte-identically")
	require.Equal(t, want.Tempo, got.Tempo, "tempo did not round-trip byte-identically")
}

// TestShowSaveLoadRoundTripProperty generalizes TestShowStoreRoundTrip's
// fixed one-pool-one-member example: for an arbitrary number of pools
// (0-4) each with an arbitrary number of members (0-4) and an arbitrary
// finite positive Tempo.BPM, Save then Load always returns
// domain-identical content and bumps Revision by exactly 1. Each check
// gets its own file within one shared temp root so 100 checks stay cheap
// (no per-check TempDir allocation) while remaining fully isolated from
// each other.
func TestShowSaveLoadRoundTripProperty(t *testing.T) {
	root := t.TempDir()
	fileCounter := 0

	rapid.Check(t, func(rt *rapid.T) {
		fileCounter++
		path := fmt.Sprintf("property-%d.golc", fileCounter)

		poolCount := rapid.IntRange(0, 4).Draw(rt, "poolCount")
		memberCounts := make([]int, poolCount)
		for i := range memberCounts {
			memberCounts[i] = rapid.IntRange(0, 4).Draw(rt, fmt.Sprintf("memberCount%d", i))
		}
		bpm := rapid.Float64Range(1, 300).Draw(rt, "bpm")

		state := show.State{
			Pools: buildPoolsWithMembers(t, memberCounts),
			Tempo: show.Tempo{BPM: bpm},
		}

		require.NoError(t, show.Save(root, path, state), "Save")
		loaded, err := show.Load(root, path)
		require.NoError(t, err, "Load")
		require.Equal(t, state.Revision+1, loaded.Revision, "expected Revision to bump by exactly 1")
		assertPoolsAndTempoEqual(t, state, loaded)
	})
}

// TestShowSaveIsIdempotentAcrossShapesProperty generalizes
// TestShowStoreSaveIsIdempotent's fixed example: saving an arbitrary
// loaded show back to the same path, unchanged, always advances Revision
// by exactly 1 with no entity gained or lost, regardless of how many
// pools/members the show carries.
func TestShowSaveIsIdempotentAcrossShapesProperty(t *testing.T) {
	root := t.TempDir()
	fileCounter := 0

	rapid.Check(t, func(rt *rapid.T) {
		fileCounter++
		path := fmt.Sprintf("idempotent-%d.golc", fileCounter)

		poolCount := rapid.IntRange(0, 4).Draw(rt, "poolCount")
		memberCounts := make([]int, poolCount)
		for i := range memberCounts {
			memberCounts[i] = rapid.IntRange(0, 4).Draw(rt, fmt.Sprintf("memberCount%d", i))
		}

		state := show.State{Pools: buildPoolsWithMembers(t, memberCounts), Tempo: show.Tempo{BPM: 120}}

		require.NoError(t, show.Save(root, path, state), "first Save")
		first, err := show.Load(root, path)
		require.NoError(t, err, "Load after first Save")

		require.NoError(t, show.Save(root, path, first), "second Save")
		second, err := show.Load(root, path)
		require.NoError(t, err, "Load after second Save")

		require.Equal(t, first.Revision+1, second.Revision, "expected Revision to advance by exactly 1")
		require.Len(t, second.Pools, len(first.Pools), "pool count changed across an idempotent save")
		for i, p := range first.Pools {
			require.Len(t, second.Pools[i].Members, len(p.Members), "pool %d member count changed across an idempotent save", i)
		}
		assertPoolsAndTempoEqual(t, first, second)
	})
}
