// linear_test.go proves the command-level idempotent-replay contract for
// "linear apply" (01-30-PLAN.md, Gap 1 / CR-01): production runLinearApply
// must reach apply.RunApply's freshness/resume orchestration, not the bare
// lower-level apply.Apply. It is package command (an internal test) because
// it must override the unexported applyRemoteClientFactory injection point
// and call runLinearApply directly, matching tools_test.go's exact
// declaration convention (MustDeclareScope beside a TestScope{PascalName}
// marker).
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/apply"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "linear-apply-replay",
	Summary: "Command-level idempotent-replay proof for runLinearApply: stale re-apply rejection and within-lineage journal resume.",
})

// ---------------------------------------------------------------------------
// Fixture repository: a minimal synthetic .planning/ tree BuildCatalog can
// dynamically discover (mirrors internal/trace/catalog/catalog_test.go's
// newFixtureRepository, trimmed to exactly one phase/requirement/plan/task
// -- this test does not exercise dynamic-discovery breadth, only that
// runLinearApply's production apply path is actually reached).
// ---------------------------------------------------------------------------

const replayFixturePhaseSlug = "01-replay-phase"

const replayFixtureLinearMap = `{
  "schema": 1,
  "repository": { "project_id": "project:golc", "name": "GOLC" },
  "active_milestone": { "milestone_id": "milestone:v1", "name": "GOLC v1" },
  "remote_mappings": []
}
`

func writeReplayFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func replayFixtureRoadmap() string {
	return strings.Join([]string{
		"# Roadmap: Replay Fixture",
		"",
		"## Phases",
		"",
		"- [ ] **Phase 1: Replay Fixture Phase** - Fixture phase.",
		"",
		"## Phase Details",
		"",
		"### Phase 1: Replay Fixture Phase",
		"",
		"**Goal:** Fixture goal.",
		"**Requirements:** TSTR-01",
		"",
	}, "\n")
}

func replayFixtureRequirements() string {
	return strings.Join([]string{
		"# Requirements: Replay Fixture",
		"",
		"- [ ] **TSTR-01**: Fixture requirement text.",
		"",
	}, "\n")
}

func replayFixturePlan() string {
	return strings.Join([]string{
		"---",
		"phase: " + replayFixturePhaseSlug,
		"plan: 01",
		"type: execute",
		"---",
		"",
		"## Objective",
		"",
		"Fixture plan body.",
		"",
		"<tasks>",
		"",
		`<task type="auto" tdd="true">`,
		"  <name>Task 1: Only executable</name>",
		"  <action>Do fixture work.</action>",
		"</task>",
		"",
		"</tasks>",
		"",
	}, "\n")
}

// newApplyReplayFixtureRepository builds a synthetic repository root with a
// complete .planning/ tree BuildCatalog can process offline: one phase, one
// requirement, one plan, one task -- five non-project catalog entities in
// total (milestone, phase, plan, req, task).
func newApplyReplayFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	planning := filepath.Join(root, ".planning")
	phaseDir := filepath.Join(planning, "phases", replayFixturePhaseSlug)

	writeReplayFixtureFile(t, filepath.Join(planning, "linear-map.json"), replayFixtureLinearMap)
	writeReplayFixtureFile(t, filepath.Join(planning, "ROADMAP.md"), replayFixtureRoadmap())
	writeReplayFixtureFile(t, filepath.Join(planning, "REQUIREMENTS.md"), replayFixtureRequirements())
	writeReplayFixtureFile(t, filepath.Join(phaseDir, "01-01-PLAN.md"), replayFixturePlan())
	return root
}

// ---------------------------------------------------------------------------
// fakeReplayClient: a deterministic, credential-free, in-memory
// apply.RemoteClient that also exposes CaptureSnapshot() (transport.Snapshot,
// error) through the same type-assertion seam processLinearClient does
// (CONTEXT: the GREEN fix reaches CaptureSnapshot via
// interface{ CaptureSnapshot() (transport.Snapshot, error) }). It mirrors
// internal/trace/apply/apply_test.go's fakeRemoteClient shape (byUUID map,
// createCalls/updateCalls counters, a settable "fail once" hook) so this
// test proves the same D-17/D-21 discipline reaches production code.
// ---------------------------------------------------------------------------

type fakeReplayClient struct {
	byUUID         map[string]apply.RemoteState
	nextUUID       int
	fixedUpdatedAt string
	failCreate     map[string]error
	failUpdate     map[string]error
	createCalls    map[string]int
	updateCalls    map[string]int
}

