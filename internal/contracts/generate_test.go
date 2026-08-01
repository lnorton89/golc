// generate_test.go proves the deterministic strict Draft 2020-12 contract
// generator contract (CONTEXT D-08): blank/duplicate schema descriptor
// rejection, RegisteredSchemas' defensive-copy and stable name-sorted
// ordering, exactly-once GenerateInto/CheckDrift traversal, the presence
// of every Phase 1 configuration descriptor (without imposing a registry-
// size ceiling), deterministic byte-identical generation, universal
// additionalProperties:false, and read-only drift comparison that never
// rewrites a "committed" target.
//
// It is an external test package (like internal/projectconfig's
// local_test.go, strict_test.go, and resolve_test.go) so it can declare
// its quick-test scope through the command package's exact registration
// entrypoint without an import cycle.
package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/contracts"
)

// The contracts quick-test scope is declared through the exact production
// entrypoint (01-VALIDATION: every owning Go test file registers its
// scope beside its TestScope marker; duplicate scope declarations fail
// when the default registry is built, before any handler could run).
var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "contracts",
	Summary: "Deterministic strict Draft 2020-12 contract generation, registry, and drift tests.",
})

// knownConfigurationDescriptors are the seven Phase 1 configuration
// schema names this plan registers. Tests assert these are present as a
// subset of the registry rather than asserting an exact registry length,
// so a later plan can extend the registry (for example a Linear mapping
// or plan schema) without breaking this test.
var knownConfigurationDescriptors = []string{
	"golc-project",
	"config-toolchain",
	"config-commands",
	"config-generation",
	"config-application-defaults",
	"config-runtime",
	"config-linear",
}

func TestScopeContracts(t *testing.T) {
	t.Run("known configuration descriptors are registered", testKnownDescriptors)
	t.Run("RegisterSchema rejects blank and nil-factory descriptors", testRegisterSchemaRejectsBlank)
	t.Run("RegisterSchema rejects duplicate names and output paths", testRegisterSchemaRejectsDuplicates)
	t.Run("RegisteredSchemas returns a defensive stable name-sorted snapshot", testRegisteredSchemasSnapshot)
	t.Run("GenerateInto and CheckDrift traverse the registry exactly once", testExactlyOnceTraversal)
	t.Run("GenerateAll writes every registered schema to its committed path", testGenerateAllWritesCommittedPath)
	t.Run("generation is deterministic and byte-identical across repeated runs", testDeterministicGeneration)
	t.Run("every generated object denies additional properties", testAdditionalPropertiesFalse)
	t.Run("CheckDrift reports changed paths without touching a committed target", testCheckDriftReadOnly)
	t.Run("generated schemas carry no timestamp machine path or credential", testNoLeakedSensitiveContent)
	t.Run("NormalizeSchema SortJSON and NormalizeLF produce stable LF output", testNormalizeHelpers)
}

// newCountingDescriptor returns a minimal valid descriptor plus a pointer
// to a call counter its factory increments, so a test can assert exactly
// how many times GenerateInto/CheckDrift invoked it.
func newCountingDescriptor(name string) (contracts.SchemaDescriptor, *int) {
	calls := 0
	descriptor := contracts.SchemaDescriptor{
		Name:       name,
		OutputPath: "schemas/" + name + ".schema.json",
		Schema: func() *jsonschema.Schema {
			calls++
			return &jsonschema.Schema{Type: "object", AdditionalProperties: jsonschema.FalseSchema}
		},
	}
	return descriptor, &calls
}

func testKnownDescriptors(t *testing.T) {
	registered := map[string]bool{}
	for _, descriptor := range contracts.RegisteredSchemas() {
		registered[descriptor.Name] = true
	}
	for _, name := range knownConfigurationDescriptors {
		require.True(t, registered[name], "expected registered configuration schema %q, got registry %v", name, registered)
	}
}

func testRegisterSchemaRejectsBlank(t *testing.T) {
	before := len(contracts.RegisteredSchemas())

	err := contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "   ",
		OutputPath: "schemas/test-blank-name.schema.json",
		Schema:     func() *jsonschema.Schema { return &jsonschema.Schema{} },
	})
	require.ErrorContains(t, err, "GOLC_CONTRACTS_NAME_EMPTY", "expected a blank name to be rejected")

	err = contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "test-blank-output",
		OutputPath: "   ",
		Schema:     func() *jsonschema.Schema { return &jsonschema.Schema{} },
	})
	require.ErrorContains(t, err, "GOLC_CONTRACTS_OUTPUT_EMPTY", "expected a blank output path to be rejected")

	err = contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "test-nil-factory",
		OutputPath: "schemas/test-nil-factory.schema.json",
		Schema:     nil,
	})
	require.ErrorContains(t, err, "GOLC_CONTRACTS_FACTORY_NIL", "expected a nil schema factory to be rejected")

	require.Equal(t, before, len(contracts.RegisteredSchemas()), "expected every rejected registration to leave the registry unchanged")
}

