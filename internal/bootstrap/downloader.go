// downloader.go is the acquisition half of the D-01/T-01-SC executable-byte
// trust boundary: OfficialSourcePolicy loads the exact committed host/path
// allowlist from config/toolchain.toml, Source abstracts "fetch these
// bytes" so production wires an HTTPS client while tests inject fakes, and
// AcquireStaged/AcquireAndPromote never write or promote anything outside
// a contained staging location. No code in this file consults
// os.Environ(), a proxy, or any host other than the one the caller passed
// after policy approval.
package bootstrap

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// SourcePattern is one committed official host/path-prefix pattern an
// archive URL must match.
type SourcePattern struct {
	Host       string
	PathPrefix string
}

// OfficialSourcePolicy is the exact allowlist of official archive sources
// declared in config/toolchain.toml. Only an https URL matching a
// committed host and path prefix may ever be downloaded.
type OfficialSourcePolicy struct {
	Patterns []SourcePattern
}

// toolchainSourcesDocument mirrors just enough of config/toolchain.toml's
// shape to recover each pinned tool's official_host/official_path_prefix
// pair without depending on internal/projectconfig's strict single-owner
// decoder (a separate package boundary; this file only ever reads the
// committed manifest, never writes it).
type toolchainSourcesDocument struct {
	Toolchain map[string]struct {
		OfficialHost       string `toml:"official_host"`
		OfficialPathPrefix string `toml:"official_path_prefix"`
	} `toml:"toolchain"`
}

// LoadOfficialSourcePolicy reads every pinned tool's official_host/
// official_path_prefix pair from config/toolchain.toml under root and
// returns the resulting allowlist. It fails if the manifest is unreadable
// or declares no official source pin at all — an empty policy would
// silently allow nothing while looking configured.
func LoadOfficialSourcePolicy(root string) (OfficialSourcePolicy, error) {
	path := filepath.Join(root, "config", "toolchain.toml")
	var document toolchainSourcesDocument
	if _, err := toml.DecodeFile(path, &document); err != nil {
		return OfficialSourcePolicy{}, fmt.Errorf("BOOTSTRAP_SOURCE_POLICY_READ: %s: %w", path, err)
	}

	names := make([]string, 0, len(document.Toolchain))
	for name := range document.Toolchain {
		names = append(names, name)
	}
	sort.Strings(names)

	var patterns []SourcePattern
	for _, name := range names {
		entry := document.Toolchain[name]
		if strings.TrimSpace(entry.OfficialHost) == "" || strings.TrimSpace(entry.OfficialPathPrefix) == "" {
			continue
		}
		patterns = append(patterns, SourcePattern{Host: entry.OfficialHost, PathPrefix: entry.OfficialPathPrefix})
	}
	if len(patterns) == 0 {
		return OfficialSourcePolicy{}, fmt.Errorf("BOOTSTRAP_SOURCE_POLICY_EMPTY: %s declares no official_host/official_path_prefix pin", path)
	}
	return OfficialSourcePolicy{Patterns: patterns}, nil
}

// Allows reports whether rawURL is an https URL matching a committed
// official host and path prefix. Anything else — a different scheme, a
// different host, or a path outside the pinned prefix — is rejected
// before any network call is ever made.
func (policy OfficialSourcePolicy) Allows(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("BOOTSTRAP_SOURCE_INVALID_URL: %q: %w", rawURL, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("BOOTSTRAP_SOURCE_SCHEME: %q must use https", rawURL)
	}
	for _, pattern := range policy.Patterns {
		if strings.EqualFold(parsed.Host, pattern.Host) && strings.HasPrefix(parsed.Path, pattern.PathPrefix) {
			return nil
		}
	}
	return fmt.Errorf("BOOTSTRAP_SOURCE_NOT_ALLOWLISTED: %q does not match a committed official host/path pattern", rawURL)
}

// Source fetches archive bytes for an already-allowlisted URL. Production
// wires HTTPSource; every test injects a fake so bootstrap-archive tests
// never perform live network I/O.
type Source interface {
	Fetch(rawURL string) (io.ReadCloser, error)
}

// HTTPSource is the production Source: an HTTPS GET through the standard
// library client. It is never constructed by a test.
type HTTPSource struct {
	Client *http.Client
}

// Fetch performs the HTTPS GET and returns the response body, or an error
// if the transport fails or the response is not 200 OK.
func (source HTTPSource) Fetch(rawURL string) (io.ReadCloser, error) {
	client := source.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("BOOTSTRAP_SOURCE_FETCH: %q: %w", rawURL, err)
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("BOOTSTRAP_SOURCE_HTTP_STATUS: %q: %s", rawURL, response.Status)
	}
	return response.Body, nil
}

// AcquireStaged validates rawURL against policy, fetches it through
// source, and writes the bytes to a fresh file under stagingDir. It never
// verifies the checksum or extracts anything — ExtractVerified and
// PromoteAtomically own that half of the boundary — so a rejected policy
// check never creates a staging file at all.
func AcquireStaged(policy OfficialSourcePolicy, source Source, rawURL, stagingDir string) (archivePath string, err error) {
	if err := policy.Allows(rawURL); err != nil {
		return "", err
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}

	body, err := source.Fetch(rawURL)
	if err != nil {
		return "", err
	}
	defer body.Close()

	staged, err := os.CreateTemp(stagingDir, ".golc-download-*.zip")
	if err != nil {
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", err)
	}
	path := staged.Name()

	if _, copyErr := io.Copy(staged, body); copyErr != nil {
		staged.Close()
		os.Remove(path)
		return "", fmt.Errorf("BOOTSTRAP_SOURCE_FETCH: %q: %w", rawURL, copyErr)
	}
	if closeErr := staged.Close(); closeErr != nil {
		os.Remove(path)
		return "", fmt.Errorf("BOOTSTRAP_STAGING_CREATE: %w", closeErr)
	}
	return path, nil
}

// AcquireAndPromote is the full D-01 acquisition boundary in one call: it
// validates rawURL against policy, downloads through source into a
// contained staging file (AcquireStaged), verifies the exact checksum and
// safe archive structure while extracting into a separate staging
// directory (ExtractVerified), and promotes that directory to installDir
// with a single atomic rename (PromoteAtomically). Any failure before the
// final rename leaves installDir untouched; a corrected retry with the
// same arguments promotes a complete verified tree.
func AcquireAndPromote(policy OfficialSourcePolicy, source Source, rawURL, expectedSHA256, cacheDir, installDir string) error {
	archivePath, err := AcquireStaged(policy, source, rawURL, cacheDir)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	stagingDir, err := ExtractVerified(archivePath, expectedSHA256, filepath.Dir(installDir))
	if err != nil {
		return err
	}
	return PromoteAtomically(stagingDir, installDir)
}
