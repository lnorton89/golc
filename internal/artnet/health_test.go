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
	"strings"
	"testing"
	"time"
)

// TestFrameHealthOnCadenceVsStalled proves D-09's core classification:
// an 8ms-old frame read is on-cadence; a 400ms-old one (with
// frameStaleAfter well under 400ms at workerTickHz=40) is stalled and
// reports GOLC_ARTNET_FRAME_STALLED.
func TestFrameHealthOnCadenceVsStalled(t *testing.T) {
	t0 := time.Now()

	onCadence := evaluateFrameHealth(t0, t0.Add(8*time.Millisecond))
	if !onCadence.OnCadence {
		t.Fatal("expected on-cadence classification for an 8ms-old frame read")
	}
	if err := onCadence.Err(); err != nil {
		t.Fatalf("expected no error for on-cadence frame health, got %v", err)
	}

	stalled := evaluateFrameHealth(t0, t0.Add(400*time.Millisecond))
	if stalled.OnCadence {
		t.Fatal("expected stalled classification for a 400ms-old frame read")
	}
	err := stalled.Err()
	if err == nil || !strings.Contains(err.Error(), "GOLC_ARTNET_FRAME_STALLED") {
		t.Fatalf("expected GOLC_ARTNET_FRAME_STALLED, got %v", err)
	}
}

// TestFrameHealthNeverRecordedIsStalled proves the zero-value case (no
// RecordFrame has ever happened) classifies as stalled, never as a false
// on-cadence positive.
func TestFrameHealthNeverRecordedIsStalled(t *testing.T) {
	fh := evaluateFrameHealth(time.Time{}, time.Now())
	if fh.OnCadence {
		t.Fatal("expected a never-recorded frame to classify as stalled")
	}
}

// TestHealthRecordFrameThenSnapshotReportsFreshness is an integration-
// level check that Health.RecordFrame + Snapshot compose evaluateFrameHealth
// correctly against real recorded timestamps.
func TestHealthRecordFrameThenSnapshotReportsFreshness(t *testing.T) {
	h := NewHealth()

	h.RecordFrame(time.Now())
	if snap := h.Snapshot(); !snap.Frame.OnCadence {
		t.Fatal("expected a freshly recorded frame to be on-cadence")
	}

	h.RecordFrame(time.Now().Add(-time.Second))
	snap := h.Snapshot()
	if snap.Frame.OnCadence {
		t.Fatal("expected a frame recorded 1s in the past to be stalled")
	}
	if err := snap.Frame.Err(); err == nil {
		t.Fatal("expected stalled frame health to report a non-nil error")
	}
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
	if !ok {
		t.Fatal("expected a tracked entry for the configured target")
	}
	if th.SendErr != 2 || th.SendOK != 0 {
		t.Fatalf("expected SendErr=2 SendOK=0, got SendErr=%d SendOK=%d", th.SendErr, th.SendOK)
	}
	if th.Reachable {
		t.Fatal("expected a target with only errors to be unreachable")
	}

	h.RecordSend(1, target, nil)
	snap = h.Snapshot()
	th = snap.Targets[key]
	if th.SendOK != 1 || !th.Reachable {
		t.Fatalf("expected SendOK=1 Reachable=true after a successful send, got SendOK=%d Reachable=%v", th.SendOK, th.Reachable)
	}
	if th.SendErr != 2 {
		t.Fatalf("expected prior SendErr=2 to be preserved, got %d", th.SendErr)
	}
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
	if _, ok := snap.Targets[keyOf(unsolicited)]; ok {
		t.Fatal("expected an unsolicited/unconfigured target to never gain a tracking entry")
	}
	if len(snap.Targets) != 1 {
		t.Fatalf("expected exactly 1 tracked (configured) target, got %d", len(snap.Targets))
	}
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
	if !strings.Contains(logLine, "GOLC_ARTNET_SEND_FAILED") {
		t.Fatalf("expected log line to contain GOLC_ARTNET_SEND_FAILED, got %q", logLine)
	}
	if !strings.Contains(logLine, "universe=5") {
		t.Fatalf("expected log line to contain universe=5, got %q", logLine)
	}
	if !strings.Contains(logLine, "10.0.0.7") {
		t.Fatalf("expected log line to contain the target IP, got %q", logLine)
	}
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
	if !strings.Contains(logLine, "GOLC_ARTNET_ENCODE_FAILED") {
		t.Fatalf("expected log line to contain GOLC_ARTNET_ENCODE_FAILED, got %q", logLine)
	}
	if !strings.Contains(logLine, "universe=7") {
		t.Fatalf("expected log line to contain universe=7, got %q", logLine)
	}
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
	if !ok {
		t.Fatal("expected a tracked UniverseValues entry for configured universe 1")
	}
	if !bytes.Equal(got, buf) {
		t.Fatalf("expected recorded universe values to equal %v, got %v", buf, got)
	}
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
	if _, ok := snap.UniverseValues[2]; ok {
		t.Fatal("expected an unconfigured universe to never gain a UniverseValues tracking entry")
	}
	if len(snap.UniverseValues) != 0 {
		t.Fatalf("expected exactly 0 tracked universe values (universe 1 never recorded), got %d", len(snap.UniverseValues))
	}
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
	if got[0] != 5 {
		t.Fatalf("expected the recorded snapshot to be unaffected by a later mutation of the caller's buffer, got byte %d", got[0])
	}
}
