// SafetyCluster.tsx is the persistent, visually distinct safety-cluster
// region (D-13/D-15): Blackout / Revoke Automation / Stop-Release-All, in
// a fixed screen position present on every view (authoring, programming,
// playback alike). Each control is a hold-to-confirm target (D-14 -- press
// and hold roughly 500ms-1s, with a visible determinate progress fill;
// releasing early, pointer cancellation, focus loss/window blur, or Escape
// cancels without ever invoking the daemon call) that, on a completed
// hold, calls the matching SafetyService binding (wailsBridge.ts) with the
// exact same route+"--source manual" shape hotkey.go's OS-level callback
// already uses (RESEARCH.md Pitfall 1: this on-screen path is the second,
// independent trigger into the same daemon override state).
//
// 13-15 design-system migration: this file now composes the shared
// `Button` primitive plus semantic `--ds-*` safety tokens instead of a raw
// styled `<button>`. Every control keeps its own independent hold state
// machine (useHoldToConfirm below), busy indicator, and failure
// announcement -- a stuck or failed action on one control never blocks or
// hides the other two (D-13's "per-action busy/error never blocks sibling
// safety actions").
//
// CR-03 fix: each hold-to-confirm control TOGGLES against the currently
// observed combined state (status.outputState/status.controllingSource,
// the same signal this file's own active/blackoutOrStopActive/revokeActive
// derivation already reads) rather than always forwarding "on=true" --
// without this, activating Blackout/Stop-Release-All/Revoke Automation
// from the desktop shell had no in-app release path at all (recovery
// required a separate CLI invocation). hotkey.go's OS-level callbacks
// carry the identical toggle fix (HotkeyManager.nextToggleValue).
//
// D-13 also means this region must remain visible AND interactive even
// when the daemon is unreachable (LiveStatusBar.tsx renders the
// daemon-unreachable copy alongside this always-mounted cluster,
// 06-UI-SPEC.md error state) -- this component therefore never gates its
// own rendering or its controls' interactivity on connection status.

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";
import { PowerOff, Power, Ban, RotateCcw, Square, Play, type LucideIcon } from "lucide-react";

import { useGolcStore } from "../../store/store";
import {
  safetyBlackout,
  safetyRevokeAutomation,
  safetyStopReleaseAll,
  type WailsResult,
} from "../../lib/wailsBridge";
import Button from "../primitives/Button/Button";
import styles from "./SafetyCluster.module.css";

// HOLD_DURATION_MS is D-14's press-and-hold window: within the
// spec'd ~500ms-1s range.
const HOLD_DURATION_MS = 750;

// PROGRESS_TICK_MS drives the visible determinate-progress updates during
// a hold -- a smooth-enough cadence for a 750ms hold without scheduling an
// excessive number of timer callbacks. The actual threshold dispatch is a
// separate, independently-scheduled timer (see useHoldToConfirm) so its
// exactly-once firing never depends on this tick's own alignment.
const PROGRESS_TICK_MS = 50;

interface HoldTimers {
  interval: number | null;
  timeout: number | null;
  startedAt: number | null;
  completed: boolean;
}

/** useHoldToConfirm is the one explicit hold state machine shared by both
 * pointer and keyboard activation (D-14): it arms a single threshold timer
 * per hold, exposes determinate 0..1 progress while holding, guards the
 * completion callback with a per-hold exactly-once latch (so a duplicate
 * terminal event -- e.g. both pointerup and pointercancel arriving after
 * the threshold already fired -- can never invoke onComplete twice or
 * un-toggle anything), and resets cleanly on every locked interruption:
 * early release, pointercancel, focus loss, window blur, or Escape. A
 * cancelled or completed hold always leaves the machine ready for a fresh,
 * independent retry. */
