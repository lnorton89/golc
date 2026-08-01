// capability_test.go covers internal/script/capability.go (08-06-PLAN.md
// Task 1): a table-driven test over every <behavior> bullet, including
// boundary cases exactly at, one below, and one above each threshold,
// and an explicit overflow case for memoryLimitBytes. It is an internal
// (white-box) test package so it can assert directly against Enforce's
// unexported collaborators (requiredScope, scopeRank, runLimiter,
// checkDeadline, memoryLimitBytes, cpuRateFor) and against session.go's
// dispatchCmdCall/Run.beginTermination for the D-11 in-flight-command
// split -- the same reason host_test.go/session_test.go are white-box.
package script

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/show"
)

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err, "uuid.NewV7: %v", err)
	return id
}

// --- Enforce: scope --------------------------------------------------

// TestEnforceScopeHierarchy covers: "Enforce with a profile scoped
// playback and a method whose descriptor requires authoring returns a
// TerminationReason with code GOLC_SCRIPT_SCOPE_DENIED naming both the
// method and the required scope" and "Enforce with a profile scoped
// admin and a method requiring playback allows the call (admin is the
// widest scope...)".
func TestEnforceScopeHierarchy(t *testing.T) {
	tests := []struct {
		name         string
		profileScope show.APIKeyScope
		method       string
		wantAllowed  bool
	}{
		{"authoring profile satisfies its own authoring method", show.APIKeyScopeAuthoring, "scene activate", true},
		{"playback profile denied an authoring method", show.APIKeyScopePlayback, "scene activate", false},
		{"admin profile satisfies a narrower playback method", show.APIKeyScopeAdmin, "show inspect", true},
		{"admin profile satisfies its own admin method", show.APIKeyScopeAdmin, "api-key create", true},
		{"authoring profile denied an admin method", show.APIKeyScopeAuthoring, "api-key create", false},
		{"playback profile satisfies its own playback method", show.APIKeyScopePlayback, "show inspect", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := show.CapabilityProfile{Scope: tt.profileScope, Preset: show.ResourcePresetQuickAction}
			reason := Enforce(profile, mustUUID(t), tt.method, nil)
			if tt.wantAllowed {
				require.Nil(t, reason, "expected the call to be allowed, got termination reason %+v", reason)
				return
			}
			require.NotNil(t, reason, "expected a termination reason for an out-of-scope call")
			require.Equal(t, "GOLC_SCRIPT_SCOPE_DENIED", reason.Code, "Code = %q, want GOLC_SCRIPT_SCOPE_DENIED", reason.Code)
			require.Contains(t, reason.Message, tt.method, "expected the message to name the method %q, got %q", tt.method, reason.Message)
			require.True(t, strings.Contains(reason.Message, "authoring") || strings.Contains(reason.Message, "admin") || strings.Contains(reason.Message, "playback"), "expected the message to name the required scope, got %q", reason.Message)
		})
	}
}

// TestEnforceUnknownMethodFailsClosed covers Enforce's defensive
// "method known" check: a method that is not in
// scriptsdk.RegisteredSDKMethods() is denied rather than silently
// allowed.
func TestEnforceUnknownMethodFailsClosed(t *testing.T) {
	reason := Enforce(show.CapabilityProfile{Scope: show.APIKeyScopeAdmin}, mustUUID(t), "bogus method", nil)
	require.NotNil(t, reason, "expected a termination reason for an unregistered method")
}

// --- runLimiter / Enforce: rate ---------------------------------------

// TestRunLimiterAdmitsExactRateThenDenies covers: "A limiter built for 5
// calls/sec admits 5 calls in a window and returns
// GOLC_SCRIPT_RATE_EXCEEDED on the 6th."
func TestRunLimiterAdmitsExactRateThenDenies(t *testing.T) {
	limiter := newRunLimiter()
	runID := mustUUID(t)
	for i := 0; i < 5; i++ {
		admitted := limiter.allow(runID, 5)
		require.True(t, admitted, "expected call %d/5 to be admitted", i+1)
	}
	denied := limiter.allow(runID, 5)
	require.False(t, denied, "expected the 6th call within the same instant to be denied")
}

