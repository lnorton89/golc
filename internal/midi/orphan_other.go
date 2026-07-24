//go:build !windows

package midi

// KillOrphanedMidicatProcesses is a no-op on non-Windows builds: the
// orphaned-subprocess failure mode this package's orphan.go documents is
// specific to midicatdrv's Windows helper-process model (v1 is
// Windows-only in practice -- see .gitignore's own "architecture stays
// portable even though v1 is Windows-only" note). Kept as a real,
// always-callable function (rather than build-tag-excluding the call
// site in cmd/golc-desktop) so that package needs no platform
// conditionals of its own.
func KillOrphanedMidicatProcesses() []error { return nil }
