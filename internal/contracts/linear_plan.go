// linear_plan.go registers the strict Draft 2020-12 "linear-plan" and
// "linear-report" contracts (CONTEXT D-08, D-13, D-14, D-17, D-18, D-21):
// the exact canonical shape of a reconcile.Plan a contributor reviews
// before "linear apply" mutates anything, and the exact shape of the
// apply.Report that mutation produces. Like linear.go's linear-map
// projection, both are fresh, purpose-built Go type projections rather
// than a direct reflection of reconcile.Plan/apply.Report: this leaf
// package gains no new internal dependency on internal/trace/reconcile or
// internal/trace/apply, and every enum/pattern/nullable constraint those
// two packages enforce only in Go is additionally expressed here as a
// strict, generated, reviewable JSON Schema contract.
//
// The actual strict decode-before-typed-use gate the self-registered
// "linear apply" route runs (duplicate/unknown JSON, a tampered plan_id,
// an out-of-canonical-order operation list, a structurally malformed D-13
// conflict, and a local id that is both planned and unresolved-conflicted)
// lives in internal/command/linear.go, composing internal/strictjson and
// the already-proven internal/trace/apply and internal/trace/reconcile
// entrypoints those packages export -- it is not reimplemented here, so
// this package's only responsibility stays schema generation (D-05: refer,
// never repeat).
package contracts

import "github.com/invopop/jsonschema"

// LinearPlanSchema projects the D-17 canonical exact-plan apply input: a
// deterministic, byte-stable, hash-bound reconciliation plan.
type LinearPlanSchema struct {
	SchemaVersion     int                         `json:"schema_version" jsonschema:"required,enum=1,description=Fixed reconcile.SchemaVersion this plan was built under."`
	IntentDigest      string                      `json:"intent_digest" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=SHA-256 digest of the sorted repository intent set this plan was computed from."`
	MappingDigest     string                      `json:"mapping_digest" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=SHA-256 digest of the sorted credential-free remote mapping set this plan was computed from."`
	RemoteScopeDigest string                      `json:"remote_scope_digest" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=SHA-256 digest of the sorted exhaustively captured remote observation scope this plan was computed from."`
	Operations        []LinearPlanOperationSchema `json:"operations" jsonschema:"required,description=Planned mutations in the fixed D-17 hierarchy/local-id order; never reordered after generation."`
	Conflicts         []LinearPlanConflictSchema  `json:"conflicts" jsonschema:"required,description=Blocked D-13 field disagreements; a local id recorded here never also owns an operation."`
	PlanID            string                      `json:"plan_id" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=SHA-256 digest binding this plan's own canonical body; plan_id equals sha256(canonical_body)."`
}

// LinearPlanOperationSchema is one planned mutation against a single
// managed Linear object, keyed by durable local id (CONTEXT D-14/D-17).
type LinearPlanOperationSchema struct {
	LocalID           string            `json:"local_id" jsonschema:"required,pattern=^[a-z]+:[A-Za-z0-9._-]+$,description=Durable local id this operation mutates; identity never changes on rename (D-14)."`
	Kind              string            `json:"kind" jsonschema:"required,enum=milestone,enum=phase,enum=req,enum=plan,enum=task,description=Catalog kind; the repository-root project kind is never remote-managed and never appears here."`
	LinearType        string            `json:"linear_type" jsonschema:"required,enum=project,enum=project_milestone,enum=issue,description=Target Linear object type this local id maps to."`
	LinearUUID        *string           `json:"linear_uuid" jsonschema:"required,nullable,description=Already-linked immutable Linear UUID; null for a not-yet-linked operation discovered by marker instead (D-14)."`
	DiscoveryMarker   string            `json:"discovery_marker" jsonschema:"required,minLength=1,description=Exact D-14 identity footer this operation's remote object must carry."`
	ParentLocalID     string            `json:"parent_local_id" jsonschema:"required,pattern=^([a-z]+:[A-Za-z0-9._-]+)?$,description=Structural parent local id; empty only for a top-level milestone operation."`
	Before            map[string]string `json:"before" jsonschema:"required,description=Canonically encoded observed remote owned-field values before this operation; empty when unobserved."`
	After             map[string]string `json:"after" jsonschema:"required,description=Canonically encoded desired repository-owned field values after this operation."`
	OwnedFields       []string          `json:"owned_fields" jsonschema:"required,uniqueItems=true,description=Sorted repository-owned field names this operation may write; never a Linear-owned operational field (D-11)."`
	ExpectedUpdatedAt *string           `json:"expected_updated_at" jsonschema:"required,nullable,description=Captured remote updated_at precondition; null when no prior observation exists."`
	DependsOn         []string          `json:"depends_on" jsonschema:"required,uniqueItems=true,description=Parent local id(s) that must already be complete or linked before this operation may run."`
}

