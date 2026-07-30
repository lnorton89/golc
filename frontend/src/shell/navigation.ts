// navigation.ts projects the shell's locked information architecture
// (.planning/sketches/references/application-shell-navigation.md's grouped
// Show/Build/Operate/Output rail). This is the single source of truth
// CommandRail.tsx and WorkspaceRouter.tsx both read from -- adding a
// destination means adding one catalog entry plus one case in
// WorkspaceRouter.tsx's switch and its exhaustive icon map.
import desktopViews from "./desktopViews.json";

export type DestinationId =
  | "show-overview"
  | "show-shows"
  | "show-save-recovery"
  | "show-settings"
  | "build-fixture-library"
  | "build-patch-pools"
  | "build-project-fixtures"
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

export const NAV_GROUPS: NavGroup[] = desktopViews.groups.map((group) => ({
  label: group.label,
  destinations: group.views.map((view) => ({
    id: view.id as DestinationId,
    label: view.navLabel,
  })),
}));

export const DEFAULT_DESTINATION: DestinationId = "show-overview";
