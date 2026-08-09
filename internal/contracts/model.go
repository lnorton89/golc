// model.go declares the seven Phase 1 configuration schema projections
// (CONTEXT D-08) and self-registers each through MustRegisterSchema: the
// root index and every strict single-authority concern
// internal/projectconfig/model.go's DefaultSpec allocates
// (config/toolchain.toml, config/commands.toml, config/generation.toml,
// config/application-defaults.toml, config/runtime.toml, and
// config/integrations/linear.toml).
//
// These are new, purpose-built authoritative Go types: internal/
// projectconfig models each concern as a dynamic map[string]KeySpec
// registry rather than a typed struct, so there is no existing Go type to
// reflect directly. The field-level shape (nesting, key names, value
// constraints) mirrors the concern files' committed content exactly.
//
// Struct-tag pattern escaping: a jsonschema struct tag value is parsed at
// runtime through reflect.StructTag.Lookup, which applies strconv.Unquote
// (Go string-literal escaping) to the quoted tag text. An unescaped
// backslash — for example the literal-dot escape "\." a plain regexp
// source string would use — is not a valid Go string-literal escape
// sequence, so the whole "jsonschema" tag silently fails to parse
// (verified empirically). Every pattern below instead uses a bracket
// character class, for example "[.]" in place of "\.", which matches an
// identical literal dot without requiring any backslash in the tag text.
package contracts

import "github.com/invopop/jsonschema"

// newReflector returns the one Reflector configuration every projection
// in this file reflects through: Draft 2020-12 by default, and
// DoNotReference so every generated schema is self-contained (none of
// these flat projections have a nested type worth extracting into a
// shared $defs entry).
func newReflector() *jsonschema.Reflector {
	return &jsonschema.Reflector{DoNotReference: true}
}

// RootIndexSchema projects golc.project.toml (CONTEXT D-05): the root
// index owns only schema metadata and the ordered list of concern files
// it discovers; it never holds a concern's own values.
type RootIndexSchema struct {
	SchemaVersion int                   `json:"schema_version" jsonschema:"required,enum=2,description=Supported root index schema version."`
	Concerns      []RootIndexConcernRef `json:"concerns" jsonschema:"required,minItems=1,uniqueItems=true,description=Ordered concern id/path pairs the root index discovers exactly (D-05)."`
}

// RootIndexConcernRef is one discovered concern: its id and the
// repository-relative path to the file that alone owns its values.
type RootIndexConcernRef struct {
	ID   string `json:"id" jsonschema:"required,pattern=^[a-z0-9][a-z0-9_-]*$,description=Concern id; matches exactly one internal/projectconfig ConcernSpec.ID."`
	Path string `json:"path" jsonschema:"required,pattern=^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$,description=Repository-relative concern file path."`
}

// ToolchainSchema projects config/toolchain.toml (concern "toolchain"):
// every checksum-pinned per-platform archive tool ([toolchain.go/node/
// deno/mage]), every go-install-provisioned CLI tool ([go_install.<name>]),
// and the repository-local cache paths bootstrap treats as invariant
// input (D-04).
type ToolchainSchema struct {
	SchemaVersion int                              `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	Toolchain     ToolchainToolsBlock              `json:"toolchain" jsonschema:"required,description=Checksum-pinned per-platform toolchain archives."`
	GoInstall     map[string]ToolchainGoInstallPin `json:"go_install" jsonschema:"required,description=Go-ecosystem CLI tools provisioned via a go install invocation, keyed by the tool's own binary basename."`
	Cache         ToolchainCacheBlock              `json:"cache" jsonschema:"required,description=Repository-local cache directories."`
}

// ToolchainToolsBlock wraps every "[toolchain.<name>]" checksum-pinned
// archive tool this concern currently declares.
type ToolchainToolsBlock struct {
	Go   ToolchainArchiveTool `json:"go" jsonschema:"required,description=Pinned Go toolchain identity -- bootstrap and every other tool's own go install step depend on it."`
	Node ToolchainArchiveTool `json:"node" jsonschema:"required,description=Pinned Node identity, used only by the isolated tools/linear-sync workspace and frontend/design-system tooling."`
	Deno ToolchainArchiveTool `json:"deno" jsonschema:"required,description=Pinned Deno identity, the process-isolated TypeScript automation sandbox runtime."`
	Mage ToolchainArchiveTool `json:"mage" jsonschema:"required,description=Pinned Mage identity, the sole contributor build entrypoint."`
}

// ToolchainArchiveTool is one checksum-pinned tool's identity plus its
// acquisition allowlist and per-platform archive pins.
type ToolchainArchiveTool struct {
	Version            string                         `json:"version" jsonschema:"required,pattern=^[0-9]+([.][0-9]+)*$,description=Exact pinned tool version."`
	OfficialHost       string                         `json:"official_host" jsonschema:"required,description=Allowlisted acquisition host (internal/bootstrap's OfficialSourcePolicy)."`
	OfficialPathPrefix string                         `json:"official_path_prefix" jsonschema:"required,description=Allowlisted acquisition URL path prefix."`
	Platforms          map[string]ToolchainArchivePin `json:"platforms" jsonschema:"required,description=Per-platform checksum-pinned archive, keyed by \"<os>-<arch>\", e.g. \"windows-amd64\"."`
}

// ToolchainArchivePin is one platform's exact archive download URL and
// content checksum.
type ToolchainArchivePin struct {
	ArchiveURL    string `json:"archive_url" jsonschema:"required,format=uri,description=Official archive download URL."`
	ArchiveSHA256 string `json:"archive_sha256" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=Lowercase hex SHA-256 of the archive."`
}

