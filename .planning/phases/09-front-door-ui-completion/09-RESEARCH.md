# Phase 9: Front-Door UI Completion - Research

**Researched:** 2026-07-27
**Domain:** Wails v2 desktop UI wiring (Go backend routes → React frontend), process-lifecycle respawn, GitHub-hosted catalog access
**Confidence:** HIGH (codebase-grounded for 95% of findings; MEDIUM for OFL catalog-search mechanics, which needed live verification against the upstream GitHub repo)

## Summary

Phase 9 closes three concrete UI gaps against an already-complete backend: (1) the Fixture
Library workspace (`ComingSoon` stub) needs browse/inspect/import wired to `fixture
validate`/`inspect`/`import`, plus a genuinely new "list what's available" capability that has
**no backend route today**; (2) show open/new/switch needs on-screen controls, but
`cmd/golc-desktop/main.go` binds all 9 constructed Wails services to one `cfg.ShowPath` read
once at process startup, so "switching shows" is architecturally a **process relaunch**, not a
live in-place swap; (3) Guided First Show (Sketch 004-B) has zero existing frontend code and
needs to be built from a fully locked design spec.

The 09-UI-SPEC.md (already approved, `.planning/phases/09-front-door-ui-completion/09-UI-SPEC.md`)
has already resolved several implementation-mechanism questions this research would otherwise
need to answer from scratch: the new Show-group nav entry is literally titled **"Shows"**, and
**both** D-04's custom-YAML file picker and D-05/D-06's show-path picker are specified to use
Wails' native `runtime.OpenFileDialog` — not a new frontend dependency. This research grounds
the two open backend-mechanism questions the UI-SPEC left to research/planning: the fixture-list
route shape, and the concrete show-relaunch mechanism, plus verifies whether OFL catalog search
is genuinely new network code (confirmed: **yes**, no simple existing index file makes it
possible without new logic).

