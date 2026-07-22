// provenance.go implements FIXT-06's provenance record: how an author
// reviews a fixture's trustworthiness before use. Provenance carries
// source, schema version, content hash, revision, validation result, and
// the lossy/unsupported-warning list 02-03's OFL import will populate.
// Source is always repository-relative or a stable label -- never an
// absolute machine path, OS username, or credential (CONTEXT threat
// T-01-23 information-disclosure discipline).
package fixture

// LossyImportWarning names one capability that could not be represented
// exactly during import (populated by 02-03's OFL import; a hand-authored
// fixture's Provenance starts with an empty Warnings list).
type LossyImportWarning struct {
	// Severity is the warning's severity label (for example "warning" or
	// "error").
	Severity string `json:"severity"`
	// CapabilityType names the affected capability, when applicable.
	CapabilityType string `json:"capability_type,omitempty"`
	// Detail is a human-readable explanation of what was lossy or
	// unsupported.
	Detail string `json:"detail"`
}

// Provenance is a fixture's reviewable trust record: where it came from,
// its pinned identity, whether it validated, and any lossy/unsupported
// import warnings -- surfaced by "fixture inspect" before the fixture is
// used in a show (FIXT-06).
type Provenance struct {
	// Source is a repository-relative path or a stable label (for example
	// "hand-authored"), never an absolute machine path or OS username.
	Source string `json:"source"`
	// SchemaVersion mirrors the pinned Identity's SchemaVersion.
	SchemaVersion int `json:"schema_version"`
	// ContentHash mirrors the pinned Identity's ContentHash.
	ContentHash string `json:"content_hash"`
	// Revision mirrors the pinned Identity's Revision.
	Revision string `json:"revision"`
	// ValidationResult is "valid" for a fixture that decoded and pinned
	// successfully.
	ValidationResult string `json:"validation_result"`
	// Warnings is the lossy/unsupported-capability warning list; empty
	// (never nil-vs-empty-ambiguous in JSON: always an array) for a
	// hand-authored fixture with nothing to warn about.
	Warnings []LossyImportWarning `json:"warnings"`
}

// validValidationResult is the ValidationResult NewProvenance reports for
// a def that has already been successfully decoded and pinned by the
// caller -- a Provenance is only ever constructed from an already-valid
// FixtureDefinition, so "valid" is the sole value this constructor
// produces today.
const validValidationResult = "valid"

// NewProvenance builds a Provenance for def, already pinned as identity,
// sourced from source. source MUST already be a repository-relative path
// or a stable label -- NewProvenance stores it verbatim and performs no
// path resolution or redaction itself; callers (for example the "fixture
// inspect" route) are responsible for never passing an absolute
// filesystem path or OS-local detail (T-01-23). Warnings starts empty;
// callers populate it (for example 02-03's OFL import) after
// construction.
func NewProvenance(def FixtureDefinition, identity Identity, source string) Provenance {
	return Provenance{
		Source:           source,
		SchemaVersion:    identity.SchemaVersion,
		ContentHash:      identity.ContentHash,
		Revision:         identity.Revision,
		ValidationResult: validValidationResult,
		Warnings:         []LossyImportWarning{},
	}
}
