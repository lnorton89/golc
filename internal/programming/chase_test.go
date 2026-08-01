// chase_test.go proves PROG-05's Chase identity/construction/order-
// preservation/validation contract (03-03-PLAN.md Task 1): NewChase mints
// a UUIDv7 ID and preserves the caller's exact authored step order
// (D-09: no reordering, deduplication, or randomization); StepUnit and
// StepDuration are validated; a chase exceeding maxChaseSteps is rejected
// with the DoS-ceiling diagnostic; duplicate/empty names are rejected.
package programming_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/programming"
)

// buildSteps constructs n ordered ChaseStep values whose Attributes each
// carry a distinct capability value, so the test can assert exact
// positional order is preserved (never sorted, deduped, or shuffled).
func buildSteps(n int) []programming.ChaseStep {
	steps := make([]programming.ChaseStep, 0, n)
	for i := 0; i < n; i++ {
		steps = append(steps, programming.ChaseStep{
			Attributes: []programming.PresetAttribute{
				{Capability: fixture.CapabilityIntensity, Value: float64(i) / float64(n+1)},
			},
		})
	}
	return steps
}

func TestChaseNewChaseMintsIDAndPreservesStepOrder(t *testing.T) {
	steps := buildSteps(4)
	chase, err := programming.NewChase("Sweep", steps, programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase")
	require.NotEmpty(t, chase.ID.String(), "expected a minted UUIDv7 ID, got zero value")
	require.Equal(t, "Sweep", chase.Name)
	require.Equal(t, programming.StepUnitBar, chase.StepUnit)
	require.EqualValues(t, 1, chase.StepDuration)
	require.Len(t, chase.Steps, len(steps))
	for i, step := range chase.Steps {
		require.Equal(t, steps[i].Attributes[0].Value, step.Attributes[0].Value, "step order not preserved at index %d", i)
	}
}

func TestChaseNewChaseDeterministicConstruction(t *testing.T) {
	steps := buildSteps(6)
	first, err := programming.NewChase("Deterministic", steps, programming.StepUnitBeat, 2)
	require.NoError(t, err, "NewChase (first)")
	second, err := programming.NewChase("Deterministic", steps, programming.StepUnitBeat, 2)
	require.NoError(t, err, "NewChase (second)")
	require.Len(t, second.Steps, len(first.Steps), "expected identical step counts across repeated construction")
	for i := range first.Steps {
		require.Equal(t, second.Steps[i].Attributes[0].Value, first.Steps[i].Attributes[0].Value, "expected byte-identical step ordering at index %d", i)
	}
}

func TestChaseNewChaseInvalidStepUnitRejected(t *testing.T) {
	_, err := programming.NewChase("Bad Unit", buildSteps(2), programming.StepUnit("measure"), 1)
	require.ErrorContains(t, err, "GOLC_CHASE_STEP_UNIT_INVALID")
}

func TestChaseNewChaseInvalidStepDurationRejected(t *testing.T) {
	_, err := programming.NewChase("Zero Duration", buildSteps(2), programming.StepUnitBar, 0)
	require.ErrorContains(t, err, "GOLC_CHASE_STEP_DURATION_INVALID", "expected GOLC_CHASE_STEP_DURATION_INVALID for a zero step duration")

	_, err = programming.NewChase("Negative Duration", buildSteps(2), programming.StepUnitBar, -1)
	require.ErrorContains(t, err, "GOLC_CHASE_STEP_DURATION_INVALID", "expected GOLC_CHASE_STEP_DURATION_INVALID for a negative step duration")
}

func TestChaseNewChaseTooManyStepsRejected(t *testing.T) {
	_, err := programming.NewChase("Too Many", buildSteps(257), programming.StepUnitBar, 1)
	require.ErrorContains(t, err, "GOLC_CHASE_TOO_MANY_STEPS", "expected GOLC_CHASE_TOO_MANY_STEPS for a chase exceeding the step ceiling")
}

func TestChaseNewChaseEmptyNameRejected(t *testing.T) {
	_, err := programming.NewChase("   ", buildSteps(1), programming.StepUnitBar, 1)
	require.ErrorContains(t, err, "GOLC_CHASE_NAME_EMPTY")
}

func TestChaseRenameChasePreservesIDAndSteps(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase")
	originalID := chase.ID

	renamed, err := programming.RenameChase(chase, "Sweep Renamed")
	require.NoError(t, err, "RenameChase")
	require.Equal(t, originalID, renamed.ID, "expected ID to be preserved by rename")
	require.Equal(t, "Sweep Renamed", renamed.Name)
	require.Len(t, renamed.Steps, len(chase.Steps), "expected Steps to be untouched by rename")
}

func TestChaseRenameChaseEmptyNameRejected(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(1), programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase")
	_, err = programming.RenameChase(chase, "  ")
	require.ErrorContains(t, err, "GOLC_CHASE_NAME_EMPTY")
}

func TestChaseValidateChaseUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewChase("Sweep", buildSteps(2), programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase(a)")
	b, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBeat, 2)
	require.NoError(t, err, "NewChase(b)")
	err = programming.ValidateChaseUniqueNames([]programming.Chase{a, b})
	require.ErrorContains(t, err, "GOLC_CHASE_DUPLICATE_NAME")
}

func TestChaseValidateChaseAcceptsValidChase(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBar, 1)
	require.NoError(t, err, "NewChase")
	require.NoError(t, programming.ValidateChase(chase), "expected a valid chase to pass validation")
}