**Primary recommendation:** Extract the existing private `loadFixtureDirectory` helper
(`internal/command/artnet.go:325`) into an exported `internal/fixture` function shared by both
its existing caller and a new **Wails-only** (no CLI-parity route) `ListLocal` projection method,
mirroring the `ListPatch`/`Inspect`/`Diagnose` "no registered read route returns structured
data" precedent already established in this codebase. For show switching, extend `App` to store
its Wails `context.Context` from `OnStartup`, add a bound method that spawns a **second
`golc-desktop.exe` process** (self-exec, mirroring `app.go`'s existing `defaultSpawn` daemon-spawn
pattern but targeting `os.Executable()` instead of `golc-project.exe`) with `GOLC_DESKTOP_SHOW`
set to the new path, then calls `wailsruntime.Quit(ctx)` on the current instance — no in-process
live-switch of any of the 9 services' bound `showPath` fields.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Fixture browse/search (local directory) | API/Backend (Go, `internal/fixture` + Wails service) | Frontend Server (SSR: N/A — desktop) / Browser (React render) | Directory scan and YAML decode must happen Go-side (`internal/fixture.Decode`/`Pin`); the frontend only renders the projected list |
| Fixture browse/search (OFL catalog) | API/Backend (Go, `internal/fixture/ofl`, SSRF-guarded) | Browser (React render, debounced search input) | Network fetch must stay behind the existing SSRF guard; frontend never calls GitHub directly |
| Fixture inspect/validate/import | API/Backend (existing `fixture validate`/`inspect`/`import` CLI routes, unchanged) | Browser (inline inspect panel render) | Reuses Phase 2's canonical pipeline verbatim — no new validation logic |
| Show open/new/switch | API/Backend (Go, new `App` respawn method + `os/exec` self-spawn) | Browser (native picker trigger via Wails `runtime.OpenFileDialog`) | Process-lifecycle decisions belong in the Go host (`internal/wails/app.go`), never the webview |
| Guided First Show orchestration | Browser (React component, reads existing show/pool/patch state) | API/Backend (existing `ShowService`/`FixturePatchService`/`ProgrammingService` calls, unchanged) | The guide is a client-side flow layer over already-existing backend calls — "the guide reads actual domain readiness; it does not own duplicate state" (locked design contract) |
| Guided First Show auto-launch trigger | Browser (React, `OverviewWorkspace` mount-time check) | API/Backend (`ShowService.Inspect` — already returns pools/deployments/scenes counts) | D-08's "no fixtures/pools/scenes" check is a pure client-side read of data `Inspect`/`ListProgramming` already return; no new backend signal needed |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/wailsapp/wails/v2` | v2.13.0 (pinned, `go.mod`) [VERIFIED: go.mod] | Go↔webview bridge, native dialogs, process lifecycle events | Already the project's only desktop-shell framework since Phase 6; no alternative considered |
| React | 19.2.7 [VERIFIED: frontend/package.json] | Frontend rendering | Existing stack, unchanged |
| `lucide-react` | ^1.27.0 [VERIFIED: frontend/package.json; also confirmed by 09-UI-SPEC.md] | Icons | UI-SPEC explicitly corrects an earlier "no icon library" claim — this is the real, installed, already-used icon set |
| TypeScript | 7.0.2 [VERIFIED: frontend/package.json] | Frontend types | Unchanged |
| Vite / Vitest | 8.1.4 / ^4.1.10 [VERIFIED: frontend/package.json] | Build/test | Unchanged |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/wailsapp/wails/v2/pkg/runtime` | same v2.13.0 module | `OpenFileDialog`/`SaveFileDialog`/`OpenDirectoryDialog`, `Quit`, `EventsEmit` | Already imported in `internal/wails/events.go`; add `OpenFileDialog`/`Quit` calls to a new service, no new dependency |
| Go stdlib `net/http` | stdlib | OFL catalog fetch (manufacturer index + any new per-manufacturer listing) | Already the pattern `internal/fixture/ofl/fetch.go` uses; no HTTP client library is justified for this small, occasional fetch volume (RESEARCH precedent from Phase 2) |
| Go stdlib `os/exec` | stdlib | Self-respawn for show switching | Already the exact pattern `internal/wails/app.go`'s `defaultSpawn` uses for the daemon child — reuse the same discipline for a desktop-self-respawn |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Wails-only `ListLocal`/`SearchOFL` (no CLI route) | A new self-registered `fixture list` CLI route + thin Wails wrapper | CLI-parity route adds real value (scriptable, testable via `go test` at the command layer, consistent with `fixture validate`/`inspect`/`import`'s existing CLI-first pattern) but is NOT required by any locked decision — D-01 explicitly leaves "CLI-parity route vs Wails-only scan" to research/planning. Given `ListPatch`/`Inspect`/`Diagnose` in this exact codebase are already Wails-only projections with no CLI-parity route (and their doc comments explicitly justify this as "no registered read route returns structured data ... shelling out and re-parsing would be a second implementation"), the Wails-only path is the lower-risk, precedent-matching choice. A CLI-parity `fixture list` route remains a legitimate v1.x addition if scriptability is later required. |
| Self-exec relaunch (D-05) | True live show-path swap across all 9 services | Live-swap requires re-plumbing `ShowPath` through every one of `SafetyService`/`PlaybackService`/`SurfaceService`/`MidiService`/`FixturePatchService`/`ArtnetConfigService`/`ProgrammingService`/`ShowService`/`ScriptService` (9 structs, not 7 as `09-CONTEXT.md`'s canonical-refs section states — `ArtnetConfigService`/`SafetyService` are also constructed against `cfg` in `main.go` even though they don't take `ShowPath` directly) plus the supervised `golc-project.exe artnet serve --show <path>` daemon child, which currently has no live-reconfigure IPC message at all (`internal/artnet/ipc`) — out of scope per D-05's own explicit rejection |
| `raw.githubusercontent.com`-only OFL fetch | GitHub REST Contents API (`api.github.com`) for per-manufacturer fixture listing | Confirmed below (Package Legitimacy / Open Questions): no single fetchable index file lists every fixture across all manufacturers on `raw.githubusercontent.com`. A real "search by fixture name" needs either a second allowed host (`api.github.com`, subject to unauthenticated GitHub rate limits of 60 req/hr/IP) or scoping D-01's OFL search to manufacturer-name-only against the existing `fixtures/manufacturers.json` index (already fetchable from the current allowed host) |

**Installation:** No new packages — this phase is UI wiring against an already-vendored stack.
No `npm install`/`go get` required.

**Version verification:** `go.mod` pins `github.com/wailsapp/wails/v2 v2.13.0` [VERIFIED: read
`go.mod` directly]; frontend `package.json` pins `react@19.2.7`, `lucide-react@^1.27.0`,
`typescript@7.0.2`, `vite@8.1.4`, `vitest@^4.1.10` [VERIFIED: read `frontend/package.json`
directly]. No package version changes are needed for this phase.

## Package Legitimacy Audit

No new external packages are introduced by this phase — every capability is either an existing
vendored dependency (`wailsapp/wails/v2`) or a Go/TypeScript stdlib/already-installed API. The
Package Legitimacy Gate is not applicable.

**Packages removed due to SLOP verdict:** none (no new packages evaluated)
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
Fixture Library workspace (browse/import):
  User types search text
        │
        ▼
  FixtureLibraryWorkspace.tsx ──► wailsBridge.ts ──► (new) FixtureLibraryService
        │                                                    │
        │ (toggle: My Library / OFL)                         ├─ ListLocal() ──► internal/fixture directory
        │                                                    │                  scan (extracted from
        │                                                    │                  loadFixtureDirectory)
        │                                                    │
        │                                                    └─ SearchOFL(query) ──► internal/fixture/ofl
        │                                                                             (manufacturer-name
        │                                                                              filter against
        │                                                                              fixtures/manufacturers.json,
        │                                                                              SSRF-guarded GET)
        ▼
  Row selected ──► inline inspect (existing "fixture validate"/"fixture inspect" pipeline,
                    unchanged — reused verbatim via a Wails wrapper) ──► "Add to Library"
                    ──► existing "fixture import" pipeline (unchanged)

Show open/new/switch:
  User clicks "Open Show…" / "New Show…"
        │
        ▼
  ShowsWorkspace.tsx ──► wailsBridge.ts ──► ShowService.PickAndSwitch()
        │                                          │
        │                                          ├─ runtime.OpenFileDialog(ctx, {filter: *.golc})
        │                                          │  (native OS picker — Go-side, ctx captured at
        │                                          │  OnStartup)
        │                                          │
        │                                          └─ App.RelaunchWithShow(newPath)
        │                                                   │
        │                                                   ├─ os/exec spawn os.Executable() with
        │                                                   │  GOLC_DESKTOP_SHOW=<newPath> in child env
        │                                                   │  (mirrors app.go's defaultSpawn daemon
        │                                                   │  pattern, one level up)
        │                                                   │
        │                                                   └─ wailsruntime.Quit(ctx) on THIS instance
        │                                                      (triggers OnShutdown: kills supervised
        │                                                      artnet daemon child, stops hotkeys/events)
        ▼
  New golc-desktop.exe process starts fresh: constructs all 9 services against the NEW
  cfg.ShowPath, runs its own OnStartup/ensureDaemon exactly like a normal launch.

Guided First Show:
  OverviewWorkspace mounts ──► ShowService.Inspect() + ProgrammingService.ListProgramming()
        │                       (already-existing calls)
        │
        │  pools.length==0 && deployments.length==0 && scenes.length==0 (D-08 check)
        ▼
  <GuidedFirstShow /> overlay replaces canvas ──► reuses existing FixturePatchService/
        (stage rail: Fixtures/Patch/Program/                ProgrammingService calls per stage —
         Assign/Verify, locked CSS)                         "the guide reads actual domain
                                                              readiness; it does not own duplicate
                                                              state" (locked interaction contract)
```

