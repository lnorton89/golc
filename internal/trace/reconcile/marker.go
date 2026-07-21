// marker.go implements the D-14 visible, parser-stable identity footer
// (RESEARCH.md Pattern 6): every managed Linear description carries an
// exact "GOLC local ID: <id>" / "GOLC mapping schema: <n>" footer so
// objects stay identifiable across renames. Titles are never identity;
// only this footer, or an already-linked immutable Linear UUID, may
// establish or confirm which local entity a remote object represents.
package reconcile

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/lnorton89/golc/internal/trace/catalog"
)

// MarkerSchema is the fixed mapping schema version stamped into every
// rendered footer, matching the schema-2 .planning/linear-map.json shape
// this package plans against.
const MarkerSchema = 2

// markerPattern matches one exact GOLC identity footer anywhere inside a
// larger description body. It is deliberately strict: a bare "---" divider
// alone, or lines in the wrong order, do not match, so accidental
// look-alike text can never be mistaken for a real footer.
var markerPattern = regexp.MustCompile(`(?s)---\r?\nGOLC local ID: (\S+)\r?\nGOLC mapping schema: (\d+)\r?\n?`)

// RenderMarker renders the exact visible identity footer for localID
// (RESEARCH.md Pattern 6). It fails if localID does not match the durable
// local-ID grammar, since a footer must never encode anything but a real
// identity.
func RenderMarker(localID string) (string, error) {
	if _, err := catalog.ParseID(localID); err != nil {
		return "", fmt.Errorf("GOLC_RECONCILE_MARKER_RENDER: %v", err)
	}
	return fmt.Sprintf("---\nGOLC local ID: %s\nGOLC mapping schema: %d\n", localID, MarkerSchema), nil
}

// ParseMarker scans description for exactly one GOLC identity footer and
// returns its parsed content. found is false when no footer is present
// (for example, a remote object nobody has managed yet). More than one
// footer, a non-numeric schema, or a local ID that does not match the
// durable grammar are reported as errors rather than silently picking one
// candidate, so ambiguous or corrupted remote content is never adopted
// blindly (CONTEXT D-14; RESEARCH.md Pattern 6 ambiguity gate).
func ParseMarker(description string) (marker Marker, found bool, err error) {
	matches := markerPattern.FindAllStringSubmatch(description, -1)
	if len(matches) == 0 {
		return Marker{}, false, nil
	}
	if len(matches) > 1 {
		return Marker{}, false, fmt.Errorf("GOLC_RECONCILE_MARKER_AMBIGUOUS: description contains %d GOLC identity footers", len(matches))
	}
	match := matches[0]
	localID := match[1]
	if _, parseErr := catalog.ParseID(localID); parseErr != nil {
		return Marker{}, false, fmt.Errorf("GOLC_RECONCILE_MARKER_PARSE: %v", parseErr)
	}
	schema, convErr := strconv.Atoi(match[2])
	if convErr != nil {
		return Marker{}, false, fmt.Errorf("GOLC_RECONCILE_MARKER_PARSE: mapping schema %q is not numeric", match[2])
	}
	return Marker{LocalID: localID, Schema: schema}, true, nil
}

// ValidateMarkerIdentity checks that a discovered remote marker matches
// the expected local operation identity: the exact mapping schema, the
// exact durable local ID, the entity kind decoded from that ID's grammar,
// and — for plan and task IDs, which structurally encode their parent's
// phase/plan numbers — that the marker's implied parent matches the
// operation's recorded parent local ID. Requirement, phase, and milestone
// IDs carry no parent structure in their own grammar, so no parent check
// applies to those kinds; their parent comes only from the catalog build,
// never from title or restructurable display text (CONTEXT D-14).
func ValidateMarkerIdentity(marker Marker, op Operation) error {
	if marker.Schema != MarkerSchema {
		return fmt.Errorf("GOLC_RECONCILE_MARKER_SCHEMA: marker schema %d does not match expected %d", marker.Schema, MarkerSchema)
	}
	if marker.LocalID != op.LocalID {
		return fmt.Errorf("GOLC_RECONCILE_MARKER_IDENTITY: marker local ID %q does not match operation local ID %q", marker.LocalID, op.LocalID)
	}
	parsed, err := catalog.ParseID(marker.LocalID)
	if err != nil {
		return fmt.Errorf("GOLC_RECONCILE_MARKER_IDENTITY: %v", err)
	}
	if string(parsed.Kind) != op.Kind {
		return fmt.Errorf("GOLC_RECONCILE_MARKER_KIND: marker local ID %q decodes to kind %q but operation declares kind %q", marker.LocalID, parsed.Kind, op.Kind)
	}
	switch parsed.Kind {
	case catalog.KindTask:
		expectedParent := "plan:" + parsed.Phase + "-" + parsed.Plan
		if op.ParentLocalID != expectedParent {
			return fmt.Errorf("GOLC_RECONCILE_MARKER_PARENT: task %q must have parent %q, operation declares %q", marker.LocalID, expectedParent, op.ParentLocalID)
		}
	case catalog.KindPlan:
		expectedParent := "phase:" + parsed.Phase
		if op.ParentLocalID != expectedParent {
			return fmt.Errorf("GOLC_RECONCILE_MARKER_PARENT: plan %q must have parent %q, operation declares %q", marker.LocalID, expectedParent, op.ParentLocalID)
		}
	}
	return nil
}
