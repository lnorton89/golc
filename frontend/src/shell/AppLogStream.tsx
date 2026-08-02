// AppLogStream.tsx is the store's sole writer of the `appLog` slice
// (store.ts), mirroring LiveStatusBar.tsx's identical "always-mounted
// subscribe-in-effect, write to the shared store" role for `status`/
// `connectionStatus` -- GlobalFrame mounts this unconditionally (never
// inside WorkspaceRouter's switch) for the same reason GlobalFrame mounts
// LiveStatusBar unconditionally: most "app:log" lines fire during
// App.OnStartup, within the first moments the window opens, well before an
// operator has necessarily navigated to the Diagnostics workspace.
//
// Even mounted here, at the shell's own top level, a *pure* live
// subscription is not enough: the very first "app:log" flush (events.go's
// EventPusher, ticking every ~25ms once App.OnStartup calls
// a.events.Start) can fire before this component's own effect has even
// run -- React mounting and the whole frontend bundle loading is not
// instant, and Wails' EventsEmit is fire-and-forget, never replayed to a
// listener that subscribes even a few milliseconds late. So this also
// fetches App.RecentAppLogs() (fetchRecentAppLogs, wailsBridge.ts) once on
// mount -- an ordinary request/response call, which unlike a push can
// never be "too late" -- and seeds the store with whatever already
// happened, before subscribing to the live push for everything from here
// on.
//
// Renders nothing: this is a pure subscription/store-write component, the
// same shape LiveStatusBar would have if it carried no visible chrome of
// its own.
import { useEffect } from "react";

import { useGolcStore } from "../store/store";
import { fetchRecentAppLogs, onAppLog } from "../lib/wailsBridge";

export default function AppLogStream() {
  const appendAppLog = useGolcStore((state) => state.appendAppLog);
  const seedAppLog = useGolcStore((state) => state.seedAppLog);

  useEffect(() => {
    // Subscribe to the live push first, so nothing from this point forward
    // is missed, then fetch the backlog -- any overlap between the two
    // (a line that arrived live in the brief window before the fetch
    // resolves) is deduped by seedAppLog itself.
    const unsubscribe = onAppLog(appendAppLog);
    void fetchRecentAppLogs().then(seedAppLog);
    return unsubscribe;
  }, [appendAppLog, seedAppLog]);

  return null;
}
