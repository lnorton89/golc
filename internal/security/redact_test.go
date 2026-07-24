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
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func TestScopeSecrets(t *testing.T) {
	t.Run("ScanCanary finds the planted token and reports it exactly", func(t *testing.T) {
		clean := []byte("nothing sensitive here\n")
		if token := security.ScanCanary(clean); token != "" {
			t.Fatalf("expected clean input to scan clean, got token %q", token)
		}
		planted := []byte("prefix " + security.CanaryToken + " suffix\n")
		if token := security.ScanCanary(planted); token != security.CanaryToken {
			t.Fatalf("ScanCanary token = %q, want %q", token, security.CanaryToken)
		}
	})

	t.Run("ScanCanary rejects common secret-shaped patterns beyond the exact canary token", func(t *testing.T) {
		cases := []string{
			"Authorization: Bearer abc123\n",
			"LINEAR_API_KEY=lin_api_deadbeef\n",
			"key=sk-fake12345\n",
		}
		for _, sample := range cases {
			if token := security.ScanCanary([]byte(sample)); token == "" {
				t.Fatalf("expected a secret-shaped match in %q", sample)
			}
		}
	})

	t.Run("ScanCanaryAll attributes violations to their exact source and stays clean when every source is safe", func(t *testing.T) {
		violations := security.ScanCanaryAll(map[string][]byte{
			"stdout": []byte("build succeeded\n"),
			"stderr": []byte(security.CanaryToken),
		})
		if len(violations) != 1 || violations[0].Source != "stderr" || violations[0].Token != security.CanaryToken {
			t.Fatalf("ScanCanaryAll violations = %+v, want exactly one stderr violation", violations)
		}

		clean := security.ScanCanaryAll(map[string][]byte{
			"stdout": []byte("ok\n"),
			"schema": []byte(`{"type":"object"}`),
		})
		if len(clean) != 0 {
			t.Fatalf("expected zero violations for clean sources, got %+v", clean)
		}
	})

	t.Run("SetState renders only set/unset, never the underlying value", func(t *testing.T) {
		if got := security.SetState(""); got != "<unset>" {
			t.Fatalf("SetState(\"\") = %q, want <unset>", got)
		}
		if got := security.SetState("   "); got != "<unset>" {
			t.Fatalf("SetState(whitespace) = %q, want <unset>", got)
		}
		secretValue := "lin_api_" + security.CanaryToken
		got := security.SetState(secretValue)
		if got != "<set>" {
			t.Fatalf("SetState(non-empty) = %q, want <set>", got)
		}
		if strings.Contains(got, secretValue) {
			t.Fatal("SetState must never echo the underlying value")
		}
	})

	t.Run("Redact passes clean values through and replaces anything canary/pattern-matched", func(t *testing.T) {
		if got := security.Redact("project"); got != "project" {
			t.Fatalf("Redact(clean) = %q, want unchanged", got)
		}
		leaked := "Bearer " + security.CanaryToken
		got := security.Redact(leaked)
		if got == leaked || strings.Contains(got, security.CanaryToken) {
			t.Fatalf("Redact must never return the original leaked bytes, got %q", got)
		}
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
		if !strings.HasPrefix(rendered, "GOLC_TEST_DIAGNOSTIC: example failure (") {
			t.Fatalf("unexpected SafeDiagnostic.String prefix: %q", rendered)
		}
		if strings.Contains(rendered, security.CanaryToken) {
			t.Fatalf("SafeDiagnostic.String leaked the canary token: %q", rendered)
		}
		if strings.Index(rendered, "alpha=") > strings.Index(rendered, "zulu=") {
			t.Fatalf("expected fields sorted by name, got %q", rendered)
		}

		bare := security.SafeDiagnostic{Code: "GOLC_TEST_BARE", Message: "no fields"}
		if bare.String() != "GOLC_TEST_BARE: no fields" {
			t.Fatalf("SafeDiagnostic with no fields = %q, want no parenthesized suffix", bare.String())
		}
	})

	t.Run("no fake-secret bytes in real captured command stdout/stderr", func(t *testing.T) {
		root := repositoryRoot(t)
		registry, err := command.NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}
		result := registry.Execute(command.Request{Root: root, Args: []string{"check", "--concern", "project"}})
		if result.ExitCode != 0 {
			t.Fatalf("check --concern project exited %d: %s", result.ExitCode, result.Stderr)
		}
		violations := security.ScanCanaryAll(map[string][]byte{
			"stdout:check --concern project": result.Stdout,
			"stderr:check --concern project": result.Stderr,
		})
		if len(violations) != 0 {
			t.Fatalf("real command output leaked fake-secret bytes: %+v", violations)
		}
	})

	t.Run("no fake-secret bytes in any committed generated schema or the committed Linear map", func(t *testing.T) {
		root := repositoryRoot(t)
		sources := map[string][]byte{}
		for _, descriptor := range contracts.RegisteredSchemas() {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
			if err != nil {
				t.Fatalf("read %s: %v", descriptor.OutputPath, err)
			}
			sources["schema:"+descriptor.OutputPath] = data
		}
		mapData, err := os.ReadFile(filepath.Join(root, ".planning", "linear-map.json"))
		if err != nil {
			t.Fatalf("read committed linear map: %v", err)
		}
		sources["map:.planning/linear-map.json"] = mapData

		if violations := security.ScanCanaryAll(sources); len(violations) != 0 {
			t.Fatalf("committed generated artifacts leaked fake-secret bytes: %+v", violations)
		}
	})

	t.Run("no fake-secret bytes in a synthesized Linear apply report", func(t *testing.T) {
		report := apply.Report{
			PlanID: "plan:test",
			Results: []apply.OperationResult{
				{LocalID: "req:TEST-01", Status: apply.StatusNoop},
			},
		}
		encoded, err := strictjson.CanonicalEncode(report)
		if err != nil {
			t.Fatalf("CanonicalEncode report: %v", err)
		}
		if token := security.ScanCanary(encoded); token != "" {
			t.Fatalf("synthesized report leaked token %q", token)
		}

		leaked := apply.Report{
			PlanID: "plan:test",
			Results: []apply.OperationResult{
				{LocalID: "req:TEST-01", Status: apply.StatusPending, Reason: security.CanaryToken},
			},
		}
		leakedEncoded, err := strictjson.CanonicalEncode(leaked)
		if err != nil {
			t.Fatalf("CanonicalEncode leaked report: %v", err)
		}
		if token := security.ScanCanary(leakedEncoded); token != security.CanaryToken {
			t.Fatalf("expected ScanCanary to catch a planted token inside an encoded report, got %q", token)
		}
	})

	t.Run("no fake-secret bytes in a synthesized foundation manifest or ZIP", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)
		bundle, err := delivery.BuildFoundationBundle(root)
		if err != nil {
			t.Fatalf("BuildFoundationBundle: %v", err)
		}
		violations := security.ScanCanaryAll(map[string][]byte{
			"manifest": bundle.ManifestBytes,
			"zip":      bundle.ZIPBytes,
		})
		if len(violations) != 0 {
			t.Fatalf("synthesized foundation bundle leaked fake-secret bytes: %+v", violations)
		}
	})
}

// writeFoundationFixture mirrors internal/delivery/delivery_test.go's
// fixture of the same name: a minimal, self-contained repository tree
// BuildFoundationBundle can operate on, independent of the real
// repository's current file set.
func writeFoundationFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"golc.ps1":                            "REM golc.ps1 fixture entrypoint\n",
		"golc.project.toml":                   "schema_version = 2\n",
		"docs/development.md":                 "# Fixture Docs\n",
		"config/commands.toml":                "schema_version = 2\n\n[commands]\nentrypoint = \"golc.ps1\"\ncli_binary = \".tools/installs/golc_project\"\ngo_version = \"1.26.5\"\n",
		"config/toolchain.toml":               "schema_version = 2\n",
		"config/integrations/linear.toml":     "schema_version = 2\n",
		"schemas/golc-project.schema.json":    "{}\n",
		"schemas/config-commands.schema.json": "{}\n",
		filepath.ToSlash(bootstrap.PlatformExecutablePath(".tools/installs/golc_project", "golc-project")): "fixture binary payload\n",
	}
	for relative, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", relative, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
}