// ToolchainGoInstallPin is one "go install <module>@<version>"-provisioned
// tool's pin (installGoInstallTools, internal/bootstrap/engine.go):
// best-effort/non-fatal contributor tooling, never a build/test
// dependency of the application itself.
type ToolchainGoInstallPin struct {
	Version string `json:"version" jsonschema:"required,pattern=^v[0-9]+[.][0-9]+[.][0-9]+$,description=Exact pinned go install module version tag."`
	Module  string `json:"module" jsonschema:"required,description=Fully qualified go install module path."`
}

// ToolchainCacheBlock is the repository-local cache directory set.
type ToolchainCacheBlock struct {
	Downloads  string `json:"downloads" jsonschema:"required,pattern=^[.]tools(/[A-Za-z0-9._-]+)+$,description=Content-addressed verified download cache directory."`
	GoModCache string `json:"gomodcache" jsonschema:"required,pattern=^[.]tools(/[A-Za-z0-9._-]+)+$,description=Repository-local GOMODCACHE directory."`
	GoCache    string `json:"gocache" jsonschema:"required,pattern=^[.]tools(/[A-Za-z0-9._-]+)+$,description=Repository-local GOCACHE directory."`
}

// CommandsSchema projects config/commands.toml (concern "commands"): the
// supported repository command surface.
type CommandsSchema struct {
	SchemaVersion int           `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	Commands      CommandsBlock `json:"commands" jsonschema:"required"`
}

// CommandsBlock is the delegated CLI location plus the Windows PR CI
// policy. Mage (magefiles/magefile.go) is the sole contributor entrypoint
// and needs no configured script path here.
type CommandsBlock struct {
	CLIBinary string `json:"cli_binary" jsonschema:"required,pattern=^[.]tools(/[A-Za-z0-9._-]+)+$,description=Delegated project-local CLI binary path."`
	// GoVersion is committed as a typed "ref:toolchain.go.version" cross-
	// concern reference (D-05: refer, never repeat), not a literal dotted
	// version, so its pattern accepts either shape.
	GoVersion string           `json:"go_version" jsonschema:"required,pattern=^([0-9]+([.][0-9]+)*|ref:[a-z0-9_]+([.][a-z0-9_]+)*)$,description=Pinned Go version; committed as a typed ref: pointer to the toolchain concern's single authority per D-05."`
	PR        CommandsPRBlock  `json:"pr" jsonschema:"required,description=The exact ordered Windows PR command graph, plus its network and mutation policy (internal/delivery.LoadPRGraph is the sole parser)."`
}

// CommandsPRBlock is the strict Windows PR CI policy (D-03/D-10/D-16):
// every value is a flat, comma-separated string -- never a TOML array or
// table array -- because the strict single-authority concern decoder
// (internal/projectconfig) does not decode array-valued keys.
type CommandsPRBlock struct {
	Steps         string `json:"steps" jsonschema:"required,description=Exact ordered comma-separated \"route [--flag ...]\" invocations the PR workflow may run."`
	NetworkSteps  string `json:"network_steps" jsonschema:"required,description=Comma-separated subset of Steps' routes permitted to open a network connection, or \"none\"."`
	MutationSteps string `json:"mutation_steps" jsonschema:"required,description=Comma-separated subset of Steps' routes permitted to mutate anything remote; must be \"none\" (D-16: pull-request CI is read-only end to end)."`
}

// GenerationSchema projects config/generation.toml (concern "generation"):
// where generated review artifacts live and how generated bytes are
// normalized (D-08).
type GenerationSchema struct {
	SchemaVersion int             `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	Generation    GenerationBlock `json:"generation" jsonschema:"required"`
}

