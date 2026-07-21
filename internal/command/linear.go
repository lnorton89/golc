// linear.go is the linear command file: it owns the "linear" routing scope
// and self-registers the offline catalog inspection route (CONTEXT D-03,
// D-11, D-14). It reads only committed repository planning artifacts
// through internal/trace/catalog; no network, Node, SDK, or Linear
// credential access is reachable from this route.
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/security"
	"github.com/lnorton89/golc/internal/strictjson"
	"github.com/lnorton89/golc/internal/trace/apply"
	"github.com/lnorton89/golc/internal/trace/catalog"
	"github.com/lnorton89/golc/internal/trace/reconcile"
	"github.com/lnorton89/golc/internal/trace/transport"
)

var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "linear",
	Summary: "Repository-owned planning identity catalog and Linear reconciliation operations.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear catalog",
	Summary: "Print the offline repository-owned planning identity catalog as deterministic JSON: linear catalog --offline --format json.",
	Handler: runLinearCatalog,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "linear preview",
	Summary: "Preview a complete-snapshot reconciliation plan against a fake transport fixture " +
		"(linear preview --snapshot <path> --out <path>) or the real process transport " +
		"(linear preview --remote --out <path>), targeted at every already-linked entity.",
	Handler: runLinearPreview,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route: "linear drift",
	Summary: "Report a read-only remote drift summary against every already-linked entity through the real " +
		"process transport, writing no file: linear drift --remote --read-only.",
	Handler: runLinearDrift,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear archive",
	Summary: "Build an explicit D-15 archive review preview for an already-linked local entity: linear archive --local-id <id> --preview-out <path>.",
	Handler: runLinearArchive,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear unlink",
	Summary: "Build an explicit D-15 unlink review preview for an already-linked local entity: linear unlink --local-id <id> --preview-out <path>.",
	Handler: runLinearUnlink,
})

// "linear map migrate --check"/"--write" and "linear status --offline"
// follow the same dash-word precedent generate.go/check.go document:
// router.go's route-word grammar rejects any word beginning with "-", so
// each is declared as one exact multi-word route ("linear map migrate",
// "linear status") whose handler strictly parses the remaining flag.

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear map migrate",
	Summary: "Check or write the canonical schema-2 identity map offline: linear map migrate --check|--write.",
	Handler: runLinearMapMigrate,
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear status",
	Summary: "Report offline mapping status with allowlisted safe fields; pending/null linkage is valid, not a failure: linear status --offline.",
	Handler: runLinearStatus,
})

// "linear apply {plan-file} --plan-id <id>" follows the same positional-
// argument-plus-flag shape "linear archive"/"linear unlink" already
// establish: the route word itself stays exactly two words ("linear
// apply"), and the plan file path is the first remaining argument, never a
// route word (router.go's route-word grammar rejects any word beginning
// with "-", so a flag can never become part of Route either way).

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "linear apply",
	Summary: "Apply an exact, already-reviewed reconciliation plan through the injected remote transport: linear apply {plan-file} --plan-id <id>.",
	Handler: runLinearApply,
})

// RemoteClientFactory builds the apply.RemoteClient "linear apply" mutates
// through. A test binary may still assign a fake-returning factory
// directly (matching Plan 01-24's original injection-seam contract); a
// nil factory means "linear apply" fails GOLC_LINEAR_TRANSPORT_UNAVAILABLE
// before any credential, subprocess, or mutation access is ever attempted
// (T-01-31/T-01-34).
type RemoteClientFactory func(root string) (apply.RemoteClient, error)

// applyRemoteClientFactory is the injection point "linear apply" resolves
// its RemoteClient through. Production wiring is newProcessRemoteClient
// (Plan 01-15): the real process-based Linear transport over the compiled
// project-local adapter (tools/linear-sync/dist/src/cli.js), reached
// through internal/trace/transport.ProcessClient and never a raw host
// PATH lookup.
var applyRemoteClientFactory RemoteClientFactory = newProcessRemoteClient

// catalogEntityView is the allowlisted JSON projection of one catalog
// entity: only durable identity, structure, and repository-relative
// source are emitted, never filesystem-absolute paths or remote state
// (T-01-23: information disclosure).
type catalogEntityView struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Parent  string `json:"parent,omitempty"`
	Display string `json:"display"`
	Source  string `json:"source"`
}

// catalogView is the deterministic JSON envelope for offline catalog
// output: entity order matches BuildCatalog's deterministic build order.
type catalogView struct {
	Entities []catalogEntityView `json:"entities"`
}

