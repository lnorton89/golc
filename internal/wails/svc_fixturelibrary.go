// svc_fixturelibrary.go fills FixtureLibraryService, the Wails binding
// backing FixtureLibraryWorkspace.tsx (09-01-PLAN.md, CONTEXT D-01 local
// half, D-02, D-03): ListLocal is a Wails-only read projection (no
// CLI-parity route, mirroring ShowService.Inspect/FixturePatchService.
// ListPatch's established precedent) over the single extracted
// internal/fixture.ListDirectory scan -- it decodes and pins nothing
// itself, so a fixture the CLI would reject can never render as usable
// on screen (T-09-01-03). Inspect forwards to the existing, already-
// tested "fixture inspect" route via the exact execute() helper
// svc_show.go/svc_fixturepatch.go already established, projecting its
// allowlisted snake_case JSON envelope into this package's camelCase
// convention -- never a second decode/pin/normalize implementation.
package wails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/fixture/ofl"
)

// defaultFixturesDirName is the directory name resolved under root when
// fixturesDir is left empty (mirrors artnet.go's own "fixtures" default
// expectation for the desktop app's local library).
const defaultFixturesDirName = "fixtures"

// FixtureLibraryService is bound to the frontend via cmd/golc-desktop/
// main.go's options.App{Bind: [...]}. root is the project root Inspect()
// forwards a relative path against (via execute); fixturesDir is the
// single local-fixture-directory ListLocal scans (mirrors
// FixturePatchService/ShowService's own root/showPath field convention).
// oflIndexRef/oflMu/oflManufacturers/oflFetchErr back SearchOFL's lazy,
// once-per-process manufacturer-index cache (T-09-05-03): oflIndexRef is
// zero-valued in production (resolving to the default upstream host,
// mirroring ofl.ManufacturerIndexRef's own "empty Mirror means default
// host" contract) and is only ever overridden by a test in this same
// package to point at a deterministic, non-live target.
type FixtureLibraryService struct {
	pipeName    string
	root        string
	fixturesDir string

	oflIndexRef ofl.ManufacturerIndexRef

	oflMu            sync.Mutex
	oflFetched       bool
	oflManufacturers []ofl.Manufacturer
	oflFetchErr      error
}

// NewFixtureLibraryService constructs a FixtureLibraryService targeting
// pipeName (reserved, unused -- mirrors every sibling service's own
// unused pipeName field) and fixturesDir, resolved to
// filepath.Join(root, "fixtures") when left empty so a fresh checkout
// with no configured fixtures directory still resolves to a real,
// scannable (if not-yet-created) path.
func NewFixtureLibraryService(pipeName, root, fixturesDir string) *FixtureLibraryService {
	if fixturesDir == "" {
		fixturesDir = filepath.Join(root, defaultFixturesDirName)
	}
	return &FixtureLibraryService{pipeName: pipeName, root: root, fixturesDir: fixturesDir}
}

// execute builds the default command registry and runs args against it,
// converting the internal/command.Result shape into this package's own
// Result shape (mirrors svc_show.go/svc_fixturepatch.go's identical
// helper).
func (s *FixtureLibraryService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}

