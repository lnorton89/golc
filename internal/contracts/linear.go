// linear.go registers the strict Draft 2020-12 "linear-map" contract
// (CONTEXT D-08, D-11, D-12, D-14): the schema-2 canonical shape of
// .planning/linear-map.json that internal/trace/catalog's MigrateV1ToV2
// derives and WriteMigration commits. Like model.go's seven configuration
// projections, this is a fresh, purpose-built Go type projection rather
// than a direct reflection of catalog.Map: this leaf package gains no new
// internal dependency, and the optional/pending remote-linkage shape
// (CONTEXT D-11: pending/null linkage is valid incomplete linkage, never
// local failure) is expressed through explicit "nullable" jsonschema tags
// on the three fields that carry a remote identity only once synchronized.
//
// The full durable local ID grammar (project:/milestone:/phase:/req:/
// plan:/task:) is owned exclusively by internal/trace/catalog/id.go
// (D-05: refer, never repeat); this projection's id patterns intentionally
// stay a looser "kind:value" shape rather than duplicating that grammar's
// six-way alternation as a second, drift-prone authority.
package contracts

import "github.com/invopop/jsonschema"

// LinearMapSchema projects the schema-2 .planning/linear-map.json document
// (CONTEXT D-08): the durable project/milestone identity seed, the
// complete dynamically discovered local catalog, and one credential-free
// remote mapping per non-project entity.
type LinearMapSchema struct {
	Schema          int                      `json:"schema" jsonschema:"required,enum=2,description=Supported linear map schema version; only schema 2 (the migrated complete catalog map) is projected."`
	Repository      LinearMapRepositoryBlock `json:"repository" jsonschema:"required,description=The durable repository root project identity seed."`
	ActiveMilestone LinearMapMilestoneBlock  `json:"active_milestone" jsonschema:"required,description=The durable active milestone identity seed."`
	Entities        []LinearMapEntitySchema  `json:"entities" jsonschema:"required,description=The complete dynamically discovered local catalog in deterministic build order."`
	RemoteMappings  []LinearMapRemoteMapping `json:"remote_mappings" jsonschema:"required,description=One remote mapping per non-project entity; pending/null linkage is valid incomplete linkage and never a local failure (CONTEXT D-11)."`
}

// LinearMapRepositoryBlock is the durable repository root project identity
// seed (CONTEXT D-14: renaming Name never changes ProjectID's identity).
type LinearMapRepositoryBlock struct {
	ProjectID string `json:"project_id" jsonschema:"required,pattern=^project:[a-z0-9]+(-[a-z0-9]+)*$,description=Durable local project id; never changes on rename (D-14)."`
	Name      string `json:"name" jsonschema:"required,minLength=1,description=Display-only project name."`
}

// LinearMapMilestoneBlock is the durable active milestone identity seed
// (CONTEXT D-14: renaming Name never changes MilestoneID's identity).
type LinearMapMilestoneBlock struct {
	MilestoneID string `json:"milestone_id" jsonschema:"required,pattern=^milestone:v[0-9]+$,description=Durable local milestone id; never changes on rename (D-14)."`
	Name        string `json:"name" jsonschema:"required,minLength=1,description=Display-only milestone name."`
}

// LinearMapEntitySchema is one durable local catalog entity summary,
// mirroring internal/trace/catalog.EntitySummary's JSON shape exactly.
type LinearMapEntitySchema struct {
	LocalID       string `json:"local_id" jsonschema:"required,pattern=^[a-z]+:[A-Za-z0-9._-]+$,description=Durable local id; identity never changes on rename (D-14)."`
	Kind          string `json:"kind" jsonschema:"required,enum=project,enum=milestone,enum=phase,enum=req,enum=plan,enum=task,description=Catalog entity kind."`
	ParentLocalID string `json:"parent_local_id" jsonschema:"required,pattern=^([a-z]+:[A-Za-z0-9._-]+)?$,description=Parent durable local id; empty only for the project root."`
	Display       string `json:"display" jsonschema:"required,description=Display-only text; renaming never changes identity (D-14)."`
	Source        string `json:"source" jsonschema:"required,pattern=^[.]planning(/[A-Za-z0-9._-]+)+$,description=Repository-relative planning artifact source; contained inside .planning/ (D-11: repository text is the authority)."`
}

// LinearMapRemoteMapping is one optional, credential-free link from a
// durable local id to a Linear remote object (CONTEXT D-11/D-14). Its
// nullable identity fields carry null while the mapping is pending; no
// value here is ever invented offline.
type LinearMapRemoteMapping struct {
	RepoID     string  `json:"repo_id" jsonschema:"required,pattern=^[a-z]+:[A-Za-z0-9._-]+$,description=The mapped durable local id."`
	LinearType string  `json:"linear_type" jsonschema:"required,enum=project,enum=project_milestone,enum=issue,description=The Linear remote object type this local id maps to."`
	Status     string  `json:"status" jsonschema:"required,minLength=1,description=Mapping status; for example pending before any remote synchronization has run."`
	LinearUUID *string `json:"linear_uuid" jsonschema:"required,nullable,description=Immutable Linear GraphQL UUID once synchronized; null while pending. Never invented offline."`
	Identifier *string `json:"identifier" jsonschema:"required,nullable,description=Human-facing Linear identifier (for example GOLC-123) once synchronized; null while pending."`
	URL        *string `json:"url" jsonschema:"required,nullable,description=Linear web URL once synchronized; null while pending."`
}

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "linear-map",
	OutputPath: "schemas/linear-map.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&LinearMapSchema{}) },
})
