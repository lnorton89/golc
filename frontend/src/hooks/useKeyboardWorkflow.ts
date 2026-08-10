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
// Which key fires which action is no longer fixed here -- it's resolved
// against lib/hotkeys.ts's persisted bindings (Settings > Hotkeys) on
// every render, so a rebind takes effect immediately without a reload.
// lib/hotkeys.ts's HOTKEY_ACTIONS is the single documented source of truth
// this hook's matcher and KeyboardShortcuts.tsx's reference panel both
// read -- adding or removing an action is a one-place change there.

import { useEffect, useRef, useState } from "react";

import { dispatch, LAYER_KINDS, type LayerKind } from "../lib/playbackDispatch";
import {
  HOTKEY_ACTIONS,
  getStoredHotkeys,
  isHotkeyCaptureActive,
  normalizeHotkeyEvent,
  onHotkeysChanged,
  type HotkeyActionId,
  type HotkeyBindings,
} from "../lib/hotkeys";

/** LAYER_ACTION_TO_KIND maps the rebindable layer-toggle actions onto
 * internal/scene's four fixed layer kinds, in the same base-look/color-
 * theme/chase/motion priority order LAYER_KINDS declares. */
const LAYER_ACTION_TO_KIND: Partial<Record<HotkeyActionId, LayerKind>> = {
  toggleBaseLook: LAYER_KINDS[0],
  toggleColorTheme: LAYER_KINDS[1],
  toggleChase: LAYER_KINDS[2],
  toggleMotion: LAYER_KINDS[3],
};

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
  const [bindings, setBindings] = useState<HotkeyBindings>(() => getStoredHotkeys());

  useEffect(() => onHotkeysChanged(() => setBindings(getStoredHotkeys())), []);

  // `layerEnabled` comes from a snapshot that only refreshes once a
  // second, so two presses of the same layer key inside one poll interval
  // would both read the same pre-toggle value and both dispatch the SAME
  // target state -- the layer ends up off when the operator expects the
  // second press to have put it back on. pendingLayers holds the value
  // this hook last asked for, and the toggle is computed from that
  // whenever it's still ahead of the snapshot.
  const pendingLayersRef = useRef<{ scene: string | null; kinds: Partial<Record<LayerKind, boolean>> }>({
    scene: activeSceneName,
    kinds: {},
  });

  // Retire a pending value as soon as the snapshot agrees with it (the
  // round trip landed), and drop the whole set on a scene change -- a
  // pending toggle is only ever meaningful for the scene it was sent for.
  useEffect(() => {
    const pending = pendingLayersRef.current;
    if (pending.scene !== activeSceneName) {
      pendingLayersRef.current = { scene: activeSceneName, kinds: {} };
      return;
    }
    for (const kind of Object.keys(pending.kinds) as LayerKind[]) {
      if (layerEnabled[kind] === pending.kinds[kind]) {
        delete pending.kinds[kind];
      }
    }
  }, [layerEnabled, activeSceneName]);

  useEffect(() => {
    const keyToAction = new Map<string, HotkeyActionId>();
    for (const action of HOTKEY_ACTIONS) {
      keyToAction.set(bindings[action.id], action.id);
    }

    function onKeyDown(event: KeyboardEvent) {
      if (isTypingTarget(event.target)) {
        return;
      }

      // Settings > Hotkeys is recording this keystroke as a new binding,
      // so it must not also *fire* as one (lib/hotkeys.ts's own
      // beginHotkeyCapture comment covers why DOM propagation control
      // can't do this job here).
      if (isHotkeyCaptureActive()) {
        return;
      }

      // OS key auto-repeat fires ~30 keydowns/second while a key is held.
      // Every action in this set is a discrete transport command, not a
      // hold-to-ramp one -- without this guard, holding the tap-tempo key
      // fills createTapTempoRecorder's buffer with keyboard-repeat
      // intervals and TapTempo computes a BPM from the repeat rate. (The
      // BPM nudges lose nothing either: they're computed off a snapshot
      // that only refreshes once a second, so a held key re-sends the same
      // target value rather than ramping.)
      if (event.repeat) {
        return;
      }

      // A Ctrl/Alt/Meta chord is always a menu-navigation shortcut
      // (useGlobalKeyboardWorkflow.ts's NAVIGATION_SHORTCUTS -- Ctrl+K,
      // Alt+Up/Down, Ctrl+Alt+Up/Down, Ctrl+,), never a playback one: every
      // rebindable action here is a bare key, so without this guard a
      // binding that happens to share a key with a chorded nav shortcut
      // (ArrowUp/ArrowDown/K default to bpmUp/bpmDown/nothing today, but a
      // rebind could pick any key) would silently fire both at once.
      if (event.ctrlKey || event.altKey || event.metaKey) {
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

      const actionId = keyToAction.get(normalizeHotkeyEvent(event));
      if (!actionId) {
        return;
      }

      const layerKind = LAYER_ACTION_TO_KIND[actionId];
      if (layerKind) {
        if (activeSceneName) {
          event.preventDefault();
          const pending = pendingLayersRef.current;
          if (pending.scene !== activeSceneName) {
            pendingLayersRef.current = { scene: activeSceneName, kinds: {} };
          }
          const current = pendingLayersRef.current.kinds[layerKind] ?? layerEnabled[layerKind];
          const next = !current;
          pendingLayersRef.current.kinds[layerKind] = next;
          void (async () => {
            const result = await dispatch.setLayerEnabled(activeSceneName, layerKind, next);
            // A rejected toggle never lands, so the snapshot will never
            // catch up to it -- drop the optimistic value immediately so
            // the next press works off real state again.
            const live = pendingLayersRef.current;
            if ((!result || result.exitCode !== 0) && live.kinds[layerKind] === next) {
              delete live.kinds[layerKind];
            }
          })();
        }
        return;
      }

      switch (actionId) {
        case "tapTempo":
          event.preventDefault();
          void dispatch.recordTap();
          return;
        case "bpmUp":
          event.preventDefault();
          void dispatch.setBPM(bpm + 1);
          return;
        case "bpmDown":
          event.preventDefault();
          void dispatch.setBPM(Math.max(1, bpm - 1));
          return;
        case "evaluate":
          event.preventDefault();
          void dispatch.evaluate(0);
          return;
      }
    }

    // capture: true keeps this window-scoped listener from being
    // intercepted by a stopPropagation() call deeper in the tree, while
    // still being ordinary in-webview DOM handling -- never the OS-level
    // golang.design/x/hotkey path (see file doc comment).
    window.addEventListener("keydown", onKeyDown, { capture: true });
    return () => window.removeEventListener("keydown", onKeyDown, { capture: true });
  }, [sceneNames, activeSceneName, layerEnabled, bpm, bindings]);
}
