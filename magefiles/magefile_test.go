//go:build mage

package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/bootstrap"
	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/delivery"
)

type fakeCommandExecutor struct {
	requests []command.Request
	fail     string
}

func (fake *fakeCommandExecutor) Execute(request command.Request) command.Result {
	fake.requests = append(fake.requests, request)
	invocation := strings.Join(request.Args, " ")
	result := command.Result{Stdout: []byte(invocation + "\n"), Stderr: []byte("stderr:" + invocation + "\n")}
	if invocation == fake.fail {
		result.ExitCode = 1
	}
	return result
}

func installTargetTestRuntime(t *testing.T, root string, fake *fakeCommandExecutor) (*bytes.Buffer, *bytes.Buffer, *[]string) {
	t.Helper()
	var established []string
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	previous := activeTargetRuntime
	activeTargetRuntime = targetRuntime{
		getenv: func(name string) string {
			if name == "GOLC_PROJECT_ROOT" {
				return filepath.Join(root, "stale")
			}
			return ""
		},
		getwd: func() (string, error) { return root, nil },
		setenv: func(name, value string) error {
			if name == "GOLC_PROJECT_ROOT" {
				established = append(established, value)
			}
			return nil
		},
		bootstrap: func(_ context.Context, gotRoot string, options bootstrap.Options) error {
			require.Equal(t, bootstrap.Options{}, options, "bootstrap options")
			established = append(established, "bootstrap:"+gotRoot)
			return nil
		},
		newRegistry: func() (commandExecutor, error) { return fake, nil },
		loadPRGraph: delivery.LoadPRGraph,
		runGraph:    delivery.Run,
		stdout:      stdout,
		stderr:      stderr,
	}
	t.Cleanup(func() { activeTargetRuntime = previous })
	return stdout, stderr, &established
}

func TestTargetMappingsAndProjectRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte("schema_version = 2\n"), 0o644))
	fake := &fakeCommandExecutor{}
	_, _, established := installTargetTestRuntime(t, root, fake)
	absoluteRoot, err := filepath.Abs(root)
	require.NoError(t, err)
	// magefile.go's own root resolution runs filepath.EvalSymlinks after
	// filepath.Abs, so this comparison must too -- otherwise a CI runner
	// whose temp directory sits behind an OS-level indirection (a
	// Windows short/8.3 alias for a long account name like GitHub
	// Actions' "runneradmin" becoming "RUNNER~1") produces two textually
	// different strings for the exact same directory (observed live in
	// cross-platform-mage.yml run 30077193060 on windows-latest). This
	// mirrors the identical fix already applied to
	// internal/bootstrap/engine_test.go's writeEngineRepository.
	if resolved, err := filepath.EvalSymlinks(absoluteRoot); err == nil {
		absoluteRoot = resolved
	}

	targets := []struct {
		name string
		call func() error
	}{
		{"generate", Generate},
		{"generatecheck", GenerateCheck},
		{"check", Check},
		{"checkoffline", CheckOffline},
		{"build", Build},
		{"test", Test},
		{"package", Package},
		{"packagefoundation", PackageFoundation},
	}
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			fake.requests = nil
			require.NoError(t, target.call(), target.name)
			require.Len(t, fake.requests, 1, "requests = %+v", fake.requests)
			request := fake.requests[0]
			descriptor, ok := delivery.LookupMageTarget(target.name)
			require.True(t, ok, "shared descriptor %q not found", target.name)
			wantArgs := append([]string{descriptor.Route}, descriptor.Args...)
			require.Equal(t, strings.Join(wantArgs, " "), strings.Join(request.Args, " "), "invocation, want shared descriptor")
			require.Equal(t, absoluteRoot, request.Root, "request root")
		})
	}

	require.NoError(t, Bootstrap(context.Background()), "Bootstrap")
	require.Equal(t, "bootstrap:"+absoluteRoot, (*established)[len(*established)-1], "bootstrap root record")
	for _, value := range *established {
		if strings.HasPrefix(value, "bootstrap:") {
			continue
		}
		require.Equal(t, absoluteRoot, value, "established GOLC_PROJECT_ROOT")
	}
}

