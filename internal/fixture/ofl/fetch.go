// fetch.go implements D-07's OFL live-fetch-plus-cache client and its
// SSRF guard (CONTEXT threat T-02-06): the target URL's scheme and host
// are validated before any request is issued, the request is bounded by a
// fixed timeout, and the response body is bounded by a fixed size cap.
// This is a narrow, occasional, user-triggered GET -- no HTTP client
// library is justified (RESEARCH Standard Stack), only net/http +
// context.
package ofl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OFLRef identifies one Open Fixture Library fixture to fetch: its
// manufacturer and fixture key, mirroring OFL's own
// fixtures/<manufacturer>/<key>.json repository layout, plus an optional
// user-supplied mirror base URL and that mirror's explicit host opt-in
// (CONTEXT threat T-02-06).
type OFLRef struct {
	// Manufacturer is the OFL manufacturer directory key (for example
	// "chauvet-dj").
	Manufacturer string
	// Key is the OFL fixture file key (for example "led-par-64-tri-b").
	Key string
	// Mirror, when non-empty, overrides the default upstream base URL --
	// a user-configured local or alternate mirror (D-07).
	Mirror string
	// AllowMirror is the caller's explicit opt-in for Mirror resolving to
	// a host other than the default upstream host (T-02-06: SSRF guard).
	AllowMirror bool
}

// Source renders ref's canonical "<manufacturer>/<key>" identity, the
// exact shape Normalize's source argument expects.
func (ref OFLRef) Source() string {
	return ref.Manufacturer + "/" + ref.Key
}

const (
	// defaultOFLHost is the host Fetch/FetchManufacturers request without an
	// explicit --allow-mirror opt-in (T-02-06).
	defaultOFLHost = "raw.githubusercontent.com"
	// githubAPIHost is the second SSRF-allowed default host (09-RESEARCH
	// Open Question 1's flagged v1.x follow-up, now implemented): the
	// GitHub REST "get a tree" endpoint FetchFixtureIndex uses to enumerate
	// every fixture key across all manufacturers, which raw-content fetches
	// alone cannot do (raw.githubusercontent.com serves file content, never
	// directory listings). Unlike defaultOFLHost, this host is never
	// resolved from a caller-supplied Mirror in production -- only a test
	// points FixtureIndexRef.Mirror at an httptest server, mirroring
	// ManufacturerIndexRef's identical test-only seam.
	githubAPIHost = "api.github.com"
	// defaultOFLURLPattern is the confirmed raw-JSON-by-key fetch URL
	// (RESEARCH Open Question 2, confirmed against the live upstream
	// repository at plan-execution time): OFL's whole repository,
	// including fixtures/*.json, is a single MIT-licensed GitHub
	// repository (RESEARCH Pitfall 1), so a raw-content fetch of the
	// fixture's own canonical JSON is both the correct and the
	// license-simplest way to import it.
	defaultOFLURLPattern = "https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/%s/%s.json"
	// fetchTimeout bounds every Fetch call's request, regardless of the
	// caller's own context deadline (T-02-06).
	fetchTimeout = 15 * time.Second
	// maxResponseBytes bounds the response body every OFL network call in
	// this package will read: a real OFL fixture JSON document or the
	// manufacturer index is at most a few hundred kilobytes, and the
	// recursive repository tree FetchFixtureIndex reads (~600 fixtures'
	// worth of paths, a few hundred KB today) fits the same generous cap
	// with headroom for catalog growth, while still bounding a hostile or
	// misbehaving mirror (T-02-06).
	maxResponseBytes = 4 * 1024 * 1024
	// userAgent is sent on every OFL network call: harmless for
	// raw.githubusercontent.com, required by api.github.com (which returns
	// 403 without one).
	userAgent = "golc-desktop"
)

// Fetch retrieves ref's OFL fixture JSON: a live GET against the default
// upstream host or an explicitly opted-into mirror (D-07/T-02-06). The
// resolved target URL's scheme and host are validated before any request
// is issued; the request is bounded by fetchTimeout and the response body
// by maxResponseBytes. On success the raw bytes are cached locally,
// content-addressed by their own sha256 (T-02-07), so a later cache
// change from the same upstream content is always a distinct cache entry
// rather than a silent overwrite. Fetch always performs a live request
// when called; it never consults the cache to skip one -- the offline
// entrypoint is "fixture import --ofl-file", which never calls Fetch at
// all (see internal/command/fixture.go).
func Fetch(ctx context.Context, ref OFLRef) ([]byte, error) {
	body, err := getBounded(ctx, resolveTargetURL(ref), defaultOFLHost, ref.AllowMirror)
	if err != nil {
		return nil, err
	}

	// Best-effort local cache write (T-02-07): a cache-directory or
	// write failure never fails an otherwise-successful fetch -- the
	// caller already has valid bytes to normalize and pin.
	_ = cacheOFLBytes(body)

	return body, nil
}

