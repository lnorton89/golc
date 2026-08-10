package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDisableWindowsResourceSyso exercises disableWindowsResourceSyso
// (dev.go), the real filesystem logic that keeps the checked-in
// rsrc_windows_amd64.syso from colliding with Wails' own generated
// resource file during `wails dev`. It has no build tag of its own (it's
// plain Go rename/stat logic, only ever invoked at runtime behind a
// `runtime.GOOS == "windows"` check in runDev), so this file needs no
// matching build tag either and runs on every platform.
func TestDisableWindowsResourceSyso(t *testing.T) {
	t.Run("existing syso file is renamed out of the link path, then restored", func(t *testing.T) {
		desktopDir := t.TempDir()
		original := filepath.Join(desktopDir, windowsResourceSyso)
		disabled := original + disabledSysoSuffix
		const content = "fake resource bytes"
		require.NoError(t, os.WriteFile(original, []byte(content), 0o644))

		restore, err := disableWindowsResourceSyso(desktopDir)
		require.NoError(t, err)
		require.NotNil(t, restore)

		// The original filename must no longer exist (so Go's *.syso
		// auto-link rule no longer picks it up), and the disabled-suffix
		// copy must exist with the same bytes.
		_, statErr := os.Stat(original)
		require.True(t, os.IsNotExist(statErr), "expected %s to no longer exist after disabling", original)
		disabledBytes, err := os.ReadFile(disabled)
		require.NoError(t, err)
		require.Equal(t, content, string(disabledBytes))

		require.NoError(t, restore())

		// After restore, the original filename is back with the same
		// content, and the disabled-suffix file is gone.
		restoredBytes, err := os.ReadFile(original)
		require.NoError(t, err)
		require.Equal(t, content, string(restoredBytes))
		_, statErr = os.Stat(disabled)
		require.True(t, os.IsNotExist(statErr), "expected %s to no longer exist after restore", disabled)
	})

	t.Run("leftover disabled file from an interrupted prior run is healed, then re-disabled for this run", func(t *testing.T) {
		desktopDir := t.TempDir()
		original := filepath.Join(desktopDir, windowsResourceSyso)
		disabled := original + disabledSysoSuffix
		const content = "fake resource bytes"
		// Simulate a prior `mage Dev` killed hard enough to skip its own
		// deferred restore: only the disabled-suffix file is left on disk,
		// the original is missing.
		require.NoError(t, os.WriteFile(disabled, []byte(content), 0o644))

		restore, err := disableWindowsResourceSyso(desktopDir)
		require.NoError(t, err)
		require.NotNil(t, restore)

		// Healed (renamed back to original) and immediately re-disabled for
		// this run, in the same call: original still absent, disabled-suffix
		// file still present with the same bytes -- not duplicated or lost.
		_, statErr := os.Stat(original)
		require.True(t, os.IsNotExist(statErr), "expected %s to still be absent after heal-and-redisable", original)
		disabledBytes, err := os.ReadFile(disabled)
		require.NoError(t, err)
		require.Equal(t, content, string(disabledBytes))

		require.NoError(t, restore())

		restoredBytes, err := os.ReadFile(original)
		require.NoError(t, err)
		require.Equal(t, content, string(restoredBytes))
		_, statErr = os.Stat(disabled)
		require.True(t, os.IsNotExist(statErr), "expected %s to no longer exist after restore", disabled)
	})

	t.Run("missing syso file is a safe no-op, not an error", func(t *testing.T) {
		desktopDir := t.TempDir()

		restore, err := disableWindowsResourceSyso(desktopDir)
		require.NoError(t, err)
		require.NotNil(t, restore)

		// Nothing should have been created on disk.
		entries, err := os.ReadDir(desktopDir)
		require.NoError(t, err)
		require.Empty(t, entries, "expected no files created in desktopDir when the syso was already absent")

		// The returned restore func must also be a safe no-op.
		require.NoError(t, restore())
	})
}
