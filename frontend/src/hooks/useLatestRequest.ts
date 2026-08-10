// useLatestRequest is the generation guard for hand-rolled async reads --
// the "a slower earlier response lands after a faster later one and
// overwrites it" class of bug the 2026-08-10 frontend review pass found
// four separate instances of (OperatorSurface's surface detail, MidiPanel's
// surface detail + mappings, FixturePatch's remove-member impact preview,
// BarTimelinePanel's Evaluate), plus the same shape in every Guided First
// Show stage reporting upward through onStatusChange after unmount.
//
// The workspaces already on TanStack Query (FixtureLibraryWorkspace,
// usePlaybackStateSnapshot, Desk) get this for free from queryKey identity;
// this is the equivalent for the call sites that are not on Query and are
// not worth migrating for one guard. It deliberately does NOT try to be a
// fetching library -- no caching, no loading state, no retries. It answers
// exactly one question: "is the response I am holding still the one this
// component wants?"
//
// Two things invalidate a response: a newer request started after it, and
// the component unmounting. Both are answered by the same `isCurrent()`
// closure, so a call site needs one check before each commit rather than a
// separate `cancelled` flag per effect.
import { useCallback, useEffect, useRef } from "react";

/** useLatestRequest returns `begin`. Call it once at the top of an async
 * handler to claim the newest generation; it hands back an `isCurrent()`
 * predicate to check before committing anything to state:
 *
 * ```ts
 * const beginLatest = useLatestRequest();
 * const refresh = useCallback(async (name: string) => {
 *   const isCurrent = beginLatest();
 *   const detail = await service().ShowSurface(name);
 *   if (!isCurrent()) return;   // a newer refresh won, or we unmounted
 *   setControls(detail.controls);
 * }, [beginLatest]);
 * ```
 *
 * `begin` is referentially stable, so it is safe in a useCallback/useEffect
 * dependency array. */
export function useLatestRequest(): () => () => boolean {
  const generationRef = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    // Re-arm on mount as well as clearing on unmount: React StrictMode
    // mounts, unmounts, and remounts every component in development, and
    // a one-way `mounted = false` would leave the remounted instance
    // permanently unable to commit anything.
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  return useCallback(() => {
    generationRef.current += 1;
    const generation = generationRef.current;
    return () => mountedRef.current && generation === generationRef.current;
  }, []);
}
