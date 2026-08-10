// playbackDispatch.ts holds the shared playback dispatch primitives
// (WailsResult, LAYER_KINDS/LayerKind, PlaybackStateSummary, and the
// `dispatch` object) that both PlaybackControls.tsx's on-screen controls
// and useKeyboardWorkflow.ts's keydown handlers call -- the SAME
// functions, so the on-screen surface (PLAY-01) and the documented
// keyboard surface (PLAY-02) can never drift out of sync.
//
// Extracted out of PlaybackControls.tsx (06-06-PLAN.md) into its own
// module because PlaybackControls.tsx importing useKeyboardWorkflow AND
// useKeyboardWorkflow importing these constants back from
// PlaybackControls.tsx formed a circular ES module dependency. Whichever
// of the two modules the bundler evaluated first left `LAYER_KINDS` still
// undefined (temporal-dead-zone) at the exact moment
// useKeyboardWorkflow.ts's top-level `LAYER_KEY_TO_KIND` object literal
// dereferenced `LAYER_KINDS[0..3]`, crashing every load with "Cannot read
// properties of undefined (reading '0')" before React ever mounted --
// caught only now, during manual end-of-phase UAT, because every
// PLAY-01/PLAY-02 human-check was deferred to this point in the phase
// (workflow.human_verify_mode=end-of-phase) and this is the first time
// the real bundle has been loaded in a browser/webview. Neither
// PlaybackControls.tsx nor useKeyboardWorkflow.ts imports the other now;
// both import this file instead.

import { getPlaybackService } from "./wailsBridge";

/** WailsResult mirrors internal/wails.Result's JSON tags exactly (0
 * success, 1 command failure, 2 routing/usage/startup failure). */
export interface WailsResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

/** The exact four fixed layer kinds every scene carries (mirrors
 * internal/scene/scene.go's layerPriority order). */
export const LAYER_KINDS = ["base_look", "color_theme", "chase", "motion"] as const;
export type LayerKind = (typeof LAYER_KINDS)[number];

export const LAYER_LABELS: Record<LayerKind, string> = {
  base_look: "Base Look",
  color_theme: "Color Theme",
  chase: "Chase",
  motion: "Motion",
};

interface LayerSummary {
  kind: LayerKind;
  enabled: boolean;
  ref?: string;
}

interface SceneSummary {
  name: string;
  active: boolean;
  barsPerLoop: number;
  layers: LayerSummary[];
}

export interface PlaybackStateSummary {
  bpm: number;
  scenes: SceneSummary[];
}

// Wails v2 injects window.go.<goPackageName>.<StructName> at runtime for
// every struct bound via cmd/golc-desktop/main.go's options.App{Bind:
// [...]} -- internal/wails.PlaybackService's Go package name is "wails".
// This module no longer reads that global itself: wailsBridge.ts owns
// every window.go access and exports getPlaybackService, which carries the
// same `typeof window` guard and the same undefined-when-unbound contract
// this file's own accessor used to. wailsBridge.ts imports only leaf
// modules that never import it back at runtime (today: wailsEventSchemas.ts,
// whose own reference to wailsBridge's types is an erased `import type`),
// so depending on it here cannot reintroduce the circular-import class of
// bug documented at the top of this file.
const playbackService = getPlaybackService;

const TAP_RESET_GAP_MS = 2000;
const TAP_HISTORY_MAX = 8;

/** createTapTempoRecorder returns a single shared "record one tap"
 * closure: on-screen tap-tempo clicks and the keyboard workflow's Space
 * key both feed the SAME accumulating tap buffer (reset after a >2s gap),
 * so an operator can mix mouse and keyboard taps into one coherent tempo
 * rather than each surface keeping its own disjoint buffer. Fewer than
 * two accumulated taps is a no-op (internal/command/playback.go's
 * "playback bpm tap" route itself requires at least two). */
function createTapTempoRecorder() {
  let taps: string[] = [];
  let lastTapAt = 0;

  return async function recordTap(): Promise<WailsResult | undefined> {
    const now = Date.now();
    if (taps.length > 0 && now - lastTapAt > TAP_RESET_GAP_MS) {
      taps = [];
    }
    lastTapAt = now;
    taps.push(new Date(now).toISOString());
    if (taps.length > TAP_HISTORY_MAX) {
      taps = taps.slice(taps.length - TAP_HISTORY_MAX);
    }
    if (taps.length < 2) {
      return undefined;
    }
    return playbackService()?.TapTempo(taps);
  };
}

const recordTap = createTapTempoRecorder();

/** dispatch is the single source of truth for every playback action.
 * PlaybackControls' own on-screen controls AND useKeyboardWorkflow's
 * keydown handlers both call these exact functions -- never a second,
 * parallel implementation -- so the on-screen surface (PLAY-01) and the
 * documented keyboard surface (PLAY-02) always reach the identical
 * workflow. */
export const dispatch = {
  async switchScene(sceneName: string): Promise<WailsResult | undefined> {
    return playbackService()?.SwitchScene(sceneName);
  },
  async setLayerEnabled(sceneName: string, kind: LayerKind, enabled: boolean): Promise<WailsResult | undefined> {
    return playbackService()?.SetLayerEnabled(sceneName, kind, enabled);
  },
  async setBPM(bpm: number): Promise<WailsResult | undefined> {
    return playbackService()?.SetBPM(bpm);
  },
  recordTap,
  async evaluate(at: number): Promise<WailsResult | undefined> {
    return playbackService()?.Evaluate(at);
  },
  async getState(): Promise<PlaybackStateSummary | undefined> {
    const result = await playbackService()?.GetState();
    if (!result || result.exitCode !== 0 || !result.stdout) {
      return undefined;
    }
    return JSON.parse(result.stdout) as PlaybackStateSummary;
  },
};
