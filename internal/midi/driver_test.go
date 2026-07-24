// driver_test.go proves 06-08-PLAN.md Task 1's acceptance criteria against
// gitlab.com/gomidi/midi/v2/drivers/testdrv's mock driver -- never against
// midicatdrv and never against physical hardware (real per-device
// acceptance is gated by the open MIDI-HW-02 blocker, RESEARCH.md
// Environment Availability). This file must NEVER import midicatdrv
// (blank or named): that package's own init() shells out to `midicat
// version` and calls panic() -- not a returnable error -- when the binary
// is missing from PATH, which would crash this entire test binary (and,
// transitively, `go test ./internal/wails/...` and `go test ./...`) on any
// machine or CI runner without midicat.exe installed. driver.go's own doc
// comment records the same constraint; see 06-08-SUMMARY.md's Decisions
// Made section for the full analysis.
//
// Each test constructs its own isolated testdrv.New(name) instance rather
// than relying on testdrv's package-level self-registration (its init()
// registers one "testdrv"-named instance into gomidi's global driver
// registry) -- Driver.Open takes a drivers.In directly, so no test needs
// the global registry at all, keeping every test independent and safe to
// run in any order.
package midi

import (
	"testing"
	"time"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	"gitlab.com/gomidi/midi/v2/drivers/testdrv"
)