// parseOfflineJSONArgs accepts exactly the supported offline JSON form:
// --offline --format json.
func parseOfflineJSONArgs(usage string, args []string) error {
	offline := false
	format := ""
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--offline":
			offline = true
			i++
		case argument == "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("GOLC_LINEAR_USAGE: --format requires a value; usage: %s", usage)
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--format="):
			format = strings.TrimPrefix(argument, "--format=")
			i++
		default:
			return fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if !offline {
		return fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	if format != "json" {
		return fmt.Errorf("GOLC_LINEAR_FORMAT_UNSUPPORTED: %q is not supported (only json); usage: %s", format, usage)
	}
	return nil
}

// catalogEntityViews projects a built catalog's entities into the
// allowlisted JSON view, preserving deterministic build order.
func catalogEntityViews(built *catalog.Catalog) []catalogEntityView {
	views := make([]catalogEntityView, 0, len(built.Entities))
	for _, entity := range built.Entities {
		views = append(views, catalogEntityView{
			ID:      entity.ID,
			Kind:    string(entity.Kind),
			Parent:  entity.Parent,
			Display: entity.Display,
			Source:  entity.Source,
		})
	}
	return views
}

// runLinearCatalog serves the self-registered "linear catalog" route.
func runLinearCatalog(request Request) Result {
	if err := parseOfflineJSONArgs("linear catalog --offline --format json", request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	built, err := catalog.BuildCatalog(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := json.MarshalIndent(catalogView{Entities: catalogEntityViews(built)}, "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_CATALOG_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}

// parseLocalIDPreviewOutArgs accepts exactly the supported archive/unlink
// form: --local-id <id> --preview-out <path>.
func parseLocalIDPreviewOutArgs(usage string, args []string) (localID, outPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--local-id":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: --local-id requires a value; usage: %s", usage)
			}
			localID = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--local-id="):
			localID = strings.TrimPrefix(argument, "--local-id=")
			i++
		case argument == "--preview-out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: --preview-out requires a path; usage: %s", usage)
			}
			outPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--preview-out="):
			outPath = strings.TrimPrefix(argument, "--preview-out=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if localID == "" || outPath == "" {
		return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return localID, outPath, nil
}

// resolveWritablePath returns path unchanged when it is already absolute
// (the contributor's own explicit choice of where to write review
// output); otherwise it is resolved relative to root.
func resolveWritablePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

// intentsFromMigratedMap derives the exact repository intent set for
// BuildCompletePreview from the canonical schema-2 map: every non-project
// entity becomes an Intent whose sole owned field is its display title
// (CONTEXT D-11), and its Linear object type comes from the matching
// remote mapping already computed by catalog.MigrateV1ToV2.
func intentsFromMigratedMap(migrated *catalog.Map) []reconcile.Intent {
	linearTypeByID := make(map[string]string, len(migrated.RemoteMappings))
	for _, mapping := range migrated.RemoteMappings {
		linearTypeByID[mapping.RepoID] = mapping.LinearType
	}
	intents := make([]reconcile.Intent, 0, len(migrated.Entities))
	for _, entity := range migrated.Entities {
		if entity.Kind == string(catalog.KindProject) {
			continue // the repository root is never remote-mapped
		}
		intents = append(intents, reconcile.Intent{
			LocalID:       entity.LocalID,
			Kind:          entity.Kind,
			LinearType:    linearTypeByID[entity.LocalID],
			ParentLocalID: entity.ParentLocalID,
			Fields:        map[string]string{"title": entity.Display},
		})
	}
	return intents
}

// previewArgs is the parsed shape of one "linear preview" invocation:
// either the original fixture-based form (snapshotPath set) or the Plan
// 01-15 real remote form (remote set) -- mutually exclusive, both
// requiring --out.
type previewArgs struct {
	remote       bool
	snapshotPath string
	outPath      string
}

// parsePreviewArgs accepts exactly "--snapshot <path> --out <path>" or
// "--remote --out <path>".
func parsePreviewArgs(usage string, args []string) (previewArgs, error) {
	parsed := previewArgs{}
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--remote":
			parsed.remote = true
			i++
		case argument == "--snapshot":
			if i+1 >= len(args) {
				return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: --snapshot requires a path; usage: %s", usage)
			}
			parsed.snapshotPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--snapshot="):
			parsed.snapshotPath = strings.TrimPrefix(argument, "--snapshot=")
			i++
		case argument == "--out":
			if i+1 >= len(args) {
				return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: --out requires a path; usage: %s", usage)
			}
			parsed.outPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--out="):
			parsed.outPath = strings.TrimPrefix(argument, "--out=")
			i++
		default:
			return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if parsed.outPath == "" {
		return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	if parsed.remote && parsed.snapshotPath != "" {
		return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: --remote and --snapshot are mutually exclusive; usage: %s", usage)
	}
	if !parsed.remote && parsed.snapshotPath == "" {
		return previewArgs{}, fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return parsed, nil
}

// parseRemoteReadOnlyArgs accepts exactly "--remote --read-only" (in
// either order), the exact grammar "linear drift" requires.
func parseRemoteReadOnlyArgs(usage string, args []string) error {
	remote := false
	readOnly := false
	for _, argument := range args {
		switch argument {
		case "--remote":
			remote = true
		case "--read-only":
			readOnly = true
		default:
			return fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if !remote || !readOnly {
		return fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return nil
}

// buildRemotePreview captures a targeted snapshot through the real process
// transport and builds the exact D-17 complete-snapshot preview against
// the repository's own catalog intent and remote mapping set (Plan
// 01-15). The returned *processLinearClient is non-nil whenever a process
// was actually launched (even on a later failure), so the caller can
// always Close it. CaptureSnapshot here is targeted, not exhaustive
// (CONTEXT: see processLinearClient.CaptureSnapshot's doc comment for
// why): it reads back only every entity this repository already recorded
// a Linear UUID for; an entity with no recorded link is simply absent
// from the snapshot and is proposed as a create, exactly matching the
// empty-fake-SDK hierarchy scenario this plan's acceptance proves.
func buildRemotePreview(root string) (reconcile.Plan, *processLinearClient, error) {
	migrated, err := catalog.MigrateV1ToV2(root)
	if err != nil {
		return reconcile.Plan{}, nil, err
	}
	client, err := newProcessLinearClient(root, migrated)
	if err != nil {
		return reconcile.Plan{}, nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_UNAVAILABLE: %v", err)
	}
	snapshot, err := client.CaptureSnapshot()
	if err != nil {
		return reconcile.Plan{}, client, err
	}
	plan, err := reconcile.BuildCompletePreview(intentsFromMigratedMap(migrated), migrated.RemoteMappings, snapshot, nil)
	if err != nil {
		return reconcile.Plan{}, client, err
	}
	return plan, client, nil
}

// writePreviewPlan canonically encodes plan and writes it to outPath,
// shared by both "linear preview" forms.
func writePreviewPlan(root, outPath string, plan reconcile.Plan) Result {
	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_ENCODE_FAILED: %v\n", err))}
	}
	destination := resolveWritablePath(root, outPath)
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_WRITE_FAILED: %v\n", err))}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW: wrote %s\n", destination))}
}

// runLinearPreview serves the self-registered "linear preview" route. Its
// fixture form (--snapshot/--out) is unchanged from Plan 01-23: it
// captures a snapshot from a credential-free fake transport fixture. Its
// Plan 01-15 remote form (--remote/--out) captures a targeted snapshot
// through the real process transport instead. Both forms build the exact
// same D-17 complete-snapshot preview and write the identical canonical
// preview JSON shape to --out. Neither form ever mutates anything (T-01-SC).
func runLinearPreview(request Request) Result {
	usage := "linear preview --snapshot <path> --out <path> | linear preview --remote --out <path>"
	parsed, err := parsePreviewArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	if parsed.remote {
		plan, client, previewErr := buildRemotePreview(request.Root)
		if client != nil {
			defer client.Close()
		}
		if previewErr != nil {
			return Result{ExitCode: 1, Stderr: []byte(previewErr.Error() + "\n")}
		}
		return writePreviewPlan(request.Root, parsed.outPath, plan)
	}

	fake, err := transport.LoadFakeSnapshot(resolveWritablePath(request.Root, parsed.snapshotPath))
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	snapshot, err := fake.CaptureSnapshot()
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	migrated, err := catalog.MigrateV1ToV2(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	plan, err := reconcile.BuildCompletePreview(intentsFromMigratedMap(migrated), migrated.RemoteMappings, snapshot, nil)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return writePreviewPlan(request.Root, parsed.outPath, plan)
}

// runLinearDrift serves the self-registered "linear drift" route (CONTEXT
// D-16: pull-request CI may run explicit, read-only drift checks): it
// runs the exact same targeted remote preview computation "linear preview
// --remote" does, but never writes a file -- only a stable one-line
// summary of what a preview would find.
func runLinearDrift(request Request) Result {
	usage := "linear drift --remote --read-only"
	if err := parseRemoteReadOnlyArgs(usage, request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	plan, client, err := buildRemotePreview(request.Root)
	if client != nil {
		defer client.Close()
	}
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	summary := fmt.Sprintf(
		"GOLC_LINEAR_DRIFT: plan_id=%s operations=%d conflicts=%d\n",
		plan.PlanID, len(plan.Operations), len(plan.Conflicts))
	return Result{Stdout: []byte(summary)}
}

// ---------------------------------------------------------------------------
// Process-based Linear transport (Plan 01-15): the real apply.RemoteClient
// adapter over internal/trace/transport.ProcessClient and the compiled
// tools/linear-sync/dist/src/cli.js adapter. This block owns zero
// reconciliation policy -- it only translates already-decided
// reconcile.Operation values into the exact wire shape
// tools/linear-sync/src/protocol.ts's decodeOperation accepts, and
// translates its responses back into apply.RemoteState /
// transport.RemoteRecord. It lives here (package command), not in
// internal/trace/transport, because apply.RemoteState is a package apply
// type and package apply already imports package transport (guard.go,
// engine.go); a transport->apply dependency here would close that cycle.
// ---------------------------------------------------------------------------

// linearSyncWorkdirOverrideEnv lets an acceptance test point the process
// transport at an isolated workspace (for example one with a fake
// @linear/sdk package injected into its node_modules) instead of the real
// tools/linear-sync tree, without touching production route resolution.
// It is never documented as a supported contributor-facing variable in
// .env.example -- only tests/acceptance/linear-transport.ps1 sets it.
const linearSyncWorkdirOverrideEnv = "GOLC_LINEAR_SYNC_WORKDIR"

// linearSyncTimeoutOverrideEnv lets an acceptance test shorten the default
// per-call deadline so a deliberately induced timeout scenario stays fast;
// production code never sets it.
const linearSyncTimeoutOverrideEnv = "GOLC_LINEAR_SYNC_TIMEOUT_MS"

// defaultLinearTransportTimeout bounds every process transport call when
// no override is set.
const defaultLinearTransportTimeout = 30 * time.Second

// resolveLinearSyncWorkspace returns the working directory and compiled
// adapter entrypoint the process transport launches. In production this
// is always project-local: tools/linear-sync and its own
// dist/src/cli.js, built by "golc.ps1 bootstrap --include linear-sync"
// plus "golc.ps1 build --scope linear-sdk" -- never a host PATH lookup.
func resolveLinearSyncWorkspace(root string) (workDir, scriptPath string) {
	if override := strings.TrimSpace(os.Getenv(linearSyncWorkdirOverrideEnv)); override != "" {
		return override, filepath.Join(override, "dist", "src", "cli.js")
	}
	workDir = filepath.Join(root, "tools", "linear-sync")
	return workDir, filepath.Join(workDir, "dist", "src", "cli.js")
}

// linearTransportCallTimeout resolves the per-call deadline every process
// transport Call enforces.
func linearTransportCallTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv(linearSyncTimeoutOverrideEnv)); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultLinearTransportTimeout
}

// linearEntityKindForOperationKind maps a catalog/reconcile Operation.Kind
// ("milestone", "phase", "req", "plan", "task") to the exact granular
// Linear SDK entity kind tools/linear-sync/src/protocol.ts's EntityKind
// declares. catalog.RemoteMapping.LinearType intentionally stays coarser
// ("project"/"project_milestone"/"issue" -- catalog/migrate.go's
// linearTypeForKind) since it only needs to name the remote object
// *category* for repository bookkeeping; this transport needs the
// granular parent_issue/requirement_issue/task_subissue distinction to
// build the correct create/update fields (CONTEXT: hierarchy
// milestone->project, phase->project_milestone, plan/requirement->issue
// under the phase's project milestone, task->sub-issue under its plan).
func linearEntityKindForOperationKind(kind string) (string, error) {
	switch catalog.Kind(kind) {
	case catalog.KindMilestone:
		return "project", nil
	case catalog.KindPhase:
		return "project_milestone", nil
	case catalog.KindPlan:
		return "parent_issue", nil
	case catalog.KindRequirement:
		return "requirement_issue", nil
	case catalog.KindTask:
		return "task_subissue", nil
	default:
		return "", fmt.Errorf("GOLC_LINEAR_TRANSPORT_KIND_UNMAPPED: %q has no Linear SDK entity kind", kind)
	}
}

// wireOperation is the exact request line tools/linear-sync/src/protocol.ts's
// decodeOperation accepts: {entity, action, linearUUID?, fields?}.
type wireOperation struct {
	Entity     string         `json:"entity"`
	Action     string         `json:"action"`
	LinearUUID string         `json:"linearUUID,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// wireRecord mirrors tools/linear-sync/src/protocol.ts's NormalizedRecord
// exactly (field names and shape).
type wireRecord struct {
	LinearUUID  string            `json:"linearUUID"`
	LinearType  string            `json:"linearType"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Fields      map[string]string `json:"fields"`
	UpdatedAt   string            `json:"updatedAt"`
}

// wireReadResult mirrors tools/linear-sync/src/protocol.ts's ReadResult.
type wireReadResult struct {
	Found  bool        `json:"found"`
	Record *wireRecord `json:"record,omitempty"`
}

// wireDiagnostic mirrors tools/linear-sync/src/protocol.ts's
// TransportDiagnostic: the exact allowlisted metadata surface a partial
// GraphQL error or rate-limit/timeout signal may ever expose. This type
// never gains a free-text field.
type wireDiagnostic struct {
	Operation  string  `json:"operation"`
	Path       string  `json:"path,omitempty"`
	Code       string  `json:"code,omitempty"`
	Request    string  `json:"request,omitempty"`
	Endpoint   string  `json:"endpoint,omitempty"`
	Complexity float64 `json:"complexity,omitempty"`
	Reset      string  `json:"reset,omitempty"`
}

// wireMutationOutcome mirrors tools/linear-sync/src/protocol.ts's
// MutationOutcome discriminated union.
type wireMutationOutcome struct {
	Status     string          `json:"status"`
	Record     *wireRecord     `json:"record,omitempty"`
	Diagnostic *wireDiagnostic `json:"diagnostic,omitempty"`
}

// diagnosticSummary renders o's diagnostic as a stable, already-redacted
// line through internal/security.SafeDiagnostic -- the exact same
// allowlist-and-redact contract every other GOLC diagnostic surface uses
// (T-01-44/T-01-45), so a hostile or malformed diagnostic can never smuggle
// unexpected content into a Go-level error message.
func (o wireMutationOutcome) diagnosticSummary() string {
	if o.Diagnostic == nil {
		return "no diagnostic reported"
	}
	return security.SafeDiagnostic{
		Code:    o.Diagnostic.Code,
		Message: o.Diagnostic.Operation,
		Fields: map[string]string{
			"path":     o.Diagnostic.Path,
			"request":  o.Diagnostic.Request,
			"endpoint": o.Diagnostic.Endpoint,
			"reset":    o.Diagnostic.Reset,
		},
	}.String()
}

// decodeMutationOutcome strictly decodes one create/update response line.
func decodeMutationOutcome(raw json.RawMessage) (wireMutationOutcome, error) {
	var outcome wireMutationOutcome
	if err := json.Unmarshal(raw, &outcome); err != nil {
		return wireMutationOutcome{}, fmt.Errorf("GOLC_LINEAR_TRANSPORT_DECODE: %v", err)
	}
	if outcome.Status != "confirmed" && outcome.Status != "unknown" {
		return wireMutationOutcome{}, fmt.Errorf("GOLC_LINEAR_TRANSPORT_PROTOCOL_NOISE: unrecognized mutation status %q", outcome.Status)
	}
	return outcome, nil
}

// stateFromWireRecord projects a wireRecord into apply.RemoteState.
// tools/linear-sync/src/adapter.ts's normalize() always emits an empty
// "fields" map (every owned value crosses the wire through the unified
// top-level "title" -- Project/ProjectMilestone's own SDK "name" included,
// since normalize() sets title from `handle.name ?? handle.title`); this
// repository's only owned field is "title" (CONTEXT D-11;
// intentsFromMigratedMap), so Fields is synthesized here to
// {"title": record.Title} rather than passed through record.Fields
// (always {}) -- otherwise fieldsMatch (apply/model.go) could never
// observe a match and every already-linked entity would spuriously
// re-update on every apply, breaking "replay is all no-op."
func stateFromWireRecord(record wireRecord) apply.RemoteState {
	return apply.RemoteState{
		LinearUUID:  record.LinearUUID,
		Fields:      map[string]string{"title": record.Title},
		Description: record.Description,
		UpdatedAt:   record.UpdatedAt,
	}
}

// buildEntityFields translates op's already-canonical owned fields
// (op.After, currently always just {"title": ...} -- CONTEXT D-11) plus
// op.DiscoveryMarker (the D-14 identity footer every managed remote
// description must carry) into the exact create/update Fields shape
// entityKind's protocol.ts type requires. teamID (from LINEAR_TEAM_ID,
// D-19/D-20: read only by this explicit remote path, never logged) and
// parentUUID (this operation's already-resolved parent, or "" for the
// root milestone->Project mapping, which has no Linear-side parent)
// supply the workspace-scoped identifiers repository intent never owns.
func buildEntityFields(op reconcile.Operation, entityKind, teamID, parentUUID string) (map[string]any, error) {
	var owned map[string]string
	if len(op.After) > 0 {
		if err := json.Unmarshal(op.After, &owned); err != nil {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_FIELDS_DECODE: %s: %v", op.LocalID, err)
		}
	}
	title := owned["title"]
	marker := op.DiscoveryMarker

	switch entityKind {
	case "project":
		if teamID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_TEAM_ID_MISSING: LINEAR_TEAM_ID must be set to create/update a Linear Project (%s)", op.LocalID)
		}
		return map[string]any{"name": title, "description": marker, "teamIds": []string{teamID}}, nil
	case "project_milestone":
		if parentUUID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_PARENT_UNRESOLVED: %s has no resolved parent project", op.LocalID)
		}
		return map[string]any{"name": title, "description": marker, "projectId": parentUUID}, nil
	case "parent_issue", "requirement_issue":
		if teamID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_TEAM_ID_MISSING: LINEAR_TEAM_ID must be set to create/update a Linear Issue (%s)", op.LocalID)
		}
		if parentUUID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_PARENT_UNRESOLVED: %s has no resolved parent project milestone", op.LocalID)
		}
		return map[string]any{"title": title, "description": marker, "teamId": teamID, "projectMilestoneId": parentUUID}, nil
	case "task_subissue":
		if teamID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_TEAM_ID_MISSING: LINEAR_TEAM_ID must be set to create/update a Linear Issue (%s)", op.LocalID)
		}
		if parentUUID == "" {
			return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_PARENT_UNRESOLVED: %s has no resolved parent issue", op.LocalID)
		}
		return map[string]any{"title": title, "description": marker, "teamId": teamID, "parentId": parentUUID}, nil
	default:
		return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_KIND_UNMAPPED: %q", entityKind)
	}
}

