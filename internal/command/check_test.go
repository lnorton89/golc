package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/delivery"
)

func TestScopeCommandParity(t *testing.T) {
	MustDeclareScope(ScopeRegistration{Scope: "command-parity", Summary: "Mage workflow parity checks."})

	graph := commandParityGraph()
	workflow := commandParityWorkflow(
		"Bootstrap",
		"GenerateCheck",
		"CheckOffline",
		"Build",
		"Test",
		"PackageFoundation",
	)

	t.Run("accepts LF and CRLF workflows through shared target descriptors", func(t *testing.T) {
		for _, test := range []struct {
			name string
			data []byte
		}{
			{name: "LF", data: workflow},
			{name: "CRLF", data: []byte(strings.ReplaceAll(string(workflow), "\n", "\r\n"))},
			{name: "case-insensitive Mage spelling", data: commandParityWorkflow(
				"bOoTsTrAp",
				"generatecheck",
				"CHECKOFFLINE",
				"build",
				"TEST",
				"packagefoundation",
			)},
		} {
			t.Run(test.name, func(t *testing.T) {
				if err := validateCommandParity(graph, test.data); err != nil {
					t.Fatalf("validateCommandParity: %v", err)
				}
			})
		}
	})

	t.Run("reports the first sequence divergence", func(t *testing.T) {
		tests := []struct {
			name   string
			data   []byte
			prefix string
		}{
			{
				name:   "missing",
				data:   commandParityWorkflow("Bootstrap", "GenerateCheck", "CheckOffline", "Build", "Test"),
				prefix: "GOLC_CHECK_PARITY_STEP_COUNT:",
			},
			{
				name: "extra",
				data: commandParityWorkflow(
					"Bootstrap", "GenerateCheck", "CheckOffline", "Build", "Test", "PackageFoundation", "Build"),
				prefix: "GOLC_CHECK_PARITY_STEP_COUNT:",
			},
			{
				name: "reordered",
				data: commandParityWorkflow(
					"Bootstrap", "CheckOffline", "GenerateCheck", "Build", "Test", "PackageFoundation"),
				prefix: "GOLC_CHECK_PARITY_STEP_MISMATCH: step 2:",
			},
			{
				name: "duplicate",
				data: commandParityWorkflow(
					"Bootstrap", "GenerateCheck", "GenerateCheck", "Build", "Test", "PackageFoundation"),
				prefix: "GOLC_CHECK_PARITY_STEP_MISMATCH: step 3:",
			},
			{
				name: "unknown retains spelling and position",
				data: commandParityWorkflow(
					"Bootstrap", "GenerateCheck", "NotARealTarget", "Build", "Test", "PackageFoundation"),
				prefix: `GOLC_CHECK_PARITY_TARGET_UNKNOWN: step 3: Mage target "NotARealTarget"`,
			},
			{
				name: "recursive PR target",
				data: commandParityWorkflow(
					"Bootstrap", "GenerateCheck", "Pr", "Build", "Test", "PackageFoundation"),
				prefix: `GOLC_CHECK_PARITY_TARGET_KIND: step 3: Mage target "Pr"`,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				assertCommandParityErrorPrefix(t, graph, test.data, test.prefix)
			})
		}
	})

	t.Run("compares descriptor routes and arguments to the supplied graph", func(t *testing.T) {
		routeMismatch := commandParityGraph()
		routeMismatch.Steps[3].Route = "compile"
		assertCommandParityErrorPrefix(
			t, routeMismatch, workflow, "GOLC_CHECK_PARITY_STEP_MISMATCH: step 4:")

		argumentMismatch := commandParityGraph()
		argumentMismatch.Steps[1].Args = []string{"--write"}
		assertCommandParityErrorPrefix(
			t, argumentMismatch, workflow, "GOLC_CHECK_PARITY_STEP_MISMATCH: step 2:")
	})

	t.Run("rejects every undeclared executable shape", func(t *testing.T) {
		lines := []string{
			`run: powershell -NoProfile -File .\golc.ps1 build`,
			"run: curl https://example.invalid/tool",
			"run: npm install",
			"run: golc-project linear apply plan.json",
			"run: mage Build && curl https://example.invalid",
			"run: mage Build --verbose",
			"run: mage",
			"run: |",
		}
		for _, line := range lines {
			t.Run(line, func(t *testing.T) {
				data := append(append([]byte(nil), workflow...), []byte("\n      - name: escape\n        "+line+"\n")...)
				assertCommandParityErrorPrefix(
					t, graph, data, "GOLC_CHECK_PARITY_RUN_INVALID:")
			})
		}
	})

	t.Run("preserves secret and mutation scans", func(t *testing.T) {
		for _, token := range prForbiddenTokens {
			t.Run(token, func(t *testing.T) {
				data := append(append([]byte(nil), workflow...), []byte("# "+token+"\n")...)
				assertCommandParityErrorPrefix(
					t, graph, data, "GOLC_CHECK_PARITY_SECRET_OR_MUTATION:")
			})
		}
	})

	t.Run("preserves trigger scans and requires pull_request", func(t *testing.T) {
		for _, trigger := range prForbiddenTriggers {
			t.Run(trigger, func(t *testing.T) {
				data := append(append([]byte(nil), workflow...), []byte("# "+trigger+"\n")...)
				assertCommandParityErrorPrefix(
					t, graph, data, "GOLC_CHECK_PARITY_TRIGGER_FORBIDDEN:")
			})
		}

		data := []byte(strings.Replace(string(workflow), "pull_request", "issues", 1))
		assertCommandParityErrorPrefix(
			t, graph, data, "GOLC_CHECK_PARITY_TRIGGER_MISSING:")
	})

	t.Run("runCheckCommandParity propagates loader and workflow read failures", func(t *testing.T) {
		missingConfig := runCheckCommandParity(t.TempDir())
		if missingConfig.ExitCode != 1 ||
			!strings.HasPrefix(string(missingConfig.Stderr), "GOLC_CHECK_PARITY_CONFIG:") {
			t.Fatalf("missing config result = %+v, want GOLC_CHECK_PARITY_CONFIG", missingConfig)
		}

		root := t.TempDir()
		configDir := filepath.Join(root, "config")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("mkdir config: %v", err)
		}
		configBytes, err := os.ReadFile(filepath.Join(commandParityRepositoryRoot(t), "config", "commands.toml"))
		if err != nil {
			t.Fatalf("read production commands config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "commands.toml"), configBytes, 0o644); err != nil {
			t.Fatalf("write commands config: %v", err)
		}
		missingWorkflow := runCheckCommandParity(root)
		if missingWorkflow.ExitCode != 1 ||
			!strings.HasPrefix(string(missingWorkflow.Stderr), "GOLC_CHECK_PARITY_WORKFLOW_MISSING:") {
			t.Fatalf("missing workflow result = %+v, want GOLC_CHECK_PARITY_WORKFLOW_MISSING", missingWorkflow)
		}
	})
}

