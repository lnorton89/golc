// testscopes.go serves golc_list_test_scopes: every valid value for
// "test --quick --scope <name>", discovered the same way test.go resolves
// one at run time — Go quick-test scopes are TestScope{PascalName} marker
// functions somewhere in *_test.go (test.go's scopeTestMarker capitalizes
// only the first letter of each hyphen segment, so the mapping back to a
// scope name is unambiguous), and Node scopes are whatever
// MustDeclareNodeScope registrations internal/command declares. This is a
// static source scan rather than a call into the registry (declaredNodeScopes
// is unexported and the Go scopes aren't centrally registered at all), so
// it is documented as best-effort/derived rather than authoritative.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	goTestScopeMarkerPattern = regexp.MustCompile(`^func (TestScope[A-Z][A-Za-z0-9]*)\(`)
	nodeScopeBlockStart      = regexp.MustCompile(`MustDeclareNodeScope\(NodeScopeRegistration\{`)
	nodeScopeFieldPattern    = regexp.MustCompile(`^\s*Scope:\s*"([a-z0-9-]+)"`)
)

var skippedDirNames = map[string]bool{
	".git": true, ".tools": true, "node_modules": true, "dist": true,
	".vscode": true, ".idea": true, "site": true, "frontend": true,
}

type listTestScopesInput struct{}

type testScope struct {
	Scope  string `json:"scope"`
	Kind   string `json:"kind"` // "go" or "node"
	Marker string `json:"marker,omitempty"`
	File   string `json:"file"`
}

type listTestScopesOutput struct {
	Scopes []testScope `json:"scopes"`
	Note   string      `json:"note"`
}

func handleListTestScopes(_ context.Context, _ *mcp.CallToolRequest, _ listTestScopesInput) (*mcp.CallToolResult, listTestScopesOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[listTestScopesOutput](err)
	}

	var scopes []testScope
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedDirNames[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relPath = path
		}
		relPath = filepath.ToSlash(relPath)

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files rather than failing the whole scan
		}
		found := scanFileForTestScopes(string(data), relPath)
		scopes = append(scopes, found...)
		return nil
	})
	if walkErr != nil {
		return toolError[listTestScopesOutput](fmt.Errorf("scan repository for test scopes: %w", walkErr))
	}

	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Kind != scopes[j].Kind {
			return scopes[i].Kind < scopes[j].Kind
		}
		return scopes[i].Scope < scopes[j].Scope
	})

	return nil, listTestScopesOutput{
		Scopes: scopes,
		Note: "Derived by scanning source for TestScope{PascalName} marker functions (Go scopes, valid for " +
			"\"test --quick --scope <name>\") and MustDeclareNodeScope registrations (Node scopes). This mirrors " +
			"test.go's own resolution logic but is not a live call into the CLI; confirm with " +
			"\"golc.ps1 test --quick --scope <name>\" if precision matters.",
	}, nil
}

func scanFileForTestScopes(content, relPath string) []testScope {
	var found []testScope
	inNodeBlock := false
	for _, line := range strings.Split(content, "\n") {
		if match := goTestScopeMarkerPattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			marker := match[1]
			found = append(found, testScope{
				Scope:  markerToScopeName(marker),
				Kind:   "go",
				Marker: marker,
				File:   relPath,
			})
			continue
		}
		if nodeScopeBlockStart.MatchString(line) {
			inNodeBlock = true
			continue
		}
		if inNodeBlock {
			if match := nodeScopeFieldPattern.FindStringSubmatch(line); match != nil {
				found = append(found, testScope{Scope: match[1], Kind: "node", File: relPath})
			}
			if strings.Contains(line, "})") {
				inNodeBlock = false
			}
		}
	}
	return found
}

// markerToScopeName inverts test.go's scopeTestMarker: each segment after
// "TestScope" begins with exactly one uppercase letter (the forward
// direction only ever capitalizes a lowercase-alphanumeric segment's first
// character), so splitting at uppercase-letter boundaries and lowercasing
// recovers the original hyphenated scope name.
func markerToScopeName(marker string) string {
	suffix := strings.TrimPrefix(marker, "TestScope")
	var segments []string
	var current strings.Builder
	for i, r := range suffix {
		if i > 0 && r >= 'A' && r <= 'Z' {
			segments = append(segments, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	for i, segment := range segments {
		segments[i] = strings.ToLower(segment)
	}
	return strings.Join(segments, "-")
}
