// fixture.go registers FIXT-01's generated fixture JSON Schema
// (schemas/fixture.schema.json) through the same MustRegisterSchema
// registry model.go's Phase 1 configuration schemas already use: one
// authoritative Go type (internal/fixture.FixtureDefinition), reflected
// through the shared newReflector() configuration, with SchemaVersion an
// enum=1 required field so the schema itself is versioned.
package contracts

import (
	"github.com/invopop/jsonschema"

	"github.com/lnorton89/golc/internal/fixture"
)

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "fixture",
	OutputPath: "schemas/fixture.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&fixture.FixtureDefinition{}) },
})
