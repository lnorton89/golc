// host.go declares the zero-permission Deno host (08-05-PLAN.md Task 2,
// CONTEXT SCRP-03): the single call site that composes every Deno
// subprocess command line for a script run, and the Host type session.go's
// Run method operates on. This is the plan the phase's central safety
// claim rests on -- every guarantee about what a script cannot do is
// either enforced here at the OS-process boundary or it is not enforced
// at all.
//
// Adapted from internal/trace/transport/process.go's ProcessConfig
// (pinned-executable, fail-closed existence check via
// ResolveDenoExecutable, explicit never-inherited Env) for Deno instead
// of the Linear adapter's Node process, per 08-PATTERNS.md.
package script

import "sync"

// LaunchMode selects which Deno command line buildDenoArgs composes.
// Only these two values exist: 08-09 adds the debug behaviour behind
// LaunchModeDebug and nothing else may add inspector arguments
// (08-RESEARCH.md Pitfall 2).
type LaunchMode string

// The closed set of launch modes a script Run accepts.
const (
	LaunchModeRun   LaunchMode = "run"
	LaunchModeDebug LaunchMode = "debug"
)

// Executor is the sole seam through which internal/script reaches
// internal/command's real command registry (mirrors internal/api/
// router.go's Executor interface exactly, the same seam shape
// internal/command/artnet.go's apiCommandExecutor already adapts to):
// internal/script never imports internal/command, so no
// command -> script -> command import cycle can ever form. route is the
// exact normalized route key the command registry expects (e.g. "scene
// activate"); args are the built argv-shaped arguments; root is the
// repository root the invocation operates on.
type Executor interface {
	Execute(route string, args []string, root string) (exitCode int, stdout, stderr []byte)
}

// HostConfig configures one Host. ShowPath is the single .golc document
// every script run through this Host touches -- injected server-side
// into every SDK call's argv that needs a --show flag, never trusted
// from the script's own Params (mirrors internal/api/translate.go's
// buildShowArgs discipline: no script-controlled path is ever allowed to
// choose which show document a call mutates or reads).
type HostConfig struct {
	Root     string
	ShowPath string
	Executor Executor
}

// Host owns Deno resolution for one script-hosting context and enforces
// v1's single-active-run restriction (08-05-PLAN.md's "Planner scope
// call", Open Question 1): a second Run request while one is active is
// rejected with GOLC_SCRIPT_RUN_ACTIVE and never queues, pre-empts, or
// silently replaces the active run.
type Host struct {
	cfg      HostConfig
	denoPath string

	mu      sync.Mutex
	running bool

	// limiter is this Host's D-09 per-run rate-limit bucket set
	// (capability.go's Enforce reads it via h.enforce). Always non-nil
	// for a Host constructed through NewHost; a Host built directly via a
	// struct literal (every white-box test in this package) leaves it nil,
	// which Enforce treats as "no rate limiting enforced" -- acceptable
	// for tests exercising dispatch mechanics rather than D-09 itself
	// (covered directly by capability_test.go's own runLimiter tests).
	limiter *runLimiter
}

// NewHost resolves the pinned, checksum-verified Deno executable once
// (script.ResolveDenoExecutable, SCRP-03) and fails closed with
// GOLC_SCRIPT_DENO_MISSING if it is absent -- nothing in this package
// ever falls back to a host PATH lookup, an environment variable, or a
// caller-supplied path.
func NewHost(cfg HostConfig) (*Host, error) {
	denoPath, err := ResolveDenoExecutable(cfg.Root)
	if err != nil {
		return nil, err
	}
	return &Host{cfg: cfg, denoPath: denoPath, limiter: newRunLimiter()}, nil
}

// forbiddenDenoArgPrefixes names every permission-granting Deno CLI flag
// prefix (08-RESEARCH.md Standard Stack / Anti-Patterns: "Granting any
// --allow-* flag 'just in case'" undermines SCRP-03's zero-ambient-access
// claim). TestDenoCommandLineHasNoAllowFlags asserts every produced
// argument, for every launch mode and every capability profile, against
// this exact list -- a forgotten forbidden prefix here would silently
// widen the sandbox rather than fail a test.
var forbiddenDenoArgPrefixes = []string{"--allow-", "-A"}

// buildDenoArgs is the single call site that composes the Deno argument
// list for one script run (08-RESEARCH.md Pitfall 2): the
// --inspect-brk=127.0.0.1:<port> argument 08-09 adds for LaunchModeDebug
// may only ever be appended here, driven by mode, never by an
// environment variable, build tag, or "debug build" convention. This
// plan implements LaunchModeRun's exact, final behavior; LaunchModeDebug
// is intentionally indistinguishable from Run at this call site until
// 08-09 threads the inspector flag through mode here and nowhere else.
// No branch of this function ever appends a permission-granting flag --
// SCRP-03's zero-ambient-access guarantee rests on that being
// permanently true, verified structurally by
// TestDenoCommandLineHasNoAllowFlags rather than by inspection alone.
func buildDenoArgs(scriptPath string, mode LaunchMode) []string {
	_ = mode
	return []string{"run", "--no-prompt", scriptPath}
}
