# Phase 9: Front-Door UI Completion - Pattern Map

**Mapped:** 2026-07-27
**Files analyzed:** 12
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/fixture/directory.go` (extracted `ListDirectory`) | utility | file-I/O | `internal/command/artnet.go:325` (`loadFixtureDirectory`) | exact (literal extraction) |
| `internal/fixture/ofl/manufacturers.go` | service | request-response (network fetch) | `internal/fixture/ofl/fetch.go` (`Fetch`/`validateTargetURL`) | exact |
| `internal/wails/svc_fixturelibrary.go` (`FixtureLibraryService`) | service | CRUD (read-projection + mutating import) | `internal/wails/svc_show.go` (`ShowService`) | exact |
| `internal/wails/svc_show.go` (extend: `PickShowPath`/`RelaunchWithShow`, or new methods on `App`) | service / controller | event-driven (process lifecycle) | `internal/wails/app.go` (`ensureDaemon`, `defaultSpawn`, `OnStartup`/`OnShutdown`) | exact |
| `internal/wails/app.go` (extend: store `ctx`, add `RelaunchWithShow`) | controller | event-driven (process lifecycle) | `internal/wails/app.go` itself (`OnStartup`, `defaultSpawn`, `spawnFunc`) | exact (self-extension) |
| `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` (replace `ComingSoon` stub) | component | request-response (list + inline inspect + mutate) | `frontend/src/workspaces/build/ScriptsWorkspace.tsx` | exact (library + inline detail pattern) |
| `frontend/src/workspaces/show/ShowsWorkspace.tsx` (new) | component | request-response (CRUD-ish: open/new/switch) | `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx` | exact (same Show nav group, same Toolbar/Panel/ScrollRegion/EmptyState skeleton) |
| `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx` (new) | component | event-driven (overlay driven by live domain reads) | `frontend/src/workspaces/build/ScriptsWorkspace.tsx` (state-derivation discipline) + `.planning/sketches/references/onboarding-readiness-impact.md` (locked structure/CSS) | role-match (no existing overlay/wizard component in this codebase; CSS is verbatim-locked, not inferred) |
| `frontend/src/workspaces/show/GuidedFirstShow/stages/*.tsx` (new, one per stage) | component | request-response (reads live ShowService/FixturePatchService/ProgrammingService state) | `frontend/src/workspaces/build/ScriptsWorkspace.tsx` (per-panel load/refresh/error pattern) | role-match |
| `frontend/src/lib/wailsBridge.ts` (extend: `listLocalFixtures`, `searchOflFixtures`, `inspectFixtureCandidate`, `importFixture`, `pickShowPath`, `relaunchWithShow`) | utility (bridge) | request-response | `frontend/src/lib/wailsBridge.ts` existing `listProgramming`/`offlineProgrammingView`/`saveShow`/`detectRecoveryPoints` block | exact (extend same file, same contract) |
| `frontend/src/shell/navigation.ts` (add `"show-shows"` destination) | config | CRUD (static config) | `frontend/src/shell/navigation.ts` itself (`NAV_GROUPS[0]` Show group) | exact (self-extension) |
| `internal/wails/app_test.go` (extend for `RelaunchWithShow`) | test | event-driven | `internal/wails/app_test.go` existing `ensureDaemon`/`spawnFunc`/`dialFunc` test-double tests | exact |

## Pattern Assignments

### `internal/fixture/directory.go` (utility, file-I/O)

**Analog:** `internal/command/artnet.go:321-354` (`loadFixtureDirectory`)

**Core pattern to extract verbatim** (lines 325-354):
```go
func loadFixtureDirectory(dir string) (map[string]fixture.FixtureDefinition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %v", err)
	}
	byStableKey := map[string]fixture.FixtureDefinition{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), readErr)
		}
		def, decodeErr := fixture.Decode(data)
		if decodeErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), decodeErr)
		}
		identity, pinErr := fixture.Pin(def)
		if pinErr != nil {
			return nil, fmt.Errorf("GOLC_ARTNET_FIXTURES_DIR_READ_FAILED: %s: %v", entry.Name(), pinErr)
		}
		byStableKey[identity.StableKey] = def
	}
	return byStableKey, nil
}
```
**Extraction instruction (RESEARCH-confirmed):** move this into an exported `internal/fixture.ListDirectory(dir string) ([]fixture.FixtureDefinition, error)` — a slice, not a map, since the new Wails list view needs stable ordering. Update `artnet.go`'s `newArtnetFixtureResolver` to call the new exported function instead of keeping its own private copy — never two implementations. Keep the exact `GOLC_ARTNET_FIXTURES_DIR_READ_FAILED`-style error code discipline (rename the code prefix appropriately, e.g. `GOLC_FIXTURE_DIR_READ_FAILED`, per the `{DOMAIN}_{CONDITION}` convention).

---

### `internal/fixture/ofl/manufacturers.go` (service, request-response/network)

**Analog:** `internal/fixture/ofl/fetch.go`

**SSRF-guard host-constant pattern** (lines 49-69):
```go
const (
	defaultOFLHost = "raw.githubusercontent.com"
	defaultOFLURLPattern = "https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/%s/%s.json"
	fetchTimeout = 15 * time.Second
	maxResponseBytes = 2 * 1024 * 1024
)
```
**Fetch signature/discipline pattern** (lines 82-100): build context with timeout, validate target URL/host before requesting, bound `CheckRedirect` to re-validate host on every redirect hop. New manufacturer-index fetch (`fixtures/manufacturers.json` at the SAME `defaultOFLHost`) must reuse this exact `validateTargetURL`/timeout/size-cap discipline — do not introduce a second host or a wildcard/suffix host check (Pitfall 3 in RESEARCH.md). Per RESEARCH's Open Question 1 recommendation, scope D-03's OFL search to manufacturer-name-only for v1 (no `api.github.com` addition).

**Error code convention:** `GOLC_FIXTURE_OFL_FETCH_FAILED` / `GOLC_FIXTURE_OFL_MIRROR_HOST` — new manufacturer-index errors should follow the same `GOLC_FIXTURE_OFL_*` prefix.

---

### `internal/wails/svc_fixturelibrary.go` (service, CRUD read-projection + mutating import)

**Analog:** `internal/wails/svc_show.go` (whole file, 252 lines — read once, already in context)

**Doc-comment convention** (lines 1-33): explain WHY this file exists, what it reuses vs. what's genuinely new, and explicitly disclaim duplicate implementations. New file's doc comment should state: "ListLocal/SearchOFL are Wails-only read projections (no CLI-parity route, mirroring `ListPatch`/`Inspect`/`Diagnose`'s established precedent); Import/Validate/Inspect forward to the existing `fixture import`/`fixture validate`/`fixture inspect` CLI routes via `execute`, never a second decode/pin/normalize implementation."

**`execute` helper pattern to copy verbatim** (lines 60-71):
```go
func (s *ShowService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}
```
`FixtureLibraryService.Validate`/`.Inspect`/`.Import` should call `s.execute([]string{"fixture", "validate", path})` / `{"fixture", "inspect", path}` / `{"fixture", "import", "--ofl", ref, "--out", outPath}` etc. exactly this way (see also svc_fixturepatch.go's identical `execute` helper cited in RESEARCH.md Pattern 2).

**Read-projection pattern** (lines 114-155, `Inspect`): load domain state read-only, build a `[]XView` slice with `make(..., 0, len(...))` pre-sized, project each field into camelCase JSON tags. `ListLocal`/`SearchOFL` should follow this exact shape — call `fixture.ListDirectory(dir)` / the new manufacturer fetch+filter, then project into `FixtureLibraryRowView{StableKey, Name, Manufacturer, Source, ValidationStatus ...}`.

**Nil-slice JSON-marshal pitfall** (lines 157-184, doc comment + `Diagnose`'s nil-normalization at line 195-198): every slice-typed view field MUST be normalized to a non-nil empty slice before return — `encoding/json` marshals nil as `null`, and `null.length`/`undefined.length` throws with no error boundary in this app. Copy this discipline into every new `FixtureLibraryRowView`/`OFLFixtureView` list return.

---

### `internal/wails/app.go` (extend: `RelaunchWithShow`, stored `ctx`)

**Analog:** `internal/wails/app.go` itself — `OnStartup`/`OnShutdown`/`ensureDaemon`/`defaultSpawn`/`spawnFunc`

**`OnStartup` pattern (add ctx storage here)** (lines 198-210):
```go
func (a *App) OnStartup(ctx context.Context) {
	a.ensureDaemon(ctx)
	failures := a.hotkeys.RegisterAll()
	...
	a.events.Start(ctx)
}
```
Per RESEARCH Pitfall 1 and Open Question 3: add a `ctx context.Context` field to `App`, set it as the first line of `OnStartup` (`a.mu.Lock(); a.ctx = ctx; a.mu.Unlock()` or an unguarded field if writes are startup-only), and expose an accessor for `PickShowPath`/`RelaunchWithShow` to call `wailsruntime.OpenFileDialog(a.ctx, ...)` / `wailsruntime.Quit(a.ctx)`.

**`spawnFunc` injectable test-double pattern to mirror** (lines 106-117):
```go
type spawnFunc func(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error)
```
`RelaunchWithShow`'s own process-spawn call should be injectable the same way (a field on `App`, swappable in tests) — this is the exact precedent `internal/wails/app_test.go` already exercises for `ensureDaemon`.

**`defaultSpawn` pattern to mirror one level up** (lines 333-364):
```go
func defaultSpawn(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
	exePath, err := resolveDaemonExecutable(cfg)
	if err != nil {
		return nil, nil, err
	}
	args := []string{"artnet", "serve", "--show", cfg.ShowPath, ...}
	cmd := exec.CommandContext(ctx, exePath, args...)
	if cfg.ProjectRoot != "" {
		cmd.Env = append(os.Environ(), "GOLC_PROJECT_ROOT="+cfg.ProjectRoot)
	}
	stderr := &daemonStderrBuffer{}
	cmd.Stderr = stderr
	if startErr := cmd.Start(); startErr != nil {
		return nil, nil, fmt.Errorf("GOLC_WAILS_DAEMON_SPAWN_FAILED: %v", startErr)
	}
	return cmd, stderr, nil
}
```
`App.RelaunchWithShow(newShowPath string) error` mirrors this one level up: resolve `os.Executable()` instead of `resolveDaemonExecutable`, build `cmd.Env` from `os.Environ()` with `GOLC_DESKTOP_SHOW` overridden, `cmd.Start()` (NOT `CommandContext(ctx,...)` since `ctx` is about to be cancelled by this process's own exit — use a fresh background exec so the child survives the parent's `Quit`), then call `wailsruntime.Quit(a.ctx)` only on `cmd.Start()` success (Security Domain: never orphan the operator with zero running instances on spawn failure — return the error without calling `Quit`).

**`OnShutdown` reverse-order kill pattern** (lines 217-229) — unmodified; the OLD process's daemon child still gets killed via this exact path once `Quit` triggers `OnShutdown`.

**Wails runtime dialog/quit signatures (from RESEARCH, vendored source):**
```go
func OpenFileDialog(ctx context.Context, dialogOptions OpenDialogOptions) (string, error)
func Quit(ctx context.Context)
```
Use `OpenDialogOptions{Filters: []FileFilter{...}}` to restrict `PickShowPath` to `*.golc`.

---

### `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx` (component, request-response)

**Analog:** `frontend/src/workspaces/build/ScriptsWorkspace.tsx` (library + inline-detail structural template; explicitly named by 08-UI-SPEC.md as the correct template, NOT the bare `ComingSoon` stub this file currently is)

**Imports pattern** (lines 42-79 of ScriptsWorkspace.tsx):
```typescript
import { useCallback, useEffect, useState } from "react";
import { FileCode2, Plus, X, Check, Save, Trash2, ShieldCheck, Play, Bug, Square } from "lucide-react";
import {
  assertOk, errorMessage, listScripts, ... type ScriptSummaryView, type WailsResult,
} from "../../lib/wailsBridge";
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip, { type ChipTone } from "../../components/primitives/Chip/Chip";
import Button from "../../components/primitives/Button/Button";
import { useInspectorSlot } from "../../shell/InspectorSlot";
import styles from "./ScriptsWorkspace.module.css";
```
`FixtureLibraryWorkspace.tsx` should import the same primitive set (`Toolbar`, `ScrollRegion`, `ListRow`, `Chip`, `Button`) plus the new bridge functions (`listLocalFixtures`, `searchOflFixtures`, `inspectFixtureCandidate`, `importFixture`).

**Current stub to replace in full** (`FixtureLibraryWorkspace.tsx` lines 1-14):
```tsx
import { Lightbulb } from "lucide-react";
import ComingSoon from "../ComingSoon";
export default function FixtureLibraryWorkspace() {
  return (
    <ComingSoon title="Fixture Library" icon={Lightbulb}
      description="The Fixture Library browser isn't wired into the desktop app yet."
      cliHint="Use golc fixture from the command line to inspect fixture definitions in the meantime." />
  );
}
```

**Row + status Chip pattern:** UI-SPEC's "each row shows name, manufacturer, and a validation-status chip (frame-lock/armed/revoked)" maps directly onto `ScriptsWorkspace.tsx`'s `ListRow` + `Chip tone={...}` usage — replicate that composition for local/OFL fixture rows, toggled by a "My Library" / "Open Fixture Library" source switch (Copywriting Contract).

**Inline inspect + confirm-CTA pattern:** the "select a row → inline inspect → Add to Library" flow (D-02) should follow `ScriptsWorkspace.tsx`'s `useInspectorSlot`-driven detail rendering + validation-gates-launch discipline (UI-SPEC: "Add to Library stays disabled until it passes, mirroring ScriptsWorkspace's identical validation-gates-launch pattern").

---

### `frontend/src/workspaces/show/ShowsWorkspace.tsx` (new component, request-response)

**Analog:** `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx` (whole file, 221 lines — read once, already in context; explicitly flagged by its own doc comment as "this workspace does not bind show open... there is no 'open a different show' flow to wire yet" — the exact gap this new file closes)

**Doc-comment / structural template to mirror** (lines 1-19 for framing, 42-70 for state shape):
```tsx
export default function SaveRecoveryWorkspace() {
  const [points, setPoints] = useState<RecoveryPointView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  ...
  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [nextPoints, report] = await Promise.all([detectRecoveryPoints(), diagnoseShow()]);
      setPoints(nextPoints);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void refresh(); }, [refresh]);
```
`ShowsWorkspace.tsx` should follow this identical load/refresh/error skeleton: `useState` for `currentShowPath`, `switching`, `error`; a `refresh()` that calls `inspectShow()` for the current path.

**Action-button-with-inline-input pattern** (lines 128-169, the Save/Save-As row):
```tsx
<Toolbar title="Save & Recovery" icon={SaveIcon} />
<div className={styles.canvas}>
  ...
  <Panel>
    <PanelHeader label="Save" icon={SaveIcon} />
    <div className={styles.saveRow}>
      <Button variant="primary" icon={SaveIcon} disabled={saving} onClick={() => void handleSave()}>
        {saving ? "Saving…" : "Save"}
      </Button>
      ...
    </div>
  </Panel>
```
`ShowsWorkspace.tsx`'s "Current Show" panel + "Open Show…"/"New Show…" primary buttons should follow this exact `Toolbar` → `Panel`/`PanelHeader` → action-row composition, using `variant="primary"` buttons per the UI-SPEC's accent-reserved-for list. The relaunch transient copy ("Switching shows — GOLC will reload in a moment…") replaces the `saving ? "Saving…" : "Save"` ternary pattern with an equivalent busy-label swap.

**Error-display pattern** (line 136): `{error ? <p className={styles.errorText}>{error}</p> : null}` — reuse verbatim for the relaunch-failure copy ("Couldn't switch to this show. GOLC is still running the previous show.").

---

### `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx` (new, overlay)

**Analog (structure/CSS):** `.planning/sketches/references/onboarding-readiness-impact.md` (LOCKED, verbatim per D-11) — no existing frontend analog exists for the overlay shell itself (RESEARCH confirmed zero `guided|onboard` hits in `frontend/src`).

**Locked CSS to reproduce verbatim** (`onboarding-readiness-impact.md` lines 19-29):
```css
.guided-flow {
  height: 100%;
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  gap: 7px;
}
.guide-step[aria-current="step"] {
  border-left: 3px solid var(--color-primary);
  background: var(--color-primary-soft);
}
.impact-preview {
  border: 1px solid rgba(200, 162, 75, .5);
  background: rgba(200, 162, 75, .08);
}
```
Do NOT round `210px`/`7px` to the project's 4px/8px spacing scale — D-11 formally locks this exception.

**Analog (data-derivation discipline):** `frontend/src/workspaces/build/ScriptsWorkspace.tsx`'s load/refresh pattern (`useCallback` + `useEffect` + live bridge calls, no separately persisted local "progress" state) — apply this same discipline per-stage: each stage's status must derive from a fresh `ShowService.Inspect()`/`FixturePatchService.ListPatch()`/`ProgrammingService.ListProgramming()` call on render, never a persisted boolean (RESEARCH Pitfall 4, "the guide reads actual domain readiness; it does not own duplicate state").

**Auto-launch trigger source:** `internal/wails/svc_show.go`'s `Inspect()` (lines 118-155) already returns `Pools`/`Deployments` counts; `ProgrammingService.ListProgramming()` (bridge line ~872) returns `scenes`. D-08's "no fixtures/pools/scenes" check reads these two existing calls — no new backend signal needed.

---

### `frontend/src/lib/wailsBridge.ts` (extend)

**Analog:** the existing `ShowService`/`ProgrammingService` block in the same file (lines 848-1183, read in full)

**Service-accessor + offline-fallback pattern** (lines 848-880):
```typescript
function programmingService() {
  return window.go?.wails?.ProgrammingService;
}
export function offlineProgrammingView(): ProgrammingView {
  return { scenes: [], themes: [], presets: [], chases: [], motions: [], blends: [], instances: [] };
}
export async function listProgramming(): Promise<ProgrammingView> {
  const svc = programmingService();
  if (!svc) return offlineProgrammingView();
  try {
    return await svc.ListProgramming();
  } catch {
    return offlineProgrammingView();
  }
}
```
Add a `fixtureLibraryService()` accessor + `offlineFixtureLibraryView()` constant + `listLocalFixtures()`/`searchOflFixtures(query)`/`inspectFixtureCandidate(...)`/`importFixture(...)` following this exact three-part contract (accessor / offline-fallback constant / try-catch wrapper — "never blank," per RESEARCH Pattern 3).

**Simple mutating-call pattern** (lines 884-891, `createScene`):
```typescript
export async function createScene(name: string, bars: number): Promise<WailsResult> {
  const svc = programmingService();
  if (!svc) return bridgeUnavailableResult();
  return svc.CreateScene(name, bars);
}
```
`pickShowPath()` and `relaunchWithShow(path)` should follow this exact shape (no offline fallback needed for a pure action call — use `bridgeUnavailableResult()` exactly as `createScene` does).

**Show-domain call pattern** (lines 1108-1119, `saveShow`/`saveShowAs`):
```typescript
export async function saveShow(): Promise<WailsResult> {
  const svc = showService();
  if (!svc) return bridgeUnavailableResult();
  return svc.Save();
}
```
Mirror this exactly for `pickShowPath`/`relaunchWithShow` against the (new or extended) `ShowService`/`App`-bound methods.

---

### `frontend/src/shell/navigation.ts` (extend)

**Analog:** the file's own existing `NAV_GROUPS[0]` ("Show") entry array

**Pattern** (lines 31-39):
```typescript
{
  label: "Show",
  destinations: [
    { id: "show-overview", label: "Overview" },
    { id: "show-save-recovery", label: "Save & Recovery" },
    { id: "show-settings", label: "Settings" },
  ],
},
```
Add `{ id: "show-shows", label: "Shows" }` (label locked verbatim by 09-UI-SPEC.md's Copywriting Contract) to the `destinations` array, and add `"show-shows"` to the `DestinationId` union at the top of the file (lines 8-19). Also add one `case "show-shows":` arm to `frontend/src/shell/WorkspaceRouter.tsx`'s switch (not separately classified above — same trivial self-extension pattern).

---

### `internal/wails/app_test.go` (extend)

**Analog:** the file's own existing `ensureDaemon`/`spawnFunc`/`dialFunc` test-double tests (per RESEARCH's Validation Architecture table: "mirrors app_test.go's existing spawnFunc test-double pattern")

**Pattern to mirror:** construct an `App` with an injected fake `spawnFunc` (or a new injected `relaunchSpawnFunc`) that records the `cmd.Env`/args passed rather than starting a real process, then assert `GOLC_DESKTOP_SHOW=<newPath>` appears in the recorded env and that `Quit` is called only after a successful fake spawn (never on spawn failure) — see RESEARCH's Security Domain "Orphaned respawned process on relaunch failure" mitigation.

---

## Shared Patterns

### Wails-only read projection (no CLI-parity route)
**Source:** `internal/wails/svc_show.go`'s `Inspect`/`Diagnose`/`DetectRecoveryPoints` (and `svc_fixturepatch.go`'s `ListPatch`, `svc_programming.go`'s `ListProgramming`)
**Apply to:** `FixtureLibraryService.ListLocal`/`SearchOFL` — no new `internal/command` "fixture list" route is required; read domain state directly and project into a JSON-safe view.
```go
func (s *ShowService) Inspect() (ShowInspectView, error) {
	state, err := show.LoadForRead(s.root, s.showPath)
	...
}
```

### Preview-then-commit / forward-to-existing-CLI-route for every mutation
**Source:** `internal/wails/svc_fixturepatch.go`'s `execute` helper (identical to `svc_show.go`'s own, shown above)
**Apply to:** `FixtureLibraryService.Validate`/`.Inspect`/`.Import` — every one MUST forward to `command.NewDefaultCommandRegistry()` + `registry.Execute(...)` against the existing `fixture validate`/`fixture inspect`/`fixture import` routes, never re-implement decode/pin/normalize.
```go
func (s *FixturePatchService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}
```

### Graceful bridge-unavailable degradation ("never blank")
**Source:** `frontend/src/lib/wailsBridge.ts`'s `offlineProgrammingView`/`offlinePatchView`/`offlineShowInspectView`/`offlineScriptList` family
**Apply to:** every new bridge export (`listLocalFixtures`, `searchOflFixtures`, `pickShowPath`, `relaunchWithShow`) — each needs an accessor that checks `window.go?.wails?.<Service>`, an explicit offline-fallback constant/value, and a try/catch that never throws to the caller.

### Nil-slice JSON-marshal "never null/undefined.length" guard
**Source:** `internal/wails/svc_show.go` lines 157-198 (`DiagnosticReportView` doc comment + `Diagnose`'s `fileLevelIssues` nil-normalization)
**Apply to:** every new Go view struct with a slice field (`FixtureLibraryRowView[]`, `OFLFixtureView[]`) — normalize nil to `[]T{}` before returning, since `encoding/json` (no `omitempty`) marshals nil as `null`, and `null.length`/`undefined.length` throws with zero error-boundary protection anywhere in this app (`main.tsx` has none).

### `{DOMAIN}_{CONDITION}` diagnostic code convention
**Source:** `GOLC_ARTNET_FIXTURES_DIR_READ_FAILED`, `GOLC_WAILS_REGISTRY_BUILD_FAILED`, `GOLC_FIXTURE_OFL_FETCH_FAILED`, `GOLC_WAILS_DAEMON_SPAWN_FAILED`
**Apply to:** any new diagnostic — e.g. `GOLC_FIXTURE_DIR_READ_FAILED` (extracted ListDirectory), `GOLC_FIXTURE_OFL_MANUFACTURERS_FETCH_FAILED` (new manufacturer-index fetch), `GOLC_WAILS_RELAUNCH_SPAWN_FAILED` (RelaunchWithShow spawn failure).

### Toolbar → Panel/PanelHeader → ScrollRegion/EmptyState workspace skeleton
**Source:** `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx` (whole-file structure)
**Apply to:** `ShowsWorkspace.tsx` in full; `FixtureLibraryWorkspace.tsx`'s outer shell (list panel + inline-inspect panel as two `Panel`s in a `styles.layout` flex/grid, matching `SaveRecoveryWorkspace`'s two-`Panel` `.layout` composition).

### Injectable spawn/dial test-double pattern
**Source:** `internal/wails/app.go`'s `spawnFunc type` + `dialFunc type` fields on `App`
**Apply to:** `App.RelaunchWithShow`'s own process-spawn call — make it a swappable field (mirroring `a.spawn`) so `app_test.go` can assert env/args without starting a real process.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.tsx` (overlay shell itself, not its data-derivation logic) | component | event-driven | No overlay/wizard/multi-stage-flow component exists anywhere in `frontend/src` today (RESEARCH: zero `guided\|onboard` hits). Structural layout comes from the locked sketch reference's CSS, not a codebase analog; data-derivation discipline borrows from `ScriptsWorkspace.tsx` as noted above. |
| `frontend/src/workspaces/show/GuidedFirstShow/stages/*.tsx` (per-stage components) | component | request-response | Same reason — no existing stage-rail/wizard-step component pattern in this codebase; use the locked design doc's stage list (Fixtures/Patch/Program/Assign/Verify) plus `ScriptsWorkspace.tsx`'s panel-load pattern as the nearest structural precedent. |

## Metadata

**Analog search scope:** `internal/command/`, `internal/wails/`, `internal/fixture/`, `internal/fixture/ofl/`, `frontend/src/workspaces/`, `frontend/src/lib/`, `frontend/src/shell/`, `.planning/sketches/references/`
**Files scanned:** `internal/wails/svc_show.go`, `internal/wails/app.go`, `internal/wails/svc_fixturepatch.go` (referenced via RESEARCH), `internal/command/artnet.go`, `internal/command/fixture.go` (route names confirmed), `internal/fixture/ofl/fetch.go`, `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx`, `frontend/src/workspaces/build/ScriptsWorkspace.tsx`, `frontend/src/workspaces/build/FixtureLibraryWorkspace.tsx`, `frontend/src/lib/wailsBridge.ts`, `frontend/src/shell/navigation.ts`, `.planning/sketches/references/onboarding-readiness-impact.md`
**Pattern extraction date:** 2026-07-27
