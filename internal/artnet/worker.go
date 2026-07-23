// worker.go implements ARTN-03/ARTN-04's non-blocking, ticker-driven
// Art-Net send loop (04-03-PLAN.md Task 1, 04-RESEARCH.md Pattern 3,
// 04-PATTERNS.md worker.go section): Worker reads
// playback.Engine.CurrentFrame() (via the narrow FrameSource interface,
// satisfied by *playback.Engine, so tests can substitute a fake) on its
// own independent workerTickHz ticker -- copying
// internal/playback/engine.go's exact Start(ctx)/Stop()/context.WithCancel/
// time.NewTicker lifecycle -- rather than being driven by the engine's own
// tick callback (04-RESEARCH.md Assumption A2), so the two cadences stay
// decoupled even though both currently run at 40Hz.
//
// Each tick, tick(frame) walks every configured universe, calls
// channelmap.Encode (pure, in-memory) scoped to that universe's own
// instances so one universe's encode failure never blocks another's, and
// dispatches each of that universe's enabled targets' sends via
// dispatchSend. dispatchSend never calls a target's Write synchronously
// on the tick goroutine: it launches a bounded per-target goroutine (an
// atomic busy flag caps concurrency at one in-flight send per target --
// a persistently slow target's send is skipped on subsequent ticks rather
// than piling up unbounded goroutines, ARTN-04/T-04-10) that sets a
// per-send write deadline (Pitfall: SetWriteDeadline, not SO_BINDTODEVICE-
// style blocking) before writing. Because the tick loop itself never
// awaits any send, a hung/slow target can never delay the next tick or
// backpressure the engine (ARTN-04) -- this is proven, not merely
// asserted, by worker_test.go's non-blocking-cadence test.
//
// Targets are dialed via net.DialUDP bound to the interface manager's
// pinned local IP (04-PATTERNS.md Pitfall 5: Windows has no
// SO_BINDTODEVICE; bind by local address instead) through the unexported
// artNetSender interface (satisfied by *net.UDPConn in production), which
// exists solely so worker_test.go can substitute a deterministic fake
// sender for its non-blocking-cadence assertion rather than depending on
// real-network hang timing, which is not portably reproducible in a unit
// test.
package artnet

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/playback"
)

// workerTickHz is the Art-Net worker's own independent send cadence
// (04-RESEARCH.md Assumption A2): deliberately a separate constant from
// internal/playback's tickHz, even though both currently equal 40, so the
// two loops are never coupled by sharing one constant or one ticker.
const workerTickHz = 40

// workerTickInterval is the concrete worker tick period derived from
// workerTickHz.
const workerTickInterval = time.Second / workerTickHz

// defaultSendTimeout bounds a single per-target send when the caller
// does not configure an explicit WorkerConfig.SendTimeout.
const defaultSendTimeout = 200 * time.Millisecond

// FrameSource is the narrow interface Worker reads each tick. Satisfied
// by *playback.Engine's CurrentFrame() (a lock-free atomic.Pointer[Frame]
// read, ARTN-04's non-backpressuring consumption point); tests substitute
// a fake implementation instead of depending on a full Engine.
type FrameSource interface {
	CurrentFrame() *playback.Frame
}

// artNetSender is the minimal per-target send surface Worker depends on
// -- satisfied by *net.UDPConn in production. Existing only as an
// interface (rather than a concrete *net.UDPConn field) lets
// worker_test.go inject a deterministic fake sender for its non-blocking-
// cadence assertion.
type artNetSender interface {
	SetWriteDeadline(t time.Time) error
	Write(b []byte) (int, error)
	Close() error
}

// WorkerConfig configures a new Worker. Targets maps each configured
// universe to its fan-out target list (CONTEXT D-08: a universe may fan
// out to multiple unicast targets simultaneously); Instances is the full
// deployment instance list the worker groups by Instance.Universe at
// construction time so one universe's channelmap.Encode call never sees
// another universe's instances.
type WorkerConfig struct {
	Frames      FrameSource
	Instances   []deployment.Instance
	Resolve     ResolveFunc
	Targets     map[int][]Target
	LocalIP     net.IP
	SendTimeout time.Duration
	Health      *Health
}