func newFakeReplayClient() *fakeReplayClient {
	return &fakeReplayClient{
		byUUID:         map[string]apply.RemoteState{},
		fixedUpdatedAt: "2026-07-21T00:00:00Z",
		failCreate:     map[string]error{},
		failUpdate:     map[string]error{},
		createCalls:    map[string]int{},
		updateCalls:    map[string]int{},
	}
}

func (f *fakeReplayClient) totalCreateCalls() int {
	total := 0
	for _, count := range f.createCalls {
		total += count
	}
	return total
}

func (f *fakeReplayClient) ReadByUUID(uuid string) (apply.RemoteState, bool, error) {
	state, found := f.byUUID[uuid]
	return state, found, nil
}

// ReadByMarker always reports "not found," deliberately mirroring
// processLinearClient.ReadByMarker's permanent stub (internal/command/linear.go,
// retained this round per the ReadByMarker decision recorded in
// 01-30-PLAN.md and WR-05): the compiled adapter has no search-by-marker
// wire operation, so the one real RemoteClient this repository ships can
// never discover a not-yet-linked object by its D-14 footer. A fake that
// implemented genuine marker discovery here would mask exactly the CR-01
// bug this test exists to catch -- with a working ReadByMarker,
// applyUnlinkedOperation's own discovery step would already prevent a
// second apply from duplicating anything, even through the unguarded bare
// apply.Apply entry point, and this test would pass against the RED code
// for the wrong reason.
func (f *fakeReplayClient) ReadByMarker(localID string) (apply.RemoteState, bool, error) {
	return apply.RemoteState{}, false, nil
}

