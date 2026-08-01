// HotkeySettings is Settings > Hotkeys' rebind UI, covering two binding
// shapes: the bare-key playback actions (HOTKEY_ACTIONS -- Layers/Tempo/
// Transport) and the modifier-chord navigation actions (NAV_ACTIONS --
// Navigation). Click a key/chord button to enter "recording" mode, then
// press the new binding -- Escape cancels. For a playback action, any
// modifier-free key either saves the binding (setHotkeyBinding) or, on
// conflict with another playback action or the fixed 1-9 scene-switch
// range (findHotkeyConflict), shows an inline message and stays in
// recording mode; a chorded press is rejected (playback shortcuts are
// matched with any modifier held, so a chord could never fire -- see
// useKeyboardWorkflow.ts's own chord guard). For a nav action, the reverse:
// only a chord (at least one of Ctrl/Alt/Shift held) is accepted, checked
// against other nav bindings (findChordConflict) -- a bare key is rejected
// since it would collide with ordinary typing. A lone modifier keydown
// (isModifierKeyEvent) is neither accepted nor rejected -- recording just
// keeps waiting for the following keydown that completes the press.
// Persisted bindings are the same lib/hotkeys.ts stores
// useKeyboardWorkflow.ts/useGlobalKeyboardWorkflow.ts's matchers and
// KeyboardShortcuts.tsx's reference panel read, so a change here takes
// effect everywhere immediately.
import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";

import { useHotkeyBindings } from "../../hooks/useHotkeyBindings";
import { useNavHotkeyBindings } from "../../hooks/useNavHotkeyBindings";
import {
  HOTKEY_ACTIONS,
  NAV_ACTIONS,
  SCENE_SWITCH_SHORTCUT,
  findChordConflict,
  findHotkeyConflict,
  formatChordLabel,
  formatHotkeyLabel,
  isModifierKeyEvent,
  normalizeHotkeyChord,
  normalizeHotkeyEvent,
  resetAllHotkeys,
  resetHotkeyBinding,
  resetNavHotkeyBinding,
  setHotkeyBinding,
  setNavHotkeyBinding,
  type HotkeyActionId,
  type NavActionId,
} from "../../lib/hotkeys";
import Button from "../primitives/Button/Button";
import styles from "./HotkeySettings.module.css";

const PLAYBACK_CATEGORIES = ["Layers", "Tempo", "Transport"];

type Recording = { kind: "playback"; id: HotkeyActionId } | { kind: "nav"; id: NavActionId } | null;

