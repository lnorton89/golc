package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	frontendBuildManifestName          = ".golc-frontend-build-manifest.json"
	frontendBuildManifestSchemaVersion = 1
)

type frontendBuildManifest struct {
	SchemaVersion     int    `json:"schema_version"`
	PackageJSONSHA256 string `json:"package_json_sha256"`
	PackageLockSHA256 string `json:"package_lock_sha256"`
}

// runFrontendBuild provisions the pinned Node toolchain and builds the
// Wails frontend unconditionally (unlike tools/linear-sync's opt-in
// path): cmd/golc-desktop's `//go:embed all:frontend/dist` means every
// default "build"/"test" route needs a real built frontend/dist to
// compile at all, on every platform. This was never wired into
// bootstrap at all before -- frontend/dist is gitignored, never
// committed, and nothing built it, so a clean checkout's cmd/golc-desktop
// simply failed to compile (observed live: cross-platform-mage.yml run
// 30075727901's "check --offline" step, on both ubuntu-latest and
// macos-latest: "pattern all:frontend/dist: no matching files found").
// Since check.yml (the existing Windows PR gate) runs the identical
// build step and had never actually executed even once in its
// Mage-based form (no pull_request had triggered it), this gap was
// latent there too, not specific to any new platform.
//
// Unlike npm ci --ignore-scripts for the small, tightly-scoped
// tools/linear-sync workspace, this runs a normal `npm ci` (lifecycle
// scripts allowed): the frontend's dependency tree is a standard
// React/Vite toolchain more likely to rely on ordinary postinstall
// behavior, and it carries none of the narrow security scoping that
// motivated disabling scripts for the isolated Linear SDK adapter.
func runFrontendBuild(ctx context.Context, engine *bootstrapEngine) (resultErr error) {
	nodePin := engine.nodePin
	nodeInstall := filepath.Join(engine.root, ".tools", "toolchains", "node", nodePin.Version, PlatformKey())
	if err := engine.installPin(nodePin, nodeInstall); err != nil {
		return fmt.Errorf("GOLC_NODE_TOOLCHAIN_INSTALL: %w", err)
	}
	node, err := ResolveNodeInstallation(nodeInstall)
	if err != nil {
		return err
	}

	frontendDir := filepath.Join(engine.root, "frontend")
	packageJSONPath := filepath.Join(frontendDir, "package.json")
	packageLockPath := filepath.Join(frontendDir, "package-lock.json")
	packageJSONBefore, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: frontend/package.json: %w", err)
	}
	packageLockBefore, err := os.ReadFile(packageLockPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: frontend/package-lock.json: %w", err)
	}
	defer func() {
		jsonAfter, jsonErr := os.ReadFile(packageJSONPath)
		lockAfter, lockErr := os.ReadFile(packageLockPath)
		if jsonErr == nil && lockErr == nil && bytes.Equal(packageJSONBefore, jsonAfter) && bytes.Equal(packageLockBefore, lockAfter) {
			return
		}
		restoreErr := writeExactFile(packageJSONPath, packageJSONBefore, 0o644)
		if err := writeExactFile(packageLockPath, packageLockBefore, 0o644); restoreErr == nil {
			restoreErr = err
		}
		if restoreErr != nil {
			resultErr = fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: inputs changed and restoration failed: %w", restoreErr)
			return
		}
		resultErr = fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: bootstrap must never rewrite frontend/package.json or package-lock.json")
	}()

	packageJSONHash := hashBytes(packageJSONBefore)
	packageLockHash := hashBytes(packageLockBefore)
	// frontend/vite.config.ts sets outDir: "../cmd/golc-desktop/frontend/dist"
	// (relative to frontend/), matching cmd/golc-desktop/main.go's
	// `//go:embed all:frontend/dist` directive, which Go resolves relative
	// to that file's own directory -- not frontend/dist under the
	// repository root, which is a plain source directory that never holds
	// a build output at all.
	distIndexPath := filepath.Join(engine.root, "cmd", "golc-desktop", "frontend", "dist", "index.html")
	matches, err := frontendBuildMatches(frontendDir, distIndexPath, packageJSONHash, packageLockHash)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}

	npmCIArgs := []string{node.NPMCLI, "ci", "--no-audit", "--no-fund"}
	if _, err := runLinearProcess(ctx, engine, frontendDir, node.Executable, "GOLC_BOOTSTRAP_FRONTEND_NPM_CI_FAILED", npmCIArgs...); err != nil {
		return err
	}
	npmBuildArgs := []string{node.NPMCLI, "run", "build"}
	if _, err := runLinearProcess(ctx, engine, frontendDir, node.Executable, "GOLC_BOOTSTRAP_FRONTEND_BUILD_FAILED", npmBuildArgs...); err != nil {
		return err
	}
	if info, err := os.Stat(distIndexPath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("GOLC_BOOTSTRAP_FRONTEND_BUILD_FAILED: expected %s after npm run build", distIndexPath)
	}

	if !lockInputsMatch(packageJSONPath, packageLockPath, packageJSONBefore, packageLockBefore) {
		if err := writeExactFile(packageJSONPath, packageJSONBefore, 0o644); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: restore package.json: %w", err)
		}
		if err := writeExactFile(packageLockPath, packageLockBefore, 0o644); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: restore package-lock.json: %w", err)
		}
		return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: bootstrap must never rewrite frontend/package.json or package-lock.json")
	}

	manifest := frontendBuildManifest{
		SchemaVersion:     frontendBuildManifestSchemaVersion,
		PackageJSONSHA256: packageJSONHash,
		PackageLockSHA256: packageLockHash,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_FRONTEND_MANIFEST: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(frontendDir, "node_modules", frontendBuildManifestName), append(manifestBytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_FRONTEND_MANIFEST: %w", err)
	}
	return nil
}

// frontendBuildMatches reports whether frontendDir already holds a build
// produced from the exact current package.json/package-lock.json: the
// recorded manifest's hashes must match and dist/index.html must still
// exist. Unlike tools/linear-sync's fixed, predictable tsc output paths,
// Vite's own output filenames are content-hashed and not enumerable in
// advance, so index.html's presence is the completeness signal instead
// of an exact output file list.
func frontendBuildMatches(frontendDir, distIndexPath, packageJSONHash, packageLockHash string) (bool, error) {
	manifestPath := filepath.Join(frontendDir, "node_modules", frontendBuildManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("GOLC_BOOTSTRAP_FRONTEND_MANIFEST: %w", err)
	}
	var manifest frontendBuildManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return false, nil
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return false, nil
	}
	if manifest.SchemaVersion != frontendBuildManifestSchemaVersion ||
		manifest.PackageJSONSHA256 != packageJSONHash ||
		manifest.PackageLockSHA256 != packageLockHash {
		return false, nil
	}
	info, err := os.Stat(distIndexPath)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	return true, nil
}
