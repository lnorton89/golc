// OverviewWorkspace.test.tsx exercises the workspace end-to-end against a
// mocked window.go.wails.ShowService, the same direct-window-object
// convention every other Wails-bridge test in this codebase uses (see
// wailsBridge.ts's own doc comment; mirrors ScenesLooksWorkspace.test.tsx's
// identical shape). 09-03-PLAN.md Task 1 extends this file with Guided
// First Show's entry-point cases (D-08/D-10): auto-launch on a genuinely
// empty show, no-auto-launch when content exists or no show path is
// resolved (the exact condition that keeps AppShell.navigation.test.tsx's
// no-bridge regression test green), the once-per-process auto-launch
// guard, and the manual "Start Guide" action -- every render now wraps
// OverviewWorkspace in GuidedFirstShowProvider, since it calls
// useGuidedFirstShow() (startGuide/requestAutoLaunch) directly.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import OverviewWorkspace from "./OverviewWorkspace";
import GuidedFirstShow from "./GuidedFirstShow/GuidedFirstShow";
import { GuidedFirstShowProvider, useGuidedFirstShow } from "./GuidedFirstShow/GuidedFirstShowContext";
import type { DestinationId } from "../../shell/navigation";

function inspectView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    showPath: "C:\\shows\\demo.golc",
    schemaVersion: 3,
    revision: 7,
    pools: [{ id: "pool-1", name: "Wash", requiredCapabilities: [], memberCount: 2 }],
    deployments: [{ id: "deploy-1", name: "Main Rig", active: true, instanceCount: 2 }],
    ...overrides,
  };
}

function diagnosticReport(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    fileLevelIssues: [],
    structuralOk: true,
    migrationRequired: false,
    schemaVersion: 3,
    revision: 7,
    ...overrides,
  };
}

function programmingView(overrides: Partial<Record<string, unknown>> = {}) {
  return { scenes: [], themes: [], presets: [], chases: [], motions: [], blends: [], instances: [], ...overrides };
}

// ShellCanvasHarness mirrors AppShell.tsx's own ShellCanvas (09-03-PLAN.md
// Task 2): renders the guide overlay instead of Overview whenever the
// provider's `open` flag is set (by auto-launch or Start Guide).
function ShellCanvasHarness({ onNavigate }: { onNavigate: (destination: DestinationId) => void }) {
  return (
    <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
      <ShellCanvasInner />
    </GuidedFirstShowProvider>
  );
}

function ShellCanvasInner() {
  const { open } = useGuidedFirstShow();
  return open ? <GuidedFirstShow /> : <OverviewWorkspace />;
}

// GuideOpenProbe exposes the provider's internal state/actions directly so
// the once-per-process auto-launch guard (09-03-PLAN.md, D-08) can be
// tested without depending on GuidedFirstShow's own rendered markup.
function GuideOpenProbe() {
  const { open, exitGuide } = useGuidedFirstShow();
  return (
    <div>
      <span data-testid="guide-open">{open ? "open" : "closed"}</span>
      <button type="button" onClick={exitGuide}>
        test-exit-guide
      </button>
    </div>
  );
}

function renderOverview(onNavigate: (destination: DestinationId) => void = vi.fn()) {
  return render(
    <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
      <OverviewWorkspace />
    </GuidedFirstShowProvider>,
  );
}

