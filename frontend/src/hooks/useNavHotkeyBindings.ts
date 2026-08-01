// useNavHotkeyBindings mirrors useHotkeyBindings.ts for lib/hotkeys.ts's
// chorded navigation bindings (NAV_ACTIONS) -- a live view that re-reads
// localStorage whenever a Settings rebind (or a cross-tab "storage" event)
// fires, so useGlobalKeyboardWorkflow.ts's matcher and the '?' reference
// panel can never act on/show a stale chord after the operator changes one.
import { useEffect, useState } from "react";

import { getStoredNavHotkeys, onHotkeysChanged, type NavHotkeyBindings } from "../lib/hotkeys";

export function useNavHotkeyBindings(): NavHotkeyBindings {
  const [bindings, setBindings] = useState<NavHotkeyBindings>(() => getStoredNavHotkeys());

  useEffect(() => onHotkeysChanged(() => setBindings(getStoredNavHotkeys())), []);

  return bindings;
}
