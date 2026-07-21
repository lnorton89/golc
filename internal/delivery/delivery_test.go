// delivery_test.go covers Plan 01-06's offline core delivery graph
// contract: LoadGraph consumes exactly config/commands.toml, the fixed
// step declaration is duplicate-safe (ValidateParity), Run/RunOffline
// execute steps in order and stop at the first failure, RunOffline
// installs the offline environment and deny transport and always restores
// the prior state, and RunOffline refuses to execute a graph containing a
// network-allowed step.
//
// This file is the external package delivery_test (not internal package
// delivery) because internal/command's check.go imports internal/delivery
// to orchestrate this graph. Declaring the "delivery" quick-test scope
// from an internal delivery_test.go would import internal/command from
// package delivery, closing delivery[test] -> command -> delivery — an
// import cycle. An external test package avoids it: delivery_test imports
// both delivery and command, while the production delivery package itself
// still never imports command (01-VALIDATION: every owning Go test task
// registers its exact scope through MustDeclareScope beside its
// TestScope marker; this is the router_test.go/bootstrap_test.go pattern
// adapted for the one case where the internal-package form would cycle).
package delivery_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/delivery"
)

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "delivery",
	Summary: "Offline core delivery graph (generate/check/build/test) tests.",
})

// writeFixtureCommandsToml writes a minimal, valid config/commands.toml
// under root so LoadGraph can be exercised without the full repository
// checkout.
func writeFixtureCommandsToml(t *testing.T, root string) {
	t.Helper()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	body := "schema_version = 1\n\n[commands]\n" +
		"entrypoint = \"golc.ps1\"\n" +
		"cli_binary = \".tools/installs/golc_project/bin/golc-project.exe\"\n" +
		"go_version = \"1.26.5\"\n"
	if err := os.WriteFile(filepath.Join(configDir, "commands.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config/commands.toml: %v", err)
	}
}

