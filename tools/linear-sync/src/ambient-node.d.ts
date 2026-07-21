// ambient-node.d.ts declares the minimal ambient Node.js runtime surface
// this isolated workspace needs -- the `process` global for src/cli.ts,
// plus the "node:test", "node:assert/strict", and "node:fs" module shapes
// test/operations.test.ts imports -- without depending on @types/node.
//
// CONTEXT D-01 and Plan 01-12's blocking human checkpoint approved exactly
// two packages for this workspace: @linear/sdk@88.1.0 and
// typescript@7.0.2. Adding @types/node would be a third package requiring
// a new approval gate (deviation Rule 3's package-install exclusion), so
// this file supplies just enough hand-written ambient typing for the
// small, stable Node surface this workspace actually touches instead.
// TypeScript ambient declarations in an included .d.ts file apply to the
// whole compilation, so this single file covers both src/**/*.ts and
// test/**/*.ts.

declare const process: {
  readonly argv: string[];
  readonly env: Record<string, string | undefined>;
  exitCode?: number;
  cwd(): string;
  readonly stdin: AsyncIterable<string> & { setEncoding(encoding: string): void };
  readonly stdout: { write(chunk: string): unknown };
  readonly stderr: { write(chunk: string): unknown };
};

declare module "node:fs" {
  export function readFileSync(path: string, encoding: string): string;
}

declare module "node:test" {
  interface TestContext {
    test(name: string, fn: (t: TestContext) => void | Promise<void>): Promise<void>;
  }
  export function test(name: string, fn: (t: TestContext) => void | Promise<void>): Promise<void>;
}

declare module "node:assert/strict" {
  type AssertErrorMatcher = string | Error | RegExp | ((error: unknown) => boolean);
  function assertStrict(value: unknown, message?: string | Error): void;
  namespace assertStrict {
    function ok(value: unknown, message?: string | Error): void;
    function strictEqual(actual: unknown, expected: unknown, message?: string | Error): void;
    function deepStrictEqual(actual: unknown, expected: unknown, message?: string | Error): void;
    function throws(fn: () => unknown, matcher?: AssertErrorMatcher, message?: string | Error): void;
    function rejects(fn: () => Promise<unknown>, matcher?: AssertErrorMatcher, message?: string | Error): Promise<void>;
  }
  export default assertStrict;
}
