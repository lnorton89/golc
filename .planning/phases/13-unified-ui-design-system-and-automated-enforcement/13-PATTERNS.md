# Phase 13: Unified UI Design System and Automated Enforcement - Pattern Map

**Mapped:** 2026-08-02
**Files analyzed:** 12 implementation slices covering the likely new/modified files
**Analogs found:** 10 / 12 slices

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `frontend/design-system/{tokens,components,runtime-geometry,exceptions}.json` and `frontend/design-system/schema/**` | config / pure-data registry | transform / batch validation | `internal/projectconfig/registry.go`; `internal/projectconfig/strict_test.go` | role + authority match |
| `frontend/scripts/design-system/{check,generate,css-policy,tsx-policy}.mjs` and `fixtures/**` | utility / validator / generator / test fixtures | file-I/O + transform + batch | `internal/contracts/generate_test.go`; `internal/projectconfig/strict_test.go` | data-flow match |
| `frontend/src/design-system/tokens.generated.{css,ts}` | generated config | transform / file-I/O | `internal/scriptsdk/generated/*`; `internal/scriptsdk/generate_test.go` | exact data-flow match |
| `frontend/src/design-system/index.ts` and `frontend/src/design-system/fixtures/DesignSystemGallery.tsx` | config barrel + component fixture | transform / render projection | `frontend/src/shell/desktopViews.json`; `frontend/e2e/helpers.ts` catalog consumption | role-match |
| Existing `frontend/src/components/primitives/**`; new `IconButton`, `Tabs`, `LoadingState`, `ErrorState`, `Dialog`, `ConfirmDialog` | component + test | request-response / event-driven UI | `primitives/Button`, `EmptyState`, `ConfirmModal`, `InfoTooltip` | exact role match |
| New `frontend/src/design-system/patterns/{WorkspaceFrame,SplitPane,DataList,FormActions,ImpactReview}/**` | component composition / hook | event-driven UI + projection | `workspace.module.css`; `FixtureLibraryWorkspace.tsx`; `useResizablePanel.ts` | role + flow match |
| `frontend/src/index.css`, `frontend/src/lib/theme.ts`, `frontend/src/lib/theme.test.ts` | config / utility / test | transform + event-driven projection | current files themselves | migration-in-place |
| `frontend/src/shell/**`, `components/{SafetyCluster,TempoControls,LiveStatusBar}/**` | component / provider | event-driven projection | existing shell and safety components; `frontend/e2e/helpers.ts` safety assertion | migration-in-place |
| `frontend/src/workspaces/**` and colocated domain components/CSS Modules | component / workspace | request-response + event-driven projection | `FixtureLibraryWorkspace.tsx`; `workspace.module.css` | exact role + flow match |
| `frontend/e2e/design-system.{visual,a11y}.spec.ts`, deterministic fixture helpers/data, `screenshot.css`, snapshots, `playwright.config.ts` | browser test / config | request-response + deterministic screenshot/geometry | `frontend/e2e/responsive.spec.ts`; `frontend/e2e/helpers.ts`; `site/tests/visual.spec.ts` | exact role match |
| `frontend/package.json`, `package-lock.json`, `internal/command/*`, `config/commands.toml`, `magefiles/magefile.go`, `.github/workflows/check.yml` or a dedicated workflow | config / command / CI | batch / subprocess | existing build route, Mage target registry, and Windows PR workflow | exact integration match |
| `frontend/DESIGN_SYSTEM.md`, frontend README/project-instruction link, generated inventory markers | documentation / generated documentation | transform / file-I/O | `docs/reference/README.md`; `internal/docgen/**` | role-match |

## Pattern Assignments

### Pure-data manifests and strict validation

**Applies to:** `frontend/design-system/*.json`, `frontend/design-system/schema/**`, and the manifest-loading portion of `frontend/scripts/design-system/check.mjs`.

**Primary analog:** `internal/projectconfig/registry.go`

