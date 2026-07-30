// capability.go implements D-06/D-08/D-09's host-side capability-scope,
// deadline, and rate enforcement seam (08-06-PLAN.md Task 1, CONTEXT
// SCRP-04/SCRP-06): every SDK call arriving over a script's stdio is
// checked here against the run's own show.CapabilityProfile before it
// ever reaches the Executor -- session.go's dispatchCmdCall wires
// Enforce into 08-05's named "enforce" seam, which always allowed until
// this plan filled it in. Nothing this file checks is ever derived from
// anything the script process itself claims about its own permissions
// (08-RESEARCH.md Pitfall 1): the required scope comes from
// scriptsdk.RegisteredSDKMethods() (the same registry that already
// classifies every route), and the rate/deadline/resource limits come
// from show.CapabilityProfile.ResolveResourceLimits() (show/scripts.go),
// the single place the "a zero/negative/absent limit is never unlimited"
// safe-default discipline lives.
//
// Scope enforcement pattern is copied structurally from
// internal/api/auth.go's HasScope/RequireScope (D-06 reuses the exact
// show.APIKeyScope model), but a script's CapabilityProfile carries
// exactly one Scope value rather than the list an API key's context
// carries -- internal/api/auth.go's HasScope is therefore a list-
// membership check, not a hierarchy, and has no ordering to copy
// directly. This file introduces the minimal ordering CapabilityProfile's
// single-scope shape requires: playback < authoring < admin, so a
// profile scoped admin (the widest scope) satisfies a method that only
// requires playback -- the exact behavior 08-06-PLAN.md Task 1 specifies
// and the only new policy decision this file makes beyond what
// internal/api/auth.go already establishes.
//
// The rate limiter is internal/api/ratelimit.go's keyRateLimiter
// mechanism (golang.org/x/time/rate, lazily created per key under a
// mutex-guarded map) re-keyed by uuid.UUID (RunID) instead of API key id
// -- otherwise the identical mechanism, reused rather than reinvented.
package script

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/lnorton89/golc/internal/scriptsdk"
	"github.com/lnorton89/golc/internal/show"
)

// TerminationReason is the immediate-hard-termination outcome D-08
// requires for a deadline, rate-limit, or resource-limit overrun, and
// (per this plan's explicit extension of D-08, 08-RESEARCH.md Assumption
// A2) for a capability-scope violation too: a machine-readable Code, a
// human-readable Message, and the moment it was recorded.
type TerminationReason struct {
	Code    string
	Message string
	At      time.Time
}

// String renders r as "<code>: <message>" -- the exact text the UI's
// "Terminated: …" copy consumes (D-12), matching the repo-wide
// "{DOMAIN}_{CONDITION}: message" diagnostic convention every other
// GOLC_* error already follows.
func (r TerminationReason) String() string {
	return fmt.Sprintf("%s: %s", r.Code, r.Message)
}

// scopeRank orders show.APIKeyScope from least to most privileged so a
// single-valued CapabilityProfile.Scope can satisfy a method requiring a
// narrower scope (D-06: "admin is the widest scope"). An unrecognized
// scope ranks 0 -- below every real scope -- so an invalid or empty
// Scope value fails closed rather than silently satisfying nothing (or,
// worse, everything).
var scopeRank = map[show.APIKeyScope]int{
	show.APIKeyScopePlayback:  1,
	show.APIKeyScopeAuthoring: 2,
	show.APIKeyScopeAdmin:     3,
}

// requiredScope looks method up in scriptsdk.RegisteredSDKMethods() by
// its Route (session.go's dispatch loop always passes descriptor.Route
// here, per protocol.go's doc comment: CmdCallFrame.Method carries the
// route string, not the TypeScript dot-path name). This is the single
// source of truth for a method's required scope -- capability.go never
// maintains a second table.
func requiredScope(method string) (show.APIKeyScope, bool) {
	for _, descriptor := range scriptsdk.RegisteredSDKMethods() {
		if descriptor.Route == method {
			return descriptor.Scope, true
		}
	}
	return "", false
}

// runLimiter holds one golang.org/x/time/rate.Limiter per active run id,
// created lazily on first use under a mutex -- copied structurally from
// internal/api/ratelimit.go's keyRateLimiter, re-keyed by uuid.UUID.
type runLimiter struct {
	mu       sync.Mutex
	limiters map[uuid.UUID]*rate.Limiter
}

