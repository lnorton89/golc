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
import { useEffect, useState } from "react";

import { useKeyboardWorkflow } from "../hooks/useKeyboardWorkflow";
import { usePlaybackSnapshot } from "./PlaybackSnapshotContext";

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || target.isContentEditable;
}

export interface GlobalKeyboardWorkflow {
  helpOpen: boolean;
  closeHelp: () => void;
}

export function useGlobalKeyboardWorkflow(): GlobalKeyboardWorkflow {
  const { state } = usePlaybackSnapshot();
  const [helpOpen, setHelpOpen] = useState(false);

  const activeScene = state?.scenes?.find((scene) => scene.active) ?? state?.scenes?.[0];
  const activeSceneName = activeScene?.name ?? null;
  const layerEnabled: Record<string, boolean> = {};
  for (const layer of activeScene?.layers ?? []) {
    layerEnabled[layer.kind] = layer.enabled;
  }
  const sceneNames = state?.scenes?.map((scene) => scene.name) ?? [];
  const bpm = state?.bpm ?? 0;

  useKeyboardWorkflow({ sceneNames, activeSceneName, layerEnabled, bpm });

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (isTypingTarget(event.target)) {
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

  return { helpOpen, closeHelp: () => setHelpOpen(false) };
}
