// errors.go provides the one shared way every tool handler reports a
// failure: a tool-level error result (IsError set, human-readable message
// in Content) rather than a protocol-level JSON-RPC error, per the MCP
// guidance that clients should be able to show tool failures inline
// instead of as transport faults.
package main

import "github.com/modelcontextprotocol/go-sdk/mcp"

// toolError builds the (*mcp.CallToolResult, Out, error) triple every
// generated ToolHandlerFor must return on failure. Out is always the
// zero value; callers must supply it explicitly, e.g.
// toolError[myOutputType](err). Returning a non-nil error here is safe
// (not a protocol-level fault): the SDK sets CallToolResult.IsError and
// surfaces err.Error() as text content so the model can see and
// self-correct, per ToolHandlerFor's documented contract.
func toolError[Out any](err error) (*mcp.CallToolResult, Out, error) {
	var zero Out
	return nil, zero, err
}
