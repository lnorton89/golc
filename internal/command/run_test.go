package command

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPrependPathDirectory exercises prependPathDirectory (run.go), the
// pure env-slice helper shared by run.go/dev.go/lint.go/vulncheck.go to get
// a project-local directory in front of a child process's PATH lookup. It
// operates on a `KEY=value` string slice matching os.Environ()'s own shape
// (confirmed against its callers: run.go passes os.Environ() directly, and
// dev.go/lint.go/vulncheck.go build their own "environment" slices in the
// same format via projectGoEnvironment).
func TestPrependPathDirectory(t *testing.T) {
	t.Run("directory not yet present gets prepended onto the existing PATH value", func(t *testing.T) {
		environment := []string{"HOME=/home/user", "PATH=/usr/bin:/bin", "LANG=en_US.UTF-8"}

		result := prependPathDirectory(environment, "/new/dir")

		require.Equal(t, []string{
			"HOME=/home/user",
			"PATH=/new/dir" + string(os.PathListSeparator) + "/usr/bin:/bin",
			"LANG=en_US.UTF-8",
		}, result, "expected /new/dir prepended in place onto the PATH entry, other entries untouched")
	})

	t.Run("directory already present in PATH is moved to the front, not duplicated", func(t *testing.T) {
		environment := []string{"PATH=/existing/dir" + string(os.PathListSeparator) + "/usr/bin"}

		result := prependPathDirectory(environment, "/existing/dir")

		require.Equal(t, []string{
			"PATH=/existing/dir" + string(os.PathListSeparator) + "/usr/bin",
		}, result, "expected /existing/dir moved to the front with no duplicate entry")
	})

	t.Run("directory present in the middle of PATH is removed from there and moved to the front", func(t *testing.T) {
		environment := []string{"PATH=/a" + string(os.PathListSeparator) + "/existing/dir" + string(os.PathListSeparator) + "/b"}

		result := prependPathDirectory(environment, "/existing/dir")

		require.Equal(t, []string{
			"PATH=/existing/dir" + string(os.PathListSeparator) + "/a" + string(os.PathListSeparator) + "/b",
		}, result, "expected /existing/dir removed from the middle and moved to the front, /a and /b order preserved")
	})

	t.Run("directory present with different case is deduplicated case-insensitively", func(t *testing.T) {
		environment := []string{"PATH=C:\\Tools\\Go-Bin" + string(os.PathListSeparator) + "C:\\Windows"}

		result := prependPathDirectory(environment, "C:\\tools\\go-bin")

		require.Equal(t, []string{
			"PATH=C:\\tools\\go-bin" + string(os.PathListSeparator) + "C:\\Windows",
		}, result, "expected the differently-cased existing entry deduplicated, not kept alongside the new one")
	})

	t.Run("case-insensitive match recognizes Windows' Path entry", func(t *testing.T) {
		environment := []string{"Path=C:\\Windows;C:\\Windows\\System32", "USERPROFILE=C:\\Users\\test"}

		result := prependPathDirectory(environment, "C:\\tools\\go-bin")

		require.Equal(t, []string{
			"Path=C:\\tools\\go-bin" + string(os.PathListSeparator) + "C:\\Windows;C:\\Windows\\System32",
			"USERPROFILE=C:\\Users\\test",
		}, result, "expected the differently-cased Path key to be matched and updated in place")
	})

	t.Run("no PATH entry at all appends a new one", func(t *testing.T) {
		environment := []string{"HOME=/home/user", "LANG=en_US.UTF-8"}

		result := prependPathDirectory(environment, "/new/dir")

		require.Equal(t, []string{
			"HOME=/home/user",
			"LANG=en_US.UTF-8",
			"PATH=/new/dir",
		}, result, "expected a new PATH entry appended at the end when none existed")
	})

	t.Run("empty environment slice appends a new PATH entry", func(t *testing.T) {
		result := prependPathDirectory(nil, "/new/dir")

		require.Equal(t, []string{"PATH=/new/dir"}, result)
	})

	t.Run("PATH entry with an empty value still gets dir prepended", func(t *testing.T) {
		environment := []string{"PATH="}

		result := prependPathDirectory(environment, "/new/dir")

		require.Equal(t, []string{
			"PATH=/new/dir" + string(os.PathListSeparator),
		}, result, "expected dir plus separator prepended onto the empty PATH value")
	})

	t.Run("malformed PATH entry without an equals sign is left untouched and a real PATH is appended", func(t *testing.T) {
		// strings.Cut(entry, "=") returns ok=false for an entry with no "=",
		// so it never matches the case-insensitive PATH-name check and is
		// copied through as-is; found stays false, so a fresh PATH=dir entry
		// is appended at the end.
		environment := []string{"PATH", "HOME=/home/user"}

		result := prependPathDirectory(environment, "/new/dir")

		require.Equal(t, []string{
			"PATH",
			"HOME=/home/user",
			"PATH=/new/dir",
		}, result)
	})
}
