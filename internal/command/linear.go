// linear.go is the linear command file: it owns the "linear" routing scope
// and self-registers the offline catalog inspection route (CONTEXT D-03,
// D-11, D-14). It reads only committed repository planning artifacts
// through internal/trace/catalog; no network, Node, SDK, or Linear
// credential access is reachable from this route.
package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lnorton89/golc/internal/strictjson"
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
	Route:   "linear preview",
	Summary: "Preview a complete-snapshot reconciliation plan against a fake transport fixture: linear preview --snapshot <path> --out <path>.",
	Handler: runLinearPreview,
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

// parseSnapshotOutArgs accepts exactly the supported preview form:
// --snapshot <path> --out <path>.
func parseSnapshotOutArgs(usage string, args []string) (snapshotPath, outPath string, err error) {
	for i := 0; i < len(args); {
		argument := args[i]
		switch {
		case argument == "--snapshot":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: --snapshot requires a path; usage: %s", usage)
			}
			snapshotPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--snapshot="):
			snapshotPath = strings.TrimPrefix(argument, "--snapshot=")
			i++
		case argument == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: --out requires a path; usage: %s", usage)
			}
			outPath = args[i+1]
			i += 2
		case strings.HasPrefix(argument, "--out="):
			outPath = strings.TrimPrefix(argument, "--out=")
			i++
		default:
			return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: unsupported argument %q; usage: %s", argument, usage)
		}
	}
	if snapshotPath == "" || outPath == "" {
		return "", "", fmt.Errorf("GOLC_LINEAR_USAGE: usage: %s", usage)
	}
	return snapshotPath, outPath, nil
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

// runLinearPreview serves the self-registered "linear preview" route: it
// captures a snapshot from a credential-free fake transport fixture,
// builds the exact D-17 complete-snapshot preview against the
// repository's own catalog intent and remote mapping set, and writes the
// canonical preview JSON to --out. No network, SDK, or Linear credential
// access is reachable from this route (T-01-SC).
func runLinearPreview(request Request) Result {
	snapshotPath, outPath, err := parseSnapshotOutArgs("linear preview --snapshot <path> --out <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	fake, err := transport.LoadFakeSnapshot(resolveWritablePath(request.Root, snapshotPath))
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

	payload, err := strictjson.CanonicalEncode(plan)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_ENCODE_FAILED: %v\n", err))}
	}
	destination := resolveWritablePath(request.Root, outPath)
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW_WRITE_FAILED: %v\n", err))}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_LINEAR_PREVIEW: wrote %s\n", destination))}
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
