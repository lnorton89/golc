import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import Chip from "./Chip";

describe("Chip", () => {
  afterEach(() => cleanup());

  it("renders its children", () => {
    render(<Chip>LIVE</Chip>);
    expect(screen.getByText("LIVE")).toBeInTheDocument();
  });

  it("defaults to the neutral tone", () => {
    render(<Chip>Idle</Chip>);
    // neutral tone should not carry any of the semantic status classNames
    const chip = screen.getByText("Idle");
    expect(chip.className).not.toMatch(/live|frameLock|armed|revoked|blackout|offline/i);
  });

  it.each(["live", "frame-lock", "armed", "revoked", "blackout", "offline"] as const)(
    "renders the %s tone without throwing",
    (tone) => {
      expect(() => render(<Chip tone={tone}>{tone}</Chip>)).not.toThrow();
      expect(screen.getByText(tone)).toBeInTheDocument();
      cleanup();
    },
  );
});
