// health_test.go proves ARTN-05's frame/target health contract
// (04-03-PLAN.md Task 2): an 8ms-old frame read classifies on-cadence
// while a 400ms-old one classifies stalled with GOLC_ARTNET_FRAME_STALLED
// (D-09); per-target send success/error counts accumulate and
// Reachable distinguishes an all-errors target from one with at least
// one success (D-10); an unsolicited/unconfigured target address never
// gains a tracking entry (Security Domain T-04-04 DoS bound); the
// published snapshot is safely readable concurrently with the send path
// (atomic.Pointer, no shared lock); and every recorded error emits a
// structured {DOMAIN}_{CONDITION} log line (D-11).
package artnet

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFrameHealthOnCadenceVsStalled proves D-09's core classification:
// an 8ms-old frame read is on-cadence; a 400ms-old one (with
// frameStaleAfter well under 400ms at workerTickHz=40) is stalled and
// reports GOLC_ARTNET_FRAME_STALLED.
func TestFrameHealthOnCadenceVsStalled(t *testing.T) {
	t0 := time.Now()

	onCadence := evaluateFrameHealth(t0, t0.Add(8*time.Millisecond))
	require.True(t, onCadence.OnCadence, "expected on-cadence classification for an 8ms-old frame read")
	require.NoError(t, onCadence.Err(), "expected no error for on-cadence frame health")

	stalled := evaluateFrameHealth(t0, t0.Add(400*time.Millisecond))
	require.False(t, stalled.OnCadence, "expected stalled classification for a 400ms-old frame read")
	err := stalled.Err()
	require.ErrorContains(t, err, "GOLC_ARTNET_FRAME_STALLED")
}

// TestFrameHealthNeverRecordedIsStalled proves the zero-value case (no
// RecordFrame has ever happened) classifies as stalled, never as a false
// on-cadence positive.
func TestFrameHealthNeverRecordedIsStalled(t *testing.T) {
	fh := evaluateFrameHealth(time.Time{}, time.Now())
	require.False(t, fh.OnCadence, "expected a never-recorded frame to classify as stalled")
}

// TestHealthRecordFrameThenSnapshotReportsFreshness is an integration-
// level check that Health.RecordFrame + Snapshot compose evaluateFrameHealth
// correctly against real recorded timestamps.
func TestHealthRecordFrameThenSnapshotReportsFreshness(t *testing.T) {
	h := NewHealth()

	h.RecordFrame(time.Now())
	snap := h.Snapshot()
	require.True(t, snap.Frame.OnCadence, "expected a freshly recorded frame to be on-cadence")

	h.RecordFrame(time.Now().Add(-time.Second))
	snap = h.Snapshot()
	require.False(t, snap.Frame.OnCadence, "expected a frame recorded 1s in the past to be stalled")
	require.Error(t, snap.Frame.Err(), "expected stalled frame health to report a non-nil error")
}

// TestHealthTargetSendAccumulatesAndDistinguishesReachability proves
// D-10: send success/error counts accumulate, and Reachable distinguishes
// a target with only errors from one with at least one success.
func TestHealthTargetSendAccumulatesAndDistinguishesReachability(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {target}})

	h.RecordSend(1, target, errors.New("boom"))
	h.RecordSend(1, target, errors.New("boom again"))

	key := keyOf(target)
	snap := h.Snapshot()
	th, ok := snap.Targets[key]
	require.True(t, ok, "expected a tracked entry for the configured target")
	require.Equalf(t, 2, th.SendErr, "expected SendErr=2 SendOK=0, got SendErr=%d SendOK=%d", th.SendErr, th.SendOK)
	require.Equalf(t, 0, th.SendOK, "expected SendErr=2 SendOK=0, got SendErr=%d SendOK=%d", th.SendErr, th.SendOK)
	require.False(t, th.Reachable, "expected a target with only errors to be unreachable")

	h.RecordSend(1, target, nil)
	snap = h.Snapshot()
	th = snap.Targets[key]
	require.Equalf(t, 1, th.SendOK, "expected SendOK=1 Reachable=true after a successful send, got SendOK=%d Reachable=%v", th.SendOK, th.Reachable)
	require.Truef(t, th.Reachable, "expected SendOK=1 Reachable=true after a successful send, got SendOK=%d Reachable=%v", th.SendOK, th.Reachable)
	require.Equalf(t, 2, th.SendErr, "expected prior SendErr=2 to be preserved, got %d", th.SendErr)
}

