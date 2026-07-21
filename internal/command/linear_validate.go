// linear_validate.go is the sole owner of the exact `linear validate
// --offline` route (CONTEXT D-03). Its initial handler is compile-safe
// against only the Plan 08 catalog model/parser/validator:
// catalog.BuildCatalog already runs the complete identity, hierarchy
// (parent/cycle/structural containment), source-containment, and
// authority-split validation before returning, so this handler reports
// that outcome and emits only allowlisted catalog fields. Plan 22 extends
// this same handler with map/schema reconciliation checks without
// redeclaring the route.
package command

import (
	"encoding/json"
	"fmt"

	"github.com/lnorton89/golc/internal/trace/catalog"
)

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear validate",
	Summary: "Validate the complete offline repository-owned planning identity catalog: linear validate --offline.",
	Handler: runLinearValidate,
})

// linearValidateView is the allowlisted JSON summary of one successful
// offline catalog validation: entity counts per kind plus the full
// deterministic entity list, matching the "linear catalog" projection.
type linearValidateView struct {
	Status   string              `json:"status"`
	Counts   map[string]int      `json:"counts"`
	Entities []catalogEntityView `json:"entities"`
}

// parseOfflineArgs accepts exactly the supported offline form: --offline.
func parseOfflineArgs(usage string, args []string) error {
	offline := false
	for _, argument := range args {
		if argument != "--offline" {
			return fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
		offline = true
	}
	if !offline {
		return fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return nil
}

// catalogCounts tallies entities per kind for the allowlisted summary.
func catalogCounts(built *catalog.Catalog) map[string]int {
	counts := map[string]int{}
	for _, entity := range built.Entities {
		counts[string(entity.Kind)]++
	}
	return counts
}

// runLinearValidate serves the self-registered "linear validate" route.
// BuildCatalog's internal Validate call is the complete offline check
// (identity, graph, source, and authority constraints); a validation
// failure surfaces its stable GOLC_CATALOG_* diagnostic directly.
func runLinearValidate(request Request) Result {
	if err := parseOfflineArgs("linear validate --offline", request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	built, err := catalog.BuildCatalog(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	view := linearValidateView{
		Status:   "ok",
		Counts:   catalogCounts(built),
		Entities: catalogEntityViews(built),
	}
	payload, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_VALIDATE_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}
