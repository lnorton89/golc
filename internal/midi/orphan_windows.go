//go:build windows

package midi

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// midicatProcessName is the exact executable filename midicatdrv spawns
// per port (gitlab.com/gomidi/tools/midicat, see driver.go's doc comment
// and cmd/golc-desktop/midi_driver.go's own PATH-lookup note).
const midicatProcessName = "midicat.exe"

// snapshotProcesses lists every currently running process's PID, parent
// PID, and executable filename via a CreateToolhelp32Snapshot walk -- the
// OS-facing half of the orphan check; OrphanedProcessPIDs (orphan.go, pure
// and unit-tested) makes the actual "is this an orphan" decision from this
// data.
func snapshotProcesses() ([]ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	var procs []ProcessInfo
	walkErr := windows.Process32First(snapshot, &entry)
	for walkErr == nil {
		procs = append(procs, ProcessInfo{
			PID:  entry.ProcessID,
			PPID: entry.ParentProcessID,
			Name: windows.UTF16ToString(entry.ExeFile[:]),
		})
		walkErr = windows.Process32Next(snapshot, &entry)
	}
	return procs, nil
}

// killProcess forcibly terminates pid, mirroring `taskkill /F /PID <pid>`.
func killProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("OpenProcess: %w", err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("TerminateProcess: %w", err)
	}
	return nil
}

// KillOrphanedMidicatProcesses finds every running midicat.exe process
// whose parent process has already exited and force-terminates it (see
// this package's orphan.go doc comment for why this class of orphan
// occurs and why it silently breaks MIDI attach). This is a startup-time
// sweep, not a continuous supervisor: only processes that are ALREADY
// orphaned at the moment this runs are cleaned up. Errors are collected
// rather than returned on first failure -- one un-killable stray process
// (e.g. a permissions issue) should never stop the sweep from clearing
// every other genuine orphan.
func KillOrphanedMidicatProcesses() []error {
	procs, err := snapshotProcesses()
	if err != nil {
		return []error{err}
	}
	alive := make(map[uint32]bool, len(procs))
	for _, p := range procs {
		alive[p.PID] = true
	}

	var errs []error
	for _, pid := range OrphanedProcessPIDs(procs, alive, midicatProcessName) {
		if err := killProcess(pid); err != nil {
			errs = append(errs, fmt.Errorf("killing orphaned %s (pid %d): %w", midicatProcessName, pid, err))
		}
	}
	return errs
}
