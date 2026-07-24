// tools.go is the tools command file: it owns the "tools" scope and
// self-registers the exact "tools update" route through the package
// declaration entrypoints (CONTEXT D-03/D-04) — the central router is
// never edited.
//
// Route registration follows the same dash-word precedent check.go/
// generate.go document: MustDeclareRoute's route-word grammar
// (routeWordPattern) never accepts a "--"-prefixed word, so a single
// exact route, "tools update", is declared here; its handler strictly
// accepts exactly one of "--check" or "--write" and rejects anything
// else. The user-facing commands "tools update --check" and
// "tools update --write" are both exact and reachable through this one
// registration, exactly mirroring test.go's "test --quick --scope <name>"
// and build.go's "build --scope <name>" dispatch shape.
//
// "tools update --check" computes and prints a deterministic proposal for
// the five declared reviewable authorities (config/toolchain.toml's
// [toolchain.go]/[toolchain.node] pins; go.mod/go.sum's one managed Go
// module pin; tools/linear-sync/package.json and package-lock.json's
// @linear/sdk and typescript pins) without writing anything.
// "tools update --write" writes exactly those five files with the
// reviewed proposal's exact bytes, using a fixed five-path allowlist that
// is never driven by external input. Neither mode ever downloads an
// archive, extracts a zip, warms a cache, runs a package manager install,
// builds a dependency tree, or compiles anything: this file never imports
// any process-execution or archive-extraction package, nor the bootstrap
// package's install/verify machinery (T-01-14/T-01-SC); both modes only
// read existing file bytes, consult an injected MetadataSource for
// candidate pin data, and (write only) write the five declared files.
// tools_test.go's static-source guard subtest enforces this structurally,
// not just by convention.
//
// Live remote metadata polling (a real go.dev/nodejs.org/npm-registry
// MetadataSource) is out of this plan's scope — see defaultMetadataSource
// below and 01-29-SUMMARY.md's Known Stubs. This file proves the
// deterministic check/write/allowlist/no-install contract any future real
// MetadataSource implementation must satisfy without changing this
// contract; only tools_test.go's fakeMetadataSource exercises an actual
// change proposal today.
package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "tools",
	Summary: "Explicit, reviewable tool/dependency update proposals -- never bootstrap.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "tools update",
	Summary: "Propose or write deterministic toolchain/Go/npm updates: tools update --check|--write.",
	Handler: runToolsUpdate,
})

// toolsUpdateAllowlist is the exact, fixed five repository-relative paths
// "tools update --write" may ever create or modify (T-01-14): the
// toolchain pin manifest, the Go module manifest/lock, and the
// tools/linear-sync npm manifest/lock. It is never constructed from
// external input, so no other path can ever be written by this command.
var toolsUpdateAllowlist = []string{
	"config/toolchain.toml",
	"go.mod",
	"go.sum",
	"tools/linear-sync/package.json",
	"tools/linear-sync/package-lock.json",
}

// ToolchainPin is one config/toolchain.toml [toolchain.<name>] table's
// proposed version/archive/checksum triple.
type ToolchainPin struct {
	Version       string
	ArchiveURL    string
	ArchiveSHA256 string
}

// GoModulePin is one go.mod "require" entry's proposed version plus its
// two go.sum hash lines (the module hash and the go.mod hash).
type GoModulePin struct {
	Path    string
	Version string
	// SumHash is the go.sum "<path> <version> h1:<SumHash>" hash value
	// (without the "h1:" prefix, which callers add).
	SumHash string
	// ModHash is the go.sum "<path> <version>/go.mod h1:<ModHash>" hash
	// value (without the "h1:" prefix).
	ModHash string
}

// NpmPackagePin is one tools/linear-sync npm dependency's proposed
// version/integrity/resolved-URL triple.
type NpmPackagePin struct {
	Name      string
	Version   string
	Integrity string
	Resolved  string
}

