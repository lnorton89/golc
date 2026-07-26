// ScriptRunDialog.test.tsx covers 08-10-PLAN.md Task 2's every
// <behavior> bullet: mode-specific title/CTA, pre-filled-never-blank
// capability/preset fields, preset-gated Advanced numeric fields,
// submit/cancel/Escape/backdrop-close, the in-flight busy state, and an
// inline error on a failed launch.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

import ScriptRunDialog, { type ScriptDialogProfile } from "./ScriptRunDialog";

function profile(overrides: Partial<ScriptDialogProfile> = {}): ScriptDialogProfile {
  return {
    scope: "playback",
    preset: "quick-action",
    deadlineSeconds: 30,
    ratePerSecond: 20,
    memoryLimitMB: 256,
    cpuCapPercent: 25,
    ...overrides,
  };
}

describe("ScriptRunDialog", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the run-mode title, a 'Run' CTA, and a Cancel action", () => {
    render(
      <ScriptRunDialog
        mode="run"
        scriptName="Chase Cycler"
        profile={profile()}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Run Chase Cycler")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Run Chase Cycler$/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("renders the debug-mode title and a 'Start Debugging' CTA", () => {
    render(
      <ScriptRunDialog
        mode="debug"
        scriptName="Chase Cycler"
        profile={profile()}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Debug Chase Cycler")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Start Debugging Chase Cycler$/ })).toBeInTheDocument();
  });

  it("carries role=dialog and aria-modal=true, and moves focus into the dialog on open", () => {
    render(
      <ScriptRunDialog
        mode="run"
        scriptName="Chase Cycler"
        profile={profile()}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
      />,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveFocus();
  });

  it("pre-selects the saved capability scope and resource preset, never blank", () => {
    render(
      <ScriptRunDialog
        mode="run"
        scriptName="Chase Cycler"
        profile={profile({ scope: "authoring", preset: "long-running-automation" })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("Capability scope")).toHaveValue("authoring");
    expect(screen.getByLabelText("Resource limits")).toHaveValue("long-running-automation");
  });

  it("hides the four numeric fields for a named preset, and reveals them, pre-filled, under Advanced", () => {
    render(
      <ScriptRunDialog
        mode="run"
        scriptName="Chase Cycler"
        profile={profile({
          preset: "quick-action",
          deadlineSeconds: 45,
          ratePerSecond: 10,
          memoryLimitMB: 128,
          cpuCapPercent: 50,
        })}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.queryByLabelText("Deadline (seconds)")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Rate limit (calls/sec)")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Memory limit (MB)")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("CPU cap (%)")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Resource limits"), { target: { value: "advanced" } });

    expect(screen.getByLabelText("Deadline (seconds)")).toHaveValue(45);
    expect(screen.getByLabelText("Rate limit (calls/sec)")).toHaveValue(10);
    expect(screen.getByLabelText("Memory limit (MB)")).toHaveValue(128);
    expect(screen.getByLabelText("CPU cap (%)")).toHaveValue(50);

    fireEvent.change(screen.getByLabelText("Resource limits"), { target: { value: "quick-action" } });
    expect(screen.queryByLabelText("Deadline (seconds)")).not.toBeInTheDocument();
  });

  it("submits the edited profile and mode via onSubmit", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <ScriptRunDialog mode="run" scriptName="Chase Cycler" profile={profile()} onSubmit={onSubmit} onCancel={vi.fn()} />,
    );

    fireEvent.change(screen.getByLabelText("Capability scope"), { target: { value: "authoring" } });
    fireEvent.click(screen.getByRole("button", { name: /^Run Chase Cycler$/ }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ scope: "authoring", preset: "quick-action" }),
        "run",
      ),
    );
  });

  it("clicking Cancel closes without launching", () => {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();
    render(<ScriptRunDialog mode="run" scriptName="Chase" profile={profile()} onSubmit={onSubmit} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("pressing Escape closes without launching", () => {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();
    render(<ScriptRunDialog mode="run" scriptName="Chase" profile={profile()} onSubmit={onSubmit} onCancel={onCancel} />);

    fireEvent.keyDown(document, { key: "Escape" });

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("clicking the backdrop closes without launching", () => {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();
    render(<ScriptRunDialog mode="run" scriptName="Chase" profile={profile()} onSubmit={onSubmit} onCancel={onCancel} />);

    const backdrop = screen.getByRole("dialog").parentElement as HTMLElement;
    fireEvent.click(backdrop);

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("disables the CTA and Cancel and shows a busy state while a submit is in flight, keeping the dialog open", async () => {
    let resolveSubmit: () => void = () => {};
    const onSubmit = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve;
        }),
    );
    render(<ScriptRunDialog mode="run" scriptName="Chase" profile={profile()} onSubmit={onSubmit} onCancel={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /^Run Chase$/ }));

    await waitFor(() => expect(screen.getByRole("button", { name: /^Run Chase$/ })).toBeDisabled());
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByText("Launching…")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    resolveSubmit();
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
  });

  it("keeps the dialog open and renders an inline error when the launch fails", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("GOLC_SCRIPT_RUN_FAILED: boom"));
    render(<ScriptRunDialog mode="run" scriptName="Chase" profile={profile()} onSubmit={onSubmit} onCancel={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /^Run Chase$/ }));

    await waitFor(() => expect(screen.getByText(/GOLC_SCRIPT_RUN_FAILED/)).toBeInTheDocument());
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Run Chase$/ })).not.toBeDisabled();
  });
});
