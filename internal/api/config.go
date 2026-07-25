// config.go resolves the "api" projectconfig concern (07-03-PLAN.md Task 2,
// 07-RESEARCH.md D-06/Pitfall 4) into a typed Config server.go's listener
// derivation reads: RemoteEnabled/Port/BindInterface govern the bind
// address; RatePerMinute/RateBurst are carried through unused by this plan
// for 07-04's rate limiter. internal/api may import internal/projectconfig
// freely -- the package this file must never import is the CLI
// command-execution package (router.go's own doc comment), and
// projectconfig does not import that package either, so no cycle risk
// exists here.
package api

import (
	"fmt"
	"strconv"

	"github.com/lnorton89/golc/internal/projectconfig"
)

// Config is the api concern's resolved bind/rate settings. The zero value
// is never bound directly: NewServer's default (no WithConfig option)
// substitutes defaultPort when Port is zero, so an unconfigured *Server
// still resolves to the safe loopback default rather than binding port 0.
type Config struct {
	// RemoteEnabled mirrors api.remote_enabled: only when true does
	// server.go's listenAddr ever consider binding beyond loopback.
	RemoteEnabled bool
	// Port mirrors api.port.
	Port int
	// BindInterface mirrors api.bind_interface: consulted only when
	// RemoteEnabled is true. An empty value with RemoteEnabled true is a
	// misconfiguration server.go's listenAddr refuses outright
	// (GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED), never silently
	// defaulted or silently widened to 0.0.0.0.
	BindInterface string
	// RatePerMinute mirrors api.rate_per_minute -- unused by this plan,
	// carried through for 07-04's per-key rate limiter.
	RatePerMinute int
	// RateBurst mirrors api.rate_burst -- unused by this plan, carried
	// through for 07-04's per-key rate limiter.
	RateBurst int
}

// apiConfigKeys are the api concern's five canonical keys, resolved
// individually through internal/projectconfig's five-layer resolution
// (committed -> user -> project-local -> environment -> CLI, D-06).
var apiConfigKeys = []string{
	"api.remote_enabled",
	"api.port",
	"api.bind_interface",
	"api.rate_per_minute",
	"api.rate_burst",
}

// ResolveConfig resolves the api concern's five canonical keys against
// root's five configuration layers into a typed Config. The committed and
// project-local layers are already pattern-validated when read
// (internal/projectconfig's strict decoders), so a malformed numeric value
// should never reach this function in production; it still returns an
// error rather than panicking on any resolution or parse failure, so a
// caller (e.g. "artnet serve"'s runArtnetServe) can surface it as a normal,
// diagnosable daemon-start failure instead of a crash.
func ResolveConfig(root string) (Config, error) {
	registry := projectconfig.DefaultRegistry()
	sources := projectconfig.NewSources(root)

	values := map[string]string{}
	for _, key := range apiConfigKeys {
		record, err := projectconfig.ResolveKey(registry, sources, key)
		if err != nil {
			return Config{}, fmt.Errorf("GOLC_API_CONFIG_RESOLVE_FAILED: %s: %v", key, err)
		}
		values[key] = record.Value
	}

	port, err := strconv.Atoi(values["api.port"])
	if err != nil {
		return Config{}, fmt.Errorf("GOLC_API_CONFIG_INVALID: api.port %q is not a valid integer: %v", values["api.port"], err)
	}
	ratePerMinute, err := strconv.Atoi(values["api.rate_per_minute"])
	if err != nil {
		return Config{}, fmt.Errorf("GOLC_API_CONFIG_INVALID: api.rate_per_minute %q is not a valid integer: %v", values["api.rate_per_minute"], err)
	}
	rateBurst, err := strconv.Atoi(values["api.rate_burst"])
	if err != nil {
		return Config{}, fmt.Errorf("GOLC_API_CONFIG_INVALID: api.rate_burst %q is not a valid integer: %v", values["api.rate_burst"], err)
	}

	return Config{
		RemoteEnabled: values["api.remote_enabled"] == "true",
		Port:          port,
		BindInterface: values["api.bind_interface"],
		RatePerMinute: ratePerMinute,
		RateBurst:     rateBurst,
	}, nil
}
