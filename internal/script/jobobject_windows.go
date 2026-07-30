//go:build windows

// jobobject_windows.go implements the kernel-enforced Windows Job Object
// every spawned Deno child is assigned to immediately after cmd.Start()
// (08-06-PLAN.md Task 2, CONTEXT D-08/D-09/SCRP-06): closing the handle
// kills every process still assigned to it
// (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE) -- an uninterceptable kernel
// action no script-authored `finally` block, unhandled-rejection
// handler, or signal listener can mask or delay. A JOBOBJECT_CPU_RATE_
// CONTROL_INFORMATION carrying the explicit ENABLE|HARD_CAP flag
// combination throttles the child to its configured CPU percentage
// (08-RESEARCH.md Pitfall 3: the weight-based mode silently produces
// fair scheduling instead of a hard ceiling and must never be set here).
//
// golang.org/x/sys/windows exposes CreateJobObject, SetInformationJobObject,
// AssignProcessToJobObject, JOBOBJECT_EXTENDED_LIMIT_INFORMATION, and the
// JOB_OBJECT_LIMIT_*/JobObject*Information constants directly -- but not
// JOBOBJECT_CPU_RATE_CONTROL_INFORMATION or its ControlFlags constants
// (v0.46.0, verified against the module's types_windows.go), so this file
// declares that struct and its two ENABLE/HARD_CAP flag values itself,
// sourced from the Win32 winnt.h header (08-RESEARCH.md Pattern 2;
// 08-PATTERNS.md "No Analog Found": no Job Object precedent exists
// anywhere else in this repository). This is a first-party, auditable
// implementation against the officially maintained x/sys/windows syscalls
// -- no third-party Job Object wrapper is introduced (08-RESEARCH.md's
// Package Legitimacy Audit rejected every candidate for this
// safety-critical role).
package script

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/lnorton89/golc/internal/show"
)

// JOB_OBJECT_CPU_RATE_CONTROL_ENABLE/JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP
// are JOBOBJECT_CPU_RATE_CONTROL_INFORMATION.ControlFlags bit values from
// the Win32 winnt.h header -- not exposed by golang.org/x/sys/windows
// (v0.46.0 declares only the JOBOBJECT_EXTENDED_LIMIT_INFORMATION-family
// JOB_OBJECT_LIMIT_* constants). Named to match the Win32 constant
// exactly, mirroring x/sys/windows' own SCREAMING_SNAKE_CASE convention
// for this class of constant. Only ENABLE|HARD_CAP is ever combined by
// this file: JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP (0x4) is what makes the
// cap an actual ceiling; the weight-based mode (0x2, never set here)
// would silently degrade to fair scheduling instead (08-RESEARCH.md
// Pitfall 3).
const (
	JOB_OBJECT_CPU_RATE_CONTROL_ENABLE   = 0x1
	JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP = 0x4
)

// jobObjectCPURateControlInformation mirrors the Win32
// JOBOBJECT_CPU_RATE_CONTROL_INFORMATION struct's layout for the
// ENABLE|HARD_CAP configuration this file always uses: ControlFlags
// followed by the union's CpuRate member (the union's other members --
// Weight, MinRate/MaxRate -- are never used by this package, so the
// union is represented as the single uint32 field this file writes).
type jobObjectCPURateControlInformation struct {
	ControlFlags uint32
	CpuRate      uint32
}

// jobObject owns one live Windows Job Object handle. Close is idempotent
// -- both run.terminate()'s kill path and Run's own unconditional defer
// may call it, and only the first call may do real work.
type jobObject struct {
	mu     sync.Mutex
	closed bool
	handle windows.Handle
}

