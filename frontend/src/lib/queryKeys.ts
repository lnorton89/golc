// queryKeys.ts is the single place a TanStack Query cache key is spelled.
// Keys are written as factories rather than inline array literals at each
// call site so that a mutation's invalidateQueries({ queryKey: ... }) and
// the useQuery it is meant to refresh can never silently drift apart -- a
// typo in one half of that pair produces a query that simply never
// refreshes, which is invisible until an operator is staring at stale data.
//
// Each factory returns a prefix-compatible tuple: invalidating
// fixtureLibrary.all() also invalidates every fixtureLibrary.* entry
// beneath it, because Query matches keys by array prefix.
export const queryKeys = {
  fixtureLibrary: {
    all: () => ["fixtureLibrary"] as const,
    /** The local fixtures directory listing (FixtureLibraryService.ListLocal). */
    local: () => ["fixtureLibrary", "local"] as const,
    /** One fixture file's Inspect result, keyed by its resolved path. */
    inspect: (path: string) => ["fixtureLibrary", "inspect", path] as const,
    /** An OFL catalog search, keyed by the trimmed query string.
     *
     * Keying by the query is what removes the stale-response race the
     * hand-rolled version needed a monotonic request-id ref to guard
     * against: two in-flight searches for different text are two distinct
     * cache entries, so a slow "chau" response can no longer land after a
     * fast "chauvet" one and overwrite it. */
    oflSearch: (query: string) => ["fixtureLibrary", "oflSearch", query] as const,
  },
} as const;
