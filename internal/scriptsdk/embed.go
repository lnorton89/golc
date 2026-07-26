// embed.go embeds the committed golc-runtime.ts bytes so internal/script
// (08-05, host.go's session materialization step) can prepend them ahead
// of a user's script source without re-generating or reading generated/
// golc-runtime.ts from disk at run time. RuntimeShimTS is always exactly
// the committed, drift-checked bytes CheckDrift already guards -- never a
// freshly re-rendered copy -- so the bytes injected into a sandboxed run
// are always the same ones a contributor reviewed and committed.
package scriptsdk

import _ "embed"

//go:embed generated/golc-runtime.ts
var RuntimeShimTS string
