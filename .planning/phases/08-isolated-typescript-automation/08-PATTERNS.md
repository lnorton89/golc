# Phase 8: Isolated TypeScript Automation - Pattern Map

**Mapped:** 2026-07-25
**Files analyzed:** 9 (new) + 2 (modified)
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/script/router.go` | route/controller | request-response | `internal/api/events.go`'s `RegisterOperation`/self-registered-route tail (`var _ = RegisterOperation(...)`) + `internal/command/router.go`'s `MustDeclareRoute` idiom | role-match |
| `internal/script/host.go` | service (process supervisor) | streaming (persistent bidirectional session) | `internal/trace/transport/process.go` (`ProcessClient`) | exact (structural), needs one-shot→persistent generalization |
| `internal/script/jobobject_windows.go` | utility (OS resource control) | event-driven (kill-on-limit) | `internal/trace/transport/process.go`'s `killProcessTree`/`terminate` (closest existing "hard-kill a child process" precedent); no direct Job Object precedent exists yet | role-match (no exact analog) |
| `internal/script/jobobject_other.go` | utility (no-op fallback) | — | none — new pattern (build-tag-gated stub); mirrors Go's own `_windows.go`/`_other.go` naming convention used nowhere else in repo yet | no analog |
| `internal/script/debugbridge.go` | service (protocol bridge) | event-driven (CDP events → SSE) | `internal/api/events.go` (SSE fan-out/ring-buffer/broadcast pattern) for the outbound half; no existing CDP/WebSocket-client precedent | role-match (outbound half only) |
| `internal/script/capability.go` | middleware (scope + rate + limit enforcement) | request-response | `internal/api/auth.go` (`RequireScope`) + `internal/api/ratelimit.go` (`keyRateLimiter`) | exact |
| `internal/scriptsdk/generate.go` | utility (code generator) | batch (deterministic file generation) | `internal/contracts/generate.go` (`SchemaDescriptor`/`MustRegisterSchema`/`GenerateInto`/`CheckDrift`) | exact |
| `internal/show/scripts.go` | model (show-state entity) | CRUD | `internal/scene/scene.go` (`Scene` type: UUIDv7 identity, unique-name validation, single-active-style invariant) for the Go type shape; `internal/show/apikeys.go` for the "secret-bearing, hashed-at-rest, revocable" persistence idiom (capability profile touches similar concerns) | role-match |
| `internal/show/state.go` (modified) | model (aggregate root) | CRUD | itself — add `Scripts []script.Script` field alongside existing `Pools []pool.Pool` / `Scenes []scene.Scene` fields, call a new `script.ValidateScriptUniqueNames`-style check from `Validate()` the same way `Pools`/`Scenes` already are | exact (extend, don't duplicate) |
| `frontend/src/workspaces/build/ScriptsWorkspace.tsx` | component (workspace/page) | request-response + streaming (SSE subscription) | `frontend/src/workspaces/build/ScenesLooksWorkspace.tsx` (real, wired workspace) — **not** `FixtureLibraryWorkspace.tsx`, which is a `ComingSoon` stub, not a library pattern | role-match (corrected from RESEARCH.md's suggestion) |

## Pattern Assignments

### `internal/script/host.go` (service, streaming)

**Analog:** `internal/trace/transport/process.go`

**Imports pattern** (lines 21-38):
```go
import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lnorton89/golc/internal/security"
)
```

**Config/launch pattern** (lines 69-97, `ProcessConfig`): pinned-executable + fail-closed existence checks before `exec.Command`, explicit (never-inherited) `Env []string`. `internal/script/host.go` must adapt this for Deno: `denoExecutable` + a materialized single-file temp script path, **no `--allow-*` flags ever appended** (per RESEARCH.md Pattern 1/Anti-Patterns), and `--inspect-brk=127.0.0.1:<port>` appended only when launch mode is Debug (RESEARCH.md Pattern 3).

**Bounded/redacted output capture** (lines 99-126, 222-245): `boundedBuffer` (drop-oldest-first byte ring) + `security.Redact` wrapping every captured line before it can reach a caller. Reuse verbatim for script stdout/stderr → SSE log lines (D-04) and for the crash/stopped-reason summary (D-12).

**Process-tree kill** (lines 247-274, `killProcessTree`/`terminate`): Windows `taskkill /T /F` fallback to `cmd.Process.Kill()` on other platforms. `internal/script/jobobject_windows.go` must call this as the fallback beneath the primary Job-Object-close kill path (RESEARCH.md "Don't Hand-Roll": Job Object is a strict superset of this tree-kill).

**One-shot Call → persistent session generalization needed:** `Call` (lines 283-357) is a single strict request/response line pair under one `sync.Mutex`-serialized call. `host.go` must generalize this into a long-lived multiplexed session (cmd-call / cmd-result / log-line / status / cancel frames, each independently addressable) rather than reusing `Call` as-is — call this out explicitly in the plan, this is new code, not a drop-in reuse.

**Error/diagnostic code convention:** `RPCError{Code, Message}` with `GOLC_TRANSPORT_*` codes (lines 40-53, 145-173) — `internal/script` should mint its own `GOLC_SCRIPT_*` family the same shape (e.g. `GOLC_SCRIPT_HOST_MISSING`, `GOLC_SCRIPT_LIMIT_EXCEEDED`), per the `{DOMAIN}_{CONDITION}` convention CONTEXT.md's Established Patterns section names.

---

### `internal/script/capability.go` (middleware, request-response)

**Analog:** `internal/api/auth.go` + `internal/api/ratelimit.go`

**Scope-check pattern** (`internal/api/auth.go` lines 88-99):
```go
func RequireScope(ctx context.Context, scope show.APIKeyScope) error {
	if HasScope(ctx, scope) {
		return nil
	}
	return huma.Error403Forbidden(
		"GOLC_API_SCOPE_REQUIRED: this operation requires the \"" + string(scope) + "\" scope")
}
```
D-06 reuses `show.APIKeyScope` (`playback`/`authoring`/`admin`) directly — `capability.go` should call this exact `RequireScope` function per SDK call arriving over the script host's stdio channel, host-side, never trusting anything the script process claims about its own permissions (RESEARCH.md Pitfall 1). A scope violation should map to the "immediate hard termination, logged" severity (RESEARCH.md Assumption A2), not a soft catchable error.

**Rate limiter pattern** (`internal/api/ratelimit.go` lines 36-89, `keyRateLimiter`):
```go
type keyRateLimiter struct {
	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
	perMinute int
	burst     int
}
func (k *keyRateLimiter) allow(keyID string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	limiter, ok := k.limiters[keyID]
	if !ok {
		limiter = rate.NewLimiter(k.limit(), k.burstOrDefault())
		k.limiters[keyID] = limiter
	}
	return limiter.Allow()
}
```
D-09's per-script rate limit: key the map by running-script-instance-id instead of API key id, one `*rate.Limiter` per active run, otherwise identical mechanism (`golang.org/x/time/rate`, lazy-created, mutex-guarded).

**Hard-fail-closed default sizing** (lines 21-29, 56-73): package-level safe defaults (`defaultRatePerMinute`/`defaultRateBurst`) used whenever a configured value is non-positive — mirror this for D-09's named presets ("Quick action"/"Long-running automation") so an unset/zero custom value never silently becomes "unlimited."

---

### `internal/scriptsdk/generate.go` (utility, batch/transform)

**Analog:** `internal/contracts/generate.go`

**Self-registering descriptor pattern** (lines 33-110, `SchemaDescriptor`/`RegisterSchema`/`MustRegisterSchema`):
```go
type SchemaDescriptor struct {
	Name       string
	OutputPath string
	Schema     func() *jsonschema.Schema
}
func MustRegisterSchema(descriptor SchemaDescriptor) SchemaDescriptor {
	if err := RegisterSchema(descriptor); err != nil {
		panic(err.Error())
	}
	return descriptor
}
```
`internal/scriptsdk` should define a sibling descriptor kind (e.g. `SDKDescriptor{Name, OutputPath, TypeScript func() string}`) reusing the exact same self-registration idiom (`var _ = scriptsdk.MustRegisterSDKEntry(...)` from each command/query/event's own file) so adding a new SDK-exposed capability later needs no central-switch edit — mirrors `internal/command/router.go`'s `MustDeclareRoute` self-registration CONTEXT.md's Established Patterns section calls out directly.

**Byte-stable generation + drift check** (lines 152-227, `GenerateInto`/`GenerateAll`/`CheckDrift`): generate into a disposable temp dir, compare bytes, never write to committed paths from `CheckDrift`. Reuse verbatim for the generated `.d.ts` + runtime-shim files — this is the exact SCRP-02 "byte-stable, drift-checked" test (`TestGenerateDrift`) RESEARCH.md's Validation Architecture table names.

**Generated-marker convention** (line 31, `generatedMarker`): `"GENERATED by github.com/lnorton89/golc/internal/contracts. DO NOT EDIT."` — `internal/scriptsdk` should mint its own equivalent marker naming its own package.

**Diagnostic code convention:** `GOLC_CONTRACTS_*` (e.g. `GOLC_CONTRACTS_NAME_DUPLICATE`, line 82) → `internal/scriptsdk` should mint `GOLC_SCRIPTSDK_*`.

---

### `internal/show/scripts.go` (model, CRUD)

**Analog:** `internal/scene/scene.go` (type shape) + `internal/show/apikeys.go` (secret/profile persistence idiom)

**Identity/type shape pattern** (`internal/scene/scene.go` lines 71-85):
```go
type Scene struct {
	ID                                 uuid.UUID `json:"id"`
	Name                               string    `json:"name"`
	Active                             bool      `json:"active"`
	BarsPerLoop                        int       `json:"bars_per_loop"`
	PreserveMusicalPositionOnBPMChange bool      `json:"preserve_musical_position_on_bpm_change"`
	Layers                             [4]Layer  `json:"layers"`
}
```
D-17's `Script` entity should follow this shape: durable UUIDv7 `ID` minted once at creation (never derived from `Name`, never re-minted on rename — doc comment lines 6-13 make this an explicit repo-wide convention, "Scene copies internal/pool/model.go's identity/construction/rename/unique-name shape verbatim"), plus `Name`, `Source` (the `.ts` text, D-14 single-file), `CapabilityProfile` (D-07: scope + deadline + rate + resource-limit fields, using `show.APIKeyScope` for the scope field per D-06), and `LastRunStatus` (D-16's library-row summary field).

**Unique-name validation convention:** `scene.go`'s sibling `ValidateSceneUniqueNames`/`ValidateSingleActiveScene` — `internal/show/scripts.go` needs an equivalent `ValidateScriptUniqueNames`, called from `state.go`'s `Validate()` the same way `scene.ValidateSceneUniqueNames(s.Scenes)` already is (see `state.go` line 173).

**State aggregation pattern** (`internal/show/state.go` lines 38-49):
```go
type State struct {
	...
	Pools            []pool.Pool                  `json:"pools"`
	...
	Scenes           []scene.Scene                `json:"scenes"`
	...
}
```
Add `Scripts []script.Script \`json:"scripts"\`` as a new field on the existing `State` struct — **do not** create a new SQLite table. `internal/show`'s persistence model is a single versioned BLOB (`show_state` table, `schema.go` lines 55-58: `blob BLOB NOT NULL`), not one table per entity — `api_keys`/`audit_log`/`recovery_points` are the only genuine separate tables, each for its own structural reason (secret hashing, audit immutability, point-in-time snapshots). Scripts belong in the blob alongside pools/scenes/groups so they get autosave/recovery/migration/export for free (D-17), exactly as CONTEXT.md's Integration Points section states.

