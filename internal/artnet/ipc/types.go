// types.go declares this package's IPC wire-format types (04-05-PLAN.md
// Task 1 deviation, [Rule 3 - circular dependency]): Request/Result/Handler
// are field-for-field identical to internal/command's Request/Result/
// CommandHandler (04-04's original reused shape, per RESEARCH.md Pattern
// 5), but declared locally here instead of imported from internal/command.
//
// Why: 04-04's daemon.go and this package's server.go/client.go originally
// imported internal/command purely for these three types. 04-05's
// internal/command/artnet.go must import this ipc package directly
// (ipc.Dial/ipc.Forward) and internal/artnet directly (artnet.Run,
// artnet.ListCandidateInterfaces) to build the CLI's client routes and the
// "artnet serve" route -- but if this package (or internal/artnet) also
// imported internal/command, that would be a hard two/three-node Go import
// cycle (command -> artnet/ipc -> command, and command -> artnet ->
// command), which fails to compile. This mirrors internal/projectconfig's
// existing precedent (see internal/command/config.go's own doc comment:
// "internal/projectconfig ... stays a pure configuration library (no
// command import)") -- a subsystem package never imports internal/command;
// only internal/command imports the subsystem.
//
// encoding/json marshals a struct by its field names when no struct tags
// are present (internal/strictjson.CanonicalEncode/DecodeStrict use plain
// encoding/json underneath); since these types declare the exact same
// field names as command.Request/command.Result with no tags either, the
// wire JSON shape this package and daemon.go encode/decode is byte-
// identical to before this change -- no wire-format behavior changes, only
// which package the Go type is declared in.
package ipc

// Request carries one routed invocation to the daemon, field-for-field
// identical to command.Request.
type Request struct {
	Route string
	Args  []string
	Root  string
}

// Result is a handler outcome, field-for-field identical to command.Result.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// Handler executes one routed request -- the daemon-side handler signature
// Serve dispatches to, mirroring command.CommandHandler's shape.
type Handler func(Request) Result