// newRunLimiter returns an empty per-run limiter set.
func newRunLimiter() *runLimiter {
	return &runLimiter{limiters: map[uuid.UUID]*rate.Limiter{}}
}

// allow reports whether runID's bucket has a token available right now,
// consuming one if so. The limiter for a never-before-seen runID is
// created with both its rate and its burst set to ratePerSecond, so a
// profile configured for N calls/sec admits exactly N calls in an
// instantaneous window and denies the N+1th (08-06-PLAN.md Task 1's
// exact <behavior> requirement) -- a larger burst would let more than N
// calls through in one instant, which is not what "N calls/sec" means
// here. ratePerSecond is expected to already be positive (resourceLimitsFor
// delegates the zero/negative/absent-is-never-unlimited discipline to
// show.CapabilityProfile.ResolveResourceLimits, the single place that
// discipline lives); a non-positive value reaching here regardless is
// still never treated as unlimited -- it fails closed to a 1-call/sec
// floor rather than an unbounded rate.Limit(0) burst-0 limiter, which
// would instead deny every call.
func (l *runLimiter) allow(runID uuid.UUID, ratePerSecond int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, ok := l.limiters[runID]
	if !ok {
		n := ratePerSecond
		if n <= 0 {
			n = 1
		}
		limiter = rate.NewLimiter(rate.Limit(n), n)
		l.limiters[runID] = limiter
	}
	return limiter.Allow()
}

// deadlineFor resolves profile's wall-clock deadline, delegating entirely
// to show.CapabilityProfile.ResolveResourceLimits so the safe-default
// discipline (a zero/negative DeadlineSeconds is never "no deadline")
// lives in exactly one place.
func deadlineFor(profile show.CapabilityProfile) time.Duration {
	return resourceLimitsFor(profile).Deadline
}

// resourceLimitsFor resolves profile's concrete, never-unlimited limits.
func resourceLimitsFor(profile show.CapabilityProfile) show.ResolvedLimits {
	return profile.ResolveResourceLimits()
}

// checkDeadline returns a GOLC_SCRIPT_DEADLINE_EXCEEDED TerminationReason
// once elapsed reaches deadline (D-08: hard termination at the boundary
// itself, with no grace period beyond it), or nil while elapsed is still
// strictly less than deadline.
func checkDeadline(elapsed, deadline time.Duration) *TerminationReason {
	if elapsed < deadline {
		return nil
	}
	return &TerminationReason{
		Code:    "GOLC_SCRIPT_DEADLINE_EXCEEDED",
		Message: fmt.Sprintf("run exceeded its %s deadline (elapsed %s)", deadline, elapsed),
		At:      time.Now(),
	}
}

// memoryPressureTriggerPercent is the fraction of the Job Object ceiling
// at which a live run is treated as having exhausted its memory limit
// (D-08's resource cause, checkMemoryPressure's proactive trigger): the
// kernel denies the commit that would cross the ceiling rather than
// killing the process, so a job whose peak committed memory has reached
// 95% of its ceiling is one small allocation away from a denied commit,
// while a well-behaved script under the 256 MB safe default never
// approaches it. A lower trigger would terminate healthy scripts (see
// 08-14-PLAN.md's threat register, T-08G-06).
const memoryPressureTriggerPercent = 95

// memoryExhaustionCorroborationPercent is classifyMemoryExhaustion's
// lower, post-exit corroboration floor -- deliberately lower than
// memoryPressureTriggerPercent because, by the time this is consulted,
// V8 has already reported an allocation failure, so the only job left is
// distinguishing a genuine ceiling collision (peak climbed substantially
// toward the limit) from one absurd single allocation on an otherwise-
// idle heap.
const memoryExhaustionCorroborationPercent = 50

