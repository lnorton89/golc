package midi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOrphanedProcessPIDsFindsOnlyDeadParents proves OrphanedProcessPIDs
// flags a matching-name process only when its parent PID is absent from
// alivePIDs -- the exact condition a force-killed golc-desktop.exe leaves
// behind (its midicat.exe children keep running with a parent PID that no
// longer belongs to any live process).
func TestOrphanedProcessPIDsFindsOnlyDeadParents(t *testing.T) {
	procs := []ProcessInfo{
		{PID: 100, PPID: 1, Name: "explorer.exe"},
		{PID: 200, PPID: 999, Name: "midicat.exe"}, // parent 999 not alive -> orphan
		{PID: 201, PPID: 100, Name: "midicat.exe"}, // parent 100 alive -> not orphan
		{PID: 300, PPID: 999, Name: "notepad.exe"}, // different name, ignored regardless of parent
	}
	alive := map[uint32]bool{100: true, 200: true, 201: true, 300: true} // 999 absent -> dead

	got := OrphanedProcessPIDs(procs, alive, "midicat.exe")
	want := []uint32{200}
	require.Equal(t, want, got)
}

// TestOrphanedProcessPIDsCaseInsensitiveName proves the process-name match
// tolerates whatever case Windows' own ExeFile field happens to report.
func TestOrphanedProcessPIDsCaseInsensitiveName(t *testing.T) {
	procs := []ProcessInfo{{PID: 5, PPID: 999, Name: "MidiCat.EXE"}}
	alive := map[uint32]bool{5: true}

	got := OrphanedProcessPIDs(procs, alive, "midicat.exe")
	require.Equal(t, []uint32{5}, got, "expected a case-insensitive match to find pid 5")
}

// TestOrphanedProcessPIDsIgnoresOtherProcessNames proves an orphaned
// process that merely happens to have a dead parent is never swept up
// just because it's orphaned -- only a name match matters.
func TestOrphanedProcessPIDsIgnoresOtherProcessNames(t *testing.T) {
	procs := []ProcessInfo{{PID: 5, PPID: 999, Name: "notepad.exe"}}
	alive := map[uint32]bool{5: true}

	got := OrphanedProcessPIDs(procs, alive, "midicat.exe")
	require.Empty(t, got, "expected no matches for a differently-named orphaned process")
}

// TestOrphanedProcessPIDsEmptyInputReturnsNil proves an empty process list
// (or one with no matches) returns no orphans rather than panicking or
// returning a non-nil empty slice a caller might mishandle.
func TestOrphanedProcessPIDsEmptyInputReturnsNil(t *testing.T) {
	got := OrphanedProcessPIDs(nil, map[uint32]bool{}, "midicat.exe")
	require.Nil(t, got, "expected nil for empty input")
}
