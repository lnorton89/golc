// ArtnetWorkspace wraps ArtnetConfig.tsx unchanged (shell restructure plan
// Step 9 CSS-retargets its internals; this wrapper only supplies the
// workspace toolbar/canvas chrome around it).
import { Network } from "lucide-react";

import Toolbar from "../../components/primitives/Toolbar/Toolbar";
import { HOW_IT_WORKS_BY_ID } from "../../shell/navigation";
import ArtnetConfig from "../../components/ArtnetConfig/ArtnetConfig";
import styles from "../workspace.module.css";

export default function ArtnetWorkspace() {
  return (
    <div className={styles.workspace}>
      <Toolbar title="Art-Net" icon={Network} info={HOW_IT_WORKS_BY_ID["output-artnet"]} />
      <div className={styles.canvas}>
        <ArtnetConfig />
      </div>
    </div>
  );
}
