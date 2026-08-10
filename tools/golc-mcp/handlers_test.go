package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- roadmap.go ---------------------------------------------------------

func TestPlanCountsByPhase(t *testing.T) {
	text := strings.Join([]string{
		"intro line before any heading",
		"**Plans:** 1/2 plans complete", // no current phase yet: must be ignored
		"### Phase 1: Alpha",
		"**Plans:** 32/32 plans complete",
		"### Phase 2: Beta",
		"no plans line for this phase yet",
		"### Phase 3: Gamma",
		"**Plans:** TBD",
		"### Phase 4: Delta",
		"**Plans:** 0/5 plans executed",
	}, "\n")

	got := planCountsByPhase(text)

	// Phases 2 and 3 have no numeric "N/M plans" line, so they must be
	// absent from the map rather than present with zeroed counts —
	// handleListPhases relies on the "ok" from a map lookup to decide
	// whether to populate PlansCompleted/PlansTotal at all.
	require.Equal(t, map[int][2]int{
		1: {32, 32},
		4: {0, 5},
	}, got)
}

func TestHandleListPhases(t *testing.T) {
	t.Run("parses bullets and merges plan counts", func(t *testing.T) {
		root := t.TempDir()
		writeProtocolFile(t, root, roadmapRelPath, strings.Join([]string{
			"## Phases",
			"",
			"- [x] **Phase 1: Offline Foundation** - Contributors can build offline. (completed 2026-07-21)",
			"- [ ] **Phase 2: Modular Fixtures** - Authors can validate fixtures.",
			"",
			"## Phase Details",
			"",
			"### Phase 1: Offline Foundation",
			"",
			"**Plans:** 32/32 plans complete",
			"",
			"### Phase 2: Modular Fixtures",
			"",
			"**Plans:** TBD",
			"",
		}, "\n"))
		t.Setenv(repoRootEnvName, root)

		_, out, err := handleListPhases(context.Background(), nil, listPhasesInput{})
		require.NoError(t, err)
		require.Equal(t, []phaseSummary{
			{
				Number: 1, Title: "Offline Foundation", Complete: true,
				Summary: "Contributors can build offline.", CompletedDate: "2026-07-21",
				PlansCompleted: 32, PlansTotal: 32,
			},
			{
				Number: 2, Title: "Modular Fixtures", Complete: false,
				Summary: "Authors can validate fixtures.",
			},
		}, out.Phases)
	})

	t.Run("no phase bullets is an error", func(t *testing.T) {
		root := t.TempDir()
		writeProtocolFile(t, root, roadmapRelPath, "## Phases\n\nnothing here\n")
		t.Setenv(repoRootEnvName, root)

		_, _, err := handleListPhases(context.Background(), nil, listPhasesInput{})
		require.ErrorContains(t, err, `no "- [ ] **Phase N: ...**" bullets found`)
	})

	t.Run("missing ROADMAP.md surfaces a read error", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv(repoRootEnvName, root)

		_, _, err := handleListPhases(context.Background(), nil, listPhasesInput{})
		require.ErrorContains(t, err, "read "+roadmapRelPath)
	})
}

func TestHandleGetPhaseDetail(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, roadmapRelPath, strings.Join([]string{
		"## Phase Details",
		"",
		"### Phase 1: Offline Foundation",
		"",
		"**Goal:** Ship offline builds.",
		"**Mode:** mvp",
		"",
		"### Phase 2: Modular Fixtures",
		"",
		"**Goal:** Validate fixtures.",
		"",
		"## Progress",
		"",
		"trailing content that must not leak into phase 2's detail",
	}, "\n"))
	t.Setenv(repoRootEnvName, root)

	t.Run("returns the section bounded by the next heading", func(t *testing.T) {
		_, out, err := handleGetPhaseDetail(context.Background(), nil, getPhaseDetailInput{Phase: 2})
		require.NoError(t, err)
		require.Equal(t, 2, out.Phase)
		require.Equal(t, "Phase 2: Modular Fixtures", out.Heading)
		require.Contains(t, out.Detail, "**Goal:** Validate fixtures.")
		require.NotContains(t, out.Detail, "trailing content", `detail must stop at the next "## " heading`)
	})

	t.Run("non-positive phase number is rejected before touching disk", func(t *testing.T) {
		_, _, err := handleGetPhaseDetail(context.Background(), nil, getPhaseDetailInput{Phase: 0})
		require.ErrorContains(t, err, "must be a positive integer")
	})

	t.Run("unknown phase number is an error", func(t *testing.T) {
		_, _, err := handleGetPhaseDetail(context.Background(), nil, getPhaseDetailInput{Phase: 99})
		require.ErrorContains(t, err, `no "### Phase 99:" section found`)
	})
}