### Recommended Project Structure
```
frontend/src/workspaces/
├── build/
│   └── FixtureLibraryWorkspace.tsx      # replaces ComingSoon stub (D-01..D-04)
├── show/
│   ├── ShowsWorkspace.tsx               # NEW — Show nav group 4th entry, "Shows" (D-05..D-07)
│   └── GuidedFirstShow/                 # NEW — overlay component tree (D-08..D-11)
│       ├── GuidedFirstShow.tsx          # top-level overlay, stage-rail + content + evidence aside
│       ├── stages/                      # one file per stage: Fixtures/Patch/Program/Assign/Verify
│       └── GuidedFirstShow.module.css   # verbatim locked CSS (210px/7px grid, D-11)
internal/
├── fixture/
│   └── directory.go                     # NEW — exported ListDirectory(dir) extracted from
│                                         #        internal/command/artnet.go's loadFixtureDirectory
├── fixture/ofl/
│   └── manufacturers.go                 # NEW — Fetch + parse fixtures/manufacturers.json,
│                                         #        name/id substring filter (D-03's basic text search)
└── wails/
    ├── svc_fixturelibrary.go            # NEW — ListLocal/SearchOFL/InspectCandidate/Import wrapper
    ├── svc_show.go                      # EXTEND — add PickShowPath/RelaunchWithShow methods
    └── app.go                            # EXTEND — store ctx from OnStartup; add RelaunchWithShow
```

### Pattern 1: Wails-only read projection (no CLI-parity route)
**What:** A Wails service method reads domain state directly (or, here, scans a directory) and
projects it into a JSON-safe view, with no matching self-registered `internal/command` route.
**When to use:** When the data is read-only and no CLI consumer currently needs the same
projection — this codebase's own `ListPatch`/`Inspect`/`Diagnose`/`ListProgramming` all follow
this pattern already.
**Example:**
```go
// Source: internal/wails/svc_show.go (existing, unmodified — the exact precedent to follow)
// Inspect/Diagnose/DetectRecoveryPoints read the ShowState (or its
// dedicated read-only internal/show helpers) directly and project into a
// JSON-safe view, mirroring ListProgramming/ListPatch/ListSurfaces's
// identical "no registered read route returns structured data" rationale
func (s *ShowService) Inspect() (ShowInspectView, error) {
	state, err := show.LoadForRead(s.root, s.showPath)
	// ... projects state into ShowInspectView
}
```
The new `FixtureLibraryService.ListLocal()` should follow this identical shape: call the newly
exported `fixture.ListDirectory(dir)` directly, project into a `FixtureLibraryRowView`, no CLI
route required.

### Pattern 2: Preview-then-commit for anything that mutates ShowState
**What:** Every mutating Wails call in this codebase that touches `ShowState` goes through
`command.NewDefaultCommandRegistry()` and the exact same CLI route a terminal invocation would
use — never a second, duplicate mutation implementation.
**When to use:** Fixture import's "Add to Library" confirm action MUST forward to the existing
`fixture import` route (unchanged), never re-implement OFL normalize/pin/write logic in the Wails
layer.
**Example:**
```go
// Source: internal/wails/svc_fixturepatch.go (existing, unmodified)
func (s *FixturePatchService) execute(args []string) Result {
	registry, err := command.NewDefaultCommandRegistry()
	if err != nil {
		return Result{ExitCode: 2, Stderr: fmt.Sprintf("GOLC_WAILS_REGISTRY_BUILD_FAILED: %v", err)}
	}
	result := registry.Execute(command.Request{Root: s.root, Args: args})
	return Result{ExitCode: result.ExitCode, Stdout: string(result.Stdout), Stderr: string(result.Stderr)}
}
```
A new `FixtureLibraryService.Import(...)` method should call `s.execute([]string{"fixture",
"import", ...})` exactly this way.

### Pattern 3: Graceful bridge-unavailable degradation
**What:** Every `wailsBridge.ts` export checks `window.go?.wails?.<Service>` and returns an
explicit offline/empty fallback rather than throwing, so `npm run build`'s type-check, a plain
browser preview, and a real missing-bridge runtime state all render identically.
**When to use:** Every new bridge function this phase adds (`listLocalFixtures`,
`searchOflFixtures`, `pickShowPath`, `relaunchWithShow`) must follow this exact contract —
including a distinct `offlineFixtureLibraryView()`-style fallback constant, per the file's own
documented "never blank" convention (`PatchView`/`ProgrammingView`/`ShowInspectView` all follow
it).