// TestHealthUnconfiguredTargetNeverTracked proves the Security Domain
// T-04-04 DoS bound: an unsolicited/unconfigured target address never
// gains a tracking entry, no matter how many times RecordSend is called
// against it.
func TestHealthUnconfiguredTargetNeverTracked(t *testing.T) {
	configured := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {configured}})

	unsolicited := Target{Universe: 1, IP: net.ParseIP("10.0.0.99"), Port: artNetPort, Enabled: true}
	h.RecordSend(1, unsolicited, errors.New("boom"))
	h.RecordSend(1, unsolicited, nil)

	snap := h.Snapshot()
	_, ok := snap.Targets[keyOf(unsolicited)]
	require.False(t, ok, "expected an unsolicited/unconfigured target to never gain a tracking entry")
	require.Len(t, snap.Targets, 1, "expected exactly 1 tracked (configured) target")
}

// TestHealthSnapshotConcurrentWithRecordSendNoRace proves the snapshot is
// safely readable concurrently with the send path (atomic.Pointer
// publish/read, no shared lock on the read side) -- run with -race.
func TestHealthSnapshotConcurrentWithRecordSendNoRace(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {target}})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			h.RecordSend(1, target, nil)
		}
	}()

	for i := 0; i < 200; i++ {
		_ = h.Snapshot()
	}
	<-done
}

// TestHealthRecordSendErrorEmitsStructuredLogLine proves D-11: a send
// failure emits a structured GOLC_ARTNET_SEND_FAILED log line carrying
// the universe and target.
func TestHealthRecordSendErrorEmitsStructuredLogLine(t *testing.T) {
	target := Target{Universe: 5, IP: net.ParseIP("10.0.0.7"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{5: {target}})

	var buf bytes.Buffer
	original := artnetLogOutput
	artnetLogOutput = &buf
	defer func() { artnetLogOutput = original }()

	h.RecordSend(5, target, errors.New("write failed"))

	logLine := buf.String()
	require.Contains(t, logLine, "GOLC_ARTNET_SEND_FAILED")
	require.Contains(t, logLine, "universe=5")
	require.Contains(t, logLine, "10.0.0.7")
}

// TestHealthRecordEncodeErrorEmitsStructuredLogLine proves D-11 for
// universe-level encode failures (GOLC_ARTNET_ENCODE_FAILED).
func TestHealthRecordEncodeErrorEmitsStructuredLogLine(t *testing.T) {
	h := NewHealth()

	var buf bytes.Buffer
	original := artnetLogOutput
	artnetLogOutput = &buf
	defer func() { artnetLogOutput = original }()

	h.RecordEncodeError(7, errors.New("bad layout"))

	logLine := buf.String()
	require.Contains(t, logLine, "GOLC_ARTNET_ENCODE_FAILED")
	require.Contains(t, logLine, "universe=7")
}

// TestHealthRecordUniverseValuesSnapshotReflectsConfiguredUniverse proves
// the ARTN-05 gap-closure contract: RecordUniverseValues on a configured
// universe is reflected verbatim in the next Snapshot's UniverseValues.
func TestHealthRecordUniverseValuesSnapshotReflectsConfiguredUniverse(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {target}})

	buf := make([]byte, channelsPerUniverse)
	buf[0] = 42
	buf[10] = 200
	h.RecordUniverseValues(1, buf)

	snap := h.Snapshot()
	got, ok := snap.UniverseValues[1]
	require.True(t, ok, "expected a tracked UniverseValues entry for configured universe 1")
	require.True(t, bytes.Equal(got, buf), "expected recorded universe values to equal %v, got %v", buf, got)
}

// TestHealthUnconfiguredUniverseValuesNeverTracked proves the Security
// Domain T-04-04 DoS bound extended to per-universe values: an
// unconfigured universe never gains a UniverseValues tracking entry.
func TestHealthUnconfiguredUniverseValuesNeverTracked(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {target}})

	h.RecordUniverseValues(2, make([]byte, channelsPerUniverse))

	snap := h.Snapshot()
	_, ok := snap.UniverseValues[2]
	require.False(t, ok, "expected an unconfigured universe to never gain a UniverseValues tracking entry")
	require.Len(t, snap.UniverseValues, 0, "expected exactly 0 tracked universe values (universe 1 never recorded)")
}

// TestHealthRecordUniverseValuesIsDefensivelyCopied proves
// RecordUniverseValues takes a defensive copy: mutating the caller's
// buffer after recording must never change the published snapshot.
func TestHealthRecordUniverseValuesIsDefensivelyCopied(t *testing.T) {
	target := Target{Universe: 1, IP: net.ParseIP("10.0.0.5"), Port: artNetPort, Enabled: true}
	h := NewHealth()
	h.Configure(map[int][]Target{1: {target}})

	buf := make([]byte, channelsPerUniverse)
	buf[0] = 5
	h.RecordUniverseValues(1, buf)

	buf[0] = 250

	snap := h.Snapshot()
	got := snap.UniverseValues[1]
	require.Equalf(t, byte(5), got[0], "expected the recorded snapshot to be unaffected by a later mutation of the caller's buffer, got byte %d", got[0])
}
