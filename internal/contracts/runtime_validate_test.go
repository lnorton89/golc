// runtime_validate_test.go closes a gap CheckDrift never covers: CheckDrift
// (generate_test.go) only proves each committed schemas/*.schema.json is a
// byte-stable reflection of its authoritative Go type -- nothing
// previously proved a real, checked-in document (golc.project.toml,
// config/toolchain.toml, a hand-authored fixture YAML, etc.) actually
// validates against the schema that claims to describe it. This file
// compiles each committed schema with santhosh-tekuri/jsonschema/v6 (a
// real Draft 2020-12 validator, independent of invopop/jsonschema's own
// generation code) and validates a real on-disk document against it.
//
// schemas/*.schema.json is JSON, but most of the documents it describes
// are TOML or YAML source files: each decodes through its native format
// (BurntSushi/toml, go.yaml.in/yaml/v4) into a generic Go value, then
// round-trips through encoding/json before jsonschema.UnmarshalJSON
// re-decodes it -- normalizing every native decoder's own type quirks
// (BurntSushi's int64 vs JSON's float64, YAML's own scalar tags) into
// exactly the primitive shapes santhosh-tekuri/jsonschema expects, rather
// than risking a false failure from an unrecognized Go type.
package contracts_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"

	"github.com/lnorton89/golc/internal/contracts"
)

// runtimeValidationSkipReasons documents every registered schema this
// file deliberately does not exercise against a real document, and why.
// TestRuntimeDocumentValidationCoversRegistry fails loudly if a
// registered schema is neither covered by a runtimeValidationCases entry
// nor listed here, so a newly registered schema can never silently go
// unvalidated.
var runtimeValidationSkipReasons = map[string]string{
	"linear-plan":   "ephemeral command-output artifact (linear preview/apply); no committed real-world example exists on disk",
	"linear-report": "ephemeral command-output artifact (linear preview/apply); no committed real-world example exists on disk",
}

// runtimeValidationCase pairs one registered schema name with a real,
// checked-in document it is expected to accept and how to decode that
// document into a JSON-compatible generic value.
type runtimeValidationCase struct {
	schemaName string
	docPath    string
	decode     func(path string) (any, error)
}

var runtimeValidationCases = []runtimeValidationCase{
	{schemaName: "golc-project", docPath: "golc.project.toml", decode: decodeTOMLDocument},
	{schemaName: "config-toolchain", docPath: filepath.Join("config", "toolchain.toml"), decode: decodeTOMLDocument},
	{schemaName: "config-commands", docPath: filepath.Join("config", "commands.toml"), decode: decodeTOMLDocument},
	{schemaName: "config-generation", docPath: filepath.Join("config", "generation.toml"), decode: decodeTOMLDocument},
	{schemaName: "config-application-defaults", docPath: filepath.Join("config", "application-defaults.toml"), decode: decodeTOMLDocument},
	{schemaName: "config-runtime", docPath: filepath.Join("config", "runtime.toml"), decode: decodeTOMLDocument},
	{schemaName: "config-linear", docPath: filepath.Join("config", "integrations", "linear.toml"), decode: decodeTOMLDocument},
	{schemaName: "linear-map", docPath: filepath.Join(".planning", "linear-map.json"), decode: decodeJSONDocument},
	{schemaName: "fixture", docPath: filepath.Join("fixtures", "chauvet-dj_colorband-t3bt.yaml"), decode: decodeYAMLDocument},
}

// decodeTOMLDocument decodes a TOML file into a generic map, then
// roundTripThroughJSON normalizes it for jsonschema.Validate.
func decodeTOMLDocument(path string) (any, error) {
	document := map[string]any{}
	if _, err := toml.DecodeFile(path, &document); err != nil {
		return nil, err
	}
	return roundTripThroughJSON(document)
}

// decodeYAMLDocument decodes a YAML file into a generic value, then
// roundTripThroughJSON normalizes it for jsonschema.Validate.
func decodeYAMLDocument(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	return roundTripThroughJSON(document)
}

// decodeJSONDocument decodes an already-JSON file directly through
// jsonschema.UnmarshalJSON -- no format conversion needed.
func decodeJSONDocument(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// roundTripThroughJSON marshals v (a TOML/YAML decoder's own Go value)
// to JSON and back through jsonschema.UnmarshalJSON, so jsonschema.Validate
// always receives exactly the primitive types a native JSON decode would
// have produced.
func roundTripThroughJSON(v any) (any, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
}

// schemaOutputPaths maps every registered schema's stable name to its
// committed repository-relative path -- the single source SchemaDescriptor
// already owns, never re-derived by string concatenation here.
func schemaOutputPaths() map[string]string {
	paths := make(map[string]string, len(contracts.RegisteredSchemas()))
	for _, descriptor := range contracts.RegisteredSchemas() {
		paths[descriptor.Name] = descriptor.OutputPath
	}
	return paths
}

// TestRuntimeDocumentValidation proves each committed schema in
// runtimeValidationCases actually accepts the real, checked-in document
// it describes, compiled and validated by santhosh-tekuri/jsonschema/v6 --
// a real Draft 2020-12 validator, independent of invopop/jsonschema's own
// generation code path.
func TestRuntimeDocumentValidation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "resolve repository root")

	outputPaths := schemaOutputPaths()

	for _, tc := range runtimeValidationCases {
		t.Run(tc.schemaName, func(t *testing.T) {
			outputPath, registered := outputPaths[tc.schemaName]
			require.True(t, registered, "schema %q is not registered in internal/contracts", tc.schemaName)

			compiler := jsonschema.NewCompiler()
			schema, err := compiler.Compile(filepath.Join(root, filepath.FromSlash(outputPath)))
			require.NoError(t, err, "compile committed schema %s", outputPath)

			docPath := filepath.Join(root, tc.docPath)
			instance, err := tc.decode(docPath)
			require.NoError(t, err, "decode real document %s", tc.docPath)

			require.NoError(t, schema.Validate(instance),
				"committed schema %s must accept real document %s", outputPath, tc.docPath)
		})
	}
}

// TestRuntimeDocumentValidationCoversRegistry proves every schema
// RegisteredSchemas returns is either exercised by runtimeValidationCases
// or explicitly acknowledged in runtimeValidationSkipReasons -- a newly
// registered schema can never silently ship with no runtime-document
// proof and no documented reason why not.
func TestRuntimeDocumentValidationCoversRegistry(t *testing.T) {
	covered := make(map[string]struct{}, len(runtimeValidationCases))
	for _, tc := range runtimeValidationCases {
		covered[tc.schemaName] = struct{}{}
	}
	for _, descriptor := range contracts.RegisteredSchemas() {
		_, hasCase := covered[descriptor.Name]
		_, skipped := runtimeValidationSkipReasons[descriptor.Name]
		require.True(t, hasCase || skipped,
			"schema %q is registered but neither validated against a real document (runtimeValidationCases) "+
				"nor listed with a reason in runtimeValidationSkipReasons", descriptor.Name)
	}
}