// GenerationBlock is the committed generation policy.
type GenerationBlock struct {
	SchemasDir  string `json:"schemas_dir" jsonschema:"required,pattern=^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$,description=Repository-relative directory committed generated schemas live under."`
	LineEndings string `json:"line_endings" jsonschema:"required,enum=lf,description=Normalized generated-file line ending; D-08 requires byte-stable LF."`
}

// ApplicationDefaultsSchema projects config/application-defaults.toml
// (concern "application_defaults"): committed product-behavior defaults.
type ApplicationDefaultsSchema struct {
	SchemaVersion       int                      `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	ApplicationDefaults ApplicationDefaultsBlock `json:"application_defaults" jsonschema:"required"`
}

// ApplicationDefaultsBlock is the committed product-behavior default set.
type ApplicationDefaultsBlock struct {
	PoolUpdateReview string `json:"pool_update_review" jsonschema:"required,enum=immediate,enum=preview,description=Fixture-pool update propagation default."`
	SceneApply       string `json:"scene_apply" jsonschema:"required,enum=immediate,enum=preview,description=Scene/layer change application default."`
}

// RuntimeSchema projects config/runtime.toml (concern "runtime"):
// committed runtime configuration defaults.
type RuntimeSchema struct {
	SchemaVersion int          `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	Runtime       RuntimeBlock `json:"runtime" jsonschema:"required"`
}

// RuntimeBlock is the committed runtime default set.
type RuntimeBlock struct {
	LogLevel string `json:"log_level" jsonschema:"required,enum=debug,enum=error,enum=info,enum=warn,description=Committed default log level; the sole writable canonical key across every override layer per D-06/D-07."`
}

// LinearSchema projects config/integrations/linear.toml (concern
// "linear"): non-secret Linear taxonomy and the documented environment
// variable names explicit Linear commands read (D-19). It never models a
// credential value or a remote UUID.
type LinearSchema struct {
	SchemaVersion int         `json:"schema_version" jsonschema:"required,enum=2,description=Supported concern schema version."`
	Linear        LinearBlock `json:"linear" jsonschema:"required"`
}

// LinearBlock is the committed Linear integration declaration set.
type LinearBlock struct {
	MappingFile string              `json:"mapping_file" jsonschema:"required,pattern=^[.]planning(/[A-Za-z0-9._-]+)+$,description=Repository-relative durable local identity map path."`
	Env         LinearEnvBlock      `json:"env" jsonschema:"required,description=Documented environment variable names only; never credential values per D-19."`
	Taxonomy    LinearTaxonomyBlock `json:"taxonomy" jsonschema:"required,description=Linear taxonomy labels."`
}

// LinearEnvBlock names the environment variables Linear commands read.
// Every field is an environment variable NAME, never a credential value.
type LinearEnvBlock struct {
	APIKey string `json:"api_key" jsonschema:"required,pattern=^[A-Z][A-Z0-9_]*$,description=Environment variable name carrying the Linear API key; never the credential value itself."`
	TeamID string `json:"team_id" jsonschema:"required,pattern=^[A-Z][A-Z0-9_]*$,description=Environment variable name carrying the Linear team id."`
}

// LinearTaxonomyBlock is the committed non-secret Linear label taxonomy.
type LinearTaxonomyBlock struct {
	RequirementLabel string `json:"requirement_label" jsonschema:"required,pattern=^[a-z][a-z0-9-]*$,description=Linear label applied to requirement issues."`
}

// The seven Phase 1 configuration projections self-register through the
// exact compile-safe entrypoint generate.go declares (mirrors CONTEXT
// D-03's MustDeclareRoute/MustDeclareScope idiom): a duplicate name or
// output path panics at program startup, before any generation or drift
// check could run against a partially registered set.

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "golc-project",
	OutputPath: "schemas/golc-project.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&RootIndexSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-toolchain",
	OutputPath: "schemas/config-toolchain.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&ToolchainSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-commands",
	OutputPath: "schemas/config-commands.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&CommandsSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-generation",
	OutputPath: "schemas/config-generation.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&GenerationSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-application-defaults",
	OutputPath: "schemas/config-application-defaults.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&ApplicationDefaultsSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-runtime",
	OutputPath: "schemas/config-runtime.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&RuntimeSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "config-linear",
	OutputPath: "schemas/config-linear.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&LinearSchema{}) },
})