### Anti-Patterns to Avoid
- **A second fixture-decode/pin/normalize implementation in the Wails layer:** every fixture
  operation (validate, inspect, import) must forward to the existing `internal/command/fixture.go`
  routes — the new work is purely *listing/searching what's available*, never re-validating.
- **A frontend-only "which show is open" state that outlives the process:** `ShowPath` is a
  Go-process-level constant for the process's whole lifetime (`cmd/golc-desktop/main.go`); do not
  try to make the frontend hold a "current show" state that the backend doesn't also restart
  around — this would drift the moment any of the 9 services makes its next call against the old
  `showPath`.
- **Treating `runtime.EventsEmit`/EventsOn as authoritative for the relaunch transient:** the
  "Switching shows…" UI-SPEC copy is a purely client-side transient shown for the duration of the
  `RelaunchWithShow` call promise, not a pushed event — there's no backend "relaunch progress"
  signal to subscribe to (the process is about to exit).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Native file/directory pickers | A custom React file-browser modal | `runtime.OpenFileDialog`/`OpenDirectoryDialog` (Wails v2 `pkg/runtime`) | Already the locked UI-SPEC decision (D-04/D-05/D-06); a webview-rendered file browser can't see the real filesystem the way an OS-native dialog can, and Wails ships this for free |
| Fixture YAML validation | A second, frontend-side or duplicate Go validation pass | `fixture.Decode`/`fixture.Pin` via the existing `fixture validate`/`fixture inspect` CLI routes | Phase 2 already built and hardened this exact pipeline (FIXT-01/02) — any second implementation risks silent divergence in accepted/rejected fixtures |
| Process supervision/respawn | A hand-rolled watchdog script or Windows service wrapper | `os/exec` self-spawn + `wailsruntime.Quit`, mirroring `app.go`'s existing `defaultSpawn`/`ensureDaemon` daemon-supervision discipline | The exact same pattern already exists one level down (desktop app supervising the artnet daemon); reuse the discipline rather than inventing a second supervision model |
| OFL fixture fetch | A new HTTP client or GitHub SDK | The existing `internal/fixture/ofl.Fetch` (SSRF-guarded `net/http`) | Already hardened (T-02-06 SSRF guard, size/timeout caps, content-addressed cache) — only the *manufacturer index* fetch and *filter* logic is new, the fetch mechanics are not |

**Key insight:** Every "don't hand-roll" item in this phase is really "don't duplicate a pipeline
Phase 2/5/6 already built and hardened" — the actual net-new code surface is narrow: a directory
listing extraction, a manufacturer-index fetch+filter, a process self-respawn, and an entirely
new (but fully spec'd) onboarding overlay component tree.

## Runtime State Inventory

Not applicable — this phase adds new UI surfaces and one new backend capability (fixture
listing) against existing, unrenamed backend routes. No rename/refactor/migration is in scope.
Verified: `git log`/`ROADMAP.md`/`REQUIREMENTS.md` for FDUI-01/02/03 describe additive UI wiring
only, never a rename of any existing route, service, or data key.

## Common Pitfalls

### Pitfall 1: Assuming a Wails app context is available for `runtime.OpenFileDialog`/`Quit` outside `OnStartup`
**What goes wrong:** `wailsruntime.OpenFileDialog(ctx, ...)` and `wailsruntime.Quit(ctx)` both
require the Wails-injected `context.Context` that `OnStartup(ctx context.Context)` receives — but
`App` (`internal/wails/app.go`) currently discards that `ctx` after `OnStartup` returns (it's
only threaded into `ensureDaemon`/`hotkeys.RegisterAll`/`events.Start`, never stored on the struct).
**Why it happens:** No existing service in this codebase has needed to make an ad-hoc Wails
runtime call *after* startup — `events.go`'s `EventsEmit` calls happen inside the already-running
`EventPusher.run(ctx)` goroutine, which captured its own `ctx` at `Start` time.
**How to avoid:** Add a `ctx context.Context` field to `App`, set it at the top of `OnStartup`,
and expose it (or a wrapper method) to whatever new service needs `OpenFileDialog`/`Quit` — e.g.
`App.Context() context.Context` or thread it directly into a new `ShowService.ctx` field set from
`main.go`'s `OnStartup` callback (mirroring how `midiService.StartFeedback(ctx)` already receives
it there).
**Warning signs:** A nil-pointer panic or a silently-no-op dialog call in production, since a Go
Wails app built without a valid runtime context for these calls does not always fail loudly.