**One-owner registry pattern** (`internal/projectconfig/registry.go:47-62`):

```go
type Registry struct {
	Fields map[string]FieldSpec
}

func (r Registry) field(key string) (FieldSpec, error) {
	spec, known := r.Fields[key]
	if !known {
		return FieldSpec{}, fmt.Errorf(
			"GOLC_CONFIG_FIELD_UNKNOWN: %q is not declared by the registry",
			key,
		)
	}
	return spec, nil
}
```

Copy the contract, not the language: each public token, component/pattern, runtime geometry variable, and exception must have exactly one manifest owner. Unknown names fail with stable rule IDs; they are never ignored or inferred.

**Closed-set and immutable-registry pattern** (`internal/projectconfig/registry.go:72-99`):

```go
func DefaultRegistry() Registry {
	fields := map[string]FieldSpec{
		"runtime.log_level": {
			Locked:        false,
			AllowedValues: []string{"debug", "error", "info", "warn"},
			EnvVar:        "GOLC_RUNTIME_LOG_LEVEL",
			CLIFlag:       "--log-level",
		},
	}
	for _, concern := range DefaultSpec().Concerns {
		for key := range concern.Keys {
			if _, declared := fields[key]; declared {
				continue
			}
			fields[key] = FieldSpec{Locked: true}
		}
	}
	return Registry{Fields: fields}
}
```

The JS loader should build normalized maps from parsed JSON, reject duplicate logical identifiers across manifests, validate closed enums and required properties, and return defensive/sorted snapshots. Do not execute manifests, dynamically import paths from them, or accept extra keys.

**Synthetic strict-fixture pattern** (`internal/projectconfig/strict_test.go:49-68`, `71-108`):

```go
func writeStrictRepository(t *testing.T, spec projectconfig.Spec, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	// Build the smallest complete repository fixture.
	// Write each case beneath the owned temp root only.
	return root
}

func syntheticSpec() projectconfig.Spec {
	return projectconfig.Spec{
		Concerns: []projectconfig.ConcernSpec{ /* minimal closed registry */ },
		Deprecations: []projectconfig.Deprecation{ /* explicit migration case */ },
	}
}
```

Mirror this under `frontend/scripts/design-system/fixtures/`: one minimal allowed and one forbidden fixture per DS001-DS010 rule. Cases should cover unknown fields, duplicate public names, invalid enums/types/ranges, absolute and `..` paths, out-of-root symlinks, stale exceptions, zero-match exceptions, multi-match exceptions, malformed CSS, and malformed TSX. All failures need stable, sorted `DSxxx path:line:column` diagnostics.

**Validation assertions** (`internal/projectconfig/strict_test.go:173-191`):

```go
values, warnings, err := projectconfig.ValidateRepository(root, spec)
require.NoError(t, err, "ValidateRepository failed")
require.Empty(t, warnings, "expected no production warnings")
require.NotEmpty(t, values["runtime.log_level"])
require.Contains(t, string(commandsBytes), "ref:toolchain.go.version")
require.NotContains(t, string(commandsBytes), goVersion)
```

For Phase 13, assert the production frontend has zero warnings and zero unregistered violations. A manifest value may be referenced by generated outputs, but its authority must not be repeated manually in CSS, TS, the public barrel, the guide, or test inventories.

### Generated source freshness

**Applies to:** `frontend/scripts/design-system/generate.mjs`, `tokens.generated.css`, `tokens.generated.ts`, the public barrel/inventory markers, and generation tests.

**Primary analog:** `internal/scriptsdk/generate_test.go`

**Determinism pattern** (`internal/scriptsdk/generate_test.go:125-145`):

