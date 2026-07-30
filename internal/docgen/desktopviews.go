package docgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	DesktopViewsSource = "frontend/src/shell/desktopViews.json"
	DesktopViewsOutput = "site/src/content/desktop-views.json"
	desktopGenerator   = "github.com/lnorton89/golc/internal/docgen"
)

var desktopSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type DesktopViewsCatalog struct {
	SchemaVersion int                `json:"schemaVersion"`
	GeneratedBy   string             `json:"generatedBy,omitempty"`
	Groups        []DesktopViewGroup `json:"groups"`
	// Onboarding documents the Guided First Show overlay (D-10: never a
	// nav destination), so it is deliberately kept separate from Groups
	// rather than appended as a fifth entry -- Groups stays the exact
	// mirror of the real navigable Show/Build/Operate/Output destinations
	// that frontend/src/shell/AppShell.navigation.test.tsx checks 1:1
	// against NAV_GROUPS.
	Onboarding *DesktopViewGroup `json:"onboarding,omitempty"`
}

type DesktopViewGroup struct {
	Label string        `json:"label"`
	Views []DesktopView `json:"views"`
}

type DesktopView struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	NavLabel       string   `json:"navLabel"`
	Title          string   `json:"title"`
	Purpose        string   `json:"purpose"`
	Actions        []string `json:"actions"`
	Concepts       []string `json:"concepts,omitempty"`
	OperatingNotes []string `json:"operatingNotes,omitempty"`
	Screenshot     string   `json:"screenshot"`
}

func NormalizeDesktopViews(source []byte) ([]byte, error) {
	var catalog DesktopViewsCatalog
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("GOLC_DOCGEN_DESKTOP_DECODE: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	if err := validateDesktopViews(catalog); err != nil {
		return nil, err
	}
	catalog.GeneratedBy = desktopGenerator
	normalized, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("GOLC_DOCGEN_DESKTOP_ENCODE: %v", err)
	}
	return append(normalized, '\n'), nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_DECODE: multiple JSON values")
		}
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_DECODE: %v", err)
	}
	return nil
}

func validateDesktopViews(catalog DesktopViewsCatalog) error {
	if catalog.SchemaVersion != 1 {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_SCHEMA: unsupported schemaVersion %d", catalog.SchemaVersion)
	}
	if catalog.GeneratedBy != "" && catalog.GeneratedBy != desktopGenerator {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_SOURCE: generatedBy must be %q", desktopGenerator)
	}
	if len(catalog.Groups) == 0 {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_REQUIRED: groups must not be empty")
	}
	groupLabels := map[string]bool{}
	ids := map[string]bool{}
	slugs := map[string]bool{}
	screenshots := map[string]bool{}
	for groupIndex, group := range catalog.Groups {
		if groupLabels[group.Label] {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_DUPLICATE: group label %q", group.Label)
		}
		groupLabels[group.Label] = true
		location := fmt.Sprintf("groups[%d]", groupIndex)
		if err := validateDesktopGroup(group, location, strings.ToLower(group.Label)+"-", ids, slugs, screenshots); err != nil {
			return err
		}
	}
	if catalog.Onboarding != nil {
		if err := validateDesktopGroup(*catalog.Onboarding, "onboarding", "guide-", ids, slugs, screenshots); err != nil {
			return err
		}
	}
	return nil
}

// validateDesktopGroup validates one group's label/views and, for each
// view, its required content, id/slug shape and idPrefix, and global
// id/slug/screenshot uniqueness against the shared maps every group (real
// nav group or the singleton Onboarding group) accumulates into.
func validateDesktopGroup(group DesktopViewGroup, location, idPrefix string, ids, slugs, screenshots map[string]bool) error {
	if strings.TrimSpace(group.Label) == "" || len(group.Views) == 0 {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_REQUIRED: %s needs label and views", location)
	}
	for viewIndex, view := range group.Views {
		viewLocation := fmt.Sprintf("%s.views[%d]", location, viewIndex)
		if strings.TrimSpace(view.ID) == "" || strings.TrimSpace(view.Slug) == "" ||
			strings.TrimSpace(view.NavLabel) == "" || strings.TrimSpace(view.Title) == "" ||
			strings.TrimSpace(view.Purpose) == "" || len(view.Actions) == 0 {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_REQUIRED: %s has empty required content", viewLocation)
		}
		for actionIndex, action := range view.Actions {
			if strings.TrimSpace(action) == "" {
				return fmt.Errorf("GOLC_DOCGEN_DESKTOP_REQUIRED: %s.actions[%d] is empty", viewLocation, actionIndex)
			}
		}
		if !desktopSlug.MatchString(view.ID) || !desktopSlug.MatchString(view.Slug) ||
			!strings.HasPrefix(view.ID, idPrefix) {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_REFERENCE: %s id/slug does not match its group", viewLocation)
		}
		if ids[view.ID] || slugs[view.Slug] {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_DUPLICATE: duplicate id %q or slug %q", view.ID, view.Slug)
		}
		ids[view.ID], slugs[view.Slug] = true, true
		expectedScreenshot := "/desktop-views/" + view.ID + ".png"
		if view.Screenshot != expectedScreenshot || filepath.IsAbs(strings.TrimPrefix(view.Screenshot, "/")) ||
			strings.Contains(view.Screenshot, "..") {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_SCREENSHOT: %s must be %q", viewLocation, expectedScreenshot)
		}
		if screenshots[view.Screenshot] {
			return fmt.Errorf("GOLC_DOCGEN_DESKTOP_DUPLICATE: screenshot %q", view.Screenshot)
		}
		screenshots[view.Screenshot] = true
	}
	return nil
}

func generateDesktopViews(root string) error {
	sourcePath := filepath.Join(root, filepath.FromSlash(DesktopViewsSource))
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_READ: %s: %v", DesktopViewsSource, err)
	}
	normalized, err := NormalizeDesktopViews(source)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(root, filepath.FromSlash(DesktopViewsOutput))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_MKDIR: %v", err)
	}
	if err := os.WriteFile(outputPath, normalized, 0o644); err != nil {
		return fmt.Errorf("GOLC_DOCGEN_DESKTOP_WRITE: %v", err)
	}
	return nil
}
