// Package deskmidi models direct MIDI Note/CC mappings onto Desk workspace
// faders (raw per-instance/per-capability DMX channel overrides,
// internal/wails/svc_desk.go), independent of internal/operatorsurface's
// Surface/ControlRef system entirely: a Desk fader is learnable without
// first being assigned to any operator surface, and every Mapping in this
// package lives in one show-global set (show.State.DeskMidiMappings), never
// scoped per-surface. This package deliberately never imports
// operatorsurface, mirroring internal/midi's own "pure, decoupled" doc
// comment -- the two mapping systems are independent by design, each with
// its own (channel, kind, number) conflict namespace.
package deskmidi
