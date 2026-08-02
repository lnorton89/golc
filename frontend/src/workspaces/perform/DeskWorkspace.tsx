// DeskWorkspace wraps Desk.tsx unchanged (mirrors ArtnetWorkspace's own
// thin-wrapper pattern): this wrapper only supplies the workspace
// toolbar/canvas chrome around it.
import { Sliders } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import Desk from "../../components/Desk/Desk";
import styles from "../workspace.module.css";

export default function DeskWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Desk" icon={Sliders} info={HOW_IT_WORKS_BY_ID["perform-desk"]} />
      <div className={styles.canvas}>
        <Desk />
      </div>
    </div>
  );
}
