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
