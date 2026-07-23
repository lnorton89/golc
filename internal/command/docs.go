// docs.go owns the "docs" scope and self-registers the exact "docs"
// route (CONTEXT D-03): internal/docgen owns discovery, extraction, and
// rendering; this file only exposes it as a reachable offline command,
// mirroring generate.go's delegation-only shape for internal/contracts.
package command

import (
	"fmt"

	"github.com/lnorton89/golc/internal/docgen"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "docs",
	Summary: "Generated Markdown reference pages from internal package doc comments.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "docs",
	Summary: "Regenerate every internal package's Markdown reference page under docs/reference and site/src/content/reference: docs.",
	Handler: runDocs,
})

// runDocs serves the self-registered "docs" route. It never opens a
// network connection: internal/docgen is pure filesystem/AST code over
// the repository-relative committed paths it writes.
func runDocs(request Request) Result {
	if len(request.Args) != 0 {
		return Result{ExitCode: 2, Stderr: []byte(fmt.Sprintf("GOLC_DOCS_USAGE: docs takes no arguments, got %q\n", request.Args))}
	}

	pages, err := docgen.GenerateAll(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	message := fmt.Sprintf(
		"docs: %d package reference page(s) written to %s and %s.\n",
		len(pages), docgen.ReferenceDocsDir, docgen.SiteReferenceDir,
	)
	return Result{Stdout: []byte(message)}
}
