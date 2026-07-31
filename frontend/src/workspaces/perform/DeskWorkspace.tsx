// DeskWorkspace wraps Desk.tsx unchanged (mirrors ArtnetWorkspace's own
// thin-wrapper pattern): this wrapper only supplies the workspace
// toolbar/canvas chrome around it.
import { Sliders } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import Desk from "../../components/Desk/Desk";
import styles from "../workspace.module.css";

export default function DeskWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Desk" icon={Sliders} />
      <div className={styles.canvas}>
        <Desk />
      </div>
    </div>
  );
}
