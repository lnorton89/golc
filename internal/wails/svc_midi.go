// svc_midi.go declares MidiService (06-04-PLAN.md Task 1): the Wails-bound
// service struct 06-08-PLAN.md fills with real methods for generic MIDI
// Note/CC learn and soft-takeover feedback (PLAY-04/05, D-05..D-12),
// backed by the internal/midi package 06-08 introduces. This file only
// registers the struct and one stub method so cmd/golc-desktop's
// wails.Run Bind list is complete and the build stays green; 06-08 is the
// plan that gives these methods real behavior.
package wails

// MidiService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. 06-08-PLAN.md fills this file's real methods.
type MidiService struct {
	pipeName string
}

// NewMidiService constructs a MidiService targeting pipeName.
func NewMidiService(pipeName string) *MidiService {
	return &MidiService{pipeName: pipeName}
}

// Ping is a placeholder Wails-bound method proving the service registers
// and builds; 06-08 replaces/extends this file with the real MIDI
// learn/mapping commands.
func (s *MidiService) Ping() Result {
	return Result{ExitCode: 2, Stderr: "GOLC_WAILS_NOT_IMPLEMENTED: MidiService is a 06-04 scaffold stub; real methods land in 06-08"}
}
