import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import DesignSystemGallery from "./DesignSystemGallery";
import * as patterns from "../patterns";

describe("DesignSystemGallery", () => {
  it("renders deterministic consideration and safety states", () => {
    render(<DesignSystemGallery />);

    expect(screen.getByRole("heading", { name: "Design system gallery" })).toBeVisible();
    expect(screen.getByText("Zero / one / many")).toBeVisible();
    expect(screen.getByText("Partial review")).toBeVisible();
    expect(screen.getByText("Busy and error")).toBeVisible();
    expect(screen.getByText("Long copy")).toBeVisible();
    expect(screen.getByRole("button", { name: "Blackout output" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Revoke automation" })).toBeEnabled();
  });
});

describe("product patterns", () => {
  it.each([
    "WorkspaceFrame",
    "SplitPane",
    "DataList",
    "FormActions",
    "ImpactReview",
    "GuidedFlow",
    "SceneStack",
    "LauncherMasters",
    "MidiPickup",
    "SafetyAction",
  ] as const)("exports %s", (name) => {
    expect(patterns[name]).toBeTypeOf("function");
  });
});