```go
dirA := t.TempDir()
dirB := t.TempDir()
require.NoError(t, scriptsdk.GenerateInto(dirA))
require.NoError(t, scriptsdk.GenerateInto(dirB))

for _, relative := range []string{
	"internal/scriptsdk/generated/golc.d.ts",
	"internal/scriptsdk/generated/golc-runtime.ts",
} {
	a, err := os.ReadFile(filepath.Join(dirA, filepath.FromSlash(relative)))
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join(dirB, filepath.FromSlash(relative)))
	require.NoError(t, err)
	require.Equal(t, a, b, "expected byte-identical generation for %s", relative)
}
```

Generate twice into independent temporary directories, then compare bytes. Sort every manifest-derived collection, normalize repository-relative paths and LF endings, omit timestamps/machine paths, and end generated text with one newline.

**Precise drift reporting pattern** (`internal/scriptsdk/generate_test.go:148-165`):

```go
require.NoError(t, scriptsdk.GenerateAll(root), "seed GenerateAll failed")
changed, err := scriptsdk.CheckDrift(root)
require.NoError(t, err)
require.Empty(t, changed)

typesPath := filepath.Join(root, "internal", "scriptsdk", "generated", "golc.d.ts")
original, err := os.ReadFile(typesPath)
require.NoError(t, err)
mutated := append(append([]byte{}, original...), []byte("\n// hand-edit")...)
require.NoError(t, os.WriteFile(typesPath, mutated, 0o644))

changed, err = scriptsdk.CheckDrift(root)
require.NoError(t, err)
require.Len(t, changed, 1)
require.Equal(t, "internal/scriptsdk/generated/golc.d.ts", changed[0])
```

`check:design-system` should byte-compare in memory and print the exact repair command (`npm run generate:design-system`). It must not silently rewrite checked-in files.

**Read-only check guarantee** (`internal/contracts/generate_test.go:236-254`):

```go
before := map[string][]byte{}
for _, descriptor := range contracts.RegisteredSchemas() {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(descriptor.OutputPath)))
	require.NoError(t, err)
	before[descriptor.OutputPath] = data
}
changed, err := contracts.CheckDrift(root)
require.NoError(t, err)
require.Empty(t, changed)
for path, want := range before {
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	require.NoError(t, err)
	require.Equal(t, want, got, "CheckDrift must leave %s untouched", path)
}
```

Apply the same guard to generated token files, the barrel, and guide inventory markers.

### Package scripts, command registry, Mage, and CI

**Applies to:** `frontend/package.json`, lockfile, a registry-backed UI command under `internal/command`, `config/commands.toml`, `magefiles/magefile.go`, and the Windows UI workflow.

**Package-script composition** (`frontend/package.json:6-13`):

```json
"scripts": {
  "dev": "vite",
  "test": "vitest run",
  "test:e2e": "playwright test",
  "test:e2e:resize": "playwright test e2e/resize.spec.ts",
  "docs:screenshots": "playwright test e2e/desktop-view-docs.spec.ts --workers=1",
  "build": "tsc --noEmit && vitest run && vite build"
}
```

Add the named design-system commands alongside these. `build` must invoke the static/freshness and unit/contract gates before TypeScript/Vite output. Keep the browser target separate and serialized:

```json
"check:design-system": "node scripts/design-system/check.mjs",
"generate:design-system": "node scripts/design-system/generate.mjs",
"test:design-system": "vitest run src/design-system scripts/design-system",
"test:e2e:design-system": "playwright test e2e/design-system.*.spec.ts --workers=1"
```

**Pinned frontend subprocess pattern** (`internal/command/build.go:203-240`):

```go
fresh, err := bootstrap.FrontendDistFresh(frontendDir, distIndexPath)
if err != nil {
	return err
}
if fresh {
	return nil
}
node, err := resolvePinnedNodeInstallation(root)
if err != nil {
	return err
}
execution := exec.Command(node.Executable, node.NPMCLI, "run", "build")
execution.Dir = frontendDir
execution.Env = upsertEnvironment(os.Environ(), "GOLC_PROJECT_ROOT", root)
if err := execution.Run(); err != nil {
	return fmt.Errorf("GOLC_BUILD_FRONTEND_FAILED: %w", err)
}
```

