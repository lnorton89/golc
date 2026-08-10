// Desk.test.tsx is the focused regression suite for Plan 13-13's design-
// system migration: initial render/empty state, the local-override
// dispatch paths (per-channel/per-fixture/per-universe/all), and the MIDI
// Learn start/cancel/conflict/timeout machine all stay intact through the
// conversion. Mocks window.go.wails directly, mirroring every other
// Wails-bridge test in this codebase (see wailsBridge.ts's own doc
// comment) -- not exhaustive geometry/clamp()/resize coverage, which is
// Plan 13-13's own scoped design-system checker exceptions' job, not this
// suite's.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "../../test/renderWithProviders";

import Desk from "./Desk";
import { useGolcStore } from "../../store/store";

function ok(stdout = "") {
  return { exitCode: 0, stdout, stderr: "" };
}

function patchView(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    pools: [{ id: "pool-1", name: "Wash", members: [{ id: "member-1", fixtureStableKey: "acme-par64", fixtureContentHash: "hash-1" }] }],
    deployments: [
      {
        id: "dep-1",
        name: "Main Rig",
        active: true,
        instances: [{ id: "inst-1", poolId: "pool-1", poolMemberId: "member-1", mode: "1ch", universe: 1, address: 1 }],
      },
    ],
    ...overrides,
  };
}

function libraryView() {
  return {
    directory: "",
    rows: [
      {
        stableKey: "acme-par64",
        contentHash: "hash-1",
        manufacturer: "Acme",
        model: "PAR64",
        modes: ["1ch"],
        modeChannelCounts: { "1ch": 1 },
        modeChannels: { "1ch": [{ index: 0, type: "intensity", occurrence: 0 }] },
        fileName: "acme-par64.yaml",
        source: "local",
        status: "valid",
        detail: "",
      },
    ],
  };
}

