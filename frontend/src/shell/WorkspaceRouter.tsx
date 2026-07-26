// WorkspaceRouter renders exactly one workspace for the active destination.
// Deliberately a plain switch (not keep-mounted-hidden): unmounting the
// previous workspace on navigate-away is what makes OperatorSurface's CR-01
// active-surface cleanup effect fire correctly with zero changes to that
// effect (shell restructure plan Step 4/7), and matches the sketch's own
// "selecting a destination replaces the workspace" wording.
import type { DestinationId } from "./navigation";

import OverviewWorkspace from "../workspaces/show/OverviewWorkspace";
import SaveRecoveryWorkspace from "../workspaces/show/SaveRecoveryWorkspace";
import FixtureLibraryWorkspace from "../workspaces/build/FixtureLibraryWorkspace";
import PatchPoolsWorkspace from "../workspaces/build/PatchPoolsWorkspace";
import ScenesLooksWorkspace from "../workspaces/build/ScenesLooksWorkspace";
import ScriptsWorkspace from "../workspaces/build/ScriptsWorkspace";
import OperatorSurfaceWorkspace from "../workspaces/operate/OperatorSurfaceWorkspace";
import MidiMappingWorkspace from "../workspaces/operate/MidiMappingWorkspace";
import ArtnetWorkspace from "../workspaces/output/ArtnetWorkspace";
import DiagnosticsWorkspace from "../workspaces/output/DiagnosticsWorkspace";

interface WorkspaceRouterProps {
  active: DestinationId;
}

export default function WorkspaceRouter({ active }: WorkspaceRouterProps) {
  switch (active) {
    case "show-overview":
      return <OverviewWorkspace />;
    case "show-save-recovery":
      return <SaveRecoveryWorkspace />;
    case "build-fixture-library":
      return <FixtureLibraryWorkspace />;
    case "build-patch-pools":
      return <PatchPoolsWorkspace />;
    case "build-scenes-looks":
      return <ScenesLooksWorkspace />;
    case "build-scripts":
      return <ScriptsWorkspace />;
    case "operate-operator-surface":
      return <OperatorSurfaceWorkspace />;
    case "operate-midi-mapping":
      return <MidiMappingWorkspace />;
    case "output-artnet":
      return <ArtnetWorkspace />;
    case "output-diagnostics":
      return <DiagnosticsWorkspace />;
  }
}
