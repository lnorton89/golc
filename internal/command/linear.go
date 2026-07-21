// linear.go is the linear command file: it owns the "linear" routing scope
// and self-registers the offline catalog inspection route (CONTEXT D-03,
// D-11, D-14). It reads only committed repository planning artifacts
// through internal/trace/catalog; no network, Node, SDK, or Linear
// credential access is reachable from this route.
package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lnorton89/golc/internal/trace/catalog"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "linear",
	Summary: "Repository-owned planning identity catalog and Linear reconciliation operations.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear catalog",
	Summary: "Print the offline repository-owned planning identity catalog as deterministic JSON: linear catalog --offline --format json.",
	Handler: runLinearCatalog,
})

// catalogEntityView is the allowlisted JSON projection of one catalog
// entity: only durable identity, structure, and repository-relative
// source are emitted, never filesystem-absolute paths or remote state
// (T-01-23: information disclosure).
type catalogEntityView struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Parent  string `json:"parent,omitempty"`
	Display string `json:"display"`
	Source  string `json:"source"`
}

// catalogView is the deterministic JSON envelope for offline catalog
// output: entity order matches BuildCatalog's deterministic build order.
type catalogView struct {
	Entities []catalogEntityView `json:"entities"`
}

// parseOfflineJSONArgs accepts exactly the supported offline JSON form:
// --offline --format json.
func parseOfflineJSONArgs(usage string, args []string) error {
	offline := false
	format := ""
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--offline":
			offline = true
			i++
		case argument == "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("GOLC_LINEAR_USAGE: --format requires a value; usage: %s", usage)
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--format="):
			format = strings.TrimPrefix(argument, "--format=")
			i++
		default:
			return fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if !offline {
		return fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	if format != "json" {
		return fmt.Errorf("GOLC_LINEAR_FORMAT_UNSUPPORTED: %q is not supported (only json); usage: %s", format, usage)
	}
	return nil
}

// catalogEntityViews projects a built catalog's entities into the
// allowlisted JSON view, preserving deterministic build order.
func catalogEntityViews(built *catalog.Catalog) []catalogEntityView {
	views := make([]catalogEntityView, 0, len(built.Entities))
	for _, entity := range built.Entities {
		views = append(views, catalogEntityView{
			ID:      entity.ID,
			Kind:    string(entity.Kind),
			Parent:  entity.Parent,
			Display: entity.Display,
			Source:  entity.Source,
		})
	}
	return views
}

// runLinearCatalog serves the self-registered "linear catalog" route.
func runLinearCatalog(request Request) Result {
	if err := parseOfflineJSONArgs("linear catalog --offline --format json", request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	built, err := catalog.BuildCatalog(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := json.MarshalIndent(catalogView{Entities: catalogEntityViews(built)}, "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_CATALOG_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}
