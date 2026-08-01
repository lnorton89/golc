// generate_test.go proves the deterministic, drift-checked typed GOLC SDK
// generator contract (08-03-PLAN.md Task 1): duplicate/invalid descriptor
// rejection, RegisteredSDKMethods' stable Route-sorted ordering regardless
// of registration order, exactly-twice byte-identical GenerateInto output,
// CheckDrift's empty-vs-non-empty drift reporting, CheckDrift's read-only
// guarantee against the committed target, and golc.d.ts's ambient-global,
// zero-import/zero-export shape for a fixture descriptor set.
//
// It is an external test package (like internal/contracts' generate_test.go)
// registering its own quick-test scope through the exact production
// entrypoint, without an import cycle back into internal/command.
package scriptsdk_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/show"
)

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "scriptsdk",
	Summary: "Deterministic, drift-checked typed GOLC SDK generator tests.",
})

// fixtureParams/fixtureResult are minimal flat Go types this file's fixture
// descriptors reflect through -- every field is one of the closed set of
// shapes lowerScalar renders (string, number, boolean, array of one of
// those).
type fixtureParams struct {
	Name string `json:"name"`
	Show string `json:"show"`
}

type fixtureResult struct {
	Message string `json:"message"`
}

func TestScopeScriptsdk(t *testing.T) {
	t.Run("RegisterSDKMethod rejects a duplicate Route and an invalid descriptor", testRegisterSDKMethodRejects)
	t.Run("RegisteredSDKMethods returns descriptors in stable Route-sorted order", testRegisteredSDKMethodsSorted)
	t.Run("GenerateInto twice into two different temp dirs produces byte-identical output", testGenerateIntoDeterministic)
	t.Run("CheckDrift reports empty then the changed path after a committed mutation", testCheckDriftReporting)
	t.Run("CheckDrift never writes to the committed target", testCheckDriftReadOnly)
	t.Run("rendered golc.d.ts is an ambient global namespace with no import or export", testRenderedTypesShape)
}

func testRegisterSDKMethodRejects(t *testing.T) {
	err := scriptsdk.RegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture alpha",
		Method: "sdkfixture.alpha",
		Scope:  show.APIKeyScopePlayback,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})
	require.NoError(t, err, "expected the first registration to succeed")

	err = scriptsdk.RegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture alpha",
		Method: "sdkfixture.alphaDuplicate",
		Scope:  show.APIKeyScopePlayback,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})
	require.ErrorContains(t, err, "GOLC_SCRIPTSDK_ROUTE_DUPLICATE", "expected a duplicate Route to be rejected")

	err = scriptsdk.RegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Method: "sdkfixture.missingRoute",
		Scope:  show.APIKeyScopePlayback,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})
	require.ErrorContains(t, err, "GOLC_SCRIPTSDK_DESCRIPTOR_INVALID", "expected a descriptor missing Route to be rejected")

	err = scriptsdk.RegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture missingscope",
		Method: "sdkfixture.missingScope",
		Params: fixtureParams{},
		Result: fixtureResult{},
	})
	require.ErrorContains(t, err, "GOLC_SCRIPTSDK_DESCRIPTOR_INVALID", "expected a descriptor missing Scope to be rejected")
}

func testRegisteredSDKMethodsSorted(t *testing.T) {
	scriptsdk.MustRegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture zzzlast",
		Method: "sdkfixture.zzzLast",
		Scope:  show.APIKeyScopePlayback,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})
	scriptsdk.MustRegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture aaafirst",
		Method: "sdkfixture.aaaFirst",
		Scope:  show.APIKeyScopePlayback,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})

	descriptors := scriptsdk.RegisteredSDKMethods()
	routes := make([]string, len(descriptors))
	for i, d := range descriptors {
		routes[i] = d.Route
	}
	for i := 1; i < len(routes); i++ {
		require.LessOrEqual(t, routes[i-1], routes[i], "expected RegisteredSDKMethods to return Route-sorted order, got %v", routes)
	}

	// Mutating the returned snapshot must never affect the package-level
	// registry.
	descriptors[0].Route = "mutated-in-place"
	again := scriptsdk.RegisteredSDKMethods()
	for _, d := range again {
		require.NotEqual(t, "mutated-in-place", d.Route, "expected mutating the returned snapshot to leave the registry unaffected")
	}
}

func testGenerateIntoDeterministic(t *testing.T) {
	scriptsdk.MustRegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:  "sdkfixture deterministic",
		Method: "sdkfixture.deterministic",
		Scope:  show.APIKeyScopeAuthoring,
		Params: fixtureParams{},
		Result: fixtureResult{},
	})

	dirA := t.TempDir()
	dirB := t.TempDir()
	require.NoError(t, scriptsdk.GenerateInto(dirA), "GenerateInto(dirA) failed")
	require.NoError(t, scriptsdk.GenerateInto(dirB), "GenerateInto(dirB) failed")

	for _, relative := range []string{"internal/scriptsdk/generated/golc.d.ts", "internal/scriptsdk/generated/golc-runtime.ts"} {
		a, err := os.ReadFile(filepath.Join(dirA, filepath.FromSlash(relative)))
		require.NoError(t, err, "read dirA %s", relative)
		b, err := os.ReadFile(filepath.Join(dirB, filepath.FromSlash(relative)))
		require.NoError(t, err, "read dirB %s", relative)
		require.Equal(t, a, b, "expected byte-identical generation for %s across repeated runs", relative)
	}
}

