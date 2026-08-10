// fuzzySearch is the one place this frontend decides what "matches" means
// for an operator-typed query.
//
// Every search here used to be `haystack.toLowerCase().includes(needle)`,
// which has two problems an operator actually hits. It cannot rank -- a
// substring hit at the end of a name scores exactly as well as an exact
// prefix, so results came back in whatever order the source array happened
// to be in. And it cannot forgive: one transposed character in "Chauvet"
// returns nothing at all, with no indication whether the fixture is absent
// or the spelling was wrong. That matters most on the surface with the most
// entries, the Open Fixture Library catalog, where a miss is indistinguishable
// from "this manufacturer isn't in the catalog".
//
// uFuzzy is the matcher because it is ~5KB and built for exactly this
// shape of problem (a flat list of short labels, typed at interactively),
// rather than a general document-search engine.
import uFuzzy from "@leeoniya/ufuzzy";

// intraMode 1 is uFuzzy's SingleError mode: one insertion, substitution,
// transposition, or deletion is tolerated PER TERM. That is the difference
// between "chauvte" finding Chauvet and finding nothing.
//
// Errors are deliberately not tolerated in the first character (uFuzzy's
// intraSlice default of [1, Infinity]). An operator who types "d" expects
// things starting with d, and allowing a first-character error turns a
// one-letter query into a match against most of the catalog.
const matcher = new uFuzzy({
  intraMode: 1,
  intraIns: 1,
  intraSub: 1,
  intraTrn: 1,
  intraDel: 1,
});

// OUT_OF_ORDER_TERMS lets "pro slimpar" find "SlimPAR Pro". Bounded at 3
// because uFuzzy permutes terms factorially -- 3 terms is 6 permutations,
// while its own docs warn that 5 is 120. Queries longer than this fall back
// to in-order matching rather than getting slow.
const OUT_OF_ORDER_TERMS = 3;

/** fuzzySearch filters and RANKS items against query, best match first.
 *
 * An empty query returns every item in its original order -- callers use
 * this as the "no search active" path, so it must not reorder anything.
 *
 * toHaystack builds the searchable string for one item. Join multiple
 * fields with a space to make them all searchable: uFuzzy matches across
 * the whole string, so a manufacturer + model haystack lets either (or a
 * query spanning both) hit. */
export function fuzzySearch<T>(items: readonly T[], query: string, toHaystack: (item: T) => string): T[] {
  const needle = query.trim();
  if (needle === "") return items.slice();

  const haystack = items.map(toHaystack);
  const [idxs, info, order] = matcher.search(haystack, needle, OUT_OF_ORDER_TERMS);

  // Aborted: uFuzzy bailed out (a needle it cannot build a search for).
  // Returning nothing rather than everything keeps "no results" honest --
  // silently showing the unfiltered list would read as "these all match".
  if (idxs === null) return [];

  // Filtered but not ranked: uFuzzy skips the ranking pass past its
  // infoThresh (1e3 matches) because sorting that many is not worth the
  // work. Order is then the haystack's own, which is what the substring
  // filter always gave.
  if (info === null || order === null) return idxs.map((index) => items[index]);

  return order.map((position) => items[info.idx[position]]);
}

/** fuzzyMatches is the single-item predicate, for callers that own only a
 * yes/no decision per candidate and cannot reorder the list themselves
 * (Base UI's Combobox `filter` prop). It buys the same typo tolerance;
 * ranking is unavailable through that API by construction. */
export function fuzzyMatches(text: string, query: string): boolean {
  const needle = query.trim();
  if (needle === "") return true;
  const idxs = matcher.filter([text], needle);
  return idxs !== null && idxs.length > 0;
}