The new browser command belongs in `internal/command` and must resolve the pinned Node/NPM entrypoint. Do not launch `npx` or ambient Node directly from `magefiles/magefile.go`.

**Thin Mage adapter pattern** (`magefiles/magefile.go:100-105`, `168-184`):

```go
func runTarget(name string, ctx context.Context) error {
	target, ok := delivery.LookupMageTarget(name)
	if !ok {
		return fmt.Errorf("GOLC_MAGE_TARGET_UNKNOWN: %s", name)
	}
	// Execute the registered target.
}

func GenerateCheck() error { return runTarget("generatecheck", context.Background()) }
func Check() error         { return runTarget("check", context.Background()) }
func Build() error         { return runTarget("build", context.Background()) }
```

Add one thin exported Mage function for the UI browser target and register its implementation in the shared target registry. Preserve command-parity tests rather than creating a second ad hoc execution graph.

**Declarative ordered graph pattern** (`config/commands.toml:20-50`):

```toml
[commands.pr]
steps = "bootstrap,generate --check,check --offline,build,test --quick,package --foundation"
network_steps = "bootstrap"
credential_steps = "none"
mutation_steps = "none"
```

The static/unit DS checks join `npm run build`, and therefore the existing offline core build. Playwright installation and browser execution are a distinct Windows job because they require a provisioned Chromium binary.

**Windows job pattern** (`.github/workflows/check.yml:33-55`):

```yaml
jobs:
  check:
    runs-on: windows-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Install pinned Mage
        run: pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/ci/install-pinned-mage.ps1
      - name: bootstrap
        run: mage Bootstrap
      - name: generate --check
        run: mage GenerateCheck
      - name: check --offline
        run: mage CheckOffline
      - name: build
        run: mage Build
```

The dedicated UI job should follow this bootstrap sequence, install the Playwright Chromium matching the lockfile, call the registry-backed Mage UI target, and upload `test-results/**`/diff artifacts on failure. Do not put canonical snapshots or a blocking visual gate in `.github/workflows/cross-platform-mage.yml`.

### React primitive APIs and tests

**Applies to:** hardened existing primitives and new `IconButton`, `Tabs`, `LoadingState`, `ErrorState`, `Dialog`, and `ConfirmDialog`.

**Primary analog:** `frontend/src/components/primitives/Button/Button.tsx`

**Typed native-prop extension and closed variants** (`Button.tsx:11-27`):

```tsx
import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "lucide-react";

export type ButtonVariant = "primary" | "secondary" | "destructive";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  icon?: LucideIcon;
}

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: styles.primary,
  secondary: styles.secondary,
  destructive: styles.destructive,
};
```

Use native element props, a small closed union for design-system variants, and a `Record` that makes new variants compile-time exhaustive. Export public prop types from the single barrel. Avoid domain state or Wails calls inside primitives.

**Safe defaults and pass-through pattern** (`Button.tsx:29-45`):

```tsx
export default function Button({
  variant = "secondary",
  icon: Icon,
  className,
  type = "button",
  children,
  ...rest
}: ButtonProps) {
  const combinedClassName = className
    ? `${styles.button} ${VARIANT_CLASS[variant]} ${className}`
    : `${styles.button} ${VARIANT_CLASS[variant]}`;
  return (
    <button type={type} className={combinedClassName} {...rest}>
      {Icon ? <Icon size={14} className={styles.icon} aria-hidden="true" /> : null}
      {children}
    </button>
  );
}
```

Extend this pattern with ref forwarding, loading semantics that preserve the accessible name, required `aria-label` on `IconButton`, and explicit size variants. Keep decorative icons `aria-hidden`.

**Behavior-oriented component tests** (`Button.test.tsx:9-40`):

