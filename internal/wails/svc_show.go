// svc_show.go fills ShowService, the Wails binding backing the Overview and
// Save & Recovery workspaces (application-shell-navigation.md's "Show"
// group): Save/SaveAs execute the matching already-implemented, already-
// tested "show save"/"show save-as" CLI route (internal/command/show.go)
// via command.NewDefaultCommandRegistry -- exactly the SurfaceService/
// ProgrammingService/FixturePatchService pattern (svc_surface.go/
// svc_programming.go/svc_fixturepatch.go) this file mirrors -- so there is
// only one show-save implementation in this codebase, never a second one
// duplicated for the GUI.
//
// Inspect/Diagnose/DetectRecoveryPoints read the ShowState (or its
// dedicated read-only internal/show helpers) directly and project into a
// JSON-safe view, mirroring ListProgramming/ListPatch/ListSurfaces's
// identical "no registered read route returns structured data" rationale --
// "show inspect"/"show diagnose"'s own CLI Result carries JSON in Stdout,
// but shelling out and re-parsing text here would be a second, needless
// implementation of the same projection.
//
// AcceptRecoveryPoint/DiscardRecoveryPoints call internal/show's own
// RecoveryPoint functions directly rather than through a CLI route: no
// standalone "show accept-recovery"/"show discard-recovery" route exists --
// both are only ever reachable bundled inside "show open"'s
// --accept-recovery/--discard-recovery flags (internal/command/show.go's
// runShowOpen) -- so calling show.AcceptRecoveryPoint/
// show.DiscardRecoveryPoints here is calling the exact same canonical
// function runShowOpen itself calls, never a duplicate reimplementation.
// This file intentionally does not bind "show open" itself: the desktop
// app resolves exactly one show path at startup (cmd/golc-desktop/main.go's
// GOLC_DESKTOP_SHOW), with no file-picker/"open a different show" flow yet
// -- offering/accepting/discarding recovery points against that one
// resolved path, and surfacing a migration-required note via Diagnose, is
// the complete Save & Recovery scope this round.
package wails

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/show"
)

// ShowService is bound to the frontend via cmd/golc-desktop/main.go's
// options.App{Bind: [...]}. root/showPath are the exact ShowState location
// every method acts against (mirrors SurfaceService/ProgrammingService's
// own fields).
type ShowService struct {
	pipeName string
	root     string
	showPath string
}

// NewShowService constructs a ShowService targeting pipeName (reserved,
// unused by this ShowState-only CRUD -- mirrors SurfaceService/
// ProgrammingService's own unused pipeName field) and the ShowState at
// showPath, resolved against root.
func NewShowService(pipeName, root, showPath string) *ShowService {
	return &ShowService{pipeName: pipeName, root: root, showPath: showPath}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_surface.go/svc_programming.go's identical
// helper).
func (s *ShowService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// Save re-saves the working show in place via "show save", bumping Revision
// and writing a fresh recovery point (SHOW-01/SHOW-03).
func (s *ShowService) Save() Result {
	return s.execute([]string{"show", "save", "--show", s.showPath})
}

// SaveAs saves a copy of the working show to destPath via "show save-as"
// without mutating the working show (SHOW-01).
func (s *ShowService) SaveAs(destPath string) Result {
	return s.execute([]string{"show", "save-as", "--show", s.showPath, "--to", destPath})
}

// ShowInspectPoolView is one Inspect row summarizing a logical pool.
type ShowInspectPoolView struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	RequiredCapabilities []string `json:"requiredCapabilities"`
	MemberCount          int      `json:"memberCount"`
}

// ShowInspectDeploymentView is one Inspect row summarizing a deployment.
type ShowInspectDeploymentView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Active        bool   `json:"active"`
	InstanceCount int    `json:"instanceCount"`
}

// ShowInspectView is Inspect's return shape: the working show's identity
// (path, schema/revision) plus its pools/deployments, the same allowlisted
// projection "show inspect" already prints, reshaped to this package's
// camelCase JSON convention (mirrors ProgrammingView/ArtnetStatusView's own
// convention).
type ShowInspectView struct {
	ShowPath      string                      `json:"showPath"`
	SchemaVersion int                         `json:"schemaVersion"`
	Revision      int                         `json:"revision"`
	Pools         []ShowInspectPoolView       `json:"pools"`
	Deployments   []ShowInspectDeploymentView `json:"deployments"`
}

