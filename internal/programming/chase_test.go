// chase_test.go proves PROG-05's Chase identity/construction/order-
// preservation/validation contract (03-03-PLAN.md Task 1): NewChase mints
// a UUIDv7 ID and preserves the caller's exact authored step order
// (D-09: no reordering, deduplication, or randomization); StepUnit and
// StepDuration are validated; a chase exceeding maxChaseSteps is rejected
// with the DoS-ceiling diagnostic; duplicate/empty names are rejected.
package programming_test

import (
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	if chase.ID.String() == "" {
		t.Fatalf("expected a minted UUIDv7 ID, got zero value")
	}
	if chase.Name != "Sweep" || chase.StepUnit != programming.StepUnitBar || chase.StepDuration != 1 {
		t.Fatalf("unexpected chase: %+v", chase)
	}
	if len(chase.Steps) != len(steps) {
		t.Fatalf("expected %d steps, got %d", len(steps), len(chase.Steps))
	}
	for i, step := range chase.Steps {
		if step.Attributes[0].Value != steps[i].Attributes[0].Value {
			t.Fatalf("step order not preserved at index %d: expected %v, got %v", i, steps[i].Attributes[0].Value, step.Attributes[0].Value)
		}
	}
}

func TestChaseNewChaseDeterministicConstruction(t *testing.T) {
	steps := buildSteps(6)
	first, err := programming.NewChase("Deterministic", steps, programming.StepUnitBeat, 2)
	if err != nil {
		t.Fatalf("NewChase (first): %v", err)
	}
	second, err := programming.NewChase("Deterministic", steps, programming.StepUnitBeat, 2)
	if err != nil {
		t.Fatalf("NewChase (second): %v", err)
	}
	if len(first.Steps) != len(second.Steps) {
		t.Fatalf("expected identical step counts across repeated construction, got %d vs %d", len(first.Steps), len(second.Steps))
	}
	for i := range first.Steps {
		if first.Steps[i].Attributes[0].Value != second.Steps[i].Attributes[0].Value {
			t.Fatalf("expected byte-identical step ordering at index %d, got %v vs %v",
				i, first.Steps[i].Attributes[0].Value, second.Steps[i].Attributes[0].Value)
		}
	}
}

func TestChaseNewChaseInvalidStepUnitRejected(t *testing.T) {
	_, err := programming.NewChase("Bad Unit", buildSteps(2), programming.StepUnit("measure"), 1)
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_STEP_UNIT_INVALID") {
		t.Fatalf("expected GOLC_CHASE_STEP_UNIT_INVALID, got %v", err)
	}
}

func TestChaseNewChaseInvalidStepDurationRejected(t *testing.T) {
	_, err := programming.NewChase("Zero Duration", buildSteps(2), programming.StepUnitBar, 0)
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_STEP_DURATION_INVALID") {
		t.Fatalf("expected GOLC_CHASE_STEP_DURATION_INVALID for a zero step duration, got %v", err)
	}

	_, err = programming.NewChase("Negative Duration", buildSteps(2), programming.StepUnitBar, -1)
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_STEP_DURATION_INVALID") {
		t.Fatalf("expected GOLC_CHASE_STEP_DURATION_INVALID for a negative step duration, got %v", err)
	}
}

func TestChaseNewChaseTooManyStepsRejected(t *testing.T) {
	_, err := programming.NewChase("Too Many", buildSteps(257), programming.StepUnitBar, 1)
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_TOO_MANY_STEPS") {
		t.Fatalf("expected GOLC_CHASE_TOO_MANY_STEPS for a chase exceeding the step ceiling, got %v", err)
	}
}

func TestChaseNewChaseEmptyNameRejected(t *testing.T) {
	_, err := programming.NewChase("   ", buildSteps(1), programming.StepUnitBar, 1)
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_NAME_EMPTY") {
		t.Fatalf("expected GOLC_CHASE_NAME_EMPTY, got %v", err)
	}
}

func TestChaseRenameChasePreservesIDAndSteps(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	originalID := chase.ID

	renamed, err := programming.RenameChase(chase, "Sweep Renamed")
	if err != nil {
		t.Fatalf("RenameChase: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to be preserved by rename, got original=%s renamed=%s", originalID, renamed.ID)
	}
	if renamed.Name != "Sweep Renamed" {
		t.Fatalf("expected renamed Name %q, got %q", "Sweep Renamed", renamed.Name)
	}
	if len(renamed.Steps) != len(chase.Steps) {
		t.Fatalf("expected Steps to be untouched by rename, got %+v", renamed.Steps)
	}
}

func TestChaseRenameChaseEmptyNameRejected(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(1), programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	_, err = programming.RenameChase(chase, "  ")
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_NAME_EMPTY") {
		t.Fatalf("expected GOLC_CHASE_NAME_EMPTY, got %v", err)
	}
}

func TestChaseValidateChaseUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewChase("Sweep", buildSteps(2), programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase(a): %v", err)
	}
	b, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBeat, 2)
	if err != nil {
		t.Fatalf("NewChase(b): %v", err)
	}
	err = programming.ValidateChaseUniqueNames([]programming.Chase{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_CHASE_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_CHASE_DUPLICATE_NAME, got %v", err)
	}
}

func TestChaseValidateChaseAcceptsValidChase(t *testing.T) {
	chase, err := programming.NewChase("Sweep", buildSteps(3), programming.StepUnitBar, 1)
	if err != nil {
		t.Fatalf("NewChase: %v", err)
	}
	if err := programming.ValidateChase(chase); err != nil {
		t.Fatalf("expected a valid chase to pass validation, got %v", err)
	}
}
