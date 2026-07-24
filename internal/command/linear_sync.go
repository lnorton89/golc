// linear_sync.go self-registers the isolated tools/linear-sync Node
// workspace's exact build and quick-test scopes (CONTEXT D-01/D-03; Plan
// 01-13) through the same MustDeclareRoute/MustDeclareScope
// self-registration contract Plan 17 established in
// internal/command/router.go: internal/command/test.go's own doc comment
// names tools/linear-sync as the intended first consumer of
// MustDeclareNodeScope, and build.go's MustDeclareNodeBuildScope (Plan
// 01-13) is the parallel build-side registration. config/commands.toml
// documents these exact two scope names/directory as the single
// authoritative declaration (D-05); this file only wires the
// already-established build/test dispatch contracts to that documented
// pair. It never redeclares the scope name, Node version, or workspace
// directory anywhere else, and it never runs npm/tsc itself -- both
// dispatchers (build.go/test.go) resolve the pinned project-local Node
// toolchain at request time.
package command

// linearSyncWorkspaceDir is the single repository-relative directory both
// registrations below point at, matching tools/linear-sync's own
// package.json/tsconfig.json location exactly.
const linearSyncWorkspaceDir = "tools/linear-sync"

// linear-sdk is the exact build scope config/commands.toml documents for
// tools/linear-sync: `build --scope linear-sdk` compiles protocol.ts/
// adapter.ts with the pinned project-local TypeScript compiler.
var _ = MustDeclareNodeBuildScope(NodeBuildScopeRegistration{
	Scope: "linear-sdk",
	Dir:   linearSyncWorkspaceDir,
})

// linear-sdk-operations is the exact quick-test scope config/commands.toml
// documents for tools/linear-sync: `test --quick --scope
// linear-sdk-operations` runs the workspace's Node test suite (Plan
// 01-25's test/operations.test.ts, compiled to dist/test/*.test.js).
// The registration retains only arguments; test.go resolves the pinned
// executable from Request.Root immediately before execution.
var _ = MustDeclareNodeScope(NodeScopeRegistration{
	Scope:     "linear-sdk-operations",
	Dir:       linearSyncWorkspaceDir,
	Marker:    "TestScopeLinearSdkOperations",
	Arguments: linearSyncNodeTestCommand(),
})

// linear-transport-pagination is the exact quick-test scope
// config/commands.toml documents for tools/linear-sync's exhaustive Relay
// connection pagination (Plan 01-14; T-01-39): `test --quick --scope
// linear-transport-pagination` runs test/pagination.test.ts alone
// (compiled to dist/test/pagination.test.js), asserting its
// TestScopeLinearTransportPagination marker before anything executes,
// exactly as MustDeclareNodeScope's fail-on-missing-marker/exit-nonzero
// contract already requires for every registered Node scope.
var _ = MustDeclareNodeScope(NodeScopeRegistration{
	Scope:     "linear-transport-pagination",
	Dir:       linearSyncWorkspaceDir,
	Marker:    "TestScopeLinearTransportPagination",
	Arguments: linearSyncNodeTestCommandPagination(),
})

// linear-transport-errors is the exact quick-test scope
// config/commands.toml documents for tools/linear-sync's HTTP-200
// data-plus-errors and rate-limit normalization (Plan 01-26; T-01-40):
// `test --quick --scope linear-transport-errors` runs both
// test/errors.test.ts and test/rate-limit.test.ts (compiled to
// dist/test/errors.test.js and dist/test/rate-limit.test.js), asserting
// its TestScopeLinearTransportErrors marker before anything executes,
// exactly as MustDeclareNodeScope's fail-on-missing-marker/exit-nonzero
// contract already requires for every registered Node scope.
var _ = MustDeclareNodeScope(NodeScopeRegistration{
	Scope:     "linear-transport-errors",
	Dir:       linearSyncWorkspaceDir,
	Marker:    "TestScopeLinearTransportErrors",
	Arguments: linearSyncNodeTestCommandErrors(),
})

