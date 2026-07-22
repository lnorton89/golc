// fixture.go is the fixture command file: it owns the "fixture" routing
// scope and self-registers the "fixture validate", "fixture inspect", and
// "fixture import" routes (CONTEXT D-01/D-02/D-04): a show author
// hand-writes a fixture definition YAML file, "fixture validate" strictly
// decodes and validates it printing a deterministic canonical summary on
// success, "fixture inspect" additionally surfaces the fixture's
// content-addressed identity and provenance (FIXT-05/FIXT-06) through an
// allowlisted JSON envelope before the fixture is used in a show, and
// "fixture import" (02-03, FIXT-03/D-06/D-07) brings an Open Fixture
// Library definition -- fetched live (SSRF-guarded) or read from a local
// file -- through the exact same canonical normalization, validation, and
// pinning pipeline. "fixture validate"/"fixture inspect" perform no
// scaffold/generator behavior (D-02: read-only) or network access;
// "fixture import --ofl-file" likewise makes no network call, and
// "fixture import --ofl" is the sole network-reaching route this file
// declares. None of the three ever emits an absolute filesystem path or
// OS-local detail (T-01-23).
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lnorton89/golc/internal/fixture"
	"github.com/lnorton89/golc/internal/fixture/ofl"
	"github.com/lnorton89/golc/internal/strictjson"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "fixture",
	Summary: "Hand-authored YAML fixture definition validation, identity pinning, and provenance inspection.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "fixture validate",
	Summary: "Strictly decode and validate a hand-authored YAML fixture definition, printing its canonical summary on success: fixture validate <file>.",
	Handler: runFixtureValidate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "fixture inspect",
	Summary: "Decode a fixture definition and print its content-addressed identity and provenance as a deterministic, path-free JSON envelope before it is used in a show: fixture inspect <file>.",
	Handler: runFixtureInspect,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "fixture import",
	Summary: "Import an Open Fixture Library definition through GOLC's canonical normalization, validation, and pinning pipeline, surfacing any lossy/unsupported OFL construct as an explicit warning: " +
		"fixture import --ofl <manufacturer>/<key> [--mirror <url>] [--allow-mirror] --out <path> | fixture import --ofl-file <path> --out <path>.",
	Handler: runFixtureImport,
})

// runFixtureValidate serves the self-registered "fixture validate" route.
// It never writes anything (D-02/D-03: validate-only, file-level share):
// success and failure alike only read the given file and print a result.
func runFixtureValidate(request Request) Result {
	path, err := parseFixtureFileArg("fixture validate <file>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	data, err := os.ReadFile(resolveWritablePath(request.Root, path))
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_READ_FAILED: %v\n", err))}
	}

	def, err := fixture.Decode(data)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	summary, err := strictjson.CanonicalEncode(def)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: summary}
}

// fixtureWarningView is the allowlisted JSON projection of one
// fixture.LossyImportWarning.
type fixtureWarningView struct {
	Severity       string `json:"severity"`
	CapabilityType string `json:"capability_type,omitempty"`
	Detail         string `json:"detail"`
}

// fixtureInspectView is the deterministic, allowlisted JSON envelope
// "fixture inspect" emits: only stable identity/provenance/validation/
// warning fields are projected, mirroring linear.go's catalogView
// allowlisted-projection discipline -- never a raw absolute path or host
// detail (T-01-23).
type fixtureInspectView struct {
	SchemaVersion    int                  `json:"schema_version"`
	StableKey        string               `json:"stable_key"`
	ContentHash      string               `json:"content_hash"`
	Revision         string               `json:"revision"`
	Source           string               `json:"source"`
	ValidationResult string               `json:"validation_result"`
	Warnings         []fixtureWarningView `json:"warnings"`
}

// fixtureInspectWarningViews projects a Provenance's Warnings into the
// allowlisted JSON view, preserving declared order; it always returns a
// non-nil (possibly empty) slice so the JSON envelope emits "[]" rather
// than "null" for a fixture with nothing to warn about.
func fixtureInspectWarningViews(warnings []fixture.LossyImportWarning) []fixtureWarningView {
	views := make([]fixtureWarningView, 0, len(warnings))
	for _, warning := range warnings {
		views = append(views, fixtureWarningView{
			Severity:       warning.Severity,
			CapabilityType: warning.CapabilityType,
			Detail:         warning.Detail,
		})
	}
	return views
}

// fixtureInspectSource derives "fixture inspect"'s Provenance.Source: a
// repository-relative, slash-normalized path when the resolved file lives
// under request root, or a stable basename-only label otherwise -- never
// the resolved absolute path itself (T-01-23: information disclosure).
func fixtureInspectSource(root, resolvedPath string) string {
	if rel, err := filepath.Rel(root, resolvedPath); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return "external:" + filepath.Base(resolvedPath)
}