```tsx
it("defaults to type=\"button\" (never accidentally submits a form)", () => {
  render(<Button>Click me</Button>);
  expect(screen.getByRole("button")).toHaveAttribute("type", "button");
});

it("does not call onClick when disabled", () => {
  const onClick = vi.fn();
  render(<Button onClick={onClick} disabled>Click me</Button>);
  fireEvent.click(screen.getByRole("button"));
  expect(onClick).not.toHaveBeenCalled();
});

it.each(["primary", "secondary", "destructive"] as const)(
  "renders the %s variant without throwing",
  (variant) => {
    expect(() => render(<Button variant={variant}>{variant}</Button>)).not.toThrow();
  },
);
```

Each primitive test should cover its semantic role/name, safe default, every variant/state, native prop forwarding, disabled/loading duplicate-dispatch prevention, and keyboard interaction. Dialog focus trapping and return focus require Playwright, not only jsdom.

**Current modal behavior to preserve then replace** (`ConfirmModal.tsx:45-61`):

```tsx
<div className={styles.backdrop} onClick={onCancel}>
  <div
    className={styles.dialog}
    role="alertdialog"
    aria-modal="true"
    aria-label={title}
    onClick={(event) => event.stopPropagation()}
  >
    <Button variant="secondary" autoFocus onClick={onCancel}>
      {cancelLabel}
    </Button>
    <Button variant="primary" onClick={onConfirm}>
      {confirmLabel}
    </Button>
  </div>
</div>
```

The new `Dialog`/`ConfirmDialog` must retain backdrop dismissal policy, Escape, alertdialog semantics where destructive confirmation warrants it, and initial safe-action focus, while adding focus containment and return focus. Migrate call sites compatibly before deleting the old public name.

### Workspace and CSS migration

**Applies to:** `WorkspaceFrame`, all `frontend/src/workspaces/**`, shell/feature components, `frontend/src/index.css`, and CSS Modules.

**Shared bounded-shell analog** (`frontend/src/workspaces/workspace.module.css:1-19`):

```css
.workspace {
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.canvas {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: var(--space-md);
}
```

Turn this repeated contract into `WorkspaceFrame`: fixed shared toolbar plus one bounded, internally scrolling canvas. Replace the legacy spacing alias with the generated semantic token; do not allow body scrolling.

**Feature composition analog** (`FixtureLibraryWorkspace.tsx:79-88`, `457-466`):

```tsx
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Panel from "../../components/primitives/Panel/Panel";
import PanelHeader from "../../components/primitives/PanelHeader/PanelHeader";
import ScrollRegion from "../../components/primitives/ScrollRegion/ScrollRegion";
import EmptyState from "../../components/primitives/EmptyState/EmptyState";
import ListRow from "../../components/primitives/ListRow/ListRow";
import Chip from "../../components/primitives/Chip/Chip";
import Field from "../../components/primitives/Field/Field";
import Button from "../../components/primitives/Button/Button";

return (
  <div className={styles.workspace}>
    <Toolbar title="Fixture Library" icon={Lightbulb} />
    <div className={styles.canvas}>
      {loading ? <p className={styles.loading}>Loading fixture library…</p> : (
        <>
          {error ? <p className={styles.errorText}>{error}</p> : null}
          {/* feature content */}
        </>
      )}
    </div>
  </div>
);
```

This is the extraction seam: replace the outer shell with `WorkspaceFrame`, the local loading/error/empty branches with `DataList`/`LoadingState`/`ErrorState`, local action ordering with `FormActions`, and preview/warning/blocker sections with `ImpactReview`. Preserve every Wails bridge call, state transition, preview/commit split, stable row key, and error-message source.

**Migration boundary:** CSS Modules may retain domain-only geometry and specialized vendor/editor rules only through exact exception records. Shared color, typography, spacing, radius, focus, motion, shadow, stacking, controls, and common states move behind generated tokens/primitives. Feature code must not branch on theme names.

### Playwright deterministic screenshots, geometry, focus, and safety