// linear-transport-node is the exact quick-test scope config/commands.toml
// documents for tools/linear-sync's redaction/uncertain-write contract
// (Plan 01-27; T-01-40/T-01-41): `test --quick --scope
// linear-transport-node` runs both test/redact.test.ts and
// test/mutation.test.ts (compiled to dist/test/redact.test.js and
// dist/test/mutation.test.js), asserting its TestScopeLinearTransportNode
// marker before anything executes, exactly as MustDeclareNodeScope's
// fail-on-missing-marker/exit-nonzero contract already requires for every
// registered Node scope.
var _ = MustDeclareNodeScope(NodeScopeRegistration{
	Scope:     "linear-transport-node",
	Dir:       linearSyncWorkspaceDir,
	Marker:    "TestScopeLinearTransportNode",
	Arguments: linearSyncNodeTestCommandNode(),
})

// linearSyncNodeTestGlob is a glob pattern, not a bare directory: Node's
// own --test path resolution (confirmed empirically against the pinned
// Node 24.18.0 build) fails a bare directory argument with
// MODULE_NOT_FOUND rather than auto-discovering test files inside it, but
// natively resolves a glob pattern itself (no shell expansion required,
// so this is safe to pass through exec.Command's argv unquoted). "**"
// covers any future nested test file without editing this dispatcher
// again.
const linearSyncNodeTestGlob = "dist/test/**/*.test.js"

// linearSyncNodeTestFilePagination is the exact compiled output path of
// Plan 01-14's pagination.test.ts. Unlike linearSyncNodeTestGlob (which
// runs every test file for the broad "linear-sdk-operations" scope), the
// "linear-transport-pagination" scope registered below targets this one
// file so a scoped `test --quick --scope linear-transport-pagination` run
// exercises exactly TestScopeLinearTransportPagination and nothing else.
const linearSyncNodeTestFilePagination = "dist/test/pagination.test.js"

// linearSyncNodeTestFilesErrors are the exact compiled output paths of
// Plan 01-26's errors.test.ts and rate-limit.test.ts. The
// "linear-transport-errors" scope registered below targets exactly these
// two files (Node's --test accepts multiple positional file arguments, no
// shell expansion required since exec.Command passes argv directly) so a
// scoped `test --quick --scope linear-transport-errors` run exercises
// exactly the data-plus-errors and rate-limit fixtures and nothing else.
var linearSyncNodeTestFilesErrors = []string{"dist/test/errors.test.js", "dist/test/rate-limit.test.js"}

// linearSyncNodeTestFilesNode are the exact compiled output paths of Plan
// 01-27's redact.test.ts and mutation.test.ts. The "linear-transport-node"
// scope registered above targets exactly these two files (mirroring
// linearSyncNodeTestFilesErrors's same multi-file precedent) so a scoped
// `test --quick --scope linear-transport-node` run exercises exactly the
// canary-scan/safe-uncertain-outcome contract and the mutation-uncertain
// fixture, and nothing else.
var linearSyncNodeTestFilesNode = []string{"dist/test/redact.test.js", "dist/test/mutation.test.js"}

// linearSyncNodeTestCommand returns only the Node arguments retained by
// the registration. The pinned executable is resolved from Request.Root
// immediately before execution.
func linearSyncNodeTestCommand() []string {
	return []string{"--test", linearSyncNodeTestGlob}
}

// linearSyncNodeTestCommandPagination retains the exact arguments for the
// pagination test scope.
func linearSyncNodeTestCommandPagination() []string {
	return []string{"--test", linearSyncNodeTestFilePagination}
}

// linearSyncNodeTestCommandErrors retains the exact arguments for the
// errors and rate-limit test scope.
func linearSyncNodeTestCommandErrors() []string {
	return append([]string{"--test"}, linearSyncNodeTestFilesErrors...)
}

// linearSyncNodeTestCommandNode retains the exact arguments for the
// redaction and mutation test scope.
func linearSyncNodeTestCommandNode() []string {
	return append([]string{"--test"}, linearSyncNodeTestFilesNode...)
}