// targetState is one configured target's live send state: the dialed
// sender connection (nil until Start succeeds in dialing it) and an
// atomic busy flag bounding in-flight sends to at most one per target.
type targetState struct {
	universe int
	target   Target
	sender   artNetSender
	busy     atomic.Bool
}

// Worker is the ticker-driven, non-blocking Art-Net send loop (ARTN-03/
// ARTN-04). See package doc comment for the full non-blocking-fan-out
// design.
type Worker struct {
	frames  FrameSource
	resolve ResolveFunc

	instancesByUniverse map[int][]deployment.Instance
	universes           []int
	targetStates        map[int][]*targetState

	localIP     net.IP
	sendTimeout time.Duration
	health      *Health

	seqMu sync.Mutex
	seq   map[int]uint8

	dialFunc func(universe int, target Target) (artNetSender, error)

	cancel context.CancelFunc
	done   chan struct{}
}

// NewWorker builds a Worker from cfg. A nil cfg.Health gets a fresh
// Health; either way Health.Configure(cfg.Targets) is called so the
// health model's tracking maps are bounded to exactly this worker's
// configured target set before any RecordSend can occur (Security
// Domain T-04-04).
func NewWorker(cfg WorkerConfig) *Worker {
	sendTimeout := cfg.SendTimeout
	if sendTimeout <= 0 {
		sendTimeout = defaultSendTimeout
	}

	health := cfg.Health
	if health == nil {
		health = NewHealth()
	}
	health.Configure(cfg.Targets)

	instancesByUniverse := map[int][]deployment.Instance{}
	for _, inst := range cfg.Instances {
		instancesByUniverse[inst.Universe] = append(instancesByUniverse[inst.Universe], inst)
	}

	universes := make([]int, 0, len(cfg.Targets))
	for u := range cfg.Targets {
		universes = append(universes, u)
	}
	sort.Ints(universes)

	w := &Worker{
		frames:              cfg.Frames,
		resolve:             cfg.Resolve,
		instancesByUniverse: instancesByUniverse,
		universes:           universes,
		targetStates:        map[int][]*targetState{},
		localIP:             cfg.LocalIP,
		sendTimeout:         sendTimeout,
		health:              health,
		seq:                 map[int]uint8{},
	}
	w.dialFunc = w.dialUDP

	for _, u := range universes {
		for _, t := range cfg.Targets[u] {
			w.targetStates[u] = append(w.targetStates[u], &targetState{universe: u, target: t})
		}
	}

	return w
}

// dialUDP opens target's real UDP connection bound to w.localIP (when
// set) as the local address (04-PATTERNS.md Pitfall 5: Windows has no
// SO_BINDTODEVICE; bind by local IP instead of a device-bind socket
// option). This is the production w.dialFunc; worker_test.go overrides
// dialFunc directly (same-package test) to substitute deterministic fake
// senders.
func (w *Worker) dialUDP(universe int, target Target) (artNetSender, error) {
	raddr := &net.UDPAddr{IP: target.IP, Port: effectivePort(target)}
	var laddr *net.UDPAddr
	if w.localIP != nil {
		laddr = &net.UDPAddr{IP: w.localIP}
	}
	conn, err := net.DialUDP("udp4", laddr, raddr)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_TARGET_DIAL_FAILED: universe %d target %s:%d: %v", universe, target.IP, effectivePort(target), err)
	}
	return conn, nil
}

