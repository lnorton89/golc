// toolchain_test.go covers script.ResolveDenoExecutable (SCRP-03,
// 08-RESEARCH.md): it resolves the pinned, checksum-verified Deno
// executable when a matching install exists, and fails closed with a
// stable GOLC_SCRIPT_DENO_MISSING diagnostic -- never a bare PATH lookup
// -- when it does not.
//
// It is an external test package, mirroring
// internal/bootstrap/scope_registration_test.go's exact rationale:
// internal/script (through internal/bootstrap) sits underneath
// internal/command in the import graph (command -> delivery -> bootstrap),
// so the production script package must never import command, but this
// _test.go binary is a separate compilation unit and may safely import it
// to declare the quick-test scope.
package script_test

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/script"
)

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "script-toolchain",
	Summary: "Deno executable resolution tests for the TypeScript automation sandbox.",
})

// writeDenoManifest writes a minimal, valid config/toolchain.toml pinning
// only [toolchain.deno] for the current platform -- readBootstrapManifest
// and bootstrap.ResolveDenoExecutable never require the go/mage/node pins
// validateManifestForPlatform (a separate, unrelated call path) checks.
func writeDenoManifest(t *testing.T, root, version, archiveURL, archiveSHA256 string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	manifest := fmt.Sprintf(`schema_version = 2

[toolchain.deno]
version = %q
official_host = "github.com"
official_path_prefix = "/denoland/deno/releases/download/"

[toolchain.deno.platforms.%q]
archive_url = %q
archive_sha256 = %q
`, version, bootstrap.PlatformKey(), archiveURL, archiveSHA256)
	if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// buildDenoZip builds a single-entry ZIP archive matching Deno's real
// archive shape (the executable at the archive root, no nested
// directory) and returns its path and lowercase hex SHA-256.
func buildDenoZip(t *testing.T, dir, executableName, body string) (path, digest string) {
	t.Helper()
	path = filepath.Join(dir, "deno-archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: executableName, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write([]byte(body)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(raw)
	return path, hex.EncodeToString(sum[:])
}

// denoArchiveFileName returns the real Deno release asset filename for the
// current platform (config/toolchain.toml's committed pins), so
// selectPlatformPinFor's platform-mismatch check accepts this fixture's
// archive_url exactly like a real bootstrap run would.
func denoArchiveFileName(t *testing.T) string {
	t.Helper()
	switch bootstrap.PlatformKey() {
	case "windows-amd64":
		return "deno-x86_64-pc-windows-msvc.zip"
	case "linux-amd64":
		return "deno-x86_64-unknown-linux-gnu.zip"
	case "linux-arm64":
		return "deno-aarch64-unknown-linux-gnu.zip"
	case "darwin-amd64":
		return "deno-x86_64-apple-darwin.zip"
	case "darwin-arm64":
		return "deno-aarch64-apple-darwin.zip"
	default:
		t.Fatalf("unsupported test platform %q", bootstrap.PlatformKey())
		return ""
	}
}

func TestScopeScriptToolchain(t *testing.T) {
	t.Run("resolves the pinned Deno executable when a verified install exists", func(t *testing.T) {
		root := t.TempDir()
		version := "2.9.4"
		executableName := "deno"
		if bootstrap.PlatformKey() == "windows-amd64" {
			executableName = "deno.exe"
		}
		archivePath, digest := buildDenoZip(t, root, executableName, "deno executable\n")
		archiveURL := "https://github.com/denoland/deno/releases/download/v" + version + "/" + denoArchiveFileName(t)
		writeDenoManifest(t, root, version, archiveURL, digest)

		installDir := filepath.Join(root, ".tools", "toolchains", "deno", version, bootstrap.PlatformKey())
		if err := bootstrap.InstallStaged(archivePath, digest, installDir); err != nil {
			t.Fatalf("InstallStaged: %v", err)
		}

		got, err := script.ResolveDenoExecutable(root)
		if err != nil {
			t.Fatalf("ResolveDenoExecutable: %v", err)
		}
		want := filepath.Join(installDir, executableName)
		if got != want {
			t.Fatalf("ResolveDenoExecutable = %q, want %q", got, want)
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("ResolveDenoExecutable returned a non-absolute path: %q", got)
		}
	})

	t.Run("fails closed with GOLC_SCRIPT_DENO_MISSING when no install exists, never falling back to PATH", func(t *testing.T) {
		root := t.TempDir()
		version := "2.9.4"
		archiveURL := "https://github.com/denoland/deno/releases/download/v" + version + "/deno-fixture.zip"
		// A syntactically valid pin with a plausible-looking checksum, but
		// no install ever staged at the resulting install directory.
		writeDenoManifest(t, root, version, archiveURL, strings.Repeat("a", 64))

		_, err := script.ResolveDenoExecutable(root)
		if err == nil {
			t.Fatal("expected an error when no Deno install exists")
		}
		if !strings.Contains(err.Error(), "GOLC_SCRIPT_DENO_MISSING") {
			t.Fatalf("expected GOLC_SCRIPT_DENO_MISSING, got %v", err)
		}
	})

	t.Run("fails closed with GOLC_SCRIPT_DENO_MISSING when config/toolchain.toml has no [toolchain.deno] pin", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
			t.Fatalf("mkdir config: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "config", "toolchain.toml"), []byte("schema_version = 2\n"), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		_, err := script.ResolveDenoExecutable(root)
		if err == nil || !strings.Contains(err.Error(), "GOLC_SCRIPT_DENO_MISSING") {
			t.Fatalf("expected GOLC_SCRIPT_DENO_MISSING, got %v", err)
		}
	})
}

// TestResolveDenoExecutableMissing is the plan's named acceptance-criteria
// marker (08-02-PLAN.md Task 2): script.ResolveDenoExecutable(root) must
// return an error containing GOLC_SCRIPT_DENO_MISSING when no verified
// install exists at the pinned location, and it must never fall back to a
// bare host-PATH lookup for "deno" -- there is no such lookup anywhere in
// this package for it to fall back to (see the toolchain.go doc comment).
func TestResolveDenoExecutableMissing(t *testing.T) {
	root := t.TempDir()
	version := "2.9.4"
	archiveURL := "https://github.com/denoland/deno/releases/download/v" + version + "/" + denoArchiveFileName(t)
	writeDenoManifest(t, root, version, archiveURL, strings.Repeat("a", 64))

	_, err := script.ResolveDenoExecutable(root)
	if err == nil {
		t.Fatal("expected an error when no Deno install exists")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_DENO_MISSING") {
		t.Fatalf("expected GOLC_SCRIPT_DENO_MISSING, got %v", err)
	}
}
