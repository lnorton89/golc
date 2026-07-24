package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Options selects optional bootstrap units. The core Go bootstrap always runs.
type Options struct {
	IncludeLinearSync bool
}

// PlatformKey is the stable install-directory key for the running process.
func PlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

var validExecutableBase = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ExecutableName returns a platform-native executable name for a safe base
// name. An empty result rejects blank names, path components, and traversal.
func ExecutableName(base string) string {
	if !validExecutableBase.MatchString(base) || base == "." || base == ".." {
		return ""
	}
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// PlatformExecutablePath resolves a provisioned command beneath its stable
// runtime platform directory. An empty result indicates an unsafe base name.
func PlatformExecutablePath(installRoot, base string) string {
	name := ExecutableName(base)
	if name == "" {
		return ""
	}
	return filepath.Join(installRoot, PlatformKey(), "bin", name)
}

// NodeInstallation is the validated executable surface of a provisioned Node
// archive.
type NodeInstallation struct {
	Root       string
	Executable string
	NPMCLI     string
}

// ResolveNodeInstallation discovers the sole verified archive payload without
// coupling consumers to the upstream archive's top-level directory spelling.
func ResolveNodeInstallation(installDir string) (NodeInstallation, error) {
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: inspect %s: %w", installDir, err)
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	manifestFound := false
	var directories []string
	for _, entry := range entries {
		path := filepath.Join(installDir, entry.Name())
		lstat, err := os.Lstat(path)
		if err != nil {
			return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: inspect %s: %w", path, err)
		}
		if entry.Name() == ManifestName {
			if lstat.Mode()&os.ModeSymlink != 0 || !lstat.Mode().IsRegular() {
				return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: install manifest must be a regular file at %s", path)
			}
			manifestFound = true
			continue
		}
		if lstat.Mode()&os.ModeSymlink != 0 || !lstat.IsDir() {
			return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: unexpected top-level entry %s", path)
		}
		stat, err := os.Stat(path)
		if err != nil || !stat.IsDir() {
			return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: invalid top-level directory %s", path)
		}
		directories = append(directories, path)
	}
	if !manifestFound {
		return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: expected regular install manifest at %s", filepath.Join(installDir, ManifestName))
	}
	if len(directories) != 1 {
		return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: expected exactly one extracted directory in %s, found %d: %v", installDir, len(directories), directories)
	}

	var executableRelative, npmRelative string
	switch runtime.GOOS {
	case "windows":
		executableRelative = "node.exe"
		npmRelative = filepath.Join("node_modules", "npm", "bin", "npm-cli.js")
	case "linux", "darwin":
		executableRelative = filepath.Join("bin", "node")
		npmRelative = filepath.Join("lib", "node_modules", "npm", "bin", "npm-cli.js")
	default:
		return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: Node has no executable mapping for %s", PlatformKey())
	}

	installation := NodeInstallation{
		Root:       directories[0],
		Executable: filepath.Join(directories[0], executableRelative),
		NPMCLI:     filepath.Join(directories[0], npmRelative),
	}
	for label, path := range map[string]string{"node": installation.Executable, "npm-cli.js": installation.NPMCLI} {
		lstat, err := os.Lstat(path)
		if err != nil || lstat.Mode()&os.ModeSymlink != 0 {
			return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: expected regular %s at %s", label, path)
		}
		stat, err := os.Stat(path)
		if err != nil || !stat.Mode().IsRegular() {
			return NodeInstallation{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: expected regular %s at %s", label, path)
		}
	}
	return installation, nil
}

type platformLayout struct {
	FileName   string
	Format     string
	Root       string
	Executable string
	NPMCLI     string
}

func platformArchiveLayout(tool, version, goos, goarch string) (platformLayout, error) {
	switch tool {
	case "go":
		switch goos {
		case "windows":
			return platformLayout{
				FileName: "go" + version + ".windows-" + goarch + ".zip",
				Format:   ".zip", Root: "go", Executable: filepath.Join("bin", "go.exe"),
			}, nil
		case "linux", "darwin":
			return platformLayout{
				FileName: "go" + version + "." + goos + "-" + goarch + ".tar.gz",
				Format:   ".tar.gz", Root: "go", Executable: filepath.Join("bin", "go"),
			}, nil
		default:
			return platformLayout{}, fmt.Errorf("GOLC_BOOTSTRAP_UNSUPPORTED_PLATFORM: Go has no mapping for %s-%s", goos, goarch)
		}
	case "node":
		arch := goarch
		if goarch == "amd64" {
			arch = "x64"
		}
		if goos == "windows" {
			root := "node-v" + version + "-win-" + arch
			return platformLayout{
				FileName: root + ".zip", Format: ".zip", Root: root,
				Executable: "node.exe", NPMCLI: filepath.Join("node_modules", "npm", "bin", "npm-cli.js"),
			}, nil
		}
		if goos == "linux" || goos == "darwin" {
			root := "node-v" + version + "-" + goos + "-" + arch
			return platformLayout{
				FileName: root + ".tar.gz", Format: ".tar.gz", Root: root,
				Executable: filepath.Join("bin", "node"),
				NPMCLI:     filepath.Join("lib", "node_modules", "npm", "bin", "npm-cli.js"),
			}, nil
		}
		return platformLayout{}, fmt.Errorf("GOLC_BOOTSTRAP_UNSUPPORTED_PLATFORM: Node has no mapping for %s-%s", goos, goarch)
	case "mage":
		switch goos + "-" + goarch {
		case "windows-amd64":
			return platformLayout{
				FileName: "mage_" + version + "_Windows-64bit.zip",
				Format:   ".zip", Executable: "mage.exe",
			}, nil
		case "linux-amd64":
			return platformLayout{
				FileName: "mage_" + version + "_Linux-64bit.tar.gz",
				Format:   ".tar.gz", Executable: "mage",
			}, nil
		case "linux-arm64":
			return platformLayout{
				FileName: "mage_" + version + "_Linux-ARM64.tar.gz",
				Format:   ".tar.gz", Executable: "mage",
			}, nil
		case "darwin-amd64":
			return platformLayout{
				FileName: "mage_" + version + "_macOS-64bit.tar.gz",
				Format:   ".tar.gz", Executable: "mage",
			}, nil
		case "darwin-arm64":
			return platformLayout{
				FileName: "mage_" + version + "_macOS-ARM64.tar.gz",
				Format:   ".tar.gz", Executable: "mage",
			}, nil
		default:
			return platformLayout{}, fmt.Errorf("GOLC_BOOTSTRAP_UNSUPPORTED_PLATFORM: Mage has no mapping for %s-%s", goos, goarch)
		}
	default:
		return platformLayout{}, fmt.Errorf("GOLC_BOOTSTRAP_UNSUPPORTED_TOOL: %q", tool)
	}
}

type processRequest struct {
	Executable string
	Dir        string
	Args       []string
	Env        map[string]string
}

type processRunner interface {
	Run(context.Context, processRequest) ([]byte, error)
}

type execProcessRunner struct{}

func (execProcessRunner) Run(ctx context.Context, request processRequest) ([]byte, error) {
	command := exec.CommandContext(ctx, request.Executable, request.Args...)
	command.Dir = request.Dir
	command.Env = environmentList(request.Env)
	output, err := command.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return output, fmt.Errorf("exit %d: %s", exitError.ExitCode(), strings.TrimSpace(string(exitError.Stderr)))
		}
		return output, err
	}
	return output, nil
}