// ToolsUpdateProposal is everything one MetadataSource.Propose() call
// returns for the five declared reviewable authorities.
type ToolsUpdateProposal struct {
	GoToolchain   ToolchainPin
	NodeToolchain ToolchainPin
	GoModule      GoModulePin
	LinearSDK     NpmPackagePin
	TypeScript    NpmPackagePin
}

// MetadataSource supplies candidate pin/version/hash data for
// "tools update". Only a fake, deterministic, in-memory implementation
// exists in Phase 1 (tools_test.go's fakeMetadataSource) -- see this
// file's package doc comment.
type MetadataSource interface {
	Propose() (ToolsUpdateProposal, error)
}

// ToolsUpdateCurrentFiles is the current on-disk bytes of the five
// declared authorities, read once before building a proposal.
type ToolsUpdateCurrentFiles struct {
	ToolchainTOML []byte
	GoMod         []byte
	GoSum         []byte
	PackageJSON   []byte
	PackageLock   []byte
}

// ToolsUpdateProposedFiles is the proposed new bytes (or, for Diffs, the
// diff bytes) of the five declared authorities, indexed the same way as
// ToolsUpdateCurrentFiles and toolsUpdateAllowlist.
type ToolsUpdateProposedFiles struct {
	ToolchainTOML []byte
	GoMod         []byte
	GoSum         []byte
	PackageJSON   []byte
	PackageLock   []byte
}

// ToolsUpdateResult is one complete, deterministic "tools update" outcome:
// the raw proposed pins, the five files' full proposed bytes, and the
// five files' diff bytes against the current on-disk bytes.
type ToolsUpdateResult struct {
	Proposal ToolsUpdateProposal
	Files    ToolsUpdateProposedFiles
	Diffs    ToolsUpdateProposedFiles
}

// readToolsUpdateCurrentFiles reads the exact five declared authorities
// from disk under root. It never reads any other path.
func readToolsUpdateCurrentFiles(root string) (ToolsUpdateCurrentFiles, error) {
	read := func(relative string) ([]byte, error) {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_READ: %s: %w", relative, err)
		}
		return content, nil
	}
	toolchainTOML, err := read(toolsUpdateAllowlist[0])
	if err != nil {
		return ToolsUpdateCurrentFiles{}, err
	}
	goMod, err := read(toolsUpdateAllowlist[1])
	if err != nil {
		return ToolsUpdateCurrentFiles{}, err
	}
	goSum, err := read(toolsUpdateAllowlist[2])
	if err != nil {
		return ToolsUpdateCurrentFiles{}, err
	}
	packageJSON, err := read(toolsUpdateAllowlist[3])
	if err != nil {
		return ToolsUpdateCurrentFiles{}, err
	}
	packageLock, err := read(toolsUpdateAllowlist[4])
	if err != nil {
		return ToolsUpdateCurrentFiles{}, err
	}
	return ToolsUpdateCurrentFiles{
		ToolchainTOML: toolchainTOML,
		GoMod:         goMod,
		GoSum:         goSum,
		PackageJSON:   packageJSON,
		PackageLock:   packageLock,
	}, nil
}

// nextTOMLHeaderPattern matches the start of any "[table]" header line.
var nextTOMLHeaderPattern = regexp.MustCompile(`(?m)^\[`)

// tomlTableSpan locates "[table]" in content and returns the byte offsets
// bounding its body: [headerEnd, bodyEnd). bodyEnd is the start of the
// next "[...]" header line, or len(content) if table is the last one.
func tomlTableSpan(content []byte, table string) (headerEnd, bodyEnd int, err error) {
	tableHeader := "[" + table + "]"
	start := strings.Index(string(content), tableHeader)
	if start == -1 {
		return 0, 0, fmt.Errorf("GOLC_TOOLS_UPDATE_TOML_TABLE: table %q not found", table)
	}
	headerEnd = start + len(tableHeader)
	bodyEnd = len(content)
	if loc := nextTOMLHeaderPattern.FindIndex(content[headerEnd:]); loc != nil {
		bodyEnd = headerEnd + loc[0]
	}
	return headerEnd, bodyEnd, nil
}

