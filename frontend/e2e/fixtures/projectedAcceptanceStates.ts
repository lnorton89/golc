// projectedAcceptanceStates.ts (Plan 13-41 Task 2, D-02/D-03/D-10/D-12/
// D-13/D-14, UI-SPEC's Safety and Authority Invariants: "An action's
// loading state cannot block the local priority path used by Revoke
// Automation or Blackout" and "Automation override must not depend on an
// AI provider, script runtime, or queued application command completing").
//
// Two named, typed projected states -- `providerOffline` and
// `daemonOffline` (matching this plan's own declared key_link pattern)
// -- each pairing an explicit, Go-owned playback/output truth (fed
// directly to the mocked `SafetyService.FetchStatus`, exactly the
// snapshot shape `internal/wails`'s real daemon emits) with the exact
// connectivity copy this app's own LiveStatusBar is expected to show (or
// not show) for that state.
//
// Why these two specific states, given this codebase has no separate
// "AI provider" integration yet (Phase 10, next milestone, PROJECT.md
// Active requirements): `daemonOffline` uses the ALREADY-REAL
// `StatusSnapshot.reachable` signal LiveStatusBar renders today (the
// connection to the Go playback daemon itself is down, while the daemon's
// own last-known truth about the scene/output stays authoritative and
// displayed, never replaced with a blank/idle placeholder).
// `providerOffline` models D-14's literal mandate structurally: the Go
// daemon itself is fully reachable and its truth is unaffected (reachable
// stays true, nothing about the daemon's own state changes), but the
// state's own narrative is that a hypothetical upstream AI/automation
// provider is unreachable -- exercising the real, load-bearing claim that
// Blackout/Revoke Automation are dispatched through the daemon's own local
// SafetyService path with ZERO observable coupling to any such external
// dependency's availability. Proving "zero coupling" this way (the
// operator-facing surface behaves identically whether or not a provider
// exists) is exactly what "must not depend on" means architecturally, and
// is the honest, testable claim this codebase can make until Phase 10
// ships a real provider integration to layer a live connectivity signal
// onto.
import type { Page } from "@playwright/test";
import type { StatusSnapshot } from "../../src/lib/wailsBridge";

export type ProjectedAcceptanceStateId = "provider-offline" | "daemon-offline";

export interface ProjectedAcceptanceState {
  id: ProjectedAcceptanceStateId;
  /** Human-readable description of the scenario this state models. */
  description: string;
  /** The single dependency this state's own connectivity copy may name as
   * unavailable -- never more than one, and never the daemon's own truth
   * itself. */
  unavailableDependency: string;
  /** Fed directly to the mocked SafetyService.FetchStatus -- explicit,
   * Go-owned playback/output truth this state asserts must survive
   * unchanged through every safety-control interaction. */
  goOwnedStatus: StatusSnapshot;
  /** A pattern LiveStatusBar's own unreachable-copy region must match, or
   * null when no such banner should render for this state (the daemon
   * itself is reachable). */
  expectedConnectivityCopyPattern: RegExp | null;
  /** Patterns that must NEVER appear anywhere in the live status region,
   * regardless of state -- an inferred "stopped" claim this state's own
   * explicit truth does not support, or the OTHER state's own unrelated
   * unavailable-dependency copy. */
  forbiddenCopyPatterns: RegExp[];
}

const SHARED_TRUTH: Omit<StatusSnapshot, "reachable" | "controllingSource"> = {
  active: true,
  sceneId: "scene-offline-1",
  sceneName: "Opening Look",
  bpm: 120,
  barIndex: 2,
  beatFraction: 0.25,
  enabledLayers: ["Color"],
  outputState: "live",
};