// Start dials every configured target's sender then begins the tick loop
// on its own goroutine, mirroring internal/playback/engine.go's
// Start(ctx)/Stop() lifecycle exactly: context.WithCancel, one goroutine,
// time.NewTicker(workerTickInterval), select{ctx.Done / ticker.C}. A
// target that fails to dial is recorded as a send error (via Health) and
// simply has no sender for tick() to use -- it never blocks Start or the
// other targets.
func (w *Worker) Start(ctx context.Context) {
	for _, u := range w.universes {
		for _, ts := range w.targetStates[u] {
			sender, err := w.dialFunc(u, ts.target)
			if err != nil {
				w.health.RecordSend(u, ts.target, err)
				continue
			}
			ts.sender = sender
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})

	ticker := time.NewTicker(workerTickInterval)
	go func() {
		defer ticker.Stop()
		defer close(w.done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(w.frames.CurrentFrame())
			}
		}
	}()
}

// Stop cancels the context Start derived and waits for the tick goroutine
// to exit before closing every dialed target sender -- calling Stop
// before Start (or more than once) is a safe no-op.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.done != nil {
		<-w.done
	}
	for _, u := range w.universes {
		for _, ts := range w.targetStates[u] {
			if ts.sender != nil {
				_ = ts.sender.Close()
			}
		}
	}
}

// nextSeq returns the next Art-Net sequence value for universe, wrapping
// 1->255->1 and never returning 0 (packet.go's nextSeq, Pitfall 2),
// tracked independently per universe so one universe's cadence never
// perturbs another's sequence numbering.
func (w *Worker) nextSeq(universe int) uint8 {
	w.seqMu.Lock()
	defer w.seqMu.Unlock()
	next := nextSeq(w.seq[universe])
	w.seq[universe] = next
	return next
}

// tick reads frame (via CurrentFrame(), called by Start's own goroutine
// -- never blocking) and, for each configured universe, builds that
// universe's own DMX buffer (scoped to only that universe's instances, so
// an encode error in one universe never blocks another's tick), records
// that final buffer into Health via RecordUniverseValues (ARTN-05: the
// per-universe final values surfaced through "golc artnet status" -- this
// is the exact per-tick buffer that would otherwise be discarded after
// EncodeArtDMX), and fans out to its enabled targets via dispatchSend,
// which never blocks tick itself (ARTN-04).
func (w *Worker) tick(frame *playback.Frame) {
	if frame == nil {
		return
	}
	w.health.RecordFrame(time.Now())

	for _, u := range w.universes {
		buffers, err := Encode(*frame, w.instancesByUniverse[u], w.resolve)
		if err != nil {
			w.health.RecordEncodeError(u, err)
			continue
		}
		data, ok := buffers[u]
		if !ok {
			data = make([]byte, channelsPerUniverse)
		}
		w.health.RecordUniverseValues(u, data)

		pkt, err := EncodeArtDMX(w.nextSeq(u), 0, PortAddress(u), data)
		if err != nil {
			w.health.RecordEncodeError(u, err)
			continue
		}

		for _, ts := range w.targetStates[u] {
			if !ts.target.Enabled {
				continue
			}
			w.dispatchSend(ts, pkt)
		}
	}
}

// dispatchSend launches (at most) one in-flight send goroutine per
// target: if a prior tick's send for this exact target is still running,
// this tick's send is skipped rather than letting goroutines pile up
// under a persistently slow target (ARTN-04/T-04-10). The goroutine
// itself -- never the caller, always tick()'s caller (Start's own ticker
// goroutine) -- is the only place that ever calls sender.Write, so tick()
// always returns immediately regardless of how long the send takes.
func (w *Worker) dispatchSend(ts *targetState, pkt []byte) {
	if ts.sender == nil {
		w.health.RecordSend(ts.universe, ts.target, fmt.Errorf("GOLC_ARTNET_TARGET_NOT_CONNECTED: target %s:%d has no open connection", ts.target.IP, effectivePort(ts.target)))
		return
	}
	if !ts.busy.CompareAndSwap(false, true) {
		return
	}

	sender := ts.sender
	timeout := w.sendTimeout
	universe := ts.universe
	target := ts.target
	health := w.health

	go func() {
		defer ts.busy.Store(false)
		_ = sender.SetWriteDeadline(time.Now().Add(timeout))
		_, err := sender.Write(pkt)
		health.RecordSend(universe, target, err)
	}()
}