// TestRunLimiterViaEnforceRateExceeded covers the same behavior through
// Enforce's own rate-check branch, asserting the exact
// GOLC_SCRIPT_RATE_EXCEEDED code.
func TestRunLimiterViaEnforceRateExceeded(t *testing.T) {
	profile := show.CapabilityProfile{Scope: show.APIKeyScopeAdmin, Preset: show.ResourcePresetAdvanced, RatePerSecond: 5}
	limiter := newRunLimiter()
	runID := mustUUID(t)
	for i := 0; i < 5; i++ {
		reason := Enforce(profile, runID, "show inspect", limiter)
		require.Nil(t, reason, "call %d/5 unexpectedly denied: %+v", i+1, reason)
	}
	reason := Enforce(profile, runID, "show inspect", limiter)
	require.NotNil(t, reason, "expected GOLC_SCRIPT_RATE_EXCEEDED on the 6th call, got %+v", reason)
	require.Equal(t, "GOLC_SCRIPT_RATE_EXCEEDED", reason.Code, "expected GOLC_SCRIPT_RATE_EXCEEDED on the 6th call, got %+v", reason)
}

// TestRunLimiterZeroRatePerSecondUsesPackageDefault covers: "A limiter
// built from a profile whose RatePerSecond is 0 uses the package
// default, not unlimited."
func TestRunLimiterZeroRatePerSecondUsesPackageDefault(t *testing.T) {
	profile := show.CapabilityProfile{Scope: show.APIKeyScopeAdmin, Preset: show.ResourcePresetAdvanced, RatePerSecond: 0}
	limits := resourceLimitsFor(profile)
	require.Greater(t, limits.RatePerSecond, 0, "expected a positive default rate, got %d", limits.RatePerSecond)

	limiter := newRunLimiter()
	runID := mustUUID(t)
	for i := 0; i < limits.RatePerSecond; i++ {
		admitted := limiter.allow(runID, limits.RatePerSecond)
		require.True(t, admitted, "expected call %d/%d to be admitted", i+1, limits.RatePerSecond)
	}
	denied := limiter.allow(runID, limits.RatePerSecond)
	require.False(t, denied, "expected the resolved default rate to be finite, not unlimited")
}

// --- deadlineFor / checkDeadline --------------------------------------

// TestDeadlineBoundary covers: "A run whose elapsed time is one tick
// short of the deadline is not terminated; a run at or past the deadline
// terminates with GOLC_SCRIPT_DEADLINE_EXCEEDED and the elapsed value in
// the reason."
func TestDeadlineBoundary(t *testing.T) {
	deadline := 30 * time.Second

	reason := checkDeadline(deadline-time.Millisecond, deadline)
	require.Nil(t, reason, "expected no termination one tick short of the deadline, got %+v", reason)

	atDeadline := checkDeadline(deadline, deadline)
	require.NotNil(t, atDeadline, "expected termination exactly at the deadline")
	require.Equal(t, "GOLC_SCRIPT_DEADLINE_EXCEEDED", atDeadline.Code, "Code = %q, want GOLC_SCRIPT_DEADLINE_EXCEEDED", atDeadline.Code)
	require.Contains(t, atDeadline.Message, deadline.String(), "expected the message to include the elapsed/deadline value, got %q", atDeadline.Message)

	pastDeadline := checkDeadline(deadline+time.Second, deadline)
	require.NotNil(t, pastDeadline, "expected termination past the deadline")
}

