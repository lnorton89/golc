// Package reconcile defines the D-17 exact reconciliation preview
// contract: repository-derived intent, the credential-free remote mapping
// set from .planning/linear-map.json, the exhausted current remote
// observation scope, canonical hash-bound plans, dependency-ordered
// operations, D-13 field-level conflicts, and the visible, parser-stable
// D-14 local-ID identity footer. It never talks to Linear itself (that
// stays behind an isolated transport adapter per RESEARCH.md Pattern 7);
// this package only canonicalizes already-normalized inputs into an
// exactly reviewable mutation plan before any apply.
package reconcile

import (
	"encoding/json"

	"github.com/lnorton89/golc/internal/trace/catalog"
)

// Intent is the repository-derived desired remote state for one managed
// local entity: its durable local ID, catalog kind, target Linear object
// type, structural parent local ID, and the exact owned display/text
// fields (CONTEXT D-11). Only repository-owned data ever appears here;
// Linear operational fields (status, assignee, priority, estimate,
// completed_at) and comment/discussion fields never do (CONTEXT D-12).
type Intent struct {
	LocalID       string            `json:"local_id"`
	Kind          string            `json:"kind"`
	LinearType    string            `json:"linear_type"`
	ParentLocalID string            `json:"parent_local_id"`
	Fields        map[string]string `json:"fields"`
}

// IntentFromEntity derives the default repository intent for a catalog
// entity: its display text becomes the sole owned "title" field, matching
// the repository-owned field set from CONTEXT D-11. Callers may extend the
// returned Intent's Fields with additional owned text (e.g. requirement
// body) before calling BuildPlan.
func IntentFromEntity(entity catalog.Entity, linearType string) Intent {
	return Intent{
		LocalID:       entity.ID,
		Kind:          string(entity.Kind),
		LinearType:    linearType,
		ParentLocalID: entity.Parent,
		Fields:        map[string]string{"title": entity.Display},
	}
}

// RemoteObservation is one exact current-state observation for a managed
// Linear object, already resolved to its durable local ID by the caller
// (via its mapped UUID or a discovered marker footer). reconcile never
// performs discovery or transport itself (RESEARCH.md Pattern 7); it only
// canonicalizes already-normalized observations.
type RemoteObservation struct {
	LocalID   string            `json:"local_id"`
	Fields    map[string]string `json:"fields"`
	UpdatedAt string            `json:"updated_at"`
}

// RemoteScope is the complete, exhaustively paginated set of current
// remote observations captured immediately before planning (RESEARCH.md
// Pattern 5/7): the exact evidence a preview was computed against.
type RemoteScope struct {
	Observations []RemoteObservation `json:"observations"`
}

// SyncBaseline holds the last-successfully-synchronized field values for
// one managed local entity, keyed by field name. It is the fixed
// three-way-merge reference point (RESEARCH.md Pattern 4/5): a field is
// blocked as a conflict only when both the repository and Linear moved
// away from this recorded baseline since the last synchronization, and
// they now disagree with each other (CONTEXT D-13).
type SyncBaseline struct {
	LocalID string            `json:"local_id"`
	Fields  map[string]string `json:"fields"`
}

// Operation is one exact planned mutation against a single managed Linear
// object, keyed by durable local ID. Before/After carry only canonically
// encoded owned-field snapshots; ExpectedUpdatedAt is the captured
// remote-observation precondition and DependsOn names the parent
// operation(s) that must complete first. No timestamp is generated at
// build time and no random or credential value is ever included, so
// identical inputs always produce byte-identical operations (CONTEXT
// D-17/D-18).
type Operation struct {
	LocalID           string          `json:"local_id"`
	Kind              string          `json:"kind"`
	LinearType        string          `json:"linear_type"`
	LinearUUID        *string         `json:"linear_uuid"`
	DiscoveryMarker   string          `json:"discovery_marker"`
	ParentLocalID     string          `json:"parent_local_id"`
	Before            json.RawMessage `json:"before"`
	After             json.RawMessage `json:"after"`
	OwnedFields       []string        `json:"owned_fields"`
	ExpectedUpdatedAt *string         `json:"expected_updated_at"`
	DependsOn         []string        `json:"depends_on"`
}

// Conflict is a blocked field-by-field D-13 disagreement: both the
// repository and Linear changed the same mapped field away from the last
// synchronized baseline, and they now disagree with each other. A
// conflicted entity never receives an Operation; it requires an explicit
// resolution command instead (CONTEXT D-13).
type Conflict struct {
	LocalID           string  `json:"local_id"`
	Field             string  `json:"field"`
	BaseValue         *string `json:"base_value"`
	RepositoryValue   *string `json:"repository_value"`
	LinearValue       *string `json:"linear_value"`
	ResolutionCommand string  `json:"resolution_command"`
}

// Plan is the complete canonical D-17 preview: a deterministic,
// byte-stable reconciliation of repository intent against a captured
// remote scope, ready for review before any apply. PlanID binds the
// canonical body (every field below except PlanID itself) via SHA-256, so
// identical intent/mapping/remote-scope inputs always produce
// byte-identical plan bytes and IDs.
type Plan struct {
	SchemaVersion     int         `json:"schema_version"`
	IntentDigest      string      `json:"intent_digest"`
	MappingDigest     string      `json:"mapping_digest"`
	RemoteScopeDigest string      `json:"remote_scope_digest"`
	Operations        []Operation `json:"operations"`
	Conflicts         []Conflict  `json:"conflicts"`
	PlanID            string      `json:"plan_id"`
}

// planBody is the exact byte-hashed subset of Plan (RESEARCH.md Pattern
// 5): it excludes PlanID itself, since plan_id = sha256(canonical_body),
// and it carries no timestamp or random value, so identical inputs always
// hash identically.
type planBody struct {
	SchemaVersion     int         `json:"schema_version"`
	IntentDigest      string      `json:"intent_digest"`
	MappingDigest     string      `json:"mapping_digest"`
	RemoteScopeDigest string      `json:"remote_scope_digest"`
	Operations        []Operation `json:"operations"`
	Conflicts         []Conflict  `json:"conflicts"`
}

// Marker is the parsed exact identity footer content (RESEARCH.md Pattern
// 6): only the durable local ID and mapping schema are ever encoded —
// never a title, kind, or parent — because titles are display-only and
// can be renamed without changing remote identity (CONTEXT D-14).
type Marker struct {
	LocalID string
	Schema  int
}
