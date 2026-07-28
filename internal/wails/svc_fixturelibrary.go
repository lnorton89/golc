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
	"os"
	"path/filepath"
	"regexp"
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

	// previewMirror/previewAllowMirror mirror ofl.OFLRef's own Mirror/
	// AllowMirror opt-in shape (09-06-PLAN.md Task 1 test seam): zero-valued
	// in production, so PreviewOFL forwards no --mirror/--allow-mirror
	// flags to the "fixture import" route and resolves to the default
	// upstream host exactly like a direct CLI invocation would. Only this
	// package's own tests override these fields, to point PreviewOFL at a
	// deterministic httptest server rather than the live catalog.
	previewMirror      string
	previewAllowMirror bool

	// previewMu guards previewDir (lazily created on first PreviewOFL/
	// PreviewFile/CommitPreview/DiscardPreview call), previewSeq (a
	// monotonically increasing counter giving every staged preview file a
	// unique name), and previewRegistry (09-07-PLAN.md Task 2): the
	// in-memory map from a staged preview's own token (its path inside
	// previewDir) to the library destination file name PreviewOFL/
	// PreviewFile already computed for it. CommitPreview reads the
	// destination from this registry rather than re-deriving it from the
	// artifact -- the one mechanism that lets a single commit path serve
	// both a JSON import artifact (OFL) and a hand-authored YAML file
	// (custom) without the Wails layer ever decoding a fixture document of
	// its own. Only PreviewOFL/PreviewFile ever add an entry (T-09-07-02):
	// the registry maps a token this service issued to a destination this
	// service computed, never caller-supplied input.
	previewMu       sync.Mutex
	previewDir      string
	previewSeq      int
	previewRegistry map[string]string
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

// --- 09-06-PLAN.md: preview-then-commit OFL import (D-02, T-09-06-*) ---

// FixturePreviewView is PreviewOFL's return shape: the candidate's
// FixtureInspectView projection, an opaque previewToken the frontend
// round-trips to CommitPreview/DiscardPreview (never rendered -- it is the
// previewed artifact's own filesystem path, outside both the project root
// and the library directory), whether the suggested destination already
// exists in the library, and that suggested library file name.
type FixturePreviewView struct {
	Inspect           FixtureInspectView `json:"inspect"`
	PreviewToken      string             `json:"previewToken"`
	DestinationExists bool               `json:"destinationExists"`
	SuggestedFileName string             `json:"suggestedFileName"`
}

// ensurePreviewDir lazily creates this service instance's own dedicated
// preview directory under the OS temp directory -- never the library
// directory, and never scanned by ListLocal/ListDirectory -- so a staged
// preview is invisible to the library until CommitPreview explicitly moves
// it there.
func (s *FixtureLibraryService) ensurePreviewDir() (string, error) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if s.previewDir != "" {
		return s.previewDir, nil
	}
	dir, err := os.MkdirTemp("", "golc-fixture-preview-")
	if err != nil {
		return "", fmt.Errorf("GOLC_WAILS_FIXTURE_PREVIEW_FAILED: creating preview directory: %v", err)
	}
	s.previewDir = dir
	return dir, nil
}

// nextPreviewPath allocates a unique path inside previewDir for one
// PreviewOFL call, keyed by a per-service monotonically increasing
// sequence number so concurrent previews (or repeated previews of the same
// candidate) never collide.
func (s *FixtureLibraryService) nextPreviewPath(previewDir, manufacturerKey, fixtureKey string) string {
	s.previewMu.Lock()
	s.previewSeq++
	seq := s.previewSeq
	s.previewMu.Unlock()
	name := fmt.Sprintf("%s_%s-%d.json", sanitizePreviewSegment(manufacturerKey), sanitizePreviewSegment(fixtureKey), seq)
	return filepath.Join(previewDir, name)
}

// registerPreviewDestination records token's already-computed library
// destination file name in the preview registry (09-07-PLAN.md Task 2) --
// called only by PreviewOFL/PreviewFile, immediately after each stages its
// own artifact, so the registry only ever maps a token this service issued
// to a destination this service computed.
func (s *FixtureLibraryService) registerPreviewDestination(token, destFileName string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if s.previewRegistry == nil {
		s.previewRegistry = make(map[string]string)
	}
	s.previewRegistry[token] = destFileName
}