// tomlKeyLinePattern matches one exact `key = "value"` line, capturing
// the quoted value for read or surgical replacement.
func tomlKeyLinePattern(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(key) + ` = ")([^"]*)(")\s*$`)
}

// readTOMLTableValue reads one exact `key = "value"` line's value inside
// "[table]" without modifying content.
func readTOMLTableValue(content []byte, table, key string) (string, error) {
	headerEnd, bodyEnd, err := tomlTableSpan(content, table)
	if err != nil {
		return "", err
	}
	body := string(content[headerEnd:bodyEnd])
	match := tomlKeyLinePattern(key).FindStringSubmatch(body)
	if match == nil {
		return "", fmt.Errorf("GOLC_TOOLS_UPDATE_TOML_KEY: key %q not found in table %q", key, table)
	}
	return match[2], nil
}

// replaceTOMLTableValue returns content with one exact `key = "value"`
// line's value inside "[table]" replaced by newValue. Every other byte,
// including comments and every other table, is preserved verbatim.
func replaceTOMLTableValue(content []byte, table, key, newValue string) ([]byte, error) {
	headerEnd, bodyEnd, err := tomlTableSpan(content, table)
	if err != nil {
		return nil, err
	}
	body := content[headerEnd:bodyEnd]
	loc := tomlKeyLinePattern(key).FindSubmatchIndex(body)
	if loc == nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_TOML_KEY: key %q not found in table %q", key, table)
	}
	var buf bytes.Buffer
	buf.Write(content[:headerEnd])
	buf.Write(body[:loc[4]])
	buf.WriteString(newValue)
	buf.Write(body[loc[5]:])
	buf.Write(content[bodyEnd:])
	return buf.Bytes(), nil
}

// readTOMLTableTriple reads version from the parent tool table and the
// archive_url/archive_sha256 pair from its exact platform table without
// modifying content.
func readTOMLTableTriple(content []byte, versionTable, archiveTable string) (ToolchainPin, error) {
	version, err := readTOMLTableValue(content, versionTable, "version")
	if err != nil {
		return ToolchainPin{}, err
	}
	archiveURL, err := readTOMLTableValue(content, archiveTable, "archive_url")
	if err != nil {
		return ToolchainPin{}, err
	}
	archiveSHA256, err := readTOMLTableValue(content, archiveTable, "archive_sha256")
	if err != nil {
		return ToolchainPin{}, err
	}
	return ToolchainPin{Version: version, ArchiveURL: archiveURL, ArchiveSHA256: archiveSHA256}, nil
}

// applyToolchainTOMLProposal returns config/toolchain.toml's proposed
// bytes: only the two parent version lines and the four exact
// windows-amd64 archive_url/archive_sha256 lines change; every other byte
// (including comments, other platform data, and [cache]) is preserved.
func applyToolchainTOMLProposal(current []byte, goPin, nodePin ToolchainPin) ([]byte, error) {
	updated := current
	var err error
	for _, edit := range []struct{ table, key, value string }{
		{"toolchain.go", "version", goPin.Version},
		{`toolchain.go.platforms."windows-amd64"`, "archive_url", goPin.ArchiveURL},
		{`toolchain.go.platforms."windows-amd64"`, "archive_sha256", goPin.ArchiveSHA256},
		{"toolchain.node", "version", nodePin.Version},
		{`toolchain.node.platforms."windows-amd64"`, "archive_url", nodePin.ArchiveURL},
		{`toolchain.node.platforms."windows-amd64"`, "archive_sha256", nodePin.ArchiveSHA256},
	} {
		updated, err = replaceTOMLTableValue(updated, edit.table, edit.key, edit.value)
		if err != nil {
			return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_TOOLCHAIN: %w", err)
		}
	}
	return updated, nil
}

