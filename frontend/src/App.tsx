// App.tsx is the thin pass-through render root -- all real layout lives in
// shell/AppShell.tsx (shell restructure, superseding the flat vertical
// stack this file used to compose directly). Kept as its own file (rather
// than pointing main.tsx at AppShell directly) because App.smoke.test.tsx
// imports this module by name as the app's build-gate entry point.
//
// ErrorBoundary wraps AppShell here -- the highest practical point in the
// tree -- so an uncaught render exception anywhere inside the shell (a
// workspace, the safety cluster, anything) renders a visible error screen
// instead of unmounting into a blank window (see ErrorBoundary.tsx's own
// doc comment for the real bug that motivated this).
import { lazy, Suspense } from "react";

import ErrorBoundary from "./shell/ErrorBoundary";
import DialogFeasibility from "./design-system/fixtures/DialogFeasibility";
import DesignSystemGallery from "./design-system/fixtures/DesignSystemGallery";

// Keep the test-only fixture route independent from the normal shell bundle.
// It still inherits index.css's generated semantic theme contract, while the
// route itself remains a proof seam rather than an operator-facing theme path.
const AppShell = lazy(() => import("./shell/AppShell"));

export default function App() {
  if (globalThis.location.search === "?e2e=dialog-feasibility") {
    return <DialogFeasibility />;
  }

  // Plan 13-17's calibration/matrix fixtures need a real, reachable browser
  // page for DesignSystemGallery.tsx (previously only rendered inside
  // Vitest's jsdom via DesignSystemGallery.test.tsx) -- mirrors the exact
  // ?e2e=dialog-feasibility seam above rather than inventing a second
  // routing mechanism.
  if (globalThis.location.search === "?e2e=design-system-gallery") {
    return <DesignSystemGallery />;
  }

  return (
    <ErrorBoundary>
      <Suspense fallback={null}>
        <AppShell />
      </Suspense>
    </ErrorBoundary>
  );
}
