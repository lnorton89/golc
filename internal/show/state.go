// state.go implements the ShowState substrate (CONTEXT POOL-01/POOL-02/
// D-16): the working document every pool.Pool, deployment.Deployment,
// and pool.Group live inside, revisioned so 02-05's impact-plan freshness
// guard can detect a stale plan against a moved Revision. Load strictly
// decodes (internal/strictjson.DecodeStrict) and runs whole-State
// validation before trusting anything from disk (CONTEXT threat T-02-10:
// an untrusted/hand-editable working show document crosses into the
// domain model here); Save canonically encodes
// (internal/strictjson.CanonicalEncode), increments Revision, and writes
// atomically (write-temp-then-rename), reusing the shape
// internal/command/linear.go already established for resolving a
// writable path and writing a canonical plan.
package show

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/strictjson"
)

// SchemaVersion is the current State schema version Save always writes.
const SchemaVersion = 1

// State is the ShowState container: a working, JSON-persisted document
// carrying every logical Pool, Group, and concrete Deployment for one
// show (Phase 5 will later supersede this working representation with
// the durable .golc format). Revision is a monotonic counter every Save
// bumps; 02-05's impact-plan freshness guard (D-16) compares an expected
// Revision against this field to detect a stale plan.
type State struct {
	SchemaVersion int                     `json:"schema_version"`
	Revision      int                     `json:"revision"`
	Pools         []pool.Pool             `json:"pools"`
	Deployments   []deployment.Deployment `json:"deployments"`
	Groups        []pool.Group            `json:"groups"`
}

// resolvePath returns path unchanged when it is already absolute (the
// caller's own explicit choice of where to read/write); otherwise it is
// resolved relative to root. Mirrors internal/command/linear.go's
// resolveWritablePath: this package cannot import internal/command
// (internal/command imports internal/show, not the reverse), so the
// shape is duplicated rather than shared.
func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// Load strictly decodes the ShowState document at path (resolved against
// root) and runs whole-State validation. A not-yet-existing file is not
// an error: it returns a fresh, empty State at the current SchemaVersion,
// so the first "pool create"/"deployment create" against a new show
// starts cleanly. Every other failure -- a malformed document, duplicate
// pool/deployment names, more than one active deployment, or an
// out-of-range instance address -- is reported as
// GOLC_SHOW_STATE_INVALID (CONTEXT threat T-02-10: nothing from disk is
// trusted before this whole-document check passes).
func Load(root, path string) (State, error) {
	resolved := resolvePath(root, path)
	data, err := os.ReadFile(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return State{SchemaVersion: SchemaVersion}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading %s: %v", resolved, err)
	}

	var state State
	if err := strictjson.DecodeStrict(data, &state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	if err := validate(state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	return state, nil
}

// Save validates s, stamps the current SchemaVersion, increments
// Revision, canonically encodes it, and writes it atomically
// (write-temp-then-rename) to path (resolved against root). s is passed
// by value and never mutated in place: callers observe the bumped
// Revision by calling Load again, exactly like
// internal/command/linear.go's preview/apply split never lets one call
// both compute and mutate silently.
func Save(root, path string, s State) error {
	if err := validate(s); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	s.SchemaVersion = SchemaVersion
	s.Revision++

	payload, err := strictjson.CanonicalEncode(s)
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}

	resolved := resolvePath(root, path)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: creating directory for %s: %v", resolved, err)
	}
	tmp := resolved + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: writing %s: %v", tmp, err)
	}
	if err := os.Rename(tmp, resolved); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: renaming %s to %s: %v", tmp, resolved, err)
	}
	return nil
}

// validate runs every whole-State invariant Load and Save both enforce
// before trusting or persisting a State: every pool individually valid,
// unique pool names, unique deployment names, at most one active
// deployment, every instance address within the valid DMX/Art-Net range,
// unique group names, and every group's member refs resolving to an
// existing pool/pool member (WR-02).
func validate(s State) error {
	for _, p := range s.Pools {
		if err := pool.Validate(p); err != nil {
			return err
		}
	}
	if err := pool.ValidateUniqueNames(s.Pools); err != nil {
		return err
	}
	if err := deployment.ValidateUniqueNames(s.Deployments); err != nil {
		return err
	}
	if err := deployment.ValidateSingleActive(s.Deployments); err != nil {
		return err
	}
	for _, d := range s.Deployments {
		for _, instance := range d.Instances {
			if err := deployment.ValidateInstanceAddress(instance); err != nil {
				return err
			}
		}
	}
	if err := pool.ValidateUniqueGroupNames(s.Groups); err != nil {
		return err
	}
	if err := pool.ValidateGroupReferences(s.Pools, s.Groups); err != nil {
		return err
	}
	return nil
}
