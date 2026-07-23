// wailsBridge.ts centralizes this frontend's only two touchpoints with
// the Wails-injected browser globals: window.go.wails.SafetyService.* (the
// generated bindings for internal/wails/svc_safety.go's bound methods) and
// window.runtime.EventsOn (the generated subscription for
// internal/wails/events.go's throttled "status:update" push). 06-05-
// PLAN.md's SafetyCluster and LiveStatusBar both import from here rather
// than referencing window.go/window.runtime directly, so every future
// Wave 3/4 component (06-06/06-07/06-08) that needs a bound Go call or an
// EventsOn subscription follows the same one pattern instead of
// re-declaring ambient globals per file.
//
// Every export here degrades gracefully (never throws) when window.go/
// window.runtime are undefined -- e.g. during `npm run build`'s tsc
// type-check, a plain browser preview, or a future component test
// harness with no real Wails webview host. This mirrors D-13's own
// "safety cluster remains reachable regardless" contract at the bridge
// layer: a missing bridge degrades to an explicit unreachable/offline
// result, never a thrown exception that would crash the safety cluster's
// render tree.

/** WailsResult mirrors internal/wails.Result's JSON shape exactly
 * (ExitCode/Stdout/Stderr -> exitCode/stdout/stderr) -- every
 * SafetyService toggle method returns this. */
export interface WailsResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}

/** StatusSnapshot mirrors internal/wails.StatusSnapshot's JSON shape
 * exactly (06-05-PLAN.md Task 1, PLAY-07). enabledLayers is always a
 * present (never undefined/null) array -- the Go side guarantees this
 * for the identical "never blank/undefined" reason the daemon's own
 * playbackStatusPayload does. */
export interface StatusSnapshot {
  reachable: boolean;
  active: boolean;
  sceneId?: string;
  sceneName?: string;
  bpm: number;
  barIndex: number;
  beatFraction: number;
  enabledLayers: string[];
  controllingSource: string;
  outputState: string;
}

interface SafetyServiceBinding {
  Blackout(on: boolean): Promise<WailsResult>;
  StopReleaseAll(on: boolean): Promise<WailsResult>;
  RevokeAutomation(on: boolean): Promise<WailsResult>;
  FetchStatus(): Promise<StatusSnapshot>;
  /** SetActiveSurface (CR-01 fix): scopes Blackout/StopReleaseAll/
   * RevokeAutomation to surfaceName's assigned SafetyRefs server-side
   * (internal/wails/svc_safety.go's authorizeSafety); "" clears the
   * active surface, returning to unrestricted/author-mode dispatch. */
  SetActiveSurface(surfaceName: string): Promise<WailsResult>;
}

interface PlaybackServiceBinding {
  SwitchScene(sceneName: string): Promise<WailsResult>;
  SetLayerEnabled(sceneName: string, kind: string, enabled: boolean): Promise<WailsResult>;
  SetBPM(bpm: number): Promise<WailsResult>;
  TapTempo(timestamps: string[]): Promise<WailsResult>;
  Evaluate(at: number): Promise<WailsResult>;
  GetState(): Promise<WailsResult>;
  /** SetActiveSurface (CR-01 fix): scopes SwitchScene/SetLayerEnabled to
   * surfaceName's assigned scene/layer refs server-side
   * (internal/wails/svc_playback.go's authorizeControl); "" clears the
   * active surface, returning to unrestricted/author-mode dispatch. */
  SetActiveSurface(surfaceName: string): Promise<WailsResult>;
}

interface SurfaceControlRefInput {
  kind: "scene" | "layer" | "master" | "safety";
  scene?: string;
  layerKind?: string;
  masterKind?: "grand" | "group";
  group?: string;
  safety?: string;
}

interface SurfaceServiceBinding {
  CreateSurface(name: string): Promise<WailsResult>;
  ListSurfaces(): Promise<unknown[]>;
  AssignItem(surfaceName: string, controlRef: SurfaceControlRefInput): Promise<WailsResult>;
  UnassignItem(surfaceName: string, controlRef: SurfaceControlRefInput): Promise<WailsResult>;
  ShowSurface(surfaceName: string): Promise<unknown>;
  RemoveSurface(surfaceName: string): Promise<WailsResult>;
  AuthorizeControl(surfaceName: string, controlRef: SurfaceControlRefInput): Promise<WailsResult>;
}

