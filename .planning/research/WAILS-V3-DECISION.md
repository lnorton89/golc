# Wails v2 → v3 Decision Gate

**Domain:** Desktop shell framework version selection for the v1 Windows release
**Project:** GOLC
**Researched:** 2026-08-09
**Confidence:** HIGH (upstream release metadata and migration guide read directly; migration surface measured against this repository, not estimated)
**Blocks:** Phase 11 (Windows Release Qualification) planning
**Current pin:** `github.com/wailsapp/wails/v2 v2.13.0` (`go.mod`), CLI pinned via `config/toolchain.toml` → `internal/bootstrap.WailsModule`

## Executive Recommendation

**Stay on Wails v2.13.0 through the v1 release. Do not migrate before or during Phase 11.**

Wails v3 was promoted from alpha to beta on **2026-08-02 — seven days ago** — and is at `v3.0.0-beta.6` as of today (2026-08-09), with beta releases shipping every one to two days. Upstream's own beta.0 release notes say, verbatim: *"Wails v2 remains the current stable release"*, *"test thoroughly before using it in a critical production deployment"*, and *"keep your v2 application in place until the new one is ready."* GitHub still marks **v2.13.0 as `Latest`**.

v2 is not abandoned: v2.11.0 (2025-11-08), v2.12.0 (2026-03-26), v2.13.0 (2026-07-06). GOLC is already on the newest v2, released one month ago.

Decisively for *this* project: **for a Windows-only v1, v3 buys almost nothing.** Its three headline features — webkit2gtk-4.1, multi-window, and richer generated bindings — map onto a platform GOLC explicitly excludes from v1, a capability GOLC does not use, and a mechanism GOLC's frontend does not consume (see *What v3 would buy* below).

The actionable output of this research is therefore not a migration; it is **one cheap piece of insurance to take now** (§ *Do this now*) plus **an explicit re-evaluation gate** (§ *The gate*).

## Correction to a prior claim

An earlier note in this project's discussion held that Wails v3 "shipped stable in March 2026." **That is wrong.** No stable v3 release exists. The complete v3 release history is pre-release only, and the alpha→beta promotion happened on 2026-08-02. The claim came from a secondary summary that conflated *"the desktop API is stable"* (a statement about API churn within the beta) with *"there is a stable release."* Primary sources are cited below; treat the earlier statement as retracted.

## Evidence

| Fact | Source | Value |
|---|---|---|
| Latest v3 | `gh release list --repo wailsapp/wails` | `v3.0.0-beta.6`, 2026-08-09, **prerelease: true** |
| Alpha → beta promotion | release `v3.0.0-beta.0` | 2026-08-02 (7 days ago) |
| Beta cadence | beta.0 → beta.6 | 7 releases in 7 days |
| v3 module status | pkg.go.dev `wailsapp/wails/v3` | pre-v1, flagged unstable |
| v2 status | GitHub releases | v2.13.0 tagged **`Latest`**, 2026-07-06 |
| Upstream migration advice | beta.0 release notes | "keep your v2 application in place until the new one is ready" |
| Supported migration path | `docs/src/content/docs/migration/v2-to-v3.mdx` | manual, 729 lines, no codemod |

## Measured migration surface

This is counted in this repository, not estimated from the guide.

### Go side — 3 files, smaller than expected

Exactly **three** Go files import the Wails API. (Four further files mention `wailsapp/wails` as a *string* — the CLI module pin in `internal/bootstrap/cache.go`, its assertion in `bootstrap_test.go`, a comment in `internal/command/test.go`, and a test URL in `app_test.go` — none of which are code coupling.)

| File | Coupling | v3 change |
|---|---|---|
| `cmd/golc-desktop/main.go` | `wails.Run` + `options.App` + `assetserver` | Split into `application.New` → window creation → `app.Run()`. The bound-services list maps onto v3 services. |
| `internal/wails/app.go` | 8 `wailsruntime.*` call sites | `runtime.*(ctx, …)` → methods on the app/window object |
| `internal/wails/events.go` | 1 `wailsruntime.EventsEmit` | `app.Event.Emit(name, data)` — context argument disappears |

The **entire** runtime surface is six distinct APIs: `OpenFileDialog`, `SaveFileDialog`, `Quit`, `WindowIsMinimised`, `EventsEmit`, `BrowserOpenURL`. Every one has a documented 1:1 v3 equivalent in the migration guide's Feature Mapping section (e.g. `runtime.OpenFileDialog(ctx, opts)` → `app.Dialog.OpenFileWithOptions(&opts).PromptForSingleSelection()`).

The context-threading change is the only structural one: v2 passes `ctx` into every runtime call, v3 hangs them off the app object. `internal/wails/app.go` already stores its own startup context, so this is mechanical.

### Frontend side — already insulated, by accident of good design

**Zero files under `frontend/src` import the generated `wailsjs/go` bindings.** All 30 generated files are dead weight today. Every Go call instead goes through `window.go.wails.<Service>.<Method>` and `window.runtime.*`, funnelled through one hand-written adapter:

- `frontend/src/lib/wailsBridge.ts` — 2,526 lines, the single intended seam
- **10 non-test files bypass it** and touch `window.go` / `window.runtime` directly:
  `Desk.tsx`, `DeskMappingsSection.tsx`, `MidiLearn.tsx`, `MidiPanel.tsx`, `OperatorSurface.tsx`, `playbackDispatch.ts`, `ScriptsWorkspace.tsx`, `GuidedFirstShow/stages/AssignStage.tsx`, `GuidedFirstShow/stages/VerifyStage.tsx`, `NotesWorkspace.tsx`

