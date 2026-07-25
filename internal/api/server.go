package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// defaultPort is the fixed default loopback port this plan binds to.
// 07-03 replaces this with the api config concern's resolved value
// (D-06); until then the listener always binds 127.0.0.1, never a
// routable address.
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
	addr     string

	router     chi.Router
	httpServer *http.Server
}

// NewServer builds a *Server wired to every currently self-registered
// operation (RegisterOperation). executor is the sole seam into domain
// logic; root is the repository root every translated invocation
// operates on; showPath is the daemon's own fixed show document path,
// injected server-side into every show-domain call (07-RESEARCH.md
// Pitfall 3) -- no request ever supplies one.
func NewServer(executor Executor, root, showPath string) *Server {
	server := &Server{
		executor: executor,
		root:     root,
		showPath: showPath,
		addr:     fmt.Sprintf("127.0.0.1:%d", defaultPort),
	}
	server.router = buildRouter(server)
	return server
}

// Handler returns server's http.Handler (its Chi router) -- used by tests
// and by anything embedding this server into another listener without
// going through Start.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Start binds s's loopback listener synchronously (so a bind failure --
// e.g. the port already in use -- surfaces immediately as a
// GOLC_API_LISTEN_FAILED error the daemon's own Subsystem-start-failure
// unwind can act on, 07-02-PLAN.md Task 2) and then serves it in a
// background goroutine, mirroring internal/artnet/ipc/server.go's
// ctx-cancellation-driven graceful-shutdown discipline via Shutdown
// rather than blocking Start on the server's entire lifetime.
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
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