type bootstrapDependencies struct {
	Source Source
	Runner processRunner
}

type manifestPin struct {
	Version            string `toml:"version"`
	ArchiveURL         string `toml:"archive_url"`
	ArchiveSHA256      string `toml:"archive_sha256"`
	OfficialHost       string `toml:"official_host"`
	OfficialPathPrefix string `toml:"official_path_prefix"`
}

type platformArchivePin struct {
	ArchiveURL    string `toml:"archive_url"`
	ArchiveSHA256 string `toml:"archive_sha256"`
}

type toolchainManifestPin struct {
	Version            string                        `toml:"version"`
	OfficialHost       string                        `toml:"official_host"`
	OfficialPathPrefix string                        `toml:"official_path_prefix"`
	Platforms          map[string]platformArchivePin `toml:"platforms"`
}

type bootstrapManifest struct {
	SchemaVersion int                             `toml:"schema_version"`
	Cache         map[string]string               `toml:"cache"`
	Tools         map[string]manifestPin          `toml:"tools"`
	Toolchain     map[string]toolchainManifestPin `toml:"toolchain"`
}

type bootstrapEngine struct {
	root     string
	options  Options
	document bootstrapManifest
	layout   ProjectCacheLayout
	policy   OfficialSourcePolicy
	source   Source
	runner   processRunner
	env      map[string]string
	goPin    manifestPin
	magePin  manifestPin
	nodePin  manifestPin
}