// previewDestination looks up token's registered destination file name; ok
// is false when token was never registered (e.g. discarded already, or
// never staged by this service at all).
func (s *FixtureLibraryService) previewDestination(token string) (string, bool) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	dest, ok := s.previewRegistry[token]
	return dest, ok
}

// forgetPreviewDestination removes token's registry entry -- called once a
// staged preview is committed or discarded, so the registry never
// accumulates unbounded entries across a long-running session.
func (s *FixtureLibraryService) forgetPreviewDestination(token string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	delete(s.previewRegistry, token)
}

// nonSlugRunPattern/repeatedHyphenPattern back
// libraryFileNameForCustomFixture's slug derivation, below.
var (
	nonSlugRunPattern     = regexp.MustCompile(`[^a-z0-9-]+`)
	repeatedHyphenPattern = regexp.MustCompile(`-+`)
)

// libraryFileNameForCustomFixture derives PreviewFile's suggested library
// destination file name from a hand-authored fixture's own inspected
// stable key (09-07-PLAN.md Task 2): lowercased, every run of characters
// that is not a letter, digit, or hyphen replaced with a single hyphen,
// consecutive hyphens collapsed, leading/trailing hyphens trimmed, with
// ext (the source file's own extension) appended so the operator's YAML
// stays YAML -- mirrors libraryFileNameForSource's identical "commit
// destination mechanically derived, never caller-supplied" discipline for
// the OFL import path.
func libraryFileNameForCustomFixture(stableKey, ext string) string {
	slug := nonSlugRunPattern.ReplaceAllString(strings.ToLower(stableKey), "-")
	slug = repeatedHyphenPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "custom-fixture"
	}
	return slug + ext
}

// sanitizePreviewSegment strips path-separator/traversal characters from a
// caller-supplied manufacturer/fixture key before it becomes part of a
// filesystem path -- defensive, since every real OFL key is a plain slug,
// but a webview-supplied string is never trusted verbatim as a path
// component (T-09-06-01's trust boundary).
func sanitizePreviewSegment(raw string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	return replacer.Replace(raw)
}

// isPathWithinDir reports whether candidate resolves to a path strictly
// inside dir: both sides are resolved to absolute, cleaned paths and
// compared with filepath.Rel on a path-separator boundary (T-09-06-01) --
// never a bare string-prefix comparison, which a crafted sibling directory
// name could defeat. dir itself does not count as "within" dir.
func isPathWithinDir(dir, candidate string) bool {
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false
	}
	absCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absCandidate)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// libraryFileNameForSource derives the library destination file name from
// an import artifact's own Provenance.Source (the "ofl:<manufacturer>/<key>"
// label ofl.Normalize always applies), mirroring internal/command/
// fixture.go's own "<manufacturer>_<key>.json" corpus naming convention
// (oflSourceFromFilename's inverse) -- so PreviewOFL's SuggestedFileName and
// CommitPreview's own re-derivation from the previewed artifact always
// agree on the identical destination name.
func libraryFileNameForSource(source string) string {
	trimmed := strings.TrimPrefix(source, "ofl:")
	return strings.ReplaceAll(trimmed, "/", "_") + ".json"
}