func testCheckDriftReporting(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, scriptsdk.GenerateAll(root), "seed GenerateAll failed")

	changed, err := scriptsdk.CheckDrift(root)
	require.NoError(t, err, "CheckDrift failed")
	require.Empty(t, changed, "expected zero drift against freshly seeded committed bytes, got %v", changed)

	typesPath := filepath.Join(root, "internal", "scriptsdk", "generated", "golc.d.ts")
	original, err := os.ReadFile(typesPath)
	require.NoError(t, err, "read seeded golc.d.ts")
	mutated := append(append([]byte{}, original...), '\n', '/', '/', ' ', 'h', 'a', 'n', 'd', '-', 'e', 'd', 'i', 't')
	require.NoError(t, os.WriteFile(typesPath, mutated, 0o644), "mutate committed golc.d.ts")

	changed, err = scriptsdk.CheckDrift(root)
	require.NoError(t, err, "CheckDrift after mutation failed")
	require.Len(t, changed, 1, "expected drift to name exactly golc.d.ts, got %v", changed)
	require.Equal(t, "internal/scriptsdk/generated/golc.d.ts", changed[0])
}

func testCheckDriftReadOnly(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, scriptsdk.GenerateAll(root), "seed GenerateAll failed")

	before := map[string][]byte{}
	beforeModTime := map[string]time.Time{}
	for _, relative := range []string{"internal/scriptsdk/generated/golc.d.ts", "internal/scriptsdk/generated/golc-runtime.ts"} {
		full := filepath.Join(root, filepath.FromSlash(relative))
		data, err := os.ReadFile(full)
		require.NoError(t, err, "read seeded %s", relative)
		before[relative] = data
		info, err := os.Stat(full)
		require.NoError(t, err, "stat seeded %s", relative)
		beforeModTime[relative] = info.ModTime()
	}

	_, err := scriptsdk.CheckDrift(root)
	require.NoError(t, err, "CheckDrift failed")

	for relative, want := range before {
		full := filepath.Join(root, filepath.FromSlash(relative))
		got, err := os.ReadFile(full)
		require.NoError(t, err, "re-read %s after CheckDrift", relative)
		require.Equal(t, want, got, "expected CheckDrift to leave committed bytes at %s untouched", relative)
		info, err := os.Stat(full)
		require.NoError(t, err, "re-stat %s after CheckDrift", relative)
		require.True(t, info.ModTime().Equal(beforeModTime[relative]), "expected CheckDrift to leave %s's mtime untouched, got %v want %v", relative, info.ModTime(), beforeModTime[relative])
	}
}

func testRenderedTypesShape(t *testing.T) {
	scriptsdk.MustRegisterSDKMethod(scriptsdk.SDKMethodDescriptor{
		Route:   "sdkfixture shapecheck",
		Method:  "sdkfixture.shapeCheck",
		Summary: "Fixture-only method proving golc.d.ts's rendered shape.",
		Scope:   show.APIKeyScopePlayback,
		Params:  fixtureParams{},
		Result:  fixtureResult{},
	})

	root := t.TempDir()
	require.NoError(t, scriptsdk.GenerateAll(root), "GenerateAll failed")
	data, err := os.ReadFile(filepath.Join(root, "internal", "scriptsdk", "generated", "golc.d.ts"))
	require.NoError(t, err, "read golc.d.ts")
	text := string(data)

	require.Contains(t, text, "declare namespace golc", "expected golc.d.ts to declare an ambient global golc namespace, got:\n%s", text)
	require.Contains(t, text, "function shapeCheck(params: fixtureParams): Promise<fixtureResult>;", "expected golc.d.ts to declare the fixture method signature, got:\n%s", text)
	require.Contains(t, text, "GENERATED by github.com/lnorton89/golc/internal/scriptsdk. DO NOT EDIT.", "expected golc.d.ts to carry the generated marker, got:\n%s", text)

	withoutComments := stripLineComments(text)
	require.NotContains(t, withoutComments, "import ", "expected golc.d.ts to contain no import keyword, got:\n%s", text)
	require.NotContains(t, withoutComments, "export ", "expected golc.d.ts to contain no export keyword, got:\n%s", text)
}

// stripLineComments removes every "// ..." line comment from text -- the
// generated marker/summary comments legitimately contain the word "export"
// nowhere today, but this keeps the no-import/no-export assertion strictly
// about executable declarations, not prose.
func stripLineComments(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
