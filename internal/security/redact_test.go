// redact_test.go proves Task 1's secret-isolation contract end to end
// (CONTEXT D-19/D-20, T-01-18): SafeDiagnostic/Redact/SetState only ever
// expose allowlisted fields, and ScanCanary/ScanCanaryAll actually detect a
// planted fake-secret token across every output-surface family the root
// graph produces -- captured real command stdout/stderr, committed
// generated schemas, the committed Linear map, a synthesized Linear apply
// report, and a synthesized foundation manifest/ZIP built through the same
// internal/delivery primitives package --foundation uses.
//
// This file is the external package security_test (not internal package
// security) because internal/command/check.go imports internal/security to
// run its own canary scan (Task 1's check.go integration). Declaring the
// "secrets" quick-test scope from an internal redact_test.go would import
// internal/command from package security, closing
// security[test] -> command -> security -- the same import-cycle shape
// internal/delivery/delivery_test.go's package doc already documents and
// avoids the identical way (01-VALIDATION: every owning Go test task
// registers its exact scope through MustDeclareScope beside its
// TestScope{PascalName} marker).
package security_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/contracts"
	"github.com/lnorton89/golc/internal/delivery"
	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/apply"
)

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "secrets",
	Summary: "Centralized allowlisted diagnostics and cross-artifact fake-secret canary scans.",
})

// repositoryRoot locates the repository root by walking up from the
// current working directory (go test's working directory is always the
// package directory, internal/security -> internal -> repository root),
// mirroring internal/delivery/delivery_test.go's goldenFoundationManifestPath
// helper.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err, "os.Getwd")
	return filepath.Dir(filepath.Dir(wd))
}

