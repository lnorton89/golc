package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	npmCIManifestName          = ".golc-npm-ci-manifest.json"
	npmCIManifestSchemaVersion = 1
)

var linearSyncExpectedOutputs = []string{
	"dist/src/protocol.js",
	"dist/src/adapter.js",
	"dist/src/cli.js",
	"dist/test/operations.test.js",
}

type npmCIManifest struct {
	SchemaVersion     int      `json:"schema_version"`
	PackageJSONSHA256 string   `json:"package_json_sha256"`
	PackageLockSHA256 string   `json:"package_lock_sha256"`
	TypeScriptPath    string   `json:"typescript_path"`
	Outputs           []string `json:"outputs"`
}

func init() {
	linearSyncBootstrap = runLinearSync
}

func runLinearSync(ctx context.Context, engine *bootstrapEngine) (resultErr error) {
	nodePin, ok := engine.document.Toolchain["node"]
	if !ok {
		return fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.node]")
	}
	nodeInstall := filepath.Join(engine.root, ".tools", "toolchains", "node", nodePin.Version, PlatformKey())
	if err := engine.installPin(nodePin, nodeInstall); err != nil {
		return fmt.Errorf("GOLC_NODE_TOOLCHAIN_INSTALL: %w", err)
	}
	nodeLayout, err := platformArchiveLayout("node", nodePin.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	extractedRoot := filepath.Join(nodeInstall, filepath.FromSlash(nodeLayout.Root))
	nodeExecutable := filepath.Join(extractedRoot, nodeLayout.Executable)
	npmCLI := filepath.Join(extractedRoot, nodeLayout.NPMCLI)
	for label, path := range map[string]string{"node": nodeExecutable, "npm-cli.js": npmCLI} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: expected %s at %s", label, path)
		}
	}

	linearDir := filepath.Join(engine.root, "tools", "linear-sync")
	packageJSONPath := filepath.Join(linearDir, "package.json")
	packageLockPath := filepath.Join(linearDir, "package-lock.json")
	packageJSONBefore, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: tools/linear-sync/package.json: %w", err)
	}
	packageLockBefore, err := os.ReadFile(packageLockPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: tools/linear-sync/package-lock.json: %w", err)
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
		resultErr = fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: bootstrap must never rewrite tools/linear-sync/package.json or package-lock.json")
	}()

	packageJSONHash := hashBytes(packageJSONBefore)
	packageLockHash := hashBytes(packageLockBefore)
	matches, err := npmInstallMatches(linearDir, packageJSONHash, packageLockHash)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}

	npmArgs := []string{npmCLI, "ci", "--ignore-scripts", "--no-audit", "--no-fund"}
	if _, err := runLinearProcess(ctx, engine, linearDir, nodeExecutable, "GOLC_BOOTSTRAP_NPM_CI_FAILED", npmArgs...); err != nil {
		return err
	}
	tscRelative := "node_modules/typescript/bin/tsc"
	tscPath := filepath.Join(linearDir, filepath.FromSlash(tscRelative))
	if info, err := os.Stat(tscPath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED: pinned TypeScript compiler missing at %s after npm ci", tscPath)
	}
	tsconfigPath := filepath.Join(linearDir, "tsconfig.json")
	if _, err := runLinearProcess(ctx, engine, linearDir, nodeExecutable, "GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED", tscPath, "-p", tsconfigPath); err != nil {
		return err
	}
	for _, relative := range linearSyncExpectedOutputs {
		path := filepath.Join(linearDir, filepath.FromSlash(relative))
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("GOLC_BOOTSTRAP_LINEAR_SYNC_BUILD_FAILED: expected compiled %s", path)
		}
	}
	if !lockInputsMatch(packageJSONPath, packageLockPath, packageJSONBefore, packageLockBefore) {
		if err := writeExactFile(packageJSONPath, packageJSONBefore, 0o644); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: restore package.json: %w", err)
		}
		if err := writeExactFile(packageLockPath, packageLockBefore, 0o644); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: restore package-lock.json: %w", err)
		}
		return fmt.Errorf("GOLC_BOOTSTRAP_NODE_LOCK_MUTATION: bootstrap must never rewrite tools/linear-sync/package.json or package-lock.json")
	}
	manifest := npmCIManifest{
		SchemaVersion:     npmCIManifestSchemaVersion,
		PackageJSONSHA256: packageJSONHash,
		PackageLockSHA256: packageLockHash,
		TypeScriptPath:    tscRelative,
		Outputs:           append([]string(nil), linearSyncExpectedOutputs...),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_NPM_MANIFEST: %w", err)
	}
	if err := writeAtomicFile(filepath.Join(linearDir, "node_modules", npmCIManifestName), append(manifestBytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_NPM_MANIFEST: %w", err)
	}
	return nil
}

func runLinearProcess(ctx context.Context, engine *bootstrapEngine, dir, executable, diagnostic string, args ...string) ([]byte, error) {
	output, err := engine.runner.Run(ctx, processRequest{
		Executable: executable,
		Dir:        dir,
		Args:       append([]string(nil), args...),
		Env:        cloneEnvironment(engine.env),
	})
	if err != nil {
		return output, fmt.Errorf("%s: %s: %w", diagnostic, strings.Join(args, " "), err)
	}
	return output, nil
}

func npmInstallMatches(linearDir, packageJSONHash, packageLockHash string) (bool, error) {
	manifestPath := filepath.Join(linearDir, "node_modules", npmCIManifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("GOLC_BOOTSTRAP_NPM_MANIFEST: %w", err)
	}
	var manifest npmCIManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return false, nil
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return false, nil
	}
	if manifest.SchemaVersion != npmCIManifestSchemaVersion ||
		manifest.PackageJSONSHA256 != packageJSONHash ||
		manifest.PackageLockSHA256 != packageLockHash ||
		manifest.TypeScriptPath != "node_modules/typescript/bin/tsc" ||
		len(manifest.Outputs) != len(linearSyncExpectedOutputs) {
		return false, nil
	}
	for index, expected := range linearSyncExpectedOutputs {
		if manifest.Outputs[index] != expected {
			return false, nil
		}
	}
	required := append([]string{manifest.TypeScriptPath}, manifest.Outputs...)
	for _, relative := range required {
		if normalized, err := normalizeArchiveEntryName(relative); err != nil || normalized != relative {
			return false, nil
		}
		info, err := os.Stat(filepath.Join(linearDir, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			return false, nil
		}
	}
	return true, nil
}

func lockInputsMatch(packageJSONPath, packageLockPath string, packageJSONBefore, packageLockBefore []byte) bool {
	jsonAfter, jsonErr := os.ReadFile(packageJSONPath)
	lockAfter, lockErr := os.ReadFile(packageLockPath)
	return jsonErr == nil && lockErr == nil && bytes.Equal(packageJSONBefore, jsonAfter) && bytes.Equal(packageLockBefore, lockAfter)
}

func hashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