describe("Desk", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.stubGlobal("go", {
      wails: {
        FixturePatchService: { ListPatch: vi.fn().mockResolvedValue(patchView()) },
        FixtureLibraryService: { ListLocal: vi.fn().mockResolvedValue(libraryView()) },
        ShowService: { GetImageDataURI: vi.fn().mockResolvedValue("") },
        DeskService: {
          SetAttribute: vi.fn().mockResolvedValue(ok()),
          ClearAttribute: vi.fn().mockResolvedValue(ok()),
          ClearInstance: vi.fn().mockResolvedValue(ok()),
          ClearAll: vi.fn().mockResolvedValue(ok()),
          FetchUniverseValues: vi.fn().mockResolvedValue([]),
        },
        MidiService: {
          ListDeskMappings: vi.fn().mockResolvedValue([]),
          StartDeskLearn: vi.fn().mockResolvedValue(ok()),
          CancelLearn: vi.fn().mockResolvedValue(ok()),
          RemoveDeskMapping: vi.fn().mockResolvedValue(ok()),
        },
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    useGolcStore.setState({ midiLearnMode: false });
  });

  function svc() {
    return (
      window as unknown as {
        go: { wails: { DeskService: Record<string, ReturnType<typeof vi.fn>>; MidiService: Record<string, ReturnType<typeof vi.fn>> } };
      }
    ).go.wails;
  }

  it("shows the empty state when no fixtures are patched into the active deployment", async () => {
    (
      window as unknown as { go: { wails: { FixturePatchService: { ListPatch: ReturnType<typeof vi.fn> } } } }
    ).go.wails.FixturePatchService.ListPatch.mockResolvedValue(patchView({ deployments: [] }));

    render(<Desk />);
    await waitFor(() => expect(screen.getByText("No patched fixtures in the active deployment")).toBeInTheDocument());
  });

  it("renders a patched fixture's fader and dispatches setDeskAttribute on change", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.change(fader, { target: { value: "128" } });

    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalledWith("inst-1", "intensity", 128 / 255));
  });

  it("renders a ColorField for a fixture with color_red/green/blue channels and dispatches all three on change", async () => {
    (
      window as unknown as { go: { wails: { FixtureLibraryService: { ListLocal: ReturnType<typeof vi.fn> } } } }
    ).go.wails.FixtureLibraryService.ListLocal.mockResolvedValue({
      directory: "",
      rows: [
        {
          stableKey: "acme-rgb64",
          contentHash: "hash-2",
          manufacturer: "Acme",
          model: "RGB64",
          modes: ["3ch"],
          modeChannelCounts: { "3ch": 3 },
          modeChannels: {
            "3ch": [
              { index: 0, type: "color_red", occurrence: 0 },
              { index: 1, type: "color_green", occurrence: 0 },
              { index: 2, type: "color_blue", occurrence: 0 },
            ],
          },
          fileName: "acme-rgb64.yaml",
          source: "local",
          status: "valid",
          detail: "",
        },
      ],
    });
    (
      window as unknown as { go: { wails: { FixturePatchService: { ListPatch: ReturnType<typeof vi.fn> } } } }
    ).go.wails.FixturePatchService.ListPatch.mockResolvedValue(
      patchView({
        pools: [{ id: "pool-1", name: "Wash", members: [{ id: "member-1", fixtureStableKey: "acme-rgb64", fixtureContentHash: "hash-2" }] }],
        deployments: [
          {
            id: "dep-1",
            name: "Main Rig",
            active: true,
            instances: [{ id: "inst-1", poolId: "pool-1", poolMemberId: "member-1", mode: "3ch", universe: 1, address: 1 }],
          },
        ],
      }),
    );

    render(<Desk />);
    const swatch = await screen.findByRole("button", { name: /color$/ });
    fireEvent.click(swatch);

    // NumberStepper's Base UI NumberField.Input renders type="text" with an
    // aria-roledescription of "Number field" rather than the implicit
    // "spinbutton" role a native type="number" input carried -- query by
    // the plain textbox role that type="text" now exposes.
    const redField = await screen.findByRole("textbox", { name: /red channel/ });
    fireEvent.change(redField, { target: { value: "200" } });

    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalledWith("inst-1", "color_red", 200 / 255));
    expect(svc().DeskService.SetAttribute).toHaveBeenCalledWith("inst-1", "color_green", 0);
    expect(svc().DeskService.SetAttribute).toHaveBeenCalledWith("inst-1", "color_blue", 0);
  });

  it("dispatches clearDeskAttribute from a fader's own clear button once overridden", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });
    fireEvent.change(fader, { target: { value: "64" } });
    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalled());

    fireEvent.click(screen.getByTitle("Release override back to programmed output"));

    await waitFor(() => expect(svc().DeskService.ClearAttribute).toHaveBeenCalledWith("inst-1", "intensity"));
  });

  it("releases every override on one fixture instance", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });
    fireEvent.change(fader, { target: { value: "64" } });
    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Release every override on Wash" }));

    await waitFor(() => expect(svc().DeskService.ClearInstance).toHaveBeenCalledWith("inst-1"));
  });

  it("releases every override in a universe", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });
    fireEvent.change(fader, { target: { value: "64" } });
    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Release every override in Universe 1" }));

    await waitFor(() => expect(svc().DeskService.ClearInstance).toHaveBeenCalledWith("inst-1"));
  });

  it("releases every override across the whole desk", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });
    fireEvent.change(fader, { target: { value: "64" } });
    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Release All" }));

    await waitFor(() => expect(svc().DeskService.ClearAll).toHaveBeenCalled());
  });

  // 2026-08-10 review pass: handleClearAll cleared `overrides` but not
  // `touchedKeys`, unlike the per-channel/per-fixture/per-universe release
  // paths -- so every previously-dragged thumb stayed accent blue (which
  // per Fader.tsx's contract means "currently doing something") while its
  // value had already fallen back to live.
  it("resets every fader thumb out of the touched state on release-all, like the narrower releases already do", async () => {
    render(<Desk />);
    const fader = await screen.findByRole("slider", { name: /Intensity/ });
    fireEvent.change(fader, { target: { value: "64" } });
    await waitFor(() => expect(svc().DeskService.SetAttribute).toHaveBeenCalled());

    const touchedClass = fader.className;
    fireEvent.click(screen.getByRole("button", { name: "Release All" }));
    await waitFor(() => expect(svc().DeskService.ClearAll).toHaveBeenCalled());

    await waitFor(() => expect(fader.className).not.toBe(touchedClass));
    // Same resting appearance a per-channel release produces.
    fireEvent.change(fader, { target: { value: "70" } });
    await waitFor(() => expect(fader.className).toBe(touchedClass));
    fireEvent.click(screen.getByTitle("Release override back to programmed output"));
    await waitFor(() => expect(svc().DeskService.ClearAttribute).toHaveBeenCalled());
    const afterPerChannelRelease = fader.className;

    fireEvent.change(fader, { target: { value: "80" } });
    fireEvent.click(screen.getByRole("button", { name: "Release All" }));
    await waitFor(() => expect(fader.className).toBe(afterPerChannelRelease));
  });

  it("starts a MIDI learn capture on click and refreshes mappings once it resolves", async () => {
    useGolcStore.setState({ midiLearnMode: true });
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByLabelText(/Learn MIDI mapping for Intensity/));

    await waitFor(() => expect(svc().MidiService.StartDeskLearn).toHaveBeenCalledWith("inst-1", "intensity"));
    await waitFor(() => expect(svc().MidiService.ListDeskMappings).toHaveBeenCalledTimes(2));
  });

  it("shows the conflict message when StartDeskLearn reports a mapping conflict", async () => {
    svc().MidiService.StartDeskLearn.mockResolvedValue({
      exitCode: 1,
      stdout: "",
      stderr: "GOLC_DESKMIDI_MAPPING_CONFLICT: already mapped to Channel Fader",
    });
    useGolcStore.setState({ midiLearnMode: true });
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByLabelText(/Learn MIDI mapping for Intensity/));

    await waitFor(() => expect(screen.getByText("already mapped to Channel Fader")).toBeInTheDocument());
  });

  it("shows the timeout message when StartDeskLearn reports no MIDI input", async () => {
    svc().MidiService.StartDeskLearn.mockResolvedValue({
      exitCode: 1,
      stdout: "",
      stderr: "GOLC_MIDI_LEARN_TIMEOUT",
    });
    useGolcStore.setState({ midiLearnMode: true });
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByLabelText(/Learn MIDI mapping for Intensity/));

    await waitFor(() => expect(screen.getByText("No MIDI input received. Try again.")).toBeInTheDocument());
  });

  it("cancels an in-flight MIDI learn capture", async () => {
    svc().MidiService.StartDeskLearn.mockReturnValue(new Promise(() => {}));
    useGolcStore.setState({ midiLearnMode: true });
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByLabelText(/Learn MIDI mapping for Intensity/));
    await waitFor(() => expect(svc().MidiService.StartDeskLearn).toHaveBeenCalled());

    fireEvent.click(screen.getByLabelText(/Cancel MIDI learn for Intensity/));

    await waitFor(() => expect(svc().MidiService.CancelLearn).toHaveBeenCalled());
  });

  it("opens FixtureStyleModal from a fixture's customize button and closes it on Cancel", async () => {
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByRole("button", { name: "Customize Wash" }));
    expect(await screen.findByRole("dialog", { name: "Customize card" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Customize card" })).not.toBeInTheDocument());
  });

  it("saves a fixture style customization and reflects it without a page reload", async () => {
    render(<Desk />);
    await screen.findByRole("slider", { name: /Intensity/ });

    fireEvent.click(screen.getByRole("button", { name: "Customize Wash" }));
    const dialog = await screen.findByRole("dialog", { name: "Customize card" });

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Customize card" })).not.toBeInTheDocument());
    expect(dialog).not.toBeInTheDocument();
    expect(JSON.parse(window.localStorage.getItem("golc.deskFixtureStyles") ?? "{}")).toHaveProperty("inst-1");
  });
});
