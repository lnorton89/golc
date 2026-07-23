# Phase 5: Durable Shows and Recovery - Pattern Map

**Mapped:** 2026-07-23
**Files analyzed:** 13 (new/modified, per RESEARCH.md's Recommended Project Structure + CONTEXT.md's new CLI routes)
**Analogs found:** 13 / 13 (all matched against existing `internal/show` and `internal/command` code; RESEARCH.md's own Code Examples supplement where no codebase analog exists for genuinely new mechanics like SQLite transactions)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/show/store.go` (`Load`/`Save`, same signatures) | service (storage) | CRUD | `internal/show/state.go` (`Load`/`Save`) | exact — same package, same function signatures, replacing only the I/O internals per D-02 |
| `internal/show/schema.go` (CREATE TABLE + PRAGMA setup) | config | file-I/O | `internal/show/state.go` (`resolvePath`, `Load`'s not-yet-existing-file handling) | role-match — no SQL-schema analog exists yet in this Go-only-JSON codebase; RESEARCH.md Pattern 1 is the concrete source |
| `internal/show/migrate.go` (schema_version registry) | service | transform/batch | `internal/show/state.go` (`validate`, `Save`'s stamp-then-encode sequence) | partial — no migration-registry precedent exists; RESEARCH.md Pitfall 2 + Pattern 3/4 supply the shape |
| `internal/show/backup.go` (`VACUUM INTO` + read-back-validate) | service | file-I/O | `internal/show/state.go` (`Load`'s decode-then-validate-before-trust sequence) | role-match — same "never trust disk content" doctrine, new file-copy mechanic |
| `internal/show/recovery.go` (insert/prune/detect/offer) | service | CRUD | `internal/show/state.go` (`Save`'s validate→stamp→encode→write sequence) | role-match — reuses Save's exact trigger point (D-04), new table target |
| `internal/show/diagnose.go` (`PRAGMA integrity_check` + `validate()`) | service | request-response | `internal/show/state.go` (`validate`) | role-match — wraps the existing whole-State `validate()` plus a new file-level check |
| `internal/show/store_test.go` | test | CRUD | `internal/show/state_test.go` | exact — same package, round-trip test shape |
| `internal/show/migrate_test.go` | test | batch | `internal/show/state_test.go` | role-match |
| `internal/show/recovery_test.go` | test | CRUD | `internal/show/state_test.go` | role-match |
| `internal/show/backup_test.go` | test | file-I/O | `internal/show/state_test.go` | role-match |
| `internal/show/diagnose_test.go` | test | request-response | `internal/show/state_test.go` | role-match |
| `internal/command/{show open/save/save-as}` handlers (likely a new `internal/command/show.go`, since `deployment.go` currently owns the `show` scope but only registers `show inspect`) | controller (CLI route) | request-response | `internal/command/deployment.go` (`runDeploymentCreate`/`runShowInspect`, `show` scope registration) | exact — same `Load → mutate/none → Save` call-site shape D-04/CONTEXT Reusable Assets explicitly names as the plug-in point |
| `internal/command/{show diagnose, show export}` handlers | controller (CLI route) | request-response | `internal/command/deployment.go` (`runShowInspect` — read-only, load + JSON-encode + print) | exact — diagnose/export are both read-only `Load` + structured-print routes, same shape as `show inspect` |
| `internal/command/{show apply-migration confirm flow}` handler | controller (CLI route) | request-response | `internal/command/pool.go` (`runPoolUpdate`/`runPoolApply` plan/apply preview-then-confirm split) | role-match — D-08's "prompt, then on confirm backup→migrate→swap" mirrors pool's dry-run-plan-then-explicit-apply UX precedent named in CONTEXT.md's Established Patterns |

## Pattern Assignments

### `internal/show/store.go` (service, CRUD)

**Analog:** `internal/show/state.go` (existing `Load`/`Save`)

**Imports pattern** (state.go lines 16-27):
```go
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lnorton89/golc/internal/deployment"
	"github.com/lnorton89/golc/internal/pool"
	"github.com/lnorton89/golc/internal/programming"
	"github.com/lnorton89/golc/internal/scene"
	"github.com/lnorton89/golc/internal/strictjson"
)
```
`store.go` keeps this import block and adds `database/sql`, `modernc.org/sqlite` (blank `_` driver import), `time`, and `crypto/sha256`/`encoding/hex` for the checksum (per RESEARCH.md Pattern 2).

**"Nothing from disk is trusted" pattern** (state.go lines 87-105, `Load`):
```go
func Load(root, path string) (State, error) {
	resolved := resolvePath(root, path)
	data, err := os.ReadFile(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return State{SchemaVersion: SchemaVersion}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: reading %s: %v", resolved, err)
	}

	var state State
	if err := strictjson.DecodeStrict(data, &state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	if err := validate(state); err != nil {
		return State{}, fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	return state, nil
}
```
`store.go`'s new `Load` keeps this exact three-step shape (read → `strictjson.DecodeStrict` → `validate`) — only the "read" step becomes a SQLite `SELECT blob FROM show_state` (see RESEARCH.md's "Open (Load)" Code Example) instead of `os.ReadFile`. The not-yet-existing-file → fresh-`State` short-circuit must be preserved (SQLite equivalent: `sql.ErrNoRows` on the `show_meta` query, per RESEARCH.md's example).

**Validate-stamp-encode-write pattern** (state.go lines 114-139, `Save`):
```go
func Save(root, path string, s State) error {
	if err := validate(s); err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	s.SchemaVersion = SchemaVersion
	s.Revision++

	payload, err := strictjson.CanonicalEncode(s)
	if err != nil {
		return fmt.Errorf("GOLC_SHOW_STATE_INVALID: %v", err)
	}
	// ... write-temp-then-rename ...
}
```
`store.go`'s new `Save` keeps validate → stamp `SchemaVersion` → bump `Revision` → `strictjson.CanonicalEncode` identical; only the persistence tail (write-temp-then-rename) becomes the SQLite transaction shown in RESEARCH.md Pattern 2 (`BEGIN` → `UPDATE show_meta` → `UPDATE show_state` → `INSERT INTO recovery_points` → prune → `COMMIT`).

**`resolvePath` helper** (state.go lines 65-76) — reuse verbatim, unchanged; `backup.go`/`migrate.go` also need it for backup/temp-copy paths (RESEARCH.md's Security Domain explicitly calls out reusing this single path-resolution rule rather than inventing a second one).

---

### `internal/show/schema.go` (config, file-I/O)

**Analog:** `internal/show/state.go`'s `resolvePath` + `Load`'s not-yet-existing-file branch (no direct SQL-schema analog exists in this codebase — this is the first SQLite usage in the project per RESEARCH.md's Integration Points).

**Source of truth for the actual content:** RESEARCH.md Pattern 1 (`CREATE TABLE IF NOT EXISTS show_meta/show_state/recovery_points`, `PRAGMA application_id/journal_mode=WAL/synchronous=FULL/foreign_keys=ON`) — copy verbatim, this is a locked-shape (D-03) schema, not a judgment call.

**Convention to carry over:** `state.go`'s doc-comment style — every new file in `internal/show` opens with a package-doc-style comment naming which CONTEXT decisions/threats it implements (see state.go lines 1-13). `schema.go`'s doc comment should cite D-03 and Pitfall 5 (application_id stamp).

---

### `internal/show/migrate.go` (service, transform/batch)

**Analog:** `internal/show/state.go`'s `validate` (single entry point every new invariant plugs into) — `migrate.go`'s registry is the same "one dispatch point per schema_version" shape, not a literal code copy.

**Core pattern:** RESEARCH.md's Pattern 3 (verified backup) + Pattern 4 (atomic swap) + Pitfall 2 (`map[int]func([]byte) ([]byte, error)` registry, not `goose`). Copy the `verifiedBackup`/`atomicReplace` function skeletons from RESEARCH.md verbatim; bounds-check `schema_version` against `[1, SchemaVersion]` before indexing the registry (RESEARCH.md Security Domain — Tampering pattern row 2) before ever calling into it.

**Error-code convention:** Follow state.go's `GOLC_SHOW_STATE_INVALID` prefix convention — new codes: `GOLC_SHOW_BACKUP_FAILED`, `GOLC_SHOW_BACKUP_UNVERIFIABLE`, `GOLC_SHOW_MIGRATE_SWAP_FAILED`, `GOLC_SHOW_SCHEMA_TOO_NEW` (all `{DOMAIN}_{CONDITION}` shaped, matching CONTEXT.md's Established Patterns note).

---

### `internal/show/backup.go` (service, file-I/O)

**Analog:** `internal/show/state.go`'s `Load` (decode-then-validate-before-trust sequence, lines 97-104) — `backup.go`'s `verifiedBackup` is structurally the same "never trust a copy operation without re-decoding + re-validating," just applied to a `VACUUM INTO` output file instead of the primary path. Use RESEARCH.md's `verifiedBackup` code example verbatim as the implementation skeleton (fresh connection, `strictjson.DecodeStrict`, `validate()` — not a checksum shortcut, per D-09).

---

### `internal/show/recovery.go` (service, CRUD)

**Analog:** `internal/show/state.go`'s `Save` (lines 114-139) — the recovery-point INSERT/prune is D-04's requirement to live "inside the same SQLite transaction as the save," so this file's core logic is embedded directly in `store.go`'s `Save` transaction (RESEARCH.md Pattern 2 lines showing `INSERT INTO recovery_points` + `DELETE ... WHERE id NOT IN (... LIMIT 5)`), while `recovery.go` itself holds the read-side query/offer/discard functions (`DetectRecoveryPoints`, `DiscardRecoveryPoint` per D-07's explicit-`DELETE`-on-discard requirement, RESEARCH.md Security Domain row 5).

---

### `internal/show/diagnose.go` (service, request-response)

**Analog:** `internal/show/state.go`'s `validate` function (the existing structural-check entry point this file wraps, not duplicates).

**Core pattern** — copy RESEARCH.md's "Diagnose combining structural + file-level checks" example verbatim:
```go
func Diagnose(root, path string) (DiagnosticReport, error) {
    db, err := openStore(root, path)
    ...
    rows, err := db.Query(`PRAGMA integrity_check`)
    ...
    state, structuralErr := Load(root, path) // reuses the same validate() every Load runs
    return DiagnosticReport{...}, nil
}
```

---

### `internal/command/show.go` (new file; controller, request-response) — `show open`/`show save`/`show save-as`

**Analog:** `internal/command/deployment.go`'s `runDeploymentCreate` (lines 52-73) and the existing `show` scope registration (lines 35-44) it already owns for `show inspect`.

**Scope/route registration pattern** (deployment.go lines 35-44):
```go
var _ = MustDeclareScope(ScopeRegistration{
	Scope:   "show",
	Summary: "Inspection of a working ShowState document's logical pools and deployments.",
})

var _ = MustDeclareRoute(CommandRegistration{
	Route:   "show inspect",
	Summary: "Print a deterministic JSON summary of a ShowState document's pools and deployments: show inspect --show <path>.",
	Handler: runShowInspect,
})
```
New `show open`/`show save`/`show save-as` routes register the same way, extending the already-declared `show` scope (do not re-declare the scope — `MustDeclareScope` for `"show"` already exists in `deployment.go`; check for duplicate-registration panics if consolidating).

**Load→mutate→Save handler pattern** (deployment.go lines 52-73, `runDeploymentCreate`):
```go
func runDeploymentCreate(request Request) Result {
	name, showPath, err := parseDeploymentNameShowArgs(...)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	// ...mutate state...

	if err := show.Save(request.Root, showPath, state); err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}
	return Result{Stdout: []byte(fmt.Sprintf("GOLC_DEPLOYMENT_CREATED: %s (%s)\n", newDeployment.Name, newDeployment.ID))}
}
```
`show open` is the read-only variant (no `Save` call — matches `runShowInspect`'s shape below, plus D-07's recovery-offer check after `Load`). `show save`/`show save-as` call `show.Save` (save-as taking an explicit destination path argument, reusing `resolveWritablePath`'s convention already established in `internal/command/linear.go`/`pool.go`).

**Exit-code convention:** `ExitCode: 2` for usage/argument-parsing errors, `ExitCode: 1` for runtime/validation errors — consistent across every command file, keep it identical in the new `show` routes.

---

### `internal/command/show.go` — `show diagnose` / `show export`

**Analog:** `internal/command/deployment.go`'s `runShowInspect` (lines 227-246) — the exact read-only `Load` + structured-encode + print shape:
```go
func runShowInspect(request Request) Result {
	showPath, err := parseShowInspectArgs("show inspect --show <path>", request.Args)
	if err != nil {
		return Result{ExitCode: 2, Stderr: []byte(err.Error() + "\n")}
	}

	state, err := show.Load(request.Root, showPath)
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(err.Error() + "\n")}
	}

	payload, err := json.MarshalIndent(buildShowInspectView(state), "", "  ")
	if err != nil {
		return Result{ExitCode: 1, Stderr: []byte(fmt.Sprintf("GOLC_DEPLOYMENT_INSPECT_ENCODE_FAILED: %v\n", err))}
	}
	return Result{Stdout: append(payload, '\n')}
}
```
`show diagnose` calls `show.Diagnose` instead of `show.Load` and prints the `DiagnosticReport`. `show export` calls `show.Load` then `strictjson.CanonicalEncode(state)` directly (D-13: byte-identical to today's whole-file format) rather than `buildShowInspectView`'s allowlisted projection — export must NOT go through the inspect view's field-filtering, since D-13 requires the full canonical document.

**D-10 newer-format read-only routing:** both `show diagnose` and `show export` (and `show inspect`) must tolerate `show.ErrSchemaTooNew` from `Load`/`Diagnose` and still proceed (decode+validate for read-only use), while `show save`/`show open`-for-editing/`show apply-migration` must reject it with `GOLC_SHOW_SCHEMA_TOO_NEW` and exit 1 — this fork doesn't exist yet in `runShowInspect`'s current form and is new branching logic this phase introduces.

---

### `internal/command/show.go` — migration confirm flow (`show apply-migration` or equivalent, D-08)

**Analog:** `internal/command/pool.go`'s `runPoolUpdate`/`runPoolApply` split (lines 343-486) — the plan-then-explicit-apply UX CONTEXT.md's Established Patterns section explicitly names as the precedent D-08's "prompt, then on confirm" flow should reuse.

**Preview/apply separation pattern** (pool.go lines 343-390, `runPoolUpdate` building a plan without ever touching disk — "no code path here can ever write the ShowState file"):
```go
// This is the dry-run half of the D-15 plan/apply split: no code path
// here can ever write the ShowState file (CONTEXT T-02-12).
func runPoolUpdate(request Request) Result {
	...
	plan, err := pool.BuildImpactPlan(state.Pools, state.Deployments, state.Groups, state.Revision, req)
	...
	if parsed.outPath != "" {
		return writeImpactPlan(request.Root, parsed.outPath, plan)
	}
	...
}
```
And the apply half re-validating freshness before mutating (pool.go lines 442-486, `runPoolApply`): decode plan → `ValidatePlanIntegrity` → `plan.PlanID` match check → `Load` fresh state → `ValidatePlanFreshness` → `Apply` → `Save`. The migration flow mirrors this exactly: detect (`show_meta.schema_version < current`) → prompt/confirm (equivalent to requiring an explicit `--confirm`/`--yes` flag, since this is a scripted CLI with no interactive prompt loop elsewhere in the codebase) → `verifiedBackup` → migrate-in-temp-copy → re-validate → `atomicReplace`. Reuse the `--plan-id`-style "explicit second confirmation token" idea only if the planner wants extra safety; otherwise a simple `--confirm` flag matches this codebase's existing non-interactive CLI convention (no other command in `internal/command/*.go` blocks on stdin).

---

## Shared Patterns

### Error-code convention: `{DOMAIN}_{CONDITION}`
**Source:** `internal/show/state.go` (`GOLC_SHOW_STATE_INVALID`), `internal/command/deployment.go` (`GOLC_DEPLOYMENT_CREATED`, `GOLC_DEPLOYMENT_ACTIVATED`), `internal/command/pool.go` (`GOLC_POOL_APPLY_USAGE`, `GOLC_POOL_NOT_FOUND`, `GOLC_POOL_APPLY_PLAN_ID_MISMATCH`)
**Apply to:** every new error path in `store.go`, `migrate.go`, `backup.go`, `recovery.go`, `diagnose.go`, and every new `show` CLI route. New codes this phase introduces: `GOLC_SHOW_BACKUP_FAILED`, `GOLC_SHOW_BACKUP_UNVERIFIABLE`, `GOLC_SHOW_MIGRATE_SWAP_FAILED`, `GOLC_SHOW_SCHEMA_TOO_NEW`, `GOLC_SHOW_NOT_GOLC_FORMAT`, `GOLC_SHOW_DIAGNOSE_FAILED` (all named in RESEARCH.md's Code Examples/Pitfalls sections — reuse those exact strings).

### "Nothing from disk is trusted before validate() passes" (T-02-10)
**Source:** `internal/show/state.go` `Load` (lines 87-105) — doc comment explicitly names this as CONTEXT threat T-02-10.
**Apply to:** `store.go`'s `Load`, `backup.go`'s `verifiedBackup`, `migrate.go`'s post-migration re-validate step, `diagnose.go`. Every one of these must run `strictjson.DecodeStrict` + `show.validate()` (or the SQLite equivalent read + decode + validate sequence) before trusting content — never a checksum-only shortcut (D-09 explicitly rejects that).

### `Load(root, path) (State, error)` / `Save(root, path, State) error` call-site shape
**Source:** every `internal/command/*.go` handler (`deployment.go`, `pool.go`, `programming.go`, `scene.go`, `artnet.go` per CONTEXT.md's Reusable Assets note).
**Apply to:** no new call-site wiring needed in existing command files — `store.go`'s reimplemented `Load`/`Save` keep identical signatures, so every existing command continues to compile and behave unchanged (D-02's explicit guarantee). Only the new `internal/command/show.go` needs new handler code.

### `resolvePath(root, path)` — root-relative-vs-absolute resolution
**Source:** `internal/show/state.go` lines 65-76.
**Apply to:** `backup.go`'s timestamped backup path, `migrate.go`'s temp-copy path — RESEARCH.md's Security Domain explicitly calls out reusing this single resolution rule rather than inventing a second, inconsistent one (directory-traversal mitigation).

### Preview-then-explicit-confirm CLI UX
**Source:** `internal/command/pool.go` `runPoolUpdate`/`runPoolApply` (lines 343-486).
**Apply to:** the migration confirm flow (D-08) and, more loosely, the recovery-offer flow (D-07) — both must present detection separately from the action that mutates the file, matching this codebase's existing dry-run/apply shape operators already know.

## No Analog Found

None outright — every file has at least a role-match analog in `internal/show/state.go` or `internal/command/deployment.go`/`pool.go`. The genuinely new mechanics (SQLite transactions, `VACUUM INTO`, `PRAGMA integrity_check`, migration-function registry) have no codebase precedent since this is the project's first SQLite usage; for those, RESEARCH.md's own "Code Examples" and "Architecture Patterns" sections (Pattern 1-5) are the concrete source to copy from instead of a codebase analog — flagged inline above wherever used.

## Metadata

**Analog search scope:** `internal/show/` (state.go, state_test.go), `internal/command/` (deployment.go, pool.go, linear.go referenced for `resolveWritablePath`/plan-apply precedent)
**Files scanned:** 2 fully read (`internal/show/state.go`, relevant sections of `internal/command/deployment.go` and `internal/command/pool.go`); directory listing of `internal/command/*.go` (33 files) and `internal/show/*` (2 files) for completeness
**Pattern extraction date:** 2026-07-23
