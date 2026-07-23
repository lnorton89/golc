// App.tsx composes the PERSISTENT global layout every remaining GUI slice
// mounts into (06-04-PLAN.md Task 2): a fixed-position safety-cluster
// region (D-13/D-15, present on every screen) above a fixed-height live
// status bar (PLAY-07 chrome), followed by the mounted feature regions.
// Wave 3/4 plans replace the CONTENTS of their own stub component file
// only -- SafetyCluster.tsx, LiveStatusBar.tsx, PlaybackControls.tsx,
// OperatorSurface.tsx, MidiPanel.tsx -- and never edit this file's layout
// or mount points.

import type { CSSProperties } from "react";

import SafetyCluster from "./components/SafetyCluster/SafetyCluster";
import LiveStatusBar from "./components/LiveStatusBar/LiveStatusBar";
import PlaybackControls from "./components/PlaybackControls/PlaybackControls";
import OperatorSurface from "./components/OperatorSurface/OperatorSurface";
import MidiPanel from "./components/MidiPanel/MidiPanel";
import FixturePatch from "./components/FixturePatch/FixturePatch";
import ArtnetConfig from "./components/ArtnetConfig/ArtnetConfig";
import SceneProgramming from "./components/SceneProgramming/SceneProgramming";

const shellStyle: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  minHeight: "100vh",
};

const featureRegionStyle: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: "var(--space-xl)",
  padding: "var(--space-xl)",
  flex: 1,
};

export default function App() {
  return (
    <div style={shellStyle}>
      <SafetyCluster />
      <LiveStatusBar />
      <main style={featureRegionStyle}>
        <PlaybackControls />
        <OperatorSurface />
        <FixturePatch />
        <ArtnetConfig />
        <SceneProgramming />
        <MidiPanel />
      </main>
    </div>
  );
}