**Capability-profile-as-persisted-secret-adjacent-data idiom** (`internal/show/apikeys.go` lines 38-56, `APIKeyScope` closed set + `ValidateAPIKeyScopes`):
```go
type APIKeyScope string
const (
	APIKeyScopePlayback  APIKeyScope = "playback"
	APIKeyScopeAuthoring APIKeyScope = "authoring"
	APIKeyScopeAdmin     APIKeyScope = "admin"
)
```
D-06 requires `Script.CapabilityProfile.Scope` to reuse this exact type/closed-set — import `show.APIKeyScope` directly rather than redeclaring a parallel enum.

---

### `frontend/src/workspaces/build/ScriptsWorkspace.tsx` (component, request-response + streaming)

**Analog:** `frontend/src/workspaces/build/ScenesLooksWorkspace.tsx` — **correction to RESEARCH.md**: `FixtureLibraryWorkspace.tsx` is a bare `ComingSoon` placeholder (4-line stub, no data flow at all), not a real library-workspace pattern. `ScenesLooksWorkspace.tsx` is the actual wired, tested (`ScenesLooksWorkspace.test.tsx` exists) reference implementation and is the correct structural template.

**Imports pattern** (lines 9-38):
```tsx
import { useCallback, useEffect, useState } from "react";
import {
  activateScene, assertOk, ..., errorMessage, listProgramming,
  offlineProgrammingView, ..., type ProgrammingView,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import SceneList from "../../components/SceneProgramming/SceneList";
...
import { useInspectorSlot } from "../../shell/InspectorSlot";
import styles from "./ScenesLooksWorkspace.module.css";
```
`ScriptsWorkspace.tsx` should follow the identical shape: a typed `wailsBridge`-style client module for script CRUD/run/stop/debug calls, an `offline*View()` fallback, primitive imports (`Toolbar`, `ScrollRegion`), a CSS Module, and `useInspectorSlot` for the contextual profile/debug-panel inspector (mirrors `LookBrowser`'s inspector-portal usage, lines 195-205).

**Load/refresh/error pattern** (lines 58-79):
```tsx
const [view, setView] = useState<ProgrammingView>(offlineProgrammingView());
const [loading, setLoading] = useState(true);
const [error, setError] = useState<string | null>(null);
const refresh = useCallback(async (): Promise<void> => {
  try {
    const next = await listProgramming();
    setView(next);
    setError(null);
  } catch (err) {
    setError(errorMessage(err));
  } finally {
    setLoading(false);
  }
}, []);
useEffect(() => { void refresh(); }, [refresh]);
```
Reuse verbatim for the script library list load (D-16).

**Mutation-handler pattern** (lines 92-193, e.g. `handleCreateScene`):
```tsx
const handleCreateScene = (name: string, bars: number) => {
  void (async () => {
    try {
      const result = await createScene(name, bars);
      assertOk(result, "CreateScene");
      await refresh();
      setSelectedSceneName(name);
    } catch (err) {
      setError(errorMessage(err));
    }
  })();
};
```
Reuse for New Script / Run / Debug / Stop / Delete Script handlers — `assertOk(result, "<ActionName>")` + `await refresh()` + local selection-state update is the established round-trip shape.

**Selection-validity-repair effect** (lines 84-90): re-selects a valid item and drops a stale one whenever `view` changes — apply the same pattern to the selected script in the library list.

**Layout/toolbar/empty-state shell** (lines 209-276): `Toolbar title="..."`, `loading ? <p>...</p> : (...)`, an `emptyState` paragraph when nothing is selected, `ScrollRegion` wrapping the scrollable list. Matches the UI-SPEC's declared empty-state copy ("No scripts yet" / "Run or Debug this script to see live logs...").

**New territory this analog does NOT cover (call out explicitly in the plan, do not force-fit):** Monaco editor mounting/theming (UI-SPEC's "Monaco chrome reconciliation" section is authoritative here, no existing analog), SSE subscription for live debug-panel streaming (see Shared Patterns below), and the Run/Debug launch dialog (see `HelpOverlay.tsx` under Shared Patterns).

---

## Shared Patterns

### Diagnostic code convention
**Source:** repo-wide `{DOMAIN}_{CONDITION}` convention, e.g. `GOLC_TRANSPORT_ADAPTER_MISSING` (`process.go` line 162), `GOLC_CONTRACTS_NAME_DUPLICATE` (`generate.go` line 82), `GOLC_API_SCOPE_REQUIRED` (`auth.go` line 98), `GOLC_APIKEY_SCOPE_INVALID` (`apikeys.go` line 62).
**Apply to:** every new `internal/script`, `internal/scriptsdk`, and `internal/show/scripts.go` error — mint `GOLC_SCRIPT_*`, `GOLC_SCRIPTSDK_*`, and reuse `GOLC_SHOW_*`/`GOLC_APIKEY_*`-family conventions respectively.

### Scope enforcement (D-06)
**Source:** `internal/api/auth.go` lines 74-99 (`HasScope`/`RequireScope`) — reuse the function and the `show.APIKeyScope` type directly, do not reimplement.
**Apply to:** `internal/script/capability.go`, every SDK call dispatched from `internal/script/host.go`.

### Rate limiting (D-09)
**Source:** `internal/api/ratelimit.go` lines 36-89 (`keyRateLimiter`) — same `golang.org/x/time/rate` mechanism, re-keyed per running script instance instead of per API key.
**Apply to:** `internal/script/capability.go`.

### Bounded, redacted process output capture
**Source:** `internal/trace/transport/process.go` lines 99-126 (`boundedBuffer`) + line 244 (`security.Redact` call site).
**Apply to:** `internal/script/host.go` for every captured stdout/stderr line feeding D-04's live log stream and D-12's "last logs remain visible" state.

### Process-tree hard kill
**Source:** `internal/trace/transport/process.go` lines 247-262 (`killProcessTree`, `taskkill /T /F` on Windows).
**Apply to:** `internal/script/jobobject_windows.go` as the fallback beneath the primary Job-Object-close kill path (RESEARCH.md: Job Object is a strict superset, not a replacement).

### SSE event streaming (D-04, D-05)
**Source:** `internal/api/events.go` — the whole file is the pattern: `eventBroadcaster` (lines 134-186, bounded ring buffer + fan-out), `subscribe`'s replay/resync-by-Last-Event-ID logic (lines 188-264), and `handleEventStream`'s per-connection loop (lines 415-457).
**Apply to:** script log/diagnostics/command-outcome/debug-session events. Reuse the existing single global `/v1/events` stream and its `domainEventPayload.Type` tagging convention (per RESEARCH.md's explicit instruction: "reusing Phase 7's SSE event-stream pattern... not building a second streaming mechanism") rather than opening a second stream endpoint — script events are just another `Type` value on the same stream, or (if per-run scoping is required) a new but structurally identical broadcaster following this exact shape.

### Modal/dialog pattern (Run/Debug launch dialog, D-07)
**Source:** `frontend/src/shell/HelpOverlay.tsx` (per 08-UI-SPEC.md's explicit correction of 06-UI-SPEC.md's stale Radix reference) — backdrop-click + Escape to close, focus moved to dialog on open, `role="dialog" aria-modal="true"`, no Radix.
**Apply to:** the Run/Debug launch dialog in `ScriptsWorkspace.tsx`.

### Show-state aggregate extension (D-17)
**Source:** `internal/show/state.go` lines 38-49 (`State` struct fields) + its `Validate()` method's per-entity validation calls (lines 109-190, e.g. `pool.ValidateUniqueNames(s.Pools)`, `scene.ValidateSceneUniqueNames(s.Scenes)`).
**Apply to:** `internal/show/scripts.go` + the `state.go` edit adding the `Scripts` field and its own `script.ValidateScriptUniqueNames(s.Scripts)` call.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/script/jobobject_windows.go` | utility | event-driven | No Windows Job Object precedent exists anywhere in the repo yet — this is genuinely new territory (RESEARCH.md Pattern 2/Pitfall 3 are the authoritative reference, not a codebase analog). Closest partial precedent is `process.go`'s `killProcessTree`, reused as the fallback layer only. |
| `internal/script/jobobject_other.go` | utility | — | No existing `_windows.go`/`_other.go` build-tag-split pair exists in the repo to copy the file-splitting convention from; follow standard Go build-tag conventions directly. |
| `internal/script/debugbridge.go` (CDP client half) | service | event-driven | No CDP/WebSocket-client precedent exists in the repo; `mafredri/cdp`'s own documentation (per RESEARCH.md Code Examples) is the primary reference for the inbound (Deno-facing) half. The outbound (SSE-facing) half reuses `internal/api/events.go` per Shared Patterns above. |
| Monaco editor integration (inside `ScriptsWorkspace.tsx`) | component | — | No existing Monaco or any code-editor-widget integration exists in `frontend/src`; 08-UI-SPEC.md's "Monaco chrome reconciliation" section is the authoritative reference, not a codebase analog. |

## Metadata

**Analog search scope:** `internal/trace/transport/`, `internal/api/`, `internal/contracts/`, `internal/show/`, `internal/scene/`, `internal/pool/` (referenced), `frontend/src/workspaces/build/`, `frontend/src/shell/`.
**Files scanned:** 9 read in full (`process.go`, `ratelimit.go`, `contracts/generate.go`, `api/auth.go`, `api/events.go`, `show/apikeys.go`, `show/schema.go` excerpt, `scene/scene.go` excerpt, `ScenesLooksWorkspace.tsx`), plus `FixtureLibraryWorkspace.tsx` (found to be a stub, corrected against RESEARCH.md's suggestion).
**Pattern extraction date:** 2026-07-25
