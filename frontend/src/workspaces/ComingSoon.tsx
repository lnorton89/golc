// ComingSoon is the shared stub for nav destinations with no frontend/
// Wails binding yet (Overview, Save & Recovery, Fixture Library,
// Diagnostics -- shell restructure plan, explicitly deferred scope: their
// Go-side CLI logic exists but nothing binds it into cmd/golc-desktop/
// main.go yet). Calm tone (--muted/--text2), not an error/warning color --
// this is expected, not broken.
import Panel from "../components/primitives/Panel/Panel";
import Toolbar from "../components/primitives/Toolbar/Toolbar";
import styles from "./ComingSoon.module.css";

interface ComingSoonProps {
  title: string;
  description: string;
  cliHint: string;
}

export default function ComingSoon({ title, description, cliHint }: ComingSoonProps) {
  return (
    <div className={styles.workspace}>
      <Toolbar title={title} />
      <div className={styles.canvas}>
        <Panel className={styles.panel}>
          <p className={styles.description}>{description}</p>
          <p className={styles.cliHint}>{cliHint}</p>
        </Panel>
      </div>
    </div>
  );
}