// LinearPlanConflictSchema is one blocked D-13 field-level disagreement: a
// conflicted local id is intentionally excluded from Operations above.
type LinearPlanConflictSchema struct {
	LocalID           string  `json:"local_id" jsonschema:"required,pattern=^[a-z]+:[A-Za-z0-9._-]+$,description=Durable local id with a blocked D-13 field disagreement."`
	Field             string  `json:"field" jsonschema:"required,minLength=1,description=Owned field name both sides changed away from the last synchronized baseline."`
	BaseValue         *string `json:"base_value" jsonschema:"required,nullable,description=Last-synchronized baseline value this field's D-13 three-way comparison is anchored to."`
	RepositoryValue   *string `json:"repository_value" jsonschema:"required,nullable,description=Current repository-owned value."`
	LinearValue       *string `json:"linear_value" jsonschema:"required,nullable,description=Current observed Linear value."`
	ResolutionCommand string  `json:"resolution_command" jsonschema:"required,minLength=1,description=Exact command a contributor runs to explicitly resolve this conflict."`
}

// LinearReportSchema projects the complete, human-reviewable apply.Report
// outcome for one applied plan: every operation's exact recorded status,
// in plan order (CONTEXT D-21).
type LinearReportSchema struct {
	PlanID  string                     `json:"plan_id" jsonschema:"required,pattern=^[0-9a-f]{64}$,description=The applied plan's own plan_id."`
	Results []LinearReportResultSchema `json:"results" jsonschema:"required,description=Every operation's exact recorded outcome in the plan's own operation order."`
}

// LinearReportResultSchema is the exact recorded apply outcome for one
// plan operation (CONTEXT D-21): as soon as any result is not
// completed/noop, every remaining result is pending or blocked -- a
// completed or noop result never legally follows one that is not.
type LinearReportResultSchema struct {
	LocalID    string  `json:"local_id" jsonschema:"required,pattern=^[a-z]+:[A-Za-z0-9._-]+$,description=Durable local id this result reports."`
	Status     string  `json:"status" jsonschema:"required,enum=completed,enum=noop,enum=pending,enum=blocked,description=Exact apply outcome for this operation."`
	LinearUUID *string `json:"linear_uuid" jsonschema:"required,nullable,description=Confirmed Linear UUID once completed or noop; null otherwise."`
	UpdatedAt  *string `json:"updated_at" jsonschema:"required,nullable,description=Confirmed remote updated_at once completed or noop; null otherwise."`
	Reason     string  `json:"reason" jsonschema:"required,description=Stable diagnostic explaining a pending or blocked outcome; empty for completed/noop."`
	RetryAfter *string `json:"retry_after" jsonschema:"required,nullable,description=Safe-retry hint from a RetryableError; null otherwise."`
}

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "linear-plan",
	OutputPath: "schemas/linear-plan.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&LinearPlanSchema{}) },
})

var _ = MustRegisterSchema(SchemaDescriptor{
	Name:       "linear-report",
	OutputPath: "schemas/linear-report.schema.json",
	Schema:     func() *jsonschema.Schema { return newReflector().Reflect(&LinearReportSchema{}) },
})
