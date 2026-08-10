// useGlobalKeyboardWorkflow mounts the playback keyboard shortcuts
// (useKeyboardWorkflow) once at shell level (shell restructure plan Step
// 5) so they fire globally regardless of which workspace is active --
// WORKFLOW-MAP.md's keyboard model treats scene-switch/layer-toggle as
// show-wide transport, not workspace-local UI, and the shell's own
// interaction contract states navigation "does not mutate show playback or
// output," implying playback control must stay reachable independent of
// nav position. Moved out of PlaybackControls.tsx, which owned this call
// exclusively before the restructure.
//
// Also owns the '?'/Esc help-overlay toggle (HelpOverlay.tsx, Step 8) --
// kept OUT of useKeyboardWorkflow.ts itself so that hook's own doc comment
// describing it as strictly the PLAY-02 playback action set stays
// accurate and untouched.
//
// And owns the menu-navigation shortcuts documented in lib/hotkeys.ts's
// NAV_ACTIONS (Discord's Alt+Up/Down channel-nav and Ctrl+Alt+Up/Down
// server-nav conventions, plus a Ctrl+K quick switcher and a Ctrl+,
// jump-to-Settings) -- a second, separate window keydown listener, matched
// against NAV_ACTIONS' persisted chord bindings (Settings > Hotkeys) via
// lib/hotkeys.ts's normalizeHotkeyChord, the chord equivalent of
// useKeyboardWorkflow.ts's own bare-key matcher. Every nav chord requires a
// Ctrl/Alt modifier, so unlike '?' this safely bypasses the isTypingTarget
// guard: no ordinary typing produces Ctrl+K or Alt+ArrowUp.
//
// These used to navigate directly, bypassing GuidedFirstShowContext's
// leave-the-guide confirm prompt -- the destination changed underneath the
// still-open guide overlay with zero visible effect. This hook is still
// instantiated above GuidedFirstShowProvider in AppShell.tsx, so
// useGuidedFirstShow() genuinely isn't reachable from here; instead
// `onNavigate` is now AppShell's `navigateGuarded`, which forwards to
// GuardedCommandRail's own already-guarded select handler (see its doc
// comment). Nothing about this hook's contract changed: it still just
// calls the navigate function it was handed.
import { useEffect, useMemo, useState } from "react";

import { useKeyboardWorkflow } from "../hooks/useKeyboardWorkflow";
import { usePlaybackSnapshot } from "./PlaybackSnapshotContext";
import { NAV_GROUPS, type DestinationId } from "./navigation";
import {
  NAV_ACTIONS,
  getStoredNavHotkeys,
  isHotkeyCaptureActive,
  normalizeHotkeyChord,
  onHotkeysChanged,
  type NavActionId,
  type NavHotkeyBindings,
} from "../lib/hotkeys";

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || target.isContentEditable;
}

/** FLAT_DESTINATIONS mirrors NAV_GROUPS as a single ordered list with each
 * destination's group index attached -- what the group/item-within-group
 * navigation below and QuickSwitcher.tsx's search both iterate. */
const FLAT_DESTINATIONS: Array<{ id: DestinationId; groupIndex: number }> = NAV_GROUPS.flatMap((group, groupIndex) =>
  group.destinations.map((destination) => ({ id: destination.id, groupIndex })),
);

function stepWithinGroup(active: DestinationId, direction: 1 | -1): DestinationId {
  const current = FLAT_DESTINATIONS.find((entry) => entry.id === active);
  if (!current) {
    return active;
  }
  const group = FLAT_DESTINATIONS.filter((entry) => entry.groupIndex === current.groupIndex);
  const index = group.findIndex((entry) => entry.id === active);
  const next = group[(index + direction + group.length) % group.length];
  return next.id;
}

function stepGroup(active: DestinationId, direction: 1 | -1): DestinationId {
  const current = FLAT_DESTINATIONS.find((entry) => entry.id === active);
  if (!current) {
    return active;
  }
  const groupCount = NAV_GROUPS.length;
  const nextGroupIndex = (current.groupIndex + direction + groupCount) % groupCount;
  const firstInGroup = FLAT_DESTINATIONS.find((entry) => entry.groupIndex === nextGroupIndex);
  return firstInGroup?.id ?? active;
}