// goModuleLinePattern matches one exact go.mod require entry for path,
// whether written as a single-line "require <path> <version>" or as one
// line inside a "require ( ... )" block ("<path> <version>"). Group 1 is
// everything up to and including the separating whitespace, group 2 is
// the version token, group 3 is anything trailing (for example
// "// indirect").
func goModuleLinePattern(path string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\s*(?:require\s+)?` + regexp.QuoteMeta(path) + `\s+)(\S+)(.*)$`)
}

// applyGoModProposal returns go.mod's proposed bytes: only pin.Path's
// require-entry version changes; every other byte is preserved verbatim.
func applyGoModProposal(current []byte, pin GoModulePin) ([]byte, error) {
	loc := goModuleLinePattern(pin.Path).FindSubmatchIndex(current)
	if loc == nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_GOMOD: module %q not found in go.mod", pin.Path)
	}
	var buf bytes.Buffer
	buf.Write(current[:loc[4]])
	buf.WriteString(pin.Version)
	buf.Write(current[loc[5]:])
	return buf.Bytes(), nil
}

// applyGoSumProposal returns go.sum's proposed bytes: pin.Path's exact two
// lines (the module hash and the go.mod hash) are replaced with the
// proposed version and hashes; every other line is preserved verbatim.
func applyGoSumProposal(current []byte, pin GoModulePin) ([]byte, error) {
	// The module-hash line's version token must not contain "/" so it can
	// never also match the go.mod-hash line's "<version>/go.mod" token.
	sumLine := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(pin.Path) + ` [^/\s]+ h1:\S+$`)
	modLine := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(pin.Path) + ` \S+/go\.mod h1:\S+$`)

	if !sumLine.Match(current) {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_GOSUM: module %q hash line not found in go.sum", pin.Path)
	}
	if !modLine.Match(current) {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_GOSUM: module %q go.mod hash line not found in go.sum", pin.Path)
	}

	newSumLine := fmt.Sprintf("%s %s h1:%s", pin.Path, pin.Version, pin.SumHash)
	newModLine := fmt.Sprintf("%s %s/go.mod h1:%s", pin.Path, pin.Version, pin.ModHash)

	updated := sumLine.ReplaceAll(current, []byte(newSumLine))
	updated = modLine.ReplaceAll(updated, []byte(newModLine))
	return updated, nil
}

// setNestedStringField sets doc[section][key] = value, requiring that
// section and key already exist as a JSON object and string respectively.
func setNestedStringField(doc map[string]any, section, key, value string) error {
	raw, ok := doc[section]
	if !ok {
		return fmt.Errorf("section %q not found", section)
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("section %q is not an object", section)
	}
	if _, exists := nested[key]; !exists {
		return fmt.Errorf("key %q not found in section %q", key, section)
	}
	nested[key] = value
	return nil
}

// marshalJSONDeterministic re-serializes doc as indented JSON with a
// trailing newline. encoding/json always sorts map keys alphabetically at
// every level, so the same doc value always produces the same bytes.
func marshalJSONDeterministic(doc any) ([]byte, error) {
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// applyPackageJSONProposal returns tools/linear-sync/package.json's
// proposed bytes with linearSDK's and typescript's declared versions
// updated.
func applyPackageJSONProposal(current []byte, linearSDK, typescriptPin NpmPackagePin) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(current, &doc); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_JSON: %w", err)
	}
	if err := setNestedStringField(doc, "dependencies", linearSDK.Name, linearSDK.Version); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_JSON: %w", err)
	}
	if err := setNestedStringField(doc, "devDependencies", typescriptPin.Name, typescriptPin.Version); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_JSON: %w", err)
	}
	return marshalJSONDeterministic(doc)
}

// applyPackageLockProposal returns
// tools/linear-sync/package-lock.json's proposed bytes: the root
// package's dependencies/devDependencies entries and both packages'
// "node_modules/<name>" version/resolved/integrity fields are updated to
// match linearSDK/typescriptPin exactly; every other field is preserved.
func applyPackageLockProposal(current []byte, linearSDK, typescriptPin NpmPackagePin) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(current, &doc); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %w", err)
	}
	packagesRaw, ok := doc["packages"]
	if !ok {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: packages not found")
	}
	packages, ok := packagesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: packages is not an object")
	}

	rootRaw, ok := packages[""]
	if !ok {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: root package entry not found")
	}
	root, ok := rootRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: root package entry is not an object")
	}
	if err := setNestedStringField(root, "dependencies", linearSDK.Name, linearSDK.Version); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %w", err)
	}
	if err := setNestedStringField(root, "devDependencies", typescriptPin.Name, typescriptPin.Version); err != nil {
		return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %w", err)
	}

	for _, pin := range []NpmPackagePin{linearSDK, typescriptPin} {
		entryKey := "node_modules/" + pin.Name
		entryRaw, ok := packages[entryKey]
		if !ok {
			return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %q entry not found", entryKey)
		}
		entry, ok := entryRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %q entry is not an object", entryKey)
		}
		entry["version"] = pin.Version
		entry["resolved"] = pin.Resolved
		entry["integrity"] = pin.Integrity
	}

	return marshalJSONDeterministic(doc)
}

// verifyNpmConsistency asserts that a proposed package.json and
// package-lock.json agree exactly: every direct dependency/devDependency
// version in package.json matches both the lockfile's root package entry
// and its resolved "node_modules/<name>" entry (version/resolved/
// integrity all present).
func verifyNpmConsistency(packageJSON, packageLock []byte) error {
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(packageJSON, &manifest); err != nil {
		return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: package.json: %w", err)
	}

	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
			Version         string            `json:"version"`
			Resolved        string            `json:"resolved"`
			Integrity       string            `json:"integrity"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(packageLock, &lock); err != nil {
		return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: package-lock.json: %w", err)
	}
	if lock.LockfileVersion == 0 {
		return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: package-lock.json missing lockfileVersion")
	}
	root, ok := lock.Packages[""]
	if !ok {
		return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: package-lock.json missing root package entry")
	}

	checkGroup := func(direct map[string]string, lockedByName map[string]string, kind string) error {
		for name, version := range direct {
			if lockedByName[name] != version {
				return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: %s %q version mismatch between package.json and package-lock.json", kind, name)
			}
			entry, ok := lock.Packages["node_modules/"+name]
			if !ok || entry.Version != version || entry.Integrity == "" || entry.Resolved == "" {
				return fmt.Errorf("GOLC_TOOLS_UPDATE_NPM_INCONSISTENT: %s %q resolved entry inconsistent", kind, name)
			}
		}
		return nil
	}
	if err := checkGroup(manifest.Dependencies, root.Dependencies, "dependency"); err != nil {
		return err
	}
	if err := checkGroup(manifest.DevDependencies, root.DevDependencies, "devDependency"); err != nil {
		return err
	}
	return nil
}

