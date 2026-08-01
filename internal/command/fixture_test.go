// fixture_test.go proves FIXT-04's "fixture validate" route contract
// (02-01-PLAN.md, Task 1 Wave-0 scaffold): a valid hand-authored fixture
// file validates with ExitCode 0 and a deterministic canonical summary; a
// fixture file with a duplicate mapping key is rejected with ExitCode 2
// and a GOLC_FIXTURE_YAML_INVALID diagnostic on Stderr. It follows
// router_test.go's exact route-invocation convention: build the default
// registry (command files self-register their routes/scopes per D-03),
// Execute a Request, assert Result.ExitCode/Stdout/Stderr.
//
// This file compiles today (it only depends on the already-implemented
// command package), but fails at RUN time until Task 2/3 of
// 02-01-PLAN.md self-register the "fixture validate" route -- that is the
// RED state this task proves.
package command_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lnorton89/golc/internal/command"
	"github.com/stretchr/testify/require"
)

const fixtureValidRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
    channels:
      - type: intensity
        occurrence: 0
      - type: color
        occurrence: 0
capabilities:
  - type: intensity
    range: [0, 1]
  - type: color
    range: [0, 1]
`

const fixtureDuplicateKeyYAML = `schema_version: 1
manufacturer: Generic
manufacturer: Generic Duplicate
model: RGB PAR
modes:
  - name: Standard
capabilities:
  - type: intensity
    range: [0, 1]
`

func writeFixtureTestFile(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		require.NoError(t, err, "write %s: %v", path, err)
	}
	return path
}

func TestFixtureValidateRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()

	t.Run("valid fixture exits 0 with a deterministic canonical summary", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "valid.yaml", fixtureValidRGBParYAML)

		first := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		require.Equal(t, 0, first.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", first.ExitCode, first.Stderr)
		require.NotEmpty(t, first.Stdout, "expected a non-empty canonical summary on Stdout")

		second := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		require.Equal(t, 0, second.ExitCode, "expected second ExitCode 0, got %d (stderr: %s)", second.ExitCode, second.Stderr)
		require.Equal(t, string(second.Stdout), string(first.Stdout), "expected byte-identical repeated validation:\nfirst:  %s\nsecond: %s", first.Stdout, second.Stdout)
	})

	t.Run("duplicate-key fixture exits 2 with GOLC_FIXTURE_YAML_INVALID", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "duplicate-key.yaml", fixtureDuplicateKeyYAML)

		result := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		require.Equal(t, 2, result.ExitCode, "expected ExitCode 2, got %d (stdout: %s)", result.ExitCode, result.Stdout)
		require.Contains(t, string(result.Stderr), "GOLC_FIXTURE_YAML_INVALID", "expected GOLC_FIXTURE_YAML_INVALID on Stderr, got %q", result.Stderr)
	})
}

// TestFixtureInspectRoute proves FIXT-05/FIXT-06's "fixture inspect" route
// contract (02-02-PLAN.md, Task 1 Wave-0 scaffold): a valid hand-authored
// fixture file inspects with ExitCode 0 and a deterministic JSON envelope
// containing an allowlisted identity + provenance projection, and that
// envelope never contains an absolute filesystem path (T-01-23); an
// invalid fixture file inspects with ExitCode 2 and the underlying
// GOLC_FIXTURE_* diagnostic.
//
// This file compiles today (it only depends on the already-implemented
// command package), but fails at RUN time until Task 2/3 of
// 02-02-PLAN.md implement identity/provenance and self-register the
// "fixture inspect" route -- that is the RED state this task proves.
func TestFixtureInspectRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()

	t.Run("valid fixture exits 0 with a deterministic identity+provenance envelope and no absolute path", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "valid.yaml", fixtureValidRGBParYAML)

		first := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		require.Equal(t, 0, first.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", first.ExitCode, first.Stderr)
		require.NotEmpty(t, first.Stdout, "expected a non-empty identity+provenance envelope on Stdout")
		require.NotContains(t, string(first.Stdout), root, "expected no absolute filesystem path (temp root %q) in Stdout, got %q", root, first.Stdout)

		second := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		require.Equal(t, 0, second.ExitCode, "expected second ExitCode 0, got %d (stderr: %s)", second.ExitCode, second.Stderr)
		require.Equal(t, string(second.Stdout), string(first.Stdout), "expected byte-identical repeated inspect:\nfirst:  %s\nsecond: %s", first.Stdout, second.Stdout)
	})

	t.Run("duplicate-key fixture exits 2 with GOLC_FIXTURE_YAML_INVALID", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "duplicate-key-inspect.yaml", fixtureDuplicateKeyYAML)

		result := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		require.Equal(t, 2, result.ExitCode, "expected ExitCode 2, got %d (stdout: %s)", result.ExitCode, result.Stdout)
		require.Contains(t, string(result.Stderr), "GOLC_FIXTURE_YAML_INVALID", "expected GOLC_FIXTURE_YAML_INVALID on Stderr, got %q", result.Stderr)
	})

	// TestFixtureInspectRoute/an OFL-imported .json envelope inspects as
	// valid, not GOLC_FIXTURE_YAML_INVALID proves the bug a library row
	// (FixtureLibraryWorkspace.tsx, ListLocal-backed) reported "valid" while
	// its own Inspect panel reported invalid: "fixture import --out" writes
	// an ImportEnvelope{Definition, Provenance} JSON document, never a bare
	// FixtureDefinition -- "fixture inspect" must decode that shape through
	// DecodeEnvelope (like ListDirectory's own .json branch already does),
	// not fixture.Decode's YAML-only bare-definition path.
	t.Run("an OFL-imported .json envelope inspects as valid, not GOLC_FIXTURE_YAML_INVALID", func(t *testing.T) {
		corpusPath := filepath.Join(oflCorpusDir(t), "chauvet-dj_led-par-64-tri-b.json")
		importedPath := filepath.Join(root, "imported-for-inspect.json")

		importResult := registry.Execute(command.Request{Root: root, Args: []string{
			"fixture", "import", "--ofl-file", corpusPath, "--out", importedPath,
		}})
		require.Equal(t, 0, importResult.ExitCode, "expected the import to succeed (ExitCode 0), got %d (stderr: %s)", importResult.ExitCode, importResult.Stderr)

		result := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", importedPath}})
		require.Equal(t, 0, result.ExitCode, "expected ExitCode 0 inspecting an imported .json envelope, got %d (stderr: %s)", result.ExitCode, result.Stderr)
		require.Contains(t, string(result.Stdout), `"content_hash"`, "expected a pinned content_hash in Stdout, got %s", result.Stdout)
		require.Contains(t, string(result.Stdout), `"ofl:`, "expected the envelope's own OFL provenance source (not a recomputed path-based one), got %s", result.Stdout)
	})
}

