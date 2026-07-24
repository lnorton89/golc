// useKeyboardWorkflow.ts registers the WINDOW-scoped (in-webview) keydown
// workflow for the complete playback action set (06-06-PLAN.md Task 2,
// PLAY-02). This is ordinary React/DOM keyboard handling scoped to the app
// window -- it deliberately does NOT use golang.design/x/hotkey, which is
// reserved for the three D-16 safety-cluster controls registered from the
// Go host in internal/wails/hotkey.go (06-RESEARCH.md Pitfall 4: "Reserve
// OS-level global hotkeys strictly for the three D-13 safety-cluster
// controls ... everything else in PLAY-02 is ordinary in-webview keyboard
// handling scoped to the app window"). Unlike the safety-cluster hotkeys,
// every shortcut here stops firing the instant the app window loses OS
// focus.
//
// Every handler below calls the exact `dispatch` action functions
// PlaybackControls.tsx's own on-screen buttons call (06-06-PLAN.md
// key_link: "keyboard events invoke the same action handlers as the
// on-screen controls") -- there is no second, parallel action
// implementation here, so the on-screen surface (PLAY-01) and this
// documented keyboard surface (PLAY-02) can never drift out of sync.
//
// PLAYBACK_SHORTCUTS below is the single documented source of truth this
// hook's own keydown handler and KeyboardShortcuts.tsx's reference panel
// both read -- adding, removing, or rebinding a shortcut is a one-place
// change.

import { useEffect } from "react";

import { dispatch, LAYER_KINDS, type LayerKind } from "../lib/playbackDispatch";

export interface KeyboardShortcut {
  category: string;
  keys: string;
  description: string;
}

/** LAYER_KEY_TO_KIND maps the fixed Q/W/E/R shortcut row onto
 * internal/scene's four fixed layer kinds, in the same base-look/color-
 * theme/chase/motion priority order LAYER_KINDS declares. */
const LAYER_KEY_TO_KIND: Record<string, LayerKind> = {
  q: LAYER_KINDS[0],
  w: LAYER_KINDS[1],
  e: LAYER_KINDS[2],
  r: LAYER_KINDS[3],
};

export const PLAYBACK_SHORTCUTS: KeyboardShortcut[] = [
  { category: "Scenes", keys: "1 – 9", description: "Switch to the Nth scene in the current show" },
  { category: "Layers", keys: "Q", description: "Toggle Base Look on the active scene" },
  { category: "Layers", keys: "W", description: "Toggle Color Theme on the active scene" },
  { category: "Layers", keys: "E", description: "Toggle Chase on the active scene" },
  { category: "Layers", keys: "R", description: "Toggle Motion on the active scene" },
  { category: "Tempo", keys: "Space", description: "Tap tempo (accumulates with prior taps within 2s)" },
  { category: "Tempo", keys: "↑", description: "Nudge BPM up by 1" },
  { category: "Tempo", keys: "↓", description: "Nudge BPM down by 1" },
  { category: "Transport", keys: "Enter", description: "Evaluate/preview the active scene at bar 0" },
];

export interface UseKeyboardWorkflowOptions {
  /** Ordered scene names -- digit key N switches to sceneNames[N-1]. */
  sceneNames: string[];
  /** The scene layer toggles (Q/W/E/R) apply to -- normally the active
   * scene; null disables the layer shortcuts entirely. */
  activeSceneName: string | null;
  /** Current per-kind Enabled state, used to compute the toggled value. */
  layerEnabled: Record<string, boolean>;
  /** Current BPM, used to compute the ArrowUp/ArrowDown nudge target. */
  bpm: number;
}

/** isTypingTarget reports whether event.target is a text-entry element
 * (input/textarea/contentEditable) -- the keyboard workflow must never
 * hijack a keystroke the operator is typing into the BPM/evaluate-position
 * fields (e.g. typing "w" while editing a number must not toggle a
 * layer). */
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || target.isContentEditable;
}

/** useKeyboardWorkflow registers a single capture-phase, WINDOW-scoped
 * keydown listener implementing the documented playback shortcut set
 * (PLAY-02). It is added/removed with the owning component's lifecycle --
 * PlaybackControls.tsx is the only caller, since App.tsx is never modified
 * to invoke feature hooks directly (06-04-PLAN.md Task 2's mount-point
 * contract). */
export function useKeyboardWorkflow(options: UseKeyboardWorkflowOptions): void {
  const { sceneNames, activeSceneName, layerEnabled, bpm } = options;

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (isTypingTarget(event.target)) {
        return;
      }

      const digit = Number(event.key);
      if (Number.isInteger(digit) && digit >= 1 && digit <= 9) {
        const sceneName = sceneNames[digit - 1];
        if (sceneName) {
          event.preventDefault();
          void dispatch.switchScene(sceneName);
        }
        return;
      }

      const kind = LAYER_KEY_TO_KIND[event.key.toLowerCase()];
      if (kind && activeSceneName) {
        event.preventDefault();
        void dispatch.setLayerEnabled(activeSceneName, kind, !layerEnabled[kind]);
        return;
      }

      if (event.code === "Space") {
        event.preventDefault();
        void dispatch.recordTap();
        return;
      }

      if (event.key === "ArrowUp") {
        event.preventDefault();
        void dispatch.setBPM(bpm + 1);
        return;
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        void dispatch.setBPM(Math.max(1, bpm - 1));
        return;
      }

      if (event.key === "Enter") {
        event.preventDefault();
        void dispatch.evaluate(0);
      }
    }

    // capture: true keeps this window-scoped listener from being
    // intercepted by a stopPropagation() call deeper in the tree, while
    // still being ordinary in-webview DOM handling -- never the OS-level
    // golang.design/x/hotkey path (see file doc comment).
    window.addEventListener("keydown", onKeyDown, { capture: true });
    return () => window.removeEventListener("keydown", onKeyDown, { capture: true });
  }, [sceneNames, activeSceneName, layerEnabled, bpm]);
}
