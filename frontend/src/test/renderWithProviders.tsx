// renderWithProviders is the drop-in @testing-library/react `render` for
// any component that needs the app-level providers App.tsx mounts. A
// component under test is mounted standalone, so it inherits none of them:
// without a wrapper, reading through TanStack Query throws "No QueryClient
// set", and calling useToast() throws for the missing Toast provider.
//
// This wrapper must stay a mirror of App.tsx's provider set. When a new
// provider is added there, add it here too -- otherwise every standalone
// component test starts failing at once with a context error, which reads
// like a broken component rather than a missing test wrapper.
//
// A test file adopts it by changing only its import:
//
//   import { render } from "../../test/renderWithProviders";
//
// rather than rewriting each render() call site.
//
// Every call builds a FRESH QueryClient. Test isolation depends on it: a
// client shared across cases would serve one case's cached rows to the next
// and turn an ordering change into a mystery failure. createQueryClient()'s
// production defaults (retry: 0, no focus/reconnect refetching) are already
// the right ones for jsdom, so this deliberately does not fork a second set
// of options that could drift from what actually ships.
import type { ReactElement, ReactNode } from "react";
import {
  render as testingLibraryRender,
  renderHook as testingLibraryRenderHook,
  type RenderHookOptions,
  type RenderHookResult,
  type RenderOptions,
  type RenderResult,
} from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";

import Toast from "../components/primitives/Toast/Toast";
import { createQueryClient } from "../lib/queryClient";

function makeWrapper() {
  const queryClient = createQueryClient();
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <Toast>{children}</Toast>
      </QueryClientProvider>
    );
  };
}

export function render(ui: ReactElement, options?: Omit<RenderOptions, "wrapper">): RenderResult {
  return testingLibraryRender(ui, { ...options, wrapper: makeWrapper() });
}

/** The renderHook counterpart, for hooks that read through Query directly
 * (usePlaybackStateSnapshot) rather than via a component under test. */
export function renderHook<Result, Props>(
  hook: (initialProps: Props) => Result,
  options?: Omit<RenderHookOptions<Props>, "wrapper">,
): RenderHookResult<Result, Props> {
  return testingLibraryRenderHook(hook, { ...options, wrapper: makeWrapper() });
}

// Re-exported so an adopting test file can take everything it needs from
// this one module instead of importing `render` from here and `screen`,
// `waitFor`, ... from @testing-library/react separately.
export * from "@testing-library/react";
