// GuidedFirstShow.test.tsx exercises the Guided First Show overlay itself
// (09-03-PLAN.md Task 1/2/3, FDUI-03): the locked five-stage rail, the
// shared footer/evidence-aside contract (GuideEvidenceList), and the two
// stages 09-03 implemented (Fixtures/Patch) against a mocked
// window.go.wails, following ScriptsWorkspace.test.tsx/
// SaveRecoveryWorkspace.test.tsx's direct-window-object mocking
// convention. Auto-launch/Start-Guide entry-point behavior lives in
// OverviewWorkspace.test.tsx instead -- this file covers the overlay's own
// contract once it is already open (rendered directly, not gated on
// "open").
//
// 09-04-PLAN.md Task 1 extends this file with the readiness-rollup pure
// function tests (deriveProgramStatus/deriveAssignStatus/aggregateReadiness,
// readiness.ts) and the remaining three stages (Program/Assign/Verify),
// including the evidence-based Perform gate this plan implements in
// VerifyStage. These new cases are written RED, against a `./readiness`
// module and Program/Assign/Verify stage behavior that do not exist yet.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import GuidedFirstShow from "./GuidedFirstShow";
import GuideEvidenceList from "./GuideEvidenceList";
import { GuidedFirstShowProvider } from "./GuidedFirstShowContext";
import { aggregateReadiness, deriveAssignStatus, deriveProgramStatus } from "./readiness";
import type { GuideStageStatus } from "./stages";
import type { DestinationId } from "../../../shell/navigation";
import type { ProgrammingView } from "../../../lib/wailsBridge";

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

function programmingView(overrides: Partial<ProgrammingView> = {}): ProgrammingView {
  return {
    scenes: [],
    themes: [],
    presets: [],
    chases: [],
    motions: [],
    blends: [],
    instances: [],
    ...overrides,
  };
}

function scene(name: string): ProgrammingView["scenes"][number] {
  return { name, active: false, barsPerLoop: 4, layers: [] };
}

function stubBridge(
  overrides: {
    fixtureRows?: ReturnType<typeof fixtureRow>[];
    pools?: unknown[];
    deployments?: unknown[];
    scenes?: ProgrammingView["scenes"];
    surfaces?: unknown[];
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
  const listProgramming = vi.fn().mockResolvedValue(programmingView({ scenes: overrides.scenes ?? [] }));
  const listSurfaces = vi.fn().mockResolvedValue(overrides.surfaces ?? []);

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
      ProgrammingService: { ListProgramming: listProgramming },
      SurfaceService: {
        ListSurfaces: listSurfaces,
        CreateSurface: vi.fn(),
        AssignItem: vi.fn(),
        UnassignItem: vi.fn(),
        ShowSurface: vi.fn(),
        RemoveSurface: vi.fn(),
        AuthorizeControl: vi.fn(),
      },
    },
  });

  return {
    listLocal,
    listPatch,
    applyPatch,
    createPool,
    addPoolMemberPreview,
    removePoolMemberPreview,
    listProgramming,
    listSurfaces,
  };
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

  // --- 09-04-PLAN.md Task 3: VerifyStage and the evidence-based Perform gate ---

  const readyBridge = {
    fixtureRows: [fixtureRow()],
    pools: [{ id: "pool-1", name: "Wash", requiredCapabilities: [], members: [] }],
    deployments: [{ id: "deploy-1", name: "Main Rig", active: true, instances: [] }],
    scenes: [scene("Intro")],
    surfaces: [{ name: "Booth", assignedCount: 0 }],
  };

  const emptyBridge = {
    fixtureRows: [],
    pools: [],
    deployments: [],
    scenes: [],
    surfaces: [],
  };

  it("VerifyStage with zero blockers renders an enabled Perform action", async () => {
    stubBridge(readyBridge);
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));

    await waitFor(() => expect(screen.getByText("0 blockers")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Perform" })).toBeEnabled();
  });

  it("VerifyStage with one or more blockers renders the Perform action disabled together with the blocker list", async () => {
    stubBridge(emptyBridge);
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));

    await waitFor(() => expect(screen.getByText("4 blockers")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Perform" })).toBeDisabled();

    const aside = screen.getByLabelText("Live preview and evidence");
    expect(within(aside).getAllByText("Blocker")).toHaveLength(4);
  });

  it("VerifyStage renders each category count with correct singular/plural agreement, including an explicit zero count", async () => {
    // Only Fixtures reports a blocker (empty library); Patch/Program/Assign
    // are otherwise ready, so blockers=1 (singular), warnings=0 (explicit
    // zero, never omitted), evidence=4 (patch + program + assign + the
    // optional MIDI row).
    stubBridge({ ...readyBridge, fixtureRows: [] });
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));

    await waitFor(() => expect(screen.getByText("1 blocker")).toBeInTheDocument());
    expect(screen.getByText("0 warnings")).toBeInTheDocument();
    expect(screen.getByText("4 evidence items")).toBeInTheDocument();
  });

  it("VerifyStage renders no percent sign and no element with role progressbar", async () => {
    stubBridge(readyBridge);
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));

    await waitFor(() => expect(screen.getByText("0 blockers")).toBeInTheDocument());
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.queryByText(/%/)).not.toBeInTheDocument();
  });

  it("VerifyStage re-derives from live reads: mounting it twice produces two ListProgramming calls", async () => {
    const { listProgramming } = stubBridge(readyBridge);
    renderGuide();

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));
    await waitFor(() => expect(listProgramming).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole("button", { name: "Fixtures" }));
    fireEvent.click(screen.getByRole("button", { name: "Verify" }));

    await waitFor(() => expect(listProgramming).toHaveBeenCalledTimes(2));
  });

  it("a later stage is viewable while an earlier one reports a blocker", async () => {
    stubBridge({ ...readyBridge, fixtureRows: [] });
    renderGuide();

    const verifyRailButton = screen.getByRole("button", { name: "Verify" });
    expect(verifyRailButton).toBeEnabled();

    fireEvent.click(verifyRailButton);

    await waitFor(() => expect(screen.getByRole("heading", { name: "Verify" })).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Verify" })).toBeEnabled();
  });
});

