// frame.go declares the Frame type (CONTEXT SCEN-09, 03-RESEARCH.md
// Pattern 3): the pure output of Evaluate(plan, position) -- one computed
// snapshot of every fixture instance's resolved semantic attribute values
// at a single musical position. The engine (03-07 Task 2) publishes each
// tick's Frame via a lock-free atomic.Pointer[Frame]; downstream consumers
// (Phase 4 Art-Net worker, Phase 6 UI, Phase 7 API) Load it independently,
// never blocking or backpressuring the tick loop.
package playback

import (
	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/scene"
)

// Frame is one fixture-instance-keyed snapshot of resolved semantic
// attribute values (SCEN-09): the deterministic output of
// Evaluate(plan, position) for a single (CompiledPlan, MusicalPosition)
// pair. encoding/json sorts map keys deterministically (mirrors
// internal/strictjson.CanonicalEncode's own guarantee), so repeated
// encodes of an identical Frame are byte-identical.
type Frame struct {
	Values map[uuid.UUID]scene.AttributeSet `json:"values,omitempty"`
}
