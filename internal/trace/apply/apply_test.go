// apply_test.go covers the D-17/D-18/D-21 exact-plan apply contract: an
// untampered plan with a hash mismatch or wrong schema is rejected
// outright (ValidatePlanIntegrity), a plan that no longer matches current
// repository/remote state is rejected before any mutation
// (ValidatePlanFreshness/remote-stale.json), mutating apply is refused
// independent of workflow YAML from a pull_request CI event
// (GuardAgainstPullRequestMutation), a clean create plan applies once and
// a later re-preview replays as an exact no-op, a create whose remote
// outcome timed out is discovered by its exact D-14 marker footer before
// any retry so it never duplicates (remote-timeout-after-create.json),
// only ApplyRemoval -- never the regular create/update Apply path -- can
// ever archive or unlink a remote object, a rate-limited mutation stops
// all further writes and reports safe retry metadata
// (remote-partial-apply.json), and a later apply of the exact same plan
// resumes only the exact already-achieved prefix without replaying a
// completed mutation.
//
// It is an external test package so it can declare its quick-test scopes
// through the command package's exact registration entrypoint, matching
// the linear-preview-contract/linear-reconcile pattern from Plans 01-10
// and 01-23.
package apply_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lnorton89/golc/internal/command"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/apply"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-apply-core",
	Summary: "Exact-plan apply integrity/freshness/PR guards, no-op replay, and timeout-after-create discovery tests.",
})

var _ = command.MustDeclareScope(command.ScopeRegistration{
	Scope:   "linear-apply-resume",
	Summary: "Partial-apply stop/report and exact achieved-prefix journal resume tests.",
})

// repositoryRoot walks upward from the test working directory to the real
// repository root (the directory owning golc.project.toml).
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err, "getwd")
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "golc.project.toml")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "repository root with golc.project.toml not found above test directory")
		dir = parent
	}
}

func requireErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	require.ErrorContains(t, err, code)
}

func strPtr(s string) *string { return &s }

func operationOrder(operations []reconcile.Operation) []string {
	ids := make([]string, 0, len(operations))
	for _, op := range operations {
		ids = append(ids, op.LocalID)
	}
	return ids
}

func resultOrder(results []apply.OperationResult) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, fmt.Sprintf("%s:%s", result.LocalID, result.Status))
	}
	return ids
}

// fakeRemoteClient is a deterministic, credential-free, in-memory
// apply.RemoteClient: it never performs network, SDK, or credential
// access, records every Create/Update call it is asked to make, and lets
// tests script exactly one failure per local ID so a retry can succeed
// (simulating a transient rate limit) without ever fabricating a second
// remote object for the same local ID.
type fakeRemoteClient struct {
	byUUID         map[string]apply.RemoteState
	nextUUID       int
	fixedUpdatedAt string
	failCreate     map[string]error
	failUpdate     map[string]error
	createCalls    map[string]int
	updateCalls    map[string]int
}

func newFakeRemoteClient() *fakeRemoteClient {
	return &fakeRemoteClient{
		byUUID:         map[string]apply.RemoteState{},
		fixedUpdatedAt: "2026-07-21T00:00:00Z",
		failCreate:     map[string]error{},
		failUpdate:     map[string]error{},
		createCalls:    map[string]int{},
		updateCalls:    map[string]int{},
	}
}

func (f *fakeRemoteClient) seed(state apply.RemoteState) {
	f.byUUID[state.LinearUUID] = state
}

func (f *fakeRemoteClient) ReadByUUID(uuid string) (apply.RemoteState, bool, error) {
	state, found := f.byUUID[uuid]
	return state, found, nil
}

