// MidiMappingWorkspace wraps MidiPanel.tsx unchanged (shell restructure
// plan Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it).
import { Music2 } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import MidiPanel from "../../components/MidiPanel/MidiPanel";
import styles from "../workspace.module.css";

export default function MidiMappingWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="MIDI Mapping" icon={Music2} info={HOW_IT_WORKS_BY_ID["operate-midi-mapping"]} />
      <div className={styles.canvas}>
        <MidiPanel />
      </div>
    </div>
  );
}