function useHoldToConfirm(durationMs: number, onComplete: () => void) {
  const [progress, setProgress] = useState(0);
  const [holding, setHolding] = useState(false);
  const timers = useRef<HoldTimers>({ interval: null, timeout: null, startedAt: null, completed: false });
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;

  const clear = useCallback(() => {
    const current = timers.current;
    if (current.interval !== null) {
      window.clearInterval(current.interval);
      current.interval = null;
    }
    if (current.timeout !== null) {
      window.clearTimeout(current.timeout);
      current.timeout = null;
    }
  }, []);

  const cancel = useCallback(() => {
    // A hold that already completed (the threshold timer already fired
    // and invoked onComplete) must never be reset by a later, redundant
    // terminal event -- this is exactly what makes duplicate
    // pointerup/pointercancel/keyup events after completion a safe no-op.
    // The `completed` latch stays untouched here (only start() clears it,
    // so onComplete still can't fire twice), but the *visual* progress
    // must still drain: nothing else called setProgress(0) after a
    // successful hold, so the .fill wash stayed at scaleX(1) covering the
    // whole button until the next hold began -- a safety control that
    // already fired permanently reading as mid-press.
    if (timers.current.completed) {
      setProgress(0);
      return;
    }
    clear();
    timers.current.startedAt = null;
    setHolding(false);
    setProgress(0);
  }, [clear]);

  const start = useCallback(() => {
    clear();
    timers.current.completed = false;
    timers.current.startedAt = Date.now();
    setHolding(true);
    setProgress(0);

    timers.current.interval = window.setInterval(() => {
      const startedAt = timers.current.startedAt;
      if (startedAt === null) return;
      setProgress(Math.min(1, (Date.now() - startedAt) / durationMs));
    }, PROGRESS_TICK_MS);

    timers.current.timeout = window.setTimeout(() => {
      if (timers.current.completed) return;
      timers.current.completed = true;
      clear();
      setProgress(1);
      setHolding(false);
      onCompleteRef.current();
    }, durationMs);
  }, [clear, durationMs]);

  // While actively holding, a window blur (switching apps/windows) or an
  // Escape keypress -- regardless of whether this particular hold started
  // via pointer or keyboard -- cancels immediately without dispatching.
  useEffect(() => {
    if (!holding) return;
    const handleWindowBlur = () => cancel();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") cancel();
    };
    window.addEventListener("blur", handleWindowBlur);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("blur", handleWindowBlur);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [holding, cancel]);

  useEffect(() => clear, [clear]);

  return { progress, holding, start, cancel };
}

interface HoldButtonProps {
  label: string;
  icon: LucideIcon;
  controlColorVar: string;
  textColorVar: string;
  active: boolean;
  onActivate: () => Promise<WailsResult>;
}

function HoldButton({ label, icon: Icon, controlColorVar, textColorVar, active, onActivate }: HoldButtonProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dispatchCommand = useCallback(() => {
    setError(null);
    setBusy(true);
    void onActivate()
      .then((result) => {
        if (result.exitCode !== 0) setError(result.stderr || `${label} failed`);
      })
      .catch(() => {
        setError(`${label} failed`);
      })
      .finally(() => {
        setBusy(false);
      });
  }, [onActivate, label]);

  const { progress, holding, start, cancel } = useHoldToConfirm(HOLD_DURATION_MS, dispatchCommand);

  const handlePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    start();
  };

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    // event.repeat guards against the browser's own key-repeat firing
    // keydown many times per second while held -- calling start() on each
    // of those would perpetually reset the timer and the hold could never
    // reach its threshold.
    if (event.repeat) return;
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      start();
    }
  };

  const handleKeyUp = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (event.key === " " || event.key === "Enter") cancel();
  };

  const toneStyle = {
    "--ds-safety-control-color": controlColorVar,
    "--ds-safety-control-text-color": textColorVar,
  } as CSSProperties;

  return (
    <Button
      type="button"
      variant="secondary"
      size="compact"
      className={styles.control}
      style={toneStyle}
      data-safety-control="true"
      data-active={active || undefined}
      data-error={error ? "true" : undefined}
      aria-pressed={active}
      aria-busy={busy}
      aria-label={label}
      onPointerDown={handlePointerDown}
      onPointerUp={cancel}
      onPointerLeave={cancel}
      onPointerCancel={cancel}
      onBlur={cancel}
      onKeyDown={handleKeyDown}
      onKeyUp={handleKeyUp}
    >
      <span
        className={styles.fill}
        style={{
          transform: `scaleX(${progress})`,
          transitionProperty: holding ? "transform" : "none",
          transitionDuration: holding ? `${PROGRESS_TICK_MS}ms` : "0ms",
          transitionTimingFunction: "linear",
        }}
        aria-hidden="true"
      />
      <Icon size={14} className={styles.icon} aria-hidden="true" />
      <span className={styles.label}>{label}</span>
      {/* role="alert" (implicit aria-live="assertive") announces a
          dispatch failure immediately to assistive tech without an
          intercepting overlay -- GlobalFrame's fixed 52px header has no
          spare layout room for a visible inline message, so the sighted
          signal is the `data-error` ring above instead (D-13: overlays
          must never be able to intercept these controls). */}
      {error ? (
        <span role="alert" className="ds-sr-only">
          {error}
        </span>
      ) : null}
    </Button>
  );
}