func (f *fakeRemoteClient) ReadByMarker(localID string) (apply.RemoteState, bool, error) {
	var matches []apply.RemoteState
	for _, state := range f.byUUID {
		marker, found, err := reconcile.ParseMarker(state.Description)
		if err != nil {
			return apply.RemoteState{}, false, err
		}
		if found && marker.LocalID == localID {
			matches = append(matches, state)
		}
	}
	switch len(matches) {
	case 0:
		return apply.RemoteState{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		return apply.RemoteState{}, false, fmt.Errorf("GOLC_APPLY_TEST_DISCOVERY_AMBIGUOUS: %s matches %d records", localID, len(matches))
	}
}

func operationFields(op reconcile.Operation) (map[string]string, error) {
	fields := map[string]string{}
	if len(op.After) == 0 {
		return fields, nil
	}
	if err := json.Unmarshal(op.After, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func (f *fakeRemoteClient) Create(op reconcile.Operation) (apply.RemoteState, error) {
	f.createCalls[op.LocalID]++
	if err, fail := f.failCreate[op.LocalID]; fail {
		delete(f.failCreate, op.LocalID) // a rate limit is transient: the next attempt may succeed
		return apply.RemoteState{}, err
	}
	fields, err := operationFields(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	marker, err := reconcile.RenderMarker(op.LocalID)
	if err != nil {
		return apply.RemoteState{}, err
	}
	f.nextUUID++
	uuid := fmt.Sprintf("bbbbbbbb-0000-0000-0000-%012d", f.nextUUID)
	state := apply.RemoteState{
		LinearUUID:  uuid,
		Fields:      fields,
		Description: "Managed by GOLC. Do not edit this footer.\n\n" + marker,
		UpdatedAt:   f.fixedUpdatedAt,
	}
	f.byUUID[uuid] = state
	return state, nil
}

func (f *fakeRemoteClient) Update(op reconcile.Operation, uuid, expectedUpdatedAt string) (apply.RemoteState, error) {
	f.updateCalls[op.LocalID]++
	if err, fail := f.failUpdate[op.LocalID]; fail {
		delete(f.failUpdate, op.LocalID)
		return apply.RemoteState{}, err
	}
	existing, found := f.byUUID[uuid]
	if !found {
		return apply.RemoteState{}, fmt.Errorf("fakeRemoteClient.Update: unknown uuid %s", uuid)
	}
	fields, err := operationFields(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	existing.Fields = fields
	existing.UpdatedAt = f.fixedUpdatedAt
	f.byUUID[uuid] = existing
	return existing, nil
}

var _ apply.RemoteClient = (*fakeRemoteClient)(nil)

// snapshotFixture is the self-contained JSON shape shared by every apply
// scenario fixture: repository intent, the credential-free remote mapping
// set, the last-synchronized baseline, and the transport-neutral snapshot
// used to build the original plan.
type snapshotFixture struct {
	Description string                   `json:"description"`
	Intents     []reconcile.Intent       `json:"intents"`
	Mappings    []catalog.RemoteMapping  `json:"mappings"`
	Baselines   []reconcile.SyncBaseline `json:"baselines"`
	Snapshot    transport.Snapshot       `json:"snapshot"`
}

// staleFixture extends snapshotFixture with the drifted post-preview
// snapshot remote-stale.json exercises (CONTEXT D-18).
type staleFixture struct {
	snapshotFixture
	DriftedSnapshot transport.Snapshot `json:"drifted_snapshot"`
}

// timeoutFixture extends snapshotFixture with the achieved remote record
// remote-timeout-after-create.json exercises (CONTEXT D-17/D-21).
type timeoutFixture struct {
	snapshotFixture
	AchievedRecord transport.RemoteRecord `json:"achieved_record"`
}

// partialApplyFixture extends snapshotFixture with the local ID whose
// mutation remote-partial-apply.json exercises as rate-limited.
type partialApplyFixture struct {
	snapshotFixture
	FailLocalID string `json:"fail_local_id"`
}

func loadFixture(t *testing.T, name string, out any) {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture %s", path)
	require.NoError(t, strictjson.DecodeStrict(data, out), "decode fixture %s", path)
}

// remoteStateFromRecord converts a fixture's transport.RemoteRecord into
// the apply.RemoteState shape a RemoteClient reports.
func remoteStateFromRecord(record transport.RemoteRecord) apply.RemoteState {
	return apply.RemoteState{
		LinearUUID:  record.LinearUUID,
		Fields:      record.Fields,
		Description: record.Description,
		UpdatedAt:   record.UpdatedAt,
	}
}

// twoOpFixture builds a small, clean two-operation hierarchy (a project
// milestone and one owned phase) with no existing remote state, so both
// operations are plain creates -- enough to exercise a full clean apply
// and a subsequent no-op replay.
func twoOpFixture() ([]reconcile.Intent, []catalog.RemoteMapping, transport.Snapshot, []reconcile.SyncBaseline) {
	intents := []reconcile.Intent{
		{
			LocalID: "milestone:v1", Kind: "milestone", LinearType: "project",
			ParentLocalID: "project:golc", Fields: map[string]string{"title": "GOLC v1"},
		},
		{
			LocalID: "phase:01", Kind: "phase", LinearType: "project_milestone",
			ParentLocalID: "milestone:v1", Fields: map[string]string{"title": "Offline Foundation and Delivery Traceability"},
		},
	}
	mappings := []catalog.RemoteMapping{
		{RepoID: "milestone:v1", LinearType: "project", Status: "pending"},
		{RepoID: "phase:01", LinearType: "project_milestone", Status: "pending"},
	}
	return intents, mappings, transport.Snapshot{Status: transport.SnapshotComplete}, nil
}

// TestScopeLinearApplyCore is the exact quick-test marker for scope
// "linear-apply-core" (test --quick --scope linear-apply-core).
func TestScopeLinearApplyCore(t *testing.T) {
	t.Run("ValidatePlanIntegrity accepts an untampered plan and rejects a tampered schema or hash", func(t *testing.T) {
		intents, mappings, snapshot, baselines := twoOpFixture()
		plan, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
		require.NoError(t, err, "BuildCompletePreview")
		require.NoError(t, apply.ValidatePlanIntegrity(plan), "ValidatePlanIntegrity (untampered)")

		tamperedID := plan
		tamperedID.PlanID = "0000000000000000000000000000000000000000000000000000000000000"
		requireErrorCode(t, apply.ValidatePlanIntegrity(tamperedID), "GOLC_APPLY_PLAN_HASH")

		tamperedSchema := plan
		tamperedSchema.SchemaVersion = plan.SchemaVersion + 1
		requireErrorCode(t, apply.ValidatePlanIntegrity(tamperedSchema), "GOLC_APPLY_PLAN_SCHEMA")
	})

	t.Run("ValidatePlanFreshness accepts an unchanged plan and rejects the remote-stale fixture before any mutation", func(t *testing.T) {
		var fixture staleFixture
		loadFixture(t, "remote-stale.json", &fixture)

		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		require.NoError(t, apply.ValidatePlanFreshness(plan, fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines), "ValidatePlanFreshness (unchanged)")
		err = apply.ValidatePlanFreshness(plan, fixture.Intents, fixture.Mappings, fixture.DriftedSnapshot, fixture.Baselines)
		requireErrorCode(t, err, "GOLC_APPLY_PLAN_STALE")
	})

	t.Run("GuardAgainstPullRequestMutation blocks a pull_request CI event independently and allows everything else", func(t *testing.T) {
		pullRequest := func(string) (string, bool) { return "pull_request", true }
		err := apply.GuardAgainstPullRequestMutation(pullRequest)
		requireErrorCode(t, err, "GOLC_APPLY_PR_BLOCKED")

		push := func(string) (string, bool) { return "push", true }
		require.NoError(t, apply.GuardAgainstPullRequestMutation(push), "GuardAgainstPullRequestMutation (push)")
		absent := func(string) (string, bool) { return "", false }
		require.NoError(t, apply.GuardAgainstPullRequestMutation(absent), "GuardAgainstPullRequestMutation (absent)")
		require.NoError(t, apply.GuardAgainstPullRequestMutation(nil), "GuardAgainstPullRequestMutation (nil lookup)")
	})

	t.Run("Apply completes a clean create plan and a later re-preview replays as an exact no-op", func(t *testing.T) {
		intents, mappings, snapshot, baselines := twoOpFixture()
		plan, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
		require.NoError(t, err, "BuildCompletePreview")
		client := newFakeRemoteClient()
		results := apply.Apply(client, plan, mappings)
		require.Len(t, results, 2, "results = %v, want 2 entries", resultOrder(results))
		for _, result := range results {
			require.Equal(t, apply.StatusCompleted, result.Status, "result %+v, want StatusCompleted", result)
			require.NotNil(t, result.LinearUUID, "result %+v has no LinearUUID", result)
			require.NotEmpty(t, *result.LinearUUID, "result %+v has no LinearUUID", result)
		}
		require.Equal(t, 1, client.createCalls["milestone:v1"], "createCalls = %v, want exactly one create per operation", client.createCalls)
		require.Equal(t, 1, client.createCalls["phase:01"], "createCalls = %v, want exactly one create per operation", client.createCalls)

		// Re-preview against the now-linked mapping set and the fake
		// client's actual current remote state, then re-apply: an exact
		// achieved postcondition plus a matching UUID must replay as a
		// no-op, never a second mutation.
		linkedMappings := make([]catalog.RemoteMapping, len(mappings))
		copy(linkedMappings, mappings)
		records := make([]transport.RemoteRecord, 0, len(results))
		for i, result := range results {
			linkedMappings[i].Status = "linked"
			linkedMappings[i].LinearUUID = result.LinearUUID
			state := client.byUUID[*result.LinearUUID]
			records = append(records, transport.RemoteRecord{
				LinearUUID:  state.LinearUUID,
				LinearType:  linkedMappings[i].LinearType,
				Description: state.Description,
				Fields:      state.Fields,
				UpdatedAt:   state.UpdatedAt,
			})
		}
		freshSnapshot := transport.Snapshot{Status: transport.SnapshotComplete, Records: records}
		plan2, err := reconcile.BuildCompletePreview(intents, linkedMappings, freshSnapshot, baselines)
		require.NoError(t, err, "BuildCompletePreview (re-preview)")
		for _, op := range plan2.Operations {
			require.NotNil(t, op.LinearUUID, "re-preview operation %s is not linked: %+v", op.LocalID, op)
		}

		results2 := apply.Apply(client, plan2, linkedMappings)
		for _, result := range results2 {
			require.Equal(t, apply.StatusNoop, result.Status, "replay result %+v, want StatusNoop", result)
		}
		require.Equal(t, 1, client.createCalls["milestone:v1"], "replay performed an extra create: createCalls = %v", client.createCalls)
		require.Equal(t, 1, client.createCalls["phase:01"], "replay performed an extra create: createCalls = %v", client.createCalls)
		require.Equal(t, 0, client.updateCalls["milestone:v1"], "replay performed an unnecessary update: updateCalls = %v", client.updateCalls)
		require.Equal(t, 0, client.updateCalls["phase:01"], "replay performed an unnecessary update: updateCalls = %v", client.updateCalls)
	})

	t.Run("Apply discovers an achieved timeout-after-create object by its exact marker footer before ever creating again", func(t *testing.T) {
		var fixture timeoutFixture
		loadFixture(t, "remote-timeout-after-create.json", &fixture)
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		require.Len(t, plan.Operations, 1, "Operations = %v, want exactly one create", operationOrder(plan.Operations))

		client := newFakeRemoteClient()
		client.seed(remoteStateFromRecord(fixture.AchievedRecord))
		client.failCreate["task:01-11.1"] = errors.New("Create must never be called: the achieved object already exists")

		results := apply.Apply(client, plan, fixture.Mappings)
		require.Len(t, results, 1, "results = %v, want 1 entry", resultOrder(results))
		result := results[0]
		require.Equal(t, apply.StatusNoop, result.Status, "result = %+v, want StatusNoop (already achieved)", result)
		require.NotNil(t, result.LinearUUID)
		require.Equal(t, fixture.AchievedRecord.LinearUUID, *result.LinearUUID)
		require.Equal(t, 0, client.createCalls["task:01-11.1"], "discovery must happen before any create attempt")
		require.Len(t, client.byUUID, 1, "want exactly the one pre-existing achieved object")
	})

	t.Run("ApplyRemoval is the only path that can archive or unlink, and it enforces the same pull-request guard", func(t *testing.T) {
		path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", "explicit-archive.json")
		data, err := os.ReadFile(path)
		require.NoError(t, err, "read fixture %s", path)
		var archiveFixture struct {
			Description string                `json:"description"`
			Mapping     catalog.RemoteMapping `json:"mapping"`
		}
		require.NoError(t, strictjson.DecodeStrict(data, &archiveFixture), "decode fixture %s", path)

		preview, err := reconcile.BuildArchivePreview(archiveFixture.Mapping)
		require.NoError(t, err, "BuildArchivePreview")

		pullRequest := func(string) (string, bool) { return "pull_request", true }
		fakeTransport := transport.NewFake(transport.Snapshot{Status: transport.SnapshotComplete})
		_, err = apply.ApplyRemoval(fakeTransport, preview, pullRequest)
		requireErrorCode(t, err, "GOLC_APPLY_PR_BLOCKED")
		require.Empty(t, fakeTransport.Applied(), "ApplyRemoval mutated despite the pull_request guard")

		mutation, err := apply.ApplyRemoval(fakeTransport, preview, nil)
		require.NoError(t, err, "ApplyRemoval")
		require.Equal(t, transport.MutationArchive, mutation.Kind, "mutation = %+v, want an archive mutation for %q", mutation, preview.LocalID)
		require.Equal(t, preview.LocalID, mutation.LocalID)
		require.Len(t, fakeTransport.Applied(), 1, "want exactly one recorded mutation")
	})
}

// TestScopeLinearApplyResume is the exact quick-test marker for scope
// "linear-apply-resume" (test --quick --scope linear-apply-resume).
func TestScopeLinearApplyResume(t *testing.T) {
	t.Run("RunApply rejects the remote-stale fixture before ever touching a RemoteClient", func(t *testing.T) {
		var fixture staleFixture
		loadFixture(t, "remote-stale.json", &fixture)
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		client := newFakeRemoteClient()
		_, _, err = apply.RunApply(client, plan, fixture.Intents, fixture.Mappings, fixture.DriftedSnapshot, fixture.Baselines, nil, nil)
		requireErrorCode(t, err, "GOLC_APPLY_PLAN_STALE")
		require.Empty(t, client.byUUID, "RunApply touched the RemoteClient before rejecting a stale plan")
		require.Empty(t, client.createCalls, "RunApply touched the RemoteClient before rejecting a stale plan")
		require.Empty(t, client.updateCalls, "RunApply touched the RemoteClient before rejecting a stale plan")
	})

	t.Run("RunApply refuses to run at all from a pull_request CI event", func(t *testing.T) {
		intents, mappings, snapshot, baselines := twoOpFixture()
		plan, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
		require.NoError(t, err, "BuildCompletePreview")
		client := newFakeRemoteClient()
		pullRequest := func(string) (string, bool) { return "pull_request", true }
		_, _, err = apply.RunApply(client, plan, intents, mappings, snapshot, baselines, nil, pullRequest)
		requireErrorCode(t, err, "GOLC_APPLY_PR_BLOCKED")
		require.Empty(t, client.createCalls, "RunApply attempted a mutation despite the pull_request guard")
	})

	t.Run("RunApply stops all writes on a retryable error, reports every operation state plus retry metadata, and resumes the exact achieved prefix without replay", func(t *testing.T) {
		var fixture partialApplyFixture
		loadFixture(t, "remote-partial-apply.json", &fixture)
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		require.NoError(t, err, "BuildCompletePreview")
		wantOrder := []string{"milestone:v1", "phase:01", "plan:01-11", "task:01-11.2"}
		require.Equal(t, wantOrder, operationOrder(plan.Operations))

		client := newFakeRemoteClient()
		client.failCreate[fixture.FailLocalID] = &apply.RetryableError{Reason: "rate limited", RetryAfter: "60s"}

		report1, journal1, err := apply.RunApply(client, plan, fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines, nil, nil)
		require.NoError(t, err, "RunApply (first attempt)")
		require.Len(t, report1.Results, len(wantOrder), "Results = %v, want an entry for every operation", resultOrder(report1.Results))
		require.Equal(t, apply.StatusCompleted, report1.Results[0].Status, "Results[0:2] = %v, want both completed before the throttled operation", resultOrder(report1.Results[:2]))
		require.Equal(t, apply.StatusCompleted, report1.Results[1].Status, "Results[0:2] = %v, want both completed before the throttled operation", resultOrder(report1.Results[:2]))
		throttled := report1.Results[2]
		require.Equal(t, fixture.FailLocalID, throttled.LocalID, "Results[2] = %+v, want a pending result for %q", throttled, fixture.FailLocalID)
		require.Equal(t, apply.StatusPending, throttled.Status, "Results[2] = %+v, want a pending result for %q", throttled, fixture.FailLocalID)
		require.Contains(t, throttled.Reason, "GOLC_APPLY_RETRYABLE")
		require.NotNil(t, throttled.RetryAfter)
		require.Equal(t, "60s", *throttled.RetryAfter)
		stopped := report1.Results[3]
		require.Equal(t, apply.StatusPending, stopped.Status, "Results[3] = %+v, want a stopped pending result", stopped)
		require.Contains(t, stopped.Reason, "GOLC_APPLY_STOPPED")
		require.Len(t, journal1.Results, 2, "journal1.Results = %v, want the exact 2-operation achieved prefix", resultOrder(journal1.Results))

		// Resuming with the exact same plan and the persisted journal must
		// never re-attempt the already-achieved prefix.
		report2, journal2, err := apply.RunApply(client, plan, fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines, &journal1, nil)
		require.NoError(t, err, "RunApply (resume)")
		require.Len(t, report2.Results, len(wantOrder), "resumed Results = %v, want an entry for every operation", resultOrder(report2.Results))
		for _, result := range report2.Results {
			require.Equal(t, apply.StatusCompleted, result.Status, "resumed result %+v, want StatusCompleted", result)
		}
		require.Len(t, journal2.Results, len(wantOrder), "journal2.Results = %v, want the full achieved prefix", resultOrder(journal2.Results))
		require.Equal(t, 1, client.createCalls["milestone:v1"], "resume replayed an already-achieved operation: createCalls = %v", client.createCalls)
		require.Equal(t, 1, client.createCalls["phase:01"], "resume replayed an already-achieved operation: createCalls = %v", client.createCalls)
		require.Equal(t, 2, client.createCalls[fixture.FailLocalID], "want exactly 2 (one failed attempt, one successful retry)")
		successfulObjects := 0
		for _, state := range client.byUUID {
			marker, found, err := reconcile.ParseMarker(state.Description)
			if err == nil && found && marker.LocalID == fixture.FailLocalID {
				successfulObjects++
			}
		}
		require.Equal(t, 1, successfulObjects, "want exactly 1 (no duplicate)")

		encoded, err := strictjson.CanonicalEncode(report2)
		require.NoError(t, err, "CanonicalEncode")
		goldenPath := filepath.Join(repositoryRoot(t), "tests", "golden", "linear-apply-report.json")
		golden, err := os.ReadFile(goldenPath)
		require.NoError(t, err, "read golden %s", goldenPath)
		require.Equal(t, string(golden), string(encoded), "resumed report does not match the committed golden")
	})

	t.Run("ResumePrefix rejects a journal bound to a different plan, an out-of-order journal, and drifted already-achieved state", func(t *testing.T) {
		intents, mappings, snapshot, baselines := twoOpFixture()
		plan, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
		require.NoError(t, err, "BuildCompletePreview")
		client := newFakeRemoteClient()
		report, _, err := apply.RunApply(client, plan, intents, mappings, snapshot, baselines, nil, nil)
		require.NoError(t, err, "RunApply")

		wrongPlan := apply.Journal{PlanID: "not-" + plan.PlanID, Results: report.Results}
		_, _, err = apply.ResumePrefix(plan, &wrongPlan, client)
		requireErrorCode(t, err, "GOLC_APPLY_RESUME_PLAN_MISMATCH")

		outOfOrder := apply.Journal{PlanID: plan.PlanID, Results: []apply.OperationResult{report.Results[1], report.Results[0]}}
		_, _, err = apply.ResumePrefix(plan, &outOfOrder, client)
		requireErrorCode(t, err, "GOLC_APPLY_RESUME_PREFIX_MISMATCH")

		drifted := apply.Journal{PlanID: plan.PlanID, Results: []apply.OperationResult{report.Results[0]}}
		driftedClient := newFakeRemoteClient()
		state := client.byUUID[*report.Results[0].LinearUUID]
		state.Fields = map[string]string{"title": "someone changed this after the journal was written"}
		driftedClient.seed(state)
		_, _, err = apply.ResumePrefix(plan, &drifted, driftedClient)
		requireErrorCode(t, err, "GOLC_APPLY_RESUME_DRIFT")

		achieved, remaining, err := apply.ResumePrefix(plan, nil, client)
		require.NoError(t, err, "ResumePrefix (nil journal)")
		require.Empty(t, achieved, "want no achieved")
		require.Len(t, remaining, len(plan.Operations), "want every operation remaining")
	})

	t.Run("CommitResultAtomically writes map/journal/report as one validated result and leaves prior state intact on failure", func(t *testing.T) {
		dir := t.TempDir()
		mapPath := filepath.Join(dir, "linear-map.json")
		journalPath := filepath.Join(dir, "linear-apply.journal.json")
		reportPath := filepath.Join(dir, "linear-apply.report.json")

		mapPayload := &catalog.Map{Schema: 2}
		mapPayload.Repository.ProjectID = "project:golc"
		mapPayload.Repository.Name = "GOLC"
		mapPayload.ActiveMilestone.MilestoneID = "milestone:v1"
		mapPayload.ActiveMilestone.Name = "GOLC v1"
		journal := apply.Journal{PlanID: "test-plan-id", Results: []apply.OperationResult{{LocalID: "milestone:v1", Status: apply.StatusCompleted, LinearUUID: strPtr("bbbbbbbb-0000-0000-0000-000000000001")}}}
		report := apply.Report{PlanID: "test-plan-id", Results: journal.Results}

		require.NoError(t, apply.CommitResultAtomically(mapPath, mapPayload, journalPath, journal, reportPath, report), "CommitResultAtomically")
		loaded, err := apply.LoadJournal(journalPath)
		require.NoError(t, err, "LoadJournal")
		require.NotNil(t, loaded, "LoadJournal = %+v, want the committed journal", loaded)
		require.Equal(t, journal.PlanID, loaded.PlanID)
		require.Len(t, loaded.Results, 1)
		_, err = os.Stat(mapPath)
		require.NoError(t, err, "map file missing after commit")
		_, err = os.Stat(reportPath)
		require.NoError(t, err, "report file missing after commit")

		// A staging failure for the third (report) destination must leave
		// no destination file behind at all -- not even the map/journal
		// that staged successfully before it.
		missingDirReportPath := filepath.Join(dir, "does-not-exist", "linear-apply.report.json")
		freshMapPath := filepath.Join(dir, "second-linear-map.json")
		freshJournalPath := filepath.Join(dir, "second-linear-apply.journal.json")
		err = apply.CommitResultAtomically(freshMapPath, mapPayload, freshJournalPath, journal, missingDirReportPath, report)
		require.Error(t, err, "CommitResultAtomically unexpectedly succeeded with an unwritable report destination")
		_, statErr := os.Stat(freshMapPath)
		require.Error(t, statErr, "CommitResultAtomically left a partially committed map file after a later staging failure")
		_, statErr = os.Stat(freshJournalPath)
		require.Error(t, statErr, "CommitResultAtomically left a partially committed journal file after a later staging failure")
	})

	t.Run("LoadJournal reports no error and a nil journal for a missing file", func(t *testing.T) {
		journal, err := apply.LoadJournal(filepath.Join(t.TempDir(), "does-not-exist.json"))
		require.NoError(t, err, "LoadJournal (missing)")
		require.Nil(t, journal, "LoadJournal (missing) = %+v, want nil", journal)
	})
}
