import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import Toast, { useToast } from "./Toast";

afterEach(() => cleanup());

/** Emitter renders the three intent-named calls as buttons so each test
 * drives the real hook through a real click, rather than reaching into the
 * toast manager directly. */
function Emitter() {
  const toast = useToast();
  return (
    <div>
      <button onClick={() => toast.success("Fixture added", "Chauvet SlimPAR Pro")}>emit success</button>
      <button onClick={() => toast.error("Add failed", "Destination already exists")}>emit error</button>
      <button onClick={() => toast.show("Nothing to report")}>emit neutral</button>
    </div>
  );
}

function renderHost() {
  return render(
    <Toast>
      <Emitter />
    </Toast>,
  );
}

describe("Toast", () => {
  it("renders nothing until something is emitted", () => {
    renderHost();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.queryByText("Fixture added")).not.toBeInTheDocument();
  });

  it("shows a title and description for an emitted toast", async () => {
    const user = userEvent.setup();
    renderHost();

    await user.click(screen.getByRole("button", { name: "emit success" }));

    expect(await screen.findByText("Fixture added")).toBeInTheDocument();
    expect(screen.getByText("Chauvet SlimPAR Pro")).toBeInTheDocument();
  });

  it("emits without a description when none is given", async () => {
    const user = userEvent.setup();
    renderHost();

    await user.click(screen.getByRole("button", { name: "emit neutral" }));

    expect(await screen.findByText("Nothing to report")).toBeInTheDocument();
  });

  it("announces toasts through a polite live region rather than stealing focus", async () => {
    const user = userEvent.setup();
    renderHost();

    await user.click(screen.getByRole("button", { name: "emit success" }));
    await screen.findByText("Fixture added");

    // Base UI's viewport IS the announcement channel: a polite live region
    // labelled "Notifications". This is what makes a toast safe to fire
    // mid-show -- it is read out without moving the operator's focus away
    // from whatever control they are on.
    const viewport = screen.getByRole("region", { name: "Notifications" });
    expect(viewport).toHaveAttribute("aria-live", "polite");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "emit success" }));
  });

  it("dismisses a toast when its close control is used", async () => {
    const user = userEvent.setup();
    renderHost();

    await user.click(screen.getByRole("button", { name: "emit error" }));
    expect(await screen.findByText("Add failed")).toBeInTheDocument();

    // Queried by label, not by role: Base UI deliberately marks the close
    // control aria-hidden while keeping it tabbable (tabindex="0"), so the
    // live region announces the toast's text without the assistive-tech
    // tree also announcing a button the operator has not navigated into
    // yet. It becomes exposed once focus enters the viewport. Asserting via
    // getByRole here would be asserting the opposite of the intended
    // behaviour.
    const close = screen.getByLabelText("Dismiss notification");
    expect(close).toHaveAttribute("tabindex", "0");
    await user.click(close);

    await waitFor(() => expect(screen.queryByText("Add failed")).not.toBeInTheDocument());
  });

  it("stacks multiple toasts at once", async () => {
    const user = userEvent.setup();
    renderHost();

    await user.click(screen.getByRole("button", { name: "emit success" }));
    await user.click(screen.getByRole("button", { name: "emit error" }));

    expect(await screen.findByText("Fixture added")).toBeInTheDocument();
    expect(screen.getByText("Add failed")).toBeInTheDocument();
  });

  it("renders children unconditionally, so mounting the host never gates the app", () => {
    renderHost();
    // The host wraps the whole shell in AppShell: if it ever rendered its
    // children conditionally (behind a toast being present, say), mounting
    // it would blank the application.
    expect(screen.getByRole("button", { name: "emit success" })).toBeInTheDocument();
  });
});