v3 removes the `window.runtime` / `window.go` globals in favour of `import { Events, Window } from '@wailsio/runtime'` and per-service generated modules under `./bindings`. So the frontend migration cost is *exactly* the size of the un-insulated set: one file if the bridge is airtight, eleven if it is not.

This also means the migration guide's single largest section — regenerating and re-importing bindings — is nearly free here, because nothing imports them.

### Build system — the real friction

v3 ships a **Taskfile-based build system** and a `wails3 setup` / `wails3 init` flow. GOLC's stated rule is: *"The sole supported entrypoint is Mage… No ecosystem tool — `npm` or anything else — is invoked directly."* Adopting v3's toolchain as-is would introduce a second build authority; keeping Mage means owning the equivalent of `wails3 build` / `wails3 generate bindings` inside `magefiles/`. `wails.json`'s schema also changes (flat `frontend:*` keys → a nested `frontend` object).

This — not the Go API — is the part of a v3 migration that would take real design work in this repository.

## What v3 would buy GOLC

| v3 feature | Value to GOLC v1 | Why |
|---|---|---|
| webkit2gtk-4.1 | **None for v1** | Fixes the exact Linux dead end the README documents at length (wailsapp/wails#4661). But the ROADMAP puts Linux outside v1 scope and Phase 11 qualifies Windows only. Real value, wrong milestone. |
| Multi-window | **None** | GOLC is a single-window operator surface. Not in any v1 requirement. |
| Richer generated TS bindings (static analysis, preserved comments) | **~None** | The frontend imports zero generated bindings; it uses a hand-written typed adapter instead. |
| Typed events (`app.Event.Emit`) | **Marginal** | One emit site. |
| Server builds, iOS/Android | **None** | Explicitly out of v1 scope. |

The honest summary: on Windows, WebView2 is the renderer under both v2 and v3, so the thing Phase 11 actually qualifies is unchanged by the version choice.

## Risks of staying on v2

- **v2 goes maintenance-only after v3 stabilises.** Mitigated by: v2.13.0 is one month old, and GOLC's Windows dependency (WebView2) is an OS component, not a Wails-vendored one.
- **The Linux build stays broken.** Already accepted, already documented in the README, already outside v1.
- **Migration cost grows with the UI.** This is the real risk, and it is exactly what § *Do this now* addresses.

## Do this now (cheap insurance, not a migration)

**Close the 10 bridge leaks.** Route `Desk.tsx`, `DeskMappingsSection.tsx`, `MidiLearn.tsx`, `MidiPanel.tsx`, `OperatorSurface.tsx`, `playbackDispatch.ts`, `ScriptsWorkspace.tsx`, `AssignStage.tsx`, `VerifyStage.tsx`, and `NotesWorkspace.tsx` through `wailsBridge.ts` instead of reading `window.go` / `window.runtime` directly.

This is worth doing **whether or not v3 ever happens**: it is the architecture the bridge file's own header comment already claims (*"rather than referencing window.go/window.runtime directly, so every future…"*), it makes the mock-bridge browser-preview workflow uniform, and it collapses a future v3 frontend migration from eleven files to one. It is ordinary insulation work, sized in hours, with no dependency on any upstream decision.

Optionally also: delete or stop generating `frontend/wailsjs` if nothing consumes it, so it cannot silently become a dependency before the gate.

## The gate

Re-evaluate when **all** of these hold:

1. A **stable, non-prerelease** `v3.x.y` is tagged and marked `Latest` on GitHub (pkg.go.dev no longer flags the module as unstable).
2. That release has been out long enough for a patch release to have landed — i.e. it is `v3.0.1`+ or equivalent, not day-zero `v3.0.0`.
3. The v3 Windows packaging story (NSIS installer, WebView2 bootstrapper strategy, SignTool + timestamping) is demonstrated end-to-end, since that is precisely what Phase 11 must qualify.
4. A `magefiles/` path exists that drives v3 builds and binding generation **without** adopting Taskfile as a second build authority — or the Mage-only rule is consciously revised.

If the gate opens **before** Phase 11 planning starts, reconsider then. If it opens **after** v1 ships, migrate in a dedicated phase, not inside a release-qualification phase — a shell-framework swap and a "measured evidence on clean Windows machines" phase must never be in flight together.

**Default if the gate never opens:** ship v1 on v2.13.0. Nothing in Phases 10–12 requires v3.

## Sources

- `gh release list --repo wailsapp/wails` and `gh release view v3.0.0-beta.0` (release metadata and notes, read 2026-08-09)
- [wailsapp/wails releases](https://github.com/wailsapp/wails/releases)
- [pkg.go.dev — wailsapp/wails/v3](https://pkg.go.dev/github.com/wailsapp/wails/v3)
- `docs/src/content/docs/migration/v2-to-v3.mdx` in wailsapp/wails (the supported migration path), via `gh api`
- [Migrating from v2 to v3](https://v3.wails.io/migration/v2-to-v3/)
- [wailsapp/wails#3345 — WebKit2-GTK 4.1 support](https://github.com/wailsapp/wails/issues/3345), [#4661](https://github.com/wailsapp/wails/issues/4661)
- This repository: `go.mod`, `cmd/golc-desktop/{main.go,wails.json}`, `internal/wails/{app.go,events.go}`, `frontend/src/lib/wailsBridge.ts`, `internal/bootstrap/cache.go`
