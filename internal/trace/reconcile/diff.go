// diff.go implements the D-17 complete-snapshot preview path over a
// transport-neutral Transport capture (CONTEXT D-13/D-14/D-15/D-21):
// ValidateCompleteSnapshot blocks any non-exhaustive or ambiguous
// capture before it can reach planning, discoverObservations resolves
// each intent's current remote identity through an already-linked UUID
// or the exact D-14 marker footer (zero matches creates, exactly one
// matching marker adopts, more than one blocks), and BuildCompletePreview
// feeds the discovered observations into the already-tested BuildPlan
// three-way conflict/ordering/hashing contract rather than reimplementing
// it. ArchivePreview/UnlinkPreview cover D-15: removal is only ever
// produced by an explicit, already-reviewed request against an
// already-linked entity, never inferred from local absence.
package reconcile

import (
	"fmt"

	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/transport"
)

// snapshotReason returns the diagnostic text for a blocked snapshot,
// falling back to a stable description of the status itself when the
// transport did not supply one.
func snapshotReason(snapshot transport.Snapshot) string {
	if snapshot.Reason != "" {
		return snapshot.Reason
	}
	return fmt.Sprintf("snapshot status %q blocks any preview", snapshot.Status)
}

// ValidateCompleteSnapshot blocks any snapshot that is not exhaustively
// complete or that could not resolve a single unambiguous remote state
// (CONTEXT D-21): incomplete, partial, cursor-anomalous, rate-limited,
// and transport-reported-ambiguous statuses all fail closed, and — even
// for a status-complete snapshot — two distinct remote records whose D-14
// identity footers both name the same local ID are rejected as ambiguous
// rather than letting one silently win (T-01-28 spoofing/duplication).
func ValidateCompleteSnapshot(snapshot transport.Snapshot) error {
	switch snapshot.Status {
	case transport.SnapshotComplete:
		// fall through to per-record identity-ambiguity checks below
	case transport.SnapshotIncomplete:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_INCOMPLETE: %s", snapshotReason(snapshot))
	case transport.SnapshotPartial:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_PARTIAL: %s", snapshotReason(snapshot))
	case transport.SnapshotCursorAnomaly:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_CURSOR_ANOMALY: %s", snapshotReason(snapshot))
	case transport.SnapshotAmbiguous:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS: %s", snapshotReason(snapshot))
	case transport.SnapshotRateLimited:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_RATE_LIMITED: %s", snapshotReason(snapshot))
	default:
		return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_STATUS_UNKNOWN: %q is not a recognized snapshot status", snapshot.Status)
	}

	seenByLocalID := make(map[string]string, len(snapshot.Records))
	for _, record := range snapshot.Records {
		marker, found, err := ParseMarker(record.Description)
		if err != nil {
			return fmt.Errorf("GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS: %v", err)
		}
		if !found {
			continue
		}
		if previous, duplicate := seenByLocalID[marker.LocalID]; duplicate {
			return fmt.Errorf(
				"GOLC_RECONCILE_SNAPSHOT_AMBIGUOUS: local ID %q matches more than one remote record (%s and %s); titles are never used to disambiguate",
				marker.LocalID, previous, record.LinearUUID)
		}
		seenByLocalID[marker.LocalID] = record.LinearUUID
	}
	return nil
}

// ThreeWayField applies the exact D-13 three-way conflict rule to one
// mapped field: a field blocks only when base, repository, and Linear
// values are pairwise distinct (all three legs disagree). It returns nil
// when the field may safely proceed as a plain create/update.
func ThreeWayField(localID, field, baseValue, repoValue, linearValue string) *Conflict {
	if baseValue == repoValue || baseValue == linearValue || repoValue == linearValue {
		return nil
	}
	base, repo, linear := baseValue, repoValue, linearValue
	return &Conflict{
		LocalID:           localID,
		Field:             field,
		BaseValue:         &base,
		RepositoryValue:   &repo,
		LinearValue:       &linear,
		ResolutionCommand: fmt.Sprintf("golc linear resolve --local-id %s --field %s", localID, field),
	}
}

