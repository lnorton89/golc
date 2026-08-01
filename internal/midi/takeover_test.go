package midi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTakeoverRisingCross covers a physical value rising through
// AppValue: the control must stay unarmed (and controlValue must stay
// the fixed ghost/target AppValue, D-10) until the message that reaches
// or crosses AppValue, then arm and track physical from then on.
func TestTakeoverRisingCross(t *testing.T) {
	st := NewTakeoverState(0.5)

	armed, cv := st.Update(0.2)
	require.False(t, armed, "Update(0.2)")
	require.Equal(t, 0.5, cv, "Update(0.2)")
	armed, cv = st.Update(0.4)
	require.False(t, armed, "Update(0.4)")
	require.Equal(t, 0.5, cv, "Update(0.4)")
	armed, cv = st.Update(0.6)
	require.True(t, armed, "Update(0.6): want armed=true (0.4 -> 0.6 crosses appValue 0.5)")
	require.Equal(t, 0.6, cv, "Update(0.6): armed control tracks physical")
	require.True(t, st.Armed, "st.Armed after crossing")
}

// TestTakeoverFallingCross covers a physical value falling through
// AppValue: same crossing rule, opposite direction.
func TestTakeoverFallingCross(t *testing.T) {
	st := NewTakeoverState(0.5)

	armed, cv := st.Update(0.9)
	require.False(t, armed, "Update(0.9)")
	require.Equal(t, 0.5, cv, "Update(0.9)")
	armed, cv = st.Update(0.6)
	require.False(t, armed, "Update(0.6)")
	require.Equal(t, 0.5, cv, "Update(0.6)")
	armed, cv = st.Update(0.4)
	require.True(t, armed, "Update(0.4): want armed=true (0.6 -> 0.4 crosses appValue 0.5)")
	require.Equal(t, 0.4, cv, "Update(0.4): armed control tracks physical")
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
		require.False(t, armed, "Update(%v): never reaches appValue 0.5", physical)
		require.Equal(t, 0.5, cv, "Update(%v): ghost/target stays fixed while unarmed", physical)
		require.Equal(t, physical, st.LastPhysical, "after Update(%v): D-09: live position still tracked while unarmed", physical)
	}
	require.False(t, st.Armed, "st.Armed: physical never reached or crossed appValue")
}

// TestTakeoverExactLanding covers a physical value that lands exactly on
// AppValue: CONTEXT.md/RESEARCH.md say an exact landing counts as a
// crossing and arms the control.
func TestTakeoverExactLanding(t *testing.T) {
	st := NewTakeoverState(0.5)

	armed, cv := st.Update(0.3)
	require.False(t, armed, "Update(0.3)")
	require.Equal(t, 0.5, cv, "Update(0.3)")
	armed, cv = st.Update(0.5)
	require.True(t, armed, "Update(0.5): want armed=true (exact landing on appValue counts as a crossing)")
	require.Equal(t, 0.5, cv, "Update(0.5)")
}

// TestTakeoverFirstMessageNeverArmsSpuriously guards the bootstrap edge
// case: the very first physical reading a fresh TakeoverState receives
// must never be treated as an implicit crossing from an unknown prior
// position, regardless of which side of AppValue it lands on.
func TestTakeoverFirstMessageNeverArmsSpuriously(t *testing.T) {
	for _, first := range []float64{0.0, 0.5, 1.0} {
		st := NewTakeoverState(0.5)
		armed, _ := st.Update(first)
		require.False(t, armed, "first Update(%v) on a fresh state: no prior physical reading to cross from", first)
	}
}

// TestTakeoverSetAppValueReseedsGhostWhileUnarmed covers SetAppValue
// re-targeting the ghost/target marker when the app value changes from a
// source other than this control's own physical crossing, while the
// control is not armed.
func TestTakeoverSetAppValueReseedsGhostWhileUnarmed(t *testing.T) {
	st := NewTakeoverState(0.5)
	st.SetAppValue(0.8)

	require.Equal(t, 0.8, st.AppValue, "after SetAppValue(0.8)")
	require.False(t, st.Armed, "SetAppValue must not arm the control")

	// A physical reading that already sits at the old target (0.5) must
	// NOT arm the control against the new target (0.8).
	armed, cv := st.Update(0.5)
	require.False(t, armed, "Update(0.5) after re-seed to 0.8")
	require.Equal(t, 0.8, cv, "Update(0.5) after re-seed to 0.8")
}