// oflCorpusDir resolves the repository's pinned offline OFL test corpus
// directory (tests/fixtures/ofl), relative to this package's own
// directory, so TestFixtureImportRoute never depends on live network
// access.
func oflCorpusDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "tests", "fixtures", "ofl"))
	require.NoError(t, err)
	return dir
}

// TestFixtureImportRoute proves FIXT-03/FIXT-06's "fixture import" route
// contract (02-03-PLAN.md, Task 1 Wave-0 scaffold): "fixture import
// --ofl-file <corpus file> --out <path>" imports offline (this code path
// never calls ofl.Fetch -- see internal/command/fixture.go's
// runFixtureImport) with ExitCode 0, and writes a pinned canonical
// fixture + provenance envelope to --out.
//
// This file compiles today (it depends only on the already-implemented
// command package), but fails at RUN time until 02-03-PLAN.md's Task 3
// self-registers the "fixture import" route -- that is the RED state
// this task proves.
func TestFixtureImportRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	require.NoError(t, err)
	root := t.TempDir()

	t.Run("--ofl-file imports offline with ExitCode 0 and writes a pinned fixture+provenance", func(t *testing.T) {
		corpusPath := filepath.Join(oflCorpusDir(t), "chauvet-dj_led-par-64-tri-b.json")
		outPath := filepath.Join(root, "imported.json")

		result := registry.Execute(command.Request{Root: root, Args: []string{
			"fixture", "import", "--ofl-file", corpusPath, "--out", outPath,
		}})
		require.Equal(t, 0, result.ExitCode, "expected ExitCode 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)

		written, readErr := os.ReadFile(outPath)
		require.NoError(t, readErr)
		require.NotEmpty(t, written, "expected a non-empty written fixture+provenance payload")
		require.Contains(t, string(written), `"content_hash"`, "expected the written payload to contain a pinned content_hash, got %s", written)
		require.Contains(t, string(written), `"warnings"`, "expected the written payload to contain a provenance warnings array, got %s", written)
	})

	t.Run("--ofl and --ofl-file together are rejected with GOLC_FIXTURE_USAGE", func(t *testing.T) {
		corpusPath := filepath.Join(oflCorpusDir(t), "chauvet-dj_led-par-64-tri-b.json")
		outPath := filepath.Join(root, "mixed.json")

		result := registry.Execute(command.Request{Root: root, Args: []string{
			"fixture", "import", "--ofl", "chauvet-dj/led-par-64-tri-b", "--ofl-file", corpusPath, "--out", outPath,
		}})
		require.Equal(t, 2, result.ExitCode, "expected ExitCode 2, got %d (stdout: %s)", result.ExitCode, result.Stdout)
		require.Contains(t, string(result.Stderr), "GOLC_FIXTURE_USAGE", "expected GOLC_FIXTURE_USAGE on Stderr, got %q", result.Stderr)
	})
}
