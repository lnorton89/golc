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

import (
	"fmt"
	"net"
	"sync"
)

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
// --inspect-brk=127.0.0.1:<port> argument may only ever be appended here,
// driven purely by mode, never by an environment variable, build tag, or
// "debug build" convention (08-RESEARCH.md Pitfall 2's exact concern).
// debugPort is only ever read when mode == LaunchModeDebug; a LaunchModeRun
// call never inspects it, so an accidentally non-zero debugPort passed
// alongside LaunchModeRun can never leak an inspector argument into a
// plain Run's command line -- the branch on mode is what gates the
// argument, not debugPort's value. No branch of this function ever
// appends a permission-granting flag -- SCRP-03's zero-ambient-access
// guarantee rests on that being permanently true, verified structurally
// by TestDenoCommandLineHasNoAllowFlags rather than by inspection alone.
// The caller (session.go's Run) resolves debugPort once via
// pickEphemeralLoopbackPort before calling this function, so the exact
// same port value is used both for the spawned process's inspector flag
// and for debugbridge.go's later CDP dial -- buildDenoArgs itself never
// picks a port, keeping it a pure function of its three inputs.
func buildDenoArgs(scriptPath string, mode LaunchMode, debugPort int) []string {
	args := []string{"run", "--no-prompt"}
	if mode == LaunchModeDebug {
		// --inspect-brk (not --inspect): the daemon's CDP client must be
		// guaranteed attached and every UI-configured breakpoint set
		// before the first authored line ever executes (08-RESEARCH.md
		// "Debug-mode-only inspector launch" -- --inspect races a short
		// script, --inspect-brk does not).
		args = append(args, fmt.Sprintf("--inspect-brk=127.0.0.1:%d", debugPort))
	}
	args = append(args, scriptPath)
	return args
}

// pickEphemeralLoopbackPort binds 127.0.0.1:0 (letting the OS assign an
// unused ephemeral port), reads the assigned port back, and immediately
// closes the listener -- the standard "reserve then release" pattern for
// handing a caller a concrete port number to pass to a child process's
// own listener a moment later. Binding to 127.0.0.1 specifically (never
// 0.0.0.0 or an unspecified address) is itself part of T-08-40's
// mitigation: the port this function returns is only ever used to build
// the --inspect-brk=127.0.0.1:<port> argument, so the picked port is
// loopback-scoped from the moment it is chosen. Two consecutive calls do
// not collide: each call's listener is bound and closed before the next
// call runs, so the OS is free to (and in practice does) hand out a
// distinct ephemeral port each time.
func pickEphemeralLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("GOLC_SCRIPT_DEBUG_PORT_FAILED: %v", err)
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("GOLC_SCRIPT_DEBUG_PORT_FAILED: unexpected listener address type %T", listener.Addr())
	}
	return addr.Port, nil
}
