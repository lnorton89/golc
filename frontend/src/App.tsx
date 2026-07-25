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
import AppShell from "./shell/AppShell";
import ErrorBoundary from "./shell/ErrorBoundary";

export default function App() {
  return (
    <ErrorBoundary>
      <AppShell />
    </ErrorBoundary>
  );
}
