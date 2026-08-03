import ErrorBoundary from "../../shell/ErrorBoundary";

// EmergencyFallbackFixture deterministically forces a render-time exception
// so Plan 13-30 Task 2's Playwright spec can prove ErrorBoundary's
// token-independent emergency fallback stays readable and operable even
// when the generated theme stylesheet is blocked -- there is no existing
// browser-reachable way to trigger this deliberately (only a Vitest-local
// `Bomb` helper in ErrorBoundary.test.tsx, unreachable from a real page).
// Mirrors the `?e2e=dialog-feasibility` / `?e2e=design-system-gallery`
// seam in App.tsx exactly: a fixed-purpose fixture route, not a second
// routing mechanism.
function AlwaysThrows(): never {
  throw new Error("Plan 13-30 deterministic emergency-fallback fixture: forced render failure");
}

export default function EmergencyFallbackFixture() {
  return (
    <ErrorBoundary>
      <AlwaysThrows />
    </ErrorBoundary>
  );
}
