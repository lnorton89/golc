// GuidedFirstShowContext is the Guided First Show's open/exit/navigate
// state and the once-per-process auto-launch guard (09-03-PLAN.md, D-08/
// D-10). Lives above WorkspaceRouter in AppShell.tsx (wrapping
// ShellCanvas) so it survives whichever workspace is currently mounted --
// this is what makes requestAutoLaunch's ref-guard actually hold across
// Overview's own unmount/remount cycles (e.g. after Exit Guide navigates
// back), rather than resetting every time Overview happens to remount.
//
// startGuide()/exitGuide()/navigateTo() never mutate anything themselves
// -- they only flip `open` and hand the active-destination string back to
// the caller-supplied onNavigate (AppShell's own setActiveDestination),
// exactly mirroring CommandRail's onSelect contract so no second
// navigation mechanism exists.
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";

import type { DestinationId } from "../../../shell/navigation";
import { GUIDE_STAGES, type GuideStageId } from "./stages";

interface GuidedFirstShowContextValue {
  open: boolean;
  stage: GuideStageId;
  setStage: (stage: GuideStageId) => void;
  /** startGuide records the destination active at entry and opens the
   * guide (the manual "Start Guide" action on Overview, D-10). */
  startGuide: () => void;
  /** exitGuide closes the guide and navigates back to the destination
   * recorded at entry, preserving `stage` in state so re-entry resumes
   * where the operator left off (locked contract: "Exiting retains
   * completed evidence and current stage"). Never disabled, never
   * destructive -- it always retains progress. */
  exitGuide: () => void;
  /** navigateTo closes the guide and navigates to `destination` -- used
   * by a stage's own primary action to hand off to the real workspace it
   * points at (e.g. Fixtures -> Fixture Library, Patch -> Patch & Pools).
   * Never mutates anything itself. */
  navigateTo: (destination: DestinationId) => void;
  /** requestAutoLaunch opens the guide only the first time it is ever
   * called in this process's lifetime (D-08: a genuinely empty show).
   * Tracked by a ref that exitGuide never resets, so Exit Guide always
   * actually escapes rather than re-trapping the operator on the next
   * Overview mount. */
  requestAutoLaunch: () => void;
}

const GuidedFirstShowContext = createContext<GuidedFirstShowContextValue | null>(null);

interface GuidedFirstShowProviderProps {
  activeDestination: DestinationId;
  onNavigate: (destination: DestinationId) => void;
  children: ReactNode;
}

export function GuidedFirstShowProvider({
  activeDestination,
  onNavigate,
  children,
}: GuidedFirstShowProviderProps) {
  const [open, setOpen] = useState(false);
  const [stage, setStage] = useState<GuideStageId>(GUIDE_STAGES[0]);
  const entryDestinationRef = useRef<DestinationId>(activeDestination);
  const autoLaunchedRef = useRef(false);

  const startGuide = useCallback(() => {
    entryDestinationRef.current = activeDestination;
    setOpen(true);
  }, [activeDestination]);

  const exitGuide = useCallback(() => {
    setOpen(false);
    onNavigate(entryDestinationRef.current);
  }, [onNavigate]);

  const navigateTo = useCallback(
    (destination: DestinationId) => {
      setOpen(false);
      onNavigate(destination);
    },
    [onNavigate],
  );

  const requestAutoLaunch = useCallback(() => {
    if (autoLaunchedRef.current) return;
    autoLaunchedRef.current = true;
    entryDestinationRef.current = activeDestination;
    setOpen(true);
  }, [activeDestination]);

  const value: GuidedFirstShowContextValue = {
    open,
    stage,
    setStage,
    startGuide,
    exitGuide,
    navigateTo,
    requestAutoLaunch,
  };

  return <GuidedFirstShowContext.Provider value={value}>{children}</GuidedFirstShowContext.Provider>;
}

export function useGuidedFirstShow(): GuidedFirstShowContextValue {
  const ctx = useContext(GuidedFirstShowContext);
  if (!ctx) {
    throw new Error("useGuidedFirstShow must be used within a GuidedFirstShowProvider");
  }
  return ctx;
}