// TestDriverDecodesNoteOn proves Listen decodes a Note-on message into an
// Event carrying a Note ControlKey and velocity normalized to 0..1.
func TestDriverDecodesNoteOn(t *testing.T) {
	testDrv := testdrv.New("test-note-on")
	ins, _ := testDrv.Ins()
	outs, _ := testDrv.Outs()
	if err := outs[0].Open(); err != nil {
		t.Fatalf("out.Open(): %v", err)
	}

	d, err := Open(ins[0])
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	events, err := d.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer d.Close()

	if err := outs[0].Send(midi.NoteOn(2, 60, 100).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case evt := <-events:
		want := Event{Key: ControlKey{Channel: 2, Kind: Note, Number: 60}, Value: float64(100) / 127}
		if evt != want {
			t.Fatalf("event = %+v, want %+v", evt, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Note-on event")
	}
}

// TestDriverDecodesNoteOff proves Listen decodes a Note-off message into an
// Event carrying Value=0 (D-12: buttons act on press; a released Note
// carries no takeover-relevant physical value).
func TestDriverDecodesNoteOff(t *testing.T) {
	testDrv := testdrv.New("test-note-off")
	ins, _ := testDrv.Ins()
	outs, _ := testDrv.Outs()
	if err := outs[0].Open(); err != nil {
		t.Fatalf("out.Open(): %v", err)
	}

	d, err := Open(ins[0])
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	events, err := d.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer d.Close()

	if err := outs[0].Send(midi.NoteOff(3, 64).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case evt := <-events:
		want := Event{Key: ControlKey{Channel: 3, Kind: Note, Number: 64}, Value: 0}
		if evt != want {
			t.Fatalf("event = %+v, want %+v", evt, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Note-off event")
	}
}

// TestDriverDecodesControlChange proves Listen decodes a Control Change
// message into an Event carrying a ControlChange ControlKey and its value
// normalized to 0..1 -- the physical reading TakeoverState.Update
// consumes (RESEARCH.md Pattern 4).
func TestDriverDecodesControlChange(t *testing.T) {
	testDrv := testdrv.New("test-cc")
	ins, _ := testDrv.Ins()
	outs, _ := testDrv.Outs()
	if err := outs[0].Open(); err != nil {
		t.Fatalf("out.Open(): %v", err)
	}

	d, err := Open(ins[0])
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	events, err := d.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer d.Close()

	if err := outs[0].Send(midi.ControlChange(1, 74, 64).Bytes()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case evt := <-events:
		want := Event{Key: ControlKey{Channel: 1, Kind: ControlChange, Number: 74}, Value: float64(64) / 127}
		if evt != want {
			t.Fatalf("event = %+v, want %+v", evt, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ControlChange event")
	}
}

// TestDriverStatusOKUntilClosed proves Status()/Err() start OK and flip to
// Closed (with a GOLC_MIDI_PORT_CLOSED diagnostic) once Close is called --
// the Status()/Err() reachability surface RESEARCH.md's Standard Stack row
// requires, mirroring internal/artnet/interfacemgr.go's accessor shape.
func TestDriverStatusOKUntilClosed(t *testing.T) {
	testDrv := testdrv.New("test-status")
	ins, _ := testDrv.Ins()

	d, err := Open(ins[0])
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if d.Status() != DriverStatusOK {
		t.Fatalf("Status() = %v, want DriverStatusOK", d.Status())
	}
	if err := d.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}

	if _, err := d.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if d.Status() != DriverStatusClosed {
		t.Fatalf("Status() after Close = %v, want DriverStatusClosed", d.Status())
	}
	if err := d.Err(); err == nil {
		t.Fatal("Err() after Close = nil, want a GOLC_MIDI_PORT_CLOSED diagnostic")
	}
}

// TestDriverOpenRejectsNilPort proves Open fails fast with a diagnostic
// rather than constructing a Driver that would panic on first use.
func TestDriverOpenRejectsNilPort(t *testing.T) {
	if _, err := Open(nil); err == nil {
		t.Fatal("Open(nil) = nil error, want GOLC_MIDI_PORT_OPEN_FAILED")
	}
}

// TestDriverListensOnEveryWrappedPort proves a Driver wrapping more than
// one port (newDriver, backing OpenFirstAvailable) merges every port's
// events onto Listen's single channel -- the fix for a controller like the
// Novation Launch Control XL, which enumerates two separate MIDI input
// ports and sends its actual control data on whichever one matches its
// current hardware template. A message sent on either port must be
// observed; listening on only the first port (the pre-fix behavior) would
// miss whichever port this test sends on second.
func TestDriverListensOnEveryWrappedPort(t *testing.T) {
	firstDrv := testdrv.New("test-multiport-first")
	secondDrv := testdrv.New("test-multiport-second")
	firstIns, _ := firstDrv.Ins()
	secondIns, _ := secondDrv.Ins()
	firstOuts, _ := firstDrv.Outs()
	secondOuts, _ := secondDrv.Outs()
	if err := firstOuts[0].Open(); err != nil {
		t.Fatalf("first out.Open(): %v", err)
	}
	if err := secondOuts[0].Open(); err != nil {
		t.Fatalf("second out.Open(): %v", err)
	}

	d, err := newDriver([]drivers.In{firstIns[0], secondIns[0]})
	if err != nil {
		t.Fatalf("newDriver: %v", err)
	}
	events, err := d.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer d.Close()

	if err := firstOuts[0].Send(midi.NoteOn(1, 10, 100).Bytes()); err != nil {
		t.Fatalf("Send on first port: %v", err)
	}
	select {
	case evt := <-events:
		want := Event{Key: ControlKey{Channel: 1, Kind: Note, Number: 10}, Value: float64(100) / 127}
		if evt != want {
			t.Fatalf("event from first port = %+v, want %+v", evt, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event from the first port")
	}

	if err := secondOuts[0].Send(midi.ControlChange(2, 20, 64).Bytes()); err != nil {
		t.Fatalf("Send on second port: %v", err)
	}
	select {
	case evt := <-events:
		want := Event{Key: ControlKey{Channel: 2, Kind: ControlChange, Number: 20}, Value: float64(64) / 127}
		if evt != want {
			t.Fatalf("event from second port = %+v, want %+v", evt, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event from the second port -- a regression here means Listen went back to only the first port")
	}
}

// TestDriverOpenFirstAvailableUsesRegisteredDriver proves
// OpenFirstAvailable resolves a real port through gomidi's normal
// global-registry codepath (gomidi.GetInPorts -> drivers.Get -> the first
// registered driver) rather than a hand-rolled lookup -- testdrv's own
// package init() (triggered simply by this test file importing it, see
// this file's own doc comment) self-registers one "testdrv" instance
// globally, so this test binary always has exactly one driver registered
// and OpenFirstAvailable must succeed against it. The
// GOLC_MIDI_NO_PORTS_AVAILABLE error path (no driver registered at all) is
// exactly production's state before cmd/golc-desktop blank-imports
// midicatdrv -- not reproducible from within this package's own test
// binary once testdrv is imported, so it is not separately asserted here;
// Open(nil)'s GOLC_MIDI_PORT_OPEN_FAILED diagnostic (TestDriverOpenRejectsNilPort
// above) covers the same "no usable port" failure shape.
func TestDriverOpenFirstAvailableUsesRegisteredDriver(t *testing.T) {
	d, err := OpenFirstAvailable()
	if err != nil {
		t.Fatalf("OpenFirstAvailable() = %v, want a Driver wrapping testdrv's auto-registered port", err)
	}
	if d.Status() != DriverStatusOK {
		t.Fatalf("Status() = %v, want DriverStatusOK", d.Status())
	}
}
