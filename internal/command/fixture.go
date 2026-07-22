// fixture.go is the fixture command file: it owns the "fixture" routing
// scope and self-registers the "fixture validate" and "fixture inspect"
// routes (CONTEXT D-01/D-02/D-04): a show author hand-writes a fixture
// definition YAML file, "fixture validate" strictly decodes and validates
// it printing a deterministic canonical summary on success, and "fixture
// inspect" additionally surfaces the fixture's content-addressed identity
// and provenance (FIXT-05/FIXT-06) through an allowlisted JSON envelope
// before the fixture is used in a show. Neither route performs any
// scaffold/generator behavior (D-02: read-only) or network access, and
// neither ever emits an absolute filesystem path or OS-local detail
// (T-01-23).
package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lnorton89/golc/internal/fixture"
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
