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
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/command"
)

const fixtureValidRGBParYAML = `schema_version: 1
manufacturer: Generic
model: RGB PAR
modes:
  - name: Standard
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
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestFixtureValidateRoute(t *testing.T) {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry failed: %v", err)
	}
	root := t.TempDir()

	t.Run("valid fixture exits 0 with a deterministic canonical summary", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "valid.yaml", fixtureValidRGBParYAML)

		first := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		if first.ExitCode != 0 {
			t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", first.ExitCode, first.Stderr)
		}
		if len(first.Stdout) == 0 {
			t.Fatal("expected a non-empty canonical summary on Stdout")
		}

		second := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		if second.ExitCode != 0 {
			t.Fatalf("expected second ExitCode 0, got %d (stderr: %s)", second.ExitCode, second.Stderr)
		}
		if string(first.Stdout) != string(second.Stdout) {
			t.Fatalf("expected byte-identical repeated validation:\nfirst:  %s\nsecond: %s", first.Stdout, second.Stdout)
		}
	})

	t.Run("duplicate-key fixture exits 2 with GOLC_FIXTURE_YAML_INVALID", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "duplicate-key.yaml", fixtureDuplicateKeyYAML)

		result := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "validate", path}})
		if result.ExitCode != 2 {
			t.Fatalf("expected ExitCode 2, got %d (stdout: %s)", result.ExitCode, result.Stdout)
		}
		if !strings.Contains(string(result.Stderr), "GOLC_FIXTURE_YAML_INVALID") {
			t.Fatalf("expected GOLC_FIXTURE_YAML_INVALID on Stderr, got %q", result.Stderr)
		}
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
	if err != nil {
		t.Fatalf("NewDefaultCommandRegistry failed: %v", err)
	}
	root := t.TempDir()

	t.Run("valid fixture exits 0 with a deterministic identity+provenance envelope and no absolute path", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "valid.yaml", fixtureValidRGBParYAML)

		first := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		if first.ExitCode != 0 {
			t.Fatalf("expected ExitCode 0, got %d (stderr: %s)", first.ExitCode, first.Stderr)
		}
		if len(first.Stdout) == 0 {
			t.Fatal("expected a non-empty identity+provenance envelope on Stdout")
		}
		if strings.Contains(string(first.Stdout), root) {
			t.Fatalf("expected no absolute filesystem path (temp root %q) in Stdout, got %q", root, first.Stdout)
		}

		second := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		if second.ExitCode != 0 {
			t.Fatalf("expected second ExitCode 0, got %d (stderr: %s)", second.ExitCode, second.Stderr)
		}
		if string(first.Stdout) != string(second.Stdout) {
			t.Fatalf("expected byte-identical repeated inspect:\nfirst:  %s\nsecond: %s", first.Stdout, second.Stdout)
		}
	})

	t.Run("duplicate-key fixture exits 2 with GOLC_FIXTURE_YAML_INVALID", func(t *testing.T) {
		path := writeFixtureTestFile(t, root, "duplicate-key-inspect.yaml", fixtureDuplicateKeyYAML)

		result := registry.Execute(command.Request{Root: root, Args: []string{"fixture", "inspect", path}})
		if result.ExitCode != 2 {
			t.Fatalf("expected ExitCode 2, got %d (stdout: %s)", result.ExitCode, result.Stdout)
		}
		if !strings.Contains(string(result.Stderr), "GOLC_FIXTURE_YAML_INVALID") {
			t.Fatalf("expected GOLC_FIXTURE_YAML_INVALID on Stderr, got %q", result.Stderr)
		}
	})
}
