// model.go is the typed concern/provenance/deprecation model for the
// strict Phase 1 configuration set (CONTEXT D-05/D-09/D-10). A Spec is the
// single-authority registry: every canonical key is owned by exactly one
// concern, deprecations are machine-readable metadata with migration
// guidance, and the production allocation lives in DefaultSpec.
//
// The root index (golc.project.toml) must discover exactly the concerns a
// Spec declares; each concern file alone owns its non-overlapping keys.
package projectconfig

import (
	"fmt"
	"regexp"
)

// KeySpec constrains one canonical configuration value. A value must be a
// TOML string and satisfy either the closed AllowedValues set or Pattern.
// Any value may instead be a typed reference "ref:<canonical.key>" to the
// single authority for that value; the resolved literal must still satisfy
// this KeySpec (D-05: refer, never repeat).
type KeySpec struct {
	// AllowedValues is the closed value set; empty means Pattern applies.
	AllowedValues []string
	// Pattern is the required value shape when AllowedValues is empty.
	Pattern *regexp.Regexp
}

// ConcernSpec is one logically separated configuration concern: its stable
// id, its repository-relative file path, and the canonical keys it alone
// owns (CONF-02 single authority).
type ConcernSpec struct {
	ID   string
	Path string
	Keys map[string]KeySpec
}

// Deprecation is one machine-readable deprecation register entry (D-09):
// old/replacement keys, introduced/deprecated/optional-removal versions,
// and a non-empty migration message. Deprecated input is never silently
// rewritten; it warns with this guidance, and old-plus-replacement input
// is a hard collision error.
type Deprecation struct {
	// OldKey is the deprecated canonical key. It is not owned by any
	// concern; it is recognized only in the file of the concern that owns
	// ReplacementKey.
	OldKey string
	// ReplacementKey is the owned canonical key that supersedes OldKey.
	ReplacementKey string
	// IntroducedIn is the version that introduced OldKey.
	IntroducedIn string
	// DeprecatedIn is the version that deprecated OldKey.
	DeprecatedIn string
	// RemovalPlanned is the optional version at which OldKey stops being
	// recognized; empty means no removal is scheduled yet.
	RemovalPlanned string
	// Message is the non-empty actionable migration guidance.
	Message string
}

// Spec is the complete strict configuration model: the concern set with
// its single-authority key registry plus the deprecation register.
type Spec struct {
	Concerns     []ConcernSpec
	Deprecations []Deprecation
}

// Diagnostic is one stable, safe validation finding. Origin is always a
// repository-relative file path — never an environment value, credential,
// or absolute machine path.
type Diagnostic struct {
	Code    string
	Key     string
	Origin  string
	Message string
}

// String renders the diagnostic in the stable "CODE: detail" shape shared
// by every projectconfig failure.
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s: %s (%s): %s", d.Code, d.Key, d.Origin, d.Message)
}

// DefaultSpec returns the production Phase 1 concern allocation.
func DefaultSpec() Spec {
	return Spec{}
}