// getBounded performs the SSRF-guarded, bounded GET (T-02-06) shared by
// every OFL network fetch in this package (09-05-PLAN.md Task 3): target's
// scheme and host are validated against defaultHost (validateTargetURL)
// before any request is issued, the request is bounded by fetchTimeout
// regardless of the caller's own context deadline, every redirect hop is
// re-validated the identical way, and the response body is bounded by
// maxResponseBytes with the existing GOLC_FIXTURE_OFL_TOO_LARGE
// diagnostic. Fetch (fixture fetch, defaultOFLHost), FetchManufacturers
// (manufacturer-index fetch, manufacturers.go, defaultOFLHost), and
// FetchFixtureIndex (repository-tree fetch, fixtureindex.go, githubAPIHost)
// all call this one implementation -- the SSRF guard, the timeout, and the
// size cap each have exactly one implementation in this package.
func getBounded(ctx context.Context, target string, defaultHost string, allowMirror bool) ([]byte, error) {
	parsed, err := validateTargetURL(target, defaultHost, allowMirror)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: %v", err)
	}
	request.Header.Set("User-Agent", userAgent)
	if parsed.Hostname() == githubAPIHost {
		request.Header.Set("Accept", "application/vnd.github+json")
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if _, err := validateTargetURL(req.URL.String(), defaultHost, allowMirror); err != nil {
				return fmt.Errorf("GOLC_FIXTURE_OFL_MIRROR_HOST: redirect to %q rejected: %v", req.URL, err)
			}
			return nil
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: unexpected HTTP status %d for %s", response.StatusCode, parsed.String())
	}

	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: %v", err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_TOO_LARGE: response for %s exceeds the %d byte limit", parsed.String(), maxResponseBytes)
	}

	return body, nil
}

// resolveTargetURL builds the URL Fetch requests: ref's own mirror base
// (joined with the same "fixtures/<man>/<key>.json" relative path the
// default upstream host uses) when set, otherwise the default upstream
// raw-GitHub-content URL.
func resolveTargetURL(ref OFLRef) string {
	if ref.Mirror == "" {
		return fmt.Sprintf(defaultOFLURLPattern, ref.Manufacturer, ref.Key)
	}
	base := strings.TrimRight(ref.Mirror, "/")
	return fmt.Sprintf("%s/fixtures/%s/%s.json", base, ref.Manufacturer, ref.Key)
}

// validateTargetURL enforces the SSRF guard (T-02-06) before Fetch issues
// any request: the resolved URL must parse and use an http(s) scheme
// (GOLC_FIXTURE_OFL_MIRROR_SCHEME otherwise), and must either target
// defaultHost (the specific call's own expected host -- defaultOFLHost or
// githubAPIHost, never both at once) or have the caller's explicit
// --allow-mirror opt-in (GOLC_FIXTURE_OFL_MIRROR_HOST otherwise).
func validateTargetURL(raw string, defaultHost string, allowMirror bool) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_MIRROR_SCHEME: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("GOLC_FIXTURE_OFL_MIRROR_SCHEME: scheme %q is not http or https", parsed.Scheme)
	}
	if parsed.Hostname() != defaultHost && !allowMirror {
		return nil, fmt.Errorf(
			"GOLC_FIXTURE_OFL_MIRROR_HOST: host %q is not the default OFL host %q; pass --allow-mirror to opt in",
			parsed.Hostname(), defaultHost)
	}
	return parsed, nil
}

// cacheOFLBytes writes body to the local content-addressed OFL cache
// (T-02-07): "<user cache dir>/golc/ofl/<sha256-hex>.json", keyed only by
// content so a later upstream content change becomes a different cache
// entry rather than a silent overwrite of a previously reviewed one.
func cacheOFLBytes(body []byte) error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: resolving cache directory: %v", err)
	}
	cacheDir := filepath.Join(dir, "golc", "ofl")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: creating cache directory: %v", err)
	}
	sum := sha256.Sum256(body)
	path := filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json")
	if _, err := os.Stat(path); err == nil {
		return nil // already cached under this exact content hash
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("GOLC_FIXTURE_OFL_FETCH_FAILED: writing cache entry: %v", err)
	}
	return nil
}
