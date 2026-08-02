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
  | "perform-desk"
  | "output-artnet"
  | "output-diagnostics";

export interface NavDestination {
  id: DestinationId;
  label: string;
  // howItWorks is the same hover-tooltip copy the golc-site docs page
  // shows for this destination (internal/docgen/desktopviews.go's
  // generated site catalog) -- both read desktopViews.json, so this
  // string can never drift between the app's own rail and the docs site.
  howItWorks: string;
}

export interface NavGroup {
  label: string;
  description: string;
  destinations: NavDestination[];
}

export const NAV_GROUPS: NavGroup[] = desktopViews.groups.map((group) => ({
  label: group.label,
  description: group.description,
  destinations: group.views.map((view) => ({
    id: view.id as DestinationId,
    label: view.navLabel,
    howItWorks: view.howItWorks,
  })),
}));

export const DEFAULT_DESTINATION: DestinationId = "show-overview";

// HOW_IT_WORKS_BY_ID lets each workspace's own Toolbar pass the exact
// same tooltip copy CommandRail already shows for that destination
// (and golc-site's docs page shows for the same route) without any
// workspace file hardcoding its own copy of the string.
export const HOW_IT_WORKS_BY_ID: Record<DestinationId, string> = Object.fromEntries(
  NAV_GROUPS.flatMap((group) => group.destinations.map((destination) => [destination.id, destination.howItWorks])),
) as Record<DestinationId, string>;