// newJobObject creates a fresh Job Object configured with limits'
// resolved memory/CPU caps: JOBOBJECT_EXTENDED_LIMIT_INFORMATION carries
// JOB_OBJECT_LIMIT_JOB_MEMORY + JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, and a
// second SetInformationJobObject call carries a
// jobObjectCPURateControlInformation with the explicit hard-cap flag
// combination (08-RESEARCH.md Pitfall 3). Both limit conversions go
// through capability.go's memoryLimitBytes/cpuRateFor -- the same exact-
// integer, explicitly-bounded conversions Task 1 already tests -- so this
// file never duplicates that arithmetic.
func newJobObject(limits show.ResolvedLimits) (*jobObject, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_CREATE_FAILED: %v", err)
	}
	job := &jobObject{handle: handle}

	memoryBytes, err := memoryLimitBytes(limits.MemoryLimitMB)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	extended := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_JOB_MEMORY | windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
		JobMemoryLimit: uintptr(memoryBytes),
	}
	if _, err := windows.SetInformationJobObject(
		handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&extended)), uint32(unsafe.Sizeof(extended)),
	); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_LIMIT_FAILED: %v", err)
	}

	cpuRate, err := cpuRateFor(limits.CPUCapPercent)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	cpuInfo := jobObjectCPURateControlInformation{
		ControlFlags: JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP,
		CpuRate:      cpuRate,
	}
	if _, err := windows.SetInformationJobObject(
		handle, windows.JobObjectCpuRateControlInformation,
		uintptr(unsafe.Pointer(&cpuInfo)), uint32(unsafe.Sizeof(cpuInfo)),
	); err != nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_LIMIT_FAILED: %v", err)
	}

	return job, nil
}

// queryExtendedLimits reads j's current JOBOBJECT_EXTENDED_LIMIT_
// INFORMATION via a single read-only QueryInformationJobObject call --
// the shared syscall block peakMemoryBytes and jobMemoryLimitBytes both
// read from, so the query is never duplicated. Takes j.mu and refuses to
// query an already-closed handle (j.closed), which is what makes the
// monitor goroutine safe against Run's own `defer job.Close()`: a query
// racing a close either completes first (mu ordering) or observes
// j.closed and errors, but never touches a released handle.
//
// This call is strictly read-only and adds no SetInformationJobObject
// call, so the kernel-enforced memory and CPU configuration newJobObject
// establishes above is untouched. PeakJobMemoryUsed is only maintained
// because JOB_OBJECT_LIMIT_JOB_MEMORY is already set by newJobObject,
// and the value remains readable after the child has exited for as long
// as the handle stays open -- which is what lets Run sample it once more
// after cmd.Wait() returns (classifyMemoryExhaustion's post-exit
// backstop).
//
// Note QueryInformationJobObject's signature returns only err, unlike
// the two SetInformationJobObject calls above (which return
// (ret int, err error)) -- do not copy that two-value arity here.
func (j *jobObject) queryExtendedLimits() (windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if j.closed {
		return info, fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_QUERY_FAILED: job handle already closed")
	}

	err := windows.QueryInformationJobObject(
		j.handle, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil,
	)
	if err != nil {
		return info, fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_QUERY_FAILED: %v", err)
	}
	return info, nil
}

// peakMemoryBytes returns j's assigned process(es)' peak committed
// memory usage, as reported by the kernel's own PeakJobMemoryUsed
// accounting -- the memorySampler seam memorywatch.go's startMemoryWatch
// polls, and the value session.go's post-exit classifier reads once more
// after cmd.Wait() returns. Returns a non-nil error (never touching a
// released handle) once j has been closed.
func (j *jobObject) peakMemoryBytes() (uint64, error) {
	info, err := j.queryExtendedLimits()
	if err != nil {
		return 0, err
	}
	return uint64(info.PeakJobMemoryUsed), nil
}

// jobMemoryLimitBytes returns j's configured JobMemoryLimit -- used only
// by memorylimit_windows_test.go to prove the read-only query above
// round-trips the exact ceiling the two existing SetInformationJobObject
// calls in newJobObject configured.
func (j *jobObject) jobMemoryLimitBytes() (uint64, error) {
	info, err := j.queryExtendedLimits()
	if err != nil {
		return 0, err
	}
	return uint64(info.JobMemoryLimit), nil
}

// assign opens pid with PROCESS_SET_QUOTA|PROCESS_TERMINATE and assigns
// it to j -- session.go calls this immediately after cmd.Start() and
// before the reader goroutines start, so the child cannot outrun
// assignment.
func (j *jobObject) assign(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_ASSIGN_FAILED: open process %d: %v", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.AssignProcessToJobObject(j.handle, handle); err != nil {
		return fmt.Errorf("GOLC_SCRIPT_JOBOBJECT_ASSIGN_FAILED: %v", err)
	}
	return nil
}

// Close closes j's handle, which kills every process still assigned to
// it (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE) -- unconditional, kernel-
// enforced, and never interceptable by anything the child process does.
// Idempotent: a second call is a no-op.
func (j *jobObject) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	return windows.CloseHandle(j.handle)
}
