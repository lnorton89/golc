// svc_playback.go declares PlaybackService (06-04-PLAN.md Task 1): the
// Wails-bound service struct 06-06-PLAN.md fills with real methods for
// on-screen playback (scene switch, layer enable/disable, BPM/tap tempo,
// transport -- PLAY-01) issued as typed commands through the existing
// internal/command registry, exactly like every other client
// (06-RESEARCH.md Architectural Responsibility Map: "no new playback
// authority introduced"). This file only registers the struct and one
// stub method so cmd/golc-desktop's wails.Run Bind list is complete and
// the build stays green; 06-06 is the plan that gives these methods real
// behavior.
package wails

// PlaybackService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. 06-06-PLAN.md fills this file's real methods.
type PlaybackService struct {
	pipeName string
}

// NewPlaybackService constructs a PlaybackService targeting pipeName.
func NewPlaybackService(pipeName string) *PlaybackService {
	return &PlaybackService{pipeName: pipeName}
}

// Ping is a placeholder Wails-bound method proving the service registers
// and builds; 06-06 replaces/extends this file with the real playback
// commands.
func (s *PlaybackService) Ping() Result {
	return Result{ExitCode: 2, Stderr: "GOLC_WAILS_NOT_IMPLEMENTED: PlaybackService is a 06-04 scaffold stub; real methods land in 06-06"}
}