// runFixtureInspect serves the self-registered "fixture inspect" route.
// It never writes anything (D-02/D-03: read-only, file-level share):
// success and failure alike only read the given file and print a result.
// On success it pins the decoded fixture (FIXT-05), builds its Provenance
// (FIXT-06), and emits both through the allowlisted fixtureInspectView
// envelope; on decode/pin/encode failure it returns ExitCode 2 with the
// underlying GOLC_FIXTURE_* diagnostic on Stderr.
func runFixtureInspect(request Request) Result {
	path, err := parseFixtureFileArg("fixture inspect <file>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	resolvedPath := resolveWritablePath(request.Root, path)
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_READ_FAILED: %v\n", err))}
	}

	def, err := fixture.Decode(data)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	identity, err := fixture.Pin(def)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	provenance := fixture.NewProvenance(def, identity, fixtureInspectSource(request.Root, resolvedPath))

	view := fixtureInspectView{
		SchemaVersion:    identity.SchemaVersion,
		StableKey:        identity.StableKey,
		ContentHash:      identity.ContentHash,
		Revision:         identity.Revision,
		Source:           provenance.Source,
		ValidationResult: provenance.ValidationResult,
		Warnings:         fixtureInspectWarningViews(provenance.Warnings),
	}

	payload, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_INSPECT_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}