// processLinearClient is the real apply.RemoteClient (Create/Update/
// ReadByUUID/ReadByMarker) and remote-preview reader (CaptureSnapshot)
// over one launched transport.ProcessClient. uuidByLocalID/kindByUUID are
// seeded once at construction from the repository's own already-recorded
// remote mappings and grow as this run's own Create/Update calls succeed,
// so a later sibling operation in the same dependency-ordered apply run
// (for example a task depending on the plan just created) can resolve its
// parent's Linear UUID without a second round trip.
type processLinearClient struct {
	proc   *transport.ProcessClient
	teamID string

	mu            sync.Mutex
	uuidByLocalID map[string]string
	kindByUUID    map[string]string
}

// newProcessLinearClient launches the process transport and seeds its
// identity caches from migrated's already-recorded remote mappings.
// LINEAR_API_KEY must already be set (D-19/D-20: read only here, from the
// process environment, never logged); LINEAR_TEAM_ID is validated lazily,
// only when a create/update actually needs it (buildEntityFields).
func newProcessLinearClient(root string, migrated *catalog.Map) (*processLinearClient, error) {
	if strings.TrimSpace(os.Getenv("LINEAR_API_KEY")) == "" {
		return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_CREDENTIAL_MISSING: LINEAR_API_KEY is not set")
	}
	nodeExecutable, err := resolvePinnedNodeExecutable(root)
	if err != nil {
		return nil, err
	}
	workDir, scriptPath := resolveLinearSyncWorkspace(root)
	proc, err := transport.NewProcessClient(transport.ProcessConfig{
		NodeExecutable: nodeExecutable,
		ScriptPath:     scriptPath,
		WorkDir:        workDir,
		Env:            os.Environ(),
		Timeout:        linearTransportCallTimeout(),
	})
	if err != nil {
		return nil, err
	}

	client := &processLinearClient{
		proc:          proc,
		teamID:        strings.TrimSpace(os.Getenv("LINEAR_TEAM_ID")),
		uuidByLocalID: map[string]string{},
		kindByUUID:    map[string]string{},
	}
	entityKindByID := make(map[string]string, len(migrated.Entities))
	for _, entity := range migrated.Entities {
		entityKindByID[entity.LocalID] = entity.Kind
	}
	for _, mapping := range migrated.RemoteMappings {
		if mapping.LinearUUID == nil {
			continue
		}
		client.uuidByLocalID[mapping.RepoID] = *mapping.LinearUUID
		if entityKind, kindErr := linearEntityKindForOperationKind(entityKindByID[mapping.RepoID]); kindErr == nil {
			client.kindByUUID[*mapping.LinearUUID] = entityKind
		}
	}
	return client, nil
}

