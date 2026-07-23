// driver.go implements 06-08-PLAN.md Task 1's live MIDI device layer:
// Driver wraps one already-resolved gomidi drivers.In port and decodes
// incoming Note-on/Note-off/Control-Change messages into an Event
// (ControlKey + normalized value) delivered on a channel -- the source
// CaptureCandidate (learn.go) and TakeoverState.Update (takeover.go, both
// 06-03) consume. Driver deliberately never imports a specific backend
// driver package (midicatdrv/testdrv) itself: the caller resolves a
// drivers.In via whichever driver has been registered (blank-imported)
// elsewhere in the running process -- production wires midicatdrv
// (cmd/golc-desktop/main.go, CGo-free per RESEARCH.md Pitfall 3), tests
// use gomidi's testdrv mock (RESEARCH.md Environment Availability: "unit
// testing can proceed against testdrv... unaffected by the open
// MIDI-HW-02 blocker"). This decoupling keeps `go test ./internal/midi/...`
// safe to run without midicat.exe on PATH or any physical MIDI hardware --
// see driver_test.go's doc comment and this plan's SUMMARY.md for why
// midicatdrv itself must NEVER be imported (blank or named) from this
// package: its own package init() shells out to `midicat.exe version` and
// calls panic() (not a returnable error) when the binary is missing,
// which would otherwise crash every test binary that transitively imports
// internal/midi.
//
// Status()/Err() mirror internal/artnet/interfacemgr.go's accessor shape
// (06-PATTERNS.md: "skim interfacemgr.go's Status()/Err() reconnect
// surface only, not its internals") and follow the same
// terminal-until-reconfigured philosophy as InterfaceManager (04-02-PLAN.md
// CONTEXT D-05): a lost port is reported, never silently auto-recovered
// onto a different port.
package midi

import (
	"fmt"
	"sync"
	"sync/atomic"

	gomidi "gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
)

// midiEventBufferSize bounds Listen's delivery channel. A full channel
// means the consumer (MidiService's dispatch loop, 06-08 Task 2) has
// stalled; decode drops rather than blocks gomidi's own callback
// goroutine, which must never be blocked (mirrors internal/artnet/
// worker.go's own "the tick goroutine must never block" discipline).
const midiEventBufferSize = 64

// normalizeValue is the fixed 0..1 normalization gomidi's 7-bit
// (0-127) Note-velocity/CC-value range uses across every Event this
// package produces -- both Note and ControlChange share the identical
// 0-127 wire range, so one shared conversion is correct for both.
func normalizeValue(raw uint8) float64 {
	return float64(raw) / 127
}

// Event is one decoded MIDI Note/CC message: Key is the ControlKey
// identity (learn.go/operatorsurface.MidiMapping's live-side counterpart),
// Value is the normalized 0..1 reading -- velocity/127 for Note-on, 0 for
// Note-off (D-12: buttons act on press, not on the fader-only takeover
// path), and CC-value/127 for ControlChange (the physical reading
// TakeoverState.Update consumes, RESEARCH.md Pattern 4).
type Event struct {
	Key   ControlKey
	Value float64
}

// DriverStatus is Driver's readable health state (mirrors
// internal/artnet/interfacemgr.go's InterfaceStatus).
type DriverStatus int32

const (
	// DriverStatusOK reports the port is open and, if Listen was called,
	// has not reported a listen error.
	DriverStatusOK DriverStatus = iota
	// DriverStatusClosed reports the port was explicitly closed or the
	// underlying listen reported an error (port unplugged/unreachable).
	// No code here ever re-opens or switches to a different port
	// automatically (mirrors CONTEXT D-05's "loss is terminal-until-
	// reconfigured" convention already established for Art-Net
	// interfaces) -- a caller must construct a fresh Driver to retry.
	DriverStatusClosed
)

