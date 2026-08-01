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
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/bootstrap"
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
	require.NoError(t, os.MkdirAll(configDir, 0o755), "mkdir config")
	body := "schema_version = 1\n\n[commands]\n" +
		"cli_binary = \".tools/installs/golc_project\"\n" +
		"go_version = \"1.26.5\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "commands.toml"), []byte(body), 0o644), "write config/commands.toml")
}

func writeFixturePRCommandsToml(t *testing.T, root, steps, network, mutation string) {
	t.Helper()
	configDir := filepath.Join(root, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755), "mkdir config")
	body := "schema_version = 2\n\n[commands]\n" +
		"cli_binary = \".tools/installs/golc_project\"\n" +
		"go_version = \"1.26.5\"\n\n" +
		"[commands.pr]\n" +
		"steps = \"" + steps + "\"\n" +
		"network_steps = \"" + network + "\"\n" +
		"mutation_steps = \"" + mutation + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "commands.toml"), []byte(body), 0o644), "write config/commands.toml")
}

func TestScopeDelivery(t *testing.T) {
	t.Run("MageTargets is the defensive deterministic target authority", func(t *testing.T) {
		want := []delivery.MageTarget{
			{
				Name: "bootstrap", Kind: delivery.MageTargetKindBootstrap, Authority: "internal/bootstrap.Bootstrap",
				EnvironmentOptions: []delivery.MageEnvironmentOption{{
					Name: delivery.BootstrapEnvironmentName, EnablingValue: delivery.BootstrapEnvironmentEnablingValue,
					Effect: "bootstrap.Options.IncludeLinearSync",
				}},
			},
			{Name: "build", Kind: delivery.MageTargetKindRoute, Route: "build", Authority: "internal/command registry"},
			{Name: "check", Kind: delivery.MageTargetKindRoute, Route: "check", Args: []string{"--concern", "project"}, Authority: "internal/command registry"},
			{Name: "checkoffline", Kind: delivery.MageTargetKindRoute, Route: "check", Args: []string{"--offline"}, Authority: "internal/command registry"},
			{Name: "dev", Kind: delivery.MageTargetKindRoute, Route: "dev", Authority: "internal/command registry"},
			{Name: "generate", Kind: delivery.MageTargetKindRoute, Route: "generate", Authority: "internal/command registry"},
			{Name: "generatecheck", Kind: delivery.MageTargetKindRoute, Route: "generate", Args: []string{"--check"}, Authority: "internal/command registry"},
			{Name: "lint", Kind: delivery.MageTargetKindRoute, Route: "lint", Authority: "internal/command registry"},
			{Name: "package", Kind: delivery.MageTargetKindRoute, Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
			{Name: "packagefoundation", Kind: delivery.MageTargetKindRoute, Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
			{Name: "pr", Kind: delivery.MageTargetKindPR, Authority: "config/commands.toml: commands.pr.steps, commands.pr.network_steps, commands.pr.mutation_steps"},
			{Name: "run", Kind: delivery.MageTargetKindRoute, Route: "run", Authority: "internal/command registry"},
			{Name: "test", Kind: delivery.MageTargetKindRoute, Route: "test", Authority: "internal/command registry"},
			{Name: "testquick", Kind: delivery.MageTargetKindRoute, Route: "test", Args: []string{"--quick"}, Authority: "internal/command registry"},
		}

		got := delivery.MageTargets()
		require.Len(t, got, len(want), "MageTargets() = %+v", got)
		for i := range want {
			require.Equal(t, want[i].Name, got[i].Name, "MageTargets()[%d].Name", i)
			require.Equal(t, want[i].Kind, got[i].Kind, "MageTargets()[%d].Kind", i)
			require.Equal(t, want[i].Route, got[i].Route, "MageTargets()[%d].Route", i)
			require.Equal(t, want[i].Authority, got[i].Authority, "MageTargets()[%d].Authority", i)
			require.Equal(t, strings.Join(want[i].Args, "\x00"), strings.Join(got[i].Args, "\x00"), "MageTargets()[%d].Args", i)
			require.Equal(t, want[i].EnvironmentOptions, got[i].EnvironmentOptions, "MageTargets()[%d].EnvironmentOptions", i)
		}

		got[0].Name = "mutated"
		got[0].EnvironmentOptions[0].Effect = "mutated"
		got[2].Args[0] = "--mutated"
		fresh := delivery.MageTargets()
		require.Equal(t, "bootstrap", fresh[0].Name, "MageTargets returned shared mutable state: %+v", fresh)
		require.Equal(t, "bootstrap.Options.IncludeLinearSync", fresh[0].EnvironmentOptions[0].Effect, "MageTargets returned shared mutable state: %+v", fresh)
		require.Equal(t, "--concern project", strings.Join(fresh[2].Args, " "), "MageTargets returned shared mutable state: %+v", fresh)

		check, ok := delivery.LookupMageTarget("check")
		require.True(t, ok, "LookupMageTarget(check) not found")
		check.Args[0] = "--mutated"
		again, ok := delivery.LookupMageTarget("check")
		require.True(t, ok, "LookupMageTarget returned shared mutable state: %+v, %v", again, ok)
		require.Equal(t, "--concern project", strings.Join(again.Args, " "), "LookupMageTarget returned shared mutable state: %+v, %v", again, ok)
		bootstrapTarget, ok := delivery.LookupMageTarget("bootstrap")
		require.True(t, ok, "LookupMageTarget(bootstrap) not found")
		bootstrapTarget.EnvironmentOptions[0].Name = "mutated"
		bootstrapAgain, _ := delivery.LookupMageTarget("bootstrap")
		require.Equal(t, delivery.BootstrapEnvironmentName, bootstrapAgain.EnvironmentOptions[0].Name, "LookupMageTarget returned shared environment option state: %+v", bootstrapAgain)
		_, ok = delivery.LookupMageTarget("Check")
		require.False(t, ok, "lookup must use the exact Mage CLI target name")
		_, ok = delivery.LookupMageTarget("unknown")
		require.False(t, ok, "unknown target unexpectedly resolved")
	})

	t.Run("LoadPRGraph follows strict configured order and policy", func(t *testing.T) {
		root := t.TempDir()
		writeFixturePRCommandsToml(t, root,
			"bootstrap,generate --check,check --offline,build,test,package --foundation",
			"bootstrap", "none")
		graph, err := delivery.LoadPRGraph(root)
		require.NoError(t, err, "LoadPRGraph")
		want := []struct {
			route string
			args  string
			net   delivery.NetworkPolicy
		}{
			{"bootstrap", "", delivery.NetworkAllowed},
			{"generate", "--check", delivery.NetworkDenied},
			{"check", "--offline", delivery.NetworkDenied},
			{"build", "", delivery.NetworkDenied},
			{"test", "", delivery.NetworkDenied},
			{"package", "--foundation", delivery.NetworkDenied},
		}
		require.Len(t, graph.Steps, len(want), "steps = %+v", graph.Steps)
		for i, expected := range want {
			step := graph.Steps[i]
			require.Equal(t, expected.route, step.Route, "step %d = %+v, want route=%q args=%q network=%v", i, step, expected.route, expected.args, expected.net)
			require.Equal(t, expected.args, strings.Join(step.Args, " "), "step %d = %+v, want route=%q args=%q network=%v", i, step, expected.route, expected.args, expected.net)
			require.Equal(t, expected.net, step.Network, "step %d = %+v, want route=%q args=%q network=%v", i, step, expected.route, expected.args, expected.net)
			require.NotEmpty(t, step.Name, "step %d has blank stable name", i)
		}
	})

	t.Run("LoadPRGraph order is fixture-authoritative", func(t *testing.T) {
		root := t.TempDir()
		writeFixturePRCommandsToml(t, root, "test,bootstrap,build", "bootstrap", "none")
		graph, err := delivery.LoadPRGraph(root)
		require.NoError(t, err, "LoadPRGraph")
		var got []string
		_, err = delivery.Run(graph, func(route string, args []string) (int, []byte, []byte) {
			got = append(got, route)
			return 0, nil, nil
		})
		require.NoError(t, err, "Run")
		require.Equal(t, "test,bootstrap,build", strings.Join(got, ","), "observed order")
	})

	t.Run("LoadPRGraph rejects malformed duplicate and orphan policy", func(t *testing.T) {
		cases := []struct {
			name, steps, network, mutation string
		}{
			{"blank step", "bootstrap,,build", "bootstrap", "none"},
			{"duplicate invocation", "bootstrap,build,build", "bootstrap", "none"},
			{"orphan network policy", "bootstrap,build", "bootstrap,test", "none"},
			{"mutation enabled", "bootstrap,build", "bootstrap", "build"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				root := t.TempDir()
				writeFixturePRCommandsToml(t, root, testCase.steps, testCase.network, testCase.mutation)
				_, err := delivery.LoadPRGraph(root)
				require.Error(t, err, "invalid PR graph unexpectedly loaded")
			})
		}
	})

	t.Run("LoadGraph reads exactly the two canonical commands keys and the fixed core steps", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureCommandsToml(t, root)

		graph, err := delivery.LoadGraph(root)
		require.NoError(t, err, "LoadGraph")
		wantCLI := filepath.ToSlash(bootstrap.PlatformExecutablePath(".tools/installs/golc_project", "golc-project"))
		require.Equal(t, wantCLI, graph.Inventory.CLIBinary, "CLIBinary")
		require.Equal(t, "1.26.5", graph.Inventory.GoVersion, "GoVersion")
		wantNames := []string{"generate", "check", "build", "test"}
		require.Len(t, graph.Steps, len(wantNames), "len(Steps)")
		for i, name := range wantNames {
			require.Equal(t, name, graph.Steps[i].Name, "Steps[%d].Name", i)
			require.Equal(t, delivery.NetworkDenied, graph.Steps[i].Network, "Steps[%d].Network", i)
		}
		// check invokes "--concern project", never "--offline" — a
		// check-driven graph run must never recurse into itself.
		checkStep := graph.Steps[1]
		require.Equal(t, "--concern project", strings.Join(checkStep.Args, " "), "check step Args")
	})

	t.Run("LoadGraph fails closed on a missing config/commands.toml", func(t *testing.T) {
		root := t.TempDir()
		_, err := delivery.LoadGraph(root)
		require.Error(t, err, "expected LoadGraph to fail for a missing config/commands.toml")
	})

	t.Run("LoadGraph fails closed on an incomplete commands inventory", func(t *testing.T) {
		root := t.TempDir()
		configDir := filepath.Join(root, "config")
		require.NoError(t, os.MkdirAll(configDir, 0o755), "mkdir config")
		body := "schema_version = 1\n\n[commands]\n"
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "commands.toml"), []byte(body), 0o644), "write config/commands.toml")
		_, err := delivery.LoadGraph(root)
		require.Error(t, err, "expected LoadGraph to fail for an incomplete commands inventory")
		require.Contains(t, err.Error(), "GOLC_DELIVERY_INVENTORY_INCOMPLETE")
	})

	t.Run("ValidateParity accepts the production graph and rejects duplicates", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureCommandsToml(t, root)
		graph, err := delivery.LoadGraph(root)
		require.NoError(t, err, "LoadGraph")
		require.NoError(t, delivery.ValidateParity(graph), "ValidateParity on the production graph")

		duplicateNames := graph
		duplicateNames.Steps = append(append([]delivery.Step{}, graph.Steps...), graph.Steps[0])
		require.Error(t, delivery.ValidateParity(duplicateNames), "expected ValidateParity to reject a duplicate step name")

		empty := graph
		empty.Steps = nil
		require.Error(t, delivery.ValidateParity(empty), "expected ValidateParity to reject a graph with zero steps")

		blankRoute := graph
		blankRoute.Steps = []delivery.Step{{Name: "x", Route: ""}}
		require.Error(t, delivery.ValidateParity(blankRoute), "expected ValidateParity to reject a step with a blank route")
	})

	t.Run("Run executes every step in order and stops at the first failure", func(t *testing.T) {
		graph := delivery.Graph{
			Root: t.TempDir(),
			Inventory: delivery.CommandInventory{
				CLIBinary: ".tools/x", GoVersion: "1.26.5",
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
		require.Error(t, err, "expected Run to fail when step two exits non-zero")
		require.Equal(t, "one,two", strings.Join(invoked, ","), "invoked routes (three must never run)")
		require.Len(t, results, 2)
		require.Equal(t, 1, results[1].ExitCode, "results[1].ExitCode")
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
		_, err := delivery.RunOffline(graph, executor)
		require.Error(t, err, "expected RunOffline to refuse a graph containing a NetworkAllowed step")
		require.False(t, executed, "RunOffline must never invoke the executor when it refuses the graph")
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

		_, err := delivery.RunOffline(graph, executor)
		require.NoError(t, err, "RunOffline")
		require.Equal(t, "off", observedGOPROXY, "observed GOPROXY during offline run")
		require.True(t, observedTransportIsDeny, "expected http.DefaultTransport to be DenyTransport during the offline run")
		require.Equal(t, "https://proxy.example.invalid", os.Getenv("GOPROXY"), "GOPROXY was not restored after RunOffline")
		require.Equal(t, previousTransport, http.DefaultTransport, "http.DefaultTransport was not restored after RunOffline")
	})

	t.Run("DenyTransport fails every request with a named diagnostic before any dial", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid/resource", nil)
		require.NoError(t, err, "NewRequest")
		_, err = (delivery.DenyTransport{}).RoundTrip(request)
		require.Error(t, err, "expected DenyTransport.RoundTrip to fail")
		require.Contains(t, err.Error(), "GOLC_DELIVERY_NETWORK_DENIED")
	})

	t.Run("NetworkPolicy renders stable diagnostics", func(t *testing.T) {
		require.Equal(t, "denied", delivery.NetworkDenied.String(), "NetworkDenied.String()")
		require.Equal(t, "allowed", delivery.NetworkAllowed.String(), "NetworkAllowed.String()")
	})

	t.Run("package --foundation route is self-registered and reachable", func(t *testing.T) {
		registry, err := command.NewDefaultCommandRegistry()
		require.NoError(t, err, "NewDefaultCommandRegistry")
		registration, rest, ok := registry.Lookup([]string{"package", "--foundation"})
		require.True(t, ok, "expected the default registry to resolve \"package --foundation\"")
		require.Equal(t, "package", registration.Route, "Route")
		require.Equal(t, "--foundation", strings.Join(rest, " "), "remaining args")
	})

	t.Run("FoundationInventory returns a sorted, duplicate-free allowlist derived from the graph inventory", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		graph, err := delivery.LoadGraph(root)
		require.NoError(t, err, "LoadGraph")

		entries, err := delivery.FoundationInventory(root, graph.Inventory)
		require.NoError(t, err, "FoundationInventory")

		wantPaths := []string{
			filepath.ToSlash(bootstrap.PlatformExecutablePath(".tools/installs/golc_project", "golc-project")),
			"config/commands.toml",
			"config/integrations/linear.toml",
			"config/toolchain.toml",
			"docs/development.md",
			"golc.project.toml",
			"schemas/config-commands.schema.json",
			"schemas/golc-project.schema.json",
		}
		gotPaths := make([]string, len(entries))
		for i, entry := range entries {
			gotPaths[i] = entry.ArchivePath
		}
		require.Equal(t, strings.Join(wantPaths, ","), strings.Join(gotPaths, ","), "FoundationInventory paths")
		require.True(t, sort.StringsAreSorted(gotPaths), "expected FoundationInventory to return sorted archive paths, got %v", gotPaths)

		incomplete := delivery.CommandInventory{}
		_, err = delivery.FoundationInventory(root, incomplete)
		require.Error(t, err, "expected FoundationInventory to reject an incomplete graph inventory")
	})

	t.Run("CanonicalManifest sorts, hashes, and rejects a duplicate archive path", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "b.txt"), []byte("second\n"), 0o644), "write b.txt")
		require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("first\n"), 0o644), "write a.txt")

		manifest, payloads, err := delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "b.txt", SourcePath: "b.txt"},
			{ArchivePath: "a.txt", SourcePath: "a.txt"},
		})
		require.NoError(t, err, "CanonicalManifest")
		require.Len(t, manifest.Files, 2, "expected 2 files/payloads, got %d/%d", len(manifest.Files), len(payloads))
		require.Len(t, payloads, 2, "expected 2 files/payloads, got %d/%d", len(manifest.Files), len(payloads))
		require.Equal(t, "a.txt", manifest.Files[0].Path, "expected manifest sorted by archive path, got %v", manifest.Files)
		require.Equal(t, "b.txt", manifest.Files[1].Path, "expected manifest sorted by archive path, got %v", manifest.Files)
		wantSHA256 := "b640e840b19d378660b32fb51ae18d67dccb4a8596a29e7bd72c1b2ae5928f41"
		require.Equal(t, wantSHA256, manifest.Files[0].SHA256, "a.txt sha256")
		require.Equal(t, int64(len("first\n")), manifest.Files[0].Size, "a.txt size")
		require.Equal(t, "first\n", string(payloads[0]), "payloads[0]")

		_, _, err = delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "a.txt", SourcePath: "a.txt"},
			{ArchivePath: "a.txt", SourcePath: "b.txt"},
		})
		require.Error(t, err, "expected CanonicalManifest to reject a duplicate archive path")

		_, _, err = delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "", SourcePath: "a.txt"},
		})
		require.Error(t, err, "expected CanonicalManifest to reject a blank archive path")

		_, _, err = delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "missing.txt", SourcePath: "missing.txt"},
		})
		require.Error(t, err, "expected CanonicalManifest to fail closed on a missing source file")
	})

	t.Run("EncodeManifest is deterministic byte-stable JSON matching the committed golden fixture", func(t *testing.T) {
		// The committed golden fixture pins the windows-amd64 CLI binary
		// path. FoundationInventory resolves that path via graph.go's
		// bootstrap.PlatformExecutablePath, which is keyed off the actual
		// host runtime.GOOS/GOARCH, not off anything this test's fixture
		// writer controls -- so on a non-Windows host the production code
		// looks for (and fails to find) a darwin-arm64/linux-amd64 binary
		// regardless of what path writeFoundationFixtureForGoldenManifest
		// wrote to disk (observed live in cross-platform-mage.yml run
		// 30111784838 on both macos-latest and ubuntu-latest). This exact
		// byte-for-byte comparison is therefore only meaningful on the one
		// platform GOLC's foundation bundle targets (see this package's
		// doc comment: "a reproducible Windows AMD64 developer-tool ZIP");
		// mage PackageFoundation on other platforms is otherwise still
		// exercised by cross-platform-mage.yml, just not against this
		// golden byte comparison.
		if runtime.GOOS != "windows" {
			t.Skip("golden foundation manifest fixture pins a windows-amd64 CLI binary path; see internal/delivery/foundation.go's package doc")
		}
		root := t.TempDir()
		writeFoundationFixtureForGoldenManifest(t, root)

		graph, err := delivery.LoadGraph(root)
		require.NoError(t, err, "LoadGraph")
		entries, err := delivery.FoundationInventory(root, graph.Inventory)
		require.NoError(t, err, "FoundationInventory")
		manifest, _, err := delivery.CanonicalManifest(root, entries)
		require.NoError(t, err, "CanonicalManifest")
		encoded, err := delivery.EncodeManifest(manifest)
		require.NoError(t, err, "EncodeManifest")

		again, err := delivery.EncodeManifest(manifest)
		require.NoError(t, err, "EncodeManifest (repeat)")
		require.True(t, bytes.Equal(encoded, again), "expected EncodeManifest to be byte-identical across repeated calls with unchanged input")
		require.Equal(t, byte('\n'), encoded[len(encoded)-1], "expected LF-only output ending with exactly one trailing newline, got %q", encoded)
		require.False(t, bytes.Contains(encoded, []byte("\r\n")), "expected LF-only output ending with exactly one trailing newline, got %q", encoded)

		golden, err := os.ReadFile(goldenFoundationManifestPath(t))
		require.NoError(t, err, "read golden foundation manifest")
		require.Equal(t, golden, encoded, "EncodeManifest output does not match tests/golden/foundation-manifest.json:\ngot:  %s\nwant: %s", encoded, golden)
	})

	t.Run("BuildFoundationBundle produces byte-identical ZIP, manifest, and checksums across repeated runs", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		first, err := delivery.BuildFoundationBundle(root)
		require.NoError(t, err, "BuildFoundationBundle (first)")
		second, err := delivery.BuildFoundationBundle(root)
		require.NoError(t, err, "BuildFoundationBundle (second)")

		require.True(t, bytes.Equal(first.ZIPBytes, second.ZIPBytes), "expected byte-identical ZIP bytes across repeated builds of unchanged inputs")
		require.True(t, bytes.Equal(first.ManifestBytes, second.ManifestBytes), "expected byte-identical manifest bytes across repeated builds of unchanged inputs")
		require.Equal(t, first.ZIPChecksum, second.ZIPChecksum, "expected byte-identical checksums across repeated builds of unchanged inputs")
		require.Equal(t, first.ManifestChecksum, second.ManifestChecksum, "expected byte-identical checksums across repeated builds of unchanged inputs")
		require.Len(t, first.ZIPChecksum, 64, "expected 64-hex-character SHA-256 checksums, got zip=%q manifest=%q", first.ZIPChecksum, first.ManifestChecksum)
		require.Len(t, first.ManifestChecksum, 64, "expected 64-hex-character SHA-256 checksums, got zip=%q manifest=%q", first.ZIPChecksum, first.ManifestChecksum)

		reader, err := zip.NewReader(bytes.NewReader(first.ZIPBytes), int64(len(first.ZIPBytes)))
		require.NoError(t, err, "zip.NewReader")
		require.Len(t, reader.File, len(first.Manifest.Files)+1, "zip entry count (manifest files + the embedded manifest)")
		seenNames := map[string]bool{}
		for _, zipEntry := range reader.File {
			seenNames[zipEntry.Name] = true
			require.False(t, strings.Contains(zipEntry.Name, "\\"), "zip entry %q must use forward slashes only", zipEntry.Name)
			require.Equal(t, os.FileMode(0o644), zipEntry.Mode().Perm(), "zip entry %q mode = %v, want 0644", zipEntry.Name, zipEntry.Mode().Perm())
			wantEpoch := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
			require.True(t, zipEntry.Modified.Equal(wantEpoch), "zip entry %q Modified = %v, want the fixed epoch %v (no machine timestamp)", zipEntry.Name, zipEntry.Modified, wantEpoch)
		}
		for _, file := range first.Manifest.Files {
			require.True(t, seenNames[file.Path], "expected manifest entry %q to be present as a zip entry", file.Path)
		}
		require.True(t, seenNames["foundation-manifest.json"], "expected the zip to embed foundation-manifest.json")

		manifestZipFile, err := reader.Open("foundation-manifest.json")
		require.NoError(t, err, "open embedded manifest")
		defer manifestZipFile.Close()
		var decodedManifest struct {
			SchemaVersion int `json:"schema_version"`
			Files         []struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
				Size   int64  `json:"size"`
			} `json:"files"`
		}
		require.NoError(t, json.NewDecoder(manifestZipFile).Decode(&decodedManifest), "decode embedded manifest")
		require.Equal(t, 1, decodedManifest.SchemaVersion, "embedded manifest schema_version")
		require.Len(t, decodedManifest.Files, len(first.Manifest.Files), "embedded manifest files")
	})

	t.Run("WriteFoundationBundle writes the ZIP, manifest, and sha256 sidecar to the fixed output paths", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		bundle, err := delivery.BuildFoundationBundle(root)
		require.NoError(t, err, "BuildFoundationBundle")
		paths := delivery.DefaultFoundationOutputPaths(root)
		wantBase := "golc-foundation-" + bootstrap.PlatformKey()
		require.Equal(t, wantBase+".zip", filepath.Base(paths.ZIPPath), "platform foundation paths = %+v", paths)
		require.Equal(t, wantBase+".manifest.json", filepath.Base(paths.ManifestPath), "platform foundation paths = %+v", paths)
		require.Equal(t, wantBase+".zip.sha256", filepath.Base(paths.ChecksumPath), "platform foundation paths = %+v", paths)
		require.NoError(t, delivery.WriteFoundationBundle(bundle, paths), "WriteFoundationBundle")

		zipBytes, err := os.ReadFile(paths.ZIPPath)
		require.NoError(t, err, "read written zip")
		require.True(t, bytes.Equal(zipBytes, bundle.ZIPBytes), "expected the written zip file to match bundle.ZIPBytes exactly")

		checksumBytes, err := os.ReadFile(paths.ChecksumPath)
		require.NoError(t, err, "read written checksum sidecar")
		wantChecksumLine := bundle.ZIPChecksum + "  golc-foundation-" + bootstrap.PlatformKey() + ".zip\n"
		require.Equal(t, wantChecksumLine, string(checksumBytes), "checksum sidecar")

		// A second write must replace the prior output at the exact same
		// path rather than accumulating a second differently-named
		// artifact (this test's own repeat-and-compare verification below
		// depends on this fixed identity).
		require.NoError(t, delivery.WriteFoundationBundle(bundle, paths), "WriteFoundationBundle (second write)")
		zipBytesAgain, err := os.ReadFile(paths.ZIPPath)
		require.NoError(t, err, "read written zip (second write)")
		require.True(t, bytes.Equal(zipBytesAgain, bundle.ZIPBytes), "expected the second write to leave byte-identical output at the same fixed path")
	})
}

