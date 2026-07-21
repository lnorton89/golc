// cli.ts is the strict NDJSON process-boundary entrypoint for this
// isolated workspace (CONTEXT D-01/D-03; Plan 01-25 must_haves.artifacts:
// "Strict NDJSON adapter entrypoint"). It reads exactly one JSON-encoded
// Operation (protocol.ts) per line from an input stream, strictly decodes
// it (decodeOperation -- unknown operation/kind/result shapes fail
// closed), executes it through an injected OperationExecutor
// (LinearSdkAdapter in production; a fake SDK stub in
// test/operations.test.ts), and writes exactly one JSON-encoded result
// per line to an output stream. It never opens an HTTP server, never
// listens on a socket, and never makes a reconciliation policy decision --
// identity discovery, three-way conflict detection, ordering, and plan
// construction all stay in Go (internal/trace/reconcile, CONTEXT
// D-11/D-13/D-17). A malformed line fails the whole run closed rather than
// being skipped silently, so a caller never mistakes a partial transcript
// for a complete one.
import { decodeOperation, ProtocolDecodeError, type Operation } from "./protocol.js";
import { createLinearSdkAdapter } from "./adapter.js";

/**
 * OperationExecutor is the narrow contract runCLI depends on: something
 * that executes one already-decoded Operation and returns its result.
 * LinearSdkAdapter satisfies this exactly; operations.test.ts's fake SDK
 * adapter also satisfies it without ever constructing a real LinearClient
 * or opening a network connection.
 */
export interface OperationExecutor {
  execute(operation: Operation): Promise<unknown>;
}

/**
 * CliInput/CliOutput are the minimal structural stream contracts runCLI
 * needs: an async-iterable source of text chunks, and a sink that accepts
 * a string. process.stdin/process.stdout satisfy these once
 * process.stdin.setEncoding("utf8") has been called (main() below does
 * this); a test harness can satisfy them with a plain in-memory
 * implementation.
 */
export type CliInput = AsyncIterable<string>;
export interface CliOutput {
  write(chunk: string): unknown;
}

/**
 * CliProtocolError names the exact input line a decode failure occurred
 * on, so a caller can report precisely which NDJSON line was malformed.
 */
export class CliProtocolError extends Error {
  constructor(lineNumber: number, detail: string) {
    super(`LINEAR_CLI_LINE_${lineNumber}: ${detail}`);
    this.name = "CliProtocolError";
  }
}

/**
 * runCLI reads newline-delimited JSON operations from input, strictly
 * decodes and executes each one through executor in order, and writes one
 * newline-delimited JSON result per operation to output. It resolves with
 * the total number of operations processed, or rejects with the first
 * CliProtocolError or executor error encountered -- there is no partial
 * best-effort mode; the caller decides how to report a failure. Manual
 * line-buffering (rather than importing "node:readline"/"node:stream")
 * keeps this module's ambient type surface to exactly the small
 * hand-declared subset ambient-node.d.ts provides.
 */
export async function runCLI(input: CliInput, output: CliOutput, executor: OperationExecutor): Promise<number> {
  let buffer = "";
  let lineNumber = 0;
  let processed = 0;

  const handleLine = async (rawLine: string): Promise<void> => {
    lineNumber++;
    const line = rawLine.trim();
    if (line.length === 0) {
      return;
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch (error) {
      throw new CliProtocolError(lineNumber, `invalid JSON: ${(error as Error).message}`);
    }

    let operation: Operation;
    try {
      operation = decodeOperation(parsed);
    } catch (error) {
      if (error instanceof ProtocolDecodeError) {
        throw new CliProtocolError(lineNumber, error.message);
      }
      throw error;
    }

    const result = await executor.execute(operation);
    output.write(JSON.stringify(result) + "\n");
    processed++;
  };

  for await (const chunk of input) {
    buffer += chunk;
    let newlineIndex = buffer.indexOf("\n");
    while (newlineIndex !== -1) {
      const line = buffer.slice(0, newlineIndex);
      buffer = buffer.slice(newlineIndex + 1);
      await handleLine(line);
      newlineIndex = buffer.indexOf("\n");
    }
  }
  if (buffer.length > 0) {
    await handleLine(buffer);
  }

  return processed;
}

/**
 * isEntryPoint is true only when this module is the process's own entry
 * script (node dist/src/cli.js), never when it is imported as a module
 * (test/operations.test.ts imports runCLI directly and never triggers
 * main()).
 */
function isEntryPoint(): boolean {
  const entry = process.argv[1];
  return typeof entry === "string" && entry.endsWith("cli.js");
}

/**
 * main is the process entrypoint: it reads LINEAR_API_KEY from the
 * environment (never logs or echoes it, matching D-19/D-20), wires a real
 * LinearSdkAdapter, and runs runCLI against process stdin/stdout. It is
 * never invoked by operations.test.ts, which injects its own fake
 * executor and never touches process.env.LINEAR_API_KEY or a real
 * LinearClient.
 */
async function main(): Promise<void> {
  const apiKey = process.env.LINEAR_API_KEY;
  if (!apiKey) {
    throw new Error("LINEAR_CLI_MISSING_API_KEY: LINEAR_API_KEY is not set");
  }
  process.stdin.setEncoding("utf8");
  const adapter = createLinearSdkAdapter(apiKey);
  await runCLI(process.stdin, process.stdout, adapter);
}

if (isEntryPoint()) {
  main().catch((error: unknown) => {
    process.stderr.write(`${(error as Error).message}\n`);
    process.exitCode = 1;
  });
}
