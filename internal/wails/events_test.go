// events_test.go proves EventPusher's per-mapping MIDI feedback staging
// (WR-02, gap-closure code review fix): a single flush tick must deliver
// every distinct mapping's staged MidiFeedback, not just the most-
// recently-queued one, while QueueStatus's own single-value-per-event-name
// "status:update" slot keeps its pre-existing behavior unchanged.
package wails

import (
	"context"
	"sync"
	"testing"
)

// TestEventPusherFlushDeliversEveryStagedMappingsMidiFeedback proves that
// staging MidiFeedback for two distinct mapping IDs within the same tick
// (before flush runs) delivers BOTH snapshots on the next flush -- not just
// the second (most-recently-queued) one, which is the bug WR-02 reports
// against the pre-fix single-key-per-event-name p.latest map.
func TestEventPusherFlushDeliversEveryStagedMappingsMidiFeedback(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []MidiFeedback
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "midi:feedback" {
			return
		}
		if fb, ok := data[0].(MidiFeedback); ok {
			mu.Lock()
			pushed = append(pushed, fb)
			mu.Unlock()
		}
	}

	// Two distinct mappings both produce feedback inside the same
	// eventsTickInterval window, before any flush has run.
	p.QueueMidiFeedback(MidiFeedback{MappingID: "mapping-a", Kind: "control_change", Physical: 0.25})
	p.QueueMidiFeedback(MidiFeedback{MappingID: "mapping-b", Kind: "note", Physical: 1.0})

	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 2 {
		t.Fatalf("expected both staged mappings' feedback to survive one flush, got %d: %+v", len(pushed), pushed)
	}
	byMapping := map[string]MidiFeedback{}
	for _, fb := range pushed {
		byMapping[fb.MappingID] = fb
	}
	if fb, ok := byMapping["mapping-a"]; !ok || fb.Physical != 0.25 {
		t.Fatalf("expected mapping-a's feedback to survive the flush, got %+v", byMapping)
	}
	if fb, ok := byMapping["mapping-b"]; !ok || fb.Physical != 1.0 {
		t.Fatalf("expected mapping-b's feedback to survive the flush, got %+v", byMapping)
	}
}

// TestEventPusherFlushOverwritesSameMappingWithLatest proves the intended
// coalescing behavior is preserved per-mapping: two updates to the SAME
// mapping ID within one tick still collapse to the latest value only
// (never queues an unbounded backlog per mapping).
func TestEventPusherFlushOverwritesSameMappingWithLatest(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []MidiFeedback
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "midi:feedback" {
			return
		}
		if fb, ok := data[0].(MidiFeedback); ok {
			mu.Lock()
			pushed = append(pushed, fb)
			mu.Unlock()
		}
	}

	p.QueueMidiFeedback(MidiFeedback{MappingID: "mapping-a", Physical: 0.1})
	p.QueueMidiFeedback(MidiFeedback{MappingID: "mapping-a", Physical: 0.9})

	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 1 {
		t.Fatalf("expected exactly one coalesced push for a single mapping updated twice, got %d: %+v", len(pushed), pushed)
	}
	if pushed[0].Physical != 0.9 {
		t.Fatalf("expected the latest value (0.9) to survive coalescing, got %+v", pushed[0])
	}
}

// TestEventPusherFlushKeepsStatusUpdateSingleValueBehavior proves
// QueueStatus's pre-existing single-value-per-event-name "status:update"
// slot is unaffected by the WR-02 per-mapping MIDI feedback staging added
// alongside it.
func TestEventPusherFlushKeepsStatusUpdateSingleValueBehavior(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []StatusSnapshot
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "status:update" {
			return
		}
		if snap, ok := data[0].(StatusSnapshot); ok {
			mu.Lock()
			pushed = append(pushed, snap)
			mu.Unlock()
		}
	}

	p.QueueStatus(StatusSnapshot{BPM: 120})
	p.QueueStatus(StatusSnapshot{BPM: 128})

	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 1 || pushed[0].BPM != 128 {
		t.Fatalf("expected exactly one coalesced status:update push carrying the latest BPM, got %+v", pushed)
	}
}