// Close ends the underlying process transport (io.Closer).
func (c *processLinearClient) Close() error {
	return c.proc.Close()
}

var _ io.Closer = (*processLinearClient)(nil)

func (c *processLinearClient) lookupKind(uuid string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kind, ok := c.kindByUUID[uuid]
	return kind, ok
}

func (c *processLinearClient) remember(localID, uuid, kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if localID != "" && uuid != "" {
		c.uuidByLocalID[localID] = uuid
	}
	if uuid != "" && kind != "" {
		c.kindByUUID[uuid] = kind
	}
}

// resolveParentUUID resolves op.ParentLocalID to its already-known Linear
// UUID, either from a prior run (seeded at construction) or from this same
// run's own earlier Create/Update (remember). The repository root
// (project:*) is the only parent that legitimately resolves to "" -- the
// milestone's own Create needs no Linear-side parent object at all.
func (c *processLinearClient) resolveParentUUID(op reconcile.Operation) (string, error) {
	if op.ParentLocalID == "" {
		return "", nil
	}
	if parsedParent, parseErr := catalog.ParseID(op.ParentLocalID); parseErr == nil && parsedParent.Kind == catalog.KindProject {
		return "", nil
	}
	c.mu.Lock()
	uuid, ok := c.uuidByLocalID[op.ParentLocalID]
	c.mu.Unlock()
	if !ok {
		return "", fmt.Errorf(
			"GOLC_LINEAR_TRANSPORT_PARENT_UNRESOLVED: %s has no resolved remote parent %s; its own create/update must complete first",
			op.LocalID, op.ParentLocalID)
	}
	return uuid, nil
}