// PreviewOFL stages an OFL import for manufacturerKey/fixtureKey into this
// service's own dedicated preview directory by forwarding to the existing,
// already-tested "fixture import" route via execute -- the Wails layer
// performs no fetch, no normalization, no pinning, and no validation of its
// own (T-09-06-04). A non-zero exit code from that route projects as a
// renderable invalid-candidate view (Inspect.Valid:false, Errors built from
// the route's own non-empty trimmed stderr lines) with a nil error, and any
// partial preview file is removed -- PreviewOFL itself never returns a
// non-nil error for a rejected candidate. On success the previewed artifact
// is read back and decoded through fixture.DecodeEnvelope/fixture.Pin --
// never re-encoded, re-normalized, or independently pinned (D-02,
// 09-RESEARCH.md anti-patterns) -- and projected into the returned view
// alongside the preview token (an opaque handle, never rendered) and
// whether the suggested destination already exists in the library
// (T-09-06-02).
func (s *FixtureLibraryService) PreviewOFL(manufacturerKey, fixtureKey string) (FixturePreviewView, error) {
	previewDir, err := s.ensurePreviewDir()
	if err != nil {
		return FixturePreviewView{}, err
	}
	previewPath := s.nextPreviewPath(previewDir, manufacturerKey, fixtureKey)

	args := []string{"fixture", "import", "--ofl", manufacturerKey + "/" + fixtureKey}
	if s.previewMirror != "" {
		args = append(args, "--mirror", s.previewMirror, "--allow-mirror")
	}
	args = append(args, "--out", previewPath)

	result := s.execute(args)
	if result.ExitCode != 0 {
		_ = os.Remove(previewPath)
		return FixturePreviewView{
			Inspect: FixtureInspectView{
				Valid:    false,
				Errors:   trimmedNonEmptyLines(result.Stderr),
				Warnings: []FixtureWarningView{},
			},
		}, nil
	}

	data, err := os.ReadFile(previewPath)
	if err != nil {
		return FixturePreviewView{}, fmt.Errorf("GOLC_WAILS_FIXTURE_PREVIEW_FAILED: reading previewed artifact: %v", err)
	}
	envelope, err := fixture.DecodeEnvelope(data)
	if err != nil {
		return FixturePreviewView{}, fmt.Errorf("GOLC_WAILS_FIXTURE_PREVIEW_FAILED: %v", err)
	}
	identity, err := fixture.Pin(envelope.Definition)
	if err != nil {
		return FixturePreviewView{}, fmt.Errorf("GOLC_WAILS_FIXTURE_PREVIEW_FAILED: %v", err)
	}

	warnings := make([]FixtureWarningView, 0, len(envelope.Provenance.Warnings))
	for _, warning := range envelope.Provenance.Warnings {
		warnings = append(warnings, FixtureWarningView{
			Severity:       warning.Severity,
			CapabilityType: warning.CapabilityType,
			Detail:         warning.Detail,
		})
	}

	suggestedFileName := libraryFileNameForSource(envelope.Provenance.Source)
	_, statErr := os.Stat(filepath.Join(s.fixturesDir, suggestedFileName))
	destinationExists := statErr == nil
	s.registerPreviewDestination(previewPath, suggestedFileName)

	return FixturePreviewView{
		Inspect: FixtureInspectView{
			Valid:            true,
			Errors:           []string{},
			SchemaVersion:    identity.SchemaVersion,
			StableKey:        identity.StableKey,
			ContentHash:      identity.ContentHash,
			Revision:         identity.Revision,
			Source:           envelope.Provenance.Source,
			ValidationResult: envelope.Provenance.ValidationResult,
			Warnings:         warnings,
		},
		PreviewToken:      previewPath,
		DestinationExists: destinationExists,
		SuggestedFileName: suggestedFileName,
	}, nil
}

// --- 09-07-PLAN.md: hand-authored YAML fixture add (D-04, T-09-07-*) ---

// PreviewFile stages a hand-authored YAML fixture file for inline
// inspection before anything is committed: the sole validation authority
// is s.Inspect(path) -- the existing forward to the registered "fixture
// inspect" route -- so a definition the CLI would reject can never render
// as addable here (T-09-07-01, the same canonical-pipeline guarantee
// PreviewOFL already gives the catalog path). An unreadable path and an
// invalid definition both project through Inspect as Valid:false with
// nothing staged and a nil error -- there is exactly one error surface for
// both failure modes, matching the UI-SPEC's single "{N} error(s)"
// diagnostic. On success the operator's file is copied byte-for-byte into
// this service's own preview directory (never re-encoded, mirroring
// CommitPreview's own move-never-re-encode discipline), the destination
// file name is derived from the inspected stable key
// (libraryFileNameForCustomFixture) keeping the source file's own
// extension, and the token/destination pair is recorded in the preview
// registry so CommitPreview never has to decode the file itself.
func (s *FixtureLibraryService) PreviewFile(path string) (FixturePreviewView, error) {
	inspectView, err := s.Inspect(path)
	if err != nil {
		return FixturePreviewView{}, err
	}
	if !inspectView.Valid {
		return FixturePreviewView{Inspect: inspectView}, nil
	}

	previewDir, err := s.ensurePreviewDir()
	if err != nil {
		return FixturePreviewView{}, err
	}

	ext := filepath.Ext(path)
	if ext == "" {
		ext = ".yaml"
	}
	previewPath := s.nextCustomPreviewPath(previewDir, inspectView.StableKey, ext)

	if err := copyFileBytes(path, previewPath); err != nil {
		return FixturePreviewView{}, fmt.Errorf("GOLC_WAILS_FIXTURE_PREVIEW_STAGE_FAILED: %v", err)
	}

	suggestedFileName := libraryFileNameForCustomFixture(inspectView.StableKey, ext)
	_, statErr := os.Stat(filepath.Join(s.fixturesDir, suggestedFileName))
	destinationExists := statErr == nil
	s.registerPreviewDestination(previewPath, suggestedFileName)

	return FixturePreviewView{
		Inspect:           inspectView,
		PreviewToken:      previewPath,
		DestinationExists: destinationExists,
		SuggestedFileName: suggestedFileName,
	}, nil
}