var validToolName = regexp.MustCompile(`^[a-z0-9_]+$`)

// Bootstrap constructs production dependencies and executes the complete
// callable bootstrap engine without changing any configuration authority.
func Bootstrap(ctx context.Context, root string, options Options) error {
	return runBootstrap(ctx, root, options, bootstrapDependencies{})
}

func runBootstrap(ctx context.Context, root string, options Options, dependencies bootstrapDependencies) error {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_ROOT: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_ROOT: %w", err)
	}
	document, policy, err := readBootstrapManifest(resolvedRoot)
	if err != nil {
		return err
	}
	goPin, magePin, nodePin, err := validateManifestForPlatform(document, options)
	if err != nil {
		return err
	}
	layout, err := newProjectCacheLayout(resolvedRoot, document.Cache)
	if err != nil {
		return err
	}
	runner := dependencies.Runner
	if runner == nil {
		runner = execProcessRunner{}
	}
	source := dependencies.Source
	if source == nil {
		source = URLSource{Policy: policy, Client: &http.Client{}}
	}
	engine := &bootstrapEngine{
		root: resolvedRoot, options: options, document: document, layout: layout,
		policy: policy, source: source, runner: runner, goPin: goPin, magePin: magePin, nodePin: nodePin,
	}
	engine.env = mergedEnvironment(layout.Environment().AsMap())
	setEnvironmentValue(engine.env, "GOLC_PROJECT_ROOT", resolvedRoot)
	return engine.run(ctx)
}

func readBootstrapManifest(root string) (bootstrapManifest, OfficialSourcePolicy, error) {
	path := filepath.Join(root, "config", "toolchain.toml")
	var document bootstrapManifest
	metadata, err := toml.DecodeFile(path, &document)
	if err != nil {
		return bootstrapManifest{}, OfficialSourcePolicy{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: %s: %w", path, err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		return bootstrapManifest{}, OfficialSourcePolicy{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: unsupported keys %v", undecoded)
	}
	if document.SchemaVersion != 2 {
		return bootstrapManifest{}, OfficialSourcePolicy{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: schema_version must be 2")
	}
	var patterns []SourcePattern
	collect := func(pins map[string]manifestPin) {
		names := make([]string, 0, len(pins))
		for name := range pins {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			pin := pins[name]
			if pin.OfficialHost != "" && pin.OfficialPathPrefix != "" {
				patterns = append(patterns, SourcePattern{Host: pin.OfficialHost, PathPrefix: pin.OfficialPathPrefix})
			}
		}
	}
	collect(document.Tools)
	toolchainNames := make([]string, 0, len(document.Toolchain))
	for name := range document.Toolchain {
		toolchainNames = append(toolchainNames, name)
	}
	sort.Strings(toolchainNames)
	for _, name := range toolchainNames {
		pin := document.Toolchain[name]
		if pin.OfficialHost != "" && pin.OfficialPathPrefix != "" {
			patterns = append(patterns, SourcePattern{Host: pin.OfficialHost, PathPrefix: pin.OfficialPathPrefix})
		}
	}
	return document, OfficialSourcePolicy{Patterns: patterns}, nil
}

func validateManifestForPlatform(document bootstrapManifest, options Options) (manifestPin, manifestPin, manifestPin, error) {
	names := make([]string, 0, len(document.Tools))
	for name := range document.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !validToolName.MatchString(name) {
			return manifestPin{}, manifestPin{}, manifestPin{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: invalid tool name %q", name)
		}
		if err := validatePin("tools."+name, document.Tools[name]); err != nil {
			return manifestPin{}, manifestPin{}, manifestPin{}, err
		}
	}
	goParent, ok := document.Toolchain["go"]
	if !ok {
		return manifestPin{}, manifestPin{}, manifestPin{}, fmt.Errorf("GOLC_GO_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.go]")
	}
	goPin, err := selectPlatformPin("go", goParent)
	if err != nil {
		return manifestPin{}, manifestPin{}, manifestPin{}, err
	}
	mageParent, ok := document.Toolchain["mage"]
	if !ok {
		return manifestPin{}, manifestPin{}, manifestPin{}, fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.mage]")
	}
	magePin, err := selectPlatformPin("mage", mageParent)
	if err != nil {
		return manifestPin{}, manifestPin{}, manifestPin{}, err
	}
	// Node is resolved (and, in run(), installed) unconditionally now,
	// not only when options.IncludeLinearSync is set: cmd/golc-desktop's
	// `//go:embed all:frontend/dist` means every default "build"/"test"
	// route needs a built frontend/dist to compile at all, so bootstrap
	// must always be able to produce one (see runFrontendBuild). Only
	// the separate tools/linear-sync npm ci/tsc build stays opt-in.
	nodeParent, ok := document.Toolchain["node"]
	if !ok {
		return manifestPin{}, manifestPin{}, manifestPin{}, fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.node]")
	}
	nodePin, err := selectPlatformPin("node", nodeParent)
	if err != nil {
		return manifestPin{}, manifestPin{}, manifestPin{}, err
	}
	return goPin, magePin, nodePin, nil
}