export interface UseGlobalKeyboardWorkflowOptions {
  /** The currently active nav destination and its setter -- both owned by
   * AppShell.tsx's ShellBody as plain state (navigation.ts's own doc
   * comment: nav selection never round-trips through a Wails binding, so
   * it doesn't live in useGolcStore). Optional so this hook still works
   * (playback shortcuts and '?' help) in any test harness that doesn't
   * wire navigation up. */
  activeDestination?: DestinationId;
  onNavigate?: (id: DestinationId) => void;
}

export interface GlobalKeyboardWorkflow {
  helpOpen: boolean;
  closeHelp: () => void;
  quickSwitcherOpen: boolean;
  closeQuickSwitcher: () => void;
}

export function useGlobalKeyboardWorkflow(options: UseGlobalKeyboardWorkflowOptions = {}): GlobalKeyboardWorkflow {
  const { activeDestination, onNavigate } = options;
  const { state } = usePlaybackSnapshot();
  const [helpOpen, setHelpOpen] = useState(false);
  const [quickSwitcherOpen, setQuickSwitcherOpen] = useState(false);
  const [navBindings, setNavBindings] = useState<NavHotkeyBindings>(() => getStoredNavHotkeys());

  useEffect(() => onHotkeysChanged(() => setNavBindings(getStoredNavHotkeys())), []);

  const activeScene = state?.scenes?.find((scene) => scene.active) ?? state?.scenes?.[0];
  const activeSceneName = activeScene?.name ?? null;
  // Both of these feed useKeyboardWorkflow's effect dependency array, so a
  // fresh object/array literal per render would tear down and re-add its
  // window keydown listener on every render -- and this hook sits at shell
  // level under a 1s snapshot poll, so that was happening at least once a
  // second for the whole session. Query applies structural sharing to
  // `state`, so an unchanged poll returns the *same* `scenes` reference
  // and these memos genuinely hold across polls.
  const layerEnabled = useMemo(() => {
    const out: Record<string, boolean> = {};
    for (const layer of activeScene?.layers ?? []) {
      out[layer.kind] = layer.enabled;
    }
    return out;
  }, [activeScene]);
  const sceneNames = useMemo(() => state?.scenes?.map((scene) => scene.name) ?? [], [state?.scenes]);
  const bpm = state?.bpm ?? 0;

  useKeyboardWorkflow({ sceneNames, activeSceneName, layerEnabled, bpm });

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (isTypingTarget(event.target)) {
        return;
      }
      // Held '?'/Escape must not machine-gun the overlay open and shut,
      // and a keystroke Settings > Hotkeys is recording must not also
      // fire. Same two guards useKeyboardWorkflow.ts applies.
      if (event.repeat || isHotkeyCaptureActive()) {
        return;
      }
      if (event.key === "?") {
        event.preventDefault();
        setHelpOpen((current) => !current);
        return;
      }
      if (event.key === "Escape") {
        setHelpOpen(false);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    const chordToAction = new Map<string, NavActionId>();
    for (const action of NAV_ACTIONS) {
      chordToAction.set(navBindings[action.id], action.id);
    }

    function onKeyDown(event: KeyboardEvent) {
      if (event.repeat || isHotkeyCaptureActive()) {
        return;
      }
      const chord = normalizeHotkeyChord(event);
      if (!chord) {
        return;
      }
      const actionId = chordToAction.get(chord);
      if (!actionId) {
        return;
      }

      if (actionId === "openQuickSwitcher") {
        event.preventDefault();
        setQuickSwitcherOpen((current) => !current);
        return;
      }

      if (!onNavigate || !activeDestination) {
        return;
      }

      switch (actionId) {
        case "openSettings":
          event.preventDefault();
          onNavigate("show-settings");
          return;
        case "navPrevInGroup":
          event.preventDefault();
          onNavigate(stepWithinGroup(activeDestination, -1));
          return;
        case "navNextInGroup":
          event.preventDefault();
          onNavigate(stepWithinGroup(activeDestination, 1));
          return;
        case "navPrevGroup":
          event.preventDefault();
          onNavigate(stepGroup(activeDestination, -1));
          return;
        case "navNextGroup":
          event.preventDefault();
          onNavigate(stepGroup(activeDestination, 1));
          return;
      }
    }
    window.addEventListener("keydown", onKeyDown, { capture: true });
    return () => window.removeEventListener("keydown", onKeyDown, { capture: true });
  }, [activeDestination, onNavigate, navBindings]);

  return {
    helpOpen,
    closeHelp: () => setHelpOpen(false),
    quickSwitcherOpen,
    closeQuickSwitcher: () => setQuickSwitcherOpen(false),
  };
}
