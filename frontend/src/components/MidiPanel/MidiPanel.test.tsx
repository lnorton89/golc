// MidiPanel.test.tsx is the focused regression suite for Plan 13-28's
// design-system migration of the generic MIDI mapping feature: surface
// selection, the assigned-controls Learn list, the mapping list's
// remove/soft-takeover/armed-chip rendering, and Desk mappings' own
// remove flow all stay intact through the conversion. Mocks
// window.go.wails directly, mirroring every other Wails-bridge test in
// this codebase (see Desk.test.tsx / wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import MidiPanel from "./MidiPanel";
import { useGolcStore } from "../../store/store";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function svc() {
  return (
    window as unknown as {
      go: {
        wails: {
          SurfaceService: Record<string, ReturnType<typeof vi.fn>>;
          MidiService: Record<string, ReturnType<typeof vi.fn>>;
        };
      };
    }
  ).go.wails;
}

describe("MidiPanel", () => {
  beforeEach(() => {
    useGolcStore.setState({ connectionStatus: "connected" });
    vi.stubGlobal("go", {
      wails: {
        SurfaceService: {
          ListSurfaces: vi.fn().mockResolvedValue([{ id: "surface-1", name: "Booth" }]),
          ShowSurface: vi.fn().mockResolvedValue({
            controls: [{ kind: "scene", scene: "Opening", label: "Opening scene", assigned: true }],
          }),
        },
        MidiService: {
          ListMappings: vi.fn().mockResolvedValue([]),
          SetActiveSurface: vi.fn().mockResolvedValue(ok()),
          RemoveMapping: vi.fn().mockResolvedValue(ok()),
          StartLearn: vi.fn().mockResolvedValue(ok()),
          CancelLearn: vi.fn().mockResolvedValue(ok()),
          ListDeskMappings: vi.fn().mockResolvedValue([]),
          RemoveDeskMapping: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
    vi.stubGlobal("confirm", vi.fn().mockReturnValue(true));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    useGolcStore.setState({ connectionStatus: "connecting", surfaceListVersion: 0 });
  });

  // The operator-surface picker is the shared Select primitive (Base
  // UI-backed): opening it and choosing an option needs realistic
  // pointer interaction via userEvent, not a bare fireEvent.change on a
  // native <select> (there isn't one anymore).
  async function selectSurface() {
    const user = userEvent.setup();
    render(<MidiPanel />);
    const trigger = await screen.findByRole("combobox", { name: "Operator surface" });
    await user.click(trigger);
    const option = await screen.findByRole("option", { name: "Booth" });
    await user.click(option);
    return trigger;
  }

  // --- 2026-08-10 review pass regressions ------------------------------

  /** deferred lets a test choose the order two in-flight reads resolve. */
  function deferred<T>() {
    let resolve!: (value: T) => void;
    const promise = new Promise<T>((r) => {
      resolve = r;
    });
    return { promise, resolve };
  }

  it("keeps the newer surface's controls when the older detail read resolves last", async () => {
    const boothDetail = deferred<unknown>();
    const stageDetail = deferred<unknown>();
    svc().SurfaceService.ListSurfaces.mockResolvedValue([
      { id: "surface-1", name: "Booth" },
      { id: "surface-2", name: "Stage" },
    ]);
    svc().SurfaceService.ShowSurface.mockImplementation((name: string) =>
      name === "Booth" ? boothDetail.promise : stageDetail.promise,
    );

    const user = userEvent.setup();
    render(<MidiPanel />);
    const trigger = await screen.findByRole("combobox", { name: "Operator surface" });

    await user.click(trigger);
    await user.click(await screen.findByRole("option", { name: "Booth" }));
    await waitFor(() => expect(svc().SurfaceService.ShowSurface).toHaveBeenCalledWith("Booth"));

    await user.click(trigger);
    await user.click(await screen.findByRole("option", { name: "Stage" }));
    await waitFor(() => expect(svc().SurfaceService.ShowSurface).toHaveBeenCalledWith("Stage"));

    // Stage lands first; the slower Booth response arrives afterwards.
    stageDetail.resolve({ controls: [{ kind: "scene", scene: "Finale", label: "Stage control", assigned: true }] });
    expect(await screen.findByText("Stage control")).toBeInTheDocument();

    boothDetail.resolve({ controls: [{ kind: "scene", scene: "Opening", label: "Booth control", assigned: true }] });
    await waitFor(() => expect(svc().SurfaceService.ShowSurface).toHaveBeenCalledTimes(2));

    expect(screen.getByText("Stage control")).toBeInTheDocument();
    expect(screen.queryByText("Booth control")).not.toBeInTheDocument();
  });

  it("deselects a surface that disappears from the list (deleted elsewhere in the app)", async () => {
    await selectSurface();
    await screen.findByText("Opening scene");

    // OperatorSurface.tsx deletes the surface and bumps the shared
    // invalidation counter; the list re-fetches without it.
    svc().SurfaceService.ListSurfaces.mockResolvedValue([]);
    useGolcStore.getState().bumpSurfaceListVersion();

    // The assigned-controls section collapses instead of sitting on a
    // GOLC_OPERATORSURFACE_NOT_FOUND error banner for a dangling value.
    await waitFor(() => expect(screen.queryByText("Opening scene")).not.toBeInTheDocument());
  });

  it("does not report a timeout for a learn the operator cancelled", async () => {
    // CancelLearn closes session.cancel on the Go side, which unblocks the
    // still-pending StartLearn so it resolves with
    // GOLC_MIDI_LEARN_TIMEOUT a moment later. That late resolution used to
    // walk into the timeout branch and put "No MIDI input received. Try
    // again." under the button for a deliberately aborted learn.
    let releaseLearn!: (value: unknown) => void;
    svc().MidiService.StartLearn.mockReturnValue(
      new Promise((resolve) => {
        releaseLearn = resolve;
      }),
    );

    await selectSurface();
    await screen.findByText("Opening scene");

    fireEvent.click(screen.getByLabelText("Learn MIDI mapping for Opening scene"));
    await waitFor(() => expect(svc().MidiService.StartLearn).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(svc().MidiService.CancelLearn).toHaveBeenCalled());

    releaseLearn({ exitCode: 1, stdout: "", stderr: "GOLC_MIDI_LEARN_TIMEOUT: no MIDI input" });
    await waitFor(() =>
      expect(screen.getByLabelText("Learn MIDI mapping for Opening scene")).toBeInTheDocument(),
    );
    expect(screen.queryByText("No MIDI input received. Try again.")).not.toBeInTheDocument();
  });

  it("hands keyboard focus to Cancel while listening and returns it to Learn afterwards", async () => {
    let releaseLearn!: (value: unknown) => void;
    svc().MidiService.StartLearn.mockReturnValue(
      new Promise((resolve) => {
        releaseLearn = resolve;
      }),
    );

    await selectSurface();
    await screen.findByText("Opening scene");

    const learnButton = screen.getByLabelText("Learn MIDI mapping for Opening scene");
    learnButton.focus();
    expect(learnButton).toHaveFocus();

    fireEvent.click(learnButton);

    // The focused Learn button unmounts and a role="status" region plus a
    // Cancel button replaces it; without a handoff focus fell to <body>.
    await waitFor(() => expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus());

    releaseLearn({ exitCode: 0, stdout: "", stderr: "" });

    await waitFor(() =>
      expect(screen.getByLabelText("Learn MIDI mapping for Opening scene")).toHaveFocus(),
    );
  });

  it("shows the empty state for Desk mappings when none exist", async () => {
    render(<MidiPanel />);
    expect(await screen.findByText("No Desk mappings yet")).toBeInTheDocument();
  });

  it("lists surfaces and refreshes assigned controls + mappings once one is selected", async () => {
    await selectSurface();

    await waitFor(() => expect(svc().MidiService.SetActiveSurface).toHaveBeenCalledWith("Booth"));
    expect(await screen.findByText("Opening scene")).toBeInTheDocument();
  });

  it("starts MIDI learn for an assigned control and refreshes on success", async () => {
    await selectSurface();
    await screen.findByText("Opening scene");

    fireEvent.click(screen.getByLabelText("Learn MIDI mapping for Opening scene"));

    await waitFor(() =>
      expect(svc().MidiService.StartLearn).toHaveBeenCalledWith("Booth", { kind: "scene", scene: "Opening" }),
    );
    await waitFor(() => expect(svc().SurfaceService.ShowSurface).toHaveBeenCalledTimes(2));
  });

  it("shows the conflict message when StartLearn reports a mapping conflict", async () => {
    svc().MidiService.StartLearn.mockResolvedValue({
      exitCode: 1,
      stdout: "",
      stderr: "GOLC_MIDI_MAPPING_CONFLICT: already mapped to Blackout",
    });
    await selectSurface();
    await screen.findByText("Opening scene");

    fireEvent.click(screen.getByLabelText("Learn MIDI mapping for Opening scene"));

    expect(await screen.findByText("already mapped to Blackout")).toBeInTheDocument();
  });

  it("renders a control_change mapping with the soft-takeover slider and a note mapping with an armed chip", async () => {
    svc().MidiService.ListMappings.mockResolvedValue([
      { id: "map-1", channel: 1, kind: "control_change", number: 20, target: { kind: "scene", scene: "Opening" }, label: "Opening scene" },
      { id: "map-2", channel: 1, kind: "note", number: 36, target: { kind: "safety", safety: "blackout" }, label: "Blackout" },
    ]);
    await selectSurface();

    expect(await screen.findByText("MIDI waiting: control 0, target 0.")).toBeInTheDocument();
    expect(screen.getByText("Not armed")).toBeInTheDocument();
  });

  it("removes a mapping after destructive confirmation", async () => {
    svc().MidiService.ListMappings.mockResolvedValue([
      { id: "map-1", channel: 1, kind: "note", number: 36, target: { kind: "safety", safety: "blackout" }, label: "Blackout" },
    ]);
    await selectSurface();
    await screen.findByText("Blackout");

    fireEvent.click(screen.getByLabelText("Remove mapping from Blackout"));

    await waitFor(() => expect(svc().MidiService.RemoveMapping).toHaveBeenCalledWith("Booth", "map-1"));
  });

  it("removes a Desk mapping after destructive confirmation", async () => {
    svc().MidiService.ListDeskMappings.mockResolvedValue([
      { id: "desk-map-1", channel: 1, kind: "control_change", number: 7, instanceId: "inst-1", capability: "intensity" },
    ]);
    render(<MidiPanel />);

    const removeButton = await screen.findByLabelText(/^Remove mapping from /);
    fireEvent.click(removeButton);

    await waitFor(() => expect(svc().MidiService.RemoveDeskMapping).toHaveBeenCalledWith("desk-map-1"));
  });

  it("shows the empty state when no controls are assigned to the selected surface", async () => {
    svc().SurfaceService.ShowSurface.mockResolvedValue({ controls: [] });
    await selectSurface();

    expect(await screen.findByText("No controls assigned")).toBeInTheDocument();
  });

  it("shows an ErrorState when ListSurfaces fails", async () => {
    svc().SurfaceService.ListSurfaces.mockRejectedValue(new Error("daemon unreachable"));
    render(<MidiPanel />);

    expect(await screen.findByText("daemon unreachable")).toBeInTheDocument();
  });
});

// Confirms the loading skeleton renders while the daemon connection is
// still establishing (a separate describe block since it needs its own
// connectionStatus, set before the shared beforeEach above would flip it).
describe("MidiPanel while connecting", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    useGolcStore.setState({ connectionStatus: "connecting" });
  });

  it("shows a loading state while the daemon connection is establishing", () => {
    useGolcStore.setState({ connectionStatus: "connecting" });
    vi.stubGlobal("go", { wails: {} });

    render(<MidiPanel />);

    expect(screen.getByText("Loading MIDI mappings")).toBeInTheDocument();
  });
});