func replayOperationFields(op reconcile.Operation) (map[string]string, error) {
	fields := map[string]string{}
	if len(op.After) == 0 {
		return fields, nil
	}
	if err := strictjson.DecodeStrict(op.After, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (f *fakeReplayClient) Create(op reconcile.Operation) (apply.RemoteState, error) {
	f.createCalls[op.LocalID]++
	if err, fail := f.failCreate[op.LocalID]; fail {
		delete(f.failCreate, op.LocalID) // a transient failure is retryable: the next attempt may succeed
		return apply.RemoteState{}, err
	}
	fields, err := replayOperationFields(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	marker, err := reconcile.RenderMarker(op.LocalID)
	if err != nil {
		return apply.RemoteState{}, err
	}
	f.nextUUID++
	uuid := fmt.Sprintf("cccccccc-0000-0000-0000-%012d", f.nextUUID)
	state := apply.RemoteState{
		LinearUUID:  uuid,
		Fields:      fields,
		Description: "Managed by GOLC. Do not edit this footer.\n\n" + marker,
		UpdatedAt:   f.fixedUpdatedAt,
	}
	f.byUUID[uuid] = state
	return state, nil
}

func (f *fakeReplayClient) Update(op reconcile.Operation, uuid, expectedUpdatedAt string) (apply.RemoteState, error) {
	f.updateCalls[op.LocalID]++
	if err, fail := f.failUpdate[op.LocalID]; fail {
		delete(f.failUpdate, op.LocalID)
		return apply.RemoteState{}, err
	}
	existing, found := f.byUUID[uuid]
	if !found {
		return apply.RemoteState{}, fmt.Errorf("fakeReplayClient.Update: unknown uuid %s", uuid)
	}
	fields, err := replayOperationFields(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	existing.Fields = fields
	existing.UpdatedAt = f.fixedUpdatedAt
	f.byUUID[uuid] = existing
	return existing, nil
}

// CaptureSnapshot returns every object the fake has already created, so a
// later apply of a stale plan observes the drifted/now-populated remote
// state that triggers ValidatePlanFreshness's staleness rejection (CONTEXT
// D-18), matching processLinearClient.CaptureSnapshot's targeted read-back
// semantics.
func (f *fakeReplayClient) CaptureSnapshot() (transport.Snapshot, error) {
	uuids := make([]string, 0, len(f.byUUID))
	for uuid := range f.byUUID {
		uuids = append(uuids, uuid)
	}
	sort.Strings(uuids)
	records := make([]transport.RemoteRecord, 0, len(uuids))
	for _, uuid := range uuids {
		state := f.byUUID[uuid]
		records = append(records, transport.RemoteRecord{
			LinearUUID:  state.LinearUUID,
			Description: state.Description,
			Fields:      state.Fields,
			UpdatedAt:   state.UpdatedAt,
		})
	}
	return transport.Snapshot{Status: transport.SnapshotComplete, Records: records}, nil
}

var _ apply.RemoteClient = (*fakeReplayClient)(nil)

type captureSnapshotter interface {
	CaptureSnapshot() (transport.Snapshot, error)
}

var _ captureSnapshotter = (*fakeReplayClient)(nil)

// ---------------------------------------------------------------------------
// Shared plan-building helper: mirrors runLinearPreview's own construction
// path (buildRemotePreview) exactly -- catalog.MigrateV1ToV2 supplies
// intents/mappings, an explicit transport.Snapshot supplies the captured
// remote scope, and reconcile.BuildCompletePreview builds the canonical,
// hash-bound plan. The plan is then written to disk with the same strict
// encoder (strictjson.CanonicalEncode) the apply route decodes with
// strictjson.DecodeStrict, so decodeAndValidatePlanStrict's own integrity
// check (recomputed hash must match) passes.
// ---------------------------------------------------------------------------

func buildAndWriteReplayPlan(t *testing.T, root string, snapshot transport.Snapshot, outName string) (reconcile.Plan, string) {
	t.Helper()
	migrated, err := catalog.MigrateV1ToV2(root)
	if err != nil {
		t.Fatalf("MigrateV1ToV2: %v", err)
	}
	intents := intentsFromMigratedMap(migrated)
	plan, err := reconcile.BuildCompletePreview(intents, migrated.RemoteMappings, snapshot, nil)
	if err != nil {
		t.Fatalf("BuildCompletePreview: %v", err)
	}
	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		t.Fatalf("CanonicalEncode(plan): %v", err)
	}
	planPath := filepath.Join(root, outName)
	if err := os.WriteFile(planPath, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", planPath, err)
	}
	return plan, planPath
}

// withOverriddenApplyFactory swaps applyRemoteClientFactory for the
// duration of the test, restoring the prior value via t.Cleanup so no
// override leaks across tests (CONTEXT: applyRemoteClientFactory is a
// package-level injection point every "linear apply" test must restore).
func withOverriddenApplyFactory(t *testing.T, client apply.RemoteClient) {
	t.Helper()
	previous := applyRemoteClientFactory
	applyRemoteClientFactory = func(string) (apply.RemoteClient, error) { return client, nil }
	t.Cleanup(func() { applyRemoteClientFactory = previous })
}

// emptyCompleteSnapshot is the credential-free, complete-but-empty
// transport.Snapshot the very first preview against a fresh repository is
// always built from -- no entity has ever been created remotely yet.
var emptyCompleteSnapshot = transport.Snapshot{Status: transport.SnapshotComplete}

// TestScopeLinearApplyReplay is the exact quick-test marker for scope
// "linear-apply-replay" (test --quick --scope linear-apply-replay).
func TestScopeLinearApplyReplay(t *testing.T) {
	t.Run("stale re-apply of an already-achieved plan is rejected without any duplicate create", func(t *testing.T) {
		root := newApplyReplayFixtureRepository(t)
		plan, planPath := buildAndWriteReplayPlan(t, root, emptyCompleteSnapshot, "apply-plan-a.json")

		fake := newFakeReplayClient()
		withOverriddenApplyFactory(t, fake)

		request := Request{Root: root, Args: []string{planPath, "--plan-id", plan.PlanID}}

		first := runLinearApply(request)
		if first.ExitCode != 0 {
			t.Fatalf("first apply: ExitCode = %d, want 0; stderr: %s", first.ExitCode, first.Stderr)
		}
		firstCreateCalls := fake.totalCreateCalls()
		if firstCreateCalls != len(plan.Operations) {
			t.Fatalf("first apply: total Create calls = %d, want %d (one per operation)", firstCreateCalls, len(plan.Operations))
		}

		// No intervening "linear preview": apply the exact same plan file a
		// second time. The bare apply.Apply entry point has no freshness
		// check at all, so today this re-creates every operation as a
		// second, duplicate remote object -- this is the RED failure this
		// task documents.
		second := runLinearApply(request)
		if second.ExitCode == 0 {
			t.Fatalf("second apply of the exact same plan unexpectedly succeeded (ExitCode 0); want a GOLC_APPLY_ staleness rejection; stdout: %s", second.Stdout)
		}
		if !strings.Contains(string(second.Stderr), "GOLC_APPLY_") {
			t.Fatalf("second apply stderr = %q, want it to contain GOLC_APPLY_ (the D-18 freshness rejection)", second.Stderr)
		}
		if got := fake.totalCreateCalls(); got != firstCreateCalls {
			t.Fatalf("second apply issued %d additional Create call(s) (total now %d, want unchanged %d) -- a stale re-apply duplicated remote objects instead of being rejected", got-firstCreateCalls, got, firstCreateCalls)
		}
	})

	t.Run("a within-lineage retry after a transient failure resumes without duplicating the achieved prefix", func(t *testing.T) {
		root := newApplyReplayFixtureRepository(t)
		snapshot := emptyCompleteSnapshot
		plan, planPath := buildAndWriteReplayPlan(t, root, snapshot, "apply-plan-b.json")
		if len(plan.Operations) == 0 {
			t.Fatal("fixture plan has no operations to exercise a partial failure against")
		}
		// The first operation in canonical D-17 order (milestone, rank 0)
		// is the one this test induces a single transient failure against,
		// so the achieved prefix on the first attempt is empty and the
		// journal/mapping state a later retry observes is unchanged from
		// the moment this plan was built -- the exact "within the same
		// journal lineage, no intervening preview" scenario a persisted,
		// non-empty achieved prefix cannot satisfy once commitApplyResults
		// has already folded a newly linked UUID back into
		// .planning/linear-map.json (CONTEXT D-18: that fold is itself a
		// legitimate repository-state change, so a later apply of the
		// original plan bytes is correctly rejected as stale -- see the
		// sibling staleness subtest above; a fresh "linear preview" is the
		// documented recovery, exercised in the third phase below).
		failLocalID := plan.Operations[0].LocalID

		fake := newFakeReplayClient()
		fake.failCreate[failLocalID] = fmt.Errorf("GOLC_APPLY_TEST_TRANSIENT: simulated transient failure for %s", failLocalID)
		withOverriddenApplyFactory(t, fake)

		request := Request{Root: root, Args: []string{planPath, "--plan-id", plan.PlanID}}

		first := runLinearApply(request)
		if first.ExitCode != 0 {
			t.Fatalf("first (partial) apply: ExitCode = %d, want 0 (a transient per-operation failure is reported inside the report, not as a route-level error); stderr: %s", first.ExitCode, first.Stderr)
		}
		if fake.createCalls[failLocalID] != 1 {
			t.Fatalf("createCalls[%s] after first attempt = %d, want exactly 1 (the induced failure)", failLocalID, fake.createCalls[failLocalID])
		}
		if _, found := fake.byUUID[failLocalID]; found {
			t.Fatalf("fake unexpectedly recorded a remote object for %s after its induced Create failure", failLocalID)
		}

		journalPath := planPath + ".journal.json"
		if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
			t.Fatalf("journal file %s exists after an empty achieved prefix (nothing should have been committed): stat err = %v", journalPath, err)
		}

		// Retry the exact same plan file within the same journal lineage:
		// nothing was committed by the failed first attempt, so
		// .planning/linear-map.json and a freshly captured snapshot are
		// byte-identical to what they were when this plan was built --
		// ValidatePlanFreshness passes, ResumePrefix sees no journal (the
		// achieved prefix was empty) and every operation is attempted, and
		// the previously failing local ID succeeds this time because
		// fakeReplayClient's failure hook fires exactly once.
		second := runLinearApply(request)
		if second.ExitCode != 0 {
			t.Fatalf("retry apply: ExitCode = %d, want 0; stderr: %s", second.ExitCode, second.Stderr)
		}
		if fake.createCalls[failLocalID] != 2 {
			t.Fatalf("createCalls[%s] after retry = %d, want exactly 2 (one failed attempt, one successful retry -- never re-attempted a third time)", failLocalID, fake.createCalls[failLocalID])
		}
		successfulObjects := 0
		for _, state := range fake.byUUID {
			marker, found, err := reconcile.ParseMarker(state.Description)
			if err == nil && found && marker.LocalID == failLocalID {
				successfulObjects++
			}
		}
		if successfulObjects != 1 {
			t.Fatalf("fake has %d remote objects for %s after retry, want exactly 1 (no duplicate)", successfulObjects, failLocalID)
		}
		for _, op := range plan.Operations {
			if op.LocalID == failLocalID {
				continue
			}
			if fake.createCalls[op.LocalID] != 1 {
				t.Fatalf("createCalls[%s] = %d after the retry completed every operation exactly once, want 1", op.LocalID, fake.createCalls[op.LocalID])
			}
		}

		// A subsequent stale re-apply of the very same plan bytes (now that
		// every operation is achieved and committed) is still rejected --
		// resuming a transient failure never weakens the D-18 staleness
		// guard for a truly stale replay.
		third := runLinearApply(request)
		if third.ExitCode == 0 {
			t.Fatalf("third apply of the now-fully-achieved plan unexpectedly succeeded; want a GOLC_APPLY_ staleness rejection")
		}
		if !strings.Contains(string(third.Stderr), "GOLC_APPLY_") {
			t.Fatalf("third apply stderr = %q, want it to contain GOLC_APPLY_", third.Stderr)
		}
		if fake.createCalls[failLocalID] != 2 {
			t.Fatalf("createCalls[%s] after the rejected third apply = %d, want unchanged 2", failLocalID, fake.createCalls[failLocalID])
		}
	})
}