### Pitfall 2: Treating "switching shows" as safe to trigger with unsaved changes
**What goes wrong:** `RelaunchWithShow` kills the current process (and its supervised daemon
child) via `Quit`+process exit; any unsaved in-memory show edits (there is no live autosave
tick faster than SHOW-03's own autosave cadence) are lost exactly as they would be on any other
unexpected exit.
**Why it happens:** The UI-SPEC's own edge-coverage table (`09-UI-SPEC.md`, "error" row for
"Show switch — relaunch fails") only covers *relaunch failure*, not *unsaved-changes-before-
relaunch* — this gap is flagged as a UI-SPEC backstop item implicitly (no explicit "confirm
before switching" copy exists yet in the Copywriting Contract).
**How to avoid:** Either call `ShowService.Save()` automatically before triggering
`RelaunchWithShow` (silent, matches SHOW-03's autosave philosophy) or add a confirmation step —
this is a genuine planning decision the plan should make explicit, since D-05..D-07 don't address
it and the UI-SPEC doesn't either.
**Warning signs:** A user reports lost programming work after using "Open Show…"/"New Show…"
mid-session.

### Pitfall 3: Extending the OFL SSRF host allowlist without re-reading the existing guard
**What goes wrong:** `internal/fixture/ofl/fetch.go`'s `defaultOFLHost` is a single string
constant (`"raw.githubusercontent.com"`), and `validateTargetURL` checks `parsed.Hostname() !=
defaultOFLHost`. A careless "just add api.github.com too" edit that widens this to accept any
GitHub subdomain (rather than an explicit second constant checked exactly) reopens exactly the
SSRF surface T-02-06 closed.
**Why it happens:** GitHub's REST API and raw-content-serving domains are genuinely different
hosts (`api.github.com` vs `raw.githubusercontent.com`) — a fixture-catalog-search feature that
needs directory listings (which raw content serving cannot provide) necessarily needs a second
host.
**How to avoid:** If a per-manufacturer fixture listing beyond the existing manufacturer-index
file turns out to be required, add a second explicit constant (e.g. `defaultOFLAPIHost =
"api.github.com"`) and validate against an explicit small allowlist `{defaultOFLHost,
defaultOFLAPIHost}`, never a suffix/wildcard match — and treat this as a discrete, reviewable
security decision at plan time, not an incidental widening.
**Warning signs:** A code reviewer (or `gsd-secure-phase`) flags a widened host check with no
corresponding threat-model update.

### Pitfall 4: Building Guided First Show as a second source of truth
**What goes wrong:** The locked design (`onboarding-readiness-impact.md`) explicitly states "the
guide reads actual domain readiness; it does not own duplicate state" — a naive implementation
that, e.g., tracks its own "stage N complete" boolean independent of whether the underlying
pool/patch/scene actually exists risks the guide showing "complete" for a stage whose real work
was since undone elsewhere in the app (e.g. a pool deleted via Patch & Pools while the guide is
exited).
**Why it happens:** Wizard/stepper UI patterns conventionally track their own linear progress
state; this one explicitly must not.
**How to avoid:** Derive every stage's status (blocker/warning/evidence) from a live read of
`ShowService.Inspect()`/`FixturePatchService.ListPatch()`/`ProgrammingService.ListProgramming()`
on each stage render, never from a separately persisted "onboarding progress" record.
**Warning signs:** A stage shows "done" immediately after mount with zero backend calls having
been made yet in the current session.

## Code Examples

### Extracting the existing local-fixture-directory scan (Pattern 1 source)
```go
// Source: internal/command/artnet.go:325-354 (existing, private — the exact logic to extract
// into an exported internal/fixture function so both the artnet resolver AND the new
// FixtureLibraryService can call one implementation, never two)
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
		// ... Decode + Pin, keyed by StableKey
	}
	return byStableKey, nil
}
```
The extracted `internal/fixture.ListDirectory(dir string) ([]fixture.FixtureDefinition, error)`
(or a slice-returning variant, since the Wails view needs stable ordering for a list UI, not a
map) should preserve this exact decode/pin/skip-non-YAML discipline, and `artnet.go`'s
`newArtnetFixtureResolver` should be updated to call the extracted function instead of its own
private copy.

### Wails v2 runtime dialog + quit signatures (confirmed from vendored source)
```go
// Source: github.com/wailsapp/wails/v2@v2.13.0/pkg/runtime/dialog.go (vendored, confirmed present
// in this repo's module cache)
func OpenDirectoryDialog(ctx context.Context, dialogOptions OpenDialogOptions) (string, error)
func OpenFileDialog(ctx context.Context, dialogOptions OpenDialogOptions) (string, error)
func SaveFileDialog(ctx context.Context, dialogOptions SaveDialogOptions) (string, error)

// Source: github.com/wailsapp/wails/v2@v2.13.0/pkg/runtime/runtime.go
func Quit(ctx context.Context)
```
`OpenFileDialog`'s `OpenDialogOptions` supports a `Filters []FileFilter` field — use it to
restrict the show picker to `*.golc` and the custom-YAML picker to `*.yaml;*.yml`.

### Self-respawn pattern to extend (mirrors the existing daemon-spawn discipline)
```go
// Source: internal/wails/app.go:333-364 (existing defaultSpawn, unmodified) — the pattern
// App.RelaunchWithShow should mirror one level up, spawning os.Executable() (this same
// golc-desktop.exe) instead of the resolved golc-project.exe daemon path:
func defaultSpawn(ctx context.Context, cfg Config) (*exec.Cmd, *daemonStderrBuffer, error) {
	exePath, err := resolveDaemonExecutable(cfg)
	// ... builds args, cmd.Env = append(os.Environ(), "GOLC_PROJECT_ROOT="+cfg.ProjectRoot)
	// ... cmd.Start()
}
```
A new `App.RelaunchWithShow(newShowPath string) error` should: resolve `os.Executable()` (the
running `golc-desktop.exe`'s own path), build `cmd.Env` from `os.Environ()` with
`GOLC_DESKTOP_SHOW` overridden to `newShowPath`, `cmd.Start()` the new process (detached — not
`CommandContext(ctx, ...)`, since `ctx` is about to be cancelled by the current process's own
`Quit`/exit), then call `wailsruntime.Quit(a.ctx)`. `OnShutdown` (already unmodified) then kills
the OLD supervised artnet-daemon child exactly as it does on any normal exit.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Fixture Library workspace is a `ComingSoon` stub pointing at the CLI | Fixture Library workspace is a real browse/inspect/import surface | This phase (FDUI-01) | Closes the last CLI-only gap in the fixture-authoring happy path |
| Show path resolved once at process startup from `GOLC_DESKTOP_SHOW`, no in-app way to change it | Show path changeable via on-screen "Open Show…"/"New Show…", implemented as a self-respawn | This phase (FDUI-02) | Operators no longer need to close the app, set an env var, and relaunch manually |
| No onboarding surface exists at all (`grep -ri "guided\|onboard" frontend/src` = 0 hits, confirmed by `POST-PHASE-8-PLAN.md` and re-confirmed by this research's own grep) | Guided First Show overlay exists, driven by Sketch 004-B's locked design | This phase (FDUI-03) | First-run experience no longer requires reading CLI docs to reach a patched fixture + scene on screen |

**Deprecated/outdated:** None — this phase adds capability, it does not deprecate any existing
route, component, or pattern.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `fixtures/manufacturers.json` at `raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/manufacturers.json` is stable, unauthenticated-fetchable, and contains every manufacturer OFL currently lists (confirmed live via WebFetch this session, but the *shape* — manufacturer id → name/website/rdmId — was read from a single live fetch, not the project's own committed schema docs) [CITED: live fetch this session, not an official OFL API contract document] | Standard Stack / Alternatives Considered, Open Questions | If OFL restructures this file or moves it, D-01's manufacturer-name search silently breaks; low risk (it's a stable, long-lived file in a mature project) but not formally verified against OFL's own versioned schema docs |
| A2 | No `register.json` or equivalent full fixture-name index is committed to the OFL git repo (only generated at OFL's own website build time via `cli/build-register.js`, confirmed by `package.json`'s `build:register` script and a live 404 on the expected path) [VERIFIED: live 404 + package.json script inspection this session] | Alternatives Considered, Common Pitfalls | If a future OFL repo change commits a generated register file after all, D-01's search could be upgraded from manufacturer-name-only to full fixture-name search without needing `api.github.com` — worth re-checking at implementation time |
| A3 | Confirming or denying an unsaved-changes-before-relaunch guard for D-05/D-07 is a genuine open planning decision, not something CONTEXT.md or the UI-SPEC already resolved | Common Pitfalls (Pitfall 2) | If the plan omits this, an operator could silently lose unsaved programming work when switching shows |

## Open Questions (RESOLVED)

1. **RESOLVED — manufacturer-name-only for v1**, per `COVERAGE.md` and `09-05-PLAN.md`'s must_haves
   (the recommended option (a) below; disclosed on-screen, not silently hidden).

   Does D-01's "OFL catalog search" need to search fixture names, or is manufacturer-name-only
   search acceptable for v1?
   - What we know: The UI-SPEC's copy ("Search fixtures by name or manufacturer…") implies both;
     `fixtures/manufacturers.json` only gives manufacturer name/id, not fixture model names. No
     full-catalog fixture-name index is fetchable from the currently-allowed
     `raw.githubusercontent.com` host without either a second allowed host (`api.github.com`,
     rate-limited) or many per-manufacturer raw-file probes (impractical — raw content serving
     has no directory-listing endpoint).
   - What's unclear: Whether the plan should (a) implement manufacturer-name-only OFL search for
     v1 and revisit fixture-name search later, (b) add `api.github.com` as a second allowed SSRF
     host and accept the unauthenticated rate-limit risk, or (c) some other approach (e.g. a
     small curated/cached local mirror of common manufacturers, refreshed occasionally).
   - Recommendation: Default to (a) for this phase — manufacturer-name-only OFL search, clearly
     scoped in the plan — since it satisfies D-01/D-03's "basic text search... not over-building"
     intent without a new SSRF-relevant host decision. Flag (b) as a v1.x follow-up if user
     feedback shows manufacturer-only search is insufficient.

2. **RESOLVED — auto-save before relaunch**, per `09-02-PLAN.md`'s objective (the recommended
   lower-risk default below: a failed save aborts the switch).

   Should `RelaunchWithShow` auto-save before respawning, or require an explicit confirm?
   - What we know: Neither `09-CONTEXT.md` nor `09-UI-SPEC.md` addresses unsaved-changes handling
     for the relaunch flow; `ShowService.Save()` already exists and is cheap to call.
   - What's unclear: Whether silent auto-save-before-switch is the right UX, or whether an
     explicit "Save & Switch" / "Discard & Switch" choice (reusing the existing `destructive`
     Button pattern from `SaveRecoveryWorkspace.tsx`'s "Discard All") is warranted.
   - Recommendation: Plan should decide explicitly; auto-save-before-switch is the lower-risk
     default (never silently loses work) and requires no new UI copy beyond what's already
     locked.

3. **RESOLVED — `PickShowPath`/`RelaunchWithShow` live on `App`**, per `09-02-PLAN.md`'s objective
   (the recommended option below, not `ShowService`).

   Where does `App`'s captured Wails context live for `PickShowPath`/`RelaunchWithShow` to
   call it?
   - What we know: `App.ctx` doesn't exist today; `OnStartup(ctx)` is the only place a valid
     Wails runtime context is currently available in this codebase.
   - What's unclear: Whether the new picker/relaunch methods belong on `App` itself (which already
     owns the daemon-supervision lifecycle) or on `ShowService` (which owns `showPath` today) with
     `ctx` threaded in from `main.go`'s `OnStartup` closure, mirroring how `midiService.
     StartFeedback(ctx)` already receives it there.
   - Recommendation: Put `PickShowPath`/`RelaunchWithShow` on `App` (it already owns
     process-lifecycle concerns — daemon spawn/kill, hotkeys, events) rather than `ShowService`
     (which owns ShowState CRUD, a different concern) — but this is a planner-level structural
     choice, not something this research locks.

## Environment Availability

Not applicable — this phase's dependencies (`wailsapp/wails/v2`, Go toolchain, Node/npm/Vite
toolchain) are already the project's existing, verified-working development environment (Phases
6-8 built and shipped against this identical stack). No new external tool/service/runtime
dependency is introduced.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (Go) | `go test` (stdlib `testing`), existing project-wide convention |
| Framework (frontend) | Vitest ^4.1.10 + `@testing-library/react` ^16.3.2 [VERIFIED: frontend/package.json] |
| Config file | `frontend/vite.config.ts` (Vitest config colocated with Vite config — no separate `vitest.config.ts` found in this repo) |
| Quick run command (Go) | `go test ./internal/fixture/... ./internal/wails/... -run <TestName>` |
| Quick run command (frontend) | `npm --prefix frontend test -- <TestFile>` (Vitest run mode, matches `"test": "vitest run"` script) |
| Full suite command | `go test ./...` and `npm --prefix frontend run build` (which itself runs `tsc --noEmit && vitest run && vite build` per `package.json`) |

### Phase Requirement → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FDUI-01 | Local fixture directory listing decodes/pins every `.yaml`/`.yml` file, skips non-fixture files | unit (Go) | `go test ./internal/fixture/... -run TestListDirectory` | ❌ Wave 0 — new `internal/fixture/directory_test.go` |
| FDUI-01 | `FixtureLibraryService.ListLocal`/`SearchOFL`/`Import` project correctly and degrade gracefully when bridge unavailable | unit (Go + frontend) | `go test ./internal/wails/... -run TestFixtureLibraryService` / `npm --prefix frontend test -- FixtureLibraryWorkspace` | ❌ Wave 0 — new `svc_fixturelibrary_test.go`; existing `FixtureLibraryWorkspace.test.tsx` currently only asserts the `ComingSoon` stub text and must be rewritten |
| FDUI-02 | `App.RelaunchWithShow` spawns a new process with the correct `GOLC_DESKTOP_SHOW` env override, using an injectable spawn func (mirrors `app_test.go`'s existing `spawnFunc` test-double pattern) | unit (Go) | `go test ./internal/wails/... -run TestRelaunchWithShow` | ❌ Wave 0 — extend existing `internal/wails/app_test.go` |
| FDUI-02 | `ShowsWorkspace.tsx` renders current-show path, Open/New actions, and relaunch transient/error copy | unit (frontend) | `npm --prefix frontend test -- ShowsWorkspace` | ❌ Wave 0 — new `ShowsWorkspace.test.tsx` |
| FDUI-03 | Guided First Show auto-launches only when show has zero fixtures/pools/scenes, never on a show with existing content | unit (frontend) | `npm --prefix frontend test -- GuidedFirstShow` | ❌ Wave 0 — new `GuidedFirstShow.test.tsx` |
| FDUI-03 | Guided First Show stage status (blocker/warning/evidence) derives from live backend reads, never a separately persisted progress flag | unit (frontend, mocked bridge) | `npm --prefix frontend test -- GuidedFirstShow` | ❌ Wave 0 — same file, additional cases |

### Sampling Rate
- **Per task commit:** targeted `go test ./internal/fixture/... ./internal/wails/...` and
  `npm --prefix frontend test -- <touched file>`
- **Per wave merge:** `go test ./...` and `npm --prefix frontend run build`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/fixture/directory_test.go` — covers the extracted `ListDirectory` function
- [ ] `internal/wails/svc_fixturelibrary_test.go` — covers `ListLocal`/`SearchOFL`/`Import`
- [ ] `internal/wails/app_test.go` extension — covers `RelaunchWithShow`'s spawn-args and
      `Quit`-call sequencing (via the existing injectable `dialFunc`/`spawnFunc` test-double
      pattern already established for `ensureDaemon`)
- [ ] `frontend/src/workspaces/show/ShowsWorkspace.test.tsx` — new file
- [ ] `frontend/src/workspaces/show/GuidedFirstShow/GuidedFirstShow.test.tsx` — new file
- [ ] `frontend/src/workspaces/build/FixtureLibraryWorkspace.test.tsx` — must be rewritten (current
      version only asserts the `ComingSoon` stub's static text and will fail/be meaningless once
      the real workspace lands)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | Desktop-local app, no new auth surface introduced |
| V3 Session Management | no | N/A |
| V4 Access Control | no | No new privilege boundary — same single-operator desktop trust model as Phases 6-8 |
| V5 Input Validation | yes | Fixture YAML/OFL JSON validated exclusively via the existing `fixture.Decode`/`ofl.Normalize` pipeline (Phase 2, unchanged); a hand-typed show path from `OpenFileDialog`/`SaveFileDialog` is resolved via the existing `resolveWritablePath`-style root-relative resolution already used elsewhere in `internal/command`, never a raw string interpolated into a shell command |
| V6 Cryptography | no | No new cryptographic operation — existing SHA-256 content-addressing (fixture pinning, OFL cache) is unchanged and reused |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| SSRF via a widened OFL host allowlist (Pitfall 3) | Tampering / Information Disclosure | Keep the existing `defaultOFLHost` single-string-equality check; if a second host is ever added, use an explicit small allowlist, never a suffix/wildcard match — mirrors the existing T-02-06 mitigation exactly |
| Process-respawn argument injection (a malicious/malformed show path passed into `GOLC_DESKTOP_SHOW`) | Tampering | The new child process env var is set via `cmd.Env = append(os.Environ(), "GOLC_DESKTOP_SHOW="+newPath)` — an env var assignment, never a shell-interpreted string; `os/exec.Command` does not invoke a shell, so no shell-metacharacter injection risk exists (mirrors `defaultSpawn`'s existing safe pattern) |
| Orphaned respawned process on relaunch failure | Denial of Service (self) | If the new process's `cmd.Start()` fails, `RelaunchWithShow` must return an error WITHOUT calling `Quit` — never orphan the user in a state with no running instance; the UI-SPEC's own "Couldn't switch to this show. GOLC is still running the previous show." copy already assumes this ordering |
| Unbounded/leaked file-descriptor or handle from the spawned self-respawn child if `Quit` never actually terminates the parent (e.g. blocked on an in-flight save) | Denial of Service | Mirror `app.go`'s `OnShutdown`'s existing bounded kill (`cmd.Process.Kill()` + `cmd.Wait()`) discipline for the OLD process's own daemon child; `wailsruntime.Quit` triggers the same `OnShutdown` path already proven in production for Phases 6-8 |

## Sources

### Primary (HIGH confidence)
- `go.mod`, `frontend/package.json` — dependency versions [VERIFIED: direct file read]
- `internal/command/fixture.go`, `internal/wails/svc_fixturepatch.go`, `internal/wails/svc_show.go`,
  `internal/wails/app.go`, `cmd/golc-desktop/main.go`, `internal/fixture/ofl/fetch.go`,
  `internal/show/store.go`, `internal/command/artnet.go` — read in full this session
- `frontend/src/lib/wailsBridge.ts`, `frontend/src/shell/navigation.ts`,
  `frontend/src/shell/WorkspaceRouter.tsx`, `frontend/src/workspaces/show/SaveRecoveryWorkspace.tsx`,
  `frontend/src/workspaces/build/ScriptsWorkspace.tsx`,
  `frontend/src/workspaces/show/SettingsWorkspace.tsx` — read in full this session
- `.planning/phases/09-front-door-ui-completion/09-CONTEXT.md`,
  `.planning/phases/09-front-door-ui-completion/09-UI-SPEC.md`,
  `.planning/sketches/references/onboarding-readiness-impact.md`,
  `.planning/sketches/references/application-shell-navigation.md` — read in full this session
- `github.com/wailsapp/wails/v2@v2.13.0/pkg/runtime/dialog.go`, `.../runtime.go` — vendored
  source confirmed via local module cache [VERIFIED: local Go module cache read]

### Secondary (MEDIUM confidence)
- `https://raw.githubusercontent.com/OpenLightingProject/open-fixture-library/master/fixtures/manufacturers.json`
  — live-fetched this session to confirm shape and reachability [CITED: live fetch, not an
  official versioned API contract]
- `https://github.com/OpenLightingProject/open-fixture-library/blob/master/package.json` — live
  fetch confirming `build:register` is a build-time-only artifact, not a repo-committed file
  [CITED: live fetch]

### Tertiary (LOW confidence)
- None — every OFL-related claim in this document was independently verified via a live fetch or
  an explicit 404 this session, not left as unverified training-data recall.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every version pin was read directly from `go.mod`/`package.json`
- Architecture (backend mechanism): HIGH — grounded in full reads of every relevant existing file
  (`app.go`, `main.go`, `svc_show.go`, `svc_fixturepatch.go`, `fetch.go`, `store.go`); the
  self-respawn recommendation directly mirrors an existing, production-proven pattern in this
  same codebase
- Architecture (OFL catalog search): MEDIUM — the upstream OFL repo structure was verified live
  this session (manufacturer index confirmed fetchable, full fixture-name index confirmed NOT
  committed to the repo), but OFL's own roadmap/future repo structure is out of this project's
  control
- Pitfalls: HIGH — every pitfall traces to a specific, named gap in the actually-read codebase
  (missing `ctx` storage, missing unsaved-changes guard, existing single-string SSRF host check,
  locked "no duplicate state" design contract)

**Research date:** 2026-07-27
**Valid until:** 30 days for the internal-codebase findings (stable, slow-moving); 14 days for
the OFL-repo-structure findings (an external project this repo doesn't control — re-verify
`fixtures/manufacturers.json`'s reachability/shape if implementation is delayed past that window)