func TestReadRoadmap(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, roadmapRelPath, "# Roadmap: GOLC\n")

	text, err := readRoadmap(root)
	require.NoError(t, err)
	require.Equal(t, "# Roadmap: GOLC\n", text)

	_, err = readRoadmap(t.TempDir())
	require.ErrorContains(t, err, "read "+roadmapRelPath)
}

// --- testscopes.go -------------------------------------------------------

func TestMarkerToScopeName(t *testing.T) {
	tests := []struct {
		marker string
		want   string
	}{
		{"TestScopeConfigStrict", "config-strict"},
		{"TestScopeConfig", "config"},
		// Real registration from internal/command/linear_sync.go: exercises
		// the three-segment case against a name this repo actually uses.
		{"TestScopeLinearSdkOperations", "linear-sdk-operations"},
	}
	for _, tt := range tests {
		t.Run(tt.marker, func(t *testing.T) {
			require.Equal(t, tt.want, markerToScopeName(tt.marker))
		})
	}
}

func TestScanFileForTestScopes(t *testing.T) {
	content := strings.Join([]string{
		"package example",
		"",
		"func TestScopeConfigStrict(t *testing.T) {",
		"}",
		"",
		`var _ = MustDeclareNodeScope(NodeScopeRegistration{`,
		`	Scope:     "linear-sdk-operations",`,
		`	Dir:       linearSyncWorkspaceDir,`,
		`	Marker:    "TestScopeLinearSdkOperations",`,
		`	Arguments: linearSyncNodeTestCommand(),`,
		`})`,
	}, "\n")

	got := scanFileForTestScopes(content, "internal/example_test.go")

	require.Equal(t, []testScope{
		{Scope: "config-strict", Kind: "go", Marker: "TestScopeConfigStrict", File: "internal/example_test.go"},
		{Scope: "linear-sdk-operations", Kind: "node", File: "internal/example_test.go"},
	}, got)
}

func TestHandleListTestScopes(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "internal/example_test.go", strings.Join([]string{
		"package example",
		"",
		"func TestScopeExampleThing(t *testing.T) {}",
	}, "\n"))
	writeProtocolFile(t, root, "internal/nodescopes.go", strings.Join([]string{
		"package example",
		"",
		`var _ = MustDeclareNodeScope(NodeScopeRegistration{`,
		`	Scope:  "widget-build",`,
		`	Marker: "TestScopeWidgetBuild",`,
		`})`,
	}, "\n"))
	// Files under a skipped directory (skippedDirNames) must never surface,
	// even though their content would otherwise match the marker pattern.
	writeProtocolFile(t, root, "node_modules/vendor_test.go", "func TestScopeShouldNeverAppear(t *testing.T) {}\n")
	t.Setenv(repoRootEnvName, root)

	_, out, err := handleListTestScopes(context.Background(), nil, listTestScopesInput{})
	require.NoError(t, err)

	var scopeNames []string
	for _, scope := range out.Scopes {
		scopeNames = append(scopeNames, scope.Scope)
	}
	require.ElementsMatch(t, []string{"example-thing", "widget-build"}, scopeNames)
	require.NotContains(t, scopeNames, "should-never-appear", "node_modules must be skipped per skippedDirNames")
	require.NotEmpty(t, out.Note)
}

// --- schemas.go ------------------------------------------------------------

func TestHandleListSchemas(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "schemas/beta.schema.json",
		`{"$comment":"Beta schema.","$id":"https://golc.dev/schemas/beta.schema.json","type":"object"}`)
	writeProtocolFile(t, root, "schemas/alpha.schema.json", `{"type":"object"}`)
	writeProtocolFile(t, root, "schemas/ignored.txt", "not a schema file")
	t.Setenv(repoRootEnvName, root)

	_, out, err := handleListSchemas(context.Background(), nil, listSchemasInput{})
	require.NoError(t, err)
	require.Equal(t, []schemaSummary{
		{Name: "alpha", File: "schemas/alpha.schema.json"},
		{
			Name: "beta", File: "schemas/beta.schema.json",
			Comment: "Beta schema.", ID: "https://golc.dev/schemas/beta.schema.json",
		},
	}, out.Schemas)
}

func TestHandleListSchemasMissingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv(repoRootEnvName, root)

	_, _, err := handleListSchemas(context.Background(), nil, listSchemasInput{})
	require.ErrorContains(t, err, "read "+schemasRelDir)
}

