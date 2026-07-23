// diagnose.go implements Diagnose (CONTEXT D-11): an on-demand, read-only
// health report combining SQLite-level file/page corruption detection
// (PRAGMA integrity_check) with the existing structural show.validate()
// (reused via LoadForRead, not reimplemented -- 05-RESEARCH.md's Diagnose
// Code Example). Diagnose is never called from Load/Save/openStore: it
// runs only when a caller explicitly asks for it (CONTEXT D-12 -- no
// diagnostics on the everyday open/save path), and it tolerates a
// newer-than-supported schema_version exactly like LoadForRead does
// (CONTEXT D-10), so a newer file is still inspectable read-only. A
// healthy file returns empty FileLevelIssues and StructuralOK=true; a
// file-level corrupted file returns the non-"ok" integrity_check lines; a
// structurally-invalid-but-readable file returns StructuralOK=false with
// the validate error. Diagnose never crashes and never reports a bad file
// as healthy (05-04-PLAN.md Task 1).
package show

import (
	"errors"
	"fmt"
)

// DiagnosticReport is the combined file-level + structural health report
// Diagnose returns. SchemaVersion/Revision are read directly from the
// show_meta row (independent of whether the structural check below
// succeeds), so a structurally-invalid file still reports what
// schema_version/revision it claims to be at. MigrationRequired is a
// distinct signal from StructuralOK=false/StructuralError (WR-06): an
// older-schema file that simply needs `show open --confirm-migration` is
// an expected, recoverable state, not the same thing as a genuine
// validate() failure -- an operator reading StructuralError alone
// shouldn't have to distinguish "corrupted" from "just needs migrating"
// by parsing error text.
type DiagnosticReport struct {
	FileLevelIssues   []string `json:"file_level_issues,omitempty"`
	StructuralOK      bool     `json:"structural_ok"`
	StructuralError   string   `json:"structural_error,omitempty"`
	MigrationRequired bool     `json:"migration_required,omitempty"`
	SchemaVersion     int      `json:"schema_version"`
	Revision          int      `json:"revision"`
}

// Diagnose runs the full PRAGMA integrity_check (not the lighter
// quick_check -- CONTEXT D-11 asked for the thorough check) against the
// .golc file at path (resolved against root), then runs the structural
// check by reusing LoadForRead (which itself reuses validate()) rather
// than reimplementing decode/validate here. At this application's
// KB-to-low-MB .golc scale, integrity_check completes well under a second
// (05-RESEARCH.md Pitfall 4; TestDiagnoseCompletesUnderOneSecond confirms
// this empirically). integrity_check is designed to report corruption as
// descriptive text rows rather than failing the query itself, so a
// query-level error here means the file could not even be opened for
// inspection -- reported as GOLC_SHOW_DIAGNOSE_FAILED, the one case
// Diagnose returns a non-nil error instead of a report.
func Diagnose(root, path string) (report DiagnosticReport, err error) {
	db, openErr := openStore(root, path)
	if openErr != nil {
		return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", openErr)
	}
	defer func() {
		if closeErr := checkpointAndClose(db); closeErr != nil && err == nil {
			err = fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: closing store: %v", closeErr)
		}
	}()

	rows, err := db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
	}
	var fileLevelIssues []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
		}
		if line != "ok" {
			fileLevelIssues = append(fileLevelIssues, line)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
	}
	if err := rows.Close(); err != nil {
		return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
	}

	// SchemaVersion/Revision come straight from show_meta, independent of
	// whether the structural check below succeeds -- a structurally
	// invalid file still reports what schema_version/revision it claims.
	meta, ok, err := readMeta(db)
	if err != nil {
		return DiagnosticReport{}, fmt.Errorf("GOLC_SHOW_DIAGNOSE_FAILED: %v", err)
	}
	schemaVersion, revision := SchemaVersion, 0
	if ok {
		schemaVersion, revision = meta.SchemaVersion, meta.Revision
	}

	// Structural check: LoadForRead tolerates a newer-than-supported
	// schema_version (D-10), so a newer file is still inspectable
	// read-only rather than refused outright.
	_, structuralErr := LoadForRead(root, path)

	var migrationRequired ErrSchemaMigrationRequired
	report = DiagnosticReport{
		FileLevelIssues:   fileLevelIssues,
		StructuralOK:      structuralErr == nil,
		MigrationRequired: errors.As(structuralErr, &migrationRequired),
		SchemaVersion:     schemaVersion,
		Revision:          revision,
	}
	if structuralErr != nil {
		report.StructuralError = structuralErr.Error()
	}
	return report, nil
}