// v8AllocationFailureSignatures are the lowercased V8/Deno out-of-memory
// substrings classifyMemoryExhaustion looks for in an already-redacted
// stderr tail: the exact string 08-13-SUMMARY.md's real repro recorded
// (array buffer allocation failed), the two other well-known V8 OOM
// shapes, and (found empirically during this plan's own real-Deno
// acceptance pass against a genuinely provisioned toolchain, 08-14-
// PLAN.md Task 2) V8's uncatchable "Fatal JavaScript out of memory"
// engine crash -- the same Job Object memory ceiling produces either
// shape nondeterministically depending on which allocation loses the
// race first: an explicit script-level `new ArrayBuffer`/typed-array
// call (the catchable RangeError) or V8's own internal GC trying to
// grow/promote its managed heap (the uncatchable engine-level abort).
// Both are genuine V8-authored OOM signals, never text a script could
// plausibly produce on its own via console output. This list is only
// ever a corroborating signal and never the sole basis for a
// reclassification, because a script controls its own stderr and could
// forge any of these strings (T-08G-02) -- see the peakBytes
// corroboration check in classifyMemoryExhaustion below.
var v8AllocationFailureSignatures = []string{
	"array buffer allocation failed",
	"javascript heap out of memory",
	"reached heap limit",
	"fatal javascript out of memory",
}

// memoryTriggerBytes resolves limits' MB ceiling into the byte threshold
// at percent of that ceiling, delegating the MB->bytes conversion to
// memoryLimitBytes so an invalid or absent limit (mb <= 0, or an
// overflowing conversion) is never treated as a trigger of zero -- it is
// instead reported as unresolvable via the boolean return. The threshold
// is computed as limitBytes/100*percent: divide before multiply so the
// product can never wrap; the truncation this ordering costs is under
// 100 bytes against a minimum 1 MiB limit and is therefore immaterial.
func memoryTriggerBytes(mb int, percent int) (uint64, bool) {
	limitBytes, err := memoryLimitBytes(mb)
	if err != nil {
		return 0, false
	}
	return limitBytes/100*uint64(percent), true
}

// memoryLimitReason is the single constructor both checkMemoryPressure
// and classifyMemoryExhaustion call, so the Code and Message text exist
// in exactly one place: Code GOLC_SCRIPT_MEMORY_EXCEEDED, Message
// "run exceeded its {N} MB memory limit". This Message's shape is a
// rendered-copy contract consumed by describeTermination in
// ScriptDebugPanel.tsx -- changing its wording requires updating that
// parser in the same commit.
func memoryLimitReason(mb int) TerminationReason {
	return TerminationReason{
		Code:    "GOLC_SCRIPT_MEMORY_EXCEEDED",
		Message: fmt.Sprintf("run exceeded its %d MB memory limit", mb),
		At:      time.Now(),
	}
}

// checkMemoryPressure is checkDeadline's sibling for D-08's resource
// cause: given a run's own currently-observed peak Job Object memory
// usage, it returns a GOLC_SCRIPT_MEMORY_EXCEEDED TerminationReason once
// peakBytes reaches memoryPressureTriggerPercent of the configured
// ceiling -- exactly like checkDeadline, the boundary itself triggers
// with no grace period beyond it -- or nil when the limit cannot be
// resolved (memoryTriggerBytes reports unresolvable) or peakBytes is
// still strictly below the trigger.
func checkMemoryPressure(peakBytes uint64, limits show.ResolvedLimits) *TerminationReason {
	trigger, ok := memoryTriggerBytes(limits.MemoryLimitMB, memoryPressureTriggerPercent)
	if !ok || peakBytes < trigger {
		return nil
	}
	reason := memoryLimitReason(limits.MemoryLimitMB)
	return &reason
}

// classifyMemoryExhaustion is the post-exit backstop for D-08's resource
// cause: reason arrives already redacted from runDispatchIO's stderr
// tail (security.Redact having already run there) and this function
// must never redact it again, never store it, and never copy any
// substring of it into the returned reason -- the returned Message is
// host-authored text plus one integer from the run's own profile,
// nothing else. A script fully controls its own stderr and could print
// one of v8AllocationFailureSignatures verbatim to try to have an
// unrelated crash reported as a limit kill, masking a real bug behind a
// benign-looking limit message (T-08G-02) -- the signature match alone
// is therefore never sufficient: peakBytes must also be at or above
// memoryExhaustionCorroborationPercent of the configured ceiling, a
// value the script can only move by genuinely committing memory.
func classifyMemoryExhaustion(reason string, peakBytes uint64, limits show.ResolvedLimits) *TerminationReason {
	lower := strings.ToLower(reason)
	matched := false
	for _, signature := range v8AllocationFailureSignatures {
		if strings.Contains(lower, signature) {
			matched = true
			break
		}
	}
	if !matched {
		return nil
	}

	trigger, ok := memoryTriggerBytes(limits.MemoryLimitMB, memoryExhaustionCorroborationPercent)
	if !ok || peakBytes < trigger {
		return nil
	}

	result := memoryLimitReason(limits.MemoryLimitMB)
	return &result
}

