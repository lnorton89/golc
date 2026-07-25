// ratelimit.go implements per-key rate limiting for every /v1 request
// (07-04-PLAN.md Task 2, T-07-06 DoS mitigation): each authenticated
// key's requests draw from its own independent golang.org/x/time/rate
// token bucket (07-RESEARCH.md "Don't Hand-Roll" -- the standard Go
// token-bucket primitive, not a hand-rolled counter+timestamp map), sized
// from the api concern's RatePerMinute/RateBurst (config.go, resolved
// once at Server construction). RateLimitMiddleware must run after
// AuthMiddleware (router.go wires them in that order): it keys off the
// authenticated key's id already attached to the request context, so an
// unauthenticated request never reaches this check at all.
package api

import (
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/time/rate"
)

// defaultRatePerMinute/defaultRateBurst mirror config/api.toml's own
// committed defaults (60 req/min, burst 10) -- used only when a *Server
// is constructed without an explicit WithConfig option, or with a
// Config whose rate fields were left at their zero value, so rate
// limiting is never silently disabled by an unconfigured Server.
const (
	defaultRatePerMinute = 60
	defaultRateBurst     = 10
)

// keyRateLimiter holds one golang.org/x/time/rate.Limiter per
// authenticated key id, created lazily on first use and guarded by a
// mutex: concurrent requests (from the same key or different keys) can
// arrive on independent goroutines, and the underlying map is not safe
// for concurrent access without one.
type keyRateLimiter struct {
	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
	perMinute int
	burst     int
}

// newKeyRateLimiter builds an empty per-key limiter set sized from
// ratePerMinute/burst (server.config's own resolved values); a
// non-positive value falls back to the package's own safe default
// (limit/burstOrDefault below) rather than ever constructing a
// zero-throughput or unbounded limiter.
func newKeyRateLimiter(ratePerMinute, burst int) *keyRateLimiter {
	return &keyRateLimiter{
		limiters:  map[string]*rate.Limiter{},
		perMinute: ratePerMinute,
		burst:     burst,
	}
}

// limit converts k's configured requests-per-minute into the
// requests-per-second rate.Limit golang.org/x/time/rate expects.
func (k *keyRateLimiter) limit() rate.Limit {
	perMinute := k.perMinute
	if perMinute <= 0 {
		perMinute = defaultRatePerMinute
	}
	return rate.Limit(float64(perMinute) / 60.0)
}

// burstOrDefault returns k's configured burst, or the package default
// when it was left at (or configured as) a non-positive value.
func (k *keyRateLimiter) burstOrDefault() int {
	if k.burst <= 0 {
		return defaultRateBurst
	}
	return k.burst
}

// allow reports whether keyID's bucket has a token available right now,
// consuming one if so. Each key id gets its own independent
// *rate.Limiter, created on first use -- a key exhausting its own bucket
// never affects any other key's independent bucket (T-07-06's
// concurrency edge).
func (k *keyRateLimiter) allow(keyID string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	limiter, ok := k.limiters[keyID]
	if !ok {
		limiter = rate.NewLimiter(k.limit(), k.burstOrDefault())
		k.limiters[keyID] = limiter
	}
	return limiter.Allow()
}

// RateLimitMiddleware returns the Huma middleware that enforces
// server's per-key token bucket. It must run after AuthMiddleware in
// router.go's UseMiddleware call: KeyIDFromContext reads the key id
// AuthMiddleware already attached, and a missing key id here (auth
// somehow bypassed) fails closed with the same 401 AuthMiddleware itself
// would produce, rather than silently skipping the rate-limit check. A
// key over its own limit gets 429 without affecting any other key
// (T-07-06).
func RateLimitMiddleware(humaAPI huma.API, server *Server) func(huma.Context, func(huma.Context)) {
	limiter := newKeyRateLimiter(server.config.RatePerMinute, server.config.RateBurst)
	return func(ctx huma.Context, next func(huma.Context)) {
		keyID, ok := KeyIDFromContext(ctx.Context())
		if !ok {
			huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, authFailureMessage)
			return
		}
		if !limiter.allow(keyID) {
			huma.WriteErr(humaAPI, ctx, http.StatusTooManyRequests,
				"GOLC_API_RATE_LIMITED: this key has exceeded its request rate limit")
			return
		}
		next(ctx)
	}
}
