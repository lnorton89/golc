// KeyboardShortcuts.tsx fills 06-04-PLAN.md Task 2's stub with the
// documented reference panel for the in-webview keyboard workflow
// (06-06-PLAN.md Task 2, PLAY-02): every shortcut listed here is read from
// lib/hotkeys.ts's HOTKEY_ACTIONS and NAV_ACTIONS, resolved against the
// operator's live bindings (useHotkeyBindings/useNavHotkeyBindings) -- the
// same bindings useKeyboardWorkflow.ts/useGlobalKeyboardWorkflow.ts's own
// keydown handlers match against -- so a rebind made in Settings > Hotkeys
// is reflected here immediately, and the reference panel can never show a
// stale key. Shortcuts are grouped by category (Scenes/Layers/Tempo/
// Transport/Navigation) and the group list scrolls within a fixed-height
// area once it exceeds one screen (06-UI-SPEC.md overflow backstop:
// "Panel scrolls or groups shortcuts by category once content exceeds one
// screen") -- ScrollRegion below is the shared bounded-scroll primitive
// that backstop maps onto.
//
// Mounted from shell/HelpOverlay.tsx (toggled by '?' via
// useGlobalKeyboardWorkflow.ts).

import { Panel, ScrollRegion } from "../../design-system";
import { useHotkeyBindings } from "../../hooks/useHotkeyBindings";
import { useNavHotkeyBindings } from "../../hooks/useNavHotkeyBindings";
import { HOTKEY_ACTIONS, NAV_ACTIONS, SCENE_SWITCH_SHORTCUT, formatChordLabel, formatHotkeyLabel } from "../../lib/hotkeys";
import styles from "./KeyboardShortcuts.module.css";

interface DisplayShortcut {
  category: string;
  keys: string;
  description: string;
}

export default function KeyboardShortcuts() {
  const bindings = useHotkeyBindings();
  const navBindings = useNavHotkeyBindings();

  const shortcuts: DisplayShortcut[] = [
    SCENE_SWITCH_SHORTCUT,
    ...HOTKEY_ACTIONS.map((action) => ({
      category: action.category,
      keys: formatHotkeyLabel(bindings[action.id]),
      description: action.description,
    })),
    ...NAV_ACTIONS.map((action) => ({
      category: action.category,
      keys: formatChordLabel(navBindings[action.id]),
      description: action.description,
    })),
  ];

  const categories: string[] = [];
  const byCategory = new Map<string, DisplayShortcut[]>();
  for (const shortcut of shortcuts) {
    const existing = byCategory.get(shortcut.category);
    if (existing) {
      existing.push(shortcut);
    } else {
      byCategory.set(shortcut.category, [shortcut]);
      categories.push(shortcut.category);
    }
  }

  return (
    <Panel className={styles.panel} aria-label="Keyboard shortcuts reference">
      <h2 className={styles.heading}>Keyboard Shortcuts</h2>
      <ScrollRegion className={styles.scrollArea}>
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
      </ScrollRegion>
    </Panel>
  );
}