// TestDeadlineForDelegatesToResolveResourceLimits covers: "deadlineFor
// ... delegates to ResolveResourceLimits so the safe-default discipline
// lives in exactly one place", including the "0 or negative
// DeadlineSeconds returns the package default duration" case.
func TestDeadlineForDelegatesToResolveResourceLimits(t *testing.T) {
	longRunning := show.CapabilityProfile{Preset: show.ResourcePresetLongRunning}
	got, want := deadlineFor(longRunning), longRunning.ResolveResourceLimits().Deadline
	require.Equal(t, want, got, "deadlineFor(long-running) = %s, want %s", got, want)

	zeroAdvanced := show.CapabilityProfile{Preset: show.ResourcePresetAdvanced, DeadlineSeconds: 0}
	zeroDeadline := deadlineFor(zeroAdvanced)
	require.Greater(t, zeroDeadline, time.Duration(0), "expected a positive default deadline for DeadlineSeconds 0, got %s", zeroDeadline)

	negativeAdvanced := show.CapabilityProfile{Preset: show.ResourcePresetAdvanced, DeadlineSeconds: -5}
	negativeDeadline := deadlineFor(negativeAdvanced)
	require.Greater(t, negativeDeadline, time.Duration(0), "expected a positive default deadline for a negative DeadlineSeconds, got %s", negativeDeadline)
}

// --- memoryLimitBytes ---------------------------------------------------

func TestMemoryLimitBytesValid(t *testing.T) {
	got, err := memoryLimitBytes(256)
	require.NoError(t, err, "unexpected error: %v", err)
	want := uint64(256) * 1024 * 1024
	require.Equal(t, want, got, "memoryLimitBytes(256) = %d, want %d", got, want)
}

func TestMemoryLimitBytesRejectsNonPositive(t *testing.T) {
	for _, mb := range []int{0, -1, -100} {
		_, err := memoryLimitBytes(mb)
		require.Error(t, err, "expected an error for memory_limit_mb %d", mb)
	}
}

// TestMemoryLimitBytesRejectsOverflow covers the explicit overflow case
// <behavior> requires: a value whose uint64(mb) * 1024 * 1024 would
// exceed math.MaxUint64/2 is rejected before the multiplication runs.
func TestMemoryLimitBytesRejectsOverflow(t *testing.T) {
	_, err := memoryLimitBytes(math.MaxInt64)
	require.Error(t, err, "expected a GOLC_SCRIPT_LIMIT_INVALID overflow error, got %v", err)
	require.Contains(t, err.Error(), "GOLC_SCRIPT_LIMIT_INVALID", "expected a GOLC_SCRIPT_LIMIT_INVALID overflow error, got %v", err)
}

// --- cpuRateFor -----------------------------------------------------

func TestCpuRateForValidRange(t *testing.T) {
	got, err := cpuRateFor(25)
	require.NoError(t, err, "unexpected error: %v", err)
	require.EqualValues(t, 2500, got, "cpuRateFor(25) = %d, want 2500", got)

	full, err := cpuRateFor(100)
	require.NoError(t, err, "cpuRateFor(100) = %d, err %v; want 10000, nil", full, err)
	require.EqualValues(t, 10000, full, "cpuRateFor(100) = %d, err %v; want 10000, nil", full, err)

	min, err := cpuRateFor(1)
	require.NoError(t, err, "cpuRateFor(1) = %d, err %v; want 100, nil", min, err)
	require.EqualValues(t, 100, min, "cpuRateFor(1) = %d, err %v; want 100, nil", min, err)
}

func TestCpuRateForRejectsOutOfRange(t *testing.T) {
	for _, percent := range []int{0, -1, 101, 1000} {
		_, err := cpuRateFor(percent)
		require.Error(t, err, "expected an error for cpu_cap_percent %d", percent)
	}
}

// --- memoryTriggerBytes / checkMemoryPressure ---------------------------