func TestScopeDelivery(t *testing.T) {
	t.Run("LoadGraph reads exactly the three canonical commands keys and the fixed core steps", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureCommandsToml(t, root)

		graph, err := delivery.LoadGraph(root)
		if err != nil {
			t.Fatalf("LoadGraph: %v", err)
		}
		if graph.Inventory.Entrypoint != "golc.ps1" {
			t.Fatalf("Entrypoint = %q, want golc.ps1", graph.Inventory.Entrypoint)
		}
		if graph.Inventory.CLIBinary != ".tools/installs/golc_project/bin/golc-project.exe" {
			t.Fatalf("CLIBinary = %q", graph.Inventory.CLIBinary)
		}
		if graph.Inventory.GoVersion != "1.26.5" {
			t.Fatalf("GoVersion = %q", graph.Inventory.GoVersion)
		}
		wantNames := []string{"generate", "check", "build", "test"}
		if len(graph.Steps) != len(wantNames) {
			t.Fatalf("len(Steps) = %d, want %d", len(graph.Steps), len(wantNames))
		}
		for i, name := range wantNames {
			if graph.Steps[i].Name != name {
				t.Fatalf("Steps[%d].Name = %q, want %q", i, graph.Steps[i].Name, name)
			}
			if graph.Steps[i].Network != delivery.NetworkDenied {
				t.Fatalf("Steps[%d].Network = %v, want NetworkDenied", i, graph.Steps[i].Network)
			}
		}
		// check invokes "--concern project", never "--offline" — a
		// check-driven graph run must never recurse into itself.
		checkStep := graph.Steps[1]
		if strings.Join(checkStep.Args, " ") != "--concern project" {
			t.Fatalf("check step Args = %v, want [--concern project]", checkStep.Args)
		}
	})

	t.Run("LoadGraph fails closed on a missing config/commands.toml", func(t *testing.T) {
		root := t.TempDir()
		if _, err := delivery.LoadGraph(root); err == nil {
			t.Fatal("expected LoadGraph to fail for a missing config/commands.toml")
		}
	})

	t.Run("LoadGraph fails closed on an incomplete commands inventory", func(t *testing.T) {
		root := t.TempDir()
		configDir := filepath.Join(root, "config")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("mkdir config: %v", err)
		}
		body := "schema_version = 1\n\n[commands]\nentrypoint = \"golc.ps1\"\n"
		if err := os.WriteFile(filepath.Join(configDir, "commands.toml"), []byte(body), 0o644); err != nil {
			t.Fatalf("write config/commands.toml: %v", err)
		}
		_, err := delivery.LoadGraph(root)
		if err == nil {
			t.Fatal("expected LoadGraph to fail for an incomplete commands inventory")
		}
		if !strings.Contains(err.Error(), "GOLC_DELIVERY_INVENTORY_INCOMPLETE") {
			t.Fatalf("error = %v, want GOLC_DELIVERY_INVENTORY_INCOMPLETE", err)
		}
	})

	t.Run("ValidateParity accepts the production graph and rejects duplicates", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureCommandsToml(t, root)
		graph, err := delivery.LoadGraph(root)
		if err != nil {
			t.Fatalf("LoadGraph: %v", err)
		}
		if err := delivery.ValidateParity(graph); err != nil {
			t.Fatalf("ValidateParity on the production graph: %v", err)
		}

		duplicateNames := graph
		duplicateNames.Steps = append(append([]delivery.Step{}, graph.Steps...), graph.Steps[0])
		if err := delivery.ValidateParity(duplicateNames); err == nil {
			t.Fatal("expected ValidateParity to reject a duplicate step name")
		}

		empty := graph
		empty.Steps = nil
		if err := delivery.ValidateParity(empty); err == nil {
			t.Fatal("expected ValidateParity to reject a graph with zero steps")
		}

		blankRoute := graph
		blankRoute.Steps = []delivery.Step{{Name: "x", Route: ""}}
		if err := delivery.ValidateParity(blankRoute); err == nil {
			t.Fatal("expected ValidateParity to reject a step with a blank route")
		}
	})

	t.Run("Run executes every step in order and stops at the first failure", func(t *testing.T) {
		graph := delivery.Graph{
			Root: t.TempDir(),
			Inventory: delivery.CommandInventory{
				Entrypoint: "golc.ps1", CLIBinary: ".tools/x", GoVersion: "1.26.5",
			},
			Steps: []delivery.Step{
				{Name: "one", Route: "one", Network: delivery.NetworkDenied},
				{Name: "two", Route: "two", Network: delivery.NetworkDenied},
				{Name: "three", Route: "three", Network: delivery.NetworkDenied},
			},
		}

		var invoked []string
		executor := func(route string, args []string) (int, []byte, []byte) {
			invoked = append(invoked, route)
			if route == "two" {
				return 1, nil, []byte("boom")
			}
			return 0, nil, nil
		}

		results, err := delivery.Run(graph, executor)
		if err == nil {
			t.Fatal("expected Run to fail when step two exits non-zero")
		}
		if got := strings.Join(invoked, ","); got != "one,two" {
			t.Fatalf("invoked routes = %q, want \"one,two\" (three must never run)", got)
		}
		if len(results) != 2 {
			t.Fatalf("len(results) = %d, want 2", len(results))
		}
		if results[1].ExitCode != 1 {
			t.Fatalf("results[1].ExitCode = %d, want 1", results[1].ExitCode)
		}
	})

	t.Run("RunOffline refuses to execute a graph containing a network-allowed step", func(t *testing.T) {
		graph := delivery.Graph{
			Root: t.TempDir(),
			Steps: []delivery.Step{
				{Name: "one", Route: "one", Network: delivery.NetworkDenied},
				{Name: "two", Route: "two", Network: delivery.NetworkAllowed},
			},
		}
		executed := false
		executor := func(route string, args []string) (int, []byte, []byte) {
			executed = true
			return 0, nil, nil
		}
		if _, err := delivery.RunOffline(graph, executor); err == nil {
			t.Fatal("expected RunOffline to refuse a graph containing a NetworkAllowed step")
		}
		if executed {
			t.Fatal("RunOffline must never invoke the executor when it refuses the graph")
		}
	})

	t.Run("RunOffline installs the offline environment and deny transport, then restores prior state", func(t *testing.T) {
		root := t.TempDir()

		previousGOPROXY, hadGOPROXY := os.LookupEnv("GOPROXY")
		os.Setenv("GOPROXY", "https://proxy.example.invalid")
		t.Cleanup(func() {
			if hadGOPROXY {
				os.Setenv("GOPROXY", previousGOPROXY)
			} else {
				os.Unsetenv("GOPROXY")
			}
		})

		previousTransport := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = previousTransport })

		graph := delivery.Graph{
			Root:  root,
			Steps: []delivery.Step{{Name: "probe", Route: "probe", Network: delivery.NetworkDenied}},
		}

		var observedGOPROXY string
		var observedTransportIsDeny bool
		executor := func(route string, args []string) (int, []byte, []byte) {
			observedGOPROXY = os.Getenv("GOPROXY")
			_, observedTransportIsDeny = http.DefaultTransport.(delivery.DenyTransport)
			return 0, nil, nil
		}

		if _, err := delivery.RunOffline(graph, executor); err != nil {
			t.Fatalf("RunOffline: %v", err)
		}
		if observedGOPROXY != "off" {
			t.Fatalf("observed GOPROXY during offline run = %q, want off", observedGOPROXY)
		}
		if !observedTransportIsDeny {
			t.Fatal("expected http.DefaultTransport to be DenyTransport during the offline run")
		}
		if os.Getenv("GOPROXY") != "https://proxy.example.invalid" {
			t.Fatalf("GOPROXY was not restored after RunOffline: %q", os.Getenv("GOPROXY"))
		}
		if http.DefaultTransport != previousTransport {
			t.Fatal("http.DefaultTransport was not restored after RunOffline")
		}
	})

	t.Run("DenyTransport fails every request with a named diagnostic before any dial", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid/resource", nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		_, err = (delivery.DenyTransport{}).RoundTrip(request)
		if err == nil {
			t.Fatal("expected DenyTransport.RoundTrip to fail")
		}
		if !strings.Contains(err.Error(), "GOLC_DELIVERY_NETWORK_DENIED") {
			t.Fatalf("error = %v, want GOLC_DELIVERY_NETWORK_DENIED", err)
		}
	})

	t.Run("NetworkPolicy renders stable diagnostics", func(t *testing.T) {
		if delivery.NetworkDenied.String() != "denied" {
			t.Fatalf("NetworkDenied.String() = %q, want denied", delivery.NetworkDenied.String())
		}
		if delivery.NetworkAllowed.String() != "allowed" {
			t.Fatalf("NetworkAllowed.String() = %q, want allowed", delivery.NetworkAllowed.String())
		}
	})
}
