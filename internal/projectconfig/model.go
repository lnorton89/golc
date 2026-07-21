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

// Shared production value shapes.
var (
	dottedVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)
	sha256Pattern        = regexp.MustCompile(`^[0-9a-f]{64}$`)
	goArchiveURLPattern  = regexp.MustCompile(`^https://go\.dev/dl/[A-Za-z0-9.\-]+\.(zip|tar\.gz)$`)
	// nodeArchiveURLPattern is the official Node.js distribution archive
	// shape (CONTEXT D-01; Plan 01-13): only the official nodejs.org/dist/
	// origin is ever an acceptable Node download source, mirroring the same
	// per-tool official-source-allowlist discipline goArchiveURLPattern
	// already establishes for Go.
	nodeArchiveURLPattern     = regexp.MustCompile(`^https://nodejs\.org/dist/v[0-9]+(\.[0-9]+)*/[A-Za-z0-9.\-]+\.(zip|tar\.gz)$`)
	officialHostPattern       = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9]+)+$`)
	officialPathPrefixPattern = regexp.MustCompile(`^/[A-Za-z0-9/_-]*/$`)
	toolsPathPattern          = regexp.MustCompile(`^\.tools(/[A-Za-z0-9._-]+)+$`)
	planningPathPattern       = regexp.MustCompile(`^\.planning(/[A-Za-z0-9._-]+)+$`)
	relativeDirPattern        = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
	fileNamePattern           = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	envVarNamePattern         = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	labelNamePattern          = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	// prStepListPattern is the flat-scalar shape commands.pr.steps must
	// satisfy: a comma-separated, ordered list of "route [--flag ...]"
	// invocations (CONTEXT D-03/D-10/D-16). It stays a single pattern-
	// matched string rather than a TOML array/table because the strict
	// single-authority decoder does not decode array-valued keys — the
	// same flat-scalar precedent config/toolchain.toml's per-tool
	// official_host/official_path_prefix keys already establish.
	prStepListPattern = regexp.MustCompile(`^[a-z][a-z0-9]*( --?[a-z0-9-]+)*(,[a-z][a-z0-9]*( --?[a-z0-9-]+)*)*$`)
	// prStepNamesPattern is the flat-scalar shape commands.pr.network_steps
	// and commands.pr.mutation_steps must satisfy: either the literal
	// "none" or a comma-separated list of bare route names.
	prStepNamesPattern = regexp.MustCompile(`^(none|[a-z][a-z0-9]*(,[a-z][a-z0-9]*)*)$`)
)

// DefaultSpec returns the production Phase 1 concern allocation: exactly
// the six concerns the root index must discover, each owning its canonical
// keys once, plus the (currently empty) production deprecation register.
func DefaultSpec() Spec {
	return Spec{
		Concerns: []ConcernSpec{
			{
				ID:   "toolchain",
				Path: "config/toolchain.toml",
				Keys: map[string]KeySpec{
					"toolchain.go.version":                {Pattern: dottedVersionPattern},
					"toolchain.go.archive_url":            {Pattern: goArchiveURLPattern},
					"toolchain.go.archive_sha256":         {Pattern: sha256Pattern},
					"toolchain.go.official_host":          {Pattern: officialHostPattern},
					"toolchain.go.official_path_prefix":   {Pattern: officialPathPrefixPattern},
					"toolchain.node.version":              {Pattern: dottedVersionPattern},
					"toolchain.node.archive_url":          {Pattern: nodeArchiveURLPattern},
					"toolchain.node.archive_sha256":       {Pattern: sha256Pattern},
					"toolchain.node.official_host":        {Pattern: officialHostPattern},
					"toolchain.node.official_path_prefix": {Pattern: officialPathPrefixPattern},
					"cache.downloads":                     {Pattern: toolsPathPattern},
					"cache.gomodcache":                    {Pattern: toolsPathPattern},
					"cache.gocache":                       {Pattern: toolsPathPattern},
				},
			},
			{
				ID:   "commands",
				Path: "config/commands.toml",
				Keys: map[string]KeySpec{
					"commands.entrypoint":        {Pattern: fileNamePattern},
					"commands.cli_binary":        {Pattern: toolsPathPattern},
					"commands.go_version":        {Pattern: dottedVersionPattern},
					"commands.pr.steps":          {Pattern: prStepListPattern},
					"commands.pr.network_steps":  {Pattern: prStepNamesPattern},
					"commands.pr.mutation_steps": {Pattern: prStepNamesPattern},
				},
			},
			{
				ID:   "generation",
				Path: "config/generation.toml",
				Keys: map[string]KeySpec{
					"generation.schemas_dir":  {Pattern: relativeDirPattern},
					"generation.line_endings": {AllowedValues: []string{"lf"}},
				},
			},
			{
				ID:   "application_defaults",
				Path: "config/application-defaults.toml",
				Keys: map[string]KeySpec{
					"application_defaults.pool_update_review": {AllowedValues: []string{"immediate", "preview"}},
					"application_defaults.scene_apply":        {AllowedValues: []string{"immediate", "preview"}},
				},
			},
			{
				ID:   "runtime",
				Path: "config/runtime.toml",
				Keys: map[string]KeySpec{
					"runtime.log_level": {AllowedValues: []string{"debug", "error", "info", "warn"}},
				},
			},
			{
				ID:   "linear",
				Path: "config/integrations/linear.toml",
				Keys: map[string]KeySpec{
					"linear.mapping_file":               {Pattern: planningPathPattern},
					"linear.env.api_key":                {Pattern: envVarNamePattern},
					"linear.env.team_id":                {Pattern: envVarNamePattern},
					"linear.taxonomy.requirement_label": {Pattern: labelNamePattern},
				},
			},
		},
		// No key has been renamed yet; the register exists so deprecations
		// are declared as reviewable metadata, never ad-hoc parser rules.
		Deprecations: []Deprecation{},
	}
}
