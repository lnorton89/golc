// SafetyCluster.tsx is the persistent, visually distinct safety-cluster
// region (D-13/D-15): Blackout / Revoke Automation / Stop-Release-All, in
// a fixed screen position present on every view (authoring, programming,
// playback alike). 06-05-PLAN.md Task 2 fills this Wave 2 stub: each
// control is a 64px hold-to-confirm target (D-14 -- press and hold
// roughly 500ms-1s, with a visible fill/progress indicator; releasing
// early cancels without ever invoking the daemon call) that, on a
// completed hold, calls the matching SafetyService binding
// (wailsBridge.ts) with the exact same route+"--source manual" shape
// hotkey.go's OS-level callback already uses (RESEARCH.md Pitfall 1: this
// on-screen path is the second, independent trigger into the same daemon
// override state).
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

import { useGolcStore } from "../../store/store";
import {
  safetyBlackout,
  safetyRevokeAutomation,
  safetyStopReleaseAll,
} from "../../lib/wailsBridge";
import styles from "./SafetyCluster.module.css";

// HOLD_DURATION_MS is D-14's press-and-hold window: within the
// spec'd ~500ms-1s range.
const HOLD_DURATION_MS = 750;

interface HoldButtonProps {
  label: string;
  controlColorVar?: string;
  textColorVar?: string;
  active: boolean;
  onActivate: () => void;
}

function HoldButton({
  label,
  controlColorVar,
  textColorVar,
  active,
  onActivate,
}: HoldButtonProps) {
  const [holding, setHolding] = useState(false);
  const timeoutRef = useRef<number | null>(null);

  const clearHoldTimer = useCallback(() => {
    if (timeoutRef.current !== null) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  }, []);

  const startHold = useCallback(() => {
    clearHoldTimer();
    setHolding(true);
    timeoutRef.current = window.setTimeout(() => {
      timeoutRef.current = null;
      setHolding(false);
      onActivate();
    }, HOLD_DURATION_MS);
  }, [clearHoldTimer, onActivate]);

  // cancelHold: releasing early (pointerup/pointerleave/pointercancel/
  // keyup before HOLD_DURATION_MS elapses) clears the pending timer and
  // resets the fill instantly (--motion-snap) -- the hold-to-confirm
  // control's own onActivate is never invoked.
  const cancelHold = useCallback(() => {
    clearHoldTimer();
    setHolding(false);
  }, [clearHoldTimer]);

  useEffect(() => clearHoldTimer, [clearHoldTimer]);

  const handlePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    startHold();
  };

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (event.repeat) return;
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      startHold();
    }
  };

  const handleKeyUp = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (event.key === " " || event.key === "Enter") {
      cancelHold();
    }
  };

  const style: CSSProperties = {
    ...(controlColorVar ? { "--control-color": controlColorVar } : {}),
    ...(textColorVar ? { "--control-text-color": textColorVar } : {}),
  } as CSSProperties;

  return (
    <button
      type="button"
      className={styles.control}
      style={style}
      aria-pressed={holding}
      onPointerDown={handlePointerDown}
      onPointerUp={cancelHold}
      onPointerLeave={cancelHold}
      onPointerCancel={cancelHold}
      onKeyDown={handleKeyDown}
      onKeyUp={handleKeyUp}
    >
      <span
        className={styles.fill}
        style={{
          transform: holding ? "scaleX(1)" : "scaleX(0)",
          transitionProperty: "transform",
          transitionDuration: holding
            ? `${HOLD_DURATION_MS}ms`
            : "var(--motion-snap)",
          transitionTimingFunction: "linear",
        }}
        aria-hidden="true"
      />
      {active && (
        <span className={styles.activeBadge} aria-hidden="true">
          ACTIVE
        </span>
      )}
      <span className={styles.label}>{label}</span>
    </button>
  );
}

export default function SafetyCluster() {
  const status = useGolcStore((state) => state.status);

  // Individual per-control "active" indicators are best-effort: the
  // daemon's PLAY-07 status vocabulary (controllingSource/outputState)
  // is a single combined descriptor, not three independent flags, so
  // Blackout and Stop/Release-All (both of which drive outputState to
  // "blackout" identically, internal/artnet/daemon.go's
  // newPlaybackStatusPayload) cannot be distinguished from this signal
  // alone -- both light up together when either is active. Revoke
  // Automation's own "revoked" controllingSource is unambiguous unless a
  // blackout is simultaneously active (blackout takes priority in the
  // combined vocabulary), in which case only the blackout state shows.
  const blackoutOrStopActive = status.outputState === "blackout";
  const revokeActive = status.controllingSource === "revoked";

  return (
    <div className={styles.cluster} aria-label="Safety cluster">
      <HoldButton
        label={blackoutOrStopActive ? "Hold to Release Blackout" : "Hold to Blackout"}
        controlColorVar="var(--status-blackout)"
        textColorVar="var(--page)"
        active={blackoutOrStopActive}
        onActivate={() => {
          void safetyBlackout(!blackoutOrStopActive);
        }}
      />
      <HoldButton
        label={revokeActive ? "Hold to Restore Automation" : "Hold to Revoke Automation"}
        controlColorVar="var(--status-revoked)"
        textColorVar="var(--page)"
        active={revokeActive}
        onActivate={() => {
          void safetyRevokeAutomation(!revokeActive);
        }}
      />
      <HoldButton
        label={blackoutOrStopActive ? "Hold to Release Stop / Release All" : "Hold to Stop / Release All"}
        active={blackoutOrStopActive}
        onActivate={() => {
          void safetyStopReleaseAll(!blackoutOrStopActive);
        }}
      />
    </div>
  );
}