// FixtureLibraryRowView is one ListLocal row for
// FixtureLibraryWorkspace.tsx's list. An entry that failed to decode/pin
// projects Status "invalid" with Detail carrying that failure's message
// and Manufacturer/Model left empty (StableKey falls back to FileName);
// a clean entry projects Status "valid" with its pinned StableKey.
type FixtureLibraryRowView struct {
	StableKey    string `json:"stableKey"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	FileName     string `json:"fileName"`
	Source       string `json:"source"`
	Status       string `json:"status"`
	Detail       string `json:"detail"`
}

// FixtureLibraryView is ListLocal's full return shape. Rows is always a
// non-nil (possibly empty) slice -- encoding/json marshals a nil slice as
// JSON "null", and "null.length" throws with no error boundary anywhere
// in this app (mirrors svc_show.go's DiagnosticReportView doc comment,
// T-09-01-04). Directory is projected repo-relative with forward slashes
// (or "external:<basename>" when outside root) -- never the resolved
// absolute path (T-09-01-01).
type FixtureLibraryView struct {
	Directory string                  `json:"directory"`
	Rows      []FixtureLibraryRowView `json:"rows"`
}

// fixtureLibraryDirectoryLabel projects fixturesDir with the exact
// repository-relative-or-external discipline internal/command/fixture.go's
// fixtureInspectSource applies for T-01-23 -- never the resolved absolute
// path (T-09-01-01).
func fixtureLibraryDirectoryLabel(root, fixturesDir string) string {
	if rel, err := filepath.Rel(root, fixturesDir); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return "external:" + filepath.Base(fixturesDir)
}

// ListLocal projects internal/fixture.ListDirectory(s.fixturesDir) into
// FixtureLibraryView. A not-yet-created fixtures directory
// (errors.Is(err, fs.ErrNotExist)) projects as an empty library, not an
// error -- an operator who hasn't imported anything yet sees "No
// fixtures yet", not a broken workspace. Rows is sorted ascending by
// StableKey and is always non-nil.
func (s *FixtureLibraryService) ListLocal() (FixtureLibraryView, error) {
	directory := fixtureLibraryDirectoryLabel(s.root, s.fixturesDir)

	entries, err := fixture.ListDirectory(s.fixturesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return FixtureLibraryView{Directory: directory, Rows: []FixtureLibraryRowView{}}, nil
		}
		return FixtureLibraryView{}, err
	}

	rows := make([]FixtureLibraryRowView, 0, len(entries))
	for _, entry := range entries {
		if entry.Err != nil {
			rows = append(rows, FixtureLibraryRowView{
				StableKey: entry.FileName,
				FileName:  entry.FileName,
				Source:    "local",
				Status:    "invalid",
				Detail:    entry.Err.Error(),
			})
			continue
		}
		rows = append(rows, FixtureLibraryRowView{
			StableKey:    entry.Identity.StableKey,
			Manufacturer: entry.Definition.Manufacturer,
			Model:        entry.Definition.Model,
			FileName:     entry.FileName,
			Source:       rowSource(entry.Provenance),
			Status:       "valid",
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].StableKey < rows[j].StableKey })

	return FixtureLibraryView{Directory: directory, Rows: rows}, nil
}

// rowSource projects a DirectoryEntry's Provenance into ListLocal's row
// "source" field: "ofl" when the provenance names an OFL source (its
// Source carries the "ofl:" prefix ofl.Normalize's NewProvenance call
// always applies), "local" otherwise -- a hand-authored .yaml/.yml entry
// carries a zero-valued Provenance, so its Source is always empty and
// projects as "local". This makes an imported fixture's catalog origin
// visible in the library rather than indistinguishable from a
// hand-authored one (T-09-05-05).
func rowSource(provenance fixture.Provenance) string {
	if strings.HasPrefix(provenance.Source, "ofl:") {
		return "ofl"
	}
	return "local"
}

// FixtureWarningView mirrors internal/fixture.LossyImportWarning's JSON
// shape in this package's camelCase convention.
type FixtureWarningView struct {
	Severity       string `json:"severity"`
	CapabilityType string `json:"capabilityType"`
	Detail         string `json:"detail"`
}

// FixtureInspectView is Inspect's return shape: the "fixture inspect"
// route's own allowlisted JSON envelope, reshaped to this package's
// camelCase convention, plus Valid/Errors so a rejected fixture renders
// as an explicit state (Valid:false, Errors populated) rather than a
// thrown exception. Errors and Warnings are always non-nil.
type FixtureInspectView struct {
	Path             string               `json:"path"`
	Valid            bool                 `json:"valid"`
	Errors           []string             `json:"errors"`
	SchemaVersion    int                  `json:"schemaVersion"`
	StableKey        string               `json:"stableKey"`
	ContentHash      string               `json:"contentHash"`
	Revision         string               `json:"revision"`
	Source           string               `json:"source"`
	ValidationResult string               `json:"validationResult"`
	Warnings         []FixtureWarningView `json:"warnings"`
}

// fixtureInspectRouteView mirrors internal/command/fixture.go's private
// fixtureInspectView JSON shape (snake_case) -- the exact allowlisted
// envelope the "fixture inspect" route already emits on success, decoded
// here rather than re-implemented.
type fixtureInspectRouteView struct {
	SchemaVersion    int                          `json:"schema_version"`
	StableKey        string                       `json:"stable_key"`
	ContentHash      string                       `json:"content_hash"`
	Revision         string                       `json:"revision"`
	Source           string                       `json:"source"`
	ValidationResult string                       `json:"validation_result"`
	Warnings         []fixtureInspectRouteWarning `json:"warnings"`
}

// fixtureInspectRouteWarning mirrors internal/command/fixture.go's
// private fixtureWarningView JSON shape (snake_case).
type fixtureInspectRouteWarning struct {
	Severity       string `json:"severity"`
	CapabilityType string `json:"capability_type"`
	Detail         string `json:"detail"`
}

// Inspect forwards path to the existing, already-tested "fixture inspect"
// route (command.NewDefaultCommandRegistry + registry.Execute) -- never a
// second decode/pin/normalize implementation (T-09-01-03). A non-zero
// exit code is a renderable rejected-fixture state (Valid:false, Errors
// populated from stderr's non-empty trimmed lines), not a thrown error;
// this method itself never decodes, validates, or pins anything.
func (s *FixtureLibraryService) Inspect(path string) (FixtureInspectView, error) {
	result := s.execute([]string{"fixture", "inspect", path})
	if result.ExitCode != 0 {
		return FixtureInspectView{
			Path:     path,
			Valid:    false,
			Errors:   trimmedNonEmptyLines(result.Stderr),
			Warnings: []FixtureWarningView{},
		}, nil
	}

	var route fixtureInspectRouteView
	if err := json.Unmarshal([]byte(result.Stdout), &route); err != nil {
		return FixtureInspectView{}, fmt.Errorf("GOLC_WAILS_FIXTURE_INSPECT_DECODE_FAILED: %v", err)
	}

	warnings := make([]FixtureWarningView, 0, len(route.Warnings))
	for _, warning := range route.Warnings {
		warnings = append(warnings, FixtureWarningView{
			Severity:       warning.Severity,
			CapabilityType: warning.CapabilityType,
			Detail:         warning.Detail,
		})
	}

	return FixtureInspectView{
		Path:             path,
		Valid:            true,
		Errors:           []string{},
		SchemaVersion:    route.SchemaVersion,
		StableKey:        route.StableKey,
		ContentHash:      route.ContentHash,
		Revision:         route.Revision,
		Source:           route.Source,
		ValidationResult: route.ValidationResult,
		Warnings:         warnings,
	}, nil
}

// trimmedNonEmptyLines splits raw (typically a Result's Stderr) into its
// non-empty, trimmed lines -- each line becomes one FixtureInspectView
// Errors entry.
func trimmedNonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// OFLManufacturerView is one SearchOFL result row (09-05-PLAN.md Task 3,
// D-01's catalog half): the manufacturer name FixtureLibraryWorkspace.tsx
// renders as the row label, its key as the row's meta text.
type OFLManufacturerView struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Website string `json:"website"`
}

// OFLSearchView is SearchOFL's full return shape. Query echoes the
// caller's own query so the frontend can interpolate it into the
// no-results copy without keeping its own separate copy of "what did I
// just search for." Manufacturers is always a non-nil (possibly empty)
// slice -- mirrors FixtureLibraryView.Rows' identical "never JSON null"
// discipline. Unreachable is true when the manufacturer-index fetch
// failed; an unreachable catalog is a renderable state, never a thrown
// exception (T-09-05-02).
type OFLSearchView struct {
	Query         string                `json:"query"`
	Manufacturers []OFLManufacturerView `json:"manufacturers"`
	Unreachable   bool                  `json:"unreachable"`
	Detail        string                `json:"detail"`
}

// SearchOFL projects a manufacturer-name/key substring search over the
// Open Fixture Library catalog into a renderable view (09-05-PLAN.md
// Task 3, D-01/D-03): the manufacturer index is fetched at most once per
// process (loadOFLManufacturers' cache), so a typing burst filters the
// already-fetched slice with no additional network call
// (T-09-05-03). A fetch failure returns unreachable:true with the
// diagnostic in Detail, a non-nil empty Manufacturers slice, and a nil
// error -- SearchOFL itself never returns a non-nil error.
func (s *FixtureLibraryService) SearchOFL(query string) (OFLSearchView, error) {
	manufacturers, fetchErr := s.loadOFLManufacturers()
	if fetchErr != nil {
		return OFLSearchView{
			Query:         query,
			Manufacturers: []OFLManufacturerView{},
			Unreachable:   true,
			Detail:        fetchErr.Error(),
		}, nil
	}

	matches := ofl.FilterManufacturers(manufacturers, query)
	views := make([]OFLManufacturerView, 0, len(matches))
	for _, manufacturer := range matches {
		views = append(views, OFLManufacturerView{
			Key:     manufacturer.Key,
			Name:    manufacturer.Name,
			Website: manufacturer.Website,
		})
	}

	return OFLSearchView{Query: query, Manufacturers: views, Unreachable: false}, nil
}

// loadOFLManufacturers lazily fetches and caches the OFL manufacturer
// index behind a mutex, so a burst of SearchOFL calls (one per operator
// keystroke) issues at most one network request per process lifetime --
// every subsequent call reuses the cached slice or the cached failure
// (T-09-05-03). A cached failure is retried on the next call (never
// permanently sticky), so a transient network blip does not permanently
// wedge the catalog for the rest of the session.
func (s *FixtureLibraryService) loadOFLManufacturers() ([]ofl.Manufacturer, error) {
	s.oflMu.Lock()
	defer s.oflMu.Unlock()

	if s.oflFetched && s.oflFetchErr == nil {
		return s.oflManufacturers, nil
	}

	manufacturers, err := ofl.FetchManufacturers(context.Background(), s.oflIndexRef)
	s.oflFetched = true
	s.oflFetchErr = err
	if err != nil {
		return nil, err
	}
	s.oflManufacturers = manufacturers
	return manufacturers, nil
}
