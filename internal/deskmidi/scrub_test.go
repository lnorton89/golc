package deskmidi_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/deskmidi"
)

func TestScrubDanglingRemovesOnlyBrokenReferences(t *testing.T) {
	live, err := uuid.NewV7()
	require.NoError(t, err)
	deleted, err := uuid.NewV7()
	require.NoError(t, err)

	mappings, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 1, InstanceID: live.String(), Capability: "intensity",
	})
	require.NoError(t, err)
	mappings, err = deskmidi.AddMapping(mappings, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 2, InstanceID: deleted.String(), Capability: "pan",
	})
	require.NoError(t, err)

	scrubbed := deskmidi.ScrubDangling(mappings, []deployment.Deployment{mustDeployment(t, live)})
	require.Len(t, scrubbed, 1, "expected only the dangling mapping to be scrubbed")
	require.Equal(t, live.String(), scrubbed[0].InstanceID)

	err = deskmidi.Validate(scrubbed, []deployment.Deployment{mustDeployment(t, live)})
	require.NoError(t, err, "expected the scrubbed result to validate cleanly")
}

func TestScrubDanglingNoopWhenNothingBroken(t *testing.T) {
	live, err := uuid.NewV7()
	require.NoError(t, err)
	mappings, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.Note, Number: 1, InstanceID: live.String(), Capability: "strobe",
	})
	require.NoError(t, err)

	scrubbed := deskmidi.ScrubDangling(mappings, []deployment.Deployment{mustDeployment(t, live)})
	require.Len(t, scrubbed, 1)
}