// parseFixtureFileArg accepts exactly one positional file path: not
// missing, not more than one, and never a dash-prefixed word (router.go's
// route-word grammar already rejects a dash-prefixed route word, but the
// remaining positional argument is handler-owned, so this handler rejects
// it explicitly too). Shared by "fixture validate" and "fixture inspect",
// which both take the same single-file-path shape.
func parseFixtureFileArg(usage string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	if strings.HasPrefix(args[0], "-") {
		return "", fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	return args[0], nil
}

// fixtureImportArgs is the parsed shape of one "fixture import"
// invocation: exactly one of the "--ofl <man>/<key>" (optionally with
// --mirror/--allow-mirror) or "--ofl-file <path>" source forms, plus
// --out <path>.
type fixtureImportArgs struct {
	oflRef      string
	oflFile     string
	mirror      string
	allowMirror bool
	outPath     string
}

// parseFixtureImportArgs accepts exactly one source form plus --out,
// rejecting a missing, mixed, or --ofl-file-plus-mirror-flag combination
// with GOLC_FIXTURE_USAGE (ExitCode 2 per this route's contract).
func parseFixtureImportArgs(usage string, args []string) (fixtureImportArgs, error) {
	parsed := fixtureImportArgs{}
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--ofl":
			if i+1 >= len(args) {
				return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --ofl requires a <manufacturer>/<key> value; usage: %s", usage)
			}
			parsed.oflRef = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--ofl="):
			parsed.oflRef = strings.TrimPrefix(argument, "--ofl=")
			i++
		case argument == "--ofl-file":
			if i+1 >= len(args) {
				return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --ofl-file requires a path; usage: %s", usage)
			}
			parsed.oflFile = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--ofl-file="):
			parsed.oflFile = strings.TrimPrefix(argument, "--ofl-file=")
			i++
		case argument == "--mirror":
			if i+1 >= len(args) {
				return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --mirror requires a URL; usage: %s", usage)
			}
			parsed.mirror = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--mirror="):
			parsed.mirror = strings.TrimPrefix(argument, "--mirror=")
			i++
		case argument == "--allow-mirror":
			parsed.allowMirror = true
			i++
		case argument == "--out":
			if i+1 >= len(args) {
				return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --out requires a path; usage: %s", usage)
			}
			parsed.outPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--out="):
			parsed.outPath = strings.TrimPrefix(argument, "--out=")
			i++
		default:
			return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.outPath == "" {
		return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	if parsed.oflRef == "" && parsed.oflFile == "" {
		return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: usage: %s", usage)
	}
	if parsed.oflRef != "" && parsed.oflFile != "" {
		return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --ofl and --ofl-file are mutually exclusive; usage: %s", usage)
	}
	if parsed.oflFile != "" && (parsed.mirror != "" || parsed.allowMirror) {
		return fixtureImportArgs{}, fmt.Errorf("GOLC_FIXTURE_USAGE: --mirror/--allow-mirror only apply to --ofl; usage: %s", usage)
	}
	return parsed, nil
}

// splitOFLRef parses a "--ofl <manufacturer>/<key>" value into its two
// parts, rejecting anything that is not exactly one non-empty
// manufacturer and one non-empty key separated by a single "/".
func splitOFLRef(raw string) (manufacturer, key string, err error) {
	idx := strings.IndexByte(raw, '/')
	if idx <= 0 || idx == len(raw)-1 {
		return "", "", fmt.Errorf("GOLC_FIXTURE_USAGE: --ofl value must be \"<manufacturer>/<key>\", got %q", raw)
	}
	return raw[:idx], raw[idx+1:], nil
}

// oflSourceFromFilename derives the "<manufacturer>/<key>" source label
// ofl.Normalize expects from a local --ofl-file path, using this repo's
// own "<manufacturer>_<key>.json" corpus naming convention (see
// tests/fixtures/ofl/README.md). A filename with no "_" is used verbatim
// as both parts (defensive default; every file in this repository's own
// corpus follows the convention).
func oflSourceFromFilename(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if idx := strings.IndexByte(base, '_'); idx >= 0 {
		return base[:idx] + "/" + base[idx+1:]
	}
	return base
}

// oflErrorExitCode maps an ofl.Fetch/ofl.Normalize error to this route's
// documented ExitCode contract: a scheme/host validation failure (caught
// before any request is issued) is a usage-shaped ExitCode 2; every other
// failure (fetch/read/normalize) is ExitCode 1.
func oflErrorExitCode(err error) int {
	message := err.Error()
	if strings.HasPrefix(message, "GOLC_FIXTURE_OFL_MIRROR_SCHEME") || strings.HasPrefix(message, "GOLC_FIXTURE_OFL_MIRROR_HOST") {
		return 2
	}
	return 1
}

// fixtureImportOutput is the canonical fixture+provenance envelope
// "fixture import" writes to --out: the full pinned FixtureDefinition
// plus its Provenance (including any LossyImportWarning entries),
// canonically encoded exactly like "linear preview"'s writePreviewPlan
// (internal/command/linear.go) writes its own plan artifacts.
type fixtureImportOutput struct {
	Definition fixture.FixtureDefinition `json:"definition"`
	Provenance fixture.Provenance        `json:"provenance"`
}

// runFixtureImport serves the self-registered "fixture import" route
// (FIXT-03/FIXT-06, D-06/D-07): it resolves exactly one source (a live,
// SSRF-guarded OFL fetch, or a local --ofl-file with no network access
// at all), runs the bytes through ofl.Normalize's canonical
// normalization + validation + pinning pipeline, and writes the pinned
// fixture + provenance to --out. Any GOLC_FIXTURE_OFL_* diagnostic
// surfaces with the ExitCode oflErrorExitCode selects.
func runFixtureImport(request Request) Result {
	usage := "fixture import --ofl <manufacturer>/<key> [--mirror <url>] [--allow-mirror] --out <path> | fixture import --ofl-file <path> --out <path>"
	parsed, err := parseFixtureImportArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	var raw []byte
	var source string
	if parsed.oflFile != "" {
		resolvedPath := resolveWritablePath(request.Root, parsed.oflFile)
		data, readErr := os.ReadFile(resolvedPath)
		if readErr != nil {
			return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_READ_FAILED: %v\n", readErr))}
		}
		raw = data
		source = oflSourceFromFilename(resolvedPath)
	} else {
		manufacturer, key, splitErr := splitOFLRef(parsed.oflRef)
		if splitErr != nil {
			return Result{ExitCode: 2, Stderr: []byte(splitErr.Error() + "\n")}
		}
		ref := ofl.OFLRef{Manufacturer: manufacturer, Key: key, Mirror: parsed.mirror, AllowMirror: parsed.allowMirror}
		fetched, fetchErr := ofl.Fetch(context.Background(), ref)
		if fetchErr != nil {
			return Result{ExitCode: oflErrorExitCode(fetchErr), Stderr: []byte(fetchErr.Error() + "\n")}
		}
		raw = fetched
		source = ref.Source()
	}

	def, provenance, normalizeErr := ofl.Normalize(raw, source)
	if normalizeErr != nil {
		return Result{ExitCode: oflErrorExitCode(normalizeErr), Stderr: []byte(normalizeErr.Error() + "\n")}
	}

	payload, encodeErr := strictjson.CanonicalEncode(fixtureImportOutput{Definition: def, Provenance: provenance})
	if encodeErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_IMPORT_ENCODE_FAILED: %v\n", encodeErr))}
	}
	destination := resolveWritablePath(request.Root, parsed.outPath)
	if writeErr := os.WriteFile(destination, payload, 0o644); writeErr != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_FIXTURE_IMPORT_WRITE_FAILED: %v\n", writeErr))}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_FIXTURE_IMPORT: wrote %s\n", destination))}
}
