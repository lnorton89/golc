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
	"sort"
	"strings"
	"testing"
	"time"

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

	t.Run("package --foundation route is self-registered and reachable", func(t *testing.T) {
		registry, err := command.NewDefaultCommandRegistry()
		if err != nil {
			t.Fatalf("NewDefaultCommandRegistry: %v", err)
		}
		registration, rest, ok := registry.Lookup([]string{"package", "--foundation"})
		if !ok {
			t.Fatal("expected the default registry to resolve \"package --foundation\"")
		}
		if registration.Route != "package" {
			t.Fatalf("Route = %q, want \"package\"", registration.Route)
		}
		if strings.Join(rest, " ") != "--foundation" {
			t.Fatalf("remaining args = %v, want [--foundation]", rest)
		}
	})

	t.Run("FoundationInventory returns a sorted, duplicate-free allowlist derived from the graph inventory", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		graph, err := delivery.LoadGraph(root)
		if err != nil {
			t.Fatalf("LoadGraph: %v", err)
		}

		entries, err := delivery.FoundationInventory(root, graph.Inventory)
		if err != nil {
			t.Fatalf("FoundationInventory: %v", err)
		}

		wantPaths := []string{
			".tools/installs/golc_project/bin/golc-project.exe",
			"config/commands.toml",
			"config/integrations/linear.toml",
			"config/toolchain.toml",
			"docs/development.md",
			"golc.project.toml",
			"golc.ps1",
			"schemas/config-commands.schema.json",
			"schemas/golc-project.schema.json",
		}
		gotPaths := make([]string, len(entries))
		for i, entry := range entries {
			gotPaths[i] = entry.ArchivePath
		}
		if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
			t.Fatalf("FoundationInventory paths = %v, want %v", gotPaths, wantPaths)
		}
		if !sort.StringsAreSorted(gotPaths) {
			t.Fatalf("expected FoundationInventory to return sorted archive paths, got %v", gotPaths)
		}

		incomplete := delivery.CommandInventory{Entrypoint: "golc.ps1"}
		if _, err := delivery.FoundationInventory(root, incomplete); err == nil {
			t.Fatal("expected FoundationInventory to reject an incomplete graph inventory")
		}
	})

	t.Run("CanonicalManifest sorts, hashes, and rejects a duplicate archive path", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("second\n"), 0o644); err != nil {
			t.Fatalf("write b.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("first\n"), 0o644); err != nil {
			t.Fatalf("write a.txt: %v", err)
		}

		manifest, payloads, err := delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "b.txt", SourcePath: "b.txt"},
			{ArchivePath: "a.txt", SourcePath: "a.txt"},
		})
		if err != nil {
			t.Fatalf("CanonicalManifest: %v", err)
		}
		if len(manifest.Files) != 2 || len(payloads) != 2 {
			t.Fatalf("expected 2 files/payloads, got %d/%d", len(manifest.Files), len(payloads))
		}
		if manifest.Files[0].Path != "a.txt" || manifest.Files[1].Path != "b.txt" {
			t.Fatalf("expected manifest sorted by archive path, got %v", manifest.Files)
		}
		wantSHA256 := "b640e840b19d378660b32fb51ae18d67dccb4a8596a29e7bd72c1b2ae5928f41"
		if manifest.Files[0].SHA256 != wantSHA256 {
			t.Fatalf("a.txt sha256 = %s, want %s", manifest.Files[0].SHA256, wantSHA256)
		}
		if manifest.Files[0].Size != int64(len("first\n")) {
			t.Fatalf("a.txt size = %d, want %d", manifest.Files[0].Size, len("first\n"))
		}
		if string(payloads[0]) != "first\n" {
			t.Fatalf("payloads[0] = %q, want %q", payloads[0], "first\n")
		}

		if _, _, err := delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "a.txt", SourcePath: "a.txt"},
			{ArchivePath: "a.txt", SourcePath: "b.txt"},
		}); err == nil {
			t.Fatal("expected CanonicalManifest to reject a duplicate archive path")
		}

		if _, _, err := delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "", SourcePath: "a.txt"},
		}); err == nil {
			t.Fatal("expected CanonicalManifest to reject a blank archive path")
		}

		if _, _, err := delivery.CanonicalManifest(root, []delivery.FoundationEntry{
			{ArchivePath: "missing.txt", SourcePath: "missing.txt"},
		}); err == nil {
			t.Fatal("expected CanonicalManifest to fail closed on a missing source file")
		}
	})

	t.Run("EncodeManifest is deterministic byte-stable JSON matching the committed golden fixture", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		graph, err := delivery.LoadGraph(root)
		if err != nil {
			t.Fatalf("LoadGraph: %v", err)
		}
		entries, err := delivery.FoundationInventory(root, graph.Inventory)
		if err != nil {
			t.Fatalf("FoundationInventory: %v", err)
		}
		manifest, _, err := delivery.CanonicalManifest(root, entries)
		if err != nil {
			t.Fatalf("CanonicalManifest: %v", err)
		}
		encoded, err := delivery.EncodeManifest(manifest)
		if err != nil {
			t.Fatalf("EncodeManifest: %v", err)
		}

		again, err := delivery.EncodeManifest(manifest)
		if err != nil {
			t.Fatalf("EncodeManifest (repeat): %v", err)
		}
		if !bytes.Equal(encoded, again) {
			t.Fatal("expected EncodeManifest to be byte-identical across repeated calls with unchanged input")
		}
		if encoded[len(encoded)-1] != '\n' || bytes.Contains(encoded, []byte("\r\n")) {
			t.Fatalf("expected LF-only output ending with exactly one trailing newline, got %q", encoded)
		}

		golden, err := os.ReadFile(goldenFoundationManifestPath(t))
		if err != nil {
			t.Fatalf("read golden foundation manifest: %v", err)
		}
		if !bytes.Equal(encoded, golden) {
			t.Fatalf("EncodeManifest output does not match tests/golden/foundation-manifest.json:\ngot:  %s\nwant: %s", encoded, golden)
		}
	})

	t.Run("BuildFoundationBundle produces byte-identical ZIP, manifest, and checksums across repeated runs", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		first, err := delivery.BuildFoundationBundle(root)
		if err != nil {
			t.Fatalf("BuildFoundationBundle (first): %v", err)
		}
		second, err := delivery.BuildFoundationBundle(root)
		if err != nil {
			t.Fatalf("BuildFoundationBundle (second): %v", err)
		}

		if !bytes.Equal(first.ZIPBytes, second.ZIPBytes) {
			t.Fatal("expected byte-identical ZIP bytes across repeated builds of unchanged inputs")
		}
		if !bytes.Equal(first.ManifestBytes, second.ManifestBytes) {
			t.Fatal("expected byte-identical manifest bytes across repeated builds of unchanged inputs")
		}
		if first.ZIPChecksum != second.ZIPChecksum || first.ManifestChecksum != second.ManifestChecksum {
			t.Fatal("expected byte-identical checksums across repeated builds of unchanged inputs")
		}
		if len(first.ZIPChecksum) != 64 || len(first.ManifestChecksum) != 64 {
			t.Fatalf("expected 64-hex-character SHA-256 checksums, got zip=%q manifest=%q", first.ZIPChecksum, first.ManifestChecksum)
		}

		reader, err := zip.NewReader(bytes.NewReader(first.ZIPBytes), int64(len(first.ZIPBytes)))
		if err != nil {
			t.Fatalf("zip.NewReader: %v", err)
		}
		if len(reader.File) != len(first.Manifest.Files)+1 {
			t.Fatalf("zip entry count = %d, want %d (manifest files + the embedded manifest)", len(reader.File), len(first.Manifest.Files)+1)
		}
		seenNames := map[string]bool{}
		for _, zipEntry := range reader.File {
			seenNames[zipEntry.Name] = true
			if strings.Contains(zipEntry.Name, "\\") {
				t.Fatalf("zip entry %q must use forward slashes only", zipEntry.Name)
			}
			if zipEntry.Mode().Perm() != 0o644 {
				t.Fatalf("zip entry %q mode = %v, want 0644", zipEntry.Name, zipEntry.Mode().Perm())
			}
			wantEpoch := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
			if !zipEntry.Modified.Equal(wantEpoch) {
				t.Fatalf("zip entry %q Modified = %v, want the fixed epoch %v (no machine timestamp)", zipEntry.Name, zipEntry.Modified, wantEpoch)
			}
		}
		for _, file := range first.Manifest.Files {
			if !seenNames[file.Path] {
				t.Fatalf("expected manifest entry %q to be present as a zip entry", file.Path)
			}
		}
		if !seenNames["foundation-manifest.json"] {
			t.Fatal("expected the zip to embed foundation-manifest.json")
		}

		manifestZipFile, err := reader.Open("foundation-manifest.json")
		if err != nil {
			t.Fatalf("open embedded manifest: %v", err)
		}
		defer manifestZipFile.Close()
		var decodedManifest struct {
			SchemaVersion int `json:"schema_version"`
			Files         []struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
				Size   int64  `json:"size"`
			} `json:"files"`
		}
		if err := json.NewDecoder(manifestZipFile).Decode(&decodedManifest); err != nil {
			t.Fatalf("decode embedded manifest: %v", err)
		}
		if decodedManifest.SchemaVersion != 1 {
			t.Fatalf("embedded manifest schema_version = %d, want 1", decodedManifest.SchemaVersion)
		}
		if len(decodedManifest.Files) != len(first.Manifest.Files) {
			t.Fatalf("embedded manifest has %d files, want %d", len(decodedManifest.Files), len(first.Manifest.Files))
		}
	})

	t.Run("WriteFoundationBundle writes the ZIP, manifest, and sha256 sidecar to the fixed output paths", func(t *testing.T) {
		root := t.TempDir()
		writeFoundationFixture(t, root)

		bundle, err := delivery.BuildFoundationBundle(root)
		if err != nil {
			t.Fatalf("BuildFoundationBundle: %v", err)
		}
		paths := delivery.DefaultFoundationOutputPaths(root)
		if err := delivery.WriteFoundationBundle(bundle, paths); err != nil {
			t.Fatalf("WriteFoundationBundle: %v", err)
		}

		zipBytes, err := os.ReadFile(paths.ZIPPath)
		if err != nil {
			t.Fatalf("read written zip: %v", err)
		}
		if !bytes.Equal(zipBytes, bundle.ZIPBytes) {
			t.Fatal("expected the written zip file to match bundle.ZIPBytes exactly")
		}

		checksumBytes, err := os.ReadFile(paths.ChecksumPath)
		if err != nil {
			t.Fatalf("read written checksum sidecar: %v", err)
		}
		wantChecksumLine := bundle.ZIPChecksum + "  golc-foundation-windows-amd64.zip\n"
		if string(checksumBytes) != wantChecksumLine {
			t.Fatalf("checksum sidecar = %q, want %q", checksumBytes, wantChecksumLine)
		}

		// A second write must replace the prior output at the exact same
		// path rather than accumulating a second differently-named
		// artifact (offline.ps1 -Mode package's repeat-and-compare
		// verification depends on this fixed identity).
		if err := delivery.WriteFoundationBundle(bundle, paths); err != nil {
			t.Fatalf("WriteFoundationBundle (second write): %v", err)
		}
		zipBytesAgain, err := os.ReadFile(paths.ZIPPath)
		if err != nil {
			t.Fatalf("read written zip (second write): %v", err)
		}
		if !bytes.Equal(zipBytesAgain, bundle.ZIPBytes) {
			t.Fatal("expected the second write to leave byte-identical output at the same fixed path")
		}
	})
}

