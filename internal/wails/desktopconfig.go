// desktopconfig.go holds cmd/golc-desktop/main.go's own Config-resolution
// helpers (CONTEXT: the desktop entrypoint resolves the same project root
// a CLI invocation would, so a supervised "golc-project.exe artnet serve"
// spawn resolves the identical show/config paths a matching CLI invocation
// would). They live here, not in cmd/golc-desktop, because that package
// deliberately carries no test files: midi_driver.go's doc comment
// explains why (a blank import whose init() panics if the pinned
// `midicat` binary is absent, and Go runs every file's init() in a
// package before any test in it executes -- adding a test file there,
// even one exercising unrelated logic, would trigger that panic on any
// machine without midicat on PATH). Extracting this file's two pure
// functions into the already-tested internal/wails package gets them
// real coverage without touching that constraint.
package wails

import (
	"os"
	"path/filepath"
	"strconv"
)

// ResolveProjectRoot prefers the GOLC_PROJECT_ROOT environment variable
// (mirroring cmd/golc-project/main.go's own resolveProjectRoot convention)
// and falls back to the current working directory. Unlike
// cmd/golc-project's version, a resolution failure (os.Getwd or
// filepath.Abs erroring) returns "" rather than an error: the desktop
// entrypoint has no CLI usage-error exit path to report through, and an
// empty ProjectRoot fails obviously and immediately the first time it is
// used to build a path, rather than needing its own distinct error
// handling here. The os.Getwd() failure branch itself is not covered by
// a test: Go offers no portable way to force it to fail from within a
// test process.
func ResolveProjectRoot() string {
	root := os.Getenv("GOLC_PROJECT_ROOT")
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return ""
		}
		root = workingDirectory
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return absolute
}

// EnvInt parses the environment variable name as an int, returning
// fallback when it is unset or does not parse.
func EnvInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