// String renders status for logging/CLI display.
func (s DriverStatus) String() string {
	switch s {
	case DriverStatusOK:
		return "ok"
	case DriverStatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Driver wraps one already-resolved gomidi input port. Construct via Open
// (an explicit port, e.g. from a test's testdrv instance) or
// OpenFirstAvailable (production: the first port reported by whichever
// driver was registered elsewhere in the process).
type Driver struct {
	in drivers.In

	mu   sync.Mutex
	stop func()

	status atomic.Int32
}

// Open wraps in, an already-resolved gomidi drivers.In port, exposing it
// through Driver's decode/Status surface. Open does not itself open the
// port (Listen does, mirroring gomidi.ListenTo's own "opens if not already
// open" contract) and does not care which driver produced in -- callers
// resolve the port themselves via whichever driver is registered in the
// running process.
func Open(in drivers.In) (*Driver, error) {
	if in == nil {
		return nil, fmt.Errorf("GOLC_MIDI_PORT_OPEN_FAILED: nil MIDI input port")
	}
	d := &Driver{in: in}
	d.status.Store(int32(DriverStatusOK))
	return d, nil
}

// OpenFirstAvailable resolves and wraps the first MIDI input port reported
// by whichever driver has been registered in the running process
// (production: midicatdrv, blank-imported by cmd/golc-desktop; tests never
// call this -- they construct Driver via Open against a testdrv port
// directly). Returns GOLC_MIDI_NO_PORTS_AVAILABLE when no driver is
// registered or no input port is present -- an expected, non-fatal
// condition: MIDI hardware remains optional (PROJECT.md "Keyboard and
// on-screen controls must provide the full playback workflow while MIDI
// hardware remains undecided"), never a reason to fail startup.
func OpenFirstAvailable() (*Driver, error) {
	ports := gomidi.GetInPorts()
	if len(ports) == 0 {
		return nil, fmt.Errorf("GOLC_MIDI_NO_PORTS_AVAILABLE: no MIDI input ports were found")
	}
	return Open(ports[0])
}

// Listen begins decoding incoming messages from d's port, delivering every
// Note-on/Note-off/Control-Change message as an Event on the returned
// channel (buffered midiEventBufferSize; a stalled consumer causes drops,
// never a block of gomidi's own callback goroutine). The channel is closed
// when Close is called. A port-level listen error (drivers.ListenConfig's
// OnErr) marks d DriverStatusClosed -- detected, never silently
// auto-recovered (CONTEXT D-05 convention). Listen must be called at most
// once per Driver.
func (d *Driver) Listen() (<-chan Event, error) {
	events := make(chan Event, midiEventBufferSize)

	stop, err := gomidi.ListenTo(d.in, func(msg gomidi.Message, _ int32) {
		evt, ok := decode(msg)
		if !ok {
			return
		}
		select {
		case events <- evt:
		default:
		}
	}, gomidi.HandleError(func(error) {
		d.status.Store(int32(DriverStatusClosed))
	}))
	if err != nil {
		d.status.Store(int32(DriverStatusClosed))
		return nil, fmt.Errorf("GOLC_MIDI_LISTEN_FAILED: %v", err)
	}

	d.mu.Lock()
	d.stop = func() {
		stop()
		close(events)
	}
	d.mu.Unlock()

	return events, nil
}

// decode converts a raw gomidi Message into an Event, reporting ok=false
// for any message kind other than Note-on/Note-off/Control-Change --
// ControlKey supports exactly those two MessageKinds (learn.go), so every
// other message (SysEx, realtime, pitchbend, ...) is intentionally
// dropped here rather than reaching CaptureCandidate/TakeoverState.Update.
func decode(msg gomidi.Message) (Event, bool) {
	var channel, key, velocity uint8
	if msg.GetNoteOn(&channel, &key, &velocity) {
		return Event{
			Key:   ControlKey{Channel: int(channel), Kind: Note, Number: int(key)},
			Value: normalizeValue(velocity),
		}, true
	}
	if msg.GetNoteOff(&channel, &key, &velocity) {
		return Event{
			Key:   ControlKey{Channel: int(channel), Kind: Note, Number: int(key)},
			Value: 0,
		}, true
	}
	var controller, value uint8
	if msg.GetControlChange(&channel, &controller, &value) {
		return Event{
			Key:   ControlKey{Channel: int(channel), Kind: ControlChange, Number: int(controller)},
			Value: normalizeValue(value),
		}, true
	}
	return Event{}, false
}

// Status returns d's current reachability (mirrors internal/artnet/
// interfacemgr.go's Status() shape). Safe to call from any goroutine.
func (d *Driver) Status() DriverStatus {
	return DriverStatus(d.status.Load())
}

// Err returns nil when Status is DriverStatusOK, or a
// GOLC_MIDI_PORT_CLOSED diagnostic identifying the port otherwise
// (mirrors internal/artnet/interfacemgr.go's Err() diagnostic
// convention).
func (d *Driver) Err() error {
	if d.Status() == DriverStatusClosed {
		return fmt.Errorf("GOLC_MIDI_PORT_CLOSED: MIDI input port %q is no longer reachable", d.in.String())
	}
	return nil
}

// Close stops listening (if Listen was called) and closes the underlying
// port. No code here ever selects or reconnects to a different port
// (CONTEXT D-05 convention) -- a caller must construct a fresh Driver via
// Open/OpenFirstAvailable to retry.
func (d *Driver) Close() error {
	d.mu.Lock()
	stop := d.stop
	d.stop = nil
	d.mu.Unlock()
	if stop != nil {
		stop()
	}
	d.status.Store(int32(DriverStatusClosed))
	return d.in.Close()
}
