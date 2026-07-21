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
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"

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
		if !registered[name] {
			t.Fatalf("expected registered configuration schema %q, got registry %v", name, registered)
		}
	}
}

func testRegisterSchemaRejectsBlank(t *testing.T) {
	before := len(contracts.RegisteredSchemas())

	if err := contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "   ",
		OutputPath: "schemas/test-blank-name.schema.json",
		Schema:     func() *jsonschema.Schema { return &jsonschema.Schema{} },
	}); err == nil {
		t.Fatal("expected a blank name to be rejected")
	} else if !strings.Contains(err.Error(), "GOLC_CONTRACTS_NAME_EMPTY") {
		t.Fatalf("expected GOLC_CONTRACTS_NAME_EMPTY, got %v", err)
	}

	if err := contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "test-blank-output",
		OutputPath: "   ",
		Schema:     func() *jsonschema.Schema { return &jsonschema.Schema{} },
	}); err == nil {
		t.Fatal("expected a blank output path to be rejected")
	} else if !strings.Contains(err.Error(), "GOLC_CONTRACTS_OUTPUT_EMPTY") {
		t.Fatalf("expected GOLC_CONTRACTS_OUTPUT_EMPTY, got %v", err)
	}

	if err := contracts.RegisterSchema(contracts.SchemaDescriptor{
		Name:       "test-nil-factory",
		OutputPath: "schemas/test-nil-factory.schema.json",
		Schema:     nil,
	}); err == nil {
		t.Fatal("expected a nil schema factory to be rejected")
	} else if !strings.Contains(err.Error(), "GOLC_CONTRACTS_FACTORY_NIL") {
		t.Fatalf("expected GOLC_CONTRACTS_FACTORY_NIL, got %v", err)
	}

	if after := len(contracts.RegisteredSchemas()); after != before {
		t.Fatalf("expected every rejected registration to leave the registry unchanged: before=%d after=%d", before, after)
	}
}

func testRegisterSchemaRejectsDuplicates(t *testing.T) {
	descriptor, _ := newCountingDescriptor("test-duplicate-schema-alpha")
	if err := contracts.RegisterSchema(descriptor); err != nil {
		t.Fatalf("expected the first registration to succeed: %v", err)
	}

	duplicateName := descriptor
	duplicateName.OutputPath = "schemas/test-duplicate-schema-alpha-2.schema.json"
	if err := contracts.RegisterSchema(duplicateName); err == nil {
		t.Fatal("expected a duplicate name to be rejected")
	} else if !strings.Contains(err.Error(), "GOLC_CONTRACTS_NAME_DUPLICATE") {
		t.Fatalf("expected GOLC_CONTRACTS_NAME_DUPLICATE, got %v", err)
	}

	duplicatePath := descriptor
	duplicatePath.Name = "test-duplicate-schema-alpha-other-name"
	if err := contracts.RegisterSchema(duplicatePath); err == nil {
		t.Fatal("expected a duplicate output path to be rejected")
	} else if !strings.Contains(err.Error(), "GOLC_CONTRACTS_OUTPUT_DUPLICATE") {
		t.Fatalf("expected GOLC_CONTRACTS_OUTPUT_DUPLICATE, got %v", err)
	}
}

func testRegisteredSchemasSnapshot(t *testing.T) {
	before := len(contracts.RegisteredSchemas())

	descriptor, _ := newCountingDescriptor("test-snapshot-schema")
	if err := contracts.RegisterSchema(descriptor); err != nil {
		t.Fatalf("expected registration to succeed: %v", err)
	}

	snapshot := contracts.RegisteredSchemas()
	if len(snapshot) != before+1 {
		t.Fatalf("expected registry length %d after one new registration, got %d", before+1, len(snapshot))
	}

	// Mutate the returned snapshot; the package-level registry must be
	// unaffected by either mutation.
	snapshot[0].Name = "mutated-in-place"
	snapshot = append(snapshot, contracts.SchemaDescriptor{Name: "injected-by-test"})

	again := contracts.RegisteredSchemas()
	if len(again) != before+1 {
		t.Fatalf("expected mutating the returned snapshot to leave the registry unaffected: got length %d, want %d", len(again), before+1)
	}

	names := make([]string, len(again))
	for i, d := range again {
		names[i] = d.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("expected RegisteredSchemas to return stable name-sorted order, got %v", names)
	}
}

func testExactlyOnceTraversal(t *testing.T) {
	descriptor, calls := newCountingDescriptor("test-exactly-once-schema")
	if err := contracts.RegisterSchema(descriptor); err != nil {
		t.Fatalf("expected registration to succeed: %v", err)
	}

	*calls = 0
	if err := contracts.GenerateInto(t.TempDir()); err != nil {
		t.Fatalf("GenerateInto failed: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected GenerateInto to call the schema factory exactly once, got %d", *calls)
	}

	*calls = 0
	changed, err := contracts.CheckDrift(t.TempDir())
	if err != nil {
		t.Fatalf("CheckDrift failed: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected CheckDrift to call the schema factory exactly once, got %d", *calls)
	}
	if !containsString(changed, descriptor.OutputPath) {
		t.Fatalf("expected drift for a never-committed schema %q, got %v", descriptor.OutputPath, changed)
	}
}

