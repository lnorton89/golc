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
//
// QueryClientProvider sits OUTSIDE the ?e2e= fixture early-returns rather
// than around <AppShell/> alone: DeskOperatorFixture renders the real Desk,
// which reads its live universe values through useQuery, so a fixture route
// that skipped the provider would throw "No QueryClient set" the moment a
// Playwright matrix run loaded it. Mounting it above every branch keeps one
// rule -- every render path this file can take has a client.
//
// The client is created via useState's lazy initialiser (not a module-level
// const) so each mounted App owns its own cache. App.smoke.test.tsx and the
// component tests mount and unmount App repeatedly in one process; a shared
// module-level client would leak one test's cached rows into the next.
// Toast mounts beside QueryClientProvider and for the same reason: it is a
// provider, so every render path this file can take must sit beneath it or
// a useToast() call somewhere below throws. Its viewport is anchored
// bottom-right and pointer-events:none, so mounting it globally can never
// intercept the safety controls in GlobalFrame's header (see Toast.tsx).
import { lazy, Suspense, useState } from "react";
import { QueryClientProvider } from "@tanstack/react-query";

import ErrorBoundary from "./shell/ErrorBoundary";
import Toast from "./components/primitives/Toast/Toast";
import { createQueryClient } from "./lib/queryClient";
import DialogFeasibility from "./design-system/fixtures/DialogFeasibility";
import DesignSystemGallery from "./design-system/fixtures/DesignSystemGallery";
import EmergencyFallbackFixture from "./design-system/fixtures/EmergencyFallbackFixture";
import DeskOperatorFixture from "./design-system/fixtures/DeskOperatorFixture";
import ScriptsNotesFixture from "./design-system/fixtures/ScriptsNotesFixture";

// Keep the test-only fixture route independent from the normal shell bundle.
// It still inherits index.css's generated semantic theme contract, while the
// route itself remains a proof seam rather than an operator-facing theme path.
const AppShell = lazy(() => import("./shell/AppShell"));

export default function App() {
  const [queryClient] = useState(createQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      <Toast>
        <AppRoute />
      </Toast>
    </QueryClientProvider>
  );
}

// AppRoute holds the pre-existing route selection verbatim -- split out of
// App only so the provider above can wrap every branch without turning each
// early return into its own nested provider.
function AppRoute() {
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

  // Plan 13-30's emergency-fallback backstop needs a deterministic,
  // browser-reachable render-time failure (UI-CONSIDERATIONS-BACKSTOP-ERROR)
  // -- mirrors the two seams above exactly rather than inventing a second
  // routing mechanism. This fixture wraps its own ErrorBoundary rather than
  // relying on the one below, so the forced failure never depends on
  // AppShell/Suspense ever mounting.
  if (globalThis.location.search === "?e2e=emergency-fallback") {
    return <EmergencyFallbackFixture />;
  }

  // Plan 13-34's Desk/Operator Surface and Scripts/Notes baseline matrices
  // each need a combined, browser-reachable bounded state UI-SPEC's
  // Required reference matrix describes as one row -- no single existing
  // navigable destination renders both halves of either row on one
  // screen. Mirrors the three seams above exactly rather than inventing a
  // second routing mechanism.
  if (globalThis.location.search === "?e2e=desk-operator") {
    return <DeskOperatorFixture />;
  }

  if (globalThis.location.search === "?e2e=scripts-notes") {
    return <ScriptsNotesFixture />;
  }

  return (
    <ErrorBoundary>
      <Suspense fallback={null}>
        <AppShell />
      </Suspense>
    </ErrorBoundary>
  );
}
