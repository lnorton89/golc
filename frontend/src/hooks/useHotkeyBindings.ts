// useHotkeyBindings gives components a live view of lib/hotkeys.ts's
// persisted bindings -- re-reading localStorage whenever a Settings rebind
// (or a cross-tab "storage" event) fires, so the '?' reference panel and
// useKeyboardWorkflow.ts's own matcher can never show/act on a stale
// binding after the operator changes one.
import { useEffect, useState } from "react";

import { getStoredHotkeys, onHotkeysChanged, type HotkeyBindings } from "../lib/hotkeys";

export function useHotkeyBindings(): HotkeyBindings {
  const [bindings, setBindings] = useState<HotkeyBindings>(() => getStoredHotkeys());

  useEffect(() => onHotkeysChanged(() => setBindings(getStoredHotkeys())), []);

  return bindings;
}