func testGenerateAllWritesCommittedPath(t *testing.T) {
	descriptor, _ := newCountingDescriptor("test-generate-all-schema")
	if err := contracts.RegisterSchema(descriptor); err != nil {
		t.Fatalf("expected registration to succeed: %v", err)
	}

	root := t.TempDir()
	if err := contracts.GenerateAll(root); err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
	if err != nil {
		t.Fatalf("expected GenerateAll to write %s: %v", descriptor.OutputPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty generated output for %s", descriptor.OutputPath)
	}
	if data[len(data)-1] != '\n' || bytes.Contains(data, []byte("\r\n")) {
		t.Fatalf("expected LF-only output ending with exactly one trailing newline, got %q", data)
	}
}

func testDeterministicGeneration(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := contracts.GenerateInto(dirA); err != nil {
		t.Fatalf("GenerateInto(dirA) failed: %v", err)
	}
	if err := contracts.GenerateInto(dirB); err != nil {
		t.Fatalf("GenerateInto(dirB) failed: %v", err)
	}

	for _, descriptor := range contracts.RegisteredSchemas() {
		a, err := os.ReadFile(filepath.Join(dirA, filepath.FromSlash(descriptor.OutputPath)))
		if err != nil {
			t.Fatalf("read dirA %s: %v", descriptor.OutputPath, err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, filepath.FromSlash(descriptor.OutputPath)))
		if err != nil {
			t.Fatalf("read dirB %s: %v", descriptor.OutputPath, err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("expected byte-identical generation for %s across repeated runs", descriptor.OutputPath)
		}
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
		if !declared || additional != false {
			t.Fatalf("%s: expected additionalProperties:false on every object with properties, got %v", path, additional)
		}
	}
	for _, value := range object {
		assertAdditionalPropertiesFalse(t, path, value)
	}
}

func testAdditionalPropertiesFalse(t *testing.T) {
	root := t.TempDir()
	if err := contracts.GenerateInto(root); err != nil {
		t.Fatalf("GenerateInto failed: %v", err)
	}
	for _, name := range knownConfigurationDescriptors {
		outputPath := "schemas/" + name + ".schema.json"
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
		if err != nil {
			t.Fatalf("read %s: %v", outputPath, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode %s: %v", outputPath, err)
		}
		if decoded["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Fatalf("%s: expected Draft 2020-12 $schema, got %v", outputPath, decoded["$schema"])
		}
		assertAdditionalPropertiesFalse(t, outputPath, decoded)
	}
}

func testCheckDriftReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := contracts.GenerateInto(root); err != nil {
		t.Fatalf("seed GenerateInto failed: %v", err)
	}

	before := map[string][]byte{}
	for _, descriptor := range contracts.RegisteredSchemas() {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
		if err != nil {
			t.Fatalf("read seeded %s: %v", descriptor.OutputPath, err)
		}
		before[descriptor.OutputPath] = data
	}

	changed, err := contracts.CheckDrift(root)
	if err != nil {
		t.Fatalf("CheckDrift failed: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("expected zero drift against freshly seeded committed bytes, got %v", changed)
	}

	for path, want := range before {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("re-read %s after CheckDrift: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("expected CheckDrift to leave committed bytes at %s untouched", path)
		}
	}
}

func testNoLeakedSensitiveContent(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	root := t.TempDir()
	if err := contracts.GenerateInto(root); err != nil {
		t.Fatalf("GenerateInto failed: %v", err)
	}

	forbidden := []string{cwd, `C:\Users`, "/home/", "linear.app", "LINEAR_API_KEY=", "Bearer ", "sk-"}
	for _, name := range knownConfigurationDescriptors {
		outputPath := "schemas/" + name + ".schema.json"
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(outputPath)))
		if err != nil {
			t.Fatalf("read %s: %v", outputPath, err)
		}
		text := string(data)
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				t.Fatalf("%s: generated schema leaks forbidden token %q", outputPath, token)
			}
		}
	}
}

func testNormalizeHelpers(t *testing.T) {
	sorted, err := contracts.SortJSON([]byte(`{"b":1,"a":{"d":2,"c":3},"e":[{"z":1,"y":2},1,2]}`))
	if err != nil {
		t.Fatalf("SortJSON failed: %v", err)
	}
	want := `{"a":{"c":3,"d":2},"b":1,"e":[{"y":2,"z":1},1,2]}`
	if string(sorted) != want {
		t.Fatalf("SortJSON: got %s want %s", sorted, want)
	}

	lf := contracts.NormalizeLF([]byte("line1\r\nline2\r\nline3\n\n\n"))
	if string(lf) != "line1\nline2\nline3\n" {
		t.Fatalf("NormalizeLF: got %q", lf)
	}

	schema := &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
	}
	first, err := contracts.NormalizeSchema(schema)
	if err != nil {
		t.Fatalf("NormalizeSchema failed: %v", err)
	}
	second, err := contracts.NormalizeSchema(schema)
	if err != nil {
		t.Fatalf("NormalizeSchema failed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("expected NormalizeSchema to be deterministic across repeated calls")
	}
	if first[len(first)-1] != '\n' || bytes.Contains(first, []byte("\r\n")) {
		t.Fatalf("expected NormalizeSchema output to be LF-only with a single trailing newline, got %q", first)
	}
	if !bytes.Contains(first, []byte("\n  \"")) {
		t.Fatalf("expected NormalizeSchema output to use two-space indentation, got %q", first)
	}
}

// containsString reports whether needle is present in haystack.
func containsString(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if candidate == needle {
			return true
		}
	}
	return false
}
