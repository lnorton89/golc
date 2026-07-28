// navigation.ts declares the shell's locked information architecture
// (.planning/sketches/references/application-shell-navigation.md's grouped
// Show/Build/Operate/Output rail). This is the single source of truth
// CommandRail.tsx and WorkspaceRouter.tsx both read from -- adding a
// destination means adding one entry here plus one case in
// WorkspaceRouter.tsx's switch.

export type DestinationId =
  | "show-overview"
  | "show-shows"
  | "show-save-recovery"
  | "show-settings"
  | "build-fixture-library"
  | "build-patch-pools"
  | "build-scenes-looks"
  | "build-scripts"
  | "operate-operator-surface"
  | "operate-midi-mapping"
  | "output-artnet"
  | "output-diagnostics";

export interface NavDestination {
  id: DestinationId;
  label: string;
}

export interface NavGroup {
  label: string;
  destinations: NavDestination[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    label: "Show",
    destinations: [
      { id: "show-overview", label: "Overview" },
      { id: "show-shows", label: "Shows" },
      { id: "show-save-recovery", label: "Save & Recovery" },
      { id: "show-settings", label: "Settings" },
    ],
  },
  {
    label: "Build",
    destinations: [
      { id: "build-fixture-library", label: "Fixture Library" },
      { id: "build-patch-pools", label: "Patch & Pools" },
      { id: "build-scenes-looks", label: "Scenes & Looks" },
      { id: "build-scripts", label: "Scripts" },
    ],
  },
  {
    label: "Operate",
    destinations: [
      { id: "operate-operator-surface", label: "Operator Surface" },
      { id: "operate-midi-mapping", label: "MIDI Mapping" },
    ],
  },
  {
    label: "Output",
    destinations: [
      { id: "output-artnet", label: "Art-Net" },
      { id: "output-diagnostics", label: "Diagnostics" },
    ],
  },
];

export const DEFAULT_DESTINATION: DestinationId = "show-overview";
