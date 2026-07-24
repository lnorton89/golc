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
			if options != (bootstrap.Options{}) {
				t.Fatalf("bootstrap options = %+v", options)
			}
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
	if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte("schema_version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCommandExecutor{}
	_, _, established := installTargetTestRuntime(t, root, fake)
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
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
			if err := target.call(); err != nil {
				t.Fatalf("%s: %v", target.name, err)
			}
			if len(fake.requests) != 1 {
				t.Fatalf("requests = %+v", fake.requests)
			}
			request := fake.requests[0]
			descriptor, ok := delivery.LookupMageTarget(target.name)
			if !ok {
				t.Fatalf("shared descriptor %q not found", target.name)
			}
			wantArgs := append([]string{descriptor.Route}, descriptor.Args...)
			if got, want := strings.Join(request.Args, " "), strings.Join(wantArgs, " "); got != want {
				t.Fatalf("invocation = %q, want shared descriptor %q", got, want)
			}
			if request.Root != absoluteRoot {
				t.Fatalf("request root = %q, want %q", request.Root, absoluteRoot)
			}
		})
	}

	if err := Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if got := (*established)[len(*established)-1]; got != "bootstrap:"+absoluteRoot {
		t.Fatalf("bootstrap root record = %q", got)
	}
	for _, value := range *established {
		if strings.HasPrefix(value, "bootstrap:") {
			continue
		}
		if value != absoluteRoot {
			t.Fatalf("established GOLC_PROJECT_ROOT = %q, want %q", value, absoluteRoot)
		}
	}
}

func TestTargetOutputFailureAndPRAuthority(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "golc.project.toml"), []byte("schema_version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeCommandExecutor{fail: "build"}
	stdout, stderr, _ := installTargetTestRuntime(t, root, fake)

	if err := Build(); err == nil {
		t.Fatal("non-zero route result must return an error")
	}
	if stdout.String() != "build\n" || stderr.String() != "stderr:build\n" {
		t.Fatalf("output not preserved: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

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
				Entrypoint: "golc.ps1", CLIBinary: ".tools/cli", GoVersion: "1.26.5",
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

	err := Pr(context.Background())
	if err == nil || !strings.Contains(err.Error(), "03-build") {
		t.Fatalf("Pr failure = %v, want failed configured step", err)
	}
	if loadCalls != 1 || runCalls != 1 {
		t.Fatalf("LoadPRGraph/Run calls = %d/%d, want 1/1", loadCalls, runCalls)
	}
	var got []string
	for _, request := range fake.requests {
		got = append(got, strings.Join(request.Args, " "))
	}
	if joined := strings.Join(got, ","); joined != "generate --check,build" {
		t.Fatalf("registry order = %q; test must not run after build failure", joined)
	}
	if !strings.Contains(stdout.String(), "generate --check") {
		t.Fatalf("prior step output missing: %q", stdout.String())
	}
}

func TestMagefileExportsAndImports(t *testing.T) {
	sourcePath := filepath.Join("magefile.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), sourcePath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse magefile.go: %v", err)
	}
	var imports []string
	for _, spec := range parsed.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	for _, forbidden := range []string{"os/exec", "syscall"} {
		for _, imported := range imports {
			if imported == forbidden {
				t.Fatalf("magefile.go must not import process execution package %q", forbidden)
			}
		}
	}
	for _, required := range []string{
		"github.com/lnorton89/golc/internal/bootstrap",
		"github.com/lnorton89/golc/internal/command",
		"github.com/lnorton89/golc/internal/delivery",
	} {
		found := false
		for _, imported := range imports {
			found = found || imported == required
		}
		if !found {
			t.Errorf("magefile.go missing Go API import %q", required)
		}
	}

	parsed, err = parser.ParseFile(token.NewFileSet(), sourcePath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var exports []string
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && ast.IsExported(function.Name.Name) {
			exports = append(exports, function.Name.Name)
		}
	}
	sort.Strings(exports)
	want := []string{"Bootstrap", "Build", "Check", "CheckOffline", "Generate", "GenerateCheck", "Package", "PackageFoundation", "Pr", "Test"}
	sort.Strings(want)
	if strings.Join(exports, ",") != strings.Join(want, ",") {
		t.Fatalf("exported functions = %v, want %v", exports, want)
	}

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
		if calls != 1 || len(literals) != 1 || literals[0] != wantName {
			t.Fatalf(
				"%s must delegate once to runTarget(%q) with no embedded route/argument table; calls=%d literals=%v",
				function.Name.Name, wantName, calls, literals)
		}
	}
}
