// theme_test.go proves PROG-04's Theme identity/construction/rename/
// duplicate-name contract (03-02-PLAN.md Task 1): NewTheme mints a
// UUIDv7 ID and never derives it from Name; RenameTheme changes Name but
// preserves ID; ValidateThemeUniqueNames rejects two themes sharing a
// name; an empty name is always rejected, never silently accepted.
package programming_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/programming"
)

func TestThemePresetNewThemeMintsID(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme")
	require.NotEmpty(t, theme.ID.String(), "expected a minted UUIDv7 ID, got zero value")
	require.Equal(t, "Sunset", theme.Name)
	require.Empty(t, theme.Colors, "expected a freshly created theme to have zero color assignments")
}

func TestThemePresetNewThemeEmptyNameRejected(t *testing.T) {
	_, err := programming.NewTheme("   ")
	require.ErrorContains(t, err, "GOLC_THEME_NAME_EMPTY")
}

func TestThemePresetRenamePreservesID(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme")
	originalID := theme.ID

	renamed, err := programming.RenameTheme(theme, "Ocean")
	require.NoError(t, err, "RenameTheme")
	require.Equal(t, originalID, renamed.ID, "expected ID to be preserved by rename")
	require.Equal(t, "Ocean", renamed.Name)
}

func TestThemePresetRenameThemeEmptyNameRejected(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme")
	_, err = programming.RenameTheme(theme, "")
	require.ErrorContains(t, err, "GOLC_THEME_NAME_EMPTY")
}

func TestThemePresetValidateThemeUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme(a)")
	b, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme(b)")
	err = programming.ValidateThemeUniqueNames([]programming.Theme{a, b})
	require.ErrorContains(t, err, "GOLC_THEME_DUPLICATE_NAME")
}

func TestThemePresetValidateThemeUniqueNamesAcceptsDistinctNames(t *testing.T) {
	a, err := programming.NewTheme("Sunset")
	require.NoError(t, err, "NewTheme(a)")
	b, err := programming.NewTheme("Ocean")
	require.NoError(t, err, "NewTheme(b)")
	require.NoError(t, programming.ValidateThemeUniqueNames([]programming.Theme{a, b}), "expected distinct names to be accepted")
}
