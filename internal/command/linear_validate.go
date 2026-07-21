// linear_validate.go is the sole owner of the exact `linear validate
// --offline` route (CONTEXT D-03). Its initial handler was compile-safe
// against only the Plan 08 catalog model/parser/validator:
// catalog.BuildCatalog already runs the complete identity, hierarchy
// (parent/cycle/structural containment), source-containment, and
// authority-split validation before returning, so this handler reports
// that outcome and emits only allowlisted catalog fields. Plan 22 (this
// revision) extends this same handler with strict map/schema
// reconciliation checks without redeclaring the route:
//
//   - strict JSON + canonical-map: catalog.CheckMigration re-derives the
//     canonical schema-2 map (which itself strictly decodes the committed
//     .planning/linear-map.json through strictjson.DecodeStrict) and
//     compares it byte-for-byte against the committed file.
//   - catalog-correspondence + authority + source-containment: both
//     CheckMigration and the explicit catalog.BuildCatalog call below run
//     catalog.Validate, covering identity/hierarchy, ValidateSources, and
//     ValidateAuthorities (CONTEXT D-11/D-12).
//   - credential-absence: catalog's own validateMap rejects any pending
//     remote mapping that carries a non-null identity (CONTEXT
//     D-11/D-14/T-01-25).
//   - generated-schema: contracts.CheckDrift confirms the committed
//     schemas/linear-map.schema.json still matches its registered
//     internal/contracts source (T-01-24).
//
// None of these checks re-implement logic that already lives in
// internal/contracts or internal/trace/catalog; this handler only
// composes their existing, already-tested entrypoints.
package command

import (
	"encoding/json"
	"fmt"

	"github.com/lnorton89/golc/internal/contracts"
	"github.com/lnorton89/golc/internal/trace/catalog"
)

// linearMapSchemaOutputPath is the exact "linear-map" schema descriptor
// output path internal/contracts/linear.go registers (CONTEXT D-08); the
// extended offline validate handler scopes its generated-schema drift
// check to only this descriptor so it never fails on an unrelated
// committed configuration schema.
const linearMapSchemaOutputPath = "schemas/linear-map.schema.json"

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
// BuildCatalog's internal Validate call is the complete offline identity
// check (identity, graph, source, and authority constraints); this
// handler additionally reconciles the committed map against its canonical
// re-derivation and confirms its generated schema has not drifted before
// reporting success. A validation failure surfaces its stable
// GOLC_CATALOG_*/GOLC_MIGRATE_*/GOLC_LINEAR_VALIDATE_* diagnostic
// directly; absent remote linkage is never a failure (CONTEXT D-11).
func runLinearValidate(request Request) Result {
	if err := parseOfflineArgs("linear validate --offline", request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	// Generated-schema check (T-01-24): the committed linear-map schema
	// still matches its registered internal/contracts source.
	changed, err := contracts.CheckDrift(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_VALIDATE_SCHEMA: %v\n", err))}
	}
	for _, path := range changed {
		if path == linearMapSchemaOutputPath {
			return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
				"GOLC_LINEAR_VALIDATE_SCHEMA_DRIFT: %s does not match its generated source\n", linearMapSchemaOutputPath))}
		}
	}

	// Strict JSON + canonical-map + catalog-correspondence + authority +
	// source-containment + credential-absence: CheckMigration re-derives
	// the canonical schema-2 map (strictly decoding the committed file,
	// running catalog.Validate, and rejecting any pending mapping that
	// carries a non-null remote identity) and compares it byte-for-byte
	// against .planning/linear-map.json.
	if err := catalog.CheckMigration(request.Root); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
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