func commandParityGraph() delivery.Graph {
	return delivery.Graph{
		Steps: []delivery.Step{
			{Name: "01-bootstrap", Route: "bootstrap", Network: delivery.NetworkAllowed},
			{Name: "02-generate---check", Route: "generate", Args: []string{"--check"}},
			{Name: "03-check---offline", Route: "check", Args: []string{"--offline"}},
			{Name: "04-build", Route: "build"},
			{Name: "05-test", Route: "test"},
			{Name: "06-package---foundation", Route: "package", Args: []string{"--foundation"}},
		},
	}
}

func commandParityWorkflow(targets ...string) []byte {
	var workflow strings.Builder
	workflow.WriteString("name: check\n\non:\n  pull_request:\n\npermissions:\n  contents: read\n\njobs:\n  check:\n    runs-on: windows-latest\n    steps:\n")
	for _, target := range targets {
		workflow.WriteString("      - name: parity\n        run: mage ")
		workflow.WriteString(target)
		workflow.WriteByte('\n')
	}
	return []byte(workflow.String())
}

func assertCommandParityErrorPrefix(
	t *testing.T,
	graph delivery.Graph,
	workflow []byte,
	prefix string,
) {
	t.Helper()
	err := validateCommandParity(graph, workflow)
	if err == nil {
		t.Fatalf("validateCommandParity unexpectedly succeeded; want prefix %q", prefix)
	}
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error = %q, want prefix %q", err, prefix)
	}
}

func commandParityRepositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
}