// nextCustomPreviewPath allocates a unique staging path inside previewDir
// for one PreviewFile call, keyed by the same per-service monotonically
// increasing sequence number nextPreviewPath uses, but keeping ext (the
// source file's own extension, never .json) so a staged custom fixture
// stays valid YAML on disk.
func (s *FixtureLibraryService) nextCustomPreviewPath(previewDir, stableKey, ext string) string {
	s.previewMu.Lock()
	s.previewSeq++
	seq := s.previewSeq
	s.previewMu.Unlock()
	name := fmt.Sprintf("custom_%s-%d%s", sanitizePreviewSegment(stableKey), seq, ext)
	return filepath.Join(previewDir, name)
}

// copyFileBytes copies src's exact bytes to dst (0o644) -- used by
// PreviewFile to stage the operator's hand-authored fixture file
// byte-for-byte, mirroring CommitPreview's own "move/copy, never
// re-encode" discipline for both import paths.
func copyFileBytes(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// CommitPreview commits the previewed artifact at previewToken into the
// library directory (T-09-06-01/T-09-06-02/T-09-06-04, T-09-07-02):
// previewToken must resolve inside this service's own preview directory,
// checked before any filesystem operation (GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN
// otherwise); the destination file name is read from the in-memory preview
// registry PreviewOFL/PreviewFile populated when they staged this exact
// token (09-07-PLAN.md Task 2) -- never re-derived by decoding the staged
// file, since a staged custom fixture is raw YAML, not the JSON import
// envelope PreviewOFL stages -- and an existing destination is refused
// with GOLC_WAILS_FIXTURE_IMPORT_EXISTS unless overwrite is true, writing
// nothing in that case. On success the previewed file is moved (never
// copied-and-re-encoded) to the destination, so the committed bytes are
// byte-identical to the bytes originally staged.
func (s *FixtureLibraryService) CommitPreview(previewToken string, overwrite bool) Result {
	previewDir, err := s.ensurePreviewDir()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("%v\n", err)}
	}
	if !isPathWithinDir(previewDir, previewToken) {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN: preview token does not resolve inside this service's own preview directory\n"}
	}

	destFileName, ok := s.previewDestination(previewToken)
	if !ok {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN: preview token was never staged by this service\n"}
	}

	if err := os.MkdirAll(s.fixturesDir, 0o755); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_FIXTURE_IMPORT_WRITE_FAILED: creating library directory: %v\n", err)}
	}

	destPath := filepath.Join(s.fixturesDir, destFileName)

	if !overwrite {
		if _, statErr := os.Stat(destPath); statErr == nil {
			return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_FIXTURE_IMPORT_EXISTS: %s already exists in the library\n", destFileName)}
		}
	}

	if err := os.Rename(previewToken, destPath); err != nil {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_FIXTURE_IMPORT_WRITE_FAILED: %v\n", err)}
	}
	s.forgetPreviewDestination(previewToken)

	return Result{Stdout: fmt.Sprintf("GOLC_FIXTURE_IMPORT: wrote %s\n", destFileName)}
}

// DiscardPreview removes the staged preview at previewToken -- the
// workspace calls this when the operator changes selection or switches
// source, so an abandoned preview never accumulates (T-09-06-05). The same
// containment check as CommitPreview applies before any filesystem
// operation; removing an already-gone preview is not an error. The
// registry entry (if any) is forgotten regardless, so it never
// accumulates across a long-running session.
func (s *FixtureLibraryService) DiscardPreview(previewToken string) Result {
	previewDir, err := s.ensurePreviewDir()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("%v\n", err)}
	}
	if !isPathWithinDir(previewDir, previewToken) {
		return Result{ExitCode: 2, Stderr: "GOLC_WAILS_FIXTURE_PREVIEW_UNKNOWN: preview token does not resolve inside this service's own preview directory\n"}
	}
	s.forgetPreviewDestination(previewToken)
	if err := os.Remove(previewToken); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("GOLC_WAILS_FIXTURE_PREVIEW_FAILED: %v\n", err)}
	}
	return Result{}
}
