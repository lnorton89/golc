package deskmidi_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/deskmidi"
)

func mustDeployment(t *testing.T, instanceIDs ...uuid.UUID) deployment.Deployment {
	t.Helper()
	d, err := deployment.NewDeployment("Test Deployment")
	require.NoError(t, err, "NewDeployment")
	for _, id := range instanceIDs {
		d.Instances = append(d.Instances, deployment.Instance{ID: id})
	}
	return d
}

func TestValidateAcceptsMappingsReferencingKnownInstances(t *testing.T) {
	instanceID, err := uuid.NewV7()
	require.NoError(t, err)

	mappings, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 7,
		InstanceID: instanceID.String(), Capability: "intensity",
	})
	require.NoError(t, err)

	err = deskmidi.Validate(mappings, []deployment.Deployment{mustDeployment(t, instanceID)})
	require.NoError(t, err, "expected a mapping referencing a known instance to validate cleanly")
}

func TestValidateRejectsDanglingInstanceReference(t *testing.T) {
	instanceID, err := uuid.NewV7()
	require.NoError(t, err)

	mappings, err := deskmidi.AddMapping(nil, deskmidi.Mapping{
		Channel: 1, Kind: deskmidi.ControlChange, Number: 7,
		InstanceID: instanceID.String(), Capability: "intensity",
	})
	require.NoError(t, err)

	err = deskmidi.Validate(mappings, nil)
	require.ErrorContains(t, err, "GOLC_DESKMIDI_DANGLING_REFERENCE")
}

func TestValidateRejectsDuplicateMappingKeys(t *testing.T) {
	instanceID, err := uuid.NewV7()
	require.NoError(t, err)

	m1 := deskmidi.Mapping{ID: uuid.Must(uuid.NewV7()), Channel: 1, Kind: deskmidi.ControlChange, Number: 7, InstanceID: instanceID.String(), Capability: "intensity"}
	m2 := deskmidi.Mapping{ID: uuid.Must(uuid.NewV7()), Channel: 1, Kind: deskmidi.ControlChange, Number: 7, InstanceID: instanceID.String(), Capability: "pan"}

	err = deskmidi.Validate([]deskmidi.Mapping{m1, m2}, []deployment.Deployment{mustDeployment(t, instanceID)})
	require.ErrorContains(t, err, "GOLC_DESKMIDI_DUPLICATE_MAPPING")
}

func TestValidateRejectsInvalidInstanceID(t *testing.T) {
	m := deskmidi.Mapping{ID: uuid.Must(uuid.NewV7()), Channel: 1, Kind: deskmidi.Note, Number: 1, InstanceID: "not-a-uuid", Capability: "intensity"}
	err := deskmidi.Validate([]deskmidi.Mapping{m}, nil)
	require.ErrorContains(t, err, "GOLC_DESKMIDI_INSTANCE_ID_INVALID")
}
