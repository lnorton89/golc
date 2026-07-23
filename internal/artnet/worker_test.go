// worker_test.go proves ARTN-03/ARTN-04's worker contract (04-03-PLAN.md
// Task 1): a loopback listener receives a decodable ArtDMX packet with
// the expected Port-Address/sequence/length/data (a); a persistently
// slow target never reduces a healthy target's tick cadence (b,
// ARTN-04's core non-blocking guarantee -- proven via a deterministic
// fake sender rather than real-network hang timing, which is not
// portably reproducible); a disabled target receives zero packets while
// its universe's enabled targets keep receiving (c, D-12); sequence
// advances per universe and never emits 0 (d, Pitfall 2); and ctx cancel
// stops the tick goroutine cleanly (e, no leak).
//
// 06-02-PLAN.md Task 2 adds: setting blackout on a Worker's own
// safetyState (exactly what the daemon's "artnet safety blackout" handler
// does) flips a healthy target's received intensity to 0 within a bounded
// wall-clock time, even while a slow target's Write is in flight (f); and
// buildMembership-derived group masters compose multiplicatively with the
// grand master to scale a target's received intensity (g, PLAY-06
// adjacency).
package artnet

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/playback"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/scene"
)

// singleChannelMode is a minimal D-16 channel layout: one intensity
// channel at offset 0.
var singleChannelMode = fixture.Mode{
	Name:     "Standard",
	Channels: []fixture.ChannelSlot{{Type: fixture.CapabilityIntensity, Occurrence: 0}},
}

func staticWorkerResolver(mode fixture.Mode) ResolveFunc {
	return func(instance deployment.Instance) (InstanceFixture, error) {
		return InstanceFixture{
			Definition: fixture.FixtureDefinition{Modes: []fixture.Mode{mode}},
			Mode:       mode,
		}, nil
	}
}

// fakeFrameSource is a minimal FrameSource test double substituting for
// *playback.Engine.
type fakeFrameSource struct {
	mu    sync.Mutex
	frame *playback.Frame
	calls atomic.Int64
}

func (f *fakeFrameSource) CurrentFrame() *playback.Frame {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frame
}

func (f *fakeFrameSource) setFrame(frame *playback.Frame) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frame = frame
}

func newLoopbackListener(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func listenerPort(t *testing.T, conn *net.UDPConn) int {
	t.Helper()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected *net.UDPAddr, got %T", conn.LocalAddr())
	}
	return addr.Port
}

// slowSender deterministically simulates a target whose Write takes
// noticeably longer than one worker tick, without depending on real OS
// network-hang semantics (which are not portably reproducible in a
// unit test).
type slowSender struct {
	delay time.Duration
	sends atomic.Int64
}

func (s *slowSender) SetWriteDeadline(time.Time) error { return nil }

func (s *slowSender) Write(b []byte) (int, error) {
	s.sends.Add(1)
	time.Sleep(s.delay)
	return len(b), nil
}

func (s *slowSender) Close() error { return nil }

func mustInstanceID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}
	return id
}

func frameWithIntensity(instanceID uuid.UUID, value float64) *playback.Frame {
	return &playback.Frame{Values: map[uuid.UUID]scene.AttributeSet{
		instanceID: {Values: map[fixture.CapabilityType]float64{fixture.CapabilityIntensity: value}},
	}}
}

