// orphan.go detects and cleans up orphaned midicat.exe helper processes
// (06-08's midicatdrv driver spawns one such subprocess per port, see
// driver.go's own doc comment) -- a real, reproduced failure mode: a
// golc-desktop.exe instance that is force-killed (Task Manager, a crash,
// or a supervising script's Stop-Process -Force) never runs its own
// graceful OnShutdown -> DetachDriver -> Driver.Close path, so any
// midicat.exe subprocess it spawned is orphaned and keeps holding its MIDI
// port open indefinitely (Windows' MIDI IN API is exclusive-access). Every
// later golc-desktop.exe launch's own MIDI attach then silently fails
// against that still-held port with no diagnostic pointing at why -- it
// just looks like "MIDI Learn never captures anything." KillOrphanedMidicatProcesses
// (orphan_windows.go/orphan_other.go) is a startup-time sweep production
// calls once, before OpenFirstAvailable/AttachDriver, so a stale hold from
// a previous crashed/force-killed session never has to be diagnosed and
// cleared by hand.
//
// This file holds only the pure, portable "which processes are orphans"
// decision (unit-tested directly, no real OS process table involved); the
// actual process-table walk and termination are OS-facing and live in
// orphan_windows.go (real Win32 syscalls) / orphan_other.go (no-op stub
// for non-Windows builds -- v1 is Windows-only in practice).
package midi

import "strings"

// ProcessInfo is one running process's identity as reported by the OS
// process table: PID, parent PID, and executable filename (not a full
// path -- matches CreateToolhelp32Snapshot's own ProcessEntry32.ExeFile
// shape on Windows).
type ProcessInfo struct {
	PID  uint32
	PPID uint32
	Name string
}

// OrphanedProcessPIDs returns the PID of every entry in procs whose Name
// case-insensitively matches processName AND whose PPID is not present in
// alivePIDs -- i.e., every matching process whose parent has already
// exited. alivePIDs is expected to be every PID procs itself reports
// (the full process table snapshot), so "not present" reliably means
// "no longer running," not merely "not a midicat.exe process."
func OrphanedProcessPIDs(procs []ProcessInfo, alivePIDs map[uint32]bool, processName string) []uint32 {
	var orphans []uint32
	for _, p := range procs {
		if !strings.EqualFold(p.Name, processName) {
			continue
		}
		if alivePIDs[p.PPID] {
			continue
		}
		orphans = append(orphans, p.PID)
	}
	return orphans
}
