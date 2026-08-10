import { describe, expect, it } from "vitest";

import { fuzzyMatches, fuzzySearch } from "./fuzzySearch";

const FIXTURES = [
  { manufacturer: "Chauvet", model: "SlimPAR Pro" },
  { manufacturer: "American DJ", model: "Mega Par Profile" },
  { manufacturer: "Martin", model: "MAC Aura" },
  { manufacturer: "Robe", model: "Pointe" },
];

const haystack = (f: { manufacturer: string; model: string }) => `${f.manufacturer} ${f.model}`;
const models = (results: typeof FIXTURES) => results.map((f) => f.model);

describe("fuzzySearch", () => {
  it("returns every item, in the original order, for an empty query", () => {
    expect(fuzzySearch(FIXTURES, "", haystack)).toEqual(FIXTURES);
    expect(fuzzySearch(FIXTURES, "   ", haystack)).toEqual(FIXTURES);
  });

  it("does not mutate or alias the caller's array on the empty-query path", () => {
    const result = fuzzySearch(FIXTURES, "", haystack);
    expect(result).not.toBe(FIXTURES);
  });

  it("matches case-insensitively", () => {
    expect(models(fuzzySearch(FIXTURES, "chauvet", haystack))).toEqual(["SlimPAR Pro"]);
    expect(models(fuzzySearch(FIXTURES, "CHAUVET", haystack))).toEqual(["SlimPAR Pro"]);
  });

  it("matches on either field of a joined haystack", () => {
    expect(models(fuzzySearch(FIXTURES, "martin", haystack))).toEqual(["MAC Aura"]);
    expect(models(fuzzySearch(FIXTURES, "pointe", haystack))).toEqual(["Pointe"]);
  });

  // The substring filter this replaced returned nothing for any of these.
  it("tolerates a single typo per term", () => {
    expect(models(fuzzySearch(FIXTURES, "chauvte", haystack)), "transposition").toEqual(["SlimPAR Pro"]);
    expect(models(fuzzySearch(FIXTURES, "chauvvet", haystack)), "insertion").toEqual(["SlimPAR Pro"]);
    expect(models(fuzzySearch(FIXTURES, "chauet", haystack)), "deletion").toEqual(["SlimPAR Pro"]);
  });

  it("does not tolerate an error in the first character", () => {
    // "xhauvet" must not find Chauvet: allowing a first-character error
    // turns a short query into a match against most of a catalog.
    expect(fuzzySearch(FIXTURES, "xhauvet", haystack)).toEqual([]);
  });

  it("finds terms typed out of order", () => {
    expect(models(fuzzySearch(FIXTURES, "pro slimpar", haystack))).toEqual(["SlimPAR Pro"]);
  });

  it("returns an empty list when nothing matches, never the unfiltered list", () => {
    expect(fuzzySearch(FIXTURES, "zzznomatch", haystack)).toEqual([]);
  });

  it("ranks an exact prefix above a mid-string match", () => {
    // Both contain "par"; the substring filter left them in array order,
    // which put the Chauvet entry first purely by accident of position.
    const ranked = models(fuzzySearch(FIXTURES, "Mega", haystack));
    expect(ranked[0]).toBe("Mega Par Profile");
  });

  it("preserves the caller's item objects rather than the haystack strings", () => {
    const [first] = fuzzySearch(FIXTURES, "robe", haystack);
    expect(first).toEqual({ manufacturer: "Robe", model: "Pointe" });
  });
});

describe("fuzzyMatches", () => {
  it("accepts everything for an empty query", () => {
    expect(fuzzyMatches("Chauvet SlimPAR Pro", "")).toBe(true);
  });

  it("matches case-insensitively and tolerates a typo", () => {
    expect(fuzzyMatches("Chauvet SlimPAR Pro", "CHAUVET")).toBe(true);
    expect(fuzzyMatches("Chauvet SlimPAR Pro", "chauvte")).toBe(true);
  });

  it("rejects a non-match", () => {
    expect(fuzzyMatches("Chauvet SlimPAR Pro", "zzznomatch")).toBe(false);
  });
});
