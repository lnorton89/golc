// logging.go is the single package-level slog logger every internal/wails
// diagnostic funnels through (App.logEvent, MidiService's dispatch-
// failure lines below) -- log/slog is the Go standard library (zero new
// dependency), replacing the plain log.Printf calls this package
// previously used, and emits structured JSON records to stderr so a
// diagnostic line's fields are machine-parseable rather than free text.
package wails

import (
	"log/slog"
	"os"
)

// logger is the one slog.Logger every internal/wails diagnostic line
// writes through. A JSON handler on stderr matches this repository's
// stack research (log/slog, JSON in releases); a colorized dev-mode
// handler can be layered on later without touching any call site, since
// every caller goes through this package-level logger rather than
// constructing its own.
var logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