// TestWorkerLoopbackReceivesDecodableArtDMX proves (a): a loopback
// listener receives an ArtDMX packet whose decoded Port-Address/
// sequence/length/data match the published frame.
func TestWorkerLoopbackReceivesDecodableArtDMX(t *testing.T) {
	instanceID := mustInstanceID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 3, Address: 1}

	listener := newLoopbackListener(t)
	target := Target{Universe: 3, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, listener), Enabled: true}

	frames := &fakeFrameSource{frame: frameWithIntensity(instanceID, 1.0)}

	w := NewWorker(WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Resolve:   staticWorkerResolver(singleChannelMode),
		Targets:   map[int][]Target{3: {target}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer func() {
		cancel()
		w.Stop()
	}()

	buf := make([]byte, 600)
	if err := listener.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, _, err := listener.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUDP: %v", err)
	}
	pkt := buf[:n]

	if n < 18 {
		t.Fatalf("packet too short: %d bytes", n)
	}
	if string(pkt[0:8]) != "Art-Net\x00" {
		t.Fatalf("bad Art-Net ID header: %q", pkt[0:8])
	}
	seq := pkt[12]
	if seq != 1 {
		t.Fatalf("expected first sequence to be 1, got %d", seq)
	}
	gotPortAddress := uint16(pkt[14]) | uint16(pkt[15])<<8
	wantPortAddress := PortAddress(3)
	if gotPortAddress != wantPortAddress {
		t.Fatalf("expected Port-Address %#x, got %#x", wantPortAddress, gotPortAddress)
	}
	length := int(pkt[16])<<8 | int(pkt[17])
	if length != channelsPerUniverse {
		t.Fatalf("expected data length %d, got %d", channelsPerUniverse, length)
	}
	data := pkt[18:]
	if len(data) != channelsPerUniverse {
		t.Fatalf("expected %d data bytes, got %d", channelsPerUniverse, len(data))
	}
	if data[0] != 255 {
		t.Fatalf("expected channel 0 (intensity=1.0) to encode to 255, got %d", data[0])
	}
}

// TestWorkerSlowTargetDoesNotStallHealthyTarget proves (b): ARTN-04's
// core guarantee -- a persistently slow target's send never delays the
// next tick, so a healthy target's received-packet cadence is unaffected
// over a short window.
func TestWorkerSlowTargetDoesNotStallHealthyTarget(t *testing.T) {
	instanceID := mustInstanceID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	healthyListener := newLoopbackListener(t)
	healthyTarget := Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, healthyListener), Enabled: true}
	slowTarget := Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: 1, Enabled: true}

	frames := &fakeFrameSource{frame: frameWithIntensity(instanceID, 1.0)}

	w := NewWorker(WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Resolve:   staticWorkerResolver(singleChannelMode),
		Targets:   map[int][]Target{1: {healthyTarget, slowTarget}},
	})

	slow := &slowSender{delay: 500 * time.Millisecond}
	w.dialFunc = func(universe int, target Target) (artNetSender, error) {
		if target.Port == slowTarget.Port {
			return slow, nil
		}
		return w.dialUDP(universe, target)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer func() {
		cancel()
		w.Stop()
	}()

	window := 150 * time.Millisecond // ~6 ticks at 40Hz
	deadline := time.Now().Add(window + time.Second)
	if err := healthyListener.SetReadDeadline(deadline); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	received := 0
	stop := time.Now().Add(window)
	buf := make([]byte, 600)
	for time.Now().Before(stop) {
		_ = healthyListener.SetReadDeadline(stop)
		n, _, err := healthyListener.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if n > 0 {
			received++
		}
	}

	if received < 3 {
		t.Fatalf("expected the healthy target to keep receiving packets on cadence despite a slow target, got only %d packets in %s", received, window)
	}

	// The slow target's Write is proven to have actually been invoked
	// (not skipped), demonstrating the tick loop dispatched to it without
	// waiting for it.
	if slow.sends.Load() == 0 {
		t.Fatal("expected the slow target's Write to have been invoked at least once")
	}
}