// Inspect reads the working show read-only (show.LoadForRead, tolerating a
// newer-than-supported schema_version exactly like "show inspect"/"show
// export"/"show diagnose" all do -- D-10) and projects it into
// ShowInspectView for OverviewWorkspace.tsx.
func (s *ShowService) Inspect() (ShowInspectView, error) {
	state, err := show.LoadForRead(s.root, s.showPath)
	if err != nil {
		return ShowInspectView{}, err
	}

	pools := make([]ShowInspectPoolView, 0, len(state.Pools))
	for _, p := range state.Pools {
		capabilities := make([]string, 0, len(p.RequiredCapabilities))
		for _, capabilityType := range p.RequiredCapabilities {
			capabilities = append(capabilities, string(capabilityType))
		}
		pools = append(pools, ShowInspectPoolView{
			ID:                   p.ID.String(),
			Name:                 p.Name,
			RequiredCapabilities: capabilities,
			MemberCount:          len(p.Members),
		})
	}

	deployments := make([]ShowInspectDeploymentView, 0, len(state.Deployments))
	for _, d := range state.Deployments {
		deployments = append(deployments, ShowInspectDeploymentView{
			ID:            d.ID.String(),
			Name:          d.Name,
			Active:        d.Active,
			InstanceCount: len(d.Instances),
		})
	}

	return ShowInspectView{
		ShowPath:      s.showPath,
		SchemaVersion: state.SchemaVersion,
		Revision:      state.Revision,
		Pools:         pools,
		Deployments:   deployments,
	}, nil
}

// DiagnosticReportView reshapes show.DiagnosticReport into this package's
// camelCase JSON convention (show.DiagnosticReport's own tags are
// snake_case, matching the CLI's JSON output convention instead).
// FileLevelIssues deliberately carries no "omitempty": Wails marshals every
// bound method's return value through plain encoding/json.Marshal (internal/
// frontend/dispatcher/calls.go), which drops an "omitempty" slice field
// from the JSON entirely whenever it's empty -- leaving
// DiagnosticsWorkspace.tsx's report.fileLevelIssues undefined on the
// overwhelmingly common "healthy show" case (show.Diagnose's own
// FileLevelIssues stays a nil slice when integrity_check reports nothing).
// undefined.length then throws with no error boundary anywhere in this app
// (main.tsx has none), unmounting the whole React tree -- a blank window
// with zero on-screen diagnostic. Diagnose (below) also normalizes a nil
// slice to a non-nil empty one for the identical reason: encoding/json
// marshals a nil slice as JSON "null" even without "omitempty", and
// "null.length" throws exactly the same way "undefined.length" does.
// TestShowServiceDiagnosticReportNeverOmitsOrNullsFileLevelIssues pins this
// contract at the actual JSON-marshal boundary Wails uses -- the level a
// hand-authored frontend test mock can never exercise, since a mock never
// round-trips through real Go JSON encoding.
type DiagnosticReportView struct {
	FileLevelIssues   []string `json:"fileLevelIssues"`
	StructuralOK      bool     `json:"structuralOk"`
	StructuralError   string   `json:"structuralError,omitempty"`
	MigrationRequired bool     `json:"migrationRequired"`
	SchemaVersion     int      `json:"schemaVersion"`
	Revision          int      `json:"revision"`
}

// Diagnose runs the same combined file-level + structural health check
// "show diagnose" runs (show.Diagnose is read-only and never runs on the
// everyday open/save path -- D-12), for DiagnosticsWorkspace.tsx and for
// SaveRecoveryWorkspace.tsx's migration-required note.
func (s *ShowService) Diagnose() (DiagnosticReportView, error) {
	report, err := show.Diagnose(s.root, s.showPath)
	if err != nil {
		return DiagnosticReportView{}, err
	}
	fileLevelIssues := report.FileLevelIssues
	if fileLevelIssues == nil {
		fileLevelIssues = []string{}
	}
	return DiagnosticReportView{
		FileLevelIssues:   fileLevelIssues,
		StructuralOK:      report.StructuralOK,
		StructuralError:   report.StructuralError,
		MigrationRequired: report.MigrationRequired,
		SchemaVersion:     report.SchemaVersion,
		Revision:          report.Revision,
	}, nil
}

