// journal.go implements the D-21 atomic achieved-prefix journal: a
// journal binds to one exact plan_id and records only the operations
// already confirmed completed or no-op, in plan order, so a later apply
// of the exact same plan can safely resume without ever replaying a
// completed mutation. ResumePrefix additionally re-reads every journaled
// object's current remote state before trusting it, so a manual edit
// between runs invalidates resume instead of silently being papered over.
// CommitResultAtomically persists the updated remote-mapping map, the
// journal, and the human-reviewable report as one validated result: every
// payload is encoded and staged to a temporary file before any
// destination is ever replaced, so an encode or staging failure leaves
// every previously committed file completely untouched.
package apply

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
)

// Journal is the atomic on-disk record of achieved apply progress for a
// single plan (CONTEXT D-21). It is always a separate artifact from
// reconcile.Plan -- timing/retry metadata never becomes part of the
// canonical hashed plan bytes, and no credential is ever recorded here.
type Journal struct {
	PlanID  string            `json:"plan_id"`
	Results []OperationResult `json:"results"`
}

// LoadJournal strictly decodes a committed journal file. A missing file
// is not an error: it means no prior apply attempt exists yet, and
// ResumePrefix treats a nil *Journal as "resume from the beginning."
func LoadJournal(path string) (*Journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("GOLC_APPLY_JOURNAL_LOAD: %s: %v", path, err)
	}
	var journal Journal
	if err := strictjson.DecodeStrict(data, &journal); err != nil {
		return nil, fmt.Errorf("GOLC_APPLY_JOURNAL_LOAD: %s: %v", path, err)
	}
	return &journal, nil
}

// ResumePrefix validates journal against plan and, if it checks out,
// returns the already-achieved results plus the exact remaining
// operations to attempt (CONTEXT D-21). journal == nil means no prior
// attempt exists: every operation remains to be attempted. A journal
// bound to a different plan_id, an out-of-order or too-long achieved
// list, a non-achieved entry, or a journaled object whose current remote
// state no longer matches what was recorded are all rejected outright --
// "unrelated changes invalidate resume" -- rather than silently resuming
// from a possibly-stale prefix.
func ResumePrefix(plan reconcile.Plan, journal *Journal, client RemoteClient) (achieved []OperationResult, remaining []reconcile.Operation, err error) {
	if journal == nil {
		return nil, append([]reconcile.Operation(nil), plan.Operations...), nil
	}
	if journal.PlanID != plan.PlanID {
		return nil, nil, fmt.Errorf(
			"GOLC_APPLY_RESUME_PLAN_MISMATCH: journal is bound to plan %s, current plan is %s; re-run linear preview and apply from a fresh plan",
			journal.PlanID, plan.PlanID)
	}
	if len(journal.Results) > len(plan.Operations) {
		return nil, nil, fmt.Errorf(
			"GOLC_APPLY_RESUME_PREFIX_MISMATCH: journal records %d achieved operations but plan has only %d",
			len(journal.Results), len(plan.Operations))
	}
	for index, result := range journal.Results {
		op := plan.Operations[index]
		if result.LocalID != op.LocalID {
			return nil, nil, fmt.Errorf(
				"GOLC_APPLY_RESUME_PREFIX_MISMATCH: journal position %d is %q, current plan position %d is %q; unrelated changes invalidate resume",
				index, result.LocalID, index, op.LocalID)
		}
		if result.Status != StatusCompleted && result.Status != StatusNoop {
			return nil, nil, fmt.Errorf(
				"GOLC_APPLY_RESUME_INCOMPLETE_PREFIX: journal position %d (%s) has non-achieved status %q",
				index, op.LocalID, result.Status)
		}
		if result.LinearUUID == nil {
			continue
		}
		current, found, readErr := client.ReadByUUID(*result.LinearUUID)
		if readErr != nil {
			return nil, nil, fmt.Errorf("GOLC_APPLY_RESUME_VERIFY_FAILED: %s: %v", op.LocalID, readErr)
		}
		if !found || !fieldsMatch(current, op) {
			return nil, nil, fmt.Errorf(
				"GOLC_APPLY_RESUME_DRIFT: %s: the remote state journaled as achieved no longer matches; re-run linear preview and apply from a fresh plan",
				op.LocalID)
		}
	}
	return append([]OperationResult(nil), journal.Results...), append([]reconcile.Operation(nil), plan.Operations[len(journal.Results):]...), nil
}

// stageTemp encodes payload to a contained temporary file beside
// destination and returns its path. The caller is responsible for
// renaming it into place (or removing it on any later failure).
func stageTemp(destination string, payload []byte) (string, error) {
	temp, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: staging %s: %v", destination, err)
	}
	path := temp.Name()
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		os.Remove(path)
		return "", fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: staging %s: %v", destination, err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: staging %s: %v", destination, err)
	}
	return path, nil
}

// CommitResultAtomically persists mapPayload, journal, and report to
// mapPath, journalPath, and reportPath as one validated result (CONTEXT
// D-21): every payload is canonically encoded and staged to a temporary
// file first; an encode failure for any of the three leaves every
// destination completely untouched, and a staging failure for any of the
// three cleans up whatever was already staged before returning. Only
// after all three stage successfully are they renamed into place.
func CommitResultAtomically(mapPath string, mapPayload *catalog.Map, journalPath string, journal Journal, reportPath string, report Report) error {
	mapEncoded, err := strictjson.CanonicalEncode(mapPayload)
	if err != nil {
		return fmt.Errorf("GOLC_APPLY_COMMIT_ENCODE: map: %v", err)
	}
	journalEncoded, err := strictjson.CanonicalEncode(journal)
	if err != nil {
		return fmt.Errorf("GOLC_APPLY_COMMIT_ENCODE: journal: %v", err)
	}
	reportEncoded, err := strictjson.CanonicalEncode(report)
	if err != nil {
		return fmt.Errorf("GOLC_APPLY_COMMIT_ENCODE: report: %v", err)
	}

	mapTemp, err := stageTemp(mapPath, mapEncoded)
	if err != nil {
		return err
	}
	journalTemp, err := stageTemp(journalPath, journalEncoded)
	if err != nil {
		os.Remove(mapTemp)
		return err
	}
	reportTemp, err := stageTemp(reportPath, reportEncoded)
	if err != nil {
		os.Remove(mapTemp)
		os.Remove(journalTemp)
		return err
	}

	if err := os.Rename(mapTemp, mapPath); err != nil {
		os.Remove(mapTemp)
		os.Remove(journalTemp)
		os.Remove(reportTemp)
		return fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: %s: %v", mapPath, err)
	}
	if err := os.Rename(journalTemp, journalPath); err != nil {
		os.Remove(journalTemp)
		os.Remove(reportTemp)
		return fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: %s: %v", journalPath, err)
	}
	if err := os.Rename(reportTemp, reportPath); err != nil {
		os.Remove(reportTemp)
		return fmt.Errorf("GOLC_APPLY_COMMIT_WRITE: %s: %v", reportPath, err)
	}
	return nil
}