func TestBootstrapEnvironmentOption(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte("schema_version = 2\n"), 0o644))
	previous := activeTargetRuntime
	t.Cleanup(func() { activeTargetRuntime = previous })

	value := ""
	var options []bootstrap.Options
	registryCalls := 0
	graphCalls := 0
	runCalls := 0
	activeTargetRuntime = targetRuntime{
		getenv: func(name string) string {
			if name == delivery.BootstrapEnvironmentName {
				return value
			}
			return ""
		},
		getwd:  func() (string, error) { return root, nil },
		setenv: func(string, string) error { return nil },
		bootstrap: func(_ context.Context, _ string, got bootstrap.Options) error {
			options = append(options, got)
			return nil
		},
		newRegistry: func() (commandExecutor, error) {
			registryCalls++
			return &fakeCommandExecutor{}, nil
		},
		loadPRGraph: func(gotRoot string) (delivery.Graph, error) {
			graphCalls++
			return delivery.Graph{
				Root:  gotRoot,
				Steps: []delivery.Step{{Name: "01-bootstrap", Route: "bootstrap"}},
			}, nil
		},
		runGraph: func(graph delivery.Graph, execute delivery.StepExecutor) ([]delivery.StepResult, error) {
			runCalls++
			return delivery.Run(graph, execute)
		},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}

	require.NoError(t, Bootstrap(context.Background()), "unset Bootstrap")
	require.Len(t, options, 1, "unset options = %+v", options)
	require.Equal(t, bootstrap.Options{}, options[0], "unset options = %+v", options)

	value = delivery.BootstrapEnvironmentEnablingValue
	require.NoError(t, Bootstrap(context.Background()), "enabled Bootstrap")
	require.Len(t, options, 2, "enabled options = %+v", options)
	require.True(t, options[1].IncludeLinearSync, "enabled options = %+v", options)

	options = nil
	graphCalls, runCalls, registryCalls = 0, 0, 0
	require.NoError(t, Pr(context.Background()), "enabled Pr")
	require.Len(t, options, 1, "Pr effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
	require.True(t, options[0].IncludeLinearSync, "Pr effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
	require.Equal(t, 1, graphCalls, "Pr effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
	require.Equal(t, 1, runCalls, "Pr effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
	require.Equal(t, 1, registryCalls, "Pr effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)

	for _, invalid := range []string{"0", "true", " 1", "1 "} {
		t.Run("invalid_"+strings.ReplaceAll(invalid, " ", "_"), func(t *testing.T) {
			value = invalid
			options = nil
			graphCalls, runCalls, registryCalls = 0, 0, 0
			for name, call := range map[string]func() error{
				"Bootstrap": func() error { return Bootstrap(context.Background()) },
				"Pr":        func() error { return Pr(context.Background()) },
			} {
				err := call()
				require.ErrorContains(t, err, "GOLC_MAGE_BOOTSTRAP_OPTION", "%s invalid option error", name)
			}
			require.Len(t, options, 0, "invalid option caused effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
			require.Equal(t, 0, graphCalls, "invalid option caused effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
			require.Equal(t, 0, runCalls, "invalid option caused effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
			require.Equal(t, 0, registryCalls, "invalid option caused effects options=%+v graph=%d run=%d registry=%d", options, graphCalls, runCalls, registryCalls)
		})
	}
}

func TestTargetOutputFailureAndPRAuthority(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte("schema_version = 2\n"), 0o644))
	fake := &fakeCommandExecutor{fail: "build"}
	stdout, stderr, _ := installTargetTestRuntime(t, root, fake)

	err := Build()
	require.Error(t, err, "non-zero route result must return an error")
	require.Equal(t, "build\n", stdout.String(), "output not preserved")
	require.Equal(t, "stderr:build\n", stderr.String(), "output not preserved")

	stdout.Reset()
	stderr.Reset()
	fake.requests = nil
	loadCalls := 0
	runCalls := 0
	activeTargetRuntime.loadPRGraph = func(gotRoot string) (delivery.Graph, error) {
		loadCalls++
		return delivery.Graph{
			Root: gotRoot,
			Inventory: delivery.CommandInventory{
				CLIBinary: ".tools/cli", GoVersion: "1.26.5",
			},
			Steps: []delivery.Step{
				{Name: "01-generate", Route: "generate", Args: []string{"--check"}},
				{Name: "02-bootstrap", Route: "bootstrap"},
				{Name: "03-build", Route: "build"},
				{Name: "04-test", Route: "test"},
			},
		}, nil
	}
	activeTargetRuntime.runGraph = func(graph delivery.Graph, execute delivery.StepExecutor) ([]delivery.StepResult, error) {
		runCalls++
		return delivery.Run(graph, execute)
	}

	err = Pr(context.Background())
	require.ErrorContains(t, err, "03-build", "Pr failure, want failed configured step")
	require.Equal(t, 1, loadCalls, "LoadPRGraph calls, want 1")
	require.Equal(t, 1, runCalls, "Run calls, want 1")
	var got []string
	for _, request := range fake.requests {
		got = append(got, strings.Join(request.Args, " "))
	}
	require.Equal(t, "generate --check,build", strings.Join(got, ","), "registry order; test must not run after build failure")
	require.Contains(t, stdout.String(), "generate --check", "prior step output missing")
}

func TestMagefileExportsAndImports(t *testing.T) {
	sourcePath := filepath.Join("magefile.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), sourcePath, nil, parser.ImportsOnly)
	require.NoError(t, err, "parse magefile.go")
	var imports []string
	for _, spec := range parsed.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	for _, forbidden := range []string{"os/exec", "syscall"} {
		require.NotContains(t, imports, forbidden, "magefile.go must not import process execution package %q", forbidden)
	}
	for _, required := range []string{
		"github.com/lnorton89/golc/internal/bootstrap",
		"github.com/lnorton89/golc/internal/command",
		"github.com/lnorton89/golc/internal/delivery",
	} {
		assert.Contains(t, imports, required, "magefile.go missing Go API import %q", required)
	}

	parsed, err = parser.ParseFile(token.NewFileSet(), sourcePath, nil, 0)
	require.NoError(t, err)
	var exports []string
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && ast.IsExported(function.Name.Name) {
			exports = append(exports, function.Name.Name)
		}
	}
	sort.Strings(exports)
	want := []string{"Bootstrap", "Build", "Check", "CheckOffline", "Dev", "Generate", "GenerateCheck", "Lint", "Package", "PackageFoundation", "Pr", "Run", "Test", "TestQuick"}
	sort.Strings(want)
	require.Equal(t, want, exports, "exported functions")

	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
			continue
		}
		calls := 0
		var literals []string
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				if identifier, ok := value.Fun.(*ast.Ident); ok && identifier.Name == "runTarget" {
					calls++
				}
			case *ast.BasicLit:
				if value.Kind == token.STRING {
					literals = append(literals, strings.Trim(value.Value, `"`))
				}
			}
			return true
		})
		wantName := strings.ToLower(function.Name.Name)
		msg := "%s must delegate once to runTarget(%q) with no embedded route/argument table; calls=%d literals=%v"
		require.Equal(t, 1, calls, msg, function.Name.Name, wantName, calls, literals)
		require.Len(t, literals, 1, msg, function.Name.Name, wantName, calls, literals)
		require.Equal(t, wantName, literals[0], msg, function.Name.Name, wantName, calls, literals)
	}
}
