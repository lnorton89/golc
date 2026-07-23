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