// TestMemoryTriggerBytesResolvesPercentOfConfiguredLimit covers
// memoryTriggerBytes' divide-before-multiply threshold computation and
// its "invalid/absent limit is never treated as a trigger of zero"
// unresolvable case.
func TestMemoryTriggerBytesResolvesPercentOfConfiguredLimit(t *testing.T) {
	trigger, ok := memoryTriggerBytes(64, 95)
	require.True(t, ok, "expected a resolvable trigger for a valid MemoryLimitMB")
	want := (uint64(64) * 1024 * 1024) / 100 * 95
	require.Equal(t, want, trigger, "memoryTriggerBytes(64, 95) = %d, want %d", trigger, want)

	_, ok = memoryTriggerBytes(0, 95)
	require.False(t, ok, "expected memoryTriggerBytes to report unresolvable for an invalid MemoryLimitMB")
}

// TestCheckMemoryPressureBoundary covers checkMemoryPressure's
// <behavior> table: below the trigger is nil, at or above it is a
// populated GOLC_SCRIPT_MEMORY_EXCEEDED reason, and an invalid limit
// never panics or divides by zero.
func TestCheckMemoryPressureBoundary(t *testing.T) {
	limits64 := show.ResolvedLimits{MemoryLimitMB: 64}
	trigger, ok := memoryTriggerBytes(64, memoryPressureTriggerPercent)
	require.True(t, ok, "expected a resolvable 64 MB trigger")

	tests := []struct {
		name      string
		peakBytes uint64
		limits    show.ResolvedLimits
		wantNil   bool
	}{
		{"98.4% of a 64 MB ceiling terminates", 63 * 1024 * 1024, limits64, false},
		{"exactly the 95% trigger terminates", trigger, limits64, false},
		{"50% of a 64 MB ceiling does not terminate", 32 * 1024 * 1024, limits64, true},
		{"zero peak against a 256 MB ceiling does not terminate", 0, show.ResolvedLimits{MemoryLimitMB: 256}, true},
		{"an invalid MemoryLimitMB never panics and never terminates", 1 << 20, show.ResolvedLimits{MemoryLimitMB: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := checkMemoryPressure(tt.peakBytes, tt.limits)
			if tt.wantNil {
				require.Nil(t, reason, "expected nil, got %+v", reason)
				return
			}
			require.NotNil(t, reason, "expected a termination reason")
			require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "Code = %q, want GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code)
		})
	}
}

// TestCheckMemoryPressureRendersExactSentence covers the exact String()
// rendering memoryLimitReason produces -- the text describeTermination
// (ScriptDebugPanel.tsx) parses.
func TestCheckMemoryPressureRendersExactSentence(t *testing.T) {
	reason := checkMemoryPressure(64*1024*1024, show.ResolvedLimits{MemoryLimitMB: 64})
	require.NotNil(t, reason, "expected a termination reason")
	want := "GOLC_SCRIPT_MEMORY_EXCEEDED: run exceeded its 64 MB memory limit"
	require.Equal(t, want, reason.String(), "String() = %q, want %q", reason.String(), want)
}

// --- classifyMemoryExhaustion ---------------------------------------------

// TestClassifyMemoryExhaustionSignatureAndCorroboration covers
// classifyMemoryExhaustion's <behavior> table: every recognized V8/Deno
// OOM signature (case-insensitively), the corroboration floor that
// rejects a signature on an otherwise-healthy heap, and the reverse
// (near-ceiling peak alone never reclassifies an unrelated crash).
func TestClassifyMemoryExhaustionSignatureAndCorroboration(t *testing.T) {
	limits64 := show.ResolvedLimits{MemoryLimitMB: 64}
	limits256 := show.ResolvedLimits{MemoryLimitMB: 256}

	tests := []struct {
		name      string
		reason    string
		peakBytes uint64
		limits    show.ResolvedLimits
		wantNil   bool
	}{
		{"array buffer allocation failed corroborated at 62 of 64 MB", "RangeError: Array buffer allocation failed", 62 * 1024 * 1024, limits64, false},
		{"javascript heap out of memory corroborated at 62 of 64 MB", "JavaScript heap out of memory", 62 * 1024 * 1024, limits64, false},
		{"reached heap limit corroborated at 62 of 64 MB", "Reached heap limit", 62 * 1024 * 1024, limits64, false},
		{"case-insensitive signature match", "RANGEERROR: ARRAY BUFFER ALLOCATION FAILED", 62 * 1024 * 1024, limits64, false},
		{"signature without corroboration is a script bug, not a limit kill", "RangeError: Array buffer allocation failed", 8 * 1024 * 1024, limits256, true},
		{"corroboration without a signature never reclassifies an unrelated crash", "TypeError: cannot read properties of undefined", 63 * 1024 * 1024, limits64, true},
		{"an empty reason never matches", "", 63 * 1024 * 1024, limits64, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := classifyMemoryExhaustion(tt.reason, tt.peakBytes, tt.limits)
			if tt.wantNil {
				require.Nil(t, reason, "expected nil, got %+v", reason)
				return
			}
			require.NotNil(t, reason, "expected a termination reason")
			require.Equal(t, "GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code, "Code = %q, want GOLC_SCRIPT_MEMORY_EXCEEDED", reason.Code)
		})
	}
}

