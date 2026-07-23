// svc_safety.go declares SafetyService (06-04-PLAN.md Task 1): the
// Wails-bound service struct 06-05-PLAN.md fills with real methods calling
// the daemon's "artnet safety blackout/stop-all/revoke-automation" and
// "artnet master set" routes (06-02-PLAN.md) -- the same routes
// hotkey.go's OS-level callbacks already call directly. This file only
// registers the struct and one stub method so cmd/golc-desktop's
// wails.Run Bind list is complete and the build stays green; 06-05 is the
// plan that gives these methods real behavior.
package wails

// SafetyService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. 06-05-PLAN.md fills this file's real methods
// (Blackout/StopAll/RevokeAutomation/MasterSet) and the live status
// projection.
type SafetyService struct {
	pipeName string
}

// NewSafetyService constructs a SafetyService targeting pipeName.
func NewSafetyService(pipeName string) *SafetyService {
	return &SafetyService{pipeName: pipeName}
}

// Ping is a placeholder Wails-bound method proving the service registers
// and builds; 06-05 replaces/extends this file with the real safety-
// cluster calls.
func (s *SafetyService) Ping() Result {
	return Result{ExitCode: 2, Stderr: "GOLC_WAILS_NOT_IMPLEMENTED: SafetyService is a 06-04 scaffold stub; real methods land in 06-05"}
}
