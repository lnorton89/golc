// App.tsx is the thin pass-through render root -- all real layout lives in
// shell/AppShell.tsx (shell restructure, superseding the flat vertical
// stack this file used to compose directly). Kept as its own file (rather
// than pointing main.tsx at AppShell directly) because App.smoke.test.tsx
// imports this module by name as the app's build-gate entry point.
import AppShell from "./shell/AppShell";

export default function App() {
  return <AppShell />;
}
