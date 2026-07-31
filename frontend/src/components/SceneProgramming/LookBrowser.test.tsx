import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import LookBrowser from "./LookBrowser";
import type { ProgrammingView } from "../../lib/wailsBridge";

const emptyView: ProgrammingView = {
  scenes: [],
  themes: [],
  presets: [],
  chases: [],
  motions: [],
  blends: [],
  instances: [],
};

const filledView: ProgrammingView = {
  ...emptyView,
  themes: [{ id: "t1", name: "Sunset" }],
  chases: [{ id: "c1", name: "Sweep", stepUnit: "bar", stepDuration: 1 }],
  motions: [{ id: "m1", name: "Orbit" }],
  presets: [{ id: "p1", name: "Full Wash", kind: "intensity" }],
  blends: [{ id: "b1", name: "Cross Fade" }],
  instances: [{ id: "i1", label: "Fixture 1" }],
};

const noop = () => {};

const baseHandlers = {
  onCreateTheme: noop,
  onCreateMotion: noop,
  onCreateChase: noop,
  onCreateBlend: noop,
  onRecordPreset: noop,
  presetLoading: false,
  onRenameTheme: noop,
  onDeleteTheme: noop,
  onRenameMotion: noop,
  onDeleteMotion: noop,
  onRenamePreset: noop,
  onDeletePreset: noop,
  onUpdateChase: noop,
  onDeleteChase: noop,
  onRenameBlend: noop,
  onDeleteBlend: noop,
};

describe("LookBrowser", () => {
  afterEach(() => cleanup());

  it("shows the empty-state copy when there are no looks yet", () => {
    render(<LookBrowser view={emptyView} {...baseHandlers} />);
    expect(screen.getByText(/No looks yet/)).toBeInTheDocument();
    expect(screen.getByText("No blend presets yet.")).toBeInTheDocument();
  });

  it("summarizes counts and lists each look category when populated", () => {
    render(<LookBrowser view={filledView} {...baseHandlers} />);
    expect(screen.getByText("1 theme, 1 chase, 1 motion preset, 1 base-look preset")).toBeInTheDocument();
    expect(screen.getByText("Sunset")).toBeInTheDocument();
    expect(screen.getByText("Sweep")).toBeInTheDocument();
    expect(screen.getByText("Orbit")).toBeInTheDocument();
    expect(screen.getByText("Full Wash")).toBeInTheDocument();
  });

  it("submits a new theme via the toggled create-theme form", () => {
    const onCreateTheme = vi.fn();
    render(<LookBrowser view={emptyView} {...baseHandlers} onCreateTheme={onCreateTheme} />);

    fireEvent.click(screen.getByRole("button", { name: "+ Theme" }));
    fireEvent.change(screen.getByLabelText("New color theme name"), { target: { value: " Sunrise " } });
    fireEvent.click(screen.getByRole("button", { name: "Create Theme" }));

    expect(onCreateTheme).toHaveBeenCalledWith("Sunrise");
    expect(screen.queryByLabelText("New color theme name")).not.toBeInTheDocument();
  });

  it("only shows one create-form at a time, switching when a different category is toggled", () => {
    render(<LookBrowser view={emptyView} {...baseHandlers} />);

    fireEvent.click(screen.getByRole("button", { name: "+ Theme" }));
    expect(screen.getByLabelText("New color theme name")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "+ Motion" }));
    expect(screen.queryByLabelText("New color theme name")).not.toBeInTheDocument();
    expect(screen.getByLabelText("New motion preset name")).toBeInTheDocument();
  });

  it("does not record a preset when required fields are missing", () => {
    const onRecordPreset = vi.fn();
    render(<LookBrowser view={filledView} {...baseHandlers} onRecordPreset={onRecordPreset} />);

    fireEvent.click(screen.getByRole("button", { name: "+ Preset" }));
    fireEvent.click(screen.getByRole("button", { name: "Record Preset" }));
    expect(onRecordPreset).not.toHaveBeenCalled();
  });

  it("records a preset with the parsed attribute list once all fields are filled", () => {
    const onRecordPreset = vi.fn();
    render(<LookBrowser view={filledView} {...baseHandlers} onRecordPreset={onRecordPreset} />);

    fireEvent.click(screen.getByRole("button", { name: "+ Preset" }));
    fireEvent.change(screen.getByLabelText("Fixture instance"), { target: { value: "i1" } });
    fireEvent.change(screen.getByLabelText("Attribute assignments"), { target: { value: "intensity=100, color=red" } });
    fireEvent.change(screen.getByLabelText("Preset name"), { target: { value: "Full Bright" } });
    fireEvent.click(screen.getByRole("button", { name: "Record Preset" }));

    expect(onRecordPreset).toHaveBeenCalledWith("i1", ["intensity=100", "color=red"], "intensity", "Full Bright");
  });

  it("submits a new blend via the toggled create-blend form", () => {
    const onCreateBlend = vi.fn();
    render(<LookBrowser view={emptyView} {...baseHandlers} onCreateBlend={onCreateBlend} />);

    fireEvent.click(screen.getByRole("button", { name: "+ Blend" }));
    fireEvent.change(screen.getByLabelText("New blend name"), { target: { value: "Cross Fade" } });
    fireEvent.change(screen.getByLabelText("Blend duration (bars)"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: "Create Blend" }));

    expect(onCreateBlend).toHaveBeenCalledWith("Cross Fade", 2, "linear");
  });

  it("renames a theme via the inline rename control", () => {
    const onRenameTheme = vi.fn();
    render(<LookBrowser view={filledView} {...baseHandlers} onRenameTheme={onRenameTheme} />);

    fireEvent.click(screen.getByRole("button", { name: "Rename Sunset" }));
    fireEvent.change(screen.getByLabelText("Theme name"), { target: { value: "Sunset Renamed" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onRenameTheme).toHaveBeenCalledWith("Sunset", "Sunset Renamed");
  });

  it("deletes a blend preset via the delete control after confirming", () => {
    const onDeleteBlend = vi.fn();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    render(<LookBrowser view={filledView} {...baseHandlers} onDeleteBlend={onDeleteBlend} />);

    fireEvent.click(screen.getByRole("button", { name: "Delete Cross Fade" }));

    expect(onDeleteBlend).toHaveBeenCalledWith("Cross Fade");
    vi.restoreAllMocks();
  });

  it("updates a chase's name/unit/step-duration via the inline edit form", () => {
    const onUpdateChase = vi.fn();
    render(<LookBrowser view={filledView} {...baseHandlers} onUpdateChase={onUpdateChase} />);

    fireEvent.click(screen.getByRole("button", { name: "Edit Sweep" }));
    fireEvent.change(screen.getByLabelText("Chase name"), { target: { value: "Sweep Renamed" } });
    fireEvent.change(screen.getByLabelText("Chase step unit"), { target: { value: "beat" } });
    fireEvent.change(screen.getByLabelText("Chase step duration"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onUpdateChase).toHaveBeenCalledWith("Sweep", "Sweep Renamed", "beat", 2);
  });
});
