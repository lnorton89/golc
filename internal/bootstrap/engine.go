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
	ArchiveURI         string `toml:"archive_uri"`
	ArchiveSHA256      string `toml:"archive_sha256"`
	OfficialHost       string `toml:"official_host"`
	OfficialPathPrefix string `toml:"official_path_prefix"`
}

func (pin manifestPin) sourceURL() string {
	if pin.ArchiveURL != "" {
		return pin.ArchiveURL
	}
	return pin.ArchiveURI
}

type bootstrapManifest struct {
	SchemaVersion int                    `toml:"schema_version"`
	Cache         map[string]string      `toml:"cache"`
	Tools         map[string]manifestPin `toml:"tools"`
	Toolchain     map[string]manifestPin `toml:"toolchain"`
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
	if err := validateManifestForPlatform(document, options); err != nil {
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
		policy: policy, source: source, runner: runner,
	}
	engine.env = mergedEnvironment(layout.Environment().AsMap())
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
	if document.SchemaVersion != 1 {
		return bootstrapManifest{}, OfficialSourcePolicy{}, fmt.Errorf("GOLC_TOOLCHAIN_PARSE: schema_version must be 1")
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
	collect(document.Toolchain)
	return document, OfficialSourcePolicy{Patterns: patterns}, nil
}

func validateManifestForPlatform(document bootstrapManifest, options Options) error {
	names := make([]string, 0, len(document.Tools))
	for name := range document.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !validToolName.MatchString(name) {
			return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: invalid tool name %q", name)
		}
		if err := validatePin("tools."+name, document.Tools[name]); err != nil {
			return err
		}
	}
	goPin, ok := document.Toolchain["go"]
	if !ok {
		return fmt.Errorf("GOLC_GO_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.go]")
	}
	if err := validatePin("toolchain.go", goPin); err != nil {
		return err
	}
	if err := validatePlatformPin("go", goPin); err != nil {
		return err
	}
	if options.IncludeLinearSync {
		nodePin, ok := document.Toolchain["node"]
		if !ok {
			return fmt.Errorf("GOLC_NODE_TOOLCHAIN_MISSING: config/toolchain.toml must pin [toolchain.node]")
		}
		if err := validatePin("toolchain.node", nodePin); err != nil {
			return err
		}
		if err := validatePlatformPin("node", nodePin); err != nil {
			return err
		}
	}
	return nil
}

func validatePin(name string, pin manifestPin) error {
	if strings.TrimSpace(pin.sourceURL()) == "" || strings.TrimSpace(pin.ArchiveSHA256) == "" {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [%s] is missing archive URL or archive_sha256", name)
	}
	normalized, err := normalizeExpectedSHA256(pin.ArchiveSHA256)
	if err != nil || normalized != pin.ArchiveSHA256 {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [%s] has invalid lowercase archive_sha256", name)
	}
	if _, err := archiveSuffix(pin.sourceURL()); err != nil {
		return err
	}
	return nil
}

func validatePlatformPin(tool string, pin manifestPin) error {
	if strings.TrimSpace(pin.Version) == "" {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: [toolchain.%s] is missing version", tool)
	}
	layout, err := platformArchiveLayout(tool, pin.Version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(pin.sourceURL())
	if err != nil {
		return fmt.Errorf("GOLC_TOOLCHAIN_PARSE: invalid %s archive URL: %w", tool, err)
	}
	actual, err := url.PathUnescape(filepath.Base(parsed.Path))
	if err != nil || actual != layout.FileName {
		return fmt.Errorf("GOLC_BOOTSTRAP_PLATFORM_MISMATCH: %s pin names %q, running platform %s requires %q", tool, actual, PlatformKey(), layout.FileName)
	}
	return nil
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
	goPin := engine.document.Toolchain["go"]
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
	if engine.options.IncludeLinearSync {
		return linearSyncBootstrap(ctx, engine)
	}
	return nil
}

func (engine *bootstrapEngine) installPin(pin manifestPin, installDir string) error {
	matches, err := InstalledMatches(installDir, pin.ArchiveSHA256)
	if err != nil {
		return err
	}
	if matches {
		return nil
	}
	archivePath, err := Acquire(engine.policy, engine.source, pin.sourceURL(), pin.ArchiveSHA256, engine.layout.Downloads)
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
		suffix := ""
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		output := filepath.Join(engine.root, ".tools", "installs", "golc_project", "bin", "golc-project"+suffix)
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
		return output, fmt.Errorf("%s: %s: %w", diagnostic, strings.Join(args, " "), err)
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