describe("readiness derivations", () => {
  it("deriveProgramStatus: zero scenes yields exactly one blocker item", () => {
    const status = deriveProgramStatus(programmingView());
    expect(status.items).toHaveLength(1);
    expect(status.items[0].tone).toBe("blocker");
  });

  it("deriveProgramStatus: one scene yields an evidence item using singular language", () => {
    const status = deriveProgramStatus(programmingView({ scenes: [scene("Intro")] }));
    const evidenceItems = status.items.filter((item) => item.tone === "evidence");
    expect(evidenceItems).toHaveLength(1);
    expect(evidenceItems[0].detail).toMatch(/\b1 scene\b/);
    expect(evidenceItems[0].detail).not.toMatch(/\b1 scenes\b/);
  });

  it("deriveProgramStatus: three scenes yields a plural evidence item", () => {
    const status = deriveProgramStatus(
      programmingView({ scenes: [scene("A"), scene("B"), scene("C")] }),
    );
    const evidenceItems = status.items.filter((item) => item.tone === "evidence");
    expect(evidenceItems).toHaveLength(1);
    expect(evidenceItems[0].detail).toMatch(/\b3 scenes\b/);
  });

  it("deriveAssignStatus: zero surfaces yields a blocker plus exactly one optional MIDI evidence row", () => {
    const status = deriveAssignStatus(0, programmingView());
    const blockerItems = status.items.filter((item) => item.tone === "blocker");
    const midiItems = status.items.filter((item) => item.label.toLowerCase().includes("midi"));
    expect(blockerItems).toHaveLength(1);
    expect(midiItems).toHaveLength(1);
    expect(midiItems[0].tone).toBe("evidence");
  });

  it("deriveAssignStatus: at least one surface yields evidence plus exactly one optional MIDI evidence row", () => {
    const status = deriveAssignStatus(2, programmingView());
    const nonMidiEvidence = status.items.filter(
      (item) => item.tone === "evidence" && !item.label.toLowerCase().includes("midi"),
    );
    const midiItems = status.items.filter((item) => item.label.toLowerCase().includes("midi"));
    expect(nonMidiEvidence).toHaveLength(1);
    expect(nonMidiEvidence[0].detail).toMatch(/\b2 operator surfaces\b/);
    expect(midiItems).toHaveLength(1);
    expect(midiItems[0].tone).toBe("evidence");
  });

  it("no MIDI-related blocker is ever produced, regardless of surface count", () => {
    for (const count of [0, 1, 5]) {
      const status = deriveAssignStatus(count, programmingView());
      const midiBlockersOrWarnings = status.items.filter(
        (item) =>
          (item.tone === "blocker" || item.tone === "warning") &&
          item.label.toLowerCase().includes("midi"),
      );
      expect(midiBlockersOrWarnings).toHaveLength(0);
    }
  });

  it("aggregateReadiness returns independent counts and preserves every item", () => {
    const statuses: GuideStageStatus[] = [
      { items: [{ tone: "blocker", label: "A", detail: "a" }], primaryLabel: "x" },
      {
        items: [
          { tone: "warning", label: "B", detail: "b" },
          { tone: "evidence", label: "C", detail: "c" },
        ],
        primaryLabel: "y",
      },
      { items: [{ tone: "evidence", label: "D", detail: "d" }], primaryLabel: "z" },
    ];

    const rollup = aggregateReadiness(statuses);

    expect(rollup.blockers).toBe(1);
    expect(rollup.warnings).toBe(1);
    expect(rollup.evidence).toBe(2);
    expect(rollup.items).toHaveLength(4);
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
