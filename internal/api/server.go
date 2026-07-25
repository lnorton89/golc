package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

// defaultPort is the fixed fallback loopback port used when a *Server is
// constructed without an explicit WithConfig option (Config.Port == 0),
// and is also the config-driven default every Config.Port committed value
// currently resolves to (config/api.toml's own "4590", 07-RESEARCH.md Open
// Question 4). listenAddr (below) is what actually derives the bind
// address at Start time (07-03-PLAN.md Task 2, D-06/Pitfall 4).
const defaultPort = 4590

// Server is the /v1 REST server: constructed once per daemon process
// (D-07) against a single Executor, repository root, and fixed show
// path. It satisfies the Art-Net daemon's Subsystem shape
// (Start(ctx) error; Shutdown(ctx) error) structurally -- this package
// never imports that package, so the interface is never referenced by
// name here, only reproduced.
type Server struct {
	executor Executor
	root     string
	showPath string
	config   Config

	router     chi.Router
	httpServer *http.Server
}

// ServerOption configures optional *Server construction settings beyond
// NewServer's three required parameters (functional-option idiom, chosen
// so every existing NewServer call site -- including this package's own
// test files -- keeps compiling unchanged with the safe loopback default
// when it does not need to name explicit bind/rate settings).
type ServerOption func(*Server)

// WithConfig attaches cfg (typically api.ResolveConfig's own return value)
// to the *Server being constructed, so Start derives its listener address
// from the api concern's real resolved settings instead of the built-in
// loopback-only default (07-03-PLAN.md Task 2).
func WithConfig(cfg Config) ServerOption {
	return func(server *Server) {
		server.config = cfg
	}
}

// NewServer builds a *Server wired to every currently self-registered
// operation (RegisterOperation). executor is the sole seam into domain
// logic; root is the repository root every translated invocation
// operates on; showPath is the daemon's own fixed show document path,
// injected server-side into every show-domain call (07-RESEARCH.md
// Pitfall 3) -- no request ever supplies one. Without a WithConfig option,
// the constructed *Server behaves exactly as before this plan: loopback
// on defaultPort, remote access never enabled.
func NewServer(executor Executor, root, showPath string, opts ...ServerOption) *Server {
	server := &Server{
		executor: executor,
		root:     root,
		showPath: showPath,
		config:   Config{Port: defaultPort},
	}
	for _, opt := range opts {
		opt(server)
	}
	server.router = buildRouter(server)
	return server
}

// listenAddr derives s's listener address from its resolved Config
// (07-RESEARCH.md D-06/Pitfall 4): the default is always loopback,
// regardless of BindInterface, unless RemoteEnabled is explicitly true --
// and even then, an empty BindInterface is refused outright (never
// silently defaulted back to loopback, never silently widened to
// 0.0.0.0). A zero Port falls back to defaultPort so a *Server built
// without WithConfig (or against a Config whose Port was never set)
// still binds a real port.
func (s *Server) listenAddr() (string, error) {
	port := s.config.Port
	if port == 0 {
		port = defaultPort
	}
	if !s.config.RemoteEnabled {
		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}
	bindInterface := strings.TrimSpace(s.config.BindInterface)
	if bindInterface == "" {
		return "", fmt.Errorf(
			"GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED: api.remote_enabled is true but api.bind_interface is empty; " +
				"an operator enabling remote access must name an explicit interface")
	}
	return fmt.Sprintf("%s:%d", bindInterface, port), nil
}

// Handler returns server's http.Handler (its Chi router) -- used by tests
// and by anything embedding this server into another listener without
// going through Start.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Start binds s's listener synchronously -- its address derived from
// listenAddr's loopback-enforced default (so a bind failure, e.g. the
// port already in use, surfaces immediately as a GOLC_API_LISTEN_FAILED
// error the daemon's own Subsystem-start-failure unwind can act on,
// 07-02-PLAN.md Task 2; a RemoteEnabled misconfiguration surfaces just as
// immediately as GOLC_API_REMOTE_BIND_INTERFACE_REQUIRED, before any
// socket is ever opened) -- and then serves it in a background goroutine,
// mirroring internal/artnet/ipc/server.go's ctx-cancellation-driven
// graceful-shutdown discipline via Shutdown rather than blocking Start on
// the server's entire lifetime.
func (s *Server) Start(ctx context.Context) error {
	addr, err := s.listenAddr()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("GOLC_API_LISTEN_FAILED: %v", err)
	}
	s.httpServer = &http.Server{Handler: s.router}
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Start has already returned by the time a post-bind Serve
			// failure could occur, so it cannot be reported through a
			// return value; this isolated background goroutine must
			// never crash the daemon process that also owns
			// deterministic playback and Art-Net output (ARTN-04), so
			// the failure is only ever surfaced as a diagnostic here.
			fmt.Fprintf(os.Stderr, "GOLC_API_SERVE_FAILED: %v\n", serveErr)
		}
	}()
	return nil
}

// Shutdown stops accepting new connections and lets in-flight requests
// drain within ctx's deadline (mirrors internal/artnet/ipc/server.go's
// own graceful-shutdown discipline, adapted for net/http). A Shutdown
// call before Start is a no-op, never an error.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
