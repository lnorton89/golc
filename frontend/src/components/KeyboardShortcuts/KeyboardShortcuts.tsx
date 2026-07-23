// KeyboardShortcuts.tsx fills 06-04-PLAN.md Task 2's stub with the
// documented reference panel for the in-webview keyboard workflow
// (06-06-PLAN.md Task 2, PLAY-02): every shortcut listed here is read
// directly from frontend/src/hooks/useKeyboardWorkflow.ts's
// PLAYBACK_SHORTCUTS constant -- the same list that hook's own keydown
// handler implements -- so the reference panel and the actual key
// bindings can never drift apart. Shortcuts are grouped by category
// (Scenes/Layers/Tempo/Transport) and the group list scrolls within a
// fixed-height area once it exceeds one screen (06-UI-SPEC.md overflow
// backstop: "Panel scrolls or groups shortcuts by category once content
// exceeds one screen").
//
// This component is mounted from PlaybackControls.tsx (toggled by a
// "Keyboard Shortcuts" button) rather than from App.tsx directly --
// App.tsx's layout/mount points are never edited by Wave 3/4 plans
// (06-04-PLAN.md Task 2's contract).

import { PLAYBACK_SHORTCUTS } from "../../hooks/useKeyboardWorkflow";
import styles from "./KeyboardShortcuts.module.css";

export default function KeyboardShortcuts() {
  const categories: string[] = [];
  const byCategory = new Map<string, typeof PLAYBACK_SHORTCUTS>();
  for (const shortcut of PLAYBACK_SHORTCUTS) {
    const existing = byCategory.get(shortcut.category);
    if (existing) {
      existing.push(shortcut);
    } else {
      byCategory.set(shortcut.category, [shortcut]);
      categories.push(shortcut.category);
    }
  }

  return (
    <section className={styles.panel} aria-label="Keyboard shortcuts reference">
      <h2 className={styles.heading}>Keyboard Shortcuts</h2>
      <div className={styles.scrollArea}>
        {categories.map((category) => (
          <div key={category} className={styles.group}>
            <h3 className={styles.groupHeading}>{category}</h3>
            <ul className={styles.list}>
              {(byCategory.get(category) ?? []).map((shortcut) => (
                <li key={`${category}-${shortcut.keys}-${shortcut.description}`} className={styles.row}>
                  <kbd className={styles.keys}>{shortcut.keys}</kbd>
                  <span className={styles.description}>{shortcut.description}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}
