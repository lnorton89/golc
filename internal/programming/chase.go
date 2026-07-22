// chase.go declares the Chase domain model (CONTEXT PROG-05/D-09/D-10): a
// reusable chase of ordered, tempo-relative steps a show author authors
// once and reuses across scenes. Chase copies internal/pool/model.go's
// identity/construction/rename/unique-name shape verbatim (03-PATTERNS.md).
// NewChase never sorts, dedupes, or shuffles the caller's authored Steps
// (D-09: no randomization in v1) -- step order is preserved exactly as
// authored, and constructing the same chase twice from identical input
// always yields byte-identical Steps ordering.
package programming

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// StepUnit names the tempo-relative unit a Chase's StepDuration is
// expressed in (PROG-05, D-10: chase step advancement is driven by the
// same global BPM+bar-position clock as scene looping). Open Question 3
// from 03-RESEARCH.md is resolved by supporting both units explicitly
// rather than picking one.
type StepUnit string

// The two v1 step units.
const (
	StepUnitBar  StepUnit = "bar"
	StepUnitBeat StepUnit = "beat"
)

// maxChaseSteps bounds a Chase's Steps length (DoS ceiling, mirrors
// internal/deployment's maxUniverseSearch precedent, CONTEXT threat
// T-03-02): a pathologically large chase is rejected with
// GOLC_CHASE_TOO_MANY_STEPS rather than allowed to grow unbounded.
const maxChaseSteps = 256

// ChaseStep is one ordered look inside a Chase: an attribute-snapshot look
// reusing PresetAttribute's shape, plus an optional Selection scoping which
// fixtures this step targets narrower than whatever selection the
// containing chase layer already resolved (CONTEXT D-03). A nil Selection
// means "no step-specific scoping" -- the containing layer's own selection
// applies unchanged.
type ChaseStep struct {
	Attributes []PresetAttribute `json:"attributes,omitempty"`
	Selection  *Selection        `json:"selection,omitempty"`
}

// Chase is a reusable chase with ordered, tempo-relative steps (PROG-05).
// Identity is a durable UUIDv7 minted once at creation -- never derived
// from Name, and never re-minted by RenameChase. Steps preserves the
// caller's authored order exactly: never sorted, deduped, or shuffled
// (D-09).
type Chase struct {
	ID           uuid.UUID   `json:"id"`
	Name         string      `json:"name"`
	Steps        []ChaseStep `json:"steps,omitempty"`
	StepUnit     StepUnit    `json:"step_unit"`
	StepDuration float64     `json:"step_duration"`
}

// validateStepUnit rejects any StepUnit outside the two declared values
// (GOLC_CHASE_STEP_UNIT_INVALID).
func validateStepUnit(unit StepUnit) error {
	if unit != StepUnitBar && unit != StepUnitBeat {
		return fmt.Errorf("GOLC_CHASE_STEP_UNIT_INVALID: %q is not a supported step unit", unit)
	}
	return nil
}

// ValidateChase re-checks every invariant a hand-edited or otherwise
// untrusted Chase must satisfy before it is trusted: Name is non-empty,
// StepUnit is one of the two declared units
// (GOLC_CHASE_STEP_UNIT_INVALID otherwise), StepDuration is strictly
// positive (GOLC_CHASE_STEP_DURATION_INVALID otherwise), and Steps does not
// exceed the maxChaseSteps DoS ceiling (GOLC_CHASE_TOO_MANY_STEPS
// otherwise). This never reorders, dedupes, or mutates c.Steps -- it only
// inspects them.
func ValidateChase(c Chase) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("GOLC_CHASE_NAME_EMPTY: chase %s declares an empty name", c.ID)
	}
	if err := validateStepUnit(c.StepUnit); err != nil {
		return err
	}
	if c.StepDuration <= 0 {
		return fmt.Errorf("GOLC_CHASE_STEP_DURATION_INVALID: step duration %v must be greater than zero", c.StepDuration)
	}
	if len(c.Steps) > maxChaseSteps {
		return fmt.Errorf("GOLC_CHASE_TOO_MANY_STEPS: chase %q declares %d steps, exceeding the maximum of %d", c.Name, len(c.Steps), maxChaseSteps)
	}
	return nil
}

// NewChase mints a fresh UUIDv7-identified Chase carrying steps in exactly
// the order the caller provided them (PROG-05, D-09): steps is never
// sorted, deduped, or shuffled. An invalid StepUnit, non-positive
// StepDuration, an empty name, or a steps count beyond maxChaseSteps is
// rejected before an ID is ever minted.
func NewChase(name string, steps []ChaseStep, stepUnit StepUnit, stepDuration float64) (Chase, error) {
	chase := Chase{Name: name, Steps: steps, StepUnit: stepUnit, StepDuration: stepDuration}
	if err := ValidateChase(chase); err != nil {
		return Chase{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Chase{}, fmt.Errorf("GOLC_CHASE_ID_MINT_FAILED: %v", err)
	}
	chase.ID = id
	return chase, nil
}

// RenameChase returns c with Name replaced by newName; ID is never
// re-minted (identity is rename-stable), and Steps is left untouched.
func RenameChase(c Chase, newName string) (Chase, error) {
	if strings.TrimSpace(newName) == "" {
		return Chase{}, fmt.Errorf("GOLC_CHASE_NAME_EMPTY: chase name must not be empty")
	}
	c.Name = newName
	return c, nil
}

// ValidateChaseUniqueNames rejects any two chases in chases sharing the
// same Name: a duplicate name is always rejected with a diagnostic, never
// silently permitted (PROG-05 idempotency).
func ValidateChaseUniqueNames(chases []Chase) error {
	seen := make(map[string]bool, len(chases))
	for _, c := range chases {
		if seen[c.Name] {
			return fmt.Errorf("GOLC_CHASE_DUPLICATE_NAME: a chase named %q already exists", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}
