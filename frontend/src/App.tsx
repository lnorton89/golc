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

// Keep the test-only fixture route independent from the normal shell bundle:
// feasibility proof must not be blocked by unrelated workspace CSS while still
// loading the exact normal shell on every operator-facing route.
const AppShell = lazy(() => import("./shell/AppShell"));

export default function App() {
  if (globalThis.location.search === "?e2e=dialog-feasibility") {
    return <DialogFeasibility />;
  }

  return (
    <ErrorBoundary>
      <Suspense fallback={null}>
        <AppShell />
      </Suspense>
    </ErrorBoundary>
  );
}