func testRegisterSchemaRejectsDuplicates(t *testing.T) {
	descriptor, _ := newCountingDescriptor("test-duplicate-schema-alpha")
	require.NoError(t, contracts.RegisterSchema(descriptor), "expected the first registration to succeed")

	duplicateName := descriptor
	duplicateName.OutputPath = "schemas/test-duplicate-schema-alpha-2.schema.json"
	err := contracts.RegisterSchema(duplicateName)
	require.ErrorContains(t, err, "GOLC_CONTRACTS_NAME_DUPLICATE", "expected a duplicate name to be rejected")

	duplicatePath := descriptor
	duplicatePath.Name = "test-duplicate-schema-alpha-other-name"
	err = contracts.RegisterSchema(duplicatePath)
	require.ErrorContains(t, err, "GOLC_CONTRACTS_OUTPUT_DUPLICATE", "expected a duplicate output path to be rejected")
}

func testRegisteredSchemasSnapshot(t *testing.T) {
	before := len(contracts.RegisteredSchemas())

	descriptor, _ := newCountingDescriptor("test-snapshot-schema")
	require.NoError(t, contracts.RegisterSchema(descriptor), "expected registration to succeed")

	snapshot := contracts.RegisteredSchemas()
	require.Len(t, snapshot, before+1, "expected registry length %d after one new registration", before+1)

	// Mutate the returned snapshot; the package-level registry must be
	// unaffected by either mutation.
	snapshot[0].Name = "mutated-in-place"
	snapshot = append(snapshot, contracts.SchemaDescriptor{Name: "injected-by-test"})

	again := contracts.RegisteredSchemas()
	require.Len(t, again, before+1, "expected mutating the returned snapshot to leave the registry unaffected")

	names := make([]string, len(again))
	for i, d := range again {
		names[i] = d.Name
	}
	require.True(t, sort.StringsAreSorted(names), "expected RegisteredSchemas to return stable name-sorted order, got %v", names)
}

func testExactlyOnceTraversal(t *testing.T) {
	descriptor, calls := newCountingDescriptor("test-exactly-once-schema")
	require.NoError(t, contracts.RegisterSchema(descriptor), "expected registration to succeed")

	*calls = 0
	require.NoError(t, contracts.GenerateInto(t.TempDir()), "GenerateInto failed")
	require.Equal(t, 1, *calls, "expected GenerateInto to call the schema factory exactly once")

	*calls = 0
	changed, err := contracts.CheckDrift(t.TempDir())
	require.NoError(t, err, "CheckDrift failed")
	require.Equal(t, 1, *calls, "expected CheckDrift to call the schema factory exactly once")
	require.Contains(t, changed, descriptor.OutputPath, "expected drift for a never-committed schema %q, got %v", descriptor.OutputPath, changed)
}

func testGenerateAllWritesCommittedPath(t *testing.T) {
	descriptor, _ := newCountingDescriptor("test-generate-all-schema")
	require.NoError(t, contracts.RegisterSchema(descriptor), "expected registration to succeed")

	root := t.TempDir()
	require.NoError(t, contracts.GenerateAll(root), "GenerateAll failed")

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
	require.NoError(t, err, "expected GenerateAll to write %s", descriptor.OutputPath)
	require.NotEmpty(t, data, "expected non-empty generated output for %s", descriptor.OutputPath)
	require.Equal(t, byte('\n'), data[len(data)-1], "expected LF-only output ending with exactly one trailing newline, got %q", data)
	require.NotContains(t, string(data), "\r\n", "expected LF-only output ending with exactly one trailing newline, got %q", data)
}