// TestQueueScriptEventStagesFiveDistinctEventsAndEmitsAllInSeqOrder covers
// 08-08-PLAN.md Task 3's exact <behavior> requirement: QueueScriptEvent
// staging N distinct events within one tick results in N EventsEmit calls
// under "script:event", in Seq order -- never coalesced.
func TestQueueScriptEventStagesFiveDistinctEventsAndEmitsAllInSeqOrder(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []ScriptEventView
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "script:event" {
			return
		}
		if view, ok := data[0].(ScriptEventView); ok {
			mu.Lock()
			pushed = append(pushed, view)
			mu.Unlock()
		}
	}

	for i := int64(1); i <= 5; i++ {
		p.QueueScriptEvent(ScriptEventView{Seq: i, Kind: "script.log", Message: "line"})
	}

	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 5 {
		t.Fatalf("expected exactly 5 emit calls for 5 staged distinct events, got %d: %+v", len(pushed), pushed)
	}
	for i, view := range pushed {
		wantSeq := int64(i + 1)
		if view.Seq != wantSeq {
			t.Fatalf("pushed[%d].Seq = %d, want %d (Seq order)", i, view.Seq, wantSeq)
		}
	}
}

// TestQueueScriptEventOverflowEmitsGapEventBeforeSurvivingEvents covers:
// "The staging buffer is bounded; when it overflows within a tick, the
// oldest staged events are dropped and a single synthetic gap event
// carrying the dropped count and the resulting Seq discontinuity is
// emitted, so the frontend can resync rather than silently miss lines."
func TestQueueScriptEventOverflowEmitsGapEventBeforeSurvivingEvents(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []ScriptEventView
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "script:event" {
			return
		}
		if view, ok := data[0].(ScriptEventView); ok {
			mu.Lock()
			pushed = append(pushed, view)
			mu.Unlock()
		}
	}

	overflowBy := 3
	for i := 0; i < maxStagedScriptEvents+overflowBy; i++ {
		p.QueueScriptEvent(ScriptEventView{Seq: int64(i + 1), Kind: "script.log"})
	}

	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != maxStagedScriptEvents+1 {
		t.Fatalf("expected %d entries (1 gap event + %d surviving), got %d",
			maxStagedScriptEvents+1, maxStagedScriptEvents, len(pushed))
	}
	if pushed[0].Kind != "script.gap" || pushed[0].GapCount != overflowBy {
		t.Fatalf("expected the first pushed entry to be a gap event carrying GapCount=%d, got %+v", overflowBy, pushed[0])
	}
	for i := 1; i < len(pushed); i++ {
		if pushed[i].Kind == "script.gap" {
			t.Fatalf("expected exactly one gap event, found a second at index %d: %+v", i, pushed[i])
		}
	}
	// The surviving events are the newest overflowBy+1..maxStagedScriptEvents+overflowBy
	// (the oldest overflowBy were dropped) -- a real Seq discontinuity a
	// consumer can detect.
	firstSurvivingSeq := pushed[1].Seq
	if firstSurvivingSeq != int64(overflowBy+1) {
		t.Fatalf("expected the first surviving event's Seq to be %d (oldest %d dropped), got %d", overflowBy+1, overflowBy, firstSurvivingSeq)
	}
}

// TestQueueScriptEventNoOverflowEmitsNoGapEvent proves a tick that never
// exceeds maxStagedScriptEvents emits no synthetic gap event at all.
func TestQueueScriptEventNoOverflowEmitsNoGapEvent(t *testing.T) {
	p := NewEventPusher()

	var mu sync.Mutex
	var pushed []ScriptEventView
	p.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName != "script:event" {
			return
		}
		if view, ok := data[0].(ScriptEventView); ok {
			mu.Lock()
			pushed = append(pushed, view)
			mu.Unlock()
		}
	}

	p.QueueScriptEvent(ScriptEventView{Seq: 1, Kind: "script.log"})
	p.flush(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if len(pushed) != 1 {
		t.Fatalf("expected exactly 1 pushed event, got %d: %+v", len(pushed), pushed)
	}
	if pushed[0].Kind == "script.gap" {
		t.Fatal("expected no gap event when the staging bound was never exceeded")
	}
}
