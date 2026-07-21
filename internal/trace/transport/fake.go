// fake.go implements a credential-free, in-memory Transport (T-01-SC): it
// never performs network, SDK, or credential access. It exists so
// reconciliation preview/archive/unlink logic is provable end to end
// without Linear ever being reachable, and so offline command routes can
// exercise the exact same Transport contract a real adapter will later
// satisfy.
package transport

import (
	"fmt"
	"os"

	"github.com/lnorton89/golc/internal/strictjson"
)

// Fake is an in-memory Transport that always returns a fixed, caller-
// supplied Snapshot and records every mutation it is asked to apply.
type Fake struct {
	snapshot Snapshot
	applied  []Mutation
}

// NewFake returns a Fake transport that reports snapshot from every
// CaptureSnapshot call.
func NewFake(snapshot Snapshot) *Fake {
	return &Fake{snapshot: snapshot}
}

// LoadFakeSnapshot strictly decodes one committed JSON snapshot fixture
// into a Fake transport. Duplicate object keys and unknown fields are
// rejected before anything is trusted (internal/strictjson).
func LoadFakeSnapshot(path string) (*Fake, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("GOLC_TRANSPORT_FAKE_LOAD: %s: %v", path, err)
	}
	var snapshot Snapshot
	if err := strictjson.DecodeStrict(data, &snapshot); err != nil {
		return nil, fmt.Errorf("GOLC_TRANSPORT_FAKE_LOAD: %s: %v", path, err)
	}
	return NewFake(snapshot), nil
}

// CaptureSnapshot returns the fixed fake snapshot. It never fails on its
// own; a non-complete Status is the fake's way of exercising the same
// diagnostic paths a real transport would report (CONTEXT D-21).
func (f *Fake) CaptureSnapshot() (Snapshot, error) {
	return f.snapshot, nil
}

// Apply records mutation and returns it unchanged. The fake never talks
// to a real remote object; it exists to prove that only explicit,
// already-reviewed archive/unlink calls ever reach a Transport's Apply
// method (CONTEXT D-15).
func (f *Fake) Apply(mutation Mutation) (Mutation, error) {
	f.applied = append(f.applied, mutation)
	return mutation, nil
}

// Applied returns every mutation recorded so far, in call order.
func (f *Fake) Applied() []Mutation {
	return append([]Mutation(nil), f.applied...)
}

var _ Transport = (*Fake)(nil)
