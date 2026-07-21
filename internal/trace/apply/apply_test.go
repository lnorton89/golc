// apply_test.go covers the D-17/D-18 exact-plan apply contract: an
// untampered plan with a hash mismatch or wrong schema is rejected
// outright (ValidatePlanIntegrity), a plan that no longer matches current
// repository/remote state is rejected before any mutation
// (ValidatePlanFreshness/remote-stale.json), mutating apply is refused
// independent of workflow YAML from a pull_request CI event
// (GuardAgainstPullRequestMutation), a clean create plan applies once and
// a later re-preview replays as an exact no-op, a create whose remote
// outcome timed out is discovered by its exact D-14 marker footer before
// any retry so it never duplicates (remote-timeout-after-create.json),
// and only ApplyRemoval -- never the regular create/update Apply path --
// can ever archive or unlink a remote object.
//
// It is an external test package so it can declare its quick-test scope
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
	"strings"
	"testing"

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

// repositoryRoot walks upward from the test working directory to the real
// repository root (the directory owning golc.project.toml).
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "golc.project.toml")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with golc.project.toml not found above test directory")
		}
		dir = parent
	}
}

func requireErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q, got nil", code)
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("error = %v, want it to contain %q", err, code)
	}
}

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

func loadFixture(t *testing.T, name string, out any) {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	if err := strictjson.DecodeStrict(data, out); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
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
		if err != nil {
			t.Fatalf("BuildCompletePreview: %v", err)
		}
		if err := apply.ValidatePlanIntegrity(plan); err != nil {
			t.Fatalf("ValidatePlanIntegrity (untampered): %v", err)
		}

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
		if err != nil {
			t.Fatalf("BuildCompletePreview: %v", err)
		}
		if err := apply.ValidatePlanFreshness(plan, fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines); err != nil {
			t.Fatalf("ValidatePlanFreshness (unchanged): %v", err)
		}
		err = apply.ValidatePlanFreshness(plan, fixture.Intents, fixture.Mappings, fixture.DriftedSnapshot, fixture.Baselines)
		requireErrorCode(t, err, "GOLC_APPLY_PLAN_STALE")
	})

	t.Run("GuardAgainstPullRequestMutation blocks a pull_request CI event independently and allows everything else", func(t *testing.T) {
		pullRequest := func(string) (string, bool) { return "pull_request", true }
		err := apply.GuardAgainstPullRequestMutation(pullRequest)
		requireErrorCode(t, err, "GOLC_APPLY_PR_BLOCKED")

		push := func(string) (string, bool) { return "push", true }
		if err := apply.GuardAgainstPullRequestMutation(push); err != nil {
			t.Fatalf("GuardAgainstPullRequestMutation (push): %v", err)
		}
		absent := func(string) (string, bool) { return "", false }
		if err := apply.GuardAgainstPullRequestMutation(absent); err != nil {
			t.Fatalf("GuardAgainstPullRequestMutation (absent): %v", err)
		}
		if err := apply.GuardAgainstPullRequestMutation(nil); err != nil {
			t.Fatalf("GuardAgainstPullRequestMutation (nil lookup): %v", err)
		}
	})

	t.Run("Apply completes a clean create plan and a later re-preview replays as an exact no-op", func(t *testing.T) {
		intents, mappings, snapshot, baselines := twoOpFixture()
		plan, err := reconcile.BuildCompletePreview(intents, mappings, snapshot, baselines)
		if err != nil {
			t.Fatalf("BuildCompletePreview: %v", err)
		}
		client := newFakeRemoteClient()
		results := apply.Apply(client, plan, mappings)
		if len(results) != 2 {
			t.Fatalf("results = %v, want 2 entries", resultOrder(results))
		}
		for _, result := range results {
			if result.Status != apply.StatusCompleted {
				t.Fatalf("result %+v, want StatusCompleted", result)
			}
			if result.LinearUUID == nil || *result.LinearUUID == "" {
				t.Fatalf("result %+v has no LinearUUID", result)
			}
		}
		if client.createCalls["milestone:v1"] != 1 || client.createCalls["phase:01"] != 1 {
			t.Fatalf("createCalls = %v, want exactly one create per operation", client.createCalls)
		}

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
		if err != nil {
			t.Fatalf("BuildCompletePreview (re-preview): %v", err)
		}
		for _, op := range plan2.Operations {
			if op.LinearUUID == nil {
				t.Fatalf("re-preview operation %s is not linked: %+v", op.LocalID, op)
			}
		}

		results2 := apply.Apply(client, plan2, linkedMappings)
		for _, result := range results2 {
			if result.Status != apply.StatusNoop {
				t.Fatalf("replay result %+v, want StatusNoop", result)
			}
		}
		if client.createCalls["milestone:v1"] != 1 || client.createCalls["phase:01"] != 1 {
			t.Fatalf("replay performed an extra create: createCalls = %v", client.createCalls)
		}
		if client.updateCalls["milestone:v1"] != 0 || client.updateCalls["phase:01"] != 0 {
			t.Fatalf("replay performed an unnecessary update: updateCalls = %v", client.updateCalls)
		}
	})

	t.Run("Apply discovers an achieved timeout-after-create object by its exact marker footer before ever creating again", func(t *testing.T) {
		var fixture timeoutFixture
		loadFixture(t, "remote-timeout-after-create.json", &fixture)
		plan, err := reconcile.BuildCompletePreview(fixture.Intents, fixture.Mappings, fixture.Snapshot, fixture.Baselines)
		if err != nil {
			t.Fatalf("BuildCompletePreview: %v", err)
		}
		if len(plan.Operations) != 1 {
			t.Fatalf("Operations = %v, want exactly one create", operationOrder(plan.Operations))
		}

		client := newFakeRemoteClient()
		client.seed(remoteStateFromRecord(fixture.AchievedRecord))
		client.failCreate["task:01-11.1"] = errors.New("Create must never be called: the achieved object already exists")

		results := apply.Apply(client, plan, fixture.Mappings)
		if len(results) != 1 {
			t.Fatalf("results = %v, want 1 entry", resultOrder(results))
		}
		result := results[0]
		if result.Status != apply.StatusNoop {
			t.Fatalf("result = %+v, want StatusNoop (already achieved)", result)
		}
		if result.LinearUUID == nil || *result.LinearUUID != fixture.AchievedRecord.LinearUUID {
			t.Fatalf("result.LinearUUID = %v, want %q", result.LinearUUID, fixture.AchievedRecord.LinearUUID)
		}
		if client.createCalls["task:01-11.1"] != 0 {
			t.Fatalf("createCalls[task:01-11.1] = %d, want 0 (discovery must happen before any create attempt)", client.createCalls["task:01-11.1"])
		}
		if len(client.byUUID) != 1 {
			t.Fatalf("byUUID has %d remote objects, want exactly the one pre-existing achieved object", len(client.byUUID))
		}
	})

	t.Run("ApplyRemoval is the only path that can archive or unlink, and it enforces the same pull-request guard", func(t *testing.T) {
		path := filepath.Join(repositoryRoot(t), "tests", "fixtures", "linear", "explicit-archive.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture %s: %v", path, err)
		}
		var archiveFixture struct {
			Description string                `json:"description"`
			Mapping     catalog.RemoteMapping `json:"mapping"`
		}
		if err := strictjson.DecodeStrict(data, &archiveFixture); err != nil {
			t.Fatalf("decode fixture %s: %v", path, err)
		}

		preview, err := reconcile.BuildArchivePreview(archiveFixture.Mapping)
		if err != nil {
			t.Fatalf("BuildArchivePreview: %v", err)
		}

		pullRequest := func(string) (string, bool) { return "pull_request", true }
		fakeTransport := transport.NewFake(transport.Snapshot{Status: transport.SnapshotComplete})
		_, err = apply.ApplyRemoval(fakeTransport, preview, pullRequest)
		requireErrorCode(t, err, "GOLC_APPLY_PR_BLOCKED")
		if len(fakeTransport.Applied()) != 0 {
			t.Fatalf("ApplyRemoval mutated despite the pull_request guard: %+v", fakeTransport.Applied())
		}

		mutation, err := apply.ApplyRemoval(fakeTransport, preview, nil)
		if err != nil {
			t.Fatalf("ApplyRemoval: %v", err)
		}
		if mutation.Kind != transport.MutationArchive || mutation.LocalID != preview.LocalID {
			t.Fatalf("mutation = %+v, want an archive mutation for %q", mutation, preview.LocalID)
		}
		if len(fakeTransport.Applied()) != 1 {
			t.Fatalf("fakeTransport.Applied() = %+v, want exactly one recorded mutation", fakeTransport.Applied())
		}
	})
}
