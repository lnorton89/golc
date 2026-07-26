// Package script is the pure-library home for TypeScript automation
// support (SCRP-01..SCRP-06, 08-RESEARCH.md): script lifecycle,
// subprocess supervision, and the generated GOLC SDK plumbing 08-05
// through 08-09 add here. It declares no CLI routes and imports neither
// internal/command nor internal/api's HTTP surface -- every "script *"
// route lives in package command, self-registering the same way
// internal/command/config.go already does for internal/projectconfig
// (STATE.md's recorded precedent). Keeping this boundary explicit here
// matters because internal/command (through internal/delivery) already
// imports internal/bootstrap, so internal/script importing command would
// close an import cycle the moment any command file needed it.
package script

import (
	"fmt"

	"github.com/lnorton89/golc/internal/bootstrap"
)

// ResolveDenoExecutable returns the absolute path to the pinned,
// checksum-verified Deno executable every script run spawns -- the
// process-isolated TypeScript automation sandbox SCRP-03 rests on. It
// delegates entirely to bootstrap.ResolveDenoExecutable and is the single
// call site anything in internal/script (or a later package built on top
// of it) may ever use to obtain that path: nothing here may search the
// host's PATH for an executable, read a DENO_* environment variable, or
// accept a caller-supplied executable path, because any of those would
// let an unpinned binary become the sandbox.
func ResolveDenoExecutable(root string) (string, error) {
	executable, err := bootstrap.ResolveDenoExecutable(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_SCRIPT_DENO_MISSING: %w", err)
	}
	return executable, nil
}
