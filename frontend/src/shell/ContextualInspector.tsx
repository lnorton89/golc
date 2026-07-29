// ContextualInspector is the shell's right-edge aside (application-shell-
// navigation.md). It is the portal TARGET for useInspectorSlot -- its DOM
// node is handed up to AppShell via onContainerReady so workspaces can
// portal content into it.
//
// onHasContentChange reports whether the aside currently has any portaled
// children, via a MutationObserver on its own childList -- AppShell uses
// this to size/reveal the column it lives in (a CSS-only `:has(aside:not(
// :empty))` approach was tried first and dropped: it matched correctly per
// element.matches(), but a portal's content mounting into the aside AFTER
// this component's own initial paint did not reliably re-trigger the
// ancestor's style recalculation, leaving the column stuck collapsed. A
// MutationObserver has no such indirect-invalidation edge case -- it fires
// exactly when the DOM it's watching actually changes.
import { useCallback, useEffect, useRef } from "react";

import styles from "./ContextualInspector.module.css";

interface ContextualInspectorProps {
  onContainerReady: (node: HTMLDivElement | null) => void;
  onHasContentChange: (hasContent: boolean) => void;
}

export default function ContextualInspector({ onContainerReady, onHasContentChange }: ContextualInspectorProps) {
  const nodeRef = useRef<HTMLDivElement | null>(null);

  const setRef = useCallback(
    (node: HTMLDivElement | null) => {
      nodeRef.current = node;
      onContainerReady(node);
    },
    [onContainerReady],
  );

  useEffect(() => {
    const node = nodeRef.current;
    if (!node) return;

    const reportHasContent = () => onHasContentChange(node.childNodes.length > 0);
    reportHasContent();

    const observer = new MutationObserver(reportHasContent);
    observer.observe(node, { childList: true });
    return () => observer.disconnect();
    // nodeRef.current only changes identity via setRef (mount/unmount);
    // re-running this effect per render would just tear down and
    // reattach the identical observer on the identical node for no
    // reason, so it's keyed on the stable callback instead.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onHasContentChange]);

  return <aside ref={setRef} className={styles.inspector} aria-label="Details" />;
}