/** MidiFeedback mirrors internal/wails.MidiFeedback's JSON shape exactly
 * (06-08-PLAN.md Task 2, D-09/D-10/D-11): Physical is the live physical
 * fader/button position (drives the on-screen slider even while not
 * armed), AppValue is the fixed ghost/target marker while unarmed or the
 * tracked controlling value once armed, and Armed reports whether the
 * cross-to-catch crossing has occurred -- always true for a Note/button
 * mapping (D-12: no arming delay). */
export interface MidiFeedback {
  surfaceName: string;
  mappingId: string;
  kind: string;
  armed: boolean;
  appValue: number;
  physical: number;
}

interface MidiServiceBinding {
  StartLearn(surfaceName: string, controlRef: SurfaceControlRefInput): Promise<WailsResult>;
  CancelLearn(): Promise<WailsResult>;
  RemoveMapping(surfaceName: string, mappingId: string): Promise<WailsResult>;
  ListMappings(surfaceName: string): Promise<unknown[]>;
  SetActiveSurface(surfaceName: string): Promise<WailsResult>;
}

// Single, centralized `window.go.wails` shape (Wails v2's runtime-injected
// bridge, one property per struct bound in cmd/golc-desktop/main.go's
// options.App{Bind: [...]}). Every component imports its binding call
// through this file's helper functions -- or, for a service without a
// helper yet, casts through `window.go?.wails?.<Service>` -- rather than
// re-declaring `declare global { interface Window {...} } }` itself:
// TypeScript's declaration merging requires every `declare global`
// augmentation of the SAME inline-typed property (`go`) to be structurally
// identical, so multiple per-component declarations of different shapes
// for `go.wails` collide at compile time (#Wave3 post-merge gate finding).
// Add a new service's binding interface above and a property below when a
// future plan (06-08's MidiService) needs one.
declare global {
  interface Window {
    go?: {
      wails?: {
        SafetyService?: SafetyServiceBinding;
        PlaybackService?: PlaybackServiceBinding;
        SurfaceService?: SurfaceServiceBinding;
        MidiService?: MidiServiceBinding;
      };
    };
    runtime?: {
      EventsOn(
        eventName: string,
        callback: (...data: unknown[]) => void,
      ): () => void;
    };
  }
}

/** bridgeUnavailableResult is the explicit, non-throwing fallback every
 * SafetyService call returns when window.go.wails.SafetyService is not
 * present (D-13: the cluster stays interactive/renderable regardless --
 * it is the daemon connection, not this bridge, that the UI-SPEC
 * unreachable copy is about). */
function bridgeUnavailableResult(): WailsResult {
  return {
    exitCode: 1,
    stdout: "",
    stderr:
      "GOLC_WAILS_BRIDGE_UNAVAILABLE: not running inside the GOLC desktop shell",
  };
}

function safetyService(): SafetyServiceBinding | undefined {
  return window.go?.wails?.SafetyService;
}

function playbackServiceBridge(): PlaybackServiceBinding | undefined {
  return window.go?.wails?.PlaybackService;
}

/** setSafetyActiveSurface (CR-01 fix) calls the bound
 * SafetyService.SetActiveSurface, scoping Blackout/StopReleaseAll/
 * RevokeAutomation to surfaceName's assigned SafetyRefs server-side; pass
 * "" to clear the active surface and return to unrestricted/author-mode
 * dispatch. OperatorSurface.tsx's "Preview as Operator" toggle is the one
 * caller today. */
export async function setSafetyActiveSurface(
  surfaceName: string,
): Promise<WailsResult> {
  const svc = safetyService();
  if (!svc) return bridgeUnavailableResult();
  return svc.SetActiveSurface(surfaceName);
}

/** setPlaybackActiveSurface (CR-01 fix) calls the bound
 * PlaybackService.SetActiveSurface, scoping SwitchScene/SetLayerEnabled to
 * surfaceName's assigned scene/layer refs server-side; pass "" to clear
 * the active surface and return to unrestricted/author-mode dispatch.
 * OperatorSurface.tsx's "Preview as Operator" toggle is the one caller
 * today. */
