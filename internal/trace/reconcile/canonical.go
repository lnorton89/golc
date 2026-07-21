// canonical.go implements the D-17 hash-bound canonical preview:
// SHA-256 digests over sorted, canonically encoded inputs, a fixed
// dependency/hierarchy operation order, D-13 three-way conflict
// detection, and the byte-stable plan_id = sha256(canonical_body) binding
// (RESEARCH.md Pattern 5). Identical intent, mapping, and remote-scope
// inputs always produce byte-identical plan bytes and IDs; no timestamp,
// random value, or credential is ever part of a hashed body.
package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
)

// SchemaVersion is the fixed schema version stamped into every canonical
// plan produced by this package.
const SchemaVersion = 1

// DigestBytes returns the lowercase hex SHA-256 digest of payload.
func DigestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// DigestValue canonically encodes v (sorted map keys, LF-terminated,
// idempotent per internal/strictjson) and returns its SHA-256 digest.
// Equal values always produce an equal digest regardless of construction
// order.
func DigestValue(v any) (string, error) {
	encoded, err := strictjson.CanonicalEncode(v)
	if err != nil {
		return "", fmt.Errorf("GOLC_RECONCILE_DIGEST: %v", err)
	}
	return DigestBytes(encoded), nil
}

// sortedIntents returns a copy of intents sorted by local ID so digesting
// and planning are independent of caller-supplied order.
func sortedIntents(intents []Intent) []Intent {
	sorted := append([]Intent(nil), intents...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].LocalID < sorted[j].LocalID })
	return sorted
}

// DigestIntent computes the intent_digest over the sorted repository
// intent set: the exact desired owned fields for every managed local
// entity (CONTEXT D-11).
func DigestIntent(intents []Intent) (string, error) {
	return DigestValue(sortedIntents(intents))
}

// sortedMappings returns a copy of mappings sorted by repo ID.
func sortedMappings(mappings []catalog.RemoteMapping) []catalog.RemoteMapping {
	sorted := append([]catalog.RemoteMapping(nil), mappings...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].RepoID < sorted[j].RepoID })
	return sorted
}

// DigestMapping computes the mapping_digest over the sorted credential-free
// remote mapping set from .planning/linear-map.json.
func DigestMapping(mappings []catalog.RemoteMapping) (string, error) {
	return DigestValue(sortedMappings(mappings))
}

// sortedObservations returns a copy of observations sorted by local ID.
func sortedObservations(observations []RemoteObservation) []RemoteObservation {
	sorted := append([]RemoteObservation(nil), observations...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].LocalID < sorted[j].LocalID })
	return sorted
}

// DigestRemoteScope computes the remote_scope_digest over the sorted,
// exhaustively captured current remote observation set (RESEARCH.md
// Pattern 5/7): the exact evidence a preview was computed against.
func DigestRemoteScope(scope RemoteScope) (string, error) {
	return DigestValue(RemoteScope{Observations: sortedObservations(scope.Observations)})
}

// hierarchyRank returns the fixed D-17 operation ordering rank for a
// catalog kind: Project Milestone maps to a Linear Project (rank 0), Phase
// maps to a Linear Project Milestone (rank 1), Requirement and Plan map to
// a parent Issue (rank 2), and Task maps to a task sub-issue (rank 3). The
// repository-root Project kind is never remote-mapped and never appears
// here.
func hierarchyRank(kind string) (int, error) {
	switch catalog.Kind(kind) {
	case catalog.KindMilestone:
		return 0, nil
	case catalog.KindPhase:
		return 1, nil
	case catalog.KindRequirement, catalog.KindPlan:
		return 2, nil
	case catalog.KindTask:
		return 3, nil
	default:
		return 0, fmt.Errorf("GOLC_RECONCILE_KIND_UNMANAGED: kind %q has no reconciliation ordering rank", kind)
	}
}

