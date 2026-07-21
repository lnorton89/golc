// cache.go establishes Plan 01-28's project-local Go/Wails cache-layout and
// offline-environment contract: ProjectCacheLayout resolves every cache
// directory bootstrap warms (downloads, Go module cache, Go build cache,
// Go bin, and manifest bookkeeping) rooted strictly inside the repository
// checkout, and OfflineEnvironment derives the exact environment variables
// every subsequent golc.ps1 subcommand must set so Go/Wails operations
// never touch a machine-global cache, bin directory, or toolchain (D-01,
// D-02). WailsModule/WailsVersion pin the exact project-local Wails CLI
// this layout reserves GoBin for; actually invoking an install for that
// pin is deferred beyond Phase 1, which explicitly excludes Wails UI work
// (see 01-01/01-02/01-15/01-16/01-20-PLAN.md), so this file only commits
// to the directory/environment contract a later phase's install step
// consumes without redefining GOBIN, module cache, or build cache
// semantics. Only an explicit `tools update` command may ever change the
// Wails pin (D-04); nothing in this package does.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WailsModule and WailsVersion are the exact project-local Wails CLI pin
// this cache layout reserves GoBin for.
const (
	WailsModule  = "github.com/wailsapp/wails/v2/cmd/wails"
	WailsVersion = "v2.13.0"
)

// ProjectCacheLayout is the complete set of repository-local Go/Wails cache
// directories bootstrap warms and every subsequent build/test/package
// operation must consume, matching the directories golc.ps1 provisions
// during bootstrap: .tools/cache/downloads, .tools/cache/go-mod,
// .tools/cache/go-build, .tools/cache/go-bin, and .tools/manifest.
type ProjectCacheLayout struct {
	Root         string
	Downloads    string
	GoModCache   string
	GoBuildCache string
	GoBin        string
	Manifest     string
}

// NewProjectCacheLayout returns the canonical project-local cache layout
// rooted at root. It fails if root is empty or the resulting layout would
// escape root.
func NewProjectCacheLayout(root string) (ProjectCacheLayout, error) {
	if strings.TrimSpace(root) == "" {
		return ProjectCacheLayout{}, fmt.Errorf("BOOTSTRAP_CACHE_ROOT: root must not be empty")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ProjectCacheLayout{}, fmt.Errorf("BOOTSTRAP_CACHE_ROOT: %w", err)
	}

	layout := ProjectCacheLayout{
		Root:         absoluteRoot,
		Downloads:    filepath.Join(absoluteRoot, ".tools", "cache", "downloads"),
		GoModCache:   filepath.Join(absoluteRoot, ".tools", "cache", "go-mod"),
		GoBuildCache: filepath.Join(absoluteRoot, ".tools", "cache", "go-build"),
		GoBin:        filepath.Join(absoluteRoot, ".tools", "cache", "go-bin"),
		Manifest:     filepath.Join(absoluteRoot, ".tools", "manifest"),
	}
	if err := layout.Validate(); err != nil {
		return ProjectCacheLayout{}, err
	}
	return layout, nil
}

// directories returns every cache directory in layout in a stable order.
func (layout ProjectCacheLayout) directories() []string {
	return []string{layout.Downloads, layout.GoModCache, layout.GoBuildCache, layout.GoBin, layout.Manifest}
}

// Validate confirms every cache directory in layout is contained inside
// Root — a hand-edited or corrupted layout can never resolve outside the
// checkout, matching the same containment discipline archive.go and
// bootstrap.go already enforce for extracted archive entries.
func (layout ProjectCacheLayout) Validate() error {
	if strings.TrimSpace(layout.Root) == "" {
		return fmt.Errorf("BOOTSTRAP_CACHE_ROOT: root must not be empty")
	}
	for _, path := range layout.directories() {
		if path == layout.Root {
			continue
		}
		if !strings.HasPrefix(path, layout.Root+string(os.PathSeparator)) {
			return fmt.Errorf("BOOTSTRAP_CACHE_ESCAPE: %q resolves outside root %q", path, layout.Root)
		}
	}
	return nil
}

// Warm ensures every cache directory in layout exists, creating any that
// are missing via EnsureDirectories. It performs no archive download,
// module fetch, or tool install — only directory provisioning — so calling
// it repeatedly is always a safe no-op once the directories exist, and it
// never removes or overwrites existing directory contents.
func (layout ProjectCacheLayout) Warm() error {
	if err := layout.Validate(); err != nil {
		return err
	}
	return EnsureDirectories(layout.directories()...)
}

// OfflineEnvironment is the exact set of environment variables bootstrap
// and every subsequent golc.ps1 subcommand must set so Go/Wails operations
// stay repository-local: GOTOOLCHAIN is pinned to "local" (never a silent
// toolchain download or host fallback), GOMODCACHE/GOCACHE/GOBIN point
// inside layout, and GOFLAGS forces readonly module resolution so nothing
// outside the explicit `tools update` command rewrites go.mod or go.sum
// (D-04).
type OfflineEnvironment struct {
	GOTOOLCHAIN string
	GOMODCACHE  string
	GOCACHE     string
	GOBIN       string
	GOFLAGS     string
}

// Environment derives the OfflineEnvironment layout requires.
func (layout ProjectCacheLayout) Environment() OfflineEnvironment {
	return OfflineEnvironment{
		GOTOOLCHAIN: "local",
		GOMODCACHE:  layout.GoModCache,
		GOCACHE:     layout.GoBuildCache,
		GOBIN:       layout.GoBin,
		GOFLAGS:     "-mod=readonly",
	}
}

// AsMap returns env as a name->value map suitable for merging into a child
// process environment (for example before invoking the pinned Go
// executable).
func (env OfflineEnvironment) AsMap() map[string]string {
	return map[string]string{
		"GOTOOLCHAIN": env.GOTOOLCHAIN,
		"GOMODCACHE":  env.GOMODCACHE,
		"GOCACHE":     env.GOCACHE,
		"GOBIN":       env.GOBIN,
		"GOFLAGS":     env.GOFLAGS,
	}
}

// WailsBinaryPath is where a project-local Wails CLI install would place
// its executable once a future phase wires WailsModule/WailsVersion into
// an actual install step: inside layout.GoBin, matching `go install`'s own
// GOBIN placement convention exactly. executableSuffix is typically ".exe"
// on Windows or "" elsewhere.
func (layout ProjectCacheLayout) WailsBinaryPath(executableSuffix string) string {
	return filepath.Join(layout.GoBin, "wails"+executableSuffix)
}