func TestHandleGetSchema(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "schemas/alpha.schema.json",
		`{"type":"object","properties":{"name":{"type":"string"}}}`)
	t.Setenv(repoRootEnvName, root)

	t.Run("returns the parsed schema", func(t *testing.T) {
		_, out, err := handleGetSchema(context.Background(), nil, getSchemaInput{Name: "alpha"})
		require.NoError(t, err)
		require.Equal(t, "alpha", out.Name)
		require.Equal(t, "schemas/alpha.schema.json", out.File)
		require.Equal(t, "object", out.Schema["type"])
	})

	t.Run("unknown schema name is an error", func(t *testing.T) {
		_, _, err := handleGetSchema(context.Background(), nil, getSchemaInput{Name: "missing"})
		require.ErrorContains(t, err, `no schema named "missing"`)
	})
}

// --- docs.go --------------------------------------------------------------

func TestFirstMarkdownHeading(t *testing.T) {
	dir := t.TempDir()

	withHeading := filepath.Join(dir, "with-heading.md")
	require.NoError(t, os.WriteFile(withHeading, []byte("\n\n# Package command\n\nBody text.\n"), 0o600))
	heading, ok := firstMarkdownHeading(withHeading)
	require.True(t, ok)
	require.Equal(t, "Package command", heading)

	noHeading := filepath.Join(dir, "no-heading.md")
	require.NoError(t, os.WriteFile(noHeading, []byte("## Not a top-level heading\n"), 0o600))
	_, ok = firstMarkdownHeading(noHeading)
	require.False(t, ok, `"## " must not satisfy the "# " prefix match`)

	_, ok = firstMarkdownHeading(filepath.Join(dir, "missing.md"))
	require.False(t, ok, "a missing file must fail closed rather than panic")
}

func TestHandleListReferenceDocs(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "docs/reference/command.md", "# Package command\n\nRoutes and registry.\n")
	writeProtocolFile(t, root, "docs/reference/artnet.md", "No heading on the first line.\n")
	t.Setenv(repoRootEnvName, root)

	_, out, err := handleListReferenceDocs(context.Background(), nil, listReferenceDocsInput{})
	require.NoError(t, err)
	require.Equal(t, []referenceDocSummary{
		{Package: "artnet", File: "docs/reference/artnet.md"},
		{Package: "command", File: "docs/reference/command.md", Title: "Package command"},
	}, out.Docs)
}

func TestHandleGetReferenceDoc(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "docs/reference/command.md", "# Package command\n\nRoutes and registry.\n")
	t.Setenv(repoRootEnvName, root)

	t.Run("returns the full markdown content", func(t *testing.T) {
		_, out, err := handleGetReferenceDoc(context.Background(), nil, getReferenceDocInput{Package: "command"})
		require.NoError(t, err)
		require.Equal(t, "docs/reference/command.md", out.File)
		require.Equal(t, "# Package command\n\nRoutes and registry.\n", out.Content)
	})

	t.Run("unknown package is an error", func(t *testing.T) {
		_, _, err := handleGetReferenceDoc(context.Background(), nil, getReferenceDocInput{Package: "missing"})
		require.ErrorContains(t, err, `no reference doc for package "missing"`)
	})
}

// --- config.go --------------------------------------------------------------

const configFixtureRootIndex = `schema_version = 2

[[concerns]]
id = "runtime"
path = "config/runtime.toml"
`

const configFixtureRuntime = `schema_version = 2

[runtime]
log_level = "info"
`

func TestHandleListConfigConcerns(t *testing.T) {
	// No fixture root needed: this handler reads projectconfig.DefaultSpec(),
	// a compiled-in registry, not anything off disk.
	_, out, err := handleListConfigConcerns(context.Background(), nil, listConfigConcernsInput{})
	require.NoError(t, err)
	require.True(t, sort.SliceIsSorted(out.Concerns, func(i, j int) bool { return out.Concerns[i].ID < out.Concerns[j].ID }))

	var toolchain *configConcernSummary
	for i := range out.Concerns {
		if out.Concerns[i].ID == "toolchain" {
			toolchain = &out.Concerns[i]
		}
	}
	require.NotNil(t, toolchain, "DefaultSpec must declare a toolchain concern")
	require.Equal(t, "config/toolchain.toml", toolchain.Path)
	require.Contains(t, toolchain.Keys, "toolchain.go.version")
	require.True(t, sort.StringsAreSorted(toolchain.Keys), "keys must be sorted for deterministic output")
}

