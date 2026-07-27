// ComingSoon is the shared stub for nav destinations with no frontend/
// Wails binding yet (currently just Fixture Library -- Overview, Save &
// Recovery, and Diagnostics have since graduated to real workspaces; see
// each one's own doc comment). Calm tone (--muted/--text2), not an error/
// warning color -- this is expected, not broken.
//
// `icon` is optional (same lucide-react component-reference convention as
// every other primitive's own `icon` prop) and passes straight through to
// Toolbar -- purely additive, the existing call site is unaffected.
import type { LucideIcon } from "lucide-react";

import Panel from "../components/primitives/Panel/Panel";
import Toolbar from "../components/primitives/Toolbar/Toolbar";
import styles from "./ComingSoon.module.css";

interface ComingSoonProps {
  title: string;
  icon?: LucideIcon;
  description: string;
  cliHint: string;
}

export default function ComingSoon({ title, icon, description, cliHint }: ComingSoonProps) {
  return (
    <div className={styles.workspace}>
      <Toolbar title={title} icon={icon} />
      <div className={styles.canvas}>
        <Panel className={styles.panel}>
          <p className={styles.description}>{description}</p>
          <p className={styles.cliHint}>{cliHint}</p>
        </Panel>
      </div>
    </div>
  );
}