func validatePin(name string, pin manifestPin) error {
	if strings.TrimSpace(pin.ArchiveURL) == "" || strings.TrimSpace(pin.ArchiveSHA256) == "" {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [%s] is missing archive URL or archive_sha256", name)
	}
	normalized, err := normalizeExpectedSHA256(pin.ArchiveSHA256)
	if err != nil || normalized != pin.ArchiveSHA256 {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [%s] has invalid lowercase archive_sha256", name)
	}
	if _, err := archiveSuffix(pin.ArchiveURL); err != nil {
		return err
	}
	return nil
}

func selectPlatformPin(tool string, parent toolchainManifestPin) (manifestPin, error) {
	return selectPlatformPinFor(tool, parent, runtime.GOOS, runtime.GOARCH)
}

func selectPlatformPinFor(tool string, parent toolchainManifestPin, goos, goarch string) (manifestPin, error) {
	if strings.TrimSpace(parent.Version) == "" {
		return manifestPin{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [toolchain.%s] is missing version", tool)
	}
	key := goos + "-" + goarch
	archive, ok := parent.Platforms[key]
	if !ok {
		return manifestPin{}, fmt.Errorf(
			"GOLC_%s_TOOLCHAIN_PLATFORM_MISSING: config/toolchain.toml must pin [toolchain.%s.platforms.%q]",
			strings.ToUpper(tool), tool, key)
	}
	pin := manifestPin{
		Version:            parent.Version,
		ArchiveURL:         archive.ArchiveURL,
		ArchiveSHA256:      archive.ArchiveSHA256,
		OfficialHost:       parent.OfficialHost,
		OfficialPathPrefix: parent.OfficialPathPrefix,
	}
	if err := validatePin(fmt.Sprintf("toolchain.%s.platforms.%q", tool, key), pin); err != nil {
		return manifestPin{}, err
	}
	layout, err := platformArchiveLayout(tool, pin.Version, goos, goarch)
	if err != nil {
		return manifestPin{}, err
	}
	parsed, err := url.Parse(pin.ArchiveURL)
	if err != nil {
		return manifestPin{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: invalid %s archive URL: %w", tool, err)
	}
	actual, err := url.PathUnescape(filepath.Base(parsed.Path))
	if err != nil || actual != layout.FileName {
		return manifestPin{}, fmt.Errorf("GOLC_BOOTSTRAP_PLATFORM_MISMATCH: %s pin names %q, running platform %s requires %q", tool, actual, key, layout.FileName)
	}
	return pin, nil
}

func (engine *bootstrapEngine) run(ctx context.Context) error {
	if err := engine.layout.Warm(); err != nil {
		return err
	}
	names := make([]string, 0, len(engine.document.Tools))
	for name := range engine.document.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pin := engine.document.Tools[name]
		installDir := filepath.Join(engine.root, ".tools", "installs", name)
		if err := engine.installPin(pin, installDir); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_TOOL_INSTALL: %s: %w", name, err)
		}
	}
	magePin := engine.magePin
	mageInstall := filepath.Join(engine.root, ".tools", "toolchains", "mage", magePin.Version, PlatformKey())
	if err := engine.installPin(magePin, mageInstall); err != nil {
		return fmt.Errorf("GOLC_MAGE_TOOLCHAIN_INSTALL: %w", err)
	}
	mageLayout, _ := platformArchiveLayout("mage", magePin.Version, runtime.GOOS, runtime.GOARCH)
	mageExecutable := filepath.Join(mageInstall, mageLayout.Executable)
	if info, err := os.Lstat(mageExecutable); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: expected regular pinned executable at %s", mageExecutable)
	}
	goPin := engine.goPin
	goInstall := filepath.Join(engine.root, ".tools", "toolchains", "go", goPin.Version, PlatformKey())
	if err := engine.installPin(goPin, goInstall); err != nil {
		return fmt.Errorf("GOLC_GO_TOOLCHAIN_INSTALL: %w", err)
	}
	goLayout, _ := platformArchiveLayout("go", goPin.Version, runtime.GOOS, runtime.GOARCH)
	goExecutable := filepath.Join(goInstall, filepath.FromSlash(goLayout.Root), goLayout.Executable)
	if info, err := os.Stat(goExecutable); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("GOLC_GO_TOOLCHAIN_MISSING: expected pinned executable at %s", goExecutable)
	}
	if err := engine.runGoPhase(ctx, goExecutable); err != nil {
		return err
	}
	if err := runFrontendBuild(ctx, engine); err != nil {
		return err
	}
	if engine.options.IncludeLinearSync {
		return linearSyncBootstrap(ctx, engine)
	}
	return nil
}