func testDeterministicGeneration(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	require.NoError(t, contracts.GenerateInto(dirA), "GenerateInto(dirA) failed")
	require.NoError(t, contracts.GenerateInto(dirB), "GenerateInto(dirB) failed")

	for _, descriptor := range contracts.RegisteredSchemas() {
		a, err := os.ReadFile(filepath.Join(dirA, filepath.FromSlash(descriptor.OutputPath)))
		require.NoError(t, err, "read dirA %s", descriptor.OutputPath)
		b, err := os.ReadFile(filepath.Join(dirB, filepath.FromSlash(descriptor.OutputPath)))
		require.NoError(t, err, "read dirB %s", descriptor.OutputPath)
		require.Equal(t, a, b, "expected byte-identical generation for %s across repeated runs", descriptor.OutputPath)
	}
}

// assertAdditionalPropertiesFalse recursively walks a decoded JSON schema
// document: every node that declares "properties" must also declare
// "additionalProperties": false.
func assertAdditionalPropertiesFalse(t *testing.T, path string, node any) {
	t.Helper()
	object, isObject := node.(map[string]any)
	if !isObject {
		return
	}
	if _, hasProperties := object["properties"]; hasProperties {
		additional, declared := object["additionalProperties"]
		require.True(t, declared && additional == false, "%s: expected additionalProperties:false on every object with properties, got %v", path, additional)
	}
	for _, value := range object {
		assertAdditionalPropertiesFalse(t, path, value)
	}
}

func testAdditionalPropertiesFalse(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, contracts.GenerateInto(root), "GenerateInto failed")
	for _, name := range knownConfigurationDescriptors {
		outputPath := "schemas/" + name + ".schema.json"
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
		require.NoError(t, err, "read %s", outputPath)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(data, &decoded), "decode %s", outputPath)
		require.Equal(t, "https://json-schema.org/draft/2020-12/schema", decoded["$schema"], "%s: expected Draft 2020-12 $schema", outputPath)
		assertAdditionalPropertiesFalse(t, outputPath, decoded)
	}
}

func testCheckDriftReadOnly(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, contracts.GenerateInto(root), "seed GenerateInto failed")

	before := map[string][]byte{}
	for _, descriptor := range contracts.RegisteredSchemas() {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
		require.NoError(t, err, "read seeded %s", descriptor.OutputPath)
		before[descriptor.OutputPath] = data
	}

	changed, err := contracts.CheckDrift(root)
	require.NoError(t, err, "CheckDrift failed")
	require.Empty(t, changed, "expected zero drift against freshly seeded committed bytes, got %v", changed)

	for path, want := range before {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err, "re-read %s after CheckDrift", path)
		require.Equal(t, want, got, "expected CheckDrift to leave committed bytes at %s untouched", path)
	}
}

func testNoLeakedSensitiveContent(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err, "os.Getwd")
	root := t.TempDir()
	require.NoError(t, contracts.GenerateInto(root), "GenerateInto failed")

	forbidden := []string{cwd, `C:\Users`, "/home/", "linear.app", "LINEAR_API_KEY=", "Bearer ", "sk-"}
	for _, name := range knownConfigurationDescriptors {
		outputPath := "schemas/" + name + ".schema.json"
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
		require.NoError(t, err, "read %s", outputPath)
		text := string(data)
		for _, token := range forbidden {
			require.NotContains(t, text, token, "%s: generated schema leaks forbidden token %q", outputPath, token)
		}
	}
}

func testNormalizeHelpers(t *testing.T) {
	sorted, err := contracts.SortJSON([]byte(`{"b":1,"a":{"d":2,"c":3},"e":[{"z":1,"y":2},1,2]}`))
	require.NoError(t, err, "SortJSON failed")
	want := `{"a":{"c":3,"d":2},"b":1,"e":[{"y":2,"z":1},1,2]}`
	require.Equal(t, want, string(sorted))

	lf := contracts.NormalizeLF([]byte("line1\r\nline2\r\nline3\n\n\n"))
	require.Equal(t, "line1\nline2\nline3\n", string(lf))

	schema := &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
	}
	first, err := contracts.NormalizeSchema(schema)
	require.NoError(t, err, "NormalizeSchema failed")
	second, err := contracts.NormalizeSchema(schema)
	require.NoError(t, err, "NormalizeSchema failed")
	require.Equal(t, first, second, "expected NormalizeSchema to be deterministic across repeated calls")
	require.Equal(t, byte('\n'), first[len(first)-1], "expected NormalizeSchema output to be LF-only with a single trailing newline, got %q", first)
	require.NotContains(t, string(first), "\r\n", "expected NormalizeSchema output to be LF-only with a single trailing newline, got %q", first)
	require.Contains(t, string(first), "\n  \"", "expected NormalizeSchema output to use two-space indentation, got %q", first)
}
