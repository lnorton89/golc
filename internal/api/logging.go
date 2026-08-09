// logging.go is the single package-level slog logger and request-logging
// middleware buildRouter installs (CONTEXT D-10): structured, one-line-
// per-request logs distinct from audit.go's mutation-of-record and
// events.go's SSE stream -- diagnostics only, matching this repository's
// stack research (log/slog, JSON in releases). github.com/go-chi/httplog/v3
// is a thin, slog-native chi middleware (no logging framework of its own)
// rather than a hand-rolled wrapper, since it already solves per-status
// log levels and a Skip predicate correctly.
package api

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/httplog/v3"
)

// logger is the one slog.Logger requestLoggerMiddleware writes every
// request line through.
var logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))

// requestLoggerMiddleware returns the chi middleware buildRouter installs
// alongside RequestID/Recoverer. It never logs the long-lived GET
// apiPathPrefix+"/events" SSE stream: that connection can stay open for a
// client's entire session (EventRevocationTickInterval-scale), so a
// per-"request" line would either never emit (still open) or, once it
// finally closes, read as an hours-long "request duration" -- neither is
// useful, and events.go/audit.go already cover that stream's own
// diagnostics. RecoverPanics stays unset (false): chi's own
// middleware.Recoverer, installed just before this middleware in
// buildRouter, already owns panic recovery -- enabling both would risk a
// double recovery attempt on the same panic.
func requestLoggerMiddleware() func(http.Handler) http.Handler {
	eventsPath := apiPathPrefix + "/events"
	return httplog.RequestLogger(logger, &httplog.Options{
		Level: slog.LevelInfo,
		Skip: func(req *http.Request, _ int) bool {
			return req.URL.Path == eventsPath
		},
	})
}