export const DAEMON_OFFLINE_STATE: ProjectedAcceptanceState = {
  id: "daemon-offline",
  description:
    "The connection to the Go playback daemon itself is unreachable (StatusSnapshot.reachable === false) while the daemon's own last-known truth -- an actively playing scene with live output -- remains authoritative and displayed, never replaced with a blank/idle placeholder.",
  unavailableDependency: "playback daemon connection",
  goOwnedStatus: { ...SHARED_TRUTH, reachable: false, controllingSource: "live" },
  expectedConnectivityCopyPattern: /Can.t reach the playback engine/i,
  forbiddenCopyPatterns: [/stopped/i, /no longer (playing|live)/i, /output.*(halt|stop)/i],
};

export const PROVIDER_OFFLINE_STATE: ProjectedAcceptanceState = {
  id: "provider-offline",
  description:
    "A hypothetical upstream AI/automation provider is unreachable while the Go daemon itself is fully reachable and its own truth is completely unaffected (reachable stays true) -- models D-14's 'must not depend on an AI provider' mandate as zero observable coupling: the operator-facing safety surface behaves identically to a fully-healthy daemon.",
  unavailableDependency: "AI automation provider",
  goOwnedStatus: { ...SHARED_TRUTH, reachable: true, controllingSource: "live" },
  expectedConnectivityCopyPattern: null,
  forbiddenCopyPatterns: [/Can.t reach the playback engine/i, /stopped/i, /no longer (playing|live)/i],
};

export const PROJECTED_ACCEPTANCE_STATES: ProjectedAcceptanceState[] = [DAEMON_OFFLINE_STATE, PROVIDER_OFFLINE_STATE];

/** SafetyCommandName mirrors the three SafetyService methods a hold-to-
 * confirm control in SafetyCluster.tsx dispatches. */
export type SafetyCommandName = "Blackout" | "RevokeAutomation" | "StopReleaseAll";

/** installProjectedAcceptanceState wires the mocked SafetyService.FetchStatus
 * to this state's own explicit goOwnedStatus, exactly as LiveStatusBar's
 * real fetchSafetyStatus() call would receive it from the real daemon. */
export async function installProjectedAcceptanceState(page: Page, state: ProjectedAcceptanceState): Promise<void> {
  await page.addInitScript((status: StatusSnapshot) => {
    const bw = window as unknown as { go: { wails: { SafetyService: Record<string, unknown> } } };
    bw.go.wails.SafetyService.FetchStatus = async () => status;
  }, state.goOwnedStatus);
}

/** installSafetyDispatchSpies wraps Blackout/RevokeAutomation/
 * StopReleaseAll so every real dispatch is counted and its exact toggle
 * argument recorded -- must run AFTER installHealthyBindings (which first
 * defines the three methods this wraps) and is safe to call from a
 * page.addInitScript registered afterward, since addInitScript scripts
 * fire in registration order before any page script, mirroring
 * designSystem.ts's installCalibrationBindings precedent. */
export async function installSafetyDispatchSpies(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const bw = window as unknown as {
      go: { wails: { SafetyService: Record<string, (...args: unknown[]) => unknown> } };
      __safetySpy: Record<string, { count: number; args: unknown[][] }>;
    };
    bw.__safetySpy = {
      Blackout: { count: 0, args: [] },
      RevokeAutomation: { count: 0, args: [] },
      StopReleaseAll: { count: 0, args: [] },
    };
    for (const name of ["Blackout", "RevokeAutomation", "StopReleaseAll"] as const) {
      const original = bw.go.wails.SafetyService[name];
      bw.go.wails.SafetyService[name] = async (...args: unknown[]) => {
        bw.__safetySpy[name].count += 1;
        bw.__safetySpy[name].args.push(args);
        return (original as (...a: unknown[]) => unknown)(...args);
      };
    }
  });
}

export interface SafetyDispatchSpySnapshot {
  Blackout: { count: number; args: unknown[][] };
  RevokeAutomation: { count: number; args: unknown[][] };
  StopReleaseAll: { count: number; args: unknown[][] };
}

export async function readSafetyDispatchSpies(page: Page): Promise<SafetyDispatchSpySnapshot> {
  return page.evaluate(
    () => (window as unknown as { __safetySpy: SafetyDispatchSpySnapshot }).__safetySpy,
  );
}