// RecoveryPointView reshapes show.RecoveryPoint into this package's
// camelCase JSON convention.
type RecoveryPointView struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"createdAt"`
	Revision  int    `json:"revision"`
}

// DetectRecoveryPoints returns every currently offered interrupted-session
// recovery point (SHOW-04, newest-first), never mutating anything -- the
// same read show.DetectRecoveryPoints performs for "show open"'s own offer.
func (s *ShowService) DetectRecoveryPoints() ([]RecoveryPointView, error) {
	points, err := show.DetectRecoveryPoints(s.root, s.showPath)
	if err != nil {
		return nil, err
	}
	views := make([]RecoveryPointView, 0, len(points))
	for _, point := range points {
		views = append(views, RecoveryPointView{ID: point.ID, CreatedAt: point.CreatedAt, Revision: point.Revision})
	}
	return views, nil
}

// AcceptRecoveryPoint promotes the offered recovery point identified by id
// into the working show (show.AcceptRecoveryPoint refuses a stale/unknown
// id with GOLC_SHOW_RECOVERY_NOT_FOUND before ever decoding anything --
// this method never re-implements that guard).
func (s *ShowService) AcceptRecoveryPoint(id int) Result {
	if err := show.AcceptRecoveryPoint(s.root, s.showPath, id); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	return Result{Stdout: fmt.Sprintf("GOLC_SHOW_RECOVERY_ACCEPTED: recovery point %d applied\n", id)}
}

// DiscardRecoveryPoints deletes every currently offered recovery point
// (show.DiscardRecoveryPoints), an explicit action this method never calls
// implicitly on the caller's behalf.
func (s *ShowService) DiscardRecoveryPoints() Result {
	if err := show.DiscardRecoveryPoints(s.root, s.showPath); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	return Result{Stdout: "GOLC_SHOW_RECOVERY_DISCARDED: offered recovery point(s) removed\n"}
}

// maxUploadedImageBytes bounds UploadImage: generous enough for even a
// sizable animated GIF (this feature's own explicit "support most images
// including animated gif" requirement), small enough that one oversized
// pick can't balloon the .golc file or the IPC round-trip carrying it back
// as a base64 data URI without a limit at all.
const maxUploadedImageBytes = 20 * 1024 * 1024 // 20 MiB

// AssetUploadView is UploadImage's return shape: the newly stored asset's
// own id (what FixtureStyle.backgroundImageAssetID persists) plus a
// ready-to-use data: URI, so the calling modal can preview the image
// immediately without a separate GetImageDataURI round trip for the exact
// bytes it just uploaded.
type AssetUploadView struct {
	ID      string `json:"id"`
	DataURI string `json:"dataUri"`
}

// detectImageMimeType resolves path's own MIME type primarily by file
// extension (Go's standard mime.TypeByExtension table already covers
// every format imageFileFilter offers, .svg included -- content-sniffing
// alone would not, since net/http.DetectContentType has no XML/SVG
// signature) with content-sniffing as the fallback for a recognized
// extension mime.TypeByExtension doesn't have registered on this OS.
func detectImageMimeType(path string, data []byte) string {
	if byExt := mime.TypeByExtension(filepath.Ext(path)); byExt != "" {
		return byExt
	}
	sniffLen := len(data)
	if sniffLen > 512 {
		sniffLen = 512
	}
	return http.DetectContentType(data[:sniffLen])
}

// UploadImage reads path's own bytes (a path this same operator just chose
// via App.PickImageFile's native dialog -- FixtureStyleModal.tsx's own
// "Choose Image" button flow) and stores them as a new show.SaveAsset row,
// keyed by a freshly generated uuid.NewV7() id. Refuses anything over
// maxUploadedImageBytes or whose detected MIME type is not image/* --
// this boundary validates rather than trusting the caller, the same
// discipline LaunchEasterEggExecutable's own doc comment describes for a
// frontend-reachable IPC method fed an operator-chosen path.
func (s *ShowService) UploadImage(path string) (AssetUploadView, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return AssetUploadView{}, errors.New("GOLC_WAILS_ASSET_PATH_EMPTY: an image path is required")
	}
	info, statErr := os.Stat(trimmed)
	if statErr != nil || info.IsDir() {
		return AssetUploadView{}, fmt.Errorf("GOLC_WAILS_ASSET_PATH_NOT_FOUND: %q", trimmed)
	}
	if info.Size() > maxUploadedImageBytes {
		return AssetUploadView{}, fmt.Errorf(
			"GOLC_WAILS_ASSET_TOO_LARGE: %q is %d bytes, over the %d byte limit", trimmed, info.Size(), int64(maxUploadedImageBytes))
	}

	data, readErr := os.ReadFile(trimmed)
	if readErr != nil {
		return AssetUploadView{}, fmt.Errorf("GOLC_WAILS_ASSET_READ_FAILED: %v", readErr)
	}
	mimeType := detectImageMimeType(trimmed, data)
	if !strings.HasPrefix(mimeType, "image/") {
		return AssetUploadView{}, fmt.Errorf("GOLC_WAILS_ASSET_TYPE_REJECTED: %q looks like %q, not an image", trimmed, mimeType)
	}

	id, idErr := uuid.NewV7()
	if idErr != nil {
		return AssetUploadView{}, fmt.Errorf("GOLC_WAILS_ASSET_ID_FAILED: %v", idErr)
	}
	if err := show.SaveAsset(s.root, s.showPath, id.String(), mimeType, filepath.Base(trimmed), data); err != nil {
		return AssetUploadView{}, err
	}

	return AssetUploadView{
		ID:      id.String(),
		DataURI: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}

// GetImageDataURI reads id back (show.LoadAsset) and returns it as a
// ready-to-use data: URI -- the Desk workspace's own read path for a
// fixture card whose backgroundImageAssetID it did not just itself
// upload this session (e.g. a show opened fresh, or a second card
// referencing an asset the first card's own upload already created).
func (s *ShowService) GetImageDataURI(id string) (string, error) {
	mimeType, data, err := show.LoadAsset(s.root, s.showPath, id)
	if err != nil {
		return "", err
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// DeleteImage removes id (show.DeleteAsset, already a no-op rather than an
// error for an id that does not exist) -- the fixture-style modal's own
// "Clear the background image" reset button calls this once it has
// confirmed the operator is dropping that asset for good, not just
// clearing the modal's own in-progress form state.
func (s *ShowService) DeleteImage(id string) Result {
	if err := show.DeleteAsset(s.root, s.showPath, id); err != nil {
		return Result{ExitCode: 1, Stderr: err.Error()}
	}
	return Result{Stdout: fmt.Sprintf("GOLC_SHOW_ASSET_DELETED: %s\n", id)}
}