// computeLineDiff returns a simple "-old"/"+new" line-pair diff between
// oldContent and newContent, or nil when they are identical. It is a
// review aid only, not a general-purpose diff algorithm: the surgical
// replace functions above only ever change fixed-position lines, so
// naive index-aligned line comparison is sufficient and fully
// deterministic for the same inputs.
func computeLineDiff(oldContent, newContent []byte) []byte {
	if bytes.Equal(oldContent, newContent) {
		return nil
	}
	oldLines := strings.Split(string(oldContent), "\n")
	newLines := strings.Split(string(newContent), "\n")
	longest := len(oldLines)
	if len(newLines) > longest {
		longest = len(newLines)
	}
	var buf bytes.Buffer
	for i := 0; i < longest; i++ {
		var oldLine, newLine string
		haveOld := i < len(oldLines)
		haveNew := i < len(newLines)
		if haveOld {
			oldLine = oldLines[i]
		}
		if haveNew {
			newLine = newLines[i]
		}
		if haveOld && haveNew && oldLine == newLine {
			continue
		}
		if haveOld {
			buf.WriteString("-" + oldLine + "\n")
		}
		if haveNew {
			buf.WriteString("+" + newLine + "\n")
		}
	}
	return buf.Bytes()
}

// BuildToolsUpdateProposal is the pure, deterministic core of "tools
// update": given a MetadataSource and the current on-disk bytes of the
// five declared authorities, it returns the exact proposed bytes and diff
// bytes for all five files. It never reads or writes any file itself and
// never opens a network connection, downloads an archive, extracts a zip,
// warms a cache, installs a package, or compiles anything -- calling it
// twice with the same source and current files always returns
// byte-identical results.
func BuildToolsUpdateProposal(source MetadataSource, current ToolsUpdateCurrentFiles) (ToolsUpdateResult, error) {
	proposal, err := source.Propose()
	if err != nil {
		return ToolsUpdateResult{}, fmt.Errorf("GOLC_TOOLS_UPDATE_SOURCE: %w", err)
	}

	newToolchainTOML, err := applyToolchainTOMLProposal(current.ToolchainTOML, proposal.GoToolchain, proposal.NodeToolchain)
	if err != nil {
		return ToolsUpdateResult{}, err
	}
	newGoMod, err := applyGoModProposal(current.GoMod, proposal.GoModule)
	if err != nil {
		return ToolsUpdateResult{}, err
	}
	newGoSum, err := applyGoSumProposal(current.GoSum, proposal.GoModule)
	if err != nil {
		return ToolsUpdateResult{}, err
	}
	newPackageJSON, err := applyPackageJSONProposal(current.PackageJSON, proposal.LinearSDK, proposal.TypeScript)
	if err != nil {
		return ToolsUpdateResult{}, err
	}
	newPackageLock, err := applyPackageLockProposal(current.PackageLock, proposal.LinearSDK, proposal.TypeScript)
	if err != nil {
		return ToolsUpdateResult{}, err
	}
	if err := verifyNpmConsistency(newPackageJSON, newPackageLock); err != nil {
		return ToolsUpdateResult{}, err
	}

	files := ToolsUpdateProposedFiles{
		ToolchainTOML: newToolchainTOML,
		GoMod:         newGoMod,
		GoSum:         newGoSum,
		PackageJSON:   newPackageJSON,
		PackageLock:   newPackageLock,
	}
	diffs := ToolsUpdateProposedFiles{
		ToolchainTOML: computeLineDiff(current.ToolchainTOML, newToolchainTOML),
		GoMod:         computeLineDiff(current.GoMod, newGoMod),
		GoSum:         computeLineDiff(current.GoSum, newGoSum),
		PackageJSON:   computeLineDiff(current.PackageJSON, newPackageJSON),
		PackageLock:   computeLineDiff(current.PackageLock, newPackageLock),
	}

	return ToolsUpdateResult{Proposal: proposal, Files: files, Diffs: diffs}, nil
}