**Applies to:** `frontend/e2e/design-system.visual.spec.ts`, `design-system.a11y.spec.ts`, deterministic fixtures/helpers, `screenshot.css`, snapshots, and `playwright.config.ts`.

**Real-layout rationale/config** (`frontend/playwright.config.ts:3-18`, `19-32`):

```ts
// jsdom does not run a layout engine; real geometry belongs in Playwright.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  use: { baseURL: "http://localhost:4790" },
  webServer: {
    command: "npm run dev -- --port 4790 --strictPort",
    port: 4790,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
```

Keep the general suite parallel, but serialize the canonical design-system screenshot command with `--workers=1`. Configure the reviewed Windows Chromium project/snapshot path without making other OS snapshots canonical.

**Viewport matrix and catalog-driven sweep** (`frontend/e2e/responsive.spec.ts:18-24`, `33-49`):

```ts
import { NAV_LABELS, settle, findOverflowingControls } from "./helpers";

const WIDTHS = [900, 1280] as const;

for (const width of WIDTHS) {
  test(`every workspace at ${width}px width`, async ({ page }) => {
    await page.setViewportSize({ width, height: 720 });
    await page.goto("/");
    for (const label of NAV_LABELS) {
      await page.getByRole("button", { name: label, exact: true }).click();
      await expect(page.getByRole("heading", { name: label, exact: true })).toBeVisible();
      await settle(page);
      expect(await findOverflowingControls(page), `${label} at ${width}px`).toEqual([]);
    }
  });
}
```

Consume the authoritative destination catalog; do not create a second hand-maintained route list. Extend the matrix with light/dark, reduced motion, modal, busy, error, compact, offline/degraded daemon, and 200% zoom states.

**Geometry helper** (`frontend/e2e/helpers.ts:247-284`):

```ts
export async function findOverflowingControls(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const offenders: string[] = [];
    const viewportWidth = window.innerWidth;
    const controls = Array.from(
      document.querySelectorAll<HTMLElement>("button, input, select, [contenteditable]"),
    );
    for (const control of controls) {
      const rect = control.getBoundingClientRect();
      if (rect.width === 0 && rect.height === 0) continue;
      if (control.scrollHeight - control.clientHeight > 1) {
        offenders.push("control content is taller than its own box");
      }
      if (rect.right > viewportWidth + 1 || rect.left < -1) {
        offenders.push("control is outside the viewport");
      }
    }
    return offenders;
  });
}
```

Reuse and extend this helper for exact minimum target geometry, workspace/body overflow, modal containment, and persistent safety-control visibility.

**Safety invariant helper** (`frontend/e2e/helpers.ts:287-302`):

```ts
export async function expectSafetyClusterAvailable(page: Page): Promise<void> {
  const cluster = page.getByLabel("Safety cluster");
  await expect(cluster).toBeVisible();
  const controls = cluster.locator("button");
  await expect(controls).toHaveCount(3);
  for (let index = 0; index < 3; index += 1) {
    await expect(controls.nth(index)).toBeVisible();
    await expect(controls.nth(index)).toBeEnabled();
  }
}
```

Call this in every normal/busy/error/modal/compact/offline state. Never mask the safety cluster in screenshots.

**Deterministic capture styling** (`frontend/e2e/desktop-view-docs.spec.ts:30-40`):

```ts
await page.addStyleTag({
  content: `
    *, *::before, *::after {
      animation: none !important;
      caret-color: transparent !important;
      transition: none !important;
    }
  `,
});
await page.waitForTimeout(250);
```

Move the stable styling to `frontend/e2e/screenshot.css`, wait for `document.fonts.ready`, seed deterministic Wails fixtures before navigation, and avoid arbitrary masks. The only acceptable masking is genuinely nondeterministic non-safety content documented in the test.

**Screenshot naming/theme pattern** (`site/tests/visual.spec.ts:24-37`):