func TestScopeSecrets(t *testing.T) {
	t.Run("ScanCanary finds the planted token and reports it exactly", func(t *testing.T) {
		clean := []byte("nothing sensitive here\n")
		require.Equal(t, "", security.ScanCanary(clean), "expected clean input to scan clean")
		planted := []byte("prefix " + security.CanaryToken + " suffix\n")
		require.Equal(t, security.CanaryToken, security.ScanCanary(planted), "ScanCanary token")
	})

	t.Run("ScanCanary rejects common secret-shaped patterns beyond the exact canary token", func(t *testing.T) {
		cases := []string{
			"Authorization: Bearer abc123\n",
			"LINEAR_API_KEY=lin_api_deadbeef\n",
			"key=sk-fake12345\n",
		}
		for _, sample := range cases {
			require.NotEqual(t, "", security.ScanCanary([]byte(sample)), "expected a secret-shaped match in %q", sample)
		}
	})

	t.Run("ScanCanaryAll attributes violations to their exact source and stays clean when every source is safe", func(t *testing.T) {
		violations := security.ScanCanaryAll(map[string][]byte{
			"stdout": []byte("build succeeded\n"),
			"stderr": []byte(security.CanaryToken),
		})
		require.Len(t, violations, 1, "ScanCanaryAll violations = %+v, want exactly one stderr violation", violations)
		require.Equal(t, "stderr", violations[0].Source, "ScanCanaryAll violations = %+v, want exactly one stderr violation", violations)
		require.Equal(t, security.CanaryToken, violations[0].Token, "ScanCanaryAll violations = %+v, want exactly one stderr violation", violations)

		clean := security.ScanCanaryAll(map[string][]byte{
			"stdout": []byte("ok\n"),
			"schema": []byte(`{"type":"object"}`),
		})
		require.Len(t, clean, 0, "expected zero violations for clean sources, got %+v", clean)
	})

	t.Run("SetState renders only set/unset, never the underlying value", func(t *testing.T) {
		require.Equal(t, "<unset>", security.SetState(""), "SetState(\"\")")
		require.Equal(t, "<unset>", security.SetState("   "), "SetState(whitespace)")
		secretValue := "lin_api_" + security.CanaryToken
		got := security.SetState(secretValue)
		require.Equal(t, "<set>", got, "SetState(non-empty)")
		require.NotContains(t, got, secretValue, "SetState must never echo the underlying value")
	})

	t.Run("Redact passes clean values through and replaces anything canary/pattern-matched", func(t *testing.T) {
		require.Equal(t, "project", security.Redact("project"), "Redact(clean)")
		leaked := "Bearer " + security.CanaryToken
		got := security.Redact(leaked)
		require.NotEqual(t, leaked, got, "Redact must never return the original leaked bytes, got %q", got)
		require.NotContains(t, got, security.CanaryToken, "Redact must never return the original leaked bytes, got %q", got)
	})

	t.Run("SafeDiagnostic.String never carries a raw environment/header/config/exception object", func(t *testing.T) {
		diagnostic := security.SafeDiagnostic{
			Code:    "GOLC_TEST_DIAGNOSTIC",
			Message: "example failure",
			Fields: map[string]string{
				"zulu":  "safe-value",
				"alpha": "Bearer " + security.CanaryToken,
			},
		}
		rendered := diagnostic.String()
		require.True(t, strings.HasPrefix(rendered, "GOLC_TEST_DIAGNOSTIC: example failure ("), "unexpected SafeDiagnostic.String prefix: %q", rendered)
		require.NotContains(t, rendered, security.CanaryToken, "SafeDiagnostic.String leaked the canary token: %q", rendered)
		require.False(t, strings.Index(rendered, "alpha=") > strings.Index(rendered, "zulu="), "expected fields sorted by name, got %q", rendered)

		bare := security.SafeDiagnostic{Code: "GOLC_TEST_BARE", Message: "no fields"}
		require.Equal(t, "GOLC_TEST_BARE: no fields", bare.String(), "SafeDiagnostic with no fields, want no parenthesized suffix")
	})

	t.Run("no fake-secret bytes in real captured command stdout/stderr", func(t *testing.T) {
		root := repositoryRoot(t)
		registry, err := command.NewDefaultCommandRegistry()
		require.NoError(t, err, "NewDefaultCommandRegistry")
		result := registry.Execute(command.Request{Root: root, Args: []string{"check", "--concern", "project"}})
		require.Equal(t, 0, result.ExitCode, "check --concern project exited %d: %s", result.ExitCode, result.Stderr)
		violations := security.ScanCanaryAll(map[string][]byte{
			"stdout:check --concern project": result.Stdout,
			"stderr:check --concern project": result.Stderr,
		})
		require.Len(t, violations, 0, "real command output leaked fake-secret bytes: %+v", violations)
	})

	t.Run("no fake-secret bytes in any committed generated schema or the committed Linear map", func(t *testing.T) {
		root := repositoryRoot(t)
		sources := map[string][]byte{}
		for _, descriptor := range contracts.RegisteredSchemas() {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
			require.NoError(t, err, "read %s", descriptor.OutputPath)
			sources["schema:"+descriptor.OutputPath] = data
		}
		mapData, err := os.ReadFile(filepath.Join(root, ".planning", "linear-map.json"))
		require.NoError(t, err, "read committed linear map")
		sources["map:.planning/linear-map.json"] = mapData

		violations := security.ScanCanaryAll(sources)
		require.Len(t, violations, 0, "committed generated artifacts leaked fake-secret bytes: %+v", violations)
	})

	t.Run("no fake-secret bytes in a synthesized Linear apply report", func(t *testing.T) {
		report := apply.Report{
			PlanID: "plan:test",
			Results: []apply.OperationResult{
				{LocalID: "req:TEST-01", Status: apply.StatusNoop},
			},
		}
		encoded, err := strictjson.CanonicalEncode(report)
		require.NoError(t, err, "CanonicalEncode report")
		require.Equal(t, "", security.ScanCanary(encoded), "synthesized report leaked token")

		leaked := apply.Report{
			PlanID: "plan:test",
			Results: []apply.OperationResult{
				{LocalID: "req:TEST-01", Status: apply.StatusPending, Reason: security.CanaryToken},
			},
		}
		leakedEncoded, err := strictjson.CanonicalEncode(leaked)
		require.NoError(t, err, "CanonicalEncode leaked report")
		require.Equal(t, security.CanaryToken, security.ScanCanary(leakedEncoded), "expected ScanCanary to catch a planted token inside an encoded report")
	})

	t.Run("no fake-secret bytes in a synthesized foundation manifest or ZIP", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)
		bundle, err := delivery.BuildFoundationBundle(root)
		require.NoError(t, err, "BuildFoundationBundle")
		violations := security.ScanCanaryAll(map[string][]byte{
			"manifest": bundle.ManifestBytes,
			"zip":      bundle.ZIPBytes,
		})
		require.Len(t, violations, 0, "synthesized foundation bundle leaked fake-secret bytes: %+v", violations)
	})
}

// writeFoundationFixture mirrors internal/delivery/delivery_test.go's
// fixture of the same name: a minimal, self-contained repository tree
// BuildFoundationBundle can operate on, independent of the real
// repository's current file set.
func writeFoundationFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"golc.project.toml":                   "schema_version = 2\n",
		"docs/development.md":                 "# Fixture Docs\n",
		"config/commands.toml":                "schema_version = 2\n\n[commands]\ncli_binary = \".tools/installs/golc_project\"\ngo_version = \"1.26.5\"\n",
		"config/toolchain.toml":               "schema_version = 2\n",
		"config/integrations/linear.toml":     "schema_version = 2\n",
		"schemas/golc-project.schema.json":    "{}\n",
		"schemas/config-commands.schema.json": "{}\n",
		filepath.ToSlash(bootstrap.PlatformExecutablePath(".tools/installs/golc_project", "golc-project")): "fixture binary payload\n",
	}
	for relative, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755), "mkdir for %s", relative)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644), "write %s", relative)
	}
}