// SortOperations orders operations in place per the fixed D-17 hierarchy —
// Project Milestone, then Project Milestone(phase), then parent/requirement
// Issue, then task sub-issue — tie-broken by local ID so ordering never
// depends on caller-supplied or map-derived traversal order (RESEARCH.md
// Pattern 5).
func SortOperations(operations []Operation) error {
	ranks := make(map[string]int, len(operations))
	for _, op := range operations {
		rank, err := hierarchyRank(op.Kind)
		if err != nil {
			return err
		}
		ranks[op.LocalID] = rank
	}
	sort.SliceStable(operations, func(i, j int) bool {
		ri, rj := ranks[operations[i].LocalID], ranks[operations[j].LocalID]
		if ri != rj {
			return ri < rj
		}
		return operations[i].LocalID < operations[j].LocalID
	})
	return nil
}

// sortedConflicts returns a copy of conflicts sorted by local ID, then
// field, for deterministic output.
func sortedConflicts(conflicts []Conflict) []Conflict {
	sorted := append([]Conflict(nil), conflicts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].LocalID != sorted[j].LocalID {
			return sorted[i].LocalID < sorted[j].LocalID
		}
		return sorted[i].Field < sorted[j].Field
	})
	return sorted
}

// sortedFieldNames returns the sorted keys of a field map for deterministic
// owned-field ordering, independent of Go's randomized map iteration.
func sortedFieldNames(fields map[string]string) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// canonicalFieldsJSON renders a field map as raw JSON with a fixed,
// sorted-key shape so Before/After snapshots are byte-stable.
func canonicalFieldsJSON(fields map[string]string) (json.RawMessage, error) {
	if fields == nil {
		fields = map[string]string{}
	}
	names := sortedFieldNames(fields)
	ordered := make(map[string]string, len(fields))
	for _, name := range names {
		ordered[name] = fields[name]
	}
	encoded, err := json.Marshal(ordered)
	if err != nil {
		return nil, fmt.Errorf("GOLC_RECONCILE_DIGEST: %v", err)
	}
	return json.RawMessage(encoded), nil
}