describe("OverviewWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ShowService: {
          Inspect: vi.fn().mockResolvedValue(inspectView()),
          Diagnose: vi.fn().mockResolvedValue(diagnosticReport()),
        },
        ProgrammingService: {
          ListProgramming: vi.fn().mockResolvedValue(programmingView()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads and displays the show's identity, pools, and deployments", async () => {
    renderOverview();
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());
    expect(screen.getByText("Schema 3 · Revision 7")).toBeInTheDocument();
    expect(screen.getByText("Wash")).toBeInTheDocument();
    expect(screen.getByText("Main Rig")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows empty states when there are no pools or deployments yet", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Inspect.mockResolvedValue(inspectView({ pools: [], deployments: [] }));

    renderOverview();
    await waitFor(() => expect(screen.getByText("No fixture pools yet.")).toBeInTheDocument());
    expect(screen.getByText("No deployments yet.")).toBeInTheDocument();
  });

  it("runs Diagnose and renders a healthy result", async () => {
    renderOverview();
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Diagnose" }));
    await waitFor(() => expect(screen.getByText("Healthy")).toBeInTheDocument());
  });

  it("surfaces structural issues and a migration-required note", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Diagnose.mockResolvedValue(
      diagnosticReport({
        structuralOk: false,
        structuralError: "GOLC_SHOW_MIGRATION_REQUIRED: schema_version 2 requires migration to 3",
        migrationRequired: true,
      }),
    );

    renderOverview();
    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Diagnose" }));
    await waitFor(() => expect(screen.getByText("Issues found")).toBeInTheDocument());
    expect(screen.getByText("Migration required")).toBeInTheDocument();
    expect(screen.getByText(/GOLC_SHOW_MIGRATION_REQUIRED/)).toBeInTheDocument();
  });

  it("auto-launches on a genuinely empty show", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Inspect.mockResolvedValue(inspectView({ pools: [], deployments: [] }));

    render(<ShellCanvasHarness onNavigate={vi.fn()} />);

    await waitFor(() => expect(screen.getByRole("navigation", { name: "First show steps" })).toBeInTheDocument());
  });

  it("does not auto-launch when the show already has content", async () => {
    render(<ShellCanvasHarness onNavigate={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());
    expect(screen.queryByRole("navigation", { name: "First show steps" })).not.toBeInTheDocument();
  });

  it("does not auto-launch when no show path is resolved", async () => {
    vi.unstubAllGlobals();

    render(<ShellCanvasHarness onNavigate={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("(unsaved show)")).toBeInTheDocument());
    expect(screen.queryByRole("navigation", { name: "First show steps" })).not.toBeInTheDocument();
  });

  it("auto-launches at most once per process", async () => {
    const svc = (window as unknown as { go: { wails: { ShowService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ShowService;
    svc.Inspect.mockResolvedValue(inspectView({ pools: [], deployments: [] }));

    const onNavigate = vi.fn();
    const { rerender } = render(
      <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
        <GuideOpenProbe />
        <OverviewWorkspace />
      </GuidedFirstShowProvider>,
    );

    await waitFor(() => expect(screen.getByTestId("guide-open")).toHaveTextContent("open"));

    fireEvent.click(screen.getByRole("button", { name: "test-exit-guide" }));
    expect(screen.getByTestId("guide-open")).toHaveTextContent("closed");
    expect(onNavigate).toHaveBeenCalledWith("show-overview");

    // Remount Overview under the SAME provider instance (Provider persists
    // across Overview's own unmount/remount in the real shell, exactly as
    // it does when Exit Guide navigates back and WorkspaceRouter unmounts/
    // remounts the workspace) -- the ref-guarded requestAutoLaunch must not
    // reopen it a second time.
    rerender(
      <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
        <GuideOpenProbe />
      </GuidedFirstShowProvider>,
    );
    rerender(
      <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
        <GuideOpenProbe />
        <OverviewWorkspace />
      </GuidedFirstShowProvider>,
    );

    await waitFor(() => expect(svc.Inspect).toHaveBeenCalledTimes(2));
    expect(screen.getByTestId("guide-open")).toHaveTextContent("closed");
  });

  it("Start Guide opens the guide on a populated show", async () => {
    render(<ShellCanvasHarness onNavigate={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("C:\\shows\\demo.golc")).toBeInTheDocument());
    expect(screen.queryByRole("navigation", { name: "First show steps" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Start Guide" }));

    await waitFor(() => expect(screen.getByRole("navigation", { name: "First show steps" })).toBeInTheDocument());
  });
});
