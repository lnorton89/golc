// theme_test.go proves PROG-04's Theme identity/construction/rename/
// duplicate-name contract (03-02-PLAN.md Task 1): NewTheme mints a
// UUIDv7 ID and never derives it from Name; RenameTheme changes Name but
// preserves ID; ValidateThemeUniqueNames rejects two themes sharing a
// name; an empty name is always rejected, never silently accepted.
package programming_test

import (
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/programming"
)

func TestThemePresetNewThemeMintsID(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	if theme.ID.String() == "" {
		t.Fatalf("expected a minted UUIDv7 ID, got zero value")
	}
	if theme.Name != "Sunset" {
		t.Fatalf("expected Name %q, got %q", "Sunset", theme.Name)
	}
	if len(theme.Colors) != 0 {
		t.Fatalf("expected a freshly created theme to have zero color assignments, got %+v", theme.Colors)
	}
}

func TestThemePresetNewThemeEmptyNameRejected(t *testing.T) {
	_, err := programming.NewTheme("   ")
	if err == nil || !strings.Contains(err.Error(), "GOLC_THEME_NAME_EMPTY") {
		t.Fatalf("expected GOLC_THEME_NAME_EMPTY, got %v", err)
	}
}

func TestThemePresetRenamePreservesID(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	originalID := theme.ID

	renamed, err := programming.RenameTheme(theme, "Ocean")
	if err != nil {
		t.Fatalf("RenameTheme: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to be preserved by rename, got original=%s renamed=%s", originalID, renamed.ID)
	}
	if renamed.Name != "Ocean" {
		t.Fatalf("expected renamed Name %q, got %q", "Ocean", renamed.Name)
	}
}

func TestThemePresetRenameThemeEmptyNameRejected(t *testing.T) {
	theme, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	if _, err := programming.RenameTheme(theme, ""); err == nil || !strings.Contains(err.Error(), "GOLC_THEME_NAME_EMPTY") {
		t.Fatalf("expected GOLC_THEME_NAME_EMPTY, got %v", err)
	}
}

func TestThemePresetValidateThemeUniqueNamesRejectsDuplicate(t *testing.T) {
	a, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme(a): %v", err)
	}
	b, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme(b): %v", err)
	}
	err = programming.ValidateThemeUniqueNames([]programming.Theme{a, b})
	if err == nil || !strings.Contains(err.Error(), "GOLC_THEME_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_THEME_DUPLICATE_NAME, got %v", err)
	}
}

func TestThemePresetValidateThemeUniqueNamesAcceptsDistinctNames(t *testing.T) {
	a, err := programming.NewTheme("Sunset")
	if err != nil {
		t.Fatalf("NewTheme(a): %v", err)
	}
	b, err := programming.NewTheme("Ocean")
	if err != nil {
		t.Fatalf("NewTheme(b): %v", err)
	}
	if err := programming.ValidateThemeUniqueNames([]programming.Theme{a, b}); err != nil {
		t.Fatalf("expected distinct names to be accepted, got %v", err)
	}
}
