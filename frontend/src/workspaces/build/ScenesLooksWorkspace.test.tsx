// ScenesLooksWorkspace.test.tsx exercises the orchestrator end-to-end
// against a mocked window.go.wails.ProgrammingService, the same
// direct-window-object convention every other Wails-bridge test in this
// codebase uses (see wailsBridge.ts's own doc comment).
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ScenesLooksWorkspace from "./ScenesLooksWorkspace";

function view(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    scenes: [
      {
        name: "Alpha",
        active: true,
        barsPerLoop: 4,
        layers: [{ kind: "base_look", enabled: true, ref: "preset-1" }],
      },
      { name: "Beta", active: false, barsPerLoop: 8, layers: [] },
    ],
    themes: [],
    presets: [{ id: "preset-1", name: "Full Wash", kind: "intensity" }],
    chases: [],
    motions: [],
    blends: [],
    instances: [],
    ...overrides,
  };
}

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

describe("ScenesLooksWorkspace", () => {
  beforeEach(() => {
    vi.stubGlobal("go", {
      wails: {
        ProgrammingService: {
          ListProgramming: vi.fn().mockResolvedValue(view()),
          CreateScene: vi.fn().mockResolvedValue(ok()),
          ActivateScene: vi.fn().mockResolvedValue(ok()),
          SetSceneLayer: vi.fn().mockResolvedValue(ok()),
          ReorderScenes: vi.fn().mockResolvedValue(ok()),
          CreateTheme: vi.fn().mockResolvedValue(ok()),
          CreateMotion: vi.fn().mockResolvedValue(ok()),
          CreateChase: vi.fn().mockResolvedValue(ok()),
          CreateBlend: vi.fn().mockResolvedValue(ok()),
          ProgrammerSet: vi.fn().mockResolvedValue(ok()),
          RecordPreset: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads and displays scenes, defaulting the selection to the active scene", async () => {
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());
    expect(screen.getByLabelText("Scene stack")).toBeInTheDocument();
    expect(screen.getAllByText("LIVE")).toHaveLength(2);
  });

  it("switches the displayed layers when a different scene is selected", async () => {
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    await waitFor(() => expect(screen.getByLabelText("Beta layers")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Activate" })).toBeInTheDocument();
  });

  it("activates a non-live scene via the Activate button", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    await waitFor(() => screen.getByRole("button", { name: "Activate" }));
    fireEvent.click(screen.getByRole("button", { name: "Activate" }));

    await waitFor(() => expect(svc.ActivateScene).toHaveBeenCalledWith("Beta"));
  });

  it("toggles a layer's enabled state on the selected scene", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Base Look" }));
    await waitFor(() => expect(svc.SetSceneLayer).toHaveBeenCalledWith("Alpha", "base_look", "", false));
  });

  it("creates a new scene from the SceneList form and selects it", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    svc.ListProgramming
      .mockResolvedValueOnce(view())
      .mockResolvedValueOnce(
        view({
          scenes: [
            ...view().scenes,
            { name: "Gamma", active: false, barsPerLoop: 4, layers: [] },
          ],
        }),
      );

    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "New" }));
    fireEvent.change(screen.getByLabelText("New scene name"), { target: { value: "Gamma" } });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(svc.CreateScene).toHaveBeenCalledWith("Gamma", 4));
    await waitFor(() => expect(screen.getByLabelText("Gamma layers")).toBeInTheDocument());
  });

  it("persists a drag-to-reorder via ReorderScenes, translating names to original 0-based indices, then refreshes", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;

    // Same rect-stubbing trick as SceneList.test.tsx: jsdom never computes
    // real layout, so dnd-kit's collision detection needs a stacked rect
    // per <li> (ordered by DOM position) to tell the rows apart.
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
      if (this.tagName === "LI") {
        const siblings = Array.from(this.parentElement?.children ?? []);
        const index = siblings.indexOf(this);
        const rectTop = index * 44;
        return {
          width: 260,
          height: 44,
          top: rectTop,
          left: 0,
          right: 260,
          bottom: rectTop + 44,
          x: 0,
          y: rectTop,
          toJSON: () => ({}),
        } as DOMRect;
      }
      return { width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
    });

    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    // Alpha is at original index 0, Beta at original index 1. Dragging
    // Alpha down past Beta must persist as --order 1,0 (Beta's original
    // index first, then Alpha's).
    const handle = screen.getByRole("button", { name: "Reorder Alpha" });
    handle.focus();
    await act(async () => {
      fireEvent.keyDown(handle, { code: "Space" });
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });
    await act(async () => {
      fireEvent.keyDown(handle, { code: "ArrowDown" });
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });
    await act(async () => {
      fireEvent.keyDown(handle, { code: "Space" });
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });

    await waitFor(() => expect(svc.ReorderScenes).toHaveBeenCalledWith([1, 0]));
    await waitFor(() => expect(svc.ListProgramming).toHaveBeenCalledTimes(2));

    vi.restoreAllMocks();
  });

  // --- 2026-08-10 review pass regressions ------------------------------

  function svcOf() {
    return (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
  }

  it("keeps the renamed scene selected instead of jumping to the active/first scene", async () => {
    const svc = svcOf();
    const renamedView = view({
      scenes: [
        { name: "Alpha", active: true, barsPerLoop: 4, layers: [{ kind: "base_look", enabled: true, ref: "preset-1" }] },
        { name: "Renamed", active: false, barsPerLoop: 8, layers: [] },
      ],
    });
    svc.ListProgramming.mockResolvedValueOnce(view()).mockResolvedValue(renamedView);
    svc.RenameScene = vi.fn().mockResolvedValue(ok());

    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    // Select the non-live scene, then rename it.
    fireEvent.click(screen.getByRole("button", { name: "Beta8bar" }));
    await waitFor(() => expect(screen.getByLabelText("Beta layers")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Beta actions" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Rename" }));
    fireEvent.change(screen.getByLabelText("Scene name"), { target: { value: "Renamed" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(svc.RenameScene).toHaveBeenCalledWith("Beta", "Renamed"));
    // Setting the selection before the refresh let the selection-validity
    // effect see a name with no match and overwrite it with the live scene.
    await waitFor(() => expect(screen.getByLabelText("Renamed layers")).toBeInTheDocument());
    expect(screen.queryByLabelText("Alpha layers")).not.toBeInTheDocument();
  });

  it("does not enable a disabled layer just because a look was picked for it", async () => {
    const svc = svcOf();
    svc.ListProgramming.mockResolvedValue(
      view({
        scenes: [
          {
            name: "Alpha",
            active: true,
            barsPerLoop: 4,
            // Base Look is OFF but its picker stays live (LayerRow's
            // contract), so pre-staging a look must not switch it on.
            layers: [{ kind: "base_look", enabled: false, ref: "" }],
          },
        ],
      }),
    );

    render(<ScenesLooksWorkspace />);
    await waitFor(() => expect(screen.getByLabelText("Alpha layers")).toBeInTheDocument());

    const picker = screen.getByLabelText("Base Look look");
    fireEvent.change(picker, { target: { value: "preset-1" } });

    await waitFor(() => expect(svc.SetSceneLayer).toHaveBeenCalledWith("Alpha", "base_look", "preset-1", false));
  });

  it("shows an empty state when there are no scenes yet", async () => {
    const svc = (window as unknown as { go: { wails: { ProgrammingService: Record<string, ReturnType<typeof vi.fn>> } } })
      .go.wails.ProgrammingService;
    svc.ListProgramming.mockResolvedValue(view({ scenes: [] }));

    render(<ScenesLooksWorkspace />);
    await waitFor(() =>
      expect(
        screen.getByText("Create a scene to start pointing its layers at reusable looks."),
      ).toBeInTheDocument(),
    );
  });
});