```ts
for (const route of ROUTES) {
  test(`${route.name} — light`, async ({ page }) => {
    await page.goto(route.path);
    await page.waitForLoadState("networkidle");
    await expect(page).toHaveScreenshot(`${route.name}-light.png`, { fullPage: true });
  });

  test(`${route.name} — dark`, async ({ page }) => {
    await page.goto(route.path);
    await page.evaluate(() => document.documentElement.setAttribute("data-theme", "dark"));
    await expect(page).toHaveScreenshot(`${route.name}-dark.png`, { fullPage: true });
  });
}
```

For the desktop app, prefer stable bounded workspace/shell locators over `fullPage` where body scrolling is prohibited. Add `toMatchAriaSnapshot`, role/name assertions, dialog initial-focus/Tab/Escape/return-focus checks, and direct clicks on Blackout/Revoke/Stop-Release in degraded states.

### Agent-facing documentation

**Applies to:** `frontend/DESIGN_SYSTEM.md`, frontend README/project instruction links, and generated inventory markers.

**Generated ownership marker** (`docs/reference/README.md:1-4`):

```md
<!-- GENERATED by github.com/lnorton89/golc/internal/docgen. DO NOT EDIT. -->
# Package reference

One page per internal package ... generated by the "docs" route.
```

Use explicit start/end markers around the generated component inventory within `DESIGN_SYSTEM.md`, naming the generator and repair command. Human-authored guidance stays outside those markers.

**Single-source inventory pattern** (`docs/reference/README.md:10-14`):

```md
- [github.com/lnorton89/golc/internal/command](./command.md) — Package command implements ...
- [github.com/lnorton89/golc/internal/contracts](./contracts.md) — Package contracts is GOLC's single deterministic JSON Schema generation and drift-check registry ...
- [github.com/lnorton89/golc/internal/docgen](./docgen.md) — Package docgen extracts ... and renders ...
```

Generate the public primitive/pattern table from `components.json`, including public name, kind, export path, guide anchor, and contract-test path. DS007 must check both directions: every manifest record resolves to an export/guide/test, and every public barrel export has one manifest record.

The guide should give agents concise rules: import only from the public barrel; choose tokens by semantic role; compose workspaces through named patterns; use exact exceptions only for approved geometry/vendor cases; never introduce theme-name branches; run `npm run check:design-system`; and use `npm run generate:design-system` to repair drift.

## Shared Patterns

### Stable diagnostics and fail-closed parsing

**Sources:** `internal/projectconfig/registry.go:55-62`; `internal/projectconfig/strict_test.go:173-191`

Apply to all manifests and DS001-DS010 rules:

- A parse or schema failure exits nonzero; malformed CSS/TSX is not skipped.
- Diagnostics carry a stable rule ID plus normalized repository-relative `path:line:column`.
- Enumerate files and diagnostics in sorted order.
- Reject absolute, traversal, and resolved out-of-root paths.
- An exception is exact: one rule, one file, one parsed locator/property/value, one rationale; zero or multiple matches fail as stale/broad.

### UI remains a projection

**Source:** `FixtureLibraryWorkspace.tsx:59-88`, existing Wails bridge imports and shared primitive imports.

Primitives/patterns own presentation, semantics, focus, layout, and common state composition. They do not call Wails, mutate Go-owned show state, schedule playback, or duplicate safety authority. Workspace migrations preserve command wiring and only change composition/chrome.

### Loading/error scope

**Source:** `FixtureLibraryWorkspace.tsx:138-190`, `457-466`

Maintain explicit local `loading` and `error` state around adapter calls, clear/set them in `try/catch/finally`, and migrate only their rendering to shared states. A mutating action disables only its initiating controls; shell and safety controls remain available.

### No second inventories

**Sources:** `frontend/e2e/responsive.spec.ts:18-24`; `docs/reference/README.md:1-14`

Navigation tests consume the navigation catalog; generated docs consume their registry; Phase 13 tests/barrel/guide consume `components.json`. Do not hand-maintain parallel component lists.