// ResolveMageExecutable returns the current platform's verified project-local
// Mage executable. It never downloads or consults the host PATH.
func ResolveMageExecutable(root string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("GOLC_BOOTSTRAP_ROOT: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", fmt.Errorf("GOLC_BOOTSTRAP_ROOT: %w", err)
	}
	document, _, err := readBootstrapManifest(resolvedRoot)
	if err != nil {
		return "", err
	}
	parent, ok := document.Toolchain["mage"]
	if !ok {
		return "", fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.mage]")
	}
	pin, err := selectPlatformPin("mage", parent)
	if err != nil {
		return "", err
	}
	installDir := filepath.Join(resolvedRoot, ".tools", "toolchains", "mage", pin.Version, PlatformKey())
	matches, err := InstalledMatches(installDir, pin.ArchiveSHA256)
	if err != nil {
		return "", fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: %w", err)
	}
	if !matches {
		return "", fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: verified install does not match pin at %s", installDir)
	}
	layout, err := platformArchiveLayout("mage", pin.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	executable := filepath.Join(installDir, layout.Executable)
	info, err := os.Lstat(executable)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("GOLC_MAGE_TOOLCHAIN_MISSING: expected regular pinned executable at %s", executable)
	}
	return executable, nil
}

func (engine *bootstrapEngine) installPin(pin manifestPin, installDir string) error {
	matches, err := InstalledMatches(installDir, pin.ArchiveSHA256)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	archivePath, err := Acquire(engine.policy, engine.source, pin.ArchiveURL, pin.ArchiveSHA256, engine.layout.Downloads)
	if err != nil {
		return err
	}
	return InstallStaged(archivePath, pin.ArchiveSHA256, installDir)
}

