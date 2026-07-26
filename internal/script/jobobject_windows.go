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
