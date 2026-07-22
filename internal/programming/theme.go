// theme.go declares the Theme domain model (CONTEXT PROG-04): a reusable
// named color theme a show author captures once and reuses across scenes.
// Theme copies internal/pool/model.go's identity/construction/rename/
// unique-name shape verbatim (03-PATTERNS.md): a durable UUIDv7 ID minted
// once at creation, never derived from Name, and never re-minted by
// RenameTheme.
package programming

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ColorAssignment associates one fixture instance with a normalized [0,1]
// color-capability value inside a Theme (PROG-04). Colors starts empty on
// a freshly created Theme (mirroring pool.Pool's zero-Members-at-creation
// convention): this plan's "theme create" route only mints the named
// container; populating Colors from resolved instances is a later
// scene/color-theme-layer concern (03-04), not this plan's own record
// path.
type ColorAssignment struct {
	InstanceID uuid.UUID `json:"instance_id"`
	Value      float64   `json:"value"`
}

// Theme is a reusable named color theme (PROG-04). Identity is a durable
// UUIDv7 minted once at creation -- never derived from Name, and never
// re-minted by RenameTheme.
type Theme struct {
	ID     uuid.UUID         `json:"id"`
	Name   string            `json:"name"`
	Colors []ColorAssignment `json:"colors,omitempty"`
}

// NewTheme mints a fresh UUIDv7-identified Theme with zero color
// assignments. IDs are minted only at creation time -- never derived from
// Name, and never re-minted by RenameTheme.
func NewTheme(name string) (Theme, error) {
	if strings.TrimSpace(name) == "" {
		return Theme{}, fmt.Errorf("GOLC_THEME_NAME_EMPTY: theme name must not be empty")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Theme{}, fmt.Errorf("GOLC_THEME_ID_MINT_FAILED: %v", err)
	}
	return Theme{ID: id, Name: name}, nil
}

// RenameTheme returns t with Name replaced by newName; ID is never
// re-minted (identity is rename-stable).
func RenameTheme(t Theme, newName string) (Theme, error) {
	if strings.TrimSpace(newName) == "" {
		return Theme{}, fmt.Errorf("GOLC_THEME_NAME_EMPTY: theme name must not be empty")
	}
	t.Name = newName
	return t, nil
}

// ValidateThemeUniqueNames rejects any two themes in themes sharing the
// same Name: a duplicate name is always rejected with a diagnostic, never
// silently permitted (PROG-04 idempotency).
func ValidateThemeUniqueNames(themes []Theme) error {
	seen := make(map[string]bool, len(themes))
	for _, th := range themes {
		if seen[th.Name] {
			return fmt.Errorf("GOLC_THEME_DUPLICATE_NAME: a theme named %q already exists", th.Name)
		}
		seen[th.Name] = true
	}
	return nil
}
