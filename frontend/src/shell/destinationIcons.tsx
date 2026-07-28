// destinationIcons.tsx maps each NAV_GROUPS destination (navigation.ts) to
// one lucide-react glyph -- the single source both CommandRail.tsx (nav
// rail) and each workspace's own Toolbar call read from, so a destination's
// nav icon and its workspace-header icon always match.
import {
  LayoutDashboard,
  FolderOpen,
  Save,
  Settings,
  Lightbulb,
  Cable,
  Layers,
  FileCode2,
  SlidersHorizontal,
  Music2,
  Network,
  Activity,
  type LucideIcon,
} from "lucide-react";

import type { DestinationId } from "./navigation";

export const DESTINATION_ICONS: Record<DestinationId, LucideIcon> = {
  "show-overview": LayoutDashboard,
  "show-shows": FolderOpen,
  "show-save-recovery": Save,
  "show-settings": Settings,
  "build-fixture-library": Lightbulb,
  "build-patch-pools": Cable,
  "build-scenes-looks": Layers,
  "build-scripts": FileCode2,
  "operate-operator-surface": SlidersHorizontal,
  "operate-midi-mapping": Music2,
  "output-artnet": Network,
  "output-diagnostics": Activity,
};
