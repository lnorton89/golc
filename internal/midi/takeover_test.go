package midi

import "testing"

// TestTakeoverRisingCross covers a physical value rising through
// AppValue: the control must stay unarmed (and controlValue must stay
// the fixed ghost/target AppValue, D-10) until the message that reaches
// or crosses AppValue, then arm and track physical from then on.
func TestTakeoverRisingCross(t *testing.T) {
	st := NewTakeoverState(0.5)

	if armed, cv := st.Update(0.2); armed || cv != 0.5 {
		t.Fatalf("Update(0.2): got armed=%v controlValue=%v, want armed=false controlValue=0.5", armed, cv)
	}
	if armed, cv := st.Update(0.4); armed || cv != 0.5 {
		t.Fatalf("Update(0.4): got armed=%v controlValue=%v, want armed=false controlValue=0.5", armed, cv)
	}
	armed, cv := st.Update(0.6)
	if !armed {
		t.Fatalf("Update(0.6): got armed=false, want armed=true (0.4 -> 0.6 crosses appValue 0.5)")
	}
	if cv != 0.6 {
		t.Fatalf("Update(0.6): got controlValue=%v, want 0.6 (armed control tracks physical)", cv)
	}
	if !st.Armed {
		t.Fatalf("st.Armed = false after crossing, want true")
	}
}

// TestTakeoverFallingCross covers a physical value falling through
// AppValue: same crossing rule, opposite direction.
func TestTakeoverFallingCross(t *testing.T) {
	st := NewTakeoverState(0.5)

	if armed, cv := st.Update(0.9); armed || cv != 0.5 {
		t.Fatalf("Update(0.9): got armed=%v controlValue=%v, want armed=false controlValue=0.5", armed, cv)
	}
	if armed, cv := st.Update(0.6); armed || cv != 0.5 {
		t.Fatalf("Update(0.6): got armed=%v controlValue=%v, want armed=false controlValue=0.5", armed, cv)
	}
	armed, cv := st.Update(0.4)
	if !armed {
		t.Fatalf("Update(0.4): got armed=false, want armed=true (0.6 -> 0.4 crosses appValue 0.5)")
	}
	if cv != 0.4 {
		t.Fatalf("Update(0.4): got controlValue=%v, want 0.4 (armed control tracks physical)", cv)
	}
}

// TestTakeoverNeverCrosses covers a physical value that hovers on one
// side of AppValue without ever reaching or crossing it: the control
// must never arm, controlValue must stay the fixed AppValue ghost/target
// marker, and LastPhysical must still update on every call (D-09: the
// live slider follows the physical position even while not armed).
func TestTakeoverNeverCrosses(t *testing.T) {
	st := NewTakeoverState(0.5)

	for _, physical := range []float64{0.1, 0.2, 0.3, 0.2, 0.1} {
		armed, cv := st.Update(physical)
		if armed {
			t.Fatalf("Update(%v): got armed=true, want armed=false (never reaches appValue 0.5)", physical)
		}
		if cv != 0.5 {
			t.Fatalf("Update(%v): got controlValue=%v, want 0.5 (ghost/target stays fixed while unarmed)", physical, cv)
		}
		if st.LastPhysical != physical {
			t.Fatalf("after Update(%v): st.LastPhysical=%v, want %v (D-09: live position still tracked while unarmed)", physical, st.LastPhysical, physical)
		}
	}
	if st.Armed {
		t.Fatalf("st.Armed = true, want false: physical never reached or crossed appValue")
	}
}

// TestTakeoverExactLanding covers a physical value that lands exactly on
// AppValue: CONTEXT.md/RESEARCH.md say an exact landing counts as a
// crossing and arms the control.
func TestTakeoverExactLanding(t *testing.T) {
	st := NewTakeoverState(0.5)

	if armed, cv := st.Update(0.3); armed || cv != 0.5 {
		t.Fatalf("Update(0.3): got armed=%v controlValue=%v, want armed=false controlValue=0.5", armed, cv)
	}
	armed, cv := st.Update(0.5)
	if !armed {
		t.Fatalf("Update(0.5): got armed=false, want armed=true (exact landing on appValue counts as a crossing)")
	}
	if cv != 0.5 {
		t.Fatalf("Update(0.5): got controlValue=%v, want 0.5", cv)
	}
}

// TestTakeoverFirstMessageNeverArmsSpuriously guards the bootstrap edge
// case: the very first physical reading a fresh TakeoverState receives
// must never be treated as an implicit crossing from an unknown prior
// position, regardless of which side of AppValue it lands on.
func TestTakeoverFirstMessageNeverArmsSpuriously(t *testing.T) {
	for _, first := range []float64{0.0, 0.5, 1.0} {
		st := NewTakeoverState(0.5)
		if armed, cv := st.Update(first); armed {
			t.Fatalf("first Update(%v) on a fresh state: got armed=%v controlValue=%v, want armed=false (no prior physical reading to cross from)", first, armed, cv)
		}
	}
}

// TestTakeoverSetAppValueReseedsGhostWhileUnarmed covers SetAppValue
// re-targeting the ghost/target marker when the app value changes from a
// source other than this control's own physical crossing, while the
// control is not armed.
func TestTakeoverSetAppValueReseedsGhostWhileUnarmed(t *testing.T) {
	st := NewTakeoverState(0.5)
	st.SetAppValue(0.8)

	if st.AppValue != 0.8 {
		t.Fatalf("after SetAppValue(0.8): st.AppValue=%v, want 0.8", st.AppValue)
	}
	if st.Armed {
		t.Fatalf("SetAppValue must not arm the control")
	}

	// A physical reading that already sits at the old target (0.5) must
	// NOT arm the control against the new target (0.8).
	if armed, cv := st.Update(0.5); armed || cv != 0.8 {
		t.Fatalf("Update(0.5) after re-seed to 0.8: got armed=%v controlValue=%v, want armed=false controlValue=0.8", armed, cv)
	}
}