// writeToolsUpdateFiles writes exactly the five toolsUpdateAllowlist paths
// under root with files' matching bytes. The set of paths written is
// fixed by toolsUpdateAllowlist, never by external input, so no other
// path can ever be created or modified by this function.
func writeToolsUpdateFiles(root string, files ToolsUpdateProposedFiles) error {
	ordered := []struct {
		relative string
		content  []byte
	}{
		{toolsUpdateAllowlist[0], files.ToolchainTOML},
		{toolsUpdateAllowlist[1], files.GoMod},
		{toolsUpdateAllowlist[2], files.GoSum},
		{toolsUpdateAllowlist[3], files.PackageJSON},
		{toolsUpdateAllowlist[4], files.PackageLock},
	}
	for _, entry := range ordered {
		absolute := filepath.Join(root, filepath.FromSlash(entry.relative))
		if err := os.WriteFile(absolute, entry.content, 0o644); err != nil {
			return fmt.Errorf("GOLC_TOOLS_UPDATE_WRITE: %s: %w", entry.relative, err)
		}
	}
	return nil
}

// defaultManagedGoModulePath is the one go.mod "require" entry the
// production default source manages. It is a direct dependency already
// present in this repository's go.mod/go.sum.
const defaultManagedGoModulePath = "github.com/BurntSushi/toml"