func TestHandleConfigInspect(t *testing.T) {
	t.Run("returns the concern's resolved JSON", func(t *testing.T) {
		root := t.TempDir()
		writeProtocolFile(t, root, "golc.project.toml", configFixtureRootIndex)
		writeProtocolFile(t, root, "config/runtime.toml", configFixtureRuntime)
		t.Setenv(repoRootEnvName, root)

		_, out, err := handleConfigInspect(context.Background(), nil, configInspectInput{Concern: "runtime"})
		require.NoError(t, err)
		require.Equal(t, "runtime", out.Concern)
		runtimeValues, ok := out.Values["runtime"].(map[string]any)
		require.True(t, ok, "expected a nested \"runtime\" object, got %#v", out.Values["runtime"])
		require.Equal(t, "info", runtimeValues["log_level"])
	})

	t.Run("empty concern is rejected before touching disk", func(t *testing.T) {
		_, _, err := handleConfigInspect(context.Background(), nil, configInspectInput{Concern: ""})
		require.ErrorContains(t, err, "concern is required")
	})

	t.Run("unknown concern surfaces InspectConcern's error", func(t *testing.T) {
		root := t.TempDir()
		writeProtocolFile(t, root, "golc.project.toml", configFixtureRootIndex)
		writeProtocolFile(t, root, "config/runtime.toml", configFixtureRuntime)
		t.Setenv(repoRootEnvName, root)

		_, _, err := handleConfigInspect(context.Background(), nil, configInspectInput{Concern: "nonexistent"})
		require.ErrorContains(t, err, "GOLC_CONFIG_CONCERN_UNKNOWN")
	})
}

func TestHandleConfigExplain(t *testing.T) {
	root := t.TempDir()
	writeProtocolFile(t, root, "golc.project.toml", configFixtureRootIndex)
	writeProtocolFile(t, root, "config/runtime.toml", configFixtureRuntime)
	t.Setenv(repoRootEnvName, root)

	t.Run("returns provenance for a writable canonical key", func(t *testing.T) {
		_, out, err := handleConfigExplain(context.Background(), nil, configExplainInput{Key: "runtime.log_level"})
		require.NoError(t, err)
		require.Equal(t, "runtime.log_level", out.Key)
		require.Equal(t, "runtime.log_level", out.Provenance["key"])
		require.Equal(t, "info", out.Provenance["value"])
		require.Equal(t, "committed", out.Provenance["layer"])
	})

	t.Run("empty key is rejected before touching disk", func(t *testing.T) {
		_, _, err := handleConfigExplain(context.Background(), nil, configExplainInput{Key: ""})
		require.ErrorContains(t, err, "key is required")
	})

	t.Run("locked key is rejected", func(t *testing.T) {
		_, _, err := handleConfigExplain(context.Background(), nil, configExplainInput{Key: "schema_version"})
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_LOCKED")
	})

	t.Run("unregistered key is rejected", func(t *testing.T) {
		_, _, err := handleConfigExplain(context.Background(), nil, configExplainInput{Key: "does.not.exist"})
		require.ErrorContains(t, err, "GOLC_CONFIG_LOCAL_KEY_UNKNOWN")
	})
}

// --- commands.go ------------------------------------------------------------

func TestHandleListCommandRoutes(t *testing.T) {
	// No fixture root needed: NewDefaultCommandRegistry builds from
	// internal/command's own self-registration (var-init side effects),
	// not anything resolveRepoRoot would find on disk.
	_, out, err := handleListCommandRoutes(context.Background(), nil, listCommandRoutesInput{})
	require.NoError(t, err)
	require.NotEmpty(t, out.Scopes)
	require.NotEmpty(t, out.Routes)
	require.True(t, sort.SliceIsSorted(out.Scopes, func(i, j int) bool { return out.Scopes[i].Scope < out.Scopes[j].Scope }))
	require.True(t, sort.SliceIsSorted(out.Routes, func(i, j int) bool { return out.Routes[i].Route < out.Routes[j].Route }))

	var routeNames []string
	for _, route := range out.Routes {
		routeNames = append(routeNames, route.Route)
	}
	for _, want := range []string{"build", "check", "generate", "test"} {
		require.Contains(t, routeNames, want, "internal/command's self-registered routes must include %q", want)
	}
}

// --- repo.go: resolveRepoRoot walk-up fallback ------------------------------

func TestResolveRepoRootWalksUpToProjectIndex(t *testing.T) {
	// Empty (not unset) still takes the os.Getenv "" == unset branch in
	// resolveRepoRoot, and t.Setenv restores the real value afterward.
	t.Setenv(repoRootEnvName, "")

	root := t.TempDir()
	writeProtocolFile(t, root, rootIndexFileName, "schema_version = 2\n")
	nested := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := resolveRepoRoot()
	require.NoError(t, err)
	require.Equal(t, mustEvalSymlinks(t, root), mustEvalSymlinks(t, got))
}

func TestResolveRepoRootFallsBackToCWDWhenNoIndexFound(t *testing.T) {
	t.Setenv(repoRootEnvName, "")

	root := t.TempDir()
	nested := filepath.Join(root, "no", "index", "here")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := resolveRepoRoot()
	require.NoError(t, err)
	require.Equal(t, mustEvalSymlinks(t, nested), mustEvalSymlinks(t, got),
		"no golc.project.toml above cwd must fall back to cwd itself, not error")
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(abs)
	require.NoError(t, err)
	return resolved
}
