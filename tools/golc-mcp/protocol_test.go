package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mageProtocolTarget struct {
	Name               string                          `json:"name"`
	Kind               string                          `json:"kind"`
	Route              string                          `json:"route"`
	Args               []string                        `json:"args"`
	Authority          string                          `json:"authority"`
	EnvironmentOptions []mageProtocolEnvironmentOption `json:"environment_options"`
	PR                 *mageProtocolPR                 `json:"pr,omitempty"`
}

type mageProtocolEnvironmentOption struct {
	Name          string `json:"name"`
	EnablingValue string `json:"enabling_value"`
	Effect        string `json:"effect"`
}

type mageProtocolPR struct {
	AuthorityFile  string               `json:"authority_file"`
	AuthorityKeys  []string             `json:"authority_keys"`
	MutationPolicy string               `json:"mutation_policy"`
	Steps          []mageProtocolPRStep `json:"steps"`
}

type mageProtocolPRStep struct {
	Name    string   `json:"name"`
	Route   string   `json:"route"`
	Args    []string `json:"args"`
	Network string   `json:"network"`
}

func TestMCPProtocolReadOnlyInventoryAndCalls(t *testing.T) {
	root := t.TempDir()
	writeProtocolFixture(t, root)
	t.Setenv(repoRootEnvName, root)

	session := connectProtocolClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listed, err := session.ListTools(ctx, nil)
	require.NoError(t, err, "ListTools")
	wantNames := []string{
		"golc_config_explain",
		"golc_config_inspect",
		"golc_get_phase_detail",
		"golc_get_reference_doc",
		"golc_get_schema",
		"golc_list_command_routes",
		"golc_list_config_concerns",
		"golc_list_mage_targets",
		"golc_list_phases",
		"golc_list_reference_docs",
		"golc_list_schemas",
		"golc_list_test_scopes",
		"golc_project_status",
	}
	var gotNames []string
	toolsByName := make(map[string]*mcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		gotNames = append(gotNames, tool.Name)
		toolsByName[tool.Name] = tool
		require.NotNil(t, tool.Annotations, "%s has no annotations", tool.Name)
		require.True(t, tool.Annotations.ReadOnlyHint, "%s annotations = %+v, want read-only and idempotent", tool.Name, tool.Annotations)
		require.True(t, tool.Annotations.IdempotentHint, "%s annotations = %+v, want read-only and idempotent", tool.Name, tool.Annotations)
		require.NotNil(t, tool.Annotations.OpenWorldHint, "%s annotations = %+v, want closed world", tool.Name, tool.Annotations)
		require.False(t, *tool.Annotations.OpenWorldHint, "%s annotations = %+v, want closed world", tool.Name, tool.Annotations)
		for _, stale := range []string{`Every "golc.ps1`, `identical to "golc.ps1`} {
			assert.NotContains(t, tool.Description, stale, "%s description presents golc.ps1 as permanent API identity: %q", tool.Name, tool.Description)
		}
	}
	sort.Strings(gotNames)
	require.Equal(t, wantNames, gotNames, "tool names")
	require.Contains(t, toolsByName["golc_list_mage_targets"].Description, "sole contributor entrypoint", "Mage tool description does not explain Mage's entrypoint authority")

	status := callProtocolTool(t, ctx, session, "golc_project_status")
	assertJSONValue(t, status, "last_activity", "2026-07-24")
	assertJSONValue(t, status, "last_activity_desc", "Completed quick task task-b: protocol body")
	assertJSONValue(t, status, "activity_source", "current_position_body")
	assertJSONValue(t, status, "activity_drift.detected", true)
	assertJSONValue(t, status, "activity_drift.frontmatter.description", "Completed quick task task-a: stale frontmatter")

	mageMap := callProtocolTool(t, ctx, session, "golc_list_mage_targets")
	var mage struct {
		Targets []mageProtocolTarget `json:"targets"`
	}
	decodeStructured(t, mageMap, &mage)

	wantTargets := []mageProtocolTarget{
		{
			Name: "bootstrap", Kind: "bootstrap", Args: []string{}, Authority: "internal/bootstrap.Bootstrap",
			EnvironmentOptions: []mageProtocolEnvironmentOption{{
				Name: "GOLC_BOOTSTRAP_INCLUDE_LINEAR_SYNC", EnablingValue: "1",
				Effect: "bootstrap.Options.IncludeLinearSync",
			}},
		},
		{Name: "build", Kind: "route", Route: "build", Args: []string{}, Authority: "internal/command registry"},
		{Name: "check", Kind: "route", Route: "check", Args: []string{"--concern", "project"}, Authority: "internal/command registry"},
		{Name: "checkoffline", Kind: "route", Route: "check", Args: []string{"--offline"}, Authority: "internal/command registry"},
		{Name: "designsystembrowser", Kind: "route", Route: "designsystem", Args: []string{"--browser"}, Authority: "internal/command registry"},
		{Name: "dev", Kind: "route", Route: "dev", Args: []string{}, Authority: "internal/command registry"},
		{Name: "generate", Kind: "route", Route: "generate", Args: []string{}, Authority: "internal/command registry"},
		{Name: "generatecheck", Kind: "route", Route: "generate", Args: []string{"--check"}, Authority: "internal/command registry"},
		{Name: "lint", Kind: "route", Route: "lint", Args: []string{}, Authority: "internal/command registry"},
		{Name: "package", Kind: "route", Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
		{Name: "packagefoundation", Kind: "route", Route: "package", Args: []string{"--foundation"}, Authority: "internal/command registry"},
		{
			Name: "pr", Kind: "pr", Args: []string{},
			Authority: "config/commands.toml: commands.pr.steps, commands.pr.network_steps, commands.pr.mutation_steps",
			PR: &mageProtocolPR{
				AuthorityFile: "config/commands.toml",
				AuthorityKeys: []string{
					"commands.pr.steps",
					"commands.pr.network_steps",
					"commands.pr.mutation_steps",
				},
				MutationPolicy: "none",
				Steps: []mageProtocolPRStep{
					{Name: "01-bootstrap", Route: "bootstrap", Args: []string{}, Network: "allowed"},
					{Name: "02-generate---check", Route: "generate", Args: []string{"--check"}, Network: "denied"},
					{Name: "03-check---offline", Route: "check", Args: []string{"--offline"}, Network: "denied"},
					{Name: "04-build", Route: "build", Args: []string{}, Network: "denied"},
					{Name: "05-test---quick", Route: "test", Args: []string{"--quick"}, Network: "denied"},
					{Name: "06-package---foundation", Route: "package", Args: []string{"--foundation"}, Network: "denied"},
				},
			},
		},
		{Name: "run", Kind: "route", Route: "run", Args: []string{}, Authority: "internal/command registry"},
		{Name: "test", Kind: "route", Route: "test", Args: []string{}, Authority: "internal/command registry"},
		{Name: "testquick", Kind: "route", Route: "test", Args: []string{"--quick"}, Authority: "internal/command registry"},
	}
	for i := range wantTargets {
		if wantTargets[i].EnvironmentOptions == nil {
			wantTargets[i].EnvironmentOptions = []mageProtocolEnvironmentOption{}
		}
	}
	require.Equal(t, wantTargets, mage.Targets, "Mage targets mismatch")
}

func TestMCPProductionSourcesCannotExecute(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err, "read package directory")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		require.NoError(t, err, "parse %s", entry.Name())
		imports := map[string]string{}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			name := filepath.Base(path)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			imports[name] = path
			switch {
			case path == "os/exec", path == "syscall", strings.Contains(path, "/magefiles"):
				assert.Fail(t, fmt.Sprintf("%s imports forbidden execution package %q", entry.Name(), path))
			case path == "github.com/lnorton89/golc/internal/bootstrap":
				assert.Fail(t, fmt.Sprintf("%s imports bootstrap execution authority", entry.Name()))
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch function := call.Fun.(type) {
			case *ast.SelectorExpr:
				qualifier, _ := function.X.(*ast.Ident)
				importPath := ""
				if qualifier != nil {
					importPath = imports[qualifier.Name]
				}
				if function.Sel.Name == "Execute" {
					assert.Fail(t, fmt.Sprintf("%s calls command execution method .Execute", entry.Name()))
				}
				if importPath == "github.com/lnorton89/golc/internal/delivery" &&
					(function.Sel.Name == "Run" || function.Sel.Name == "RunOffline") {
					assert.Fail(t, fmt.Sprintf("%s calls delivery.%s", entry.Name(), function.Sel.Name))
				}
				if importPath == "os" && function.Sel.Name == "StartProcess" {
					assert.Fail(t, fmt.Sprintf("%s calls os.StartProcess", entry.Name()))
				}
			case *ast.Ident:
				switch function.Name {
				case "Bootstrap", "Build", "Check", "CheckOffline", "Generate", "GenerateCheck", "Package", "PackageFoundation", "Pr", "Test", "TestQuick":
					assert.Fail(t, fmt.Sprintf("%s calls execution path %s", entry.Name(), function.Name))
				}
			}
			return true
		})
	}
}

