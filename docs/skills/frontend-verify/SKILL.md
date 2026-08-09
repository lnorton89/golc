---
name: golc-desktop-frontend-verify
description: How to verify golc-desktop frontend changes — mock-bridge browser preview instead of native window automation, the Vitest runtime-error gate, and the missing-box-sizing-reset gotcha. Load before or while verifying any frontend/src change.
---

<context>
## Why this exists

`golc-desktop` is a Wails app: in production, `window.go.wails.<Service>`
methods are injected by the native runtime. That bridge does not exist in a
plain browser tab or in jsdom, so verifying UI changes needs one of two
approaches — never blind native-window coordinate automation, which risks
focus silently landing on a different live window on this machine and
driving the wrong app.
</context>

<mock_bridge_browser_verification>
## Verifying interactively: mock-bridge browser preview

Start the frontend dev server (`.claude/launch.json` — `golc-desktop-frontend-dev`,
port 4788; ports 4789-4791 are spare instances for when 4788 is already
running) and open it as a plain browser preview tab, then inject the same
mock bridge the app's own e2e suite uses before the app reads it. The
canonical shape is `installHealthyBindings` in
[`frontend/e2e/helpers.ts`](../../../frontend/e2e/helpers.ts) — every
`SafetyService`/`PlaybackService`/`ShowService`/etc. method the components
under test call, each returning a plausible success response. Real usage
sites of the bridge live in `frontend/src/lib/wailsBridge.ts` and the
`*.test.tsx` files under `frontend/src/workspaces/**` — check there for the
exact method signatures a given component expects before injecting a mock
for it.

```js
window.go = { wails: { SafetyService: { FetchStatus: async () => ({ reachable: true, /* ... */ }) }, /* ... */ } };
window.runtime = { EventsOn: () => () => {} };
```

Inject via the browser preview's `javascript_tool` (or a Playwright
`page.addInitScript`) *before* navigating/reloading, since components read
`window.go` at mount. This drives real component code end-to-end without
touching a native window at all.

Every component also has a documented "bridge unavailable" degraded-render
path (see `wailsBridge.ts`) for when `window.go` is absent entirely — worth
checking too, since that's the path a bare `npm run dev` browser tab
exercises by default with no injection at all.
</mock_bridge_browser_verification>

<build_gate>
## The build gate: App.smoke.test.tsx

`npm run build` (`frontend/package.json`) runs `node scripts/build.mjs`,
which runs `tsc --noEmit` and `vitest run` concurrently (they don't depend
on each other), then `vite build` once both pass. The vitest step
runs [`frontend/src/App.smoke.test.tsx`](../../../frontend/src/App.smoke.test.tsx),
which mounts the real `<App/>` tree in jsdom and fails the build if
anything throws or logs a `console.error`/`console.warn` during import or
render — this exists because `tsc --noEmit` and `vite build` alone only
catch type/syntax errors, never actually *execute* the app, so a
runtime-only bug (e.g. a circular ES module import) can compile and bundle
cleanly and then crash the instant a real webview loads it.

Practical implication: if you add a new component or change import order
and only run `tsc --noEmit` or eyeball the diff, you have not verified the
change — run `npm test` (or the full `npm run build`) to get this gate.
</build_gate>

<box_sizing_gotcha>
## No global box-sizing reset

`golc-desktop`'s frontend has no global `box-sizing: border-box` reset.
Setting `width` together with `padding` or `border` on the same rule
without an explicit `box-sizing` will silently overflow its container —
this has caused real layout bugs. When writing or editing a `.module.css`
rule that combines width with padding/border, set `box-sizing: border-box`
explicitly on that rule; don't assume a global reset has you covered.
</box_sizing_gotcha>