// TestWorkerDisabledTargetReceivesNothing proves (c, D-12): a disabled
// target receives zero packets while its universe's enabled targets keep
// receiving.
func TestWorkerDisabledTargetReceivesNothing(t *testing.T) {
	instanceID := mustInstanceID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 2, Address: 1}

	enabledListener := newLoopbackListener(t)
	disabledListener := newLoopbackListener(t)

	enabledTarget := Target{Universe: 2, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, enabledListener), Enabled: true}
	disabledTarget := Target{Universe: 2, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, disabledListener), Enabled: false}

	frames := &fakeFrameSource{frame: frameWithIntensity(instanceID, 1.0)}

	w := NewWorker(WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Resolve:   staticWorkerResolver(singleChannelMode),
		Targets:   map[int][]Target{2: {enabledTarget, disabledTarget}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer func() {
		cancel()
		w.Stop()
	}()

	buf := make([]byte, 600)
	if err := enabledListener.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, _, err := enabledListener.ReadFromUDP(buf); err != nil {
		t.Fatalf("expected the enabled target to receive a packet, got error: %v", err)
	}

	if err := disabledListener.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if n, _, err := disabledListener.ReadFromUDP(buf); err == nil {
		t.Fatalf("expected the disabled target to receive nothing, got %d bytes", n)
	}
}

// TestSafetyOverrideBlackoutTakesEffectDespiteSlowTarget proves
// 06-02-PLAN.md Task 2's local-priority contract (PLAY-06/08/09): setting
// blackout on a Worker's own safetyState -- exactly what the daemon's
// "artnet safety blackout" handler does -- flips a healthy target's
// received intensity to 0 within a bounded wall-clock time (a handful of
// tick periods), even while a persistently slow/hung target's Write is
// also in flight on the same tick (reusing worker_test.go's own
// slowSender fake-sender harness, mirroring
// TestWorkerSlowTargetDoesNotStallHealthyTarget's non-blocking proof).
// This demonstrates the override is never delayed by a slow "client" of
// the tick loop, matching the daemon-side setter's own non-blocking,
// atomic-only contract (safety.go).
func TestSafetyOverrideBlackoutTakesEffectDespiteSlowTarget(t *testing.T) {
	instanceID := mustInstanceID(t)
	instance := deployment.Instance{ID: instanceID, Mode: "Standard", Universe: 1, Address: 1}

	healthyListener := newLoopbackListener(t)
	healthyTarget := Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, healthyListener), Enabled: true}
	slowTarget := Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: 1, Enabled: true}

	frames := &fakeFrameSource{frame: frameWithIntensity(instanceID, 1.0)}
	safety := newSafetyState()

	w := NewWorker(WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Resolve:   staticWorkerResolver(singleChannelMode),
		Targets:   map[int][]Target{1: {healthyTarget, slowTarget}},
		Safety:    safety,
	})

	slow := &slowSender{delay: 500 * time.Millisecond}
	w.dialFunc = func(universe int, target Target) (artNetSender, error) {
		if target.Port == slowTarget.Port {
			return slow, nil
		}
		return w.dialUDP(universe, target)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer func() {
		cancel()
		w.Stop()
	}()

	// Confirm the healthy target first receives the programmed (non-zero)
	// intensity before any override is set.
	buf := make([]byte, 600)
	if err := healthyListener.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, _, err := healthyListener.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("ReadFromUDP (pre-blackout): %v", err)
	}
	if buf[18] != 255 {
		t.Fatalf("expected pre-blackout channel 0 to be 255 (intensity=1.0), got %d", buf[18])
	}

	// setBlackout mirrors exactly what the daemon's "artnet safety
	// blackout" handler calls -- a single non-blocking atomic Store, never
	// gated on the slow target's in-flight Write.
	safety.setBlackout(true)

	// Within a bounded wall-clock window (a handful of tick periods), the
	// healthy target must start receiving zeroed intensity.
	deadline := time.Now().Add(500 * time.Millisecond)
	sawZero := false
	for time.Now().Before(deadline) {
		if err := healthyListener.SetReadDeadline(deadline); err != nil {
			t.Fatalf("SetReadDeadline: %v", err)
		}
		n, _, err = healthyListener.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if n >= 19 && buf[18] == 0 {
			sawZero = true
			break
		}
	}
	if !sawZero {
		t.Fatal("expected the healthy target to receive zeroed intensity within the bounded window after setBlackout, despite a slow target in flight")
	}

	if slow.sends.Load() == 0 {
		t.Fatal("expected the slow target's Write to have been invoked at least once")
	}
}