// discoverObservations resolves each intent's current remote observation
// from an already-validated complete snapshot. An intent whose remote
// mapping already carries a linked Linear UUID is matched by that UUID
// directly — the immutable UUID alone establishes identity (CONTEXT
// D-14). An unlinked intent is matched by scanning every record's D-14
// identity footer for its exact local ID: zero matches means no current
// observation exists yet (the entity will be planned as a create), one
// matching marker whose decoded kind/parent also validates adopts that
// record's fields, and more than one match blocks — titles are never
// consulted to break the tie (CONTEXT D-14).
func discoverObservations(intents []Intent, mappings []catalog.RemoteMapping, snapshot transport.Snapshot) ([]RemoteObservation, error) {
	mappingByID := make(map[string]catalog.RemoteMapping, len(mappings))
	for _, mapping := range mappings {
		mappingByID[mapping.RepoID] = mapping
	}

	recordsByUUID := make(map[string]transport.RemoteRecord, len(snapshot.Records))
	for _, record := range snapshot.Records {
		if record.LinearUUID != "" {
			recordsByUUID[record.LinearUUID] = record
		}
	}

	observations := make([]RemoteObservation, 0, len(intents))
	for _, intent := range intents {
		mapping, mapped := mappingByID[intent.LocalID]
		if !mapped {
			continue // BuildPlan itself rejects an intent with no remote mapping
		}

		if mapping.LinearUUID != nil {
			record, found := recordsByUUID[*mapping.LinearUUID]
			if !found {
				continue // no current observation captured; treated as unobserved by BuildPlan
			}
			observations = append(observations, RemoteObservation{
				LocalID:   intent.LocalID,
				Fields:    record.Fields,
				UpdatedAt: record.UpdatedAt,
			})
			continue
		}

		matches := make([]transport.RemoteRecord, 0, 1)
		for _, record := range snapshot.Records {
			marker, found, err := ParseMarker(record.Description)
			if err != nil {
				return nil, fmt.Errorf("GOLC_RECONCILE_DISCOVERY_FAILED: %s: %v", intent.LocalID, err)
			}
			if !found || marker.LocalID != intent.LocalID {
				continue
			}
			candidateOp := Operation{LocalID: intent.LocalID, Kind: intent.Kind, ParentLocalID: intent.ParentLocalID}
			if err := ValidateMarkerIdentity(marker, candidateOp); err != nil {
				return nil, fmt.Errorf("GOLC_RECONCILE_DISCOVERY_INVALID: %s: %v", intent.LocalID, err)
			}
			matches = append(matches, record)
		}

		switch len(matches) {
		case 0:
			// No discovered remote identity: plan this intent as a create.
		case 1:
			observations = append(observations, RemoteObservation{
				LocalID:   intent.LocalID,
				Fields:    matches[0].Fields,
				UpdatedAt: matches[0].UpdatedAt,
			})
		default:
			return nil, fmt.Errorf(
				"GOLC_RECONCILE_DISCOVERY_AMBIGUOUS: %s matches %d remote records by identity footer; titles are never used to disambiguate",
				intent.LocalID, len(matches))
		}
	}
	return observations, nil
}

// BuildCompletePreview builds the exact D-17 preview from a transport
// snapshot: it blocks before any planning if the snapshot is not complete
// or is ambiguous (ValidateCompleteSnapshot), resolves every intent's
// current remote observation through UUID linkage or exact D-14 marker
// discovery (discoverObservations), and then delegates the already-tested
// D-13 three-way conflict, D-17 ordering, and hash-binding logic to
// BuildPlan. It never reimplements that contract — only feeds it a
// discovered RemoteScope.
func BuildCompletePreview(intents []Intent, mappings []catalog.RemoteMapping, snapshot transport.Snapshot, baselines []SyncBaseline) (Plan, error) {
	if err := ValidateCompleteSnapshot(snapshot); err != nil {
		return Plan{}, err
	}
	observations, err := discoverObservations(intents, mappings, snapshot)
	if err != nil {
		return Plan{}, err
	}
	return BuildPlan(intents, mappings, RemoteScope{Observations: observations}, baselines)
}

// ArchivePreview is one exact, explicitly requested D-15 removal preview.
// It is never produced by local absence — only an explicit "linear
// archive" or "linear unlink" invocation against an already-linked
// managed entity ever creates one, and it is always reviewable before any
// apply (CONTEXT D-15/D-17).
type ArchivePreview struct {
	LocalID    string  `json:"local_id"`
	Action     string  `json:"action"`
	LinearType string  `json:"linear_type"`
	LinearUUID *string `json:"linear_uuid"`
}

// buildArchivePreview is the shared archive/unlink builder: it fails
// closed for any mapping with no recorded Linear link, since there is
// nothing to archive or unlink.
func buildArchivePreview(action string, mapping catalog.RemoteMapping) (ArchivePreview, error) {
	if mapping.LinearUUID == nil {
		return ArchivePreview{}, fmt.Errorf("GOLC_RECONCILE_ARCHIVE_UNMAPPED: %s has no linked Linear object to %s", mapping.RepoID, action)
	}
	uuid := *mapping.LinearUUID
	return ArchivePreview{
		LocalID:    mapping.RepoID,
		Action:     action,
		LinearType: mapping.LinearType,
		LinearUUID: &uuid,
	}, nil
}

// BuildArchivePreview builds the explicit archive-review preview for an
// already-linked managed entity (CONTEXT D-15).
func BuildArchivePreview(mapping catalog.RemoteMapping) (ArchivePreview, error) {
	return buildArchivePreview("archive", mapping)
}

// BuildUnlinkPreview builds the explicit unlink-review preview for an
// already-linked managed entity (CONTEXT D-15): only the local-to-remote
// link is removed; the remote object itself is left untouched.
func BuildUnlinkPreview(mapping catalog.RemoteMapping) (ArchivePreview, error) {
	return buildArchivePreview("unlink", mapping)
}
