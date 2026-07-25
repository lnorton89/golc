// MidiMappingWorkspace wraps MidiPanel.tsx unchanged (shell restructure
// plan Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it).
import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import MidiPanel from "../../components/MidiPanel/MidiPanel";
import styles from "../workspace.module.css";

export default function MidiMappingWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="MIDI Mapping" />
      <div className={styles.canvas}>
        <MidiPanel />
      </div>
    </div>
  );
}