// readCurrentGoModulePin reads defaultManagedGoModulePath's exact current
// version and go.sum hashes without modifying anything, so
// defaultMetadataSource can echo them back as a safe no-op pin.
func readCurrentGoModulePin(goMod, goSum []byte) (GoModulePin, error) {
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(defaultManagedGoModulePath) + `\s+(\S+)`)
	match := pattern.FindSubmatch(goMod)
	if match == nil {
		return GoModulePin{}, fmt.Errorf("GOLC_TOOLS_UPDATE_GOMOD: module %q not found in go.mod", defaultManagedGoModulePath)
	}
	version := string(match[1])

	sumPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(defaultManagedGoModulePath) + ` ` + regexp.QuoteMeta(version) + ` h1:(\S+)$`)
	sumMatch := sumPattern.FindSubmatch(goSum)
	if sumMatch == nil {
		return GoModulePin{}, fmt.Errorf("GOLC_TOOLS_UPDATE_GOSUM: module %q hash line not found in go.sum", defaultManagedGoModulePath)
	}
	modPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(defaultManagedGoModulePath) + ` ` + regexp.QuoteMeta(version) + `/go\.mod h1:(\S+)$`)
	modMatch := modPattern.FindSubmatch(goSum)
	if modMatch == nil {
		return GoModulePin{}, fmt.Errorf("GOLC_TOOLS_UPDATE_GOSUM: module %q go.mod hash line not found in go.sum", defaultManagedGoModulePath)
	}

	return GoModulePin{
		Path:    defaultManagedGoModulePath,
		Version: version,
		SumHash: string(sumMatch[1]),
		ModHash: string(modMatch[1]),
	}, nil
}

