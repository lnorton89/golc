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
// position within the same always-mounted header. MidiLearnToggle sits
// immediately after it: the global MIDI Learn on/off switch, visually
// grouped with the safety cluster but a plain reversible toggle rather
// than a hold-to-confirm control.
//
// AppLogStream mounts here for the identical reason LiveStatusBar does: it
// is the store's sole writer of the `appLog` slice, and most "app:log"
// lines fire during App.OnStartup -- before any workspace navigation --
// so its subscription must start as early and unconditionally as
// LiveStatusBar's own. It renders nothing (see its own doc comment).
import LiveStatusBar from "../components/LiveStatusBar/LiveStatusBar";
import TempoControls from "../components/TempoControls/TempoControls";
import SafetyCluster from "../components/SafetyCluster/SafetyCluster";
import MidiLearnToggle from "../components/MidiLearnToggle/MidiLearnToggle";
import AppLogStream from "./AppLogStream";
import styles from "./GlobalFrame.module.css";

export default function GlobalFrame() {
  return (
    <header className={styles.frame}>
      <AppLogStream />
      <div className={styles.statusSlot}>
        <LiveStatusBar />
      </div>
      <TempoControls />
      <div className={styles.safetyDivider} aria-hidden="true" />
      <SafetyCluster />
      <MidiLearnToggle />
    </header>
  );
}
