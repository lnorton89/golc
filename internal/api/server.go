package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// defaultPort is the fixed default loopback port this plan binds to.
// 07-03 replaces this with the api config concern's resolved value
// (D-06); until then the listener always binds 127.0.0.1, never a
// routable address.
const defaultPort = 4590

// Server is the /v1 REST server: constructed once per daemon process
// (D-07) against a single Executor, repository root, and fixed show
// path. It is designed to satisfy the Art-Net daemon's Subsystem shape
// (Start(ctx) error; Shutdown(ctx) error) structurally, added in this
// phase's next task.
type Server struct {
	executor Executor
	root     string
	showPath string
	addr     string

	router chi.Router
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