// writeFoundationFixture writes a minimal, self-contained repository tree
// under root that FoundationInventory/BuildFoundationBundle can operate
// on: config/commands.toml (with an exact, deterministic
// cli_binary/go_version so LoadGraph succeeds), one additional config
// concern file, one nested integrations concern file, the cli_binary
// file itself, docs/development.md, and two schema fixtures —
// deliberately independent of the real repository's current file set so
// this fixture (and the golden manifest it produces) never drifts when
// the real repository gains or loses files.
func writeFoundationFixture(t *testing.T, root string) {
	t.Helper()
	writeFoundationFixtureWithBinaryPath(t, root, bootstrap.PlatformExecutablePath(".tools/installs/golc_project", "golc-project"))
}

// writeFoundationFixtureForGoldenManifest is writeFoundationFixture with
// the CLI binary path fixed to windows-amd64 regardless of the host
// platform running the test. tests/golden/foundation-manifest.json is
// one committed, platform-specific reference file: EncodeManifest's own
// determinism/byte-stability is what this test verifies, not "the
// running host happens to be Windows", so the fixture must simulate a
// Windows install on every host (observed live: cross-platform-mage.yml
// run 30110425773 failed this test on ubuntu-latest and macos-latest
// with a linux-amd64/darwin-arm64 path where the golden fixture always
// expects windows-amd64 -- the plain writeFoundationFixture above uses
// bootstrap.PlatformExecutablePath, which resolves against the actual
// running platform, exactly the coupling this test must not have).
func writeFoundationFixtureForGoldenManifest(t *testing.T, root string) {
	t.Helper()
	writeFoundationFixtureWithBinaryPath(t, root, ".tools/installs/golc_project/windows-amd64/bin/golc-project.exe")
}

