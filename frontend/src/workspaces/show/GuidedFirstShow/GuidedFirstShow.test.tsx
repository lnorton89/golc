// GuidedFirstShow.test.tsx exercises the Guided First Show overlay itself
// (09-03-PLAN.md Task 1/2/3, FDUI-03): the locked five-stage rail, the
// shared footer/evidence-aside contract (GuideEvidenceList), and the two
// stages this plan implements (Fixtures/Patch) against a mocked
// window.go.wails, following ScriptsWorkspace.test.tsx/
// SaveRecoveryWorkspace.test.tsx's direct-window-object mocking
// convention. Auto-launch/Start-Guide entry-point behavior lives in
// OverviewWorkspace.test.tsx instead -- this file covers the overlay's own
// contract once it is already open (rendered directly, not gated on
// "open").
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import GuidedFirstShow from "./GuidedFirstShow";
import GuideEvidenceList from "./GuideEvidenceList";
import { GuidedFirstShowProvider } from "./GuidedFirstShowContext";
import type { DestinationId } from "../../../shell/navigation";

function fixtureRow(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    stableKey: "acme/par64",
    manufacturer: "Acme",
    model: "PAR64",
    fileName: "par64.yaml",
    source: "local",
    status: "valid",
    detail: "",
    ...overrides,
  };
}

function stubBridge(
  overrides: {
    fixtureRows?: ReturnType<typeof fixtureRow>[];
    pools?: unknown[];
    deployments?: unknown[];
  } = {},
) {
  const listLocal = vi.fn().mockResolvedValue({
    directory: "C:\\fixtures",
    rows: overrides.fixtureRows ?? [],
  });
  const listPatch = vi.fn().mockResolvedValue({
    pools: overrides.pools ?? [],
    deployments: overrides.deployments ?? [],
  });
  const applyPatch = vi.fn();
  const createPool = vi.fn();
  const addPoolMemberPreview = vi.fn();
  const removePoolMemberPreview = vi.fn();

  vi.stubGlobal("go", {
    wails: {
      FixtureLibraryService: { ListLocal: listLocal, Inspect: vi.fn() },
      FixturePatchService: {
        ListPatch: listPatch,
        ApplyPatch: applyPatch,
        CreatePool: createPool,
        AddPoolMemberPreview: addPoolMemberPreview,
        RemovePoolMemberPreview: removePoolMemberPreview,
        CreateDeployment: vi.fn(),
        ActivateDeployment: vi.fn(),
      },
      ShowService: { Inspect: vi.fn(), Diagnose: vi.fn() },
      ProgrammingService: {
        ListProgramming: vi.fn().mockResolvedValue({
          scenes: [],
          themes: [],
          presets: [],
          chases: [],
          motions: [],
          blends: [],
          instances: [],
        }),
      },
    },
  });

  return { listLocal, listPatch, applyPatch, createPool, addPoolMemberPreview, removePoolMemberPreview };
}

function renderGuide(onNavigate: (destination: DestinationId) => void = vi.fn()) {
  return render(
    <GuidedFirstShowProvider activeDestination="show-overview" onNavigate={onNavigate}>
      <GuidedFirstShow />
    </GuidedFirstShowProvider>,
  );
}

describe("GuidedFirstShow", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("renders all five locked stage labels in order", () => {
    stubBridge();
    renderGuide();

    const rail = screen.getByRole("navigation", { name: "First show steps" });
    const labels = Array.from(rail.querySelectorAll("button")).map((button) => button.textContent);
    expect(labels).toEqual(["Fixtures", "Patch", "Program", "Assign", "Verify"]);
  });

  it('marks the current stage with aria-current="step"', () => {
    stubBridge();
    renderGuide();

    const rail = screen.getByRole("navigation", { name: "First show steps" });
    const current = rail.querySelectorAll('[aria-current="step"]');
    expect(current).toHaveLength(1);
    expect(current[0]).toHaveTextContent("Fixtures");
  });

  it("Exit Guide is present and enabled on every stage, and returns to the previous workspace", () => {
    stubBridge();
    const onNavigate = vi.fn();
    renderGuide(onNavigate);

    for (const label of ["Fixtures", "Patch", "Program", "Assign", "Verify"]) {
      fireEvent.click(screen.getByRole("button", { name: label }));
      expect(screen.getByRole("button", { name: "Exit Guide" })).toBeEnabled();
    }

    fireEvent.click(screen.getByRole("button", { name: "Exit Guide" }));
    expect(onNavigate).toHaveBeenCalledWith("show-overview");
  });

  it("renders the nothing-to-preview empty state on a not-yet-implemented stage", () => {
    stubBridge();
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Program" }));
    expect(
      screen.getByText("Nothing to preview yet — complete this stage's action to see it here."),
    ).toBeInTheDocument();
  });

  it("Fixtures stage reports a blocker with an empty library and evidence with a populated one", async () => {
    const { listLocal } = stubBridge({ fixtureRows: [] });
    renderGuide();

    await waitFor(() => expect(listLocal).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByText("Blocker")).toBeInTheDocument());

    listLocal.mockResolvedValue({ directory: "C:\\fixtures", rows: [fixtureRow()] });
    // Switch away and back so FixturesStage unmounts/remounts -- it has no
    // cached progress flag, so a fresh mount re-reads the library (proven
    // by the ListLocal call count below).
    fireEvent.click(screen.getByRole("button", { name: "Patch" }));
    fireEvent.click(screen.getByRole("button", { name: "Fixtures" }));

    await waitFor(() => expect(listLocal).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByText("Evidence")).toBeInTheDocument());
  });

  it("Patch stage never applies a patch", async () => {
    const { listPatch, applyPatch, createPool, addPoolMemberPreview } = stubBridge({
      pools: [{ id: "pool-1", name: "Wash", requiredCapabilities: [], members: [] }],
      deployments: [{ id: "deploy-1", name: "Main Rig", active: true, instances: [] }],
    });
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Patch" }));
    await waitFor(() => expect(listPatch).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByRole("button", { name: "Review Patch & Continue" })).toBeEnabled());

    fireEvent.click(screen.getByRole("button", { name: "Review Patch & Continue" }));

    expect(applyPatch).not.toHaveBeenCalled();
    expect(createPool).not.toHaveBeenCalled();
    expect(addPoolMemberPreview).not.toHaveBeenCalled();
  });
});

describe("GuideEvidenceList", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders blocker, warning, and evidence as distinct labelled rows with no percentage or progressbar", () => {
    render(
      <GuideEvidenceList
        items={[
          { tone: "blocker", label: "No pools yet", detail: "Create a pool first." },
          { tone: "warning", label: "No active deployment", detail: "Activate a deployment." },
          { tone: "evidence", label: "Patch ready", detail: "A deployment is active." },
        ]}
      />,
    );

    expect(screen.getByText("Blocker")).toBeInTheDocument();
    expect(screen.getByText("Warning")).toBeInTheDocument();
    expect(screen.getByText("Evidence")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.queryByText(/%/)).not.toBeInTheDocument();
  });

  it("renders the nothing-to-preview empty state when there are no items", () => {
    render(<GuideEvidenceList items={[]} />);
    expect(
      screen.getByText("Nothing to preview yet — complete this stage's action to see it here."),
    ).toBeInTheDocument();
  });
});