// --- D-11: in-flight command split ------------------------------------

// TestInFlightCallCompletesAfterTerminationBegins covers: "Once a
// TerminationReason is set for a run, every subsequent inbound cmd-call
// is answered with that same reason and never reaches the Executor" and
// "A cmd-call already dispatched to the Executor when termination begins
// runs to completion and its result is recorded in the run outcome."
func TestInFlightCallCompletesAfterTerminationBegins(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	exec := &fakeExecutor{result: func(route string, args []string) (int, []byte, []byte) {
		close(started)
		<-release
		return 0, []byte("GOLC_OK\n"), nil
	}}
	h := &Host{cfg: HostConfig{Root: "/repo", ShowPath: "/repo/show.golc", Executor: exec}}
	run := mustNewRun(t)

	resultCh := make(chan CallOutcome, 1)
	go func() {
		_, outcome := h.dispatchCmdCall(run, CmdCallFrame{ID: "c1", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`)})
		resultCh <- outcome
	}()

	<-started
	began := run.beginTermination(TerminationReason{Code: "GOLC_SCRIPT_DEADLINE_EXCEEDED", Message: "test", At: time.Now()})
	require.True(t, began, "expected beginTermination to record the first reason")
	close(release)

	outcome := <-resultCh
	require.True(t, outcome.Ok, "expected the in-flight call to complete normally despite termination beginning mid-call, got %+v", outcome)

	_, secondOutcome := h.dispatchCmdCall(run, CmdCallFrame{ID: "c2", Method: "scene activate", Params: []byte(`{"name":"Alpha"}`)})
	require.False(t, secondOutcome.Ok, "expected a cmd-call arriving after termination began to be denied without reaching the Executor")
	require.Equal(t, "GOLC_SCRIPT_DEADLINE_EXCEEDED", secondOutcome.Code, "expected the recorded termination reason's code, got %q", secondOutcome.Code)
	require.Len(t, exec.calls, 1, "expected exactly one Execute call (the in-flight one), got %d", len(exec.calls))
}

// TestBeginTerminationFirstWriterWins covers that a second
// beginTermination call never overwrites an already-recorded reason.
func TestBeginTerminationFirstWriterWins(t *testing.T) {
	run := mustNewRun(t)
	first := TerminationReason{Code: "GOLC_SCRIPT_DEADLINE_EXCEEDED", Message: "first", At: time.Now()}
	second := TerminationReason{Code: "GOLC_SCRIPT_RATE_EXCEEDED", Message: "second", At: time.Now()}

	firstSet := run.beginTermination(first)
	require.True(t, firstSet, "expected the first beginTermination to succeed")
	secondSet := run.beginTermination(second)
	require.False(t, secondSet, "expected the second beginTermination to report it did not set the reason")
	reason, terminating := run.terminationReason()
	require.True(t, terminating, "expected the first reason to be retained, got %+v (terminating=%v)", reason, terminating)
	require.Equal(t, first.Code, reason.Code, "expected the first reason to be retained, got %+v (terminating=%v)", reason, terminating)
}