// bytesPerMB is the exact integer multiplier memoryLimitBytes uses --
// pure integer arithmetic throughout, with no non-integer intermediate
// value anywhere in the conversion.
const bytesPerMB = uint64(1024 * 1024)

// maxSafeLimitBytes is memoryLimitBytes' overflow ceiling
// (math.MaxUint64/2): a limit at or below this can never wrap during the
// uint64(mb) * bytesPerMB multiplication this function performs, because
// the pre-multiplication bound check below rejects any mb whose product
// would exceed it before the multiplication ever runs.
const maxSafeLimitBytes = math.MaxUint64 / 2

// memoryLimitBytes converts mb (whole megabytes) to bytes by exact
// uint64 multiplication, rejecting a non-positive mb and any mb whose
// conversion would exceed maxSafeLimitBytes (GOLC_SCRIPT_LIMIT_INVALID)
// -- checked before the multiplication runs, so the multiplication
// itself can never silently wrap.
func memoryLimitBytes(mb int) (uint64, error) {
	if mb <= 0 {
		return 0, fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: memory_limit_mb %d must be positive", mb)
	}
	mbU := uint64(mb)
	if mbU > maxSafeLimitBytes/bytesPerMB {
		return 0, fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: memory_limit_mb %d overflows the byte conversion", mb)
	}
	return mbU * bytesPerMB, nil
}

// cpuPercentToRateUnits converts a whole percent into the Windows Job
// Object CpuRate field's 1/100-of-a-percent unit (100% == 10000).
const cpuPercentToRateUnits = 100

// cpuRateFor converts percent (a whole CPU percentage, 1..100) into the
// Job Object CpuRate field's 1/100-of-a-percent unit by exact integer
// multiplication, rejecting anything outside 1..100
// (GOLC_SCRIPT_LIMIT_INVALID).
func cpuRateFor(percent int) (uint32, error) {
	if percent < 1 || percent > 100 {
		return 0, fmt.Errorf("GOLC_SCRIPT_LIMIT_INVALID: cpu_cap_percent %d must be between 1 and 100", percent)
	}
	return uint32(percent) * cpuPercentToRateUnits, nil
}

// Enforce checks one SDK call against profile, in order: method known →
// scope satisfied → rate admitted. It returns nil when the call is
// permitted, and a populated TerminationReason otherwise -- session.go's
// Host.enforce is the only production caller, always passing a method
// (route) it already resolved via scriptsdk.RegisteredSDKMethods() one
// layer up, so the "method known" branch here is a defensive fail-closed
// floor for any other caller (including this file's own tests) rather
// than the primary unknown-method path (that is
// GOLC_SCRIPT_METHOD_UNKNOWN, session.go's dispatchCmdCall). A
// capability-scope violation and a rate-limit violation are both treated
// with D-08's immediate-hard-termination severity (08-RESEARCH.md
// Assumption A2's explicit planner extension of D-08 to scope
// violations) -- neither is a soft, catchable per-call error the script
// could retry past.
func Enforce(profile show.CapabilityProfile, runID uuid.UUID, method string, limiter *runLimiter) *TerminationReason {
	required, known := requiredScope(method)
	if !known {
		return &TerminationReason{
			Code:    "GOLC_SCRIPT_SCOPE_DENIED",
			Message: fmt.Sprintf("method %q is not a registered SDK method", method),
			At:      time.Now(),
		}
	}

	if scopeRank[profile.Scope] < scopeRank[required] {
		return &TerminationReason{
			Code:    "GOLC_SCRIPT_SCOPE_DENIED",
			Message: fmt.Sprintf("method %q requires scope %q, profile carries %q", method, required, profile.Scope),
			At:      time.Now(),
		}
	}

	if limiter != nil {
		limits := resourceLimitsFor(profile)
		if !limiter.allow(runID, limits.RatePerSecond) {
			return &TerminationReason{
				Code:    "GOLC_SCRIPT_RATE_EXCEEDED",
				Message: fmt.Sprintf("run exceeded its %d call/sec rate limit", limits.RatePerSecond),
				At:      time.Now(),
			}
		}
	}

	return nil
}
