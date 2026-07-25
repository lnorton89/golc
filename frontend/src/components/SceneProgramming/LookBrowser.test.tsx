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
  chases: [{ id: "c1", name: "Sweep" }],
  motions: [{ id: "m1", name: "Orbit" }],
  presets: [{ id: "p1", name: "Full Wash", kind: "intensity" }],
  instances: [{ id: "i1", label: "Fixture 1" }],
};

const noop = () => {};

describe("LookBrowser", () => {
  afterEach(() => cleanup());

  it("shows the empty-state copy when there are no looks yet", () => {
    render(
      <LookBrowser
        view={emptyView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={noop}
        presetLoading={false}
      />,
    );
    expect(screen.getByText(/No looks yet/)).toBeInTheDocument();
    expect(screen.getByText("No blend presets yet.")).toBeInTheDocument();
  });

  it("summarizes counts and lists each look category when populated", () => {
    render(
      <LookBrowser
        view={filledView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={noop}
        presetLoading={false}
      />,
    );
    expect(screen.getByText("1 theme, 1 chase, 1 motion preset, 1 base-look preset")).toBeInTheDocument();
    expect(screen.getByText("Sunset")).toBeInTheDocument();
    expect(screen.getByText("Sweep")).toBeInTheDocument();
    expect(screen.getByText("Orbit")).toBeInTheDocument();
    expect(screen.getByText("Full Wash")).toBeInTheDocument();
  });

  it("submits a new theme via the toggled create-theme form", () => {
    const onCreateTheme = vi.fn();
    render(
      <LookBrowser
        view={emptyView}
        onCreateTheme={onCreateTheme}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={noop}
        presetLoading={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "+ Theme" }));
    fireEvent.change(screen.getByLabelText("New color theme name"), { target: { value: " Sunrise " } });
    fireEvent.click(screen.getByRole("button", { name: "Create Theme" }));

    expect(onCreateTheme).toHaveBeenCalledWith("Sunrise");
    expect(screen.queryByLabelText("New color theme name")).not.toBeInTheDocument();
  });

  it("only shows one create-form at a time, switching when a different category is toggled", () => {
    render(
      <LookBrowser
        view={emptyView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={noop}
        presetLoading={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "+ Theme" }));
    expect(screen.getByLabelText("New color theme name")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "+ Motion" }));
    expect(screen.queryByLabelText("New color theme name")).not.toBeInTheDocument();
    expect(screen.getByLabelText("New motion preset name")).toBeInTheDocument();
  });

  it("does not record a preset when required fields are missing", () => {
    const onRecordPreset = vi.fn();
    render(
      <LookBrowser
        view={filledView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={onRecordPreset}
        presetLoading={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "+ Preset" }));
    fireEvent.click(screen.getByRole("button", { name: "Record Preset" }));
    expect(onRecordPreset).not.toHaveBeenCalled();
  });

  it("records a preset with the parsed attribute list once all fields are filled", () => {
    const onRecordPreset = vi.fn();
    render(
      <LookBrowser
        view={filledView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={noop}
        onRecordPreset={onRecordPreset}
        presetLoading={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "+ Preset" }));
    fireEvent.change(screen.getByLabelText("Fixture instance"), { target: { value: "i1" } });
    fireEvent.change(screen.getByLabelText("Attribute assignments"), { target: { value: "intensity=100, color=red" } });
    fireEvent.change(screen.getByLabelText("Preset name"), { target: { value: "Full Bright" } });
    fireEvent.click(screen.getByRole("button", { name: "Record Preset" }));

    expect(onRecordPreset).toHaveBeenCalledWith("i1", ["intensity=100", "color=red"], "intensity", "Full Bright");
  });

  it("submits a new blend via the toggled create-blend form", () => {
    const onCreateBlend = vi.fn();
    render(
      <LookBrowser
        view={emptyView}
        onCreateTheme={noop}
        onCreateMotion={noop}
        onCreateChase={noop}
        onCreateBlend={onCreateBlend}
        onRecordPreset={noop}
        presetLoading={false}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "+ Blend" }));
    fireEvent.change(screen.getByLabelText("New blend name"), { target: { value: "Cross Fade" } });
    fireEvent.change(screen.getByLabelText("Blend duration (bars)"), { target: { value: "2" } });
    fireEvent.click(screen.getByRole("button", { name: "Create Blend" }));

    expect(onCreateBlend).toHaveBeenCalledWith("Cross Fade", 2, "linear");
  });
});
