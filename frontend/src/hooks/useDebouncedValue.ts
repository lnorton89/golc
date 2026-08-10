// useDebouncedValue returns `value` delayed until it has stopped changing
// for `delayMs`, so a typing burst settles into one downstream read.
//
// This stays a separate concern from TanStack Query deliberately. Query
// dedupes and caches by key, but it has no notion of "wait until the
// operator stops typing" -- keying a query directly off raw input would
// issue one bridge call per keystroke, each with its own distinct key, and
// cache every intermediate prefix. Debouncing first means the key only ever
// takes settled values.
import { useEffect, useState } from "react";

export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
