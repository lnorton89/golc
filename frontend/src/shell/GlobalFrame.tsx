// GlobalFrame is the shell's persistent 52px top-frame region
// (application-shell-navigation.md): show identity, transport/BPM, live
// truth. LiveStatusBar owns the actual status fetch/subscribe logic (it is
// the store's sole writer of `status`/`connectionStatus` -- see store.ts)
// and must never be conditionally rendered; GlobalFrame only composes it
// alongside a show-identity placeholder and (from Step 5) TempoControls.
import LiveStatusBar from "../components/LiveStatusBar/LiveStatusBar";
import TempoControls from "../components/TempoControls/TempoControls";
import styles from "./GlobalFrame.module.css";

export default function GlobalFrame() {
  return (
    <header className={styles.frame}>
      <div className={styles.identity}>
        <span className={styles.identityLabel}>GOLC</span>
      </div>
      <div className={styles.statusSlot}>
        <LiveStatusBar />
      </div>
      <TempoControls />
    </header>
  );
}
