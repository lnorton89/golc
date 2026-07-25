package command

import (
	"strings"
	"testing"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "offline-acceptance",
	Summary: "Real-repository, network-denied route acceptance (golc.ps1 removal Step 7).",
})

// TestScopeOfflineAcceptance is the Go-native replacement for
// tests/acceptance/offline.ps1 -Mode core: unlike TestBuildRouteCompiles
// TheProductionRepository (build_test.go), which already proves the bare
// "build" route against this real repository, nothing previously called
// "test --quick", "generate --check", or "check --offline" directly
// against the real repository -- internal/delivery/delivery_test.go
// exercises LoadGraph/RunOffline only through a fake step executor, never
// through the real self-registered command registry. "-Mode package"
// (foundation ZIP determinism) is not ported here: it is fully redundant
// with internal/delivery/delivery_test.go's "BuildFoundationBundle
// produces byte-identical ZIP, manifest, and checksums across repeated
// runs" and "WriteFoundationBundle writes the ZIP, manifest, and sha256
// sidecar to the fixed output paths" subtests.
func TestScopeOfflineAcceptance(t *testing.T) {
	root := commandParityRepositoryRoot(t)
	if _, err := resolvePinnedGoExecutable(root); err != nil {
		t.Fatalf("pinned Go toolchain not bootstrapped: %v", err)
	}

	t.Run("test --quick passes against the real repository", func(t *testing.T) {
		result := runTestQuick(root)
		if result.ExitCode != 0 {
			t.Fatalf("test --quick exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
		}
	})

	t.Run("generate --check reports zero drift against the real repository", func(t *testing.T) {
		result := runGenerate(Request{Root: root, Args: []string{"--check"}})
		if result.ExitCode != 0 {
			t.Fatalf("generate --check exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
		}
		want := "generate --check: no drift; every committed schema matches its source.\n"
		if string(result.Stdout) != want {
			t.Fatalf("generate --check stdout = %q, want %q", result.Stdout, want)
		}
	})

	t.Run("check --offline completes the whole graph with network denied", func(t *testing.T) {
		result := runCheckOffline(root)
		if result.ExitCode != 0 {
			t.Fatalf("check --offline exited %d\nstdout: %s\nstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
		}
		want := "check --offline: generate, check, build, and test all completed with network denied.\n"
		if !strings.HasSuffix(string(result.Stdout), want) {
			t.Fatalf("check --offline stdout = %q, want it to end with %q", result.Stdout, want)
		}
	})
}
