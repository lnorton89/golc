// queryClient.ts owns the single TanStack Query configuration for the app.
// Its defaults are deliberately NOT the library's web defaults, because
// every query function in this codebase reads through wailsBridge.ts, whose
// transport is an in-process Go call rather than the network Query was
// tuned for:
//
//   - retry: 0. wailsBridge.ts's read functions are documented as
//     non-throwing ("callers never need their own try/catch"): a missing
//     bridge or a rejected call is caught there and turned into an explicit
//     degraded view (offlineFixtureLibraryView, offlineOflSearchView, ...)
//     that says so honestly. A promise that never rejects can never trigger
//     a retry, so leaving Query's default of 3 would be inert bookkeeping
//     that also misleads the next reader into thinking failures are retried
//     here. The degraded view IS the error channel -- read `unreachable` /
//     `valid` / `errors` off the data, never `useQuery().isError`.
//
//   - refetchOnWindowFocus: false. Focus is a staleness proxy for a browser
//     tab that may have sat behind other tabs for an hour. This is a desktop
//     window whose backing state changes are pushed to us by the Go host
//     over runtime.EventsOn, so refocusing the console mid-show is not
//     evidence that anything went stale -- and a burst of refetches every
//     time an operator alt-tabs back to the desk is exactly the kind of
//     unrequested work a live output path should not be doing.
//
//   - refetchOnReconnect: false. navigator.onLine describes the machine's
//     network, which has no bearing on a bridge that lives inside this same
//     process. An offline show network (the case main.tsx already self-hosts
//     fonts for) must not invalidate local reads.
//
//   - staleTime: 0 with explicit invalidation. Reads stay cheap and
//     deduped within a render pass, and freshness comes from mutations
//     calling invalidateQueries against the keys in queryKeys.ts rather
//     than from a timer nobody can trace back to a cause.
import { QueryClient } from "@tanstack/react-query";

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: 0,
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        staleTime: 0,
      },
      mutations: {
        retry: 0,
      },
    },
  });
}