// readCurrentNpmPackagePin reads name's exact current
// version/resolved/integrity from packageLock's "node_modules/<name>"
// entry without modifying anything, so defaultMetadataSource can echo it
// back as a safe no-op pin.
func readCurrentNpmPackagePin(packageLock []byte, name string) (NpmPackagePin, error) {
	var lock struct {
		Packages map[string]struct {
			Version   string `json:"version"`
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(packageLock, &lock); err != nil {
		return NpmPackagePin{}, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %w", err)
	}
	entry, ok := lock.Packages["node_modules/"+name]
	if !ok {
		return NpmPackagePin{}, fmt.Errorf("GOLC_TOOLS_UPDATE_PACKAGE_LOCK: %q entry not found", name)
	}
	return NpmPackagePin{
		Name:      name,
		Version:   entry.Version,
		Resolved:  entry.Resolved,
		Integrity: entry.Integrity,
	}, nil
}

// defaultMetadataSource is the production MetadataSource used by the
// registered "tools update" route. Live remote metadata polling
// (go.dev/nodejs.org/npm-registry) is out of this plan's scope (see this
// file's package doc comment and 01-29-SUMMARY.md's Known Stubs): it
// deterministically re-affirms every currently pinned value already
// present on disk, so a production "tools update --check" run over the
// real repository always proposes the exact same version/archive/hash
// values until a future plan wires a real MetadataSource, and
// "tools update --write" is a safe, value-for-value no-op: config/
// toolchain.toml, go.mod, and go.sum are rewritten byte-for-byte
// identically (surgical line replacement never reformats untouched
// bytes), while tools/linear-sync/package.json and package-lock.json are
// rewritten through canonical deterministic JSON re-serialization
// (marshalJSONDeterministic) and so may change byte layout (for example
// key ordering) even though every value stays exactly the same.
type defaultMetadataSource struct {
	current ToolsUpdateCurrentFiles
}

func newDefaultMetadataSource(current ToolsUpdateCurrentFiles) (MetadataSource, error) {
	return defaultMetadataSource{current: current}, nil
}

func (s defaultMetadataSource) Propose() (ToolsUpdateProposal, error) {
	goPin, err := readTOMLTableTriple(
		s.current.ToolchainTOML,
		"toolchain.go",
		`toolchain.go.platforms."windows-amd64"`,
	)
	if err != nil {
		return ToolsUpdateProposal{}, err
	}
	nodePin, err := readTOMLTableTriple(
		s.current.ToolchainTOML,
		"toolchain.node",
		`toolchain.node.platforms."windows-amd64"`,
	)
	if err != nil {
		return ToolsUpdateProposal{}, err
	}
	goModulePin, err := readCurrentGoModulePin(s.current.GoMod, s.current.GoSum)
	if err != nil {
		return ToolsUpdateProposal{}, err
	}
	linearSDK, err := readCurrentNpmPackagePin(s.current.PackageLock, "@linear/sdk")
	if err != nil {
		return ToolsUpdateProposal{}, err
	}
	typescriptPin, err := readCurrentNpmPackagePin(s.current.PackageLock, "typescript")
	if err != nil {
		return ToolsUpdateProposal{}, err
	}
	return ToolsUpdateProposal{
		GoToolchain:   goPin,
		NodeToolchain: nodePin,
		GoModule:      goModulePin,
		LinearSDK:     linearSDK,
		TypeScript:    typescriptPin,
	}, nil
}

// parseToolsUpdateArgs accepts exactly one of "--check" or "--write" and
// rejects anything else, including both together.
func parseToolsUpdateArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("GOLC_TOOLS_USAGE: usage: tools update --check|--write")
	}
	switch args[0] {
	case "--check":
		return "check", nil
	case "--write":
		return "write", nil
	default:
		return "", fmt.Errorf("GOLC_TOOLS_USAGE: unsupported argument %q; usage: tools update --check|--write", args[0])
	}
}

// renderToolsUpdateReport formats one ToolsUpdateResult for stdout.
func renderToolsUpdateReport(mode string, result ToolsUpdateResult) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "GOLC tools update --%s\n", mode)
	for _, entry := range []struct {
		path string
		diff []byte
	}{
		{toolsUpdateAllowlist[0], result.Diffs.ToolchainTOML},
		{toolsUpdateAllowlist[1], result.Diffs.GoMod},
		{toolsUpdateAllowlist[2], result.Diffs.GoSum},
		{toolsUpdateAllowlist[3], result.Diffs.PackageJSON},
		{toolsUpdateAllowlist[4], result.Diffs.PackageLock},
	} {
		if len(entry.diff) == 0 {
			fmt.Fprintf(&buf, "GOLC tools update: %s unchanged\n", entry.path)
			continue
		}
		fmt.Fprintf(&buf, "GOLC tools update: %s changed\n", entry.path)
		buf.Write(entry.diff)
	}
	return buf.Bytes()
}

// runToolsUpdate serves the self-registered "tools update" route,
// dispatching to --check (compute and report only) or --write (compute,
// then write exactly the five allowlisted files).
func runToolsUpdate(request Request) Result {
	mode, err := parseToolsUpdateArgs(request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	current, err := readToolsUpdateCurrentFiles(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	source, err := newDefaultMetadataSource(current)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	result, err := BuildToolsUpdateProposal(source, current)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	if mode == "check" {
		return Result{Stdout: renderToolsUpdateReport("check", result)}
	}

	if err := writeToolsUpdateFiles(request.Root, result.Files); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: renderToolsUpdateReport("write", result)}
}