export default function SafetyCluster() {
  const status = useGolcStore((state) => state.status);

  // The daemon's PLAY-07 status vocabulary (controllingSource/outputState)
  // is a single combined descriptor, not three independent flags: Blackout
  // and Stop/Release-All both drive outputState to "blackout" identically
  // (internal/artnet/daemon.go's newPlaybackStatusPayload), so that signal
  // alone cannot say which one is engaged.
  //
  // Both controls used to render straight off it, which meant more than a
  // fuzzy indicator: the same ambiguous boolean also produced the argument
  // sent to the daemon. Engaging Blackout alone made BOTH read "Release…",
  // and holding "Release Stop / Release All" then sent
  // safetyStopReleaseAll(false) -- releasing something that was never
  // engaged, leaving outputState "blackout" and the button still saying
  // "Release". The reverse case sent safetyBlackout(false) for the same
  // reason.
  //
  // stopEngaged records which of the two THIS surface engaged, so exactly
  // one of them claims the "Release" state and each sends the argument
  // matching its own label. Output that went to blackout from anywhere
  // else (another surface, a MIDI-mapped control, before this component
  // mounted) attributes to Blackout -- the conservative default, since
  // Blackout is the primary safety control and its release path is the one
  // an operator reaches for first.
  const [stopEngaged, setStopEngaged] = useState(false);

  const outputStopped = status.outputState === "blackout";
  const stopActive = outputStopped && stopEngaged;
  const blackoutActive = outputStopped && !stopEngaged;
  const revokeActive = status.controllingSource === "revoked";

  // Output came back up (from either control, or from anywhere else), so
  // nothing this surface engaged is still engaged.
  useEffect(() => {
    if (!outputStopped) {
      setStopEngaged(false);
    }
  }, [outputStopped]);

  return (
    <div className={styles.cluster} aria-label="Safety cluster">
      <HoldButton
        label={blackoutActive ? "Release Blackout" : "Blackout"}
        icon={blackoutActive ? Power : PowerOff}
        controlColorVar="var(--ds-status-blackout)"
        textColorVar="var(--ds-status-on-blackout)"
        active={blackoutActive}
        onActivate={() => safetyBlackout(!blackoutActive)}
      />
      <HoldButton
        label={revokeActive ? "Restore Automation" : "Automation"}
        icon={revokeActive ? RotateCcw : Ban}
        controlColorVar="var(--ds-status-revoked)"
        textColorVar="var(--ds-status-on-revoked)"
        active={revokeActive}
        onActivate={() => safetyRevokeAutomation(!revokeActive)}
      />
      <HoldButton
        label={stopActive ? "Release Stop / Release All" : "Stop / Release All"}
        icon={stopActive ? Play : Square}
        controlColorVar="var(--ds-surface-control)"
        textColorVar="var(--ds-text-primary)"
        active={stopActive}
        onActivate={async () => {
          const next = !stopActive;
          const result = await safetyStopReleaseAll(next);
          // Only claim the engaged state once the daemon accepted it --
          // a rejected hold must not leave this surface believing it owns
          // an output stop it never caused.
          if (result.exitCode === 0) {
            setStopEngaged(next);
          }
          return result;
        }}
      />
    </div>
  );
}
