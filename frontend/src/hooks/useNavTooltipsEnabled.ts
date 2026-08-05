import { useSyncExternalStore } from "react";

import { getStoredNavTooltipsEnabled, subscribeNavTooltipsEnabled } from "../lib/navTooltips";

/** Reactive read of the nav-tooltips preference (lib/navTooltips.ts) --
 * every subscriber re-renders the instant NavTooltipsToggle flips it,
 * rather than only on next mount. */
export function useNavTooltipsEnabled(): boolean {
  return useSyncExternalStore(subscribeNavTooltipsEnabled, getStoredNavTooltipsEnabled);
}
