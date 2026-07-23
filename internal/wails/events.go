// events.go implements 06-04-PLAN.md Task 1's throttled pushStatus
// scaffold: EventPusher coalesces per-feature snapshots into a single
// fixed-cadence emit loop (eventsTickInterval, ~one push per rendered
// frame budget), deliberately decoupled from both the Art-Net Worker's
// 40Hz tick and any MIDI message rate (internal/artnet/worker.go's own
// "independent cadence, never share one ticker" convention;
// 06-RESEARCH.md Open Question 3). A burst of rapid updates (e.g. a fast
// MIDI fader sweep, 06-08) therefore coalesces into eventsTickInterval-
// spaced pushes rather than one runtime.EventsEmit call per message --
// this is a throttled hint stream, never the playback/status source of
// truth (06-RESEARCH.md Anti-Pattern: "Treating Wails EventsEmit as ...
// authoritative"; the frontend re-queries authoritative state on any
// detected gap).
//
// 06-05/06-08 fill this scaffold's per-feature emit helpers (QueueStatus,
// QueueMidiFeedback) with real payload shapes; this file only owns the
// throttle mechanism itself.
package wails

import (
	"context"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventsTickInterval is pushStatus's own throttle cadence -- roughly one
// push per rendered frame budget (16-33ms), independent of the Art-Net
// Worker's 40Hz tick.
const eventsTickInterval = 25 * time.Millisecond

// emitFunc abstracts runtime.EventsEmit so tests never need a real Wails
// application context.
type emitFunc func(ctx context.Context, eventName string, data ...interface{})

func defaultEmit(ctx context.Context, eventName string, data ...interface{}) {
	wailsruntime.EventsEmit(ctx, eventName, data...)
}

// EventPusher is the throttled EventsEmit scaffold: Start begins one
// fixed-cadence ticker; QueueStatus/QueueMidiFeedback only stage the
// latest snapshot under their event name -- the ticker goroutine is the
// sole place that actually calls emit, so bursts coalesce rather than
// emitting once per update.
type EventPusher struct {
	emit emitFunc

	mu     sync.Mutex
	latest map[string]interface{}
	// midiFeedback stages one MidiFeedback snapshot per mapping ID (WR-02
	// gap-closure-review fix), separate from latest's single-value-per-
	// event-name slot: latest can only ever remember the single
	// most-recently-queued MidiFeedback across ALL mappings, silently
	// dropping every other mapping's feedback staged in the same
	// eventsTickInterval window (e.g. two faders touched within the same
	// ~25ms tick). Keying by mapping ID here lets flush emit every
	// distinct mapping's feedback each tick, still under the one
	// unchanged "midi:feedback" event name the frontend subscribes to.
	midiFeedback map[string]MidiFeedback
	cancel       context.CancelFunc
	done         chan struct{}
}

// NewEventPusher constructs an idle EventPusher; call Start to begin the
// emit loop.
func NewEventPusher() *EventPusher {
	return &EventPusher{
		latest:       map[string]interface{}{},
		midiFeedback: map[string]MidiFeedback{},
	}
}

// QueueStatus stages the latest PLAY-07 status snapshot (StatusSnapshot,
// svc_safety.go) for the next throttled push under the "status:update"
// event name -- SafetyService.pollStatus (svc_safety.go) is this event's
// own producer, calling QueueStatus once per statusPollInterval poll so a
// burst of polls between flushes coalesces into a single EventsEmit
// rather than one per poll.
func (p *EventPusher) QueueStatus(snapshot StatusSnapshot) {
	p.queue("status:update", snapshot)
}

// QueueMidiFeedback stages the latest D-09/D-10/D-11 soft-takeover
// feedback (MidiFeedback, svc_midi.go) for the next throttled push under
// the "midi:feedback" event name -- MidiService.dispatchToActiveSurface
// (svc_midi.go) is this event's own producer, calling QueueMidiFeedback
// once per arbitrated MIDI message so a fast fader sweep coalesces into
// eventsTickInterval-spaced pushes rather than one EventsEmit per message
// (06-RESEARCH.md Open Question 3: the crossing/arming decision itself
// stays unthrottled -- TakeoverState.Update runs on every message; only
// this visual-feedback push is throttled). Staged per mapping ID (WR-02),
// not under the single "midi:feedback" key latest's own queue() uses, so
// a surface with more than one active mapping (e.g. two faders touched
// within the same tick) has every distinct mapping's feedback survive to
// the next flush, instead of only the most-recently-queued mapping's.
func (p *EventPusher) QueueMidiFeedback(snapshot MidiFeedback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.midiFeedback[snapshot.MappingID] = snapshot
}

func (p *EventPusher) queue(eventName string, snapshot interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latest[eventName] = snapshot
}

// Start begins the fixed-cadence emit loop bound to ctx (Wails' OnStartup
// context, per runtime.EventsEmit's own contract). Calling Start again
// without an intervening Stop is a no-op.
func (p *EventPusher) Start(ctx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})
	if p.emit == nil {
		p.emit = defaultEmit
	}
	p.mu.Unlock()

	go p.run(runCtx)
}

func (p *EventPusher) run(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(eventsTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.flush(ctx)
		}
	}
}

func (p *EventPusher) flush(ctx context.Context) {
	p.mu.Lock()
	pending := p.latest
	p.latest = map[string]interface{}{}
	pendingMidi := p.midiFeedback
	p.midiFeedback = map[string]MidiFeedback{}
	emit := p.emit
	p.mu.Unlock()

	for name, snapshot := range pending {
		emit(ctx, name, snapshot)
	}
	// Every staged mapping's MidiFeedback is emitted individually under
	// the same "midi:feedback" event name (WR-02) -- the frontend's
	// onMidiFeedback subscribes once and expects one MidiFeedback per
	// EventsEmit call, so a tick with N distinct mappings staged now
	// produces N pushes instead of silently keeping only the last one.
	for _, snapshot := range pendingMidi {
		emit(ctx, "midi:feedback", snapshot)
	}
}

// Stop cancels the emit loop and waits for it to exit. Safe to call more
// than once or before Start.
func (p *EventPusher) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	done := p.done
	p.cancel = nil
	p.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}