// call marshals one wireOperation and sends it through the process
// transport, enforcing the client's configured deadline.
func (c *processLinearClient) call(entity, action, uuid string, fields map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(wireOperation{Entity: entity, Action: action, LinearUUID: uuid, Fields: fields})
	if err != nil {
		return nil, fmt.Errorf("GOLC_LINEAR_TRANSPORT_ENCODE: %v", err)
	}
	return c.proc.Call(context.Background(), payload)
}

// readRecord issues one "read" operation by immutable UUID and returns the
// full wireRecord (including title/linearType, which apply.RemoteState
// does not carry but CaptureSnapshot needs).
func (c *processLinearClient) readRecord(uuid string) (wireRecord, bool, error) {
	kind, ok := c.lookupKind(uuid)
	if !ok {
		return wireRecord{}, false, fmt.Errorf("GOLC_LINEAR_TRANSPORT_UNKNOWN_UUID: no known entity kind for linear UUID %q", uuid)
	}
	raw, err := c.call(kind, "read", uuid, nil)
	if err != nil {
		return wireRecord{}, false, err
	}
	var result wireReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return wireRecord{}, false, fmt.Errorf("GOLC_LINEAR_TRANSPORT_DECODE: %v", err)
	}
	if !result.Found || result.Record == nil {
		return wireRecord{}, false, nil
	}
	return *result.Record, true, nil
}

// ReadByUUID implements apply.RemoteClient.
func (c *processLinearClient) ReadByUUID(uuid string) (apply.RemoteState, bool, error) {
	record, found, err := c.readRecord(uuid)
	if err != nil || !found {
		return apply.RemoteState{}, found, err
	}
	return stateFromWireRecord(record), true, nil
}

// ReadByMarker implements apply.RemoteClient. The compiled adapter's
// strict NDJSON contract (tools/linear-sync/src/protocol.ts) supports
// only a read-by-immutable-UUID operation -- there is no list/search-by-
// description action to discover a not-yet-linked remote object by its
// D-14 marker footer. This client therefore never fabricates a match
// here: an uncertain outcome from a prior Create/Update (see Create/Update
// below) surfaces as a StatusPending result (internal/trace/apply/engine.go)
// that requires a human-reviewed re-run rather than an unattended blind
// retry -- matching this phase's "explicit reviewed" mutation ethos
// (CONTEXT D-16/D-21; see also this plan's SUMMARY.md "Known Stubs" for
// the follow-up this leaves for a future plan that extends
// protocol.ts/adapter.ts with a real search/connection operation).
func (c *processLinearClient) ReadByMarker(localID string) (apply.RemoteState, bool, error) {
	return apply.RemoteState{}, false, nil
}