// writeFoundationFixture writes a minimal, self-contained repository tree
// under root that FoundationInventory/BuildFoundationBundle can operate
// on: config/commands.toml (with an exact, deterministic
// entrypoint/cli_binary/go_version so LoadGraph succeeds), one additional
// config concern file, one nested integrations concern file, the
// entrypoint and cli_binary files themselves, docs/development.md, and
// two schema fixtures — deliberately independent of the real repository's
// current file set so this fixture (and the golden manifest it produces)
// never drifts when the real repository gains or loses files.
func writeFoundationFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"golc.ps1":                                          "REM golc.ps1 fixture entrypoint\n",
		"golc.project.toml":                                 "schema_version = 1\n",
		"docs/development.md":                               "# Fixture Docs\n",
		"config/commands.toml":                              "schema_version = 1\n\n[commands]\nentrypoint = \"golc.ps1\"\ncli_binary = \".tools/installs/golc_project/bin/golc-project.exe\"\ngo_version = \"1.26.5\"\n",
		"config/toolchain.toml":                             "schema_version = 1\n",
		"config/integrations/linear.toml":                   "schema_version = 1\n",
		"schemas/golc-project.schema.json":                  "{}\n",
		"schemas/config-commands.schema.json":               "{}\n",
		".tools/installs/golc_project/bin/golc-project.exe": "fixture binary payload\n",
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

// goldenFoundationManifestPath locates the committed golden fixture
// tests/golden/foundation-manifest.json by walking up from the current
// working directory (go test's working directory is always the package
// directory, internal/delivery) to the repository root.
func goldenFoundationManifestPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	// internal/delivery -> internal -> repository root
	repoRoot := filepath.Dir(filepath.Dir(wd))
	return filepath.Join(repoRoot, "tests", "golden", "foundation-manifest.json")
}
