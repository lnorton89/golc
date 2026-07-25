// InspectorSlot is the cross-tree channel a workspace uses to render
// content into the shell's ContextualInspector -- the one case (per the
// shell restructure plan Step 3) where a workspace several levels deep
// needs to reach a sibling-of-its-parent region.
//
// Implemented as a React portal (createPortal into the inspector's DOM
// node), not state lifted to a common ancestor -- an earlier state-based
// design (useInspectorSlot calling a setState passed down via context)
// caused a genuine infinite render loop: publishing a fresh JSX object
// every render re-triggered the publishing effect every render, and
// because the state lived in AppShell (an ancestor of the workspace doing
// the publishing), every publish re-rendered the workspace, which
// recreated the JSX, forever. A portal has no such cycle -- it renders
// directly into the target DOM node with no state round-trip.
import { createContext, useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";

const InspectorContainerContext = createContext<HTMLElement | null>(null);

interface InspectorPortalProviderProps {
  container: HTMLElement | null;
  children: ReactNode;
}

export function InspectorPortalProvider({ container, children }: InspectorPortalProviderProps) {
  return <InspectorContainerContext.Provider value={container}>{children}</InspectorContainerContext.Provider>;
}

/** Renders `node` into the shell's contextual inspector via a portal.
 * Render the returned value somewhere in the calling component's own JSX
 * tree (position doesn't affect where it visually appears -- only where
 * it appears in the React tree for context/lifecycle purposes). Returns
 * null before the inspector's DOM container exists yet (first paint). */
export function useInspectorSlot(node: ReactNode): ReactNode {
  const container = useContext(InspectorContainerContext);
  if (!container) {
    return null;
  }
  return createPortal(node, container);
}
