// docs.go serves golc_list_reference_docs and golc_get_reference_doc over
// the generated docs/reference/*.md package documentation ("golc.ps1
// docs" regenerates these from Go doc comments). Reading the committed
// Markdown is far cheaper for a caller than reading source directly, and
// stays exactly in sync with it since the docs command is part of the
// offline core graph. Requested names are resolved against a directory
// listing, never joined into a path directly, so a caller can't read
// outside docs/reference/.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const referenceDocsRelDir = "docs/reference"

type listReferenceDocsInput struct{}

type referenceDocSummary struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Title   string `json:"title,omitempty"`
}

type listReferenceDocsOutput struct {
	Docs []referenceDocSummary `json:"docs"`
}

func handleListReferenceDocs(_ context.Context, _ *mcp.CallToolRequest, _ listReferenceDocsInput) (*mcp.CallToolResult, listReferenceDocsOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[listReferenceDocsOutput](err)
	}
	dir := filepath.Join(root, filepath.FromSlash(referenceDocsRelDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return toolError[listReferenceDocsOutput](fmt.Errorf("read %s: %w", referenceDocsRelDir, err))
	}

	var out listReferenceDocsOutput
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		pkg := strings.TrimSuffix(entry.Name(), ".md")
		summary := referenceDocSummary{Package: pkg, File: referenceDocsRelDir + "/" + entry.Name()}
		if title, ok := firstMarkdownHeading(filepath.Join(dir, entry.Name())); ok {
			summary.Title = title
		}
		out.Docs = append(out.Docs, summary)
	}
	sort.Slice(out.Docs, func(i, j int) bool { return out.Docs[i].Package < out.Docs[j].Package })

	return nil, out, nil
}

func firstMarkdownHeading(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if heading, found := strings.CutPrefix(strings.TrimSpace(line), "# "); found {
			return heading, true
		}
	}
	return "", false
}

type getReferenceDocInput struct {
	Package string `json:"package" jsonschema:"internal package name, e.g. \"command\" or \"projectconfig\" (see golc_list_reference_docs)"`
}

type getReferenceDocOutput struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Content string `json:"content"`
}

func handleGetReferenceDoc(_ context.Context, _ *mcp.CallToolRequest, input getReferenceDocInput) (*mcp.CallToolResult, getReferenceDocOutput, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return toolError[getReferenceDocOutput](err)
	}
	dir := filepath.Join(root, filepath.FromSlash(referenceDocsRelDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return toolError[getReferenceDocOutput](fmt.Errorf("read %s: %w", referenceDocsRelDir, err))
	}

	var fileName string
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() == input.Package+".md" {
			fileName = entry.Name()
			break
		}
	}
	if fileName == "" {
		return toolError[getReferenceDocOutput](fmt.Errorf("no reference doc for package %q under %s; call golc_list_reference_docs for the available names", input.Package, referenceDocsRelDir))
	}

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		return toolError[getReferenceDocOutput](fmt.Errorf("read %s/%s: %w", referenceDocsRelDir, fileName, err))
	}

	return nil, getReferenceDocOutput{Package: input.Package, File: referenceDocsRelDir + "/" + fileName, Content: string(data)}, nil
}
