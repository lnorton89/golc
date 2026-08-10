import { describe, expect, it } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";

import { useLatestRequest } from "./useLatestRequest";

/** deferred hands back a promise plus its resolver, so a test can land two
 * overlapping responses in whichever order it likes. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

describe("useLatestRequest", () => {
  it("lets a lone request commit", async () => {
    const { result } = renderHook(() => useLatestRequest());
    const isCurrent = result.current();
    expect(isCurrent()).toBe(true);
  });

  it("invalidates an earlier request once a later one starts, even if the earlier one resolves last", async () => {
    const { result } = renderHook(() => useLatestRequest());
    const first = deferred<string>();
    const second = deferred<string>();
    const committed: string[] = [];

    // Two overlapping reads, started in order A then B.
    const runA = (async () => {
      const isCurrent = result.current();
      const value = await first.promise;
      if (isCurrent()) committed.push(value);
    })();
    const runB = (async () => {
      const isCurrent = result.current();
      const value = await second.promise;
      if (isCurrent()) committed.push(value);
    })();

    // B resolves first, then the slower A lands afterwards.
    second.resolve("B");
    await runB;
    first.resolve("A");
    await runA;

    expect(committed).toEqual(["B"]);
  });

  it("still commits the newest response when the older one resolves first", async () => {
    const { result } = renderHook(() => useLatestRequest());
    const first = deferred<string>();
    const second = deferred<string>();
    const committed: string[] = [];

    const runA = (async () => {
      const isCurrent = result.current();
      const value = await first.promise;
      if (isCurrent()) committed.push(value);
    })();
    const runB = (async () => {
      const isCurrent = result.current();
      const value = await second.promise;
      if (isCurrent()) committed.push(value);
    })();

    first.resolve("A");
    await runA;
    second.resolve("B");
    await runB;

    expect(committed).toEqual(["B"]);
  });

  it("invalidates an in-flight request when the component unmounts", async () => {
    const { result, unmount } = renderHook(() => useLatestRequest());
    const pending = deferred<string>();
    const committed: string[] = [];

    const run = (async () => {
      const isCurrent = result.current();
      const value = await pending.promise;
      if (isCurrent()) committed.push(value);
    })();

    act(() => {
      unmount();
    });
    pending.resolve("late");
    await run;

    expect(committed).toEqual([]);
  });

  it("keeps a stable `begin` identity across re-renders", () => {
    const { result, rerender } = renderHook(() => useLatestRequest());
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });

  it("re-arms after a StrictMode-style unmount/remount cycle", async () => {
    const { result, unmount } = renderHook(() => useLatestRequest());
    act(() => {
      unmount();
    });
    cleanup();

    const remounted = renderHook(() => useLatestRequest());
    const isCurrent = remounted.result.current();
    expect(isCurrent()).toBe(true);
    void result;
  });
});
