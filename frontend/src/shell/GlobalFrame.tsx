// GlobalFrame is the shell's persistent 52px top-frame region
// (application-shell-navigation.md): transport/BPM, live truth, and the
// safety cluster. Show/app identity now lives in TitleBar.tsx (the app's
// own self-drawn window chrome), so this no longer repeats a "GOLC" label
// directly beneath it. LiveStatusBar owns the actual status
// fetch/subscribe logic (it is the store's sole writer of
// `status`/`connectionStatus` -- see store.ts) and must never be
// conditionally rendered; GlobalFrame only composes it alongside a
// show-identity placeholder, TempoControls, and (icon/polish + header-
// merge pass) SafetyCluster, which used to be AppShell's own dedicated
// row directly beneath this header -- moved here, to the right of Tap, so
// it reads as one persistent transport/safety strip instead of two
// stacked chrome rows. D-13's "visible and interactive on every
// workspace, independent of daemon reachability" contract is unaffected:
// SafetyCluster still mounts unconditionally, just at a new screen
// position within the same always-mounted header.
import LiveStatusBar from "../components/LiveStatusBar/LiveStatusBar";
import TempoControls from "../components/TempoControls/TempoControls";
import SafetyCluster from "../components/SafetyCluster/SafetyCluster";
import styles from "./GlobalFrame.module.css";

export default function GlobalFrame() {
  return (
    <header className={styles.frame}>
      <div className={styles.statusSlot}>
        <LiveStatusBar />
      </div>
      <TempoControls />
      <div className={styles.safetyDivider} aria-hidden="true" />
      <SafetyCluster />
    </header>
  );
}
