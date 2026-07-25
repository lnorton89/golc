// ContextualInspector is the shell's right-edge aside (application-shell-
// navigation.md). It is the portal TARGET for useInspectorSlot -- its DOM
// node is handed up to AppShell via onContainerReady so workspaces can
// portal content into it. The empty state ("Select an item to see
// details.") is pure CSS (:empty::before in ContextualInspector.module.css)
// rather than a React child, since a React-rendered fallback child would
// coexist in the DOM alongside any portaled content instead of being
// replaced by it.
import { useCallback } from "react";

import styles from "./ContextualInspector.module.css";

interface ContextualInspectorProps {
  onContainerReady: (node: HTMLDivElement | null) => void;
}

export default function ContextualInspector({ onContainerReady }: ContextualInspectorProps) {
  const setRef = useCallback(
    (node: HTMLDivElement | null) => {
      onContainerReady(node);
    },
    [onContainerReady],
  );

  return <aside ref={setRef} className={styles.inspector} aria-label="Details" />;
}