export async function setPlaybackActiveSurface(
  surfaceName: string,
): Promise<WailsResult> {
  const svc = playbackServiceBridge();
  if (!svc) return bridgeUnavailableResult();
  return svc.SetActiveSurface(surfaceName);
}

/** offlineStatusSnapshot mirrors internal/wails.offlineStatusSnapshot's
 * explicit idle/offline projection -- the same fallback shape FetchStatus
 * returns Go-side, reused here so a missing bridge and an unreachable
 * daemon render identically in the frontend. */
export function offlineStatusSnapshot(): StatusSnapshot {
  return {
    reachable: false,
    active: false,
    enabledLayers: [],
    bpm: 0,
    barIndex: 0,
    beatFraction: 0,
    controllingSource: "offline",
    outputState: "offline",
  };
}

/** safetyBlackout dials+forwards "artnet safety blackout --on <on>
 * --source manual" via the bound SafetyService.Blackout. */
export async function safetyBlackout(on: boolean): Promise<WailsResult> {
  const svc = safetyService();
  if (!svc) return bridgeUnavailableResult();
  return svc.Blackout(on);
}

/** safetyStopReleaseAll dials+forwards "artnet safety stop-all --on <on>
 * --source manual" via the bound SafetyService.StopReleaseAll. */
export async function safetyStopReleaseAll(
  on: boolean,
): Promise<WailsResult> {
  const svc = safetyService();
  if (!svc) return bridgeUnavailableResult();
  return svc.StopReleaseAll(on);
}

/** safetyRevokeAutomation dials+forwards "artnet safety
 * revoke-automation --on <on> --source manual" via the bound
 * SafetyService.RevokeAutomation. */
export async function safetyRevokeAutomation(
  on: boolean,
): Promise<WailsResult> {
  const svc = safetyService();
  if (!svc) return bridgeUnavailableResult();
  return svc.RevokeAutomation(on);
}

/** fetchSafetyStatus calls the bound SafetyService.FetchStatus,
 * returning offlineStatusSnapshot() when the bridge is unavailable or the
 * call itself rejects -- callers never need their own try/catch. */
export async function fetchSafetyStatus(): Promise<StatusSnapshot> {
  const svc = safetyService();
  if (!svc) return offlineStatusSnapshot();
  try {
    return await svc.FetchStatus();
  } catch {
    return offlineStatusSnapshot();
  }
}

/** onStatusUpdate subscribes to the Go host's throttled "status:update"
 * EventsEmit push (internal/wails/events.go), invoking callback with each
 * pushed StatusSnapshot. Returns an unsubscribe function; a missing
 * bridge returns a no-op unsubscribe rather than throwing. This is a
 * throttled hint stream only (06-RESEARCH.md anti-pattern) -- callers
 * must still treat fetchSafetyStatus as the authoritative re-query on a
 * detected gap, never rely on this push alone. */
export function onStatusUpdate(
  callback: (snapshot: StatusSnapshot) => void,
): () => void {
  const runtime = window.runtime;
  if (!runtime) return () => {};
  return runtime.EventsOn("status:update", (...data: unknown[]) => {
    const snapshot = data[0] as StatusSnapshot | undefined;
    if (snapshot) callback(snapshot);
  });
}

/** onMidiFeedback subscribes to the Go host's throttled "midi:feedback"
 * EventsEmit push (internal/wails/events.go's QueueMidiFeedback,
 * 06-08-PLAN.md Task 2), invoking callback with each pushed MidiFeedback
 * (D-09/D-10/D-11). Returns an unsubscribe function; a missing bridge
 * returns a no-op unsubscribe rather than throwing -- mirrors
 * onStatusUpdate's identical contract. The crossing/arming decision
 * itself runs unthrottled Go-side (06-RESEARCH.md Open Question 3); this
 * push is only the throttled visual reflection. */
export function onMidiFeedback(
  callback: (feedback: MidiFeedback) => void,
): () => void {
  const runtime = window.runtime;
  if (!runtime) return () => {};
  return runtime.EventsOn("midi:feedback", (...data: unknown[]) => {
    const feedback = data[0] as MidiFeedback | undefined;
    if (feedback) callback(feedback);
  });
}
