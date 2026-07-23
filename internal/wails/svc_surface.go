// svc_surface.go declares SurfaceService (06-04-PLAN.md Task 1): the
// Wails-bound service struct 06-07-PLAN.md fills with real methods for
// the operator-surface builder (D-01..D-04) and its constrained
// visible-but-locked authorization (PLAY-03), backed by
// internal/operatorsurface and command.Authorize (06-01-PLAN.md). This
// file only registers the struct and one stub method so
// cmd/golc-desktop's wails.Run Bind list is complete and the build stays
// green; 06-07 is the plan that gives these methods real behavior.
package wails

// SurfaceService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. 06-07-PLAN.md fills this file's real methods.
type SurfaceService struct {
	pipeName string
}

// NewSurfaceService constructs a SurfaceService targeting pipeName.
func NewSurfaceService(pipeName string) *SurfaceService {
	return &SurfaceService{pipeName: pipeName}
}

// Ping is a placeholder Wails-bound method proving the service registers
// and builds; 06-07 replaces/extends this file with the real
// operator-surface commands.
func (s *SurfaceService) Ping() Result {
	return Result{ExitCode: 2, Stderr: "GOLC_WAILS_NOT_IMPLEMENTED: SurfaceService is a 06-04 scaffold stub; real methods land in 06-07"}
}