## Parallel Execution Ownership and Collision Boundaries

| Owner slice | Exclusive write scope | Must coordinate before touching |
|---|---|---|
| Manifest/checker foundation | `frontend/design-system/**`, `frontend/scripts/design-system/**`, generated token files | `frontend/package.json`, lockfile, public barrel, guide markers |
| Primitive completion | Individual primitive directories/tests and new primitive directories | public `index.ts`, `components.json`, shared generated token APIs |
| Pattern extraction | Individual pattern directories/tests | public `index.ts`, `components.json`, workspace call sites |
| Theme/shell | `frontend/src/index.css`, `lib/theme*`, assigned shell/safety component files | generated token names, `playwright.config.ts` |
| Workspace migrations | A disjoint workspace/domain component/CSS set per worker | `workspace.module.css`, public primitive/pattern APIs, shell files |
| Browser gates | `frontend/e2e/design-system.*`, screenshot assets, `playwright.config.ts` | deterministic gallery/fixture route, package scripts, safety component selectors |
| Root integration | `frontend/package*.json`, assigned `internal/command/*`, `config/commands.toml`, `magefiles/magefile.go`, chosen workflow | all command names and test script names |
| Documentation | `frontend/DESIGN_SYSTEM.md` and approved instruction/README links | generated inventory marker content and public barrel |

Collision rules:

1. Give `components.json` and `frontend/src/design-system/index.ts` one owner. Primitive/pattern workers add implementation files and report intended records/exports; the inventory owner lands them in one deterministic change.
2. Give `frontend/package.json` and `package-lock.json` one integration owner. Other slices must not independently add dependencies or scripts.
3. Give `frontend/src/index.css` one theme owner; workspace workers modify only their assigned CSS Modules.
4. Split workspace migrations by complete TSX/test/module triplets so behavior and visual migration land together.
5. Land manifest/token names before parallel CSS migration, or freeze a reviewed generated token API for the duration of the wave.
6. Land the deterministic gallery/fixture contract before browser baselines; otherwise baseline churn will hide real regressions.
7. The working tree already contains unrelated user work (`site` is modified) and the phase directory contains a user-owned `.gitkeep`; do not alter or revert either.
8. **Do not inspect, modify, move, format, stage, or delete `internal/deskmidi/`.** Treat the entire tree as user-owned untracked work even if a later status view exposes it. Phase 13 has no legitimate dependency on it.

## No Analog Found

| File / Slice | Role | Data Flow | Reason / Planner Guidance |
|---|---|---|---|
| `frontend/scripts/design-system/{css-policy,tsx-policy}.mjs` implementing DS001-DS010 | validator | AST/CSS transform + batch | No existing JS syntax-policy checker combines PostCSS and the TypeScript 6 compatibility compiler. Use RESEARCH.md parsing guidance, but copy strict registry, deterministic ordering, path containment, stable diagnostics, and allowed/forbidden fixture patterns above. |
| `frontend/e2e/design-system.a11y.spec.ts` full focus/ARIA/reduced-motion/safety matrix | browser test | request-response + event-driven | Existing Playwright tests provide geometry, safety availability, capture, and modal role checks, but no single complete accessibility matrix. Compose the existing helpers and add the missing focus/ARIA assertions from the UI-SPEC. |

## Metadata

**Analog search scope:** `frontend/**`, `internal/{projectconfig,scriptsdk,contracts,docgen,command}/**`, `config/**`, `magefiles/**`, `.github/workflows/**`, `docs/reference/**`, plus the read-only Phase 13 context/UI/research artifacts.

**Files scanned:** 362 files in the focused implementation/analog scope, excluding `frontend/node_modules/**` and `internal/deskmidi/**`.

**Strong analog groups:** 5 — strict registries, generated freshness, command/Mage/CI routing, React/workspace composition, and Playwright/docs automation.

**Pattern extraction date:** 2026-08-02