// TestMCPDescriptionsHaveNoGolcPs1References guards against golc.ps1
// language creeping back into this server's tool descriptions or README
// now that golc.ps1 itself has been deleted (PowerShell removal plan
// Step 7): Mage is the sole contributor entrypoint, not a "configured" or
// "currently retained compatibility" one.
func TestMCPDescriptionsHaveNoGolcPs1References(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	require.NoError(t, err)
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)
	combined := string(mainSource) + "\n" + string(readme)
	for _, stale := range []string{
		"golc.ps1",
		"configured contributor entrypoint",
		"configured entrypoint",
		"compatibility entrypoint",
	} {
		assert.NotContains(t, combined, stale, "golc-mcp documentation still references %q; golc.ps1 no longer exists", stale)
	}
	for _, required := range []string{
		"sole contributor entrypoint",
		".mcp.json",
		"go build -o tools/golc-mcp/golc-mcp.exe ./tools/golc-mcp",
	} {
		assert.Contains(t, combined, required, "golc-mcp documentation missing %q", required)
	}
}

func connectProtocolClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	server := mcp.NewServer(&mcp.Implementation{Name: "golc-mcp", Version: serverVersion}, nil)
	registerTools(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err, "connect server")
	client := mcp.NewClient(&mcp.Implementation{Name: "golc-mcp-test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
	}
	require.NoError(t, err, "connect client")
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}

func callProtocolTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string) map[string]any {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
	require.NoError(t, err, "CallTool(%s)", name)
	require.False(t, result.IsError, "CallTool(%s) returned tool error: %+v", name, result.Content)
	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err, "marshal %s structured content", name)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded), "decode %s structured content", name)
	return decoded
}

func decodeStructured(t *testing.T, source any, destination any) {
	t.Helper()
	data, err := json.Marshal(source)
	require.NoError(t, err, "marshal structured value")
	require.NoError(t, json.Unmarshal(data, destination), "decode structured value")
}

func writeProtocolFixture(t *testing.T, root string) {
	t.Helper()
	state := projectStatusFrontmatter + `
## Current Position

Phase: 06 (Wails Authoring and Operator Surface) — EXECUTING
Last activity: 2026-07-24 — Completed quick task task-b: protocol body

## Performance Metrics
`
	writeProtocolFile(t, root, ".planning/STATE.md", state)
	writeProtocolFile(t, root, "config/commands.toml", `schema_version = 2

[commands]
cli_binary = ".tools/installs/golc_project"
go_version = "1.26.5"

[commands.pr]
steps = "bootstrap,generate --check,check --offline,build,test --quick,package --foundation"
network_steps = "bootstrap"
mutation_steps = "none"
`)
}

func writeProtocolFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755), "mkdir %s", relative)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600), "write %s", relative)
}