func (engine *bootstrapEngine) runGoPhase(ctx context.Context, goExecutable string) (resultErr error) {
	goModPath := filepath.Join(engine.root, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil
	}
	goSumPath := filepath.Join(engine.root, "go.sum")
	goModBefore, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: go.mod: %w", err)
	}
	goSumBefore, err := os.ReadFile(goSumPath)
	if err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_OFFLINE_ARTIFACT_MISSING: go.sum: %w", err)
	}
	defer func() {
		modAfter, modErr := os.ReadFile(goModPath)
		sumAfter, sumErr := os.ReadFile(goSumPath)
		mutated := modErr != nil || sumErr != nil || !bytes.Equal(goModBefore, modAfter) || !bytes.Equal(goSumBefore, sumAfter)
		if !mutated {
			return
		}
		restoreErr := writeExactFile(goModPath, goModBefore, 0o644)
		if err := writeExactFile(goSumPath, goSumBefore, 0o644); restoreErr == nil {
			restoreErr = err
		}
		if restoreErr != nil {
			resultErr = fmt.Errorf("GOLC_BOOTSTRAP_LOCK_MUTATION: locks changed and restoration failed: %w", restoreErr)
			return
		}
		resultErr = fmt.Errorf("GOLC_BOOTSTRAP_LOCK_MUTATION: bootstrap must never rewrite go.mod or go.sum")
	}()

	if _, err := engine.runProcess(ctx, goExecutable, "GOLC_BOOTSTRAP_MODULE_DOWNLOAD", "mod", "download", "all"); err != nil {
		return err
	}
	if _, err := engine.runProcess(ctx, goExecutable, "GOLC_BOOTSTRAP_MODULE_VERIFY", "mod", "verify"); err != nil {
		return err
	}
	graphOutput, err := engine.runProcess(ctx, goExecutable, "GOLC_BOOTSTRAP_MODULE_GRAPH", "list", "-m", "all")
	if err != nil {
		return err
	}
	graph := normalizeModuleGraph(graphOutput)
	required := []string{
		"github.com/BurntSushi/toml v1.6.0",
		"github.com/invopop/jsonschema v0.14.0",
	}
	lineSet := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSuffix(graph, "\n"), "\n") {
		lineSet[line] = struct{}{}
	}
	for _, module := range required {
		if _, ok := lineSet[module]; !ok {
			return fmt.Errorf("GOLC_BOOTSTRAP_MODULE_PIN_MISSING: %s", module)
		}
	}
	if err := writeAtomicFile(filepath.Join(engine.layout.Manifest, "go-modules.txt"), []byte(graph), 0o644); err != nil {
		return fmt.Errorf("GOLC_BOOTSTRAP_MODULE_GRAPH: %w", err)
	}
	if _, err := engine.runProcess(ctx, goExecutable, "GOLC_BOOTSTRAP_PROBE_FAILED", "test", "-count=1", "./internal/bootstrap/"); err != nil {
		return err
	}
	projectDir := filepath.Join(engine.root, "cmd", "golc-project")
	if info, err := os.Stat(projectDir); err == nil && info.IsDir() {
		output := PlatformExecutablePath(filepath.Join(engine.root, ".tools", "installs", "golc_project"), "golc-project")
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return fmt.Errorf("GOLC_BOOTSTRAP_PROJECT_BUILD: %w", err)
		}
		if _, err := engine.runProcess(ctx, goExecutable, "GOLC_BOOTSTRAP_PROJECT_BUILD", "build", "-trimpath", "-o", output, "./cmd/golc-project"); err != nil {
			return err
		}
	}
	return nil
}

func (engine *bootstrapEngine) runProcess(ctx context.Context, executable, diagnostic string, args ...string) ([]byte, error) {
	output, err := engine.runner.Run(ctx, processRequest{
		Executable: executable,
		Dir:        engine.root,
		Args:       append([]string(nil), args...),
		Env:        cloneEnvironment(engine.env),
	})
	if err != nil {
		// The captured process output is included here (not just the
		// bare "exit status 1"-shaped exec error) because without it a
		// failing "go test"/"go build" invocation reports zero
		// diagnostic detail — exactly the gap that made a real CI
		// failure (run 30074378227's GOLC_BOOTSTRAP_PROBE_FAILED)
		// unreadable from its own logged error message.
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return output, fmt.Errorf("%s: %s: %w", diagnostic, strings.Join(args, " "), err)
		}
		return output, fmt.Errorf("%s: %s: %w\n%s", diagnostic, strings.Join(args, " "), err, trimmed)
	}
	return output, nil
}

func normalizeModuleGraph(output []byte) string {
	text := strings.ReplaceAll(string(output), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.Join(lines, "\n") + "\n"
}

func mergedEnvironment(overrides map[string]string) map[string]string {
	result := map[string]string{}
	for _, entry := range os.Environ() {
		if index := strings.IndexByte(entry, '='); index > 0 {
			result[entry[:index]] = entry[index+1:]
		}
	}
	for key, value := range overrides {
		result[key] = value
	}
	return result
}

func setEnvironmentValue(environment map[string]string, name, value string) {
	for existing := range environment {
		if strings.EqualFold(existing, name) {
			delete(environment, existing)
		}
	}
	environment[name] = value
}

func cloneEnvironment(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func environmentList(environment map[string]string) []string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+environment[key])
	}
	return result
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".golc-write-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tempPath, path)
	}
	return nil
}

func writeExactFile(path string, data []byte, mode os.FileMode) error {
	return writeAtomicFile(path, data, mode)
}

var linearSyncBootstrap = func(context.Context, *bootstrapEngine) error {
	return fmt.Errorf("GOLC_BOOTSTRAP_LINEAR_SYNC_NOT_IMPLEMENTED")
}