// Create implements apply.RemoteClient.
func (c *processLinearClient) Create(op reconcile.Operation) (apply.RemoteState, error) {
	entityKind, err := linearEntityKindForOperationKind(op.Kind)
	if err != nil {
		return apply.RemoteState{}, err
	}
	parentUUID, err := c.resolveParentUUID(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	fields, err := buildEntityFields(op, entityKind, c.teamID, parentUUID)
	if err != nil {
		return apply.RemoteState{}, err
	}
	raw, err := c.call(entityKind, "create", "", fields)
	if err != nil {
		return apply.RemoteState{}, err
	}
	outcome, err := decodeMutationOutcome(raw)
	if err != nil {
		return apply.RemoteState{}, err
	}
	if outcome.Status != "confirmed" || outcome.Record == nil {
		return apply.RemoteState{}, fmt.Errorf("GOLC_LINEAR_TRANSPORT_CREATE_UNCERTAIN: %s: %s", op.LocalID, outcome.diagnosticSummary())
	}
	state := stateFromWireRecord(*outcome.Record)
	c.remember(op.LocalID, state.LinearUUID, entityKind)
	return state, nil
}

// Update implements apply.RemoteClient.
func (c *processLinearClient) Update(op reconcile.Operation, uuid, expectedUpdatedAt string) (apply.RemoteState, error) {
	entityKind, err := linearEntityKindForOperationKind(op.Kind)
	if err != nil {
		return apply.RemoteState{}, err
	}
	parentUUID, err := c.resolveParentUUID(op)
	if err != nil {
		return apply.RemoteState{}, err
	}
	fields, err := buildEntityFields(op, entityKind, c.teamID, parentUUID)
	if err != nil {
		return apply.RemoteState{}, err
	}
	raw, err := c.call(entityKind, "update", uuid, fields)
	if err != nil {
		return apply.RemoteState{}, err
	}
	outcome, err := decodeMutationOutcome(raw)
	if err != nil {
		return apply.RemoteState{}, err
	}
	if outcome.Status != "confirmed" || outcome.Record == nil {
		return apply.RemoteState{}, fmt.Errorf("GOLC_LINEAR_TRANSPORT_UPDATE_UNCERTAIN: %s: %s", op.LocalID, outcome.diagnosticSummary())
	}
	state := stateFromWireRecord(*outcome.Record)
	c.remember(op.LocalID, state.LinearUUID, entityKind)
	return state, nil
}

// CaptureSnapshot builds a targeted transport.Snapshot by reading back,
// through the real process transport, every entity this repository
// already recorded a Linear UUID for -- never an exhaustive connection
// scan (CONTEXT: tools/linear-sync/src/protocol.ts's Operation contract
// exposes only read-by-UUID/create/update, no list/search action). An
// entity with no recorded link is simply absent from the returned
// records, exactly as if it had never been observed remotely: reconcile's
// discoverObservations (diff.go) already treats an absent observation as
// "plan a create," so a not-yet-linked entity is proposed for creation
// exactly like the documented empty-fake-SDK hierarchy scenario, and an
// already-linked entity whose remote state now matches is proposed as a
// no-op -- the exact "replay performs zero creates/updates" property this
// plan's acceptance proves. Status is always "complete": a targeted read
// either succeeds for every already-linked entity or this function
// returns the first error encountered (never a silent partial result).
func (c *processLinearClient) CaptureSnapshot() (transport.Snapshot, error) {
	c.mu.Lock()
	localIDs := make([]string, 0, len(c.uuidByLocalID))
	for localID := range c.uuidByLocalID {
		localIDs = append(localIDs, localID)
	}
	c.mu.Unlock()
	sort.Strings(localIDs)

	records := make([]transport.RemoteRecord, 0, len(localIDs))
	for _, localID := range localIDs {
		c.mu.Lock()
		uuid := c.uuidByLocalID[localID]
		c.mu.Unlock()
		record, found, err := c.readRecord(uuid)
		if err != nil {
			return transport.Snapshot{}, fmt.Errorf("GOLC_LINEAR_TRANSPORT_SNAPSHOT_FAILED: %s: %v", localID, err)
		}
		if !found {
			continue
		}
		records = append(records, transport.RemoteRecord{
			LinearUUID:  record.LinearUUID,
			LinearType:  record.LinearType,
			Title:       record.Title,
			Description: record.Description,
			// Synthesized to {"title": ...}, matching stateFromWireRecord's
			// same rationale: the adapter's own "fields" is always empty.
			Fields:    map[string]string{"title": record.Title},
			UpdatedAt: record.UpdatedAt,
		})
	}
	return transport.Snapshot{Status: transport.SnapshotComplete, Records: records}, nil
}

// newProcessRemoteClient is the production RemoteClientFactory
// implementation (Plan 01-15): it loads the repository's own canonical
// remote mapping set and constructs a processLinearClient over it.
func newProcessRemoteClient(root string) (apply.RemoteClient, error) {
	migrated, err := catalog.MigrateV1ToV2(root)
	if err != nil {
		return nil, err
	}
	return newProcessLinearClient(root, migrated)
}

// writeArchivePreview is the shared handler body for the "linear archive"
// and "linear unlink" routes: it resolves localID's already-recorded
// remote mapping from the canonical schema-2 map, builds the requested
// explicit D-15 removal preview, and writes it to --preview-out.
func writeArchivePreview(request Request, localID, outPath string, build func(catalog.RemoteMapping) (reconcile.ArchivePreview, error)) Result {
	migrated, err := catalog.MigrateV1ToV2(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	var mapping catalog.RemoteMapping
	found := false
	for _, candidate := range migrated.RemoteMappings {
		if candidate.RepoID == localID {
			mapping = candidate
			found = true
			break
		}
	}
	if !found {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_ARCHIVE_UNKNOWN: %q has no recorded remote mapping\n", localID))}
	}
	preview, err := build(mapping)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	payload, err := strictjson.CanonicalEncode(preview)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_ARCHIVE_ENCODE_FAILED: %v\n", err))}
	}
	destination := resolveWritablePath(request.Root, outPath)
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_ARCHIVE_WRITE_FAILED: %v\n", err))}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_LINEAR_ARCHIVE: wrote %s\n", destination))}
}

