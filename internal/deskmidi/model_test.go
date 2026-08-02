// model_test.go proves deskmidi's identity and conflict-rejection contract,
// mirroring internal/operatorsurface/model_test.go's own coverage of the
// identical AddMidiMapping/RemoveMidiMapping shape.
package deskmidi_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deskmidi"
)

func TestAddMappingMintsIdentityAndRejectsConflict(t *testing.T) {
	first, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 74,
		InstanceID: "instance-a", Capability: "intensity",
	})
	require.NoError(t, err, "AddMapping")
	require.Len(t, first, 1)
	require.NotEqual(t, uuid.UUID{}, first[0].ID, "expected AddMapping to mint a non-zero ID")

	_, err = deskmidi.AddMapping(first, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 74,
		InstanceID: "instance-b", Capability: "pan",
	})
	require.ErrorContains(t, err, "GOLC_DESKMIDI_MAPPING_CONFLICT", "expected a colliding (channel, kind, number) tuple to be rejected")

	// A Note sharing the same channel/number as a ControlChange is a
	// distinct control -- Kind is part of the conflict key, same as
	// operatorsurface's own AddMidiMapping/midi.ProposeMapping.
	second, err := deskmidi.AddMapping(first, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.Note, Number: 74,
		InstanceID: "instance-b", Capability: "pan",
	})
	require.NoError(t, err, "expected a Note at the same channel/number as an existing CC to be allowed")
	require.Len(t, second, 2)
}

func TestRemoveMappingIdempotentIfAbsent(t *testing.T) {
	mapped, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 2, Kind: deskmidi.Note, Number: 36,
		InstanceID: "instance-a", Capability: "strobe",
	})
	require.NoError(t, err, "AddMapping")
	id := mapped[0].ID

	removed := deskmidi.RemoveMapping(mapped, id)
	require.Empty(t, removed, "expected mapping to be removed")

	noop := deskmidi.RemoveMapping(removed, id)
	require.Empty(t, noop, "expected removing an absent ID to be a no-op, not an error")

	noop2 := deskmidi.RemoveMapping(removed, uuid.Nil)
	require.Empty(t, noop2, "expected removing an unrelated absent ID to be a no-op")
}

func TestAddMappingNeverAliasesCallerSlice(t *testing.T) {
	base, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 1,
		InstanceID: "instance-a", Capability: "intensity",
	})
	require.NoError(t, err)

	extended, err := deskmidi.AddMapping(base, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 2,
		InstanceID: "instance-a", Capability: "pan",
	})
	require.NoError(t, err)
	require.Len(t, base, 1, "expected AddMapping to never mutate the caller's own slice")
	require.Len(t, extended, 2)
}