func writeFoundationFixtureWithBinaryPath(t *testing.T, root, binaryPath string) {
	t.Helper()
	files := map[string]string{
		"golc.project.toml":                   "schema_version = 1\n",
		"docs/development.md":                 "# Fixture Docs\n",
		"config/commands.toml":                "schema_version = 1\n\n[commands]\ncli_binary = \".tools/installs/golc_project\"\ngo_version = \"1.26.5\"\n",
		"config/toolchain.toml":               "schema_version = 1\n",
		"config/integrations/linear.toml":     "schema_version = 1\n",
		"schemas/golc-project.schema.json":    "{}\n",
		"schemas/config-commands.schema.json": "{}\n",
		filepath.ToSlash(binaryPath):          "fixture binary payload\n",
	}
	for relative, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(relative))
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755), "mkdir for %s", relative)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644), "write %s", relative)
	}
}

// goldenFoundationManifestPath locates the committed golden fixture
// tests/golden/foundation-manifest.json by walking up from the current
// working directory (go test's working directory is always the package
// directory, internal/delivery) to the repository root.
func goldenFoundationManifestPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err, "os.Getwd")
	// internal/delivery -> internal -> repository root
	repoRoot := filepath.Dir(filepath.Dir(wd))
	return filepath.Join(repoRoot, "tests", "golden", "foundation-manifest.json")
}