// runLinearArchive serves the self-registered "linear archive" route
// (CONTEXT D-15): it never infers removal from local absence, only from
// this explicit, already-reviewed invocation against an already-linked
// entity.
func runLinearArchive(request Request) Result {
	localID, outPath, err := parseLocalIDPreviewOutArgs("linear archive --local-id <id> --preview-out <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	return writeArchivePreview(request, localID, outPath, reconcile.BuildArchivePreview)
}

// runLinearUnlink serves the self-registered "linear unlink" route
// (CONTEXT D-15): only the local-to-remote link is previewed for
// removal; the remote object itself is left untouched.
func runLinearUnlink(request Request) Result {
	localID, outPath, err := parseLocalIDPreviewOutArgs("linear unlink --local-id <id> --preview-out <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	return writeArchivePreview(request, localID, outPath, reconcile.BuildUnlinkPreview)
}

// parseMigrateArgs accepts exactly one of "--check" or "--write".
func parseMigrateArgs(usage string, args []string) (write bool, err error) {
	if len(args) != 1 {
		return false, fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	switch args[0] {
	case "--check":
		return false, nil
	case "--write":
		return true, nil
	default:
		return false, fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", args[0], usage)
	}
}

// runLinearMapMigrate serves the self-registered "linear map migrate"
// route: "--check" reports drift read-only (catalog.CheckMigration),
// "--write" atomically replaces .planning/linear-map.json with the
// canonical schema-2 migration (catalog.WriteMigration). Neither branch
// invents a remote identity; any entity without an already-recorded
// mapping receives a fresh pending/null one (CONTEXT D-11/D-14).
func runLinearMapMigrate(request Request) Result {
	write, err := parseMigrateArgs("linear map migrate --check|--write", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	if write {
		if err := catalog.WriteMigration(request.Root); err != nil {
			return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
		}
		return Result{Stdout: []byte("linear map migrate --write: .planning/linear-map.json now matches the canonical schema-2 migration.\n")}
	}
	if err := catalog.CheckMigration(request.Root); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte("linear map migrate --check: no drift; .planning/linear-map.json matches the canonical schema-2 migration.\n")}
}

// linearStatusEntry is the allowlisted JSON projection of one catalog
// entity's mapping status: durable local id, kind, and repository-relative
// source are always present; the Linear fields are populated only for a
// mapped (non-project) entity, and Identifier/URL stay empty while a
// mapping is pending (T-01-26: information disclosure; CONTEXT D-11 treats
// pending/null linkage as valid, never a local failure).
type linearStatusEntry struct {
	LocalID    string `json:"local_id"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	LinearType string `json:"linear_type,omitempty"`
	Status     string `json:"status,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	URL        string `json:"url,omitempty"`
}

// linearStatusView is the deterministic JSON envelope "linear status
// --offline" emits: per-status counts plus every catalog entity's
// allowlisted mapping status, in the catalog's deterministic build order.
type linearStatusView struct {
	Status   string              `json:"status"`
	Counts   map[string]int      `json:"counts"`
	Entities []linearStatusEntry `json:"entities"`
}

// linearStatusCounts tallies remote mappings per status value (for
// example "pending" vs "linked"), giving a safe at-a-glance summary
// without exposing any per-entity identity.
func linearStatusCounts(migrated *catalog.Map) map[string]int {
	counts := map[string]int{}
	for _, mapping := range migrated.RemoteMappings {
		counts[mapping.Status]++
	}
	return counts
}

// linearStatusEntries projects the migrated map's complete entity set,
// joined against its remote mappings, into the allowlisted view. A
// pending mapping's nullable Identifier/URL are omitted rather than
// rendered as null, never invented (CONTEXT D-11).
func linearStatusEntries(migrated *catalog.Map) []linearStatusEntry {
	mappingByLocalID := make(map[string]catalog.RemoteMapping, len(migrated.RemoteMappings))
	for _, mapping := range migrated.RemoteMappings {
		mappingByLocalID[mapping.RepoID] = mapping
	}
	entries := make([]linearStatusEntry, 0, len(migrated.Entities))
	for _, entity := range migrated.Entities {
		entry := linearStatusEntry{
			LocalID: entity.LocalID,
			Kind:    entity.Kind,
			Source:  entity.Source,
		}
		if mapping, mapped := mappingByLocalID[entity.LocalID]; mapped {
			entry.LinearType = mapping.LinearType
			entry.Status = mapping.Status
			if mapping.Identifier != nil {
				entry.Identifier = *mapping.Identifier
			}
			if mapping.URL != nil {
				entry.URL = *mapping.URL
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

// runLinearStatus serves the self-registered "linear status" route.
// catalog.MigrateV1ToV2 derives the canonical in-memory schema-2 map
// (preserving every already-recorded remote mapping and creating a fresh
// pending/null one for anything unmapped) without writing anything;
// pending/null linkage is reported as valid status, never a failure.
func runLinearStatus(request Request) Result {
	if err := parseOfflineArgs("linear status --offline", request.Args); err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}
	migrated, err := catalog.MigrateV1ToV2(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	view := linearStatusView{
		Status:   "ok",
		Counts:   linearStatusCounts(migrated),
		Entities: linearStatusEntries(migrated),
	}
	payload, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_STATUS_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}

// parseApplyArgs accepts exactly the supported apply form: a plan file
// path (the first argument, never a flag) followed by --plan-id <id> /
// --plan-id=<id>.
func parseApplyArgs(usage string, args []string) (planFile, planID string, err error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	planFile = args[0]
	if strings.HasPrefix(planFile, "--") {
		return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	for i := 1; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--plan-id":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: --plan-id requires a value; usage: %s", usage)
			}
			planID = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--plan-id="):
			planID = strings.TrimPrefix(argument, "--plan-id=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if planFile == "" || planID == "" {
		return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return planFile, planID, nil
}

// validateOperationsSorted rejects a plan whose operations are not already
// in the exact canonical D-17 hierarchy/local-id order. This is an
// independent check from ValidatePlanIntegrity's hash self-consistency:
// a tampered plan that reordered its operations and then recomputed its
// own plan_id over that new order would otherwise still pass integrity
// alone, so reordering is caught here explicitly before any typed apply.
func validateOperationsSorted(plan reconcile.Plan) error {
	sorted := append([]reconcile.Operation(nil), plan.Operations...)
	if err := reconcile.SortOperations(sorted); err != nil {
		return fmt.Errorf("GOLC_LINEAR_APPLY_PLAN_INVALID: %v", err)
	}
	for index := range sorted {
		if sorted[index].LocalID != plan.Operations[index].LocalID {
			return fmt.Errorf(
				"GOLC_LINEAR_APPLY_PLAN_UNSORTED: plan operations are not in canonical D-17 order at position %d (expected %s, found %s); re-run linear preview",
				index, sorted[index].LocalID, plan.Operations[index].LocalID)
		}
	}
	return nil
}

// validateConflictsWellFormed rejects any D-13 conflict entry that is not
// structurally complete: every conflict must name the local id and field
// in disagreement, carry the resolution command a contributor runs, and
// record all three sides of the three-way comparison it is reporting.
func validateConflictsWellFormed(plan reconcile.Plan) error {
	for _, conflict := range plan.Conflicts {
		if conflict.LocalID == "" || conflict.Field == "" || conflict.ResolutionCommand == "" {
			return fmt.Errorf(
				"GOLC_LINEAR_APPLY_PLAN_CONFLICT_INVALID: conflict for %q field %q is missing required local id, field, or resolution command text",
				conflict.LocalID, conflict.Field)
		}
		if conflict.BaseValue == nil || conflict.RepositoryValue == nil || conflict.LinearValue == nil {
			return fmt.Errorf(
				"GOLC_LINEAR_APPLY_PLAN_CONFLICT_INVALID: conflict for %q field %q must carry base, repository, and linear values",
				conflict.LocalID, conflict.Field)
		}
	}
	return nil
}

// validateNoConflictedOperations rejects a plan where the same local id
// appears both as a planned operation and as an unresolved D-13 conflict:
// those two states are mutually exclusive by construction (canonical.go's
// BuildPlan never emits both for the same local id), so their joint
// presence in a loaded plan file is an illegal state transition rather
// than a plan reconcile itself would ever produce.
func validateNoConflictedOperations(plan reconcile.Plan) error {
	conflicted := make(map[string]bool, len(plan.Conflicts))
	for _, conflict := range plan.Conflicts {
		conflicted[conflict.LocalID] = true
	}
	for _, op := range plan.Operations {
		if conflicted[op.LocalID] {
			return fmt.Errorf(
				"GOLC_LINEAR_APPLY_PLAN_ILLEGAL_TRANSITION: %s has both a planned operation and an unresolved D-13 conflict",
				op.LocalID)
		}
	}
	return nil
}

// decodeAndValidatePlanStrict reads path and strictly decodes it into a
// reconcile.Plan before anything typed is ever handed to apply.Apply
// (CONTEXT D-17/D-18): duplicate object member names and unknown JSON
// fields are rejected by strictjson.DecodeStrict; the plan's own recorded
// plan_id must match its independently recomputed canonical hash
// (apply.ValidatePlanIntegrity); its operations must already be in the
// exact canonical D-17 order; every recorded conflict must be structurally
// well-formed; and no local id may simultaneously own both a planned
// operation and an unresolved conflict. Any violation returns before a
// single typed field is used.
func decodeAndValidatePlanStrict(path string) (reconcile.Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reconcile.Plan{}, fmt.Errorf("GOLC_LINEAR_APPLY_PLAN_READ: %s: %v", path, err)
	}
	var plan reconcile.Plan
	if err := strictjson.DecodeStrict(data, &plan); err != nil {
		return reconcile.Plan{}, fmt.Errorf("GOLC_LINEAR_APPLY_PLAN_DECODE: %s: %v", path, err)
	}
	if err := apply.ValidatePlanIntegrity(plan); err != nil {
		return reconcile.Plan{}, err
	}
	if err := validateOperationsSorted(plan); err != nil {
		return reconcile.Plan{}, err
	}
	if err := validateConflictsWellFormed(plan); err != nil {
		return reconcile.Plan{}, err
	}
	if err := validateNoConflictedOperations(plan); err != nil {
		return reconcile.Plan{}, err
	}
	return plan, nil
}

// achievedApplyPrefix returns the leading contiguous run of
// completed/noop results, mirroring internal/trace/apply/engine.go's own
// unexported achievedPrefix exactly (that helper is not exported; this is
// the same three-line rule, not a reimplementation of any different
// policy) -- the exact achieved prefix safe to journal and to fold back
// into the committed remote mapping map.
func achievedApplyPrefix(results []apply.OperationResult) []apply.OperationResult {
	prefix := make([]apply.OperationResult, 0, len(results))
	for _, result := range results {
		if result.Status != apply.StatusCompleted && result.Status != apply.StatusNoop {
			break
		}
		prefix = append(prefix, result)
	}
	return prefix
}

// applyMapPath is the one fixed destination "linear apply" ever commits
// updated remote mappings to: the same .planning/linear-map.json every
// other Linear route already reads through catalog.MigrateV1ToV2.
func applyMapPath(root string) string {
	return filepath.Join(root, ".planning", "linear-map.json")
}

// mergeApplyResultsIntoMap folds every completed/noop result's discovered
// LinearUUID back into a copy of migrated's remote mappings (CONTEXT
// D-11/D-14: no identifier or URL is ever invented -- this transport's
// wire protocol supplies only a UUID, so Identifier/URL stay exactly as
// migrated already recorded them). It never mutates migrated itself.
func mergeApplyResultsIntoMap(migrated *catalog.Map, results []apply.OperationResult) *catalog.Map {
	updated := *migrated
	updated.RemoteMappings = append([]catalog.RemoteMapping(nil), migrated.RemoteMappings...)

	resultByID := make(map[string]apply.OperationResult, len(results))
	for _, result := range results {
		if (result.Status == apply.StatusCompleted || result.Status == apply.StatusNoop) && result.LinearUUID != nil {
			resultByID[result.LocalID] = result
		}
	}
	for i, mapping := range updated.RemoteMappings {
		result, achieved := resultByID[mapping.RepoID]
		if !achieved {
			continue
		}
		uuid := *result.LinearUUID
		updated.RemoteMappings[i].LinearUUID = &uuid
		updated.RemoteMappings[i].Status = "linked"
	}
	return &updated
}

// commitApplyResults persists the exact D-21 achieved prefix (journal),
// the folded-forward remote mapping map, and the human-reviewable report
// as one atomic result (apply.CommitResultAtomically), so a later "linear
// preview --remote"/"linear apply" against the same repository observes
// every object this run actually created or confirmed as already linked
// -- the exact mechanism that makes replay a safe no-op (CONTEXT D-17/D-21;
// this plan's must_haves truth: "replay is all no-op"). Nothing is
// written when zero operations achieved a clean outcome, so a wholly
// failed first attempt never fabricates progress.
func commitApplyResults(root, planFile string, migrated *catalog.Map, results []apply.OperationResult, report apply.Report) error {
	prefix := achievedApplyPrefix(results)
	if len(prefix) == 0 {
		return nil
	}
	updatedMap := mergeApplyResultsIntoMap(migrated, results)
	journal := apply.Journal{PlanID: report.PlanID, Results: prefix}
	journalPath := planFile + ".journal.json"
	reportPath := planFile + ".report.json"
	if err := apply.CommitResultAtomically(applyMapPath(root), updatedMap, journalPath, journal, reportPath, report); err != nil {
		return fmt.Errorf("GOLC_LINEAR_APPLY_COMMIT_FAILED: %v", err)
	}
	return nil
}

// runLinearApply serves the self-registered "linear apply" route (CONTEXT
// D-17/D-18/D-21): it strictly decodes and validates the plan file, then
// requires --plan-id to exactly match the loaded plan's own plan_id before
// anything is attempted (CONTEXT D-17: an exact-plan bound apply never
// runs against an implicitly selected or approximately matching plan).
// With no RemoteClientFactory wired, apply fails
// GOLC_LINEAR_TRANSPORT_UNAVAILABLE before any credential, subprocess, or
// mutation access. Production wiring (applyRemoteClientFactory =
// newProcessRemoteClient, Plan 01-15) commits every achieved result back
// into .planning/linear-map.json before returning, so a subsequent
// preview/apply against the same repository observes it.
func runLinearApply(request Request) Result {
	usage := "linear apply {plan-file} --plan-id <id>"
	planFile, planID, err := parseApplyArgs(usage, request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	resolvedPlanFile := resolveWritablePath(request.Root, planFile)
	plan, err := decodeAndValidatePlanStrict(resolvedPlanFile)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	if plan.PlanID != planID {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf(
			"GOLC_LINEAR_APPLY_PLAN_ID_MISMATCH: --plan-id %q does not match the loaded plan's own plan_id %q\n", planID, plan.PlanID))}
	}

	if applyRemoteClientFactory == nil {
		return Result{ExitCode: 1, Stderr: []byte(
			"GOLC_LINEAR_TRANSPORT_UNAVAILABLE: no RemoteClientFactory is wired; the process-based Linear transport does not exist yet\n")}
	}

	if err := apply.GuardAgainstPullRequestMutation(os.LookupEnv); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	client, err := applyRemoteClientFactory(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_TRANSPORT_UNAVAILABLE: %v\n", err))}
	}
	if closer, ok := client.(io.Closer); ok {
		defer closer.Close()
	}

	migrated, err := catalog.MigrateV1ToV2(request.Root)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	results := apply.Apply(client, plan, migrated.RemoteMappings)
	report := apply.Report{PlanID: plan.PlanID, Results: results}

	if err := commitApplyResults(request.Root, resolvedPlanFile, migrated, results, report); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, err := strictjson.CanonicalEncode(report)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_APPLY_REPORT_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: payload}
}