// TestWorkerGroupMasterComposesWithGrandMaster proves (g, PLAY-06
// adjacency): a Worker built with WorkerConfig.Groups carrying a
// MemberRef matching the instance's own (PoolID, PoolMemberID) has that
// instance's intensity scaled by grand x group master, multiplicatively --
// grand=0.5, group=0.5, programmed=full -> the healthy target receives
// intensity 0.25*255 rounded, proving buildMembership's instance -> group
// mapping reaches applyOverrides end-to-end through the real Worker tick.
func TestWorkerGroupMasterComposesWithGrandMaster(t *testing.T) {
	poolID := mustInstanceID(t)
	poolMemberID := mustInstanceID(t)
	groupID := mustInstanceID(t)
	instanceID := mustInstanceID(t)

	instance := deployment.Instance{
		ID: instanceID, PoolID: poolID, PoolMemberID: poolMemberID,
		Mode: "Standard", Universe: 1, Address: 1,
	}
	group := pool.Group{ID: groupID, Name: "Test Group", MemberRefs: []pool.MemberRef{
		{PoolID: poolID, PoolMemberID: poolMemberID},
	}}

	healthyListener := newLoopbackListener(t)
	healthyTarget := Target{Universe: 1, IP: net.IPv4(127, 0, 0, 1), Port: listenerPort(t, healthyListener), Enabled: true}

	frames := &fakeFrameSource{frame: frameWithIntensity(instanceID, 1.0)}
	safety := newSafetyState()
	if err := safety.setGrandMaster(0.5); err != nil {
		t.Fatalf("setGrandMaster: %v", err)
	}
	if err := safety.setGroupMaster(groupID, 0.5); err != nil {
		t.Fatalf("setGroupMaster: %v", err)
	}

	w := NewWorker(WorkerConfig{
		Frames:    frames,
		Instances: []deployment.Instance{instance},
		Groups:    []pool.Group{group},
		Resolve:   staticWorkerResolver(singleChannelMode),
		Targets:   map[int][]Target{1: {healthyTarget}},
		Safety:    safety,
	})

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	defer func() {
		cancel()
		w.Stop()
	}()

	buf := make([]byte, 600)
	if err := healthyListener.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, _, err := healthyListener.ReadFromUDP(buf); err != nil {
		t.Fatalf("ReadFromUDP: %v", err)
	}

	var composed float64 = 0.25
	wantChannel := byte(composed * 255) // channelmap's own [0,1]->[0,255] scaling (truncating)
	if buf[18] != wantChannel {
		t.Fatalf("expected channel 0 to reflect grand(0.5)*group(0.5)=0.25 of full intensity (%d), got %d", wantChannel, buf[18])
	}
}

// TestWorkerSequenceAdvancesPerUniverseNeverZero proves (d): sequence
// advances independently per universe and never emits 0 (Pitfall 2).
func TestWorkerSequenceAdvancesPerUniverseNeverZero(t *testing.T) {
	w := NewWorker(WorkerConfig{
		Frames:  &fakeFrameSource{},
		Targets: map[int][]Target{1: nil, 2: nil},
	})

	var gotUniverse1, gotUniverse2 []uint8
	for i := 0; i < 3; i++ {
		gotUniverse1 = append(gotUniverse1, w.nextSeq(1))
	}
	for i := 0; i < 2; i++ {
		gotUniverse2 = append(gotUniverse2, w.nextSeq(2))
	}

	wantUniverse1 := []uint8{1, 2, 3}
	for i, want := range wantUniverse1 {
		if gotUniverse1[i] != want {
			t.Fatalf("universe 1 sequence[%d] = %d, want %d", i, gotUniverse1[i], want)
		}
	}
	wantUniverse2 := []uint8{1, 2}
	for i, want := range wantUniverse2 {
		if gotUniverse2[i] != want {
			t.Fatalf("universe 2 sequence[%d] = %d, want %d", i, gotUniverse2[i], want)
		}
	}
	for _, seq := range append(gotUniverse1, gotUniverse2...) {
		if seq == 0 {
			t.Fatal("sequence must never emit 0 (Pitfall 2)")
		}
	}
}

// TestWorkerStopEndsGoroutine proves (e): ctx cancel (via Stop) ends the
// tick goroutine cleanly -- no further CurrentFrame reads occur once Stop
// returns.
func TestWorkerStopEndsGoroutine(t *testing.T) {
	frames := &fakeFrameSource{frame: &playback.Frame{}}

	w := NewWorker(WorkerConfig{
		Frames:  frames,
		Targets: map[int][]Target{},
	})

	ctx := context.Background()
	w.Start(ctx)

	time.Sleep(3 * workerTickInterval)
	w.Stop()

	callsAtStop := frames.calls.Load()
	time.Sleep(5 * workerTickInterval)
	callsAfterWait := frames.calls.Load()

	if callsAfterWait != callsAtStop {
		t.Fatalf("expected no further CurrentFrame reads after Stop, got %d more", callsAfterWait-callsAtStop)
	}
}