export default function HotkeySettings() {
  const bindings = useHotkeyBindings();
  const navBindings = useNavHotkeyBindings();
  const [recording, setRecording] = useState<Recording>(null);
  const [conflict, setConflict] = useState<string | null>(null);

  useEffect(() => {
    if (!recording) {
      return;
    }

    function onKeyDown(event: KeyboardEvent) {
      if (isModifierKeyEvent(event)) {
        // Still waiting for the real key that completes the press/chord.
        return;
      }

      event.preventDefault();
      event.stopPropagation();

      if (event.key === "Escape") {
        setRecording(null);
        setConflict(null);
        return;
      }

      if (recording!.kind === "playback") {
        if (event.ctrlKey || event.altKey || event.metaKey) {
          setConflict("Playback shortcuts can't use Ctrl/Alt/Cmd -- press a plain key.");
          return;
        }
        const key = normalizeHotkeyEvent(event);
        const activeId = recording!.id as HotkeyActionId;
        const clash = findHotkeyConflict(bindings, activeId, key);
        if (clash) {
          setConflict(
            clash === "scene-switch"
              ? "That key is reserved for scene switching (1–9)."
              : `That key is already used by "${HOTKEY_ACTIONS.find((action) => action.id === clash)?.description}".`,
          );
          return;
        }
        setHotkeyBinding(activeId, key);
      } else {
        if (!event.ctrlKey && !event.altKey) {
          setConflict("Navigation shortcuts need Ctrl or Alt held down.");
          return;
        }
        const chord = normalizeHotkeyChord(event);
        if (!chord) {
          return;
        }
        const activeId = recording!.id as NavActionId;
        const clash = findChordConflict(navBindings, activeId, chord);
        if (clash) {
          setConflict(`That combination is already used by "${NAV_ACTIONS.find((action) => action.id === clash)?.description}".`);
          return;
        }
        setNavHotkeyBinding(activeId, chord);
      }

      setRecording(null);
      setConflict(null);
    }

    window.addEventListener("keydown", onKeyDown, { capture: true });
    return () => window.removeEventListener("keydown", onKeyDown, { capture: true });
  }, [recording, bindings, navBindings]);

  const hasCustomBindings =
    HOTKEY_ACTIONS.some((action) => bindings[action.id] !== action.defaultKey) ||
    NAV_ACTIONS.some((action) => navBindings[action.id] !== action.defaultChord);

  const startRecordingPlayback = (id: HotkeyActionId) => {
    setRecording({ kind: "playback", id });
    setConflict(null);
  };
  const startRecordingNav = (id: NavActionId) => {
    setRecording({ kind: "nav", id });
    setConflict(null);
  };

  return (
    <div className={styles.hotkeys}>
      {PLAYBACK_CATEGORIES.map((category) => (
        <div key={category} className={styles.group}>
          <span className={styles.groupHeading}>{category}</span>
          <ul className={styles.list}>
            {HOTKEY_ACTIONS.filter((action) => action.category === category).map((action) => {
              const isRecording = recording?.kind === "playback" && recording.id === action.id;
              const isCustom = bindings[action.id] !== action.defaultKey;
              return (
                <li key={action.id} className={styles.row}>
                  <span className={styles.description}>{action.description}</span>
                  <div className={styles.controls}>
                    <button
                      type="button"
                      className={isRecording ? `${styles.keyButton} ${styles.recording}` : styles.keyButton}
                      onClick={() => startRecordingPlayback(action.id)}
                    >
                      {isRecording ? "Press a key…" : formatHotkeyLabel(bindings[action.id])}
                    </button>
                    <button
                      type="button"
                      className={styles.resetButton}
                      aria-label={`Reset ${action.description} to default`}
                      disabled={!isCustom}
                      onClick={() => resetHotkeyBinding(action.id)}
                    >
                      <RotateCcw size={12} aria-hidden="true" />
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      ))}

      <div className={styles.group}>
        <span className={styles.groupHeading}>Navigation</span>
        <ul className={styles.list}>
          {NAV_ACTIONS.map((action) => {
            const isRecording = recording?.kind === "nav" && recording.id === action.id;
            const isCustom = navBindings[action.id] !== action.defaultChord;
            return (
              <li key={action.id} className={styles.row}>
                <span className={styles.description}>{action.description}</span>
                <div className={styles.controls}>
                  <button
                    type="button"
                    className={isRecording ? `${styles.keyButton} ${styles.recording}` : styles.keyButton}
                    onClick={() => startRecordingNav(action.id)}
                  >
                    {isRecording ? "Press a combo…" : formatChordLabel(navBindings[action.id])}
                  </button>
                  <button
                    type="button"
                    className={styles.resetButton}
                    aria-label={`Reset ${action.description} to default`}
                    disabled={!isCustom}
                    onClick={() => resetNavHotkeyBinding(action.id)}
                  >
                    <RotateCcw size={12} aria-hidden="true" />
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      </div>

      {conflict ? (
        <p className={styles.conflict} role="alert">
          {conflict}
        </p>
      ) : null}

      <div className={styles.group}>
        <span className={styles.groupHeading}>Fixed</span>
        <p className={styles.hint}>Not rebindable -- scene digits are an ordinal range over the show's own scene count.</p>
        <ul className={styles.list}>
          <li className={styles.row}>
            <span className={styles.description}>{SCENE_SWITCH_SHORTCUT.description}</span>
            <kbd className={styles.fixedKey}>{SCENE_SWITCH_SHORTCUT.keys}</kbd>
          </li>
        </ul>
      </div>

      <div className={styles.footer}>
        <Button variant="secondary" disabled={!hasCustomBindings} onClick={() => resetAllHotkeys()}>
          Reset all to defaults
        </Button>
      </div>
    </div>
  );
}
