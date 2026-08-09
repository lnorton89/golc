# GOLC desktop frontend

This is the React/TypeScript UI for **golc-desktop**, the Wails-hosted operator console for GOLC (a Go-based Art-Net lighting control system). It renders inside a native WebView2 window on Windows, embedded directly into the `golc-desktop` binary at build time (`cmd/golc-desktop/main.go`'s `//go:embed all:frontend/dist`) — there is no separate frontend server or deployment target in production.

> **Before adding or migrating any UI**, read [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md). It is the authoritative component inventory, token vocabulary, selection guide, and mechanically-enforced style contract for this codebase. Nothing in this README repeats what's there.

## What this app is

GOLC operators use this console to patch fixtures, program scenes and looks, map MIDI controllers, run the live playback desk, and monitor Art-Net output — all while a Go daemon owns the actual deterministic timing, safety state, and Art-Net transmission. The frontend's job is to **project and control** that Go-owned state; it never becomes the source of truth for anything safety- or timing-critical (see [Architecture](#architecture) below).

The interface is a dense, keyboard-and-mouse operator console styled as **"Paper/Ink"** — a purposeful, information-dense look distinct from a typical consumer web app. See [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md) for the full visual language.

## Tech stack

| Layer | Choice |
|---|---|
| UI framework | React 19 + TypeScript (strict), no framework router — a fixed shell owns navigation |
| Build tool | Vite 8 |
| State | Zustand (thin cache of Go-pushed snapshots — see [Architecture](#architecture)) |
| Styling | Plain CSS Modules + a generated semantic token system (no CSS-in-JS, no utility framework) |
| Desktop host | [Wails](https://wails.io) v2 — binds Go methods onto `window.go.wails.*` and pushes events via `window.runtime.EventsOn` |
| Rich text / code | Tiptap (Notes), Monaco (Scripts editor) |
| Icons | lucide-react |
| Fonts | Archivo + JetBrains Mono, self-hosted via `@fontsource` (no runtime Google Fonts request — this app is expected to run on isolated/offline show networks) |
| Unit/component tests | Vitest + Testing Library + jsdom |
| Browser/E2E tests | Playwright (Chromium) — real layout engine coverage jsdom can't provide |

## Project structure

```
frontend/
├── src/
│   ├── main.tsx              # Entry point: fonts, stored theme, React root
│   ├── App.tsx                # Top-level app shell wiring
│   ├── shell/                 # Persistent chrome: rail, title bar, inspector, overlays, routing
│   ├── workspaces/             # One folder per top-level destination, grouped by rail section
│   │   ├── show/               #   Show:    Overview, Shows, Save/Recovery, Settings, Notes, Guided First Show
│   │   ├── build/               #   Build:   Fixture Library, Patch & Pools, Project Fixtures, Scenes & Looks, Scripts
│   │   ├── operate/              #   Operate: Operator Surface, MIDI Mapping
│   │   ├── perform/               #   Perform: Desk (live playback)
│   │   └── output/                 #   Output:  Art-Net, Diagnostics
│   ├── components/             # Feature-specific components used by one or a few workspaces
│   │   └── primitives/          # Shared, typed UI primitives (Button, Field, Dialog, ...) — see DESIGN_SYSTEM.md
│   ├── design-system/          # Generated tokens, semantic patterns, the public design-system barrel export
│   ├── store/                  # Zustand store — cache of Go-pushed snapshots, never authoritative
│   ├── lib/                    # wailsBridge (the only two touchpoints with window.go/window.runtime), theme, hotkeys
│   ├── hooks/                  # Reusable React hooks (keyboard workflow, resizable panels, playback snapshot)
│   └── test/                   # Vitest setup
├── e2e/                        # Playwright specs — real-browser layout, resize, dialog, and design-system suites
├── design-system/              # Non-source design-system data: tokens.json, components.json, exceptions.json, schema/
├── scripts/design-system/      # The design-system generator + enforcement checker (see DESIGN_SYSTEM.md)
├── vite.config.ts              # Also carries the Vitest config (shares the real Vite module graph)
└── playwright.config.ts
```

## Architecture

### React is a projection, never the source of truth

The single most important architectural rule in this codebase: **all playback, safety, Art-Net, and show state lives in the Go daemon.** The frontend never computes or holds that state as authoritative — it only renders a projection of it and dispatches commands back through Wails. Concretely:

- `src/lib/wailsBridge.ts` is the **only** place that touches the Wails-injected globals (`window.go.wails.<Service>.*` bound methods, `window.runtime.EventsOn` push subscriptions). Every other file that needs a Go-bound call or a live event goes through this module — never through ambient globals directly.
- `src/store/store.ts` is a Zustand cache of what the Go host last pushed via `runtime.EventsEmit`. No store action mutates application state without a corresponding Go-bound call; the store is read-and-render only.
- Every export in `wailsBridge.ts` degrades gracefully (never throws) when `window.go`/`window.runtime` are undefined — e.g. during `tsc --noEmit`, a plain browser preview, or a component test with no real WebView2 host. A missing bridge resolves to an explicit "unreachable/offline" result, never a crash — this matters most for the safety cluster, which must stay visibly present and honest about its own reachability even when disconnected.

### Shell, navigation, and workspaces

`src/shell/` owns the persistent chrome around whatever workspace is active: the command rail, title bar, contextual inspector, log stream, and global overlays (help, quick switcher, error boundary). `src/shell/navigation.ts` is the single source of truth for the rail's grouped information architecture (Show / Build / Operate / Perform / Output) — both `CommandRail.tsx` and `WorkspaceRouter.tsx` read from the same `NavDestination` catalog, so adding a destination means one catalog entry plus one `WorkspaceRouter` case, never two independently-maintained lists.

Each `src/workspaces/<group>/<Name>Workspace.tsx` is a top-level, routed screen. Workspaces own their own domain state and Wails calls; they should never reimplement chrome, dialogs, empty/loading/error states, or spacing that already exists as a design-system primitive or pattern (see [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md)).

### Theming

Two independent axes: **light/dark mode** and **palette (theme name)** — 12 named palettes × 2 modes = 24 selectable theme faces, all built on the same Paper/Ink semantic structure (`design-system/tokens.json`: `semanticRoles` → `foundation` → `palettes` → `themes`). `src/lib/theme.ts` persists both selections to `localStorage` and applies them via `data-theme`/`data-theme-name` attributes on `<html>`, applied **before** the first React render in `main.tsx` so there's no flash of the wrong theme on launch. Component code should never branch on a theme name or mode directly — that's exactly what the semantic `--ds-*` tokens exist to make unnecessary, and it's mechanically forbidden by the design-system checker (see DS004 in [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md)).

## Development

```bash
npm install
npm run dev        # Vite dev server
```

The app can run **outside** a real WebView2 host — in a plain browser, or under Playwright — because every Wails call degrades gracefully to an "unreachable" result when the bridge globals are absent. For interactive browser-based development or debugging against real component code (not just a static mock), inject `window.go.wails.<Service>` methods directly in a browser console/preview tab rather than driving the native app window — this avoids the risk of a blind coordinate click silently landing on a different, unrelated native window.

## Testing

Three distinct layers, each catching a different class of bug:

| Layer | Command | What it catches |
|---|---|---|
| Unit/component (Vitest + jsdom) | `npm test` | Logic, rendering, component contracts — fast, no real layout engine |
| Real-browser E2E (Playwright) | `npm run test:e2e` | Actual on-screen geometry, resize/overflow, dialogs, keyboard workflows — jsdom's `getBoundingClientRect` is always zeroed, so layout bugs are invisible to it |
| Design-system enforcement | `npm run check:design-system` + `npm run test:e2e:design-system` | Token/primitive drift, theme parity, accessibility invariants, and pixel-calibrated visual regressions — see [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md) |

```bash
npm test                          # Vitest, all unit/component tests
npm run test:e2e                  # Full Playwright suite (real Chromium)
npm run test:e2e:resize           # Just the aggressive window-resize/overflow suite
npm run test:e2e:design-system    # Just the design-system-scoped Playwright project
npm run check:design-system       # Static design-system policy checker (see DESIGN_SYSTEM.md)
npm run build                     # tsc --noEmit + vitest run (concurrently), then vite build — the full local gate
```

Playwright specs are deliberately **outside** `npm test`/`npm run build` and outside the Go-side pinned-toolchain build pipeline (`mage Build`) — those need to stay fast and network-free for every Go build, while a real-browser suite needs its own downloaded Chromium binary and takes real wall-clock time per spec. Run the E2E suites explicitly when touching layout, dialogs, resize behavior, or anything design-system-scoped.

A known, harmless flake: `scripts/design-system/manifest.test.ts`'s "is byte-stable..." test can occasionally time out under full-suite parallel load (5000ms budget) but passes reliably in isolation (`npx vitest run scripts/design-system/manifest.test.ts`) — this is a scheduling artifact of the full run, not a real failure.

## Build

```bash
npm run build
```

This runs `tsc --noEmit`, the full Vitest suite, and `vite build` in sequence — any failure at any stage fails the whole command. Output is redirected to `../cmd/golc-desktop/frontend/dist` (not `frontend/dist`) so Go's `//go:embed` directive, which cannot reference a path outside its own package tree, can see it. This build also runs automatically as part of `mage Build` at the repository root.

## Contributing conventions

- **Never** introduce a raw color, spacing value, or ad-hoc CSS custom property — use the semantic `--ds-*` tokens (see [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md)). This is mechanically enforced by `npm run check:design-system`, not just a style guideline.
- **Never** style a native `<button>`/`<input>`/`<select>`/`<textarea>` directly, and never reinvent a shared primitive's own chrome in feature code — use the primitive.
- **Never** treat a Wails-pushed event or the Zustand store as authoritative — dispatch a Go-bound call for anything that changes real state.
- Prefer a design-system primitive or pattern over a new bespoke component; if a genuinely new reusable need exists, add it to the inventory (`design-system/components.json`) and its own guide entry, not just local component code.
- A necessary, narrow exception to a design-system rule (a real geometry constraint, a third-party vendor requirement) must be an exact, individually-reasoned record in `design-system/exceptions.json` — never a blanket suppression, and never a spacing exception (padding/margin/gap can never be excepted; the 4px grid is non-negotiable).