// BuildPlan derives the complete deterministic D-17 preview from
// repository intent, the credential-free remote mapping set, the
// exhausted current remote observation scope, and the last-synchronized
// three-way baseline. Every intent must already carry a remote mapping
// (the caller is responsible for keeping .planning/linear-map.json
// complete via catalog.MigrateV1ToV2 before planning). Any owned field
// that changed on both sides since its recorded baseline, and now
// disagrees between the repository and Linear, is blocked as a D-13
// conflict instead of becoming an operation. Identical inputs always
// produce a byte-identical Plan and PlanID (CONTEXT D-17).
func BuildPlan(intents []Intent, mappings []catalog.RemoteMapping, scope RemoteScope, baselines []SyncBaseline) (Plan, error) {
	intentDigest, err := DigestIntent(intents)
	if err != nil {
		return Plan{}, err
	}
	mappingDigest, err := DigestMapping(mappings)
	if err != nil {
		return Plan{}, err
	}
	remoteScopeDigest, err := DigestRemoteScope(scope)
	if err != nil {
		return Plan{}, err
	}

	mappingByID := make(map[string]catalog.RemoteMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByID[mapping.RepoID] = mapping
	}
	observationByID := make(map[string]RemoteObservation, len(scope.Observations))
	for _, observation := range scope.Observations {
		observationByID[observation.LocalID] = observation
	}
	baselineByID := make(map[string]SyncBaseline, len(baselines))
	for _, baseline := range baselines {
		baselineByID[baseline.LocalID] = baseline
	}

	operations := make([]Operation, 0, len(intents))
	conflicts := []Conflict{}

	for _, intent := range sortedIntents(intents) {
		if catalog.Kind(intent.Kind) == catalog.KindProject {
			continue // the repository root is never remote-mapped
		}
		mapping, mapped := mappingByID[intent.LocalID]
		if !mapped {
			return Plan{}, fmt.Errorf("GOLC_RECONCILE_MAPPING_MISSING: %s has no remote mapping", intent.LocalID)
		}

		observation, observed := observationByID[intent.LocalID]
		baseline := baselineByID[intent.LocalID]

		blocked := false
		for _, field := range sortedFieldNames(intent.Fields) {
			repoValue := intent.Fields[field]
			linearValue, hasLinear := observation.Fields[field]
			baseValue, hasBase := baseline.Fields[field]
			if !observed || !hasLinear || !hasBase {
				continue
			}
			if baseValue == repoValue || baseValue == linearValue || repoValue == linearValue {
				continue
			}
			blocked = true
			base, repo, linear := baseValue, repoValue, linearValue
			conflicts = append(conflicts, Conflict{
				LocalID:           intent.LocalID,
				Field:             field,
				BaseValue:         &base,
				RepositoryValue:   &repo,
				LinearValue:       &linear,
				ResolutionCommand: fmt.Sprintf("golc linear resolve --local-id %s --field %s", intent.LocalID, field),
			})
		}
		if blocked {
			continue
		}

		before, err := canonicalFieldsJSON(observation.Fields)
		if err != nil {
			return Plan{}, err
		}
		after, err := canonicalFieldsJSON(intent.Fields)
		if err != nil {
			return Plan{}, err
		}
		marker, err := RenderMarker(intent.LocalID)
		if err != nil {
			return Plan{}, err
		}

		var linearUUID *string
		if mapping.LinearUUID != nil {
			uuid := *mapping.LinearUUID
			linearUUID = &uuid
		}
		var expectedUpdatedAt *string
		if observed && observation.UpdatedAt != "" {
			updatedAt := observation.UpdatedAt
			expectedUpdatedAt = &updatedAt
		}

		dependsOn := []string{}
		if intent.ParentLocalID != "" {
			if parsedParent, parseErr := catalog.ParseID(intent.ParentLocalID); parseErr == nil && parsedParent.Kind != catalog.KindProject {
				dependsOn = []string{intent.ParentLocalID}
			}
		}

		operations = append(operations, Operation{
			LocalID:           intent.LocalID,
			Kind:              intent.Kind,
			LinearType:        intent.LinearType,
			LinearUUID:        linearUUID,
			DiscoveryMarker:   marker,
			ParentLocalID:     intent.ParentLocalID,
			Before:            before,
			After:             after,
			OwnedFields:       sortedFieldNames(intent.Fields),
			ExpectedUpdatedAt: expectedUpdatedAt,
			DependsOn:         dependsOn,
		})
	}

	if err := SortOperations(operations); err != nil {
		return Plan{}, err
	}
	conflicts = sortedConflicts(conflicts)

	body := planBody{
		SchemaVersion:     SchemaVersion,
		IntentDigest:      intentDigest,
		MappingDigest:     mappingDigest,
		RemoteScopeDigest: remoteScopeDigest,
		Operations:        operations,
		Conflicts:         conflicts,
	}
	encodedBody, err := strictjson.CanonicalEncode(body)
	if err != nil {
		return Plan{}, fmt.Errorf("GOLC_RECONCILE_DIGEST: %v", err)
	}
	planID := PlanID(encodedBody)

	return Plan{
		SchemaVersion:     body.SchemaVersion,
		IntentDigest:      body.IntentDigest,
		MappingDigest:     body.MappingDigest,
		RemoteScopeDigest: body.RemoteScopeDigest,
		Operations:        body.Operations,
		Conflicts:         body.Conflicts,
		PlanID:            planID,
	}, nil
}

// PlanID returns the SHA-256 hex digest of an exact canonical plan body
// (RESEARCH.md Pattern 5): plan_id = sha256(canonical_body). canonicalBody
// must never include plan_id itself, a timestamp, or a random value, so
// identical repository/mapping/remote-scope inputs always produce an
// identical PlanID.
func PlanID(canonicalBody []byte) string {
	return DigestBytes(canonicalBody)
}
